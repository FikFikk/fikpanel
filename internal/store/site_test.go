package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateSiteWithCloudflareSSL(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_store_cf_test_*")
	if err != nil {
		t.Fatalf("Gagal membuat direktori sementara: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_panel.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Gagal membuka store SQLite: %v", err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	site := Site{
		Domain:      "go.fikfikk.my.id",
		Aliases:     "[]",
		Kind:        "proxy",
		Upstream:    "http://localhost:5000",
		SSLMode:     "cloudflare",
		ForceHTTPS:  0,
		Enabled:     1,
		Description: "Uji adopsi cloudflare tunnel",
	}

	created, err := s.CreateSite(ctx, &site)
	if err != nil {
		t.Fatalf("Gagal membuat situs dengan ssl_mode 'cloudflare': %v", err)
	}
	if created.ID == 0 {
		t.Errorf("ID situs baru bernilai 0")
	}
}
