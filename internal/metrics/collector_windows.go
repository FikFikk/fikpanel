//go:build !linux

package metrics

import (
	"math/rand"
	"runtime"
	"time"
)

// WindowsCollector menyediakan implementasi metrik untuk lingkungan development Windows
type WindowsCollector struct {
	baseTime time.Time
}

// NewCollector membuat instance pengumpul metrik Windows
func NewCollector() Collector {
	return &WindowsCollector{
		baseTime: time.Now(),
	}
}

// Collect mengumpulkan data metrik simulasi realistis untuk pengujian lokal
func (c *WindowsCollector) Collect() (Point, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	now := time.Now()
	// Simulasi penggunaan CPU yang berfluktuasi wajar
	cpuPercent := 12.5 + (rand.Float64() * 8.0)

	totalRAM := uint64(16 * 1024 * 1024 * 1024) // Simulasi 16 GB
	usedRAM := uint64(6*1024*1024*1024) + m.Alloc

	totalDisk := uint64(512 * 1024 * 1024 * 1024) // 512 GB
	usedDisk := uint64(128 * 1024 * 1024 * 1024)  // 128 GB

	return Point{
		Timestamp: now.Unix(),
		CPU:       cpuPercent,
		RAMUsed:   usedRAM,
		RAMTotal:  totalRAM,
		DiskUsed:  usedDisk,
		DiskTotal: totalDisk,
		NetIn:     uint64(1024*50 + rand.Intn(1024*100)),
		NetOut:    uint64(1024*20 + rand.Intn(1024*50)),
		LoadAvg:   0.45 + (rand.Float64() * 0.2),
	}, nil
}

