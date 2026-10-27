package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"fikpanel/internal/nginx"
	"fikpanel/internal/sites"
	"fikpanel/internal/store"
)

// writeSiteError memetakan error domain situs/nginx ke HTTP status
func writeSiteError(w http.ResponseWriter, err error) {
	var sv *sites.ValidationError
	var sc *sites.ConflictError
	var nv *nginx.ValidationError
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "situs tidak ditemukan"})
	case errors.As(err, &sv):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": sv.Msg})
	case errors.As(err, &sc):
		writeJSON(w, http.StatusConflict, map[string]string{"error": sc.Msg})
	case errors.As(err, &nv):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

func siteID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, &sites.ValidationError{Msg: "ID situs tidak valid"}
	}
	return id, nil
}

func (s *Server) auditSite(r *http.Request, action, target string, ok bool, detail string) {
	var uid *int64
	if u := GetUserFromContext(r.Context()); u != nil {
		uid = &u.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", action, target, ok, detail, uid)
}

// SiteInput adalah payload pembuatan/perubahan situs
type SiteInput struct {
	Domains     string `json:"domains"`
	AddWWW      bool   `json:"add_www"`
	Kind        string `json:"kind"`
	User        string `json:"user"`
	Upstream    string `json:"upstream"`
	RootPath    string `json:"root_path"`
	PHPVersion  string `json:"php_version"`
	Description string `json:"description"`
	ExtraConf   string `json:"extra_conf"`
}

// domainReport adalah hasil analisis daftar domain untuk validasi realtime di UI
type domainReport struct {
	Primary   string              `json:"primary"`
	Aliases   []string            `json:"aliases"`
	Checks    []sites.DomainCheck `json:"checks"`
	DNS       []sites.DNSResult   `json:"dns"`
	ServerIPs []string            `json:"server_ips"`
	Valid     bool                `json:"valid"`
}

// analyzeDomains memvalidasi format, ketersediaan, dan (opsional) DNS seluruh domain input
func (s *Server) analyzeDomains(r *http.Request, text string, addWWW bool, excludeID int64, withDNS bool) (domainReport, error) {
	primary, aliases, checks := sites.ParseDomainList(text, addWWW)
	rep := domainReport{Primary: primary, Aliases: aliases, Checks: checks, DNS: []sites.DNSResult{}, ServerIPs: []string{}}
	if rep.Aliases == nil {
		rep.Aliases = []string{}
	}

	valid := primary != ""
	for i := range rep.Checks {
		c := &rep.Checks[i]
		if c.Error != "" {
			valid = false
			continue
		}
		taken, err := s.store.DomainTaken(r.Context(), c.Domain, excludeID)
		if err != nil {
			return rep, err
		}
		if taken {
			c.Error = "sudah dipakai situs lain"
			valid = false
		}
	}
	rep.Valid = valid

	if withDNS && primary != "" {
		rep.DNS, rep.ServerIPs = s.sites.CheckDNS(r.Context(), append([]string{primary}, aliases...))
	}
	return rep, nil
}

// buildSite memvalidasi input dan menyusun struct Site (tanpa menyentuh database)
func (s *Server) buildSite(r *http.Request, in SiteInput, base *store.Site) (*store.Site, error) {
	rep, err := s.analyzeDomains(r, in.Domains, in.AddWWW, base.ID, false)
	if err != nil {
		return nil, err
	}
	if !rep.Valid {
		for _, c := range rep.Checks {
			if c.Error != "" {
				return nil, &sites.ValidationError{Msg: fmt.Sprintf("%s: %s", c.Input, c.Error)}
			}
		}
		return nil, &sites.ValidationError{Msg: "minimal satu domain utama (non-wildcard) wajib diisi"}
	}

	site := *base
	site.Domain = rep.Primary
	aliasJSON, _ := json.Marshal(rep.Aliases)
	site.Aliases = string(aliasJSON)
	site.Description = strings.TrimSpace(in.Description)
	if len(site.Description) > 200 {
		return nil, &sites.ValidationError{Msg: "deskripsi maksimal 200 karakter"}
	}
	site.ExtraConf = strings.TrimSpace(in.ExtraConf)

	site.Kind = in.Kind
	site.Upstream, site.RootPath, site.PHPVersion = "", "", ""
	switch in.Kind {
	case "proxy", "node", "go", "python":
		ups := in.Upstream
		if strings.TrimSpace(ups) == "" {
			switch in.Kind {
			case "node":
				ups = "http://127.0.0.1:3000"
			case "go":
				ups = "http://127.0.0.1:8080"
			case "python":
				ups = "http://127.0.0.1:8000"
			default:
				ups = "http://127.0.0.1:3000"
			}
		}
		if site.Upstream, err = sites.ValidateUpstream(ups); err != nil {
			return nil, &sites.ValidationError{Msg: err.Error()}
		}
		root := in.RootPath
		if strings.TrimSpace(root) == "" {
			root = sites.DefaultRoot(rep.Primary, in.Kind, in.User)
		}
		if site.RootPath, err = sites.ValidateRootPath(root); err != nil {
			return nil, &sites.ValidationError{Msg: err.Error()}
		}
	case "static", "php":
		root := in.RootPath
		if strings.TrimSpace(root) == "" {
			root = sites.DefaultRoot(rep.Primary, in.Kind, in.User)
		}
		if site.RootPath, err = sites.ValidateRootPath(root); err != nil {
			return nil, &sites.ValidationError{Msg: err.Error()}
		}
		if in.Kind == "php" {
			if err := sites.ValidatePHPVersion(in.PHPVersion); err != nil {
				return nil, &sites.ValidationError{Msg: err.Error()}
			}
			site.PHPVersion = in.PHPVersion
		}
	default:
		return nil, &sites.ValidationError{Msg: "jenis situs harus static, php, node, go, python, atau proxy"}
	}
	return &site, nil
}

func (s *Server) handleListSites(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListSites(r.Context())
	if err != nil {
		writeSiteError(w, err)
		return
	}
	if list == nil {
		list = []store.Site{}
	}
	ws, err := s.nginx.Status(r.Context())
	if err != nil {
		writeSiteError(w, err)
		return
	}
	webServer := map[string]interface{}{
		"name": "nginx", "display_name": "Nginx Web Server", "category": "webserver",
		"installed": ws.Installed, "version": ws.Version, "running": ws.Running, "service_unit": "nginx.service",
	}
	if r.URL.Query().Get("mock_nginx") == "0" {
		webServer["installed"], webServer["running"] = false, false
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"sites": list, "web_server": webServer})
}

