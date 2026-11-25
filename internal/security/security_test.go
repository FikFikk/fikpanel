package security

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fikpanel/internal/store"
)

// openTestStore membuka database SQLite sementara beserta seluruh migrasi untuk pengujian unit
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "security_test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("gagal membuka store pengujian: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// TestParseUFWNumberedAndDefaults memverifikasi parsing UFW verbose & numbered untuk IPv4, IPv6, IN/OUT, dan komentar
func TestParseUFWNumberedAndDefaults(t *testing.T) {
	verboseOut := `Status: active
Logging: on (low)
Default: deny (incoming), allow (outgoing), disabled (routed)
New profiles: skip`
	inPol, outPol := parseUFWDefaultPolicies(verboseOut)
	if inPol != "deny" || outPol != "allow" {
		t.Fatalf("kebijakan default tidak sesuai: dapat in=%s out=%s", inPol, outPol)
	}

	numberedOut := `Status: active

     To                         Action      From
     --                         ------      ----
[ 1] 22/tcp                     LIMIT IN    Anywhere                   # SSH Guard
[ 2] 80/tcp (v6)                ALLOW IN    Anywhere (v6)
[ 3] 3306/tcp                   ALLOW IN    10.0.0.0/8                 # Internal DB
[ 4] Anywhere                   DENY IN     185.220.101.44             # Brute Force Ban
[ 5] 443/tcp                    ALLOW OUT   Anywhere (out)             # HTTPS Keluar
[ 6] 39000:40000/tcp            ALLOW IN    Anywhere
`
	enabled, rules := parseUFWNumbered(numberedOut)
	if !enabled {
		t.Fatalf("status UFW seharusnya aktif")
	}
	if len(rules) != 6 {
		t.Fatalf("jumlah aturan diharapkan 6, dapat %d", len(rules))
	}

	if rules[0].Action != "LIMIT" || rules[0].Port != "22" || rules[0].Remarks != "SSH Guard" || rules[0].From != "0.0.0.0/0" {
		t.Errorf("rule #1 salah parse: %+v", rules[0])
	}
	if rules[1].IPVersion != "IPv6" || rules[1].From != "::/0" {
		t.Errorf("rule #2 IPv6 salah parse: %+v", rules[1])
	}
	if rules[2].From != "10.0.0.0/8" || rules[2].Service != "MySQL / MariaDB" || rules[2].Remarks != "Internal DB" {
		t.Errorf("rule #3 CIDR salah parse: %+v", rules[2])
	}
	if rules[3].Action != "DENY" || rules[3].Port != "All" || rules[3].From != "185.220.101.44" {
		t.Errorf("rule #4 IP deny salah parse: %+v", rules[3])
	}
	if rules[4].Direction != "Outbound" || rules[4].Port != "443" {
		t.Errorf("rule #5 outbound salah parse: %+v", rules[4])
	}
	if rules[5].Port != "39000-40000" || rules[5].Protocol != "tcp" {
		t.Errorf("rule #6 rentang port salah parse: %+v", rules[5])
	}
}

// TestParseSSListeningOutput memverifikasi ekstraksi port, proses, PID, dan eksposur publik dari output ss -tulnp
func TestParseSSListeningOutput(t *testing.T) {
	sample := `Netid State  Recv-Q Send-Q Local Address:Port Peer Address:Port Process
tcp   LISTEN 0      511    0.0.0.0:80         0.0.0.0:*         users:(("nginx",pid=1120,fd=6))
tcp   LISTEN 0      128    127.0.0.1:3306     0.0.0.0:*         users:(("mariadbd",pid=980,fd=21))
tcp   LISTEN 0      128    [::]:22            [::]:*            users:(("sshd",pid=501,fd=4))
udp   UNCONN 0      0      127.0.0.53%lo:53   0.0.0.0:*         users:(("systemd-resolve",pid=310,fd=12))`

	ports := parseSSListeningOutput(sample)
	if len(ports) != 4 {
		t.Fatalf("diharapkan 4 soket listening, dapat %d", len(ports))
	}

	byPort := map[string]ListeningPort{}
	for _, p := range ports {
		byPort[p.Port] = p
	}
	if !byPort["80"].ExposedPublic || byPort["80"].Process != "nginx" || byPort["80"].PID != 1120 {
		t.Errorf("parsing port 80 tidak sesuai: %+v", byPort["80"])
	}
	if byPort["3306"].ExposedPublic || byPort["3306"].Process != "mariadbd" {
		t.Errorf("port 3306 loopback seharusnya tidak ExposedPublic: %+v", byPort["3306"])
	}
	if !byPort["22"].ExposedPublic || byPort["22"].Protocol != "tcp6" {
		t.Errorf("parsing port 22 IPv6 tidak sesuai: %+v", byPort["22"])
	}
	if byPort["53"].ExposedPublic || byPort["53"].Address != "127.0.0.53" {
		t.Errorf("parsing port 53 interface suffix tidak sesuai: %+v", byPort["53"])
	}
}

