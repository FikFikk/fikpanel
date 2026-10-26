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

// LinuxCollector mengumpulkan metrik dari virtual filesystem /proc di kernel Linux
type LinuxCollector struct {
	prevIdle   uint64
	prevTotal  uint64
	prevNetIn  uint64
	prevNetOut uint64
	prevTime   time.Time
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

	// 1. Baca penggunaan CPU dari /proc/stat
	if idle, total, err := readCPUStat(); err == nil {
		if c.prevTotal > 0 && total > c.prevTotal {
			deltaTotal := float64(total - c.prevTotal)
			deltaIdle := float64(idle - c.prevIdle)
			p.CPU = ((deltaTotal - deltaIdle) / deltaTotal) * 100.0
		}
		c.prevIdle = idle
		c.prevTotal = total
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

	// 4. Baca penggunaan disk root via syscall.Statfs
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		p.DiskTotal = stat.Blocks * uint64(stat.Bsize)
		p.DiskUsed = (stat.Blocks - stat.Bfree) * uint64(stat.Bsize)
	}

	// 5. Baca trafik jaringan dari /proc/net/dev
	if netIn, netOut, err := readNetDev(); err == nil {
		duration := now.Sub(c.prevTime).Seconds()
		if duration > 0 && c.prevNetIn > 0 && netIn >= c.prevNetIn {
			p.NetIn = uint64(float64(netIn-c.prevNetIn) / duration)
			p.NetOut = uint64(float64(netOut-c.prevNetOut) / duration)
		}
		c.prevNetIn = netIn
		c.prevNetOut = netOut
		c.prevTime = now
	}

	return p, nil
}

func readCPUStat() (idle, total uint64, err error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 5 && fields[0] == "cpu" {
			for i := 1; i < len(fields); i++ {
				val, _ := strconv.ParseUint(fields[i], 10, 64)
				total += val
				if i == 4 {
					idle = val
				}
			}
		}
	}
	return idle, total, nil
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
