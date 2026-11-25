package system

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Manager mengelola seluruh operasi pemantauan host, proses, jaringan, swap/memori, disk cleaner, update OS, dan diagnostik
type Manager struct{}

// NewManager membuat instance baru manajer sistem host
func NewManager() *Manager {
	return &Manager{}
}

// GetHostOverview mengumpulkan informasi perangkat keras, beban sistem, pecahan RAM/Swap, proses, dan partisi disk+inode
func (m *Manager) GetHostOverview(ctx context.Context) (*HostOverview, error) {
	hostname, _ := os.Hostname()
	ov := &HostOverview{
		IsLinux:  runtime.GOOS == "linux",
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCores: runtime.NumCPU(),
		Disks:    []DiskPartition{},
	}

	if runtime.GOOS != "linux" {
		osLabel := runtime.GOOS
		if len(osLabel) > 0 {
			osLabel = strings.ToUpper(osLabel[:1]) + osLabel[1:]
		}
		ov.OS = fmt.Sprintf("%s (%s)", osLabel, runtime.GOARCH)
		ov.Kernel = "Non-Linux Host"
		ov.CPUModel = fmt.Sprintf("%d Logical Cores (%s)", runtime.NumCPU(), runtime.GOARCH)
		return ov, nil
	}

	ov.OS = readLinuxPrettyOS()
	ov.Kernel = readLinuxKernel()
	ov.CPUModel = readCPUModel()
	ov.UptimeSec = readLinuxUptimeSec()
	ov.Load1, ov.Load5, ov.Load15 = readLoadAvg()

	// Baca pecahan memori aktual dari /proc/meminfo
	mem := readProcMemInfo()
	ov.MemTotalBytes = mem.Total
	ov.MemAvailBytes = mem.Available
	ov.MemCacheBytes = mem.Cache
	if mem.Total >= mem.Available {
		ov.MemUsedBytes = mem.Total - mem.Available
	}
	if mem.Total > 0 {
		ov.MemUsedPct = float64(ov.MemUsedBytes) * 100.0 / float64(mem.Total)
	}
	ov.SwapTotalBytes = mem.SwapTotal
	if mem.SwapTotal >= mem.SwapFree {
		ov.SwapUsedBytes = mem.SwapTotal - mem.SwapFree
	}
	if mem.SwapTotal > 0 {
		ov.SwapUsedPct = float64(ov.SwapUsedBytes) * 100.0 / float64(mem.SwapTotal)
	}

	// Hitung ringkasan status proses (total, running, zombie)
	ov.TotalProcs, ov.RunningProcs, ov.ZombieProcs = countProcessStates(ctx)

	// Periksa kebutuhan reboot setelah pembaruan kernel/paket Debian/Ubuntu
	if _, err := os.Stat("/var/run/reboot-required"); err == nil {
		ov.NeedsReboot = true
	}

	// Hitung cepat insiden OOM killer terbaru pada dmesg
	ov.OOMKillsRecent = countRecentOOMKills(ctx)

	// Baca daftar partisi disk dan persentase inode
	ov.Disks = readDiskPartitions(ctx)

	return ov, nil
}

// procMemSnapshot menyimpan hasil pembacaan terstruktur /proc/meminfo dalam satuan byte
type procMemSnapshot struct {
	Total     uint64
	Available uint64
	Free      uint64
	Buffers   uint64
	Cached    uint64
	SReclaim  uint64
	Cache     uint64
	SwapTotal uint64
	SwapFree  uint64
}

// readProcMemInfo mem-parsing /proc/meminfo menjadi struktur byte presisi
func readProcMemInfo() procMemSnapshot {
	var snap procMemSnapshot
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return snap
	}
	parseProcMemInfo(string(data), &snap)
	return snap
}

