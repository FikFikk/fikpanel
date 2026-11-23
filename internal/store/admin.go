package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

// ListUsers mengembalikan seluruh akun panel, diurutkan berdasarkan ID.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, username, password_hash, totp_secret, role, created_at, last_login_at FROM users ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("gagal membaca daftar pengguna: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.TotpSecret, &u.Role, &u.CreatedAt, &u.LastLoginAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// CreateAdminUser membuat akun administrator baru dengan hash kata sandi yang sudah dihitung.
func (s *Store) CreateAdminUser(ctx context.Context, username, passwordHash string) (*User, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, 'admin', ?)",
		username, passwordHash, now)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat akun %s: %w", username, err)
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, PasswordHash: passwordHash, Role: "admin", CreatedAt: now}, nil
}

// SetPasswordAndRevokeSessions mengganti hash kata sandi dan mencabut sesi login pengguna tersebut
// dalam satu transaksi. Jika keepRawToken tidak kosong, sesi dengan token itu (sesi yang sedang
// dipakai untuk mengganti kata sandi) tetap dipertahankan.
func (s *Store) SetPasswordAndRevokeSessions(ctx context.Context, userID int64, passwordHash, keepRawToken string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID); err != nil {
		return 0, fmt.Errorf("gagal menyimpan kata sandi: %w", err)
	}

	var res interface{ RowsAffected() (int64, error) }
	if keepRawToken != "" {
		keep := sha256.Sum256([]byte(keepRawToken))
		res, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ? AND token_hash <> ?", userID, keep[:])
	} else {
		res, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userID)
	}
	if err != nil {
		return 0, fmt.Errorf("gagal mencabut sesi login: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	revoked, _ := res.RowsAffected()
	return revoked, nil
}
