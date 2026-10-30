package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound menandai data yang dicari tidak ada
var ErrNotFound = errors.New("data tidak ditemukan")

// Site merepresentasikan konfigurasi virtual host web yang dikelola oleh fikpanel
type Site struct {
	ID          int64  `json:"id"`
	Domain      string `json:"domain"`
	Aliases     string `json:"aliases"`
	Kind        string `json:"kind"`
	Upstream    string `json:"upstream"`
	RootPath    string `json:"root_path"`
	SSLMode     string `json:"ssl_mode"`
	ForceHTTPS  int    `json:"force_https"`
	HSTS        int    `json:"hsts"`
	PHPVersion  string `json:"php_version"`
	Description string `json:"description"`
	ExtraConf   string `json:"extra_conf"`
	Enabled     int    `json:"enabled"`
	UpdatedAt   int64  `json:"updated_at"`

	// Ringkasan sertifikat (hasil LEFT JOIN, hanya-baca)
	SSLStatus  string `json:"ssl_status"`
	SSLExpires int64  `json:"ssl_expires"`

	// Metadata runtime, log akses nyata, dan repositori Git (diperkaya saat query API)
	RequestCount int64  `json:"request_count"`
	BackupCount  int    `json:"backup_count"`
	HasGit       bool   `json:"has_git"`
	GitBranch    string `json:"git_branch,omitempty"`
	GitRepo      string `json:"git_repo,omitempty"`
	GitCommit    string `json:"git_commit,omitempty"`
}

// Certificate merepresentasikan metadata sertifikat TLS milik sebuah situs
type Certificate struct {
	SiteID    int64  `json:"site_id"`
	Status    string `json:"status"`
	Issuer    string `json:"issuer"`
	NotBefore int64  `json:"not_before"`
	NotAfter  int64  `json:"not_after"`
	Domains   string `json:"domains"`
	Signature string `json:"signature"`
	KeyType   string `json:"key_type"`
	LastError string `json:"last_error"`
	RenewedAt int64  `json:"renewed_at"`
}

const siteColumns = `s.id, s.domain, s.aliases, s.kind, COALESCE(s.upstream, ''), COALESCE(s.root_path, ''),
	s.ssl_mode, s.force_https, s.hsts, s.php_version, s.description, s.extra_conf, s.enabled, s.updated_at,
	COALESCE(c.status, 'none'), COALESCE(c.not_after, 0)`

// scanSite memetakan satu baris hasil query ke struct Site
func scanSite(sc interface{ Scan(...any) error }) (Site, error) {
	var it Site
	err := sc.Scan(&it.ID, &it.Domain, &it.Aliases, &it.Kind, &it.Upstream, &it.RootPath,
		&it.SSLMode, &it.ForceHTTPS, &it.HSTS, &it.PHPVersion, &it.Description, &it.ExtraConf,
		&it.Enabled, &it.UpdatedAt, &it.SSLStatus, &it.SSLExpires)
	return it, err
}

// ListSites mengambil seluruh situs beserta ringkasan status sertifikat
func (s *Store) ListSites(ctx context.Context) ([]Site, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+siteColumns+`
		FROM sites s LEFT JOIN certificates c ON c.site_id = s.id
		ORDER BY s.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("gagal query daftar situs: %w", err)
	}
	defer rows.Close()

	var list []Site
	for rows.Next() {
		it, err := scanSite(rows)
		if err != nil {
			return nil, fmt.Errorf("gagal memindai baris situs: %w", err)
		}
		list = append(list, it)
	}
	return list, rows.Err()
}

// GetSite mengambil satu situs berdasarkan ID
func (s *Store) GetSite(ctx context.Context, id int64) (*Site, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+siteColumns+`
		FROM sites s LEFT JOIN certificates c ON c.site_id = s.id WHERE s.id = ?`, id)
	it, err := scanSite(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("gagal membaca situs: %w", err)
	}
	return &it, nil
}

// DomainTaken memeriksa apakah domain sudah dipakai sebagai domain utama atau alias situs lain
func (s *Store) DomainTaken(ctx context.Context, domain string, excludeID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM sites
		WHERE id != ? AND (domain = ? OR EXISTS (SELECT 1 FROM json_each(sites.aliases) WHERE value = ?))`,
		excludeID, domain, domain).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("gagal memeriksa ketersediaan domain: %w", err)
	}
	return n > 0, nil
}

