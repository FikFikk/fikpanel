package cron

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// daftar pola perintah destruktif yang dilarang dijalankan melalui penjadwal cron
var dangerousCommandPatterns = []struct {
	re     *regexp.Regexp
	reason string
}{
	{regexp.MustCompile(`(?i)(^|[;&|(\s])(shutdown|reboot|poweroff|halt)(\s|$|[;&|)])`), "perintah mematikan/restart mesin (shutdown/reboot/poweroff/halt) dilarang di cron"},
	{regexp.MustCompile(`(?i)(^|[;&|(\s])(tel)?init\s+[06](\s|$|[;&|)])`), "perintah pengubah runlevel init 0/6 dilarang di cron"},
	{regexp.MustCompile(`(?i)systemctl\s+(poweroff|halt|reboot)\b`), "perintah systemctl poweroff/halt/reboot dilarang di cron"},
	{regexp.MustCompile(`(?i)(^|[;&|(\s])(mkfs(\.[a-z0-9]+)?|wipefs|fdisk|parted)(\s|$)`), "perintah format/partisi disk (mkfs/wipefs/fdisk/parted) dilarang di cron"},
	{regexp.MustCompile(`(?i)\bdd\b[^\n;&|]*\bof=/dev/(sd|nvme|mmcblk|vd|hd)`), "penulisan langsung ke perangkat blok /dev/* melalui dd dilarang di cron"},
	{regexp.MustCompile(`(?i)\brm\s+[^\n;&|]*--no-preserve-root\b`), "penghapusan root filesystem (--no-preserve-root) dilarang"},
	{regexp.MustCompile(`(?i)\brm\s+-[a-z]*r[a-z]*f[a-z]*\s+/(\*|\s|$|[;&|])`), "penghapusan rekursif direktori root '/' dilarang"},
	{regexp.MustCompile(`(?i)\brm\s+-[a-z]*f[a-z]*r[a-z]*\s+/(\*|\s|$|[;&|])`), "penghapusan rekursif direktori root '/' dilarang"},
	{regexp.MustCompile(`:\(\)\s*\{[^}]*:\s*\|\s*:`), "pola fork bomb terdeteksi dan diblokir"},
	{regexp.MustCompile(`(?i)(^|[;&|(\s])(passwd|chpasswd)(\s|$|[;&|)])`), "perintah pengubah sandi sistem (passwd/chpasswd) dilarang di cron"},
}

// ValidateCommand memeriksa apakah perintah shell aman dijalankan sebagai tugas cron
func ValidateCommand(command string) error {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return fmt.Errorf("perintah eksekusi wajib diisi")
	}
	if len(trimmed) > 8192 {
		return fmt.Errorf("perintah eksekusi terlalu panjang (maksimal 8192 karakter)")
	}
	if strings.ContainsRune(trimmed, '\x00') {
		return fmt.Errorf("perintah mengandung karakter ilegal")
	}
	for _, rule := range dangerousCommandPatterns {
		if rule.re.MatchString(trimmed) {
			return fmt.Errorf("%s", rule.reason)
		}
	}
	return nil
}

// allowedLinuxLogPrefixes membatasi direktori berkas log kustom agar tidak dapat membaca atau mengosongkan berkas sistem sensitif
var allowedLinuxLogPrefixes = []string{
	"/var/log/",
	"/var/cronlog/",
	"/tmp/",
	"/home/",
	"/www/",
	"/opt/fikpanel/logs/",
}

// ValidateLogFilePath memvalidasi jalur berkas log kustom dari path traversal dan direktori sensitif sistem
func ValidateLogFilePath(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if strings.ContainsRune(trimmed, '\x00') {
		return fmt.Errorf("jalur berkas log mengandung karakter ilegal")
	}
	slash := filepath.ToSlash(trimmed)
	for _, seg := range strings.Split(slash, "/") {
		if seg == ".." {
			return fmt.Errorf("jalur berkas log tidak boleh mengandung traversal '..'")
		}
	}
	if runtime.GOOS != "linux" {
		// Di mode pengembangan/uji Windows, izinkan jalur absolut temp atau folder dev-data
		clean := filepath.Clean(trimmed)
		tmpDir := filepath.Clean(os.TempDir())
		if strings.HasPrefix(strings.ToLower(clean), strings.ToLower(tmpDir)) || strings.HasPrefix(filepath.ToSlash(clean), "dev-data/") {
			return nil
		}
		if filepath.IsAbs(clean) {
			return nil
		}
		return fmt.Errorf("jalur berkas log harus absolut atau berada di dalam dev-data/")
	}

	if !strings.HasPrefix(slash, "/") {
		return fmt.Errorf("jalur berkas log di Linux harus berupa jalur absolut (diawali '/')")
	}
	cleanSlash := filepath.ToSlash(filepath.Clean(slash))
	// Pastikan direktori atau file berada di bawah salah satu prefix aman
	checkPath := cleanSlash
	if !strings.HasSuffix(checkPath, "/") {
		checkPath += "/"
	}
	for _, prefix := range allowedLinuxLogPrefixes {
		if strings.HasPrefix(checkPath, prefix) {
			return nil
		}
	}
	return fmt.Errorf("berkas log hanya diizinkan di dalam /var/log/, /var/cronlog/, /tmp/, /home/, /www/, atau /opt/fikpanel/logs/")
}

// ValidateWorkingDir memvalidasi direktori kerja opsional tugas cron
func ValidateWorkingDir(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if strings.ContainsRune(trimmed, '\x00') {
		return fmt.Errorf("direktori kerja mengandung karakter ilegal")
	}
	for _, seg := range strings.Split(filepath.ToSlash(trimmed), "/") {
		if seg == ".." {
			return fmt.Errorf("direktori kerja tidak boleh mengandung traversal '..'")
		}
	}
	if runtime.GOOS == "linux" && !strings.HasPrefix(trimmed, "/") {
		return fmt.Errorf("direktori kerja harus berupa jalur absolut (diawali '/')")
	}
	return nil
}
