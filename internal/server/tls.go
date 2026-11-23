package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// selfSignedOrg menandai sertifikat yang dibuat & dikelola FikPanel (boleh diperbarui otomatis).
	selfSignedOrg = "FikPanel Self-Signed"
	// selfSignedValidity dijaga < 398 hari agar diterima browser/OS yang membatasi masa berlaku sertifikat.
	selfSignedValidity = 397 * 24 * time.Hour
	selfSignedRenewAt  = 30 * 24 * time.Hour
	certCheckInterval  = 30 * time.Second
	firstByteTimeout   = 30 * time.Second
	tlsRecordHandshake = 0x16
)

var dnsNamePattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*$`)

// certManager menyajikan sertifikat TLS panel dari <dir>/panel.crt + panel.key.
// Jika belum ada, sertifikat self-signed dibuat otomatis. Berkas yang diganti admin
// (misalnya sertifikat Let's Encrypt) dimuat ulang tanpa restart.
type certManager struct {
	certPath string
	keyPath  string
	logf     func(format string, args ...any)

	mu         sync.Mutex
	cert       *tls.Certificate
	selfSigned bool
	notAfter   time.Time
	modTime    time.Time
	lastCheck  time.Time
	persistent bool
}

func newCertManager(dir string, logf func(string, ...any)) (*certManager, error) {
	m := &certManager{
		certPath:   filepath.Join(dir, "panel.crt"),
		keyPath:    filepath.Join(dir, "panel.key"),
		logf:       logf,
		persistent: true,
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		m.logf("Peringatan: direktori sertifikat %s tidak dapat dibuat (%v); memakai sertifikat sementara di memori.", dir, err)
		m.persistent = false
		return m, m.useEphemeral()
	}

	_, certErr := os.Stat(m.certPath)
	_, keyErr := os.Stat(m.keyPath)
	if os.IsNotExist(certErr) && os.IsNotExist(keyErr) {
		if err := m.generateAndStore(); err != nil {
			m.logf("Peringatan: gagal menyimpan sertifikat self-signed (%v); memakai sertifikat sementara di memori.", err)
			m.persistent = false
			return m, m.useEphemeral()
		}
	}
	if err := m.load(); err != nil {
		// Jangan menimpa sertifikat milik admin yang rusak; tetap layani panel dengan sertifikat sementara.
		m.logf("Peringatan: sertifikat %s tidak dapat dimuat (%v); memakai sertifikat sementara di memori.", m.certPath, err)
		return m, m.useEphemeral()
	}
	m.renewIfNeeded()
	return m, nil
}

// TLSConfig mengembalikan konfigurasi TLS panel (TLS 1.2+, HTTP/1.1 agar SSE tetap stabil).
func (m *certManager) TLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:     tls.VersionTLS12,
		NextProtos:     []string{"http/1.1"},
		GetCertificate: m.getCertificate,
	}
}

// Fingerprint mengembalikan SHA-256 sertifikat aktif (untuk verifikasi manual oleh admin).
func (m *certManager) Fingerprint() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cert == nil || len(m.cert.Certificate) == 0 {
		return ""
	}
	sum := sha256.Sum256(m.cert.Certificate[0])
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = strings.ToUpper(hex.EncodeToString([]byte{b}))
	}
	return strings.Join(parts, ":")
}

// Describe menjelaskan sumber sertifikat aktif untuk log startup.
func (m *certManager) Describe() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	kind := "sertifikat kustom"
	if m.selfSigned {
		kind = "self-signed"
	}
	if !m.persistent || m.modTime.IsZero() {
		return kind + " sementara (memori)"
	}
	return fmt.Sprintf("%s, %s, berlaku hingga %s", kind, m.certPath, m.notAfter.Format("2006-01-02"))
}

func (m *certManager) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.persistent && time.Since(m.lastCheck) > certCheckInterval {
		m.lastCheck = time.Now()
		m.reloadIfChangedLocked()
		m.renewIfNeededLocked()
	}
	if m.cert == nil {
		return nil, errors.New("sertifikat TLS panel belum tersedia")
	}
	return m.cert, nil
}

func (m *certManager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadLocked()
}

func (m *certManager) loadLocked() error {
	pair, err := tls.LoadX509KeyPair(m.certPath, m.keyPath)
	if err != nil {
		return err
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return err
	}
	pair.Leaf = leaf
	m.cert = &pair
	m.notAfter = leaf.NotAfter
	m.selfSigned = isManagedSelfSigned(leaf)
	m.modTime = latestModTime(m.certPath, m.keyPath)
	return nil
}

func (m *certManager) reloadIfChangedLocked() {
	mt := latestModTime(m.certPath, m.keyPath)
	if mt.IsZero() || mt.Equal(m.modTime) {
		return
	}
	if err := m.loadLocked(); err != nil {
		// Admin mungkin sedang mengganti berkas; coba lagi pada pemeriksaan berikutnya.
		m.logf("Peringatan: sertifikat panel baru belum valid (%v); tetap memakai sertifikat sebelumnya.", err)
		return
	}
	m.logf("Sertifikat TLS panel dimuat ulang dari %s.", m.certPath)
}

func (m *certManager) renewIfNeeded() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.renewIfNeededLocked()
}

func (m *certManager) renewIfNeededLocked() {
	if !m.selfSigned || time.Until(m.notAfter) > selfSignedRenewAt {
		return
	}
	if !m.persistent {
		_ = m.useEphemeralLocked()
		return
	}
	if err := m.generateAndStore(); err != nil {
		m.logf("Peringatan: gagal memperbarui sertifikat self-signed: %v", err)
		return
	}
	if err := m.loadLocked(); err != nil {
		m.logf("Peringatan: gagal memuat sertifikat self-signed baru: %v", err)
		return
	}
	m.logf("Sertifikat self-signed panel diperbarui (berlaku hingga %s).", m.notAfter.Format("2006-01-02"))
}

func (m *certManager) useEphemeral() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.useEphemeralLocked()
}

func (m *certManager) useEphemeralLocked() error {
	certPEM, keyPEM, err := generateSelfSigned()
	if err != nil {
		return err
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}
	leaf, _ := x509.ParseCertificate(pair.Certificate[0])
	pair.Leaf = leaf
	m.cert = &pair
	m.selfSigned = true
	m.notAfter = leaf.NotAfter
	m.persistent = false
	return nil
}

func (m *certManager) generateAndStore() error {
	certPEM, keyPEM, err := generateSelfSigned()
	if err != nil {
		return err
	}
	// Kunci ditulis lebih dulu; reload hanya terpicu oleh perubahan berkas dan memvalidasi pasangan.
	if err := writeAtomic(m.keyPath, keyPEM, 0o600); err != nil {
		return err
	}
	return writeAtomic(m.certPath, certPEM, 0o644)
}

func isManagedSelfSigned(leaf *x509.Certificate) bool {
	for _, o := range leaf.Subject.Organization {
		if o == selfSignedOrg {
			return true
		}
	}
	return false
}

func latestModTime(paths ...string) time.Time {
	var latest time.Time
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil && fi.ModTime().After(latest) {
			latest = fi.ModTime()
		}
	}
	return latest
}

// generateSelfSigned membuat pasangan sertifikat ECDSA P-256 untuk hostname, localhost, dan seluruh IP host.
func generateSelfSigned() (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return nil, nil, err
	}

	cn := "fikpanel"
	dnsNames := []string{"localhost"}
	if host, err := os.Hostname(); err == nil && dnsNamePattern.MatchString(host) && host != "localhost" {
		cn = host
		dnsNames = append(dnsNames, host)
	}
	ips := []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok && ipNet.IP.IsGlobalUnicast() {
				ips = append(ips, ipNet.IP)
			}
		}
	}

	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn, Organization: []string{selfSignedOrg}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(selfSignedValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, writeErr := tmp.Write(data)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Chmod(name, perm); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

// sniffListener memisahkan koneksi pada satu port berdasarkan byte pertama:
//   - TLS ClientHello (0x16)           -> dibungkus tls.Server, dilayani server panel
//   - HTTP biasa dari loopback          -> dilayani server panel (reverse proxy lokal, health check updater)
//   - HTTP biasa dari alamat lain       -> dilayani server redirect ke https:// (tidak pernah menyentuh handler panel)
type sniffListener struct {
	inner     net.Listener
	tlsConfig *tls.Config
	main      chan net.Conn
	redirect  *chanListener
	done      chan struct{}
	closeOnce sync.Once
}

func newSniffListener(inner net.Listener, cfg *tls.Config) *sniffListener {
	l := &sniffListener{
		inner:     inner,
		tlsConfig: cfg,
		main:      make(chan net.Conn),
		done:      make(chan struct{}),
	}
	l.redirect = &chanListener{parent: l, ch: make(chan net.Conn), done: make(chan struct{})}
	go l.acceptLoop()
	return l
}

func (l *sniffListener) acceptLoop() {
	backoff := 5 * time.Millisecond
	for {
		c, err := l.inner.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				l.Close()
				return
			}
			// Galat sementara (mis. EMFILE): tunggu sebentar lalu lanjut, seperti http.Server.
			select {
			case <-l.done:
				return
			case <-time.After(backoff):
			}
			if backoff < time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = 5 * time.Millisecond
		go l.classify(c)
	}
}

func (l *sniffListener) classify(c net.Conn) {
	_ = c.SetReadDeadline(time.Now().Add(firstByteTimeout))
	var first [1]byte
	if _, err := io.ReadFull(c, first[:]); err != nil {
		_ = c.Close()
		return
	}
	_ = c.SetReadDeadline(time.Time{})
	pc := &prefixConn{Conn: c, prefix: []byte{first[0]}}

	switch {
	case first[0] == tlsRecordHandshake:
		l.deliver(l.main, l.done, tls.Server(pc, l.tlsConfig))
	case isLoopbackAddr(c.RemoteAddr()):
		l.deliver(l.main, l.done, pc)
	default:
		l.deliver(l.redirect.ch, l.redirect.done, pc)
	}
}

func (l *sniffListener) deliver(ch chan net.Conn, closed chan struct{}, c net.Conn) {
	select {
	case ch <- c:
	case <-closed:
		_ = c.Close()
	case <-l.done:
		_ = c.Close()
	}
}

func (l *sniffListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.main:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *sniffListener) Close() error {
	var err error
	l.closeOnce.Do(func() {
		close(l.done)
		err = l.inner.Close()
	})
	return err
}

func (l *sniffListener) Addr() net.Addr { return l.inner.Addr() }

// chanListener adalah listener virtual untuk koneksi HTTP biasa yang akan di-redirect.
type chanListener struct {
	parent    *sniffListener
	ch        chan net.Conn
	done      chan struct{}
	closeOnce sync.Once
}

func (c *chanListener) Accept() (net.Conn, error) {
	select {
	case conn := <-c.ch:
		return conn, nil
	case <-c.done:
		return nil, net.ErrClosed
	case <-c.parent.done:
		return nil, net.ErrClosed
	}
}

func (c *chanListener) Close() error {
	c.closeOnce.Do(func() { close(c.done) })
	return nil
}

func (c *chanListener) Addr() net.Addr { return c.parent.Addr() }

// prefixConn mengembalikan byte yang sudah diintip sebelum melanjutkan pembacaan dari koneksi asli.
type prefixConn struct {
	net.Conn
	prefix []byte
}

func (p *prefixConn) Read(b []byte) (int, error) {
	if len(p.prefix) > 0 {
		n := copy(b, p.prefix)
		p.prefix = p.prefix[n:]
		return n, nil
	}
	return p.Conn.Read(b)
}

func isLoopbackAddr(a net.Addr) bool {
	tcp, ok := a.(*net.TCPAddr)
	return ok && tcp.IP.IsLoopback()
}

// redirectToHTTPS mengarahkan permintaan HTTP biasa ke skema https pada host dan port yang sama.
func redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		if addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
			host = addr.String()
		}
	}
	w.Header().Set("Connection", "close")
	http.Redirect(w, r, "https://"+host+r.URL.RequestURI(), http.StatusPermanentRedirect)
}
