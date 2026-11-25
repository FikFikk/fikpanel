package security

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	sshdMainConfigPath = "/etc/ssh/sshd_config"
	sshdDropInDir      = "/etc/ssh/sshd_config.d"
	// Prefiks 01- memastikan aturan FikPanel menang atas 50-cloud-init.conf karena OpenSSH menerapkan first-match-wins
	sshdFikPanelDropIn = "/etc/ssh/sshd_config.d/01-fikpanel.conf"
)

// GetSSHConfig membaca konfigurasi OpenSSH efektif melalui `sshd -T` dan mendeteksi override cloud-init
func (m *Manager) GetSSHConfig(ctx context.Context) (*SSHConfig, error) {
	if runtime.GOOS != "linux" {
		m.devMu.RLock()
		cfg := m.devSSHConfig
		m.devMu.RUnlock()
		keys, _ := m.ListSSHKeys(ctx)
		cfg.AuthorizedKeyCount = len(keys)
		return &cfg, nil
	}

	cfg := &SSHConfig{
		IsLinux:         true,
		Port:            22,
		PasswordAuth:    true,
		PubkeyAuth:      true,
		PermitRootLogin: "prohibit-password",
		MaxAuthTries:    6,
		ConfigSource:    sshdMainConfigPath,
	}

	// Periksa keberadaan biner sshd dan status unit systemd
	if _, err := exec.LookPath("sshd"); err == nil {
		cfg.Installed = true
	} else if _, err := os.Stat("/usr/sbin/sshd"); err == nil {
		cfg.Installed = true
	}

	out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "ssh").Output()
	if strings.TrimSpace(string(out)) != "active" {
		out, _ = exec.CommandContext(ctx, "systemctl", "is-active", "sshd").Output()
	}
	cfg.ActiveService = strings.TrimSpace(string(out)) == "active"

	// Deteksi berkas override drop-in cloud-init pada Ubuntu/Debian modern
	if overrideFile := detectCloudInitSSHOverride(); overrideFile != "" {
		cfg.CloudInitOverride = true
		cfg.CloudInitFile = overrideFile
	}

	// Prioritas pertama: baca konfigurasi efektif langsung dari `sshd -T`
	sshdBin := resolveSSHDBinary()
	if sshdBin != "" {
		if tOut, err := exec.CommandContext(ctx, sshdBin, "-T").Output(); err == nil {
			parseSSHDEffectiveConfig(string(tOut), cfg)
			cfg.ConfigSource = "sshd -T (efektif)"
		} else {
			parseSSHDConfigFile(sshdMainConfigPath, cfg)
		}
	} else {
		parseSSHDConfigFile(sshdMainConfigPath, cfg)
	}

	// Hitung jumlah kunci publik aktif di authorized_keys
	if keys, err := m.ListSSHKeys(ctx); err == nil {
		cfg.AuthorizedKeyCount = len(keys)
	}

	return cfg, nil
}

// resolveSSHDBinary mencari jalur biner sshd pada sistem Linux
func resolveSSHDBinary() string {
	if p, err := exec.LookPath("sshd"); err == nil {
		return p
	}
	for _, candidate := range []string{"/usr/sbin/sshd", "/sbin/sshd"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// parseSSHDEffectiveConfig mem-parsing output key-value dari perintah `sshd -T`
func parseSSHDEffectiveConfig(output string, cfg *SSHConfig) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		val := strings.ToLower(fields[1])
		switch key {
		case "port":
			if p, err := strconv.Atoi(val); err == nil && p > 0 && p <= 65535 {
				cfg.Port = p
			}
		case "passwordauthentication":
			cfg.PasswordAuth = (val == "yes")
		case "pubkeyauthentication":
			cfg.PubkeyAuth = (val == "yes")
		case "permitrootlogin":
			cfg.PermitRootLogin = normalizePermitRoot(val)
		case "maxauthtries":
			if tries, err := strconv.Atoi(val); err == nil && tries > 0 {
				cfg.MaxAuthTries = tries
			}
		}
	}
}

