package server

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/store"
)

// fullRuntimeExecEnv menyusun variabel environment aman untuk build & eksekusi PHP, Node, Go, dan Python
func fullRuntimeExecEnv() []string {
	return append(os.Environ(),
		"PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/root/.local/bin:/root/.bun/bin",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=accept-new",
		"COMPOSER_ALLOW_SUPERUSER=1",
		"COMPOSER_MEMORY_LIMIT=-1",
		"NODE_OPTIONS=--max-old-space-size=768",
		"CI=true",
		"HOME=/root",
	)
}

// siteSystemdSlug menghasilkan pengenal aman huruf kecil dan tanda hubung untuk nama unit systemd
func siteSystemdSlug(domain string) string {
	slug := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return '-'
	}, strings.TrimSpace(domain))
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "site"
	}
	if len(slug) > 48 {
		slug = slug[:48]
	}
	return slug
}

// siteAppUnitName mengembalikan nama unit systemd terkelola untuk aplikasi Go/Python/Node
func siteAppUnitName(domain string) string {
	return "fikpanel-app-" + siteSystemdSlug(domain) + ".service"
}

// siteQueueUnitName mengembalikan nama unit systemd terkelola untuk Laravel Queue Worker
func siteQueueUnitName(domain string) string {
	return "fikpanel-queue-" + siteSystemdSlug(domain) + ".service"
}

// extractSitePort mengambil angka port lokal dari URL upstream situs
func extractSitePort(upstream, fallback string) string {
	if u, err := url.Parse(strings.TrimSpace(upstream)); err == nil && u.Port() != "" {
		return u.Port()
	}
	return fallback
}

// allocateFreeAppPort mencari port TCP lokal yang belum dipakai situs lain di DB maupun proses aktif di host
func (s *Server) allocateFreeAppPort(ctx context.Context, kind string) int {
	startPort, endPort := 3100, 3999
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "go":
		startPort, endPort = 8100, 8499
	case "python":
		startPort, endPort = 8500, 8999
	case "node", "proxy":
		startPort, endPort = 3100, 3999
	}

	usedPorts := map[int]bool{}
	if list, err := s.store.ListSites(ctx); err == nil {
		for _, item := range list {
			if pStr := extractSitePort(item.Upstream, ""); pStr != "" {
				if p, err := strconv.Atoi(pStr); err == nil {
					usedPorts[p] = true
				}
			}
		}
	}

	for p := startPort; p <= endPort; p++ {
		if usedPorts[p] {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			_ = ln.Close()
			return p
		}
	}
	return startPort
}

// isUpstreamPortTaken memeriksa apakah port pada URL upstream sudah digunakan situs lain
func (s *Server) isUpstreamPortTaken(ctx context.Context, rawUpstream string, excludeSiteID int64) bool {
	targetPort := extractSitePort(rawUpstream, "")
	if targetPort == "" {
		return false
	}
	list, err := s.store.ListSites(ctx)
	if err != nil {
		return false
	}
	for _, item := range list {
		if item.ID == excludeSiteID {
			continue
		}
		if extractSitePort(item.Upstream, "") == targetPort {
			return true
		}
	}
	return false
}

// handleNextFreeAppPort mengembalikan alamat upstream dengan port bebas untuk form pembuatan situs
func (s *Server) handleNextFreeAppPort(w http.ResponseWriter, r *http.Request) {
	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
	if kind == "" {
		kind = "node"
	}
	port := s.allocateFreeAppPort(r.Context(), kind)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"kind":     kind,
		"port":     port,
		"upstream": fmt.Sprintf("http://127.0.0.1:%d", port),
	})
}

// projectInspection merangkum deteksi mendalam struktur proyek lokal untuk auto-deploy multi-runtime
type projectInspection struct {
	Kind            string
	Framework       string
	PackageManager  string // npm | pnpm | yarn | bun
	IsLaravel       bool
	IsFilament      bool
	IsWordPress     bool
	IsStaticSPA     bool
	StaticBuildDir  string // dist | build | out
	HasBuildScript  bool
	HasStartScript  bool
	MainEntry       string
	HasPrisma       bool
	HasDrizzle      bool
	GoEntry         string // . | ./cmd/xxx
	PythonFramework string // fastapi | django | flask | generic
	PythonASGI      string // main:app | app.main:app
	PythonWSGI      string // project.wsgi:application | app:app
	HasDockerfile   bool
}

// inspectPackageJSON menganalisis isi package.json untuk membedakan Node SSR/API dengan Static SPA
func inspectPackageJSON(raw []byte) (framework string, isStaticSPA bool, staticDir string, hasBuild bool, hasStart bool, mainEntry string, hasPrisma bool, hasDrizzle bool) {
	var pkg struct {
		Main            string            `json:"main"`
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(raw, &pkg) != nil {
		return "Node.js", false, "", false, false, "", false, false
	}

	allDeps := map[string]bool{}
	for k := range pkg.Dependencies {
		allDeps[strings.ToLower(k)] = true
	}
	for k := range pkg.DevDependencies {
		allDeps[strings.ToLower(k)] = true
	}

	_, hasBuild = pkg.Scripts["build"]
	_, hasStart = pkg.Scripts["start"]
	mainEntry = strings.TrimSpace(pkg.Main)

	hasPrisma = allDeps["prisma"] || allDeps["@prisma/client"]
	hasDrizzle = allDeps["drizzle-orm"] || allDeps["drizzle-kit"]

	// Deteksi framework server-side / SSR terlebih dahulu
	switch {
	case allDeps["next"]:
		return "Next.js", false, "", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
	case allDeps["nuxt"] || allDeps["nuxt3"]:
		return "Nuxt", false, "", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
	case allDeps["@sveltejs/kit"]:
		return "SvelteKit", false, "", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
	case allDeps["@nestjs/core"]:
		return "NestJS", false, "", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
	case allDeps["@adonisjs/core"]:
		return "AdonisJS", false, "", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
	case allDeps["express"]:
		return "Express", false, "", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
	case allDeps["fastify"]:
		return "Fastify", false, "", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
	case allDeps["hono"]:
		return "Hono", false, "", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
	case allDeps["koa"]:
		return "Koa", false, "", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
	case allDeps["@remix-run/node"] || allDeps["react-router-serve"]:
		return "Remix", false, "", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
	}

	// Cek apakah proyek memiliki server backend khusus meskipun bukan framework populer di atas
	hasServerDep := allDeps["socket.io"] || allDeps["ws"] || allDeps["mysql2"] || allDeps["pg"] || allDeps["mongoose"] || allDeps["sequelize"] || allDeps["typeorm"] || allDeps["@strapi/strapi"]
	if !hasServerDep {
		if allDeps["vite"] {
			fw := "Vite SPA"
			switch {
			case allDeps["svelte"]:
				fw = "Svelte SPA"
			case allDeps["vue"]:
				fw = "Vue SPA"
			case allDeps["react"]:
				fw = "React SPA"
			}
			return fw, true, "dist", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
		}
		if allDeps["react-scripts"] {
			return "React SPA", true, "build", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
		}
		if allDeps["@vue/cli-service"] {
			return "Vue SPA", true, "dist", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
		}
		if allDeps["@angular/core"] && !allDeps["@angular/ssr"] {
			return "Angular SPA", true, "dist", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
		}
		if allDeps["astro"] && !allDeps["@astrojs/node"] {
			return "Astro Static", true, "dist", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
		}
	}

	return "Node.js", false, "", hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle
}

// inspectLocalProject memindai direktori proyek lokal untuk menentukan strategi build dan service manager
func inspectLocalProject(projectDir string) projectInspection {
	insp := projectInspection{
		PackageManager: "npm",
		GoEntry:        ".",
	}
	if strings.TrimSpace(projectDir) == "" {
		return insp
	}

	exists := func(rel ...string) bool {
		parts := append([]string{projectDir}, rel...)
		_, err := os.Stat(filepath.Join(parts...))
		return err == nil
	}

	if exists("Dockerfile") || exists("docker-compose.yml") || exists("compose.yml") {
		insp.HasDockerfile = true
	}

	// Deteksi package manager JS/TS dari berkas lock
	switch {
	case exists("pnpm-lock.yaml"):
		insp.PackageManager = "pnpm"
	case exists("yarn.lock"):
		insp.PackageManager = "yarn"
	case exists("bun.lockb") || exists("bun.lock"):
		insp.PackageManager = "bun"
	}

	if exists("prisma", "schema.prisma") {
		insp.HasPrisma = true
	}
	if exists("drizzle.config.ts") || exists("drizzle.config.js") {
		insp.HasDrizzle = true
	}

	// 1. PHP / Laravel / Filament / WordPress
	if exists("artisan") || exists("composer.json") {
		insp.Kind = "php"
		if exists("artisan") {
			insp.IsLaravel = true
			insp.Framework = "Laravel"
		}
		if rawComp, err := os.ReadFile(filepath.Join(projectDir, "composer.json")); err == nil {
			lower := strings.ToLower(string(rawComp))
			if strings.Contains(lower, "filament/filament") {
				insp.IsFilament = true
				insp.IsLaravel = true
				insp.Framework = "Filament"
			} else if strings.Contains(lower, "laravel/framework") {
				insp.IsLaravel = true
				insp.Framework = "Laravel"
			}
		}
		return insp
	}
	if exists("wp-includes") || exists("wp-config.php") || exists("wp-config-sample.php") {
		insp.Kind = "php"
		insp.IsWordPress = true
		insp.Framework = "WordPress"
		return insp
	}

	// 2. Golang
	if exists("go.mod") {
		insp.Kind = "go"
		insp.Framework = "Go"
		if rawMod, err := os.ReadFile(filepath.Join(projectDir, "go.mod")); err == nil {
			lower := strings.ToLower(string(rawMod))
			switch {
			case strings.Contains(lower, "github.com/gin-gonic/gin"):
				insp.Framework = "Gin"
			case strings.Contains(lower, "github.com/gofiber/fiber"):
				insp.Framework = "Fiber"
			case strings.Contains(lower, "github.com/labstack/echo"):
				insp.Framework = "Echo"
			}
		}
		// Periksa apakah main.go berada di root atau di dalam cmd/<app>
		if !exists("main.go") {
			if entries, err := os.ReadDir(filepath.Join(projectDir, "cmd")); err == nil {
				for _, e := range entries {
					if e.IsDir() && exists("cmd", e.Name(), "main.go") {
						insp.GoEntry = "./cmd/" + e.Name()
						break
					}
				}
			}
		}
		return insp
	}

	// 3. Python (FastAPI / Django / Flask)
	if exists("requirements.txt") || exists("pyproject.toml") || exists("manage.py") {
		insp.Kind = "python"
		insp.Framework = "Python"
		reqText := ""
		if raw, err := os.ReadFile(filepath.Join(projectDir, "requirements.txt")); err == nil {
			reqText += strings.ToLower(string(raw))
		}
		if raw, err := os.ReadFile(filepath.Join(projectDir, "pyproject.toml")); err == nil {
			reqText += "\n" + strings.ToLower(string(raw))
		}

		switch {
		case exists("manage.py") || strings.Contains(reqText, "django"):
			insp.PythonFramework = "django"
			insp.Framework = "Django"
			insp.PythonWSGI = detectDjangoWSGIModule(projectDir)
		case strings.Contains(reqText, "fastapi") || strings.Contains(reqText, "uvicorn") || strings.Contains(reqText, "starlette"):
			insp.PythonFramework = "fastapi"
			insp.Framework = "FastAPI"
			insp.PythonASGI = detectPythonASGIModule(projectDir)
		case strings.Contains(reqText, "flask") || strings.Contains(reqText, "gunicorn"):
			insp.PythonFramework = "flask"
			insp.Framework = "Flask"
			insp.PythonWSGI = detectFlaskWSGIModule(projectDir)
		default:
			insp.PythonFramework = "generic"
			insp.PythonASGI = detectPythonASGIModule(projectDir)
		}
		return insp
	}

	// 4. Node.js atau Static SPA
	if rawPkg, err := os.ReadFile(filepath.Join(projectDir, "package.json")); err == nil {
		fw, isSPA, staticDir, hasBuild, hasStart, mainEntry, hasPrisma, hasDrizzle := inspectPackageJSON(rawPkg)
		insp.Framework = fw
		insp.IsStaticSPA = isSPA
		insp.StaticBuildDir = staticDir
		insp.HasBuildScript = hasBuild
		insp.HasStartScript = hasStart
		insp.MainEntry = mainEntry
		insp.HasPrisma = insp.HasPrisma || hasPrisma
		insp.HasDrizzle = insp.HasDrizzle || hasDrizzle
		if isSPA {
			insp.Kind = "static"
		} else {
			insp.Kind = "node"
		}
		return insp
	}

	insp.Kind = "static"
	return insp
}

// detectDjangoWSGIModule mencari nama modul wsgi.py di proyek Django
func detectDjangoWSGIModule(projectDir string) string {
	entries, err := os.ReadDir(projectDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "venv" {
				continue
			}
			if _, statErr := os.Stat(filepath.Join(projectDir, e.Name(), "wsgi.py")); statErr == nil {
				return e.Name() + ".wsgi:application"
			}
		}
	}
	return "config.wsgi:application"
}

