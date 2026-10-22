package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	// version menyimpan informasi versi rilis saat kompilasi dengan -ldflags
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
		fmt.Printf("fik-panel v%s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "Perintah tidak dikenal: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// printUsage menampilkan panduan penggunaan CLI fik-panel
func printUsage() {
	fmt.Println(`Penggunaan: fik-panel <perintah> [argumen]

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
	port := fs.Int("port", 8443, "Port server HTTPS")
	dbPath := fs.String("db", "/var/lib/panel/panel.db", "Lokasi file database SQLite")
	fs.Parse(args)

	fmt.Printf("[fik-panel serve] Memulai server pada port %d (DB: %s)...\n", *port, *dbPath)
	// Implementasi penuh server HTTP akan dipanggil di sini
}

// runAgent menangani eksekusi mode RPC privileged agent
func runAgent(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	idleExit := fs.Duration("idle-exit", 60, "Batas durasi idle sebelum proses keluar otomatis")
	fs.Parse(args)

	fmt.Printf("[fik-panel agent] Memulai agen privileged (idle timeout: %v)...\n", *idleExit)
	// Implementasi socket listener agent akan dipanggil di sini
}

// runInit menangani inisialisasi awal kredensial dan sertifikat TLS
func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	username := fs.String("user", "admin", "Nama pengguna administrator")
	password := fs.String("password", "", "Kata sandi admin (kosongkan untuk acak)")
	fs.Parse(args)

	fmt.Printf("[fik-panel init] Menginisialisasi sistem untuk user: %s (password ditentukan: %t)\n", *username, *password != "")
	// Logika pembuatan kredensial awal
}

// runResetPassword menangani pemulihan kata sandi darurat
func runResetPassword(args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	username := fs.String("user", "admin", "Nama pengguna yang direset")
	fs.Parse(args)

	fmt.Printf("[fik-panel reset-password] Mereset kata sandi untuk: %s\n", *username)
	// Logika reset password dan invalidasi sesi
}

// runDoctor melakukan pengecekan diagnostik sistem
func runDoctor(args []string) {
	fmt.Println("[fik-panel doctor] Memeriksa status kesehatan sistem...")
	// Pengecekan integritas SQLite, sintaks Nginx, dan izin direktori
}
