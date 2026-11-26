package waf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// InstalledStackItem merepresentasikan satu komponen software/runtime yang terdeteksi di server host
type InstalledStackItem struct {
	Product   string `json:"product"`
	Label     string `json:"label"`
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
}

// CVEAdvisory merepresentasikan satu berita kerentanan CVE dari CISA KEV / GitHub Advisories / Katalog Web Kritis
type CVEAdvisory struct {
	CVEID              string  `json:"cve_id"`
	GHSAID             string  `json:"ghsa_id,omitempty"`
	Title              string  `json:"title"`
	Description        string  `json:"description"`
	Severity           string  `json:"severity"` // critical, high, medium, low
	CVSSScore          float64 `json:"cvss_score"`
	Source             string  `json:"source"` // cisa_kev, github_advisory, builtin_advisory
	Product            string  `json:"product"`
	AffectedRange      string  `json:"affected_range,omitempty"`
	PatchedVersion     string  `json:"patched_version,omitempty"`
	PublishedAt        string  `json:"published_at"`
	IsKEV              bool    `json:"is_kev"`
	ReferenceURL       string  `json:"reference_url"`
	VirtualPatchID     string  `json:"virtual_patch_id,omitempty"`
	VirtualPatchActive bool    `json:"virtual_patch_active"`
	HostMatch          string  `json:"host_match"` // affected, installed_stack, global
	InstalledVersion   string  `json:"installed_version,omitempty"`
}

// CVEFeedReport merangkum hasil feed CVE, deteksi stack host, dan daftar virtual patch 1-klik
type CVEFeedReport struct {
	LastFetchedAt      int64                `json:"last_fetched_at"`
	SourcesStatus      map[string]string    `json:"sources_status"`
	InstalledStack     []InstalledStackItem `json:"installed_stack"`
	TotalCount         int                  `json:"total_count"`
	KEVCount           int                  `json:"kev_count"`
	HostMatchedCount   int                  `json:"host_matched_count"`
	ActivePatchesCount int                  `json:"active_patches_count"`
	Advisories         []CVEAdvisory        `json:"advisories"`
	VirtualPatches     []VirtualPatch       `json:"virtual_patches"`
}

var (
	stackCacheMu      sync.Mutex
	cachedStack       []InstalledStackItem
	cachedStackAt     time.Time
	cveHTTPClient     = &http.Client{Timeout: 8 * time.Second}
	reSemverExtract   = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)
	cveFeedTTLSeconds = int64(6 * 3600)
)

