package discovery

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// ProjectRuntimeInfo merangkum hasil deteksi runtime dan framework sebuah direktori atau port proxy
type ProjectRuntimeInfo struct {
	Kind       string `json:"kind"`        // "php", "node", "go", "python", "static", "proxy"
	Framework  string `json:"framework"`   // "Laravel", "Nuxt", "Next.js", "Hono", "Express", "NestJS", "Fastify", "CodeIgniter", "WordPress", "Go Service", "Python", "Static"
	PHPVersion string `json:"php_version"` // "8.3", "8.2", dst.
}

// DetectProjectRuntime menganalisis direktori root proyek atau target proxy untuk mengidentifikasi runtime dan framework secara presisi
func DetectProjectRuntime(rootDir string, proxyPass string, domain string) ProjectRuntimeInfo {
	info := ProjectRuntimeInfo{
		Kind:       "static",
		Framework:  "",
		PHPVersion: "8.3",
	}

	// 1. Jika direktori web root fisik tersedia, inspeksi manifest file proyek
	cleanRoot := strings.TrimSpace(rootDir)
	if cleanRoot != "" {
		if phpInfo, ok := inspectPHPDirectory(cleanRoot); ok {
			return phpInfo
		}
		if nodeInfo, ok := inspectNodeDirectory(cleanRoot); ok {
			return nodeInfo
		}
		if goInfo, ok := inspectGoDirectory(cleanRoot); ok {
			return goInfo
		}
		if pyInfo, ok := inspectPythonDirectory(cleanRoot); ok {
			return pyInfo
		}
	}

	// 2. Jika konfigurasi menggunakan proxy_pass (misal reverse proxy ke localhost/internal port)
	cleanProxy := strings.TrimSpace(proxyPass)
	if cleanProxy != "" {
		return inspectProxyRuntime(cleanProxy, domain, cleanRoot)
	}

	// 3. Fallback jika direktori root ada dan tidak memiliki manifest backend
	if cleanRoot != "" {
		if hasStaticAssets(cleanRoot) {
			return ProjectRuntimeInfo{Kind: "static", Framework: "Static", PHPVersion: "8.3"}
		}
	}

	return info
}

// inspectPHPDirectory memeriksa apakah sebuah direktori merupakan proyek PHP (Laravel, CodeIgniter, WordPress, dsb)
func inspectPHPDirectory(rootDir string) (ProjectRuntimeInfo, bool) {
	// Kumpulkan kandidat direktori (root saat ini, parent directory jika berakhiran public/dist)
	candidates := []string{rootDir}
	parent := filepath.Dir(rootDir)
	if parent != rootDir && parent != "/" && parent != "." {
		candidates = append(candidates, parent)
	}

	activePHPVers := detectAvailablePHPVersion()

	for _, dir := range candidates {
		// A. Deteksi Laravel (keberadaan berkas artisan atau dependency laravel/framework)
		artisanPath := filepath.Join(dir, "artisan")
		if st, err := os.Stat(artisanPath); err == nil && !st.IsDir() {
			return ProjectRuntimeInfo{Kind: "php", Framework: "Laravel", PHPVersion: activePHPVers}, true
		}

		composerPath := filepath.Join(dir, "composer.json")
		if data, err := os.ReadFile(composerPath); err == nil {
			content := strings.ToLower(string(data))
			if strings.Contains(content, "laravel/framework") {
				return ProjectRuntimeInfo{Kind: "php", Framework: "Laravel", PHPVersion: activePHPVers}, true
			}
			if strings.Contains(content, "codeigniter") {
				return ProjectRuntimeInfo{Kind: "php", Framework: "CodeIgniter", PHPVersion: activePHPVers}, true
			}
			if strings.Contains(content, "wordpress") {
				return ProjectRuntimeInfo{Kind: "php", Framework: "WordPress", PHPVersion: activePHPVers}, true
			}
			if strings.Contains(content, "symfony/") {
				return ProjectRuntimeInfo{Kind: "php", Framework: "Symfony", PHPVersion: activePHPVers}, true
			}
			return ProjectRuntimeInfo{Kind: "php", Framework: "PHP", PHPVersion: activePHPVers}, true
		}

		// B. Deteksi WordPress klasik
		if st, err := os.Stat(filepath.Join(dir, "wp-config.php")); err == nil && !st.IsDir() {
			return ProjectRuntimeInfo{Kind: "php", Framework: "WordPress", PHPVersion: activePHPVers}, true
		}
		if st, err := os.Stat(filepath.Join(dir, "wp-load.php")); err == nil && !st.IsDir() {
			return ProjectRuntimeInfo{Kind: "php", Framework: "WordPress", PHPVersion: activePHPVers}, true
		}

		// C. Deteksi CodeIgniter klasik
		if st, err := os.Stat(filepath.Join(dir, "system", "core", "CodeIgniter.php")); err == nil && !st.IsDir() {
			return ProjectRuntimeInfo{Kind: "php", Framework: "CodeIgniter", PHPVersion: activePHPVers}, true
		}

		// D. Deteksi berkas index.php
		if st, err := os.Stat(filepath.Join(dir, "index.php")); err == nil && !st.IsDir() {
			return ProjectRuntimeInfo{Kind: "php", Framework: "PHP", PHPVersion: activePHPVers}, true
		}
	}

	return ProjectRuntimeInfo{}, false
}

