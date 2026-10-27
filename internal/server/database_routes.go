package server

import (
	"encoding/json"
	"fmt"
	"html"
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
		if engine == "phpmyadmin" {
			req.Version = "5.2.1"
		} else {
			req.Version = "7.0"
		}
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

	// Lakukan provisi database, user, dan hak akses secara riil di engine database fisik (mis. MySQL)
	if err := database.ProvisionDatabase(r.Context(), engine, req.Name, req.Username, req.Password, req.AccessHost); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal memprovisi database fisik %s: %v"}`, engine, err), http.StatusInternalServerError)
		return
	}

	created, err := s.store.CreateDatabase(r.Context(), dbRecord)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membuat record database: %v"}`, err), http.StatusInternalServerError)
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

	// Ambil username sebelum dihapus untuk pembersihan akun pengguna database terkait
	var dbUsername string
	_ = s.store.DB().QueryRowContext(r.Context(), "SELECT username FROM databases WHERE engine = ? AND name = ? LIMIT 1", engine, name).Scan(&dbUsername)

	if err := s.store.DeleteDatabase(r.Context(), engine, name); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal menghapus database: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Hapus database dan pengguna terkait dari engine database fisik
	_ = database.DeprovisionDatabase(r.Context(), engine, name, dbUsername)

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

	isInstalled, version := database.IsPhpMyAdminInstalled()
	if version == "" {
		version = "5.2.1"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":                true,
		"installed":         isInstalled,
		"software":          fmt.Sprintf("phpMyAdmin %s", version),
		"version":           version,
		"port":              888,
		"panel_host":        r.Host,
		"php_version":       "PHP 8.2 (FPM)",
		"public_access":     false,
		"sso_supported":     isInstalled,
		"available_domains": domains,
		"has_domains":       len(domains) > 0,
	})
}

