package waf

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/store"
)

// WAFSite merepresentasikan konfigurasi proteksi WAF pada satu virtual host situs
type WAFSite struct {
	SiteName            string `json:"site_name"`
	TotalInterceptions  int    `json:"total_interceptions"`
	FakeSpider          bool   `json:"fake_spider"`
	UsingCDN            bool   `json:"using_cdn"`
	SmartCC             int    `json:"smart_cc"`
	CrawlProtection     bool   `json:"crawl_protection"`
	RateLimit           bool   `json:"rate_limit"`
	CCDefense           int    `json:"cc_defense"`
	Status              bool   `json:"status"`
	UpdatedAt           int64  `json:"updated_at"`
}

// WAFRule merepresentasikan aturan filter IP, User-Agent, atau URL
type WAFRule struct {
	ID        int64  `json:"id"`
	RuleType  string `json:"rule_type"` // 'ip_whitelist', 'ip_blacklist', 'ua_whitelist', 'ua_blacklist', 'url_whitelist', 'url_blacklist'
	IPVersion string `json:"ip_version"`
	StartIP   string `json:"start_ip"`
	EndIP     string `json:"end_ip"`
	Pattern   string `json:"pattern"`
	Remarks   string `json:"remarks"`
	CreatedAt int64  `json:"created_at"`
}

// WAFEvent merepresentasikan satu insiden penangkapan serangan oleh WAF dengan telemetri lengkap
type WAFEvent struct {
	ID          int64   `json:"id"`
	SiteName    string  `json:"site_name"`
	ClientIP    string  `json:"client_ip"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	ISP         string  `json:"isp"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Device      string  `json:"device"`
	OS          string  `json:"os"`
	Browser     string  `json:"browser"`
	Method      string  `json:"method"`
	AttackType  string  `json:"attack_type"`
	Action      string  `json:"action"`
	URLPath     string  `json:"url_path"`
	UserAgent   string  `json:"user_agent"`
	RuleName    string  `json:"rule_name"`
	StatusCode  int     `json:"status_code"`
	CreatedAt   int64   `json:"created_at"`
}

// StatusBreakdown merepresentasikan distribusi kode respons HTTP di seluruh situs
type StatusBreakdown struct {
	S2xx int64 `json:"s2xx"`
	S3xx int64 `json:"s3xx"`
	S4xx int64 `json:"s4xx"`
	S5xx int64 `json:"s5xx"`
}

// VisitorItem merepresentasikan entitas IP pengunjung/pengakses paling aktif beserta atribut perangkat dan ancamannya
type VisitorItem struct {
	IP          string `json:"ip"`
	Count       int    `json:"count"`
	Requests    int    `json:"requests"`
	BytesTotal  int64  `json:"bytes_total"`
	BytesSent   int64  `json:"bytes_sent"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
	ISP         string `json:"isp"`
	Device      string `json:"device"`
	OS          string `json:"os"`
	Browser     string `json:"browser"`
	ThreatCount int    `json:"threat_count"`
	Threats     int    `json:"threats"`
	LastSite    string `json:"last_site"`
	LastPath    string `json:"last_path"`
	LastSeen    int64  `json:"last_seen"`
}

// WAFOverview merepresentasikan ringkasan metrik dashboard WAF dan observabilitas lalu lintas HTTP
type WAFOverview struct {
	TodayRequests        int64                 `json:"today_requests"`
	MaliciousRequests    int64                 `json:"malicious_requests"`
	YesterdayRequests    int64                 `json:"yesterday_requests"`
	YesterdayMalicious   int64                 `json:"yesterday_malicious"`
	TotalRequestsAllTime int64                 `json:"total_requests_all_time"`
	TotalBandwidthBytes  int64                 `json:"total_bandwidth_bytes"`
	StatusBreakdown      StatusBreakdown       `json:"status_breakdown"`
	InterceptRate        float64               `json:"intercept_rate"`
	RealtimeQPS          float64               `json:"realtime_qps"`
	OriginLatencyMs      int                   `json:"origin_latency_ms"`
	TopAttackIPs         []AttackIPItem        `json:"top_attack_ips"`
	AttackedDomains      []AttackedDomain      `json:"attacked_domains"`
	TrafficRanking       []TrafficRankItem     `json:"traffic_ranking"`
	VisitedPages         []VisitedPageItem     `json:"visited_pages"`
	TopVisitors          []VisitorItem         `json:"top_visitors"`
	RecentEvents         []WAFEvent            `json:"recent_events"`
	RecentAccessLogs     []WAFAccessLog        `json:"recent_access_logs"`
	TotalRulesCount      int                   `json:"total_rules_count"`
	WAFGlobalEnabled     bool                  `json:"waf_global_enabled"`
	HostLocation         HostLocation          `json:"host_location"`
	Enforcement          *WAFEnforcementStatus `json:"enforcement,omitempty"`
}

type AttackIPItem struct {
	IP          string `json:"ip"`
	Count       int    `json:"count"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Region      string `json:"region"`
	City        string `json:"city"`
	ISP         string `json:"isp"`
}

