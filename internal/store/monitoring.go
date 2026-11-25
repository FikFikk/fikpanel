package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

var _ = sql.ErrNoRows

// MonitorSnapshot menyimpan satu titik rekaman historis metrik server lengkap di SQLite
type MonitorSnapshot struct {
	ID        int64   `json:"id"`
	Timestamp int64   `json:"timestamp"`
	CPU       float64 `json:"cpu"`
	CPUIOWait float64 `json:"cpu_iowait"`
	CPUSteal  float64 `json:"cpu_steal"`
	RAMUsed   uint64  `json:"ram_used"`
	RAMTotal  uint64  `json:"ram_total"`
	Load1m    float64 `json:"load_1m"`
	Load5m    float64 `json:"load_5m"`
	Load15m   float64 `json:"load_15m"`
	DiskUsed  uint64  `json:"disk_used"`
	DiskTotal uint64  `json:"disk_total"`
	DiskRead  uint64  `json:"disk_read"`
	DiskWrite uint64  `json:"disk_write"`
	NetIn     uint64  `json:"net_in"`
	NetOut    uint64  `json:"net_out"`
}

// MonitorConfig menyimpan status aktif/nonaktif perekaman dan lama retensi histori
type MonitorConfig struct {
	Enabled  bool `json:"enabled"`
	SaveDays int  `json:"save_days"`
}

// MonitorAlert merepresentasikan satu catatan alert ambang resource atau insiden Uptime/SSL
type MonitorAlert struct {
	ID           int64   `json:"id"`
	Source       string  `json:"source"`   // cpu, ram, disk, uptime, ssl
	Severity     string  `json:"severity"` // critical, warning, recovered
	Target       string  `json:"target"`
	MetricValue  float64 `json:"metric_value"`
	Threshold    float64 `json:"threshold"`
	Message      string  `json:"message"`
	Notified     bool    `json:"notified"`
	Acknowledged bool    `json:"acknowledged"`
	CreatedAt    int64   `json:"created_at"`
}

// GetMonitorConfig membaca konfigurasi monitoring persisten dari monitor_settings
func (s *Store) GetMonitorConfig(ctx context.Context) (*MonitorConfig, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key, value FROM monitor_settings")
	if err != nil {
		return &MonitorConfig{Enabled: true, SaveDays: 30}, nil
	}
	defer rows.Close()

	cfg := &MonitorConfig{Enabled: true, SaveDays: 30}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			if k == "enabled" {
				cfg.Enabled = (v == "1" || v == "true")
			} else if k == "save_days" {
				var days int
				if _, err := fmt.Sscanf(v, "%d", &days); err == nil && days > 0 {
					cfg.SaveDays = days
				}
			}
		}
	}
	return cfg, nil
}

// UpdateMonitorConfig memperbarui status perekaman dan masa retensi hari secara atomik
func (s *Store) UpdateMonitorConfig(ctx context.Context, enabled bool, saveDays int) error {
	enVal := "0"
	if enabled {
		enVal = "1"
	}
	if saveDays < 1 {
		saveDays = 30
	} else if saveDays > 365 {
		saveDays = 365
	}
	now := time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO monitor_settings (key, value, updated_at) VALUES ('enabled', ?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at
	`, enVal, now)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO monitor_settings (key, value, updated_at) VALUES ('save_days', ?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at
	`, fmt.Sprintf("%d", saveDays), now)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// InsertMonitorSnapshot menyimpan satu sampel metrik sistem ke monitor_snapshots
func (s *Store) InsertMonitorSnapshot(ctx context.Context, snap *MonitorSnapshot) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_snapshots
		(timestamp, cpu, cpu_iowait, cpu_steal, ram_used, ram_total, load_1m, load_5m, load_15m,
		 disk_used, disk_total, disk_read, disk_write, net_in, net_out)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, snap.Timestamp, snap.CPU, snap.CPUIOWait, snap.CPUSteal, snap.RAMUsed, snap.RAMTotal,
		snap.Load1m, snap.Load5m, snap.Load15m, snap.DiskUsed, snap.DiskTotal,
		snap.DiskRead, snap.DiskWrite, snap.NetIn, snap.NetOut)
	return err
}

// GetMonitorHistory mengambil seluruh sampel metrik dalam rentang timestamp inklusif
func (s *Store) GetMonitorHistory(ctx context.Context, startTS, endTS int64) ([]MonitorSnapshot, error) {
	query := `
		SELECT id, timestamp, cpu, cpu_iowait, cpu_steal, ram_used, ram_total,
		       load_1m, load_5m, load_15m, disk_used, disk_total, disk_read, disk_write, net_in, net_out
		FROM monitor_snapshots
		WHERE timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`
	rows, err := s.db.QueryContext(ctx, query, startTS, endTS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]MonitorSnapshot, 0)
	for rows.Next() {
		var sn MonitorSnapshot
		if err := rows.Scan(
			&sn.ID, &sn.Timestamp, &sn.CPU, &sn.CPUIOWait, &sn.CPUSteal,
			&sn.RAMUsed, &sn.RAMTotal, &sn.Load1m, &sn.Load5m, &sn.Load15m,
			&sn.DiskUsed, &sn.DiskTotal, &sn.DiskRead, &sn.DiskWrite, &sn.NetIn, &sn.NetOut,
		); err == nil {
			result = append(result, sn)
		}
	}
	return result, nil
}

