package database

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// EngineVersion mendefinisikan opsi versi terverifikasi untuk Quick Install
type EngineVersion struct {
	Version       string `json:"version"`
	Label         string `json:"label"`
	IsRecommended bool   `json:"is_recommended"`
}

// EngineInfo merepresentasikan informasi lengkap tentang mesin database
type EngineInfo struct {
	Name              string          `json:"name"`               // 'mysql', 'postgresql', 'mongodb', 'redis'
	DisplayName       string          `json:"display_name"`
	Installed         bool            `json:"installed"`
	Version           string          `json:"version"`
	Running           bool            `json:"running"`
	Port              int             `json:"port"`
	DatabasesCount    int             `json:"databases_count"`
	ServiceUnit       string          `json:"service_unit"`
	SupportedVersions []string        `json:"supported_versions"` // Opsi versi string
	Versions          []EngineVersion `json:"versions"`           // Opsi versi lengkap dengan label & info rekomendasi
	Description       string          `json:"description"`
}

// EngineState menyimpan override dinamis (misal setelah Quick Install atau kontrol layanan)
type EngineState struct {
	mu           sync.RWMutex
	installed    map[string]bool
	versions     map[string]string
	running      map[string]bool
	configMemory map[string]string
}

// Global state untuk melacak perubahan runtime instalasi mesin database
var globalState = &EngineState{
	installed:    make(map[string]bool),
	versions:     make(map[string]string),
	running:      make(map[string]bool),
	configMemory: make(map[string]string),
}

