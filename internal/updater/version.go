// Package updater mengimplementasikan pembaruan mandiri (self-update) FikPanel yang aman:
// resolusi rilis GitHub, verifikasi checksum SHA-256, penggantian biner atomik, cadangan
// database, health check pasca-restart, dan rollback otomatis bila versi baru gagal berjalan.
package updater

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// tagPattern membatasi format tag rilis agar aman dipakai di dalam URL dan nama berkas.
	tagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:[-.][0-9A-Za-z.-]+)?$`)
	// versionOutputPattern mengekstrak nomor versi dari output `fikpanel version`.
	versionOutputPattern = regexp.MustCompile(`v?([0-9]+\.[0-9]+\.[0-9]+(?:[-.][0-9A-Za-z.-]+)?)`)
)

// NormalizeTag mengubah input versi pengguna ("0.3.20" atau "v0.3.20") menjadi tag rilis kanonik ("v0.3.20").
func NormalizeTag(input string) (string, error) {
	tag := strings.TrimSpace(input)
	if tag == "" {
		return "", fmt.Errorf("versi target kosong")
	}
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	if !tagPattern.MatchString(tag) {
		return "", fmt.Errorf("format versi tidak valid: %q (contoh yang benar: v0.3.20)", input)
	}
	return tag, nil
}

// TagVersion mengembalikan nomor versi tanpa awalan "v" ("v0.3.20" -> "0.3.20").
func TagVersion(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// ParseVersionOutput mengekstrak nomor versi dari output `fikpanel version` (misal "fikpanel v0.3.19").
// Mengembalikan string kosong bila tidak ada pola versi yang dikenali.
func ParseVersionOutput(out string) string {
	m := versionOutputPattern.FindStringSubmatch(out)
	if m == nil {
		return ""
	}
	return m[1]
}

// CompareVersions membandingkan dua versi semver numerik (major.minor.patch).
// Mengembalikan 1 jika a > b, -1 jika a < b, dan 0 jika sama. Sufiks pre-release diabaikan.
func CompareVersions(a, b string) int {
	pa, pb := numericParts(a), numericParts(b)
	for i := range pa {
		if pa[i] > pb[i] {
			return 1
		}
		if pa[i] < pb[i] {
			return -1
		}
	}
	return 0
}

func numericParts(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.Split(v, ".") {
		if i >= len(out) {
			break
		}
		n, _ := strconv.Atoi(part)
		out[i] = n
	}
	return out
}
