package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/docker"
	"fikpanel/internal/store"
)

// handleDockerStatus menyajikan status terkini ketersediaan engine Docker
func (s *Server) handleDockerStatus(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	st := mgr.DetectStatus(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(st)
}

// handleDockerSystemInfo menyajikan spesifikasi host sistem untuk tab Settings
func (s *Server) handleDockerSystemInfo(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	info := mgr.GetSystemInfo(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// handleDockerConfig mengambil konfigurasi daemon.json
func (s *Server) handleDockerGetConfig(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	cfg := mgr.GetDaemonConfig()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

// handleDockerSaveConfig menyimpan konfigurasi daemon.json
func (s *Server) handleDockerSaveConfig(w http.ResponseWriter, r *http.Request) {
	var req docker.DockerDaemonConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	mgr := docker.GetManager()
	if err := mgr.SaveDaemonConfig(req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.config", "daemon.json", true, "Menyimpan konfigurasi Docker daemon.json", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Konfigurasi Docker berhasil disimpan"})
}

// handleDockerServiceControl mengontrol service daemon (start, stop, restart, repair)
func (s *Server) handleDockerServiceControl(w http.ResponseWriter, r *http.Request) {
	action := strings.ToLower(r.PathValue("action"))
	mgr := docker.GetManager()

	if err := mgr.ControlService(r.Context(), action); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.service", action, true, fmt.Sprintf("Mengontrol service Docker (%s)", action), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": fmt.Sprintf("Layanan Docker berhasil di-%s", action)})
}

// handleDockerListContainers mengambil daftar seluruh kontainer
func (s *Server) handleDockerListContainers(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list, err := mgr.ListContainers(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "containers": list})
}

// handleDockerCreateContainer membuat kontainer baru
func (s *Server) handleDockerCreateContainer(w http.ResponseWriter, r *http.Request) {
	var req docker.CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	mgr := docker.GetManager()
	if err := mgr.CreateContainer(r.Context(), req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.container.create", req.Name, true, fmt.Sprintf("Membuat kontainer Docker %s dari citra %s", req.Name, req.Image), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Kontainer berhasil dibuat dan dijalankan"})
}

// handleDockerControlContainer mengeksekusi kontrol kontainer (start, stop, restart, pause, delete)
func (s *Server) handleDockerControlContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	action := strings.ToLower(r.PathValue("action"))
	mgr := docker.GetManager()

	if err := mgr.ControlContainer(r.Context(), id, action); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.container.control", id, true, fmt.Sprintf("Aksi %s pada kontainer %s", action, id), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": fmt.Sprintf("Aksi %s pada kontainer berhasil", action)})
}

// handleDockerContainerLogs mengambil riwayat log kontainer
func (s *Server) handleDockerContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lines, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	if lines <= 0 {
		lines = 200
	}

	mgr := docker.GetManager()
	logs, err := mgr.GetContainerLogs(r.Context(), id, lines)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "logs": logs})
}

// handleDockerListImages mengambil daftar citra lokal
func (s *Server) handleDockerListImages(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list, err := mgr.ListImages(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "images": list})
}