// curatedCriticalAdvisories menjadi jaring pengaman berita CVE web/server kritikal terbaru agar feed tidak pernah kosong saat offline
var curatedCriticalAdvisories = []CVEAdvisory{
	{
		CVEID:          "CVE-2025-55182",
		Title:          "React Server Components / Flight Protocol RCE (React2Shell)",
		Description:    "Kerentanan deserialisasi kritis pada protokol Flight React Server Components (RSC) dan Next.js App Router yang memungkinkan Remote Code Execution tanpa autentikasi.",
		Severity:       "critical",
		CVSSScore:      10.0,
		Source:         "cisa_kev",
		Product:        "react",
		AffectedRange:  "react-server-dom 19.0.0–19.2.0 / Next.js 15.x–16.x",
		PatchedVersion: "React 19.2.1+ / Next.js 15.5.7+",
		PublishedAt:    "2025-12-03",
		IsKEV:          true,
		ReferenceURL:   "https://nvd.nist.gov/vuln/detail/CVE-2025-55182",
		VirtualPatchID: "vp-react2shell-2025-55182",
	},
	{
		CVEID:          "CVE-2025-29927",
		Title:          "Next.js Middleware Authorization Bypass via x-middleware-subrequest",
		Description:    "Penyerang dapat melewati pemeriksaan autentikasi di Middleware Next.js secara penuh hanya dengan menyuntikkan header HTTP x-middleware-subrequest.",
		Severity:       "critical",
		CVSSScore:      9.1,
		Source:         "github_advisory",
		Product:        "nextjs",
		AffectedRange:  "Next.js 11.1.4 – 15.2.2",
		PatchedVersion: "14.2.25 / 15.2.3",
		PublishedAt:    "2025-03-21",
		IsKEV:          true,
		ReferenceURL:   "https://nvd.nist.gov/vuln/detail/CVE-2025-29927",
		VirtualPatchID: "vp-nextjs-2025-29927",
	},
	{
		CVEID:          "CVE-2024-4577",
		Title:          "PHP-CGI Argument Injection Remote Code Execution (PHP2Shell)",
		Description:    "Celah encoding karakter soft-hyphen (%AD) pada PHP-CGI yang memungkinkan penyerang menyuntikkan opsi -d allow_url_include & auto_prepend_file untuk menjalankan kode PHP jarak jauh.",
		Severity:       "critical",
		CVSSScore:      9.8,
		Source:         "cisa_kev",
		Product:        "php",
		AffectedRange:  "PHP < 8.1.29, < 8.2.20, < 8.3.8",
		PatchedVersion: "8.1.29 / 8.2.20 / 8.3.8+",
		PublishedAt:    "2024-06-06",
		IsKEV:          true,
		ReferenceURL:   "https://nvd.nist.gov/vuln/detail/CVE-2024-4577",
		VirtualPatchID: "vp-php-cgi-2024-4577",
	},
	{
		CVEID:          "CVE-2024-7347",
		Title:          "NGINX ngx_http_mp4_module Out-of-Bounds Memory Read",
		Description:    "Berkas MP4 rekayasa khusus dapat memicu pembacaan memori di luar batas dan crash pada worker process Nginx.",
		Severity:       "high",
		CVSSScore:      7.5,
		Source:         "github_advisory",
		Product:        "nginx",
		AffectedRange:  "NGINX < 1.26.2 / < 1.27.1",
		PatchedVersion: "1.26.2 / 1.27.1+",
		PublishedAt:    "2024-08-14",
		IsKEV:          false,
		ReferenceURL:   "https://nvd.nist.gov/vuln/detail/CVE-2024-7347",
		VirtualPatchID: "vp-nginx-traversal",
	},
	{
		CVEID:          "CVE-2024-6387",
		Title:          "OpenSSH Server Remote Unauthenticated Code Execution (regreSSHion)",
		Description:    "Race condition pada signal handler sshd di sistem glibc Linux yang memungkinkan eskalasi RCE sebagai root dari jarak jauh.",
		Severity:       "high",
		CVSSScore:      8.1,
		Source:         "cisa_kev",
		Product:        "openssh",
		AffectedRange:  "OpenSSH 8.5p1 – 9.7p1",
		PatchedVersion: "OpenSSH 9.8p1+",
		PublishedAt:    "2024-07-01",
		IsKEV:          true,
		ReferenceURL:   "https://nvd.nist.gov/vuln/detail/CVE-2024-6387",
	},
	{
		CVEID:          "CVE-2025-22871",
		Title:          "Go net/http Request Smuggling pada Bare LF Chunked Encoding",
		Description:    "Pustaka standar net/http Golang menerima bare LF sebagai terminator chunk-size yang berpotensi memicu HTTP Request Smuggling di balik reverse proxy.",
		Severity:       "high",
		CVSSScore:      7.5,
		Source:         "github_advisory",
		Product:        "go",
		AffectedRange:  "Go < 1.23.8 / < 1.24.2",
		PatchedVersion: "Go 1.23.8 / 1.24.2+",
		PublishedAt:    "2025-04-02",
		IsKEV:          false,
		ReferenceURL:   "https://nvd.nist.gov/vuln/detail/CVE-2025-22871",
		VirtualPatchID: "vp-nginx-traversal",
	},
	{
		CVEID:          "CVE-2021-3129",
		Title:          "Laravel Ignition Debug Mode Remote Code Execution",
		Description:    "Mode debug Laravel dengan Facade Ignition rentan terhadap eksploitasi phar:// pada endpoint /_ignition/execute-solution.",
		Severity:       "critical",
		CVSSScore:      9.8,
		Source:         "cisa_kev",
		Product:        "laravel",
		AffectedRange:  "facade/ignition <= 2.5.1",
		PatchedVersion: "facade/ignition 2.5.2+",
		PublishedAt:    "2021-01-13",
		IsKEV:          true,
		ReferenceURL:   "https://nvd.nist.gov/vuln/detail/CVE-2021-3129",
		VirtualPatchID: "vp-laravel-ignition",
	},
}