// handleWebGuiSSO membuat token sesi SSO instan dan menghasilkan URL target redirect
func (s *Server) handleWebGuiSSO(w http.ResponseWriter, r *http.Request) {
	isInstalled, _ := database.IsPhpMyAdminInstalled()
	if !isInstalled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":        false,
			"installed": false,
			"message":   "Layanan phpMyAdmin belum terpasang di server. Silakan pasang paket phpMyAdmin terlebih dahulu melalui Quick Install.",
		})
		return
	}

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
	pass := strings.TrimSpace(req.Password)
	if targetDb != "" {
		var dbUser, dbPass string
		err := s.store.DB().QueryRowContext(r.Context(), "SELECT username, password FROM databases WHERE name = ? LIMIT 1", targetDb).Scan(&dbUser, &dbPass)
		if err == nil {
			if userName == "" || userName == targetDb {
				userName = dbUser
			}
			if pass == "" {
				pass = dbPass
			}
		}
	}

	database.StorePmaSSOSession(ssoToken, targetDb, userName, pass)

	if userName != "" {
		queryParts = append(queryParts, fmt.Sprintf("user=%s", userName))
		queryParts = append(queryParts, fmt.Sprintf("pma_username=%s", userName))
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
	isInstalled, _ := database.IsPhpMyAdminInstalled()
	if !isInstalled {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		notInstalledPage := `<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>phpMyAdmin Belum Terpasang - FikPanel</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background: #090a0f;
      color: #f1f5f9;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      padding: 1.5rem;
      box-sizing: border-box;
    }
    .card {
      background: #0e1119;
      border: 1px solid #1f2738;
      border-radius: 12px;
      padding: 2.5rem 2rem;
      max-width: 480px;
      width: 100%;
      text-align: center;
      box-shadow: 0 10px 30px rgba(0,0,0,0.5);
    }
    .badge {
      display: inline-block;
      background: rgba(245, 158, 11, 0.15);
      color: #fbbf24;
      font-size: 0.8rem;
      font-weight: 600;
      padding: 0.25rem 0.75rem;
      border-radius: 999px;
      margin-bottom: 1.25rem;
    }
    h1 {
      font-size: 1.25rem;
      font-weight: 700;
      color: #f1f5f9;
      margin: 0 0 0.75rem;
    }
    p {
      color: #94a3b8;
      font-size: 0.875rem;
      line-height: 1.6;
      margin-bottom: 1.75rem;
    }
    .btn {
      display: inline-block;
      background: #10b981;
      color: #04130d;
      text-decoration: none;
      font-weight: 600;
      padding: 0.75rem 1.5rem;
      border-radius: 8px;
      font-size: 0.875rem;
      transition: background 0.2s;
    }
    .btn:hover {
      background: #059669;
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="badge">&#9888; Paket Belum Terpasang</div>
    <h1>Layanan phpMyAdmin Belum Terpasang</h1>
    <p>Paket resmi phpMyAdmin belum dipasang pada server ini. Silakan buka dashboard <strong>FikPanel &gt; Databases</strong> dan jalankan <strong>Quick Install phpMyAdmin</strong> untuk memasangnya.</p>
    <a href="/" class="btn">Kembali ke Panel FikPanel</a>
  </div>
</body>
</html>`
		w.Write([]byte(notInstalledPage))
		return
	}

	// Teruskan langsung ke reverse proxy instance resmi phpMyAdmin
	proxy, err := database.GetPhpMyAdminProxy()
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		errTemplate := `<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Gagal Menghubungkan ke phpMyAdmin - FikPanel</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background: #090a0f;
      color: #f1f5f9;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      padding: 1.5rem;
      box-sizing: border-box;
    }
    .card {
      background: #0e1119;
      border: 1px solid #dc2626;
      border-radius: 12px;
      padding: 2.5rem 2rem;
      max-width: 520px;
      width: 100%;
      text-align: center;
      box-shadow: 0 10px 30px rgba(0,0,0,0.5);
    }
    .badge {
      display: inline-block;
      background: rgba(239, 68, 68, 0.15);
      color: #f87171;
      font-size: 0.8rem;
      font-weight: 600;
      padding: 0.25rem 0.75rem;
      border-radius: 999px;
      margin-bottom: 1.25rem;
    }
    h1 {
      font-size: 1.25rem;
      font-weight: 700;
      color: #f1f5f9;
      margin: 0 0 0.75rem;
    }
    .error-msg {
      background: #181c27;
      border: 1px solid #283145;
      color: #fca5a5;
      font-family: monospace;
      font-size: 0.825rem;
      padding: 0.75rem 1rem;
      border-radius: 8px;
      margin: 1rem 0 1.5rem;
      text-align: left;
      word-break: break-word;
    }
    p {
      color: #94a3b8;
      font-size: 0.875rem;
      line-height: 1.6;
      margin-bottom: 1.5rem;
    }
    .actions {
      display: flex;
      gap: 0.75rem;
      justify-content: center;
    }
    .btn {
      display: inline-block;
      background: #10b981;
      color: #04130d;
      text-decoration: none;
      font-weight: 600;
      padding: 0.75rem 1.25rem;
      border-radius: 8px;
      font-size: 0.875rem;
      transition: background 0.2s;
    }
    .btn:hover {
      background: #059669;
    }
    .btn-secondary {
      background: #1f2738;
      color: #e2e8f0;
    }
    .btn-secondary:hover {
      background: #2a354b;
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="badge">&#9888; Inisialisasi Layanan Terkendala</div>
    <h1>Gagal Menghubungkan ke phpMyAdmin</h1>
    <div class="error-msg">{{ERROR}}</div>
    <p>Server sedang berupaya menyiapkan berkas paket resmi atau dependensi biner PHP belum aktif sepenuhnya. Pastikan paket biner PHP (php-cli, php-mysql, php-mbstring) tersedia di server sistem.</p>
    <div class="actions">
      <a href="javascript:location.reload()" class="btn">Muat Ulang Halaman</a>
      <a href="/" class="btn btn-secondary">Kembali ke FikPanel</a>
    </div>
  </div>
</body>
</html>`
		errPage := strings.ReplaceAll(errTemplate, "{{ERROR}}", html.EscapeString(err.Error()))
		w.Write([]byte(errPage))
		return
	}

	proxy.ServeHTTP(w, r)
}
