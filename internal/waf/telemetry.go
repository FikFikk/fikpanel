package waf

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// GeoLocation merepresentasikan data geolokasi lengkap dari suatu alamat IP
type GeoLocation struct {
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	ISP         string  `json:"isp"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

// ClientTelemetry merepresentasikan identifikasi perangkat dan browser klien penyerang
type ClientTelemetry struct {
	Device  string `json:"device"`
	OS      string `json:"os"`
	Browser string `json:"browser"`
}

// ThreatAssessment merepresentasikan hasil analisis ancaman dari suatu permintaan HTTP
type ThreatAssessment struct {
	IsSuspicious bool
	AttackType   string
	Action       string
	RuleName     string
}

// Cache in-memory untuk menyimpan riwayat lookup GeoIP agar cepat dan tidak boros jaringan
var (
	geoCacheMutex sync.RWMutex
	geoCache      = make(map[string]*GeoLocation)
	httpClient    = &http.Client{Timeout: 3 * time.Second}
)

// ResolveGeoLocation mencari lokasi geografis detail (Negara, Provinsi, Kota, ISP, Lat/Lng) dari alamat IP
func ResolveGeoLocation(ip string) *GeoLocation {
	cleanIP := strings.TrimSpace(ip)
	if h, _, err := net.SplitHostPort(cleanIP); err == nil && h != "" {
		cleanIP = h
	}
	parsedIP := net.ParseIP(cleanIP)
	if cleanIP == "" || parsedIP == nil {
		return &GeoLocation{Country: "Unknown", CountryCode: "XX", Region: "-", City: "-", ISP: "Unresolved"}
	}

	// Tangani IP loopback / jaringan privat / link-local secara presisi
	if parsedIP.IsLoopback() || parsedIP.IsPrivate() || parsedIP.IsLinkLocalUnicast() || parsedIP.IsLinkLocalMulticast() {
		return &GeoLocation{
			Country:     "Local / Private Network",
			CountryCode: "LO",
			Region:      "Internal Network",
			City:        "Local Node",
			ISP:         "Private / Loopback Interface",
			Latitude:    0,
			Longitude:   0,
		}
	}

	// Cek cache memori (maks 4096 entri agar memori terjaga)
	geoCacheMutex.RLock()
	cached, ok := geoCache[cleanIP]
	geoCacheMutex.RUnlock()
	if ok && cached != nil {
		return cached
	}

	// Lookup online via ip-api.com dengan IP yang sudah tervalidasi net.ParseIP
	geo := lookupOnlineGeoIP(parsedIP.String())
	if geo != nil {
		geoCacheMutex.Lock()
		if len(geoCache) > 4096 {
			geoCache = make(map[string]*GeoLocation)
		}
		geoCache[cleanIP] = geo
		geoCacheMutex.Unlock()
		return geo
	}

	// Transparan: jangan memalsukan negara/ISP jika layanan GeoIP publik sedang tidak terjangkau
	return &GeoLocation{
		Country:     "Unresolved Network",
		CountryCode: "XX",
		Region:      "-",
		City:        "Unresolved",
		ISP:         "Unverified ASN",
		Latitude:    0,
		Longitude:   0,
	}
}

// lookupOnlineGeoIP melakukan request HTTP ke endpoint GeoIP publik
func lookupOnlineGeoIP(ip string) *GeoLocation {
	resp, err := httpClient.Get("http://ip-api.com/json/" + url.PathEscape(ip) + "?fields=status,country,countryCode,regionName,city,lat,lon,isp,org")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var data struct {
		Status      string  `json:"status"`
		Country     string  `json:"country"`
		CountryCode string  `json:"countryCode"`
		RegionName  string  `json:"regionName"`
		City        string  `json:"city"`
		Lat         float64 `json:"lat"`
		Lon         float64 `json:"lon"`
		ISP         string  `json:"isp"`
		Org         string  `json:"org"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || data.Status != "success" {
		return nil
	}

	isp := data.ISP
	if isp == "" {
		isp = data.Org
	}
	if isp == "" {
		isp = "Autonomous System"
	}

	return &GeoLocation{
		Country:     data.Country,
		CountryCode: data.CountryCode,
		Region:      data.RegionName,
		City:        data.City,
		ISP:         isp,
		Latitude:    data.Lat,
		Longitude:   data.Lon,
	}
}

