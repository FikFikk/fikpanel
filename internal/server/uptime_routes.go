package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/store"
)

// StartUptimeProber menjalankan worker latar belakang pemantauan Uptime & SSL untuk seluruh monitor aktif
func (s *Server) StartUptimeProber(ctx context.Context) {
	go func() {
		// Tunggu sebentar pasca-boot agar startup server tidak terbebani
		select {
		case <-ctx.Done():
			return
		case <-time.After(8 * time.Second):
		}

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			s.runDueUptimeProbes(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *Server) runDueUptimeProbes(ctx context.Context) {
	monitors, err := s.store.ListUptimeMonitors(ctx, false)
	if err != nil || len(monitors) == 0 {
		return
	}
	now := time.Now().Unix()
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup

	for _, mon := range monitors {
		if !mon.Enabled {
			continue
		}
		interval := mon.IntervalSec
		if interval < 30 {
			interval = 30
		}
		if mon.LastCheckedAt > 0 && now-mon.LastCheckedAt < int64(interval) {
			continue
		}

		m := mon
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			probeCtx, cancel := context.WithTimeout(ctx, time.Duration(clampInt(m.TimeoutSec, 2, 30, 10)+3)*time.Second)
			defer cancel()
			s.executeAndRecordProbe(probeCtx, &m)
		}()
	}
	wg.Wait()
}

type probeOutcome struct {
	Status    string
	Code      int
	LatencyMs int64
	SSLDays   int
	SSLIssuer string
	ErrMsg    string
}

func runUptimeProbe(ctx context.Context, m *store.UptimeMonitor) probeOutcome {
	timeout := time.Duration(clampInt(m.TimeoutSec, 2, 30, 10)) * time.Second
	start := time.Now()

	if strings.EqualFold(m.MonitorType, "tcp") {
		dialer := &net.Dialer{Timeout: timeout}
		conn, err := dialer.DialContext(ctx, "tcp", strings.TrimSpace(m.Target))
		lat := time.Since(start).Milliseconds()
		if err != nil {
			return probeOutcome{Status: "down", LatencyMs: lat, SSLDays: -1, ErrMsg: err.Error()}
		}
		_ = conn.Close()
		return probeOutcome{Status: "up", Code: 1, LatencyMs: lat, SSLDays: -1}
	}

	// Probe HTTP / HTTPS beserta inspeksi sisa masa berlaku sertifikat TLS peer
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		DisableKeepAlives: true,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(m.Target), nil)
	if err != nil {
		return probeOutcome{Status: "down", LatencyMs: time.Since(start).Milliseconds(), SSLDays: -1, ErrMsg: err.Error()}
	}
	req.Header.Set("User-Agent", "FikPanel-UptimeMonitor/0.7.0")

	resp, err := client.Do(req)
	lat := time.Since(start).Milliseconds()
	if err != nil {
		return probeOutcome{Status: "down", LatencyMs: lat, SSLDays: -1, ErrMsg: err.Error()}
	}
	defer resp.Body.Close()

	sslDays := -1
	sslIssuer := ""
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		leaf := resp.TLS.PeerCertificates[0]
		sslDays = int(time.Until(leaf.NotAfter).Hours() / 24)
		sslIssuer = leaf.Issuer.CommonName
		if sslIssuer == "" && len(leaf.Issuer.Organization) > 0 {
			sslIssuer = leaf.Issuer.Organization[0]
		}
	}

	statusOK := false
	if m.ExpectedStatus > 0 {
		statusOK = resp.StatusCode == m.ExpectedStatus
	} else {
		statusOK = resp.StatusCode >= 200 && resp.StatusCode < 400
	}

	errMsg := ""
	if !statusOK {
		errMsg = fmt.Sprintf("HTTP %d (harapan %d)", resp.StatusCode, m.ExpectedStatus)
	}

	// Pemeriksaan kata kunci isi respons bila dikonfigurasi
	if statusOK && strings.TrimSpace(m.Keyword) != "" {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		if !strings.Contains(string(body), m.Keyword) {
			statusOK = false
			errMsg = fmt.Sprintf("Kata kunci %q tidak ditemukan pada respons", m.Keyword)
		}
	} else {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8*1024))
	}

	st := "down"
	if statusOK {
		st = "up"
	}
	return probeOutcome{
		Status:    st,
		Code:      resp.StatusCode,
		LatencyMs: lat,
		SSLDays:   sslDays,
		SSLIssuer: sslIssuer,
		ErrMsg:    errMsg,
	}
}

