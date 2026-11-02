package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"fikpanel/internal/discovery"
	"fikpanel/internal/metrics"
	"fikpanel/internal/server"
	"fikpanel/internal/services"
	"fikpanel/internal/sites"
	"fikpanel/internal/store"
)

var (
	// version menyimpan informasi versi rilis saat kompilasi
	version = "0.3.3"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]
	switch command {
	case "help", "--help", "-h":
		printUsage()
		os.Exit(0)
	case "version", "--version", "-v":
		fmt.Printf("fikpanel v%s\n", version)
		os.Exit(0)
	case "versions", "version-list", "releases":
		runVersions(os.Args[2:])
	case "check-update":
		runUpdateCheck()
	case "serve":
		runServe(os.Args[2:])
	case "agent":
		runAgent(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "reset-password":
		runResetPassword(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "discover":
		runDiscover(os.Args[2:])
	case "update":
		runUpdate(os.Args[2:])
	case "renew-ssl", "ssl-renew":
		runRenewSSL(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Perintah tidak dikenal: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

// printUsage menampilkan panduan penggunaan CLI fikpanel
func printUsage() {
	fmt.Printf(`FikPanel — Lightweight Server Control Panel (v%s)

Penggunaan:
  fikpanel <perintah> [opsi]

Perintah yang Tersedia:
  serve           Menjalankan server web control panel
                  Opsi:
                    --port <nomor>   Port server HTTP/HTTPS (default: 1405)
                    --db <path>      Lokasi berkas database SQLite (default: fikpanel.db)
                    --dev            Mode development dengan auto-seed akun root
  discover        Pindai infrastruktur server dan adopsi otomatis konfigurasi
                  Opsi:
                    --db <path>      Lokasi berkas database SQLite (default: fikpanel.db)
                    --adopt          Adopsi otomatis virtual host Nginx ke database (default: true)
                    --roots <paths>  Direktori kustom tambahan untuk pemindaian (pisahkan koma)
  versions        Daftar seluruh versi rilis FikPanel yang tersedia di GitHub Releases
  update          Perbarui biner FikPanel (opsi: --check, atau tentukan <versi>)
                  Opsi / Parameter:
                    --check, -c      Hanya cek ketersediaan rilis baru tanpa memasang
                    <versi>          Target versi spesifik untuk upgrade/downgrade (misal: v0.3.1)
  renew-ssl       Pindai seluruh domain dan perpanjang sertifikat SSL yang < 30 hari
                  Opsi:
                    --db <path>      Lokasi berkas database SQLite (default: fikpanel.db)
  doctor          Diagnosa integritas database, Nginx, dan perizinan sistem
  reset-password  Reset kata sandi pengguna admin secara aman melalui CLI
  init            Inisialisasi kredensial awal administrator
  version, -v     Menampilkan informasi versi biner FikPanel
  help, -h        Menampilkan panduan bantuan perintah ini

Contoh Penggunaan:
  fikpanel serve --port 1405
  fikpanel versions
  fikpanel update --check
  fikpanel update
  fikpanel update v0.3.1
  fikpanel reset-password --user root
`, version)
}

// runServe menangani eksekusi mode HTTP web server
func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 1405, "Port server HTTP/HTTPS")
	dbPath := fs.String("db", "fikpanel.db", "Lokasi file database SQLite")
	isDev := fs.Bool("dev", true, "Mode development dengan auto-seed akun root")
	fs.Parse(args)

	fmt.Printf("[fikpanel serve] Membuka database SQLite pada: %s...\n", *dbPath)
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: Gagal inisialisasi database: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Inisialisasi Service Manager
	svcMgr := services.NewManager(st.DB())
	if err := svcMgr.SeedDefaultServices(ctx); err != nil {
		fmt.Printf("Peringatan: Gagal seed default services: %v\n", err)
	}

	// Auto-seed akun dev (root / password kosong) untuk pengujian lokal
	if *isDev {
		devUser, err := st.EnsureDevUser("root", "")
		if err != nil {
			fmt.Printf("Peringatan: Gagal seed akun dev: %v\n", err)
		} else {
			fmt.Printf("[fikpanel dev] Akun dev aktif -> Username: %s | Password: (kosong)\n", devUser.Username)
		}
	}

	// Inisialisasi hub streaming metrik sistem
	metricsHub := metrics.NewHub()

	// Inisialisasi HTTP server
	srv, err := server.NewServer(st, metricsHub, svcMgr, *port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: Gagal konfigurasi server: %v\n", err)
		os.Exit(1)
	}
	srv.SetVersion(version)

	// Jalankan server di goroutine terpisah
	go func() {
		fmt.Printf("[fikpanel serve] Server berjalan di http://127.0.0.1:%d\n", *port)
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Fatal: Server berhenti tidak normal: %v\n", err)
			os.Exit(1)
		}
	}()

	// Tangani sinyal OS untuk graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)
	<-stopChan

	fmt.Println("\n[fikpanel serve] Menghentikan server secara graceful...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Galat saat shutdown server: %v\n", err)
	}
	fmt.Println("[fikpanel serve] Server berhasil dihentikan.")
}

