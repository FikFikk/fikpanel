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

// WAFEnforcementStatus merepresentasikan transparansi penuh status sinkronisasi aturan WAF ke mesin Nginx
type WAFEnforcementStatus struct {
	GlobalEnabled             bool   `json:"global_enabled"`
	NginxEnforced             bool   `json:"nginx_enforced"`
	ConfigPath                string `json:"config_path"`
	HTTPConfigPath            string `json:"http_config_path"`
	ActiveRulesCount          int    `json:"active_rules_count"`
	ActiveVirtualPatchesCount int    `json:"active_virtual_patches_count"`
	ProtectedSitesCount       int    `json:"protected_sites_count"`
	AttachedVhostsCount       int    `json:"attached_vhosts_count"`
	TotalVhostsCount          int    `json:"total_vhosts_count"`
	CloudflareRealIPEnabled   bool   `json:"cloudflare_real_ip_enabled"`
	LastSyncedAt              int64  `json:"last_synced_at"`
	LastSyncError             string `json:"last_sync_error,omitempty"`
	ConfigPreview             string `json:"config_preview,omitempty"`
	HTTPConfigPreview         string `json:"http_config_preview,omitempty"`
}

var (
	hostLocMu      sync.Mutex
	cachedHostLoc  *HostLocation
	hostLocFetched time.Time

	latencyMu      sync.Mutex
	cachedLatency  int
	latencyFetched time.Time
)

// Daftar rentang IP resmi Cloudflare untuk directive set_real_ip_from Nginx agar IP asli pengunjung terbaca
var cloudflareRealIPRanges = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
	"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
	"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
	"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
}

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

// DefaultWAFRulesPath mengembalikan jalur lengkap snippet aturan WAF untuk di-include di server block vhost
func DefaultWAFRulesPath() string {
	return filepath.ToSlash(filepath.Join(defaultWAFDir(), "waf-rules.conf"))
}

