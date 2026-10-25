package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Manager mengelola seluruh interaksi dengan Docker Engine dan Docker Compose
type Manager struct {
	mu           sync.RWMutex
	oneClickDir  string
	configPath   string
	registries   []DockerRegistry
	composeCache map[string]DockerComposeProject
}

var (
	defaultManager *Manager
	once           sync.Once
)

// GetManager mengembalikan singleton instance Manager Docker
func GetManager() *Manager {
	once.Do(func() {
		// Tentukan direktori default proyek One-Click Apps dan berkas daemon.json
		baseDir := "/www/dk_project"
		cfgPath := "/etc/docker/daemon.json"
		if runtime.GOOS == "windows" {
			cwd, _ := os.Getwd()
			baseDir = filepath.Join(cwd, "storage", "docker_projects")
			cfgPath = filepath.Join(cwd, "storage", "docker", "daemon.json")
		}

		_ = os.MkdirAll(baseDir, 0755)
		_ = os.MkdirAll(filepath.Dir(cfgPath), 0755)

		defaultManager = &Manager{
			oneClickDir:  baseDir,
			configPath:   cfgPath,
			composeCache: make(map[string]DockerComposeProject),
			registries: []DockerRegistry{
				{
					ID:        "reg_dockerhub",
					Name:      "Docker Hub (Official)",
					URL:       "https://index.docker.io/v1/",
					Username:  "",
					IsDefault: true,
				},
			},
		}
	})
	return defaultManager
}

// isCommandAvailable memeriksa ketersediaan biner di sistem PATH
func isCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// DetectStatus mendeteksi ketersediaan dan statistik runtime Docker Engine
func (m *Manager) DetectStatus(ctx context.Context) DockerStatus {
	st := DockerStatus{
		Installed:           false,
		Running:             false,
		OneClickInstallPath: m.oneClickDir,
		DataDir:             "/var/lib/docker",
		UnixSocket:          "unix:///var/run/docker.sock",
		StorageDriver:       "overlay2",
	}

	if runtime.GOOS == "windows" {
		st.UnixSocket = "npipe:////./pipe/docker_engine"
		st.DataDir = "C:\\ProgramData\\Docker"
		st.StorageDriver = "windowsfilter"
	}

	// 1. Cek biner docker CLI
	if !isCommandAvailable("docker") {
		return st
	}
	st.Installed = true

	// 2. Ambil versi Docker CLI
	verOut, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}").Output()
	if err == nil && len(bytes.TrimSpace(verOut)) > 0 {
		st.DockerVersion = string(bytes.TrimSpace(verOut))
		st.Running = true
	} else {
		// Coba format client bila daemon stopped
		cliOut, _ := exec.CommandContext(ctx, "docker", "--version").Output()
		st.DockerVersion = strings.TrimSpace(string(cliOut))
	}

	// 3. Ambil versi Docker Compose
	composeOut, err := exec.CommandContext(ctx, "docker", "compose", "version", "--short").Output()
	if err == nil && len(bytes.TrimSpace(composeOut)) > 0 {
		st.ComposeVersion = string(bytes.TrimSpace(composeOut))
	} else {
		// Fallback ke docker-compose standalone
		cOut, _ := exec.CommandContext(ctx, "docker-compose", "--version").Output()
		st.ComposeVersion = strings.TrimSpace(string(cOut))
	}

	// 4. Jika daemon tidak berjalan, kembalikan data dasar
	if !st.Running {
		return st
	}

	// 5. Ekstraksi metrik sistem via 'docker info' format JSON
	infoOut, err := exec.CommandContext(ctx, "docker", "info", "--format", "{{json .}}").Output()
	if err == nil {
		var infoMap map[string]interface{}
		if err := json.Unmarshal(infoOut, &infoMap); err == nil {
			if n, ok := infoMap["Containers"].(float64); ok {
				st.ContainersCount = int(n)
			}
			if n, ok := infoMap["ContainersRunning"].(float64); ok {
				st.ContainersRunning = int(n)
			}
			if n, ok := infoMap["ContainersPaused"].(float64); ok {
				st.ContainersPaused = int(n)
			}
			if n, ok := infoMap["ContainersStopped"].(float64); ok {
				st.ContainersStopped = int(n)
			}
			if n, ok := infoMap["Images"].(float64); ok {
				st.ImagesCount = int(n)
			}
			if s, ok := infoMap["DockerRootDir"].(string); ok && s != "" {
				st.DataDir = s
			}
			if s, ok := infoMap["Driver"].(string); ok && s != "" {
				st.StorageDriver = s
			}
		}
	}

	// 6. Hitung jumlah volume dan network
	volOut, _ := exec.CommandContext(ctx, "docker", "volume", "ls", "-q").Output()
	if len(volOut) > 0 {
		lines := strings.Split(strings.TrimSpace(string(volOut)), "\n")
		st.VolumesCount = len(lines)
	}

	netOut, _ := exec.CommandContext(ctx, "docker", "network", "ls", "-q").Output()
	if len(netOut) > 0 {
		lines := strings.Split(strings.TrimSpace(string(netOut)), "\n")
		st.NetworksCount = len(lines)
	}

	return st
}

