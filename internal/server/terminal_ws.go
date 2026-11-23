package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/terminal"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

type wsControlMsg struct {
	Type string `json:"type"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
	Data string `json:"data,omitempty"`
}

// isSameOriginWS memverifikasi header Origin terhadap Host untuk mencegah Cross-Site WebSocket Hijacking (CSWSH)
func isSameOriginWS(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

var terminalWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 8192,
	CheckOrigin:     isSameOriginWS,
}

func clampInt(val, minV, maxV, fallback int) int {
	if val <= 0 {
		return fallback
	}
	if val < minV {
		return minV
	}
	if val > maxV {
		return maxV
	}
	return val
}

// handleTerminalWS membuka jembatan WebSocket PTY interaktif (local Linux PTY / remote SSH PTY)
func (s *Server) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	serverID, _ := strconv.ParseInt(q.Get("server_id"), 10, 64)
	colsRaw, _ := strconv.Atoi(q.Get("cols"))
	rowsRaw, _ := strconv.Atoi(q.Get("rows"))
	cols := clampInt(colsRaw, 20, 500, 120)
	rows := clampInt(rowsRaw, 5, 200, 32)

	conn, err := terminalWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Hapus batas deadline bawaan http.Server agar sesi PTY interaktif tetap hidup lama
	_ = conn.SetReadDeadline(time.Time{})
	_ = conn.SetWriteDeadline(time.Time{})

	var writeMu sync.Mutex
	sendJSON := func(payload map[string]interface{}) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
		err := conn.WriteJSON(payload)
		_ = conn.SetWriteDeadline(time.Time{})
		return err
	}
	sendBinary := func(data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
		err := conn.WriteMessage(websocket.BinaryMessage, data)
		_ = conn.SetWriteDeadline(time.Time{})
		return err
	}

	var ptySess terminal.PTYSession
	meta := map[string]interface{}{
		"type":    "meta",
		"version": s.Version(),
		"cwd":     "~",
		"os":      runtime.GOOS,
	}

	if serverID > 0 {
		// Hubungkan sesi PTY ke server remote melalui SSH connection pool + TOFU verification
		srv, err := s.getDecryptedTerminalServer(r.Context(), serverID)
		if err != nil || srv == nil {
			_ = sendJSON(map[string]interface{}{
				"type":    "error",
				"message": "Server remote tidak ditemukan atau gagal didekripsi",
			})
			return
		}

		client, fp, err := s.sshPool.Acquire(terminal.SSHConfig{
			ServerID:   srv.ID,
			Host:       srv.Host,
			Port:       srv.Port,
			Username:   srv.Username,
			AuthType:   srv.AuthType,
			Password:   srv.Password,
			PrivateKey: srv.PrivateKey,
			Passphrase: srv.Passphrase,
		}, 15*time.Second)
		if err != nil {
			_, isMismatch := terminal.IsHostKeyMismatch(err)
			_ = sendJSON(map[string]interface{}{
				"type":              "error",
				"message":           err.Error(),
				"host_key_mismatch": isMismatch,
			})
			return
		}

		ptySess, err = terminal.StartSSHPTY(client, cols, rows)
		if err != nil {
			// Invalidasi koneksi jika pembukaan channel gagal (kemungkinan koneksi setengah putus)
			s.sshPool.InvalidateServer(srv.ID)
			_ = sendJSON(map[string]interface{}{
				"type":    "error",
				"message": fmt.Sprintf("Gagal membuka shell PTY SSH: %v", err),
			})
			return
		}
		meta["mode"] = "ssh-pty"
		meta["host"] = srv.Host
		meta["user"] = srv.Username
		meta["fingerprint"] = fp
	} else {
		// Buka sesi PTY lokal langsung di $HOME (layaknya login SSH)
		homeDir := resolveLocalDefaultCwd()
		ptySess, err = terminal.StartLocalPTY(homeDir, cols, rows)
		if err != nil {
			_ = sendJSON(map[string]interface{}{
				"type":    "error",
				"message": fmt.Sprintf("Gagal memulai sesi PTY lokal: %v", err),
			})
			return
		}
		meta["mode"] = "local-pty"
		meta["host"] = "fikpanel"
		meta["user"] = "root"
	}
	defer ptySess.Close()

	_ = sendJSON(meta)

	done := make(chan struct{})
	var closeDone sync.Once
	signalDone := func() { closeDone.Do(func() { close(done) }) }

	// Keepalive WebSocket ping setiap 25 detik agar reverse proxy tidak memutus sesi idle
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-s.bgCtx.Done():
				_ = ptySess.Close()
				return
			case <-ticker.C:
				writeMu.Lock()
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				_ = conn.SetWriteDeadline(time.Time{})
				writeMu.Unlock()
				if err != nil {
					signalDone()
					_ = ptySess.Close()
					return
				}
			}
		}
	}()

	// Goroutine pembaca output PTY -> stream biner ke browser (xterm.js)
	go func() {
		defer signalDone()
		buf := make([]byte, 4096)
		for {
			n, readErr := ptySess.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				if err := sendBinary(chunk); err != nil {
					return
				}
			}
			if readErr != nil {
				exitCode, _ := ptySess.Wait()
				_ = sendJSON(map[string]interface{}{
					"type": "exit",
					"code": exitCode,
				})
				return
			}
		}
	}()

	// Loop pembaca input browser -> PTY (karakter ketikan, Ctrl+C, dan event resize)
	for {
		msgType, payload, err := conn.ReadMessage()
		if err != nil {
			signalDone()
			return
		}
		switch msgType {
		case websocket.BinaryMessage:
			if len(payload) > 0 {
				if _, err := ptySess.Write(payload); err != nil {
					signalDone()
					return
				}
			}
		case websocket.TextMessage:
			var ctrl wsControlMsg
			if jsonErr := json.Unmarshal(payload, &ctrl); jsonErr == nil && ctrl.Type != "" {
				switch ctrl.Type {
				case "resize":
					_ = ptySess.Resize(clampInt(ctrl.Cols, 20, 500, 120), clampInt(ctrl.Rows, 5, 200, 32))
				case "input":
					if ctrl.Data != "" {
						_, _ = ptySess.Write([]byte(ctrl.Data))
					}
				case "ping":
					_ = sendJSON(map[string]interface{}{"type": "pong"})
				}
			} else if len(payload) > 0 {
				_, _ = ptySess.Write(payload)
			}
		}
	}
}

// sseEventWriter menulis frame Server-Sent Events (SSE) secara aman dari banyak goroutine
type sseEventWriter struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	flusher http.Flusher
}

func (ew *sseEventWriter) Send(event string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	ew.mu.Lock()
	defer ew.mu.Unlock()
	if _, err := fmt.Fprintf(ew.w, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return err
	}
	ew.flusher.Flush()
	return nil
}

// sentinelFilterWriter menyaring blok sentinel pwd dari aliran stdout real-time sebelum dikirim via SSE
type sentinelFilterWriter struct {
	mu        sync.Mutex
	pending   string
	captured  string
	totalSent int
	truncated bool
	maxBytes  int
	onChunk   func(text string)
}

func (sw *sentinelFilterWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.pending += string(p)
	// Jika sentinel awal terdeteksi, tahan seluruh sisa output agar diproses saat Flush()
	if idx := strings.Index(sw.pending, pwdSentinelStart); idx >= 0 {
		if idx > 0 {
			sw.emitLocked(strings.TrimSuffix(strings.TrimSuffix(sw.pending[:idx], "\r\n"), "\n"))
		}
		sw.captured += sw.pending[idx:]
		sw.pending = ""
		return len(p), nil
	}
	if sw.captured != "" {
		sw.captured += sw.pending
		sw.pending = ""
		return len(p), nil
	}

	// Sisakan ekor seukuran sentinel agar potongan sentinel lintas-chunk tidak bocor ke klien
	keepTail := len(pwdSentinelStart) + 2
	if len(sw.pending) > keepTail {
		emitPart := sw.pending[:len(sw.pending)-keepTail]
		sw.pending = sw.pending[len(sw.pending)-keepTail:]
		sw.emitLocked(emitPart)
	}
	return len(p), nil
}

func (sw *sentinelFilterWriter) Flush(fallbackCwd string) string {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	combined := sw.pending + sw.captured
	sw.pending = ""
	sw.captured = ""
	clean, resolvedCwd := extractAndStripPwdSentinel(combined, fallbackCwd)
	if clean != "" {
		sw.emitLocked(clean)
	}
	return resolvedCwd
}

func (sw *sentinelFilterWriter) emitLocked(text string) {
	if text == "" || sw.truncated {
		return
	}
	if sw.maxBytes > 0 && sw.totalSent+len(text) > sw.maxBytes {
		remain := sw.maxBytes - sw.totalSent
		if remain > 0 {
			sw.onChunk(text[:remain])
			sw.totalSent += remain
		}
		sw.onChunk("\n... [output stream dibatasi 512KB]")
		sw.truncated = true
		return
	}
	sw.totalSent += len(text)
	sw.onChunk(text)
}

// handleTerminalExecStream mengeksekusi perintah dengan streaming output SSE real-time dan dukungan pembatalan (Ctrl+C)
func (s *Server) handleTerminalExecStream(w http.ResponseWriter, r *http.Request) {
	var req TerminalExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload tidak valid"})
		return
	}

	cmdClean := strings.TrimSpace(req.Command)
	if cmdClean == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "perintah kosong"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming SSE tidak didukung"})
		return
	}

	// Nonaktifkan WriteTimeout untuk koneksi streaming perintah berdurasi panjang
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ew := &sseEventWriter{w: w, flusher: flusher}
	startTime := time.Now()
	const maxStreamBytes = 512 * 1024

	// 1. Streaming Remote SSH via SSHPool
	if req.ServerID > 0 {
		srv, err := s.getDecryptedTerminalServer(r.Context(), req.ServerID)
		if err != nil || srv == nil {
			_ = ew.Send("error", map[string]interface{}{"message": "server remote tidak ditemukan"})
			return
		}

		client, fp, err := s.sshPool.Acquire(terminal.SSHConfig{
			ServerID:   srv.ID,
			Host:       srv.Host,
			Port:       srv.Port,
			Username:   srv.Username,
			AuthType:   srv.AuthType,
			Password:   srv.Password,
			PrivateKey: srv.PrivateKey,
			Passphrase: srv.Passphrase,
		}, 15*time.Second)
		if err != nil {
			_, isMismatch := terminal.IsHostKeyMismatch(err)
			_ = ew.Send("error", map[string]interface{}{
				"message":           fmt.Sprintf("Gagal terhubung ke remote server [%s:%d]: %v", srv.Host, srv.Port, err),
				"host_key_mismatch": isMismatch,
			})
			return
		}

		session, err := client.NewSession()
		if err != nil {
			s.sshPool.InvalidateServer(srv.ID)
			_ = ew.Send("error", map[string]interface{}{"message": fmt.Sprintf("Gagal membuka sesi SSH: %v", err)})
			return
		}
		defer session.Close()

		filterOut := &sentinelFilterWriter{
			maxBytes: maxStreamBytes,
			onChunk: func(text string) {
				_ = ew.Send("chunk", map[string]interface{}{"stream": "stdout", "text": text})
			},
		}
		filterErr := &sentinelFilterWriter{
			maxBytes: maxStreamBytes,
			onChunk: func(text string) {
				_ = ew.Send("chunk", map[string]interface{}{"stream": "stderr", "text": text})
			},
		}
		session.Stdout = filterOut
		session.Stderr = filterErr

		// Pantau pembatalan konteks klien (Ctrl+C / Stop) untuk langsung mengirim SIGINT ke remote
		cancelDone := make(chan struct{})
		go func() {
			select {
			case <-r.Context().Done():
				_ = session.Signal(ssh.SIGINT)
				_ = session.Close()
			case <-cancelDone:
			}
		}()

		wrappedRemoteCmd := "bash -c " + shellSingleQuote(buildStatefulBashCommand(req.Cwd, cmdClean))
		runErr := session.Run(wrappedRemoteCmd)
		close(cancelDone)

		resolvedCwd := filterOut.Flush(req.Cwd)
		filterErr.Flush(req.Cwd)
		exitCode := 0
		if runErr != nil {
			exitCode = 1
			var sshExit *ssh.ExitError
			if errors.As(runErr, &sshExit) {
				exitCode = sshExit.ExitStatus()
			}
		}

		_ = ew.Send("done", map[string]interface{}{
			"ok":          runErr == nil,
			"exit_code":   exitCode,
			"cwd":         resolvedCwd,
			"duration_ms": time.Since(startTime).Milliseconds(),
			"prompt_user": srv.Username,
			"prompt_host": srv.Host,
			"fingerprint": fp,
			"aborted":     r.Context().Err() != nil,
		})
		return
	}

	// 2. Streaming Lokal dengan pembatalan process group saat klien memutus koneksi (Ctrl+C)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	localHome := resolveLocalDefaultCwd()
	baseCwd := strings.TrimSpace(req.Cwd)
	if baseCwd == "" {
		baseCwd = "~"
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		execDir := baseCwd
		if execDir == "~" {
			execDir = localHome
		} else if strings.HasPrefix(execDir, "~/") {
			execDir = filepath.Join(localHome, execDir[2:])
		}
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdClean)
		if execDir != "" {
			cmd.Dir = execDir
		}
	} else {
		wrappedLocalCmd := buildStatefulBashCommand(baseCwd, cmdClean)
		cmd = exec.CommandContext(ctx, "bash", "-c", wrappedLocalCmd)
		if os.Getenv("HOME") == "" && localHome != "" {
			cmd.Env = append(os.Environ(), "HOME="+localHome)
		}
		terminal.ConfigureKillableCmd(cmd)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = ew.Send("error", map[string]interface{}{"message": fmt.Sprintf("Gagal membuka stdout: %v", err)})
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = ew.Send("error", map[string]interface{}{"message": fmt.Sprintf("Gagal membuka stderr: %v", err)})
		return
	}

	if err := cmd.Start(); err != nil {
		_ = ew.Send("error", map[string]interface{}{"message": fmt.Sprintf("Gagal menjalankan perintah: %v", err)})
		return
	}

	// Jika klien membatalkan request (Ctrl+C / Stop), bunuh seluruh pohon proses
	killStop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			terminal.KillCmdTree(cmd)
		case <-killStop:
		}
	}()

	filterOut := &sentinelFilterWriter{
		maxBytes: maxStreamBytes,
		onChunk: func(text string) {
			_ = ew.Send("chunk", map[string]interface{}{"stream": "stdout", "text": text})
		},
	}
	filterErr := &sentinelFilterWriter{
		maxBytes: maxStreamBytes,
		onChunk: func(text string) {
			_ = ew.Send("chunk", map[string]interface{}{"stream": "stderr", "text": text})
		},
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(filterOut, stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(filterErr, stderrPipe)
	}()
	wg.Wait()

	runErr := cmd.Wait()
	close(killStop)

	resolvedCwd := formatPromptCwd(filterOut.Flush(baseCwd), localHome)
	filterErr.Flush(baseCwd)

	exitCode := 0
	if runErr != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}

	_ = ew.Send("done", map[string]interface{}{
		"ok":          runErr == nil,
		"exit_code":   exitCode,
		"cwd":         resolvedCwd,
		"duration_ms": time.Since(startTime).Milliseconds(),
		"prompt_user": "root",
		"prompt_host": "fikpanel",
		"aborted":     ctx.Err() != nil,
	})
}

// handleResetTerminalHostKey menghapus sidik jari host key tersimpan agar server bisa dipercaya ulang setelah rebuild
func (s *Server) handleResetTerminalHostKey(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID server tidak valid"})
		return
	}

	srv, err := s.store.GetTerminalServer(r.Context(), id)
	if err != nil || srv == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "server tidak ditemukan"})
		return
	}

	port := srv.Port
	if port <= 0 {
		port = 22
	}
	hostPort := fmt.Sprintf("%s:%d", strings.TrimSpace(srv.Host), port)
	if err := s.store.DeleteKnownHost(r.Context(), hostPort); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.sshPool.InvalidateServer(srv.ID)
	s.sshPool.InvalidateHost(hostPort)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"host_port": hostPort,
		"message":   fmt.Sprintf("Host key untuk %s berhasil direset; koneksi berikutnya akan memverifikasi ulang (TOFU)", hostPort),
	})
}
