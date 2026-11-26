package docker

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// GetTemplates menyajikan katalog 15 aplikasi Docker One-Click siap pakai dengan kontrak penuh
func (m *Manager) GetTemplates() []OneClickTemplate {
	return []OneClickTemplate{
		{
			ID:             "wordpress",
			Name:           "WordPress + MariaDB",
			Category:       "cms",
			Description:    "Platform CMS & blogging paling populer lengkap dengan database MariaDB terisolasi.",
			Icon:           "ri-wordpress-line",
			Image:          "wordpress:latest",
			DefaultPort:    8080,
			DefaultPorts:   []string{"8080:80"},
			DefaultVolumes: []string{"./wp_data:/var/www/html", "./db_data:/var/lib/mysql"},
			VolumeMount:    "/var/www/html",
			DefaultEnv: map[string]string{
				"WORDPRESS_DB_USER":          "wpuser",
				"WORDPRESS_DB_PASSWORD":      "WpStrongPass2026!",
				"WORDPRESS_DB_NAME":          "wordpress",
				"MYSQL_ROOT_PASSWORD":        "RootStrongPass2026!",
			},
			EnvParams: []OneClickEnvParam{
				{Key: "WORDPRESS_DB_NAME", Label: "Nama Database", Default: "wordpress", Type: "text"},
				{Key: "WORDPRESS_DB_USER", Label: "User Database", Default: "wpuser", Type: "text"},
				{Key: "WORDPRESS_DB_PASSWORD", Label: "Password Database", Default: "WpStrongPass2026!", Type: "password"},
				{Key: "MYSQL_ROOT_PASSWORD", Label: "Password Root MariaDB", Default: "RootStrongPass2026!", Type: "password"},
			},
			DockerComposeYAML: `services:
  db:
    image: mariadb:11
    restart: unless-stopped
    environment:
      MYSQL_ROOT_PASSWORD: RootStrongPass2026!
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wpuser
      MYSQL_PASSWORD: WpStrongPass2026!
    volumes:
      - ./db_data:/var/lib/mysql
  wordpress:
    depends_on:
      - db
    image: wordpress:latest
    restart: unless-stopped
    ports:
      - "8080:80"
    environment:
      WORDPRESS_DB_HOST: db:3306
      WORDPRESS_DB_USER: wpuser
      WORDPRESS_DB_PASSWORD: WpStrongPass2026!
      WORDPRESS_DB_NAME: wordpress
    volumes:
      - ./wp_data:/var/www/html
`,
		},
		{
			ID:             "n8n",
			Name:           "n8n Workflow Automation",
			Category:       "tools",
			Description:    "Platform otomasi workflow low-code & AI agent self-hosted yang sangat fleksibel.",
			Icon:           "ri-flow-chart",
			Image:          "n8nio/n8n:latest",
			DefaultPort:    5678,
			DefaultPorts:   []string{"5678:5678"},
			DefaultVolumes: []string{"./n8n_data:/home/node/.n8n"},
			VolumeMount:    "/home/node/.n8n",
			DefaultEnv: map[string]string{
				"GENERIC_TIMEZONE": "Asia/Jakarta",
				"TZ":               "Asia/Jakarta",
			},
			EnvParams: []OneClickEnvParam{
				{Key: "GENERIC_TIMEZONE", Label: "Zona Waktu (Timezone)", Default: "Asia/Jakarta", Type: "text"},
				{Key: "N8N_ENCRYPTION_KEY", Label: "Encryption Key (Opsional)", Default: "changeme-n8n-secret-key", Type: "password"},
			},
			DockerComposeYAML: `services:
  n8n:
    image: n8nio/n8n:latest
    restart: unless-stopped
    ports:
      - "5678:5678"
    environment:
      GENERIC_TIMEZONE: Asia/Jakarta
      TZ: Asia/Jakarta
      N8N_ENCRYPTION_KEY: changeme-n8n-secret-key
    volumes:
      - ./n8n_data:/home/node/.n8n
`,
		},
		{
			ID:             "uptime-kuma",
			Name:           "Uptime Kuma",
			Category:       "monitoring",
			Description:    "Monitoring uptime HTTP/TCP/Docker & status page elegan dengan notifikasi Telegram/Discord.",
			Icon:           "ri-pulse-line",
			Image:          "louislam/uptime-kuma:1",
			DefaultPort:    3001,
			DefaultPorts:   []string{"3001:3001"},
			DefaultVolumes: []string{"./kuma_data:/app/data"},
			VolumeMount:    "/app/data",
			DefaultEnv:     map[string]string{"TZ": "Asia/Jakarta"},
			EnvParams: []OneClickEnvParam{
				{Key: "TZ", Label: "Zona Waktu", Default: "Asia/Jakarta", Type: "text"},
			},
			DockerComposeYAML: `services:
  uptime-kuma:
    image: louislam/uptime-kuma:1
    restart: unless-stopped
    ports:
      - "3001:3001"
    environment:
      TZ: Asia/Jakarta
    volumes:
      - ./kuma_data:/app/data
`,
		},
		{
			ID:             "minio",
			Name:           "MinIO Object Storage",
			Category:       "database",
			Description:    "Object storage berperforma tinggi yang 100% kompatibel dengan AWS S3 API & web console.",
			Icon:           "ri-hard-drive-2-line",
			Image:          "minio/minio:latest",
			DefaultPort:    9001,
			DefaultPorts:   []string{"9000:9000", "9001:9001"},
			DefaultVolumes: []string{"./minio_data:/data"},
			VolumeMount:    "/data",
			DefaultEnv: map[string]string{
				"MINIO_ROOT_USER":     "minioadmin",
				"MINIO_ROOT_PASSWORD": "MinioStrongPass2026!",
			},
			EnvParams: []OneClickEnvParam{
				{Key: "MINIO_ROOT_USER", Label: "Root Access Key", Default: "minioadmin", Type: "text"},
				{Key: "MINIO_ROOT_PASSWORD", Label: "Root Secret Key", Default: "MinioStrongPass2026!", Type: "password"},
			},
			DockerComposeYAML: `services:
  minio:
    image: minio/minio:latest
    restart: unless-stopped
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: MinioStrongPass2026!
    volumes:
      - ./minio_data:/data
`,
		},
		{
			ID:             "portainer",
			Name:           "Portainer CE",
			Category:       "tools",
			Description:    "Manajer kontainer web alternatif untuk mengelola Docker & Kubernetes multi-node.",
			Icon:           "ri-ship-2-line",
			Image:          "portainer/portainer-ce:latest",
			DefaultPort:    9443,
			DefaultPorts:   []string{"9443:9443"},
			DefaultVolumes: []string{"/var/run/docker.sock:/var/run/docker.sock", "./portainer_data:/data"},
			VolumeMount:    "/data",
			DefaultEnv:     map[string]string{},
			EnvParams:      []OneClickEnvParam{},
			DockerComposeYAML: `services:
  portainer:
    image: portainer/portainer-ce:latest
    restart: unless-stopped
    ports:
      - "9443:9443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./portainer_data:/data
`,
		},
		{
			ID:             "grafana",
			Name:           "Grafana Observability",
			Category:       "monitoring",
			Description:    "Dashboard analitik & visualisasi metrik real-time untuk Prometheus, Loki, dan database SQL.",
			Icon:           "ri-bar-chart-grouped-line",
			Image:          "grafana/grafana-oss:latest",
			DefaultPort:    3000,
			DefaultPorts:   []string{"3000:3000"},
			DefaultVolumes: []string{"./grafana_data:/var/lib/grafana"},
			VolumeMount:    "/var/lib/grafana",
			DefaultEnv: map[string]string{
				"GF_SECURITY_ADMIN_USER":     "admin",
				"GF_SECURITY_ADMIN_PASSWORD": "GrafanaStrongPass2026!",
			},
			EnvParams: []OneClickEnvParam{
				{Key: "GF_SECURITY_ADMIN_USER", Label: "Username Admin", Default: "admin", Type: "text"},
				{Key: "GF_SECURITY_ADMIN_PASSWORD", Label: "Password Admin", Default: "GrafanaStrongPass2026!", Type: "password"},
			},
			DockerComposeYAML: `services:
  grafana:
    image: grafana/grafana-oss:latest
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      GF_SECURITY_ADMIN_USER: admin
      GF_SECURITY_ADMIN_PASSWORD: GrafanaStrongPass2026!
    volumes:
      - ./grafana_data:/var/lib/grafana
`,
		},
		{
			ID:             "vaultwarden",
			Name:           "Vaultwarden (Bitwarden)",
			Category:       "tools",
			Description:    "Password manager ringan berbasis Rust yang kompatibel penuh dengan seluruh klien Bitwarden.",
			Icon:           "ri-shield-keyhole-line",
			Image:          "vaultwarden/server:latest",
			DefaultPort:    8088,
			DefaultPorts:   []string{"8088:80"},
			DefaultVolumes: []string{"./vw_data:/data"},
			VolumeMount:    "/data",
			DefaultEnv: map[string]string{
				"SIGNUPS_ALLOWED": "true",
			},
			EnvParams: []OneClickEnvParam{
				{Key: "SIGNUPS_ALLOWED", Label: "Izinkan Registrasi Baru (true/false)", Default: "true", Type: "text"},
				{Key: "DOMAIN", Label: "Domain Publik HTTPS", Default: "https://vault.example.com", Type: "text"},
			},
			DockerComposeYAML: `services:
  vaultwarden:
    image: vaultwarden/server:latest
    restart: unless-stopped
    ports:
      - "8088:80"
    environment:
      SIGNUPS_ALLOWED: "true"
      DOMAIN: https://vault.example.com
    volumes:
      - ./vw_data:/data
`,
		},
		{
			ID:             "gitea",
			Name:           "Gitea Git Server",
			Category:       "tools",
			Description:    "Server Git self-hosted yang sangat hemat RAM (cocok untuk ARM64/VPS) lengkap dengan CI Actions.",
			Icon:           "ri-git-branch-line",
			Image:          "gitea/gitea:latest",
			DefaultPort:    3002,
			DefaultPorts:   []string{"3002:3000", "2222:22"},
			DefaultVolumes: []string{"./gitea_data:/data"},
			VolumeMount:    "/data",
			DefaultEnv:     map[string]string{"USER_UID": "1000", "USER_GID": "1000"},
			EnvParams: []OneClickEnvParam{
				{Key: "USER_UID", Label: "UID Host", Default: "1000", Type: "text"},
				{Key: "USER_GID", Label: "GID Host", Default: "1000", Type: "text"},
			},
			DockerComposeYAML: `services:
  gitea:
    image: gitea/gitea:latest
    restart: unless-stopped
    ports:
      - "3002:3000"
      - "2222:22"
    environment:
      USER_UID: "1000"
      USER_GID: "1000"
    volumes:
      - ./gitea_data:/data
`,
		},
		{
			ID:             "postgres",
			Name:           "PostgreSQL 16",
			Category:       "database",
			Description:    "Database relasional open-source tangguh untuk produksi, analitik, dan aplikasi modern.",
			Icon:           "ri-database-2-line",
			Image:          "postgres:16-alpine",
			DefaultPort:    5432,
			DefaultPorts:   []string{"5432:5432"},
			DefaultVolumes: []string{"./pg_data:/var/lib/postgresql/data"},
			VolumeMount:    "/var/lib/postgresql/data",
			DefaultEnv: map[string]string{
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "PgStrongPass2026!",
				"POSTGRES_DB":       "appdb",
			},
			EnvParams: []OneClickEnvParam{
				{Key: "POSTGRES_DB", Label: "Database Awal", Default: "appdb", Type: "text"},
				{Key: "POSTGRES_USER", Label: "Superuser", Default: "postgres", Type: "text"},
				{Key: "POSTGRES_PASSWORD", Label: "Password Superuser", Default: "PgStrongPass2026!", Type: "password"},
			},
			DockerComposeYAML: `services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: PgStrongPass2026!
      POSTGRES_DB: appdb
    volumes:
      - ./pg_data:/var/lib/postgresql/data
`,
		},
		{
			ID:             "mysql",
			Name:           "MySQL 8.4 LTS",
			Category:       "database",
			Description:    "Database relasional MySQL 8.4 jangka panjang dengan performa InnoDB optimal.",
			Icon:           "ri-database-line",
			Image:          "mysql:8.4",
			DefaultPort:    3307,
			DefaultPorts:   []string{"3307:3306"},
			DefaultVolumes: []string{"./mysql_data:/var/lib/mysql"},
			VolumeMount:    "/var/lib/mysql",
			DefaultEnv: map[string]string{
				"MYSQL_ROOT_PASSWORD": "MysqlRootPass2026!",
				"MYSQL_DATABASE":      "appdb",
				"MYSQL_USER":          "appuser",
				"MYSQL_PASSWORD":      "MysqlUserPass2026!",
			},
			EnvParams: []OneClickEnvParam{
				{Key: "MYSQL_DATABASE", Label: "Nama Database", Default: "appdb", Type: "text"},
				{Key: "MYSQL_USER", Label: "Username Aplikasi", Default: "appuser", Type: "text"},
				{Key: "MYSQL_PASSWORD", Label: "Password Aplikasi", Default: "MysqlUserPass2026!", Type: "password"},
				{Key: "MYSQL_ROOT_PASSWORD", Label: "Password Root", Default: "MysqlRootPass2026!", Type: "password"},
			},
			DockerComposeYAML: `services:
  mysql:
    image: mysql:8.4
    restart: unless-stopped
    ports:
      - "3307:3306"
    environment:
      MYSQL_ROOT_PASSWORD: MysqlRootPass2026!
      MYSQL_DATABASE: appdb
      MYSQL_USER: appuser
      MYSQL_PASSWORD: MysqlUserPass2026!
    volumes:
      - ./mysql_data:/var/lib/mysql
`,
		},
		{
			ID:             "mariadb",
			Name:           "MariaDB 11 LTS",
			Category:       "database",
			Description:    "Drop-in replacement MySQL yang cepat, ringan, dan kompatibel luas untuk ekosistem PHP/Laravel.",
			Icon:           "ri-server-line",
			Image:          "mariadb:11",
			DefaultPort:    3308,
			DefaultPorts:   []string{"3308:3306"},
			DefaultVolumes: []string{"./mariadb_data:/var/lib/mysql"},
			VolumeMount:    "/var/lib/mysql",
			DefaultEnv: map[string]string{
				"MARIADB_ROOT_PASSWORD": "MariaRootPass2026!",
				"MARIADB_DATABASE":      "appdb",
				"MARIADB_USER":          "appuser",
				"MARIADB_PASSWORD":      "MariaUserPass2026!",
			},
			EnvParams: []OneClickEnvParam{
				{Key: "MARIADB_DATABASE", Label: "Nama Database", Default: "appdb", Type: "text"},
				{Key: "MARIADB_USER", Label: "Username", Default: "appuser", Type: "text"},
				{Key: "MARIADB_PASSWORD", Label: "Password User", Default: "MariaUserPass2026!", Type: "password"},
				{Key: "MARIADB_ROOT_PASSWORD", Label: "Password Root", Default: "MariaRootPass2026!", Type: "password"},
			},
			DockerComposeYAML: `services:
  mariadb:
    image: mariadb:11
    restart: unless-stopped
    ports:
      - "3308:3306"
    environment:
      MARIADB_ROOT_PASSWORD: MariaRootPass2026!
      MARIADB_DATABASE: appdb
      MARIADB_USER: appuser
      MARIADB_PASSWORD: MariaUserPass2026!
    volumes:
      - ./mariadb_data:/var/lib/mysql
`,
		},
		{
			ID:             "redis",
			Name:           "Redis 7 Cache & Queue",
			Category:       "database",
			Description:    "In-memory key-value store super cepat dengan persistensi AOF untuk session, cache, dan antrean.",
			Icon:           "ri-flashlight-line",
			Image:          "redis:7-alpine",
			DefaultPort:    6379,
			DefaultPorts:   []string{"6379:6379"},
			DefaultVolumes: []string{"./redis_data:/data"},
			VolumeMount:    "/data",
			DefaultEnv:     map[string]string{"REDIS_PASSWORD": "RedisStrongPass2026!"},
			EnvParams: []OneClickEnvParam{
				{Key: "REDIS_PASSWORD", Label: "Password Akses Redis", Default: "RedisStrongPass2026!", Type: "password"},
			},
			DockerComposeYAML: `services:
  redis:
    image: redis:7-alpine
    restart: unless-stopped
    command: redis-server --appendonly yes --requirepass RedisStrongPass2026!
    ports:
      - "6379:6379"
    volumes:
      - ./redis_data:/data
`,
		},
		{
			ID:             "mongodb",
			Name:           "MongoDB 7 Document DB",
			Category:       "database",
			Description:    "Database dokumen NoSQL berkinerja tinggi dengan skema JSON fleksibel untuk backend modern.",
			Icon:           "ri-leaf-line",
			Image:          "mongo:7",
			DefaultPort:    27017,
			DefaultPorts:   []string{"27017:27017"},
			DefaultVolumes: []string{"./mongo_data:/data/db"},
			VolumeMount:    "/data/db",
			DefaultEnv: map[string]string{
				"MONGO_INITDB_ROOT_USERNAME": "root",
				"MONGO_INITDB_ROOT_PASSWORD": "MongoStrongPass2026!",
			},
			EnvParams: []OneClickEnvParam{
				{Key: "MONGO_INITDB_ROOT_USERNAME", Label: "Root Username", Default: "root", Type: "text"},
				{Key: "MONGO_INITDB_ROOT_PASSWORD", Label: "Root Password", Default: "MongoStrongPass2026!", Type: "password"},
			},
			DockerComposeYAML: `services:
  mongo:
    image: mongo:7
    restart: unless-stopped
    ports:
      - "27017:27017"
    environment:
      MONGO_INITDB_ROOT_USERNAME: root
      MONGO_INITDB_ROOT_PASSWORD: MongoStrongPass2026!
    volumes:
      - ./mongo_data:/data/db
`,
		},
		{
			ID:             "caddy",
			Name:           "Caddy Reverse Proxy",
			Category:       "cms",
			Description:    "Web server & reverse proxy modern dengan penerbitan sertifikat TLS/HTTPS otomatis.",
			Icon:           "ri-global-line",
			Image:          "caddy:2-alpine",
			DefaultPort:    8082,
			DefaultPorts:   []string{"8082:80"},
			DefaultVolumes: []string{"./caddy_data:/data", "./caddy_site:/usr/share/caddy"},
			VolumeMount:    "/usr/share/caddy",
			DefaultEnv:     map[string]string{},
			EnvParams:      []OneClickEnvParam{},
			DockerComposeYAML: `services:
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "8082:80"
    volumes:
      - ./caddy_data:/data
      - ./caddy_site:/usr/share/caddy
`,
		},
		{
			ID:             "nginx",
			Name:           "Nginx Web Server",
			Category:       "cms",
			Description:    "Web server statis dan reverse proxy ringan berbasis Alpine Linux siap pakai.",
			Icon:           "ri-window-line",
			Image:          "nginx:alpine",
			DefaultPort:    8081,
			DefaultPorts:   []string{"8081:80"},
			DefaultVolumes: []string{"./html:/usr/share/nginx/html"},
			VolumeMount:    "/usr/share/nginx/html",
			DefaultEnv:     map[string]string{},
			EnvParams:      []OneClickEnvParam{},
			DockerComposeYAML: `services:
  nginx:
    image: nginx:alpine
    restart: unless-stopped
    ports:
      - "8081:80"
    volumes:
      - ./html:/usr/share/nginx/html
`,
		},
	}
}

