package cloudflare

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/store"
)

// Config menyimpan kredensial dan status integrasi Cloudflare
type Config struct {
	Connected      bool   `json:"connected"`
	APITokenMasked string `json:"api_token_masked"`
	Email          string `json:"email"`
	AccountName    string `json:"account_name"`
	AccountID      string `json:"account_id"`
	LastSync       int64  `json:"last_sync"`
}

// Zone merepresentasikan domain di akun Cloudflare
type Zone struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Status          string      `json:"status"` // 'active', 'pending', dll
	Paused          bool        `json:"paused"`
	DevelopmentMode int         `json:"development_mode"`
	SSLMode         string      `json:"ssl_mode"` // 'full', 'strict', 'flexible', 'off'
	DNSRecords      []DNSRecord `json:"dns_records"`
	PointsToServer  bool        `json:"points_to_server"`
}

// DNSRecord merepresentasikan entri DNS (A, AAAA, CNAME)
type DNSRecord struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Name           string `json:"name"`
	Content        string `json:"content"`
	Proxied        bool   `json:"proxied"`
	TTL            int    `json:"ttl"`
	PointsToServer bool   `json:"points_to_server"`
}

// Client mengelola panggilan REST API ke Cloudflare v4 endpoint
type Client struct {
	store      *store.Store
	httpClient *http.Client
	mu         sync.RWMutex
}

