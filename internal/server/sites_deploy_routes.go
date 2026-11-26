package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	"fikpanel/internal/cloudflare"
	"fikpanel/internal/store"
)

var (
	siteDeployLocks   sync.Map // map[int64]*sync.Mutex untuk mencegah deploy tumpang-tindih pada situs yang sama
	gitCommitHashRe   = regexp.MustCompile(`^[0-9a-fA-F]{6,40}$`)
	laravelDefaultCmd = "composer install --no-interaction --prefer-dist --optimize-autoloader && php artisan key:generate --force && php artisan migrate --force"
)

// getSiteDeployLock mengembalikan mutex eksklusif per ID situs agar proses deploy tidak berjalan paralel di direktori yang sama
func getSiteDeployLock(siteID int64) *sync.Mutex {
	val, _ := siteDeployLocks.LoadOrStore(siteID, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// resolvePHPBinary memilih binary PHP CLI sesuai versi PHP-FPM situs (mis. /usr/bin/php8.3) agar cocok dengan runtime web
func resolvePHPBinary(phpVersion string) string {
	ver := strings.TrimSpace(phpVersion)
	if ver != "" && runtime.GOOS != "windows" {
		cand := "/usr/bin/php" + ver
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand
		}
	}
	return "php"
}

// resolveComposerCommand menjalankan Composer menggunakan binary PHP situs agar tidak bentrok dengan versi PHP default OS
func resolveComposerCommand(phpBin string) string {
	if runtime.GOOS != "windows" && phpBin != "" && phpBin != "php" {
		for _, compPath := range []string{"/usr/local/bin/composer", "/usr/bin/composer"} {
			if st, err := os.Stat(compPath); err == nil && !st.IsDir() {
				return fmt.Sprintf("%s %s", phpBin, compPath)
			}
		}
	}
	return "composer"
}

// quoteEnvValue mengutip nilai .env secara aman agar karakter spesial (#, $, ", spasi) tidak merusak parser phpdotenv
func quoteEnvValue(v string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`, "\r", "", "\n", "").Replace(v)
	return `"` + escaped + `"`
}

// resolveAppURL menentukan skema APP_URL (https vs http) sesuai kesiapan SSL/Cloudflare untuk mencegah 403 Invalid Signature Livewire/Filament
func resolveAppURL(ctx context.Context, site *store.Site, autoSSL bool) string {
	if autoSSL || (site.SSLMode != "" && site.SSLMode != "none") {
		return "https://" + site.Domain
	}
	if cloudflare.IsDomainProxiedByCloudflare(ctx, site.Domain) {
		return "https://" + site.Domain
	}
	return "http://" + site.Domain
}

// hasLaravelFrontendChanges mendeteksi apakah aset Vite/Tailwind perlu dibangun ulang saat re-deploy
func hasLaravelFrontendChanges(ctx context.Context, projectDir, oldCommit, newCommit string) bool {
	if _, err := os.Stat(filepath.Join(projectDir, "package.json")); err != nil {
		return false
	}
	// Jika manifest produksi belum pernah ada, wajib build
	if _, err := os.Stat(filepath.Join(projectDir, "public", "build", "manifest.json")); err != nil {
		return true
	}
	if oldCommit == "" || newCommit == "" || oldCommit == newCommit {
		return false
	}
	out, err := exec.CommandContext(ctx, "git", "-C", projectDir, "diff", "--name-only", oldCommit, newCommit).Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.TrimSpace(line)
		if f == "" {
			continue
		}
		if f == "package.json" || f == "package-lock.json" ||
			strings.HasPrefix(f, "resources/") ||
			strings.HasPrefix(f, "vite.config") ||
			strings.HasPrefix(f, "tailwind.config") {
			return true
		}
	}
	return false
}

// siteDeployOptions mengatur perilaku eksekusi deploy situs (manual, webhook, atau rollback)
type siteDeployOptions struct {
	Trigger         string // manual | webhook | rollback
	CustomCommand   string
	RollbackCommit  string
	RunComposer     bool
	RunMigrate      bool
	ClearCache      bool
	AutoOptimize    bool
	MaintenanceMode bool
	RebuildAssets   bool
}

// siteDeployResult merangkum hasil eksekusi deploy atau rollback situs beserta uji kesehatan HTTP pasca deploy
type siteDeployResult struct {
	Domain          string
	Branch          string
	Repo            string
	OldCommit       string
	NewCommit       string
	CommitMessage   string
	Updated         bool
	DurationMs      int64
	Output          string
	HealthOK        bool
	HealthCode      int
	HealthLatencyMs int64
	HealthSummary   string
}

// runLaravelProductionOptimize membersihkan cache lama lalu membangun cache produksi Laravel & Filament (v3/v4)
func runLaravelProductionOptimize(ctx context.Context, projectDir, phpBin string, execEnv []string, autoOptimize bool, appendLog func(string, string)) {
	optClear := exec.CommandContext(ctx, phpBin, "artisan", "optimize:clear")
	optClear.Dir = projectDir
	optClear.Env = execEnv
	outClear, _ := optClear.CombinedOutput()
	appendLog(phpBin+" artisan optimize:clear", string(outClear))

	// Pastikan symlink storage tersedia
	linkCmd := exec.CommandContext(ctx, phpBin, "artisan", "storage:link", "--force")
	linkCmd.Dir = projectDir
	linkCmd.Env = execEnv
	_, _ = linkCmd.CombinedOutput()

	isFilament := false
	if compBytes, err := os.ReadFile(filepath.Join(projectDir, "composer.json")); err == nil &&
		strings.Contains(strings.ToLower(string(compBytes)), "filament/filament") {
		isFilament = true
		assetsCmd := exec.CommandContext(ctx, phpBin, "artisan", "filament:assets")
		assetsCmd.Dir = projectDir
		assetsCmd.Env = execEnv
		assetsOut, _ := assetsCmd.CombinedOutput()
		if strings.TrimSpace(string(assetsOut)) != "" {
			appendLog(phpBin+" artisan filament:assets", string(assetsOut))
		}
	}

	if autoOptimize {
		// Bangun cache config, route, events, dan view Laravel untuk performa tinggi di ARM64
		optCmd := exec.CommandContext(ctx, phpBin, "artisan", "optimize")
		optCmd.Dir = projectDir
		optCmd.Env = execEnv
		optOut, optErr := optCmd.CombinedOutput()
		if optErr != nil {
			// Fallback aman: bila route mengandung closure, tetap cache config & view
			cfgCmd := exec.CommandContext(ctx, phpBin, "artisan", "config:cache")
			cfgCmd.Dir = projectDir
			cfgCmd.Env = execEnv
			cfgOut, _ := cfgCmd.CombinedOutput()
			viewCmd := exec.CommandContext(ctx, phpBin, "artisan", "view:cache")
			viewCmd.Dir = projectDir
			viewCmd.Env = execEnv
			viewOut, _ := viewCmd.CombinedOutput()
			appendLog("php artisan optimize (fallback config+view cache)", string(optOut)+"\n"+string(cfgOut)+"\n"+string(viewOut))
		} else {
			appendLog(phpBin+" artisan optimize", string(optOut))
		}

		// Cache komponen Blade & ikon SVG Filament (percepatan 2-5x pada server RAM kecil)
		if isFilament {
			filOpt := exec.CommandContext(ctx, phpBin, "artisan", "filament:optimize")
			filOpt.Dir = projectDir
			filOpt.Env = execEnv
			filOut, _ := filOpt.CombinedOutput()
			if strings.TrimSpace(string(filOut)) != "" {
				appendLog(phpBin+" artisan filament:optimize", string(filOut))
			}
		}
	}
}

// fixSiteWebPermissions merapikan kepemilikan www-data, izin ketat .env (0640), dan izin tulis storage/cache
func fixSiteWebPermissions(ctx context.Context, projectDir string, site *store.Site) {
	if runtime.GOOS == "windows" {
		return
	}
	_ = exec.CommandContext(ctx, "chown", "-R", "www-data:www-data", projectDir).Run()
	envPath := filepath.Join(projectDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		_ = os.Chmod(envPath, 0o640)
	}
	for _, writableSub := range []string{"storage", "bootstrap/cache"} {
		subPath := filepath.Join(projectDir, writableSub)
		if _, err := os.Stat(subPath); err == nil {
			_ = exec.CommandContext(ctx, "chmod", "-R", "775", subPath).Run()
		}
	}
	if site != nil && site.Kind == "php" && site.PHPVersion != "" {
		_ = exec.CommandContext(ctx, "systemctl", "reload", fmt.Sprintf("php%s-fpm", site.PHPVersion)).Run()
	}
}

// executeSiteDeploy menjalankan pull/rollback aman dengan kunci eksklusif per situs, maintenance mode, anti-OOM build, dan pencatatan riwayat SQLite
func (s *Server) executeSiteDeploy(ctx context.Context, site *store.Site, opts siteDeployOptions) (*siteDeployResult, error) {
	lock := getSiteDeployLock(site.ID)
	lock.Lock()
	defer lock.Unlock()

	started := time.Now()
	projectDir := resolveSiteProjectDir(site)
	hasGit, branch, repoSlug, oldCommit := inspectProjectGitMeta(projectDir)
	if !hasGit {
		return nil, fmt.Errorf("situs ini tidak memiliki repositori Git (.git tidak ditemukan pada direktori proyek)")
	}
	if branch == "" || branch == "HEAD" {
		branch = "main"
	}

	dctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Pastikan safe.directory dan credential helper aktif
	s.syncGitGlobalCredentials(dctx, projectDir)

	token, _ := s.store.GetSetting(dctx, "github_token")
	origRemoteBytes, _ := exec.CommandContext(dctx, "git", "-C", projectDir, "remote", "get-url", "origin").Output()
	origRemoteURL := strings.TrimSpace(string(origRemoteBytes))
	cleanRemoteURL := origRemoteURL
	if strings.Contains(cleanRemoteURL, "@github.com/") {
		if u, err := url.Parse(cleanRemoteURL); err == nil {
			u.User = nil
			cleanRemoteURL = u.String()
		}
	}
	if token != "" && strings.HasPrefix(cleanRemoteURL, "https://github.com/") {
		authRemoteURL := strings.Replace(cleanRemoteURL, "https://github.com/", "https://x-access-token:"+token+"@github.com/", 1)
		_ = exec.CommandContext(dctx, "git", "-C", projectDir, "remote", "set-url", "origin", authRemoteURL).Run()
		defer func() {
			_ = exec.CommandContext(context.Background(), "git", "-C", projectDir, "remote", "set-url", "origin", cleanRemoteURL).Run()
		}()
	}

	// Cadangkan .env lokal di memori agar tidak tertimpa/hilang selama pull atau rollback
	envPath := filepath.Join(projectDir, ".env")
	envBackup, envErr := os.ReadFile(envPath)
	restoreEnv := func() {
		if envErr == nil && len(envBackup) > 0 {
			_ = os.WriteFile(envPath, envBackup, 0o640)
		}
	}

	var logBuilder strings.Builder
	appendStepLog := func(title, out string) {
		clean := strings.TrimSpace(out)
		if token != "" {
			clean = strings.ReplaceAll(clean, token, "***")
		}
		logBuilder.WriteString("==> " + title + "\n")
		if clean != "" {
			logBuilder.WriteString(clean + "\n\n")
		} else {
			logBuilder.WriteString("OK\n\n")
		}
	}

	_ = exec.CommandContext(dctx, "git", "-C", projectDir, "config", "core.fileMode", "false").Run()

	phpBin := resolvePHPBinary(site.PHPVersion)
	compPrefix := resolveComposerCommand(phpBin)
	execEnv := fullRuntimeExecEnv()

	_, hasArtisanErr := os.Stat(filepath.Join(projectDir, "artisan"))
	hasArtisan := hasArtisanErr == nil

	// Aktifkan maintenance mode singkat Laravel selama proses update agar pengguna tidak menerima error 500 setengah jalan
	inMaintenance := false
	if runtime.GOOS != "windows" && hasArtisan && opts.MaintenanceMode {
		downCmd := exec.CommandContext(dctx, phpBin, "artisan", "down", "--retry=15")
		downCmd.Dir = projectDir
		downCmd.Env = execEnv
		if out, err := downCmd.CombinedOutput(); err == nil {
			inMaintenance = true
			appendStepLog(phpBin+" artisan down --retry=15", string(out))
		}
	}
	defer func() {
		if inMaintenance {
			upCtx, upCancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer upCancel()
			upCmd := exec.CommandContext(upCtx, phpBin, "artisan", "up")
			upCmd.Dir = projectDir
			upCmd.Env = execEnv
			_ = upCmd.Run()
		}
	}()

	recordHistory := func(status, newCommit, commitMsg string, err error) {
		dur := time.Since(started).Milliseconds()
		outStr := strings.TrimSpace(logBuilder.String())
		if err != nil && !strings.Contains(outStr, err.Error()) {
			outStr = strings.TrimSpace(outStr + "\nERROR: " + err.Error())
		}
		_, _ = s.store.RecordSiteDeployment(context.Background(), &store.SiteDeployment{
			SiteID:        site.ID,
			Trigger:       opts.Trigger,
			Branch:        branch,
			OldCommit:     oldCommit,
			NewCommit:     newCommit,
			CommitMessage: commitMsg,
			Status:        status,
			DurationMs:    dur,
			Output:        outStr,
		})
	}

	if opts.RollbackCommit != "" {
		// Mode 1-Klik Rollback ke commit tertentu
		fetchCmd := exec.CommandContext(dctx, "git", "-C", projectDir, "fetch", "origin", branch)
		fetchCmd.Env = execEnv
		_, _ = fetchCmd.CombinedOutput()

		resetCmd := exec.CommandContext(dctx, "git", "-C", projectDir, "reset", "--hard", opts.RollbackCommit)
		resetCmd.Env = execEnv
		resetOut, resetErr := resetCmd.CombinedOutput()
		restoreEnv()
		if resetErr != nil {
			cleanErr := strings.TrimSpace(string(resetOut))
			if token != "" {
				cleanErr = strings.ReplaceAll(cleanErr, token, "***")
			}
			err := fmt.Errorf("rollback ke commit %s gagal: %s", opts.RollbackCommit, cleanErr)
			recordHistory("failed", oldCommit, "", err)
			return nil, err
		}
		appendStepLog("git reset --hard "+opts.RollbackCommit, string(resetOut))

		if runtime.GOOS != "windows" {
			if _, err := os.Stat(filepath.Join(projectDir, "composer.json")); err == nil && opts.RunComposer {
				cCmd := exec.CommandContext(dctx, "sh", "-c", compPrefix+" install --no-interaction --prefer-dist --optimize-autoloader || "+compPrefix+" install --no-interaction --prefer-dist --optimize-autoloader --ignore-platform-reqs")
				cCmd.Dir = projectDir
				cCmd.Env = execEnv
				cOut, _ := cCmd.CombinedOutput()
				appendStepLog(compPrefix+" install", string(cOut))
			}
			if hasArtisan {
				sanitizeFilamentShieldPolicies(projectDir)
				runLaravelProductionOptimize(dctx, projectDir, phpBin, execEnv, opts.AutoOptimize, appendStepLog)
				qCmd := exec.CommandContext(dctx, phpBin, "artisan", "queue:restart")
				qCmd.Dir = projectDir
				qCmd.Env = execEnv
				_, _ = qCmd.CombinedOutput()
			} else if site.Kind == "node" || site.Kind == "go" || site.Kind == "python" || site.Kind == "static" {
				s.buildAndStartNonPHPProject(dctx, site, projectDir, execEnv, appendStepLog)
			}
			fixSiteWebPermissions(dctx, projectDir, site)
		}
	} else if opts.CustomCommand != "" && runtime.GOOS != "windows" {
		// Mode perintah kustom dari modal Git Pull
		shCmd := exec.CommandContext(dctx, "bash", "-c", opts.CustomCommand)
		shCmd.Dir = projectDir
		shCmd.Env = execEnv
		shOut, shErr := shCmd.CombinedOutput()
		restoreEnv()
		if shErr != nil {
			cleanErr := strings.TrimSpace(string(shOut))
			if token != "" {
				cleanErr = strings.ReplaceAll(cleanErr, token, "***")
			}
			err := fmt.Errorf("eksekusi perintah gagal: %s", cleanErr)
			recordHistory("failed", oldCommit, "", err)
			return nil, err
		}
		appendStepLog(opts.CustomCommand, string(shOut))
		if hasArtisan {
			sanitizeFilamentShieldPolicies(projectDir)
		}
		fixSiteWebPermissions(dctx, projectDir, site)
	} else {
		// Mode standar anti-konflik: git fetch + reset --hard origin/<branch> (kebal konflik berkas policy hasil sanitasi) dengan .env selalu dipulihkan
		fetchCmd := exec.CommandContext(dctx, "git", "-C", projectDir, "fetch", "origin", branch)
		fetchCmd.Env = execEnv
		fetchOut, fetchErr := fetchCmd.CombinedOutput()
		if fetchErr != nil {
			restoreEnv()
			cleanErr := strings.TrimSpace(string(fetchOut))
			if token != "" {
				cleanErr = strings.ReplaceAll(cleanErr, token, "***")
			}
			err := fmt.Errorf("git fetch origin %s gagal: %s", branch, cleanErr)
			recordHistory("failed", oldCommit, "", err)
			return nil, err
		}
		resetCmd := exec.CommandContext(dctx, "git", "-C", projectDir, "-c", "core.fileMode=false", "reset", "--hard", "origin/"+branch)
		resetCmd.Env = execEnv
		resetOut, resetErr := resetCmd.CombinedOutput()
		restoreEnv()
		if resetErr != nil {
			cleanErr := strings.TrimSpace(string(resetOut))
			if token != "" {
				cleanErr = strings.ReplaceAll(cleanErr, token, "***")
			}
			err := fmt.Errorf("git reset --hard origin/%s gagal: %s", branch, cleanErr)
			recordHistory("failed", oldCommit, "", err)
			return nil, err
		}
		appendStepLog("git fetch && reset --hard origin/"+branch, strings.TrimSpace(string(fetchOut)+"\n"+string(resetOut)))

		_, _, _, pulledCommit := inspectProjectGitMeta(projectDir)

		if runtime.GOOS != "windows" {
			_, hasComposer := os.Stat(filepath.Join(projectDir, "composer.json"))

			if hasComposer == nil && opts.RunComposer {
				compCmd := exec.CommandContext(dctx, "sh", "-c", compPrefix+" install --no-interaction --prefer-dist --optimize-autoloader")
				compCmd.Dir = projectDir
				compCmd.Env = execEnv
				compOut, err := compCmd.CombinedOutput()
				if err != nil {
					compCmd2 := exec.CommandContext(dctx, "sh", "-c", compPrefix+" install --no-interaction --prefer-dist --optimize-autoloader --ignore-platform-reqs")
					compCmd2.Dir = projectDir
					compCmd2.Env = execEnv
					compOut2, _ := compCmd2.CombinedOutput()
					appendStepLog(compPrefix+" install (--ignore-platform-reqs)", string(compOut)+"\n"+string(compOut2))
				} else {
					appendStepLog(compPrefix+" install", string(compOut))
				}
			}

			if hasArtisan {
				if opts.RebuildAssets && hasLaravelFrontendChanges(dctx, projectDir, oldCommit, pulledCommit) {
					npmScript := "if [ -f package-lock.json ]; then (npm ci --no-audit --no-fund || npm install --no-audit --no-fund); else npm install --no-audit --no-fund; fi && npm run build"
					npmCmd := exec.CommandContext(dctx, "sh", "-c", npmScript)
					npmCmd.Dir = projectDir
					npmCmd.Env = execEnv
					npmOut, _ := npmCmd.CombinedOutput()
					appendStepLog("npm build (aset Laravel/Filament berubah)", string(npmOut))
				}

				if opts.RunMigrate {
					migCmd := exec.CommandContext(dctx, phpBin, "artisan", "migrate", "--force")
					migCmd.Dir = projectDir
					migCmd.Env = execEnv
					migOut, _ := migCmd.CombinedOutput()
					appendStepLog(phpBin+" artisan migrate --force", string(migOut))
				}

				sanitizeFilamentShieldPolicies(projectDir)

				if opts.ClearCache || opts.AutoOptimize {
					runLaravelProductionOptimize(dctx, projectDir, phpBin, execEnv, opts.AutoOptimize, appendStepLog)
				}

				// Beri sinyal queue worker agar memuat ulang kode terbaru secara mulus tanpa memutus job aktif
				qCmd := exec.CommandContext(dctx, phpBin, "artisan", "queue:restart")
				qCmd.Dir = projectDir
				qCmd.Env = execEnv
				if qOut, qErr := qCmd.CombinedOutput(); qErr == nil && strings.TrimSpace(string(qOut)) != "" {
					appendStepLog(phpBin+" artisan queue:restart", string(qOut))
				}
			} else if site.Kind == "node" || site.Kind == "go" || site.Kind == "python" || site.Kind == "static" {
				// Pipeline build & restart universal untuk Node.js (PM2), Static SPA (dist/), Golang (systemd), & Python (.venv + systemd)
				s.buildAndStartNonPHPProject(dctx, site, projectDir, execEnv, appendStepLog)
			}

			fixSiteWebPermissions(dctx, projectDir, site)
		}
	}

	if inMaintenance {
		upCmd := exec.CommandContext(dctx, phpBin, "artisan", "up")
		upCmd.Dir = projectDir
		upCmd.Env = execEnv
		upOut, _ := upCmd.CombinedOutput()
		inMaintenance = false
		appendStepLog(phpBin+" artisan up", string(upOut))
	}

	// Uji kesehatan HTTP origin lokal pasca-deploy untuk mendeteksi error 500/502 secara instan
	hc := runPostDeploySmokeTest(dctx, site)
	appendStepLog("Post-Deploy HTTP Smoke Test", hc.Summary)

	_, branchNow, repoNow, newCommit := inspectProjectGitMeta(projectDir)
	if branchNow != "" {
		branch = branchNow
	}
	if repoNow != "" {
		repoSlug = repoNow
	}
	commitMsgBytes, _ := exec.CommandContext(dctx, "git", "-C", projectDir, "log", "-1", "--pretty=%s").Output()
	commitMsg := strings.TrimSpace(string(commitMsgBytes))

	deployStatus := "success"
	if !hc.OK {
		deployStatus = "warning"
	}
	recordHistory(deployStatus, newCommit, commitMsg, nil)

	return &siteDeployResult{
		Domain:          site.Domain,
		Branch:          branch,
		Repo:            repoSlug,
		OldCommit:       oldCommit,
		NewCommit:       newCommit,
		CommitMessage:   commitMsg,
		Updated:         oldCommit != "" && newCommit != "" && oldCommit != newCommit,
		DurationMs:      time.Since(started).Milliseconds(),
		Output:          strings.TrimSpace(logBuilder.String()),
		HealthOK:        hc.OK,
		HealthCode:      hc.StatusCode,
		HealthLatencyMs: hc.LatencyMs,
		HealthSummary:   hc.Summary,
	}, nil
}

// buildPublicWebhookURL menyusun URL webhook lengkap sesuai origin panel yang sedang diakses admin
func buildPublicWebhookURL(r *http.Request, siteID int64, secret string) string {
	scheme := "https"
	if r.TLS == nil {
		if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
			scheme = proto
		} else {
			scheme = "http"
		}
	}
	host := r.Host
	if host == "" {
		host = "localhost:1405"
	}
	return fmt.Sprintf("%s://%s/api/webhooks/git/%d/%s", scheme, host, siteID, secret)
}

// findSiteLaravelSchedulerCron mencari apakah cron schedule:run milik situs sudah terdaftar di Cron Manager
func (s *Server) findSiteLaravelSchedulerCron(ctx context.Context, domain string) *store.CronJob {
	targetName := "Laravel Scheduler: " + domain
	jobs, err := s.store.ListCronJobs(ctx)
	if err != nil {
		return nil
	}
	for i := range jobs {
		if jobs[i].Name == targetName {
			return &jobs[i]
		}
	}
	return nil
}

// handleGetSiteDeployInfo mengembalikan konfigurasi Webhook Push-to-Deploy, status Scheduler Laravel, dan riwayat deploy situs
func (s *Server) handleGetSiteDeployInfo(w http.ResponseWriter, r *http.Request) {
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

	cfg, err := s.store.GetOrInitSiteDeployConfig(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memuat konfigurasi deploy: " + err.Error()})
		return
	}
	deployments, err := s.store.ListSiteDeployments(r.Context(), id, 20)
	if err != nil {
		deployments = []store.SiteDeployment{}
	}

	projectDir := resolveSiteProjectDir(site)
	hasGit, branch, _, currentCommit := inspectProjectGitMeta(projectDir)
	_, errArtisan := os.Stat(filepath.Join(projectDir, "artisan"))
	isLaravel := errArtisan == nil
	isFilament := false
	if compBytes, err := os.ReadFile(filepath.Join(projectDir, "composer.json")); err == nil &&
		strings.Contains(strings.ToLower(string(compBytes)), "filament/filament") {
		isFilament = true
		isLaravel = true
	}

	schedJob := s.findSiteLaravelSchedulerCron(r.Context(), site.Domain)
	schedEnabled := schedJob != nil && schedJob.Enabled
	schedID := int64(0)
	if schedJob != nil {
		schedID = schedJob.ID
	}

	procMgr, procUnit, procState, queueEnabled := inspectSiteProcess(r.Context(), site)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                   true,
		"config":               cfg,
		"webhook_url":          buildPublicWebhookURL(r, id, cfg.WebhookSecret),
		"has_git":              hasGit,
		"branch":               branch,
		"current_commit":       currentCommit,
		"is_laravel":           isLaravel,
		"is_filament":          isFilament,
		"php_binary":           resolvePHPBinary(site.PHPVersion),
		"scheduler_enabled":    schedEnabled,
		"scheduler_cron_id":    schedID,
		"queue_worker_enabled": queueEnabled,
		"queue_unit":           siteQueueUnitName(site.Domain),
		"process_manager":      procMgr,
		"process_unit":         procUnit,
		"process_state":        procState,
		"framework":            site.Framework,
		"deployments":          deployments,
	})
}

// handleUpdateSiteDeployConfig memperbarui konfigurasi Webhook Push-to-Deploy dan opsi optimasi/maintenance situs
func (s *Server) handleUpdateSiteDeployConfig(w http.ResponseWriter, r *http.Request) {
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

	cfg, err := s.store.GetOrInitSiteDeployConfig(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var req struct {
		WebhookEnabled  *bool  `json:"webhook_enabled"`
		AutoOptimize    *bool  `json:"auto_optimize"`
		MaintenanceMode *bool  `json:"maintenance_mode"`
		RebuildAssets   *bool  `json:"rebuild_assets"`
		Branch          string `json:"branch"`
		RotateSecret    bool   `json:"rotate_secret"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}

	if req.WebhookEnabled != nil {
		cfg.WebhookEnabled = *req.WebhookEnabled
	}
	if req.AutoOptimize != nil {
		cfg.AutoOptimize = *req.AutoOptimize
	}
	if req.MaintenanceMode != nil {
		cfg.MaintenanceMode = *req.MaintenanceMode
	}
	if req.RebuildAssets != nil {
		cfg.RebuildAssets = *req.RebuildAssets
	}
	if req.Branch != "" {
		cfg.Branch = strings.TrimSpace(req.Branch)
	}
	if req.RotateSecret {
		cfg.WebhookSecret = store.GenerateDeploySecret()
	}

	if err := s.store.SaveSiteDeployConfig(r.Context(), cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menyimpan konfigurasi deploy: " + err.Error()})
		return
	}

	s.auditSite(r, "site.deploy.config", site.Domain, true,
		fmt.Sprintf("webhook=%v optimize=%v maintenance=%v rotate=%v", cfg.WebhookEnabled, cfg.AutoOptimize, cfg.MaintenanceMode, req.RotateSecret))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"config":      cfg,
		"webhook_url": buildPublicWebhookURL(r, id, cfg.WebhookSecret),
		"message":     "Konfigurasi auto-deploy berhasil disimpan",
	})
}