type AttackedDomain struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

type TrafficRankItem struct {
	Domain         string `json:"domain"`
	Requests       int    `json:"requests"`
	BandwidthBytes int64  `json:"bandwidth_bytes"`
	BytesSent      int64  `json:"bytes_sent"`
	Threats        int    `json:"threats"`
	Status2xx      int    `json:"status_2xx"`
	Status3xx      int    `json:"status_3xx"`
	Status4xx      int    `json:"status_4xx"`
	Status5xx      int    `json:"status_5xx"`
}

type VisitedPageItem struct {
	SiteName   string `json:"site_name"`
	Domain     string `json:"domain"`
	Path       string `json:"path"`
	Method     string `json:"method"`
	Count      int    `json:"count"`
	StatusCode int    `json:"status_code"`
	LastStatus int    `json:"last_status"`
	BytesTotal int64  `json:"bytes_total"`
	BytesSent  int64  `json:"bytes_sent"`
	IsThreat   bool   `json:"is_threat"`
}

// Manager mengelola mesin WAF, aturan proteksi, watcher log, dan pencatatan telemetri insiden
type Manager struct {
	store           *store.Store
	logWatcher      *LogWatcher
	mu              sync.RWMutex
	policyMu        sync.RWMutex
	globalEnabled   bool
	policySites     map[string]WAFSite
	policyRules     []WAFRule
	lastEnforcement *WAFEnforcementStatus
}

// NewManager membuat instance manajer WAF baru dan menyalakan watcher log Nginx
func NewManager(s *store.Store) *Manager {
	m := &Manager{
		store:         s,
		logWatcher:    NewLogWatcher(s.DB(), "/var/log/nginx"),
		globalEnabled: true,
		policySites:   make(map[string]WAFSite),
	}
	if m.logWatcher != nil {
		m.logWatcher.SetPolicyProvider(m.getPolicySnapshot)
	}
	ctx := context.Background()
	go func() {
		m.seedInitialData(ctx)
		m.reloadPolicyCache(ctx)
		_, _ = m.SyncNginxWAFConfig(ctx)
		if m.logWatcher != nil {
			m.logWatcher.Start(ctx)
		}
	}()
	return m
}

// getPolicySnapshot mengembalikan salinan thread-safe state Global WAF, aturan situs, dan aturan Black/White List
func (m *Manager) getPolicySnapshot() (bool, map[string]WAFSite, []WAFRule) {
	m.policyMu.RLock()
	defer m.policyMu.RUnlock()
	siteCopy := make(map[string]WAFSite, len(m.policySites))
	for k, v := range m.policySites {
		siteCopy[k] = v
	}
	rulesCopy := make([]WAFRule, len(m.policyRules))
	copy(rulesCopy, m.policyRules)
	return m.globalEnabled, siteCopy, rulesCopy
}

