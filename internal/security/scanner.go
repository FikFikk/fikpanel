package security

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type SSHConfig struct {
	Port            int  `json:"port"`
	PasswordAuth    bool `json:"password_auth"`
	PermitRootLogin bool `json:"permit_root_login"`
	PubkeyAuth      bool `json:"pubkey_auth"`
	ActiveService   bool `json:"active_service"`
}

type Fail2BanJail struct {
	Name          string   `json:"name"`
	CurrentlyFail int      `json:"currently_failed"`
	TotalFail     int      `json:"total_failed"`
	BannedCount   int      `json:"banned_count"`
	BannedIPs     []string `json:"banned_ips"`
}

type Fail2BanStatus struct {
	Installed bool           `json:"installed"`
	Running   bool           `json:"running"`
	Jails     []Fail2BanJail `json:"jails"`
}

type ScanFinding struct {
	Category string `json:"category"` // 'baseline', 'risk', 'malware', 'vuln'
	Severity string `json:"severity"` // 'critical', 'warning', 'safe'
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	FixHint  string `json:"fix_hint"`
}

type SecurityScanReport struct {
	Score        int           `json:"score"`         // 0 - 100
	ScannedAt    int64         `json:"scanned_at"`    // unix timestamp
	RiskCount    int           `json:"risk_count"`
	MalwareCount int           `json:"malware_count"`
	VulnCount    int           `json:"vuln_count"`
	BaselinePass int           `json:"baseline_pass"`
	BaselineTot  int           `json:"baseline_total"`
	Findings     []ScanFinding `json:"findings"`
}

// ── 1. SSH Hardening ────────────────────────────────────────────────────────

func (m *Manager) GetSSHConfig(ctx context.Context) (*SSHConfig, error) {
	cfg := &SSHConfig{
		Port:            22,
		PasswordAuth:    true,
		PermitRootLogin: true,
		PubkeyAuth:      true,
		ActiveService:   true,
	}

	if runtime.GOOS != "linux" {
		return cfg, nil
	}

	// Cek service ssh / sshd
	out, _ := exec.CommandContext(ctx, "systemctl", "is-active", "ssh").Output()
	if strings.TrimSpace(string(out)) != "active" {
		out, _ = exec.CommandContext(ctx, "systemctl", "is-active", "sshd").Output()
	}
	cfg.ActiveService = strings.TrimSpace(string(out)) == "active"

	data, err := os.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		return cfg, nil
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
			if p, err := strconv.Atoi(val); err == nil && p > 0 {
				cfg.Port = p
			}
		case "passwordauthentication":
			cfg.PasswordAuth = (val == "yes")
		case "permitrootlogin":
			cfg.PermitRootLogin = (val == "yes" || val == "prohibit-password" || val == "without-password")
		case "pubkeyauthentication":
			cfg.PubkeyAuth = (val == "yes")
		}
	}

	return cfg, nil
}

func (m *Manager) UpdateSSHConfig(ctx context.Context, port int, passwordAuth bool, permitRoot bool) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port SSH tidak valid (1-65535)")
	}
	if runtime.GOOS != "linux" {
		return nil
	}

	path := "/etc/ssh/sshd_config"
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("gagal membaca sshd_config: %w", err)
	}

	pwVal := "no"
	if passwordAuth {
		pwVal = "yes"
	}
	rootVal := "no"
	if permitRoot {
		rootVal = "yes"
	}

	lines := strings.Split(string(data), "\n")
	foundPort, foundPw, foundRoot := false, false, false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if strings.HasPrefix(lower, "port ") || strings.HasPrefix(lower, "#port ") {
			lines[i] = fmt.Sprintf("Port %d", port)
			foundPort = true
		} else if strings.HasPrefix(lower, "passwordauthentication ") || strings.HasPrefix(lower, "#passwordauthentication ") {
			lines[i] = fmt.Sprintf("PasswordAuthentication %s", pwVal)
			foundPw = true
		} else if strings.HasPrefix(lower, "permitrootlogin ") || strings.HasPrefix(lower, "#permitrootlogin ") {
			lines[i] = fmt.Sprintf("PermitRootLogin %s", rootVal)
			foundRoot = true
		}
	}

	if !foundPort {
		lines = append(lines, fmt.Sprintf("Port %d", port))
	}
	if !foundPw {
		lines = append(lines, fmt.Sprintf("PasswordAuthentication %s", pwVal))
	}
	if !foundRoot {
		lines = append(lines, fmt.Sprintf("PermitRootLogin %s", rootVal))
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("gagal menulis sshd_config: %w", err)
	}

	// Reload ssh service
	_ = exec.CommandContext(ctx, "systemctl", "reload", "ssh").Run()
	_ = exec.CommandContext(ctx, "systemctl", "reload", "sshd").Run()
	return nil
}

