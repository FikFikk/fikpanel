package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