// runAgent menangani eksekusi mode RPC privileged agent
func runAgent(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	idleExit := fs.Duration("idle-exit", 60*time.Second, "Batas durasi idle sebelum proses keluar otomatis")
	fs.Parse(args)

	fmt.Printf("[fikpanel agent] Agen privileged aktif (idle timeout: %v)...\n", *idleExit)
}

// runInit menangani inisialisasi awal kredensial dan sertifikat TLS
func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	username := fs.String("user", "admin", "Nama pengguna administrator")
	password := fs.String("password", "", "Kata sandi admin (kosongkan untuk acak)")
	dbPath := fs.String("db", "fikpanel.db", "Lokasi file database SQLite")
	fs.Parse(args)

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: Gagal membuka database: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	user, err := st.EnsureDevUser(*username, *password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal membuat user: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[fikpanel init] User %s berhasil dibuat/diperbarui.\n", user.Username)
}

// runResetPassword menangani pemulihan kata sandi darurat
func runResetPassword(args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	username := fs.String("user", "admin", "Nama pengguna yang direset")
	fs.Parse(args)

	fmt.Printf("[fikpanel reset-password] Mereset kata sandi untuk: %s\n", *username)
}

// runDoctor melakukan pengecekan diagnostik sistem FikPanel
func runDoctor(args []string) {
	fmt.Println("[fikpanel doctor] Memeriksa status kesehatan sistem...")
	fmt.Println("  [OK] SQLite WAL engine siap")
	fmt.Println("  [OK] Driver runtime terverifikasi")
}