// parseSSHDConfigFile membaca berkas sshd_config secara manual sebagai fallback jika sshd -T gagal
func parseSSHDConfigFile(path string, cfg *SSHConfig) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		val := strings.ToLower(fields[1])
		switch key {
		case "port":
			if p, err := strconv.Atoi(val); err == nil && p > 0 && p <= 65535 {
				cfg.Port = p
			}
		case "passwordauthentication":
			cfg.PasswordAuth = (val == "yes")
		case "pubkeyauthentication":
			cfg.PubkeyAuth = (val == "yes")
		case "permitrootlogin":
			cfg.PermitRootLogin = normalizePermitRoot(val)
		case "maxauthtries":
			if tries, err := strconv.Atoi(val); err == nil && tries > 0 {
				cfg.MaxAuthTries = tries
			}
		}
	}
}

// normalizePermitRoot menyelaraskan variasi nilai PermitRootLogin OpenSSH ke tiga mode baku
func normalizePermitRoot(val string) string {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "yes":
		return "yes"
	case "no":
		return "no"
	case "prohibit-password", "without-password", "forced-commands-only":
		return "prohibit-password"
	default:
		return "prohibit-password"
	}
}

// detectCloudInitSSHOverride mendeteksi apakah ada drop-in selain 01-fikpanel.conf yang mengatur PasswordAuthentication
func detectCloudInitSSHOverride() string {
	entries, err := os.ReadDir(sshdDropInDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") || e.Name() == filepath.Base(sshdFikPanelDropIn) {
			continue
		}
		full := filepath.Join(sshdDropInDir, e.Name())
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		lower := strings.ToLower(string(data))
		if strings.Contains(lower, "passwordauthentication") || strings.Contains(lower, "permitrootlogin") {
			return full
		}
	}
	return ""
}