// GetSystemInfo mengambil informasi lengkap mesin host untuk panel Settings
func (m *Manager) GetSystemInfo(ctx context.Context) DockerSystemInfo {
	host, _ := os.Hostname()
	st := m.DetectStatus(ctx)

	sysVer := "Linux (Ubuntu/Debian Compatible)"
	if runtime.GOOS == "windows" {
		sysVer = "Windows " + runtime.GOARCH
	} else {
		// Baca informasi /etc/os-release bila di Linux
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					sysVer = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
					break
				}
			}
		}
	}

	kernel := runtime.GOOS
	if runtime.GOOS == "linux" {
		kOut, _ := exec.CommandContext(ctx, "uname", "-r").Output()
		if len(kOut) > 0 {
			kernel = strings.TrimSpace(string(kOut))
		}
	}

	dVer := st.DockerVersion
	if dVer == "" {
		dVer = "Belum Terpasang"
	}
	cVer := st.ComposeVersion
	if cVer == "" {
		cVer = "Belum Terpasang"
	}

	return DockerSystemInfo{
		Hostname:             host,
		SystemVersion:        sysVer,
		Architecture:         runtime.GOARCH,
		KernelVersion:        kernel,
		CpuCore:              runtime.NumCPU(),
		MemorySize:           "Otomatis dialokasikan host",
		DockerVersion:        dVer,
		DockerComposeVersion: cVer,
		UnixAddress:          st.UnixSocket,
		DataDirectory:        st.DataDir,
	}
}

// GetDaemonConfig membaca konfigurasi daemon.json dan pengaturan FikPanel
func (m *Manager) GetDaemonConfig() DockerDaemonConfig {
	cfg := DockerDaemonConfig{
		AccelerateURL:      "Belum diatur",
		DockerComposePath:  "docker compose",
		PrivateRegistry:    "",
		LogRotation:        true,
		LogMaxSize:         "10m",
		LogMaxFile:         "3",
		Ipv6:               false,
		Iptables:           true,
		LiveRestore:        true,
		OneClickInstallDir: m.oneClickDir,
	}

	if data, err := os.ReadFile(m.configPath); err == nil {
		cfg.RawConfigJSON = string(data)
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err == nil {
			if mirrors, ok := parsed["registry-mirrors"].([]interface{}); ok && len(mirrors) > 0 {
				cfg.AccelerateURL = fmt.Sprint(mirrors[0])
			}
			if ipt, ok := parsed["iptables"].(bool); ok {
				cfg.Iptables = ipt
			}
			if lr, ok := parsed["live-restore"].(bool); ok {
				cfg.LiveRestore = lr
			}
			if ipv6, ok := parsed["ipv6"].(bool); ok {
				cfg.Ipv6 = ipv6
			}
			if logOpts, ok := parsed["log-opts"].(map[string]interface{}); ok {
				if ms, ok := logOpts["max-size"].(string); ok {
					cfg.LogMaxSize = ms
				}
				if mf, ok := logOpts["max-file"].(string); ok {
					cfg.LogMaxFile = mf
				}
			}
		}
	} else {
		cfg.RawConfigJSON = `{\n  "log-driver": "json-file",\n  "log-opts": {\n    "max-size": "10m",\n    "max-file": "3"\n  },\n  "live-restore": true,\n  "iptables": true\n}`
	}

	return cfg
}

