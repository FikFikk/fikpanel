package appstore

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type AppPackage struct {
	ID          string   `json:"id"`           // misal: 'nginx', 'php8.5', 'php8.3', 'mysql', 'redis', 'nodejs'
	Name        string   `json:"name"`         // 'Nginx Web Server'
	Category    string   `json:"category"`     // 'webserver', 'runtime', 'database', 'tools'
	Version     string   `json:"version"`      // '1.24.0', '8.5', '8.3'
	Description string   `json:"description"`  // deskripsi singkat
	Icon        string   `json:"icon"`         // remixicon class
	Installed   bool     `json:"installed"`
	Status      string   `json:"status"`       // 'running', 'stopped', 'not_installed'
	AptPackages []string `json:"apt_packages"` // apt packages yang diinstall
}

type GlobalPackage struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
}

type InstallTask struct {
	AppID     string `json:"app_id"`
	Status    string `json:"status"` // 'running', 'success', 'failed'
	Logs      string `json:"logs"`
	StartedAt int64  `json:"started_at"`
}

type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*InstallTask
}

func NewManager() *Manager {
	return &Manager{
		tasks: make(map[string]*InstallTask),
	}
}

func (m *Manager) Catalog(ctx context.Context) []AppPackage {
	apps := []AppPackage{
		{
			ID:          "nginx",
			Name:        "Nginx Web Server",
			Category:    "webserver",
			Version:     "1.24 / Latest",
			Description: "High-performance HTTP server & reverse proxy untuk routing domain, SSL, dan static files.",
			Icon:        "ri-server-line",
			AptPackages: []string{"nginx"},
		},
		{
			ID:          "php8.5",
			Name:        "PHP 8.5 + FPM Suite",
			Category:    "runtime",
			Version:     "8.5 / Latest",
			Description: "PHP 8.5 modern runtime lengkap dengan FastCGI Process Manager, ekstensi MySQL, GD, cURL, dan Mbstring.",
			Icon:        "ri-code-s-slash-line",
			AptPackages: []string{"php8.5-fpm", "php8.5-mysql", "php8.5-cli", "php8.5-curl", "php8.5-mbstring", "php8.5-gd", "php8.5-xml", "php8.5-zip"},
		},
		{
			ID:          "php8.3",
			Name:        "PHP 8.3 + FPM Suite",
			Category:    "runtime",
			Version:     "8.3",
			Description: "Modern PHP runtime lengkap dengan FastCGI Process Manager, ekstensi MySQL, GD, cURL, dan Mbstring.",
			Icon:        "ri-code-s-slash-line",
			AptPackages: []string{"php8.3-fpm", "php8.3-mysql", "php8.3-cli", "php8.3-curl", "php8.3-mbstring", "php8.3-gd", "php8.3-xml", "php8.3-zip"},
		},
		{
			ID:          "php8.2",
			Name:        "PHP 8.2 + FPM Suite",
			Category:    "runtime",
			Version:     "8.2",
			Description: "LTS PHP 8.2 runtime stabil untuk kompatibilitas framework Laravel dan CMS WordPress.",
			Icon:        "ri-code-s-slash-line",
			AptPackages: []string{"php8.2-fpm", "php8.2-mysql", "php8.2-cli", "php8.2-curl", "php8.2-mbstring", "php8.2-gd", "php8.2-xml", "php8.2-zip"},
		},
		{
			ID:          "mariadb",
			Name:        "MariaDB / MySQL Server",
			Category:    "database",
			Version:     "10.11 / Latest",
			Description: "Relational database server open-source drop-in replacement untuk MySQL.",
			Icon:        "ri-database-2-line",
			AptPackages: []string{"mariadb-server", "mariadb-client"},
		},
		{
			ID:          "redis",
			Name:        "Redis In-Memory Cache",
			Category:    "database",
			Version:     "7.0 / Latest",
			Description: "Ultra-fast in-memory key-value data store untuk cache session, queue worker, dan pub/sub.",
			Icon:        "ri-flashlight-line",
			AptPackages: []string{"redis-server"},
		},
		{
			ID:          "nodejs",
			Name:        "Node.js & NPM / PM2",
			Category:    "runtime",
			Version:     "20.x / 22.x LTS",
			Description: "JavaScript runtime environment untuk menjalankan aplikasi Nuxt, Next.js, Express, dan PM2.",
			Icon:        "ri-nodejs-line",
			AptPackages: []string{"nodejs", "npm"},
		},
		{
			ID:          "certbot",
			Name:        "Certbot Let's Encrypt",
			Category:    "tools",
			Version:     "2.x",
			Description: "Automated tool untuk penerbitan dan pembaruan sertifikat SSL gratis secara otomatis.",
			Icon:        "ri-shield-keyhole-line",
			AptPackages: []string{"certbot", "python3-certbot-nginx"},
		},
		{
			ID:          "fail2ban",
			Name:        "Fail2Ban Intrusion Guard",
			Category:    "tools",
			Version:     "1.0.x",
			Description: "Sistem pencegahan intrusi otomatis yang memblokir alamat IP penyerang brute-force SSH dan Nginx.",
			Icon:        "ri-Alarm-warning-line",
			AptPackages: []string{"fail2ban"},
		},
		{
			ID:          "apache2",
			Name:        "Apache HTTP Server",
			Category:    "webserver",
			Version:     "2.4 / Latest",
			Description: "Web server modular standar industri dengan dukungan berkas konfigurasi .htaccess dan modul rewrite.",
			Icon:        "ri-server-line",
			AptPackages: []string{"apache2"},
		},
		{
			ID:          "postgresql",
			Name:        "PostgreSQL Database",
			Category:    "database",
			Version:     "16 / Latest",
			Description: "Sistem basis data relasional objek canggih dengan keandalan tinggi dan konkurensi data yang kuat.",
			Icon:        "ri-database-2-line",
			AptPackages: []string{"postgresql", "postgresql-contrib"},
		},
		{
			ID:          "composer",
			Name:        "Composer PHP Manager",
			Category:    "runtime",
			Version:     "2.x / Latest",
			Description: "Manajer dependensi resmi untuk ekosistem PHP dan framework Laravel, Symfony, dan WordPress.",
			Icon:        "ri-terminal-box-line",
			AptPackages: []string{"composer"},
		},
		{
			ID:          "docker",
			Name:        "Docker CE Engine",
			Category:    "tools",
			Version:     "26.x / Latest",
			Description: "Platform kontainerisasi untuk membangun, mendistribusikan, dan menjalankan kontainer aplikasi.",
			Icon:        "ri-instance-line",
			AptPackages: []string{"docker.io"},
		},
		{
			ID:          "ufw",
			Name:        "UFW Firewall Shield",
			Category:    "tools",
			Version:     "0.36 / Latest",
			Description: "Uncomplicated Firewall untuk manajemen aturan port dan keamanan jaringan server Linux.",
			Icon:        "ri-shield-check-line",
			AptPackages: []string{"ufw"},
		},
		{
			ID:          "supervisor",
			Name:        "Supervisor Process Control",
			Category:    "tools",
			Version:     "4.x / Latest",
			Description: "Sistem kontrol proses client/server untuk menjaga worker queue dan daemon tetap hidup.",
			Icon:        "ri-cpu-line",
			AptPackages: []string{"supervisor"},
		},
		{
			ID:          "git",
			Name:        "Git Version Control",
			Category:    "tools",
			Version:     "2.x",
			Description: "Sistem kendali versi untuk deployment otomatis via git pull direktori web.",
			Icon:        "ri-git-branch-line",
			AptPackages: []string{"git"},
		},
	}

	for i, app := range apps {
		installed, running := m.checkAppStatus(ctx, app.ID)
		apps[i].Installed = installed
		if running {
			apps[i].Status = "running"
		} else if installed {
			apps[i].Status = "stopped"
		} else {
			apps[i].Status = "not_installed"
		}
	}

	return apps
}

