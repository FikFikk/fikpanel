package tui

import (
	"fmt"
	"math"
	"strings"

	"fikpanel/internal/store"

	"github.com/charmbracelet/lipgloss"
)

// Palet warna resmi FikPanel untuk antarmuka terminal (Lava Orange + Emerald + Sky + Rose + Amber)
var (
	colAccent  = lipgloss.Color("#FF5A2B")
	colSuccess = lipgloss.Color("#10B981")
	colInfo    = lipgloss.Color("#38BDF8")
	colViolet  = lipgloss.Color("#818CF8")
	colWarning = lipgloss.Color("#F59E0B")
	colDanger  = lipgloss.Color("#F43F5E")
	colText    = lipgloss.Color("#F1F5F9")
	colMuted   = lipgloss.Color("#94A3B8")
	colSubtle  = lipgloss.Color("#64748B")
	colBorder  = lipgloss.Color("#2C3850")
	colSurface = lipgloss.Color("#131722")
	colRowSel  = lipgloss.Color("#1E2433")

	stBrand = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#121110")).
		Background(colAccent).
		Padding(0, 1)

	stHeaderMeta = lipgloss.NewStyle().Foreground(colMuted)
	stTitle      = lipgloss.NewStyle().Bold(true).Foreground(colText)
	stMuted      = lipgloss.NewStyle().Foreground(colMuted)
	stSubtle     = lipgloss.NewStyle().Foreground(colSubtle)
	stAccent     = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	stSuccess    = lipgloss.NewStyle().Bold(true).Foreground(colSuccess)
	stInfo       = lipgloss.NewStyle().Bold(true).Foreground(colInfo)
	stViolet     = lipgloss.NewStyle().Bold(true).Foreground(colViolet)
	stWarning    = lipgloss.NewStyle().Bold(true).Foreground(colWarning)
	stDanger     = lipgloss.NewStyle().Bold(true).Foreground(colDanger)

	stTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#121110")).
			Background(colAccent).
			Padding(0, 1)

	stTabInactive = lipgloss.NewStyle().
			Foreground(colMuted).
			Background(colSurface).
			Padding(0, 1)

	stCard = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(0, 1)

	stCardActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(0, 1)

	stTableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colSubtle)

	stRowSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(colText).
			Background(colRowSel)

	stBannerConfirm = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#121110")).
			Background(colWarning).
			Padding(0, 1)

	stBannerStatus = lipgloss.NewStyle().
			Foreground(colText).
			Background(colSurface).
			Padding(0, 1)

	stFooter = lipgloss.NewStyle().
			Foreground(colSubtle)

	stKeyHint = lipgloss.NewStyle().
			Bold(true).
			Foreground(colAccent)
)

// renderBar menggambar bilah progres blok dengan warna adaptif sesuai ambang batas
func renderBar(pct float64, width int) string {
	if width < 6 {
		width = 6
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(math.Round((pct / 100.0) * float64(width)))
	if filled > width {
		filled = width
	}
	barColor := colSuccess
	if pct >= 85 {
		barColor = colDanger
	} else if pct >= 70 {
		barColor = colWarning
	}
	fullPart := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", filled))
	emptyPart := lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("░", width-filled))
	return fullPart + emptyPart
}

// renderSparkline mengubah riwayat nilai (0..100) menjadi grafik batang Unicode ringkas
func renderSparkline(values []float64, width int, color lipgloss.Color) string {
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	if width < 8 {
		width = 8
	}
	start := 0
	if len(values) > width {
		start = len(values) - width
	}
	slice := values[start:]
	var sb strings.Builder
	pad := width - len(slice)
	for i := 0; i < pad; i++ {
		sb.WriteRune('▁')
	}
	for _, v := range slice {
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		idx := int(math.Round((v / 100.0) * float64(len(blocks)-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		sb.WriteRune(blocks[idx])
	}
	return lipgloss.NewStyle().Foreground(color).Render(sb.String())
}

// renderHeartbeats menggambar strip 20-bar heartbeat kronologis ala Uptime Kuma di terminal
func renderHeartbeats(checks []store.UptimeCheck, slots int) string {
	if slots <= 0 {
		slots = 20
	}
	upChar := lipgloss.NewStyle().Foreground(colSuccess).Render("▪")
	downChar := lipgloss.NewStyle().Foreground(colDanger).Render("▪")
	emptyChar := lipgloss.NewStyle().Foreground(colBorder).Render("▪")

	start := 0
	if len(checks) > slots {
		start = len(checks) - slots
	}
	visible := checks[start:]
	pad := slots - len(visible)

	var sb strings.Builder
	for i := 0; i < pad; i++ {
		sb.WriteString(emptyChar)
	}
	for _, c := range visible {
		if c.Status == "up" {
			sb.WriteString(upChar)
		} else {
			sb.WriteString(downChar)
		}
	}
	return sb.String()
}

// formatBytes memformat ukuran byte menjadi satuan manusiawi ringkas (B, KiB, MiB, GiB)
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	suffixes := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), suffixes[exp])
}

// truncate memotong string panjang secara aman agar kolom tabel tetap sejajar
func truncate(s string, max int) string {
	if max <= 1 {
		return ""
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max-1]) + "…"
}

// padRight memastikan teks memiliki lebar karakter tetap untuk perataan kolom
func padRight(s string, width int) string {
	r := []rune(s)
	if len(r) >= width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-len(r))
}
