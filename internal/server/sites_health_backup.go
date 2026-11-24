package server

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fikpanel/internal/sites"
	"fikpanel/internal/store"
)

// extractDialHostPort mengekstrak alamat host:port bersih dari URL upstream situs
func extractDialHostPort(rawUpstream string) (host, port, display string) {
	trimmed := strings.TrimSpace(rawUpstream)
	if trimmed == "" {
		return "", "", ""
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" {
		return "", "", ""
	}
	h := u.Hostname()
	p := u.Port()
	if p == "" {
		if strings.EqualFold(u.Scheme, "https") {
			p = "443"
		} else {
			p = "80"
		}
	}
	if h == "" {
		return "", "", ""
	}
	return h, p, net.JoinHostPort(h, p)
}

// isLocalUpstreamHost memeriksa apakah alamat upstream mengarah ke loopback lokal (umum pada Cloudflare Tunnel / PM2 / Go service)
func isLocalUpstreamHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	return h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "0.0.0.0"
}

// dialUpstreamTCP mencoba membuka koneksi TCP singkat ke upstream dengan fallback IPv4/IPv6 untuk localhost
func dialUpstreamTCP(host, port string, timeout time.Duration) bool {
	if host == "" || port == "" {
		return false
	}
	candidates := []string{net.JoinHostPort(host, port)}
	if strings.EqualFold(host, "localhost") {
		// Uji 127.0.0.1 lebih dahulu lalu [::1] agar tidak gagal saat layanan hanya listen pada salah satu protokol
		candidates = []string{net.JoinHostPort("127.0.0.1", port), net.JoinHostPort("::1", port)}
	}
	for _, addr := range candidates {
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}

// probePHPFPMUnixSocket memeriksa apakah socket Unix PHP-FPM versi situs aktif menerima koneksi
func probePHPFPMUnixSocket(phpVersion string, timeout time.Duration) (ok bool, label string) {
	ver := strings.TrimSpace(phpVersion)
	var candidates []string
	if ver != "" {
		candidates = append(candidates, fmt.Sprintf("/run/php/php%s-fpm.sock", ver))
		label = ver
	} else {
		candidates = append(candidates, "/run/php/php-fpm.sock")
		if matches, err := filepath.Glob("/run/php/php*-fpm.sock"); err == nil {
			candidates = append(candidates, matches...)
		}
	}
	for _, sock := range candidates {
		conn, err := net.DialTimeout("unix", sock, timeout)
		if err == nil {
			_ = conn.Close()
			if label == "" {
				base := filepath.Base(sock)
				label = strings.TrimSuffix(strings.TrimPrefix(base, "php"), "-fpm.sock")
			}
			return true, label
		}
	}
	return false, label
}

// probeSiteLiveStatus memeriksa kesehatan operasional aktual situs (TCP upstream, socket PHP-FPM, dan status Nginx)
func probeSiteLiveStatus(site *store.Site, nginxRunning bool) (status, detail string) {
	if site == nil {
		return "stopped", "Situs tidak valid"
	}
	if site.Enabled == 0 {
		return "stopped", "Vhost dinonaktifkan secara manual"
	}

	// 1. Situs berbasis upstream TCP (Proxy, Node.js, Go, Python)
	if strings.TrimSpace(site.Upstream) != "" {
		host, port, display := extractDialHostPort(site.Upstream)
		if host != "" && port != "" {
			if dialUpstreamTCP(host, port, 250*time.Millisecond) {
				if !nginxRunning && site.SSLMode != "cloudflare" {
					return "degraded", fmt.Sprintf("Upstream %s aktif, namun layanan Nginx mati", display)
				}
				return "running", fmt.Sprintf("Upstream %s aktif merespons", display)
			}
			return "down", fmt.Sprintf("Upstream %s tidak merespons koneksi", display)
		}
	}

	// 2. Situs berbasis PHP-FPM
	if site.Kind == "php" {
		if !nginxRunning {
			return "down", "Layanan Nginx tidak aktif"
		}
		if runtime.GOOS != "linux" {
			return "running", "Nginx aktif (mode pengembangan)"
		}
		ok, ver := probePHPFPMUnixSocket(site.PHPVersion, 150*time.Millisecond)
		verText := "PHP-FPM"
		if ver != "" {
			verText = "PHP-FPM " + ver
		}
		if ok {
			return "running", fmt.Sprintf("Nginx + %s aktif", verText)
		}
		return "degraded", fmt.Sprintf("Socket %s tidak aktif atau tidak ditemukan", verText)
	}

	// 3. Situs statis atau proyek tanpa upstream TCP eksplisit
	if !nginxRunning {
		return "down", "Layanan Nginx tidak aktif"
	}
	if runtime.GOOS == "linux" && strings.TrimSpace(site.RootPath) != "" {
		if st, err := os.Stat(site.RootPath); err != nil || !st.IsDir() {
			return "degraded", "Direktori root situs tidak ditemukan"
		}
	}
	return "running", "Nginx aktif melayani situs"
}

// countFileWithRotations menghitung total baris pada berkas log utama serta arsip rotasi (.1 dan .*.gz)
func countFileWithRotations(basePath string) (count int64, exists bool) {
	for _, p := range []string{basePath, basePath + ".1"} {
		if f, err := os.Open(p); err == nil {
			exists = true
			count += countReaderLines(f)
			_ = f.Close()
		}
	}
	if gzMatches, err := filepath.Glob(basePath + ".*.gz"); err == nil {
		for _, gzPath := range gzMatches {
			if f, err := os.Open(gzPath); err == nil {
				exists = true
				if gr, err := gzip.NewReader(f); err == nil {
					count += countReaderLines(gr)
					_ = gr.Close()
				}
				_ = f.Close()
			}
		}
	}
	return count, exists
}

// countDomainInGlobalLog menghitung baris yang memuat domain di ekor log global Nginx (dibatasi 4 MB demi hemat RAM)
func countDomainInGlobalLog(globalLogPath, domain string) int64 {
	target := strings.ToLower(strings.TrimSpace(domain))
	if target == "" {
		return 0
	}
	f, err := os.Open(globalLogPath)
	if err != nil {
		return 0
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil || st.IsDir() || st.Size() == 0 {
		return 0
	}

	const maxScanBytes int64 = 4 * 1024 * 1024
	if st.Size() > maxScanBytes {
		if _, err := f.Seek(st.Size()-maxScanBytes, io.SeekStart); err != nil {
			return 0
		}
	}

	var count int64
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 32*1024), 512*1024)
	firstSkipped := st.Size() > maxScanBytes
	for scanner.Scan() {
		if firstSkipped {
			firstSkipped = false
			continue
		}
		line := strings.ToLower(scanner.Text())
		if strings.Contains(line, target) {
			count++
		}
	}
	return count
}