// defaultWAFHTTPConfigPath menentukan lokasi snippet level http { } (rate-limit zone & Cloudflare real_ip)
func defaultWAFHTTPConfigPath() string {
	if v := os.Getenv("FIKPANEL_WAF_DIR"); v != "" {
		return filepath.ToSlash(filepath.Join(v, "waf-http.conf"))
	}
	if runtime.GOOS != "linux" {
		return filepath.ToSlash(filepath.Join("dev-data", "nginx", "fikpanel-waf", "waf-http.conf"))
	}
	if fi, err := os.Stat("/etc/nginx/conf.d"); err == nil && fi.IsDir() {
		return "/etc/nginx/conf.d/00-fikpanel-waf-http.conf"
	}
	return "/etc/nginx/fikpanel-waf/waf-http.conf"
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
	lat, lng := geo.Latitude, geo.Longitude
	country, cc, city, isp := geo.Country, geo.CountryCode, geo.City, geo.ISP

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

// MeasureOriginLatencyMs mengukur latensi HTTP TTFB nyata ke Nginx lokal (cached 15 detik, fallback TCP)
func MeasureOriginLatencyMs() int {
	latencyMu.Lock()
	defer latencyMu.Unlock()

	if time.Since(latencyFetched) < 15*time.Second && cachedLatency > 0 {
		return cachedLatency
	}

	cli := &http.Client{
		Timeout: 800 * time.Millisecond,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	start := time.Now()
	if resp, err := cli.Head("http://127.0.0.1/"); err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
		_ = resp.Body.Close()
		ms := int(time.Since(start).Milliseconds())
		if ms < 1 {
			ms = 1
		}
		cachedLatency = ms
		latencyFetched = time.Now()
		return ms
	}

	for _, addr := range []string{"127.0.0.1:80", "127.0.0.1:443"} {
		t0 := time.Now()
		conn, err := net.DialTimeout("tcp", addr, 350*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			ms := int(time.Since(t0).Milliseconds())
			if ms < 1 {
				ms = 1
			}
			cachedLatency = ms
			latencyFetched = time.Now()
			return ms
		}
	}

	cachedLatency = 0
	latencyFetched = time.Now()
	return 0
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

// BuildNginxWAFHTTPConfig merender konfigurasi level http { } untuk SmartCC limit_req_zone & Cloudflare Real-IP
func BuildNginxWAFHTTPConfig(globalEnabled bool, sites []WAFSite) (string, bool) {
	var b strings.Builder
	b.WriteString("# ============================================================================\n")
	b.WriteString("# FikPanel WAF HTTP-Level Config (Rate Limit Zone & Cloudflare Real-IP)\n")
	b.WriteString(fmt.Sprintf("# Disinkronkan pada: %s\n", time.Now().Format(time.RFC3339)))
	b.WriteString("# ============================================================================\n\n")

	if !globalEnabled {
		b.WriteString("# Global WAF nonaktif (Pass-Through Mode).\n")
		return b.String(), false
	}

	// Hitung rate terendah aktif SmartCC antar situs (default 100 req/s)
	ccRate := 100
	hasCDN := false
	for _, s := range sites {
		if !s.Status {
			continue
		}
		if s.UsingCDN {
			hasCDN = true
		}
		if s.RateLimit && s.SmartCC >= 5 && s.SmartCC < ccRate {
			ccRate = s.SmartCC
		}
	}

	b.WriteString(fmt.Sprintf("limit_req_zone $binary_remote_addr zone=fikpanel_waf_cc:10m rate=%dr/s;\n", ccRate))
	b.WriteString("limit_req_status 429;\n\n")

	if hasCDN {
		b.WriteString("# Ekstraksi IP Klien Asli di Balik Cloudflare Proxy (Using CDN aktif)\n")
		for _, cidr := range cloudflareRealIPRanges {
			b.WriteString(fmt.Sprintf("set_real_ip_from %s;\n", cidr))
		}
		b.WriteString("real_ip_header CF-Connecting-IP;\n")
		b.WriteString("real_ip_recursive on;\n")
	}
	return b.String(), hasCDN
}

// BuildNginxWAFConfig merender snippet aturan WAF Nginx lengkap (IP, URL, UA, Virtual Patches, Bot, OWASP, Rate-Limit)
func BuildNginxWAFConfig(globalEnabled bool, rules []WAFRule, sites []WAFSite, activePatches ...map[string]bool) string {
	var patchMap map[string]bool
	if len(activePatches) > 0 {
		patchMap = activePatches[0]
	}

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

	var ipWhite, ipBlack, uaWhite, uaBlack, urlWhite, urlBlack []WAFRule
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
		if ipTarget != "" && (net.ParseIP(ipTarget) != nil || strings.Contains(ipTarget, "/")) {
			b.WriteString(fmt.Sprintf("allow %s; # Whitelist: %s\n", ipTarget, sanitizeNginxPattern(r.Remarks)))
		}
	}
	for _, r := range ipBlack {
		ipTarget := strings.TrimSpace(r.StartIP)
		if ipTarget != "" && (net.ParseIP(ipTarget) != nil || strings.Contains(ipTarget, "/")) {
			b.WriteString(fmt.Sprintf("deny %s; # Blacklist: %s\n", ipTarget, sanitizeNginxPattern(r.Remarks)))
		}
	}
	b.WriteString("\n")

	b.WriteString("# --- 2. Inspeksi Pola URL & User-Agent ---\n")
	b.WriteString("set $fik_waf_allow 0;\n")
	for _, r := range urlWhite {
		if pat := sanitizeNginxPattern(r.Pattern); pat != "" {
			b.WriteString(fmt.Sprintf("if ($request_uri ~* \"%s\") { set $fik_waf_allow 1; }\n", regexp.QuoteMeta(pat)))
		}
	}
	for _, r := range uaWhite {
		if pat := sanitizeNginxPattern(r.Pattern); pat != "" {
			b.WriteString(fmt.Sprintf("if ($http_user_agent ~* \"%s\") { set $fik_waf_allow 1; }\n", regexp.QuoteMeta(pat)))
		}
	}

	b.WriteString("set $fik_waf_block 0;\n")
	for _, r := range urlBlack {
		if pat := sanitizeNginxPattern(r.Pattern); pat != "" {
			b.WriteString(fmt.Sprintf("if ($request_uri ~* \"%s\") { set $fik_waf_block 1; }\n", regexp.QuoteMeta(pat)))
		}
	}
	for _, r := range uaBlack {
		if pat := sanitizeNginxPattern(r.Pattern); pat != "" {
			b.WriteString(fmt.Sprintf("if ($http_user_agent ~* \"%s\") { set $fik_waf_block 1; }\n", regexp.QuoteMeta(pat)))
		}
	}

	// 3. Paket Virtual Patching CVE 1-Klik yang Aktif
	if len(patchMap) > 0 {
		b.WriteString("\n# --- 3. Virtual Patching CVE Aktif ---\n")
		for _, vp := range builtinVirtualPatches {
			if patchMap[vp.meta.ID] {
				b.WriteString(vp.meta.NginxSnippet + "\n")
			}
		}
	}

	// 4. Proteksi Bot / Fake Spider & Anti-Scraping bila diaktifkan pada situs
	anyFakeSpider, anyCrawlProt, anyRateLimit := false, false, false
	for _, s := range sites {
		if !s.Status {
			continue
		}
		if s.FakeSpider {
			anyFakeSpider = true
		}
		if s.CrawlProtection {
			anyCrawlProt = true
		}
		if s.RateLimit {
			anyRateLimit = true
		}
	}
	if anyFakeSpider || anyCrawlProt {
		b.WriteString("\n# --- 4. Mitigasi Fake Spider & Scraper Otomatis ---\n")
		if anyFakeSpider {
			b.WriteString(`if ($http_user_agent ~* "(semrushbot|ahrefsbot|mj12bot|dotbot|petalbot|bytespider|megaindex|blexbot|sqlmap|nikto|zgrab|masscan|leakix)") { set $fik_waf_block 1; }` + "\n")
		}
		if anyCrawlProt {
			b.WriteString(`if ($http_user_agent ~* "(httrack|scrapy|libwww-perl|pycurl|mechanize|colly)") { set $fik_waf_block 1; }` + "\n")
		}
	}

	// 5. Signature Proteksi Inti OWASP (SQLi, XSS, Path Traversal, Sensitive Files)
	b.WriteString("\n# --- 5. Proteksi Signature Inti OWASP (SQLi, XSS, LFI, Sensitive Probe) ---\n")
	b.WriteString(`if ($query_string ~* "(union\s+select|information_schema|sleep\(|benchmark\(|load_file\()") { set $fik_waf_block 1; }` + "\n")
	b.WriteString(`if ($query_string ~* "(<script|javascript:|onerror\s*=|onload\s*=)") { set $fik_waf_block 1; }` + "\n")
	b.WriteString(`if ($request_uri ~* "(\.\./|\.\.\\|/etc/passwd|/etc/shadow|/\.env|/\.git|/wp-config\.php)") { set $fik_waf_block 1; }` + "\n")

	b.WriteString(`if ($fik_waf_allow = 1) { set $fik_waf_block 0; }` + "\n")
	b.WriteString(`if ($fik_waf_block = 1) { return 403; }` + "\n\n")

	if anyRateLimit {
		b.WriteString("# --- 6. Smart CC Rate Limiting (Aktif via fikpanel_waf_cc zone) ---\n")
		b.WriteString("limit_req zone=fikpanel_waf_cc burst=40 nodelay;\n\n")
	}

	return b.String()
}

// CountAttachedNginxVhosts menghitung berapa vhost Nginx di server yang benar-benar sudah menyertakan include waf-rules.conf
func CountAttachedNginxVhosts() (attached, total int) {
	if runtime.GOOS != "linux" {
		return 0, 0
	}
	dirs := []string{"/etc/nginx/sites-enabled", "/etc/nginx/conf.d"}
	seen := make(map[string]bool)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), "00-fikpanel-waf") || strings.HasSuffix(e.Name(), ".disabled") || strings.HasSuffix(e.Name(), ".bak") {
				continue
			}
			full := filepath.Join(dir, e.Name())
			realPath, err := filepath.EvalSymlinks(full)
			if err != nil {
				realPath = full
			}
			if seen[realPath] {
				continue
			}
			seen[realPath] = true
			raw, err := os.ReadFile(full)
			if err != nil {
				continue
			}
			text := string(raw)
			if !strings.Contains(text, "server {") && !strings.Contains(text, "server{") {
				continue
			}
			total++
			if strings.Contains(text, "waf-rules.conf") {
				attached++
			}
		}
	}
	return attached, total
}

