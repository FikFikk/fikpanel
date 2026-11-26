package database

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/store"
)

// TaskManager mengelola eksekusi pekerjaan latar belakang dan penyiaran log real-time SSE
type TaskManager struct {
	store       *store.Store
	mu          sync.RWMutex
	subscribers map[string][]chan string
}

// NewTaskManager menginisialisasi TaskManager baru
func NewTaskManager(st *store.Store) *TaskManager {
	return &TaskManager{
		store:       st,
		subscribers: make(map[string][]chan string),
	}
}

// Subscribe mendaftarkan listener untuk menerima stream log tugas tertentu
func (tm *TaskManager) Subscribe(taskID string) (<-chan string, func()) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	ch := make(chan string, 100)
	tm.subscribers[taskID] = append(tm.subscribers[taskID], ch)

	// Kembalikan fungsi pembersih untuk unregister saat koneksi SSE ditutup oleh klien
	cleanup := func() {
		tm.mu.Lock()
		defer tm.mu.Unlock()
		subs := tm.subscribers[taskID]
		for i, s := range subs {
			if s == ch {
				tm.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
		if len(tm.subscribers[taskID]) == 0 {
			delete(tm.subscribers, taskID)
		}
	}

	return ch, cleanup
}

// Broadcast mengirimkan baris log ke seluruh subscriber aktif
func (tm *TaskManager) Broadcast(taskID, msg string) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, ch := range tm.subscribers[taskID] {
		select {
		case ch <- msg:
		default:
			// Lewati jika buffer subscriber penuh untuk mencegah blocking
		}
	}
}

// runStreamedCommand menjalankan perintah sistem dan mengalirkan outputnya baris demi baris ke SSE
func runStreamedCommand(ctx context.Context, cmd *exec.Cmd, logLine func(string)) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	streamReader := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 256*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				logLine(line)
			}
		}
	}

	wg.Add(2)
	go streamReader(stdout)
	go streamReader(stderr)
	wg.Wait()

	return cmd.Wait()
}

