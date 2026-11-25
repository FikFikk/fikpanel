package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"fikpanel/internal/security"
)

// handleGetFirewallStatus mengembalikan status dasar UFW dan daftar aturan bernomor
func (s *Server) handleGetFirewallStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.firewall.GetStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memeriksa status firewall: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"status": status,
	})
}

// handleToggleFirewall menyalakan atau mematikan firewall UFW dengan proteksi lockout port SSH & panel
func (s *Server) handleToggleFirewall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}

	if err := s.firewall.ToggleFirewall(r.Context(), req.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	actionDesc := "Menonaktifkan firewall UFW"
	if req.Enabled {
		actionDesc = "Mengaktifkan firewall UFW"
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.firewall_toggle", strconv.FormatBool(req.Enabled), true, actionDesc, s.callerUserID(r))

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"enabled": req.Enabled,
	})
}

// handleSetFirewallDefaultPolicy memperbarui kebijakan bawaan incoming & outgoing UFW
func (s *Server) handleSetFirewallDefaultPolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Incoming string `json:"incoming"`
		Outgoing string `json:"outgoing"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}

	if err := s.firewall.SetDefaultPolicy(r.Context(), req.Incoming, req.Outgoing); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.firewall_policy", req.Incoming+"/"+req.Outgoing, true, "Memperbarui kebijakan default UFW", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"incoming": req.Incoming,
		"outgoing": req.Outgoing,
	})
}

// handleAddFirewallRule menambahkan aturan port sederhana (kompatibilitas endpoint dasar)
func (s *Server) handleAddFirewallRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port   string `json:"port"`
		Proto  string `json:"proto"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}

	if err := s.firewall.AddPortRule(r.Context(), security.AddFirewallRuleInput{
		Port:      req.Port,
		Protocol:  req.Proto,
		Action:    req.Action,
		Direction: "in",
	}); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.port_rule_add", req.Port, true, "Menambahkan aturan firewall", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleDeleteFirewallRule menghapus satu aturan UFW berdasarkan nomornya
