package updater

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Lokasi default instalasi produksi (sesuai scripts/install.sh).
const (
	DefaultInstallDir = "/opt/fikpanel"
	DefaultBinPath    = DefaultInstallDir + "/bin/fikpanel"
	DefaultLinkPath   = "/usr/local/bin/fikpanel"
	DefaultStateDir   = DefaultInstallDir + "/data/update"
	DefaultBackupDir  = DefaultInstallDir + "/backups/updates"
	DefaultDBPath     = DefaultInstallDir + "/data/fikpanel.db"
	DefaultPort       = 1405
	ServiceUnit       = "fikpanel.service"
	// backupsToKeep adalah jumlah set cadangan (biner + database) yang disimpan.
	backupsToKeep = 3
	logsToKeep    = 10
)

var (
	backupDirPattern = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}-v`)
	logFilePattern   = regexp.MustCompile(`^update-[0-9A-Za-z-]+\.log$`)
)

// Paths berisi lokasi berkas yang disentuh updater.
type Paths struct {
	BinPath   string
	LinkPath  string
	StateDir  string
	BackupDir string
	DBPath    string
	Port      int
	Service   string
}

// DetectPaths membaca unit fikpanel.service (bila ada) untuk mengetahui path biner, database, dan port aktual.
func DetectPaths(ctx context.Context, sys System) Paths {
	p := Paths{
		BinPath:   DefaultBinPath,
		LinkPath:  DefaultLinkPath,
		StateDir:  DefaultStateDir,
		BackupDir: DefaultBackupDir,
		DBPath:    DefaultDBPath,
		Port:      DefaultPort,
		Service:   ServiceUnit,
	}
	unitFile := "/etc/systemd/system/" + ServiceUnit
	if st, err := sys.ServiceState(ctx, ServiceUnit); err == nil && st.FragmentPath != "" {
		unitFile = st.FragmentPath
	}
	if cfg, err := ParseServiceConfig(unitFile); err == nil {
		if cfg.BinPath != "" && filepath.IsAbs(cfg.BinPath) {
			p.BinPath = cfg.BinPath
			// Ganti berkas aslinya, bukan symlink yang ditunjuk ExecStart.
			if resolved, err := filepath.EvalSymlinks(cfg.BinPath); err == nil {
				p.BinPath = resolved
			}
		}
		if cfg.DBPath != "" && filepath.IsAbs(cfg.DBPath) {
			p.DBPath = cfg.DBPath
		}
		if cfg.Port > 0 {
			p.Port = cfg.Port
		}
	}
	if p.LinkPath == p.BinPath {
		p.LinkPath = ""
	}
	return p
}

// Options mengatur satu eksekusi pembaruan.
type Options struct {
	// Target berisi "latest" atau tag spesifik (v0.3.20 / 0.3.20).
	Target string
	// Force memasang ulang walaupun versi target sama dengan versi terpasang.
	Force bool
	// Auto menandakan pemicu dari timer auto-update (tidak menyalakan layanan yang sengaja dihentikan admin).
	Auto bool
	// RunID mengidentifikasi eksekusi ini pada status.json dan berkas log.
	RunID string
}

// Result adalah hasil akhir satu eksekusi pembaruan.
type Result struct {
	State   string
	From    string
	To      string
	Message string
}

// Updater menjalankan alur pembaruan aman.
type Updater struct {
	Paths         Paths
	Client        *Client
	Sys           System
	Arch          string
	Log           io.Writer
	HealthTimeout time.Duration
	StableFor     time.Duration
	PollInterval  time.Duration
	// RestartDelay memberi jeda sebelum restart agar klien Web Terminal sempat menerima respons HTTP.
	RestartDelay time.Duration

	probe  func(ctx context.Context, port int) error
	now    func() time.Time
	status *Status
}

// New membuat Updater dengan parameter health check produksi.
func New(paths Paths, client *Client, sys System, arch string, log io.Writer) *Updater {
	return &Updater{
		Paths:         paths,
		Client:        client,
		Sys:           sys,
		Arch:          arch,
		Log:           log,
		HealthTimeout: 60 * time.Second,
		StableFor:     8 * time.Second,
		PollInterval:  time.Second,
		probe:         httpProbe,
		now:           time.Now,
	}
}

func (u *Updater) logf(format string, args ...any) {
	fmt.Fprintf(u.Log, "[%s] %s\n", u.now().Format("15:04:05"), fmt.Sprintf(format, args...))
}

// Run mengeksekusi pembaruan dengan kunci eksklusif dan mencatat hasilnya ke status.json.
func (u *Updater) Run(ctx context.Context, opts Options) Result {
	if err := os.MkdirAll(u.Paths.StateDir, 0o755); err != nil {
		msg := fmt.Sprintf("Gagal membuat direktori state %s: %v. Tidak ada perubahan yang dilakukan.", u.Paths.StateDir, err)
		u.logf("%s", msg)
		return Result{State: StateFailed, Message: msg}
	}
	release, err := acquireLock(filepath.Join(u.Paths.StateDir, ".lock"))
	if err != nil {
		msg := fmt.Sprintf("Tidak dapat memulai pembaruan: %v.", err)
		if errors.Is(err, ErrLocked) {
			msg += " Pantau dengan: fikpanel update --status"
		}
		u.logf("%s", msg)
		return Result{State: StateFailed, Message: msg}
	}
	defer release()

	trigger := "manual"
	if opts.Auto {
		trigger = "auto"
	}
	st := Status{
		RunID:     opts.RunID,
		State:     StateRunning,
		Trigger:   trigger,
		Target:    opts.Target,
		StartedAt: u.now().Unix(),
	}
	if opts.RunID != "" {
		st.LogPath = LogPath(u.Paths.StateDir, opts.RunID)
	}
	u.status = &st
	defer func() { u.status = nil }()
	u.setPhase(PhaseResolving)

	res := u.run(ctx, opts)

	st.State, st.From, st.To, st.Message = res.State, res.From, res.To, res.Message
	st.Phase = ""
	st.FinishedAt = u.now().Unix()
	if err := writeStatus(u.Paths.StateDir, st); err != nil {
		u.logf("Peringatan: gagal menyimpan status pembaruan: %v", err)
	}
	pruneEntries(filepath.Join(u.Paths.StateDir, "logs"), logFilePattern, logsToKeep)
	// Biner staging dari scripts/update.sh tidak diperlukan lagi (proses yang sedang berjalan tetap aman di Linux).
	_ = os.RemoveAll(filepath.Join(u.Paths.StateDir, "bootstrap"))
	u.logf("%s", res.Message)
	return res
}

func (u *Updater) setPhase(phase string) {
	if u.status == nil {
		return
	}
	u.status.Phase = phase
	_ = writeStatus(u.Paths.StateDir, *u.status)
}

// beforeRestart menandai fase restart dan memberi jeda opsional untuk klien Web Terminal.
func (u *Updater) beforeRestart(ctx context.Context) error {
	u.setPhase(PhaseRestarting)
	if u.RestartDelay <= 0 {
		return nil
	}
	u.logf("Layanan akan di-restart dalam %s...", u.RestartDelay)
	return sleepCtx(ctx, u.RestartDelay)
}

func (u *Updater) fail(from, to, format string, args ...any) Result {
	return Result{State: StateFailed, From: from, To: to, Message: fmt.Sprintf(format, args...)}
}

func (u *Updater) run(ctx context.Context, opts Options) Result {
	target := strings.TrimSpace(opts.Target)
	if target == "" {
		target = "latest"
	}

	// 1. Tentukan tag target secara eksplisit (bukan sekadar URL /latest/download) agar versi hasil dapat diverifikasi.
	var tag string
	if target == "latest" {
		u.logf("Menentukan rilis stabil terbaru dari GitHub...")
		t, err := u.Client.ResolveLatestTag(ctx)
		if err != nil {
			return u.fail("", "", "Gagal menentukan rilis terbaru: %v. Tidak ada perubahan yang dilakukan.", err)
		}
		tag = t
	} else {
		t, err := NormalizeTag(target)
		if err != nil {
			return u.fail("", "", "%v", err)
		}
		tag = t
	}
	toVer := TagVersion(tag)

	// Rilis yang pernah gagal health check tidak dicoba ulang tiap jam oleh auto-update.
	if opts.Auto && !opts.Force && fileExists(u.rolledBackMarker(tag)) {
		return Result{State: StateNoop, To: toVer,
			Message: fmt.Sprintf("Rilis %s sebelumnya gagal health check dan di-rollback; auto-update menunggu rilis berikutnya. Paksa manual: sudo fikpanel update %s", tag, tag)}
	}

	// 2. Periksa versi terpasang dan status layanan.
	fromVer := binaryVersion(ctx, u.Paths.BinPath)
	svc, svcErr := u.Sys.ServiceState(ctx, u.Paths.Service)
	managed := svcErr == nil && svc.LoadState == "loaded"
	wasActive := managed && svc.ActiveState == "active"
	u.logf("Versi terpasang: %s | Target: %s | Layanan: %s", displayVersion(fromVer), tag, describeService(managed, svc))

	if fromVer != "" && !opts.Force {
		cmp := CompareVersions(fromVer, toVer)
		if cmp > 0 && target == "latest" {
			return Result{State: StateNoop, From: fromVer, To: fromVer,
				Message: fmt.Sprintf("Versi terpasang v%s lebih baru dari rilis stabil terbaru %s. Tidak ada perubahan.", fromVer, tag)}
		}
		if cmp == 0 {
			return u.ensureRunning(ctx, opts, svc, managed, fromVer)
		}
	}

	// 3. Unduh arsip + SHA256SUMS dari tag yang sama, lalu verifikasi checksum.
	u.setPhase(PhaseDownloading)
	work := filepath.Join(u.Paths.StateDir, "download", tag)
	_ = os.RemoveAll(work)
	if err := os.MkdirAll(work, 0o700); err != nil {
		return u.fail(fromVer, toVer, "Gagal menyiapkan direktori unduhan: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(work)
		_ = os.Remove(filepath.Dir(work)) // hanya terhapus bila sudah kosong
	}()

	asset := AssetName(u.Arch)
	sumsPath := filepath.Join(work, ChecksumsAsset)
	u.logf("Mengunduh %s untuk %s...", ChecksumsAsset, tag)
	if err := u.Client.Download(ctx, tag, ChecksumsAsset, sumsPath, maxChecksumsBytes); err != nil {
		return u.fail(fromVer, toVer, "Gagal mengunduh daftar checksum: %v.%s Tidak ada perubahan yang dilakukan.", err, notFoundHint(err))
	}
	sumsData, err := os.ReadFile(sumsPath)
	if err != nil {
		return u.fail(fromVer, toVer, "Gagal membaca %s: %v", ChecksumsAsset, err)
	}
	expected := ParseChecksums(sumsData)[asset]
	if expected == "" {
		return u.fail(fromVer, toVer, "%s pada rilis %s tidak memuat checksum untuk %s. Tidak ada perubahan yang dilakukan.", ChecksumsAsset, tag, asset)
	}

	archivePath := filepath.Join(work, asset)
	u.logf("Mengunduh %s...", asset)
	if err := u.Client.Download(ctx, tag, asset, archivePath, maxArchiveBytes); err != nil {
		return u.fail(fromVer, toVer, "Gagal mengunduh paket rilis: %v.%s Tidak ada perubahan yang dilakukan.", err, notFoundHint(err))
	}
	actual, err := FileSHA256(archivePath)
	if err != nil {
		return u.fail(fromVer, toVer, "Gagal menghitung checksum paket: %v", err)
	}
	if actual != expected {
		return u.fail(fromVer, toVer, "Checksum SHA-256 paket tidak cocok (diharapkan %s, didapat %s). Paket dibuang, tidak ada perubahan.", expected, actual)
	}
	u.logf("Checksum SHA-256 cocok: %s", actual)

	// 4. Ekstrak ke berkas staging di direktori yang sama dengan biner aktif (syarat rename atomik).
	binDir := filepath.Dir(u.Paths.BinPath)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return u.fail(fromVer, toVer, "Gagal menyiapkan direktori biner: %v", err)
	}
	staged := filepath.Join(binDir, ".fikpanel.new")
	if err := ExtractBinary(archivePath, BinaryName, staged, maxBinaryBytes); err != nil {
		return u.fail(fromVer, toVer, "Gagal mengekstrak biner: %v. Tidak ada perubahan yang dilakukan.", err)
	}
	defer os.Remove(staged)

	// 5. Uji eksekusi biner baru dan pastikan versinya sesuai tag.
	if got := binaryVersion(ctx, staged); got != toVer {
		return u.fail(fromVer, toVer, "Biner baru melaporkan versi %q, bukan %s (arsitektur salah atau paket rusak). Tidak ada perubahan yang dilakukan.", got, tag)
	}
	u.logf("Biner baru lolos uji eksekusi (v%s).", toVer)

	// 6. Cadangkan biner dan database sebelum menyentuh apa pun.
	u.setPhase(PhaseBackup)
	backup, err := u.createBackup(ctx, fromVer)
	if err != nil {
		return u.fail(fromVer, toVer, "Gagal membuat cadangan: %v. Pembaruan dibatalkan, tidak ada perubahan.", err)
	}
	u.logf("Cadangan biner & database tersimpan di %s", backup.Dir)

	// 7. Ganti biner secara atomik (rename(2) di filesystem yang sama).
	u.setPhase(PhaseInstalling)
	if err := os.Rename(staged, u.Paths.BinPath); err != nil {
		return u.fail(fromVer, toVer, "Gagal memasang biner baru: %v. Tidak ada perubahan.", err)
	}
	u.logf("Biner v%s terpasang di %s", toVer, u.Paths.BinPath)
	if err := u.ensureLink(); err != nil {
		u.logf("Peringatan: gagal memperbarui symlink %s: %v", u.Paths.LinkPath, err)
	}

	if !managed {
		u.runHook(ctx, u.Paths.BinPath, fromVer, toVer)
		return Result{State: StateSuccess, From: fromVer, To: toVer,
			Message: fmt.Sprintf("Biner diperbarui ke v%s. Unit %s tidak ditemukan, jalankan ulang proses FikPanel secara manual.", toVer, u.Paths.Service)}
	}
	if !wasActive && opts.Auto {
		u.runHook(ctx, u.Paths.BinPath, fromVer, toVer)
		return Result{State: StateSuccess, From: fromVer, To: toVer,
			Message: fmt.Sprintf("Biner diperbarui ke v%s. Layanan sedang dihentikan oleh admin sehingga tidak dinyalakan.", toVer)}
	}

	// 8. Restart dan verifikasi; rollback otomatis jika layanan tidak sehat.
	if err := u.beforeRestart(ctx); err != nil {
		return u.fail(fromVer, toVer, "Pembaruan dibatalkan sebelum restart: %v. Biner v%s sudah terpasang; jalankan ulang `fikpanel update` untuk me-restart.", err, toVer)
	}
	u.logf("Me-restart %s...", u.Paths.Service)
	if err := u.restartAndVerify(ctx); err != nil {
		u.logf("Versi baru tidak sehat: %v", err)
		u.logf("Memulai rollback ke %s...", displayVersion(fromVer))
		u.setPhase(PhaseRollingBack)
		u.markRolledBack(tag)
		if rbErr := u.rollback(ctx, backup); rbErr != nil {
			return u.fail(fromVer, toVer, "Pembaruan ke %s gagal (%v) dan rollback juga gagal (%v). Cadangan tersedia di %s. Periksa: journalctl -u fikpanel -n 50 --no-pager", tag, err, rbErr, backup.Dir)
		}
		return Result{State: StateRolledBack, From: fromVer, To: fromVer,
			Message: fmt.Sprintf("Pembaruan ke %s gagal (%v). FikPanel dikembalikan ke %s beserta database-nya dan sudah berjalan normal. Auto-update melewati %s sampai rilis berikutnya terbit.", tag, err, displayVersion(fromVer), tag)}
	}
	pruneEntries(u.Paths.BackupDir, backupDirPattern, backupsToKeep)
	_ = os.Remove(u.rolledBackMarker(tag))

	// 9. Versi baru terbukti sehat: beri kesempatan menyiapkan kebutuhannya (timer auto-update, dll.).
	u.runHook(ctx, u.Paths.BinPath, fromVer, toVer)

	if fromVer != "" && CompareVersions(toVer, fromVer) < 0 && AutoUpdateEnabled(ctx, u.Sys) {
		u.logf("Catatan: auto-update aktif dan akan menaikkan kembali ke rilis terbaru. Nonaktifkan dengan: sudo fikpanel auto-update disable")
	}
	return Result{State: StateSuccess, From: fromVer, To: toVer,
		Message: fmt.Sprintf("FikPanel berhasil diperbarui dari %s ke v%s dan layanan sudah menjalankan versi baru.", displayVersion(fromVer), toVer)}
}

// ensureRunning menangani kasus biner di disk sudah versi target: pastikan layanan juga menjalankannya.
func (u *Updater) ensureRunning(ctx context.Context, opts Options, svc ServiceState, managed bool, ver string) Result {
	if !managed {
		return Result{State: StateNoop, From: ver, To: ver, Message: fmt.Sprintf("FikPanel sudah berada di v%s. Tidak ada perubahan.", ver)}
	}
	if svc.ActiveState != "active" && opts.Auto {
		return Result{State: StateNoop, From: ver, To: ver,
			Message: fmt.Sprintf("FikPanel sudah v%s; layanan sedang dihentikan oleh admin sehingga tidak disentuh.", ver)}
	}
	if svc.ActiveState == "active" && u.isCurrentProcess(svc.MainPID) {
		return Result{State: StateNoop, From: ver, To: ver,
			Message: fmt.Sprintf("FikPanel sudah berada di v%s dan layanan sudah menjalankan biner tersebut. Tidak ada perubahan.", ver)}
	}
	u.logf("Biner di disk sudah v%s, tetapi layanan belum menjalankannya.", ver)
	if err := u.beforeRestart(ctx); err != nil {
		return u.fail(ver, ver, "Restart dibatalkan: %v", err)
	}
	u.logf("Me-restart %s...", u.Paths.Service)
	if err := u.restartAndVerify(ctx); err != nil {
		return u.fail(ver, ver, "Restart layanan gagal: %v. Periksa: journalctl -u fikpanel -n 50 --no-pager", err)
	}
	return Result{State: StateSuccess, From: ver, To: ver, Message: fmt.Sprintf("Layanan di-restart dan kini menjalankan v%s.", ver)}
}

func (u *Updater) rolledBackMarker(tag string) string {
	return filepath.Join(u.Paths.StateDir, "rolled-back-"+tag)
}

func (u *Updater) markRolledBack(tag string) {
	msg := fmt.Sprintf("%s di-rollback pada %s\n", tag, u.now().Format(time.RFC3339))
	if err := writeFileAtomic(u.rolledBackMarker(tag), []byte(msg), 0o644); err != nil {
		u.logf("Peringatan: gagal mencatat rilis gagal %s: %v", tag, err)
	}
}

type backupSet struct {
	Dir string
	Bin string
	DB  string
}

func (u *Updater) createBackup(ctx context.Context, fromVer string) (*backupSet, error) {
	label := fromVer
	if label == "" {
		label = "unknown"
	}
	dir := filepath.Join(u.Paths.BackupDir, u.now().Format("20060102-150405")+"-v"+label)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	b := &backupSet{Dir: dir}
	if fileExists(u.Paths.BinPath) {
		dst := filepath.Join(dir, BinaryName)
		if err := copyFile(u.Paths.BinPath, dst, 0o755); err != nil {
			_ = os.RemoveAll(dir)
			return nil, fmt.Errorf("salin biner: %w", err)
		}
		b.Bin = dst
	}
	if fileExists(u.Paths.DBPath) {
		dst := filepath.Join(dir, filepath.Base(u.Paths.DBPath))
		if err := snapshotDatabase(ctx, u.Paths.DBPath, dst); err != nil {
			_ = os.RemoveAll(dir)
			return nil, fmt.Errorf("snapshot database: %w", err)
		}
		b.DB = dst
	}
	return b, nil
}

// snapshotDatabase membuat salinan konsisten database SQLite (aman walau layanan sedang menulis, mode WAL).
func snapshotDatabase(ctx context.Context, src, dst string) error {
	db, err := sql.Open("sqlite", src+"?_pragma=busy_timeout(15000)")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", dst); err != nil {
		return err
	}

	check, err := sql.Open("sqlite", dst)
	if err != nil {
		return err
	}
	defer check.Close()
	var res string
	if err := check.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&res); err != nil {
		return err
	}
	if res != "ok" {
		return fmt.Errorf("hasil snapshot tidak valid: %s", res)
	}
	return os.Chmod(dst, 0o600)
}

func (u *Updater) rollback(ctx context.Context, b *backupSet) error {
	if b == nil || b.Bin == "" {
		return errors.New("tidak ada cadangan biner untuk dipulihkan")
	}
	if err := u.Sys.Systemctl(ctx, "stop", u.Paths.Service); err != nil {
		u.logf("Peringatan: %v", err)
	}

	staged := filepath.Join(filepath.Dir(u.Paths.BinPath), ".fikpanel.rollback")
	if err := copyFile(b.Bin, staged, 0o755); err != nil {
		return fmt.Errorf("salin biner cadangan: %w", err)
	}
	if err := os.Rename(staged, u.Paths.BinPath); err != nil {
		_ = os.Remove(staged)
		return fmt.Errorf("pulihkan biner: %w", err)
	}
	u.logf("Biner lama dipulihkan.")

	if b.DB != "" {
		mode := os.FileMode(0o644)
		if fi, err := os.Stat(u.Paths.DBPath); err == nil {
			mode = fi.Mode().Perm()
		}
		tmp := u.Paths.DBPath + ".restore"
		if err := copyFile(b.DB, tmp, mode); err != nil {
			return fmt.Errorf("salin database cadangan: %w", err)
		}
		// WAL/SHM milik versi baru harus dibuang agar tidak diterapkan ke snapshot lama.
		_ = os.Remove(u.Paths.DBPath + "-wal")
		_ = os.Remove(u.Paths.DBPath + "-shm")
		if err := os.Rename(tmp, u.Paths.DBPath); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("pulihkan database: %w", err)
		}
		u.logf("Database dipulihkan dari snapshot sebelum pembaruan.")
	}
	if err := u.ensureLink(); err != nil {
		u.logf("Peringatan: gagal memperbarui symlink: %v", err)
	}

	if err := u.Sys.Systemctl(ctx, "start", u.Paths.Service); err != nil {
		return err
	}
	return u.waitHealthy(ctx, 0)
}

func (u *Updater) restartAndVerify(ctx context.Context) error {
	before, _ := u.Sys.ServiceState(ctx, u.Paths.Service)
	if err := u.Sys.Systemctl(ctx, "restart", u.Paths.Service); err != nil {
		return err
	}
	return u.waitHealthy(ctx, before.MainPID)
}

// waitHealthy menunggu hingga layanan aktif, menjalankan biner terbaru, dan merespons HTTP,
// lalu memastikan proses tetap stabil (tidak crash-loop) selama StableFor.
func (u *Updater) waitHealthy(ctx context.Context, oldPID int) error {
	deadline := u.now().Add(u.HealthTimeout)
	var st ServiceState
	for {
		var err error
		st, err = u.checkHealthy(ctx, oldPID)
		if err == nil {
			break
		}
		if !u.now().Before(deadline) {
			return fmt.Errorf("layanan tidak sehat dalam %s: %v", u.HealthTimeout, err)
		}
		if err := sleepCtx(ctx, u.PollInterval); err != nil {
			return err
		}
	}
	u.logf("Layanan merespons (PID %d). Memantau stabilitas selama %s...", st.MainPID, u.StableFor)
	if err := sleepCtx(ctx, u.StableFor); err != nil {
		return err
	}
	after, err := u.checkHealthy(ctx, oldPID)
	if err != nil {
		return fmt.Errorf("layanan tidak stabil setelah start: %v", err)
	}
	if after.MainPID != st.MainPID || after.NRestarts != st.NRestarts {
		return fmt.Errorf("layanan ter-restart sendiri (PID %d -> %d), kemungkinan crash", st.MainPID, after.MainPID)
	}
	return nil
}

func (u *Updater) checkHealthy(ctx context.Context, oldPID int) (ServiceState, error) {
	st, err := u.Sys.ServiceState(ctx, u.Paths.Service)
	if err != nil {
		return st, err
	}
	if st.ActiveState != "active" {
		return st, fmt.Errorf("status layanan %q", st.ActiveState)
	}
	if st.MainPID <= 0 {
		return st, errors.New("proses utama layanan belum tersedia")
	}
	if oldPID > 0 && st.MainPID == oldPID {
		return st, errors.New("proses lama masih berjalan")
	}
	if !u.isCurrentProcess(st.MainPID) {
		return st, errors.New("proses layanan belum menjalankan biner terbaru")
	}
	if err := u.probe(ctx, u.Paths.Port); err != nil {
		return st, err
	}
	return st, nil
}

// isCurrentProcess memastikan proses pid mengeksekusi berkas biner yang saat ini ada di BinPath
// (bukan inode lama yang sudah diganti, yang ditandai kernel dengan sufiks " (deleted)").
func (u *Updater) isCurrentProcess(pid int) bool {
	if pid <= 0 {
		return false
	}
	exe, err := u.Sys.ProcessExe(pid)
	if err != nil || strings.HasSuffix(exe, " (deleted)") {
		return false
	}
	want := u.Paths.BinPath
	if resolved, err := filepath.EvalSymlinks(want); err == nil {
		want = resolved
	}
	return exe == want || exe == u.Paths.BinPath
}

func (u *Updater) ensureLink() error {
	link := u.Paths.LinkPath
	if link == "" {
		return nil
	}
	if _, err := os.Stat(filepath.Dir(link)); err != nil {
		return nil
	}
	if target, err := os.Readlink(link); err == nil && target == u.Paths.BinPath {
		return nil
	}
	tmp := link + ".fikpanel-tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(u.Paths.BinPath, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (u *Updater) runHook(ctx context.Context, bin, from, to string) {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(hctx, bin, "post-update", "--from", from, "--to", to).CombinedOutput()
	if err != nil && strings.Contains(string(out), "Perintah tidak dikenal: post-update") {
		u.logf("Versi v%s belum memiliki langkah post-update; dilewati.", to)
		return
	}
	if text := strings.TrimSpace(string(out)); text != "" {
		fmt.Fprintln(u.Log, text)
	}
	if err != nil {
		u.logf("Peringatan: langkah post-update v%s gagal (%v). Pembaruan dilanjutkan.", to, err)
	}
}

// binaryVersion menjalankan `<bin> version` dan mengembalikan nomor versinya ("" jika gagal).
func binaryVersion(ctx context.Context, bin string) string {
	if !fileExists(bin) {
		return ""
	}
	vctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(vctx, bin, "version").Output()
	if err != nil {
		return ""
	}
	return ParseVersionOutput(string(out))
}

func httpProbe(ctx context.Context, port int) error {
	client := &http.Client{
		Timeout:       3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/api/auth/me", port), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("port %d belum merespons: %v", port, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("HTTP health check mengembalikan %s", resp.Status)
	}
	return nil
}

func notFoundHint(err error) string {
	if errors.Is(err, ErrReleaseNotFound) {
		return " Jika tag baru saja di-push, workflow GitHub Actions biasanya butuh 2-3 menit untuk mengunggah biner; coba lagi setelahnya."
	}
	return ""
}

func displayVersion(v string) string {
	if v == "" {
		return "(tidak diketahui)"
	}
	return "v" + v
}

func describeService(managed bool, st ServiceState) string {
	if !managed {
		return "tidak terdaftar di systemd"
	}
	return st.ActiveState
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	_ = os.Remove(dst)
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return os.Chmod(dst, perm)
}
