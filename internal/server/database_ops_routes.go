package server

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/database"
	"fikpanel/internal/store"
)

const fikpanelBackupDir = "/var/backups/fikpanel"

// DatabaseEnrichedItem memperluas DatabaseRecord dengan metadata proteksi sistem, kuota, dan status kata sandi vault
type DatabaseEnrichedItem struct {
	store.DatabaseRecord
	IsSystem    bool `json:"is_system"`
	QuotaMB     int  `json:"quota_mb"`
	HasPassword bool `json:"has_password"`
}

// DatabaseBackupFileItem merepresentasikan satu arsip cadangan SQL milik database tertentu
type DatabaseBackupFileItem struct {
	FileName  string `json:"file_name"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt int64  `json:"created_at"`
}

// DatabaseTableInfo merepresentasikan metadata satu tabel pada SQL Studio
type DatabaseTableInfo struct {
	Name       string `json:"name"`
	Engine     string `json:"engine"`
	Rows       int64  `json:"rows"`
	SizeBytes  int64  `json:"size_bytes"`
	Collation  string `json:"collation"`
}

// DatabaseProcessItem merepresentasikan satu sesi/kueri aktif pada Processlist
type DatabaseProcessItem struct {
	PID     int64  `json:"pid"`
	User    string `json:"user"`
	Host    string `json:"host"`
	DB      string `json:"db"`
	Command string `json:"command"`
	TimeSec int    `json:"time_sec"`
	State   string `json:"state"`
	Query   string `json:"query"`
}

// RedisDBSpaceInfo menyimpan ringkasan keyspace per index database Redis (db0..db15)
type RedisDBSpaceInfo struct {
	DB      int   `json:"db"`
	Name    string `json:"name"`
	Keys    int64 `json:"keys"`
	Expires int64 `json:"expires"`
}

// DatabaseTelemetry menyimpan metrik performa langsung dari engine database
type DatabaseTelemetry struct {
	Engine            string             `json:"engine"`
	Status            string             `json:"status"`
	UptimeSec         int64              `json:"uptime_sec"`
	ConnectionsActive int64              `json:"connections_active"`
	ConnectionsMax    int64              `json:"connections_max"`
	QueriesTotal      int64              `json:"queries_total"`
	QPS               float64            `json:"qps"`
	SlowQueries       int64              `json:"slow_queries"`
	CacheHitPct       float64            `json:"cache_hit_pct"`
	MemoryUsedBytes   int64              `json:"memory_used_bytes"`
	MemoryLimitBytes  int64              `json:"memory_limit_bytes"`
	ThreadsRunning    int64              `json:"threads_running"`
	TotalKeys         int64              `json:"total_keys"`
	RedisDatabases    []RedisDBSpaceInfo `json:"redis_databases,omitempty"`
}

// RedisKeySummary merepresentasikan ringkasan satu kunci pada Redis Key Browser
type RedisKeySummary struct {
	Key  string `json:"key"`
	Type string `json:"type"`
	TTL  int64  `json:"ttl"`
}

// backupDirPath mengembalikan direktori backup aktif (menggunakan temp lokal saat bukan Linux)
func backupDirPath() string {
	if runtime.GOOS != "linux" {
		return filepath.Join(os.TempDir(), "fikpanel-backups")
	}
	return fikpanelBackupDir
}

// isBackupFileForDB memeriksa apakah sebuah nama berkas merupakan arsip cadangan dari database tertentu
func isBackupFileForDB(fileName, engine, dbName string) bool {
	lower := strings.ToLower(fileName)
	if !strings.HasSuffix(lower, ".sql") && !strings.HasSuffix(lower, ".sql.gz") && !strings.HasSuffix(lower, ".tar.gz") {
		return false
	}
	prefixExact := fmt.Sprintf("db_%s_%s_", strings.ToLower(engine), dbName)
	prefixLegacy := fmt.Sprintf("database_%s_", dbName)
	return strings.HasPrefix(fileName, prefixExact) || strings.HasPrefix(fileName, prefixLegacy)
}

// listDbBackupFiles memindai berkas arsip cadangan milik database tertentu dari disk
func listDbBackupFiles(engine, dbName string) []DatabaseBackupFileItem {
	dir := backupDirPath()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []DatabaseBackupFileItem{}
	}
	var items []DatabaseBackupFileItem
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isBackupFileForDB(name, engine, dbName) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, DatabaseBackupFileItem{
			FileName:  name,
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime().Unix(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt > items[j].CreatedAt
	})
	if items == nil {
		return []DatabaseBackupFileItem{}
	}
	return items
}

// validateBackupFileName memastikan nama berkas aman tanpa path traversal
func validateBackupFileName(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.Contains(trimmed, "..") || strings.ContainsAny(trimmed, `/\`) {
		return "", "", fmt.Errorf("nama berkas backup tidak valid")
	}
	base := filepath.Base(trimmed)
	lower := strings.ToLower(base)
	if !strings.HasSuffix(lower, ".sql") && !strings.HasSuffix(lower, ".sql.gz") && !strings.HasSuffix(lower, ".tar.gz") && !strings.HasSuffix(lower, ".gz") {
		return "", "", fmt.Errorf("ekstensi berkas backup tidak didukung")
	}
	fullPath := filepath.Join(backupDirPath(), base)
	return base, fullPath, nil
}

// collectProjectDirsForCredentialScan mengumpulkan seluruh direktori proyek terdaftar untuk pemindaian kredensial DB
func (s *Server) collectProjectDirsForCredentialScan(ctx context.Context) []string {
	var dirs []string
	if sitesList, err := s.store.ListSites(ctx); err == nil {
		for _, st := range sitesList {
			if p := strings.TrimSpace(st.RootPath); p != "" {
				dirs = append(dirs, p)
			}
		}
	}
	if rawRoots, err := s.store.GetSetting(ctx, "discovery.custom_roots"); err == nil && rawRoots != "" {
		var customRoots []string
		if err := json.Unmarshal([]byte(rawRoots), &customRoots); err == nil {
			dirs = append(dirs, customRoots...)
		}
	}
	return dirs
}

// recoverMissingDBCredentials mengisi username/password yang masih kosong di SQLite dari berkas .env/config proyek tanpa mengubah server DB
func (s *Server) recoverMissingDBCredentials(ctx context.Context, engine string, records []store.DatabaseRecord) int {
	needScan := false
	for _, rec := range records {
		if strings.TrimSpace(rec.Password) == "" {
			needScan = true
			break
		}
	}
	if !needScan {
		return 0
	}

	creds := database.DiscoverLocalDBCredentials(s.collectProjectDirsForCredentialScan(ctx))
	if len(creds) == 0 {
		return 0
	}

	recovered := 0
	engKey := strings.ToLower(engine)
	for i := range records {
		if strings.TrimSpace(records[i].Password) != "" {
			continue
		}
		lookupKey := engKey + ":" + strings.ToLower(strings.TrimSpace(records[i].Name))
		c, ok := creds[lookupKey]
		if !ok || strings.TrimSpace(c.Password) == "" {
			continue
		}
		user := strings.TrimSpace(c.Username)
		if user == "" {
			user = records[i].Username
		}
		if user == "" {
			user = records[i].Name
		}
		loc := strings.TrimSpace(c.Host)
		if loc == "" {
			loc = records[i].Location
		}
		if err := s.store.UpdateDatabaseCredentials(ctx, engine, records[i].Name, user, c.Password, loc); err == nil {
			records[i].Username = user
			records[i].Password = c.Password
			records[i].Location = loc
			recovered++
		}
	}
	return recovered
}

// handleSyncDatabases memindai database fisik pada server host, menyinkronkannya ke panel, dan otomatis mendeteksi kredensial dari .env/config proyek
func (s *Server) handleSyncDatabases(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	if engine == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parameter engine wajib diisi"})
		return
	}

	hostList, err := database.ListHostDatabases(r.Context(), engine)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	existingList, err := s.store.ListDatabases(r.Context(), engine)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membaca daftar database panel"})
		return
	}

	// Pindai kredensial dari .env, wp-config.php, docker-compose.yml, dan config-db.php di server
	creds := database.DiscoverLocalDBCredentials(s.collectProjectDirsForCredentialScan(r.Context()))
	engKey := strings.ToLower(engine)

	existingMap := make(map[string]store.DatabaseRecord, len(existingList))
	for _, rec := range existingList {
		existingMap[rec.Name] = rec
	}

	discovered := 0
	updated := 0
	credsRecovered := 0
	for _, h := range hostList {
		c, hasCred := creds[engKey+":"+strings.ToLower(strings.TrimSpace(h.Name))]
		if rec, ok := existingMap[h.Name]; !ok {
			owner := h.Owner
			pass := ""
			loc := "localhost"
			if hasCred {
				if strings.TrimSpace(c.Username) != "" {
					owner = c.Username
				}
				if strings.TrimSpace(c.Password) != "" {
					pass = c.Password
					credsRecovered++
				}
				if strings.TrimSpace(c.Host) != "" {
					loc = c.Host
				}
			}
			if owner == "" {
				owner = h.Name
			}
			_, createErr := s.store.CreateDatabase(r.Context(), store.DatabaseRecord{
				Engine:    engine,
				Name:      h.Name,
				Username:  owner,
				Password:  pass,
				SizeBytes: h.SizeBytes,
				Location:  loc,
				Note:      "",
			})
			if createErr == nil {
				discovered++
			}
		} else {
			if err := s.store.UpdateDatabaseSize(r.Context(), engine, h.Name, h.SizeBytes); err == nil {
				updated++
			}
			// Isi otomatis password & username yang sebelumnya kosong dari .env/config proyek tanpa mengubah server DB
			if strings.TrimSpace(rec.Password) == "" && hasCred && strings.TrimSpace(c.Password) != "" {
				user := strings.TrimSpace(c.Username)
				if user == "" {
					user = rec.Username
				}
				loc := strings.TrimSpace(c.Host)
				if loc == "" {
					loc = rec.Location
				}
				if err := s.store.UpdateDatabaseCredentials(r.Context(), engine, h.Name, user, c.Password, loc); err == nil {
					credsRecovered++
				}
			}
		}
	}

	// Pulihkan juga kredensial database eksisting yang sudah ada di daftar panel bila belum tercakup hostList
	if refreshed, err := s.store.ListDatabases(r.Context(), engine); err == nil {
		credsRecovered += s.recoverMissingDBCredentials(r.Context(), engine, refreshed)
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.sync", engine, true,
		fmt.Sprintf("Sinkronisasi host %s: %d baru, %d kredensial terdeteksi dari konfigurasi proyek, %d diperbarui", engine, discovered, credsRecovered, updated), nil)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                    true,
		"discovered":            discovered,
		"updated":               updated,
		"credentials_recovered": credsRecovered,
		"total_host":            len(hostList),
		"message":               fmt.Sprintf("Sinkronisasi selesai: %d DB baru, %d kredensial terbaca otomatis dari proyek, %d diperbarui", discovered, credsRecovered, updated),
	})
}

// handleUpdateRootPassword mengubah kata sandi administrator utama engine database secara riil
func (s *Server) handleUpdateRootPassword(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	newPass := strings.TrimSpace(req.Password)
	if len(newPass) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kata sandi root minimal 6 karakter"})
		return
	}

	if err := database.UpdateRootPassword(r.Context(), engine, newPass); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.root_password", engine, true,
		fmt.Sprintf("Memperbarui kata sandi root administrator untuk engine %s", engine), nil)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Kata sandi root untuk %s berhasil diperbarui di server", engine),
	})
}

// handleUpdateDatabaseQuota menyimpan batas kuota kapasitas per database ke pengaturan panel
func (s *Server) handleUpdateDatabaseQuota(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	name := strings.TrimSpace(r.PathValue("name"))
	if engine == "" || name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parameter engine dan nama database wajib diisi"})
		return
	}
	var req struct {
		QuotaMB int `json:"quota_mb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if req.QuotaMB < 0 {
		req.QuotaMB = 0
	}

	key := fmt.Sprintf("db_quota:%s:%s", engine, name)
	if err := s.store.SetSetting(r.Context(), key, strconv.Itoa(req.QuotaMB)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menyimpan kuota database"})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.quota", fmt.Sprintf("%s:%s", engine, name), true,
		fmt.Sprintf("Mengatur kuota database %s menjadi %d MB", name, req.QuotaMB), nil)

	msg := fmt.Sprintf("Kuota peringatan database %s diatur ke %d MB", name, req.QuotaMB)
	if req.QuotaMB == 0 {
		msg = fmt.Sprintf("Batas kuota database %s dinonaktifkan (tanpa batas)", name)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"quota_mb": req.QuotaMB,
		"message":  msg,
	})
}

