package server

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/auth"
	"fikpanel/internal/store"
	"fikpanel/internal/updater"
)

const (
	panelPrefsSettingKey = "panel_preferences_v1"
	totpRecoveryPrefix   = "totp_recovery_hashes_"
)

var (
	hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	hostnameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	timezoneRe = regexp.MustCompile(`^[A-Za-z0-9_+-]+(/[A-Za-z0-9_+-]+){0,2}$`)
)

// PanelPreferences menyimpan seluruh preferensi tampilan, branding, bahasa, akses, dan ambang alert panel
type PanelPreferences struct {
	ThemeMode        string   `json:"theme_mode"`         // dark | light | auto
	AccentPreset     string   `json:"accent_preset"`      // lava | ocean | emerald | violet | crimson | amber | slate | custom
	AccentCustom     string   `json:"accent_custom"`      // #RRGGBB
	SurfaceStyle     string   `json:"surface_style"`      // obsidian | midnight | carbon
	Density          string   `json:"density"`            // comfortable | compact
	RadiusStyle      string   `json:"radius_style"`       // sharp | default | rounded
	BrandName        string   `json:"brand_name"`         // nama white-label panel
	EnvBadge         string   `json:"env_badge"`          // '' | production | staging | development
	Language         string   `json:"language"`           // id | en
	SessionHours     int      `json:"session_hours"`      // durasi sesi login (jam)
	IPAllowlist      []string `json:"ip_allowlist"`       // daftar IP/CIDR yang diizinkan (kosong = semua)
	AlertCPUPct      int      `json:"alert_cpu_pct"`      // ambang alert CPU (%)
	AlertRAMPct      int      `json:"alert_ram_pct"`      // ambang alert RAM (%)
	AlertDiskPct     int      `json:"alert_disk_pct"`     // ambang alert Disk (%)
	AlertSSLDays     int      `json:"alert_ssl_days"`     // ambang peringatan kedaluwarsa SSL (hari)
	AlertCooldownMin int      `json:"alert_cooldown_min"` // jeda minimal antar alert (menit)
}

// DefaultPanelPreferences mengembalikan nilai bawaan standar FikPanel
func DefaultPanelPreferences() PanelPreferences {
	return PanelPreferences{
		ThemeMode:        "dark",
		AccentPreset:     "lava",
		AccentCustom:     "#ff5a2b",
		SurfaceStyle:     "obsidian",
		Density:          "comfortable",
		RadiusStyle:      "default",
		BrandName:        "fikpanel",
		EnvBadge:         "",
		Language:         "id",
		SessionHours:     24,
		IPAllowlist:      []string{},
		AlertCPUPct:      85,
		AlertRAMPct:      85,
		AlertDiskPct:     90,
		AlertSSLDays:     14,
		AlertCooldownMin: 15,
	}
}

func sanitizePanelPreferences(in PanelPreferences) (PanelPreferences, error) {
	def := DefaultPanelPreferences()

	switch in.ThemeMode {
	case "dark", "light", "auto":
	default:
		in.ThemeMode = def.ThemeMode
	}

	switch in.AccentPreset {
	case "lava", "ocean", "emerald", "violet", "crimson", "amber", "slate", "custom":
	default:
		in.AccentPreset = def.AccentPreset
	}

	in.AccentCustom = strings.TrimSpace(in.AccentCustom)
	if !hexColorRe.MatchString(in.AccentCustom) {
		in.AccentCustom = def.AccentCustom
	}

	switch in.SurfaceStyle {
	case "obsidian", "midnight", "carbon":
	default:
		in.SurfaceStyle = def.SurfaceStyle
	}

	switch in.Density {
	case "comfortable", "compact":
	default:
		in.Density = def.Density
	}

	switch in.RadiusStyle {
	case "sharp", "default", "rounded":
	default:
		in.RadiusStyle = def.RadiusStyle
	}

	in.BrandName = strings.TrimSpace(in.BrandName)
	if in.BrandName == "" {
		in.BrandName = def.BrandName
	}
	if len(in.BrandName) > 40 {
		in.BrandName = in.BrandName[:40]
	}

	switch in.EnvBadge {
	case "", "production", "staging", "development":
	default:
		in.EnvBadge = ""
	}

	switch in.Language {
	case "id", "en":
	default:
		in.Language = def.Language
	}

	if in.SessionHours < 1 {
		in.SessionHours = def.SessionHours
	}
	if in.SessionHours > 720 {
		in.SessionHours = 720
	}

	// Validasi setiap entri IP atau CIDR pada allowlist
	cleanedAllow := make([]string, 0, len(in.IPAllowlist))
	seen := make(map[string]bool)
	for _, raw := range in.IPAllowlist {
		entry := strings.TrimSpace(raw)
		if entry == "" || seen[entry] {
			continue
		}
		if strings.Contains(entry, "/") {
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return in, fmt.Errorf("format CIDR tidak valid pada IP Allowlist: %s", entry)
			}
		} else if net.ParseIP(entry) == nil {
			return in, fmt.Errorf("format IP tidak valid pada IP Allowlist: %s", entry)
		}
		seen[entry] = true
		cleanedAllow = append(cleanedAllow, entry)
	}
	in.IPAllowlist = cleanedAllow

	in.AlertCPUPct = clampInt(in.AlertCPUPct, 50, 99, def.AlertCPUPct)
	in.AlertRAMPct = clampInt(in.AlertRAMPct, 50, 99, def.AlertRAMPct)
	in.AlertDiskPct = clampInt(in.AlertDiskPct, 50, 99, def.AlertDiskPct)
	in.AlertSSLDays = clampInt(in.AlertSSLDays, 1, 90, def.AlertSSLDays)
	in.AlertCooldownMin = clampInt(in.AlertCooldownMin, 1, 1440, def.AlertCooldownMin)

	return in, nil
}

