package sites

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var labelRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// Second-level domain umum (heuristik) agar www tidak ditambahkan ke subdomain biasa
var secondLevel = map[string]bool{
	"co": true, "com": true, "net": true, "org": true, "ac": true, "or": true,
	"go": true, "web": true, "my": true, "sch": true, "biz": true, "gov": true, "edu": true,
}

// DomainCheck adalah hasil validasi satu baris input domain
type DomainCheck struct {
	Input    string `json:"input"`
	Domain   string `json:"domain"`
	Wildcard bool   `json:"wildcard"`
	Auto     bool   `json:"auto"`
	Error    string `json:"error,omitempty"`
}

// NormalizeDomain membersihkan input (skema, path, titik akhir) dan memvalidasi format RFC 1123
func NormalizeDomain(raw string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(raw))
	if strings.Contains(d, "://") {
		if u, err := url.Parse(d); err == nil && u.Host != "" {
			d = u.Host
		}
	}
	if i := strings.IndexAny(d, "/?#"); i >= 0 {
		d = d[:i]
	}
	d = strings.TrimSuffix(d, ".")
	if d == "" {
		return "", fmt.Errorf("domain kosong")
	}
	if strings.Contains(d, ":") {
		return "", fmt.Errorf("port kustom belum didukung, gunakan domain tanpa ':port'")
	}
	for _, r := range d {
		if r > 127 {
			return "", fmt.Errorf("domain IDN harus ditulis dalam punycode (xn--...)")
		}
	}
	if net.ParseIP(d) != nil {
		return "", fmt.Errorf("gunakan nama domain, bukan alamat IP")
	}
	if len(d) > 253 {
		return "", fmt.Errorf("domain melebihi 253 karakter")
	}

	host := d
	wildcard := strings.HasPrefix(d, "*.")
	if wildcard {
		host = d[2:]
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("domain harus memiliki minimal 2 label (contoh: situs.com)")
	}
	for _, l := range labels {
		if !labelRe.MatchString(l) {
			return "", fmt.Errorf("label '%s' tidak valid (huruf, angka, '-' dan maks 63 karakter)", l)
		}
	}
	tld := labels[len(labels)-1]
	if strings.Trim(tld, "0123456789") == "" {
		return "", fmt.Errorf("TLD tidak boleh berupa angka")
	}
	return d, nil
}

// isApex menebak apakah domain adalah domain utama (bukan subdomain) untuk penambahan www otomatis
func isApex(d string) bool {
	if strings.HasPrefix(d, "www.") || strings.HasPrefix(d, "*.") {
		return false
	}
	labels := strings.Split(d, ".")
	if len(labels) == 2 {
		return true
	}
	return len(labels) == 3 && len(labels[2]) == 2 && secondLevel[labels[1]]
}

// ParseDomainList memecah input multi-baris menjadi domain utama + alias beserta hasil validasi per baris
func ParseDomainList(text string, addWWW bool) (primary string, aliases []string, checks []DomainCheck) {
	seen := map[string]bool{}
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ' ' || r == '\t'
	})

	var valid []string
	for _, f := range fields {
		d, err := NormalizeDomain(f)
		c := DomainCheck{Input: f, Domain: d}
		if err != nil {
			c.Error = err.Error()
			checks = append(checks, c)
			continue
		}
		c.Wildcard = strings.HasPrefix(d, "*.")
		if seen[d] {
			c.Error = "duplikat"
			checks = append(checks, c)
			continue
		}
		seen[d] = true
		checks = append(checks, c)
		valid = append(valid, d)

		if addWWW && isApex(d) && !seen["www."+d] {
			www := "www." + d
			seen[www] = true
			checks = append(checks, DomainCheck{Input: www, Domain: www, Auto: true})
			valid = append(valid, www)
		}
	}

	// Domain utama tidak boleh wildcard; pilih domain non-wildcard pertama
	for i, d := range valid {
		if !strings.HasPrefix(d, "*.") {
			primary = d
			aliases = append(append([]string{}, valid[:i]...), valid[i+1:]...)
			return primary, aliases, checks
		}
	}
	return "", valid, checks
}

// ValidateUpstream memastikan upstream proxy berupa URL http(s) dengan host yang valid
func ValidateUpstream(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("upstream harus berupa URL http:// atau https:// (contoh: http://127.0.0.1:3000)")
	}
	if strings.ContainsAny(u.Host, " ;{}'\"") {
		return "", fmt.Errorf("upstream mengandung karakter terlarang")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// ValidateRootPath memastikan path web root absolut, bersih, dan aman disisipkan ke konfigurasi
func ValidateRootPath(raw string) (string, error) {
	p := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("direktori situs harus berupa path absolut (diawali '/')")
	}
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("direktori situs tidak boleh mengandung '..'")
	}
	if strings.ContainsAny(p, " ;{}'\"$\n") {
		return "", fmt.Errorf("direktori situs mengandung karakter terlarang")
	}
	clean := path.Clean(p)
	for _, forbidden := range []string{"/", "/etc", "/bin", "/sbin", "/usr", "/boot", "/proc", "/sys", "/dev", "/root"} {
		if clean == forbidden {
			return "", fmt.Errorf("direktori sistem %s tidak boleh dijadikan web root", clean)
		}
	}
	return clean, nil
}

var phpVersionRe = regexp.MustCompile(`^[0-9]\.[0-9]$`)

// ValidatePHPVersion memastikan format versi PHP seperti 8.3
func ValidatePHPVersion(v string) error {
	if v != "" && !phpVersionRe.MatchString(v) {
		return fmt.Errorf("format versi PHP tidak valid")
	}
	return nil
}
