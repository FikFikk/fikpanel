package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
	"fikpanel/internal/terminal"
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
	var execResp map[string]interface{}
	if err := json.Unmarshal(wExec.Body.Bytes(), &execResp); err != nil {
		t.Fatalf("Gagal unmarshal respons exec terminal: %v", err)
	}
	if execResp["cwd"] != "~" {
		t.Errorf("CWD awal sesi lokal harus '~', got %v", execResp["cwd"])
	}

	// 1b. Uji init_only pada endpoint complete agar mengembalikan cwd '~' dan versi panel
	initBody, _ := json.Marshal(map[string]interface{}{
		"command":   "",
		"cwd":       "~",
		"init_only": true,
	})
	reqInit := httptest.NewRequest("POST", "/api/terminal/complete", bytes.NewReader(initBody))
	reqInit.Header.Set("Content-Type", "application/json")
	reqInit.Header.Set("X-Panel-Request", "1")
	reqInit.AddCookie(sessionCookie)
	wInit := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wInit, reqInit)

	if wInit.Code != http.StatusOK {
		t.Fatalf("Inisialisasi sesi terminal gagal: status code %d, body %s", wInit.Code, wInit.Body.String())
	}
	var initResp map[string]interface{}
	if err := json.Unmarshal(wInit.Body.Bytes(), &initResp); err != nil {
		t.Fatalf("Gagal unmarshal respons init terminal: %v", err)
	}
	if initResp["cwd"] != "~" {
		t.Errorf("CWD hasil inisialisasi sesi harus '~', got %v", initResp["cwd"])
	}
	if initResp["panel_version"] == "" || initResp["panel_version"] == nil {
		t.Errorf("panel_version harus terisi pada respons init terminal")
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

	// 2b. Verifikasi password tersimpan terenkripsi AES-256-GCM di SQLite (tidak plaintext)
	rawServers, err := s.ListTerminalServers(context.Background())
	if err != nil || len(rawServers) != 1 {
		t.Fatalf("Gagal membaca server mentah dari database: %v", err)
	}
	if !terminal.IsEncrypted(rawServers[0].Password) || rawServers[0].Password == "ssh-pass-123" {
		t.Errorf("Password server di SQLite wajib terenkripsi enc:v1:, got %q", rawServers[0].Password)
	}

	// 2c. Uji streaming SSE endpoint (/api/terminal/exec/stream)
	streamBody, _ := json.Marshal(map[string]interface{}{
		"command": "echo sse_stream_ok",
		"cwd":     "~",
	})
	reqStream := httptest.NewRequest("POST", "/api/terminal/exec/stream", bytes.NewReader(streamBody))
	reqStream.Header.Set("Content-Type", "application/json")
	reqStream.Header.Set("X-Panel-Request", "1")
	reqStream.AddCookie(sessionCookie)
	wStream := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wStream, reqStream)

	if wStream.Code != http.StatusOK {
		t.Fatalf("Stream eksekusi terminal gagal: status code %d, body %s", wStream.Code, wStream.Body.String())
	}
	ssePayload := wStream.Body.String()
	if !strings.Contains(ssePayload, "sse_stream_ok") || !strings.Contains(ssePayload, "event: done") {
		t.Errorf("Payload SSE tidak mengandung output dan event done: %s", ssePayload)
	}

	// 2d. Uji reset host key TOFU endpoint
	reqResetHK := httptest.NewRequest("DELETE", "/api/terminal/servers/1/host-key", nil)
	reqResetHK.Header.Set("X-Panel-Request", "1")
	reqResetHK.AddCookie(sessionCookie)
	wResetHK := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wResetHK, reqResetHK)
	if wResetHK.Code != http.StatusOK {
		t.Fatalf("Reset host key SSH gagal: status code %d, body %s", wResetHK.Code, wResetHK.Body.String())
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

func TestTerminalPromptCwdFormatting(t *testing.T) {
	// Pastikan path di dalam $HOME diringkas menjadi '~' layaknya prompt SSH Linux/AWS
	cases := []struct {
		absPath string
		homeDir string
		want    string
	}{
		{"/root", "/root", "~"},
		{"/root/projects", "/root", "~/projects"},
		{"/root-backup", "/root", "/root-backup"},
		{"/var/www/html", "/root", "/var/www/html"},
		{"/home/ubuntu", "/home/ubuntu", "~"},
		{"/home/ubuntu/app", "/home/ubuntu", "~/app"},
		{"", "/root", "~"},
		{"/var/lib/fikpanel", "/", "/var/lib/fikpanel"},
	}

	for _, tc := range cases {
		if got := formatPromptCwd(tc.absPath, tc.homeDir); got != tc.want {
			t.Errorf("formatPromptCwd(%q, %q) = %q, want %q", tc.absPath, tc.homeDir, got, tc.want)
		}
	}

	raw := "hello world\n" + pwdSentinelStart + "/root/app\t/root" + pwdSentinelEnd + "\n"
	clean, cwd := extractAndStripPwdSentinel(raw, "~")
	if clean != "hello world" {
		t.Errorf("Output bersih tidak sesuai: got %q, want %q", clean, "hello world")
	}
	if cwd != "~/app" {
		t.Errorf("CWD hasil ekstraksi sentinel tidak sesuai: got %q, want %q", cwd, "~/app")
	}
}