// detectPythonASGIModule mencari target modul FastAPI/Uvicorn pada struktur folder umum
func detectPythonASGIModule(projectDir string) string {
	candidates := [][2]string{
		{"main.py", "main:app"},
		{"app/main.py", "app.main:app"},
		{"src/main.py", "src.main:app"},
		{"app.py", "app:app"},
		{"server.py", "server:app"},
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(c[0]))); err == nil {
			return c[1]
		}
	}
	return "main:app"
}

// detectFlaskWSGIModule mencari target modul Flask/Gunicorn pada struktur folder umum
func detectFlaskWSGIModule(projectDir string) string {
	candidates := [][2]string{
		{"wsgi.py", "wsgi:app"},
		{"app.py", "app:app"},
		{"main.py", "main:app"},
		{"run.py", "run:app"},
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(projectDir, c[0])); err == nil {
			return c[1]
		}
	}
	return "app:app"
}

// nodeInstallBuildScript menyusun perintah install & build yang sesuai dengan package manager proyek
func nodeInstallBuildScript(pm string, runBuild bool) string {
	var installCmd, buildCmd string
	switch pm {
	case "pnpm":
		installCmd = "(command -v pnpm >/dev/null 2>&1 && pnpm install --frozen-lockfile || pnpm install || npm install --no-audit --no-fund)"
		buildCmd = "(command -v pnpm >/dev/null 2>&1 && pnpm run build || npm run build)"
	case "yarn":
		installCmd = "(command -v yarn >/dev/null 2>&1 && yarn install --frozen-lockfile || yarn install || npm install --no-audit --no-fund)"
		buildCmd = "(command -v yarn >/dev/null 2>&1 && yarn build || npm run build)"
	case "bun":
		installCmd = "(command -v bun >/dev/null 2>&1 && bun install || npm install --no-audit --no-fund)"
		buildCmd = "(command -v bun >/dev/null 2>&1 && bun run build || npm run build)"
	default:
		installCmd = "if [ -f package-lock.json ]; then (npm ci --no-audit --no-fund || npm install --no-audit --no-fund); else npm install --no-audit --no-fund; fi"
		buildCmd = "npm run build"
	}
	if runBuild {
		return installCmd + " && " + buildCmd
	}
	return installCmd
}

// ensureNodePM2Service membuat ecosystem.config.cjs otomatis (jika belum ada) dan menjalankan/mereload proses PM2 di direktori proyek
func ensureNodePM2Service(ctx context.Context, site *store.Site, projectDir, appPort string, insp projectInspection, execEnv []string, appendLog func(string, string)) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if appPort == "" {
		appPort = extractSitePort(site.Upstream, "3000")
	}

	ecoFile := ""
	for _, cand := range []string{"ecosystem.config.cjs", "ecosystem.config.js"} {
		if _, err := os.Stat(filepath.Join(projectDir, cand)); err == nil {
			ecoFile = cand
			break
		}
	}

	// Buat ecosystem.config.cjs otomatis sesuai framework yang terdeteksi
	if ecoFile == "" {
		ecoFile = "ecosystem.config.cjs"
		script := "npm"
		args := "run start"
		switch insp.Framework {
		case "Next.js":
			script = "npm"
			args = fmt.Sprintf("run start -- -p %s -H 127.0.0.1", appPort)
		case "Nuxt":
			if _, err := os.Stat(filepath.Join(projectDir, ".output", "server", "index.mjs")); err == nil {
				script = ".output/server/index.mjs"
				args = ""
			}
		case "SvelteKit":
			if _, err := os.Stat(filepath.Join(projectDir, "build", "index.js")); err == nil {
				script = "build/index.js"
				args = ""
			}
		default:
			if !insp.HasStartScript {
				for _, entry := range []string{insp.MainEntry, "dist/main.js", "dist/index.js", "dist/server.js", "server.js", "index.js", "app.js"} {
					if entry == "" {
						continue
					}
					if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(entry))); err == nil {
						script = entry
						args = ""
						break
					}
				}
			}
		}

		argsLine := ""
		if args != "" {
			argsLine = fmt.Sprintf("    args: %q,\n", args)
		}
		ecoContent := fmt.Sprintf(`// Dihasilkan otomatis oleh FikPanel untuk %s
module.exports = {
  apps: [{
    name: %q,
    cwd: %q,
    script: %q,
%s    instances: 1,
    exec_mode: 'fork',
    max_memory_restart: '512M',
    env: {
      NODE_ENV: 'production',
      HOST: '127.0.0.1',
      PORT: %q,
      NITRO_HOST: '127.0.0.1',
      NITRO_PORT: %q
    }
  }]
};
`, site.Domain, site.Domain, projectDir, script, argsLine, appPort, appPort)
		if err := os.WriteFile(filepath.Join(projectDir, ecoFile), []byte(ecoContent), 0o644); err != nil {
			return fmt.Errorf("gagal menulis %s: %w", ecoFile, err)
		}
		appendLog("auto-generate "+ecoFile, fmt.Sprintf("PM2 app=%s script=%s %s port=%s", site.Domain, script, args, appPort))
	}

	// Pastikan PM2 tersedia lalu startOrReload dengan working directory proyek yang benar
	pm2Cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("if command -v pm2 >/dev/null 2>&1; then (pm2 startOrReload %q --update-env && pm2 save); else echo 'PM2 belum terpasang di server — pasang Node.js/PM2 di App Store'; exit 1; fi", ecoFile))
	pm2Cmd.Dir = projectDir
	pm2Cmd.Env = execEnv
	out, err := pm2Cmd.CombinedOutput()
	appendLog("pm2 startOrReload "+ecoFile, string(out))
	return err
}