// parseProcMemInfo mem-parsing konten teks /proc/meminfo (dipisah agar dapat diuji unit)
func parseProcMemInfo(raw string, snap *procMemSnapshot) {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		valKB, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		valBytes := valKB * 1024
		switch key {
		case "MemTotal":
			snap.Total = valBytes
		case "MemAvailable":
			snap.Available = valBytes
		case "MemFree":
			snap.Free = valBytes
		case "Buffers":
			snap.Buffers = valBytes
		case "Cached":
			snap.Cached = valBytes
		case "SReclaimable":
			snap.SReclaim = valBytes
		case "SwapTotal":
			snap.SwapTotal = valBytes
		case "SwapFree":
			snap.SwapFree = valBytes
		}
	}
	snap.Cache = snap.Buffers + snap.Cached + snap.SReclaim
	if snap.Available == 0 && snap.Total > 0 {
		snap.Available = snap.Free + snap.Cache
	}
}

func readLinuxPrettyOS() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		}
	}
	return "Linux"
}

func readLinuxKernel() string {
	if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return "unknown"
}

func readCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return fmt.Sprintf("%d Cores", runtime.NumCPU())
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.ToLower(line), "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return fmt.Sprintf("%d Cores (%s)", runtime.NumCPU(), runtime.GOARCH)
}

func readLinuxUptimeSec() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	f, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return int64(f)
}

func readLoadAvg() (l1, l5, l15 float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		l1, _ = strconv.ParseFloat(fields[0], 64)
		l5, _ = strconv.ParseFloat(fields[1], 64)
		l15, _ = strconv.ParseFloat(fields[2], 64)
	}
	return l1, l5, l15
}

func countProcessStates(ctx context.Context) (total, running, zombie int) {
	out, err := exec.CommandContext(ctx, "ps", "-eo", "stat=").Output()
	if err != nil {
		return 0, 0, 0
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		st := strings.TrimSpace(scanner.Text())
		if st == "" {
			continue
		}
		total++
		switch st[0] {
		case 'R':
			running++
		case 'Z':
			zombie++
		}
	}
	return total, running, zombie
}

// readDiskPartitions membaca penggunaan blok dan inode seluruh partisi fisik melalui `df -B1 -P -T` dan `df -i -P`
func readDiskPartitions(ctx context.Context) []DiskPartition {
	outBytes, err := exec.CommandContext(ctx, "df", "-B1", "-P", "-T").Output()
	if err != nil {
		return []DiskPartition{}
	}
	outInodes, _ := exec.CommandContext(ctx, "df", "-i", "-P").Output()
	return parseDiskPartitions(string(outBytes), string(outInodes))
}

// parseDiskPartitions mem-parsing output df blok dan df inode menjadi slice DiskPartition bersih (tanpa pseudo-fs)
func parseDiskPartitions(dfBytesOut, dfInodesOut string) []DiskPartition {
	inodeMap := make(map[string]struct {
		total uint64
		used  uint64
		pct   float64
	})

	if dfInodesOut != "" {
		sc := bufio.NewScanner(strings.NewReader(dfInodesOut))
		first := true
		for sc.Scan() {
			if first {
				first = false
				continue
			}
			f := strings.Fields(sc.Text())
			if len(f) < 6 {
				continue
			}
			mount := f[5]
			tot, _ := strconv.ParseUint(f[1], 10, 64)
			used, _ := strconv.ParseUint(f[2], 10, 64)
			pctStr := strings.TrimSuffix(f[4], "%")
			pct, _ := strconv.ParseFloat(pctStr, 64)
			inodeMap[mount] = struct {
				total uint64
				used  uint64
				pct   float64
			}{tot, used, pct}
		}
	}

	ignoredFS := map[string]bool{
		"tmpfs": true, "devtmpfs": true, "squashfs": true, "overlay": true,
		"proc": true, "sysfs": true, "cgroup": true, "cgroup2": true, "devpts": true,
		"efivarfs": true, "fuse.lxcfs": true, "tracefs": true, "securityfs": true,
	}

	list := make([]DiskPartition, 0)
	seenMount := make(map[string]bool)
	sc := bufio.NewScanner(strings.NewReader(dfBytesOut))
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		f := strings.Fields(sc.Text())
		if len(f) < 7 {
			continue
		}
		dev := f[0]
		fsType := strings.ToLower(f[1])
		mount := f[6]
		if ignoredFS[fsType] || strings.HasPrefix(dev, "/dev/loop") || strings.HasPrefix(mount, "/snap/") || seenMount[mount] {
			continue
		}
		seenMount[mount] = true

		total, _ := strconv.ParseUint(f[2], 10, 64)
		used, _ := strconv.ParseUint(f[3], 10, 64)
		avail, _ := strconv.ParseUint(f[4], 10, 64)
		if total == 0 {
			continue
		}
		usedPct := float64(used) * 100.0 / float64(total)
		ino := inodeMap[mount]

		list = append(list, DiskPartition{
			Device:      dev,
			Mountpoint:  mount,
			FSType:      fsType,
			TotalBytes:  total,
			UsedBytes:   used,
			AvailBytes:  avail,
			UsedPct:     usedPct,
			InodesTotal: ino.total,
			InodesUsed:  ino.used,
			InodesPct:   ino.pct,
		})
	}
	return list
}

