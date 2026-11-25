package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"fikpanel/internal/store"
)

const bruteForceSettingKey = "security_bruteforce_cfg_v1"

// DefaultBruteForceConfig mengembalikan konfigurasi bawaan proteksi brute-force yang aman
func DefaultBruteForceConfig() BruteForceConfig {
	return BruteForceConfig{
		Enabled:              true,
		UsernameProtection:   true,
		UsernameMaxRetries:   5,
		IPProtection:         true,
		IPMaxRetries:         5,
		IPBanDurationMinutes: 60,
		RetentionMinutes:     1440,
		NotifyOnLogin:        false,
		NotifyOnBan:          true,
		AutoSyncUFW:          false,
	}
}

// sanitizeBruteForceConfig menormalisasi batas aman setiap nilai numerik konfigurasi brute-force
func sanitizeBruteForceConfig(cfg BruteForceConfig) BruteForceConfig {
	if cfg.UsernameMaxRetries < 3 {
		cfg.UsernameMaxRetries = 3
	}
	if cfg.UsernameMaxRetries > 50 {
		cfg.UsernameMaxRetries = 50
	}
	if cfg.IPMaxRetries < 3 {
		cfg.IPMaxRetries = 3
	}
	if cfg.IPMaxRetries > 100 {
		cfg.IPMaxRetries = 100
	}
	if cfg.IPBanDurationMinutes < 1 {
		cfg.IPBanDurationMinutes = 15
	}
	if cfg.IPBanDurationMinutes > 43200 { // maks 30 hari
		cfg.IPBanDurationMinutes = 43200
	}
	if cfg.RetentionMinutes < 15 {
		cfg.RetentionMinutes = 60
	}
	if cfg.RetentionMinutes > 43200 {
		cfg.RetentionMinutes = 43200
	}
	return cfg
}

// GetBruteForceConfig memuat konfigurasi brute-force dari cache memori atau database SQLite
func (m *Manager) GetBruteForceConfig(ctx context.Context) BruteForceConfig {
	m.mu.RLock()
	if m.bfLoaded {
		cfg := m.bfCfg
		m.mu.RUnlock()
		return cfg
	}
	m.mu.RUnlock()

	cfg := DefaultBruteForceConfig()
	if m.store != nil {
		if raw, err := m.store.GetSetting(ctx, bruteForceSettingKey); err == nil && strings.TrimSpace(raw) != "" {
			var parsed BruteForceConfig
			if json.Unmarshal([]byte(raw), &parsed) == nil {
				cfg = sanitizeBruteForceConfig(parsed)
			}
		}
	}

	m.mu.Lock()
	m.bfCfg = cfg
	m.bfLoaded = true
	m.mu.Unlock()
	return cfg
}

// SaveBruteForceConfig memvalidasi dan menyimpan konfigurasi proteksi brute-force ke SQLite
func (m *Manager) SaveBruteForceConfig(ctx context.Context, cfg BruteForceConfig) (BruteForceConfig, error) {
	clean := sanitizeBruteForceConfig(cfg)
	if m.store != nil {
		raw, err := json.Marshal(clean)
		if err != nil {
			return clean, err
		}
		if err := m.store.SetSetting(ctx, bruteForceSettingKey, string(raw)); err != nil {
			return clean, err
		}
	}
	m.mu.Lock()
	m.bfCfg = clean
	m.bfLoaded = true
	m.mu.Unlock()
	return clean, nil
}

// refreshIPCache menyegarkan cache daftar putih dan daftar hitam IP dari SQLite serta membersihkan ban kedaluwarsa
func (m *Manager) refreshIPCache(ctx context.Context) error {
	if m.store == nil {
		return nil
	}
	// Bersihkan aturan UFW milik entri blacklist yang sudah kedaluwarsa
	if expired, err := m.store.PurgeExpiredBlacklist(ctx); err == nil {
		for _, ex := range expired {
			if ex.SyncUFW {
				_ = m.UnblockIPInUFW(ctx, ex.IP)
			}
		}
	}

	wl, err := m.store.ListSecurityIPs(ctx, "whitelist")
	if err != nil {
		return err
	}
	bl, err := m.store.ListSecurityIPs(ctx, "blacklist")
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.whitelistCache = wl
	m.blacklistCache = bl
	m.ipCacheLoaded = true
	m.mu.Unlock()
	return nil
}

