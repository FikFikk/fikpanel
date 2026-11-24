package tui

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"fikpanel/internal/ai"
	"fikpanel/internal/docker"
	"fikpanel/internal/metrics"
	"fikpanel/internal/node"
	"fikpanel/internal/services"
	"fikpanel/internal/store"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Config menyimpan parameter inisialisasi dashboard TUI FikPanel
type Config struct {
	Version         string
	DBPath          string
	RefreshInterval time.Duration
}

type tabID int

const (
	tabOverview tabID = iota
	tabSites
	tabUptime
	tabServices
	tabRuntime // Docker & PM2
)

// runtimeEntry menyatukan item Docker container dan proses PM2 dalam satu daftar interaktif
type runtimeEntry struct {
	Kind    string // "docker" atau "pm2"
	ID      string
	Name    string
	Sub     string // image (Docker) atau user (PM2)
	Status  string
	Extra   string // port atau CPU/RAM/restarts
	Running bool
	User    string
}

type pendingAction struct {
	Kind   string // "service", "docker", "pm2"
	Target string
	Label  string
	User   string
}

type metricTickMsg struct {
	point metrics.Point
	at    time.Time
}

type snapshotLoadedMsg struct {
	sites      []store.Site
	uptime     []store.UptimeMonitor
	mcpCount   int
	services   []services.ServiceInfo
	svcCounts  services.ServiceCounts
	runtimes   []runtimeEntry
	dockerUp   int
	pm2Up      int
	fetchedAt  time.Time
	errSummary string
}

type actionResultMsg struct {
	ok      bool
	message string
}

type logOverlayMsg struct {
	title   string
	content string
	err     error
}

// Model mengimplementasikan tea.Model (Bubble Tea Elm Architecture) untuk FikPanel TUI
type Model struct {
	cfg        Config
	store      *store.Store
	collector  metrics.Collector
	svcMgr     *services.Manager
	pm2Mgr     *node.Manager
	hostname   string
	osLabel    string
	width      int
	height     int
	activeTab  tabID
	cursor     int
	filterMode bool
	filterText string

	// Metrik live & riwayat sparkline (maks 40 titik)
	latestMetric metrics.Point
	cpuHist      []float64
	ramHist      []float64
	lastRefresh  time.Time
	tickCount    int

	// Data snapshot infrastruktur
	sites     []store.Site
	uptime    []store.UptimeMonitor
	mcpCount  int
	services  []services.ServiceInfo
	svcCounts services.ServiceCounts
	runtimes  []runtimeEntry
	dockerUp  int
	pm2Up     int
	loading   bool

	// State aksi interaktif, konfirmasi aman, dan log viewer overlay
	confirm     *pendingAction
	statusMsg   string
	statusIsErr bool
	logOpen     bool
	logTitle    string
	logLines    []string
	logScroll   int
}

// NewModel membangun instance Model TUI siap pakai (juga dipakai langsung oleh unit test)
func NewModel(cfg Config, st *store.Store) Model {
	if cfg.RefreshInterval < time.Second {
		cfg.RefreshInterval = 2 * time.Second
	}
	if cfg.Version == "" {
		cfg.Version = "0.7.2"
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "localhost"
	}
	var svcMgr *services.Manager
	if st != nil {
		svcMgr = services.NewManager(st.DB())
	}
	return Model{
		cfg:       cfg,
		store:     st,
		collector: metrics.NewCollector(),
		svcMgr:    svcMgr,
		pm2Mgr:    node.NewManager(),
		hostname:  host,
		osLabel:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		width:     110,
		height:    32,
		activeTab: tabOverview,
		loading:   true,
	}
}

// Run membuka database SQLite FikPanel dan meluncurkan antarmuka terminal interaktif layar penuh
func Run(cfg Config) error {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("gagal membuka database %s: %w", cfg.DBPath, err)
	}
	defer st.Close()

	m := NewModel(cfg, st)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.collectMetricCmd(),
		m.loadSnapshotCmd(),
	)
}

func (m Model) scheduleMetricTick() tea.Cmd {
	return tea.Tick(m.cfg.RefreshInterval, func(t time.Time) tea.Msg {
		pt, _ := m.collector.Collect()
		return metricTickMsg{point: pt, at: t}
	})
}

func (m Model) collectMetricCmd() tea.Cmd {
	return func() tea.Msg {
		pt, _ := m.collector.Collect()
		return metricTickMsg{point: pt, at: time.Now()}
	}
}