// PurgeMonitorSnapshots menghapus sampel lama melebihi batas hari retensi serta alert > 90 hari
func (s *Store) PurgeMonitorSnapshots(ctx context.Context) error {
	cfg, err := s.GetMonitorConfig(ctx)
	if err != nil {
		return err
	}
	cutoff := time.Now().AddDate(0, 0, -cfg.SaveDays).Unix()
	if _, err := s.db.ExecContext(ctx, "DELETE FROM monitor_snapshots WHERE timestamp < ?", cutoff); err != nil {
		return err
	}
	alertCutoff := time.Now().AddDate(0, 0, -90).Unix()
	_, _ = s.db.ExecContext(ctx, "DELETE FROM monitor_alerts WHERE created_at < ?", alertCutoff)
	return nil
}

// ClearAllMonitorSnapshots mengosongkan seluruh rekaman historis metrik
func (s *Store) ClearAllMonitorSnapshots(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM monitor_snapshots")
	return err
}

// ── Manajemen Riwayat Alert & Insiden Terpadu (monitor_alerts) ──────────────

// InsertMonitorAlert mencatat satu peristiwa alert/pemulihan baru ke monitor_alerts
func (s *Store) InsertMonitorAlert(ctx context.Context, a *MonitorAlert) error {
	if a.CreatedAt == 0 {
		a.CreatedAt = time.Now().Unix()
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_alerts (source, severity, target, metric_value, threshold, message, notified, acknowledged, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, a.Source, a.Severity, a.Target, a.MetricValue, a.Threshold, a.Message,
		boolToInt(a.Notified), boolToInt(a.Acknowledged), a.CreatedAt)
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

// ListMonitorAlerts mengambil daftar alert terbaru dengan filter sumber dan status belum dibaca
func (s *Store) ListMonitorAlerts(ctx context.Context, limit int, source string, onlyUnacked bool) ([]MonitorAlert, error) {
	if limit <= 0 || limit > 200 {
		limit = 80
	}
	var conds []string
	var args []any
	if src := strings.ToLower(strings.TrimSpace(source)); src != "" && src != "all" {
		conds = append(conds, "source = ?")
		args = append(args, src)
	}
	if onlyUnacked {
		conds = append(conds, "acknowledged = 0")
	}

	q := `SELECT id, source, severity, target, metric_value, threshold, message, notified, acknowledged, created_at
	      FROM monitor_alerts`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY created_at DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]MonitorAlert, 0)
	for rows.Next() {
		var a MonitorAlert
		var notif, ack int
		if err := rows.Scan(
			&a.ID, &a.Source, &a.Severity, &a.Target, &a.MetricValue, &a.Threshold,
			&a.Message, &notif, &ack, &a.CreatedAt,
		); err == nil {
			a.Notified = notif == 1
			a.Acknowledged = ack == 1
			list = append(list, a)
		}
	}
	return list, nil
}

// GetLatestAlertBySource mengembalikan alert paling akhir untuk satu sumber (mis. cpu/ram/disk) guna pelacakan cooldown
func (s *Store) GetLatestAlertBySource(ctx context.Context, source string) (*MonitorAlert, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, source, severity, target, metric_value, threshold, message, notified, acknowledged, created_at
		FROM monitor_alerts WHERE source = ? ORDER BY created_at DESC, id DESC LIMIT 1
	`, source)
	var a MonitorAlert
	var notif, ack int
	if err := row.Scan(
		&a.ID, &a.Source, &a.Severity, &a.Target, &a.MetricValue, &a.Threshold,
		&a.Message, &notif, &ack, &a.CreatedAt,
	); err != nil {
		return nil, err
	}
	a.Notified = notif == 1
	a.Acknowledged = ack == 1
	return &a, nil
}

// CountUnackedMonitorAlerts menghitung jumlah alert belum dibaca (termasuk insiden aktif)
func (s *Store) CountUnackedMonitorAlerts(ctx context.Context) int {
	var n int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM monitor_alerts WHERE acknowledged = 0").Scan(&n)
	return n
}

// AcknowledgeMonitorAlert menandai satu alert telah dibaca/ditangani
func (s *Store) AcknowledgeMonitorAlert(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE monitor_alerts SET acknowledged = 1 WHERE id = ?", id)
	return err
}

// AcknowledgeAllMonitorAlerts menandai seluruh alert telah dibaca
func (s *Store) AcknowledgeAllMonitorAlerts(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "UPDATE monitor_alerts SET acknowledged = 1 WHERE acknowledged = 0")
	return err
}

// ClearMonitorAlerts menghapus seluruh riwayat log alert
func (s *Store) ClearMonitorAlerts(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM monitor_alerts")
	return err
}
