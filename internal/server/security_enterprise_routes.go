package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/security"
)

// callerUserID mengekstrak pointer ID pengguna aktif dari context request untuk pencatatan audit
func (s *Server) callerUserID(r *http.Request) *int64 {
	if u := GetUserFromContext(r.Context()); u != nil {
		id := u.ID
		return &id
	}
	return nil
}

// getRealWAFSummary membaca status aktif Nginx WAF dan jumlah situs terproteksi langsung dari store & WAF manager
func (s *Server) getRealWAFSummary(ctx context.Context) (enabled bool, sitesCount int) {
	enabled = true
	if val, err := s.store.GetSetting(ctx, "waf.global_enabled"); err == nil && val != "" {
		enabled = val != "false" && val != "0"
	}
	if s.waf != nil {
		if list, err := s.waf.ListSites(ctx); err == nil {
			sitesCount = len(list)
		}
	}
	return enabled, sitesCount
}

// buildSecurityScanOptions mengumpulkan konteks nyata panel, TLS, 2FA, Passkey, WAF, dan situs untuk audit keamanan
func (s *Server) buildSecurityScanOptions(r *http.Request) security.ScanOptions {
	ctx := r.Context()
	totpEnabled := false
	if user := GetUserFromContext(ctx); user != nil {
		if dbUser, err := s.store.GetUserByID(ctx, user.ID); err == nil && len(dbUser.TotpSecret) > 0 {
			totpEnabled = true
		}
	}

	prefs := s.loadPanelPreferences(ctx)
	wafEnabled, _ := s.getRealWAFSummary(ctx)
	bfCfg := s.firewall.GetBruteForceConfig(ctx)

	// Kumpulkan root_path seluruh situs aktif dan hitung SSL yang hampir/sudah kedaluwarsa (<= 14 hari)
	siteRoots := make([]string, 0)
	expiringSSL := 0
	expThreshold := time.Now().Add(14 * 24 * time.Hour).Unix()
	if sitesList, err := s.store.ListSites(ctx); err == nil {
		for _, site := range sitesList {
			if strings.TrimSpace(site.RootPath) != "" {
				siteRoots = append(siteRoots, site.RootPath)
			}
			if site.Enabled == 1 && site.SSLMode != "none" && site.SSLExpires > 0 && site.SSLExpires <= expThreshold {
				expiringSSL++
			}
		}
	}

	return security.ScanOptions{
		IsHTTPS:           s.TLSEnabled() || r.TLS != nil,
		TOTPEnabled:       totpEnabled,
		PasskeyCount:      s.store.CountAllPasskeys(ctx),
		IPAllowlistCount:  len(prefs.IPAllowlist),
		WAFEnabled:        wafEnabled,
		BruteForceEnabled: bfCfg.Enabled,
		SiteRoots:         siteRoots,
		ExpiringSSLCount:  expiringSSL,
		ClientIP:          getIP(r),
	}
}

// handleGetSecurityOverview menyajikan status 6 pilar perlindungan aktual, login terakhir, dan laporan scan terbaru
func (s *Server) handleGetSecurityOverview(w http.ResponseWriter, r *http.Request) {
	clientIP := getIP(r)
	wafEnabled, wafSitesCount := s.getRealWAFSummary(r.Context())
	overview, err := s.firewall.GetProtectionOverview(r.Context(), clientIP, wafEnabled, wafSitesCount)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memuat ikhtisar perlindungan sistem: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"overview": overview,
	})
}

// handleGetExtendedFirewallStatus mengembalikan status lengkap UFW, kebijakan default, soket listening, dan log web
func (s *Server) handleGetExtendedFirewallStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.firewall.GetExtendedFirewallStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memuat status firewall: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"status": status,
	})
}