// countSiteRequestsInDirs menghitung permintaan HTTP situs lintas seluruh vhost domain (conf.d + sites-enabled), log global, waf_events, dan deteksi tunnel
func countSiteRequestsInDirs(site *store.Site, logDir, vhostDir string, wafCounter func(string) int64) (int64, string) {
	if site == nil || strings.TrimSpace(site.Domain) == "" {
		return 0, "none"
	}

	// 1. Gabungkan & jumlahkan seluruh berkas access_log khusus milik domain di conf.d maupun sites-enabled
	primaryPath := filepath.Join(logDir, site.Domain+".access.log")
	seenPath := map[string]bool{}
	var dedicatedPaths []string
	var sharedLogPaths []string

	addLogPath := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		clean := filepath.Clean(p)
		if seenPath[clean] {
			return
		}
		seenPath[clean] = true
		if strings.EqualFold(filepath.Base(p), "access.log") {
			sharedLogPaths = append(sharedLogPaths, p)
		} else {
			dedicatedPaths = append(dedicatedPaths, p)
		}
	}

	addLogPath(primaryPath)
	discoveredAccess, _ := sites.DiscoverAllDomainLogPaths(site.Domain, vhostDir)
	for _, p := range discoveredAccess {
		addLogPath(p)
	}

	var totalDedicated int64
	anyDedicatedExists := false
	for _, p := range dedicatedPaths {
		cnt, exists := countFileWithRotations(p)
		if exists {
			anyDedicatedExists = true
		}
		totalDedicated += cnt
	}
	if totalDedicated > 0 {
		return totalDedicated, "nginx"
	}

	// 2. Cek log akses global Nginx (termasuk path global kustom dari vhost) dengan penyaringan domain
	addLogPath(filepath.Join(logDir, "access.log"))
	var totalGlobal int64
	for _, gp := range sharedLogPaths {
		totalGlobal += countDomainInGlobalLog(gp, site.Domain)
	}
	if totalGlobal > 0 {
		return totalGlobal, "global_log"
	}

	// 3. Cek catatan insiden/trafik di tabel waf_events
	if wafCounter != nil {
		if n := wafCounter(site.Domain); n > 0 {
			return n, "waf"
		}
	}

	// 4. Deteksi jujur situs Cloudflare Tunnel / direct localhost port yang tidak menerima trafik lewat Nginx
	// Tetap berlaku meski file kosong 0-byte <domain>.access.log dibuat Nginx dari vhost conf.d
	host, port, _ := extractDialHostPort(site.Upstream)
	if host != "" && isLocalUpstreamHost(host) && port != "80" && port != "443" {
		if site.SSLMode == "cloudflare" || site.Kind == "proxy" {
			return 0, "tunnel"
		}
	}

	if anyDedicatedExists {
		return 0, "nginx"
	}
	return 0, "none"
}

