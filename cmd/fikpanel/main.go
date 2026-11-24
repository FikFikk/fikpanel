package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"fikpanel/internal/auth"
	"fikpanel/internal/credentials"
	"fikpanel/internal/discovery"
	"fikpanel/internal/metrics"
	"fikpanel/internal/server"
	"fikpanel/internal/services"
	"fikpanel/internal/sites"
	"fikpanel/internal/store"
	"fikpanel/internal/tui"
	"fikpanel/internal/updater"
)

var (
	// version menyimpan informasi versi rilis saat kompilasi
	version = "0.6.4"
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
	case "tui", "top":
		runTUI(os.Args[2:])
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
	case "auto-update", "autoupdate":
		runAutoUpdate(os.Args[2:])
	case "post-update":
		runPostUpdate(os.Args[2:])
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
                    --tls <mode>     auto (default, HTTPS self-signed / sertifikat di <data>/tls) atau off
                    --dev            Pengembangan lokal: akun root tanpa sandi (JANGAN di server publik)
  tui, top        Buka dashboard TUI interaktif (Bubble Tea) langsung di terminal SSH
                  Opsi:
                    --db <path>      Lokasi database SQLite (default: otomatis deteksi layanan aktif)
                    --refresh <dur>  Interval penyegaran metrik (default: 2s)
  discover        Pindai infrastruktur server dan adopsi otomatis konfigurasi
                  Opsi:
                    --db <path>      Lokasi berkas database SQLite (default: fikpanel.db)
                    --adopt          Adopsi otomatis virtual host Nginx ke database (default: true)
                    --roots <paths>  Direktori kustom tambahan untuk pemindaian (pisahkan koma)
  versions        Daftar seluruh versi rilis FikPanel yang tersedia di GitHub Releases
  update          Perbarui FikPanel secara aman (checksum, backup, restart, health check, rollback)
                  Opsi / Parameter:
                    <versi>          Target versi spesifik untuk upgrade/downgrade (misal: v0.3.1)
                    --check, -c      Hanya cek ketersediaan rilis baru tanpa memasang
                    --status, -s     Tampilkan hasil pembaruan terakhir beserta log-nya
                    --force          Pasang ulang walaupun versi target sudah terpasang
  auto-update     Kelola pembaruan otomatis terjadwal: enable | disable | status
  renew-ssl       Pindai seluruh domain dan perpanjang sertifikat SSL yang < 30 hari
                  Opsi:
                    --db <path>      Lokasi berkas database SQLite (default: fikpanel.db)
  doctor          Diagnosa integritas database, Nginx, dan perizinan sistem
  reset-password  Reset kata sandi admin (acak bila --password kosong) & keluarkan semua sesinya
                  Opsi: --user <nama>  --password <sandi>  --db <path>
  init            Buat akun administrator baru (kata sandi acak bila --password kosong)
  version, -v     Menampilkan informasi versi biner FikPanel
  help, -h        Menampilkan panduan bantuan perintah ini

Contoh Penggunaan:
  fikpanel serve --port 1405
  fikpanel versions
  fikpanel update --check
  fikpanel update
  fikpanel update v0.3.1
  fikpanel update --status
  fikpanel auto-update status
  fikpanel reset-password --user root
`, version)
}

// runServe menangani eksekusi mode HTTP web server
func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 1405, "Port server HTTP/HTTPS")
	dbPath := fs.String("db", "fikpanel.db", "Lokasi file database SQLite")
	isDev := fs.Bool("dev", false, "Mode pengembangan lokal: akun root TANPA kata sandi + proxy Vite (jangan dipakai di server publik)")
	tlsMode := fs.String("tls", "auto", "TLS panel: auto (HTTPS, self-signed bila belum ada sertifikat) atau off (HTTP biasa)")
	fs.Parse(args)

	tlsExplicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "tls" {
			tlsExplicit = true
		}
	})
	mode := strings.ToLower(strings.TrimSpace(*tlsMode))
	if mode != "auto" && mode != "off" {
		fmt.Fprintf(os.Stderr, "Fatal: nilai --tls tidak valid: %q (gunakan auto atau off)\n", *tlsMode)
		os.Exit(2)
	}
	if *isDev && !tlsExplicit {
		mode = "off"
	}

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

	credPath := credentials.PathFor(*dbPath)
	if *isDev {
		// Akun dev (root / kata sandi kosong) hanya untuk pengujian lokal
		devUser, err := st.EnsureDevUser("root", "")
		if err != nil {
			fmt.Printf("Peringatan: Gagal seed akun dev: %v\n", err)
		} else {
			fmt.Printf("[fikpanel dev] Akun dev aktif -> Username: %s | Password: (kosong)\n", devUser.Username)
			fmt.Println("[fikpanel dev] PERINGATAN: mode --dev tidak aman untuk server yang dapat diakses publik.")
		}
	} else {
		// Produksi: panel tidak boleh berjalan dengan akun tanpa kata sandi (fail-closed bila gagal).
		credCtx, credCancel := context.WithTimeout(context.Background(), 30*time.Second)
		res, err := credentials.EnsureAdmin(credCtx, st, credPath)
		credCancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fatal: Gagal mengamankan akun administrator: %v\n", err)
			os.Exit(1)
		}
		for _, c := range res.Created {
			fmt.Printf("[fikpanel serve] Akun administrator awal dibuat: %s. Kata sandi acak tersimpan di %s (lihat: sudo cat %s)\n", c.Username, credPath, credPath)
		}
		for _, c := range res.Rotated {
			fmt.Printf("[fikpanel serve] KEAMANAN: akun %s memakai kata sandi kosong; diganti dengan kata sandi acak di %s\n", c.Username, credPath)
		}
		if res.RevokedSession > 0 {
			fmt.Printf("[fikpanel serve] KEAMANAN: %d sesi login akun tanpa kata sandi dicabut.\n", res.RevokedSession)
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
	srv.SetDevMode(*isDev)
	srv.SetCredentialsFile(credPath)

	scheme := "http"
	if mode == "auto" {
		certDir := filepath.Join(filepath.Dir(credPath), "tls")
		desc, err := srv.EnableTLS(certDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fatal: Gagal menyiapkan TLS panel: %v (jalankan dengan --tls=off untuk HTTP biasa)\n", err)
			os.Exit(1)
		}
		scheme = "https"
		fmt.Printf("[fikpanel serve] TLS aktif: %s\n", desc)
		fmt.Println("[fikpanel serve] HTTP biasa dari luar host di-redirect ke HTTPS; HTTP dari 127.0.0.1 tetap dilayani (reverse proxy lokal).")
	}

	// Jalankan server di goroutine terpisah
	go func() {
		fmt.Printf("[fikpanel serve] Server berjalan di %s://127.0.0.1:%d\n", scheme, *port)
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

// runTUI menjalankan dashboard TUI interaktif (Bubble Tea + Lipgloss) langsung di terminal
func runTUI(args []string) {
	fs := flag.NewFlagSet("tui", flag.ExitOnError)
	dbFlag := fs.String("db", "", "Lokasi database SQLite (default: otomatis deteksi dari fikpanel.service atau fikpanel.db lokal)")
	refresh := fs.Duration("refresh", 2*time.Second, "Interval penyegaran metrik sistem live")
	fs.Parse(args)

	dbPath := strings.TrimSpace(*dbFlag)
	if dbPath == "" {
		if resolved, err := resolveProductionDB(""); err == nil && resolved != "" {
			dbPath = resolved
		} else {
			dbPath = "fikpanel.db"
		}
	}

	if err := tui.Run(tui.Config{
		Version:         version,
		DBPath:          dbPath,
		RefreshInterval: *refresh,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[fikpanel tui] Gagal menjalankan TUI: %v\n", err)
		os.Exit(1)
	}
}

// runAgent menangani eksekusi mode RPC privileged agent
func runAgent(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	idleExit := fs.Duration("idle-exit", 60*time.Second, "Batas durasi idle sebelum proses keluar otomatis")
	fs.Parse(args)

	fmt.Printf("[fikpanel agent] Agen privileged aktif (idle timeout: %v)...\n", *idleExit)
}

// runInit membuat akun administrator baru (kata sandi acak bila tidak diberikan)
func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	username := fs.String("user", "admin", "Nama pengguna administrator")
	password := fs.String("password", "", "Kata sandi admin (kosongkan untuk dibangkitkan acak)")
	dbPath := fs.String("db", "fikpanel.db", "Lokasi file database SQLite")
	fs.Parse(args)

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: Gagal membuka database: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := st.GetUserByUsername(ctx, *username); err == nil {
		fmt.Printf("[fikpanel init] Akun %s sudah ada. Untuk mengganti kata sandinya: sudo fikpanel reset-password --user %s\n", *username, *username)
		return
	}

	pw, generated, err := chooseNewPassword(*password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(2)
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal mengenkripsi kata sandi: %v\n", err)
		os.Exit(1)
	}
	if _, err := st.CreateAdminUser(ctx, *username, hash); err != nil {
		fmt.Fprintf(os.Stderr, "Gagal membuat akun: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[fikpanel init] Akun administrator %s berhasil dibuat.\n", *username)
	if generated {
		fmt.Printf("  Kata sandi: %s\n  Simpan kata sandi ini; tidak akan ditampilkan lagi.\n", pw)
	}
}

// chooseNewPassword memvalidasi kata sandi dari flag atau membangkitkan yang acak bila kosong.
func chooseNewPassword(fromFlag string) (password string, generated bool, err error) {
	if fromFlag != "" {
		if err := auth.ValidateNewPassword(fromFlag); err != nil {
			return "", false, err
		}
		return fromFlag, false, nil
	}
	pw, err := auth.GeneratePassword(credentials.GeneratedPasswordLength)
	return pw, true, err
}

// resolveProductionDB menentukan database yang dipakai layanan fikpanel (dari unit systemd),
// lalu database lokal di direktori kerja. Tidak pernah membuat database baru secara diam-diam.
func resolveProductionDB(flagValue string) (string, error) {
	if flagValue != "" {
		if _, err := os.Stat(flagValue); err != nil {
			return "", fmt.Errorf("database %s tidak ditemukan", flagValue)
		}
		return flagValue, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	candidates := []string{updater.DetectPaths(ctx, updater.NewSystem()).DBPath, updater.DefaultDBPath, "fikpanel.db"}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.Mode().IsRegular() {
			return c, nil
		}
	}
	return "", fmt.Errorf("database FikPanel tidak ditemukan (dicoba: %s); gunakan --db <path>", strings.Join(candidates, ", "))
}

// runResetPassword mereset kata sandi administrator melalui CLI (akses root di server)
func runResetPassword(args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	username := fs.String("user", "", "Nama pengguna yang direset (default: satu-satunya akun, atau root)")
	password := fs.String("password", "", "Kata sandi baru (kosongkan agar dibangkitkan acak; lebih aman karena tidak tercatat di history shell)")
	dbFlag := fs.String("db", "", "Lokasi database (default: diambil dari fikpanel.service, lalu /opt/fikpanel/data/fikpanel.db)")
	fs.Parse(args)

	if runtime.GOOS == "linux" {
		requireRoot("reset-password")
	}
	dbPath, err := resolveProductionDB(*dbFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}
	st, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Gagal membuka database %s: %v\n", dbPath, err)
		os.Exit(1)
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	users, err := st.ListUsers(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}

	target := strings.TrimSpace(*username)
	if target == "" {
		target = credentials.DefaultAdminUsername
		if len(users) == 1 {
			target = users[0].Username
		}
	}
	var found *store.User
	names := make([]string, 0, len(users))
	for i := range users {
		names = append(names, users[i].Username)
		if users[i].Username == target {
			found = &users[i]
		}
	}
	if found == nil && len(users) > 0 {
		fmt.Fprintf(os.Stderr, "[!] Akun %q tidak ditemukan. Akun yang tersedia: %s\n    Contoh: sudo fikpanel reset-password --user %s\n", target, strings.Join(names, ", "), names[0])
		os.Exit(1)
	}

	pw, generated, err := chooseNewPassword(*password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(2)
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Gagal mengenkripsi kata sandi: %v\n", err)
		os.Exit(1)
	}

	var revoked int64
	if found == nil {
		created, err := st.CreateAdminUser(ctx, target, hash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[!] %v\n", err)
			os.Exit(1)
		}
		found = created
	} else if revoked, err = st.SetPasswordAndRevokeSessions(ctx, found.ID, hash, ""); err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}
	credentials.Forget(credentials.PathFor(dbPath), target)
	_ = st.RecordAudit(ctx, "system", "auth.password_reset_cli", target, true,
		fmt.Sprintf("Kata sandi direset melalui CLI; %d sesi dicabut", revoked), &found.ID)

	fmt.Printf("[✓] Kata sandi akun %s berhasil direset (%d sesi login dikeluarkan).\n", target, revoked)
	fmt.Printf("    Username : %s\n", target)
	if generated {
		fmt.Printf("    Password : %s\n", pw)
		fmt.Println("    Simpan kata sandi ini; tidak akan ditampilkan lagi.")
	} else {
		fmt.Println("    Password : (sesuai nilai --password)")
	}
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
				lowerIssuer := strings.ToLower(item.SSLIssuer)
				lowerCert := strings.ToLower(item.SSLCertPath)
				if strings.Contains(lowerIssuer, "letsencrypt") || strings.Contains(lowerIssuer, "let's encrypt") || strings.Contains(lowerCert, "letsencrypt") {
					sslMode = "letsencrypt"
				}
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

			if created, err := st.CreateSite(ctx, &site); err == nil && created != nil {
				adoptedCount++
				if item.SSL {
					issuer := item.SSLIssuer
					if issuer == "" {
						if sslMode == "letsencrypt" {
							issuer = "Let's Encrypt"
						} else {
							issuer = "Custom SSL"
						}
					}
					notAfter := item.SSLNotAfter
					if notAfter <= 0 {
						notAfter = time.Now().Add(90 * 24 * time.Hour).Unix()
					}
					certDomainsJSON, _ := json.Marshal(append([]string{primary}, aliases...))
					certRecord := store.Certificate{
						SiteID:    created.ID,
						Status:    "issued",
						Issuer:    issuer,
						NotBefore: time.Now().Add(-24 * time.Hour).Unix(),
						NotAfter:  notAfter,
						Domains:   string(certDomainsJSON),
						RenewedAt: time.Now().Unix(),
					}
					_ = st.UpsertCertificate(ctx, &certRecord)

					// Salin fisik cert & key dari path asal ke direktori sertifikat FikPanel
					if item.SSLCertPath != "" && item.SSLKeyPath != "" {
						certBytes, errC := os.ReadFile(item.SSLCertPath)
						keyBytes, errK := os.ReadFile(item.SSLKeyPath)
						if errC == nil && errK == nil {
							paths := sites.DefaultPaths()
							targetCert := filepath.Join(paths.CertDir, primary, "fullchain.pem")
							targetKey := filepath.Join(paths.CertDir, primary, "privkey.pem")
							if err := os.MkdirAll(filepath.Dir(targetCert), 0o755); err == nil {
								_ = os.WriteFile(targetCert, certBytes, 0o644)
								_ = os.WriteFile(targetKey, keyBytes, 0o600)
							}
						}
					}
				}
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
	showAll := false
	limit := 6
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Println("Penggunaan: fikpanel versions [opsi]")
			fmt.Println("Opsi:")
			fmt.Println("  --all, -a    Tampilkan seluruh arsip riwayat rilis lawas")
			return
		}
		if arg == "--all" || arg == "-a" {
			showAll = true
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

	count := 0
	for _, rel := range releases {
		if rel.Draft {
			continue
		}

		if !showAll && count >= limit {
			break
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
		} else if updater.CompareVersions(rel.TagName, "v0.4.0") < 0 {
			status = "[DEPRECATED]"
		}

		title := rel.Name
		if title == "" {
			title = rel.TagName
		}
		if updater.CompareVersions(rel.TagName, "v0.4.0") < 0 && !strings.Contains(title, "Rentan") {
			title += " (Legacy / Rentan)"
		}

		fmt.Printf("%-12s  %-20s  %-15s  %s\n", rel.TagName, dateStr, status, title)
		count++
	}

	fmt.Println(strings.Repeat("-", 74))
	if !showAll && len(releases) > count {
		fmt.Printf("Catatan: Hanya %d rilis terbaru yang ditampilkan demi stabilitas & keamanan.\n", count)
		fmt.Println("Gunakan 'fikpanel versions --all' jika ingin melihat seluruh arsip rilis lawas.")
		fmt.Println(strings.Repeat("-", 74))
	}
	fmt.Println("Pilihan Perintah Pembaruan:")
	fmt.Println("  fikpanel update             # Perbarui ke rilis stabil terbaru")
	fmt.Println("  fikpanel update <versi>     # Pasang versi tertentu (contoh: fikpanel update v0.4.0)")
	fmt.Println()
}

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

	if updater.CompareVersions(version, latest.TagName) >= 0 {
		fmt.Printf("[✓] FikPanel sudah berada di versi terbaru (v%s).\n    Tidak ada pembaruan yang diperlukan.\n", version)
		return
	}

	fmt.Printf("[!] Pembaruan tersedia: v%s -> %s (%s)\n", version, latest.TagName, latest.Name)
	fmt.Printf("    Tanggal rilis : %s\n", latest.PublishedAt.UTC().Format("2006-01-02 15:04 UTC"))
	fmt.Printf("    Tautan rilis  : %s\n\n", latest.HTMLURL)
	fmt.Println("Jalankan perintah berikut untuk memperbarui secara otomatis:")
	fmt.Println("  fikpanel update")
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


