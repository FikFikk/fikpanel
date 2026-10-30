package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"fikpanel/internal/node"
)

func (s *Server) handleGetNodeStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.pm2.GetStatus(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal mengecek status Node/PM2"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"status": status,
	})
}

func (s *Server) handleListNodeProcesses(w http.ResponseWriter, r *http.Request) {
	processes, err := s.pm2.ListProcesses(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil daftar proses PM2"}`, http.StatusInternalServerError)
		return
	}

	status, _ := s.pm2.GetStatus(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"status":    status,
		"processes": processes,
	})
}

func (s *Server) handleStartNodeProcess(w http.ResponseWriter, r *http.Request) {
	var req node.StartAppParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Script == "" {
		http.Error(w, `{"error":"nama aplikasi dan berkas script wajib diisi"}`, http.StatusBadRequest)
		return
	}

	if err := s.pm2.StartApp(r.Context(), req); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "node.start", req.Name, true, "Menjalankan aplikasi Node.js PM2 baru: "+req.Name, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Aplikasi PM2 berhasil dijalankan",
	})
}

func (s *Server) handleControlNodeProcess(w http.ResponseWriter, r *http.Request) {
	idOrName := r.PathValue("id")
	action := r.PathValue("action")

	if idOrName == "" || action == "" {
		http.Error(w, `{"error":"ID proses dan aksi wajib diisi"}`, http.StatusBadRequest)
		return
	}

	// Baca user pemilik proses jika dikirim melalui query param atau body JSON
	ownerUser := strings.TrimSpace(r.URL.Query().Get("user"))
	if ownerUser == "" && r.Body != nil {
		var body struct {
			User string `json:"user"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		ownerUser = strings.TrimSpace(body.User)
	}

	if err := s.pm2.Action(r.Context(), idOrName, action, ownerUser); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "node."+action, idOrName, true, "Eksekusi aksi PM2 "+action+" pada target: "+idOrName, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"target": idOrName,
		"action": action,
	})
}

func (s *Server) handleGetNodeProcessLogs(w http.ResponseWriter, r *http.Request) {
	idOrName := r.PathValue("id")
	if idOrName == "" {
		http.Error(w, `{"error":"ID atau nama proses wajib diisi"}`, http.StatusBadRequest)
		return
	}

	lines := 60
	if lStr := r.URL.Query().Get("lines"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			lines = l
		}
	}

	ownerUser := strings.TrimSpace(r.URL.Query().Get("user"))

	logs, err := s.pm2.GetLogs(r.Context(), idOrName, lines, ownerUser)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"target": idOrName,
		"logs":   logs,
	})
}
