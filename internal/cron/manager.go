package cron

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/ai"
	"fikpanel/internal/store"
)

const (
	maxCronOutputBytes  = 64 * 1024        // Batas ukuran output tersimpan per eksekusi (64 KB)
	maxCronLogFileBytes = 10 * 1024 * 1024 // Rotasi berkas .log otomatis bila melebihi 10 MB
)

// FailureCallback adalah fungsi notifikasi yang dipanggil saat tugas cron gagal atau timeout
type FailureCallback func(job *store.CronJob, logEntry *store.CronLog)

// runningEntry melacak tugas yang sedang berjalan beserta fungsi penghentiannya
type runningEntry struct {
	cancel      context.CancelFunc
	stopFlagged bool
}

// Manager mengelola penjadwalan dan eksekusi tugas cron secara presisi di latar belakang
type Manager struct {
	store     *store.Store
	mu        sync.RWMutex
	running   map[int64]*runningEntry
	lastFired map[int64]string // Deduplikasi eksekusi per menit (format "2006-01-02 15:04")
	onFailure FailureCallback
	stopChan  chan struct{}
	stopOnce  sync.Once
}

// NewManager membuat instance Cron Manager baru
func NewManager(s *store.Store) *Manager {
	return &Manager{
		store:     s,
		running:   make(map[int64]*runningEntry),
		lastFired: make(map[int64]string),
		stopChan:  make(chan struct{}),
	}
}

// SetFailureCallback mendaftarkan handler alert (mis. Webhook Telegram/Discord) saat tugas cron gagal atau timeout
func (m *Manager) SetFailureCallback(fn FailureCallback) {
	m.mu.Lock()
	m.onFailure = fn
	m.mu.Unlock()
}

// SeedDefaultJobs mendaftarkan tugas cron bawaan sistem (seperti auto-renew SSL) bila belum ada
func (m *Manager) SeedDefaultJobs(ctx context.Context) error {
	jobs, err := m.store.ListCronJobs(ctx)
	if err != nil {
		return err
	}

	for _, j := range jobs {
		if strings.Contains(strings.ToLower(j.Name), "ssl") || strings.Contains(j.Command, "renew-ssl") {
			return nil
		}
	}

	sslJob := &store.CronJob{
		Name:         "Domain SSL Auto-Renew (Let's Encrypt / ACME)",
		ScheduleType: "daily",
		Spec:         "0 2 * * *", // Dijalankan setiap hari pukul 02:00 waktu server
		Command:      "fikpanel renew-ssl",
		Enabled:      true,
		SaveLog:      true,
		NotifyOnFail: true,
		TimeoutSec:   1800,
	}

	_, err = m.store.CreateCronJob(ctx, sslJob)
	return err
}

// Start memulihkan status tertinggal, menyemai tugas bawaan, dan menjalankan scheduler tersinkronisasi batas menit (:00)
func (m *Manager) Start(ctx context.Context) {
	_, _ = m.store.RecoverStaleRunningCronJobs(ctx)
	_ = m.SeedDefaultJobs(ctx)

	go func() {
		for {
			// Hitung jeda tepat hingga awal menit berikutnya (+200ms margin agar clock melewati detik :00)
			now := time.Now()
			nextTick := now.Truncate(time.Minute).Add(time.Minute).Add(200 * time.Millisecond)
			wait := time.Until(nextTick)
			if wait < 200*time.Millisecond {
				wait = 200 * time.Millisecond
			}

			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-m.stopChan:
				timer.Stop()
				return
			case tickTime := <-timer.C:
				m.checkAndRun(ctx, tickTime)
			}
		}
	}()
}

// Stop menghentikan scheduler secara aman (idempoten)
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopChan)
	})
}

// IsRunning melaporkan apakah tugas cron dengan ID terkait sedang aktif dieksekusi
func (m *Manager) IsRunning(jobID int64) bool {
	m.mu.RLock()
	_, ok := m.running[jobID]
	m.mu.RUnlock()
	return ok
}

