package cron

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// SystemCronEntry merepresentasikan satu entri cron sistem operasi (read-only) dari crontab atau /etc/cron.d
type SystemCronEntry struct {
	Source   string `json:"source"`
	File     string `json:"file"`
	Schedule string `json:"schedule"`
	User     string `json:"user"`
	Command  string `json:"command"`
}

// ListSystemCrons memindai crontab root, /etc/crontab, dan /etc/cron.d/* secara read-only pada host Linux
func ListSystemCrons(ctx context.Context) []SystemCronEntry {
	if runtime.GOOS != "linux" {
		return []SystemCronEntry{}
	}

	var entries []SystemCronEntry

	// 1. Pindai user crontab root (crontab -l)
	cCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	out, err := exec.CommandContext(cCtx, "crontab", "-l").Output()
	cancel()
	if err == nil && len(out) > 0 {
		entries = append(entries, parseCrontabContent(string(out), "crontab", "root", false)...)
	}

	// 2. Pindai /etc/crontab (format sistem memiliki kolom user di posisi ke-6)
	if raw, err := os.ReadFile("/etc/crontab"); err == nil {
		entries = append(entries, parseCrontabContent(string(raw), "system", "/etc/crontab", true)...)
	}

	// 3. Pindai seluruh berkas di /etc/cron.d/*
	if dirEntries, err := os.ReadDir("/etc/cron.d"); err == nil {
		for _, de := range dirEntries {
			name := de.Name()
			if de.IsDir() || strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") {
				continue
			}
			fullPath := filepath.Join("/etc/cron.d", name)
			if raw, err := os.ReadFile(fullPath); err == nil {
				entries = append(entries, parseCrontabContent(string(raw), "cron.d", fullPath, true)...)
			}
		}
	}

	if entries == nil {
		return []SystemCronEntry{}
	}
	return entries
}

// parseCrontabContent membedah baris aktif dari crontab OS (mendukung 5-field maupun @macro)
func parseCrontabContent(content, source, file string, hasUserCol bool) []SystemCronEntry {
	var list []SystemCronEntry
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Lewati deklarasi variabel lingkungan seperti SHELL=/bin/sh, PATH=..., MAILTO=...
		if !strings.HasPrefix(line, "@") {
			if eqIdx := strings.IndexByte(line, '='); eqIdx > 0 {
				left := strings.TrimSpace(line[:eqIdx])
				if !strings.ContainsAny(left, " \t*") {
					continue
				}
			}
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		if strings.HasPrefix(fields[0], "@") {
			if hasUserCol {
				if len(fields) < 3 {
					continue
				}
				list = append(list, SystemCronEntry{
					Source:   source,
					File:     file,
					Schedule: fields[0],
					User:     fields[1],
					Command:  strings.Join(fields[2:], " "),
				})
			} else {
				list = append(list, SystemCronEntry{
					Source:   source,
					File:     file,
					Schedule: fields[0],
					User:     "root",
					Command:  strings.Join(fields[1:], " "),
				})
			}
			continue
		}

		minCols := 6
		if hasUserCol {
			minCols = 7
		}
		if len(fields) < minCols {
			continue
		}
		schedule := strings.Join(fields[:5], " ")
		if hasUserCol {
			list = append(list, SystemCronEntry{
				Source:   source,
				File:     file,
				Schedule: schedule,
				User:     fields[5],
				Command:  strings.Join(fields[6:], " "),
			})
		} else {
			list = append(list, SystemCronEntry{
				Source:   source,
				File:     file,
				Schedule: schedule,
				User:     "root",
				Command:  strings.Join(fields[5:], " "),
			})
		}
	}
	return list
}
