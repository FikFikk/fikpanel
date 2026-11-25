package system

import (
	"context"
	"os"
	"testing"
)

// TestParseProcMemInfo memverifikasi pembacaan /proc/meminfo termasuk kalkulasi cache gabungan dan fallback Available
func TestParseProcMemInfo(t *testing.T) {
	raw := `MemTotal:        4000000 kB
MemFree:          500000 kB
MemAvailable:    2200000 kB
Buffers:          150000 kB
Cached:          1200000 kB
SReclaimable:     100000 kB
SwapTotal:       2097152 kB
SwapFree:        1048576 kB
`
	var snap procMemSnapshot
	parseProcMemInfo(raw, &snap)

	if snap.Total != 4000000*1024 {
		t.Fatalf("MemTotal tidak sesuai: got %d", snap.Total)
	}
	if snap.Available != 2200000*1024 {
		t.Fatalf("MemAvailable tidak sesuai: got %d", snap.Available)
	}
	wantCache := uint64(150000+1200000+100000) * 1024
	if snap.Cache != wantCache {
		t.Fatalf("MemCache diharapkan %d, dapat %d", wantCache, snap.Cache)
	}
	if snap.SwapTotal != 2097152*1024 || snap.SwapFree != 1048576*1024 {
		t.Fatalf("SwapTotal/SwapFree tidak sesuai: %+v", snap)
	}
}

// TestParseDiskPartitions memverifikasi pemilahan partisi fisik, penggabungan inode, dan penyaringan pseudo-filesystem
func TestParseDiskPartitions(t *testing.T) {
	dfBytes := `Filesystem     Type     1B-blocks      Used   Available Capacity Mounted on
/dev/sda1      ext4   50000000000 20000000000 30000000000      40% /
tmpfs          tmpfs   2000000000    10000000  1990000000       1% /run
/dev/loop0     squashfs 100000000   100000000           0     100% /snap/core/1
/dev/sdb1      xfs    100000000000 75000000000 25000000000     75% /www
`
	dfInodes := `Filesystem      Inodes  IUsed   IFree IUse% Mounted on
/dev/sda1      3200000 640000 2560000   20% /
/dev/sdb1      6400000 5760000  640000  90% /www
`
	parts := parseDiskPartitions(dfBytes, dfInodes)
	if len(parts) != 2 {
		t.Fatalf("diharapkan 2 partisi fisik (tmpfs & loop diabaikan), dapat %d: %+v", len(parts), parts)
	}
	if parts[0].Mountpoint != "/" || parts[0].InodesPct != 20 {
		t.Fatalf("partisi root tidak sesuai: %+v", parts[0])
	}
	if parts[1].Mountpoint != "/www" || parts[1].FSType != "xfs" || parts[1].InodesPct != 90 {
		t.Fatalf("partisi /www tidak sesuai: %+v", parts[1])
	}
}

// TestParsePSOutput memverifikasi parsing ps, filter nonroot/zombie, pencarian teks, dan flag proses terlindungi
func TestParsePSOutput(t *testing.T) {
	psOut := `  PID  PPID USER                             STAT %CPU %MEM    RSS NLWP     ELAPSED COMMAND
    1     0 root                             Ss    0.1  0.2  12288    1 10-02:15:00 /sbin/init
  712     1 root                             Ss    0.0  0.1   8192    1 10-02:14:50 /usr/sbin/sshd -D
 2310  1100 www-data                         R    42.5  3.4 131072    6    03:12:44 node /www/app/server.js
 2999  2310 www-data                         Z     0.0  0.0      0    1       01:12 [defunct-worker] <defunct>
`
	all := parsePSOutput(psOut, 20, "", "all")
	if len(all) != 4 {
		t.Fatalf("diharapkan 4 proses, dapat %d", len(all))
	}
	if !all[0].Protected || !all[1].Protected {
		t.Fatalf("PID 1 dan sshd wajib ditandai Protected=true")
	}
	if all[2].Protected {
		t.Fatalf("proses aplikasi node seharusnya Protected=false")
	}
	if all[2].Memory != 128.0 || all[2].Threads != 6 {
		t.Fatalf("konversi RSS MB atau threads salah: %+v", all[2])
	}

	zombies := parsePSOutput(psOut, 20, "", "zombie")
	if len(zombies) != 1 || zombies[0].PID != 2999 {
		t.Fatalf("filter zombie diharapkan hanya PID 2999, dapat %+v", zombies)
	}

	nonRoot := parsePSOutput(psOut, 20, "node", "nonroot")
	if len(nonRoot) != 1 || nonRoot[0].PID != 2310 {
		t.Fatalf("filter nonroot + search node diharapkan PID 2310, dapat %+v", nonRoot)
	}
}