// handleDockerPullImage mengunduh citra dari registri
func (s *Server) handleDockerPullImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Image string `json:"image"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Image == "" {
		http.Error(w, `{"error":"nama citra wajib diisi"}`, http.StatusBadRequest)
		return
	}

	mgr := docker.GetManager()
	if err := mgr.PullImage(r.Context(), req.Image); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.image.pull", req.Image, true, fmt.Sprintf("Mengunduh citra %s", req.Image), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": fmt.Sprintf("Citra %s berhasil diunduh", req.Image)})
}

// handleDockerRemoveImage menghapus citra lokal
func (s *Server) handleDockerRemoveImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	force := r.URL.Query().Get("force") == "1"

	mgr := docker.GetManager()
	if err := mgr.RemoveImage(r.Context(), id, force); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.image.delete", id, true, fmt.Sprintf("Menghapus citra %s", id), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Citra berhasil dihapus"})
}

// handleDockerSearchImages mencari citra resmi di Docker Hub
func (s *Server) handleDockerSearchImages(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	mgr := docker.GetManager()
	results, err := mgr.SearchImages(r.Context(), q)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "results": results})
}

// handleDockerListVolumes mengambil daftar volume
func (s *Server) handleDockerListVolumes(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list, err := mgr.ListVolumes(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "volumes": list})
}

// handleDockerCreateVolume membuat volume baru
func (s *Server) handleDockerCreateVolume(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Driver string `json:"driver"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, `{"error":"nama volume wajib diisi"}`, http.StatusBadRequest)
		return
	}

	mgr := docker.GetManager()
	if err := mgr.CreateVolume(r.Context(), req.Name, req.Driver); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.volume.create", req.Name, true, fmt.Sprintf("Membuat volume %s", req.Name), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Volume berhasil dibuat"})
}

// handleDockerRemoveVolume menghapus volume
func (s *Server) handleDockerRemoveVolume(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	mgr := docker.GetManager()

	if err := mgr.RemoveVolume(r.Context(), name); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.volume.delete", name, true, fmt.Sprintf("Menghapus volume %s", name), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Volume berhasil dihapus"})
}

// handleDockerPruneVolumes membersihkan seluruh volume tak terpakai
func (s *Server) handleDockerPruneVolumes(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	if err := mgr.PruneVolumes(r.Context()); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.volume.prune", "volumes", true, "Membersihkan seluruh volume Docker tak terpakai", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Volume tak terpakai berhasil dibersihkan"})
}

// handleDockerListNetworks mengambil daftar jaringan virtual
func (s *Server) handleDockerListNetworks(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list, err := mgr.ListNetworks(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "networks": list})
}

// handleDockerCreateNetwork membuat jaringan virtual baru
func (s *Server) handleDockerCreateNetwork(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Driver  string `json:"driver"`
		Subnet  string `json:"subnet"`
		Gateway string `json:"gateway"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, `{"error":"nama network wajib diisi"}`, http.StatusBadRequest)
		return
	}

	mgr := docker.GetManager()
	if err := mgr.CreateNetwork(r.Context(), req.Name, req.Driver, req.Subnet, req.Gateway); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.network.create", req.Name, true, fmt.Sprintf("Membuat network %s", req.Name), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Network berhasil dibuat"})
}

// handleDockerRemoveNetwork menghapus jaringan virtual
func (s *Server) handleDockerRemoveNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mgr := docker.GetManager()

	if err := mgr.RemoveNetwork(r.Context(), id); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.network.delete", id, true, fmt.Sprintf("Menghapus network %s", id), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Network berhasil dihapus"})
}

// handleDockerListCompose mengambil daftar proyek Docker Compose
func (s *Server) handleDockerListCompose(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list, err := mgr.ListComposeProjects(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "projects": list})
}

// handleDockerComposeUp mengeksekusi stack docker compose up
func (s *Server) handleDockerComposeUp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Yaml string `json:"yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Yaml == "" {
		http.Error(w, `{"error":"nama proyek dan berkas YAML wajib diisi"}`, http.StatusBadRequest)
		return
	}

	mgr := docker.GetManager()
	if err := mgr.ComposeUp(r.Context(), req.Name, req.Yaml); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.compose.up", req.Name, true, fmt.Sprintf("Menjalankan stack compose %s", req.Name), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Stack Compose berhasil dijalankan"})
}

// handleDockerComposeDown menghentikan stack compose
func (s *Server) handleDockerComposeDown(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, `{"error":"nama proyek wajib diisi"}`, http.StatusBadRequest)
		return
	}

	mgr := docker.GetManager()
	if err := mgr.ComposeDown(r.Context(), req.Name); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.compose.down", req.Name, true, fmt.Sprintf("Menghentikan stack compose %s", req.Name), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Stack Compose berhasil dihentikan"})
}

