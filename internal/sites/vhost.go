package sites

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"fikpanel/internal/store"
)

// vhostTmpl adalah template server block Nginx untuk static, php, dan proxy
var vhostTmpl = template.Must(template.New("vhost").Parse(`# Dikelola oleh FikPanel untuk {{.Domain}} — perubahan manual akan ditimpa.
server {
    listen 80;
    listen [::]:80;
    server_name {{.ServerNames}};

    location ^~ /.well-known/acme-challenge/ {
        root {{.ACMEWebroot}};
        default_type "text/plain";
        try_files $uri =404;
    }
{{- if and .SSL .ForceHTTPS}}

    location / {
        return 301 https://$host$request_uri;
    }
}
{{- else}}
{{template "body" .}}
}
{{- end}}
{{- if .SSL}}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name {{.ServerNames}};

    ssl_certificate     {{.CertPath}};
    ssl_certificate_key {{.KeyPath}};
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:FIKPANEL_SSL:10m;
    ssl_session_timeout 1d;
{{- if .HSTS}}
    add_header Strict-Transport-Security "max-age=31536000" always;
{{- end}}

    location ^~ /.well-known/acme-challenge/ {
        root {{.ACMEWebroot}};
        default_type "text/plain";
        try_files $uri =404;
    }
{{template "body" .}}
}
{{- end}}
{{define "body"}}
    access_log {{.LogDir}}/{{.Domain}}.access.log;
    error_log  {{.LogDir}}/{{.Domain}}.error.log;
{{- if eq .Kind "proxy"}}

    location / {
        proxy_pass {{.Upstream}};
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $http_connection;
        proxy_read_timeout 60s;
    }
{{- else}}

    root {{.Root}};
{{- if eq .Kind "php"}}
    index index.php index.html index.htm;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        include fastcgi_params;
        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
        fastcgi_pass unix:{{.PHPSocket}};
    }
{{- else}}
    index index.html index.htm;

    location / {
        try_files $uri $uri/ =404;
    }
{{- end}}

    location ~ /\.(?!well-known) {
        deny all;
    }
{{- end}}
{{- if .ExtraConf}}

    # Konfigurasi tambahan
    {{.ExtraConf}}
{{- end}}
{{end}}`))

type vhostData struct {
	Domain      string
	ServerNames string
	Kind        string
	Root        string
	Upstream    string
	PHPSocket   string
	ExtraConf   string
	LogDir      string
	ACMEWebroot string
	SSL         bool
	ForceHTTPS  bool
	HSTS        bool
	CertPath    string
	KeyPath     string
}

// AllDomains mengembalikan domain utama + alias situs
func AllDomains(site *store.Site) []string {
	var aliases []string
	_ = json.Unmarshal([]byte(site.Aliases), &aliases)
	return append([]string{site.Domain}, aliases...)
}

// phpSocket menentukan socket PHP-FPM sesuai versi (konvensi Debian/Ubuntu)
func phpSocket(version string) string {
	if version == "" {
		return "/run/php/php-fpm.sock"
	}
	return fmt.Sprintf("/run/php/php%s-fpm.sock", version)
}

// certFiles mengembalikan path fullchain & privkey sertifikat situs
func (m *Manager) certFiles(domain string) (string, string) {
	dir := filepath.ToSlash(filepath.Join(m.paths.CertDir, domain))
	return dir + "/fullchain.pem", dir + "/privkey.pem"
}

// Render menghasilkan konfigurasi vhost; blok 443 hanya aktif bila sertifikat sudah terbit
func (m *Manager) Render(site *store.Site, cert *store.Certificate) (string, error) {
	certPath, keyPath := m.certFiles(site.Domain)
	sslReady := site.SSLMode != "none" && cert != nil && cert.Status == "issued" && m.certExists(site.Domain)

	data := vhostData{
		Domain:      site.Domain,
		ServerNames: strings.Join(AllDomains(site), " "),
		Kind:        site.Kind,
		Root:        site.RootPath,
		Upstream:    site.Upstream,
		PHPSocket:   phpSocket(site.PHPVersion),
		ExtraConf:   strings.TrimSpace(site.ExtraConf),
		LogDir:      filepath.ToSlash(m.paths.LogDir),
		ACMEWebroot: filepath.ToSlash(m.paths.ACMEWebroot),
		SSL:         sslReady,
		ForceHTTPS:  site.ForceHTTPS == 1,
		HSTS:        site.HSTS == 1,
		CertPath:    certPath,
		KeyPath:     keyPath,
	}
	var buf bytes.Buffer
	if err := vhostTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("gagal merender vhost: %w", err)
	}
	return buf.String(), nil
}
