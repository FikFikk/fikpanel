//go:build linux

package metrics

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// LinuxCollector mengumpulkan metrik sistem nyata dari virtual filesystem /proc di kernel Linux
type LinuxCollector struct {
	prevAgg       cpuSample
	prevCores     []cpuSample
	prevNetIn     uint64
	prevNetOut    uint64
	prevDiskRead  uint64
	prevDiskWrite uint64
	prevTime      time.Time
}

// NewCollector membuat instance pengumpul metrik untuk sistem Linux
func NewCollector() Collector {
	return &LinuxCollector{
		prevTime: time.Now(),
	}
}

// Collect membaca data metrik sistem terkini secara langsung tanpa proses fork
func (c *LinuxCollector) Collect() (Point, error) {
	now := time.Now()
	p := Point{Timestamp: now.Unix()}
	duration := now.Sub(c.prevTime).Seconds()

	// 1. Baca penggunaan CPU agregat (User/System/IOWait/Steal) & per-core dari /proc/stat
	if raw, err := os.ReadFile("/proc/stat"); err == nil {
		agg, cores := parseProcStatRaw(string(raw))
		if c.prevAgg.total() > 0 {
			bd := computeCPUDelta(c.prevAgg, agg)
			p.CPU = bd.Total
			p.CPUUser = bd.User
			p.CPUSystem = bd.System
			p.CPUIOWait = bd.IOWait
			p.CPUSteal = bd.Steal
		}
		if len(cores) > 0 && len(c.prevCores) == len(cores) {
			perCore := make([]float64, len(cores))
			for i := range cores {
				perCore[i] = computeCPUDelta(c.prevCores[i], cores[i]).Total
			}
			p.PerCore = perCore
		}
		c.prevAgg = agg
		c.prevCores = cores
	}

	// 2. Baca informasi memori dari /proc/meminfo
	if total, avail, err := readMemInfo(); err == nil {
		p.RAMTotal = total
		if total >= avail {
			p.RAMUsed = total - avail
		}
	}

	// 3. Baca load average dari /proc/loadavg
	if l1, l5, l15, err := readLoadAvg(); err == nil {
		p.LoadAvg = l1
		p.Load5m = l5
		p.Load15m = l15
	}

	// 4. Baca penggunaan ruang disk root via syscall.Statfs
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		p.DiskTotal = stat.Blocks * uint64(stat.Bsize)
		p.DiskUsed = (stat.Blocks - stat.Bfree) * uint64(stat.Bsize)
	}

	// 5. Baca kecepatan Disk I/O (Read/Write byte/detik) dari /proc/diskstats
	if raw, err := os.ReadFile("/proc/diskstats"); err == nil {
		rd, wr := parseDiskStatsRaw(string(raw))
		if duration > 0 && (c.prevDiskRead > 0 || c.prevDiskWrite > 0) {
			if rd >= c.prevDiskRead {
				p.DiskRead = uint64(float64(rd-c.prevDiskRead) / duration)
			}
			if wr >= c.prevDiskWrite {
				p.DiskWrite = uint64(float64(wr-c.prevDiskWrite) / duration)
			}
		}
		c.prevDiskRead = rd
		c.prevDiskWrite = wr
	}

	// 6. Baca trafik jaringan dari /proc/net/dev
	if netIn, netOut, err := readNetDev(); err == nil {
		if duration > 0 && c.prevNetIn > 0 && netIn >= c.prevNetIn {
			p.NetIn = uint64(float64(netIn-c.prevNetIn) / duration)
			if netOut >= c.prevNetOut {
				p.NetOut = uint64(float64(netOut-c.prevNetOut) / duration)
			}
		}
		c.prevNetIn = netIn
		c.prevNetOut = netOut
	}

	c.prevTime = now
	return p, nil
}

func readMemInfo() (total, avail uint64, err error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				total = kb * 1024
			}
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				avail = kb * 1024
			}
		}
	}
	return total, avail, nil
}

func readLoadAvg() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		l1, _ := strconv.ParseFloat(fields[0], 64)
		l5, _ := strconv.ParseFloat(fields[1], 64)
		l15, _ := strconv.ParseFloat(fields[2], 64)
		return l1, l5, l15, nil
	}
	return 0, 0, 0, nil
}

func readNetDev() (netIn, netOut uint64, err error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, ":") {
			parts := strings.Split(line, ":")
			if strings.TrimSpace(parts[0]) == "lo" {
				continue
			}
			fields := strings.Fields(parts[1])
			if len(fields) >= 9 {
				rx, _ := strconv.ParseUint(fields[0], 10, 64)
				tx, _ := strconv.ParseUint(fields[8], 10, 64)
				netIn += rx
				netOut += tx
			}
		}
	}
	return netIn, netOut, nil
}
