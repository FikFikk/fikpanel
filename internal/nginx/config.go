package nginx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Jumlah backup konfigurasi yang dipertahankan
const maxBackups = 10

// ReadConfig membaca isi file konfigurasi utama Nginx
func (m *Manager) ReadConfig() (string, error) {
	if err := m.ensureDevFiles(); err != nil {
		return "", err
	}
	f, err := os.Open(m.paths.ConfFile)
	if err != nil {
		return "", fmt.Errorf("gagal membuka %s: %w", m.paths.ConfFile, err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxConfigSize+1))
	if err != nil {
		return "", fmt.Errorf("gagal membaca %s: %w", m.paths.ConfFile, err)
	}
	if len(data) > maxConfigSize {
		return "", &ValidationError{Msg: "file konfigurasi melebihi batas 2 MB"}
	}
	return string(data), nil
}

// SaveConfig memvalidasi, mem-backup, menulis atomik, lalu reload Nginx bila berjalan
func (m *Manager) SaveConfig(ctx context.Context, content string) (string, error) {
	if err := m.ensureDevFiles(); err != nil {
		return "", err
	}
	if strings.TrimSpace(content) == "" {
		return "", &ValidationError{Msg: "konfigurasi tidak boleh kosong"}
	}
	if len(content) > maxConfigSize {
		return "", &ValidationError{Msg: "konfigurasi melebihi batas 2 MB"}
	}
	// Normalisasi line ending agar file tetap konsisten di Linux
	content = strings.ReplaceAll(content, "\r\n", "\n")

	if err := checkSyntax(content); err != nil {
		return "", err
	}

	m.cfgMu.Lock()
	defer m.cfgMu.Unlock()

	mode := os.FileMode(0o644)
	if fi, err := os.Stat(m.paths.ConfFile); err == nil {
		mode = fi.Mode().Perm()
	}

	// File sementara wajib di folder yang sama agar include relatif ter-resolve saat nginx -t
	dir := filepath.Dir(m.paths.ConfFile)
	tmp := filepath.Join(dir, fmt.Sprintf(".nginx.conf.fikpanel-%d.tmp", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return "", fmt.Errorf("gagal menulis file sementara: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmp)
		}
	}()

	if !m.mock {
		if err := m.runNginxTest(ctx, tmp); err != nil {
			return "", err
		}
	}

	backupPath, err := m.backupCurrent()
	if err != nil {
		return "", err
	}

	if err := os.Rename(tmp, m.paths.ConfFile); err != nil {
		return "", fmt.Errorf("gagal mengganti file konfigurasi: %w", err)
	}
	committed = true

	st, err := m.Status(ctx)
	if err == nil && st.Running {
		if err := m.ServiceAction(ctx, "reload"); err != nil {
			return backupPath, fmt.Errorf("konfigurasi tersimpan tetapi reload gagal: %w", err)
		}
	}
	return backupPath, nil
}

// TestConfigFile menguji file konfigurasi (cek sintaks dasar + nginx -t di Linux)
func (m *Manager) TestConfigFile(ctx context.Context, path string) error {
	if err := m.ensureDevFiles(); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("gagal membaca %s: %w", path, err)
	}
	if err := checkSyntax(string(data)); err != nil {
		return err
	}
	if m.mock {
		return nil
	}
	return m.runNginxTest(ctx, path)
}

// runNginxTest mengeksekusi `nginx -t -c <path>` dan mengembalikan output bila gagal
func (m *Manager) runNginxTest(ctx context.Context, path string) error {
	bin, err := exec.LookPath("nginx")
	if err != nil {
		return &ValidationError{Msg: "binary nginx tidak ditemukan di PATH"}
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, bin, "-t", "-c", path).CombinedOutput()
	if err != nil {
		return &ValidationError{Msg: "uji konfigurasi nginx gagal:\n" + strings.TrimSpace(string(out))}
	}
	return nil
}

// backupCurrent menyalin konfigurasi aktif ke folder backup dan merotasi backup lama
func (m *Manager) backupCurrent() (string, error) {
	src, err := os.ReadFile(m.paths.ConfFile)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("gagal membaca konfigurasi untuk backup: %w", err)
	}
	if err := os.MkdirAll(m.paths.BackupDir, 0o750); err != nil {
		return "", fmt.Errorf("gagal membuat folder backup: %w", err)
	}
	name := fmt.Sprintf("nginx.conf.%s.bak", time.Now().Format("20060102-150405.000"))
	dst := filepath.Join(m.paths.BackupDir, name)
	if err := os.WriteFile(dst, src, 0o640); err != nil {
		return "", fmt.Errorf("gagal menulis backup: %w", err)
	}

	entries, err := os.ReadDir(m.paths.BackupDir)
	if err != nil {
		return dst, nil
	}
	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "nginx.conf.") && strings.HasSuffix(e.Name(), ".bak") {
			backups = append(backups, e.Name())
		}
	}
	// Nama berbasis timestamp sehingga urutan leksikal = urutan waktu
	sort.Strings(backups)
	for i := 0; i < len(backups)-maxBackups; i++ {
		_ = os.Remove(filepath.Join(m.paths.BackupDir, backups[i]))
	}
	return dst, nil
}

