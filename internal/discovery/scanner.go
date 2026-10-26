package discovery

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type NginxVhostItem struct {
	ConfigFile string `json:"config_file"`
	ServerName string `json:"server_name"`
	RootDir    string `json:"root_dir"`
	Port       int    `json:"port"`
	SSL        bool   `json:"ssl"`
	ProxyPass  string `json:"proxy_pass"`
	IsAdopted  bool   `json:"is_adopted"`
}

type DockerDiscoveredItem struct {
	ProjectName string   `json:"project_name"`
	ComposePath string   `json:"compose_path"`
	WorkingDir  string   `json:"working_dir"`
	Containers  []string `json:"containers"`
	Running     bool     `json:"running"`
}

type PM2DiscoveredItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Script     string `json:"script"`
	WorkingDir string `json:"working_dir"`
	Status     string `json:"status"`
	Instances  int    `json:"instances"`
}

type DatabaseDiscoveredItem struct {
	Engine    string   `json:"engine"` // mysql, postgresql, mongodb, redis
	Port      int      `json:"port"`
	Running   bool     `json:"running"`
	Databases []string `json:"databases"`
}

type DiscoveryResult struct {
	NginxVhosts        []NginxVhostItem         `json:"nginx_vhosts"`
	DockerProjects     []DockerDiscoveredItem   `json:"docker_projects"`
	PM2Apps            []PM2DiscoveredItem      `json:"pm2_apps"`
	Databases          []DatabaseDiscoveredItem `json:"databases"`
	KubernetesDetected bool                     `json:"kubernetes_detected"`
	KubernetesType     string                   `json:"kubernetes_type"` // k3s, microk8s, standard, none
}

type Scanner struct{}

func NewScanner() *Scanner {
	return &Scanner{}
}

// parseConfigFile membaca konfigurasi vhost Nginx dan mengekstrak blok server
func parseConfigFile(path string) []NginxVhostItem {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	// Bersihkan baris komentar (#) agar tidak memproses blok atau directive yang dinonaktifkan
	lines := strings.Split(string(data), "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Hapus inline comment jika ada: directive # komentar
		if idx := strings.Index(line, "#"); idx != -1 {
			line = line[:idx]
		}
		cleanLines = append(cleanLines, line)
	}
	content := strings.Join(cleanLines, "\n")

	// Regex pencocokan blok server
	serverBlockRe := regexp.MustCompile(`(?s)server\s*\{([^{}]*(?:\{[^{}]*\}[^{}]*)*)\}`)
	matches := serverBlockRe.FindAllStringSubmatch(content, -1)

	var items []NginxVhostItem
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		block := m[1]

		item := NginxVhostItem{
			ConfigFile: path,
			Port:       80,
		}

		// Cari listen port & ssl
		listenRe := regexp.MustCompile(`listen\s+([0-9]+)(\s+ssl)?`)
		if lMatch := listenRe.FindStringSubmatch(block); len(lMatch) > 1 {
			if p, err := strconv.Atoi(lMatch[1]); err == nil {
				item.Port = p
			}
			if len(lMatch) > 2 && strings.Contains(lMatch[2], "ssl") {
				item.SSL = true
			}
		}
		if strings.Contains(block, "ssl_certificate") {
			item.SSL = true
		}

		// Cari server_name
		nameRe := regexp.MustCompile(`server_name\s+([^;]+);`)
		if nMatch := nameRe.FindStringSubmatch(block); len(nMatch) > 1 {
			names := strings.Fields(nMatch[1])
			if len(names) > 0 {
				item.ServerName = names[0]
			}
		}

		// Cari root directory
		rootRe := regexp.MustCompile(`root\s+([^;]+);`)
		if rMatch := rootRe.FindStringSubmatch(block); len(rMatch) > 1 {
			item.RootDir = strings.TrimSpace(rMatch[1])
		}

		// Cari proxy_pass
		proxyRe := regexp.MustCompile(`proxy_pass\s+([^;]+);`)
		if pMatch := proxyRe.FindStringSubmatch(block); len(pMatch) > 1 {
			item.ProxyPass = strings.TrimSpace(pMatch[1])
		}

		// Hanya sertakan domain aktif riil (bukan placeholder, wildcard default, atau localhost)
		if item.ServerName != "" && item.ServerName != "_" && item.ServerName != "localhost" && item.ServerName != "example.com" {
			items = append(items, item)
		}
	}

	return items
}