// countSiteRequests menghitung permintaan HTTP riil situs menggunakan konfigurasi direktori server & SQLite WAF
func (s *Server) countSiteRequests(ctx context.Context, site *store.Site) (int64, string) {
	logDir := "/var/log/nginx"
	vhostDir := "/etc/nginx/conf.d"
	if runtime.GOOS != "linux" {
		logDir = filepath.Join("dev-data", "nginx", "logs")
		vhostDir = filepath.Join("dev-data", "nginx", "conf.d")
	}
	if s.sites != nil {
		if d := s.sites.LogDir(); d != "" {
			logDir = d
		}
		if d := s.sites.VhostDir(); d != "" {
			vhostDir = d
		}
	}
	wafCounter := func(domain string) int64 {
		if s.store == nil || s.store.DB() == nil {
			return 0
		}
		var n int64
		_ = s.store.DB().QueryRowContext(ctx, `SELECT COUNT(1) FROM waf_events WHERE LOWER(site_name) = LOWER(?)`, domain).Scan(&n)
		return n
	}
	return countSiteRequestsInDirs(site, logDir, vhostDir, wafCounter)
}

// primarySiteBackupDir menentukan direktori penyimpanan arsip cadangan situs (dapat ditimpa lewat FIKPANEL_BACKUP_DIR untuk uji)
func primarySiteBackupDir() string {
	if envDir := strings.TrimSpace(os.Getenv("FIKPANEL_BACKUP_DIR")); envDir != "" {
		return envDir
	}
	if runtime.GOOS != "linux" {
		return filepath.Join("dev-data", "backups")
	}
	return "/var/backups/fikpanel"
}

// siteBackupSearchDirs mengembalikan seluruh direktori tempat berkas cadangan situs dicari
func siteBackupSearchDirs() []string {
	var dirs []string
	seen := map[string]bool{}
	add := func(d string) {
		d = strings.TrimSpace(d)
		if d == "" || seen[d] {
			return
		}
		seen[d] = true
		dirs = append(dirs, d)
	}
	add(primarySiteBackupDir())
	if runtime.GOOS == "linux" {
		add("/var/backups/fikpanel")
		add("/home/fikpanel/backups")
	}
	return dirs
}

// createSiteTarGzGo membuat arsip .tar.gz secara lintas-platform menggunakan stdlib Go sambil melewati direktori cache berat
func createSiteTarGzGo(srcDir, dstPath string) error {
	outFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("gagal membuat berkas arsip: %w", err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	cleanSrc := filepath.Clean(srcDir)
	return filepath.Walk(cleanSrc, func(currPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(cleanSrc, currPath)
		if err != nil || rel == "." {
			return nil
		}
		slashRel := filepath.ToSlash(rel)
		if info.IsDir() {
			base := info.Name()
			if base == "node_modules" || base == ".cache" || slashRel == "storage/framework/cache" || slashRel == "storage/framework/views" {
				return filepath.SkipDir
			}
		}
		// Lewati symlink atau berkas khusus selain file biasa dan direktori
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return nil
		}
		hdr.Name = slashRel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(currPath)
		if err != nil {
			return nil
		}
		_, copyErr := io.Copy(tw, f)
		_ = f.Close()
		return copyErr
	})
}

