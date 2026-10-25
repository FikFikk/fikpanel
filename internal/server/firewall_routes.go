package server

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (s *Server) handleGetFirewallStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.firewall.GetStatus(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal mengecek status firewall"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"status": status,
	})
}

func (s *Server) handleToggleFirewall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.firewall.ToggleFirewall(r.Context(), req.Enabled); err != nil {
		http.Error(w, `{"error":"gagal mengubah status firewall"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"enabled": req.Enabled,
	})
}

func (s *Server) handleAddFirewallRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port   string `json:"port"`
		Proto  string `json:"proto"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if req.Port == "" {
		http.Error(w, `{"error":"nomor port wajib diisi"}`, http.StatusBadRequest)
		return
	}

	if err := s.firewall.AddRule(r.Context(), req.Port, req.Proto, req.Action); err != nil {
		http.Error(w, `{"error":"gagal menambahkan aturan firewall"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
	})
}

func (s *Server) handleDeleteFirewallRule(w http.ResponseWriter, r *http.Request) {
	numStr := r.PathValue("number")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		http.Error(w, `{"error":"nomor aturan tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.firewall.DeleteRule(r.Context(), num); err != nil {
		http.Error(w, `{"error":"gagal menghapus aturan firewall"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
	})
}
