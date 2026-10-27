package php

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
	"strconv"
	"strings"
)

type PHPVersionInfo struct {
	Version   string            `json:"version"`   // misal: "8.5", "8.3", "8.2", "8.1"
	Installed bool              `json:"installed"` // true/false
	Running   bool              `json:"running"`   // FPM running
	FpmSocket string            `json:"fpm_socket"`
	IniPath   string            `json:"ini_path"`
	Settings  map[string]string `json:"settings"` // upload_max_filesize, memory_limit, max_execution_time
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

// isPHPVersionFormat memeriksa apakah string memiliki format angka versi X.Y
func isPHPVersionFormat(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	return err1 == nil && err2 == nil && major >= 5 && major <= 9 && minor >= 0 && minor <= 99
}

// comparePHPVersions membandingkan dua versi PHP secara numerik
func comparePHPVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	if len(partsA) != 2 || len(partsB) != 2 {
		return strings.Compare(a, b)
	}
	majA, _ := strconv.Atoi(partsA[0])
	minA, _ := strconv.Atoi(partsA[1])
	majB, _ := strconv.Atoi(partsB[0])
	minB, _ := strconv.Atoi(partsB[1])
	if majA != majB {
		return majA - majB
	}
	return minA - minB
}

// discoverInstalledVersions memindai seluruh versi PHP nyata yang terpasang pada sistem host Linux
func (m *Manager) discoverInstalledVersions() []string {
	var installed []string
	seen := make(map[string]bool)

	// 1. Pindai folder konfigurasi /etc/php/*
	if entries, err := os.ReadDir("/etc/php"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				ver := e.Name()
				if isPHPVersionFormat(ver) && !seen[ver] {
					seen[ver] = true
					installed = append(installed, ver)
				}
			}
		}
	}

	// 2. Pindai biner /usr/sbin/php-fpm*
	if matches, err := filepath.Glob("/usr/sbin/php-fpm*"); err == nil {
		for _, p := range matches {
			base := filepath.Base(p)
			ver := strings.TrimPrefix(base, "php-fpm")
			if isPHPVersionFormat(ver) && !seen[ver] {
				seen[ver] = true
				installed = append(installed, ver)
			}
		}
	}

	// 3. Pindai soket runtime /run/php/php*-fpm.sock
	if matches, err := filepath.Glob("/run/php/php*-fpm.sock"); err == nil {
		re := regexp.MustCompile(`php([0-9.]+)-fpm\.sock`)
		for _, p := range matches {
			base := filepath.Base(p)
			sub := re.FindStringSubmatch(base)
			if len(sub) > 1 && isPHPVersionFormat(sub[1]) && !seen[sub[1]] {
				seen[sub[1]] = true
				installed = append(installed, sub[1])
			}
		}
	}

	// 4. Pindai versi CLI aktif (php -v)
	if out, err := exec.Command("php", "-r", "echo PHP_MAJOR_VERSION.'.'.PHP_MINOR_VERSION;").Output(); err == nil {
		ver := strings.TrimSpace(string(out))
		if isPHPVersionFormat(ver) && !seen[ver] {
			seen[ver] = true
			installed = append(installed, ver)
		}
	}

	return installed
}

