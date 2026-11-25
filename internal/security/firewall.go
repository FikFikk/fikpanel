package security

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"fikpanel/internal/store"
)

// Manager mengelola seluruh subsistem keamanan host: UFW, SSH, Brute Force, Hardening, Fail2Ban, FIM, dan Scanner
type Manager struct {
	store     *store.Store
	panelPort int

	mu             sync.RWMutex
	bfLoaded       bool
	bfCfg          BruteForceConfig
	ipCacheLoaded  bool
	whitelistCache []store.SecurityIPRule
	blacklistCache []store.SecurityIPRule

	// State lokal khusus mode pengembangan non-Linux agar simulasi mutasi tetap konsisten tanpa data fiktif statis
	devMu              sync.RWMutex
	devFirewallEnabled bool
	devDefaultIn       string
	devDefaultOut      string
	devRules           []FirewallRule
	devSSHConfig       SSHConfig
	devSysctl          map[string]bool
	devCompilerBlocked bool
	devFail2BanRunning bool
}

// NewManager menginisialisasi instance Security Manager dengan koneksi store SQLite dan port panel aktif
func NewManager(st *store.Store, panelPort int) *Manager {
	if panelPort <= 0 {
		panelPort = 1405
	}
	m := &Manager{
		store:              st,
		panelPort:          panelPort,
		devFirewallEnabled: false,
		devDefaultIn:       "deny",
		devDefaultOut:      "allow",
		devRules:           []FirewallRule{},
		devSSHConfig: SSHConfig{
			IsLinux:         false,
			Installed:       false,
			ActiveService:   false,
			Port:            22,
			PasswordAuth:    true,
			PubkeyAuth:      true,
			PermitRootLogin: "prohibit-password",
			MaxAuthTries:    6,
			ConfigSource:    "non-linux",
		},
		devSysctl: map[string]bool{
			"net.ipv4.icmp_echo_ignore_all":     false,
			"net.ipv4.tcp_syncookies":           true,
			"net.ipv4.conf.all.rp_filter":       true,
			"fs.suid_dumpable":                  true,
			"net.ipv4.conf.all.accept_redirects": true,
			"kernel.dmesg_restrict":             false,
		},
	}
	return m
}

// PanelPort mengembalikan port HTTP/HTTPS panel yang sedang berjalan
func (m *Manager) PanelPort() int {
	return m.panelPort
}

// GetStatus membaca status ringkas UFW beserta daftar aturan bernomor aktual
func (m *Manager) GetStatus(ctx context.Context) (*FirewallStatus, error) {
	if runtime.GOOS != "linux" {
		m.devMu.RLock()
		defer m.devMu.RUnlock()
		rules := make([]FirewallRule, len(m.devRules))
		copy(rules, m.devRules)
		return &FirewallStatus{
			Enabled:         m.devFirewallEnabled,
			Installed:       false,
			DefaultIncoming: m.devDefaultIn,
			DefaultOutgoing: m.devDefaultOut,
			Rules:           rules,
			Raw:             "Mode pengembangan non-Linux (UFW hanya aktif di host Linux)",
		}, nil
	}

	if _, err := exec.LookPath("ufw"); err != nil {
		return &FirewallStatus{
			Enabled:         false,
			Installed:       false,
			DefaultIncoming: "unknown",
			DefaultOutgoing: "unknown",
			Rules:           []FirewallRule{},
			Raw:             "Paket ufw belum terpasang di host Linux ini",
		}, nil
	}

	// Ambil kebijakan default dari `ufw status verbose`
	defIn, defOut := "deny", "allow"
	if vOut, err := exec.CommandContext(ctx, "ufw", "status", "verbose").CombinedOutput(); err == nil {
		defIn, defOut = parseUFWDefaultPolicies(string(vOut))
	}

	// Ambil daftar aturan bernomor dari `ufw status numbered`
	cmd := exec.CommandContext(ctx, "ufw", "status", "numbered")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if err != nil && !strings.Contains(output, "Status:") {
		return nil, fmt.Errorf("gagal memeriksa status UFW: %s %w", strings.TrimSpace(stderr.String()), err)
	}

	enabled, rules := parseUFWNumbered(output)
	return &FirewallStatus{
		Enabled:         enabled,
		Installed:       true,
		DefaultIncoming: defIn,
		DefaultOutgoing: defOut,
		Rules:           rules,
		Raw:             output,
	}, nil
}

