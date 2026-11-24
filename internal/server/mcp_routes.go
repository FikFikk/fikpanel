package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/ai"
	"fikpanel/internal/docker"
	"fikpanel/internal/store"
	"fikpanel/internal/waf"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpContextKey string

const (
	mcpTokenCtxKey mcpContextKey = "mcp_token"
	mcpIPCtxKey    mcpContextKey = "mcp_client_ip"
)

// MCPToolCatalogItem mendeskripsikan satu tool MCP yang ditampilkan di UI Pengaturan
type MCPToolCatalogItem struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"` // "readonly" atau "operator"
	Title       string `json:"title"`
	Description string `json:"description"`
}

// MCPPromptCatalogItem mendeskripsikan prompt runbook bawaan MCP yang tersedia di IDE
type MCPPromptCatalogItem struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func mcpToolCatalog() []MCPToolCatalogItem {
	return []MCPToolCatalogItem{
		// 18 Read-Only Diagnostic Tools
		{Name: "server_overview", Scope: "readonly", Title: "Ringkasan Server", Description: "CPU, RAM, Disk, Load Average, OS, kernel, uptime, jumlah situs & layanan aktif."},
		{Name: "list_sites", Scope: "readonly", Title: "Daftar Website", Description: "Seluruh situs web FikPanel beserta domain, runtime (PHP/Node/Go/Python/Static/Proxy), SSL, dan root path."},
		{Name: "read_site_vhost", Scope: "readonly", Title: "Baca Konfigurasi Vhost Nginx", Description: "Inspeksi isi konfigurasi Nginx virtual host suatu domain (proxy_pass, root, fastcgi_pass, SSL) untuk debug 502/404."},
		{Name: "read_site_logs", Scope: "readonly", Title: "Log Nginx Situs (Error/Access)", Description: "Baca tail log error atau access Nginx milik domain tertentu (otomatis menyensor rahasia/token)."},
		{Name: "read_site_app_log", Scope: "readonly", Title: "Log Aplikasi Situs (Laravel/App)", Description: "Baca tail log aplikasi seperti storage/logs/laravel.log atau logs/*.log di proyek situs (hemat RAM & tersensor)."},
		{Name: "nginx_status_and_logs", Scope: "readonly", Title: "Status & Error Log Nginx", Description: "Inspeksi status runtime Nginx, PID, worker, RAM, dan tail error.log global."},
		{Name: "list_services", Scope: "readonly", Title: "Daftar Layanan Systemd", Description: "Daftar daemon Linux (running/failed/all) beserta PID, penggunaan memori, dan uptime."},
		{Name: "read_service_logs", Scope: "readonly", Title: "Log Jurnal Systemd", Description: "Baca output journalctl terbaru untuk unit layanan tertentu (nginx, php-fpm, mysql, dll.)."},
		{Name: "list_pm2_processes", Scope: "readonly", Title: "Daftar Proses Node.js / PM2", Description: "Daftar proses PM2 lintas user beserta status online/errored, CPU%, RAM, restart count, dan user pemilik."},
		{Name: "read_pm2_logs", Scope: "readonly", Title: "Log Proses Node.js / PM2", Description: "Baca tail log stdout/stderr proses PM2 tertentu (tersensor otomatis dari rahasia)."},
		{Name: "list_docker_containers", Scope: "readonly", Title: "Daftar Kontainer Docker", Description: "Daftar kontainer Docker aktif & berhenti beserta image, status, dan pemetaan port."},
		{Name: "read_docker_logs", Scope: "readonly", Title: "Log Kontainer Docker", Description: "Baca tail log stdout/stderr kontainer Docker tertentu (tersensor otomatis dari rahasia)."},
		{Name: "list_databases", Scope: "readonly", Title: "Metadata Database (Tanpa Sandi)", Description: "Daftar nama database, engine, username, host akses, dan kapasitas real tanpa pernah membuka kata sandi."},
		{Name: "list_listening_ports", Scope: "readonly", Title: "Daftar Port Listening & Proses", Description: "Audit port TCP/UDP yang sedang aktif beserta alamat bind, PID, dan program pemilik (debug 502/port conflict)."},
		{Name: "list_uptime_monitors", Scope: "readonly", Title: "Uptime & Masa Aktif SSL", Description: "Status online/offline monitor HTTP/TCP, persentase uptime 24 jam, latensi, dan sisa hari SSL."},
		{Name: "get_waf_summary", Scope: "readonly", Title: "Ringkasan & Insiden WAF", Description: "Statistik proteksi WAF beserta daftar serangan terbaru (IP, negara, jenis serangan, path target)."},
		{Name: "list_cron_jobs", Scope: "readonly", Title: "Daftar Tugas Cron", Description: "Seluruh jadwal cron aktif beserta perintah dan status eksekusi terakhir."},
		{Name: "list_audit_logs", Scope: "readonly", Title: "Audit Log Keamanan", Description: "Riwayat aktivitas keamanan panel terbaru termasuk login dan pemanggilan tool MCP."},
		// 8 Operator Action Tools (Terukur & Non-Destruktif)
		{Name: "git_pull_site", Scope: "operator", Title: "Deploy Git Pull Situs", Description: "Tarik commit terbaru dari Git (aman dengan backup .env, opsi composer/migrate/cache Laravel, tanpa arbitrary shell)."},
		{Name: "restart_service", Scope: "operator", Title: "Restart / Reload Service", Description: "Me-restart atau me-reload unit systemd non-destruktif (unit vital tetap dilindungi dari stop)."},
		{Name: "restart_pm2_process", Scope: "operator", Title: "Restart / Reload Proses PM2", Description: "Me-restart, me-reload, atau menyalakan proses Node.js/PM2 tanpa akses delete/stop destruktif."},
		{Name: "restart_docker_container", Scope: "operator", Title: "Restart / Start Kontainer Docker", Description: "Menjalankan restart atau start pada kontainer Docker target secara aman tanpa menghapus kontainer/volume."},
		{Name: "test_and_reload_nginx", Scope: "operator", Title: "Uji & Reload Nginx", Description: "Menjalankan pengujian sintaks nginx -t lalu menerapkan reload konfigurasi tanpa downtime."},
		{Name: "check_uptime_now", Scope: "operator", Title: "Probe Uptime Langsung", Description: "Menjalankan pemeriksaan HTTP/TCP/SSL real-time untuk satu target monitor tertentu."},
		{Name: "check_site_dns", Scope: "operator", Title: "Cek Resolusi DNS Domain", Description: "Memeriksa apakah DNS publik domain situs sudah mengarah ke alamat IP server ini."},
		{Name: "block_waf_ip", Scope: "operator", Title: "Blokir IP Penyerang di WAF", Description: "Menambahkan IP publik penyerang ke daftar hitam WAF (otomatis menolak IP loopback/privat demi keamanan)."},
	}
}

func mcpPromptCatalog() []MCPPromptCatalogItem {
	return []MCPPromptCatalogItem{
		{Name: "diagnose_site_error", Title: "Diagnosa Error Situs (500/502/404)", Description: "Runbook investigasi lengkap: cek konfigurasi vhost, error log Nginx, log aplikasi Laravel/PM2, dan port listening."},
		{Name: "server_health_audit", Title: "Audit Kesehatan & Keamanan Harian", Description: "Ringkasan menyeluruh beban CPU/RAM/Disk, layanan gagal, Uptime/SSL, kontainer Docker, PM2, dan serangan WAF."},
		{Name: "deploy_and_verify", Title: "Deploy Git Pull & Verifikasi Pasca-Push", Description: "Tarik kode terbaru situs via git_pull_site lalu verifikasi error log dan status Uptime/HTTP secara menyeluruh."},
	}
}

func ptrBool(v bool) *bool { return &v }