// ── 2. Fail2Ban Intrusion Prevention ────────────────────────────────────────

func (m *Manager) GetFail2BanStatus(ctx context.Context) (*Fail2BanStatus, error) {
	st := &Fail2BanStatus{
		Installed: false,
		Running:   false,
		Jails:     []Fail2BanJail{},
	}

	if runtime.GOOS != "linux" {
		st.Installed = true
		st.Running = true
		st.Jails = []Fail2BanJail{
			{Name: "sshd", CurrentlyFail: 2, TotalFail: 48, BannedCount: 1, BannedIPs: []string{"185.220.101.44"}},
		}
		return st, nil
	}

	if err := exec.CommandContext(ctx, "which", "fail2ban-client").Run(); err != nil {
		return st, nil
	}
	st.Installed = true

	out, err := exec.CommandContext(ctx, "fail2ban-client", "status").Output()
	if err != nil {
		return st, nil
	}
	st.Running = true

	// Parse Jail list
	var jailNames []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "Jail list:") {
			parts := strings.SplitN(line, "Jail list:", 2)
			if len(parts) == 2 {
				rawJails := strings.Split(parts[1], ",")
				for _, j := range rawJails {
					jName := strings.TrimSpace(j)
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
				parts := strings.Split(l, ":")
				if len(parts) == 2 {
					jail.CurrentlyFail, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
				}
			} else if strings.Contains(l, "Total failed:") {
				parts := strings.Split(l, ":")
				if len(parts) == 2 {
					jail.TotalFail, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
				}
			} else if strings.Contains(l, "Currently banned:") {
				parts := strings.Split(l, ":")
				if len(parts) == 2 {
					jail.BannedCount, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
				}
			} else if strings.Contains(l, "Banned IP list:") {
				parts := strings.SplitN(l, "Banned IP list:", 2)
				if len(parts) == 2 {
					ips := strings.Fields(parts[1])
					jail.BannedIPs = append(jail.BannedIPs, ips...)
				}
			}
		}
		st.Jails = append(st.Jails, jail)
	}

	return st, nil
}

func (m *Manager) UnbanIP(ctx context.Context, jail string, ip string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "fail2ban-client", "set", jail, "unbanip", ip)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal melakukan unban IP %s: %s %w", ip, string(out), err)
	}
	return nil
}

// ── 3. Comprehensive Security Scanner (Baseline, Risk, Malware, Vuln) ───────