// NewClient membuat instance client Cloudflare baru
func NewClient(s *store.Store) *Client {
	return &Client{
		store: s,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// GetConfig membaca konfigurasi Cloudflare dari database
func (c *Client) GetConfig(ctx context.Context) (*Config, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var token, email, accName, accID string
	var connected int
	var updatedAt int64

	err := c.store.DB().QueryRowContext(ctx, `
		SELECT api_token, email, connected, account_name, account_id, updated_at
		FROM cloudflare_configs WHERE id = 1
	`).Scan(&token, &email, &connected, &accName, &accID, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return &Config{Connected: false}, nil
		}
		return nil, err
	}

	masked := ""
	if len(token) > 8 {
		masked = token[:4] + "••••••••" + token[len(token)-4:]
	} else if len(token) > 0 {
		masked = "••••••••"
	}

	return &Config{
		Connected:      connected == 1,
		APITokenMasked: masked,
		Email:          email,
		AccountName:    accName,
		AccountID:      accID,
		LastSync:       updatedAt,
	}, nil
}

func (c *Client) getRawToken(ctx context.Context) (string, error) {
	var token string
	err := c.store.DB().QueryRowContext(ctx, "SELECT api_token FROM cloudflare_configs WHERE id = 1 AND connected = 1").Scan(&token)
	if err != nil {
		return "", fmt.Errorf("Cloudflare belum terhubung atau API token tidak ditemukan")
	}
	return strings.TrimSpace(token), nil
}

// SaveConfig menyimpan token API Cloudflare, memverifikasinya ke Cloudflare API, dan mencatat profil
func (c *Client) SaveConfig(ctx context.Context, apiToken, email string) (*Config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cleanToken := strings.TrimSpace(apiToken)
	if cleanToken == "" {
		return nil, fmt.Errorf("API Token Cloudflare tidak boleh kosong")
	}

	// Verifikasi token via Cloudflare API v4
	accName := "Cloudflare User"
	accID := ""

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.cloudflare.com/client/v4/user/tokens/verify", nil)
	if err == nil {
		req.Header.Set("Authorization", "Bearer "+cleanToken)
		req.Header.Set("Content-Type", "application/json")
		resp, errResp := c.httpClient.Do(req)
		if errResp == nil {
			defer resp.Body.Close()
			var vResp struct {
				Success bool `json:"success"`
				Result  struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"result"`
				Errors []struct {
					Message string `json:"message"`
				} `json:"errors"`
			}
			if errJson := json.NewDecoder(resp.Body).Decode(&vResp); errJson == nil && vResp.Success {
				accName = "Verified API Token"
				accID = vResp.Result.ID
			} else if len(vResp.Errors) > 0 {
				return nil, fmt.Errorf("verifikasi Cloudflare gagal: %s", vResp.Errors[0].Message)
			}
		}
	}

	now := time.Now().Unix()
	_, err = c.store.DB().ExecContext(ctx, `
		INSERT INTO cloudflare_configs (id, api_token, email, connected, account_name, account_id, created_at, updated_at)
		VALUES (1, ?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			api_token = excluded.api_token,
			email = excluded.email,
			connected = 1,
			account_name = excluded.account_name,
			account_id = excluded.account_id,
			updated_at = excluded.updated_at
	`, cleanToken, email, accName, accID, now, now)

	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan konfigurasi Cloudflare: %w", err)
	}

	masked := cleanToken
	if len(cleanToken) > 8 {
		masked = cleanToken[:4] + "••••••••" + cleanToken[len(cleanToken)-4:]
	}

	return &Config{
		Connected:      true,
		APITokenMasked: masked,
		Email:          email,
		AccountName:    accName,
		AccountID:      accID,
		LastSync:       now,
	}, nil
}

// Disconnect memutuskan tautan integrasi Cloudflare
func (c *Client) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.store.DB().ExecContext(ctx, "DELETE FROM cloudflare_configs WHERE id = 1")
	return err
}

// ListZones mengambil daftar seluruh domain (zones) dari akun Cloudflare dan mengecek arah DNS ke server ini
func (c *Client) ListZones(ctx context.Context, serverIP string) ([]Zone, error) {
	token, err := c.getRawToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.cloudflare.com/client/v4/zones?per_page=50", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("koneksi ke Cloudflare gagal: %w", err)
	}
	defer resp.Body.Close()

	var cfResp struct {
		Success bool `json:"success"`
		Result  []struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			Status          string `json:"status"`
			Paused          bool   `json:"paused"`
			DevelopmentMode int    `json:"development_mode"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, fmt.Errorf("gagal membaca respon Cloudflare: %w", err)
	}
	if !cfResp.Success && len(cfResp.Errors) > 0 {
		return nil, fmt.Errorf("Cloudflare API error: %s", cfResp.Errors[0].Message)
	}

	var zones []Zone
	cleanServerIP := strings.TrimSpace(serverIP)

	for _, z := range cfResp.Result {
		zoneItem := Zone{
			ID:              z.ID,
			Name:            z.Name,
			Status:          z.Status,
			Paused:          z.Paused,
			DevelopmentMode: z.DevelopmentMode,
			SSLMode:         "Full (Strict)",
		}

		// Ambil rekaman DNS untuk domain ini
		dnsRecords, _ := c.ListDNSRecords(ctx, z.ID, cleanServerIP)
		zoneItem.DNSRecords = dnsRecords

		// Periksa apakah ada record A/CNAME yang mengarah ke IP server kita
		pointsHere := false
		for _, r := range dnsRecords {
			if r.PointsToServer {
				pointsHere = true
				break
			}
		}
		zoneItem.PointsToServer = pointsHere

		zones = append(zones, zoneItem)
	}

	return zones, nil
}

// ListDNSRecords mengambil rekaman DNS spesifik dari satu zone
func (c *Client) ListDNSRecords(ctx context.Context, zoneID, serverIP string) ([]DNSRecord, error) {
	token, err := c.getRawToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?per_page=100", zoneID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var cfResp struct {
		Success bool `json:"success"`
		Result  []struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Name    string `json:"name"`
			Content string `json:"content"`
			Proxied bool   `json:"proxied"`
			TTL     int    `json:"ttl"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, err
	}

	var records []DNSRecord
	cleanServerIP := strings.TrimSpace(serverIP)

	for _, r := range cfResp.Result {
		pointsHere := false
		if cleanServerIP != "" && strings.TrimSpace(r.Content) == cleanServerIP {
			pointsHere = true
		}

		records = append(records, DNSRecord{
			ID:             r.ID,
			Type:           r.Type,
			Name:           r.Name,
			Content:        r.Content,
			Proxied:        r.Proxied,
			TTL:            r.TTL,
			PointsToServer: pointsHere,
		})
	}

	return records, nil
}

// PointZoneToServer mengarahkan record A domain ke IP server ini secara otomatis dengan toggle Cloudflare proxy
func (c *Client) PointZoneToServer(ctx context.Context, zoneID, domain, serverIP string, proxied bool) error {
	token, err := c.getRawToken(ctx)
	if err != nil {
		return err
	}

	cleanIP := strings.TrimSpace(serverIP)
	if cleanIP == "" {
		return fmt.Errorf("IP server tidak valid")
	}

	// 1. Periksa apakah sudah ada record A untuk domain ini
	records, err := c.ListDNSRecords(ctx, zoneID, cleanIP)
	if err != nil {
		return err
	}

	var existingID string
	for _, r := range records {
		if r.Type == "A" && (r.Name == domain || r.Name == "@") {
			existingID = r.ID
			break
		}
	}

	payload := map[string]interface{}{
		"type":    "A",
		"name":    domain,
		"content": cleanIP,
		"ttl":     1, // Auto TTL
		"proxied": proxied,
	}
	bodyData, _ := json.Marshal(payload)

	var req *http.Request
	if existingID != "" {
		// Update record existing
		url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, existingID)
		req, err = http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(bodyData))
	} else {
		// Buat record baru
		url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
		req, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyData))
	}
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gagal memperbarui DNS di Cloudflare: %s", string(respBody))
	}

	return nil
}

// PurgeZoneCache mengosongkan cache CDN Cloudflare untuk seluruh domain
func (c *Client) PurgeZoneCache(ctx context.Context, zoneID string) error {
	token, err := c.getRawToken(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/purge_cache", zoneID)
	payload := map[string]interface{}{
		"purge_everything": true,
	}
	bodyData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyData))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("gagal mengosongkan cache Cloudflare (HTTP %d)", resp.StatusCode)
	}

	return nil
}

// ToggleDevelopmentMode mengaktifkan atau menonaktifkan mode pengembang (bypass cache) selama 3 jam
func (c *Client) ToggleDevelopmentMode(ctx context.Context, zoneID string, enable bool) error {
	token, err := c.getRawToken(ctx)
	if err != nil {
		return err
	}

	val := "off"
	if enable {
		val = "on"
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/settings/development_mode", zoneID)
	payload := map[string]interface{}{
		"value": val,
	}
	bodyData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(bodyData))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("gagal mengubah mode pengembangan di Cloudflare (HTTP %d)", resp.StatusCode)
	}

	return nil
}