// ensureGoSystemdService mengompilasi biner Go dan memasang/merestart unit systemd terkelola
func ensureGoSystemdService(ctx context.Context, site *store.Site, projectDir, appPort string, insp projectInspection, execEnv []string, appendLog func(string, string)) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if appPort == "" {
		appPort = extractSitePort(site.Upstream, "8100")
	}
	entry := insp.GoEntry
	if entry == "" {
		entry = "."
	}
	binPath := filepath.Join(projectDir, ".fikpanel-bin")
	buildCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("go build -o %q %s", binPath, entry))
	buildCmd.Dir = projectDir
	buildCmd.Env = execEnv
	buildOut, buildErr := buildCmd.CombinedOutput()
	appendLog("go build "+entry, string(buildOut))
	if buildErr != nil {
		return fmt.Errorf("go build gagal: %w", buildErr)
	}
	_ = os.Chmod(binPath, 0o755)

	unitName := siteAppUnitName(site.Domain)
	unitPath := filepath.Join("/etc/systemd/system", unitName)
	unitContent := fmt.Sprintf(`[Unit]
Description=FikPanel Go App (%s)
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
EnvironmentFile=-%s/.env
Environment=PORT=%s
Environment=ADDR=127.0.0.1:%s
Environment=GIN_MODE=release
ExecStart=%s
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, site.Domain, projectDir, projectDir, appPort, appPort, binPath)

	if err := os.WriteFile(unitPath, []byte(unitContent), 0o644); err != nil {
		return fmt.Errorf("gagal menulis unit systemd %s: %w", unitName, err)
	}

	sysCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("systemctl daemon-reload && systemctl enable %q && systemctl restart %q", unitName, unitName))
	sysCmd.Env = execEnv
	sysOut, sysErr := sysCmd.CombinedOutput()
	appendLog("systemctl restart "+unitName, string(sysOut))
	return sysErr
}

// ensurePythonSystemdService menyiapkan .venv terisolasi (aman PEP 668), install dependensi, dan menjalankan Uvicorn/Gunicorn via systemd
func ensurePythonSystemdService(ctx context.Context, site *store.Site, projectDir, appPort string, insp projectInspection, execEnv []string, appendLog func(string, string)) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if appPort == "" {
		appPort = extractSitePort(site.Upstream, "8500")
	}

	venvDir := filepath.Join(projectDir, ".venv")
	if _, err := os.Stat(filepath.Join(venvDir, "bin", "python")); err != nil {
		venvCmd := exec.CommandContext(ctx, "python3", "-m", "venv", venvDir)
		venvCmd.Dir = projectDir
		venvCmd.Env = execEnv
		vOut, vErr := venvCmd.CombinedOutput()
		appendLog("python3 -m venv .venv", string(vOut))
		if vErr != nil {
			return fmt.Errorf("gagal membuat virtualenv Python (.venv): %w", vErr)
		}
	}

	pipBin := filepath.Join(venvDir, "bin", "pip")
	pyBin := filepath.Join(venvDir, "bin", "python")

	if _, err := os.Stat(filepath.Join(projectDir, "requirements.txt")); err == nil {
		pipCmd := exec.CommandContext(ctx, pipBin, "install", "--no-input", "-r", "requirements.txt")
		pipCmd.Dir = projectDir
		pipCmd.Env = execEnv
		pOut, pErr := pipCmd.CombinedOutput()
		appendLog(".venv/bin/pip install -r requirements.txt", string(pOut))
		if pErr != nil {
			return fmt.Errorf("instalasi requirements.txt gagal: %w", pErr)
		}
	} else if _, err := os.Stat(filepath.Join(projectDir, "pyproject.toml")); err == nil {
		pipCmd := exec.CommandContext(ctx, pipBin, "install", "--no-input", ".")
		pipCmd.Dir = projectDir
		pipCmd.Env = execEnv
		pOut, _ := pipCmd.CombinedOutput()
		appendLog(".venv/bin/pip install .", string(pOut))
	}

	// Tentukan perintah ExecStart sesuai framework Python
	var execStart string
	switch insp.PythonFramework {
	case "django":
		migCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%q manage.py migrate --noinput && (%q manage.py collectstatic --noinput || true)", pyBin, pyBin))
		migCmd.Dir = projectDir
		migCmd.Env = execEnv
		mOut, _ := migCmd.CombinedOutput()
		appendLog("django migrate & collectstatic", string(mOut))

		if _, err := os.Stat(filepath.Join(venvDir, "bin", "gunicorn")); err != nil {
			_, _ = exec.CommandContext(ctx, pipBin, "install", "--no-input", "gunicorn").CombinedOutput()
		}
		execStart = fmt.Sprintf("%s/bin/gunicorn %s --bind 127.0.0.1:%s --workers 2", venvDir, insp.PythonWSGI, appPort)
	case "flask":
		if _, err := os.Stat(filepath.Join(venvDir, "bin", "gunicorn")); err != nil {
			_, _ = exec.CommandContext(ctx, pipBin, "install", "--no-input", "gunicorn").CombinedOutput()
		}
		execStart = fmt.Sprintf("%s/bin/gunicorn %s --bind 127.0.0.1:%s --workers 2", venvDir, insp.PythonWSGI, appPort)
	default:
		if _, err := os.Stat(filepath.Join(venvDir, "bin", "uvicorn")); err != nil {
			_, _ = exec.CommandContext(ctx, pipBin, "install", "--no-input", "uvicorn").CombinedOutput()
		}
		asgiTarget := insp.PythonASGI
		if asgiTarget == "" {
			asgiTarget = "main:app"
		}
		execStart = fmt.Sprintf("%s/bin/uvicorn %s --host 127.0.0.1 --port %s", venvDir, asgiTarget, appPort)
	}

	unitName := siteAppUnitName(site.Domain)
	unitPath := filepath.Join("/etc/systemd/system", unitName)
	unitContent := fmt.Sprintf(`[Unit]
Description=FikPanel Python App (%s)
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
EnvironmentFile=-%s/.env
Environment=PORT=%s
Environment=PYTHONUNBUFFERED=1
ExecStart=%s
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, site.Domain, projectDir, projectDir, appPort, execStart)

	if err := os.WriteFile(unitPath, []byte(unitContent), 0o644); err != nil {
		return fmt.Errorf("gagal menulis unit systemd %s: %w", unitName, err)
	}

	sysCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("systemctl daemon-reload && systemctl enable %q && systemctl restart %q", unitName, unitName))
	sysCmd.Env = execEnv
	sysOut, sysErr := sysCmd.CombinedOutput()
	appendLog("systemctl restart "+unitName, string(sysOut))
	return sysErr
}

// buildAndStartNonPHPProject menjalankan pipeline zero-config untuk Static SPA, Node.js, Go, dan Python
func (s *Server) buildAndStartNonPHPProject(ctx context.Context, site *store.Site, projectDir string, execEnv []string, appendLog func(string, string)) []string {
	var warnings []string
	if runtime.GOOS == "windows" {
		return warnings
	}
	insp := inspectLocalProject(projectDir)
	if insp.Framework != "" && site.Framework != insp.Framework {
		site.Framework = insp.Framework
		_ = s.store.UpdateSite(ctx, site)
	}

	// Kasus 1: Static SPA (Vite / React / Vue / Svelte / Astro static) — build lalu layani langsung oleh Nginx tanpa PM2
	if insp.IsStaticSPA || (site.Kind == "static" && insp.HasBuildScript) {
		script := nodeInstallBuildScript(insp.PackageManager, insp.HasBuildScript)
		cmd := exec.CommandContext(ctx, "sh", "-c", script)
		cmd.Dir = projectDir
		cmd.Env = execEnv
		out, err := cmd.CombinedOutput()
		appendLog("SPA build ("+insp.PackageManager+")", string(out))
		if err != nil {
			warnings = append(warnings, "Build frontend SPA gagal; periksa log deployment")
		}

		// Arahkan Web Root Nginx ke direktori output build (dist / build / out)
		outDir := ""
		for _, cand := range []string{insp.StaticBuildDir, "dist", "build", "out"} {
			if cand == "" {
				continue
			}
			full := filepath.Join(projectDir, cand)
			if st, statErr := os.Stat(full); statErr == nil && st.IsDir() {
				outDir = filepath.ToSlash(full)
				break
			}
		}
		changed := false
		if site.Kind != "static" {
			site.Kind = "static"
			site.Upstream = ""
			changed = true
		}
		if outDir != "" && site.RootPath != outDir {
			site.RootPath = outDir
			changed = true
		}
		if changed {
			_ = s.store.UpdateSite(ctx, site)
			_ = s.sites.Apply(ctx, site, "")
			appendLog("nginx spa routing", fmt.Sprintf("Dialihkan otomatis ke Nginx Static SPA (%s) dengan try_files /index.html", site.RootPath))
		}
		return warnings
	}

	// Kasus 2: Node.js SSR / Backend API
	if site.Kind == "node" || insp.Kind == "node" {
		appPort := extractSitePort(site.Upstream, "3000")
		script := nodeInstallBuildScript(insp.PackageManager, insp.HasBuildScript)
		cmd := exec.CommandContext(ctx, "sh", "-c", script)
		cmd.Dir = projectDir
		cmd.Env = execEnv
		out, err := cmd.CombinedOutput()
		appendLog("Node build ("+insp.PackageManager+")", string(out))
		if err != nil {
			warnings = append(warnings, "Build Node.js gagal; periksa log deployment")
		}

		// Jalankan migrasi ORM otomatis jika Prisma / Drizzle terdeteksi
		if insp.HasPrisma {
			pCmd := exec.CommandContext(ctx, "sh", "-c", "npx prisma generate && (npx prisma migrate deploy || true)")
			pCmd.Dir = projectDir
			pCmd.Env = execEnv
			pOut, _ := pCmd.CombinedOutput()
			appendLog("prisma generate & migrate", string(pOut))
		} else if insp.HasDrizzle {
			dCmd := exec.CommandContext(ctx, "sh", "-c", "npx drizzle-kit push --force || npx drizzle-kit push || true")
			dCmd.Dir = projectDir
			dCmd.Env = execEnv
			dOut, _ := dCmd.CombinedOutput()
			appendLog("drizzle-kit push", string(dOut))
		}

		if err := ensureNodePM2Service(ctx, site, projectDir, appPort, insp, execEnv, appendLog); err != nil {
			warnings = append(warnings, "PM2 gagal menjalankan aplikasi: "+err.Error())
		}
		return warnings
	}

	// Kasus 3: Golang Service
	if site.Kind == "go" || insp.Kind == "go" {
		appPort := extractSitePort(site.Upstream, "8100")
		if err := ensureGoSystemdService(ctx, site, projectDir, appPort, insp, execEnv, appendLog); err != nil {
			warnings = append(warnings, "Build/Service Go gagal: "+err.Error())
		}
		return warnings
	}

	// Kasus 4: Python Service (FastAPI / Django / Flask)
	if site.Kind == "python" || insp.Kind == "python" {
		appPort := extractSitePort(site.Upstream, "8500")
		if err := ensurePythonSystemdService(ctx, site, projectDir, appPort, insp, execEnv, appendLog); err != nil {
			warnings = append(warnings, "Build/Service Python gagal: "+err.Error())
		}
		return warnings
	}

	return warnings
}

