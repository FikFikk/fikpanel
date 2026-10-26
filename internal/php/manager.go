package php

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type PHPVersionInfo struct {
	Version   string            `json:"version"`   // misal: "8.5", "8.3", "8.2", "8.1"
	Installed bool              `json:"installed"` // true/false
	Running   bool              `json:"running"`   // FPM running
	FpmSocket string            `json:"fpm_socket"`
	IniPath   string            `json:"ini_path"`
	Settings  map[string]string `json:"settings"` // upload_max_filesize, memory_limit, max_execution_time
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

// isPHPVersionFormat memeriksa apakah string memiliki format angka versi X.Y
func isPHPVersionFormat(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	return err1 == nil && err2 == nil && major >= 5 && major <= 9 && minor >= 0 && minor <= 99
}

// comparePHPVersions membandingkan dua versi PHP secara numerik
func comparePHPVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	if len(partsA) != 2 || len(partsB) != 2 {
		return strings.Compare(a, b)
	}
	majA, _ := strconv.Atoi(partsA[0])
	minA, _ := strconv.Atoi(partsA[1])
	majB, _ := strconv.Atoi(partsB[0])
	minB, _ := strconv.Atoi(partsB[1])
	if majA != majB {
		return majA - majB
	}
	return minA - minB
}

// discoverInstalledVersions memindai seluruh versi PHP nyata yang terpasang pada sistem host Linux
func (m *Manager) discoverInstalledVersions() []string {
	var installed []string
	seen := make(map[string]bool)

	// 1. Pindai folder konfigurasi /etc/php/*
	if entries, err := os.ReadDir("/etc/php"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				ver := e.Name()
				if isPHPVersionFormat(ver) && !seen[ver] {
					seen[ver] = true
					installed = append(installed, ver)
				}
			}
		}
	}

	// 2. Pindai biner /usr/sbin/php-fpm*
	if matches, err := filepath.Glob("/usr/sbin/php-fpm*"); err == nil {
		for _, p := range matches {
			base := filepath.Base(p)
			ver := strings.TrimPrefix(base, "php-fpm")
			if isPHPVersionFormat(ver) && !seen[ver] {
				seen[ver] = true
				installed = append(installed, ver)
			}
		}
	}

	// 3. Pindai soket runtime /run/php/php*-fpm.sock
	if matches, err := filepath.Glob("/run/php/php*-fpm.sock"); err == nil {
		re := regexp.MustCompile(`php([0-9.]+)-fpm\.sock`)
		for _, p := range matches {
			base := filepath.Base(p)
			sub := re.FindStringSubmatch(base)
			if len(sub) > 1 && isPHPVersionFormat(sub[1]) && !seen[sub[1]] {
				seen[sub[1]] = true
				installed = append(installed, sub[1])
			}
		}
	}

	// 4. Pindai versi CLI aktif (php -v)
	if out, err := exec.Command("php", "-r", "echo PHP_MAJOR_VERSION.'.'.PHP_MINOR_VERSION;").Output(); err == nil {
		ver := strings.TrimSpace(string(out))
		if isPHPVersionFormat(ver) && !seen[ver] {
			seen[ver] = true
			installed = append(installed, ver)
		}
	}

	return installed
}

func (m *Manager) ListVersions(ctx context.Context) ([]PHPVersionInfo, error) {
	// Versi kandidat standar yang umum digunakan di server web
	candidates := []string{"8.5", "8.4", "8.3", "8.2", "8.1", "8.0", "7.4"}
	seen := make(map[string]bool)
	var allVersions []string

	// Temukan versi PHP terpasang secara dinamis di sistem host Linux
	if runtime.GOOS == "linux" {
		installed := m.discoverInstalledVersions()
		for _, v := range installed {
			if !seen[v] {
				seen[v] = true
				allVersions = append(allVersions, v)
			}
		}
	}

	// Gabungkan dengan kandidat versi umum lainnya
	for _, v := range candidates {
		if !seen[v] {
			seen[v] = true
			allVersions = append(allVersions, v)
		}
	}

	// Urutkan versi secara menurun (versi terbaru berada di atas/awal)
	sort.Slice(allVersions, func(i, j int) bool {
		return comparePHPVersions(allVersions[i], allVersions[j]) > 0
	})

	var list []PHPVersionInfo
	for _, ver := range allVersions {
		info := PHPVersionInfo{
			Version:   ver,
			Settings:  make(map[string]string),
			FpmSocket: fmt.Sprintf("/run/php/php%s-fpm.sock", ver),
			IniPath:   fmt.Sprintf("/etc/php/%s/fpm/php.ini", ver),
		}

		if runtime.GOOS != "linux" {
			info.Installed = ver == "8.5" || ver == "8.3"
			info.Running = ver == "8.5"
			info.Settings["memory_limit"] = "256M"
			info.Settings["upload_max_filesize"] = "64M"
			info.Settings["post_max_size"] = "64M"
			info.Settings["max_execution_time"] = "300"
			list = append(list, info)
			continue
		}

		// Cek apakah biner / direktori konfigurasi / soket php-fpm ada
		binPath := fmt.Sprintf("/usr/sbin/php-fpm%s", ver)
		sockPath := fmt.Sprintf("/run/php/php%s-fpm.sock", ver)
		if _, err := os.Stat(binPath); err == nil {
			info.Installed = true
		} else if _, err := os.Stat(info.IniPath); err == nil {
			info.Installed = true
		} else if _, err := os.Stat(sockPath); err == nil {
			info.Installed = true
		}

		// Cek apakah service running
		serviceName := fmt.Sprintf("php%s-fpm", ver)
		cmd := exec.CommandContext(ctx, "systemctl", "is-active", serviceName)
		out, _ := cmd.Output()
		info.Running = strings.TrimSpace(string(out)) == "active"
		if info.Running {
			info.Installed = true
		}

		if info.Installed {
			// Cari lokasi php.ini jika path default fpm belum ada
			if _, err := os.Stat(info.IniPath); err != nil {
				altPath := fmt.Sprintf("/etc/php/%s/cli/php.ini", ver)
				if _, err := os.Stat(altPath); err == nil {
					info.IniPath = altPath
				}
			}

			// Baca settings penting dari php.ini
			if iniFile, err := os.Open(info.IniPath); err == nil {
				scanner := bufio.NewScanner(iniFile)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if strings.HasPrefix(line, ";") || !strings.Contains(line, "=") {
						continue
					}
					parts := strings.SplitN(line, "=", 2)
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					switch key {
					case "memory_limit", "upload_max_filesize", "post_max_size", "max_execution_time":
						info.Settings[key] = val
					}
				}
				iniFile.Close()
			}
		}

		list = append(list, info)
	}

	return list, nil
}

func (m *Manager) RestartFPM(ctx context.Context, ver string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	serviceName := fmt.Sprintf("php%s-fpm", ver)
	cmd := exec.CommandContext(ctx, "systemctl", "restart", serviceName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal restart %s: %s %w", serviceName, string(out), err)
	}
	return nil
}

func (m *Manager) UpdateSetting(ctx context.Context, ver string, key string, val string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	iniPath := fmt.Sprintf("/etc/php/%s/fpm/php.ini", ver)
	content, err := os.ReadFile(iniPath)
	if err != nil {
		return fmt.Errorf("tidak dapat membuka php.ini: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			lines[i] = fmt.Sprintf("%s = %s", key, val)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("%s = %s", key, val))
	}

	if err := os.WriteFile(iniPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("gagal menyimpan php.ini: %w", err)
	}

	_ = m.RestartFPM(ctx, ver)
	return nil
}
