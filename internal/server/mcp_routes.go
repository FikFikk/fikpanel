package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/ai"
	"fikpanel/internal/docker"
	"fikpanel/internal/store"

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

func mcpToolCatalog() []MCPToolCatalogItem {
	return []MCPToolCatalogItem{
		{Name: "server_overview", Scope: "readonly", Title: "Ringkasan Server", Description: "CPU, RAM, Disk, Load Average, OS, kernel, uptime, jumlah situs & layanan aktif."},
		{Name: "list_sites", Scope: "readonly", Title: "Daftar Website", Description: "Seluruh situs web FikPanel beserta domain, runtime (PHP/Node/Go/Python/Static/Proxy), SSL, dan root path."},
		{Name: "read_site_logs", Scope: "readonly", Title: "Log Situs (Error/Access)", Description: "Baca tail log error atau access domain tertentu (otomatis menyensor rahasia/token)."},
		{Name: "nginx_status_and_logs", Scope: "readonly", Title: "Status & Error Log Nginx", Description: "Inspeksi status runtime Nginx, PID, worker, RAM, dan tail error.log global."},
		{Name: "list_services", Scope: "readonly", Title: "Daftar Layanan Systemd", Description: "Daftar daemon Linux (running/failed/all) beserta PID, penggunaan memori, dan uptime."},
		{Name: "read_service_logs", Scope: "readonly", Title: "Log Jurnal Systemd", Description: "Baca output journalctl terbaru untuk unit layanan tertentu (nginx, php-fpm, mysql, dll.)."},
		{Name: "list_uptime_monitors", Scope: "readonly", Title: "Uptime & Masa Aktif SSL", Description: "Status online/offline monitor HTTP/TCP, persentase uptime 24 jam, latensi, dan sisa hari SSL."},
		{Name: "get_waf_summary", Scope: "readonly", Title: "Ringkasan & Insiden WAF", Description: "Statistik proteksi WAF beserta daftar serangan terbaru (IP, negara, jenis serangan, path target)."},
		{Name: "list_cron_jobs", Scope: "readonly", Title: "Daftar Tugas Cron", Description: "Seluruh jadwal cron aktif beserta perintah dan status eksekusi terakhir."},
		{Name: "list_audit_logs", Scope: "readonly", Title: "Audit Log Keamanan", Description: "Riwayat aktivitas keamanan panel terbaru termasuk login dan pemanggilan tool MCP."},
		{Name: "list_docker_containers", Scope: "readonly", Title: "Daftar Kontainer Docker", Description: "Daftar kontainer Docker aktif & berhenti beserta image, status, dan pemetaan port."},
		{Name: "read_docker_logs", Scope: "readonly", Title: "Log Kontainer Docker", Description: "Baca tail log stdout/stderr kontainer Docker tertentu (tersensor otomatis dari rahasia)."},
		{Name: "restart_service", Scope: "operator", Title: "Restart / Reload Service", Description: "Me-restart atau me-reload unit systemd non-destruktif (unit vital tetap dilindungi dari stop)."},
		{Name: "restart_docker_container", Scope: "operator", Title: "Restart / Start Kontainer Docker", Description: "Menjalankan restart atau start pada kontainer Docker target secara aman tanpa menghapus kontainer/volume."},
		{Name: "test_and_reload_nginx", Scope: "operator", Title: "Uji & Reload Nginx", Description: "Menjalankan pengujian sintaks nginx -t lalu menerapkan reload konfigurasi tanpa downtime."},
		{Name: "check_uptime_now", Scope: "operator", Title: "Probe Uptime Langsung", Description: "Menjalankan pemeriksaan HTTP/TCP/SSL real-time untuk satu target monitor tertentu."},
		{Name: "check_site_dns", Scope: "operator", Title: "Cek Resolusi DNS Domain", Description: "Memeriksa apakah DNS publik domain situs sudah mengarah ke alamat IP server ini."},
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

func (s *Server) buildMCPServer(includeOperator bool) *mcp.Server {
	modeDesc := "READ-ONLY (hanya diagnosa, metrik, dan pembacaan log tersensor)"
	if includeOperator {
		modeDesc = "OPERATOR (diagnosa lengkap + restart/reload layanan, uji/reload Nginx, probe Uptime, cek DNS)"
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "fikpanel",
		Version: "v" + s.Version(),
	}, &mcp.ServerOptions{
		Instructions: "FikPanel Server Control Panel MCP Server. Mode sesi ini: " + modeDesc + ". " +
			"Selalu panggil list_sites atau list_services terlebih dahulu sebelum menebak nama domain/unit. " +
			"Seluruh output log sudah disensor otomatis dari rahasia/kata sandi (BR-04).",
	})

	// ── 10 Read-Only Diagnostic Tools ────────────────────────────────────────
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

	type readSiteLogsArgs struct {
		Domain  string `json:"domain" jsonschema:"Nama domain situs yang terdaftar di FikPanel (contoh: api.domain.com)"`
		LogType string `json:"log_type,omitempty" jsonschema:"Jenis log: 'error' (default) atau 'access'"`
		Lines   int    `json:"lines,omitempty" jsonschema:"Jumlah baris terakhir yang dibaca (default 80, maks 300)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_site_logs",
		Title:       "Baca Log Situs (Error / Access)",
		Description: "Membaca baris terakhir error.log atau access.log milik suatu domain situs untuk diagnosa 500/502/404. Data sensitif disensor otomatis.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args readSiteLogsArgs) (*mcp.CallToolResult, any, error) {
		domain := strings.TrimSpace(args.Domain)
		if domain == "" {
			return nil, nil, fmt.Errorf("parameter domain wajib diisi")
		}
		// Pastikan domain benar-benar terdaftar di FikPanel agar mencegah path traversal
		allSites, err := s.store.ListSites(ctx)
		if err != nil {
			return nil, nil, err
		}
		var matched *store.Site
		for i := range allSites {
			if strings.EqualFold(allSites[i].Domain, domain) {
				matched = &allSites[i]
				break
			}
		}
		if matched == nil {
			s.recordMCPAudit(ctx, "read_site_logs", domain, false, "Domain tidak ditemukan")
			return nil, nil, fmt.Errorf("domain %q tidak terdaftar di FikPanel; panggil list_sites untuk melihat daftar domain valid", domain)
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
		redacted := ai.RedactSecrets(content)
		s.recordMCPAudit(ctx, "read_site_logs", matched.Domain, true, fmt.Sprintf("Membaca %d baris %s log", lines, logType))
		return mcpJSONResult(map[string]any{
			"domain":   matched.Domain,
			"log_type": logType,
			"lines":    lines,
			"content":  redacted,
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

	if !includeOperator {
		return srv
	}

	// ── 5 Operator Action Tools (Khusus Token Scope "operator") ──────────────
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