// inspectSiteProcess mendeteksi process manager terkelola (systemd / PM2) dan status Queue Worker Laravel milik situs
func inspectSiteProcess(ctx context.Context, site *store.Site) (manager, unit, state string, queueEnabled bool) {
	if site == nil || runtime.GOOS == "windows" {
		return "", "", "", false
	}

	// Cek Laravel Queue Worker unit untuk situs PHP
	if site.Kind == "php" {
		qUnit := siteQueueUnitName(site.Domain)
		if _, err := os.Stat(filepath.Join("/etc/systemd/system", qUnit)); err == nil {
			qCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			out, _ := exec.CommandContext(qCtx, "systemctl", "is-active", qUnit).Output()
			cancel()
			if strings.TrimSpace(string(out)) == "active" {
				queueEnabled = true
			}
		}
		return "", "", "", queueEnabled
	}

	// Cek unit systemd aplikasi Go / Python / Node terkelola
	appUnit := siteAppUnitName(site.Domain)
	if _, err := os.Stat(filepath.Join("/etc/systemd/system", appUnit)); err == nil {
		sCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		out, _ := exec.CommandContext(sCtx, "systemctl", "is-active", appUnit).Output()
		cancel()
		st := strings.TrimSpace(string(out))
		if st == "" {
			st = "inactive"
		}
		return "systemd", appUnit, st, false
	}

	// Cek aplikasi PM2 untuk Node.js
	if site.Kind == "node" {
		projectDir := resolveSiteProjectDir(site)
		hasEco := false
		for _, f := range []string{"ecosystem.config.cjs", "ecosystem.config.js"} {
			if _, err := os.Stat(filepath.Join(projectDir, f)); err == nil {
				hasEco = true
				break
			}
		}
		if hasEco || site.Upstream != "" {
			st := "inactive"
			if site.LiveStatus == "running" {
				st = "active"
			}
			return "pm2", site.Domain, st, false
		}
	}

	return "", "", "", false
}

// cleanupSiteManagedUnits menghentikan dan menghapus unit systemd aplikasi dan queue worker saat situs dihapus
func cleanupSiteManagedUnits(ctx context.Context, domain string) {
	if runtime.GOOS == "windows" || strings.TrimSpace(domain) == "" {
		return
	}
	removed := false
	for _, unit := range []string{siteAppUnitName(domain), siteQueueUnitName(domain)} {
		unitPath := filepath.Join("/etc/systemd/system", unit)
		if _, err := os.Stat(unitPath); err == nil {
			_ = exec.CommandContext(ctx, "systemctl", "disable", "--now", unit).Run()
			_ = os.Remove(unitPath)
			removed = true
		}
	}
	if removed {
		_ = exec.CommandContext(ctx, "systemctl", "daemon-reload").Run()
	}
}

// handleSiteProcessAction menjalankan aksi start / restart / stop pada proses aplikasi situs (systemd, PM2, atau PHP-FPM)
func (s *Server) handleSiteProcessAction(w http.ResponseWriter, r *http.Request) {
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
		Action string `json:"action"` // start | restart | stop
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "restart"
	}
	if action != "start" && action != "restart" && action != "stop" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "aksi proses harus start, restart, atau stop"})
		return
	}

	if runtime.GOOS == "windows" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"action":  action,
			"output":  "Mode pengembangan Windows — simulasi aksi proses berhasil",
			"message": fmt.Sprintf("Aksi %s untuk %s selesai (mode dev)", action, site.Domain),
		})
		return
	}

	pctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	execEnv := fullRuntimeExecEnv()
	projectDir := resolveSiteProjectDir(site)

	var logBuf strings.Builder
	appendLog := func(title, out string) {
		logBuf.WriteString("==> " + title + "\n")
		if trimmed := strings.TrimSpace(out); trimmed != "" {
			logBuf.WriteString(trimmed + "\n\n")
		} else {
			logBuf.WriteString("OK\n\n")
		}
	}

	// 1. Situs PHP: reload layanan PHP-FPM versi terkait (+ restart queue worker jika aktif)
	if site.Kind == "php" {
		fpmUnit := "php-fpm"
		if site.PHPVersion != "" {
			fpmUnit = fmt.Sprintf("php%s-fpm", site.PHPVersion)
		}
		sysAct := "reload"
		if action == "restart" {
			sysAct = "restart"
		}
		out, cmdErr := exec.CommandContext(pctx, "systemctl", sysAct, fpmUnit).CombinedOutput()
		appendLog("systemctl "+sysAct+" "+fpmUnit, string(out))
		if cmdErr != nil {
			s.auditSite(r, "site.process."+action, site.Domain, false, string(out))
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "gagal " + sysAct + " " + fpmUnit + ": " + strings.TrimSpace(string(out))})
			return
		}
		qUnit := siteQueueUnitName(site.Domain)
		if _, err := os.Stat(filepath.Join("/etc/systemd/system", qUnit)); err == nil {
			qOut, _ := exec.CommandContext(pctx, "systemctl", action, qUnit).CombinedOutput()
			appendLog("systemctl "+action+" "+qUnit, string(qOut))
		}
		s.auditSite(r, "site.process."+action, site.Domain, true, fpmUnit)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"manager": "php-fpm",
			"unit":    fpmUnit,
			"output":  strings.TrimSpace(logBuf.String()),
			"message": fmt.Sprintf("Layanan %s berhasil di-%s", fpmUnit, sysAct),
		})
		return
	}

	// 2. Unit systemd aplikasi terkelola (Go / Python / custom)
	appUnit := siteAppUnitName(site.Domain)
	if _, statErr := os.Stat(filepath.Join("/etc/systemd/system", appUnit)); statErr == nil {
		out, cmdErr := exec.CommandContext(pctx, "systemctl", action, appUnit).CombinedOutput()
		appendLog("systemctl "+action+" "+appUnit, string(out))
		if cmdErr != nil {
			s.auditSite(r, "site.process."+action, site.Domain, false, string(out))
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("gagal %s %s: %s", action, appUnit, strings.TrimSpace(string(out)))})
			return
		}
		s.auditSite(r, "site.process."+action, site.Domain, true, appUnit)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"manager": "systemd",
			"unit":    appUnit,
			"output":  strings.TrimSpace(logBuf.String()),
			"message": fmt.Sprintf("Service %s berhasil di-%s", appUnit, action),
		})
		return
	}

	// 3. Aplikasi Node.js via PM2 (bootstrap otomatis jika belum ada ecosystem.config.cjs saat start/restart)
	if site.Kind == "node" {
		if action == "stop" {
			cmd := exec.CommandContext(pctx, "sh", "-c", fmt.Sprintf("pm2 stop %q || (test -f ecosystem.config.cjs && pm2 stop ecosystem.config.cjs)", site.Domain))
			cmd.Dir = projectDir
			cmd.Env = execEnv
			out, cmdErr := cmd.CombinedOutput()
			appendLog("pm2 stop "+site.Domain, string(out))
			if cmdErr != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "gagal menghentikan proses PM2: " + strings.TrimSpace(string(out))})
				return
			}
		} else {
			insp := inspectLocalProject(projectDir)
			appPort := extractSitePort(site.Upstream, "3000")
			if err := ensureNodePM2Service(pctx, site, projectDir, appPort, insp, execEnv, appendLog); err != nil {
				s.auditSite(r, "site.process."+action, site.Domain, false, err.Error())
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error(), "output": strings.TrimSpace(logBuf.String())})
				return
			}
		}
		s.auditSite(r, "site.process."+action, site.Domain, true, "pm2:"+site.Domain)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"manager": "pm2",
			"unit":    site.Domain,
			"output":  strings.TrimSpace(logBuf.String()),
			"message": fmt.Sprintf("Proses PM2 %s berhasil di-%s", site.Domain, action),
		})
		return
	}

	// 4. Bootstrap otomatis jika Go/Python belum memiliki unit systemd saat user klik start/restart
	if action != "stop" && (site.Kind == "go" || site.Kind == "python") {
		insp := inspectLocalProject(projectDir)
		var bErr error
		if site.Kind == "go" {
			bErr = ensureGoSystemdService(pctx, site, projectDir, extractSitePort(site.Upstream, "8100"), insp, execEnv, appendLog)
		} else {
			bErr = ensurePythonSystemdService(pctx, site, projectDir, extractSitePort(site.Upstream, "8500"), insp, execEnv, appendLog)
		}
		if bErr != nil {
			s.auditSite(r, "site.process."+action, site.Domain, false, bErr.Error())
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": bErr.Error(), "output": strings.TrimSpace(logBuf.String())})
			return
		}
		s.auditSite(r, "site.process."+action, site.Domain, true, appUnit)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"manager": "systemd",
			"unit":    appUnit,
			"output":  strings.TrimSpace(logBuf.String()),
			"message": fmt.Sprintf("Service %s berhasil dibuat & dijalankan", appUnit),
		})
		return
	}

	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "situs ini tidak memiliki proses aplikasi terkelola (PM2/systemd)"})
}