func (m *Manager) checkAppStatus(ctx context.Context, id string) (bool, bool) {
	if runtime.GOOS != "linux" {
		return id == "nginx" || id == "php8.5" || id == "php8.3", id == "nginx" || id == "php8.5"
	}

	switch {
	case id == "nginx":
		cmd := exec.CommandContext(ctx, "which", "nginx")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "nginx").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case strings.HasPrefix(id, "php"):
		ver := strings.TrimPrefix(id, "php")
		cmd := exec.CommandContext(ctx, "which", "php"+ver)
		installed := cmd.Run() == nil
		if !installed {
			// Cek apakah socket atau direktori konfigurasi /etc/php/<ver> ada
			sock := fmt.Sprintf("/run/php/php%s-fpm.sock", ver)
			if _, err := os.Stat(sock); err == nil {
				installed = true
			} else if _, err := os.Stat(fmt.Sprintf("/etc/php/%s", ver)); err == nil {
				installed = true
			}
		}
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", fmt.Sprintf("php%s-fpm", ver)).Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case id == "mariadb":
		cmd := exec.CommandContext(ctx, "which", "mariadb")
		installed := cmd.Run() == nil
		if !installed {
			cmd = exec.CommandContext(ctx, "which", "mysql")
			installed = cmd.Run() == nil
		}
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "mariadb").Output()
			if strings.TrimSpace(string(out)) != "active" {
				out, _ = exec.CommandContext(ctx, "systemctl", "is-active", "mysql").Output()
			}
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case id == "redis":
		cmd := exec.CommandContext(ctx, "which", "redis-server")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "redis-server").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case id == "nodejs":
		cmd := exec.CommandContext(ctx, "which", "node")
		return cmd.Run() == nil, false

	case id == "certbot":
		cmd := exec.CommandContext(ctx, "which", "certbot")
		return cmd.Run() == nil, false

	case id == "fail2ban":
		cmd := exec.CommandContext(ctx, "which", "fail2ban-client")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "fail2ban").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case id == "apache2":
		cmd := exec.CommandContext(ctx, "which", "apache2")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "apache2").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case id == "postgresql":
		cmd := exec.CommandContext(ctx, "which", "psql")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "postgresql").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case id == "docker":
		cmd := exec.CommandContext(ctx, "which", "docker")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "docker").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case id == "composer":
		cmd := exec.CommandContext(ctx, "which", "composer")
		return cmd.Run() == nil, false

	case id == "ufw":
		cmd := exec.CommandContext(ctx, "which", "ufw")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "ufw").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case id == "supervisor":
		cmd := exec.CommandContext(ctx, "which", "supervisord")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "supervisor").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case id == "git":
		cmd := exec.CommandContext(ctx, "which", "git")
		return cmd.Run() == nil, false
	}

	return false, false
}