func (m *Manager) ListVersions(ctx context.Context) ([]PHPVersionInfo, error) {
	// Versi kandidat standar yang umum digunakan di server web
	candidates := []string{"8.5", "8.4", "8.3", "8.2", "8.1", "8.0", "7.4"}
	seen := make(map[string]bool)
	var allVersions []string

	// Temukan versi PHP terpasang secara dinamis di sistem host Linux
	if runtime.GOOS == "linux" {
		installed := m.discoverInstalledVersions()
		for _, v := range installed {
			if !seen[v] {
				seen[v] = true
				allVersions = append(allVersions, v)
			}
		}
	}

	// Gabungkan dengan kandidat versi umum lainnya
	for _, v := range candidates {
		if !seen[v] {
			seen[v] = true
			allVersions = append(allVersions, v)
		}
	}

	// Urutkan versi secara menurun (versi terbaru berada di atas/awal)
	sort.Slice(allVersions, func(i, j int) bool {
		return comparePHPVersions(allVersions[i], allVersions[j]) > 0
	})

	var list []PHPVersionInfo
	for _, ver := range allVersions {
		info := PHPVersionInfo{
			Version:   ver,
			Settings:  make(map[string]string),
			FpmSocket: fmt.Sprintf("/run/php/php%s-fpm.sock", ver),
			IniPath:   fmt.Sprintf("/etc/php/%s/fpm/php.ini", ver),
		}

		if runtime.GOOS != "linux" {
			info.Installed = ver == "8.5" || ver == "8.3"
			info.Running = ver == "8.5"
			info.Settings["memory_limit"] = "256M"
			info.Settings["upload_max_filesize"] = "64M"
			info.Settings["post_max_size"] = "64M"
			info.Settings["max_execution_time"] = "300"
			list = append(list, info)
			continue
		}

		// Cek apakah biner / direktori konfigurasi / soket php-fpm ada
		binPath := fmt.Sprintf("/usr/sbin/php-fpm%s", ver)
		sockPath := fmt.Sprintf("/run/php/php%s-fpm.sock", ver)
		if _, err := os.Stat(binPath); err == nil {
			info.Installed = true
		} else if _, err := os.Stat(info.IniPath); err == nil {
			info.Installed = true
		} else if _, err := os.Stat(sockPath); err == nil {
			info.Installed = true
		}

		// Cek apakah service running
		serviceName := fmt.Sprintf("php%s-fpm", ver)
		cmd := exec.CommandContext(ctx, "systemctl", "is-active", serviceName)
		out, _ := cmd.Output()
		info.Running = strings.TrimSpace(string(out)) == "active"
		if info.Running {
			info.Installed = true
		}

		if info.Installed {
			// Cari lokasi php.ini jika path default fpm belum ada
			if _, err := os.Stat(info.IniPath); err != nil {
				altPath := fmt.Sprintf("/etc/php/%s/cli/php.ini", ver)
				if _, err := os.Stat(altPath); err == nil {
					info.IniPath = altPath
				}
			}

			// Baca settings penting dari php.ini
			if iniFile, err := os.Open(info.IniPath); err == nil {
				scanner := bufio.NewScanner(iniFile)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if strings.HasPrefix(line, ";") || !strings.Contains(line, "=") {
						continue
					}
					parts := strings.SplitN(line, "=", 2)
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					switch key {
					case "memory_limit", "upload_max_filesize", "post_max_size", "max_execution_time":
						info.Settings[key] = val
					}
				}
				iniFile.Close()
			}
		}

		list = append(list, info)
	}

	return list, nil
}

type PHPExtensionInfo struct {
	Name             string `json:"name"`
	Type             string `json:"type"` // "cache", "database", "graphics", "network", "system", "general"
	Description      string `json:"description"`
	Installed        bool   `json:"installed"`
	Enabled          bool   `json:"enabled"`
	PackageName      string `json:"package_name"`
	InstallCommand   string `json:"install_command"`
	UninstallCommand string `json:"uninstall_command"`
	IsCore           bool   `json:"is_core"`
}

type PHPVersionDetail struct {
	Version           string                 `json:"version"`
	Installed         bool                   `json:"installed"`
	Running           bool                   `json:"running"`
	FpmSocket         string                 `json:"fpm_socket"`
	IniPath           string                 `json:"ini_path"`
	PoolConfPath      string                 `json:"pool_conf_path"`
	LogPath           string                 `json:"log_path"`
	SlowLogPath       string                 `json:"slow_log_path"`
	ServiceStatus     map[string]interface{} `json:"service_status"`
	Settings          map[string]string      `json:"settings"`
	FpmSettings       map[string]string      `json:"fpm_settings"`
	OpcacheSettings   map[string]string      `json:"opcache_settings"`
	DisabledFunctions []string               `json:"disabled_functions"`
	LoadedExtensions  []string               `json:"loaded_extensions"`
	AllExtensions     []PHPExtensionInfo     `json:"all_extensions"`
}