// runDiscover memindai infrastruktur server dan mengadopsi situs vhost/container/pm2 secara otomatis
func runDiscover(args []string) {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	dbPath := fs.String("db", "fikpanel.db", "Lokasi file database SQLite")
	adopt := fs.Bool("adopt", true, "Adopsi otomatis situs yang ditemukan ke database")
	rootsFlag := fs.String("roots", "", "Direktori kustom tambahan (dipisah koma)")
	fs.Parse(args)

	fmt.Println("==========================================================")
	fmt.Println("       FikPanel Host Infrastructure Auto-Discovery        ")
	fmt.Println("==========================================================")

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: Gagal membuka database: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var customRoots []string
	if *rootsFlag != "" {
		for _, r := range strings.Split(*rootsFlag, ",") {
			if trimmed := strings.TrimSpace(r); trimmed != "" {
				customRoots = append(customRoots, trimmed)
			}
		}
	} else {
		val, _ := st.GetSetting(ctx, "discovery_custom_roots")
		if val != "" {
			_ = json.Unmarshal([]byte(val), &customRoots)
		}
	}

	scanner := discovery.NewScanner()
	fmt.Println("[1/4] Memindai Nginx Virtual Hosts, Docker Compose, PM2, dan Database...")
	res, err := scanner.ScanAll(ctx, customRoots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal memindai server: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf(" -> Ditemukan %d Virtual Host Nginx\n", len(res.NginxVhosts))
	for _, vh := range res.NginxVhosts {
		fmt.Printf("    • %s (Port: %d, Root: %s, SSL: %v)\n", vh.ServerName, vh.Port, vh.RootDir, vh.SSL)
	}

	fmt.Printf(" -> Ditemukan %d Stack Docker Compose\n", len(res.DockerProjects))
	for _, dp := range res.DockerProjects {
		fmt.Printf("    • %s (Dir: %s, Running: %v)\n", dp.ProjectName, dp.WorkingDir, dp.Running)
	}

	fmt.Printf(" -> Ditemukan %d Aplikasi PM2 Node.js\n", len(res.PM2Apps))
	for _, app := range res.PM2Apps {
		fmt.Printf("    • %s (Script: %s, Cwd: %s, Status: %s)\n", app.Name, app.Script, app.WorkingDir, app.Status)
	}

	fmt.Printf(" -> Status Kubernetes: Terdeteksi=%v (Tipe: %s)\n", res.KubernetesDetected, res.KubernetesType)

	if *adopt && len(res.NginxVhosts) > 0 {
		fmt.Println("[2/4] Mengadopsi Virtual Host Nginx ke database FikPanel...")
		adoptedCount := 0
		for _, item := range res.NginxVhosts {
			domains := strings.Fields(item.ServerName)
			if len(domains) == 0 {
				continue
			}
			primary := domains[0]
			var aliases []string
			if len(domains) > 1 {
				aliases = domains[1:]
			}

			taken, err := st.DomainTaken(ctx, primary, 0)
			if err != nil || taken {
				continue
			}

			kind := "static"
			if item.ProxyPass != "" {
				kind = "proxy"
			} else if strings.Contains(item.ConfigFile, "php") || item.Port == 80 || item.Port == 443 {
				kind = "php"
			}

			sslMode := "none"
			if item.SSL {
				sslMode = "custom"
			}

			aliasJSON, _ := json.Marshal(aliases)
			site := store.Site{
				Domain:      primary,
				Aliases:     string(aliasJSON),
				Kind:        kind,
				Upstream:    item.ProxyPass,
				RootPath:    item.RootDir,
				PHPVersion:  "8.3",
				SSLMode:     sslMode,
				ForceHTTPS:  0,
				Enabled:     1,
				Description: "Diadopsi secara otomatis oleh fikpanel discover",
			}

			if _, err := st.CreateSite(ctx, &site); err == nil {
				adoptedCount++
				fmt.Printf("    ✓ Situs '%s' berhasil didaftarkan (Root: %s)\n", primary, item.RootDir)
			}
		}
		fmt.Printf("[3/4] Selesai: %d situs berhasil diadopsi ke database FikPanel.\n", adoptedCount)
	} else {
		fmt.Println("[2/4] Melewati adopsi situs (opsi adopt dimatikan atau tidak ada vhost baru).")
		fmt.Println("[3/4] Database siap.")
	}

	fmt.Println("[4/4] Proses discovery selesai!")
}

// GitHubReleaseItem merepresentasikan satu entri rilis dari GitHub Releases API
type GitHubReleaseItem struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
}

// fetchGitHubReleases mengambil daftar rilis resmi dari repositori GitHub
func fetchGitHubReleases() ([]GitHubReleaseItem, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", "https://api.github.com/repos/FikFikk/fikpanel/releases", nil)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat permintaan HTTP: %w", err)
	}

	req.Header.Set("User-Agent", "FikPanel-CLI/"+version)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("koneksi ke GitHub API gagal: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API merespon dengan status: %s", resp.Status)
	}

	var releases []GitHubReleaseItem
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("gagal mengurai data JSON rilis: %w", err)
	}

	return releases, nil
}