func (s *Server) executeAndRecordProbe(ctx context.Context, m *store.UptimeMonitor) probeOutcome {
	out := runUptimeProbe(ctx, m)
	now := time.Now().Unix()
	var notifiedAt int64

	if m.NotifyWebhook {
		prefs := s.loadPanelPreferences(ctx)
		cooldownSec := int64(clampInt(prefs.AlertCooldownMin, 1, 1440, 15)) * 60
		canNotify := m.LastNotifiedAt == 0 || (now-m.LastNotifiedAt) >= cooldownSec

		// Kirim notifikasi perubahan status (DOWN / RECOVERED) atau SSL mendekati kedaluwarsa
		if m.LastStatus != "down" && out.Status == "down" {
			notifiedAt = now
			go s.broadcastUptimeWebhook(m, "DOWN", fmt.Sprintf("Layanan %s (%s) TIDAK DAPAT DIAKSES. Detail: %s", m.Name, m.Target, out.ErrMsg), true)
		} else if m.LastStatus == "down" && out.Status == "up" {
			notifiedAt = now
			go s.broadcastUptimeWebhook(m, "RECOVERED", fmt.Sprintf("Layanan %s (%s) kembali ONLINE (HTTP %d, %d ms).", m.Name, m.Target, out.Code, out.LatencyMs), false)
		} else if out.Status == "up" && out.SSLDays >= 0 && out.SSLDays <= prefs.AlertSSLDays && canNotify {
			notifiedAt = now
			go s.broadcastUptimeWebhook(m, "SSL EXPIRING", fmt.Sprintf("Sertifikat SSL %s (%s) tersisa %d hari lagi.", m.Name, m.Target, out.SSLDays), true)
		}
	}

	_ = s.store.RecordUptimeCheck(ctx, m.ID, out.Status, out.Code, out.LatencyMs, out.SSLDays, out.SSLIssuer, out.ErrMsg, notifiedAt)
	return out
}

// broadcastUptimeWebhook mengirim notifikasi alert ke seluruh webhook aktif (Telegram, Discord Embed, Custom JSON)
func (s *Server) broadcastUptimeWebhook(m *store.UptimeMonitor, eventTag, detail string, isAlert bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hooks, err := s.store.ListWebhooks(ctx)
	if err != nil || len(hooks) == 0 {
		return
	}
	host, _ := os.Hostname()
	title := fmt.Sprintf("[FikPanel · %s] %s", eventTag, m.Name)
	fullMsg := fmt.Sprintf("%s\n%s\nHost: %s · %s", title, detail, host, time.Now().Format("2006-01-02 15:04:05 MST"))

	color := 0x10B981 // Hijau pulih
	if isAlert {
		color = 0xF43F5E // Merah gangguan
	}

	client := &http.Client{Timeout: 8 * time.Second}
	for _, h := range hooks {
		// Untuk event SSL, hormati flag notify_on_ssl_expiry
		if eventTag == "SSL EXPIRING" && !h.NotifyOnSSLExpiry {
			continue
		}
		targetURL, payload, err := buildAlertWebhookPayload(&h, title, fullMsg, detail, eventTag, color, s.Version())
		if err != nil {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "FikPanel-UptimeAlert/"+s.Version())
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
			_ = resp.Body.Close()
		}
	}
}

func buildAlertWebhookPayload(hook *store.SystemWebhook, title, fullMsg, detail, eventTag string, discordColor int, ver string) (string, []byte, error) {
	rawURL := strings.TrimSpace(hook.WebhookURL)
	if strings.HasPrefix(rawURL, "tg://") {
		parts := strings.SplitN(strings.TrimPrefix(rawURL, "tg://"), "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", nil, fmt.Errorf("format tg:// tidak valid")
		}
		endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", parts[0])
		b, _ := json.Marshal(map[string]interface{}{"chat_id": parts[1], "text": fullMsg})
		return endpoint, b, nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", nil, fmt.Errorf("URL webhook tidak valid")
	}
	switch strings.ToLower(hook.Channel) {
	case "telegram":
		body := map[string]interface{}{"text": fullMsg}
		if cid := parsed.Query().Get("chat_id"); cid != "" {
			body["chat_id"] = cid
		}
		b, _ := json.Marshal(body)
		return rawURL, b, nil
	case "discord":
		b, _ := json.Marshal(map[string]interface{}{
			"username": "FikPanel Uptime",
			"embeds": []map[string]interface{}{{
				"title":       title,
				"description": detail,
				"color":       discordColor,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			}},
		})
		return rawURL, b, nil
	default:
		b, _ := json.Marshal(map[string]interface{}{
			"source":  "fikpanel",
			"version": ver,
			"event":   strings.ToLower(eventTag),
			"title":   title,
			"message": detail,
			"ts":      time.Now().Unix(),
		})
		return rawURL, b, nil
	}
}

