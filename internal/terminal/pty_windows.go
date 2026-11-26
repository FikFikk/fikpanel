//go:build windows

package terminal

import (
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
)

type localWindowsPTY struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	pr    *io.PipeReader
	pw    *io.PipeWriter
	once  sync.Once
}

// StartLocalPTY menyediakan shell interaktif berbasis pipe untuk mode pengembangan di Windows
func StartLocalPTY(homeDir string, cols, rows int) (PTYSession, error) {
	cmd := exec.Command("cmd.exe", "/Q")
	if strings.TrimSpace(homeDir) != "" {
		cmd.Dir = homeDir
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(pw, stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(pw, stderr)
	}()
	go func() {
		wg.Wait()
		_ = pw.Close()
	}()

	return &localWindowsPTY{
		cmd:   cmd,
		stdin: stdin,
		pr:    pr,
		pw:    pw,
	}, nil
}

// StartDockerExecPTY membuka sesi docker exec berbasis pipe untuk lingkungan pengembangan Windows
func StartDockerExecPTY(containerID, shell string, cols, rows int) (PTYSession, error) {
	cid := strings.TrimSpace(containerID)
	if cid == "" || strings.HasPrefix(cid, "-") || strings.ContainsAny(cid, " \t\r\n;|&$`<>\"'") {
		return nil, errors.New("ID atau nama kontainer tidak valid")
	}

	args := []string{"exec", "-i", "-e", "TERM=xterm-256color", cid}
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
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(pw, stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(pw, stderr)
	}()
	go func() {
		wg.Wait()
		_ = pw.Close()
	}()

	return &localWindowsPTY{
		cmd:   cmd,
		stdin: stdin,
		pr:    pr,
		pw:    pw,
	}, nil
}

func (w *localWindowsPTY) Read(p []byte) (int, error) { return w.pr.Read(p) }
func (w *localWindowsPTY) Write(p []byte) (int, error) {
	// Konversi CR (\r) dari xterm.js menjadi CRLF agar cmd.exe di Windows memproses Enter
	converted := strings.ReplaceAll(string(p), "\r", "\r\n")
	_, err := w.stdin.Write([]byte(converted))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *localWindowsPTY) Resize(cols, rows int) error { return nil }

func (w *localWindowsPTY) Wait() (int, error) {
	err := w.cmd.Wait()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 1, err
}

func (w *localWindowsPTY) Close() error {
	var closeErr error
	w.once.Do(func() {
		_ = w.stdin.Close()
		if w.cmd.Process != nil {
			closeErr = w.cmd.Process.Kill()
		}
		_ = w.pw.Close()
		_ = w.pr.Close()
	})
	return closeErr
}

// ConfigureKillableCmd adalah no-op aman pada platform Windows
func ConfigureKillableCmd(cmd *exec.Cmd) {}

// KillCmdTree menghentikan proses perintah pada Windows saat dibatalkan klien
func KillCmdTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
