package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var dockerHubHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// SearchDockerHub mencari citra di Docker Hub via HTTP API v2 (kaya metadata & pull count),
// dengan fallback otomatis ke CLI `docker search` bila jaringan hub API tidak terjangkau.
func (m *Manager) SearchDockerHub(ctx context.Context, query string, page int) ([]DockerHubSearchItem, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		q = "nginx"
	}
	if page <= 0 {
		page = 1
	}

	items, err := searchDockerHubAPI(ctx, q, page)
	if err == nil && len(items) > 0 {
		return items, nil
	}

	// Fallback ke CLI `docker search` dengan parsing kunci PascalCase yang benar
	return m.searchDockerCLI(ctx, q)
}

// SearchImages mempertahankan alias kompatibilitas untuk panggilan lama
func (m *Manager) SearchImages(ctx context.Context, query string) ([]DockerHubSearchItem, error) {
	return m.SearchDockerHub(ctx, query, 1)
}

func searchDockerHubAPI(ctx context.Context, query string, page int) ([]DockerHubSearchItem, error) {
	endpoint := fmt.Sprintf(
		"https://hub.docker.com/v2/search/repositories/?query=%s&page=%d&page_size=25",
		url.QueryEscape(query),
		page,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "FikPanel-DockerStudio/0.7")

	resp, err := dockerHubHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Docker Hub HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Results []struct {
			RepoName    string `json:"repo_name"`
			ShortDesc   string `json:"short_description"`
			StarCount   int    `json:"star_count"`
			PullCount   int64  `json:"pull_count"`
			IsOfficial  bool   `json:"is_official"`
			IsAutomated bool   `json:"is_automated"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&payload); err != nil {
		return nil, err
	}

	results := make([]DockerHubSearchItem, 0, len(payload.Results))
	for _, r := range payload.Results {
		name := strings.TrimSpace(r.RepoName)
		if name == "" {
			continue
		}
		results = append(results, DockerHubSearchItem{
			Name:        name,
			Description: strings.TrimSpace(r.ShortDesc),
			StarCount:   r.StarCount,
			PullCount:   r.PullCount,
			IsOfficial:  r.IsOfficial,
			IsAutomated: r.IsAutomated,
		})
	}
	return results, nil
}

func (m *Manager) searchDockerCLI(ctx context.Context, query string) ([]DockerHubSearchItem, error) {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, "docker", "search", "--limit", "25", "--format", "{{json .}}", query)
	out, err := cmd.Output()
	if err != nil {
		return []DockerHubSearchItem{}, fmt.Errorf("gagal mencari citra di Docker Hub: %w", err)
	}

	results := make([]DockerHubSearchItem, 0)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// CLI `docker search --format "{{json .}}"` mengeluarkan field PascalCase
		var raw struct {
			Name        string `json:"Name"`
			Description string `json:"Description"`
			StarCount   string `json:"StarCount"`
			IsOfficial  string `json:"IsOfficial"`
			IsAutomated string `json:"IsAutomated"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil || raw.Name == "" {
			continue
		}
		stars := 0
		fmt.Sscanf(raw.StarCount, "%d", &stars)
		results = append(results, DockerHubSearchItem{
			Name:        raw.Name,
			Description: raw.Description,
			StarCount:   stars,
			IsOfficial:  strings.Contains(strings.ToUpper(raw.IsOfficial), "OK") || strings.EqualFold(raw.IsOfficial, "true"),
			IsAutomated: strings.Contains(strings.ToUpper(raw.IsAutomated), "OK") || strings.EqualFold(raw.IsAutomated, "true"),
		})
	}
	return results, nil
}