// inspectNodeDirectory memeriksa apakah direktori proyek menggunakan ekosistem Node.js (Next, Nuxt, Hono, dsb)
func inspectNodeDirectory(rootDir string) (ProjectRuntimeInfo, bool) {
	candidates := []string{rootDir}
	parent := filepath.Dir(rootDir)
	if parent != rootDir && parent != "/" && parent != "." {
		candidates = append(candidates, parent)
	}

	for _, dir := range candidates {
		pkgPath := filepath.Join(dir, "package.json")
		data, err := os.ReadFile(pkgPath)
		if err != nil {
			continue
		}

		fw := detectNodeFrameworkFromPackageJSON(data)
		return ProjectRuntimeInfo{Kind: "node", Framework: fw, PHPVersion: "8.3"}, true
	}

	return ProjectRuntimeInfo{}, false
}

// detectNodeFrameworkFromPackageJSON membaca berkas package.json untuk mendeteksi framework Node.js
func detectNodeFrameworkFromPackageJSON(data []byte) string {
	var pkg struct {
		Dependencies    map[string]interface{} `json:"dependencies"`
		DevDependencies map[string]interface{} `json:"devDependencies"`
		Scripts         map[string]interface{} `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		// Fallback pencarian string langsung jika parsing json parsial
		content := strings.ToLower(string(data))
		if strings.Contains(content, `"hono"`) || strings.Contains(content, `"@hono/`) {
			return "Hono"
		}
		if strings.Contains(content, `"nuxt"`) || strings.Contains(content, `"@nuxt/`) {
			return "Nuxt"
		}
		if strings.Contains(content, `"next"`) {
			return "Next.js"
		}
		if strings.Contains(content, `"express"`) {
			return "Express"
		}
		return "Node.js"
	}

	hasDep := func(name string) bool {
		if _, ok := pkg.Dependencies[name]; ok {
			return true
		}
		if _, ok := pkg.DevDependencies[name]; ok {
			return true
		}
		return false
	}

	if hasDep("hono") || hasDep("@hono/node-server") {
		return "Hono"
	}
	if hasDep("nuxt") || hasDep("nuxt3") || hasDep("@nuxt/kit") {
		return "Nuxt"
	}
	if hasDep("next") {
		return "Next.js"
	}
	if hasDep("@nestjs/core") || hasDep("nest") {
		return "NestJS"
	}
	if hasDep("fastify") {
		return "Fastify"
	}
	if hasDep("express") {
		return "Express"
	}
	if hasDep("@sveltejs/kit") || hasDep("svelte") {
		return "SvelteKit"
	}
	if hasDep("astro") {
		return "Astro"
	}

	return "Node.js"
}

// inspectGoDirectory memeriksa apakah direktori proyek menggunakan bahasa pemrograman Go
func inspectGoDirectory(rootDir string) (ProjectRuntimeInfo, bool) {
	candidates := []string{rootDir}
	parent := filepath.Dir(rootDir)
	if parent != rootDir && parent != "/" && parent != "." {
		candidates = append(candidates, parent)
	}

	for _, dir := range candidates {
		if st, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !st.IsDir() {
			return ProjectRuntimeInfo{Kind: "go", Framework: "Go Service", PHPVersion: "8.3"}, true
		}
	}
	return ProjectRuntimeInfo{}, false
}