// initMCPServers membangun dua instance MCP Server (Read-Only & Operator) dengan Streamable HTTP Handler
func (s *Server) initMCPServers() {
	s.mcpReadOnlySrv = s.buildMCPServer(false)
	s.mcpOperatorSrv = s.buildMCPServer(true)

	// DisableLocalhostProtection diaktifkan karena /mcp wajib Bearer Token 256-bit
	// dan sering diakses melalui reverse proxy Nginx lokal (127.0.0.1 -> domain panel).
	s.mcpHandler = mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		tok, _ := r.Context().Value(mcpTokenCtxKey).(*store.MCPToken)
		if tok == nil {
			return nil
		}
		if tok.Scope == "operator" {
			return s.mcpOperatorSrv
		}
		return s.mcpReadOnlySrv
	}, &mcp.StreamableHTTPOptions{
		SessionTimeout:             30 * time.Minute,
		DisableLocalhostProtection: true,
		MaxRequestBodyBytes:        1 << 20,
	})
}

func (s *Server) findRegisteredSiteByDomain(ctx context.Context, domain string) (*store.Site, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil, fmt.Errorf("parameter domain wajib diisi")
	}
	allSites, err := s.store.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	for i := range allSites {
		if strings.EqualFold(allSites[i].Domain, domain) {
			return &allSites[i], nil
		}
	}
	return nil, fmt.Errorf("domain %q tidak terdaftar di FikPanel; panggil list_sites untuk melihat daftar domain valid", domain)
}

// readFileTailLines membaca maks N baris terakhir dari file log besar secara hemat RAM (seek 256 KB terakhir)
func readFileTailLines(path string, maxLines int) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	size := st.Size()
	const maxReadBytes int64 = 256 * 1024
	if size > maxReadBytes {
		if _, err := f.Seek(size-maxReadBytes, io.SeekStart); err != nil {
			return "", size, err
		}
	}
	data, err := io.ReadAll(io.LimitReader(f, maxReadBytes))
	if err != nil {
		return "", size, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), size, nil
}

// locateSiteAppLog mencari berkas log aplikasi terbaru di dalam proyek situs terdaftar (Laravel/PHP/Node)
func locateSiteAppLog(projectDir, hintFile string) (string, error) {
	cleanRoot := filepath.Clean(projectDir)
	if hintFile != "" {
		// Tolak path absolut atau traversal keluar dari direktori proyek
		if filepath.IsAbs(hintFile) || strings.Contains(hintFile, "..") {
			return "", fmt.Errorf("path log relatif tidak valid")
		}
		cand := filepath.Clean(filepath.Join(cleanRoot, hintFile))
		if !strings.HasPrefix(cand, cleanRoot+string(filepath.Separator)) {
			return "", fmt.Errorf("akses berkas di luar direktori proyek ditolak")
		}
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
		return "", fmt.Errorf("berkas log %q tidak ditemukan di %s", hintFile, cleanRoot)
	}

	// Prioritas 1: Laravel default storage/logs/laravel.log
	directCandidates := []string{
		filepath.Join(cleanRoot, "storage", "logs", "laravel.log"),
		filepath.Join(cleanRoot, "logs", "error.log"),
		filepath.Join(cleanRoot, "logs", "app.log"),
		filepath.Join(cleanRoot, "error.log"),
	}
	for _, c := range directCandidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() && st.Size() > 0 {
			return c, nil
		}
	}

	// Prioritas 2: Berkas *.log terbaru di storage/logs/ (misal laravel-2026-11-24.log daily channel)
	for _, dir := range []string{filepath.Join(cleanRoot, "storage", "logs"), filepath.Join(cleanRoot, "logs")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		type logEntry struct {
			path string
			mod  time.Time
		}
		var found []logEntry
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
				continue
			}
			if info, err := e.Info(); err == nil {
				found = append(found, logEntry{path: filepath.Join(dir, e.Name()), mod: info.ModTime()})
			}
		}
		if len(found) > 0 {
			sort.Slice(found, func(i, j int) bool { return found[i].mod.After(found[j].mod) })
			return found[0].path, nil
		}
	}

	return "", fmt.Errorf("tidak ditemukan berkas log aplikasi (storage/logs/*.log atau logs/*.log) di %s", cleanRoot)
}

func (s *Server) registerMCPPrompts(srv *mcp.Server, includeOperator bool) {
	srv.AddPrompt(&mcp.Prompt{
		Name:        "diagnose_site_error",
		Title:       "Diagnosa Error Situs (500/502/404)",
		Description: "Runbook sistematis untuk mendiagnosa error HTTP 500/502/504/404 pada situs FikPanel dan mencocokkannya dengan kode lokal di IDE.",
		Arguments: []*mcp.PromptArgument{
			{Name: "domain", Title: "Domain Situs", Description: "Nama domain situs target (contoh: api.domain.com)", Required: true},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		domain := strings.TrimSpace(req.Params.Arguments["domain"])
		if domain == "" {
			domain = "<domain>"
		}
		text := fmt.Sprintf(
			"Lakukan investigasi root-cause untuk situs %q di FikPanel secara berurutan:\n"+
				"1. Panggil list_sites(search=%q) dan read_site_vhost(domain=%q) untuk memeriksa tipe runtime, root_path, upstream port, atau PHP-FPM socket.\n"+
				"2. Panggil read_site_logs(domain=%q, log_type='error', lines=100) dan read_site_app_log(domain=%q, lines=120) untuk menangkap stack trace atau error upstream.\n"+
				"3. Jika error mengarah ke 502/connection refused, periksa list_listening_ports(), list_services(scope='failed'), list_pm2_processes(), dan list_docker_containers().\n"+
				"4. Rangkum: Akar Masalah → Bukti Log → Perbaikan Kode di Workspace / Langkah Pemulihan.",
			domain, domain, domain, domain, domain,
		)
		return &mcp.GetPromptResult{
			Description: "Runbook diagnosa error situs " + domain,
			Messages: []*mcp.PromptMessage{
				{Role: mcp.Role("user"), Content: &mcp.TextContent{Text: text}},
			},
		}, nil
	})

	srv.AddPrompt(&mcp.Prompt{
		Name:        "server_health_audit",
		Title:       "Audit Kesehatan & Keamanan Server",
		Description: "Pemeriksaan menyeluruh CPU/RAM/Disk, layanan systemd gagal, Uptime/SSL, kontainer Docker, PM2, dan serangan WAF.",
	}, func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		text := "Jalankan audit kesehatan & keamanan menyeluruh server FikPanel:\n" +
			"1. Panggil server_overview(), list_uptime_monitors(), dan list_services(scope='failed').\n" +
			"2. Panggil list_docker_containers() dan list_pm2_processes() untuk mendeteksi kontainer/proses yang crash atau sering restart.\n" +
			"3. Panggil get_waf_summary(limit=20) dan list_audit_logs(limit=20) untuk memeriksa aktivitas mencurigakan.\n" +
			"4. Sajikan laporan ringkas dalam tabel: Status Kesehatan, Peringatan SSL (<14 hari) / Resource (>80%), dan Rekomendasi Aksi Prioritas."
		return &mcp.GetPromptResult{
			Description: "Runbook audit kesehatan harian server FikPanel",
			Messages: []*mcp.PromptMessage{
				{Role: mcp.Role("user"), Content: &mcp.TextContent{Text: text}},
			},
		}, nil
	})

	if includeOperator {
		srv.AddPrompt(&mcp.Prompt{
			Name:        "deploy_and_verify",
			Title:       "Deploy Git Pull & Verifikasi Pasca-Push",
			Description: "Tarik commit terbaru situs via git_pull_site lalu verifikasi error log dan status Uptime/HTTP secara otomatis.",
			Arguments: []*mcp.PromptArgument{
				{Name: "domain", Title: "Domain Situs", Description: "Nama domain situs yang akan di-deploy ulang via Git Pull", Required: true},
			},
		}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			domain := strings.TrimSpace(req.Params.Arguments["domain"])
			if domain == "" {
				domain = "<domain>"
			}
			text := fmt.Sprintf(
				"Jalankan alur deploy & verifikasi aman untuk situs %q:\n"+
					"1. Panggil list_sites(search=%q) untuk memastikan situs terdaftar.\n"+
					"2. Panggil git_pull_site(domain=%q) untuk menarik pembaruan Git terbaru.\n"+
					"3. Periksa read_site_logs(domain=%q, log_type='error', lines=40) dan read_site_app_log(domain=%q, lines=50) guna memastikan tidak ada error baru.\n"+
					"4. Panggil list_uptime_monitors() lalu check_uptime_now() bila monitor tersedia untuk memverifikasi HTTP 200 OK.",
				domain, domain, domain, domain, domain,
			)
			return &mcp.GetPromptResult{
				Description: "Runbook deploy & verifikasi situs " + domain,
				Messages: []*mcp.PromptMessage{
					{Role: mcp.Role("user"), Content: &mcp.TextContent{Text: text}},
				},
			}, nil
		})
	}
}

