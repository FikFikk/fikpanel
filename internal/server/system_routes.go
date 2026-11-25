package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"fikpanel/internal/system"
)

// handleGetSystemOverview menyajikan telemetri perangkat keras, beban CPU, RAM/Swap, proses, dan partisi disk+inode
func (s *Server) handleGetSystemOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := s.systemMgr.GetHostOverview(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memuat ringkasan host: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "overview": ov})
}

// handleAnalyzeDiskHogs memindai direktori terbesar pada root target yang diizinkan
func (s *Server) handleAnalyzeDiskHogs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	res, err := s.systemMgr.AnalyzeDiskHogs(r.Context(), req.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": res})
}

// handleRebootHost menjadwalkan reboot server secara aman setelah memverifikasi kecocokan hostname
func (s *Server) handleRebootHost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConfirmHostname string `json:"confirm_hostname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.systemMgr.RebootHost(r.Context(), req.ConfirmHostname); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.reboot", req.ConfirmHostname, true, "Menjadwalkan reboot host server", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Server akan reboot dalam 2 detik"})
}

// handleListSystemProcesses mengembalikan daftar proses aktual beserta dukungan sorting, pencarian, dan filter
func (s *Server) handleListSystemProcesses(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 60
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 {
		limit = l
	}
	sortBy := q.Get("sort")
	search := q.Get("search")
	filter := q.Get("filter")

	procs, err := s.systemMgr.ListProcesses(r.Context(), limit, sortBy, search, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mengambil daftar proses: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "processes": procs})
}

// handleInspectSystemProcess membaca detail /proc/<pid> untuk satu proses target
func (s *Server) handleInspectSystemProcess(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.Atoi(r.PathValue("pid"))
	if err != nil || pid <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "PID tidak valid"})
		return
	}
	detail, err := s.systemMgr.InspectProcess(r.Context(), pid)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "process": detail})
}

// handleKillSystemProcess mengirim sinyal TERM/KILL/HUP ke proses non-vital dengan audit log
func (s *Server) handleKillSystemProcess(w http.ResponseWriter, r *http.Request) {
	pidStr := r.PathValue("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "PID tidak valid"})
		return
	}

	signal := strings.TrimSpace(r.URL.Query().Get("signal"))
	if signal == "" {
		if r.URL.Query().Get("force") == "true" {
			signal = "KILL"
		} else {
			signal = "TERM"
		}
	}

	if err := s.systemMgr.KillProcess(r.Context(), pid, signal); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.process_kill", pidStr, true, fmt.Sprintf("Mengirim sinyal %s ke proses #%d", signal, pid), s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pid": pid, "signal": signal})
}

// handleReniceSystemProcess mengubah prioritas CPU (nice) satu proses secara aman
func (s *Server) handleReniceSystemProcess(w http.ResponseWriter, r *http.Request) {
	pidStr := r.PathValue("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "PID tidak valid"})
		return
	}
	var req struct {
		Priority int `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.systemMgr.ReniceProcess(r.Context(), pid, req.Priority); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.process_renice", pidStr, true, fmt.Sprintf("Mengubah nice proses #%d menjadi %d", pid, req.Priority), s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pid": pid, "priority": req.Priority})
}

// handleListListeningPorts mengembalikan daftar soket listening aktual sekaligus ringkasan state koneksi TCP
func (s *Server) handleListListeningPorts(w http.ResponseWriter, r *http.Request) {
	netOv, err := s.systemMgr.GetNetworkOverview(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memindai jaringan host: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"is_linux":    netOv.IsLinux,
		"ports":       netOv.Ports,
		"connections": netOv.Connections,
	})
}

// ── Swap & Memory Optimizer ──────────────────────────────────────────────────

// handleGetMemoryStatus mengembalikan status RAM, swap aktif, dan nilai live vm.swappiness/vfs_cache_pressure
func (s *Server) handleGetMemoryStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.systemMgr.GetMemoryOptimizationStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": st})
}