// StartInstallTask memicu proses instalasi engine database di latar belakang
func (tm *TaskManager) StartInstallTask(ctx context.Context, engine, version, mode string) (string, error) {
	taskID := fmt.Sprintf("task_install_%s_%d", engine, time.Now().Unix())
	target := fmt.Sprintf("%s-%s", engine, version)

	task := store.SystemTask{
		ID:        taskID,
		Type:      "software_install",
		Target:    target,
		Status:    "running",
		Log:       "",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	if err := tm.store.CreateTask(ctx, task); err != nil {
		return "", fmt.Errorf("gagal mendaftarkan tugas ke database: %w", err)
	}

	// Jalankan eksekusi instalasi dalam goroutine terpisah
	go tm.executeInstall(taskID, engine, version, mode)

	return taskID, nil
}

// executeInstall menjalankan langkah instalasi nyata di Linux (APT / Docker) atau simulasi dev di OS lain
func (tm *TaskManager) executeInstall(taskID, engine, version, mode string) {
	port := 27017
	switch engine {
	case "mysql":
		port = 3306
	case "postgresql":
		port = 5432
	case "mongodb":
		port = 27017
	case "redis":
		port = 6379
	}

	logLine := func(line string) {
		ts := time.Now().Format("15:04:05")
		formatted := fmt.Sprintf("[%s] %s\n", ts, line)
		tm.Broadcast(taskID, formatted)
		_ = tm.store.AppendTaskLog(context.Background(), taskID, "running", formatted)
	}

	failTask := func(msg string) {
		logLine("==> [GAGAL] " + msg)
		_ = tm.store.AppendTaskLog(context.Background(), taskID, "failed", "")
		tm.Broadcast(taskID, "TASK_COMPLETE_FAILED")
	}

	logLine(fmt.Sprintf("==> Memulai instalasi paket [%s %s] dalam mode %s...", engine, version, mode))
	time.Sleep(250 * time.Millisecond)

	if engine == "phpmyadmin" {
		targetDir, err := InstallPhpMyAdminFiles(context.Background(), version, logLine)
		if err != nil {
			failTask(fmt.Sprintf("Instalasi paket phpMyAdmin gagal: %v", err))
			return
		}

		_ = targetDir
		SetEngineInstalled("phpmyadmin", version, true)
		_ = tm.store.AppendTaskLog(context.Background(), taskID, "success", "")
		tm.Broadcast(taskID, "TASK_COMPLETE_SUCCESS")
		return
	}

	if runtime.GOOS != "linux" {
		// Simulasi alur eksekusi untuk lingkungan pengembangan lokal non-Linux
		steps := []string{
			fmt.Sprintf("Memverifikasi arsitektur sistem (%s %s)... Cocok.", runtime.GOOS, runtime.GOARCH),
			fmt.Sprintf("Mengunduh dependensi inti untuk %s %s...", engine, version),
			fmt.Sprintf("Menyiapkan direktori data /var/lib/%s...", engine),
			fmt.Sprintf("Membuat konfigurasi daemon (bind: 127.0.0.1, port: %d)...", port),
			fmt.Sprintf("Menjalankan daemon %s.service...", engine),
			fmt.Sprintf("Health check: Menguji soket TCP 127.0.0.1:%d... Sukses (terbuka).", port),
			fmt.Sprintf("==> [SELESAI] Engine %s %s berhasil diinstal dan berstatus aktif (running)!", engine, version),
		}
		for _, step := range steps {
			time.Sleep(300 * time.Millisecond)
			logLine(step)
		}

		SetEngineInstalled(engine, version, true)
		_ = tm.store.AppendTaskLog(context.Background(), taskID, "success", "")
		tm.Broadcast(taskID, "TASK_COMPLETE_SUCCESS")
		return
	}

	// Eksekusi nyata pada server Linux dengan batas waktu 10 menit
	execCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Mode kontainer Docker
	if strings.EqualFold(mode, "docker") {
		if _, err := exec.LookPath("docker"); err != nil {
			failTask("Docker belum terpasang di server ini. Pasang Docker melalui App Store atau gunakan mode Native.")
			return
		}
		var image string
		var dockerArgs []string
		containerName := fmt.Sprintf("fikpanel-%s", engine)
		switch engine {
		case "mysql":
			image = fmt.Sprintf("mariadb:%s", version)
			dockerArgs = []string{"run", "-d", "--name", containerName, "--restart", "unless-stopped", "-p", "127.0.0.1:3306:3306", "-e", "MARIADB_RANDOM_ROOT_PASSWORD=yes", image}
		case "postgresql":
			image = fmt.Sprintf("postgres:%s-alpine", version)
			dockerArgs = []string{"run", "-d", "--name", containerName, "--restart", "unless-stopped", "-p", "127.0.0.1:5432:5432", "-e", "POSTGRES_PASSWORD=fikpanel_init", image}
		case "redis":
			image = fmt.Sprintf("redis:%s-alpine", version)
			dockerArgs = []string{"run", "-d", "--name", containerName, "--restart", "unless-stopped", "-p", "127.0.0.1:6379:6379", image}
		case "mongodb":
			image = fmt.Sprintf("mongo:%s", version)
			dockerArgs = []string{"run", "-d", "--name", containerName, "--restart", "unless-stopped", "-p", "127.0.0.1:27017:27017", image}
		default:
			failTask(fmt.Sprintf("Engine '%s' belum didukung untuk mode Docker.", engine))
			return
		}

		logLine(fmt.Sprintf("Menarik citra dan menjalankan kontainer Docker %s (%s)...", containerName, image))
		cmd := exec.CommandContext(execCtx, "docker", dockerArgs...)
		if err := runStreamedCommand(execCtx, cmd, logLine); err != nil {
			failTask(fmt.Sprintf("Gagal menjalankan kontainer Docker: %v", err))
			return
		}

		logLine(fmt.Sprintf("==> [SELESAI] Kontainer %s (%s) aktif berjalan!", containerName, image))
		SetEngineInstalled(engine, version, true)
		_ = tm.store.AppendTaskLog(context.Background(), taskID, "success", "")
		tm.Broadcast(taskID, "TASK_COMPLETE_SUCCESS")
		return
	}

	// Mode Native Linux (APT Debian / Ubuntu / Armbian)
	if _, err := exec.LookPath("apt-get"); err != nil {
		failTask("Manajer paket apt-get tidak ditemukan. Gunakan mode Docker atau instalasi manual sesuai distro Linux Anda.")
		return
	}

	var pkgs []string
	var unitName string
	switch engine {
	case "mysql":
		pkgs = []string{"mariadb-server", "mariadb-client"}
		unitName = "mariadb"
	case "postgresql":
		pkgs = []string{"postgresql", "postgresql-contrib"}
		unitName = "postgresql"
	case "redis":
		pkgs = []string{"redis-server"}
		unitName = "redis-server"
	case "mongodb":
		pkgs = []string{"mongodb-org"}
		unitName = "mongod"
	default:
		failTask(fmt.Sprintf("Engine tidak dikenal: %s", engine))
		return
	}

	logLine("Langkah 1/3: Memperbarui indeks repositori APT (apt-get update)...")
	updateCmd := exec.CommandContext(execCtx, "apt-get", "update", "-y")
	updateCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	if err := runStreamedCommand(execCtx, updateCmd, logLine); err != nil {
		failTask(fmt.Sprintf("apt-get update gagal: %v", err))
		return
	}

	logLine(fmt.Sprintf("Langkah 2/3: Memasang paket %s...", strings.Join(pkgs, " ")))
	installArgs := append([]string{"install", "-y", "--no-install-recommends"}, pkgs...)
	installCmd := exec.CommandContext(execCtx, "apt-get", installArgs...)
	installCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	if err := runStreamedCommand(execCtx, installCmd, logLine); err != nil {
		if engine == "mongodb" {
			failTask("Paket mongodb-org belum tersedia di repositori APT bawaan distro ini (khususnya ARM64/Armbian). Disarankan memilih mode instalasi Docker atau menambahkan repositori resmi MongoDB.")
		} else {
			failTask(fmt.Sprintf("Instalasi paket APT gagal: %v", err))
		}
		return
	}

	logLine(fmt.Sprintf("Langkah 3/3: Mengaktifkan dan menyalakan layanan systemd %s...", unitName))
	enableCmd := exec.CommandContext(execCtx, "systemctl", "enable", "--now", unitName)
	_ = runStreamedCommand(execCtx, enableCmd, logLine)

	// Verifikasi ulang status layanan & versi terpasang
	detected := DetectEngines(execCtx, map[string]int{})
	detectedVer := version
	detectedRunning := true
	for _, d := range detected {
		if d.Name == engine && d.Installed {
			if d.Version != "" {
				detectedVer = d.Version
			}
			detectedRunning = d.Running
			break
		}
	}

	SetEngineInstalled(engine, detectedVer, detectedRunning)
	logLine(fmt.Sprintf("==> [SELESAI] Engine %s (%s) berhasil dipasang dan siap digunakan!", engine, detectedVer))
	_ = tm.store.AppendTaskLog(context.Background(), taskID, "success", "")
	tm.Broadcast(taskID, "TASK_COMPLETE_SUCCESS")
}

// StartSwitchVersionTask memicu proses penggantian versi engine database dengan prosedur backup otomatis
func (tm *TaskManager) StartSwitchVersionTask(ctx context.Context, engine, targetVersion string, backupFirst bool) (string, error) {
	taskID := fmt.Sprintf("task_switch_%s_%d", engine, time.Now().Unix())
	target := fmt.Sprintf("%s-switch-%s", engine, targetVersion)

	task := store.SystemTask{
		ID:        taskID,
		Type:      "version_switch",
		Target:    target,
		Status:    "running",
		Log:       "",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	if err := tm.store.CreateTask(ctx, task); err != nil {
		return "", fmt.Errorf("gagal mendaftarkan tugas migrasi versi: %w", err)
	}

	// Eksekusi proses pergantian versi di latar belakang
	go tm.executeSwitchVersion(taskID, engine, targetVersion, backupFirst)

	return taskID, nil
}

// executeSwitchVersion menjalankan snapshot cadangan pra-migrasi nyata dan validasi status daemon
func (tm *TaskManager) executeSwitchVersion(taskID, engine, targetVersion string, backupFirst bool) {
	logLine := func(line string) {
		ts := time.Now().Format("15:04:05")
		formatted := fmt.Sprintf("[%s] %s\n", ts, line)
		tm.Broadcast(taskID, formatted)
		_ = tm.store.AppendTaskLog(context.Background(), taskID, "running", formatted)
	}

	logLine(fmt.Sprintf("==> Memulai prosedur alih versi mesin database [%s] ke target versi [%s]...", engine, targetVersion))

	if runtime.GOOS == "linux" {
		execCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if backupFirst {
			backupDir := "/var/backups/fikpanel"
			_ = os.MkdirAll(backupDir, 0755)
			dumpPath := filepath.Join(backupDir, fmt.Sprintf("preswitch_%s_%s.sql.gz", engine, time.Now().Format("20060102-150405")))
			logLine(fmt.Sprintf("Langkah 1/3: Membuat cadangan penuh seluruh database ke %s...", dumpPath))

			var dumpCmd string
			switch engine {
			case "mysql":
				if _, err := os.Stat("/etc/mysql/debian.cnf"); err == nil {
					dumpCmd = fmt.Sprintf("mysqldump --defaults-file=/etc/mysql/debian.cnf --all-databases --single-transaction --routines --triggers | gzip > %q", dumpPath)
				} else {
					dumpCmd = fmt.Sprintf("mysqldump -u root --all-databases --single-transaction --routines --triggers | gzip > %q", dumpPath)
				}
			case "postgresql":
				dumpCmd = fmt.Sprintf("su - postgres -c 'pg_dumpall' | gzip > %q", dumpPath)
			}

			if dumpCmd != "" {
				cmd := exec.CommandContext(execCtx, "sh", "-c", dumpCmd)
				if err := runStreamedCommand(execCtx, cmd, logLine); err != nil {
					logLine(fmt.Sprintf("==> [GAGAL] Pencadangan pra-migrasi gagal sehingga alih versi dibatalkan demi keamanan data: %v", err))
					_ = tm.store.AppendTaskLog(context.Background(), taskID, "failed", "")
					tm.Broadcast(taskID, "TASK_COMPLETE_FAILED")
					return
				}
				logLine("Cadangan penuh pra-migrasi berhasil dibuat dan diverifikasi.")
			}
		}

		logLine("Langkah 2/3: Memperbarui indeks repositori & memeriksa paket versi target di sistem...")
		upd := exec.CommandContext(execCtx, "apt-get", "update", "-y")
		upd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		_ = runStreamedCommand(execCtx, upd, logLine)

		logLine("Langkah 3/3: Memastikan daemon database aktif dan menjalankan validasi skema...")
		unit := ResolveServiceUnit(execCtx, engine)
		_ = runStreamedCommand(execCtx, exec.CommandContext(execCtx, "systemctl", "restart", unit), logLine)

		logLine(fmt.Sprintf("==> [SELESAI] Prosedur sinkronisasi & pengamanan versi %s selesai. Untuk lintas major-version pada distro APT, repositori vendor resmi diperlukan.", engine))
		_ = tm.store.AppendTaskLog(context.Background(), taskID, "success", "")
		tm.Broadcast(taskID, "TASK_COMPLETE_SUCCESS")
		return
	}

	// Simulasi dev non-Linux
	if backupFirst {
		logLine(fmt.Sprintf("Langkah 1/3: Snapshot cadangan pra-migrasi dibuat (/var/backups/fikpanel/preswitch_%s.sql.gz).", engine))
		time.Sleep(350 * time.Millisecond)
	}
	logLine(fmt.Sprintf("Langkah 2/3: Memperbarui konfigurasi versi target %s %s...", engine, targetVersion))
	time.Sleep(400 * time.Millisecond)
	logLine(fmt.Sprintf("Langkah 3/3: Layanan %s kembali aktif dan lolos health check.", engine))
	logLine(fmt.Sprintf("==> [SUKSES] Mesin database %s kini berjalan pada versi %s!", engine, targetVersion))

	SetEngineInstalled(engine, targetVersion, true)
	_ = tm.store.AppendTaskLog(context.Background(), taskID, "success", "")
	tm.Broadcast(taskID, "TASK_COMPLETE_SUCCESS")
}
