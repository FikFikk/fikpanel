package system

import (
	"bufio"
	"bytes"
	"context"
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

// ScanJunk memindai 6 kategori sampah nyata (APT, Journald, Log Rotasi, Temp >3 hari, Cache Dev, dan Docker)
func (m *Manager) ScanJunk(ctx context.Context) (*CleanerScanResult, error) {
	res := &CleanerScanResult{
		IsLinux:    runtime.GOOS == "linux",
		ScannedAt:  time.Now().Unix(),
		Categories: make([]CleanerCategory, 0, 6),
	}

	if runtime.GOOS != "linux" {
		res.Categories = buildCleanerCategories(res, false)
		return res, nil
	}

	// 1. Cache paket APT (/var/cache/apt/archives)
	res.AptCacheBytes = getDirSizeFilesOnly("/var/cache/apt/archives")

	// 2. Log Systemd Journald aktual melalui `journalctl --disk-usage`
	res.JournalBytes = readJournaldUsageBytes(ctx)

	// 3. Arsip log rotasi (*.gz, *.1, *.old) di /var/log
	var logBytes int64
	_ = filepath.Walk("/var/log", func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			if strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".1") || strings.HasSuffix(path, ".old") {
				logBytes += info.Size()
			}
		}
		return nil
	})
	res.SystemLogBytes = logBytes

	// 4. Berkas sementara yang benar-benar berumur > 3 hari di /tmp & /var/tmp
	cutoff := time.Now().Add(-72 * time.Hour)
	res.TmpBytes = getOldFilesSize("/tmp", cutoff) + getOldFilesSize("/var/tmp", cutoff)

	// 5. Cache build developer (Composer, NPM, Pip) di /root dan /home/*
	res.DevCacheBytes = scanDeveloperCaches()

	// 6. Ruang Docker yang dapat direklamasi (`docker system df`)
	dockerInstalled := false
	if _, err := exec.LookPath("docker"); err == nil {
		dockerInstalled = true
		res.DockerReclaim = readDockerReclaimableBytes(ctx)
	}

	res.TotalBytes = res.AptCacheBytes + res.JournalBytes + res.SystemLogBytes + res.TmpBytes + res.DevCacheBytes + res.DockerReclaim
	res.Categories = buildCleanerCategories(res, dockerInstalled)
	return res, nil
}

func buildCleanerCategories(res *CleanerScanResult, dockerInstalled bool) []CleanerCategory {
	return []CleanerCategory{
		{
			Key:         "clean_apt",
			Label:       "Cache Paket APT",
			Path:        "/var/cache/apt/archives",
			Bytes:       res.AptCacheBytes,
			Description: "Arsip paket .deb bekas instalasi yang dapat diunduh ulang kapan saja.",
			Available:   res.IsLinux,
		},
		{
			Key:         "clean_journal",
			Label:       "Systemd Journald Logs",
			Path:        "/var/log/journal",
			Bytes:       res.JournalBytes,
			Description: "Pangkas ukuran log biner systemd journal menjadi maksimal 100 MB terbaru.",
			Available:   res.IsLinux,
		},
		{
			Key:         "clean_logs",
			Label:       "Arsip Log Rotasi",
			Path:        "/var/log (*.gz, *.1, *.old)",
			Bytes:       res.SystemLogBytes,
			Description: "Berkas log terkompresi dan arsip lama hasil logrotate.",
			Available:   res.IsLinux,
		},
		{
			Key:         "clean_tmp",
			Label:       "Berkas Temporer Usang (> 3 Hari)",
			Path:        "/tmp & /var/tmp",
			Bytes:       res.TmpBytes,
			Description: "Hanya menghapus berkas temporer yang tidak diakses selama lebih dari 72 jam.",
			Available:   res.IsLinux,
		},
		{
			Key:         "clean_dev_cache",
			Label:       "Cache Build (Composer / NPM / Pip)",
			Path:        "/root & /home/*/.cache",
			Bytes:       res.DevCacheBytes,
			Description: "Cache unduhan dependensi PHP Composer, Node NPM, dan Python Pip.",
			Available:   res.IsLinux,
		},
		{
			Key:         "clean_docker",
			Label:       "Docker Dangling Images & Build Cache",
			Path:        "docker system prune",
			Bytes:       res.DockerReclaim,
			Description: "Image tidak terpakai, kontainer berhenti, dan build cache Docker.",
			Available:   dockerInstalled,
		},
	}
}

