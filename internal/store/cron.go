package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CronJob merepresentasikan satu tugas terjadwal di database FikPanel
type CronJob struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	ScheduleType string `json:"schedule_type"`
	Spec         string `json:"spec"`
	Command      string `json:"command"`
	WorkingDir   string `json:"working_dir"`
	Enabled      bool   `json:"enabled"`
	SaveLog      bool   `json:"save_log"`
	NotifyOnFail bool   `json:"notify_on_fail"`
	LogFile      string `json:"log_file"` // Jalur berkas log kustom (misal: /var/cronlog/my-task.log)
	TimeoutSec   int    `json:"timeout_sec"`
	LastRunAt    int64  `json:"last_run_at"`
	LastStatus   string `json:"last_status"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`

	// Metadata runtime (dihitung saat API dipanggil, tidak disimpan di tabel)
	NextRunAt int64 `json:"next_run_at"`
	IsRunning bool  `json:"is_running"`
}

// CronLog menyimpan riwayat satu kali eksekusi tugas cron
type CronLog struct {
	ID         int64  `json:"id"`
	JobID      int64  `json:"job_id"`
	Status     string `json:"status"`
	Output     string `json:"output"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
}

const cronMaxLogsPerJob = 100

// ListCronJobs mengambil seluruh tugas cron yang terdaftar
func (s *Store) ListCronJobs(ctx context.Context) ([]CronJob, error) {
	query := `SELECT id, name, schedule_type, spec, command, coalesce(working_dir, ''), enabled, save_log,
	                 coalesce(notify_on_fail, 1), coalesce(log_file, ''), timeout_sec,
	                 coalesce(last_run_at, 0), coalesce(last_status, ''), created_at, updated_at
	          FROM cron_jobs ORDER BY id DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list cron jobs: %w", err)
	}
	defer rows.Close()

	var jobs []CronJob
	for rows.Next() {
		var j CronJob
		var enabled, saveLog, notifyOnFail int
		if err := rows.Scan(
			&j.ID, &j.Name, &j.ScheduleType, &j.Spec, &j.Command, &j.WorkingDir,
			&enabled, &saveLog, &notifyOnFail, &j.LogFile, &j.TimeoutSec,
			&j.LastRunAt, &j.LastStatus, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, err
		}
		j.Enabled = enabled == 1
		j.SaveLog = saveLog == 1
		j.NotifyOnFail = notifyOnFail == 1
		jobs = append(jobs, j)
	}
	if jobs == nil {
		jobs = []CronJob{}
	}
	return jobs, rows.Err()
}

// GetCronJob mengambil detail satu tugas cron berdasarkan ID
func (s *Store) GetCronJob(ctx context.Context, id int64) (*CronJob, error) {
	query := `SELECT id, name, schedule_type, spec, command, coalesce(working_dir, ''), enabled, save_log,
	                 coalesce(notify_on_fail, 1), coalesce(log_file, ''), timeout_sec,
	                 coalesce(last_run_at, 0), coalesce(last_status, ''), created_at, updated_at
	          FROM cron_jobs WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	var j CronJob
	var enabled, saveLog, notifyOnFail int
	if err := row.Scan(
		&j.ID, &j.Name, &j.ScheduleType, &j.Spec, &j.Command, &j.WorkingDir,
		&enabled, &saveLog, &notifyOnFail, &j.LogFile, &j.TimeoutSec,
		&j.LastRunAt, &j.LastStatus, &j.CreatedAt, &j.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	j.Enabled = enabled == 1
	j.SaveLog = saveLog == 1
	j.NotifyOnFail = notifyOnFail == 1
	return &j, nil
}

// CreateCronJob menyimpan tugas cron baru ke database
func (s *Store) CreateCronJob(ctx context.Context, j *CronJob) (int64, error) {
	now := time.Now().Unix()
	enabled := boolToInt(j.Enabled)
	saveLog := boolToInt(j.SaveLog)
	notifyOnFail := boolToInt(j.NotifyOnFail)
	if j.TimeoutSec <= 0 {
		j.TimeoutSec = 3600
	}

	query := `INSERT INTO cron_jobs (name, schedule_type, spec, command, working_dir, enabled, save_log, notify_on_fail, log_file, timeout_sec, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, j.Name, j.ScheduleType, j.Spec, j.Command, j.WorkingDir, enabled, saveLog, notifyOnFail, j.LogFile, j.TimeoutSec, now, now)
	if err != nil {
		return 0, fmt.Errorf("create cron job: %w", err)
	}
	j.CreatedAt = now
	j.UpdatedAt = now
	return res.LastInsertId()
}

