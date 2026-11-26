package database

import (
	"bufio"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// DiscoveredDBCredential menyimpan kredensial database plaintext yang terdeteksi dari konfigurasi proyek di server
type DiscoveredDBCredential struct {
	Engine     string
	DBName     string
	Username   string
	Password   string
	Host       string
	SourcePath string
}

var (
	reWPDefine       = regexp.MustCompile(`(?i)define\s*\(\s*['"](DB_NAME|DB_USER|DB_PASSWORD|DB_HOST)['"]\s*,\s*['"]([^'"]*)['"]\s*\)`)
	rePmaConfigVar   = regexp.MustCompile(`\$(dbname|dbuser|dbpass|dbserver)\s*=\s*['"]([^'"]*)['"]\s*;`)
	reComposeEnvLine = regexp.MustCompile(`^\s*-?\s*['"]?([A-Za-z0-9_]+)['"]?\s*[:=]\s*(.+?)\s*,?\s*$`)
	// Pola DSN Go go-sql-driver/mysql: user:pass@tcp(host:port)/dbname?params
	reGoMysqlDSN = regexp.MustCompile(`^([^:\s@]+):([^@\s]*)@(?:tcp|unix)\(([^)]*)\)/([^?\s/'"]+)`)
	// Pola URI koneksi tertanam di berkas JSON/JS/YAML
	reEmbeddedURI = regexp.MustCompile(`(?:mysql2?|mariadb|postgres(?:ql)?|pgsql|mongodb(?:\+srv)?)://[^\s'"` + "`" + `,;}\]]+`)
	// Pola Go MySQL DSN tertanam di berkas JSON/JS/YAML
	reEmbeddedGoDSN = regexp.MustCompile(`[A-Za-z0-9_.-]+:[^\s'"` + "`" + `@]+@tcp\([^)]+\)/[A-Za-z0-9_.-]+`)
	// Pola pasangan key: "val" atau key: 'val' inline (untuk JSON/JS minified atau objek satu baris)
	reInlineConfigKV = regexp.MustCompile(`['"]?((?:DB|MYSQL|POSTGRES|DATABASE)_[A-Za-z0-9_]+|(?:[A-Za-z0-9_]+_)?DSN)['"]?\s*[:=]\s*['"]([^'"]+)['"]`)
)

// unquoteConfigValue membersihkan tanda kutip tunggal/ganda serta komentar ekor pada nilai konfigurasi
func unquoteConfigValue(raw string) string {
	v := strings.TrimSpace(raw)
	v = strings.TrimSuffix(v, ",")
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') || (v[0] == '`' && v[len(v)-1] == '`') {
			return v[1 : len(v)-1]
		}
	}
	if idx := strings.Index(v, " #"); idx > 0 {
		v = strings.TrimSpace(v[:idx])
	}
	return v
}

// ParseDatabaseURL mengekstrak kredensial dari URI (mysql://, postgres://), Go MySQL DSN (user:pass@tcp(...)/db), atau Go Postgres DSN (host=... user=...)
func ParseDatabaseURL(rawURL string) (DiscoveredDBCredential, bool) {
	uStr := unquoteConfigValue(rawURL)
	if uStr == "" {
		return DiscoveredDBCredential{}, false
	}

	// 1. Format URI standar (mysql://, postgres://, mongodb://)
	if strings.Contains(uStr, "://") {
		u, err := url.Parse(uStr)
		if err != nil {
			return DiscoveredDBCredential{}, false
		}
		var eng string
		switch strings.ToLower(u.Scheme) {
		case "mysql", "mysql2", "mariadb":
			eng = "mysql"
		case "postgres", "postgresql", "pgsql":
			eng = "postgresql"
		case "mongodb", "mongodb+srv":
			eng = "mongodb"
		default:
			return DiscoveredDBCredential{}, false
		}

		dbName := strings.TrimPrefix(strings.TrimSpace(u.Path), "/")
		if idx := strings.Index(dbName, "/"); idx >= 0 {
			dbName = dbName[:idx]
		}
		if dbName == "" {
			return DiscoveredDBCredential{}, false
		}

		var user, pass string
		if u.User != nil {
			user = u.User.Username()
			pass, _ = u.User.Password()
		}
		host := u.Hostname()
		if host == "" {
			host = "localhost"
		}

		return DiscoveredDBCredential{
			Engine:   eng,
			DBName:   dbName,
			Username: user,
			Password: pass,
			Host:     host,
		}, true
	}

	// 2. Format Go go-sql-driver/mysql: user:pass@tcp(127.0.0.1:3306)/qr-fik?parseTime=true
	if m := reGoMysqlDSN.FindStringSubmatch(uStr); len(m) >= 5 {
		hostPort := strings.TrimSpace(m[3])
		host := "localhost"
		if hostPort != "" && !strings.HasPrefix(hostPort, "/") {
			if colon := strings.LastIndex(hostPort, ":"); colon > 0 {
				host = hostPort[:colon]
			} else {
				host = hostPort
			}
		}
		return DiscoveredDBCredential{
			Engine:   "mysql",
			DBName:   strings.TrimSpace(m[4]),
			Username: strings.TrimSpace(m[1]),
			Password: m[2],
			Host:     host,
		}, true
	}

	// 3. Format Go lib/pq & GORM Postgres key-value: host=127.0.0.1 user=app password=secret dbname=mydb port=5432
	if strings.Contains(uStr, "dbname=") && (strings.Contains(uStr, "user=") || strings.Contains(uStr, "password=")) {
		kv := map[string]string{}
		for _, tok := range strings.Fields(uStr) {
			if eq := strings.IndexByte(tok, '='); eq > 0 {
				kv[strings.ToLower(tok[:eq])] = unquoteConfigValue(tok[eq+1:])
			}
		}
		if kv["dbname"] != "" {
			host := firstNonEmpty(kv["host"], "localhost")
			return DiscoveredDBCredential{
				Engine:   "postgresql",
				DBName:   kv["dbname"],
				Username: kv["user"],
				Password: kv["password"],
				Host:     host,
			}, true
		}
	}

	return DiscoveredDBCredential{}, false
}