func (s *Server) handleDeleteFirewallRule(w http.ResponseWriter, r *http.Request) {
	numStr := r.PathValue("number")
	num, err := strconv.Atoi(numStr)
	if err != nil || num <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nomor aturan firewall tidak valid"})
		return
	}

	force := strings.EqualFold(r.URL.Query().Get("force"), "true") || r.URL.Query().Get("force") == "1"
	if err := s.firewall.DeleteRule(r.Context(), num, force); err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "proteksi lockout") {
			statusCode = http.StatusConflict
		}
		writeJSON(w, statusCode, map[string]string{"error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.port_rule_delete", numStr, true, "Menghapus aturan firewall #"+numStr, s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleBatchDeleteFirewallRules menghapus beberapa aturan UFW secara aman dari nomor terbesar ke terkecil
func (s *Server) handleBatchDeleteFirewallRules(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Numbers []int `json:"numbers"`
		Force   bool  `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if len(req.Numbers) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pilih minimal satu aturan untuk dihapus"})
		return
	}

	if err := s.firewall.DeleteRules(r.Context(), req.Numbers, req.Force); err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "proteksi lockout") {
			statusCode = http.StatusConflict
		}
		writeJSON(w, statusCode, map[string]string{"error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.port_rule_batch_delete", strconv.Itoa(len(req.Numbers)), true, "Menghapus beberapa aturan firewall sekaligus", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": len(req.Numbers)})
}

// handleGetSSHConfig membaca konfigurasi OpenSSH efektif beserta status cloud-init override
func (s *Server) handleGetSSHConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.firewall.GetSSHConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memuat konfigurasi SSH: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ssh": cfg})
}

// handleUpdateSSHConfig memperbarui konfigurasi OpenSSH dengan validasi sshd -t dan lockout guard
func (s *Server) handleUpdateSSHConfig(w http.ResponseWriter, r *http.Request) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}

	// Bangun SSHConfigUpdate sambil menjaga kompatibilitas payload lama (permit_root_login bool atau string)
	curCfg, _ := s.firewall.GetSSHConfig(r.Context())
	req := security.SSHConfigUpdate{
		Port:            22,
		PasswordAuth:    true,
		PubkeyAuth:      true,
		PermitRootLogin: "prohibit-password",
		MaxAuthTries:    6,
	}
	if curCfg != nil {
		req.Port = curCfg.Port
		req.PasswordAuth = curCfg.PasswordAuth
		req.PubkeyAuth = curCfg.PubkeyAuth
		req.PermitRootLogin = curCfg.PermitRootLogin
		req.MaxAuthTries = curCfg.MaxAuthTries
	}

	if v, ok := raw["port"].(float64); ok {
		req.Port = int(v)
	}
	if v, ok := raw["password_auth"].(bool); ok {
		req.PasswordAuth = v
	}
	if v, ok := raw["pubkey_auth"].(bool); ok {
		req.PubkeyAuth = v
	}
	if v, ok := raw["max_auth_tries"].(float64); ok {
		req.MaxAuthTries = int(v)
	}
	if v, ok := raw["force"].(bool); ok {
		req.Force = v
	}
	switch v := raw["permit_root_login"].(type) {
	case string:
		req.PermitRootLogin = v
	case bool:
		if v {
			req.PermitRootLogin = "yes"
		} else {
			req.PermitRootLogin = "no"
		}
	}

	if err := s.firewall.UpdateSSHConfig(r.Context(), req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.ssh_update", strconv.Itoa(req.Port), true, "Memperbarui konfigurasi SSH Hardening", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Konfigurasi SSH berhasil divalidasi (sshd -t), disimpan, dan dimuat ulang",
	})
}

// handleGetFail2BanStatus mengembalikan status instalasi, daemon, dan daftar jail Fail2Ban aktual
func (s *Server) handleGetFail2BanStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.firewall.GetFail2BanStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memeriksa Fail2Ban: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "fail2ban": st})
}

// handleControlFail2BanService menjalankan start, stop, atau restart pada layanan Fail2Ban
func (s *Server) handleControlFail2BanService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.firewall.ControlFail2BanService(r.Context(), req.Action); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.fail2ban_service", req.Action, true, "Kontrol layanan Fail2Ban: "+req.Action, s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "action": req.Action})
}

// handleBanFail2BanIP memblokir alamat IP secara manual pada jail Fail2Ban
func (s *Server) handleBanFail2BanIP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Jail string `json:"jail"`
		IP   string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if strings.TrimSpace(req.IP) == getIP(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tidak dapat memblokir alamat IP Anda sendiri"})
		return
	}
	if err := s.firewall.BanIP(r.Context(), req.Jail, req.IP); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.ban_ip", req.IP, true, "Memblokir IP di Fail2Ban jail "+req.Jail, s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleUnbanFail2BanIP membuka blokir IP dari jail Fail2Ban
func (s *Server) handleUnbanFail2BanIP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Jail string `json:"jail"`
		IP   string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.firewall.UnbanIP(r.Context(), req.Jail, req.IP); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.unban_ip", req.IP, true, "Membuka blokir IP dari Fail2Ban jail "+req.Jail, s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleRunSecurityScan menjalankan audit dan pemindaian keamanan server terpadu dan menyimpannya ke SQLite
func (s *Server) handleRunSecurityScan(w http.ResponseWriter, r *http.Request) {
	opts := s.buildSecurityScanOptions(r)
	report, err := s.firewall.RunSecurityScan(r.Context(), opts)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menjalankan pemindaian keamanan: " + err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.scan", strconv.Itoa(report.Score), true, "Menjalankan audit keamanan server menyeluruh", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "report": report})
}

// handleGetLatestSecurityScan mengambil laporan pemindaian terakhir yang tersimpan di SQLite
func (s *Server) handleGetLatestSecurityScan(w http.ResponseWriter, r *http.Request) {
	report := s.firewall.GetLatestScanReport(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "report": report})
}
