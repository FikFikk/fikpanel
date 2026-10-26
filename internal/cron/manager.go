package cron

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
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

// SeedDefaultJobs mendaftarkan tugas cron bawaan sistem (seperti auto-renew SSL ala aaPanel/HestiaCP)
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
		TimeoutSec:   1800,
	}

	_, err = m.store.CreateCronJob(ctx, sslJob)
	return err
}

// Start memulai loop pengecekan cron setiap menit
func (m *Manager) Start(ctx context.Context) {
	_ = m.SeedDefaultJobs(ctx)

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
	// Evaluasi langsung jika spec merupakan ekspresi standar 5-field (misal: "0 5 * * *")
	fields := strings.Fields(strings.TrimSpace(job.Spec))
	if len(fields) == 5 {
		return matchCronExpression(fields, now)
	}

	// Evaluasi fallback berdasarkan tipe jadwal bawaan
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
		return false
	}
}

// matchCronExpression mencocokkan waktu sekarang terhadap 5 kolom ekspresi cron standard
func matchCronExpression(fields []string, t time.Time) bool {
	if len(fields) != 5 {
		return false
	}
	minuteMatch := matchCronField(fields[0], t.Minute(), 0, 59)
	hourMatch := matchCronField(fields[1], t.Hour(), 0, 23)
	dayMatch := matchCronField(fields[2], t.Day(), 1, 31)
	monthMatch := matchCronField(fields[3], int(t.Month()), 1, 12)
	dowMatch := matchCronField(fields[4], int(t.Weekday()), 0, 7) // 0 atau 7 = Minggu
	return minuteMatch && hourMatch && dayMatch && monthMatch && dowMatch
}

// matchCronField mengecek satu token cron (mendukung *, */N, rentang N-M, dan koma A,B)
func matchCronField(field string, value, min, max int) bool {
	if field == "*" {
		return true
	}

	// Tangani step */N
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(strings.TrimPrefix(field, "*/"))
		if err != nil || step <= 0 {
			return false
		}
		return value%step == 0
	}

	// Tangani daftar nilai dipisah koma
	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		for _, p := range parts {
			if matchCronField(strings.TrimSpace(p), value, min, max) {
				return true
			}
		}
		return false
	}

	// Tangani rentang N-M
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(parts[0])
			end, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				return value >= start && value <= end
			}
		}
		return false
	}

	// Nilai angka tunggal
	num, err := strconv.Atoi(field)
	if err == nil {
		if field == "7" && min == 0 && max == 7 {
			return value == 0 // Di cron, 7 adalah hari Minggu
		}
		return value == num
	}

	return false
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

	// 1. Tulis ke berkas log disk bila opsi log_file diaktifkan
	if strings.TrimSpace(job.LogFile) != "" {
		resolvedPath := resolveLogFilePath(job.LogFile, job.Name, job.ID)
		_ = writeToLogFile(resolvedPath, job.Name, job.ID, status, exitCode, duration.Milliseconds(), outCombined, startTime)
	}

	// 2. Simpan riwayat eksekusi ke database bila opsi SaveLog aktif
	if job.SaveLog {
		_ = m.store.RecordCronLog(ctx, logEntry)
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

// writeToLogFile menulis hasil eksekusi ke file log di disk secara otomatis membuat folder bila belum ada
func writeToLogFile(filePath, jobName string, jobID int64, status string, exitCode int, durationMs int64, output string, startTime time.Time) error {
	if filePath == "" {
		return nil
	}
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("gagal membuat direktori log %s: %w", dir, err)
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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

// ReadJobLogFile membaca isi berkas log langsung dari disk untuk ditampilkan di UI
func (m *Manager) ReadJobLogFile(ctx context.Context, jobID int64, maxBytes int64) (string, string, error) {
	job, err := m.store.GetCronJob(ctx, jobID)
	if err != nil || job == nil {
		return "", "", fmt.Errorf("tugas cron tidak ditemukan")
	}
	if strings.TrimSpace(job.LogFile) == "" {
		return "", "", fmt.Errorf("berkas log kustom tidak dikonfigurasi untuk tugas ini")
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

// ClearJobLogFile mengosongkan berkas log di disk
func (m *Manager) ClearJobLogFile(ctx context.Context, jobID int64) (string, error) {
	job, err := m.store.GetCronJob(ctx, jobID)
	if err != nil || job == nil {
		return "", fmt.Errorf("tugas cron tidak ditemukan")
	}
	if strings.TrimSpace(job.LogFile) == "" {
		return "", fmt.Errorf("berkas log kustom tidak dikonfigurasi")
	}
	resolved := resolveLogFilePath(job.LogFile, job.Name, job.ID)
	if err := os.Truncate(resolved, 0); err != nil {
		return resolved, err
	}
	return resolved, nil
}