// handleToggleICMP mengubah konfigurasi pemblokiran paket ping ICMP secara persisten
func (s *Server) handleToggleICMP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Block bool `json:"block"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.firewall.ToggleICMP(r.Context(), req.Block); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.icmp_toggle", strconv.FormatBool(req.Block), true, "Mengubah pemblokiran ping ICMP", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "blocked": req.Block})
}

// handleClearSiteLogs mengosongkan berkas log akses & error webserver
func (s *Server) handleClearSiteLogs(w http.ResponseWriter, r *http.Request) {
	if err := s.firewall.ClearSiteLogs(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membersihkan log situs: " + err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.logs_clear", "", true, "Mengosongkan berkas log akses web", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Berkas log web berhasil dikosongkan"})
}

// handleAddExtendedPortRule menambahkan aturan firewall lengkap (arah, aksi, port/rentang, IP/CIDR, komentar)
func (s *Server) handleAddExtendedPortRule(w http.ResponseWriter, r *http.Request) {
	var req security.AddFirewallRuleInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.firewall.AddPortRule(r.Context(), req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.port_rule_add", req.Port, true, "Menambah aturan firewall "+req.Action+" "+req.Port+"/"+req.Protocol, s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Aturan firewall berhasil ditambahkan"})
}

// ── SSH Stats, Service, Authorized Keys & System Accounts ───────────────────

// handleGetSSHStats mengembalikan metrik login 7 hari, konfigurasi efektif, dan log autentikasi SSH aktual
func (s *Server) handleGetSSHStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.firewall.GetSSHStats(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memuat statistik SSH: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stats": stats})
}

// handleRestartSSHService memulai ulang daemon SSH server
func (s *Server) handleRestartSSHService(w http.ResponseWriter, r *http.Request) {
	if err := s.firewall.RestartSSHService(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.ssh_restart", "", true, "Memulai ulang layanan daemon SSH", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Layanan SSH berhasil dimulai ulang"})
}

// handleListSSHKeys mengembalikan daftar SSH Public Key terdaftar di authorized_keys
func (s *Server) handleListSSHKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.firewall.ListSSHKeys(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "keys": keys})
}

// handleAddSSHKey memvalidasi dan menambahkan SSH Public Key baru ke authorized_keys
func (s *Server) handleAddSSHKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	key, err := s.firewall.AddSSHKey(r.Context(), req.PublicKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.ssh_key_add", key.Fingerprint, true, "Menambahkan SSH Public Key ke authorized_keys", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "key": key, "message": "SSH Public Key berhasil ditambahkan"})
}

// handleRemoveSSHKey menghapus SSH Public Key berdasarkan ID fingerprint
func (s *Server) handleRemoveSSHKey(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("id")
	if err := s.firewall.RemoveSSHKey(r.Context(), keyID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.ssh_key_remove", keyID, true, "Menghapus SSH Public Key dari authorized_keys", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "SSH Public Key berhasil dihapus"})
}

// handleGetSystemAccounts menyajikan daftar akun sistem Linux beserta status kunci sebenarnya
func (s *Server) handleGetSystemAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.firewall.GetSystemAccounts(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "accounts": accounts})
}

// handleToggleAccountLock mengunci atau membuka kunci password akun sistem Linux (root dilindungi)
func (s *Server) handleToggleAccountLock(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("user")
	var req struct {
		Lock bool `json:"lock"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.firewall.ToggleAccountLock(r.Context(), username, req.Lock); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.account_lock", username, true, "Mengubah status kunci akun sistem", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ── Quick Fix Aman (Allowlist Terbatas tanpa Lockout SSH) ───────────────────