// handleDumpDatabase melakukan dump terkompresi (.sql.gz) satu klik untuk database target
func (s *Server) handleDumpDatabase(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	name := strings.TrimSpace(r.PathValue("name"))
	if !database.IsValidIdentifier(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nama database tidak valid"})
		return
	}

	dir := backupDirPath()
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("gagal menyiapkan direktori backup: %v", err)})
		return
	}

	ts := time.Now().Format("20060102-150405")
	fileName := fmt.Sprintf("db_%s_%s_%s.sql.gz", engine, name, ts)
	targetPath := filepath.Join(dir, fileName)

	dumpCtx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	if runtime.GOOS != "linux" {
		// Lingkungan pengembangan lokal: buat berkas .sql.gz valid agar alur unduh & restore dapat diuji
		f, err := os.Create(targetPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		gz := gzip.NewWriter(f)
		_, _ = fmt.Fprintf(gz, "-- FikPanel SQL Dump (%s / %s) dibuat pada %s\nSELECT 1;\n", engine, name, time.Now().Format(time.RFC3339))
		_ = gz.Close()
		_ = f.Close()
	} else {
		if err := dumpSingleDatabaseToGzip(dumpCtx, engine, name, targetPath); err != nil {
			_ = os.Remove(targetPath)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	var fileSize int64
	if fi, err := os.Stat(targetPath); err == nil {
		fileSize = fi.Size()
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.dump", fmt.Sprintf("%s:%s", engine, name), true,
		fmt.Sprintf("Membuat cadangan %s (%d bytes)", fileName, fileSize), nil)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"file_name":  fileName,
		"size_bytes": fileSize,
		"created_at": time.Now().Unix(),
		"message":    fmt.Sprintf("Cadangan database %s berhasil dibuat (%s)", name, fileName),
	})
}

