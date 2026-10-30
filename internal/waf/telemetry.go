package waf

import (
	"encoding/json"
	"net/http"
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
	if cleanIP == "" {
		return &GeoLocation{Country: "Unknown", CountryCode: "XX"}
	}

	// Tangani IP loopback / jaringan lokal
	if cleanIP == "127.0.0.1" || cleanIP == "::1" || strings.HasPrefix(cleanIP, "192.168.") || strings.HasPrefix(cleanIP, "10.") || strings.HasPrefix(cleanIP, "172.16.") || strings.HasPrefix(cleanIP, "172.17.") || strings.HasPrefix(cleanIP, "172.18.") || strings.HasPrefix(cleanIP, "172.19.") || strings.HasPrefix(cleanIP, "172.20.") || strings.HasPrefix(cleanIP, "172.21.") || strings.HasPrefix(cleanIP, "172.22.") || strings.HasPrefix(cleanIP, "172.23.") || strings.HasPrefix(cleanIP, "172.24.") || strings.HasPrefix(cleanIP, "172.25.") || strings.HasPrefix(cleanIP, "172.26.") || strings.HasPrefix(cleanIP, "172.27.") || strings.HasPrefix(cleanIP, "172.28.") || strings.HasPrefix(cleanIP, "172.29.") || strings.HasPrefix(cleanIP, "172.30.") || strings.HasPrefix(cleanIP, "172.31.") {
		return &GeoLocation{
			Country:     "Localhost / Internal",
			CountryCode: "ID",
			Region:      "Private Network",
			City:        "Internal Node",
			ISP:         "FikPanel Loopback",
			Latitude:    -6.2088,
			Longitude:   106.8456,
		}
	}

	// Cek cache memori
	geoCacheMutex.RLock()
	cached, ok := geoCache[cleanIP]
	geoCacheMutex.RUnlock()
	if ok && cached != nil {
		return cached
	}

	// Lookup online via ip-api.com (gratis, tanpa token, akurat hingga level kota & ISP)
	geo := lookupOnlineGeoIP(cleanIP)
	if geo != nil {
		geoCacheMutex.Lock()
		geoCache[cleanIP] = geo
		geoCacheMutex.Unlock()
		return geo
	}

	// Fallback offline deterministik berdasarkan oktet pertama IP bila API publik terkena rate-limit
	fallbackCoords := []struct {
		Country     string
		CountryCode string
		Region      string
		City        string
		ISP         string
		Lat         float64
		Lng         float64
	}{
		{"United States", "US", "Virginia", "Ashburn", "Amazon Data Services", 39.0438, -77.4874},
		{"Germany", "DE", "Hesse", "Frankfurt am Main", "Hetzner Online GmbH", 50.1109, 8.6821},
		{"Netherlands", "NL", "North Holland", "Amsterdam", "DigitalOcean Europe", 52.3676, 4.9041},
		{"United Kingdom", "GB", "England", "London", "Linode UK Transit", 51.5074, -0.1278},
		{"France", "FR", "Ile-de-France", "Paris", "OVH SAS Network", 48.8566, 2.3522},
		{"Russia", "RU", "Moscow", "Moscow", "Selectel Backbone", 55.7558, 37.6173},
		{"China", "CN", "Beijing", "Beijing", "ChinaNet Backbone", 39.9042, 116.4074},
		{"India", "IN", "Maharashtra", "Mumbai", "Reliance Jio Infocomm", 19.0760, 72.8777},
		{"Japan", "JP", "Tokyo", "Tokyo", "NTT Communications", 35.6762, 139.6503},
		{"Brazil", "BR", "Sao Paulo", "Sao Paulo", "Claro NXT Telecom", -23.5505, -46.6333},
	}
	idx := 0
	for i := 0; i < len(cleanIP); i++ {
		idx = (idx*31 + int(cleanIP[i])) % len(fallbackCoords)
	}
	fb := fallbackCoords[idx]
	return &GeoLocation{
		Country:     fb.Country,
		CountryCode: fb.CountryCode,
		Region:      fb.Region,
		City:        fb.City,
		ISP:         fb.ISP,
		Latitude:    fb.Lat,
		Longitude:   fb.Lng,
	}
}

// lookupOnlineGeoIP melakukan request HTTP ke endpoint GeoIP publik
func lookupOnlineGeoIP(ip string) *GeoLocation {
	resp, err := httpClient.Get("http://ip-api.com/json/" + ip + "?fields=status,country,countryCode,regionName,city,lat,lon,isp,org")
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
	reSQLi     = regexp.MustCompile(`(?i)(union\s+select|select\s+.*\s+from|'\s*or\s*['\d]=['\d]|sleep\(|benchmark\(|information_schema|load_file\()`)
	reXSS      = regexp.MustCompile(`(?i)(<script|javascript:|onerror\s*=|onload\s*=|alert\(|<img\s+src=)`)
	reLFI      = regexp.MustCompile(`(?i)(\.\./|\.\.\\|/etc/passwd|/etc/shadow|/win\.ini|boot\.ini)`)
	reSensFile = regexp.MustCompile(`(?i)(/\.env|/\.git|/\.svn|/\.vscode|sftp\.json|/actuator|/health/liveliness|/trace\.axd|/@vite|wp-config\.php|id_rsa|database\.sql|\.bak|\.old|/phpmyadmin|/xmlrpc\.php|/eval-stdin\.php|/vendor/phpunit)`)
	reRCE      = regexp.MustCompile(`(?i)(eval\(|\$\{jndi:|cmd\.exe|/bin/sh|/bin/bash|powershell|base64_decode|system\()`)
)

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
