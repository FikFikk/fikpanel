package updater

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// Status akhir dan sementara sebuah proses pembaruan.
const (
	StateRunning    = "running"
	StateSuccess    = "success"
	StateNoop       = "noop"
	StateFailed     = "failed"
	StateRolledBack = "rolled_back"
)

// Tahapan proses pembaruan yang sedang berjalan (field Status.Phase).
const (
	PhaseResolving   = "resolving"
	PhaseDownloading = "downloading"
	PhaseBackup      = "backup"
	PhaseInstalling  = "installing"
	PhaseRestarting  = "restarting"
	PhaseRollingBack = "rolling_back"
)

// ErrLocked menandakan ada proses pembaruan lain yang sedang memegang kunci.
var ErrLocked = errors.New("proses pembaruan lain sedang berjalan")

var runIDPattern = regexp.MustCompile(`^[0-9A-Za-z-]{1,64}$`)

// Status disimpan sebagai JSON di <StateDir>/status.json agar CLI dan dashboard dapat membaca hasil terakhir.
type Status struct {
	RunID      string `json:"run_id"`
	State      string `json:"state"`
	Phase      string `json:"phase,omitempty"`
	Trigger    string `json:"trigger"`
	Target     string `json:"target"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Message    string `json:"message,omitempty"`
	LogPath    string `json:"log_path,omitempty"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at,omitempty"`
}

// Terminal melaporkan apakah status sudah final (bukan sedang berjalan).
func (s Status) Terminal() bool { return s.State != "" && s.State != StateRunning }

// OK melaporkan apakah hasil pembaruan berhasil atau memang tidak diperlukan.
func (s Status) OK() bool { return s.State == StateSuccess || s.State == StateNoop }

// NewRunID membuat ID unik per eksekusi pembaruan (dipakai untuk nama berkas log).
func NewRunID(now time.Time) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return now.UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

// ValidRunID memastikan run ID aman dipakai sebagai bagian nama berkas.
func ValidRunID(id string) bool { return runIDPattern.MatchString(id) }

// StatusPath mengembalikan lokasi status.json di direktori state.
func StatusPath(stateDir string) string { return filepath.Join(stateDir, "status.json") }

// LogPath mengembalikan lokasi berkas log untuk satu eksekusi pembaruan.
func LogPath(stateDir, runID string) string {
	return filepath.Join(stateDir, "logs", "update-"+runID+".log")
}

// ReadStatus membaca status pembaruan terakhir.
func ReadStatus(stateDir string) (Status, error) {
	var st Status
	data, err := os.ReadFile(StatusPath(stateDir))
	if err != nil {
		return st, err
	}
	err = json.Unmarshal(data, &st)
	return st, err
}

func writeStatus(stateDir string, st Status) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(StatusPath(stateDir), append(data, '\n'), 0o644)
}

// writeFileAtomic menulis ke berkas sementara di direktori yang sama lalu rename, sehingga
// pembaca tidak pernah melihat isi berkas setengah jadi.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, writeErr := tmp.Write(data)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// pruneEntries menghapus entri lama di dir yang namanya cocok dengan pattern, menyisakan keep entri terbaru.
// Nama entri diawali timestamp sehingga urutan leksikal sama dengan urutan waktu.
func pruneEntries(dir string, pattern *regexp.Regexp, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if pattern.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for i := 0; i < len(names)-keep; i++ {
		_ = os.RemoveAll(filepath.Join(dir, names[i]))
	}
}