// parseUFWDefaultPolicies mengekstrak kebijakan default incoming dan outgoing dari output `ufw status verbose`
func parseUFWDefaultPolicies(output string) (incoming, outgoing string) {
	incoming, outgoing = "deny", "allow"
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(strings.ToLower(line), "default:") {
			continue
		}
		rest := strings.TrimSpace(line[len("default:"):])
		parts := strings.Split(rest, ",")
		for _, p := range parts {
			p = strings.TrimSpace(strings.ToLower(p))
			if strings.Contains(p, "(incoming)") {
				fields := strings.Fields(p)
				if len(fields) > 0 {
					incoming = fields[0]
				}
			} else if strings.Contains(p, "(outgoing)") {
				fields := strings.Fields(p)
				if len(fields) > 0 {
					outgoing = fields[0]
				}
			}
		}
	}
	return incoming, outgoing
}

// Ekspresi reguler untuk mem-parsing satu baris aturan `ufw status numbered` (IPv4/IPv6, IN/OUT, komentar)
var ufwNumberedRe = regexp.MustCompile(`^\[\s*(\d+)\]\s+(.+?)\s+(ALLOW|DENY|REJECT|LIMIT)(?:\s+(IN|OUT|FWD))?\s+(.+)$`)

// parseUFWNumbered mem-parsing output lengkap `ufw status numbered` menjadi slice FirewallRule akurat
func parseUFWNumbered(output string) (bool, []FirewallRule) {
	enabled := false
	rules := make([]FirewallRule, 0)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		rawLine := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(rawLine, "Status:") {
			enabled = strings.Contains(strings.ToLower(rawLine), "active") && !strings.Contains(strings.ToLower(rawLine), "inactive")
			continue
		}
		if !strings.HasPrefix(rawLine, "[") {
			continue
		}
		if rule := parseUFWRuleLine(rawLine); rule != nil {
			rules = append(rules, *rule)
		}
	}
	return enabled, rules
}

// parseUFWRuleLine mem-parsing satu baris aturan UFW bernomor beserta komentar opsional (# ...)
func parseUFWRuleLine(line string) *FirewallRule {
	remarks := ""
	mainPart := line
	if hashIdx := strings.Index(line, " # "); hashIdx != -1 {
		remarks = strings.TrimSpace(line[hashIdx+3:])
		mainPart = strings.TrimSpace(line[:hashIdx])
	}

	m := ufwNumberedRe.FindStringSubmatch(mainPart)
	if len(m) != 6 {
		return nil
	}

	num, err := strconv.Atoi(m[1])
	if err != nil {
		return nil
	}
	toRaw := strings.TrimSpace(m[2])
	action := strings.ToUpper(strings.TrimSpace(m[3]))
	dirRaw := strings.ToUpper(strings.TrimSpace(m[4]))
	fromRaw := strings.TrimSpace(m[5])

	ipVersion := "IPv4"
	if strings.Contains(toRaw, "(v6)") || strings.Contains(fromRaw, "(v6)") || strings.Contains(fromRaw, ":") {
		ipVersion = "IPv6"
	}

	direction := "Inbound"
	if dirRaw == "OUT" || strings.Contains(toRaw, "(out)") || strings.Contains(fromRaw, "(out)") {
		direction = "Outbound"
	}

	// Bersihkan penanda (v6) dan (out) dari target dan sumber
	toClean := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(toRaw, "(v6)", ""), "(out)", ""))
	fromClean := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(fromRaw, "(v6)", ""), "(out)", ""))
	if fromClean == "" || strings.EqualFold(fromClean, "Anywhere") {
		if ipVersion == "IPv6" {
			fromClean = "::/0"
		} else {
			fromClean = "0.0.0.0/0"
		}
	}

	port, proto := splitPortProto(toClean)
	service := detectPortService(port, proto)

	return &FirewallRule{
		Number:    num,
		To:        toClean,
		Port:      port,
		Protocol:  proto,
		Action:    action,
		Direction: direction,
		From:      fromClean,
		IPVersion: ipVersion,
		Service:   service,
		Remarks:   remarks,
	}
}

