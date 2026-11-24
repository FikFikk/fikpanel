package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// SiteDeployConfig menyimpan konfigurasi Webhook Push-to-Deploy dan perilaku deploy Laravel/Git per situs
type SiteDeployConfig struct {
	SiteID          int64  `json:"site_id"`
	WebhookSecret   string `json:"webhook_secret"`
	WebhookEnabled  bool   `json:"webhook_enabled"`
	AutoOptimize    bool   `json:"auto_optimize"`
	MaintenanceMode bool   `json:"maintenance_mode"`
	RebuildAssets   bool   `json:"rebuild_assets"`
	Branch          string `json:"branch"`
	UpdatedAt       int64  `json:"updated_at"`
}

// SiteDeployment menyimpan satu catatan riwayat deploy (initial, manual, webhook, atau rollback)
type SiteDeployment struct {
	ID            int64  `json:"id"`
	SiteID        int64  `json:"site_id"`
	Trigger       string `json:"trigger"` // initial | manual | webhook | rollback
	Branch        string `json:"branch"`
	OldCommit     string `json:"old_commit"`
	NewCommit     string `json:"new_commit"`
	CommitMessage string `json:"commit_message"`
	Status        string `json:"status"` // running | success | failed
	DurationMs    int64  `json:"duration_ms"`
	Output        string `json:"output"`
	CreatedAt     int64  `json:"created_at"`
}

// GenerateDeploySecret membangkitkan rahasia webhook 256-bit (64 karakter hex) secara kriptografis
func GenerateDeploySecret() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf)
}

// GetOrInitSiteDeployConfig mengambil konfigurasi deploy situs; membuat default + secret baru jika belum ada
func (s *Store) GetOrInitSiteDeployConfig(ctx context.Context, siteID int64) (*SiteDeployConfig, error) {
	var cfg SiteDeployConfig
	var whEn, autoOpt, maintMode, rebuildAssets int
	err := s.db.QueryRowContext(ctx,
		`SELECT site_id, webhook_secret, webhook_enabled, auto_optimize, maintenance_mode, rebuild_assets, branch, updated_at
		 FROM site_deploy_configs WHERE site_id = ?`, siteID,
	).Scan(&cfg.SiteID, &cfg.WebhookSecret, &whEn, &autoOpt, &maintMode, &rebuildAssets, &cfg.Branch, &cfg.UpdatedAt)
	if err == nil {
		cfg.WebhookEnabled = whEn == 1
		cfg.AutoOptimize = autoOpt == 1
		cfg.MaintenanceMode = maintMode == 1
		cfg.RebuildAssets = rebuildAssets == 1
		if strings.TrimSpace(cfg.WebhookSecret) == "" {
			cfg.WebhookSecret = GenerateDeploySecret()
			_ = s.SaveSiteDeployConfig(ctx, &cfg)
		}
		return &cfg, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Buat konfigurasi bawaan yang aman (optimasi produksi, maintenance mode, dan rebuild aset aktif)
	def := &SiteDeployConfig{
		SiteID:          siteID,
		WebhookSecret:   GenerateDeploySecret(),
		WebhookEnabled:  false,
		AutoOptimize:    true,
		MaintenanceMode: true,
		RebuildAssets:   true,
		UpdatedAt:       time.Now().Unix(),
	}
	if err := s.SaveSiteDeployConfig(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

// SaveSiteDeployConfig menyimpan atau memperbarui konfigurasi deploy situs
func (s *Store) SaveSiteDeployConfig(ctx context.Context, cfg *SiteDeployConfig) error {
	if strings.TrimSpace(cfg.WebhookSecret) == "" {
		cfg.WebhookSecret = GenerateDeploySecret()
	}
	cfg.UpdatedAt = time.Now().Unix()
	boolInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO site_deploy_configs
		 (site_id, webhook_secret, webhook_enabled, auto_optimize, maintenance_mode, rebuild_assets, branch, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(site_id) DO UPDATE SET
		   webhook_secret = excluded.webhook_secret,
		   webhook_enabled = excluded.webhook_enabled,
		   auto_optimize = excluded.auto_optimize,
		   maintenance_mode = excluded.maintenance_mode,
		   rebuild_assets = excluded.rebuild_assets,
		   branch = excluded.branch,
		   updated_at = excluded.updated_at`,
		cfg.SiteID, cfg.WebhookSecret, boolInt(cfg.WebhookEnabled), boolInt(cfg.AutoOptimize),
		boolInt(cfg.MaintenanceMode), boolInt(cfg.RebuildAssets), strings.TrimSpace(cfg.Branch), cfg.UpdatedAt,
	)
	return err
}

// RecordSiteDeployment menyimpan catatan eksekusi deploy & menjaga retensi 25 riwayat terakhir per situs
func (s *Store) RecordSiteDeployment(ctx context.Context, d *SiteDeployment) (int64, error) {
	if d.CreatedAt == 0 {
		d.CreatedAt = time.Now().Unix()
	}
	// Batasi ukuran log tersimpan di SQLite (maks 64 KB) agar database tetap ramping
	out := d.Output
	if len(out) > 65536 {
		out = "... [dipotong]\n" + out[len(out)-65536:]
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO site_deployments
		 (site_id, trigger, branch, old_commit, new_commit, commit_message, status, duration_ms, output, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.SiteID, d.Trigger, d.Branch, d.OldCommit, d.NewCommit, d.CommitMessage, d.Status, d.DurationMs, out, d.CreatedAt,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	d.ID = id

	// Pangkas riwayat lama agar setiap situs menyimpan maksimal 25 deployment terbaru
	_, _ = s.db.ExecContext(ctx,
		`DELETE FROM site_deployments WHERE site_id = ? AND id NOT IN (
			SELECT id FROM site_deployments WHERE site_id = ? ORDER BY created_at DESC, id DESC LIMIT 25
		)`, d.SiteID, d.SiteID,
	)
	return id, nil
}

// ListSiteDeployments mengembalikan riwayat deployment terbaru untuk satu situs
func (s *Store) ListSiteDeployments(ctx context.Context, siteID int64, limit int) ([]SiteDeployment, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, trigger, branch, old_commit, new_commit, commit_message, status, duration_ms, output, created_at
		 FROM site_deployments WHERE site_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`,
		siteID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SiteDeployment{}
	for rows.Next() {
		var d SiteDeployment
		if err := rows.Scan(&d.ID, &d.SiteID, &d.Trigger, &d.Branch, &d.OldCommit, &d.NewCommit, &d.CommitMessage, &d.Status, &d.DurationMs, &d.Output, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
