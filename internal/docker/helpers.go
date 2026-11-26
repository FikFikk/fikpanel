package docker

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reHumanSize   = regexp.MustCompile(`(?i)^\s*([0-9]+(?:\.[0-9]+)?)\s*(b|kb|kib|mb|mib|gb|gib|tb|tib|pb|pib|k|m|g|t)?\s*$`)
	reReclaimSize = regexp.MustCompile(`(?i)^\s*([0-9]+(?:\.[0-9]+)?\s*[a-zA-Z]+)\s*(?:\(([0-9]+(?:\.[0-9]+)?)%\))?`)
	reValidRef    = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-/:@]*$`)
	reValidProj   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-]{0,63}$`)
	reHostPortMap = regexp.MustCompile(`^(?:(\[?[0-9a-fA-F:.]+\]?):)?(\d+)(?:-\d+)?->(\d+)(?:-\d+)?/(tcp|udp|sctp)$`)
	reExposedPort = regexp.MustCompile(`^(\d+)(?:-\d+)?/(tcp|udp|sctp)$`)
	reComposeCnt  = regexp.MustCompile(`(\w+)\((\d+)\)`)
)

// ParseHumanSize mengubah teks ukuran Docker (mis. "142MB", "1.45 GiB", "520kB") menjadi satuan byte
func ParseHumanSize(raw string) int64 {
	s := strings.TrimSpace(raw)
	if s == "" || s == "0" || strings.EqualFold(s, "n/a") {
		return 0
	}
	// Jika ada komponen virtual seperti "12.4MB (virtual 180MB)", ambil bagian depan
	if idx := strings.Index(s, "("); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	m := reHumanSize.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil || math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	unit := strings.ToLower(strings.TrimSpace(m[2]))
	var mult float64 = 1
	switch unit {
	case "", "b":
		mult = 1
	case "k", "kb":
		mult = 1000
	case "kib":
		mult = 1024
	case "m", "mb":
		mult = 1000 * 1000
	case "mib":
		mult = 1024 * 1024
	case "g", "gb":
		mult = 1000 * 1000 * 1000
	case "gib":
		mult = 1024 * 1024 * 1024
	case "t", "tb":
		mult = 1000 * 1000 * 1000 * 1000
	case "tib":
		mult = 1024 * 1024 * 1024 * 1024
	case "pb":
		mult = 1000 * 1000 * 1000 * 1000 * 1000
	case "pib":
		mult = 1024 * 1024 * 1024 * 1024 * 1024
	}
	return int64(math.Round(val * mult))
}

// ParseReclaimableField memisahkan ukuran byte dan persentase dari kolom Reclaimable `docker system df`
func ParseReclaimableField(raw string) (int64, string, float64) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, "0 B", 0
	}
	m := reReclaimSize.FindStringSubmatch(s)
	if len(m) < 2 {
		b := ParseHumanSize(s)
		return b, FormatBytes(b), 0
	}
	sizeBytes := ParseHumanSize(m[1])
	pct := 0.0
	if len(m) >= 3 && m[2] != "" {
		pct, _ = strconv.ParseFloat(m[2], 64)
	}
	return sizeBytes, FormatBytes(sizeBytes), pct
}

// FormatBytes memformat jumlah byte menjadi string ringkas yang mudah dibaca (B, KB, MB, GB, TB)
func FormatBytes(b int64) string {
	if b <= 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp])
}

// ParseDockerPorts mengurai string pemetaan port dari `docker ps` (mendukung IPv4, IPv6, dan port ekspos internal)
func ParseDockerPorts(raw string) []DockerContainerPort {
	out := make([]DockerContainerPort, 0)
	seen := make(map[string]struct{})
	if strings.TrimSpace(raw) == "" {
		return out
	}

	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if m := reHostPortMap.FindStringSubmatch(p); len(m) == 5 {
			ip := strings.Trim(m[1], "[]")
			if ip == "" {
				ip = "0.0.0.0"
			}
			pub, _ := strconv.Atoi(m[2])
			priv, _ := strconv.Atoi(m[3])
			proto := strings.ToLower(m[4])
			// Deduplikasi mapping IPv4 0.0.0.0 dan IPv6 ::: pada port publik/privat yang identik agar UI tetap bersih
			dedupKey := fmt.Sprintf("%d->%d/%s", pub, priv, proto)
			if _, exists := seen[dedupKey]; exists {
				continue
			}
			seen[dedupKey] = struct{}{}
			out = append(out, DockerContainerPort{
				IP:          ip,
				PublicPort:  pub,
				PrivatePort: priv,
				Type:        proto,
			})
			continue
		}
		if m := reExposedPort.FindStringSubmatch(p); len(m) == 3 {
			priv, _ := strconv.Atoi(m[1])
			proto := strings.ToLower(m[2])
			dedupKey := fmt.Sprintf("0->%d/%s", priv, proto)
			if _, exists := seen[dedupKey]; exists {
				continue
			}
			seen[dedupKey] = struct{}{}
			out = append(out, DockerContainerPort{
				PrivatePort: priv,
				Type:        proto,
			})
		}
	}
	return out
}

