package services

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ServiceInfo merepresentasikan status operasional unit layanan systemd secara komprehensif
type ServiceInfo struct {
	Unit                 string `json:"unit"`
	Name                 string `json:"name"`
	Label                string `json:"label"`
	Description          string `json:"description"`
	Category             string `json:"category"`
	CategoryLabel        string `json:"category_label"`
	Protected            bool   `json:"protected"`
	LoadState            string `json:"load_state"`
	ActiveState          string `json:"active_state"`
	SubState             string `json:"sub_state"`
	UnitFileState        string `json:"unit_file_state"`
	MainPID              int    `json:"main_pid"`
	MemoryCurrent        uint64 `json:"memory_current"`
	MemoryPeak           uint64 `json:"memory_peak"`
	TasksCurrent         int    `json:"tasks_current"`
	ActiveEnterTimestamp string `json:"active_enter_timestamp"`
	Uptime               string `json:"uptime"`
	ExecStart            string `json:"exec_start"`
	Restart              string `json:"restart"`
}

// ServiceCounts merepresentasikan ringkasan kuantitatif status layanan pada sistem
type ServiceCounts struct {
	Total       int    `json:"total"`
	Running     int    `json:"running"`
	Stopped     int    `json:"stopped"`
	Failed      int    `json:"failed"`
	TotalMemory uint64 `json:"total_memory"`
}

// CategoryStat merepresentasikan agregasi kuantitas per kategori layanan
type CategoryStat struct {
	Category string `json:"category"`
	Label    string `json:"label"`
	Count    int    `json:"count"`
}

// Manager mengelola allowlist, deteksi dinamis, inspeksi mendalam, dan eksekusi layanan sistem
type Manager struct {
	db *sql.DB
}

// NewManager membuat instance Service Manager baru
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// SeedDefaultServices mendaftarkan unit layanan standar yang umum dikelola
func (m *Manager) SeedDefaultServices(ctx context.Context) error {
	defaults := []struct {
		unit      string
		label     string
		protected int
	}{
		{"nginx.service", "Nginx Reverse Proxy", 0},
		{"sshd.service", "OpenSSH Server", 1},
		{"fikpanel.service", "Fikpanel Web Server", 1},
	}

	for _, d := range defaults {
		_, err := m.db.ExecContext(ctx,
			"INSERT OR IGNORE INTO managed_services (unit, label, protected) VALUES (?, ?, ?)",
			d.unit, d.label, d.protected)
		if err != nil {
			return fmt.Errorf("gagal mendaftarkan default service %s: %w", d.unit, err)
		}
	}

	// Pindai dan daftarkan service sistem nyata yang terpasang pada host Linux
	_ = m.discoverInstalledServices(ctx)
	return nil
}

// discoverInstalledServices memindai unit service nyata di Linux dan mendaftarkannya otomatis
func (m *Manager) discoverInstalledServices(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cmdCtx, "systemctl", "list-unit-files", "--type=service", "--no-legend").Output()
	if err != nil {
		return err
	}

	phpFpmRe := regexp.MustCompile(`^php([0-9.]*)-fpm\.service$`)

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		unit := fields[0]

		var label string
		var protected int

		switch {
		case unit == "fikpanel.service":
			label = "Fikpanel Web Server"
			protected = 1
		case unit == "sshd.service" || unit == "ssh.service":
			label = "OpenSSH Server"
			protected = 1
		case unit == "nginx.service":
			label = "Nginx Reverse Proxy"
		case unit == "apache2.service" || unit == "httpd.service":
			label = "Apache HTTP Server"
		case unit == "mysql.service" || unit == "mysqld.service":
			label = "MySQL Database Server"
		case unit == "mariadb.service":
			label = "MariaDB Database Server"
		case unit == "docker.service":
			label = "Docker Container Engine"
		case unit == "redis.service" || unit == "redis-server.service":
			label = "Redis In-Memory Store"
		case unit == "postgresql.service":
			label = "PostgreSQL Database Server"
		case unit == "mongod.service" || unit == "mongodb.service":
			label = "MongoDB Database Server"
		case unit == "fail2ban.service":
			label = "Fail2ban Security Daemon"
		case unit == "cron.service" || unit == "crond.service":
			label = "Cron Task Scheduler"
		case unit == "ufw.service":
			label = "UFW Firewall Daemon"
		case phpFpmRe.MatchString(unit):
			matches := phpFpmRe.FindStringSubmatch(unit)
			if len(matches) > 1 && matches[1] != "" {
				label = fmt.Sprintf("PHP %s FastCGI (FPM)", matches[1])
			} else {
				label = "PHP FastCGI (FPM)"
			}
		default:
			continue
		}

		// Daftarkan ke tabel managed_services jika belum terdaftar
		_, _ = m.db.ExecContext(ctx,
			"INSERT OR IGNORE INTO managed_services (unit, label, protected) VALUES (?, ?, ?)",
			unit, label, protected)
	}

	return nil
}

