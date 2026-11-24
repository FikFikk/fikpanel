package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

func setupTestServerWithCookie(t *testing.T) (*Server, *http.Cookie, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "fikpanel_acme_srv_*")
	if err != nil {
		t.Fatalf("Gagal membuat direktori sementara: %v", err)
	}
	dbPath := filepath.Join(tempDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Gagal membuka SQLite: %v", err)
	}
	if _, err := s.EnsureDevUser("root", ""); err != nil {
		s.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Gagal seed dev user: %v", err)
	}

	m := metrics.NewHub()
	svc := services.NewManager(s.DB())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = svc.SeedDefaultServices(ctx)

	srv, err := NewServer(s, m, svc, 0)
	if err != nil {
		s.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Gagal membuat server: %v", err)
	}
	// Aktifkan certManager agar endpoint /api/settings/tls/acme bisa diuji validasinya
	cm, err := newCertManager(filepath.Join(tempDir, "tls"), func(string, ...any) {})
	if err != nil {
		s.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Gagal membuat certManager uji: %v", err)
	}
	srv.certs = cm

	loginBody, _ := json.Marshal(map[string]string{"username": "root", "password": ""})
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wLogin, reqLogin)
	if wLogin.Code != http.StatusOK {
		s.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Login gagal: %d %s", wLogin.Code, wLogin.Body.String())
	}

	var sessionCookie *http.Cookie
	for _, c := range wLogin.Result().Cookies() {
		if c.Name == "panel_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		s.Close()
		os.RemoveAll(tempDir)
		t.Fatal("Cookie panel_session tidak ditemukan")
	}

	cleanup := func() {
		s.Close()
		os.RemoveAll(tempDir)
	}
	return srv, sessionCookie, cleanup
}

func TestACMEDNSProvidersAndPanelTLSValidation(t *testing.T) {
	srv, cookie, cleanup := setupTestServerWithCookie(t)
	defer cleanup()

	// 1. GET /api/ssl/dns-providers mengembalikan 5 provider ringan
	reqList := httptest.NewRequest(http.MethodGet, "/api/ssl/dns-providers", nil)
	reqList.AddCookie(cookie)
	wList := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("GET /api/ssl/dns-providers gagal: %d %s", wList.Code, wList.Body.String())
	}
	var listResp struct {
		OK        bool `json:"ok"`
		Providers []struct {
			ID         string `json:"id"`
			Configured bool   `json:"configured"`
			Source     string `json:"source"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(wList.Body).Decode(&listResp); err != nil {
		t.Fatalf("Decode JSON gagal: %v", err)
	}
	if !listResp.OK || len(listResp.Providers) != 5 {
		t.Fatalf("Diharapkan 5 DNS provider, didapat: %d", len(listResp.Providers))
	}

	// 2. POST /api/ssl/dns-providers/porkbun menyimpan kredensial dan menandai configured=true
	saveBody, _ := json.Marshal(map[string]any{
		"credentials": map[string]string{
			"api_key":        "pk1_demo",
			"secret_api_key": "sk1_demo",
		},
	})
	reqSave := httptest.NewRequest(http.MethodPost, "/api/ssl/dns-providers/porkbun", bytes.NewReader(saveBody))
	reqSave.AddCookie(cookie)
	reqSave.Header.Set("Content-Type", "application/json")
	reqSave.Header.Set("X-Panel-Request", "1")
	wSave := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wSave, reqSave)
	if wSave.Code != http.StatusOK {
		t.Fatalf("POST /api/ssl/dns-providers/porkbun gagal: %d %s", wSave.Code, wSave.Body.String())
	}

	var saveResp struct {
		OK        bool `json:"ok"`
		Providers []struct {
			ID         string `json:"id"`
			Configured bool   `json:"configured"`
			Source     string `json:"source"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(wSave.Body).Decode(&saveResp); err != nil {
		t.Fatalf("Decode respons simpan gagal: %v", err)
	}
	foundPorkbun := false
	for _, p := range saveResp.Providers {
		if p.ID == "porkbun" && p.Configured && p.Source == "saved" {
			foundPorkbun = true
		}
	}
	if !foundPorkbun {
		t.Fatalf("Provider porkbun seharusnya berstatus configured=true (saved)")
	}

	// 3. Tolak provider tidak dikenal
	reqBadProv := httptest.NewRequest(http.MethodPost, "/api/ssl/dns-providers/unknown_dns", bytes.NewReader(saveBody))
	reqBadProv.AddCookie(cookie)
	reqBadProv.Header.Set("Content-Type", "application/json")
	reqBadProv.Header.Set("X-Panel-Request", "1")
	wBadProv := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wBadProv, reqBadProv)
	if wBadProv.Code != http.StatusBadRequest {
		t.Fatalf("Diharapkan 400 untuk provider tidak didukung, didapat: %d", wBadProv.Code)
	}

	// 4. POST /api/settings/tls/acme menolak IP address / domain tidak valid
	badAcmeBody, _ := json.Marshal(map[string]any{
		"domain":       "127.0.0.1",
		"dns_provider": "cloudflare",
	})
	reqBadAcme := httptest.NewRequest(http.MethodPost, "/api/settings/tls/acme", bytes.NewReader(badAcmeBody))
	reqBadAcme.AddCookie(cookie)
	reqBadAcme.Header.Set("Content-Type", "application/json")
	reqBadAcme.Header.Set("X-Panel-Request", "1")
	wBadAcme := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wBadAcme, reqBadAcme)
	if wBadAcme.Code != http.StatusBadRequest {
		t.Fatalf("Diharapkan 400 untuk domain IP, didapat: %d %s", wBadAcme.Code, wBadAcme.Body.String())
	}

	// 5. POST /api/settings/tls/acme menolak jika kredensial provider belum ada
	noCredBody, _ := json.Marshal(map[string]any{
		"domain":       "panel.example.com",
		"dns_provider": "digitalocean",
	})
	reqNoCred := httptest.NewRequest(http.MethodPost, "/api/settings/tls/acme", bytes.NewReader(noCredBody))
	reqNoCred.AddCookie(cookie)
	reqNoCred.Header.Set("Content-Type", "application/json")
	reqNoCred.Header.Set("X-Panel-Request", "1")
	wNoCred := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wNoCred, reqNoCred)
	if wNoCred.Code != http.StatusBadRequest {
		t.Fatalf("Diharapkan 400 ketika kredensial DigitalOcean kosong, didapat: %d %s", wNoCred.Code, wNoCred.Body.String())
	}
}