// handleApplySecurityFix mengeksekusi tindakan perbaikan aman dari hasil audit keamanan
func (s *Server) handleApplySecurityFix(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ActionKey string `json:"action_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}

	ctx := r.Context()
	var msg string
	switch req.ActionKey {
	case "enable_ufw":
		if err := s.firewall.ToggleFirewall(ctx, true); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		msg = "Firewall UFW berhasil diaktifkan dengan proteksi port SSH & panel"
	case "set_ufw_default_deny":
		if err := s.firewall.SetDefaultPolicy(ctx, "deny", "allow"); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		msg = "Kebijakan default incoming UFW berhasil diatur ke DENY"
	case "start_fail2ban":
		if err := s.firewall.ControlFail2BanService(ctx, "start"); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		msg = "Layanan Fail2Ban berhasil dijalankan"
	case "enable_bruteforce":
		cfg := s.firewall.GetBruteForceConfig(ctx)
		cfg.Enabled = true
		if _, err := s.firewall.SaveBruteForceConfig(ctx, cfg); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		msg = "Proteksi Brute Force Panel berhasil diaktifkan"
	case "apply_kernel_baseline":
		if err := s.firewall.SetSysctlParam(ctx, "net.ipv4.tcp_syncookies", true); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if err := s.firewall.SetSysctlParam(ctx, "net.ipv4.conf.all.rp_filter", true); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		msg = "Proteksi kernel SYN Flood Cookies & Reverse Path Filter berhasil diterapkan"
	case "create_fim_baseline":
		if _, err := s.firewall.CreateFIMBaseline(ctx); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		msg = "Baseline SHA-256 File Integrity Monitor berhasil dibuat"
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tindakan perbaikan otomatis tidak didukung untuk item ini; silakan ubah secara manual di tab terkait"})
		return
	}

	_ = s.store.RecordAudit(ctx, "user", "security.apply_fix", req.ActionKey, true, msg, s.callerUserID(r))

	// Jalankan ulang pemindaian otomatis agar skor langsung terbarui
	report, _ := s.firewall.RunSecurityScan(ctx, s.buildSecurityScanOptions(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": msg,
		"report":  report,
	})
}

// ── Brute Force Protection, Whitelist, Blacklist & History Persisten ────────

// handleGetBruteForce mengembalikan konfigurasi brute force, whitelist, blacklist, dan riwayat login gagal aktual
func (s *Server) handleGetBruteForce(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientIP := getIP(r)
	cfg := s.firewall.GetBruteForceConfig(ctx)
	whitelist, _ := s.firewall.GetWhitelistIPs(ctx)
	blacklist, _ := s.firewall.GetBlacklistIPs(ctx)
	events, _ := s.store.ListSecurityLoginEvents(ctx, 100, "")

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                    true,
		"client_ip":             clientIP,
		"is_client_whitelisted": s.firewall.IsIPWhitelisted(ctx, clientIP),
		"config":                cfg,
		"whitelist":             whitelist,
		"blacklist":             blacklist,
		"history":               events,
	})
}

// handleSaveBruteForceConfig menyimpan konfigurasi proteksi brute force secara persisten ke SQLite
func (s *Server) handleSaveBruteForceConfig(w http.ResponseWriter, r *http.Request) {
	var cfg security.BruteForceConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	saved, err := s.firewall.SaveBruteForceConfig(r.Context(), cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menyimpan konfigurasi brute force: " + err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.bruteforce_config", strconv.FormatBool(saved.Enabled), true, "Memperbarui konfigurasi proteksi brute force", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"config":  saved,
		"message": "Konfigurasi proteksi brute force berhasil disimpan",
	})
}

// handleAddWhitelistIP menambahkan IP/CIDR baru ke daftar putih persisten
func (s *Server) handleAddWhitelistIP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IP      string `json:"ip"`
		Remarks string `json:"remarks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.firewall.AddWhitelistIP(r.Context(), req.IP, req.Remarks); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.whitelist_add", req.IP, true, "Menambahkan IP ke whitelist", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleRemoveWhitelistIPBody menghapus IP/CIDR dari whitelist menggunakan JSON payload (aman untuk subnet CIDR '/')
func (s *Server) handleRemoveWhitelistIPBody(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64  `json:"id"`
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.firewall.RemoveWhitelistIP(r.Context(), req.ID, req.IP); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.whitelist_remove", req.IP, true, "Menghapus IP dari whitelist", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleRemoveWhitelistIP mempertahankan kompatibilitas hapus whitelist via path parameter
func (s *Server) handleRemoveWhitelistIP(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	if err := s.firewall.RemoveWhitelistIP(r.Context(), 0, ip); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.whitelist_remove", ip, true, "Menghapus IP dari whitelist", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleAddBlacklistIP memblokir alamat IP/CIDR secara persisten (dengan proteksi self-lockout & opsi sinkron UFW)
func (s *Server) handleAddBlacklistIP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IP              string `json:"ip"`
		Reason          string `json:"reason"`
		DurationMinutes int    `json:"duration_minutes"`
		SyncUFW         bool   `json:"sync_ufw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.firewall.AddBlacklistIP(r.Context(), req.IP, req.Reason, req.DurationMinutes, req.SyncUFW, "manual", getIP(r)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.blacklist_add", req.IP, true, "Memblokir IP secara manual", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleRemoveBlacklistIPBody mencabut pemblokiran IP/CIDR menggunakan JSON payload (aman untuk subnet CIDR '/')
func (s *Server) handleRemoveBlacklistIPBody(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64  `json:"id"`
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.firewall.RemoveBlacklistIP(r.Context(), req.ID, req.IP); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.blacklist_remove", req.IP, true, "Membuka blokir IP dari blacklist", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleRemoveBlacklistIP mempertahankan kompatibilitas hapus blacklist via path parameter
func (s *Server) handleRemoveBlacklistIP(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	if err := s.firewall.RemoveBlacklistIP(r.Context(), 0, ip); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.blacklist_remove", ip, true, "Membuka blokir IP dari blacklist", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ── Hardening Kernel Sysctl, Compiler & File Integrity Monitor (FIM) ────────

// handleGetHardeningStatus membaca status pengerasan kernel aktual (/proc/sys), compiler, dan FIM
func (s *Server) handleGetHardeningStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.firewall.GetHardeningStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memuat status hardening: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "hardening": status})
}

// handleToggleSysctlParam menerapkan perubahan parameter sysctl kernel secara live dan persisten
func (s *Server) handleToggleSysctlParam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.firewall.SetSysctlParam(r.Context(), req.Key, req.Enabled); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.sysctl_toggle", req.Key, true, fmt.Sprintf("Mengatur %s=%v", req.Key, req.Enabled), s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "key": req.Key, "enabled": req.Enabled})
}

// handleToggleCompilerAccess mengatur perizinan eksekusi compiler sistem (0700 / 0755)
func (s *Server) handleToggleCompilerAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Block bool `json:"block"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.firewall.ToggleCompilerAccess(r.Context(), req.Block); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.compiler_toggle", strconv.FormatBool(req.Block), true, "Mengubah izin akses compiler sistem", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "blocked": req.Block})
}

// handleCheckFIMIntegrity memverifikasi berkas kritis terhadap snapshot baseline SHA-256
func (s *Server) handleCheckFIMIntegrity(w http.ResponseWriter, r *http.Request) {
	res, err := s.firewall.CheckFIMIntegrity(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memeriksa integritas FIM: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "fim": res})
}

// handleCreateFIMBaseline membuat atau memperbarui snapshot hash SHA-256 berkas kritis
func (s *Server) handleCreateFIMBaseline(w http.ResponseWriter, r *http.Request) {
	res, err := s.firewall.CreateFIMBaseline(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membuat baseline FIM: " + err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.fim_baseline", strconv.Itoa(res.TotalFiles), true, "Membuat/memperbarui baseline SHA-256 FIM", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "fim": res, "message": fmt.Sprintf("Baseline FIM berhasil dibuat untuk %d berkas kritis", res.TotalFiles)})
}