// TestValidatePortSpec memverifikasi validasi port tunggal, rentang port, dan penolakan input tidak valid
func TestValidatePortSpec(t *testing.T) {
	if got, err := ValidatePortSpec("8080"); err != nil || got != "8080" {
		t.Fatalf("port 8080 seharusnya valid, got=%s err=%v", got, err)
	}
	if got, err := ValidatePortSpec("3000-4000"); err != nil || got != "3000:4000" {
		t.Fatalf("rentang 3000-4000 seharusnya dinormalisasi ke 3000:4000, got=%s err=%v", got, err)
	}
	for _, bad := range []string{"0", "70000", "4000-3000", "abc", "80:90:100"} {
		if _, err := ValidatePortSpec(bad); err == nil {
			t.Errorf("port tidak valid %q seharusnya mengembalikan error", bad)
		}
	}
}

// TestParseSSHAuthLogs memverifikasi ekstraksi statistik 7 hari dan log sukses/gagal dari journalctl short-unix
func TestParseSSHAuthLogs(t *testing.T) {
	now := time.Now()
	tsToday := now.Unix() - 600
	tsYesterday := now.AddDate(0, 0, -1).Unix()

	raw := "" +
		formatUnixLine(tsYesterday, "sshd[100]: Failed password for invalid user admin from 185.220.101.44 port 55112 ssh2") +
		formatUnixLine(tsToday, "sshd[101]: Accepted publickey for root from 203.0.113.10 port 49122 ssh2") +
		formatUnixLine(tsToday+5, "sshd[102]: Invalid user testuser from 198.51.100.8 port 41100")

	stats := &SSHStats{
		DailyStats: buildEmptySevenDays(now),
		RecentLogs: []SSHLog{},
	}
	parseSSHAuthLogs(raw, now, stats)

	if stats.SuccessfulLogins != 1 || stats.FailedLogins != 2 {
		t.Fatalf("total login tidak sesuai: success=%d fail=%d", stats.SuccessfulLogins, stats.FailedLogins)
	}
	if stats.TodaySuccess != 1 || stats.TodayFail != 1 || stats.YesterdayFail != 1 {
		t.Fatalf("ringkasan harian salah: today=%d/%d yesterday=%d/%d",
			stats.TodaySuccess, stats.TodayFail, stats.YesterdaySuccess, stats.YesterdayFail)
	}
	if len(stats.RecentLogs) != 3 {
		t.Fatalf("jumlah log terbaru diharapkan 3, dapat %d", len(stats.RecentLogs))
	}
	// Urutan terbaru di atas
	if stats.RecentLogs[0].IP != "198.51.100.8" || stats.RecentLogs[1].Status != "success" {
		t.Errorf("urutan log terbaru tidak tepat: %+v", stats.RecentLogs)
	}
}

func formatUnixLine(ts int64, msg string) string {
	return time.Unix(ts, 0).Format("1700000000")[:0] + strconvFormat(ts) + ".000000 host " + msg + "\n"
}

func strconvFormat(v int64) string {
	// Helper sederhana untuk menyusun prefix unix timestamp journalctl
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[i:])
}

