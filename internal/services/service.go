package services

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ServiceInfo merepresentasikan status operasional unit layanan systemd
type ServiceInfo struct {
	Unit          string `json:"unit"`
	Label         string `json:"label"`
	Protected     bool   `json:"protected"`
	ActiveState   string `json:"active_state"`
	SubState      string `json:"sub_state"`
	MainPID       int    `json:"main_pid"`
	MemoryCurrent uint64 `json:"memory_current"`
}

// Manager mengelola allowlist dan pemantauan layanan sistem
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

// ListServices mengambil seluruh daftar layanan terkelola beserta status aktifnya
func (m *Manager) ListServices(ctx context.Context) ([]ServiceInfo, error) {
	// Lakukan penemuan service sistem baru pada host Linux
	_ = m.discoverInstalledServices(ctx)

	rows, err := m.db.QueryContext(ctx, "SELECT unit, coalesce(label, ''), protected FROM managed_services ORDER BY unit ASC")
	if err != nil {
		return nil, fmt.Errorf("gagal query tabel managed_services: %w", err)
	}
	defer rows.Close()

	var result []ServiceInfo
	var hasSSHD, hasSSH bool
	for rows.Next() {
		var s ServiceInfo
		var protectedInt int
		if err := rows.Scan(&s.Unit, &s.Label, &protectedInt); err != nil {
			continue
		}
		s.Protected = (protectedInt == 1)

		// Periksa status terkini via systemctl
		if !m.populateStatus(ctx, &s) {
			// Lewati unit yang tidak terpasang di sistem host
			continue
		}

		if s.Unit == "sshd.service" {
			hasSSHD = true
		} else if s.Unit == "ssh.service" {
			hasSSH = true
		}

		result = append(result, s)
	}

	// Jika sshd.service dan ssh.service sama-sama terdeteksi, hapus duplikat ssh.service
	if hasSSHD && hasSSH {
		var deduped []ServiceInfo
		for _, s := range result {
			if s.Unit == "ssh.service" {
				continue
			}
			deduped = append(deduped, s)
		}
		result = deduped
	}

	return result, nil
}

// populateStatus mengisi informasi status runtime ActiveState, SubState, MainPID, dan MemoryCurrent.
// Mengembalikan false jika unit tidak ditemukan pada host Linux (LoadState == "not-found").
func (m *Manager) populateStatus(ctx context.Context, s *ServiceInfo) bool {
	if runtime.GOOS != "linux" {
		// Mock data realistis untuk pengujian di lingkungan development Windows
		s.ActiveState = "active"
		s.SubState = "running"
		s.MainPID = 1234
		s.MemoryCurrent = 18 * 1024 * 1024
		return true
	}

	// Pada Linux, panggil systemctl show dengan argumen tetap tanpa shell string
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "systemctl", "show", s.Unit,
		"--property=LoadState,ActiveState,SubState,MainPID,MemoryCurrent")
	out, err := cmd.Output()
	if err != nil {
		s.ActiveState = "unknown"
		s.SubState = "error"
		return false
	}

	loadState := ""
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
		case "LoadState":
			loadState = val
		case "ActiveState":
			s.ActiveState = val
		case "SubState":
			s.SubState = val
		case "MainPID":
			s.MainPID, _ = strconv.Atoi(val)
		case "MemoryCurrent":
			s.MemoryCurrent, _ = strconv.ParseUint(val, 10, 64)
		}
	}

	if loadState == "not-found" {
		return false
	}
	return true
}

