package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"fikpanel/internal/auth"
)

const vaultUnlockTTL = 5 * time.Minute

type vaultUnlockEntry struct {
	userID    int64
	expiresAt time.Time
}

var (
	vaultMu           sync.RWMutex
	vaultUnlocks      = make(map[string]vaultUnlockEntry)
	vaultWebAuthnMu   sync.Mutex
	vaultWebAuthnSess = make(map[string]pendingWebAuthnSession)
)

// vaultSessionKey menghasilkan kunci deterministik berbasis hash SHA-256 cookie sesi agar token mentah tidak tersimpan di map
func vaultSessionKey(r *http.Request) string {
	c, err := r.Cookie("panel_session")
	if err != nil || strings.TrimSpace(c.Value) == "" {
		// Fallback untuk pengujian unit tanpa cookie sesi: ikat berdasarkan user ID konteks
		if u := GetUserFromContext(r.Context()); u != nil {
			return fmt.Sprintf("uid:%d", u.ID)
		}
		return ""
	}
	sum := sha256.Sum256([]byte(c.Value))
	return hex.EncodeToString(sum[:])
}

// isVaultUnlocked memeriksa apakah sesi saat ini telah lolos verifikasi step-up Vault dan belum kedaluwarsa
func (s *Server) isVaultUnlocked(r *http.Request) (bool, int64) {
	key := vaultSessionKey(r)
	if key == "" {
		return false, 0
	}
	user := GetUserFromContext(r.Context())

	vaultMu.RLock()
	entry, ok := vaultUnlocks[key]
	vaultMu.RUnlock()
	if !ok {
		return false, 0
	}
	if time.Now().After(entry.expiresAt) {
		vaultMu.Lock()
		delete(vaultUnlocks, key)
		vaultMu.Unlock()
		return false, 0
	}
	if user != nil && entry.userID != 0 && entry.userID != user.ID {
		return false, 0
	}
	return true, entry.expiresAt.Unix()
}

// grantVaultUnlock membuka kunci Vault untuk sesi saat ini selama 5 menit
func (s *Server) grantVaultUnlock(r *http.Request, userID int64) int64 {
	key := vaultSessionKey(r)
	if key == "" {
		return 0
	}
	exp := time.Now().Add(vaultUnlockTTL)
	vaultMu.Lock()
	// Bersihkan entri kedaluwarsa saat penulisan agar memori tetap bersih
	now := time.Now()
	for k, v := range vaultUnlocks {
		if now.After(v.expiresAt) {
			delete(vaultUnlocks, k)
		}
	}
	vaultUnlocks[key] = vaultUnlockEntry{
		userID:    userID,
		expiresAt: exp,
	}
	vaultMu.Unlock()
	return exp.Unix()
}

// revokeVaultUnlock mengunci kembali Vault untuk sesi saat ini secara instan
func (s *Server) revokeVaultUnlock(r *http.Request) {
	key := vaultSessionKey(r)
	if key == "" {
		return
	}
	vaultMu.Lock()
	delete(vaultUnlocks, key)
	vaultMu.Unlock()
}

// handleVaultStatus melaporkan status kunci Vault, sisa durasi buka kunci, dan ketersediaan Passkey pengguna
func (s *Server) handleVaultStatus(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	unlocked, expUnix := s.isVaultUnlocked(r)
	remaining := int64(0)
	if unlocked && expUnix > time.Now().Unix() {
		remaining = expUnix - time.Now().Unix()
	}

	passkeys, _ := s.store.ListUserPasskeys(r.Context(), user.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":            true,
		"unlocked":      unlocked,
		"expires_at":    expUnix,
		"remaining_sec": remaining,
		"has_passkeys":  len(passkeys) > 0,
		"passkey_count": len(passkeys),
		"username":      user.Username,
	})
}

