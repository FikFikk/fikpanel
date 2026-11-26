package appstore

// AppPackage merepresentasikan satu paket server pada katalog resmi App Store FikPanel
type AppPackage struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Category         string   `json:"category"` // webserver, runtime, database, tools
	Version          string   `json:"version"`
	InstalledVersion string   `json:"installed_version,omitempty"`
	CandidateVersion string   `json:"candidate_version,omitempty"`
	UpdateAvailable  bool     `json:"update_available"`
	DiskSizeKB       int64    `json:"disk_size_kb,omitempty"`
	Description      string   `json:"description"`
	Icon             string   `json:"icon"`
	Installed        bool     `json:"installed"`
	Status           string   `json:"status"` // running, stopped, not_installed
	ServiceUnit      string   `json:"service_unit,omitempty"`
	ManagerRoute     string   `json:"manager_route,omitempty"`
	Critical         bool     `json:"critical,omitempty"`
	AptPackages      []string `json:"apt_packages"`

	// Metadata internal deteksi biner & dpkg (tidak diekspos ke JSON)
	PrimaryAptPkg string   `json:"-"`
	BinaryNames   []string `json:"-"`
	AltServices   []string `json:"-"`
}

// StackBundleComponent merangkum status satu aplikasi di dalam Stack Bundle
type StackBundleComponent struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Status    string `json:"status"`
	Version   string `json:"version"`
}

// StackBundle merepresentasikan paket bundel lingkungan produksi siap pasang 1-klik
type StackBundle struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Tagline        string                 `json:"tagline"`
	Description    string                 `json:"description"`
	Icon           string                 `json:"icon"`
	Badge          string                 `json:"badge"`
	AppIDs         []string               `json:"app_ids"`
	Components     []StackBundleComponent `json:"components"`
	InstalledCount int                    `json:"installed_count"`
	TotalCount     int                    `json:"total_count"`
	MissingAppIDs  []string               `json:"missing_app_ids"`
	Ready          bool                   `json:"ready"`
}

// GlobalPackage merepresentasikan satu paket dari repositori sistem Linux (APT)
type GlobalPackage struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
}

// UpgradablePackage merepresentasikan paket APT yang memiliki pembaruan versi di repositori
type UpgradablePackage struct {
	Name             string `json:"name"`
	CurrentVersion   string `json:"current_version"`
	CandidateVersion string `json:"candidate_version"`
	Architecture     string `json:"architecture,omitempty"`
}

// PackageDetail berisi metadata mendalam hasil inspeksi apt-cache show & dpkg-query
type PackageDetail struct {
	Name             string `json:"name"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installed_version,omitempty"`
	CandidateVersion string `json:"candidate_version,omitempty"`
	UpdateAvailable  bool   `json:"update_available"`
	Section          string `json:"section,omitempty"`
	Maintainer       string `json:"maintainer,omitempty"`
	Homepage         string `json:"homepage,omitempty"`
	InstalledSizeKB  int64  `json:"installed_size_kb,omitempty"`
	DownloadSize     string `json:"download_size,omitempty"`
	Depends          string `json:"depends,omitempty"`
	Description      string `json:"description"`
}

// InstallTask melacak status dan log real-time operasi paket APT
type InstallTask struct {
	AppID      string `json:"app_id"`
	Action     string `json:"action"` // install, uninstall, upgrade, apt-update, bundle
	Title      string `json:"title"`
	Status     string `json:"status"` // queued, running, success, failed
	Logs       string `json:"logs"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at,omitempty"`
}