// StopJob menghentikan paksa tugas cron yang sedang berjalan beserta seluruh process group-nya
func (m *Manager) StopJob(jobID int64) error {
	m.mu.Lock()
	entry, ok := m.running[jobID]
	if !ok || entry == nil {
		m.mu.Unlock()
		return fmt.Errorf("tugas cron #%d sedang tidak berjalan", jobID)
	}
	entry.stopFlagged = true
	cancelFn := entry.cancel
	m.mu.Unlock()

	if cancelFn != nil {
		cancelFn()
	}
	return nil
}

func (m *Manager) checkAndRun(ctx context.Context, now time.Time) {
	jobs, err := m.store.ListCronJobs(ctx)
	if err != nil {
		return
	}

	minuteKey := now.Format("2006-01-02 15:04")
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		if !m.shouldRun(job, now) {
			continue
		}

		// Deduplikasi per menit agar tidak pernah terpicu dua kali akibat pergeseran jam NTP
		m.mu.Lock()
		if m.lastFired[job.ID] == minuteKey {
			m.mu.Unlock()
			continue
		}
		m.lastFired[job.ID] = minuteKey
		m.mu.Unlock()

		go func(jID int64) {
			_, _ = m.ExecuteJob(ctx, jID)
		}(job.ID)
	}
}

// shouldRun memeriksa apakah saat ini jadwal cron terpenuhi
func (m *Manager) shouldRun(job store.CronJob, now time.Time) bool {
	norm := NormalizeSpec(job.Spec)
	fields := strings.Fields(norm)
	if len(fields) == 5 {
		return matchCronExpression(fields, now)
	}

	// Evaluasi fallback bagi data lawas non-5-kolom
	switch job.ScheduleType {
	case "minute_n":
		var n int
		fmt.Sscanf(job.Spec, "%d", &n)
		if n <= 0 {
			n = 10
		}
		return now.Minute()%n == 0
	case "hourly":
		return now.Minute() == 0
	case "daily":
		hour, min := 0, 0
		fmt.Sscanf(job.Spec, "%d:%d", &hour, &min)
		return now.Hour() == hour && now.Minute() == min
	case "weekly":
		var day, hour, min int
		fmt.Sscanf(job.Spec, "%d %d:%d", &day, &hour, &min)
		return int(now.Weekday()) == day && now.Hour() == hour && now.Minute() == min
	case "monthly":
		var date, hour, min int
		fmt.Sscanf(job.Spec, "%d %d:%d", &date, &hour, &min)
		return now.Day() == date && now.Hour() == hour && now.Minute() == min
	default:
		return false
	}
}

// TriggerJobAsync menjalankan tugas di latar belakang (terlepas dari request HTTP) dan menunggu maksimal waitDur untuk hasil cepat
func (m *Manager) TriggerJobAsync(bgCtx context.Context, jobID int64, waitDur time.Duration) (logEntry *store.CronLog, stillRunning bool, err error) {
	if m.IsRunning(jobID) {
		return nil, true, fmt.Errorf("tugas cron #%d masih dalam status berjalan", jobID)
	}
	job, err := m.store.GetCronJob(bgCtx, jobID)
	if err != nil {
		return nil, false, err
	}
	if job == nil {
		return nil, false, fmt.Errorf("tugas cron tidak ditemukan")
	}

	type execResult struct {
		log *store.CronLog
		err error
	}
	done := make(chan execResult, 1)
	go func() {
		l, e := m.ExecuteJob(bgCtx, jobID)
		done <- execResult{log: l, err: e}
	}()

	if waitDur <= 0 {
		waitDur = 2500 * time.Millisecond
	}
	timer := time.NewTimer(waitDur)
	defer timer.Stop()

	select {
	case res := <-done:
		return res.log, false, res.err
	case <-timer.C:
		return nil, true, nil
	}
}

