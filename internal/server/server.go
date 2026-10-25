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

	"fikpanel/internal/ai"
	"fikpanel/internal/database"
	"fikpanel/internal/metrics"
	"fikpanel/internal/nginx"
	"fikpanel/internal/services"
	"fikpanel/internal/sites"
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
	nginx    *nginx.Manager
	sites    *sites.Manager
	ai       *ai.Client
	tasks    *database.TaskManager
	limiter  *RateLimiter

	// bgCtx/bgCancel mengatur umur goroutine background (watchdog) hingga shutdown
	bgCtx    context.Context
	bgCancel context.CancelFunc
}

// NewServer membuat instansiasi Server baru dengan routing terdaftar
func NewServer(s *store.Store, m *metrics.Hub, svc *services.Manager, port int) (*Server, error) {
	mux := http.NewServeMux()

	srv := &Server{
		mux:      mux,
		store:    s,
		metrics:  m,
		services: svc,
		nginx:    nginx.NewManager(nginx.DefaultPaths(), s, s),
		tasks:    database.NewTaskManager(s),
		limiter:  NewRateLimiter(),
	}
	srv.bgCtx, srv.bgCancel = context.WithCancel(context.Background())
	srv.sites = sites.NewManager(sites.DefaultPaths(), srv.nginx, s)
	srv.ai = ai.NewClient(s, m, srv.nginx)

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
	s.mux.Handle("GET /api/sites/meta", s.AuthMiddleware(http.HandlerFunc(s.handleSiteMeta)))
	s.mux.Handle("POST /api/sites/check-domains", s.AuthMiddleware(http.HandlerFunc(s.handleCheckDomains)))
	s.mux.Handle("GET /api/sites/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleGetSite)))
	s.mux.Handle("PUT /api/sites/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleUpdateSite)))
	s.mux.Handle("GET /api/sites/{id}/vhost", s.AuthMiddleware(http.HandlerFunc(s.handleSiteVhost)))
	s.mux.Handle("POST /api/sites/{id}/ssl", s.AuthMiddleware(http.HandlerFunc(s.handleSiteSSL)))
	s.mux.Handle("POST /api/sites/{id}/toggle", s.AuthMiddleware(http.HandlerFunc(s.handleToggleSite)))
	s.mux.Handle("DELETE /api/sites/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteSite)))

	// Deteksi & Manajemen Software Sistem
	s.mux.Handle("GET /api/system/software", s.AuthMiddleware(http.HandlerFunc(s.handleListSoftware)))
	s.mux.Handle("POST /api/system/software/install", s.AuthMiddleware(http.HandlerFunc(s.handleInstallSoftware)))

	// Audit Log Keamanan
	s.mux.Handle("GET /api/audit", s.AuthMiddleware(http.HandlerFunc(s.handleListAudit)))

	// Terminal Web Aman
	s.mux.Handle("POST /api/terminal/exec", s.AuthMiddleware(http.HandlerFunc(s.handleTerminalExec)))

	// AI Copilot & Manajemen Provider LLM
	s.mux.Handle("GET /api/ai/providers", s.AuthMiddleware(http.HandlerFunc(s.handleListAIProviders)))
	s.mux.Handle("POST /api/ai/providers", s.AuthMiddleware(http.HandlerFunc(s.handleCreateAIProvider)))
	s.mux.Handle("PUT /api/ai/providers/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleUpdateAIProvider)))
	s.mux.Handle("DELETE /api/ai/providers/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteAIProvider)))
	s.mux.Handle("POST /api/ai/providers/{id}/default", s.AuthMiddleware(http.HandlerFunc(s.handleSetDefaultAIProvider)))
	s.mux.Handle("POST /api/ai/providers/test", s.AuthMiddleware(http.HandlerFunc(s.handleTestAIProvider)))

	// AI Riwayat Percakapan (Conversations & Messages)
	s.mux.Handle("GET /api/ai/conversations", s.AuthMiddleware(http.HandlerFunc(s.handleListAIConversations)))
	s.mux.Handle("POST /api/ai/conversations", s.AuthMiddleware(http.HandlerFunc(s.handleCreateAIConversation)))
	s.mux.Handle("DELETE /api/ai/conversations/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteAIConversation)))
	s.mux.Handle("DELETE /api/ai/conversations", s.AuthMiddleware(http.HandlerFunc(s.handleClearAIConversations)))
	s.mux.Handle("GET /api/ai/conversations/{id}/messages", s.AuthMiddleware(http.HandlerFunc(s.handleGetAIMessages)))
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

	// Manajemen Database & Engine Terkelola
	s.mux.Handle("GET /api/databases/engines", s.AuthMiddleware(http.HandlerFunc(s.handleListEngines)))
	s.mux.Handle("POST /api/databases/install", s.AuthMiddleware(http.HandlerFunc(s.handleInstallDatabase)))
	s.mux.Handle("GET /api/databases/{engine}/list", s.AuthMiddleware(http.HandlerFunc(s.handleListDatabases)))
	s.mux.Handle("POST /api/databases/{engine}/create", s.AuthMiddleware(http.HandlerFunc(s.handleCreateDatabase)))
	s.mux.Handle("DELETE /api/databases/{engine}/{name}", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteDatabase)))
	s.mux.Handle("POST /api/databases/{engine}/service", s.AuthMiddleware(http.HandlerFunc(s.handleEngineService)))
	s.mux.Handle("POST /api/databases/{engine}/switch-version", s.AuthMiddleware(http.HandlerFunc(s.handleEngineSwitchVersion)))
	s.mux.Handle("GET /api/databases/{engine}/config", s.AuthMiddleware(http.HandlerFunc(s.handleEngineGetConfig)))
	s.mux.Handle("POST /api/databases/{engine}/config", s.AuthMiddleware(http.HandlerFunc(s.handleEngineSaveConfig)))
	s.mux.Handle("GET /api/databases/{engine}/logs", s.AuthMiddleware(http.HandlerFunc(s.handleEngineLogs)))

	// Antrean Tugas Sistem & Streaming Log Real-time
	s.mux.Handle("GET /api/system/tasks", s.AuthMiddleware(http.HandlerFunc(s.handleListTasks)))
	s.mux.Handle("GET /api/system/tasks/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleGetTask)))
	s.mux.Handle("GET /api/system/tasks/{id}/stream", s.AuthMiddleware(http.HandlerFunc(s.handleStreamTask)))


	// Nginx Manager (layanan, config, versi, beban, optimasi, log error)
	nginxRoutes := map[string]http.HandlerFunc{
		"GET /api/software/nginx/status":           s.handleNginxStatus,
		"POST /api/software/nginx/service":         s.handleNginxService,
		"GET /api/software/nginx/settings":         s.handleNginxGetSettings,
		"POST /api/software/nginx/settings":        s.handleNginxSaveSettings,
		"GET /api/software/nginx/config":           s.handleNginxGetConfig,
		"POST /api/software/nginx/config":          s.handleNginxSaveConfig,
		"GET /api/software/nginx/versions":         s.handleNginxVersions,
		"POST /api/software/nginx/versions/switch": s.handleNginxSwitchVersion,
		"GET /api/software/nginx/load":             s.handleNginxLoad,
		"GET /api/software/nginx/optimize":         s.handleNginxGetOptimize,
		"POST /api/software/nginx/optimize":        s.handleNginxSaveOptimize,
		"GET /api/software/nginx/error-log":        s.handleNginxErrorLog,
		"POST /api/software/nginx/error-log/clear": s.handleNginxClearErrorLog,
	}
	for pattern, h := range nginxRoutes {
		s.mux.Handle(pattern, s.AuthMiddleware(h))
	}

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

		// Terapkan Content Security Policy ketat pada mode produksi/statis (bukan proxy Vite dev)
		setCSP := func() {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
		}

		// 2. Jika Vite tidak aktif, sajikan dari disk lokal (agar refresh langsung membaca file terbaru tanpa restart Go)
		if stat, err := os.Stat(localDistDir); err == nil && stat.IsDir() {
			setCSP()
			localFileServer.ServeHTTP(w, r)
			return
		}

		// 3. Fallback biner produksi (disematkan via embed.FS)
		if embedFileServer != nil {
			setCSP()
			embedFileServer.ServeHTTP(w, r)
			return
		}

		http.NotFound(w, r)
	})
}

// Start menjalankan listener HTTP
func (s *Server) Start() error {
	go s.nginx.RunWatchdog(s.bgCtx, time.Minute)
	go s.sites.RunRenewLoop(s.bgCtx, func(ok bool, target, msg string) {
		_ = s.store.RecordAudit(s.bgCtx, "system", "ssl.renew", target, ok, msg, nil)
	})
	return s.server.ListenAndServe()
}

// Shutdown menghentikan server secara graceful dengan batas waktu
func (s *Server) Shutdown(ctx context.Context) error {
	s.bgCancel()
	return s.server.Shutdown(ctx)
}
