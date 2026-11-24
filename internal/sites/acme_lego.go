package sites

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fikpanel/internal/store"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	legocf "github.com/go-acme/lego/v4/providers/dns/cloudflare"
	legodo "github.com/go-acme/lego/v4/providers/dns/digitalocean"
	legoduck "github.com/go-acme/lego/v4/providers/dns/duckdns"
	legonc "github.com/go-acme/lego/v4/providers/dns/namecheap"
	legopb "github.com/go-acme/lego/v4/providers/dns/porkbun"
	"github.com/go-acme/lego/v4/registration"
)

// IssueOptions menyimpan opsi penerbitan SSL termasuk tantangan DNS-01 Wildcard berbasis go-acme/lego
type IssueOptions struct {
	SkipDNS         bool
	ChallengeType   string            // "http01" (default) atau "dns01"
	DNSProvider     string            // "cloudflare", "digitalocean", "porkbun", "namecheap", "duckdns"
	IncludeWildcard bool              // otomatis sertakan "*.<domain>" saat dns01 aktif
	Credentials     map[string]string // opsional: kredensial baru untuk disimpan sekaligus dipakai
}

// DNSProviderField mendeskripsikan satu input kredensial DNS provider untuk antarmuka panel
type DNSProviderField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	Secret      bool   `json:"secret"`
	Required    bool   `json:"required"`
}

// DNSProviderMeta mendeskripsikan provider DNS-01 ringan yang didukung go-acme/lego di FikPanel
type DNSProviderMeta struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Configured  bool               `json:"configured"`
	Source      string             `json:"source,omitempty"` // "saved" | "cloudflare_hub"
	Fields      []DNSProviderField `json:"fields"`
}