// handleBackupSite membuat arsip cadangan nyata (.tar.gz) untuk direktori proyek situs dan (opsional) dump database MySQL dari .env
func (s *Server) handleBackupSite(w http.ResponseWriter, r *http.Request) {
	id, err := siteID(r)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	site, err := s.store.GetSite(r.Context(), id)
	if err != nil {
		writeSiteError(w, err)
		return
	}

	projectDir := resolveSiteProjectDir(site)
	if projectDir == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Direktori proyek situs belum terkonfigurasi untuk dicadangkan"})
		return
	}
	st, err := os.Stat(projectDir)
	if err != nil || !st.IsDir() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Direktori proyek tidak ditemukan: %s", projectDir)})
		return
	}

	// Nonaktifkan WriteDeadline agar pengarsipan proyek besar di ARM64 tidak terpotong batas waktu HTTP standar
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})

	bCtx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	backupDir := primarySiteBackupDir()
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal membuat direktori backup: " + err.Error()})
		return
	}

	ts := time.Now().Format("20060102-150405")
	safeDom := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_").Replace(site.Domain)
	fileName := fmt.Sprintf("%s-%s.tar.gz", safeDom, ts)
	dstPath := filepath.Join(backupDir, fileName)

	// Deteksi & dump database MySQL opsional bila file .env mencantumkan DB_DATABASE pada host Linux
	dbIncluded := false
	var tmpSQLDir, tmpSQLName string
	if runtime.GOOS == "linux" {
		dbName := ""
		if envBytes, err := os.ReadFile(filepath.Join(projectDir, ".env")); err == nil {
			for _, line := range strings.Split(string(envBytes), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "DB_DATABASE=") {
					dbName = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "DB_DATABASE=")), `"'`)
					break
				}
			}
		}
		if dbName != "" {
			if dir, err := os.MkdirTemp("", "fikpanel-sqldump-*"); err == nil {
				tmpSQLDir = dir
				defer os.RemoveAll(tmpSQLDir)
				tmpSQLName = fmt.Sprintf("database-%s.sql", dbName)
				sqlOutPath := filepath.Join(tmpSQLDir, tmpSQLName)
				var dumpCmd *exec.Cmd
				if _, err := os.Stat("/etc/mysql/debian.cnf"); err == nil {
					dumpCmd = exec.CommandContext(bCtx, "mysqldump", "--defaults-file=/etc/mysql/debian.cnf", "--single-transaction", "--routines", "--triggers", dbName)
				} else {
					dumpCmd = exec.CommandContext(bCtx, "mysqldump", "-u", "root", "--single-transaction", "--routines", "--triggers", dbName)
				}
				if outF, err := os.Create(sqlOutPath); err == nil {
					dumpCmd.Stdout = outF
					if dumpErr := dumpCmd.Run(); dumpErr == nil {
						if fi, statErr := outF.Stat(); statErr == nil && fi.Size() > 0 {
							dbIncluded = true
						}
					}
					_ = outF.Close()
				}
			}
		}
	}

	var archiveErr error
	if runtime.GOOS == "linux" {
		args := []string{
			"--exclude=node_modules",
			"--exclude=.cache",
			"--exclude=storage/framework/cache",
			"--exclude=storage/framework/views",
			"-czf", dstPath,
			"-C", projectDir, ".",
		}
		if dbIncluded && tmpSQLDir != "" && tmpSQLName != "" {
			args = append(args, "-C", tmpSQLDir, tmpSQLName)
		}
		cmd := exec.CommandContext(bCtx, "tar", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			archiveErr = fmt.Errorf("tar gagal: %v (%s)", err, strings.TrimSpace(string(out)))
		}
	} else {
		archiveErr = createSiteTarGzGo(projectDir, dstPath)
	}

	if archiveErr != nil {
		_ = os.Remove(dstPath)
		s.auditSite(r, "site.backup", site.Domain, false, archiveErr.Error())
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal membuat cadangan situs: " + archiveErr.Error()})
		return
	}

	var sizeBytes int64
	if fi, err := os.Stat(dstPath); err == nil {
		sizeBytes = fi.Size()
	}
	backupCount := countDomainBackups(site.Domain)
	s.auditSite(r, "site.backup", site.Domain, true, fmt.Sprintf("file=%s size=%d db=%v", fileName, sizeBytes, dbIncluded))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":           true,
		"domain":       site.Domain,
		"file_name":    fileName,
		"path":         filepath.ToSlash(dstPath),
		"size_bytes":   sizeBytes,
		"db_included":  dbIncluded,
		"backup_count": backupCount,
		"message":      fmt.Sprintf("Cadangan %s berhasil disimpan (%s)", site.Domain, fileName),
	})
}