// handleCreateSwapfile membuat atau mengubah ukuran swapfile /swapfile.fikpanel secara persisten di /etc/fstab
func (s *Server) handleCreateSwapfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SizeMB int `json:"size_mb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.systemMgr.CreateSwapfile(r.Context(), req.SizeMB); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.swap_create", strconv.Itoa(req.SizeMB)+"MB", true, fmt.Sprintf("Membuat swapfile %d MB", req.SizeMB), s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": fmt.Sprintf("Swapfile %d MB berhasil diaktifkan dan disimpan ke /etc/fstab", req.SizeMB)})
}

// handleRemoveSwapfile menonaktifkan dan menghapus /swapfile.fikpanel
func (s *Server) handleRemoveSwapfile(w http.ResponseWriter, r *http.Request) {
	if err := s.systemMgr.RemoveSwapfile(r.Context()); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.swap_remove", "/swapfile.fikpanel", true, "Menghapus swapfile FikPanel", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Swapfile FikPanel berhasil dinonaktifkan dan dihapus"})
}

// handleSetMemoryTuning memperbarui vm.swappiness dan vm.vfs_cache_pressure secara live dan persisten
func (s *Server) handleSetMemoryTuning(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Swappiness       int `json:"swappiness"`
		VFSCachePressure int `json:"vfs_cache_pressure"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	if err := s.systemMgr.SetMemoryTuning(r.Context(), req.Swappiness, req.VFSCachePressure); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.memory_tuning", fmt.Sprintf("swappiness=%d,vfs=%d", req.Swappiness, req.VFSCachePressure), true, "Memperbarui tuning memori kernel", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Tuning memori kernel berhasil diterapkan secara persisten"})
}

// handleDropRAMCaches melakukan sync disk dan mengosongkan RAM page cache serta melaporkan byte yang dibebaskan
func (s *Server) handleDropRAMCaches(w http.ResponseWriter, r *http.Request) {
	freed, availAfter, err := s.systemMgr.DropRAMCaches(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.drop_caches", strconv.FormatInt(freed, 10), true, "Mengosongkan RAM page cache kernel", s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"freed_bytes": freed,
		"avail_after": availAfter,
	})
}

// ── Smart Disk Cleaner, OS Updates & Kernel Diagnostics ─────────────────────

// handleScanSystemJunk memindai 6 kategori sampah nyata host (APT, Journald, Log, Temp >3d, Dev Cache, Docker)
func (s *Server) handleScanSystemJunk(w http.ResponseWriter, r *http.Request) {
	result, err := s.systemMgr.ScanJunk(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memindai file sampah: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "scan": result})
}

// handleCleanSystemJunk mengeksekusi pembersihan kategori terpilih dan mengembalikan laporan byte yang dibebaskan
func (s *Server) handleCleanSystemJunk(w http.ResponseWriter, r *http.Request) {
	var req system.CleanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}
	report, err := s.systemMgr.CleanJunk(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.cleaner_run", strconv.FormatInt(report.ReclaimedBytes, 10), true, strings.Join(report.Actions, "; "), s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"report":  report,
		"message": "Pembersihan sistem berhasil diselesaikan",
	})
}

// handleGetSystemUpdates mengembalikan daftar paket OS yang dapat diperbarui melalui APT
func (s *Server) handleGetSystemUpdates(w http.ResponseWriter, r *http.Request) {
	refresh := r.URL.Query().Get("refresh") == "1" || r.URL.Query().Get("refresh") == "true"
	ov, err := s.systemMgr.CheckUpdates(r.Context(), refresh)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updates": ov})
}

// handleApplySecurityUpdates menerapkan pembaruan paket keamanan OS secara non-interaktif
func (s *Server) handleApplySecurityUpdates(w http.ResponseWriter, r *http.Request) {
	msg, err := s.systemMgr.ApplySecurityUpdates(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "system.security_updates", "", true, msg, s.callerUserID(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

// handleGetSystemDiagnostics membaca log galat/peringatan kernel & systemd beserta deteksi insiden OOM Killer
func (s *Server) handleGetSystemDiagnostics(w http.ResponseWriter, r *http.Request) {
	rep, err := s.systemMgr.GetDiagnostics(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "diagnostics": rep})
}