// CleanJunk mengeksekusi pembersihan kategori terpilih dan melaporkan jumlah byte disk & RAM yang benar-benar dibebaskan
func (m *Manager) CleanJunk(ctx context.Context, req CleanRequest) (*CleanReport, error) {
	beforeScan, _ := m.ScanJunk(ctx)
	report := &CleanReport{
		Actions: []string{},
		Errors:  []string{},
	}
	if beforeScan != nil {
		report.BeforeTotal = beforeScan.TotalBytes
	}

	if runtime.GOOS != "linux" {
		return report, fmt.Errorf("pembersihan sistem hanya beroperasi pada server Linux")
	}

	if req.CleanApt {
		if out, err := exec.CommandContext(ctx, "apt-get", "clean").CombinedOutput(); err != nil {
			report.Errors = append(report.Errors, "APT clean gagal: "+strings.TrimSpace(string(out)))
		} else {
			report.Actions = append(report.Actions, "Cache paket APT dibersihkan")
		}
	}

	if req.CleanJournal {
		if out, err := exec.CommandContext(ctx, "journalctl", "--vacuum-size=100M").CombinedOutput(); err != nil {
			report.Errors = append(report.Errors, "Journald vacuum gagal: "+strings.TrimSpace(string(out)))
		} else {
			report.Actions = append(report.Actions, "Log systemd journald dipangkas ke maksimal 100 MB")
		}
	}

	if req.CleanLogs {
		removed := 0
		_ = filepath.Walk("/var/log", func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				if strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".1") || strings.HasSuffix(path, ".old") {
					if os.Remove(path) == nil {
						removed++
					}
				}
			}
			return nil
		})
		report.Actions = append(report.Actions, fmt.Sprintf("%d berkas arsip log rotasi dihapus", removed))
	}

	if req.CleanTmp {
		cutoff := time.Now().Add(-72 * time.Hour)
		n := removeOldFiles("/tmp", cutoff) + removeOldFiles("/var/tmp", cutoff)
		report.Actions = append(report.Actions, fmt.Sprintf("%d berkas temporer (> 3 hari) dihapus", n))
	}

	if req.CleanDevCache {
		cleared := clearDeveloperCaches()
		report.Actions = append(report.Actions, fmt.Sprintf("%d direktori cache developer dibersihkan", cleared))
	}

	if req.CleanDocker {
		if _, err := exec.LookPath("docker"); err == nil {
			if out, err := exec.CommandContext(ctx, "docker", "system", "prune", "-f").CombinedOutput(); err != nil {
				report.Errors = append(report.Errors, "Docker prune gagal: "+strings.TrimSpace(string(out)))
			} else {
				report.Actions = append(report.Actions, "Docker unused images & build cache dibersihkan")
			}
		}
	}

	if req.DropCaches {
		freed, _, err := m.DropRAMCaches(ctx)
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
		} else {
			report.RAMFreedBytes = freed
			report.Actions = append(report.Actions, "Buffer & page cache RAM disinkronisasi dan dilepaskan")
		}
	}

	afterScan, _ := m.ScanJunk(ctx)
	if afterScan != nil {
		report.AfterTotal = afterScan.TotalBytes
		if report.BeforeTotal > report.AfterTotal {
			report.ReclaimedBytes = report.BeforeTotal - report.AfterTotal
		}
	}

	return report, nil
}

// ── Helper Pemindaian Ukuran Berkas & Parser Unit ────────────────────────────