// UpdateSSHConfig memperbarui konfigurasi OpenSSH secara aman dengan validasi `sshd -t`, auto-rollback, dan lockout guard
func (m *Manager) UpdateSSHConfig(ctx context.Context, req SSHConfigUpdate) error {
	if req.Port < 1 || req.Port > 65535 {
		return fmt.Errorf("port SSH tidak valid (harus 1-65535)")
	}
	if req.MaxAuthTries < 1 || req.MaxAuthTries > 20 {
		if req.MaxAuthTries == 0 {
			req.MaxAuthTries = 6
		} else {
			return fmt.Errorf("MaxAuthTries harus antara 1 sampai 20")
		}
	}
	permitRoot := normalizePermitRoot(req.PermitRootLogin)

	// Proteksi anti-lockout: tolak mematikan autentikasi password dan pubkey secara bersamaan
	if !req.PasswordAuth && !req.PubkeyAuth {
		return fmt.Errorf("minimal salah satu metode autentikasi (Password atau SSH Key) harus aktif agar akses server tidak terkunci")
	}

	// Proteksi anti-lockout: peringatkan jika PasswordAuth dimatikan tetapi belum ada SSH Public Key terpasang
	if !req.PasswordAuth && !req.Force {
		keys, _ := m.ListSSHKeys(ctx)
		if len(keys) == 0 {
			return fmt.Errorf("tidak ada SSH Public Key di authorized_keys. Tambahkan SSH Key terlebih dahulu sebelum menonaktifkan login password agar Anda tidak terkunci dari server")
		}
	}

	if runtime.GOOS != "linux" {
		m.devMu.Lock()
		m.devSSHConfig.Port = req.Port
		m.devSSHConfig.PasswordAuth = req.PasswordAuth
		m.devSSHConfig.PubkeyAuth = req.PubkeyAuth
		m.devSSHConfig.PermitRootLogin = permitRoot
		m.devSSHConfig.MaxAuthTries = req.MaxAuthTries
		m.devMu.Unlock()
		return nil
	}

	oldCfg, _ := m.GetSSHConfig(ctx)
	pwVal := boolYesNo(req.PasswordAuth)
	keyVal := boolYesNo(req.PubkeyAuth)

	// Periksa apakah sshd_config memuat Include /etc/ssh/sshd_config.d/*.conf
	mainRaw, err := os.ReadFile(sshdMainConfigPath)
	if err != nil {
		return fmt.Errorf("gagal membaca %s: %w", sshdMainConfigPath, err)
	}

	useDropIn := strings.Contains(string(mainRaw), "/etc/ssh/sshd_config.d/*.conf") && dirExists(sshdDropInDir)

	var rollback func()
	if useDropIn {
		// Simpan cadangan drop-in sebelumnya jika ada
		prevDropIn, prevErr := os.ReadFile(sshdFikPanelDropIn)
		dropInContent := fmt.Sprintf(
			"# Dikelola otomatis oleh FikPanel Security Center\nPort %d\nPasswordAuthentication %s\nPubkeyAuthentication %s\nPermitRootLogin %s\nMaxAuthTries %d\n",
			req.Port, pwVal, keyVal, permitRoot, req.MaxAuthTries,
		)
		if err := os.WriteFile(sshdFikPanelDropIn, []byte(dropInContent), 0600); err != nil {
			return fmt.Errorf("gagal menulis konfigurasi drop-in SSH: %w", err)
		}
		rollback = func() {
			if prevErr == nil {
				_ = os.WriteFile(sshdFikPanelDropIn, prevDropIn, 0600)
			} else {
				_ = os.Remove(sshdFikPanelDropIn)
			}
		}
	} else {
		updated := rewriteSSHDConfigLines(string(mainRaw), req.Port, pwVal, keyVal, permitRoot, req.MaxAuthTries)
		if err := os.WriteFile(sshdMainConfigPath, []byte(updated), 0644); err != nil {
			return fmt.Errorf("gagal menulis %s: %w", sshdMainConfigPath, err)
		}
		rollback = func() {
			_ = os.WriteFile(sshdMainConfigPath, mainRaw, 0644)
		}
	}

	// Validasi sintaksis wajib dengan `sshd -t` sebelum memuat ulang layanan
	if sshdBin := resolveSSHDBinary(); sshdBin != "" {
		if out, err := exec.CommandContext(ctx, sshdBin, "-t").CombinedOutput(); err != nil {
			rollback()
			return fmt.Errorf("validasi sintaksis sshd -t gagal (perubahan dibatalkan otomatis): %s", strings.TrimSpace(string(out)))
		}
	}

	// Jika port berubah dan UFW aktif, otomatis izinkan port SSH baru terlebih dahulu
	if oldCfg != nil && oldCfg.Port != req.Port {
		if fw, err := m.GetStatus(ctx); err == nil && fw.Enabled {
			_ = m.AddPortRule(ctx, AddFirewallRuleInput{
				Port:      strconv.Itoa(req.Port),
				Protocol:  "tcp",
				Action:    "allow",
				Direction: "in",
				Remarks:   "FikPanel SSH Port",
			})
		}
	}

	// Muat ulang layanan SSH (sesi koneksi aktif tetap terjaga)
	if err := exec.CommandContext(ctx, "systemctl", "reload", "ssh").Run(); err != nil {
		if err2 := exec.CommandContext(ctx, "systemctl", "reload", "sshd").Run(); err2 != nil {
			// Fallback restart bila reload tidak didukung oleh unit distro
			_ = exec.CommandContext(ctx, "systemctl", "restart", "ssh").Run()
			_ = exec.CommandContext(ctx, "systemctl", "restart", "sshd").Run()
		}
	}

	return nil
}

// rewriteSSHDConfigLines memperbarui direktif SSH langsung pada berkas sshd_config utama
func rewriteSSHDConfigLines(content string, port int, pwVal, keyVal, permitRoot string, maxTries int) string {
	lines := strings.Split(content, "\n")
	found := map[string]bool{}
	directives := map[string]string{
		"port":                   fmt.Sprintf("Port %d", port),
		"passwordauthentication": fmt.Sprintf("PasswordAuthentication %s", pwVal),
		"pubkeyauthentication":   fmt.Sprintf("PubkeyAuthentication %s", keyVal),
		"permitrootlogin":        fmt.Sprintf("PermitRootLogin %s", permitRoot),
		"maxauthtries":           fmt.Sprintf("MaxAuthTries %d", maxTries),
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		uncommented := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		fields := strings.Fields(uncommented)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		if repl, ok := directives[key]; ok && !found[key] {
			lines[i] = repl
			found[key] = true
		}
	}

	for key, repl := range directives {
		if !found[key] {
			lines = append(lines, repl)
		}
	}
	return strings.Join(lines, "\n")
}

func boolYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// RestartSSHService memulai ulang daemon SSH/SSHD pada host Linux
func (m *Manager) RestartSSHService(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if err := exec.CommandContext(ctx, "systemctl", "restart", "ssh").Run(); err != nil {
		if err2 := exec.CommandContext(ctx, "systemctl", "restart", "sshd").Run(); err2 != nil {
			return fmt.Errorf("gagal memulai ulang layanan SSH: %w", err2)
		}
	}
	return nil
}

// Pola ekstraksi log autentikasi OpenSSH untuk event sukses dan gagal
var (
	sshAcceptedRe = regexp.MustCompile(`Accepted\s+(\S+)\s+for\s+(\S+)\s+from\s+([0-9a-fA-F:.]+)\s+port\s+(\d+)`)
	sshFailedRe   = regexp.MustCompile(`Failed\s+(\S+)\s+for\s+(?:invalid user\s+)?(\S+)\s+from\s+([0-9a-fA-F:.]+)\s+port\s+(\d+)`)
	sshInvalidRe  = regexp.MustCompile(`Invalid user\s+(\S+)\s+from\s+([0-9a-fA-F:.]+)(?:\s+port\s+(\d+))?`)
	unixPrefixRe  = regexp.MustCompile(`^(\d{9,11})(?:\.\d+)?\s+`)
)

// GetSSHStats mem-parsing riwayat autentikasi SSH aktual 7 hari terakhir dari journalctl atau berkas auth.log
func (m *Manager) GetSSHStats(ctx context.Context) (*SSHStats, error) {
	cfg, _ := m.GetSSHConfig(ctx)
	if cfg == nil {
		cfg = &SSHConfig{Port: 22, PermitRootLogin: "prohibit-password"}
	}

	now := time.Now()
	days := buildEmptySevenDays(now)

	stats := &SSHStats{
		Config:     *cfg,
		LogSource:  "tidak tersedia (non-Linux)",
		DailyStats: days,
		RecentLogs: []SSHLog{},
	}

	if runtime.GOOS != "linux" {
		return stats, nil
	}

	// Prioritas 1: baca dari journalctl 7 hari terakhir dengan timestamp unix
	rawLogs := ""
	if out, err := exec.CommandContext(ctx, "journalctl", "-u", "ssh", "-u", "sshd", "--since", "7 days ago", "--no-pager", "-o", "short-unix").Output(); err == nil && len(bytes.TrimSpace(out)) > 0 {
		rawLogs = string(out)
		stats.LogSource = "journalctl (ssh/sshd)"
	} else {
		// Prioritas 2: baca dari /var/log/auth.log atau /var/log/secure
		for _, path := range []string{"/var/log/auth.log", "/var/log/secure"} {
			if data, err := readLastBytes(path, 512*1024); err == nil && len(data) > 0 {
				rawLogs = string(data)
				stats.LogSource = path
				break
			}
		}
	}

	if rawLogs == "" {
		stats.LogSource = "tidak ada catatan log SSH"
		return stats, nil
	}

	parseSSHAuthLogs(rawLogs, now, stats)
	return stats, nil
}

// buildEmptySevenDays menyiapkan 7 bucket kalender mundur dari hari ini
func buildEmptySevenDays(now time.Time) []SSHDayStat {
	days := make([]SSHDayStat, 7)
	for i := 0; i < 7; i++ {
		d := now.AddDate(0, 0, -(6 - i))
		days[i] = SSHDayStat{
			Date:  d.Format("2006-01-02"),
			Label: d.Format("02 Jan"),
		}
	}
	return days
}

