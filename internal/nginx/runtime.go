package nginx

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ═══ Status Beban (stub_status) ═══

// LoadStatus berisi metrik koneksi dan proses Nginx
type LoadStatus struct {
	Available   bool   `json:"available"`
	Hint        string `json:"hint,omitempty"`
	Active      int64  `json:"active"`
	Accepts     int64  `json:"accepts"`
	Handled     int64  `json:"handled"`
	Requests    int64  `json:"requests"`
	Reading     int64  `json:"reading"`
	Writing     int64  `json:"writing"`
	Waiting     int64  `json:"waiting"`
	Workers     int    `json:"workers"`
	MemoryBytes uint64 `json:"memory_bytes"`
	Timestamp   int64  `json:"timestamp"`
}

var (
	stubActiveRe = regexp.MustCompile(`Active connections:\s*(\d+)`)
	stubTotalsRe = regexp.MustCompile(`(?m)^\s*(\d+)\s+(\d+)\s+(\d+)\s*$`)
	stubRWWRe    = regexp.MustCompile(`Reading:\s*(\d+)\s+Writing:\s*(\d+)\s+Waiting:\s*(\d+)`)
)

// stubHint adalah panduan mengaktifkan stub_status bila endpoint belum tersedia
const stubHint = "Aktifkan stub_status: tambahkan di salah satu server block 'location = /nginx_status { stub_status; allow 127.0.0.1; deny all; }' lalu reload."

// Load mengambil metrik stub_status dan pemakaian memori proses Nginx
func (m *Manager) Load(ctx context.Context) (LoadStatus, error) {
	ls := LoadStatus{Timestamp: time.Now().Unix()}

	if m.mock {
		m.mu.Lock()
		running, started := m.mockRunning, m.mockStarted
		m.mu.Unlock()
		if !running {
			ls.Hint = "Nginx tidak berjalan"
			return ls, nil
		}
		// Simulasi metrik yang bergerak natural berbasis waktu untuk pengujian UI
		sec := time.Since(started).Seconds()
		now := time.Now().Unix()
		ls.Available = true
		ls.Accepts = int64(sec*4.2) + 120
		ls.Handled = ls.Accepts
		ls.Requests = int64(sec*11.7) + 480
		ls.Reading = now % 3
		ls.Writing = 1 + now%5
		ls.Waiting = 8 + (now*7)%23
		ls.Active = ls.Reading + ls.Writing + ls.Waiting
		ls.Workers = 4
		ls.MemoryBytes = uint64(30+now%6) * 1024 * 1024
		return ls, nil
	}

	procs := scanNginxProcs()
	ls.Workers = procs.Workers
	ls.MemoryBytes = procs.RSSBytes

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, m.paths.StatusURL, nil)
	if err != nil {
		return ls, fmt.Errorf("URL stub_status tidak valid: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ls.Hint = stubHint
		return ls, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil || resp.StatusCode != http.StatusOK {
		ls.Hint = stubHint
		return ls, nil
	}

	text := string(body)
	mA := stubActiveRe.FindStringSubmatch(text)
	mT := stubTotalsRe.FindStringSubmatch(text)
	mR := stubRWWRe.FindStringSubmatch(text)
	if mA == nil || mT == nil || mR == nil {
		ls.Hint = "Respons " + m.paths.StatusURL + " bukan format stub_status. " + stubHint
		return ls, nil
	}
	ls.Available = true
	ls.Active = atoi64(mA[1])
	ls.Accepts, ls.Handled, ls.Requests = atoi64(mT[1]), atoi64(mT[2]), atoi64(mT[3])
	ls.Reading, ls.Writing, ls.Waiting = atoi64(mR[1]), atoi64(mR[2]), atoi64(mR[3])
	return ls, nil
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// procSummary merangkum proses nginx yang ditemukan di /proc
type procSummary struct {
	Workers  int
	RSSBytes uint64
}

// scanNginxProcs memindai /proc untuk menghitung worker dan total RSS seluruh proses nginx
func scanNginxProcs() procSummary {
	var sum procSummary
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return sum
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		base := filepath.Join("/proc", e.Name())
		cmdline, err := os.ReadFile(filepath.Join(base, "cmdline"))
		if err != nil {
			continue
		}
		cmd := strings.ReplaceAll(string(cmdline), "\x00", " ")
		if !strings.HasPrefix(cmd, "nginx:") {
			continue
		}
		if strings.Contains(cmd, "worker process") {
			sum.Workers++
		}
		sum.RSSBytes += readVmRSS(filepath.Join(base, "status"))
	}
	return sum
}

// readVmRSS membaca nilai VmRSS (kB) dari /proc/<pid>/status dalam byte
func readVmRSS(path string) uint64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			kb, _ := strconv.ParseUint(fields[1], 10, 64)
			return kb * 1024
		}
	}
	return 0
}

// ═══ Ganti Versi ═══