var humanSizeRe = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(B|K|KB|KiB|M|MB|MiB|G|GB|GiB|T|TB|TiB)\b`)

// parseHumanBytes mengonversi string ukuran seperti "412.5M", "1.4G", atau "230.2MB" menjadi satuan byte
func parseHumanBytes(s string) int64 {
	m := humanSizeRe.FindStringSubmatch(s)
	if len(m) != 3 {
		return 0
	}
	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	switch strings.ToUpper(m[2]) {
	case "K", "KB", "KIB":
		return int64(val * 1024)
	case "M", "MB", "MIB":
		return int64(val * 1024 * 1024)
	case "G", "GB", "GIB":
		return int64(val * 1024 * 1024 * 1024)
	case "T", "TB", "TIB":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	default:
		return int64(val)
	}
}

func readJournaldUsageBytes(ctx context.Context) int64 {
	out, err := exec.CommandContext(ctx, "journalctl", "--disk-usage").Output()
	if err != nil {
		return 0
	}
	return parseHumanBytes(string(out))
}

func readDockerReclaimableBytes(ctx context.Context) int64 {
	tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(tCtx, "docker", "system", "df", "--format", "{{.Reclaimable}}").Output()
	if err != nil {
		return 0
	}
	var total int64
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		// Format contoh: "1.2GB (45%)" -> ambil bagian sebelum spasi pertama
		line := strings.TrimSpace(sc.Text())
		if idx := strings.Index(line, "("); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		total += parseHumanBytes(line)
	}
	return total
}

func developerCacheDirs() []string {
	dirs := []string{
		"/root/.npm/_cacache",
		"/root/.composer/cache",
		"/root/.cache/composer",
		"/root/.cache/pip",
	}
	if homes, err := filepath.Glob("/home/*"); err == nil {
		for _, h := range homes {
			dirs = append(dirs,
				filepath.Join(h, ".npm", "_cacache"),
				filepath.Join(h, ".composer", "cache"),
				filepath.Join(h, ".cache", "composer"),
				filepath.Join(h, ".cache", "pip"),
			)
		}
	}
	return dirs
}

func scanDeveloperCaches() int64 {
	var total int64
	for _, d := range developerCacheDirs() {
		total += getDirSizeFilesOnly(d)
	}
	return total
}

func clearDeveloperCaches() int {
	cleared := 0
	for _, d := range developerCacheDirs() {
		fi, err := os.Stat(d)
		if err != nil || !fi.IsDir() {
			continue
		}
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			_ = os.RemoveAll(filepath.Join(d, e.Name()))
		}
		cleared++
	}
	return cleared
}

func getDirSizeFilesOnly(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func getOldFilesSize(root string, cutoff time.Time) int64 {
	var size int64
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.ModTime().Before(cutoff) {
			size += info.Size()
		}
		return nil
	})
	return size
}

func removeOldFiles(root string, cutoff time.Time) int {
	removed := 0
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.ModTime().Before(cutoff) {
			if os.Remove(path) == nil {
				removed++
			}
		}
		return nil
	})
	return removed
}

// ── Pending OS Updates & Kernel / OOM Killer Diagnostics ────────────────────

// CheckUpdates memeriksa daftar paket OS yang dapat diperbarui melalui APT beserta deteksi patch keamanan
func (m *Manager) CheckUpdates(ctx context.Context, refresh bool) (*UpdatesOverview, error) {
	ov := &UpdatesOverview{
		IsLinux:        runtime.GOOS == "linux",
		PackageManager: "tidak tersedia",
		CheckedAt:      time.Now().Unix(),
		Updates:        []PackageUpdate{},
	}

	if runtime.GOOS != "linux" {
		return ov, nil
	}
	if _, err := os.Stat("/var/run/reboot-required"); err == nil {
		ov.NeedsReboot = true
	}

	if _, err := exec.LookPath("apt"); err != nil {
		return ov, nil
	}
	ov.PackageManager = "apt"

	if refresh {
		tCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		_ = exec.CommandContext(tCtx, "apt-get", "update", "-qq").Run()
	}

	out, err := exec.CommandContext(ctx, "apt", "list", "--upgradable").Output()
	if err != nil {
		return ov, nil
	}

	ov.Updates = parseAptUpgradable(string(out))
	ov.TotalCount = len(ov.Updates)
	for _, u := range ov.Updates {
		if u.IsSecurity {
			ov.SecurityCount++
		}
	}
	return ov, nil
}

// Ekspresi reguler untuk baris output `apt list --upgradable`:
// contoh: openssl/jammy-security 3.0.2-0ubuntu1.18 amd64 [upgradable from: 3.0.2-0ubuntu1.16]
var aptUpgradableRe = regexp.MustCompile(`^([^/\s]+)/([^\s]+)\s+([^\s]+)\s+\S+\s+\[upgradable from:\s+([^\]]+)\]`)

// parseAptUpgradable mem-parsing output `apt list --upgradable` menjadi daftar PackageUpdate
func parseAptUpgradable(output string) []PackageUpdate {
	list := make([]PackageUpdate, 0)
	sc := bufio.NewScanner(strings.NewReader(output))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		m := aptUpgradableRe.FindStringSubmatch(line)
		if len(m) != 5 {
			continue
		}
		src := m[2]
		list = append(list, PackageUpdate{
			Name:           m[1],
			Source:         src,
			NewVersion:     m[3],
			CurrentVersion: m[4],
			IsSecurity:     strings.Contains(strings.ToLower(src), "security"),
		})
	}
	return list
}

// ApplySecurityUpdates menerapkan pembaruan paket keamanan secara non-interaktif tanpa menimpa konfigurasi kustom
func (m *Manager) ApplySecurityUpdates(ctx context.Context) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("pembaruan paket OS hanya tersedia pada host server Linux")
	}
	if _, err := exec.LookPath("apt-get"); err != nil {
		return "", fmt.Errorf("manajer paket apt-get tidak tersedia di server ini")
	}

	ov, err := m.CheckUpdates(ctx, true)
	if err != nil {
		return "", err
	}
	secPkgs := make([]string, 0, ov.SecurityCount)
	for _, u := range ov.Updates {
		if u.IsSecurity {
			secPkgs = append(secPkgs, u.Name)
		}
	}
	if len(secPkgs) == 0 {
		return "Tidak ada paket pembaruan keamanan yang tertunda.", nil
	}

	tCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	args := append([]string{"install", "-y", "--only-upgrade", "-o", "Dpkg::Options::=--force-confold"}, secPkgs...)
	cmd := exec.CommandContext(tCtx, "apt-get", args...)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gagal memasang pembaruan keamanan: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return fmt.Sprintf("Berhasil memperbarui %d paket keamanan.", len(secPkgs)), nil
}

// GetDiagnostics membaca log galat/peringatan kernel (`dmesg`) dan `journalctl -p warning`, termasuk deteksi OOM Killer
func (m *Manager) GetDiagnostics(ctx context.Context) (*DiagnosticsReport, error) {
	rep := &DiagnosticsReport{
		IsLinux: runtime.GOOS == "linux",
		Entries: []DiagnosticLogEntry{},
	}
	if runtime.GOOS != "linux" {
		return rep, nil
	}

	// 1. Baca pesan dmesg level err & warn (khususnya mendeteksi OOM Killer)
	if out, err := exec.CommandContext(ctx, "dmesg", "-T", "--level=emerg,alert,crit,err,warn").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 60 {
			lines = lines[len(lines)-60:]
		}
		for i := len(lines) - 1; i >= 0; i-- {
			l := strings.TrimSpace(lines[i])
			if l == "" {
				continue
			}
			entry := classifyDiagnosticLine(l, "kernel")
			rep.Entries = append(rep.Entries, entry)
		}
	}

	// 2. Tambahkan galat layanan systemd terbaru (`journalctl -p err -n 40`)
	if out, err := exec.CommandContext(ctx, "journalctl", "-p", "err", "-n", "40", "--no-pager", "-o", "short-iso").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			l := strings.TrimSpace(lines[i])
			if l == "" || strings.HasPrefix(l, "--") {
				continue
			}
			entry := classifyDiagnosticLine(l, "systemd")
			rep.Entries = append(rep.Entries, entry)
		}
	}

	for _, e := range rep.Entries {
		if e.IsOOM {
			rep.OOMCount++
		}
		if e.Level == "error" || e.Level == "oom" {
			rep.ErrorCount++
		} else {
			rep.WarnCount++
		}
	}
	return rep, nil
}

func classifyDiagnosticLine(line, source string) DiagnosticLogEntry {
	ts := ""
	msg := line
	// Ekstrak timestamp format dmesg `[Mon Nov 25 10:00:00 2026]` atau ISO journalctl
	if strings.HasPrefix(line, "[") {
		if end := strings.Index(line, "]"); end != -1 && end < 40 {
			ts = strings.TrimSpace(line[1:end])
			msg = strings.TrimSpace(line[end+1:])
		}
	} else {
		f := strings.Fields(line)
		if len(f) >= 3 && strings.Contains(f[0], "T") {
			ts = f[0]
			msg = strings.Join(f[2:], " ")
		}
	}

	lower := strings.ToLower(msg)
	isOOM := strings.Contains(lower, "out of memory") || strings.Contains(lower, "oom-killer") || strings.Contains(lower, "killed process")
	level := "warning"
	if isOOM {
		level = "oom"
	} else if source == "systemd" || strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "panic") || strings.Contains(lower, "segfault") {
		level = "error"
	}

	return DiagnosticLogEntry{
		Timestamp: ts,
		Level:     level,
		Source:    source,
		Message:   msg,
		IsOOM:     isOOM,
	}
}

func countRecentOOMKills(ctx context.Context) int {
	out, err := exec.CommandContext(ctx, "dmesg").Output()
	if err != nil {
		return 0
	}
	count := 0
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		low := strings.ToLower(sc.Text())
		if strings.Contains(low, "killed process") || strings.Contains(low, "oom-killer") {
			count++
		}
	}
	return count
}
