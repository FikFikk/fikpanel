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

func TestPasskeyRoutesLifecycleAndGuards(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_passkey_test_*")
	if err != nil {
		t.Fatalf("Gagal membuat tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	st, err := store.Open(filepath.Join(tempDir, "passkey.db"))
	if err != nil {
		t.Fatalf("Gagal membuka store: %v", err)
	}
	defer st.Close()

	user, err := st.EnsureDevUser("root", "Rahasia123!")
	if err != nil {
		t.Fatalf("Gagal membuat user uji: %v", err)
	}

	srv, err := NewServer(st, metrics.NewHub(), services.NewManager(st.DB()), 0)
	if err != nil {
		t.Fatalf("Gagal membuat server: %v", err)
	}

	// Login untuk mendapatkan sesi admin
	loginBody, _ := json.Marshal(map[string]string{"username": "root", "password": "Rahasia123!"})
	reqLog := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	reqLog.Header.Set("Content-Type", "application/json")
	wLog := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wLog, reqLog)
	if wLog.Code != http.StatusOK || len(wLog.Result().Cookies()) == 0 {
		t.Fatalf("Login gagal: %d %s", wLog.Code, wLog.Body.String())
	}
	cookie := wLog.Result().Cookies()[0]

	// 1. Pendaftaran via IP mentah wajib ditolak dengan pesan jelas
	reqIP := httptest.NewRequest(http.MethodPost, "https://103.120.45.10:1405/api/auth/passkey/register/begin", nil)
	reqIP.Host = "103.120.45.10:1405"
	reqIP.Header.Set("X-Panel-Request", "1")
	reqIP.AddCookie(cookie)
	wIP := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wIP, reqIP)
	if wIP.Code != http.StatusBadRequest {
		t.Fatalf("Host IP mentah harus ditolak HTTP 400, got %d %s", wIP.Code, wIP.Body.String())
	}

	// 2. Pendaftaran via domain HTTPS valid harus mengembalikan session_id + publicKey challenge
	reqReg := httptest.NewRequest(http.MethodPost, "https://panel.example.com/api/auth/passkey/register/begin", nil)
	reqReg.Host = "panel.example.com"
	reqReg.Header.Set("Origin", "https://panel.example.com")
	reqReg.Header.Set("X-Panel-Request", "1")
	reqReg.AddCookie(cookie)
	wReg := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wReg, reqReg)
	if wReg.Code != http.StatusOK || !bytes.Contains(wReg.Body.Bytes(), []byte(`"session_id"`)) || !bytes.Contains(wReg.Body.Bytes(), []byte(`"challenge"`)) {
		t.Fatalf("Begin registration domain valid gagal: %d %s", wReg.Code, wReg.Body.String())
	}

	// 3. Sebelum ada passkey terdaftar, public settings melaporkan passkeys_available=false dan login/begin menolak
	reqPub := httptest.NewRequest(http.MethodGet, "/api/settings/public", nil)
	wPub := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wPub, reqPub)
	if !bytes.Contains(wPub.Body.Bytes(), []byte(`"passkeys_available":false`)) {
		t.Fatalf("Seharusnya passkeys_available=false, got %s", wPub.Body.String())
	}

	// 4. Simpan 1 passkey di store, verifikasi list, public flag=true, login/begin OK, dan delete
	pk, err := st.CreatePasskey(reqPub.Context(), user.ID, "HP Android", "Y3JlZC0xMjM", `{"ID":"Y3JlZC0xMjM"}`, "hybrid")
	if err != nil {
		t.Fatalf("Gagal menyimpan passkey uji: %v", err)
	}

	wPub2 := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wPub2, httptest.NewRequest(http.MethodGet, "/api/settings/public", nil))
	if !bytes.Contains(wPub2.Body.Bytes(), []byte(`"passkeys_available":true`)) {
		t.Fatalf("Seharusnya passkeys_available=true, got %s", wPub2.Body.String())
	}

	reqLoginBegin := httptest.NewRequest(http.MethodPost, "https://panel.example.com/api/auth/passkey/login/begin", nil)
	reqLoginBegin.Host = "panel.example.com"
	reqLoginBegin.Header.Set("Origin", "https://panel.example.com")
	reqLoginBegin.Header.Set("X-Panel-Request", "1")
	wLoginBegin := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wLoginBegin, reqLoginBegin)
	if wLoginBegin.Code != http.StatusOK || !bytes.Contains(wLoginBegin.Body.Bytes(), []byte(`"challenge"`)) {
		t.Fatalf("Begin discoverable login gagal: %d %s", wLoginBegin.Code, wLoginBegin.Body.String())
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/auth/passkeys", nil)
	reqList.AddCookie(cookie)
	wList := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK || !bytes.Contains(wList.Body.Bytes(), []byte(`"HP Android"`)) {
		t.Fatalf("List passkeys gagal: %d %s", wList.Code, wList.Body.String())
	}

	reqDel := httptest.NewRequest(http.MethodDelete, "/api/auth/passkeys/"+json.Number(string(rune('0'+pk.ID))).String(), nil)
	reqDel.Header.Set("X-Panel-Request", "1")
	reqDel.AddCookie(cookie)
	wDel := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("Hapus passkey gagal: %d %s", wDel.Code, wDel.Body.String())
	}
}