// TestParseNetworkPortsAndConnections memverifikasi parsing soket ss -tulnp (IPv4/IPv6 + proteksi sshd) dan ss -tan
func TestParseNetworkPortsAndConnections(t *testing.T) {
	ssPorts := `Netid State  Recv-Q Send-Q Local Address:Port   Peer Address:Port Process
tcp   LISTEN 0      128          0.0.0.0:22          0.0.0.0:*     users:(("sshd",pid=712,fd=3))
tcp   LISTEN 0      511             [::]:3000           [::]:*     users:(("node",pid=2310,fd=18))
tcp   LISTEN 0      128        127.0.0.1:3306        0.0.0.0:*     users:(("mysqld",pid=1180,fd=21))
udp   UNCONN 0      0      127.0.0.53%lo:53          0.0.0.0:*     users:(("systemd-resolve",pid=510,fd=12))
`
	ports := parseSystemListeningPorts(ssPorts)
	if len(ports) != 4 {
		t.Fatalf("diharapkan 4 port listening, dapat %d: %+v", len(ports), ports)
	}

	// Terurut berdasarkan angka port: 22, 53, 3000, 3306
	if ports[0].Port != "22" || !ports[0].ExposedPublic || ports[0].CanKill {
		t.Fatalf("port 22 sshd seharusnya ExposedPublic=true dan CanKill=false: %+v", ports[0])
	}
	if ports[2].Port != "3000" || ports[2].Proto != "tcp6" || !ports[2].CanKill {
		t.Fatalf("port 3000 node seharusnya tcp6 dan CanKill=true: %+v", ports[2])
	}
	if ports[3].Port != "3306" || ports[3].ExposedPublic {
		t.Fatalf("port 3306 loopback seharusnya ExposedPublic=false: %+v", ports[3])
	}

	ssConns := `State      Recv-Q Send-Q Local Address:Port Peer Address:Port
LISTEN     0      128    0.0.0.0:22         0.0.0.0:*
ESTAB      0      0      10.0.0.2:443       1.2.3.4:52341
ESTAB      0      0      10.0.0.2:443       1.2.3.5:52342
TIME-WAIT  0      0      10.0.0.2:80        1.2.3.6:41001
SYN-RECV   0      0      10.0.0.2:443       1.2.3.7:59001
`
	counts := parseTCPConnectionStates(ssConns)
	if counts["ESTABLISHED"] != 2 || counts["LISTEN"] != 1 || counts["TIME-WAIT"] != 1 || counts["SYN-RECV"] != 1 || counts["TOTAL"] != 5 {
		t.Fatalf("distribusi state TCP tidak sesuai: %+v", counts)
	}
}

// TestParseProcSwaps memverifikasi parsing /proc/swaps dari satuan KB ke byte
func TestParseProcSwaps(t *testing.T) {
	raw := `Filename				Type		Size		Used		Priority
/swapfile.fikpanel                      file		2097148		1024		-2
`
	swaps := parseProcSwaps(raw)
	if len(swaps) != 1 {
		t.Fatalf("diharapkan 1 perangkat swap, dapat %d", len(swaps))
	}
	if swaps[0].Filename != "/swapfile.fikpanel" || swaps[0].SizeBytes != 2097148*1024 || swaps[0].Priority != -2 {
		t.Fatalf("entri swap tidak sesuai: %+v", swaps[0])
	}
}