// DetectInstalledStack memindai versi aktual perangkat lunak web/server yang terpasang pada host
func (m *Manager) DetectInstalledStack(ctx context.Context) []InstalledStackItem {
	stackCacheMu.Lock()
	if len(cachedStack) > 0 && time.Since(cachedStackAt) < 10*time.Minute {
		cp := make([]InstalledStackItem, len(cachedStack))
		copy(cp, cachedStack)
		stackCacheMu.Unlock()
		return cp
	}
	stackCacheMu.Unlock()

	runVer := func(bin string, args ...string) (string, bool) {
		if _, err := exec.LookPath(bin); err != nil {
			return "", false
		}
		tCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		defer cancel()
		out, _ := exec.CommandContext(tCtx, bin, args...).CombinedOutput()
		text := strings.TrimSpace(string(out))
		if m := reSemverExtract.FindStringSubmatch(text); len(m) == 2 {
			return m[1], true
		}
		return "terpasang", true
	}

	items := make([]InstalledStackItem, 0, 10)
	add := func(prod, label, ver string, ok bool) {
		items = append(items, InstalledStackItem{Product: prod, Label: label, Version: ver, Installed: ok})
	}

	if v, ok := runVer("nginx", "-v"); ok {
		add("nginx", "Nginx", v, true)
	}
	if v, ok := runVer("php", "-v"); ok {
		add("php", "PHP", v, true)
	}
	if v, ok := runVer("node", "-v"); ok {
		add("node", "Node.js", v, true)
	}
	if v, ok := runVer("go", "version"); ok {
		add("go", "Go Runtime", v, true)
	} else {
		// Biner FikPanel sendiri menggunakan runtime Go
		add("go", "Go Runtime (Panel)", strings.TrimPrefix(runtime.Version(), "go"), true)
	}
	if v, ok := runVer("openssl", "version"); ok {
		add("openssl", "OpenSSL", v, true)
	}
	if v, ok := runVer("sshd", "-V"); ok {
		add("openssh", "OpenSSH", v, true)
	}
	if v, ok := runVer("mariadb", "--version"); ok {
		add("mysql", "MariaDB / MySQL", v, true)
	} else if v, ok := runVer("mysql", "--version"); ok {
		add("mysql", "MySQL", v, true)
	}
	if v, ok := runVer("docker", "--version"); ok {
		add("docker", "Docker", v, true)
	}

	// Tambahkan deteksi framework dari situs yang dikelola FikPanel (Laravel, Next.js, React, WordPress)
	if sites, err := m.store.ListSites(ctx); err == nil {
		fwSeen := make(map[string]bool)
		for _, s := range sites {
			fw := strings.ToLower(strings.TrimSpace(s.Framework))
			if strings.Contains(fw, "laravel") && !fwSeen["laravel"] {
				fwSeen["laravel"] = true
				add("laravel", "Laravel App", "terdeteksi", true)
			}
			if (strings.Contains(fw, "next") || strings.Contains(fw, "react")) && !fwSeen["nextjs"] {
				fwSeen["nextjs"] = true
				add("nextjs", "Next.js / React", "terdeteksi", true)
				add("react", "React RSC", "terdeteksi", true)
			}
			if strings.Contains(fw, "wordpress") && !fwSeen["wordpress"] {
				fwSeen["wordpress"] = true
				add("wordpress", "WordPress", "terdeteksi", true)
			}
		}
	}

	stackCacheMu.Lock()
	cachedStack = items
	cachedStackAt = time.Now()
	stackCacheMu.Unlock()
	return items
}