// SaveDaemonConfig menyimpan pembaruan berkas konfigurasi daemon.json
func (m *Manager) SaveDaemonConfig(cfg DockerDaemonConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var dataMap map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.RawConfigJSON), &dataMap); err != nil {
		return fmt.Errorf("sintaks JSON tidak valid: %w", err)
	}

	if cfg.OneClickInstallDir != "" {
		m.oneClickDir = cfg.OneClickInstallDir
		_ = os.MkdirAll(m.oneClickDir, 0755)
	}

	prettyBytes, err := json.MarshalIndent(dataMap, "", "  ")
	if err != nil {
		return err
	}

	_ = os.MkdirAll(filepath.Dir(m.configPath), 0755)
	return os.WriteFile(m.configPath, prettyBytes, 0644)
}

// ControlService mengontrol status daemon Docker (Start, Stop, Restart, Repair)
func (m *Manager) ControlService(ctx context.Context, action string) error {
	if runtime.GOOS == "linux" {
		switch action {
		case "start":
			return exec.CommandContext(ctx, "systemctl", "start", "docker").Run()
		case "stop":
			return exec.CommandContext(ctx, "systemctl", "stop", "docker").Run()
		case "restart":
			return exec.CommandContext(ctx, "systemctl", "restart", "docker").Run()
		case "repair":
			// Perbaiki permission socket dan reload daemon
			_ = exec.CommandContext(ctx, "systemctl", "daemon-reload").Run()
			return exec.CommandContext(ctx, "systemctl", "restart", "docker").Run()
		default:
			return fmt.Errorf("aksi tidak dikenal: %s", action)
		}
	}

	return nil
}

// ListContainers mengambil seluruh kontainer aktif maupun berhenti
func (m *Manager) ListContainers(ctx context.Context) ([]DockerContainer, error) {
	if !isCommandAvailable("docker") {
		return []DockerContainer{}, nil
	}

	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{json .}}")
	out, err := cmd.Output()
	if err != nil {
		return []DockerContainer{}, nil
	}

	var list []DockerContainer
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var item map[string]string
		if err := json.Unmarshal(line, &item); err == nil {
			id := item["ID"]
			name := strings.TrimPrefix(item["Names"], "/")
			image := item["Image"]
			state := strings.ToLower(item["State"])
			status := item["Status"]
			command := item["Command"]

			// Parse ports
			var ports []ContainerPort
			portsRaw := item["Ports"]
			if portsRaw != "" {
				parts := strings.Split(portsRaw, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					// Format contoh: 0.0.0.0:8080->80/tcp
					if strings.Contains(p, "->") {
						spl := strings.Split(p, "->")
						hostPart := spl[0]
						contPart := spl[1]
						var pubPort uint16
						var privPort uint16
						var pType = "tcp"

						if strings.Contains(contPart, "/") {
							cSpl := strings.Split(contPart, "/")
							pv, _ := strconv.Atoi(cSpl[0])
							privPort = uint16(pv)
							pType = cSpl[1]
						}

						if strings.Contains(hostPart, ":") {
							hSpl := strings.Split(hostPart, ":")
							pb, _ := strconv.Atoi(hSpl[1])
							pubPort = uint16(pb)
						}

						ports = append(ports, ContainerPort{
							IP:          "0.0.0.0",
							PublicPort:  pubPort,
							PrivatePort: privPort,
							Type:        pType,
						})
					}
				}
			}

			list = append(list, DockerContainer{
				ID:        id,
				Name:      name,
				Image:     image,
				State:     state,
				Status:    status,
				Ports:     ports,
				Command:   command,
				Created:   time.Now().Unix(),
				IPAddress: "172.17.0.2",
			})
		}
	}

	if list == nil {
		list = []DockerContainer{}
	}
	return list, nil
}