func (m Model) loadSnapshotCmd() tea.Cmd {
	st := m.store
	svcMgr := m.svcMgr
	pm2Mgr := m.pm2Mgr
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()

		var msg snapshotLoadedMsg
		msg.fetchedAt = time.Now()

		if st != nil {
			msg.sites, _ = st.ListSites(ctx)
			msg.uptime, _ = st.ListUptimeMonitors(ctx, false)
			if toks, err := st.ListMCPTokens(ctx); err == nil {
				for _, tk := range toks {
					if tk.Enabled {
						msg.mcpCount++
					}
				}
			}
		}

		if svcMgr != nil {
			svcs, counts, err := svcMgr.ListServicesFiltered(ctx, "all", "", "")
			if err == nil {
				// Urutkan layanan gagal (failed) paling atas agar langsung terlihat
				sort.SliceStable(svcs, func(i, j int) bool {
					fi := svcs[i].ActiveState == "failed"
					fj := svcs[j].ActiveState == "failed"
					if fi != fj {
						return fi
					}
					ai := svcs[i].ActiveState == "active"
					aj := svcs[j].ActiveState == "active"
					if ai != aj {
						return ai
					}
					return svcs[i].Unit < svcs[j].Unit
				})
				msg.services = svcs
				msg.svcCounts = counts
			}
		}

		// Kumpulkan kontainer Docker bila daemon tersedia
		if containers, err := docker.GetManager().ListContainers(ctx); err == nil {
			for _, c := range containers {
				isRun := strings.EqualFold(c.State, "running")
				if isRun {
					msg.dockerUp++
				}
				var portParts []string
				for _, p := range c.Ports {
					if p.PublicPort > 0 {
						portParts = append(portParts, fmt.Sprintf("%d→%d", p.PublicPort, p.PrivatePort))
					}
				}
				extra := strings.Join(portParts, ", ")
				if extra == "" {
					extra = c.Status
				}
				msg.runtimes = append(msg.runtimes, runtimeEntry{
					Kind:    "docker",
					ID:      c.ID,
					Name:    strings.TrimPrefix(c.Name, "/"),
					Sub:     c.Image,
					Status:  c.State,
					Extra:   extra,
					Running: isRun,
				})
			}
		}

		// Kumpulkan proses Node.js / PM2 lintas user bila tersedia
		if pm2Mgr != nil {
			if procs, err := pm2Mgr.ListProcesses(ctx); err == nil {
				for _, p := range procs {
					isRun := strings.EqualFold(p.Status, "online")
					if isRun {
						msg.pm2Up++
					}
					msg.runtimes = append(msg.runtimes, runtimeEntry{
						Kind:    "pm2",
						ID:      p.Name,
						Name:    p.Name,
						Sub:     p.User,
						Status:  p.Status,
						Extra:   fmt.Sprintf("CPU %.1f%% · %s · ↻%d", p.CPU, formatBytes(p.Memory), p.RestartCount),
						Running: isRun,
						User:    p.User,
					})
				}
			}
		}

		return msg
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case metricTickMsg:
		m.latestMetric = msg.point
		m.lastRefresh = msg.at
		m.cpuHist = append(m.cpuHist, msg.point.CPU)
		if len(m.cpuHist) > 40 {
			m.cpuHist = m.cpuHist[len(m.cpuHist)-40:]
		}
		ramPct := 0.0
		if msg.point.RAMTotal > 0 {
			ramPct = (float64(msg.point.RAMUsed) / float64(msg.point.RAMTotal)) * 100.0
		}
		m.ramHist = append(m.ramHist, ramPct)
		if len(m.ramHist) > 40 {
			m.ramHist = m.ramHist[len(m.ramHist)-40:]
		}
		m.tickCount++
		cmds := []tea.Cmd{m.scheduleMetricTick()}
		// Segarkan snapshot SQLite/Services/Docker tiap 3 tick (~6 detik)
		if m.tickCount%3 == 0 {
			cmds = append(cmds, m.loadSnapshotCmd())
		}
		return m, tea.Batch(cmds...)

	case snapshotLoadedMsg:
		m.loading = false
		m.sites = msg.sites
		m.uptime = msg.uptime
		m.mcpCount = msg.mcpCount
		m.services = msg.services
		m.svcCounts = msg.svcCounts
		m.runtimes = msg.runtimes
		m.dockerUp = msg.dockerUp
		m.pm2Up = msg.pm2Up
		m.clampCursor()
		return m, nil

	case actionResultMsg:
		m.statusMsg = msg.message
		m.statusIsErr = !msg.ok
		return m, m.loadSnapshotCmd()

	case logOverlayMsg:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusIsErr = true
			return m, nil
		}
		m.logOpen = true
		m.logTitle = msg.title
		clean := strings.ReplaceAll(ai.RedactSecrets(msg.content), "\r\n", "\n")
		m.logLines = strings.Split(strings.TrimSpace(clean), "\n")
		m.logScroll = maxInt(0, len(m.logLines)-(m.height-8))
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// 1. Jika overlay Log sedang terbuka, tangani navigasi log terlebih dahulu
	if m.logOpen {
		switch key {
		case "esc", "q", "l":
			m.logOpen = false
			m.logLines = nil
		case "up", "k":
			if m.logScroll > 0 {
				m.logScroll--
			}
		case "down", "j":
			if m.logScroll < maxInt(0, len(m.logLines)-5) {
				m.logScroll++
			}
		case "pgup":
			m.logScroll = maxInt(0, m.logScroll-10)
		case "pgdown":
			m.logScroll = minInt(maxInt(0, len(m.logLines)-5), m.logScroll+10)
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	// 2. Jika konfirmasi restart aktif, hanya terima y / n / esc
	if m.confirm != nil {
		switch strings.ToLower(key) {
		case "y", "enter":
			act := *m.confirm
			m.confirm = nil
			m.statusMsg = fmt.Sprintf("Menjalankan restart %s…", act.Label)
			m.statusIsErr = false
			return m, m.executePendingActionCmd(act)
		case "n", "esc":
			m.confirm = nil
			m.statusMsg = "Aksi dibatalkan"
			m.statusIsErr = false
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	// 3. Mode ketik pencarian cepat (/)
	if m.filterMode {
		switch key {
		case "esc":
			m.filterMode = false
			m.filterText = ""
			m.cursor = 0
		case "enter":
			m.filterMode = false
			m.clampCursor()
		case "backspace":
			r := []rune(m.filterText)
			if len(r) > 0 {
				m.filterText = string(r[:len(r)-1])
				m.cursor = 0
			}
		case "ctrl+c":
			return m, tea.Quit
		default:
			if len([]rune(key)) == 1 {
				m.filterText += key
				m.cursor = 0
			}
		}
		return m, nil
	}

	// 4. Navigasi & aksi global
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "1":
		m.switchTab(tabOverview)
	case "2":
		m.switchTab(tabSites)
	case "3":
		m.switchTab(tabUptime)
	case "4":
		m.switchTab(tabServices)
	case "5":
		m.switchTab(tabRuntime)
	case "tab", "right":
		m.switchTab(tabID((int(m.activeTab) + 1) % 5))
	case "shift+tab", "left":
		m.switchTab(tabID((int(m.activeTab) + 4) % 5))
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if n := m.activeListLen(); n > 0 && m.cursor < n-1 {
			m.cursor++
		}
	case "/":
		if m.activeTab != tabOverview {
			m.filterMode = true
			m.filterText = ""
			m.cursor = 0
		}
	case "esc":
		m.filterText = ""
		m.statusMsg = ""
		m.clampCursor()
	case "R":
		m.statusMsg = "Menyegarkan data server…"
		m.statusIsErr = false
		return m, tea.Batch(m.collectMetricCmd(), m.loadSnapshotCmd())
	case "r":
		return m.promptRestartSelected()
	case "l":
		return m.openLogSelected()
	case "c":
		if m.activeTab == tabUptime {
			return m.probeSelectedUptime()
		}
	}
	return m, nil
}

func (m *Model) switchTab(t tabID) {
	m.activeTab = t
	m.cursor = 0
	m.filterText = ""
	m.filterMode = false
}

func (m *Model) clampCursor() {
	n := m.activeListLen()
	if n == 0 {
		m.cursor = 0
	} else if m.cursor >= n {
		m.cursor = n - 1
	}
}

func (m Model) filteredSites() []store.Site {
	q := strings.ToLower(strings.TrimSpace(m.filterText))
	if q == "" {
		return m.sites
	}
	out := make([]store.Site, 0)
	for _, s := range m.sites {
		if strings.Contains(strings.ToLower(s.Domain+" "+s.Kind+" "+s.Upstream), q) {
			out = append(out, s)
		}
	}
	return out
}

func (m Model) filteredUptime() []store.UptimeMonitor {
	q := strings.ToLower(strings.TrimSpace(m.filterText))
	if q == "" {
		return m.uptime
	}
	out := make([]store.UptimeMonitor, 0)
	for _, u := range m.uptime {
		if strings.Contains(strings.ToLower(u.Name+" "+u.Target), q) {
			out = append(out, u)
		}
	}
	return out
}

func (m Model) filteredServices() []services.ServiceInfo {
	q := strings.ToLower(strings.TrimSpace(m.filterText))
	if q == "" {
		return m.services
	}
	out := make([]services.ServiceInfo, 0)
	for _, s := range m.services {
		if strings.Contains(strings.ToLower(s.Unit+" "+s.Label+" "+s.Category), q) {
			out = append(out, s)
		}
	}
	return out
}

func (m Model) filteredRuntimes() []runtimeEntry {
	q := strings.ToLower(strings.TrimSpace(m.filterText))
	if q == "" {
		return m.runtimes
	}
	out := make([]runtimeEntry, 0)
	for _, r := range m.runtimes {
		if strings.Contains(strings.ToLower(r.Name+" "+r.Kind+" "+r.Sub), q) {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) activeListLen() int {
	switch m.activeTab {
	case tabSites:
		return len(m.filteredSites())
	case tabUptime:
		return len(m.filteredUptime())
	case tabServices:
		return len(m.filteredServices())
	case tabRuntime:
		return len(m.filteredRuntimes())
	default:
		return 0
	}
}

func (m Model) promptRestartSelected() (tea.Model, tea.Cmd) {
	switch m.activeTab {
	case tabServices:
		list := m.filteredServices()
		if len(list) == 0 || m.cursor >= len(list) {
			return m, nil
		}
		svc := list[m.cursor]
		m.confirm = &pendingAction{Kind: "service", Target: svc.Unit, Label: svc.Unit}
	case tabRuntime:
		list := m.filteredRuntimes()
		if len(list) == 0 || m.cursor >= len(list) {
			return m, nil
		}
		rt := list[m.cursor]
		m.confirm = &pendingAction{Kind: rt.Kind, Target: rt.ID, Label: rt.Kind + ":" + rt.Name, User: rt.User}
	}
	return m, nil
}

func (m Model) executePendingActionCmd(act pendingAction) tea.Cmd {
	svcMgr := m.svcMgr
	pm2Mgr := m.pm2Mgr
	st := m.store
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var err error
		switch act.Kind {
		case "service":
			if svcMgr != nil {
				err = svcMgr.ExecuteAction(ctx, act.Target, "restart")
			}
		case "docker":
			err = docker.GetManager().ControlContainer(ctx, act.Target, "restart")
		case "pm2":
			if pm2Mgr != nil {
				err = pm2Mgr.Action(ctx, act.Target, "restart", act.User)
			}
		}
		if st != nil {
			_ = st.RecordAudit(ctx, "tui", "tui.restart."+act.Kind, act.Target, err == nil, fmt.Sprintf("Restart %s via fikpanel tui", act.Label), nil)
		}
		if err != nil {
			return actionResultMsg{ok: false, message: fmt.Sprintf("Gagal restart %s: %v", act.Label, err)}
		}
		return actionResultMsg{ok: true, message: fmt.Sprintf("Berhasil restart %s", act.Label)}
	}
}

func (m Model) openLogSelected() (tea.Model, tea.Cmd) {
	switch m.activeTab {
	case tabServices:
		list := m.filteredServices()
		if len(list) == 0 || m.cursor >= len(list) {
			return m, nil
		}
		unit := list[m.cursor].Unit
		svcMgr := m.svcMgr
		m.statusMsg = "Memuat journalctl " + unit + "…"
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			logs, err := svcMgr.GetServiceLogs(ctx, unit, 80)
			if err != nil && logs == "" {
				return logOverlayMsg{err: fmt.Errorf("gagal membaca log %s: %w", unit, err)}
			}
			return logOverlayMsg{title: "Journalctl · " + unit, content: logs}
		}
	case tabRuntime:
		list := m.filteredRuntimes()
		if len(list) == 0 || m.cursor >= len(list) {
			return m, nil
		}
		rt := list[m.cursor]
		pm2Mgr := m.pm2Mgr
		m.statusMsg = "Memuat log " + rt.Name + "…"
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var logs string
			var err error
			if rt.Kind == "docker" {
				logs, err = docker.GetManager().GetContainerLogs(ctx, rt.ID, 80)
			} else if pm2Mgr != nil {
				logs, err = pm2Mgr.GetLogs(ctx, rt.ID, 80, rt.User)
			}
			if err != nil {
				return logOverlayMsg{err: fmt.Errorf("gagal membaca log %s: %w", rt.Name, err)}
			}
			return logOverlayMsg{title: strings.ToUpper(rt.Kind) + " Logs · " + rt.Name, content: logs}
		}
	}
	return m, nil
}

func (m Model) probeSelectedUptime() (tea.Model, tea.Cmd) {
	list := m.filteredUptime()
	if len(list) == 0 || m.cursor >= len(list) {
		return m, nil
	}
	mon := list[m.cursor]
	st := m.store
	m.statusMsg = fmt.Sprintf("Memeriksa %s (%s)…", mon.Name, mon.Target)
	m.statusIsErr = false

	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		start := time.Now()
		status := "down"
		code := 0
		sslDays := -1
		sslIssuer := ""
		errMsg := ""

		if strings.EqualFold(mon.MonitorType, "tcp") {
			conn, err := (&net.Dialer{Timeout: 8 * time.Second}).DialContext(ctx, "tcp", mon.Target)
			if err != nil {
				errMsg = err.Error()
			} else {
				_ = conn.Close()
				status = "up"
				code = 1
			}
		} else {
			client := &http.Client{
				Timeout: 10 * time.Second,
				Transport: &http.Transport{
					TLSClientConfig:   &tls.Config{InsecureSkipVerify: false},
					DisableKeepAlives: true,
				},
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, mon.Target, nil)
			if err != nil {
				errMsg = err.Error()
			} else {
				req.Header.Set("User-Agent", "FikPanel-TUI/"+m.cfg.Version)
				resp, err := client.Do(req)
				if err != nil {
					errMsg = err.Error()
				} else {
					defer resp.Body.Close()
					_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
					code = resp.StatusCode
					if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
						leaf := resp.TLS.PeerCertificates[0]
						sslDays = int(time.Until(leaf.NotAfter).Hours() / 24)
						sslIssuer = leaf.Issuer.CommonName
					}
					if (mon.ExpectedStatus > 0 && code == mon.ExpectedStatus) || (mon.ExpectedStatus == 0 && code >= 200 && code < 400) {
						status = "up"
					} else {
						errMsg = fmt.Sprintf("HTTP %d", code)
					}
				}
			}
		}

		lat := time.Since(start).Milliseconds()
		if st != nil {
			_ = st.RecordUptimeCheck(ctx, mon.ID, status, code, lat, sslDays, sslIssuer, errMsg, 0)
		}
		if status == "up" {
			return actionResultMsg{ok: true, message: fmt.Sprintf("%s ONLINE (%d ms, HTTP %d)", mon.Name, lat, code)}
		}
		return actionResultMsg{ok: false, message: fmt.Sprintf("%s OFFLINE (%s)", mon.Name, errMsg)}
	}
}

