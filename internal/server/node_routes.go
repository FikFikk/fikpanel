package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleListNodeProcesses(w http.ResponseWriter, r *http.Request) {
	processes, err := s.pm2.ListProcesses(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil daftar proses PM2"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"processes": processes,
	})
}

func (s *Server) handleControlNodeProcess(w http.ResponseWriter, r *http.Request) {
	idOrName := r.PathValue("id")
	action := r.PathValue("action")

	if idOrName == "" || action == "" {
		http.Error(w, `{"error":"ID proses dan aksi wajib diisi"}`, http.StatusBadRequest)
		return
	}

	if err := s.pm2.Action(r.Context(), idOrName, action); err != nil {
		http.Error(w, `{"error":"gagal mengeksekusi aksi PM2"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"target": idOrName,
		"action": action,
	})
}
