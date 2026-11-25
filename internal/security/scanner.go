package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/store"
)

// RunSecurityScan menjalankan audit keamanan terpadu (12 titik kontrol nyata) dan menyimpannya ke SQLite
func (m *Manager) RunSecurityScan(ctx context.Context, opts ScanOptions) (*SecurityScanReport, error) {
	report := &SecurityScanReport{
		Score:     100,
		ScannedAt: time.Now().Unix(),
		Findings:  make([]ScanFinding, 0, 16),
	}

	// 1. Audit UFW Firewall & Default Policy
	report.BaselineTotal++
	fw, _ := m.GetStatus(ctx)
	if fw != nil && fw.Enabled {
		if strings.EqualFold(fw.DefaultIncoming, "allow") {
			report.Score -= 10
			report.RiskCount++
			report.Findings = append(report.Findings, ScanFinding{
				ID:        "ufw_default_policy",
				Category:  "risk",
				Severity:  "warning",
				Title:     "Kebijakan Default Incoming UFW Masih ALLOW",
				Detail:    "UFW aktif tetapi default incoming disetel ALLOW sehingga seluruh port baru otomatis terbuka.",
				FixHint:   "Ubah default incoming policy menjadi DENY pada tab Firewall.",
				TargetTab: "firewall",
				ActionKey: "set_ufw_default_deny",
				Fixable:   true,
			})
		} else {
			report.BaselinePass++
			report.Findings = append(report.Findings, ScanFinding{
				ID:        "ufw_active",
				Category:  "baseline",
				Severity:  "safe",
				Title:     "Firewall UFW Aktif & Terlindungi",
				Detail:    fmt.Sprintf("UFW aktif dengan %d aturan dan kebijakan default incoming %s.", len(fw.Rules), strings.ToUpper(fw.DefaultIncoming)),
				FixHint:   "-",
				TargetTab: "firewall",
			})
		}
	} else {
		report.Score -= 15
		report.RiskCount++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "ufw_inactive",
			Category:  "risk",
			Severity:  "critical",
			Title:     "Firewall UFW Belum Aktif",
			Detail:    "Host berjalan tanpa filter paket UFW; port layanan rentan dipindai langsung dari internet.",
			FixHint:   "Aktifkan UFW Firewall di tab Firewall (port SSH dan panel akan diamankan otomatis).",
			TargetTab: "firewall",
			ActionKey: "enable_ufw",
			Fixable:   true,
		})
	}

	// 2. Audit Port Database Terekspos Publik (MySQL 3306, PostgreSQL 5432, Redis 6379, MongoDB 27017)
	report.BaselineTotal++
	exposedDB := m.findPublicDatabasePorts(ctx, fw)
	if len(exposedDB) > 0 {
		report.Score -= 15
		report.RiskCount++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "db_public_exposed",
			Category:  "risk",
			Severity:  "critical",
			Title:     "Port Database Terbuka ke Jaringan Publik",
			Detail:    fmt.Sprintf("Port database terdeteksi listen pada alamat publik (0.0.0.0/::): %s.", strings.Join(exposedDB, ", ")),
			FixHint:   "Bind layanan database ke 127.0.0.1 atau batasi Source IP khusus di Firewall UFW.",
			TargetTab: "firewall",
		})
	} else {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "db_public_safe",
			Category:  "baseline",
			Severity:  "safe",
			Title:     "Port Database Terisolasi dari Publik",
			Detail:    "Tidak ditemukan port database utama (3306/5432/6379/27017) yang terekspos bebas ke internet.",
			FixHint:   "-",
			TargetTab: "firewall",
		})
	}

	// 3. Audit Konfigurasi SSH (Port, Password Auth, Root Login)
	report.BaselineTotal++
	sshCfg, _ := m.GetSSHConfig(ctx)
	if sshCfg != nil {
		if sshCfg.PermitRootLogin == "yes" && sshCfg.PasswordAuth {
			report.Score -= 15
			report.RiskCount++
			report.Findings = append(report.Findings, ScanFinding{
				ID:        "ssh_root_password",
				Category:  "risk",
				Severity:  "critical",
				Title:     "Login SSH Root via Password Diizinkan",
				Detail:    fmt.Sprintf("SSH pada port %d mengizinkan root login dengan kata sandi — target utama botnet brute-force.", sshCfg.Port),
				FixHint:   "Tambahkan SSH Public Key lalu ubah PermitRootLogin ke prohibit-password atau no di tab SSH.",
				TargetTab: "ssh",
			})
		} else if sshCfg.Port == 22 && sshCfg.PasswordAuth {
			report.Score -= 8
			report.RiskCount++
			report.Findings = append(report.Findings, ScanFinding{
				ID:        "ssh_default_port_pw",
				Category:  "risk",
				Severity:  "warning",
				Title:     "SSH Port Default 22 Masih Menggunakan Password",
				Detail:    "Port 22 dengan PasswordAuthentication aktif meningkatkan volume serangan brute-force otomatis.",
				FixHint:   "Pasang SSH Key dan matikan PasswordAuthentication, atau ubah ke port SSH kustom di tab SSH.",
				TargetTab: "ssh",
			})
		} else {
			report.BaselinePass++
			report.Findings = append(report.Findings, ScanFinding{
				ID:        "ssh_hardened",
				Category:  "baseline",
				Severity:  "safe",
				Title:     "Konfigurasi SSH Sudah Diperkeras",
				Detail:    fmt.Sprintf("SSH berjalan di port %d (PermitRootLogin=%s, PasswordAuth=%v).", sshCfg.Port, sshCfg.PermitRootLogin, sshCfg.PasswordAuth),
				FixHint:   "-",
				TargetTab: "ssh",
			})
		}
	}

	// 4. Audit Fail2Ban Intrusion Prevention
	report.BaselineTotal++
	f2b, _ := m.GetFail2BanStatus(ctx)
	if f2b != nil && f2b.Running {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "fail2ban_active",
			Category:  "baseline",
			Severity:  "safe",
			Title:     "Fail2Ban Intrusion Prevention Aktif",
			Detail:    fmt.Sprintf("Daemon Fail2Ban berjalan aktif dengan %d jail pemantau.", len(f2b.Jails)),
			FixHint:   "-",
			TargetTab: "hardening",
		})
	} else {
		report.Score -= 10
		report.RiskCount++
		detail := "Paket Fail2Ban belum terpasang; percobaan brute-force SSH tidak diblokir otomatis."
		if f2b != nil && f2b.Installed {
			detail = "Fail2Ban sudah terpasang tetapi layanannya sedang berhenti (stopped)."
		}
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "fail2ban_inactive",
			Category:  "risk",
			Severity:  "warning",
			Title:     "Fail2Ban Tidak Aktif",
			Detail:    detail,
			FixHint:   "Nyalakan layanan Fail2Ban di tab Hardening & Scanner atau pasang via apt install fail2ban.",
			TargetTab: "hardening",
			ActionKey: "start_fail2ban",
			Fixable:   f2b != nil && f2b.Installed,
		})
	}

	// 5. Audit Enkripsi HTTPS/TLS Panel
	report.BaselineTotal++
	if opts.IsHTTPS {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "panel_tls_ok",
			Category:  "baseline",
			Severity:  "safe",
			Title:     "Enkripsi HTTPS/TLS Panel Aktif",
			Detail:    "Seluruh lalu lintas kredensial dan manajemen FikPanel dienkripsi menggunakan TLS.",
			FixHint:   "-",
			TargetTab: "settings",
		})
	} else {
		report.Score -= 10
		report.RiskCount++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "panel_tls_off",
			Category:  "risk",
			Severity:  "warning",
			Title:     "Akses Panel Belum Menggunakan HTTPS/TLS",
			Detail:    "Koneksi ke panel masih berjalan lewat HTTP biasa tanpa enkripsi transport.",
			FixHint:   "Aktifkan sertifikat TLS panel (ACME/Custom/Self-Signed) pada menu Pengaturan → Keamanan.",
			TargetTab: "settings",
		})
	}

	// 6. Audit Autentikasi Multi-Faktor (2FA TOTP / Passkey)
	report.BaselineTotal++
	if opts.TOTPEnabled || opts.PasskeyCount > 0 {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "panel_mfa_ok",
			Category:  "baseline",
			Severity:  "safe",
			Title:     "Autentikasi Multi-Faktor (2FA / Passkey) Aktif",
			Detail:    fmt.Sprintf("Akun administrator dilindungi 2FA TOTP (%v) dan %d Passkey terdaftar.", opts.TOTPEnabled, opts.PasskeyCount),
			FixHint:   "-",
			TargetTab: "settings",
		})
	} else {
		report.Score -= 10
		report.RiskCount++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "panel_mfa_off",
			Category:  "risk",
			Severity:  "warning",
			Title:     "2FA TOTP & Passkey Belum Diaktifkan",
			Detail:    "Akun panel saat ini hanya mengandalkan kata sandi tanpa autentikasi dua faktor.",
			FixHint:   "Aktifkan Authenticator 2FA atau daftarkan Passkey di menu Pengaturan → Keamanan.",
			TargetTab: "settings",
		})
	}

	// 7. Audit Proteksi Brute Force Panel
	report.BaselineTotal++
	if opts.BruteForceEnabled {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "bruteforce_ok",
			Category:  "baseline",
			Severity:  "safe",
			Title:     "Proteksi Brute Force Panel Aktif",
			Detail:    "Pembatasan percobaan gagal per IP/Username serta mekanisme auto-ban aktif melindungi endpoint login.",
			FixHint:   "-",
			TargetTab: "bruteforce",
		})
	} else {
		report.Score -= 10
		report.RiskCount++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "bruteforce_off",
			Category:  "risk",
			Severity:  "warning",
			Title:     "Proteksi Brute Force Panel Non-Aktif",
			Detail:    "Percobaan tebak kata sandi berulang tidak diblokir otomatis oleh sistem.",
			FixHint:   "Aktifkan kembali Brute Force Protection di tab Brute Force & IP.",
			TargetTab: "bruteforce",
			ActionKey: "enable_bruteforce",
			Fixable:   true,
		})
	}

	// 8. Audit Kernel Hardening (TCP SYN Cookies & Reverse Path Filter)
	report.BaselineTotal++
	synOK := m.readSysctlBool("net.ipv4.tcp_syncookies", "1")
	rpOK := m.readSysctlBool("net.ipv4.conf.all.rp_filter", "1")
	if synOK && rpOK {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "kernel_sysctl_ok",
			Category:  "baseline",
			Severity:  "safe",
			Title:     "Parameter Kernel Anti-SYN Flood & Spoofing Aktif",
			Detail:    "net.ipv4.tcp_syncookies=1 dan net.ipv4.conf.all.rp_filter=1 aktif pada kernel.",
			FixHint:   "-",
			TargetTab: "hardening",
		})
	} else {
		report.Score -= 8
		report.VulnCount++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "kernel_sysctl_weak",
			Category:  "vuln",
			Severity:  "warning",
			Title:     "Proteksi Kernel Jaringan Belum Optimal",
			Detail:    fmt.Sprintf("Status saat ini: tcp_syncookies=%v, rp_filter=%v.", synOK, rpOK),
			FixHint:   "Aktifkan SYN Flood Cookies dan Reverse Path Filter di tab Hardening & Scanner.",
			TargetTab: "hardening",
			ActionKey: "apply_kernel_baseline",
			Fixable:   true,
		})
	}

	// 9. Audit File Integrity Monitor (FIM)
	report.BaselineTotal++
	fimCheck, _ := m.CheckFIMIntegrity(ctx)
	if fimCheck != nil && fimCheck.Enabled {
		if fimCheck.ModifiedCount > 0 || fimCheck.MissingCount > 0 {
			report.Score -= 15
			report.VulnCount += fimCheck.ModifiedCount + fimCheck.MissingCount
			report.Findings = append(report.Findings, ScanFinding{
				ID:        "fim_drift",
				Category:  "vuln",
				Severity:  "critical",
				Title:     "Perubahan Tak Terduga pada Berkas Kritis (FIM)",
				Detail:    fmt.Sprintf("Terdeteksi %d berkas berubah dan %d berkas hilang dibandingkan baseline SHA-256.", fimCheck.ModifiedCount, fimCheck.MissingCount),
				FixHint:   "Periksa daftar berkas yang berubah pada panel File Integrity Monitor di tab Hardening.",
				TargetTab: "hardening",
			})
		} else {
			report.BaselinePass++
			report.Findings = append(report.Findings, ScanFinding{
				ID:        "fim_ok",
				Category:  "baseline",
				Severity:  "safe",
				Title:     "Integritas Berkas Kritis (FIM) Valid",
				Detail:    fmt.Sprintf("Seluruh %d berkas sistem/konfigurasi cocok dengan checksum SHA-256 baseline.", fimCheck.TotalFiles),
				FixHint:   "-",
				TargetTab: "hardening",
			})
		}
	} else {
		report.Score -= 5
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "fim_uninitialized",
			Category:  "baseline",
			Severity:  "warning",
			Title:     "Baseline File Integrity Monitor Belum Dibuat",
			Detail:    "Belum ada snapshot SHA-256 berkas sistem untuk mendeteksi modifikasi diam-diam.",
			FixHint:   "Buat baseline FIM dengan 1 klik di tab Hardening & Scanner.",
			TargetTab: "hardening",
			ActionKey: "create_fim_baseline",
			Fixable:   true,
		})
	}

	// 10. Audit Izin Berkas Kritis Sistem (/etc/shadow & /etc/ssh/sshd_config)
	report.BaselineTotal++
	if runtime.GOOS == "linux" {
		shadowBad := false
		if fi, err := os.Stat("/etc/shadow"); err == nil && fi.Mode().Perm()&0007 != 0 {
			shadowBad = true
			report.Score -= 20
			report.VulnCount++
			report.Findings = append(report.Findings, ScanFinding{
				ID:        "shadow_perm",
				Category:  "vuln",
				Severity:  "critical",
				Title:     "Izin /etc/shadow Terlalu Terbuka",
				Detail:    fmt.Sprintf("Permission saat ini %04o dapat dibaca user non-privileged.", fi.Mode().Perm()),
				FixHint:   "Jalankan chmod 640 /etc/shadow",
				TargetTab: "hardening",
			})
		}
		if !shadowBad {
			report.BaselinePass++
			report.Findings = append(report.Findings, ScanFinding{
				ID:        "shadow_ok",
				Category:  "baseline",
				Severity:  "safe",
				Title:     "Hak Akses Berkas Sistem Kritis Aman",
				Detail:    "Berkas /etc/shadow terkunci rapat dari pembacaan publik.",
				FixHint:   "-",
				TargetTab: "hardening",
			})
		}
	} else {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "shadow_dev",
			Category:  "baseline",
			Severity:  "safe",
			Title:     "Audit Permission OS (Mode Non-Linux)",
			Detail:    "Pemeriksaan izin /etc/shadow otomatis aktif penuh saat berjalan di server Linux.",
			FixHint:   "-",
			TargetTab: "hardening",
		})
	}

	// 11. Audit Webroots (Seluruh root_path situs di DB + /var/www) — .env terbuka, .git terekspos, dan pola webshell PHP
	report.BaselineTotal++
	malwareHits, exposedHits, webFindings := scanWebroots(opts.SiteRoots)
	report.MalwareCount = malwareHits
	report.VulnCount += exposedHits
	if len(webFindings) > 0 {
		report.Findings = append(report.Findings, webFindings...)
		report.Score -= (malwareHits * 25) + (exposedHits * 8)
	} else {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "webroot_clean",
			Category:  "malware",
			Severity:  "safe",
			Title:     "Direktori Web Bersih dari Webshell & Kebocoran Berkas",
			Detail:    "Pemindaian heuristik pada webroot terdaftar tidak menemukan pola backdoor PHP, .env world-readable, maupun .git publik.",
			FixHint:   "-",
			TargetTab: "hardening",
		})
	}

	// 12. Audit Sertifikat SSL Situs yang Hampir Kedaluwarsa
	report.BaselineTotal++
	if opts.ExpiringSSLCount > 0 {
		report.Score -= 5
		report.VulnCount += opts.ExpiringSSLCount
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "ssl_expiring",
			Category:  "vuln",
			Severity:  "warning",
			Title:     "Sertifikat SSL Situs Hampir/Sudah Kedaluwarsa",
			Detail:    fmt.Sprintf("Terdapat %d situs dengan sertifikat SSL kedaluwarsa atau kurang dari 14 hari lagi.", opts.ExpiringSSLCount),
			FixHint:   "Perbarui sertifikat SSL pada halaman Website.",
			TargetTab: "overview",
		})
	} else {
		report.BaselinePass++
		report.Findings = append(report.Findings, ScanFinding{
			ID:        "ssl_healthy",
			Category:  "baseline",
			Severity:  "safe",
			Title:     "Status Sertifikat SSL Situs Sehat",
			Detail:    "Tidak ada sertifikat SSL situs yang kedaluwarsa atau kritis dalam 14 hari ke depan.",
			FixHint:   "-",
			TargetTab: "overview",
		})
	}

	if report.Score < 0 {
		report.Score = 0
	}
	report.Rating = rateSecurityScore(report.Score)

	// Persist laporan hasil pemindaian ke SQLite
	if m.store != nil {
		if rawFindings, err := json.Marshal(report.Findings); err == nil {
			if id, err := m.store.SaveSecurityScan(ctx, store.SecurityScanRecord{
				ScannedAt:     report.ScannedAt,
				Score:         report.Score,
				Rating:        report.Rating,
				RiskCount:     report.RiskCount,
				MalwareCount:  report.MalwareCount,
				VulnCount:     report.VulnCount,
				BaselinePass:  report.BaselinePass,
				BaselineTotal: report.BaselineTotal,
				FindingsJSON:  string(rawFindings),
			}); err == nil {
				report.ID = id
			}
		}
	}

	return report, nil
}

