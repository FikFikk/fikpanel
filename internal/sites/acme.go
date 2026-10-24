package sites

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/acme"

	"fikpanel/internal/store"
)

// SSLJob merepresentasikan progres penerbitan sertifikat yang berjalan di background
type SSLJob struct {
	Running    bool   `json:"running"`
	Step       string `json:"step"`
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
}

// ValidationError menandai input tidak valid (HTTP 400)
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// ConflictError menandai operasi bentrok (HTTP 409)
type ConflictError struct{ Msg string }

func (e *ConflictError) Error() string { return e.Msg }

// Job mengembalikan status job SSL terakhir untuk situs
func (m *Manager) Job(siteID int64) SSLJob {
	m.jobsMu.Lock()
	defer m.jobsMu.Unlock()
	return m.jobs[siteID]
}

func (m *Manager) setJob(siteID int64, fn func(j *SSLJob)) {
	m.jobsMu.Lock()
	defer m.jobsMu.Unlock()
	j := m.jobs[siteID]
	fn(&j)
	m.jobs[siteID] = j
}

// IssueSSL memvalidasi prasyarat lalu memulai job penerbitan sertifikat di background
func (m *Manager) IssueSSL(ctx context.Context, siteID int64, skipDNS bool, onDone func(ok bool, msg string)) error {
	site, err := m.store.GetSite(ctx, siteID)
	if err != nil {
		return err
	}
	if site.Enabled != 1 {
		return &ValidationError{Msg: "aktifkan situs terlebih dahulu sebelum menerbitkan SSL"}
	}
	domains := AllDomains(site)
	for _, d := range domains {
		if strings.HasPrefix(d, "*.") {
			return &ValidationError{Msg: "domain wildcard membutuhkan validasi DNS-01 yang belum didukung; hapus alias " + d + " dulu"}
		}
	}

	// Let's Encrypt HTTP-01 hanya berhasil bila semua domain sudah mengarah ke server ini
	if !skipDNS && !m.mock {
		results, _ := m.CheckDNS(ctx, domains)
		var bad []string
		for _, r := range results {
			if !r.PointsHere {
				bad = append(bad, r.Domain)
			}
		}
		if len(bad) > 0 {
			return &ValidationError{Msg: "domain belum mengarah ke IP server: " + strings.Join(bad, ", ")}
		}
	}

	m.jobsMu.Lock()
	if m.jobs[siteID].Running {
		m.jobsMu.Unlock()
		return &ConflictError{Msg: "penerbitan SSL untuk situs ini sedang berjalan"}
	}
	m.jobs[siteID] = SSLJob{Running: true, Step: "Memulai", StartedAt: time.Now().Unix()}
	m.jobsMu.Unlock()

	_ = m.store.UpsertCertificate(ctx, &store.Certificate{SiteID: siteID, Status: "pending", Domains: "[]"})

	// Job dilepas dari context request supaya tetap berjalan setelah respons HTTP dikirim
	go m.runIssue(site, domains, onDone)
	return nil
}

// runIssue mengeksekusi penerbitan, menyimpan metadata, dan menerapkan vhost HTTPS
func (m *Manager) runIssue(site *store.Site, domains []string, onDone func(ok bool, msg string)) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var cert *store.Certificate
	var err error
	if m.mock {
		cert, err = m.simulateIssue(ctx, site.ID, domains)
	} else {
		cert, err = m.acmeIssue(ctx, site, domains)
	}

	if err == nil {
		m.setJob(site.ID, func(j *SSLJob) { j.Step = "Menerapkan HTTPS ke Nginx" })
		if err = m.store.UpsertCertificate(ctx, cert); err == nil {
			var fresh *store.Site
			if fresh, err = m.store.GetSite(ctx, site.ID); err == nil {
				err = m.Apply(ctx, fresh, "")
			}
		}
	}

	msg := "Sertifikat SSL berhasil diterbitkan untuk " + strings.Join(domains, ", ")
	if err != nil {
		msg = "Penerbitan SSL gagal: " + err.Error()
		_ = m.store.UpsertCertificate(ctx, &store.Certificate{
			SiteID: site.ID, Status: "failed", Domains: "[]", LastError: err.Error(),
		})
	}
	m.setJob(site.ID, func(j *SSLJob) {
		j.Running = false
		j.OK = err == nil
		j.Message = msg
		j.Step = "Selesai"
		j.FinishedAt = time.Now().Unix()
	})
	if onDone != nil {
		onDone(err == nil, msg)
	}
}

