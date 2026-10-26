package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"fikpanel/internal/discovery"
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

// handleDiscoveryScan menjalankan pemindaian menyeluruh terhadap infrastruktur server
func (s *Server) handleDiscoveryScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomRoots []string `json:"custom_roots"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	roots := req.CustomRoots
	if len(roots) == 0 {
		val, err := s.store.GetSetting(r.Context(), "discovery_custom_roots")
		if err == nil && val != "" {
			_ = json.Unmarshal([]byte(val), &roots)
		}
	}

	if s.discoveryScanner == nil {
		s.discoveryScanner = discovery.NewScanner()
	}

	result, err := s.discoveryScanner.ScanAll(r.Context(), roots)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal melakukan pemindaian server: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Bandingkan dengan situs yang sudah ada di database untuk menandai status adopsi
	existingSites, err := s.store.ListSites(r.Context())
	if err == nil {
		registeredDomains := make(map[string]bool)
		for _, es := range existingSites {
			registeredDomains[strings.ToLower(es.Domain)] = true
		}
		for i := range result.NginxVhosts {
			domains := strings.Fields(result.NginxVhosts[i].ServerName)
			for _, d := range domains {
				if registeredDomains[strings.ToLower(d)] {
					result.NginxVhosts[i].IsAdopted = true
					break
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"result": result,
	})
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
			kind = "proxy"
		} else if item.HasPHP {
			kind = "php"
		}

		sslMode := "none"
		if item.HasSSL {
			sslMode = "custom"
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
		if err == nil && created != nil {
			adoptedSitesCount++
			adoptedNames = append(adoptedNames, domain)
		}
	}

	if adoptedSitesCount > 0 {
		_ = s.store.RecordAudit(r.Context(), "user", "discovery.adopt", strings.Join(adoptedNames, ", "), true,
			fmt.Sprintf("Mengadopsi %d situs vhost yang ada di server ke dalam FikPanel", adoptedSitesCount), nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":            true,
		"adopted_sites": adoptedSitesCount,
		"message":       fmt.Sprintf("Berhasil mengadopsi %d situs ke dalam FikPanel", adoptedSitesCount),
	})
}