// GetLatestScanReport membaca laporan hasil audit terakhir dari SQLite
func (m *Manager) GetLatestScanReport(ctx context.Context) *SecurityScanReport {
	if m.store == nil {
		return nil
	}
	rec, err := m.store.GetLatestSecurityScan(ctx)
	if err != nil || rec == nil {
		return nil
	}
	findings := make([]ScanFinding, 0)
	_ = json.Unmarshal([]byte(rec.FindingsJSON), &findings)
	return &SecurityScanReport{
		ID:            rec.ID,
		Score:         rec.Score,
		Rating:        rec.Rating,
		ScannedAt:     rec.ScannedAt,
		RiskCount:     rec.RiskCount,
		MalwareCount:  rec.MalwareCount,
		VulnCount:     rec.VulnCount,
		BaselinePass:  rec.BaselinePass,
		BaselineTotal: rec.BaselineTotal,
		Findings:      findings,
	}
}

// rateSecurityScore mengonversi angka skor 0-100 menjadi kategori status keamanan
func rateSecurityScore(score int) string {
	switch {
	case score >= 80:
		return "Aman"
	case score >= 55:
		return "Perlu Perhatian"
	default:
		return "Berisiko"
	}
}

// findPublicDatabasePorts mendeteksi port database utama yang listen di 0.0.0.0/:: tanpa tertutup firewall
func (m *Manager) findPublicDatabasePorts(ctx context.Context, fw *FirewallStatus) []string {
	if runtime.GOOS != "linux" {
		return nil
	}
	dbPorts := map[string]string{
		"3306":  "MySQL (3306)",
		"5432":  "PostgreSQL (5432)",
		"6379":  "Redis (6379)",
		"27017": "MongoDB (27017)",
	}
	listening := m.DetectListeningPorts(ctx)
	exposed := make([]string, 0)
	seen := make(map[string]bool)

	for _, lp := range listening {
		label, isDB := dbPorts[lp.Port]
		if !isDB || !lp.ExposedPublic || seen[lp.Port] {
			continue
		}
		// Jika UFW aktif & default incoming deny, pastikan memang ada rule ALLOW 0.0.0.0/0 untuk port DB ini
		if fw != nil && fw.Enabled && strings.EqualFold(fw.DefaultIncoming, "deny") {
			openAnywhere := false
			for _, r := range fw.Rules {
				if r.Action == "ALLOW" && r.Direction == "Inbound" && r.Port == lp.Port && (r.From == "0.0.0.0/0" || r.From == "::/0") {
					openAnywhere = true
					break
				}
			}
			if !openAnywhere {
				continue
			}
		}
		seen[lp.Port] = true
		exposed = append(exposed, label)
	}
	return exposed
}

