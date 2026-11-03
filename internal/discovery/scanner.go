package discovery

import (
	"bufio"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
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
	ConfigFile  string   `json:"config_file"`
	ServerName  string   `json:"server_name"`
	Aliases     []string `json:"aliases,omitempty"`
	RootDir     string   `json:"root_dir"`
	Port        int      `json:"port"`
	SSL         bool     `json:"ssl"`
	ProxyPass   string   `json:"proxy_pass"`
	IsAdopted   bool     `json:"is_adopted"`
	SSLCertPath string   `json:"ssl_cert_path,omitempty"`
	SSLKeyPath  string   `json:"ssl_key_path,omitempty"`
	SSLIssuer   string   `json:"ssl_issuer,omitempty"`
	SSLNotAfter int64    `json:"ssl_not_after,omitempty"`
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

// InspectCertificate menganalisis berkas sertifikat TLS dan mengekstrak issuer, notAfter, dan domain SAN
func InspectCertificate(certPath string) (issuer string, notAfter int64, domains []string) {
	return inspectCertificate(certPath)
}

// inspectCertificate menganalisis berkas sertifikat TLS dan mengekstrak issuer, notAfter, dan domain SAN
func inspectCertificate(certPath string) (issuer string, notAfter int64, domains []string) {
	cleanPath := filepath.ToSlash(certPath)
	lowerPath := strings.ToLower(cleanPath)

	// Klasifikasi issuer awal berdasarkan path direktori standar
	if strings.Contains(lowerPath, "letsencrypt") {
		issuer = "Let's Encrypt"
	} else if strings.Contains(lowerPath, "zerossl") {
		issuer = "ZeroSSL"
	} else if strings.Contains(lowerPath, "cloudflare") {
		issuer = "Cloudflare Origin CA"
	} else {
		issuer = "Custom SSL"
	}

	data, err := os.ReadFile(certPath)
	if err != nil {
		return issuer, 0, nil
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return issuer, 0, nil
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return issuer, 0, nil
	}

	notAfter = cert.NotAfter.Unix()
	domains = cert.DNSNames
	if len(domains) == 0 && cert.Subject.CommonName != "" {
		domains = []string{cert.Subject.CommonName}
	}

	// Gunakan CommonName atau Organization dari sertifikat X.509 bila tersedia
	if cert.Issuer.CommonName != "" {
		issuer = cert.Issuer.CommonName
	} else if len(cert.Issuer.Organization) > 0 {
		issuer = cert.Issuer.Organization[0]
	}

	return issuer, notAfter, domains
}

// extractServerBlocks mengekstrak blok "server { ... }" dengan parser berbasis stack kurung kurawal
// Mendukung kedalaman nested location berapapun (misal location di dalam location pada kostash.conf)
func extractServerBlocks(content string) []string {
	var blocks []string
	n := len(content)
	i := 0

	for i < n {
		idx := strings.Index(content[i:], "server")
		if idx == -1 {
			break
		}
		pos := i + idx

		// Pastikan "server" berdiri sendiri sebagai kata kunci
		isWordStart := (pos == 0) || isWhitespaceOrDelimiter(content[pos-1])
		posAfter := pos + len("server")
		isWordEnd := (posAfter < n) && isWhitespaceOrOpenBrace(content[posAfter])

		if !isWordStart || !isWordEnd {
			i = pos + len("server")
			continue
		}

		// Telusuri hingga kurung kurawal buka '{'
		braceStart := -1
		j := posAfter
		for j < n {
			c := content[j]
			if c == '{' {
				braceStart = j
				break
			}
			// Batalkan jika ada ';' atau karakter non-spasi (contoh "server 127.0.0.1:8000;" pada upstream)
			if c == ';' || (c != ' ' && c != '\t' && c != '\r' && c != '\n') {
				break
			}
			j++
		}

		if braceStart == -1 {
			i = posAfter
			continue
		}

		// Hitung nesting kurung kurawal dengan stack counter serta mengabaikan kurung dalam tanda kutip
		depth := 1
		k := braceStart + 1
		var inQuote byte

		for k < n && depth > 0 {
			c := content[k]
			if inQuote != 0 {
				if c == inQuote && (k == 0 || content[k-1] != '\\') {
					inQuote = 0
				}
			} else {
				switch c {
				case '"', '\'':
					inQuote = c
				case '{':
					depth++
				case '}':
					depth--
				}
			}
			k++
		}

		if depth == 0 {
			blockContent := content[braceStart+1 : k-1]
			blocks = append(blocks, blockContent)
			i = k
		} else {
			// Kurung kurawal tidak seimbang (unbalanced braces)
			break
		}
	}

	return blocks
}

func isWhitespaceOrDelimiter(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == ';' || b == '}' || b == '{'
}

func isWhitespaceOrOpenBrace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '{'
}

func uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	var list []string
	for _, entry := range slice {
		entry = strings.TrimSpace(entry)
		if entry != "" && !keys[entry] {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// mergeVhostEntries menggabungkan blok HTTP (redirect port 80) dan HTTPS (port 443) untuk domain yang sama
func mergeVhostEntries(items []NginxVhostItem) []NginxVhostItem {
	byPrimary := make(map[string]int)
	var merged []NginxVhostItem

	for _, it := range items {
		fields := strings.Fields(it.ServerName)
		if len(fields) == 0 {
			continue
		}
		primary := fields[0]

		if idx, exists := byPrimary[primary]; exists {
			existing := &merged[idx]
			if it.RootDir != "" && existing.RootDir == "" {
				existing.RootDir = it.RootDir
			}
			if it.ProxyPass != "" && existing.ProxyPass == "" {
				existing.ProxyPass = it.ProxyPass
			}
			if it.SSL {
				existing.SSL = true
				existing.Port = it.Port
				if it.SSLCertPath != "" {
					existing.SSLCertPath = it.SSLCertPath
				}
				if it.SSLKeyPath != "" {
					existing.SSLKeyPath = it.SSLKeyPath
				}
				if it.SSLIssuer != "" {
					existing.SSLIssuer = it.SSLIssuer
				}
				if it.SSLNotAfter > 0 {
					existing.SSLNotAfter = it.SSLNotAfter
				}
			}
			allAliases := append(existing.Aliases, it.Aliases...)
			existing.Aliases = uniqueStrings(allAliases)
			existing.ServerName = strings.Join(append([]string{primary}, existing.Aliases...), " ")
		} else {
			byPrimary[primary] = len(merged)
			merged = append(merged, it)
		}
	}

	return merged
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

	// Ekstrak seluruh blok server menggunakan stack parser kurung kurawal
	serverBlocks := extractServerBlocks(content)

	var items []NginxVhostItem
	for _, block := range serverBlocks {
		item := NginxVhostItem{
			ConfigFile: path,
			Port:       80,
		}

		// 1. Cari server_name (seluruh domain dan alias)
		nameRe := regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)
		if nMatch := nameRe.FindStringSubmatch(block); len(nMatch) > 1 {
			names := strings.Fields(nMatch[1])
			var validNames []string
			for _, nm := range names {
				nm = strings.Trim(strings.TrimSpace(nm), "\"'")
				if nm != "" && nm != "_" && nm != "localhost" && nm != "example.com" {
					validNames = append(validNames, nm)
				}
			}
			if len(validNames) > 0 {
				item.ServerName = strings.Join(validNames, " ")
				if len(validNames) > 1 {
					item.Aliases = validNames[1:]
				}
			}
		}

		// Hanya sertakan domain aktif riil (bukan placeholder, wildcard default, atau localhost)
		if item.ServerName == "" {
			continue
		}

		// 2. Cari listen port & ssl
		listenRe := regexp.MustCompile(`(?m)^\s*listen\s+(?:\[[^\]]+\]:|[0-9.]+:)?([0-9]+)(?:\s+[^;]*)?;`)
		listenMatches := listenRe.FindAllStringSubmatch(block, -1)
		for _, lm := range listenMatches {
			if len(lm) > 1 {
				if p, err := strconv.Atoi(lm[1]); err == nil {
					if item.Port == 0 || item.Port == 80 || p == 443 {
						item.Port = p
					}
				}
			}
			if strings.Contains(lm[0], "ssl") {
				item.SSL = true
			}
		}

		// 3. Cari root directory
		rootRe := regexp.MustCompile(`(?m)^\s*root\s+([^;]+);`)
		if rMatch := rootRe.FindStringSubmatch(block); len(rMatch) > 1 {
			item.RootDir = strings.Trim(strings.TrimSpace(rMatch[1]), "\"'")
		}

		// 4. Cari proxy_pass
		proxyRe := regexp.MustCompile(`(?m)^\s*proxy_pass\s+([^;]+);`)
		if pMatch := proxyRe.FindStringSubmatch(block); len(pMatch) > 1 {
			item.ProxyPass = strings.Trim(strings.TrimSpace(pMatch[1]), "\"'")
		}

		// 5. Ekstraksi dan inspeksi berkas sertifikat SSL
		certRe := regexp.MustCompile(`(?m)^\s*ssl_certificate\s+([^;]+);`)
		if cMatch := certRe.FindStringSubmatch(block); len(cMatch) > 1 {
			item.SSL = true
			item.SSLCertPath = strings.Trim(strings.TrimSpace(cMatch[1]), "\"'")
		}

		keyRe := regexp.MustCompile(`(?m)^\s*ssl_certificate_key\s+([^;]+);`)
		if kMatch := keyRe.FindStringSubmatch(block); len(kMatch) > 1 {
			item.SSLKeyPath = strings.Trim(strings.TrimSpace(kMatch[1]), "\"'")
		}

		if item.SSLCertPath != "" {
			item.SSLIssuer, item.SSLNotAfter, _ = inspectCertificate(item.SSLCertPath)
		}

		items = append(items, item)
	}

	return mergeVhostEntries(items)
}

// ScanNginxVhosts memindai vhost Nginx yang sudah aktif di direktori Linux standar
func (s *Scanner) ScanNginxVhosts(ctx context.Context) ([]NginxVhostItem, error) {
	results := []NginxVhostItem{}
	// Hanya periksa konfigurasi yang benar-benar aktif (sites-enabled dan conf.d)
	dirs := []string{
		"/etc/nginx/sites-enabled",
		"/etc/nginx/conf.d",
	}

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
			results = append(results, vhosts...)
		}
	}

	merged := mergeVhostEntries(results)

	// Deteksi berkas sertifikat SSL fisik dari lokasi umum jika vhost Nginx tidak mencantumkan ssl_certificate secara eksplisit
	for i := range merged {
		if !merged[i].SSL {
			domains := strings.Fields(merged[i].ServerName)
			for _, d := range domains {
				d = strings.ToLower(strings.TrimSpace(d))
				if d == "" || d == "_" || d == "localhost" {
					continue
				}
				leCert := filepath.Join("/etc/letsencrypt/live", d, "fullchain.pem")
				leKey := filepath.Join("/etc/letsencrypt/live", d, "privkey.pem")
				if _, errC := os.Stat(leCert); errC == nil {
					if _, errK := os.Stat(leKey); errK == nil {
						merged[i].SSL = true
						merged[i].SSLCertPath = leCert
						merged[i].SSLKeyPath = leKey
						merged[i].SSLIssuer, merged[i].SSLNotAfter, _ = inspectCertificate(leCert)
						break
					}
				}
			}
		}
	}

	return merged, nil
}

