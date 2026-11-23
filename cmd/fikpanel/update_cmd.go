package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"fikpanel/internal/credentials"
	"fikpanel/internal/updater"
)

// updateArgs menampung opsi perintah `fikpanel update`.
type updateArgs struct {
	target       string
	check        bool
	status       bool
	force        bool
	foreground   bool
	worker       bool
	auto         bool
	help         bool
	runID        string
	restartDelay time.Duration
}

// parseUpdateArgs mengurai argumen secara manual agar opsi boleh ditulis sebelum maupun sesudah versi target.
func parseUpdateArgs(args []string) (updateArgs, error) {
	var ua updateArgs
	for i := 0; i < len(args); i++ {
		a := strings.TrimSpace(args[i])
		switch {
		case a == "":
			continue
		case a == "--check" || a == "-c":
			ua.check = true
		case a == "--status" || a == "-s":
			ua.status = true
		case a == "--force" || a == "-f":
			ua.force = true
		case a == "--foreground":
			ua.foreground = true
		case a == "--worker":
			ua.worker = true
		case a == "--auto":
			ua.auto = true
		case a == "--help" || a == "-h":
			ua.help = true
		case a == "--run-id" || a == "--restart-delay":
			if i+1 >= len(args) {
				return ua, fmt.Errorf("opsi %s membutuhkan nilai", a)
			}
			i++
			if err := ua.setValue(a, args[i]); err != nil {
				return ua, err
			}
		case strings.HasPrefix(a, "--run-id=") || strings.HasPrefix(a, "--restart-delay="):
			name, val, _ := strings.Cut(a, "=")
			if err := ua.setValue(name, val); err != nil {
				return ua, err
			}
		case strings.HasPrefix(a, "-"):
			return ua, fmt.Errorf("opsi tidak dikenal: %s", a)
		default:
			if ua.target != "" {
				return ua, fmt.Errorf("hanya satu versi target yang boleh diberikan (%s dan %s)", ua.target, a)
			}
			ua.target = a
		}
	}
	return ua, nil
}

func (ua *updateArgs) setValue(name, val string) error {
	switch name {
	case "--run-id":
		if !updater.ValidRunID(val) {
			return fmt.Errorf("run-id tidak valid: %q", val)
		}
		ua.runID = val
	case "--restart-delay":
		d, err := time.ParseDuration(val)
		if err != nil || d < 0 || d > time.Minute {
			return fmt.Errorf("restart-delay tidak valid: %q", val)
		}
		ua.restartDelay = d
	}
	return nil
}

func printUpdateHelp() {
	fmt.Print(`Penggunaan:
  fikpanel update                 Perbarui ke rilis stabil terbaru (restart & verifikasi otomatis)
  fikpanel update <versi>         Pasang versi tertentu, upgrade maupun downgrade (contoh: v0.3.1)
  fikpanel update --check         Cek ketersediaan rilis baru tanpa memasang
  fikpanel update --status        Tampilkan hasil pembaruan terakhir beserta log-nya
  fikpanel update --force         Pasang ulang walaupun versi target sudah terpasang
  fikpanel update --foreground    Jalankan langsung di sesi ini (hanya via SSH, bukan Web Terminal)

Alur pembaruan:
  1. Tentukan tag rilis target dan bandingkan dengan versi terpasang (tidak ada restart bila sudah sama)
  2. Unduh paket + SHA256SUMS.txt dari tag yang sama, tolak bila checksum tidak cocok
  3. Uji eksekusi biner baru dan pastikan versinya sesuai tag
  4. Cadangkan biner + database ke /opt/fikpanel/backups/updates (3 cadangan terakhir disimpan)
  5. Ganti biner secara atomik, restart fikpanel.service, lalu health check (proses + HTTP)
  6. Jika versi baru gagal sehat dalam 60 detik: rollback otomatis biner + database

Proses berjalan di unit systemd terpisah (fikpanel-updater.service), sehingga tetap selesai
walaupun dipicu dari Web Terminal yang ikut terputus saat panel di-restart.
`)
}