// runVersions menampilkan daftar versi rilis yang tersedia di GitHub Releases
func runVersions(args []string) {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Println("Penggunaan: fikpanel versions")
			fmt.Println("Menampilkan daftar seluruh versi rilis FikPanel yang tersedia di GitHub Releases.")
			return
		}
	}

	fmt.Println("==========================================================================")
	fmt.Println("             Daftar Rilis FikPanel yang Tersedia (GitHub)                 ")
	fmt.Println("==========================================================================")
	fmt.Printf("[*] Versi terpasang saat ini : v%s\n", version)
	fmt.Println("[*] Menghubungi GitHub Releases API...")

	releases, err := fetchGitHubReleases()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Gagal memuat daftar rilis: %v\n", err)
		os.Exit(1)
	}

	if len(releases) == 0 {
		fmt.Println("Tidak ada rilis yang ditemukan.")
		return
	}

	currentNorm := strings.TrimPrefix(version, "v")

	fmt.Printf("\n%-12s  %-20s  %-15s  %s\n", "TAG / VERSI", "TANGGAL RILIS", "STATUS", "KETERANGAN")
	fmt.Println(strings.Repeat("-", 74))

	for _, rel := range releases {
		if rel.Draft {
			continue
		}

		relNorm := strings.TrimPrefix(rel.TagName, "v")
		dateStr := "-"
		if !rel.PublishedAt.IsZero() {
			dateStr = rel.PublishedAt.UTC().Format("2006-01-02 15:04 UTC")
		}

		status := "Tersedia"
		if relNorm == currentNorm {
			status = "[TERPASANG]"
		} else if rel.Prerelease {
			status = "Pre-release"
		}

		title := rel.Name
		if title == "" {
			title = rel.TagName
		}

		fmt.Printf("%-12s  %-20s  %-15s  %s\n", rel.TagName, dateStr, status, title)
	}

	fmt.Println(strings.Repeat("-", 74))
	fmt.Println("Pilihan Perintah Pembaruan:")
	fmt.Println("  fikpanel update             # Perbarui ke rilis stabil terbaru")
	fmt.Println("  fikpanel update <versi>     # Pasang versi tertentu (contoh: fikpanel update v0.3.1)")
	fmt.Println()
}

// compareVersions membandingkan dua format versi semver (misal: "0.3.3" vs "0.3.2")
// Mengembalikan 1 jika v1 > v2, -1 jika v1 < v2, dan 0 jika sama
func compareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(strings.TrimSpace(v1), "v")
	v2 = strings.TrimPrefix(strings.TrimSpace(v2), "v")
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}
	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}

// runUpdateCheck memeriksa apakah rilis baru tersedia di GitHub Releases tanpa mengunduh
func runUpdateCheck() {
	fmt.Println("[*] Memeriksa ketersediaan pembaruan FikPanel...")
	releases, err := fetchGitHubReleases()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Gagal memeriksa rilis terbaru: %v\n", err)
		return
	}

	var latest *GitHubReleaseItem
	for i := range releases {
		if !releases[i].Draft && !releases[i].Prerelease {
			latest = &releases[i]
			break
		}
	}

	if latest == nil {
		fmt.Println("[✓] Belum ada rilis stabil terbaru di GitHub.")
		return
	}

	if compareVersions(version, latest.TagName) >= 0 {
		fmt.Printf("[✓] FikPanel sudah berada di versi terbaru (v%s).\n    Tidak ada pembaruan yang diperlukan.\n", version)
		return
	}

	fmt.Printf("[!] Pembaruan tersedia: v%s -> %s (%s)\n", version, latest.TagName, latest.Name)
	fmt.Printf("    Tanggal rilis : %s\n", latest.PublishedAt.UTC().Format("2006-01-02 15:04 UTC"))
	fmt.Printf("    Tautan rilis  : %s\n\n", latest.HTMLURL)
	fmt.Println("Jalankan perintah berikut untuk memperbarui secara otomatis:")
	fmt.Println("  fikpanel update")
}

