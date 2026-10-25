package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"fikpanel/internal/store"
)

type CreateCronJobRequest struct {
	Name         string `json:"name"`
	ScheduleType string `json:"schedule_type"`
	Spec         string `json:"spec"`
	Command      string `json:"command"`
	SaveLog      bool   `json:"save_log"`
	TimeoutSec   int    `json:"timeout_sec"`
}

func (s *Server) handleListCronJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.ListCronJobs(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil daftar cron job"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"jobs": jobs,
	})
}

func (s *Server) handleCreateCronJob(w http.ResponseWriter, r *http.Request) {
	var req CreateCronJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Command == "" {
		http.Error(w, `{"error":"nama dan perintah eksekusi wajib diisi"}`, http.StatusBadRequest)
		return
	}

	job := &store.CronJob{
		Name:         req.Name,
		ScheduleType: req.ScheduleType,
		Spec:         req.Spec,
		Command:      req.Command,
		Enabled:      true,
		SaveLog:      req.SaveLog,
		TimeoutSec:   req.TimeoutSec,
	}

	id, err := s.store.CreateCronJob(r.Context(), job)
	if err != nil {
		http.Error(w, `{"error":"gagal menyimpan tugas cron"}`, http.StatusInternalServerError)
		return
	}
	job.ID = id

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "cron.create", req.Name, true, "Membuat tugas cron baru", uid)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":  true,
		"job": job,
	})
}

func (s *Server) handleToggleCronJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"ID tugas tidak valid"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.store.ToggleCronJob(r.Context(), id, req.Enabled); err != nil {
		http.Error(w, `{"error":"gagal mengubah status aktif cron"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"id":      id,
		"enabled": req.Enabled,
	})
}

func (s *Server) handleDeleteCronJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"ID tugas tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteCronJob(r.Context(), id); err != nil {
		http.Error(w, `{"error":"gagal menghapus tugas cron"}`, http.StatusInternalServerError)
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "cron.delete", idStr, true, "Menghapus tugas cron", uid)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
		"id": id,
	})
}

func (s *Server) handleRunCronJobNow(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"ID tugas tidak valid"}`, http.StatusBadRequest)
		return
	}

	if s.cronMgr == nil {
		http.Error(w, `{"error":"cron manager belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	logEntry, err := s.cronMgr.ExecuteJob(r.Context(), id)
	if err != nil && logEntry == nil {
		http.Error(w, `{"error":"gagal mengeksekusi tugas cron"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":  true,
		"log": logEntry,
	})
}

func (s *Server) handleListCronLogs(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"ID tugas tidak valid"}`, http.StatusBadRequest)
		return
	}

	logs, err := s.store.ListCronLogs(r.Context(), id, 30)
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil log cron"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"logs": logs,
	})
}
