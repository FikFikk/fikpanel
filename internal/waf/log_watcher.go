package waf

import (
	"bufio"
	"compress/gzip"
	"context"
	"database/sql"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// WAFAccessLog merepresentasikan satu baris log akses HTTP Nginx (baik normal maupun serangan) dengan telemetri penuh
type WAFAccessLog struct {
	ID          int64   `json:"id"`
	SiteName    string  `json:"site_name"`
	ClientIP    string  `json:"client_ip"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	ISP         string  `json:"isp"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Device      string  `json:"device"`
	OS          string  `json:"os"`
	Browser     string  `json:"browser"`
	Method      string  `json:"method"`
	URLPath     string  `json:"url_path"`
	Protocol    string  `json:"protocol"`
	StatusCode  int     `json:"status_code"`
	BytesSent   int64   `json:"bytes_sent"`
	Referer     string  `json:"referer"`
	UserAgent   string  `json:"user_agent"`
	IsThreat    bool    `json:"is_threat"`
	AttackType  string  `json:"attack_type"`
	Action      string  `json:"action"`
	RuleName    string  `json:"rule_name"`
	CreatedAt   int64   `json:"created_at"`
}

// LogAggregates menyimpan agregasi statistik trafik riil hasil pemindaian seluruh berkas log Nginx
type LogAggregates struct {
	TotalRequestsAllTime int64
	TodayRequests        int64
	YesterdayRequests    int64
	TotalBandwidthBytes  int64
	Status2xx            int64
	Status3xx            int64
	Status4xx            int64
	Status5xx            int64
	DomainStats          map[string]*DomainTrafficStat
	PageStats            map[string]*VisitedPageItem
	VisitorStats         map[string]*VisitorItem
}

type DomainTrafficStat struct {
	Domain         string
	Requests       int
	BandwidthBytes int64
	Threats        int
	Status2xx      int
	Status3xx      int
	Status4xx      int
	Status5xx      int
}

// LogWatcher memindai log akses Nginx secara berkala untuk mendeteksi ancaman dan telemetri akses detail
type LogWatcher struct {
	db           *sql.DB
	fileOffsets  map[string]int64
	mu           sync.Mutex
	accessMu     sync.RWMutex
	recentAccess []WAFAccessLog
	seqID        int64
	logDir       string
	stopChan     chan struct{}
}

// Format log Nginx kombinasi standar (mendukung request HTTP normal maupun payload biner mentah tanpa spasi):
// $remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
var reNginxLog = regexp.MustCompile(`^(\S+)\s+\S+\s+(\S+)\s+\[([^\]]+)\]\s+"([^"]*)"\s+(\d{3})\s+(\d+)\s+"([^"]*)"\s+"([^"]*)"`)

// NewLogWatcher membuat instance watcher log akses Nginx
func NewLogWatcher(db *sql.DB, logDir string) *LogWatcher {
	if logDir == "" {
		logDir = "/var/log/nginx"
	}
	return &LogWatcher{
		db:           db,
		fileOffsets:  make(map[string]int64),
		recentAccess: make([]WAFAccessLog, 0, 300),
		logDir:       logDir,
		stopChan:     make(chan struct{}),
	}
}

// Start menjalankan pemindaian berkala di latar belakang
func (w *LogWatcher) Start(ctx context.Context) {
	// 1. Pemindaian inisial: baca berkas rotasi (.access.log.1) dan berkas aktif (.access.log)
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

// extractSiteNameFromLogFile mengekstrak nama domain/situs dari nama berkas log Nginx
func extractSiteNameFromLogFile(name string) (string, bool) {
	if strings.HasSuffix(name, ".access.log") {
		site := strings.TrimSuffix(name, ".access.log")
		if site == "access" || site == "" {
			site = "default-server"
		}
		return site, true
	}
	if strings.HasSuffix(name, "access.log") {
		return "default-server", true
	}
	if strings.HasSuffix(name, ".access.log.1") {
		site := strings.TrimSuffix(name, ".access.log.1")
		if site == "access" || site == "" {
			site = "default-server"
		}
		return site, true
	}
	if name == "access.log.1" {
		return "default-server", true
	}
	return "", false
}

// scanAllLogs memeriksa seluruh file *.access.log (serta *.access.log.1 saat inisialisasi) di folder log Nginx
func (w *LogWatcher) scanAllLogs(ctx context.Context, isInitial bool) {
	if _, err := os.Stat(w.logDir); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(w.logDir)
	if err != nil {
		return
	}

	// Pada saat inisialisasi, pindai berkas .access.log.1 terlebih dahulu agar urutan waktu kronologis
	if isInitial {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasSuffix(name, "access.log.1") {
				if siteName, ok := extractSiteNameFromLogFile(name); ok {
					w.processLogFile(ctx, filepath.Join(w.logDir, name), siteName, true)
				}
			}
		}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "access.log") {
			if siteName, ok := extractSiteNameFromLogFile(name); ok {
				w.processLogFile(ctx, filepath.Join(w.logDir, name), siteName, isInitial)
			}
		}
	}
}

