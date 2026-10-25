package cron

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"fikpanel/internal/store"
)

// Manager mengelola penjadwalan dan eksekusi tugas cron secara background
type Manager struct {
	store    *store.Store
	mu       sync.RWMutex
	running  map[int64]bool
	stopChan chan struct{}
}

// NewManager membuat instance Cron Manager baru
func NewManager(s *store.Store) *Manager {
	return &Manager{
		store:    s,
		running:  make(map[int64]bool),
		stopChan: make(chan struct{}),
	}
}

// Start memulai loop pengecekan cron setiap menit
func (m *Manager) Start(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-m.stopChan:
				ticker.Stop()
				return
			case <-ticker.C:
				m.checkAndRun(ctx)
			}
		}
	}()
}

// Stop menghentikan scheduler
func (m *Manager) Stop() {
	close(m.stopChan)
}

func (m *Manager) checkAndRun(ctx context.Context) {
	jobs, err := m.store.ListCronJobs(ctx)
	if err != nil {
		return
	}

	now := time.Now()
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		if m.shouldRun(job, now) {
			go m.ExecuteJob(context.Background(), job.ID)
		}
	}
}

// shouldRun memeriksa apakah saat ini jadwal cron terpenuhi
func (m *Manager) shouldRun(job store.CronJob, now time.Time) bool {
	// Simple matching berdasarkan schedule_type
	switch job.ScheduleType {
	case "minute_n": // per N menit misal spec="15"
		var n int
		fmt.Sscanf(job.Spec, "%d", &n)
		if n <= 0 {
			n = 10
		}
		return now.Minute()%n == 0
	case "hourly": // setiap jam di menit ke-0
		return now.Minute() == 0
	case "daily": // setiap hari di jam 00:00 atau spec="HH:MM"
		hour, min := 0, 0
		fmt.Sscanf(job.Spec, "%d:%d", &hour, &min)
		return now.Hour() == hour && now.Minute() == min
	case "weekly": // setiap minggu spec="D HH:MM" (0: Sunday, 1: Monday)
		var day, hour, min int
		fmt.Sscanf(job.Spec, "%d %d:%d", &day, &hour, &min)
		return int(now.Weekday()) == day && now.Hour() == hour && now.Minute() == min
	case "monthly": // setiap bulan spec="DD HH:MM"
		var date, hour, min int
		fmt.Sscanf(job.Spec, "%d %d:%d", &date, &hour, &min)
		return now.Day() == date && now.Hour() == hour && now.Minute() == min
	default:
		// Fallback custom: cocokkan tiap jam jika belum ada runtime cron parser kompleks
		return false
	}
}

// ExecuteJob menjalankan perintah shell dari cron job dan merekam lognya
func (m *Manager) ExecuteJob(ctx context.Context, jobID int64) (*store.CronLog, error) {
	m.mu.Lock()
	if m.running[jobID] {
		m.mu.Unlock()
		return nil, fmt.Errorf("tugas cron #%d masih dalam status berjalan", jobID)
	}
	m.running[jobID] = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.running, jobID)
		m.mu.Unlock()
	}()

	job, err := m.store.GetCronJob(ctx, jobID)
	if err != nil || job == nil {
		return nil, fmt.Errorf("cron job tidak ditemukan: %w", err)
	}

	_ = m.store.UpdateCronJobStatus(ctx, jobID, time.Now().Unix(), "running")

	timeout := time.Duration(job.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 1 * time.Hour
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "cmd", "/c", job.Command)
	} else {
		cmd = exec.CommandContext(execCtx, "bash", "-c", job.Command)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	runErr := cmd.Run()
	duration := time.Since(startTime)
	finishedAt := time.Now().Unix()

	exitCode := 0
	status := "success"
	if runErr != nil {
		status = "failed"
		if execCtx.Err() == context.DeadlineExceeded {
			status = "timeout"
		}
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	outCombined := stdout.String()
	if stderr.Len() > 0 {
		if len(outCombined) > 0 {
			outCombined += "\n--- STDERR ---\n"
		}
		outCombined += stderr.String()
	}

	// Batasi ukuran output log maksimal 64KB
	if len(outCombined) > 65536 {
		outCombined = outCombined[:65536] + "\n... [log dipotong karena melebihi 64KB]"
	}

	_ = m.store.UpdateCronJobStatus(ctx, jobID, finishedAt, status)

	logEntry := &store.CronLog{
		JobID:      jobID,
		Status:     status,
		Output:     outCombined,
		ExitCode:   exitCode,
		DurationMs: duration.Milliseconds(),
		StartedAt:  startTime.Unix(),
		FinishedAt: finishedAt,
	}

	if job.SaveLog {
		_ = m.store.RecordCronLog(ctx, logEntry)
	}

	return logEntry, runErr
}