// ParseEnvCredentials mengekstrak kredensial database dari konten berkas .env atau pasangan key=value lingkungan
func ParseEnvCredentials(content, sourcePath string) []DiscoveredDBCredential {
	kv := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.ToUpper(strings.TrimSpace(line[:eq]))
		v := unquoteConfigValue(line[eq+1:])
		kv[k] = v
	}

	var results []DiscoveredDBCredential

	// 1. Cek DSN berbasis URL atau format Go DSN pada seluruh variabel umum
	for _, urlKey := range []string{
		"DATABASE_URL", "DB_URL", "MYSQL_URL", "POSTGRES_URL", "POSTGRESQL_URL",
		"DSN", "DB_DSN", "MYSQL_DSN", "POSTGRES_DSN", "DATABASE_DSN", "GO_DSN",
	} {
		if rawURL := kv[urlKey]; rawURL != "" {
			if cred, ok := ParseDatabaseURL(rawURL); ok {
				cred.SourcePath = sourcePath
				results = append(results, cred)
			}
		}
	}

	// 2. Cek skema standar DB_DATABASE / DB_USERNAME / DB_PASSWORD (Laravel, Node, Go, Python)
	if dbName := firstNonEmpty(kv["DB_DATABASE"], kv["DB_NAME"], kv["MYSQL_DATABASE"], kv["MYSQL_DB"], kv["DATABASE_NAME"]); dbName != "" {
		conn := strings.ToLower(firstNonEmpty(kv["DB_CONNECTION"], kv["DB_DRIVER"], kv["DB_TYPE"], "mysql"))
		eng := "mysql"
		if strings.Contains(conn, "pg") || strings.Contains(conn, "postgres") {
			eng = "postgresql"
		}
		user := firstNonEmpty(kv["DB_USERNAME"], kv["DB_USER"], kv["MYSQL_USER"], kv["MYSQL_USERNAME"], kv["DATABASE_USER"])
		pass := firstNonEmpty(kv["DB_PASSWORD"], kv["DB_PASS"], kv["MYSQL_PASSWORD"], kv["MYSQL_PASS"], kv["MYSQL_ROOT_PASSWORD"], kv["DATABASE_PASSWORD"])
		host := firstNonEmpty(kv["DB_HOST"], kv["MYSQL_HOST"], kv["DATABASE_HOST"], "localhost")
		results = append(results, DiscoveredDBCredential{
			Engine:     eng,
			DBName:     dbName,
			Username:   user,
			Password:   pass,
			Host:       host,
			SourcePath: sourcePath,
		})
	}

	// 3. Cek skema variabel khusus PostgreSQL (POSTGRES_DB / PGDATABASE)
	if pgDB := firstNonEmpty(kv["POSTGRES_DB"], kv["PGDATABASE"]); pgDB != "" {
		user := firstNonEmpty(kv["POSTGRES_USER"], kv["PGUSER"], "postgres")
		pass := firstNonEmpty(kv["POSTGRES_PASSWORD"], kv["PGPASSWORD"])
		host := firstNonEmpty(kv["POSTGRES_HOST"], kv["PGHOST"], "localhost")
		results = append(results, DiscoveredDBCredential{
			Engine:     "postgresql",
			DBName:     pgDB,
			Username:   user,
			Password:   pass,
			Host:       host,
			SourcePath: sourcePath,
		})
	}

	return results
}

