package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

func TestTerminalRoutes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_term_routes_test_*")
	if err != nil {
		t.Fatalf("Gagal membuat direktori sementara: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_term.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Gagal membuka database store: %v", err)
	}
	defer s.Close()

	_, err = s.EnsureDevUser("root", "")
	if err != nil {
		t.Fatalf("Gagal inisialisasi dev user: %v", err)
	}

	m := metrics.NewHub()
	svc := services.NewManager(s.DB())
	srv, err := NewServer(s, m, svc, 0)
	if err != nil {
		t.Fatalf("Gagal membuat instance server: %v", err)
	}

	// Login untuk mendapatkan cookie sesi
	loginBody, _ := json.Marshal(map[string]string{
		"username": "root",
		"password": "",
	})
	reqLogin := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wLogin, reqLogin)

	var sessionCookie *http.Cookie
	for _, c := range wLogin.Result().Cookies() {
		if c.Name == "panel_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("Cookie sesi tidak ditemukan")
	}

	// 1. Uji eksekusi perintah lokal
	execBody, _ := json.Marshal(map[string]interface{}{
		"command": "echo test_terminal_exec",
	})
	reqExec := httptest.NewRequest("POST", "/api/terminal/exec", bytes.NewReader(execBody))
	reqExec.Header.Set("Content-Type", "application/json")
	reqExec.Header.Set("X-Panel-Request", "1")
	reqExec.AddCookie(sessionCookie)
	wExec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wExec, reqExec)

	if wExec.Code != http.StatusOK {
		t.Fatalf("Eksekusi perintah terminal gagal: status code %d, body %s", wExec.Code, wExec.Body.String())
	}

	// 2. Uji tambah server remote
	srvBody, _ := json.Marshal(map[string]interface{}{
		"name":      "Server Singapore",
		"host":      "10.0.0.1",
		"port":      22,
		"username":  "ubuntu",
		"auth_type": "password",
		"password":  "ssh-pass-123",
		"remarks":   "Server cadangan",
	})
	reqSrv := httptest.NewRequest("POST", "/api/terminal/servers", bytes.NewReader(srvBody))
	reqSrv.Header.Set("Content-Type", "application/json")
	reqSrv.Header.Set("X-Panel-Request", "1")
	reqSrv.AddCookie(sessionCookie)
	wSrv := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wSrv, reqSrv)

	if wSrv.Code != http.StatusCreated {
		t.Fatalf("Tambah server terminal gagal: status code %d, body %s", wSrv.Code, wSrv.Body.String())
	}

	// 3. Uji list server remote (pastikan password tidak bocor dalam list)
	reqList := httptest.NewRequest("GET", "/api/terminal/servers", nil)
	reqList.AddCookie(sessionCookie)
	wList := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("List server terminal gagal: status code %d", wList.Code)
	}
	var serverList []map[string]interface{}
	if err := json.Unmarshal(wList.Body.Bytes(), &serverList); err != nil {
		t.Fatalf("Gagal unmarshal daftar server: %v", err)
	}
	if len(serverList) != 1 {
		t.Fatalf("Jumlah server salah: got %d, want 1", len(serverList))
	}
	if _, exists := serverList[0]["password"]; exists {
		t.Errorf("Password tidak boleh terekspos di API list server")
	}

	// 4. Uji list command shortcut (harus ada default bawaan)
	reqCmdList := httptest.NewRequest("GET", "/api/terminal/commands", nil)
	reqCmdList.AddCookie(sessionCookie)
	wCmdList := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wCmdList, reqCmdList)

	if wCmdList.Code != http.StatusOK {
		t.Fatalf("List pintasan perintah gagal: status code %d", wCmdList.Code)
	}
	var cmdList []map[string]interface{}
	if err := json.Unmarshal(wCmdList.Body.Bytes(), &cmdList); err != nil {
		t.Fatalf("Gagal unmarshal daftar perintah: %v", err)
	}
	if len(cmdList) == 0 {
		t.Fatalf("Pintasan perintah default tidak boleh kosong")
	}

	// 5. Uji tambah command shortcut baru
	addCmdBody, _ := json.Marshal(map[string]string{
		"name":    "Check Port 80",
		"content": "curl -I http://localhost",
	})
	reqAddCmd := httptest.NewRequest("POST", "/api/terminal/commands", bytes.NewReader(addCmdBody))
	reqAddCmd.Header.Set("Content-Type", "application/json")
	reqAddCmd.Header.Set("X-Panel-Request", "1")
	reqAddCmd.AddCookie(sessionCookie)
	wAddCmd := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wAddCmd, reqAddCmd)

	if wAddCmd.Code != http.StatusCreated {
		t.Fatalf("Tambah perintah terminal gagal: status code %d", wAddCmd.Code)
	}
}
