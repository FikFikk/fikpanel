package discovery

import (
	"context"
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
	"sync"
	"time"
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

// listeningProcess menyimpan informasi proses yang sedang me-listen suatu port di Linux
type listeningProcess struct {
	Port int
	PID  string
	Name string
	CWD  string
}

var (
	inspectorMu          sync.Mutex
	cachedListeningPorts map[int]listeningProcess
	cachedPM2Ports       map[int]ProjectRuntimeInfo
	cachedPHPVersion     string
	lastInspectorScan    time.Time
)

// InvalidateInspectorCache mengosongkan cache proses agar pemindaian berikutnya mengambil data segar dari sistem
func InvalidateInspectorCache() {
	inspectorMu.Lock()
	defer inspectorMu.Unlock()
	cachedListeningPorts = nil
	cachedPM2Ports = nil
	cachedPHPVersion = ""
	lastInspectorScan = time.Time{}
}

// ensureInspectorData memuat daftar port listening dan proses PM2 secara efisien satu kali untuk digunakan bersama
func ensureInspectorData(ctx context.Context) {
	inspectorMu.Lock()
	defer inspectorMu.Unlock()

	// Jika data cache masih baru (kurang dari 15 detik), hindari eksekusi perintah berulang
	if !lastInspectorScan.IsZero() && time.Since(lastInspectorScan) < 15*time.Second && cachedListeningPorts != nil {
		return
	}

	cachedListeningPorts = make(map[int]listeningProcess)
	cachedPM2Ports = make(map[int]ProjectRuntimeInfo)
	lastInspectorScan = time.Now()

	if runtime.GOOS != "linux" {
		return
	}

	// 1. Pindai seluruh port listening via satu perintah ss -lptn (maksimal 1 detik)
	ssCtx, cancelSS := context.WithTimeout(ctx, 1*time.Second)
	defer cancelSS()

	if out, err := exec.CommandContext(ssCtx, "ss", "-lptn").Output(); err == nil && len(out) > 0 {
		lines := strings.Split(string(out), "\n")
		portRe := regexp.MustCompile(`:([0-9]+)\s+`)
		userRe := regexp.MustCompile(`users:\(\("([^"]+)",pid=([0-9]+)`)

		for _, line := range lines {
			if !strings.Contains(line, "users:") {
				continue
			}
			pMatch := portRe.FindStringSubmatch(line)
			uMatch := userRe.FindStringSubmatch(line)
			if len(pMatch) >= 2 && len(uMatch) >= 3 {
				p, err := strconv.Atoi(pMatch[1])
				if err == nil && p > 0 {
					pid := uMatch[2]
					name := strings.ToLower(uMatch[1])
					cwd, _ := os.Readlink(fmt.Sprintf("/proc/%s/cwd", pid))
					cachedListeningPorts[p] = listeningProcess{
						Port: p,
						PID:  pid,
						Name: name,
						CWD:  cwd,
					}
				}
			}
		}
	}

	// 2. Pindai proses PM2 satu kali dengan batas waktu 1 detik
	pm2Ctx, cancelPM2 := context.WithTimeout(ctx, 1*time.Second)
	defer cancelPM2()

	pm2Bin := "pm2"
	if p, err := exec.LookPath("pm2"); err == nil {
		pm2Bin = p
	}

	cmd := exec.CommandContext(pm2Ctx, pm2Bin, "jlist")
	if out, err := cmd.Output(); err == nil && len(out) > 0 {
		outStr := string(out)
		fB := strings.Index(outStr, "[")
		lB := strings.LastIndex(outStr, "]")
		if fB != -1 && lB != -1 && lB > fB {
			var apps []struct {
				Name   string `json:"name"`
				PM2Env struct {
					CWD  string                 `json:"pm_cwd"`
					Env  map[string]interface{} `json:"env"`
					Port interface{}            `json:"PORT"`
				} `json:"pm2_env"`
			}
			if err := json.Unmarshal([]byte(outStr[fB:lB+1]), &apps); err == nil {
				for _, app := range apps {
					cwd := app.PM2Env.CWD
					var foundPort int

					if app.PM2Env.Port != nil {
						if p, ok := app.PM2Env.Port.(float64); ok {
							foundPort = int(p)
						} else if pStr, ok := app.PM2Env.Port.(string); ok {
							foundPort, _ = strconv.Atoi(pStr)
						}
					}
					if foundPort == 0 && app.PM2Env.Env != nil {
						if pVal, ok := app.PM2Env.Env["PORT"]; ok {
							if p, ok := pVal.(float64); ok {
								foundPort = int(p)
							} else if pStr, ok := pVal.(string); ok {
								foundPort, _ = strconv.Atoi(pStr)
							}
						}
					}

					if foundPort > 0 {
						rt := ProjectRuntimeInfo{Kind: "node", Framework: "Node.js", PHPVersion: "8.3"}
						if cwd != "" {
							if node, ok := inspectNodeDirectory(cwd); ok {
								rt = node
							}
						}
						cachedPM2Ports[foundPort] = rt
					}
				}
			}
		}
	}
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

	// Pastikan data port listening dan PM2 telah dimuat secara aman tanpa blokir
	ensureInspectorData(context.Background())

	inspectorMu.Lock()
	lp, hasLP := cachedListeningPorts[port]
	pm2Info, hasPM2 := cachedPM2Ports[port]
	inspectorMu.Unlock()

	// 1. Cek aplikasi PM2 yang cocok
	if hasPM2 {
		return pm2Info
	}

	// 2. Cek proses listening sistem (ss -lptn)
	if hasLP {
		if lp.CWD != "" {
			if php, ok := inspectPHPDirectory(lp.CWD); ok {
				return php
			}
			if node, ok := inspectNodeDirectory(lp.CWD); ok {
				return node
			}
			if g, ok := inspectGoDirectory(lp.CWD); ok {
				return g
			}
			if py, ok := inspectPythonDirectory(lp.CWD); ok {
				return py
			}
		}

		if strings.Contains(lp.Name, "node") {
			return ProjectRuntimeInfo{Kind: "node", Framework: "Node.js", PHPVersion: "8.3"}
		}
		if strings.Contains(lp.Name, "python") || strings.Contains(lp.Name, "gunicorn") || strings.Contains(lp.Name, "uvicorn") {
			return ProjectRuntimeInfo{Kind: "python", Framework: "Python", PHPVersion: "8.3"}
		}
		if strings.Contains(lp.Name, "go") || strings.Contains(lp.Name, "api") || strings.Contains(lp.Name, "server") || strings.Contains(lp.Name, "app") {
			return ProjectRuntimeInfo{Kind: "go", Framework: "Go Service", PHPVersion: "8.3"}
		}
	}

	// 3. Fallback periksa kandidat folder proyek berdasarkan domain jika cwd proses tidak ditemukan
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

	// 4. Jika nama domain memuat kata kunci seperti "go-api", "go.", atau "api" dengan biner di sistem
	lowerDom := strings.ToLower(domain)
	if strings.Contains(lowerDom, "go-") || strings.Contains(lowerDom, "go.") || strings.HasSuffix(lowerDom, ".go") {
		return ProjectRuntimeInfo{Kind: "go", Framework: "Go Service", PHPVersion: "8.3"}
	}

	return def
}

// inspectPM2Port membaca daftar aplikasi PM2 untuk menemukan aplikasi yang berjalan pada port tertentu
func inspectPM2Port(targetPort int) (ProjectRuntimeInfo, bool) {
	ensureInspectorData(context.Background())
	inspectorMu.Lock()
	defer inspectorMu.Unlock()
	info, ok := cachedPM2Ports[targetPort]
	return info, ok
}

// inspectListeningProcessPort memeriksa proses Linux yang sedang me-listen pada port tertentu
func inspectListeningProcessPort(port int) (ProjectRuntimeInfo, bool) {
	ensureInspectorData(context.Background())
	inspectorMu.Lock()
	lp, ok := cachedListeningPorts[port]
	inspectorMu.Unlock()
	if !ok {
		return ProjectRuntimeInfo{}, false
	}

	if lp.CWD != "" {
		if php, ok := inspectPHPDirectory(lp.CWD); ok {
			return php, true
		}
		if node, ok := inspectNodeDirectory(lp.CWD); ok {
			return node, true
		}
		if g, ok := inspectGoDirectory(lp.CWD); ok {
			return g, true
		}
		if py, ok := inspectPythonDirectory(lp.CWD); ok {
			return py, true
		}
	}

	if strings.Contains(lp.Name, "node") {
		return ProjectRuntimeInfo{Kind: "node", Framework: "Node.js", PHPVersion: "8.3"}, true
	}
	if strings.Contains(lp.Name, "python") || strings.Contains(lp.Name, "gunicorn") || strings.Contains(lp.Name, "uvicorn") {
		return ProjectRuntimeInfo{Kind: "python", Framework: "Python", PHPVersion: "8.3"}, true
	}
	if strings.Contains(lp.Name, "go") || strings.Contains(lp.Name, "api") || strings.Contains(lp.Name, "server") || strings.Contains(lp.Name, "app") {
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
		"/var/www",
		"/var/www/html",
	}

	// Deteksi direktori home pengguna Linux secara dinamis
	if entries, err := os.ReadDir("/home"); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				uDir := filepath.Join("/home", e.Name())
				baseLocations = append(baseLocations,
					filepath.Join(uDir, "projects"),
					filepath.Join(uDir, "projects/web"),
					filepath.Join(uDir, "projects/node"),
					filepath.Join(uDir, "projects/go"),
				)
			}
		}
	}

	var results []string
	for _, base := range baseLocations {
		// Validasi direktori dasar sebelum memeriksa anak jalurnya
		if st, err := os.Stat(base); err == nil && st.IsDir() {
			results = append(results,
				filepath.Join(base, clean),
				filepath.Join(base, sub),
				filepath.Join(base, sub, "public"),
				filepath.Join(base, sub, "dist"),
			)
		}
	}

	return results
}

// detectAvailablePHPVersion mencari socket PHP-FPM aktif tertinggi di sistem dengan cache memori
func detectAvailablePHPVersion() string {
	inspectorMu.Lock()
	if cachedPHPVersion != "" {
		v := cachedPHPVersion
		inspectorMu.Unlock()
		return v
	}
	inspectorMu.Unlock()

	v := "8.3"
	if runtime.GOOS == "linux" {
		if entries, err := os.ReadDir("/run/php"); err == nil {
			versions := []string{"8.4", "8.3", "8.2", "8.1", "8.0", "7.4"}
			found := false
			for _, ver := range versions {
				for _, e := range entries {
					if strings.Contains(e.Name(), "php"+ver+"-fpm.sock") {
						v = ver
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
	}

	inspectorMu.Lock()
	cachedPHPVersion = v
	inspectorMu.Unlock()
	return v
}