// ═══ Render Tampilan Terminal (View) ═══

func (m Model) View() string {
	w := maxInt(76, m.width)

	// Header baris atas
	brand := stBrand.Render("◆ FIKPANEL TUI v" + m.cfg.Version)
	clock := time.Now().Format("15:04:05")
	meta := stHeaderMeta.Render(fmt.Sprintf("  %s · %s · %s", m.hostname, m.osLabel, clock))
	header := brand + meta

	// Bar Tab navigasi
	tabs := []struct {
		id    tabID
		label string
	}{
		{tabOverview, "1 Overview"},
		{tabSites, fmt.Sprintf("2 Sites (%d)", len(m.sites))},
		{tabUptime, fmt.Sprintf("3 Uptime (%d)", len(m.uptime))},
		{tabServices, fmt.Sprintf("4 Services (%d)", len(m.services))},
		{tabRuntime, fmt.Sprintf("5 Docker/PM2 (%d)", len(m.runtimes))},
	}
	tabParts := make([]string, 0, len(tabs))
	for _, tb := range tabs {
		if tb.id == m.activeTab {
			tabParts = append(tabParts, stTabActive.Render(tb.label))
		} else {
			tabParts = append(tabParts, stTabInactive.Render(tb.label))
		}
	}
	tabBar := strings.Join(tabParts, " ")

	var body string
	if m.logOpen {
		body = m.viewLogOverlay(w)
	} else {
		switch m.activeTab {
		case tabOverview:
			body = m.viewOverview(w)
		case tabSites:
			body = m.viewSites(w)
		case tabUptime:
			body = m.viewUptime(w)
		case tabServices:
			body = m.viewServices(w)
		case tabRuntime:
			body = m.viewRuntime(w)
		}
	}

	// Bar konfirmasi / pencarian / status
	var statusBar string
	if m.confirm != nil {
		statusBar = stBannerConfirm.Render(fmt.Sprintf("⚠ Konfirmasi restart %s? Tekan [y] Ya / [n] Batal", m.confirm.Label))
	} else if m.filterMode {
		statusBar = stBannerStatus.Render(fmt.Sprintf("Cari (/): %s▌   (Enter selesai · Esc batal)", m.filterText))
	} else if m.statusMsg != "" {
		if m.statusIsErr {
			statusBar = stDanger.Render("✖ " + m.statusMsg)
		} else {
			statusBar = stSuccess.Render("✓ " + m.statusMsg)
		}
	}

	footer := m.viewFooter()

	parts := []string{header, tabBar, body}
	if statusBar != "" {
		parts = append(parts, statusBar)
	}
	parts = append(parts, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) viewOverview(w int) string {
	pt := m.latestMetric
	ramPct := 0.0
	if pt.RAMTotal > 0 {
		ramPct = (float64(pt.RAMUsed) / float64(pt.RAMTotal)) * 100.0
	}
	diskPct := 0.0
	if pt.DiskTotal > 0 {
		diskPct = (float64(pt.DiskUsed) / float64(pt.DiskTotal)) * 100.0
	}

	cardW := maxInt(32, (w-6)/2)
	barW := maxInt(14, cardW-14)

	cpuCard := stCard.Width(cardW).Render(strings.Join([]string{
		stTitle.Render("CPU & Load Average"),
		fmt.Sprintf("%s  %s", renderBar(pt.CPU, barW), stInfo.Render(fmt.Sprintf("%5.1f%%", pt.CPU))),
		stMuted.Render(fmt.Sprintf("Load 1m: %.2f   ·   Sparkline:", pt.LoadAvg)),
		renderSparkline(m.cpuHist, maxInt(16, cardW-4), colInfo),
	}, "\n"))

	ramCard := stCard.Width(cardW).Render(strings.Join([]string{
		stTitle.Render("Memory (RAM)"),
		fmt.Sprintf("%s  %s", renderBar(ramPct, barW), stViolet.Render(fmt.Sprintf("%5.1f%%", ramPct))),
		stMuted.Render(fmt.Sprintf("%s / %s", formatBytes(pt.RAMUsed), formatBytes(pt.RAMTotal))),
		renderSparkline(m.ramHist, maxInt(16, cardW-4), colViolet),
	}, "\n"))

	diskCard := stCard.Width(cardW).Render(strings.Join([]string{
		stTitle.Render("Disk Storage (/)"),
		fmt.Sprintf("%s  %s", renderBar(diskPct, barW), stAccent.Render(fmt.Sprintf("%5.1f%%", diskPct))),
		stMuted.Render(fmt.Sprintf("Terpakai: %s / %s", formatBytes(pt.DiskUsed), formatBytes(pt.DiskTotal))),
	}, "\n"))

	netCard := stCard.Width(cardW).Render(strings.Join([]string{
		stTitle.Render("Network I/O Realtime"),
		fmt.Sprintf("Inbound : %s", stSuccess.Render(formatBytes(pt.NetIn)+"/s")),
		fmt.Sprintf("Outbound: %s", stWarning.Render(formatBytes(pt.NetOut)+"/s")),
	}, "\n"))

	// Ringkasan infrastruktur
	activeSites := 0
	for _, s := range m.sites {
		if s.Enabled == 1 {
			activeSites++
		}
	}
	upMons, downMons, sslWarn := 0, 0, 0
	for _, u := range m.uptime {
		if !u.Enabled {
			continue
		}
		if u.LastStatus == "up" {
			upMons++
		} else if u.LastStatus == "down" {
			downMons++
		}
		if u.SSLDaysRemaining >= 0 && u.SSLDaysRemaining <= 14 {
			sslWarn++
		}
	}
	downTxt := stSuccess.Render("0 down")
	if downMons > 0 {
		downTxt = stDanger.Render(fmt.Sprintf("%d DOWN", downMons))
	}
	sslTxt := stSuccess.Render("SSL aman")
	if sslWarn > 0 {
		sslTxt = stWarning.Render(fmt.Sprintf("%d SSL ≤14h", sslWarn))
	}

	infraLines := []string{
		stTitle.Render("Ringkasan Infrastruktur"),
		fmt.Sprintf("  Websites   : %s aktif / %d total      Services : %s running / %d failed",
			stInfo.Render(fmt.Sprintf("%d", activeSites)), len(m.sites),
			stSuccess.Render(fmt.Sprintf("%d", m.svcCounts.Running)), m.svcCounts.Failed),
		fmt.Sprintf("  Uptime/SSL : %s online · %s · %s",
			stSuccess.Render(fmt.Sprintf("%d", upMons)), downTxt, sslTxt),
		fmt.Sprintf("  Runtime    : Docker %s up · PM2 %s online · MCP Tokens %s aktif",
			stInfo.Render(fmt.Sprintf("%d", m.dockerUp)),
			stViolet.Render(fmt.Sprintf("%d", m.pm2Up)),
			stAccent.Render(fmt.Sprintf("%d", m.mcpCount))),
		stSubtle.Render("  Database   : " + m.cfg.DBPath),
	}
	infraCard := stCard.Width(maxInt(66, cardW*2+2)).Render(strings.Join(infraLines, "\n"))

	row1 := lipgloss.JoinHorizontal(lipgloss.Top, cpuCard, "  ", ramCard)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, diskCard, "  ", netCard)
	return lipgloss.JoinVertical(lipgloss.Left, row1, row2, infraCard)
}