// ParseUserAgent mendeteksi perangkat, sistem operasi, dan browser dari header User-Agent
func ParseUserAgent(ua string) ClientTelemetry {
	raw := strings.TrimSpace(ua)
	if raw == "" || raw == "-" {
		return ClientTelemetry{
			Device:  "Automated Scanner",
			OS:      "Non-Interactive",
			Browser: "Direct Socket / Unknown",
		}
	}

	lower := strings.ToLower(raw)

	// 1. Deteksi Perangkat
	device := "Desktop (PC)"
	if strings.Contains(lower, "leakix") || strings.Contains(lower, "l9scan") || strings.Contains(lower, "sqlmap") ||
		strings.Contains(lower, "nikto") || strings.Contains(lower, "nmap") || strings.Contains(lower, "masscan") ||
		strings.Contains(lower, "zgrab") || strings.Contains(lower, "curl") || strings.Contains(lower, "python") ||
		strings.Contains(lower, "go-http-client") || strings.Contains(lower, "axios") || strings.Contains(lower, "bot") ||
		strings.Contains(lower, "spider") || strings.Contains(lower, "crawler") {
		device = "Bot / Automated Scanner"
	} else if strings.Contains(lower, "ipad") || strings.Contains(lower, "tablet") || strings.Contains(lower, "kindle") {
		device = "Tablet"
	} else if strings.Contains(lower, "mobile") || strings.Contains(lower, "iphone") || strings.Contains(lower, "android") {
		device = "Mobile (Smartphone)"
	}

	// 2. Deteksi Sistem Operasi
	os := "Unknown OS"
	if strings.Contains(lower, "windows nt 10.0") || strings.Contains(lower, "windows nt 11.0") {
		os = "Windows 10 / 11"
	} else if strings.Contains(lower, "windows") {
		os = "Windows"
	} else if strings.Contains(lower, "iphone") {
		os = "Apple iOS (iPhone)"
	} else if strings.Contains(lower, "ipad") {
		os = "Apple iPadOS"
	} else if strings.Contains(lower, "macintosh") || strings.Contains(lower, "mac os x") {
		os = "macOS"
	} else if strings.Contains(lower, "android") {
		os = "Google Android"
	} else if strings.Contains(lower, "ubuntu") {
		os = "Ubuntu Linux"
	} else if strings.Contains(lower, "linux") {
		os = "GNU/Linux"
	}

	// 3. Deteksi Peramban / Alat Penyerang
	browser := "Web Browser"
	if strings.Contains(lower, "leakix") || strings.Contains(lower, "l9scan") {
		browser = "LeakIX Vulnerability Scanner"
	} else if strings.Contains(lower, "sqlmap") {
		browser = "sqlmap Injection Engine"
	} else if strings.Contains(lower, "nikto") {
		browser = "Nikto Scanner"
	} else if strings.Contains(lower, "apcosysbot") {
		browser = "ApcosysBot Crawler"
	} else if strings.Contains(lower, "curl/") {
		browser = "cURL Terminal Client"
	} else if strings.Contains(lower, "python-requests") {
		browser = "Python Requests Suite"
	} else if strings.Contains(lower, "go-http-client") {
		browser = "Go HTTP Engine"
	} else if strings.Contains(lower, "axios") {
		browser = "Axios HTTP Client"
	} else if strings.Contains(lower, "edg/") {
		browser = "Microsoft Edge"
	} else if strings.Contains(lower, "yabrowser") {
		browser = "Yandex Browser"
	} else if strings.Contains(lower, "chrome/") {
		browser = "Google Chrome"
	} else if strings.Contains(lower, "firefox/") {
		browser = "Mozilla Firefox"
	} else if strings.Contains(lower, "safari/") && !strings.Contains(lower, "chrome") {
		browser = "Apple Safari"
	}

	return ClientTelemetry{
		Device:  device,
		OS:      os,
		Browser: browser,
	}
}