// processLogFile membaca baris-baris baru dari berkas log dan mencatat telemetri akses serta insiden serangan
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
			// Pada start awal, jika berkas besar, ambil 128KB terakhir agar mencakup ratusan akses terkini
			const maxInitialTail = 128 * 1024
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

// parseNginxRequestString memecah string "$request" Nginx menjadi method, path, dan protocol
func parseNginxRequestString(reqStr string) (method, rawPath, protocol string) {
	trimmed := strings.TrimSpace(reqStr)
	if trimmed == "" || trimmed == "-" {
		return "RAW", "/", "HTTP/1.1"
	}
	parts := strings.Fields(trimmed)
	if len(parts) >= 3 {
		return strings.ToUpper(parts[0]), parts[1], parts[len(parts)-1]
	}
	if len(parts) == 2 {
		return strings.ToUpper(parts[0]), parts[1], "HTTP/1.1"
	}
	// Jika tidak ada spasi (misal payload biner \x05\x01\x00 atau \x16\x03\x01...)
	display := trimmed
	if len(display) > 80 {
		display = display[:80] + "..."
	}
	return "RAW", display, "BINARY"
}

// parseNginxTimestamp mengonversi waktu lokal Nginx (02/Jan/2006:15:04:05 -0700) ke epoch Unix
func parseNginxTimestamp(rawTime string, fallback int64) int64 {
	if t, err := time.Parse("02/Jan/2006:15:04:05 -0700", strings.TrimSpace(rawTime)); err == nil {
		return t.Unix()
	}
	return fallback
}

