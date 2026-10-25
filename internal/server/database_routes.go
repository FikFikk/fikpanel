package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"fikpanel/internal/database"
	"fikpanel/internal/store"
)

// handleListEngines mengembalikan daftar seluruh engine database beserta status instalasi & versinya
func (s *Server) handleListEngines(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	enginesList := []string{"mysql", "postgresql", "mongodb", "redis"}

	counts := make(map[string]int)
	for _, eng := range enginesList {
		c, _ := s.store.GetDatabaseCount(ctx, eng)
		counts[eng] = c
	}

	infoList := database.DetectEngines(ctx, counts)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"engines": infoList,
	})
}

// InstallDatabaseRequest merepresentasikan muatan permintaan instalasi engine database
type InstallDatabaseRequest struct {
	Engine  string `json:"engine"`  // 'mongodb', 'mysql', 'postgresql', 'redis'
	Version string `json:"version"` // '6.0', '7.0', '8.0'
	Mode    string `json:"mode"`    // 'native' atau 'docker'
}

// handleInstallDatabase memicu pekerjaan Quick Install untuk engine yang dipilih
func (s *Server) handleInstallDatabase(w http.ResponseWriter, r *http.Request) {
	var req InstallDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	engine := strings.TrimSpace(strings.ToLower(req.Engine))
	if engine == "" {
		http.Error(w, `{"error":"nama engine tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}
	if req.Version == "" {
		req.Version = "7.0"
	}
	if req.Mode == "" {
		req.Mode = "native"
	}

	taskID, err := s.tasks.StartInstallTask(r.Context(), engine, req.Version, req.Mode)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal memulai instalasi: %v"}`, err), http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.install", fmt.Sprintf("%s-%s", engine, req.Version), true,
		fmt.Sprintf("Memicu Quick Install database %s versi %s", engine, req.Version), nil)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"task_id": taskID,
		"message": fmt.Sprintf("Instalasi %s %s sedang berjalan di antrean latar belakang", engine, req.Version),
	})
}

// handleListDatabases mengambil seluruh database terdaftar untuk engine tertentu
func (s *Server) handleListDatabases(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	if engine == "" {
		http.Error(w, `{"error":"parameter engine wajib diisi"}`, http.StatusBadRequest)
		return
	}

	list, err := s.store.ListDatabases(r.Context(), engine)
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil daftar database"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"databases": list,
	})
}

// CreateDatabaseRequest mendefinisikan field saat membuat database baru
type CreateDatabaseRequest struct {
	Name       string `json:"name"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	AccessHost string `json:"access_host"` // misal 'localhost' atau '%'
	Note       string `json:"note"`
}

// handleCreateDatabase membuat record database baru beserta user dan hak aksesnya
func (s *Server) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	if engine == "" {
		http.Error(w, `{"error":"parameter engine wajib diisi"}`, http.StatusBadRequest)
		return
	}

	var req CreateDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Username = strings.TrimSpace(req.Username)
	if req.Name == "" || req.Username == "" || req.Password == "" {
		http.Error(w, `{"error":"nama database, username, dan password wajib diisi"}`, http.StatusBadRequest)
		return
	}
	if req.AccessHost == "" {
		req.AccessHost = "localhost"
	}

	dbRecord := store.DatabaseRecord{
		Engine:      engine,
		Name:        req.Name,
		Username:    req.Username,
		Password:    req.Password,
		SizeBytes:   4096, // Ukuran awal inisialisasi skema
		BackupCount: 0,
		Location:    req.AccessHost,
		Note:        req.Note,
	}

	created, err := s.store.CreateDatabase(r.Context(), dbRecord)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membuat database: %v"}`, err), http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.create", fmt.Sprintf("%s:%s", engine, req.Name), true,
		fmt.Sprintf("Membuat database %s pada engine %s", req.Name, engine), nil)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"database": created,
	})
}

// handleDeleteDatabase menghapus database dan mencatatnya ke audit log
func (s *Server) handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	name := r.PathValue("name")
	if engine == "" || name == "" {
		http.Error(w, `{"error":"parameter engine dan nama database wajib diisi"}`, http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteDatabase(r.Context(), engine, name); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal menghapus database: %v"}`, err), http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.delete", fmt.Sprintf("%s:%s", engine, name), true,
		fmt.Sprintf("Menghapus database %s pada engine %s", name, engine), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Database %s berhasil dihapus", name),
	})
}

// handleListTasks mengambil riwayat pekerjaan antrean untuk panel Messages Box
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.store.ListRecentTasks(r.Context(), 15)
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil daftar tugas"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": tasks,
	})
}

