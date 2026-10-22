package server

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"fikpanel/internal/store"
	"fikpanel/web"
)

// Server mengelola siklus hidup HTTP router dan middleware unprivileged
type Server struct {
	mux    *http.ServeMux
	server *http.Server
	store  *store.Store
}

// NewServer membuat instansiasi Server baru dengan routing terdaftar
func NewServer(s *store.Store, port int) (*Server, error) {
	mux := http.NewServeMux()

	srv := &Server{
		mux:   mux,
		store: s,
	}

	srv.registerRoutes()

	srv.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      srv.mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return srv, nil
}

// registerRoutes mendaftarkan seluruh endpoint REST API dan handler aset statis Svelte
func (s *Server) registerRoutes() {
	// Endpoint pemeriksaan kesehatan (health check)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

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

