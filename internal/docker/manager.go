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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Manager mengelola seluruh interaksi dengan Docker Engine, Docker Compose, dan Docker Hub
type Manager struct {
	mu           sync.RWMutex
	oneClickDir  string
	configPath   string
	registries   []DockerRegistry
	saveRegsFunc func([]DockerRegistry) error
	tasks        *StreamTaskRegistry
}

var (
	defaultManager *Manager
	once           sync.Once
)

func defaultRegistries() []DockerRegistry {
	return []DockerRegistry{
		{
			ID:        "reg_dockerhub",
			Name:      "Docker Hub (Official)",
			URL:       "https://index.docker.io/v1/",
			Username:  "",
			IsDefault: true,
			LoggedIn:  false,
		},
	}
}

// GetManager mengembalikan singleton instance Manager Docker
func GetManager() *Manager {
	once.Do(func() {
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
			oneClickDir: baseDir,
			configPath:  cfgPath,
			registries:  defaultRegistries(),
			tasks:       newStreamTaskRegistry(),
		}
	})
	return defaultManager
}

// Tasks mengembalikan registri tugas streaming Docker untuk konsumsi handler SSE
func (m *Manager) Tasks() *StreamTaskRegistry {
	return m.tasks
}

// SetRegistryPersistence menghubungkan penyimpanan registri Docker ke basis data persisten panel (SQLite)
func (m *Manager) SetRegistryPersistence(initial []DockerRegistry, saveFn func([]DockerRegistry) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveRegsFunc = saveFn
	if len(initial) > 0 {
		// Pastikan Docker Hub resmi selalu tersedia sebagai entri default
		hasDefault := false
		for _, r := range initial {
			if r.IsDefault || r.ID == "reg_dockerhub" {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			m.registries = append(defaultRegistries(), initial...)
		} else {
			m.registries = initial
		}
	}
}

func isCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// composeBaseArgs menentukan perintah Compose yang tersedia (`docker compose` v2 atau `docker-compose` v1)
func (m *Manager) composeBaseArgs(ctx context.Context) (string, []string) {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := exec.CommandContext(cctx, "docker", "compose", "version").Run(); err == nil {
		return "docker", []string{"compose"}
	}
	if isCommandAvailable("docker-compose") {
		return "docker-compose", nil
	}
	return "docker", []string{"compose"}
}

// DetectStatus mendeteksi ketersediaan, versi, jumlah sumber daya, serta pemakaian disk Docker Engine
func (m *Manager) DetectStatus(ctx context.Context) DockerStatus {
	st := DockerStatus{
		Installed:           false,
		Running:             false,
		OneClickInstallPath: m.oneClickDir,
		RootDir:             "/var/lib/docker",
		DataDir:             "/var/lib/docker",
		UnixSocket:          "unix:///var/run/docker.sock",
		StorageDriver:       "overlay2",
	}

	if !isCommandAvailable("docker") {
		return st
	}
	st.Installed = true

	// Cek versi Docker CLI (timeout singkat 3 detik)
	vCtx, vCancel := context.WithTimeout(ctx, 3*time.Second)
	if out, err := exec.CommandContext(vCtx, "docker", "version", "--format", "{{.Client.Version}}").Output(); err == nil {
		ver := strings.TrimSpace(string(out))
		if ver != "" {
			st.Version = ver
			st.DockerVersion = ver
		}
	}
	vCancel()

	// Cek ketersediaan Docker Compose v2 / v1
	cCtx, cCancel := context.WithTimeout(ctx, 3*time.Second)
	if out, err := exec.CommandContext(cCtx, "docker", "compose", "version", "--short").Output(); err == nil {
		st.ComposeInstalled = true
		st.ComposeVersion = strings.TrimSpace(string(out))
	} else if isCommandAvailable("docker-compose") {
		if out2, err2 := exec.CommandContext(cCtx, "docker-compose", "version", "--short").Output(); err2 == nil {
			st.ComposeInstalled = true
			st.ComposeVersion = strings.TrimSpace(string(out2))
		}
	}
	cCancel()

	// Ambil snapshot lengkap dari `docker info --format "{{json .}}"` dalam satu pemanggilan cepat
	iCtx, iCancel := context.WithTimeout(ctx, 4*time.Second)
	infoOut, err := exec.CommandContext(iCtx, "docker", "info", "--format", "{{json .}}").Output()
	iCancel()
	if err != nil {
		return st
	}

	st.Running = true
	var info struct {
		ServerVersion     string `json:"ServerVersion"`
		Driver            string `json:"Driver"`
		DockerRootDir     string `json:"DockerRootDir"`
		Containers        int    `json:"Containers"`
		ContainersRunning int    `json:"ContainersRunning"`
		ContainersPaused  int    `json:"ContainersPaused"`
		ContainersStopped int    `json:"ContainersStopped"`
		Images            int    `json:"Images"`
		MemoryLimit       bool   `json:"MemoryLimit"`
		SwapLimit         bool   `json:"SwapLimit"`
	}
	if json.Unmarshal(infoOut, &info) == nil {
		if info.ServerVersion != "" {
			st.ServerVersion = info.ServerVersion
			st.Version = info.ServerVersion
			st.DockerVersion = info.ServerVersion
		}
		if info.Driver != "" {
			st.StorageDriver = info.Driver
		}
		if info.DockerRootDir != "" {
			st.RootDir = info.DockerRootDir
			st.DataDir = info.DockerRootDir
		}
		st.ContainersTotal = info.Containers
		st.ContainersCount = info.Containers
		st.ContainersRunning = info.ContainersRunning
		st.ContainersPaused = info.ContainersPaused
		st.ContainersStopped = info.ContainersStopped
		st.ImagesTotal = info.Images
		st.ImagesCount = info.Images
		st.MemoryLimit = info.MemoryLimit
		st.SwapLimit = info.SwapLimit
	}

	// Hitung volume, network, compose stack, dan pemakaian disk secara paralel agar respons super cepat
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(tctx, "docker", "volume", "ls", "-q").Output(); err == nil {
			cnt := countNonEmptyLines(string(out))
			st.VolumesTotal = cnt
			st.VolumesCount = cnt
		}
	}()
	go func() {
		defer wg.Done()
		tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(tctx, "docker", "network", "ls", "-q").Output(); err == nil {
			cnt := countNonEmptyLines(string(out))
			st.NetworksTotal = cnt
			st.NetworksCount = cnt
		}
	}()
	go func() {
		defer wg.Done()
		tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(tctx, "docker", "compose", "ls", "-a", "-q").Output(); err == nil {
			st.ComposeStacksTotal = countNonEmptyLines(string(out))
		}
	}()
	go func() {
		defer wg.Done()
		if df, err := m.GetDiskUsage(ctx); err == nil {
			st.DiskUsage = df
		}
	}()
	wg.Wait()

	return st
}

func countNonEmptyLines(s string) int {
	cnt := 0
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if strings.TrimSpace(line) != "" {
			cnt++
		}
	}
	return cnt
}

// GetDiskUsage membaca ringkasan pemakaian ruang disk Docker dari `docker system df`
func (m *Manager) GetDiskUsage(ctx context.Context) (*DockerDiskUsage, error) {
	cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", "system", "df", "--format", "{{json .}}").Output()
	if err != nil {
		return nil, err
	}
	return ParseSystemDFOutput(string(out)), nil
}