func (m *Manager) RestartFPM(ctx context.Context, ver string) error {
	return m.ControlService(ctx, ver, "restart")
}

// ControlService mengontrol status daemon systemd php-fpm (start, stop, restart, reload)
func (m *Manager) ControlService(ctx context.Context, ver string, action string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	validActions := map[string]bool{"start": true, "stop": true, "restart": true, "reload": true}
	if !validActions[action] {
		return fmt.Errorf("aksi layanan %q tidak valid", action)
	}
	serviceName := fmt.Sprintf("php%s-fpm", ver)
	cmd := exec.CommandContext(ctx, "systemctl", action, serviceName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal mengeksekusi %s %s: %s %w", action, serviceName, string(out), err)
	}
	return nil
}

// resolveIniPath mencari letak berkas konfigurasi php.ini aktif
func (m *Manager) resolveIniPath(ver string) string {
	fpmIni := fmt.Sprintf("/etc/php/%s/fpm/php.ini", ver)
	if _, err := os.Stat(fpmIni); err == nil {
		return fpmIni
	}
	cliIni := fmt.Sprintf("/etc/php/%s/cli/php.ini", ver)
	if _, err := os.Stat(cliIni); err == nil {
		return cliIni
	}
	return fpmIni
}

// resolvePoolPath mencari letak berkas konfigurasi pool FPM www.conf
func (m *Manager) resolvePoolPath(ver string) string {
	poolPath := fmt.Sprintf("/etc/php/%s/fpm/pool.d/www.conf", ver)
	if _, err := os.Stat(poolPath); err == nil {
		return poolPath
	}
	return poolPath
}

