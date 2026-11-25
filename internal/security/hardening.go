package security

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/store"
)

const sysctlDropInPath = "/etc/sysctl.d/99-fikpanel-hardening.conf"

// sysctlSpec mendefinisikan parameter pengerasan kernel yang aman dikelola dari panel
type sysctlSpec struct {
	Key         string
	Title       string
	Description string
	SecureVal   string
	InsecureVal string
}

var managedSysctlSpecs = []sysctlSpec{
	{
		Key:         "net.ipv4.icmp_echo_ignore_all",
		Title:       "Blokir Ping ICMP Echo",
		Description: "Cegah server membalas paket ping ICMP dari jaringan publik untuk mengurangi eksposur pemindaian.",
		SecureVal:   "1",
		InsecureVal: "0",
	},
	{
		Key:         "net.ipv4.tcp_syncookies",
		Title:       "Proteksi TCP SYN Flood Cookies",
		Description: "Aktifkan SYN cookies kernel untuk menahan serangan banjir koneksi setengah-terbuka (SYN Flood).",
		SecureVal:   "1",
		InsecureVal: "0",
	},
	{
		Key:         "net.ipv4.conf.all.rp_filter",
		Title:       "Reverse Path Filter (Anti IP Spoofing)",
		Description: "Validasi rute sumber setiap paket masuk untuk menolak paket dengan alamat IP palsu.",
		SecureVal:   "1",
		InsecureVal: "0",
	},
	{
		Key:         "fs.suid_dumpable",
		Title:       "Nonaktifkan Core Dump SUID",
		Description: "Mencegah proses istimewa (SUID) menulis core dump ke disk yang berisiko membocorkan data memori.",
		SecureVal:   "0",
		InsecureVal: "1",
	},
	{
		Key:         "net.ipv4.conf.all.accept_redirects",
		Title:       "Tolak ICMP Redirect",
		Description: "Mencegah manipulasi tabel routing kernel melalui paket ICMP redirect berbahaya.",
		SecureVal:   "0",
		InsecureVal: "1",
	},
	{
		Key:         "kernel.dmesg_restrict",
		Title:       "Batasi Akses Kernel Log (dmesg)",
		Description: "Membatasi pembacaan ring buffer kernel (dmesg) hanya untuk pengguna dengan hak istimewa root.",
		SecureVal:   "1",
		InsecureVal: "0",
	},
}

var managedCompilers = []string{"/usr/bin/gcc", "/usr/bin/g++", "/usr/bin/make", "/usr/bin/as"}