// CategorizeUnit mengklasifikasikan unit layanan ke dalam kelompok arsitektur sistem
func CategorizeUnit(unit, desc string) (category string, categoryLabel string, friendlyLabel string) {
	u := strings.ToLower(unit)
	d := strings.ToLower(desc)
	name := strings.TrimSuffix(u, ".service")

	// 1. Stack Web & Aplikasi
	if strings.HasPrefix(name, "php") && strings.Contains(name, "fpm") {
		ver := strings.TrimSuffix(strings.TrimPrefix(name, "php"), "-fpm")
		if ver != "" {
			return "web", "Web & App Stack", fmt.Sprintf("PHP %s FastCGI (FPM)", ver)
		}
		return "web", "Web & App Stack", "PHP FastCGI (FPM)"
	}
	switch name {
	case "nginx":
		return "web", "Web & App Stack", "Nginx Reverse Proxy & Web Server"
	case "apache2", "httpd":
		return "web", "Web & App Stack", "Apache HTTP Server"
	case "caddy":
		return "web", "Web & App Stack", "Caddy Web Server"
	case "fikpanel":
		return "web", "Web & App Stack", "FikPanel Server Control Panel"
	case "node", "pm2":
		return "web", "Web & App Stack", "Node.js Process Daemon"
	}
	if strings.Contains(d, "web server") || strings.Contains(d, "reverse proxy") || strings.Contains(d, "fastcgi") || strings.Contains(d, "http") {
		return "web", "Web & App Stack", desc
	}

	// 2. Database & Cache
	switch name {
	case "mysql", "mysqld":
		return "database", "Database & Cache", "MySQL Community Server"
	case "mariadb":
		return "database", "Database & Cache", "MariaDB Database Server"
	case "postgresql", "postgres":
		return "database", "Database & Cache", "PostgreSQL Relational Database"
	case "redis", "redis-server":
		return "database", "Database & Cache", "Redis In-Memory Data Store"
	case "memcached":
		return "database", "Database & Cache", "Memcached Caching Daemon"
	case "mongod", "mongodb":
		return "database", "Database & Cache", "MongoDB NoSQL Database"
	case "clickhouse-server":
		return "database", "Database & Cache", "ClickHouse Analytical Database"
	}
	if strings.Contains(d, "database") || strings.Contains(d, "sql server") || strings.Contains(d, "cache") {
		return "database", "Database & Cache", desc
	}

	// 3. Supervisor & Worker / Antrean
	if strings.Contains(name, "supervisor") || strings.Contains(name, "celery") ||
		strings.Contains(name, "worker") || strings.Contains(name, "queue") ||
		strings.Contains(name, "sidekiq") || (strings.Contains(name, "daemon") && strings.Contains(d, "process")) {
		if strings.Contains(name, "supervisor") {
			return "supervisor", "Supervisor & Worker", "Supervisord Process Controller"
		}
		return "supervisor", "Supervisor & Worker", desc
	}

	// 4. Keamanan, Firewall, & Jaringan
	switch name {
	case "sshd", "ssh":
		return "security", "Keamanan & Jaringan", "OpenSSH Server Daemon"
	case "fail2ban":
		return "security", "Keamanan & Jaringan", "Fail2Ban Intrusion Prevention"
	case "ufw":
		return "security", "Keamanan & Jaringan", "UFW Uncomplicated Firewall"
	case "firewalld":
		return "security", "Keamanan & Jaringan", "Firewalld Network Packet Filter"
	case "chrony", "chronyd":
		return "security", "Keamanan & Jaringan", "Chrony Network Time Protocol"
	case "systemd-resolved":
		return "security", "Keamanan & Jaringan", "Systemd Network Name Resolution"
	case "systemd-timesyncd":
		return "security", "Keamanan & Jaringan", "Systemd Network Time Synchronization"
	case "apparmor":
		return "security", "Keamanan & Jaringan", "AppArmor Security Module"
	}
	if strings.Contains(d, "firewall") || strings.Contains(d, "security") || strings.Contains(d, "ssh") || strings.Contains(d, "ntp") || strings.Contains(d, "dns") || strings.Contains(d, "network") {
		return "security", "Keamanan & Jaringan", desc
	}

	// 5. Sistem Inti & Background Daemons
	switch name {
	case "cron", "crond":
		return "system", "Sistem & Daemon", "Cron Scheduled Task Daemon"
	case "docker":
		return "system", "Sistem & Daemon", "Docker Container Engine"
	case "containerd":
		return "system", "Sistem & Daemon", "Containerd Runtime Daemon"
	case "dbus":
		return "system", "Sistem & Daemon", "D-Bus System Message Bus"
	case "systemd-journald":
		return "system", "Sistem & Daemon", "Systemd Journal Event Logger"
	case "rsyslog":
		return "system", "Sistem & Daemon", "Rsyslog System Logging Service"
	case "acpid":
		return "system", "Sistem & Daemon", "ACPI Power Event Daemon"
	case "multipathd":
		return "system", "Sistem & Daemon", "Multipath Device Controller"
	}

	if desc != "" {
		return "system", "Sistem & Daemon", desc
	}
	return "system", "Sistem & Daemon", name
}

