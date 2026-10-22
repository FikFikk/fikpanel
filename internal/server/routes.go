package server

import (
	"encoding/json"
	"net/http"
	"time"

	"fikpanel/internal/auth"
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
		http.Error(w, `{"error":"kredensial tidak valid"}`, http.StatusUnauthorized)
		return
	}

	valid, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !valid {
		s.limiter.RecordFailure(ip)
		http.Error(w, `{"error":"kredensial tidak valid"}`, http.StatusUnauthorized)
		return
	}

	// Reset percobaan gagal setelah autentikasi berhasil
	s.limiter.Reset(ip)
	_ = s.store.UpdateLastLogin(r.Context(), user.ID)

	// Buat token sesi aktif selama 24 jam
	token, err := s.store.CreateSession(r.Context(), user.ID, ip, r.UserAgent(), 24*time.Hour)
	if err != nil {
		http.Error(w, `{"error":"gagal membuat sesi login"}`, http.StatusInternalServerError)
		return
	}

	// Simpan cookie sesi
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
	if cookie, err := r.Cookie("panel_session"); err == nil && cookie.Value != "" {
		_ = s.store.DeleteSession(r.Context(), cookie.Value)
	}

	// Bersihkan cookie sesi di browser
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

	// Validasi aksi yang diizinkan
	switch req.Action {
	case "start", "stop", "restart", "reload":
		// Aksi valid
	default:
		http.Error(w, `{"error":"aksi tidak dikenal"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"unit":   unit,
		"action": req.Action,
	})
}