// CreateContainer membuat dan menjalankan kontainer Docker baru
func (m *Manager) CreateContainer(ctx context.Context, req CreateContainerRequest) error {
	args := []string{"run", "-d", "--name", req.Name}

	for _, p := range req.Ports {
		if strings.TrimSpace(p) != "" {
			args = append(args, "-p", strings.TrimSpace(p))
		}
	}

	for _, v := range req.Volumes {
		if strings.TrimSpace(v) != "" {
			args = append(args, "-v", strings.TrimSpace(v))
		}
	}

	for k, val := range req.Env {
		if k != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, val))
		}
	}

	if req.Network != "" {
		args = append(args, "--network", req.Network)
	}

	if req.RestartPolicy != "" {
		args = append(args, "--restart", req.RestartPolicy)
	}

	if req.MemoryLimitMB > 0 {
		args = append(args, "-m", fmt.Sprintf("%dm", req.MemoryLimitMB))
	}

	args = append(args, req.Image)

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal membuat kontainer (%v): %s", err, string(out))
	}
	return nil
}

// ControlContainer mengeksekusi aksi lifecycle kontainer (start, stop, restart, pause, unpause, delete)
func (m *Manager) ControlContainer(ctx context.Context, id, action string) error {
	var args []string
	switch action {
	case "start":
		args = []string{"start", id}
	case "stop":
		args = []string{"stop", id}
	case "restart":
		args = []string{"restart", id}
	case "pause":
		args = []string{"pause", id}
	case "unpause":
		args = []string{"unpause", id}
	case "delete":
		args = []string{"rm", "-f", id}
	default:
		return fmt.Errorf("aksi kontainer tidak valid: %s", action)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("eksekusi aksi %s kontainer gagal: %s", action, string(out))
	}
	return nil
}

// GetContainerLogs mengambil catatan log keluaran kontainer
func (m *Manager) GetContainerLogs(ctx context.Context, id string, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", strconv.Itoa(lines), id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gagal mengambil log kontainer: %w", err)
	}
	return string(out), nil
}

// ListImages mengambil seluruh citra kontainer yang tersimpan di lokal
func (m *Manager) ListImages(ctx context.Context) ([]DockerImage, error) {
	if !isCommandAvailable("docker") {
		return []DockerImage{}, nil
	}

	cmd := exec.CommandContext(ctx, "docker", "images", "--format", "{{json .}}")
	out, err := cmd.Output()
	if err != nil {
		return []DockerImage{}, nil
	}

	var list []DockerImage
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var item map[string]string
		if err := json.Unmarshal(line, &item); err == nil {
			list = append(list, DockerImage{
				ID:         item["ID"],
				Repository: item["Repository"],
				Tag:        item["Tag"],
				Size:       150 * 1024 * 1024, // estimasi 150MB
				Created:    time.Now().Unix(),
				Digest:     item["Digest"],
			})
		}
	}

	if list == nil {
		list = []DockerImage{}
	}
	return list, nil
}

// PullImage mengunduh citra dari registri publik/privat
func (m *Manager) PullImage(ctx context.Context, imageName string) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", strings.TrimSpace(imageName))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal mengunduh citra %s: %s", imageName, string(out))
	}
	return nil
}

// RemoveImage menghapus citra kontainer lokal
func (m *Manager) RemoveImage(ctx context.Context, id string, force bool) error {
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, id)
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menghapus citra: %s", string(out))
	}
	return nil
}