// reloadPolicyCache memuat ulang state waf.global_enabled, waf_sites, dan waf_rules dari SQLite ke memori
func (m *Manager) reloadPolicyCache(ctx context.Context) {
	globalEnabled := true
	if val, err := m.store.GetSetting(ctx, "waf.global_enabled"); err == nil && val != "" {
		globalEnabled = val != "false" && val != "0"
	}
	sites, _ := m.ListSites(ctx)
	siteMap := make(map[string]WAFSite, len(sites))
	for _, s := range sites {
		siteMap[s.SiteName] = s
	}
	rules, _ := m.ListRules(ctx, "")

	m.policyMu.Lock()
	m.globalEnabled = globalEnabled
	m.policySites = siteMap
	m.policyRules = rules
	m.policyMu.Unlock()
}

// Koordinat referensi negara untuk Attack Map visualizer tanpa dependensi biner pihak ketiga
var countryCoordinates = map[string]struct {
	Name string
	Lat  float64
	Lng  float64
}{
	"US": {Name: "United States", Lat: 37.0902, Lng: -95.7129},
	"CN": {Name: "China", Lat: 35.8617, Lng: 104.1954},
	"RU": {Name: "Russia", Lat: 61.5240, Lng: 105.3188},
	"DE": {Name: "Germany", Lat: 51.1657, Lng: 10.4515},
	"NL": {Name: "Netherlands", Lat: 52.1326, Lng: 5.2913},
	"SG": {Name: "Singapore", Lat: 1.3521, Lng: 103.8198},
	"ID": {Name: "Indonesia", Lat: -0.7893, Lng: 113.9213},
	"GB": {Name: "United Kingdom", Lat: 55.3781, Lng: -3.4360},
	"FR": {Name: "France", Lat: 46.2276, Lng: 2.2137},
	"CA": {Name: "Canada", Lat: 56.1304, Lng: -106.3468},
	"AU": {Name: "Australia", Lat: -25.2744, Lng: 133.7751},
	"RO": {Name: "Romania", Lat: 45.9432, Lng: 24.9668},
	"IN": {Name: "India", Lat: 20.5937, Lng: 78.9629},
	"BR": {Name: "Brazil", Lat: -14.2350, Lng: -51.9253},
	"JP": {Name: "Japan", Lat: 36.2048, Lng: 138.2529},
	"VN": {Name: "Vietnam", Lat: 14.0583, Lng: 108.2772},
}

// seedInitialData memastikan sinkronisasi situs dan aturan loopback dasar tersedia tanpa menyuntikkan event palsu
func (m *Manager) seedInitialData(ctx context.Context) {
	time.Sleep(300 * time.Millisecond)
	_ = m.SyncSites(ctx)

	// Pastikan aturan dasar loopback whitelist tersedia jika tabel waf_rules masih kosong
	var ruleCount int
	_ = m.store.DB().QueryRowContext(ctx, "SELECT COUNT(1) FROM waf_rules").Scan(&ruleCount)
	if ruleCount == 0 {
		_, _ = m.store.DB().ExecContext(ctx, `
			INSERT INTO waf_rules (rule_type, ip_version, start_ip, end_ip, remarks, created_at)
			VALUES ('ip_whitelist', 'IPv4', '127.0.0.1', '127.0.0.255', 'Local loopback trust', ?)
		`, time.Now().Unix())
	}
}

// SyncSites menyinkronkan situs yang terdaftar di FikPanel ke tabel konfigurasi WAF
func (m *Manager) SyncSites(ctx context.Context) error {
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	for _, s := range sites {
		_, _ = m.store.DB().ExecContext(ctx, `
			INSERT OR IGNORE INTO waf_sites (site_name, total_interceptions, fake_spider, using_cdn, smart_cc, crawl_protection, rate_limit, cc_defense, status, updated_at)
			VALUES (?, 0, 1, 0, 100, 0, 0, 1, 1, ?)
		`, s.Domain, now)
	}

	// Jika tidak ada situs yang dibuat, tambahkan fallback default host dengan counter riil 0
	if len(sites) == 0 {
		_, _ = m.store.DB().ExecContext(ctx, `
			INSERT OR IGNORE INTO waf_sites (site_name, total_interceptions, fake_spider, using_cdn, smart_cc, crawl_protection, rate_limit, cc_defense, status, updated_at)
			VALUES ('default-server', 0, 1, 0, 100, 0, 0, 1, 1, ?)
		`, now)
	}

	return nil
}

