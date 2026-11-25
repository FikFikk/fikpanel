package store

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// UptimeCheck merepresentasikan satu titik pemeriksaan heartbeat Uptime/SSL
type UptimeCheck struct {
	ID         int64  `json:"id"`
	MonitorID  int64  `json:"monitor_id"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int64  `json:"latency_ms"`
	SSLDays    int    `json:"ssl_days"`
	ErrorMsg   string `json:"error_msg,omitempty"`
	CheckedAt  int64  `json:"checked_at"`
}

// UptimeMonitor merepresentasikan target pemantauan HTTP/HTTPS/TCP & sertifikat SSL
type UptimeMonitor struct {
	ID               int64         `json:"id"`
	Name             string        `json:"name"`
	Target           string        `json:"target"`
	MonitorType      string        `json:"monitor_type"`
	IntervalSec      int           `json:"interval_sec"`
	TimeoutSec       int           `json:"timeout_sec"`
	ExpectedStatus   int           `json:"expected_status"`
	Keyword          string        `json:"keyword"`
	NotifyWebhook    bool          `json:"notify_webhook"`
	PublicVisible    bool          `json:"public_visible"`
	Enabled          bool          `json:"enabled"`
	LastStatus       string        `json:"last_status"`
	LastCode         int           `json:"last_code"`
	LastLatencyMs    int64         `json:"last_latency_ms"`
	LastError        string        `json:"last_error"`
	SSLDaysRemaining int           `json:"ssl_days_remaining"`
	SSLIssuer        string        `json:"ssl_issuer"`
	LastCheckedAt    int64         `json:"last_checked_at"`
	LastNotifiedAt   int64         `json:"last_notified_at"`
	CreatedAt        int64         `json:"created_at"`
	Uptime24hPct     float64       `json:"uptime_24h_pct"`
	AvgLatencyMs     int64         `json:"avg_latency_ms"`
	RecentChecks     []UptimeCheck `json:"recent_checks"`
}

// ListUptimeMonitors mengembalikan seluruh monitor beserta 30 heartbeat terakhir dan rasio uptime 24 jam
func (s *Store) ListUptimeMonitors(ctx context.Context, publicOnly bool) ([]UptimeMonitor, error) {
	query := `SELECT id, name, target, monitor_type, interval_sec, timeout_sec, expected_status, keyword,
	                 notify_webhook, public_visible, enabled, last_status, last_code, last_latency_ms,
	                 last_error, ssl_days_remaining, ssl_issuer, last_checked_at, last_notified_at, created_at
	          FROM uptime_monitors`
	if publicOnly {
		query += " WHERE enabled = 1 AND public_visible = 1"
	}
	query += " ORDER BY id ASC"

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca daftar monitor uptime: %w", err)
	}
	defer rows.Close()

	list := make([]UptimeMonitor, 0)
	for rows.Next() {
		var m UptimeMonitor
		var nHook, pubVis, en int
		if err := rows.Scan(
			&m.ID, &m.Name, &m.Target, &m.MonitorType, &m.IntervalSec, &m.TimeoutSec, &m.ExpectedStatus, &m.Keyword,
			&nHook, &pubVis, &en, &m.LastStatus, &m.LastCode, &m.LastLatencyMs,
			&m.LastError, &m.SSLDaysRemaining, &m.SSLIssuer, &m.LastCheckedAt, &m.LastNotifiedAt, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		m.NotifyWebhook = nHook == 1
		m.PublicVisible = pubVis == 1
		m.Enabled = en == 1
		list = append(list, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Lengkapi setiap monitor dengan 30 heartbeat terbaru & statistik 24 jam
	since24h := time.Now().Add(-24 * time.Hour).Unix()
	for i := range list {
		checks, _ := s.listRecentUptimeChecks(ctx, list[i].ID, 30)
		list[i].RecentChecks = checks
		pct, avgLat := s.calcUptimeStats(ctx, list[i].ID, since24h)
		list[i].Uptime24hPct = pct
		list[i].AvgLatencyMs = avgLat
	}
	return list, nil
}

func (s *Store) listRecentUptimeChecks(ctx context.Context, monitorID int64, limit int) ([]UptimeCheck, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, monitor_id, status, status_code, latency_ms, ssl_days, error_msg, checked_at
		 FROM uptime_checks WHERE monitor_id = ? ORDER BY checked_at DESC LIMIT ?`, monitorID, limit)
	if err != nil {
		return []UptimeCheck{}, err
	}
	defer rows.Close()

	desc := make([]UptimeCheck, 0, limit)
	for rows.Next() {
		var c UptimeCheck
		if err := rows.Scan(&c.ID, &c.MonitorID, &c.Status, &c.StatusCode, &c.LatencyMs, &c.SSLDays, &c.ErrorMsg, &c.CheckedAt); err == nil {
			desc = append(desc, c)
		}
	}
	// Balik urutan menjadi kronologis lama -> terbaru agar bar heartbeat mengalir kiri ke kanan
	for l, r := 0, len(desc)-1; l < r; l, r = l+1, r-1 {
		desc[l], desc[r] = desc[r], desc[l]
	}
	return desc, nil
}