// GetCVEFeedReport menyajikan daftar berita CVE terbaru (CISA KEV + GitHub Advisories) dengan korelasi stack host
func (m *Manager) GetCVEFeedReport(ctx context.Context, forceRefresh bool) (*CVEFeedReport, error) {
	now := time.Now().Unix()
	lastFetched := m.getLatestCVEFetchTime(ctx)
	sourcesStatus := map[string]string{
		"cisa_kev":        "cache",
		"github_advisory": "cache",
	}

	// Segarkan dari sumber eksternal jika cache kosong, sudah > 6 jam, atau diminta eksplisit oleh admin
	if forceRefresh || lastFetched == 0 || (now-lastFetched) > cveFeedTTLSeconds {
		sourcesStatus = m.syncLiveCVEFeeds(ctx)
		lastFetched = m.getLatestCVEFetchTime(ctx)
		if lastFetched == 0 {
			lastFetched = now
		}
	}

	stack := m.DetectInstalledStack(ctx)
	stackMap := make(map[string]InstalledStackItem, len(stack))
	for _, st := range stack {
		if st.Installed {
			stackMap[strings.ToLower(st.Product)] = st
		}
	}

	patches := m.ListVirtualPatches(ctx)
	patchActive := make(map[string]bool, len(patches))
	activePatchCount := 0
	for _, p := range patches {
		if p.Enabled {
			patchActive[p.ID] = true
			activePatchCount++
		}
	}

	advisories := m.loadCachedCVEs(ctx)
	kevCount := 0
	hostMatchedCount := 0

	for i := range advisories {
		a := &advisories[i]
		if a.VirtualPatchID == "" {
			a.VirtualPatchID = findVirtualPatchForCVE(a.CVEID, a.Product, a.Title)
		}
		if a.VirtualPatchID != "" {
			a.VirtualPatchActive = patchActive[a.VirtualPatchID]
		}
		if a.IsKEV {
			kevCount++
		}

		// Korelasikan produk CVE dengan stack nyata yang terpasang di server ini
		if st, ok := stackMap[strings.ToLower(a.Product)]; ok {
			a.InstalledVersion = st.Version
			if (a.IsKEV || a.Severity == "critical") && !a.VirtualPatchActive {
				a.HostMatch = "affected"
			} else {
				a.HostMatch = "installed_stack"
			}
			hostMatchedCount++
		} else {
			a.HostMatch = "global"
		}
	}

	// Urutkan prioritas: terdampak di host -> stack terpasang -> KEV -> tanggal terbaru -> skor CVSS
	sort.SliceStable(advisories, func(i, j int) bool {
		rank := func(m string) int {
			switch m {
			case "affected":
				return 3
			case "installed_stack":
				return 2
			default:
				return 1
			}
		}
		ri, rj := rank(advisories[i].HostMatch), rank(advisories[j].HostMatch)
		if ri != rj {
			return ri > rj
		}
		if advisories[i].IsKEV != advisories[j].IsKEV {
			return advisories[i].IsKEV
		}
		if advisories[i].PublishedAt != advisories[j].PublishedAt {
			return advisories[i].PublishedAt > advisories[j].PublishedAt
		}
		return advisories[i].CVSSScore > advisories[j].CVSSScore
	})

	return &CVEFeedReport{
		LastFetchedAt:      lastFetched,
		SourcesStatus:      sourcesStatus,
		InstalledStack:     stack,
		TotalCount:         len(advisories),
		KEVCount:           kevCount,
		HostMatchedCount:   hostMatchedCount,
		ActivePatchesCount: activePatchCount,
		Advisories:         advisories,
		VirtualPatches:     patches,
	}, nil
}

// CountHostMatchedCVEsCached menghitung jumlah CVE relevan terhadap stack host hanya dari cache lokal (non-blocking HTTP)
func (m *Manager) CountHostMatchedCVEsCached(ctx context.Context) int {
	stack := m.DetectInstalledStack(ctx)
	if len(stack) == 0 {
		return 0
	}
	stackMap := make(map[string]bool, len(stack))
	for _, st := range stack {
		if st.Installed {
			stackMap[strings.ToLower(st.Product)] = true
		}
	}
	advisories := m.loadCachedCVEs(ctx)
	if len(advisories) == 0 {
		advisories = curatedCriticalAdvisories
	}
	matched := 0
	for _, a := range advisories {
		if stackMap[strings.ToLower(a.Product)] {
			matched++
		}
	}
	return matched
}