// GetVersionDetail mengambil seluruh informasi komprehensif versi PHP aktif
func (m *Manager) GetVersionDetail(ctx context.Context, ver string) (*PHPVersionDetail, error) {
	iniPath := m.resolveIniPath(ver)
	poolPath := m.resolvePoolPath(ver)
	logPath := fmt.Sprintf("/var/log/php%s-fpm.log", ver)
	slowLogPath := fmt.Sprintf("/var/log/php%s-fpm.slow.log", ver)

	detail := &PHPVersionDetail{
		Version:           ver,
		IniPath:           iniPath,
		PoolConfPath:      poolPath,
		FpmSocket:         fmt.Sprintf("/run/php/php%s-fpm.sock", ver),
		LogPath:           logPath,
		SlowLogPath:       slowLogPath,
		ServiceStatus:     make(map[string]interface{}),
		Settings:          make(map[string]string),
		FpmSettings:       make(map[string]string),
		OpcacheSettings:   make(map[string]string),
		DisabledFunctions: []string{},
		LoadedExtensions:  []string{},
		AllExtensions:     []PHPExtensionInfo{},
	}

	// Nilai default populer
	detail.Settings["memory_limit"] = "256M"
	detail.Settings["upload_max_filesize"] = "64M"
	detail.Settings["post_max_size"] = "64M"
	detail.Settings["max_execution_time"] = "300"
	detail.Settings["max_input_time"] = "300"
	detail.Settings["display_errors"] = "Off"
	detail.Settings["error_reporting"] = "E_ALL & ~E_DEPRECATED & ~E_STRICT"
	detail.Settings["date.timezone"] = "UTC"
	detail.Settings["default_charset"] = "UTF-8"

	detail.FpmSettings["pm"] = "dynamic"
	detail.FpmSettings["pm.max_children"] = "50"
	detail.FpmSettings["pm.start_servers"] = "5"
	detail.FpmSettings["pm.min_spare_servers"] = "5"
	detail.FpmSettings["pm.max_spare_servers"] = "35"
	detail.FpmSettings["pm.max_requests"] = "500"

	detail.OpcacheSettings["opcache.enable"] = "1"
	detail.OpcacheSettings["opcache.memory_consumption"] = "128"
	detail.OpcacheSettings["opcache.max_accelerated_files"] = "10000"
	detail.OpcacheSettings["opcache.validate_timestamps"] = "1"
	detail.OpcacheSettings["opcache.revalidate_freq"] = "2"

	if runtime.GOOS != "linux" {
		detail.Installed = true
		detail.Running = true
		detail.DisabledFunctions = []string{"exec", "passthru", "shell_exec", "system", "proc_open", "popen"}
		detail.LoadedExtensions = []string{"Core", "date", "libxml", "openssl", "pcre", "zlib", "filter", "hash", "json", "SPL", "session", "standard", "mysqlnd", "PDO", "pdo_mysql", "curl", "fileinfo", "gd", "mbstring", "Zend OPcache"}
		detail.AllExtensions = m.getDefaultExtensionList(ver, detail.LoadedExtensions)
		detail.ServiceStatus["active"] = true
		detail.ServiceStatus["pid"] = 18726
		detail.ServiceStatus["memory"] = "14.2 MB"
		detail.ServiceStatus["uptime"] = "19 jam"
		return detail, nil
	}

	// 1. Cek status systemd service
	serviceName := fmt.Sprintf("php%s-fpm", ver)
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", serviceName)
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) == "active" {
		detail.Running = true
		detail.Installed = true
		detail.ServiceStatus["active"] = true
	}

	if _, err := os.Stat(iniPath); err == nil {
		detail.Installed = true
	}

	// Ambil PID & memory service jika running
	if detail.Running {
		cmdPid := exec.CommandContext(ctx, "systemctl", "show", serviceName, "--property=MainPID,ActiveEnterTimestamp")
		if out, err := cmdPid.Output(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, l := range lines {
				if strings.HasPrefix(l, "MainPID=") {
					pidStr := strings.TrimPrefix(l, "MainPID=")
					detail.ServiceStatus["pid"] = strings.TrimSpace(pidStr)
				} else if strings.HasPrefix(l, "ActiveEnterTimestamp=") {
					detail.ServiceStatus["since"] = strings.TrimPrefix(l, "ActiveEnterTimestamp=")
				}
			}
		}
	}

	// 2. Baca isi php.ini
	if content, err := os.ReadFile(iniPath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, ";") || !strings.Contains(line, "=") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if idx := strings.Index(val, ";"); idx > 0 {
				val = strings.TrimSpace(val[:idx])
			}
			switch key {
			case "memory_limit", "upload_max_filesize", "post_max_size", "max_execution_time", "max_input_time",
				"display_errors", "error_reporting", "date.timezone", "default_charset":
				detail.Settings[key] = val
			case "disable_functions":
				valClean := strings.Trim(val, `"'`)
				if valClean != "" {
					fnList := strings.Split(valClean, ",")
					for _, fn := range fnList {
						trimmed := strings.TrimSpace(fn)
						if trimmed != "" {
							detail.DisabledFunctions = append(detail.DisabledFunctions, trimmed)
						}
					}
				}
			case "opcache.enable", "opcache.memory_consumption", "opcache.max_accelerated_files",
				"opcache.validate_timestamps", "opcache.revalidate_freq":
				detail.OpcacheSettings[key] = val
			}
		}
	}

	// 3. Baca isi pool www.conf
	if content, err := os.ReadFile(poolPath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, ";") || !strings.Contains(line, "=") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if idx := strings.Index(val, ";"); idx > 0 {
				val = strings.TrimSpace(val[:idx])
			}
			switch key {
			case "pm", "pm.max_children", "pm.start_servers", "pm.min_spare_servers", "pm.max_spare_servers", "pm.max_requests", "listen":
				detail.FpmSettings[key] = val
			}
		}
	}

	// 4. Deteksi modul terpasang via php CLI
	phpBin := "php"
	if _, err := exec.LookPath(fmt.Sprintf("php%s", ver)); err == nil {
		phpBin = fmt.Sprintf("php%s", ver)
	}
	cmdMods := exec.CommandContext(ctx, phpBin, "-m")
	if out, err := cmdMods.Output(); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			mod := strings.TrimSpace(scanner.Text())
			if mod != "" && !strings.HasPrefix(mod, "[") {
				detail.LoadedExtensions = append(detail.LoadedExtensions, mod)
			}
		}
	}

	detail.AllExtensions = m.getDefaultExtensionList(ver, detail.LoadedExtensions)
	return detail, nil
}

