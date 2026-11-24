package sites

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/nginx"
	"fikpanel/internal/store"
)

// Paths berisi lokasi file yang dikelola modul situs
type Paths struct {
	VhostDir    string
	LogDir      string
	ACMEWebroot string
	CertDir     string
	AccountKey  string
	ErrorDir    string
}

// DefaultPaths mengembalikan lokasi standar Linux atau folder dev-data di non-Linux
func DefaultPaths() Paths {
	p := Paths{
		VhostDir:    "/etc/nginx/conf.d",
		LogDir:      "/var/log/nginx",
		ACMEWebroot: "/var/lib/fikpanel/acme-webroot",
		CertDir:     "/etc/fikpanel/ssl",
		AccountKey:  "/var/lib/fikpanel/acme/account.key",
		ErrorDir:    "/var/lib/fikpanel/error_pages",
	}
	if runtime.GOOS != "linux" {
		base := filepath.Join("dev-data", "nginx")
		p = Paths{
			VhostDir:    filepath.Join(base, "conf.d"),
			LogDir:      filepath.Join(base, "logs"),
			ACMEWebroot: filepath.Join(base, "acme-webroot"),
			CertDir:     filepath.Join(base, "ssl"),
			AccountKey:  filepath.Join(base, "acme", "account.key"),
			ErrorDir:    filepath.Join(base, "error_pages"),
		}
	}
	if v := os.Getenv("FIKPANEL_VHOST_DIR"); v != "" {
		p.VhostDir = v
	}
	if v := os.Getenv("FIKPANEL_CERT_DIR"); v != "" {
		p.CertDir = v
	}
	if v := os.Getenv("FIKPANEL_ERROR_DIR"); v != "" {
		p.ErrorDir = v
	}
	return p
}

// Manager mengelola siklus hidup vhost dan sertifikat situs
type Manager struct {
	paths Paths
	nginx *nginx.Manager
	store *store.Store
	mock  bool
	ipc   ipCache

	// applyMu menserialisasi penulisan vhost + reload agar tidak saling tumpang tindih
	applyMu sync.Mutex

	jobsMu sync.Mutex
	jobs   map[int64]SSLJob
}

// NewManager membuat manager situs
func NewManager(p Paths, ngx *nginx.Manager, st *store.Store) *Manager {
	if runtime.GOOS == "linux" {
		// Pastikan permission direktori ACMEWebroot dan parent-nya 0755 agar dapat dibaca Nginx (www-data)
		_ = os.MkdirAll(p.ACMEWebroot, 0o755)
		_ = os.Chmod(filepath.Dir(p.ACMEWebroot), 0o755)
		_ = os.Chmod(p.ACMEWebroot, 0o755)
	}
	// Pastikan berkas template error sistem tersedia
	_ = EnsureErrorPages(p.ErrorDir)

	return &Manager{
		paths: p,
		nginx: ngx,
		store: st,
		mock:  runtime.GOOS != "linux",
		jobs:  map[int64]SSLJob{},
	}
}

// vhostFiles mengembalikan path file vhost aktif dan nonaktif untuk domain
func (m *Manager) vhostFiles(domain string) (string, string) {
	base := filepath.Join(m.paths.VhostDir, "fikpanel-"+domain+".conf")
	return base, base + ".disabled"
}

// certExists memeriksa keberadaan file sertifikat (selalu true di mode simulasi)
func (m *Manager) certExists(domain string) bool {
	if m.mock {
		return true
	}
	certPath, keyPath := m.certFiles(domain)
	_, e1 := os.Stat(certPath)
	_, e2 := os.Stat(keyPath)
	return e1 == nil && e2 == nil
}

// fileSnapshot menyimpan isi file sebelum diubah untuk keperluan rollback
type fileSnapshot struct {
	path   string
	data   []byte
	exists bool
}

func snapshot(path string) fileSnapshot {
	data, err := os.ReadFile(path)
	return fileSnapshot{path: path, data: data, exists: err == nil}
}

func (s fileSnapshot) restore() {
	if s.exists {
		_ = os.WriteFile(s.path, s.data, 0o644)
		return
	}
	_ = os.Remove(s.path)
}