// AttachAllNginxVhosts menyuntikkan directive include waf-rules.conf ke seluruh vhost Nginx secara aman dengan uji nginx -t & rollback
func (m *Manager) AttachAllNginxVhosts(ctx context.Context) (int, *WAFEnforcementStatus, error) {
	// Pastikan berkas waf-rules.conf dan waf-http.conf terbaru sudah ditulis terlebih dahulu
	if _, err := m.SyncNginxWAFConfig(ctx); err != nil {
		return 0, nil, err
	}
	if runtime.GOOS != "linux" {
		st, _ := m.GetEnforcementStatus(ctx)
		return 0, st, fmt.Errorf("penyuntikan include vhost Nginx hanya tersedia pada host server Linux")
	}

	includeLine := fmt.Sprintf("    include %s;\n", DefaultWAFRulesPath())
	dirs := []string{"/etc/nginx/sites-enabled", "/etc/nginx/conf.d"}
	backups := make(map[string][]byte)
	modified := 0

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), "00-fikpanel-waf") || strings.HasSuffix(e.Name(), ".disabled") || strings.HasSuffix(e.Name(), ".bak") {
				continue
			}
			full := filepath.Join(dir, e.Name())
			raw, err := os.ReadFile(full)
			if err != nil {
				continue
			}
			text := string(raw)
			if strings.Contains(text, "waf-rules.conf") || (!strings.Contains(text, "server {") && !strings.Contains(text, "server{")) {
				continue
			}
			// Sisipkan include tepat setelah baris `server_name ...;` pertama di setiap blok server
			reSrv := regexp.MustCompile(`(?m)^([ \t]*server_name[^\n;]+;[ \t]*\r?\n)`)
			updated := reSrv.ReplaceAllString(text, "${1}"+includeLine)
			if updated == text {
				continue
			}
			backups[full] = raw
			if err := os.WriteFile(full, []byte(updated), 0o644); err == nil {
				modified++
			}
		}
	}

	if modified > 0 {
		if nginxBin, err := exec.LookPath("nginx"); err == nil {
			tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			out, testErr := exec.CommandContext(tCtx, nginxBin, "-t").CombinedOutput()
			cancel()
			if testErr != nil {
				// Rollback otomatis seluruh berkas vhost agar Nginx tidak pernah rusak
				for path, orig := range backups {
					_ = os.WriteFile(path, orig, 0o644)
				}
				st, _ := m.SyncNginxWAFConfig(ctx)
				return 0, st, fmt.Errorf("validasi nginx -t gagal pasca penyuntikan (di-rollback): %s", strings.TrimSpace(string(out)))
			}
			rCtx, rCancel := context.WithTimeout(ctx, 5*time.Second)
			_ = exec.CommandContext(rCtx, nginxBin, "-s", "reload").Run()
			rCancel()
		}
	}

	st, err := m.SyncNginxWAFConfig(ctx)
	return modified, st, err
}

