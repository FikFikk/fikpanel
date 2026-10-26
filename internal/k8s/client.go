package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	kubeconfig string
	binPath    string
	clusterType string
}

func NewClient() *Client {
	c := &Client{clusterType: "none"}
	c.detectEnvironment()
	return c
}

// detectEnvironment mendeteksi jenis cluster Kubernetes dan biner kubectl
func (c *Client) detectEnvironment() {
	// 1. Cek K3s bawaan
	if _, err := os.Stat("/etc/rancher/k3s/k3s.yaml"); err == nil {
		c.kubeconfig = "/etc/rancher/k3s/k3s.yaml"
		c.clusterType = "k3s"
		if path, err := exec.LookPath("k3s"); err == nil {
			c.binPath = path
			return
		}
	}

	// 2. Cek MicroK8s
	if path, err := exec.LookPath("microk8s"); err == nil {
		c.binPath = path
		c.clusterType = "microk8s"
		return
	}

	// 3. Cek kubectl standar
	if path, err := exec.LookPath("kubectl"); err == nil {
		c.binPath = path
		c.clusterType = "standard"
		if envKube := os.Getenv("KUBECONFIG"); envKube != "" {
			c.kubeconfig = envKube
		}
	}
}

// runKubectl menjalankan perintah kubectl dengan argumen terformat
func (c *Client) runKubectl(ctx context.Context, stdinInput string, args ...string) ([]byte, error) {
	if c.binPath == "" {
		return nil, errors.New("Kubernetes (kubectl/k3s) tidak ditemukan di server")
	}

	var finalArgs []string
	if c.clusterType == "k3s" && strings.HasSuffix(c.binPath, "k3s") {
		finalArgs = append(finalArgs, "kubectl")
	} else if c.clusterType == "microk8s" {
		finalArgs = append(finalArgs, "kubectl")
	}

	if c.kubeconfig != "" && c.clusterType != "k3s" {
		finalArgs = append(finalArgs, "--kubeconfig", c.kubeconfig)
	}

	finalArgs = append(finalArgs, args...)

	cmd := exec.CommandContext(ctx, c.binPath, finalArgs...)
	if c.kubeconfig != "" && c.clusterType == "k3s" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+c.kubeconfig)
	}

	if stdinInput != "" {
		cmd.Stdin = strings.NewReader(stdinInput)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("eksekusi kubectl gagal: %s", errMsg)
	}

	return stdout.Bytes(), nil
}

// GetStatus memeriksa ketersediaan cluster dan mengambil ringkasan node
func (c *Client) GetStatus(ctx context.Context) (*ClusterStatus, error) {
	status := &ClusterStatus{
		Available:   false,
		ClusterType: c.clusterType,
	}

	if c.binPath == "" {
		return status, nil
	}

	out, err := c.runKubectl(ctx, "", "get", "nodes", "-o", "json")
	if err != nil {
		return status, nil
	}

	nodes, err := parseNodesFromJSON(out)
	if err != nil {
		return status, nil
	}

	status.Available = true
	status.NodesCount = len(nodes)
	status.Nodes = nodes

	// Hitung total pods aktif di seluruh namespace
	if podOut, err := c.runKubectl(ctx, "", "get", "pods", "-A", "-o", "json"); err == nil {
		if pods, err := parsePodsFromJSON(podOut); err == nil {
			status.PodsCount = len(pods)
		}
	}

	// Versi server
	if verOut, err := c.runKubectl(ctx, "", "version", "--output=json"); err == nil {
		var verMap map[string]interface{}
		if err := json.Unmarshal(verOut, &verMap); err == nil {
			if srv, ok := verMap["serverVersion"].(map[string]interface{}); ok {
				if gitVer, ok := srv["gitVersion"].(string); ok {
					status.ServerVersion = gitVer
				}
			}
		}
	}

	return status, nil
}