// splitPortProto memecah format target UFW seperti "80/tcp", "39000:40000/tcp", "22", atau "Anywhere"
func splitPortProto(to string) (port, proto string) {
	to = strings.TrimSpace(to)
	if to == "" || strings.EqualFold(to, "Anywhere") {
		return "All", "any"
	}
	// Format seperti "192.168.1.5 80/tcp" -> ambil token terakhir
	fields := strings.Fields(to)
	target := fields[len(fields)-1]

	if strings.Contains(target, "/") {
		parts := strings.SplitN(target, "/", 2)
		return strings.ReplaceAll(parts[0], ":", "-"), strings.ToLower(parts[1])
	}
	return strings.ReplaceAll(target, ":", "-"), "any"
}

// detectPortService memberi label deskriptif berdasarkan nomor port standar
func detectPortService(port, proto string) string {
	p := strings.ToLower(proto)
	switch port {
	case "All", "any":
		return "Semua Port"
	case "20", "21":
		return "FTP"
	case "22":
		return "SSH"
	case "25":
		return "SMTP"
	case "53":
		if p == "udp" {
			return "DNS (UDP)"
		}
		return "DNS"
	case "80":
		return "HTTP"
	case "443":
		return "HTTPS"
	case "465", "587":
		return "SMTPS"
	case "888":
		return "phpMyAdmin"
	case "1405", "24441":
		return "FikPanel"
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
		if strings.Contains(port, "-") {
			return "Rentang Port"
		}
		if p == "udp" {
			return "Custom UDP"
		}
		return "Custom TCP"
	}
}

// GetExtendedFirewallStatus mengembalikan status lengkap firewall, soket listening, ICMP, dan ukuran log web
func (m *Manager) GetExtendedFirewallStatus(ctx context.Context) (*FirewallExtendedStatus, error) {
	basic, err := m.GetStatus(ctx)
	if err != nil {
		return nil, err
	}

	listeningPorts := m.DetectListeningPorts(ctx)
	listeningByPort := make(map[string]ListeningPort)
	for _, lp := range listeningPorts {
		if _, exists := listeningByPort[lp.Port]; !exists {
			listeningByPort[lp.Port] = lp
		}
	}

	sshCfg, _ := m.GetSSHConfig(ctx)
	sshPort := 22
	if sshCfg != nil && sshCfg.Port > 0 {
		sshPort = sshCfg.Port
	}
	sshPortStr := strconv.Itoa(sshPort)
	panelPortStr := strconv.Itoa(m.panelPort)

	inboundCount := 0
	outboundCount := 0
	for i := range basic.Rules {
		r := &basic.Rules[i]
		if r.Direction == "Outbound" {
			outboundCount++
		} else {
			inboundCount++
		}
		if lp, ok := listeningByPort[r.Port]; ok {
			r.Listening = true
			r.Process = lp.Process
		}
		// Tandai aturan port kritikal agar UI menampilkan proteksi lockout
		if r.Action == "ALLOW" && r.Direction == "Inbound" && (r.Port == sshPortStr || r.Port == panelPortStr) {
			r.Protected = true
		}
	}

	hostname, _ := os.Hostname()
	blockICMP := m.readSysctlBool("net.ipv4.icmp_echo_ignore_all", "1")
	logPath, logBytes := inspectSiteLogsDir()

	return &FirewallExtendedStatus{
		IsLinux:         runtime.GOOS == "linux",
		Installed:       basic.Installed,
		Enabled:         basic.Enabled,
		Hostname:        hostname,
		DefaultIncoming: basic.DefaultIncoming,
		DefaultOutgoing: basic.DefaultOutgoing,
		BlockICMP:       blockICMP,
		SSHPort:         sshPort,
		PanelPort:       m.panelPort,
		InboundCount:    inboundCount,
		OutboundCount:   outboundCount,
		ListeningCount:  len(listeningPorts),
		Rules:           basic.Rules,
		ListeningPorts:  listeningPorts,
		SiteLogsPath:    logPath,
		SiteLogsSize:    formatByteSize(logBytes),
		SiteLogsBytes:   logBytes,
		Raw:             basic.Raw,
	}, nil
}

// Ekspresi reguler untuk mengekstrak nama proses dan PID dari output `ss -tulnp`
var ssUsersRe = regexp.MustCompile(`"([^"]+)",pid=(\d+)`)