// inspectPythonDirectory memeriksa apakah direktori proyek menggunakan bahasa pemrograman Python
func inspectPythonDirectory(rootDir string) (ProjectRuntimeInfo, bool) {
	candidates := []string{rootDir}
	parent := filepath.Dir(rootDir)
	if parent != rootDir && parent != "/" && parent != "." {
		candidates = append(candidates, parent)
	}

	for _, dir := range candidates {
		isPy := false
		fw := "Python"

		reqPath := filepath.Join(dir, "requirements.txt")
		if data, err := os.ReadFile(reqPath); err == nil {
			isPy = true
			content := strings.ToLower(string(data))
			if strings.Contains(content, "django") {
				fw = "Django"
			} else if strings.Contains(content, "fastapi") {
				fw = "FastAPI"
			} else if strings.Contains(content, "flask") {
				fw = "Flask"
			}
		}

		if !isPy {
			if st, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil && !st.IsDir() {
				isPy = true
			} else if st, err := os.Stat(filepath.Join(dir, "Pipfile")); err == nil && !st.IsDir() {
				isPy = true
			}
		}

		if isPy {
			return ProjectRuntimeInfo{Kind: "python", Framework: fw, PHPVersion: "8.3"}, true
		}
	}
	return ProjectRuntimeInfo{}, false
}

// inspectProxyRuntime memeriksa proses internal sistem yang me-listen target port localhost
func inspectProxyRuntime(proxyPass string, domain string, fallbackRootDir string) ProjectRuntimeInfo {
	// Nilai bawaan jika bukan proxy lokal atau tidak terdeteksi
	def := ProjectRuntimeInfo{Kind: "proxy", Framework: "", PHPVersion: "8.3"}

	u, err := url.Parse(proxyPass)
	if err != nil {
		return def
	}

	host := strings.ToLower(u.Hostname())
	portStr := u.Port()
	if portStr == "" {
		if u.Scheme == "https" {
			portStr = "443"
		} else {
			portStr = "80"
		}
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return def
	}

	// Hanya telusuri proses lokal jika host mengarah ke localhost/127.0.0.1
	isLocal := host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" || strings.HasPrefix(host, "127.")
	if !isLocal {
		return def
	}

	// 1. Periksa kandidat folder proyek berdasarkan domain
	for _, cand := range candidateDirsForDomain(domain) {
		if php, ok := inspectPHPDirectory(cand); ok {
			return php
		}
		if node, ok := inspectNodeDirectory(cand); ok {
			return node
		}
		if g, ok := inspectGoDirectory(cand); ok {
			return g
		}
		if py, ok := inspectPythonDirectory(cand); ok {
			return py
		}
	}

	// 2. Periksa PM2 process list di Linux jika tersedia
	if pm2Info, ok := inspectPM2Port(port); ok {
		return pm2Info
	}

	// 3. Periksa port listening via command 'ss -lptn' di Linux
	if sysInfo, ok := inspectListeningProcessPort(port); ok {
		return sysInfo
	}

	// 4. Jika nama domain memuat kata kunci seperti "go-api", "go.", atau "api" dengan biner di sistem
	lowerDom := strings.ToLower(domain)
	if strings.Contains(lowerDom, "go-") || strings.Contains(lowerDom, "go.") || strings.HasSuffix(lowerDom, ".go") {
		return ProjectRuntimeInfo{Kind: "go", Framework: "Go Service", PHPVersion: "8.3"}
	}

	return def
}

// inspectPM2Port membaca daftar aplikasi PM2 untuk menemukan aplikasi yang berjalan pada port tertentu
func inspectPM2Port(targetPort int) (ProjectRuntimeInfo, bool) {
	if runtime.GOOS != "linux" {
		return ProjectRuntimeInfo{}, false
	}

	out, err := exec.Command("pm2", "jlist").Output()
	if err != nil || len(out) == 0 {
		return ProjectRuntimeInfo{}, false
	}

	var apps []struct {
		Name   string `json:"name"`
		PM2Env struct {
			CWD  string                 `json:"pm_cwd"`
			Env  map[string]interface{} `json:"env"`
			Port interface{}            `json:"PORT"`
		} `json:"pm2_env"`
	}
	if err := json.Unmarshal(out, &apps); err != nil {
		return ProjectRuntimeInfo{}, false
	}

	for _, app := range apps {
		cwd := app.PM2Env.CWD
		// Periksa kecocokan port dari env
		match := false
		if app.PM2Env.Port != nil {
			if p, ok := app.PM2Env.Port.(float64); ok && int(p) == targetPort {
				match = true
			} else if pStr, ok := app.PM2Env.Port.(string); ok && pStr == strconv.Itoa(targetPort) {
				match = true
			}
		}
		if !match && app.PM2Env.Env != nil {
			if pVal, ok := app.PM2Env.Env["PORT"]; ok {
				if p, ok := pVal.(float64); ok && int(p) == targetPort {
					match = true
				} else if pStr, ok := pVal.(string); ok && pStr == strconv.Itoa(targetPort) {
					match = true
				}
			}
		}

		if match && cwd != "" {
			if node, ok := inspectNodeDirectory(cwd); ok {
				return node, true
			}
			return ProjectRuntimeInfo{Kind: "node", Framework: "Node.js", PHPVersion: "8.3"}, true
		}
	}

	return ProjectRuntimeInfo{}, false
}