// handleSiteProcessLogs mengambil log stdout/stderr terbaru dari proses aplikasi (systemd journalctl / PM2 / queue worker)
func (s *Server) handleSiteProcessLogs(w http.ResponseWriter, r *http.Request) {
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

	target := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("target")))
	if runtime.GOOS == "windows" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"source":  "dev",
			"content": "(Log proses tidak tersedia pada mode pengembangan Windows)",
		})
		return
	}

	lctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if target == "queue" {
		qUnit := siteQueueUnitName(site.Domain)
		out, _ := exec.CommandContext(lctx, "journalctl", "-u", qUnit, "-n", "120", "--no-pager").CombinedOutput()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"source":  "systemd:" + qUnit,
			"content": strings.TrimSpace(string(out)),
		})
		return
	}

	appUnit := siteAppUnitName(site.Domain)
	if _, statErr := os.Stat(filepath.Join("/etc/systemd/system", appUnit)); statErr == nil {
		out, _ := exec.CommandContext(lctx, "journalctl", "-u", appUnit, "-n", "120", "--no-pager").CombinedOutput()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"source":  "systemd:" + appUnit,
			"content": strings.TrimSpace(string(out)),
		})
		return
	}

	if site.Kind == "node" {
		cmd := exec.CommandContext(lctx, "pm2", "logs", site.Domain, "--nostream", "--lines", "120")
		cmd.Env = fullRuntimeExecEnv()
		out, _ := cmd.CombinedOutput()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"source":  "pm2:" + site.Domain,
			"content": strings.TrimSpace(string(out)),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"source":  "none",
		"content": "Belum ada log proses aplikasi terkelola (PM2/systemd) untuk situs ini.",
	})
}

// deployHealthResult merepresentasikan hasil uji kesehatan HTTP lokal pasca deployment
type deployHealthResult struct {
	OK         bool   `json:"ok"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int64  `json:"latency_ms"`
	Summary    string `json:"summary"`
}

// runPostDeploySmokeTest mengirim permintaan HTTP singkat ke origin lokal untuk memastikan aplikasi tidak error 500/502
func runPostDeploySmokeTest(ctx context.Context, site *store.Site) deployHealthResult {
	if site == nil || runtime.GOOS == "windows" {
		return deployHealthResult{OK: true, StatusCode: 200, LatencyMs: 1, Summary: "HTTP 200 OK (mode dev)"}
	}

	// Beri jeda singkat agar proses PM2/systemd/PHP-FPM selesai mengikat port
	select {
	case <-time.After(700 * time.Millisecond):
	case <-ctx.Done():
	}

	targetURL := "http://127.0.0.1:80/"
	if strings.TrimSpace(site.Upstream) != "" && (site.Kind == "node" || site.Kind == "go" || site.Kind == "python" || site.Kind == "proxy") {
		targetURL = strings.TrimRight(site.Upstream, "/") + "/"
	}

	hctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(hctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return deployHealthResult{OK: false, Summary: "Gagal menyusun smoke test: " + err.Error()}
	}
	req.Host = site.Domain
	req.Header.Set("User-Agent", "FikPanel-DeploySmokeTest/1.0")

	client := &http.Client{
		Timeout: 4 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	started := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		return deployHealthResult{
			OK:        false,
			LatencyMs: latency,
			Summary:   "Origin belum merespons: " + err.Error(),
		}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	// Kode 2xx/3xx/401/403 (misal redirect login) dianggap sehat; 5xx adalah indikasi crash aplikasi
	healthy := resp.StatusCode >= 200 && resp.StatusCode < 500
	summary := fmt.Sprintf("HTTP %d (%dms)", resp.StatusCode, latency)
	if resp.StatusCode >= 500 {
		summary = fmt.Sprintf("Peringatan HTTP %d (%dms) — periksa Error Log / .env aplikasi", resp.StatusCode, latency)
	}
	return deployHealthResult{
		OK:         healthy,
		StatusCode: resp.StatusCode,
		LatencyMs:  latency,
		Summary:    summary,
	}
}

// provisionRealDeployTemplate membangun proyek nyata siap jalan untuk setiap template One-Click Deploy
func (s *Server) provisionRealDeployTemplate(ctx context.Context, site *store.Site, tpl, dbName, dbUser, dbPass string, notes map[string]string) error {
	root := site.RootPath
	projectDir := root
	if filepath.Base(root) == "public" {
		projectDir = filepath.Dir(root)
	}
	_ = os.MkdirAll(root, 0o755)
	_ = os.MkdirAll(projectDir, 0o755)

	execEnv := fullRuntimeExecEnv()
	var logBuf strings.Builder
	appendLog := func(title, out string) {
		logBuf.WriteString("==> " + title + "\n")
		if t := strings.TrimSpace(out); t != "" {
			logBuf.WriteString(t + "\n\n")
		} else {
			logBuf.WriteString("OK\n\n")
		}
	}

	switch tpl {
	case "laravel":
		updateSiteDeployProgress(site.Domain, 50, "composer", "Mengunduh & membuat proyek Laravel baru via Composer...")
		phpBin := resolvePHPBinary(site.PHPVersion)
		compCmd := resolveComposerCommand(phpBin)

		// Hapus placeholder index agar create-project berjalan di direktori kosong
		_ = os.RemoveAll(projectDir)
		_ = os.MkdirAll(projectDir, 0o755)

		bctx, cancel := context.WithTimeout(ctx, 240*time.Second)
		defer cancel()
		cmd := exec.CommandContext(bctx, "sh", "-c", fmt.Sprintf("%s create-project --no-interaction --prefer-dist laravel/laravel .", compCmd))
		cmd.Dir = projectDir
		cmd.Env = execEnv
		out, err := cmd.CombinedOutput()
		appendLog("composer create-project laravel/laravel", string(out))
		if err != nil {
			// Fallback aman bila server offline / Composer tidak tersedia
			_ = os.MkdirAll(root, 0o755)
			fallback := fmt.Sprintf(`<?php header('Content-Type: text/html; charset=utf-8'); ?><!doctype html><html lang="id"><head><meta charset="utf-8"><title>%s — Laravel Starter</title></head><body style="font-family:system-ui;background:#0b1020;color:#e5e7eb;padding:3rem"><h2>Laravel Environment Siap (%s)</h2><p>Composer gagal mengunduh otomatis (%s). Jalankan <code>composer create-project laravel/laravel .</code> di Terminal.</p></body></html>`, site.Domain, phpVersionLabel(site.PHPVersion), strings.TrimSpace(err.Error()))
			_ = os.WriteFile(filepath.Join(root, "index.php"), []byte(fallback), 0o644)
			return fmt.Errorf("composer create-project gagal: %s", strings.TrimSpace(string(out)))
		}

		// Suntikkan .env aman dengan APP_KEY baru & kredensial database MySQL
		keyBytes := make([]byte, 32)
		_, _ = rand.Read(keyBytes)
		appKey := "base64:" + base64.StdEncoding.EncodeToString(keyBytes)
		cookieName := "fikpanel_" + sanitizeCookieSlug(site.Domain) + "_session"
		appURL := resolveAppURL(ctx, site, false)
		envContent := fmt.Sprintf("APP_NAME=%s\nAPP_ENV=production\nAPP_KEY=%s\nAPP_DEBUG=false\nAPP_URL=%s\nSESSION_DRIVER=file\nSESSION_COOKIE=%s\nCACHE_STORE=file\n\nDB_CONNECTION=mysql\nDB_HOST=127.0.0.1\nDB_PORT=3306\nDB_DATABASE=%s\nDB_USERNAME=%s\nDB_PASSWORD=%s\n",
			quoteEnvValue(site.Domain), appKey, appURL, cookieName, dbName, dbUser, quoteEnvValue(dbPass))
		_ = os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0o640)

		updateSiteDeployProgress(site.Domain, 78, "migrate", "Menjalankan migrasi database & storage:link Laravel...")
		migCmd := exec.CommandContext(bctx, "sh", "-c", fmt.Sprintf("(%s artisan migrate --force || true) && (%s artisan storage:link --force || true) && (%s artisan optimize || true)", phpBin, phpBin, phpBin))
		migCmd.Dir = projectDir
		migCmd.Env = execEnv
		mOut, _ := migCmd.CombinedOutput()
		appendLog("artisan migrate & optimize", string(mOut))

		site.Framework = "Laravel"
		site.RootPath = filepath.ToSlash(filepath.Join(projectDir, "public"))
		_ = s.store.UpdateSite(ctx, site)
		_ = s.sites.Apply(ctx, site, "")
		fixSiteWebPermissions(ctx, projectDir, site)
		_ = os.WriteFile(filepath.Join(projectDir, "fikpanel-deploy.log"), []byte(logBuf.String()), 0o640)

	case "wordpress":
		// WordPress meletakkan index.php langsung di root proyek (bukan /public)
		wpRoot := projectDir
		_ = os.MkdirAll(wpRoot, 0o755)
		_ = os.Remove(filepath.Join(wpRoot, "public", "index.php"))
		_ = os.Remove(filepath.Join(wpRoot, "public", "index.html"))
		_ = os.Remove(filepath.Join(wpRoot, "public"))

		updateSiteDeployProgress(site.Domain, 50, "template", "Mengunduh arsip resmi WordPress terbaru...")
		wctx, cancel := context.WithTimeout(ctx, 180*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(wctx, http.MethodGet, "https://wordpress.org/latest.tar.gz", nil)
		req.Header.Set("User-Agent", "FikPanel-WordPressInstaller/1.0")
		resp, err := (&http.Client{Timeout: 150 * time.Second}).Do(req)
		if err != nil {
			return fmt.Errorf("gagal mengunduh WordPress dari wordpress.org: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unduhan WordPress gagal (HTTP %d)", resp.StatusCode)
		}

		updateSiteDeployProgress(site.Domain, 72, "template", "Mengekstrak berkas WordPress & menghasilkan wp-config.php...")
		if err := extractWordPressTarGz(resp.Body, wpRoot); err != nil {
			return fmt.Errorf("ekstraksi WordPress gagal: %w", err)
		}

		// Hasilkan 8 salt kriptografis unik untuk wp-config.php
		salts := make([]string, 8)
		for i := range salts {
			b := make([]byte, 32)
			_, _ = rand.Read(b)
			salts[i] = hex.EncodeToString(b)
		}
		escPHP := func(v string) string {
			return strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v)
		}
		wpConfig := fmt.Sprintf(`<?php
// Konfigurasi WordPress Otomatis oleh FikPanel — %s
define( 'DB_NAME', '%s' );
define( 'DB_USER', '%s' );
define( 'DB_PASSWORD', '%s' );
define( 'DB_HOST', '127.0.0.1' );
define( 'DB_CHARSET', 'utf8mb4' );
define( 'DB_COLLATE', '' );

define( 'AUTH_KEY',         '%s' );
define( 'SECURE_AUTH_KEY',  '%s' );
define( 'LOGGED_IN_KEY',    '%s' );
define( 'NONCE_KEY',        '%s' );
define( 'AUTH_SALT',        '%s' );
define( 'SECURE_AUTH_SALT', '%s' );
define( 'LOGGED_IN_SALT',   '%s' );
define( 'NONCE_SALT',       '%s' );

$table_prefix = 'wp_';
define( 'WP_DEBUG', false );
define( 'FS_METHOD', 'direct' );

if ( ! defined( 'ABSPATH' ) ) {
	define( 'ABSPATH', __DIR__ . '/' );
}
require_once ABSPATH . 'wp-settings.php';
`, site.Domain, escPHP(dbName), escPHP(dbUser), escPHP(dbPass),
			salts[0], salts[1], salts[2], salts[3], salts[4], salts[5], salts[6], salts[7])
		_ = os.WriteFile(filepath.Join(wpRoot, "wp-config.php"), []byte(wpConfig), 0o640)

		site.RootPath = filepath.ToSlash(wpRoot)
		site.Framework = "WordPress"
		_ = s.store.UpdateSite(ctx, site)
		_ = s.sites.Apply(ctx, site, "")
		fixSiteWebPermissions(ctx, wpRoot, site)
		_ = os.Chmod(filepath.Join(wpRoot, "wp-config.php"), 0o640)

	case "express":
		updateSiteDeployProgress(site.Domain, 55, "template", "Membuat layanan Node.js HTTP & mendaftarkan ke PM2...")
		appPort := extractSitePort(site.Upstream, "3100")
		pkgJSON := fmt.Sprintf(`{
  "name": %q,
  "version": "1.0.0",
  "private": true,
  "main": "server.js",
  "scripts": {
    "start": "node server.js"
  }
}
`, siteSystemdSlug(site.Domain))
		serverJS := fmt.Sprintf(`// Layanan Node.js starter — dibuat otomatis oleh FikPanel
const http = require('http');
const port = parseInt(process.env.PORT || %q, 10);
const host = process.env.HOST || '127.0.0.1';

const server = http.createServer((req, res) => {
  if (req.url === '/health') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ ok: true, service: %q, runtime: 'node', port }));
    return;
  }
  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
  res.end('<!doctype html><html lang="id"><head><meta charset="utf-8"><title>%s</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#0b1020;color:#f8fafc;font-family:system-ui,sans-serif}.card{padding:2rem 2.5rem;border-radius:14px;background:#111827;border:1px solid #1f2937;max-width:520px}h1{margin:0 0 .5rem;color:#34d399;font-size:1.35rem}code{background:#090d16;padding:.2rem .45rem;border-radius:4px}</style></head><body><div class="card"><h1>Node.js Service Aktif</h1><p>Domain <b>%s</b> berjalan via PM2 di port <code>' + port + '</code>. Endpoint kesehatan: <code>/health</code>.</p></div></body></html>');
});

