package server

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/store"
)

// MonitorHistorySummary merangkum statistik analitik server pada rentang waktu histori yang diminta
type MonitorHistorySummary struct {
	RawCount         int     `json:"raw_count"`
	ReturnedCount    int     `json:"returned_count"`
	CPUAvg           float64 `json:"cpu_avg"`
	CPUMax           float64 `json:"cpu_max"`
	CPUP95           float64 `json:"cpu_p95"`
	CPUIOWaitAvg     float64 `json:"cpu_iowait_avg"`
	RAMAvgPct        float64 `json:"ram_avg_pct"`
	RAMMaxPct        float64 `json:"ram_max_pct"`
	LoadMax          float64 `json:"load_max"`
	DiskLatestPct    float64 `json:"disk_latest_pct"`
	NetInPeak        uint64  `json:"net_in_peak"`
	NetOutPeak       uint64  `json:"net_out_peak"`
	NetInTotalBytes  uint64  `json:"net_in_total_bytes"`
	NetOutTotalBytes uint64  `json:"net_out_total_bytes"`
	DiskReadPeak     uint64  `json:"disk_read_peak"`
	DiskWritePeak    uint64  `json:"disk_write_peak"`
}

// StartMonitoringRecorder menjalankan goroutine pencatat metrik berkala (60 detik) beserta mesin evaluasi ambang alert
func (s *Server) StartMonitoringRecorder(ctx context.Context) {
	go func() {
		// Tunggu sebentar pasca-start untuk mengambil sampel baseline segar (menghindari grafik kosong pasca-reboot)
		select {
		case <-ctx.Done():
			return
		case <-time.After(6 * time.Second):
			// Pemanasan delta counter CPU/Disk/Net pertama lalu catat sampel setelah 2 detik
			_ = s.metrics.CollectFresh()
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				s.recordMonitorTick(ctx)
			}
		}

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		purgeTicker := time.NewTicker(24 * time.Hour)
		defer purgeTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-purgeTicker.C:
				_ = s.store.PurgeMonitorSnapshots(ctx)
			case <-ticker.C:
				s.recordMonitorTick(ctx)
			}
		}
	}()
}

// recordMonitorTick mengambil metrik segar langsung dari kernel, menyimpannya ke SQLite, dan mengevaluasi ambang alert
func (s *Server) recordMonitorTick(ctx context.Context) {
	cfg, err := s.store.GetMonitorConfig(ctx)
	if err != nil || !cfg.Enabled {
		return
	}

	// Wajib pakai CollectFresh agar rekaman 24/7 tetap akurat meski tidak ada tab browser yang membuka SSE
	pt := s.metrics.CollectFresh()
	if pt.Timestamp == 0 {
		return
	}

	snap := &store.MonitorSnapshot{
		Timestamp: pt.Timestamp,
		CPU:       pt.CPU,
		CPUIOWait: pt.CPUIOWait,
		CPUSteal:  pt.CPUSteal,
		RAMUsed:   pt.RAMUsed,
		RAMTotal:  pt.RAMTotal,
		Load1m:    pt.LoadAvg,
		Load5m:    pt.Load5m,
		Load15m:   pt.Load15m,
		DiskUsed:  pt.DiskUsed,
		DiskTotal: pt.DiskTotal,
		DiskRead:  pt.DiskRead,
		DiskWrite: pt.DiskWrite,
		NetIn:     pt.NetIn,
		NetOut:    pt.NetOut,
	}
	_ = s.store.InsertMonitorSnapshot(ctx, snap)

	// Evaluasi ambang batas CPU, RAM, dan Disk terhadap pengaturan preferensi alert & webhook
	s.evaluateResourceAlerts(ctx, snap)
}