// allowedDiskHogRoots membatasi direktori yang diizinkan untuk dianalisis agar bebas path traversal
var allowedDiskHogRoots = map[string]bool{
	"/":        true,
	"/var":     true,
	"/var/log": true,
	"/www":     true,
	"/home":    true,
	"/opt":     true,
	"/usr":     true,
}

// AnalyzeDiskHogs memindai subdirektori terbesar di bawah root target dengan batas waktu aman (15 detik)
func (m *Manager) AnalyzeDiskHogs(ctx context.Context, targetPath string) (*DiskHogsResult, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(targetPath)))
	if clean == "" || clean == "." {
		clean = "/"
	}
	if !allowedDiskHogRoots[clean] {
		return nil, fmt.Errorf("direktori target tidak termasuk daftar root yang diizinkan (%s)", clean)
	}

	res := &DiskHogsResult{
		IsLinux:    runtime.GOOS == "linux",
		TargetPath: clean,
		Entries:    []DiskHogEntry{},
	}
	if runtime.GOOS != "linux" {
		return res, nil
	}

	if _, err := os.Stat(clean); err != nil {
		return nil, fmt.Errorf("direktori %s tidak ditemukan di host", clean)
	}

	// Batasi eksekusi du agar tidak menahan I/O disk terlalu lama pada server sibuk
	tCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Gunakan -x agar tidak menyeberang ke pseudo-fs /proc, /sys, /dev
	cmd := exec.CommandContext(tCtx, "du", "-x", "-B1", "--max-depth=1", clean)
	out, _ := cmd.Output()
	if len(out) == 0 {
		return res, nil
	}

	var totalBytes int64
	entries := make([]DiskHogEntry, 0, 20)
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		sz, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		p := strings.TrimSpace(strings.Join(fields[1:], " "))
		if filepath.Clean(p) == filepath.Clean(clean) {
			totalBytes = sz
			continue
		}
		entries = append(entries, DiskHogEntry{
			Path:      p,
			SizeBytes: sz,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SizeBytes > entries[j].SizeBytes
	})
	if len(entries) > 15 {
		entries = entries[:15]
	}

	if totalBytes == 0 {
		for _, e := range entries {
			totalBytes += e.SizeBytes
		}
	}
	for i := range entries {
		if totalBytes > 0 {
			entries[i].Percent = float64(entries[i].SizeBytes) * 100.0 / float64(totalBytes)
		}
	}

	res.TotalBytes = totalBytes
	res.Entries = entries
	return res, nil
}

// RebootHost menjadwalkan reboot server Linux secara aman setelah jeda singkat agar respons HTTP selesai terkirim
func (m *Manager) RebootHost(ctx context.Context, confirmHostname string) error {
	host, _ := os.Hostname()
	if strings.TrimSpace(confirmHostname) == "" || strings.TrimSpace(confirmHostname) != host {
		return fmt.Errorf("konfirmasi hostname tidak cocok (ketik persis '%s')", host)
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("reboot server hanya tersedia saat berjalan pada host Linux")
	}

	go func() {
		time.Sleep(2 * time.Second)
		if err := exec.Command("systemctl", "reboot").Run(); err != nil {
			_ = exec.Command("shutdown", "-r", "now").Run()
		}
	}()
	return nil
}
