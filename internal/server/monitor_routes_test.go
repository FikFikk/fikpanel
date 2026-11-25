package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

// newMonitorTestServer membangun instance Server terisolasi untuk pengujian Observability & Alert Hub
func newMonitorTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "srv_mon_test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("gagal membuka store pengujian: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	user, err := st.EnsureDevUser("monadmin", "SuperSecret123!")
	if err != nil {
		t.Fatalf("gagal membuat akun admin pengujian: %v", err)
	}

	hub := metrics.NewHub()
	svc := services.NewManager(st.DB())
	srv, err := NewServer(st, hub, svc, 1405)
	if err != nil {
		t.Fatalf("gagal menginisialisasi Server: %v", err)
	}
	t.Cleanup(func() { srv.bgCancel() })

	token, err := st.CreateSession(context.Background(), user.ID, "127.0.0.1", "go-test", 3600*1e9)
	if err != nil {
		t.Fatalf("gagal membuat sesi uji: %v", err)
	}
	return srv, token
}

// TestMonitorDownsamplingAndSummary memverifikasi bucket downsampling mempertahankan lonjakan puncak dan menghitung analitik P95
func TestMonitorDownsamplingAndSummary(t *testing.T) {
	now := time.Now().Unix()
	raw := make([]store.MonitorSnapshot, 0, 200)
	for i := 0; i < 200; i++ {
		cpu := 20.0
		if i == 100 {
			cpu = 96.5 // Lonjakan singkat yang wajib tetap terlihat pasca-downsampling
		}
		raw = append(raw, store.MonitorSnapshot{
			ID:        int64(i + 1),
			Timestamp: now - int64(200-i)*60,
			CPU:       cpu,
			CPUIOWait: 2.5,
			RAMUsed:   2 * 1024 * 1024 * 1024,
			RAMTotal:  4 * 1024 * 1024 * 1024,
			Load1m:    1.2,
			DiskUsed:  20 * 1024 * 1024 * 1024,
			DiskTotal: 50 * 1024 * 1024 * 1024,
			DiskRead:  1024 * 500,
			DiskWrite: 1024 * 250,
			NetIn:     1024 * 100,
			NetOut:    1024 * 50,
		})
	}

	sampled := downsampleMonitorSnapshots(raw, 40)
	if len(sampled) != 40 {
		t.Fatalf("diharapkan 40 titik hasil downsampling, dapat %d", len(sampled))
	}
	foundSpike := false
	for _, s := range sampled {
		if s.CPU >= 96.0 {
			foundSpike = true
			break
		}
	}
	if !foundSpike {
		t.Fatal("downsampling menghilangkan lonjakan puncak CPU 96.5%")
	}

	sum := computeMonitorSummary(raw, len(sampled))
	if sum.RawCount != 200 || sum.ReturnedCount != 40 || sum.CPUMax != 96.5 || sum.RAMAvgPct != 50.0 || sum.DiskLatestPct != 40.0 {
		t.Fatalf("ringkasan analitik monitor tidak sesuai: %+v", sum)
	}
}

