package node

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type PM2Process struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   string `json:"version"`
	Mode      string `json:"mode"`
	PID       int    `json:"pid"`
	Uptime    int64  `json:"uptime"`
	RestartCount int `json:"restart_count"`
	Status    string `json:"status"` // 'online', 'stopped', 'errored'
	CPU       float64 `json:"cpu"`
	Memory    uint64  `json:"memory"`
	User      string `json:"user"`
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) ListProcesses(ctx context.Context) ([]PM2Process, error) {
	if runtime.GOOS != "linux" {
		return []PM2Process{
			{ID: 0, Name: "figo-web", Status: "online", CPU: 0.2, Memory: 85000000, PID: 12450, User: "fikfikk"},
			{ID: 1, Name: "fikvault-api", Status: "online", CPU: 0.1, Memory: 62000000, PID: 12455, User: "fikfikk"},
		}, nil
	}

	cmd := exec.CommandContext(ctx, "pm2", "jlist")
	out, err := cmd.Output()
	if err != nil {
		// Fallback: coba su - fikfikk jika PM2 ada di user fikfikk
		cmdUser := exec.CommandContext(ctx, "su", "-", "fikfikk", "-c", "pm2 jlist")
		outUser, errUser := cmdUser.Output()
		if errUser != nil {
			return nil, fmt.Errorf("pm2 jlist gagal: %w (fallback: %v)", err, errUser)
		}
		out = outUser
	}

	var rawList []map[string]interface{}
	if err := json.Unmarshal(out, &rawList); err != nil {
		return nil, fmt.Errorf("gagal parse output pm2 json: %w", err)
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

func (m *Manager) Action(ctx context.Context, idOrName string, action string) error {
	action = strings.ToLower(action)
	switch action {
	case "restart", "stop", "start", "reload":
	default:
		return fmt.Errorf("aksi pm2 tidak valid: %s", action)
	}

	if runtime.GOOS != "linux" {
		return nil
	}

	cmd := exec.CommandContext(ctx, "pm2", action, idOrName)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Fallback user fikfikk
		cmdUser := exec.CommandContext(ctx, "su", "-", "fikfikk", "-c", fmt.Sprintf("pm2 %s %s", action, idOrName))
		if outU, errU := cmdUser.CombinedOutput(); errU != nil {
			return fmt.Errorf("gagal eksekusi pm2 %s: %s %v", action, string(out), outU)
		}
	}
	return nil
}