// syncLiveCVEFeeds menarik data secara paralel dari CISA KEV Catalog dan GitHub Security Advisories lalu menyimpannya ke SQLite
func (m *Manager) syncLiveCVEFeeds(ctx context.Context) map[string]string {
	status := map[string]string{
		"cisa_kev":        "offline",
		"github_advisory": "offline",
	}

	var mu sync.Mutex
	collected := make([]CVEAdvisory, 0, 80)
	collected = append(collected, curatedCriticalAdvisories...)

	var wg sync.WaitGroup

	// 1. Ambil CISA Known Exploited Vulnerabilities (KEV) Catalog resmi
	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := fetchCISAKEVAdvisories(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			status["cisa_kev"] = "error: " + err.Error()
			return
		}
		status["cisa_kev"] = fmt.Sprintf("ok (%d)", len(items))
		collected = append(collected, items...)
	}()

	// 2. Ambil GitHub Security Advisories terbaru (ekosistem Go, Composer/PHP, NPM/Next/React)
	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := fetchGitHubAdvisories(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			status["github_advisory"] = "error: " + err.Error()
			return
		}
		status["github_advisory"] = fmt.Sprintf("ok (%d)", len(items))
		collected = append(collected, items...)
	}()

	wg.Wait()

	// Deduplikasi berdasarkan CVE ID (prioritaskan entri KEV / CVSS lebih tinggi)
	dedup := make(map[string]CVEAdvisory, len(collected))
	for _, item := range collected {
		key := strings.ToUpper(strings.TrimSpace(item.CVEID))
		if key == "" {
			continue
		}
		item.CVEID = key
		if prev, exists := dedup[key]; exists {
			if item.IsKEV && !prev.IsKEV {
				if item.VirtualPatchID == "" {
					item.VirtualPatchID = prev.VirtualPatchID
				}
				dedup[key] = item
			} else if prev.VirtualPatchID == "" && item.VirtualPatchID != "" {
				prev.VirtualPatchID = item.VirtualPatchID
				dedup[key] = prev
			}
			continue
		}
		dedup[key] = item
	}

	now := time.Now().Unix()
	tx, err := m.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return status
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO waf_cve_cache
		(cve_id, ghsa_id, title, description, severity, cvss_score, source, product,
		 affected_range, patched_version, published_at, is_kev, reference_url, virtual_patch_id, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cve_id) DO UPDATE SET
			ghsa_id = excluded.ghsa_id,
			title = excluded.title,
			description = excluded.description,
			severity = excluded.severity,
			cvss_score = excluded.cvss_score,
			source = excluded.source,
			product = excluded.product,
			affected_range = excluded.affected_range,
			patched_version = excluded.patched_version,
			published_at = excluded.published_at,
			is_kev = excluded.is_kev,
			reference_url = excluded.reference_url,
			virtual_patch_id = excluded.virtual_patch_id,
			fetched_at = excluded.fetched_at
	`)
	if err != nil {
		return status
	}
	defer stmt.Close()

	for _, a := range dedup {
		kevInt := 0
		if a.IsKEV {
			kevInt = 1
		}
		_, _ = stmt.ExecContext(ctx,
			a.CVEID, a.GHSAID, a.Title, a.Description, a.Severity, a.CVSSScore,
			a.Source, a.Product, a.AffectedRange, a.PatchedVersion, a.PublishedAt,
			kevInt, a.ReferenceURL, a.VirtualPatchID, now,
		)
	}
	_ = tx.Commit()
	return status
}

func (m *Manager) getLatestCVEFetchTime(ctx context.Context) int64 {
	var ts int64
	_ = m.store.DB().QueryRowContext(ctx, "SELECT COALESCE(MAX(fetched_at), 0) FROM waf_cve_cache").Scan(&ts)
	return ts
}

func (m *Manager) loadCachedCVEs(ctx context.Context) []CVEAdvisory {
	rows, err := m.store.DB().QueryContext(ctx, `
		SELECT cve_id, ghsa_id, title, description, severity, cvss_score, source, product,
		       affected_range, patched_version, published_at, is_kev, reference_url, virtual_patch_id
		FROM waf_cve_cache
		ORDER BY is_kev DESC, published_at DESC, cvss_score DESC
		LIMIT 90
	`)
	if err != nil {
		return append([]CVEAdvisory(nil), curatedCriticalAdvisories...)
	}
	defer rows.Close()

	list := make([]CVEAdvisory, 0, 64)
	for rows.Next() {
		var a CVEAdvisory
		var kevInt int
		if err := rows.Scan(
			&a.CVEID, &a.GHSAID, &a.Title, &a.Description, &a.Severity, &a.CVSSScore,
			&a.Source, &a.Product, &a.AffectedRange, &a.PatchedVersion, &a.PublishedAt,
			&kevInt, &a.ReferenceURL, &a.VirtualPatchID,
		); err == nil {
			a.IsKEV = kevInt == 1
			list = append(list, a)
		}
	}
	if len(list) == 0 {
		return append([]CVEAdvisory(nil), curatedCriticalAdvisories...)
	}
	return list
}

// detectCVEProduct mengklasifikasikan CVE ke kategori stack web/server relevan (mengembalikan "" bila di luar domain web/server)
func detectCVEProduct(vendor, product, title string) string {
	joined := strings.ToLower(vendor + " " + product + " " + title)
	switch {
	case strings.Contains(joined, "nginx"):
		return "nginx"
	case strings.Contains(joined, "next.js") || strings.Contains(joined, "nextjs"):
		return "nextjs"
	case strings.Contains(joined, "react") && (strings.Contains(joined, "server") || strings.Contains(joined, "dom") || strings.Contains(joined, "flight")):
		return "react"
	case strings.Contains(joined, "laravel"):
		return "laravel"
	case strings.Contains(joined, "wordpress") || strings.Contains(joined, "wp-"):
		return "wordpress"
	case strings.Contains(joined, "php"):
		return "php"
	case strings.Contains(joined, "golang") || strings.Contains(joined, "go standard") || strings.Contains(joined, "net/http") || strings.HasPrefix(strings.ToLower(product), "go "):
		return "go"
	case strings.Contains(joined, "node.js") || strings.Contains(joined, "nodejs"):
		return "node"
	case strings.Contains(joined, "openssh") || strings.Contains(joined, "sshd"):
		return "openssh"
	case strings.Contains(joined, "openssl"):
		return "openssl"
	case strings.Contains(joined, "mysql") || strings.Contains(joined, "mariadb"):
		return "mysql"
	case strings.Contains(joined, "postgresql") || strings.Contains(joined, "postgres"):
		return "postgresql"
	case strings.Contains(joined, "redis"):
		return "redis"
	case strings.Contains(joined, "docker") || strings.Contains(joined, "runc") || strings.Contains(joined, "containerd"):
		return "docker"
	case strings.Contains(joined, "linux kernel"):
		return "linux"
	case strings.Contains(joined, "apache") || strings.Contains(joined, "tomcat") || strings.Contains(joined, "spring"):
		return "java"
	default:
		return ""
	}
}

// fetchCISAKEVAdvisories mengambil dan menyaring Katalog Kerentanan Aktif Dieksploitasi (CISA KEV)
func fetchCISAKEVAdvisories(ctx context.Context) ([]CVEAdvisory, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "FikPanel-CVEIntel/0.7.6")
	req.Header.Set("Accept", "application/json")

	resp, err := cveHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}
	return parseCISAKEVJSON(body)
}

// parseCISAKEVJSON mem-parsing JSON katalog CISA KEV menjadi daftar CVEAdvisory web/server (dipisah agar dapat diuji unit)
func parseCISAKEVJSON(raw []byte) ([]CVEAdvisory, error) {
	var doc struct {
		Vulnerabilities []struct {
			CVEID            string `json:"cveID"`
			VendorProject    string `json:"vendorProject"`
			Product          string `json:"product"`
			VulnName         string `json:"vulnerabilityName"`
			DateAdded        string `json:"dateAdded"`
			ShortDescription string `json:"shortDescription"`
			RequiredAction   string `json:"requiredAction"`
		} `json:"vulnerabilities"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}

	// Urutkan dari yang terbaru ditambahkan ke katalog KEV
	sort.Slice(doc.Vulnerabilities, func(i, j int) bool {
		return doc.Vulnerabilities[i].DateAdded > doc.Vulnerabilities[j].DateAdded
	})

	out := make([]CVEAdvisory, 0, 35)
	for _, v := range doc.Vulnerabilities {
		prod := detectCVEProduct(v.VendorProject, v.Product, v.VulnName+" "+v.ShortDescription)
		if prod == "" {
			continue
		}
		out = append(out, CVEAdvisory{
			CVEID:          v.CVEID,
			Title:          v.VulnName,
			Description:    v.ShortDescription,
			Severity:       "critical",
			CVSSScore:      9.0,
			Source:         "cisa_kev",
			Product:        prod,
			AffectedRange:  strings.TrimSpace(v.VendorProject + " " + v.Product),
			PatchedVersion: v.RequiredAction,
			PublishedAt:    v.DateAdded,
			IsKEV:          true,
			ReferenceURL:   "https://nvd.nist.gov/vuln/detail/" + v.CVEID,
			VirtualPatchID: findVirtualPatchForCVE(v.CVEID, prod, v.VulnName),
		})
		if len(out) >= 35 {
			break
		}
	}
	return out, nil
}