// inspectListeningProcessPort memeriksa proses Linux yang sedang me-listen pada port tertentu
func inspectListeningProcessPort(port int) (ProjectRuntimeInfo, bool) {
	if runtime.GOOS != "linux" {
		return ProjectRuntimeInfo{}, false
	}

	// Gunakan perintah ss -lptn "sport = :<port>"
	out, err := exec.Command("ss", "-lptn", fmt.Sprintf("sport = :%d", port)).Output()
	if err != nil || len(out) == 0 {
		return ProjectRuntimeInfo{}, false
	}

	outStr := string(out)
	// Ekstrak nama proses dan PID: misal users:(("node",pid=1234,fd=19)) atau users:(("go-api",pid=5678,fd=3))
	pidRe := regexp.MustCompile(`users:\(\("([^"]+)",pid=([0-9]+)`)
	matches := pidRe.FindStringSubmatch(outStr)
	if len(matches) < 3 {
		return ProjectRuntimeInfo{}, false
	}

	procName := strings.ToLower(matches[1])
	pidStr := matches[2]

	// Baca working directory proses via /proc/<pid>/cwd
	cwdPath, err := os.Readlink(fmt.Sprintf("/proc/%s/cwd", pidStr))
	if err == nil && cwdPath != "" {
		if php, ok := inspectPHPDirectory(cwdPath); ok {
			return php, true
		}
		if node, ok := inspectNodeDirectory(cwdPath); ok {
			return node, true
		}
		if g, ok := inspectGoDirectory(cwdPath); ok {
			return g, true
		}
		if py, ok := inspectPythonDirectory(cwdPath); ok {
			return py, true
		}
	}

	// Deteksi berdasarkan nama biner proses
	if strings.Contains(procName, "node") {
		return ProjectRuntimeInfo{Kind: "node", Framework: "Node.js", PHPVersion: "8.3"}, true
	}
	if strings.Contains(procName, "python") || strings.Contains(procName, "gunicorn") || strings.Contains(procName, "uvicorn") {
		return ProjectRuntimeInfo{Kind: "python", Framework: "Python", PHPVersion: "8.3"}, true
	}
	if strings.Contains(procName, "go") || strings.Contains(procName, "api") || strings.Contains(procName, "server") || strings.Contains(procName, "app") {
		return ProjectRuntimeInfo{Kind: "go", Framework: "Go Service", PHPVersion: "8.3"}, true
	}

	return ProjectRuntimeInfo{}, false
}

// hasStaticAssets memeriksa apakah direktori memuat berkas html/css/js statis
func hasStaticAssets(rootDir string) bool {
	st, err := os.Stat(rootDir)
	if err != nil || !st.IsDir() {
		return false
	}
	if _, err := os.Stat(filepath.Join(rootDir, "index.html")); err == nil {
		return true
	}
	return true
}

// candidateDirsForDomain menghasilkan daftar kemungkinan path direktori proyek berdasarkan domain
func candidateDirsForDomain(domain string) []string {
	clean := strings.TrimSpace(domain)
	if clean == "" {
		return nil
	}
	parts := strings.Split(clean, ".")
	sub := parts[0]

	baseLocations := []string{
		"/home/fikfikk/projects",
		"/home/fikfikk/projects/web",
		"/home/fikfikk/projects/node",
		"/home/fikfikk/projects/go",
		"/var/www",
		"/var/www/html",
	}

	var results []string
	for _, base := range baseLocations {
		results = append(results,
			filepath.Join(base, clean),
			filepath.Join(base, sub),
			filepath.Join(base, sub, "public"),
			filepath.Join(base, sub, "dist"),
		)
	}

	return results
}

// detectAvailablePHPVersion mencari socket PHP-FPM aktif tertinggi di sistem
func detectAvailablePHPVersion() string {
	if runtime.GOOS != "linux" {
		return "8.3"
	}
	entries, err := os.ReadDir("/run/php")
	if err != nil {
		return "8.3"
	}

	// Prioritaskan versi PHP yang sering digunakan
	versions := []string{"8.4", "8.3", "8.2", "8.1", "8.0", "7.4"}
	for _, v := range versions {
		for _, e := range entries {
			if strings.Contains(e.Name(), "php"+v+"-fpm.sock") {
				return v
			}
		}
	}
	return "8.3"
}
