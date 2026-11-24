package cron

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"fikpanel/internal/store"
)

// TestMatchCronExpression menguji evaluasi ekspresi cron 5-kolom lengkap termasuk rentang/step, L, nama hari/bulan, dan macro
func TestMatchCronExpression(t *testing.T) {
	// Waktu acuan: 2026-10-26 12:00:00 (Senin, Tanggal 26, Bulan 10, Jam 12, Menit 0)
	fixedTime := time.Date(2026, time.October, 26, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		expr     string
		at       time.Time
		expected bool
	}{
		{"Setiap hari jam 12:00", "0 12 * * *", fixedTime, true},
		{"Setiap jam menit 0", "0 * * * *", fixedTime, true},
		{"Setiap 15 menit", "*/15 * * * *", fixedTime, true},
		{"Jam 13 tidak cocok", "0 13 * * *", fixedTime, false},
		{"Hari Senin (1)", "0 12 * * 1", fixedTime, true},
		{"Hari Senin (MON)", "0 12 * * MON", fixedTime, true},
		{"Hari kerja MON-FRI", "0 12 * * MON-FRI", fixedTime, true},
		{"Bulan OCT", "0 12 * OCT *", fixedTime, true},
		{"Rentang dengan step 8-16/2 cocok jam 12", "0 8-16/2 * * *", fixedTime, true},
		{"Rentang dengan step 9-17/2 tidak cocok jam 12", "0 9-17/2 * * *", fixedTime, false},
		{"Macro @daily jam 00:00", "@daily", time.Date(2026, time.October, 26, 0, 0, 0, 0, time.UTC), true},
		{"Token L hari terakhir Februari kabisat (29 Feb 2028)", "0 0 L 2 *", time.Date(2028, time.February, 29, 0, 0, 0, 0, time.UTC), true},
		{"Token L bukan hari terakhir Februari kabisat (28 Feb 2028)", "0 0 L 2 *", time.Date(2028, time.February, 28, 0, 0, 0, 0, time.UTC), false},
		{"Semantik Vixie OR ketika tanggal dan hari sama-sama dibatasi", "0 12 1 * 1", fixedTime, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			norm := NormalizeSpec(tt.expr)
			got := matchCronExpression(strings.Fields(norm), tt.at)
			if got != tt.expected {
				t.Errorf("matchCronExpression(%q) = %v, ingin %v", tt.expr, got, tt.expected)
			}
		})
	}
}

func TestValidateSpecAndNextRunAt(t *testing.T) {
	invalidSpecs := []string{"", "halo dunia", "60 * * * *", "0 24 * * *", "*/0 * * * *", "0 12 * 13 *", "0 12 10-5 * *"}
	for _, bad := range invalidSpecs {
		if err := ValidateSpec(bad); err == nil {
			t.Errorf("ValidateSpec(%q) seharusnya gagal, namun lolos", bad)
		}
	}

	base := time.Date(2026, time.November, 24, 15, 17, 35, 0, time.UTC)
	next, err := NextRunAt("0 2 * * *", base)
	if err != nil {
		t.Fatalf("NextRunAt gagal: %v", err)
	}
	want := time.Date(2026, time.November, 25, 2, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("NextRunAt tidak sesuai: got=%v want=%v", next, want)
	}

	// Uji lompatan kabisat 29 Februari
	leapNext, err := NextRunAt("0 0 29 2 *", base)
	if err != nil {
		t.Fatalf("NextRunAt kabisat gagal: %v", err)
	}
	if leapNext.Year() != 2028 || leapNext.Month() != time.February || leapNext.Day() != 29 {
		t.Fatalf("NextRunAt kabisat salah: got=%v", leapNext)
	}
}

func TestValidateCommandAndLogPath(t *testing.T) {
	blocked := []string{
		"shutdown -h now",
		"echo ok && reboot",
		"mkfs.ext4 /dev/sda1",
		"rm -rf /",
		"passwd root",
		":(){ :|:& };:",
	}
	for _, cmd := range blocked {
		if err := ValidateCommand(cmd); err == nil {
			t.Errorf("ValidateCommand(%q) seharusnya diblokir", cmd)
		}
	}

	allowed := []string{
		"php artisan schedule:run",
		"rm -rf /tmp/app-cache/*",
		"fikpanel renew-ssl",
	}
	for _, cmd := range allowed {
		if err := ValidateCommand(cmd); err != nil {
			t.Errorf("ValidateCommand(%q) seharusnya diizinkan, error: %v", cmd, err)
		}
	}

	if err := ValidateLogFilePath("/var/log/../../etc/passwd"); err == nil {
		t.Errorf("Traversal '..' pada log_file seharusnya ditolak")
	}
	if runtime.GOOS == "linux" {
		if err := ValidateLogFilePath("/etc/nginx/nginx.conf"); err == nil {
			t.Errorf("Jalur /etc/ pada Linux seharusnya ditolak")
		}
	}
}

