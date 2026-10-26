package discovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseNginxConfig(t *testing.T) {
	sampleConf := `
server {
    listen 80;
    server_name kostash.my.id www.kostash.my.id;
    root /home/fikfikk/projects/web/kostash/public;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }
}

server {
    listen 443 ssl;
    server_name qr-fik.id;
    ssl_certificate /etc/letsencrypt/live/qr-fik.id/fullchain.pem;

    location /api {
        proxy_pass http://127.0.0.1:3000;
    }
}
`
	tmpDir := t.TempDir()
	confFile := filepath.Join(tmpDir, "sample.conf")
	if err := os.WriteFile(confFile, []byte(sampleConf), 0644); err != nil {
		t.Fatalf("Gagal membuat file sementara: %v", err)
	}

	items := parseConfigFile(confFile)
	if len(items) != 2 {
		t.Fatalf("Diharapkan 2 vhost, didapatkan: %d", len(items))
	}

	// Verifikasi vhost 1
	if items[0].ServerName != "kostash.my.id" {
		t.Errorf("ServerName tidak sesuai: %s", items[0].ServerName)
	}
	if items[0].RootDir != "/home/fikfikk/projects/web/kostash/public" {
		t.Errorf("RootDir tidak sesuai: %s", items[0].RootDir)
	}
	if items[0].Port != 80 {
		t.Errorf("Port tidak sesuai: %d", items[0].Port)
	}

	// Verifikasi vhost 2 (SSL + Reverse Proxy)
	if items[1].ServerName != "qr-fik.id" {
		t.Errorf("ServerName tidak sesuai: %s", items[1].ServerName)
	}
	if !items[1].SSL {
		t.Errorf("Diharapkan SSL bernilai true")
	}
	if items[1].ProxyPass != "http://127.0.0.1:3000" {
		t.Errorf("ProxyPass tidak sesuai: %s", items[1].ProxyPass)
	}
}

func TestScanDockerComposeDirectory(t *testing.T) {
	tmpRoot := t.TempDir()
	projDir := filepath.Join(tmpRoot, "projects", "web", "app1")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("Gagal membuat folder: %v", err)
	}

	composeContent := `
services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
`
	composeFile := filepath.Join(projDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		t.Fatalf("Gagal membuat compose file: %v", err)
	}

	projects := scanComposeInPaths(context.Background(), []string{tmpRoot})
	if len(projects) != 1 {
		t.Fatalf("Diharapkan 1 proyek compose, didapatkan: %d", len(projects))
	}
	if projects[0].ProjectName != "app1" {
		t.Errorf("Nama proyek compose salah: %s", projects[0].ProjectName)
	}
	if projects[0].ComposePath != composeFile {
		t.Errorf("ComposePath salah: %s", projects[0].ComposePath)
	}
}