// runUpdate menangani perintah `fikpanel update`.
func runUpdate(args []string) {
	ua, err := parseUpdateArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n\n", err)
		printUpdateHelp()
		os.Exit(2)
	}
	switch {
	case ua.help:
		printUpdateHelp()
		return
	case ua.check:
		runUpdateCheck()
		return
	case ua.status:
		runUpdateStatus()
		return
	}

	if runtime.GOOS != "linux" {
		fmt.Printf("Pembaruan mandiri hanya tersedia di server Linux.\nVersi lokal saat ini: v%s\n", version)
		return
	}
	requireRoot("update")
	if ua.target == "" || ua.auto {
		ua.target = "latest"
	}

	if ua.worker {
		os.Exit(runUpdateWorker(ua))
	}

	fmt.Println("==========================================================")
	fmt.Println("             FikPanel Self-Update Utility                 ")
	fmt.Println("==========================================================")
	if ua.foreground || !updater.SystemdAvailable() {
		if updater.InsideUnitCgroup(updater.ServiceUnit) {
			fmt.Fprintln(os.Stderr, "[!] --foreground tidak bisa dipakai dari Web Terminal karena prosesnya ikut berhenti saat panel di-restart.")
			fmt.Fprintln(os.Stderr, "    Jalankan `fikpanel update` tanpa --foreground, atau gunakan SSH.")
			os.Exit(1)
		}
		os.Exit(runUpdateWorker(ua))
	}
	os.Exit(launchUpdateWorker(ua))
}

// launchUpdateWorker menjalankan worker di unit systemd transient lalu menampilkan log-nya secara langsung.
func launchUpdateWorker(ua updateArgs) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Gagal menentukan lokasi biner: %v\n", err)
		return 1
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	fromWebTerminal := updater.InsideUnitCgroup(updater.ServiceUnit)
	runID := updater.NewRunID(time.Now())
	workerArgs := []string{"update", "--worker", "--run-id", runID}
	if ua.force {
		workerArgs = append(workerArgs, "--force")
	}
	if fromWebTerminal {
		workerArgs = append(workerArgs, "--restart-delay", "3s")
	}
	workerArgs = append(workerArgs, ua.target)

	sys := updater.NewSystem()
	if err := updater.LaunchDetached(ctx, sys, exe, workerArgs); err != nil {
		if errors.Is(err, updater.ErrLocked) {
			fmt.Fprintln(os.Stderr, "[!] Proses pembaruan lain sedang berjalan. Pantau dengan: fikpanel update --status")
		} else {
			fmt.Fprintf(os.Stderr, "[!] Gagal memulai proses pembaruan: %v\n", err)
		}
		return 1
	}
	fmt.Printf("[*] Proses pembaruan berjalan di unit terpisah %s (run %s)\n\n", updater.WorkerUnit, runID)

	st, err := updater.Follow(ctx, sys, updater.DefaultStateDir, runID, os.Stdout, 20*time.Minute, fromWebTerminal)
	if err != nil {
		if ctx.Err() != nil {
			fmt.Println("\n[i] Pemantauan dihentikan; proses pembaruan tetap berjalan di latar belakang.")
			fmt.Println("    Lihat hasilnya dengan: fikpanel update --status")
			return 1
		}
		fmt.Fprintf(os.Stderr, "\n[!] %v\n", err)
		return 1
	}
	if !st.Terminal() {
		fmt.Println("\n[i] FikPanel sedang di-restart ke versi baru; koneksi Web Terminal akan terputus sebentar.")
		fmt.Println("    Muat ulang halaman dalam ±20 detik, lalu jalankan `fikpanel update --status` untuk hasil akhir")
		fmt.Println("    (termasuk rollback otomatis bila versi baru gagal berjalan).")
		return 0
	}
	fmt.Println()
	printUpdateSummary(st)
	if !st.OK() {
		return 1
	}
	return 0
}

// runUpdateWorker menjalankan alur pembaruan di proses ini (dipanggil oleh unit transient, timer, atau --foreground).
func runUpdateWorker(ua updateArgs) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 25*time.Minute)
	defer cancel()

	runID := ua.runID
	if runID == "" {
		runID = updater.NewRunID(time.Now())
	}

	sys := updater.NewSystem()
	paths := updater.DetectPaths(ctx, sys)
	if ua.auto && updater.AutoUpdateDisabledByAdmin(paths.StateDir) {
		fmt.Println("[i] Auto-update dinonaktifkan oleh admin (fikpanel auto-update disable). Tidak ada tindakan.")
		return 0
	}

	var out io.Writer = os.Stdout
	logPath := updater.LogPath(paths.StateDir, runID)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err == nil {
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			defer f.Close()
			out = io.MultiWriter(os.Stdout, f)
		}
	}

	u := updater.New(paths, updater.NewClient("FikPanel-Updater/"+version), sys, runtime.GOARCH, out)
	u.RestartDelay = ua.restartDelay
	res := u.Run(ctx, updater.Options{Target: ua.target, Force: ua.force, Auto: ua.auto, RunID: runID})
	if res.State == updater.StateSuccess || res.State == updater.StateNoop {
		return 0
	}
	return 1
}