// evaluateResourceAlerts memeriksa lonjakan CPU, RAM, dan Disk terhadap ambang batas & cooldown serta mencatat alert/pemulihan
func (s *Server) evaluateResourceAlerts(ctx context.Context, snap *store.MonitorSnapshot) {
	prefs := s.loadPanelPreferences(ctx)
	cooldownSec := int64(clampInt(prefs.AlertCooldownMin, 1, 1440, 15)) * 60
	now := snap.Timestamp
	if now == 0 {
		now = time.Now().Unix()
	}

	ramPct := 0.0
	if snap.RAMTotal > 0 {
		ramPct = (float64(snap.RAMUsed) / float64(snap.RAMTotal)) * 100.0
	}
	diskPct := 0.0
	if snap.DiskTotal > 0 {
		diskPct = (float64(snap.DiskUsed) / float64(snap.DiskTotal)) * 100.0
	}

	checks := []struct {
		source    string
		label     string
		value     float64
		threshold float64
	}{
		{"cpu", "Penggunaan CPU", snap.CPU, float64(clampInt(prefs.AlertCPUPct, 50, 99, 85))},
		{"ram", "Penggunaan RAM", ramPct, float64(clampInt(prefs.AlertRAMPct, 50, 99, 85))},
		{"disk", "Kapasitas Disk Root (/)", diskPct, float64(clampInt(prefs.AlertDiskPct, 50, 99, 90))},
	}

	for _, c := range checks {
		latest, _ := s.store.GetLatestAlertBySource(ctx, c.source)
		wasAlerting := latest != nil && latest.Severity != "recovered"

		if c.value >= c.threshold {
			// Hormati jeda cooldown agar tidak membanjiri log & webhook saat beban tinggi berkelanjutan
			if latest != nil && latest.Severity != "recovered" && (now-latest.CreatedAt) < cooldownSec {
				continue
			}
			sev := "warning"
			if c.value >= 95 || c.value >= c.threshold+7 {
				sev = "critical"
			}
			msg := fmt.Sprintf("%s mencapai %.1f%% (melewati ambang %.0f%%)", c.label, c.value, c.threshold)
			notified := s.broadcastResourceAlertWebhook(ctx, c.source, strings.ToUpper(sev), msg, true)
			_ = s.store.InsertMonitorAlert(ctx, &store.MonitorAlert{
				Source:      c.source,
				Severity:    sev,
				Target:      "host",
				MetricValue: math.Round(c.value*10) / 10,
				Threshold:   c.threshold,
				Message:     msg,
				Notified:    notified,
				CreatedAt:   now,
			})
		} else if wasAlerting && c.value <= c.threshold-5.0 {
			// Kirim catatan pemulihan dengan hysteresis 5% agar tidak flapping
			msg := fmt.Sprintf("%s kembali normal pada %.1f%% (di bawah ambang %.0f%%)", c.label, c.value, c.threshold)
			notified := s.broadcastResourceAlertWebhook(ctx, c.source, "RECOVERED", msg, false)
			_ = s.store.InsertMonitorAlert(ctx, &store.MonitorAlert{
				Source:      c.source,
				Severity:    "recovered",
				Target:      "host",
				MetricValue: math.Round(c.value*10) / 10,
				Threshold:   c.threshold,
				Message:     msg,
				Notified:    notified,
				CreatedAt:   now,
			})
		}
	}
}

// broadcastResourceAlertWebhook menyiarkan alert ambang CPU/RAM/Disk ke seluruh webhook aktif yang berlangganan
func (s *Server) broadcastResourceAlertWebhook(ctx context.Context, source, eventTag, detail string, isAlert bool) bool {
	hooks, err := s.store.ListWebhooks(ctx)
	if err != nil || len(hooks) == 0 {
		return false
	}

	host, _ := os.Hostname()
	title := fmt.Sprintf("[FikPanel · %s %s] %s", strings.ToUpper(source), eventTag, host)
	fullMsg := fmt.Sprintf("%s\n%s\nWaktu: %s", title, detail, time.Now().Format("2006-01-02 15:04:05 MST"))

	color := 0x10B981
	if isAlert {
		color = 0xF43F5E
	}

	sentAny := false
	reqCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 6 * time.Second}

	for _, h := range hooks {
		if source == "cpu" && !h.NotifyOnHighCPU {
			continue
		}
		if source == "ram" && !h.NotifyOnHighRAM {
			continue
		}
		targetURL, payload, err := buildAlertWebhookPayload(&h, title, fullMsg, detail, eventTag, color, s.Version())
		if err != nil {
			continue
		}
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, targetURL, bytes.NewReader(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "FikPanel-MonitorAlert/"+s.Version())
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				sentAny = true
			}
		}
	}
	return sentAny
}

