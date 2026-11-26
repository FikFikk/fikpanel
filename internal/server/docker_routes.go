package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"fikpanel/internal/docker"
)

const dockerRegistriesSettingKey = "docker.registries"

// initDockerPersistence memuat daftar registri Docker tersimpan dari SQLite dan menghubungkan callback simpannya
func (s *Server) initDockerPersistence() {
	mgr := docker.GetManager()
	var saved []docker.DockerRegistry
	if raw, err := s.store.GetSetting(context.Background(), dockerRegistriesSettingKey); err == nil && strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &saved)
	}
	mgr.SetRegistryPersistence(saved, func(regs []docker.DockerRegistry) error {
		b, err := json.Marshal(regs)
		if err != nil {
			return err
		}
		return s.store.SetSetting(context.Background(), dockerRegistriesSettingKey, string(b))
	})
}

func writeDockerJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// handleDockerStatus menyajikan status terkini ketersediaan engine Docker & pemakaian disk
func (s *Server) handleDockerStatus(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	st := mgr.DetectStatus(r.Context())
	writeDockerJSON(w, http.StatusOK, st)
}

// handleDockerDiskUsage menyajikan rincian pemakaian ruang disk (`docker system df`)
func (s *Server) handleDockerDiskUsage(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	df, err := mgr.GetDiskUsage(r.Context())
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, df)
}

// handleDockerSystemPrune membersihkan ruang disk Docker secara selektif
func (s *Server) handleDockerSystemPrune(w http.ResponseWriter, r *http.Request) {
	var req docker.SystemPruneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "payload JSON tidak valid"})
		return
	}

	mgr := docker.GetManager()
	res, err := mgr.SystemPrune(r.Context(), req)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.system.prune", "system", true, fmt.Sprintf("Pembersihan disk Docker selesai (bebas %s)", res.ReclaimedHuman), nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"result":  res,
		"message": fmt.Sprintf("Ruang disk dibebaskan: %s", res.ReclaimedHuman),
	})
}

// handleDockerSystemInfo menyajikan spesifikasi perangkat keras dan versi Docker host
func (s *Server) handleDockerSystemInfo(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	info := mgr.GetSystemInfo(r.Context())
	writeDockerJSON(w, http.StatusOK, info)
}

// handleDockerGetConfig mengambil konfigurasi /etc/docker/daemon.json
func (s *Server) handleDockerGetConfig(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	cfg := mgr.GetDaemonConfig()
	writeDockerJSON(w, http.StatusOK, cfg)
}

// handleDockerSaveConfig menyimpan konfigurasi /etc/docker/daemon.json
func (s *Server) handleDockerSaveConfig(w http.ResponseWriter, r *http.Request) {
	var req docker.DockerDaemonConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "payload JSON tidak valid"})
		return
	}

	mgr := docker.GetManager()
	if err := mgr.SaveDaemonConfig(req); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.config", "daemon.json", true, "Menyimpan konfigurasi Docker daemon.json", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Konfigurasi Docker berhasil disimpan"})
}

// handleDockerServiceControl mengontrol layanan sistem Docker (start, stop, restart, repair)
func (s *Server) handleDockerServiceControl(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	mgr := docker.GetManager()
	if err := mgr.ControlService(r.Context(), action); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.service."+action, "docker.service", true, fmt.Sprintf("Aksi %s pada layanan Docker berhasil", action), nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": fmt.Sprintf("Layanan Docker berhasil di-%s", action)})
}

// handleDockerInstall menjalankan instalasi resmi Docker CE & Compose secara streaming SSE
func (s *Server) handleDockerInstall(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	taskID, err := mgr.StartInstallDockerTask()
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.install", "docker-ce", true, "Memulai instalasi resmi Docker Engine & Compose", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"task_id": taskID,
		"message": "Instalasi Docker Engine resmi dimulai secara real-time",
	})
}