// ipMatchesAllowlist memeriksa apakah alamat IP cocok dengan salah satu entri IP/CIDR (loopback selalu diizinkan)
func ipMatchesAllowlist(ipStr string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	// Loopback selalu diizinkan agar health check updater & reverse proxy lokal tidak pernah terblokir
	if ip.IsLoopback() {
		return true
	}
	for _, entry := range allowlist {
		if strings.Contains(entry, "/") {
			if _, cidr, err := net.ParseCIDR(entry); err == nil && cidr.Contains(ip) {
				return true
			}
		} else if allowed := net.ParseIP(entry); allowed != nil && allowed.Equal(ip) {
			return true
		}
	}
	return false
}

// loadPanelPreferences membaca preferensi panel dari cache memori atau database SQLite
func (s *Server) loadPanelPreferences(ctx context.Context) PanelPreferences {
	s.prefsMu.RLock()
	if s.prefsLoaded {
		p := s.prefsCache
		s.prefsMu.RUnlock()
		return p
	}
	s.prefsMu.RUnlock()

	prefs := DefaultPanelPreferences()
	raw, err := s.store.GetSetting(ctx, panelPrefsSettingKey)
	if err == nil && strings.TrimSpace(raw) != "" {
		var parsed PanelPreferences
		if json.Unmarshal([]byte(raw), &parsed) == nil {
			if clean, valErr := sanitizePanelPreferences(parsed); valErr == nil {
				prefs = clean
			}
		}
	}

	s.prefsMu.Lock()
	s.prefsCache = prefs
	s.prefsLoaded = true
	s.prefsMu.Unlock()
	return prefs
}

func (s *Server) savePanelPreferences(ctx context.Context, prefs PanelPreferences) error {
	b, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, panelPrefsSettingKey, string(b)); err != nil {
		return err
	}
	s.prefsMu.Lock()
	s.prefsCache = prefs
	s.prefsLoaded = true
	s.prefsMu.Unlock()
	return nil
}

// handleGetPublicSettings menyediakan preferensi tampilan, branding, & bahasa publik agar layar Login langsung selaras
func (s *Server) handleGetPublicSettings(w http.ResponseWriter, r *http.Request) {
	p := s.loadPanelPreferences(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                 true,
		"theme_mode":         p.ThemeMode,
		"accent_preset":      p.AccentPreset,
		"accent_custom":      p.AccentCustom,
		"surface_style":      p.SurfaceStyle,
		"density":            p.Density,
		"radius_style":       p.RadiusStyle,
		"brand_name":         p.BrandName,
		"env_badge":          p.EnvBadge,
		"language":           p.Language,
		"version":            s.Version(),
		"passkeys_available": s.store.CountAllPasskeys(r.Context()) > 0,
	})
}

// handleGetPanelPreferences mengembalikan seluruh konfigurasi preferensi panel untuk halaman Settings
func (s *Server) handleGetPanelPreferences(w http.ResponseWriter, r *http.Request) {
	p := s.loadPanelPreferences(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"preferences": p,
		"client_ip":   getIP(r),
	})
}

// handleUpdatePanelPreferences memvalidasi dan menyimpan preferensi panel beserta proteksi anti-lockout IP Allowlist
func (s *Server) handleUpdatePanelPreferences(w http.ResponseWriter, r *http.Request) {
	var req PanelPreferences
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}

	clean, err := sanitizePanelPreferences(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Proteksi anti-lockout: tolak bila daftar IP Allowlist akan memblokir IP administrator saat ini
	clientIP := getIP(r)
	if len(clean.IPAllowlist) > 0 && !ipMatchesAllowlist(clientIP, clean.IPAllowlist) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("IP Anda saat ini (%s) tidak termasuk dalam IP Allowlist. Tambahkan IP Anda terlebih dahulu agar tidak terkunci dari panel.", clientIP),
		})
		return
	}

	if err := s.savePanelPreferences(r.Context(), clean); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menyimpan preferensi panel"})
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	actorTarget := "panel"
	if user != nil {
		uid = &user.ID
		actorTarget = user.Username
	}
	_ = s.store.RecordAudit(r.Context(), "user", "settings.preferences_update", actorTarget, true,
		fmt.Sprintf("Tema=%s/%s Bahasa=%s Allowlist=%d", clean.ThemeMode, clean.AccentPreset, clean.Language, len(clean.IPAllowlist)), uid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"preferences": clean,
		"message":     "Pengaturan panel berhasil disimpan",
	})
}

// ═══ B. 2FA TOTP & Manajemen Sesi Aktif ═══

type pendingTOTPSetup struct {
	secret    string
	createdAt time.Time
}

var (
	pendingTOTPMu  sync.Mutex
	pendingTOTPMAP = make(map[int64]pendingTOTPSetup)
)

func (s *Server) loadRecoveryHashes(ctx context.Context, userID int64) []string {
	raw, err := s.store.GetSetting(ctx, fmt.Sprintf("%s%d", totpRecoveryPrefix, userID))
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	var list []string
	_ = json.Unmarshal([]byte(raw), &list)
	return list
}

