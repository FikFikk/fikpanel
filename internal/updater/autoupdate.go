package updater

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Unit systemd untuk auto-update terjadwal.
const (
	AutoUpdateService = "fikpanel-autoupdate.service"
	AutoUpdateTimer   = "fikpanel-autoupdate.timer"
	unitDir           = "/etc/systemd/system"
	disabledMarker    = "autoupdate.disabled"
)

func autoUpdateServiceUnit(bin string) string {
	return fmt.Sprintf(`# Dikelola oleh FikPanel (fikpanel auto-update). Berkas ini ditulis ulang saat pembaruan;
# gunakan "systemctl edit %[1]s" untuk kustomisasi.
[Unit]
Description=FikPanel automatic update (stable releases)
Documentation=https://github.com/%[2]s
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=%[3]s update --worker --auto latest
TimeoutStartSec=30min
Nice=10
`, AutoUpdateService, DefaultRepo, bin)
}

func autoUpdateTimerUnit() string {
	return fmt.Sprintf(`# Dikelola oleh FikPanel (fikpanel auto-update). Berkas ini ditulis ulang saat pembaruan;
# gunakan "systemctl edit %[1]s" untuk mengubah jadwal.
[Unit]
Description=FikPanel automatic update check

[Timer]
OnCalendar=hourly
RandomizedDelaySec=30min
Persistent=true

[Install]
WantedBy=timers.target
`, AutoUpdateTimer)
}

// AutoUpdateDisabledByAdmin melaporkan apakah admin menonaktifkan auto-update secara eksplisit.
func AutoUpdateDisabledByAdmin(stateDir string) bool {
	_, err := os.Stat(filepath.Join(stateDir, disabledMarker))
	return err == nil
}

// AutoUpdateEnabled melaporkan apakah timer auto-update aktif di systemd.
func AutoUpdateEnabled(ctx context.Context, sys System) bool {
	return sys.Systemctl(ctx, "is-enabled", "--quiet", AutoUpdateTimer) == nil
}

// EnableAutoUpdate menulis (atau memperbarui) unit timer + service lalu mengaktifkannya.
func EnableAutoUpdate(ctx context.Context, sys System, bin, stateDir string) error {
	changed := false
	units := map[string]string{
		AutoUpdateService: autoUpdateServiceUnit(bin),
		AutoUpdateTimer:   autoUpdateTimerUnit(),
	}
	for name, content := range units {
		path := filepath.Join(unitDir, name)
		if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, []byte(content)) {
			continue
		}
		if err := writeFileAtomic(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("gagal menulis %s: %w", path, err)
		}
		changed = true
	}
	if changed {
		if err := sys.Systemctl(ctx, "daemon-reload"); err != nil {
			return err
		}
	}
	if err := sys.Systemctl(ctx, "enable", "--now", AutoUpdateTimer); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(stateDir, disabledMarker)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DisableAutoUpdate mematikan timer dan menyimpan penanda agar pembaruan berikutnya tidak menyalakannya lagi.
func DisableAutoUpdate(ctx context.Context, sys System, stateDir string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(stateDir, disabledMarker), []byte("disabled by admin\n"), 0o644); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(unitDir, AutoUpdateTimer)); os.IsNotExist(err) {
		return nil
	}
	return sys.Systemctl(ctx, "disable", "--now", AutoUpdateTimer)
}

// EnsureAutoUpdate dipanggil oleh `fikpanel post-update`: memasang/menyegarkan timer kecuali admin menonaktifkannya.
func EnsureAutoUpdate(ctx context.Context, sys System, bin, stateDir string) (string, error) {
	if AutoUpdateDisabledByAdmin(stateDir) {
		return "Auto-update dinonaktifkan oleh admin; timer tidak disentuh.", nil
	}
	if err := EnableAutoUpdate(ctx, sys, bin, stateDir); err != nil {
		return "", err
	}
	return "Auto-update aktif (pemeriksaan rilis setiap jam).", nil
}

// AutoUpdateInfo merangkum status auto-update untuk ditampilkan di CLI.
type AutoUpdateInfo struct {
	Installed       bool
	Enabled         bool
	Active          bool
	DisabledByAdmin bool
	NextRun         string
	LastRun         string
}

// AutoUpdateStatus membaca status timer auto-update dari systemd.
func AutoUpdateStatus(ctx context.Context, sys System, stateDir string) AutoUpdateInfo {
	info := AutoUpdateInfo{DisabledByAdmin: AutoUpdateDisabledByAdmin(stateDir)}
	if _, err := os.Stat(filepath.Join(unitDir, AutoUpdateTimer)); err == nil {
		info.Installed = true
	}
	info.Enabled = AutoUpdateEnabled(ctx, sys)
	info.Active = sys.Systemctl(ctx, "is-active", "--quiet", AutoUpdateTimer) == nil
	info.NextRun = systemctlValue(ctx, AutoUpdateTimer, "NextElapseUSecRealtime")
	info.LastRun = systemctlValue(ctx, AutoUpdateTimer, "LastTriggerUSec")
	return info
}

func systemctlValue(ctx context.Context, unit, prop string) string {
	out, err := exec.CommandContext(ctx, "systemctl", "show", unit, "-p", prop).Output()
	if err != nil {
		return ""
	}
	_, val, _ := strings.Cut(strings.TrimSpace(string(out)), "=")
	if val == "" || val == "0" || val == "n/a" {
		return ""
	}
	return val
}