// formatUptime menghitung durasi aktif ramah manusia dari timestamp systemd
func formatUptime(activeEnterTimestamp string) string {
	ts := strings.TrimSpace(activeEnterTimestamp)
	if ts == "" || ts == "n/a" || ts == "[not set]" {
		return ""
	}

	var t time.Time
	var err error
	layouts := []string{
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		t, err = time.Parse(layout, ts)
		if err == nil {
			break
		}
	}

	if err != nil || t.IsZero() {
		return ""
	}

	d := time.Since(t)
	if d < 0 {
		return "baru saja"
	}

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dj %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dh %dj", days, hours)
}

// cleanExecStart menyaring format ExecStart systemd yang kompleks menjadi representasi perintah bersih
func cleanExecStart(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "n/a" || raw == "[not set]" {
		return ""
	}

	if idx := strings.Index(raw, "argv[]="); idx != -1 {
		rest := raw[idx+7:]
		if end := strings.Index(rest, " ; ignore_errors"); end != -1 {
			return strings.TrimSpace(rest[:end])
		}
		if end := strings.Index(rest, " ; start_time"); end != -1 {
			return strings.TrimSpace(rest[:end])
		}
		if end := strings.Index(rest, " ;"); end != -1 {
			return strings.TrimSpace(rest[:end])
		}
		return strings.TrimSpace(rest)
	}

	if idx := strings.Index(raw, "path="); idx != -1 {
		rest := raw[idx+5:]
		if end := strings.Index(rest, " ;"); end != -1 {
			return strings.TrimSpace(rest[:end])
		}
	}

	return raw
}

