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
	"fikpanel/internal/appstore"
	"fikpanel/internal/cron"
	"fikpanel/internal/database"
	"fikpanel/internal/discovery"
	"fikpanel/internal/k8s"
	"fikpanel/internal/metrics"
	"fikpanel/internal/nginx"
	"fikpanel/internal/node"
	"fikpanel/internal/php"
	"fikpanel/internal/security"
	"fikpanel/internal/services"
	"fikpanel/internal/sites"
	"fikpanel/internal/store"
	"fikpanel/internal/system"
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
	cronMgr  *cron.Manager
	firewall *security.Manager
	pm2      *node.Manager
	phpMgr   *php.Manager
	systemMgr *system.Manager
	appStore *appstore.Manager
	discoveryScanner *discovery.Scanner
	k8sClient        *k8s.Client
	limiter  *RateLimiter
	version  string

	// bgCtx/bgCancel mengatur umur goroutine background (watchdog) hingga shutdown
	bgCtx    context.Context
	bgCancel context.CancelFunc
}

// SetVersion menetapkan nomor versi biner server yang sedang berjalan
func (s *Server) SetVersion(v string) {
	s.version = v
}

// Version mengembalikan versi biner server aktif
func (s *Server) Version() string {
	if s.version == "" {
		return "0.1.3"
	}
	return s.version
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
		cronMgr:  cron.NewManager(s),
		firewall: security.NewManager(),
		pm2:      node.NewManager(),
		phpMgr:   php.NewManager(),
		systemMgr: system.NewManager(),
		appStore:  appstore.NewManager(),
		discoveryScanner: discovery.NewScanner(),
		k8sClient:        k8s.NewClient(),
		limiter:  NewRateLimiter(),
	}
	srv.bgCtx, srv.bgCancel = context.WithCancel(context.Background())
	srv.cronMgr.Start(srv.bgCtx)
	srv.sites = sites.NewManager(sites.DefaultPaths(), srv.nginx, s)
	srv.ai = ai.NewClient(s, m, srv.nginx)

	srv.registerRoutes()

	// Mulai pencatat monitoring persisten di background
	srv.StartMonitoringRecorder(srv.bgCtx)

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

	// Penjadwal Tugas (Cron Jobs)
	s.mux.Handle("GET /api/cron/jobs", s.AuthMiddleware(http.HandlerFunc(s.handleListCronJobs)))
	s.mux.Handle("POST /api/cron/jobs", s.AuthMiddleware(http.HandlerFunc(s.handleCreateCronJob)))
	s.mux.Handle("POST /api/cron/jobs/{id}/toggle", s.AuthMiddleware(http.HandlerFunc(s.handleToggleCronJob)))
	s.mux.Handle("DELETE /api/cron/jobs/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteCronJob)))
	s.mux.Handle("POST /api/cron/jobs/{id}/run", s.AuthMiddleware(http.HandlerFunc(s.handleRunCronJobNow)))
	s.mux.Handle("GET /api/cron/jobs/{id}/logs", s.AuthMiddleware(http.HandlerFunc(s.handleListCronLogs)))

	// Backup & Arsip (S3 & Lokal)
	s.mux.Handle("GET /api/backups", s.AuthMiddleware(http.HandlerFunc(s.handleListBackups)))
	s.mux.Handle("POST /api/backups", s.AuthMiddleware(http.HandlerFunc(s.handleCreateBackupConfig)))
	s.mux.Handle("DELETE /api/backups/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteBackupConfig)))
	s.mux.Handle("POST /api/backups/{id}/run", s.AuthMiddleware(http.HandlerFunc(s.handleRunBackupNow)))
	s.mux.Handle("GET /api/backups/{id}/records", s.AuthMiddleware(http.HandlerFunc(s.handleListBackupRecords)))

	// Firewall UFW Guard
	s.mux.Handle("GET /api/firewall/status", s.AuthMiddleware(http.HandlerFunc(s.handleGetFirewallStatus)))
	s.mux.Handle("POST /api/firewall/toggle", s.AuthMiddleware(http.HandlerFunc(s.handleToggleFirewall)))
	s.mux.Handle("POST /api/firewall/rules", s.AuthMiddleware(http.HandlerFunc(s.handleAddFirewallRule)))
	s.mux.Handle("DELETE /api/firewall/rules/{number}", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteFirewallRule)))

	// Security Center: SSH Hardening, Fail2Ban & Vulnerability Scanner
	s.mux.Handle("GET /api/security/ssh", s.AuthMiddleware(http.HandlerFunc(s.handleGetSSHConfig)))
	s.mux.Handle("POST /api/security/ssh", s.AuthMiddleware(http.HandlerFunc(s.handleUpdateSSHConfig)))
	s.mux.Handle("GET /api/security/fail2ban", s.AuthMiddleware(http.HandlerFunc(s.handleGetFail2BanStatus)))
	s.mux.Handle("POST /api/security/fail2ban/unban", s.AuthMiddleware(http.HandlerFunc(s.handleUnbanFail2BanIP)))
	s.mux.Handle("POST /api/security/scan", s.AuthMiddleware(http.HandlerFunc(s.handleRunSecurityScan)))

	// Node.js & PM2 Management
	s.mux.Handle("GET /api/node/status", s.AuthMiddleware(http.HandlerFunc(s.handleGetNodeStatus)))
	s.mux.Handle("GET /api/node/processes", s.AuthMiddleware(http.HandlerFunc(s.handleListNodeProcesses)))
	s.mux.Handle("POST /api/node/processes/start", s.AuthMiddleware(http.HandlerFunc(s.handleStartNodeProcess)))
	s.mux.Handle("POST /api/node/processes/{id}/{action}", s.AuthMiddleware(http.HandlerFunc(s.handleControlNodeProcess)))
	s.mux.Handle("GET /api/node/processes/{id}/logs", s.AuthMiddleware(http.HandlerFunc(s.handleGetNodeProcessLogs)))

	// DNS Management
	s.mux.Handle("GET /api/dns/zones", s.AuthMiddleware(http.HandlerFunc(s.handleListDNSZones)))
	s.mux.Handle("POST /api/dns/zones", s.AuthMiddleware(http.HandlerFunc(s.handleCreateDNSZone)))
	s.mux.Handle("DELETE /api/dns/zones/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteDNSZone)))
	s.mux.Handle("GET /api/dns/zones/{id}/records", s.AuthMiddleware(http.HandlerFunc(s.handleListDNSRecords)))
	s.mux.Handle("POST /api/dns/records", s.AuthMiddleware(http.HandlerFunc(s.handleCreateDNSRecord)))
	s.mux.Handle("DELETE /api/dns/records/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteDNSRecord)))

	// PHP Multi-version & FPM
	s.mux.Handle("GET /api/php/versions", s.AuthMiddleware(http.HandlerFunc(s.handleListPHPVersions)))
	s.mux.Handle("POST /api/php/versions/{version}/restart", s.AuthMiddleware(http.HandlerFunc(s.handleRestartPHPFPM)))
	s.mux.Handle("POST /api/php/versions/{version}/settings", s.AuthMiddleware(http.HandlerFunc(s.handleUpdatePHPSetting)))

	// Webhooks & Alerts
	s.mux.Handle("GET /api/webhooks", s.AuthMiddleware(http.HandlerFunc(s.handleListWebhooks)))
	s.mux.Handle("POST /api/webhooks", s.AuthMiddleware(http.HandlerFunc(s.handleCreateWebhook)))
	s.mux.Handle("DELETE /api/webhooks/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDeleteWebhook)))

	// System Process & Task Manager
	s.mux.Handle("GET /api/system/processes", s.AuthMiddleware(http.HandlerFunc(s.handleListSystemProcesses)))
	s.mux.Handle("POST /api/system/processes/{pid}/kill", s.AuthMiddleware(http.HandlerFunc(s.handleKillSystemProcess)))
	s.mux.Handle("GET /api/system/ports", s.AuthMiddleware(http.HandlerFunc(s.handleListListeningPorts)))

	// System Cleaner & Optimization
	s.mux.Handle("GET /api/system/cleaner/scan", s.AuthMiddleware(http.HandlerFunc(s.handleScanSystemJunk)))
	s.mux.Handle("POST /api/system/cleaner/clean", s.AuthMiddleware(http.HandlerFunc(s.handleCleanSystemJunk)))

	// Software App Store & Toolkit Installer
	s.mux.Handle("GET /api/appstore/catalog", s.AuthMiddleware(http.HandlerFunc(s.handleListAppStore)))
	s.mux.Handle("GET /api/appstore/search", s.AuthMiddleware(http.HandlerFunc(s.handleSearchAppStore)))
	s.mux.Handle("POST /api/appstore/install/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleInstallApp)))
	s.mux.Handle("POST /api/appstore/uninstall/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleUninstallApp)))
	s.mux.Handle("GET /api/appstore/tasks/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleGetAppTask)))

	// Persistent System Monitor History & Control
	s.mux.Handle("GET /api/monitor/config", s.AuthMiddleware(http.HandlerFunc(s.handleGetMonitorConfig)))
	s.mux.Handle("POST /api/monitor/config", s.AuthMiddleware(http.HandlerFunc(s.handleUpdateMonitorConfig)))
	s.mux.Handle("GET /api/monitor/history", s.AuthMiddleware(http.HandlerFunc(s.handleGetMonitorHistory)))
	s.mux.Handle("DELETE /api/monitor/history", s.AuthMiddleware(http.HandlerFunc(s.handleClearMonitorHistory)))

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
	s.mux.Handle("GET /api/databases/webgui/status", s.AuthMiddleware(http.HandlerFunc(s.handleWebGuiStatus)))
	s.mux.Handle("POST /api/databases/webgui/sso", s.AuthMiddleware(http.HandlerFunc(s.handleWebGuiSSO)))
	s.mux.Handle("POST /api/databases/webgui/toggle-public", s.AuthMiddleware(http.HandlerFunc(s.handleWebGuiTogglePublic)))

	// Web Client Database Terintegrasi (Bebas Port 888 & Bebas Keterikatan Domain)
	s.mux.HandleFunc("/pma/", s.handleBuiltinWebGuiClient)
	s.mux.HandleFunc("/phpmyadmin/", s.handleBuiltinWebGuiClient)
	s.mux.HandleFunc("/phpMyAdmin/", s.handleBuiltinWebGuiClient)
	s.mux.HandleFunc("GET /pma/api/tables", s.handleBuiltinWebGuiTables)
	s.mux.HandleFunc("POST /pma/api/query", s.handleBuiltinWebGuiQuery)

	// Manajemen Docker & Kontainerisasi Lengkap (10 Modul aaPanel Style)
	s.mux.Handle("GET /api/docker/status", s.AuthMiddleware(http.HandlerFunc(s.handleDockerStatus)))
	s.mux.Handle("GET /api/docker/system-info", s.AuthMiddleware(http.HandlerFunc(s.handleDockerSystemInfo)))
	s.mux.Handle("GET /api/docker/config", s.AuthMiddleware(http.HandlerFunc(s.handleDockerGetConfig)))
	s.mux.Handle("POST /api/docker/config", s.AuthMiddleware(http.HandlerFunc(s.handleDockerSaveConfig)))
	s.mux.Handle("POST /api/docker/service/{action}", s.AuthMiddleware(http.HandlerFunc(s.handleDockerServiceControl)))
	s.mux.Handle("POST /api/docker/install", s.AuthMiddleware(http.HandlerFunc(s.handleDockerInstall)))

	// Kontainer
	s.mux.Handle("GET /api/docker/containers", s.AuthMiddleware(http.HandlerFunc(s.handleDockerListContainers)))
	s.mux.Handle("POST /api/docker/containers", s.AuthMiddleware(http.HandlerFunc(s.handleDockerCreateContainer)))
	s.mux.Handle("POST /api/docker/containers/{id}/{action}", s.AuthMiddleware(http.HandlerFunc(s.handleDockerControlContainer)))
	s.mux.Handle("GET /api/docker/containers/{id}/logs", s.AuthMiddleware(http.HandlerFunc(s.handleDockerContainerLogs)))

	// Citra (Images)
	s.mux.Handle("GET /api/docker/images", s.AuthMiddleware(http.HandlerFunc(s.handleDockerListImages)))
	s.mux.Handle("POST /api/docker/images/pull", s.AuthMiddleware(http.HandlerFunc(s.handleDockerPullImage)))
	s.mux.Handle("DELETE /api/docker/images/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDockerRemoveImage)))
	s.mux.Handle("GET /api/docker/images/search", s.AuthMiddleware(http.HandlerFunc(s.handleDockerSearchImages)))

	// Volume
	s.mux.Handle("GET /api/docker/volumes", s.AuthMiddleware(http.HandlerFunc(s.handleDockerListVolumes)))
	s.mux.Handle("POST /api/docker/volumes", s.AuthMiddleware(http.HandlerFunc(s.handleDockerCreateVolume)))
	s.mux.Handle("DELETE /api/docker/volumes/{name}", s.AuthMiddleware(http.HandlerFunc(s.handleDockerRemoveVolume)))
	s.mux.Handle("POST /api/docker/volumes/prune", s.AuthMiddleware(http.HandlerFunc(s.handleDockerPruneVolumes)))

	// Jaringan (Networks)
	s.mux.Handle("GET /api/docker/networks", s.AuthMiddleware(http.HandlerFunc(s.handleDockerListNetworks)))
	s.mux.Handle("POST /api/docker/networks", s.AuthMiddleware(http.HandlerFunc(s.handleDockerCreateNetwork)))
	s.mux.Handle("DELETE /api/docker/networks/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDockerRemoveNetwork)))

	// Docker Compose
	s.mux.Handle("GET /api/docker/compose", s.AuthMiddleware(http.HandlerFunc(s.handleDockerListCompose)))
	s.mux.Handle("POST /api/docker/compose/up", s.AuthMiddleware(http.HandlerFunc(s.handleDockerComposeUp)))
	s.mux.Handle("POST /api/docker/compose/down", s.AuthMiddleware(http.HandlerFunc(s.handleDockerComposeDown)))

	// One-Click Template
	s.mux.Handle("GET /api/docker/templates", s.AuthMiddleware(http.HandlerFunc(s.handleDockerGetTemplates)))
	s.mux.Handle("POST /api/docker/templates/deploy", s.AuthMiddleware(http.HandlerFunc(s.handleDockerDeployTemplate)))

	// Registri Privat
	s.mux.Handle("GET /api/docker/registries", s.AuthMiddleware(http.HandlerFunc(s.handleDockerListRegistries)))
	s.mux.Handle("POST /api/docker/registries", s.AuthMiddleware(http.HandlerFunc(s.handleDockerAddRegistry)))
	s.mux.Handle("DELETE /api/docker/registries/{id}", s.AuthMiddleware(http.HandlerFunc(s.handleDockerRemoveRegistry)))

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

	// Deteksi Otomatis Sumber Daya Host (Discovery) & Adopsi Konfigurasi
	s.mux.Handle("GET /api/discovery/config", s.AuthMiddleware(http.HandlerFunc(s.handleGetDiscoveryConfig)))
	s.mux.Handle("POST /api/discovery/config", s.AuthMiddleware(http.HandlerFunc(s.handleSaveDiscoveryConfig)))
	s.mux.Handle("POST /api/discovery/scan", s.AuthMiddleware(http.HandlerFunc(s.handleDiscoveryScan)))
	s.mux.Handle("POST /api/discovery/adopt", s.AuthMiddleware(http.HandlerFunc(s.handleDiscoveryAdopt)))

	// Orkestrasi Lengkap Kubernetes & K3s Host
	s.mux.Handle("GET /api/k8s/status", s.AuthMiddleware(http.HandlerFunc(s.handleK8sStatus)))
	s.mux.Handle("GET /api/k8s/nodes", s.AuthMiddleware(http.HandlerFunc(s.handleK8sNodes)))
	s.mux.Handle("GET /api/k8s/namespaces", s.AuthMiddleware(http.HandlerFunc(s.handleK8sNamespaces)))
	s.mux.Handle("GET /api/k8s/pods", s.AuthMiddleware(http.HandlerFunc(s.handleK8sPods)))
	s.mux.Handle("GET /api/k8s/deployments", s.AuthMiddleware(http.HandlerFunc(s.handleK8sDeployments)))
	s.mux.Handle("GET /api/k8s/services", s.AuthMiddleware(http.HandlerFunc(s.handleK8sServices)))
	s.mux.Handle("POST /api/k8s/deployments/{name}/scale", s.AuthMiddleware(http.HandlerFunc(s.handleK8sScaleDeployment)))
	s.mux.Handle("POST /api/k8s/apply", s.AuthMiddleware(http.HandlerFunc(s.handleK8sApplyManifest)))
	s.mux.Handle("GET /api/k8s/pods/{name}/logs", s.AuthMiddleware(http.HandlerFunc(s.handleK8sPodLogs)))
	s.mux.Handle("DELETE /api/k8s/resources", s.AuthMiddleware(http.HandlerFunc(s.handleK8sDeleteResource)))
	s.mux.Handle("POST /api/k8s/install-k3s", s.AuthMiddleware(http.HandlerFunc(s.handleK8sInstallK3s)))

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
