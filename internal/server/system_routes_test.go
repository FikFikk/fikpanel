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

// newSystemTestServer membangun instance Server terisolasi untuk uji integrasi seluruh endpoint System Console
func newSystemTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "srv_sys_test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("gagal membuka store pengujian: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	user, err := st.EnsureDevUser("sysadmin", "SuperSecret123!")
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

// TestSystemRoutesReadAndSafetyGuards memverifikasi seluruh endpoint GET /api/system/* serta proteksi aksi destruktif
func TestSystemRoutesReadAndSafetyGuards(t *testing.T) {
	srv, token := newSystemTestServer(t)

	doReq := func(method, path, body string) *httptest.ResponseRecorder {
		var buf *bytes.Buffer
		if body != "" {
			buf = bytes.NewBufferString(body)
		} else {
			buf = bytes.NewBuffer(nil)
		}
		req := httptest.NewRequest(method, path, buf)
		req.RemoteAddr = "127.0.0.1:46000"
		if method != http.MethodGet {
			req.Header.Set("X-Panel-Request", "1")
			req.Header.Set("Content-Type", "application/json")
		}
		req.AddCookie(&http.Cookie{Name: "panel_session", Value: token})
		rec := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rec, req)
		return rec
	}

	// 1. Uji semua endpoint GET utama harus merespons HTTP 200 dengan ok=true
	readEndpoints := []string{
		"/api/system/overview",
		"/api/system/processes?limit=20&sort=cpu&filter=all",
		"/api/system/ports",
		"/api/system/memory",
		"/api/system/cleaner/scan",
		"/api/system/updates",
		"/api/system/diagnostics",
	}
	for _, ep := range readEndpoints {
		rec := doReq(http.MethodGet, ep, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s diharapkan 200, dapat %d: %s", ep, rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || payload["ok"] != true {
			t.Fatalf("respons JSON %s tidak valid: %s", ep, rec.Body.String())
		}
	}

	// 2. Uji pagar keamanan: kill PID 1 harus ditolak dengan HTTP 400
	recKill := doReq(http.MethodPost, "/api/system/processes/1/kill?signal=KILL", "")
	if recKill.Code != http.StatusBadRequest {
		t.Fatalf("kill PID 1 diharapkan 400, dapat %d: %s", recKill.Code, recKill.Body.String())
	}

	// 3. Uji pagar keamanan: disk hogs path traversal (/etc) harus ditolak HTTP 400
	recHogs := doReq(http.MethodPost, "/api/system/disk-hogs", `{"path":"/etc"}`)
	if recHogs.Code != http.StatusBadRequest {
		t.Fatalf("disk-hogs /etc diharapkan 400, dapat %d: %s", recHogs.Code, recHogs.Body.String())
	}

	// 4. Uji pagar keamanan: tuning swappiness di luar rentang harus ditolak HTTP 400
	recTune := doReq(http.MethodPost, "/api/system/memory/tuning", `{"swappiness":200,"vfs_cache_pressure":100}`)
	if recTune.Code != http.StatusBadRequest {
		t.Fatalf("memory tuning swappiness=200 diharapkan 400, dapat %d: %s", recTune.Code, recTune.Body.String())
	}

	// 5. Uji pagar keamanan: reboot dengan konfirmasi hostname salah harus ditolak HTTP 400
	recReboot := doReq(http.MethodPost, "/api/system/reboot", `{"confirm_hostname":"host-keliru-abc"}`)
	if recReboot.Code != http.StatusBadRequest {
		t.Fatalf("reboot hostname keliru diharapkan 400, dapat %d: %s", recReboot.Code, recReboot.Body.String())
	}
}