// runUpdateStatus menampilkan hasil pembaruan terakhir (manual maupun auto-update).
func runUpdateStatus() {
	fmt.Printf("[*] Versi biner CLI : v%s\n", version)
	st, err := updater.ReadStatus(updater.DefaultStateDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("[i] Belum ada riwayat pembaruan dari updater ini.")
		} else {
			fmt.Fprintf(os.Stderr, "[!] Gagal membaca status pembaruan: %v\n", err)
		}
		return
	}
	printUpdateSummary(st)
	if st.LogPath != "" {
		if lines := tailLines(st.LogPath, 25); len(lines) > 0 {
			fmt.Printf("\n--- %d baris terakhir log (%s) ---\n", len(lines), st.LogPath)
			for _, l := range lines {
				fmt.Println(l)
			}
		}
	}
}

func printUpdateSummary(st updater.Status) {
	labels := map[string]string{
		updater.StateRunning:    "SEDANG BERJALAN",
		updater.StateSuccess:    "BERHASIL",
		updater.StateNoop:       "TIDAK ADA PERUBAHAN",
		updater.StateFailed:     "GAGAL",
		updater.StateRolledBack: "GAGAL - DIKEMBALIKAN KE VERSI SEBELUMNYA",
	}
	label := labels[st.State]
	if label == "" {
		label = st.State
	}
	if st.State == updater.StateRunning {
		if st.Phase != "" {
			label += " (" + st.Phase + ")"
		}
		if time.Since(time.Unix(st.StartedAt, 0)) > 30*time.Minute {
			label += " - lebih dari 30 menit, kemungkinan proses terhenti"
		}
	}
	trigger := "manual"
	if st.Trigger == "auto" {
		trigger = "otomatis (timer auto-update)"
	}

	fmt.Println("==========================================================")
	fmt.Printf("Status     : %s\n", label)
	if st.From != "" || st.To != "" {
		fmt.Printf("Versi      : %s -> %s\n", orUnknown(st.From), orUnknown(st.To))
	}
	fmt.Printf("Pemicu     : %s\n", trigger)
	if st.StartedAt > 0 {
		fmt.Printf("Mulai      : %s\n", time.Unix(st.StartedAt, 0).Format("2006-01-02 15:04:05"))
	}
	if st.FinishedAt > 0 {
		fmt.Printf("Selesai    : %s\n", time.Unix(st.FinishedAt, 0).Format("2006-01-02 15:04:05"))
	}
	if st.Message != "" {
		fmt.Printf("Keterangan : %s\n", st.Message)
	}
	fmt.Println("==========================================================")
}

func orUnknown(v string) string {
	if v == "" {
		return "(tidak diketahui)"
	}
	return "v" + v
}

func tailLines(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	return lines
}

func requireRoot(cmd string) {
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "[!] Perintah ini wajib dijalankan sebagai root: sudo fikpanel %s\n", cmd)
		os.Exit(1)
	}
}

func printAutoUpdateHelp() {
	fmt.Print(`Penggunaan:
  fikpanel auto-update status     Tampilkan status timer auto-update dan hasil pembaruan terakhir
  fikpanel auto-update enable     Aktifkan pembaruan otomatis (cek rilis stabil setiap jam)
  fikpanel auto-update disable    Nonaktifkan pembaruan otomatis (tetap nonaktif setelah update)

Auto-update memakai alur yang sama dengan "fikpanel update": verifikasi checksum, cadangan
biner + database, health check, dan rollback otomatis. Hanya rilis stabil yang dipasang,
dan layanan yang sengaja dihentikan admin tidak akan dinyalakan.
Jadwal dapat diubah dengan: sudo systemctl edit fikpanel-autoupdate.timer
`)
}

