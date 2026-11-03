package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fikpanel/internal/cloudflare"
	"fikpanel/internal/discovery"
	"fikpanel/internal/sites"
	"fikpanel/internal/store"
)

// AdoptSiteItem merepresentasikan input situs hasil pemindaian yang akan diadopsi ke database
type AdoptSiteItem struct {
	PrimaryDomain string   `json:"primary_domain"`
	Aliases       []string `json:"aliases"`
	DocumentRoot  string   `json:"document_root"`
	HasPHP        bool     `json:"has_php"`
	ProxyPass     string   `json:"proxy_pass"`
	HasSSL        bool     `json:"has_ssl"`
	SSLCertPath   string   `json:"ssl_cert_path,omitempty"`
	SSLKeyPath    string   `json:"ssl_key_path,omitempty"`
	SSLIssuer     string   `json:"ssl_issuer,omitempty"`
	SSLNotAfter   int64    `json:"ssl_not_after,omitempty"`
}

// AdoptPayload merepresentasikan daftar resource yang dipilih pengguna untuk didaftarkan ke FikPanel
type AdoptPayload struct {
	Sites []AdoptSiteItem `json:"sites"`
}

// handleGetDiscoveryConfig mengambil daftar direktori kustom yang dipindai
func (s *Server) handleGetDiscoveryConfig(w http.ResponseWriter, r *http.Request) {
	val, err := s.store.GetSetting(r.Context(), "discovery_custom_roots")
	if err != nil {
		http.Error(w, `{"error":"gagal membaca konfigurasi direktori pencarian"}`, http.StatusInternalServerError)
		return
	}

	var roots []string
	if val != "" {
		_ = json.Unmarshal([]byte(val), &roots)
	}
	if roots == nil {
		roots = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"roots": roots,
	})
}

// handleSaveDiscoveryConfig menyimpan direktori kustom baru untuk pemindaian resource host
func (s *Server) handleSaveDiscoveryConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Roots []string `json:"roots"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	// Normalisasi dan validasi direktori
	var cleaned []string
	for _, root := range req.Roots {
		t := strings.TrimSpace(root)
		if t != "" {
			cleaned = append(cleaned, t)
		}
	}

	raw, _ := json.Marshal(cleaned)
	if err := s.store.SetSetting(r.Context(), "discovery_custom_roots", string(raw)); err != nil {
		http.Error(w, `{"error":"gagal menyimpan direktori pencarian"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "discovery.config", "custom_roots", true, fmt.Sprintf("Memperbarui direktori pemindaian host: %v", cleaned), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Konfigurasi direktori pemindaian berhasil disimpan",
		"roots":   cleaned,
	})
}