func (s *Server) saveRecoveryHashes(ctx context.Context, userID int64, hashes []string) error {
	b, _ := json.Marshal(hashes)
	return s.store.SetSetting(ctx, fmt.Sprintf("%s%d", totpRecoveryPrefix, userID), string(b))
}

// verifyUserTOTPOrRecovery memverifikasi kode 6 digit TOTP atau mengonsumsi 1 kode pemulihan milik pengguna
func (s *Server) verifyUserTOTPOrRecovery(ctx context.Context, user *store.User, code string) bool {
	if user == nil || len(user.TotpSecret) == 0 {
		return true
	}
	secretPlain, err := s.termSecrets.Decrypt(ctx, string(user.TotpSecret))
	if err != nil || secretPlain == "" {
		return false
	}
	if auth.VerifyTOTPCode(secretPlain, code, time.Now()) {
		return true
	}
	// Periksa apakah input merupakan salah satu kode pemulihan sekali pakai (XXXX-XXXX)
	hashes := s.loadRecoveryHashes(ctx, user.ID)
	if len(hashes) == 0 {
		return false
	}
	inputHash := auth.HashRecoveryCode(code)
	for i, h := range hashes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(inputHash)) == 1 {
			remaining := append(hashes[:i], hashes[i+1:]...)
			_ = s.saveRecoveryHashes(ctx, user.ID, remaining)
			return true
		}
	}
	return false
}

// handleGet2FAStatus mengembalikan status 2FA aktif dan sisa kode pemulihan pengguna
func (s *Server) handleGet2FAStatus(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	dbUser, err := s.store.GetUserByID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "pengguna tidak ditemukan"})
		return
	}
	enabled := len(dbUser.TotpSecret) > 0
	rem := 0
	if enabled {
		rem = len(s.loadRecoveryHashes(r.Context(), user.ID))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                 true,
		"enabled":            enabled,
		"recovery_remaining": rem,
	})
}

// handleSetup2FA menghasilkan secret TOTP baru beserta URI otpauth:// untuk dipindai di aplikasi Authenticator
func (s *Server) handleSetup2FA(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membuat secret 2FA"})
		return
	}

	pendingTOTPMu.Lock()
	pendingTOTPMAP[user.ID] = pendingTOTPSetup{secret: secret, createdAt: time.Now()}
	pendingTOTPMu.Unlock()

	prefs := s.loadPanelPreferences(r.Context())
	issuer := prefs.BrandName
	if issuer == "" {
		issuer = "FikPanel"
	}
	uri := auth.BuildOtpAuthURI(issuer, user.Username, secret)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"secret":      secret,
		"otpauth_uri": uri,
		"issuer":      issuer,
		"account":     user.Username,
	})
}

// handleEnable2FA memverifikasi kode 6 digit pertama, mengaktifkan 2FA terenkripsi AES-256-GCM, dan menerbitkan recovery codes
func (s *Server) handleEnable2FA(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kode OTP 6 digit wajib diisi"})
		return
	}

	pendingTOTPMu.Lock()
	pending, exists := pendingTOTPMAP[user.ID]
	pendingTOTPMu.Unlock()

	if !exists || time.Since(pending.createdAt) > 15*time.Minute {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sesi setup 2FA kedaluwarsa, klik Mulai Setup ulang"})
		return
	}
	if !auth.VerifyTOTPCode(pending.secret, req.Code, time.Now()) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kode OTP 6 digit tidak cocok; pastikan jam perangkat sinkron"})
		return
	}

	encSecret, err := s.termSecrets.Encrypt(r.Context(), pending.secret)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mengenkripsi secret 2FA"})
		return
	}
	if err := s.store.SetUserTOTPSecret(r.Context(), user.ID, []byte(encSecret)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mengaktifkan 2FA"})
		return
	}

	plainCodes, hashes, err := auth.GenerateRecoveryCodes(8)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membuat kode pemulihan"})
		return
	}
	_ = s.saveRecoveryHashes(r.Context(), user.ID, hashes)

	pendingTOTPMu.Lock()
	delete(pendingTOTPMAP, user.ID)
	pendingTOTPMu.Unlock()

	_ = s.store.RecordAudit(r.Context(), "user", "auth.2fa_enable", user.Username, true, "Mengaktifkan autentikasi 2FA TOTP", &user.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":             true,
		"enabled":        true,
		"recovery_codes": plainCodes,
		"message":        "2FA berhasil diaktifkan. Simpan 8 kode pemulihan di tempat aman.",
	})
}

// handleDisable2FA menonaktifkan 2FA setelah memverifikasi kata sandi saat ini atau kode OTP valid
func (s *Server) handleDisable2FA(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload tidak valid"})
		return
	}

	dbUser, err := s.store.GetUserByID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "pengguna tidak ditemukan"})
		return
	}

	verified := false
	if req.Password != "" {
		if ok, _ := auth.VerifyPassword(req.Password, dbUser.PasswordHash); ok {
			verified = true
		}
	}
	if !verified && req.Code != "" {
		if s.verifyUserTOTPOrRecovery(r.Context(), dbUser, req.Code) {
			verified = true
		}
	}
	if !verified {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kata sandi atau kode 2FA tidak valid"})
		return
	}

	if err := s.store.SetUserTOTPSecret(r.Context(), user.ID, nil); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menonaktifkan 2FA"})
		return
	}
	_ = s.saveRecoveryHashes(r.Context(), user.ID, []string{})
	_ = s.store.RecordAudit(r.Context(), "user", "auth.2fa_disable", user.Username, true, "Menonaktifkan autentikasi 2FA TOTP", &user.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"enabled": false,
		"message": "2FA berhasil dinonaktifkan",
	})
}

