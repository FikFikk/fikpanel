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
	"time"
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
	User      string `json:"user"`
}

// pm2UserTarget menyimpan informasi user pemilik direktori .pm2 di sistem
type pm2UserTarget struct {
	Username string
	HomeDir  string
	PM2Home  string
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

// resolveBinary mencari lokasi biner di sistem (PATH, direktori standar OS, atau NVM multi-user)
func resolveBinary(name string) (string, error) {
	return resolveBinaryForUser(name, "")
}

// resolveBinaryForUser mencari biner untuk user tertentu (termasuk NVM/npm-global milik user tersebut)
func resolveBinaryForUser(name string, username string) (string, error) {
	if runtime.GOOS == "windows" {
		if path, err := exec.LookPath(name + ".cmd"); err == nil {
			return path, nil
		}
		if path, err := exec.LookPath(name + ".exe"); err == nil {
			return path, nil
		}
	}

	// Jika username spesifik diberikan di Linux, prioritaskan NVM/npm-global milik user tersebut
	if runtime.GOOS == "linux" && username != "" && username != "root" {
		userHome := filepath.Join("/home", username)
		userPatterns := []string{
			filepath.Join(userHome, ".nvm/versions/node/*/bin", name),
			filepath.Join(userHome, ".npm-global/bin", name),
			filepath.Join(userHome, ".local/share/pnpm", name),
			filepath.Join(userHome, ".volta/bin", name),
		}
		for _, pat := range userPatterns {
			if matches, _ := filepath.Glob(pat); len(matches) > 0 {
				return matches[len(matches)-1], nil
			}
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

		// Cek instalasi NVM di /root dan seluruh /home/*
		nvmPatterns := []string{
			filepath.Join("/root", ".nvm/versions/node/*/bin", name),
			filepath.Join("/home/*", ".nvm/versions/node/*/bin", name),
			filepath.Join("/home/*", ".npm-global/bin", name),
			filepath.Join("/home/*", ".local/share/pnpm", name),
			filepath.Join("/home/*", ".volta/bin", name),
		}
		for _, pat := range nvmPatterns {
			if matches, _ := filepath.Glob(pat); len(matches) > 0 {
				return matches[len(matches)-1], nil
			}
		}
	}

	return "", fmt.Errorf("perintah %s tidak ditemukan di sistem", name)
}

// discoverPM2Users mendeteksi seluruh direktori .pm2 di server (/root/.pm2 dan /home/*/.pm2)
func discoverPM2Users() []pm2UserTarget {
	if runtime.GOOS != "linux" {
		home, _ := os.UserHomeDir()
		return []pm2UserTarget{
			{
				Username: "default",
				HomeDir:  home,
				PM2Home:  filepath.Join(home, ".pm2"),
			},
		}
	}

	var targets []pm2UserTarget
	seen := make(map[string]bool)

	// 1. Cek /root/.pm2
	if info, err := os.Stat("/root/.pm2"); err == nil && info.IsDir() {
		targets = append(targets, pm2UserTarget{
			Username: "root",
			HomeDir:  "/root",
			PM2Home:  "/root/.pm2",
		})
		seen["root"] = true
	}

	// 2. Cek seluruh /home/*/.pm2
	if entries, err := os.ReadDir("/home"); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			username := entry.Name()
			if seen[username] {
				continue
			}
			userHome := filepath.Join("/home", username)
			pm2Dir := filepath.Join(userHome, ".pm2")
			if info, err := os.Stat(pm2Dir); err == nil && info.IsDir() {
				targets = append(targets, pm2UserTarget{
					Username: username,
					HomeDir:  userHome,
					PM2Home:  pm2Dir,
				})
				seen[username] = true
			}
		}
	}

	// Fallback ke root jika belum ada direktori .pm2 sama sekali
	if len(targets) == 0 {
		targets = append(targets, pm2UserTarget{
			Username: "root",
			HomeDir:  "/root",
			PM2Home:  "/root/.pm2",
		})
	}

	return targets
}