// handleDockerStreamTask mengalirkan log tugas Docker jangka panjang (pull, build, install) melalui SSE
func (s *Server) handleDockerStreamTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "ID tugas wajib diisi", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming SSE tidak didukung", http.StatusInternalServerError)
		return
	}

	mgr := docker.GetManager()
	snap, ch, cleanup, found := mgr.Tasks().Subscribe(id)
	if !found {
		http.Error(w, "Tugas streaming Docker tidak ditemukan", http.StatusNotFound)
		return
	}
	defer cleanup()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Kirim seluruh log yang telah tercatat sebelum koneksi SSE terhubung
	for _, line := range snap.Logs {
		enc, _ := json.Marshal(line)
		fmt.Fprintf(w, "data: %s\n\n", enc)
	}
	flusher.Flush()

	if snap.Status != "running" || ch == nil {
		donePayload, _ := json.Marshal(map[string]string{"status": snap.Status, "error": snap.Error})
		fmt.Fprintf(w, "event: done\ndata: %s\n\n", donePayload)
		flusher.Flush()
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case line, open := <-ch:
			if !open {
				finalSnap, _ := mgr.Tasks().GetSnapshot(id)
				donePayload, _ := json.Marshal(map[string]string{"status": finalSnap.Status, "error": finalSnap.Error})
				fmt.Fprintf(w, "event: done\ndata: %s\n\n", donePayload)
				flusher.Flush()
				return
			}
			enc, _ := json.Marshal(line)
			fmt.Fprintf(w, "data: %s\n\n", enc)
			flusher.Flush()
		}
	}
}

// ── KONTAINER ────────────────────────────────────────────────────────────────

func (s *Server) handleDockerListContainers(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list, err := mgr.ListContainers(r.Context())
	if err != nil {
		writeDockerJSON(w, http.StatusOK, map[string]interface{}{"containers": []docker.DockerContainer{}, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"containers": list})
}

func (s *Server) handleDockerCreateContainer(w http.ResponseWriter, r *http.Request) {
	var req docker.CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "payload JSON tidak valid"})
		return
	}

	mgr := docker.GetManager()
	id, err := mgr.CreateContainer(r.Context(), req)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.container.create", req.Name, true, fmt.Sprintf("Membuat kontainer dari citra %s", req.Image), nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "id": id, "message": "Kontainer berhasil dibuat & dijalankan"})
}

func (s *Server) handleDockerControlContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	action := r.PathValue("action")

	mgr := docker.GetManager()
	if err := mgr.ControlContainer(r.Context(), id, action); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.container."+action, id, true, fmt.Sprintf("Aksi %s pada kontainer %s berhasil", action, id), nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": fmt.Sprintf("Kontainer berhasil di-%s", action)})
}

func (s *Server) handleDockerBulkContainers(w http.ResponseWriter, r *http.Request) {
	var req docker.BulkContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "pilih minimal satu kontainer"})
		return
	}
	mgr := docker.GetManager()
	res := mgr.BulkControlContainers(r.Context(), req)
	_ = s.store.RecordAudit(r.Context(), "user", "docker.container.bulk_"+req.Action, fmt.Sprintf("%d containers", len(req.IDs)), len(res.Failed) == 0, fmt.Sprintf("Aksi massal %s: %d berhasil, %d gagal", req.Action, len(res.Succeeded), len(res.Failed)), nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "result": res})
}

func (s *Server) handleDockerInspectContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mgr := docker.GetManager()
	insp, err := mgr.InspectContainer(r.Context(), id)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, insp)
}

