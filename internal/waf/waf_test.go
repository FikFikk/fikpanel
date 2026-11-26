package waf

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"fikpanel/internal/store"
)

func TestMatchIPRule(t *testing.T) {
	// 1. Single IP
	if !MatchIPRule("203.0.113.9", "203.0.113.9", "") {
		t.Fatal("expected single IP to match")
	}
	if MatchIPRule("203.0.113.10", "203.0.113.9", "") {
		t.Fatal("expected different single IP not to match")
	}

	// 2. IPv4 Range
	if !MatchIPRule("192.168.1.50", "192.168.1.1", "192.168.1.100") {
		t.Fatal("expected IP inside range to match")
	}
	if MatchIPRule("192.168.1.150", "192.168.1.1", "192.168.1.100") {
		t.Fatal("expected IP outside range not to match")
	}

	// 3. CIDR
	if !MatchIPRule("10.20.30.40", "10.20.30.0/24", "") {
		t.Fatal("expected IP inside CIDR to match")
	}
	if MatchIPRule("10.20.31.1", "10.20.30.0/24", "") {
		t.Fatal("expected IP outside CIDR not to match")
	}
}

func TestClassifyThreatWithRulesHierarchy(t *testing.T) {
	rules := []WAFRule{
		{RuleType: "ip_whitelist", StartIP: "198.51.100.7"},
		{RuleType: "ip_blacklist", StartIP: "203.0.113.0/24"},
		{RuleType: "url_blacklist", Pattern: "/secret-admin"},
		{RuleType: "ua_blacklist", Pattern: "BadCrawler"},
	}
	activeSite := &WAFSite{
		SiteName:        "example.com",
		Status:          true,
		FakeSpider:      true,
		CrawlProtection: true,
	}

	// 1. Global WAF Disabled -> Allow even SQLi
	res := ClassifyThreatWithRules("203.0.113.5", "GET", "/login?id=1' OR '1'='1", "Mozilla/5.0", 200, false, activeSite, rules)
	if res.IsSuspicious || res.Action != "allow" {
		t.Fatalf("expected global disabled WAF to allow traffic, got %+v", res)
	}

	// 2. Whitelisted IP -> Allow even if path looks like SQLi
	res = ClassifyThreatWithRules("198.51.100.7", "GET", "/login?id=1' OR '1'='1", "Mozilla/5.0", 200, true, activeSite, rules)
	if res.IsSuspicious || res.Action != "allow" {
		t.Fatalf("expected whitelisted IP to bypass block, got %+v", res)
	}

	// 3. Blacklisted IP CIDR -> Block normal GET /
	res = ClassifyThreatWithRules("203.0.113.42", "GET", "/", "Mozilla/5.0", 200, true, activeSite, rules)
	if !res.IsSuspicious || res.Action != "block" || res.AttackType != "Blocked IP (Blacklist)" {
		t.Fatalf("expected blacklisted CIDR IP to be blocked, got %+v", res)
	}

	// 4. Blacklisted URL Pattern -> Block
	res = ClassifyThreatWithRules("8.8.8.8", "GET", "/secret-admin/panel", "Mozilla/5.0", 200, true, activeSite, rules)
	if !res.IsSuspicious || res.AttackType != "Blocked URL Rule" {
		t.Fatalf("expected blacklisted URL to be blocked, got %+v", res)
	}

	// 5. Site CrawlProtection -> Block Scrapy/wget
	res = ClassifyThreatWithRules("8.8.4.4", "GET", "/products", "Scrapy/2.11 (+https://scrapy.org)", 200, true, activeSite, rules)
	if !res.IsSuspicious || res.AttackType != "Bad Bot" {
		t.Fatalf("expected scraper UA to be blocked when CrawlProtection is enabled, got %+v", res)
	}
}

func TestManagerDynamicTransparencyAndNginxSync(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FIKPANEL_WAF_DIR", filepath.Join(tmpDir, "waf"))

	st, err := store.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer st.Close()

	mgr := NewManager(st)
	ctx := context.Background()

	// Pastikan tidak ada angka fiktif saat database masih bersih
	ov, err := mgr.GetOverview(ctx)
	if err != nil {
		t.Fatalf("GetOverview error: %v", err)
	}
	if ov.TodayRequests != 0 || ov.YesterdayRequests != 0 || ov.MaliciousRequests != 0 || ov.YesterdayMalicious != 0 {
		t.Fatalf("expected zero real metrics on fresh DB, got today=%d yesterday=%d mal=%d ymal=%d",
			ov.TodayRequests, ov.YesterdayRequests, ov.MaliciousRequests, ov.YesterdayMalicious)
	}

	// Tambah aturan IP blacklist dan pastikan tercermin di snippet konfigurasi Nginx WAF
	_, err = mgr.AddRule(ctx, WAFRule{
		RuleType:  "ip_blacklist",
		IPVersion: "IPv4",
		StartIP:   "198.51.100.99",
		Remarks:   "Test Attacker",
	})
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	enf, err := mgr.GetEnforcementStatus(ctx)
	if err != nil {
		t.Fatalf("GetEnforcementStatus failed: %v", err)
	}
	if !strings.Contains(enf.ConfigPreview, "deny 198.51.100.99;") {
		t.Fatalf("expected generated Nginx WAF config to contain deny 198.51.100.99;, got:\n%s", enf.ConfigPreview)
	}

	// Matikan Global WAF dan pastikan mode Pass-Through tertulis di konfigurasi
	enfOff, err := mgr.SetGlobalEnabled(ctx, false)
	if err != nil {
		t.Fatalf("SetGlobalEnabled(false) failed: %v", err)
	}
	if enfOff.GlobalEnabled || !strings.Contains(enfOff.ConfigPreview, "Pass-Through Mode") {
		t.Fatalf("expected disabled WAF config preview, got:\n%s", enfOff.ConfigPreview)
	}
}