func (m Model) tableWindow(total int) (int, int) {
	maxRows := maxInt(6, m.height-11)
	if total <= maxRows {
		return 0, total
	}
	start := m.cursor - maxRows/2
	if start < 0 {
		start = 0
	}
	end := start + maxRows
	if end > total {
		end = total
		start = maxInt(0, end-maxRows)
	}
	return start, end
}

func (m Model) viewSites(w int) string {
	list := m.filteredSites()
	boxW := maxInt(72, w-2)
	lines := []string{
		stTableHeader.Render(fmt.Sprintf("  %-28s %-8s %-11s %-9s %s", "DOMAIN", "RUNTIME", "SSL", "STATUS", "TARGET / ROOT")),
	}
	if len(list) == 0 {
		lines = append(lines, stMuted.Render("  Tidak ada website terdaftar atau tidak cocok dengan pencarian."))
	} else {
		start, end := m.tableWindow(len(list))
		for i := start; i < end; i++ {
			s := list[i]
			status := stSuccess.Render("AKTIF   ")
			if s.Enabled != 1 {
				status = stWarning.Render("NONAKTIF")
			}
			target := s.RootPath
			if s.Upstream != "" {
				target = s.Upstream
			}
			ssl := s.SSLMode
			if ssl == "" {
				ssl = "none"
			}
			row := fmt.Sprintf("  %s %s %s %s %s",
				padRight(s.Domain, 28),
				padRight(strings.ToUpper(s.Kind), 8),
				padRight(ssl, 11),
				status,
				truncate(target, maxInt(16, boxW-66)),
			)
			if i == m.cursor {
				lines = append(lines, stRowSelected.Render("▸"+row[1:]))
			} else {
				lines = append(lines, row)
			}
		}
	}
	return stCard.Width(boxW).Render(strings.Join(lines, "\n"))
}

