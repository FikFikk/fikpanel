package waf

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// HostLocation merepresentasikan identitas dan koordinat geografis server origin FikPanel secara dinamis
type HostLocation struct {
	IP          string  `json:"ip"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	City        string  `json:"city"`
	ISP         string  `json:"isp"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Label       string  `json:"label"`
}

// WAFEnforcementStatus merepresentasikan transparansi status sinkronisasi aturan WAF ke mesin Nginx
type WAFEnforcementStatus struct {
	GlobalEnabled       bool   `json:"global_enabled"`
	NginxEnforced       bool   `json:"nginx_enforced"`
	ConfigPath          string `json:"config_path"`
	ActiveRulesCount    int    `json:"active_rules_count"`
	ProtectedSitesCount int    `json:"protected_sites_count"`
	LastSyncedAt        int64  `json:"last_synced_at"`
	LastSyncError       string `json:"last_sync_error,omitempty"`
	ConfigPreview       string `json:"config_preview,omitempty"`
}

var (
	hostLocMu      sync.Mutex
	cachedHostLoc  *HostLocation
	hostLocFetched time.Time

	latencyMu      sync.Mutex
	cachedLatency  int
	latencyFetched time.Time
)

// defaultWAFDir mengembalikan direktori penyimpanan snippet konfigurasi WAF Nginx
func defaultWAFDir() string {
	if v := os.Getenv("FIKPANEL_WAF_DIR"); v != "" {
		return v
	}
	if runtime.GOOS != "linux" {
		return filepath.Join("dev-data", "nginx", "fikpanel-waf")
	}
	return "/etc/nginx/fikpanel-waf"
}

// DetectHostLocation mendeteksi IP publik dan geolokasi server origin FikPanel secara dinamis (tanpa hardcode)
func DetectHostLocation(ctx context.Context) HostLocation {
	hostLocMu.Lock()
	defer hostLocMu.Unlock()

	if cachedHostLoc != nil && time.Since(hostLocFetched) < 30*time.Minute {
		return *cachedHostLoc
	}

	pubIP := strings.TrimSpace(os.Getenv("FIKPANEL_PUBLIC_IP"))
	if idx := strings.IndexByte(pubIP, ','); idx > 0 {
		pubIP = strings.TrimSpace(pubIP[:idx])
	}

	if net.ParseIP(pubIP) == nil {
		endpoints := []string{
			"https://api.ipify.org",
			"https://icanhazip.com",
			"https://ifconfig.me/ip",
		}
		for _, ep := range endpoints {
			reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, ep, nil)
			if err != nil {
				cancel()
				continue
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				cancel()
				continue
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
			resp.Body.Close()
			cancel()
			if err == nil && resp.StatusCode == http.StatusOK {
				candidate := strings.TrimSpace(string(body))
				if net.ParseIP(candidate) != nil {
					pubIP = candidate
					break
				}
			}
		}
	}

	// Fallback ke IP antarmuka lokal bila server berada di jaringan terisolasi/offline
	if net.ParseIP(pubIP) == nil {
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range addrs {
				if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
					pubIP = ipNet.IP.String()
					break
				}
			}
		}
	}
	if pubIP == "" {
		pubIP = "127.0.0.1"
	}

	geo := ResolveGeoLocation(pubIP)
	lat := geo.Latitude
	lng := geo.Longitude
	country := geo.Country
	cc := geo.CountryCode
	city := geo.City
	isp := geo.ISP

	// Jika server berjalan di jaringan lokal/offline tanpa koordinat GPS publik, gunakan titik netral equator/Indonesia
	if lat == 0 && lng == 0 {
		lat = -6.2088
		lng = 106.8456
		if country == "" || country == "Local / Private Network" {
			country = "Local Origin Node"
			cc = "LO"
		}
	}

	label := fmt.Sprintf("FikPanel Origin (%s", pubIP)
	if city != "" && city != "-" && city != "Unresolved" {
		label += " · " + city
	} else if country != "" {
		label += " · " + country
	}
	label += ")"

	loc := HostLocation{
		IP:          pubIP,
		Country:     country,
		CountryCode: cc,
		City:        city,
		ISP:         isp,
		Latitude:    lat,
		Longitude:   lng,
		Label:       label,
	}
	cachedHostLoc = &loc
	hostLocFetched = time.Now()
	return loc
}

