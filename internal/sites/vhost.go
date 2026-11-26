package sites

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"fikpanel/internal/store"
	"fikpanel/internal/waf"
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
{{- if .WAFInclude}}
    include {{.WAFInclude}};
{{- end}}

    # Halaman galat (Prioritas: /error/*.html di proyek -> fallback bawaan FikPanel)
    error_page 403 /_fik_error_403;
    error_page 404 /_fik_error_404;
    error_page 500 502 503 504 /_fik_error_50x;

    location = /_fik_error_403 {
{{- if .Root}}
        root {{.Root}};
        try_files /error/403.html /403.html @fallback_403;
{{- else}}
        root {{.ErrorDir}};
        rewrite ^ /403.html break;
{{- end}}
        internal;
    }
    location @fallback_403 {
        root {{.ErrorDir}};
        rewrite ^ /403.html break;
    }

    location = /_fik_error_404 {
{{- if .Root}}
        root {{.Root}};
        try_files /error/404.html /404.html @fallback_404;
{{- else}}
        root {{.ErrorDir}};
        rewrite ^ /404.html break;
{{- end}}
        internal;
    }
    location @fallback_404 {
        root {{.ErrorDir}};
        rewrite ^ /404.html break;
    }

    location = /_fik_error_50x {
{{- if .Root}}
        root {{.Root}};
        try_files /error/50x.html /50x.html /error/500.html /500.html @fallback_50x;
{{- else}}
        root {{.ErrorDir}};
        rewrite ^ /50x.html break;
{{- end}}
        internal;
    }
    location @fallback_50x {
        root {{.ErrorDir}};
        rewrite ^ /50x.html break;
    }
{{- if or (eq .Kind "proxy") (eq .Kind "node") (eq .Kind "go") (eq .Kind "python")}}
    client_max_body_size 64m;

    location / {
        proxy_pass {{.Upstream}};
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $http_connection;
        proxy_connect_timeout 15s;
        proxy_send_timeout 120s;
        proxy_read_timeout 120s;
    }
{{- else}}

    root {{.Root}};
{{- if eq .Kind "php"}}
    index index.php index.html index.htm;
    client_max_body_size 64m;
{{- if not .HasCustomRootLocation}}

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }
{{- end}}

    location ~ \.php$ {
        try_files $uri =404;
        include fastcgi_params;
        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
        fastcgi_read_timeout 120s;
        fastcgi_buffers 16 16k;
        fastcgi_buffer_size 32k;
        fastcgi_pass unix:{{.PHPSocket}};
    }
{{- else}}
    index index.html index.htm;
{{- if not .HasCustomRootLocation}}

    location / {
{{- if .IsSPA}}
        try_files $uri $uri/ /index.html;
{{- else}}
        try_files $uri $uri/ =404;
{{- end}}
    }
{{- end}}
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
	Domain                string
	ServerNames           string
	Kind                  string
	Root                  string
	Upstream              string
	PHPSocket             string
	ExtraConf             string
	LogDir                string
	ACMEWebroot           string
	ErrorDir              string
	SSL                   bool
	ForceHTTPS            bool
	HSTS                  bool
	CertPath              string
	KeyPath               string
	WAFInclude            string
	IsSPA                 bool
	HasCustomRootLocation bool
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

// CertFiles mengembalikan path fullchain & privkey sertifikat situs
func (m *Manager) CertFiles(domain string) (string, string) {
	return m.certFiles(domain)
}

// certFiles mengembalikan path fullchain & privkey sertifikat situs
func (m *Manager) certFiles(domain string) (string, string) {
	dir := filepath.ToSlash(filepath.Join(m.paths.CertDir, domain))
	return dir + "/fullchain.pem", dir + "/privkey.pem"
}

// Render menghasilkan konfigurasi vhost; blok 443 hanya aktif bila sertifikat sudah terbit
func (m *Manager) Render(site *store.Site, cert *store.Certificate) (string, error) {
	certPath, keyPath := m.certFiles(site.Domain)
	sslReady := site.SSLMode != "none" && site.SSLMode != "cloudflare" && cert != nil && cert.Status == "issued" && m.certExists(site.Domain)

	// Sisipkan aturan WAF Nginx secara otomatis bila berkas waf-rules.conf sudah tersedia pada host
	wafInclude := ""
	if !m.mock {
		wafPath := waf.DefaultWAFRulesPath()
		if fi, err := os.Stat(wafPath); err == nil && !fi.IsDir() {
			wafInclude = wafPath
		}
	}

	// Deteksi apakah situs statis memakai routing SPA (Vite/React/Vue/Svelte) dan apakah ExtraConf sudah menimpa location /
	extraTrim := strings.TrimSpace(site.ExtraConf)
	fwLower := strings.ToLower(strings.TrimSpace(site.Framework))
	isSPA := site.Kind == "static" && (strings.Contains(fwLower, "spa") ||
		strings.Contains(fwLower, "vite") ||
		strings.Contains(fwLower, "react") ||
		strings.Contains(fwLower, "vue") ||
		strings.Contains(fwLower, "svelte") ||
		strings.Contains(fwLower, "angular"))
	hasCustomRootLoc := strings.Contains(extraTrim, "location / {") || strings.Contains(extraTrim, "location /{")

	data := vhostData{
		Domain:                site.Domain,
		ServerNames:           strings.Join(AllDomains(site), " "),
		Kind:                  site.Kind,
		Root:                  site.RootPath,
		Upstream:              site.Upstream,
		PHPSocket:             phpSocket(site.PHPVersion),
		ExtraConf:             extraTrim,
		LogDir:                filepath.ToSlash(m.paths.LogDir),
		ACMEWebroot:           filepath.ToSlash(m.paths.ACMEWebroot),
		ErrorDir:              filepath.ToSlash(m.paths.ErrorDir),
		SSL:                   sslReady,
		ForceHTTPS:            site.ForceHTTPS == 1,
		HSTS:                  site.HSTS == 1,
		CertPath:              certPath,
		KeyPath:               keyPath,
		WAFInclude:            wafInclude,
		IsSPA:                 isSPA,
		HasCustomRootLocation: hasCustomRootLoc,
	}
	var buf bytes.Buffer
	if err := vhostTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("gagal merender vhost: %w", err)
	}
	return buf.String(), nil
}