server.listen(port, host, () => {
  console.log('Node.js listening on http://' + host + ':' + port);
});
`, appPort, site.Domain, site.Domain, site.Domain)

		_ = os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(pkgJSON), 0o644)
		_ = os.WriteFile(filepath.Join(projectDir, "server.js"), []byte(serverJS), 0o644)
		_ = os.WriteFile(filepath.Join(projectDir, ".env"), []byte(fmt.Sprintf("NODE_ENV=production\nHOST=127.0.0.1\nPORT=%s\n", appPort)), 0o640)

		site.Framework = "Node.js"
		_ = s.store.UpdateSite(ctx, site)
		insp := inspectLocalProject(projectDir)
		if err := ensureNodePM2Service(ctx, site, projectDir, appPort, insp, execEnv, appendLog); err != nil {
			return err
		}

	case "gin":
		updateSiteDeployProgress(site.Domain, 55, "template", "Membuat & mengompilasi layanan Golang HTTP + systemd...")
		appPort := extractSitePort(site.Upstream, "8100")
		modContent := fmt.Sprintf("module %s\n\ngo 1.21\n", siteSystemdSlug(site.Domain))
		mainGo := fmt.Sprintf(`package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

// main menjalankan server HTTP Golang starter yang dikelola FikPanel
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = %q
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"ok\":true,\"service\":%q,\"runtime\":\"go\",\"port\":%q}", port)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<!doctype html><html lang=\"id\"><head><meta charset=\"utf-8\"><title>%s</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#0b1020;color:#f8fafc;font-family:system-ui,sans-serif}.card{padding:2rem 2.5rem;border-radius:14px;background:#111827;border:1px solid #1f2937;max-width:520px}h1{margin:0 0 .5rem;color:#38bdf8;font-size:1.35rem}code{background:#090d16;padding:.2rem .45rem;border-radius:4px}</style></head><body><div class=\"card\"><h1>Go Service Aktif</h1><p>Domain <b>%s</b> berjalan via systemd di port <code>%%s</code>. Endpoint kesehatan: <code>/health</code>.</p></div></body></html>", port)
	})
	addr := "127.0.0.1:" + port
	log.Println("Go service listening on http://" + addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
`, appPort, site.Domain, "%s", site.Domain, site.Domain)

		_ = os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(modContent), 0o644)
		_ = os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(mainGo), 0o644)
		_ = os.WriteFile(filepath.Join(projectDir, ".env"), []byte(fmt.Sprintf("PORT=%s\nGIN_MODE=release\n", appPort)), 0o640)

		site.Framework = "Go"
		_ = s.store.UpdateSite(ctx, site)
		insp := inspectLocalProject(projectDir)
		if err := ensureGoSystemdService(ctx, site, projectDir, appPort, insp, execEnv, appendLog); err != nil {
			return err
		}

	case "fastapi":
		updateSiteDeployProgress(site.Domain, 55, "template", "Membuat layanan Python ASGI/HTTP (.venv + systemd)...")
		appPort := extractSitePort(site.Upstream, "8500")
		reqTxt := "fastapi\nuvicorn[standard]\n"
		mainPy := fmt.Sprintf(`# Layanan Python FastAPI starter — dibuat otomatis oleh FikPanel
from fastapi import FastAPI
from fastapi.responses import HTMLResponse
import os

app = FastAPI(title=%q)

@app.get("/health")
def health():
    return {"ok": True, "service": %q, "runtime": "python", "port": os.getenv("PORT", %q)}

@app.get("/", response_class=HTMLResponse)
def index():
    port = os.getenv("PORT", %q)
    return f"""<!doctype html><html lang="id"><head><meta charset="utf-8"><title>%s</title>
    <style>body{{margin:0;min-height:100vh;display:grid;place-items:center;background:#0b1020;color:#f8fafc;font-family:system-ui,sans-serif}}.card{{padding:2rem 2.5rem;border-radius:14px;background:#111827;border:1px solid #1f2937;max-width:520px}}h1{{margin:0 0 .5rem;color:#facc15;font-size:1.35rem}}code{{background:#090d16;padding:.2rem .45rem;border-radius:4px}}</style></head>
    <body><div class="card"><h1>Python FastAPI Aktif</h1><p>Domain <b>%s</b> berjalan via Uvicorn systemd di port <code>{{port}}</code>. Docs interaktif: <code>/docs</code>.</p></div></body></html>"""
`, site.Domain, site.Domain, appPort, appPort, site.Domain, site.Domain)

		_ = os.WriteFile(filepath.Join(projectDir, "requirements.txt"), []byte(reqTxt), 0o644)
		_ = os.WriteFile(filepath.Join(projectDir, "main.py"), []byte(mainPy), 0o644)
		_ = os.WriteFile(filepath.Join(projectDir, ".env"), []byte(fmt.Sprintf("PORT=%s\nPYTHONUNBUFFERED=1\n", appPort)), 0o640)

		site.Framework = "FastAPI"
		_ = s.store.UpdateSite(ctx, site)
		insp := inspectLocalProject(projectDir)
		if err := ensurePythonSystemdService(ctx, site, projectDir, appPort, insp, execEnv, appendLog); err != nil {
			return err
		}

	case "spa":
		spaHTML := fmt.Sprintf(`<!doctype html>
<html lang="id">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title>
<style>body{background:#090d16;color:#f8fafc;font-family:system-ui,sans-serif;display:grid;place-items:center;min-height:100vh;margin:0}.box{background:#111827;border:1px solid #1f2937;border-radius:12px;padding:2rem 2.5rem;max-width:500px}h2{margin:0 0 .5rem;color:#fb923c}code{background:#090d16;padding:.2rem .45rem;border-radius:4px}</style>
</head>
<body><div id="app" class="box"><h2>%s</h2><p>SPA History Routing (React / Vue / Vite / Svelte) aktif dengan fallback <code>try_files $uri $uri/ /index.html</code>.</p></div></body></html>`, site.Domain, site.Domain)
		_ = os.WriteFile(filepath.Join(root, "index.html"), []byte(spaHTML), 0o644)
		site.Framework = "SPA"
		_ = s.store.UpdateSite(ctx, site)
		_ = s.sites.Apply(ctx, site, "")
	}

	if notes != nil && logBuf.Len() > 0 {
		_ = os.WriteFile(filepath.Join(projectDir, "fikpanel-deploy.log"), []byte(logBuf.String()), 0o640)
	}
	return nil
}

// extractWordPressTarGz mengekstrak isi folder "wordpress/" dari arsip tar.gz resmi langsung ke direktori target dengan proteksi path traversal
func extractWordPressTarGz(r io.Reader, targetDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	cleanTarget := filepath.Clean(targetDir)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "wordpress/")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || rel == "." {
			continue
		}
		dst := filepath.Clean(filepath.Join(cleanTarget, filepath.FromSlash(rel)))
		if !strings.HasPrefix(dst, cleanTarget+string(os.PathSeparator)) && dst != cleanTarget {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(dst, 0o755)
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(dst), 0o755)
			f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, io.LimitReader(tr, 50<<20)); err != nil {
				_ = f.Close()
				return err
			}
			_ = f.Close()
		}
	}
	return nil
}

// ════════════════════════════════════════════════════════════════════════════
// EDITOR .ENV VISUAL & AMAN PER SITUS
// ════════════════════════════════════════════════════════════════════════════

type siteEnvEntry struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

var envKeyRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// isSensitiveEnvKey menandai apakah variabel .env sebaiknya disamarkan secara default pada UI
func isSensitiveEnvKey(k string) bool {
	u := strings.ToUpper(k)
	for _, needle := range []string{"PASS", "SECRET", "TOKEN", "KEY", "PRIVATE", "AUTH", "DATABASE_URL", "SMTP"} {
		if strings.Contains(u, needle) {
			return true
		}
	}
	return false
}

// parseEnvEntries mengurai isi berkas .env menjadi pasangan key-value terstruktur
func parseEnvEntries(raw string) []siteEnvEntry {
	entries := []siteEnvEntry{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "export ")
		idx := strings.IndexByte(trimmed, '=')
		if idx <= 0 {
			continue
		}
		k := strings.TrimSpace(trimmed[:idx])
		v := strings.TrimSpace(trimmed[idx+1:])
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		entries = append(entries, siteEnvEntry{
			Key:    k,
			Value:  v,
			Secret: isSensitiveEnvKey(k),
		})
	}
	return entries
}

// handleGetSiteEnv membaca berkas .env proyek situs untuk editor visual dan mentah
func (s *Server) handleGetSiteEnv(w http.ResponseWriter, r *http.Request) {
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
	projectDir := resolveSiteProjectDir(site)
	envPath := filepath.Join(projectDir, ".env")
	rawBytes, readErr := os.ReadFile(envPath)
	exists := readErr == nil
	rawStr := ""
	if exists {
		rawStr = string(rawBytes)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"exists":  exists,
		"path":    filepath.ToSlash(envPath),
		"raw":     rawStr,
		"entries": parseEnvEntries(rawStr),
	})
}

// handleUpdateSiteEnv menyimpan berkas .env situs dengan izin aman 0640 serta opsi reload cache / restart aplikasi
func (s *Server) handleUpdateSiteEnv(w http.ResponseWriter, r *http.Request) {
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
		Mode       string         `json:"mode"` // structured | raw
		Raw        string         `json:"raw"`
		Entries    []siteEnvEntry `json:"entries"`
		ApplyCache bool           `json:"apply_cache"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}

	projectDir := resolveSiteProjectDir(site)
	if strings.TrimSpace(projectDir) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "direktori proyek situs tidak tersedia"})
		return
	}
	_ = os.MkdirAll(projectDir, 0o755)
	envPath := filepath.Join(projectDir, ".env")

	var finalContent string
	if req.Mode == "raw" {
		finalContent = strings.ReplaceAll(req.Raw, "\r\n", "\n")
		if finalContent != "" && !strings.HasSuffix(finalContent, "\n") {
			finalContent += "\n"
		}
	} else {
		var lines []string
		for _, item := range req.Entries {
			k := strings.TrimSpace(item.Key)
			if k == "" {
				continue
			}
			if !envKeyRegex.MatchString(k) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("nama variabel .env tidak valid: %q", k)})
				return
			}
			v := item.Value
			if strings.ContainsAny(v, " #$\"'\\") {
				lines = append(lines, fmt.Sprintf("%s=%s", k, quoteEnvValue(v)))
			} else {
				lines = append(lines, fmt.Sprintf("%s=%s", k, v))
			}
		}
		finalContent = strings.Join(lines, "\n") + "\n"
	}

	if err := os.WriteFile(envPath, []byte(finalContent), 0o640); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menyimpan berkas .env: " + err.Error()})
		return
	}

	reloadedNote := ""
	if runtime.GOOS != "windows" {
		if site.Kind == "php" {
			_ = exec.CommandContext(r.Context(), "chown", "www-data:www-data", envPath).Run()
			_ = os.Chmod(envPath, 0o640)
		}
		if req.ApplyCache {
			actx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
			defer cancel()
			execEnv := fullRuntimeExecEnv()
			if site.Kind == "php" {
				if _, statErr := os.Stat(filepath.Join(projectDir, "artisan")); statErr == nil {
					phpBin := resolvePHPBinary(site.PHPVersion)
					runLaravelProductionOptimize(actx, projectDir, phpBin, execEnv, true, func(string, string) {})
					reloadedNote = " & cache Laravel/Filament diperbarui"
				}
			} else if site.Kind == "node" {
				cmd := exec.CommandContext(actx, "sh", "-c", fmt.Sprintf("pm2 restart %q --update-env || (test -f ecosystem.config.cjs && pm2 restart ecosystem.config.cjs --update-env)", site.Domain))
				cmd.Dir = projectDir
				cmd.Env = execEnv
				_ = cmd.Run()
				reloadedNote = " & proses PM2 direstart"
			} else if site.Kind == "go" || site.Kind == "python" {
				appUnit := siteAppUnitName(site.Domain)
				if _, statErr := os.Stat(filepath.Join("/etc/systemd/system", appUnit)); statErr == nil {
					_ = exec.CommandContext(actx, "systemctl", "restart", appUnit).Run()
					reloadedNote = " & service " + appUnit + " direstart"
				}
			}
		}
	}

	s.auditSite(r, "site.env.save", site.Domain, true, fmt.Sprintf("mode=%s apply=%v", req.Mode, req.ApplyCache))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"path":    filepath.ToSlash(envPath),
		"raw":     finalContent,
		"entries": parseEnvEntries(finalContent),
		"message": "Berkas .env (izin 0640) berhasil disimpan" + reloadedNote,
	})
}