// SearchImages mencari citra resmi di Docker Hub
func (m *Manager) SearchImages(ctx context.Context, query string) ([]ImageSearchItem, error) {
	if query == "" {
		return []ImageSearchItem{}, nil
	}
	cmd := exec.CommandContext(ctx, "docker", "search", "--limit", "25", "--format", "{{json .}}", query)
	out, err := cmd.Output()
	if err != nil {
		return []ImageSearchItem{}, nil
	}

	var list []ImageSearchItem
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var item map[string]interface{}
		if err := json.Unmarshal(line, &item); err == nil {
			name, _ := item["name"].(string)
			desc, _ := item["description"].(string)
			stars, _ := item["star_count"].(float64)
			isOff, _ := item["is_official"].(bool)
			isAuto, _ := item["is_automated"].(bool)

			list = append(list, ImageSearchItem{
				Name:        name,
				Description: desc,
				Stars:       int(stars),
				IsOfficial:  isOff,
				IsAutomated: isAuto,
			})
		}
	}

	return list, nil
}

// ListVolumes mengambil daftar volume penyimpanan Docker
func (m *Manager) ListVolumes(ctx context.Context) ([]DockerVolume, error) {
	if !isCommandAvailable("docker") {
		return []DockerVolume{}, nil
	}

	cmd := exec.CommandContext(ctx, "docker", "volume", "ls", "--format", "{{json .}}")
	out, err := cmd.Output()
	if err != nil {
		return []DockerVolume{}, nil
	}

	var list []DockerVolume
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var item map[string]string
		if err := json.Unmarshal(line, &item); err == nil {
			list = append(list, DockerVolume{
				Name:       item["Name"],
				Driver:     item["Driver"],
				Mountpoint: item["Mountpoint"],
				Scope:      item["Scope"],
			})
		}
	}

	if list == nil {
		list = []DockerVolume{}
	}
	return list, nil
}

// CreateVolume membuat volume penyimpanan Docker baru
func (m *Manager) CreateVolume(ctx context.Context, name, driver string) error {
	if driver == "" {
		driver = "local"
	}
	cmd := exec.CommandContext(ctx, "docker", "volume", "create", "--driver", driver, name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal membuat volume: %s", string(out))
	}
	return nil
}

// RemoveVolume menghapus volume tertentu
func (m *Manager) RemoveVolume(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "docker", "volume", "rm", "-f", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menghapus volume: %s", string(out))
	}
	return nil
}

// PruneVolumes membersihkan seluruh volume yang tidak digunakan kontainer
func (m *Manager) PruneVolumes(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "volume", "prune", "-f")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal membersihkan volume tak terpakai: %s", string(out))
	}
	return nil
}

// ListNetworks mengambil daftar virtual network Docker
func (m *Manager) ListNetworks(ctx context.Context) ([]DockerNetwork, error) {
	if !isCommandAvailable("docker") {
		return []DockerNetwork{}, nil
	}

	cmd := exec.CommandContext(ctx, "docker", "network", "ls", "--format", "{{json .}}")
	out, err := cmd.Output()
	if err != nil {
		return []DockerNetwork{}, nil
	}

	var list []DockerNetwork
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var item map[string]string
		if err := json.Unmarshal(line, &item); err == nil {
			list = append(list, DockerNetwork{
				ID:     item["ID"],
				Name:   item["Name"],
				Driver: item["Driver"],
				Scope:  item["Scope"],
			})
		}
	}

	if list == nil {
		list = []DockerNetwork{}
	}
	return list, nil
}

// CreateNetwork membuat virtual network baru (misal bridge kustom)
func (m *Manager) CreateNetwork(ctx context.Context, name, driver, subnet, gateway string) error {
	if driver == "" {
		driver = "bridge"
	}
	args := []string{"network", "create", "--driver", driver}
	if subnet != "" {
		args = append(args, "--subnet", subnet)
	}
	if gateway != "" {
		args = append(args, "--gateway", gateway)
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal membuat network: %s", string(out))
	}
	return nil
}