// handleVaultUnlockPassword membuka Vault setelah memverifikasi kata sandi akun administrator yang sedang login
func (s *Server) handleVaultUnlockPassword(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	ip := getIP(r)
	if !s.limiter.Allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "terlalu banyak percobaan gagal, coba lagi nanti"})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kata sandi administrator wajib diisi"})
		return
	}

	// Baca ulang user terbaru dari database untuk mendapatkan PasswordHash terkini
	dbUser, err := s.store.GetUserByID(r.Context(), user.ID)
	if err != nil || dbUser == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "akun administrator tidak ditemukan"})
		return
	}

	valid, err := auth.VerifyPassword(req.Password, dbUser.PasswordHash)
	if err != nil || !valid {
		s.limiter.RecordFailure(ip)
		_ = s.store.RecordAudit(r.Context(), "user", "auth.vault_unlock_fail", dbUser.Username, false,
			"Verifikasi kata sandi pembukaan Vault gagal", &dbUser.ID)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "kata sandi administrator tidak cocok"})
		return
	}

	s.limiter.Reset(ip)
	expUnix := s.grantVaultUnlock(r, dbUser.ID)
	_ = s.store.RecordAudit(r.Context(), "user", "auth.vault_unlock_password", dbUser.Username, true,
		"Membuka Vault kredensial via kata sandi administrator (5 menit)", &dbUser.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":            true,
		"unlocked":      true,
		"expires_at":    expUnix,
		"remaining_sec": int64(vaultUnlockTTL.Seconds()),
		"message":       "Vault terbuka selama 5 menit",
	})
}

// handleVaultUnlockPasskeyBegin memulai tantangan FIDO2/Biometrik (Windows Hello / Touch ID / QR HP) untuk membuka Vault
func (s *Server) handleVaultUnlockPasskeyBegin(w http.ResponseWriter, r *http.Request) {
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
	if err != nil || len(waUser.creds) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "belum ada Passkey/Biometrik yang terdaftar pada akun ini — gunakan kata sandi admin atau daftarkan Passkey di Pengaturan"})
		return
	}

	// Wajibkan verifikasi pengguna (biometrik/PIN perangkat) untuk proteksi Vault step-up
	assertion, sessionData, err := wa.BeginLogin(
		waUser,
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memulai verifikasi Passkey Vault: " + err.Error()})
		return
	}

	sessID := generateCeremonyID()
	vaultWebAuthnMu.Lock()
	cleanupExpiredCeremonies(vaultWebAuthnSess)
	vaultWebAuthnSess[sessID] = pendingWebAuthnSession{
		data:      *sessionData,
		userID:    user.ID,
		expiresAt: time.Now().Add(3 * time.Minute),
	}
	vaultWebAuthnMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"session_id": sessID,
		"publicKey":  assertion.Response,
	})
}

// handleVaultUnlockPasskeyFinish memverifikasi tanda tangan FIDO2/Biometrik dan membuka Vault selama 5 menit
func (s *Server) handleVaultUnlockPasskeyFinish(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload verifikasi Passkey tidak valid"})
		return
	}

	vaultWebAuthnMu.Lock()
	pending, exists := vaultWebAuthnSess[req.SessionID]
	delete(vaultWebAuthnSess, req.SessionID)
	vaultWebAuthnMu.Unlock()

	if !exists || time.Now().After(pending.expiresAt) || pending.userID != user.ID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tantangan Passkey Vault kedaluwarsa, silakan coba lagi"})
		return
	}

	wa, err := s.buildWebAuthnForRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	waUser, err := s.loadWebAuthnUser(r, user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memuat Passkey akun"})
		return
	}

	parsed, err := protocol.ParseCredentialRequestResponseBytes(req.Credential)
	if err != nil {
		s.limiter.RecordFailure(ip)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "respons Passkey tidak valid: " + err.Error()})
		return
	}

	cred, err := wa.ValidateLogin(waUser, pending.data, parsed)
	if err != nil {
		s.limiter.RecordFailure(ip)
		_ = s.store.RecordAudit(r.Context(), "user", "auth.vault_unlock_fail", user.Username, false,
			"Verifikasi biometrik/Passkey pembukaan Vault gagal", &user.ID)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "verifikasi Passkey/Biometrik gagal"})
		return
	}

	if cred.Authenticator.CloneWarning {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "peringatan kloning authenticator terdeteksi"})
		return
	}

	// Perbarui sign counter dan waktu terakhir dipakai
	credIDB64 := base64.RawURLEncoding.EncodeToString(cred.ID)
	if updatedBytes, err := json.Marshal(cred); err == nil {
		_ = s.store.UpdatePasskeyUsage(r.Context(), credIDB64, string(updatedBytes))
	}

	s.limiter.Reset(ip)
	expUnix := s.grantVaultUnlock(r, user.ID)
	_ = s.store.RecordAudit(r.Context(), "user", "auth.vault_unlock_passkey", user.Username, true,
		"Membuka Vault kredensial via Passkey/Biometrik (5 menit)", &user.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":            true,
		"unlocked":      true,
		"expires_at":    expUnix,
		"remaining_sec": int64(vaultUnlockTTL.Seconds()),
		"message":       "Vault terbuka via Biometrik/Passkey selama 5 menit",
	})
}

