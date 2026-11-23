package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"fikpanel/internal/store"
)

// webauthnUserAdapter menjembatani store.User + daftar kredensial SQLite agar memenuhi interface webauthn.User
type webauthnUserAdapter struct {
	user  *store.User
	creds []webauthn.Credential
}

func (u *webauthnUserAdapter) WebAuthnID() []byte {
	// Handle stabil 8-byte big-endian berbasis ID numerik pengguna (unik & tetap di seluruh sesi)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(u.user.ID))
	return buf
}

func (u *webauthnUserAdapter) WebAuthnName() string {
	return u.user.Username
}

func (u *webauthnUserAdapter) WebAuthnDisplayName() string {
	return u.user.Username
}

func (u *webauthnUserAdapter) WebAuthnCredentials() []webauthn.Credential {
	return u.creds
}

type pendingWebAuthnSession struct {
	data      webauthn.SessionData
	userID    int64
	expiresAt time.Time
}

var (
	passkeySessMu sync.Mutex
	passkeyRegMap = make(map[string]pendingWebAuthnSession)
	passkeyLogMap = make(map[string]pendingWebAuthnSession)
)

func generateCeremonyID() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func cleanupExpiredCeremonies(m map[string]pendingWebAuthnSession) {
	now := time.Now()
	for k, v := range m {
		if now.After(v.expiresAt) {
			delete(m, k)
		}
	}
}

// buildWebAuthnForRequest membentuk konfigurasi WebAuthn dinamis sesuai domain/origin HTTPS permintaan saat ini
func (s *Server) buildWebAuthnForRequest(r *http.Request) (*webauthn.WebAuthn, error) {
	rawHost := strings.TrimSpace(r.Host)
	if xfHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); xfHost != "" {
		rawHost = strings.Split(xfHost, ",")[0]
	}

	originHeader := strings.TrimSpace(r.Header.Get("Origin"))
	hostOnly := rawHost
	if h, _, err := net.SplitHostPort(rawHost); err == nil {
		hostOnly = h
	}
	hostOnly = strings.Trim(strings.ToLower(strings.TrimSpace(hostOnly)), "[]")

	// Jika Origin header ada dan valid, pastikan host-nya selaras
	origins := make([]string, 0, 3)
	if originHeader != "" {
		if u, err := url.Parse(originHeader); err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
			origHost := strings.ToLower(u.Hostname())
			if hostOnly == "" {
				hostOnly = origHost
			}
			origins = append(origins, u.Scheme+"://"+u.Host)
		}
	}

	if hostOnly == "" {
		return nil, fmt.Errorf("domain host tidak dikenali untuk WebAuthn")
	}

	// Browser menolak WebAuthn pada alamat IP mentah (kecuali localhost)
	if ip := net.ParseIP(hostOnly); ip != nil {
		return nil, fmt.Errorf("Passkey/WebAuthn mewajibkan akses melalui domain HTTPS atau localhost (tidak didukung browser pada alamat IP mentah)")
	}

	if rawHost != "" {
		httpsOrigin := "https://" + rawHost
		if !containsString(origins, httpsOrigin) {
			origins = append(origins, httpsOrigin)
		}
		if hostOnly == "localhost" {
			httpOrigin := "http://" + rawHost
			if !containsString(origins, httpOrigin) {
				origins = append(origins, httpOrigin)
			}
		}
	}

	prefs := s.loadPanelPreferences(r.Context())
	displayName := strings.TrimSpace(prefs.BrandName)
	if displayName == "" {
		displayName = "FikPanel"
	}

	cfg := &webauthn.Config{
		RPID:          hostOnly,
		RPDisplayName: displayName,
		RPOrigins:     origins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification:   protocol.VerificationPreferred,
		},
	}

	return webauthn.New(cfg)
}

