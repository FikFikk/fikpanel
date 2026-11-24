package updater

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveLatestTagRetryAndAPIFallback(t *testing.T) {
	// 1. Sukses setelah 2 kali gagal 503 sementara (verifikasi retry 3x)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Location", "https://github.com/FikFikk/fikpanel/releases/tag/v0.6.5")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	cli := NewClient("test-updater")
	cli.BaseURL = srv.URL
	cli.APIBaseURL = ""
	cli.HTTP = srv.Client()
	cli.IPv4HTTP = srv.Client()
	cli.RetryDelay = 10 * time.Millisecond

	tag, err := cli.ResolveLatestTag(context.Background())
	if err != nil {
		t.Fatalf("ResolveLatestTag seharusnya sukses pada percobaan ke-3: %v", err)
	}
	if tag != "v0.6.5" || hits.Load() != 3 {
		t.Fatalf("Hasil tidak sesuai: tag=%q hits=%d", tag, hits.Load())
	}

	// 2. Fallback ke GitHub API jika github.com/releases/latest terus 503
	badWeb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer badWeb.Close()

	goodAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.6.6","draft":false,"prerelease":false}`))
	}))
	defer goodAPI.Close()

	cli2 := NewClient("test-updater")
	cli2.BaseURL = badWeb.URL
	cli2.APIBaseURL = goodAPI.URL
	cli2.HTTP = badWeb.Client()
	cli2.IPv4HTTP = goodAPI.Client()
	cli2.RetryDelay = 10 * time.Millisecond

	tag2, err := cli2.ResolveLatestTag(context.Background())
	if err != nil {
		t.Fatalf("Fallback GitHub API gagal: %v", err)
	}
	if tag2 != "v0.6.6" {
		t.Fatalf("Tag fallback API tidak sesuai: got=%q want=v0.6.6", tag2)
	}
}

func TestDownloadRetryAndNoRetryOn404(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/FikFikk/fikpanel/releases/download/v0.6.5/missing.tar.gz" {
			hits.Add(1)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if hits.Add(1) < 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("payload-ok"))
	}))
	defer srv.Close()

	cli := NewClient("test-updater")
	cli.BaseURL = srv.URL
	cli.HTTP = srv.Client()
	cli.IPv4HTTP = srv.Client()
	cli.RetryDelay = 10 * time.Millisecond

	dst := filepath.Join(t.TempDir(), "asset.bin")
	if err := cli.Download(context.Background(), "v0.6.5", "ok.tar.gz", dst, 1024); err != nil {
		t.Fatalf("Download seharusnya sukses setelah retry: %v", err)
	}
	raw, _ := os.ReadFile(dst)
	if string(raw) != "payload-ok" || hits.Load() != 2 {
		t.Fatalf("Konten/hits tidak sesuai: raw=%q hits=%d", string(raw), hits.Load())
	}

	// 404 tidak boleh di-retry (langsung mengembalikan ErrReleaseNotFound dalam 1 hit)
	hits.Store(0)
	err := cli.Download(context.Background(), "v0.6.5", "missing.tar.gz", dst, 1024)
	if !errors.Is(err, ErrReleaseNotFound) || hits.Load() != 1 {
		t.Fatalf("404 seharusnya langsung gagal tanpa retry: err=%v hits=%d", err, hits.Load())
	}
}
