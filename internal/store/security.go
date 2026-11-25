package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SecurityIPRule merepresentasikan satu entri daftar putih atau daftar hitam IP/CIDR persisten
type SecurityIPRule struct {
	ID        int64  `json:"id"`
	ListType  string `json:"list_type"` // "whitelist" atau "blacklist"
	IP        string `json:"ip"`
	Remarks   string `json:"remarks"`
	Source    string `json:"source"` // "manual", "auto_bruteforce", "system"
	SyncUFW   bool   `json:"sync_ufw"`
	AddedAt   int64  `json:"added_at"`
	ExpiresAt int64  `json:"expires_at"` // 0 = permanen
}

// SecurityLoginEvent merepresentasikan satu catatan percobaan login panel/SSH
type SecurityLoginEvent struct {
	ID        int64  `json:"id"`
	TS        int64  `json:"time"`
	IP        string `json:"ip"`
	Username  string `json:"username"`
	Source    string `json:"source"` // "panel" atau "ssh"
	Status    string `json:"status"` // "failed", "success", "banned"
	Reason    string `json:"reason"`
	UserAgent string `json:"user_agent,omitempty"`
}

// SecurityScanRecord menyimpan snapshot laporan hasil audit/pemindaian keamanan
type SecurityScanRecord struct {
	ID            int64  `json:"id"`
	ScannedAt     int64  `json:"scanned_at"`
	Score         int    `json:"score"`
	Rating        string `json:"rating"`
	RiskCount     int    `json:"risk_count"`
	MalwareCount  int    `json:"malware_count"`
	VulnCount     int    `json:"vuln_count"`
	BaselinePass  int    `json:"baseline_pass"`
	BaselineTotal int    `json:"baseline_total"`
	FindingsJSON  string `json:"findings_json"`
}

// FIMBaselineEntry menyimpan jejak hash SHA-256 satu berkas kritis untuk File Integrity Monitor
type FIMBaselineEntry struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	Mode      string `json:"mode"`
	ModTime   int64  `json:"mod_time"`
	UpdatedAt int64  `json:"updated_at"`
}

// ListSecurityIPs mengembalikan daftar IP whitelist atau blacklist yang masih aktif (belum kedaluwarsa)
func (s *Store) ListSecurityIPs(ctx context.Context, listType string) ([]SecurityIPRule, error) {
	now := time.Now().Unix()
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, list_type, ip, remarks, source, sync_ufw, added_at, expires_at
		FROM security_ip_lists
		WHERE list_type = ? AND (expires_at = 0 OR expires_at > ?)
		ORDER BY added_at DESC, id DESC
	`, listType, now)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca daftar %s IP: %w", listType, err)
	}
	defer rows.Close()

	list := make([]SecurityIPRule, 0)
	for rows.Next() {
		var r SecurityIPRule
		var syncInt int
		if err := rows.Scan(&r.ID, &r.ListType, &r.IP, &r.Remarks, &r.Source, &syncInt, &r.AddedAt, &r.ExpiresAt); err != nil {
			return nil, fmt.Errorf("gagal memindai baris security_ip_lists: %w", err)
		}
		r.SyncUFW = syncInt != 0
		list = append(list, r)
	}
	return list, nil
}

// UpsertSecurityIP menyimpan atau memperbarui entri IP/CIDR pada whitelist atau blacklist
func (s *Store) UpsertSecurityIP(ctx context.Context, r SecurityIPRule) error {
	if r.AddedAt == 0 {
		r.AddedAt = time.Now().Unix()
	}
	if r.Source == "" {
		r.Source = "manual"
	}
	syncInt := 0
	if r.SyncUFW {
		syncInt = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO security_ip_lists (list_type, ip, remarks, source, sync_ufw, added_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(list_type, ip) DO UPDATE SET
			remarks = excluded.remarks,
			source = excluded.source,
			sync_ufw = excluded.sync_ufw,
			added_at = excluded.added_at,
			expires_at = excluded.expires_at
	`, r.ListType, r.IP, r.Remarks, r.Source, syncInt, r.AddedAt, r.ExpiresAt)
	if err != nil {
		return fmt.Errorf("gagal menyimpan aturan IP %s: %w", r.IP, err)
	}
	return nil
}