// computeMonitorSummary menghitung analitik periode (Avg, Max, P95, Total Transfer, Peak I/O) dari data mentah
func computeMonitorSummary(raw []store.MonitorSnapshot, returnedCount int) MonitorHistorySummary {
	sum := MonitorHistorySummary{
		RawCount:      len(raw),
		ReturnedCount: returnedCount,
	}
	if len(raw) == 0 {
		return sum
	}

	cpuVals := make([]float64, 0, len(raw))
	var cpuSum, ioWaitSum, ramPctSum float64

	for i, s := range raw {
		cpuVals = append(cpuVals, s.CPU)
		cpuSum += s.CPU
		ioWaitSum += s.CPUIOWait
		if s.CPU > sum.CPUMax {
			sum.CPUMax = s.CPU
		}
		if s.Load1m > sum.LoadMax {
			sum.LoadMax = s.Load1m
		}
		ramPct := 0.0
		if s.RAMTotal > 0 {
			ramPct = (float64(s.RAMUsed) / float64(s.RAMTotal)) * 100.0
		}
		ramPctSum += ramPct
		if ramPct > sum.RAMMaxPct {
			sum.RAMMaxPct = ramPct
		}
		if s.NetIn > sum.NetInPeak {
			sum.NetInPeak = s.NetIn
		}
		if s.NetOut > sum.NetOutPeak {
			sum.NetOutPeak = s.NetOut
		}
		if s.DiskRead > sum.DiskReadPeak {
			sum.DiskReadPeak = s.DiskRead
		}
		if s.DiskWrite > sum.DiskWritePeak {
			sum.DiskWritePeak = s.DiskWrite
		}

		// Integrasi volume transfer jaringan berdasarkan selisih waktu antarsampel (dibatasi maks 300 detik per celah)
		var dt int64 = 60
		if i > 0 {
			diff := s.Timestamp - raw[i-1].Timestamp
			if diff > 0 && diff <= 300 {
				dt = diff
			}
		}
		sum.NetInTotalBytes += s.NetIn * uint64(dt)
		sum.NetOutTotalBytes += s.NetOut * uint64(dt)
	}

	n := float64(len(raw))
	sum.CPUAvg = math.Round((cpuSum/n)*10) / 10
	sum.CPUMax = math.Round(sum.CPUMax*10) / 10
	sum.CPUIOWaitAvg = math.Round((ioWaitSum/n)*10) / 10
	sum.RAMAvgPct = math.Round((ramPctSum/n)*10) / 10
	sum.RAMMaxPct = math.Round(sum.RAMMaxPct*10) / 10
	sum.LoadMax = math.Round(sum.LoadMax*100) / 100

	sort.Float64s(cpuVals)
	idx95 := int(math.Ceil(float64(len(cpuVals))*0.95)) - 1
	if idx95 < 0 {
		idx95 = 0
	} else if idx95 >= len(cpuVals) {
		idx95 = len(cpuVals) - 1
	}
	sum.CPUP95 = math.Round(cpuVals[idx95]*10) / 10

	last := raw[len(raw)-1]
	if last.DiskTotal > 0 {
		sum.DiskLatestPct = math.Round(((float64(last.DiskUsed)/float64(last.DiskTotal))*100.0)*10) / 10
	}
	return sum
}

