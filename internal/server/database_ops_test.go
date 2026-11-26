package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fikpanel/internal/database"
	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
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

// TestCredentialDiscoveryParsers memverifikasi ekstraksi kredensial database dari .env, DATABASE_URL, wp-config.php, docker-compose, dan config-db.php
func TestCredentialDiscoveryParsers(t *testing.T) {
	// 1. Parser .env Laravel + DATABASE_URL
	envSample := `
APP_NAME=QRFik
DB_CONNECTION=mysql
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE="qr-fik"
DB_USERNAME=qrfik_user
DB_PASSWORD="Super#Secret!99"
DATABASE_URL=postgresql://pguser:pg%23pass@localhost:5432/analytics_db
`
	creds := database.ParseEnvCredentials(envSample, "/var/www/qr-fik/.env")
	if len(creds) != 2 {
		t.Fatalf("jumlah kredensial .env = %d, ingin 2", len(creds))
	}

	// 2. Parser wp-config.php
	wpSample := `<?php
define( 'DB_NAME', 'wp_blog' );
define( 'DB_USER', 'wp_admin' );
define( 'DB_PASSWORD', 'WpSecret123' );
define( 'DB_HOST', 'localhost:3306' );
`
	wpCred, ok := database.ParseWPConfigCredentials(wpSample, "/var/www/blog/wp-config.php")
	if !ok || wpCred.DBName != "wp_blog" || wpCred.Username != "wp_admin" || wpCred.Password != "WpSecret123" || wpCred.Host != "localhost" {
		t.Fatalf("parsing wp-config.php gagal: %+v (ok=%v)", wpCred, ok)
	}

	// 3. Parser /etc/phpmyadmin/config-db.php
	pmaSample := `<?php
$dbuser='phpmyadmin';
$dbpass='PmaAutoPass42';
$dbname='phpmyadmin';
$dbserver='localhost';
`
	pmaCred, ok := database.ParsePhpMyAdminConfigDB(pmaSample, "/etc/phpmyadmin/config-db.php")
	if !ok || pmaCred.DBName != "phpmyadmin" || pmaCred.Password != "PmaAutoPass42" {
		t.Fatalf("parsing config-db.php gagal: %+v (ok=%v)", pmaCred, ok)
	}

	// 4. End-to-end DiscoverLocalDBCredentials pada direktori sementara (termasuk root_path /public Laravel)
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "qr-fik")
	pubDir := filepath.Join(projDir, "public")
	if err := os.MkdirAll(pubDir, 0755); err != nil {
		t.Fatalf("gagal membuat direktori uji: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, ".env"), []byte(envSample), 0644); err != nil {
		t.Fatalf("gagal menulis .env uji: %v", err)
	}

	found := database.DiscoverLocalDBCredentials([]string{pubDir})
	mysqlCred, ok := found["mysql:qr-fik"]
	if !ok || mysqlCred.Username != "qrfik_user" || mysqlCred.Password != "Super#Secret!99" {
		t.Fatalf("DiscoverLocalDBCredentials gagal membaca mysql:qr-fik dari induk /public: %+v", found)
	}
	pgCred, ok := found["postgresql:analytics_db"]
	if !ok || pgCred.Username != "pguser" || pgCred.Password != "pg#pass" {
		t.Fatalf("DiscoverLocalDBCredentials gagal membaca postgresql:analytics_db: %+v", found)
	}

	// 5. Parser Go MySQL DSN (user:pass@tcp(host:port)/dbname) & Go Postgres key-value DSN
	goMysql, ok := database.ParseDatabaseURL("wa_user:WaPass!77@tcp(127.0.0.1:3306)/wa_bot?charset=utf8mb4&parseTime=True")
	if !ok || goMysql.Engine != "mysql" || goMysql.DBName != "wa_bot" || goMysql.Username != "wa_user" || goMysql.Password != "WaPass!77" {
		t.Fatalf("parsing Go MySQL DSN gagal: %+v (ok=%v)", goMysql, ok)
	}
	goPg, ok := database.ParseDatabaseURL("host=localhost user=vault_usr password=VaultPass99 dbname=fikvault port=5432 sslmode=disable")
	if !ok || goPg.Engine != "postgresql" || goPg.DBName != "fikvault" || goPg.Username != "vault_usr" || goPg.Password != "VaultPass99" {
		t.Fatalf("parsing Go Postgres DSN gagal: %+v (ok=%v)", goPg, ok)
	}

	// 6. Parser ecosystem.config.cjs / config.json terstruktur
	pm2Sample := `module.exports = { apps: [{ name: 'api', env: { DB_DATABASE: 'api_user_crud', DB_USERNAME: 'crud_usr', DB_PASSWORD: 'CrudSecret88' } }] }`
	pm2Creds := database.ParseStructuredConfigCredentials(pm2Sample, "/var/www/api/ecosystem.config.cjs")
	if len(pm2Creds) == 0 || pm2Creds[0].DBName != "api_user_crud" || pm2Creds[0].Password != "CrudSecret88" {
		t.Fatalf("parsing ecosystem.config.cjs gagal: %+v", pm2Creds)
	}
}