func TestParseCISAKEVAndGitHubAdvisories(t *testing.T) {
	// Uji parser katalog CISA KEV JSON untuk PHP2Shell (CVE-2024-4577) dan React2Shell (CVE-2025-55182)
	kevSample := []byte(`{
		"vulnerabilities": [
			{
				"cveID": "CVE-2024-4577",
				"vendorProject": "PHP Group",
				"product": "PHP",
				"vulnerabilityName": "PHP CGI Argument Injection Vulnerability",
				"dateAdded": "2024-06-12",
				"shortDescription": "PHP-CGI memungkinkan RCE lewat soft-hyphen encoding.",
				"requiredAction": "Upgrade ke PHP 8.3.8+"
			},
			{
				"cveID": "CVE-2025-55182",
				"vendorProject": "Meta",
				"product": "React Server Components",
				"vulnerabilityName": "React2Shell Flight Protocol RCE",
				"dateAdded": "2025-12-05",
				"shortDescription": "Remote code execution pada React Server Components.",
				"requiredAction": "Upgrade React 19.2.1+"
			}
		]
	}`)
	kevItems, err := parseCISAKEVJSON(kevSample)
	if err != nil {
		t.Fatalf("parseCISAKEVJSON gagal: %v", err)
	}
	if len(kevItems) != 2 {
		t.Fatalf("ekspektasi 2 entri KEV, dapat %d", len(kevItems))
	}
	byCVE := make(map[string]CVEAdvisory, len(kevItems))
	for _, it := range kevItems {
		byCVE[it.CVEID] = it
	}
	if byCVE["CVE-2024-4577"].VirtualPatchID != "vp-php-cgi-2024-4577" {
		t.Fatalf("CVE-2024-4577 harus terpetakan ke vp-php-cgi-2024-4577, dapat %q", byCVE["CVE-2024-4577"].VirtualPatchID)
	}
	if byCVE["CVE-2025-55182"].VirtualPatchID != "vp-react2shell-2025-55182" {
		t.Fatalf("CVE-2025-55182 harus terpetakan ke vp-react2shell-2025-55182, dapat %q", byCVE["CVE-2025-55182"].VirtualPatchID)
	}

	// Uji parser GitHub Security Advisories JSON
	ghSample := []byte(`[
		{
			"ghsa_id": "GHSA-f82v-jwr5-mffw",
			"cve_id": "CVE-2025-29927",
			"summary": "Authorization Bypass in Next.js Middleware",
			"description": "Header x-middleware-subrequest melewati middleware auth.",
			"severity": "critical",
			"html_url": "https://github.com/advisories/GHSA-f82v-jwr5-mffw",
			"published_at": "2025-03-21T12:00:00Z",
			"cvss": {"score": 9.1},
			"vulnerabilities": [
				{
					"package": {"ecosystem": "npm", "name": "next"},
					"vulnerable_version_range": "< 15.2.3",
					"first_patched_version": "15.2.3"
				}
			]
		}
	]`)
	ghItems, err := parseGitHubAdvisoriesJSON(ghSample)
	if err != nil {
		t.Fatalf("parseGitHubAdvisoriesJSON gagal: %v", err)
	}
	if len(ghItems) != 1 || ghItems[0].VirtualPatchID != "vp-nextjs-2025-29927" {
		t.Fatalf("Next.js advisory harus terpetakan ke vp-nextjs-2025-29927, dapat %+v", ghItems)
	}
}