// ensureValidEventCoordinates memastikan setiap event serangan memiliki koordinat GPS valid yang berbeda dari titik host server
func ensureValidEventCoordinates(e *WAFEvent) {
	if e == nil {
		return
	}
	isZero := e.Latitude == 0 && e.Longitude == 0
	isSameAsHost := math.Abs(e.Latitude-1.3521) < 0.02 && math.Abs(e.Longitude-103.8198) < 0.02 && e.CountryCode != "SG"

	if isZero || isSameAsHost {
		if cc, ok := countryCoordinates[strings.ToUpper(e.CountryCode)]; ok {
			e.Latitude = cc.Lat
			e.Longitude = cc.Lng
			if e.Country == "" || e.Country == "Unknown Country" {
				e.Country = cc.Name
			}
			return
		}
		geo := ResolveGeoLocation(e.ClientIP)
		if geo != nil {
			e.Latitude = geo.Latitude
			e.Longitude = geo.Longitude
			if e.Country == "" || e.Country == "Unknown Country" {
				e.Country = geo.Country
				e.CountryCode = geo.CountryCode
				e.Region = geo.Region
				e.City = geo.City
				e.ISP = geo.ISP
			}
		}
	}
}

// GetOverview mengembalikan metrik agregasi WAF riil, analitik log akses mendalam, top serangan, dan telemetri peta
func (m *Manager) GetOverview(ctx context.Context) (*WAFOverview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var ov WAFOverview
	globalEnabled, _, _ := m.getPolicySnapshot()
	ov.WAFGlobalEnabled = globalEnabled
	ov.OriginLatencyMs = MeasureOriginLatencyMs()
	ov.HostLocation = DetectHostLocation(ctx)
	ov.Enforcement = m.lastEnforcement

	// Ambil agregasi trafik riil dari berkas log akses Nginx di server (tanpa angka fiktif)
	var agg *LogAggregates
	if m.logWatcher != nil {
		agg = m.logWatcher.ComputeLogAggregates()
	}

	if agg != nil {
		ov.TotalRequestsAllTime = agg.TotalRequestsAllTime
		ov.TotalBandwidthBytes = agg.TotalBandwidthBytes
		ov.TodayRequests = agg.TodayRequests
		ov.YesterdayRequests = agg.YesterdayRequests
		ov.RealtimeQPS = math.Round(agg.RealtimeQPS*100) / 100
		ov.StatusBreakdown = StatusBreakdown{
			S2xx: agg.Status2xx,
			S3xx: agg.Status3xx,
			S4xx: agg.Status4xx,
			S5xx: agg.Status5xx,
		}
	}

	// Hitung total pencegatan hari ini & kemarin secara aktual dari tabel waf_events
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	startOfYesterday := startOfDay - 86400

	var todayMalicious, yesterdayMalicious int64
	_ = m.store.DB().QueryRowContext(ctx, "SELECT COUNT(1) FROM waf_events WHERE created_at >= ?", startOfDay).Scan(&todayMalicious)
	_ = m.store.DB().QueryRowContext(ctx, "SELECT COUNT(1) FROM waf_events WHERE created_at >= ? AND created_at < ?", startOfYesterday, startOfDay).Scan(&yesterdayMalicious)

	ov.MaliciousRequests = todayMalicious
	ov.YesterdayMalicious = yesterdayMalicious
	if ov.TodayRequests > 0 {
		ov.InterceptRate = float64(ov.MaliciousRequests) / float64(ov.TodayRequests) * 100
		if ov.InterceptRate > 100 {
			ov.InterceptRate = 100
		}
	}

	// 1. Top Attack IPs dengan geolokasi dan ISP lengkap
	rowsIP, err := m.store.DB().QueryContext(ctx, `
		SELECT client_ip, 
		       COALESCE(country, ''), 
		       COALESCE(country_code, ''), 
		       COALESCE(region, ''), 
		       COALESCE(city, ''), 
		       COALESCE(isp, ''), 
		       COUNT(1) as cnt 
		FROM waf_events 
		GROUP BY client_ip, country, country_code, region, city, isp 
		ORDER BY cnt DESC LIMIT 10
	`)
	if err == nil {
		defer rowsIP.Close()
		for rowsIP.Next() {
			var item AttackIPItem
			if err := rowsIP.Scan(&item.IP, &item.Country, &item.CountryCode, &item.Region, &item.City, &item.ISP, &item.Count); err == nil {
				ov.TopAttackIPs = append(ov.TopAttackIPs, item)
			}
		}
	}

	// 2. Attacked Domains
	rowsDomain, err := m.store.DB().QueryContext(ctx, `
		SELECT site_name, COUNT(1) as cnt 
		FROM waf_events 
		GROUP BY site_name 
		ORDER BY cnt DESC LIMIT 10
	`)
	if err == nil {
		defer rowsDomain.Close()
		for rowsDomain.Next() {
			var item AttackedDomain
			if err := rowsDomain.Scan(&item.Domain, &item.Count); err == nil {
				ov.AttackedDomains = append(ov.AttackedDomains, item)
			}
		}
	}

	// 3. Traffic Ranking per Domain (Data Riil dari Log Nginx + Situs Terdaftar)
	sites, _ := m.store.ListSites(ctx)
	seenDomains := make(map[string]bool)
	for _, s := range sites {
		seenDomains[s.Domain] = true
		var item TrafficRankItem
		item.Domain = s.Domain
		if agg != nil {
			if ds, ok := agg.DomainStats[s.Domain]; ok {
				item.Requests = ds.Requests
				item.BandwidthBytes = ds.BandwidthBytes
				item.BytesSent = ds.BandwidthBytes
				item.Threats = ds.Threats
				item.Status2xx = ds.Status2xx
				item.Status3xx = ds.Status3xx
				item.Status4xx = ds.Status4xx
				item.Status5xx = ds.Status5xx
			}
		}
		ov.TrafficRanking = append(ov.TrafficRanking, item)
	}
	if agg != nil {
		for dom, ds := range agg.DomainStats {
			if !seenDomains[dom] && ds.Requests > 0 {
				ov.TrafficRanking = append(ov.TrafficRanking, TrafficRankItem{
					Domain:         dom,
					Requests:       ds.Requests,
					BandwidthBytes: ds.BandwidthBytes,
					BytesSent:      ds.BandwidthBytes,
					Threats:        ds.Threats,
					Status2xx:      ds.Status2xx,
					Status3xx:      ds.Status3xx,
					Status4xx:      ds.Status4xx,
					Status5xx:      ds.Status5xx,
				})
			}
		}
	}
	sort.Slice(ov.TrafficRanking, func(i, j int) bool {
		return ov.TrafficRanking[i].Requests > ov.TrafficRanking[j].Requests
	})

	// 4. Visited Pages Top 15 (Menggabungkan Endpoint Riil dari Log Akses & Insiden WAF)
	if agg != nil && len(agg.PageStats) > 0 {
		for _, ps := range agg.PageStats {
			ov.VisitedPages = append(ov.VisitedPages, *ps)
		}
		sort.Slice(ov.VisitedPages, func(i, j int) bool {
			return ov.VisitedPages[i].Count > ov.VisitedPages[j].Count
		})
		if len(ov.VisitedPages) > 15 {
			ov.VisitedPages = ov.VisitedPages[:15]
		}
	} else {
		rowsPages, err := m.store.DB().QueryContext(ctx, `
			SELECT site_name, url_path, COALESCE(method, 'GET'), status_code, COUNT(1) as cnt 
			FROM waf_events 
			GROUP BY site_name, url_path, method, status_code 
			ORDER BY cnt DESC LIMIT 15
		`)
		if err == nil {
			defer rowsPages.Close()
			for rowsPages.Next() {
				var item VisitedPageItem
				item.IsThreat = true
				if err := rowsPages.Scan(&item.SiteName, &item.Path, &item.Method, &item.StatusCode, &item.Count); err == nil {
					item.Domain = item.SiteName
					item.LastStatus = item.StatusCode
					ov.VisitedPages = append(ov.VisitedPages, item)
				}
			}
		}
	}

	// 5. Top Visitors (Pengakses Normal maupun Bot dari Log Nginx)
	if agg != nil && len(agg.VisitorStats) > 0 {
		for _, vs := range agg.VisitorStats {
			ov.TopVisitors = append(ov.TopVisitors, *vs)
		}
		sort.Slice(ov.TopVisitors, func(i, j int) bool {
			return ov.TopVisitors[i].Count > ov.TopVisitors[j].Count
		})
		if len(ov.TopVisitors) > 12 {
			ov.TopVisitors = ov.TopVisitors[:12]
		}
	}

	// 6. Recent Events untuk Peta Serangan (Pastikan mencakup seluruh asal IP unik agar peta selalu kaya dan tidak kosong)
	recentEvs, _ := m.ListEvents(ctx, 40)
	seenIPs := make(map[string]bool)
	var diverseEvents []WAFEvent
	for _, ev := range recentEvs {
		if !seenIPs[ev.ClientIP] {
			seenIPs[ev.ClientIP] = true
			diverseEvents = append(diverseEvents, ev)
		}
	}
	// Tambahkan perwakilan dari tiap IP penyerang unik lainnya di waf_events bila belum masuk
	rowsDistinct, err := m.store.DB().QueryContext(ctx, `
		SELECT id, site_name, client_ip,
		       COALESCE(country, ''), COALESCE(country_code, ''),
		       COALESCE(region, ''), COALESCE(city, ''), COALESCE(isp, ''),
		       latitude, longitude,
		       COALESCE(device, ''), COALESCE(os, ''), COALESCE(browser, ''), COALESCE(method, 'GET'),
		       attack_type, action, url_path,
		       COALESCE(user_agent, ''), COALESCE(rule_name, ''), status_code, MAX(created_at)
		FROM waf_events
		GROUP BY client_ip
		ORDER BY COUNT(1) DESC LIMIT 25
	`)
	if err == nil {
		defer rowsDistinct.Close()
		for rowsDistinct.Next() {
			var e WAFEvent
			if err := rowsDistinct.Scan(
				&e.ID, &e.SiteName, &e.ClientIP,
				&e.Country, &e.CountryCode,
				&e.Region, &e.City, &e.ISP,
				&e.Latitude, &e.Longitude,
				&e.Device, &e.OS, &e.Browser, &e.Method,
				&e.AttackType, &e.Action, &e.URLPath,
				&e.UserAgent, &e.RuleName, &e.StatusCode, &e.CreatedAt,
			); err == nil {
				ensureValidEventCoordinates(&e)
				if !seenIPs[e.ClientIP] {
					seenIPs[e.ClientIP] = true
					diverseEvents = append(diverseEvents, e)
				}
			}
		}
	}
	// Gabungkan event tambahan hingga 30 entri
	for _, ev := range recentEvs {
		if len(diverseEvents) >= 30 {
			break
		}
		exists := false
		for _, de := range diverseEvents {
			if de.ID == ev.ID {
				exists = true
				break
			}
		}
		if !exists {
			diverseEvents = append(diverseEvents, ev)
		}
	}
	ov.RecentEvents = diverseEvents

	// 7. Recent Access Logs (Semua Lalu Lintas HTTP Terkini)
	if m.logWatcher != nil {
		ov.RecentAccessLogs = m.logWatcher.GetRecentAccessLogs(80)
	}

	// 8. Total Rules Count
	_ = m.store.DB().QueryRowContext(ctx, "SELECT COUNT(1) FROM waf_rules").Scan(&ov.TotalRulesCount)

	return &ov, nil
}

