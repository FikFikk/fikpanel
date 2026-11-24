package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SaveACMEDNSProvider menyimpan kredensial API DNS provider untuk penerbitan & auto-renew DNS-01
func (s *Store) SaveACMEDNSProvider(ctx context.Context, provider string, creds map[string]string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return fmt.Errorf("nama DNS provider wajib diisi")
	}
	clean := make(map[string]string, len(creds))
	for k, v := range creds {
		clean[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO acme_dns_providers (provider, credentials_json, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(provider) DO UPDATE SET credentials_json = excluded.credentials_json, updated_at = excluded.updated_at`,
		provider, string(raw), time.Now().Unix())
	return err
}

// GetACMEDNSProvider membaca kredensial DNS-01 tersimpan; untuk Cloudflare otomatis fallback ke Cloudflare Hub aktif
func (s *Store) GetACMEDNSProvider(ctx context.Context, provider string) (map[string]string, string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT credentials_json FROM acme_dns_providers WHERE provider = ?`, provider).Scan(&raw)
	if err == nil && strings.TrimSpace(raw) != "" {
		out := map[string]string{}
		if jErr := json.Unmarshal([]byte(raw), &out); jErr == nil && len(out) > 0 {
			return out, "saved", nil
		}
	}

	// Khusus Cloudflare: manfaatkan token/global key yang sudah terhubung di Cloudflare Hub FikPanel
	if provider == "cloudflare" {
		var token, email string
		cfErr := s.db.QueryRowContext(ctx, `SELECT api_token, email FROM cloudflare_configs WHERE id = 1 AND connected = 1`).Scan(&token, &email)
		if cfErr == nil && strings.TrimSpace(token) != "" {
			out := map[string]string{}
			if strings.TrimSpace(email) != "" {
				out["email"] = strings.TrimSpace(email)
				out["api_key"] = strings.TrimSpace(token)
			} else {
				out["api_token"] = strings.TrimSpace(token)
			}
			return out, "cloudflare_hub", nil
		}
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	return nil, "", err
}

// ListConfiguredACMEDNSProviders mengembalikan peta provider -> sumber ("saved" | "cloudflare_hub")
func (s *Store) ListConfiguredACMEDNSProviders(ctx context.Context) map[string]string {
	result := map[string]string{}
	rows, err := s.db.QueryContext(ctx, `SELECT provider FROM acme_dns_providers`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil && p != "" {
				result[strings.ToLower(p)] = "saved"
			}
		}
	}
	if _, exists := result["cloudflare"]; !exists {
		var tok string
		if s.db.QueryRowContext(ctx, `SELECT api_token FROM cloudflare_configs WHERE id = 1 AND connected = 1`).Scan(&tok) == nil && strings.TrimSpace(tok) != "" {
			result["cloudflare"] = "cloudflare_hub"
		}
	}
	return result
}
