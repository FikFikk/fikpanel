package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"fikpanel/internal/store"
)

// StartMonitoringRecorder menjalankan goroutine pencatat background
func (s *Server) StartMonitoringRecorder(ctx context.Context) {
	go func() {
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
				cfg, err := s.store.GetMonitorConfig(ctx)
				if err != nil || !cfg.Enabled {
					// Jika status monitoring mati, lewati dan jangan catat apa pun ke database
					continue
				}

				pt := s.metrics.GetLatest()
				if pt.Timestamp == 0 {
					continue
				}

				snap := &store.MonitorSnapshot{
					Timestamp: pt.Timestamp,
					CPU:       pt.CPU,
					RAMUsed:   pt.RAMUsed,
					RAMTotal:  pt.RAMTotal,
					Load1m:    pt.LoadAvg,
					Load5m:    pt.Load5m,
					Load15m:   pt.Load15m,
					DiskUsed:  pt.DiskUsed,
					DiskTotal: pt.DiskTotal,
					NetIn:     pt.NetIn,
					NetOut:    pt.NetOut,
				}
				_ = s.store.InsertMonitorSnapshot(ctx, snap)
			}
		}
	}()
}

func (s *Server) handleGetMonitorConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetMonitorConfig(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil konfigurasi monitoring"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "config": cfg})
}

func (s *Server) handleUpdateMonitorConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled  bool `json:"enabled"`
		SaveDays int  `json:"save_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.store.UpdateMonitorConfig(r.Context(), req.Enabled, req.SaveDays); err != nil {
		http.Error(w, `{"error":"gagal memperbarui konfigurasi monitoring"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Konfigurasi status monitoring berhasil diperbarui"})
}

func (s *Server) handleGetMonitorHistory(w http.ResponseWriter, r *http.Request) {
	now := time.Now().Unix()
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	startTS := now - 3600 // default 1 jam terakhir
	endTS := now

	if sVal, err := strconv.ParseInt(startStr, 10, 64); err == nil && sVal > 0 {
		startTS = sVal
	}
	if eVal, err := strconv.ParseInt(endStr, 10, 64); err == nil && eVal > 0 {
		endTS = eVal
	}

	history, err := s.store.GetMonitorHistory(r.Context(), startTS, endTS)
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil histori metrik"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"history": history,
	})
}

func (s *Server) handleClearMonitorHistory(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ClearAllMonitorSnapshots(r.Context()); err != nil {
		http.Error(w, `{"error":"gagal membersihkan rekaman histori"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Seluruh rekaman histori monitoring berhasil dikosongkan",
	})
}
