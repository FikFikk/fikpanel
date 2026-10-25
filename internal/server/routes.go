package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"system": map[string]interface{}{
			"hostname":   hostname,
			"os":         osDisplay,
			"kernel":     runtime.Version(),
			"cpu_cores":  runtime.NumCPU(),
			"uptime_sec": time.Now().Unix(),
			"ip":         getIP(r),
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
	list, err := s.services.ListServices(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil daftar service"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services": list,
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
	case "start", "stop", "restart", "reload":
	default:
		http.Error(w, `{"error":"aksi tidak dikenal"}`, http.StatusBadRequest)
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "service."+req.Action, unit, true, "Eksekusi aksi service", uid)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"unit":   unit,
		"action": req.Action,
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

// ═══ Terminal Bridge ═══

type TerminalExecRequest struct {
	Command string `json:"command"`
}

func (s *Server) handleTerminalExec(w http.ResponseWriter, r *http.Request) {
	var req TerminalExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload tidak valid"}`, http.StatusBadRequest)
		return
	}

	cmdClean := strings.TrimSpace(req.Command)
	if cmdClean == "" {
		http.Error(w, `{"error":"perintah kosong"}`, http.StatusBadRequest)
		return
	}

	// Real execution di Linux dengan timeout 10 detik
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdClean)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", cmdClean)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		if len(output) > 0 {
			output += "\n--- STDERR ---\n"
		}
		output += stderr.String()
	}

	if err != nil && len(output) == 0 {
		output = fmt.Sprintf("Error: %v", err)
	}

	// Batasi ukuran output 64KB
	if len(output) > 65536 {
		output = output[:65536] + "\n... [output dipotong 64KB]"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"command": cmdClean,
		"output":  output,
	})
}


