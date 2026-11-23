package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fikpanel/internal/auth"
	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

func TestSettingsSuiteEndToEnd(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_settings_test_*")
	if err != nil {
		t.Fatalf("Gagal membuat tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "settings.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Gagal membuka store: %v", err)
	}
	defer st.Close()

	if _, err := st.EnsureDevUser("root", "Rahasia123!"); err != nil {
		t.Fatalf("Gagal seed user: %v", err)
	}

	srv, err := NewServer(st, metrics.NewHub(), services.NewManager(st.DB()), 0)
	if err != nil {
		t.Fatalf("Gagal membuat server: %v", err)
	}

	doLogin := func(totp string) (*httptest.ResponseRecorder, *http.Cookie) {
		body, _ := json.Marshal(map[string]string{
			"username":  "root",
			"password":  "Rahasia123!",
			"totp_code": totp,
		})
		req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(w, req)
		var ck *http.Cookie
		for _, c := range w.Result().Cookies() {
			if c.Name == "panel_session" {
				ck = c
			}
		}
		return w, ck
	}

	wLogin, cookie := doLogin("")
	if wLogin.Code != http.StatusOK || cookie == nil {
		t.Fatalf("Login awal gagal: code=%d body=%s", wLogin.Code, wLogin.Body.String())
	}

	// 1. Update preferensi tampilan, branding, dan bahasa (A + G)
	prefsPayload, _ := json.Marshal(map[string]interface{}{
		"theme_mode":         "dark",
		"accent_preset":      "ocean",
		"accent_custom":      "#0ea5e9",
		"surface_style":      "midnight",
		"density":            "compact",
		"radius_style":       "rounded",
		"brand_name":         "Acme Cloud",
		"env_badge":          "production",
		"language":           "en",
		"session_hours":      48,
		"ip_allowlist":       []string{},
		"alert_cpu_pct":      90,
		"alert_ram_pct":      85,
		"alert_disk_pct":     92,
		"alert_ssl_days":     21,
		"alert_cooldown_min": 10,
	})
	reqPref := httptest.NewRequest("PUT", "/api/settings/preferences", bytes.NewReader(prefsPayload))
	reqPref.Header.Set("Content-Type", "application/json")
	reqPref.Header.Set("X-Panel-Request", "1")
	reqPref.AddCookie(cookie)
	wPref := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wPref, reqPref)
	if wPref.Code != http.StatusOK {
		t.Fatalf("Update preferensi gagal: %d %s", wPref.Code, wPref.Body.String())
	}

	// Cek endpoint publik mengembalikan branding & bahasa tanpa cookie
	wPub := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wPub, httptest.NewRequest("GET", "/api/settings/public", nil))
	if wPub.Code != http.StatusOK || !bytes.Contains(wPub.Body.Bytes(), []byte(`"Acme Cloud"`)) {
		t.Fatalf("Endpoint publik tidak menyertakan brand_name baru: %d %s", wPub.Code, wPub.Body.String())
	}

	// 2. Uji proteksi anti-lockout IP Allowlist (C)
	lockoutPayload, _ := json.Marshal(map[string]interface{}{
		"ip_allowlist": []string{"203.0.113.99"},
	})
	reqLock := httptest.NewRequest("PUT", "/api/settings/preferences", bytes.NewReader(lockoutPayload))
	reqLock.RemoteAddr = "198.51.100.10:54321"
	reqLock.Header.Set("Content-Type", "application/json")
	reqLock.Header.Set("X-Panel-Request", "1")
	reqLock.AddCookie(cookie)
	wLock := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wLock, reqLock)
	if wLock.Code != http.StatusBadRequest {
		t.Fatalf("Proteksi anti-lockout IP Allowlist seharusnya menolak HTTP 400, got %d", wLock.Code)
	}

	// 3. Uji penuh siklus 2FA TOTP: setup -> enable -> login butuh 2FA -> login valid -> disable (B)
	reqSetup := httptest.NewRequest("POST", "/api/auth/2fa/setup", nil)
	reqSetup.Header.Set("X-Panel-Request", "1")
	reqSetup.AddCookie(cookie)
	wSetup := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wSetup, reqSetup)
	if wSetup.Code != http.StatusOK {
		t.Fatalf("Setup 2FA gagal: %d %s", wSetup.Code, wSetup.Body.String())
	}
	var setupResp struct {
		Secret string `json:"secret"`
	}
	_ = json.Unmarshal(wSetup.Body.Bytes(), &setupResp)
	codeNow, err := auth.ComputeTOTPCode(setupResp.Secret, time.Now())
	if err != nil {
		t.Fatalf("ComputeTOTPCode gagal: %v", err)
	}

	enableBody, _ := json.Marshal(map[string]string{"code": codeNow})
	reqEnable := httptest.NewRequest("POST", "/api/auth/2fa/enable", bytes.NewReader(enableBody))
	reqEnable.Header.Set("Content-Type", "application/json")
	reqEnable.Header.Set("X-Panel-Request", "1")
	reqEnable.AddCookie(cookie)
	wEnable := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wEnable, reqEnable)
	if wEnable.Code != http.StatusOK {
		t.Fatalf("Enable 2FA gagal: %d %s", wEnable.Code, wEnable.Body.String())
	}
	var enableResp struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	_ = json.Unmarshal(wEnable.Body.Bytes(), &enableResp)
	if len(enableResp.RecoveryCodes) != 8 {
		t.Fatalf("Harus menghasilkan 8 kode pemulihan, got %d", len(enableResp.RecoveryCodes))
	}

	// Login tanpa kode 2FA harus membalas require_totp: true
	wNeed2FA, ckNeed := doLogin("")
	if wNeed2FA.Code != http.StatusOK || ckNeed != nil || !bytes.Contains(wNeed2FA.Body.Bytes(), []byte(`"require_totp":true`)) {
		t.Fatalf("Login tanpa OTP saat 2FA aktif harus mengembalikan require_totp:true, got %d %s", wNeed2FA.Code, wNeed2FA.Body.String())
	}

	// Login dengan kode pemulihan 1x pakai harus sukses
	wRec, ckRec := doLogin(enableResp.RecoveryCodes[0])
	if wRec.Code != http.StatusOK || ckRec == nil {
		t.Fatalf("Login dengan kode pemulihan gagal: %d %s", wRec.Code, wRec.Body.String())
	}

	// Regenerasi kode pemulihan 2FA harus menerbitkan 8 kode baru setelah verifikasi kata sandi
	reqRegen := httptest.NewRequest("POST", "/api/auth/2fa/recovery-regenerate", bytes.NewReader([]byte(`{"password":"Rahasia123!"}`)))
	reqRegen.Header.Set("Content-Type", "application/json")
	reqRegen.Header.Set("X-Panel-Request", "1")
	reqRegen.AddCookie(cookie)
	wRegen := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wRegen, reqRegen)
	if wRegen.Code != http.StatusOK || !bytes.Contains(wRegen.Body.Bytes(), []byte(`"recovery_remaining":8`)) {
		t.Fatalf("Regenerasi kode pemulihan gagal: %d %s", wRegen.Code, wRegen.Body.String())
	}

	// Uji daftar akun panel & ganti kata sandi akun lain oleh administrator
	otherUser, err := st.EnsureDevUser("admin", "AdminAwal12345!")
	if err != nil {
		t.Fatalf("Gagal membuat akun kedua: %v", err)
	}
	reqUsers := httptest.NewRequest("GET", "/api/auth/users", nil)
	reqUsers.AddCookie(cookie)
	wUsers := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wUsers, reqUsers)
	if wUsers.Code != http.StatusOK || !bytes.Contains(wUsers.Body.Bytes(), []byte(`"admin"`)) {
		t.Fatalf("Daftar akun panel harus berisi admin: %d %s", wUsers.Code, wUsers.Body.String())
	}

	pwOtherBody, _ := json.Marshal(map[string]any{
		"old_password":   "Rahasia123!",
		"new_password":   "AdminBaruSuperAman99!",
		"target_user_id": otherUser.ID,
	})
	reqPwOther := httptest.NewRequest("POST", "/api/auth/password", bytes.NewReader(pwOtherBody))
	reqPwOther.Header.Set("Content-Type", "application/json")
	reqPwOther.Header.Set("X-Panel-Request", "1")
	reqPwOther.AddCookie(cookie)
	wPwOther := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wPwOther, reqPwOther)
	if wPwOther.Code != http.StatusOK || !bytes.Contains(wPwOther.Body.Bytes(), []byte(`"target_username":"admin"`)) {
		t.Fatalf("Reset sandi akun lain oleh admin gagal: %d %s", wPwOther.Code, wPwOther.Body.String())
	}

	// Daftar sesi aktif harus berisi minimal 2 sesi & revoke-others menyisakan 1 sesi
	reqSess := httptest.NewRequest("GET", "/api/auth/sessions", nil)
	reqSess.AddCookie(cookie)
	wSess := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wSess, reqSess)
	var sessResp struct {
		Sessions []store.ActiveSessionView `json:"sessions"`
	}
	_ = json.Unmarshal(wSess.Body.Bytes(), &sessResp)
	if len(sessResp.Sessions) < 2 {
		t.Fatalf("Sesi aktif minimal 2, got %d", len(sessResp.Sessions))
	}

	reqRev := httptest.NewRequest("POST", "/api/auth/sessions/revoke-others", nil)
	reqRev.Header.Set("X-Panel-Request", "1")
	reqRev.AddCookie(cookie)
	wRev := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wRev, reqRev)
	if wRev.Code != http.StatusOK {
		t.Fatalf("Revoke other sessions gagal: %d %s", wRev.Code, wRev.Body.String())
	}

	// 4. Uji snapshot SQLite VACUUM INTO & Export/Import konfigurasi (F)
	reqBak := httptest.NewRequest("GET", "/api/settings/backup/db", nil)
	reqBak.AddCookie(cookie)
	wBak := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wBak, reqBak)
	if wBak.Code != http.StatusOK || wBak.Body.Len() < 1024 {
		t.Fatalf("Unduhan snapshot SQLite gagal: code=%d size=%d", wBak.Code, wBak.Body.Len())
	}

	reqExp := httptest.NewRequest("GET", "/api/settings/config/export", nil)
	reqExp.AddCookie(cookie)
	wExp := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wExp, reqExp)
	if wExp.Code != http.StatusOK || !bytes.Contains(wExp.Body.Bytes(), []byte(`"Acme Cloud"`)) {
		t.Fatalf("Export konfigurasi JSON gagal: %d", wExp.Code)
	}

	reqImp := httptest.NewRequest("POST", "/api/settings/config/import", bytes.NewReader(wExp.Body.Bytes()))
	reqImp.Header.Set("Content-Type", "application/json")
	reqImp.Header.Set("X-Panel-Request", "1")
	reqImp.AddCookie(cookie)
	wImp := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wImp, reqImp)
	if wImp.Code != http.StatusOK {
		t.Fatalf("Import konfigurasi JSON gagal: %d %s", wImp.Code, wImp.Body.String())
	}
}