func (s *Server) handleDockerRecreateContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		PullLatest bool `json:"pull_latest"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	mgr := docker.GetManager()
	newID, err := mgr.RecreateContainer(r.Context(), id, req.PullLatest)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.container.recreate", id, true, fmt.Sprintf("Kontainer dibuat ulang (pull_latest=%v)", req.PullLatest), nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "id": newID, "message": "Kontainer berhasil dibuat ulang"})
}

func (s *Server) handleDockerContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lines := 200
	if lStr := r.URL.Query().Get("lines"); lStr != "" {
		if n, err := strconv.Atoi(lStr); err == nil && n > 0 {
			lines = n
		}
	}
	timestamps := r.URL.Query().Get("timestamps") == "1" || r.URL.Query().Get("timestamps") == "true"

	mgr := docker.GetManager()
	logs, err := mgr.GetContainerLogsWithOptions(r.Context(), id, lines, timestamps)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"logs": logs})
}

// ── CITRA (IMAGES), DOCKER HUB & BUILDER ────────────────────────────────────

func (s *Server) handleDockerListImages(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list, err := mgr.ListImages(r.Context())
	if err != nil {
		writeDockerJSON(w, http.StatusOK, map[string]interface{}{"images": []docker.DockerImage{}, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"images": list})
}

func (s *Server) handleDockerPullImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Image string `json:"image"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Image) == "" {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "nama citra wajib diisi"})
		return
	}

	mgr := docker.GetManager()
	if err := mgr.PullImage(r.Context(), req.Image); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.image.pull", req.Image, true, "Menarik citra Docker dari registry", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Citra berhasil ditarik"})
}

func (s *Server) handleDockerPullImageStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Image string `json:"image"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Image) == "" {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "nama citra wajib diisi"})
		return
	}

	mgr := docker.GetManager()
	taskID, err := mgr.StartPullImageTask(req.Image)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.image.pull", req.Image, true, "Memulai pull citra streaming", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "task_id": taskID})
}

func (s *Server) handleDockerBuildImage(w http.ResponseWriter, r *http.Request) {
	var req docker.BuildImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "payload JSON tidak valid"})
		return
	}

	mgr := docker.GetManager()
	taskID, err := mgr.StartBuildImageTask(req)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.image.build", req.Tag, true, "Memulai build citra dari Dockerfile", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "task_id": taskID})
}

func (s *Server) handleDockerRemoveImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	force := r.URL.Query().Get("force") == "true" || r.URL.Query().Get("force") == "1"

	mgr := docker.GetManager()
	if err := mgr.RemoveImage(r.Context(), id, force); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.image.remove", id, true, "Menghapus citra Docker lokal", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Citra lokal berhasil dihapus"})
}

func (s *Server) handleDockerImageHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mgr := docker.GetManager()
	items, err := mgr.GetImageHistory(r.Context(), id)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"history": items})
}

func (s *Server) handleDockerTagImage(w http.ResponseWriter, r *http.Request) {
	var req docker.TagImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "payload JSON tidak valid"})
		return
	}
	mgr := docker.GetManager()
	if err := mgr.TagImage(r.Context(), req.Source, req.Target); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.image.tag", req.Target, true, fmt.Sprintf("Retag citra %s -> %s", req.Source, req.Target), nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Tag citra berhasil dibuat"})
}

func (s *Server) handleDockerPruneImages(w http.ResponseWriter, r *http.Request) {
	var req struct {
		All bool `json:"all"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	mgr := docker.GetManager()
	out, err := mgr.PruneImages(r.Context(), req.All)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.image.prune", "images", true, "Membersihkan citra Docker usang", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "output": out, "message": "Citra usang berhasil dibersihkan"})
}

func (s *Server) handleDockerExportImage(w http.ResponseWriter, r *http.Request) {
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		ref = strings.TrimSpace(r.PathValue("id"))
	}
	if !docker.IsValidDockerRef(ref) {
		http.Error(w, "Referensi citra tidak valid", http.StatusBadRequest)
		return
	}

	safeName := strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(ref)
	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.tar"`, filepath.Base(safeName)))

	mgr := docker.GetManager()
	if err := mgr.ExportImageTar(r.Context(), ref, w); err != nil {
		// Header sudah terkirim jika streaming sudah berjalan, hentikan saja
		return
	}
}