// readSysctlValue membaca nilai live satu parameter kernel langsung dari /proc/sys
func (m *Manager) readSysctlValue(key string) string {
	if runtime.GOOS != "linux" {
		m.devMu.RLock()
		on := m.devSysctl[key]
		m.devMu.RUnlock()
		for _, spec := range managedSysctlSpecs {
			if spec.Key == key {
				if on {
					return spec.SecureVal
				}
				return spec.InsecureVal
			}
		}
		return "0"
	}
	procPath := "/proc/sys/" + strings.ReplaceAll(key, ".", "/")
	data, err := os.ReadFile(procPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// readSysctlBool memeriksa apakah nilai kernel saat ini sama dengan nilai aman yang diharapkan
func (m *Manager) readSysctlBool(key, secureVal string) bool {
	return m.readSysctlValue(key) == secureVal
}

// readPersistedSysctlMap membaca isi berkas /etc/sysctl.d/99-fikpanel-hardening.conf
func readPersistedSysctlMap() map[string]string {
	res := make(map[string]string)
	data, err := os.ReadFile(sysctlDropInPath)
	if err != nil {
		return res
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			res[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return res
}

// GetHardeningStatus membaca status pengerasan kernel aktual (/proc/sys), izin compiler, dan baseline FIM
func (m *Manager) GetHardeningStatus(ctx context.Context) (*HardeningStatus, error) {
	persisted := readPersistedSysctlMap()
	params := make([]SysctlParam, 0, len(managedSysctlSpecs))
	icmpBlocked := false

	for _, spec := range managedSysctlSpecs {
		cur := m.readSysctlValue(spec.Key)
		enabled := cur == spec.SecureVal
		if spec.Key == "net.ipv4.icmp_echo_ignore_all" {
			icmpBlocked = enabled
		}
		_, isPersisted := persisted[spec.Key]
		params = append(params, SysctlParam{
			Key:              spec.Key,
			Title:            spec.Title,
			Description:      spec.Description,
			Enabled:          enabled,
			CurrentValue:     cur,
			RecommendedValue: spec.SecureVal,
			Persisted:        isPersisted,
		})
	}

	compilersFound, compilerBlocked := m.inspectCompilerAccess()

	fimCount := 0
	var fimLast int64
	if m.store != nil {
		if list, err := m.store.ListFIMBaselines(ctx); err == nil && len(list) > 0 {
			fimCount = len(list)
			for _, item := range list {
				if item.UpdatedAt > fimLast {
					fimLast = item.UpdatedAt
				}
			}
		}
	}

	return &HardeningStatus{
		IsLinux:          runtime.GOOS == "linux",
		CompilerBlocked:  compilerBlocked,
		CompilersFound:   compilersFound,
		ICMPBlocked:      icmpBlocked,
		SysctlParams:     params,
		FIMEnabled:       fimCount > 0,
		FIMBaselineCount: fimCount,
		FIMLastUpdated:   fimLast,
	}, nil
}

// inspectCompilerAccess memeriksa biner compiler yang terpasang dan apakah izinnya sudah dibatasi hanya untuk root (0700)
func (m *Manager) inspectCompilerAccess() ([]string, bool) {
	if runtime.GOOS != "linux" {
		m.devMu.RLock()
		blocked := m.devCompilerBlocked
		m.devMu.RUnlock()
		return []string{}, blocked
	}

	found := make([]string, 0, len(managedCompilers))
	allRestricted := true
	for _, c := range managedCompilers {
		fi, err := os.Stat(c)
		if err != nil {
			continue
		}
		found = append(found, filepath.Base(c))
		if fi.Mode().Perm()&0077 != 0 {
			allRestricted = false
		}
	}
	if len(found) == 0 {
		return found, false
	}
	return found, allRestricted
}

// SetSysctlParam menerapkan satu parameter kernel secara langsung (`sysctl -w`) dan menyimpannya permanen di /etc/sysctl.d/
func (m *Manager) SetSysctlParam(ctx context.Context, key string, enable bool) error {
	var matched *sysctlSpec
	for i := range managedSysctlSpecs {
		if managedSysctlSpecs[i].Key == key {
			matched = &managedSysctlSpecs[i]
			break
		}
	}
	if matched == nil {
		return fmt.Errorf("parameter kernel tidak dikenali atau tidak diizinkan: %s", key)
	}

	if runtime.GOOS != "linux" {
		m.devMu.Lock()
		m.devSysctl[key] = enable
		m.devMu.Unlock()
		return nil
	}

	val := matched.InsecureVal
	if enable {
		val = matched.SecureVal
	}

	cmd := exec.CommandContext(ctx, "sysctl", "-w", fmt.Sprintf("%s=%s", matched.Key, val))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gagal menerapkan sysctl %s: %s (%w)", matched.Key, strings.TrimSpace(string(out)), err)
	}

	// Simpan permanen ke /etc/sysctl.d/99-fikpanel-hardening.conf agar bertahan setelah reboot
	persisted := readPersistedSysctlMap()
	persisted[matched.Key] = val

	var buf strings.Builder
	buf.WriteString("# Konfigurasi kernel hardening dikelola oleh FikPanel Security Center\n")
	for _, spec := range managedSysctlSpecs {
		if v, ok := persisted[spec.Key]; ok {
			buf.WriteString(fmt.Sprintf("%s = %s\n", spec.Key, v))
		}
	}
	_ = os.MkdirAll(filepath.Dir(sysctlDropInPath), 0755)
	if err := os.WriteFile(sysctlDropInPath, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("sysctl diterapkan sementara, namun gagal menulis %s: %w", sysctlDropInPath, err)
	}
	return nil
}

// ToggleICMP mengatur pemblokiran ping ICMP secara persisten melalui SetSysctlParam
func (m *Manager) ToggleICMP(ctx context.Context, block bool) error {
	return m.SetSysctlParam(ctx, "net.ipv4.icmp_echo_ignore_all", block)
}

// ToggleCompilerAccess mengatur izin eksekusi biner compiler (gcc, g++, make, as) ke 0700 atau 0755
func (m *Manager) ToggleCompilerAccess(ctx context.Context, block bool) error {
	if runtime.GOOS != "linux" {
		m.devMu.Lock()
		m.devCompilerBlocked = block
		m.devMu.Unlock()
		return nil
	}

	perm := os.FileMode(0755)
	if block {
		perm = 0700
	}
	changed := 0
	for _, c := range managedCompilers {
		if _, err := os.Stat(c); err != nil {
			continue
		}
		if err := os.Chmod(c, perm); err != nil {
			return fmt.Errorf("gagal mengubah izin %s: %w", c, err)
		}
		changed++
	}
	if changed == 0 {
		return fmt.Errorf("tidak ditemukan biner compiler (gcc/g++/make/as) pada sistem ini")
	}
	return nil
}

// ── Fail2Ban Intrusion Prevention ───────────────────────────────────────────

var validJailNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)

// GetFail2BanStatus memeriksa status instalasi, daemon aktif, dan seluruh jail Fail2Ban aktual
func (m *Manager) GetFail2BanStatus(ctx context.Context) (*Fail2BanStatus, error) {
	st := &Fail2BanStatus{
		IsLinux:   runtime.GOOS == "linux",
		Installed: false,
		Running:   false,
		Jails:     []Fail2BanJail{},
	}

	if runtime.GOOS != "linux" {
		m.devMu.RLock()
		st.Running = m.devFail2BanRunning
		m.devMu.RUnlock()
		return st, nil
	}

	if _, err := exec.LookPath("fail2ban-client"); err != nil {
		return st, nil
	}
	st.Installed = true

	out, err := exec.CommandContext(ctx, "fail2ban-client", "status").Output()
	if err != nil {
		return st, nil
	}
	st.Running = true

	var jailNames []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "Jail list:") {
			parts := strings.SplitN(line, "Jail list:", 2)
			if len(parts) == 2 {
				for _, raw := range strings.Split(parts[1], ",") {
					jName := strings.TrimSpace(raw)
					if jName != "" {
						jailNames = append(jailNames, jName)
					}
				}
			}
		}
	}

	for _, jName := range jailNames {
		jOut, err := exec.CommandContext(ctx, "fail2ban-client", "status", jName).Output()
		if err != nil {
			continue
		}
		jail := Fail2BanJail{Name: jName, BannedIPs: []string{}}
		jScan := bufio.NewScanner(bytes.NewReader(jOut))
		for jScan.Scan() {
			l := strings.TrimSpace(jScan.Text())
			if strings.Contains(l, "Currently failed:") {
				parts := strings.SplitN(l, ":", 2)
				if len(parts) == 2 {
					jail.CurrentlyFail, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
				}
			} else if strings.Contains(l, "Total failed:") {
				parts := strings.SplitN(l, ":", 2)
				if len(parts) == 2 {
					jail.TotalFail, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
				}
			} else if strings.Contains(l, "Currently banned:") {
				parts := strings.SplitN(l, ":", 2)
				if len(parts) == 2 {
					jail.BannedCount, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
				}
			} else if strings.Contains(l, "Banned IP list:") {
				parts := strings.SplitN(l, "Banned IP list:", 2)
				if len(parts) == 2 {
					jail.BannedIPs = append(jail.BannedIPs, strings.Fields(parts[1])...)
				}
			}
		}
		st.Jails = append(st.Jails, jail)
	}

	return st, nil
}

