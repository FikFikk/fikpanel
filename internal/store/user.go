package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"fikpanel/internal/auth"
)

// User merepresentasikan entitas operator atau administrator panel
type User struct {
	ID           int64          `json:"id"`
	Username     string         `json:"username"`
	PasswordHash string         `json:"-"`
	TotpSecret   []byte         `json:"-"`
	Role         string         `json:"role"`
	CreatedAt    int64          `json:"created_at"`
	LastLoginAt  sql.NullInt64  `json:"last_login_at"`
}

// EnsureDevUser memastikan ketersediaan akun pengembang lokal untuk pengujian dev
func (s *Store) EnsureDevUser(username, rawPassword string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := s.GetUserByUsername(ctx, username)
	if err == nil {
		return user, nil // Akun sudah ada
	}

	// Buat hash password Argon2id
	hash, err := auth.HashPassword(rawPassword)
	if err != nil {
		return nil, fmt.Errorf("gagal hashing password dev user: %w", err)
	}

	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, 'admin', ?)",
		username, hash, now)
	if err != nil {
		return nil, fmt.Errorf("gagal menyisipkan dev user: %w", err)
	}

	id, _ := res.LastInsertId()
	return &User{
		ID:           id,
		Username:     username,
		PasswordHash: hash,
		Role:         "admin",
		CreatedAt:    now,
	}, nil
}

// GetUserByUsername mengambil satu data user berdasarkan username unik
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, username, password_hash, totp_secret, role, created_at, last_login_at FROM users WHERE username = ?",
		username)

	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.TotpSecret, &u.Role, &u.CreatedAt, &u.LastLoginAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateLastLogin memperbarui waktu login terakhir pengguna saat autentikasi sukses
func (s *Store) UpdateLastLogin(ctx context.Context, userID int64) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, "UPDATE users SET last_login_at = ? WHERE id = ?", now, userID)
	return err
}