// Pola heuristik backdoor / webshell PHP berisiko tinggi
var webshellSignatures = []string{
	"eval(base64_decode(",
	"eval(gzinflate(",
	"eval(str_rot13(",
	"passthru($_",
	"shell_exec($_GET",
	"shell_exec($_POST",
	"system($_GET",
	"system($_POST",
	"assert($_POST",
}

// scanWebroots memindai seluruh direktori web terdaftar terhadap webshell PHP, .env terbuka, dan .git terekspos
func scanWebroots(siteRoots []string) (malwareHits, exposedHits int, findings []ScanFinding) {
	roots := make([]string, 0, len(siteRoots)+1)
	seenRoot := make(map[string]bool)
	for _, r := range siteRoots {
		clean := filepath.Clean(strings.TrimSpace(r))
		if clean != "" && clean != "." && !seenRoot[clean] && dirExists(clean) {
			seenRoot[clean] = true
			roots = append(roots, clean)
		}
	}
	if runtime.GOOS == "linux" && !seenRoot["/var/www"] && dirExists("/var/www") {
		roots = append(roots, "/var/www")
	}

	const maxFindingsPerKind = 15
	for _, root := range roots {
		// Cek cepat .git/config di root situs (berisiko terekspos publik bila tidak diblokir Nginx)
		gitCfg := filepath.Join(root, ".git", "config")
		if fi, err := os.Stat(gitCfg); err == nil && !fi.IsDir() {
			exposedHits++
			if len(findings) < maxFindingsPerKind*2 {
				findings = append(findings, ScanFinding{
					ID:        "exposed_git_" + strconv.Itoa(exposedHits),
					Category:  "vuln",
					Severity:  "warning",
					Title:     "Direktori .git Ditemukan di Webroot",
					Detail:    fmt.Sprintf("Lokasi: %s — pastikan blok Nginx menolak akses '/.git' agar kredensial repositori tidak bocor.", gitCfg),
					FixHint:   "Tambahkan blok 'location ~ /\\.git { deny all; }' pada konfigurasi vhost Nginx.",
					TargetTab: "hardening",
				})
			}
		}

		scannedFiles := 0
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				name := strings.ToLower(info.Name())
				// Lewati folder dependensi besar agar pemindaian cepat dan minim false positive
				if name == "vendor" || name == "node_modules" || name == ".git" || name == "storage" || name == "cache" {
					return filepath.SkipDir
				}
				return nil
			}
			scannedFiles++
			if scannedFiles > 8000 {
				return filepath.SkipAll
			}

			base := strings.ToLower(info.Name())
			ext := strings.ToLower(filepath.Ext(path))

			// Cek berkas .env dengan izin baca publik (o+r) pada Linux
			if runtime.GOOS == "linux" && base == ".env" && info.Mode().Perm()&0004 != 0 {
				exposedHits++
				if exposedHits <= maxFindingsPerKind {
					findings = append(findings, ScanFinding{
						ID:        "env_world_readable_" + strconv.Itoa(exposedHits),
						Category:  "vuln",
						Severity:  "warning",
						Title:     "Berkas .env Memiliki Izin Baca Publik",
						Detail:    fmt.Sprintf("Lokasi: %s (mode %04o) dapat dibaca oleh user lokal lain.", path, info.Mode().Perm()),
						FixHint:   fmt.Sprintf("Jalankan chmod 600 %s", path),
						TargetTab: "hardening",
					})
				}
			}

			// Pemindaian heuristik webshell pada berkas PHP < 512 KB
			if ext == ".php" && info.Size() < 512*1024 {
				content, err := os.ReadFile(path)
				if err == nil {
					str := string(content)
					for _, sig := range webshellSignatures {
						if strings.Contains(str, sig) {
							malwareHits++
							if malwareHits <= maxFindingsPerKind {
								findings = append(findings, ScanFinding{
									ID:        "php_webshell_" + strconv.Itoa(malwareHits),
									Category:  "malware",
									Severity:  "critical",
									Title:     "Indikasi Webshell / Backdoor PHP Terdeteksi",
									Detail:    fmt.Sprintf("Pola '%s' ditemukan pada berkas: %s", sig, path),
									FixHint:   "Audit isi berkas tersebut dan hapus segera jika bukan bagian dari kode sah.",
									TargetTab: "hardening",
								})
							}
							break
						}
					}
				}
			}
			return nil
		})
	}
	return malwareHits, exposedHits, findings
}