// ParseWPConfigCredentials mengekstrak kredensial MySQL dari berkas wp-config.php WordPress
func ParseWPConfigCredentials(content, sourcePath string) (DiscoveredDBCredential, bool) {
	vals := make(map[string]string)
	for _, m := range reWPDefine.FindAllStringSubmatch(content, -1) {
		if len(m) >= 3 {
			vals[strings.ToUpper(m[1])] = m[2]
		}
	}
	if vals["DB_NAME"] == "" {
		return DiscoveredDBCredential{}, false
	}
	host := vals["DB_HOST"]
	if host == "" {
		host = "localhost"
	}
	if colon := strings.IndexByte(host, ':'); colon > 0 {
		host = host[:colon]
	}
	return DiscoveredDBCredential{
		Engine:     "mysql",
		DBName:     vals["DB_NAME"],
		Username:   vals["DB_USER"],
		Password:   vals["DB_PASSWORD"],
		Host:       host,
		SourcePath: sourcePath,
	}, true
}

// ParsePhpMyAdminConfigDB mengekstrak kredensial database phpmyadmin dari /etc/phpmyadmin/config-db.php
func ParsePhpMyAdminConfigDB(content, sourcePath string) (DiscoveredDBCredential, bool) {
	vals := make(map[string]string)
	for _, m := range rePmaConfigVar.FindAllStringSubmatch(content, -1) {
		if len(m) >= 3 {
			vals[strings.ToLower(m[1])] = m[2]
		}
	}
	if vals["dbname"] == "" {
		return DiscoveredDBCredential{}, false
	}
	host := vals["dbserver"]
	if host == "" {
		host = "localhost"
	}
	return DiscoveredDBCredential{
		Engine:     "mysql",
		DBName:     vals["dbname"],
		Username:   vals["dbuser"],
		Password:   vals["dbpass"],
		Host:       host,
		SourcePath: sourcePath,
	}, true
}

// ParseStructuredConfigCredentials mengekstrak kredensial dari berkas docker-compose.yml, ecosystem.config.cjs, config.json/yaml/toml
func ParseStructuredConfigCredentials(content, sourcePath string) []DiscoveredDBCredential {
	var results []DiscoveredDBCredential

	// 1. Pindai URI koneksi & Go DSN yang tertanam langsung di dalam berkas
	for _, uri := range reEmbeddedURI.FindAllString(content, 10) {
		if c, ok := ParseDatabaseURL(uri); ok {
			c.SourcePath = sourcePath
			results = append(results, c)
		}
	}
	for _, dsn := range reEmbeddedGoDSN.FindAllString(content, 10) {
		if c, ok := ParseDatabaseURL(dsn); ok {
			c.SourcePath = sourcePath
			results = append(results, c)
		}
	}

	// 2. Ekstrak pasangan key: value / key = value (baik format multiline maupun inline JSON/JS)
	var envLines strings.Builder
	for _, m := range reInlineConfigKV.FindAllStringSubmatch(content, -1) {
		if len(m) >= 3 {
			envLines.WriteString(strings.ToUpper(m[1]) + "=" + m[2] + "\n")
		}
	}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		m := reComposeEnvLine.FindStringSubmatch(scanner.Text())
		if len(m) >= 3 {
			k := strings.ToUpper(m[1])
			if strings.HasPrefix(k, "DB_") || strings.HasPrefix(k, "MYSQL_") || strings.HasPrefix(k, "POSTGRES_") || strings.HasPrefix(k, "DATABASE_") || strings.HasSuffix(k, "_DSN") || k == "DSN" {
				envLines.WriteString(k + "=" + m[2] + "\n")
			}
		}
	}
	if envLines.Len() > 0 {
		results = append(results, ParseEnvCredentials(envLines.String(), sourcePath)...)
	}

	return results
}

// ParseDockerComposeCredentials tetap dipertahankan sebagai alias kompatibel
func ParseDockerComposeCredentials(content, sourcePath string) []DiscoveredDBCredential {
	return ParseStructuredConfigCredentials(content, sourcePath)
}

