package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type CronJob struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	ScheduleType string `json:"schedule_type"`
	Spec         string `json:"spec"`
	Command      string `json:"command"`
	Enabled      bool   `json:"enabled"`
	SaveLog      bool   `json:"save_log"`
	TimeoutSec   int    `json:"timeout_sec"`
	LastRunAt    int64  `json:"last_run_at"`
	LastStatus   string `json:"last_status"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

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

func (s *Store) ListCronJobs(ctx context.Context) ([]CronJob, error) {
	query := `SELECT id, name, schedule_type, spec, command, enabled, save_log, timeout_sec, 
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
		var enabled, saveLog int
		if err := rows.Scan(
			&j.ID, &j.Name, &j.ScheduleType, &j.Spec, &j.Command,
			&enabled, &saveLog, &j.TimeoutSec,
			&j.LastRunAt, &j.LastStatus, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, err
		}
		j.Enabled = enabled == 1
		j.SaveLog = saveLog == 1
		jobs = append(jobs, j)
	}
	if jobs == nil {
		jobs = []CronJob{}
	}
	return jobs, nil
}

func (s *Store) GetCronJob(ctx context.Context, id int64) (*CronJob, error) {
	query := `SELECT id, name, schedule_type, spec, command, enabled, save_log, timeout_sec, 
	                 coalesce(last_run_at, 0), coalesce(last_status, ''), created_at, updated_at 
	          FROM cron_jobs WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	var j CronJob
	var enabled, saveLog int
	if err := row.Scan(
		&j.ID, &j.Name, &j.ScheduleType, &j.Spec, &j.Command,
		&enabled, &saveLog, &j.TimeoutSec,
		&j.LastRunAt, &j.LastStatus, &j.CreatedAt, &j.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	j.Enabled = enabled == 1
	j.SaveLog = saveLog == 1
	return &j, nil
}

func (s *Store) CreateCronJob(ctx context.Context, j *CronJob) (int64, error) {
	now := time.Now().Unix()
	enabled := 0
	if j.Enabled {
		enabled = 1
	}
	saveLog := 0
	if j.SaveLog {
		saveLog = 1
	}
	if j.TimeoutSec <= 0 {
		j.TimeoutSec = 3600
	}

	query := `INSERT INTO cron_jobs (name, schedule_type, spec, command, enabled, save_log, timeout_sec, created_at, updated_at) 
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, j.Name, j.ScheduleType, j.Spec, j.Command, enabled, saveLog, j.TimeoutSec, now, now)
	if err != nil {
		return 0, fmt.Errorf("create cron job: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) UpdateCronJob(ctx context.Context, j *CronJob) error {
	now := time.Now().Unix()
	enabled := 0
	if j.Enabled {
		enabled = 1
	}
	saveLog := 0
	if j.SaveLog {
		saveLog = 1
	}

	query := `UPDATE cron_jobs SET name = ?, schedule_type = ?, spec = ?, command = ?, enabled = ?, save_log = ?, timeout_sec = ?, updated_at = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, j.Name, j.ScheduleType, j.Spec, j.Command, enabled, saveLog, j.TimeoutSec, now, j.ID)
	return err
}

func (s *Store) DeleteCronJob(ctx context.Context, id int64) error {
	_, _ = s.db.ExecContext(ctx, `DELETE FROM cron_logs WHERE job_id = ?`, id)
	_, err := s.db.ExecContext(ctx, `DELETE FROM cron_jobs WHERE id = ?`, id)
	return err
}

func (s *Store) ToggleCronJob(ctx context.Context, id int64, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE cron_jobs SET enabled = ?, updated_at = ? WHERE id = ?`, en, now, id)
	return err
}

func (s *Store) UpdateCronJobStatus(ctx context.Context, id int64, lastRun int64, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE cron_jobs SET last_run_at = ?, last_status = ? WHERE id = ?`, lastRun, status, id)
	return err
}

func (s *Store) RecordCronLog(ctx context.Context, log *CronLog) error {
	query := `INSERT INTO cron_logs (job_id, status, output, exit_code, duration_ms, started_at, finished_at) 
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, log.JobID, log.Status, log.Output, log.ExitCode, log.DurationMs, log.StartedAt, log.FinishedAt)
	return err
}

func (s *Store) ListCronLogs(ctx context.Context, jobID int64, limit int) ([]CronLog, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT id, job_id, status, coalesce(output, ''), exit_code, duration_ms, started_at, finished_at 
	          FROM cron_logs WHERE job_id = ? ORDER BY started_at DESC LIMIT ?`
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
	return logs, nil
}
