//go:build windows

package cron

import (
	"os/exec"
	"time"
)

// configureCommandProcessGroup mengatur batas tunggu pelepasan I/O saat perintah dihentikan pada Windows
func configureCommandProcessGroup(cmd *exec.Cmd) {
	cmd.WaitDelay = 5 * time.Second
}