// SupportedDNSProviders mengembalikan katalog 5 provider DNS-01 ringan (tanpa SDK cloud raksasa)
func SupportedDNSProviders(configured map[string]string) []DNSProviderMeta {
	items := []DNSProviderMeta{
		{
			ID:          "cloudflare",
			Name:        "Cloudflare DNS",
			Description: "Otomatis memakai token Cloudflare Hub bila sudah terhubung, atau isi API Token (Zone:DNS:Edit).",
			Fields: []DNSProviderField{
				{Key: "api_token", Label: "Cloudflare API Token (Zone:DNS:Edit)", Placeholder: "Kosongkan jika sudah terhubung di Cloudflare Hub", Secret: true, Required: false},
				{Key: "email", Label: "Email Akun (Opsional — khusus Global API Key)", Placeholder: "admin@domain.com", Secret: false, Required: false},
				{Key: "api_key", Label: "Global API Key (Opsional)", Placeholder: "Kosongkan jika memakai API Token", Secret: true, Required: false},
			},
		},
		{
			ID:          "digitalocean",
			Name:        "DigitalOcean DNS",
			Description: "Personal Access Token DigitalOcean dengan izin tulis DNS.",
			Fields: []DNSProviderField{
				{Key: "auth_token", Label: "DigitalOcean Auth Token", Placeholder: "dop_v1_...", Secret: true, Required: true},
			},
		},
		{
			ID:          "porkbun",
			Name:        "Porkbun DNS",
			Description: "API Key dan Secret API Key dari panel API Access Porkbun.",
			Fields: []DNSProviderField{
				{Key: "api_key", Label: "Porkbun API Key", Placeholder: "pk1_...", Secret: true, Required: true},
				{Key: "secret_api_key", Label: "Porkbun Secret API Key", Placeholder: "sk1_...", Secret: true, Required: true},
			},
		},
		{
			ID:          "namecheap",
			Name:        "Namecheap DNS",
			Description: "API User, API Key, dan IP publik yang terdaftar di whitelist API Namecheap.",
			Fields: []DNSProviderField{
				{Key: "api_user", Label: "Namecheap API User", Placeholder: "username", Secret: false, Required: true},
				{Key: "api_key", Label: "Namecheap API Key", Placeholder: "api-key", Secret: true, Required: true},
				{Key: "client_ip", Label: "Client IP (Opsional)", Placeholder: "Auto-detect bila kosong", Secret: false, Required: false},
			},
		},
		{
			ID:          "duckdns",
			Name:        "DuckDNS",
			Description: "Token DuckDNS untuk subdomain *.duckdns.org tanpa membuka port 80.",
			Fields: []DNSProviderField{
				{Key: "token", Label: "DuckDNS Token", Placeholder: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", Secret: true, Required: true},
			},
		},
	}
	for i := range items {
		if src, ok := configured[items[i].ID]; ok {
			items[i].Configured = true
			items[i].Source = src
		}
	}
	return items
}

// legoUser mengimplementasikan antarmuka registration.User milik go-acme/lego
type legoUser struct {
	email        string
	registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *legoUser) GetEmail() string                        { return u.email }
func (u *legoUser) GetRegistration() *registration.Resource { return u.registration }
func (u *legoUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

// buildLegoDNSProvider membangun challenge.Provider ringan dari 5 DNS provider pilihan
func buildLegoDNSProvider(provider string, creds map[string]string) (challenge.Provider, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "cloudflare":
		cfg := legocf.NewDefaultConfig()
		cfg.PropagationTimeout = 2 * time.Minute
		cfg.PollingInterval = 4 * time.Second
		if tok := strings.TrimSpace(creds["api_token"]); tok != "" {
			cfg.AuthToken = tok
		} else if key := strings.TrimSpace(creds["api_key"]); key != "" && strings.TrimSpace(creds["email"]) != "" {
			cfg.AuthEmail = strings.TrimSpace(creds["email"])
			cfg.AuthKey = key
		} else {
			return nil, fmt.Errorf("kredensial Cloudflare belum diisi (hubungkan Cloudflare Hub atau masukkan API Token Zone:DNS:Edit)")
		}
		return legocf.NewDNSProviderConfig(cfg)

	case "digitalocean":
		tok := strings.TrimSpace(creds["auth_token"])
		if tok == "" {
			return nil, fmt.Errorf("DigitalOcean auth_token wajib diisi")
		}
		cfg := legodo.NewDefaultConfig()
		cfg.AuthToken = tok
		cfg.PropagationTimeout = 2 * time.Minute
		return legodo.NewDNSProviderConfig(cfg)

	case "porkbun":
		ak := strings.TrimSpace(creds["api_key"])
		sk := strings.TrimSpace(creds["secret_api_key"])
		if ak == "" || sk == "" {
			return nil, fmt.Errorf("Porkbun api_key dan secret_api_key wajib diisi")
		}
		cfg := legopb.NewDefaultConfig()
		cfg.APIKey = ak
		cfg.SecretAPIKey = sk
		cfg.PropagationTimeout = 2 * time.Minute
		return legopb.NewDNSProviderConfig(cfg)

	case "namecheap":
		u := strings.TrimSpace(creds["api_user"])
		k := strings.TrimSpace(creds["api_key"])
		if u == "" || k == "" {
			return nil, fmt.Errorf("Namecheap api_user dan api_key wajib diisi")
		}
		cfg := legonc.NewDefaultConfig()
		cfg.APIUser = u
		cfg.APIKey = k
		if ip := strings.TrimSpace(creds["client_ip"]); ip != "" {
			cfg.ClientIP = ip
		}
		cfg.PropagationTimeout = 3 * time.Minute
		return legonc.NewDNSProviderConfig(cfg)

	case "duckdns":
		tok := strings.TrimSpace(creds["token"])
		if tok == "" {
			return nil, fmt.Errorf("DuckDNS token wajib diisi")
		}
		cfg := legoduck.NewDefaultConfig()
		cfg.Token = tok
		cfg.PropagationTimeout = 2 * time.Minute
		return legoduck.NewDNSProviderConfig(cfg)

	default:
		return nil, fmt.Errorf("provider DNS-01 %q tidak didukung (gunakan: cloudflare, digitalocean, porkbun, namecheap, duckdns)", provider)
	}
}

// expandDomainsWithWildcard menambahkan entri wildcard "*.<base>" secara rapi tanpa duplikasi
func expandDomainsWithWildcard(domains []string, includeWildcard bool) []string {
	seen := make(map[string]bool, len(domains)+1)
	out := make([]string, 0, len(domains)+1)
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	if includeWildcard && len(out) > 0 {
		base := strings.TrimPrefix(out[0], "*.")
		wc := "*." + base
		if !seen[wc] {
			out = append(out, wc)
		}
	}
	return out
}

func hasAnyCredValue(creds map[string]string) bool {
	for _, v := range creds {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// resolveDNSCredentials menggabungkan kredensial input dengan yang tersimpan di SQLite / Cloudflare Hub
func (m *Manager) resolveDNSCredentials(ctx context.Context, provider string, override map[string]string) (map[string]string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if hasAnyCredValue(override) {
		_ = m.store.SaveACMEDNSProvider(ctx, provider, override)
		return override, nil
	}
	saved, _, err := m.store.GetACMEDNSProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// ObtainCertBundleDNS01 menerbitkan sertifikat ACME (termasuk Wildcard) via go-acme/lego DNS-01
// dan mengembalikan pasangan PEM fullchain + privkey (digunakan oleh SSL Situs maupun Auto-SSL Panel)
func (m *Manager) ObtainCertBundleDNS01(
	ctx context.Context,
	domains []string,
	providerName string,
	overrideCreds map[string]string,
	onStep func(string),
) (certPEM []byte, keyPEM []byte, leaf *x509.Certificate, err error) {
	step := func(s string) {
		if onStep != nil {
			onStep(s)
		}
	}
	if len(domains) == 0 {
		return nil, nil, nil, &ValidationError{Msg: "daftar domain wajib diisi"}
	}
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	if providerName == "" {
		providerName = "cloudflare"
	}

	creds, err := m.resolveDNSCredentials(ctx, providerName, overrideCreds)
	if err != nil {
		return nil, nil, nil, err
	}
	dnsProvider, err := buildLegoDNSProvider(providerName, creds)
	if err != nil {
		return nil, nil, nil, &ValidationError{Msg: err.Error()}
	}

	step("Mendaftarkan akun ACME (lego)")
	acctKey, err := m.accountKey()
	if err != nil {
		return nil, nil, nil, err
	}

	email := strings.TrimSpace(creds["email"])
	if email == "" {
		email = "admin@" + strings.TrimPrefix(domains[0], "*.")
	}
	user := &legoUser{email: email, key: acctKey}
	cfg := lego.NewConfig(user)
	cfg.CADirURL = acmeDirectory()
	cfg.Certificate.KeyType = certcrypto.EC256

	client, err := lego.NewClient(cfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("gagal inisialisasi klien ACME lego: %w", err)
	}

	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("registrasi akun ACME gagal: %w", err)
	}
	user.registration = reg

	step("Validasi DNS-01 (" + providerName + ")")
	// Gunakan resolver publik Cloudflare & Google agar propagasi TXT cepat terdeteksi dan bebas cache ISP
	err = client.Challenge.SetDNS01Provider(dnsProvider,
		dns01.AddRecursiveNameservers([]string{"1.1.1.1:53", "8.8.8.8:53"}),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("gagal memasang DNS-01 provider %s: %w", providerName, err)
	}

	step("Menerbitkan sertifikat")
	res, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("penerbitan DNS-01 gagal: %w", err)
	}

	block, _ := pem.Decode(res.Certificate)
	if block == nil {
		return nil, nil, nil, fmt.Errorf("sertifikat PEM dari CA tidak valid")
	}
	parsedLeaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("gagal mem-parsing sertifikat X.509: %w", err)
	}
	return res.Certificate, res.PrivateKey, parsedLeaf, nil
}

// acmeIssueDNS01 menerbitkan sertifikat DNS-01 via lego dan menyimpannya ke direktori SSL situs
func (m *Manager) acmeIssueDNS01(ctx context.Context, site *store.Site, domains []string, opts IssueOptions) (*store.Certificate, error) {
	step := func(s string) { m.setJob(site.ID, func(j *SSLJob) { j.Step = s }) }

	// Gunakan ulang sertifikat lokal bila valid >= 30 hari dan mencakup seluruh domain (termasuk wildcard)
	if existing, ok := m.tryReuseExistingCert(site.ID, site.Domain, domains, 30); ok {
		existing.ChallengeType = "dns01"
		existing.DNSProvider = opts.DNSProvider
		step("Menerbitkan sertifikat")
		return existing, nil
	}

	certPEM, keyPEM, leaf, err := m.ObtainCertBundleDNS01(ctx, domains, opts.DNSProvider, opts.Credentials, step)
	if err != nil {
		return nil, err
	}

	certPath, keyPath := m.certFiles(site.Domain)
	if err := os.MkdirAll(filepath.Dir(certPath), 0o700); err != nil {
		return nil, fmt.Errorf("gagal membuat folder sertifikat: %w", err)
	}
	if err := atomicWrite(keyPath, keyPEM, 0o600); err != nil {
		return nil, err
	}
	if err := atomicWrite(certPath, certPEM, 0o644); err != nil {
		return nil, err
	}

	sans, _ := json.Marshal(leaf.DNSNames)
	return &store.Certificate{
		SiteID:        site.ID,
		Status:        "issued",
		Issuer:        leaf.Issuer.String(),
		NotBefore:     leaf.NotBefore.Unix(),
		NotAfter:      leaf.NotAfter.Unix(),
		Domains:       string(sans),
		Signature:     leaf.SignatureAlgorithm.String(),
		KeyType:       "ECDSA P-256",
		RenewedAt:     time.Now().Unix(),
		ChallengeType: "dns01",
		DNSProvider:   strings.ToLower(strings.TrimSpace(opts.DNSProvider)),
	}, nil
}