func (s *Server) handleDockerLoadImage(w http.ResponseWriter, r *http.Request) {
	// Batasi ukuran unggahan arsip tar citra maksimal 2 GB
	r.Body = http.MaxBytesReader(w, r.Body, 2<<30)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "gagal membaca berkas unggahan tar"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "berkas .tar wajib disertakan"})
		return
	}
	defer file.Close()

	mgr := docker.GetManager()
	out, err := mgr.LoadImageTar(r.Context(), io.LimitReader(file, 2<<30))
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.image.load", header.Filename, true, "Mengimpor arsip tar citra Docker", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "output": out, "message": "Citra dari arsip tar berhasil diimpor"})
}

func (s *Server) handleDockerSearchImages(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	mgr := docker.GetManager()
	results, err := mgr.SearchDockerHub(r.Context(), q, page)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"results": []docker.DockerHubSearchItem{}, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}

func (s *Server) handleDockerHubTags(w http.ResponseWriter, r *http.Request) {
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	filter := strings.TrimSpace(r.URL.Query().Get("q"))
	if repo == "" {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"tags": []docker.DockerHubTagItem{}, "error": "parameter repo wajib diisi"})
		return
	}

	mgr := docker.GetManager()
	tags, err := mgr.ListDockerHubTags(r.Context(), repo, filter)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"tags": []docker.DockerHubTagItem{}, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"repo": repo, "tags": tags})
}

// ── VOLUME ───────────────────────────────────────────────────────────────────

func (s *Server) handleDockerListVolumes(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list, err := mgr.ListVolumes(r.Context())
	if err != nil {
		writeDockerJSON(w, http.StatusOK, map[string]interface{}{"volumes": []docker.DockerVolume{}, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"volumes": list})
}

func (s *Server) handleDockerCreateVolume(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Driver string `json:"driver"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "nama volume wajib diisi"})
		return
	}

	mgr := docker.GetManager()
	if err := mgr.CreateVolume(r.Context(), req.Name, req.Driver); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.volume.create", req.Name, true, "Membuat volume Docker baru", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Volume berhasil dibuat"})
}

func (s *Server) handleDockerRemoveVolume(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	mgr := docker.GetManager()
	if err := mgr.RemoveVolume(r.Context(), name); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.volume.remove", name, true, "Menghapus volume Docker", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Volume berhasil dihapus"})
}

func (s *Server) handleDockerPruneVolumes(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	out, err := mgr.PruneVolumes(r.Context())
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.volume.prune", "volumes", true, "Membersihkan volume tak terpakai", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "output": out, "message": "Volume usang berhasil dibersihkan"})
}

// ── JARINGAN (NETWORKS) ─────────────────────────────────────────────────────

func (s *Server) handleDockerListNetworks(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list, err := mgr.ListNetworks(r.Context())
	if err != nil {
		writeDockerJSON(w, http.StatusOK, map[string]interface{}{"networks": []docker.DockerNetwork{}, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"networks": list})
}

func (s *Server) handleDockerCreateNetwork(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Driver  string `json:"driver"`
		Subnet  string `json:"subnet"`
		Gateway string `json:"gateway"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "nama jaringan wajib diisi"})
		return
	}

	mgr := docker.GetManager()
	if err := mgr.CreateNetwork(r.Context(), req.Name, req.Driver, req.Subnet, req.Gateway); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.network.create", req.Name, true, "Membuat jaringan virtual Docker", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Jaringan berhasil dibuat"})
}

func (s *Server) handleDockerRemoveNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mgr := docker.GetManager()
	if err := mgr.RemoveNetwork(r.Context(), id); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.network.remove", id, true, "Menghapus jaringan Docker", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Jaringan berhasil dihapus"})
}

func (s *Server) handleDockerPruneNetworks(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	out, err := mgr.PruneNetworks(r.Context())
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.network.prune", "networks", true, "Membersihkan jaringan Docker tak terpakai", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "output": out, "message": "Jaringan tak terpakai berhasil dibersihkan"})
}

