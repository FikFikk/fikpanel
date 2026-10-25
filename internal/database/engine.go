package database

import (
	"context"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

// EngineInfo merepresentasikan informasi lengkap tentang mesin database
type EngineInfo struct {
	Name              string   `json:"name"`               // 'mysql', 'postgresql', 'mongodb', 'redis'
	DisplayName       string   `json:"display_name"`
	Installed         bool     `json:"installed"`
	Version           string   `json:"version"`
	Running           bool     `json:"running"`
	Port              int      `json:"port"`
	DatabasesCount    int      `json:"databases_count"`
	ServiceUnit       string   `json:"service_unit"`
	SupportedVersions []string `json:"supported_versions"` // Opsi versi untuk Quick Install
	Description       string   `json:"description"`
}

// EngineState menyimpan override dinamis (misal setelah Quick Install selesai)
type EngineState struct {
	mu        sync.RWMutex
	installed map[string]bool
	versions  map[string]string
	running   map[string]bool
}

// Global state untuk melacak perubahan runtime instalasi mesin database
var globalState = &EngineState{
	installed: make(map[string]bool),
	versions:  make(map[string]string),
	running:   make(map[string]bool),
}

// SetEngineInstalled memperbarui status instalasi engine di runtime
func SetEngineInstalled(engine, version string, running bool) {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.installed[engine] = true
	globalState.versions[engine] = version
	globalState.running[engine] = running
}

// DetectEngines mendeteksi status ketersediaan seluruh engine database
func DetectEngines(ctx context.Context, dbsCountMap map[string]int) []EngineInfo {
	globalState.mu.RLock()
	defer globalState.mu.RUnlock()

	engines := []EngineInfo{
		{
			Name:              "mysql",
			DisplayName:       "MySQL / MariaDB",
			Port:              3306,
			ServiceUnit:       "mariadb.service",
			SupportedVersions: []string{"10.11", "11.4", "8.0"},
			Description:       "Relational database standar untuk aplikasi web PHP, WordPress, dan CMS.",
		},
		{
			Name:              "postgresql",
			DisplayName:       "PostgreSQL",
			Port:              5432,
			ServiceUnit:       "postgresql.service",
			SupportedVersions: []string{"16", "15", "14"},
			Description:       "Database relasional canggih berorientasi objek dengan performa query analitik tinggi.",
		},
		{
			Name:              "mongodb",
			DisplayName:       "MongoDB",
			Port:              27017,
			ServiceUnit:       "mongod.service",
			SupportedVersions: []string{"6.0", "7.0", "8.0", "8.2", "8.3"},
			Description:       "Database dokumen NoSQL terdistribusi untuk struktur data JSON / BSON fleksibel.",
		},
		{
			Name:              "redis",
			DisplayName:       "Redis",
			Port:              6379,
			ServiceUnit:       "redis-server.service",
			SupportedVersions: []string{"7.0", "7.2"},
			Description:       "In-memory data structure store untuk caching berkecepatan ultra-tinggi dan sesi.",
		},
	}

	for i := range engines {
		eng := &engines[i]
		if count, ok := dbsCountMap[eng.Name]; ok {
			eng.DatabasesCount = count
		}

		// Cek apakah ada override dari instalasi sebelumnya
		if inst, ok := globalState.installed[eng.Name]; ok {
			eng.Installed = inst
			eng.Version = globalState.versions[eng.Name]
			eng.Running = globalState.running[eng.Name]
			continue
		}

		if runtime.GOOS != "linux" {
			// Mode simulasi lokal (Windows / macOS)
			// Sesuai tangkapan layar aaPanel: MySQL, Postgres, Redis aktif; MongoDB belum terpasang
			switch eng.Name {
			case "mysql":
				eng.Installed = true
				eng.Version = "10.11.6-MariaDB"
				eng.Running = true
			case "postgresql":
				eng.Installed = true
				eng.Version = "16.2"
				eng.Running = true
			case "redis":
				eng.Installed = true
				eng.Version = "7.0.15"
				eng.Running = true
			case "mongodb":
				// Belum terpasang secara default untuk memicu Quick Install edukatif
				eng.Installed = false
				eng.Version = ""
				eng.Running = false
			}
			continue
		}

		// Deteksi real di sistem Linux
		detectLinuxEngine(ctx, eng)
	}

	return engines
}

// detectLinuxEngine memeriksa keberadaan binary dan service systemd di Linux
func detectLinuxEngine(ctx context.Context, eng *EngineInfo) {
	var binaries []string
	var serviceUnits []string

	switch eng.Name {
	case "mysql":
		binaries = []string{"mariadb", "mysqld", "mysql"}
		serviceUnits = []string{"mariadb", "mysql"}
	case "postgresql":
		binaries = []string{"psql", "postgres"}
		serviceUnits = []string{"postgresql"}
	case "mongodb":
		binaries = []string{"mongod", "mongosh"}
		serviceUnits = []string{"mongod", "mongodb"}
	case "redis":
		binaries = []string{"redis-server", "redis-cli"}
		serviceUnits = []string{"redis-server", "redis"}
	}

	// 1. Cek binary
	foundBin := ""
	for _, b := range binaries {
		if path, err := exec.LookPath(b); err == nil {
			foundBin = path
			break
		}
	}

	if foundBin == "" {
		eng.Installed = false
		eng.Running = false
		return
	}
	eng.Installed = true

	// 2. Ambil versi binary
	out, err := exec.CommandContext(ctx, foundBin, "--version").CombinedOutput()
	if err == nil {
		re := regexp.MustCompile(`([0-9]+\.[0-9]+(\.[0-9]+)?)`)
		if m := re.FindStringSubmatch(string(out)); len(m) > 1 {
			eng.Version = m[1]
		}
	}

	// 3. Cek status systemd
	for _, s := range serviceUnits {
		sysOut, err := exec.CommandContext(ctx, "systemctl", "is-active", s).Output()
		if err == nil && strings.TrimSpace(string(sysOut)) == "active" {
			eng.Running = true
			eng.ServiceUnit = s + ".service"
			break
		}
	}
}