// dumpSingleDatabaseToGzip menjalankan mysqldump / pg_dump dan menulis langsung melalui gzip writer Go
func dumpSingleDatabaseToGzip(ctx context.Context, engine, dbName, targetPath string) error {
	var cmd *exec.Cmd
	switch strings.ToLower(engine) {
	case "mysql":
		args := []string{}
		if _, err := os.Stat("/etc/mysql/debian.cnf"); err == nil {
			args = append(args, "--defaults-file=/etc/mysql/debian.cnf")
		} else {
			args = append(args, "-u", "root")
		}
		args = append(args, "--single-transaction", "--routines", "--triggers", "--quick")
		if dbName == "*" || dbName == "" {
			args = append(args, "--all-databases")
		} else {
			args = append(args, dbName)
		}
		dumpBin := "mysqldump"
		if _, err := exec.LookPath("mariadb-dump"); err == nil {
			dumpBin = "mariadb-dump"
		}
		cmd = exec.CommandContext(ctx, dumpBin, args...)

	case "postgresql", "postgres":
		pgArgs := []string{"pg_dump", "--no-owner", "--no-acl", dbName}
		if dbName == "*" || dbName == "" {
			pgArgs = []string{"pg_dumpall"}
		}
		if _, err := exec.LookPath("runuser"); err == nil {
			cmd = exec.CommandContext(ctx, "runuser", append([]string{"-u", "postgres", "--"}, pgArgs...)...)
		} else if _, err := exec.LookPath("sudo"); err == nil {
			cmd = exec.CommandContext(ctx, "sudo", append([]string{"-u", "postgres"}, pgArgs...)...)
		} else {
			cmd = exec.CommandContext(ctx, pgArgs[0], append([]string{"-U", "postgres"}, pgArgs[1:]...)...)
		}

	default:
		return fmt.Errorf("dump SQL tidak didukung untuk engine %s", engine)
	}

	outFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("gagal membuat berkas backup: %w", err)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	var stderrBuf bytes.Buffer
	cmd.Stdout = gzWriter
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		_ = gzWriter.Close()
		errText := strings.TrimSpace(stderrBuf.String())
		if errText == "" {
			errText = err.Error()
		}
		return fmt.Errorf("proses dump %s gagal: %s", engine, errText)
	}
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("gagal menutup stream gzip: %w", err)
	}
	return nil
}

// handleListDbBackups mengembalikan daftar arsip cadangan SQL milik satu database
func (s *Server) handleListDbBackups(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	name := strings.TrimSpace(r.PathValue("name"))
	items := listDbBackupFiles(engine, name)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"backups": items,
	})
}

// handleDownloadDbBackup melayani pengunduhan berkas arsip cadangan dari direktori backup panel
func (s *Server) handleDownloadDbBackup(w http.ResponseWriter, r *http.Request) {
	base, fullPath, err := validateBackupFileName(r.URL.Query().Get("file"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if _, err := os.Stat(fullPath); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "berkas backup tidak ditemukan"})
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, base))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, fullPath)
}

// handleDeleteDbBackup menghapus satu berkas arsip cadangan dari disk
func (s *Server) handleDeleteDbBackup(w http.ResponseWriter, r *http.Request) {
	base, fullPath, err := validateBackupFileName(r.URL.Query().Get("file"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := os.Remove(fullPath); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "gagal menghapus berkas backup: berkas tidak ditemukan"})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.backup_delete", base, true, "Menghapus arsip cadangan "+base, nil)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Arsip %s berhasil dihapus", base),
	})
}

// handleRestoreDatabase memulihkan database dari arsip backup tersimpan atau berkas unggahan .sql/.sql.gz
func (s *Server) handleRestoreDatabase(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	name := strings.TrimSpace(r.PathValue("name"))
	if !database.IsValidIdentifier(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nama database tidak valid"})
		return
	}

	var reader io.ReadCloser
	var sourceLabel string
	var isGzip bool

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Batasi ukuran unggahan impor hingga 128 MB
		if err := r.ParseMultipartForm(128 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ukuran berkas impor melampaui batas 128 MB atau format multipart tidak valid"})
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "berkas .sql atau .sql.gz wajib dilampirkan"})
			return
		}
		defer file.Close()
		sourceLabel = header.Filename
		isGzip = strings.HasSuffix(strings.ToLower(header.Filename), ".gz")
		reader = file
	} else {
		var req struct {
			FileName string `json:"file_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
			return
		}
		base, fullPath, err := validateBackupFileName(req.FileName)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		f, err := os.Open(fullPath)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "berkas cadangan tidak ditemukan di server"})
			return
		}
		defer f.Close()
		sourceLabel = base
		isGzip = strings.HasSuffix(strings.ToLower(base), ".gz")
		reader = f
	}

	var sqlStream io.Reader = reader
	if isGzip {
		gzReader, err := gzip.NewReader(reader)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("berkas gzip rusak atau tidak valid: %v", err)})
			return
		}
		defer gzReader.Close()
		sqlStream = gzReader
	}

	if runtime.GOOS != "linux" {
		// Validasi stream dapat dibaca penuh di lingkungan dev
		n, err := io.Copy(io.Discard, io.LimitReader(sqlStream, 10<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"message": fmt.Sprintf("Simulasi pemulihan %s ke database %s berhasil (%d bytes dibaca)", sourceLabel, name, n),
		})
		return
	}

	restoreCtx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	switch engine {
	case "mysql":
		bin := "mysql"
		if _, err := exec.LookPath("mariadb"); err == nil {
			bin = "mariadb"
		}
		args := []string{}
		if _, err := os.Stat("/etc/mysql/debian.cnf"); err == nil {
			args = append(args, "--defaults-file=/etc/mysql/debian.cnf")
		} else {
			args = append(args, "-u", "root")
		}
		args = append(args, name)
		cmd = exec.CommandContext(restoreCtx, bin, args...)

	case "postgresql", "postgres":
		pgArgs := []string{"psql", "-v", "ON_ERROR_STOP=0", "-d", name}
		if _, err := exec.LookPath("runuser"); err == nil {
			cmd = exec.CommandContext(restoreCtx, "runuser", append([]string{"-u", "postgres", "--"}, pgArgs...)...)
		} else if _, err := exec.LookPath("sudo"); err == nil {
			cmd = exec.CommandContext(restoreCtx, "sudo", append([]string{"-u", "postgres"}, pgArgs...)...)
		} else {
			cmd = exec.CommandContext(restoreCtx, "psql", "-U", "postgres", "-d", name)
		}

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("impor/restore SQL belum didukung untuk engine %s", engine)})
		return
	}

	cmd.Stdin = sqlStream
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderrBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("proses restore/impor gagal: %s", msg)})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.restore", fmt.Sprintf("%s:%s", engine, name), true,
		fmt.Sprintf("Memulihkan/impor %s ke database %s", sourceLabel, name), nil)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Berkas %s berhasil diimpor ke database %s", sourceLabel, name),
	})
}

// handleListDatabaseTables mengambil daftar tabel, jumlah baris, dan ukuran nyata untuk SQL Studio
func (s *Server) handleListDatabaseTables(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	name := strings.TrimSpace(r.PathValue("name"))
	if !database.IsValidIdentifier(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nama database tidak valid"})
		return
	}

	if runtime.GOOS != "linux" {
		// Contoh tabel simulasi untuk pengembangan antarmuka SQL Studio
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok": true,
			"tables": []DatabaseTableInfo{
				{Name: "users", Engine: "InnoDB", Rows: 148, SizeBytes: 65536, Collation: "utf8mb4_unicode_ci"},
				{Name: "migrations", Engine: "InnoDB", Rows: 24, SizeBytes: 16384, Collation: "utf8mb4_unicode_ci"},
				{Name: "sessions", Engine: "InnoDB", Rows: 512, SizeBytes: 131072, Collation: "utf8mb4_unicode_ci"},
				{Name: "jobs", Engine: "InnoDB", Rows: 19, SizeBytes: 32768, Collation: "utf8mb4_unicode_ci"},
			},
		})
		return
	}

	qCtx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	var tables []DatabaseTableInfo
	switch engine {
	case "mysql":
		q := fmt.Sprintf(
			`SELECT TABLE_NAME, COALESCE(ENGINE, '-'), COALESCE(TABLE_ROWS, 0), COALESCE(DATA_LENGTH + INDEX_LENGTH, 0), COALESCE(TABLE_COLLATION, '-')
			 FROM information_schema.TABLES WHERE TABLE_SCHEMA = '%s' ORDER BY TABLE_NAME ASC;`,
			strings.ReplaceAll(name, "'", "''"),
		)
		out, err := database.RunMysqlQueryOutput(qCtx, "", q)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			cols := strings.Split(scanner.Text(), "\t")
			if len(cols) < 5 {
				continue
			}
			rowsCount, _ := strconv.ParseInt(strings.TrimSpace(cols[2]), 10, 64)
			sizeBytes, _ := strconv.ParseInt(strings.TrimSpace(cols[3]), 10, 64)
			tables = append(tables, DatabaseTableInfo{
				Name:      strings.TrimSpace(cols[0]),
				Engine:    strings.TrimSpace(cols[1]),
				Rows:      rowsCount,
				SizeBytes: sizeBytes,
				Collation: strings.TrimSpace(cols[4]),
			})
		}

	case "postgresql", "postgres":
		q := `SELECT relname, 'PostgreSQL', COALESCE(n_live_tup, 0), COALESCE(pg_total_relation_size(relid), 0), 'UTF8'
		      FROM pg_stat_user_tables ORDER BY relname ASC;`
		out, err := database.RunPostgresQueryOutput(qCtx, name, q, false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			cols := strings.Split(scanner.Text(), "\t")
			if len(cols) < 5 {
				continue
			}
			rowsCount, _ := strconv.ParseInt(strings.TrimSpace(cols[2]), 10, 64)
			sizeBytes, _ := strconv.ParseInt(strings.TrimSpace(cols[3]), 10, 64)
			tables = append(tables, DatabaseTableInfo{
				Name:      strings.TrimSpace(cols[0]),
				Engine:    strings.TrimSpace(cols[1]),
				Rows:      rowsCount,
				SizeBytes: sizeBytes,
				Collation: strings.TrimSpace(cols[4]),
			})
		}

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "SQL Studio hanya tersedia untuk MySQL/MariaDB dan PostgreSQL"})
		return
	}

	if tables == nil {
		tables = []DatabaseTableInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"tables": tables,
	})
}

// handleOptimizeDatabaseTables menjalankan OPTIMIZE TABLE / VACUUM ANALYZE untuk mengurangi fragmentasi tabel
func (s *Server) handleOptimizeDatabaseTables(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	name := strings.TrimSpace(r.PathValue("name"))
	if !database.IsValidIdentifier(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nama database tidak valid"})
		return
	}
	var req struct {
		Table string `json:"table"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	table := strings.TrimSpace(req.Table)
	if table != "" && !database.IsValidIdentifier(table) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nama tabel tidak valid"})
		return
	}

	if runtime.GOOS != "linux" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"message": fmt.Sprintf("Optimasi tabel pada %s selesai (mode lokal)", name),
		})
		return
	}

	optCtx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	switch engine {
	case "mysql":
		if table != "" {
			if _, err := database.RunMysqlQueryOutput(optCtx, name, fmt.Sprintf("OPTIMIZE TABLE `%s`;", table)); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		} else {
			// Optimasi seluruh tabel di skema target
			out, err := database.RunMysqlQueryOutput(optCtx, "", fmt.Sprintf("SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = '%s' AND TABLE_TYPE = 'BASE TABLE';", strings.ReplaceAll(name, "'", "''")))
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			var stmts []string
			scanner := bufio.NewScanner(strings.NewReader(out))
			for scanner.Scan() {
				t := strings.TrimSpace(scanner.Text())
				if t != "" && database.IsValidIdentifier(t) {
					stmts = append(stmts, fmt.Sprintf("OPTIMIZE TABLE `%s`;", t))
				}
			}
			if len(stmts) > 0 {
				if _, err := database.RunMysqlQueryOutput(optCtx, name, strings.Join(stmts, " ")); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
			}
		}

	case "postgresql", "postgres":
		sqlStmt := "VACUUM ANALYZE;"
		if table != "" {
			sqlStmt = fmt.Sprintf(`VACUUM ANALYZE "%s";`, table)
		}
		if _, err := database.RunPostgresQueryOutput(optCtx, name, sqlStmt, false); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	targetDesc := name
	if table != "" {
		targetDesc = name + "." + table
	}
	_ = s.store.RecordAudit(r.Context(), "user", "database.optimize", targetDesc, true, "Menjalankan optimasi tabel "+targetDesc, nil)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Optimasi %s berhasil dijalankan", targetDesc),
	})
}