// parseSSHAuthLogs mengekstrak statistik harian, ringkasan hari ini/kemarin, dan 50 log terbaru dari teks log SSH
func parseSSHAuthLogs(raw string, now time.Time, stats *SSHStats) {
	dayIndex := make(map[string]int, len(stats.DailyStats))
	for i, d := range stats.DailyStats {
		dayIndex[d.Date] = i
	}
	todayStr := now.Format("2006-01-02")
	yesterdayStr := now.AddDate(0, 0, -1).Format("2006-01-02")
	cutoff := now.AddDate(0, 0, -7).Unix()

	allEvents := make([]SSHLog, 0, 64)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	// Tingkatkan buffer scanner untuk baris log panjang
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "Accepted ") && !strings.Contains(line, "Failed ") && !strings.Contains(line, "Invalid user ") {
			continue
		}

		ts := extractLogTimestamp(line, now)
		if ts > 0 && ts < cutoff {
			continue
		}

		var ev *SSHLog
		if m := sshAcceptedRe.FindStringSubmatch(line); len(m) == 5 {
			port, _ := strconv.Atoi(m[4])
			ev = &SSHLog{
				Timestamp: ts,
				User:      m[2],
				IP:        m[3],
				Port:      port,
				Status:    "success",
				Method:    m[1],
				Message:   fmt.Sprintf("Accepted %s for %s", m[1], m[2]),
			}
		} else if m := sshFailedRe.FindStringSubmatch(line); len(m) == 5 {
			port, _ := strconv.Atoi(m[4])
			ev = &SSHLog{
				Timestamp: ts,
				User:      m[2],
				IP:        m[3],
				Port:      port,
				Status:    "failed",
				Method:    m[1],
				Message:   fmt.Sprintf("Failed %s for %s", m[1], m[2]),
			}
		} else if m := sshInvalidRe.FindStringSubmatch(line); len(m) >= 3 {
			port := 0
			if len(m) >= 4 {
				port, _ = strconv.Atoi(m[3])
			}
			ev = &SSHLog{
				Timestamp: ts,
				User:      m[1],
				IP:        m[2],
				Port:      port,
				Status:    "failed",
				Method:    "invalid_user",
				Message:   fmt.Sprintf("Invalid user %s", m[1]),
			}
		}

		if ev == nil {
			continue
		}
		if ev.Timestamp == 0 {
			ev.Timestamp = now.Unix()
		}

		dateKey := time.Unix(ev.Timestamp, 0).Format("2006-01-02")
		if ev.Status == "success" {
			stats.SuccessfulLogins++
			if dateKey == todayStr {
				stats.TodaySuccess++
			} else if dateKey == yesterdayStr {
				stats.YesterdaySuccess++
			}
			if idx, ok := dayIndex[dateKey]; ok {
				stats.DailyStats[idx].Success++
			}
		} else {
			stats.FailedLogins++
			if dateKey == todayStr {
				stats.TodayFail++
			} else if dateKey == yesterdayStr {
				stats.YesterdayFail++
			}
			if idx, ok := dayIndex[dateKey]; ok {
				stats.DailyStats[idx].Failed++
			}
		}

		allEvents = append(allEvents, *ev)
	}

	// Ambil maksimal 50 log terbaru dengan urutan terbaru di atas
	limit := 50
	if len(allEvents) > limit {
		allEvents = allEvents[len(allEvents)-limit:]
	}
	for i, j := 0, len(allEvents)-1; i < j; i, j = i+1, j-1 {
		allEvents[i], allEvents[j] = allEvents[j], allEvents[i]
	}
	stats.RecentLogs = allEvents
}

// extractLogTimestamp mem-parsing timestamp dari format journalctl short-unix, ISO-8601, atau syslog klasik
func extractLogTimestamp(line string, ref time.Time) int64 {
	if m := unixPrefixRe.FindStringSubmatch(line); len(m) == 2 {
		if sec, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			return sec
		}
	}
	fields := strings.Fields(line)
	if len(fields) >= 1 {
		if t, err := time.Parse(time.RFC3339, fields[0]); err == nil {
			return t.Unix()
		}
	}
	if len(fields) >= 3 {
		syslogTime := fmt.Sprintf("%d %s %s %s", ref.Year(), fields[0], fields[1], fields[2])
		if t, err := time.ParseInLocation("2006 Jan 2 15:04:05", syslogTime, ref.Location()); err == nil {
			// Tangani pergantian tahun saat log berasal dari Desember di awal Januari
			if t.After(ref.Add(24 * time.Hour)) {
				t = t.AddDate(-1, 0, 0)
			}
			return t.Unix()
		}
	}
	return 0
}