// handleDisableFIM menghapus baseline FIM yang tersimpan
func (s *Server) handleDisableFIM(w http.ResponseWriter, r *http.Request) {
	if err := s.firewall.DisableFIM(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "security.fim_disable", "", true, "Menonaktifkan File Integrity Monitor", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "File Integrity Monitor dinonaktifkan"})
}

// notifySecurityEvent mengirim peringatan keamanan (login panel baru / auto-ban brute force) ke seluruh webhook aktif
func (s *Server) notifySecurityEvent(eventTag, summary, detail string, color int) {
	if s.store == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		hooks, err := s.store.ListWebhooks(ctx)
		if err != nil || len(hooks) == 0 {
			return
		}
		host, _ := os.Hostname()
		title := fmt.Sprintf("[FikPanel · %s] %s", eventTag, summary)
		fullMsg := fmt.Sprintf("%s\n%s\nHost: %s · %s", title, detail, host, time.Now().Format("2006-01-02 15:04:05 MST"))

		client := &http.Client{Timeout: 8 * time.Second}
		for _, h := range hooks {
			targetURL, payload, err := buildAlertWebhookPayload(&h, title, fullMsg, detail, eventTag, color, s.Version())
			if err != nil {
				continue
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "FikPanel-SecurityAlert/"+s.Version())
			resp, err := client.Do(req)
			if err == nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
				_ = resp.Body.Close()
			}
		}
	}()
}
