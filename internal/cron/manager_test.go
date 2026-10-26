package cron

import (
	"strings"
	"testing"
	"time"
)

// TestMatchCronExpression menguji evaluasi ekspresi cron 5-field standar
func TestMatchCronExpression(t *testing.T) {
	// Waktu acuan: 2026-10-26 12:00:00 (Senin, Day 26, Month 10, Hour 12, Min 0)
	fixedTime := time.Date(2026, time.October, 26, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		expr     string
		expected bool
	}{
		{"Setiap hari jam 12:00", "0 12 * * *", true},
		{"Setiap jam menit 0", "0 * * * *", true},
		{"Setiap 15 menit", "*/15 * * * *", true},
		{"Jam 13 (tidak cocok)", "0 13 * * *", false},
		{"Hari Senin (1)", "0 12 * * 1", true},
		{"Hari Minggu (0)", "0 12 * * 0", false},
		{"Rentang jam 10-15", "0 10-15 * * *", true},
		{"Rentang jam 1-5", "0 1-5 * * *", false},
		{"Daftar jam 8,12,18", "0 8,12,18 * * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := strings.Fields(tt.expr)
			got := matchCronExpression(fields, fixedTime)
			if got != tt.expected {
				t.Errorf("matchCronExpression(%q) = %v, ingin %v", tt.expr, got, tt.expected)
			}
		})
	}
}
