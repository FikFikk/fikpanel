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

	// Aktifkan kembali Global WAF sebelum menguji Virtual Patch & Simulator Payload
	toggleOnBody, _ := json.Marshal(map[string]bool{"enabled": true})
	reqToggleOn := httptest.NewRequest("POST", "/api/waf/global", bytes.NewReader(toggleOnBody))
	reqToggleOn.Header.Set("Content-Type", "application/json")
	reqToggleOn.Header.Set("X-Panel-Request", "1")
	reqToggleOn.AddCookie(sessionCookie)
	srv.server.Handler.ServeHTTP(httptest.NewRecorder(), reqToggleOn)

	// 5. Aktifkan Virtual Patch PHP2Shell (CVE-2024-4577) via POST /api/waf/virtual-patches/{id}
	vpBody, _ := json.Marshal(map[string]bool{"enabled": true})
	reqVP := httptest.NewRequest("POST", "/api/waf/virtual-patches/vp-php-cgi-2024-4577", bytes.NewReader(vpBody))
	reqVP.Header.Set("Content-Type", "application/json")
	reqVP.Header.Set("X-Panel-Request", "1")
	reqVP.AddCookie(sessionCookie)
	wVP := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wVP, reqVP)
	if wVP.Code != http.StatusOK || !strings.Contains(wVP.Body.String(), "vp-php-cgi-2024-4577") {
		t.Fatalf("ekspektasi 200 saat mengaktifkan virtual patch, dapat %d: %s", wVP.Code, wVP.Body.String())
	}

	// 6. Uji Simulator Payload via POST /api/waf/test-payload
	testPayloadBody, _ := json.Marshal(map[string]interface{}{
		"method": "POST",
		"path":   "/index.php?%ADd+allow_url_include%3d1",
	})
	reqSim := httptest.NewRequest("POST", "/api/waf/test-payload", bytes.NewReader(testPayloadBody))
	reqSim.Header.Set("Content-Type", "application/json")
	reqSim.Header.Set("X-Panel-Request", "1")
	reqSim.AddCookie(sessionCookie)
	wSim := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wSim, reqSim)
	if wSim.Code != http.StatusOK || !strings.Contains(wSim.Body.String(), `"blocked":true`) {
		t.Fatalf("ekspektasi payload PHP2Shell terblokir via API, dapat %d: %s", wSim.Code, wSim.Body.String())
	}

	// 7. Simpan dan baca konfigurasi Auto-Ban via POST & GET /api/waf/autoban
	abBody, _ := json.Marshal(map[string]interface{}{
		"enabled":         true,
		"threshold":       4,
		"window_minutes":  15,
		"ban_minutes":     120,
		"sync_ufw":        false,
		"exclude_private": true,
	})
	reqAB := httptest.NewRequest("POST", "/api/waf/autoban", bytes.NewReader(abBody))
	reqAB.Header.Set("Content-Type", "application/json")
	reqAB.Header.Set("X-Panel-Request", "1")
	reqAB.AddCookie(sessionCookie)
	wAB := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wAB, reqAB)
	if wAB.Code != http.StatusOK || !strings.Contains(wAB.Body.String(), `"threshold":4`) {
		t.Fatalf("ekspektasi simpan autoban sukses, dapat %d: %s", wAB.Code, wAB.Body.String())
	}

	// 8. Verifikasi endpoint daftar Virtual Patches
	reqListVP := httptest.NewRequest("GET", "/api/waf/virtual-patches", nil)
	reqListVP.AddCookie(sessionCookie)
	wListVP := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wListVP, reqListVP)
	if wListVP.Code != http.StatusOK || !strings.Contains(wListVP.Body.String(), "vp-react2shell-2025-55182") {
		t.Fatalf("ekspektasi daftar virtual patches memuat React2Shell, dapat %d: %s", wListVP.Code, wListVP.Body.String())
	}
}
