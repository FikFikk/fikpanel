package updater

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// WorkerUnit adalah nama unit systemd transient tempat proses pembaruan berjalan.
// Unit terpisah membuat proses pembaruan tidak ikut terbunuh saat fikpanel.service di-restart,
// termasuk ketika perintah dipicu dari Web Terminal (yang berada di cgroup fikpanel.service).
const WorkerUnit = "fikpanel-updater.service"

// LaunchDetached menjalankan `exe args...` sebagai unit systemd transient WorkerUnit.
func LaunchDetached(ctx context.Context, sys System, exe string, args []string) error {
	if st, err := sys.ServiceState(ctx, WorkerUnit); err == nil && unitBusy(st.ActiveState) {
		return ErrLocked
	}
	_ = sys.Systemctl(ctx, "reset-failed", WorkerUnit)

	base := []string{
		"--unit=" + WorkerUnit,
		"--description=FikPanel self-update",
		"--quiet",
		"--collect",
	}
	run := func(opts []string) ([]byte, error) {
		full := make([]string, 0, len(opts)+1+len(args))
		full = append(full, opts...)
		full = append(full, exe)
		full = append(full, args...)
		return exec.CommandContext(ctx, "systemd-run", full...).CombinedOutput()
	}
	out, err := run(base)
	if err != nil && strings.Contains(string(out), "--collect") {
		// systemd < 236 belum mengenal --collect.
		out, err = run(base[:len(base)-1])
	}
	if err != nil {
		return fmt.Errorf("systemd-run gagal: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func unitBusy(state string) bool {
	switch state {
	case "active", "activating", "deactivating", "reloading":
		return true
	}
	return false
}

// Follow menampilkan log run tertentu secara langsung sampai statusnya final.
// Jika stopAtRestart true, Follow kembali lebih awal saat worker memasuki fase restart
// (dipakai Web Terminal agar respons HTTP terkirim sebelum layanan di-restart).
func Follow(ctx context.Context, sys System, stateDir, runID string, out io.Writer, timeout time.Duration, stopAtRestart bool) (Status, error) {
	logPath := LogPath(stateDir, runID)
	started := time.Now()
	deadline := started.Add(timeout)
	var offset int64

	for {
		offset = copyFrom(logPath, offset, out)
		if st, err := ReadStatus(stateDir); err == nil && st.RunID == runID {
			if st.Terminal() || (stopAtRestart && st.Phase == PhaseRestarting) {
				copyFrom(logPath, offset, out)
				return st, nil
			}
		}
		if time.Since(started) > 5*time.Second {
			if ws, err := sys.ServiceState(ctx, WorkerUnit); err == nil && !unitBusy(ws.ActiveState) {
				time.Sleep(500 * time.Millisecond)
				copyFrom(logPath, offset, out)
				if st, err := ReadStatus(stateDir); err == nil && st.RunID == runID && st.Terminal() {
					return st, nil
				}
				return Status{}, fmt.Errorf("proses pembaruan berhenti tanpa status akhir; periksa: journalctl -u %s --no-pager", WorkerUnit)
			}
		}
		if time.Now().After(deadline) {
			return Status{}, fmt.Errorf("pembaruan masih berjalan di latar belakang setelah %s; pantau dengan: fikpanel update --status", timeout)
		}
		if err := sleepCtx(ctx, 500*time.Millisecond); err != nil {
			return Status{}, err
		}
	}
}

func copyFrom(path string, offset int64, out io.Writer) int64 {
	f, err := os.Open(path)
	if err != nil {
		return offset
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset
	}
	n, _ := io.Copy(out, f)
	return offset + n
}
