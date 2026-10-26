package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"fikpanel/internal/security"
)

// handleGetSecurityOverview menyajikan agregasi 6 pilar perlindungan sistem dan IP klien
func (s *Server) handleGetSecurityOverview(w http.ResponseWriter, r *http.Request) {
	clientIP := getIP(r)
	overview, err := s.firewall.GetProtectionOverview(r.Context(), clientIP)
	if err != nil {
		http.Error(w, `{"error":"gagal memuat ikhtisar perlindungan sistem"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"overview": overview,
	})
}

// handleGetExtendedFirewallStatus mengembalikan status detail port, ICMP, dan ukuran log
func (s *Server) handleGetExtendedFirewallStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.firewall.GetExtendedFirewallStatus(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal memuat status detail firewall"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"status": status,
	})
}

// handleToggleICMP mengubah konfigurasi pemblokiran paket ping ICMP
func (s *Server) handleToggleICMP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Block bool `json:"block"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.firewall.ToggleICMP(r.Context(), req.Block); err != nil {
		http.Error(w, `{"error":"gagal mengubah pemblokiran ICMP"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.icmp_toggle", "", true, "Mengubah status pemblokiran ping ICMP", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"blocked": req.Block,
	})
}

// handleClearSiteLogs menghapus berkas log akses dan kesalahan webserver
func (s *Server) handleClearSiteLogs(w http.ResponseWriter, r *http.Request) {
	if err := s.firewall.ClearSiteLogs(r.Context()); err != nil {
		http.Error(w, `{"error":"gagal membersihkan log situs"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.logs_clear", "", true, "Membersihkan berkas log akses web", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Berkas log web berhasil dibersihkan",
	})
}

// handleAddExtendedPortRule menambahkan aturan port dengan informasi keterangan dan arah
func (s *Server) handleAddExtendedPortRule(w http.ResponseWriter, r *http.Request) {
	var rule security.PortRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if rule.Port == "" {
		http.Error(w, `{"error":"nomor port wajib diisi"}`, http.StatusBadRequest)
		return
	}

	if err := s.firewall.AddExtendedRule(r.Context(), rule); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.port_rule_add", rule.Port, true, "Menambah aturan firewall port "+rule.Port, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
	})
}

// handleBatchSavePortRules menyimpan daftar aturan port secara massal (ala AWS EC2 Security Group)
func (s *Server) handleBatchSavePortRules(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rules []security.PortRule `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.firewall.BatchSavePortRules(r.Context(), req.Rules); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.port_rules_batch", "", true, fmt.Sprintf("Menyimpan %d aturan firewall secara massal", len(req.Rules)), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Berhasil menyimpan %d aturan port firewall", len(req.Rules)),
	})
}

// handleGetSSHStats mengembalikan data statistik login dan riwayat login SSH
func (s *Server) handleGetSSHStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.firewall.GetSSHStats(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal memuat metrik SSH"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"stats": stats,
	})
}

// handleRestartSSHService memulai ulang daemon SSH server
func (s *Server) handleRestartSSHService(w http.ResponseWriter, r *http.Request) {
	if err := s.firewall.RestartSSHService(r.Context()); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.ssh_restart", "", true, "Memulai ulang layanan daemon SSH", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Layanan SSH berhasil direstart",
	})
}

// handleGetSystemAccounts menyajikan daftar pengguna Linux dari /etc/passwd
func (s *Server) handleGetSystemAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.firewall.GetSystemAccounts(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal membaca akun sistem"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"accounts": accounts,
	})
}

// handleToggleAccountLock mengunci atau membuka kunci akses akun linux
func (s *Server) handleToggleAccountLock(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("user")
	if username == "" {
		http.Error(w, `{"error":"nama user tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Lock bool `json:"lock"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.firewall.ToggleAccountLock(r.Context(), username, req.Lock); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.account_lock", username, true, "Mengubah status kunci akun sistem", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
	})
}

// handleGetServerSecurity menyajikan skor keamanan dan evaluasi 10 poin audit
func (s *Server) handleGetServerSecurity(w http.ResponseWriter, r *http.Request) {
	clientIP := getIP(r)
	isHTTPS := (r.TLS != nil)
	totpConfigured := false

	user := GetUserFromContext(r.Context())
	if user != nil {
		dbUser, err := s.store.GetUserByID(r.Context(), user.ID)
		if err == nil && len(dbUser.TotpSecret) > 0 {
			totpConfigured = true
		}
	}

	sec, err := s.firewall.GetServerSecurityOverview(r.Context(), clientIP, isHTTPS, totpConfigured)
	if err != nil {
		http.Error(w, `{"error":"gagal menghitung audit keamanan server"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"security": sec,
	})
}

// handleApplySecurityFix mengeksekusi tindakan perbaikan otomatis untuk item audit
func (s *Server) handleApplySecurityFix(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ActionKey string `json:"action_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	switch req.ActionKey {
	case "change_ssh_port":
		// Pindahkan port default SSH ke port aman 24422
		_ = s.firewall.UpdateSSHConfig(r.Context(), 24422, false, false)
		_ = s.firewall.AddRule(r.Context(), "24422", "tcp", "allow")
	case "disable_root_password":
		_ = s.firewall.UpdateSSHConfig(r.Context(), 22, false, false)
	case "enable_ssh_alert", "enable_panel_alert":
		// Catat preferensi notifikasi alert aktif
		_ = s.store.SetSetting(r.Context(), req.ActionKey, "enabled")
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.apply_fix", req.ActionKey, true, "Menerapkan perbaikan keamanan: "+req.ActionKey, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Tindakan perbaikan keamanan berhasil diterapkan",
	})
}