// simulateIssue meniru alur ACME di lingkungan dev non-Linux tanpa menghubungi Let's Encrypt
func (m *Manager) simulateIssue(ctx context.Context, siteID int64, domains []string) (*store.Certificate, error) {
	for _, step := range []string{"Mendaftarkan akun ACME", "Validasi HTTP-01 domain", "Menerbitkan sertifikat"} {
		m.setJob(siteID, func(j *SSLJob) { j.Step = step })
		select {
		case <-time.After(1200 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	sans, _ := json.Marshal(domains)
	now := time.Now()
	return &store.Certificate{
		SiteID:    siteID,
		Status:    "issued",
		Issuer:    "C = US, O = Let's Encrypt, CN = E6 (simulasi)",
		NotBefore: now.Unix(),
		NotAfter:  now.Add(90 * 24 * time.Hour).Unix(),
		Domains:   string(sans),
		Signature: "ECDSA-SHA384",
		KeyType:   "ECDSA P-256",
		RenewedAt: now.Unix(),
	}, nil
}

// acmeDirectory memilih endpoint produksi atau staging (FIKPANEL_ACME_STAGING=1) Let's Encrypt
func acmeDirectory() string {
	if os.Getenv("FIKPANEL_ACME_STAGING") == "1" {
		return "https://acme-staging-v02.api.letsencrypt.org/directory"
	}
	return acme.LetsEncryptURL
}

// accountKey memuat atau membuat kunci akun ACME persisten
func (m *Manager) accountKey() (*ecdsa.PrivateKey, error) {
	if data, err := os.ReadFile(m.paths.AccountKey); err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("file kunci akun ACME rusak")
		}
		return x509.ParseECPrivateKey(block.Bytes)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat kunci akun ACME: %w", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(m.paths.AccountKey), 0o700); err != nil {
		return nil, fmt.Errorf("gagal membuat folder akun ACME: %w", err)
	}
	if err := os.WriteFile(m.paths.AccountKey, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, fmt.Errorf("gagal menyimpan kunci akun ACME: %w", err)
	}
	return key, nil
}

// acmeIssue menjalankan alur RFC 8555 dengan tantangan HTTP-01 via webroot
func (m *Manager) acmeIssue(ctx context.Context, site *store.Site, domains []string) (*store.Certificate, error) {
	step := func(s string) { m.setJob(site.ID, func(j *SSLJob) { j.Step = s }) }

	step("Mendaftarkan akun ACME")
	key, err := m.accountKey()
	if err != nil {
		return nil, err
	}
	client := &acme.Client{Key: key, DirectoryURL: acmeDirectory()}
	if _, err := client.Register(ctx, &acme.Account{}, acme.AcceptTOS); err != nil && !errors.Is(err, acme.ErrAccountAlreadyExists) {
		return nil, fmt.Errorf("registrasi akun ACME gagal: %w", err)
	}

	step("Membuat order sertifikat")
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(domains...))
	if err != nil {
		return nil, fmt.Errorf("gagal membuat order: %w", err)
	}

	var tokenFiles []string
	defer func() {
		for _, f := range tokenFiles {
			_ = os.Remove(f)
		}
	}()

	for _, authzURL := range order.AuthzURLs {
		authz, err := client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return nil, fmt.Errorf("gagal membaca otorisasi: %w", err)
		}
		if authz.Status == acme.StatusValid {
			continue
		}
		var chal *acme.Challenge
		for _, c := range authz.Challenges {
			if c.Type == "http-01" {
				chal = c
				break
			}
		}
		if chal == nil {
			return nil, fmt.Errorf("CA tidak menawarkan tantangan http-01 untuk %s", authz.Identifier.Value)
		}

		step("Validasi HTTP-01: " + authz.Identifier.Value)
		body, err := client.HTTP01ChallengeResponse(chal.Token)
		if err != nil {
			return nil, err
		}
		file := filepath.Join(m.paths.ACMEWebroot, filepath.FromSlash(client.HTTP01ChallengePath(chal.Token)))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return nil, fmt.Errorf("gagal membuat folder tantangan: %w", err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			return nil, fmt.Errorf("gagal menulis token tantangan: %w", err)
		}
		tokenFiles = append(tokenFiles, file)

		if _, err := client.Accept(ctx, chal); err != nil {
			return nil, fmt.Errorf("gagal menerima tantangan: %w", err)
		}
		if _, err := client.WaitAuthorization(ctx, authz.URI); err != nil {
			return nil, fmt.Errorf("validasi %s gagal (pastikan port 80 terbuka & DNS benar): %w", authz.Identifier.Value, err)
		}
	}

	step("Menerbitkan sertifikat")
	order, err = client.WaitOrder(ctx, order.URI)
	if err != nil {
		return nil, fmt.Errorf("order tidak siap: %w", err)
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{DNSNames: domains}, certKey)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat CSR: %w", err)
	}
	chain, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return nil, fmt.Errorf("gagal finalisasi sertifikat: %w", err)
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("CA mengembalikan rantai sertifikat kosong")
	}

	if err := m.writeCertFiles(site.Domain, chain, certKey); err != nil {
		return nil, err
	}

	leaf, err := x509.ParseCertificate(chain[0])
	if err != nil {
		return nil, fmt.Errorf("gagal mem-parsing sertifikat: %w", err)
	}
	sans, _ := json.Marshal(leaf.DNSNames)
	return &store.Certificate{
		SiteID:    site.ID,
		Status:    "issued",
		Issuer:    leaf.Issuer.String(),
		NotBefore: leaf.NotBefore.Unix(),
		NotAfter:  leaf.NotAfter.Unix(),
		Domains:   string(sans),
		Signature: leaf.SignatureAlgorithm.String(),
		KeyType:   "ECDSA P-256",
		RenewedAt: time.Now().Unix(),
	}, nil
}

