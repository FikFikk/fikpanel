package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	ErrSessionExpired = errors.New("sesi telah kedaluwarsa atau tidak valid")
)

// SessionInfo menyimpan informasi sesi login aktif
type SessionInfo struct {
	TokenHash  []byte
	UserID     int64
	CreatedAt  int64
	ExpiresAt  int64
	SudoUntil  int64
	IP         string
	UserAgent  string
}

// CreateSession membuat token sesi acak baru, menyimpannya dalam bentuk hash SHA-256 di DB
func (s *Store) CreateSession(ctx context.Context, userID int64, ip, userAgent string, duration time.Duration) (string, error) {
	rawTokenBytes := make([]byte, 32)
	if _, err := rand.Read(rawTokenBytes); err != nil {
		return "", fmt.Errorf("gagal menghasilkan token sesi: %w", err)
	}

	rawToken := hex.EncodeToString(rawTokenBytes)
	tokenHash := sha256.Sum256([]byte(rawToken))

	now := time.Now().Unix()
	expiresAt := now + int64(duration.Seconds())

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions (token_hash, user_id, created_at, expires_at, ip, user_agent) VALUES (?, ?, ?, ?, ?, ?)",
		tokenHash[:], userID, now, expiresAt, ip, userAgent)
	if err != nil {
		return "", fmt.Errorf("gagal menyimpan sesi ke database: %w", err)
	}

	return rawToken, nil
}

// GetSessionUser memverifikasi token sesi mentah dan mengembalikan data pengguna jika valid
func (s *Store) GetSessionUser(ctx context.Context, rawToken string) (*User, error) {
	tokenHash := sha256.Sum256([]byte(rawToken))
	now := time.Now().Unix()

	row := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.password_hash, u.totp_secret, u.role, u.created_at, u.last_login_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token_hash = ? AND s.expires_at > ?
	`, tokenHash[:], now)

	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.TotpSecret, &u.Role, &u.CreatedAt, &u.LastLoginAt)
	if err != nil {
		return nil, ErrSessionExpired
	}

	return &u, nil
}

// DeleteSession menghapus sesi aktif saat logout
func (s *Store) DeleteSession(ctx context.Context, rawToken string) error {
	tokenHash := sha256.Sum256([]byte(rawToken))
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash[:])
	return err
}