// Apply merender dan menulis vhost situs, menguji nginx, lalu reload; rollback otomatis bila gagal
func (m *Manager) Apply(ctx context.Context, site *store.Site, previousDomain string) error {
	m.applyMu.Lock()
	defer m.applyMu.Unlock()

	cert, err := m.store.GetCertificate(ctx, site.ID)
	if err != nil {
		return err
	}
	content, err := m.Render(site, cert)
	if err != nil {
		return err
	}
	if err := nginx.CheckSyntax(content); err != nil {
		return err
	}
	if err := os.MkdirAll(m.paths.VhostDir, 0o755); err != nil {
		return fmt.Errorf("gagal membuat folder vhost: %w", err)
	}

	active, disabled := m.vhostFiles(site.Domain)
	snaps := []fileSnapshot{snapshot(active), snapshot(disabled)}
	// Bila domain utama berganti, file lama ikut dihapus dan disiapkan untuk rollback
	if previousDomain != "" && previousDomain != site.Domain {
		oldA, oldD := m.vhostFiles(previousDomain)
		snaps = append(snaps, snapshot(oldA), snapshot(oldD))
		_ = os.Remove(oldA)
		_ = os.Remove(oldD)
	}

	target, other := active, disabled
	if site.Enabled != 1 {
		target, other = disabled, active
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		rollback(snaps)
		return fmt.Errorf("gagal menulis vhost: %w", err)
	}
	_ = os.Remove(other)

	if err := m.nginx.ApplyChange(ctx); err != nil {
		rollback(snaps)
		return fmt.Errorf("konfigurasi dibatalkan: %w", err)
	}
	return nil
}

func rollback(snaps []fileSnapshot) {
	for _, s := range snaps {
		s.restore()
	}
}

// Remove menghapus file vhost situs lalu menerapkan perubahan ke nginx
func (m *Manager) Remove(ctx context.Context, domain string) error {
	m.applyMu.Lock()
	defer m.applyMu.Unlock()

	active, disabled := m.vhostFiles(domain)
	for _, f := range []string{active, disabled} {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("gagal menghapus vhost: %w", err)
		}
	}
	return m.nginx.ApplyChange(ctx)
}

// EnsureRoot membuat direktori proyek dan berkas awal sesuai runtime bila belum ada
func (m *Manager) EnsureRoot(site *store.Site) error {
	if m.mock || site.RootPath == "" {
		return nil
	}
	if err := os.MkdirAll(site.RootPath, 0o755); err != nil {
		return fmt.Errorf("gagal membuat direktori situs: %w", err)
	}

	// Pastikan direktori induk hingga folder proyek dapat diakses oleh worker Nginx (www-data)
	if runtime.GOOS == "linux" && strings.HasPrefix(site.RootPath, "/home/") {
		parts := strings.Split(strings.TrimPrefix(site.RootPath, "/"), "/")
		curr := ""
		for _, part := range parts {
			curr += "/" + part
			_ = os.Chmod(curr, 0o755)
		}
	}

	// Buat folder logs di sebelah root atau di dalam direktori proyek
	parent := filepath.Dir(site.RootPath)
	if filepath.Base(site.RootPath) == "public" {
		_ = os.MkdirAll(filepath.Join(parent, "logs"), 0o755)
	} else {
		_ = os.MkdirAll(filepath.Join(site.RootPath, "logs"), 0o755)
	}

	entries, err := os.ReadDir(site.RootPath)
	if err != nil {
		return fmt.Errorf("gagal membaca direktori situs: %w", err)
	}
	if len(entries) > 0 {
		return nil
	}

	// Berikan template awal ramah developer sesuai runtime
	switch site.Kind {
	case "node":
		pkg := fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "main": "server.js",
  "scripts": {
    "start": "node server.js"
  }
}
`, site.Domain)
		_ = os.WriteFile(filepath.Join(site.RootPath, "package.json"), []byte(pkg), 0o644)
		serverJS := fmt.Sprintf(`const http = require('http');
const server = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
  res.end('<h1>%s</h1><p>Aplikasi Node.js aktif di <b>FikPanel</b>.</p>');
});
server.listen(3000, () => console.log('Server berjalan pada port 3000'));
`, site.Domain)
		_ = os.WriteFile(filepath.Join(site.RootPath, "server.js"), []byte(serverJS), 0o644)
	case "go":
		mainGo := fmt.Sprintf(`package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<h1>%s</h1><p>Aplikasi Golang aktif di <b>FikPanel</b>.</p>")
	})
	http.ListenAndServe(":8080", nil)
}
`, site.Domain)
		_ = os.WriteFile(filepath.Join(site.RootPath, "main.go"), []byte(mainGo), 0o644)
	case "python":
		appPy := fmt.Sprintf(`from http.server import SimpleHTTPRequestHandler, HTTPServer