// ListServices mengambil daftar layanan terkelola (kompatibel dengan modul dashboard lama)
func (m *Manager) ListServices(ctx context.Context) ([]ServiceInfo, error) {
	services, _, err := m.ListServicesFiltered(ctx, "managed", "", "")
	return services, err
}

// ListServicesFiltered mengambil daftar layanan secara mendalam dengan dukungan scope, kategori, dan pencarian
func (m *Manager) ListServicesFiltered(ctx context.Context, scope, category, search string) ([]ServiceInfo, ServiceCounts, error) {
	var counts ServiceCounts

	// Di lingkungan non-Linux, berikan data tiruan realistis untuk pengujian antarmuka
	if runtime.GOOS != "linux" {
		return m.mockServicesData(scope, category, search)
	}

	// 1. Ambil daftar allowlist yang tersimpan di basis data SQLite
	managedMap := make(map[string]struct {
		label     string
		protected bool
	})
	rows, err := m.db.QueryContext(ctx, "SELECT unit, coalesce(label, ''), protected FROM managed_services")
	if err == nil {
		for rows.Next() {
			var u, l string
			var p int
			if err := rows.Scan(&u, &l, &p); err == nil {
				managedMap[u] = struct {
					label     string
					protected bool
				}{label: l, protected: p == 1}
			}
		}
		rows.Close()
	}

	// 2. Tentukan unit target berdasarkan parameter scope
	var targetUnits []string
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "" {
		scope = "running"
	}

	if scope == "managed" {
		for u := range managedMap {
			targetUnits = append(targetUnits, u)
		}
		sort.Strings(targetUnits)
	} else {
		// Panggil systemctl list-units
		cmdArgs := []string{"list-units", "--type=service", "--no-legend", "--no-pager"}
		if scope == "running" {
			cmdArgs = append(cmdArgs, "--state=running")
		} else if scope == "all" {
			cmdArgs = append(cmdArgs, "--all")
		}

		cmdCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()

		out, err := exec.CommandContext(cmdCtx, "systemctl", cmdArgs...).Output()
		if err != nil {
			return nil, counts, fmt.Errorf("gagal mengeksekusi systemctl list-units: %w", err)
		}

		unitSeen := make(map[string]bool)
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			unitName := fields[0]
			if !strings.HasSuffix(unitName, ".service") {
				continue
			}
			if !unitSeen[unitName] {
				unitSeen[unitName] = true
				targetUnits = append(targetUnits, unitName)
			}
		}

		// Pastikan unit penting dari allowlist tetap dimasukkan ke target agar tidak terlewat
		for u := range managedMap {
			if !unitSeen[u] {
				unitSeen[u] = true
				targetUnits = append(targetUnits, u)
			}
		}
	}

	if len(targetUnits) == 0 {
		return []ServiceInfo{}, counts, nil
	}

	// 3. Batch query properti teknis via systemctl show dalam chunk maksimal 40 unit
	properties := "Id,Description,LoadState,ActiveState,SubState,UnitFileState,MainPID,MemoryCurrent,MemoryPeak,TasksCurrent,ActiveEnterTimestamp,ExecStart,Restart"
	infoMap := make(map[string]*ServiceInfo)

	chunkSize := 40
	for i := 0; i < len(targetUnits); i += chunkSize {
		end := i + chunkSize
		if end > len(targetUnits) {
			end = len(targetUnits)
		}
		chunk := targetUnits[i:end]

		showArgs := append([]string{"show"}, chunk...)
		showArgs = append(showArgs, "--property="+properties)

		cmdCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		out, err := exec.CommandContext(cmdCtx, "systemctl", showArgs...).Output()
		cancel()
		if err != nil {
			continue
		}

		m.parseBatchShowOutput(string(out), infoMap)
	}

	// 4. Proses agregasi data, kategorisasi, filter, dan perhitungan uptime
	var allServices []ServiceInfo
	category = strings.ToLower(strings.TrimSpace(category))
	search = strings.ToLower(strings.TrimSpace(search))

	for _, unit := range targetUnits {
		s, exists := infoMap[unit]
		if !exists {
			continue
		}
		if s.LoadState == "not-found" {
			continue
		}

		s.Unit = unit
		s.Name = strings.TrimSuffix(unit, ".service")

		// Tentukan proteksi sistem
		if unit == "fikpanel.service" || unit == "sshd.service" || unit == "ssh.service" {
			s.Protected = true
		} else if mMeta, ok := managedMap[unit]; ok && mMeta.protected {
			s.Protected = true
		}

		// Klasifikasikan kategori dan label ramah pengguna
		cat, catLabel, friendlyLabel := CategorizeUnit(s.Unit, s.Description)
		s.Category = cat
		s.CategoryLabel = catLabel

		if mMeta, ok := managedMap[unit]; ok && mMeta.label != "" {
			s.Label = mMeta.label
		} else if friendlyLabel != "" {
			s.Label = friendlyLabel
		} else if s.Description != "" {
			s.Label = s.Description
		} else {
			s.Label = s.Name
		}

		// Hitung uptime dan rapikan ExecStart
		s.Uptime = formatUptime(s.ActiveEnterTimestamp)
		s.ExecStart = cleanExecStart(s.ExecStart)

		// Hitung statistik global sebelum filter diterapkan
		counts.Total++
		if s.ActiveState == "active" {
			counts.Running++
		} else if s.ActiveState == "failed" {
			counts.Failed++
		} else {
			counts.Stopped++
		}
		counts.TotalMemory += s.MemoryCurrent

		// Terapkan filter kategori
		if category != "" && category != "all" && s.Category != category {
			continue
		}

		// Terapkan filter pencarian teks
		if search != "" {
			match := strings.Contains(strings.ToLower(s.Unit), search) ||
				strings.Contains(strings.ToLower(s.Label), search) ||
				strings.Contains(strings.ToLower(s.Description), search) ||
				strings.Contains(strings.ToLower(s.CategoryLabel), search)
			if !match {
				continue
			}
		}

		allServices = append(allServices, *s)
	}

	// 5. Urutkan daftar layanan: Protected di atas, lalu Failed, lalu Running, lalu Inactive
	sort.Slice(allServices, func(i, j int) bool {
		a, b := allServices[i], allServices[j]

		// Proteksi panel dan SSH selalu di posisi teratas
		if a.Protected != b.Protected {
			return a.Protected
		}

		// Service yang mengalami kegagalan (failed) diprioritaskan agar langsung terlihat
		if (a.ActiveState == "failed") != (b.ActiveState == "failed") {
			return a.ActiveState == "failed"
		}

		// Service aktif didahulukan daripada inactive
		if (a.ActiveState == "active") != (b.ActiveState == "active") {
			return a.ActiveState == "active"
		}

		// Urutkan berdasarkan unit name
		return a.Unit < b.Unit
	})

	return allServices, counts, nil
}