// fetchGitHubAdvisories mengambil daftar CVE kritikal & tinggi terbaru dari GitHub Security Advisories (Go, Composer/PHP, NPM)
func fetchGitHubAdvisories(ctx context.Context) ([]CVEAdvisory, error) {
	endpoints := []string{
		"https://api.github.com/advisories?type=reviewed&ecosystem=composer&severity=critical,high&per_page=15",
		"https://api.github.com/advisories?type=reviewed&ecosystem=go&severity=critical,high&per_page=15",
		"https://api.github.com/advisories?type=reviewed&ecosystem=npm&severity=critical&per_page=15",
	}

	out := make([]CVEAdvisory, 0, 40)
	var lastErr error
	for _, ep := range endpoints {
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, ep, nil)
		if err != nil {
			cancel()
			continue
		}
		req.Header.Set("User-Agent", "FikPanel-CVEIntel/0.7.6")
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := cveHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			cancel()
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		resp.Body.Close()
		cancel()
		if readErr != nil || resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		parsed, err := parseGitHubAdvisoriesJSON(body)
		if err == nil {
			out = append(out, parsed...)
		}
	}
	if len(out) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return out, nil
}

// parseGitHubAdvisoriesJSON mem-parsing respons REST GitHub Advisories menjadi slice CVEAdvisory
func parseGitHubAdvisoriesJSON(raw []byte) ([]CVEAdvisory, error) {
	var docs []struct {
		GHSAID      string `json:"ghsa_id"`
		CVEID       string `json:"cve_id"`
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Severity    string `json:"severity"`
		PublishedAt string `json:"published_at"`
		HTMLURL     string `json:"html_url"`
		CVSS        struct {
			Score float64 `json:"score"`
		} `json:"cvss"`
		Vulnerabilities []struct {
			Package struct {
				Ecosystem string `json:"ecosystem"`
				Name      string `json:"name"`
			} `json:"package"`
			VulnerableVersionRange string `json:"vulnerable_version_range"`
			FirstPatchedVersion    string `json:"first_patched_version"`
		} `json:"vulnerabilities"`
	}
	if err := json.Unmarshal(raw, &docs); err != nil {
		return nil, err
	}

	out := make([]CVEAdvisory, 0, len(docs))
	for _, d := range docs {
		id := strings.TrimSpace(d.CVEID)
		if id == "" {
			id = strings.TrimSpace(d.GHSAID)
		}
		if id == "" {
			continue
		}
		pkgName := ""
		eco := ""
		affRange := ""
		patched := ""
		if len(d.Vulnerabilities) > 0 {
			pkgName = d.Vulnerabilities[0].Package.Name
			eco = strings.ToLower(d.Vulnerabilities[0].Package.Ecosystem)
			affRange = strings.TrimSpace(pkgName + " " + d.Vulnerabilities[0].VulnerableVersionRange)
			patched = d.Vulnerabilities[0].FirstPatchedVersion
		}
		prod := detectCVEProduct(eco, pkgName, d.Summary)
		if prod == "" {
			switch eco {
			case "composer":
				prod = "php"
			case "go":
				prod = "go"
			case "npm":
				prod = "node"
			default:
				prod = "web"
			}
		}
		pubDate := d.PublishedAt
		if len(pubDate) >= 10 {
			pubDate = pubDate[:10]
		}
		desc := strings.TrimSpace(d.Description)
		if len(desc) > 260 {
			desc = desc[:257] + "..."
		}
		sev := strings.ToLower(strings.TrimSpace(d.Severity))
		if sev == "" {
			sev = "high"
		}
		out = append(out, CVEAdvisory{
			CVEID:          id,
			GHSAID:         d.GHSAID,
			Title:          d.Summary,
			Description:    desc,
			Severity:       sev,
			CVSSScore:      d.CVSS.Score,
			Source:         "github_advisory",
			Product:        prod,
			AffectedRange:  affRange,
			PatchedVersion: patched,
			PublishedAt:    pubDate,
			IsKEV:          false,
			ReferenceURL:   d.HTMLURL,
			VirtualPatchID: findVirtualPatchForCVE(id, prod, d.Summary),
		})
	}
	return out, nil
}