// ── DOCKER COMPOSE ──────────────────────────────────────────────────────────

func (s *Server) handleDockerListCompose(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list, err := mgr.ListComposeProjects(r.Context())
	if err != nil {
		writeDockerJSON(w, http.StatusOK, map[string]interface{}{"projects": []docker.DockerComposeProject{}, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"projects": list})
}

func (s *Server) handleDockerComposeUp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		YAMLContent string `json:"yaml_content"`
		YAML        string `json:"yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "nama proyek dan konfigurasi YAML wajib diisi"})
		return
	}
	yamlBody := req.YAMLContent
	if strings.TrimSpace(yamlBody) == "" {
		yamlBody = req.YAML
	}

	mgr := docker.GetManager()
	if err := mgr.ComposeUp(r.Context(), req.Name, yamlBody); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.compose.up", req.Name, true, "Menjalankan stack Docker Compose", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Stack Compose berhasil dijalankan"})
}

func (s *Server) handleDockerComposeDown(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		ConfigPath string `json:"config_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "nama proyek wajib diisi"})
		return
	}

	mgr := docker.GetManager()
	if err := mgr.ComposeAction(r.Context(), docker.ComposeActionRequest{
		Name:       req.Name,
		ConfigPath: req.ConfigPath,
		Action:     "down",
	}); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.compose.down", req.Name, true, "Menghentikan stack Docker Compose", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Stack Compose berhasil dihentikan"})
}

func (s *Server) handleDockerComposeAction(w http.ResponseWriter, r *http.Request) {
	var req docker.ComposeActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "nama proyek dan aksi wajib diisi"})
		return
	}

	mgr := docker.GetManager()
	if err := mgr.ComposeAction(r.Context(), req); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.compose."+req.Action, req.Name, true, fmt.Sprintf("Aksi Compose %s berhasil", req.Action), nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": fmt.Sprintf("Stack %s berhasil di-%s", req.Name, req.Action)})
}

func (s *Server) handleDockerComposeLogs(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	configPath := strings.TrimSpace(r.URL.Query().Get("config_path"))
	lines, _ := strconv.Atoi(r.URL.Query().Get("lines"))

	mgr := docker.GetManager()
	logs, err := mgr.ComposeLogs(r.Context(), name, configPath, lines)
	if err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"logs": logs})
}

// ── ONE-CLICK TEMPLATES & REGISTRIES ────────────────────────────────────────

func (s *Server) handleDockerGetTemplates(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"templates": mgr.GetTemplates()})
}

func (s *Server) handleDockerDeployTemplate(w http.ResponseWriter, r *http.Request) {
	var req docker.DeployTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TemplateID) == "" {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "ID template wajib diisi"})
		return
	}

	mgr := docker.GetManager()
	if err := mgr.DeployTemplate(r.Context(), req); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	targetName := req.ProjectName
	if targetName == "" {
		targetName = req.Name
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.template.deploy", req.TemplateID, true, fmt.Sprintf("Memasang aplikasi 1-Klik (%s)", targetName), nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Aplikasi 1-Klik berhasil dipasang dan dijalankan"})
}

func (s *Server) handleDockerListRegistries(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"registries": mgr.ListRegistries()})
}

func (s *Server) handleDockerAddRegistry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.URL) == "" {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "nama dan URL registri wajib diisi"})
		return
	}

	mgr := docker.GetManager()
	if err := mgr.AddRegistry(r.Context(), req.Name, req.URL, req.Username, req.Password); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.registry.add", req.URL, true, fmt.Sprintf("Menambahkan & login ke registri %s", req.Name), nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Registri berhasil ditambahkan"})
}

func (s *Server) handleDockerRemoveRegistry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mgr := docker.GetManager()
	if err := mgr.RemoveRegistry(r.Context(), id); err != nil {
		writeDockerJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "docker.registry.remove", id, true, "Menghapus registri Docker", nil)
	writeDockerJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Registri berhasil dihapus"})
}