// TestVaultLockEnforcement memverifikasi sensor password saat Vault terkunci dan pembukaan via kata sandi administrator
func TestVaultLockEnforcement(t *testing.T) {
	tempDir := t.TempDir()
	st, err := store.Open(filepath.Join(tempDir, "vault_test.db"))
	if err != nil {
		t.Fatalf("gagal membuka store: %v", err)
	}
	defer st.Close()

	usr, err := st.EnsureDevUser("admin_vault", "AdminVaultPass#2026")
	if err != nil || usr == nil {
		t.Fatalf("gagal membuat user uji: %v", err)
	}

	_, err = st.CreateDatabase(context.Background(), store.DatabaseRecord{
		Engine:   "mysql",
		Name:     "kostash",
		Username: "kostash",
		Password: "SuperSecretDBPassword",
		Location: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("gagal membuat DB uji: %v", err)
	}

	srv, err := NewServer(st, metrics.NewHub(), services.NewManager(st.DB()), 8889)
	if err != nil {
		t.Fatalf("gagal inisialisasi server: %v", err)
	}

	cookie := &http.Cookie{Name: "panel_session", Value: "test_vault_session_token_abc"}
	withUser := func(req *http.Request) *http.Request {
		req.AddCookie(cookie)
		return req.WithContext(context.WithValue(req.Context(), userContextKey, usr))
	}

	// 1. Saat Vault terkunci: GET /list harus menyensor password & tetap set has_password = true
	reqList := withUser(httptest.NewRequest(http.MethodGet, "/api/databases/mysql/list", nil))
	reqList.SetPathValue("engine", "mysql")
	wList := httptest.NewRecorder()
	srv.handleListDatabases(wList, reqList)
	var listResp struct {
		VaultUnlocked bool                   `json:"vault_unlocked"`
		Databases     []DatabaseEnrichedItem `json:"databases"`
	}
	if err := json.Unmarshal(wList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("gagal parse list: %v", err)
	}
	if listResp.VaultUnlocked || len(listResp.Databases) != 1 || listResp.Databases[0].Password != "" || !listResp.Databases[0].HasPassword {
		t.Fatalf("password seharusnya disensor saat Vault terkunci: %+v", listResp)
	}

	// 2. Saat Vault terkunci: GET /secret harus menolak dengan HTTP 423 Locked
	reqSecretLocked := withUser(httptest.NewRequest(http.MethodGet, "/api/databases/mysql/kostash/secret", nil))
	reqSecretLocked.SetPathValue("engine", "mysql")
	reqSecretLocked.SetPathValue("name", "kostash")
	wLocked := httptest.NewRecorder()
	srv.handleRevealDatabaseSecret(wLocked, reqSecretLocked)
	if wLocked.Code != http.StatusLocked {
		t.Fatalf("diharapkan HTTP 423 Locked saat Vault terkunci, didapat %d", wLocked.Code)
	}

	// 3. Buka Vault dengan kata sandi admin salah -> HTTP 401
	badBody, _ := json.Marshal(map[string]string{"password": "salah"})
	reqBad := withUser(httptest.NewRequest(http.MethodPost, "/api/auth/vault/unlock/password", bytes.NewReader(badBody)))
	wBad := httptest.NewRecorder()
	srv.handleVaultUnlockPassword(wBad, reqBad)
	if wBad.Code != http.StatusUnauthorized {
		t.Fatalf("kata sandi salah seharusnya 401, didapat %d", wBad.Code)
	}

	// 4. Buka Vault dengan kata sandi admin benar -> HTTP 200
	okBody, _ := json.Marshal(map[string]string{"password": "AdminVaultPass#2026"})
	reqOk := withUser(httptest.NewRequest(http.MethodPost, "/api/auth/vault/unlock/password", bytes.NewReader(okBody)))
	wOk := httptest.NewRecorder()
	srv.handleVaultUnlockPassword(wOk, reqOk)
	if wOk.Code != http.StatusOK {
		t.Fatalf("buka Vault seharusnya 200, didapat %d: %s", wOk.Code, wOk.Body.String())
	}

	// 5. Setelah Vault terbuka: GET /secret mengembalikan password & env_block lengkap
	reqSecretOpen := withUser(httptest.NewRequest(http.MethodGet, "/api/databases/mysql/kostash/secret", nil))
	reqSecretOpen.SetPathValue("engine", "mysql")
	reqSecretOpen.SetPathValue("name", "kostash")
	wOpen := httptest.NewRecorder()
	srv.handleRevealDatabaseSecret(wOpen, reqSecretOpen)
	if wOpen.Code != http.StatusOK || !strings.Contains(wOpen.Body.String(), "SuperSecretDBPassword") {
		t.Fatalf("reveal secret gagal setelah Vault dibuka: %d %s", wOpen.Code, wOpen.Body.String())
	}

	// 6. Kunci kembali Vault secara manual -> GET /secret kembali 423 Locked
	reqLock := withUser(httptest.NewRequest(http.MethodPost, "/api/auth/vault/lock", nil))
	wLock := httptest.NewRecorder()
	srv.handleVaultLock(wLock, reqLock)
	wAfterLock := httptest.NewRecorder()
	srv.handleRevealDatabaseSecret(wAfterLock, reqSecretLocked)
	if wAfterLock.Code != http.StatusLocked {
		t.Fatalf("setelah manual lock seharusnya 423 Locked, didapat %d", wAfterLock.Code)
	}
}