func (s *Store) calcUptimeStats(ctx context.Context, monitorID int64, sinceUnix int64) (float64, int64) {
	var total, upCount int
	var avgLat float64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(1),
		        COALESCE(SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END), 0),
		        COALESCE(AVG(CASE WHEN status = 'up' THEN latency_ms END), 0)
		 FROM uptime_checks WHERE monitor_id = ? AND checked_at >= ?`,
		monitorID, sinceUnix).Scan(&total, &upCount, &avgLat)
	if err != nil || total == 0 {
		return 100.0, 0
	}
	pct := (float64(upCount) / float64(total)) * 100.0
	return float64(int(pct*100+0.5)) / 100.0, int64(avgLat + 0.5)
}

// GetUptimeMonitor mengambil satu entri monitor berdasarkan ID
func (s *Store) GetUptimeMonitor(ctx context.Context, id int64) (*UptimeMonitor, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, target, monitor_type, interval_sec, timeout_sec, expected_status, keyword,
		        notify_webhook, public_visible, enabled, last_status, last_code, last_latency_ms,
		        last_error, ssl_days_remaining, ssl_issuer, last_checked_at, last_notified_at, created_at
		 FROM uptime_monitors WHERE id = ?`, id)
	var m UptimeMonitor
	var nHook, pubVis, en int
	if err := row.Scan(
		&m.ID, &m.Name, &m.Target, &m.MonitorType, &m.IntervalSec, &m.TimeoutSec, &m.ExpectedStatus, &m.Keyword,
		&nHook, &pubVis, &en, &m.LastStatus, &m.LastCode, &m.LastLatencyMs,
		&m.LastError, &m.SSLDaysRemaining, &m.SSLIssuer, &m.LastCheckedAt, &m.LastNotifiedAt, &m.CreatedAt,
	); err != nil {
		return nil, err
	}
	m.NotifyWebhook = nHook == 1
	m.PublicVisible = pubVis == 1
	m.Enabled = en == 1
	m.RecentChecks, _ = s.listRecentUptimeChecks(ctx, m.ID, 30)
	m.Uptime24hPct, m.AvgLatencyMs = s.calcUptimeStats(ctx, m.ID, time.Now().Add(-24*time.Hour).Unix())
	return &m, nil
}

// CreateUptimeMonitor menambahkan monitor uptime baru
func (s *Store) CreateUptimeMonitor(ctx context.Context, m *UptimeMonitor) error {
	now := time.Now().Unix()
	m.CreatedAt = now
	if m.LastStatus == "" {
		m.LastStatus = "pending"
	}
	if m.SSLDaysRemaining == 0 {
		m.SSLDaysRemaining = -1
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO uptime_monitors (name, target, monitor_type, interval_sec, timeout_sec, expected_status, keyword,
		        notify_webhook, public_visible, enabled, last_status, ssl_days_remaining, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, -1, ?)`,
		m.Name, m.Target, m.MonitorType, m.IntervalSec, m.TimeoutSec, m.ExpectedStatus, m.Keyword,
		boolToInt(m.NotifyWebhook), boolToInt(m.PublicVisible), boolToInt(m.Enabled), m.LastStatus, now)
	if err != nil {
		return fmt.Errorf("gagal menambahkan monitor uptime: %w", err)
	}
	m.ID, _ = res.LastInsertId()
	m.RecentChecks = []UptimeCheck{}
	return nil
}

// UpdateUptimeMonitor memperbarui konfigurasi monitor yang sudah ada
func (s *Store) UpdateUptimeMonitor(ctx context.Context, m *UptimeMonitor) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE uptime_monitors
		 SET name = ?, target = ?, monitor_type = ?, interval_sec = ?, timeout_sec = ?, expected_status = ?,
		     keyword = ?, notify_webhook = ?, public_visible = ?, enabled = ?
		 WHERE id = ?`,
		m.Name, m.Target, m.MonitorType, m.IntervalSec, m.TimeoutSec, m.ExpectedStatus,
		m.Keyword, boolToInt(m.NotifyWebhook), boolToInt(m.PublicVisible), boolToInt(m.Enabled), m.ID)
	return err
}

// DeleteUptimeMonitor menghapus monitor beserta seluruh histori heartbeat-nya
func (s *Store) DeleteUptimeMonitor(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM uptime_monitors WHERE id = ?", id)
	return err
}

