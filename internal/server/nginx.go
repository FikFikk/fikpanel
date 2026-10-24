package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"fikpanel/internal/nginx"
)

// Batas ukuran body request untuk endpoint nginx (config maks 2 MB + overhead JSON)
const nginxMaxBody = 3 << 20

// writeJSON menulis respons JSON dengan status code tertentu
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeNginxError memetakan error domain nginx ke HTTP status yang sesuai
func writeNginxError(w http.ResponseWriter, err error) {
	var vErr *nginx.ValidationError
	var cErr *nginx.ConflictError
	switch {
	case errors.As(err, &vErr):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": vErr.Msg})
	case errors.As(err, &cErr):
		writeJSON(w, http.StatusConflict, map[string]string{"error": cErr.Msg})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

// decodeNginxBody membaca body JSON dengan batas ukuran
func decodeNginxBody(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, nginxMaxBody)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return false
	}
	return true
}

// auditNginx mencatat aksi user terkait nginx ke audit log
func (s *Server) auditNginx(r *http.Request, action, target string, ok bool, detail string) {
	var uid *int64
	if u := GetUserFromContext(r.Context()); u != nil {
		uid = &u.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", action, target, ok, detail, uid)
}

func (s *Server) handleNginxStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.nginx.Status(r.Context())
	if err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleNginxService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	err := s.nginx.ServiceAction(r.Context(), req.Action)
	detail := "Aksi layanan nginx: " + req.Action
	if err != nil {
		detail = err.Error()
	}
	s.auditNginx(r, "nginx."+req.Action, "nginx.service", err == nil, detail)
	if err != nil {
		writeNginxError(w, err)
		return
	}
	st, err := s.nginx.Status(r.Context())
	if err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "status": st})
}

func (s *Server) handleNginxGetSettings(w http.ResponseWriter, r *http.Request) {
	prefs, err := s.nginx.GetSettings(r.Context())
	if err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleNginxSaveSettings(w http.ResponseWriter, r *http.Request) {
	var req nginx.Settings
	if !decodeNginxBody(w, r, &req) {
		return
	}
	if err := s.nginx.SaveSettings(r.Context(), req); err != nil {
		writeNginxError(w, err)
		return
	}
	s.auditNginx(r, "nginx.settings", "nginx.service", true,
		fmt.Sprintf("daemon=%t alert=%t", req.Daemon, req.Alert))
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "settings": req})
}

func (s *Server) handleNginxGetConfig(w http.ResponseWriter, r *http.Request) {
	content, err := s.nginx.ReadConfig()
	if err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":    s.nginx.Paths().ConfFile,
		"content": content,
	})
}

func (s *Server) handleNginxSaveConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	backup, err := s.nginx.SaveConfig(r.Context(), req.Content)
	detail := "Konfigurasi utama disimpan, backup: " + backup
	if err != nil {
		detail = err.Error()
	}
	s.auditNginx(r, "nginx.config_save", s.nginx.Paths().ConfFile, err == nil, detail)
	if err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "backup": backup})
}

func (s *Server) handleNginxVersions(w http.ResponseWriter, r *http.Request) {
	info, err := s.nginx.Versions(r.Context())
	if err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleNginxSwitchVersion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	var uid *int64
	if u := GetUserFromContext(r.Context()); u != nil {
		uid = &u.ID
	}
	if err := s.nginx.SwitchVersion(r.Context(), req.Version, uid); err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"ok": true, "target": req.Version})
}

func (s *Server) handleNginxLoad(w http.ResponseWriter, r *http.Request) {
	ls, err := s.nginx.Load(r.Context())
	if err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ls)
}

func (s *Server) handleNginxGetOptimize(w http.ResponseWriter, r *http.Request) {
	opt, err := s.nginx.ReadOptimization()
	if err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, opt)
}

func (s *Server) handleNginxSaveOptimize(w http.ResponseWriter, r *http.Request) {
	var req nginx.Optimization
	if !decodeNginxBody(w, r, &req) {
		return
	}
	backup, err := s.nginx.SaveOptimization(r.Context(), req)
	detail := "Optimasi directive diterapkan, backup: " + backup
	if err != nil {
		detail = err.Error()
	}
	s.auditNginx(r, "nginx.optimize", s.nginx.Paths().ConfFile, err == nil, detail)
	if err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "backup": backup})
}

func (s *Server) handleNginxErrorLog(w http.ResponseWriter, r *http.Request) {
	lines, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	logData, err := s.nginx.TailErrorLog(lines)
	if err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logData)
}

func (s *Server) handleNginxClearErrorLog(w http.ResponseWriter, r *http.Request) {
	err := s.nginx.ClearErrorLog()
	detail := "Log error dikosongkan"
	if err != nil {
		detail = err.Error()
	}
	s.auditNginx(r, "nginx.error_log_clear", s.nginx.Paths().ErrorLog, err == nil, detail)
	if err != nil {
		writeNginxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