// MeasureOriginLatencyMs mengukur latensi TCP/HTTP aktual ke server web lokal (cached 15 detik)
func MeasureOriginLatencyMs() int {
	latencyMu.Lock()
	defer latencyMu.Unlock()

	if time.Since(latencyFetched) < 15*time.Second && cachedLatency > 0 {
		return cachedLatency
	}

	targets := []string{"127.0.0.1:80", "127.0.0.1:443", "127.0.0.1:2026"}
	for _, addr := range targets {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", addr, 350*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			ms := int(time.Since(start).Milliseconds())
			if ms < 1 {
				ms = 1
			}
			cachedLatency = ms
			latencyFetched = time.Now()
			return ms
		}
	}

	cachedLatency = 1
	latencyFetched = time.Now()
	return cachedLatency
}

// sanitizeNginxPattern membersihkan karakter kutip/semicolon agar aman dimasukkan ke konfigurasi Nginx
func sanitizeNginxPattern(s string) string {
	r := strings.NewReplacer(
		`"`, `\"`,
		`;`, ``,
		"\n", "",
		"\r", "",
		`$`, `\$`,
	)
	return strings.TrimSpace(r.Replace(s))
}

// BuildNginxWAFConfig merender snippet aturan WAF Nginx dari state global, daftar aturan Black/White List, dan situs
func BuildNginxWAFConfig(globalEnabled bool, rules []WAFRule, sites []WAFSite) string {
	var b strings.Builder
	b.WriteString("# ============================================================================\n")
	b.WriteString("# FikPanel Dynamic WAF Rules Snippet — Dikelola Otomatis oleh Mesin WAF\n")
	b.WriteString(fmt.Sprintf("# Disinkronkan pada: %s\n", time.Now().Format(time.RFC3339)))
	b.WriteString("# ============================================================================\n\n")

	if !globalEnabled {
		b.WriteString("# [STATUS] Global WAF saat ini DINONAKTIFKAN oleh Administrator.\n")
		b.WriteString("# Seluruh aturan pencegatan dilewati (Pass-Through Mode).\n")
		return b.String()
	}

	// 1. Aturan IP Whitelist & Blacklist (Nginx Access Module)
	var ipWhite, ipBlack []WAFRule
	var uaWhite, uaBlack []WAFRule
	var urlWhite, urlBlack []WAFRule

	for _, r := range rules {
		switch r.RuleType {
		case "ip_whitelist":
			ipWhite = append(ipWhite, r)
		case "ip_blacklist":
			ipBlack = append(ipBlack, r)
		case "ua_whitelist":
			uaWhite = append(uaWhite, r)
		case "ua_blacklist":
			uaBlack = append(uaBlack, r)
		case "url_whitelist":
			urlWhite = append(urlWhite, r)
		case "url_blacklist":
			urlBlack = append(urlBlack, r)
		}
	}

	b.WriteString("# --- 1. Hierarki Akses IP (Whitelist & Blacklist) ---\n")
	for _, r := range ipWhite {
		ipTarget := strings.TrimSpace(r.StartIP)
		if ipTarget == "" {
			continue
		}
		if net.ParseIP(ipTarget) != nil || strings.Contains(ipTarget, "/") {
			b.WriteString(fmt.Sprintf("allow %s; # Whitelist: %s\n", ipTarget, sanitizeNginxPattern(r.Remarks)))
		}
	}
	for _, r := range ipBlack {
		ipTarget := strings.TrimSpace(r.StartIP)
		if ipTarget == "" {
			continue
		}
		if net.ParseIP(ipTarget) != nil || strings.Contains(ipTarget, "/") {
			b.WriteString(fmt.Sprintf("deny %s; # Blacklist: %s\n", ipTarget, sanitizeNginxPattern(r.Remarks)))
		}
	}
	b.WriteString("\n")

	// 2. Evaluasi Whitelist & Blacklist URL / User-Agent
	b.WriteString("# --- 2. Inspeksi Pola URL & User-Agent ---\n")
	b.WriteString("set $fik_waf_allow 0;\n")
	for _, r := range urlWhite {
		pat := sanitizeNginxPattern(r.Pattern)
		if pat != "" {
			b.WriteString(fmt.Sprintf("if ($request_uri ~* \"%s\") { set $fik_waf_allow 1; }\n", regexp.QuoteMeta(pat)))
		}
	}
	for _, r := range uaWhite {
		pat := sanitizeNginxPattern(r.Pattern)
		if pat != "" {
			b.WriteString(fmt.Sprintf("if ($http_user_agent ~* \"%s\") { set $fik_waf_allow 1; }\n", regexp.QuoteMeta(pat)))
		}
	}

	b.WriteString("set $fik_waf_block 0;\n")
	for _, r := range urlBlack {
		pat := sanitizeNginxPattern(r.Pattern)
		if pat != "" {
			b.WriteString(fmt.Sprintf("if ($request_uri ~* \"%s\") { set $fik_waf_block 1; }\n", regexp.QuoteMeta(pat)))
		}
	}
	for _, r := range uaBlack {
		pat := sanitizeNginxPattern(r.Pattern)
		if pat != "" {
			b.WriteString(fmt.Sprintf("if ($http_user_agent ~* \"%s\") { set $fik_waf_block 1; }\n", regexp.QuoteMeta(pat)))
		}
	}

	// 3. Signature Proteksi Inti OWASP (SQLi, XSS, Path Traversal, Sensitive Files)
	b.WriteString("\n# --- 3. Proteksi Signature Inti OWASP (SQLi, XSS, LFI, Sensitive Probe) ---\n")
	b.WriteString(`if ($query_string ~* "(union\s+select|information_schema|sleep\(|benchmark\(|load_file\()") { set $fik_waf_block 1; }` + "\n")
	b.WriteString(`if ($query_string ~* "(<script|javascript:|onerror\s*=|onload\s*=)") { set $fik_waf_block 1; }` + "\n")
	b.WriteString(`if ($request_uri ~* "(\.\./|\.\.\\|/etc/passwd|/etc/shadow|/\.env|/\.git|/wp-config\.php)") { set $fik_waf_block 1; }` + "\n")

	b.WriteString(`if ($fik_waf_allow = 1) { set $fik_waf_block 0; }` + "\n")
	b.WriteString(`if ($fik_waf_block = 1) { return 403; }` + "\n\n")

	// 4. Ringkasan Kebijakan Per-Situs
	b.WriteString("# --- 4. Status Kebijakan Per-Situs ---\n")
	for _, s := range sites {
		b.WriteString(fmt.Sprintf("# Situs: %s | Aktif: %v | Anti-FakeSpider: %v | CrawlProt: %v | CDN: %v | SmartCC: %d req/s\n",
			s.SiteName, s.Status, s.FakeSpider, s.CrawlProtection, s.UsingCDN, s.SmartCC))
	}

	return b.String()
}

