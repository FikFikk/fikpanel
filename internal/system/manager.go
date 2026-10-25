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
	"strconv"
	"strings"
	"syscall"
)

type ProcessInfo struct {
	PID     int     `json:"pid"`
	User    string  `json:"user"`
	CPU     float64 `json:"cpu"`
	Memory  float64 `json:"memory"` // RSS dalam MB
	Command string  `json:"command"`
}

type PortListening struct {
	Proto   string `json:"proto"`   // tcp, udp
	Port    string `json:"port"`    // 80, 443, dll
	Address string `json:"address"` // 0.0.0.0, 127.0.0.1
	Program string `json:"program"` // nginx, mysqld, node
	PID     int    `json:"pid"`
}

type CleanerScanResult struct {
	AptCacheBytes   int64 `json:"apt_cache_bytes"`
	SystemLogBytes  int64 `json:"system_log_bytes"`
	TmpBytes        int64 `json:"tmp_bytes"`
	DockerReclaim   int64 `json:"docker_reclaim_bytes"`
	TotalBytes      int64 `json:"total_bytes"`
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) ListProcesses(ctx context.Context, limit int) ([]ProcessInfo, error) {
	if limit <= 0 {
		limit = 30
	}

	if runtime.GOOS != "linux" {
		return []ProcessInfo{
			{PID: 1, User: "root", CPU: 0.1, Memory: 12.5, Command: "/sbin/init"},
			{PID: 1024, User: "root", CPU: 1.2, Memory: 45.8, Command: "nginx: master process"},
			{PID: 1540, User: "mysql", CPU: 0.8, Memory: 210.4, Command: "/usr/sbin/mysqld"},
			{PID: 2100, User: "fikfikk", CPU: 0.5, Memory: 85.0, Command: "node /var/www/figo/server.js"},
		}, nil
	}

	// Gunakan ps aux untuk mendapatkan daftar proses terurut berdasarkan CPU/Mem
	cmd := exec.CommandContext(ctx, "ps", "-eo", "pid,user,%cpu,rss,args", "--sort=-%cpu")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gagal mengeksekusi ps: %w", err)
	}

	var list []ProcessInfo
	scanner := bufio.NewScanner(bytes.NewReader(out))
	isHeader := true

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if isHeader {
			isHeader = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		pid, _ := strconv.Atoi(fields[0])
		user := fields[1]
		cpu, _ := strconv.ParseFloat(fields[2], 64)
		rssKB, _ := strconv.ParseFloat(fields[3], 64)
		cmdText := strings.Join(fields[4:], " ")

		list = append(list, ProcessInfo{
			PID:     pid,
			User:    user,
			CPU:     cpu,
			Memory:  rssKB / 1024.0, // konversi ke MB
			Command: cmdText,
		})

		if len(list) >= limit {
			break
		}
	}

	return list, nil
}

func (m *Manager) KillProcess(ctx context.Context, pid int, force bool) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	sig := "-15"
	if force {
		sig = "-9"
	}
	cmd := exec.CommandContext(ctx, "kill", sig, strconv.Itoa(pid))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menghentikan proses #%d: %s %w", pid, string(out), err)
	}
	return nil
}

func (m *Manager) ListListeningPorts(ctx context.Context) ([]PortListening, error) {
	if runtime.GOOS != "linux" {
		return []PortListening{
			{Proto: "tcp", Port: "22", Address: "0.0.0.0", Program: "sshd", PID: 890},
			{Proto: "tcp", Port: "80", Address: "0.0.0.0", Program: "nginx", PID: 1024},
			{Proto: "tcp", Port: "443", Address: "0.0.0.0", Program: "nginx", PID: 1024},
			{Proto: "tcp", Port: "3306", Address: "127.0.0.1", Program: "mysqld", PID: 1540},
		}, nil
	}

	// Gunakan ss -tulpn
	cmd := exec.CommandContext(ctx, "ss", "-tulpn")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gagal mengeksekusi ss: %w", err)
	}

	var list []PortListening
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Netid") || strings.HasPrefix(line, "State") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		proto := strings.ToLower(fields[0])
		localAddr := fields[4]

		port := ""
		addr := ""
		lastColon := strings.LastIndex(localAddr, ":")
		if lastColon != -1 {
			port = localAddr[lastColon+1:]
			addr = localAddr[:lastColon]
		}

		program := "-"
		pid := 0
		if len(fields) >= 7 {
			procInfo := fields[6]
			// format: users:(("nginx",pid=1024,fd=6))
			if pStart := strings.Index(procInfo, `("`); pStart != -1 {
				if pEnd := strings.Index(procInfo[pStart+2:], `"`); pEnd != -1 {
					program = procInfo[pStart+2 : pStart+2+pEnd]
				}
			}
			if pidStart := strings.Index(procInfo, "pid="); pidStart != -1 {
				numStr := ""
				for _, ch := range procInfo[pidStart+4:] {
					if ch >= '0' && ch <= '9' {
						numStr += string(ch)
					} else {
						break
					}
				}
				pid, _ = strconv.Atoi(numStr)
			}
		}

		if port != "" {
			list = append(list, PortListening{
				Proto:   proto,
				Port:    port,
				Address: addr,
				Program: program,
				PID:     pid,
			})
		}
	}

	return list, nil
}

func (m *Manager) ScanJunk(ctx context.Context) (*CleanerScanResult, error) {
	res := &CleanerScanResult{}

	// 1. Apt cache
	res.AptCacheBytes = getDirSize("/var/cache/apt/archives")

	// 2. System logs (/var/log/*.gz, *.1)
	var logBytes int64
	_ = filepath.Walk("/var/log", func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			if strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".1") || strings.HasSuffix(path, ".old") {
				logBytes += info.Size()
			}
		}
		return nil
	})
	res.SystemLogBytes = logBytes

	// 3. /tmp & /var/tmp junk
	res.TmpBytes = getDirSize("/tmp") + getDirSize("/var/tmp")

	res.TotalBytes = res.AptCacheBytes + res.SystemLogBytes + res.TmpBytes
	return res, nil
}

func (m *Manager) CleanJunk(ctx context.Context, cleanApt bool, cleanLogs bool, cleanTmp bool, dropCaches bool) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	if cleanApt {
		cmd := exec.CommandContext(ctx, "apt-get", "clean")
		_ = cmd.Run()
	}

	if cleanLogs {
		_ = filepath.Walk("/var/log", func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				if strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".1") || strings.HasSuffix(path, ".old") {
					_ = os.Remove(path)
				}
			}
			return nil
		})
	}

	if cleanTmp {
		cmd := exec.CommandContext(ctx, "find", "/tmp", "-type", "f", "-atime", "+3", "-delete")
		_ = cmd.Run()
	}

	if dropCaches {
		// Sync disk dan drop RAM buffer cache aman: echo 3 > /proc/sys/vm/drop_caches
		syscall.Sync()
		_ = os.WriteFile("/proc/sys/vm/drop_caches", []byte("3\n"), 0644)
	}

	return nil
}

func getDirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
