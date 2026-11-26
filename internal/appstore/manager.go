package appstore

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// Batas memori log per task agar VPS RAM kecil tetap aman
	maxTaskLogBytes = 256 * 1024
	catalogCacheTTL = 10 * time.Second
	upgradeCacheTTL = 60 * time.Second
)

// Pola validasi ketat nama paket Debian untuk mencegah injeksi flag/argumen ke apt-get
var (
	reDebianPkgName = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]{0,127}$`)
	reVersionNum    = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)
	reAptUpgradable = regexp.MustCompile(`^([^/\s]+)/\S+\s+(\S+)\s+(\S+)\s+\[upgradable from:\s+([^\]]+)\]`)
)

// IsValidDebianPackage memverifikasi nama paket agar aman diteruskan ke apt-get / apt-cache
func IsValidDebianPackage(name string) bool {
	return reDebianPkgName.MatchString(strings.TrimSpace(name))
}

// Manager mengelola katalog paket server, bundel 1-klik, inspeksi APT, kontrol layanan, dan antrean instalasi
type Manager struct {
	mu    sync.RWMutex
	aptMu sync.Mutex // Mengantrekan semua operasi tulis APT agar bebas bentrok dpkg/lock-frontend
	tasks map[string]*InstallTask

	cacheMu        sync.RWMutex
	cachedCatalog  []AppPackage
	catalogAt      time.Time
	cachedUpgrades map[string]UpgradablePackage
	upgradesAt     time.Time
	lastAptUpdate  time.Time
}

// NewManager membuat instance manajer App Store baru
func NewManager() *Manager {
	return &Manager{
		tasks:          make(map[string]*InstallTask),
		cachedUpgrades: make(map[string]UpgradablePackage),
	}
}

// InvalidateCache menghapus cache status katalog dan daftar update agar pemindaian berikutnya membaca kondisi terbaru
func (m *Manager) InvalidateCache() {
	m.cacheMu.Lock()
	m.cachedCatalog = nil
	m.catalogAt = time.Time{}
	m.upgradesAt = time.Time{}
	m.cacheMu.Unlock()
}

// Catalog mengembalikan seluruh katalog aplikasi dengan versi nyata, status daemon, ukuran disk, dan status pembaruan APT
func (m *Manager) Catalog(ctx context.Context) []AppPackage {
	return m.CatalogWithRefresh(ctx, false)
}

// CatalogWithRefresh memuat katalog dari cache singkat atau memindai ulang host secara paralel bila forceRefresh=true
func (m *Manager) CatalogWithRefresh(ctx context.Context, forceRefresh bool) []AppPackage {
	if !forceRefresh {
		m.cacheMu.RLock()
		if len(m.cachedCatalog) > 0 && time.Since(m.catalogAt) < catalogCacheTTL {
			out := make([]AppPackage, len(m.cachedCatalog))
			copy(out, m.cachedCatalog)
			m.cacheMu.RUnlock()
			return out
		}
		m.cacheMu.RUnlock()
	}

	apps := builtinCatalogDefs()

	if runtime.GOOS != "linux" {
		// Data simulasi realistis untuk pengembangan pada Windows/macOS
		simInstalled := map[string]struct {
			ver       string
			candidate string
			running   bool
			sizeKB    int64
		}{
			"nginx":   {"1.24.0", "1.26.2", true, 1840},
			"php8.3":  {"8.3.14", "", true, 12450},
			"mariadb": {"10.11.8", "", true, 68200},
			"nodejs":  {"22.11.0", "", false, 44100},
			"docker":  {"26.1.4", "27.3.1", true, 145000},
			"ufw":     {"0.36.2", "", true, 860},
			"git":     {"2.43.0", "", false, 18200},
		}
		for i := range apps {
			if sim, ok := simInstalled[apps[i].ID]; ok {
				apps[i].Installed = true
				apps[i].InstalledVersion = sim.ver
				apps[i].Version = sim.ver
				apps[i].DiskSizeKB = sim.sizeKB
				if sim.candidate != "" {
					apps[i].CandidateVersion = sim.candidate
					apps[i].UpdateAvailable = true
				}
				if sim.running {
					apps[i].Status = "running"
				} else {
					apps[i].Status = "stopped"
				}
			} else {
				apps[i].Status = "not_installed"
			}
		}
		m.storeCatalogCache(apps)
		return apps
	}

	// 1. Kumpulkan semua paket dpkg utama dalam 1 kali pemanggilan batch (menghindari N+1 subprocess)
	primaryPkgs := make([]string, 0, len(apps))
	for _, a := range apps {
		if a.PrimaryAptPkg != "" {
			primaryPkgs = append(primaryPkgs, a.PrimaryAptPkg)
		}
	}
	dpkgMap := batchDpkgQuery(ctx, primaryPkgs)
	upgradeMap := m.getUpgradableMap(ctx, forceRefresh)

	// 2. Kumpulkan seluruh unit systemd untuk dicek secara paralel ringan
	serviceActive := batchSystemctlActive(ctx, apps)

	// 3. Evaluasi tiap aplikasi secara paralel dengan batas waktu singkat
	var wg sync.WaitGroup
	for i := range apps {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			app := &apps[idx]

			// Cek status dari batch dpkg terlebih dahulu
			if info, ok := dpkgMap[app.PrimaryAptPkg]; ok && info.installed {
				app.Installed = true
				app.InstalledVersion = cleanDebianVersion(info.version)
				app.DiskSizeKB = info.sizeKB
			}

			// Fallback deteksi biner/socket (penting untuk PHP FPM, Node via NVM, Go di /usr/local/go/bin, Composer manual)
			if !app.Installed {
				app.Installed = detectInstalledByBinaryOrPath(app)
			}

			// Cek versi biner aktual bila dpkg tidak mencatatnya (instalasi non-APT)
			if app.Installed && app.InstalledVersion == "" {
				if v := detectBinaryVersion(ctx, app); v != "" {
					app.InstalledVersion = v
				}
			}
			if app.InstalledVersion != "" {
				app.Version = app.InstalledVersion
			}

			// Cek apakah salah satu paket APT milik aplikasi memiliki update di repositori
			if app.Installed {
				for _, pName := range app.AptPackages {
					if up, hasUp := upgradeMap[pName]; hasUp {
						app.UpdateAvailable = true
						app.CandidateVersion = cleanDebianVersion(up.CandidateVersion)
						break
					}
				}
			}

			// Cek status layanan daemon
			running := false
			if app.Installed {
				if app.ServiceUnit != "" && serviceActive[app.ServiceUnit] {
					running = true
				}
				if !running {
					for _, alt := range app.AltServices {
						if serviceActive[alt] {
							running = true
							break
						}
					}
				}
			}

			if running {
				app.Status = "running"
			} else if app.Installed {
				app.Status = "stopped"
			} else {
				app.Status = "not_installed"
			}
		}(i)
	}
	wg.Wait()

	m.storeCatalogCache(apps)
	return apps
}

func (m *Manager) storeCatalogCache(apps []AppPackage) {
	cp := make([]AppPackage, len(apps))
	copy(cp, apps)
	m.cacheMu.Lock()
	m.cachedCatalog = cp
	m.catalogAt = time.Now()
	m.cacheMu.Unlock()
}

// Bundles mengembalikan daftar Stack Bundles 1-klik yang sudah dikorelasikan dengan status aplikasi terpasang
func (m *Manager) Bundles(ctx context.Context) []StackBundle {
	catalog := m.Catalog(ctx)
	byID := make(map[string]AppPackage, len(catalog))
	for _, a := range catalog {
		byID[a.ID] = a
	}

	bundles := builtinStackBundleDefs()
	for i := range bundles {
		b := &bundles[i]
		b.TotalCount = len(b.AppIDs)
		b.Components = make([]StackBundleComponent, 0, len(b.AppIDs))
		b.MissingAppIDs = make([]string, 0)
		for _, id := range b.AppIDs {
			app, ok := byID[id]
			if !ok {
				continue
			}
			b.Components = append(b.Components, StackBundleComponent{
				ID:        app.ID,
				Name:      app.Name,
				Installed: app.Installed,
				Status:    app.Status,
				Version:   app.Version,
			})
			if app.Installed {
				b.InstalledCount++
			} else {
				b.MissingAppIDs = append(b.MissingAppIDs, app.ID)
			}
		}
		b.Ready = len(b.MissingAppIDs) == 0 && b.TotalCount > 0
	}
	return bundles
}

// ListUpgradable mengembalikan seluruh paket APT yang memiliki pembaruan versi pada server
func (m *Manager) ListUpgradable(ctx context.Context, forceRefresh bool) []UpgradablePackage {
	upMap := m.getUpgradableMap(ctx, forceRefresh)
	out := make([]UpgradablePackage, 0, len(upMap))
	for _, v := range upMap {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (m *Manager) getUpgradableMap(ctx context.Context, forceRefresh bool) map[string]UpgradablePackage {
	if !forceRefresh {
		m.cacheMu.RLock()
		if time.Since(m.upgradesAt) < upgradeCacheTTL && m.cachedUpgrades != nil {
			cp := make(map[string]UpgradablePackage, len(m.cachedUpgrades))
			for k, v := range m.cachedUpgrades {
				cp[k] = v
			}
			m.cacheMu.RUnlock()
			return cp
		}
		m.cacheMu.RUnlock()
	}

	res := make(map[string]UpgradablePackage)
	if runtime.GOOS != "linux" {
		res["nginx"] = UpgradablePackage{Name: "nginx", CurrentVersion: "1.24.0-2ubuntu7", CandidateVersion: "1.26.2-1ubuntu1", Architecture: "arm64"}
		res["docker.io"] = UpgradablePackage{Name: "docker.io", CurrentVersion: "26.1.4-0ubuntu1", CandidateVersion: "27.3.1-0ubuntu1", Architecture: "arm64"}
	} else {
		cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(cmdCtx, "apt", "list", "--upgradable").Output()
		if err == nil {
			res = ParseAptUpgradableOutput(string(out))
		}
	}

	m.cacheMu.Lock()
	m.cachedUpgrades = res
	m.upgradesAt = time.Now()
	m.cacheMu.Unlock()

	cp := make(map[string]UpgradablePackage, len(res))
	for k, v := range res {
		cp[k] = v
	}
	return cp
}

// ParseAptUpgradableOutput mem-parsing keluaran standar `apt list --upgradable`
func ParseAptUpgradableOutput(raw string) map[string]UpgradablePackage {
	res := make(map[string]UpgradablePackage)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Listing") || strings.HasPrefix(line, "WARNING") {
			continue
		}
		m := reAptUpgradable.FindStringSubmatch(line)
		if len(m) == 5 {
			pkg := strings.TrimSpace(m[1])
			res[pkg] = UpgradablePackage{
				Name:             pkg,
				CandidateVersion: strings.TrimSpace(m[2]),
				Architecture:     strings.TrimSpace(m[3]),
				CurrentVersion:   strings.TrimSpace(m[4]),
			}
		}
	}
	return res
}

type dpkgPkgInfo struct {
	installed bool
	version   string
	sizeKB    int64
}

// batchDpkgQuery mengeksekusi 1 panggilan dpkg-query untuk banyak paket sekaligus (menghilangkan N+1)
func batchDpkgQuery(ctx context.Context, pkgs []string) map[string]dpkgPkgInfo {
	out := make(map[string]dpkgPkgInfo, len(pkgs))
	if len(pkgs) == 0 || runtime.GOOS != "linux" {
		return out
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	args := append([]string{"-W", "-f=${Package}\t${Status}\t${Version}\t${Installed-Size}\n"}, pkgs...)
	raw, _ := exec.CommandContext(cmdCtx, "dpkg-query", args...).Output()
	return parseBatchDpkgOutput(string(raw))
}

// parseBatchDpkgOutput mem-parsing baris keluaran tab-separated dpkg-query
func parseBatchDpkgOutput(raw string) map[string]dpkgPkgInfo {
	out := make(map[string]dpkgPkgInfo)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) < 3 {
			continue
		}
		pkg := strings.TrimSpace(parts[0])
		status := strings.TrimSpace(parts[1])
		ver := strings.TrimSpace(parts[2])
		var sizeKB int64
		if len(parts) >= 4 {
			sizeKB, _ = strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
		}
		if strings.Contains(status, "ok installed") {
			out[pkg] = dpkgPkgInfo{installed: true, version: ver, sizeKB: sizeKB}
		}
	}
	return out
}

// batchSystemctlActive memeriksa keaktifan seluruh unit systemd katalog dalam satu perintah
func batchSystemctlActive(ctx context.Context, apps []AppPackage) map[string]bool {
	active := make(map[string]bool)
	if runtime.GOOS != "linux" {
		return active
	}
	units := make([]string, 0, len(apps))
	seen := make(map[string]bool)
	addUnit := func(u string) {
		if u != "" && !seen[u] {
			seen[u] = true
			units = append(units, u)
		}
	}
	for _, a := range apps {
		addUnit(a.ServiceUnit)
		for _, alt := range a.AltServices {
			addUnit(alt)
		}
	}
	if len(units) == 0 {
		return active
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	args := append([]string{"is-active"}, units...)
	raw, _ := exec.CommandContext(cmdCtx, "systemctl", args...).Output()
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for i, u := range units {
		if i < len(lines) && strings.TrimSpace(lines[i]) == "active" {
			active[u] = true
		}
	}
	return active
}

// detectInstalledByBinaryOrPath mendeteksi software yang terpasang di luar APT (NVM, /usr/local/go/bin, socket PHP-FPM)
func detectInstalledByBinaryOrPath(app *AppPackage) bool {
	for _, bin := range app.BinaryNames {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
		// Cek jalur umum non-PATH (misal Go atau binary lokal)
		for _, extraDir := range []string{"/usr/local/bin", "/usr/local/go/bin", "/usr/sbin", "/sbin"} {
			if fi, err := os.Stat(extraDir + "/" + bin); err == nil && !fi.IsDir() {
				return true
			}
		}
	}
	if strings.HasPrefix(app.ID, "php") {
		ver := strings.TrimPrefix(app.ID, "php")
		if _, err := os.Stat(fmt.Sprintf("/run/php/php%s-fpm.sock", ver)); err == nil {
			return true
		}
		if _, err := os.Stat(fmt.Sprintf("/etc/php/%s", ver)); err == nil {
			return true
		}
	}
	return false
}

// detectBinaryVersion membaca versi aktual langsung dari biner ketika paket terpasang di luar APT
func detectBinaryVersion(ctx context.Context, app *AppPackage) string {
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch {
	case app.ID == "nginx":
		cmd = exec.CommandContext(cmdCtx, "nginx", "-v")
	case strings.HasPrefix(app.ID, "php"):
		cmd = exec.CommandContext(cmdCtx, "php"+strings.TrimPrefix(app.ID, "php"), "-v")
	case app.ID == "nodejs":
		cmd = exec.CommandContext(cmdCtx, "node", "-v")
	case app.ID == "golang":
		bin := "go"
		if _, err := exec.LookPath(bin); err != nil {
			bin = "/usr/local/go/bin/go"
		}
		cmd = exec.CommandContext(cmdCtx, bin, "version")
	case app.ID == "docker":
		cmd = exec.CommandContext(cmdCtx, "docker", "--version")
	case app.ID == "composer":
		cmd = exec.CommandContext(cmdCtx, "composer", "--version")
	case app.ID == "cloudflared":
		cmd = exec.CommandContext(cmdCtx, "cloudflared", "--version")
	case app.ID == "python3":
		cmd = exec.CommandContext(cmdCtx, "python3", "--version")
	case app.ID == "caddy":
		cmd = exec.CommandContext(cmdCtx, "caddy", "version")
	default:
		return ""
	}
	out, _ := cmd.CombinedOutput()
	if m := reVersionNum.FindStringSubmatch(string(out)); len(m) >= 2 {
		return m[1]
	}
	return ""
}

// cleanDebianVersion menyederhanakan versi Debian (mis. "1:10.11.8-0+deb12u1" -> "10.11.8")
func cleanDebianVersion(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	// Buang epoch Debian seperti "1:" atau "2:"
	if idx := strings.IndexByte(v, ':'); idx >= 0 && idx < 4 {
		v = v[idx+1:]
	}
	// Buang sufiks revisi distro setelah '-' atau '+'
	for _, sep := range []string{"-", "+", "~"} {
		if idx := strings.Index(v, sep); idx > 0 {
			v = v[:idx]
		}
	}
	return v
}

// resolvePackagesForAppID memvalidasi appID dan menerjemahkannya ke daftar paket APT aman
func (m *Manager) resolvePackagesForAppID(appID string) ([]string, string, error) {
	id := strings.TrimSpace(appID)
	if id == "" {
		return nil, "", fmt.Errorf("ID aplikasi tidak boleh kosong")
	}
	if strings.HasPrefix(id, "pkg:") {
		pkg := strings.TrimSpace(strings.TrimPrefix(id, "pkg:"))
		if !IsValidDebianPackage(pkg) {
			return nil, "", fmt.Errorf("nama paket Debian tidak valid: %q", pkg)
		}
		return []string{pkg}, pkg, nil
	}

	for _, a := range builtinCatalogDefs() {
		if a.ID == id {
			return a.AptPackages, a.Name, nil
		}
	}

	// Dukungan dinamis versi PHP lain (mis. php8.1, php8.0, php7.4) — selalu pasang suite FPM, bukan mod_php Apache
	if strings.HasPrefix(id, "php") && len(id) >= 6 {
		ver := strings.TrimPrefix(id, "php")
		if regexp.MustCompile(`^\d+\.\d+$`).MatchString(ver) {
			return []string{
				fmt.Sprintf("php%s-fpm", ver),
				fmt.Sprintf("php%s-mysql", ver),
				fmt.Sprintf("php%s-cli", ver),
				fmt.Sprintf("php%s-curl", ver),
				fmt.Sprintf("php%s-mbstring", ver),
				fmt.Sprintf("php%s-gd", ver),
				fmt.Sprintf("php%s-xml", ver),
				fmt.Sprintf("php%s-zip", ver),
			}, "PHP " + ver + " + FPM Suite", nil
		}
	}

	if !IsValidDebianPackage(id) {
		return nil, "", fmt.Errorf("ID paket tidak dikenali atau tidak valid: %q", id)
	}
	return []string{id}, id, nil
}

// appendTaskLog menambahkan baris log ke InstallTask secara thread-safe dengan batas ukuran aman
func (m *Manager) appendTaskLog(task *InstallTask, chunk string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(task.Logs)+len(chunk) > maxTaskLogBytes {
		keep := maxTaskLogBytes - len(chunk) - 64
		if keep < 0 {
			keep = 0
		}
		if keep < len(task.Logs) {
			task.Logs = "... [log lama dipangkas] ...\n" + task.Logs[len(task.Logs)-keep:]
		}
	}
	task.Logs += chunk
}

// runStreamingCommand mengeksekusi perintah OS dan mengalirkan stdout+stderr baris-per-baris secara live ke task.Logs
func (m *Manager) runStreamingCommand(ctx context.Context, task *InstallTask, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(cmd.Environ(), "DEBIAN_FRONTEND=noninteractive")

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		_ = pr.Close()
		return err
	}

	doneRead := make(chan struct{})
	go func() {
		defer close(doneRead)
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 0, 16*1024), 128*1024)
		for scanner.Scan() {
			m.appendTaskLog(task, scanner.Text()+"\n")
		}
	}()

	waitErr := cmd.Wait()
	_ = pw.Close()
	<-doneRead
	_ = pr.Close()
	return waitErr
}

// startQueuedAptTask mendaftarkan task baru dan mengeksekusi worker di belakang antrean mutex APT
func (m *Manager) startQueuedAptTask(taskID, action, title string, timeout time.Duration, fn func(ctx context.Context, task *InstallTask) error) error {
	m.mu.Lock()
	if existing, ok := m.tasks[taskID]; ok && (existing.Status == "running" || existing.Status == "queued") {
		m.mu.Unlock()
		return fmt.Errorf("operasi %s sedang berjalan atau dalam antrean", title)
	}
	task := &InstallTask{
		AppID:     taskID,
		Action:    action,
		Title:     title,
		Status:    "queued",
		Logs:      fmt.Sprintf("[%s] Menjadwalkan %s...\n", time.Now().Format("15:04:05"), title),
		StartedAt: time.Now().Unix(),
	}
	m.tasks[taskID] = task
	m.mu.Unlock()

	go func() {
		// Tunggu giliran lock APT agar tidak pernah tabrakan dpkg/lock-frontend
		m.aptMu.Lock()
		defer m.aptMu.Unlock()

		m.mu.Lock()
		task.Status = "running"
		task.Logs += fmt.Sprintf("[%s] Memulai eksekusi %s...\n", time.Now().Format("15:04:05"), title)
		m.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		err := fn(ctx, task)

		m.InvalidateCache()
		m.mu.Lock()
		task.FinishedAt = time.Now().Unix()
		if err != nil {
			task.Status = "failed"
			task.Logs += fmt.Sprintf("\n[GAGAL] %v\n", err)
		} else {
			task.Status = "success"
			task.Logs += fmt.Sprintf("\n[SUKSES] %s selesai dengan sukses!\n", title)
		}
		m.mu.Unlock()
	}()

	return nil
}

// ensureAptIndexFresh memperbarui indeks APT otomatis bila belum diperbarui dalam 6 jam terakhir
func (m *Manager) ensureAptIndexFresh(ctx context.Context, task *InstallTask) {
	m.cacheMu.RLock()
	last := m.lastAptUpdate
	m.cacheMu.RUnlock()
	if time.Since(last) < 6*time.Hour {
		if entries, err := os.ReadDir("/var/lib/apt/lists"); err == nil && len(entries) > 3 {
			return
		}
	}
	m.appendTaskLog(task, "$ apt-get update -qq\n")
	if err := m.runStreamingCommand(ctx, task, "apt-get", "update", "-qq"); err == nil {
		m.cacheMu.Lock()
		m.lastAptUpdate = time.Now()
		m.cacheMu.Unlock()
	}
}

// Install memasang paket katalog atau paket Debian (pkg:<name>) dengan streaming log real-time
func (m *Manager) Install(appID string) error {
	pkgs, displayName, err := m.resolvePackagesForAppID(appID)
	if err != nil {
		return err
	}
	return m.startQueuedAptTask(appID, "install", "Instalasi "+displayName, 15*time.Minute, func(ctx context.Context, task *InstallTask) error {
		if runtime.GOOS != "linux" {
			m.appendTaskLog(task, fmt.Sprintf("$ apt-get install -y --no-install-recommends %s\n", strings.Join(pkgs, " ")))
			time.Sleep(1200 * time.Millisecond)
			m.appendTaskLog(task, "[Simulasi] Paket berhasil dipasang di lingkungan pengembangan non-Linux.\n")
			return nil
		}
		m.ensureAptIndexFresh(ctx, task)
		args := append([]string{"install", "-y", "--no-install-recommends"}, pkgs...)
		m.appendTaskLog(task, "$ apt-get "+strings.Join(args, " ")+"\n")
		return m.runStreamingCommand(ctx, task, "apt-get", args...)
	})
}

// Upgrade memperbarui paket katalog atau paket Debian ke versi terbaru repositori APT
func (m *Manager) Upgrade(appID string) error {
	pkgs, displayName, err := m.resolvePackagesForAppID(appID)
	if err != nil {
		return err
	}
	taskID := "upgrade:" + appID
	return m.startQueuedAptTask(taskID, "upgrade", "Upgrade "+displayName, 15*time.Minute, func(ctx context.Context, task *InstallTask) error {
		if runtime.GOOS != "linux" {
			m.appendTaskLog(task, fmt.Sprintf("$ apt-get install -y --only-upgrade %s\n", strings.Join(pkgs, " ")))
			time.Sleep(1200 * time.Millisecond)
			m.appendTaskLog(task, "[Simulasi] Paket berhasil diperbarui ke versi kandidat terbaru.\n")
			return nil
		}
		m.ensureAptIndexFresh(ctx, task)
		args := append([]string{"install", "-y", "--only-upgrade", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold"}, pkgs...)
		m.appendTaskLog(task, "$ apt-get "+strings.Join(args, " ")+"\n")
		return m.runStreamingCommand(ctx, task, "apt-get", args...)
	})
}

// Uninstall mencopot paket software sistem secara aman dengan live log streaming
func (m *Manager) Uninstall(appID string) error {
	pkgs, displayName, err := m.resolvePackagesForAppID(appID)
	if err != nil {
		return err
	}
	return m.startQueuedAptTask(appID, "uninstall", "Pencopotan "+displayName, 10*time.Minute, func(ctx context.Context, task *InstallTask) error {
		if runtime.GOOS != "linux" {
			m.appendTaskLog(task, fmt.Sprintf("$ apt-get remove -y %s\n", strings.Join(pkgs, " ")))
			time.Sleep(1200 * time.Millisecond)
			m.appendTaskLog(task, "[Simulasi] Paket berhasil dicopot di lingkungan non-Linux.\n")
			return nil
		}
		args := append([]string{"remove", "-y"}, pkgs...)
		m.appendTaskLog(task, "$ apt-get "+strings.Join(args, " ")+"\n")
		return m.runStreamingCommand(ctx, task, "apt-get", args...)
	})
}

// AptUpdate menyinkronkan indeks repositori APT Linux secara nyata (`apt-get update`)
func (m *Manager) AptUpdate() error {
	return m.startQueuedAptTask("sys:apt-update", "apt-update", "Sinkronisasi Repositori APT", 5*time.Minute, func(ctx context.Context, task *InstallTask) error {
		if runtime.GOOS != "linux" {
			m.appendTaskLog(task, "$ apt-get update\n")
			time.Sleep(1000 * time.Millisecond)
			m.appendTaskLog(task, "[Simulasi] Indeks repositori APT berhasil diperbarui.\n")
			return nil
		}
		m.appendTaskLog(task, "$ apt-get update\n")
		err := m.runStreamingCommand(ctx, task, "apt-get", "update")
		if err == nil {
			m.cacheMu.Lock()
			m.lastAptUpdate = time.Now()
			m.cacheMu.Unlock()
		}
		return err
	})
}

// InstallBundle memasang seluruh komponen yang belum terpasang dari satu Stack Bundle 1-klik
func (m *Manager) InstallBundle(ctx context.Context, bundleID string) (string, error) {
	bundles := m.Bundles(ctx)
	var target *StackBundle
	for i := range bundles {
		if bundles[i].ID == bundleID {
			target = &bundles[i]
			break
		}
	}
	if target == nil {
		return "", fmt.Errorf("stack bundle tidak ditemukan: %s", bundleID)
	}
	if len(target.MissingAppIDs) == 0 {
		return "", fmt.Errorf("seluruh komponen pada bundel %s sudah terpasang", target.Name)
	}

	// Gabungkan seluruh paket APT unik dari komponen yang belum terpasang
	pkgSet := make(map[string]bool)
	allPkgs := make([]string, 0, len(target.MissingAppIDs)*2)
	for _, appID := range target.MissingAppIDs {
		pkgs, _, err := m.resolvePackagesForAppID(appID)
		if err != nil {
			continue
		}
		for _, p := range pkgs {
			if !pkgSet[p] {
				pkgSet[p] = true
				allPkgs = append(allPkgs, p)
			}
		}
	}
	if len(allPkgs) == 0 {
		return "", fmt.Errorf("tidak ada paket APT yang perlu dipasang untuk bundel %s", target.Name)
	}

	taskID := "bundle:" + bundleID
	err := m.startQueuedAptTask(taskID, "bundle", "Bundel "+target.Name, 20*time.Minute, func(runCtx context.Context, task *InstallTask) error {
		m.appendTaskLog(task, fmt.Sprintf("Komponen yang akan dipasang (%d): %s\n", len(target.MissingAppIDs), strings.Join(target.MissingAppIDs, ", ")))
		if runtime.GOOS != "linux" {
			m.appendTaskLog(task, fmt.Sprintf("$ apt-get install -y --no-install-recommends %s\n", strings.Join(allPkgs, " ")))
			time.Sleep(1500 * time.Millisecond)
			m.appendTaskLog(task, "[Simulasi] Seluruh komponen bundel berhasil dipasang.\n")
			return nil
		}
		m.ensureAptIndexFresh(runCtx, task)
		args := append([]string{"install", "-y", "--no-install-recommends"}, allPkgs...)
		m.appendTaskLog(task, "$ apt-get "+strings.Join(args, " ")+"\n")
		return m.runStreamingCommand(runCtx, task, "apt-get", args...)
	})
	return taskID, err
}

// ControlService mengontrol daemon systemd aplikasi katalog (start / stop / restart) secara langsung
func (m *Manager) ControlService(ctx context.Context, appID, action string) (string, error) {
	act := strings.ToLower(strings.TrimSpace(action))
	if act != "start" && act != "stop" && act != "restart" && act != "reload" {
		return "", fmt.Errorf("aksi layanan tidak valid: %s (gunakan start, stop, restart, atau reload)", action)
	}

	var target *AppPackage
	for _, a := range builtinCatalogDefs() {
		if a.ID == appID {
			cp := a
			target = &cp
			break
		}
	}
	if target == nil || target.ServiceUnit == "" {
		return "", fmt.Errorf("aplikasi %s tidak memiliki layanan daemon systemd yang dapat dikontrol", appID)
	}

	if runtime.GOOS != "linux" {
		m.InvalidateCache()
		return fmt.Sprintf("[Simulasi] Layanan %s berhasil di-%s", target.ServiceUnit, act), nil
	}

	unit := target.ServiceUnit
	// Pilih unit alternatif (mis. mysql bila mariadb tidak ada)
	if len(target.AltServices) > 0 {
		out, _ := exec.CommandContext(ctx, "systemctl", "list-unit-files", unit+".service").Output()
		if !strings.Contains(string(out), unit) {
			unit = target.AltServices[0]
		}
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "systemctl", act, unit).CombinedOutput()
	m.InvalidateCache()
	if err != nil {
		return "", fmt.Errorf("gagal %s layanan %s: %s", act, unit, strings.TrimSpace(string(out)))
	}
	return fmt.Sprintf("Layanan %s berhasil di-%s", unit, act), nil
}

// InspectPackage membaca metadata lengkap paket Debian via apt-cache show & dpkg-query
func (m *Manager) InspectPackage(ctx context.Context, name string) (*PackageDetail, error) {
	pkg := strings.TrimSpace(strings.TrimPrefix(name, "pkg:"))
	// Jika ID berasal dari katalog (mis. "php8.3" atau "golang"), petakan ke PrimaryAptPkg
	for _, a := range builtinCatalogDefs() {
		if a.ID == pkg && a.PrimaryAptPkg != "" {
			pkg = a.PrimaryAptPkg
			break
		}
	}
	if !IsValidDebianPackage(pkg) {
		return nil, fmt.Errorf("nama paket tidak valid: %q", name)
	}

	if runtime.GOOS != "linux" {
		return &PackageDetail{
			Name:             pkg,
			Installed:        pkg == "nginx" || pkg == "mariadb-server" || pkg == "git",
			InstalledVersion: "1.24.0-2ubuntu7",
			CandidateVersion: "1.26.2-1ubuntu1",
			UpdateAvailable:  pkg == "nginx",
			Section:          "web",
			Maintainer:       "Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>",
			Homepage:         "https://packages.ubuntu.com/search?keywords=" + pkg,
			InstalledSizeKB:  1840,
			DownloadSize:     "520 KB",
			Depends:          "libc6 (>= 2.34), libssl3t64, zlib1g",
			Description:      fmt.Sprintf("Informasi simulasi paket %s dari repositori APT Linux.", pkg),
		}, nil
	}

	detail := &PackageDetail{Name: pkg}

	// 1. Cek status terpasang lokal via dpkg-query
	dpkgInfo := batchDpkgQuery(ctx, []string{pkg})
	if info, ok := dpkgInfo[pkg]; ok && info.installed {
		detail.Installed = true
		detail.InstalledVersion = cleanDebianVersion(info.version)
		detail.InstalledSizeKB = info.sizeKB
	}

	// 2. Cek metadata repositori via apt-cache show
	cmdCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	rawShow, err := exec.CommandContext(cmdCtx, "apt-cache", "show", pkg).Output()
	if err == nil && len(rawShow) > 0 {
		parseAptCacheShow(string(rawShow), detail)
	}

	// 3. Cek apakah ada update pada cache upgradable
	upMap := m.getUpgradableMap(ctx, false)
	if up, ok := upMap[pkg]; ok {
		detail.UpdateAvailable = true
		detail.CandidateVersion = cleanDebianVersion(up.CandidateVersion)
	}

	return detail, nil
}

// parseAptCacheShow mengekstrak stanza pertama keluaran `apt-cache show` ke PackageDetail
func parseAptCacheShow(raw string, d *PackageDetail) {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	var descLines []string
	inDesc := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			// Berhenti di stanza pertama (kandidat terbaru)
			if d.CandidateVersion != "" || len(descLines) > 0 {
				break
			}
			continue
		}
		if inDesc && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			txt := strings.TrimSpace(line)
			if txt == "." {
				descLines = append(descLines, "")
			} else {
				descLines = append(descLines, txt)
			}
			continue
		}
		inDesc = false

		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		switch key {
		case "Version":
			d.CandidateVersion = cleanDebianVersion(val)
		case "Section":
			d.Section = val
		case "Maintainer":
			d.Maintainer = val
		case "Homepage":
			d.Homepage = val
		case "Depends":
			d.Depends = val
		case "Installed-Size":
			if d.InstalledSizeKB == 0 {
				d.InstalledSizeKB, _ = strconv.ParseInt(val, 10, 64)
			}
		case "Size":
			if sz, err := strconv.ParseInt(val, 10, 64); err == nil && sz > 0 {
				if sz >= 1024*1024 {
					d.DownloadSize = fmt.Sprintf("%.1f MB", float64(sz)/(1024*1024))
				} else {
					d.DownloadSize = fmt.Sprintf("%d KB", sz/1024)
				}
			}
		case "Description", "Description-en":
			inDesc = true
			descLines = append(descLines, val)
		}
	}
	if len(descLines) > 0 {
		d.Description = strings.Join(descLines, "\n")
	}
}

// SearchGlobalPackages mencari paket software di repositori global OS (APT) dengan batch dpkg-query tunggal
func (m *Manager) SearchGlobalPackages(ctx context.Context, query string) ([]GlobalPackage, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	var safeBuilder strings.Builder
	for _, r := range query {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '+' {
			safeBuilder.WriteRune(r)
		}
	}
	safeQuery := safeBuilder.String()
	if safeQuery == "" {
		return nil, nil
	}

	if runtime.GOOS != "linux" {
		mocks := []GlobalPackage{
			{Name: "htop", Version: "3.3.0", Description: "Interactive processes viewer for Linux", Installed: true},
			{Name: "btop", Version: "", Description: "Modern and colorful resource monitor", Installed: false},
			{Name: "ncdu", Version: "", Description: "NCurses disk usage analyzer", Installed: false},
			{Name: "ffmpeg", Version: "", Description: "Tools for transcoding, streaming and playing multimedia files", Installed: false},
			{Name: "jq", Version: "1.7.1", Description: "Lightweight and flexible command-line JSON processor", Installed: true},
			{Name: "unzip", Version: "6.0", Description: "De-archiver for .zip files", Installed: true},
			{Name: "python3-pip", Version: "", Description: "Python package installer", Installed: false},
		}
		var matched []GlobalPackage
		qLower := strings.ToLower(safeQuery)
		for _, pkg := range mocks {
			if strings.Contains(strings.ToLower(pkg.Name), qLower) || strings.Contains(strings.ToLower(pkg.Description), qLower) {
				matched = append(matched, pkg)
			}
		}
		if len(matched) == 0 && IsValidDebianPackage(strings.ToLower(safeQuery)) {
			matched = append(matched, GlobalPackage{
				Name:        strings.ToLower(safeQuery),
				Description: fmt.Sprintf("Paket sistem Linux %s dari repositori APT", safeQuery),
				Installed:   false,
			})
		}
		return matched, nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "apt-cache", "search", "--names-only", safeQuery)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		cmdFallback := exec.CommandContext(cmdCtx, "apt-cache", "search", safeQuery)
		out, _ = cmdFallback.Output()
	}

	results := make([]GlobalPackage, 0, 30)
	pkgNames := make([]string, 0, 30)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() && len(results) < 30 {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " - ", 2)
		pkgName := strings.TrimSpace(parts[0])
		if !IsValidDebianPackage(pkgName) {
			continue
		}
		desc := ""
		if len(parts) > 1 {
			desc = strings.TrimSpace(parts[1])
		}
		results = append(results, GlobalPackage{
			Name:        pkgName,
			Description: desc,
		})
		pkgNames = append(pkgNames, pkgName)
	}

	// Satu panggilan batch dpkg-query untuk seluruh hasil pencarian (jauh lebih cepat dari 30 subprocess)
	if len(pkgNames) > 0 {
		installedMap := batchDpkgQuery(ctx, pkgNames)
		for i := range results {
			if info, ok := installedMap[results[i].Name]; ok && info.installed {
				results[i].Installed = true
				results[i].Version = cleanDebianVersion(info.version)
			}
		}
	}

	return results, nil
}

// GetTask mengembalikan salinan status dan log satu operasi paket berdasarkan ID
func (m *Manager) GetTask(appID string) *InstallTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[appID]
	if !ok {
		return nil
	}
	cp := *t
	return &cp
}

// ListTasks mengembalikan seluruh antrean & riwayat tugas App Store diurutkan dari yang paling baru
func (m *Manager) ListTasks() []InstallTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]InstallTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt > out[j].StartedAt
	})
	if len(out) > 30 {
		out = out[:30]
	}
	return out
}
