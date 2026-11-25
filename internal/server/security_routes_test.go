package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

// newSecurityTestServer membangun instance Server terisolasi untuk pengujian integrasi endpoint Security Center
func newSecurityTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "srv_sec_test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("gagal membuka store pengujian: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	user, err := st.EnsureDevUser("secadmin", "SuperSecret123!")
	if err != nil {
		t.Fatalf("gagal membuat akun admin pengujian: %v", err)
	}

	hub := metrics.NewHub()
	svc := services.NewManager(st.DB())
	srv, err := NewServer(st, hub, svc, 1405)
	if err != nil {
		t.Fatalf("gagal menginisialisasi Server: %v", err)
	}
	t.Cleanup(func() { srv.bgCancel() })

	token, err := st.CreateSession(context.Background(), user.ID, "127.0.0.1", "go-test", 3600*1e9)
	if err != nil {
		t.Fatalf("gagal membuat sesi uji: %v", err)
	}
	return srv, token
}

// TestSecurityBlacklistEnforcementAndRoutes memverifikasi penegakan IP Blacklist di middleware dan endpoint Security Center
func TestSecurityBlacklistEnforcementAndRoutes(t *testing.T) {
	srv, token := newSecurityTestServer(t)

	// 1. Endpoint overview harus merespons 200 dengan 6 pilar perlindungan
	req := httptest.NewRequest(http.MethodGet, "/api/security/overview", nil)
	req.RemoteAddr = "127.0.0.1:45000"
	req.AddCookie(&http.Cookie{Name: "panel_session", Value: token})
	rec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/security/overview diharapkan 200, dapat %d: %s", rec.Code, rec.Body.String())
	}

	var ovResp struct {
		OK       bool `json:"ok"`
		Overview struct {
			TotalCount int `json:"total_count"`
		} `json:"overview"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ovResp); err != nil || !ovResp.OK || ovResp.Overview.TotalCount != 6 {
		t.Fatalf("payload overview tidak sesuai: %s", rec.Body.String())
	}

	// 2. Tambahkan IP publik ke blacklist melalui API
	body := bytes.NewBufferString(`{"ip":"203.0.113.88","reason":"Uji blokir middleware","duration_minutes":60,"sync_ufw":false}`)
	reqAdd := httptest.NewRequest(http.MethodPost, "/api/security/bruteforce/blacklist", body)
	reqAdd.RemoteAddr = "127.0.0.1:45001"
	reqAdd.Header.Set("X-Panel-Request", "1")
	reqAdd.AddCookie(&http.Cookie{Name: "panel_session", Value: token})
	recAdd := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(recAdd, reqAdd)
	if recAdd.Code != http.StatusOK {
		t.Fatalf("POST /api/security/bruteforce/blacklist gagal (%d): %s", recAdd.Code, recAdd.Body.String())
	}

	// 3. Request dari IP yang masuk blacklist wajib ditolak HTTP 403 oleh middleware
	reqBlocked := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"secadmin","password":"SuperSecret123!"}`))
	reqBlocked.RemoteAddr = "203.0.113.88:51234"
	recBlocked := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(recBlocked, reqBlocked)
	if recBlocked.Code != http.StatusForbidden {
		t.Fatalf("IP blacklist seharusnya mendapat 403 Forbidden, dapat %d: %s", recBlocked.Code, recBlocked.Body.String())
	}

	// 4. Jalankan audit keamanan via POST /api/security/scan dan pastikan tersimpan
	reqScan := httptest.NewRequest(http.MethodPost, "/api/security/scan", nil)
	reqScan.RemoteAddr = "127.0.0.1:45002"
	reqScan.Header.Set("X-Panel-Request", "1")
	reqScan.AddCookie(&http.Cookie{Name: "panel_session", Value: token})
	recScan := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(recScan, reqScan)
	if recScan.Code != http.StatusOK {
		t.Fatalf("POST /api/security/scan gagal (%d): %s", recScan.Code, recScan.Body.String())
	}
}
