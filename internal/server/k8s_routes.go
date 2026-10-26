package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"fikpanel/internal/k8s"
)

// ensureK8sClient memastikan instansi K8s client aktif
func (s *Server) ensureK8sClient() *k8s.Client {
	if s.k8sClient == nil {
		s.k8sClient = k8s.NewClient()
	}
	return s.k8sClient
}

// handleK8sStatus menyajikan status konektivitas dan informasi cluster Kubernetes
func (s *Server) handleK8sStatus(w http.ResponseWriter, r *http.Request) {
	client := s.ensureK8sClient()
	status, err := client.GetStatus(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal mengecek status cluster: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"status": status,
	})
}

// handleK8sNodes mengambil daftar node worker dan control plane
func (s *Server) handleK8sNodes(w http.ResponseWriter, r *http.Request) {
	client := s.ensureK8sClient()
	nodes, err := client.ListNodes(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membaca node cluster: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"nodes": nodes,
	})
}

// handleK8sNamespaces mengambil daftar namespace isolasi
func (s *Server) handleK8sNamespaces(w http.ResponseWriter, r *http.Request) {
	client := s.ensureK8sClient()
	nsList, err := client.ListNamespaces(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membaca daftar namespace: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":         true,
		"namespaces": nsList,
	})
}

// handleK8sPods mengambil daftar Pod kontainer
func (s *Server) handleK8sPods(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	client := s.ensureK8sClient()

	pods, err := client.ListPods(r.Context(), namespace)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membaca daftar pods: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"pods": pods,
	})
}

// handleK8sDeployments mengambil daftar workload deployment
func (s *Server) handleK8sDeployments(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	client := s.ensureK8sClient()

	deployments, err := client.ListDeployments(r.Context(), namespace)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membaca daftar deployment: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":          true,
		"deployments": deployments,
	})
}

// handleK8sServices mengambil daftar service jaringan
func (s *Server) handleK8sServices(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	client := s.ensureK8sClient()

	services, err := client.ListServices(r.Context(), namespace)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membaca daftar service: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"services": services,
	})
}

// handleK8sScaleDeployment mengatur jumlah replika pod dari sebuah deployment
func (s *Server) handleK8sScaleDeployment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, `{"error":"nama deployment wajib diisi"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Namespace string `json:"namespace"`
		Replicas  int    `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if req.Namespace == "" {
		req.Namespace = "default"
	}
	if req.Replicas < 0 {
		http.Error(w, `{"error":"jumlah replika tidak boleh negatif"}`, http.StatusBadRequest)
		return
	}

	client := s.ensureK8sClient()
	if err := client.ScaleDeployment(r.Context(), req.Namespace, name, req.Replicas); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal mengubah skala deployment: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "k8s.scale", fmt.Sprintf("%s/%s", req.Namespace, name), true,
		fmt.Sprintf("Mengubah skala deployment %s menjadi %d replika", name, req.Replicas), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Deployment %s berhasil di-scale menjadi %d replika", name, req.Replicas),
	})
}

// handleK8sApplyManifest menerapkan berkas konfigurasi YAML Kubernetes ke cluster
func (s *Server) handleK8sApplyManifest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Manifest  string `json:"manifest"`
		Namespace string `json:"namespace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Manifest) == "" {
		http.Error(w, `{"error":"isi manifest YAML tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	client := s.ensureK8sClient()
	out, err := client.ApplyManifest(r.Context(), req.Manifest, req.Namespace)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
			"output": out,
		})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "k8s.apply", req.Namespace, true, "Menerapkan manifest YAML kustom ke cluster Kubernetes", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Manifest berhasil diterapkan ke cluster",
		"output":  out,
	})
}

// handleK8sPodLogs mengambil log output kontainer di dalam Pod
func (s *Server) handleK8sPodLogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, `{"error":"nama pod wajib diisi"}`, http.StatusBadRequest)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}
	container := r.URL.Query().Get("container")
	tailStr := r.URL.Query().Get("tail")
	tail := 100
	if tailStr != "" {
		if t, err := strconv.Atoi(tailStr); err == nil && t > 0 {
			tail = t
		}
	}

	client := s.ensureK8sClient()
	logs, err := client.GetPodLogs(r.Context(), namespace, name, container, tail)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membaca log pod: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"logs": logs,
	})
}

// handleK8sDeleteResource menghapus resource Kubernetes (pod, deployment, service)
func (s *Server) handleK8sDeleteResource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if req.Kind == "" || req.Name == "" {
		http.Error(w, `{"error":"jenis resource (kind) dan nama wajib diisi"}`, http.StatusBadRequest)
		return
	}
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	client := s.ensureK8sClient()
	if err := client.DeleteResource(r.Context(), req.Kind, req.Namespace, req.Name); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal menghapus resource: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "k8s.delete", fmt.Sprintf("%s/%s", req.Kind, req.Name), true,
		fmt.Sprintf("Menghapus resource Kubernetes %s %s pada namespace %s", req.Kind, req.Name, req.Namespace), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Resource %s '%s' berhasil dihapus", req.Kind, req.Name),
	})
}

// handleK8sInstallK3s memasang cluster K3s secara otomatis via script resmi Rancher
func (s *Server) handleK8sInstallK3s(w http.ResponseWriter, r *http.Request) {
	// Pastikan hanya dijalankan jika K8s belum terpasang
	client := s.ensureK8sClient()
	status, _ := client.GetStatus(r.Context())
	if status.Available {
		http.Error(w, `{"error":"Kubernetes sudah terpasang dan aktif di server ini"}`, http.StatusBadRequest)
		return
	}

	go func() {
		// Jalankan instalasi K3s via curl script di latar belakang
		cmd := exec.Command("sh", "-c", "curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644")
		_ = cmd.Run()
	}()

	_ = s.store.RecordAudit(r.Context(), "user", "k8s.install", "k3s", true, "Memulai instalasi otomatis K3s lightweight Kubernetes", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Instalasi K3s telah dimulai di latar belakang. Silakan refresh status dalam beberapa saat.",
	})
}
