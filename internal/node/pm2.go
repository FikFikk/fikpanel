package node

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type NodeSystemStatus struct {
	NodeInstalled bool   `json:"node_installed"`
	NodeVersion   string `json:"node_version"`
	PM2Installed  bool   `json:"pm2_installed"`
	PM2Version    string `json:"pm2_version"`
}

type PM2Process struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Namespace    string  `json:"namespace"`
	Version      string  `json:"version"`
	Mode         string  `json:"mode"`
	PID          int     `json:"pid"`
	Uptime       int64   `json:"uptime"`
	RestartCount int     `json:"restart_count"`
	Status       string  `json:"status"` // 'online', 'stopped', 'errored', 'launching'
	CPU          float64 `json:"cpu"`
	Memory       uint64  `json:"memory"`
	User         string  `json:"user"`
	Script       string  `json:"script"`
	Cwd          string  `json:"cwd"`
}

type StartAppParams struct {
	Name      string `json:"name"`
	Script    string `json:"script"`
	Cwd       string `json:"cwd"`
	Instances string `json:"instances"`
	Watch     bool   `json:"watch"`
	Args      string `json:"args"`
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

// resolveBinary mencari lokasi biner di sistem (PATH atau direktori standar OS)
func resolveBinary(name string) (string, error) {
	if runtime.GOOS == "windows" {
		if path, err := exec.LookPath(name + ".cmd"); err == nil {
			return path, nil
		}
		if path, err := exec.LookPath(name + ".exe"); err == nil {
			return path, nil
		}
	}

	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	// Cek lokasi umum di Linux
	if runtime.GOOS == "linux" {
		candidates := []string{
			"/usr/local/bin/" + name,
			"/usr/bin/" + name,
			"/bin/" + name,
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
		}

		// Cek instalasi NVM di home direktori
		if home, err := os.UserHomeDir(); err == nil {
			matches, _ := filepath.Glob(filepath.Join(home, ".nvm/versions/node/*/bin", name))
			if len(matches) > 0 {
				return matches[len(matches)-1], nil
			}
		}
	}

	return "", fmt.Errorf("perintah %s tidak ditemukan di sistem", name)
}

// GetStatus memeriksa ketersediaan Node.js dan PM2 di server
func (m *Manager) GetStatus(ctx context.Context) (NodeSystemStatus, error) {
	status := NodeSystemStatus{}

	if nodeBin, err := resolveBinary("node"); err == nil {
		cmd := exec.CommandContext(ctx, nodeBin, "-v")
		if out, err := cmd.Output(); err == nil {
			status.NodeInstalled = true
			status.NodeVersion = strings.TrimSpace(string(out))
		}
	}

	if pm2Bin, err := resolveBinary("pm2"); err == nil {
		cmd := exec.CommandContext(ctx, pm2Bin, "-v")
		if out, err := cmd.Output(); err == nil {
			status.PM2Installed = true
			// Ambil baris terakhir bila ada banner daemon
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			status.PM2Version = strings.TrimSpace(lines[len(lines)-1])
		}
	}

	return status, nil
}

// ListProcesses mengambil daftar proses aktif langsung dari daemon PM2
func (m *Manager) ListProcesses(ctx context.Context) ([]PM2Process, error) {
	pm2Bin, err := resolveBinary("pm2")
	if err != nil {
		// PM2 belum terpasang, kembalikan daftar kosong (bukan dummy)
		return []PM2Process{}, nil
	}

	cmd := exec.CommandContext(ctx, pm2Bin, "jlist")
	out, err := cmd.Output()
	if err != nil {
		return []PM2Process{}, nil
	}

	// Ekstrak array JSON murni (mengabaikan log teks daemon yang mungkin tercetak di awal)
	outStr := string(out)
	firstBracket := strings.Index(outStr, "[")
	lastBracket := strings.LastIndex(outStr, "]")
	if firstBracket == -1 || lastBracket == -1 || lastBracket < firstBracket {
		return []PM2Process{}, nil
	}
	jsonClean := outStr[firstBracket : lastBracket+1]

	var rawList []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonClean), &rawList); err != nil {
		return nil, fmt.Errorf("gagal membaca data proses PM2: %w", err)
	}

	var result []PM2Process
	for _, item := range rawList {
		p := PM2Process{}
		if pmID, ok := item["pm_id"].(float64); ok {
			p.ID = int(pmID)
		}
		if name, ok := item["name"].(string); ok {
			p.Name = name
		}
		if pid, ok := item["pid"].(float64); ok {
			p.PID = int(pid)
		}

		if pm2Env, ok := item["pm2_env"].(map[string]interface{}); ok {
			if st, ok := pm2Env["status"].(string); ok {
				p.Status = st
			}
			if ver, ok := pm2Env["version"].(string); ok {
				p.Version = ver
			}
			if u, ok := pm2Env["username"].(string); ok {
				p.User = u
			}
			if rc, ok := pm2Env["restart_time"].(float64); ok {
				p.RestartCount = int(rc)
			}
			if up, ok := pm2Env["pm_uptime"].(float64); ok {
				p.Uptime = int64(up)
			}
			if sc, ok := pm2Env["pm_exec_path"].(string); ok {
				p.Script = sc
			}
			if cwd, ok := pm2Env["pm_cwd"].(string); ok {
				p.Cwd = cwd
			}
			if mode, ok := pm2Env["exec_mode"].(string); ok {
				p.Mode = mode
			}
		}

		if mon, ok := item["monit"].(map[string]interface{}); ok {
			if mem, ok := mon["memory"].(float64); ok {
				p.Memory = uint64(mem)
			}
			if cpu, ok := mon["cpu"].(float64); ok {
				p.CPU = cpu
			}
		}

		result = append(result, p)
	}

	return result, nil
}

