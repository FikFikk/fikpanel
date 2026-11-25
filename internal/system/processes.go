package system

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// isProtectedProcess menandai proses kritikal sistem yang tidak boleh dihentikan sembarangan
func isProtectedProcess(pid int, cmd string) bool {
	if pid <= 1 || pid == os.Getpid() {
		return true
	}
	lower := strings.TrimSpace(strings.ToLower(cmd))
	if lower == "sshd" || lower == "systemd" || lower == "init" {
		return true
	}
	protectedKeywords := []string{"/sbin/init", "systemd-journald", "systemd-udevd", "/usr/sbin/sshd", "sshd:", "fikpanel"}
	for _, kw := range protectedKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// ListProcesses membaca daftar proses sistem aktual melalui `ps` dengan dukungan sorting, filter, dan pencarian
func (m *Manager) ListProcesses(ctx context.Context, limit int, sortBy, search, filter string) ([]ProcessInfo, error) {
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	if runtime.GOOS != "linux" {
		return []ProcessInfo{}, nil
	}

	sortFlag := "--sort=-%cpu"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "mem", "memory", "rss":
		sortFlag = "--sort=-rss"
	case "pid":
		sortFlag = "--sort=pid"
	}

	cmd := exec.CommandContext(ctx, "ps", "-eo", "pid,ppid,user:32,stat,%cpu,%mem,rss,nlwp,etime,args", sortFlag)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gagal membaca daftar proses (ps): %w", err)
	}
	return parsePSOutput(string(out), limit, search, filter), nil
}

// parsePSOutput mem-parsing output `ps -eo pid,ppid,user:32,stat,%cpu,%mem,rss,nlwp,etime,args`
func parsePSOutput(output string, limit int, search, filter string) []ProcessInfo {
	q := strings.ToLower(strings.TrimSpace(search))
	fMode := strings.ToLower(strings.TrimSpace(filter))

	list := make([]ProcessInfo, 0, limit)
	scanner := bufio.NewScanner(strings.NewReader(output))
	isHeader := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if isHeader {
			isHeader = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, _ := strconv.Atoi(fields[1])
		uname := fields[2]
		stat := fields[3]
		cpu, _ := strconv.ParseFloat(fields[4], 64)
		memPct, _ := strconv.ParseFloat(fields[5], 64)
		rssKB, _ := strconv.ParseFloat(fields[6], 64)
		threads, _ := strconv.Atoi(fields[7])
		elapsed := fields[8]
		cmdText := strings.Join(fields[9:], " ")

		// Terapkan filter kategori proses
		if fMode == "nonroot" && uname == "root" {
			continue
		}
		if fMode == "zombie" && !strings.HasPrefix(stat, "Z") {
			continue
		}
		// Terapkan filter pencarian teks
		if q != "" {
			if !strings.Contains(strings.ToLower(cmdText), q) &&
				!strings.Contains(strings.ToLower(uname), q) &&
				!strings.Contains(fields[0], q) {
				continue
			}
		}

		list = append(list, ProcessInfo{
			PID:       pid,
			PPID:      ppid,
			User:      uname,
			State:     string(stat[0]),
			CPU:       cpu,
			MemPct:    memPct,
			Memory:    rssKB / 1024.0,
			Threads:   threads,
			Elapsed:   elapsed,
			Command:   cmdText,
			Protected: isProtectedProcess(pid, cmdText),
		})
		if len(list) >= limit {
			break
		}
	}
	return list
}