class Handler(SimpleHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-type', 'text/html; charset=utf-8')
        self.end_headers()
        self.wfile.write(b'<h1>%s</h1><p>Aplikasi Python aktif di <b>FikPanel</b>.</p>')

if __name__ == '__main__':
    server = HTTPServer(('127.0.0.1', 8000), Handler)
    print("Python app running on port 8000...")
    server.serve_forever()
`, site.Domain)
		_ = os.WriteFile(filepath.Join(site.RootPath, "main.py"), []byte(appPy), 0o644)
	case "php":
		// Buat berkas index.php dan index.html selamat datang agar langsung melayani traffic via PHP-FPM
		indexPHP := filepath.Join(site.RootPath, "index.php")
		_ = os.WriteFile(indexPHP, []byte(RenderWelcomePage(site.Domain)), 0o644)
		indexHTML := filepath.Join(site.RootPath, "index.html")
		_ = os.WriteFile(indexHTML, []byte(RenderWelcomePage(site.Domain)), 0o644)
	default:
		index := filepath.Join(site.RootPath, "index.html")
		_ = os.WriteFile(index, []byte(RenderWelcomePage(site.Domain)), 0o644)
	}

	return nil
}

// PHPVersions mendeteksi versi PHP-FPM terpasang dari socket/konfigurasi Debian
func (m *Manager) PHPVersions() []string {
	if m.mock {
		return []string{"8.3", "8.2", "8.1", "7.4"}
	}
	set := map[string]bool{}
	dirs, _ := filepath.Glob("/etc/php/*/fpm")
	for _, d := range dirs {
		v := filepath.Base(filepath.Dir(d))
		if phpVersionRe.MatchString(v) {
			set[v] = true
		}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	// Versi terbaru di urutan pertama
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

// DefaultRoot menghasilkan direktori web root standar untuk domain sesuai runtime dan user
func DefaultRoot(domain, kind, user string) string {
	clean := strings.TrimPrefix(domain, "*.")
	if user == "" {
		user = "fikpanel"
	}
	switch kind {
	case "php":
		return fmt.Sprintf("/home/%s/web/%s/public", user, clean)
	case "static":
		return fmt.Sprintf("/home/%s/static/%s/public", user, clean)
	case "node":
		return fmt.Sprintf("/home/%s/node/%s", user, clean)
	case "go":
		return fmt.Sprintf("/home/%s/go/%s", user, clean)
	case "python":
		return fmt.Sprintf("/home/%s/python/%s", user, clean)
	default:
		return fmt.Sprintf("/home/%s/web/%s/public", user, clean)
	}
}

// GetCertPair membaca file sertifikat fullchain dan privkey domain jika ada
func (m *Manager) GetCertPair(domain string) (string, string, error) {
	certPath, keyPath := m.certFiles(domain)
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return "", "", err
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return string(certData), "", err
	}
	return string(certData), string(keyData), nil
}

// SaveCustomCert memvalidasi dan menyimpan sertifikat kustom pengguna
func (m *Manager) SaveCustomCert(ctx context.Context, siteID int64, domain, keyPEM, certPEM string) (*store.Certificate, error) {
	keyPEM = strings.TrimSpace(keyPEM)
	certPEM = strings.TrimSpace(certPEM)
	if keyPEM == "" || certPEM == "" {
		return nil, &ValidationError{Msg: "kunci privat dan sertifikat CRT/PEM wajib diisi"}
	}

	// Validasi pasangan kunci dan sertifikat TLS
	tlsCert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, &ValidationError{Msg: fmt.Sprintf("pasangan sertifikat/kunci tidak valid: %v", err)}
	}
	if len(tlsCert.Certificate) == 0 {
		return nil, &ValidationError{Msg: "sertifikat tidak memuat blok data X.509"}
	}

	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, &ValidationError{Msg: fmt.Sprintf("gagal membedah sertifikat X.509: %v", err)}
	}

	certPath, keyPath := m.certFiles(domain)
	if err := os.MkdirAll(filepath.Dir(certPath), 0o700); err != nil {
		return nil, fmt.Errorf("gagal membuat folder sertifikat: %w", err)
	}

	if err := atomicWrite(keyPath, []byte(keyPEM), 0o600); err != nil {
		return nil, fmt.Errorf("gagal menulis kunci privat: %w", err)
	}
	if err := atomicWrite(certPath, []byte(certPEM), 0o644); err != nil {
		return nil, fmt.Errorf("gagal menulis sertifikat: %w", err)
	}

	sans, _ := json.Marshal(leaf.DNSNames)
	issuer := leaf.Issuer.CommonName
	if issuer == "" && len(leaf.Issuer.Organization) > 0 {
		issuer = leaf.Issuer.Organization[0]
	}
	if issuer == "" {
		issuer = leaf.Issuer.String()
	}

	cert := &store.Certificate{
		SiteID:    siteID,
		Status:    "issued",
		Issuer:    issuer,
		NotBefore: leaf.NotBefore.Unix(),
		NotAfter:  leaf.NotAfter.Unix(),
		Domains:   string(sans),
		Signature: leaf.SignatureAlgorithm.String(),
		KeyType:   leaf.PublicKeyAlgorithm.String(),
		RenewedAt: time.Now().Unix(),
	}

	if err := m.store.UpsertCertificate(ctx, cert); err != nil {
		return nil, err
	}

	site, err := m.store.GetSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	site.SSLMode = "custom"
	if err := m.store.UpdateSite(ctx, site); err != nil {
		return nil, err
	}
	if err := m.Apply(ctx, site, ""); err != nil {
		return nil, fmt.Errorf("gagal menerapkan vhost SSL: %w", err)
	}

	return cert, nil
}

// LogDir mengembalikan direktori log Nginx yang digunakan oleh manajer situs
func (m *Manager) LogDir() string {
	return m.paths.LogDir
}

// VhostDir mengembalikan direktori vhost Nginx yang digunakan oleh manajer situs
func (m *Manager) VhostDir() string {
	return m.paths.VhostDir
}

// DiscoverAllDomainLogPaths memindai seluruh vhost Nginx (sites-enabled & conf.d) untuk mengumpulkan semua jalur access_log dan error_log domain
func DiscoverAllDomainLogPaths(domain string, extraDirs ...string) (accessPaths, errorPaths []string) {
	target := strings.ToLower(strings.TrimSpace(domain))
	if target == "" {
		return nil, nil
	}
	var dirs []string
	seenDir := map[string]bool{}
	addDir := func(d string) {
		d = strings.TrimSpace(d)
		if d == "" || seenDir[d] {
			return
		}
		seenDir[d] = true
		dirs = append(dirs, d)
	}
	// Prioritaskan vhost host asli di sites-enabled terlebih dahulu lalu conf.d dan direktori tambahan
	if runtime.GOOS == "linux" {
		addDir("/etc/nginx/sites-enabled")
		addDir("/etc/nginx/conf.d")
	}
	for _, d := range extraDirs {
		addDir(d)
	}

	seenAccess := map[string]bool{}
	seenError := map[string]bool{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fullPath := filepath.Join(dir, entry.Name())
			raw, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			aPaths, ePaths := extractAllLogPathsFromVhostConfig(string(raw), target)
			for _, p := range aPaths {
				clean := filepath.Clean(p)
				if !seenAccess[clean] {
					seenAccess[clean] = true
					accessPaths = append(accessPaths, p)
				}
			}
			for _, p := range ePaths {
				clean := filepath.Clean(p)
				if !seenError[clean] {
					seenError[clean] = true
					errorPaths = append(errorPaths, p)
				}
			}
		}
	}
	return accessPaths, errorPaths
}

// DiscoverDomainLogPaths mengembalikan jalur access_log dan error_log pertama yang ditemukan untuk domain
func DiscoverDomainLogPaths(domain string, extraDirs ...string) (accessPath, errorPath string) {
	aPaths, ePaths := DiscoverAllDomainLogPaths(domain, extraDirs...)
	if len(aPaths) > 0 {
		accessPath = aPaths[0]
	}
	if len(ePaths) > 0 {
		errorPath = ePaths[0]
	}
	return accessPath, errorPath
}

// extractAllLogPathsFromVhostConfig membedah seluruh blok server{} Nginx dan mengumpulkan semua access_log/error_log bila server_name cocok
func extractAllLogPathsFromVhostConfig(content, targetDomain string) (accessPaths, errorPaths []string) {
	inServer := false
	braceDepth := 0
	serverMatched := false
	var blockAccess, blockError []string

	flushBlock := func() {
		if serverMatched {
			accessPaths = append(accessPaths, blockAccess...)
			errorPaths = append(errorPaths, blockError...)
		}
		inServer = false
		braceDepth = 0
		serverMatched = false
		blockAccess = nil
		blockError = nil
	}

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		if !inServer {
			if strings.HasPrefix(line, "server") && strings.Contains(line, "{") {
				rest := strings.TrimSpace(strings.TrimPrefix(line, "server"))
				if strings.HasPrefix(rest, "{") {
					inServer = true
					braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
					if braceDepth <= 0 {
						flushBlock()
					}
				}
			}
			continue
		}

		// Periksa direktif server_name, access_log, dan error_log di dalam blok server
		fields := strings.Fields(strings.TrimRight(line, ";"))
		if len(fields) >= 2 {
			switch strings.ToLower(fields[0]) {
			case "server_name":
				for _, tok := range fields[1:] {
					cleanTok := strings.ToLower(strings.Trim(tok, `;"'`))
					if cleanTok == targetDomain {
						serverMatched = true
						break
					}
				}
			case "access_log":
				cand := strings.Trim(fields[1], `;"'`)
				if cand != "" && !strings.EqualFold(cand, "off") && !strings.HasPrefix(cand, "syslog:") {
					blockAccess = append(blockAccess, cand)
				}
			case "error_log":
				cand := strings.Trim(fields[1], `;"'`)
				if cand != "" && !strings.EqualFold(cand, "off") && !strings.HasPrefix(cand, "syslog:") {
					blockError = append(blockError, cand)
				}
			}
		}

		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
		if braceDepth <= 0 {
			flushBlock()
		}
	}
	if inServer {
		flushBlock()
	}
	return accessPaths, errorPaths
}

