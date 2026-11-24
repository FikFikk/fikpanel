package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/cron"
	"fikpanel/internal/store"
)

// CreateCronJobRequest adalah payload JSON pembuatan/perubahan tugas cron
type CreateCronJobRequest struct {
	Name         string `json:"name"`
	ScheduleType string `json:"schedule_type"`
	Spec         string `json:"spec"`
	Command      string `json:"command"`
	WorkingDir   string `json:"working_dir"`
	SaveLog      *bool  `json:"save_log"`
	NotifyOnFail *bool  `json:"notify_on_fail"`
	LogFile      string `json:"log_file"`
	TimeoutSec   int    `json:"timeout_sec"`
}

// enrichCronJobRuntime mengisi metadata runtime (next_run_at dan is_running) pada objek CronJob
func (s *Server) enrichCronJobRuntime(job *store.CronJob, now time.Time) {
	if job == nil {
		return
	}
	if s.cronMgr != nil {
		job.IsRunning = s.cronMgr.IsRunning(job.ID)
	}
	if job.Enabled {
		if next, err := cron.NextRunAt(job.Spec, now); err == nil {
			job.NextRunAt = next.Unix()
		}
	}
}

// notifyCronFailure mengirim notifikasi alert ke seluruh webhook aktif bila tugas cron gagal atau melampaui timeout
func (s *Server) notifyCronFailure(job *store.CronJob, logEntry *store.CronLog) {
	if s.store == nil || job == nil || logEntry == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hooks, err := s.store.ListWebhooks(ctx)
	if err != nil || len(hooks) == 0 {
		return
	}

	host, _ := os.Hostname()
	eventTag := "CRON FAILED"
	if logEntry.Status == "timeout" {
		eventTag = "CRON TIMEOUT"
	}

	snippet := strings.TrimSpace(logEntry.Output)
	if len(snippet) > 400 {
		snippet = "..." + snippet[len(snippet)-400:]
	}
	if snippet == "" {
		snippet = "(tidak ada output teks)"
	}

	title := fmt.Sprintf("[FikPanel · %s] %s", eventTag, job.Name)
	detail := fmt.Sprintf("Jadwal: `%s`\nStatus: %s (Exit Code: %d, Durasi: %d ms)\nOutput terakhir:\n%s",
		job.Spec, strings.ToUpper(logEntry.Status), logEntry.ExitCode, logEntry.DurationMs, snippet)
	fullMsg := fmt.Sprintf("%s\n%s\nHost: %s · %s", title, detail, host, time.Now().Format("2006-01-02 15:04:05 MST"))

	client := &http.Client{Timeout: 8 * time.Second}
	for _, h := range hooks {
		targetURL, payload, err := buildAlertWebhookPayload(&h, title, fullMsg, detail, eventTag, 0xF43F5E, s.Version())
		if err != nil {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "FikPanel-CronAlert/"+s.Version())
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
			_ = resp.Body.Close()
		}
	}
}

// validateCronJobInput menormalisasi dan memvalidasi seluruh field input tugas cron
func validateCronJobInput(req *CreateCronJobRequest) (string, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Command = strings.TrimSpace(req.Command)
	req.WorkingDir = strings.TrimSpace(req.WorkingDir)
	req.LogFile = strings.TrimSpace(req.LogFile)

	if req.Name == "" {
		return "", fmt.Errorf("nama tugas wajib diisi")
	}
	if len(req.Name) > 120 {
		return "", fmt.Errorf("nama tugas maksimal 120 karakter")
	}
	if err := cron.ValidateCommand(req.Command); err != nil {
		return "", err
	}
	normSpec := cron.NormalizeSpec(req.Spec)
	if err := cron.ValidateSpec(normSpec); err != nil {
		return "", err
	}
	if err := cron.ValidateLogFilePath(req.LogFile); err != nil {
		return "", err
	}
	if err := cron.ValidateWorkingDir(req.WorkingDir); err != nil {
		return "", err
	}
	if req.TimeoutSec <= 0 {
		req.TimeoutSec = 3600
	} else if req.TimeoutSec < 10 {
		req.TimeoutSec = 10
	} else if req.TimeoutSec > 86400 {
		req.TimeoutSec = 86400
	}
	if strings.TrimSpace(req.ScheduleType) == "" {
		req.ScheduleType = "custom"
	}
	return normSpec, nil
}

func (s *Server) handleListCronJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.ListCronJobs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mengambil daftar cron job"})
		return
	}

	now := time.Now()
	for i := range jobs {
		s.enrichCronJobRuntime(&jobs[i], now)
	}
	zoneName, zoneOffset := now.Zone()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"jobs": jobs,
		"server_time": map[string]interface{}{
			"unix":          now.Unix(),
			"iso":           now.Format(time.RFC3339),
			"timezone_name": zoneName,
			"offset_sec":    zoneOffset,
		},
	})
}

// handleListSystemCrons mengembalikan daftar tugas cron sistem operasi (read-only) dari crontab & /etc/cron.d
func (s *Server) handleListSystemCrons(w http.ResponseWriter, r *http.Request) {
	entries := cron.ListSystemCrons(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"entries": entries,
	})
}

func (s *Server) handleCreateCronJob(w http.ResponseWriter, r *http.Request) {
	var req CreateCronJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}

	normSpec, err := validateCronJobInput(&req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	saveLog := true
	if req.SaveLog != nil {
		saveLog = *req.SaveLog
	}
	notifyOnFail := true
	if req.NotifyOnFail != nil {
		notifyOnFail = *req.NotifyOnFail
	}

	job := &store.CronJob{
		Name:         req.Name,
		ScheduleType: req.ScheduleType,
		Spec:         normSpec,
		Command:      req.Command,
		WorkingDir:   req.WorkingDir,
		Enabled:      true,
		SaveLog:      saveLog,
		NotifyOnFail: notifyOnFail,
		LogFile:      req.LogFile,
		TimeoutSec:   req.TimeoutSec,
	}

	id, err := s.store.CreateCronJob(r.Context(), job)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menyimpan tugas cron"})
		return
	}
	job.ID = id
	s.enrichCronJobRuntime(job, time.Now())

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "cron.create", req.Name, true, fmt.Sprintf("spec=%s", normSpec), uid)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"ok":  true,
		"job": job,
	})
}

