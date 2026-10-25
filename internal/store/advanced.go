package store

import (
	"context"
	"fmt"
	"time"
)

type DNSZone struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	TTL       int    `json:"ttl"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type DNSRecord struct {
	ID        int64  `json:"id"`
	ZoneID    int64  `json:"zone_id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Priority  int    `json:"priority"`
	TTL       int    `json:"ttl"`
	CreatedAt int64  `json:"created_at"`
}

type SystemWebhook struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Channel           string `json:"channel"`
	WebhookURL        string `json:"webhook_url"`
	NotifyOnHighCPU   bool   `json:"notify_on_high_cpu"`
	NotifyOnHighRAM   bool   `json:"notify_on_high_ram"`
	NotifyOnBackup    bool   `json:"notify_on_backup"`
	NotifyOnSSLExpiry bool   `json:"notify_on_ssl_expiry"`
	CreatedAt         int64  `json:"created_at"`
}

func (s *Store) ListDNSZones(ctx context.Context) ([]DNSZone, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, ttl, created_at, updated_at FROM dns_zones ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []DNSZone
	for rows.Next() {
		var z DNSZone
		if err := rows.Scan(&z.ID, &z.Name, &z.TTL, &z.CreatedAt, &z.UpdatedAt); err != nil {
			return nil, err
		}
		zones = append(zones, z)
	}
	if zones == nil {
		zones = []DNSZone{}
	}
	return zones, nil
}

func (s *Store) CreateDNSZone(ctx context.Context, name string, ttl int) (int64, error) {
	if ttl <= 0 {
		ttl = 3600
	}
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `INSERT INTO dns_zones (name, ttl, created_at, updated_at) VALUES (?, ?, ?, ?)`, name, ttl, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) DeleteDNSZone(ctx context.Context, id int64) error {
	_, _ = s.db.ExecContext(ctx, `DELETE FROM dns_records WHERE zone_id = ?`, id)
	_, err := s.db.ExecContext(ctx, `DELETE FROM dns_zones WHERE id = ?`, id)
	return err
}

func (s *Store) ListDNSRecords(ctx context.Context, zoneID int64) ([]DNSRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, zone_id, type, name, content, priority, ttl, created_at FROM dns_records WHERE zone_id = ? ORDER BY type, name`, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []DNSRecord
	for rows.Next() {
		var r DNSRecord
		if err := rows.Scan(&r.ID, &r.ZoneID, &r.Type, &r.Name, &r.Content, &r.Priority, &r.TTL, &r.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	if records == nil {
		records = []DNSRecord{}
	}
	return records, nil
}

func (s *Store) CreateDNSRecord(ctx context.Context, r *DNSRecord) (int64, error) {
	if r.TTL <= 0 {
		r.TTL = 3600
	}
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `INSERT INTO dns_records (zone_id, type, name, content, priority, ttl, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.ZoneID, r.Type, r.Name, r.Content, r.Priority, r.TTL, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) DeleteDNSRecord(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM dns_records WHERE id = ?`, id)
	return err
}

func (s *Store) ListWebhooks(ctx context.Context) ([]SystemWebhook, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, channel, webhook_url, notify_on_high_cpu, notify_on_high_ram, notify_on_backup, notify_on_ssl_expiry, created_at FROM system_webhooks ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SystemWebhook
	for rows.Next() {
		var w SystemWebhook
		var c, r, b, ssl int
		if err := rows.Scan(&w.ID, &w.Name, &w.Channel, &w.WebhookURL, &c, &r, &b, &ssl, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.NotifyOnHighCPU = c == 1
		w.NotifyOnHighRAM = r == 1
		w.NotifyOnBackup = b == 1
		w.NotifyOnSSLExpiry = ssl == 1
		list = append(list, w)
	}
	if list == nil {
		list = []SystemWebhook{}
	}
	return list, nil
}

func (s *Store) CreateWebhook(ctx context.Context, w *SystemWebhook) (int64, error) {
	now := time.Now().Unix()
	c, r, b, ssl := 0, 0, 0, 0
	if w.NotifyOnHighCPU {
		c = 1
	}
	if w.NotifyOnHighRAM {
		r = 1
	}
	if w.NotifyOnBackup {
		b = 1
	}
	if w.NotifyOnSSLExpiry {
		ssl = 1
	}

	res, err := s.db.ExecContext(ctx, `INSERT INTO system_webhooks (name, channel, webhook_url, notify_on_high_cpu, notify_on_high_ram, notify_on_backup, notify_on_ssl_expiry, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		w.Name, w.Channel, w.WebhookURL, c, r, b, ssl, now)
	if err != nil {
		return 0, fmt.Errorf("create webhook: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) DeleteWebhook(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM system_webhooks WHERE id = ?`, id)
	return err
}