// handleVaultLock mengunci kembali Vault kredensial secara manual untuk sesi aktif
func (s *Server) handleVaultLock(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	s.revokeVaultUnlock(r)
	if user != nil {
		_ = s.store.RecordAudit(r.Context(), "user", "auth.vault_lock", user.Username, true,
			"Mengunci kembali Vault kredensial secara manual", &user.ID)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"unlocked": false,
		"message":  "Vault kredensial telah dikunci kembali",
	})
}

// handleRevealDatabaseSecret mengembalikan kredensial plaintext dan blok .env database HANYA jika Vault terbuka
func (s *Server) handleRevealDatabaseSecret(w http.ResponseWriter, r *http.Request) {
	unlocked, expUnix := s.isVaultUnlocked(r)
	if !unlocked {
		writeJSON(w, http.StatusLocked, map[string]interface{}{
			"ok":           false,
			"vault_locked": true,
			"error":        "Vault terkunci — verifikasi Passkey/Biometrik atau kata sandi admin diperlukan untuk melihat atau menyalin kredensial",
		})
		return
	}

	engine := strings.ToLower(r.PathValue("engine"))
	name := strings.TrimSpace(r.PathValue("name"))
	if engine == "" || name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parameter engine dan nama database wajib diisi"})
		return
	}

	rec, err := s.store.GetDatabaseByName(r.Context(), engine, name)
	if err != nil || rec == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "database tidak ditemukan"})
		return
	}

	host := strings.TrimSpace(rec.Location)
	if host == "" || host == "%" {
		host = "127.0.0.1"
	}

	var port int
	var connDriver, dsnURL string
	if engine == "postgresql" || engine == "postgres" {
		port = 5432
		connDriver = "pgsql"
		dsnURL = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", rec.Username, url.QueryEscape(rec.Password), host, port, rec.Name)
	} else {
		port = 3306
		connDriver = "mysql"
		dsnURL = fmt.Sprintf("mysql://%s:%s@%s:%d/%s", rec.Username, url.QueryEscape(rec.Password), host, port, rec.Name)
	}

	envBlock := strings.Join([]string{
		"DB_CONNECTION=" + connDriver,
		"DB_HOST=" + host,
		fmt.Sprintf("DB_PORT=%d", port),
		"DB_DATABASE=" + rec.Name,
		"DB_USERNAME=" + rec.Username,
		"DB_PASSWORD=" + rec.Password,
		"# DATABASE_URL=" + dsnURL,
	}, "\n")

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "database.reveal_secret", fmt.Sprintf("%s:%s", engine, name), true,
		fmt.Sprintf("Membuka kredensial database %s (%s) melalui Vault terverifikasi", name, engine), uid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":           true,
		"engine":       engine,
		"name":         rec.Name,
		"username":     rec.Username,
		"password":     rec.Password,
		"has_password": strings.TrimSpace(rec.Password) != "",
		"host":         host,
		"port":         port,
		"env_block":    envBlock,
		"dsn_url":      dsnURL,
		"expires_at":   expUnix,
	})
}
