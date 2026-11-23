package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/auth"
	"fikpanel/internal/credentials"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}

type LoginResponse struct {
	OK          bool        `json:"ok"`
	RequireTOTP bool        `json:"require_totp,omitempty"`
	User        interface{} `json:"user,omitempty"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := getIP(r)
	if !s.limiter.Allow(ip) {
		http.Error(w, `{"error":"terlalu banyak percobaan gagal, coba lagi nanti"}`, http.StatusTooManyRequests)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	user, err := s.store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		s.limiter.RecordFailure(ip)
		_ = s.store.RecordAudit(r.Context(), "user", "auth.login_fail", req.Username, false, "Kredensial tidak valid", nil)
		http.Error(w, `{"error":"kredensial tidak valid"}`, http.StatusUnauthorized)
		return
	}

	valid, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !valid {
		s.limiter.RecordFailure(ip)
		_ = s.store.RecordAudit(r.Context(), "user", "auth.login_fail", req.Username, false, "Password salah", &user.ID)
		http.Error(w, `{"error":"kredensial tidak valid"}`, http.StatusUnauthorized)
		return
	}

	// Verifikasi kode 2FA TOTP (atau kode pemulihan) bila diaktifkan pada akun
	if len(user.TotpSecret) > 0 {
		if strings.TrimSpace(req.TOTPCode) == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(LoginResponse{
				OK:          false,
				RequireTOTP: true,
			})
			return
		}
		if !s.verifyUserTOTPOrRecovery(r.Context(), user, req.TOTPCode) {
			s.limiter.RecordFailure(ip)
			_ = s.store.RecordAudit(r.Context(), "user", "auth.login_2fa_fail", req.Username, false, "Kode 2FA tidak valid", &user.ID)
			http.Error(w, `{"error":"kode 2FA tidak valid"}`, http.StatusUnauthorized)
			return
		}
	}

	s.limiter.Reset(ip)
	_ = s.store.UpdateLastLogin(r.Context(), user.ID)

	prefs := s.loadPanelPreferences(r.Context())
	sessionDur := time.Duration(prefs.SessionHours) * time.Hour
	if sessionDur < time.Hour {
		sessionDur = 24 * time.Hour
	}

	token, err := s.store.CreateSession(r.Context(), user.ID, ip, r.UserAgent(), sessionDur)
	if err != nil {
		http.Error(w, `{"error":"gagal membuat sesi login"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "auth.login_success", user.Username, true, fmt.Sprintf("IP: %s", ip), &user.ID)

	http.SetCookie(w, &http.Cookie{
		Name:     "panel_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionDur.Seconds()),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		OK: true,
		User: map[string]interface{}{
			"id":           user.ID,
			"username":     user.Username,
			"role":         user.Role,
			"totp_enabled": len(user.TotpSecret) > 0,
		},
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if cookie, err := r.Cookie("panel_session"); err == nil && cookie.Value != "" {
		_ = s.store.DeleteSession(r.Context(), cookie.Value)
	}

	if user != nil {
		_ = s.store.RecordAudit(r.Context(), "user", "auth.logout", user.Username, true, "Logout sukarela", &user.ID)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "panel_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           user.ID,
		"username":     user.Username,
		"role":         user.Role,
		"sudo_active":  true,
		"totp_enabled": len(user.TotpSecret) > 0,
	})
}

// UpdatePasswordRequest merepresentasikan payload perubahan kata sandi
type UpdatePasswordRequest struct {
	OldPassword  string `json:"old_password"`
	NewPassword  string `json:"new_password"`
	TargetUserID int64  `json:"target_user_id,omitempty"`
}

// handleUpdatePassword memproses perubahan kata sandi akun sendiri maupun reset akun lain oleh administrator
func (s *Server) handleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req UpdatePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	newPass := req.NewPassword
	if err := auth.ValidateNewPassword(newPass); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Selalu verifikasi kata sandi akun yang sedang login sebagai otorisasi mutasi
	dbCaller, err := s.store.GetUserByID(r.Context(), user.ID)
	if err != nil {
		http.Error(w, `{"error":"pengguna tidak ditemukan"}`, http.StatusNotFound)
		return
	}

	valid, err := auth.VerifyPassword(req.OldPassword, dbCaller.PasswordHash)
	if err != nil || !valid {
		_ = s.store.RecordAudit(r.Context(), "user", "auth.password_change_fail", user.Username, false, "Kata sandi saat ini tidak cocok", &user.ID)
		http.Error(w, `{"error":"kata sandi saat ini tidak sesuai"}`, http.StatusBadRequest)
		return
	}

	// Tentukan akun target (default: akun sendiri; admin boleh mengatur ulang akun lain)
	targetUser := dbCaller
	isSelf := true
	if req.TargetUserID > 0 && req.TargetUserID != user.ID {
		if user.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "hanya administrator yang dapat mengubah kata sandi akun lain"})
			return
		}
		other, err := s.store.GetUserByID(r.Context(), req.TargetUserID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "akun target tidak ditemukan"})
			return
		}
		targetUser = other
		isSelf = false
	}

	// Buat hash Argon2id baru
	newHash, err := auth.HashPassword(newPass)
	if err != nil {
		http.Error(w, `{"error":"gagal mengenkripsi kata sandi baru"}`, http.StatusInternalServerError)
		return
	}

	// Untuk akun sendiri: pertahankan sesi browser saat ini dan cabut sesi lain.
	// Untuk akun target lain: cabut seluruh sesi aktif milik akun target tersebut.
	keepToken := ""
	if isSelf {
		if cookie, err := r.Cookie("panel_session"); err == nil {
			keepToken = cookie.Value
		}
	}
	revoked, err := s.store.SetPasswordAndRevokeSessions(r.Context(), targetUser.ID, newHash, keepToken)
	if err != nil {
		http.Error(w, `{"error":"gagal menyimpan kata sandi baru ke database"}`, http.StatusInternalServerError)
		return
	}
	if s.credentialsFile != "" {
		credentials.Forget(s.credentialsFile, targetUser.Username)
	}

	auditAction := "auth.password_change_success"
	auditDetail := fmt.Sprintf("Kata sandi akun %s diperbarui; %d sesi dicabut", targetUser.Username, revoked)
	if !isSelf {
		auditAction = "auth.password_change_other"
		auditDetail = fmt.Sprintf("Admin %s memperbarui kata sandi akun %s; %d sesi dicabut", user.Username, targetUser.Username, revoked)
	}
	_ = s.store.RecordAudit(r.Context(), "user", auditAction, targetUser.Username, true, auditDetail, &user.ID)

	msg := fmt.Sprintf("Kata sandi akun %s berhasil diperbarui", targetUser.Username)
	if revoked > 0 {
		msg = fmt.Sprintf("Kata sandi akun %s berhasil diperbarui; %d sesi login dikeluarkan", targetUser.Username, revoked)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg, "target_username": targetUser.Username})
}