// ExecuteJob menjalankan perintah shell dalam process group terisolasi, menyensor rahasia, merotasi log, dan mengirim alert jika gagal
func (m *Manager) ExecuteJob(ctx context.Context, jobID int64) (*store.CronLog, error) {
	job, err := m.store.GetCronJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca tugas cron: %w", err)
	}
	if job == nil {
		return nil, fmt.Errorf("cron job #%d tidak ditemukan", jobID)
	}

	timeout := time.Duration(job.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 1 * time.Hour
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	entry := &runningEntry{cancel: cancel}

	m.mu.Lock()
	if _, exists := m.running[jobID]; exists {
		m.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("tugas cron #%d masih dalam status berjalan", jobID)
	}
	m.running[jobID] = entry
	m.mu.Unlock()

	defer func() {
		cancel()
		m.mu.Lock()
		delete(m.running, jobID)
		m.mu.Unlock()
	}()

	dbCtx := context.Background()
	_ = m.store.UpdateCronJobStatus(dbCtx, jobID, time.Now().Unix(), "running")

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "cmd", "/c", job.Command)
	} else {
		cmd = exec.CommandContext(execCtx, "bash", "-c", job.Command)
	}
	configureCommandProcessGroup(cmd)

	if wd := strings.TrimSpace(job.WorkingDir); wd != "" {
		if st, statErr := os.Stat(wd); statErr == nil && st.IsDir() {
			cmd.Dir = wd
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	runErr := cmd.Run()
	duration := time.Since(startTime)
	finishedAt := time.Now().Unix()

	m.mu.RLock()
	manuallyStopped := entry.stopFlagged
	failCb := m.onFailure
	m.mu.RUnlock()

	exitCode := 0
	status := "success"
	if runErr != nil {
		status = "failed"
		if manuallyStopped {
			status = "stopped"
			exitCode = 137
		} else if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			status = "timeout"
			exitCode = 124
		} else if exitErr, ok := runErr.(*exec.ExitError); ok {
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
	if status == "timeout" {
		outCombined = strings.TrimSpace(outCombined + fmt.Sprintf("\n[FikPanel] Eksekusi dihentikan otomatis karena melampaui batas waktu %d detik.", int(timeout.Seconds())))
	} else if status == "stopped" {
		outCombined = strings.TrimSpace(outCombined + "\n[FikPanel] Eksekusi dihentikan secara manual oleh administrator.")
	}

	// Sensor pola rahasia (token/password/API key) sebelum disimpan ke SQLite atau berkas log
	outCombined = ai.RedactSecrets(outCombined)
	if len(outCombined) > maxCronOutputBytes {
		outCombined = outCombined[:maxCronOutputBytes] + "\n... [log dipotong karena melebihi 64KB]"
	}

	_ = m.store.UpdateCronJobStatus(dbCtx, jobID, finishedAt, status)

	logEntry := &store.CronLog{
		JobID:      jobID,
		Status:     status,
		Output:     outCombined,
		ExitCode:   exitCode,
		DurationMs: duration.Milliseconds(),
		StartedAt:  startTime.Unix(),
		FinishedAt: finishedAt,
	}

	// 1. Tulis ke berkas log disk bila opsi log_file dikonfigurasi (dengan rotasi otomatis 10 MB)
	if strings.TrimSpace(job.LogFile) != "" && ValidateLogFilePath(job.LogFile) == nil {
		resolvedPath := resolveLogFilePath(job.LogFile, job.Name, job.ID)
		_ = writeToLogFile(resolvedPath, job.Name, job.ID, status, exitCode, duration.Milliseconds(), outCombined, startTime)
	}

	// 2. Simpan riwayat eksekusi ke SQLite (otomatis memangkas di atas 100 riwayat per tugas)
	if job.SaveLog {
		_ = m.store.RecordCronLog(dbCtx, logEntry)
	}

	// 3. Kirim alert webhook asinkron bila tugas gagal atau timeout dan notify_on_fail aktif
	if failCb != nil && job.NotifyOnFail && (status == "failed" || status == "timeout") {
		jobCopy := *job
		logCopy := *logEntry
		go failCb(&jobCopy, &logCopy)
	}

	return logEntry, runErr
}

// sanitizeSlug membersihkan nama tugas agar aman dijadikan nama file
func sanitizeSlug(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-]+`)
	slug := strings.ToLower(reg.ReplaceAllString(s, "-"))
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "cron-task"
	}
	return slug
}

// resolveLogFilePath menentukan path berkas log final dan memastikan ekstensinya .log
func resolveLogFilePath(rawPath, jobName string, jobID int64) string {
	clean := strings.TrimSpace(rawPath)
	if clean == "" {
		return ""
	}
	// Jika pengguna hanya memasukkan folder (misal: /var/cronlog atau /var/cronlog/)
	if strings.HasSuffix(clean, "/") || strings.HasSuffix(clean, "\\") || !strings.Contains(filepath.Base(clean), ".") {
		slug := sanitizeSlug(jobName)
		return filepath.Join(clean, fmt.Sprintf("%s-%d.log", slug, jobID))
	}
	return filepath.Clean(clean)
}

// writeToLogFile menulis hasil eksekusi ke berkas log di disk dengan rotasi otomatis (.1) bila melampaui 10 MB
func writeToLogFile(filePath, jobName string, jobID int64, status string, exitCode int, durationMs int64, output string, startTime time.Time) error {
	if filePath == "" {
		return nil
	}
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("gagal membuat direktori log %s: %w", dir, err)
	}

	// Rotasi otomatis berkas log jika ukurannya melebihi 10 MB agar penyimpanan server tidak penuh
	if fi, err := os.Stat(filePath); err == nil && fi.Size() >= maxCronLogFileBytes {
		_ = os.Rename(filePath, filePath+".1")
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("gagal membuka berkas log %s: %w", filePath, err)
	}
	defer f.Close()

	header := fmt.Sprintf("\n================================================================================\n[%s] Eksekusi: %s (#%d)\nStatus: %s | Durasi: %d ms | Kode Keluar: %d\n--------------------------------------------------------------------------------\n",
		startTime.Format("2006-01-02 15:04:05"), jobName, jobID, strings.ToUpper(status), durationMs, exitCode)

	if _, err := f.WriteString(header); err != nil {
		return err
	}
	if len(output) > 0 {
		if _, err := f.WriteString(output); err != nil {
			return err
		}
		if !strings.HasSuffix(output, "\n") {
			_, _ = f.WriteString("\n")
		}
	}
	_, err = f.WriteString("================================================================================\n")
	return err
}

// ReadJobLogFile membaca isi berkas log langsung dari disk secara aman (tervalidasi prefix)
func (m *Manager) ReadJobLogFile(ctx context.Context, jobID int64, maxBytes int64) (string, string, error) {
	job, err := m.store.GetCronJob(ctx, jobID)
	if err != nil || job == nil {
		return "", "", fmt.Errorf("tugas cron tidak ditemukan")
	}
	if strings.TrimSpace(job.LogFile) == "" {
		return "", "", fmt.Errorf("berkas log kustom tidak dikonfigurasi untuk tugas ini")
	}
	if err := ValidateLogFilePath(job.LogFile); err != nil {
		return "", job.LogFile, err
	}

	resolved := resolveLogFilePath(job.LogFile, job.Name, job.ID)
	info, err := os.Stat(resolved)
	if os.IsNotExist(err) {
		return "", resolved, fmt.Errorf("berkas log belum dibuat (tugas belum pernah dijalankan)")
	} else if err != nil {
		return "", resolved, err
	}

	if maxBytes <= 0 {
		maxBytes = 128 * 1024 // 128 KB
	}

	f, err := os.Open(resolved)
	if err != nil {
		return "", resolved, err
	}
	defer f.Close()

	size := info.Size()
	offset := int64(0)
	if size > maxBytes {
		offset = size - maxBytes
		_, _ = f.Seek(offset, 0)
	}

	buf := make([]byte, maxBytes)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", resolved, err
	}

	content := string(buf[:n])
	if offset > 0 {
		content = "... [menampilkan 128KB terakhir dari berkas log]\n\n" + content
	}
	return content, resolved, nil
}

// ClearJobLogFile mengosongkan berkas log di disk secara aman setelah validasi prefix
func (m *Manager) ClearJobLogFile(ctx context.Context, jobID int64) (string, error) {
	job, err := m.store.GetCronJob(ctx, jobID)
	if err != nil || job == nil {
		return "", fmt.Errorf("tugas cron tidak ditemukan")
	}
	if strings.TrimSpace(job.LogFile) == "" {
		return "", fmt.Errorf("berkas log kustom tidak dikonfigurasi")
	}
	if err := ValidateLogFilePath(job.LogFile); err != nil {
		return job.LogFile, err
	}
	resolved := resolveLogFilePath(job.LogFile, job.Name, job.ID)
	if err := os.Truncate(resolved, 0); err != nil {
		return resolved, err
	}
	return resolved, nil
}