// GetProtectionOverview menyusun status aktual 6 pilar perlindungan, login terakhir, dan laporan audit terbaru
func (m *Manager) GetProtectionOverview(ctx context.Context, clientIP string, wafEnabled bool, wafSitesCount int) (*ProtectionOverview, error) {
	fwStatus, _ := m.GetStatus(ctx)
	fwEnabled := fwStatus != nil && fwStatus.Enabled
	fwInstalled := fwStatus != nil && fwStatus.Installed

	sshStats, _ := m.GetSSHStats(ctx)
	sshCfg := SSHConfig{Port: 22, PermitRootLogin: "prohibit-password"}
	if sshStats != nil {
		sshCfg = sshStats.Config
	}

	f2b, _ := m.GetFail2BanStatus(ctx)
	f2bInstalled := f2b != nil && f2b.Installed
	f2bRunning := f2b != nil && f2b.Running
	f2bBannedTotal := 0
	if f2b != nil {
		for _, j := range f2b.Jails {
			f2bBannedTotal += j.BannedCount
		}
	}

	bfCfg := m.GetBruteForceConfig(ctx)
	blList, _ := m.GetBlacklistIPs(ctx)
	hardening, _ := m.GetHardeningStatus(ctx)
	fimEnabled := hardening != nil && hardening.FIMEnabled

	// Ringkasan port terbuka aktual (tanpa hardcoded!)
	fwSummary := "Firewall UFW tidak aktif"
	if !fwInstalled && runtime.GOOS == "linux" {
		fwSummary = "Paket UFW belum terpasang di server"
	} else if fwEnabled && fwStatus != nil {
		allowPorts := make([]string, 0, 8)
		seenP := make(map[string]bool)
		for _, r := range fwStatus.Rules {
			if r.Action == "ALLOW" && r.Direction == "Inbound" && !seenP[r.Port] {
				seenP[r.Port] = true
				allowPorts = append(allowPorts, r.Port)
			}
		}
		if len(allowPorts) > 0 {
			if len(allowPorts) > 6 {
				fwSummary = fmt.Sprintf("%d aturan aktif · Port terbuka: %s (+%d)", len(fwStatus.Rules), strings.Join(allowPorts[:6], ", "), len(allowPorts)-6)
			} else {
				fwSummary = fmt.Sprintf("%d aturan aktif · Port terbuka: %s", len(fwStatus.Rules), strings.Join(allowPorts, ", "))
			}
		} else {
			fwSummary = fmt.Sprintf("Aktif (Default incoming: %s)", strings.ToUpper(fwStatus.DefaultIncoming))
		}
	}

	sshSummary := fmt.Sprintf("Port %d · Root: %s · Password: %s · %d Key",
		sshCfg.Port, sshCfg.PermitRootLogin, boolTextID(sshCfg.PasswordAuth), sshCfg.AuthorizedKeyCount)

	wafSummary := "Nginx WAF tidak aktif"
	if wafEnabled {
		wafSummary = fmt.Sprintf("Aktif melindungi %d situs terdaftar pada Nginx Layer-7", wafSitesCount)
	}

	f2bSummary := "Paket Fail2Ban belum terpasang"
	if f2bInstalled && !f2bRunning {
		f2bSummary = "Terpasang namun layanan sedang berhenti"
	} else if f2bRunning {
		f2bSummary = fmt.Sprintf("%d jail aktif · %d IP sedang diblokir", len(f2b.Jails), f2bBannedTotal)
	}

	bfSummary := "Proteksi brute force login panel dinonaktifkan"
	if bfCfg.Enabled {
		bfSummary = fmt.Sprintf("Maks %dx gagal IP / %dx user · %d IP di blacklist aktif", bfCfg.IPMaxRetries, bfCfg.UsernameMaxRetries, len(blList))
	}

	fimSummary := "Belum ada baseline SHA-256 berkas sistem"
	if fimEnabled && hardening != nil {
		fimSummary = fmt.Sprintf("Memantau integritas %d berkas kritis sistem & Nginx", hardening.FIMBaselineCount)
	}

	pillars := []ProtectionItem{
		{
			Key:        "firewall",
			Name:       "Firewall UFW",
			Installed:  fwInstalled,
			Enabled:    fwEnabled,
			StatusText: statusLabel(fwInstalled, fwEnabled),
			Summary:    fwSummary,
			TargetTab:  "firewall",
			Toggleable: true,
		},
		{
			Key:        "ssh",
			Name:       "SSH Guard",
			Installed:  sshCfg.Installed,
			Enabled:    sshCfg.ActiveService,
			StatusText: statusLabel(sshCfg.Installed, sshCfg.ActiveService),
			Summary:    sshSummary,
			TargetTab:  "ssh",
			Toggleable: false,
		},
		{
			Key:        "waf",
			Name:       "Nginx WAF",
			Installed:  true,
			Enabled:    wafEnabled,
			StatusText: statusLabel(true, wafEnabled),
			Summary:    wafSummary,
			TargetTab:  "waf",
			Toggleable: true,
		},
		{
			Key:        "bruteforce",
			Name:       "Brute Force Guard",
			Installed:  true,
			Enabled:    bfCfg.Enabled,
			StatusText: statusLabel(true, bfCfg.Enabled),
			Summary:    bfSummary,
			TargetTab:  "bruteforce",
			Toggleable: true,
		},
		{
			Key:        "fail2ban",
			Name:       "Fail2Ban IPS",
			Installed:  f2bInstalled,
			Enabled:    f2bRunning,
			StatusText: statusLabel(f2bInstalled, f2bRunning),
			Summary:    f2bSummary,
			TargetTab:  "hardening",
			Toggleable: f2bInstalled,
		},
		{
			Key:        "fim",
			Name:       "File Integrity (FIM)",
			Installed:  true,
			Enabled:    fimEnabled,
			StatusText: statusLabel(true, fimEnabled),
			Summary:    fimSummary,
			TargetTab:  "hardening",
			Toggleable: false,
		},
	}

	enabledCount := 0
	for _, p := range pillars {
		if p.Enabled {
			enabledCount++
		}
	}

	// Login SSH terakhir aktual dari log auth SSH
	lastSSH := LastLoginInfo{Available: false}
	if sshStats != nil {
		for _, ev := range sshStats.RecentLogs {
			if ev.Status == "success" {
				lastSSH = LastLoginInfo{
					Available: true,
					IP:        ev.IP,
					User:      ev.User,
					Time:      time.Unix(ev.Timestamp, 0).Format("2006-01-02 15:04:05"),
					Timestamp: ev.Timestamp,
					Port:      ev.Port,
					Success:   true,
				}
				break
			}
		}
	}

	// Login Panel terakhir aktual dari tabel security_login_events / sessions
	lastPanel := LastLoginInfo{Available: false}
	if m.store != nil {
		if ev, err := m.store.GetLatestPanelLogin(ctx); err == nil && ev != nil {
			lastPanel = LastLoginInfo{
				Available: true,
				IP:        ev.IP,
				User:      ev.Username,
				Time:      time.Unix(ev.TS, 0).Format("2006-01-02 15:04:05"),
				Timestamp: ev.TS,
				Port:      m.panelPort,
				Success:   true,
			}
		}
	}

	hostname, _ := os.Hostname()
	return &ProtectionOverview{
		IsLinux:             runtime.GOOS == "linux",
		Hostname:            hostname,
		EnabledCount:        enabledCount,
		TotalCount:          len(pillars),
		ClientIP:            clientIP,
		IsClientWhitelisted: m.IsIPWhitelisted(ctx, clientIP),
		LastSSHLogin:        lastSSH,
		LastPanelLogin:      lastPanel,
		LatestScan:          m.GetLatestScanReport(ctx),
		Pillars:             pillars,
	}, nil
}

func boolTextID(b bool) string {
	if b {
		return "Aktif"
	}
	return "Nonaktif"
}

func statusLabel(installed, enabled bool) string {
	if !installed {
		return "Belum Terpasang"
	}
	if enabled {
		return "Aktif"
	}
	return "Nonaktif"
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