// handleExecuteDatabaseQuery menjalankan query SQL interaktif pada database MySQL atau PostgreSQL target
func (s *Server) handleExecuteDatabaseQuery(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	name := strings.TrimSpace(r.PathValue("name"))
	if !database.IsValidIdentifier(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nama database tidak valid"})
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	q := strings.TrimSpace(req.Query)
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kueri SQL tidak boleh kosong"})
		return
	}

	start := time.Now()
	if runtime.GOOS != "linux" {
		elapsed := float64(time.Since(start).Microseconds()) / 1000.0
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":        true,
			"columns":   []string{"id", "name", "email", "created_at"},
			"rows":      [][]string{{"1", "Admin FikPanel", "admin@fikfikk.my.id", time.Now().Format("2006-01-02 15:04:05")}},
			"row_count": 1,
			"truncated": false,
			"time_ms":   elapsed,
			"note":      "Hasil simulasi lingkungan pengembangan lokal",
		})
		return
	}

	qCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	var rawOut string
	var err error
	switch engine {
	case "mysql":
		rawOut, err = database.RunMysqlQueryWithHeaders(qCtx, name, q)
	case "postgresql", "postgres":
		rawOut, err = database.RunPostgresQueryOutput(qCtx, name, q, true)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "SQL Query Runner hanya mendukung MySQL/MariaDB dan PostgreSQL"})
		return
	}

	elapsed := float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":      false,
			"error":   err.Error(),
			"time_ms": elapsed,
		})
		return
	}

	// Parsing hasil TSV dengan batas aman 500 baris
	const maxRows = 500
	var columns []string
	var rows [][]string
	truncated := false
	note := ""

	trimmed := strings.TrimSpace(rawOut)
	if trimmed != "" {
		lines := strings.Split(strings.ReplaceAll(trimmed, "\r\n", "\n"), "\n")
		// Khusus PostgreSQL, baris terakhir kadang berupa "(N rows)" atau "UPDATE N"
		if engine == "postgresql" || engine == "postgres" {
			last := strings.TrimSpace(lines[len(lines)-1])
			if strings.HasPrefix(last, "(") && strings.HasSuffix(last, "rows)") || strings.HasSuffix(last, "row)") {
				lines = lines[:len(lines)-1]
			}
		}
		if len(lines) > 0 {
			if len(lines) == 1 && !strings.Contains(lines[0], "\t") && (strings.HasPrefix(lines[0], "INSERT") || strings.HasPrefix(lines[0], "UPDATE") || strings.HasPrefix(lines[0], "DELETE") || strings.HasPrefix(lines[0], "CREATE") || strings.HasPrefix(lines[0], "ALTER")) {
				note = lines[0]
			} else {
				columns = strings.Split(lines[0], "\t")
				for i := 1; i < len(lines); i++ {
					if len(rows) >= maxRows {
						truncated = true
						break
					}
					rows = append(rows, strings.Split(lines[i], "\t"))
				}
			}
		}
	} else {
		note = "Kueri berhasil dieksekusi (tidak ada baris hasil dikembalikan)."
	}

	if columns == nil {
		columns = []string{}
	}
	if rows == nil {
		rows = [][]string{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"columns":   columns,
		"rows":      rows,
		"row_count": len(rows),
		"truncated": truncated,
		"time_ms":   elapsed,
		"note":      note,
	})
}

