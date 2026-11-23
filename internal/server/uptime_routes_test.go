package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

func TestUptimeMonitorsAndPublicStatusPage(t *testing.T) {
	// Server HTTP uji lokal untuk target probe uptime
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello fikpanel healthy"))
	}))
	defer targetSrv.Close()

	tempDir, err := os.MkdirTemp("", "fikpanel_uptime_test_*")
	if err != nil {
		t.Fatalf("Gagal membuat tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	st, err := store.Open(filepath.Join(tempDir, "uptime.db"))
	if err != nil {
		t.Fatalf("Gagal membuka store: %v", err)
	}
	defer st.Close()

	if _, err := st.EnsureDevUser("root", "Rahasia123!"); err != nil {
		t.Fatalf("Gagal membuat user: %v", err)
	}

	srv, err := NewServer(st, metrics.NewHub(), services.NewManager(st.DB()), 0)
	if err != nil {
		t.Fatalf("Gagal membuat server: %v", err)
	}

	// Login
	loginBody, _ := json.Marshal(map[string]string{"username": "root", "password": "Rahasia123!"})
	reqLog := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	reqLog.Header.Set("Content-Type", "application/json")
	wLog := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wLog, reqLog)
	cookie := wLog.Result().Cookies()[0]

	// 1. Tambah monitor HTTP mengarah ke targetSrv dengan pengecekan keyword
	createBody, _ := json.Marshal(map[string]any{
		"name":            "API Lokal Uji",
		"target":          targetSrv.URL,
		"monitor_type":    "http",
		"interval_sec":    60,
		"timeout_sec":     5,
		"expected_status": 200,
		"keyword":         "fikpanel healthy",
		"notify_webhook":  false,
		"public_visible":  true,
		"enabled":         true,
	})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/uptime/monitors", bytes.NewReader(createBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("X-Panel-Request", "1")
	reqCreate.AddCookie(cookie)
	wCreate := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wCreate, reqCreate)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("Gagal menambah monitor uptime: %d %s", wCreate.Code, wCreate.Body.String())
	}
	var createResp struct {
		Monitor store.UptimeMonitor `json:"monitor"`
	}
	_ = json.Unmarshal(wCreate.Body.Bytes(), &createResp)

	// 2. Picu pemeriksaan langsung & verifikasi status = up
	reqChk := httptest.NewRequest(http.MethodPost, "/api/uptime/monitors/"+strconv.FormatInt(createResp.Monitor.ID, 10)+"/check", nil)
	reqChk.Header.Set("X-Panel-Request", "1")
	reqChk.AddCookie(cookie)
	wChk := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wChk, reqChk)
	if wChk.Code != http.StatusOK || !bytes.Contains(wChk.Body.Bytes(), []byte(`"last_status":"up"`)) {
		t.Fatalf("Pemeriksaan uptime seharusnya up, got %d %s", wChk.Code, wChk.Body.String())
	}

	// 3. Uji endpoint publik JSON /api/status/public & halaman HTML /status tanpa cookie
	wPubJSON := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wPubJSON, httptest.NewRequest(http.MethodGet, "/api/status/public", nil))
	if wPubJSON.Code != http.StatusOK || !bytes.Contains(wPubJSON.Body.Bytes(), []byte(`"overall_status":"operational"`)) {
		t.Fatalf("Public status JSON gagal: %d %s", wPubJSON.Code, wPubJSON.Body.String())
	}

	wPubHTML := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wPubHTML, httptest.NewRequest(http.MethodGet, "/status", nil))
	if wPubHTML.Code != http.StatusOK || !bytes.Contains(wPubHTML.Body.Bytes(), []byte("API Lokal Uji")) {
		t.Fatalf("Public status HTML gagal: %d", wPubHTML.Code)
	}

	// 4. Hapus monitor
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/uptime/monitors/"+strconv.FormatInt(createResp.Monitor.ID, 10), nil)
	reqDel.Header.Set("X-Panel-Request", "1")
	reqDel.AddCookie(cookie)
	wDel := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("Gagal menghapus monitor: %d %s", wDel.Code, wDel.Body.String())
	}
}
