package metrics

import "sync"

// RingBuffer menampung maksimal n titik data metrik dalam memori tanpa alokasi berulang
type RingBuffer struct {
	mu       sync.RWMutex
	capacity int
	data     []Point
	cursor   int
	isFull   bool
}

// NewRingBuffer membuat instansiasi RingBuffer baru dengan kapasitas tetap
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		capacity: capacity,
		data:     make([]Point, capacity),
	}
}

// Push menambahkan satu data titik metrik baru ke dalam buffer
func (rb *RingBuffer) Push(p Point) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.data[rb.cursor] = p
	rb.cursor = (rb.cursor + 1) % rb.capacity
	if rb.cursor == 0 {
		rb.isFull = true
	}
}

// GetAll mengambil seluruh data titik metrik berurutan dari yang paling lama ke terbaru
func (rb *RingBuffer) GetAll() []Point {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if !rb.isFull {
		result := make([]Point, rb.cursor)
		copy(result, rb.data[:rb.cursor])
		return result
	}

	result := make([]Point, rb.capacity)
	// Salin dari cursor hingga akhir (data terlama)
	n := copy(result, rb.data[rb.cursor:])
	// Salin dari awal hingga cursor (data terbaru)
	copy(result[n:], rb.data[:rb.cursor])
	return result
}