// ListSites mengembalikan seluruh situs dengan status proteksi WAF
func (m *Manager) ListSites(ctx context.Context) ([]WAFSite, error) {
	_ = m.SyncSites(ctx)

	// Hitung jumlah intersepsi aktual per situs dari tabel waf_events
	interceptionsBySite := make(map[string]int)
	evRows, err := m.store.DB().QueryContext(ctx, `SELECT site_name, COUNT(1) FROM waf_events GROUP BY site_name`)
	if err == nil {
		for evRows.Next() {
			var sn string
			var cnt int
			if err := evRows.Scan(&sn, &cnt); err == nil {
				interceptionsBySite[sn] = cnt
			}
		}
		evRows.Close()
	}

	rows, err := m.store.DB().QueryContext(ctx, `
		SELECT site_name, total_interceptions, fake_spider, using_cdn, smart_cc, crawl_protection, rate_limit, cc_defense, status, updated_at
		FROM waf_sites ORDER BY site_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []WAFSite
	for rows.Next() {
		var s WAFSite
		var fakeSpider, usingCDN, crawlProt, rateLimit, status int
		if err := rows.Scan(
			&s.SiteName, &s.TotalInterceptions, &fakeSpider, &usingCDN, &s.SmartCC,
			&crawlProt, &rateLimit, &s.CCDefense, &status, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		s.TotalInterceptions = interceptionsBySite[s.SiteName]
		s.FakeSpider = fakeSpider == 1
		s.UsingCDN = usingCDN == 1
		s.CrawlProtection = crawlProt == 1
		s.RateLimit = rateLimit == 1
		s.Status = status == 1
		list = append(list, s)
	}
	return list, nil
}

// UpdateSite memperbarui parameter proteksi WAF pada satu situs dan menyinkronkan ke Nginx
func (m *Manager) UpdateSite(ctx context.Context, site WAFSite) error {
	m.mu.Lock()
	fakeSpider := 0
	if site.FakeSpider {
		fakeSpider = 1
	}
	usingCDN := 0
	if site.UsingCDN {
		usingCDN = 1
	}
	crawlProt := 0
	if site.CrawlProtection {
		crawlProt = 1
	}
	rateLimit := 0
	if site.RateLimit {
		rateLimit = 1
	}
	status := 0
	if site.Status {
		status = 1
	}

	_, err := m.store.DB().ExecContext(ctx, `
		UPDATE waf_sites SET 
			fake_spider = ?, using_cdn = ?, smart_cc = ?, crawl_protection = ?, rate_limit = ?, cc_defense = ?, status = ?, updated_at = ?
		WHERE site_name = ?
	`, fakeSpider, usingCDN, site.SmartCC, crawlProt, rateLimit, site.CCDefense, status, time.Now().Unix(), site.SiteName)
	m.mu.Unlock()
	if err != nil {
		return err
	}
	m.reloadPolicyCache(ctx)
	_, _ = m.SyncNginxWAFConfig(ctx)
	return nil
}

// ListRules mengambil daftar aturan filter berdasarkan tipenya
func (m *Manager) ListRules(ctx context.Context, ruleType string) ([]WAFRule, error) {
	var rows *sql.Rows
	var err error
	if ruleType != "" {
		rows, err = m.store.DB().QueryContext(ctx, `
			SELECT id, rule_type, ip_version, start_ip, end_ip, pattern, remarks, created_at
			FROM waf_rules WHERE rule_type = ? ORDER BY id DESC
		`, ruleType)
	} else {
		rows, err = m.store.DB().QueryContext(ctx, `
			SELECT id, rule_type, ip_version, start_ip, end_ip, pattern, remarks, created_at
			FROM waf_rules ORDER BY id DESC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []WAFRule
	for rows.Next() {
		var r WAFRule
		if err := rows.Scan(&r.ID, &r.RuleType, &r.IPVersion, &r.StartIP, &r.EndIP, &r.Pattern, &r.Remarks, &r.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

// AddRule menambahkan aturan WAF baru (IP whitelist/blacklist, UA, atau URL)
func (m *Manager) AddRule(ctx context.Context, rule WAFRule) (int64, error) {
	m.mu.Lock()
	if rule.IPVersion == "" {
		rule.IPVersion = "IPv4"
	}
	res, err := m.store.DB().ExecContext(ctx, `
		INSERT INTO waf_rules (rule_type, ip_version, start_ip, end_ip, pattern, remarks, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, rule.RuleType, rule.IPVersion, strings.TrimSpace(rule.StartIP), strings.TrimSpace(rule.EndIP), strings.TrimSpace(rule.Pattern), strings.TrimSpace(rule.Remarks), time.Now().Unix())
	m.mu.Unlock()
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	m.reloadPolicyCache(ctx)
	_, _ = m.SyncNginxWAFConfig(ctx)
	return id, err
}

// DeleteRule menghapus aturan WAF berdasarkan ID dan menyinkronkan ke Nginx
func (m *Manager) DeleteRule(ctx context.Context, id int64) error {
	m.mu.Lock()
	_, err := m.store.DB().ExecContext(ctx, "DELETE FROM waf_rules WHERE id = ?", id)
	m.mu.Unlock()
	if err != nil {
		return err
	}
	m.reloadPolicyCache(ctx)
	_, _ = m.SyncNginxWAFConfig(ctx)
	return nil
}

// SetGlobalEnabled mengaktifkan atau menonaktifkan mesin WAF secara global dan menyinkronkan ke Nginx
func (m *Manager) SetGlobalEnabled(ctx context.Context, enabled bool) (*WAFEnforcementStatus, error) {
	val := "false"
	if enabled {
		val = "true"
	}
	if err := m.store.SetSetting(ctx, "waf.global_enabled", val); err != nil {
		return nil, err
	}
	m.reloadPolicyCache(ctx)
	return m.SyncNginxWAFConfig(ctx)
}

// ClearEvents membersihkan seluruh riwayat insiden serangan WAF (waf_events)
func (m *Manager) ClearEvents(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.store.DB().ExecContext(ctx, "DELETE FROM waf_events")
	return err
}

// GetEnforcementStatus mengembalikan status penegakan dan preview konfigurasi WAF Nginx saat ini
func (m *Manager) GetEnforcementStatus(ctx context.Context) (*WAFEnforcementStatus, error) {
	m.mu.RLock()
	st := m.lastEnforcement
	m.mu.RUnlock()
	if st != nil {
		return st, nil
	}
	return m.SyncNginxWAFConfig(ctx)
}

// ListEvents mengambil rekaman insiden serangan terbaru beserta telemetri lengkap
func (m *Manager) ListEvents(ctx context.Context, limit int) ([]WAFEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := m.store.DB().QueryContext(ctx, `
		SELECT id, site_name, client_ip, 
		       COALESCE(country, ''), COALESCE(country_code, ''), 
		       COALESCE(region, ''), COALESCE(city, ''), COALESCE(isp, ''),
		       latitude, longitude, 
		       COALESCE(device, ''), COALESCE(os, ''), COALESCE(browser, ''), COALESCE(method, 'GET'),
		       attack_type, action, url_path, 
		       COALESCE(user_agent, ''), COALESCE(rule_name, ''), status_code, created_at
		FROM waf_events ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []WAFEvent
	for rows.Next() {
		var e WAFEvent
		if err := rows.Scan(
			&e.ID, &e.SiteName, &e.ClientIP,
			&e.Country, &e.CountryCode,
			&e.Region, &e.City, &e.ISP,
			&e.Latitude, &e.Longitude,
			&e.Device, &e.OS, &e.Browser, &e.Method,
			&e.AttackType, &e.Action, &e.URLPath,
			&e.UserAgent, &e.RuleName, &e.StatusCode, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		ensureValidEventCoordinates(&e)
		list = append(list, e)
	}
	return list, nil
}

// ListAccessLogs mengembalikan riwayat seluruh lalu lintas akses HTTP (baik normal maupun serangan) dari berkas log Nginx
func (m *Manager) ListAccessLogs(limit int) []WAFAccessLog {
	if m.logWatcher == nil {
		return nil
	}
	return m.logWatcher.GetRecentAccessLogs(limit)
}