// ParseSystemDFOutput mengurai keluaran JSON-per-baris dari `docker system df --format "{{json .}}"`
func ParseSystemDFOutput(raw string) *DockerDiskUsage {
	df := &DockerDiskUsage{
		Images:     DockerDiskUsageCategory{Type: "Images", SizeHuman: "0 B", ReclaimableHuman: "0 B"},
		Containers: DockerDiskUsageCategory{Type: "Containers", SizeHuman: "0 B", ReclaimableHuman: "0 B"},
		Volumes:    DockerDiskUsageCategory{Type: "Local Volumes", SizeHuman: "0 B", ReclaimableHuman: "0 B"},
		BuildCache: DockerDiskUsageCategory{Type: "Build Cache", SizeHuman: "0 B", ReclaimableHuman: "0 B"},
	}

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row struct {
			Type        string `json:"Type"`
			TotalCount  string `json:"TotalCount"`
			Active      string `json:"Active"`
			Size        string `json:"Size"`
			Reclaimable string `json:"Reclaimable"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		totalCnt, _ := strconv.Atoi(strings.TrimSpace(row.TotalCount))
		activeCnt, _ := strconv.Atoi(strings.TrimSpace(row.Active))
		sizeBytes := ParseHumanSize(row.Size)
		recBytes, recHuman, recPct := ParseReclaimableField(row.Reclaimable)

		cat := DockerDiskUsageCategory{
			Type:             row.Type,
			TotalCount:       totalCnt,
			ActiveCount:      activeCnt,
			SizeBytes:        sizeBytes,
			SizeHuman:        FormatBytes(sizeBytes),
			ReclaimableBytes: recBytes,
			ReclaimableHuman: recHuman,
			ReclaimablePct:   recPct,
		}

		switch strings.ToLower(strings.TrimSpace(row.Type)) {
		case "images":
			df.Images = cat
		case "containers":
			df.Containers = cat
		case "local volumes", "volumes":
			df.Volumes = cat
		case "build cache":
			df.BuildCache = cat
		}
	}

	df.TotalSizeBytes = df.Images.SizeBytes + df.Containers.SizeBytes + df.Volumes.SizeBytes + df.BuildCache.SizeBytes
	df.TotalSizeHuman = FormatBytes(df.TotalSizeBytes)
	df.TotalReclaimableBytes = df.Images.ReclaimableBytes + df.Containers.ReclaimableBytes + df.Volumes.ReclaimableBytes + df.BuildCache.ReclaimableBytes
	df.TotalReclaimableHuman = FormatBytes(df.TotalReclaimableBytes)
	return df
}

// SystemPrune menjalankan pembersihan selektif (container mati, image usang, volume bebas, network, build cache)
func (m *Manager) SystemPrune(ctx context.Context, req SystemPruneRequest) (*SystemPruneResult, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	res := &SystemPruneResult{Details: make([]string, 0)}

	runPrune := func(label string, args ...string) {
		out, err := exec.CommandContext(cctx, "docker", args...).CombinedOutput()
		text := strings.TrimSpace(string(out))
		if err != nil {
			res.Details = append(res.Details, fmt.Sprintf("%s: gagal (%s)", label, text))
			return
		}
		// Cari baris "Total reclaimed space: ..."
		var reclaimed int64
		for _, line := range strings.Split(text, "\n") {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "reclaimed space:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					reclaimed = ParseHumanSize(parts[1])
				}
			}
		}
		res.ReclaimedBytes += reclaimed
		res.Details = append(res.Details, fmt.Sprintf("%s: dibersihkan (%s)", label, FormatBytes(reclaimed)))
	}

	if req.Containers {
		runPrune("Kontainer Berhenti", "container", "prune", "-f")
	}
	if req.Images || req.AllImages {
		if req.AllImages {
			runPrune("Semua Citra Tak Terpakai", "image", "prune", "-a", "-f")
		} else {
			runPrune("Citra Dangling", "image", "prune", "-f")
		}
	}
	if req.Volumes {
		runPrune("Volume Bebas", "volume", "prune", "-f")
	}
	if req.Networks {
		runPrune("Jaringan Tak Terpakai", "network", "prune", "-f")
	}
	if req.BuildCache {
		runPrune("Build Cache", "builder", "prune", "-f")
	}

	res.ReclaimedHuman = FormatBytes(res.ReclaimedBytes)
	return res, nil
}

// GetSystemInfo menyajikan detail perangkat keras host, versi OS, RAM aktual, serta versi Docker & Compose
func (m *Manager) GetSystemInfo(ctx context.Context) DockerSystemInfo {
	hostname, _ := os.Hostname()
	osPretty := ReadHostOSPrettyName()
	if osPretty == "" {
		osPretty = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	}

	memTotal := ReadHostMemoryBytes()
	info := DockerSystemInfo{
		Hostname:         hostname,
		OSVersion:        osPretty,
		SystemVersion:    osPretty,
		Architecture:     runtime.GOARCH,
		KernelVersion:    "-",
		CPUCores:         runtime.NumCPU(),
		MemoryTotalBytes: memTotal,
		MemorySize:       FormatBytes(memTotal),
		DockerVersion:    "-",
		ComposeVersion:   "-",
		StorageDriver:    "overlay2",
		UnixAddress:      "unix:///var/run/docker.sock",
		DataDirectory:    "/var/lib/docker",
	}

	cctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	if runtime.GOOS != "windows" {
		if out, err := exec.CommandContext(cctx, "uname", "-r").Output(); err == nil {
			info.KernelVersion = strings.TrimSpace(string(out))
		}
	}

	// Perkaya dari `docker info --format "{{json .}}"` (termasuk MemTotal & OperatingSystem bila /proc tidak ada)
	if out, err := exec.CommandContext(cctx, "docker", "info", "--format", "{{json .}}").Output(); err == nil {
		var di struct {
			ServerVersion   string `json:"ServerVersion"`
			Driver          string `json:"Driver"`
			DockerRootDir   string `json:"DockerRootDir"`
			OperatingSystem string `json:"OperatingSystem"`
			KernelVersion   string `json:"KernelVersion"`
			Architecture    string `json:"Architecture"`
			NCPU            int    `json:"NCPU"`
			MemTotal        int64  `json:"MemTotal"`
			Name            string `json:"Name"`
		}
		if json.Unmarshal(out, &di) == nil {
			if di.Name != "" {
				info.Hostname = di.Name
			}
			if di.OperatingSystem != "" && (osPretty == "" || strings.Contains(osPretty, "/")) {
				info.OSVersion = di.OperatingSystem
				info.SystemVersion = di.OperatingSystem
			}
			if di.KernelVersion != "" {
				info.KernelVersion = di.KernelVersion
			}
			if di.Architecture != "" {
				info.Architecture = di.Architecture
			}
			if di.NCPU > 0 {
				info.CPUCores = di.NCPU
			}
			if di.MemTotal > 0 && info.MemoryTotalBytes == 0 {
				info.MemoryTotalBytes = di.MemTotal
				info.MemorySize = FormatBytes(di.MemTotal)
			}
			if di.ServerVersion != "" {
				info.DockerVersion = di.ServerVersion
			}
			if di.Driver != "" {
				info.StorageDriver = di.Driver
			}
			if di.DockerRootDir != "" {
				info.DataDirectory = di.DockerRootDir
			}
		}
	}

	if out, err := exec.CommandContext(cctx, "docker", "compose", "version", "--short").Output(); err == nil {
		info.ComposeVersion = strings.TrimSpace(string(out))
	} else if isCommandAvailable("docker-compose") {
		if out2, err2 := exec.CommandContext(cctx, "docker-compose", "version", "--short").Output(); err2 == nil {
			info.ComposeVersion = strings.TrimSpace(string(out2))
		}
	}

	if info.MemoryTotalBytes == 0 {
		info.MemorySize = "Tidak terdeteksi"
	}
	return info
}

// GetDaemonConfig membaca konfigurasi /etc/docker/daemon.json
func (m *Manager) GetDaemonConfig() DockerDaemonConfig {
	m.mu.RLock()
	cfgPath := m.configPath
	oneClickDir := m.oneClickDir
	m.mu.RUnlock()

	cfg := DockerDaemonConfig{
		RegistryMirrors: []string{},
		ComposePath:     "/usr/local/bin/docker-compose",
		PrivateRegistry: "official",
		LogRotation:     true,
		LogMaxSize:      "10m",
		LogMaxFile:      "3",
		IPv6:            false,
		IPv6Subnet:      "fd00::/80",
		IPTables:        true,
		LiveRestore:     true,
		OneClickDir:     oneClickDir,
		RawJSON:         "{}",
	}

	if path, err := exec.LookPath("docker-compose"); err == nil && path != "" {
		cfg.ComposePath = path
	}

	raw, err := os.ReadFile(cfgPath)
	if err == nil && len(bytes.TrimSpace(raw)) > 0 {
		cfg.RawJSON = string(raw)
		var rawMap map[string]interface{}
		if json.Unmarshal(raw, &rawMap) == nil {
			if mirrors, ok := rawMap["registry-mirrors"].([]interface{}); ok {
				for _, item := range mirrors {
					if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
						cfg.RegistryMirrors = append(cfg.RegistryMirrors, s)
					}
				}
			}
			if v, ok := rawMap["ipv6"].(bool); ok {
				cfg.IPv6 = v
			}
			if v, ok := rawMap["fixed-cidr-v6"].(string); ok && v != "" {
				cfg.IPv6Subnet = v
			}
			if v, ok := rawMap["iptables"].(bool); ok {
				cfg.IPTables = v
			}
			if v, ok := rawMap["live-restore"].(bool); ok {
				cfg.LiveRestore = v
			}
			if logOpts, ok := rawMap["log-opts"].(map[string]interface{}); ok {
				if v, ok := logOpts["max-size"].(string); ok && v != "" {
					cfg.LogMaxSize = v
				}
				if v, ok := logOpts["max-file"].(string); ok && v != "" {
					cfg.LogMaxFile = v
				}
				cfg.LogRotation = true
			}
		}
	} else {
		// Hasilkan representasi JSON rapi default agar modal editor langsung terisi valid
		defaultObj := map[string]interface{}{
			"log-driver": "json-file",
			"log-opts": map[string]string{
				"max-size": cfg.LogMaxSize,
				"max-file": cfg.LogMaxFile,
			},
			"live-restore": cfg.LiveRestore,
			"iptables":     cfg.IPTables,
		}
		if b, err := json.MarshalIndent(defaultObj, "", "  "); err == nil {
			cfg.RawJSON = string(b)
		}
	}
	return cfg
}

// SaveDaemonConfig memvalidasi dan menyimpan konfigurasi ke /etc/docker/daemon.json
func (m *Manager) SaveDaemonConfig(cfg DockerDaemonConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(cfg.OneClickDir) != "" {
		m.oneClickDir = strings.TrimSpace(cfg.OneClickDir)
		_ = os.MkdirAll(m.oneClickDir, 0755)
	}

	var output []byte
	if strings.TrimSpace(cfg.RawJSON) != "" {
		// Validasi JSON mentah bila pengguna menyimpan langsung dari editor daemon.json
		var check map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.RawJSON), &check); err != nil {
			return fmt.Errorf("format JSON daemon.json tidak valid: %w", err)
		}
		formatted, err := json.MarshalIndent(check, "", "  ")
		if err != nil {
			return err
		}
		output = formatted
	} else {
		// Baca konfigurasi lama agar field kustom pengguna lain tidak hilang
		data := make(map[string]interface{})
		if oldRaw, err := os.ReadFile(m.configPath); err == nil {
			_ = json.Unmarshal(oldRaw, &data)
		}

		if len(cfg.RegistryMirrors) > 0 {
			data["registry-mirrors"] = cfg.RegistryMirrors
		} else {
			delete(data, "registry-mirrors")
		}

		if cfg.LogRotation {
			data["log-driver"] = "json-file"
			maxSize := strings.TrimSpace(cfg.LogMaxSize)
			if maxSize == "" {
				maxSize = "10m"
			}
			maxFile := strings.TrimSpace(cfg.LogMaxFile)
			if maxFile == "" {
				maxFile = "3"
			}
			data["log-opts"] = map[string]string{
				"max-size": maxSize,
				"max-file": maxFile,
			}
		} else {
			delete(data, "log-opts")
		}

		data["ipv6"] = cfg.IPv6
		if cfg.IPv6 {
			subnet := strings.TrimSpace(cfg.IPv6Subnet)
			if subnet == "" {
				subnet = "fd00::/80"
			}
			data["fixed-cidr-v6"] = subnet
		} else {
			delete(data, "fixed-cidr-v6")
		}

		data["iptables"] = cfg.IPTables
		data["live-restore"] = cfg.LiveRestore

		formatted, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		output = formatted
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0755); err != nil {
		return fmt.Errorf("gagal membuat direktori konfigurasi: %w", err)
	}
	if err := os.WriteFile(m.configPath, output, 0644); err != nil {
		return fmt.Errorf("gagal menulis berkas %s: %w", m.configPath, err)
	}
	return nil
}

// ControlService mengendalikan layanan systemd Docker Engine (start, stop, restart, repair)
func (m *Manager) ControlService(ctx context.Context, action string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("kontrol layanan systemd Docker hanya didukung pada server Linux")
	}
	cctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	switch action {
	case "start", "stop", "restart":
		out, err := exec.CommandContext(cctx, "systemctl", action, "docker").CombinedOutput()
		if err != nil {
			return fmt.Errorf("gagal %s layanan docker: %s", action, strings.TrimSpace(string(out)))
		}
		return nil
	case "repair":
		_ = exec.CommandContext(cctx, "systemctl", "daemon-reload").Run()
		_ = exec.CommandContext(cctx, "systemctl", "restart", "containerd").Run()
		out, err := exec.CommandContext(cctx, "systemctl", "restart", "docker").CombinedOutput()
		if err != nil {
			return fmt.Errorf("gagal memperbaiki layanan docker: %s", strings.TrimSpace(string(out)))
		}
		return nil
	default:
		return fmt.Errorf("aksi layanan tidak dikenal: %s", action)
	}
}

type containerStatSnapshot struct {
	CPUPercent    float64
	MemUsageBytes int64
	MemLimitBytes int64
	MemUsageStr   string
	NetIO         string
	BlockIO       string
}

func (m *Manager) fetchContainerStats(ctx context.Context) map[string]containerStatSnapshot {
	result := make(map[string]containerStatSnapshot)
	cctx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", "stats", "--no-stream", "--no-trunc", "--format", "{{json .}}").Output()
	if err != nil {
		return result
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row struct {
			ID       string `json:"ID"`
			Name     string `json:"Name"`
			CPUPerc  string `json:"CPUPerc"`
			MemUsage string `json:"MemUsage"`
			NetIO    string `json:"NetIO"`
			BlockIO  string `json:"BlockIO"`
		}
		if json.Unmarshal([]byte(line), &row) != nil {
			continue
		}
		cpuVal, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(row.CPUPerc), "%"), 64)
		var memUsed, memLimit int64
		if parts := strings.SplitN(row.MemUsage, "/", 2); len(parts) == 2 {
			memUsed = ParseHumanSize(parts[0])
			memLimit = ParseHumanSize(parts[1])
		}
		snap := containerStatSnapshot{
			CPUPercent:    cpuVal,
			MemUsageBytes: memUsed,
			MemLimitBytes: memLimit,
			MemUsageStr:   strings.TrimSpace(row.MemUsage),
			NetIO:         strings.TrimSpace(row.NetIO),
			BlockIO:       strings.TrimSpace(row.BlockIO),
		}
		if row.ID != "" {
			result[row.ID] = snap
			if len(row.ID) >= 12 {
				result[row.ID[:12]] = snap
			}
		}
		if row.Name != "" {
			result[strings.TrimPrefix(row.Name, "/")] = snap
		}
	}
	return result
}

// ListContainers mengambil seluruh kontainer aktif & berhenti dilengkapi metrik CPU/RAM real-time
func (m *Manager) ListContainers(ctx context.Context) ([]DockerContainer, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var (
		psOut   []byte
		psErr   error
		statMap map[string]containerStatSnapshot
	)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		psOut, psErr = exec.CommandContext(cctx, "docker", "ps", "-a", "--no-trunc", "--format", "{{json .}}").Output()
	}()
	go func() {
		defer wg.Done()
		statMap = m.fetchContainerStats(cctx)
	}()
	wg.Wait()

	if psErr != nil {
		return []DockerContainer{}, psErr
	}

	containers := make([]DockerContainer, 0)
	scanner := bufio.NewScanner(bytes.NewReader(psOut))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw struct {
			ID         string `json:"ID"`
			Names      string `json:"Names"`
			Image      string `json:"Image"`
			Command    string `json:"Command"`
			State      string `json:"State"`
			Status     string `json:"Status"`
			CreatedAt  string `json:"CreatedAt"`
			RunningFor string `json:"RunningFor"`
			Ports      string `json:"Ports"`
			Networks   string `json:"Networks"`
			Mounts     string `json:"Mounts"`
			Labels     string `json:"Labels"`
			Size       string `json:"Size"`
		}
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}

		cleanName := strings.TrimPrefix(strings.TrimSpace(raw.Names), "/")
		var netList []string
		for _, n := range strings.Split(raw.Networks, ",") {
			if tn := strings.TrimSpace(n); tn != "" {
				netList = append(netList, tn)
			}
		}
		if netList == nil {
			netList = []string{}
		}

		mountsCount := 0
		if strings.TrimSpace(raw.Mounts) != "" {
			mountsCount = len(strings.Split(raw.Mounts, ","))
		}

		// Ekstrak label com.docker.compose.project jika kontainer merupakan bagian dari stack Compose
		composeProject := ""
		for _, kv := range strings.Split(raw.Labels, ",") {
			if strings.HasPrefix(kv, "com.docker.compose.project=") {
				composeProject = strings.TrimPrefix(kv, "com.docker.compose.project=")
				break
			}
		}

		shortID := raw.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}

		stat, hasStat := statMap[raw.ID]
		if !hasStat {
			stat, hasStat = statMap[shortID]
		}
		if !hasStat {
			stat = statMap[cleanName]
		}
		memStr := stat.MemUsageStr
		if memStr == "" {
			memStr = "-"
		}

		containers = append(containers, DockerContainer{
			ID:             raw.ID,
			Name:           cleanName,
			Names:          []string{cleanName},
			Image:          raw.Image,
			Command:        strings.Trim(raw.Command, `"`),
			State:          strings.ToLower(strings.TrimSpace(raw.State)),
			Status:         raw.Status,
			Created:        ParseDockerTimestamp(raw.CreatedAt),
			CreatedSince:   raw.RunningFor,
			Ports:          ParseDockerPorts(raw.Ports),
			Networks:       netList,
			MountsCount:    mountsCount,
			ComposeProject: composeProject,
			CPUPercent:     stat.CPUPercent,
			MemUsageBytes:  stat.MemUsageBytes,
			MemLimitBytes:  stat.MemLimitBytes,
			MemUsage:       memStr,
			NetIO:          stat.NetIO,
			BlockIO:        stat.BlockIO,
			SizeRW:         raw.Size,
		})
	}

	return containers, nil
}