// downsampleMonitorSnapshots merangkum ribuan baris menjadi maksimal maxPoints bucket dengan tetap menjaga lonjakan puncak
func downsampleMonitorSnapshots(raw []store.MonitorSnapshot, maxPoints int) []store.MonitorSnapshot {
	if maxPoints <= 0 || len(raw) <= maxPoints {
		return raw
	}
	out := make([]store.MonitorSnapshot, 0, maxPoints)
	bucketSize := float64(len(raw)) / float64(maxPoints)

	for b := 0; b < maxPoints; b++ {
		startIdx := int(math.Floor(float64(b) * bucketSize))
		endIdx := int(math.Floor(float64(b+1) * bucketSize))
		if endIdx <= startIdx {
			endIdx = startIdx + 1
		}
		if endIdx > len(raw) {
			endIdx = len(raw)
		}
		if startIdx >= len(raw) {
			break
		}

		slice := raw[startIdx:endIdx]
		agg := slice[len(slice)/2]
		var cpuPeak, ioWaitSum, stealMax, l1Max, l5Sum, l15Sum float64
		var ramMax, diskUsedMax, netInMax, netOutMax, diskRdMax, diskWrMax uint64
		for _, s := range slice {
			if s.CPU > cpuPeak {
				cpuPeak = s.CPU
			}
			ioWaitSum += s.CPUIOWait
			if s.CPUSteal > stealMax {
				stealMax = s.CPUSteal
			}
			if s.RAMUsed > ramMax {
				ramMax = s.RAMUsed
			}
			if s.Load1m > l1Max {
				l1Max = s.Load1m
			}
			l5Sum += s.Load5m
			l15Sum += s.Load15m
			if s.DiskUsed > diskUsedMax {
				diskUsedMax = s.DiskUsed
			}
			if s.NetIn > netInMax {
				netInMax = s.NetIn
			}
			if s.NetOut > netOutMax {
				netOutMax = s.NetOut
			}
			if s.DiskRead > diskRdMax {
				diskRdMax = s.DiskRead
			}
			if s.DiskWrite > diskWrMax {
				diskWrMax = s.DiskWrite
			}
		}
		cnt := float64(len(slice))
		agg.CPU = math.Round(cpuPeak*10) / 10
		agg.CPUIOWait = math.Round((ioWaitSum/cnt)*10) / 10
		agg.CPUSteal = math.Round(stealMax*10) / 10
		agg.RAMUsed = ramMax
		agg.Load1m = math.Round(l1Max*100) / 100
		agg.Load5m = math.Round((l5Sum/cnt)*100) / 100
		agg.Load15m = math.Round((l15Sum/cnt)*100) / 100
		agg.DiskUsed = diskUsedMax
		agg.NetIn = netInMax
		agg.NetOut = netOutMax
		agg.DiskRead = diskRdMax
		agg.DiskWrite = diskWrMax
		out = append(out, agg)
	}
	return out
}

// ── HTTP Handlers Monitor Histori, Export CSV, dan Alert/Insiden ────────────

func (s *Server) handleGetMonitorConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetMonitorConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mengambil konfigurasi monitoring"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "config": cfg})
}

