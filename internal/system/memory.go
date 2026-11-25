package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	fikPanelSwapPath   = "/swapfile.fikpanel"
	memorySysctlPath   = "/etc/sysctl.d/99-fikpanel-memory.conf"
	minRootFreeBytes   = 2 * 1024 * 1024 * 1024 // Wajib sisakan minimal 2 GB ruang kosong pada root
	minSwapSizeMB      = 512
	maxSwapSizeMB      = 16384
)

// GetMemoryOptimizationStatus membaca status RAM, daftar swap aktif (/proc/swaps), dan tuning kernel memori
func (m *Manager) GetMemoryOptimizationStatus(ctx context.Context) (*MemoryOptimizationStatus, error) {
	st := &MemoryOptimizationStatus{
		IsLinux:          runtime.GOOS == "linux",
		Swappiness:       60,
		VFSCachePressure: 100,
		Swaps:            []SwapDevice{},
		FikPanelSwapPath: fikPanelSwapPath,
	}

	if runtime.GOOS != "linux" {
		return st, nil
	}

	mem := readProcMemInfo()
	st.MemTotalBytes = mem.Total
	st.MemAvailBytes = mem.Available
	st.MemCacheBytes = mem.Cache
	if mem.Total >= mem.Available {
		st.MemUsedBytes = mem.Total - mem.Available
	}
	st.SwapTotalBytes = mem.SwapTotal
	if mem.SwapTotal >= mem.SwapFree {
		st.SwapUsedBytes = mem.SwapTotal - mem.SwapFree
	}

	if raw, err := os.ReadFile("/proc/sys/vm/swappiness"); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
			st.Swappiness = v
		}
	}
	if raw, err := os.ReadFile("/proc/sys/vm/vfs_cache_pressure"); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
			st.VFSCachePressure = v
		}
	}

	if raw, err := os.ReadFile("/proc/swaps"); err == nil {
		st.Swaps = parseProcSwaps(string(raw))
		for _, sw := range st.Swaps {
			if sw.Filename == fikPanelSwapPath {
				st.HasFikPanelSwap = true
				break
			}
		}
	}
	if !st.HasFikPanelSwap {
		if _, err := os.Stat(fikPanelSwapPath); err == nil {
			st.HasFikPanelSwap = true
		}
	}

	// Baca ruang kosong partisi root '/' agar UI & backend dapat memvalidasi kapasitas aman swapfile
	for _, d := range readDiskPartitions(ctx) {
		if d.Mountpoint == "/" {
			st.RootFreeBytes = d.AvailBytes
			break
		}
	}

	return st, nil
}

// parseProcSwaps mem-parsing konten /proc/swaps menjadi slice SwapDevice
func parseProcSwaps(raw string) []SwapDevice {
	list := make([]SwapDevice, 0)
	sc := bufio.NewScanner(strings.NewReader(raw))
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		f := strings.Fields(sc.Text())
		if len(f) < 5 {
			continue
		}
		szKB, _ := strconv.ParseUint(f[2], 10, 64)
		usedKB, _ := strconv.ParseUint(f[3], 10, 64)
		prio, _ := strconv.Atoi(f[4])
		list = append(list, SwapDevice{
			Filename:  f[0],
			Type:      f[1],
			SizeBytes: szKB * 1024,
			UsedBytes: usedKB * 1024,
			Priority:  prio,
		})
	}
	return list
}

// CreateSwapfile membuat atau mengubah ukuran /swapfile.fikpanel secara aman dan mendaftarkannya ke /etc/fstab
func (m *Manager) CreateSwapfile(ctx context.Context, sizeMB int) error {
	if sizeMB < minSwapSizeMB || sizeMB > maxSwapSizeMB {
		return fmt.Errorf("ukuran swapfile harus antara %d MB hingga %d MB", minSwapSizeMB, maxSwapSizeMB)
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("pembuatan swapfile hanya didukung pada server Linux")
	}

	// Hitung sisa ruang kosong root (tambahkan ukuran swapfile lama bila akan diganti)
	st, err := m.GetMemoryOptimizationStatus(ctx)
	if err != nil {
		return err
	}
	var existingSize uint64
	if fi, err := os.Stat(fikPanelSwapPath); err == nil {
		existingSize = uint64(fi.Size())
	}
	neededBytes := uint64(sizeMB) * 1024 * 1024
	if st.RootFreeBytes+existingSize < neededBytes+minRootFreeBytes {
		return fmt.Errorf("ruang disk kosong pada '/' tidak cukup (wajib menyisakan minimal 2 GB setelah pembuatan swapfile)")
	}

	// Nonaktifkan swapfile FikPanel lama jika ada
	if existingSize > 0 {
		if out, err := exec.CommandContext(ctx, "swapoff", fikPanelSwapPath).CombinedOutput(); err != nil && !strings.Contains(string(out), "not active") {
			// Tetap lanjut jika belum aktif, gagal bila RAM terlalu penuh untuk menampung swap
			if !strings.Contains(strings.ToLower(string(out)), "invalid argument") {
				return fmt.Errorf("gagal menonaktifkan swapfile lama: %s", strings.TrimSpace(string(out)))
			}
		}
		_ = os.Remove(fikPanelSwapPath)
	}

	// Alokasikan berkas swap menggunakan fallocate (fallback ke dd bila filesystem tidak mendukung fallocate)
	if err := exec.CommandContext(ctx, "fallocate", "-l", fmt.Sprintf("%dM", sizeMB), fikPanelSwapPath).Run(); err != nil {
		if out, err2 := exec.CommandContext(ctx, "dd", "if=/dev/zero", "of="+fikPanelSwapPath, "bs=1M", fmt.Sprintf("count=%d", sizeMB)).CombinedOutput(); err2 != nil {
			_ = os.Remove(fikPanelSwapPath)
			return fmt.Errorf("gagal mengalokasikan berkas swap: %s (%w)", strings.TrimSpace(string(out)), err2)
		}
	}

	if err := os.Chmod(fikPanelSwapPath, 0600); err != nil {
		_ = os.Remove(fikPanelSwapPath)
		return fmt.Errorf("gagal mengatur izin 0600 pada swapfile: %w", err)
	}

	if out, err := exec.CommandContext(ctx, "mkswap", fikPanelSwapPath).CombinedOutput(); err != nil {
		_ = os.Remove(fikPanelSwapPath)
		return fmt.Errorf("gagal memformat swapfile (mkswap): %s (%w)", strings.TrimSpace(string(out)), err)
	}

	if out, err := exec.CommandContext(ctx, "swapon", fikPanelSwapPath).CombinedOutput(); err != nil {
		_ = os.Remove(fikPanelSwapPath)
		return fmt.Errorf("gagal mengaktifkan swapfile (swapon): %s (%w)", strings.TrimSpace(string(out)), err)
	}

	// Pastikan entri persisten tercatat di /etc/fstab agar otomatis aktif pasca-reboot
	_ = ensureFstabSwapEntry(fikPanelSwapPath, true)
	return nil
}