func (m *Manager) ensureIPCache(ctx context.Context) {
	m.mu.RLock()
	loaded := m.ipCacheLoaded
	m.mu.RUnlock()
	if !loaded {
		_ = m.refreshIPCache(ctx)
	}
}

// GetWhitelistIPs mengembalikan daftar IP whitelist aktif dari database
func (m *Manager) GetWhitelistIPs(ctx context.Context) ([]store.SecurityIPRule, error) {
	if err := m.refreshIPCache(ctx); err != nil {
		return []store.SecurityIPRule{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]store.SecurityIPRule, len(m.whitelistCache))
	copy(out, m.whitelistCache)
	return out, nil
}

// GetBlacklistIPs mengembalikan daftar IP blacklist aktif dari database
func (m *Manager) GetBlacklistIPs(ctx context.Context) ([]store.SecurityIPRule, error) {
	if err := m.refreshIPCache(ctx); err != nil {
		return []store.SecurityIPRule{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]store.SecurityIPRule, len(m.blacklistCache))
	copy(out, m.blacklistCache)
	return out, nil
}

// AddWhitelistIP memvalidasi dan menyimpan alamat IP/CIDR ke daftar putih persisten
func (m *Manager) AddWhitelistIP(ctx context.Context, rawIP, remarks string) error {
	normIP, err := ValidateIPOrCIDR(rawIP)
	if err != nil {
		return err
	}
	if strings.TrimSpace(remarks) == "" {
		remarks = "Whitelist Operator"
	}
	if m.store != nil {
		if err := m.store.UpsertSecurityIP(ctx, store.SecurityIPRule{
			ListType:  "whitelist",
			IP:        normIP,
			Remarks:   strings.TrimSpace(remarks),
			Source:    "manual",
			AddedAt:   time.Now().Unix(),
			ExpiresAt: 0,
		}); err != nil {
			return err
		}
		// Hapus otomatis dari blacklist bila IP sebelumnya sempat terblokir
		if removed, err := m.store.DeleteSecurityIP(ctx, "blacklist", normIP); err == nil && removed != nil && removed.SyncUFW {
			_ = m.UnblockIPInUFW(ctx, normIP)
		}
	}
	return m.refreshIPCache(ctx)
}

// RemoveWhitelistIP menghapus entri whitelist berdasarkan ID atau alamat IP dengan proteksi loopback
func (m *Manager) RemoveWhitelistIP(ctx context.Context, id int64, ip string) error {
	if m.store == nil {
		return nil
	}
	if id > 0 {
		if _, err := m.store.DeleteSecurityIPByID(ctx, id); err != nil {
			return err
		}
	} else {
		if _, err := m.store.DeleteSecurityIP(ctx, "whitelist", strings.TrimSpace(ip)); err != nil {
			return err
		}
	}
	return m.refreshIPCache(ctx)
}