// parseBatchShowOutput membaca blok output systemctl show untuk banyak unit sekaligus
func (m *Manager) parseBatchShowOutput(output string, infoMap map[string]*ServiceInfo) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var current *ServiceInfo

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]

		if key == "Id" {
			if val != "" {
				current = &ServiceInfo{
					Unit: val,
				}
				infoMap[val] = current
			}
			continue
		}

		if current == nil {
			continue
		}

		switch key {
		case "Description":
			current.Description = val
		case "LoadState":
			current.LoadState = val
		case "ActiveState":
			current.ActiveState = val
		case "SubState":
			current.SubState = val
		case "UnitFileState":
			current.UnitFileState = val
		case "MainPID":
			current.MainPID, _ = strconv.Atoi(val)
		case "MemoryCurrent":
			// Hindari nilai tidak valid atau overflow dari cgroup yang tidak aktif
			if val != "[not set]" && val != "" {
				if v, err := strconv.ParseUint(val, 10, 64); err == nil && v < 18446744073709550000 {
					current.MemoryCurrent = v
				}
			}
		case "MemoryPeak":
			if val != "[not set]" && val != "" {
				if v, err := strconv.ParseUint(val, 10, 64); err == nil && v < 18446744073709550000 {
					current.MemoryPeak = v
				}
			}
		case "TasksCurrent":
			if val != "[not set]" && val != "" {
				current.TasksCurrent, _ = strconv.Atoi(val)
			}
		case "ActiveEnterTimestamp":
			current.ActiveEnterTimestamp = val
		case "ExecStart":
			current.ExecStart = val
		case "Restart":
			current.Restart = val
		}
	}
}