func sanitizeUptimeMonitorInput(in *store.UptimeMonitor) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Target = strings.TrimSpace(in.Target)
	in.MonitorType = strings.ToLower(strings.TrimSpace(in.MonitorType))
	if in.MonitorType != "tcp" {
		in.MonitorType = "http"
	}
	if in.Name == "" || in.Target == "" {
		return fmt.Errorf("nama dan target URL/Host wajib diisi")
	}
	if in.MonitorType == "http" {
		if !strings.HasPrefix(strings.ToLower(in.Target), "http://") && !strings.HasPrefix(strings.ToLower(in.Target), "https://") {
			in.Target = "https://" + in.Target
		}
		u, err := url.Parse(in.Target)
		if err != nil || u.Host == "" {
			return fmt.Errorf("format URL HTTP/HTTPS tidak valid")
		}
	} else {
		if _, _, err := net.SplitHostPort(in.Target); err != nil {
			return fmt.Errorf("target TCP wajib berformat host:port (contoh: 127.0.0.1:3306)")
		}
	}
	in.IntervalSec = clampInt(in.IntervalSec, 30, 3600, 60)
	in.TimeoutSec = clampInt(in.TimeoutSec, 2, 30, 10)
	if in.ExpectedStatus < 0 || in.ExpectedStatus > 599 {
		in.ExpectedStatus = 200
	}
	in.Keyword = strings.TrimSpace(in.Keyword)
	return nil
}

// ═══ HTTP Handlers Uptime & SSL Monitor (Terautentikasi) ═══

func (s *Server) handleListUptimeMonitors(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListUptimeMonitors(r.Context(), false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"monitors":   list,
		"status_url": "/status",
	})
}

func (s *Server) handleCreateUptimeMonitor(w http.ResponseWriter, r *http.Request) {
	var m store.UptimeMonitor
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if m.IntervalSec == 0 {
		m.IntervalSec = 60
	}
	if m.TimeoutSec == 0 {
		m.TimeoutSec = 10
	}
	if err := sanitizeUptimeMonitorInput(&m); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.CreateUptimeMonitor(r.Context(), &m); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Langsung jalankan pemeriksaan perdana di latar belakang agar status segera terisi
	if m.Enabled {
		go func(mon store.UptimeMonitor) {
			ctx, cancel := context.WithTimeout(s.bgCtx, time.Duration(mon.TimeoutSec+3)*time.Second)
			defer cancel()
			s.executeAndRecordProbe(ctx, &mon)
		}(m)
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"ok":      true,
		"monitor": m,
		"message": fmt.Sprintf("Monitor %s berhasil ditambahkan", m.Name),
	})
}

func (s *Server) handleUpdateUptimeMonitor(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID monitor tidak valid"})
		return
	}
	var m store.UptimeMonitor
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	m.ID = id
	if err := sanitizeUptimeMonitorInput(&m); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.UpdateUptimeMonitor(r.Context(), &m); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memperbarui monitor"})
		return
	}
	updated, _ := s.store.GetUptimeMonitor(r.Context(), id)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"monitor": updated,
		"message": "Monitor berhasil diperbarui",
	})
}

func (s *Server) handleDeleteUptimeMonitor(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID monitor tidak valid"})
		return
	}
	if err := s.store.DeleteUptimeMonitor(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menghapus monitor"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Monitor dihapus"})
}