// SyncNginxWAFConfig menulis ulang berkas snippet aturan WAF Nginx dan melakukan reload aman bila di Linux
func (m *Manager) SyncNginxWAFConfig(ctx context.Context) (*WAFEnforcementStatus, error) {
	globalEnabled, siteMap, rules := m.getPolicySnapshot()
	sites := make([]WAFSite, 0, len(siteMap))
	protectedCount := 0
	for _, s := range siteMap {
		sites = append(sites, s)
		if s.Status {
			protectedCount++
		}
	}

	wafDir := defaultWAFDir()
	_ = os.MkdirAll(wafDir, 0o755)
	cfgPath := filepath.ToSlash(filepath.Join(wafDir, "waf-rules.conf"))
	content := BuildNginxWAFConfig(globalEnabled, rules, sites)

	var prevBytes []byte
	hadPrev := false
	if b, err := os.ReadFile(cfgPath); err == nil {
		prevBytes = b
		hadPrev = true
	}

	err := os.WriteFile(cfgPath, []byte(content), 0o644)
	syncErrStr := ""
	nginxEnforced := err == nil
	if err != nil {
		syncErrStr = err.Error()
	} else if runtime.GOOS == "linux" {
		if nginxBin, lookErr := exec.LookPath("nginx"); lookErr == nil {
			tCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
			out, testErr := exec.CommandContext(tCtx, nginxBin, "-t").CombinedOutput()
			cancel()
			if testErr != nil {
				nginxEnforced = false
				syncErrStr = fmt.Sprintf("validasi nginx -t gagal: %s", strings.TrimSpace(string(out)))
				if hadPrev {
					_ = os.WriteFile(cfgPath, prevBytes, 0o644)
				}
			} else {
				rCtx, rCancel := context.WithTimeout(ctx, 4*time.Second)
				_ = exec.CommandContext(rCtx, nginxBin, "-s", "reload").Run()
				rCancel()
			}
		}
	}

	now := time.Now().Unix()
	st := &WAFEnforcementStatus{
		GlobalEnabled:       globalEnabled,
		NginxEnforced:       nginxEnforced,
		ConfigPath:          cfgPath,
		ActiveRulesCount:    len(rules),
		ProtectedSitesCount: protectedCount,
		LastSyncedAt:        now,
		LastSyncError:       syncErrStr,
		ConfigPreview:       content,
	}

	m.mu.Lock()
	m.lastEnforcement = st
	m.mu.Unlock()

	return st, err
}