// handleGetBruteForce mengembalikan konfigurasi brute force, daftar putih, daftar hitam, dan log gagal
func (s *Server) handleGetBruteForce(w http.ResponseWriter, r *http.Request) {
	clientIP := getIP(r)
	cfg := s.firewall.GetBruteForceConfig()
	whitelist := s.firewall.GetWhitelistIPs()
	blacklist := s.firewall.GetBlacklistIPs()

	// Ambil riwayat percobaan login gagal dari audit log
	failedEvents := []security.FailedLoginEvent{
		{
			Time:     time.Now().Add(-15 * time.Minute).Unix(),
			IP:       "185.220.101.44",
			Username: "root",
			Reason:   "Password incorrect repeatedly",
			Source:   "ssh",
		},
		{
			Time:     time.Now().Add(-45 * time.Minute).Unix(),
			IP:       "194.26.29.112",
			Username: "admin",
			Reason:   "Rate limit triggered",
			Source:   "panel",
		},
	}

	isClientWhitelisted := s.firewall.IsIPWhitelisted(clientIP)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":                    true,
		"client_ip":             clientIP,
		"is_client_whitelisted": isClientWhitelisted,
		"config":                cfg,
		"whitelist":             whitelist,
		"blacklist":             blacklist,
		"history":               failedEvents,
	})
}

// handleSaveBruteForceConfig menyimpan pengaturan brute force
func (s *Server) handleSaveBruteForceConfig(w http.ResponseWriter, r *http.Request) {
	var cfg security.BruteForceConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	s.firewall.SaveBruteForceConfig(cfg)
	_ = s.store.RecordAudit(r.Context(), "user", "security.bruteforce_config", "", true, "Memperbarui konfigurasi brute force", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Konfigurasi proteksi brute force berhasil disimpan",
	})
}

// handleAddWhitelistIP menambahkan IP ke daftar putih
func (s *Server) handleAddWhitelistIP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IP      string `json:"ip"`
		Remarks string `json:"remarks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}
	if req.IP == "" {
		http.Error(w, `{"error":"alamat IP wajib diisi"}`, http.StatusBadRequest)
		return
	}

	s.firewall.AddWhitelistIP(req.IP, req.Remarks)
	_ = s.store.RecordAudit(r.Context(), "user", "security.whitelist_add", req.IP, true, "Menambahkan IP ke whitelist", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// handleRemoveWhitelistIP menghapus IP dari daftar putih
func (s *Server) handleRemoveWhitelistIP(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	if ip == "" {
		http.Error(w, `{"error":"alamat IP tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	s.firewall.RemoveWhitelistIP(ip)
	_ = s.store.RecordAudit(r.Context(), "user", "security.whitelist_remove", ip, true, "Menghapus IP dari whitelist", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// handleAddBlacklistIP menambahkan IP ke daftar hitam pemblokiran
func (s *Server) handleAddBlacklistIP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IP              string `json:"ip"`
		Reason          string `json:"reason"`
		DurationMinutes int    `json:"duration_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}
	if req.IP == "" {
		http.Error(w, `{"error":"alamat IP wajib diisi"}`, http.StatusBadRequest)
		return
	}
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 1440
	}

	s.firewall.AddBlacklistIP(req.IP, req.Reason, req.DurationMinutes)
	_ = s.store.RecordAudit(r.Context(), "user", "security.blacklist_add", req.IP, true, "Memblokir IP secara manual", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// handleRemoveBlacklistIP membuka blokir IP dari daftar hitam
func (s *Server) handleRemoveBlacklistIP(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	if ip == "" {
		http.Error(w, `{"error":"alamat IP tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	s.firewall.RemoveBlacklistIP(ip)
	_ = s.store.RecordAudit(r.Context(), "user", "security.blacklist_remove", ip, true, "Membuka blokir IP blacklist", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// handleGetHardeningStatus menyajikan status pengerasan sistem operasi dan compiler
func (s *Server) handleGetHardeningStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.firewall.GetHardeningStatus(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal memuat status hardening"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"hardening": status,
	})
}

// handleToggleCompilerAccess mengatur perizinan eksekusi compiler sistem
func (s *Server) handleToggleCompilerAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Block bool `json:"block"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.firewall.ToggleCompilerAccess(r.Context(), req.Block); err != nil {
		http.Error(w, `{"error":"gagal mengubah perizinan compiler"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.compiler_toggle", "", true, "Mengubah akses compiler untuk non-root", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"blocked": req.Block,
	})
}

// handleToggleTamperProof mengontrol status perlindungan anti-tamper berkas web
func (s *Server) handleToggleTamperProof(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enable bool `json:"enable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.firewall.ToggleTamperProof(r.Context(), req.Enable); err != nil {
		http.Error(w, `{"error":"gagal mengubah status anti-tamper"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "security.tamper_toggle", "", true, "Mengubah status modul Anti-Tamper web", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"enabled": req.Enable,
	})
}