// AddBlacklistIP memblokir alamat IP/CIDR secara persisten dengan proteksi anti self-lockout dan opsi sinkron UFW
func (m *Manager) AddBlacklistIP(ctx context.Context, rawIP, reason string, durationMinutes int, syncUFW bool, source string, callerIP string) error {
	normIP, err := ValidateIPOrCIDR(rawIP)
	if err != nil {
		return err
	}

	// Proteksi loopback & IP admin saat ini agar operator tidak mengunci dirinya sendiri
	if ipMatchesEntry("127.0.0.1", normIP) || ipMatchesEntry("::1", normIP) {
		return fmt.Errorf("alamat loopback (127.0.0.1 / ::1) tidak boleh dimasukkan ke daftar hitam")
	}
	if callerIP != "" && ipMatchesEntry(callerIP, normIP) {
		return fmt.Errorf("tidak dapat memblokir alamat IP Anda sendiri (%s)", callerIP)
	}
	if m.IsIPWhitelisted(ctx, normIP) {
		return fmt.Errorf("alamat IP %s terdaftar di Whitelist; hapus dari Whitelist terlebih dahulu bila ingin memblokir", normIP)
	}

	if strings.TrimSpace(reason) == "" {
		reason = "Pemblokiran manual oleh administrator"
	}
	if source == "" {
		source = "manual"
	}

	now := time.Now().Unix()
	var expiresAt int64
	if durationMinutes > 0 {
		expiresAt = now + int64(durationMinutes*60)
	}

	if syncUFW {
		if err := m.BlockIPInUFW(ctx, normIP, reason); err != nil {
			// Tetap simpan blokir level panel tetapi kembalikan error UFW yang jelas
			syncUFW = false
		}
	}

	if m.store != nil {
		if err := m.store.UpsertSecurityIP(ctx, store.SecurityIPRule{
			ListType:  "blacklist",
			IP:        normIP,
			Remarks:   strings.TrimSpace(reason),
			Source:    source,
			SyncUFW:   syncUFW,
			AddedAt:   now,
			ExpiresAt: expiresAt,
		}); err != nil {
			return err
		}
	}
	return m.refreshIPCache(ctx)
}

// RemoveBlacklistIP mencabut pemblokiran IP dari database dan sekaligus membersihkan aturan UFW bila tersinkron
func (m *Manager) RemoveBlacklistIP(ctx context.Context, id int64, ip string) error {
	if m.store == nil {
		return nil
	}
	var removed *store.SecurityIPRule
	var err error
	if id > 0 {
		removed, err = m.store.DeleteSecurityIPByID(ctx, id)
	} else {
		removed, err = m.store.DeleteSecurityIP(ctx, "blacklist", strings.TrimSpace(ip))
	}
	if err != nil {
		return err
	}
	if removed != nil && removed.SyncUFW {
		_ = m.UnblockIPInUFW(ctx, removed.IP)
	}
	return m.refreshIPCache(ctx)
}

// IsIPWhitelisted memeriksa apakah alamat IP cocok dengan salah satu entri IP/CIDR whitelist (termasuk loopback)
func (m *Manager) IsIPWhitelisted(ctx context.Context, ip string) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed != nil && parsed.IsLoopback() {
		return true
	}
	m.ensureIPCache(ctx)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, entry := range m.whitelistCache {
		if ipMatchesEntry(ip, entry.IP) {
			return true
		}
	}
	return false
}

// IsIPBlacklisted memeriksa apakah alamat IP sedang diblokir aktif (belum kedaluwarsa dan tidak masuk whitelist)
func (m *Manager) IsIPBlacklisted(ctx context.Context, ip string) (*store.SecurityIPRule, bool) {
	if m.IsIPWhitelisted(ctx, ip) {
		return nil, false
	}
	m.ensureIPCache(ctx)
	now := time.Now().Unix()

	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, entry := range m.blacklistCache {
		if entry.ExpiresAt > 0 && entry.ExpiresAt <= now {
			continue
		}
		if ipMatchesEntry(ip, entry.IP) {
			cp := entry
			return &cp, true
		}
	}
	return nil, false
}

// ipMatchesEntry mencocokkan satu alamat IP klien terhadap entri IP tunggal atau subnet CIDR
func ipMatchesEntry(clientIP, entry string) bool {
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	entry = strings.TrimSpace(entry)
	if ip == nil {
		return strings.EqualFold(strings.TrimSpace(clientIP), entry)
	}
	if strings.Contains(entry, "/") {
		_, cidr, err := net.ParseCIDR(entry)
		return err == nil && cidr.Contains(ip)
	}
	target := net.ParseIP(entry)
	return target != nil && target.Equal(ip)
}