// CreateSite menambahkan situs web baru
func (s *Store) CreateSite(ctx context.Context, site *Site) (*Site, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO sites (domain, aliases, kind, upstream, root_path, ssl_mode, force_https, hsts,
		                   php_version, description, extra_conf, enabled, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		site.Domain, site.Aliases, site.Kind, site.Upstream, site.RootPath, site.SSLMode, site.ForceHTTPS,
		site.HSTS, site.PHPVersion, site.Description, site.ExtraConf, site.Enabled, now)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan situs: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("gagal membaca ID situs: %w", err)
	}
	site.ID = id
	site.UpdatedAt = now
	if site.SSLStatus == "" {
		site.SSLStatus = "none"
	}
	return site, nil
}

// UpdateSite memperbarui seluruh kolom konfigurasi situs yang dapat diubah
func (s *Store) UpdateSite(ctx context.Context, site *Site) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE sites SET domain = ?, aliases = ?, kind = ?, upstream = ?, root_path = ?, ssl_mode = ?,
		       force_https = ?, hsts = ?, php_version = ?, description = ?, extra_conf = ?, updated_at = ?
		WHERE id = ?`,
		site.Domain, site.Aliases, site.Kind, site.Upstream, site.RootPath, site.SSLMode,
		site.ForceHTTPS, site.HSTS, site.PHPVersion, site.Description, site.ExtraConf, time.Now().Unix(), site.ID)
	if err != nil {
		return fmt.Errorf("gagal memperbarui situs: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSite menghapus situs berdasarkan ID (sertifikat ikut terhapus via ON DELETE CASCADE)
func (s *Store) DeleteSite(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sites WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("gagal menghapus situs: %w", err)
	}
	return nil
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

// GetCertificate mengambil metadata sertifikat situs; status 'none' bila belum ada
func (s *Store) GetCertificate(ctx context.Context, siteID int64) (*Certificate, error) {
	c := &Certificate{SiteID: siteID, Status: "none", Domains: "[]"}
	err := s.db.QueryRowContext(ctx, `
		SELECT status, issuer, not_before, not_after, domains, signature, key_type,
		       COALESCE(last_error, ''), COALESCE(renewed_at, 0)
		FROM certificates WHERE site_id = ?`, siteID).Scan(
		&c.Status, &c.Issuer, &c.NotBefore, &c.NotAfter, &c.Domains, &c.Signature, &c.KeyType,
		&c.LastError, &c.RenewedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gagal membaca sertifikat: %w", err)
	}
	return c, nil
}

// UpsertCertificate menyimpan atau memperbarui metadata sertifikat situs
func (s *Store) UpsertCertificate(ctx context.Context, c *Certificate) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO certificates (site_id, status, issuer, not_before, not_after, domains, signature, key_type, last_error, renewed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_id) DO UPDATE SET
			status = excluded.status, issuer = excluded.issuer, not_before = excluded.not_before,
			not_after = excluded.not_after, domains = excluded.domains, signature = excluded.signature,
			key_type = excluded.key_type, last_error = excluded.last_error, renewed_at = excluded.renewed_at`,
		c.SiteID, c.Status, c.Issuer, c.NotBefore, c.NotAfter, c.Domains, c.Signature, c.KeyType,
		nullIfEmpty(c.LastError), c.RenewedAt)
	if err != nil {
		return fmt.Errorf("gagal menyimpan sertifikat: %w", err)
	}
	return nil
}

// DeleteCertificate menghapus metadata sertifikat situs
func (s *Store) DeleteCertificate(ctx context.Context, siteID int64) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM certificates WHERE site_id = ?", siteID); err != nil {
		return fmt.Errorf("gagal menghapus sertifikat: %w", err)
	}
	return nil
}

// ListExpiringCertificates mengambil ID situs Let's Encrypt yang sertifikatnya habis sebelum batas waktu
func (s *Store) ListExpiringCertificates(ctx context.Context, before int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.site_id FROM certificates c JOIN sites s ON s.id = c.site_id
		WHERE s.ssl_mode = 'letsencrypt' AND c.status = 'issued' AND c.not_after < ?`, before)
	if err != nil {
		return nil, fmt.Errorf("gagal query sertifikat kedaluwarsa: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("gagal memindai sertifikat: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