// handleSiteDeployRollback memulihkan situs ke commit Git sebelumnya secara 1-klik dengan proteksi .env & re-optimize
func (s *Server) handleSiteDeployRollback(w http.ResponseWriter, r *http.Request) {
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
		Commit      string `json:"commit"`
		RunComposer *bool  `json:"run_composer"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	commit := strings.TrimSpace(req.Commit)
	if !gitCommitHashRe.MatchString(commit) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hash commit tidak valid (wajib 6-40 karakter hex)"})
		return
	}

	cfg, _ := s.store.GetOrInitSiteDeployConfig(r.Context(), id)
	runComp := true
	if req.RunComposer != nil {
		runComp = *req.RunComposer
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	res, err := s.executeSiteDeploy(r.Context(), site, siteDeployOptions{
		Trigger:         "rollback",
		RollbackCommit:  commit,
		RunComposer:     runComp,
		AutoOptimize:    cfg == nil || cfg.AutoOptimize,
		MaintenanceMode: cfg == nil || cfg.MaintenanceMode,
	})
	if err != nil {
		s.auditSite(r, "site.deploy.rollback", site.Domain, false, err.Error())
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	s.auditSite(r, "site.deploy.rollback", site.Domain, true, fmt.Sprintf("commit=%s->%s", res.OldCommit, res.NewCommit))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                true,
		"domain":            res.Domain,
		"branch":            res.Branch,
		"repo":              res.Repo,
		"old_commit":        res.OldCommit,
		"new_commit":        res.NewCommit,
		"commit_message":    res.CommitMessage,
		"updated":           res.Updated,
		"duration_ms":       res.DurationMs,
		"output":            res.Output,
		"health_ok":         res.HealthOK,
		"health_code":       res.HealthCode,
		"health_latency_ms": res.HealthLatencyMs,
		"health_summary":    res.HealthSummary,
		"message":           fmt.Sprintf("Rollback %s ke #%s berhasil", res.Domain, res.NewCommit),
	})
}

// handleGitWebhookDeploy menerima trigger Push-to-Deploy dari GitHub / GitLab dengan verifikasi secret 256-bit & HMAC-SHA256
func (s *Server) handleGitWebhookDeploy(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID situs tidak valid"})
		return
	}
	secret := strings.TrimSpace(r.PathValue("secret"))
	if secret == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "secret webhook kosong"})
		return
	}

	site, err := s.store.GetSite(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "situs tidak ditemukan"})
		return
	}

	cfg, err := s.store.GetOrInitSiteDeployConfig(r.Context(), id)
	if err != nil || !cfg.WebhookEnabled {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "webhook auto-deploy belum diaktifkan untuk situs ini"})
		return
	}

	// Bandingkan secret URL secara constant-time untuk mencegah timing attack
	if subtle.ConstantTimeCompare([]byte(secret), []byte(cfg.WebhookSecret)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "secret webhook tidak valid"})
		return
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gagal membaca payload webhook"})
		return
	}

	// Verifikasi HMAC-SHA256 jika GitHub mengirimkan header X-Hub-Signature-256
	if sig := strings.TrimSpace(r.Header.Get("X-Hub-Signature-256")); sig != "" {
		mac := hmac.New(sha256.New, []byte(cfg.WebhookSecret))
		_, _ = mac.Write(bodyBytes)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "tanda tangan HMAC X-Hub-Signature-256 tidak valid"})
			return
		}
	}
	// Verifikasi token jika GitLab mengirimkan X-Gitlab-Token
	if glTok := strings.TrimSpace(r.Header.Get("X-Gitlab-Token")); glTok != "" {
		if subtle.ConstantTimeCompare([]byte(glTok), []byte(cfg.WebhookSecret)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "header X-Gitlab-Token tidak valid"})
			return
		}
	}

	// Tangani event ping dari GitHub saat pertama kali mendaftarkan webhook
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-GitHub-Event")), "ping") {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"domain":  site.Domain,
			"message": "pong — webhook FikPanel aktif",
		})
		return
	}

	projectDir := resolveSiteProjectDir(site)
	hasGit, activeBranch, _, _ := inspectProjectGitMeta(projectDir)
	if !hasGit {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "direktori situs bukan repositori Git"})
		return
	}
	targetBranch := strings.TrimSpace(cfg.Branch)
	if targetBranch == "" {
		targetBranch = activeBranch
	}

	// Periksa kecocokan branch push (jika payload menyertakan field "ref")
	var payload struct {
		Ref string `json:"ref"`
	}
	if len(bodyBytes) > 0 && json.Unmarshal(bodyBytes, &payload) == nil && payload.Ref != "" && targetBranch != "" {
		pushedBranch := strings.TrimPrefix(payload.Ref, "refs/heads/")
		if pushedBranch != targetBranch {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"ok":      true,
				"skipped": true,
				"reason":  fmt.Sprintf("push ke branch %q diabaikan (target aktif: %q)", pushedBranch, targetBranch),
			})
			return
		}
	}

	// Eksekusi deploy di latar belakang agar HTTP webhook GitHub tidak timeout > 10 detik
	siteCopy := *site
	cfgCopy := *cfg
	go func() {
		bg := s.bgCtx
		if bg == nil {
			bg = context.Background()
		}
		res, dErr := s.executeSiteDeploy(bg, &siteCopy, siteDeployOptions{
			Trigger:         "webhook",
			RunComposer:     true,
			RunMigrate:      true,
			ClearCache:      true,
			AutoOptimize:    cfgCopy.AutoOptimize,
			MaintenanceMode: cfgCopy.MaintenanceMode,
			RebuildAssets:   cfgCopy.RebuildAssets,
		})
		if dErr != nil {
			_ = s.store.RecordAudit(bg, "webhook", "site.deploy.webhook", siteCopy.Domain, false, dErr.Error(), nil)
		} else {
			_ = s.store.RecordAudit(bg, "webhook", "site.deploy.webhook", siteCopy.Domain, true,
				fmt.Sprintf("branch=%s commit=%s->%s (%dms)", res.Branch, res.OldCommit, res.NewCommit, res.DurationMs), nil)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"ok":      true,
		"queued":  true,
		"domain":  site.Domain,
		"branch":  targetBranch,
		"message": "Auto-deploy dijadwalkan di latar belakang",
	})
}

// handleToggleLaravelScheduler mengaktifkan atau menonaktifkan cron '* * * * * php artisan schedule:run' di Cron Manager
func (s *Server) handleToggleLaravelScheduler(w http.ResponseWriter, r *http.Request) {
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
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}

	projectDir := resolveSiteProjectDir(site)
	if _, err := os.Stat(filepath.Join(projectDir, "artisan")); err != nil && runtime.GOOS != "windows" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file artisan Laravel tidak ditemukan pada direktori proyek"})
		return
	}

	existing := s.findSiteLaravelSchedulerCron(r.Context(), site.Domain)
	if req.Enabled {
		phpBin := resolvePHPBinary(site.PHPVersion)
		cmd := fmt.Sprintf("cd %q && %s artisan schedule:run >> /dev/null 2>&1", projectDir, phpBin)
		if existing != nil {
			existing.Command = cmd
			existing.Enabled = true
			if err := s.store.UpdateCronJob(r.Context(), existing); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memperbarui cron scheduler: " + err.Error()})
				return
			}
		} else {
			job := &store.CronJob{
				Name:         "Laravel Scheduler: " + site.Domain,
				ScheduleType: "custom",
				Spec:         "* * * * *",
				Command:      cmd,
				Enabled:      true,
				SaveLog:      false,
				TimeoutSec:   120,
			}
			newID, err := s.store.CreateCronJob(r.Context(), job)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membuat cron scheduler: " + err.Error()})
				return
			}
			job.ID = newID
			existing = job
		}
		s.auditSite(r, "site.laravel.scheduler", site.Domain, true, "Mengaktifkan Laravel Scheduler (* * * * *)")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":                true,
			"scheduler_enabled": true,
			"scheduler_cron_id": existing.ID,
			"message":           "Laravel Scheduler (* * * * *) aktif di Cron Manager",
		})
		return
	}

	if existing != nil {
		existing.Enabled = false
		if err := s.store.UpdateCronJob(r.Context(), existing); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menonaktifkan cron scheduler: " + err.Error()})
			return
		}
	}
	s.auditSite(r, "site.laravel.scheduler", site.Domain, true, "Menonaktifkan Laravel Scheduler")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                true,
		"scheduler_enabled": false,
		"message":           "Laravel Scheduler dinonaktifkan",
	})
}

// handleOptimizeLaravelSite menjalankan optimize:clear, optimize, filament:optimize, dan reload PHP-FPM secara 1-klik
func (s *Server) handleOptimizeLaravelSite(w http.ResponseWriter, r *http.Request) {
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
	if _, err := os.Stat(filepath.Join(projectDir, "artisan")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file artisan Laravel tidak ditemukan pada direktori proyek"})
		return
	}

	octx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	phpBin := resolvePHPBinary(site.PHPVersion)
	execEnv := append(os.Environ(),
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"COMPOSER_ALLOW_SUPERUSER=1",
	)
	var logBuf strings.Builder
	appendLog := func(title, out string) {
		logBuf.WriteString("==> " + title + "\n")
		if trimmed := strings.TrimSpace(out); trimmed != "" {
			logBuf.WriteString(trimmed + "\n\n")
		} else {
			logBuf.WriteString("OK\n\n")
		}
	}

	sanitizeFilamentShieldPolicies(projectDir)
	runLaravelProductionOptimize(octx, projectDir, phpBin, execEnv, true, appendLog)
	fixSiteWebPermissions(octx, projectDir, site)

	s.auditSite(r, "site.laravel.optimize", site.Domain, true, "Menjalankan optimasi cache produksi Laravel/Filament")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"output":  strings.TrimSpace(logBuf.String()),
		"message": "Cache produksi Laravel & Filament berhasil dioptimasi",
	})
}