// builtinCatalogDefs mendefinisikan daftar lengkap perangkat lunak server kurasi FikPanel
func builtinCatalogDefs() []AppPackage {
	return []AppPackage{
		// ── Web Server ───────────────────────────────────────────────────────
		{
			ID:            "nginx",
			Name:          "Nginx Web Server",
			Category:      "webserver",
			Version:       "APT Latest",
			Description:   "HTTP server & reverse proxy performa tinggi untuk virtual host, SSL, WAF, dan static files.",
			Icon:          "ri-server-line",
			ServiceUnit:   "nginx",
			ManagerRoute:  "#/website",
			Critical:      true,
			PrimaryAptPkg: "nginx",
			BinaryNames:   []string{"nginx"},
			AptPackages:   []string{"nginx"},
		},
		{
			ID:            "caddy",
			Name:          "Caddy Web Server",
			Category:      "webserver",
			Version:       "2.x",
			Description:   "Web server modern berbahasa Go dengan Auto-HTTPS ACME bawaan dan reverse proxy ringkas.",
			Icon:          "ri-global-line",
			ServiceUnit:   "caddy",
			ManagerRoute:  "#/services",
			PrimaryAptPkg: "caddy",
			BinaryNames:   []string{"caddy"},
			AptPackages:   []string{"caddy"},
		},
		{
			ID:            "apache2",
			Name:          "Apache HTTP Server",
			Category:      "webserver",
			Version:       "2.4",
			Description:   "Web server modular standar industri dengan dukungan berkas .htaccess dan modul rewrite.",
			Icon:          "ri-server-fill",
			ServiceUnit:   "apache2",
			ManagerRoute:  "#/services",
			PrimaryAptPkg: "apache2",
			BinaryNames:   []string{"apache2"},
			AptPackages:   []string{"apache2"},
		},

		// ── Runtime & Bahasa ─────────────────────────────────────────────────
		{
			ID:            "php8.5",
			Name:          "PHP 8.5 + FPM Suite",
			Category:      "runtime",
			Version:       "8.5",
			Description:   "Runtime PHP 8.5 dengan FastCGI Process Manager serta ekstensi inti MySQL, GD, cURL, Mbstring, XML, Zip.",
			Icon:          "ri-code-s-slash-line",
			ServiceUnit:   "php8.5-fpm",
			ManagerRoute:  "#/php",
			PrimaryAptPkg: "php8.5-fpm",
			BinaryNames:   []string{"php8.5"},
			AptPackages:   []string{"php8.5-fpm", "php8.5-mysql", "php8.5-cli", "php8.5-curl", "php8.5-mbstring", "php8.5-gd", "php8.5-xml", "php8.5-zip"},
		},
		{
			ID:            "php8.4",
			Name:          "PHP 8.4 + FPM Suite",
			Category:      "runtime",
			Version:       "8.4",
			Description:   "Runtime PHP 8.4 dengan property hooks, asymmetric visibility, FPM, dan ekstensi inti Laravel/Filament.",
			Icon:          "ri-code-s-slash-line",
			ServiceUnit:   "php8.4-fpm",
			ManagerRoute:  "#/php",
			PrimaryAptPkg: "php8.4-fpm",
			BinaryNames:   []string{"php8.4"},
			AptPackages:   []string{"php8.4-fpm", "php8.4-mysql", "php8.4-cli", "php8.4-curl", "php8.4-mbstring", "php8.4-gd", "php8.4-xml", "php8.4-zip"},
		},
		{
			ID:            "php8.3",
			Name:          "PHP 8.3 + FPM Suite",
			Category:      "runtime",
			Version:       "8.3",
			Description:   "Runtime PHP 8.3 LTS produksi lengkap dengan FPM, ekstensi MySQL, GD, cURL, Mbstring, XML, dan Zip.",
			Icon:          "ri-code-s-slash-line",
			ServiceUnit:   "php8.3-fpm",
			ManagerRoute:  "#/php",
			PrimaryAptPkg: "php8.3-fpm",
			BinaryNames:   []string{"php8.3"},
			AptPackages:   []string{"php8.3-fpm", "php8.3-mysql", "php8.3-cli", "php8.3-curl", "php8.3-mbstring", "php8.3-gd", "php8.3-xml", "php8.3-zip"},
		},
		{
			ID:            "php8.2",
			Name:          "PHP 8.2 + FPM Suite",
			Category:      "runtime",
			Version:       "8.2",
			Description:   "Runtime PHP 8.2 stabil untuk kompatibilitas tinggi Laravel, CodeIgniter, dan WordPress.",
			Icon:          "ri-code-s-slash-line",
			ServiceUnit:   "php8.2-fpm",
			ManagerRoute:  "#/php",
			PrimaryAptPkg: "php8.2-fpm",
			BinaryNames:   []string{"php8.2"},
			AptPackages:   []string{"php8.2-fpm", "php8.2-mysql", "php8.2-cli", "php8.2-curl", "php8.2-mbstring", "php8.2-gd", "php8.2-xml", "php8.2-zip"},
		},
		{
			ID:            "nodejs",
			Name:          "Node.js & NPM",
			Category:      "runtime",
			Version:       "LTS",
			Description:   "Runtime JavaScript untuk Next.js, Nuxt, SvelteKit, Express, build frontend Vite, dan proses PM2.",
			Icon:          "ri-nodejs-line",
			ManagerRoute:  "#/node",
			PrimaryAptPkg: "nodejs",
			BinaryNames:   []string{"node"},
			AptPackages:   []string{"nodejs", "npm"},
		},
		{
			ID:            "composer",
			Name:          "Composer PHP Manager",
			Category:      "runtime",
			Version:       "2.x",
			Description:   "Manajer paket resmi PHP untuk membangun dan memelihara proyek Laravel, Filament, dan Symfony.",
			Icon:          "ri-terminal-box-line",
			ManagerRoute:  "#/php",
			PrimaryAptPkg: "composer",
			BinaryNames:   []string{"composer"},
			AptPackages:   []string{"composer"},
		},
		{
			ID:            "golang",
			Name:          "Go (Golang) Toolchain",
			Category:      "runtime",
			Version:       "1.2x",
			Description:   "Kompiler dan toolchain bahasa Go untuk membangun backend API, worker, dan layanan biner statis.",
			Icon:          "ri-braces-line",
			PrimaryAptPkg: "golang-go",
			BinaryNames:   []string{"go"},
			AptPackages:   []string{"golang-go"},
		},
		{
			ID:            "python3",
			Name:          "Python 3 + Pip & Venv",
			Category:      "runtime",
			Version:       "3.x",
			Description:   "Runtime Python 3 lengkap dengan pip dan virtual environment untuk Django, FastAPI, dan skrip otomasi.",
			Icon:          "ri-code-box-line",
			PrimaryAptPkg: "python3",
			BinaryNames:   []string{"python3"},
			AptPackages:   []string{"python3", "python3-pip", "python3-venv"},
		},

		// ── Database & Cache ─────────────────────────────────────────────────
		{
			ID:            "mariadb",
			Name:          "MariaDB / MySQL Server",
			Category:      "database",
			Version:       "10.11+",
			Description:   "Server database relasional open-source drop-in replacement untuk MySQL dengan performa tinggi.",
			Icon:          "ri-database-2-line",
			ServiceUnit:   "mariadb",
			AltServices:   []string{"mysql"},
			ManagerRoute:  "#/databases",
			Critical:      true,
			PrimaryAptPkg: "mariadb-server",
			BinaryNames:   []string{"mariadb", "mysql"},
			AptPackages:   []string{"mariadb-server", "mariadb-client"},
		},
		{
			ID:            "postgresql",
			Name:          "PostgreSQL Database",
			Category:      "database",
			Version:       "16+",
			Description:   "Sistem basis data relasional objek canggih dengan dukungan JSONB, indeks GIN, dan konkurensi tinggi.",
			Icon:          "ri-database-line",
			ServiceUnit:   "postgresql",
			ManagerRoute:  "#/databases",
			PrimaryAptPkg: "postgresql",
			BinaryNames:   []string{"psql"},
			AptPackages:   []string{"postgresql", "postgresql-contrib"},
		},
		{
			ID:            "redis",
			Name:          "Redis In-Memory Cache",
			Category:      "database",
			Version:       "7.x",
			Description:   "Penyimpanan key-value dalam memori super cepat untuk cache sesi, antrean Laravel Horizon, dan pub/sub.",
			Icon:          "ri-flashlight-line",
			ServiceUnit:   "redis-server",
			AltServices:   []string{"redis"},
			ManagerRoute:  "#/databases",
			PrimaryAptPkg: "redis-server",
			BinaryNames:   []string{"redis-server"},
			AptPackages:   []string{"redis-server"},
		},
		{
			ID:            "memcached",
			Name:          "Memcached Object Cache",
			Category:      "database",
			Version:       "1.6+",
			Description:   "Cache objek memori terdistribusi super ringan untuk mempercepat query WordPress dan PHP.",
			Icon:          "ri-hard-drive-3-line",
			ServiceUnit:   "memcached",
			ManagerRoute:  "#/services",
			PrimaryAptPkg: "memcached",
			BinaryNames:   []string{"memcached"},
			AptPackages:   []string{"memcached"},
		},
		{
			ID:            "sqlite3",
			Name:          "SQLite 3 CLI Engine",
			Category:      "database",
			Version:       "3.x",
			Description:   "Utilitas CLI database SQLite embedded untuk inspeksi, migrasi, dan cadangan ringan tanpa daemon.",
			Icon:          "ri-file-list-3-line",
			PrimaryAptPkg: "sqlite3",
			BinaryNames:   []string{"sqlite3"},
			AptPackages:   []string{"sqlite3"},
		},

		// ── Alat, Tunnel & Keamanan ──────────────────────────────────────────
		{
			ID:            "docker",
			Name:          "Docker CE Engine",
			Category:      "tools",
			Version:       "26.x+",
			Description:   "Platform kontainerisasi lengkap dengan Docker Compose untuk menjalankan aplikasi 1-klik terisolasi.",
			Icon:          "ri-box-3-line",
			ServiceUnit:   "docker",
			ManagerRoute:  "#/docker",
			PrimaryAptPkg: "docker.io",
			BinaryNames:   []string{"docker"},
			AptPackages:   []string{"docker.io", "docker-compose-v2"},
		},
		{
			ID:            "cloudflared",
			Name:          "Cloudflare Tunnel (cloudflared)",
			Category:      "tools",
			Version:       "Latest",
			Description:   "Daemon tunnel Zero Trust Cloudflare untuk mengekspos layanan lokal tanpa membuka port publik VPS.",
			Icon:          "ri-cloud-line",
			ServiceUnit:   "cloudflared",
			ManagerRoute:  "#/services",
			PrimaryAptPkg: "cloudflared",
			BinaryNames:   []string{"cloudflared"},
			AptPackages:   []string{"cloudflared"},
		},
		{
			ID:            "certbot",
			Name:          "Certbot Let's Encrypt",
			Category:      "tools",
			Version:       "2.x",
			Description:   "CLI ACME untuk penerbitan dan pembaruan otomatis sertifikat SSL/TLS Let's Encrypt.",
			Icon:          "ri-lock-password-line",
			ManagerRoute:  "#/website",
			PrimaryAptPkg: "certbot",
			BinaryNames:   []string{"certbot"},
			AptPackages:   []string{"certbot", "python3-certbot-nginx"},
		},
		{
			ID:            "fail2ban",
			Name:          "Fail2Ban Intrusion Guard",
			Category:      "tools",
			Version:       "1.x",
			Description:   "Sistem pencegahan intrusi otomatis yang memblokir alamat IP penyerang brute-force SSH dan Nginx.",
			Icon:          "ri-alarm-warning-line",
			ServiceUnit:   "fail2ban",
			ManagerRoute:  "#/security",
			PrimaryAptPkg: "fail2ban",
			BinaryNames:   []string{"fail2ban-client"},
			AptPackages:   []string{"fail2ban"},
		},
		{
			ID:            "ufw",
			Name:          "UFW Firewall Shield",
			Category:      "tools",
			Version:       "0.36+",
			Description:   "Uncomplicated Firewall untuk manajemen aturan port jaringan masuk/keluar server Linux.",
			Icon:          "ri-shield-check-line",
			ServiceUnit:   "ufw",
			ManagerRoute:  "#/security",
			Critical:      true,
			PrimaryAptPkg: "ufw",
			BinaryNames:   []string{"ufw"},
			AptPackages:   []string{"ufw"},
		},
		{
			ID:            "supervisor",
			Name:          "Supervisor Process Control",
			Category:      "tools",
			Version:       "4.x",
			Description:   "Pengelola daemon & worker latar belakang untuk menjaga Laravel Queue dan skrip tetap aktif 24/7.",
			Icon:          "ri-cpu-line",
			ServiceUnit:   "supervisor",
			ManagerRoute:  "#/services",
			PrimaryAptPkg: "supervisor",
			BinaryNames:   []string{"supervisord"},
			AptPackages:   []string{"supervisor"},
		},
		{
			ID:            "git",
			Name:          "Git Version Control",
			Category:      "tools",
			Version:       "2.x",
			Description:   "Sistem kendali versi untuk auto-deploy GitHub/GitLab, git pull situs, dan rollback rilis.",
			Icon:          "ri-git-branch-line",
			ManagerRoute:  "#/website",
			Critical:      true,
			PrimaryAptPkg: "git",
			BinaryNames:   []string{"git"},
			AptPackages:   []string{"git"},
		},
		{
			ID:            "wireguard",
			Name:          "WireGuard VPN",
			Category:      "tools",
			Version:       "1.x",
			Description:   "VPN kernel modern berkecepatan tinggi dengan enkripsi mutakhir untuk jaringan privat antar-server.",
			Icon:          "ri-vpn-line",
			ManagerRoute:  "#/security",
			PrimaryAptPkg: "wireguard-tools",
			BinaryNames:   []string{"wg"},
			AptPackages:   []string{"wireguard", "wireguard-tools"},
		},
		{
			ID:            "rclone",
			Name:          "Rclone Cloud Sync",
			Category:      "tools",
			Version:       "1.x",
			Description:   "Alat sinkronisasi dan cadangan cloud multi-provider (S3, Cloudflare R2, Google Drive, SFTP).",
			Icon:          "ri-upload-cloud-2-line",
			ManagerRoute:  "#/databases",
			PrimaryAptPkg: "rclone",
			BinaryNames:   []string{"rclone"},
			AptPackages:   []string{"rclone"},
		},
		{
			ID:            "ffmpeg",
			Name:          "FFmpeg Media Suite",
			Category:      "tools",
			Version:       "6.x+",
			Description:   "Mesin pemrosesan video, audio, thumbnail, dan transcoding streaming untuk aplikasi media.",
			Icon:          "ri-film-line",
			PrimaryAptPkg: "ffmpeg",
			BinaryNames:   []string{"ffmpeg"},
			AptPackages:   []string{"ffmpeg"},
		},
		{
			ID:            "imagemagick",
			Name:          "ImageMagick Processor",
			Category:      "tools",
			Version:       "6.x+",
			Description:   "Toolkit manipulasi gambar, konversi WebP/AVIF, dan pemrosesan PDF untuk aplikasi web.",
			Icon:          "ri-image-edit-line",
			PrimaryAptPkg: "imagemagick",
			BinaryNames:   []string{"convert", "magick"},
			AptPackages:   []string{"imagemagick"},
		},
	}
}

