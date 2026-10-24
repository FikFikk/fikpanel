package sites

import (
	"strings"
	"testing"

	"fikpanel/internal/store"
)

func TestNormalizeDomain(t *testing.T) {
	ok := map[string]string{
		"Example.COM":               "example.com",
		"https://situs.id/path?x=1": "situs.id",
		"blog.example.co.id.":       "blog.example.co.id",
		"*.example.com":             "*.example.com",
		"xn--bcher-kva.example":     "xn--bcher-kva.example",
	}
	for in, want := range ok {
		got, err := NormalizeDomain(in)
		if err != nil || got != want {
			t.Errorf("NormalizeDomain(%q) = %q, %v; ingin %q", in, got, err, want)
		}
	}
	bad := []string{"", "localhost", "192.168.1.1", "example.com:8080", "-abc.com", "abc.123", "bücher.de", "a_b.com"}
	for _, in := range bad {
		if _, err := NormalizeDomain(in); err == nil {
			t.Errorf("NormalizeDomain(%q) seharusnya error", in)
		}
	}
}

func TestParseDomainList(t *testing.T) {
	primary, aliases, checks := ParseDomainList("*.app.com\nexample.com\nexample.com\nblog.example.com\nbad_domain", true)
	if primary != "example.com" {
		t.Fatalf("primary = %q; ingin example.com (wildcard tidak boleh jadi primary)", primary)
	}
	want := []string{"*.app.com", "www.example.com", "blog.example.com"}
	if strings.Join(aliases, ",") != strings.Join(want, ",") {
		t.Fatalf("aliases = %v; ingin %v", aliases, want)
	}
	var dup, invalid, auto int
	for _, c := range checks {
		switch {
		case c.Error == "duplikat":
			dup++
		case c.Error != "":
			invalid++
		case c.Auto:
			auto++
		}
	}
	if dup != 1 || invalid != 1 || auto != 1 {
		t.Fatalf("dup=%d invalid=%d auto=%d; ingin 1/1/1", dup, invalid, auto)
	}

	// www tidak ditambahkan untuk subdomain dan bila toggle mati
	_, aliases, _ = ParseDomainList("api.example.com", true)
	if len(aliases) != 0 {
		t.Fatalf("subdomain tidak boleh mendapat www otomatis: %v", aliases)
	}
	_, aliases, _ = ParseDomainList("example.co.id", false)
	if len(aliases) != 0 {
		t.Fatalf("addWWW=false tidak boleh menambah alias: %v", aliases)
	}
	_, aliases, _ = ParseDomainList("example.co.id", true)
	if len(aliases) != 1 || aliases[0] != "www.example.co.id" {
		t.Fatalf("SLD .co.id harus mendapat www: %v", aliases)
	}
}

func TestValidateRootPathAndUpstream(t *testing.T) {
	if p, err := ValidateRootPath("/var/www/a.com/public_html/"); err != nil || p != "/var/www/a.com/public_html" {
		t.Fatalf("root valid ditolak: %q %v", p, err)
	}
	for _, bad := range []string{"var/www", "/etc", "/", "/var/../etc", "/var/www/a;b", "/root/"} {
		if _, err := ValidateRootPath(bad); err == nil {
			t.Errorf("ValidateRootPath(%q) seharusnya error", bad)
		}
	}
	if u, err := ValidateUpstream("http://127.0.0.1:3000/"); err != nil || u != "http://127.0.0.1:3000" {
		t.Fatalf("upstream valid ditolak: %q %v", u, err)
	}
	for _, bad := range []string{"127.0.0.1:3000", "ftp://x", "http://"} {
		if _, err := ValidateUpstream(bad); err == nil {
			t.Errorf("ValidateUpstream(%q) seharusnya error", bad)
		}
	}
}

func testManager() *Manager {
	m := NewManager(Paths{VhostDir: "/vh", LogDir: "/log", ACMEWebroot: "/acme", CertDir: "/ssl"}, nil, nil)
	m.mock = true
	return m
}

func TestRenderWithoutSSL(t *testing.T) {
	m := testManager()
	site := &store.Site{Domain: "a.com", Aliases: `["www.a.com"]`, Kind: "php", RootPath: "/var/www/a.com", PHPVersion: "8.3", SSLMode: "letsencrypt", ForceHTTPS: 1}
	// Sertifikat masih pending: blok 443 dan redirect tidak boleh muncul
	out, err := m.Render(site, &store.Certificate{Status: "pending"})
	if err != nil {
		t.Fatal(err)
	}
	for _, must := range []string{"server_name a.com www.a.com;", "/.well-known/acme-challenge/", "php8.3-fpm.sock"} {
		if !strings.Contains(out, must) {
			t.Errorf("output tidak memuat %q", must)
		}
	}
	for _, mustNot := range []string{"listen 443", "return 301"} {
		if strings.Contains(out, mustNot) {
			t.Errorf("output tanpa SSL tidak boleh memuat %q", mustNot)
		}
	}
	if strings.Count(out, "{") != strings.Count(out, "}") {
		t.Error("kurung kurawal tidak seimbang")
	}
}

func TestRenderWithSSL(t *testing.T) {
	m := testManager()
	site := &store.Site{Domain: "a.com", Aliases: `[]`, Kind: "proxy", Upstream: "http://127.0.0.1:3000", SSLMode: "letsencrypt", ForceHTTPS: 1, HSTS: 1}
	out, err := m.Render(site, &store.Certificate{Status: "issued"})
	if err != nil {
		t.Fatal(err)
	}
	for _, must := range []string{"listen 443 ssl http2;", "return 301 https://", "Strict-Transport-Security", "/ssl/a.com/fullchain.pem", "proxy_pass http://127.0.0.1:3000;"} {
		if !strings.Contains(out, must) {
			t.Errorf("output tidak memuat %q", must)
		}
	}
	if strings.Count(out, "{") != strings.Count(out, "}") {
		t.Error("kurung kurawal tidak seimbang")
	}

	// SSL dinonaktifkan: sertifikat diabaikan
	site.SSLMode = "none"
	out, _ = m.Render(site, &store.Certificate{Status: "issued"})
	if strings.Contains(out, "listen 443") {
		t.Error("ssl_mode none tidak boleh merender blok 443")
	}
}
