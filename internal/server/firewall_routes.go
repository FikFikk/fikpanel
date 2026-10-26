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

func (s *Server) handleGetSSHConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.firewall.GetSSHConfig(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal memuat konfigurasi SSH"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "ssh": cfg})
}

func (s *Server) handleUpdateSSHConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port            int  `json:"port"`
		PasswordAuth    bool `json:"password_auth"`
		PermitRootLogin bool `json:"permit_root_login"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.firewall.UpdateSSHConfig(r.Context(), req.Port, req.PasswordAuth, req.PermitRootLogin); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.ssh_update", strconv.Itoa(req.Port), true, "Memperbarui konfigurasi SSH Hardening", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Konfigurasi SSH berhasil disimpan dan di-reload"})
}

func (s *Server) handleGetFail2BanStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.firewall.GetFail2BanStatus(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal mengecek Fail2Ban"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "fail2ban": st})
}

func (s *Server) handleUnbanFail2BanIP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Jail string `json:"jail"`
		IP   string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.firewall.UnbanIP(r.Context(), req.Jail, req.IP); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.unban_ip", req.IP, true, "Membuka blokir IP dari Fail2Ban jail "+req.Jail, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (s *Server) handleRunSecurityScan(w http.ResponseWriter, r *http.Request) {
	report, err := s.firewall.RunSecurityScan(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal menjalankan pemindaian keamanan"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.scan", strconv.Itoa(report.Score), true, "Menjalankan pemindaian keamanan server menyeluruh", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "report": report})
}