// StartApp menjalankan aplikasi Node.js baru ke dalam manajemen proses PM2
func (m *Manager) StartApp(ctx context.Context, p StartAppParams) error {
	pm2Bin, err := resolveBinary("pm2")
	if err != nil {
		return errors.New("PM2 belum terpasang di sistem")
	}

	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Script) == "" {
		return errors.New("nama aplikasi dan berkas script wajib diisi")
	}

	args := []string{"start", p.Script, "--name", p.Name}

	if strings.TrimSpace(p.Cwd) != "" {
		args = append(args, "--cwd", p.Cwd)
	}

	if p.Instances != "" && p.Instances != "1" {
		args = append(args, "-i", p.Instances)
	}

	if p.Watch {
		args = append(args, "--watch")
	}

	if strings.TrimSpace(p.Args) != "" {
		args = append(args, "--")
		args = append(args, strings.Fields(p.Args)...)
	}

	cmd := exec.CommandContext(ctx, pm2Bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gagal memulai aplikasi: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	// Simpan konfigurasi agar proses bertahan saat server reboot
	_ = exec.CommandContext(ctx, pm2Bin, "save").Run()
	return nil
}

// Action mengontrol siklus hidup proses (restart, stop, start, reload, delete)
func (m *Manager) Action(ctx context.Context, idOrName string, action string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "restart", "stop", "start", "reload", "delete":
	default:
		return fmt.Errorf("aksi PM2 tidak valid: %s", action)
	}

	pm2Bin, err := resolveBinary("pm2")
	if err != nil {
		return errors.New("PM2 belum terpasang di sistem")
	}

	cmd := exec.CommandContext(ctx, pm2Bin, action, idOrName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gagal mengeksekusi %s pada proses %s: %s", action, idOrName, strings.TrimSpace(string(out)))
	}

	// Jika menghapus proses, perbarui status penyimpanan PM2
	if action == "delete" {
		_ = exec.CommandContext(ctx, pm2Bin, "save").Run()
	}

	return nil
}

// GetLogs mengambil baris log stdout dan stderr proses dari PM2
func (m *Manager) GetLogs(ctx context.Context, idOrName string, lines int) (string, error) {
	pm2Bin, err := resolveBinary("pm2")
	if err != nil {
		return "", errors.New("PM2 belum terpasang di sistem")
	}

	if lines <= 0 || lines > 500 {
		lines = 60
	}

	cmd := exec.CommandContext(ctx, pm2Bin, "logs", idOrName, "--lines", strconv.Itoa(lines), "--nostream")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	combined := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	if combined == "" {
		return "Belum ada catatan log untuk proses ini.", nil
	}

	return combined, nil
}