// handleRegenerate2FARecovery menerbitkan 8 kode pemulihan baru (menggantikan kode lama) setelah verifikasi sandi/OTP
func (s *Server) handleRegenerate2FARecovery(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload tidak valid"})
		return
	}

	dbUser, err := s.store.GetUserByID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "pengguna tidak ditemukan"})
		return
	}
	if len(dbUser.TotpSecret) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "2FA belum diaktifkan pada akun ini"})
		return
	}

	verified := false
	if req.Password != "" {
		if ok, _ := auth.VerifyPassword(req.Password, dbUser.PasswordHash); ok {
			verified = true
		}
	}
	if !verified && req.Code != "" {
		if s.verifyUserTOTPOrRecovery(r.Context(), dbUser, req.Code) {
			verified = true
		}
	}
	if !verified {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kata sandi atau kode 2FA tidak valid"})
		return
	}

	plainCodes, hashes, err := auth.GenerateRecoveryCodes(8)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membuat kode pemulihan baru"})
		return
	}
	if err := s.saveRecoveryHashes(r.Context(), user.ID, hashes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menyimpan kode pemulihan baru"})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "auth.2fa_recovery_regen", user.Username, true, "Membuat ulang 8 kode pemulihan 2FA", &user.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                 true,
		"recovery_codes":     plainCodes,
		"recovery_remaining": len(plainCodes),
		"message":            "8 kode pemulihan baru berhasil dibuat; kode lama tidak lagi berlaku.",
	})
}

// handleListPanelUsers mengembalikan daftar akun panel (admin melihat semua akun, non-admin hanya melihat akunnya sendiri)
func (s *Server) handleListPanelUsers(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	type panelUserView struct {
		ID          int64  `json:"id"`
		Username    string `json:"username"`
		Role        string `json:"role"`
		TOTPEnabled bool   `json:"totp_enabled"`
		LastLoginAt int64  `json:"last_login_at"`
		IsCurrent   bool   `json:"is_current"`
	}

	var rows []store.User
	if user.Role == "admin" {
		list, err := s.store.ListUsers(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membaca daftar akun"})
			return
		}
		rows = list
	} else {
		dbUser, err := s.store.GetUserByID(r.Context(), user.ID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "pengguna tidak ditemukan"})
			return
		}
		rows = []store.User{*dbUser}
	}

	out := make([]panelUserView, 0, len(rows))
	for _, u := range rows {
		var lastLogin int64
		if u.LastLoginAt.Valid {
			lastLogin = u.LastLoginAt.Int64
		}
		out = append(out, panelUserView{
			ID:          u.ID,
			Username:    u.Username,
			Role:        u.Role,
			TOTPEnabled: len(u.TotpSecret) > 0,
			LastLoginAt: lastLogin,
			IsCurrent:   u.ID == user.ID,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":           true,
		"current_user": user.Username,
		"current_id":   user.ID,
		"users":        out,
	})
}

// handleListActiveSessions mengembalikan daftar seluruh perangkat yang sedang masuk pada akun admin aktif
func (s *Server) handleListActiveSessions(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	curToken := ""
	if c, err := r.Cookie("panel_session"); err == nil {
		curToken = c.Value
	}
	sessions, err := s.store.ListUserSessions(r.Context(), user.ID, curToken)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"sessions": sessions,
	})
}

// handleRevokeSession mencabut satu sesi perangkat aktif berdasarkan prefiks ID sesi
func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	deleted, err := s.store.RevokeSessionByPrefix(r.Context(), user.ID, id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "sesi tidak ditemukan"})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "auth.session_revoke", id, true, "Mencabut sesi perangkat", &user.ID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Sesi berhasil dicabut"})
}

// handleRevokeOtherSessions mengeluarkan seluruh perangkat lain kecuali sesi browser saat ini
func (s *Server) handleRevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	curToken := ""
	if c, err := r.Cookie("panel_session"); err == nil {
		curToken = c.Value
	}
	count, err := s.store.RevokeOtherSessions(r.Context(), user.ID, curToken)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mencabut sesi lain"})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "auth.session_revoke_others", user.Username, true,
		fmt.Sprintf("Mencabut %d sesi perangkat lain", count), &user.ID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"revoked": count,
		"message": fmt.Sprintf("%d sesi perangkat lain telah dikeluarkan", count),
	})
}

// ═══ C. Jaringan Panel & Sertifikat HTTPS/TLS ═══

// handleGetPanelTLS mengembalikan metadata sertifikat HTTPS panel aktif (atau status HTTP)
func (s *Server) handleGetPanelTLS(w http.ResponseWriter, r *http.Request) {
	if s.certs == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":        true,
			"tls":       TLSStatusInfo{Enabled: false, DNSNames: []string{}},
			"client_ip": getIP(r),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"tls":       s.certs.StatusInfo(),
		"client_ip": getIP(r),
	})
}