// TestBruteForcePersistenceAndEnforcement memverifikasi whitelist, proteksi self-lockout, dan auto-ban persisten
func TestBruteForcePersistenceAndEnforcement(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	mgr := NewManager(st, 1405)

	// Simpan konfigurasi batas ketat (3x gagal -> ban 30 menit)
	cfg := DefaultBruteForceConfig()
	cfg.IPMaxRetries = 3
	cfg.UsernameMaxRetries = 3
	cfg.IPBanDurationMinutes = 30
	if _, err := mgr.SaveBruteForceConfig(ctx, cfg); err != nil {
		t.Fatalf("gagal menyimpan konfigurasi brute force: %v", err)
	}

	// Tolak blacklist IP loopback & IP pemanggil sendiri
	if err := mgr.AddBlacklistIP(ctx, "127.0.0.1", "tes", 60, false, "manual", "203.0.113.5"); err == nil {
		t.Fatalf("seharusnya menolak blacklist loopback")
	}
	if err := mgr.AddBlacklistIP(ctx, "203.0.113.5", "tes", 60, false, "manual", "203.0.113.5"); err == nil {
		t.Fatalf("seharusnya menolak blacklist IP admin sendiri")
	}

	// Whitelist CIDR harus mengizinkan IP di dalam subnet
	if err := mgr.AddWhitelistIP(ctx, "10.20.0.0/24", "VPN Kantor"); err != nil {
		t.Fatalf("gagal menambah whitelist CIDR: %v", err)
	}
	if !mgr.IsIPWhitelisted(ctx, "10.20.0.42") {
		t.Fatalf("IP 10.20.0.42 seharusnya cocok dengan CIDR whitelist 10.20.0.0/24")
	}

	// Simulasikan 3 percobaan login gagal dari IP penyerang -> memicu auto-ban
	attacker := "198.51.100.77"
	for i := 1; i <= 3; i++ {
		banned, _ := mgr.RecordLoginFailure(ctx, attacker, "admin", "Password salah", "TestAgent")
		if i < 3 && banned {
			t.Fatalf("tidak boleh auto-ban sebelum batas ke-3 (percobaan #%d)", i)
		}
		if i == 3 && !banned {
			t.Fatalf("seharusnya memicu auto-ban pada kegagalan ke-3")
		}
	}

	if _, isBanned := mgr.IsIPBlacklisted(ctx, attacker); !isBanned {
		t.Fatalf("IP penyerang seharusnya aktif di blacklist setelah auto-ban")
	}
	if allowed, _ := mgr.CheckLoginAllowed(ctx, attacker, "admin"); allowed {
		t.Fatalf("CheckLoginAllowed seharusnya menolak IP yang terkena auto-ban")
	}
}

// TestFIMAndWebrootScanner memverifikasi deteksi modifikasi berkas FIM dan deteksi webshell PHP pada webroot
func TestFIMAndWebrootScanner(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	mgr := NewManager(st, 1405)

	// Uji pemindaian webroot terhadap webshell PHP dan direktori .git
	webDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(webDir, ".git"), 0755)
	_ = os.WriteFile(filepath.Join(webDir, ".git", "config"), []byte("[core]\n"), 0644)
	_ = os.WriteFile(filepath.Join(webDir, "shell.php"), []byte("<?php eval(base64_decode($_POST['x'])); ?>"), 0644)

	report, err := mgr.RunSecurityScan(ctx, ScanOptions{
		IsHTTPS:           true,
		TOTPEnabled:       true,
		BruteForceEnabled: true,
		SiteRoots:         []string{webDir},
	})
	if err != nil {
		t.Fatalf("RunSecurityScan gagal: %v", err)
	}
	if report.MalwareCount != 1 {
		t.Errorf("diharapkan 1 deteksi malware PHP, dapat %d", report.MalwareCount)
	}
	if report.VulnCount < 1 {
		t.Errorf("diharapkan minimal 1 kerentanan (.git), dapat %d", report.VulnCount)
	}

	// Pastikan hasil pemindaian tersimpan persisten di SQLite
	latest := mgr.GetLatestScanReport(ctx)
	if latest == nil || latest.Score != report.Score {
		t.Fatalf("laporan scan terakhir gagal dibaca kembali dari SQLite")
	}

	// Uji FIM baseline & deteksi perubahan hash berkas
	tmpFile := filepath.Join(t.TempDir(), "critical.conf")
	_ = os.WriteFile(tmpFile, []byte("PermitRootLogin no\n"), 0600)
	sum, size, mode, mtime, err := hashFileSHA256(tmpFile)
	if err != nil {
		t.Fatalf("hashFileSHA256 gagal: %v", err)
	}
	if err := st.ReplaceFIMBaselines(ctx, []store.FIMBaselineEntry{{
		Path:      tmpFile,
		SHA256:    sum,
		Size:      size,
		Mode:      mode,
		ModTime:   mtime,
		UpdatedAt: time.Now().Unix(),
	}}); err != nil {
		t.Fatalf("ReplaceFIMBaselines gagal: %v", err)
	}

	okCheck, err := mgr.CheckFIMIntegrity(ctx)
	if err != nil || okCheck.OKCount != 1 || okCheck.ModifiedCount != 0 {
		t.Fatalf("cek FIM awal seharusnya OK, dapat: %+v err=%v", okCheck, err)
	}

	// Ubah isi berkas dan pastikan FIM langsung mendeteksi drift
	_ = os.WriteFile(tmpFile, []byte("PermitRootLogin yes\n"), 0600)
	driftCheck, err := mgr.CheckFIMIntegrity(ctx)
	if err != nil || driftCheck.ModifiedCount != 1 {
		t.Fatalf("FIM seharusnya mendeteksi 1 berkas berubah, dapat: %+v err=%v", driftCheck, err)
	}
}