func (m Model) viewUptime(w int) string {
	list := m.filteredUptime()
	boxW := maxInt(72, w-2)
	lines := []string{
		stTableHeader.Render(fmt.Sprintf("  %-22s %-8s %-20s %-8s %-8s %s", "MONITOR", "STATUS", "HEARTBEAT (20)", "UPTIME", "LATENSI", "SSL")),
	}
	if len(list) == 0 {
		lines = append(lines, stMuted.Render("  Belum ada target Uptime Monitor. Tambahkan melalui menu Monitor → Uptime & SSL."))
	} else {
		start, end := m.tableWindow(len(list))
		for i := start; i < end; i++ {
			u := list[i]
			stTxt := stMuted.Render("PENDING ")
			if !u.Enabled {
				stTxt = stWarning.Render("DIJEDA  ")
			} else if u.LastStatus == "up" {
				stTxt = stSuccess.Render("ONLINE  ")
			} else if u.LastStatus == "down" {
				stTxt = stDanger.Render("OFFLINE ")
			}
			sslTxt := stSubtle.Render("-")
			if u.SSLDaysRemaining >= 0 {
				if u.SSLDaysRemaining <= 3 {
					sslTxt = stDanger.Render(fmt.Sprintf("%dd", u.SSLDaysRemaining))
				} else if u.SSLDaysRemaining <= 14 {
					sslTxt = stWarning.Render(fmt.Sprintf("%dd", u.SSLDaysRemaining))
				} else {
					sslTxt = stSuccess.Render(fmt.Sprintf("%dd", u.SSLDaysRemaining))
				}
			}
			row := fmt.Sprintf("  %s %s %s %s %s %s",
				padRight(u.Name, 22),
				stTxt,
				renderHeartbeats(u.RecentChecks, 20),
				padRight(fmt.Sprintf("%.1f%%", u.Uptime24hPct), 8),
				padRight(fmt.Sprintf("%dms", u.LastLatencyMs), 8),
				sslTxt,
			)
			if i == m.cursor {
				lines = append(lines, stRowSelected.Render("▸"+row[1:]))
			} else {
				lines = append(lines, row)
			}
		}
	}
	return stCard.Width(boxW).Render(strings.Join(lines, "\n"))
}