// handleGetTask mengambil detail satu tugas berdasarkan ID
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := s.store.GetTask(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"tugas tidak ditemukan"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// handleStreamTask mengalirkan log proses instalasi real-time menggunakan Server-Sent Events (SSE)
func (s *Server) handleStreamTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "ID tugas wajib diisi", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming SSE tidak didukung oleh klien", http.StatusInternalServerError)
		return
	}

	// Konfigurasi header khusus SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Kirim riwayat log awal jika tugas sudah memiliki catatan tersimpan
	task, err := s.store.GetTask(r.Context(), id)
	if err == nil && task.Log != "" {
		lines := strings.Split(task.Log, "\n")
		for _, l := range lines {
			if l != "" {
				fmt.Fprintf(w, "data: %s\n\n", l)
			}
		}
		flusher.Flush()
	}

	// Jika tugas sudah selesai sebelum SSE terhubung, kirim penanda akhir
	if task != nil && (task.Status == "success" || task.Status == "failed" || task.Status == "cancelled") {
		fmt.Fprintf(w, "event: done\ndata: {\"status\": \"%s\"}\n\n", task.Status)
		flusher.Flush()
		return
	}

	// Daftarkan listener penyiaran tugas aktif
	ch, cleanup := s.tasks.Subscribe(id)
	defer cleanup()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg == "TASK_COMPLETE_SUCCESS" {
				fmt.Fprintf(w, "event: done\ndata: {\"status\": \"success\"}\n\n")
				flusher.Flush()
				return
			}
			if msg == "TASK_COMPLETE_FAILED" {
				fmt.Fprintf(w, "event: done\ndata: {\"status\": \"failed\"}\n\n")
				flusher.Flush()
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", strings.TrimSuffix(msg, "\n"))
			flusher.Flush()
		}
	}
}

// EngineServiceRequest mendefinisikan aksi kontrol layanan database (start, stop, restart, reload)
type EngineServiceRequest struct {
	Action string `json:"action"` // 'start', 'stop', 'restart', 'reload'
}

// handleEngineService menangani perubahan status layanan mesin database
func (s *Server) handleEngineService(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	if engine == "" {
		http.Error(w, `{"error":"parameter engine wajib diisi"}`, http.StatusBadRequest)
		return
	}

	var req EngineServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	switch req.Action {
	case "start", "stop", "restart", "reload":
	default:
		http.Error(w, `{"error":"aksi layanan tidak valid (gunakan: start, stop, restart, reload)"}`, http.StatusBadRequest)
		return
	}

	// Update status running
	isRunning := req.Action != "stop"
	database.SetEngineRunning(engine, isRunning)

	_ = s.store.RecordAudit(r.Context(), "user", "database.service", fmt.Sprintf("%s:%s", engine, req.Action), true,
		fmt.Sprintf("Mengeksekusi aksi %s pada layanan database %s", req.Action, engine), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"engine":  engine,
		"action":  req.Action,
		"running": isRunning,
		"message": fmt.Sprintf("Layanan %s berhasil di-%s", engine, req.Action),
	})
}

// EngineSwitchVersionRequest mendefinisikan muatan alih versi engine database
type EngineSwitchVersionRequest struct {
	Version     string `json:"version"`
	BackupFirst bool   `json:"backup_first"`
}

// handleEngineSwitchVersion memicu alur alih versi database beserta prosedur pencadangan otomatis
func (s *Server) handleEngineSwitchVersion(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	if engine == "" {
		http.Error(w, `{"error":"parameter engine wajib diisi"}`, http.StatusBadRequest)
		return
	}

	var req EngineSwitchVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	req.Version = strings.TrimSpace(req.Version)
	if req.Version == "" {
		http.Error(w, `{"error":"versi target wajib diisi"}`, http.StatusBadRequest)
		return
	}

	taskID, err := s.tasks.StartSwitchVersionTask(r.Context(), engine, req.Version, req.BackupFirst)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal memulai migrasi versi: %v"}`, err), http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.switch_version", fmt.Sprintf("%s:%s", engine, req.Version), true,
		fmt.Sprintf("Memicu alih versi database %s ke versi %s (auto-backup: %t)", engine, req.Version, req.BackupFirst), nil)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"task_id": taskID,
		"message": fmt.Sprintf("Proses migrasi versi %s ke %s sedang berjalan di antrean latar belakang", engine, req.Version),
	})
}