func firstNonEmpty(items ...string) string {
	for _, s := range items {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// scanSystemdUnits membaca Environment, EnvironmentFile, dan WorkingDirectory dari /etc/systemd/system/*.service
func scanSystemdUnits(recordCred func(DiscoveredDBCredential), addDir func(string)) {
	files, err := filepath.Glob("/etc/systemd/system/*.service")
	if err != nil {
		return
	}
	for _, fpath := range files {
		raw, err := os.ReadFile(fpath)
		if err != nil {
			continue
		}
		var envBuf strings.Builder
		scanner := bufio.NewScanner(strings.NewReader(string(raw)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "WorkingDirectory=") {
				wd := unquoteConfigValue(strings.TrimPrefix(line, "WorkingDirectory="))
				if wd != "" {
					addDir(wd)
				}
			} else if strings.HasPrefix(line, "EnvironmentFile=") {
				ef := strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(line, "EnvironmentFile=")), "-")
				ef = unquoteConfigValue(ef)
				if ef != "" {
					if data, err := os.ReadFile(ef); err == nil {
						for _, c := range ParseEnvCredentials(string(data), ef) {
							recordCred(c)
						}
					}
				}
			} else if strings.HasPrefix(line, "Environment=") {
				val := unquoteConfigValue(strings.TrimPrefix(line, "Environment="))
				if val != "" {
					envBuf.WriteString(val + "\n")
				}
			}
		}
		if envBuf.Len() > 0 {
			for _, c := range ParseEnvCredentials(envBuf.String(), fpath) {
				recordCred(c)
			}
		}
	}
}

// scanPM2Dumps membaca env & direktori proyek dari berkas dump.pm2 di /root/.pm2 dan /home/*/.pm2
func scanPM2Dumps(recordCred func(DiscoveredDBCredential), addDir func(string)) {
	candidates := []string{"/root/.pm2/dump.pm2"}
	if matches, err := filepath.Glob("/home/*/.pm2/dump.pm2"); err == nil {
		candidates = append(candidates, matches...)
	}
	for _, dumpPath := range candidates {
		raw, err := os.ReadFile(dumpPath)
		if err != nil {
			continue
		}
		var procs []struct {
			PmCwd string                 `json:"pm_cwd"`
			Env   map[string]interface{} `json:"env"`
		}
		if err := json.Unmarshal(raw, &procs); err != nil {
			continue
		}
		for _, p := range procs {
			if p.PmCwd != "" {
				addDir(p.PmCwd)
			}
			if len(p.Env) > 0 {
				var envBuf strings.Builder
				for k, v := range p.Env {
					if s, ok := v.(string); ok {
						envBuf.WriteString(k + "=" + s + "\n")
					}
				}
				for _, c := range ParseEnvCredentials(envBuf.String(), dumpPath) {
					recordCred(c)
				}
			}
		}
	}
}

// scanDockerContainers membaca variabel lingkungan database dari kontainer Docker yang sedang berjalan (timeout 2 detik)
func scanDockerContainers(recordCred func(DiscoveredDBCredential)) {
	if _, err := exec.LookPath("docker"); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	idsOut, err := exec.CommandContext(ctx, "docker", "ps", "-q").Output()
	if err != nil {
		return
	}
	ids := strings.Fields(strings.TrimSpace(string(idsOut)))
	if len(ids) == 0 {
		return
	}
	args := append([]string{"inspect", "--format", "{{.Name}}\n{{range .Config.Env}}{{println .}}{{end}}---FIKPANEL_SEP---"}, ids...)
	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return
	}
	for _, block := range strings.Split(string(out), "---FIKPANEL_SEP---") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.SplitN(block, "\n", 2)
		if len(lines) < 2 {
			continue
		}
		cName := strings.TrimPrefix(strings.TrimSpace(lines[0]), "/")
		for _, c := range ParseEnvCredentials(lines[1], "docker:"+cName) {
			recordCred(c)
		}
	}
}