// ResolveExtensionPackage memetakan nama ekstensi ke nama paket APT dan menentukan apakah modul merupakan modul inti PHP
func ResolveExtensionPackage(version, extName string) (string, bool) {
	coreModules := map[string]bool{
		"core": true, "date": true, "pcre": true, "reflection": true,
		"spl": true, "hash": true, "standard": true, "fileinfo": true,
		"json": true, "filter": true, "session": true, "tokenizer": true,
		"opcache": true,
	}
	ext := strings.ToLower(extName)
	if coreModules[ext] {
		return fmt.Sprintf("php%s-common", version), true
	}

	switch ext {
	case "pdo_mysql", "mysqli", "mysql":
		return fmt.Sprintf("php%s-mysql", version), false
	case "pdo_pgsql", "pgsql":
		return fmt.Sprintf("php%s-pgsql", version), false
	case "sqlite3", "pdo_sqlite":
		return fmt.Sprintf("php%s-sqlite3", version), false
	default:
		return fmt.Sprintf("php%s-%s", version, ext), false
	}
}

// getDefaultExtensionList menyusun katalog ekstensi populer beserta status aktif dan perintah SSH
func (m *Manager) getDefaultExtensionList(ver string, loaded []string) []PHPExtensionInfo {
	loadedMap := make(map[string]bool)
	for _, l := range loaded {
		loadedMap[strings.ToLower(l)] = true
	}

	catalog := []struct {
		name string
		typ  string
		desc string
	}{
		{"opcache", "cache", "Zend OPcache bytecode cache untuk mempercepat eksekusi skrip PHP"},
		{"redis", "database", "Driver ekstensi koneksi in-memory database Redis berkecepatan tinggi"},
		{"memcached", "cache", "Ekstensi client Memcached untuk caching objek terdistribusi"},
		{"imagick", "graphics", "Pustaka ImageMagick untuk manipulasi dan pemrosesan grafis tingkat tinggi"},
		{"fileinfo", "system", "Deteksi MIME type dan karakter encoding berkas secara otomatis"},
		{"curl", "network", "Klien transfer data HTTP / HTTPS multi-protokol"},
		{"mbstring", "general", "Penanganan string multibyte Unicode (UTF-8, emoji, aksara non-latin)"},
		{"intl", "general", "Pustaka internasionalisasi ICU untuk lokalisasi tanggal dan mata uang"},
		{"gd", "graphics", "Pustaka manipulasi gambar dasar dan pembuatan thumbnail"},
		{"zip", "system", "Kompresi dan dekompresi berkas arsip ZIP"},
		{"bcmath", "general", "Kalkulasi matematika presisi bebas tanpa batas desimal"},
		{"xml", "general", "Parser dan manipulator dokumen XML, DOM, dan SimpleXML"},
		{"soap", "network", "Protokol pertukaran data SOAP Web Services"},
		{"sqlite3", "database", "Driver basis data tertanam SQLite lokal"},
		{"pdo_mysql", "database", "Driver PDO MySQL / MariaDB standar enterprise"},
		{"mysqli", "database", "Ekstensi koneksi relasional database MySQL"},
		{"xdebug", "general", "Alat debugging dan profiling kode PHP untuk developer"},
		{"swoole", "network", "Framework asinkronik berbasis event coroutine berkinerja tinggi"},
	}

	var res []PHPExtensionInfo
	for _, c := range catalog {
		isLoaded := loadedMap[c.name]
		if c.name == "opcache" && (loadedMap["zend opcache"] || loadedMap["opcache"]) {
			isLoaded = true
		}

		pkg, isCore := ResolveExtensionPackage(ver, c.name)
		installCmd := fmt.Sprintf("sudo DEBIAN_FRONTEND=noninteractive apt-get update && sudo apt-get install -y %s && sudo systemctl restart php%s-fpm", pkg, ver)
		uninstallCmd := fmt.Sprintf("sudo DEBIAN_FRONTEND=noninteractive apt-get remove -y %s && sudo systemctl restart php%s-fpm", pkg, ver)

		res = append(res, PHPExtensionInfo{
			Name:             c.name,
			Type:             c.typ,
			Description:      c.desc,
			Installed:        isLoaded,
			Enabled:          isLoaded,
			PackageName:      pkg,
			InstallCommand:   installCmd,
			UninstallCommand: uninstallCmd,
			IsCore:           isCore,
		})
	}
	return res
}

