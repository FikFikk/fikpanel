package server

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (s *Server) handleListSystemProcesses(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 30
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	procs, err := s.systemMgr.ListProcesses(r.Context(), limit)
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil daftar proses"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "processes": procs})
}

func (s *Server) handleKillSystemProcess(w http.ResponseWriter, r *http.Request) {
	pidStr := r.PathValue("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		http.Error(w, `{"error":"PID tidak valid"}`, http.StatusBadRequest)
		return
	}

	force := r.URL.Query().Get("force") == "true"
	if err := s.systemMgr.KillProcess(r.Context(), pid, force); err != nil {
		http.Error(w, `{"error":"gagal menghentikan proses"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "pid": pid})
}

func (s *Server) handleListListeningPorts(w http.ResponseWriter, r *http.Request) {
	ports, err := s.systemMgr.ListListeningPorts(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil daftar port listening"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "ports": ports})
}

func (s *Server) handleScanSystemJunk(w http.ResponseWriter, r *http.Request) {
	result, err := s.systemMgr.ScanJunk(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal memindai file sampah"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "scan": result})
}

func (s *Server) handleCleanSystemJunk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CleanApt   bool `json:"clean_apt"`
		CleanLogs  bool `json:"clean_logs"`
		CleanTmp   bool `json:"clean_tmp"`
		DropCaches bool `json:"drop_caches"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := s.systemMgr.CleanJunk(r.Context(), req.CleanApt, req.CleanLogs, req.CleanTmp, req.DropCaches); err != nil {
		http.Error(w, `{"error":"gagal melakukan pembersihan sistem"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Pembersihan sistem berhasil diselesaikan"})
}