// DetectListeningPorts membaca seluruh soket TCP/UDP yang sedang LISTEN beserta proses & PID melalui `ss -tulnp`
func (m *Manager) DetectListeningPorts(ctx context.Context) []ListeningPort {
	if runtime.GOOS != "linux" {
		return []ListeningPort{}
	}

	out, err := exec.CommandContext(ctx, "ss", "-tulnp").Output()
	if err != nil {
		// Fallback tanpa flag -p jika tidak berjalan sebagai root
		out, err = exec.CommandContext(ctx, "ss", "-tuln").Output()
		if err != nil {
			return []ListeningPort{}
		}
	}
	return parseSSListeningOutput(string(out))
}

// parseSSListeningOutput mem-parsing output teks `ss -tulnp` menjadi slice ListeningPort terurut dan bebas duplikat
func parseSSListeningOutput(output string) []ListeningPort {
	result := make([]ListeningPort, 0)
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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

		localAddr := fields[4]
		addr, port := splitHostPortLoose(localAddr)
		if port == "" || port == "*" {
			continue
		}

		procName := ""
		pid := 0
		if len(fields) >= 7 {
			usersCol := strings.Join(fields[6:], " ")
			if m := ssUsersRe.FindStringSubmatch(usersCol); len(m) == 3 {
				procName = m[1]
				pid, _ = strconv.Atoi(m[2])
			}
		}

		proto := "tcp"
		if strings.HasPrefix(netid, "udp") {
			proto = "udp"
		}
		if strings.Contains(addr, ":") {
			proto += "6"
		}

		key := fmt.Sprintf("%s|%s|%s", proto, addr, port)
		if seen[key] {
			continue
		}
		seen[key] = true

		cleanAddr := strings.Trim(addr, "[]")
		exposed := cleanAddr == "0.0.0.0" || cleanAddr == "*" || cleanAddr == "::" || cleanAddr == ""
		if strings.HasPrefix(cleanAddr, "127.") || cleanAddr == "::1" {
			exposed = false
		}

		baseProto := "tcp"
		if strings.HasPrefix(proto, "udp") {
			baseProto = "udp"
		}
		result = append(result, ListeningPort{
			Protocol:      proto,
			Address:       cleanAddr,
			Port:          port,
			Process:       procName,
			PID:           pid,
			ExposedPublic: exposed,
			Service:       detectPortService(port, baseProto),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		pi, _ := strconv.Atoi(result[i].Port)
		pj, _ := strconv.Atoi(result[j].Port)
		if pi != pj {
			return pi < pj
		}
		return result[i].Protocol < result[j].Protocol
	})
	return result
}

// splitHostPortLoose memecah format alamat ss seperti "0.0.0.0:80", "[::]:443", "*:22", atau "127.0.0.53%lo:53"
func splitHostPortLoose(raw string) (addr, port string) {
	idx := strings.LastIndex(raw, ":")
	if idx == -1 || idx >= len(raw)-1 {
		return raw, ""
	}
	addr = raw[:idx]
	port = raw[idx+1:]
	// Hilangkan interface suffix seperti %lo atau %eth0
	if pct := strings.Index(addr, "%"); pct != -1 {
		addr = addr[:pct]
	}
	return addr, port
}

// ValidatePortSpec memvalidasi format nomor port tunggal (1-65535) atau rentang (start-end / start:end)
func ValidatePortSpec(raw string) (ufwPort string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "all") || strings.EqualFold(s, "any") {
		return "", nil
	}
	normalized := strings.ReplaceAll(s, "-", ":")
	if strings.Contains(normalized, ":") {
		parts := strings.Split(normalized, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("format rentang port tidak valid: %s (gunakan contoh 3000-4000)", raw)
		}
		start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil || start < 1 || end > 65535 || start >= end {
			return "", fmt.Errorf("rentang port tidak valid: %s (harus antara 1-65535)", raw)
		}
		return fmt.Sprintf("%d:%d", start, end), nil
	}

	p, err := strconv.Atoi(normalized)
	if err != nil || p < 1 || p > 65535 {
		return "", fmt.Errorf("nomor port tidak valid: %s (harus antara 1-65535)", raw)
	}
	return strconv.Itoa(p), nil
}

// sanitizeUFWComment membersihkan komentar agar aman dikirim sebagai argumen UFW
func sanitizeUFWComment(raw string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || r == '\'' || r == '"' || r == '`' || r == '\\' {
			return ' '
		}
		return r
	}, strings.TrimSpace(raw))
	if len(cleaned) > 80 {
		cleaned = cleaned[:80]
	}
	return strings.TrimSpace(cleaned)
}