// GetRawIni membaca teks mentah berkas php.ini
func (m *Manager) GetRawIni(ctx context.Context, ver string) (string, string, error) {
	iniPath := m.resolveIniPath(ver)
	if runtime.GOOS != "linux" {
		sample := fmt.Sprintf(`[PHP]
;;;;;;;;;;;;;;;;;;;
; About php.ini   ;
;;;;;;;;;;;;;;;;;;;
; Berkas konfigurasi utama PHP %s dibuat otomatis oleh FikPanel
engine = On
short_open_tag = Off
precision = 14
output_buffering = 4096
zlib.output_compression = Off
implicit_flush = Off

max_execution_time = 300
max_input_time = 300
memory_limit = 256M

error_reporting = E_ALL & ~E_DEPRECATED & ~E_STRICT
display_errors = Off
display_startup_errors = Off
log_errors = On
log_errors_max_len = 1024
ignore_repeated_errors = Off
ignore_repeated_source = Off
report_memleaks = On

post_max_size = 64M
default_mimetype = "text/html"
default_charset = "UTF-8"

file_uploads = On
upload_max_filesize = 64M
max_file_uploads = 20

allow_url_fopen = On
allow_url_include = Off
default_socket_timeout = 60

disable_functions = exec,passthru,shell_exec,system,proc_open,popen

[Date]
date.timezone = UTC

[opcache]
opcache.enable = 1
opcache.memory_consumption = 128
opcache.max_accelerated_files = 10000
`, ver)
		return sample, iniPath, nil
	}

	data, err := os.ReadFile(iniPath)
	if err != nil {
		return "", iniPath, fmt.Errorf("gagal membaca berkas %s: %w", iniPath, err)
	}
	return string(data), iniPath, nil
}

// SaveRawIni menyimpan teks mentah ke berkas php.ini dengan backup otomatis
func (m *Manager) SaveRawIni(ctx context.Context, ver string, content string) error {
	iniPath := m.resolveIniPath(ver)
	if runtime.GOOS != "linux" {
		return nil
	}

	// Backup berkas lama jika ada
	if oldData, err := os.ReadFile(iniPath); err == nil {
		_ = os.WriteFile(iniPath+".bak", oldData, 0644)
	}

	if err := os.WriteFile(iniPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("gagal menulis ke berkas %s: %w", iniPath, err)
	}

	// Reload layanan agar perubahan langsung efektif
	_ = m.ControlService(ctx, ver, "reload")
	return nil
}

// GetRawFpm membaca teks mentah berkas www.conf (pool FPM)
func (m *Manager) GetRawFpm(ctx context.Context, ver string) (string, string, error) {
	poolPath := m.resolvePoolPath(ver)
	if runtime.GOOS != "linux" {
		sample := fmt.Sprintf(`[www]
user = www-data
group = www-data
listen = /run/php/php%s-fpm.sock
listen.owner = www-data
listen.group = www-data
listen.mode = 0660

pm = dynamic
pm.max_children = 50
pm.start_servers = 5
pm.min_spare_servers = 5
pm.max_spare_servers = 35
pm.max_requests = 500

pm.status_path = /status
ping.path = /ping
`, ver)
		return sample, poolPath, nil
	}

	data, err := os.ReadFile(poolPath)
	if err != nil {
		return "", poolPath, fmt.Errorf("gagal membaca berkas %s: %w", poolPath, err)
	}
	return string(data), poolPath, nil
}

