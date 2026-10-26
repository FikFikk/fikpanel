package appstore

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type AppPackage struct {
	ID          string   `json:"id"`           // misal: 'nginx', 'php8.3', 'mysql', 'redis', 'nodejs'
	Name        string   `json:"name"`         // 'Nginx Web Server'
	Category    string   `json:"category"`     // 'webserver', 'runtime', 'database', 'tools'
	Version     string   `json:"version"`      // '1.24.0', '8.3'
	Description string   `json:"description"`  // deskripsi singkat
	Icon        string   `json:"icon"`         // remixicon class
	Installed   bool     `json:"installed"`
	Status      string   `json:"status"`       // 'running', 'stopped', 'not_installed'
	AptPackages []string `json:"apt_packages"` // apt packages yang diinstall
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
		return id == "nginx" || id == "php8.3", id == "nginx"
	}

	switch id {
	case "nginx":
		cmd := exec.CommandContext(ctx, "which", "nginx")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "nginx").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case "php8.3":
		cmd := exec.CommandContext(ctx, "which", "php8.3")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "php8.3-fpm").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case "php8.2":
		cmd := exec.CommandContext(ctx, "which", "php8.2")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "php8.2-fpm").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case "mariadb":
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

	case "redis":
		cmd := exec.CommandContext(ctx, "which", "redis-server")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "redis-server").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case "nodejs":
		cmd := exec.CommandContext(ctx, "which", "node")
		return cmd.Run() == nil, false

	case "certbot":
		cmd := exec.CommandContext(ctx, "which", "certbot")
		return cmd.Run() == nil, false

	case "fail2ban":
		cmd := exec.CommandContext(ctx, "which", "fail2ban-client")
		installed := cmd.Run() == nil
		running := false
		if installed {
			out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "fail2ban").Output()
			running = strings.TrimSpace(string(out)) == "active"
		}
		return installed, running

	case "git":
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

func (m *Manager) GetTask(appID string) *InstallTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[appID]
}