// InspectProcess membaca detail lengkap satu proses langsung dari /proc/<pid>
func (m *Manager) InspectProcess(ctx context.Context, pid int) (*ProcessDetail, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("PID tidak valid")
	}
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("inspeksi /proc hanya tersedia di host Linux")
	}

	procDir := fmt.Sprintf("/proc/%d", pid)
	statusRaw, err := os.ReadFile(procDir + "/status")
	if err != nil {
		return nil, fmt.Errorf("proses #%d sudah tidak aktif atau tidak dapat diakses", pid)
	}

	detail := &ProcessDetail{PID: pid}
	var uidStr string
	sc := bufio.NewScanner(bytes.NewReader(statusRaw))
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		switch k {
		case "Name":
			detail.Name = v
		case "State":
			detail.State = v
		case "PPid":
			detail.PPID, _ = strconv.Atoi(v)
		case "Threads":
			detail.Threads, _ = strconv.Atoi(v)
		case "Uid":
			f := strings.Fields(v)
			if len(f) > 0 {
				uidStr = f[0]
			}
		case "VmRSS":
			f := strings.Fields(v)
			if len(f) > 0 {
				kb, _ := strconv.ParseUint(f[0], 10, 64)
				detail.RSSBytes = kb * 1024
			}
		case "VmSwap":
			f := strings.Fields(v)
			if len(f) > 0 {
				kb, _ := strconv.ParseUint(f[0], 10, 64)
				detail.SwapBytes = kb * 1024
			}
		}
	}

	if uidStr != "" {
		if u, err := user.LookupId(uidStr); err == nil {
			detail.User = u.Username
		} else {
			detail.User = uidStr
		}
	}
	if cmdBytes, err := os.ReadFile(procDir + "/cmdline"); err == nil {
		detail.Cmdline = strings.TrimSpace(strings.ReplaceAll(string(cmdBytes), "\x00", " "))
	}
	if detail.Cmdline == "" {
		detail.Cmdline = "[" + detail.Name + "]"
	}
	if exe, err := os.Readlink(procDir + "/exe"); err == nil {
		detail.Exe = exe
	}
	if cwd, err := os.Readlink(procDir + "/cwd"); err == nil {
		detail.Cwd = cwd
	}
	if fds, err := os.ReadDir(procDir + "/fd"); err == nil {
		detail.FDCount = len(fds)
	}
	if oomRaw, err := os.ReadFile(procDir + "/oom_score"); err == nil {
		detail.OOMScore, _ = strconv.Atoi(strings.TrimSpace(string(oomRaw)))
	}
	// Ambil nilai nice dari kolom ke-19 /proc/<pid>/stat (setelah tanda tutup kurung nama proses)
	if statRaw, err := os.ReadFile(procDir + "/stat"); err == nil {
		s := string(statRaw)
		if idx := strings.LastIndex(s, ")"); idx != -1 && idx+2 < len(s) {
			fields := strings.Fields(s[idx+2:])
			if len(fields) >= 17 {
				detail.Nice, _ = strconv.Atoi(fields[16])
			}
		}
	}

	detail.Protected = isProtectedProcess(pid, detail.Cmdline+" "+detail.Name)
	return detail, nil
}