// ParseRedisInfo mem-parsing output mentah `redis-cli INFO` menjadi objek telemetri terstruktur
func ParseRedisInfo(raw string) DatabaseTelemetry {
	tel := DatabaseTelemetry{
		Engine: "redis",
		Status: "running",
	}
	var hits, misses int64
	var totalKeys int64

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])

		switch k {
		case "uptime_in_seconds":
			tel.UptimeSec, _ = strconv.ParseInt(v, 10, 64)
		case "connected_clients":
			tel.ConnectionsActive, _ = strconv.ParseInt(v, 10, 64)
		case "maxclients":
			tel.ConnectionsMax, _ = strconv.ParseInt(v, 10, 64)
		case "total_commands_processed":
			tel.QueriesTotal, _ = strconv.ParseInt(v, 10, 64)
		case "instantaneous_ops_per_sec":
			tel.QPS, _ = strconv.ParseFloat(v, 64)
		case "used_memory":
			tel.MemoryUsedBytes, _ = strconv.ParseInt(v, 10, 64)
		case "maxmemory":
			tel.MemoryLimitBytes, _ = strconv.ParseInt(v, 10, 64)
		case "keyspace_hits":
			hits, _ = strconv.ParseInt(v, 10, 64)
		case "keyspace_misses":
			misses, _ = strconv.ParseInt(v, 10, 64)
		default:
			if strings.HasPrefix(k, "db") && len(k) > 2 {
				dbIdx, err := strconv.Atoi(k[2:])
				if err == nil {
					info := RedisDBSpaceInfo{DB: dbIdx, Name: k}
					for _, part := range strings.Split(v, ",") {
						p := strings.SplitN(part, "=", 2)
						if len(p) != 2 {
							continue
						}
						switch p[0] {
						case "keys":
							info.Keys, _ = strconv.ParseInt(p[1], 10, 64)
						case "expires":
							info.Expires, _ = strconv.ParseInt(p[1], 10, 64)
						}
					}
					totalKeys += info.Keys
					tel.RedisDatabases = append(tel.RedisDatabases, info)
				}
			}
		}
	}

	tel.TotalKeys = totalKeys
	if hits+misses > 0 {
		tel.CacheHitPct = float64(hits) / float64(hits+misses) * 100.0
	} else {
		tel.CacheHitPct = 100.0
	}
	if tel.ConnectionsMax == 0 {
		tel.ConnectionsMax = 10000
	}
	sort.Slice(tel.RedisDatabases, func(i, j int) bool {
		return tel.RedisDatabases[i].DB < tel.RedisDatabases[j].DB
	})
	return tel
}

// handleDatabaseTelemetry mengembalikan metrik kinerja real-time untuk engine aktif
func (s *Server) handleDatabaseTelemetry(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	if runtime.GOOS != "linux" {
		sim := DatabaseTelemetry{
			Engine:            engine,
			Status:            "running",
			UptimeSec:         184320,
			ConnectionsActive: 14,
			ConnectionsMax:    150,
			QueriesTotal:      842910,
			QPS:               42.6,
			SlowQueries:       2,
			CacheHitPct:       99.4,
			MemoryUsedBytes:   168 * 1024 * 1024,
			MemoryLimitBytes:  512 * 1024 * 1024,
			ThreadsRunning:    3,
		}
		if engine == "redis" {
			sim.TotalKeys = 1284
			sim.ConnectionsMax = 10000
			sim.MemoryUsedBytes = 24 * 1024 * 1024
			sim.MemoryLimitBytes = 256 * 1024 * 1024
			sim.RedisDatabases = []RedisDBSpaceInfo{
				{DB: 0, Name: "db0", Keys: 1120, Expires: 840},
				{DB: 1, Name: "db1", Keys: 164, Expires: 164},
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "telemetry": sim})
		return
	}

	tCtx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()

	tel := DatabaseTelemetry{Engine: engine, Status: "running"}

	switch engine {
	case "mysql":
		q := `SHOW GLOBAL STATUS WHERE Variable_name IN (
			'Uptime','Threads_connected','Threads_running','Questions','Slow_queries',
			'Innodb_buffer_pool_read_requests','Innodb_buffer_pool_reads',
			'Innodb_buffer_pool_bytes_data'
		);`
		out, err := database.RunMysqlQueryOutput(tCtx, "", q)
		if err != nil {
			tel.Status = "offline"
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "telemetry": tel})
			return
		}
		kv := map[string]int64{}
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			parts := strings.Split(scanner.Text(), "\t")
			if len(parts) >= 2 {
				val, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				kv[strings.TrimSpace(parts[0])] = val
			}
		}
		tel.UptimeSec = kv["Uptime"]
		tel.ConnectionsActive = kv["Threads_connected"]
		tel.ThreadsRunning = kv["Threads_running"]
		tel.QueriesTotal = kv["Questions"]
		tel.SlowQueries = kv["Slow_queries"]
		tel.MemoryUsedBytes = kv["Innodb_buffer_pool_bytes_data"]
		if tel.UptimeSec > 0 {
			tel.QPS = float64(tel.QueriesTotal) / float64(tel.UptimeSec)
		}
		if reqs := kv["Innodb_buffer_pool_read_requests"]; reqs > 0 {
			hit := (1.0 - float64(kv["Innodb_buffer_pool_reads"])/float64(reqs)) * 100.0
			if hit < 0 {
				hit = 0
			}
			tel.CacheHitPct = hit
		} else {
			tel.CacheHitPct = 100.0
		}

		// Ambil variabel max_connections dan innodb_buffer_pool_size
		vOut, vErr := database.RunMysqlQueryOutput(tCtx, "", "SHOW GLOBAL VARIABLES WHERE Variable_name IN ('max_connections','innodb_buffer_pool_size');")
		if vErr == nil {
			sc := bufio.NewScanner(strings.NewReader(vOut))
			for sc.Scan() {
				parts := strings.Split(sc.Text(), "\t")
				if len(parts) >= 2 {
					val, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
					switch strings.TrimSpace(parts[0]) {
					case "max_connections":
						tel.ConnectionsMax = val
					case "innodb_buffer_pool_size":
						tel.MemoryLimitBytes = val
					}
				}
			}
		}

	case "postgresql", "postgres":
		q := `SELECT
			EXTRACT(EPOCH FROM (now() - pg_postmaster_start_time()))::bigint,
			(SELECT count(*) FROM pg_stat_activity),
			current_setting('max_connections')::bigint,
			COALESCE((SELECT sum(xact_commit + xact_rollback) FROM pg_stat_database), 0),
			COALESCE((SELECT CASE WHEN sum(blks_hit + blks_read) > 0 THEN round(sum(blks_hit)::numeric * 100.0 / sum(blks_hit + blks_read), 2) ELSE 100 END FROM pg_stat_database), 100),
			(SELECT count(*) FROM pg_stat_activity WHERE state = 'active');`
		out, err := database.RunPostgresQueryOutput(tCtx, "postgres", q, false)
		if err != nil {
			tel.Status = "offline"
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "telemetry": tel})
			return
		}
		parts := strings.Split(strings.TrimSpace(out), "\t")
		if len(parts) >= 6 {
			tel.UptimeSec, _ = strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
			tel.ConnectionsActive, _ = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			tel.ConnectionsMax, _ = strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
			tel.QueriesTotal, _ = strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
			tel.CacheHitPct, _ = strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
			tel.ThreadsRunning, _ = strconv.ParseInt(strings.TrimSpace(parts[5]), 10, 64)
			if tel.UptimeSec > 0 {
				tel.QPS = float64(tel.QueriesTotal) / float64(tel.UptimeSec)
			}
		}

	case "redis":
		out, err := exec.CommandContext(tCtx, "redis-cli", "INFO").CombinedOutput()
		if err != nil {
			tel.Status = "offline"
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "telemetry": tel})
			return
		}
		tel = ParseRedisInfo(string(out))

	default:
		tel.Status = "running"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"telemetry": tel,
	})
}

