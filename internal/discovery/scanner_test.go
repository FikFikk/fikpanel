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
	if items[0].ServerName != "kostash.my.id www.kostash.my.id" {
		t.Errorf("ServerName tidak sesuai: %s", items[0].ServerName)
	}
	if len(items[0].Aliases) == 0 || items[0].Aliases[0] != "www.kostash.my.id" {
		t.Errorf("Aliases tidak sesuai: %v", items[0].Aliases)
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

// TestParseNginxConfigIgnoresComments memastikan komentar dan template contoh diabaikan
func TestParseNginxConfigIgnoresComments(t *testing.T) {
	ubuntuDefaultConf := `
server {
	listen 80 default_server;
	root /var/www/html;
	index index.html;
	server_name _;
}

# Virtual Host configuration for example.com
#
#server {
#	listen 80;
#	server_name example.com;
#	root /var/www/example.com;
#}
`
	tmpDir := t.TempDir()
	confFile := filepath.Join(tmpDir, "default")
	if err := os.WriteFile(confFile, []byte(ubuntuDefaultConf), 0644); err != nil {
		t.Fatalf("Gagal membuat file sementara: %v", err)
	}

	items := parseConfigFile(confFile)
	if len(items) != 0 {
		t.Fatalf("Diharapkan 0 vhost untuk default template terkomentar, tetapi didapatkan: %d (%+v)", len(items), items)
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

// TestParseNginxConfigNestedLocations menguji pemindaian konfigurasi Nginx dengan lokasi bersarang (nested locations)
func TestParseNginxConfigNestedLocations(t *testing.T) {
	kostashConf := `
server {
    listen 80;
    server_name kostash.my.id www.kostash.my.id;
    root /var/www/kostash/public;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location /phpmyadmin {
        alias /usr/share/phpmyadmin;
        location ~ \.php$ {
            fastcgi_pass unix:/run/php/php8.3-fpm.sock;
            fastcgi_param SCRIPT_FILENAME $request_filename;
            include fastcgi_params;
        }
    }
}
`
	tmpDir := t.TempDir()
	confFile := filepath.Join(tmpDir, "kostash.conf")
	if err := os.WriteFile(confFile, []byte(kostashConf), 0644); err != nil {
		t.Fatalf("Gagal membuat file sementara: %v", err)
	}

	items := parseConfigFile(confFile)
	if len(items) != 1 {
		t.Fatalf("Diharapkan 1 vhost untuk konfigurasi nested location, didapatkan: %d", len(items))
	}

	if items[0].ServerName != "kostash.my.id www.kostash.my.id" {
		t.Errorf("ServerName salah: got %s, want kostash.my.id www.kostash.my.id", items[0].ServerName)
	}
	if items[0].RootDir != "/var/www/kostash/public" {
		t.Errorf("RootDir salah: got %s, want /var/www/kostash/public", items[0].RootDir)
	}
	if items[0].Port != 80 {
		t.Errorf("Port salah: got %d, want 80", items[0].Port)
	}
	if len(items[0].Aliases) != 1 || items[0].Aliases[0] != "www.kostash.my.id" {
		t.Errorf("Aliases salah: %v", items[0].Aliases)
	}
}

func TestParseCloudflaredIngress(t *testing.T) {
	sampleYAML := `
tunnel: 6ff42ae2-765d-4adf-8112-31c55c1551ef
credentials-file: /root/.cloudflared/6ff42ae2-765d-4adf-8112-31c55c1551ef.json

ingress:
  - hostname: go.fikfikk.my.id
    service: http://localhost:5000
  - hostname: "nuxt.fikfikk.my.id"
    service: "http://127.0.0.1:3000"
  - hostname: api.fikfikk.my.id
    service: http://localhost:8080
  - hostname: ssh.fikfikk.my.id
    service: ssh://localhost:22
  - hostname: db.fikfikk.my.id
    service: tcp://localhost:3306
  - service: http_status:404
`
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(cfgFile, []byte(sampleYAML), 0o644); err != nil {
		t.Fatalf("Gagal membuat file sementara: %v", err)
	}

	items := parseCloudflaredIngress(cfgFile)
	if len(items) != 3 {
		t.Fatalf("Diharapkan 3 aturan ingress, didapatkan: %d", len(items))
	}

	// Verifikasi item 1
	if items[0].ServerName != "go.fikfikk.my.id" {
		t.Errorf("ServerName salah: got %s, want go.fikfikk.my.id", items[0].ServerName)
	}
	if items[0].ProxyPass != "http://localhost:5000" {
		t.Errorf("ProxyPass salah: got %s, want http://localhost:5000", items[0].ProxyPass)
	}
	if items[0].Port != 5000 {
		t.Errorf("Port salah: got %d, want 5000", items[0].Port)
	}
	if !items[0].SSL {
		t.Errorf("Diharapkan SSL bernilai true untuk Cloudflare Tunnel")
	}

	// Verifikasi item 2
	if items[1].ServerName != "nuxt.fikfikk.my.id" {
		t.Errorf("ServerName salah: got %s, want nuxt.fikfikk.my.id", items[1].ServerName)
	}
	if items[1].Port != 3000 {
		t.Errorf("Port salah: got %d, want 3000", items[1].Port)
	}

	// Verifikasi item 3
	if items[2].ServerName != "api.fikfikk.my.id" {
		t.Errorf("ServerName salah: got %s, want api.fikfikk.my.id", items[2].ServerName)
	}
	if items[2].Port != 8080 {
		t.Errorf("Port salah: got %d, want 8080", items[2].Port)
	}
}

func TestScanDatabases(t *testing.T) {
	scanner := NewScanner()
	items, err := scanner.ScanDatabases(context.Background())
	if err != nil {
		t.Fatalf("ScanDatabases mengembalikan error: %v", err)
	}
	if items == nil {
		t.Fatalf("ScanDatabases tidak boleh mengembalikan nil slice")
	}
}
