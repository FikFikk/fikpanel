package cloudflare

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/store"
)

// Config menyimpan kredensial dan status integrasi Cloudflare
type Config struct {
	Connected      bool   `json:"connected"`
	APITokenMasked string `json:"api_token_masked"`
	Email          string `json:"email"`
	AccountName    string `json:"account_name"`
	AccountID      string `json:"account_id"`
	AuthMode       string `json:"auth_mode"` // "api_token" atau "global_key"
	LastSync       int64  `json:"last_sync"`
}

// Zone merepresentasikan domain di akun Cloudflare beserta pengaturan edge & status arah DNS
type Zone struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	Status            string      `json:"status"` // 'active', 'pending', 'active (DNS Terdeteksi)'
	Paused            bool        `json:"paused"`
	DevelopmentMode   int         `json:"development_mode"`
	SSLMode           string      `json:"ssl_mode"`            // 'off', 'flexible', 'full', 'strict'
	SecurityLevel     string      `json:"security_level"`      // 'essentially_off', 'low', 'medium', 'high', 'under_attack'
	AlwaysUseHTTPS    bool        `json:"always_use_https"`    // Redirect otomatis HTTP -> HTTPS
	AutoHTTPSRewrites bool        `json:"auto_https_rewrites"` // Rewrite mixed-content HTTP -> HTTPS
	Brotli            bool        `json:"brotli"`              // Kompresi Brotli Edge
	TLS13             bool        `json:"tls_1_3"`             // TLS 1.3 aktif
	NameServers       []string    `json:"name_servers"`
	DNSRecords        []DNSRecord `json:"dns_records"`
	PointsToServer    bool        `json:"points_to_server"`
	MatchedRecords    []string    `json:"matched_records"` // Daftar record yang mengarah ke server ini
	LocalSites        []string    `json:"local_sites"`     // Domain situs FikPanel yang cocok dengan zona ini
	Source            string      `json:"source"`          // 'cloudflare_api' atau 'local_dns_fallback'
	ReadOnly          bool        `json:"read_only"`       // True jika token belum punya akses Zone:Read/Edit penuh
}

// DNSRecord merepresentasikan entri DNS (A, AAAA, CNAME, TXT, MX)
type DNSRecord struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Name           string `json:"name"`
	Content        string `json:"content"`
	Proxied        bool   `json:"proxied"`
	Proxiable      bool   `json:"proxiable"`
	TTL            int    `json:"ttl"`
	Priority       int    `json:"priority,omitempty"`
	PointsToServer bool   `json:"points_to_server"`
}

// DNSRecordInput adalah payload pembuatan atau pembaruan record DNS
type DNSRecordInput struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	Proxied  bool   `json:"proxied"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority,omitempty"`
}

// ZonesResponse memuat daftar zona beserta diagnostik izin token
type ZonesResponse struct {
	Zones           []Zone   `json:"zones"`
	ServerIP        string   `json:"server_ip"`
	AllServerIPs    []string `json:"all_server_ips"`
	APIZonesCount   int      `json:"api_zones_count"`
	TokenDiagnostic string   `json:"token_diagnostic,omitempty"`
}

// Client mengelola panggilan REST API ke Cloudflare v4 endpoint
type Client struct {
	store      *store.Store
	httpClient *http.Client
	mu         sync.RWMutex
}

// Rentang CIDR resmi Cloudflare IPv4 untuk mendeteksi apakah domain diproxy oleh Cloudflare (Orange Cloud)
var cloudflareCIDRs = []string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
}

var parsedCFCIDRs []*net.IPNet

func init() {
	for _, cidr := range cloudflareCIDRs {
		if _, netIP, err := net.ParseCIDR(cidr); err == nil {
			parsedCFCIDRs = append(parsedCFCIDRs, netIP)
		}
	}
}

// isCloudflareEdgeIP memeriksa apakah IP merupakan IP proxy Anycast Cloudflare (Orange Cloud)
func isCloudflareEdgeIP(ipStr string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	for _, netIP := range parsedCFCIDRs {
		if netIP.Contains(ip) {
			return true
		}
	}
	return false
}

