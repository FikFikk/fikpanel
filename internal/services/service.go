package services

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os/exec"
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
	return nil
}

// ListServices mengambil seluruh daftar layanan terkelola beserta status aktifnya
func (m *Manager) ListServices(ctx context.Context) ([]ServiceInfo, error) {
	rows, err := m.db.QueryContext(ctx, "SELECT unit, coalesce(label, ''), protected FROM managed_services ORDER BY unit ASC")
	if err != nil {
		return nil, fmt.Errorf("gagal query tabel managed_services: %w", err)
	}
	defer rows.Close()

	var result []ServiceInfo
	for rows.Next() {
		var s ServiceInfo
		var protectedInt int
		if err := rows.Scan(&s.Unit, &s.Label, &protectedInt); err != nil {
			continue
		}
		s.Protected = (protectedInt == 1)

		// Periksa status terkini via systemctl
		m.populateStatus(ctx, &s)
		result = append(result, s)
	}

	return result, nil
}

// populateStatus mengisi informasi status runtime ActiveState, SubState, MainPID, dan MemoryCurrent
func (m *Manager) populateStatus(ctx context.Context, s *ServiceInfo) {
	if runtime.GOOS != "linux" {
		// Mock data realistis untuk pengujian di lingkungan development Windows
		s.ActiveState = "active"
		s.SubState = "running"
		s.MainPID = 1234
		s.MemoryCurrent = 18 * 1024 * 1024
		return
	}

	// Pada Linux, panggil systemctl show dengan argumen tetap tanpa shell string
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "systemctl", "show", s.Unit,
		"--property=ActiveState,SubState,MainPID,MemoryCurrent")
	out, err := cmd.Output()
	if err != nil {
		s.ActiveState = "unknown"
		s.SubState = "error"
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
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
}