// handleDatabaseProcesslist mengambil daftar koneksi & kueri aktif pada engine database
func (s *Server) handleDatabaseProcesslist(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	if runtime.GOOS != "linux" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok": true,
			"processes": []DatabaseProcessItem{
				{PID: 42, User: "kostash", Host: "127.0.0.1:49210", DB: "kostash", Command: "Sleep", TimeSec: 12, State: "idle", Query: ""},
				{PID: 58, User: "wa_bot", Host: "127.0.0.1:51042", DB: "wa_bot", Command: "Query", TimeSec: 1, State: "Sending data", Query: "SELECT id, phone, status FROM messages ORDER BY id DESC LIMIT 50"},
			},
		})
		return
	}

	pCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var procs []DatabaseProcessItem
	switch engine {
	case "mysql":
		out, err := database.RunMysqlQueryOutput(pCtx, "", "SHOW FULL PROCESSLIST;")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			cols := strings.Split(scanner.Text(), "\t")
			if len(cols) < 8 {
				continue
			}
			pid, _ := strconv.ParseInt(strings.TrimSpace(cols[0]), 10, 64)
			timeSec, _ := strconv.Atoi(strings.TrimSpace(cols[5]))
			qStr := strings.TrimSpace(cols[7])
			if qStr == "NULL" {
				qStr = ""
			}
			dbVal := strings.TrimSpace(cols[3])
			if dbVal == "NULL" {
				dbVal = "-"
			}
			procs = append(procs, DatabaseProcessItem{
				PID:     pid,
				User:    strings.TrimSpace(cols[1]),
				Host:    strings.TrimSpace(cols[2]),
				DB:      dbVal,
				Command: strings.TrimSpace(cols[4]),
				TimeSec: timeSec,
				State:   strings.TrimSpace(cols[6]),
				Query:   qStr,
			})
		}

	case "postgresql", "postgres":
		q := `SELECT pid, COALESCE(usename,''), COALESCE(client_addr::text,'local'), COALESCE(datname,'-'),
			COALESCE(state,'idle'), COALESCE(EXTRACT(EPOCH FROM (now() - query_start))::int, 0),
			COALESCE(wait_event_type,'-'), LEFT(COALESCE(query,''), 400)
			FROM pg_stat_activity WHERE pid <> pg_backend_pid() ORDER BY query_start DESC NULLS LAST LIMIT 100;`
		out, err := database.RunPostgresQueryOutput(pCtx, "postgres", q, false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			cols := strings.Split(scanner.Text(), "\t")
			if len(cols) < 8 {
				continue
			}
			pid, _ := strconv.ParseInt(strings.TrimSpace(cols[0]), 10, 64)
			timeSec, _ := strconv.Atoi(strings.TrimSpace(cols[5]))
			procs = append(procs, DatabaseProcessItem{
				PID:     pid,
				User:    strings.TrimSpace(cols[1]),
				Host:    strings.TrimSpace(cols[2]),
				DB:      strings.TrimSpace(cols[3]),
				Command: strings.TrimSpace(cols[4]),
				TimeSec: timeSec,
				State:   strings.TrimSpace(cols[6]),
				Query:   strings.TrimSpace(cols[7]),
			})
		}
	}

	if procs == nil {
		procs = []DatabaseProcessItem{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"processes": procs,
	})
}

// handleKillDatabaseProcess menghentikan kueri/koneksi database tertentu berdasarkan PID
func (s *Server) handleKillDatabaseProcess(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	pidStr := strings.TrimSpace(r.PathValue("pid"))
	pid, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil || pid <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "PID proses database tidak valid"})
		return
	}

	if runtime.GOOS == "linux" {
		kCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		switch engine {
		case "mysql":
			if _, err := database.RunMysqlQueryOutput(kCtx, "", fmt.Sprintf("KILL %d;", pid)); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		case "postgresql", "postgres":
			if _, err := database.RunPostgresQueryOutput(kCtx, "postgres", fmt.Sprintf("SELECT pg_terminate_backend(%d);", pid), false); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kill process tidak didukung untuk engine " + engine})
			return
		}
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.kill_process", fmt.Sprintf("%s:%d", engine, pid), true,
		fmt.Sprintf("Menghentikan sesi/kueri database PID %d pada %s", pid, engine), nil)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Proses PID %d pada %s berhasil dihentikan", pid, engine),
	})
}

// TuningPreset merepresentasikan hasil generator konfigurasi tuning RAM
type TuningPreset struct {
	Profile     string            `json:"profile"`
	FilePath    string            `json:"file_path"`
	Content     string            `json:"content"`
	Summary     map[string]string `json:"summary"`
}

// BuildTuningConfig menghasilkan konfigurasi performa optimal berdasarkan kapasitas RAM server (1gb, 2gb, 4gb, 8gb)
func BuildTuningConfig(engine, profile string) (TuningPreset, error) {
	p := strings.ToLower(strings.TrimSpace(profile))
	switch p {
	case "1gb", "2gb", "4gb", "8gb":
	default:
		return TuningPreset{}, fmt.Errorf("profil RAM tidak valid (gunakan: 1gb, 2gb, 4gb, 8gb)")
	}

	switch strings.ToLower(engine) {
	case "mysql":
		cfgMap := map[string]struct {
			bufPool, logSize, tmpSize, maxConn, tableCache string
		}{
			"1gb": {"256M", "64M", "32M", "80", "400"},
			"2gb": {"512M", "128M", "64M", "150", "800"},
			"4gb": {"1536M", "256M", "128M", "250", "1600"},
			"8gb": {"4096M", "512M", "256M", "400", "3200"},
		}[p]

		targetPath := "/etc/mysql/mariadb.conf.d/99-fikpanel-tuning.cnf"
		if _, err := os.Stat("/etc/mysql/mysql.conf.d"); err == nil {
			if _, errMaria := os.Stat("/etc/mysql/mariadb.conf.d"); errMaria != nil {
				targetPath = "/etc/mysql/mysql.conf.d/99-fikpanel-tuning.cnf"
			}
		}

		content := fmt.Sprintf(`# Dihasilkan otomatis oleh FikPanel RAM Tuner (Profil: %s)
[mysqld]
innodb_buffer_pool_size = %s
innodb_log_file_size = %s
innodb_flush_log_at_trx_commit = 2
innodb_file_per_table = 1
max_connections = %s
table_open_cache = %s
tmp_table_size = %s
max_heap_table_size = %s
slow_query_log = 1
long_query_time = 2
`, strings.ToUpper(p), cfgMap.bufPool, cfgMap.logSize, cfgMap.maxConn, cfgMap.tableCache, cfgMap.tmpSize, cfgMap.tmpSize)

		return TuningPreset{
			Profile:  p,
			FilePath: targetPath,
			Content:  content,
			Summary: map[string]string{
				"innodb_buffer_pool_size": cfgMap.bufPool,
				"max_connections":         cfgMap.maxConn,
				"innodb_log_file_size":    cfgMap.logSize,
				"table_open_cache":        cfgMap.tableCache,
			},
		}, nil

	case "postgresql", "postgres":
		cfgMap := map[string]struct {
			sharedBuf, effCache, workMem, maintMem, maxConn string
		}{
			"1gb": {"256MB", "768MB", "4MB", "64MB", "80"},
			"2gb": {"512MB", "1536MB", "8MB", "128MB", "120"},
			"4gb": {"1GB", "3GB", "16MB", "256MB", "200"},
			"8gb": {"2GB", "6GB", "32MB", "512MB", "300"},
		}[p]

		baseCfg := database.ResolveConfigPath("postgresql")
		targetPath := filepath.Join(filepath.Dir(baseCfg), "conf.d", "99-fikpanel-tuning.conf")
		content := fmt.Sprintf(`# Dihasilkan otomatis oleh FikPanel RAM Tuner (Profil: %s)
shared_buffers = %s
effective_cache_size = %s
work_mem = %s
maintenance_work_mem = %s
max_connections = %s
`, strings.ToUpper(p), cfgMap.sharedBuf, cfgMap.effCache, cfgMap.workMem, cfgMap.maintMem, cfgMap.maxConn)

		return TuningPreset{
			Profile:  p,
			FilePath: targetPath,
			Content:  content,
			Summary: map[string]string{
				"shared_buffers":       cfgMap.sharedBuf,
				"effective_cache_size": cfgMap.effCache,
				"work_mem":             cfgMap.workMem,
				"max_connections":      cfgMap.maxConn,
			},
		}, nil

	case "redis":
		maxMem := map[string]string{
			"1gb": "256mb",
			"2gb": "512mb",
			"4gb": "1gb",
			"8gb": "2gb",
		}[p]
		return TuningPreset{
			Profile:  p,
			FilePath: database.ResolveConfigPath("redis"),
			Content:  fmt.Sprintf("maxmemory %s\nmaxmemory-policy allkeys-lru\n", maxMem),
			Summary: map[string]string{
				"maxmemory":        maxMem,
				"maxmemory-policy": "allkeys-lru",
			},
		}, nil

	default:
		return TuningPreset{}, fmt.Errorf("RAM Tuner belum didukung untuk engine %s", engine)
	}
}

