package k8s

import (
	"context"
	"testing"
)

func TestParsePodsJSON(t *testing.T) {
	samplePodsJSON := `{
		"items": [
			{
				"metadata": {
					"name": "web-pod-xyz",
					"namespace": "production",
					"creationTimestamp": "2026-10-26T10:00:00Z"
				},
				"status": {
					"phase": "Running",
					"podIP": "10.42.0.12",
					"containerStatuses": [
						{
							"name": "nginx-web",
							"ready": true,
							"restartCount": 2
						}
					]
				},
				"spec": {
					"nodeName": "node-1"
				}
			}
		]
	}`

	pods, err := parsePodsFromJSON([]byte(samplePodsJSON))
	if err != nil {
		t.Fatalf("Gagal parse pods json: %v", err)
	}

	if len(pods) != 1 {
		t.Fatalf("Diharapkan 1 pod, didapatkan: %d", len(pods))
	}

	p := pods[0]
	if p.Name != "web-pod-xyz" || p.Namespace != "production" {
		t.Errorf("Identitas pod tidak sesuai: %s/%s", p.Namespace, p.Name)
	}
	if p.Status != "Running" {
		t.Errorf("Status pod tidak sesuai: %s", p.Status)
	}
	if p.Restarts != 2 {
		t.Errorf("Restart count tidak sesuai: %d", p.Restarts)
	}
	if p.Ready != "1/1" {
		t.Errorf("Ready count tidak sesuai: %s", p.Ready)
	}
}

func TestParseDeploymentsJSON(t *testing.T) {
	sampleDeployJSON := `{
		"items": [
			{
				"metadata": {
					"name": "api-service",
					"namespace": "default",
					"creationTimestamp": "2026-10-26T08:00:00Z"
				},
				"spec": {
					"replicas": 3,
					"template": {
						"spec": {
							"containers": [
								{ "image": "node:20-alpine" }
							]
						}
					}
				},
				"status": {
					"replicas": 3,
					"readyReplicas": 3,
					"availableReplicas": 3
				}
			}
		]
	}`

	deps, err := parseDeploymentsFromJSON([]byte(sampleDeployJSON))
	if err != nil {
		t.Fatalf("Gagal parse deployments json: %v", err)
	}

	if len(deps) != 1 {
		t.Fatalf("Diharapkan 1 deployment, didapatkan: %d", len(deps))
	}

	d := deps[0]
	if d.Name != "api-service" || d.Replicas != 3 || d.ReadyReplicas != 3 {
		t.Errorf("Data deployment tidak cocok: %+v", d)
	}
	if len(d.Images) == 0 || d.Images[0] != "node:20-alpine" {
		t.Errorf("Image deployment tidak sesuai: %+v", d.Images)
	}
}

func TestClientFallbackWhenUnavailable(t *testing.T) {
	// Klien harus mengembalikan status Available: false dengan anggun tanpa error fatal
	client := NewClient()
	status, err := client.GetStatus(context.Background())
	if err != nil {
		t.Fatalf("GetStatus tidak boleh melempar error saat k8s tidak ada: %v", err)
	}
	if status == nil {
		t.Fatal("Status tidak boleh bernilai nil")
	}
}