func (s *Server) buildMCPServer(includeOperator bool) *mcp.Server {
	modeDesc := "READ-ONLY (18 tools diagnosa: metrik, situs, vhost, log Nginx/Laravel/PM2/Docker/Systemd, database metadata, port listening, Uptime/SSL, WAF, Cron, Audit)"
	if includeOperator {
		modeDesc = "OPERATOR (18 tools diagnosa + 8 aksi aman: git_pull_site, restart service/PM2/Docker, test_and_reload_nginx, probe Uptime, cek DNS, blokir IP WAF)"
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "fikpanel",
		Version: "v" + s.Version(),
	}, &mcp.ServerOptions{
		Instructions: "FikPanel Server Control Panel MCP Server. Mode sesi ini: " + modeDesc + ". " +
			"Selalu panggil list_sites, list_services, list_pm2_processes, atau list_docker_containers terlebih dahulu sebelum menebak nama target. " +
			"Seluruh output log sudah disensor otomatis dari rahasia/kata sandi (BR-04).",
	})

	s.registerMCPPrompts(srv, includeOperator)

	// ── 18 Read-Only Diagnostic Tools ────────────────────────────────────────
	type emptyArgs struct{}

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "server_overview",
		Title:       "Ringkasan Kesehatan Server",
		Description: "Mengembalikan metrik server real-time (CPU%, RAM, Disk, Load Average), OS, kernel, uptime, versi FikPanel, serta jumlah situs & layanan aktif.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		hostname, _ := os.Hostname()
		m := s.metrics.GetLatest()
		sitesList, _ := s.store.ListSites(ctx)
		runningSites := 0
		for _, st := range sitesList {
			if st.Enabled == 1 {
				runningSites++
			}
		}
		_, svcCounts, _ := s.services.ListServicesFiltered(ctx, "running", "", "")
		uptimeMons, _ := s.store.ListUptimeMonitors(ctx, false)
		downMons := 0
		for _, um := range uptimeMons {
			if um.Enabled && um.LastStatus == "down" {
				downMons++
			}
		}

		res := map[string]any{
			"hostname":         hostname,
			"fikpanel_version": s.Version(),
			"os":               detectHostOSPretty(),
			"kernel":           detectHostKernel(),
			"uptime_sec":       detectHostUptimeSec(),
			"cpu_percent":      m.CPU,
			"load_avg_1m":      m.LoadAvg,
			"ram_used_bytes":   m.RAMUsed,
			"ram_total_bytes":  m.RAMTotal,
			"disk_used_bytes":  m.DiskUsed,
			"disk_total_bytes": m.DiskTotal,
			"net_in_bps":       m.NetIn,
			"net_out_bps":      m.NetOut,
			"sites_total":      len(sitesList),
			"sites_running":    runningSites,
			"services_running": svcCounts.Running,
			"services_total":   svcCounts.Total,
			"uptime_total":     len(uptimeMons),
			"uptime_down":      downMons,
		}
		s.recordMCPAudit(ctx, "server_overview", hostname, true, "Membaca ringkasan server")
		return mcpJSONResult(res), nil, nil
	})

	type listSitesArgs struct {
		Search string `json:"search,omitempty" jsonschema:"Kata kunci opsional untuk memfilter domain atau deskripsi situs"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_sites",
		Title:       "Daftar Website FikPanel",
		Description: "Mengembalikan seluruh website/domain yang dikelola FikPanel beserta tipe runtime (php/node/go/python/static/proxy), upstream, root path, mode SSL, versi PHP, dan status aktif.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listSitesArgs) (*mcp.CallToolResult, any, error) {
		all, err := s.store.ListSites(ctx)
		if err != nil {
			s.recordMCPAudit(ctx, "list_sites", args.Search, false, err.Error())
			return nil, nil, fmt.Errorf("gagal membaca daftar situs: %w", err)
		}
		q := strings.ToLower(strings.TrimSpace(args.Search))
		out := make([]map[string]any, 0, len(all))
		for _, st := range all {
			if q != "" && !strings.Contains(strings.ToLower(st.Domain+" "+st.Aliases+" "+st.Description), q) {
				continue
			}
			out = append(out, map[string]any{
				"id":          st.ID,
				"domain":      st.Domain,
				"aliases":     st.Aliases,
				"kind":        st.Kind,
				"upstream":    st.Upstream,
				"root_path":   st.RootPath,
				"ssl_mode":    st.SSLMode,
				"force_https": st.ForceHTTPS == 1,
				"php_version": st.PHPVersion,
				"enabled":     st.Enabled == 1,
				"description": st.Description,
			})
		}
		s.recordMCPAudit(ctx, "list_sites", args.Search, true, fmt.Sprintf("Mengembalikan %d situs", len(out)))
		return mcpJSONResult(map[string]any{"count": len(out), "sites": out}), nil, nil
	})

	type readSiteVhostArgs struct {
		Domain string `json:"domain" jsonschema:"Nama domain situs yang terdaftar di FikPanel"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_site_vhost",
		Title:       "Baca Konfigurasi Vhost Nginx Situs",
		Description: "Membaca isi konfigurasi virtual host Nginx sebuah situs (proxy_pass, root, fastcgi_pass, rewrite, SSL) untuk diagnosa 502/404/redirect loop.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args readSiteVhostArgs) (*mcp.CallToolResult, any, error) {
		site, err := s.findRegisteredSiteByDomain(ctx, args.Domain)
		if err != nil {
			s.recordMCPAudit(ctx, "read_site_vhost", args.Domain, false, err.Error())
			return nil, nil, err
		}
		cert, _ := s.store.GetCertificate(ctx, site.ID)
		content, err := s.sites.Render(site, cert)
		if err != nil {
			s.recordMCPAudit(ctx, "read_site_vhost", site.Domain, false, err.Error())
			return nil, nil, fmt.Errorf("gagal merender vhost %s: %w", site.Domain, err)
		}
		s.recordMCPAudit(ctx, "read_site_vhost", site.Domain, true, "Membaca konfigurasi vhost Nginx")
		return mcpJSONResult(map[string]any{
			"domain":      site.Domain,
			"kind":        site.Kind,
			"upstream":    site.Upstream,
			"root_path":   site.RootPath,
			"php_version": site.PHPVersion,
			"ssl_mode":    site.SSLMode,
			"vhost_conf":  ai.RedactSecrets(content),
		}), nil, nil
	})

	type readSiteLogsArgs struct {
		Domain  string `json:"domain" jsonschema:"Nama domain situs yang terdaftar di FikPanel (contoh: api.domain.com)"`
		LogType string `json:"log_type,omitempty" jsonschema:"Jenis log: 'error' (default) atau 'access'"`
		Lines   int    `json:"lines,omitempty" jsonschema:"Jumlah baris terakhir yang dibaca (default 80, maks 300)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_site_logs",
		Title:       "Baca Log Nginx Situs (Error / Access)",
		Description: "Membaca baris terakhir error.log atau access.log Nginx milik suatu domain situs untuk diagnosa 500/502/404. Data sensitif disensor otomatis.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args readSiteLogsArgs) (*mcp.CallToolResult, any, error) {
		matched, err := s.findRegisteredSiteByDomain(ctx, args.Domain)
		if err != nil {
			s.recordMCPAudit(ctx, "read_site_logs", args.Domain, false, err.Error())
			return nil, nil, err
		}
		logType := strings.ToLower(strings.TrimSpace(args.LogType))
		if logType != "access" {
			logType = "error"
		}
		lines := clampInt(args.Lines, 10, 300, 80)
		content, err := s.sites.GetLog(matched.Domain, logType, lines)
		if err != nil {
			s.recordMCPAudit(ctx, "read_site_logs", matched.Domain, false, err.Error())
			return nil, nil, fmt.Errorf("gagal membaca log %s %s: %w", logType, matched.Domain, err)
		}
		s.recordMCPAudit(ctx, "read_site_logs", matched.Domain, true, fmt.Sprintf("Membaca %d baris %s log", lines, logType))
		return mcpJSONResult(map[string]any{
			"domain":   matched.Domain,
			"log_type": logType,
			"lines":    lines,
			"content":  ai.RedactSecrets(content),
		}), nil, nil
	})

	type readSiteAppLogArgs struct {
		Domain       string `json:"domain" jsonschema:"Nama domain situs yang terdaftar di FikPanel"`
		RelativePath string `json:"relative_path,omitempty" jsonschema:"Path relatif log opsional di dalam proyek (contoh: storage/logs/laravel.log). Kosongkan untuk deteksi otomatis."`
		Lines        int    `json:"lines,omitempty" jsonschema:"Jumlah baris terakhir yang dibaca (default 100, maks 300)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_site_app_log",
		Title:       "Baca Log Aplikasi Situs (Laravel / App)",
		Description: "Membaca tail log aplikasi di dalam direktori proyek situs (otomatis mendeteksi storage/logs/laravel.log, storage/logs/*.log terbaru, atau logs/*.log). Hemat RAM dan rahasia disensor otomatis.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args readSiteAppLogArgs) (*mcp.CallToolResult, any, error) {
		site, err := s.findRegisteredSiteByDomain(ctx, args.Domain)
		if err != nil {
			s.recordMCPAudit(ctx, "read_site_app_log", args.Domain, false, err.Error())
			return nil, nil, err
		}
		projectDir := resolveSiteProjectDir(site)
		logPath, err := locateSiteAppLog(projectDir, strings.TrimSpace(args.RelativePath))
		if err != nil {
			s.recordMCPAudit(ctx, "read_site_app_log", site.Domain, false, err.Error())
			return nil, nil, err
		}
		lines := clampInt(args.Lines, 10, 300, 100)
		content, fileSize, err := readFileTailLines(logPath, lines)
		if err != nil {
			s.recordMCPAudit(ctx, "read_site_app_log", site.Domain, false, err.Error())
			return nil, nil, fmt.Errorf("gagal membaca berkas log aplikasi: %w", err)
		}
		s.recordMCPAudit(ctx, "read_site_app_log", site.Domain, true, fmt.Sprintf("Membaca %d baris %s", lines, filepath.Base(logPath)))
		return mcpJSONResult(map[string]any{
			"domain":     site.Domain,
			"file_path":  logPath,
			"size_bytes": fileSize,
			"lines":      lines,
			"content":    ai.RedactSecrets(content),
		}), nil, nil
	})

	type nginxLogsArgs struct {
		Lines int `json:"lines,omitempty" jsonschema:"Jumlah baris terakhir error.log global Nginx (default 60, maks 250)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "nginx_status_and_logs",
		Title:       "Status & Error Log Global Nginx",
		Description: "Memeriksa status runtime daemon Nginx (versi, PID, workers, RAM) sekaligus membaca tail error.log global Nginx (tersensor).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args nginxLogsArgs) (*mcp.CallToolResult, any, error) {
		st, err := s.nginx.Status(ctx)
		if err != nil {
			s.recordMCPAudit(ctx, "nginx_status_and_logs", "nginx", false, err.Error())
			return nil, nil, fmt.Errorf("gagal membaca status Nginx: %w", err)
		}
		lines := clampInt(args.Lines, 10, 250, 60)
		errLog, _ := s.nginx.TailErrorLog(lines)
		errLog.Content = ai.RedactSecrets(errLog.Content)
		s.recordMCPAudit(ctx, "nginx_status_and_logs", "nginx", true, fmt.Sprintf("Status Nginx + %d baris error log", lines))
		return mcpJSONResult(map[string]any{
			"status":    st,
			"error_log": errLog,
		}), nil, nil
	})

	type listServicesArgs struct {
		Scope    string `json:"scope,omitempty" jsonschema:"Filter status: 'running' (default), 'failed', atau 'all'"`
		Category string `json:"category,omitempty" jsonschema:"Filter kategori opsional: web, database, security, supervisor, system"`
		Search   string `json:"search,omitempty" jsonschema:"Kata kunci filter nama unit layanan"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_services",
		Title:       "Daftar Layanan Systemd",
		Description: "Menampilkan daftar layanan systemd host beserta status aktif, PID, penggunaan memori, uptime, dan status proteksi.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listServicesArgs) (*mcp.CallToolResult, any, error) {
		scope := strings.TrimSpace(args.Scope)
		if scope == "" {
			scope = "running"
		}
		list, counts, err := s.services.ListServicesFiltered(ctx, scope, strings.TrimSpace(args.Category), strings.TrimSpace(args.Search))
		if err != nil {
			s.recordMCPAudit(ctx, "list_services", scope, false, err.Error())
			return nil, nil, fmt.Errorf("gagal memuat daftar layanan: %w", err)
		}
		s.recordMCPAudit(ctx, "list_services", scope, true, fmt.Sprintf("Mengembalikan %d layanan", len(list)))
		return mcpJSONResult(map[string]any{
			"counts":   counts,
			"services": list,
		}), nil, nil
	})

	type readServiceLogsArgs struct {
		Unit  string `json:"unit" jsonschema:"Nama unit systemd (contoh: nginx.service, php8.3-fpm.service, mysql.service)"`
		Lines int    `json:"lines,omitempty" jsonschema:"Jumlah baris journalctl (default 80, maks 300)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_service_logs",
		Title:       "Baca Log Journalctl Layanan",
		Description: "Membaca log terbaru journalctl dari unit systemd tertentu untuk diagnosa daemon crash atau error konfigurasi. Rahasia disensor otomatis.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args readServiceLogsArgs) (*mcp.CallToolResult, any, error) {
		unit := strings.TrimSpace(args.Unit)
		if unit == "" {
			return nil, nil, fmt.Errorf("parameter unit wajib diisi")
		}
		lines := clampInt(args.Lines, 10, 300, 80)
		logs, err := s.services.GetServiceLogs(ctx, unit, lines)
		if err != nil && logs == "" {
			s.recordMCPAudit(ctx, "read_service_logs", unit, false, err.Error())
			return nil, nil, fmt.Errorf("gagal membaca log unit %s: %w", unit, err)
		}
		s.recordMCPAudit(ctx, "read_service_logs", unit, true, fmt.Sprintf("Membaca %d baris journalctl", lines))
		return mcpJSONResult(map[string]any{
			"unit":  unit,
			"lines": lines,
			"logs":  ai.RedactSecrets(logs),
		}), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_pm2_processes",
		Title:       "Daftar Proses Node.js / PM2",
		Description: "Mengembalikan daftar proses Node.js / PM2 lintas user pada server beserta status (online/errored/stopped), PID, CPU%, RAM, jumlah restart, dan user pemilik.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		list, err := s.pm2.ListProcesses(ctx)
		if err != nil {
			s.recordMCPAudit(ctx, "list_pm2_processes", "pm2", false, err.Error())
			return nil, nil, fmt.Errorf("gagal membaca proses PM2: %w", err)
		}
		s.recordMCPAudit(ctx, "list_pm2_processes", "pm2", true, fmt.Sprintf("Mengembalikan %d proses PM2", len(list)))
		return mcpJSONResult(map[string]any{"count": len(list), "processes": list}), nil, nil
	})

	type readPM2LogsArgs struct {
		Process string `json:"process" jsonschema:"Nama atau ID proses PM2 (diperoleh dari list_pm2_processes)"`
		User    string `json:"user,omitempty" jsonschema:"Opsional nama user Linux pemilik proses PM2 (misal root atau ubuntu)"`
		Lines   int    `json:"lines,omitempty" jsonschema:"Jumlah baris log terakhir (default 80, maks 300)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_pm2_logs",
		Title:       "Baca Log Proses Node.js / PM2",
		Description: "Membaca tail log stdout dan stderr milik proses PM2 tertentu. Rahasia/token disensor otomatis.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args readPM2LogsArgs) (*mcp.CallToolResult, any, error) {
		proc := strings.TrimSpace(args.Process)
		if proc == "" {
			return nil, nil, fmt.Errorf("parameter process wajib diisi")
		}
		lines := clampInt(args.Lines, 10, 300, 80)
		logs, err := s.pm2.GetLogs(ctx, proc, lines, strings.TrimSpace(args.User))
		if err != nil {
			s.recordMCPAudit(ctx, "read_pm2_logs", proc, false, err.Error())
			return nil, nil, fmt.Errorf("gagal membaca log PM2 %s: %w", proc, err)
		}
		s.recordMCPAudit(ctx, "read_pm2_logs", proc, true, fmt.Sprintf("Membaca %d baris log PM2", lines))
		return mcpJSONResult(map[string]any{
			"process": proc,
			"lines":   lines,
			"logs":    ai.RedactSecrets(logs),
		}), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_docker_containers",
		Title:       "Daftar Kontainer Docker",
		Description: "Mengembalikan seluruh kontainer Docker pada host beserta ID, nama, image, status runtime, dan pemetaan port.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		mgr := docker.GetManager()
		containers, err := mgr.ListContainers(ctx)
		if err != nil {
			s.recordMCPAudit(ctx, "list_docker_containers", "docker", false, err.Error())
			return nil, nil, fmt.Errorf("gagal membaca kontainer Docker: %w", err)
		}
		s.recordMCPAudit(ctx, "list_docker_containers", "docker", true, fmt.Sprintf("Mengembalikan %d kontainer", len(containers)))
		return mcpJSONResult(map[string]any{"count": len(containers), "containers": containers}), nil, nil
	})

	type dockerLogsArgs struct {
		Container string `json:"container" jsonschema:"ID atau nama kontainer Docker (diperoleh dari list_docker_containers)"`
		Lines     int    `json:"lines,omitempty" jsonschema:"Jumlah baris log terakhir yang dibaca (default 100, maks 300)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_docker_logs",
		Title:       "Baca Log Kontainer Docker",
		Description: "Membaca tail log stdout/stderr milik sebuah kontainer Docker untuk diagnosa error aplikasi. Rahasia/token disensor otomatis.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args dockerLogsArgs) (*mcp.CallToolResult, any, error) {
		cid := strings.TrimSpace(args.Container)
		if cid == "" {
			return nil, nil, fmt.Errorf("parameter container wajib diisi")
		}
		lines := clampInt(args.Lines, 10, 300, 100)
		mgr := docker.GetManager()
		logs, err := mgr.GetContainerLogs(ctx, cid, lines)
		if err != nil {
			s.recordMCPAudit(ctx, "read_docker_logs", cid, false, err.Error())
			return nil, nil, fmt.Errorf("gagal membaca log kontainer %s: %w", cid, err)
		}
		s.recordMCPAudit(ctx, "read_docker_logs", cid, true, fmt.Sprintf("Membaca %d baris log kontainer", lines))
		return mcpJSONResult(map[string]any{
			"container": cid,
			"lines":     lines,
			"logs":      ai.RedactSecrets(logs),
		}), nil, nil
	})

	type listDatabasesArgs struct {
		Engine string `json:"engine,omitempty" jsonschema:"Filter opsional engine database: mysql, postgresql, mongodb, redis (kosongkan untuk semua)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_databases",
		Title:       "Daftar Metadata Database (Tanpa Password)",
		Description: "Mengembalikan metadata database yang dikelola FikPanel (engine, nama, username, host akses, ukuran byte, catatan). Kata sandi tidak pernah dikembalikan.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listDatabasesArgs) (*mcp.CallToolResult, any, error) {
		eng := strings.ToLower(strings.TrimSpace(args.Engine))
		engines := []string{"mysql", "postgresql", "mongodb", "redis"}
		if eng != "" {
			engines = []string{eng}
		}
		out := make([]map[string]any, 0)
		for _, e := range engines {
			recs, err := s.store.ListDatabases(ctx, e)
			if err != nil {
				continue
			}
			for _, db := range recs {
				out = append(out, map[string]any{
					"id":          db.ID,
					"engine":      db.Engine,
					"name":        db.Name,
					"username":    db.Username,
					"location":    db.Location,
					"size_bytes":  db.SizeBytes,
					"note":        db.Note,
					"created_at":  db.CreatedAt,
				})
			}
		}
		s.recordMCPAudit(ctx, "list_databases", eng, true, fmt.Sprintf("Mengembalikan %d metadata database", len(out)))
		return mcpJSONResult(map[string]any{"count": len(out), "databases": out}), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_listening_ports",
		Title:       "Daftar Port Listening & Proses Pemilik",
		Description: "Mengembalikan daftar seluruh port TCP/UDP yang sedang mendengarkan pada server beserta alamat bind, PID, dan program pemilik (sangat berguna untuk debug 502/port conflict).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		ports, err := s.systemMgr.ListListeningPorts(ctx)
		if err != nil {
			s.recordMCPAudit(ctx, "list_listening_ports", "ports", false, err.Error())
			return nil, nil, fmt.Errorf("gagal memindai port listening: %w", err)
		}
		s.recordMCPAudit(ctx, "list_listening_ports", "ports", true, fmt.Sprintf("Mengembalikan %d port aktif", len(ports)))
		return mcpJSONResult(map[string]any{"count": len(ports), "ports": ports}), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_uptime_monitors",
		Title:       "Daftar Monitor Uptime & SSL",
		Description: "Mengembalikan seluruh target Uptime Monitor (HTTP/HTTPS/TCP) beserta status terakhir, uptime 24 jam %, latensi rata-rata, sisa masa berlaku sertifikat SSL, dan pesan error terakhir.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		list, err := s.store.ListUptimeMonitors(ctx, false)
		if err != nil {
			s.recordMCPAudit(ctx, "list_uptime_monitors", "uptime", false, err.Error())
			return nil, nil, err
		}
		s.recordMCPAudit(ctx, "list_uptime_monitors", "uptime", true, fmt.Sprintf("Mengembalikan %d monitor uptime", len(list)))
		return mcpJSONResult(map[string]any{
			"count":    len(list),
			"monitors": list,
		}), nil, nil
	})

	type wafSummaryArgs struct {
		Limit int `json:"limit,omitempty" jsonschema:"Jumlah maksimal insiden WAF terbaru yang diambil (default 20, maks 50)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_waf_summary",
		Title:       "Ringkasan & Insiden WAF",
		Description: "Mengembalikan statistik overview Web Application Firewall (WAF) dan daftar percobaan serangan terbaru (SQLi, XSS, path traversal, bad bot, IP penyerang).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args wafSummaryArgs) (*mcp.CallToolResult, any, error) {
		if s.waf == nil {
			return nil, nil, fmt.Errorf("WAF engine belum diinisialisasi")
		}
		limit := clampInt(args.Limit, 5, 50, 20)
		ov, err := s.waf.GetOverview(ctx)
		if err != nil {
			s.recordMCPAudit(ctx, "get_waf_summary", "waf", false, err.Error())
			return nil, nil, err
		}
		events, _ := s.waf.ListEvents(ctx, limit)
		s.recordMCPAudit(ctx, "get_waf_summary", "waf", true, fmt.Sprintf("Ringkasan WAF + %d insiden", len(events)))
		return mcpJSONResult(map[string]any{
			"overview":      ov,
			"recent_events": events,
		}), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_cron_jobs",
		Title:       "Daftar Tugas Cron",
		Description: "Menampilkan seluruh tugas terjadwal (Cron Jobs) beserta jadwal crontab, perintah, status aktif, dan status eksekusi terakhir.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		jobs, err := s.store.ListCronJobs(ctx)
		if err != nil {
			s.recordMCPAudit(ctx, "list_cron_jobs", "cron", false, err.Error())
			return nil, nil, err
		}
		s.recordMCPAudit(ctx, "list_cron_jobs", "cron", true, fmt.Sprintf("Mengembalikan %d cron job", len(jobs)))
		return mcpJSONResult(map[string]any{"count": len(jobs), "jobs": jobs}), nil, nil
	})

	type auditLogsArgs struct {
		Limit int `json:"limit,omitempty" jsonschema:"Jumlah baris audit log terbaru (default 30, maks 100)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_audit_logs",
		Title:       "Audit Log Keamanan Panel",
		Description: "Membaca riwayat audit log keamanan FikPanel terbaru (login, perubahan pengaturan, reload layanan, dan pemanggilan tool MCP).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args auditLogsArgs) (*mcp.CallToolResult, any, error) {
		limit := clampInt(args.Limit, 5, 100, 30)
		logs, err := s.store.ListAuditLogs(ctx, limit)
		if err != nil {
			s.recordMCPAudit(ctx, "list_audit_logs", "audit", false, err.Error())
			return nil, nil, err
		}
		s.recordMCPAudit(ctx, "list_audit_logs", "audit", true, fmt.Sprintf("Mengembalikan %d catatan audit", len(logs)))
		return mcpJSONResult(map[string]any{"count": len(logs), "logs": logs}), nil, nil
	})

	if !includeOperator {
		return srv
	}

	// ── 8 Operator Action Tools (Khusus Token Scope "operator") ──────────────
	type gitPullSiteArgs struct {
		Domain      string `json:"domain" jsonschema:"Nama domain situs berbasis Git yang terdaftar di FikPanel"`
		RunComposer bool   `json:"run_composer,omitempty" jsonschema:"Jalankan composer install / npm build pasca pull (default false agar cepat)"`
		RunMigrate  bool   `json:"run_migrate,omitempty" jsonschema:"Jalankan php artisan migrate --force bila proyek Laravel (default false)"`
		ClearCache  bool   `json:"clear_cache,omitempty" jsonschema:"Jalankan php artisan optimize:clear dan reload PHP-FPM bila proyek Laravel/PHP (default true)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "git_pull_site",
		Title:       "Deploy Git Pull Situs",
		Description: "Menarik commit terbaru dari remote Git untuk situs terdaftar (otomatis melindungi .env, mendukung opsi terukur composer/migrate/clear_cache, tanpa arbitrary shell).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptrBool(false), IdempotentHint: false},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args gitPullSiteArgs) (*mcp.CallToolResult, any, error) {
		if err := s.requireMCPOperatorScope(ctx); err != nil {
			return nil, nil, err
		}
		site, err := s.findRegisteredSiteByDomain(ctx, args.Domain)
		if err != nil {
			s.recordMCPAudit(ctx, "git_pull_site", args.Domain, false, err.Error())
			return nil, nil, err
		}
		projectDir := resolveSiteProjectDir(site)
		hasGit, branch, repoSlug, oldCommit := inspectProjectGitMeta(projectDir)
		if !hasGit {
			s.recordMCPAudit(ctx, "git_pull_site", site.Domain, false, "Repositori .git tidak ditemukan")
			return nil, nil, fmt.Errorf("situs %s tidak memiliki repositori .git di %s", site.Domain, projectDir)
		}
		if branch == "" || branch == "HEAD" {
			branch = "main"
		}

		pullCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()

		s.syncGitGlobalCredentials(pullCtx, projectDir)
		token, _ := s.store.GetSetting(pullCtx, "github_token")
		origRemoteBytes, _ := exec.CommandContext(pullCtx, "git", "-C", projectDir, "remote", "get-url", "origin").Output()
		cleanRemoteURL := strings.TrimSpace(string(origRemoteBytes))
		if strings.Contains(cleanRemoteURL, "@github.com/") {
			if u, uErr := url.Parse(cleanRemoteURL); uErr == nil {
				u.User = nil
				cleanRemoteURL = u.String()
			}
		}
		if token != "" && strings.HasPrefix(cleanRemoteURL, "https://github.com/") {
			authURL := strings.Replace(cleanRemoteURL, "https://github.com/", "https://x-access-token:"+token+"@github.com/", 1)
			_ = exec.CommandContext(pullCtx, "git", "-C", projectDir, "remote", "set-url", "origin", authURL).Run()
			defer func() {
				_ = exec.CommandContext(context.Background(), "git", "-C", projectDir, "remote", "set-url", "origin", cleanRemoteURL).Run()
			}()
		}

		envPath := filepath.Join(projectDir, ".env")
		envBackup, envErr := os.ReadFile(envPath)

		execEnv := append(os.Environ(),
			"GIT_TERMINAL_PROMPT=0",
			"GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=accept-new",
			"COMPOSER_ALLOW_SUPERUSER=1",
			"HOME=/root",
		)
		_ = exec.CommandContext(pullCtx, "git", "-C", projectDir, "config", "core.fileMode", "false").Run()

		var logBuf strings.Builder
		appendStep := func(title, out string) {
			clean := strings.TrimSpace(out)
			if token != "" {
				clean = strings.ReplaceAll(clean, token, "***")
			}
			logBuf.WriteString("==> " + title + "\n" + ai.RedactSecrets(clean) + "\n\n")
		}

		pullCmd := exec.CommandContext(pullCtx, "git", "-C", projectDir, "-c", "core.fileMode=false", "pull", "origin", branch)
		pullCmd.Env = execEnv
		pullOut, pullErr := pullCmd.CombinedOutput()
		if envErr == nil && len(envBackup) > 0 {
			_ = os.WriteFile(envPath, envBackup, 0o644)
		}
		if pullErr != nil {
			cleanErr := ai.RedactSecrets(strings.TrimSpace(string(pullOut)))
			if token != "" {
				cleanErr = strings.ReplaceAll(cleanErr, token, "***")
			}
			s.recordMCPAudit(ctx, "git_pull_site", site.Domain, false, cleanErr)
			return nil, nil, fmt.Errorf("git pull gagal: %s", cleanErr)
		}
		appendStep("git pull origin "+branch, string(pullOut))

		if runtime.GOOS != "windows" {
			_, hasComposer := os.Stat(filepath.Join(projectDir, "composer.json"))
			_, hasArtisan := os.Stat(filepath.Join(projectDir, "artisan"))
			if hasComposer == nil && args.RunComposer {
				cCmd := exec.CommandContext(pullCtx, "composer", "install", "--no-interaction", "--prefer-dist", "--optimize-autoloader")
				cCmd.Dir = projectDir
				cCmd.Env = execEnv
				cOut, _ := cCmd.CombinedOutput()
				appendStep("composer install", string(cOut))
			}
			if hasArtisan == nil {
				if args.RunMigrate {
					mCmd := exec.CommandContext(pullCtx, "php", "artisan", "migrate", "--force")
					mCmd.Dir = projectDir
					mCmd.Env = execEnv
					mOut, _ := mCmd.CombinedOutput()
					appendStep("php artisan migrate --force", string(mOut))
				}
				sanitizeFilamentShieldPolicies(projectDir)
				if args.ClearCache {
					oCmd := exec.CommandContext(pullCtx, "php", "artisan", "optimize:clear")
					oCmd.Dir = projectDir
					oCmd.Env = execEnv
					oOut, _ := oCmd.CombinedOutput()
					appendStep("php artisan optimize:clear", string(oOut))
				}
			}
			_ = exec.CommandContext(pullCtx, "chown", "-R", "www-data:www-data", projectDir).Run()
			for _, sub := range []string{"storage", "bootstrap/cache"} {
				subPath := filepath.Join(projectDir, sub)
				if _, statErr := os.Stat(subPath); statErr == nil {
					_ = exec.CommandContext(pullCtx, "chmod", "-R", "775", subPath).Run()
				}
			}
			if site.Kind == "php" && site.PHPVersion != "" {
				_ = exec.CommandContext(pullCtx, "systemctl", "reload", fmt.Sprintf("php%s-fpm", site.PHPVersion)).Run()
			}
		}

		_, branchNow, repoNow, newCommit := inspectProjectGitMeta(projectDir)
		commitMsgBytes, _ := exec.CommandContext(pullCtx, "git", "-C", projectDir, "log", "-1", "--pretty=%s").Output()
		s.recordMCPAudit(ctx, "git_pull_site", site.Domain, true, fmt.Sprintf("%s -> %s (%s)", oldCommit, newCommit, branchNow))
		return mcpJSONResult(map[string]any{
			"ok":             true,
			"domain":         site.Domain,
			"repo":           repoNow,
			"branch":         branchNow,
			"old_commit":     oldCommit,
			"new_commit":     newCommit,
			"commit_message": strings.TrimSpace(string(commitMsgBytes)),
			"repo_slug":      repoSlug,
			"output":         strings.TrimSpace(logBuf.String()),
		}), nil, nil
	})

	type restartServiceArgs struct {
		Unit   string `json:"unit" jsonschema:"Nama unit systemd target (contoh: nginx.service, php8.3-fpm.service)"`
		Action string `json:"action,omitempty" jsonschema:"Aksi aman yang diizinkan: 'restart' (default) atau 'reload'"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "restart_service",
		Title:       "Restart / Reload Layanan Systemd",
		Description: "Menjalankan restart atau reload pada unit systemd tertentu (membutuhkan token scope operator; stop/disable sengaja tidak dibuka via MCP).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptrBool(false), IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args restartServiceArgs) (*mcp.CallToolResult, any, error) {
		if err := s.requireMCPOperatorScope(ctx); err != nil {
			return nil, nil, err
		}
		unit := strings.TrimSpace(args.Unit)
		if unit == "" {
			return nil, nil, fmt.Errorf("parameter unit wajib diisi")
		}
		action := strings.ToLower(strings.TrimSpace(args.Action))
		if action != "reload" {
			action = "restart"
		}
		if err := s.services.ExecuteAction(ctx, unit, action); err != nil {
			s.recordMCPAudit(ctx, "restart_service", unit, false, err.Error())
			return nil, nil, fmt.Errorf("gagal %s unit %s: %w", action, unit, err)
		}
		s.recordMCPAudit(ctx, "restart_service", unit, true, "Aksi "+action+" berhasil via MCP")
		return mcpJSONResult(map[string]any{
			"ok":      true,
			"unit":    unit,
			"action":  action,
			"message": fmt.Sprintf("Unit %s berhasil di-%s", unit, action),
		}), nil, nil
	})

	type restartPM2Args struct {
		Process string `json:"process" jsonschema:"Nama atau ID proses PM2 target (diperoleh dari list_pm2_processes)"`
		Action  string `json:"action,omitempty" jsonschema:"Aksi aman yang diizinkan: 'restart' (default), 'reload', atau 'start'"`
		User    string `json:"user,omitempty" jsonschema:"Opsional nama user Linux pemilik proses PM2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "restart_pm2_process",
		Title:       "Restart / Reload Proses Node.js / PM2",
		Description: "Menjalankan restart, reload, atau start pada proses PM2 tertentu (membutuhkan token scope operator; stop/delete sengaja tidak dibuka via MCP).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptrBool(false), IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args restartPM2Args) (*mcp.CallToolResult, any, error) {
		if err := s.requireMCPOperatorScope(ctx); err != nil {
			return nil, nil, err
		}
		proc := strings.TrimSpace(args.Process)
		if proc == "" {
			return nil, nil, fmt.Errorf("parameter process wajib diisi")
		}
		action := strings.ToLower(strings.TrimSpace(args.Action))
		if action != "reload" && action != "start" {
			action = "restart"
		}
		if err := s.pm2.Action(ctx, proc, action, strings.TrimSpace(args.User)); err != nil {
			s.recordMCPAudit(ctx, "restart_pm2_process", proc, false, err.Error())
			return nil, nil, fmt.Errorf("gagal %s proses PM2 %s: %w", action, proc, err)
		}
		s.recordMCPAudit(ctx, "restart_pm2_process", proc, true, "PM2 "+action+" berhasil via MCP")
		return mcpJSONResult(map[string]any{
			"ok":      true,
			"process": proc,
			"action":  action,
			"message": fmt.Sprintf("Proses PM2 %s berhasil di-%s", proc, action),
		}), nil, nil
	})

	type restartDockerArgs struct {
		Container string `json:"container" jsonschema:"ID atau nama kontainer Docker target"`
		Action    string `json:"action,omitempty" jsonschema:"Aksi aman yang diizinkan: 'restart' (default) atau 'start'"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "restart_docker_container",
		Title:       "Restart / Start Kontainer Docker",
		Description: "Menjalankan restart atau start pada kontainer Docker tertentu (membutuhkan token scope operator; remove/stop sengaja tidak dibuka via MCP).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptrBool(false), IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args restartDockerArgs) (*mcp.CallToolResult, any, error) {
		if err := s.requireMCPOperatorScope(ctx); err != nil {
			return nil, nil, err
		}
		cid := strings.TrimSpace(args.Container)
		if cid == "" {
			return nil, nil, fmt.Errorf("parameter container wajib diisi")
		}
		action := strings.ToLower(strings.TrimSpace(args.Action))
		if action != "start" {
			action = "restart"
		}
		mgr := docker.GetManager()
		if err := mgr.ControlContainer(ctx, cid, action); err != nil {
			s.recordMCPAudit(ctx, "restart_docker_container", cid, false, err.Error())
			return nil, nil, fmt.Errorf("gagal %s kontainer %s: %w", action, cid, err)
		}
		s.recordMCPAudit(ctx, "restart_docker_container", cid, true, "Kontainer di-"+action+" via MCP")
		return mcpJSONResult(map[string]any{
			"ok":        true,
			"container": cid,
			"action":    action,
			"message":   fmt.Sprintf("Kontainer %s berhasil di-%s", cid, action),
		}), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "test_and_reload_nginx",
		Title:       "Uji Sintaks & Reload Nginx",
		Description: "Menjalankan nginx -t untuk validasi konfigurasi lalu me-reload Nginx secara aman tanpa downtime.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptrBool(false), IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
		if err := s.requireMCPOperatorScope(ctx); err != nil {
			return nil, nil, err
		}
		if err := s.nginx.ApplyChange(ctx); err != nil {
			s.recordMCPAudit(ctx, "test_and_reload_nginx", "nginx", false, err.Error())
			return nil, nil, fmt.Errorf("validasi/reload Nginx gagal: %w", err)
		}
		s.recordMCPAudit(ctx, "test_and_reload_nginx", "nginx", true, "nginx -t lolos dan reload sukses")
		return mcpJSONResult(map[string]any{
			"ok":      true,
			"message": "Konfigurasi Nginx valid (nginx -t lolos) dan layanan berhasil di-reload",
		}), nil, nil
	})

	type checkUptimeNowArgs struct {
		MonitorID int64 `json:"monitor_id" jsonschema:"ID monitor uptime dari hasil list_uptime_monitors"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "check_uptime_now",
		Title:       "Jalankan Probe Uptime Sekarang",
		Description: "Menjalankan pemeriksaan HTTP/HTTPS/TCP & sertifikat SSL secara langsung untuk satu monitor_id dan menyimpan hasilnya.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptrBool(false), IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args checkUptimeNowArgs) (*mcp.CallToolResult, any, error) {
		if err := s.requireMCPOperatorScope(ctx); err != nil {
			return nil, nil, err
		}
		if args.MonitorID <= 0 {
			return nil, nil, fmt.Errorf("monitor_id harus bilangan positif")
		}
		mon, err := s.store.GetUptimeMonitor(ctx, args.MonitorID)
		if err != nil {
			s.recordMCPAudit(ctx, "check_uptime_now", strconv.FormatInt(args.MonitorID, 10), false, "Monitor tidak ditemukan")
			return nil, nil, fmt.Errorf("monitor ID %d tidak ditemukan", args.MonitorID)
		}
		out := s.executeAndRecordProbe(ctx, mon)
		s.recordMCPAudit(ctx, "check_uptime_now", mon.Name, out.Status == "up", fmt.Sprintf("Status=%s Latency=%dms SSL=%dd", out.Status, out.LatencyMs, out.SSLDays))
		return mcpJSONResult(map[string]any{
			"ok":         true,
			"monitor_id": mon.ID,
			"name":       mon.Name,
			"target":     mon.Target,
			"outcome":    out,
		}), nil, nil
	})

	type checkDNSArgs struct {
		Domains []string `json:"domains" jsonschema:"Daftar domain yang ingin diperiksa resolusi DNS-nya terhadap IP server ini"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "check_site_dns",
		Title:       "Cek Resolusi DNS Domain",
		Description: "Memeriksa apakah rekaman DNS publik satu atau lebih domain sudah mengarah ke alamat IP publik server ini.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args checkDNSArgs) (*mcp.CallToolResult, any, error) {
		if err := s.requireMCPOperatorScope(ctx); err != nil {
			return nil, nil, err
		}
		if len(args.Domains) == 0 || len(args.Domains) > 10 {
			return nil, nil, fmt.Errorf("berikan 1 sampai 10 nama domain")
		}
		results, serverIPs := s.sites.CheckDNS(ctx, args.Domains)
		s.recordMCPAudit(ctx, "check_site_dns", strings.Join(args.Domains, ","), true, "Cek DNS domain via MCP")
		return mcpJSONResult(map[string]any{
			"server_ips": serverIPs,
			"results":    results,
		}), nil, nil
	})

	type blockWAFIPArgs struct {
		IP      string `json:"ip" jsonschema:"Alamat IPv4/IPv6 publik penyerang yang akan diblokir di WAF blacklist"`
		Remarks string `json:"remarks,omitempty" jsonschema:"Alasan pemblokiran singkat (contoh: SQLi scanner dari audit MCP)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "block_waf_ip",
		Title:       "Blokir IP Penyerang di WAF",
		Description: "Menambahkan alamat IP publik penyerang ke WAF ip_blacklist (otomatis menolak loopback/IP privat agar server & admin tidak terkunci).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptrBool(false), IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args blockWAFIPArgs) (*mcp.CallToolResult, any, error) {
		if err := s.requireMCPOperatorScope(ctx); err != nil {
			return nil, nil, err
		}
		if s.waf == nil {
			return nil, nil, fmt.Errorf("WAF engine belum diinisialisasi")
		}
		ipStr := strings.TrimSpace(args.IP)
		parsed := net.ParseIP(ipStr)
		if parsed == nil {
			return nil, nil, fmt.Errorf("alamat IP %q tidak valid", ipStr)
		}
		if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast() || parsed.IsUnspecified() {
			s.recordMCPAudit(ctx, "block_waf_ip", ipStr, false, "Penolakan blokir IP loopback/privat")
			return nil, nil, fmt.Errorf("ditolak: IP loopback atau privat (%s) tidak boleh diblokir via MCP", ipStr)
		}
		ipVer := "IPv4"
		if parsed.To4() == nil {
			ipVer = "IPv6"
		}
		remarks := strings.TrimSpace(args.Remarks)
		if remarks == "" {
			remarks = "Diblokir via MCP Operator"
		}
		ruleID, err := s.waf.AddRule(ctx, waf.WAFRule{
			RuleType:  "ip_blacklist",
			IPVersion: ipVer,
			StartIP:   parsed.String(),
			EndIP:     parsed.String(),
			Remarks:   remarks,
		})
		if err != nil {
			s.recordMCPAudit(ctx, "block_waf_ip", parsed.String(), false, err.Error())
			return nil, nil, fmt.Errorf("gagal menambahkan IP ke WAF blacklist: %w", err)
		}
		s.recordMCPAudit(ctx, "block_waf_ip", parsed.String(), true, remarks)
		return mcpJSONResult(map[string]any{
			"ok":      true,
			"rule_id": ruleID,
			"ip":      parsed.String(),
			"remarks": remarks,
			"message": fmt.Sprintf("IP %s berhasil ditambahkan ke WAF blacklist", parsed.String()),
		}), nil, nil
	})

	return srv
}

func mcpJSONResult(v any) *mcp.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		b = []byte(`{"error":"gagal serialisasi hasil"}`)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(b)},
		},
	}
}

func (s *Server) requireMCPOperatorScope(ctx context.Context) error {
	tok, _ := ctx.Value(mcpTokenCtxKey).(*store.MCPToken)
	if tok == nil || tok.Scope != "operator" {
		return fmt.Errorf("akses ditolak: tool ini memerlukan token MCP dengan scope 'operator'")
	}
	return nil
}

func (s *Server) recordMCPAudit(ctx context.Context, toolName, target string, ok bool, detail string) {
	tok, _ := ctx.Value(mcpTokenCtxKey).(*store.MCPToken)
	ip, _ := ctx.Value(mcpIPCtxKey).(string)
	if tok != nil {
		_ = s.store.TouchMCPToken(ctx, tok.ID, ip)
		uid := tok.UserID
		_ = s.store.RecordAudit(ctx, "mcp:"+tok.Name, "mcp."+toolName, target, ok, detail, &uid)
	}
}

// extractMCPBearerToken membaca token Bearer dari header Authorization atau X-API-Key
func extractMCPBearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if key := strings.TrimSpace(r.Header.Get("X-API-Key")); key != "" {
		return key
	}
	return ""
}

// handleMCP menangani endpoint publik /mcp (Streamable HTTP) dengan autentikasi Bearer Token 256-bit
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	ip := getIP(r)
	if !s.limiter.Allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{
			"error": "terlalu banyak percobaan autentikasi MCP gagal; coba lagi nanti",
		})
		return
	}

	rawToken := extractMCPBearerToken(r)
	if rawToken == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="FikPanel MCP"`)
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "header Authorization: Bearer <token> wajib disertakan",
		})
		return
	}

	tok, err := s.store.VerifyMCPToken(r.Context(), rawToken)
	if err != nil {
		s.limiter.RecordFailure(ip)
		w.Header().Set("WWW-Authenticate", `Bearer realm="FikPanel MCP", error="invalid_token"`)
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "token MCP tidak valid atau telah dinonaktifkan",
		})
		return
	}
	s.limiter.Reset(ip)

	// Nonaktifkan WriteTimeout global (180s) khusus aliran SSE jangka panjang GET /mcp
	if r.Method == http.MethodGet {
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})
	}

	ctx := context.WithValue(r.Context(), mcpTokenCtxKey, tok)
	ctx = context.WithValue(ctx, mcpIPCtxKey, ip)
	s.mcpHandler.ServeHTTP(w, r.WithContext(ctx))
}