// handleTuneDatabaseEngine menerapkan profil tuning RAM 1-klik lalu me-restart layanan secara aman
func (s *Server) handleTuneDatabaseEngine(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	var req struct {
		Profile string `json:"profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}

	preset, err := BuildTuningConfig(engine, req.Profile)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if runtime.GOOS == "linux" {
		if engine == "redis" {
			// Terapkan live via CONFIG SET dan persist di redis.conf
			maxMem := preset.Summary["maxmemory"]
			_ = exec.CommandContext(r.Context(), "redis-cli", "CONFIG", "SET", "maxmemory", maxMem).Run()
			_ = exec.CommandContext(r.Context(), "redis-cli", "CONFIG", "SET", "maxmemory-policy", "allkeys-lru").Run()
			if raw, err := os.ReadFile(preset.FilePath); err == nil {
				lines := strings.Split(string(raw), "\n")
				setMem, setPol := false, false
				for i, l := range lines {
					tr := strings.TrimSpace(l)
					if strings.HasPrefix(tr, "maxmemory ") || strings.HasPrefix(tr, "# maxmemory ") {
						lines[i] = "maxmemory " + maxMem
						setMem = true
					} else if strings.HasPrefix(tr, "maxmemory-policy ") || strings.HasPrefix(tr, "# maxmemory-policy ") {
						lines[i] = "maxmemory-policy allkeys-lru"
						setPol = true
					}
				}
				if !setMem {
					lines = append(lines, "maxmemory "+maxMem)
				}
				if !setPol {
					lines = append(lines, "maxmemory-policy allkeys-lru")
				}
				_ = os.WriteFile(preset.FilePath, []byte(strings.Join(lines, "\n")), 0640)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(preset.FilePath), 0755); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if err := os.WriteFile(preset.FilePath, []byte(preset.Content), 0644); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("gagal menulis berkas tuning: %v", err)})
				return
			}
			if _, err := database.ControlEngineService(r.Context(), engine, "restart"); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("konfigurasi tersimpan di %s, namun restart layanan gagal: %v", preset.FilePath, err)})
				return
			}
		}
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.tune", fmt.Sprintf("%s:%s", engine, preset.Profile), true,
		fmt.Sprintf("Menerapkan profil RAM Tuner %s pada %s", strings.ToUpper(preset.Profile), engine), nil)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"preset":  preset,
		"message": fmt.Sprintf("Profil RAM %s berhasil diterapkan pada %s (%s)", strings.ToUpper(preset.Profile), engine, preset.FilePath),
	})
}

// handleRedisKeys memindai kunci Redis secara aman menggunakan SCAN tanpa memblokir event loop Redis
func (s *Server) handleRedisKeys(w http.ResponseWriter, r *http.Request) {
	dbIdx, _ := strconv.Atoi(r.URL.Query().Get("db"))
	if dbIdx < 0 || dbIdx > 15 {
		dbIdx = 0
	}
	pattern := strings.TrimSpace(r.URL.Query().Get("pattern"))
	if pattern == "" {
		pattern = "*"
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("count"))
	if limit <= 0 || limit > 150 {
		limit = 100
	}

	if runtime.GOOS != "linux" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok": true,
			"db": dbIdx,
			"keys": []RedisKeySummary{
				{Key: "laravel_cache:settings_global", Type: "string", TTL: 3540},
				{Key: "laravel_session:a8f2c91e04", Type: "string", TTL: 7190},
				{Key: "queues:default", Type: "list", TTL: -1},
				{Key: "rate_limit:api:103.120.1.5", Type: "string", TTL: 48},
			},
		})
		return
	}

	rCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Gunakan redis-cli --scan dengan penghentian dini saat mencapai batas aman
	cmd := exec.CommandContext(rCtx, "redis-cli", "-n", strconv.Itoa(dbIdx), "--scan", "--pattern", pattern)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := cmd.Start(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("gagal menghubungi redis-cli: %v", err)})
		return
	}

	var foundKeys []string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		k := strings.TrimSpace(scanner.Text())
		if k != "" {
			foundKeys = append(foundKeys, k)
			if len(foundKeys) >= limit {
				break
			}
		}
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	items := make([]RedisKeySummary, 0, len(foundKeys))
	if len(foundKeys) > 0 {
		// Ambil TYPE dan TTL secara batch dalam 1 panggilan stdin agar sangat cepat
		var batchBuf strings.Builder
		for _, k := range foundKeys {
			quoted := strconv.Quote(k)
			batchBuf.WriteString("TYPE " + quoted + "\n")
			batchBuf.WriteString("TTL " + quoted + "\n")
		}
		metaCmd := exec.CommandContext(rCtx, "redis-cli", "-n", strconv.Itoa(dbIdx))
		metaCmd.Stdin = strings.NewReader(batchBuf.String())
		metaOut, _ := metaCmd.Output()
		metaLines := strings.Split(strings.TrimSpace(string(metaOut)), "\n")

		for i, k := range foundKeys {
			keyType := "string"
			var ttl int64 = -1
			if 2*i < len(metaLines) {
				keyType = strings.TrimSpace(metaLines[2*i])
			}
			if 2*i+1 < len(metaLines) {
				ttl, _ = strconv.ParseInt(strings.TrimSpace(metaLines[2*i+1]), 10, 64)
			}
			items = append(items, RedisKeySummary{
				Key:  k,
				Type: keyType,
				TTL:  ttl,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"db":   dbIdx,
		"keys": items,
	})
}

// handleRedisInspectKey mengambil tipe, TTL, dan isi nilai lengkap dari satu kunci Redis
func (s *Server) handleRedisInspectKey(w http.ResponseWriter, r *http.Request) {
	dbIdx, _ := strconv.Atoi(r.URL.Query().Get("db"))
	if dbIdx < 0 || dbIdx > 15 {
		dbIdx = 0
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parameter key wajib diisi"})
		return
	}

	if runtime.GOOS != "linux" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":    true,
			"key":   key,
			"type":  "string",
			"ttl":   3500,
			"value": `{"panel":"FikPanel","cached":true,"version":"0.7.9"}`,
		})
		return
	}

	rCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dbStr := strconv.Itoa(dbIdx)
	typeOut, err := exec.CommandContext(rCtx, "redis-cli", "-n", dbStr, "TYPE", key).Output()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memeriksa kunci Redis"})
		return
	}
	keyType := strings.TrimSpace(string(typeOut))
	if keyType == "none" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "kunci sudah kedaluwarsa atau tidak ditemukan"})
		return
	}

	ttlOut, _ := exec.CommandContext(rCtx, "redis-cli", "-n", dbStr, "TTL", key).Output()
	ttl, _ := strconv.ParseInt(strings.TrimSpace(string(ttlOut)), 10, 64)

	var valOut []byte
	switch keyType {
	case "string":
		valOut, _ = exec.CommandContext(rCtx, "redis-cli", "-n", dbStr, "--raw", "GET", key).Output()
	case "hash":
		valOut, _ = exec.CommandContext(rCtx, "redis-cli", "-n", dbStr, "HGETALL", key).Output()
	case "list":
		valOut, _ = exec.CommandContext(rCtx, "redis-cli", "-n", dbStr, "LRANGE", key, "0", "200").Output()
	case "set":
		valOut, _ = exec.CommandContext(rCtx, "redis-cli", "-n", dbStr, "SMEMBERS", key).Output()
	case "zset":
		valOut, _ = exec.CommandContext(rCtx, "redis-cli", "-n", dbStr, "ZRANGE", key, "0", "200", "WITHSCORES").Output()
	default:
		valOut = []byte(fmt.Sprintf("(tipe data '%s' tidak dapat ditampilkan sebagai teks)", keyType))
	}

	valStr := string(valOut)
	if len(valStr) > 64*1024 {
		valStr = valStr[:64*1024] + "\n... [terpotong di 64 KB]"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"key":   key,
		"type":  keyType,
		"ttl":   ttl,
		"value": valStr,
	})
}

// handleRedisDeleteKey menghapus satu kunci Redis tertentu
func (s *Server) handleRedisDeleteKey(w http.ResponseWriter, r *http.Request) {
	dbIdx, _ := strconv.Atoi(r.URL.Query().Get("db"))
	if dbIdx < 0 || dbIdx > 15 {
		dbIdx = 0
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parameter key wajib diisi"})
		return
	}

	if runtime.GOOS == "linux" {
		rCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(rCtx, "redis-cli", "-n", strconv.Itoa(dbIdx), "DEL", key).CombinedOutput(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": strings.TrimSpace(string(out))})
			return
		}
	}

	_ = s.store.RecordAudit(r.Context(), "user", "redis.delete_key", fmt.Sprintf("db%d:%s", dbIdx, key), true, "Menghapus kunci Redis", nil)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Kunci '%s' pada db%d berhasil dihapus", key, dbIdx),
	})
}

// handleRedisFlushDB mengosongkan seluruh kunci pada satu index database Redis (FLUSHDB)
func (s *Server) handleRedisFlushDB(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DB int `json:"db"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if req.DB < 0 || req.DB > 15 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "indeks DB Redis harus antara 0 hingga 15"})
		return
	}

	if runtime.GOOS == "linux" {
		rCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(rCtx, "redis-cli", "-n", strconv.Itoa(req.DB), "FLUSHDB").CombinedOutput(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": strings.TrimSpace(string(out))})
			return
		}
	}

	_ = s.store.RecordAudit(r.Context(), "user", "redis.flushdb", fmt.Sprintf("db%d", req.DB), true,
		fmt.Sprintf("Mengosongkan seluruh kunci pada Redis db%d (FLUSHDB)", req.DB), nil)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Seluruh cache pada Redis db%d berhasil dikosongkan", req.DB),
	})
}

