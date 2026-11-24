package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUIHelpersAndFormatting(t *testing.T) {
	if got := formatBytes(512); got != "512 B" {
		t.Fatalf("formatBytes(512) = %q", got)
	}
	if got := formatBytes(2 * 1024 * 1024 * 1024); got != "2.0 GiB" {
		t.Fatalf("formatBytes(2GiB) = %q", got)
	}
	if bar := renderBar(75.0, 10); bar == "" {
		t.Fatalf("renderBar tidak boleh kosong")
	}
	if sp := renderSparkline([]float64{10, 40, 80, 95}, 12, colInfo); sp == "" {
		t.Fatalf("renderSparkline tidak boleh kosong")
	}
	checks := []store.UptimeCheck{{Status: "up"}, {Status: "down"}}
	if hb := renderHeartbeats(checks, 20); hb == "" {
		t.Fatalf("renderHeartbeats tidak boleh kosong")
	}
}

func TestTUIModelNavigationFilterAndConfirm(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_tui_test_*")
	if err != nil {
		t.Fatalf("gagal membuat tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	st, err := store.Open(filepath.Join(tempDir, "tui.db"))
	if err != nil {
		t.Fatalf("gagal membuka store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	_ = st.CreateUptimeMonitor(ctx, &store.UptimeMonitor{
		Name:          "API Utama",
		Target:        "https://api.example.com",
		MonitorType:   "http",
		IntervalSec:   60,
		TimeoutSec:    10,
		Enabled:       true,
		PublicVisible: true,
	})

	m := NewModel(Config{Version: "0.6.2", DBPath: filepath.Join(tempDir, "tui.db"), RefreshInterval: 2 * time.Second}, st)

	// Simulasi window resize + metric tick + snapshot loaded
	var next tea.Model = m
	next, _ = next.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	next, _ = next.Update(metricTickMsg{
		point: metrics.Point{CPU: 35.5, RAMUsed: 1024 * 1024 * 1024, RAMTotal: 2 * 1024 * 1024 * 1024, LoadAvg: 0.85},
		at:    time.Now(),
	})
	next, _ = next.Update(snapshotLoadedMsg{
		sites:    []store.Site{{ID: 1, Domain: "app.example.com", Kind: "php", Enabled: 1, SSLMode: "letsencrypt"}},
		uptime:   []store.UptimeMonitor{{ID: 1, Name: "API Utama", LastStatus: "up", Uptime24hPct: 99.9, LastLatencyMs: 32, SSLDaysRemaining: 45, Enabled: true}},
		services: []services.ServiceInfo{{Unit: "nginx.service", ActiveState: "active", Category: "web", MainPID: 1234}},
		runtimes: []runtimeEntry{{Kind: "docker", ID: "abc123", Name: "redis-cache", Sub: "redis:7", Status: "running", Running: true}},
		dockerUp: 1,
		mcpCount: 2,
	})

	// Verifikasi render tab Overview
	ovView := next.View()
	if !strings.Contains(ovView, "FIKPANEL TUI") || !strings.Contains(ovView, "Ringkasan Infrastruktur") {
		t.Fatalf("render Overview tidak lengkap: %s", ovView)
	}

	// Pindah ke tab Sites (2), Uptime (3), Services (4), Docker/PM2 (5)
	for _, tc := range []struct {
		key    string
		expect string
	}{
		{"2", "app.example.com"},
		{"3", "API Utama"},
		{"4", "nginx.service"},
		{"5", "redis-cache"},
	} {
		next, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
		if v := next.View(); !strings.Contains(v, tc.expect) {
			t.Fatalf("tab %s tidak menampilkan %q", tc.key, tc.expect)
		}
	}

	// Uji dialog konfirmasi restart (r) lalu batalkan (n) pada tab Docker/PM2
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if v := next.View(); !strings.Contains(v, "Konfirmasi restart") {
		t.Fatalf("prompt konfirmasi restart tidak muncul")
	}
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if v := next.View(); !strings.Contains(v, "Aksi dibatalkan") {
		t.Fatalf("pembatalan aksi tidak tercatat di status")
	}

	// Uji overlay log viewer dan penutupannya via Esc
	next, _ = next.Update(logOverlayMsg{title: "Test Log", content: "baris 1\npassword=rahasia_super_panjang_123"})
	if v := next.View(); !strings.Contains(v, "Test Log") || strings.Contains(v, "rahasia_super_panjang_123") {
		t.Fatalf("log overlay gagal merender atau gagal menyensor rahasia")
	}
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if mAfter, ok := next.(Model); !ok || mAfter.logOpen {
		t.Fatalf("log overlay seharusnya tertutup saat tekan Esc")
	}
}