// handleInstallCustomPanelTLS memvalidasi & memasang sertifikat SSL kustom panel dengan hot-reload tanpa restart
func (s *Server) handleInstallCustomPanelTLS(w http.ResponseWriter, r *http.Request) {
	if s.certs == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "TLS panel tidak aktif (panel dijalankan dengan --tls=off)"})
		return
	}
	var req struct {
		CertPEM string `json:"cert_pem"`
		KeyPEM  string `json:"key_pem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	certPEM := strings.TrimSpace(req.CertPEM)
	keyPEM := strings.TrimSpace(req.KeyPEM)
	if certPEM == "" || keyPEM == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sertifikat PEM dan kunci privat PEM wajib diisi"})
		return
	}
	if err := s.certs.InstallCustomCert([]byte(certPEM+"\n"), []byte(keyPEM+"\n")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "settings.tls_custom", "panel.crt", true, "Memasang sertifikat TLS kustom panel (hot-reload)", uid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"tls":     s.certs.StatusInfo(),
		"message": "Sertifikat HTTPS panel berhasil dipasang dan langsung aktif tanpa restart",
	})
}

// handleResetSelfSignedTLS membuat ulang sertifikat self-signed bawaan panel secara hot-reload
func (s *Server) handleResetSelfSignedTLS(w http.ResponseWriter, r *http.Request) {
	if s.certs == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "TLS panel tidak aktif"})
		return
	}
	if err := s.certs.ResetToSelfSigned(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("gagal membuat sertifikat self-signed: %v", err)})
		return
	}
	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "settings.tls_self_signed", "panel.crt", true, "Membuat ulang sertifikat self-signed panel", uid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"tls":     s.certs.StatusInfo(),
		"message": "Sertifikat self-signed panel berhasil dibuat ulang dan aktif",
	})
}

// ═══ D. Informasi Sistem Nyata, Hostname, Timezone & Update Center ═══

// detectHostOSPretty membaca nama distro OS asli dari /etc/os-release di Linux
func detectHostOSPretty() string {
	if runtime.GOOS == "windows" {
		return "Windows (" + runtime.GOARCH + ")"
	}
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				val := strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"'\r ")
				if val != "" {
					return val + " (" + runtime.GOARCH + ")"
				}
			}
		}
	}
	return runtime.GOOS + " (" + runtime.GOARCH + ")"
}

// detectHostKernel membaca versi kernel Linux nyata dari /proc/sys/kernel/osrelease
func detectHostKernel() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
			if k := strings.TrimSpace(string(data)); k != "" {
				return k
			}
		}
	}
	return runtime.GOOS + "/" + runtime.GOARCH
}

// detectHostUptimeSec membaca durasi nyala host Linux dari /proc/uptime dalam detik
func detectHostUptimeSec() int64 {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/uptime"); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) > 0 {
				if f, err := strconv.ParseFloat(fields[0], 64); err == nil {
					return int64(f)
				}
			}
		}
	}
	return 0
}

func detectHostTimezone() string {
	if runtime.GOOS == "linux" {
		if out, err := exec.Command("timedatectl", "show", "-p", "Timezone", "--value").Output(); err == nil {
			if tz := strings.TrimSpace(string(out)); tz != "" {
				return tz
			}
		}
		if data, err := os.ReadFile("/etc/timezone"); err == nil {
			if tz := strings.TrimSpace(string(data)); tz != "" {
				return tz
			}
		}
	}
	name, _ := time.Now().Zone()
	return name
}

var commonTimezones = []string{
	"UTC",
	"Asia/Jakarta",
	"Asia/Makassar",
	"Asia/Jayapura",
	"Asia/Singapore",
	"Asia/Kuala_Lumpur",
	"Asia/Bangkok",
	"Asia/Tokyo",
	"Asia/Seoul",
	"Asia/Hong_Kong",
	"Asia/Kolkata",
	"Asia/Dubai",
	"Europe/London",
	"Europe/Amsterdam",
	"Europe/Berlin",
	"America/New_York",
	"America/Chicago",
	"America/Los_Angeles",
	"Australia/Sydney",
}

// handleGetSettingsSystem mengembalikan metadata lingkungan host & database nyata (bebas nilai hardcoded)
func (s *Server) handleGetSettingsSystem(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	dbPath := s.store.Path()
	var dbSize int64
	if dbPath != "" {
		if fi, err := os.Stat(dbPath); err == nil {
			dbSize = fi.Size()
		}
	}
	backupDir := "/opt/fikpanel/backups"
	if dbPath != "" {
		backupDir = filepath.Join(filepath.Dir(dbPath), "backups")
	}

	now := time.Now()
	zoneName, zoneOffset := now.Zone()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"system": map[string]interface{}{
			"hostname":      hostname,
			"os_pretty":     detectHostOSPretty(),
			"kernel":        detectHostKernel(),
			"go_version":    runtime.Version(),
			"arch":          runtime.GOARCH,
			"platform":      runtime.GOOS,
			"cpu_cores":     runtime.NumCPU(),
			"uptime_sec":    detectHostUptimeSec(),
			"version":       s.Version(),
			"db_path":       dbPath,
			"db_size_bytes": dbSize,
			"backup_dir":    backupDir,
			"timezone":      detectHostTimezone(),
			"tz_abbr":       zoneName,
			"tz_offset_sec": zoneOffset,
			"server_unix":   now.Unix(),
			"client_ip":     getIP(r),
		},
		"timezones": commonTimezones,
	})
}

// handleSetHostname mengubah hostname server host menggunakan hostnamectl (Linux)
func (s *Server) handleSetHostname(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	h := strings.TrimSpace(req.Hostname)
	if h == "" || len(h) > 63 || !hostnameRe.MatchString(h) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "format hostname tidak valid (huruf, angka, tanda hubung)"})
		return
	}
	if runtime.GOOS != "linux" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "perubahan hostname sistem hanya didukung di server Linux"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "hostnamectl", "set-hostname", h).CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("hostnamectl gagal: %s", strings.TrimSpace(string(out)))})
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.hostname", h, true, "Mengubah hostname host", uid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"hostname": h,
		"message":  fmt.Sprintf("Hostname berhasil diubah menjadi %s", h),
	})
}

// handleSetTimezone mengubah zona waktu sistem menggunakan timedatectl (Linux)
func (s *Server) handleSetTimezone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Timezone string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	tz := strings.TrimSpace(req.Timezone)
	if tz == "" || !timezoneRe.MatchString(tz) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "format timezone tidak valid (contoh: Asia/Jakarta)"})
		return
	}
	if runtime.GOOS != "linux" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "perubahan timezone sistem hanya didukung di server Linux"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "timedatectl", "set-timezone", tz).CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("timedatectl gagal: %s", strings.TrimSpace(string(out)))})
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.timezone", tz, true, "Mengubah zona waktu host", uid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"timezone": tz,
		"message":  fmt.Sprintf("Timezone berhasil diubah menjadi %s", tz),
	})
}

// handleGetUpdaterStatus menyajikan status versi, auto-update timer, dan log pembaruan terakhir dari internal/updater
func (s *Server) handleGetUpdaterStatus(w http.ResponseWriter, r *http.Request) {
	sys := updater.NewSystem()
	paths := updater.DetectPaths(r.Context(), sys)
	lastStatus, _ := updater.ReadStatus(paths.StateDir)
	autoInfo := updater.AutoUpdateInfo{DisabledByAdmin: updater.AutoUpdateDisabledByAdmin(paths.StateDir)}
	if runtime.GOOS == "linux" {
		autoInfo = updater.AutoUpdateStatus(r.Context(), sys, paths.StateDir)
	}

	var logTail string
	if lastStatus.LogPath != "" {
		if data, err := os.ReadFile(lastStatus.LogPath); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) > 60 {
				lines = lines[len(lines)-60:]
			}
			logTail = strings.Join(lines, "\n")
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":              true,
		"current_version": s.Version(),
		"platform":        runtime.GOOS,
		"auto_update": map[string]interface{}{
			"installed":         autoInfo.Installed,
			"enabled":           autoInfo.Enabled,
			"active":            autoInfo.Active,
			"disabled_by_admin": autoInfo.DisabledByAdmin,
			"next_run":          autoInfo.NextRun,
			"last_run":          autoInfo.LastRun,
		},
		"last_status": lastStatus,
		"log_tail":    logTail,
	})
}

// handleCheckUpdaterRelease memeriksa tag rilis stabil terbaru di GitHub tanpa membentur rate-limit API
func (s *Server) handleCheckUpdaterRelease(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	client := updater.NewClient("FikPanel-Dashboard/" + s.Version())
	latestTag, err := client.ResolveLatestTag(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":               false,
			"current_version":  s.Version(),
			"update_available": false,
			"error":            err.Error(),
		})
		return
	}

	hasUpdate := updater.CompareVersions(s.Version(), latestTag) < 0
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":               true,
		"current_version":  s.Version(),
		"latest_tag":       latestTag,
		"update_available": hasUpdate,
		"release_url":      fmt.Sprintf("https://github.com/%s/releases/tag/%s", updater.DefaultRepo, latestTag),
	})
}

// handleRunUpdater memicu proses self-update atomik terisolasi via systemd-run (fikpanel-updater.service)
func (s *Server) handleRunUpdater(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "linux" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Pembaruan otomatis 1-klik hanya tersedia pada host server Linux"})
		return
	}
	var req struct {
		Target string `json:"target"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	target := strings.TrimSpace(req.Target)
	if target == "" {
		target = "latest"
	}

	sys := updater.NewSystem()
	paths := updater.DetectPaths(r.Context(), sys)
	exe, err := os.Executable()
	if err != nil || exe == "" {
		exe = paths.BinPath
	}

	runID := updater.NewRunID(time.Now())
	args := []string{"update", "--worker", "--run-id=" + runID, "--trigger=dashboard", target}
	if err := updater.LaunchDetached(r.Context(), sys, exe, args); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.update", target, true, "Memicu pembaruan panel dari Settings GUI (run "+runID+")", uid)

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"ok":      true,
		"run_id":  runID,
		"target":  target,
		"message": "Proses pembaruan dimulai di latar belakang. Panel akan menyegarkan otomatis setelah restart.",
	})
}