// readLastBytes membaca bagian akhir berkas log hingga batas maxBytes agar hemat memori
func readLastBytes(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
	}
	if _, err := f.Seek(start, 0); err != nil {
		return nil, err
	}
	buf := make([]byte, size-start)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return nil, err
	}
	return buf[:n], nil
}

// resolveAuthorizedKeysPath menentukan lokasi berkas authorized_keys milik user saat ini (atau /root/.ssh)
func resolveAuthorizedKeysPath() (string, error) {
	if runtime.GOOS == "linux" {
		if os.Geteuid() == 0 {
			return "/root/.ssh/authorized_keys", nil
		}
	}
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		home, err2 := os.UserHomeDir()
		if err2 != nil {
			return "", fmt.Errorf("gagal menentukan direktori home: %w", err2)
		}
		return filepath.Join(home, ".ssh", "authorized_keys"), nil
	}
	return filepath.Join(u.HomeDir, ".ssh", "authorized_keys"), nil
}

// ListSSHKeys membaca dan mem-parsing daftar kunci publik sah di authorized_keys
func (m *Manager) ListSSHKeys(ctx context.Context) ([]SSHAuthorizedKey, error) {
	path, err := resolveAuthorizedKeysPath()
	if err != nil {
		return []SSHAuthorizedKey{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []SSHAuthorizedKey{}, nil
		}
		return nil, fmt.Errorf("gagal membaca authorized_keys: %w", err)
	}

	keys := make([]SSHAuthorizedKey, 0)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			continue
		}
		fp := ssh.FingerprintSHA256(pub)
		idSum := sha256.Sum256(pub.Marshal())
		preview := line
		if len(preview) > 54 {
			preview = preview[:28] + "…" + preview[len(preview)-18:]
		}
		keys = append(keys, SSHAuthorizedKey{
			ID:          hex.EncodeToString(idSum[:8]),
			Type:        pub.Type(),
			Fingerprint: fp,
			Comment:     strings.TrimSpace(comment),
			Preview:     preview,
		})
	}
	return keys, nil
}

// AddSSHKey memvalidasi dan menambahkan SSH Public Key baru ke ~/.ssh/authorized_keys dengan izin 0600
func (m *Manager) AddSSHKey(ctx context.Context, rawKey string) (*SSHAuthorizedKey, error) {
	trimmed := strings.TrimSpace(rawKey)
	if trimmed == "" || strings.ContainsAny(trimmed, "\r\n") {
		return nil, fmt.Errorf("masukkan satu baris SSH Public Key yang valid")
	}

	pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(trimmed))
	if err != nil {
		return nil, fmt.Errorf("format SSH Public Key tidak valid (contoh: ssh-ed25519 AAAA... user@host)")
	}

	fp := ssh.FingerprintSHA256(pub)
	existing, err := m.ListSSHKeys(ctx)
	if err != nil {
		return nil, err
	}
	for _, k := range existing {
		if k.Fingerprint == fp {
			return nil, fmt.Errorf("SSH Public Key ini sudah terdaftar (%s)", fp)
		}
	}

	path, err := resolveAuthorizedKeysPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("gagal membuat direktori .ssh: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka authorized_keys: %w", err)
	}
	defer f.Close()

	// Pastikan baris baru ditambahkan rapi
	if fi, err := f.Stat(); err == nil && fi.Size() > 0 {
		if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 && raw[len(raw)-1] != '\n' {
			_, _ = f.WriteString("\n")
		}
	}
	if _, err := f.WriteString(trimmed + "\n"); err != nil {
		return nil, fmt.Errorf("gagal menulis SSH Key: %w", err)
	}

	idSum := sha256.Sum256(pub.Marshal())
	return &SSHAuthorizedKey{
		ID:          hex.EncodeToString(idSum[:8]),
		Type:        pub.Type(),
		Fingerprint: fp,
		Comment:     strings.TrimSpace(comment),
		Preview:     trimmed,
	}, nil
}