// buildUserPM2Command menyusun perintah eksekusi PM2 atas nama user pemilik proses (sudo -u <username> pm2 ...)
func buildUserPM2Command(ctx context.Context, username string, args ...string) (*exec.Cmd, error) {
	username = strings.TrimSpace(username)
	pm2Bin, err := resolveBinaryForUser("pm2", username)
	if err != nil {
		return nil, err
	}

	if runtime.GOOS != "linux" {
		return exec.CommandContext(ctx, pm2Bin, args...), nil
	}

	if username == "" {
		username = "root"
	}

	homeDir := "/root"
	if username != "root" {
		homeDir = filepath.Join("/home", username)
	}
	pm2Home := filepath.Join(homeDir, ".pm2")

	// Susun PATH yang menyertakan lokasi biner PM2 dan Node.js (termasuk NVM user)
	pathParts := []string{filepath.Dir(pm2Bin)}
	if nodeBin, errNode := resolveBinaryForUser("node", username); errNode == nil {
		nodeDir := filepath.Dir(nodeBin)
		if nodeDir != pathParts[0] {
			pathParts = append(pathParts, nodeDir)
		}
	}
	pathParts = append(pathParts, "/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin")
	richPath := strings.Join(pathParts, ":")

	// Jika target adalah user non-root, jalankan dengan sudo -u <username> dan PM2_HOME user tersebut
	if username != "root" {
		if sudoBin, errSudo := exec.LookPath("sudo"); errSudo == nil {
			sudoArgs := []string{
				"-n", "-u", username,
				"env",
				"HOME=" + homeDir,
				"PM2_HOME=" + pm2Home,
				"PATH=" + richPath,
				pm2Bin,
			}
			sudoArgs = append(sudoArgs, args...)
			return exec.CommandContext(ctx, sudoBin, sudoArgs...), nil
		}
		if runuserBin, errRu := exec.LookPath("runuser"); errRu == nil {
			ruArgs := []string{
				"-u", username, "--",
				"env",
				"HOME=" + homeDir,
				"PM2_HOME=" + pm2Home,
				"PATH=" + richPath,
				pm2Bin,
			}
			ruArgs = append(ruArgs, args...)
			return exec.CommandContext(ctx, runuserBin, ruArgs...), nil
		}
	}

	// Eksekusi sebagai root (atau fallback jika sudo tidak tersedia) dengan PM2_HOME eksplisit
	cmd := exec.CommandContext(ctx, pm2Bin, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"PM2_HOME="+pm2Home,
		"PATH="+richPath,
	)
	return cmd, nil
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
		// Pastikan PATH menyertakan direktori node jika pm2 berada di NVM
		if nodeBin, errNode := resolveBinary("node"); errNode == nil {
			cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(nodeBin)+":"+filepath.Dir(pm2Bin)+":/usr/local/bin:/usr/bin:/bin")
		}
		if out, err := cmd.Output(); err == nil {
			status.PM2Installed = true
			// Ambil baris terakhir bila ada banner daemon
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			status.PM2Version = strings.TrimSpace(lines[len(lines)-1])
		}
	}

	return status, nil
}

