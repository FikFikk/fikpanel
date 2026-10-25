package ai

import (
	"context"
	"strings"
	"testing"

	"fikpanel/internal/metrics"
)

func TestRedactSecrets(t *testing.T) {
	input := "Kunci saya adalah api_key=\"sk-1234567890abcdef1234567890\" dan password: 'rahasiasuperkuat123'"
	clean := RedactSecrets(input)

	if strings.Contains(clean, "sk-1234567890abcdef1234567890") {
		t.Errorf("API key tidak tersensor: %s", clean)
	}
	if strings.Contains(clean, "rahasiasuperkuat123") {
		t.Errorf("Password tidak tersensor: %s", clean)
	}
	if !strings.Contains(clean, "[TEREDAKSI-SECRET]") {
		t.Errorf("Placeholder redaksi tidak ditemukan: %s", clean)
	}
}

func TestLocalFallback(t *testing.T) {
	hub := metrics.NewHub()
	c := NewClient(nil, hub, nil)

	var actions []map[string]string
	respNginx := c.localFallback("kenapa nginx saya 502 bad gateway?", &actions)
	if !strings.Contains(strings.ToLower(respNginx), "nginx") {
		t.Errorf("Respon Nginx tidak sesuai: %s", respNginx)
	}
	if len(actions) == 0 {
		t.Errorf("Aksi rekomendasi restart Nginx tidak muncul")
	}

	actions = nil
	respRAM := c.localFallback("cek pemakaian memory dan ram", &actions)
	if !strings.Contains(strings.ToLower(respRAM), "ram") {
		t.Errorf("Respon RAM tidak sesuai: %s", respRAM)
	}
}

func TestSystemPrompt(t *testing.T) {
	c := NewClient(nil, nil, nil)
	prompt := c.GenerateSystemPrompt(context.Background())
	if !strings.Contains(prompt, "AI Copilot") {
		t.Errorf("System prompt tidak memiliki identitas AI Copilot")
	}
	if !strings.Contains(prompt, "Read-Only Guardrail") {
		t.Errorf("System prompt tidak menegakkan read-only guardrail")
	}
}
