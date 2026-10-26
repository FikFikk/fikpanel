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

func TestDiscoveryAndK8sEndpoints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_disc_test_*")
	if err != nil {
		t.Fatalf("Gagal membuat direktori sementara: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_disc.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Gagal membuka store SQLite: %v", err)
	}
	defer s.Close()

	user, err := s.EnsureDevUser("root", "")
	if err != nil {
		t.Fatalf("Gagal membuat user dev: %v", err)
	}

	m := metrics.NewHub()
	svc := services.NewManager(s.DB())
	srv, err := NewServer(s, m, svc, 0)
	if err != nil {
		t.Fatalf("Gagal membuat server: %v", err)
	}

	token, err := s.CreateSession(context.Background(), user.ID, "127.0.0.1", "test-agent", time.Hour)
	if err != nil {
		t.Fatalf("Gagal membuat session: %v", err)
	}

	// 1. Uji GET /api/discovery/config
	reqDiscConf := httptest.NewRequest("GET", "/api/discovery/config", nil)
	reqDiscConf.AddCookie(httpCookieSession(token))
	wDiscConf := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wDiscConf, reqDiscConf)
	if wDiscConf.Code != 200 {
		t.Errorf("GET /api/discovery/config mengharapkan status 200, didapat %d: %s", wDiscConf.Code, wDiscConf.Body.String())
	}

	// 2. Uji POST /api/discovery/config
	saveConfBody, _ := json.Marshal(map[string]interface{}{
		"roots": []string{"/home/fikfikk/projects", "/var/www"},
	})
	reqSaveConf := httptest.NewRequest("POST", "/api/discovery/config", bytes.NewReader(saveConfBody))
	reqSaveConf.Header.Set("Content-Type", "application/json")
	reqSaveConf.Header.Set("X-Panel-Request", "true")
	reqSaveConf.AddCookie(httpCookieSession(token))
	wSaveConf := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wSaveConf, reqSaveConf)
	if wSaveConf.Code != 200 {
		t.Errorf("POST /api/discovery/config mengharapkan status 200, didapat %d: %s", wSaveConf.Code, wSaveConf.Body.String())
	}

	// 3. Uji POST /api/discovery/scan
	reqScan := httptest.NewRequest("POST", "/api/discovery/scan", bytes.NewReader([]byte("{}")))
	reqScan.Header.Set("Content-Type", "application/json")
	reqScan.Header.Set("X-Panel-Request", "true")
	reqScan.AddCookie(httpCookieSession(token))
	wScan := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wScan, reqScan)
	if wScan.Code != 200 {
		t.Errorf("POST /api/discovery/scan mengharapkan status 200, didapat %d: %s", wScan.Code, wScan.Body.String())
	}

	// 4. Uji POST /api/discovery/adopt
	adoptBody, _ := json.Marshal(map[string]interface{}{
		"sites": []map[string]interface{}{
			{
				"primary_domain": "kostash.test",
				"aliases":       []string{"www.kostash.test"},
				"document_root":  "/home/fikfikk/projects/web/kostash",
				"has_php":        true,
				"has_ssl":        false,
			},
		},
	})
	reqAdopt := httptest.NewRequest("POST", "/api/discovery/adopt", bytes.NewReader(adoptBody))
	reqAdopt.Header.Set("Content-Type", "application/json")
	reqAdopt.Header.Set("X-Panel-Request", "true")
	reqAdopt.AddCookie(httpCookieSession(token))
	wAdopt := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wAdopt, reqAdopt)
	if wAdopt.Code != 200 {
		t.Errorf("POST /api/discovery/adopt mengharapkan status 200, didapat %d: %s", wAdopt.Code, wAdopt.Body.String())
	}

	// Verifikasi situs diadopsi ke SQLite
	sitesList, err := s.ListSites(context.Background())
	if err != nil {
		t.Fatalf("Gagal membaca daftar situs: %v", err)
	}
	found := false
	for _, st := range sitesList {
		if st.Domain == "kostash.test" {
			found = true
			if st.RootPath != "/home/fikfikk/projects/web/kostash" {
				t.Errorf("RootPath situs salah: got %s, want /home/fikfikk/projects/web/kostash", st.RootPath)
			}
			break
		}
	}
	if !found {
		t.Errorf("Situs yang diadopsi tidak ditemukan di daftar situs SQLite")
	}

	// 5. Uji GET /api/k8s/status (Fallback jika k8s belum ada)
	reqK8s := httptest.NewRequest("GET", "/api/k8s/status", nil)
	reqK8s.AddCookie(httpCookieSession(token))
	wK8s := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wK8s, reqK8s)
	if wK8s.Code != 200 {
		t.Errorf("GET /api/k8s/status mengharapkan status 200, didapat %d: %s", wK8s.Code, wK8s.Body.String())
	}
}

func httpCookieSession(val string) *http.Cookie {
	return &http.Cookie{Name: "panel_session", Value: val}
}