// handleToggleAutoUpdate mengaktifkan atau menonaktifkan timer auto-update systemd
func (s *Server) handleToggleAutoUpdate(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "linux" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Timer auto-update systemd hanya tersedia di server Linux"})
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload tidak valid"})
		return
	}

	sys := updater.NewSystem()
	paths := updater.DetectPaths(r.Context(), sys)
	var err error
	if req.Enabled {
		err = updater.EnableAutoUpdate(r.Context(), sys, paths.BinPath, paths.StateDir)
	} else {
		err = updater.DisableAutoUpdate(r.Context(), sys, paths.StateDir)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.autoupdate", strconv.FormatBool(req.Enabled), true, "Mengubah status auto-update panel", uid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"enabled": req.Enabled,
		"message": map[bool]string{true: "Auto-update terjadwal diaktifkan", false: "Auto-update terjadwal dinonaktifkan"}[req.Enabled],
	})
}

// handleRestartPanel menjadwalkan restart layanan fikpanel.service secara terlepas (systemd-run) setelah 2 detik
func (s *Server) handleRestartPanel(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "linux" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Restart layanan daemon hanya didukung pada server Linux systemd"})
		return
	}
	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.panel_restart", "fikpanel.service", true, "Memicu restart layanan panel dari Settings", uid)

	// Jadwalkan restart terlepas dari cgroup proses saat ini agar respons HTTP 200 terkirim utuh
	go func() {
		time.Sleep(1500 * time.Millisecond)
		if err := exec.Command("systemd-run", "--no-block", "--unit=fikpanel-self-restart", "systemctl", "restart", "fikpanel.service").Run(); err != nil {
			_ = exec.Command("systemctl", "restart", "fikpanel.service").Run()
		}
	}()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": "Layanan FikPanel akan dimulai ulang dalam 2 detik. Muat ulang halaman sesaat lagi.",
	})
}