func (s *Server) handleSiteMeta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"php_versions": s.sites.PHPVersions(),
		"server_ips":   s.sites.ServerIPs(r.Context()),
	})
}

func (s *Server) handleCheckDomains(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domains   string `json:"domains"`
		AddWWW    bool   `json:"add_www"`
		ExcludeID int64  `json:"exclude_id"`
		DNS       bool   `json:"dns"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	if len(req.Domains) > 4096 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "daftar domain terlalu panjang"})
		return
	}
	rep, err := s.analyzeDomains(r, req.Domains, req.AddWWW, req.ExcludeID, req.DNS)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (s *Server) handleCreateSite(w http.ResponseWriter, r *http.Request) {
	var in SiteInput
	if !decodeNginxBody(w, r, &in) {
		return
	}
	site, err := s.buildSite(r, in, &store.Site{SSLMode: "none", ForceHTTPS: 1, Enabled: 1})
	if err != nil {
		writeSiteError(w, err)
		return
	}

	created, err := s.store.CreateSite(r.Context(), site)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	// Situs baru hanya dianggap sah bila web root & vhost berhasil diterapkan; selain itu baris DB dibatalkan
	if err := s.sites.EnsureRoot(created); err == nil {
		err = s.sites.Apply(r.Context(), created, "")
		if err != nil {
			_ = s.store.DeleteSite(r.Context(), created.ID)
			s.auditSite(r, "site.create", created.Domain, false, err.Error())
			writeSiteError(w, err)
			return
		}
	} else {
		_ = s.store.DeleteSite(r.Context(), created.ID)
		writeSiteError(w, err)
		return
	}

	s.auditSite(r, "site.create", created.Domain, true,
		fmt.Sprintf("kind=%s alias=%s", created.Kind, created.Aliases))
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetSite(w http.ResponseWriter, r *http.Request) {
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
	cert, err := s.store.GetCertificate(r.Context(), id)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"site":        site,
		"domains":     sites.AllDomains(site),
		"certificate": cert,
		"ssl_job":     s.sites.Job(id),
	})
}

func (s *Server) handleUpdateSite(w http.ResponseWriter, r *http.Request) {
	id, err := siteID(r)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	current, err := s.store.GetSite(r.Context(), id)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	var in SiteInput
	if !decodeNginxBody(w, r, &in) {
		return
	}
	next, err := s.buildSite(r, in, current)
	if err != nil {
		writeSiteError(w, err)
		return
	}

	// Sertifikat lama tidak lagi mencakup daftar domain baru, jadi HTTPS dinonaktifkan sampai terbit ulang
	domainsChanged := next.Domain != current.Domain || next.Aliases != current.Aliases
	if domainsChanged && current.SSLStatus == "issued" {
		if err := s.store.DeleteCertificate(r.Context(), id); err != nil {
			writeSiteError(w, err)
			return
		}
	}

	if err := s.store.UpdateSite(r.Context(), next); err != nil {
		writeSiteError(w, err)
		return
	}
	if err := s.sites.EnsureRoot(next); err != nil {
		_ = s.store.UpdateSite(r.Context(), current)
		writeSiteError(w, err)
		return
	}
	if err := s.sites.Apply(r.Context(), next, current.Domain); err != nil {
		_ = s.store.UpdateSite(r.Context(), current)
		s.auditSite(r, "site.update", current.Domain, false, err.Error())
		writeSiteError(w, err)
		return
	}
	s.auditSite(r, "site.update", next.Domain, true, "Konfigurasi situs diperbarui")

	fresh, err := s.store.GetSite(r.Context(), id)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "site": fresh, "ssl_reset": domainsChanged && current.SSLStatus == "issued"})
}

func (s *Server) handleToggleSite(w http.ResponseWriter, r *http.Request) {
	id, err := siteID(r)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	newState, err := s.store.ToggleSite(r.Context(), id)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	site, err := s.store.GetSite(r.Context(), id)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	if err := s.sites.Apply(r.Context(), site, ""); err != nil {
		_, _ = s.store.ToggleSite(r.Context(), id)
		writeSiteError(w, err)
		return
	}
	s.auditSite(r, "site.toggle", site.Domain, true, fmt.Sprintf("enabled=%d", newState))
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "id": id, "enabled": newState})
}

func (s *Server) handleDeleteSite(w http.ResponseWriter, r *http.Request) {
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
	if err := s.sites.Remove(r.Context(), site.Domain); err != nil {
		writeSiteError(w, err)
		return
	}
	if err := s.store.DeleteSite(r.Context(), id); err != nil {
		writeSiteError(w, err)
		return
	}
	s.auditSite(r, "site.delete", site.Domain, true, "Situs dan vhost dihapus (file web root dipertahankan)")
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "id": id})
}

func (s *Server) handleSiteVhost(w http.ResponseWriter, r *http.Request) {
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
	cert, err := s.store.GetCertificate(r.Context(), id)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	content, err := s.sites.Render(site, cert)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"content": content})
}

// handleSaveSiteVhost menyimpan vhost Nginx mentah setelah lolos uji sintaks
func (s *Server) handleSaveSiteVhost(w http.ResponseWriter, r *http.Request) {
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
	var req struct {
		Content string `json:"content"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	if err := s.sites.SaveRawVhost(r.Context(), site, req.Content); err != nil {
		writeSiteError(w, err)
		return
	}
	s.auditSite(r, "site.vhost.save", site.Domain, true, "Konfigurasi vhost mentah berhasil disimpan")
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleSiteLogs membaca potongan baris log akses atau error domain
func (s *Server) handleSiteLogs(w http.ResponseWriter, r *http.Request) {
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
	logType := r.URL.Query().Get("type")
	if logType != "error" {
		logType = "access"
	}
	content, err := s.sites.GetLog(site.Domain, logType, 200)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"domain":  site.Domain,
		"type":    logType,
		"content": content,
	})
}