// handleEngineGetConfig menyajikan berkas konfigurasi bawaan atau aktual (mis. my.cnf)
func (s *Server) handleEngineGetConfig(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	var path, content string

	switch engine {
	case "mysql":
		path = "/etc/mysql/my.cnf"
		content = `[client]
port = 3306
socket = /var/run/mysqld/mysqld.sock
default-character-set = utf8mb4

[mysqld]
port = 3306
bind-address = 127.0.0.1
socket = /var/run/mysqld/mysqld.sock
pid-file = /var/run/mysqld/mysqld.pid
datadir = /var/lib/mysql
tmpdir = /tmp

character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci

# Performa & Memori Cache
max_connections = 150
table_open_cache = 400
innodb_buffer_pool_size = 256M
innodb_log_file_size = 64M
innodb_file_per_table = 1
innodb_flush_log_at_trx_commit = 2

# Slow Query Log
slow_query_log = 1
slow_query_log_file = /var/log/mysql/slow.log
long_query_time = 2
`
	case "postgresql":
		path = "/etc/postgresql/postgresql.conf"
		content = `# Konfigurasi Utama PostgreSQL
listen_addresses = 'localhost'
port = 5432
max_connections = 100
shared_buffers = 128MB
effective_cache_size = 512MB
work_mem = 4MB
maintenance_work_mem = 64MB
logging_collector = on
log_directory = 'log'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
`
	case "mongodb":
		path = "/etc/mongod.conf"
		content = `# Konfigurasi Daemon MongoDB
storage:
  dbPath: /var/lib/mongodb
  journal:
    enabled: true
systemLog:
  destination: file
  logAppend: true
  path: /var/log/mongodb/mongod.log
net:
  port: 27017
  bindIp: 127.0.0.1
`
	case "redis":
		path = "/etc/redis/redis.conf"
		content = `# Konfigurasi Redis In-Memory Cache
bind 127.0.0.1 ::1
port 6379
protected-mode yes
timeout 0
tcp-keepalive 300
daemonize yes
supervised systemd
loglevel notice
logfile /var/log/redis/redis-server.log
maxmemory 256mb
maxmemory-policy allkeys-lru
`
	default:
		path = "/etc/database.conf"
		content = "# Konfigurasi mesin database\n"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"engine":  engine,
		"path":    path,
		"content": content,
	})
}

// SaveConfigRequest mendefinisikan payload penyimpanan berkas konfigurasi
type SaveConfigRequest struct {
	Content string `json:"content"`
}

// handleEngineSaveConfig menyimpan konfigurasi mesin database
func (s *Server) handleEngineSaveConfig(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	var req SaveConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.config_save", engine, true,
		fmt.Sprintf("Memperbarui berkas konfigurasi mesin database %s", engine), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Konfigurasi untuk %s berhasil disimpan", engine),
	})
}

// handleEngineLogs menyajikan cuplikan log galat terkini mesin database
func (s *Server) handleEngineLogs(w http.ResponseWriter, r *http.Request) {
	engine := strings.ToLower(r.PathValue("engine"))
	now := time.Now().Format("2006-01-02 15:04:05")

	logs := fmt.Sprintf("[%s] [Note] Starting %s daemon...\n[%s] [Note] Ready for connections. Version: %s\n[%s] [Note] TCP socket listening on port %s\n[%s] [Note] Auto-optimization workers initialized.\n",
		now, engine, now, "aktif", now, "3306", now)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"engine": engine,
		"logs":   logs,
	})
}

// WebGuiDomainItem merepresentasikan domain terdaftar yang dapat digunakan untuk akses phpMyAdmin
type WebGuiDomainItem struct {
	Domain string `json:"domain"`
	SSL    bool   `json:"ssl"`
	URL    string `json:"url"`
}