func (m *Manager) RunSecurityScan(ctx context.Context) (*SecurityScanReport, error) {
	report := &SecurityScanReport{
		Score:     100,
		ScannedAt: time.Now().Unix(),
		Findings:  []ScanFinding{},
	}

	// 1. Audit Firewall Status
	report.BaselineTot++
	fw, err := m.GetStatus(ctx)
	if err == nil && fw != nil && fw.Enabled {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			Category: "baseline",
			Severity: "safe",
			Title:    "UFW Firewall Aktif",
			Detail:   fmt.Sprintf("Firewall menjaga sistem dengan %d aturan port aktif.", len(fw.Rules)),
			FixHint:  "-",
		})
	} else {
		report.Score -= 20
		report.RiskCount++
		report.Findings = append(report.Findings, ScanFinding{
			Category: "risk",
			Severity: "critical",
			Title:    "Firewall UFW Non-Aktif",
			Detail:   "Port jaringan terbuka tanpa perlindungan paket filter.",
			FixHint:  "Aktifkan UFW Firewall pada tab Firewall Guard.",
		})
	}

	// 2. Audit SSH Configuration
	report.BaselineTot++
	sshCfg, _ := m.GetSSHConfig(ctx)
	if sshCfg != nil {
		if sshCfg.Port == 22 && sshCfg.PasswordAuth {
			report.Score -= 15
			report.RiskCount++
			report.Findings = append(report.Findings, ScanFinding{
				Category: "risk",
				Severity: "warning",
				Title:    "SSH Menggunakan Port Default 22 dengan Password",
				Detail:   "Kombinasi port 22 dan PasswordAuthentication rentan terhadap serangan brute-force botnet.",
				FixHint:  "Ubah port SSH atau matikan Password Authentication dan gunakan SSH Key.",
			})
		} else {
			report.BaselinePass++
			report.Findings = append(report.Findings, ScanFinding{
				Category: "baseline",
				Severity: "safe",
				Title:    "Konfigurasi SSH Terlindungi",
				Detail:   fmt.Sprintf("SSH berjalan di port %d.", sshCfg.Port),
				FixHint:  "-",
			})
		}
	}

	// 3. Audit Fail2Ban Intrusion Prevention
	report.BaselineTot++
	f2b, _ := m.GetFail2BanStatus(ctx)
	if f2b != nil && f2b.Running {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			Category: "baseline",
			Severity: "safe",
			Title:    "Fail2Ban Intrusion Prevention Berjalan",
			Detail:   fmt.Sprintf("Memantau %d jail aktif terhadap serangan brute-force.", len(f2b.Jails)),
			FixHint:  "-",
		})
	} else {
		report.Score -= 10
		report.RiskCount++
		report.Findings = append(report.Findings, ScanFinding{
			Category: "baseline",
			Severity: "warning",
			Title:    "Fail2Ban Belum Aktif / Terpasang",
			Detail:   "Tidak ada pemblokiran IP otomatis untuk percobaan login SSH atau Nginx yang gagal berulang kali.",
			FixHint:  "Pasang paket fail2ban untuk memblokir IP penyerang otomatis.",
		})
	}

	// 4. Audit File Izin Kritis (/etc/shadow & /etc/passwd)
	report.BaselineTot++
	if runtime.GOOS == "linux" {
		info, err := os.Stat("/etc/shadow")
		if err == nil {
			mode := info.Mode().Perm()
			if mode&0007 != 0 {
				report.Score -= 25
				report.VulnCount++
				report.Findings = append(report.Findings, ScanFinding{
					Category: "vuln",
					Severity: "critical",
					Title:    "Izin Berkas /etc/shadow Terlalu Terbuka",
					Detail:   fmt.Sprintf("Permission saat ini %04o mengizinkan pembacaan oleh user biasa.", mode),
					FixHint:  "Jalankan chmod 640 /etc/shadow",
				})
			} else {
				report.BaselinePass++
				report.Findings = append(report.Findings, ScanFinding{
					Category: "baseline",
					Severity: "safe",
					Title:    "Hak Akses /etc/shadow Aman",
					Detail:   "File hash kata sandi sistem terkunci rapat dari akses publik.",
					FixHint:  "-",
				})
			}
		}
	} else {
		report.BaselinePass++
	}

	// 5. Audit Webroots Malware / Webshell & Exposed Sensitive Files (.env / .git)
	report.BaselineTot++
	malwareHits := 0
	exposedHits := 0

	if runtime.GOOS == "linux" && dirExists("/var/www") {
		_ = filepath.Walk("/var/www", func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			base := strings.ToLower(filepath.Base(path))

			// Cek file .env yang world-readable
			if base == ".env" && info.Mode().Perm()&0004 != 0 {
				exposedHits++
				report.Findings = append(report.Findings, ScanFinding{
					Category: "vuln",
					Severity: "warning",
					Title:    "Berkas .env Memiliki Izin Baca Publik",
					Detail:   fmt.Sprintf("Lokasi: %s (%04o)", path, info.Mode().Perm()),
					FixHint:  fmt.Sprintf("Jalankan chmod 600 %s", path),
				})
			}

			// Cek indikasi PHP webshell sederhana pada file kecil (< 512KB)
			if ext == ".php" && info.Size() < 512*1024 {
				content, err := os.ReadFile(path)
				if err == nil {
					str := string(content)
					if strings.Contains(str, "eval(base64_decode(") ||
						strings.Contains(str, "eval(gzinflate(") ||
						strings.Contains(str, "passthru($_") ||
						strings.Contains(str, "shell_exec($_GET") {
						malwareHits++
						report.Findings = append(report.Findings, ScanFinding{
							Category: "malware",
							Severity: "critical",
							Title:    "Indikasi Webshell / Backdoor PHP Terdeteksi",
							Detail:   fmt.Sprintf("Pola eksekusi mencurigakan ditemukan di: %s", path),
							FixHint:  "Periksa isi berkas dan hapus jika bukan bagian dari kode resmi.",
						})
					}
				}
			}
			return nil
		})
	}

	report.MalwareCount = malwareHits
	report.VulnCount += exposedHits

	if malwareHits == 0 && exposedHits == 0 {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			Category: "malware",
			Severity: "safe",
			Title:    "Direktori Web Bersih dari Indikasi Webshell",
			Detail:   "Pemindaian heuristik pada /var/www tidak menemukan pola backdoor eval/base64 atau kebocoran .env.",
			FixHint:  "-",
		})
	} else {
		report.Score -= (malwareHits * 25) + (exposedHits * 10)
	}

	if report.Score < 0 {
		report.Score = 0
	}

	return report, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
