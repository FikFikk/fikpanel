//go:build !windows

package cron

import (
	"os/exec"
	"syscall"
	"time"
)

// configureCommandProcessGroup menempatkan perintah cron dalam process group tersendiri agar timeout/stop menghentikan seluruh anak proses
func configureCommandProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Kirim SIGKILL ke seluruh process group (-pid) agar subproses (mysqldump/tar/php) tidak menjadi orphan
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second
}