// KillProcess mengirim sinyal aman (TERM, KILL, HUP) ke proses target dengan proteksi proses vital
func (m *Manager) KillProcess(ctx context.Context, pid int, signal string) error {
	if pid <= 1 {
		return fmt.Errorf("dilarang keras menghentikan proses vital sistem (PID <= 1)")
	}
	if pid == os.Getpid() {
		return fmt.Errorf("dilarang menghentikan proses daemon FikPanel sendiri (PID #%d)", pid)
	}

	sig := "-15"
	switch strings.ToUpper(strings.TrimSpace(signal)) {
	case "", "TERM", "SIGTERM":
		sig = "-15"
	case "KILL", "SIGKILL", "FORCE":
		sig = "-9"
	case "HUP", "SIGHUP":
		sig = "-1"
	default:
		return fmt.Errorf("sinyal tidak didukung: %s (gunakan TERM, KILL, atau HUP)", signal)
	}

	if runtime.GOOS != "linux" {
		return fmt.Errorf("penghentian proses hanya tersedia pada host server Linux")
	}

	// Cek apakah proses target termasuk daemon kritikal (sshd / systemd-journald / init)
	if detail, err := m.InspectProcess(ctx, pid); err == nil && detail.Protected {
		return fmt.Errorf("proses #%d (%s) dilindungi karena krusial bagi akses server", pid, detail.Name)
	}

	cmd := exec.CommandContext(ctx, "kill", sig, strconv.Itoa(pid))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal mengirim sinyal %s ke proses #%d: %s (%w)", sig, pid, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ReniceProcess mengubah prioritas penjadwalan CPU proses (`renice`) dalam batas aman (-10 sampai 19)
func (m *Manager) ReniceProcess(ctx context.Context, pid int, priority int) error {
	if pid <= 1 {
		return fmt.Errorf("tidak dapat mengubah prioritas PID <= 1")
	}
	if priority < -10 || priority > 19 {
		return fmt.Errorf("nilai nice harus antara -10 (prioritas tinggi) hingga 19 (prioritas rendah)")
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("renice proses hanya tersedia pada host Linux")
	}

	out, err := exec.CommandContext(ctx, "renice", "-n", strconv.Itoa(priority), "-p", strconv.Itoa(pid)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal mengubah prioritas proses #%d: %s (%w)", pid, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ── Network & Port Explorer ──────────────────────────────────────────────────

var ssProcExtractRe = regexp.MustCompile(`"([^"]+)",pid=(\d+)`)

// GetNetworkOverview membaca seluruh soket listening (ss -tulnp) dan ringkasan status koneksi TCP (ss -tan)
func (m *Manager) GetNetworkOverview(ctx context.Context) (*NetworkOverview, error) {
	ov := &NetworkOverview{
		IsLinux: runtime.GOOS == "linux",
		Ports:   []PortListening{},
		Connections: map[string]int{
			"ESTABLISHED": 0,
			"TIME-WAIT":   0,
			"LISTEN":      0,
			"SYN-RECV":    0,
			"CLOSE-WAIT":  0,
			"TOTAL":       0,
		},
	}

	if runtime.GOOS != "linux" {
		return ov, nil
	}

	out, err := exec.CommandContext(ctx, "ss", "-tulnp").Output()
	if err != nil {
		out, err = exec.CommandContext(ctx, "ss", "-tuln").Output()
		if err != nil {
			return nil, fmt.Errorf("gagal menjalankan ss: %w", err)
		}
	}
	ov.Ports = parseSystemListeningPorts(string(out))

	// Hitung distribusi status koneksi TCP aktif melalui `ss -tan`
	if connOut, err := exec.CommandContext(ctx, "ss", "-tan").Output(); err == nil {
		ov.Connections = parseTCPConnectionStates(string(connOut))
	}
	return ov, nil
}

// ListListeningPorts menyediakan kompatibilitas langsung bagi konsumen internal (seperti MCP Server)
func (m *Manager) ListListeningPorts(ctx context.Context) ([]PortListening, error) {
	ov, err := m.GetNetworkOverview(ctx)
	if err != nil {
		return nil, err
	}
	return ov.Ports, nil
}

// parseSystemListeningPorts mem-parsing output `ss -tulnp` secara akurat termasuk IPv6 dan flag proteksi
func parseSystemListeningPorts(output string) []PortListening {
	list := make([]PortListening, 0)
	seen := make(map[string]bool)

	sc := bufio.NewScanner(strings.NewReader(output))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "Netid") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		netid := strings.ToLower(fields[0])
		state := strings.ToUpper(fields[1])
		if strings.HasPrefix(netid, "tcp") && state != "LISTEN" {
			continue
		}

		local := fields[4]
		idx := strings.LastIndex(local, ":")
		if idx == -1 || idx >= len(local)-1 {
			continue
		}
		addr := local[:idx]
		port := local[idx+1:]
		if pct := strings.Index(addr, "%"); pct != -1 {
			addr = addr[:pct]
		}
		cleanAddr := strings.Trim(addr, "[]")
		if port == "" || port == "*" {
			continue
		}

		proto := "tcp"
		if strings.HasPrefix(netid, "udp") {
			proto = "udp"
		}
		if strings.Contains(cleanAddr, ":") {
			proto += "6"
		}

		key := proto + "|" + cleanAddr + "|" + port
		if seen[key] {
			continue
		}
		seen[key] = true

		program := "-"
		pid := 0
		if len(fields) >= 7 {
			usersCol := strings.Join(fields[6:], " ")
			if m := ssProcExtractRe.FindStringSubmatch(usersCol); len(m) == 3 {
				program = m[1]
				pid, _ = strconv.Atoi(m[2])
			}
		}

		exposed := cleanAddr == "0.0.0.0" || cleanAddr == "*" || cleanAddr == "::" || cleanAddr == ""
		if strings.HasPrefix(cleanAddr, "127.") || cleanAddr == "::1" {
			exposed = false
		}

		canKill := pid > 1 && !isProtectedProcess(pid, program)
		list = append(list, PortListening{
			Proto:         proto,
			Port:          port,
			Address:       cleanAddr,
			Program:       program,
			PID:           pid,
			ExposedPublic: exposed,
			Service:       detectCommonService(port),
			CanKill:       canKill,
		})
	}

	sort.Slice(list, func(i, j int) bool {
		pi, _ := strconv.Atoi(list[i].Port)
		pj, _ := strconv.Atoi(list[j].Port)
		if pi != pj {
			return pi < pj
		}
		return list[i].Proto < list[j].Proto
	})
	return list
}

// parseTCPConnectionStates menghitung jumlah koneksi TCP per state dari output `ss -tan`
func parseTCPConnectionStates(output string) map[string]int {
	counts := map[string]int{
		"ESTABLISHED": 0,
		"TIME-WAIT":   0,
		"LISTEN":      0,
		"SYN-RECV":    0,
		"CLOSE-WAIT":  0,
		"TOTAL":       0,
	}
	sc := bufio.NewScanner(strings.NewReader(output))
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		st := strings.ToUpper(fields[0])
		if st == "ESTAB" {
			st = "ESTABLISHED"
		} else if st == "TIME-WAIT" || st == "TIMEWAIT" {
			st = "TIME-WAIT"
		}
		counts["TOTAL"]++
		if _, tracked := counts[st]; tracked {
			counts[st]++
		}
	}
	return counts
}

func detectCommonService(port string) string {
	switch port {
	case "22":
		return "SSH"
	case "53":
		return "DNS"
	case "80":
		return "HTTP"
	case "443":
		return "HTTPS"
	case "1405":
		return "FikPanel"
	case "3000":
		return "Node.js / Web"
	case "3306":
		return "MySQL / MariaDB"
	case "5432":
		return "PostgreSQL"
	case "6379":
		return "Redis"
	case "8080", "8443":
		return "HTTP Proxy"
	case "27017":
		return "MongoDB"
	default:
		return "Aplikasi"
	}
}