// RemoveSwapfile menonaktifkan dan menghapus /swapfile.fikpanel beserta entrinya di /etc/fstab
func (m *Manager) RemoveSwapfile(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("penghapusan swapfile hanya didukung pada server Linux")
	}
	if _, err := os.Stat(fikPanelSwapPath); os.IsNotExist(err) {
		_ = ensureFstabSwapEntry(fikPanelSwapPath, false)
		return fmt.Errorf("berkas %s tidak ditemukan", fikPanelSwapPath)
	}

	if out, err := exec.CommandContext(ctx, "swapoff", fikPanelSwapPath).CombinedOutput(); err != nil {
		low := strings.ToLower(string(out))
		if !strings.Contains(low, "not active") && !strings.Contains(low, "invalid argument") {
			return fmt.Errorf("gagal menonaktifkan swapfile (pastikan RAM tersedia mencukupi): %s", strings.TrimSpace(string(out)))
		}
	}
	if err := os.Remove(fikPanelSwapPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("gagal menghapus %s: %w", fikPanelSwapPath, err)
	}
	_ = ensureFstabSwapEntry(fikPanelSwapPath, false)
	return nil
}

// ensureFstabSwapEntry menambah atau menghapus baris swapfile FikPanel di /etc/fstab secara idempoten
func ensureFstabSwapEntry(swapPath string, add bool) error {
	raw, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	filtered := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), swapPath+" ") || strings.HasPrefix(strings.TrimSpace(line), swapPath+"\t") {
			continue
		}
		filtered = append(filtered, line)
	}
	if add {
		filtered = append(filtered, fmt.Sprintf("%s none swap sw 0 0", swapPath))
	}
	content := strings.TrimRight(strings.Join(filtered, "\n"), "\n") + "\n"
	return os.WriteFile("/etc/fstab", []byte(content), 0644)
}

// SetMemoryTuning menerapkan vm.swappiness dan vm.vfs_cache_pressure secara live dan persisten di sysctl.d
func (m *Manager) SetMemoryTuning(ctx context.Context, swappiness, vfsCachePressure int) error {
	if swappiness < 0 || swappiness > 100 {
		return fmt.Errorf("vm.swappiness harus antara 0 sampai 100")
	}
	if vfsCachePressure < 10 || vfsCachePressure > 500 {
		return fmt.Errorf("vm.vfs_cache_pressure harus antara 10 sampai 500")
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("tuning parameter kernel memori hanya tersedia pada host Linux")
	}

	if out, err := exec.CommandContext(ctx, "sysctl", "-w", fmt.Sprintf("vm.swappiness=%d", swappiness)).CombinedOutput(); err != nil {
		return fmt.Errorf("gagal mengatur vm.swappiness: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	if out, err := exec.CommandContext(ctx, "sysctl", "-w", fmt.Sprintf("vm.vfs_cache_pressure=%d", vfsCachePressure)).CombinedOutput(); err != nil {
		return fmt.Errorf("gagal mengatur vm.vfs_cache_pressure: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	body := fmt.Sprintf("# Konfigurasi optimasi memori kernel dikelola oleh FikPanel\nvm.swappiness = %d\nvm.vfs_cache_pressure = %d\n", swappiness, vfsCachePressure)
	_ = os.MkdirAll(filepath.Dir(memorySysctlPath), 0755)
	if err := os.WriteFile(memorySysctlPath, []byte(body), 0644); err != nil {
		return fmt.Errorf("tuning diterapkan sementara, namun gagal menyimpan %s: %w", memorySysctlPath, err)
	}
	return nil
}

// DropRAMCaches menjalankan sync disk diikuti pengosongan page/dentry/inode cache dan mengukur RAM yang dibebaskan
func (m *Manager) DropRAMCaches(ctx context.Context) (freedBytes int64, availAfter uint64, err error) {
	if runtime.GOOS != "linux" {
		return 0, 0, fmt.Errorf("drop caches kernel hanya tersedia pada host Linux")
	}
	before := readProcMemInfo()
	_ = exec.CommandContext(ctx, "sync").Run()
	if err := os.WriteFile("/proc/sys/vm/drop_caches", []byte("3\n"), 0644); err != nil {
		return 0, before.Available, fmt.Errorf("gagal mengosongkan RAM page cache: %w", err)
	}
	after := readProcMemInfo()
	var freed int64
	if after.Available > before.Available {
		freed = int64(after.Available - before.Available)
	}
	return freed, after.Available, nil
}
