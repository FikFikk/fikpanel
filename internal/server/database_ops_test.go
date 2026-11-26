package server

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"fikpanel/internal/database"
)

// TestParseEngineVersion memverifikasi ekstraksi versi server database agar tidak tertukar dengan versi pustaka klien
func TestParseEngineVersion(t *testing.T) {
	cases := []struct {
		engine string
		raw    string
		want   string
	}{
		{
			engine: "mysql",
			raw:    "mariadb  Ver 15.1 Distrib 10.11.6-MariaDB, for debian-linux-gnu (aarch64) using  EditLine wrapper",
			want:   "10.11.6-MariaDB",
		},
		{
			engine: "mysql",
			raw:    "mariadb from 11.4.3-MariaDB, client 15.2 for debian-linux-gnu (x86_64)",
			want:   "11.4.3-MariaDB",
		},
		{
			engine: "mysql",
			raw:    "mysqld  Ver 8.0.39-0ubuntu0.24.04.2 for Linux on x86_64 ((Ubuntu))",
			want:   "8.0.39",
		},
		{
			engine: "postgresql",
			raw:    "psql (PostgreSQL) 16.4 (Ubuntu 16.4-1.pgdg24.04+1)",
			want:   "16.4",
		},
		{
			engine: "redis",
			raw:    "Redis server v=7.0.15 sha=00000000:0 malloc=jemalloc-5.3.0 bits=64",
			want:   "7.0.15",
		},
		{
			engine: "mongodb",
			raw:    "db version v7.0.8\nBuild Info: {}",
			want:   "7.0.8",
		},
	}

	for _, tc := range cases {
		got := database.ParseEngineVersion(tc.engine, tc.raw)
		if got != tc.want {
			t.Errorf("ParseEngineVersion(%s) = %q, ingin %q", tc.engine, got, tc.want)
		}
	}
}

// TestParseRedisInfo memverifikasi parsing metrik Redis INFO dan statistik keyspace db0..db15
func TestParseRedisInfo(t *testing.T) {
	sample := `# Server
uptime_in_seconds:86400
# Clients
connected_clients:12
maxclients:10000
# Memory
used_memory:10485760
maxmemory:268435456
# Stats
total_commands_processed:50000
instantaneous_ops_per_sec:34
keyspace_hits:900
keyspace_misses:100
# Keyspace
db0:keys=120,expires=40,avg_ttl=3600000
db2:keys=15,expires=0,avg_ttl=0
`
	tel := ParseRedisInfo(sample)
	if tel.UptimeSec != 86400 || tel.ConnectionsActive != 12 {
		t.Fatalf("uptime/clients tidak sesuai: %+v", tel)
	}
	if tel.TotalKeys != 135 {
		t.Fatalf("total keys = %d, ingin 135", tel.TotalKeys)
	}
	if tel.CacheHitPct < 89.9 || tel.CacheHitPct > 90.1 {
		t.Fatalf("cache hit ratio = %.2f, ingin 90.00", tel.CacheHitPct)
	}
	if len(tel.RedisDatabases) != 2 || tel.RedisDatabases[0].Name != "db0" || tel.RedisDatabases[1].DB != 2 {
		t.Fatalf("daftar keyspace Redis tidak sesuai: %+v", tel.RedisDatabases)
	}
}

// TestBuildTuningConfig memverifikasi profil generator RAM Tuner untuk MySQL, PostgreSQL, dan Redis
func TestBuildTuningConfig(t *testing.T) {
	pMySQL, err := BuildTuningConfig("mysql", "4gb")
	if err != nil {
		t.Fatalf("tuning mysql 4gb gagal: %v", err)
	}
	if !strings.Contains(pMySQL.Content, "innodb_buffer_pool_size = 1536M") {
		t.Fatalf("konten tuning mysql tidak sesuai: %s", pMySQL.Content)
	}

	pPG, err := BuildTuningConfig("postgresql", "2gb")
	if err != nil {
		t.Fatalf("tuning postgresql 2gb gagal: %v", err)
	}
	if !strings.Contains(pPG.Content, "shared_buffers = 512MB") {
		t.Fatalf("konten tuning postgres tidak sesuai: %s", pPG.Content)
	}

	pRedis, err := BuildTuningConfig("redis", "8gb")
	if err != nil {
		t.Fatalf("tuning redis 8gb gagal: %v", err)
	}
	if pRedis.Summary["maxmemory"] != "2gb" {
		t.Fatalf("maxmemory redis = %s, ingin 2gb", pRedis.Summary["maxmemory"])
	}

	if _, err := BuildTuningConfig("mysql", "999gb"); err == nil {
		t.Fatalf("profil tidak valid seharusnya mengembalikan error")
	}
}

// TestSignS3PutRequest memverifikasi header tanda tangan AWS Signature Version 4 untuk S3 / MinIO / R2
func TestSignS3PutRequest(t *testing.T) {
	targetURL, err := buildS3ObjectURL("https://minio.example.com", "backups", "ap-southeast-1", "fikpanel-backups/test.sql.gz")
	if err != nil {
		t.Fatalf("buildS3ObjectURL gagal: %v", err)
	}
	if targetURL != "https://minio.example.com/backups/fikpanel-backups/test.sql.gz" {
		t.Fatalf("URL S3 tidak sesuai: %s", targetURL)
	}

	req, err := http.NewRequest(http.MethodPut, targetURL, nil)
	if err != nil {
		t.Fatalf("gagal membuat request: %v", err)
	}
	fixedTime := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	emptyHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	signS3PutRequest(req, "AKIAEXAMPLE", "SECRETKEYEXAMPLE", "ap-southeast-1", emptyHash, fixedTime)

	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=AKIAEXAMPLE/20261126/ap-southeast-1/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=") {
		t.Fatalf("header Authorization SigV4 tidak valid: %s", auth)
	}
	if req.Header.Get("x-amz-date") != "20261126T120000Z" {
		t.Fatalf("x-amz-date tidak valid: %s", req.Header.Get("x-amz-date"))
	}
}

// TestValidateBackupFileName memastikan pencegahan path traversal pada unduh/hapus backup
func TestValidateBackupFileName(t *testing.T) {
	if _, _, err := validateBackupFileName("../etc/passwd"); err == nil {
		t.Fatal("path traversal ../ seharusnya ditolak")
	}
	if _, _, err := validateBackupFileName("sub/file.sql.gz"); err == nil {
		t.Fatal("path slash seharusnya ditolak")
	}
	if _, _, err := validateBackupFileName("db_mysql_test.sh"); err == nil {
		t.Fatal("ekstensi .sh seharusnya ditolak")
	}
	base, _, err := validateBackupFileName("db_mysql_kostash_20261126-120000.sql.gz")
	if err != nil || base != "db_mysql_kostash_20261126-120000.sql.gz" {
		t.Fatalf("berkas backup valid gagal divalidasi: %v", err)
	}
}