// parseAndRecordLine membedah satu baris log, menyimpan ke ring buffer akses live, dan mencatat ke waf_events bila mencurigakan
func (w *LogWatcher) parseAndRecordLine(ctx context.Context, line, siteName string, fallbackTS int64) {
	matches := reNginxLog.FindStringSubmatch(line)
	if len(matches) < 9 {
		return
	}

	clientIP := matches[1]
	timestamp := parseNginxTimestamp(matches[3], fallbackTS)
	method, rawPath, protocol := parseNginxRequestString(matches[4])
	statusCode, _ := strconv.Atoi(matches[5])
	bytesSent, _ := strconv.ParseInt(matches[6], 10, 64)
	referer := matches[7]
	if referer == "-" {
		referer = ""
	}
	userAgent := matches[8]
	if userAgent == "-" {
		userAgent = ""
	}

	// Evaluasi apakah permintaan ini mencurigakan atau merupakan serangan
	threat := ClassifyThreat(method, rawPath, userAgent, statusCode)
	geo := ResolveGeoLocation(clientIP)
	deviceInfo := ParseUserAgent(userAgent)

	entry := WAFAccessLog{
		ID:          atomic.AddInt64(&w.seqID, 1),
		SiteName:    siteName,
		ClientIP:    clientIP,
		Country:     geo.Country,
		CountryCode: geo.CountryCode,
		Region:      geo.Region,
		City:        geo.City,
		ISP:         geo.ISP,
		Latitude:    geo.Latitude,
		Longitude:   geo.Longitude,
		Device:      deviceInfo.Device,
		OS:          deviceInfo.OS,
		Browser:     deviceInfo.Browser,
		Method:      method,
		URLPath:     rawPath,
		Protocol:    protocol,
		StatusCode:  statusCode,
		BytesSent:   bytesSent,
		Referer:     referer,
		UserAgent:   userAgent,
		IsThreat:    threat.IsSuspicious,
		AttackType:  threat.AttackType,
		Action:      threat.Action,
		RuleName:    threat.RuleName,
		CreatedAt:   timestamp,
	}

	w.appendAccessLog(entry)

	if !threat.IsSuspicious {
		return
	}

	// Cek apakah serangan persis dari IP & path ini sudah dicatat dalam 60 detik terakhir (deduplikasi feed)
	var recentCount int
	_ = w.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM waf_events 
		WHERE client_ip = ? AND url_path = ? AND ABS(created_at - ?) <= 60
	`, clientIP, rawPath, timestamp).Scan(&recentCount)
	if recentCount > 0 {
		return
	}

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

// appendAccessLog menambahkan satu entri akses ke dalam ring buffer in-memory (maksimal 350 entri terbaru)
func (w *LogWatcher) appendAccessLog(entry WAFAccessLog) {
	w.accessMu.Lock()
	defer w.accessMu.Unlock()

	w.recentAccess = append(w.recentAccess, entry)
	if len(w.recentAccess) > 350 {
		w.recentAccess = w.recentAccess[len(w.recentAccess)-350:]
	}
}

// GetRecentAccessLogs mengembalikan daftar log akses HTTP terbaru (terurut dari yang paling baru)
func (w *LogWatcher) GetRecentAccessLogs(limit int) []WAFAccessLog {
	if limit <= 0 {
		limit = 100
	}
	w.accessMu.RLock()
	copied := make([]WAFAccessLog, len(w.recentAccess))
	copy(copied, w.recentAccess)
	w.accessMu.RUnlock()

	// Urutkan dari timestamp terbaru ke terlama
	sort.SliceStable(copied, func(i, j int) bool {
		if copied[i].CreatedAt == copied[j].CreatedAt {
			return copied[i].ID > copied[j].ID
		}
		return copied[i].CreatedAt > copied[j].CreatedAt
	})

	if len(copied) > limit {
		copied = copied[:limit]
	}
	return copied
}

// ComputeLogAggregates memindai berkas log Nginx di disk untuk menghitung metrik trafik riil (hari ini, kemarin, bandwidth, status, endpoint, pengunjung)
func (w *LogWatcher) ComputeLogAggregates() *LogAggregates {
	agg := &LogAggregates{
		DomainStats:  make(map[string]*DomainTrafficStat),
		PageStats:    make(map[string]*VisitedPageItem),
		VisitorStats: make(map[string]*VisitorItem),
	}

	entries, err := os.ReadDir(w.logDir)
	if err != nil {
		return agg
	}

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	startOfYesterday := startOfToday - 86400

	processReader := func(r io.Reader, siteName string, parseDetailed bool) {
		scanner := bufio.NewScanner(r)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 256*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			agg.TotalRequestsAllTime++

			dStat, exists := agg.DomainStats[siteName]
			if !exists {
				dStat = &DomainTrafficStat{Domain: siteName}
				agg.DomainStats[siteName] = dStat
			}
			dStat.Requests++

			if !parseDetailed {
				continue
			}

			matches := reNginxLog.FindStringSubmatch(line)
			if len(matches) < 9 {
				continue
			}

			clientIP := matches[1]
			ts := parseNginxTimestamp(matches[3], 0)
			method, rawPath, _ := parseNginxRequestString(matches[4])
			statusCode, _ := strconv.Atoi(matches[5])
			bytesSent, _ := strconv.ParseInt(matches[6], 10, 64)
			userAgent := matches[8]

			if ts >= startOfToday {
				agg.TodayRequests++
			} else if ts >= startOfYesterday {
				agg.YesterdayRequests++
			}

			agg.TotalBandwidthBytes += bytesSent
			dStat.BandwidthBytes += bytesSent

			switch {
			case statusCode >= 200 && statusCode < 300:
				agg.Status2xx++
				dStat.Status2xx++
			case statusCode >= 300 && statusCode < 400:
				agg.Status3xx++
				dStat.Status3xx++
			case statusCode >= 400 && statusCode < 500:
				agg.Status4xx++
				dStat.Status4xx++
			case statusCode >= 500:
				agg.Status5xx++
				dStat.Status5xx++
			}

			threat := ClassifyThreat(method, rawPath, userAgent, statusCode)
			if threat.IsSuspicious {
				dStat.Threats++
			}

			// Jangan masukkan aset statis (.css, .js, .woff, .png, .ico) ke dalam Top Visited Pages kecuali jika status >= 400
			cleanPath := rawPath
			if qIdx := strings.IndexByte(cleanPath, '?'); qIdx > 0 && !threat.IsSuspicious {
				cleanPath = cleanPath[:qIdx]
			}
			lowerClean := strings.ToLower(cleanPath)
			isStaticAsset := strings.HasSuffix(lowerClean, ".js") || strings.HasSuffix(lowerClean, ".css") ||
				strings.HasSuffix(lowerClean, ".woff") || strings.HasSuffix(lowerClean, ".woff2") ||
				strings.HasSuffix(lowerClean, ".png") || strings.HasSuffix(lowerClean, ".jpg") ||
				strings.HasSuffix(lowerClean, ".svg") || strings.HasSuffix(lowerClean, ".ico")

			if !isStaticAsset || statusCode >= 400 || threat.IsSuspicious {
				pageKey := siteName + "|" + method + "|" + cleanPath
				pStat, ok := agg.PageStats[pageKey]
				if !ok {
					pStat = &VisitedPageItem{
						SiteName:   siteName,
						Path:       cleanPath,
						Method:     method,
						StatusCode: statusCode,
						IsThreat:   threat.IsSuspicious,
					}
					agg.PageStats[pageKey] = pStat
				}
				pStat.Count++
				pStat.BytesTotal += bytesSent
				if threat.IsSuspicious {
					pStat.IsThreat = true
				}
			}

			// Catat statistik pengunjung per IP
			vStat, ok := agg.VisitorStats[clientIP]
			if !ok {
				geo := ResolveGeoLocation(clientIP)
				dev := ParseUserAgent(userAgent)
				vStat = &VisitorItem{
					IP:          clientIP,
					Country:     geo.Country,
					CountryCode: geo.CountryCode,
					City:        geo.City,
					ISP:         geo.ISP,
					Device:      dev.Device,
					OS:          dev.OS,
					Browser:     dev.Browser,
				}
				agg.VisitorStats[clientIP] = vStat
			}
			vStat.Count++
			vStat.BytesTotal += bytesSent
			if threat.IsSuspicious {
				vStat.ThreatCount++
			}
			if ts >= vStat.LastSeen {
				vStat.LastSeen = ts
				vStat.LastSite = siteName
				vStat.LastPath = cleanPath
			}
		}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		fullPath := filepath.Join(w.logDir, name)

		if siteName, ok := extractSiteNameFromLogFile(name); ok {
			if f, err := os.Open(fullPath); err == nil {
				processReader(f, siteName, true)
				_ = f.Close()
			}
			continue
		}

		// Hitung juga total baris dari arsip log terkompresi (.access.log.*.gz) untuk total all-time
		if strings.Contains(name, "access.log.") && strings.HasSuffix(name, ".gz") {
			siteName := strings.Split(name, ".access.log")[0]
			if siteName == "" || siteName == "access" {
				siteName = "default-server"
			}
			if f, err := os.Open(fullPath); err == nil {
				if gr, err := gzip.NewReader(f); err == nil {
					processReader(gr, siteName, false)
					_ = gr.Close()
				}
				_ = f.Close()
			}
		}
	}

	return agg
}