func (s *Server) handleCheckUptimeMonitorNow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID monitor tidak valid"})
		return
	}
	mon, err := s.store.GetUptimeMonitor(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "monitor tidak ditemukan"})
		return
	}
	out := s.executeAndRecordProbe(r.Context(), mon)
	fresh, _ := s.store.GetUptimeMonitor(r.Context(), id)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"outcome": out,
		"monitor": fresh,
	})
}

// handleImportSitesToUptime mengimpor seluruh situs aktif yang dikelola FikPanel ke daftar Uptime Monitor secara 1-klik
func (s *Server) handleImportSitesToUptime(w http.ResponseWriter, r *http.Request) {
	sitesList, err := s.store.ListSites(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membaca daftar situs"})
		return
	}
	existing, _ := s.store.ListUptimeMonitors(r.Context(), false)
	seen := make(map[string]bool, len(existing))
	for _, ex := range existing {
		seen[strings.ToLower(strings.TrimRight(ex.Target, "/"))] = true
	}

	added := 0
	for _, st := range sitesList {
		if st.Enabled != 1 || strings.TrimSpace(st.Domain) == "" {
			continue
		}
		scheme := "http"
		if st.SSLMode != "" && st.SSLMode != "none" {
			scheme = "https"
		}
		target := fmt.Sprintf("%s://%s", scheme, strings.TrimSpace(st.Domain))
		key := strings.ToLower(target)
		if seen[key] {
			continue
		}
		m := store.UptimeMonitor{
			Name:           st.Domain,
			Target:         target,
			MonitorType:    "http",
			IntervalSec:    60,
			TimeoutSec:     10,
			ExpectedStatus: 0, // Terima 2xx–3xx agar redirect www/login tidak false-alarm
			NotifyWebhook:  true,
			PublicVisible:  true,
			Enabled:        true,
		}
		if err := s.store.CreateUptimeMonitor(r.Context(), &m); err == nil {
			seen[key] = true
			added++
			go func(mon store.UptimeMonitor) {
				ctx, cancel := context.WithTimeout(s.bgCtx, 12*time.Second)
				defer cancel()
				s.executeAndRecordProbe(ctx, &mon)
			}(m)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"added":   added,
		"message": fmt.Sprintf("%d situs berhasil ditambahkan ke Uptime Monitor", added),
	})
}

// ═══ Public Status Page API & Halaman HTML Mandiri (/status) ═══

func (s *Server) buildPublicStatusData(ctx context.Context) map[string]interface{} {
	prefs := s.loadPanelPreferences(ctx)
	brand := prefs.BrandName
	if strings.TrimSpace(brand) == "" {
		brand = "FikPanel"
	}
	monitors, _ := s.store.ListUptimeMonitors(ctx, true)

	overall := "operational"
	downCount := 0
	var uptimeSum float64

	type publicCheck struct {
		Status    string `json:"status"`
		LatencyMs int64  `json:"latency_ms"`
		CheckedAt int64  `json:"checked_at"`
	}
	type publicMon struct {
		Name          string        `json:"name"`
		MonitorType   string        `json:"monitor_type"`
		Status        string        `json:"status"`
		Uptime24hPct  float64       `json:"uptime_24h_pct"`
		LatencyMs     int64         `json:"latency_ms"`
		AvgLatencyMs  int64         `json:"avg_latency_ms"`
		SSLDays       int           `json:"ssl_days"`
		LastCheckedAt int64         `json:"last_checked_at"`
		Heartbeats    []publicCheck `json:"heartbeats"`
	}

	outMons := make([]publicMon, 0, len(monitors))
	for _, m := range monitors {
		if m.LastStatus == "down" {
			downCount++
		}
		uptimeSum += m.Uptime24hPct
		hbs := make([]publicCheck, 0, len(m.RecentChecks))
		for _, c := range m.RecentChecks {
			hbs = append(hbs, publicCheck{Status: c.Status, LatencyMs: c.LatencyMs, CheckedAt: c.CheckedAt})
		}
		outMons = append(outMons, publicMon{
			Name:          m.Name,
			MonitorType:   m.MonitorType,
			Status:        m.LastStatus,
			Uptime24hPct:  m.Uptime24hPct,
			LatencyMs:     m.LastLatencyMs,
			AvgLatencyMs:  m.AvgLatencyMs,
			SSLDays:       m.SSLDaysRemaining,
			LastCheckedAt: m.LastCheckedAt,
			Heartbeats:    hbs,
		})
	}

	if len(monitors) > 0 && downCount == len(monitors) {
		overall = "outage"
	} else if downCount > 0 {
		overall = "degraded"
	}
	overallPct := 100.0
	if len(monitors) > 0 {
		overallPct = float64(int((uptimeSum/float64(len(monitors)))*100+0.5)) / 100.0
	}

	return map[string]interface{}{
		"ok":                 true,
		"brand_name":         brand,
		"overall_status":     overall,
		"overall_uptime_pct": overallPct,
		"down_count":         downCount,
		"total_count":        len(monitors),
		"updated_at":         time.Now().Unix(),
		"monitors":           outMons,
	}
}