func TestVirtualPatchesNginxAndPayloadTester(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FIKPANEL_WAF_DIR", filepath.Join(tmpDir, "waf"))

	st, err := store.Open(filepath.Join(tmpDir, "test_vp.db"))
	if err != nil {
		t.Fatalf("gagal membuka store: %v", err)
	}
	defer st.Close()

	mgr := NewManager(st)
	ctx := context.Background()

	// Aktifkan Virtual Patch PHP2Shell & Next.js Middleware Bypass
	enf, err := mgr.SetVirtualPatchEnabled(ctx, "vp-php-cgi-2024-4577", true)
	if err != nil {
		t.Fatalf("SetVirtualPatchEnabled PHP2Shell gagal: %v", err)
	}
	if !strings.Contains(enf.ConfigPreview, "Virtual Patch: CVE-2024-4577") {
		t.Fatalf("waf-rules.conf harus memuat snippet PHP2Shell, dapat:\n%s", enf.ConfigPreview)
	}
	if _, err := mgr.SetVirtualPatchEnabled(ctx, "vp-nextjs-2025-29927", true); err != nil {
		t.Fatalf("SetVirtualPatchEnabled Next.js gagal: %v", err)
	}

	// Uji simulasi payload PHP2Shell harus terblokir
	resPHP := mgr.TestPayload(ctx, PayloadTestRequest{
		Method: "POST",
		Path:   "/index.php?%ADd+allow_url_include%3d1",
	})
	if !resPHP.Blocked || resPHP.MatchedPatchID != "vp-php-cgi-2024-4577" {
		t.Fatalf("ekspektasi payload PHP2Shell diblokir oleh vp-php-cgi-2024-4577, dapat %+v", resPHP)
	}

	// Uji simulasi payload Next.js x-middleware-subrequest harus terblokir
	resNext := mgr.TestPayload(ctx, PayloadTestRequest{
		Method:  "GET",
		Path:    "/admin",
		Headers: map[string]string{"x-middleware-subrequest": "middleware"},
	})
	if !resNext.Blocked || resNext.MatchedPatchID != "vp-nextjs-2025-29927" {
		t.Fatalf("ekspektasi payload Next.js bypass diblokir oleh vp-nextjs-2025-29927, dapat %+v", resNext)
	}

	// Uji trafik normal harus diizinkan
	resNormal := mgr.TestPayload(ctx, PayloadTestRequest{
		Method: "GET",
		Path:   "/artikel/panduan-server",
	})
	if resNormal.Blocked {
		t.Fatalf("ekspektasi trafik normal diizinkan, dapat %+v", resNormal)
	}

	// Uji Cloudflare Real-IP di waf-http.conf saat salah satu situs mengaktifkan UsingCDN
	httpConf, cfOn := BuildNginxWAFHTTPConfig(true, []WAFSite{{SiteName: "app.id", Status: true, UsingCDN: true, SmartCC: 60}})
	if !cfOn || !strings.Contains(httpConf, "real_ip_header CF-Connecting-IP;") || !strings.Contains(httpConf, "limit_req_zone") {
		t.Fatalf("waf-http.conf harus memuat real_ip_header & limit_req_zone, dapat:\n%s", httpConf)
	}
}

func TestAutoBanConfigAndTrigger(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FIKPANEL_WAF_DIR", filepath.Join(tmpDir, "waf"))

	st, err := store.Open(filepath.Join(tmpDir, "test_ab.db"))
	if err != nil {
		t.Fatalf("gagal membuka store: %v", err)
	}
	defer st.Close()

	mgr := NewManager(st)
	ctx := context.Background()

	// Simpan konfigurasi Auto-Ban ambang 3 serangan dalam 10 menit
	saved, err := mgr.UpdateAutoBanConfig(ctx, AutoBanConfig{
		Enabled:        true,
		Threshold:      3,
		WindowMinutes:  10,
		BanMinutes:     60,
		SyncUFW:        false,
		ExcludePrivate: true,
	})
	if err != nil || !saved.Enabled || saved.Threshold != 3 {
		t.Fatalf("UpdateAutoBanConfig gagal: %+v, err=%v", saved, err)
	}

	// Masukkan 3 event serangan dari IP publik yang sama ke waf_events
	attackerIP := "198.51.100.88"
	for i := 0; i < 3; i++ {
		_, err := st.DB().ExecContext(ctx, `
			INSERT INTO waf_events (site_name, client_ip, country, country_code, attack_type, action, url_path, user_agent, rule_name, status_code, created_at)
			VALUES ('app.id', ?, 'United States', 'US', 'SQL Injection', 'block', '/login?id=1', 'sqlmap/1.8', 'OWASP SQLi', 403, strftime('%s','now'))
		`, attackerIP)
		if err != nil {
			t.Fatalf("gagal menyisipkan event uji: %v", err)
		}
	}

	// Picu Auto-Ban dan pastikan IP publik masuk ke ip_blacklist
	mgr.handleAutoBanTrigger(ctx, attackerIP, "app.id", "SQL Injection")
	rules, err := mgr.ListRules(ctx, "ip_blacklist")
	if err != nil {
		t.Fatalf("ListRules gagal: %v", err)
	}
	found := false
	for _, r := range rules {
		if r.StartIP == attackerIP {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ekspektasi IP penyerang %s otomatis masuk ke ip_blacklist", attackerIP)
	}

	// IP privat tidak boleh diblokir saat ExcludePrivate aktif
	mgr.handleAutoBanTrigger(ctx, "192.168.1.10", "app.id", "SQL Injection")
	rules2, _ := mgr.ListRules(ctx, "ip_blacklist")
	for _, r := range rules2 {
		if r.StartIP == "192.168.1.10" {
			t.Fatalf("IP privat seharusnya dikecualikan dari Auto-Ban")
		}
	}
}