// Regex pencocokan payload serangan umum
var (
	reSQLi        = regexp.MustCompile(`(?i)(union\s+select|select\s+.*\s+from|'\s*or\s*['\d]=['\d]|sleep\(|benchmark\(|information_schema|load_file\()`)
	reXSS         = regexp.MustCompile(`(?i)(<script|javascript:|onerror\s*=|onload\s*=|alert\(|<img\s+src=)`)
	reLFI         = regexp.MustCompile(`(?i)(\.\./|\.\.\\|/etc/passwd|/etc/shadow|/win\.ini|boot\.ini)`)
	reSensFile    = regexp.MustCompile(`(?i)(/\.env|/\.git|/\.svn|/\.vscode|sftp\.json|/actuator|/health/liveliness|/trace\.axd|/@vite|wp-config\.php|id_rsa|database\.sql|\.bak|\.old|/phpmyadmin|/xmlrpc\.php|/eval-stdin\.php|/vendor/phpunit)`)
	reRCE         = regexp.MustCompile(`(?i)(eval\(|\$\{jndi:|cmd\.exe|/bin/sh|/bin/bash|powershell|base64_decode|system\()`)
	reFakeSpider  = regexp.MustCompile(`(?i)(semrushbot|ahrefsbot|mj12bot|dotbot|petalbot|bytespider|megaindex|blexbot|sogou)`)
	reScraperTool = regexp.MustCompile(`(?i)(httrack|scrapy|wget/|python-urllib|libwww-perl|go-http-client|pycurl|mechanize|colly)`)
)

// MatchIPRule memeriksa apakah clientIP cocok dengan aturan IP tunggal, CIDR, atau rentang StartIP-EndIP
func MatchIPRule(clientIP, startIP, endIP string) bool {
	cIP := net.ParseIP(strings.TrimSpace(clientIP))
	if cIP == nil {
		return false
	}
	sRaw := strings.TrimSpace(startIP)
	eRaw := strings.TrimSpace(endIP)
	if sRaw == "" {
		return false
	}

	// Dukungan notasi CIDR (mis. 203.0.113.0/24 atau 2001:db8::/32)
	if strings.Contains(sRaw, "/") {
		if _, ipNet, err := net.ParseCIDR(sRaw); err == nil && ipNet != nil {
			return ipNet.Contains(cIP)
		}
	}

	sIP := net.ParseIP(sRaw)
	if sIP == nil {
		return false
	}
	if eRaw == "" || eRaw == sRaw {
		return cIP.Equal(sIP)
	}

	eIP := net.ParseIP(eRaw)
	if eIP == nil {
		return cIP.Equal(sIP)
	}

	// Normalisasi ke ukuran byte yang sama (IPv4 4-byte atau IPv6 16-byte)
	if c4, s4, e4 := cIP.To4(), sIP.To4(), eIP.To4(); c4 != nil && s4 != nil && e4 != nil {
		return bytes.Compare(c4, s4) >= 0 && bytes.Compare(c4, e4) <= 0
	}
	c16, s16, e16 := cIP.To16(), sIP.To16(), eIP.To16()
	if c16 != nil && s16 != nil && e16 != nil {
		return bytes.Compare(c16, s16) >= 0 && bytes.Compare(c16, e16) <= 0
	}
	return false
}

// matchPattern memeriksa pencocokan substring case-insensitive atau ekspresi reguler sederhana
func matchPattern(value, pattern string) bool {
	pat := strings.TrimSpace(pattern)
	if pat == "" || value == "" {
		return false
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(pat))
}