// ═══ E. Upgrade Webhook (Update & Test Kirim Real) ═══

// handleUpdateWebhook memperbarui konfigurasi nama, kanal, URL, dan event trigger webhook
func (s *Server) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID webhook tidak valid"})
		return
	}
	var hook store.SystemWebhook
	if err := json.NewDecoder(r.Body).Decode(&hook); err != nil || strings.TrimSpace(hook.Name) == "" || strings.TrimSpace(hook.WebhookURL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nama dan URL webhook wajib diisi"})
		return
	}
	hook.ID = id
	hook.Name = strings.TrimSpace(hook.Name)
	hook.WebhookURL = strings.TrimSpace(hook.WebhookURL)
	if err := s.store.UpdateWebhook(r.Context(), &hook); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memperbarui webhook"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "webhook": hook})
}

// handleTestWebhook mengirim notifikasi uji coba nyata ke Telegram, Discord Embed, atau endpoint HTTP kustom
func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID webhook tidak valid"})
		return
	}
	hook, err := s.store.GetWebhook(r.Context(), id)
	if err != nil || hook == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook tidak ditemukan"})
		return
	}

	targetURL, payloadBytes, err := buildTestWebhookRequest(hook, s.Version())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, targetURL, bytes.NewReader(payloadBytes))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("URL webhook tidak valid: %v", err)})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FikPanel-Webhook/"+s.Version())

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":         false,
			"latency_ms": latency,
			"error":      fmt.Sprintf("Gagal mengirim ke endpoint: %v", err),
		})
		return
	}
	defer resp.Body.Close()
	bodyPreview, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":          false,
			"status_code": resp.StatusCode,
			"latency_ms":  latency,
			"error":       fmt.Sprintf("Endpoint membalas HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyPreview))),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"status_code": resp.StatusCode,
		"latency_ms":  latency,
		"message":     fmt.Sprintf("Notifikasi uji coba terkirim (HTTP %d, %d ms)", resp.StatusCode, latency),
	})
}

func buildTestWebhookRequest(hook *store.SystemWebhook, ver string) (string, []byte, error) {
	host, _ := os.Hostname()
	nowStr := time.Now().Format("2006-01-02 15:04:05 MST")
	msgText := fmt.Sprintf("[FikPanel v%s] Uji coba notifikasi webhook '%s' dari host %s (%s)", ver, hook.Name, host, nowStr)

	rawURL := strings.TrimSpace(hook.WebhookURL)
	// Mendukung format ringkas Telegram tg://<bot_token>/<chat_id> maupun URL https:// penuh
	if strings.HasPrefix(rawURL, "tg://") {
		trimmed := strings.TrimPrefix(rawURL, "tg://")
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", nil, fmt.Errorf("format Telegram harus tg://<bot_token>/<chat_id> atau URL Bot API penuh")
		}
		endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", parts[0])
		payload, _ := json.Marshal(map[string]interface{}{
			"chat_id": parts[1],
			"text":    msgText,
		})
		return endpoint, payload, nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", nil, fmt.Errorf("URL webhook wajib memakai protokol http:// atau https://")
	}

	switch strings.ToLower(hook.Channel) {
	case "telegram":
		chatID := parsed.Query().Get("chat_id")
		body := map[string]interface{}{"text": msgText}
		if chatID != "" {
			body["chat_id"] = chatID
		}
		payload, _ := json.Marshal(body)
		return rawURL, payload, nil
	case "discord":
		payload, _ := json.Marshal(map[string]interface{}{
			"username": "FikPanel Alert",
			"embeds": []map[string]interface{}{
				{
					"title":       "Uji Coba Webhook FikPanel",
					"description": msgText,
					"color":       16734763,
					"footer":      map[string]string{"text": fmt.Sprintf("Host: %s · v%s", host, ver)},
				},
			},
		})
		return rawURL, payload, nil
	default:
		payload, _ := json.Marshal(map[string]interface{}{
			"event":     "test",
			"source":    "fikpanel",
			"version":   ver,
			"host":      host,
			"webhook":   hook.Name,
			"message":   msgText,
			"timestamp": time.Now().Unix(),
		})
		return rawURL, payload, nil
	}
}

// ═══ F. Snapshot Backup SQLite & Export/Import Profil Konfigurasi Panel ═══