// InspectContainer membaca detail lengkap kontainer dari `docker inspect` (Env, Mounts, Networks, Ports, Health)
func (m *Manager) InspectContainer(ctx context.Context, id string) (*DockerContainerInspect, error) {
	ref := strings.TrimSpace(id)
	if !IsValidDockerRef(ref) {
		return nil, fmt.Errorf("ID kontainer tidak valid")
	}
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", "inspect", ref).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gagal menginspeksi kontainer: %s", strings.TrimSpace(string(out)))
	}

	var list []struct {
		ID           string `json:"Id"`
		Name         string `json:"Name"`
		Created      string `json:"Created"`
		Image        string `json:"Image"`
		Platform     string `json:"Platform"`
		Driver       string `json:"Driver"`
		RestartCount int    `json:"RestartCount"`
		State        struct {
			Status     string `json:"Status"`
			Running    bool   `json:"Running"`
			Paused     bool   `json:"Paused"`
			Restarting bool   `json:"Restarting"`
			Pid        int    `json:"Pid"`
			StartedAt  string `json:"StartedAt"`
			Health     *struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
		HostConfig struct {
			RestartPolicy struct {
				Name string `json:"Name"`
			} `json:"RestartPolicy"`
		} `json:"HostConfig"`
		Config struct {
			Hostname   string            `json:"Hostname"`
			Image      string            `json:"Image"`
			WorkingDir string            `json:"WorkingDir"`
			Env        []string          `json:"Env"`
			Cmd        []string          `json:"Cmd"`
			Entrypoint []string          `json:"Entrypoint"`
			Labels     map[string]string `json:"Labels"`
		} `json:"Config"`
		Mounts []struct {
			Type        string `json:"Type"`
			Name        string `json:"Name"`
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
			Mode        string `json:"Mode"`
			RW          bool   `json:"RW"`
		} `json:"Mounts"`
		NetworkSettings struct {
			Ports map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
			Networks map[string]struct {
				NetworkID  string `json:"NetworkID"`
				IPAddress  string `json:"IPAddress"`
				Gateway    string `json:"Gateway"`
				MacAddress string `json:"MacAddress"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}

	if err := json.Unmarshal(out, &list); err != nil || len(list) == 0 {
		return nil, fmt.Errorf("gagal mengurai data docker inspect")
	}

	raw := list[0]
	envItems := make([]DockerEnvItem, 0, len(raw.Config.Env))
	for _, kv := range raw.Config.Env {
		parts := strings.SplitN(kv, "=", 2)
		k := parts[0]
		v := ""
		if len(parts) == 2 {
			v = parts[1]
		}
		envItems = append(envItems, DockerEnvItem{
			Key:    k,
			Value:  v,
			Secret: IsSecretEnvKey(k),
		})
	}

	mounts := make([]DockerMountItem, 0, len(raw.Mounts))
	for _, mt := range raw.Mounts {
		mounts = append(mounts, DockerMountItem{
			Type:        mt.Type,
			Name:        mt.Name,
			Source:      mt.Source,
			Destination: mt.Destination,
			Mode:        mt.Mode,
			RW:          mt.RW,
		})
	}

	nets := make([]DockerNetworkAttachment, 0, len(raw.NetworkSettings.Networks))
	for nName, nObj := range raw.NetworkSettings.Networks {
		nets = append(nets, DockerNetworkAttachment{
			Name:       nName,
			NetworkID:  nObj.NetworkID,
			IPAddress:  nObj.IPAddress,
			Gateway:    nObj.Gateway,
			MacAddress: nObj.MacAddress,
		})
	}
	sort.Slice(nets, func(i, j int) bool { return nets[i].Name < nets[j].Name })

	ports := make([]DockerContainerPort, 0)
	seenPorts := make(map[string]struct{})
	for cPortProto, bindings := range raw.NetworkSettings.Ports {
		parts := strings.SplitN(cPortProto, "/", 2)
		priv, _ := strconv.Atoi(parts[0])
		proto := "tcp"
		if len(parts) == 2 {
			proto = parts[1]
		}
		if len(bindings) == 0 {
			ports = append(ports, DockerContainerPort{PrivatePort: priv, Type: proto})
			continue
		}
		for _, b := range bindings {
			pub, _ := strconv.Atoi(b.HostPort)
			key := fmt.Sprintf("%d->%d/%s", pub, priv, proto)
			if _, ok := seenPorts[key]; ok {
				continue
			}
			seenPorts[key] = struct{}{}
			ports = append(ports, DockerContainerPort{
				IP:          b.HostIP,
				PublicPort:  pub,
				PrivatePort: priv,
				Type:        proto,
			})
		}
	}

	health := ""
	if raw.State.Health != nil {
		health = raw.State.Health.Status
	}

	labels := raw.Config.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	return &DockerContainerInspect{
		ID:             raw.ID,
		Name:           strings.TrimPrefix(raw.Name, "/"),
		Image:          raw.Config.Image,
		ImageID:        raw.Image,
		State:          raw.State.Status,
		Status:         raw.State.Status,
		Running:        raw.State.Running,
		Paused:         raw.State.Paused,
		Restarting:     raw.State.Restarting,
		PID:            raw.State.Pid,
		StartedAt:      raw.State.StartedAt,
		CreatedAt:      raw.Created,
		RestartPolicy:  raw.HostConfig.RestartPolicy.Name,
		RestartCount:   raw.RestartCount,
		Platform:       raw.Platform,
		Driver:         raw.Driver,
		Hostname:       raw.Config.Hostname,
		WorkingDir:     raw.Config.WorkingDir,
		Cmd:            raw.Config.Cmd,
		Entrypoint:     raw.Config.Entrypoint,
		Env:            envItems,
		Ports:          ports,
		Mounts:         mounts,
		Networks:       nets,
		Labels:         labels,
		ComposeProject: labels["com.docker.compose.project"],
		HealthStatus:   health,
	}, nil
}

// splitCommandArgs memecah string perintah kustom menjadi slice argumen dengan dukungan tanda kutip
func splitCommandArgs(cmd string) []string {
	var args []string
	var cur strings.Builder
	var quote rune
	escaped := false

	for _, r := range strings.TrimSpace(cmd) {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' {
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}

// CreateContainer membuat dan langsung menjalankan kontainer Docker baru
func (m *Manager) CreateContainer(ctx context.Context, req CreateContainerRequest) (string, error) {
	img := strings.TrimSpace(req.Image)
	if !IsValidDockerRef(img) {
		return "", fmt.Errorf("nama citra (image) tidak valid")
	}

	if req.PullFirst {
		pctx, pcancel := context.WithTimeout(ctx, 5*time.Minute)
		_ = exec.CommandContext(pctx, "docker", "pull", img).Run()
		pcancel()
	}

	args := []string{"run", "-d"}
	if name := strings.TrimSpace(req.Name); name != "" {
		if !IsValidDockerRef(name) {
			return "", fmt.Errorf("nama kontainer tidak valid")
		}
		args = append(args, "--name", name)
	}

	if req.AutoRemove {
		args = append(args, "--rm")
	} else {
		rp := strings.TrimSpace(req.RestartPolicy)
		if rp == "" {
			rp = "unless-stopped"
		}
		args = append(args, "--restart", rp)
	}

	if netName := strings.TrimSpace(req.Network); netName != "" && netName != "default" {
		if !IsValidDockerRef(netName) {
			return "", fmt.Errorf("nama jaringan tidak valid")
		}
		args = append(args, "--network", netName)
	}

	if req.MemoryLimitMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", req.MemoryLimitMB))
	}
	if req.CPULimit > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", req.CPULimit))
	}

	for _, p := range req.Ports {
		if tp := strings.TrimSpace(p); tp != "" {
			args = append(args, "-p", tp)
		}
	}
	for _, v := range req.Volumes {
		if tv := strings.TrimSpace(v); tv != "" {
			args = append(args, "-v", tv)
		}
	}

	// Gabungkan map `env` dan slice `environment` agar kompatibel penuh dengan semua payload klien
	for k, v := range req.Env {
		if strings.TrimSpace(k) != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", strings.TrimSpace(k), v))
		}
	}
	for _, kv := range req.Environment {
		if t := strings.TrimSpace(kv); t != "" && strings.Contains(t, "=") {
			args = append(args, "-e", t)
		}
	}

	args = append(args, img)
	if strings.TrimSpace(req.Command) != "" {
		args = append(args, splitCommandArgs(req.Command)...)
	}

	cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gagal membuat kontainer: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// RecreateContainer membuat ulang kontainer dengan konfigurasi persis sama (opsional tarik citra terbaru)
func (m *Manager) RecreateContainer(ctx context.Context, id string, pullLatest bool) (string, error) {
	insp, err := m.InspectContainer(ctx, id)
	if err != nil {
		return "", err
	}

	// Tarik citra terbaru terlebih dahulu sebelum menghentikan kontainer lama agar aman bila pull gagal
	if pullLatest {
		pctx, pcancel := context.WithTimeout(ctx, 5*time.Minute)
		out, pullErr := exec.CommandContext(pctx, "docker", "pull", insp.Image).CombinedOutput()
		pcancel()
		if pullErr != nil {
			return "", fmt.Errorf("gagal menarik citra terbaru %s: %s", insp.Image, strings.TrimSpace(string(out)))
		}
	}

	// Susun ulang parameter run dari hasil inspeksi
	req := CreateContainerRequest{
		Name:          insp.Name,
		Image:         insp.Image,
		RestartPolicy: insp.RestartPolicy,
		Ports:         make([]string, 0, len(insp.Ports)),
		Volumes:       make([]string, 0, len(insp.Mounts)),
		Environment:   make([]string, 0, len(insp.Env)),
	}
	for _, p := range insp.Ports {
		if p.PublicPort > 0 {
			bind := fmt.Sprintf("%d:%d/%s", p.PublicPort, p.PrivatePort, p.Type)
			if p.IP != "" && p.IP != "0.0.0.0" && p.IP != "::" {
				bind = fmt.Sprintf("%s:%s", p.IP, bind)
			}
			req.Ports = append(req.Ports, bind)
		}
	}
	for _, mt := range insp.Mounts {
		src := mt.Source
		if mt.Type == "volume" && mt.Name != "" {
			src = mt.Name
		}
		if src != "" && mt.Destination != "" {
			spec := fmt.Sprintf("%s:%s", src, mt.Destination)
			if !mt.RW {
				spec += ":ro"
			}
			req.Volumes = append(req.Volumes, spec)
		}
	}
	for _, e := range insp.Env {
		req.Environment = append(req.Environment, fmt.Sprintf("%s=%s", e.Key, e.Value))
	}
	if len(insp.Networks) > 0 {
		req.Network = insp.Networks[0].Name
	}

	// Hentikan & hapus kontainer lama lalu jalankan kontainer baru dengan nama identik
	if err := m.ControlContainer(ctx, insp.ID, "remove"); err != nil {
		return "", fmt.Errorf("gagal menghapus kontainer lama sebelum recreate: %w", err)
	}

	newID, err := m.CreateContainer(ctx, req)
	if err != nil {
		return "", err
	}

	// Sambungkan kembali jaringan tambahan bila kontainer sebelumnya tergabung di lebih dari 1 jaringan
	for i := 1; i < len(insp.Networks); i++ {
		_ = exec.CommandContext(ctx, "docker", "network", "connect", insp.Networks[i].Name, newID).Run()
	}
	return newID, nil
}

// ControlContainer mengendalikan status kontainer (start, stop, restart, pause, unpause, kill, remove)
func (m *Manager) ControlContainer(ctx context.Context, id, action string) error {
	ref := strings.TrimSpace(id)
	if !IsValidDockerRef(ref) {
		return fmt.Errorf("ID kontainer tidak valid")
	}

	var args []string
	switch action {
	case "start", "stop", "restart", "pause", "unpause", "kill":
		args = []string{action, ref}
	case "remove", "rm":
		args = []string{"rm", "-f", ref}
	default:
		return fmt.Errorf("aksi kontainer tidak valid: %s", action)
	}

	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menjalankan %s pada kontainer: %s", action, strings.TrimSpace(string(out)))
	}
	return nil
}

// BulkControlContainers menerapkan satu aksi ke banyak kontainer sekaligus
func (m *Manager) BulkControlContainers(ctx context.Context, req BulkContainerRequest) BulkContainerResult {
	res := BulkContainerResult{
		Succeeded: make([]string, 0, len(req.IDs)),
		Failed:    make(map[string]string),
	}
	for _, id := range req.IDs {
		cid := strings.TrimSpace(id)
		if cid == "" {
			continue
		}
		if err := m.ControlContainer(ctx, cid, req.Action); err != nil {
			res.Failed[cid] = err.Error()
		} else {
			res.Succeeded = append(res.Succeeded, cid)
		}
	}
	return res
}

// GetContainerLogs membaca log stdout/stderr kontainer dengan batas baris tertentu
func (m *Manager) GetContainerLogs(ctx context.Context, id string, lines int) (string, error) {
	return m.GetContainerLogsWithOptions(ctx, id, lines, false)
}

// GetContainerLogsWithOptions membaca log kontainer dengan dukungan opsi timestamp
func (m *Manager) GetContainerLogsWithOptions(ctx context.Context, id string, lines int, timestamps bool) (string, error) {
	ref := strings.TrimSpace(id)
	if !IsValidDockerRef(ref) {
		return "", fmt.Errorf("ID kontainer tidak valid")
	}
	if lines <= 0 {
		lines = 200
	}
	if lines > 5000 {
		lines = 5000
	}

	args := []string{"logs", "--tail", strconv.Itoa(lines)}
	if timestamps {
		args = append(args, "-t")
	}
	args = append(args, ref)

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", args...).CombinedOutput()
	if err != nil && len(out) == 0 {
		return "", fmt.Errorf("gagal mengambil log kontainer: %w", err)
	}
	return string(out), nil
}

// ListImages mengambil daftar citra lokal lengkap dengan ukuran akurat, waktu pembuatan, dan jumlah kontainer pemakai
func (m *Manager) ListImages(ctx context.Context) ([]DockerImage, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var (
		imgOut []byte
		imgErr error
		psOut  []byte
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		imgOut, imgErr = exec.CommandContext(cctx, "docker", "image", "ls", "--no-trunc", "--format", "{{json .}}").Output()
	}()
	go func() {
		defer wg.Done()
		psOut, _ = exec.CommandContext(cctx, "docker", "ps", "-a", "--no-trunc", "--format", "{{.Image}}").Output()
	}()
	wg.Wait()

	if imgErr != nil {
		return []DockerImage{}, imgErr
	}

	// Hitung pemakaian citra oleh seluruh kontainer (termasuk yang berhenti)
	usageByRef := make(map[string]int)
	for _, line := range strings.Split(strings.TrimSpace(string(psOut)), "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" {
			continue
		}
		usageByRef[ref]++
		if !strings.Contains(ref, ":") {
			usageByRef[ref+":latest"]++
		}
	}

	images := make([]DockerImage, 0)
	scanner := bufio.NewScanner(bytes.NewReader(imgOut))
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw struct {
			ID           string `json:"ID"`
			Repository   string `json:"Repository"`
			Tag          string `json:"Tag"`
			Digest       string `json:"Digest"`
			Size         string `json:"Size"`
			CreatedAt    string `json:"CreatedAt"`
			CreatedSince string `json:"CreatedSince"`
		}
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}

		szBytes := ParseHumanSize(raw.Size)
		fullRef := raw.Repository + ":" + raw.Tag
		usedCount := usageByRef[fullRef]
		if usedCount == 0 && raw.ID != "" {
			usedCount = usageByRef[raw.ID]
			if usedCount == 0 && strings.HasPrefix(raw.ID, "sha256:") && len(raw.ID) > 19 {
				usedCount = usageByRef[raw.ID[7:19]]
			}
		}

		dangling := raw.Repository == "<none>" && raw.Tag == "<none>"
		images = append(images, DockerImage{
			ID:           raw.ID,
			Repository:   raw.Repository,
			Tag:          raw.Tag,
			Digest:       raw.Digest,
			Size:         szBytes,
			SizeHuman:    FormatBytes(szBytes),
			Created:      ParseDockerTimestamp(raw.CreatedAt),
			CreatedSince: raw.CreatedSince,
			Containers:   usedCount,
			Dangling:     dangling,
		})
	}
	return images, nil
}

// PullImage menarik citra Docker secara sinkron (dipertahankan untuk panggilan API sederhana)
func (m *Manager) PullImage(ctx context.Context, imageName string) error {
	ref := strings.TrimSpace(imageName)
	if !IsValidDockerRef(ref) {
		return fmt.Errorf("nama citra tidak valid")
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", "pull", ref).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal pull citra %s: %s", ref, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveImage menghapus citra lokal berdasarkan ID atau nama:tag
func (m *Manager) RemoveImage(ctx context.Context, id string, force bool) error {
	ref := strings.TrimSpace(id)
	if !IsValidDockerRef(ref) {
		return fmt.Errorf("referensi citra tidak valid")
	}
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, ref)

	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menghapus citra: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ListVolumes mengambil daftar volume Docker lengkap dengan ukuran aktual dan ref count dari `docker system df -v`
func (m *Manager) ListVolumes(ctx context.Context) ([]DockerVolume, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var (
		lsOut []byte
		lsErr error
		dfOut []byte
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		lsOut, lsErr = exec.CommandContext(cctx, "docker", "volume", "ls", "--format", "{{json .}}").Output()
	}()
	go func() {
		defer wg.Done()
		dfOut, _ = exec.CommandContext(cctx, "docker", "system", "df", "-v", "--format", "{{json .}}").Output()
	}()
	wg.Wait()

	if lsErr != nil {
		return []DockerVolume{}, lsErr
	}

	// Kumpulkan statistik ukuran dan ref count per volume dari `docker system df -v`
	type volMetric struct {
		sizeBytes int64
		sizeHuman string
		links     int
	}
	metricsMap := make(map[string]volMetric)
	if len(dfOut) > 0 {
		var dfVerbose struct {
			Volumes []struct {
				Name  string `json:"Name"`
				Links string `json:"Links"`
				Size  string `json:"Size"`
			} `json:"Volumes"`
		}
		if json.Unmarshal(dfOut, &dfVerbose) == nil {
			for _, v := range dfVerbose.Volumes {
				sz := ParseHumanSize(v.Size)
				links, _ := strconv.Atoi(strings.TrimSpace(v.Links))
				metricsMap[v.Name] = volMetric{
					sizeBytes: sz,
					sizeHuman: FormatBytes(sz),
					links:     links,
				}
			}
		}
	}

	vols := make([]DockerVolume, 0)
	scanner := bufio.NewScanner(bytes.NewReader(lsOut))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw struct {
			Name       string `json:"Name"`
			Driver     string `json:"Driver"`
			Mountpoint string `json:"Mountpoint"`
			Scope      string `json:"Scope"`
		}
		if json.Unmarshal([]byte(line), &raw) != nil || raw.Name == "" {
			continue
		}
		mp := raw.Mountpoint
		if mp == "" {
			mp = "/var/lib/docker/volumes/" + raw.Name + "/_data"
		}
		met := metricsMap[raw.Name]
		szHuman := met.sizeHuman
		if szHuman == "" {
			szHuman = "0 B"
		}
		vols = append(vols, DockerVolume{
			Name:       raw.Name,
			Driver:     raw.Driver,
			Scope:      raw.Scope,
			Mountpoint: mp,
			SizeBytes:  met.sizeBytes,
			SizeHuman:  szHuman,
			RefCount:   met.links,
		})
	}
	return vols, nil
}

// CreateVolume membuat volume penyimpanan Docker baru
func (m *Manager) CreateVolume(ctx context.Context, name, driver string) error {
	clean := strings.TrimSpace(name)
	if !IsValidDockerRef(clean) {
		return fmt.Errorf("nama volume tidak valid")
	}
	args := []string{"volume", "create"}
	if d := strings.TrimSpace(driver); d != "" {
		if !IsValidDockerRef(d) {
			return fmt.Errorf("nama driver volume tidak valid")
		}
		args = append(args, "--driver", d)
	}
	args = append(args, clean)

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal membuat volume: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveVolume menghapus volume berdasarkan nama
func (m *Manager) RemoveVolume(ctx context.Context, name string) error {
	clean := strings.TrimSpace(name)
	if !IsValidDockerRef(clean) {
		return fmt.Errorf("nama volume tidak valid")
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", "volume", "rm", clean).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menghapus volume: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// PruneVolumes membersihkan seluruh volume yang tidak terpasang pada kontainer mana pun
func (m *Manager) PruneVolumes(ctx context.Context) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", "volume", "prune", "-f").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gagal membersihkan volume usang: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// ListNetworks mengambil daftar jaringan Docker lengkap dengan Subnet, Gateway, dan kontainer terhubung via `network inspect`
func (m *Manager) ListNetworks(ctx context.Context) ([]DockerNetwork, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	lsOut, err := exec.CommandContext(cctx, "docker", "network", "ls", "--no-trunc", "--format", "{{json .}}").Output()
	if err != nil {
		return []DockerNetwork{}, err
	}

	nets := make([]DockerNetwork, 0)
	ids := make([]string, 0)
	scanner := bufio.NewScanner(bytes.NewReader(lsOut))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw struct {
			ID     string `json:"ID"`
			Name   string `json:"Name"`
			Driver string `json:"Driver"`
			Scope  string `json:"Scope"`
		}
		if json.Unmarshal([]byte(line), &raw) != nil || raw.ID == "" {
			continue
		}
		nets = append(nets, DockerNetwork{
			ID:         raw.ID,
			Name:       raw.Name,
			Driver:     raw.Driver,
			Scope:      raw.Scope,
			Containers: []string{},
		})
		ids = append(ids, raw.ID)
	}

	// Perkaya seluruh jaringan dalam satu batch pemanggilan `docker network inspect`
	if len(ids) > 0 {
		inspectArgs := append([]string{"network", "inspect"}, ids...)
		if inspOut, err := exec.CommandContext(cctx, "docker", inspectArgs...).Output(); err == nil {
			var details []struct {
				ID         string `json:"Id"`
				Internal   bool   `json:"Internal"`
				Attachable bool   `json:"Attachable"`
				IPAM       struct {
					Config []struct {
						Subnet  string `json:"Subnet"`
						Gateway string `json:"Gateway"`
					} `json:"Config"`
				} `json:"IPAM"`
				Containers map[string]struct {
					Name string `json:"Name"`
				} `json:"Containers"`
			}
			if json.Unmarshal(inspOut, &details) == nil {
				byID := make(map[string]int, len(nets))
				for i, n := range nets {
					byID[n.ID] = i
				}
				for _, d := range details {
					idx, ok := byID[d.ID]
					if !ok {
						continue
					}
					nets[idx].Internal = d.Internal
					nets[idx].Attachable = d.Attachable
					if len(d.IPAM.Config) > 0 {
						nets[idx].Subnet = d.IPAM.Config[0].Subnet
						nets[idx].Gateway = d.IPAM.Config[0].Gateway
					}
					cNames := make([]string, 0, len(d.Containers))
					for _, c := range d.Containers {
						if c.Name != "" {
							cNames = append(cNames, c.Name)
						}
					}
					sort.Strings(cNames)
					nets[idx].Containers = cNames
					nets[idx].ContainersCount = len(cNames)
				}
			}
		}
	}

	return nets, nil
}

// CreateNetwork membuat jaringan virtual Docker baru
func (m *Manager) CreateNetwork(ctx context.Context, name, driver, subnet, gateway string) error {
	clean := strings.TrimSpace(name)
	if !IsValidDockerRef(clean) {
		return fmt.Errorf("nama jaringan tidak valid")
	}
	args := []string{"network", "create"}
	if d := strings.TrimSpace(driver); d != "" {
		args = append(args, "--driver", d)
	}
	if s := strings.TrimSpace(subnet); s != "" {
		args = append(args, "--subnet", s)
	}
	if g := strings.TrimSpace(gateway); g != "" {
		args = append(args, "--gateway", g)
	}
	args = append(args, clean)

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal membuat jaringan: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveNetwork menghapus jaringan berdasarkan ID atau nama
func (m *Manager) RemoveNetwork(ctx context.Context, id string) error {
	clean := strings.TrimSpace(id)
	if !IsValidDockerRef(clean) {
		return fmt.Errorf("ID jaringan tidak valid")
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", "network", "rm", clean).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menghapus jaringan: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// PruneNetworks membersihkan jaringan Docker yang tidak digunakan kontainer mana pun
func (m *Manager) PruneNetworks(ctx context.Context) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", "network", "prune", "-f").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gagal membersihkan jaringan usang: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// ListComposeProjects menemukan seluruh stack Compose di host (`docker compose ls --all`) digabung dengan proyek di oneClickDir
func (m *Manager) ListComposeProjects(ctx context.Context) ([]DockerComposeProject, error) {
	m.mu.RLock()
	baseDir := m.oneClickDir
	m.mu.RUnlock()

	projectsByName := make(map[string]DockerComposeProject)

	// 1. Auto-discover seluruh stack Compose aktif maupun berhenti di seluruh host
	cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	lsOut, err := exec.CommandContext(cctx, "docker", "compose", "ls", "--all", "--format", "json").Output()
	cancel()
	if err == nil && len(bytes.TrimSpace(lsOut)) > 0 {
		var discovered []struct {
			Name        string `json:"Name"`
			Status      string `json:"Status"`
			ConfigFiles string `json:"ConfigFiles"`
		}
		if json.Unmarshal(lsOut, &discovered) == nil {
			for _, d := range discovered {
				name := strings.TrimSpace(d.Name)
				if name == "" {
					continue
				}
				// ConfigFiles bisa berisi beberapa file dipisah koma; ambil berkas pertama
				cfgPath := strings.TrimSpace(strings.Split(d.ConfigFiles, ",")[0])
				workDir := ""
				yamlStr := ""
				if cfgPath != "" {
					workDir = filepath.Dir(cfgPath)
					if raw, rerr := os.ReadFile(cfgPath); rerr == nil && len(raw) <= 512*1024 {
						yamlStr = string(raw)
					}
				}
				status, total, running := ParseComposeStatusSummary(d.Status)
				managed := strings.HasPrefix(filepath.Clean(cfgPath), filepath.Clean(baseDir))
				projectsByName[name] = DockerComposeProject{
					Name:            name,
					Status:          status,
					RawStatus:       d.Status,
					ConfigPath:      cfgPath,
					WorkingDir:      workDir,
					ContainersCount: total,
					RunningCount:    running,
					YAMLContent:     yamlStr,
					Managed:         managed,
					Services:        []string{},
				}
			}
		}
	}

	// 2. Gabungkan dengan proyek YAML yang tersimpan di direktori oneClickDir (termasuk yang belum pernah dijalankan)
	if entries, err := os.ReadDir(baseDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			projDir := filepath.Join(baseDir, name)
			ymlPath := filepath.Join(projDir, "docker-compose.yml")
			if _, statErr := os.Stat(ymlPath); statErr != nil {
				alt := filepath.Join(projDir, "compose.yaml")
				if _, altErr := os.Stat(alt); altErr == nil {
					ymlPath = alt
				} else {
					continue
				}
			}

			raw, _ := os.ReadFile(ymlPath)
			if existing, ok := projectsByName[name]; ok {
				existing.Managed = true
				if existing.ConfigPath == "" {
					existing.ConfigPath = ymlPath
					existing.WorkingDir = projDir
				}
				if existing.YAMLContent == "" {
					existing.YAMLContent = string(raw)
				}
				projectsByName[name] = existing
			} else {
				projectsByName[name] = DockerComposeProject{
					Name:            name,
					Status:          "stopped",
					RawStatus:       "stopped",
					ConfigPath:      ymlPath,
					WorkingDir:      projDir,
					ContainersCount: 0,
					RunningCount:    0,
					YAMLContent:     string(raw),
					Managed:         true,
					Services:        []string{},
				}
			}
		}
	}

	result := make([]DockerComposeProject, 0, len(projectsByName))
	for _, p := range projectsByName {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Status != result[j].Status {
			return result[i].Status == "running"
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// ComposeUp menyimpan berkas docker-compose.yml dan menjalankan stack (`compose up -d`)
func (m *Manager) ComposeUp(ctx context.Context, projectName, yamlContent string) error {
	name := strings.TrimSpace(projectName)
	if !IsValidProjectName(name) {
		return fmt.Errorf("nama proyek Compose tidak valid")
	}
	if strings.TrimSpace(yamlContent) == "" {
		return fmt.Errorf("isi konfigurasi YAML Compose tidak boleh kosong")
	}

	m.mu.RLock()
	baseDir := m.oneClickDir
	m.mu.RUnlock()

	projDir := filepath.Join(baseDir, name)
	if err := os.MkdirAll(projDir, 0755); err != nil {
		return fmt.Errorf("gagal membuat direktori proyek Compose: %w", err)
	}

	ymlPath := filepath.Join(projDir, "docker-compose.yml")
	if err := os.WriteFile(ymlPath, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("gagal menulis docker-compose.yml: %w", err)
	}

	return m.ComposeAction(ctx, ComposeActionRequest{
		Name:       name,
		ConfigPath: ymlPath,
		Action:     "up",
	})
}

// ComposeDown menghentikan dan menghapus seluruh kontainer milik stack Compose
func (m *Manager) ComposeDown(ctx context.Context, projectName string) error {
	return m.ComposeAction(ctx, ComposeActionRequest{
		Name:   projectName,
		Action: "down",
	})
}

// ComposeAction mengeksekusi aksi siklus hidup stack Docker Compose (up, down, restart, stop, start, pull)
func (m *Manager) ComposeAction(ctx context.Context, req ComposeActionRequest) error {
	name := strings.TrimSpace(req.Name)
	if !IsValidProjectName(name) {
		return fmt.Errorf("nama proyek Compose tidak valid")
	}

	m.mu.RLock()
	baseDir := m.oneClickDir
	m.mu.RUnlock()

	cfgPath := strings.TrimSpace(req.ConfigPath)
	if cfgPath == "" {
		candidate := filepath.Join(baseDir, name, "docker-compose.yml")
		if _, err := os.Stat(candidate); err == nil {
			cfgPath = candidate
		}
	}

	bin, baseArgs := m.composeBaseArgs(ctx)
	runCompose := func(timeout time.Duration, extraArgs ...string) error {
		cctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		args := append([]string{}, baseArgs...)
		args = append(args, "-p", name)
		if cfgPath != "" {
			args = append(args, "-f", cfgPath)
		}
		args = append(args, extraArgs...)

		cmd := exec.CommandContext(cctx, bin, args...)
		if cfgPath != "" {
			cmd.Dir = filepath.Dir(cfgPath)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("compose %s gagal: %s", strings.Join(extraArgs, " "), strings.TrimSpace(string(out)))
		}
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(req.Action)) {
	case "up":
		return runCompose(5*time.Minute, "up", "-d", "--remove-orphans")
	case "down":
		return runCompose(2*time.Minute, "down")
	case "stop":
		return runCompose(1*time.Minute, "stop")
	case "start":
		return runCompose(1*time.Minute, "start")
	case "restart":
		return runCompose(2*time.Minute, "restart")
	case "pull":
		if err := runCompose(8*time.Minute, "pull"); err != nil {
			return err
		}
		return runCompose(5*time.Minute, "up", "-d", "--remove-orphans")
	default:
		return fmt.Errorf("aksi Compose tidak valid: %s", req.Action)
	}
}

// ComposeLogs mengambil gabungan tail log seluruh layanan dalam satu stack Compose
func (m *Manager) ComposeLogs(ctx context.Context, name, configPath string, lines int) (string, error) {
	cleanName := strings.TrimSpace(name)
	if !IsValidProjectName(cleanName) {
		return "", fmt.Errorf("nama proyek Compose tidak valid")
	}
	if lines <= 0 {
		lines = 200
	}
	if lines > 2000 {
		lines = 2000
	}

	bin, baseArgs := m.composeBaseArgs(ctx)
	args := append([]string{}, baseArgs...)
	args = append(args, "-p", cleanName)
	if strings.TrimSpace(configPath) != "" {
		args = append(args, "-f", strings.TrimSpace(configPath))
	}
	args = append(args, "logs", "--no-color", "--tail", strconv.Itoa(lines))

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, bin, args...)
	if strings.TrimSpace(configPath) != "" {
		cmd.Dir = filepath.Dir(strings.TrimSpace(configPath))
	}
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return "", fmt.Errorf("gagal membaca log stack Compose: %w", err)
	}
	return string(out), nil
}

// ListRegistries mengembalikan daftar registri Docker tersimpan
func (m *Manager) ListRegistries() []DockerRegistry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]DockerRegistry, len(m.registries))
	copy(out, m.registries)
	return out
}

// AddRegistry mendaftarkan registri privat baru dan langsung mengeksekusi `docker login` bila kredensial diisi
func (m *Manager) AddRegistry(ctx context.Context, name, regURL, username, password string) error {
	cleanName := strings.TrimSpace(name)
	cleanURL := strings.TrimSpace(regURL)
	cleanUser := strings.TrimSpace(username)
	if cleanName == "" || cleanURL == "" {
		return fmt.Errorf("nama dan URL registri wajib diisi")
	}

	loggedIn := false
	if cleanUser != "" && password != "" {
		cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		cmd := exec.CommandContext(cctx, "docker", "login", cleanURL, "-u", cleanUser, "--password-stdin")
		cmd.Stdin = strings.NewReader(password)
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			return fmt.Errorf("docker login ke %s gagal: %s", cleanURL, strings.TrimSpace(string(out)))
		}
		loggedIn = true
	}

	m.mu.Lock()
	id := fmt.Sprintf("reg_%d", time.Now().UnixNano())
	m.registries = append(m.registries, DockerRegistry{
		ID:        id,
		Name:      cleanName,
		URL:       cleanURL,
		Username:  cleanUser,
		IsDefault: false,
		LoggedIn:  loggedIn,
	})
	snapshot := make([]DockerRegistry, len(m.registries))
	copy(snapshot, m.registries)
	saveFn := m.saveRegsFunc
	m.mu.Unlock()

	if saveFn != nil {
		if err := saveFn(snapshot); err != nil {
			return fmt.Errorf("registri terhubung namun gagal disimpan persisten: %w", err)
		}
	}
	return nil
}

// RemoveRegistry menghapus registri dan menjalankan `docker logout` terhadap host registri terkait
func (m *Manager) RemoveRegistry(ctx context.Context, id string) error {
	m.mu.Lock()
	var targetURL string
	updated := make([]DockerRegistry, 0, len(m.registries))
	for _, r := range m.registries {
		if r.ID == id && !r.IsDefault {
			targetURL = r.URL
			continue
		}
		updated = append(updated, r)
	}
	m.registries = updated
	snapshot := make([]DockerRegistry, len(m.registries))
	copy(snapshot, m.registries)
	saveFn := m.saveRegsFunc
	m.mu.Unlock()

	if targetURL != "" {
		cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		_ = exec.CommandContext(cctx, "docker", "logout", targetURL).Run()
		cancel()
	}

	if saveFn != nil {
		_ = saveFn(snapshot)
	}
	return nil
}