// ParseDockerTimestamp mengubah stempel waktu keluaran CLI Docker menjadi Unix timestamp detik
func ParseDockerTimestamp(raw string) int64 {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	layouts := []string{
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700 -0700",
		"2006-01-02 15:04:05 +0000 UTC",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Unix()
		}
	}
	// Fallback: potong nama zona waktu akhir jika format non-standar (mis. "2026-11-26 12:00:00 +0700 WIB")
	fields := strings.Fields(s)
	if len(fields) >= 3 {
		candidate := strings.Join(fields[:3], " ")
		if t, err := time.Parse("2006-01-02 15:04:05 -0700", candidate); err == nil {
			return t.Unix()
		}
	}
	return 0
}

// ParseComposeStatusSummary mengurai status `docker compose ls` (mis. "running(3), exited(1)") menjadi status, total, dan aktif
func ParseComposeStatusSummary(raw string) (status string, total int, running int) {
	s := strings.ToLower(strings.TrimSpace(raw))
	matches := reComposeCnt.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		state := m[1]
		cnt, _ := strconv.Atoi(m[2])
		total += cnt
		if state == "running" {
			running += cnt
		}
	}
	if running > 0 {
		if total > running {
			return "partial", total, running
		}
		return "running", total, running
	}
	if strings.Contains(s, "running") {
		if total == 0 {
			total = 1
		}
		return "running", total, total
	}
	if total == 0 {
		return "stopped", 0, 0
	}
	return "stopped", total, 0
}

// IsSecretEnvKey menentukan apakah nama variabel lingkungan berisi rahasia (password, token, key, dsb.)
func IsSecretEnvKey(key string) bool {
	u := strings.ToUpper(strings.TrimSpace(key))
	secretTokens := []string{
		"PASS", "PWD", "SECRET", "TOKEN", "API_KEY", "APIKEY",
		"PRIVATE_KEY", "ACCESS_KEY", "AUTH_KEY", "DSN", "CONNECTION_STRING",
		"CREDENTIAL", "JWT", "ENCRYPTION_KEY", "SIGNING_KEY", "WEBHOOK_SECRET",
	}
	for _, tok := range secretTokens {
		if strings.Contains(u, tok) {
			return true
		}
	}
	return false
}

// IsValidDockerRef memvalidasi referensi kontainer, citra, volume, atau jaringan agar bebas injeksi flag CLI
func IsValidDockerRef(ref string) bool {
	s := strings.TrimSpace(ref)
	if s == "" || len(s) > 256 || strings.HasPrefix(s, "-") {
		return false
	}
	return reValidRef.MatchString(s)
}

// IsValidProjectName memvalidasi nama proyek Docker Compose agar aman sebagai nama direktori
func IsValidProjectName(name string) bool {
	s := strings.TrimSpace(name)
	if s == "" || strings.Contains(s, "..") {
		return false
	}
	return reValidProj.MatchString(s)
}

// ReadHostMemoryBytes membaca total RAM host secara akurat dari /proc/meminfo pada Linux
func ReadHostMemoryBytes() int64 {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseInt(fields[1], 10, 64)
				if err == nil && kb > 0 {
					return kb * 1024
				}
			}
		}
	}
	return 0
}

// ReadHostOSPrettyName membaca nama distribusi sistem operasi host dari /etc/os-release
func ReadHostOSPrettyName() string {
	raw, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"' \r\n")
		}
	}
	return ""
}