func (s *Server) handlePublicStatusJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, s.buildPublicStatusData(r.Context()))
}

var publicStatusTmpl = template.Must(template.New("status").Parse(`<!doctype html>
<html lang="id">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <meta http-equiv="refresh" content="30" />
  <title>Status Layanan · {{.Brand}}</title>
  <style>
    :root { --bg:#090a0f; --card:#0e1119; --inset:#0b0d14; --border:#1f2738; --text:#f1f5f9; --muted:#94a3b8; --up:#10b981; --warn:#f59e0b; --down:#f43f5e; }
    * { box-sizing:border-box; margin:0; padding:0; }
    body { font-family:system-ui,-apple-system,Segoe UI,sans-serif; background:var(--bg); color:var(--text); line-height:1.5; padding:2rem 1rem; }
    .wrap { max-width:860px; margin:0 auto; display:flex; flex-direction:column; gap:1.25rem; }
    .head { display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:0.75rem; }
    .brand { font-size:1.25rem; font-weight:700; letter-spacing:-0.02em; }
    .meta { font-size:0.8rem; color:var(--muted); }
    .banner { display:flex; align-items:center; justify-content:space-between; gap:1rem; padding:1rem 1.25rem; border-radius:12px; border:1px solid var(--border); background:var(--card); }
    .banner.up { border-color:rgba(16,185,129,.45); background:rgba(16,185,129,.08); }
    .banner.degraded { border-color:rgba(245,158,11,.45); background:rgba(245,158,11,.08); }
    .banner.outage { border-color:rgba(244,63,94,.45); background:rgba(244,63,94,.1); }
    .banner-title { font-weight:700; font-size:1rem; display:flex; align-items:center; gap:0.6rem; }
    .dot { width:10px; height:10px; border-radius:50%; display:inline-block; background:var(--up); }
    .degraded .dot { background:var(--warn); }
    .outage .dot { background:var(--down); }
    .list { display:flex; flex-direction:column; gap:0.75rem; }
    .item { background:var(--card); border:1px solid var(--border); border-radius:12px; padding:1rem 1.15rem; display:flex; flex-direction:column; gap:0.65rem; }
    .item-top { display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:0.5rem; }
    .item-name { font-weight:600; font-size:0.95rem; display:flex; align-items:center; gap:0.5rem; }
    .pill { font-size:0.72rem; font-weight:600; padding:2px 8px; border-radius:999px; background:var(--inset); border:1px solid var(--border); color:var(--muted); }
    .pill.up { color:#34d399; border-color:rgba(16,185,129,.35); background:rgba(16,185,129,.1); }
    .pill.down { color:#fb7185; border-color:rgba(244,63,94,.35); background:rgba(244,63,94,.1); }
    .bars { display:grid; grid-template-columns:repeat(30,minmax(0,1fr)); gap:3px; height:24px; align-items:stretch; }
    .bar { border-radius:3px; background:#1e2433; }
    .bar.up { background:var(--up); }
    .bar.down { background:var(--down); }
    .item-foot { display:flex; justify-content:space-between; font-size:0.75rem; color:var(--muted); }
    .empty { text-align:center; padding:2.5rem 1rem; color:var(--muted); background:var(--card); border:1px solid var(--border); border-radius:12px; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="head">
      <div class="brand">{{.Brand}} · Status Page</div>
      <div class="meta">Auto-refresh 30s · Diperbarui {{.UpdatedStr}}</div>
    </div>
    <div class="banner {{.BannerClass}}">
      <div class="banner-title"><span class="dot"></span> {{.BannerTitle}}</div>
      <div class="pill {{if eq .BannerClass "up"}}up{{else}}down{{end}}">{{.OverallPct}}% Uptime (24j)</div>
    </div>
    {{if eq (len .Monitors) 0}}
      <div class="empty">Belum ada layanan publik yang dipublikasikan pada Status Page.</div>
    {{else}}
      <div class="list">
        {{range .Monitors}}
          <div class="item">
            <div class="item-top">
              <div class="item-name">
                <span>{{.Name}}</span>
                <span class="pill">{{.TypeUpper}}</span>
                {{if ge .SSLDays 0}}<span class="pill {{if gt .SSLDays 14}}up{{else}}down{{end}}">SSL {{.SSLDays}} hari</span>{{end}}
              </div>
              <div class="item-name">
                <span class="pill">{{.LatencyMs}} ms</span>
                <span class="pill {{if eq .Status "up"}}up{{else if eq .Status "down"}}down{{end}}">{{.StatusLabel}}</span>
              </div>
            </div>
            <div class="bars">
              {{range .Bars}}<span class="bar {{.Class}}" title="{{.Title}}"></span>{{end}}
            </div>
            <div class="item-foot">
              <span>30 pemeriksaan terakhir</span>
              <span>Uptime 24j: <strong>{{.UptimePct}}%</strong></span>
            </div>
          </div>
        {{end}}
      </div>
    {{end}}
  </div>
</body>
</html>`))