// CheckSyntax melakukan validasi struktural: kurung kurawal seimbang dan string tertutup
func CheckSyntax(content string) error {
	return checkSyntax(content)
}

// checkSyntax melakukan validasi struktural: kurung kurawal seimbang dan string tertutup
func checkSyntax(content string) error {
	depth := 0
	line := 1
	var quote byte
	inComment := false
	escaped := false

	for i := 0; i < len(content); i++ {
		c := content[i]
		if c == '\n' {
			line++
			inComment = false
		}
		if inComment {
			continue
		}
		if quote != 0 {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == quote:
				quote = 0
			}
			continue
		}
		switch c {
		case '#':
			inComment = true
		case '"', '\'':
			quote = c
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return &ValidationError{Msg: fmt.Sprintf("kurung kurawal '}' berlebih pada baris %d", line)}
			}
		}
	}
	if quote != 0 {
		return &ValidationError{Msg: "terdapat string dengan tanda kutip yang tidak ditutup"}
	}
	if depth != 0 {
		return &ValidationError{Msg: fmt.Sprintf("terdapat %d blok '{' yang tidak ditutup", depth)}
	}
	return nil
}

// ═══ Optimasi Directive ═══

// Optimization berisi directive performa yang bisa diatur dari panel (unit sesuai tag json)
type Optimization struct {
	WorkerProcesses           string `json:"worker_processes"`
	WorkerConnections         int    `json:"worker_connections"`
	KeepaliveTimeout          int    `json:"keepalive_timeout"`
	Gzip                      bool   `json:"gzip"`
	GzipMinLengthKB           int    `json:"gzip_min_length"`
	GzipCompLevel             int    `json:"gzip_comp_level"`
	ClientMaxBodySizeMB       int    `json:"client_max_body_size"`
	ServerNamesHashBucketSize int    `json:"server_names_hash_bucket_size"`
	ClientHeaderBufferSizeKB  int    `json:"client_header_buffer_size"`
	ClientBodyBufferSizeKB    int    `json:"client_body_buffer_size"`
}

