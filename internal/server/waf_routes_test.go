package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

func TestWAFRoutesDynamicAndSecure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("FIKPANEL_WAF_DIR", filepath.Join(tempDir, "waf"))

	s, err := store.Open(filepath.Join(tempDir, "test_waf_srv.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	if _, err := s.EnsureDevUser("root", ""); err != nil {
		t.Fatalf("failed to seed dev user: %v", err)
	}

	srv, err := NewServer(s, metrics.NewHub(), services.NewManager(s.DB()), 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Login untuk mendapatkan cookie sesi
	loginBody, _ := json.Marshal(map[string]string{"username": "root", "password": ""})
	reqLogin := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wLogin, reqLogin)
	if wLogin.Code != http.StatusOK {
		t.Fatalf("login failed: %d", wLogin.Code)
	}

	var sessionCookie *http.Cookie
	for _, c := range wLogin.Result().Cookies() {
		if c.Name == "panel_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("missing panel_session cookie")
	}

	// 1. Uji endpoint tile peta tanpa autentikasi harus ditolak (401)
	reqUnauthTile := httptest.NewRequest("GET", "/api/waf/tiles/2/1/1.png", nil)
	wUnauthTile := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wUnauthTile, reqUnauthTile)
	if wUnauthTile.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthenticated tile request, got %d", wUnauthTile.Code)
	}

	// 2. Uji endpoint tile peta dengan koordinat non-numerik harus ditolak (400)
	reqBadTile := httptest.NewRequest("GET", "/api/waf/tiles/invalid/x/y.png", nil)
	reqBadTile.AddCookie(sessionCookie)
	wBadTile := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wBadTile, reqBadTile)
	if wBadTile.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-numeric tile coordinates, got %d", wBadTile.Code)
	}

	// 3. Tambah aturan IP Blacklist dan periksa sinkronisasi Nginx WAF via /api/waf/enforcement
	ruleBody, _ := json.Marshal(map[string]string{
		"rule_type":  "ip_blacklist",
		"ip_version": "IPv4",
		"start_ip":   "203.0.113.77",
		"remarks":    "E2E Blocked IP",
	})
	reqRule := httptest.NewRequest("POST", "/api/waf/rules", bytes.NewReader(ruleBody))
	reqRule.Header.Set("Content-Type", "application/json")
	reqRule.Header.Set("X-Panel-Request", "1")
	reqRule.AddCookie(sessionCookie)
	wRule := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wRule, reqRule)
	if wRule.Code != http.StatusCreated {
		t.Fatalf("expected 201 on add rule, got %d: %s", wRule.Code, wRule.Body.String())
	}

	reqEnf := httptest.NewRequest("GET", "/api/waf/enforcement", nil)
	reqEnf.AddCookie(sessionCookie)
	wEnf := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wEnf, reqEnf)
	if wEnf.Code != http.StatusOK || !strings.Contains(wEnf.Body.String(), "deny 203.0.113.77;") {
		t.Fatalf("expected enforcement preview to contain deny 203.0.113.77;, got %d: %s", wEnf.Code, wEnf.Body.String())
	}

	// 4. Toggle Global WAF ke false via POST /api/waf/global
	toggleBody, _ := json.Marshal(map[string]bool{"enabled": false})
	reqToggle := httptest.NewRequest("POST", "/api/waf/global", bytes.NewReader(toggleBody))
	reqToggle.Header.Set("Content-Type", "application/json")
	reqToggle.Header.Set("X-Panel-Request", "1")
	reqToggle.AddCookie(sessionCookie)
	wToggle := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wToggle, reqToggle)
	if wToggle.Code != http.StatusOK || !strings.Contains(wToggle.Body.String(), `"waf_global_enabled":false`) {
		t.Fatalf("expected global WAF toggle false to succeed, got %d: %s", wToggle.Code, wToggle.Body.String())
	}
}