var safeUnitRe = regexp.MustCompile(`^[a-zA-Z0-9_.@-]+$`)

// ExecuteAction mengeksekusi aksi lifecycle systemd secara terkontrol dan aman
func (m *Manager) ExecuteAction(ctx context.Context, unit, action string) error {
	unit = strings.TrimSpace(unit)
	if !strings.HasSuffix(unit, ".service") {
		unit += ".service"
	}

	if !safeUnitRe.MatchString(unit) {
		return fmt.Errorf("nama unit layanan tidak valid: %s", unit)
	}

	switch action {
	case "start", "stop", "restart", "reload", "enable", "disable":
	default:
		return fmt.Errorf("aksi layanan tidak didukung: %s", action)
	}

	// Proteksi unit krusial: dilarang mematikan atau menonaktifkan unit inti panel / remote access
	isProtected := (unit == "fikpanel.service" || unit == "sshd.service" || unit == "ssh.service")
	if isProtected && (action == "stop" || action == "disable") {
		return fmt.Errorf("layanan %s dilindungi sistem (PROTECTED). Aksi '%s' ditolak untuk mencegah hilangnya akses panel/SSH.", unit, action)
	}

	if runtime.GOOS != "linux" {
		// Di lingkungan pengembangan non-Linux, return berhasil
		return nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "systemctl", action, unit)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if outStr != "" {
			return fmt.Errorf("gagal %s %s: %s", action, unit, outStr)
		}
		return fmt.Errorf("gagal %s %s: %w", action, unit, err)
	}

	return nil
}

// GetServiceLogs mengambil log jurnal systemd terkini untuk unit layanan tertentu
func (m *Manager) GetServiceLogs(ctx context.Context, unit string, lines int) (string, error) {
	unit = strings.TrimSpace(unit)
	if !strings.HasSuffix(unit, ".service") {
		unit += ".service"
	}
	if !safeUnitRe.MatchString(unit) {
		return "", fmt.Errorf("nama unit layanan tidak valid")
	}

	if lines <= 0 || lines > 1000 {
		lines = 100
	}

	if runtime.GOOS != "linux" {
		now := time.Now().Format("2006-01-02 15:04:05")
		return fmt.Sprintf("[%s] Log simulasi Windows untuk %s\n[%s] systemd[1]: Started %s\n[%s] worker[1234]: Active and accepting connections.",
			now, unit, now, unit, now), nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "journalctl", "-u", unit, "-n", strconv.Itoa(lines), "--no-pager")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("gagal mengambil log: %w", err)
	}

	return string(out), nil
}

// InspectService mengambil seluruh pasangan key-value properti systemd dari unit
func (m *Manager) InspectService(ctx context.Context, unit string) (map[string]string, error) {
	unit = strings.TrimSpace(unit)
	if !strings.HasSuffix(unit, ".service") {
		unit += ".service"
	}
	if !safeUnitRe.MatchString(unit) {
		return nil, fmt.Errorf("nama unit layanan tidak valid")
	}

	if runtime.GOOS != "linux" {
		return map[string]string{
			"Id":            unit,
			"Description":   "Simulated Windows Service",
			"ActiveState":   "active",
			"SubState":      "running",
			"LoadState":     "loaded",
			"UnitFileState": "enabled",
			"MainPID":       "1234",
			"Restart":       "always",
		}, nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "systemctl", "show", unit)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gagal memeriksa status unit: %w", err)
	}

	props := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			props[parts[0]] = parts[1]
		}
	}

	return props, nil
}

