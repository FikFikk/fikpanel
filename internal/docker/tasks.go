package docker

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// streamTask adalah state internal untuk tugas Docker jangka panjang yang mengalirkan log via SSE
type streamTask struct {
	mu        sync.RWMutex
	id        string
	kind      string
	target    string
	status    string
	errMsg    string
	startedAt int64
	endedAt   int64
	logs      []string
	subs      map[chan string]struct{}
	done      chan struct{}
}

// StreamTaskRegistry mengelola tugas streaming Docker (pull image, build Dockerfile, install Docker CE)
type StreamTaskRegistry struct {
	mu    sync.RWMutex
	tasks map[string]*streamTask
}

func newStreamTaskRegistry() *StreamTaskRegistry {
	return &StreamTaskRegistry{
		tasks: make(map[string]*streamTask),
	}
}

func generateTaskID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%d_%s", prefix, time.Now().Unix(), hex.EncodeToString(b))
}

// Create membuat entri tugas streaming baru dan menjadwalkan pembersihan otomatis setelah selesai
func (r *StreamTaskRegistry) Create(kind, target string) *streamTask {
	t := &streamTask{
		id:        generateTaskID("dk"),
		kind:      kind,
		target:    target,
		status:    "running",
		startedAt: time.Now().Unix(),
		logs:      make([]string, 0, 64),
		subs:      make(map[chan string]struct{}),
		done:      make(chan struct{}),
	}
	r.mu.Lock()
	r.tasks[t.id] = t
	// Bersihkan tugas lama yang sudah selesai lebih dari 30 menit agar memori tetap efisien
	cutoff := time.Now().Add(-30 * time.Minute).Unix()
	for id, existing := range r.tasks {
		existing.mu.RLock()
		expired := existing.status != "running" && existing.endedAt > 0 && existing.endedAt < cutoff
		existing.mu.RUnlock()
		if expired {
			delete(r.tasks, id)
		}
	}
	r.mu.Unlock()
	return t
}

// Append menambahkan satu baris log ke buffer tugas dan menyiarkannya ke semua klien SSE aktif
func (t *streamTask) Append(line string) {
	clean := strings.TrimRight(line, "\r\n")
	if clean == "" {
		return
	}
	t.mu.Lock()
	// Batasi ring buffer maksimal 1200 baris per tugas agar penggunaan RAM terkendali
	if len(t.logs) >= 1200 {
		t.logs = t.logs[len(t.logs)-1000:]
	}
	t.logs = append(t.logs, clean)
	for ch := range t.subs {
		select {
		case ch <- clean:
		default:
			// Lewati frame bila klien lambat agar proses docker tidak terblokir
		}
	}
	t.mu.Unlock()
}

// Finish menandai tugas selesai (success atau failed) dan menutup kanal tunggu
func (t *streamTask) Finish(err error) {
	t.mu.Lock()
	if err != nil {
		t.status = "failed"
		t.errMsg = err.Error()
	} else {
		t.status = "success"
	}
	t.endedAt = time.Now().Unix()
	for ch := range t.subs {
		close(ch)
		delete(t.subs, ch)
	}
	select {
	case <-t.done:
	default:
		close(t.done)
	}
	t.mu.Unlock()
}

// Snapshot mengambil salinan immutable status dan log tugas saat ini
func (t *streamTask) Snapshot() DockerStreamTaskSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	logsCopy := make([]string, len(t.logs))
	copy(logsCopy, t.logs)
	return DockerStreamTaskSnapshot{
		ID:        t.id,
		Kind:      t.kind,
		Target:    t.target,
		Status:    t.status,
		Error:     t.errMsg,
		StartedAt: t.startedAt,
		EndedAt:   t.endedAt,
		Logs:      logsCopy,
	}
}

// Subscribe mengembalikan riwayat log yang sudah ada beserta kanal live stream dan fungsi pembersihnya
func (r *StreamTaskRegistry) Subscribe(id string) (DockerStreamTaskSnapshot, <-chan string, func(), bool) {
	r.mu.RLock()
	t, ok := r.tasks[id]
	r.mu.RUnlock()
	if !ok {
		return DockerStreamTaskSnapshot{}, nil, func() {}, false
	}

	t.mu.Lock()
	logsCopy := make([]string, len(t.logs))
	copy(logsCopy, t.logs)
	snap := DockerStreamTaskSnapshot{
		ID:        t.id,
		Kind:      t.kind,
		Target:    t.target,
		Status:    t.status,
		Error:     t.errMsg,
		StartedAt: t.startedAt,
		EndedAt:   t.endedAt,
		Logs:      logsCopy,
	}
	if t.status != "running" {
		t.mu.Unlock()
		return snap, nil, func() {}, true
	}

	ch := make(chan string, 128)
	t.subs[ch] = struct{}{}
	t.mu.Unlock()

	cleanup := func() {
		t.mu.Lock()
		if _, exists := t.subs[ch]; exists {
			delete(t.subs, ch)
			close(ch)
		}
		t.mu.Unlock()
	}
	return snap, ch, cleanup, true
}

// GetSnapshot mengembalikan status terbaru dari tugas berdasarkan ID
func (r *StreamTaskRegistry) GetSnapshot(id string) (DockerStreamTaskSnapshot, bool) {
	r.mu.RLock()
	t, ok := r.tasks[id]
	r.mu.RUnlock()
	if !ok {
		return DockerStreamTaskSnapshot{}, false
	}
	return t.Snapshot(), true
}

// runCommandStream mengeksekusi perintah CLI secara asinkron sambil meneruskan stdout & stderr baris demi baris
func runCommandStream(ctx context.Context, task *streamTask, cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	scanStream := func(rd io.Reader) {
		scanner := bufio.NewScanner(rd)
		// Perbesar kapasitas buffer scanner untuk output progress layer yang panjang
		scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)
		scanner.Split(splitCRorLF)
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text != "" {
				task.Append(text)
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanStream(stdout)
	}()
	go func() {
		defer wg.Done()
		scanStream(stderr)
	}()

	wg.Wait()
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}

// splitCRorLF memecah aliran teks berdasarkan \r maupun \n agar progress bar `docker pull` tertangkap real-time
func splitCRorLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == '\n' || b == '\r' {
			// Lewati pasangan \r\n sekaligus
			if b == '\r' && i+1 < len(data) && data[i+1] == '\n' {
				return i + 2, data[:i], nil
			}
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