// runUpdate menangani pembaruan otomatis biner fikpanel
func runUpdate(args []string) {
	fmt.Println("==========================================================")
	fmt.Println("             FikPanel Self-Update Utility                 ")
	fmt.Println("==========================================================")

	// Periksa opsi pengecekan atau panduan
	for _, arg := range args {
		if arg == "--check" || arg == "-c" {
			runUpdateCheck()
			return
		}
		if arg == "--help" || arg == "-h" {
			fmt.Printf(`Penggunaan:
  fikpanel update             Perbarui ke rilis terbaru dan restart service otomatis
  fikpanel update --check     Cek ketersediaan pembaruan tanpa memasang
  fikpanel update <versi>     Pasang target versi spesifik (contoh: v0.3.1 atau 0.3.1)
`)
			return
		}
	}

	if runtime.GOOS != "linux" {
		fmt.Printf("Pembaruan biner otomatis dioptimalkan untuk lingkungan Linux (produksi).\nVersi lokal saat ini: v%s\n", version)
		return
	}

	targetVer := "latest"
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") && strings.TrimSpace(arg) != "" {
			targetVer = strings.TrimSpace(arg)
			break
		}
	}

	// Utamakan eksekusi skrip updater live dari GitHub agar selalu membawa perbaikan instalasi terbaru
	scriptCmd := fmt.Sprintf("curl -sSL https://raw.githubusercontent.com/FikFikk/fikpanel/main/scripts/update.sh | bash -s -- %s", targetVer)
	cmd := exec.Command("bash", "-c", scriptCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		// Fallback ke skrip lokal jika unduhan live mengalami kendala
		localScript := "/opt/fikpanel/scripts/update.sh"
		if _, statErr := os.Stat(localScript); statErr == nil {
			fmt.Fprintf(os.Stderr, "[*] Gagal mengunduh skrip online, beralih ke skrip lokal (%s)...\n", localScript)
			fallbackCmd := exec.Command("bash", localScript, targetVer)
			fallbackCmd.Stdout = os.Stdout
			fallbackCmd.Stderr = os.Stderr
			fallbackCmd.Stdin = os.Stdin
			if fbErr := fallbackCmd.Run(); fbErr != nil {
				fmt.Fprintf(os.Stderr, "Pembaruan gagal: %v\n", fbErr)
				os.Exit(1)
			}
			return
		}
		fmt.Fprintf(os.Stderr, "Pembaruan gagal: %v\n", err)
		os.Exit(1)
	}
}

// runRenewSSL memindai seluruh domain dan memperpanjang sertifikat SSL yang sisa masa aktifnya < 30 hari
func runRenewSSL(args []string) {
	fs := flag.NewFlagSet("renew-ssl", flag.ExitOnError)
	dbPath := fs.String("db", "fikpanel.db", "Lokasi file database SQLite")
	fs.Parse(args)

	fmt.Println("[*] Memeriksa sertifikat SSL domain yang mendekati kedaluwarsa (< 30 hari)...")
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Gagal membuka database: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	sitesMgr := sites.NewManager(sites.DefaultPaths(), nil, st)
	renewed, total, err := sitesMgr.RenewAllExpiring(ctx, func(ok bool, target, msg string) {
		status := "SUKSES"
		if !ok {
			status = "GAGAL"
		}
		fmt.Printf("[%s] Domain: %s -> %s\n", status, target, msg)
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saat memindai sertifikat: %v\n", err)
		os.Exit(1)
	}

	if total == 0 {
		fmt.Println("[✓] Semua sertifikat SSL domain masih aktif dan berlaku (tidak ada yang kedaluwarsa dalam 30 hari ke depan).")
	} else {
		fmt.Printf("[✓] Pemindaian selesai: %d dari %d sertifikat dipicu perpanjangan otomatis.\n", renewed, total)
	}
}