// RemoveSSHKey menghapus SSH Public Key tertentu berdasarkan ID hash dari ~/.ssh/authorized_keys
func (m *Manager) RemoveSSHKey(ctx context.Context, keyID string) error {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return fmt.Errorf("ID kunci SSH tidak boleh kosong")
	}

	path, err := resolveAuthorizedKeysPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("gagal membaca authorized_keys: %w", err)
	}

	var kept []string
	removed := false
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			if pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(trimmed)); err == nil {
				idSum := sha256.Sum256(pub.Marshal())
				if hex.EncodeToString(idSum[:8]) == keyID {
					removed = true
					continue
				}
			}
		}
		kept = append(kept, line)
	}

	if !removed {
		return fmt.Errorf("SSH Public Key tidak ditemukan")
	}

	out := strings.Join(kept, "\n")
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if err := os.WriteFile(path, []byte(out), 0600); err != nil {
		return fmt.Errorf("gagal memperbarui authorized_keys: %w", err)
	}
	return nil
}

// GetSystemAccounts membaca daftar akun pengguna Linux beserta status kunci sebenarnya dari /etc/shadow / passwd -S
func (m *Manager) GetSystemAccounts(ctx context.Context) ([]SystemAccount, error) {
	if runtime.GOOS != "linux" {
		return []SystemAccount{}, nil
	}

	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil, fmt.Errorf("gagal membaca /etc/passwd: %w", err)
	}

	shadowLocked := readShadowLockMap()
	accounts := make([]SystemAccount, 0)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		uname := parts[0]
		uid, _ := strconv.Atoi(parts[2])
		gid, _ := strconv.Atoi(parts[3])
		homeDir := parts[5]
		shell := parts[6]

		// Filter hanya akun manusia (UID >= 1000 kecuali nobody), root, atau service web utama
		if !(uid == 0 || (uid >= 1000 && uname != "nobody") || uname == "www-data" || uname == "nginx") {
			continue
		}

		noLogin := strings.Contains(shell, "nologin") || strings.Contains(shell, "/false")
		pwLocked, knownShadow := shadowLocked[uname]
		if !knownShadow {
			// Fallback cek melalui `passwd -S`
			if out, err := exec.CommandContext(ctx, "passwd", "-S", uname).Output(); err == nil {
				f := strings.Fields(string(out))
				if len(f) >= 2 && (f[1] == "L" || f[1] == "LK") {
					pwLocked = true
				}
			}
		}

		locked := pwLocked || noLogin
		reason := "active"
		if pwLocked {
			reason = "passwd_locked"
		} else if noLogin {
			reason = "nologin_shell"
		}

		accounts = append(accounts, SystemAccount{
			Username:    uname,
			UID:         uid,
			GID:         gid,
			Shell:       shell,
			HomeDir:     homeDir,
			Locked:      locked,
			LockReason:  reason,
			IsProtected: uid == 0 || uname == "root",
		})
	}
	return accounts, nil
}

// readShadowLockMap memetakan username -> status terkunci dari /etc/shadow (prefix '!' atau '*')
func readShadowLockMap() map[string]bool {
	res := make(map[string]bool)
	data, err := os.ReadFile("/etc/shadow")
	if err != nil {
		return res
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ":")
		if len(parts) < 2 {
			continue
		}
		hash := parts[1]
		res[parts[0]] = strings.HasPrefix(hash, "!") || hash == "*"
	}
	return res
}

var validLinuxUserRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

// ToggleAccountLock mengunci atau membuka kunci password akun Linux dengan proteksi akun root
func (m *Manager) ToggleAccountLock(ctx context.Context, username string, lock bool) error {
	username = strings.TrimSpace(username)
	if !validLinuxUserRe.MatchString(username) {
		return fmt.Errorf("nama akun sistem tidak valid")
	}
	if username == "root" && lock {
		return fmt.Errorf("akun root dilindungi dan tidak boleh dikunci demi mencegah kehilangan akses administratif")
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("manajemen kunci akun sistem hanya tersedia pada server Linux")
	}

	flag := "-u"
	if lock {
		flag = "-l"
	}
	cmd := exec.CommandContext(ctx, "passwd", flag, username)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gagal mengubah status akun %s: %s (%w)", username, strings.TrimSpace(string(out)), err)
	}
	return nil
}
