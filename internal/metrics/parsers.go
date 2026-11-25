package metrics

import (
	"bufio"
	"math"
	"strconv"
	"strings"
)

// cpuSample menyimpan hitungan tick mentah dari satu baris /proc/stat
type cpuSample struct {
	user    uint64
	nice    uint64
	system  uint64
	idle    uint64
	iowait  uint64
	irq     uint64
	softirq uint64
	steal   uint64
}

func (s cpuSample) total() uint64 {
	return s.user + s.nice + s.system + s.idle + s.iowait + s.irq + s.softirq + s.steal
}

func (s cpuSample) idleTotal() uint64 {
	return s.idle + s.iowait
}

// cpuBreakdown merangkum persentase pecahan CPU hasil delta dua pembacaan /proc/stat
type cpuBreakdown struct {
	Total  float64
	User   float64
	System float64
	IOWait float64
	Steal  float64
}

// parseProcStatRaw mem-parsing baris agregat "cpu" dan per-core "cpu0..cpuN" dari teks /proc/stat
func parseProcStatRaw(raw string) (agg cpuSample, cores []cpuSample) {
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 {
			continue
		}
		name := fields[0]
		if name != "cpu" && !strings.HasPrefix(name, "cpu") {
			continue
		}
		// Pastikan selain "cpu" hanya menerima indeks numerik seperti cpu0, cpu1, ...
		if name != "cpu" {
			if _, err := strconv.Atoi(name[3:]); err != nil {
				continue
			}
		}

		parseAt := func(idx int) uint64 {
			if idx >= len(fields) {
				return 0
			}
			v, _ := strconv.ParseUint(fields[idx], 10, 64)
			return v
		}
		s := cpuSample{
			user:    parseAt(1),
			nice:    parseAt(2),
			system:  parseAt(3),
			idle:    parseAt(4),
			iowait:  parseAt(5),
			irq:     parseAt(6),
			softirq: parseAt(7),
			steal:   parseAt(8),
		}
		if name == "cpu" {
			agg = s
		} else {
			cores = append(cores, s)
		}
	}
	return agg, cores
}

// computeCPUDelta menghitung pecahan persentase CPU (Total, User, System, IOWait, Steal) secara presisi
func computeCPUDelta(prev, cur cpuSample) cpuBreakdown {
	prevTot := prev.total()
	curTot := cur.total()
	if prevTot == 0 || curTot <= prevTot {
		return cpuBreakdown{}
	}
	dt := float64(curTot - prevTot)
	clampPct := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 100 {
			return 100
		}
		return math.Round(v*10) / 10
	}

	dIdle := float64(cur.idleTotal() - prev.idleTotal())
	dUser := float64((cur.user + cur.nice) - (prev.user + prev.nice))
	dSys := float64((cur.system + cur.irq + cur.softirq) - (prev.system + prev.irq + prev.softirq))
	dIOWait := float64(cur.iowait - prev.iowait)
	dSteal := float64(cur.steal - prev.steal)

	return cpuBreakdown{
		Total:  clampPct(((dt - dIdle) / dt) * 100.0),
		User:   clampPct((dUser / dt) * 100.0),
		System: clampPct((dSys / dt) * 100.0),
		IOWait: clampPct((dIOWait / dt) * 100.0),
		Steal:  clampPct((dSteal / dt) * 100.0),
	}
}

// isPhysicalBlockDevice menyaring perangkat disk induk agar tidak menghitung ganda partisi (sda1) atau loop device
func isPhysicalBlockDevice(name string) bool {
	if name == "" || strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") ||
		strings.HasPrefix(name, "dm-") || strings.HasPrefix(name, "sr") || strings.HasPrefix(name, "zram") {
		return false
	}
	// Disk NVMe (nvme0n1 diterima, partisi nvme0n1p1 ditolak)
	if strings.HasPrefix(name, "nvme") {
		return strings.Contains(name, "n") && !strings.Contains(name, "p")
	}
	// Disk MMC/SD (mmcblk0 diterima, partisi mmcblk0p1 ditolak)
	if strings.HasPrefix(name, "mmcblk") {
		return !strings.Contains(name, "p")
	}
	// Disk SATA/VirtIO/Xen (sda, vda, xvda diterima; partisi berakhiran angka seperti sda1 ditolak)
	if strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "vd") || strings.HasPrefix(name, "xvd") || strings.HasPrefix(name, "hd") {
		last := name[len(name)-1]
		return last < '0' || last > '9'
	}
	return false
}

// parseDiskStatsRaw menjumlahkan sektor baca dan tulis seluruh perangkat blok fisik dari /proc/diskstats
func parseDiskStatsRaw(raw string) (readBytes, writeBytes uint64) {
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		// Format standar Linux /proc/diskstats: major minor name rd_ios rd_merges rd_sectors rd_ticks wr_ios wr_merges wr_sectors ...
		if len(f) < 10 {
			continue
		}
		if !isPhysicalBlockDevice(f[2]) {
			continue
		}
		rdSectors, _ := strconv.ParseUint(f[5], 10, 64)
		wrSectors, _ := strconv.ParseUint(f[9], 10, 64)
		readBytes += rdSectors * 512
		writeBytes += wrSectors * 512
	}
	return readBytes, writeBytes
}