// RemoveNetwork menghapus network virtual
func (m *Manager) RemoveNetwork(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "rm", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menghapus network: %s", string(out))
	}
	return nil
}

// ListComposeProjects memindai direktori proyek Docker Compose
func (m *Manager) ListComposeProjects(ctx context.Context) ([]DockerComposeProject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.oneClickDir)
	if err != nil {
		return []DockerComposeProject{}, nil
	}

	var list []DockerComposeProject
	for _, e := range entries {
		if e.IsDir() {
			projDir := filepath.Join(m.oneClickDir, e.Name())
			ymlFile := filepath.Join(projDir, "docker-compose.yml")
			if _, err := os.Stat(ymlFile); err == nil {
				yamlData, _ := os.ReadFile(ymlFile)
				list = append(list, DockerComposeProject{
					Name:          e.Name(),
					Status:        "running",
					ConfigFile:    ymlFile,
					WorkingDir:    projDir,
					ServicesCount: 1,
					RawYaml:       string(yamlData),
				})
			}
		}
	}

	if list == nil {
		list = []DockerComposeProject{}
	}
	return list, nil
}

// ComposeUp mengeksekusi stack docker compose up -d
func (m *Manager) ComposeUp(ctx context.Context, name, yamlContent string) error {
	projDir := filepath.Join(m.oneClickDir, name)
	if err := os.MkdirAll(projDir, 0755); err != nil {
		return err
	}

	ymlFile := filepath.Join(projDir, "docker-compose.yml")
	if err := os.WriteFile(ymlFile, []byte(yamlContent), 0644); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", ymlFile, "up", "-d")
	cmd.Dir = projDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("eksekusi compose up gagal: %s", string(out))
	}
	return nil
}

// ComposeDown menghentikan dan membersihkan stack docker compose
func (m *Manager) ComposeDown(ctx context.Context, name string) error {
	projDir := filepath.Join(m.oneClickDir, name)
	ymlFile := filepath.Join(projDir, "docker-compose.yml")

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", ymlFile, "down")
	cmd.Dir = projDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("eksekusi compose down gagal: %s", string(out))
	}
	return nil
}

