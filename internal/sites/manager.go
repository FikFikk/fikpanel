package sites

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

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
}

// DefaultPaths mengembalikan lokasi standar Linux atau folder dev-data di non-Linux
func DefaultPaths() Paths {
	p := Paths{
		VhostDir:    "/etc/nginx/conf.d",
		LogDir:      "/var/log/nginx",
		ACMEWebroot: "/var/lib/fikpanel/acme-webroot",
		CertDir:     "/etc/fikpanel/ssl",
		AccountKey:  "/var/lib/fikpanel/acme/account.key",
	}
	if runtime.GOOS != "linux" {
		base := filepath.Join("dev-data", "nginx")
		p = Paths{
			VhostDir:    filepath.Join(base, "conf.d"),
			LogDir:      filepath.Join(base, "logs"),
			ACMEWebroot: filepath.Join(base, "acme-webroot"),
			CertDir:     filepath.Join(base, "ssl"),
			AccountKey:  filepath.Join(base, "acme", "account.key"),
		}
	}
	if v := os.Getenv("FIKPANEL_VHOST_DIR"); v != "" {
		p.VhostDir = v
	}
	if v := os.Getenv("FIKPANEL_CERT_DIR"); v != "" {
		p.CertDir = v
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

// defaultIndex adalah halaman sambutan untuk situs static/php baru
const defaultIndex = `<!doctype html>
<html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%[1]s</title>
<style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#0b0e15;color:#e2e8f0;font-family:system-ui,sans-serif}
main{text-align:center}h1{font-size:2rem;margin:0 0 .5rem}p{color:#8b95a5}b{color:#10b981}</style></head>
<body><main><h1>%[1]s</h1><p>Situs berhasil dibuat dengan <b>FikPanel</b>. Unggah berkas Anda ke folder ini.</p></main></body></html>
`

// EnsureRoot membuat direktori web root dan index.html sambutan bila folder masih kosong
func (m *Manager) EnsureRoot(site *store.Site) error {
	if m.mock || site.Kind == "proxy" || site.RootPath == "" {
		return nil
	}
	if err := os.MkdirAll(site.RootPath, 0o755); err != nil {
		return fmt.Errorf("gagal membuat direktori situs: %w", err)
	}
	entries, err := os.ReadDir(site.RootPath)
	if err != nil {
		return fmt.Errorf("gagal membaca direktori situs: %w", err)
	}
	if len(entries) > 0 {
		return nil
	}
	index := filepath.Join(site.RootPath, "index.html")
	if err := os.WriteFile(index, []byte(fmt.Sprintf(defaultIndex, site.Domain)), 0o644); err != nil {
		return fmt.Errorf("gagal menulis index.html: %w", err)
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

// DefaultRoot menghasilkan direktori web root standar untuk domain
func DefaultRoot(domain string) string {
	return "/var/www/" + strings.TrimPrefix(domain, "*.") + "/public_html"
}
