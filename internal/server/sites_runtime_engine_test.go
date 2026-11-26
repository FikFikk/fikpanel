package server

import (
	"testing"

	"fikpanel/internal/sites"
	"fikpanel/internal/store"
)

func TestInspectPackageJSONDistinguishesSPAVsNodeServer(t *testing.T) {
	// Vite + React tanpa dependensi server harus terdeteksi sebagai Static SPA (dist/)
	viteSPA := []byte(`{"name":"web","scripts":{"build":"vite build"},"dependencies":{"react":"^19.0.0"},"devDependencies":{"vite":"^6.0.0"}}`)
	fw, isSPA, staticDir, hasBuild, _, _, _, _ := inspectPackageJSON(viteSPA)
	if !isSPA || fw != "React SPA" || staticDir != "dist" || !hasBuild {
		t.Fatalf("Vite React seharusnya terdeteksi Static SPA dist/: fw=%q isSPA=%v dir=%q build=%v", fw, isSPA, staticDir, hasBuild)
	}

	// Next.js + Prisma harus terdeteksi sebagai Node.js SSR dengan kebutuhan DB
	nextApp := []byte(`{"name":"app","scripts":{"build":"next build","start":"next start"},"dependencies":{"next":"15.0.0","@prisma/client":"^6.0.0"}}`)
	fw, isSPA, _, hasBuild, hasStart, _, hasPrisma, _ := inspectPackageJSON(nextApp)
	if isSPA || fw != "Next.js" || !hasBuild || !hasStart || !hasPrisma {
		t.Fatalf("Next.js seharusnya terdeteksi Node SSR + Prisma: fw=%q isSPA=%v build=%v start=%v prisma=%v", fw, isSPA, hasBuild, hasStart, hasPrisma)
	}

	// Express API harus terdeteksi sebagai Node.js Express
	expressApp := []byte(`{"name":"api","main":"server.js","dependencies":{"express":"^4.21.0","drizzle-orm":"^0.38.0"}}`)
	fw, isSPA, _, _, _, mainEntry, _, hasDrizzle := inspectPackageJSON(expressApp)
	if isSPA || fw != "Express" || mainEntry != "server.js" || !hasDrizzle {
		t.Fatalf("Express seharusnya terdeteksi Node API + Drizzle: fw=%q isSPA=%v main=%q drizzle=%v", fw, isSPA, mainEntry, hasDrizzle)
	}
}

func TestParseEnvEntriesAndMasking(t *testing.T) {
	raw := "# Komentar\nAPP_NAME=\"FikPanel App\"\nAPP_ENV=production\nDB_PASSWORD=\"rahasia#123\"\nSTRIPE_SECRET_KEY=sk_live_abc\n"
	entries := parseEnvEntries(raw)
	if len(entries) != 4 {
		t.Fatalf("jumlah entri .env = %d; ingin 4", len(entries))
	}
	if entries[0].Key != "APP_NAME" || entries[0].Value != "FikPanel App" || entries[0].Secret {
		t.Errorf("entri APP_NAME salah: %+v", entries[0])
	}
	if entries[2].Key != "DB_PASSWORD" || entries[2].Value != "rahasia#123" || !entries[2].Secret {
		t.Errorf("DB_PASSWORD harus bertanda secret=true: %+v", entries[2])
	}
	if !entries[3].Secret {
		t.Errorf("STRIPE_SECRET_KEY harus bertanda secret=true")
	}
}

func TestSiteSystemdNaming(t *testing.T) {
	if got := siteAppUnitName("API.FikFikk.My.ID"); got != "fikpanel-app-api-fikfikk-my-id.service" {
		t.Fatalf("siteAppUnitName = %q", got)
	}
	if got := siteQueueUnitName("shop.example.com"); got != "fikpanel-queue-shop-example-com.service" {
		t.Fatalf("siteQueueUnitName = %q", got)
	}
}

func TestVhostProxyAndSPARendering(t *testing.T) {
	mgr := sites.NewManager(sites.Paths{VhostDir: "/vh", LogDir: "/log", ACMEWebroot: "/acme", CertDir: "/ssl"}, nil, nil)

	// Proxy/Node tidak boleh mencegat respons error JSON API dan wajib mengizinkan upload 64 MB
	apiSite := &store.Site{Domain: "api.test.id", Aliases: `[]`, Kind: "go", Upstream: "http://127.0.0.1:8100"}
	out, err := mgr.Render(apiSite, nil)
	if err != nil {
		t.Fatal(err)
	}
	if contains := stringContainsAll(out, "client_max_body_size 64m;", "proxy_pass http://127.0.0.1:8100;", "proxy_read_timeout 120s;"); !contains {
		t.Errorf("vhost API kekurangan direktif penting:\n%s", out)
	}
	if stringContainsAll(out, "proxy_intercept_errors on;") {
		t.Errorf("vhost API/proxy tidak boleh menyalakan proxy_intercept_errors on")
	}

	// Situs Static SPA otomatis memakai try_files /index.html
	spaSite := &store.Site{Domain: "web.test.id", Aliases: `[]`, Kind: "static", RootPath: "/home/fikpanel/static/web.test.id/dist", Framework: "Vite SPA"}
	outSPA, err := mgr.Render(spaSite, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !stringContainsAll(outSPA, "try_files $uri $uri/ /index.html;") {
		t.Errorf("vhost Static SPA harus memakai fallback /index.html:\n%s", outSPA)
	}
}

func stringContainsAll(haystack string, needles ...string) bool {
	for _, n := range needles {
		if !containsSubstring(haystack, n) {
			return false
		}
	}
	return true
}

func containsSubstring(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexString(s, sub) >= 0)
}

func indexString(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