func TestExecuteStopTimeoutRetentionAndSecretRedaction(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "cron.db"))
	if err != nil {
		t.Fatalf("Gagal membuka DB uji: %v", err)
	}
	defer st.Close()

	mgr := NewManager(st)
	logFile := filepath.Join(tmpDir, "job.log")

	// 1. Uji eksekusi sukses + redaksi secret + penulisan berkas log
	jobID, err := st.CreateCronJob(t.Context(), &store.CronJob{
		Name:         "Cek Echo Secret",
		ScheduleType: "custom",
		Spec:         "* * * * *",
		Command:      "echo token=supersecret1234567890",
		Enabled:      true,
		SaveLog:      true,
		NotifyOnFail: true,
		LogFile:      logFile,
		TimeoutSec:   30,
	})
	if err != nil {
		t.Fatalf("CreateCronJob gagal: %v", err)
	}

	entry, err := mgr.ExecuteJob(t.Context(), jobID)
	if err != nil || entry == nil || entry.Status != "success" {
		t.Fatalf("ExecuteJob gagal: entry=%+v err=%v", entry, err)
	}
	if strings.Contains(entry.Output, "supersecret1234567890") {
		t.Fatalf("Secret seharusnya diredaksi sebelum disimpan ke log: %s", entry.Output)
	}
	if raw, err := os.ReadFile(logFile); err != nil || !strings.Contains(string(raw), "Eksekusi: Cek Echo Secret") {
		t.Fatalf("Berkas log disk tidak tertulis benar: err=%v", err)
	}

	// 2. Uji pemulihan status running tertinggal saat startup
	_ = st.UpdateCronJobStatus(t.Context(), jobID, time.Now().Unix(), "running")
	recovered, err := st.RecoverStaleRunningCronJobs(t.Context())
	if err != nil || recovered != 1 {
		t.Fatalf("RecoverStaleRunningCronJobs salah: recovered=%d err=%v", recovered, err)
	}
	jAfter, _ := st.GetCronJob(t.Context(), jobID)
	if jAfter.LastStatus != "interrupted" {
		t.Fatalf("Status seharusnya pulih ke interrupted, didapat: %s", jAfter.LastStatus)
	}

	// 3. Uji pemangkasan retensi 100 baris di cron_logs
	for i := 0; i < 110; i++ {
		_ = st.RecordCronLog(t.Context(), &store.CronLog{
			JobID:      jobID,
			Status:     "success",
			Output:     "ok",
			StartedAt:  int64(1700000000 + i),
			FinishedAt: int64(1700000001 + i),
		})
	}
	allLogs, _ := st.ListCronLogs(t.Context(), jobID, 200)
	if len(allLogs) != 100 {
		t.Fatalf("Retensi cron_logs seharusnya membatasi 100 baris, didapat: %d", len(allLogs))
	}

	// 4. Uji StopJob paksa pada tugas jangka panjang
	longCmd := "sleep 15"
	if runtime.GOOS == "windows" {
		longCmd = "ping -n 15 127.0.0.1 >nul"
	}
	longID, err := st.CreateCronJob(t.Context(), &store.CronJob{
		Name:         "Tugas Panjang",
		ScheduleType: "custom",
		Spec:         "0 0 1 1 *",
		Command:      longCmd,
		Enabled:      true,
		SaveLog:      true,
		TimeoutSec:   60,
	})
	if err != nil {
		t.Fatalf("CreateCronJob panjang gagal: %v", err)
	}

	doneCh := make(chan *store.CronLog, 1)
	go func() {
		l, _ := mgr.ExecuteJob(context.Background(), longID)
		doneCh <- l
	}()

	// Tunggu hingga benar-benar tercatat berjalan lalu hentikan paksa
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !mgr.IsRunning(longID) {
		time.Sleep(25 * time.Millisecond)
	}
	if !mgr.IsRunning(longID) {
		t.Fatalf("Tugas panjang tidak berstatus running")
	}
	if err := mgr.StopJob(longID); err != nil {
		t.Fatalf("StopJob gagal: %v", err)
	}
	select {
	case res := <-doneCh:
		if res == nil || res.Status != "stopped" {
			t.Fatalf("Status setelah StopJob harus stopped, didapat: %+v", res)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Tugas panjang tidak berhenti dalam 5 detik setelah StopJob")
	}
}

// TestLinuxRealProcessGroupTimeoutAndSystemCrons menguji isolasi process-group nyata (anti-orphan) dan pemindaian /etc/cron.d pada host Linux
func TestLinuxRealProcessGroupTimeoutAndSystemCrons(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pengujian process-group Linux hanya berjalan pada host Linux")
	}

	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "linux-cron.db"))
	if err != nil {
		t.Fatalf("Gagal membuka DB Linux uji: %v", err)
	}
	defer st.Close()

	mgr := NewManager(st)
	pidFile := filepath.Join(tmpDir, "child.pid")
	jobID, err := st.CreateCronJob(t.Context(), &store.CronJob{
		Name:         "Uji Timeout Anti-Orphan",
		ScheduleType: "custom",
		Spec:         "0 0 1 1 *",
		Command:      "sleep 45 & echo $! > " + pidFile + "; wait",
		Enabled:      true,
		SaveLog:      true,
		TimeoutSec:   1,
	})
	if err != nil {
		t.Fatalf("CreateCronJob gagal: %v", err)
	}

	logEntry, _ := mgr.ExecuteJob(context.Background(), jobID)
	if logEntry == nil || logEntry.Status != "timeout" || logEntry.ExitCode != 124 {
		t.Fatalf("Status timeout Linux salah: %+v", logEntry)
	}

	// Pastikan subproses sleep di dalam process-group sudah ikut mati (tidak menjadi orphan)
	time.Sleep(150 * time.Millisecond)
	pidRaw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("PID file subproses tidak dibuat: %v", err)
	}
	childPID := strings.TrimSpace(string(pidRaw))
	if _, statErr := os.Stat("/proc/" + childPID); statErr == nil {
		t.Fatalf("Subproses child PID %s masih hidup setelah timeout (orphan process terdeteksi!)", childPID)
	}

	// Verifikasi pemindaian read-only cron sistem OS (/etc/cron.d/* & /etc/crontab)
	sysList := ListSystemCrons(t.Context())
	if len(sysList) == 0 {
		t.Fatalf("Diharapkan menemukan minimal 1 cron sistem OS di host Linux")
	}
}

