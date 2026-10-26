package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

var _ = sql.ErrNoRows

type MonitorSnapshot struct {
	ID        int64   `json:"id"`
	Timestamp int64   `json:"timestamp"`
	CPU       float64 `json:"cpu"`
	RAMUsed   uint64  `json:"ram_used"`
	RAMTotal  uint64  `json:"ram_total"`
	Load1m    float64 `json:"load_1m"`
	Load5m    float64 `json:"load_5m"`
	Load15m   float64 `json:"load_15m"`
	DiskUsed  uint64  `json:"disk_used"`
	DiskTotal uint64  `json:"disk_total"`
	NetIn     uint64  `json:"net_in"`
	NetOut    uint64  `json:"net_out"`
}

type MonitorConfig struct {
	Enabled  bool `json:"enabled"`
	SaveDays int  `json:"save_days"`
}

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

func (s *Store) UpdateMonitorConfig(ctx context.Context, enabled bool, saveDays int) error {
	enVal := "0"
	if enabled {
		enVal = "1"
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

	if saveDays <= 0 {
		saveDays = 30
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

func (s *Store) InsertMonitorSnapshot(ctx context.Context, snap *MonitorSnapshot) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_snapshots 
		(timestamp, cpu, ram_used, ram_total, load_1m, load_5m, load_15m, disk_used, disk_total, net_in, net_out)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, snap.Timestamp, snap.CPU, snap.RAMUsed, snap.RAMTotal, snap.Load1m, snap.Load5m, snap.Load15m, snap.DiskUsed, snap.DiskTotal, snap.NetIn, snap.NetOut)
	return err
}

func (s *Store) GetMonitorHistory(ctx context.Context, startTS, endTS int64) ([]MonitorSnapshot, error) {
	query := `
		SELECT id, timestamp, cpu, ram_used, ram_total, load_1m, load_5m, load_15m, disk_used, disk_total, net_in, net_out
		FROM monitor_snapshots
		WHERE timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`
	rows, err := s.db.QueryContext(ctx, query, startTS, endTS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []MonitorSnapshot
	for rows.Next() {
		var sn MonitorSnapshot
		if err := rows.Scan(
			&sn.ID, &sn.Timestamp, &sn.CPU, &sn.RAMUsed, &sn.RAMTotal,
			&sn.Load1m, &sn.Load5m, &sn.Load15m, &sn.DiskUsed, &sn.DiskTotal,
			&sn.NetIn, &sn.NetOut,
		); err == nil {
			result = append(result, sn)
		}
	}
	return result, nil
}

func (s *Store) PurgeMonitorSnapshots(ctx context.Context) error {
	cfg, err := s.GetMonitorConfig(ctx)
	if err != nil {
		return err
	}
	cutoff := time.Now().AddDate(0, 0, -cfg.SaveDays).Unix()
	_, err = s.db.ExecContext(ctx, "DELETE FROM monitor_snapshots WHERE timestamp < ?", cutoff)
	return err
}

func (s *Store) ClearAllMonitorSnapshots(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM monitor_snapshots")
	return err
}