func (m *Manager) Install(appID string) error {
	m.mu.Lock()
	if task, exists := m.tasks[appID]; exists && task.Status == "running" {
		m.mu.Unlock()
		return fmt.Errorf("instalasi %s sedang berjalan", appID)
	}

	task := &InstallTask{
		AppID:     appID,
		Status:    "running",
		Logs:      fmt.Sprintf("[%s] Memulai instalasi %s...\n", time.Now().Format("15:04:05"), appID),
		StartedAt: time.Now().Unix(),
	}
	m.tasks[appID] = task
	m.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		var pkgs []string
		if strings.HasPrefix(appID, "pkg:") {
			pkgs = []string{strings.TrimPrefix(appID, "pkg:")}
		} else {
			catalog := m.Catalog(ctx)
			for _, a := range catalog {
				if a.ID == appID {
					pkgs = a.AptPackages
					break
				}
			}
			if len(pkgs) == 0 {
				pkgs = []string{appID}
			}
		}

		if runtime.GOOS != "linux" {
			time.Sleep(2 * time.Second)
			m.mu.Lock()
			task.Status = "success"
			task.Logs += "[Simulasi] Paket berhasil dipasang di lingkungan non-Linux.\n"
			m.mu.Unlock()
			return
		}

		args := append([]string{"install", "-y", "--no-install-recommends"}, pkgs...)
		cmd := exec.CommandContext(ctx, "apt-get", args...)
		cmd.Env = append(cmd.Environ(), "DEBIAN_FRONTEND=noninteractive")

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		m.mu.Lock()
		defer m.mu.Unlock()

		task.Logs += stdout.String()
		if stderr.Len() > 0 {
			task.Logs += "\n[STDERR]\n" + stderr.String()
		}

		if err != nil {
			task.Status = "failed"
			task.Logs += fmt.Sprintf("\n[GAGAL] %v\n", err)
		} else {
			task.Status = "success"
			task.Logs += fmt.Sprintf("\n[SUKSES] %s berhasil dipasang ke sistem!\n", appID)
		}
	}()

	return nil
}