// UpdateCronJob memperbarui seluruh konfigurasi tugas cron yang sudah ada
func (s *Store) UpdateCronJob(ctx context.Context, j *CronJob) error {
	now := time.Now().Unix()
	enabled := boolToInt(j.Enabled)
	saveLog := boolToInt(j.SaveLog)
	notifyOnFail := boolToInt(j.NotifyOnFail)

	query := `UPDATE cron_jobs
	          SET name = ?, schedule_type = ?, spec = ?, command = ?, working_dir = ?, enabled = ?, save_log = ?, notify_on_fail = ?, log_file = ?, timeout_sec = ?, updated_at = ?
	          WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, j.Name, j.ScheduleType, j.Spec, j.Command, j.WorkingDir, enabled, saveLog, notifyOnFail, j.LogFile, j.TimeoutSec, now, j.ID)
	if err == nil {
		j.UpdatedAt = now
	}
	return err
}

// DeleteCronJob menghapus tugas cron beserta seluruh riwayat lognya
func (s *Store) DeleteCronJob(ctx context.Context, id int64) error {
	_, _ = s.db.ExecContext(ctx, `DELETE FROM cron_logs WHERE job_id = ?`, id)
	_, err := s.db.ExecContext(ctx, `DELETE FROM cron_jobs WHERE id = ?`, id)
	return err
}

// ToggleCronJob mengubah status aktif atau jeda tugas cron
func (s *Store) ToggleCronJob(ctx context.Context, id int64, enabled bool) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE cron_jobs SET enabled = ?, updated_at = ? WHERE id = ?`, boolToInt(enabled), now, id)
	return err
}

// UpdateCronJobStatus memperbarui waktu dan status eksekusi terakhir tugas cron
func (s *Store) UpdateCronJobStatus(ctx context.Context, id int64, lastRun int64, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE cron_jobs SET last_run_at = ?, last_status = ? WHERE id = ?`, lastRun, status, id)
	return err
}

// RecoverStaleRunningCronJobs memulihkan status tugas yang tertinggal "running" akibat restart/update panel
func (s *Store) RecoverStaleRunningCronJobs(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE cron_jobs SET last_status = 'interrupted' WHERE last_status = 'running'`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RecordCronLog mencatat riwayat eksekusi dan memangkas otomatis baris lama agar maksimal 100 riwayat per tugas
func (s *Store) RecordCronLog(ctx context.Context, logEntry *CronLog) error {
	query := `INSERT INTO cron_logs (job_id, status, output, exit_code, duration_ms, started_at, finished_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, logEntry.JobID, logEntry.Status, logEntry.Output, logEntry.ExitCode, logEntry.DurationMs, logEntry.StartedAt, logEntry.FinishedAt)
	if err != nil {
		return err
	}
	if id, idErr := res.LastInsertId(); idErr == nil {
		logEntry.ID = id
	}
	// Pangkas otomatis riwayat berlebih per tugas agar database SQLite tetap ramping meski cron berjalan tiap menit
	_, _ = s.db.ExecContext(ctx, `
		DELETE FROM cron_logs
		WHERE job_id = ? AND id NOT IN (
			SELECT id FROM cron_logs WHERE job_id = ? ORDER BY started_at DESC, id DESC LIMIT ?
		)`, logEntry.JobID, logEntry.JobID, cronMaxLogsPerJob)
	return nil
}

// ClearCronLogs menghapus seluruh riwayat eksekusi database untuk satu tugas cron
func (s *Store) ClearCronLogs(ctx context.Context, jobID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cron_logs WHERE job_id = ?`, jobID)
	return err
}

// ListCronLogs mengambil daftar riwayat eksekusi terbaru untuk satu tugas cron
func (s *Store) ListCronLogs(ctx context.Context, jobID int64, limit int) ([]CronLog, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT id, job_id, status, coalesce(output, ''), exit_code, duration_ms, started_at, finished_at
	          FROM cron_logs WHERE job_id = ? ORDER BY started_at DESC, id DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []CronLog
	for rows.Next() {
		var l CronLog
		if err := rows.Scan(&l.ID, &l.JobID, &l.Status, &l.Output, &l.ExitCode, &l.DurationMs, &l.StartedAt, &l.FinishedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []CronLog{}
	}
	return logs, rows.Err()
}

