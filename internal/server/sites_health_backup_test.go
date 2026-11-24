package server

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fikpanel/internal/sites"
	"fikpanel/internal/store"
)

func TestProbeSiteLiveStatus(t *testing.T) {
	// Buka listener TCP lokal untuk mensimulasikan upstream hidup
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Gagal membuat listener TCP uji: %v", err)
	}
	defer ln.Close()
	liveAddr := ln.Addr().String()

	// Cari port tertutup yang tidak menerima koneksi
	lnClosed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Gagal mengalokasi port tertutup: %v", err)
	}
	closedAddr := lnClosed.Addr().String()
	_ = lnClosed.Close()

	// 1. Upstream aktif -> status "running"
	stLive, _ := probeSiteLiveStatus(&store.Site{
		Domain:   "live.example.com",
		Kind:     "proxy",
		Upstream: "http://" + liveAddr,
		Enabled:  1,
	}, true)
	if stLive != "running" {
		t.Fatalf("Diharapkan status running untuk upstream aktif, didapat: %s", stLive)
	}

	// 2. Upstream mati -> status "down"
	stDown, _ := probeSiteLiveStatus(&store.Site{
		Domain:   "down.example.com",
		Kind:     "node",
		Upstream: "http://" + closedAddr,
		Enabled:  1,
	}, true)
	if stDown != "down" {
		t.Fatalf("Diharapkan status down untuk upstream mati, didapat: %s", stDown)
	}

	// 3. Situs dinonaktifkan manual -> status "stopped"
	stStopped, _ := probeSiteLiveStatus(&store.Site{
		Domain:   "paused.example.com",
		Kind:     "proxy",
		Upstream: "http://" + liveAddr,
		Enabled:  0,
	}, true)
	if stStopped != "stopped" {
		t.Fatalf("Diharapkan status stopped saat enabled=0, didapat: %s", stStopped)
	}

	// 4. Situs PHP saat Nginx mati -> status "down"
	stNginxOff, _ := probeSiteLiveStatus(&store.Site{
		Domain:  "php.example.com",
		Kind:    "php",
		Enabled: 1,
	}, false)
	if stNginxOff != "down" {
		t.Fatalf("Diharapkan status down saat Nginx tidak berjalan, didapat: %s", stNginxOff)
	}
}

func TestCountSiteRequestsAndHostLogDiscovery(t *testing.T) {
	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "logs")
	vhostDir := filepath.Join(tmp, "conf.d")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(vhostDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Kasus 1: Vhost host kustom dengan direktif access_log khusus
	customLog := filepath.Join(logDir, "custom-office.log")
	if err := os.WriteFile(customLog, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vhostContent := fmt.Sprintf(`
server {
    listen 80;
    server_name office.example.com www.office.example.com;
    access_log %s combined;
    error_log %s;
}
`, customLog, filepath.Join(logDir, "custom-office.err"))
	if err := os.WriteFile(filepath.Join(vhostDir, "office.conf"), []byte(vhostContent), 0o644); err != nil {
		t.Fatal(err)
	}

	gotAccess, _ := sites.DiscoverDomainLogPaths("office.example.com", vhostDir)
	if filepath.Clean(gotAccess) != filepath.Clean(customLog) {
		t.Fatalf("DiscoverDomainLogPaths salah: got=%s want=%s", gotAccess, customLog)
	}

	cnt1, src1 := countSiteRequestsInDirs(&store.Site{Domain: "office.example.com", Kind: "node"}, logDir, vhostDir, nil)
	if cnt1 != 3 || src1 != "nginx" {
		t.Fatalf("Kasus log vhost kustom salah: count=%d source=%s", cnt1, src1)
	}

	// Kasus 2: Fallback ke log global access.log berdasarkan nama domain
	globalLog := filepath.Join(logDir, "access.log")
	globalLines := strings.Join([]string{
		`127.0.0.1 - - [24/Nov/2026:12:00:00 +0700] "GET https://blog.example.com/ HTTP/1.1" 200 123 "-" "curl"`,
		`127.0.0.1 - - [24/Nov/2026:12:01:00 +0700] "GET https://other.example.com/ HTTP/1.1" 200 123 "-" "curl"`,
		`127.0.0.1 - - [24/Nov/2026:12:02:00 +0700] "GET https://blog.example.com/about HTTP/1.1" 200 456 "-" "curl"`,
	}, "\n") + "\n"
	if err := os.WriteFile(globalLog, []byte(globalLines), 0o644); err != nil {
		t.Fatal(err)
	}

	cnt2, src2 := countSiteRequestsInDirs(&store.Site{Domain: "blog.example.com", Kind: "static"}, logDir, vhostDir, nil)
	if cnt2 != 2 || src2 != "global_log" {
		t.Fatalf("Kasus fallback log global salah: count=%d source=%s", cnt2, src2)
	}

	// Kasus 3: Cloudflare Tunnel langsung ke localhost tanpa log Nginx -> source "tunnel"
	cnt3, src3 := countSiteRequestsInDirs(&store.Site{
		Domain:   "tunnel.example.com",
		Kind:     "proxy",
		Upstream: "http://localhost:8888",
		SSLMode:  "cloudflare",
	}, logDir, vhostDir, nil)
	if cnt3 != 0 || src3 != "tunnel" {
		t.Fatalf("Kasus deteksi tunnel salah: count=%d source=%s", cnt3, src3)
	}
}

func TestHandleBackupSiteEndpoint(t *testing.T) {
	backupDir := t.TempDir()
	t.Setenv("FIKPANEL_BACKUP_DIR", backupDir)

	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "index.php"), []byte("<?php echo 'ok';"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "node_modules", "pkg", "ignored.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, cookie, cleanup := setupTestServerWithCookie(t)
	defer cleanup()

	site, err := srv.store.CreateSite(t.Context(), &store.Site{
		Domain:     "backup-test.example.com",
		Aliases:    "[]",
		Kind:       "php",
		RootPath:   projectDir,
		PHPVersion: "8.3",
		SSLMode:    "none",
		Enabled:    1,
	})
	if err != nil {
		t.Fatalf("Gagal membuat situs uji: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/sites/%d/backup", site.ID), nil)
	req.AddCookie(cookie)
	req.Header.Set("X-Panel-Request", "1")
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/sites/{id}/backup gagal: status=%d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		OK          bool   `json:"ok"`
		FileName    string `json:"file_name"`
		Path        string `json:"path"`
		SizeBytes   int64  `json:"size_bytes"`
		BackupCount int    `json:"backup_count"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode respon backup gagal: %v", err)
	}
	if !resp.OK || resp.SizeBytes <= 0 || resp.BackupCount != 1 || !strings.HasSuffix(resp.FileName, ".tar.gz") {
		t.Fatalf("Respon backup tidak valid: %+v", resp)
	}

	// Verifikasi isi arsip .tar.gz valid: memuat index.php dan mengecualikan node_modules
	f, err := os.Open(filepath.Join(backupDir, resp.FileName))
	if err != nil {
		t.Fatalf("Berkas arsip backup tidak ditemukan: %v", err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("Arsip bukan gzip valid: %v", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	foundIndex := false
	foundNodeModules := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Gagal membaca entri tar: %v", err)
		}
		if hdr.Name == "index.php" {
			foundIndex = true
		}
		if strings.Contains(hdr.Name, "node_modules") {
			foundNodeModules = true
		}
	}
	if !foundIndex {
		t.Fatalf("index.php seharusnya ada di dalam arsip backup")
	}
	if foundNodeModules {
		t.Fatalf("node_modules seharusnya dikecualikan dari arsip backup")
	}
}
