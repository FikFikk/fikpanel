//go:build !windows

package terminal

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"
)

type localUnixPTY struct {
	cmd  *exec.Cmd
	ptmx *os.File
	once sync.Once
}

// StartLocalPTY membuka shell Linux interaktif dengan pseudo-terminal (PTY) asli mulai di $HOME
func StartLocalPTY(homeDir string, cols, rows int) (PTYSession, error) {
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 32
	}

	shellPath := "/bin/bash"
	if p, err := exec.LookPath("bash"); err == nil && p != "" {
		shellPath = p
	} else if p, err := exec.LookPath("sh"); err == nil && p != "" {
		shellPath = p
	}

	var cmd *exec.Cmd
	if strings.HasSuffix(shellPath, "bash") {
		cmd = exec.Command(shellPath, "-i")
	} else {
		cmd = exec.Command(shellPath)
	}

	if strings.TrimSpace(homeDir) != "" {
		cmd.Dir = homeDir
	}

	// Siapkan environment standar sesi SSH interaktif (warna truecolor + HOME yang akurat)
	env := make([]string, 0, len(os.Environ())+5)
	hasHome := false
	hasLang := false
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "TERM=") || strings.HasPrefix(kv, "COLORTERM=") {
			continue
		}
		if strings.HasPrefix(kv, "HOME=") && strings.TrimSpace(kv[5:]) != "" {
			hasHome = true
		}
		if strings.HasPrefix(kv, "LANG=") {
			hasLang = true
		}
		env = append(env, kv)
	}
	env = append(env, "TERM=xterm-256color", "COLORTERM=truecolor")
	if !hasHome && homeDir != "" {
		env = append(env, "HOME="+homeDir)
	}
	if !hasLang {
		env = append(env, "LANG=C.UTF-8")
	}
	cmd.Env = env

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if err != nil {
		return nil, err
	}

	return &localUnixPTY{
		cmd:  cmd,
		ptmx: ptmx,
	}, nil
}

// StartDockerExecPTY membuka sesi shell interaktif dengan PTY asli ke dalam kontainer Docker target
func StartDockerExecPTY(containerID, shell string, cols, rows int) (PTYSession, error) {
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 32
	}

	cid := strings.TrimSpace(containerID)
	if cid == "" || strings.HasPrefix(cid, "-") || strings.ContainsAny(cid, " \t\r\n;|&$`<>\"'") {
		return nil, errors.New("ID atau nama kontainer tidak valid")
	}

	// Bangun argumen shell secara aman menggunakan allowlist agar bebas injeksi perintah
	args := []string{"exec", "-it", "-e", "TERM=xterm-256color", "-e", "COLORTERM=truecolor", cid}
	switch strings.TrimSpace(shell) {
	case "/bin/bash", "bash":
		args = append(args, "/bin/bash")
	case "/bin/sh", "sh":
		args = append(args, "/bin/sh")
	case "/bin/ash", "ash":
		args = append(args, "/bin/ash")
	case "/bin/zsh", "zsh":
		args = append(args, "/bin/zsh")
	default:
		args = append(args, "/bin/sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; elif command -v ash >/dev/null 2>&1; then exec ash; else exec sh; fi")
	}

	cmd := exec.Command("docker", args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if err != nil {
		return nil, err
	}

	return &localUnixPTY{
		cmd:  cmd,
		ptmx: ptmx,
	}, nil
}

func (l *localUnixPTY) Read(p []byte) (int, error)  { return l.ptmx.Read(p) }
func (l *localUnixPTY) Write(p []byte) (int, error) { return l.ptmx.Write(p) }

func (l *localUnixPTY) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return pty.Setsize(l.ptmx, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
}

func (l *localUnixPTY) Wait() (int, error) {
	err := l.cmd.Wait()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 1, err
}

func (l *localUnixPTY) Close() error {
	var closeErr error
	l.once.Do(func() {
		closeErr = l.ptmx.Close()
		if l.cmd.Process != nil {
			// Kirim SIGHUP lalu SIGKILL ke seluruh process group agar tidak menyisakan proses zombie
			_ = syscall.Kill(-l.cmd.Process.Pid, syscall.SIGHUP)
			_ = syscall.Kill(-l.cmd.Process.Pid, syscall.SIGKILL)
		}
	})
	return closeErr
}

// ConfigureKillableCmd mengelompokkan child process dalam PGID tersendiri agar dapat dihentikan total saat Ctrl+C
func ConfigureKillableCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// KillCmdTree menghentikan paksa seluruh proses turunan dalam process group perintah yang dibatalkan
func KillCmdTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
