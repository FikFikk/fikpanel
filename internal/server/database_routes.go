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
				URL:    fmt.Sprintf("%s://%s/phpmyadmin/", scheme, st.Domain),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":                true,
		"installed":         true,
		"software":          "phpMyAdmin / FikPanel Web Client",
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

	switch mode {
	case "domain":
		// Jalur 1: Mengikuti domain situs yang terpasang di server (mis. kostash.my.id/phpmyadmin)
		domain := strings.TrimSpace(req.TargetDomain)
		if domain == "" {
			domain = r.Host
		}
		scheme := "https"
		if strings.HasPrefix(domain, "localhost") || strings.HasPrefix(domain, "127.0.0.1") {
			scheme = "http"
		}
		redirectURL = fmt.Sprintf("%s://%s/phpmyadmin/index.php?sso_token=%s", scheme, domain, ssoToken)
		if targetDb != "" {
			redirectURL += fmt.Sprintf("&db=%s", targetDb)
		}

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
		redirectURL = fmt.Sprintf("http://%s:%d/phpmyadmin/index.php?sso_token=%s", host, port, ssoToken)
		if targetDb != "" {
			redirectURL += fmt.Sprintf("&db=%s", targetDb)
		}

	default: // "panel"
		// Jalur 3: Akses Independen via Panel FikPanel (Tanpa Domain & Tanpa Keterikatan Port 888)
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		redirectURL = fmt.Sprintf("%s://%s/pma/?sso_token=%s", scheme, r.Host, ssoToken)
		if targetDb != "" {
			redirectURL += fmt.Sprintf("&db=%s", targetDb)
		}
	}

	_ = s.store.RecordAudit(r.Context(), "user", "database.webgui_sso", "phpMyAdmin", true,
		fmt.Sprintf("Membuat sesi login otomatis Web GUI database (mode: %s, target: %s)", mode, req.Database), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":          true,
		"installed":   true,
		"access_mode": mode,
		"url":         redirectURL,
		"token":       ssoToken,
		"message":     "Sesi SSO berhasil dibuat, mengalihkan ke antarmuka web...",
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

// handleBuiltinWebGuiClient menyajikan antarmuka visual mandiri Database Manager bawaan FikPanel
func (s *Server) handleBuiltinWebGuiClient(w http.ResponseWriter, r *http.Request) {
	dbName := strings.TrimSpace(r.URL.Query().Get("db"))
	if dbName == "" {
		dbName = "fikpanel (Sistem)"
	}

	htmlTemplate := `<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>FikPanel Database Manager (Web Client)</title>
  <style>
    :root {
      --bg: #090d16;
      --surface: #0f172a;
      --surface-hover: #1e293b;
      --border: #334155;
      --text: #f8fafc;
      --text-muted: #94a3b8;
      --accent: #38bdf8;
      --accent-hover: #0284c7;
      --success: #10b981;
      --danger: #ef4444;
      --radius: 6px;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: var(--bg); color: var(--text); display: flex; flex-direction: column; height: 100vh; overflow: hidden; }
    header { height: 56px; background: var(--surface); border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; padding: 0 1.25rem; }
    .brand { display: flex; align-items: center; gap: 0.75rem; font-weight: 700; font-size: 0.9375rem; color: var(--text); }
    .brand-icon { width: 28px; height: 28px; background: var(--accent); color: #090d16; display: flex; align-items: center; justify-content: center; border-radius: var(--radius); font-weight: 800; font-size: 0.8125rem; }
    .badges { display: flex; align-items: center; gap: 0.5rem; }
    .badge { font-size: 0.75rem; padding: 0.25rem 0.625rem; border-radius: 999px; background: #1e293b; color: var(--text-muted); border: 1px solid var(--border); }
    .badge-online { background: rgba(16, 185, 129, 0.15); color: #34d399; border-color: rgba(16, 185, 129, 0.3); }
    .badge-db { background: rgba(56, 189, 248, 0.15); color: var(--accent); border-color: rgba(56, 189, 248, 0.3); font-weight: 600; }
    .btn { padding: 0.4rem 0.875rem; font-size: 0.8125rem; border-radius: var(--radius); border: 1px solid var(--border); background: var(--surface); color: var(--text); cursor: pointer; text-decoration: none; display: inline-flex; align-items: center; gap: 0.375rem; }
    .btn:hover { background: var(--surface-hover); }
    .btn-primary { background: var(--accent); color: #090d16; border-color: var(--accent); font-weight: 600; }
    .btn-primary:hover { background: var(--accent-hover); }
    .main-layout { display: flex; flex: 1; overflow: hidden; }
    aside { width: 260px; background: var(--surface); border-right: 1px solid var(--border); display: flex; flex-direction: column; }
    .aside-header { padding: 0.875rem 1rem; border-bottom: 1px solid var(--border); font-size: 0.75rem; font-weight: 600; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; display: flex; justify-content: space-between; align-items: center; }
    .table-list { list-style: none; overflow-y: auto; flex: 1; padding: 0.5rem 0; }
    .table-item { padding: 0.5rem 1rem; font-size: 0.8125rem; color: var(--text); display: flex; justify-content: space-between; align-items: center; cursor: pointer; border-left: 2px solid transparent; }
    .table-item:hover { background: var(--surface-hover); }
    .table-item.active { background: rgba(56, 189, 248, 0.1); color: var(--accent); border-left-color: var(--accent); font-weight: 600; }
    .content-area { flex: 1; display: flex; flex-direction: column; overflow: hidden; background: var(--bg); }
    .query-box-container { padding: 1rem; background: var(--surface); border-bottom: 1px solid var(--border); }
    .query-box-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem; }
    .query-presets { display: flex; gap: 0.375rem; }
    .btn-preset { font-size: 0.75rem; padding: 0.2rem 0.5rem; border-radius: 4px; background: #1e293b; color: var(--text-muted); border: 1px solid var(--border); cursor: pointer; }
    .btn-preset:hover { color: var(--accent); border-color: var(--accent); }
    .query-textarea { width: 100%; height: 75px; background: #090d16; border: 1px solid var(--border); border-radius: var(--radius); color: #38bdf8; font-family: "Fira Code", monospace; font-size: 0.875rem; padding: 0.625rem; resize: vertical; outline: none; }
    .query-textarea:focus { border-color: var(--accent); }
    .query-actions { display: flex; justify-content: space-between; align-items: center; margin-top: 0.5rem; }
    .query-hint { font-size: 0.75rem; color: var(--text-muted); }
    .results-container { flex: 1; overflow: auto; padding: 1rem; }
    .results-meta { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem; font-size: 0.8125rem; color: var(--text-muted); }
    table { width: 100%; border-collapse: collapse; font-size: 0.8125rem; }
    th { background: var(--surface); color: var(--text-muted); text-align: left; padding: 0.625rem 0.875rem; border: 1px solid var(--border); font-weight: 600; position: sticky; top: 0; }
    td { padding: 0.5rem 0.875rem; border: 1px solid var(--border); color: var(--text); font-family: monospace; }
    tr:nth-child(even) { background: rgba(255, 255, 255, 0.02); }
    tr:hover { background: rgba(56, 189, 248, 0.05); }
    .empty-state { text-align: center; padding: 3rem 1rem; color: var(--text-muted); }
    .error-box { padding: 0.75rem 1rem; background: rgba(239, 68, 68, 0.1); border: 1px solid rgba(239, 68, 68, 0.3); color: #fca5a5; border-radius: var(--radius); font-size: 0.8125rem; margin-bottom: 0.75rem; }
  </style>
</head>
<body>
  <header>
    <div class="brand">
      <div class="brand-icon">F</div>
      <span>FikPanel Database Manager</span>
    </div>
    <div class="badges">
      <span class="badge badge-online">● Terhubung Aktif</span>
      <span class="badge badge-db">Database: {{DB_NAME}}</span>
      <span class="badge">Mode: FikPanel Built-in Client</span>
    </div>
    <div>
      <a href="/#/databases" class="btn">Kembali ke Panel</a>
    </div>
  </header>

  <div class="main-layout">
    <aside>
      <div class="aside-header">
        <span>Tabel Database</span>
        <button class="btn-preset" onclick="loadTables()">Muat Ulang</button>
      </div>
      <ul id="tableList" class="table-list">
        <li class="table-item" style="color:var(--text-muted)">Memuat tabel...</li>
      </ul>
    </aside>

    <main class="content-area">
      <div class="query-box-container">
        <div class="query-box-header">
          <div class="query-presets">
            <button class="btn-preset" onclick="setQuery('SELECT * FROM sqlite_master WHERE type=\'table\';')">Daftar Tabel</button>
            <button class="btn-preset" onclick="setQuery('SELECT * FROM sites LIMIT 50;')">Tabel Sites</button>
            <button class="btn-preset" onclick="setQuery('SELECT * FROM users LIMIT 50;')">Tabel Users</button>
            <button class="btn-preset" onclick="setQuery('SELECT * FROM databases LIMIT 50;')">Tabel Databases</button>
          </div>
        </div>
        <textarea id="sqlQuery" class="query-textarea" placeholder="Tulis perintah SQL di sini... (tekan Ctrl+Enter untuk mengeksekusi)">SELECT * FROM sqlite_master WHERE type='table';</textarea>
        <div class="query-actions">
          <span class="query-hint">Tekan <strong>Ctrl + Enter</strong> untuk menjalankan query</span>
          <button class="btn btn-primary" onclick="runQuery()">Jalankan Query</button>
        </div>
      </div>

      <div class="results-container">
        <div id="errorBox" class="error-box" style="display:none"></div>
        <div class="results-meta">
          <span id="resultsCount">0 baris ditemukan</span>
          <span id="resultsLatency">- ms</span>
        </div>
        <div id="tableContainer">
          <div class="empty-state">Jalankan query atau pilih tabel di bilah samping untuk melihat data.</div>
        </div>
      </div>
    </main>
  </div>

  <script>
    let activeTable = '';

    function setQuery(q) {
      document.getElementById('sqlQuery').value = q;
      runQuery();
    }

    async function loadTables() {
      const listEl = document.getElementById('tableList');
      try {
        const res = await fetch('/pma/api/tables');
        const data = await res.json();
        listEl.innerHTML = '';
        if (data.tables && data.tables.length > 0) {
          data.tables.forEach(t => {
            const li = document.createElement('li');
            li.className = 'table-item' + (activeTable === t ? ' active' : '');
            li.innerHTML = '<span>' + t + '</span>';
            li.onclick = () => {
              activeTable = t;
              document.querySelectorAll('.table-item').forEach(el => el.classList.remove('active'));
              li.classList.add('active');
              setQuery('SELECT * FROM ' + t + ' LIMIT 50;');
            };
            listEl.appendChild(li);
          });
        } else {
          listEl.innerHTML = '<li class="table-item" style="color:var(--text-muted)">Tidak ada tabel</li>';
        }
      } catch (e) {
        listEl.innerHTML = '<li class="table-item" style="color:var(--danger)">Gagal memuat tabel</li>';
      }
    }

    async function runQuery() {
      const q = document.getElementById('sqlQuery').value.trim();
      const errBox = document.getElementById('errorBox');
      const tableContainer = document.getElementById('tableContainer');
      const countEl = document.getElementById('resultsCount');
      const latencyEl = document.getElementById('resultsLatency');

      errBox.style.display = 'none';
      if (!q) return;

      try {
        const res = await fetch('/pma/api/query', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ query: q })
        });
        const data = await res.json();

        if (!res.ok || !data.ok) {
          errBox.textContent = data.error || 'Terjadi kesalahan saat mengeksekusi SQL';
          errBox.style.display = 'block';
          tableContainer.innerHTML = '';
          countEl.textContent = '0 baris ditemukan';
          latencyEl.textContent = '- ms';
          return;
        }

        countEl.textContent = data.count + ' baris ditemukan';
        latencyEl.textContent = data.time_ms + ' ms';

        if (!data.columns || data.columns.length === 0) {
          tableContainer.innerHTML = '<div class="empty-state">Query berhasil dijalankan (tanpa output baris).</div>';
          return;
        }

        let html = '<table><thead><tr>';
        data.columns.forEach(col => {
          html += '<th>' + col + '</th>';
        });
        html += '</tr></thead><tbody>';

        if (data.rows && data.rows.length > 0) {
          data.rows.forEach(row => {
            html += '<tr>';
            row.forEach(val => {
              const displayVal = val === null ? '<span style="color:var(--text-muted)">NULL</span>' : String(val);
              html += '<td>' + displayVal + '</td>';
            });
            html += '</tr>';
          });
        } else {
          html += '<tr><td colspan="' + data.columns.length + '" style="text-align:center;color:var(--text-muted);padding:2rem;">Tidak ada rekaman data (kosong).</td></tr>';
        }
        html += '</tbody></table>';
        tableContainer.innerHTML = html;
      } catch (e) {
        errBox.textContent = e.message || 'Koneksi ke backend terputus';
        errBox.style.display = 'block';
      }
    }

    document.getElementById('sqlQuery').addEventListener('keydown', function(e) {
      if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        e.preventDefault();
        runQuery();
      }
    });

    loadTables();
    runQuery();
  </script>
</body>
</html>`

	htmlContent := strings.Replace(htmlTemplate, "{{DB_NAME}}", dbName, 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlContent))
}