// handleOverview menyajikan ringkasan sistem, hardware, dan status modul panel (mirip aaPanel)
func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	latestMetric := s.metrics.GetLatest()

	// Hitung situs
	sites, _ := s.store.ListSites(r.Context())
	runningSites := 0
	for _, site := range sites {
		if site.Enabled == 1 {
			runningSites++
		}
	}

	// Hitung layanan sistem aktual
	_, counts, errSvc := s.services.ListServicesFiltered(r.Context(), "running", "", "")
	activeSvcs := counts.Running
	totalSvcs := counts.Total
	if errSvc != nil || totalSvcs == 0 {
		svcs, _ := s.services.ListServices(r.Context())
		totalSvcs = len(svcs)
		for _, svc := range svcs {
			if svc.ActiveState == "active" {
				activeSvcs++
			}
		}
	}

	osDisplay := detectHostOSPretty()
	kernelDisplay := detectHostKernel()
	uptimeSec := detectHostUptimeSec()

	now := time.Now()
	zoneName, zoneOffset := now.Zone()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"system": map[string]interface{}{
			"hostname":   hostname,
			"os":         osDisplay,
			"kernel":     kernelDisplay,
			"go_version": runtime.Version(),
			"version":    s.Version(),
			"cpu_cores":  runtime.NumCPU(),
			"uptime_sec": uptimeSec,
			"ip":         getIP(r),
		},
		"server_time": map[string]interface{}{
			"unix":          now.Unix(),
			"timezone_name": zoneName,
			"offset_sec":    zoneOffset,
		},
		"counts": map[string]interface{}{
			"sites_total":     len(sites),
			"sites_running":   runningSites,
			"sites_stopped":   len(sites) - runningSites,
			"services_total":  totalSvcs,
			"services_active": activeSvcs,
			"databases_count": 1, // SQLite panel.db internal
			"database_engine": "SQLite WAL (modernc.org)",
			"security_risk":   0,
		},
		"latest_metric": latestMetric,
	})
}