// Pola regex ekstraksi versi engine agar tidak keliru menangkap versi pustaka klien (mis. Ver 15.1)
var (
	reMariaDBDistrib = regexp.MustCompile(`(?i)Distrib\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-MariaDB)?)`)
	reMariaDBFrom    = regexp.MustCompile(`(?i)from\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-MariaDB)?)`)
	reMariaDBTag     = regexp.MustCompile(`(?i)([0-9]+\.[0-9]+(?:\.[0-9]+)?-MariaDB)`)
	reRedisVer       = regexp.MustCompile(`v=([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	reGenericVer     = regexp.MustCompile(`([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
)

// ParseEngineVersion mengekstrak versi server database secara akurat dari output CLI
func ParseEngineVersion(engine, rawOutput string) string {
	out := strings.TrimSpace(rawOutput)
	if out == "" {
		return ""
	}

	switch strings.ToLower(engine) {
	case "mysql":
		if m := reMariaDBDistrib.FindStringSubmatch(out); len(m) > 1 {
			return m[1]
		}
		if m := reMariaDBFrom.FindStringSubmatch(out); len(m) > 1 {
			return m[1]
		}
		if m := reMariaDBTag.FindStringSubmatch(out); len(m) > 1 {
			return m[1]
		}
		if m := reGenericVer.FindStringSubmatch(out); len(m) > 1 {
			return m[1]
		}
	case "redis":
		if m := reRedisVer.FindStringSubmatch(out); len(m) > 1 {
			return m[1]
		}
		if m := reGenericVer.FindStringSubmatch(out); len(m) > 1 {
			return m[1]
		}
	default:
		if m := reGenericVer.FindStringSubmatch(out); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

// SetEngineInstalled memperbarui status instalasi engine di runtime
func SetEngineInstalled(engine, version string, running bool) {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.installed[engine] = true
	globalState.versions[engine] = version
	globalState.running[engine] = running
}

// SetEngineRunning mengubah status aktif layanan engine di runtime
func SetEngineRunning(engine string, running bool) {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.running[engine] = running
}

// IsPhpMyAdminInstalled memeriksa apakah paket resmi phpMyAdmin telah terpasang
func IsPhpMyAdminInstalled() (bool, string) {
	globalState.mu.RLock()
	defer globalState.mu.RUnlock()

	// 1. Periksa berkas fisik direktori phpMyAdmin di disk sistem
	if _, _, _, found := FindPhpMyAdminLocation(); found {
		ver := globalState.versions["phpmyadmin"]
		if ver == "" {
			ver = "5.2.1"
		}
		return true, ver
	}

	// 2. Periksa override state in-memory (setelah Quick Install selesai di FikPanel)
	if inst, ok := globalState.installed["phpmyadmin"]; ok && inst {
		ver := globalState.versions["phpmyadmin"]
		if ver == "" {
			ver = "5.2.1"
		}
		return true, ver
	}

	return false, ""
}

// DetectEngines mendeteksi status ketersediaan seluruh engine database
func DetectEngines(ctx context.Context, dbsCountMap map[string]int) []EngineInfo {
	globalState.mu.RLock()
	defer globalState.mu.RUnlock()

	engines := []EngineInfo{
		{
			Name:              "mysql",
			DisplayName:       "MySQL / MariaDB",
			Port:              3306,
			ServiceUnit:       "mariadb.service",
			SupportedVersions: []string{"10.11", "11.4", "8.0", "8.4"},
			Versions: []EngineVersion{
				{Version: "10.11", Label: "MariaDB 10.11 LTS (Rekomendasi Hosting)", IsRecommended: true},
				{Version: "11.4", Label: "MariaDB 11.4 LTS (Fitur Terbaru)", IsRecommended: false},
				{Version: "8.0", Label: "MySQL 8.0 LTS (Oracle Standar)", IsRecommended: false},
				{Version: "8.4", Label: "MySQL 8.4 LTS (Enterprise)", IsRecommended: false},
			},
			Description: "Relational database standar untuk aplikasi web PHP, Laravel, WordPress, dan CMS.",
		},
		{
			Name:              "postgresql",
			DisplayName:       "PostgreSQL",
			Port:              5432,
			ServiceUnit:       "postgresql.service",
			SupportedVersions: []string{"16", "15", "17", "14"},
			Versions: []EngineVersion{
				{Version: "16", Label: "PostgreSQL 16 (Rekomendasi Stabil)", IsRecommended: true},
				{Version: "15", Label: "PostgreSQL 15 LTS", IsRecommended: false},
				{Version: "17", Label: "PostgreSQL 17 (Fitur Terbaru)", IsRecommended: false},
				{Version: "14", Label: "PostgreSQL 14 LTS", IsRecommended: false},
			},
			Description: "Database relasional canggih berorientasi objek dengan performa query analitik tinggi.",
		},
		{
			Name:              "mongodb",
			DisplayName:       "MongoDB",
			Port:              27017,
			ServiceUnit:       "mongod.service",
			SupportedVersions: []string{"7.0", "6.0", "8.0", "5.0"},
			Versions: []EngineVersion{
				{Version: "7.0", Label: "MongoDB 7.0 LTS (Rekomendasi Stabil)", IsRecommended: true},
				{Version: "6.0", Label: "MongoDB 6.0 LTS (Kompatibilitas Luas)", IsRecommended: false},
				{Version: "8.0", Label: "MongoDB 8.0 (Modern Engine)", IsRecommended: false},
				{Version: "5.0", Label: "MongoDB 5.0 (Legacy)", IsRecommended: false},
			},
			Description: "Database dokumen NoSQL terdistribusi untuk struktur data JSON / BSON fleksibel.",
		},
		{
			Name:              "redis",
			DisplayName:       "Redis",
			Port:              6379,
			ServiceUnit:       "redis-server.service",
			SupportedVersions: []string{"7.2", "7.0", "6.2"},
			Versions: []EngineVersion{
				{Version: "7.2", Label: "Redis 7.2 (Rekomendasi Performa)", IsRecommended: true},
				{Version: "7.0", Label: "Redis 7.0 LTS", IsRecommended: false},
				{Version: "6.2", Label: "Redis 6.2 LTS (Stabil)", IsRecommended: false},
			},
			Description: "In-memory data structure store untuk caching berkecepatan ultra-tinggi, queue, dan sesi.",
		},
	}

	for i := range engines {
		eng := &engines[i]
		if count, ok := dbsCountMap[eng.Name]; ok {
			eng.DatabasesCount = count
		}

		// Pada Linux, selalu prioritaskan deteksi nyata dari host agar status systemd & versi selalu akurat
		if runtime.GOOS == "linux" {
			detectLinuxEngine(ctx, eng)
			if !eng.Installed {
				if inst, ok := globalState.installed[eng.Name]; ok && inst {
					eng.Installed = inst
					eng.Version = globalState.versions[eng.Name]
					eng.Running = globalState.running[eng.Name]
				}
			}
			continue
		}

		// Cek apakah ada override dari instalasi atau aksi kontrol sebelumnya (non-Linux dev mode)
		if inst, ok := globalState.installed[eng.Name]; ok {
			eng.Installed = inst
			eng.Version = globalState.versions[eng.Name]
			eng.Running = globalState.running[eng.Name]
			continue
		}

		// Mode simulasi lokal (Windows / macOS)
		switch eng.Name {
		case "mysql":
			eng.Installed = true
			eng.Version = "10.11.6-MariaDB"
			eng.Running = true
		case "postgresql":
			eng.Installed = true
			eng.Version = "16.4"
			eng.Running = true
		case "redis":
			eng.Installed = true
			eng.Version = "7.0.15"
			eng.Running = true
		case "mongodb":
			eng.Installed = false
			eng.Version = ""
			eng.Running = false
		}
		if runOverride, ok := globalState.running[eng.Name]; ok {
			eng.Running = runOverride
		}
	}

	return engines
}

// engineServiceCandidates mengembalikan daftar kandidat nama unit systemd untuk engine terkait
func engineServiceCandidates(engine string) []string {
	switch strings.ToLower(engine) {
	case "mysql":
		return []string{"mariadb", "mysql", "mysqld"}
	case "postgresql", "postgres":
		return []string{"postgresql"}
	case "mongodb":
		return []string{"mongod", "mongodb"}
	case "redis":
		return []string{"redis-server", "redis"}
	default:
		return []string{strings.ToLower(engine)}
	}
}

// ResolveServiceUnit menemukan nama unit systemd aktual yang terpasang di host Linux
func ResolveServiceUnit(ctx context.Context, engine string) string {
	candidates := engineServiceCandidates(engine)
	if runtime.GOOS != "linux" {
		return candidates[0]
	}
	// Prioritaskan unit yang berstatus active terlebih dahulu
	for _, s := range candidates {
		out, err := exec.CommandContext(ctx, "systemctl", "is-active", s).Output()
		if err == nil && strings.TrimSpace(string(out)) == "active" {
			return s
		}
	}
	// Jika belum aktif, cari unit yang file layanannya terdaftar di systemd
	for _, s := range candidates {
		out, _ := exec.CommandContext(ctx, "systemctl", "list-unit-files", s+".service", "--no-legend").Output()
		if strings.Contains(string(out), s+".service") {
			return s
		}
	}
	return candidates[0]
}

// ControlEngineService menjalankan aksi start/stop/restart/reload nyata terhadap systemd di Linux
func ControlEngineService(ctx context.Context, engine, action string) (bool, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "start", "stop", "restart", "reload":
	default:
		return false, fmt.Errorf("aksi layanan tidak valid: %s", action)
	}

	if runtime.GOOS != "linux" {
		isRunning := action != "stop"
		SetEngineRunning(engine, isRunning)
		return isRunning, nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	unit := ResolveServiceUnit(cmdCtx, engine)
	// Beberapa daemon (seperti redis/mongodb) tidak mendukung reload; gunakan restart bila reload gagal
	cmd := exec.CommandContext(cmdCtx, "systemctl", action, unit)
	out, err := cmd.CombinedOutput()
	if err != nil && action == "reload" {
		cmdRetry := exec.CommandContext(cmdCtx, "systemctl", "restart", unit)
		if retryOut, retryErr := cmdRetry.CombinedOutput(); retryErr == nil {
			err = nil
		} else {
			out = retryOut
			err = retryErr
		}
	}
	if err != nil {
		return false, fmt.Errorf("systemctl %s %s gagal: %w (%s)", action, unit, err, strings.TrimSpace(string(out)))
	}

	// Verifikasi status aktif terbaru dari systemd
	statusOut, statusErr := exec.CommandContext(cmdCtx, "systemctl", "is-active", unit).Output()
	isRunning := statusErr == nil && strings.TrimSpace(string(statusOut)) == "active"
	SetEngineRunning(engine, isRunning)
	return isRunning, nil
}

// detectLinuxEngine memeriksa keberadaan binary, versi server akurat, dan status service systemd di Linux
func detectLinuxEngine(ctx context.Context, eng *EngineInfo) {
	var binaries []string
	serviceUnits := engineServiceCandidates(eng.Name)

	switch eng.Name {
	case "mysql":
		// Prioritaskan biner server (mariadbd / mysqld) agar versi server akurat, bukan versi pustaka klien 15.1
		binaries = []string{"mariadbd", "mysqld", "mariadb", "mysql"}
	case "postgresql":
		binaries = []string{"psql", "postgres"}
	case "mongodb":
		binaries = []string{"mongod", "mongosh"}
	case "redis":
		binaries = []string{"redis-server", "redis-cli"}
	}

	// 1. Cek ketersediaan biner di PATH (atau sbin untuk mariadbd/mysqld)
	foundBin := ""
	for _, b := range binaries {
		if path, err := exec.LookPath(b); err == nil {
			foundBin = path
			break
		}
		for _, extraDir := range []string{"/usr/sbin", "/sbin", "/usr/local/sbin"} {
			candidate := filepath.Join(extraDir, b)
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				foundBin = candidate
				break
			}
		}
		if foundBin != "" {
			break
		}
	}

	if foundBin == "" {
		eng.Installed = false
		eng.Running = false
		return
	}
	eng.Installed = true

	// 2. Ekstrak versi server akurat (khusus MySQL/MariaDB, fallback ke SELECT VERSION() bila perlu)
	verCtx, cancelVer := context.WithTimeout(ctx, 2*time.Second)
	defer cancelVer()

	if out, err := exec.CommandContext(verCtx, foundBin, "--version").CombinedOutput(); err == nil {
		eng.Version = ParseEngineVersion(eng.Name, string(out))
	}
	if eng.Name == "mysql" && (eng.Version == "" || strings.HasPrefix(eng.Version, "15.")) {
		if sqlVer, err := RunMysqlQueryOutput(verCtx, "", "SELECT VERSION();"); err == nil {
			clean := strings.TrimSpace(sqlVer)
			if clean != "" {
				eng.Version = clean
			}
		}
	}

	// 3. Cek status systemd
	for _, s := range serviceUnits {
		sysOut, err := exec.CommandContext(verCtx, "systemctl", "is-active", s).Output()
		if err == nil && strings.TrimSpace(string(sysOut)) == "active" {
			eng.Running = true
			eng.ServiceUnit = s + ".service"
			break
		}
	}
	if !eng.Running {
		eng.ServiceUnit = ResolveServiceUnit(verCtx, eng.Name) + ".service"
	}
}

// DefaultConfigTemplate menyediakan template konfigurasi default saat file disk belum tersedia
func DefaultConfigTemplate(engine string) (string, string) {
	switch strings.ToLower(engine) {
	case "mysql":
		return "/etc/mysql/mariadb.conf.d/50-server.cnf", `[client]
port = 3306
socket = /run/mysqld/mysqld.sock
default-character-set = utf8mb4

[mysqld]
port = 3306
bind-address = 127.0.0.1
socket = /run/mysqld/mysqld.sock
datadir = /var/lib/mysql
tmpdir = /tmp

character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci

# Performa & Memori Cache
max_connections = 150
table_open_cache = 400
innodb_buffer_pool_size = 256M
innodb_log_file_size = 64M
innodb_file_per_table = 1
innodb_flush_log_at_trx_commit = 2

# Slow Query Log
slow_query_log = 1
slow_query_log_file = /var/log/mysql/slow.log
long_query_time = 2
`
	case "postgresql", "postgres":
		return "/etc/postgresql/16/main/postgresql.conf", `# Konfigurasi Utama PostgreSQL
listen_addresses = 'localhost'
port = 5432
max_connections = 100
shared_buffers = 128MB
effective_cache_size = 512MB
work_mem = 4MB
maintenance_work_mem = 64MB
logging_collector = on
log_directory = 'log'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
`
	case "mongodb":
		return "/etc/mongod.conf", `# Konfigurasi Daemon MongoDB
storage:
  dbPath: /var/lib/mongodb
  journal:
    enabled: true
systemLog:
  destination: file
  logAppend: true
  path: /var/log/mongodb/mongod.log
net:
  port: 27017
  bindIp: 127.0.0.1
`
	case "redis":
		return "/etc/redis/redis.conf", `# Konfigurasi Redis In-Memory Cache
bind 127.0.0.1 ::1
port 6379
protected-mode yes
timeout 0
tcp-keepalive 300
daemonize yes
supervised systemd
loglevel notice
logfile /var/log/redis/redis-server.log
maxmemory 256mb
maxmemory-policy allkeys-lru
`
	default:
		return "/etc/database.conf", "# Konfigurasi mesin database\n"
	}
}

// ResolveConfigPath menemukan path berkas konfigurasi aktual pada host Linux
func ResolveConfigPath(engine string) string {
	switch strings.ToLower(engine) {
	case "mysql":
		candidates := []string{
			"/etc/mysql/mariadb.conf.d/50-server.cnf",
			"/etc/mysql/mysql.conf.d/mysqld.cnf",
			"/etc/mysql/my.cnf",
			"/etc/my.cnf",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		return candidates[0]
	case "postgresql", "postgres":
		matches, _ := filepath.Glob("/etc/postgresql/*/main/postgresql.conf")
		if len(matches) > 0 {
			sort.Strings(matches)
			return matches[len(matches)-1]
		}
		if _, err := os.Stat("/var/lib/pgsql/data/postgresql.conf"); err == nil {
			return "/var/lib/pgsql/data/postgresql.conf"
		}
		return "/etc/postgresql/16/main/postgresql.conf"
	case "redis":
		for _, p := range []string{"/etc/redis/redis.conf", "/etc/redis.conf"} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		return "/etc/redis/redis.conf"
	case "mongodb":
		for _, p := range []string{"/etc/mongod.conf", "/etc/mongodb.conf"} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		return "/etc/mongod.conf"
	default:
		return "/etc/database.conf"
	}
}

// ReadEngineConfig membaca berkas konfigurasi engine langsung dari disk (atau fallback template di non-Linux)
func ReadEngineConfig(engine string) (string, string, bool, error) {
	engine = strings.ToLower(engine)
	if runtime.GOOS != "linux" {
		defPath, defContent := DefaultConfigTemplate(engine)
		globalState.mu.RLock()
		memContent, ok := globalState.configMemory[engine]
		globalState.mu.RUnlock()
		if ok && memContent != "" {
			return defPath, memContent, false, nil
		}
		return defPath, defContent, false, nil
	}

	path := ResolveConfigPath(engine)
	data, err := os.ReadFile(path)
	if err != nil {
		// Kembalikan template jika berkas belum ada di disk
		_, defContent := DefaultConfigTemplate(engine)
		return path, defContent, false, nil
	}
	return path, string(data), true, nil
}

// WriteEngineConfig menulis perubahan konfigurasi ke disk secara aman dengan salinan cadangan (.bak)
func WriteEngineConfig(engine, content string) (string, string, error) {
	engine = strings.ToLower(engine)
	if runtime.GOOS != "linux" {
		defPath, _ := DefaultConfigTemplate(engine)
		globalState.mu.Lock()
		globalState.configMemory[engine] = content
		globalState.mu.Unlock()
		return defPath, "", nil
	}

	path := ResolveConfigPath(engine)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return path, "", fmt.Errorf("gagal menyiapkan direktori konfigurasi: %w", err)
	}

	backupPath := ""
	if oldBytes, err := os.ReadFile(path); err == nil {
		backupPath = fmt.Sprintf("%s.bak.%s", path, time.Now().Format("20060102-150405"))
		_ = os.WriteFile(backupPath, oldBytes, 0644)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return path, backupPath, fmt.Errorf("gagal menulis konfigurasi ke %s: %w", path, err)
	}
	return path, backupPath, nil
}

// tailFileLines membaca N baris terakhir dari berkas teks secara efisien
func tailFileLines(path string, maxLines int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var ring []string
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 512*1024)
	for scanner.Scan() {
		ring = append(ring, scanner.Text())
		if len(ring) > maxLines {
			ring = ring[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return strings.Join(ring, "\n"), nil
}

// ReadEngineLogs membaca log riil dari journalctl atau file error log daemon di Linux
func ReadEngineLogs(ctx context.Context, engine string, lines int) (string, string, error) {
	engine = strings.ToLower(engine)
	if lines <= 0 || lines > 500 {
		lines = 150
	}

	if runtime.GOOS != "linux" {
		port := 3306
		switch engine {
		case "postgresql", "postgres":
			port = 5432
		case "redis":
			port = 6379
		case "mongodb":
			port = 27017
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		sim := fmt.Sprintf(
			"[%s] [Note] Starting %s daemon (mode pengembangan lokal)...\n[%s] [Note] Ready for connections on 127.0.0.1:%d.\n[%s] [Note] InnoDB / Storage Engine initialized cleanly without crash recovery.\n[%s] [Note] Listening socket active. Health check OK.\n",
			now, engine, now, port, now, now,
		)
		return "Simulasi Lokal", sim, nil
	}

	logCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 1. Coba journalctl untuk unit systemd engine terkait
	unit := ResolveServiceUnit(logCtx, engine)
	jOut, jErr := exec.CommandContext(logCtx, "journalctl", "-u", unit, "-n", fmt.Sprintf("%d", lines), "--no-pager").CombinedOutput()
	trimmedJournal := strings.TrimSpace(string(jOut))
	if jErr == nil && trimmedJournal != "" && !strings.Contains(trimmedJournal, "-- No entries --") {
		return fmt.Sprintf("journalctl -u %s", unit), trimmedJournal, nil
	}

	// 2. Fallback ke berkas error log fisik di /var/log
	var logCandidates []string
	switch engine {
	case "mysql":
		logCandidates = []string{"/var/log/mysql/error.log", "/var/log/mariadb/mariadb.log", "/var/log/mysqld.log"}
	case "postgresql", "postgres":
		matches, _ := filepath.Glob("/var/log/postgresql/*.log")
		sort.Strings(matches)
		if len(matches) > 0 {
			logCandidates = append(logCandidates, matches[len(matches)-1])
		}
	case "redis":
		logCandidates = []string{"/var/log/redis/redis-server.log", "/var/log/redis/redis.log"}
	case "mongodb":
		logCandidates = []string{"/var/log/mongodb/mongod.log"}
	}

	for _, p := range logCandidates {
		if text, err := tailFileLines(p, lines); err == nil && strings.TrimSpace(text) != "" {
			return p, text, nil
		}
	}

	if trimmedJournal != "" {
		return fmt.Sprintf("journalctl -u %s", unit), trimmedJournal, nil
	}
	return "journalctl", fmt.Sprintf("Belum ada entri log terbaru untuk layanan %s.", unit), nil
}