// writeCertFiles menulis fullchain dan privkey secara atomik
func (m *Manager) writeCertFiles(domain string, chain [][]byte, key *ecdsa.PrivateKey) error {
	certPath, keyPath := m.certFiles(domain)
	if err := os.MkdirAll(filepath.Dir(certPath), 0o700); err != nil {
		return fmt.Errorf("gagal membuat folder sertifikat: %w", err)
	}
	var pemChain []byte
	for _, der := range chain {
		pemChain = append(pemChain, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	if err := atomicWrite(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return err
	}
	return atomicWrite(certPath, pemChain, 0o644)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return fmt.Errorf("gagal menulis %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("gagal mengganti %s: %w", path, err)
	}
	return nil
}

// RunRenewLoop memeriksa sertifikat setiap 12 jam dan memperpanjang yang sisa masa aktifnya < 30 hari
func (m *Manager) RunRenewLoop(ctx context.Context, audit func(ok bool, target, msg string)) {
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ids, err := m.store.ListExpiringCertificates(ctx, time.Now().Add(30*24*time.Hour).Unix())
			if err != nil {
				continue
			}
			for _, id := range ids {
				site, err := m.store.GetSite(ctx, id)
				if err != nil {
					continue
				}
				// skipDNS=false: renew tetap memvalidasi DNS agar error jelas di audit log
				err = m.IssueSSL(ctx, id, false, func(ok bool, msg string) { audit(ok, site.Domain, msg) })
				if err != nil {
					audit(false, site.Domain, "Auto-renew gagal dimulai: "+err.Error())
				}
			}
		}
	}
}
