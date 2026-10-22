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

func TestServerAuthAndProtectedRoutes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_srv_test_*")
	if err != nil {
		t.Fatalf("Gagal membuat direktori sementara: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_srv.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Gagal membuka store SQLite: %v", err)
	}
	defer s.Close()

	// Seed user dev (root / "")
	_, err = s.EnsureDevUser("root", "")
	if err != nil {
		t.Fatalf("Gagal inisialisasi dev user: %v", err)
	}

	m := metrics.NewHub()
	svc := services.NewManager(s.DB())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = svc.SeedDefaultServices(ctx)

	srv, err := NewServer(s, m, svc, 0)
	if err != nil {
		t.Fatalf("Gagal membuat instance server: %v", err)
	}

	// 1. Uji Login dengan root dan password kosong
	loginBody, _ := json.Marshal(map[string]string{
		"username": "root",
		"password": "",
	})

	reqLogin := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()

	srv.server.Handler.ServeHTTP(wLogin, reqLogin)

	if wLogin.Code != http.StatusOK {
		t.Fatalf("Login gagal, status code: %d, body: %s", wLogin.Code, wLogin.Body.String())
	}

	// Ambil cookie sesi dari response login
	cookies := wLogin.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "panel_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("Cookie panel_session tidak ditemukan pada response login")
	}

	// 2. Uji endpoint terproteksi /api/auth/me menggunakan cookie sesi
	reqMe := httptest.NewRequest("GET", "/api/auth/me", nil)
	reqMe.AddCookie(sessionCookie)
	wMe := httptest.NewRecorder()

	srv.server.Handler.ServeHTTP(wMe, reqMe)

	if wMe.Code != http.StatusOK {
		t.Fatalf("Akses /api/auth/me gagal: %d, body: %s", wMe.Code, wMe.Body.String())
	}

	var meResp map[string]interface{}
	if err := json.NewDecoder(wMe.Body).Decode(&meResp); err != nil {
		t.Fatalf("Gagal decode JSON me: %v", err)
	}
	if meResp["username"] != "root" {
		t.Errorf("Username tidak sesuai, diharapkan root, didapat: %v", meResp["username"])
	}

	// 3. Uji proteksi CSRF pada request mutasi POST tanpa header
	actionBody, _ := json.Marshal(map[string]string{"action": "restart"})
	reqActionNoCSRF := httptest.NewRequest("POST", "/api/services/nginx.service/action", bytes.NewReader(actionBody))
	reqActionNoCSRF.AddCookie(sessionCookie)
	wActionNoCSRF := httptest.NewRecorder()

	srv.server.Handler.ServeHTTP(wActionNoCSRF, reqActionNoCSRF)
	if wActionNoCSRF.Code != http.StatusForbidden {
		t.Errorf("Request tanpa header CSRF harus ditolak 403, didapat: %d", wActionNoCSRF.Code)
	}

	// 4. Uji request mutasi POST DENGAN header CSRF X-Panel-Request: 1
	reqActionValid := httptest.NewRequest("POST", "/api/services/nginx.service/action", bytes.NewReader(actionBody))
	reqActionValid.AddCookie(sessionCookie)
	reqActionValid.Header.Set("X-Panel-Request", "1")
	wActionValid := httptest.NewRecorder()

	srv.server.Handler.ServeHTTP(wActionValid, reqActionValid)
	if wActionValid.Code != http.StatusOK {
		t.Errorf("Request mutasi dengan header CSRF gagal: %d, body: %s", wActionValid.Code, wActionValid.Body.String())
	}

	// 5. Uji Logout
	reqLogout := httptest.NewRequest("POST", "/api/auth/logout", nil)
	reqLogout.AddCookie(sessionCookie)
	reqLogout.Header.Set("X-Panel-Request", "1")
	wLogout := httptest.NewRecorder()

	srv.server.Handler.ServeHTTP(wLogout, reqLogout)
	if wLogout.Code != http.StatusOK {
		t.Errorf("Logout gagal: %d", wLogout.Code)
	}
}