func parseNodesFromJSON(data []byte) ([]K8sNode, error) {
	var raw struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
				NodeInfo struct {
					KubeletVersion string `json:"kubeletVersion"`
					OSImage        string `json:"osImage"`
				} `json:"nodeInfo"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var nodes []K8sNode
	for _, item := range raw.Items {
		node := K8sNode{
			Name:    item.Metadata.Name,
			Version: item.Status.NodeInfo.KubeletVersion,
			OSImage: item.Status.NodeInfo.OSImage,
			Status:  "NotReady",
			Roles:   "worker",
		}

		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				node.Status = "Ready"
			}
		}

		for k := range item.Metadata.Labels {
			if strings.Contains(k, "node-role.kubernetes.io/control-plane") || strings.Contains(k, "node-role.kubernetes.io/master") {
				node.Roles = "control-plane"
				break
			}
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// ListNamespaces mengambil daftar seluruh namespace
func (c *Client) ListNamespaces(ctx context.Context) ([]K8sNamespace, error) {
	out, err := c.runKubectl(ctx, "", "get", "namespaces", "-o", "json")
	if err != nil {
		return nil, err
	}

	var raw struct {
		Items []struct {
			Metadata struct {
				Name              string    `json:"name"`
				CreationTimestamp time.Time `json:"creationTimestamp"`
			} `json:"metadata"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	var nss []K8sNamespace
	for _, item := range raw.Items {
		nss = append(nss, K8sNamespace{
			Name:   item.Metadata.Name,
			Status: item.Status.Phase,
			Age:    time.Since(item.Metadata.CreationTimestamp).Round(time.Minute).String(),
		})
	}

	return nss, nil
}

// ListPods mengambil daftar pod pada namespace tertentu (atau semua jika kosong)
func (c *Client) ListPods(ctx context.Context, namespace string) ([]K8sPod, error) {
	args := []string{"get", "pods", "-o", "json"}
	if namespace != "" && namespace != "all" {
		args = append(args, "-n", namespace)
	} else {
		args = append(args, "-A")
	}

	out, err := c.runKubectl(ctx, "", args...)
	if err != nil {
		return nil, err
	}

	return parsePodsFromJSON(out)
}