// ControlFail2BanService menjalankan start, stop, atau restart pada layanan systemd Fail2Ban
func (m *Manager) ControlFail2BanService(ctx context.Context, action string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "start", "stop", "restart":
	default:
		return fmt.Errorf("aksi Fail2Ban tidak valid: %s", action)
	}

	if runtime.GOOS != "linux" {
		m.devMu.Lock()
		m.devFail2BanRunning = (action != "stop")
		m.devMu.Unlock()
		return nil
	}

	if _, err := exec.LookPath("fail2ban-client"); err != nil {
		return fmt.Errorf("paket fail2ban belum terpasang di server ini")
	}

	out, err := exec.CommandContext(ctx, "systemctl", action, "fail2ban").CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menjalankan %s fail2ban: %s (%w)", action, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// BanIP memblokir alamat IP secara manual pada jail Fail2Ban tertentu
func (m *Manager) BanIP(ctx context.Context, jail, rawIP string) error {
	if !validJailNameRe.MatchString(jail) {
		return fmt.Errorf("nama jail tidak valid")
	}
	normIP, err := ValidateIPOrCIDR(rawIP)
	if err != nil {
		return err
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("aksi Fail2Ban hanya tersedia pada server Linux")
	}
	out, err := exec.CommandContext(ctx, "fail2ban-client", "set", jail, "banip", normIP).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal melakukan ban IP %s pada jail %s: %s (%w)", normIP, jail, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// UnbanIP mencabut pemblokiran IP dari jail Fail2Ban tertentu
func (m *Manager) UnbanIP(ctx context.Context, jail, rawIP string) error {
	if !validJailNameRe.MatchString(jail) {
		return fmt.Errorf("nama jail tidak valid")
	}
	normIP, err := ValidateIPOrCIDR(rawIP)
	if err != nil {
		return err
	}
	if runtime.GOOS != "linux" {
		return nil
	}
	out, err := exec.CommandContext(ctx, "fail2ban-client", "set", jail, "unbanip", normIP).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal melakukan unban IP %s pada jail %s: %s (%w)", normIP, jail, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ── File Integrity Monitor (FIM / Anti-Tamper Real) ─────────────────────────

// collectFIMTargetFiles mengumpulkan daftar berkas kritis sistem & konfigurasi layanan yang dipantau integritasnya
func collectFIMTargetFiles() []string {
	if runtime.GOOS != "linux" {
		// Pada lingkungan pengembangan lokal, pantau berkas konfigurasi proyek yang nyata agar fitur FIM tetap dapat diuji
		candidates := []string{"go.mod", "Makefile", "SECURITY.md"}
		found := make([]string, 0, len(candidates))
		for _, p := range candidates {
			if abs, err := filepath.Abs(p); err == nil {
				if fi, err := os.Stat(abs); err == nil && !fi.IsDir() {
					found = append(found, abs)
				}
			}
		}
		return found
	}

	staticFiles := []string{
		"/etc/passwd",
		"/etc/group",
		"/etc/sudoers",
		"/etc/hosts",
		"/etc/crontab",
		"/etc/ssh/sshd_config",
		"/etc/nginx/nginx.conf",
		"/etc/sysctl.d/99-fikpanel-hardening.conf",
	}
	globPatterns := []string{
		"/etc/ssh/sshd_config.d/*.conf",
		"/etc/nginx/conf.d/*.conf",
		"/etc/nginx/sites-enabled/*",
	}

	seen := make(map[string]bool)
	targets := make([]string, 0, 32)
	addIfFile := func(p string) {
		if seen[p] {
			return
		}
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() || fi.Size() > 5*1024*1024 {
			return
		}
		seen[p] = true
		targets = append(targets, p)
	}

	for _, f := range staticFiles {
		addIfFile(f)
	}
	for _, pat := range globPatterns {
		if matches, err := filepath.Glob(pat); err == nil {
			for _, m := range matches {
				addIfFile(m)
			}
		}
	}
	return targets
}

// hashFileSHA256 menghitung digest SHA-256 suatu berkas secara streaming
func hashFileSHA256(path string) (sum string, size int64, mode string, modTime int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, "", 0, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", 0, "", 0, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", 0, "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), fi.Size(), fmt.Sprintf("%04o", fi.Mode().Perm()), fi.ModTime().Unix(), nil
}

// CreateFIMBaseline membangun dan menyimpan snapshot hash SHA-256 berkas kritis sebagai baseline FIM
func (m *Manager) CreateFIMBaseline(ctx context.Context) (*FIMCheckResult, error) {
	if m.store == nil {
		return nil, fmt.Errorf("database penyimpanan FIM tidak tersedia")
	}

	targets := collectFIMTargetFiles()
	now := time.Now().Unix()
	entries := make([]store.FIMBaselineEntry, 0, len(targets))
	items := make([]FIMEntryStatus, 0, len(targets))

	for _, path := range targets {
		sum, size, mode, mtime, err := hashFileSHA256(path)
		if err != nil {
			continue
		}
		entries = append(entries, store.FIMBaselineEntry{
			Path:      path,
			SHA256:    sum,
			Size:      size,
			Mode:      mode,
			ModTime:   mtime,
			UpdatedAt: now,
		})
		items = append(items, FIMEntryStatus{
			Path:         path,
			Status:       "ok",
			ExpectedHash: sum,
			ActualHash:   sum,
			Size:         size,
			Mode:         mode,
			ModTime:      mtime,
		})
	}

	if err := m.store.ReplaceFIMBaselines(ctx, entries); err != nil {
		return nil, err
	}

	return &FIMCheckResult{
		Enabled:        len(entries) > 0,
		TotalFiles:     len(entries),
		OKCount:        len(entries),
		ModifiedCount:  0,
		MissingCount:   0,
		LastBaselineAt: now,
		CheckedAt:      now,
		Items:          items,
	}, nil
}

// CheckFIMIntegrity memverifikasi seluruh berkas di baseline FIM terhadap kondisi disk saat ini
func (m *Manager) CheckFIMIntegrity(ctx context.Context) (*FIMCheckResult, error) {
	now := time.Now().Unix()
	res := &FIMCheckResult{
		Enabled:   false,
		CheckedAt: now,
		Items:     []FIMEntryStatus{},
	}
	if m.store == nil {
		return res, nil
	}

	baselines, err := m.store.ListFIMBaselines(ctx)
	if err != nil {
		return nil, err
	}
	if len(baselines) == 0 {
		return res, nil
	}

	res.Enabled = true
	res.TotalFiles = len(baselines)
	for _, b := range baselines {
		if b.UpdatedAt > res.LastBaselineAt {
			res.LastBaselineAt = b.UpdatedAt
		}
		sum, size, mode, mtime, err := hashFileSHA256(b.Path)
		if err != nil {
			res.MissingCount++
			res.Items = append(res.Items, FIMEntryStatus{
				Path:         b.Path,
				Status:       "missing",
				ExpectedHash: b.SHA256,
				Size:         b.Size,
				Mode:         b.Mode,
				ModTime:      b.ModTime,
			})
			continue
		}

		status := "ok"
		if sum != b.SHA256 || mode != b.Mode {
			status = "modified"
			res.ModifiedCount++
		} else {
			res.OKCount++
		}
		res.Items = append(res.Items, FIMEntryStatus{
			Path:         b.Path,
			Status:       status,
			ExpectedHash: b.SHA256,
			ActualHash:   sum,
			Size:         size,
			Mode:         mode,
			ModTime:      mtime,
		})
	}
	return res, nil
}

// DisableFIM menonaktifkan File Integrity Monitor dengan menghapus baseline tersimpan
func (m *Manager) DisableFIM(ctx context.Context) error {
	if m.store == nil {
		return nil
	}
	return m.store.ClearFIMBaselines(ctx)
}
