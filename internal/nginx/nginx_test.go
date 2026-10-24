package nginx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// memSettings adalah SettingsStore in-memory untuk pengujian
type memSettings map[string]string

func (m memSettings) GetSetting(_ context.Context, k string) (string, error) { return m[k], nil }
func (m memSettings) SetSetting(_ context.Context, k, v string) error      { m[k] = v; return nil }

// nopAudit adalah Auditor yang mengabaikan seluruh catatan
type nopAudit struct{}

func (nopAudit) RecordAudit(context.Context, string, string, string, bool, string, *int64) error {
	return nil
}

func TestCheckSyntax(t *testing.T) {
	cases := []struct {
		name    string
		conf    string
		wantErr bool
	}{
		{"valid", "events { worker_connections 1; }\nhttp { server { listen 80; } }", false},
		{"kurung kurang", "http { server { listen 80; }", true},
		{"kurung lebih", "http { } }", true},
		{"kurung di komentar diabaikan", "http { # } } }\n}", false},
		{"kurung di string diabaikan", "http { return 200 '}'; }", false},
		{"kutip tidak ditutup", "http { return 200 \"abc; }", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkSyntax(c.conf)
			if (err != nil) != c.wantErr {
				t.Fatalf("checkSyntax() err=%v, wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestSetDirective(t *testing.T) {
	conf := "worker_processes 1;\nevents {\n    worker_connections 512;\n}\nhttp {\n    #gzip on;\n    gzip_comp_level 3;\n}\n"

	out, err := setDirective(conf, "worker_connections", "4096", "events")
	if err != nil || !strings.Contains(out, "    worker_connections 4096;") {
		t.Fatalf("ganti directive gagal: %v\n%s", err, out)
	}

	// Baris komentar tidak boleh dianggap directive aktif; gzip harus disisipkan ke blok http
	out, err = setDirective(out, "gzip", "on", "http")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "http {\n    gzip on;") || !strings.Contains(out, "#gzip on;") {
		t.Fatalf("sisip directive gzip salah:\n%s", out)
	}

	// gzip tidak boleh menimpa gzip_comp_level
	if readDirective(out, "gzip_comp_level") != "3" {
		t.Fatalf("gzip_comp_level ikut berubah:\n%s", out)
	}

	if _, err := setDirective("events {}\n", "gzip", "on", "http"); err == nil {
		t.Fatal("harus error bila blok http tidak ada")
	}
}

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		unit byte
		want int
	}{
		{"50m", 'm', 50},
		{"1g", 'm', 1024},
		{"512k", 'k', 512},
		{"2m", 'k', 2048},
		{"", 'k', 7},
		{"abc", 'm', 7},
	}
	for _, c := range cases {
		if got := parseSize(c.in, c.unit, 7); got != c.want {
			t.Errorf("parseSize(%q,%c)=%d want %d", c.in, c.unit, got, c.want)
		}
	}
}

func TestOptimizationValidate(t *testing.T) {
	ok := Optimization{WorkerProcesses: "auto", WorkerConnections: 1024, KeepaliveTimeout: 60, GzipCompLevel: 5,
		ClientMaxBodySizeMB: 50, ServerNamesHashBucketSize: 512, ClientHeaderBufferSizeKB: 32, ClientBodyBufferSizeKB: 512}
	if err := ok.validate(); err != nil {
		t.Fatalf("nilai valid ditolak: %v", err)
	}
	bad := ok
	bad.ServerNamesHashBucketSize = 100
	if bad.validate() == nil {
		t.Fatal("hash bucket bukan pangkat dua harus ditolak")
	}
	bad = ok
	bad.WorkerProcesses = "banyak"
	if bad.validate() == nil {
		t.Fatal("worker_processes tidak valid harus ditolak")
	}
}

func TestSaveOptimizationRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := Paths{
		ConfFile:  filepath.Join(dir, "nginx.conf"),
		ErrorLog:  filepath.Join(dir, "error.log"),
		BackupDir: filepath.Join(dir, "backups"),
		Unit:      "nginx",
	}
	m := NewManager(p, nopAudit{}, memSettings{})
	m.mock = true

	if err := os.WriteFile(p.ConfFile, []byte(defaultDevConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	opt, err := m.ReadOptimization()
	if err != nil {
		t.Fatal(err)
	}
	if opt.ClientMaxBodySizeMB != 50 || opt.WorkerConnections != 51200 || !opt.Gzip {
		t.Fatalf("parse awal salah: %+v", opt)
	}

	opt.ClientMaxBodySizeMB = 128
	opt.Gzip = false
	if _, err := m.SaveOptimization(context.Background(), opt); err != nil {
		t.Fatal(err)
	}
	got, err := m.ReadOptimization()
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientMaxBodySizeMB != 128 || got.Gzip {
		t.Fatalf("round-trip gagal: %+v", got)
	}

	entries, _ := os.ReadDir(p.BackupDir)
	if len(entries) != 1 {
		t.Fatalf("backup harus dibuat 1 file, dapat %d", len(entries))
	}
}

func TestSaveConfigRejectsBrokenSyntax(t *testing.T) {
	dir := t.TempDir()
	p := Paths{ConfFile: filepath.Join(dir, "nginx.conf"), ErrorLog: filepath.Join(dir, "e.log"), BackupDir: filepath.Join(dir, "b")}
	m := NewManager(p, nopAudit{}, memSettings{})
	m.mock = true
	original := "http { }\n"
	if err := os.WriteFile(p.ConfFile, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := m.SaveConfig(context.Background(), "http { "); err == nil {
		t.Fatal("konfigurasi rusak harus ditolak")
	}
	data, _ := os.ReadFile(p.ConfFile)
	if string(data) != original {
		t.Fatal("file asli tidak boleh berubah saat validasi gagal")
	}
}
