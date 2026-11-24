package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// MCPToken merepresentasikan token Bearer ber-scope untuk klien MCP (Google Antigravity)
type MCPToken struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Name        string `json:"name"`
	TokenPrefix string `json:"token_prefix"`
	Scope       string `json:"scope"` // "readonly" atau "operator"
	Enabled     bool   `json:"enabled"`
	CallCount   int64  `json:"call_count"`
	LastUsedAt  int64  `json:"last_used_at"`
	LastUsedIP  string `json:"last_used_ip"`
	CreatedAt   int64  `json:"created_at"`
}

func hashMCPToken(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

// CreateMCPToken membangkitkan token Bearer acak 256-bit dan menyimpan hash SHA-256 ke SQLite
func (s *Store) CreateMCPToken(ctx context.Context, userID int64, name, scope string) (*MCPToken, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", fmt.Errorf("nama token wajib diisi")
	}
	if scope != "operator" {
		scope = "readonly"
	}

	rawBytes := make([]byte, 24)
	if _, err := rand.Read(rawBytes); err != nil {
		return nil, "", fmt.Errorf("gagal membangkitkan entropi token: %w", err)
	}
	plain := "fkp_" + hex.EncodeToString(rawBytes)
	prefix := plain[:12] + "…"
	hash := hashMCPToken(plain)
	now := time.Now().Unix()

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO mcp_tokens (user_id, name, token_prefix, token_hash, scope, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, 1, ?)`,
		userID, name, prefix, hash, scope, now)
	if err != nil {
		return nil, "", fmt.Errorf("gagal menyimpan token MCP: %w", err)
	}
	id, _ := res.LastInsertId()

	tok := &MCPToken{
		ID:          id,
		UserID:      userID,
		Name:        name,
		TokenPrefix: prefix,
		Scope:       scope,
		Enabled:     true,
		CreatedAt:   now,
	}
	return tok, plain, nil
}

// ListMCPTokens mengembalikan seluruh token MCP terdaftar (tanpa nilai rahasia)
func (s *Store) ListMCPTokens(ctx context.Context) ([]MCPToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, token_prefix, scope, enabled, call_count, last_used_at, last_used_ip, created_at
		 FROM mcp_tokens ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca daftar token MCP: %w", err)
	}
	defer rows.Close()

	list := make([]MCPToken, 0)
	for rows.Next() {
		var t MCPToken
		var en int
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.TokenPrefix, &t.Scope, &en, &t.CallCount, &t.LastUsedAt, &t.LastUsedIP, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Enabled = en == 1
		list = append(list, t)
	}
	return list, rows.Err()
}

// VerifyMCPToken memvalidasi token Bearer mentah dan mengembalikan metadata bila valid & aktif
func (s *Store) VerifyMCPToken(ctx context.Context, rawToken string) (*MCPToken, error) {
	rawToken = strings.TrimSpace(rawToken)
	if !strings.HasPrefix(rawToken, "fkp_") || len(rawToken) < 20 {
		return nil, fmt.Errorf("format token tidak valid")
	}
	hash := hashMCPToken(rawToken)
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, token_prefix, scope, enabled, call_count, last_used_at, last_used_ip, created_at
		 FROM mcp_tokens WHERE token_hash = ?`, hash)
	var t MCPToken
	var en int
	if err := row.Scan(&t.ID, &t.UserID, &t.Name, &t.TokenPrefix, &t.Scope, &en, &t.CallCount, &t.LastUsedAt, &t.LastUsedIP, &t.CreatedAt); err != nil {
		return nil, fmt.Errorf("token tidak dikenali")
	}
	t.Enabled = en == 1
	if !t.Enabled {
		return nil, fmt.Errorf("token dinonaktifkan")
	}
	return &t, nil
}

// TouchMCPToken memperbarui waktu akses terakhir, IP, dan hitungan pemanggilan tool MCP
func (s *Store) TouchMCPToken(ctx context.Context, id int64, ip string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE mcp_tokens SET call_count = call_count + 1, last_used_at = ?, last_used_ip = ? WHERE id = ?`,
		time.Now().Unix(), strings.TrimSpace(ip), id)
	return err
}

// UpdateMCPToken mengubah nama, scope, atau status aktif token MCP
func (s *Store) UpdateMCPToken(ctx context.Context, id int64, name, scope string, enabled bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("nama token tidak boleh kosong")
	}
	if scope != "operator" {
		scope = "readonly"
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE mcp_tokens SET name = ?, scope = ?, enabled = ? WHERE id = ?`,
		name, scope, boolToInt(enabled), id)
	return err
}

// DeleteMCPToken mencabut dan menghapus token MCP secara permanen
func (s *Store) DeleteMCPToken(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM mcp_tokens WHERE id = ?`, id)
	return err
}