// readBoundedLogLines membaca ekor berkas log dengan batas byte ketat untuk melindungi memori server kecil
func readBoundedLogLines(path string, maxBytes int64, filterDomain string, maxLines int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil || st.IsDir() || st.Size() == 0 {
		return nil
	}

	skipFirst := false
	if maxBytes > 0 && st.Size() > maxBytes {
		if _, err := f.Seek(st.Size()-maxBytes, 0); err != nil {
			return nil
		}
		skipFirst = true
	}

	buf := make([]byte, min(st.Size(), maxBytes))
	if maxBytes <= 0 {
		buf = make([]byte, st.Size())
	}
	n, _ := f.Read(buf)
	if n <= 0 {
		return nil
	}
	text := string(buf[:n])
	if skipFirst {
		if nl := strings.IndexByte(text, '\n'); nl >= 0 {
			text = text[nl+1:]
		}
	}

	lowerFilter := strings.ToLower(strings.TrimSpace(filterDomain))
	var out []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if lowerFilter != "" && !strings.Contains(strings.ToLower(trimmed), lowerFilter) {
			continue
		}
		out = append(out, trimmed)
	}
	if maxLines > 0 && len(out) > maxLines {
		out = out[len(out)-maxLines:]
	}
	return out
}

// GetLogWithSource membaca baris terakhir log akses/error lintas seluruh vhost domain beserta jalur berkas log aktif yang terbaca
func (m *Manager) GetLogWithSource(domain, logType string, maxLines int) (content, sourcePath string, err error) {
	if logType != "access" && logType != "error" {
		logType = "access"
	}
	if maxLines <= 0 {
		maxLines = 200
	}
	const maxReadBytes = 2 * 1024 * 1024

	// Kumpulkan seluruh kandidat berkas log khusus domain (vhost FikPanel + seluruh vhost host sites-enabled/conf.d)
	primaryPath := filepath.Join(m.paths.LogDir, fmt.Sprintf("%s.%s.log", domain, logType))
	var dedicatedPaths []string
	seen := map[string]bool{}
	addDedicated := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		base := strings.ToLower(filepath.Base(p))
		if base == "access.log" || base == "error.log" {
			return
		}
		clean := filepath.Clean(p)
		if seen[clean] {
			return
		}
		seen[clean] = true
		dedicatedPaths = append(dedicatedPaths, p)
	}
	addDedicated(primaryPath)
	aPaths, ePaths := DiscoverAllDomainLogPaths(domain, m.paths.VhostDir)
	if logType == "error" {
		for _, p := range ePaths {
			addDedicated(p)
		}
	} else {
		for _, p := range aPaths {
			addDedicated(p)
		}
	}

	var allLines []string
	var usedPaths []string
	for _, p := range dedicatedPaths {
		var lines []string
		lines = append(lines, readBoundedLogLines(p+".1", maxReadBytes, "", maxLines)...)
		lines = append(lines, readBoundedLogLines(p, maxReadBytes, "", maxLines)...)
		if len(lines) > 0 {
			allLines = append(allLines, lines...)
			usedPaths = append(usedPaths, filepath.ToSlash(p))
		}
	}

	// Fallback ke log global Nginx (<LogDir>/access.log atau error.log) dengan penyaringan domain
	if len(allLines) == 0 {
		globalPath := filepath.Join(m.paths.LogDir, logType+".log")
		gLines := readBoundedLogLines(globalPath, 4*1024*1024, domain, maxLines)
		if len(gLines) > 0 {
			allLines = append(allLines, gLines...)
			usedPaths = append(usedPaths, filepath.ToSlash(globalPath))
		}
	}

	if len(allLines) == 0 {
		// Tampilkan jalur berkas log yang dipantau meski masih kosong agar transparan
		if len(dedicatedPaths) > 0 {
			sourcePath = filepath.ToSlash(dedicatedPaths[0])
		}
		return fmt.Sprintf("Belum ada catatan %s log untuk %s.", logType, domain), sourcePath, nil
	}
	if len(allLines) > maxLines {
		allLines = allLines[len(allLines)-maxLines:]
	}
	return strings.Join(allLines, "\n"), strings.Join(usedPaths, ", "), nil
}