// SaveRawFpm menyimpan teks mentah ke berkas www.conf pool FPM
func (m *Manager) SaveRawFpm(ctx context.Context, ver string, content string) error {
	poolPath := m.resolvePoolPath(ver)
	if runtime.GOOS != "linux" {
		return nil
	}

	if oldData, err := os.ReadFile(poolPath); err == nil {
		_ = os.WriteFile(poolPath+".bak", oldData, 0644)
	}

	if err := os.WriteFile(poolPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("gagal menulis ke berkas %s: %w", poolPath, err)
	}

	_ = m.ControlService(ctx, ver, "reload")
	return nil
}

// SaveConfigBatch memperbarui beberapa parameter konfigurasi php.ini sekaligus secara atomik
func (m *Manager) SaveConfigBatch(ctx context.Context, ver string, settings map[string]string) error {
	iniPath := m.resolveIniPath(ver)
	if runtime.GOOS != "linux" {
		return nil
	}

	content, err := os.ReadFile(iniPath)
	if err != nil {
		return fmt.Errorf("tidak dapat membuka php.ini: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	updatedKeys := make(map[string]bool)

	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, ";") || !strings.Contains(trimmed, "=") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		k := strings.TrimSpace(parts[0])
		if newVal, ok := settings[k]; ok {
			lines[i] = fmt.Sprintf("%s = %s", k, newVal)
			updatedKeys[k] = true
		}
	}

	// Sisipkan key baru jika belum ada di berkas
	for k, v := range settings {
		if !updatedKeys[k] {
			lines = append(lines, fmt.Sprintf("%s = %s", k, v))
		}
	}

	// Backup berkas lama
	_ = os.WriteFile(iniPath+".bak", content, 0644)

	if err := os.WriteFile(iniPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("gagal menyimpan konfigurasi php.ini: %w", err)
	}

	_ = m.ControlService(ctx, ver, "reload")
	return nil
}

// UpdateDisabledFunctions memperbarui direktif disable_functions di php.ini
func (m *Manager) UpdateDisabledFunctions(ctx context.Context, ver string, funcs []string) error {
	joined := strings.Join(funcs, ",")
	return m.SaveConfigBatch(ctx, ver, map[string]string{"disable_functions": joined})
}

// SaveFpmPoolBatch memperbarui parameter pool www.conf FPM
func (m *Manager) SaveFpmPoolBatch(ctx context.Context, ver string, settings map[string]string) error {
	poolPath := m.resolvePoolPath(ver)
	if runtime.GOOS != "linux" {
		return nil
	}

	content, err := os.ReadFile(poolPath)
	if err != nil {
		return fmt.Errorf("tidak dapat membuka pool www.conf: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	updatedKeys := make(map[string]bool)

	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, ";") || !strings.Contains(trimmed, "=") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		k := strings.TrimSpace(parts[0])
		if newVal, ok := settings[k]; ok {
			lines[i] = fmt.Sprintf("%s = %s", k, newVal)
			updatedKeys[k] = true
		}
	}

	for k, v := range settings {
		if !updatedKeys[k] {
			lines = append(lines, fmt.Sprintf("%s = %s", k, v))
		}
	}

	_ = os.WriteFile(poolPath+".bak", content, 0644)
	if err := os.WriteFile(poolPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("gagal menyimpan konfigurasi pool FPM: %w", err)
	}

	_ = m.ControlService(ctx, ver, "reload")
	return nil
}

// GetLogs membaca isi log FPM atau slow log
func (m *Manager) GetLogs(ctx context.Context, ver string, logType string) (string, error) {
	logPath := fmt.Sprintf("/var/log/php%s-fpm.log", ver)
	if logType == "slow" {
		logPath = fmt.Sprintf("/var/log/php%s-fpm.slow.log", ver)
	}

	if runtime.GOOS != "linux" {
		return fmt.Sprintf(`[%s] NOTICE: fpm is running, pid 18726
[%s] NOTICE: ready to handle connections
[%s] NOTICE: systemd monitor active
`, ver, ver, ver), nil
	}

	if _, err := os.Stat(logPath); err != nil {
		return fmt.Sprintf("Berkas log %s belum tersedia atau belum ada entri tercatat.", logPath), nil
	}

	// Gunakan tail -n 250 untuk performa maksimal
	cmd := exec.CommandContext(ctx, "tail", "-n", "250", logPath)
	out, err := cmd.Output()
	if err != nil {
		data, rErr := os.ReadFile(logPath)
		if rErr != nil {
			return "", fmt.Errorf("gagal membaca berkas log: %w", rErr)
		}
		return string(data), nil
	}
	return string(out), nil
}