// builtinStackBundleDefs mendefinisikan bundel lingkungan produksi 1-klik
func builtinStackBundleDefs() []StackBundle {
	return []StackBundle{
		{
			ID:          "bundle-laravel",
			Name:        "Laravel & Filament Production",
			Tagline:     "Stack penuh siap deploy Laravel 11/12 + Queue + Cache",
			Description: "Memasang Nginx, PHP 8.3 FPM + ekstensi inti, MariaDB, Redis, Composer, Supervisor untuk queue worker, dan Git.",
			Icon:        "ri-rocket-2-line",
			Badge:       "Populer",
			AppIDs:      []string{"nginx", "php8.3", "mariadb", "redis", "composer", "supervisor", "git"},
		},
		{
			ID:          "bundle-nextjs",
			Name:        "Next.js / Nuxt / Node.js Stack",
			Tagline:     "Reverse proxy Nginx + Node.js runtime + Redis Cache",
			Description: "Lingkungan ringan untuk aplikasi SSR JavaScript/TypeScript, API Express/Fastify, dan manajemen proses PM2.",
			Icon:        "ri-nodejs-line",
			Badge:       "Modern Web",
			AppIDs:      []string{"nginx", "nodejs", "redis", "git"},
		},
		{
			ID:          "bundle-golang",
			Name:        "Go + PostgreSQL Microservices",
			Tagline:     "Backend performa tinggi Golang + Postgres + Redis",
			Description: "Kombinasi toolchain Go, PostgreSQL, Redis, Nginx reverse proxy, dan Git untuk layanan API berskala besar.",
			Icon:        "ri-braces-line",
			Badge:       "High Perf",
			AppIDs:      []string{"nginx", "golang", "postgresql", "redis", "git"},
		},
		{
			ID:          "bundle-wordpress",
			Name:        "WordPress High-Speed LEMP",
			Tagline:     "Nginx + PHP 8.3 FPM + MariaDB + Redis + Certbot",
			Description: "Stack LEMP optimal untuk WordPress berkecepatan tinggi lengkap dengan object cache Redis dan SSL Certbot.",
			Icon:        "ri-wordpress-fill",
			Badge:       "CMS Ready",
			AppIDs:      []string{"nginx", "php8.3", "mariadb", "redis", "certbot"},
		},
		{
			ID:          "bundle-devops",
			Name:        "Container, Tunnel & Hardening",
			Tagline:     "Docker CE + Cloudflare Tunnel + Fail2Ban + UFW",
			Description: "Paket operasional modern untuk menjalankan aplikasi Docker 1-klik dengan akses Zero Trust dan proteksi intrusi.",
			Icon:        "ri-shield-star-line",
			Badge:       "Ops & Edge",
			AppIDs:      []string{"docker", "cloudflared", "fail2ban", "ufw", "git"},
		},
	}
}
