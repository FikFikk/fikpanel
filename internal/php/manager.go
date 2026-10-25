package php

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type PHPVersionInfo struct {
	Version   string            `json:"version"`   // misal: "8.3", "8.2", "8.1"
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

func (m *Manager) ListVersions(ctx context.Context) ([]PHPVersionInfo, error) {
	candidates := []string{"8.3", "8.2", "8.1", "8.0", "7.4"}
	var list []PHPVersionInfo

	for _, ver := range candidates {
		info := PHPVersionInfo{
			Version:   ver,
			Settings:  make(map[string]string),
			FpmSocket: fmt.Sprintf("/run/php/php%s-fpm.sock", ver),
			IniPath:   fmt.Sprintf("/etc/php/%s/fpm/php.ini", ver),
		}

		if runtime.GOOS != "linux" {
			info.Installed = ver == "8.3"
			info.Running = ver == "8.3"
			info.Settings["memory_limit"] = "256M"
			info.Settings["upload_max_filesize"] = "64M"
			info.Settings["post_max_size"] = "64M"
			info.Settings["max_execution_time"] = "300"
			list = append(list, info)
			continue
		}

		// Cek apakah binary / socket php fpm ada
		binPath := fmt.Sprintf("/usr/sbin/php-fpm%s", ver)
		if _, err := os.Stat(binPath); err == nil {
			info.Installed = true
		} else if _, err := os.Stat(info.IniPath); err == nil {
			info.Installed = true
		}

		if info.Installed {
			// Cek apakah service running
			serviceName := fmt.Sprintf("php%s-fpm", ver)
			cmd := exec.CommandContext(ctx, "systemctl", "is-active", serviceName)
			out, _ := cmd.Output()
			info.Running = strings.TrimSpace(string(out)) == "active"

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
