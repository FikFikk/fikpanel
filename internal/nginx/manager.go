package nginx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Batas ukuran file konfigurasi utama yang boleh diedit lewat panel
const maxConfigSize = 2 << 20

// Kunci setting yang disimpan di tabel settings
const (
	settingDaemon = "nginx.daemon"
	settingAlert  = "nginx.alert"
)

// Auditor merekam peristiwa penting ke audit log
type Auditor interface {
	RecordAudit(ctx context.Context, actor, action, target string, ok bool, detail string, userID *int64) error
}

// SettingsStore menyimpan preferensi key-value persisten
type SettingsStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

// ValidationError menandai kesalahan input/konfigurasi (dipetakan ke HTTP 400)
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

// ConflictError menandai operasi yang bentrok dengan proses lain (dipetakan ke HTTP 409)
type ConflictError struct {
	Msg string
}

func (e *ConflictError) Error() string { return e.Msg }

// Paths berisi lokasi file dan endpoint yang dipakai Nginx Manager
type Paths struct {
	ConfFile  string
	ErrorLog  string
	BackupDir string
	StatusURL string
	Unit      string
}

// DefaultPaths mengembalikan lokasi standar (Linux) atau folder dev-data (non-Linux), bisa dioverride env
func DefaultPaths() Paths {
	p := Paths{
		ConfFile:  "/etc/nginx/nginx.conf",
		ErrorLog:  "/var/log/nginx/error.log",
		BackupDir: "/var/lib/fikpanel/backups/nginx",
		StatusURL: "http://127.0.0.1/nginx_status",
		Unit:      "nginx",
	}
	if runtime.GOOS != "linux" {
		p.ConfFile = filepath.Join("dev-data", "nginx", "nginx.conf")
		p.ErrorLog = filepath.Join("dev-data", "nginx", "error.log")
		p.BackupDir = filepath.Join("dev-data", "nginx", "backups")
	}
	if v := os.Getenv("FIKPANEL_NGINX_CONF"); v != "" {
		p.ConfFile = v
	}
	if v := os.Getenv("FIKPANEL_NGINX_ERROR_LOG"); v != "" {
		p.ErrorLog = v
	}
	if v := os.Getenv("FIKPANEL_NGINX_BACKUP_DIR"); v != "" {
		p.BackupDir = v
	}
	if v := os.Getenv("FIKPANEL_NGINX_STATUS_URL"); v != "" {
		p.StatusURL = v
	}
	return p
}

// Status merepresentasikan kondisi runtime Nginx
type Status struct {
	Installed    bool   `json:"installed"`
	Running      bool   `json:"running"`
	Version      string `json:"version"`
	MasterPID    int    `json:"master_pid"`
	Workers      int    `json:"workers"`
	MemoryBytes  uint64 `json:"memory_bytes"`
	StartedAt    int64  `json:"started_at"`
	ConfPath     string `json:"conf_path"`
	ErrorLogPath string `json:"error_log_path"`
	Daemon       bool   `json:"daemon"`
	Alert        bool   `json:"alert"`
	Mock         bool   `json:"mock"`
}

// Settings berisi preferensi watchdog Nginx
type Settings struct {
	Daemon bool `json:"daemon"`
	Alert  bool `json:"alert"`
}

// Manager mengelola siklus hidup, konfigurasi, dan observabilitas Nginx
type Manager struct {
	paths    Paths
	mock     bool
	audit    Auditor
	settings SettingsStore

	devOnce sync.Once
	devErr  error

	// cfgMu menserialisasi seluruh penulisan file konfigurasi
	cfgMu sync.Mutex

	// mu melindungi state simulasi dan job ganti versi
	mu          sync.Mutex
	mockRunning bool
	mockVersion string
	mockStarted time.Time
	job         SwitchJob
}

// NewManager membuat Nginx Manager; mode simulasi aktif otomatis di non-Linux
func NewManager(p Paths, a Auditor, s SettingsStore) *Manager {
	return &Manager{
		paths:       p,
		mock:        runtime.GOOS != "linux",
		audit:       a,
		settings:    s,
		mockRunning: true,
		mockVersion: "1.24.0",
		mockStarted: time.Now(),
	}
}

// Paths mengembalikan konfigurasi lokasi yang dipakai manager
func (m *Manager) Paths() Paths { return m.paths }

// ensureDevFiles membuat file konfigurasi & log contoh untuk mode simulasi (sekali saja)
func (m *Manager) ensureDevFiles() error {
	if !m.mock {
		return nil
	}
	m.devOnce.Do(func() {
		if err := os.MkdirAll(filepath.Dir(m.paths.ConfFile), 0o755); err != nil {
			m.devErr = fmt.Errorf("gagal membuat folder dev nginx: %w", err)
			return
		}
		if _, err := os.Stat(m.paths.ConfFile); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(m.paths.ConfFile, []byte(defaultDevConfig), 0o644); err != nil {
				m.devErr = fmt.Errorf("gagal menulis nginx.conf contoh: %w", err)
				return
			}
		}
		if _, err := os.Stat(m.paths.ErrorLog); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(m.paths.ErrorLog, []byte(sampleErrorLog()), 0o644); err != nil {
				m.devErr = fmt.Errorf("gagal menulis error.log contoh: %w", err)
				return
			}
		}
	})
	return m.devErr
}

// ═══ Status & Layanan ═══

var versionRe = regexp.MustCompile(`nginx/([0-9][0-9.]*)`)