func (m Model) viewServices(w int) string {
	list := m.filteredServices()
	boxW := maxInt(72, w-2)
	lines := []string{
		stTableHeader.Render(fmt.Sprintf("  %-30s %-10s %-10s %-8s %-10s %s", "UNIT", "STATUS", "KATEGORI", "PID", "MEMORI", "UPTIME")),
	}
	if len(list) == 0 {
		lines = append(lines, stMuted.Render("  Tidak ada layanan systemd yang tersedia pada host ini."))
	} else {
		start, end := m.tableWindow(len(list))
		for i := start; i < end; i++ {
			s := list[i]
			stTxt := stMuted.Render(padRight(s.ActiveState, 10))
			if s.ActiveState == "active" {
				stTxt = stSuccess.Render(padRight(s.ActiveState, 10))
			} else if s.ActiveState == "failed" {
				stTxt = stDanger.Render(padRight(s.ActiveState, 10))
			}
			row := fmt.Sprintf("  %s %s %s %-8d %-10s %s",
				padRight(s.Unit, 30),
				stTxt,
				padRight(s.Category, 10),
				s.MainPID,
				formatBytes(s.MemoryCurrent),
				truncate(s.Uptime, maxInt(10, boxW-76)),
			)
			if i == m.cursor {
				lines = append(lines, stRowSelected.Render("▸"+row[1:]))
			} else {
				lines = append(lines, row)
			}
		}
	}
	return stCard.Width(boxW).Render(strings.Join(lines, "\n"))
}

