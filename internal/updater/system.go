package updater

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ServiceState adalah ringkasan status unit systemd dari `systemctl show`.
type ServiceState struct {
	LoadState    string
	ActiveState  string
	MainPID      int
	NRestarts    int
	FragmentPath string
}

// System mengabstraksi interaksi dengan systemd dan /proc agar alur updater dapat diuji.
type System interface {
	ServiceState(ctx context.Context, unit string) (ServiceState, error)
	Systemctl(ctx context.Context, args ...string) error
	ProcessExe(pid int) (string, error)
}

type execSystem struct{}

// NewSystem mengembalikan implementasi System berbasis perintah systemctl dan /proc.
func NewSystem() System { return execSystem{} }

func (execSystem) ServiceState(ctx context.Context, unit string) (ServiceState, error) {
	out, err := exec.CommandContext(ctx, "systemctl", "show", unit,
		"-p", "LoadState", "-p", "ActiveState", "-p", "MainPID", "-p", "NRestarts", "-p", "FragmentPath").Output()
	if err != nil {
		return ServiceState{}, fmt.Errorf("systemctl show %s gagal: %w", unit, err)
	}
	return parseServiceState(string(out)), nil
}

func parseServiceState(out string) ServiceState {
	var st ServiceState
	for _, line := range strings.Split(out, "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "LoadState":
			st.LoadState = val
		case "ActiveState":
			st.ActiveState = val
		case "MainPID":
			st.MainPID, _ = strconv.Atoi(val)
		case "NRestarts":
			st.NRestarts, _ = strconv.Atoi(val)
		case "FragmentPath":
			st.FragmentPath = val
		}
	}
	return st
}

func (execSystem) Systemctl(ctx context.Context, args ...string) error {
	out, err := exec.CommandContext(ctx, "systemctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s gagal: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (execSystem) ProcessExe(pid int) (string, error) {
	return os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
}

// SystemdAvailable melaporkan apakah host berjalan dengan systemd sebagai init (PID 1).
func SystemdAvailable() bool {
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return false
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	_, err := exec.LookPath("systemd-run")
	return err == nil
}

// InsideUnitCgroup melaporkan apakah proses saat ini berada di cgroup unit systemd tertentu
// (misalnya perintah yang dijalankan dari Web Terminal berada di cgroup fikpanel.service).
func InsideUnitCgroup(unit string) bool {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return false
	}
	return cgroupContainsUnit(string(data), unit)
}

func cgroupContainsUnit(cgroupFile, unit string) bool {
	for _, line := range strings.Split(cgroupFile, "\n") {
		idx := strings.LastIndex(line, ":")
		if idx < 0 {
			continue
		}
		for _, seg := range strings.Split(line[idx+1:], "/") {
			if seg == unit {
				return true
			}
		}
	}
	return false
}

// ServiceConfig adalah parameter penting yang diambil dari unit fikpanel.service.
type ServiceConfig struct {
	BinPath string
	DBPath  string
	Port    int
}

// ParseServiceConfig membaca unit file systemd dan mengekstrak path biner, --db, dan --port.
// Path database relatif di-resolve terhadap WorkingDirectory unit.
func ParseServiceConfig(unitFile string) (ServiceConfig, error) {
	f, err := os.Open(unitFile)
	if err != nil {
		return ServiceConfig{}, err
	}
	defer f.Close()

	var cfg ServiceConfig
	var workDir, execStart string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "WorkingDirectory="):
			workDir = strings.TrimSpace(strings.TrimPrefix(line, "WorkingDirectory="))
		case strings.HasPrefix(line, "ExecStart="):
			execStart = strings.TrimSpace(strings.TrimPrefix(line, "ExecStart="))
		}
	}
	if err := sc.Err(); err != nil {
		return ServiceConfig{}, err
	}
	if execStart == "" {
		return ServiceConfig{}, fmt.Errorf("ExecStart tidak ditemukan di %s", unitFile)
	}

	fields := strings.Fields(execStart)
	cfg.BinPath = strings.TrimLeft(fields[0], "-@+!:")
	for i := 1; i < len(fields); i++ {
		name, val, hasVal := strings.Cut(strings.TrimLeft(fields[i], "-"), "=")
		if !strings.HasPrefix(fields[i], "-") || (name != "port" && name != "db") {
			continue
		}
		if !hasVal {
			if i+1 >= len(fields) {
				break
			}
			i++
			val = fields[i]
		}
		switch name {
		case "port":
			if p, err := strconv.Atoi(val); err == nil && p > 0 && p < 65536 {
				cfg.Port = p
			}
		case "db":
			cfg.DBPath = val
		}
	}
	if filepath.IsAbs(workDir) {
		// `fikpanel serve` memakai "fikpanel.db" relatif terhadap direktori kerja bila --db tidak diberikan.
		if cfg.DBPath == "" {
			cfg.DBPath = "fikpanel.db"
		}
		if !filepath.IsAbs(cfg.DBPath) {
			cfg.DBPath = filepath.Join(workDir, cfg.DBPath)
		}
	}
	return cfg, nil
}