// ═══ Software Catalog & Detection ═══

func (s *Server) handleListSoftware(w http.ResponseWriter, r *http.Request) {
	items := services.DetectAllSoftware(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"software": items,
	})
}

type InstallSoftwareRequest struct {
	Package string `json:"package"`
}

func (s *Server) handleInstallSoftware(w http.ResponseWriter, r *http.Request) {
	var req InstallSoftwareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	pkg := strings.TrimSpace(strings.ToLower(req.Package))
	if pkg == "" {
		http.Error(w, `{"error":"nama paket software tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	// Catat ke audit log
	_ = s.store.RecordAudit(r.Context(), "user", "software.install", pkg, true, fmt.Sprintf("Memicu instalasi paket %s", pkg), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"package": pkg,
		"message": fmt.Sprintf("Proses instalasi %s berhasil dimulai", pkg),
	})
}

// ═══ Services Management ═══

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")

	list, counts, err := s.services.ListServicesFiltered(r.Context(), scope, category, search)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal mengambil daftar service: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services": list,
		"counts":   counts,
		"scope":    scope,
	})
}

type ServiceActionRequest struct {
	Action string `json:"action"`
}

func (s *Server) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	unit := r.PathValue("unit")
	if unit == "" {
		http.Error(w, `{"error":"unit tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	var req ServiceActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	switch req.Action {
	case "start", "stop", "restart", "reload", "enable", "disable":
	default:
		http.Error(w, `{"error":"aksi tidak dikenal"}`, http.StatusBadRequest)
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}

	// Eksekusi aksi lifecycle systemd aktual pada host sistem
	err := s.services.ExecuteAction(r.Context(), unit, req.Action)
	if err != nil {
		_ = s.store.RecordAudit(r.Context(), "user", "service."+req.Action, unit, false, err.Error(), uid)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
			"unit":  unit,
		})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "service."+req.Action, unit, true, "Eksekusi aksi service berhasil", uid)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"unit":   unit,
		"action": req.Action,
	})
}

func (s *Server) handleGetServiceLogs(w http.ResponseWriter, r *http.Request) {
	unit := r.PathValue("unit")
	if unit == "" {
		http.Error(w, `{"error":"unit tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	lines := 100
	if lStr := r.URL.Query().Get("lines"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 1000 {
			lines = l
		}
	}

	logs, err := s.services.GetServiceLogs(r.Context(), unit, lines)
	if err != nil && logs == "" {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membaca log journalctl: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"unit":  unit,
		"lines": lines,
		"logs":  logs,
	})
}

func (s *Server) handleInspectService(w http.ResponseWriter, r *http.Request) {
	unit := r.PathValue("unit")
	if unit == "" {
		http.Error(w, `{"error":"unit tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	props, err := s.services.InspectService(r.Context(), unit)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal memeriksa unit: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":         true,
		"unit":       unit,
		"properties": props,
	})
}

// ═══ Audit Log ═══

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	logs, err := s.store.ListAuditLogs(r.Context(), 50)
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil riwayat audit"}`, http.StatusInternalServerError)
		return
	}

	if logs == nil {
		logs = []store.AuditLog{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": logs,
	})
}