// RecordUptimeCheck mencatat hasil probe heartbeat terbaru, memperbarui status monitor, dan memangkas histori >7 hari
func (s *Store) RecordUptimeCheck(ctx context.Context, monitorID int64, status string, code int, latencyMs int64, sslDays int, sslIssuer, errMsg string, notifiedAt int64) error {
	now := time.Now().Unix()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO uptime_checks (monitor_id, status, status_code, latency_ms, ssl_days, error_msg, checked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		monitorID, status, code, latencyMs, sslDays, errMsg, now); err != nil {
		return err
	}

	if notifiedAt > 0 {
		_, err := s.db.ExecContext(ctx,
			`UPDATE uptime_monitors
			 SET last_status = ?, last_code = ?, last_latency_ms = ?, last_error = ?,
			     ssl_days_remaining = ?, ssl_issuer = ?, last_checked_at = ?, last_notified_at = ?
			 WHERE id = ?`,
			status, code, latencyMs, errMsg, sslDays, sslIssuer, now, notifiedAt, monitorID)
		if err != nil {
			return err
		}
	} else {
		_, err := s.db.ExecContext(ctx,
			`UPDATE uptime_monitors
			 SET last_status = ?, last_code = ?, last_latency_ms = ?, last_error = ?,
			     ssl_days_remaining = ?, ssl_issuer = ?, last_checked_at = ?
			 WHERE id = ?`,
			status, code, latencyMs, errMsg, sslDays, sslIssuer, now, monitorID)
		if err != nil {
			return err
		}
	}

	// Pertahankan retensi 7 hari agar ukuran panel.db tetap ringan
	cutoff := now - 7*86400
	_, _ = s.db.ExecContext(ctx, "DELETE FROM uptime_checks WHERE monitor_id = ? AND checked_at < ?", monitorID, cutoff)
	return nil
}

// UptimeMonitorHistory menyajikan detail analitik latensi (Min/Avg/P95/Max) dan riwayat panjang pemeriksaan
type UptimeMonitorHistory struct {
	Monitor      *UptimeMonitor `json:"monitor"`
	Checks       []UptimeCheck  `json:"checks"`
	Uptime24hPct float64        `json:"uptime_24h_pct"`
	Uptime7dPct  float64        `json:"uptime_7d_pct"`
	MinLatencyMs int64          `json:"min_latency_ms"`
	AvgLatencyMs int64          `json:"avg_latency_ms"`
	P95LatencyMs int64          `json:"p95_latency_ms"`
	MaxLatencyMs int64          `json:"max_latency_ms"`
	DownCount24h int            `json:"down_count_24h"`
}

// GetUptimeMonitorHistory membaca hingga 120 pemeriksaan terakhir beserta persentil latensi dan SLA 24j/7h
func (s *Store) GetUptimeMonitorHistory(ctx context.Context, id int64, limit int) (*UptimeMonitorHistory, error) {
	if limit <= 0 || limit > 200 {
		limit = 120
	}
	mon, err := s.GetUptimeMonitor(ctx, id)
	if err != nil {
		return nil, err
	}
	checks, err := s.listRecentUptimeChecks(ctx, id, limit)
	if err != nil {
		checks = []UptimeCheck{}
	}

	now := time.Now()
	pct24h, avg24h := s.calcUptimeStats(ctx, id, now.Add(-24*time.Hour).Unix())
	pct7d, _ := s.calcUptimeStats(ctx, id, now.Add(-7*24*time.Hour).Unix())

	var down24h int
	_ = s.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM uptime_checks WHERE monitor_id = ? AND status = 'down' AND checked_at >= ?`,
		id, now.Add(-24*time.Hour).Unix()).Scan(&down24h)

	lats := make([]int64, 0, len(checks))
	for _, c := range checks {
		if c.Status == "up" && c.LatencyMs > 0 {
			lats = append(lats, c.LatencyMs)
		}
	}
	var minL, maxL, p95L int64
	if len(lats) > 0 {
		sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
		minL = lats[0]
		maxL = lats[len(lats)-1]
		idx95 := int(math.Ceil(float64(len(lats))*0.95)) - 1
		if idx95 < 0 {
			idx95 = 0
		} else if idx95 >= len(lats) {
			idx95 = len(lats) - 1
		}
		p95L = lats[idx95]
	}

	return &UptimeMonitorHistory{
		Monitor:      mon,
		Checks:       checks,
		Uptime24hPct: pct24h,
		Uptime7dPct:  pct7d,
		MinLatencyMs: minL,
		AvgLatencyMs: avg24h,
		P95LatencyMs: p95L,
		MaxLatencyMs: maxL,
		DownCount24h: down24h,
	}, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
