package discovery

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
	User       string `json:"user"`
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

// skippedComposeDirs daftar direktori besar yang wajib dilewati saat memindai berkas compose
var skippedComposeDirs = map[string]bool{
	"actions-runner":  true,
	".actions-runner": true,
	"cache":           true,
	".cache":          true,
	"logs":            true,
	"tmp":             true,
	"temp":            true,
	"vendor":          true,
	"node_modules":    true,
	".output":         true,
	"dist":            true,
	"build":           true,
	".git":            true,
	"storage":         true,
	"AppData":         true,
	"Local":           true,
	"Windows":         true,
	"Program Files":   true,
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
	results := []NginxVhostItem{}
	// Hanya periksa konfigurasi yang benar-benar aktif (sites-enabled dan conf.d)
	dirs := []string{
		"/etc/nginx/sites-enabled",
		"/etc/nginx/conf.d",
	}

	seen := make(map[string]bool)
	for _, dir := range dirs {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

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
	projects := []DockerDiscoveredItem{}
	seenDirs := make(map[string]bool)

	for _, root := range roots {
		select {
		case <-ctx.Done():
			return projects
		default:
		}

		if _, err := os.Stat(root); err != nil {
			continue
		}
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			// Periksa pembatalan context agar tidak hang
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Batasi kedalaman pencarian maksimal 2 subdirektori
			rel, errRel := filepath.Rel(root, path)
			if errRel == nil && strings.Count(rel, string(filepath.Separator)) > 2 {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if info.IsDir() {
				name := info.Name()
				if skippedComposeDirs[name] || skippedComposeDirs[strings.ToLower(name)] ||
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
						Containers:  []string{},
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
	defaultRoots := []string{"/var/www", "/opt", "/www/dk_project"}
	if home, err := os.UserHomeDir(); err == nil {
		// Pindai subdirektori proyek umum di home direktori pengguna saat ini
		for _, sub := range []string{"projects", "workspace", "www", "web", "node", "dev", "docker"} {
			subPath := filepath.Join(home, sub)
			if _, err := os.Stat(subPath); err == nil {
				defaultRoots = append(defaultRoots, subPath)
			}
		}
	}

	// Jika berjalan di Linux, pindai direktori akun /home/<user> (terproteksi oleh batas depth <= 2 dan blacklist)
	if entries, err := os.ReadDir("/home"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				userHome := filepath.Join("/home", e.Name())
				defaultRoots = append(defaultRoots, userHome)
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

// resolvePM2BinForUser mencari biner pm2 atau node untuk user tertentu (termasuk NVM)
func resolvePM2BinForUser(name string, username string) string {
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath(name + ".cmd"); err == nil {
			return p
		}
		if p, err := exec.LookPath(name + ".exe"); err == nil {
			return p
		}
	}

	if runtime.GOOS == "linux" && username != "" && username != "root" {
		userHome := filepath.Join("/home", username)
		for _, pat := range []string{
			filepath.Join(userHome, ".nvm/versions/node/*/bin", name),
			filepath.Join(userHome, ".npm-global/bin", name),
			filepath.Join(userHome, ".local/share/pnpm", name),
			filepath.Join(userHome, ".volta/bin", name),
		} {
			if matches, _ := filepath.Glob(pat); len(matches) > 0 {
				return matches[len(matches)-1]
			}
		}
	}

	if p, err := exec.LookPath(name); err == nil {
		return p
	}

	if runtime.GOOS == "linux" {
		for _, c := range []string{"/usr/local/bin/" + name, "/usr/bin/" + name, "/bin/" + name} {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
		for _, pat := range []string{
			filepath.Join("/root", ".nvm/versions/node/*/bin", name),
			filepath.Join("/home/*", ".nvm/versions/node/*/bin", name),
			filepath.Join("/home/*", ".npm-global/bin", name),
		} {
			if matches, _ := filepath.Glob(pat); len(matches) > 0 {
				return matches[len(matches)-1]
			}
		}
	}

	return ""
}

// discoverHostPM2Users mendeteksi seluruh pemilik folder .pm2 (/root/.pm2 dan /home/*/.pm2)
func discoverHostPM2Users() []string {
	if runtime.GOOS != "linux" {
		return []string{"default"}
	}

	var users []string
	seen := make(map[string]bool)

	if info, err := os.Stat("/root/.pm2"); err == nil && info.IsDir() {
		users = append(users, "root")
		seen["root"] = true
	}

	if entries, err := os.ReadDir("/home"); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			u := entry.Name()
			if seen[u] {
				continue
			}
			pm2Dir := filepath.Join("/home", u, ".pm2")
			if info, err := os.Stat(pm2Dir); err == nil && info.IsDir() {
				users = append(users, u)
				seen[u] = true
			}
		}
	}

	if len(users) == 0 {
		users = append(users, "root")
	}
	return users
}

// ScanPM2Apps memindai proses PM2 yang sedang berjalan dari seluruh user (/root/.pm2 dan /home/*/.pm2)
func (s *Scanner) ScanPM2Apps(ctx context.Context) ([]PM2DiscoveredItem, error) {
	if resolvePM2BinForUser("pm2", "") == "" {
		return []PM2DiscoveredItem{}, nil
	}

	users := discoverHostPM2Users()
	apps := []PM2DiscoveredItem{}
	seenKey := make(map[string]bool)

	for _, username := range users {
		select {
		case <-ctx.Done():
			return apps, nil
		default:
		}

		pm2Bin := resolvePM2BinForUser("pm2", username)
		if pm2Bin == "" {
			continue
		}

		userCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		var cmd *exec.Cmd

		if runtime.GOOS == "linux" && username != "root" && username != "default" {
			homeDir := filepath.Join("/home", username)
			pm2Home := filepath.Join(homeDir, ".pm2")
			pathParts := []string{filepath.Dir(pm2Bin)}
			if nodeBin := resolvePM2BinForUser("node", username); nodeBin != "" {
				pathParts = append(pathParts, filepath.Dir(nodeBin))
			}
			pathParts = append(pathParts, "/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin")
			richPath := strings.Join(pathParts, ":")

			if sudoBin, errSudo := exec.LookPath("sudo"); errSudo == nil {
				cmd = exec.CommandContext(userCtx, sudoBin, "-n", "-u", username, "env",
					"HOME="+homeDir,
					"PM2_HOME="+pm2Home,
					"PATH="+richPath,
					pm2Bin, "jlist")
			} else {
				cmd = exec.CommandContext(userCtx, pm2Bin, "jlist")
				cmd.Env = append(os.Environ(), "HOME="+homeDir, "PM2_HOME="+pm2Home, "PATH="+richPath)
			}
		} else {
			cmd = exec.CommandContext(userCtx, pm2Bin, "jlist")
			if runtime.GOOS == "linux" {
				cmd.Env = append(os.Environ(), "HOME=/root", "PM2_HOME=/root/.pm2")
			}
		}

		out, err := cmd.Output()
		cancel()
		if err != nil || len(out) == 0 {
			continue
		}

		outStr := string(out)
		firstBracket := strings.Index(outStr, "[")
		lastBracket := strings.LastIndex(outStr, "]")
		if firstBracket == -1 || lastBracket == -1 || lastBracket < firstBracket {
			continue
		}

		var rawList []map[string]interface{}
		if err := json.Unmarshal([]byte(outStr[firstBracket:lastBracket+1]), &rawList); err != nil {
			continue
		}

		for _, item := range rawList {
			app := PM2DiscoveredItem{
				Instances: 1,
				User:      username,
			}
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
				if u, ok := pm2Env["username"].(string); ok && strings.TrimSpace(u) != "" {
					app.User = strings.TrimSpace(u)
				}
				if sc, ok := pm2Env["pm_exec_path"].(string); ok {
					app.Script = sc
				}
				if cwd, ok := pm2Env["pm_cwd"].(string); ok {
					app.WorkingDir = cwd
				}
			}

			key := fmt.Sprintf("%s:%d:%s", app.User, app.ID, app.Name)
			if !seenKey[key] {
				seenKey[key] = true
				apps = append(apps, app)
			}
		}
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

	// Cek kubectl umum dengan batas waktu singkat
	if _, err := exec.LookPath("kubectl"); err == nil {
		k8sCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(k8sCtx, "kubectl", "version", "--client", "--output=json")
		if err := cmd.Run(); err == nil {
			return true, "standard"
		}
	}

	return false, "none"
}

// ScanAll menjalankan seluruh modul deteksi secara komprehensif dengan batas waktu 10 detik
func (s *Scanner) ScanAll(ctx context.Context, customRoots []string) (*DiscoveryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res := &DiscoveryResult{
		NginxVhosts:    []NginxVhostItem{},
		DockerProjects: []DockerDiscoveredItem{},
		PM2Apps:        []PM2DiscoveredItem{},
		Databases:      []DatabaseDiscoveredItem{},
		KubernetesType: "none",
	}

	if vhosts, err := s.ScanNginxVhosts(ctx); err == nil && vhosts != nil {
		res.NginxVhosts = vhosts
	}

	// Batasi pemindaian berkas compose maksimal 5 detik agar PM2 & K8s tetap memiliki sisa waktu
	dockerCtx, cancelDocker := context.WithTimeout(ctx, 5*time.Second)
	if dockerProjs, _ := s.ScanDockerProjects(dockerCtx, customRoots); dockerProjs != nil {
		res.DockerProjects = dockerProjs
	}
	cancelDocker()

	if pm2Apps, _ := s.ScanPM2Apps(ctx); pm2Apps != nil {
		res.PM2Apps = pm2Apps
	}

	k8sDetected, k8sType := DetectKubernetes(ctx)
	res.KubernetesDetected = k8sDetected
	res.KubernetesType = k8sType

	return res, nil
}