// handleClearSingleSiteLogs mengosongkan riwayat log akses dan error spesifik situs
func (s *Server) handleClearSingleSiteLogs(w http.ResponseWriter, r *http.Request) {
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
	if err := s.sites.ClearLog(site.Domain); err != nil {
		writeSiteError(w, err)
		return
	}
	s.auditSite(r, "site.logs.clear", site.Domain, true, "Log akses dan error situs dikosongkan")
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleSiteCustomSSL menerima kunci privat dan sertifikat TLS kustom
func (s *Server) handleSiteCustomSSL(w http.ResponseWriter, r *http.Request) {
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
	var req struct {
		Key  string `json:"key"`
		Cert string `json:"cert"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	cert, err := s.sites.SaveCustomCert(r.Context(), id, site.Domain, req.Key, req.Cert)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	s.auditSite(r, "ssl.custom", site.Domain, true, fmt.Sprintf("Sertifikat kustom dipasang: issuer=%s", cert.Issuer))
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "certificate": cert})
}

// handleSiteGetCerts mengambil kunci privat dan sertifikat tersimpan untuk domain
func (s *Server) handleSiteGetCerts(w http.ResponseWriter, r *http.Request) {
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
	certData, keyData, _ := s.sites.GetCertPair(site.Domain)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cert": certData,
		"key":  keyData,
	})
}

func (s *Server) handleSiteSSL(w http.ResponseWriter, r *http.Request) {
	id, err := siteID(r)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	var req struct {
		Enabled    bool `json:"enabled"`
		ForceHTTPS bool `json:"force_https"`
		HSTS       bool `json:"hsts"`
		Issue      bool `json:"issue"`
		SkipDNS    bool `json:"skip_dns"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	site, err := s.store.GetSite(r.Context(), id)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	prev := *site

	site.SSLMode = "none"
	if req.Enabled {
		site.SSLMode = "letsencrypt"
	}
	site.ForceHTTPS = boolInt(req.ForceHTTPS)
	site.HSTS = boolInt(req.HSTS && req.ForceHTTPS)
	if err := s.store.UpdateSite(r.Context(), site); err != nil {
		writeSiteError(w, err)
		return
	}
	if err := s.sites.Apply(r.Context(), site, ""); err != nil {
		_ = s.store.UpdateSite(r.Context(), &prev)
		writeSiteError(w, err)
		return
	}

	issuing := false
	if req.Enabled && (req.Issue || site.SSLStatus != "issued") {
		var uid *int64
		if u := GetUserFromContext(r.Context()); u != nil {
			uid = &u.ID
		}
		domain := site.Domain
		err := s.sites.IssueSSL(r.Context(), id, req.SkipDNS, func(ok bool, msg string) {
			_ = s.store.RecordAudit(s.bgCtx, "user", "ssl.issue", domain, ok, msg, uid)
		})
		if err != nil {
			writeSiteError(w, err)
			return
		}
		issuing = true
	}

	s.auditSite(r, "ssl.settings", site.Domain, true,
		fmt.Sprintf("enabled=%t force_https=%t hsts=%t", req.Enabled, req.ForceHTTPS, req.HSTS))
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "issuing": issuing, "ssl_job": s.sites.Job(id)})
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