// CheckLoginAllowed memverifikasi apakah percobaan login dari IP dan username diizinkan oleh kebijakan Brute Force
func (m *Manager) CheckLoginAllowed(ctx context.Context, ip, username string) (allowed bool, reason string) {
	// Selalu izinkan IP yang masuk daftar putih
	if m.IsIPWhitelisted(ctx, ip) {
		return true, ""
	}
	// Tolak langsung IP yang masuk daftar hitam
	if rule, banned := m.IsIPBlacklisted(ctx, ip); banned {
		if rule.ExpiresAt > 0 {
			rem := time.Until(time.Unix(rule.ExpiresAt, 0)).Round(time.Minute)
			if rem < time.Minute {
				rem = time.Minute
			}
			return false, fmt.Sprintf("alamat IP Anda diblokir sementara (%s). Sisa waktu: %s", rule.Remarks, rem)
		}
		return false, fmt.Sprintf("alamat IP Anda diblokir oleh kebijakan keamanan (%s)", rule.Remarks)
	}

	cfg := m.GetBruteForceConfig(ctx)
	if !cfg.Enabled || m.store == nil {
		return true, ""
	}

	// Periksa kunci sementara berdasarkan username bila proteksi username aktif
	if cfg.UsernameProtection && strings.TrimSpace(username) != "" {
		windowStart := time.Now().Add(-15 * time.Minute).Unix()
		count, err := m.store.CountRecentFailedLoginsByUser(ctx, strings.TrimSpace(username), windowStart)
		if err == nil && count >= cfg.UsernameMaxRetries {
			return false, fmt.Sprintf("akun '%s' dikunci sementara selama 15 menit karena terlalu banyak percobaan gagal", username)
		}
	}

	return true, ""
}

// RecordLoginFailure mencatat kegagalan autentikasi panel dan memicu auto-ban IP jika melampaui ambang batas
func (m *Manager) RecordLoginFailure(ctx context.Context, ip, username, reason, userAgent string) (autoBanned bool, banMinutes int) {
	if m.store == nil {
		return false, 0
	}
	_ = m.store.RecordSecurityLoginEvent(ctx, store.SecurityLoginEvent{
		TS:        time.Now().Unix(),
		IP:        ip,
		Username:  strings.TrimSpace(username),
		Source:    "panel",
		Status:    "failed",
		Reason:    reason,
		UserAgent: userAgent,
	})

	cfg := m.GetBruteForceConfig(ctx)
	if !cfg.Enabled || !cfg.IPProtection || m.IsIPWhitelisted(ctx, ip) {
		return false, 0
	}

	windowStart := time.Now().Add(-15 * time.Minute).Unix()
	count, err := m.store.CountRecentFailedLoginsByIP(ctx, ip, windowStart)
	if err == nil && count >= cfg.IPMaxRetries {
		banReason := fmt.Sprintf("Auto-ban: %d percobaan login gagal dalam 15 menit", count)
		_ = m.AddBlacklistIP(ctx, ip, banReason, cfg.IPBanDurationMinutes, cfg.AutoSyncUFW, "auto_bruteforce", "")
		_ = m.store.RecordSecurityLoginEvent(ctx, store.SecurityLoginEvent{
			TS:        time.Now().Unix(),
			IP:        ip,
			Username:  strings.TrimSpace(username),
			Source:    "panel",
			Status:    "banned",
			Reason:    banReason,
			UserAgent: userAgent,
		})
		return true, cfg.IPBanDurationMinutes
	}

	// Bersihkan riwayat login di luar jendela retensi secara periodik
	retentionCutoff := time.Now().Add(-time.Duration(cfg.RetentionMinutes) * time.Minute).Unix()
	_ = m.store.PruneSecurityLoginEvents(ctx, retentionCutoff)
	return false, 0
}

// RecordLoginSuccess mencatat keberhasilan login panel ke riwayat keamanan persisten
func (m *Manager) RecordLoginSuccess(ctx context.Context, ip, username, userAgent string) {
	if m.store == nil {
		return
	}
	_ = m.store.RecordSecurityLoginEvent(ctx, store.SecurityLoginEvent{
		TS:        time.Now().Unix(),
		IP:        ip,
		Username:  strings.TrimSpace(username),
		Source:    "panel",
		Status:    "success",
		Reason:    "Login panel berhasil",
		UserAgent: userAgent,
	})
}