// ClassifyThreatWithRules mengevaluasi permintaan terhadap saklar Global WAF, konfigurasi per-situs, aturan Black/White List, dan signature OWASP
func ClassifyThreatWithRules(clientIP, method, path, ua string, statusCode int, globalEnabled bool, siteCfg *WAFSite, rules []WAFRule) ThreatAssessment {
	// 1. Jika WAF Global dimatikan atau proteksi situs ini dinonaktifkan, izinkan semua lalu lintas secara transparan
	if !globalEnabled {
		return ThreatAssessment{
			IsSuspicious: false,
			AttackType:   "Normal Traffic",
			Action:       "allow",
			RuleName:     "Global WAF Dinonaktifkan",
		}
	}
	if siteCfg != nil && !siteCfg.Status {
		return ThreatAssessment{
			IsSuspicious: false,
			AttackType:   "Normal Traffic",
			Action:       "allow",
			RuleName:     "Proteksi WAF Situs Dinonaktifkan",
		}
	}

	// 2. Prioritas Tertinggi: Evaluasi Whitelist (IP -> URL -> UA)
	for _, r := range rules {
		switch r.RuleType {
		case "ip_whitelist":
			if MatchIPRule(clientIP, r.StartIP, r.EndIP) {
				return ThreatAssessment{
					IsSuspicious: false,
					AttackType:   "Whitelisted Traffic",
					Action:       "allow",
					RuleName:     "IP Whitelist: " + r.StartIP,
				}
			}
		case "url_whitelist":
			if matchPattern(path, r.Pattern) {
				return ThreatAssessment{
					IsSuspicious: false,
					AttackType:   "Whitelisted Traffic",
					Action:       "allow",
					RuleName:     "URL Whitelist: " + r.Pattern,
				}
			}
		case "ua_whitelist":
			if matchPattern(ua, r.Pattern) {
				return ThreatAssessment{
					IsSuspicious: false,
					AttackType:   "Whitelisted Traffic",
					Action:       "allow",
					RuleName:     "UA Whitelist: " + r.Pattern,
				}
			}
		}
	}

	// 3. Prioritas Kedua: Evaluasi Blacklist (IP -> URL -> UA)
	for _, r := range rules {
		switch r.RuleType {
		case "ip_blacklist":
			if MatchIPRule(clientIP, r.StartIP, r.EndIP) {
				label := r.StartIP
				if r.EndIP != "" {
					label += "-" + r.EndIP
				}
				return ThreatAssessment{
					IsSuspicious: true,
					AttackType:   "Blocked IP (Blacklist)",
					Action:       "block",
					RuleName:     "Aturan IP Blacklist (" + label + ")",
				}
			}
		case "url_blacklist":
			if matchPattern(path, r.Pattern) {
				return ThreatAssessment{
					IsSuspicious: true,
					AttackType:   "Blocked URL Rule",
					Action:       "block",
					RuleName:     "Aturan URL Blacklist (" + r.Pattern + ")",
				}
			}
		case "ua_blacklist":
			if matchPattern(ua, r.Pattern) {
				return ThreatAssessment{
					IsSuspicious: true,
					AttackType:   "Bad Bot",
					Action:       "block",
					RuleName:     "Aturan UA Blacklist (" + r.Pattern + ")",
				}
			}
		}
	}

	// 4. Evaluasi Proteksi Spesifik Situs (Fake Spider & Crawl Protection)
	if siteCfg != nil {
		if siteCfg.FakeSpider && reFakeSpider.MatchString(ua) {
			return ThreatAssessment{
				IsSuspicious: true,
				AttackType:   "Bad Bot",
				Action:       "block",
				RuleName:     "Proteksi Fake Spider & Bad Crawler Situs",
			}
		}
		if siteCfg.CrawlProtection && reScraperTool.MatchString(ua) {
			return ThreatAssessment{
				IsSuspicious: true,
				AttackType:   "Bad Bot",
				Action:       "block",
				RuleName:     "Crawl Protection (Anti-Scraping) Situs",
			}
		}
	}

	// 5. Evaluasi Signature OWASP Bawaan
	return ClassifyThreat(method, path, ua, statusCode)
}
// ClassifyThreat menganalisis URI, User-Agent, status code, dan payload untuk menentukan kategori serangan
func ClassifyThreat(method, path, ua string, statusCode int) ThreatAssessment {
	p := strings.TrimSpace(path)
	u := strings.TrimSpace(ua)
	m := strings.ToUpper(strings.TrimSpace(method))
	lowerPath := strings.ToLower(p)
	lowerUA := strings.ToLower(u)

	// Deteksi Open Proxy / SOCKS Tunneling Probe (CONNECT atau byte SOCKS5 \x05\x01)
	if m == "CONNECT" || strings.HasPrefix(m, `\X05`) || strings.Contains(p, `\x05\x01`) || strings.Contains(m, `\X16\X03`) {
		return ThreatAssessment{
			IsSuspicious: true,
			AttackType:   "Remote Code Execution (RCE)",
			Action:       "block",
			RuleName:     "Pencegatan Eksploitasi Protokol Biner / Open Proxy Probe",
		}
	}

	// Deteksi RCE / Binary Exploits
	if strings.Contains(p, `\x16\x03\x01`) || reRCE.MatchString(p) {
		return ThreatAssessment{
			IsSuspicious: true,
			AttackType:   "Remote Code Execution (RCE)",
			Action:       "block",
			RuleName:     "Pencegatan Eksploitasi RCE & Protokol TLS Biner",
		}
	}

	// Deteksi SQL Injection
	if reSQLi.MatchString(p) {
		return ThreatAssessment{
			IsSuspicious: true,
			AttackType:   "SQL Injection",
			Action:       "block",
			RuleName:     "Pola SQLi OWASP Dideteksi pada Query String",
		}
	}

	// Deteksi Path Traversal / LFI
	if reLFI.MatchString(p) {
		return ThreatAssessment{
			IsSuspicious: true,
			AttackType:   "Path Traversal",
			Action:       "block",
			RuleName:     "Percobaan Mengakses File Sistem /etc/passwd atau Direktori Induk",
		}
	}

	// Deteksi XSS
	if reXSS.MatchString(p) {
		return ThreatAssessment{
			IsSuspicious: true,
			AttackType:   "XSS Attack",
			Action:       "block",
			RuleName:     "Pencegatan Skrip Jahat Cross-Site Scripting",
		}
	}

	// Deteksi Pemindaian File Sensitif (.env, .git, .svn, sftp.json, trace.axd, actuator, dsb.)
	if reSensFile.MatchString(p) {
		return ThreatAssessment{
			IsSuspicious: true,
			AttackType:   "Sensitive File Probe",
			Action:       "block",
			RuleName:     "Pemindaian File Konfigurasi Rahasia & Repositori Server",
		}
	}

	// Deteksi Bot Pemindai Kelemahan (LeakIX, sqlmap, nikto, zgrab, GenomeCrawlerd, dsb.)
	if strings.Contains(lowerUA, "leakix") || strings.Contains(lowerUA, "l9scan") || strings.Contains(lowerUA, "sqlmap") ||
		strings.Contains(lowerUA, "nikto") || strings.Contains(lowerUA, "masscan") || strings.Contains(lowerUA, "zgrab") ||
		strings.Contains(lowerUA, "genomecrawler") || strings.Contains(lowerUA, "censys") || strings.Contains(lowerUA, "shodan") {
		return ThreatAssessment{
			IsSuspicious: true,
			AttackType:   "Vulnerability Scanner",
			Action:       "intercept",
			RuleName:     "Pemindai Otomatis Botnet / Security Scanner Dideteksi",
		}
	}

	// Status 403 Forbidden pada akses direktori / CMS terlarang
	if statusCode == 403 {
		return ThreatAssessment{
			IsSuspicious: true,
			AttackType:   "Blocked Probe",
			Action:       "block",
			RuleName:     "Akses Ditolak (HTTP 403 Forbidden)",
		}
	}

	// Bila status 404 pada endpoint tak lazim / probing
	if statusCode == 404 && (strings.Contains(lowerPath, "admin") || strings.Contains(lowerPath, "manage") || strings.Contains(lowerPath, "wp-") || strings.Contains(lowerPath, "login") || strings.Contains(lowerPath, ".php") || strings.Contains(lowerPath, "config") || strings.Contains(lowerPath, "health")) {
		return ThreatAssessment{
			IsSuspicious: true,
			AttackType:   "Malicious Probe",
			Action:       "block",
			RuleName:     "Pencarian Titik Masuk CMS / Endpoint Tersembunyi",
		}
	}

	return ThreatAssessment{
		IsSuspicious: false,
		AttackType:   "Normal Traffic",
		Action:       "allow",
		RuleName:     "Lalu Lintas HTTP Valid",
	}
}