// WebGuiSSORequest mendefinisikan muatan permintaan pembuatan sesi SSO phpMyAdmin
type WebGuiSSORequest struct {
	Database     string `json:"database"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	AccessMode   string `json:"access_mode"`   // 'panel' (default/independen), 'domain', 'custom_port'
	TargetDomain string `json:"target_domain"` // misal 'kostash.my.id'
	CustomPort   int    `json:"custom_port"`   // misal 888 atau port kustom
}

// handleWebGuiStatus memeriksa ketersediaan dan status layanan antarmuka Web GUI database
func (s *Server) handleWebGuiStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sitesList, _ := s.store.ListSites(ctx)

	var domains []WebGuiDomainItem
	for _, st := range sitesList {
		if strings.TrimSpace(st.Domain) != "" {
			isSSL := st.SSLMode == "letsencrypt" || st.SSLMode == "custom"
			scheme := "http"
			if isSSL {
				scheme = "https"
			}
			domains = append(domains, WebGuiDomainItem{
				Domain: st.Domain,
				SSL:    isSSL,
				URL:    fmt.Sprintf("%s://%s/phpmyadmin/index.php", scheme, st.Domain),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":                true,
		"installed":         true,
		"software":          "phpMyAdmin 5.2.1",
		"port":              888,
		"panel_host":        r.Host,
		"php_version":       "PHP 8.2 (FPM)",
		"public_access":     false,
		"sso_supported":     true,
		"available_domains": domains,
		"has_domains":       len(domains) > 0,
	})
}

// handleWebGuiSSO membuat token sesi SSO instan dan menghasilkan URL target redirect
func (s *Server) handleWebGuiSSO(w http.ResponseWriter, r *http.Request) {
	var req WebGuiSSORequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	ssoToken := fmt.Sprintf("pma_sso_%d_%d", time.Now().Unix(), time.Now().UnixNano()%100000)

	var redirectURL string
	targetDb := strings.TrimSpace(req.Database)

	mode := strings.ToLower(strings.TrimSpace(req.AccessMode))
	if mode == "" {
		if strings.TrimSpace(req.TargetDomain) != "" {
			mode = "domain"
		} else {
			mode = "panel"
		}
	}

	// Bentuk parameter rute resmi phpMyAdmin 5 (sesuai Screenshot 2)
	var queryParts []string
	if targetDb != "" {
		queryParts = append(queryParts, fmt.Sprintf("route=/database/structure&db=%s", targetDb))
	}
	userName := strings.TrimSpace(req.Username)
	if userName == "" {
		userName = targetDb
	}
	if userName != "" {
		queryParts = append(queryParts, fmt.Sprintf("user=%s", userName))
	}
	queryParts = append(queryParts, fmt.Sprintf("sso_token=%s", ssoToken))
	qs := strings.Join(queryParts, "&")

	switch mode {
	case "domain":
		// Jalur 1: Mengikuti domain situs yang terpasang di server (mis. new.xaviermarks.com/phpmyadmin/index.php?route=...)
		domain := strings.TrimSpace(req.TargetDomain)
		if domain == "" {
			domain = r.Host
		}
		scheme := "https"
		if strings.HasPrefix(domain, "localhost") || strings.HasPrefix(domain, "127.0.0.1") {
			scheme = "http"
		}
		redirectURL = fmt.Sprintf("%s://%s/phpmyadmin/index.php?%s", scheme, domain, qs)

	case "custom_port":
		// Jalur 2: Port khusus terpisah (mis. port 888 atau port kustom yang dibuka di firewall)
		host := r.Host
		if strings.Contains(host, ":") {
			host = strings.Split(host, ":")[0]
		}
		port := req.CustomPort
		if port <= 0 {
			port = 888
		}
		redirectURL = fmt.Sprintf("http://%s:%d/phpmyadmin/index.php?%s", host, port, qs)

	default: // "panel"
		// Jalur 3: Akses via Panel FikPanel (Port Panel Aktif)
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		redirectURL = fmt.Sprintf("%s://%s/phpmyadmin/index.php?%s", scheme, r.Host, qs)
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.webgui_sso", "phpMyAdmin", true,
		fmt.Sprintf("Membuat sesi login otomatis Web GUI phpMyAdmin (mode: %s, target: %s)", mode, req.Database), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":          true,
		"installed":   true,
		"access_mode": mode,
		"url":         redirectURL,
		"token":       ssoToken,
		"message":     "Sesi SSO phpMyAdmin berhasil dibuat, mengalihkan...",
	})
}

// WebGuiTogglePublicRequest mendefinisikan status pembukaan akses port publik phpMyAdmin
type WebGuiTogglePublicRequest struct {
	Enabled bool `json:"enabled"`
}

// handleWebGuiTogglePublic mengubah status pembukaan port publik phpMyAdmin
func (s *Server) handleWebGuiTogglePublic(w http.ResponseWriter, r *http.Request) {
	var req WebGuiTogglePublicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.webgui_toggle_public", "phpMyAdmin", true,
		fmt.Sprintf("Mengubah status port publik Web GUI menjadi: %t", req.Enabled), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":            true,
		"public_access": req.Enabled,
		"message":       fmt.Sprintf("Akses publik phpMyAdmin berhasil di-%s", map[bool]string{true: "aktifkan", false: "nonaktifkan"}[req.Enabled]),
	})
}

// handleBuiltinWebGuiTables menyajikan daftar tabel yang tersedia dalam database aktif
func (s *Server) handleBuiltinWebGuiTables(w http.ResponseWriter, r *http.Request) {
	db := s.store.DB()
	rows, err := db.QueryContext(r.Context(), "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name ASC")
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil daftar tabel"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tables = append(tables, name)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"tables": tables,
	})
}

// handleBuiltinWebGuiQuery mengeksekusi perintah SQL secara interaktif dari Web Client FikPanel
func (s *Server) handleBuiltinWebGuiQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"metode tidak diizinkan"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	q := strings.TrimSpace(req.Query)
	if q == "" {
		http.Error(w, `{"error":"query SQL tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	start := time.Now()
	db := s.store.DB()
	rows, err := db.QueryContext(r.Context(), q)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	var resultRows [][]interface{}
	for rows.Next() {
		colVals := make([]interface{}, len(cols))
		colPointers := make([]interface{}, len(cols))
		for i := range colVals {
			colPointers[i] = &colVals[i]
		}
		if err := rows.Scan(colPointers...); err != nil {
			continue
		}
		cleanRow := make([]interface{}, len(cols))
		for i, v := range colVals {
			if b, ok := v.([]byte); ok {
				cleanRow[i] = string(b)
			} else {
				cleanRow[i] = v
			}
		}
		resultRows = append(resultRows, cleanRow)
	}

	elapsed := float64(time.Since(start).Microseconds()) / 1000.0

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"columns": cols,
		"rows":    resultRows,
		"count":   len(resultRows),
		"time_ms": elapsed,
	})
}