func containsString(list []string, target string) bool {
	for _, s := range list {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

func (s *Server) loadWebAuthnUser(r *http.Request, user *store.User) (*webauthnUserAdapter, error) {
	rows, err := s.store.ListUserPasskeys(r.Context(), user.ID)
	if err != nil {
		return nil, err
	}
	creds := make([]webauthn.Credential, 0, len(rows))
	for _, row := range rows {
		var c webauthn.Credential
		if json.Unmarshal([]byte(row.CredentialJSON), &c) == nil {
			creds = append(creds, c)
		}
	}
	return &webauthnUserAdapter{user: user, creds: creds}, nil
}

// handleListPasskeys mengembalikan daftar perangkat Passkey milik pengguna yang sedang login
func (s *Server) handleListPasskeys(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	list, err := s.store.ListUserPasskeys(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membaca daftar passkey"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"passkeys": list,
	})
}

// handlePasskeyRegisterBegin memulai seremoni pendaftaran Passkey baru untuk akun yang sedang login
func (s *Server) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	wa, err := s.buildWebAuthnForRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	waUser, err := s.loadWebAuthnUser(r, user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memuat kredensial pengguna"})
		return
	}

	exclusions := make([]protocol.CredentialDescriptor, 0, len(waUser.creds))
	for _, c := range waUser.creds {
		exclusions = append(exclusions, c.Descriptor())
	}

	options, sessionData, err := wa.BeginRegistration(
		waUser,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithExclusions(exclusions),
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memulai pendaftaran passkey: " + err.Error()})
		return
	}

	sessID := generateCeremonyID()
	passkeySessMu.Lock()
	cleanupExpiredCeremonies(passkeyRegMap)
	passkeyRegMap[sessID] = pendingWebAuthnSession{
		data:      *sessionData,
		userID:    user.ID,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	passkeySessMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"session_id": sessID,
		"publicKey":  options.Response,
	})
}

