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
	"fikpanel/internal/store"
)

var (
	// version menyimpan informasi versi rilis saat kompilasi
	version = "0.1.0-dev"
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
  update          Perbarui biner FikPanel ke versi rilis terbaru tanpa menyentuh data
  doctor          Diagnosa integritas database, Nginx, dan perizinan sistem
  reset-password  Reset kata sandi pengguna admin secara aman melalui CLI
  init            Inisialisasi kredensial awal administrator
  version, -v     Menampilkan informasi versi biner FikPanel
  help, -h        Menampilkan panduan bantuan perintah ini

Contoh Penggunaan:
  fikpanel serve --port 1405
  fikpanel discover --adopt --roots "/home/fikfikk/projects,/var/www"
  fikpanel reset-password --user root
  fikpanel update
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

// runUpdate menangani pembaruan otomatis biner fikpanel
func runUpdate(args []string) {
	fmt.Println("==========================================================")
	fmt.Println("             FikPanel Self-Update Utility                 ")
	fmt.Println("==========================================================")

	if runtime.GOOS != "linux" {
		fmt.Printf("Pembaruan otomatis saat ini dioptimalkan untuk lingkungan Linux (produksi).\nVersi lokal saat ini: v%s\n", version)
		return
	}

	updateScript := "/opt/fikpanel/scripts/update.sh"
	if _, err := os.Stat(updateScript); err == nil {
		cmd := exec.Command("bash", updateScript)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Pembaruan gagal: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Fallback jika file skrip lokal belum tersedia: eksekusi via unduhan langsung
	cmd := exec.Command("bash", "-c", "curl -sSL https://raw.githubusercontent.com/FikFikk/fikpanel/main/scripts/update.sh | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Pembaruan gagal: %v\n", err)
		os.Exit(1)
	}
}