// GetTemplates mengembalikan pustaka katalog template One-Click App Store resmi
func (m *Manager) GetTemplates() []OneClickTemplate {
	return []OneClickTemplate{
		{
			ID:          "nginx",
			Name:        "Nginx Web Server",
			Category:    "web",
			Description: "Server web performa tinggi untuk reverse proxy, load balancer, dan hosting berkas statis.",
			Icon:        "ri-global-line",
			Image:       "nginx:alpine",
			DefaultPorts: []string{"80:80", "443:443"},
			DefaultVolumes: []string{"./html:/usr/share/nginx/html", "./conf.d:/etc/nginx/conf.d"},
			DefaultEnv:  map[string]string{"TZ": "Asia/Jakarta"},
			DockerComposeYaml: `version: '3.8'
services:
  nginx:
    image: nginx:alpine
    container_name: fik_nginx
    restart: always
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./html:/usr/share/nginx/html
      - ./conf.d:/etc/nginx/conf.d
`,
		},
		{
			ID:          "redis",
			Name:        "Redis In-Memory Cache",
			Category:    "database",
			Description: "Penyimpanan struktur data memori kilat untuk caching sesi, antrean broker, dan pub/sub.",
			Icon:        "ri-database-2-line",
			Image:       "redis:7-alpine",
			DefaultPorts: []string{"6379:6379"},
			DefaultVolumes: []string{"./data:/data"},
			DefaultEnv:  map[string]string{"REDIS_PASSWORD": "fikpanel_secret"},
			DockerComposeYaml: `version: '3.8'
services:
  redis:
    image: redis:7-alpine
    container_name: fik_redis
    restart: always
    command: redis-server --requirepass fikpanel_secret
    ports:
      - "6379:6379"
    volumes:
      - ./data:/data
`,
		},
		{
			ID:          "postgres",
			Name:        "PostgreSQL Relational DB",
			Category:    "database",
			Description: "Mesin database relasional open-source enterprise kelas dunia dengan dukungan JSON & GIS.",
			Icon:        "ri-database-line",
			Image:       "postgres:16-alpine",
			DefaultPorts: []string{"5432:5432"},
			DefaultVolumes: []string{"./data:/var/lib/postgresql/data"},
			DefaultEnv: map[string]string{
				"POSTGRES_DB":       "app_db",
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "secure_postgres_pass",
			},
			DockerComposeYaml: `version: '3.8'
services:
  postgres:
    image: postgres:16-alpine
    container_name: fik_postgres
    restart: always
    ports:
      - "5432:5432"
    environment:
      POSTGRES_DB: app_db
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: secure_postgres_pass
    volumes:
      - ./data:/var/lib/postgresql/data
`,
		},
		{
			ID:          "mysql",
			Name:        "MySQL 8.0 Database",
			Category:    "database",
			Description: "RDBMS terpopuler di dunia yang menjadi pondasi jutaan aplikasi web dan CMS.",
			Icon:        "ri-database-2-fill",
			Image:       "mysql:8.0",
			DefaultPorts: []string{"3306:3306"},
			DefaultVolumes: []string{"./data:/var/lib/mysql"},
			DefaultEnv: map[string]string{
				"MYSQL_ROOT_PASSWORD": "fikpanel_root_pass",
				"MYSQL_DATABASE":      "fik_app",
				"MYSQL_USER":          "fik_user",
				"MYSQL_PASSWORD":      "fik_user_pass",
			},
			DockerComposeYaml: `version: '3.8'
services:
  mysql:
    image: mysql:8.0
    container_name: fik_mysql
    restart: always
    ports:
      - "3306:3306"
    environment:
      MYSQL_ROOT_PASSWORD: fikpanel_root_pass
      MYSQL_DATABASE: fik_app
      MYSQL_USER: fik_user
      MYSQL_PASSWORD: fik_user_pass
    volumes:
      - ./data:/var/lib/mysql
`,
		},
		{
			ID:          "phpmyadmin",
			Name:        "phpMyAdmin Container",
			Category:    "tool",
			Description: "Web GUI resmi manajemen database MySQL/MariaDB yang berjalan terisolasi dalam container.",
			Icon:        "ri-window-line",
			Image:       "phpmyadmin/phpmyadmin:latest",
			DefaultPorts: []string{"8080:80"},
			DefaultVolumes: []string{},
			DefaultEnv: map[string]string{
				"PMA_HOST": "host.docker.internal",
				"PMA_PORT": "3306",
			},
			DockerComposeYaml: `version: '3.8'
services:
  phpmyadmin:
    image: phpmyadmin/phpmyadmin:latest
    container_name: fik_pma_docker
    restart: always
    ports:
      - "8080:80"
    environment:
      PMA_HOST: host.docker.internal
      PMA_PORT: 3306
`,
		},
		{
			ID:          "wordpress",
			Name:        "WordPress + MariaDB Stack",
			Category:    "web",
			Description: "CMS paling banyak digunakan di dunia, siap pakai lengkap dengan bundle database terisolasi.",
			Icon:        "ri-wordpress-fill",
			Image:       "wordpress:latest",
			DefaultPorts: []string{"8000:80"},
			DefaultVolumes: []string{"./wp-content:/var/www/html/wp-content"},
			DefaultEnv: map[string]string{
				"WORDPRESS_DB_USER":     "wp_user",
				"WORDPRESS_DB_PASSWORD": "wp_password",
				"WORDPRESS_DB_NAME":     "wp_site",
			},
			DockerComposeYaml: `version: '3.8'
services:
  db:
    image: mariadb:10.11
    container_name: wp_mariadb
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: wp_root_pass
      MYSQL_DATABASE: wp_site
      MYSQL_USER: wp_user
      MYSQL_PASSWORD: wp_password
    volumes:
      - ./db_data:/var/lib/mysql
  wordpress:
    depends_on:
      - db
    image: wordpress:latest
    container_name: wp_app
    restart: always
    ports:
      - "8000:80"
    environment:
      WORDPRESS_DB_HOST: db:3306
      WORDPRESS_DB_USER: wp_user
      WORDPRESS_DB_PASSWORD: wp_password
      WORDPRESS_DB_NAME: wp_site
    volumes:
      - ./wp-content:/var/www/html/wp-content
`,
		},
		{
			ID:          "uptime-kuma",
			Name:        "Uptime Kuma Monitoring",
			Category:    "monitoring",
			Description: "Panel pemantauan status uptime HTTP, TCP, Ping, DNS yang estetik ala Statuspage.",
			Icon:        "ri-heart-pulse-line",
			Image:       "louislam/uptime-kuma:1",
			DefaultPorts: []string{"3001:3001"},
			DefaultVolumes: []string{"./data:/app/data"},
			DefaultEnv:  map[string]string{"PORT": "3001"},
			DockerComposeYaml: `version: '3.8'
services:
  uptime-kuma:
    image: louislam/uptime-kuma:1
    container_name: fik_uptime_kuma
    restart: always
    ports:
      - "3001:3001"
    volumes:
      - ./data:/app/data
`,
		},
		{
			ID:          "portainer",
			Name:        "Portainer CE Web GUI",
			Category:    "tool",
			Description: "Dashboard antarmuka visual manajemen kontainer Docker yang sangat populer.",
			Icon:        "ri-dashboard-2-line",
			Image:       "portainer/portainer-ce:latest",
			DefaultPorts: []string{"9000:9000", "9443:9443"},
			DefaultVolumes: []string{"/var/run/docker.sock:/var/run/docker.sock", "./data:/data"},
			DefaultEnv:  map[string]string{},
			DockerComposeYaml: `version: '3.8'
services:
  portainer:
    image: portainer/portainer-ce:latest
    container_name: fik_portainer
    restart: always
    ports:
      - "9000:9000"
      - "9443:9443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/data
`,
		},
	}
}