// Status mengumpulkan status instalasi, proses, dan preferensi watchdog
func (m *Manager) Status(ctx context.Context) (Status, error) {
	st := Status{
		ConfPath:     m.paths.ConfFile,
		ErrorLogPath: m.paths.ErrorLog,
		Mock:         m.mock,
	}

	prefs, err := m.GetSettings(ctx)
	if err != nil {
		return st, err
	}
	st.Daemon, st.Alert = prefs.Daemon, prefs.Alert

	if m.mock {
		m.mu.Lock()
		defer m.mu.Unlock()
		st.Installed = true
		st.Version = m.mockVersion
		st.Running = m.mockRunning
		if st.Running {
			st.MasterPID = 1024
			st.Workers = runtime.NumCPU()
			st.MemoryBytes = uint64(st.Workers+1) * 6 * 1024 * 1024
			st.StartedAt = m.mockStarted.Unix()
		}
		return st, nil
	}

	bin, err := exec.LookPath("nginx")
	if err != nil {
		return st, nil
	}
	st.Installed = true

	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if out, err := exec.CommandContext(cmdCtx, bin, "-v").CombinedOutput(); err == nil {
		if mm := versionRe.FindStringSubmatch(string(out)); len(mm) > 1 {
			st.Version = mm[1]
		}
	}

	out, err := exec.CommandContext(cmdCtx, "systemctl", "show", m.paths.Unit,
		"--property=ActiveState,MainPID,ActiveEnterTimestamp").Output()
	if err != nil {
		return st, nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "ActiveState":
			st.Running = v == "active"
		case "MainPID":
			st.MasterPID, _ = strconv.Atoi(v)
		case "ActiveEnterTimestamp":
			if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", v); err == nil {
				st.StartedAt = t.Unix()
			}
		}
	}

	if st.Running {
		procs := scanNginxProcs()
		st.Workers = procs.Workers
		st.MemoryBytes = procs.RSSBytes
	}
	return st, nil
}

// ServiceAction menjalankan start/stop/restart/reload dengan uji konfigurasi lebih dulu
func (m *Manager) ServiceAction(ctx context.Context, action string) error {
	switch action {
	case "start", "stop", "restart", "reload":
	default:
		return &ValidationError{Msg: "aksi layanan tidak dikenal: " + action}
	}

	// Cegah restart/reload dengan konfigurasi rusak yang akan mematikan semua situs
	if action != "stop" {
		if err := m.TestConfigFile(ctx, m.paths.ConfFile); err != nil {
			return err
		}
	}

	if m.mock {
		m.mu.Lock()
		defer m.mu.Unlock()
		switch action {
		case "start":
			if !m.mockRunning {
				m.mockRunning = true
				m.mockStarted = time.Now()
			}
		case "stop":
			m.mockRunning = false
		case "restart":
			m.mockRunning = true
			m.mockStarted = time.Now()
		case "reload":
			if !m.mockRunning {
				return &ValidationError{Msg: "nginx tidak berjalan, reload tidak bisa dilakukan"}
			}
		}
		return nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "systemctl", action, m.paths.Unit).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s %s gagal: %s", action, m.paths.Unit, strings.TrimSpace(string(out)))
	}
	return nil
}

// ApplyChange menguji seluruh konfigurasi lalu reload bila Nginx berjalan (dipakai setelah vhost berubah)
func (m *Manager) ApplyChange(ctx context.Context) error {
	if err := m.TestConfigFile(ctx, m.paths.ConfFile); err != nil {
		return err
	}
	st, err := m.Status(ctx)
	if err != nil {
		return err
	}
	if !st.Running || m.mock {
		return nil
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "systemctl", "reload", m.paths.Unit).CombinedOutput()
	if err != nil {
		return fmt.Errorf("reload nginx gagal: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ═══ Preferensi Watchdog ═══

// GetSettings membaca preferensi daemon & alert dari database
func (m *Manager) GetSettings(ctx context.Context) (Settings, error) {
	daemon, err := m.settings.GetSetting(ctx, settingDaemon)
	if err != nil {
		return Settings{}, err
	}
	alert, err := m.settings.GetSetting(ctx, settingAlert)
	if err != nil {
		return Settings{}, err
	}
	return Settings{Daemon: daemon == "1", Alert: alert == "1"}, nil
}

// SaveSettings menyimpan preferensi daemon & alert
func (m *Manager) SaveSettings(ctx context.Context, s Settings) error {
	if err := m.settings.SetSetting(ctx, settingDaemon, boolStr(s.Daemon)); err != nil {
		return err
	}
	return m.settings.SetSetting(ctx, settingAlert, boolStr(s.Alert))
}

// RunWatchdog memeriksa status Nginx berkala; auto-start bila daemon aktif dan catat alert
func (m *Manager) RunWatchdog(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.watchdogTick(ctx)
		}
	}
}

// watchdogTick menjalankan satu siklus pemeriksaan watchdog
func (m *Manager) watchdogTick(ctx context.Context) {
	tickCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	st, err := m.Status(tickCtx)
	if err != nil || !st.Installed || st.Running || (!st.Daemon && !st.Alert) {
		return
	}

	if st.Alert {
		_ = m.audit.RecordAudit(tickCtx, "system", "nginx.alert_down", m.paths.Unit, false,
			"Watchdog mendeteksi Nginx berhenti", nil)
	}
	if st.Daemon {
		err := m.ServiceAction(tickCtx, "start")
		detail := "Watchdog berhasil menyalakan ulang Nginx"
		if err != nil {
			detail = "Watchdog gagal menyalakan Nginx: " + err.Error()
		}
		_ = m.audit.RecordAudit(tickCtx, "system", "nginx.watchdog_start", m.paths.Unit, err == nil, detail, nil)
	}
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
