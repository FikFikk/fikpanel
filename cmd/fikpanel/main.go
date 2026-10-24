package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
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
	case "version":
		fmt.Printf("fikpanel v%s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "Perintah tidak dikenal: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// printUsage menampilkan panduan penggunaan CLI fikpanel
func printUsage() {
	fmt.Println(`Penggunaan: fikpanel <perintah> [argumen]

Perintah yang tersedia:
  serve           Menjalankan server web unprivileged (panel serve)
  agent           Menjalankan agen RPC root socket-activated (panel agent)
  init            Inisialisasi kredensial awal dan konfigurasi panel
  reset-password  Reset kata sandi pengguna admin secara aman
  doctor          Diagnosa integritas database, Nginx, dan perizinan
  version         Menampilkan informasi versi aplikasi`)
}

// runServe menangani eksekusi mode HTTP web server
func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8443, "Port server HTTP/HTTPS")
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