// DeleteSecurityIP menghapus entri IP berdasarkan tipe daftar dan alamat IP/CIDR
func (s *Store) DeleteSecurityIP(ctx context.Context, listType, ip string) (*SecurityIPRule, error) {
	var r SecurityIPRule
	var syncInt int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, list_type, ip, remarks, source, sync_ufw, added_at, expires_at
		FROM security_ip_lists
		WHERE list_type = ? AND ip = ?
	`, listType, ip).Scan(&r.ID, &r.ListType, &r.IP, &r.Remarks, &r.Source, &syncInt, &r.AddedAt, &r.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("gagal mencari aturan IP %s: %w", ip, err)
	}
	r.SyncUFW = syncInt != 0

	if _, err := s.db.ExecContext(ctx, `DELETE FROM security_ip_lists WHERE id = ?`, r.ID); err != nil {
		return nil, fmt.Errorf("gagal menghapus aturan IP %s: %w", ip, err)
	}
	return &r, nil
}

// DeleteSecurityIPByID menghapus entri IP berdasarkan ID unik dan mengembalikan data yang dihapus
func (s *Store) DeleteSecurityIPByID(ctx context.Context, id int64) (*SecurityIPRule, error) {
	var r SecurityIPRule
	var syncInt int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, list_type, ip, remarks, source, sync_ufw, added_at, expires_at
		FROM security_ip_lists
		WHERE id = ?
	`, id).Scan(&r.ID, &r.ListType, &r.IP, &r.Remarks, &r.Source, &syncInt, &r.AddedAt, &r.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("gagal mencari aturan IP #%d: %w", id, err)
	}
	r.SyncUFW = syncInt != 0

	if _, err := s.db.ExecContext(ctx, `DELETE FROM security_ip_lists WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("gagal menghapus aturan IP #%d: %w", id, err)
	}
	return &r, nil
}

// PurgeExpiredBlacklist menghapus seluruh entri blacklist yang sudah melampaui masa kedaluwarsa
func (s *Store) PurgeExpiredBlacklist(ctx context.Context) ([]SecurityIPRule, error) {
	now := time.Now().Unix()
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, list_type, ip, remarks, source, sync_ufw, added_at, expires_at
		FROM security_ip_lists
		WHERE list_type = 'blacklist' AND expires_at > 0 AND expires_at <= ?
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	expired := make([]SecurityIPRule, 0)
	for rows.Next() {
		var r SecurityIPRule
		var syncInt int
		if err := rows.Scan(&r.ID, &r.ListType, &r.IP, &r.Remarks, &r.Source, &syncInt, &r.AddedAt, &r.ExpiresAt); err == nil {
			r.SyncUFW = syncInt != 0
			expired = append(expired, r)
		}
	}
	if len(expired) > 0 {
		_, _ = s.db.ExecContext(ctx, `
			DELETE FROM security_ip_lists
			WHERE list_type = 'blacklist' AND expires_at > 0 AND expires_at <= ?
		`, now)
	}
	return expired, nil
}

// RecordSecurityLoginEvent mencatat peristiwa autentikasi (gagal/sukses/diblokir) ke database
func (s *Store) RecordSecurityLoginEvent(ctx context.Context, ev SecurityLoginEvent) error {
	if ev.TS == 0 {
		ev.TS = time.Now().Unix()
	}
	if ev.Source == "" {
		ev.Source = "panel"
	}
	if ev.Status == "" {
		ev.Status = "failed"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO security_login_events (ts, ip, username, source, status, reason, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, ev.TS, ev.IP, ev.Username, ev.Source, ev.Status, ev.Reason, ev.UserAgent)
	if err != nil {
		return fmt.Errorf("gagal mencatat riwayat login keamanan: %w", err)
	}
	return nil
}

// ListSecurityLoginEvents mengembalikan riwayat percobaan login terbaru (opsional filter status)
func (s *Store) ListSecurityLoginEvents(ctx context.Context, limit int, statusFilter string) ([]SecurityLoginEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if statusFilter != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, ts, ip, username, source, status, reason, user_agent
			FROM security_login_events
			WHERE status = ?
			ORDER BY ts DESC, id DESC
			LIMIT ?
		`, statusFilter, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, ts, ip, username, source, status, reason, user_agent
			FROM security_login_events
			ORDER BY ts DESC, id DESC
			LIMIT ?
		`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("gagal memuat riwayat login keamanan: %w", err)
	}
	defer rows.Close()

	list := make([]SecurityLoginEvent, 0)
	for rows.Next() {
		var ev SecurityLoginEvent
		if err := rows.Scan(&ev.ID, &ev.TS, &ev.IP, &ev.Username, &ev.Source, &ev.Status, &ev.Reason, &ev.UserAgent); err != nil {
			return nil, fmt.Errorf("gagal memindai baris security_login_events: %w", err)
		}
		list = append(list, ev)
	}
	return list, nil
}

// CountRecentFailedLoginsByIP menghitung jumlah kegagalan login panel dari satu IP sejak timestamp tertentu
func (s *Store) CountRecentFailedLoginsByIP(ctx context.Context, ip string, sinceUnix int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM security_login_events
		WHERE source = 'panel' AND status = 'failed' AND ip = ? AND ts >= ?
	`, ip, sinceUnix).Scan(&count)
	return count, err
}