// ════════════════════════════════════════════════════════════════════════════
// LARAVEL & FILAMENT: READINESS CHECKLIST, EKSTENSI PHP, & QUEUE WORKER
// ════════════════════════════════════════════════════════════════════════════

// checkPHPModulesInstalled memeriksa daftar ekstensi PHP yang aktif pada binary PHP versi situs
func checkPHPModulesInstalled(ctx context.Context, phpBin string) map[string]bool {
	mods := map[string]bool{}
	if runtime.GOOS == "windows" {
		for _, m := range []string{"intl", "bcmath", "mbstring", "xml", "zip", "pdo_mysql", "curl", "gd", "exif"} {
			mods[m] = true
		}
		return mods
	}
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, phpBin, "-m").Output()
	if err != nil {
		return mods
	}
	for _, line := range strings.Split(string(out), "\n") {
		m := strings.ToLower(strings.TrimSpace(line))
		if m != "" && !strings.HasPrefix(m, "[") {
			mods[m] = true
		}
	}
	return mods
}

// ensureLaravelPHPExtensions memastikan ekstensi wajib Filament (intl, bcmath) terpasang bila apt-get tersedia
func ensureLaravelPHPExtensions(ctx context.Context, phpVersion string, appendLog func(string, string)) {
	if runtime.GOOS == "windows" {
		return
	}
	phpBin := resolvePHPBinary(phpVersion)
	mods := checkPHPModulesInstalled(ctx, phpBin)
	ver := strings.TrimSpace(phpVersion)
	if ver == "" {
		// Deteksi versi minor dari php -r
		if out, err := exec.CommandContext(ctx, phpBin, "-r", "echo PHP_MAJOR_VERSION.'.'.PHP_MINOR_VERSION;").Output(); err == nil {
			ver = strings.TrimSpace(string(out))
		}
	}
	if ver == "" {
		return
	}

	var missingPkgs []string
	for _, ext := range []string{"intl", "bcmath"} {
		if !mods[ext] {
			missingPkgs = append(missingPkgs, fmt.Sprintf("php%s-%s", ver, ext))
		}
	}
	if len(missingPkgs) == 0 {
		return
	}

	ictx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	args := append([]string{"install", "-y", "-qq"}, missingPkgs...)
	cmd := exec.CommandContext(ictx, "apt-get", args...)
	cmd.Env = append(fullRuntimeExecEnv(), "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	if appendLog != nil {
		appendLog("auto-install ekstensi Filament ("+strings.Join(missingPkgs, ", ")+")", string(out))
	}
	if err == nil {
		_ = exec.CommandContext(ictx, "systemctl", "reload", fmt.Sprintf("php%s-fpm", ver)).Run()
	}
}