// ListDockerHubTags mengambil daftar tag versi, ukuran, dan arsitektur (amd64/arm64) dari Docker Hub API v2
func (m *Manager) ListDockerHubTags(ctx context.Context, repo, filter string) ([]DockerHubTagItem, error) {
	cleanRepo := strings.TrimSpace(repo)
	if cleanRepo == "" || !IsValidDockerRef(cleanRepo) {
		return nil, fmt.Errorf("nama repositori tidak valid")
	}

	// Citra resmi tanpa namespace (mis. "nginx") berada di bawah namespace "library/" pada API Docker Hub v2
	namespace := "library"
	name := cleanRepo
	if strings.Contains(cleanRepo, "/") {
		parts := strings.SplitN(cleanRepo, "/", 2)
		namespace = parts[0]
		name = parts[1]
	}

	endpoint := fmt.Sprintf(
		"https://hub.docker.com/v2/repositories/%s/%s/tags/?page_size=30&ordering=last_updated",
		url.PathEscape(namespace),
		url.PathEscape(name),
	)
	if q := strings.TrimSpace(filter); q != "" {
		endpoint += "&name=" + url.QueryEscape(q)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "FikPanel-DockerStudio/0.7")

	resp, err := dockerHubHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi Docker Hub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []DockerHubTagItem{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Docker Hub mengembalikan HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Results []struct {
			Name        string `json:"name"`
			FullSize    int64  `json:"full_size"`
			LastUpdated string `json:"last_updated"`
			Digest      string `json:"digest"`
			Images      []struct {
				Architecture string `json:"architecture"`
				OS           string `json:"os"`
				Size         int64  `json:"size"`
			} `json:"images"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("gagal mengurai daftar tag Docker Hub: %w", err)
	}

	tags := make([]DockerHubTagItem, 0, len(payload.Results))
	for _, t := range payload.Results {
		archSet := make(map[string]struct{})
		var fallbackSize int64
		for _, img := range t.Images {
			a := strings.TrimSpace(img.Architecture)
			if a != "" && a != "unknown" {
				archSet[a] = struct{}{}
			}
			if img.Size > fallbackSize {
				fallbackSize = img.Size
			}
		}
		arches := make([]string, 0, len(archSet))
		for a := range archSet {
			arches = append(arches, a)
		}
		sort.Strings(arches)

		sz := t.FullSize
		if sz == 0 {
			sz = fallbackSize
		}
		tags = append(tags, DockerHubTagItem{
			Name:          t.Name,
			FullSize:      sz,
			SizeHuman:     FormatBytes(sz),
			LastUpdated:   t.LastUpdated,
			Digest:        t.Digest,
			Architectures: arches,
		})
	}
	return tags, nil
}

// StartPullImageTask memulai proses `docker pull` di latar belakang dan mengalirkan progres via SSE
func (m *Manager) StartPullImageTask(imageRef string) (string, error) {
	ref := strings.TrimSpace(imageRef)
	if !IsValidDockerRef(ref) {
		return "", fmt.Errorf("nama citra Docker tidak valid")
	}

	task := m.tasks.Create("pull", ref)
	task.Append(fmt.Sprintf("▶ Memulai penarikan citra: %s", ref))

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "docker", "pull", ref)
		err := runCommandStream(ctx, task, cmd)
		if err != nil {
			task.Append(fmt.Sprintf("✖ Penarikan citra gagal: %v", err))
			task.Finish(err)
			return
		}
		task.Append(fmt.Sprintf("✔ Citra %s berhasil ditarik dan siap digunakan", ref))
		task.Finish(nil)
	}()

	return task.id, nil
}

// StartBuildImageTask membangun citra lokal dari Dockerfile kustom secara asinkron dengan log live SSE
func (m *Manager) StartBuildImageTask(req BuildImageRequest) (string, error) {
	tag := strings.TrimSpace(req.Tag)
	if !IsValidDockerRef(tag) {
		return "", fmt.Errorf("nama tag citra tidak valid (mis. myapp:latest)")
	}
	if strings.TrimSpace(req.Dockerfile) == "" {
		return "", fmt.Errorf("isi Dockerfile tidak boleh kosong")
	}
	if len(req.Dockerfile) > 256*1024 {
		return "", fmt.Errorf("ukuran Dockerfile melebihi batas 256 KB")
	}

	task := m.tasks.Create("build", tag)
	task.Append(fmt.Sprintf("▶ Menyiapkan build context sementara untuk citra %s", tag))

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()

		tmpDir, err := os.MkdirTemp("", "fikpanel-docker-build-*")
		if err != nil {
			task.Append(fmt.Sprintf("✖ Gagal membuat direktori build sementara: %v", err))
			task.Finish(err)
			return
		}
		defer os.RemoveAll(tmpDir)

		dfPath := filepath.Join(tmpDir, "Dockerfile")
		if err := os.WriteFile(dfPath, []byte(req.Dockerfile), 0600); err != nil {
			task.Append(fmt.Sprintf("✖ Gagal menulis Dockerfile: %v", err))
			task.Finish(err)
			return
		}

		args := []string{"build", "--progress=plain", "-t", tag}
		if req.NoCache {
			args = append(args, "--no-cache")
		}
		if req.Pull {
			args = append(args, "--pull")
		}
		args = append(args, tmpDir)

		task.Append(fmt.Sprintf("$ docker %s", strings.Join(args, " ")))
		cmd := exec.CommandContext(ctx, "docker", args...)
		if err := runCommandStream(ctx, task, cmd); err != nil {
			task.Append(fmt.Sprintf("✖ Proses build Dockerfile gagal: %v", err))
			task.Finish(err)
			return
		}
		task.Append(fmt.Sprintf("✔ Citra %s berhasil dibangun!", tag))
		task.Finish(nil)
	}()

	return task.id, nil
}

// StartInstallDockerTask menjalankan instalasi Docker Engine & Compose Plugin asli di host Linux secara streaming
func (m *Manager) StartInstallDockerTask() (string, error) {
	task := m.tasks.Create("install", "docker-ce")
	task.Append("▶ Memulai instalasi resmi Docker Engine CE & Docker Compose Plugin...")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		// Gunakan skrip resmi get.docker.com untuk kompatibilitas Ubuntu/Debian/Armbian (amd64 & arm64)
		script := `set -e
echo "[1/4] Memeriksa curl dan prasyarat paket..."
if ! command -v curl >/dev/null 2>&1; then
  apt-get update -y && apt-get install -y curl ca-certificates gnupg
fi
echo "[2/4] Mengunduh dan menjalankan installer resmi Docker CE (get.docker.com)..."
curl -fsSL https://get.docker.com | sh
echo "[3/4] Mengaktifkan layanan systemd Docker..."
if command -v systemctl >/dev/null 2>&1; then
  systemctl enable --now docker
fi
echo "[4/4] Memverifikasi versi Docker & Compose..."
docker --version
docker compose version || true
echo "Instalasi Docker selesai!"`

		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script)
		if err := runCommandStream(ctx, task, cmd); err != nil {
			task.Append(fmt.Sprintf("✖ Instalasi Docker gagal: %v", err))
			task.Finish(err)
			return
		}
		task.Append("✔ Docker Engine & Compose siap digunakan!")
		task.Finish(nil)
	}()

	return task.id, nil
}

// GetImageHistory mengembalikan daftar layer dan riwayat instruksi dari citra lokal
func (m *Manager) GetImageHistory(ctx context.Context, id string) ([]DockerImageHistoryItem, error) {
	ref := strings.TrimSpace(id)
	if !IsValidDockerRef(ref) {
		return nil, fmt.Errorf("referensi citra tidak valid")
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, "docker", "image", "history", "--no-trunc", "--format", "{{json .}}", ref)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gagal membaca riwayat layer citra: %s", strings.TrimSpace(string(out)))
	}

	items := make([]DockerImageHistoryItem, 0)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw struct {
			ID           string `json:"ID"`
			CreatedSince string `json:"CreatedSince"`
			CreatedBy    string `json:"CreatedBy"`
			Size         string `json:"Size"`
			Comment      string `json:"Comment"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		sz := ParseHumanSize(raw.Size)
		shortID := raw.ID
		if strings.HasPrefix(shortID, "sha256:") && len(shortID) > 19 {
			shortID = shortID[7:19]
		}
		items = append(items, DockerImageHistoryItem{
			ID:           shortID,
			CreatedSince: raw.CreatedSince,
			CreatedBy:    strings.TrimSpace(raw.CreatedBy),
			SizeBytes:    sz,
			SizeHuman:    FormatBytes(sz),
			Comment:      raw.Comment,
		})
	}
	return items, nil
}

// TagImage menandai ulang (retag) citra lokal menjadi nama repositori:tag baru
func (m *Manager) TagImage(ctx context.Context, source, target string) error {
	src := strings.TrimSpace(source)
	tgt := strings.TrimSpace(target)
	if !IsValidDockerRef(src) || !IsValidDockerRef(tgt) {
		return fmt.Errorf("referensi citra sumber atau target tidak valid")
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "docker", "tag", src, tgt).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal melakukan retag citra: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// PruneImages membersihkan citra dangling (atau seluruh citra tidak terpakai jika all=true)
func (m *Manager) PruneImages(ctx context.Context, all bool) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	args := []string{"image", "prune", "-f"}
	if all {
		args = append(args, "-a")
	}
	out, err := exec.CommandContext(cctx, "docker", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gagal membersihkan citra: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// ExportImageTar mengalirkan arsip tar citra dari `docker save` langsung ke http.ResponseWriter
func (m *Manager) ExportImageTar(ctx context.Context, ref string, w io.Writer) error {
	clean := strings.TrimSpace(ref)
	if !IsValidDockerRef(clean) {
		return fmt.Errorf("referensi citra tidak valid")
	}
	cmd := exec.CommandContext(ctx, "docker", "save", clean)
	cmd.Stdout = w
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gagal mengekspor citra tar: %s", strings.TrimSpace(errBuf.String()))
	}
	return nil
}

// LoadImageTar mengimpor berkas arsip tar citra Docker menggunakan `docker load`
func (m *Manager) LoadImageTar(ctx context.Context, r io.Reader) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cctx, "docker", "load")
	cmd.Stdin = r
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gagal mengimpor berkas tar citra: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