// SyncNginxWAFConfig menulis ulang berkas waf-http.conf dan waf-rules.conf serta melakukan uji nginx -t & reload aman
func (m *Manager) SyncNginxWAFConfig(ctx context.Context) (*WAFEnforcementStatus, error) {
	globalEnabled, siteMap, rules, patchMap := m.getFullPolicySnapshot()
	sites := make([]WAFSite, 0, len(siteMap))
	protectedCount := 0
	for _, s := range siteMap {
		sites = append(sites, s)
		if s.Status {
			protectedCount++
		}
	}

	activePatchCount := 0
	for _, enabled := range patchMap {
		if enabled {
			activePatchCount++
		}
	}

	wafDir := defaultWAFDir()
	_ = os.MkdirAll(wafDir, 0o755)
	cfgPath := filepath.ToSlash(filepath.Join(wafDir, "waf-rules.conf"))
	httpCfgPath := defaultWAFHTTPConfigPath()
	_ = os.MkdirAll(filepath.Dir(httpCfgPath), 0o755)

	httpContent, cfRealIP := BuildNginxWAFHTTPConfig(globalEnabled, sites)
	content := BuildNginxWAFConfig(globalEnabled, rules, sites, patchMap)

	prevRules, hadPrevRules := readBytesIfExist(cfgPath)
	prevHTTP, hadPrevHTTP := readBytesIfExist(httpCfgPath)

	err := os.WriteFile(httpCfgPath, []byte(httpContent), 0o644)
	if err == nil {
		err = os.WriteFile(cfgPath, []byte(content), 0o644)
	}

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
				if hadPrevRules {
					_ = os.WriteFile(cfgPath, prevRules, 0o644)
				}
				if hadPrevHTTP {
					_ = os.WriteFile(httpCfgPath, prevHTTP, 0o644)
				}
			} else {
				rCtx, rCancel := context.WithTimeout(ctx, 4*time.Second)
				_ = exec.CommandContext(rCtx, nginxBin, "-s", "reload").Run()
				rCancel()
			}
		}
	}

	attachedVhosts, totalVhosts := CountAttachedNginxVhosts()
	now := time.Now().Unix()
	st := &WAFEnforcementStatus{
		GlobalEnabled:             globalEnabled,
		NginxEnforced:             nginxEnforced,
		ConfigPath:                cfgPath,
		HTTPConfigPath:            httpCfgPath,
		ActiveRulesCount:          len(rules),
		ActiveVirtualPatchesCount: activePatchCount,
		ProtectedSitesCount:       protectedCount,
		AttachedVhostsCount:       attachedVhosts,
		TotalVhostsCount:          totalVhosts,
		CloudflareRealIPEnabled:   cfRealIP,
		LastSyncedAt:              now,
		LastSyncError:             syncErrStr,
		ConfigPreview:             content,
		HTTPConfigPreview:         httpContent,
	}

	m.mu.Lock()
	m.lastEnforcement = st
	m.mu.Unlock()

	return st, err
}

func readBytesIfExist(path string) ([]byte, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return b, true
}
