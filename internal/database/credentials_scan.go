package database

import (
	"bufio"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
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
	reComposeEnvLine = regexp.MustCompile(`^\s*-?\s*([A-Za-z0-9_]+)\s*[:=]\s*(.+?)\s*$`)
)

// unquoteConfigValue membersihkan tanda kutip tunggal/ganda serta komentar ekor pada nilai konfigurasi
func unquoteConfigValue(raw string) string {
	v := strings.TrimSpace(raw)
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	// Buang komentar inline apabila nilai tidak dibungkus tanda kutip
	if idx := strings.Index(v, " #"); idx > 0 {
		v = strings.TrimSpace(v[:idx])
	}
	return v
}

// ParseDatabaseURL mengekstrak engine, host, username, password, dan nama DB dari URI koneksi (mysql:// atau postgres://)
func ParseDatabaseURL(rawURL string) (DiscoveredDBCredential, bool) {
	uStr := unquoteConfigValue(rawURL)
	if uStr == "" || !strings.Contains(uStr, "://") {
		return DiscoveredDBCredential{}, false
	}
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

// ParseEnvCredentials mengekstrak kredensial database dari konten berkas .env (Laravel, Node, Go, Python, Docker)
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

	// 1. Cek DSN berbasis URL (DATABASE_URL, MYSQL_URL, DB_URL, POSTGRES_URL)
	for _, urlKey := range []string{"DATABASE_URL", "DB_URL", "MYSQL_URL", "POSTGRES_URL", "POSTGRESQL_URL"} {
		if rawURL := kv[urlKey]; rawURL != "" {
			if cred, ok := ParseDatabaseURL(rawURL); ok {
				cred.SourcePath = sourcePath
				results = append(results, cred)
			}
		}
	}

	// 2. Cek skema standar DB_DATABASE / DB_USERNAME / DB_PASSWORD (Laravel, CodeIgniter, Node, Go)
	if dbName := firstNonEmpty(kv["DB_DATABASE"], kv["DB_NAME"], kv["MYSQL_DATABASE"], kv["MYSQL_DB"]); dbName != "" {
		conn := strings.ToLower(firstNonEmpty(kv["DB_CONNECTION"], kv["DB_DRIVER"], "mysql"))
		eng := "mysql"
		if strings.Contains(conn, "pg") || strings.Contains(conn, "postgres") {
			eng = "postgresql"
		}
		user := firstNonEmpty(kv["DB_USERNAME"], kv["DB_USER"], kv["MYSQL_USER"])
		pass := firstNonEmpty(kv["DB_PASSWORD"], kv["DB_PASS"], kv["MYSQL_PASSWORD"])
		host := firstNonEmpty(kv["DB_HOST"], kv["MYSQL_HOST"], "localhost")
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
	// Pisahkan port bila tertulis localhost:3306
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

// ParseDockerComposeCredentials mengekstrak variabel lingkungan database dari berkas docker-compose.yml
func ParseDockerComposeCredentials(content, sourcePath string) []DiscoveredDBCredential {
	var envLines strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		m := reComposeEnvLine.FindStringSubmatch(line)
		if len(m) >= 3 {
			k := strings.ToUpper(m[1])
			if strings.HasPrefix(k, "DB_") || strings.HasPrefix(k, "MYSQL_") || strings.HasPrefix(k, "POSTGRES_") || k == "DATABASE_URL" {
				envLines.WriteString(k + "=" + m[2] + "\n")
			}
		}
	}
	return ParseEnvCredentials(envLines.String(), sourcePath)
}

func firstNonEmpty(items ...string) string {
	for _, s := range items {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// DiscoverLocalDBCredentials memindai berkas .env, wp-config.php, docker-compose, dan config-db.php di server host
// untuk mencocokkan username & password plaintext database yang sudah ada tanpa mengubah server database.
func DiscoverLocalDBCredentials(extraDirs []string) map[string]DiscoveredDBCredential {
	found := make(map[string]DiscoveredDBCredential)

	recordCred := func(c DiscoveredDBCredential) {
		if strings.TrimSpace(c.DBName) == "" {
			return
		}
		// Abaikan placeholder umum
		if strings.Contains(c.Password, "${") || strings.Contains(c.Password, "<") {
			return
		}
		key := strings.ToLower(c.Engine) + ":" + strings.ToLower(strings.TrimSpace(c.DBName))
		prev, exists := found[key]
		// Prioritaskan entri yang memiliki password terisi
		if !exists || (prev.Password == "" && c.Password != "") {
			found[key] = c
		}
	}

	// 1. Cek berkas standar Debian/Ubuntu phpMyAdmin
	if runtime.GOOS == "linux" {
		if raw, err := os.ReadFile("/etc/phpmyadmin/config-db.php"); err == nil {
			if cred, ok := ParsePhpMyAdminConfigDB(string(raw), "/etc/phpmyadmin/config-db.php"); ok {
				recordCred(cred)
			}
		}
	}

	// 2. Kumpulkan seluruh direktori proyek unik yang akan dipindai (prioritaskan extraDirs dari situs terdaftar)
	candidateRoots := make([]string, 0, len(extraDirs)+12)
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

	for _, d := range extraDirs {
		addDir(d)
		// Situs Laravel / SPA sering punya root_path di /public atau /dist, cek juga direktori induknya
		parent := filepath.Dir(filepath.Clean(d))
		base := strings.ToLower(filepath.Base(filepath.Clean(d)))
		if base == "public" || base == "dist" || base == "build" || base == "out" {
			addDir(parent)
		}
	}

	if runtime.GOOS == "linux" {
		for _, stdDir := range []string{"/var/www", "/www/wwwroot", "/www/sites", "/opt", "/srv"} {
			addDir(stdDir)
		}
		// Tambahkan direktori home pengguna (mis. /home/fikfikk, /home/fikfikk/projects)
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
	}

	scanConfigFilesInDir := func(dir string) {
		for _, envName := range []string{".env", ".env.production", ".env.local"} {
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
		for _, dcName := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml"} {
			p := filepath.Join(dir, dcName)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() <= 512*1024 {
				if raw, err := os.ReadFile(p); err == nil {
					for _, c := range ParseDockerComposeCredentials(string(raw), p) {
						recordCred(c)
					}
				}
			}
		}
	}

	// Pindai langsung + 2 tingkat subdirektori agar super cepat (< 100ms) dan tidak tersesat
	scannedDir := make(map[string]bool)
	var walkLimited func(dir string, depth int)
	walkLimited = func(dir string, depth int) {
		if scannedDir[dir] || depth > 2 {
			return
		}
		scannedDir[dir] = true
		scanConfigFilesInDir(dir)
		if depth == 2 {
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