// SwitchJob merepresentasikan status job ganti versi yang berjalan di background
type SwitchJob struct {
	Running    bool   `json:"running"`
	Target     string `json:"target"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
}

// VersionInfo berisi versi aktif, versi tersedia, dan status job
type VersionInfo struct {
	Current   string    `json:"current"`
	Available []string  `json:"available"`
	Job       SwitchJob `json:"job"`
}

var pkgVersionRe = regexp.MustCompile(`^[0-9][0-9A-Za-z.+~:-]*$`)

// Versions mengambil daftar versi paket nginx dari repositori (apt) atau daftar simulasi
func (m *Manager) Versions(ctx context.Context) (VersionInfo, error) {
	st, err := m.Status(ctx)
	if err != nil {
		return VersionInfo{}, err
	}
	m.mu.Lock()
	info := VersionInfo{Current: st.Version, Job: m.job}
	m.mu.Unlock()

	if m.mock {
		info.Available = []string{"1.28.0", "1.27.4", "1.26.3", "1.25.5", "1.24.0", "1.22.1"}
		return info, nil
	}

	available, err := aptVersions(ctx)
	if err != nil {
		return info, err
	}
	info.Available = available
	return info, nil
}

// aptVersions mem-parsing output `apt-cache madison nginx`
func aptVersions(ctx context.Context) ([]string, error) {
	if _, err := exec.LookPath("apt-cache"); err != nil {
		return nil, &ValidationError{Msg: "apt-cache tidak tersedia; ganti versi saat ini hanya mendukung Debian/Ubuntu"}
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "apt-cache", "madison", "nginx").Output()
	if err != nil {
		return nil, fmt.Errorf("gagal membaca daftar versi apt: %w", err)
	}
	seen := map[string]bool{}
	var list []string
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}
		v := strings.TrimSpace(parts[1])
		if v != "" && !seen[v] && pkgVersionRe.MatchString(v) {
			seen[v] = true
			list = append(list, v)
		}
	}
	return list, nil
}

// SwitchVersion memulai job background untuk memasang versi nginx tertentu
func (m *Manager) SwitchVersion(ctx context.Context, version string, userID *int64) error {
	version = strings.TrimSpace(version)
	if !pkgVersionRe.MatchString(version) {
		return &ValidationError{Msg: "format versi tidak valid"}
	}
	info, err := m.Versions(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, v := range info.Available {
		if v == version {
			found = true
			break
		}
	}
	if !found {
		return &ValidationError{Msg: "versi " + version + " tidak tersedia di repositori"}
	}

	m.mu.Lock()
	if m.job.Running {
		m.mu.Unlock()
		return &ConflictError{Msg: "job ganti versi lain masih berjalan"}
	}
	m.job = SwitchJob{Running: true, Target: version, StartedAt: time.Now().Unix(), Message: "Memasang nginx " + version + "..."}
	m.mu.Unlock()

	// Job dilepas dari context request agar tidak terbunuh saat HTTP selesai
	go m.runSwitch(version, userID)
	return nil
}

// runSwitch mengeksekusi instalasi versi dan memperbarui status job
func (m *Manager) runSwitch(version string, userID *int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var err error
	var output string
	if m.mock {
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			err = ctx.Err()
		}
		if err == nil {
			m.mu.Lock()
			m.mockVersion = version
			m.mockStarted = time.Now()
			m.mu.Unlock()
		}
	} else {
		// confold mempertahankan nginx.conf yang sudah dikustomisasi saat paket diganti
		cmd := exec.CommandContext(ctx, "apt-get", "install", "-y", "--allow-downgrades",
			"-o", "Dpkg::Options::=--force-confold", "nginx="+version)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		out, runErr := cmd.CombinedOutput()
		output = tailString(string(out), 1500)
		err = runErr
	}

	job := SwitchJob{Target: version, FinishedAt: time.Now().Unix()}
	m.mu.Lock()
	job.StartedAt = m.job.StartedAt
	m.mu.Unlock()

	if err != nil {
		job.Message = "Gagal memasang nginx " + version + ": " + err.Error()
		if output != "" {
			job.Message += "\n" + output
		}
	} else {
		job.OK = true
		job.Message = "Nginx berhasil diganti ke versi " + version
	}

	m.mu.Lock()
	m.job = job
	m.mu.Unlock()

	_ = m.audit.RecordAudit(context.Background(), "user", "nginx.switch_version", version, job.OK, job.Message, userID)
}

// ═══ Log Error ═══

// ErrorLog berisi potongan akhir file log error
type ErrorLog struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Lines   int    `json:"lines"`
	Content string `json:"content"`
}

// TailErrorLog membaca N baris terakhir log error (maksimal 1 MB dari akhir file)
func (m *Manager) TailErrorLog(lines int) (ErrorLog, error) {
	if err := m.ensureDevFiles(); err != nil {
		return ErrorLog{}, err
	}
	if lines <= 0 || lines > 5000 {
		lines = 200
	}
	res := ErrorLog{Path: m.paths.ErrorLog}

	f, err := os.Open(m.paths.ErrorLog)
	if errors.Is(err, os.ErrNotExist) {
		return res, nil
	}
	if err != nil {
		return res, fmt.Errorf("gagal membuka log error: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return res, fmt.Errorf("gagal membaca info log error: %w", err)
	}
	res.Size = fi.Size()

	const window = 1 << 20
	offset := int64(0)
	if fi.Size() > window {
		offset = fi.Size() - window
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return res, fmt.Errorf("gagal seek log error: %w", err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return res, fmt.Errorf("gagal membaca log error: %w", err)
	}

	text := strings.TrimRight(string(data), "\n")
	if text == "" {
		return res, nil
	}
	all := strings.Split(text, "\n")
	// Buang baris pertama yang kemungkinan terpotong bila membaca dari tengah file
	if offset > 0 && len(all) > 1 {
		all = all[1:]
	}
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	res.Lines = len(all)
	res.Content = strings.Join(all, "\n")
	return res, nil
}

// ClearErrorLog mengosongkan file log error tanpa menghapus file (handle nginx tetap valid)
func (m *Manager) ClearErrorLog() error {
	if err := m.ensureDevFiles(); err != nil {
		return err
	}
	if err := os.Truncate(m.paths.ErrorLog, 0); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("gagal mengosongkan log error: %w", err)
	}
	return nil
}

func tailString(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
