package terminal

import (
	"io"
	"sync"

	"golang.org/x/crypto/ssh"
)

// PTYSession merepresentasikan sesi terminal interaktif lokal maupun remote SSH
type PTYSession interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
	Wait() (int, error)
}

type sshPTYSession struct {
	session *ssh.Session
	stdin   io.WriteCloser
	pr      *io.PipeReader
	pw      *io.PipeWriter
	once    sync.Once
}

// StartSSHPTY membuka sesi shell interaktif dengan PTY asli di atas koneksi SSH yang sudah ada
func StartSSHPTY(client *ssh.Client, cols, rows int) (PTYSession, error) {
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 32
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = session.Close()
		return nil, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}

	if err := session.Shell(); err != nil {
		_ = session.Close()
		return nil, err
	}

	pr, pw := io.Pipe()
	// Gabungkan aliran stdout dan stderr remote ke satu stream baca
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

	return &sshPTYSession{
		session: session,
		stdin:   stdin,
		pr:      pr,
		pw:      pw,
	}, nil
}

func (s *sshPTYSession) Read(p []byte) (int, error)  { return s.pr.Read(p) }
func (s *sshPTYSession) Write(p []byte) (int, error) { return s.stdin.Write(p) }

func (s *sshPTYSession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return s.session.WindowChange(rows, cols)
}

func (s *sshPTYSession) Wait() (int, error) {
	err := s.session.Wait()
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := err.(*ssh.ExitError); ok {
		return exitErr.ExitStatus(), nil
	}
	return 1, err
}

func (s *sshPTYSession) Close() error {
	var closeErr error
	s.once.Do(func() {
		_ = s.session.Signal(ssh.SIGHUP)
		_ = s.stdin.Close()
		closeErr = s.session.Close()
		_ = s.pw.Close()
		_ = s.pr.Close()
	})
	return closeErr
}
