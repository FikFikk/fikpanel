package security

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type FirewallRule struct {
	Number int    `json:"number"`
	To     string `json:"to"`
	Action string `json:"action"` // ALLOW, DENY
	From   string `json:"from"`
	Proto  string `json:"proto"`  // tcp, udp, any
}

type FirewallStatus struct {
	Enabled bool           `json:"enabled"`
	Rules   []FirewallRule `json:"rules"`
	Raw     string         `json:"raw"`
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) GetStatus(ctx context.Context) (*FirewallStatus, error) {
	if runtime.GOOS != "linux" {
		return &FirewallStatus{
			Enabled: true,
			Rules: []FirewallRule{
				{Number: 1, To: "22/tcp", Action: "ALLOW", From: "Anywhere", Proto: "tcp"},
				{Number: 2, To: "80/tcp", Action: "ALLOW", From: "Anywhere", Proto: "tcp"},
				{Number: 3, To: "443/tcp", Action: "ALLOW", From: "Anywhere", Proto: "tcp"},
			},
			Raw: "Simulasi UFW (Non-Linux)",
		}, nil
	}

	cmd := exec.CommandContext(ctx, "ufw", "status", "numbered")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if err != nil && !strings.Contains(output, "Status:") {
		return nil, fmt.Errorf("gagal mengecek status ufw: %s %w", stderr.String(), err)
	}

	status := &FirewallStatus{
		Enabled: strings.Contains(output, "Status: active"),
		Rules:   []FirewallRule{},
		Raw:     output,
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	inRules := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inRules = true
			rule := parseUFWRule(line)
			if rule != nil {
				status.Rules = append(status.Rules, *rule)
			}
		} else if inRules && line == "" {
			inRules = false
		}
	}

	return status, nil
}

func parseUFWRule(line string) *FirewallRule {
	closeBracket := strings.Index(line, "]")
	if closeBracket == -1 {
		return nil
	}
	numStr := strings.TrimSpace(line[1:closeBracket])
	num, _ := strconv.Atoi(numStr)

	rest := strings.TrimSpace(line[closeBracket+1:])
	fields := strings.Fields(rest)
	if len(fields) < 3 {
		return nil
	}

	to := fields[0]
	action := fields[1]
	from := strings.Join(fields[2:], " ")

	proto := "any"
	if strings.Contains(to, "/tcp") {
		proto = "tcp"
	} else if strings.Contains(to, "/udp") {
		proto = "udp"
	}

	return &FirewallRule{
		Number: num,
		To:     to,
		Action: action,
		From:   from,
		Proto:  proto,
	}
}

func (m *Manager) AddRule(ctx context.Context, port string, proto string, action string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if proto == "" {
		proto = "tcp"
	}
	if action == "" {
		action = "allow"
	}

	target := port
	if proto != "any" && !strings.Contains(port, "/") {
		target = fmt.Sprintf("%s/%s", port, proto)
	}

	cmd := exec.CommandContext(ctx, "ufw", action, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menambah aturan firewall: %s %w", string(out), err)
	}
	return nil
}

func (m *Manager) DeleteRule(ctx context.Context, ruleNum int) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "ufw", "--force", "delete", strconv.Itoa(ruleNum))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal menghapus aturan firewall #%d: %s %w", ruleNum, string(out), err)
	}
	return nil
}

func (m *Manager) ToggleFirewall(ctx context.Context, enable bool) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	action := "disable"
	if enable {
		action = "enable"
	}
	cmd := exec.CommandContext(ctx, "ufw", "--force", action)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal mengubah status firewall: %s %w", string(out), err)
	}
	return nil
}