// parsePM2JList mengekstrak daftar PM2Process dari output JSON `pm2 jlist` milik satu user
func parsePM2JList(out []byte, fallbackUser string) []PM2Process {
	outStr := string(out)
	firstBracket := strings.Index(outStr, "[")
	lastBracket := strings.LastIndex(outStr, "]")
	if firstBracket == -1 || lastBracket == -1 || lastBracket < firstBracket {
		return nil
	}
	jsonClean := outStr[firstBracket : lastBracket+1]

	var rawList []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonClean), &rawList); err != nil {
		return nil
	}

	var result []PM2Process
	for _, item := range rawList {
		p := PM2Process{
			User: fallbackUser,
		}
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
			if ns, ok := pm2Env["namespace"].(string); ok {
				p.Namespace = ns
			}
			if ver, ok := pm2Env["version"].(string); ok {
				p.Version = ver
			}
			if u, ok := pm2Env["username"].(string); ok && strings.TrimSpace(u) != "" {
				p.User = strings.TrimSpace(u)
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

	return result
}

// ListProcesses mengambil dan menggabungkan daftar proses aktif dari seluruh user yang memiliki folder .pm2
func (m *Manager) ListProcesses(ctx context.Context) ([]PM2Process, error) {
	if _, err := resolveBinary("pm2"); err != nil {
		// PM2 belum terpasang, kembalikan daftar kosong
		return []PM2Process{}, nil
	}

	targets := discoverPM2Users()
	result := []PM2Process{}
	seenPID := make(map[int]bool)
	seenUserKey := make(map[string]bool)

	for _, target := range targets {
		userCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		cmd, err := buildUserPM2Command(userCtx, target.Username, "jlist")
		if err != nil {
			cancel()
			continue
		}

		out, err := cmd.Output()
		cancel()
		if err != nil || len(out) == 0 {
			continue
		}

		procs := parsePM2JList(out, target.Username)
		for _, p := range procs {
			if p.User == "" {
				p.User = target.Username
			}
			// Hindari duplikasi proses yang sama jika socket terdeteksi dua kali
			if p.PID > 0 {
				if seenPID[p.PID] {
					continue
				}
				seenPID[p.PID] = true
			}
			ukey := fmt.Sprintf("%s:%d:%s", p.User, p.ID, p.Name)
			if seenUserKey[ukey] {
				continue
			}
			seenUserKey[ukey] = true
			result = append(result, p)
		}
	}

	return result, nil
}

// resolveProcessOwner menentukan username pemilik proses PM2 berdasarkan ID atau nama proses
func (m *Manager) resolveProcessOwner(ctx context.Context, idOrName string, explicitUser string) (string, string) {
	idOrName = strings.TrimSpace(idOrName)
	explicitUser = strings.TrimSpace(explicitUser)

	// Mendukung format "username:id" bila dikirim menyatu pada parameter id
	if strings.Contains(idOrName, ":") {
		parts := strings.SplitN(idOrName, ":", 2)
		if parts[0] != "" && parts[1] != "" {
			explicitUser = parts[0]
			idOrName = parts[1]
		}
	}

	if explicitUser != "" {
		return explicitUser, idOrName
	}

	// Cari pemilik proses dari daftar proses semua user
	procs, err := m.ListProcesses(ctx)
	if err == nil && len(procs) > 0 {
		// Prioritaskan pencocokan pada user non-root terlebih dahulu bila ada ID yang sama
		for _, p := range procs {
			if (strconv.Itoa(p.ID) == idOrName || p.Name == idOrName) && p.User != "" && p.User != "root" {
				return p.User, idOrName
			}
		}
		for _, p := range procs {
			if strconv.Itoa(p.ID) == idOrName || p.Name == idOrName {
				if p.User != "" {
					return p.User, idOrName
				}
				return "root", idOrName
			}
		}
	}

	// Fallback ke user pertama yang memiliki direktori .pm2
	targets := discoverPM2Users()
	if len(targets) > 0 {
		return targets[0].Username, idOrName
	}
	return "root", idOrName
}

// inferUserFromPath menebak pemilik user Linux dari path direktori /home/<username>/...
func inferUserFromPath(pathStr string) string {
	cleaned := filepath.ToSlash(strings.TrimSpace(pathStr))
	if strings.HasPrefix(cleaned, "/home/") {
		rest := strings.TrimPrefix(cleaned, "/home/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
	}
	return "root"
}

// StartApp menjalankan aplikasi Node.js baru ke dalam manajemen proses PM2
func (m *Manager) StartApp(ctx context.Context, p StartAppParams) error {
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Script) == "" {
		return errors.New("nama aplikasi dan berkas script wajib diisi")
	}

	targetUser := strings.TrimSpace(p.User)
	if targetUser == "" {
		if u := inferUserFromPath(p.Cwd); u != "root" {
			targetUser = u
		} else if u := inferUserFromPath(p.Script); u != "root" {
			targetUser = u
		} else {
			targetUser = "root"
		}
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

	cmd, err := buildUserPM2Command(ctx, targetUser, args...)
	if err != nil {
		return errors.New("PM2 belum terpasang di sistem")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gagal memulai aplikasi: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	// Simpan konfigurasi agar proses bertahan saat server reboot atas nama user pemilik
	if saveCmd, errSave := buildUserPM2Command(ctx, targetUser, "save"); errSave == nil {
		_ = saveCmd.Run()
	}
	return nil
}

// Action mengontrol siklus hidup proses (restart, stop, start, reload, delete) atas nama user pemilik proses
func (m *Manager) Action(ctx context.Context, idOrName string, action string, explicitUser ...string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "restart", "stop", "start", "reload", "delete":
	default:
		return fmt.Errorf("aksi PM2 tidak valid: %s", action)
	}

	userArg := ""
	if len(explicitUser) > 0 {
		userArg = explicitUser[0]
	}
	ownerUser, targetID := m.resolveProcessOwner(ctx, idOrName, userArg)

	cmd, err := buildUserPM2Command(ctx, ownerUser, action, targetID)
	if err != nil {
		return errors.New("PM2 belum terpasang di sistem")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gagal mengeksekusi %s pada proses %s (user %s): %s", action, targetID, ownerUser, strings.TrimSpace(string(out)))
	}

	// Jika menghapus proses, perbarui status penyimpanan PM2 milik user tersebut
	if action == "delete" {
		if saveCmd, errSave := buildUserPM2Command(ctx, ownerUser, "save"); errSave == nil {
			_ = saveCmd.Run()
		}
	}

	return nil
}

// GetLogs mengambil baris log stdout dan stderr proses dari PM2 atas nama user pemilik proses
func (m *Manager) GetLogs(ctx context.Context, idOrName string, lines int, explicitUser ...string) (string, error) {
	if lines <= 0 || lines > 500 {
		lines = 60
	}

	userArg := ""
	if len(explicitUser) > 0 {
		userArg = explicitUser[0]
	}
	ownerUser, targetID := m.resolveProcessOwner(ctx, idOrName, userArg)

	cmd, err := buildUserPM2Command(ctx, ownerUser, "logs", targetID, "--lines", strconv.Itoa(lines), "--nostream")
	if err != nil {
		return "", errors.New("PM2 belum terpasang di sistem")
	}
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
