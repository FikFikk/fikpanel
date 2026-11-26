//go:build !linux

package metrics

import (
	"math"
	"math/rand"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WindowsCollector membaca metrik sistem nyata (RAM & Disk) dari API kernel Windows beserta estimasi CPU/IO mode dev
type WindowsCollector struct {
	prevTime time.Time
}

// NewCollector membuat instance pengumpul metrik untuk Windows
func NewCollector() Collector {
	return &WindowsCollector{
		prevTime: time.Now(),
	}
}

// memoryStatusEx struktur Win32 untuk GlobalMemoryStatusEx
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// Collect membaca data metrik sistem dari Windows API
func (c *WindowsCollector) Collect() (Point, error) {
	now := time.Now()
	p := Point{Timestamp: now.Unix()}

	// Baca RAM fisik asli via GlobalMemoryStatusEx
	var memStatus memoryStatusEx
	memStatus.Length = uint32(unsafe.Sizeof(memStatus))
	kernel32 := windows.NewLazyDLL("kernel32.dll")
	globalMemoryStatusEx := kernel32.NewProc("GlobalMemoryStatusEx")
	ret, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret != 0 {
		p.RAMTotal = memStatus.TotalPhys
		p.RAMUsed = memStatus.TotalPhys - memStatus.AvailPhys
	}

	// Baca disk asli via GetDiskFreeSpaceExW (drive C:\)
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	pathPtr, _ := windows.UTF16PtrFromString("C:\\")
	err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes)
	if err == nil {
		p.DiskTotal = totalBytes
		p.DiskUsed = totalBytes - totalFreeBytes
	}

	// Estimasi CPU & pecahan mode pengembangan non-Linux
	p.CPU = math.Round((8.0+rand.Float64()*12.0)*10) / 10
	p.CPUUser = math.Round((p.CPU*0.68)*10) / 10
	p.CPUSystem = math.Round((p.CPU*0.24)*10) / 10
	p.CPUIOWait = math.Round((p.CPU*0.08)*10) / 10
	p.CPUSteal = 0

	cores := runtime.NumCPU()
	if cores > 16 {
		cores = 16
	}
	p.PerCore = make([]float64, cores)
	for i := 0; i < cores; i++ {
		val := p.CPU + (rand.Float64()*8.0 - 4.0)
		if val < 1 {
			val = 1
		}
		if val > 100 {
			val = 100
		}
		p.PerCore[i] = math.Round(val*10) / 10
	}

	// Estimasi Disk I/O & Network pada lingkungan pengembangan non-Linux
	p.DiskRead = uint64(1024*64 + rand.Intn(1024*256))
	p.DiskWrite = uint64(1024*32 + rand.Intn(1024*128))
	p.NetIn = uint64(1024*30 + rand.Intn(1024*80))
	p.NetOut = uint64(1024*10 + rand.Intn(1024*40))

	p.LoadAvg = math.Round((p.CPU/100.0*float64(runtime.NumCPU()))*100) / 100
	p.Load5m = math.Round((p.LoadAvg*0.9)*100) / 100
	p.Load15m = math.Round((p.LoadAvg*0.8)*100) / 100

	c.prevTime = now
	return p, nil
}