// handleUpdateCronJob memperbarui data konfigurasi tugas cron yang sudah ada setelah validasi ketat
func (s *Server) handleUpdateCronJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID tugas tidak valid"})
		return
	}

	existing, err := s.store.GetCronJob(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mencari data cron job"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tugas cron tidak ditemukan"})
		return
	}

	var req CreateCronJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}

	normSpec, err := validateCronJobInput(&req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	existing.Name = req.Name
	existing.ScheduleType = req.ScheduleType
	existing.Spec = normSpec
	existing.Command = req.Command
	existing.WorkingDir = req.WorkingDir
	if req.SaveLog != nil {
		existing.SaveLog = *req.SaveLog
	}
	if req.NotifyOnFail != nil {
		existing.NotifyOnFail = *req.NotifyOnFail
	}
	existing.LogFile = req.LogFile
	existing.TimeoutSec = req.TimeoutSec

	if err := s.store.UpdateCronJob(r.Context(), existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal memperbarui tugas cron"})
		return
	}
	s.enrichCronJobRuntime(existing, time.Now())

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "cron.update", req.Name, true, fmt.Sprintf("spec=%s", normSpec), uid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":  true,
		"job": existing,
	})
}

func (s *Server) handleToggleCronJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID tugas tidak valid"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload JSON tidak valid"})
		return
	}

	if err := s.store.ToggleCronJob(r.Context(), id, req.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mengubah status aktif cron"})
		return
	}

	var nextRunAt int64
	if req.Enabled {
		if job, err := s.store.GetCronJob(r.Context(), id); err == nil && job != nil {
			if next, err := cron.NextRunAt(job.Spec, time.Now()); err == nil {
				nextRunAt = next.Unix()
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"id":          id,
		"enabled":     req.Enabled,
		"next_run_at": nextRunAt,
	})
}

func (s *Server) handleDeleteCronJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID tugas tidak valid"})
		return
	}

	// Hentikan dulu bila tugas sedang berjalan sebelum dihapus
	if s.cronMgr != nil && s.cronMgr.IsRunning(id) {
		_ = s.cronMgr.StopJob(id)
	}

	if err := s.store.DeleteCronJob(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menghapus tugas cron"})
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "cron.delete", idStr, true, "Menghapus tugas cron", uid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"id": id,
	})
}

// handleRunCronJobNow memicu eksekusi manual secara terlepas (detached) agar tidak terputus saat pengguna berpindah menu
func (s *Server) handleRunCronJobNow(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID tugas tidak valid"})
		return
	}

	if s.cronMgr == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cron manager belum diinisialisasi"})
		return
	}

	logEntry, stillRunning, err := s.cronMgr.TriggerJobAsync(s.bgCtx, id, 2500*time.Millisecond)
	if err != nil && logEntry == nil {
		status := http.StatusInternalServerError
		if stillRunning {
			status = http.StatusConflict
		} else if strings.Contains(err.Error(), "tidak ditemukan") {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "cron.run", idStr, true, fmt.Sprintf("running=%v", stillRunning), uid)

	if stillRunning {
		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"ok":      true,
			"running": true,
			"message": "Tugas sedang berjalan di latar belakang; status akan diperbarui otomatis.",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"running": false,
		"log":     logEntry,
	})
}

// handleStopCronJob menghentikan tugas cron yang sedang aktif berjalan beserta seluruh process group-nya
func (s *Server) handleStopCronJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID tugas tidak valid"})
		return
	}
	if s.cronMgr == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cron manager belum diinisialisasi"})
		return
	}
	if err := s.cronMgr.StopJob(id); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "cron.stop", idStr, true, "Menghentikan paksa tugas cron berjalan", uid)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"id":      id,
		"message": "Sinyal henti telah dikirim ke seluruh grup proses tugas cron",
	})
}

func (s *Server) handleListCronLogs(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID tugas tidak valid"})
		return
	}

	logs, err := s.store.ListCronLogs(r.Context(), id, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal mengambil log cron"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"logs": logs,
	})
}

// handleClearCronDBLogs membersihkan riwayat eksekusi SQLite untuk satu tugas cron
func (s *Server) handleClearCronDBLogs(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID tugas tidak valid"})
		return
	}
	if err := s.store.ClearCronLogs(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membersihkan riwayat log database"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": "Riwayat eksekusi database berhasil dibersihkan",
	})
}

func (s *Server) handleGetCronJobFileLog(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID tugas tidak valid"})
		return
	}

	if s.cronMgr == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cron manager belum diinisialisasi"})
		return
	}

	content, path, err := s.cronMgr.ReadJobLogFile(r.Context(), id, 256*1024)
	if err != nil && content == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      false,
			"path":    path,
			"message": err.Error(),
			"content": "",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"path":    path,
		"content": content,
	})
}

func (s *Server) handleClearCronJobFileLog(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID tugas tidak valid"})
		return
	}

	if s.cronMgr == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cron manager belum diinisialisasi"})
		return
	}

	path, err := s.cronMgr.ClearJobLogFile(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gagal membersihkan berkas log: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"path":    path,
		"message": "Berkas log berhasil dikosongkan",
	})
}