func (s *Server) handlePublicStatusHTML(w http.ResponseWriter, r *http.Request) {
	prefs := s.loadPanelPreferences(r.Context())
	brand := strings.TrimSpace(prefs.BrandName)
	if brand == "" {
		brand = "FikPanel"
	}
	monitors, _ := s.store.ListUptimeMonitors(r.Context(), true)

	type barView struct {
		Class string
		Title string
	}
	type monView struct {
		Name        string
		TypeUpper   string
		Status      string
		StatusLabel string
		LatencyMs   int64
		UptimePct   string
		SSLDays     int
		Bars        []barView
	}

	downCount := 0
	var sumPct float64
	views := make([]monView, 0, len(monitors))
	for _, m := range monitors {
		if m.LastStatus == "down" {
			downCount++
		}
		sumPct += m.Uptime24hPct
		bars := make([]barView, 0, 30)
		pad := 30 - len(m.RecentChecks)
		for i := 0; i < pad; i++ {
			bars = append(bars, barView{Class: "", Title: "Belum ada data"})
		}
		for _, c := range m.RecentChecks {
			ts := time.Unix(c.CheckedAt, 0).Format("15:04:05")
			bars = append(bars, barView{
				Class: c.Status,
				Title: fmt.Sprintf("%s · %s · %d ms", ts, strings.ToUpper(c.Status), c.LatencyMs),
			})
		}
		stLabel := "Menunggu"
		if m.LastStatus == "up" {
			stLabel = "Operational"
		} else if m.LastStatus == "down" {
			stLabel = "Gangguan"
		}
		views = append(views, monView{
			Name:        m.Name,
			TypeUpper:   strings.ToUpper(m.MonitorType),
			Status:      m.LastStatus,
			StatusLabel: stLabel,
			LatencyMs:   m.LastLatencyMs,
			UptimePct:   fmt.Sprintf("%.2f", m.Uptime24hPct),
			SSLDays:     m.SSLDaysRemaining,
			Bars:        bars,
		})
	}

	bannerClass := "up"
	bannerTitle := "Semua Layanan Berjalan Normal"
	if len(monitors) > 0 && downCount == len(monitors) {
		bannerClass = "outage"
		bannerTitle = "Seluruh Layanan Mengalami Gangguan"
	} else if downCount > 0 {
		bannerClass = "degraded"
		bannerTitle = fmt.Sprintf("%d Layanan Mengalami Gangguan", downCount)
	}
	overallPct := 100.0
	if len(monitors) > 0 {
		overallPct = sumPct / float64(len(monitors))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = publicStatusTmpl.Execute(w, map[string]interface{}{
		"Brand":       brand,
		"UpdatedStr":  time.Now().Format("02 Jan 2006 15:04:05 MST"),
		"BannerClass": bannerClass,
		"BannerTitle": bannerTitle,
		"OverallPct":  fmt.Sprintf("%.2f", overallPct),
		"Monitors":    views,
	})
}