// autoBackupCronName menghasilkan nama tugas cron baku untuk auto-backup engine tertentu
func autoBackupCronName(engine string) string {
	return fmt.Sprintf("Auto Backup Database (%s)", strings.ToLower(engine))
}

// findAutoBackupJob mencari tugas cron auto-backup yang terhubung dengan engine
func (s *Server) findAutoBackupJob(ctx context.Context, engine string) (*store.CronJob, error) {
	jobs, err := s.store.ListCronJobs(ctx)
	if err != nil {
		return nil, err
	}
	targetName := autoBackupCronName(engine)
	for i := range jobs {
		if jobs[i].Name == targetName {
			return &jobs[i], nil
		}
	}
	return nil, nil
}

// handleGetAutoBackup mengambil status cron auto-backup database untuk engine terkait
func (s *Server) handleGetAutoBackup(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	job, err := s.findAutoBackupJob(r.Context(), engine)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memeriksa jadwal auto-backup"})
		return
	}

	enabled := false
	schedule := "0 2 * * *"
	var jobID int64
	lastStatus := ""
	var lastRunAt int64
	if job != nil {
		enabled = job.Enabled
		schedule = job.Spec
		jobID = job.ID
		lastStatus = job.LastStatus
		lastRunAt = job.LastRunAt
	}

	keepStr, _ := s.store.GetSetting(r.Context(), "db_autobackup_keep:"+engine)
	keepDays, _ := strconv.Atoi(keepStr)
	if keepDays <= 0 {
		keepDays = 7
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"enabled":     enabled,
		"schedule":    schedule,
		"keep_days":   keepDays,
		"cron_job_id": jobID,
		"last_status": lastStatus,
		"last_run_at": lastRunAt,
	})
}

// handleSetAutoBackup mengaktifkan atau menonaktifkan tugas cron otomatis untuk pencadangan harian database
func (s *Server) handleSetAutoBackup(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	var req struct {
		Enabled  bool   `json:"enabled"`
		Schedule string `json:"schedule"`
		KeepDays int    `json:"keep_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	schedule := strings.TrimSpace(req.Schedule)
	if schedule == "" {
		schedule = "0 2 * * *"
	}
	if req.KeepDays <= 0 {
		req.KeepDays = 7
	}

	_ = s.store.SetSetting(r.Context(), "db_autobackup_keep:"+engine, strconv.Itoa(req.KeepDays))

	// Susun perintah backup otomatis yang aman beserta pembersihan retensi berkas lama
	var cmdStr string
	switch engine {
	case "postgresql", "postgres":
		cmdStr = fmt.Sprintf(
			`mkdir -p /var/backups/fikpanel && su - postgres -c "pg_dumpall" | gzip > /var/backups/fikpanel/autobackup_postgresql_$(date +%%Y%%m%%d-%%H%%M%%S).sql.gz && find /var/backups/fikpanel -name "autobackup_postgresql_*.sql.gz" -mtime +%d -delete`,
			req.KeepDays,
		)
	default:
		cmdStr = fmt.Sprintf(
			`mkdir -p /var/backups/fikpanel && ( [ -f /etc/mysql/debian.cnf ] && mysqldump --defaults-file=/etc/mysql/debian.cnf --all-databases --single-transaction --routines --triggers || mysqldump -u root --all-databases --single-transaction --routines --triggers ) | gzip > /var/backups/fikpanel/autobackup_mysql_$(date +%%Y%%m%%d-%%H%%M%%S).sql.gz && find /var/backups/fikpanel -name "autobackup_mysql_*.sql.gz" -mtime +%d -delete`,
			req.KeepDays,
		)
	}

	existing, err := s.findAutoBackupJob(r.Context(), engine)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memeriksa tugas cron"})
		return
	}

	var jobID int64
	if existing == nil {
		job := &store.CronJob{
			Name:         autoBackupCronName(engine),
			ScheduleType: "custom",
			Spec:         schedule,
			Command:      cmdStr,
			WorkingDir:   "/var/backups/fikpanel",
			Enabled:      req.Enabled,
			SaveLog:      true,
			NotifyOnFail: true,
			TimeoutSec:   1800,
		}
		id, err := s.store.CreateCronJob(r.Context(), job)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("gagal membuat cron auto-backup: %v", err)})
			return
		}
		jobID = id
	} else {
		existing.Spec = schedule
		existing.Command = cmdStr
		existing.Enabled = req.Enabled
		if err := s.store.UpdateCronJob(r.Context(), existing); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("gagal memperbarui cron auto-backup: %v", err)})
			return
		}
		_ = s.store.ToggleCronJob(r.Context(), existing.ID, req.Enabled)
		jobID = existing.ID
	}

	stateLabel := "dinonaktifkan"
	if req.Enabled {
		stateLabel = "diaktifkan (" + schedule + ")"
	}
	_ = s.store.RecordAudit(r.Context(), "user", "database.auto_backup", engine, true,
		fmt.Sprintf("Auto-backup database %s %s", engine, stateLabel), nil)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"enabled":     req.Enabled,
		"schedule":    schedule,
		"keep_days":   req.KeepDays,
		"cron_job_id": jobID,
		"message":     fmt.Sprintf("Auto-backup terjadwal untuk %s berhasil %s", engine, stateLabel),
	})
}