// DiscoverLocalDBCredentials memindai berkas proyek, systemd, PM2, Docker, dan config-db.php di server host
// untuk mencocokkan username & password plaintext database yang sudah ada tanpa mengubah server database.
func DiscoverLocalDBCredentials(extraDirs []string) map[string]DiscoveredDBCredential {
	found := make(map[string]DiscoveredDBCredential)

	recordCred := func(c DiscoveredDBCredential) {
		if strings.TrimSpace(c.DBName) == "" {
			return
		}
		if strings.Contains(c.Password, "${") || strings.Contains(c.Password, "<") {
			return
		}
		key := strings.ToLower(c.Engine) + ":" + strings.ToLower(strings.TrimSpace(c.DBName))
		prev, exists := found[key]
		if !exists || (prev.Password == "" && c.Password != "") {
			found[key] = c
		}
	}

	candidateRoots := make([]string, 0, len(extraDirs)+20)
	seenDir := make(map[string]bool)
	addDir := func(d string) {
		clean := filepath.Clean(strings.TrimSpace(d))
		if clean == "" || clean == "." || clean == "/" || seenDir[clean] {
			return
		}
		if fi, err := os.Stat(clean); err == nil && fi.IsDir() {
			seenDir[clean] = true
			candidateRoots = append(candidateRoots, clean)
		}
	}

	// 1. Cek sumber khusus Linux (phpMyAdmin, systemd, PM2 dump, Docker running containers)
	if runtime.GOOS == "linux" {
		if raw, err := os.ReadFile("/etc/phpmyadmin/config-db.php"); err == nil {
			if cred, ok := ParsePhpMyAdminConfigDB(string(raw), "/etc/phpmyadmin/config-db.php"); ok {
				recordCred(cred)
			}
		}
		scanSystemdUnits(recordCred, addDir)
		scanPM2Dumps(recordCred, addDir)
		scanDockerContainers(recordCred)
	}

	// 2. Kumpulkan seluruh direktori proyek dari situs terdaftar dan direktori standar host
	for _, d := range extraDirs {
		addDir(d)
		parent := filepath.Dir(filepath.Clean(d))
		base := strings.ToLower(filepath.Base(filepath.Clean(d)))
		if base == "public" || base == "dist" || base == "build" || base == "out" {
			addDir(parent)
		}
	}

	if runtime.GOOS == "linux" {
		for _, stdDir := range []string{"/var/www", "/www", "/www/wwwroot", "/www/sites", "/opt", "/srv", "/root", "/root/projects"} {
			addDir(stdDir)
		}
		if homes, err := filepath.Glob("/home/*"); err == nil {
			for _, h := range homes {
				addDir(h)
				addDir(filepath.Join(h, "projects"))
				addDir(filepath.Join(h, "apps"))
			}
		}
	}

	skipDirs := map[string]bool{
		"node_modules": true, ".git": true, "vendor": true, "storage": true,
		".cache": true, ".venv": true, "venv": true, "dist": true, "build": true,
		".next": true, ".nuxt": true, ".output": true, "tmp": true, "logs": true,
	}

	envFiles := []string{".env", ".env.production", ".env.prod", ".env.local", ".env.development"}
	structFiles := []string{
		"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml",
		"ecosystem.config.js", "ecosystem.config.cjs", "ecosystem.config.json",
		"config.json", "config.yaml", "config.yml", "application.yml", "config.toml",
	}

	scanConfigFilesInDir := func(dir string) {
		for _, envName := range envFiles {
			p := filepath.Join(dir, envName)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() <= 512*1024 {
				if raw, err := os.ReadFile(p); err == nil {
					for _, c := range ParseEnvCredentials(string(raw), p) {
						recordCred(c)
					}
				}
			}
		}
		wpPath := filepath.Join(dir, "wp-config.php")
		if fi, err := os.Stat(wpPath); err == nil && !fi.IsDir() && fi.Size() <= 512*1024 {
			if raw, err := os.ReadFile(wpPath); err == nil {
				if c, ok := ParseWPConfigCredentials(string(raw), wpPath); ok {
					recordCred(c)
				}
			}
		}
		for _, sName := range structFiles {
			p := filepath.Join(dir, sName)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() <= 512*1024 {
				if raw, err := os.ReadFile(p); err == nil {
					for _, c := range ParseStructuredConfigCredentials(string(raw), p) {
						recordCred(c)
					}
				}
			}
		}
	}

	// Pindai hingga 3 tingkat subdirektori (mendukung monorepo /backend, /api, /server) dengan batas ketat skipDirs
	scannedDir := make(map[string]bool)
	var walkLimited func(dir string, depth int)
	walkLimited = func(dir string, depth int) {
		if scannedDir[dir] || depth > 3 {
			return
		}
		scannedDir[dir] = true
		scanConfigFilesInDir(dir)
		if depth == 3 {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") || skipDirs[strings.ToLower(name)] {
				continue
			}
			walkLimited(filepath.Join(dir, name), depth+1)
		}
	}

	for _, root := range candidateRoots {
		walkLimited(root, 0)
	}

	return found
}