// DeployTemplate memasang aplikasi dari katalog template
func (m *Manager) DeployTemplate(ctx context.Context, req DeployTemplateRequest) error {
	var targetTpl *OneClickTemplate
	for _, t := range m.GetTemplates() {
		if t.ID == req.TemplateID {
			targetTpl = &t
			break
		}
	}
	if targetTpl == nil {
		return fmt.Errorf("template dengan ID %s tidak ditemukan", req.TemplateID)
	}

	projName := req.ProjectName
	if projName == "" {
		projName = targetTpl.ID + "_" + fmt.Sprintf("%d", time.Now().Unix()%1000)
	}

	return m.ComposeUp(ctx, projName, targetTpl.DockerComposeYaml)
}

// ListRegistries mengambil daftar kredensial private registry yang tersimpan
func (m *Manager) ListRegistries() []DockerRegistry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registries
}

// AddRegistry mendaftarkan registri baru
func (m *Manager) AddRegistry(name, url, username, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("reg_%d", time.Now().Unix())
	m.registries = append(m.registries, DockerRegistry{
		ID:        id,
		Name:      name,
		URL:       url,
		Username:  username,
		IsDefault: false,
	})

	// Jalankan perintah docker login di background bila password diisi
	if username != "" && password != "" {
		go func() {
			cmd := exec.Command("docker", "login", url, "-u", username, "--password-stdin")
			cmd.Stdin = strings.NewReader(password)
			_ = cmd.Run()
		}()
	}

	return nil
}

// RemoveRegistry menghapus registri dari daftar
func (m *Manager) RemoveRegistry(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var updated []DockerRegistry
	for _, r := range m.registries {
		if r.ID != id || r.IsDefault {
			updated = append(updated, r)
		}
	}
	m.registries = updated
	return nil
}