// ScanNginxVhosts memindai vhost Nginx yang sudah aktif di direktori Linux standar
func (s *Scanner) ScanNginxVhosts(ctx context.Context) ([]NginxVhostItem, error) {
	var results []NginxVhostItem
	// Hanya periksa konfigurasi yang benar-benar aktif (sites-enabled dan conf.d)
	dirs := []string{
		"/etc/nginx/sites-enabled",
		"/etc/nginx/conf.d",
	}

	seen := make(map[string]bool)
	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() || strings.HasPrefix(f.Name(), ".") {
				continue
			}
			filePath := filepath.Join(dir, f.Name())
			vhosts := parseConfigFile(filePath)
			for _, v := range vhosts {
				key := v.ServerName + ":" + strconv.Itoa(v.Port)
				if !seen[key] {
					seen[key] = true
					results = append(results, v)
				}
			}
		}
	}

	return results, nil
}

// scanComposeInPaths menelusuri direktori target untuk menemukan berkas docker-compose
func scanComposeInPaths(ctx context.Context, roots []string) []DockerDiscoveredItem {
	var projects []DockerDiscoveredItem
	seenDirs := make(map[string]bool)

	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			// Periksa pembatalan context
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Batasi kedalaman pencarian maksimal 3 subdirektori
			rel, errRel := filepath.Rel(root, path)
			if errRel == nil && strings.Count(rel, string(filepath.Separator)) > 3 {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if info.IsDir() {
				name := info.Name()
				if name == "node_modules" || name == ".git" || name == "vendor" || name == "storage" ||
					name == "AppData" || name == "Local" || name == "Windows" || name == "Program Files" ||
					(strings.HasPrefix(name, ".") && name != "." && name != "..") {
					return filepath.SkipDir
				}
				return nil
			}

			fileName := strings.ToLower(info.Name())
			if fileName == "docker-compose.yml" || fileName == "docker-compose.yaml" || fileName == "compose.yml" || fileName == "compose.yaml" {
				dir := filepath.Dir(path)
				if !seenDirs[dir] {
					seenDirs[dir] = true
					projectName := filepath.Base(dir)
					projects = append(projects, DockerDiscoveredItem{
						ProjectName: projectName,
						ComposePath: path,
						WorkingDir:  dir,
					})
				}
			}
			return nil
		})
	}

	return projects
}

// ScanDockerProjects memindai direktori proyek compose dan kontainer berjalan
func (s *Scanner) ScanDockerProjects(ctx context.Context, customRoots []string) ([]DockerDiscoveredItem, error) {
	defaultRoots := []string{"/var/www", "/opt"}
	if home, err := os.UserHomeDir(); err == nil {
		// Pindai subdirektori proyek umum di home direktori pengguna saat ini
		for _, sub := range []string{"projects", "workspace", "www", "web", "node", "dev"} {
			subPath := filepath.Join(home, sub)
			if _, err := os.Stat(subPath); err == nil {
				defaultRoots = append(defaultRoots, subPath)
			}
		}
	}

	// Jika berjalan sebagai root di Linux, pindai subdirektori proyek di seluruh akun /home/<user>
	if entries, err := os.ReadDir("/home"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				userHome := filepath.Join("/home", e.Name())
				for _, sub := range []string{"projects", "workspace", "www", "web", "node", "dev"} {
					subPath := filepath.Join(userHome, sub)
					if _, err := os.Stat(subPath); err == nil {
						defaultRoots = append(defaultRoots, subPath)
					}
				}
			}
		}
	}

	allRoots := append(defaultRoots, customRoots...)

	projects := scanComposeInPaths(ctx, allRoots)

	// Periksa kontainer docker aktif jika docker daemon tersedia (timeout 2 detik jika daemon offline)
	if _, err := exec.LookPath("docker"); err == nil {
		dockerCtx, cancelDocker := context.WithTimeout(ctx, 2*time.Second)
		defer cancelDocker()
		cmd := exec.CommandContext(dockerCtx, "docker", "ps", "--format", "{{.Names}}\t{{.Labels}}")
		if out, err := cmd.Output(); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.Split(line, "\t")
				if len(parts) >= 2 {
					cName := parts[0]
					labels := parts[1]
					for i := range projects {
						if strings.Contains(labels, projects[i].ProjectName) {
							projects[i].Containers = append(projects[i].Containers, cName)
							projects[i].Running = true
						}
					}
				}
			}
		}
	}

	return projects, nil
}