func (m Model) viewRuntime(w int) string {
	list := m.filteredRuntimes()
	boxW := maxInt(72, w-2)
	lines := []string{
		stTableHeader.Render(fmt.Sprintf("  %-8s %-24s %-10s %-20s %s", "TIPE", "NAMA", "STATUS", "IMAGE / USER", "DETAIL / PORTS")),
	}
	if len(list) == 0 {
		lines = append(lines, stMuted.Render("  Tidak ada kontainer Docker maupun proses PM2 terdeteksi pada host ini."))
	} else {
		start, end := m.tableWindow(len(list))
		for i := start; i < end; i++ {
			r := list[i]
			typeBadge := stInfo.Render(padRight("DOCKER", 8))
			if r.Kind == "pm2" {
				typeBadge = stViolet.Render(padRight("PM2", 8))
			}
			stTxt := stWarning.Render(padRight(r.Status, 10))
			if r.Running {
				stTxt = stSuccess.Render(padRight(r.Status, 10))
			}
			row := fmt.Sprintf("  %s %s %s %s %s",
				typeBadge,
				padRight(r.Name, 24),
				stTxt,
				padRight(r.Sub, 20),
				truncate(r.Extra, maxInt(14, boxW-68)),
			)
			if i == m.cursor {
				lines = append(lines, stRowSelected.Render("▸"+row[1:]))
			} else {
				lines = append(lines, row)
			}
		}
	}
	return stCard.Width(boxW).Render(strings.Join(lines, "\n"))
}