// TestParseHumanBytesAndAptUpdates memverifikasi konversi ukuran human-readable dan parsing apt list --upgradable
func TestParseHumanBytesAndAptUpdates(t *testing.T) {
	if got := parseHumanBytes("Archived and active journals take up 256.0M in the file system."); got != 256*1024*1024 {
		t.Fatalf("parseHumanBytes(256.0M) salah: got %d", got)
	}
	if got := parseHumanBytes("1.5GB"); got != int64(1.5*1024*1024*1024) {
		t.Fatalf("parseHumanBytes(1.5GB) salah: got %d", got)
	}

	aptOut := `Listing...
openssl/jammy-security 3.0.2-0ubuntu1.18 amd64 [upgradable from: 3.0.2-0ubuntu1.16]
curl/jammy-updates 7.81.0-1ubuntu1.17 amd64 [upgradable from: 7.81.0-1ubuntu1.15]
`
	updates := parseAptUpgradable(aptOut)
	if len(updates) != 2 {
		t.Fatalf("diharapkan 2 paket upgradable, dapat %d", len(updates))
	}
	if !updates[0].IsSecurity || updates[0].Name != "openssl" || updates[0].CurrentVersion != "3.0.2-0ubuntu1.16" {
		t.Fatalf("paket keamanan openssl tidak sesuai: %+v", updates[0])
	}
	if updates[1].IsSecurity || updates[1].Name != "curl" {
		t.Fatalf("paket reguler curl tidak sesuai: %+v", updates[1])
	}
}

// TestClassifyDiagnosticLine memverifikasi deteksi insiden OOM Killer, warning kernel, dan error systemd
func TestClassifyDiagnosticLine(t *testing.T) {
	oom := classifyDiagnosticLine("[Mon Nov 25 10:15:02 2026] Out of memory: Killed process 4201 (php-fpm8.3) total-vm:1200MB", "kernel")
	if !oom.IsOOM || oom.Level != "oom" || oom.Timestamp == "" {
		t.Fatalf("deteksi baris OOM gagal: %+v", oom)
	}

	sysErr := classifyDiagnosticLine("2026-11-25T10:16:00+0700 vps01 nginx[901]: upstream timed out", "systemd")
	if sysErr.IsOOM || sysErr.Level != "error" || sysErr.Source != "systemd" {
		t.Fatalf("klasifikasi error systemd gagal: %+v", sysErr)
	}
}

// TestSafetyGuards memverifikasi seluruh pagar keamanan validasi (PID vital, sinyal, swap size, sysctl range, disk hogs allowlist, reboot hostname)
func TestSafetyGuards(t *testing.T) {
	mgr := NewManager()
	ctx := context.Background()

	if err := mgr.KillProcess(ctx, 1, "KILL"); err == nil {
		t.Fatal("KillProcess(1) seharusnya ditolak")
	}
	if err := mgr.KillProcess(ctx, os.Getpid(), "TERM"); err == nil {
		t.Fatal("KillProcess(self) seharusnya ditolak")
	}
	if err := mgr.KillProcess(ctx, 99999, "INVALID_SIG"); err == nil {
		t.Fatal("sinyal tidak dikenal seharusnya ditolak")
	}
	if err := mgr.ReniceProcess(ctx, 1, 5); err == nil {
		t.Fatal("ReniceProcess(1) seharusnya ditolak")
	}
	if err := mgr.ReniceProcess(ctx, 100, -20); err == nil {
		t.Fatal("nilai nice di luar batas (-10..19) seharusnya ditolak")
	}
	if _, err := mgr.AnalyzeDiskHogs(ctx, "/etc"); err == nil {
		t.Fatal("direktori di luar allowlist (/etc) seharusnya ditolak")
	}
	if err := mgr.CreateSwapfile(ctx, 128); err == nil {
		t.Fatal("swapfile di bawah 512 MB seharusnya ditolak")
	}
	if err := mgr.CreateSwapfile(ctx, 32768); err == nil {
		t.Fatal("swapfile di atas 16384 MB seharusnya ditolak")
	}
	if err := mgr.SetMemoryTuning(ctx, 150, 100); err == nil {
		t.Fatal("swappiness > 100 seharusnya ditolak")
	}
	if err := mgr.SetMemoryTuning(ctx, 30, 5); err == nil {
		t.Fatal("vfs_cache_pressure < 10 seharusnya ditolak")
	}
	if err := mgr.RebootHost(ctx, "hostname-yang-pasti-salah-9999"); err == nil {
		t.Fatal("RebootHost dengan hostname tidak cocok seharusnya ditolak")
	}
}