func parsePodsFromJSON(data []byte) ([]K8sPod, error) {
	var raw struct {
		Items []struct {
			Metadata struct {
				Name              string    `json:"name"`
				Namespace         string    `json:"namespace"`
				CreationTimestamp time.Time `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				NodeName   string `json:"nodeName"`
				Containers []struct {
					Name  string `json:"name"`
					Image string `json:"image"`
				} `json:"containers"`
			} `json:"spec"`
			Status struct {
				Phase             string `json:"phase"`
				PodIP             string `json:"podIP"`
				ContainerStatuses []struct {
					Name         string `json:"name"`
					Ready        bool   `json:"ready"`
					RestartCount int    `json:"restartCount"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var pods []K8sPod
	for _, item := range raw.Items {
		readyCount := 0
		totalRestarts := 0
		for _, cs := range item.Status.ContainerStatuses {
			if cs.Ready {
				readyCount++
			}
			totalRestarts += cs.RestartCount
		}

		var containers []string
		for _, ct := range item.Spec.Containers {
			containers = append(containers, ct.Name)
		}

		totalContainers := len(item.Spec.Containers)
		if totalContainers == 0 {
			totalContainers = len(item.Status.ContainerStatuses)
		}

		pods = append(pods, K8sPod{
			Name:       item.Metadata.Name,
			Namespace:  item.Metadata.Namespace,
			Status:     item.Status.Phase,
			Ready:      fmt.Sprintf("%d/%d", readyCount, totalContainers),
			Restarts:   totalRestarts,
			IP:         item.Status.PodIP,
			Node:       item.Spec.NodeName,
			Age:        time.Since(item.Metadata.CreationTimestamp).Round(time.Minute).String(),
			Containers: containers,
		})
	}

	return pods, nil
}

// ListDeployments mengambil daftar deployment workload
func (c *Client) ListDeployments(ctx context.Context, namespace string) ([]K8sDeployment, error) {
	args := []string{"get", "deployments", "-o", "json"}
	if namespace != "" && namespace != "all" {
		args = append(args, "-n", namespace)
	} else {
		args = append(args, "-A")
	}

	out, err := c.runKubectl(ctx, "", args...)
	if err != nil {
		return nil, err
	}

	return parseDeploymentsFromJSON(out)
}

func parseDeploymentsFromJSON(data []byte) ([]K8sDeployment, error) {
	var raw struct {
		Items []struct {
			Metadata struct {
				Name              string    `json:"name"`
				Namespace         string    `json:"namespace"`
				CreationTimestamp time.Time `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Replicas int `json:"replicas"`
				Template struct {
					Spec struct {
						Containers []struct {
							Image string `json:"image"`
						} `json:"containers"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
			Status struct {
				ReadyReplicas     int `json:"readyReplicas"`
				AvailableReplicas int `json:"availableReplicas"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var deps []K8sDeployment
	for _, item := range raw.Items {
		var images []string
		for _, ct := range item.Spec.Template.Spec.Containers {
			images = append(images, ct.Image)
		}

		deps = append(deps, K8sDeployment{
			Name:              item.Metadata.Name,
			Namespace:         item.Metadata.Namespace,
			Replicas:          item.Spec.Replicas,
			ReadyReplicas:     item.Status.ReadyReplicas,
			AvailableReplicas: item.Status.AvailableReplicas,
			Age:               time.Since(item.Metadata.CreationTimestamp).Round(time.Minute).String(),
			Images:            images,
		})
	}

	return deps, nil
}

// ListServices mengambil daftar service jaringan Kubernetes
func (c *Client) ListServices(ctx context.Context, namespace string) ([]K8sService, error) {
	args := []string{"get", "services", "-o", "json"}
	if namespace != "" && namespace != "all" {
		args = append(args, "-n", namespace)
	} else {
		args = append(args, "-A")
	}

	out, err := c.runKubectl(ctx, "", args...)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Items []struct {
			Metadata struct {
				Name              string    `json:"name"`
				Namespace         string    `json:"namespace"`
				CreationTimestamp time.Time `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Type      string `json:"type"`
				ClusterIP string `json:"clusterIP"`
				Ports     []struct {
					Port     int    `json:"port"`
					Protocol string `json:"protocol"`
					NodePort int    `json:"nodePort,omitempty"`
				} `json:"ports"`
			} `json:"spec"`
			Status struct {
				LoadBalancer struct {
					Ingress []struct {
						IP string `json:"ip"`
					} `json:"ingress"`
				} `json:"loadBalancer"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	var svcs []K8sService
	for _, item := range raw.Items {
		var portStrs []string
		for _, p := range item.Spec.Ports {
			if p.NodePort > 0 {
				portStrs = append(portStrs, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol))
			} else {
				portStrs = append(portStrs, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
			}
		}

		extIP := "-"
		if len(item.Status.LoadBalancer.Ingress) > 0 {
			extIP = item.Status.LoadBalancer.Ingress[0].IP
		}

		svcs = append(svcs, K8sService{
			Name:       item.Metadata.Name,
			Namespace:  item.Metadata.Namespace,
			Type:       item.Spec.Type,
			ClusterIP:  item.Spec.ClusterIP,
			ExternalIP: extIP,
			Ports:      strings.Join(portStrs, ", "),
			Age:        time.Since(item.Metadata.CreationTimestamp).Round(time.Minute).String(),
		})
	}

	return svcs, nil
}

// ScaleDeployment mengubah jumlah replika pod pada deployment tertentu
func (c *Client) ScaleDeployment(ctx context.Context, namespace, name string, replicas int) error {
	if namespace == "" {
		namespace = "default"
	}
	replicaArg := fmt.Sprintf("--replicas=%d", replicas)
	_, err := c.runKubectl(ctx, "", "scale", "deployment", name, replicaArg, "-n", namespace)
	return err
}

// ApplyManifest mengeksekusi deklarasi YAML manifest langsung ke cluster
func (c *Client) ApplyManifest(ctx context.Context, yamlContent string) (string, error) {
	if strings.TrimSpace(yamlContent) == "" {
		return "", errors.New("konten manifest YAML tidak boleh kosong")
	}

	out, err := c.runKubectl(ctx, yamlContent, "apply", "-f", "-")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetPodLogs membaca baris log dari kontainer di dalam Pod
func (c *Client) GetPodLogs(ctx context.Context, namespace, name, container string, lines int) (string, error) {
	if namespace == "" {
		namespace = "default"
	}
	if lines <= 0 || lines > 500 {
		lines = 100
	}

	args := []string{"logs", name, "-n", namespace, "--tail=" + strconv.Itoa(lines)}
	if container != "" {
		args = append(args, "-c", container)
	}

	out, err := c.runKubectl(ctx, "", args...)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// DeleteResource menghapus objek Kubernetes (pod, deployment, service)
func (c *Client) DeleteResource(ctx context.Context, kind, namespace, name string) error {
	if namespace == "" {
		namespace = "default"
	}
	_, err := c.runKubectl(ctx, "", "delete", kind, name, "-n", namespace)
	return err
}