// ═══ HTTP Handlers Manajemen Token MCP (Terautentikasi Panel) ═══

func (s *Server) handleListMCPTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.store.ListMCPTokens(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"endpoint":  "/mcp",
		"transport": "streamable-http",
		"protocol":  "MCP 2025-03-26",
		"tokens":    tokens,
		"tools":     mcpToolCatalog(),
		"prompts":   mcpPromptCatalog(),
	})
}

func (s *Server) handleCreateMCPToken(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	var uid int64
	if user != nil {
		uid = user.ID
	}
	var req struct {
		Name  string `json:"name"`
		Scope string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	tok, plain, err := s.store.CreateMCPToken(r.Context(), uid, req.Name, req.Scope)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "mcp.token_create", tok.Name, true, "Scope: "+tok.Scope, &uid)
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":      true,
		"token":   plain,
		"item":    tok,
		"message": fmt.Sprintf("Token MCP %q berhasil dibuat", tok.Name),
	})
}

func (s *Server) handleUpdateMCPToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID token tidak valid"})
		return
	}
	var req struct {
		Name    string `json:"name"`
		Scope   string `json:"scope"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.store.UpdateMCPToken(r.Context(), id, req.Name, req.Scope, req.Enabled); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "mcp.token_update", req.Name, true, fmt.Sprintf("Scope=%s Enabled=%v", req.Scope, req.Enabled), uid)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Token MCP diperbarui"})
}

func (s *Server) handleDeleteMCPToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID token tidak valid"})
		return
	}
	if err := s.store.DeleteMCPToken(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mencabut token MCP"})
		return
	}
	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "mcp.token_delete", strconv.FormatInt(id, 10), true, "Token MCP dicabut", uid)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Token MCP berhasil dicabut"})
}