// handleDownloadDBBackup membuat snapshot atomik SQLite via VACUUM INTO dan mengirimkannya sebagai unduhan .db
func (s *Server) handleDownloadDBBackup(w http.ResponseWriter, r *http.Request) {
	tmpFile, err := os.CreateTemp("", "fikpanel-snapshot-*.db")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membuat berkas sementara backup"})
		return
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	_ = os.Remove(tmpPath) // VACUUM INTO mewajibkan berkas tujuan belum ada
	defer os.Remove(tmpPath)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := s.store.VacuumBackup(ctx, tmpPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membaca snapshot backup"})
		return
	}
	defer f.Close()

	fi, _ := f.Stat()
	filename := fmt.Sprintf("fikpanel-backup-%s.db", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/x-sqlite3")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if fi != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	}
	_, _ = io.Copy(w, f)

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "settings.db_backup", filename, true, "Mengunduh snapshot database SQLite panel", uid)
}

type panelConfigExportBundle struct {
	SchemaVersion    int                     `json:"schema_version"`
	ExportedAt       int64                   `json:"exported_at"`
	PanelVersion     string                  `json:"panel_version"`
	Preferences      PanelPreferences        `json:"preferences"`
	TerminalCommands []store.TerminalCommand `json:"terminal_commands"`
	Webhooks         []store.SystemWebhook   `json:"webhooks"`
}

// handleExportPanelConfig mengekspor profil pengaturan panel (tema, branding, bahasa, shortcut terminal, webhook) sebagai JSON
func (s *Server) handleExportPanelConfig(w http.ResponseWriter, r *http.Request) {
	prefs := s.loadPanelPreferences(r.Context())
	cmds, _ := s.store.ListTerminalCommands(r.Context())
	if cmds == nil {
		cmds = []store.TerminalCommand{}
	}
	hooks, _ := s.store.ListWebhooks(r.Context())
	if hooks == nil {
		hooks = []store.SystemWebhook{}
	}

	bundle := panelConfigExportBundle{
		SchemaVersion:    1,
		ExportedAt:       time.Now().Unix(),
		PanelVersion:     s.Version(),
		Preferences:      prefs,
		TerminalCommands: cmds,
		Webhooks:         hooks,
	}

	filename := fmt.Sprintf("fikpanel-config-%s.json", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(bundle)
}

// handleImportPanelConfig mengimpor berkas profil JSON (preferensi tampilan/bahasa, shortcut command baru, dan webhook baru)
func (s *Server) handleImportPanelConfig(w http.ResponseWriter, r *http.Request) {
	var bundle panelConfigExportBundle
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&bundle); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "berkas JSON konfigurasi tidak valid"})
		return
	}

	// Amankan IP allowlist saat impor agar tidak mengunci admin di server baru
	bundle.Preferences.IPAllowlist = s.loadPanelPreferences(r.Context()).IPAllowlist
	cleanPrefs, err := sanitizePanelPreferences(bundle.Preferences)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.savePanelPreferences(r.Context(), cleanPrefs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menerapkan preferensi impor"})
		return
	}

	// Impor shortcut terminal yang belum ada (hindari duplikat berdasarkan nama)
	existingCmds, _ := s.store.ListTerminalCommands(r.Context())
	cmdNames := make(map[string]bool, len(existingCmds))
	for _, c := range existingCmds {
		cmdNames[strings.ToLower(strings.TrimSpace(c.Name))] = true
	}
	addedCmds := 0
	for _, c := range bundle.TerminalCommands {
		name := strings.TrimSpace(c.Name)
		content := strings.TrimSpace(c.Content)
		if name == "" || content == "" || cmdNames[strings.ToLower(name)] {
			continue
		}
		if _, err := s.store.CreateTerminalCommand(r.Context(), &store.TerminalCommand{Name: name, Content: content}); err == nil {
			cmdNames[strings.ToLower(name)] = true
			addedCmds++
		}
	}

	// Impor webhook yang belum ada (hindari duplikat berdasarkan URL)
	existingHooks, _ := s.store.ListWebhooks(r.Context())
	hookURLs := make(map[string]bool, len(existingHooks))
	for _, h := range existingHooks {
		hookURLs[strings.TrimSpace(h.WebhookURL)] = true
	}
	addedHooks := 0
	for _, h := range bundle.Webhooks {
		name := strings.TrimSpace(h.Name)
		u := strings.TrimSpace(h.WebhookURL)
		if name == "" || u == "" || hookURLs[u] {
			continue
		}
		if _, err := s.store.CreateWebhook(r.Context(), &store.SystemWebhook{
			Name:              name,
			Channel:           h.Channel,
			WebhookURL:        u,
			NotifyOnHighCPU:   h.NotifyOnHighCPU,
			NotifyOnHighRAM:   h.NotifyOnHighRAM,
			NotifyOnBackup:    h.NotifyOnBackup,
			NotifyOnSSLExpiry: h.NotifyOnSSLExpiry,
		}); err == nil {
			hookURLs[u] = true
			addedHooks++
		}
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "settings.config_import", "panel", true,
		fmt.Sprintf("Impor profil: +%d commands, +%d webhooks", addedCmds, addedHooks), uid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":             true,
		"preferences":    cleanPrefs,
		"added_commands": addedCmds,
		"added_webhooks": addedHooks,
		"message":        fmt.Sprintf("Profil diterapkan (+%d perintah terminal, +%d webhook)", addedCmds, addedHooks),
	})
}