// directiveRe membuat regex directive aktif (bukan komentar) di awal baris
func directiveRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^([ \t]*)` + regexp.QuoteMeta(name) + `[ \t]+([^;#\n]+);`)
}

// readDirective mengambil nilai directive pertama, string kosong jika tidak ada
func readDirective(conf, name string) string {
	mm := directiveRe(name).FindStringSubmatch(conf)
	if len(mm) < 3 {
		return ""
	}
	return strings.TrimSpace(mm[2])
}

// parseSize mengonversi ukuran nginx (512k, 50m, 1g, 1024) ke satuan target (k atau m)
func parseSize(val string, unit byte, fallback int) int {
	val = strings.ToLower(strings.TrimSpace(val))
	if val == "" {
		return fallback
	}
	mult := int64(1)
	switch val[len(val)-1] {
	case 'k':
		mult, val = 1024, val[:len(val)-1]
	case 'm':
		mult, val = 1024*1024, val[:len(val)-1]
	case 'g':
		mult, val = 1024*1024*1024, val[:len(val)-1]
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return fallback
	}
	bytes := n * mult
	if unit == 'm' {
		return int(bytes / (1024 * 1024))
	}
	return int(bytes / 1024)
}

// parseIntFirst mengambil angka dari token pertama (mis. "65s 60" -> 65)
func parseIntFirst(val string, fallback int) int {
	fields := strings.Fields(val)
	if len(fields) == 0 {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSuffix(fields[0], "s"))
	if err != nil {
		return fallback
	}
	return n
}

// ReadOptimization mem-parsing directive performa dari konfigurasi aktif (default nginx bila absen)
func (m *Manager) ReadOptimization() (Optimization, error) {
	conf, err := m.ReadConfig()
	if err != nil {
		return Optimization{}, err
	}
	wp := readDirective(conf, "worker_processes")
	if wp == "" {
		wp = "1"
	}
	gz := strings.ToLower(readDirective(conf, "gzip"))
	return Optimization{
		WorkerProcesses:           wp,
		WorkerConnections:         parseIntFirst(readDirective(conf, "worker_connections"), 512),
		KeepaliveTimeout:          parseIntFirst(readDirective(conf, "keepalive_timeout"), 75),
		Gzip:                      gz == "on",
		GzipMinLengthKB:           parseSize(readDirective(conf, "gzip_min_length"), 'k', 0),
		GzipCompLevel:             parseIntFirst(readDirective(conf, "gzip_comp_level"), 1),
		ClientMaxBodySizeMB:       parseSize(readDirective(conf, "client_max_body_size"), 'm', 1),
		ServerNamesHashBucketSize: parseIntFirst(readDirective(conf, "server_names_hash_bucket_size"), 64),
		ClientHeaderBufferSizeKB:  parseSize(readDirective(conf, "client_header_buffer_size"), 'k', 1),
		ClientBodyBufferSizeKB:    parseSize(readDirective(conf, "client_body_buffer_size"), 'k', 16),
	}, nil
}

// validate memeriksa rentang nilai yang aman untuk setiap directive
func (o Optimization) validate() error {
	wp := strings.TrimSpace(strings.ToLower(o.WorkerProcesses))
	if wp != "auto" {
		n, err := strconv.Atoi(wp)
		if err != nil || n < 1 || n > 1024 {
			return &ValidationError{Msg: "worker_processes harus 'auto' atau angka 1-1024"}
		}
	}
	checks := []struct {
		name     string
		val      int
		min, max int
	}{
		{"worker_connections", o.WorkerConnections, 1, 1048576},
		{"keepalive_timeout", o.KeepaliveTimeout, 0, 3600},
		{"gzip_min_length", o.GzipMinLengthKB, 0, 102400},
		{"gzip_comp_level", o.GzipCompLevel, 1, 9},
		{"client_max_body_size", o.ClientMaxBodySizeMB, 0, 102400},
		{"server_names_hash_bucket_size", o.ServerNamesHashBucketSize, 32, 4096},
		{"client_header_buffer_size", o.ClientHeaderBufferSizeKB, 1, 1024},
		{"client_body_buffer_size", o.ClientBodyBufferSizeKB, 1, 102400},
	}
	for _, c := range checks {
		if c.val < c.min || c.val > c.max {
			return &ValidationError{Msg: fmt.Sprintf("%s harus di antara %d dan %d", c.name, c.min, c.max)}
		}
	}
	// Nginx mewajibkan hash bucket size berupa kelipatan pangkat dua
	if b := o.ServerNamesHashBucketSize; b&(b-1) != 0 {
		return &ValidationError{Msg: "server_names_hash_bucket_size harus pangkat dua (32, 64, 128, ...)"}
	}
	return nil
}

// SaveOptimization menerapkan directive ke konfigurasi lewat pipeline simpan yang aman
func (m *Manager) SaveOptimization(ctx context.Context, o Optimization) (string, error) {
	if err := o.validate(); err != nil {
		return "", err
	}
	conf, err := m.ReadConfig()
	if err != nil {
		return "", err
	}

	gzip := "off"
	if o.Gzip {
		gzip = "on"
	}
	updates := []struct {
		name, value, block string
	}{
		{"worker_processes", strings.ToLower(strings.TrimSpace(o.WorkerProcesses)), ""},
		{"worker_connections", strconv.Itoa(o.WorkerConnections), "events"},
		{"keepalive_timeout", strconv.Itoa(o.KeepaliveTimeout), "http"},
		{"gzip", gzip, "http"},
		{"gzip_min_length", fmt.Sprintf("%dk", o.GzipMinLengthKB), "http"},
		{"gzip_comp_level", strconv.Itoa(o.GzipCompLevel), "http"},
		{"client_max_body_size", fmt.Sprintf("%dm", o.ClientMaxBodySizeMB), "http"},
		{"server_names_hash_bucket_size", strconv.Itoa(o.ServerNamesHashBucketSize), "http"},
		{"client_header_buffer_size", fmt.Sprintf("%dk", o.ClientHeaderBufferSizeKB), "http"},
		{"client_body_buffer_size", fmt.Sprintf("%dk", o.ClientBodyBufferSizeKB), "http"},
	}
	for _, u := range updates {
		conf, err = setDirective(conf, u.name, u.value, u.block)
		if err != nil {
			return "", err
		}
	}
	return m.SaveConfig(ctx, conf)
}

// setDirective mengganti nilai directive pertama atau menyisipkannya ke blok target
func setDirective(conf, name, value, block string) (string, error) {
	re := directiveRe(name)
	if loc := re.FindStringSubmatchIndex(conf); loc != nil {
		indent := conf[loc[2]:loc[3]]
		return conf[:loc[0]] + indent + name + " " + value + ";" + conf[loc[1]:], nil
	}

	line := name + " " + value + ";"
	if block == "" {
		return line + "\n" + conf, nil
	}

	blockRe := regexp.MustCompile(`(?m)^[ \t]*` + block + `\s*\{`)
	loc := blockRe.FindStringIndex(conf)
	if loc == nil {
		if block == "events" {
			return conf + "\nevents {\n    " + line + "\n}\n", nil
		}
		return "", &ValidationError{Msg: fmt.Sprintf("blok '%s' tidak ditemukan di konfigurasi utama", block)}
	}
	return conf[:loc[1]] + "\n    " + line + conf[loc[1]:], nil
}
