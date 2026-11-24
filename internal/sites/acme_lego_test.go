package sites

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"fikpanel/internal/store"
)

func TestLegoDNSProviderBuilderAndWildcardExpansion(t *testing.T) {
	// 1. Uji ekspansi wildcard tanpa duplikasi
	got := expandDomainsWithWildcard([]string{"example.com", "www.example.com", "example.com"}, true)
	if len(got) != 3 || got[2] != "*.example.com" {
		t.Fatalf("ekspansi wildcard tidak sesuai: %+v", got)
	}
	gotNoDup := expandDomainsWithWildcard([]string{"example.com", "*.example.com"}, true)
	if len(gotNoDup) != 2 {
		t.Fatalf("wildcard duplikat seharusnya tidak ditambahkan: %+v", gotNoDup)
	}

	// 2. Uji validasi 5 DNS provider ringan dengan kredensial valid & kosong
	if _, err := buildLegoDNSProvider("cloudflare", map[string]string{}); err == nil {
		t.Fatalf("harapan error bila kredensial Cloudflare kosong")
	}
	if p, err := buildLegoDNSProvider("cloudflare", map[string]string{"api_token": "cf-test-token-123456"}); err != nil || p == nil {
		t.Fatalf("Cloudflare API Token gagal dibangun: %v", err)
	}
	if p, err := buildLegoDNSProvider("digitalocean", map[string]string{"auth_token": "do-test-token"}); err != nil || p == nil {
		t.Fatalf("DigitalOcean provider gagal dibangun: %v", err)
	}
	if p, err := buildLegoDNSProvider("porkbun", map[string]string{"api_key": "pk1_abc", "secret_api_key": "sk1_xyz"}); err != nil || p == nil {
		t.Fatalf("Porkbun provider gagal dibangun: %v", err)
	}
	if p, err := buildLegoDNSProvider("duckdns", map[string]string{"token": "duck-token"}); err != nil || p == nil {
		t.Fatalf("DuckDNS provider gagal dibangun: %v", err)
	}
	if _, err := buildLegoDNSProvider("unknown_provider", nil); err == nil {
		t.Fatalf("harapan error untuk provider tidak dikenal")
	}
}

func TestACMEDNSProviderStorePersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_lego_store_*")
	if err != nil {
		t.Fatalf("gagal membuat tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	st, err := store.Open(filepath.Join(tempDir, "lego.db"))
	if err != nil {
		t.Fatalf("gagal membuka store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	if err := st.SaveACMEDNSProvider(ctx, "porkbun", map[string]string{"api_key": "pk1_test", "secret_api_key": "sk1_test"}); err != nil {
		t.Fatalf("SaveACMEDNSProvider gagal: %v", err)
	}
	creds, src, err := st.GetACMEDNSProvider(ctx, "porkbun")
	if err != nil || src != "saved" || creds["api_key"] != "pk1_test" {
		t.Fatalf("GetACMEDNSProvider tidak sesuai: creds=%+v src=%q err=%v", creds, src, err)
	}

	configured := st.ListConfiguredACMEDNSProviders(ctx)
	providers := SupportedDNSProviders(configured)
	if len(providers) != 5 {
		t.Fatalf("harapan 5 DNS providers, dapat %d", len(providers))
	}
	foundPB := false
	for _, p := range providers {
		if p.ID == "porkbun" && p.Configured && p.Source == "saved" {
			foundPB = true
		}
	}
	if !foundPB {
		t.Fatalf("porkbun seharusnya berstatus configured=true")
	}
}