// runAutoUpdate mengelola timer systemd pembaruan otomatis.
func runAutoUpdate(args []string) {
	sub := "status"
	if len(args) > 0 {
		sub = strings.ToLower(strings.TrimSpace(args[0]))
	}
	if sub == "help" || sub == "--help" || sub == "-h" {
		printAutoUpdateHelp()
		return
	}
	if runtime.GOOS != "linux" || !updater.SystemdAvailable() {
		fmt.Fprintln(os.Stderr, "[!] Auto-update membutuhkan server Linux dengan systemd.")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	sys := updater.NewSystem()
	paths := updater.DetectPaths(ctx, sys)

	switch sub {
	case "enable", "on":
		requireRoot("auto-update enable")
		if err := updater.EnableAutoUpdate(ctx, sys, paths.BinPath, paths.StateDir); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Gagal mengaktifkan auto-update: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[✓] Auto-update aktif. FikPanel memeriksa rilis stabil baru setiap jam (jeda acak hingga 30 menit)")
		fmt.Println("    dan memasangnya dengan verifikasi checksum, health check, serta rollback otomatis.")
	case "disable", "off":
		requireRoot("auto-update disable")
		if err := updater.DisableAutoUpdate(ctx, sys, paths.StateDir); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Gagal menonaktifkan auto-update: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[✓] Auto-update dinonaktifkan. Pembaruan manual tetap tersedia: sudo fikpanel update")
	case "status":
		info := updater.AutoUpdateStatus(ctx, sys, paths.StateDir)
		state := "NONAKTIF"
		if info.Enabled && info.Active {
			state = "AKTIF"
		}
		fmt.Printf("Auto-update        : %s\n", state)
		if info.DisabledByAdmin {
			fmt.Println("Catatan            : dinonaktifkan oleh admin (aktifkan: sudo fikpanel auto-update enable)")
		} else if !info.Installed {
			fmt.Println("Catatan            : timer belum terpasang (aktifkan: sudo fikpanel auto-update enable)")
		}
		if info.NextRun != "" {
			fmt.Printf("Pemeriksaan berikut: %s\n", info.NextRun)
		}
		if info.LastRun != "" {
			fmt.Printf("Pemeriksaan terakhir: %s\n", info.LastRun)
		}
		fmt.Println()
		runUpdateStatus()
	default:
		fmt.Fprintf(os.Stderr, "[!] Sub-perintah tidak dikenal: %s\n\n", sub)
		printAutoUpdateHelp()
		os.Exit(2)
	}
}

// runPostUpdate dijalankan updater memakai biner BARU sebelum layanan di-restart, sehingga setiap
// rilis dapat menyiapkan kebutuhan sistemnya sendiri (saat ini: memasang/menyegarkan timer auto-update).
// Argumen --from/--to diterima untuk kebutuhan migrasi rilis berikutnya.
func runPostUpdate(_ []string) {
	if runtime.GOOS != "linux" || !updater.SystemdAvailable() {
		return
	}
	requireRoot("post-update")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	sys := updater.NewSystem()
	paths := updater.DetectPaths(ctx, sys)
	msg, err := updater.EnsureAutoUpdate(ctx, sys, paths.BinPath, paths.StateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[post-update] Gagal menyiapkan auto-update: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[post-update] %s\n", msg)
	printSecurityNotices(paths.DBPath)
}

// printSecurityNotices memberi tahu admin tentang perubahan keamanan yang baru diterapkan oleh
// layanan versi baru (kata sandi kosong yang diganti, HTTPS yang baru diaktifkan).
func printSecurityNotices(dbPath string) {
	recent := func(p string) bool {
		fi, err := os.Stat(p)
		return err == nil && time.Since(fi.ModTime()) < 15*time.Minute
	}
	credPath := credentials.PathFor(dbPath)
	if data, err := os.ReadFile(credPath); err == nil && recent(credPath) && strings.Contains(string(data), "diganti otomatis") {
		fmt.Println("[post-update] PENTING: akun admin yang sebelumnya TANPA kata sandi kini memakai kata sandi acak.")
		fmt.Printf("[post-update]   Lihat kata sandinya: sudo cat %s  (atau reset: sudo fikpanel reset-password)\n", credPath)
	}
	if cert := filepath.Join(filepath.Dir(credPath), "tls", "panel.crt"); recent(cert) {
		fmt.Println("[post-update] Panel kini melayani HTTPS (sertifikat self-signed). Buka https://<IP-server>:1405;")
		fmt.Println("[post-update]   browser akan menampilkan peringatan sertifikat sekali. Alamat http:// otomatis dialihkan.")
	}
}
