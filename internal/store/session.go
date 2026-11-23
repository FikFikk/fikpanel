package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
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

// ActiveSessionView merepresentasikan informasi sesi perangkat yang aman ditampilkan di antarmuka Settings
type ActiveSessionView struct {
	ID        string `json:"id"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	IsCurrent bool   `json:"is_current"`
}

// ListUserSessions mengembalikan daftar sesi aktif milik pengguna beserta penanda sesi saat ini
func (s *Store) ListUserSessions(ctx context.Context, userID int64, currentRawToken string) ([]ActiveSessionView, error) {
	now := time.Now().Unix()
	var currentHex string
	if currentRawToken != "" {
		sum := sha256.Sum256([]byte(currentRawToken))
		currentHex = hex.EncodeToString(sum[:])
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT token_hash, COALESCE(ip, ''), COALESCE(user_agent, ''), created_at, expires_at
		FROM sessions
		WHERE user_id = ? AND expires_at > ?
		ORDER BY created_at DESC
	`, userID, now)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca daftar sesi aktif: %w", err)
	}
	defer rows.Close()

	list := make([]ActiveSessionView, 0)
	for rows.Next() {
		var hashBytes []byte
		var v ActiveSessionView
		if err := rows.Scan(&hashBytes, &v.IP, &v.UserAgent, &v.CreatedAt, &v.ExpiresAt); err != nil {
			return nil, err
		}
		fullHex := hex.EncodeToString(hashBytes)
		if len(fullHex) >= 16 {
			v.ID = fullHex[:16]
		} else {
			v.ID = fullHex
		}
		v.IsCurrent = currentHex != "" && fullHex == currentHex
		list = append(list, v)
	}
	return list, nil
}

// RevokeSessionByPrefix menghapus satu sesi milik pengguna berdasarkan prefiks hex hash token
func (s *Store) RevokeSessionByPrefix(ctx context.Context, userID int64, idPrefix string) (bool, error) {
	if len(idPrefix) < 8 {
		return false, errors.New("ID sesi tidak valid")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT token_hash FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var target []byte
	for rows.Next() {
		var hb []byte
		if err := rows.Scan(&hb); err != nil {
			return false, err
		}
		if strings.HasPrefix(hex.EncodeToString(hb), idPrefix) {
			target = hb
			break
		}
	}
	if len(target) == 0 {
		return false, nil
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ? AND user_id = ?`, target, userID)
	return err == nil, err
}

// RevokeOtherSessions mencabut seluruh sesi login milik pengguna kecuali sesi saat ini
func (s *Store) RevokeOtherSessions(ctx context.Context, userID int64, currentRawToken string) (int64, error) {
	if currentRawToken == "" {
		res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		return n, nil
	}
	sum := sha256.Sum256([]byte(currentRawToken))
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, userID, sum[:])
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