// AddPortRule menambahkan aturan firewall lengkap (arah, aksi, protokol, port/rentang, sumber IP/CIDR, dan komentar)
func (m *Manager) AddPortRule(ctx context.Context, in AddFirewallRuleInput) error {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	switch action {
	case "", "allow":
		action = "allow"
	case "deny", "reject", "limit":
	default:
		return fmt.Errorf("aksi firewall tidak valid: %s (gunakan allow, deny, reject, atau limit)", in.Action)
	}

	direction := strings.ToLower(strings.TrimSpace(in.Direction))
	switch direction {
	case "", "in", "inbound":
		direction = "in"
	case "out", "outbound":
		direction = "out"
	default:
		return fmt.Errorf("arah trafik tidak valid: %s", in.Direction)
	}

	proto := strings.ToLower(strings.TrimSpace(in.Protocol))
	switch proto {
	case "", "tcp", "udp", "any":
		if proto == "" {
			proto = "tcp"
		}
	default:
		return fmt.Errorf("protokol tidak valid: %s (gunakan tcp, udp, atau any)", in.Protocol)
	}

	ufwPort, err := ValidatePortSpec(in.Port)
	if err != nil {
		return err
	}
	// Rentang port di UFW wajib mencantumkan protokol tcp atau udp
	if strings.Contains(ufwPort, ":") && proto == "any" {
		return fmt.Errorf("rentang port wajib menentukan protokol spesifik (tcp atau udp)")
	}

	src := strings.TrimSpace(in.SourceIP)
	isAnySrc := src == "" || strings.EqualFold(src, "any") || strings.EqualFold(src, "all") ||
		strings.EqualFold(src, "anywhere") || src == "0.0.0.0/0" || src == "::/0"
	if !isAnySrc {
		normIP, err := ValidateIPOrCIDR(src)
		if err != nil {
			return fmt.Errorf("alamat IP/CIDR sumber tidak valid: %w", err)
		}
		src = normIP
	}

	if ufwPort == "" && isAnySrc {
		return fmt.Errorf("isi nomor port atau alamat IP/CIDR spesifik agar aturan tidak membuka/menutup seluruh trafik tanpa sengaja")
	}

	comment := sanitizeUFWComment(in.Remarks)

	if runtime.GOOS != "linux" {
		m.devMu.Lock()
		defer m.devMu.Unlock()
		dirLabel := "Inbound"
		if direction == "out" {
			dirLabel = "Outbound"
		}
		srcLabel := src
		if isAnySrc {
			srcLabel = "0.0.0.0/0"
		}
		portLabel := strings.ReplaceAll(ufwPort, ":", "-")
		if portLabel == "" {
			portLabel = "All"
		}
		m.devRules = append(m.devRules, FirewallRule{
			Number:    len(m.devRules) + 1,
			To:        portLabel + "/" + proto,
			Port:      portLabel,
			Protocol:  proto,
			Action:    strings.ToUpper(action),
			Direction: dirLabel,
			From:      srcLabel,
			IPVersion: "IPv4",
			Service:   detectPortService(portLabel, proto),
			Remarks:   comment,
		})
		return nil
	}

	// Susun argumen perintah UFW tanpa shell interpolation (aman dari command injection)
	args := []string{action}
	if direction == "out" {
		args = append(args, "out")
	} else {
		args = append(args, "in")
	}

	if !isAnySrc {
		if direction == "out" {
			args = append(args, "to", src)
		} else {
			args = append(args, "from", src, "to", "any")
		}
		if ufwPort != "" {
			args = append(args, "port", ufwPort)
		}
		if proto != "any" {
			args = append(args, "proto", proto)
		}
	} else {
		// Sumber bebas (0.0.0.0/0), target berupa port/proto
		target := ufwPort
		if proto != "any" {
			target = fmt.Sprintf("%s/%s", ufwPort, proto)
		}
		args = append(args, target)
	}

	if comment != "" {
		args = append(args, "comment", comment)
	}

	cmd := exec.CommandContext(ctx, "ufw", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menambahkan aturan UFW: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// AddRule menyediakan kompatibilitas singkat penambahan aturan port inbound standar
func (m *Manager) AddRule(ctx context.Context, port, proto, action string) error {
	return m.AddPortRule(ctx, AddFirewallRuleInput{
		Port:      port,
		Protocol:  proto,
		Action:    action,
		Direction: "in",
	})
}

// DeleteRules menghapus satu atau banyak aturan UFW secara aman (diurutkan menurun agar nomor tidak bergeser)
func (m *Manager) DeleteRules(ctx context.Context, numbers []int, force bool) error {
	if len(numbers) == 0 {
		return fmt.Errorf("daftar nomor aturan yang akan dihapus kosong")
	}

	// Deduplikasi dan urutkan dari nomor terbesar ke terkecil
	unique := make([]int, 0, len(numbers))
	seen := make(map[int]bool)
	for _, n := range numbers {
		if n > 0 && !seen[n] {
			seen[n] = true
			unique = append(unique, n)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(unique)))

	// Periksa perlindungan anti-lockout port SSH & panel kecuali dipaksa dengan konfirmasi eksplisit
	if !force {
		status, err := m.GetStatus(ctx)
		if err == nil && status.Enabled {
			sshCfg, _ := m.GetSSHConfig(ctx)
			sshPortStr := "22"
			if sshCfg != nil && sshCfg.Port > 0 {
				sshPortStr = strconv.Itoa(sshCfg.Port)
			}
			panelPortStr := strconv.Itoa(m.panelPort)

			// Hitung jumlah aturan ALLOW yang tersisa untuk port SSH & panel setelah penghapusan
			remainSSH, remainPanel := 0, 0
			deletingSSH, deletingPanel := false, false
			for _, r := range status.Rules {
				if r.Action != "ALLOW" || r.Direction != "Inbound" {
					continue
				}
				if r.Port == sshPortStr {
					if seen[r.Number] {
						deletingSSH = true
					} else {
						remainSSH++
					}
				}
				if r.Port == panelPortStr {
					if seen[r.Number] {
						deletingPanel = true
					} else {
						remainPanel++
					}
				}
			}
			if deletingSSH && remainSSH == 0 {
				return fmt.Errorf("proteksi lockout: aturan ini adalah satu-satunya izin port SSH aktif (%s). Konfirmasi ulang dengan opsi paksa jika benar-benar ingin menghapus", sshPortStr)
			}
			if deletingPanel && remainPanel == 0 {
				return fmt.Errorf("proteksi lockout: aturan ini adalah satu-satunya izin port FikPanel (%s). Konfirmasi ulang dengan opsi paksa jika benar-benar ingin menghapus", panelPortStr)
			}
		}
	}

	if runtime.GOOS != "linux" {
		m.devMu.Lock()
		defer m.devMu.Unlock()
		filtered := make([]FirewallRule, 0, len(m.devRules))
		for _, r := range m.devRules {
			if !seen[r.Number] {
				r.Number = len(filtered) + 1
				filtered = append(filtered, r)
			}
		}
		m.devRules = filtered
		return nil
	}

	for _, num := range unique {
		cmd := exec.CommandContext(ctx, "ufw", "--force", "delete", strconv.Itoa(num))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("gagal menghapus aturan UFW #%d: %s (%w)", num, strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

// DeleteRule menghapus satu aturan firewall berdasarkan nomornya
func (m *Manager) DeleteRule(ctx context.Context, ruleNum int, force bool) error {
	return m.DeleteRules(ctx, []int{ruleNum}, force)
}

// ToggleFirewall mengaktifkan atau menonaktifkan UFW dengan jaminan port SSH & panel tetap terbuka saat diaktifkan
func (m *Manager) ToggleFirewall(ctx context.Context, enable bool) error {
	if runtime.GOOS != "linux" {
		m.devMu.Lock()
		m.devFirewallEnabled = enable
		m.devMu.Unlock()
		return nil
	}

	if enable {
		// Proteksi lockout otomatis: pastikan port SSH aktif dan port FikPanel diizinkan sebelum UFW dinyalakan
		sshPort := 22
		if cfg, err := m.GetSSHConfig(ctx); err == nil && cfg != nil && cfg.Port > 0 {
			sshPort = cfg.Port
		}
		_ = exec.CommandContext(ctx, "ufw", "allow", fmt.Sprintf("%d/tcp", sshPort), "comment", "FikPanel SSH Guard").Run()
		_ = exec.CommandContext(ctx, "ufw", "allow", fmt.Sprintf("%d/tcp", m.panelPort), "comment", "FikPanel Management").Run()
	}

	action := "disable"
	if enable {
		action = "enable"
	}
	cmd := exec.CommandContext(ctx, "ufw", "--force", action)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal mengubah status UFW: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// SetDefaultPolicy mengatur kebijakan bawaan incoming dan outgoing UFW (deny/allow/reject)
func (m *Manager) SetDefaultPolicy(ctx context.Context, incoming, outgoing string) error {
	in := strings.ToLower(strings.TrimSpace(incoming))
	outPol := strings.ToLower(strings.TrimSpace(outgoing))
	validPolicy := func(p string) bool {
		return p == "allow" || p == "deny" || p == "reject"
	}
	if !validPolicy(in) || !validPolicy(outPol) {
		return fmt.Errorf("kebijakan default tidak valid (gunakan allow, deny, atau reject)")
	}

	if runtime.GOOS != "linux" {
		m.devMu.Lock()
		m.devDefaultIn = in
		m.devDefaultOut = outPol
		m.devMu.Unlock()
		return nil
	}

	if out, err := exec.CommandContext(ctx, "ufw", "default", in, "incoming").CombinedOutput(); err != nil {
		return fmt.Errorf("gagal mengatur default incoming: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	if out, err := exec.CommandContext(ctx, "ufw", "default", outPol, "outgoing").CombinedOutput(); err != nil {
		return fmt.Errorf("gagal mengatur default outgoing: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// BlockIPInUFW menyisipkan aturan pemblokiran IP di posisi teratas UFW agar dievaluasi sebelum aturan ALLOW
func (m *Manager) BlockIPInUFW(ctx context.Context, ip, reason string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if _, err := exec.LookPath("ufw"); err != nil {
		return nil
	}
	normIP, err := ValidateIPOrCIDR(ip)
	if err != nil {
		return err
	}
	comment := sanitizeUFWComment("FikPanel Ban: " + reason)
	args := []string{"insert", "1", "deny", "from", normIP, "to", "any"}
	if comment != "" {
		args = append(args, "comment", comment)
	}
	out, err := exec.CommandContext(ctx, "ufw", args...).CombinedOutput()
	if err != nil {
		// Fallback bila UFW belum memiliki aturan apa pun (insert 1 gagal saat rule kosong)
		argsFallback := []string{"deny", "from", normIP, "to", "any"}
		if comment != "" {
			argsFallback = append(argsFallback, "comment", comment)
		}
		if out2, err2 := exec.CommandContext(ctx, "ufw", argsFallback...).CombinedOutput(); err2 != nil {
			return fmt.Errorf("gagal memblokir IP di UFW: %s / %s", strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)))
		}
	}
	return nil
}

// UnblockIPInUFW menghapus aturan pemblokiran IP dari UFW
func (m *Manager) UnblockIPInUFW(ctx context.Context, ip string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if _, err := exec.LookPath("ufw"); err != nil {
		return nil
	}
	normIP, err := ValidateIPOrCIDR(ip)
	if err != nil {
		return err
	}
	_ = exec.CommandContext(ctx, "ufw", "--force", "delete", "deny", "from", normIP, "to", "any").Run()
	return nil
}

// ClearSiteLogs mengosongkan isi berkas log webserver secara aman tanpa memutus handle file (truncate -s 0)
func (m *Manager) ClearSiteLogs(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	logDirs := []string{"/var/log/nginx", "/www/wwwlogs"}
	for _, dir := range logDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
				continue
			}
			_ = os.Truncate(filepath.Join(dir, e.Name()), 0)
		}
	}
	return nil
}

// inspectSiteLogsDir menghitung total ukuran seluruh berkas log di direktori log Nginx aktual
func inspectSiteLogsDir() (string, int64) {
	candidates := []string{"/var/log/nginx", "/www/wwwlogs"}
	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		var total int64
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				if fi, err := e.Info(); err == nil {
					total += fi.Size()
				}
			}
		}
		return dir, total
	}
	return "/var/log/nginx", 0
}

// formatByteSize memformat ukuran byte ke satuan manusiawi (B, KB, MB, GB)
func formatByteSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ValidateIPOrCIDR memvalidasi dan menormalisasi alamat IP tunggal atau subnet CIDR (IPv4/IPv6)
func ValidateIPOrCIDR(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("alamat IP tidak boleh kosong")
	}
	if strings.Contains(s, "/") {
		_, ipNet, err := net.ParseCIDR(s)
		if err != nil {
			return "", fmt.Errorf("format CIDR tidak valid: %s", s)
		}
		return ipNet.String(), nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return "", fmt.Errorf("format alamat IP tidak valid: %s", s)
	}
	return ip.String(), nil
}
