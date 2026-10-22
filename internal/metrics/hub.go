package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Hub mengelola distribusi data metrik secara real-time via Server-Sent Events (SSE)
type Hub struct {
	mu          sync.Mutex
	ring        *RingBuffer
	collector   Collector // Pada Linux di-resolve ke interface atau collector terkait
	subscribers map[chan Point]struct{}
	tickerDone  chan struct{}
	running     bool
}

// NewHub membuat instance Hub metrik dengan buffer 300 titik (5 menit)
func NewHub() *Hub {
	return &Hub{
		ring:        NewRingBuffer(300),
		collector:   NewCollector(),
		subscribers: make(map[chan Point]struct{}),
	}
}

// subscribe mendaftarkan channel subscriber baru dan mengaktifkan ticker jika subscriber pertama
func (h *Hub) subscribe(ch chan Point) []Point {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.subscribers[ch] = struct{}{}
	history := h.ring.GetAll()

	// Jika subscriber pertama, aktifkan ticker pengumpul metrik 1 detik (lazy evaluation)
	if len(h.subscribers) == 1 && !h.running {
		h.tickerDone = make(chan struct{})
		h.running = true
		go h.runTicker(h.tickerDone)
	}

	return history
}

// unsubscribe melepaskan pendaftaran subscriber dan menghentikan ticker jika subscriber kosong
func (h *Hub) unsubscribe(ch chan Point) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.subscribers, ch)
	close(ch)

	// Hentikan ticker otomatis jika tidak ada klien yang mendengarkan
	if len(h.subscribers) == 0 && h.running {
		close(h.tickerDone)
		h.running = false
	}
}

// runTicker menjalankan goroutine pengumpul data berkala setiap 1 detik
func (h *Hub) runTicker(done chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			pt, err := h.collector.Collect()
			if err != nil {
				continue
			}

			h.ring.Push(pt)

			// Broadcast titik metrik ke seluruh subscriber aktif
			h.mu.Lock()
			for ch := range h.subscribers {
				select {
				case ch <- pt:
				default:
					// Lewati jika buffer subscriber penuh agar tidak menghambat goroutine utama
				}
			}
			h.mu.Unlock()
		}
	}
}

// ServeHTTP menangani koneksi SSE dari browser
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming tidak didukung oleh browser", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	msgChan := make(chan Point, 10)
	history := h.subscribe(msgChan)
	defer h.unsubscribe(msgChan)

	// Kirim data riwayat 300 titik pertama untuk mengisi grafik saat halaman pertama kali dimuat
	if len(history) > 0 {
		histBytes, _ := json.Marshal(history)
		fmt.Fprintf(w, "event: history\ndata: %s\n\n", histBytes)
		flusher.Flush()
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case pt, ok := <-msgChan:
			if !ok {
				return
			}
			ptBytes, err := json.Marshal(pt)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: metric\ndata: %s\n\n", ptBytes)
			flusher.Flush()
		}
	}
}