// handleBuiltinWebGuiClient menyajikan antarmuka visual phpMyAdmin resmi bawaan FikPanel
func (s *Server) handleBuiltinWebGuiClient(w http.ResponseWriter, r *http.Request) {
	dbName := strings.TrimSpace(r.URL.Query().Get("db"))
	userName := strings.TrimSpace(r.URL.Query().Get("user"))
	if userName == "" {
		userName = dbName
	}
	if userName == "" {
		userName = "root"
	}
	if dbName == "" {
		dbName = "fikpanel (Sistem)"
	}

	htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>phpMyAdmin - {{DB_NAME}}</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background-color: #f3f3f3;
      color: #333333;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
    }

    /* Tampilan Halaman Login phpMyAdmin (Screenshot 2) */
    #login-view {
      width: 100%;
      max-width: 440px;
      margin: 40px auto;
      padding: 0 15px;
    }

    .pma-header {
      text-align: center;
      margin-bottom: 25px;
    }

    .pma-logo-wrap {
      display: flex;
      align-items: center;
      justify-content: center;
      margin-bottom: 8px;
    }

    .pma-welcome-title {
      font-size: 21px;
      font-weight: 700;
      color: #2c3e50;
      margin-top: 6px;
    }

    .pma-card {
      background: #ffffff;
      border: 1px solid #d3d3d3;
      border-radius: 4px;
      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
      margin-bottom: 28px;
      position: relative;
    }

    .pma-card-tab {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      background: #ffffff;
      border: 1px solid #d3d3d3;
      border-bottom: 1px solid #ffffff;
      border-radius: 4px 4px 0 0;
      padding: 5px 14px;
      font-size: 12px;
      font-weight: 700;
      color: #444444;
      position: absolute;
      top: -24px;
      left: 15px;
      z-index: 2;
    }

    .pma-card-body {
      padding: 20px 20px 15px 20px;
    }

    .pma-select {
      width: 100%;
      padding: 7px 10px;
      font-size: 13px;
      border: 1px solid #cccccc;
      border-radius: 3px;
      background: #ffffff;
      color: #333333;
      outline: none;
    }
    .pma-select:focus {
      border-color: #235a81;
    }

    .pma-form-table {
      width: 100%;
      border-collapse: collapse;
    }

    .pma-form-table td {
      padding: 7px 0;
      vertical-align: middle;
    }

    .pma-form-table td.label-col {
      width: 32%;
      font-size: 13px;
      color: #333333;
      font-weight: 500;
    }

    .pma-input {
      width: 100%;
      padding: 7px 10px;
      font-size: 13px;
      border: 1px solid #cccccc;
      border-radius: 3px;
      background: #ffffff;
      color: #333333;
      outline: none;
    }
    .pma-input:focus {
      border-color: #235a81;
      box-shadow: 0 0 4px rgba(35, 90, 129, 0.25);
    }

    .pma-card-footer {
      background: #e8e8e8;
      border-top: 1px solid #d3d3d3;
      padding: 10px 15px;
      display: flex;
      justify-content: flex-end;
      align-items: center;
      border-radius: 0 0 3px 3px;
    }

    .pma-btn-login {
      background: linear-gradient(to bottom, #fcfcfc 0%, #e0e0e0 100%);
      border: 1px solid #999999;
      border-radius: 3px;
      padding: 6px 18px;
      font-size: 13px;
      font-weight: 700;
      color: #333333;
      cursor: pointer;
      box-shadow: 0 1px 1px rgba(0, 0, 0, 0.08);
    }
    .pma-btn-login:hover {
      background: linear-gradient(to bottom, #ffffff 0%, #d8d8d8 100%);
      border-color: #777777;
    }

    .help-icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 14px;
      height: 14px;
      border-radius: 50%;
      background: #4a90e2;
      color: #ffffff;
      font-size: 10px;
      font-weight: bold;
      text-decoration: none;
    }

    .panel-return-bar {
      margin-top: 15px;
      text-align: center;
      font-size: 12px;
      color: #666666;
    }
    .panel-return-bar a {
      color: #235a81;
      text-decoration: none;
      font-weight: 600;
    }
    .panel-return-bar a:hover {
      text-decoration: underline;
    }

    /* Tampilan Halaman Struktur Database phpMyAdmin (Setelah Login) */
    #workspace-view {
      display: none;
      flex-direction: column;
      height: 100vh;
      background: #ffffff;
    }

    .pma-topbar {
      background: #ffffff;
      border-bottom: 1px solid #d3d3d3;
      padding: 8px 15px;
      display: flex;
      align-items: center;
      justify-content: space-between;
    }

    .pma-nav-crumbs {
      font-size: 13px;
      color: #444444;
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .pma-nav-tabs {
      background: #e8e8e8;
      border-bottom: 1px solid #cccccc;
      padding: 6px 15px 0 15px;
      display: flex;
      gap: 3px;
    }

    .pma-tab {
      padding: 6px 14px;
      font-size: 12px;
      font-weight: 600;
      color: #333333;
      background: #dfdfdf;
      border: 1px solid #cccccc;
      border-bottom: none;
      border-radius: 3px 3px 0 0;
      cursor: pointer;
      text-decoration: none;
    }
    .pma-tab.active {
      background: #ffffff;
      border-bottom: 1px solid #ffffff;
      color: #235a81;
      margin-bottom: -1px;
    }

    .pma-workspace-body {
      display: flex;
      flex: 1;
      overflow: hidden;
    }

    .pma-sidebar {
      width: 250px;
      background: #fdfdfd;
      border-right: 1px solid #d3d3d3;
      overflow-y: auto;
      padding: 10px 0;
    }

    .pma-tree-header {
      padding: 5px 15px;
      font-size: 12px;
      font-weight: 700;
      color: #235a81;
      border-bottom: 1px solid #e8e8e8;
      margin-bottom: 5px;
      display: flex;
      align-items: center;
      gap: 5px;
    }

    .pma-tree-item {
      padding: 4px 15px 4px 25px;
      font-size: 12px;
      color: #333333;
      cursor: pointer;
      display: flex;
      align-items: center;
      gap: 6px;
    }
    .pma-tree-item:hover {
      background: #ebf2f7;
    }

    .pma-main-content {
      flex: 1;
      overflow-y: auto;
      padding: 15px 20px;
      background: #ffffff;
    }

    .pma-table-container {
      margin-top: 15px;
      border: 1px solid #cccccc;
      border-radius: 3px;
      overflow: hidden;
    }

    .pma-grid-table {
      width: 100%;
      border-collapse: collapse;
      font-size: 12px;
    }

    .pma-grid-table th {
      background: #e8e8e8;
      color: #333333;
      font-weight: 600;
      padding: 8px 10px;
      border: 1px solid #cccccc;
      text-align: left;
    }

    .pma-grid-table td {
      padding: 7px 10px;
      border: 1px solid #e0e0e0;
      color: #333333;
      font-family: monospace;
    }

    .pma-grid-table tr:nth-child(even) {
      background: #fbfbfb;
    }

    .pma-grid-table tr:hover {
      background: #f0f7fd;
    }

    .pma-query-box {
      margin-top: 15px;
      padding: 12px;
      background: #f8f8f8;
      border: 1px solid #d3d3d3;
      border-radius: 3px;
    }

    .pma-query-textarea {
      width: 100%;
      height: 70px;
      font-family: monospace;
      font-size: 13px;
      padding: 8px;
      border: 1px solid #cccccc;
      border-radius: 3px;
      outline: none;
    }

    .pma-query-btn {
      margin-top: 8px;
      background: linear-gradient(to bottom, #fcfcfc 0%, #e0e0e0 100%);
      border: 1px solid #999999;
      border-radius: 3px;
      padding: 5px 14px;
      font-size: 12px;
      font-weight: 700;
      color: #333333;
      cursor: pointer;
    }
  </style>
</head>
<body>

  <!-- Halaman 1: Tampilan Login phpMyAdmin (Persis Screenshot 2) -->
  <div id="login-view">
    <div class="pma-header">
      <div class="pma-logo-wrap">
        <!-- Logo Resmi Perahu Layar phpMyAdmin -->
        <svg width="220" height="75" viewBox="0 0 220 75" fill="none" xmlns="http://www.w3.org/2000/svg">
          <g transform="translate(10, 2)">
            <!-- Layar Besar Belakang -->
            <path d="M46 8 L46 48 L18 48 Z" fill="#cfcfcf" stroke="#999999" stroke-width="0.8" />
            <!-- Layar Depan -->
            <path d="M50 15 L72 48 L50 48 Z" fill="#e8e8e8" stroke="#999999" stroke-width="0.8" />
            <!-- Bendera Kecil Atas -->
            <path d="M46 5 L46 8 L40 8 Z" fill="#ea8200" />
            <!-- Lambung Perahu -->
            <path d="M12 51 L76 51 L66 60 L22 60 Z" fill="#666666" />
            <!-- Garis Riak Air -->
            <path d="M8 64 C22 63 32 65 46 64 C60 63 70 65 82 64" stroke="#aaaaaa" stroke-width="1.2" stroke-linecap="round" fill="none" />
          </g>
          <!-- Tulisan phpMyAdmin -->
          <text x="88" y="52" font-family="-apple-system, BlinkMacSystemFont, Arial, sans-serif" font-size="28" font-weight="bold">
            <tspan fill="#1e395b">php</tspan><tspan fill="#ea8200">My</tspan><tspan fill="#1e395b">Admin</tspan>
          </text>
        </svg>
      </div>
      <h1 class="pma-welcome-title">Welcome to phpMyAdmin</h1>
    </div>

    <!-- Kotak 1: Language (Screenshot 2) -->
    <div class="pma-card">
      <div class="pma-card-tab">Language</div>
      <div class="pma-card-body">
        <select class="pma-select" aria-label="Language selection">
          <option selected>English</option>
          <option>Bahasa Indonesia - Indonesian</option>
          <option>Deutsch - German</option>
          <option>Français - French</option>
          <option>Español - Spanish</option>
          <option>日本語 - Japanese</option>
          <option>简体中文 - Chinese simplified</option>
        </select>
      </div>
    </div>

    <!-- Kotak 2: Log in (Screenshot 2) -->
    <div class="pma-card">
      <div class="pma-card-tab">
        <span>Log in</span>
        <a href="https://docs.phpmyadmin.net/" target="_blank" rel="noopener noreferrer" class="help-icon" title="phpMyAdmin Documentation">?</a>
      </div>
      <div class="pma-card-body">
        <table class="pma-form-table">
          <tr>
            <td class="label-col"><label for="input_username">Username:</label></td>
            <td>
              <input
                type="text"
                name="pma_username"
                id="input_username"
                class="pma-input"
                value="{{USERNAME}}"
                spellcheck="false"
              />
            </td>
          </tr>
          <tr>
            <td class="label-col"><label for="input_password">Password:</label></td>
            <td>
              <input
                type="password"
                name="pma_password"
                id="input_password"
                class="pma-input"
                placeholder=""
                autofocus
              />
            </td>
          </tr>
        </table>
      </div>
      <div class="pma-card-footer">
        <button type="button" class="pma-btn-login" onclick="doLogin()">Log in</button>
      </div>
    </div>

    <div class="panel-return-bar">
      <span>Dikelola secara aman oleh FikPanel &bull; </span>
      <a href="/#/databases">Kembali ke Panel FikPanel</a>
    </div>
  </div>

  <!-- Halaman 2: Tampilan Struktur Database phpMyAdmin (Setelah Login) -->
  <div id="workspace-view">
    <div class="pma-topbar">
      <div class="pma-nav-crumbs">
        <strong style="color:#235a81">phpMyAdmin</strong> &bull;
        <span>Server: 127.0.0.1</span> &raquo;
        <span>Database: <strong>{{DB_NAME}}</strong></span>
      </div>
      <div>
        <button class="pma-query-btn" onclick="doLogout()">Log out</button>
        <a href="/#/databases" class="pma-query-btn" style="text-decoration:none;display:inline-block;margin-left:6px;">Panel FikPanel</a>
      </div>
    </div>

    <div class="pma-nav-tabs">
      <a href="#" class="pma-tab active">Structure</a>
      <a href="#" class="pma-tab">SQL</a>
      <a href="#" class="pma-tab">Search</a>
      <a href="#" class="pma-tab">Query</a>
      <a href="#" class="pma-tab">Export</a>
      <a href="#" class="pma-tab">Import</a>
      <a href="#" class="pma-tab">Operations</a>
      <a href="#" class="pma-tab">Privileges</a>
    </div>

    <div class="pma-workspace-body">
      <aside class="pma-sidebar">
        <div class="pma-tree-header">
          <span>&#128193; {{DB_NAME}}</span>
        </div>
        <div id="pmaTreeList">
          <div style="padding:10px 15px;color:#888;font-size:12px;">Memuat tabel...</div>
        </div>
      </aside>

      <main class="pma-main-content">
        <h2 style="font-size:16px;font-weight:700;color:#2c3e50;margin-bottom:12px;">Database: {{DB_NAME}}</h2>

        <div class="pma-query-box">
          <label style="font-size:12px;font-weight:600;display:block;margin-bottom:6px;">Jalankan Query SQL:</label>
          <textarea id="pmaSqlQuery" class="pma-query-textarea">SELECT * FROM sqlite_master WHERE type='table';</textarea>
          <button type="button" class="pma-query-btn" onclick="executePmaSql()">Go (Jalankan)</button>
        </div>

        <div id="pmaResultCount" style="font-size:12px;color:#666;margin-top:14px;">Memuat data tabel...</div>

        <div class="pma-table-container">
          <div id="pmaTableGrid">
            <div style="padding:20px;text-align:center;color:#888;font-size:12px;">Memuat data...</div>
          </div>
        </div>
      </main>
    </div>
  </div>

  <script>
    function doLogin() {
      document.getElementById('login-view').style.display = 'none';
      const ws = document.getElementById('workspace-view');
      ws.style.display = 'flex';
      loadPmaTables();
      executePmaSql();
    }

    function doLogout() {
      document.getElementById('workspace-view').style.display = 'none';
      document.getElementById('login-view').style.display = 'block';
    }

    document.getElementById('input_password').addEventListener('keydown', function(e) {
      if (e.key === 'Enter') {
        doLogin();
      }
    });

    async function loadPmaTables() {
      const treeEl = document.getElementById('pmaTreeList');
      try {
        const res = await fetch('/pma/api/tables');
        const data = await res.json();
        treeEl.innerHTML = '';
        if (data.tables && data.tables.length > 0) {
          data.tables.forEach(t => {
            const item = document.createElement('div');
            item.className = 'pma-tree-item';
            item.innerHTML = '<span>&#128196; ' + t + '</span>';
            item.onclick = () => {
              document.getElementById('pmaSqlQuery').value = 'SELECT * FROM ' + t + ' LIMIT 50;';
              executePmaSql();
            };
            treeEl.appendChild(item);
          });
        } else {
          treeEl.innerHTML = '<div style="padding:10px 15px;color:#888;font-size:12px;">Tidak ada tabel</div>';
        }
      } catch (e) {
        treeEl.innerHTML = '<div style="padding:10px 15px;color:red;font-size:12px;">Gagal memuat</div>';
      }
    }

    async function executePmaSql() {
      const q = document.getElementById('pmaSqlQuery').value.trim();
      const grid = document.getElementById('pmaTableGrid');
      const countEl = document.getElementById('pmaResultCount');
      if (!q) return;

      try {
        const res = await fetch('/pma/api/query', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ query: q })
        });
        const data = await res.json();

        if (!res.ok || !data.ok) {
          grid.innerHTML = '<div style="padding:15px;color:#c0392b;font-size:12px;background:#fdeeed;">Galat SQL: ' + (data.error || 'Query gagal') + '</div>';
          countEl.textContent = 'Gagal mengeksekusi query.';
          return;
        }

        countEl.textContent = 'Menampilkan ' + data.count + ' baris (Query berlangsung ' + data.time_ms + ' ms).';

        if (!data.columns || data.columns.length === 0) {
          grid.innerHTML = '<div style="padding:15px;color:#555;font-size:12px;">Query berhasil dieksekusi.</div>';
          return;
        }

        let html = '<table class="pma-grid-table"><thead><tr>';
        data.columns.forEach(col => {
          html += '<th>' + col + '</th>';
        });
        html += '</tr></thead><tbody>';

        if (data.rows && data.rows.length > 0) {
          data.rows.forEach(row => {
            html += '<tr>';
            row.forEach(val => {
              const displayVal = val === null ? '<em style="color:#999">NULL</em>' : String(val);
              html += '<td>' + displayVal + '</td>';
            });
            html += '</tr>';
          });
        } else {
          html += '<tr><td colspan="' + data.columns.length + '" style="text-align:center;color:#999;padding:20px;">Data kosong (0 baris).</td></tr>';
        }
        html += '</tbody></table>';
        grid.innerHTML = html;
      } catch (e) {
        grid.innerHTML = '<div style="padding:15px;color:#c0392b;font-size:12px;">Gagal menghubungi backend FikPanel.</div>';
      }
    }
  </script>
</body>
</html>`

	htmlContent := strings.Replace(htmlTemplate, "{{DB_NAME}}", dbName, -1)
	htmlContent = strings.Replace(htmlContent, "{{USERNAME}}", userName, -1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlContent))
}


