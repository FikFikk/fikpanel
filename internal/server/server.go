package server

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
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

	// Audit Log Keamanan
	s.mux.Handle("GET /api/audit", s.AuthMiddleware(http.HandlerFunc(s.handleListAudit)))

	// Terminal Web Aman
	s.mux.Handle("POST /api/terminal/exec", s.AuthMiddleware(http.HandlerFunc(s.handleTerminalExec)))

	// AI Copilot Diagnostik
	s.mux.Handle("POST /api/ai/chat", s.AuthMiddleware(http.HandlerFunc(s.handleAIChat)))

	// Sajikan aset statis frontend dari embed.FS web.DistFS
	distSub, err := fs.Sub(web.DistFS, "dist")
	if err == nil {
		s.mux.Handle("/", http.FileServer(http.FS(distSub)))
	}
}

// Start menjalankan listener HTTP
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown menghentikan server secara graceful dengan batas waktu
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