// Uninstall mencopot paket software sistem
func (m *Manager) Uninstall(appID string) error {
	m.mu.Lock()
	if task, exists := m.tasks[appID]; exists && task.Status == "running" {
		m.mu.Unlock()
		return fmt.Errorf("operasi untuk %s sedang berjalan", appID)
	}

	task := &InstallTask{
		AppID:     appID,
		Status:    "running",
		Logs:      fmt.Sprintf("[%s] Memulai pencopotan paket %s...\n", time.Now().Format("15:04:05"), appID),
		StartedAt: time.Now().Unix(),
	}
	m.tasks[appID] = task
	m.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		var pkgs []string
		if strings.HasPrefix(appID, "pkg:") {
			pkgs = []string{strings.TrimPrefix(appID, "pkg:")}
		} else {
			catalog := m.Catalog(ctx)
			for _, a := range catalog {
				if a.ID == appID {
					pkgs = a.AptPackages
					break
				}
			}
			if len(pkgs) == 0 {
				pkgs = []string{appID}
			}
		}

		if runtime.GOOS != "linux" {
			time.Sleep(2 * time.Second)
			m.mu.Lock()
			task.Status = "success"
			task.Logs += "[Simulasi] Paket berhasil dicopot di lingkungan non-Linux.\n"
			m.mu.Unlock()
			return
		}

		args := append([]string{"remove", "-y"}, pkgs...)
		cmd := exec.CommandContext(ctx, "apt-get", args...)
		cmd.Env = append(cmd.Environ(), "DEBIAN_FRONTEND=noninteractive")

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		m.mu.Lock()
		defer m.mu.Unlock()

		task.Logs += stdout.String()
		if stderr.Len() > 0 {
			task.Logs += "\n[STDERR]\n" + stderr.String()
		}

		if err != nil {
			task.Status = "failed"
			task.Logs += fmt.Sprintf("\n[GAGAL] %v\n", err)
		} else {
			task.Status = "success"
			task.Logs += fmt.Sprintf("\n[SUKSES] Paket %s berhasil dicopot dari sistem!\n", appID)
		}
	}()

	return nil
}

// SearchGlobalPackages mencari paket software di repositori global OS (APT)
func (m *Manager) SearchGlobalPackages(ctx context.Context, query string) ([]GlobalPackage, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	// Sanitasi query agar hanya mengandung karakter aman
	var safeBuilder strings.Builder
	for _, r := range query {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			safeBuilder.WriteRune(r)
		}
	}
	safeQuery := safeBuilder.String()
	if safeQuery == "" {
		return nil, nil
	}

	if runtime.GOOS != "linux" {
		// Mock data untuk lingkungan development non-Linux
		mocks := []GlobalPackage{
			{Name: "composer", Description: "Dependency Manager for PHP", Installed: false},
			{Name: "htop", Description: "Interactive processes viewer for Linux", Installed: true},
			{Name: "ffmpeg", Description: "Tools for transcoding, streaming and playing multimedia files", Installed: false},
			{Name: "postgresql", Description: "Object-relational SQL database server", Installed: false},
			{Name: "supervisor", Description: "System for controlling process state on UNIX-like OS", Installed: false},
			{Name: "python3-pip", Description: "Python package installer", Installed: true},
			{Name: "unzip", Description: "De-archiver for .zip files", Installed: true},
		}
		var matched []GlobalPackage
		qLower := strings.ToLower(safeQuery)
		for _, pkg := range mocks {
			if strings.Contains(strings.ToLower(pkg.Name), qLower) || strings.Contains(strings.ToLower(pkg.Description), qLower) {
				matched = append(matched, pkg)
			}
		}
		if len(matched) == 0 {
			matched = append(matched, GlobalPackage{
				Name:        safeQuery,
				Description: fmt.Sprintf("Paket sistem Linux %s dari repositori", safeQuery),
				Installed:   false,
			})
		}
		return matched, nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "apt-cache", "search", "--names-only", safeQuery)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		cmdFallback := exec.CommandContext(cmdCtx, "apt-cache", "search", safeQuery)
		out, _ = cmdFallback.Output()
	}

	var results []GlobalPackage
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	count := 0
	for scanner.Scan() && count < 30 {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) == 0 {
			continue
		}
		pkgName := strings.TrimSpace(parts[0])
		desc := ""
		if len(parts) > 1 {
			desc = strings.TrimSpace(parts[1])
		}

		chkCmd := exec.CommandContext(ctx, "dpkg-query", "-W", "-f=${Status}", pkgName)
		chkOut, _ := chkCmd.Output()
		installed := strings.Contains(string(chkOut), "ok installed")

		results = append(results, GlobalPackage{
			Name:        pkgName,
			Description: desc,
			Installed:   installed,
		})
		count++
	}

	return results, nil
}

func (m *Manager) GetTask(appID string) *InstallTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[appID]
}

