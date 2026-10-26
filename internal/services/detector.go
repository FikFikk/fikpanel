package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// SoftwareInfo merepresentasikan status instalasi dan operasional paket software sistem
type SoftwareInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Installed   bool   `json:"installed"`
	Version     string `json:"version"`
	Running     bool   `json:"running"`
	ServiceUnit string `json:"service_unit"`
}

// DetectSoftware mengecek status instalasi dan runtime dari sebuah software
func DetectSoftware(ctx context.Context, name string) SoftwareInfo {
	switch name {
	case "nginx":
		return CheckNginx(ctx)
	case "apache2":
		return CheckApache(ctx)
	case "mysql":
		return CheckMySQL(ctx)
	case "php":
		return CheckPHP(ctx)
	case "node":
		return CheckNode(ctx)
	case "python3":
		return CheckPython(ctx)
	default:
		return SoftwareInfo{Name: name, DisplayName: name}
	}
}

// CheckNginx memeriksa status Nginx di server
func CheckNginx(ctx context.Context) SoftwareInfo {
	info := SoftwareInfo{
		Name:        "nginx",
		DisplayName: "Nginx Web Server",
		Category:    "webserver",
		ServiceUnit: "nginx.service",
	}

	if runtime.GOOS != "linux" {
		// Mock otomatis untuk lingkungan dev Windows
		// Jika env FIKPANEL_MOCK_NGINX=0 maka diset belum terpasang untuk menguji tampilan warning
		if os.Getenv("FIKPANEL_MOCK_NGINX") == "0" {
			info.Installed = false
			return info
		}
		info.Installed = true
		info.Version = "1.24.0"
		info.Running = true
		return info
	}

	path, err := exec.LookPath("nginx")
	if err != nil {
		info.Installed = false
		return info
	}
	info.Installed = true

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out, _ := exec.CommandContext(cmdCtx, path, "-v").CombinedOutput()
	re := regexp.MustCompile(`nginx/([0-9.]+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		info.Version = matches[1]
	}

	sysOut, _ := exec.CommandContext(cmdCtx, "systemctl", "is-active", "nginx").Output()
	info.Running = (strings.TrimSpace(string(sysOut)) == "active")
	return info
}

// CheckApache memeriksa status Apache2 / httpd di server
func CheckApache(ctx context.Context) SoftwareInfo {
	info := SoftwareInfo{
		Name:        "apache2",
		DisplayName: "Apache HTTP Server",
		Category:    "webserver",
		ServiceUnit: "apache2.service",
	}

	if runtime.GOOS != "linux" {
		info.Installed = false
		return info
	}

	bin := "apache2"
	path, err := exec.LookPath(bin)
	if err != nil {
		bin = "httpd"
		path, err = exec.LookPath(bin)
	}
	if err != nil {
		info.Installed = false
		return info
	}
	info.Installed = true

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out, _ := exec.CommandContext(cmdCtx, path, "-v").CombinedOutput()
	re := regexp.MustCompile(`Apache/([0-9.]+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		info.Version = matches[1]
	}

	sysOut, _ := exec.CommandContext(cmdCtx, "systemctl", "is-active", info.ServiceUnit).Output()
	info.Running = (strings.TrimSpace(string(sysOut)) == "active")
	return info
}

// CheckMySQL memeriksa status MySQL atau MariaDB
func CheckMySQL(ctx context.Context) SoftwareInfo {
	info := SoftwareInfo{
		Name:        "mysql",
		DisplayName: "MySQL Database Server",
		Category:    "database",
		ServiceUnit: "mysql.service",
	}

	if runtime.GOOS != "linux" {
		info.Installed = true
		info.Version = "8.0.36"
		info.Running = true
		return info
	}

	bin := "mysqld"
	path, err := exec.LookPath(bin)
	if err != nil {
		bin = "mysql"
		path, err = exec.LookPath(bin)
	}
	if err != nil {
		info.Installed = false
		return info
	}
	info.Installed = true

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out, _ := exec.CommandContext(cmdCtx, path, "--version").CombinedOutput()
	re := regexp.MustCompile(`(8\.[0-9.]+|[0-9.]+-MariaDB|[0-9.]+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		info.Version = matches[1]
	}

	sysOut, _ := exec.CommandContext(cmdCtx, "systemctl", "is-active", "mysql").Output()
	if strings.TrimSpace(string(sysOut)) != "active" {
		sysOut, _ = exec.CommandContext(cmdCtx, "systemctl", "is-active", "mariadb").Output()
	}
	info.Running = (strings.TrimSpace(string(sysOut)) == "active")
	return info
}

// CheckPHP memeriksa ketersediaan interpreter PHP dan FPM service
func CheckPHP(ctx context.Context) SoftwareInfo {
	info := SoftwareInfo{
		Name:        "php",
		DisplayName: "PHP FastCGI (FPM)",
		Category:    "runtime",
		ServiceUnit: "php-fpm.service",
	}

	if runtime.GOOS != "linux" {
		info.Installed = true
		info.Version = "8.3.4"
		info.Running = true
		return info
	}

	path, err := exec.LookPath("php")
	if err != nil {
		info.Installed = false
		return info
	}
	info.Installed = true

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out, _ := exec.CommandContext(cmdCtx, path, "-v").CombinedOutput()
	re := regexp.MustCompile(`PHP ([0-9.]+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		info.Version = matches[1]
	}

	// Deteksi unit FPM dinamis sesuai versi PHP yang terdeteksi (misal: php8.5-fpm atau php8.3-fpm)
	fpmUnit := "php-fpm"
	if info.Version != "" {
		parts := strings.Split(info.Version, ".")
		if len(parts) >= 2 {
			fpmUnit = fmt.Sprintf("php%s.%s-fpm", parts[0], parts[1])
		}
	}
	info.ServiceUnit = fpmUnit + ".service"

	sysOut, _ := exec.CommandContext(cmdCtx, "systemctl", "is-active", fpmUnit).Output()
	if strings.TrimSpace(string(sysOut)) != "active" {
		sysOut, _ = exec.CommandContext(cmdCtx, "systemctl", "is-active", "php-fpm").Output()
	}
	info.Running = (strings.TrimSpace(string(sysOut)) == "active")
	return info
}

// CheckNode memeriksa runtime Node.js
func CheckNode(ctx context.Context) SoftwareInfo {
	info := SoftwareInfo{
		Name:        "node",
		DisplayName: "Node.js Runtime",
		Category:    "runtime",
	}

	path, err := exec.LookPath("node")
	if err != nil {
		info.Installed = false
		return info
	}
	info.Installed = true

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out, _ := exec.CommandContext(cmdCtx, path, "--version").CombinedOutput()
	info.Version = strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
	info.Running = true
	return info
}

// CheckPython memeriksa runtime Python 3
func CheckPython(ctx context.Context) SoftwareInfo {
	info := SoftwareInfo{
		Name:        "python3",
		DisplayName: "Python 3 Runtime",
		Category:    "runtime",
	}

	path, err := exec.LookPath("python3")
	if err != nil {
		path, err = exec.LookPath("python")
	}
	if err != nil {
		info.Installed = false
		return info
	}
	info.Installed = true

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out, _ := exec.CommandContext(cmdCtx, path, "--version").CombinedOutput()
	re := regexp.MustCompile(`Python ([0-9.]+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		info.Version = matches[1]
	}
	info.Running = true
	return info
}

// DetectAllSoftware mengembalikan status seluruh software dalam katalog FikPanel
func DetectAllSoftware(ctx context.Context) []SoftwareInfo {
	items := []string{"nginx", "apache2", "mysql", "php", "node", "python3"}
	result := make([]SoftwareInfo, 0, len(items))
	for _, item := range items {
		result = append(result, DetectSoftware(ctx, item))
	}
	return result
}