// mockServicesData menghasilkan data tiruan kaya saat dijalankan di lingkungan Windows
func (m *Manager) mockServicesData(scope, category, search string) ([]ServiceInfo, ServiceCounts, error) {
	// Jika scope adalah "managed", kembalikan data yang sesuai dengan tabel managed_services
	if scope == "managed" {
		var managedList []ServiceInfo
		rows, err := m.db.Query("SELECT unit, coalesce(label, ''), protected FROM managed_services ORDER BY unit ASC")
		if err == nil {
			for rows.Next() {
				var u, l string
				var p int
				if err := rows.Scan(&u, &l, &p); err == nil {
					cat, catLabel, _ := CategorizeUnit(u, l)
					managedList = append(managedList, ServiceInfo{
						Unit:          u,
						Name:          strings.TrimSuffix(u, ".service"),
						Label:         l,
						Category:      cat,
						CategoryLabel: catLabel,
						Protected:     p == 1,
						LoadState:     "loaded",
						ActiveState:   "active",
						SubState:      "running",
						UnitFileState: "enabled",
						MainPID:       1000,
						MemoryCurrent: 16 * 1024 * 1024,
						TasksCurrent:  4,
						Uptime:        "1h 30m",
					})
				}
			}
			rows.Close()
		}
		counts := ServiceCounts{
			Total:       len(managedList),
			Running:     len(managedList),
			Stopped:     0,
			Failed:      0,
			TotalMemory: uint64(len(managedList)) * 16 * 1024 * 1024,
		}
		return managedList, counts, nil
	}

	mocks := []ServiceInfo{
		{
			Unit: "fikpanel.service", Name: "fikpanel", Label: "FikPanel Server Control Panel",
			Description: "FikPanel Server Control Panel Daemon", Category: "web", CategoryLabel: "Web & App Stack",
			Protected: true, LoadState: "loaded", ActiveState: "active", SubState: "running",
			UnitFileState: "enabled", MainPID: 62636, MemoryCurrent: 12582912, MemoryPeak: 52428800,
			TasksCurrent: 10, Uptime: "17j 42m", ExecStart: "/opt/fikpanel/bin/fikpanel serve --port 1405",
			Restart: "always",
		},
		{
			Unit: "sshd.service", Name: "sshd", Label: "OpenSSH Server Daemon",
			Description: "OpenSSH Server", Category: "security", CategoryLabel: "Keamanan & Jaringan",
			Protected: true, LoadState: "loaded", ActiveState: "active", SubState: "running",
			UnitFileState: "enabled", MainPID: 44743, MemoryCurrent: 6606848, MemoryPeak: 8388608,
			TasksCurrent: 1, Uptime: "1d 3h", ExecStart: "/usr/sbin/sshd -D",
			Restart: "on-failure",
		},
		{
			Unit: "nginx.service", Name: "nginx", Label: "Nginx Reverse Proxy & Web Server",
			Description: "A high performance web server and a reverse proxy server", Category: "web", CategoryLabel: "Web & App Stack",
			Protected: false, LoadState: "loaded", ActiveState: "active", SubState: "running",
			UnitFileState: "enabled", MainPID: 44753, MemoryCurrent: 7172096, MemoryPeak: 10485760,
			TasksCurrent: 3, Uptime: "1d 3h", ExecStart: "/usr/sbin/nginx -g daemon on; master_process on;",
			Restart: "no",
		},
		{
			Unit: "php8.5-fpm.service", Name: "php8.5-fpm", Label: "PHP 8.5 FastCGI (FPM)",
			Description: "The PHP 8.5 FastCGI Process Manager", Category: "web", CategoryLabel: "Web & App Stack",
			Protected: false, LoadState: "loaded", ActiveState: "active", SubState: "running",
			UnitFileState: "enabled", MainPID: 44728, MemoryCurrent: 13841203, MemoryPeak: 20971520,
			TasksCurrent: 5, Uptime: "1d 3h", ExecStart: "/usr/sbin/php-fpm8.5 --nodaemonize --fpm-config /etc/php/8.5/fpm/php-fpm.conf",
			Restart: "on-failure",
		},
		{
			Unit: "mysql.service", Name: "mysql", Label: "MySQL Community Server",
			Description: "MySQL Community Server", Category: "database", CategoryLabel: "Database & Cache",
			Protected: false, LoadState: "loaded", ActiveState: "active", SubState: "running",
			UnitFileState: "enabled", MainPID: 44845, MemoryCurrent: 148234240, MemoryPeak: 367001600,
			TasksCurrent: 36, Uptime: "1d 3h", ExecStart: "/usr/sbin/mysqld",
			Restart: "on-failure",
		},
		{
			Unit: "supervisor.service", Name: "supervisor", Label: "Supervisord Process Controller",
			Description: "Supervisor process control system for UNIX", Category: "supervisor", CategoryLabel: "Supervisor & Worker",
			Protected: false, LoadState: "loaded", ActiveState: "active", SubState: "running",
			UnitFileState: "enabled", MainPID: 51201, MemoryCurrent: 18432000, MemoryPeak: 25165824,
			TasksCurrent: 6, Uptime: "12j 10m", ExecStart: "/usr/bin/supervisord -n -c /etc/supervisor/supervisord.conf",
			Restart: "always",
		},
		{
			Unit: "fail2ban.service", Name: "fail2ban", Label: "Fail2Ban Intrusion Prevention",
			Description: "Fail2Ban Service", Category: "security", CategoryLabel: "Keamanan & Jaringan",
			Protected: false, LoadState: "loaded", ActiveState: "active", SubState: "running",
			UnitFileState: "enabled", MainPID: 44807, MemoryCurrent: 19713228, MemoryPeak: 22020096,
			TasksCurrent: 5, Uptime: "1d 3h", ExecStart: "/usr/bin/fail2ban-server -xf start",
			Restart: "on-failure",
		},
		{
			Unit: "cron.service", Name: "cron", Label: "Cron Scheduled Task Daemon",
			Description: "Regular background program processing daemon", Category: "system", CategoryLabel: "Sistem & Daemon",
			Protected: false, LoadState: "loaded", ActiveState: "active", SubState: "running",
			UnitFileState: "enabled", MainPID: 712, MemoryCurrent: 1153433, MemoryPeak: 2097152,
			TasksCurrent: 1, Uptime: "1d 21h", ExecStart: "/usr/sbin/cron -f",
			Restart: "on-failure",
		},
		{
			Unit: "ufw.service", Name: "ufw", Label: "UFW Uncomplicated Firewall",
			Description: "CLI tooling for managing a netfilter firewall", Category: "security", CategoryLabel: "Keamanan & Jaringan",
			Protected: false, LoadState: "loaded", ActiveState: "active", SubState: "exited",
			UnitFileState: "enabled", MainPID: 0, MemoryCurrent: 0, MemoryPeak: 0,
			TasksCurrent: 0, Uptime: "1d 21h", ExecStart: "/lib/ufw/ufw-init start",
			Restart: "no",
		},
	}

	var filtered []ServiceInfo
	var counts ServiceCounts

	for _, s := range mocks {
		counts.Total++
		if s.ActiveState == "active" {
			counts.Running++
		} else if s.ActiveState == "failed" {
			counts.Failed++
		} else {
			counts.Stopped++
		}
		counts.TotalMemory += s.MemoryCurrent

		if category != "" && category != "all" && s.Category != category {
			continue
		}

		if search != "" {
			q := strings.ToLower(search)
			match := strings.Contains(strings.ToLower(s.Unit), q) ||
				strings.Contains(strings.ToLower(s.Label), q) ||
				strings.Contains(strings.ToLower(s.Description), q)
			if !match {
				continue
			}
		}

		filtered = append(filtered, s)
	}

	return filtered, counts, nil
}