func (s *Server) handleUpdateMonitorConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled  bool `json:"enabled"`
		SaveDays int  `json:"save_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if req.SaveDays < 1 {
		req.SaveDays = 30
	} else if req.SaveDays > 365 {
		req.SaveDays = 365
	}

	if err := s.store.UpdateMonitorConfig(r.Context(), req.Enabled, req.SaveDays); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memperbarui konfigurasi monitoring"})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "monitor.config_update", fmt.Sprintf("enabled=%v,days=%d", req.Enabled, req.SaveDays), true, "Memperbarui konfigurasi perekaman monitor", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"config":  store.MonitorConfig{Enabled: req.Enabled, SaveDays: req.SaveDays},
		"message": "Konfigurasi monitoring berhasil diperbarui",
	})
}

func (s *Server) handleGetMonitorHistory(w http.ResponseWriter, r *http.Request) {
	now := time.Now().Unix()
	q := r.URL.Query()
	startTS := now - 86400
	endTS := now
	maxPoints := 360

	if sVal, err := strconv.ParseInt(q.Get("start"), 10, 64); err == nil && sVal > 0 {
		startTS = sVal
	}
	if eVal, err := strconv.ParseInt(q.Get("end"), 10, 64); err == nil && eVal > 0 {
		endTS = eVal
	}
	if mp, err := strconv.Atoi(q.Get("max_points")); err == nil && mp >= 30 && mp <= 1500 {
		maxPoints = mp
	}

	raw, err := s.store.GetMonitorHistory(r.Context(), startTS, endTS)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mengambil histori metrik"})
		return
	}

	sampled := downsampleMonitorSnapshots(raw, maxPoints)
	summary := computeMonitorSummary(raw, len(sampled))
	prefs := s.loadPanelPreferences(r.Context())

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"history": sampled,
		"summary": summary,
		"thresholds": map[string]int{
			"alert_cpu_pct":      prefs.AlertCPUPct,
			"alert_ram_pct":      prefs.AlertRAMPct,
			"alert_disk_pct":     prefs.AlertDiskPct,
			"alert_ssl_days":     prefs.AlertSSLDays,
			"alert_cooldown_min": prefs.AlertCooldownMin,
		},
	})
}

// handleExportMonitorHistoryCSV mengekspor data historis mentah pada rentang waktu terpilih ke berkas CSV siap unduh
func (s *Server) handleExportMonitorHistoryCSV(w http.ResponseWriter, r *http.Request) {
	now := time.Now().Unix()
	q := r.URL.Query()
	startTS := now - 86400
	endTS := now

	if sVal, err := strconv.ParseInt(q.Get("start"), 10, 64); err == nil && sVal > 0 {
		startTS = sVal
	}
	if eVal, err := strconv.ParseInt(q.Get("end"), 10, 64); err == nil && eVal > 0 {
		endTS = eVal
	}

	raw, err := s.store.GetMonitorHistory(r.Context(), startTS, endTS)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mengekspor data histori"})
		return
	}

	filename := fmt.Sprintf("fikpanel-monitor-%s.csv", time.Now().Format("20060102-1504"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-store")

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"timestamp", "iso_time", "cpu_pct", "cpu_iowait_pct", "cpu_steal_pct",
		"ram_used_bytes", "ram_total_bytes", "load_1m", "load_5m", "load_15m",
		"disk_used_bytes", "disk_total_bytes", "disk_read_bps", "disk_write_bps",
		"net_in_bps", "net_out_bps",
	})
	for _, sn := range raw {
		_ = cw.Write([]string{
			strconv.FormatInt(sn.Timestamp, 10),
			time.Unix(sn.Timestamp, 0).Format(time.RFC3339),
			fmt.Sprintf("%.1f", sn.CPU),
			fmt.Sprintf("%.1f", sn.CPUIOWait),
			fmt.Sprintf("%.1f", sn.CPUSteal),
			strconv.FormatUint(sn.RAMUsed, 10),
			strconv.FormatUint(sn.RAMTotal, 10),
			fmt.Sprintf("%.2f", sn.Load1m),
			fmt.Sprintf("%.2f", sn.Load5m),
			fmt.Sprintf("%.2f", sn.Load15m),
			strconv.FormatUint(sn.DiskUsed, 10),
			strconv.FormatUint(sn.DiskTotal, 10),
			strconv.FormatUint(sn.DiskRead, 10),
			strconv.FormatUint(sn.DiskWrite, 10),
			strconv.FormatUint(sn.NetIn, 10),
			strconv.FormatUint(sn.NetOut, 10),
		})
	}
	cw.Flush()
}

func (s *Server) handleClearMonitorHistory(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ClearAllMonitorSnapshots(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membersihkan rekaman histori"})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "monitor.history_clear", "monitor_snapshots", true, "Mengosongkan seluruh rekaman histori monitor", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Seluruh rekaman histori monitoring berhasil dikosongkan",
	})
}

// handleListMonitorAlerts mengembalikan riwayat alert resource & insiden uptime/SSL beserta konfigurasi ambang batas
func (s *Server) handleListMonitorAlerts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 80
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 {
		limit = l
	}
	source := q.Get("source")
	onlyUnacked := q.Get("unacked") == "1" || q.Get("unacked") == "true"

	alerts, err := s.store.ListMonitorAlerts(r.Context(), limit, source, onlyUnacked)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mengambil daftar alert"})
		return
	}
	unacked := s.store.CountUnackedMonitorAlerts(r.Context())
	prefs := s.loadPanelPreferences(r.Context())
	hooks, _ := s.store.ListWebhooks(r.Context())

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"alerts":         alerts,
		"unacked_count":  unacked,
		"webhooks_count": len(hooks),
		"thresholds": map[string]int{
			"alert_cpu_pct":      prefs.AlertCPUPct,
			"alert_ram_pct":      prefs.AlertRAMPct,
			"alert_disk_pct":     prefs.AlertDiskPct,
			"alert_ssl_days":     prefs.AlertSSLDays,
			"alert_cooldown_min": prefs.AlertCooldownMin,
		},
	})
}

// handleUpdateAlertThresholds menyimpan ambang batas peringatan langsung dari halaman Monitor (tersinkron dengan Settings)
func (s *Server) handleUpdateAlertThresholds(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AlertCPUPct      int `json:"alert_cpu_pct"`
		AlertRAMPct      int `json:"alert_ram_pct"`
		AlertDiskPct     int `json:"alert_disk_pct"`
		AlertSSLDays     int `json:"alert_ssl_days"`
		AlertCooldownMin int `json:"alert_cooldown_min"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}

	prefs := s.loadPanelPreferences(r.Context())
	prefs.AlertCPUPct = clampInt(req.AlertCPUPct, 50, 99, prefs.AlertCPUPct)
	prefs.AlertRAMPct = clampInt(req.AlertRAMPct, 50, 99, prefs.AlertRAMPct)
	prefs.AlertDiskPct = clampInt(req.AlertDiskPct, 50, 99, prefs.AlertDiskPct)
	prefs.AlertSSLDays = clampInt(req.AlertSSLDays, 1, 90, prefs.AlertSSLDays)
	prefs.AlertCooldownMin = clampInt(req.AlertCooldownMin, 1, 1440, prefs.AlertCooldownMin)

	if err := s.savePanelPreferences(r.Context(), prefs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menyimpan ambang batas alert"})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "monitor.thresholds_update",
		fmt.Sprintf("cpu=%d,ram=%d,disk=%d,ssl=%d,cd=%d", prefs.AlertCPUPct, prefs.AlertRAMPct, prefs.AlertDiskPct, prefs.AlertSSLDays, prefs.AlertCooldownMin),
		true, "Memperbarui ambang batas alert monitor", s.callerUserID(r))

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Ambang batas alert berhasil disimpan dan langsung aktif",
		"thresholds": map[string]int{
			"alert_cpu_pct":      prefs.AlertCPUPct,
			"alert_ram_pct":      prefs.AlertRAMPct,
			"alert_disk_pct":     prefs.AlertDiskPct,
			"alert_ssl_days":     prefs.AlertSSLDays,
			"alert_cooldown_min": prefs.AlertCooldownMin,
		},
	})
}

func (s *Server) handleAckMonitorAlert(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID alert tidak valid"})
		return
	}
	if err := s.store.AcknowledgeMonitorAlert(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menandai alert"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAckAllMonitorAlerts(w http.ResponseWriter, r *http.Request) {
	if err := s.store.AcknowledgeAllMonitorAlerts(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menandai seluruh alert"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Seluruh alert ditandai sudah dibaca"})
}

func (s *Server) handleClearMonitorAlerts(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ClearMonitorAlerts(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menghapus riwayat alert"})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "monitor.alerts_clear", "monitor_alerts", true, "Mengosongkan riwayat alert & insiden", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Riwayat alert & insiden berhasil dikosongkan"})
}
