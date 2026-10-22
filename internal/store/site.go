package store

import (
	"context"
	"fmt"
	"time"
)

// Site merepresentasikan konfigurasi virtual host web yang dikelola oleh fikpanel
type Site struct {
	ID         int64  `json:"id"`
	Domain     string `json:"domain"`
	Aliases    string `json:"aliases"`
	Kind       string `json:"kind"`
	Upstream   string `json:"upstream"`
	RootPath   string `json:"root_path"`
	SSLMode    string `json:"ssl_mode"`
	ForceHTTPS int    `json:"force_https"`
	ExtraConf  string `json:"extra_conf"`
	Enabled    int    `json:"enabled"`
	UpdatedAt  int64  `json:"updated_at"`
}

// ListSites mengambil seluruh situs yang terdaftar di database
func (s *Store) ListSites(ctx context.Context) ([]Site, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, domain, aliases, kind, COALESCE(upstream, ''), COALESCE(root_path, ''),
		       ssl_mode, force_https, extra_conf, enabled, updated_at
		FROM sites
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("gagal query daftar situs: %w", err)
	}
	defer rows.Close()

	var list []Site
	for rows.Next() {
		var item Site
		err := rows.Scan(
			&item.ID, &item.Domain, &item.Aliases, &item.Kind, &item.Upstream,
			&item.RootPath, &item.SSLMode, &item.ForceHTTPS, &item.ExtraConf,
			&item.Enabled, &item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal memindai baris situs: %w", err)
		}
		list = append(list, item)
	}

	return list, nil
}

// CreateSite menambahkan situs web baru
func (s *Store) CreateSite(ctx context.Context, site *Site) (*Site, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO sites (domain, aliases, kind, upstream, root_path, ssl_mode, force_https, extra_conf, enabled, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, site.Domain, site.Aliases, site.Kind, site.Upstream, site.RootPath, site.SSLMode, site.ForceHTTPS, site.ExtraConf, site.Enabled, now)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan situs: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("gagal membaca ID situs: %w", err)
	}

	site.ID = id
	site.UpdatedAt = now
	return site, nil
}

// DeleteSite menghapus situs berdasarkan ID
func (s *Store) DeleteSite(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sites WHERE id = ?", id)
	return err
}

// ToggleSite mengubah status aktif/nonaktif situs
func (s *Store) ToggleSite(ctx context.Context, id int64) (int, error) {
	now := time.Now().Unix()
	var current int
	err := s.db.QueryRowContext(ctx, "SELECT enabled FROM sites WHERE id = ?", id).Scan(&current)
	if err != nil {
		return 0, fmt.Errorf("situs tidak ditemukan: %w", err)
	}

	newState := 1
	if current == 1 {
		newState = 0
	}

	_, err = s.db.ExecContext(ctx, "UPDATE sites SET enabled = ?, updated_at = ? WHERE id = ?", newState, now, id)
	return newState, err
}