func (m Model) viewLogOverlay(w int) string {
	boxW := maxInt(72, w-2)
	maxRows := maxInt(8, m.height-9)
	start := m.logScroll
	if start > len(m.logLines) {
		start = maxInt(0, len(m.logLines)-1)
	}
	end := minInt(len(m.logLines), start+maxRows)

	lines := []string{
		stAccent.Render("▤ "+m.logTitle) + stSubtle.Render(fmt.Sprintf("  (baris %d–%d dari %d · ↑/↓ scroll · Esc tutup)", start+1, end, len(m.logLines))),
		"",
	}
	for i := start; i < end; i++ {
		lines = append(lines, truncate(m.logLines[i], boxW-4))
	}
	return stCardActive.Width(boxW).Render(strings.Join(lines, "\n"))
}

func (m Model) viewFooter() string {
	hints := []string{
		stKeyHint.Render("1-5") + " Tab",
		stKeyHint.Render("↑/↓") + " Pilih",
	}
	if m.activeTab != tabOverview {
		hints = append(hints, stKeyHint.Render("/")+" Cari")
	}
	switch m.activeTab {
	case tabUptime:
		hints = append(hints, stKeyHint.Render("c")+" Cek Sekarang")
	case tabServices, tabRuntime:
		hints = append(hints, stKeyHint.Render("r")+" Restart", stKeyHint.Render("l")+" Log")
	}
	hints = append(hints, stKeyHint.Render("R")+" Segarkan", stKeyHint.Render("q")+" Keluar")
	return stFooter.Render(strings.Join(hints, "  ·  "))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