// NewClient membuat instance client Cloudflare baru
func NewClient(s *store.Store) *Client {
	return &Client{
		store: s,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// isHexGlobalKey memeriksa apakah string adalah Global API Key Cloudflare (32-40 karakter hex)
func isHexGlobalKey(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 32 || len(s) > 40 {
		return false
	}
	for _, ch := range s {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

// GetConfig membaca konfigurasi Cloudflare dari database
func (c *Client) GetConfig(ctx context.Context) (*Config, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var token, email, accName, accID string
	var connected int
	var updatedAt int64

	err := c.store.DB().QueryRowContext(ctx, `
		SELECT api_token, email, connected, account_name, account_id, updated_at
		FROM cloudflare_configs WHERE id = 1
	`).Scan(&token, &email, &connected, &accName, &accID, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return &Config{Connected: false}, nil
		}
		return nil, err
	}

	masked := ""
	if len(token) > 8 {
		masked = token[:4] + "••••••••" + token[len(token)-4:]
	} else if len(token) > 0 {
		masked = "••••••••"
	}

	authMode := "api_token"
	if email != "" && isHexGlobalKey(token) {
		authMode = "global_key"
	}

	return &Config{
		Connected:      connected == 1,
		APITokenMasked: masked,
		Email:          email,
		AccountName:    accName,
		AccountID:      accID,
		AuthMode:       authMode,
		LastSync:       updatedAt,
	}, nil
}

// getCredentials mengambil token/key dan email dari database
func (c *Client) getCredentials(ctx context.Context) (token string, email string, err error) {
	err = c.store.DB().QueryRowContext(ctx, "SELECT api_token, email FROM cloudflare_configs WHERE id = 1 AND connected = 1").Scan(&token, &email)
	if err != nil {
		return "", "", fmt.Errorf("Cloudflare belum terhubung atau API token tidak ditemukan")
	}
	return strings.TrimSpace(token), strings.TrimSpace(email), nil
}

// applyAuthHeaders menyetel header autentikasi Cloudflare (mendukung API Token Bearer maupun Global API Key + Email)
func applyAuthHeaders(req *http.Request, token, email string) {
	req.Header.Set("Content-Type", "application/json")
	if email != "" && isHexGlobalKey(token) {
		req.Header.Set("X-Auth-Email", email)
		req.Header.Set("X-Auth-Key", token)
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// SaveConfig menyimpan token/key API Cloudflare, memverifikasinya ke Cloudflare API, dan memeriksa akses daftar zona
func (c *Client) SaveConfig(ctx context.Context, apiToken, email string) (*Config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cleanToken := strings.TrimSpace(apiToken)
	cleanEmail := strings.TrimSpace(email)
	if cleanToken == "" {
		return nil, fmt.Errorf("API Token / Global API Key Cloudflare tidak boleh kosong")
	}

	accName := "Cloudflare User"
	accID := ""
	verified := false

	// 1. Jika format Global API Key + Email, verifikasi via /client/v4/user
	if cleanEmail != "" && isHexGlobalKey(cleanToken) {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://api.cloudflare.com/client/v4/user", nil)
		if err == nil {
			req.Header.Set("X-Auth-Email", cleanEmail)
			req.Header.Set("X-Auth-Key", cleanToken)
			req.Header.Set("Content-Type", "application/json")
			if resp, errResp := c.httpClient.Do(req); errResp == nil {
				defer resp.Body.Close()
				var uResp struct {
					Success bool `json:"success"`
					Result  struct {
						ID    string `json:"id"`
						Email string `json:"email"`
					} `json:"result"`
				}
				if json.NewDecoder(resp.Body).Decode(&uResp) == nil && uResp.Success {
					accName = "Global API Key (" + uResp.Result.Email + ")"
					accID = uResp.Result.ID
					verified = true
				}
			}
		}
	}

	// 2. Verifikasi via Bearer API Token (/client/v4/user/tokens/verify atau /client/v4/zones)
	if !verified {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://api.cloudflare.com/client/v4/user/tokens/verify", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+cleanToken)
			req.Header.Set("Content-Type", "application/json")
			resp, errResp := c.httpClient.Do(req)
			if errResp == nil {
				defer resp.Body.Close()
				var vResp struct {
					Success bool `json:"success"`
					Result  struct {
						ID     string `json:"id"`
						Status string `json:"status"`
					} `json:"result"`
					Errors []struct {
						Message string `json:"message"`
					} `json:"errors"`
				}
				if errJson := json.NewDecoder(resp.Body).Decode(&vResp); errJson == nil && vResp.Success {
					accName = "Verified API Token"
					accID = vResp.Result.ID
					verified = true
				} else if len(vResp.Errors) > 0 && cleanEmail == "" {
					return nil, fmt.Errorf("verifikasi Cloudflare gagal: %s", vResp.Errors[0].Message)
				}
			}
		}
	}

	// 3. Cek apakah token dapat membaca nama akun di /client/v4/accounts atau /client/v4/zones
	if verified {
		reqZ, errZ := http.NewRequestWithContext(ctx, "GET", "https://api.cloudflare.com/client/v4/zones?per_page=5", nil)
		if errZ == nil {
			applyAuthHeaders(reqZ, cleanToken, cleanEmail)
			if respZ, errRespZ := c.httpClient.Do(reqZ); errRespZ == nil {
				defer respZ.Body.Close()
				var zResp struct {
					Success bool `json:"success"`
					Result  []struct {
						Account struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"account"`
					} `json:"result"`
				}
				if json.NewDecoder(respZ.Body).Decode(&zResp) == nil && zResp.Success && len(zResp.Result) > 0 {
					if zResp.Result[0].Account.Name != "" {
						accName = zResp.Result[0].Account.Name
					}
				}
			}
		}
	}

	now := time.Now().Unix()
	_, err := c.store.DB().ExecContext(ctx, `
		INSERT INTO cloudflare_configs (id, api_token, email, connected, account_name, account_id, created_at, updated_at)
		VALUES (1, ?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			api_token = excluded.api_token,
			email = excluded.email,
			connected = 1,
			account_name = excluded.account_name,
			account_id = excluded.account_id,
			updated_at = excluded.updated_at
	`, cleanToken, cleanEmail, accName, accID, now, now)

	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan konfigurasi Cloudflare: %w", err)
	}

	masked := cleanToken
	if len(cleanToken) > 8 {
		masked = cleanToken[:4] + "••••••••" + cleanToken[len(cleanToken)-4:]
	}

	authMode := "api_token"
	if cleanEmail != "" && isHexGlobalKey(cleanToken) {
		authMode = "global_key"
	}

	return &Config{
		Connected:      true,
		APITokenMasked: masked,
		Email:          cleanEmail,
		AccountName:    accName,
		AccountID:      accID,
		AuthMode:       authMode,
		LastSync:       now,
	}, nil
}

// Disconnect memutuskan tautan integrasi Cloudflare
func (c *Client) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.store.DB().ExecContext(ctx, "DELETE FROM cloudflare_configs WHERE id = 1")
	return err
}

// ListZonesWithDiagnostics mengambil daftar seluruh zona dari API Cloudflare, melengkapi pengaturan riil,
// serta melakukan fallback penemuan DNS lokal otomatis bila API Token belum diberi izin Zone:Read.
func (c *Client) ListZonesWithDiagnostics(ctx context.Context, serverIPs []string) (*ZonesResponse, error) {
	token, email, err := c.getCredentials(ctx)
	if err != nil {
		return nil, err
	}

	primaryIP := ""
	ownIPMap := make(map[string]bool)
	for _, ip := range serverIPs {
		clean := strings.TrimSpace(ip)
		if clean != "" {
			if primaryIP == "" {
				primaryIP = clean
			}
			ownIPMap[clean] = true
		}
	}

	// Ambil daftar situs yang terdaftar di FikPanel untuk pemetaan silang
	localDomains := c.getLocalSiteDomains(ctx)

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.cloudflare.com/client/v4/zones?per_page=50", nil)
	if err != nil {
		return nil, err
	}
	applyAuthHeaders(req, token, email)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("koneksi ke Cloudflare gagal: %w", err)
	}
	defer resp.Body.Close()

	var cfResp struct {
		Success bool `json:"success"`
		Result  []struct {
			ID              string   `json:"id"`
			Name            string   `json:"name"`
			Status          string   `json:"status"`
			Paused          bool     `json:"paused"`
			DevelopmentMode int      `json:"development_mode"`
			NameServers     []string `json:"name_servers"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, fmt.Errorf("gagal membaca respon Cloudflare: %w", err)
	}
	if !cfResp.Success && len(cfResp.Errors) > 0 {
		return nil, fmt.Errorf("Cloudflare API error: %s", cfResp.Errors[0].Message)
	}

	out := &ZonesResponse{
		Zones:        []Zone{},
		ServerIP:     primaryIP,
		AllServerIPs: serverIPs,
		APIZonesCount: len(cfResp.Result),
	}

	for _, z := range cfResp.Result {
		zoneItem := Zone{
			ID:              z.ID,
			Name:            z.Name,
			Status:          z.Status,
			Paused:          z.Paused,
			DevelopmentMode: z.DevelopmentMode,
			SSLMode:         "full",
			SecurityLevel:   "medium",
			AlwaysUseHTTPS:  true,
			Brotli:          true,
			TLS13:           true,
			NameServers:     z.NameServers,
			Source:          "cloudflare_api",
			MatchedRecords:  []string{},
			LocalSites:      matchLocalSitesForZone(z.Name, localDomains),
		}

		// Ambil pengaturan zona riil dari Cloudflare API (/zones/{id}/settings)
		c.populateZoneSettings(ctx, token, email, &zoneItem)

		// Ambil rekaman DNS untuk domain ini
		dnsRecords, _ := c.listDNSRecordsWithIPMap(ctx, z.ID, ownIPMap)
		zoneItem.DNSRecords = dnsRecords

		for _, r := range dnsRecords {
			if r.PointsToServer {
				zoneItem.PointsToServer = true
				zoneItem.MatchedRecords = append(zoneItem.MatchedRecords, fmt.Sprintf("%s (%s → %s)", r.Name, r.Type, r.Content))
			}
		}

		out.Zones = append(out.Zones, zoneItem)
	}

	// Jika API Cloudflare mengembalikan 0 zona (karena token belum diberi izin Zone -> Zone -> Read atau Zone Resources: All Zones),
	// pindai otomatis domain situs lokal di FikPanel yang memakai Cloudflare Nameserver / Proxy!
	if len(out.Zones) == 0 {
		out.TokenDiagnostic = "API Token Anda terverifikasi aktif, namun endpoint Cloudflare `/client/v4/zones` mengembalikan 0 domain. Hal ini terjadi karena saat membuat API Token di Cloudflare: (1) izin `Zone → Zone → Read` belum ditambahkan (hanya `DNS` saja tidak menampilkan daftar zona), atau (2) bagian `Zone Resources` belum disetel ke `Include → All zones`. Untuk dapat mengubah pengaturan dari FikPanel (Point DNS 1-Klik, Mode SSL, Under Attack Mode, & Purge Cache), pastikan Token memiliki izin: `Zone → Zone → Read`, `Zone → DNS → Edit`, `Zone → Zone Settings → Edit`, dan `Zone → Cache Purge → Purge` (atau gunakan Global API Key + Email)."
		discovered := c.discoverLocalCloudflareZones(ctx, localDomains, ownIPMap, primaryIP)
		out.Zones = append(out.Zones, discovered...)
	}

	return out, nil
}

// ListZones mempertahankan tanda tangan fungsi lama untuk kompatibilitas
func (c *Client) ListZones(ctx context.Context, serverIP string) ([]Zone, error) {
	ips := []string{}
	if strings.TrimSpace(serverIP) != "" {
		ips = append(ips, strings.TrimSpace(serverIP))
	}
	res, err := c.ListZonesWithDiagnostics(ctx, ips)
	if err != nil {
		return nil, err
	}
	return res.Zones, nil
}

// populateZoneSettings mengambil pengaturan riil SSL, Security Level, Always Use HTTPS, Brotli, dll. dari Cloudflare API
func (c *Client) populateZoneSettings(ctx context.Context, token, email string, zone *Zone) {
	if zone == nil || zone.ID == "" || strings.HasPrefix(zone.ID, "local-") {
		return
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/settings", zone.ID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}
	applyAuthHeaders(req, token, email)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var sResp struct {
		Success bool `json:"success"`
		Result  []struct {
			ID    string      `json:"id"`
			Value interface{} `json:"value"`
		} `json:"result"`
	}
	if json.NewDecoder(resp.Body).Decode(&sResp) != nil || !sResp.Success {
		return
	}

	for _, item := range sResp.Result {
		valStr, _ := item.Value.(string)
		switch item.ID {
		case "ssl":
			if valStr != "" {
				zone.SSLMode = valStr
			}
		case "security_level":
			if valStr != "" {
				zone.SecurityLevel = valStr
			}
		case "always_use_https":
			zone.AlwaysUseHTTPS = valStr == "on"
		case "automatic_https_rewrites":
			zone.AutoHTTPSRewrites = valStr == "on"
		case "brotli":
			zone.Brotli = valStr == "on"
		case "tls_1_3":
			zone.TLS13 = valStr == "on" || valStr == "zrt"
		case "development_mode":
			if valStr == "on" {
				zone.DevelopmentMode = 1
			} else if valStr == "off" {
				zone.DevelopmentMode = 0
			}
		}
	}
}

// getLocalSiteDomains membaca daftar domain dan alias yang terdaftar pada tabel sites di FikPanel
func (c *Client) getLocalSiteDomains(ctx context.Context) []string {
	rows, err := c.store.DB().QueryContext(ctx, "SELECT domain, aliases FROM sites")
	if err != nil {
		return nil
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var list []string
	for rows.Next() {
		var d, a string
		if err := rows.Scan(&d, &a); err == nil {
			d = strings.ToLower(strings.TrimSpace(d))
			if d != "" && !seen[d] {
				seen[d] = true
				list = append(list, d)
			}
			for _, part := range strings.Split(a, ",") {
				part = strings.ToLower(strings.TrimSpace(part))
				if part != "" && !seen[part] {
					seen[part] = true
					list = append(list, part)
				}
			}
		}
	}
	return list
}

// extractRegistrableRoot mengekstrak root domain dari subdomain (misal "yuha.fikfikk.my.id" -> "fikfikk.my.id")
func extractRegistrableRoot(domain string) string {
	d := strings.ToLower(strings.Trim(strings.TrimSpace(domain), "."))
	parts := strings.Split(d, ".")
	if len(parts) <= 2 {
		return d
	}
	// TLD tingkat dua populer (.my.id, .co.id, .ac.id, .or.id, .web.id, .biz.id, .co.uk, .com.sg, .com.au)
	secondLevelTLDs := map[string]bool{
		"my.id": true, "co.id": true, "ac.id": true, "or.id": true, "web.id": true, "biz.id": true, "sch.id": true, "go.id": true,
		"co.uk": true, "org.uk": true, "com.sg": true, "com.au": true, "com.my": true, "net.id": true,
	}
	lastTwo := parts[len(parts)-2] + "." + parts[len(parts)-1]
	if secondLevelTLDs[lastTwo] && len(parts) >= 3 {
		return parts[len(parts)-3] + "." + lastTwo
	}
	return lastTwo
}

// matchLocalSitesForZone mengembalikan daftar domain situs FikPanel yang berada di bawah zona Cloudflare ini
func matchLocalSitesForZone(zoneName string, localDomains []string) []string {
	zn := strings.ToLower(strings.TrimSpace(zoneName))
	var matched []string
	for _, d := range localDomains {
		if d == zn || strings.HasSuffix(d, "."+zn) {
			matched = append(matched, d)
		}
	}
	return matched
}

// discoverLocalCloudflareZones memindai DNS langsung untuk situs-situs FikPanel ketika API Token belum memiliki izin Zone:Read
func (c *Client) discoverLocalCloudflareZones(ctx context.Context, localDomains []string, ownIPMap map[string]bool, primaryServerIP string) []Zone {
	zoneGroups := make(map[string][]string)
	for _, d := range localDomains {
		root := extractRegistrableRoot(d)
		if root == "" || net.ParseIP(root) != nil {
			continue
		}
		zoneGroups[root] = append(zoneGroups[root], d)
	}

	var discovered []Zone
	for rootZone, subDomains := range zoneGroups {
		lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		nsList, _ := net.DefaultResolver.LookupNS(lookupCtx, rootZone)
		cancel()

		var cfNS []string
		isCF := false
		for _, ns := range nsList {
			host := strings.TrimSuffix(strings.ToLower(ns.Host), ".")
			cfNS = append(cfNS, host)
			if strings.Contains(host, "cloudflare.com") {
				isCF = true
			}
		}

		// Kandidat host yang akan diperiksa record A/AAAA-nya
		hostsToProbe := []string{rootZone, "www." + rootZone}
		seenHost := map[string]bool{rootZone: true, "www." + rootZone: true}
		for _, sd := range subDomains {
			if !seenHost[sd] {
				seenHost[sd] = true
				hostsToProbe = append(hostsToProbe, sd)
			}
		}

		var records []DNSRecord
		var matchedRecs []string
		pointsHere := false

		for idx, h := range hostsToProbe {
			hCtx, hCancel := context.WithTimeout(ctx, 2500*time.Millisecond)
			ips, err := net.DefaultResolver.LookupHost(hCtx, h)
			hCancel()
			if err != nil || len(ips) == 0 {
				continue
			}

			for ipIdx, ipStr := range ips {
				proxied := isCloudflareEdgeIP(ipStr)
				if proxied {
					isCF = true
				}
				recPointsHere := ownIPMap[ipStr]
				// Jika domain ini adalah situs aktif di server ini dan diproxy oleh Cloudflare (Orange Cloud),
				// tandai bahwa domain ini dikelola oleh server ini melalui Cloudflare Proxy
				isLocalSite := false
				for _, sd := range subDomains {
					if sd == h {
						isLocalSite = true
						break
					}
				}
				if recPointsHere || (proxied && isLocalSite) {
					pointsHere = true
					recPointsHere = true
				}

				recType := "A"
				if strings.Contains(ipStr, ":") {
					recType = "AAAA"
				}
				records = append(records, DNSRecord{
					ID:             fmt.Sprintf("dns-live-%d-%d", idx, ipIdx),
					Type:           recType,
					Name:           h,
					Content:        ipStr,
					Proxied:        proxied,
					Proxiable:      true,
					TTL:            1,
					PointsToServer: recPointsHere,
				})
				if recPointsHere {
					label := fmt.Sprintf("%s → %s", h, ipStr)
					if proxied && primaryServerIP != "" {
						label = fmt.Sprintf("%s → Cloudflare Proxy (%s)", h, ipStr)
					}
					matchedRecs = append(matchedRecs, label)
				}
				// Cukup tampilkan 1 IP representatif jika diproxy Cloudflare (karena Cloudflare mengembalikan 2 Anycast IP)
				if proxied {
					break
				}
			}
		}

		if !isCF && len(records) == 0 {
			continue
		}

		statusLabel := "active"
		if !isCF {
			statusLabel = "dns-external"
		}

		discovered = append(discovered, Zone{
			ID:                "local-" + rootZone,
			Name:              rootZone,
			Status:            statusLabel,
			Paused:            false,
			DevelopmentMode:   0,
			SSLMode:           "full",
			SecurityLevel:     "medium",
			AlwaysUseHTTPS:    true,
			AutoHTTPSRewrites: true,
			Brotli:            true,
			TLS13:             true,
			NameServers:       cfNS,
			DNSRecords:        records,
			PointsToServer:    pointsHere,
			MatchedRecords:    matchedRecs,
			LocalSites:        subDomains,
			Source:            "local_dns_fallback",
			ReadOnly:          true,
		})
	}

	return discovered
}

// ListDNSRecords mengambil rekaman DNS spesifik dari satu zone
func (c *Client) ListDNSRecords(ctx context.Context, zoneID, serverIP string) ([]DNSRecord, error) {
	ownIPMap := make(map[string]bool)
	for _, ip := range strings.Split(serverIP, ",") {
		if clean := strings.TrimSpace(ip); clean != "" {
			ownIPMap[clean] = true
		}
	}
	return c.listDNSRecordsWithIPMap(ctx, zoneID, ownIPMap)
}

func (c *Client) listDNSRecordsWithIPMap(ctx context.Context, zoneID string, ownIPMap map[string]bool) ([]DNSRecord, error) {
	token, email, err := c.getCredentials(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?per_page=100", zoneID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	applyAuthHeaders(req, token, email)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var cfResp struct {
		Success bool `json:"success"`
		Result  []struct {
			ID        string `json:"id"`
			Type      string `json:"type"`
			Name      string `json:"name"`
			Content   string `json:"content"`
			Proxied   bool   `json:"proxied"`
			Proxiable bool   `json:"proxiable"`
			TTL       int    `json:"ttl"`
			Priority  int    `json:"priority"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, err
	}

	// Petakan domain yang sudah mengarah ke server ini (untuk melacak CNAME yang merujuk ke domain server ini)
	pointedNames := make(map[string]bool)
	for _, r := range cfResp.Result {
		if ownIPMap[strings.TrimSpace(r.Content)] {
			pointedNames[strings.ToLower(strings.TrimSpace(r.Name))] = true
		}
	}

	var records []DNSRecord
	for _, r := range cfResp.Result {
		contentClean := strings.TrimSpace(r.Content)
		pointsHere := ownIPMap[contentClean]
		if !pointsHere && r.Type == "CNAME" && pointedNames[strings.ToLower(contentClean)] {
			pointsHere = true
		}

		records = append(records, DNSRecord{
			ID:             r.ID,
			Type:           r.Type,
			Name:           r.Name,
			Content:        r.Content,
			Proxied:        r.Proxied,
			Proxiable:      r.Proxiable,
			TTL:            r.TTL,
			Priority:       r.Priority,
			PointsToServer: pointsHere,
		})
	}

	return records, nil
}

// SaveDNSRecord membuat record DNS baru (jika ID kosong) atau memperbarui record DNS yang sudah ada di Cloudflare
func (c *Client) SaveDNSRecord(ctx context.Context, zoneID string, input DNSRecordInput) (*DNSRecord, error) {
	if strings.HasPrefix(zoneID, "local-") {
		return nil, fmt.Errorf("tidak dapat menulis ke Cloudflare karena API Token Anda belum memiliki izin 'Zone → Zone → Read' & 'Zone → DNS → Edit'. Silakan perbarui izin Token di Cloudflare terlebih dahulu")
	}

	token, email, err := c.getCredentials(ctx)
	if err != nil {
		return nil, err
	}

	recType := strings.ToUpper(strings.TrimSpace(input.Type))
	if recType == "" {
		recType = "A"
	}
	name := strings.TrimSpace(input.Name)
	content := strings.TrimSpace(input.Content)
	if name == "" || content == "" {
		return nil, fmt.Errorf("nama host dan target content wajib diisi")
	}
	ttl := input.TTL
	if ttl <= 0 {
		ttl = 1 // Auto TTL
	}

	payload := map[string]interface{}{
		"type":    recType,
		"name":    name,
		"content": content,
		"ttl":     ttl,
	}
	if recType == "A" || recType == "AAAA" || recType == "CNAME" {
		payload["proxied"] = input.Proxied
	}
	if recType == "MX" && input.Priority >= 0 {
		payload["priority"] = input.Priority
	}

	bodyData, _ := json.Marshal(payload)
	var req *http.Request
	if strings.TrimSpace(input.ID) != "" && !strings.HasPrefix(input.ID, "dns-live-") {
		url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, strings.TrimSpace(input.ID))
		req, err = http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(bodyData))
	} else {
		url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
		req, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyData))
	}
	if err != nil {
		return nil, err
	}
	applyAuthHeaders(req, token, email)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var cfResp struct {
		Success bool `json:"success"`
		Result  struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Name    string `json:"name"`
			Content string `json:"content"`
			Proxied bool   `json:"proxied"`
			TTL     int    `json:"ttl"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, fmt.Errorf("gagal mem-parsing respons Cloudflare: %w", err)
	}
	if !cfResp.Success {
		msg := "ditolak oleh Cloudflare API (pastikan Token memiliki izin Zone → DNS → Edit)"
		if len(cfResp.Errors) > 0 {
			msg = cfResp.Errors[0].Message
		}
		return nil, fmt.Errorf("gagal menyimpan DNS record: %s", msg)
	}

	return &DNSRecord{
		ID:      cfResp.Result.ID,
		Type:    cfResp.Result.Type,
		Name:    cfResp.Result.Name,
		Content: cfResp.Result.Content,
		Proxied: cfResp.Result.Proxied,
		TTL:     cfResp.Result.TTL,
	}, nil
}

// DeleteDNSRecord menghapus satu entri DNS dari zona Cloudflare
func (c *Client) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	if strings.HasPrefix(zoneID, "local-") || strings.HasPrefix(recordID, "dns-live-") {
		return fmt.Errorf("tidak dapat menghapus record karena API Token belum memiliki izin 'Zone → Zone → Read' & 'Zone → DNS → Edit'")
	}

	token, email, err := c.getCredentials(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, recordID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	applyAuthHeaders(req, token, email)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gagal menghapus DNS record di Cloudflare: %s", string(respBody))
	}
	return nil
}

// PointZoneToServer mengarahkan record A domain (serta subdomain situs lokal terkait) ke IP server ini secara otomatis
func (c *Client) PointZoneToServer(ctx context.Context, zoneID, domain, serverIP string, proxied bool) error {
	if strings.HasPrefix(zoneID, "local-") {
		return fmt.Errorf("API Token Anda belum memiliki izin 'Zone → Zone → Read' & 'Zone → DNS → Edit'. Perbarui izin Token di Cloudflare untuk mengarahkan DNS secara otomatis")
	}

	cleanIP := strings.TrimSpace(serverIP)
	if cleanIP == "" {
		return fmt.Errorf("IP publik server tidak valid")
	}

	records, err := c.ListDNSRecords(ctx, zoneID, cleanIP)
	if err != nil {
		return err
	}

	var existingID string
	for _, r := range records {
		if r.Type == "A" && (strings.EqualFold(r.Name, domain) || r.Name == "@") {
			existingID = r.ID
			break
		}
	}

	_, err = c.SaveDNSRecord(ctx, zoneID, DNSRecordInput{
		ID:      existingID,
		Type:    "A",
		Name:    domain,
		Content: cleanIP,
		Proxied: proxied,
		TTL:     1,
	})
	return err
}

// UpdateZoneSetting memperbarui pengaturan zona Cloudflare (ssl, security_level, always_use_https, automatic_https_rewrites, brotli, tls_1_3, development_mode)
func (c *Client) UpdateZoneSetting(ctx context.Context, zoneID, settingKey, value string) error {
	if strings.HasPrefix(zoneID, "local-") {
		return fmt.Errorf("API Token Anda belum memiliki izin 'Zone → Zone → Read' & 'Zone → Zone Settings → Edit'. Perbarui izin Token di Cloudflare terlebih dahulu")
	}

	allowedKeys := map[string]bool{
		"ssl":                      true,
		"security_level":           true,
		"always_use_https":         true,
		"automatic_https_rewrites": true,
		"brotli":                   true,
		"tls_1_3":                  true,
		"development_mode":         true,
	}
	if !allowedKeys[settingKey] {
		return fmt.Errorf("pengaturan zona '%s' tidak didukung", settingKey)
	}

	token, email, err := c.getCredentials(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/settings/%s", zoneID, settingKey)
	payload := map[string]interface{}{
		"value": value,
	}
	bodyData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(bodyData))
	if err != nil {
		return err
	}
	applyAuthHeaders(req, token, email)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var cfResp struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&cfResp)
	if resp.StatusCode >= 400 || !cfResp.Success {
		errMsg := fmt.Sprintf("HTTP %d (pastikan Token memiliki izin Zone → Zone Settings → Edit)", resp.StatusCode)
		if len(cfResp.Errors) > 0 {
			errMsg = cfResp.Errors[0].Message + " (pastikan Token memiliki izin Zone → Zone Settings → Edit)"
		}
		return fmt.Errorf("gagal mengubah pengaturan %s: %s", settingKey, errMsg)
	}

	return nil
}

// PurgeZoneCache mengosongkan cache CDN Cloudflare untuk seluruh domain
func (c *Client) PurgeZoneCache(ctx context.Context, zoneID string) error {
	if strings.HasPrefix(zoneID, "local-") {
		return fmt.Errorf("API Token Anda belum memiliki izin 'Zone → Zone → Read' & 'Zone → Cache Purge → Purge'")
	}

	token, email, err := c.getCredentials(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/purge_cache", zoneID)
	payload := map[string]interface{}{
		"purge_everything": true,
	}
	bodyData, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyData))
	if err != nil {
		return err
	}
	applyAuthHeaders(req, token, email)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("gagal mengosongkan cache Cloudflare (HTTP %d — pastikan Token memiliki izin Zone → Cache Purge → Purge)", resp.StatusCode)
	}

	return nil
}

// ToggleDevelopmentMode mengaktifkan atau menonaktifkan mode pengembang (bypass cache) selama 3 jam
func (c *Client) ToggleDevelopmentMode(ctx context.Context, zoneID string, enable bool) error {
	val := "off"
	if enable {
		val = "on"
	}
	return c.UpdateZoneSetting(ctx, zoneID, "development_mode", val)
}