// RenderTemplateCompose menerapkan kustomisasi port, volume, dan env secara aman terhadap YAML template
func RenderTemplateCompose(tpl OneClickTemplate, req DeployTemplateRequest) string {
	yamlContent := tpl.DockerComposeYAML

	// 1. Terapkan override port tunggal dari field req.Port (jika disediakan oleh UI modal)
	if req.Port > 0 && req.Port <= 65535 && len(tpl.DefaultPorts) > 0 {
		first := tpl.DefaultPorts[0]
		parts := strings.SplitN(first, ":", 2)
		if len(parts) == 2 {
			customMapping := fmt.Sprintf("%d:%s", req.Port, parts[1])
			if customMapping != first {
				yamlContent = strings.ReplaceAll(yamlContent, `"`+first+`"`, `"`+customMapping+`"`)
			}
		}
	}

	// 2. Terapkan override peta port eksplisit jika diberikan melalui req.Ports
	for origPort, customPort := range req.Ports {
		op := strings.TrimSpace(origPort)
		cp := strings.TrimSpace(customPort)
		if op != "" && cp != "" && op != cp && !strings.ContainsAny(cp, "\r\n\"'") {
			yamlContent = strings.ReplaceAll(yamlContent, `"`+op+`"`, `"`+cp+`"`)
		}
	}

	// 3. Terapkan override direktori volume host utama jika diubah pengguna
	if vp := strings.TrimSpace(req.VolumePath); vp != "" && tpl.VolumeMount != "" && !strings.ContainsAny(vp, "\r\n\"'") {
		for _, defVol := range tpl.DefaultVolumes {
			if strings.HasSuffix(defVol, ":"+tpl.VolumeMount) {
				yamlContent = strings.ReplaceAll(yamlContent, defVol, vp+":"+tpl.VolumeMount)
				break
			}
		}
	}

	// 4. Gabungkan req.Env dan req.EnvValues lalu terapkan ke nilai default pada YAML secara aman
	mergedEnv := make(map[string]string)
	for k, v := range req.Env {
		mergedEnv[k] = v
	}
	for k, v := range req.EnvValues {
		mergedEnv[k] = v
	}
	// Buat indeks nilai default dari DefaultEnv dan EnvParams
	defaultLookup := make(map[string]string)
	for k, v := range tpl.DefaultEnv {
		defaultLookup[k] = v
	}
	for _, p := range tpl.EnvParams {
		if _, ok := defaultLookup[p.Key]; !ok {
			defaultLookup[p.Key] = p.Default
		}
	}

	for k, customVal := range mergedEnv {
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(customVal)
		if key == "" || strings.ContainsAny(key+val, "\r\n") {
			continue
		}
		defVal, ok := defaultLookup[key]
		if !ok || defVal == "" || defVal == val {
			continue
		}
		yamlContent = strings.ReplaceAll(yamlContent, key+": "+defVal, key+": "+val)
		yamlContent = strings.ReplaceAll(yamlContent, key+`: "`+defVal+`"`, key+`: "`+val+`"`)
		// Khusus template seperti Redis yang menaruh password langsung di command flag
		if key == "REDIS_PASSWORD" {
			yamlContent = strings.ReplaceAll(yamlContent, "--requirepass "+defVal, "--requirepass "+val)
		}
		// Khusus WordPress yang menyalin nilai password/user/db ke kontainer db juga
		if tpl.ID == "wordpress" {
			switch key {
			case "WORDPRESS_DB_USER":
				yamlContent = strings.ReplaceAll(yamlContent, "MYSQL_USER: "+defVal, "MYSQL_USER: "+val)
			case "WORDPRESS_DB_PASSWORD":
				yamlContent = strings.ReplaceAll(yamlContent, "MYSQL_PASSWORD: "+defVal, "MYSQL_PASSWORD: "+val)
			case "WORDPRESS_DB_NAME":
				yamlContent = strings.ReplaceAll(yamlContent, "MYSQL_DATABASE: "+defVal, "MYSQL_DATABASE: "+val)
			}
		}
	}

	return yamlContent
}

// DeployTemplate men-deploy aplikasi 1-klik dari katalog template ke dalam proyek Compose terisolasi
func (m *Manager) DeployTemplate(ctx context.Context, req DeployTemplateRequest) error {
	templates := m.GetTemplates()
	var targetTpl *OneClickTemplate
	for i := range templates {
		if templates[i].ID == req.TemplateID {
			targetTpl = &templates[i]
			break
		}
	}
	if targetTpl == nil {
		return fmt.Errorf("template dengan ID %s tidak ditemukan", req.TemplateID)
	}

	projName := strings.TrimSpace(req.ProjectName)
	if projName == "" {
		projName = strings.TrimSpace(req.Name)
	}
	if projName == "" {
		projName = fmt.Sprintf("%s-%d", targetTpl.ID, time.Now().Unix()%10000)
	}
	if !IsValidProjectName(projName) {
		return fmt.Errorf("nama proyek hanya boleh menggunakan huruf, angka, tanda hubung (-), atau garis bawah (_)")
	}

	yamlContent := RenderTemplateCompose(*targetTpl, req)
	return m.ComposeUp(ctx, projName, yamlContent)
}