// parseCloudflaredIngress mengekstrak aturan ingress dari berkas konfigurasi cloudflared YAML
func parseCloudflaredIngress(filePath string) []NginxVhostItem {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	var items []NginxVhostItem
	inIngress := false
	ingressIndent := -1

	var curHost string
	var curService string

	flush := func() {
		h := strings.Trim(strings.TrimSpace(curHost), `"'`)
		s := strings.Trim(strings.TrimSpace(curService), `"'`)
		curHost = ""
		curService = ""

		if h == "" || strings.HasPrefix(s, "http_status:") || h == "*" || h == "localhost" {
			return
		}

		// Cloudflare Tunnel ingress mendukung ssh://, tcp://, rdp://, smb:// dll.
		// Virtual host Nginx hanya valid untuk protokol web HTTP, HTTPS, dan UNIX socket.
		lowerS := strings.ToLower(s)
		if !strings.HasPrefix(lowerS, "http://") && !strings.HasPrefix(lowerS, "https://") && !strings.HasPrefix(lowerS, "unix:") {
			return
		}

		port := 443
		if u, err := url.Parse(s); err == nil && u.Port() != "" {
			if p, err := strconv.Atoi(u.Port()); err == nil && p > 0 {
				port = p
			}
		}

		item := NginxVhostItem{
			ConfigFile:  filePath,
			ServerName:  h,
			ProxyPass:   s,
			Port:        port,
			SSL:         true,
			SSLIssuer:   "Cloudflare Tunnel (Edge SSL)",
			SSLNotAfter: time.Now().Add(3650 * 24 * time.Hour).Unix(),
		}
		items = append(items, item)
	}

	for _, line := range lines {
		// Hapus komentar (#)
		trimmedComment := line
		if idx := strings.IndexByte(trimmedComment, '#'); idx >= 0 {
			trimmedComment = trimmedComment[:idx]
		}
		trimmed := strings.TrimSpace(trimmedComment)
		if trimmed == "" {
			continue
		}

		// Deteksi awal blok ingress:
		if strings.HasPrefix(trimmed, "ingress:") {
			inIngress = true
			ingressIndent = len(line) - len(strings.TrimLeft(line, " \t"))
			continue
		}

		if !inIngress {
			continue
		}

		// Hitung indentasi baris saat ini
		curIndent := len(line) - len(strings.TrimLeft(line, " \t"))

		// Jika ada kunci level atas baru dengan indentasi <= ingressIndent, blok ingress berakhir
		if curIndent <= ingressIndent && !strings.HasPrefix(trimmed, "-") && strings.Contains(trimmed, ":") {
			flush()
			inIngress = false
			continue
		}

		// Deteksi item baru dalam daftar YAML (dimulai dengan '-')
		if strings.HasPrefix(trimmed, "-") {
			flush()
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
		}

		// Parse pasangan key: value
		if idx := strings.IndexByte(trimmed, ':'); idx > 0 {
			key := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
			val := strings.TrimSpace(trimmed[idx+1:])

			switch key {
			case "hostname":
				curHost = val
			case "service":
				curService = val
			}
		}
	}

	flush()
	return items
}

