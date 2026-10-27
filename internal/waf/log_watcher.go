package waf

import (
	"bufio"
	"context"
	"database/sql"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LogWatcher memindai log akses Nginx secara berkala untuk mendeteksi ancaman dan telemetri serangan
type LogWatcher struct {
	db          *sql.DB
	fileOffsets map[string]int64
	mu          sync.Mutex
	logDir      string
	stopChan    chan struct{}
}

// Format log Nginx kombinasi standar:
// $remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
var reNginxLog = regexp.MustCompile(`^(\S+)\s+\S+\s+(\S+)\s+\[([^\]]+)\]\s+"([A-Z]+|\S+)\s+([^\s"]+)(?:\s+HTTP\/[0-9.]+)?(?:.*)?"\s+(\d{3})\s+(\d+)\s+"([^"]*)"\s+"([^"]*)"`)

// NewLogWatcher membuat instance watcher log akses Nginx
func NewLogWatcher(db *sql.DB, logDir string) *LogWatcher {
	if logDir == "" {
		logDir = "/var/log/nginx"
	}
	return &LogWatcher{
		db:          db,
		fileOffsets: make(map[string]int64),
		logDir:      logDir,
		stopChan:    make(chan struct{}),
	}
}

// Start menjalankan pemindaian berkala di latar belakang
func (w *LogWatcher) Start(ctx context.Context) {
	// 1. Pemindaian inisial: baca ekor berkas (tail) agar insiden terkini langsung terpetakan
	w.scanAllLogs(ctx, true)

	// 2. Loop pemindaian berkala setiap 4 detik
	ticker := time.NewTicker(4 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.scanAllLogs(ctx, false)
			case <-w.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop menghentikan watcher
func (w *LogWatcher) Stop() {
	close(w.stopChan)
}

// scanAllLogs memeriksa seluruh file *.access.log di folder log Nginx
func (w *LogWatcher) scanAllLogs(ctx context.Context, isInitial bool) {
	if _, err := os.Stat(w.logDir); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(w.logDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "access.log") {
			fullPath := filepath.Join(w.logDir, name)
			// Ekstrak nama situs dari nama file (misal: yuha.fikfikk.my.id.access.log -> yuha.fikfikk.my.id)
			siteName := strings.TrimSuffix(name, ".access.log")
			if siteName == "access" {
				siteName = "default-server"
			}
			w.processLogFile(ctx, fullPath, siteName, isInitial)
		}
	}
}

// processLogFile membaca baris-baris baru dari berkas log dan mencatat insiden serangan
func (w *LogWatcher) processLogFile(ctx context.Context, filePath, siteName string, isInitial bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return
	}

	currSize := fi.Size()
	lastOffset, exists := w.fileOffsets[filePath]

	// Jika pemindaian awal atau berkas di-rotasi (ukuran mengecil)
	if !exists || currSize < lastOffset {
		if isInitial {
			// Pada start awal, jika berkas besar, ambil 64KB terakhir agar tidak overload
			const maxInitialTail = 64 * 1024
			if currSize > maxInitialTail {
				lastOffset = currSize - maxInitialTail
			} else {
				lastOffset = 0
			}
		} else {
			lastOffset = 0
		}
	}

	if currSize == lastOffset {
		return
	}

	if _, err := f.Seek(lastOffset, io.SeekStart); err != nil {
		return
	}

	reader := bufio.NewReader(f)
	var bytesRead int64

	// Jika kita mulai dari tengah berkas karena tail cut, lewati baris parsial pertama
	if lastOffset > 0 && isInitial {
		line, err := reader.ReadBytes('\n')
		bytesRead += int64(len(line))
		if err != nil {
			w.fileOffsets[filePath] = currSize
			return
		}
	}

	now := time.Now().Unix()

	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			bytesRead += int64(len(line))
			lineStr := strings.TrimSpace(line)
			if lineStr != "" {
				w.parseAndRecordLine(ctx, lineStr, siteName, now)
			}
		}
		if err != nil {
			break
		}
	}

	w.fileOffsets[filePath] = lastOffset + bytesRead
}

// parseAndRecordLine membedah satu baris log dan memasukkan ke basis data jika merupakan serangan/probe
func (w *LogWatcher) parseAndRecordLine(ctx context.Context, line, siteName string, timestamp int64) {
	matches := reNginxLog.FindStringSubmatch(line)
	if len(matches) < 7 {
		return
	}

	clientIP := matches[1]
	method := strings.ToUpper(matches[4])
	rawPath := matches[5]
	statusCode, _ := strconv.Atoi(matches[6])
	userAgent := ""
	if len(matches) >= 10 {
		userAgent = matches[9]
	}

	// Evaluasi apakah permintaan ini mencurigakan atau merupakan serangan
	threat := ClassifyThreat(method, rawPath, userAgent, statusCode)
	if !threat.IsSuspicious {
		return
	}

	// Cek apakah serangan persis dari IP & path ini sudah dicatat dalam 60 detik terakhir (deduplikasi feed)
	var recentCount int
	_ = w.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM waf_events 
		WHERE client_ip = ? AND url_path = ? AND created_at >= ?
	`, clientIP, rawPath, timestamp-60).Scan(&recentCount)
	if recentCount > 0 {
		return
	}

	// Resolusi Geolokasi & Perangkat secara komprehensif
	geo := ResolveGeoLocation(clientIP)
	deviceInfo := ParseUserAgent(userAgent)

	// Simpan insiden serangan ke basis data waf_events
	_, err := w.db.ExecContext(ctx, `
		INSERT INTO waf_events (
			site_name, client_ip, country, country_code, region, city, isp,
			latitude, longitude, device, os, browser, method,
			attack_type, action, url_path, user_agent, rule_name, status_code, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		siteName, clientIP, geo.Country, geo.CountryCode, geo.Region, geo.City, geo.ISP,
		geo.Latitude, geo.Longitude, deviceInfo.Device, deviceInfo.OS, deviceInfo.Browser, method,
		threat.AttackType, threat.Action, rawPath, userAgent, threat.RuleName, statusCode, timestamp,
	)

	if err != nil {
		log.Printf("[WAF] Gagal merekam peristiwa serangan: %v", err)
	}
}