// ScanPM2Apps memindai proses PM2 yang sedang berjalan di host
func (s *Scanner) ScanPM2Apps(ctx context.Context) ([]PM2DiscoveredItem, error) {
	pm2Bin, err := exec.LookPath("pm2")
	if err != nil {
		if runtime.GOOS == "windows" {
			pm2Bin, err = exec.LookPath("pm2.cmd")
		}
	}
	if err != nil {
		return []PM2DiscoveredItem{}, nil
	}

	cmd := exec.CommandContext(ctx, pm2Bin, "jlist")
	out, err := cmd.Output()
	if err != nil {
		return []PM2DiscoveredItem{}, nil
	}

	outStr := string(out)
	firstBracket := strings.Index(outStr, "[")
	lastBracket := strings.LastIndex(outStr, "]")
	if firstBracket == -1 || lastBracket == -1 || lastBracket < firstBracket {
		return []PM2DiscoveredItem{}, nil
	}

	var rawList []map[string]interface{}
	if err := json.Unmarshal([]byte(outStr[firstBracket:lastBracket+1]), &rawList); err != nil {
		return []PM2DiscoveredItem{}, nil
	}

	var apps []PM2DiscoveredItem
	for _, item := range rawList {
		app := PM2DiscoveredItem{Instances: 1}
		if pmID, ok := item["pm_id"].(float64); ok {
			app.ID = int(pmID)
		}
		if name, ok := item["name"].(string); ok {
			app.Name = name
		}
		if pm2Env, ok := item["pm2_env"].(map[string]interface{}); ok {
			if st, ok := pm2Env["status"].(string); ok {
				app.Status = st
			}
			if sc, ok := pm2Env["pm_exec_path"].(string); ok {
				app.Script = sc
			}
			if cwd, ok := pm2Env["pm_cwd"].(string); ok {
				app.WorkingDir = cwd
			}
		}
		apps = append(apps, app)
	}

	return apps, nil
}

// DetectKubernetes memeriksa apakah cluster Kubernetes / K3s aktif di server
func DetectKubernetes(ctx context.Context) (bool, string) {
	// Cek K3s bawaan
	if _, err := os.Stat("/etc/rancher/k3s/k3s.yaml"); err == nil {
		return true, "k3s"
	}

	// Cek MicroK8s
	if _, err := exec.LookPath("microk8s"); err == nil {
		return true, "microk8s"
	}

	// Cek kubectl umum
	if _, err := exec.LookPath("kubectl"); err == nil {
		cmd := exec.CommandContext(ctx, "kubectl", "version", "--client", "--output=json")
		if err := cmd.Run(); err == nil {
			return true, "standard"
		}
	}

	return false, "none"
}

// ScanAll menjalankan seluruh modul deteksi secara komprehensif
func (s *Scanner) ScanAll(ctx context.Context, customRoots []string) (*DiscoveryResult, error) {
	res := &DiscoveryResult{}

	vhosts, _ := s.ScanNginxVhosts(ctx)
	res.NginxVhosts = vhosts

	dockerProjs, _ := s.ScanDockerProjects(ctx, customRoots)
	res.DockerProjects = dockerProjs

	pm2Apps, _ := s.ScanPM2Apps(ctx)
	res.PM2Apps = pm2Apps

	k8sDetected, k8sType := DetectKubernetes(ctx)
	res.KubernetesDetected = k8sDetected
	res.KubernetesType = k8sType

	return res, nil
}