// GetLog membaca baris terakhir log akses atau error untuk domain (mendukung seluruh vhost host & fallback global)
func (m *Manager) GetLog(domain, logType string, maxLines int) (string, error) {
	content, _, err := m.GetLogWithSource(domain, logType, maxLines)
	return content, err
}

// ClearLog mengosongkan log akses dan error untuk domain
func (m *Manager) ClearLog(domain string) error {
	for _, t := range []string{"access", "error"} {
		path := filepath.Join(m.paths.LogDir, fmt.Sprintf("%s.%s.log", domain, t))
		_ = os.WriteFile(path, []byte{}, 0o644)
	}
	return nil
}

// SaveRawVhost menguji sintaks Nginx lalu menyimpan konfigurasi vhost mentah domain
func (m *Manager) SaveRawVhost(ctx context.Context, site *store.Site, content string) error {
	m.applyMu.Lock()
	defer m.applyMu.Unlock()

	if err := nginx.CheckSyntax(content); err != nil {
		return err
	}

	active, _ := m.vhostFiles(site.Domain)
	snap := snapshot(active)

	if err := os.WriteFile(active, []byte(content), 0o644); err != nil {
		snap.restore()
		return fmt.Errorf("gagal menulis file vhost: %w", err)
	}

	if err := m.nginx.ApplyChange(ctx); err != nil {
		snap.restore()
		return fmt.Errorf("konfigurasi vhost gagal dimuat: %w", err)
	}
	return nil
}