// TestMonitorRoutesAlertsAndCSVExport memverifikasi endpoint histori, export CSV, mesin alert threshold + recovery, dan uptime detail
func TestMonitorRoutesAlertsAndCSVExport(t *testing.T) {
	srv, token := newMonitorTestServer(t)
	ctx := context.Background()

	doReq := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.RemoteAddr = "127.0.0.1:47000"
		if method != http.MethodGet {
			req.Header.Set("X-Panel-Request", "1")
			req.Header.Set("Content-Type", "application/json")
		}
		req.AddCookie(&http.Cookie{Name: "panel_session", Value: token})
		rec := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rec, req)
		return rec
	}

	// 1. Masukkan sampel historis dan uji mesin evaluasi ambang batas (spike -> alert, turun -> recovered)
	now := time.Now().Unix()
	spikeSnap := &store.MonitorSnapshot{
		Timestamp: now - 120,
		CPU:       96.0,
		RAMUsed:   3 * 1024 * 1024 * 1024,
		RAMTotal:  4 * 1024 * 1024 * 1024,
		DiskUsed:  10 * 1024 * 1024 * 1024,
		DiskTotal: 50 * 1024 * 1024 * 1024,
	}
	if err := srv.store.InsertMonitorSnapshot(ctx, spikeSnap); err != nil {
		t.Fatalf("gagal menyimpan snapshot spike: %v", err)
	}
	srv.evaluateResourceAlerts(ctx, spikeSnap)

	coolSnap := &store.MonitorSnapshot{
		Timestamp: now - 60,
		CPU:       30.0,
		RAMUsed:   2 * 1024 * 1024 * 1024,
		RAMTotal:  4 * 1024 * 1024 * 1024,
		DiskUsed:  10 * 1024 * 1024 * 1024,
		DiskTotal: 50 * 1024 * 1024 * 1024,
	}
	_ = srv.store.InsertMonitorSnapshot(ctx, coolSnap)
	srv.evaluateResourceAlerts(ctx, coolSnap)

	// 2. GET /api/monitor/alerts wajib memuat 2 peristiwa CPU (critical & recovered)
	recAlerts := doReq(http.MethodGet, "/api/monitor/alerts?source=cpu", "")
	if recAlerts.Code != http.StatusOK {
		t.Fatalf("GET /api/monitor/alerts diharapkan 200, dapat %d: %s", recAlerts.Code, recAlerts.Body.String())
	}
	var alertsPayload struct {
		OK           bool                 `json:"ok"`
		UnackedCount int                  `json:"unacked_count"`
		Alerts       []store.MonitorAlert `json:"alerts"`
	}
	if err := json.Unmarshal(recAlerts.Body.Bytes(), &alertsPayload); err != nil || !alertsPayload.OK || len(alertsPayload.Alerts) != 2 {
		t.Fatalf("diharapkan 2 alert CPU (critical & recovered), dapat: %s", recAlerts.Body.String())
	}

	// 3. POST /api/monitor/alerts/ack-all harus menolkan unacked_count
	recAck := doReq(http.MethodPost, "/api/monitor/alerts/ack-all", "")
	if recAck.Code != http.StatusOK {
		t.Fatalf("POST /api/monitor/alerts/ack-all gagal: %d", recAck.Code)
	}
	if srv.store.CountUnackedMonitorAlerts(ctx) != 0 {
		t.Fatal("unacked_count seharusnya 0 setelah ack-all")
	}

	// 4. POST /api/monitor/alerts/thresholds harus menyinkronkan preferensi ambang batas
	recTh := doReq(http.MethodPost, "/api/monitor/alerts/thresholds", `{"alert_cpu_pct":80,"alert_ram_pct":82,"alert_disk_pct":88,"alert_ssl_days":21,"alert_cooldown_min":10}`)
	if recTh.Code != http.StatusOK {
		t.Fatalf("POST /api/monitor/alerts/thresholds gagal (%d): %s", recTh.Code, recTh.Body.String())
	}
	prefs := srv.loadPanelPreferences(ctx)
	if prefs.AlertCPUPct != 80 || prefs.AlertDiskPct != 88 || prefs.AlertSSLDays != 21 {
		t.Fatalf("preferensi threshold tidak tersimpan: %+v", prefs)
	}

	// 5. GET /api/monitor/history dan GET /api/monitor/history/export (CSV)
	recHist := doReq(http.MethodGet, "/api/monitor/history?max_points=100", "")
	if recHist.Code != http.StatusOK {
		t.Fatalf("GET /api/monitor/history gagal (%d): %s", recHist.Code, recHist.Body.String())
	}
	recCSV := doReq(http.MethodGet, "/api/monitor/history/export", "")
	if recCSV.Code != http.StatusOK || !strings.Contains(recCSV.Body.String(), "cpu_iowait_pct") {
		t.Fatalf("GET /api/monitor/history/export gagal atau header CSV hilang: %d %s", recCSV.Code, recCSV.Body.String())
	}

	// 6. Uji endpoint baru Uptime history & check-all
	mon := &store.UptimeMonitor{Name: "API Uji", Target: "https://example.com", MonitorType: "http", IntervalSec: 60, TimeoutSec: 5, Enabled: false}
	if err := srv.store.CreateUptimeMonitor(ctx, mon); err != nil {
		t.Fatalf("gagal membuat monitor uji: %v", err)
	}
	_ = srv.store.RecordUptimeCheck(ctx, mon.ID, "up", 200, 45, 60, "Let's Encrypt", "", 0)
	_ = srv.store.RecordUptimeCheck(ctx, mon.ID, "up", 200, 120, 60, "Let's Encrypt", "", 0)

	recUptimeHist := doReq(http.MethodGet, "/api/uptime/monitors/1/history", "")
	if recUptimeHist.Code != http.StatusOK || !strings.Contains(recUptimeHist.Body.String(), `"p95_latency_ms":120`) {
		t.Fatalf("GET /api/uptime/monitors/1/history gagal: %d %s", recUptimeHist.Code, recUptimeHist.Body.String())
	}
	recCheckAll := doReq(http.MethodPost, "/api/uptime/check-all", "")
	if recCheckAll.Code != http.StatusOK {
		t.Fatalf("POST /api/uptime/check-all gagal: %d %s", recCheckAll.Code, recCheckAll.Body.String())
	}
}