// ScanCloudflareTunnelIngress memindai seluruh konfigurasi Cloudflare Tunnel (cloudflared) di server
func (s *Scanner) ScanCloudflareTunnelIngress(ctx context.Context) ([]NginxVhostItem, error) {
	var results []NginxVhostItem
	seenFiles := make(map[string]bool)

	// Daftar berkas dan direktori konfigurasi cloudflared standar
	candidatePaths := []string{
		"/etc/cloudflared/config.yml",
		"/etc/cloudflared/config.yaml",
		"/usr/local/etc/cloudflared/config.yml",
		"/usr/local/etc/cloudflared/config.yaml",
	}

	// Direktori pencarian wildcard (*.yml dan *.yaml)
	candidateDirs := []string{
		"/etc/cloudflared",
		"/usr/local/etc/cloudflared",
	}

	// Periksa home direktori user saat ini
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidatePaths = append(candidatePaths,
			filepath.Join(home, ".cloudflared", "config.yml"),
			filepath.Join(home, ".cloudflared", "config.yaml"),
		)
		candidateDirs = append(candidateDirs, filepath.Join(home, ".cloudflared"))
	}

	// Periksa juga /root/.cloudflared jika ada
	candidatePaths = append(candidatePaths,
		"/root/.cloudflared/config.yml",
		"/root/.cloudflared/config.yaml",
	)
	candidateDirs = append(candidateDirs, "/root/.cloudflared")

	// Pindai /home/*/.cloudflared
	if entries, err := os.ReadDir("/home"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				userCFDir := filepath.Join("/home", e.Name(), ".cloudflared")
				candidateDirs = append(candidateDirs, userCFDir)
				candidatePaths = append(candidatePaths,
					filepath.Join(userCFDir, "config.yml"),
					filepath.Join(userCFDir, "config.yaml"),
				)
			}
		}
	}

	// Pindai file spesifik
	for _, p := range candidatePaths {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		if seenFiles[p] {
			continue
		}
		seenFiles[p] = true

		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			items := parseCloudflaredIngress(p)
			results = append(results, items...)
		}
	}

	// Pindai seluruh file *.yml dan *.yaml di direktori kandidat
	for _, d := range candidateDirs {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
				p := filepath.Join(d, e.Name())
				if seenFiles[p] {
					continue
				}
				seenFiles[p] = true
				items := parseCloudflaredIngress(p)
				results = append(results, items...)
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
			if errRel == nil && strings.Count(rel, string(filepath.Separator)) > 3 {
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

	// Pindai aturan ingress Cloudflare Tunnel (cloudflared)
	if cfTunnels, err := s.ScanCloudflareTunnelIngress(ctx); err == nil && len(cfTunnels) > 0 {
		res.NginxVhosts = append(res.NginxVhosts, cfTunnels...)
	}
	res.NginxVhosts = mergeVhostEntries(res.NginxVhosts)

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
