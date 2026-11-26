package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/docker"
	"fikpanel/internal/terminal"
	"github.com/gorilla/websocket"
)

// handleDockerContainerExecWS membuka shell interaktif (`docker exec -it`) di atas WebSocket PTY untuk xterm.js
func (s *Server) handleDockerContainerExecWS(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !docker.IsValidDockerRef(id) {
		http.Error(w, "ID kontainer tidak valid", http.StatusBadRequest)
		return
	}

	q := r.URL.Query()
	shell := strings.TrimSpace(q.Get("shell"))
	colsRaw, _ := strconv.Atoi(q.Get("cols"))
	rowsRaw, _ := strconv.Atoi(q.Get("rows"))
	cols := clampInt(colsRaw, 20, 500, 120)
	rows := clampInt(rowsRaw, 5, 200, 32)

	conn, err := terminalWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

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

	ptySess, err := terminal.StartDockerExecPTY(id, shell, cols, rows)
	if err != nil {
		_ = sendJSON(map[string]interface{}{
			"type":    "error",
			"message": fmt.Sprintf("Gagal membuka shell ke kontainer %s: %v", id, err),
		})
		return
	}
	defer ptySess.Close()

	_ = s.store.RecordAudit(r.Context(), "user", "docker.container.exec", id, true, "Membuka terminal shell interaktif kontainer", nil)

	_ = sendJSON(map[string]interface{}{
		"type":      "meta",
		"mode":      "docker-exec",
		"container": id,
		"shell":     shell,
	})

	done := make(chan struct{})
	var closeDone sync.Once
	signalDone := func() { closeDone.Do(func() { close(done) }) }

	// Keepalive WebSocket ping setiap 25 detik agar koneksi melalui reverse proxy tidak terputus saat idle
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

	// Stream output PTY kontainer ke browser xterm.js dalam bentuk biner
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

	// Terima ketikan pengguna & event resize dari xterm.js lalu teruskan ke PTY kontainer
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

// handleDockerContainerLogsStream mengalirkan log kontainer secara real-time (`docker logs -f`) via SSE
func (s *Server) handleDockerContainerLogsStream(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !docker.IsValidDockerRef(id) {
		http.Error(w, "ID kontainer tidak valid", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming SSE tidak didukung", http.StatusInternalServerError)
		return
	}

	tail := 150
	if raw := r.URL.Query().Get("lines"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 2000 {
			tail = n
		}
	}
	withTimestamps := r.URL.Query().Get("timestamps") == "1" || r.URL.Query().Get("timestamps") == "true"

	args := []string{"logs", "-f", "--tail", strconv.Itoa(tail)}
	if withTimestamps {
		args = append(args, "-t")
	}
	args = append(args, id)

	cmd := exec.CommandContext(r.Context(), "docker", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "Gagal membuka pipa stdout log", http.StatusInternalServerError)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		http.Error(w, "Gagal membuka pipa stderr log", http.StatusInternalServerError)
		return
	}
	if err := cmd.Start(); err != nil {
		http.Error(w, "Gagal memulai streaming log kontainer", http.StatusInternalServerError)
		return
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	lineCh := make(chan string, 128)
	var wg sync.WaitGroup
	pipeReader := func(s *bufio.Scanner) {
		defer wg.Done()
		s.Buffer(make([]byte, 0, 32*1024), 512*1024)
		for s.Scan() {
			select {
			case lineCh <- s.Text():
			case <-r.Context().Done():
				return
			}
		}
	}

	wg.Add(2)
	go pipeReader(bufio.NewScanner(stdout))
	go pipeReader(bufio.NewScanner(stderr))
	go func() {
		wg.Wait()
		close(lineCh)
	}()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case line, ok := <-lineCh:
			if !ok {
				fmt.Fprint(w, "event: done\ndata: {\"status\":\"eof\"}\n\n")
				flusher.Flush()
				return
			}
			encoded, _ := json.Marshal(line)
			fmt.Fprintf(w, "data: %s\n\n", encoded)
			flusher.Flush()
		}
	}
}
