package php

import (
	"strings"
	"testing"
)

func TestValidateVersion(t *testing.T) {
	valid := []string{"7.4", "8.0", "8.1", "8.2", "8.3", "8.4", "8.5"}
	for _, v := range valid {
		if err := validateVersion(v); err != nil {
			t.Errorf("versi %q seharusnya valid, dapat error: %v", v, err)
		}
	}

	invalid := []string{"", "../etc", "8.3;rm -rf /", "10.0", "php8.3", "8", "8.3.1"}
	for _, v := range invalid {
		if err := validateVersion(v); err == nil {
			t.Errorf("versi %q seharusnya ditolak", v)
		}
	}
}

func TestValidateExtensionName(t *testing.T) {
	if got, err := validateExtensionName("  PDO_MySQL "); err != nil || got != "pdo_mysql" {
		t.Fatalf("ekstensi valid gagal diparsing: got=%q err=%v", got, err)
	}

	bad := []string{"", "../evil", "redis;id", "-flag", "ext name"}
	for _, b := range bad {
		if _, err := validateExtensionName(b); err == nil {
			t.Errorf("ekstensi %q seharusnya ditolak", b)
		}
	}
}

func TestSortVersionInfosInstalledFirst(t *testing.T) {
	list := []PHPVersionInfo{
		{Version: "8.5", Installed: false, Running: false},
		{Version: "8.4", Installed: false, Running: false},
		{Version: "8.3", Installed: true, Running: true},
		{Version: "8.2", Installed: true, Running: false},
		{Version: "8.1", Installed: false, Running: false},
	}

	sortVersionInfos(list)

	want := []string{"8.3", "8.2", "8.5", "8.4", "8.1"}
	for i, w := range want {
		if list[i].Version != w {
			t.Fatalf("urutan indeks %d: dapat %s, harap %s", i, list[i].Version, w)
		}
	}
}

func TestApplyIniUpdates(t *testing.T) {
	sample := `[PHP]
memory_limit = 128M
upload_max_filesize = 2M

[Date]
;date.timezone =

[opcache]
;opcache.enable=1
;opcache.memory_consumption=128
`
	updated, err := applyIniUpdates(sample, map[string]string{
		"memory_limit":               "512M",
		"date.timezone":              "Asia/Jakarta",
		"opcache.memory_consumption": "256",
		"custom_directive":           "On",
	})
	if err != nil {
		t.Fatalf("applyIniUpdates gagal: %v", err)
	}

	// Pastikan baris aktif diganti dan baris komentar di-uncomment di seksi aslinya
	if !strings.Contains(updated, "memory_limit = 512M") {
		t.Errorf("memory_limit tidak diperbarui:\n%s", updated)
	}
	if strings.Contains(updated, ";date.timezone") || !strings.Contains(updated, "[Date]\ndate.timezone = Asia/Jakarta") {
		t.Errorf("date.timezone tidak di-uncomment di seksi [Date]:\n%s", updated)
	}
	if !strings.Contains(updated, "opcache.memory_consumption = 256") {
		t.Errorf("opcache.memory_consumption tidak di-uncomment:\n%s", updated)
	}
	if !strings.Contains(updated, "custom_directive = On") {
		t.Errorf("custom_directive baru tidak disisipkan:\n%s", updated)
	}

	// Pastikan injeksi baris baru ditolak
	if _, err := applyIniUpdates(sample, map[string]string{"memory_limit": "256M\nextension=evil.so"}); err == nil {
		t.Errorf("injeksi newline pada nilai INI seharusnya ditolak")
	}
}

func TestFormatByteSize(t *testing.T) {
	if got := formatByteSize(24 * 1024 * 1024); got != "24.0 MB" {
		t.Errorf("formatByteSize(24MiB) = %q, harap 24.0 MB", got)
	}
	if got := formatByteSize(512); got != "512 B" {
		t.Errorf("formatByteSize(512) = %q, harap 512 B", got)
	}
}