// isLocalRedisListening memeriksa apakah layanan Redis lokal aktif menerima koneksi di 127.0.0.1:6379
func isLocalRedisListening() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:6379", 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// handleGetLaravelReadiness mengembalikan daftar cek kesiapan produksi Laravel / Filament (ekstensi PHP, FilamentUser, symlink, Queue Worker, Scheduler)
func (s *Server) handleGetLaravelReadiness(w http.ResponseWriter, r *http.Request) {
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

	projectDir := resolveSiteProjectDir(site)
	phpBin := resolvePHPBinary(site.PHPVersion)
	mods := checkPHPModulesInstalled(r.Context(), phpBin)

	requiredExts := []string{"intl", "bcmath", "mbstring", "xml", "zip", "pdo_mysql", "curl", "gd"}
	missingExts := []string{}
	extStatus := make([]map[string]interface{}, 0, len(requiredExts))
	for _, ext := range requiredExts {
		installed := mods[ext]
		if !installed {
			missingExts = append(missingExts, ext)
		}
		extStatus = append(extStatus, map[string]interface{}{
			"name":      ext,
			"installed": installed,
			"required":  ext == "intl" || ext == "mbstring" || ext == "pdo_mysql" || ext == "xml",
		})
	}

	// Periksa .env (APP_ENV, APP_DEBUG, QUEUE_CONNECTION)
	appEnv, appDebug, queueConn := "", "", ""
	if rawEnv, err := os.ReadFile(filepath.Join(projectDir, ".env")); err == nil {
		for _, e := range parseEnvEntries(string(rawEnv)) {
			switch e.Key {
			case "APP_ENV":
				appEnv = e.Value
			case "APP_DEBUG":
				appDebug = e.Value
			case "QUEUE_CONNECTION":
				queueConn = e.Value
			}
		}
	}

	// Periksa implementasi FilamentUser pada app/Models/User.php
	filamentUserOK := true
	if rawUser, err := os.ReadFile(filepath.Join(projectDir, "app", "Models", "User.php")); err == nil {
		code := string(rawUser)
		filamentUserOK = strings.Contains(code, "FilamentUser") || strings.Contains(code, "canAccessPanel")
	}

	_, storageLinkErr := os.Stat(filepath.Join(projectDir, "public", "storage"))
	_, vendorErr := os.Stat(filepath.Join(projectDir, "vendor", "autoload.php"))
	_, _, _, queueEnabled := inspectSiteProcess(r.Context(), site)
	schedJob := s.findSiteLaravelSchedulerCron(r.Context(), site.Domain)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                   true,
		"php_binary":           phpBin,
		"php_version":          site.PHPVersion,
		"extensions":           extStatus,
		"missing_extensions":   missingExts,
		"storage_linked":       storageLinkErr == nil,
		"vendor_ready":         vendorErr == nil,
		"filament_user_ok":     filamentUserOK,
		"app_env":              appEnv,
		"app_debug":            appDebug,
		"queue_connection":     queueConn,
		"queue_worker_enabled": queueEnabled,
		"queue_unit":           siteQueueUnitName(site.Domain),
		"scheduler_enabled":    schedJob != nil && schedJob.Enabled,
		"redis_available":      isLocalRedisListening(),
	})
}

// handleInstallLaravelExtensions memasang ekstensi PHP yang hilang (intl, bcmath, exif, zip, dll.) secara 1-klik lalu mereload PHP-FPM
func (s *Server) handleInstallLaravelExtensions(w http.ResponseWriter, r *http.Request) {
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

	if runtime.GOOS == "windows" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"message": "Mode dev Windows: simulasi instalasi ekstensi PHP berhasil",
		})
		return
	}

	phpBin := resolvePHPBinary(site.PHPVersion)
	ver := strings.TrimSpace(site.PHPVersion)
	if ver == "" {
		if out, err := exec.CommandContext(r.Context(), phpBin, "-r", "echo PHP_MAJOR_VERSION.'.'.PHP_MINOR_VERSION;").Output(); err == nil {
			ver = strings.TrimSpace(string(out))
		}
	}
	if ver == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "versi PHP situs tidak terdeteksi"})
		return
	}

	mods := checkPHPModulesInstalled(r.Context(), phpBin)
	var pkgs []string
	for _, ext := range []string{"intl", "bcmath", "mbstring", "xml", "zip", "curl", "gd", "mysql"} {
		checkKey := ext
		if ext == "mysql" {
			checkKey = "pdo_mysql"
		}
		if !mods[checkKey] {
			pkgs = append(pkgs, fmt.Sprintf("php%s-%s", ver, ext))
		}
	}
	if len(pkgs) == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"message": "Semua ekstensi wajib Laravel & Filament sudah terpasang pada PHP " + ver,
		})
		return
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	ictx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	args := append([]string{"install", "-y"}, pkgs...)
	cmd := exec.CommandContext(ictx, "apt-get", args...)
	cmd.Env = append(fullRuntimeExecEnv(), "DEBIAN_FRONTEND=noninteractive")
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		s.auditSite(r, "site.laravel.extensions", site.Domain, false, string(out))
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "gagal memasang ekstensi PHP: " + strings.TrimSpace(string(out))})
		return
	}
	_ = exec.CommandContext(ictx, "systemctl", "reload", fmt.Sprintf("php%s-fpm", ver)).Run()

	s.auditSite(r, "site.laravel.extensions", site.Domain, true, strings.Join(pkgs, ", "))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"installed": pkgs,
		"output":    strings.TrimSpace(string(out)),
		"message":   fmt.Sprintf("Ekstensi %s berhasil dipasang & php%s-fpm di-reload", strings.Join(pkgs, ", "), ver),
	})
}

// handleToggleLaravelQueueWorker mengaktifkan atau menonaktifkan daemon systemd 'php artisan queue:work' untuk situs Laravel/Filament
func (s *Server) handleToggleLaravelQueueWorker(w http.ResponseWriter, r *http.Request) {
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
		Enabled bool `json:"enabled"`
		Tries   int  `json:"tries"`
		Timeout int  `json:"timeout"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	if req.Tries <= 0 || req.Tries > 10 {
		req.Tries = 3
	}
	if req.Timeout <= 0 || req.Timeout > 600 {
		req.Timeout = 120
	}

	projectDir := resolveSiteProjectDir(site)
	unitName := siteQueueUnitName(site.Domain)
	if runtime.GOOS == "windows" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":                   true,
			"queue_worker_enabled": req.Enabled,
			"queue_unit":           unitName,
			"message":              "Mode dev: status Laravel Queue Worker diperbarui",
		})
		return
	}

	if _, err := os.Stat(filepath.Join(projectDir, "artisan")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "berkas artisan Laravel tidak ditemukan pada direktori proyek"})
		return
	}

	unitPath := filepath.Join("/etc/systemd/system", unitName)
	qctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if req.Enabled {
		phpBin := resolvePHPBinary(site.PHPVersion)
		unitContent := fmt.Sprintf(`[Unit]
Description=FikPanel Laravel Queue Worker (%s)
After=network.target mysql.service mariadb.service redis-server.service

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=%s
ExecStart=%s artisan queue:work --sleep=3 --tries=%d --timeout=%d --max-time=3600
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, site.Domain, projectDir, phpBin, req.Tries, req.Timeout)
		if err := os.WriteFile(unitPath, []byte(unitContent), 0o644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menulis unit queue worker: " + err.Error()})
			return
		}
		out, cmdErr := exec.CommandContext(qctx, "sh", "-c", fmt.Sprintf("systemctl daemon-reload && systemctl enable %q && systemctl restart %q", unitName, unitName)).CombinedOutput()
		if cmdErr != nil {
			s.auditSite(r, "site.laravel.queue", site.Domain, false, string(out))
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "gagal menjalankan queue worker: " + strings.TrimSpace(string(out))})
			return
		}
		s.auditSite(r, "site.laravel.queue", site.Domain, true, "Aktifkan "+unitName)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":                   true,
			"queue_worker_enabled": true,
			"queue_unit":           unitName,
			"message":              "Laravel Queue Worker aktif (" + unitName + ")",
		})
		return
	}

	_ = exec.CommandContext(qctx, "systemctl", "disable", "--now", unitName).Run()
	_ = os.Remove(unitPath)
	_ = exec.CommandContext(qctx, "systemctl", "daemon-reload").Run()
	s.auditSite(r, "site.laravel.queue", site.Domain, true, "Nonaktifkan "+unitName)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                   true,
		"queue_worker_enabled": false,
		"queue_unit":           unitName,
		"message":              "Laravel Queue Worker dinonaktifkan",
	})
}
