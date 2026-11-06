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