// handleDiscoveryScan menjalankan pemindaian menyeluruh terhadap infrastruktur server dengan batas waktu 10 detik
func (s *Server) handleDiscoveryScan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var req struct {
		CustomRoots []string `json:"custom_roots"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	roots := req.CustomRoots
	if len(roots) == 0 {
		val, err := s.store.GetSetting(ctx, "discovery_custom_roots")
		if err == nil && val != "" {
			_ = json.Unmarshal([]byte(val), &roots)
		}
	}

	if s.discoveryScanner == nil {
		s.discoveryScanner = discovery.NewScanner()
	}

	result, scanErr := s.discoveryScanner.ScanAll(ctx, roots)
	if result == nil {
		result = &discovery.DiscoveryResult{
			NginxVhosts:    []discovery.NginxVhostItem{},
			DockerProjects: []discovery.DockerDiscoveredItem{},
			PM2Apps:        []discovery.PM2DiscoveredItem{},
			Databases:      []discovery.DatabaseDiscoveredItem{},
			KubernetesType: "none",
		}
	}
	if result.NginxVhosts == nil {
		result.NginxVhosts = []discovery.NginxVhostItem{}
	}
	if result.DockerProjects == nil {
		result.DockerProjects = []discovery.DockerDiscoveredItem{}
	}
	if result.PM2Apps == nil {
		result.PM2Apps = []discovery.PM2DiscoveredItem{}
	}
	if result.Databases == nil {
		result.Databases = []discovery.DatabaseDiscoveredItem{}
	}

	// Gunakan context terpisah agar query database tidak gagal jika context scan terkena timeout
	dbCtx, cancelDB := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelDB()

	// Bandingkan dengan situs yang sudah ada di database untuk menandai status adopsi & sinkronisasi status SSL
	existingSites, err := s.store.ListSites(dbCtx)
	if err == nil {
		siteMap := make(map[string]store.Site)
		for _, es := range existingSites {
			siteMap[strings.ToLower(es.Domain)] = es
		}
		for i := range result.NginxVhosts {
			domains := strings.Fields(result.NginxVhosts[i].ServerName)
			for _, d := range domains {
				lowerDomain := strings.ToLower(d)
				if es, ok := siteMap[lowerDomain]; ok {
					result.NginxVhosts[i].IsAdopted = true

					// Sinkronkan status SSL dari konfigurasi situs yang terdaftar di FikPanel
					if es.SSLMode == "cloudflare" {
						result.NginxVhosts[i].SSL = true
						result.NginxVhosts[i].SSLIssuer = "Cloudflare Edge SSL (Flexible)"
					} else if es.SSLMode != "none" {
						result.NginxVhosts[i].SSL = true
					}

					// Ambil detail sertifikat yang tersimpan di FikPanel bila ada
					if cert, _ := s.store.GetCertificate(dbCtx, es.ID); cert != nil && cert.Status == "issued" {
						result.NginxVhosts[i].SSL = true
						if result.NginxVhosts[i].SSLIssuer == "" {
							result.NginxVhosts[i].SSLIssuer = cert.Issuer
						}
						result.NginxVhosts[i].SSLNotAfter = cert.NotAfter
					}
					break
				}
			}

			// Jika status SSL masih false, periksa apakah domain diproxy oleh Cloudflare Anycast Edge
			if !result.NginxVhosts[i].SSL {
				for _, d := range domains {
					lowerDomain := strings.ToLower(d)
					if cloudflare.IsDomainProxiedByCloudflare(ctx, lowerDomain) {
						result.NginxVhosts[i].SSL = true
						result.NginxVhosts[i].SSLIssuer = "Cloudflare Edge SSL (Flexible)"

						// Jika situs sudah diadopsi dan sebelumnya masih 'none', perbarui otomatis ke mode cloudflare
						if es, ok := siteMap[lowerDomain]; ok && es.SSLMode == "none" {
							es.SSLMode = "cloudflare"
							es.ForceHTTPS = 0
							es.HSTS = 0
							_ = s.store.UpdateSite(dbCtx, &es)

							certDomainsJSON, _ := json.Marshal(sites.AllDomains(&es))
							certRecord := store.Certificate{
								SiteID:    es.ID,
								Status:    "issued",
								Issuer:    "Cloudflare Edge SSL (Flexible)",
								NotBefore: time.Now().Add(-24 * time.Hour).Unix(),
								NotAfter:  time.Now().Add(3650 * 24 * time.Hour).Unix(),
								Domains:   string(certDomainsJSON),
								RenewedAt: time.Now().Unix(),
							}
							_ = s.store.UpsertCertificate(dbCtx, &certRecord)
						}
						break
					}
				}
			}
		}
	}

	resp := map[string]interface{}{
		"ok":     true,
		"result": result,
	}
	if scanErr != nil || ctx.Err() != nil {
		resp["partial"] = true
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleDiscoveryAdopt mendaftarkan konfigurasi vhost/aplikasi yang ditemukan ke sistem database panel
func (s *Server) handleDiscoveryAdopt(w http.ResponseWriter, r *http.Request) {
	var payload AdoptPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	adoptedSitesCount := 0
	var adoptedNames []string

	for _, item := range payload.Sites {
		domain := strings.TrimSpace(item.PrimaryDomain)
		if domain == "" {
			continue
		}

		// Cek apakah sudah ada di database
		taken, err := s.store.DomainTaken(r.Context(), domain, 0)
		if err != nil || taken {
			continue
		}

		kind := "static"
		if item.ProxyPass != "" {
			lowerProxy := strings.ToLower(item.ProxyPass)
			// Abaikan protokol non-HTTP seperti ssh:// atau tcp://
			if strings.HasPrefix(lowerProxy, "ssh://") || strings.HasPrefix(lowerProxy, "tcp://") || strings.HasPrefix(lowerProxy, "rdp://") {
				continue
			}
			kind = "proxy"
		} else if item.HasPHP {
			kind = "php"
		}

		sslMode := "none"
		if item.HasSSL {
			sslMode = "custom"
			lowerIssuer := strings.ToLower(item.SSLIssuer)
			lowerCert := strings.ToLower(item.SSLCertPath)
			if strings.Contains(lowerIssuer, "cloudflare") {
				sslMode = "cloudflare"
			} else if strings.Contains(lowerIssuer, "letsencrypt") || strings.Contains(lowerIssuer, "let's encrypt") || strings.Contains(lowerCert, "letsencrypt") {
				sslMode = "letsencrypt"
			}
		} else if strings.Contains(strings.ToLower(item.SSLIssuer), "cloudflare") || cloudflare.IsDomainProxiedByCloudflare(r.Context(), domain) {
			sslMode = "cloudflare"
		}

		aliasJSON, _ := json.Marshal(item.Aliases)
		site := store.Site{
			Domain:      domain,
			Aliases:     string(aliasJSON),
			Kind:        kind,
			Upstream:    item.ProxyPass,
			RootPath:    item.DocumentRoot,
			PHPVersion:  "8.3",
			SSLMode:     sslMode,
			ForceHTTPS:  0,
			Enabled:     1,
			Description: "Diadopsi secara otomatis dari host server oleh FikPanel",
		}

		created, err := s.store.CreateSite(r.Context(), &site)
		if err != nil && site.SSLMode == "cloudflare" {
			// Fallback aman jika database belum memperbarui constraint
			site.SSLMode = "none"
			created, err = s.store.CreateSite(r.Context(), &site)
		}

		if err != nil {
			log.Printf("[discovery] Gagal mengadopsi situs %s: %v", domain, err)
			continue
		}

		if created != nil {
			adoptedSitesCount++
			adoptedNames = append(adoptedNames, domain)

			// Jika vhost memiliki SSL aktif, simpan metadata sertifikat ke database & salin berkas fisik cert/key
			if item.HasSSL {
				issuer := item.SSLIssuer
				if issuer == "" {
					if sslMode == "letsencrypt" {
						issuer = "Let's Encrypt"
					} else {
						issuer = "Custom SSL"
					}
				}
				notAfter := item.SSLNotAfter
				if notAfter <= 0 {
					notAfter = time.Now().Add(90 * 24 * time.Hour).Unix()
				}
				certDomainsJSON, _ := json.Marshal(append([]string{domain}, item.Aliases...))
				certRecord := store.Certificate{
					SiteID:    created.ID,
					Status:    "issued",
					Issuer:    issuer,
					NotBefore: time.Now().Add(-24 * time.Hour).Unix(),
					NotAfter:  notAfter,
					Domains:   string(certDomainsJSON),
					RenewedAt: time.Now().Unix(),
				}
				_ = s.store.UpsertCertificate(r.Context(), &certRecord)

				// Salin fisik cert & key dari path asal host ke direktori internal FikPanel
				if s.sites != nil && item.SSLCertPath != "" && item.SSLKeyPath != "" {
					certBytes, errC := os.ReadFile(item.SSLCertPath)
					keyBytes, errK := os.ReadFile(item.SSLKeyPath)
					if errC == nil && errK == nil {
						targetCert, targetKey := s.sites.CertFiles(domain)
						if err := os.MkdirAll(filepath.Dir(targetCert), 0o755); err == nil {
							_ = os.WriteFile(targetCert, certBytes, 0o644)
							_ = os.WriteFile(targetKey, keyBytes, 0o600)
						}
					}
				}
			} else if sslMode == "cloudflare" {
				certDomainsJSON, _ := json.Marshal(sites.AllDomains(created))
				certRecord := store.Certificate{
					SiteID:    created.ID,
					Status:    "issued",
					Issuer:    "Cloudflare Edge SSL (Flexible)",
					NotBefore: time.Now().Add(-24 * time.Hour).Unix(),
					NotAfter:  time.Now().Add(3650 * 24 * time.Hour).Unix(),
					Domains:   string(certDomainsJSON),
					RenewedAt: time.Now().Unix(),
				}
				_ = s.store.UpsertCertificate(r.Context(), &certRecord)
			}

			// Terapkan konfigurasi vhost Nginx FikPanel langsung agar blok listen 443 ssl aktif
			if s.sites != nil {
				_ = s.sites.Apply(r.Context(), created, "")
			}
		}
	}

	if adoptedSitesCount > 0 {
		_ = s.store.RecordAudit(r.Context(), "user", "discovery.adopt", strings.Join(adoptedNames, ", "), true,
			fmt.Sprintf("Mengadopsi %d situs vhost yang ada di server ke dalam FikPanel", adoptedSitesCount), nil)
	}

	msg := fmt.Sprintf("Berhasil mengadopsi %d situs ke dalam FikPanel", adoptedSitesCount)
	if adoptedSitesCount == 0 {
		msg = "Tidak ada situs baru yang dapat diadopsi (semua situs yang ditemukan sudah terdaftar di FikPanel)"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":            true,
		"adopted_sites": adoptedSitesCount,
		"message":       msg,
	})
}
