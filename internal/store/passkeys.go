package store

import (
	"context"
	"fmt"
	"time"
)

// UserPasskey merepresentasikan satu perangkat Passkey / WebAuthn yang terdaftar milik akun panel
type UserPasskey struct {
	ID             int64  `json:"id"`
	UserID         int64  `json:"user_id"`
	Name           string `json:"name"`
	CredentialID   string `json:"credential_id"`
	CredentialJSON string `json:"-"`
	Transports     string `json:"transports"`
	CreatedAt      int64  `json:"created_at"`
	LastUsedAt     int64  `json:"last_used_at"`
}

// ListUserPasskeys mengembalikan seluruh Passkey terdaftar milik satu pengguna
func (s *Store) ListUserPasskeys(ctx context.Context, userID int64) ([]UserPasskey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, credential_id, credential_json, transports, created_at, last_used_at
		 FROM user_passkeys WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca daftar passkey: %w", err)
	}
	defer rows.Close()

	out := make([]UserPasskey, 0)
	for rows.Next() {
		var p UserPasskey
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.CredentialID, &p.CredentialJSON, &p.Transports, &p.CreatedAt, &p.LastUsedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CountAllPasskeys menghitung total Passkey aktif di seluruh panel (untuk menampilkan opsi login Passkey)
func (s *Store) CountAllPasskeys(ctx context.Context) int {
	var cnt int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM user_passkeys").Scan(&cnt)
	return cnt
}

// CreatePasskey menyimpan kredensial Passkey baru ke SQLite
func (s *Store) CreatePasskey(ctx context.Context, userID int64, name, credIDB64, credJSON, transports string) (*UserPasskey, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO user_passkeys (user_id, name, credential_id, credential_json, transports, created_at, last_used_at)
		 VALUES (?, ?, ?, ?, ?, ?, 0)`,
		userID, name, credIDB64, credJSON, transports, now)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan passkey: %w", err)
	}
	id, _ := res.LastInsertId()
	return &UserPasskey{
		ID:             id,
		UserID:         userID,
		Name:           name,
		CredentialID:   credIDB64,
		CredentialJSON: credJSON,
		Transports:     transports,
		CreatedAt:      now,
	}, nil
}

// FindPasskeyByCredentialID mencari pemilik Passkey berdasarkan ID kredensial Base64URL
func (s *Store) FindPasskeyByCredentialID(ctx context.Context, credIDB64 string) (*UserPasskey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, credential_id, credential_json, transports, created_at, last_used_at
		 FROM user_passkeys WHERE credential_id = ?`, credIDB64)
	var p UserPasskey
	if err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.CredentialID, &p.CredentialJSON, &p.Transports, &p.CreatedAt, &p.LastUsedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdatePasskeyUsage memperbarui JSON kredensial (sign count / backup flags) dan waktu pakai terakhir
func (s *Store) UpdatePasskeyUsage(ctx context.Context, credIDB64, credJSON string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		"UPDATE user_passkeys SET credential_json = ?, last_used_at = ? WHERE credential_id = ?",
		credJSON, now, credIDB64)
	return err
}

// DeleteUserPasskey menghapus satu Passkey milik pengguna tertentu
func (s *Store) DeleteUserPasskey(ctx context.Context, userID, passkeyID int64) (bool, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM user_passkeys WHERE id = ? AND user_id = ?", passkeyID, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