// CountRecentFailedLoginsByUser menghitung jumlah kegagalan login panel pada satu username sejak timestamp tertentu
func (s *Store) CountRecentFailedLoginsByUser(ctx context.Context, username string, sinceUnix int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM security_login_events
		WHERE source = 'panel' AND status = 'failed' AND username = ? AND ts >= ?
	`, username, sinceUnix).Scan(&count)
	return count, err
}

// GetLatestPanelLogin mengembalikan catatan login panel sukses terbaru dari log keamanan atau sesi aktif
func (s *Store) GetLatestPanelLogin(ctx context.Context) (*SecurityLoginEvent, error) {
	var ev SecurityLoginEvent
	err := s.db.QueryRowContext(ctx, `
		SELECT id, ts, ip, username, source, status, reason, user_agent
		FROM security_login_events
		WHERE source = 'panel' AND status = 'success'
		ORDER BY ts DESC, id DESC
		LIMIT 1
	`).Scan(&ev.ID, &ev.TS, &ev.IP, &ev.Username, &ev.Source, &ev.Status, &ev.Reason, &ev.UserAgent)
	if err == nil {
		return &ev, nil
	}

	// Fallback ke tabel sessions bila belum ada catatan di security_login_events
	var ts int64
	var ip, ua, uname string
	err2 := s.db.QueryRowContext(ctx, `
		SELECT s.created_at, COALESCE(s.ip, ''), COALESCE(s.user_agent, ''), COALESCE(u.username, '')
		FROM sessions s
		LEFT JOIN users u ON u.id = s.user_id
		ORDER BY s.created_at DESC
		LIMIT 1
	`).Scan(&ts, &ip, &ua, &uname)
	if err2 != nil {
		return nil, ErrNotFound
	}
	return &SecurityLoginEvent{
		TS:        ts,
		IP:        ip,
		Username:  uname,
		Source:    "panel",
		Status:    "success",
		Reason:    "Sesi panel aktif",
		UserAgent: ua,
	}, nil
}

// PruneSecurityLoginEvents membersihkan catatan riwayat login yang lebih tua dari batas retensi
func (s *Store) PruneSecurityLoginEvents(ctx context.Context, olderThanUnix int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM security_login_events WHERE ts < ?`, olderThanUnix)
	return err
}

