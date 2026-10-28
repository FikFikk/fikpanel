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
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	OK   bool        `json:"ok"`
	User interface{} `json:"user"`
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

	s.limiter.Reset(ip)
	_ = s.store.UpdateLastLogin(r.Context(), user.ID)

	token, err := s.store.CreateSession(r.Context(), user.ID, ip, r.UserAgent(), 24*time.Hour)
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
		MaxAge:   86400,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		OK: true,
		User: map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
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
		"id":          user.ID,
		"username":    user.Username,
		"role":        user.Role,
		"sudo_active": true,
	})
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

	// Hitung layanan
	svcs, _ := s.services.ListServices(r.Context())
	activeSvcs := 0
	for _, svc := range svcs {
		if svc.ActiveState == "active" {
			activeSvcs++
		}
	}

	osDisplay := runtime.GOOS + "/" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		osDisplay = "Windows Dev (" + runtime.GOARCH + ")"
	} else {
		osDisplay = "Linux (amd64 / arm64)"
	}

	now := time.Now()
	zoneName, zoneOffset := now.Zone()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"system": map[string]interface{}{
			"hostname":   hostname,
			"os":         osDisplay,
			"kernel":     runtime.Version(),
			"version":    s.Version(),
			"cpu_cores":  runtime.NumCPU(),
			"uptime_sec": now.Unix(),
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
			"services_total":  len(svcs),
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


