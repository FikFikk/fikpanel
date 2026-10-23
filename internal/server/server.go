package server

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
	"fikpanel/web"
)

// Server mengelola siklus hidup HTTP router dan middleware unprivileged
type Server struct {
	mux      *http.ServeMux
	server   *http.Server
	store    *store.Store
	metrics  *metrics.Hub
	services *services.Manager
	limiter  *RateLimiter
}

// NewServer membuat instansiasi Server baru dengan routing terdaftar
func NewServer(s *store.Store, m *metrics.Hub, svc *services.Manager, port int) (*Server, error) {
	mux := http.NewServeMux()

	srv := &Server{
		mux:      mux,
		store:    s,
		metrics:  m,
		services: svc,
		limiter:  NewRateLimiter(),
	}

	srv.registerRoutes()

	// Terapkan rantai middleware keamanan
	handler := srv.SecurityHeadersMiddleware(srv.CSRFMiddleware(srv.mux))

	srv.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return srv, nil
}

// registerRoutes mendaftarkan seluruh endpoint REST API dan handler aset statis Svelte
func (s *Server) registerRoutes() {
	// Endpoint pemeriksaan kesehatan (health check) publik
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Endpoint autentikasi publik
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	// Endpoint terproteksi autentikasi sesi
	s.mux.Handle("GET /api/auth/me", s.AuthMiddleware(http.HandlerFunc(s.handleMe)))
	s.mux.Handle("GET /api/metrics/live", s.AuthMiddleware(http.HandlerFunc(s.metrics.ServeHTTP)))
	s.mux.Handle("GET /api/overview", s.AuthMiddleware(http.HandlerFunc(s.handleOverview)))

	// Manajemen Layanan (Services)
	s.mux.Handle("GET /api/services", s.AuthMiddleware(http.HandlerFunc(s.handleListServices)))
	s.mux.Handle("POST /api/services/{unit}/action", s.AuthMiddleware(http.HandlerFunc(s.handleServiceAction)))

	// Manajemen Situs Web (Sites)
	s.mux.Handle("GET /api/sites", s.AuthMiddleware(http.HandlerFunc(s.handleListSites)))
	s.mux.Handle("POST /api/sites", s.AuthMiddleware(http.HandlerFunc(s.handleCreateSite)))
	s.mux.Handle("POST /api/sites/{id}/toggle", s.AuthMiddleware(http.HandlerFunc(s.handleToggleSite)))
	s.mux.Handle("DELETE /api/sites/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteSite)))

	// Deteksi & Manajemen Software Sistem
	s.mux.Handle("GET /api/system/software", s.AuthMiddleware(http.HandlerFunc(s.handleListSoftware)))
	s.mux.Handle("POST /api/system/software/install", s.AuthMiddleware(http.HandlerFunc(s.handleInstallSoftware)))

	// Audit Log Keamanan
	s.mux.Handle("GET /api/audit", s.AuthMiddleware(http.HandlerFunc(s.handleListAudit)))

	// Terminal Web Aman
	s.mux.Handle("POST /api/terminal/exec", s.AuthMiddleware(http.HandlerFunc(s.handleTerminalExec)))

	// AI Copilot Diagnostik
	s.mux.Handle("POST /api/ai/chat", s.AuthMiddleware(http.HandlerFunc(s.handleAIChat)))

	// Manajemen Berkas (File Manager)
	s.mux.Handle("GET /api/files/list", s.AuthMiddleware(http.HandlerFunc(s.handleListFiles)))
	s.mux.Handle("GET /api/files/read", s.AuthMiddleware(http.HandlerFunc(s.handleReadFile)))
	s.mux.Handle("POST /api/files/save", s.AuthMiddleware(http.HandlerFunc(s.handleSaveFile)))
	s.mux.Handle("POST /api/files/new", s.AuthMiddleware(http.HandlerFunc(s.handleNewFile)))
	s.mux.Handle("POST /api/files/rename", s.AuthMiddleware(http.HandlerFunc(s.handleRenameFile)))
	s.mux.Handle("POST /api/files/delete", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteFiles)))
	s.mux.Handle("POST /api/files/chmod", s.AuthMiddleware(http.HandlerFunc(s.handleChmodFile)))
	s.mux.Handle("POST /api/files/upload", s.AuthMiddleware(http.HandlerFunc(s.handleUploadFile)))
	s.mux.Handle("POST /api/files/paste", s.AuthMiddleware(http.HandlerFunc(s.handlePasteFiles)))
	s.mux.Handle("POST /api/files/compress", s.AuthMiddleware(http.HandlerFunc(s.handleCompressFiles)))
	s.mux.Handle("POST /api/files/remote-download", s.AuthMiddleware(http.HandlerFunc(s.handleRemoteDownload)))
	s.mux.Handle("POST /api/files/symlink", s.AuthMiddleware(http.HandlerFunc(s.handleCreateSymlink)))
	s.mux.Handle("POST /api/files/search-content", s.AuthMiddleware(http.HandlerFunc(s.handleSearchContent)))

	// Penyajian aset frontend dinamis (Vite HMR proxy -> Local Disk -> Embedded FS)
	viteURL, _ := url.Parse("http://127.0.0.1:5173")
	viteProxy := httputil.NewSingleHostReverseProxy(viteURL)

	localDistDir := "web/dist"
	localFileServer := http.FileServer(http.Dir(localDistDir))

	distSub, _ := fs.Sub(web.DistFS, "dist")
	var embedFileServer http.Handler
	if distSub != nil {
		embedFileServer = http.FileServer(http.FS(distSub))
	}

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 1. Cek apakah Vite Dev Server aktif di port 5173 (memberikan HMR otomatis seketika saat file disimpan)
		conn, err := net.DialTimeout("tcp", "127.0.0.1:5173", 50*time.Millisecond)
		if err == nil {
			conn.Close()
			viteProxy.ServeHTTP(w, r)
			return
		}

		// 2. Jika Vite tidak aktif, sajikan dari disk lokal (agar refresh langsung membaca file terbaru tanpa restart Go)
		if stat, err := os.Stat(localDistDir); err == nil && stat.IsDir() {
			localFileServer.ServeHTTP(w, r)
			return
		}

		// 3. Fallback biner produksi (disematkan via embed.FS)
		if embedFileServer != nil {
			embedFileServer.ServeHTTP(w, r)
			return
		}

		http.NotFound(w, r)
	})
}

// Start menjalankan listener HTTP
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown menghentikan server secara graceful dengan batas waktu
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