// handlePasskeyRegisterFinish memverifikasi attestation pendaftaran dan menyimpan Passkey ke SQLite
func (s *Server) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req struct {
		SessionID  string          `json:"session_id"`
		Name       string          `json:"name"`
		Credential json.RawMessage `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload pendaftaran passkey tidak valid"})
		return
	}

	passkeySessMu.Lock()
	pending, exists := passkeyRegMap[req.SessionID]
	delete(passkeyRegMap, req.SessionID)
	passkeySessMu.Unlock()

	if !exists || time.Now().After(pending.expiresAt) || pending.userID != user.ID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sesi pendaftaran passkey kedaluwarsa, silakan ulangi"})
		return
	}

	wa, err := s.buildWebAuthnForRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	waUser, err := s.loadWebAuthnUser(r, user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memuat data pengguna"})
		return
	}

	parsed, err := protocol.ParseCredentialCreationResponseBytes(req.Credential)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "respons kredensial browser tidak valid: " + err.Error()})
		return
	}

	cred, err := wa.CreateCredential(waUser, pending.data, parsed)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "verifikasi passkey gagal: " + err.Error()})
		return
	}

	credBytes, err := json.Marshal(cred)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal serialisasi kredensial"})
		return
	}

	label := strings.TrimSpace(req.Name)
	if label == "" {
		label = "Passkey Perangkat"
	}
	if len(label) > 60 {
		label = label[:60]
	}

	transports := make([]string, 0, len(cred.Transport))
	for _, tr := range cred.Transport {
		transports = append(transports, string(tr))
	}

	credIDB64 := base64.RawURLEncoding.EncodeToString(cred.ID)
	saved, err := s.store.CreatePasskey(r.Context(), user.ID, label, credIDB64, string(credBytes), strings.Join(transports, ","))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menyimpan passkey ke database"})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "auth.passkey_register", user.Username, true,
		fmt.Sprintf("Mendaftarkan passkey: %s", label), &user.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"passkey": saved,
		"message": fmt.Sprintf("Passkey \"%s\" berhasil didaftarkan", label),
	})
}

// handleDeletePasskey menghapus satu perangkat Passkey milik pengguna yang sedang login
func (s *Server) handleDeletePasskey(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID passkey tidak valid"})
		return
	}
	deleted, err := s.store.DeleteUserPasskey(r.Context(), user.ID, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menghapus passkey"})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "passkey tidak ditemukan"})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "auth.passkey_delete", user.Username, true,
		fmt.Sprintf("Menghapus passkey ID #%d", id), &user.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": "Passkey berhasil dihapus",
	})
}

// handlePasskeyLoginBegin memulai login tanpa kata sandi (discoverable passkey / QR HP / Windows Hello)
func (s *Server) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	ip := getIP(r)
	if !s.limiter.Allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "terlalu banyak percobaan gagal, coba lagi nanti"})
		return
	}

	if s.store.CountAllPasskeys(r.Context()) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "belum ada passkey yang terdaftar pada panel ini"})
		return
	}

	wa, err := s.buildWebAuthnForRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	assertion, sessionData, err := wa.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationPreferred),
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memulai login passkey: " + err.Error()})
		return
	}

	sessID := generateCeremonyID()
	passkeySessMu.Lock()
	cleanupExpiredCeremonies(passkeyLogMap)
	passkeyLogMap[sessID] = pendingWebAuthnSession{
		data:      *sessionData,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	passkeySessMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"session_id": sessID,
		"publicKey":  assertion.Response,
	})
}

// handlePasskeyLoginFinish memverifikasi tanda tangan kriptografi Passkey dan menerbitkan sesi panel
func (s *Server) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	ip := getIP(r)
	if !s.limiter.Allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "terlalu banyak percobaan gagal, coba lagi nanti"})
		return
	}

	var req struct {
		SessionID  string          `json:"session_id"`
		Credential json.RawMessage `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload login passkey tidak valid"})
		return
	}

	passkeySessMu.Lock()
	pending, exists := passkeyLogMap[req.SessionID]
	delete(passkeyLogMap, req.SessionID)
	passkeySessMu.Unlock()

	if !exists || time.Now().After(pending.expiresAt) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tantangan passkey kedaluwarsa, silakan coba lagi"})
		return
	}

	wa, err := s.buildWebAuthnForRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	parsed, err := protocol.ParseCredentialRequestResponseBytes(req.Credential)
	if err != nil {
		s.limiter.RecordFailure(ip)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "respons passkey tidak valid: " + err.Error()})
		return
	}

	var matchedUser *store.User
	var matchedPasskey *store.UserPasskey

	userHandler := func(rawID, userHandle []byte) (webauthn.User, error) {
		credIDB64 := base64.RawURLEncoding.EncodeToString(rawID)
		pk, err := s.store.FindPasskeyByCredentialID(r.Context(), credIDB64)
		if err != nil {
			return nil, fmt.Errorf("passkey tidak dikenali")
		}
		dbUser, err := s.store.GetUserByID(r.Context(), pk.UserID)
		if err != nil {
			return nil, fmt.Errorf("pengguna pemilik passkey tidak ditemukan")
		}
		matchedUser = dbUser
		matchedPasskey = pk
		return s.loadWebAuthnUser(r, dbUser)
	}

	_, cred, err := wa.ValidatePasskeyLogin(userHandler, pending.data, parsed)
	if err != nil || matchedUser == nil {
		s.limiter.RecordFailure(ip)
		_ = s.store.RecordAudit(r.Context(), "user", "auth.passkey_login_fail", ip, false, "Verifikasi tanda tangan passkey gagal", nil)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "verifikasi passkey gagal"})
		return
	}

	// Tolak bila authenticator terdeteksi dikloning (sign counter mundur)
	if cred.Authenticator.CloneWarning {
		s.limiter.RecordFailure(ip)
		_ = s.store.RecordAudit(r.Context(), "user", "auth.passkey_clone_warning", matchedUser.Username, false, "Peringatan kloning authenticator terdeteksi", &matchedUser.ID)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "kredensial passkey terdeteksi tidak konsisten (clone warning)"})
		return
	}

	// Simpan pembaruan sign counter & waktu pakai terakhir
	if updatedBytes, err := json.Marshal(cred); err == nil && matchedPasskey != nil {
		_ = s.store.UpdatePasskeyUsage(r.Context(), matchedPasskey.CredentialID, string(updatedBytes))
	}

	s.limiter.Reset(ip)
	_ = s.store.UpdateLastLogin(r.Context(), matchedUser.ID)

	prefs := s.loadPanelPreferences(r.Context())
	sessionDur := time.Duration(prefs.SessionHours) * time.Hour
	if sessionDur < time.Hour {
		sessionDur = 24 * time.Hour
	}

	token, err := s.store.CreateSession(r.Context(), matchedUser.ID, ip, r.UserAgent(), sessionDur)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membuat sesi login"})
		return
	}

	pkLabel := "Passkey"
	if matchedPasskey != nil {
		pkLabel = matchedPasskey.Name
	}
	_ = s.store.RecordAudit(r.Context(), "user", "auth.passkey_login_success", matchedUser.Username, true,
		fmt.Sprintf("Login tanpa sandi via %s (IP: %s)", pkLabel, ip), &matchedUser.ID)

	http.SetCookie(w, &http.Cookie{
		Name:     "panel_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionDur.Seconds()),
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"user": map[string]interface{}{
			"id":           matchedUser.ID,
			"username":     matchedUser.Username,
			"role":         matchedUser.Role,
			"totp_enabled": len(matchedUser.TotpSecret) > 0,
		},
	})
}