// SaveSecurityScan menyimpan laporan pemindaian keamanan dan menjaga retensi maksimal 20 laporan terakhir
func (s *Store) SaveSecurityScan(ctx context.Context, rec SecurityScanRecord) (int64, error) {
	if rec.ScannedAt == 0 {
		rec.ScannedAt = time.Now().Unix()
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO security_scan_reports
			(scanned_at, score, rating, risk_count, malware_count, vuln_count, baseline_pass, baseline_total, findings_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, rec.ScannedAt, rec.Score, rec.Rating, rec.RiskCount, rec.MalwareCount, rec.VulnCount, rec.BaselinePass, rec.BaselineTotal, rec.FindingsJSON)
	if err != nil {
		return 0, fmt.Errorf("gagal menyimpan laporan pemindaian keamanan: %w", err)
	}
	id, _ := res.LastInsertId()

	// Pangkas laporan lama agar ukuran database tetap terkendali
	_, _ = s.db.ExecContext(ctx, `
		DELETE FROM security_scan_reports
		WHERE id NOT IN (SELECT id FROM security_scan_reports ORDER BY scanned_at DESC, id DESC LIMIT 20)
	`)
	return id, nil
}

// GetLatestSecurityScan mengambil laporan pemindaian keamanan paling baru
func (s *Store) GetLatestSecurityScan(ctx context.Context) (*SecurityScanRecord, error) {
	var rec SecurityScanRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT id, scanned_at, score, rating, risk_count, malware_count, vuln_count, baseline_pass, baseline_total, findings_json
		FROM security_scan_reports
		ORDER BY scanned_at DESC, id DESC
		LIMIT 1
	`).Scan(
		&rec.ID, &rec.ScannedAt, &rec.Score, &rec.Rating,
		&rec.RiskCount, &rec.MalwareCount, &rec.VulnCount,
		&rec.BaselinePass, &rec.BaselineTotal, &rec.FindingsJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("gagal membaca laporan pemindaian terbaru: %w", err)
	}
	return &rec, nil
}

// ReplaceFIMBaselines mengganti seluruh snapshot baseline File Integrity Monitor secara transaksional
func (s *Store) ReplaceFIMBaselines(ctx context.Context, entries []FIMBaselineEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi FIM: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM security_fim_baselines`); err != nil {
		return fmt.Errorf("gagal mengosongkan baseline FIM lama: %w", err)
	}

	now := time.Now().Unix()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO security_fim_baselines (path, sha256, size, mode, mod_time, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("gagal menyiapkan query FIM: %w", err)
	}
	defer stmt.Close()

	for _, e := range entries {
		upd := e.UpdatedAt
		if upd == 0 {
			upd = now
		}
		if _, err := stmt.ExecContext(ctx, e.Path, e.SHA256, e.Size, e.Mode, e.ModTime, upd); err != nil {
			return fmt.Errorf("gagal menyimpan baseline FIM untuk %s: %w", e.Path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("gagal commit baseline FIM: %w", err)
	}
	return nil
}

// ListFIMBaselines mengambil seluruh berkas yang terdaftar dalam baseline FIM
func (s *Store) ListFIMBaselines(ctx context.Context) ([]FIMBaselineEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, sha256, size, mode, mod_time, updated_at
		FROM security_fim_baselines
		ORDER BY path ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca baseline FIM: %w", err)
	}
	defer rows.Close()

	list := make([]FIMBaselineEntry, 0)
	for rows.Next() {
		var e FIMBaselineEntry
		if err := rows.Scan(&e.Path, &e.SHA256, &e.Size, &e.Mode, &e.ModTime, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("gagal memindai baris baseline FIM: %w", err)
		}
		list = append(list, e)
	}
	return list, nil
}

// ClearFIMBaselines menghapus seluruh baseline FIM (menonaktifkan pemantauan FIM)
func (s *Store) ClearFIMBaselines(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM security_fim_baselines`)
	return err
}