// ClearLogs mengosongkan berkas log
func (m *Manager) ClearLogs(ctx context.Context, ver string, logType string) error {
	logPath := fmt.Sprintf("/var/log/php%s-fpm.log", ver)
	if logType == "slow" {
		logPath = fmt.Sprintf("/var/log/php%s-fpm.slow.log", ver)
	}
	if runtime.GOOS != "linux" {
		return nil
	}
	return os.WriteFile(logPath, []byte(""), 0644)
}

// GetPhpInfo menghasilkan output diagnostik phpinfo live
func (m *Manager) GetPhpInfo(ctx context.Context, ver string) (string, error) {
	phpBin := "php"
	if _, err := exec.LookPath(fmt.Sprintf("php%s", ver)); err == nil {
		phpBin = fmt.Sprintf("php%s", ver)
	}
	cmd := exec.CommandContext(ctx, phpBin, "-i")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("gagal menjalankan php -i: %w", err)
	}
	return string(out), nil
}

// UpdateSetting memperbarui satu setting pada php.ini (kompatibilitas handler lama)
func (m *Manager) UpdateSetting(ctx context.Context, ver string, key string, val string) error {
	return m.SaveConfigBatch(ctx, ver, map[string]string{key: val})
}

// InstallExtension memasang paket ekstensi PHP menggunakan manajer paket APT dan me-restart layanan FPM
func (m *Manager) InstallExtension(ctx context.Context, version, extName string) (string, error) {
	if runtime.GOOS != "linux" {
		return fmt.Sprintf("[Dev Mode] Ekstensi %s untuk PHP %s berhasil disimulasikan terpasang.", extName, version), nil
	}

	pkg, _ := ResolveExtensionPackage(version, extName)

	// Jalankan apt-get update & install
	cmd := exec.CommandContext(ctx, "apt-get", "install", "-y", "--no-install-recommends", pkg)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Jika paket spesifik versi gagal, coba fallback ke paket generik php-<ext>
		altPkg := fmt.Sprintf("php-%s", strings.ToLower(extName))
		cmdAlt := exec.CommandContext(ctx, "apt-get", "install", "-y", "--no-install-recommends", altPkg)
		cmdAlt.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		outAlt, errAlt := cmdAlt.CombinedOutput()
		if errAlt != nil {
			return string(out), fmt.Errorf("gagal memasang paket %s: %s (%w)", pkg, strings.TrimSpace(string(outAlt)), err)
		}
		pkg = altPkg
		out = outAlt
	}

	// Restart layanan FPM agar modul langsung aktif
	_ = m.ControlService(ctx, version, "restart")
	return fmt.Sprintf("Paket %s berhasil dipasang. %s", pkg, strings.TrimSpace(string(out))), nil
}

// UninstallExtension mencopot paket ekstensi PHP dan me-restart layanan FPM
func (m *Manager) UninstallExtension(ctx context.Context, version, extName string) (string, error) {
	if runtime.GOOS != "linux" {
		return fmt.Sprintf("[Dev Mode] Ekstensi %s untuk PHP %s berhasil disimulasikan dicopot.", extName, version), nil
	}

	pkg, isCore := ResolveExtensionPackage(version, extName)
	if isCore {
		return "", fmt.Errorf("ekstensi %s merupakan modul inti PHP dan tidak dapat dicopot secara mandiri", extName)
	}

	cmd := exec.CommandContext(ctx, "apt-get", "remove", "-y", pkg)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("gagal mencopot paket %s: %s (%w)", pkg, strings.TrimSpace(string(out)), err)
	}

	// Restart layanan FPM agar modul dinonaktifkan
	_ = m.ControlService(ctx, version, "restart")
	return fmt.Sprintf("Paket %s berhasil dicopot. %s", pkg, strings.TrimSpace(string(out))), nil
}