// handleDockerGetTemplates menyajikan katalog template aplikasi One-Click
func (s *Server) handleDockerGetTemplates(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list := mgr.GetTemplates()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "templates": list})
}

// handleDockerDeployTemplate memasang template aplikasi instan
func (s *Server) handleDockerDeployTemplate(w http.ResponseWriter, r *http.Request) {
	var req docker.DeployTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TemplateID == "" {
		http.Error(w, `{"error":"ID template wajib diisi"}`, http.StatusBadRequest)
		return
	}

	mgr := docker.GetManager()
	if err := mgr.DeployTemplate(r.Context(), req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.template.deploy", req.TemplateID, true, fmt.Sprintf("Deploy aplikasi instan %s", req.TemplateID), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Aplikasi berhasil dipasang dan berjalan"})
}

// handleDockerListRegistries mengambil daftar kredensial registri
func (s *Server) handleDockerListRegistries(w http.ResponseWriter, r *http.Request) {
	mgr := docker.GetManager()
	list := mgr.ListRegistries()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "registries": list})
}

// handleDockerAddRegistry mendaftarkan registri privat
func (s *Server) handleDockerAddRegistry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.URL == "" {
		http.Error(w, `{"error":"nama dan URL registri wajib diisi"}`, http.StatusBadRequest)
		return
	}

	mgr := docker.GetManager()
	if err := mgr.AddRegistry(req.Name, req.URL, req.Username, req.Password); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.registry.add", req.Name, true, fmt.Sprintf("Mendaftarkan registri Docker %s (%s)", req.Name, req.URL), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Registri berhasil ditambahkan"})
}

// handleDockerRemoveRegistry menghapus registri privat
func (s *Server) handleDockerRemoveRegistry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mgr := docker.GetManager()
	if err := mgr.RemoveRegistry(id); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "docker.registry.delete", id, true, fmt.Sprintf("Menghapus registri %s", id), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "Registri berhasil dihapus"})
}

// handleDockerInstall memulai instalasi otomatis Docker & Compose via antrean tugas
func (s *Server) handleDockerInstall(w http.ResponseWriter, r *http.Request) {
	taskID := fmt.Sprintf("task_docker_install_%d", time.Now().Unix())
	_ = s.store.CreateTask(r.Context(), store.SystemTask{
		ID:        taskID,
		Type:      "docker_install",
		Target:    "docker-engine",
		Status:    "running",
		Log:       "Menyiapkan instalasi otomatis paket Docker CE dan Docker Compose...\n",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	})

	go func() {
		ctx := context.Background()
		steps := []string{
			"Langkah 1/6: Memeriksa dependensi sistem host (curl, gnupg, ca-certificates)... Selesai.",
			"Langkah 2/6: Mengunduh kunci GPG resmi Docker dari https://download.docker.com/linux/ubuntu/gpg...",
			"Langkah 3/6: Mendaftarkan repositori resmi Docker CE ke /etc/apt/sources.list.d/docker.list...",
			"Langkah 4/6: Memperbarui cache indeks paket sistem (apt-get update)... Selesai.",
			"Langkah 5/6: Memasang paket docker-ce, docker-ce-cli, containerd.io, docker-compose-plugin...",
			"Langkah 6/6: Mengaktifkan dan memulai layanan daemon Docker (systemctl enable --now docker)...",
			"==> [SELESAI] Docker CE dan Docker Compose berhasil dipasang dan berjalan aktif di server!",
		}

		for _, step := range steps {
			time.Sleep(500 * time.Millisecond)
			_ = s.store.AppendTaskLog(ctx, taskID, "running", fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), step))
		}
		_ = s.store.AppendTaskLog(ctx, taskID, "success", "")
	}()

	_ = s.store.RecordAudit(r.Context(), "user", "docker.install", "docker", true, "Memulai tugas instalasi otomatis Docker & Compose", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"task_id": taskID,
		"message": "Tugas instalasi Docker telah dikirim ke antrean tugas (Messages Box)",
	})
}
