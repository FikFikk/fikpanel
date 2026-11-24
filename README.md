# fikpanel

<p align="center">
  <strong>Server Control Panel Ringan, Modern, & Berkinerja Tinggi</strong><br>
  Dibangun dengan arsitektur <em>Single Static Binary</em> bertenaga <strong>Golang</strong> dan antarmuka <strong>Svelte 5</strong>.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go Version" />
  <img src="https://img.shields.io/badge/Svelte-5.0_(Runes)-FF3E00?style=flat&logo=svelte&logoColor=white" alt="Svelte 5" />
  <img src="https://img.shields.io/badge/Database-SQLite_WAL-003B57?style=flat&logo=sqlite&logoColor=white" alt="SQLite WAL" />
  <img src="https://img.shields.io/badge/MCP-Antigravity_Ready-8b5cf6?style=flat" alt="MCP Server" />
  <img src="https://img.shields.io/badge/Architecture-Single_Binary-10b981?style=flat" alt="Single Binary" />
  <img src="https://img.shields.io/badge/License-AGPL--3.0-blue.svg" alt="License: AGPL-3.0" />
</p>

---

## Tentang fikpanel

**fikpanel** adalah web control panel manajemen server open-source yang dirancang untuk kecepatan ekstrem, penggunaan memori minimal (< 25 MB RAM saat idle), dan instalasi tanpa dependensi runtime rumit. Seluruh antarmuka web dikompilasi dan disematkan langsung (*embedded*) ke dalam satu biner statis Go.

Menggabungkan keandalan sistem backend **Golang** dengan reaktivitas generasi terbaru dari **Svelte 5 (Runes)**, fikpanel menghadirkan pengalaman kontrol server bergaya *Bento Dark Dashboard* yang responsif, mulus, dan bebas lag.

---

## Fitur Unggulan

### 1. Dashboard Bento Realtime
- **Penjadwal Tugas (Cron Jobs)**: Manajemen eksekusi skrip otomatis, pembersihan log berkala, trigger manual, dan riwayat terminal log per tugas.
- **Monitor Performa Persisten ala aaPanel (Status Switch & Histori)**: Saklar on/off status monitoring — jika aktif, daemon background mencatat metrik CPU, RAM, Network I/O, dan Load Average (1m, 5m, 15m) tiap 60 detik ke SQLite dengan retensi hingga 30 hari. Jika dimatikan, pencatatan berhenti. Dilengkapi grafik visual time-range (1 jam, 6 jam, hari ini, 7 hari, 30 hari) serta mode Live Streaming SSE (1s).
- **Software App Store & Toolkit 1-Klik**: Katalog instalasi paket software esensial Linux otomatis (`Nginx`, `PHP 8.2/8.3 + FPM Suite`, `MariaDB/MySQL`, `Redis Cache`, `Node.js + NPM`, `Certbot`, `Git`) dilengkapi terminal viewer log proses instalasi live.
- **Task Manager & Process Killer**: Pantau proses sistem paling berat (CPU/RAM teratas), tombol kill proses, dan audit port jaringan yang mendengarkan (*Listening Ports*).
- **Pembersih Sistem & Optimasi RAM (Cleaner)**: Pindai dan bersihkan sampah paket APT, arsip log lama (`.gz`, `.old`), file temporary, serta flush buffer RAM (*drop caches*) dalam 1-klik.
- **Pencadangan & Snapshot (Backup S3 / Lokal)**: Cadangkan database MySQL / direktori website ke MinIO / S3 Object Storage atau arsip lokal secara instan.
- **DNS Zone & Record Manager**: Manajemen authoritative DNS records (A/AAAA, CNAME, TXT, MX) visual per domain.
- **PHP Multi-Version & FPM Tuning**: Deteksi versi PHP aktif (8.3, 8.2, 8.1, 7.4), kontrol daemon FPM, dan form penyesuaian parameter `php.ini` (memory limit, upload size).
- **Webhook Alert & Notifikasi Sistem**: Integrasi bot notifikasi (Telegram, Discord, Generic HTTP) untuk alert beban CPU/RAM tinggi atau selesainya backup.
- **Firewall Guard (UFW GUI)**: Monitoring dan kendali buka-tutup port TCP/UDP sistem operasi secara visual tanpa terminal SSH.
- **Node.js & PM2 Process Manager**: Monitor utilisasi CPU/RAM per proses Node, status uptime, dan kontrol siklus hidup (start/stop/restart) aplikasi.
- **Terminal Shell Nyata**: Eksekusi perintah bash/cmd sistem secara langsung dengan timeout guard dan limit output 64KB (menggantikan mode simulasi mock).
- **SSL Let's Encrypt Auto-Renew Button**: Pemeriksaan masa aktif sertifikat SSL semua domain dan pemicu pembaruan massal secara 1-klik di panel situs.
- **4 Circular Radial Gauges**: Visualisasi status beban (*Load Average*), utilisasi CPU, konsumsi RAM, dan sisa kapasitas penyimpanan Disk (`/`) dilengkapi *ambient glow* dan peringatan ambang batas otomatis (>= 85%).
- **Streaming Metrik SSE (1 Detik)**: Data metrik dikirim via *Server-Sent Events* dengan penyimpanan riwayat *Ring Buffer* 300 titik tanpa membebani database.
- **Canvas Realtime Wave Chart**: Grafik performa bergradasi warna dinamis, deteksi metrik puncak (*Peak Stats*), throughput jaringan (Download / Upload), serta crosshair interaktif.
- **Top Bar Utilitas**: Indikator status Live dengan animasi *two-layer pulse*, IP *click-to-copy*, dan toggle *eye mask* untuk privasi screenshot.

### 2. Manajemen Website & Reverse Proxy
- Pembuatan dan pengelolaan *Virtual Host* Nginx (Reverse Proxy, Static HTML/JS, dan PHP-FPM).
- Pengaturan SSL/TLS otomatis via Let's Encrypt (protokol ACME HTTP-01) dengan rotasi otomatis.
- Validasi konfigurasi atomik (`nginx -t`) sebelum commit perubahan konfigurasi untuk mencegah *downtime*.

### 3. Manajemen Layanan (Services)
- Kontrol status layanan (Start, Stop, Restart) langsung dari dashboard.
- Dilengkapi sistem *allowlist* keamanan untuk unit penting: `fikpanel`, `nginx`, `sshd`, `mariadb`, `redis`, dan `php-fpm`.

### 4. Keamanan Berlapis & Audit Trail
- **Zero CGO SQLite WAL**: Penyimpanan lokal berkecepatan tinggi dengan mode *Write-Ahead Logging*.
- **Argon2id Password Hashing**: Standar industri paling aman terhadap serangan brute-force.
- **Proteksi Mutasi CSRF**: Seluruh permintaan mutasi HTTP diwajibkan menyertakan header keamanan `X-Panel-Request: 1`.
- **Audit Log Terperinci**: Riwayat aktivitas administratif tersimpan rapi dengan retensi otomatis 90 hari.

### 5. Web Terminal PTY, CodeMirror 6 IDE & Built-in MCP Server (Google Antigravity)
- **Built-in MCP Server (`/mcp` Streamable HTTP):** Hubungkan **Google Antigravity** langsung ke server FikPanel dengan **Bearer Token 256-bit** ber-scope (**18 Read-Only** diagnostic tools + **8 Operator** tools non-destruktif termasuk Vhost, Log Laravel/App, PM2, Docker, Port Listening, Database metadata, Git Pull Situs, WAF IP Block, serta **3 MCP Runbook Prompts**), penyensoran rahasia log otomatis, audit trail lengkap, dan generator `mcp_config.json` 1-klik di Pengaturan.
- **Built-in Uptime & SSL Monitor + Public Status Page (`/status`):** Pemantauan HTTP/HTTPS/TCP terjadwal dengan strip 30-bar heartbeat, peringatan kedaluwarsa SSL, notifikasi webhook Telegram/Discord/Custom JSON, impor situs 1-klik, dan halaman status publik mandiri tanpa login.
- **Web Terminal PTY WebGL & Remote SSH Pool:** Terminal interaktif `xterm.js` berakselerasi GPU WebGL (`@xterm/addon-webgl`), mode Console SSE dengan tombol Stop, pencarian & export log, serta manajer remote SSH terenkripsi **AES-256-GCM** dengan verifikasi host **TOFU**.
- **CodeMirror 6 IDE Editor:** Editor kode bertema design token (17+ bahasa: Nginx, YAML, JSON, PHP, JS/TS, HTML, SQL, Dockerfile, Shell, INI) di File Manager, Nginx Manager, Vhost/Rewrite, Docker Compose, Kubernetes, dan PHP Manager.
- **Keamanan & Personalisasi Modern:** **FIDO2 Passkeys / WebAuthn** (Windows Hello PIN, QR HP, Biometrik), **2FA TOTP** dengan kode pemulihan, **Custom Panel TLS Hot-Reload**, **IP/CIDR Allowlist anti-lockout**, **Appearance & Branding Studio** (7 palet + Custom Hex + multi-bahasa ID/EN), **Actionable `Ctrl+K` Command Palette**, serta pengiriman aset pra-kompresi **Brotli (`.br`) + Gzip** dengan `immutable` caching.
- **AI Copilot Diagnostik:** Asisten AI internal (OpenAI-compatible, Anthropic, Gemini) dengan fallback diagnostik lokal offline.

---

## Arsitektur & Teknologi

| Lapisan | Teknologi | Peran & Alasan Pemilihan |
|---|---|---|
| **Backend Core** | **Go (Golang 1.26+)** | Stdlib-first, *cross-compile* `CGO_ENABLED=0`, pemakaian RAM idle 8–15 MB, konkurensi goroutine efisien, MCP SDK resmi |
| **Frontend UI** | **Svelte 5 (Runes)** | Reaktivitas berbasis `$state` dan `$derived`, ukuran bundel ultra-ringan (~90 KB), zero virtual DOM overhead |
| **Database** | **SQLite (modernc.org/sqlite)** | Pure-Go tanpa CGO, binary tetap 100% static, bebas dependensi eksternal, performa tinggi dengan WAL mode |
| **Aset Web** | **Go `embed.FS`** | Seluruh aset HTML, CSS, dan JS dikompilasi dan disematkan langsung ke dalam satu biner `fikpanel` |
| **Styling & Ikon** | **CSS3 Bento + RemixIcon v4** | Desain bertema dark modern (Linear/Coolify aesthetic), bebas emoji, ramah aksesibilitas |

---

## Struktur Direktori

```text
fikpanel/
├── cmd/
│   └── fikpanel/            # Entry point biner utama CLI (serve, init, reset-password, doctor)
├── internal/
│   ├── auth/                # Hashing kata sandi Argon2id & validasi keamanan
│   ├── metrics/             # Kolektor metrik sistem OS, ring buffer 300 titik, & SSE hub
│   ├── server/              # HTTP Mux, middleware autentikasi, CSRF guard, & REST API
│   ├── services/            # Manager systemd & pemantau status unit layanan
│   └── store/               # Abstraksi database SQLite WAL, skema migrasi, & audit trail
├── web/
│   ├── src/
│   │   ├── lib/             # Komponen Gauge radial, Chart canvas, API helper, & tipe
│   │   ├── routes/          # Tampilan rute Svelte (Home, Sites, Services, Terminal, dll)
│   │   ├── App.svelte       # Root component, hash routing, & shell layout
│   │   └── main.ts          # Mount Svelte 5 app
│   ├── dist/                # Hasil build produksi frontend (disematkan via embed.FS)
│   ├── embed.go             # Inisialisasi embed.FS Go
│   └── vite.config.mjs      # Konfigurasi bundler Vite
├── documentation.md         # Dokumentasi logika bisnis & Mermaid flowchart entitas
├── documentation-api.md     # Spesifikasi lengkap endpoint REST API & tugas terjadwal
└── README.md                # Dokumentasi proyek utama
```

---

## Panduan Memulai (Development & Build)

### Prasyarat
1. **Go 1.26** atau versi lebih baru.
2. **Node.js 18+** & **npm** (untuk kompilasi frontend).

### 1. Klon Repositori
```bash
git clone https://github.com/FikFikk/fikpanel.git
cd fikpanel
```

### 2. Kompilasi Aset Frontend (Svelte 5)
```bash
cd web
npm install
npm run build
cd ..
```

### 3. Jalankan Server dalam Mode Development
```bash
go run ./cmd/fikpanel serve --dev --port 1405
```
> Server akan berjalan di `http://127.0.0.1:1405` (mode `--dev` memakai HTTP biasa).
> Pada mode `--dev`, akun pengujian lokal dibuat otomatis:
> - **Username**: `root`
> - **Password**: *(kosongkan / tekan Masuk)*
>
> **Jangan pernah memakai `--dev` di server publik.** Tanpa `--dev` (mode produksi), FikPanel membuat akun `root` dengan kata sandi acak saat start pertama, mengganti kata sandi kosong yang tersisa dari instalasi lama, dan melayani HTTPS.

### 4. Build Single Binary Produksi
```bash
# Untuk Windows
go build -ldflags="-s -w" -o bin/fikpanel.exe ./cmd/fikpanel

# Untuk Linux (Cross-Compile)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/fikpanel ./cmd/fikpanel
```

### 5. Alur Rilis Versi Baru (Git Tag & CI/CD Otomatis)

Setiap kali Anda selesai mengembangkan fitur atau menu baru dan ingin merilis versi terbaru ke publik/server:

#### Opsi A: Menggunakan Git Tag (Rekomendasi Developer)
```bash
# 1. Pastikan semua perubahan sudah di-commit dan dipush ke branch main
git add .
git commit -m "feat: deskripsi pembaruan atau fitur baru"
git push origin main

# 2. Buat tag versi baru (gunakan format vX.Y.Z, misal: v0.6.0)
git tag -a v0.6.0 -m "Release v0.6.0"

# 3. Push tag ke GitHub (otomatis memicu build & release di GitHub Actions)
git push origin main --follow-tags
```

#### Opsi B: Menggunakan Web GitHub (Tombol "Run workflow")
1. Buka repositori Anda di GitHub > tab **Actions**.
2. Pilih workflow **Release FikPanel Binaries** di bilah kiri.
3. Klik tombol biru **Run workflow** di sebelah kanan.
4. Masukkan nama tag rilis (misal `v0.3.12`), lalu klik **Run workflow**.

> **Apa yang terjadi secara otomatis?**  
> Runner GitHub Actions akan secara otomatis mengompilasi frontend Svelte 5, mengompilasi biner Go multi-arsitektur (`Linux amd64`, `Linux arm64`, dan `Windows amd64`), lalu mengunggahnya ke halaman [GitHub Releases](https://github.com/FikFikk/fikpanel/releases). Server yang menjalankan `curl ... install.sh` atau `fikpanel update` mendapatkan versi tersebut setelah workflow selesai (±2–3 menit setelah tag di-push), dan server dengan auto-update aktif memasangnya dalam pemeriksaan berikutnya (paling lambat ±1,5 jam).

---

## Instalasi Cepat di Server Linux (One-Line Installer)

FikPanel dapat langsung dipasang di server Linux Anda (Ubuntu, Debian, AlmaLinux, Rocky Linux, CentOS, RHEL) hanya dengan satu baris perintah:

### 1. Metode Utama (Tersedia Sekarang via GitHub CDN)
```bash
curl -sSL https://raw.githubusercontent.com/FikFikk/fikpanel/main/scripts/install.sh | sudo bash
```

### 2. Metode Domain Kustom (*Coming Soon*)
```bash
# Segera hadir melalui jalur domain resmi:
curl -sSL https://panel.fikfikk.my.id/install.sh | sudo bash
```

> **Catatan Instalasi:**
> - Installer secara otomatis melakukan **pre-deteksi software** pada host server.
> - Jika server sudah memiliki proyek berjalan (Nginx, MySQL, PHP, Docker), konfigurasi dan database yang ada **TIDAK AKAN DITIMPA** (fitur *smart-skip*).
> - Komponen FikPanel mandiri dan menggunakan database embedded SQLite. Software website seperti MySQL, PHP, Docker, dan Fail2ban bersifat opsional dan dapat dipasang belakangan dengan 1-klik dari Web Panel.
> - Akses Web Panel setelah instalasi: **`https://IP_SERVER:1405`**. Alamat `http://` otomatis dialihkan ke HTTPS.

### Login Pertama & Keamanan Akun
- Installer menampilkan **username `root` dan kata sandi acak 20 karakter** di akhir instalasi. Kata sandi yang sama tersimpan di `/opt/fikpanel/data/admin-credentials.txt` (izin `0600`, hanya root): `sudo cat /opt/fikpanel/data/admin-credentials.txt`.
- Ganti kata sandi di menu **Pengaturan** setelah login (minimal 8 karakter). Berkas kredensial di atas otomatis dihapus dan sesi login di perangkat lain dikeluarkan.
- Lupa kata sandi: `sudo fikpanel reset-password` (kata sandi baru acak ditampilkan sekali, semua sesi akun tersebut dikeluarkan). Opsi: `--user <nama>`, `--password <sandi>`.
- **Server lama (≤ v0.3.19)** yang akun `root`-nya masih tanpa kata sandi: saat diperbarui, kata sandi kosong diganti otomatis dengan kata sandi acak (lihat berkas kredensial di atas) dan semua sesi lama dicabut.

### HTTPS Panel
- Panel melayani HTTPS di port `1405` dengan sertifikat **self-signed** yang dibuat otomatis (`/opt/fikpanel/data/tls/panel.crt` + `panel.key`, diperbarui otomatis sebelum kedaluwarsa). Browser menampilkan peringatan sertifikat saat pertama dibuka; sidik jari SHA-256 sertifikat tercatat di `journalctl -u fikpanel`.
- Sertifikat sendiri (misalnya Let's Encrypt untuk `panel.domain-anda.com`) cukup ditimpa ke dua berkas tersebut; dimuat ulang otomatis dalam ±30 detik tanpa restart, dan tidak pernah ditimpa FikPanel.
- HTTP biasa dari `127.0.0.1` tetap dilayani (reverse proxy Nginx / Cloudflare Tunnel di host yang sama). Untuk mematikan TLS sepenuhnya, tambahkan `--tls=off` pada `ExecStart` di `fikpanel.service`.

---

## Cara Pembaruan (Aman, dengan Rollback Otomatis)

Pembaruan hanya mengganti biner panel. Website, virtual host Nginx, PHP, MySQL, dan container tetap berjalan; yang di-restart hanya layanan `fikpanel` (panel tidak dapat diakses beberapa detik).

### 1. Auto-Update Terjadwal (Default)
Instalasi baru (dan server yang sudah sekali diperbarui ke versi dengan updater baru) otomatis memeriksa rilis stabil baru **setiap jam** (dengan jeda acak hingga 30 menit agar tidak semua server update bersamaan) melalui `fikpanel-autoupdate.timer`.

```bash
sudo fikpanel auto-update status    # status timer + hasil pembaruan terakhir
sudo fikpanel auto-update disable   # matikan (tetap mati walaupun panel diperbarui)
sudo fikpanel auto-update enable    # nyalakan kembali
```
Auto-update tidak menyalakan layanan yang sengaja dihentikan admin, dan tidak mencoba ulang rilis yang pernah gagal health check sampai rilis berikutnya terbit.

### 2. Pembaruan Manual
```bash
sudo fikpanel update            # ke rilis stabil terbaru
sudo fikpanel update v0.3.18    # upgrade/downgrade ke versi tertentu
sudo fikpanel update --status   # hasil pembaruan terakhir + log
```

### 3. Pemulihan / Server Lama (≤ v0.3.19)
```bash
curl -fsSL https://raw.githubusercontent.com/FikFikk/fikpanel/main/scripts/update.sh | sudo bash
```
Skrip ini hanya mengunduh biner rilis terbaru, memverifikasi SHA-256, lalu menyerahkan proses ke updater bawaan biner tersebut. Server lama yang menjalankan `fikpanel update` otomatis melewati jalur ini, sehingga cukup satu kali pembaruan manual; setelah itu auto-update aktif.

> **Alur pembaruan:**
> 1. Tentukan tag rilis target; jika sudah terpasang dan layanan sudah menjalankannya, tidak ada unduhan maupun restart.
> 2. Unduh paket dan `SHA256SUMS.txt` dari tag yang sama; tolak bila checksum tidak cocok.
> 3. Jalankan `version` pada biner baru dan pastikan sesuai tag (menangkap paket rusak / arsitektur salah).
> 4. Cadangkan biner + snapshot database SQLite ke `/opt/fikpanel/backups/updates/` (3 cadangan terakhir).
> 5. Ganti biner secara atomik, restart `fikpanel.service`, lalu health check: proses aktif, menjalankan biner baru, merespons HTTP, dan stabil (tidak crash-loop).
> 6. Bila versi baru tidak sehat dalam 60 detik, biner **dan** database dikembalikan otomatis ke kondisi sebelum pembaruan.
>
> Proses berjalan di unit systemd terpisah (`fikpanel-updater.service`), sehingga tetap selesai walaupun dipicu dari Web Terminal yang ikut terputus saat panel di-restart. Jika tag baru saja di-push, tunggu 2–3 menit sampai GitHub Actions selesai mengunggah biner; sebelum itu updater akan melaporkan rilis belum tersedia tanpa mengubah apa pun.

---

## Cara Pencopotan (Clean Uninstaller)

Jika Anda ingin mencopot FikPanel sepenuhnya dari server:

### 1. Pencopotan Cepat via One-Line Command
```bash
curl -sSL https://raw.githubusercontent.com/FikFikk/fikpanel/main/scripts/uninstall.sh | sudo bash
```

### 2. Melalui Skrip Lokal di Server
```bash
sudo /opt/fikpanel/scripts/uninstall.sh
```

> **Opsi Uninstaller:**
> - `-y` atau `--yes`: Melewati konfirmasi interaktif.
> - `--full`: Mencopot FikPanel sekaligus membersihkan paket stack (Nginx, MySQL, PHP) dari server.

---

## Panduan Perintah CLI (`fikpanel --help`)

Biner `fikpanel` menyediakan utilitas baris perintah terintegrasi:

```text
FikPanel — Lightweight Server Control Panel (v0.1.5)

Penggunaan:
  fikpanel <perintah> [opsi]

Perintah yang Tersedia:
  serve           Menjalankan server web control panel
                  Opsi:
                    --port <nomor>   Port server HTTP/HTTPS (default: 1405)
                    --db <path>      Lokasi berkas database SQLite (default: fikpanel.db)
                    --tls <mode>     auto (default, HTTPS self-signed / sertifikat di <data>/tls) atau off
                    --dev            Pengembangan lokal: akun root tanpa sandi (JANGAN di server publik)
  discover        Pindai infrastruktur server dan adopsi otomatis konfigurasi
                  Opsi:
                    --db <path>      Lokasi berkas database SQLite (default: fikpanel.db)
                    --adopt          Adopsi otomatis virtual host Nginx ke database (default: true)
                    --roots <paths>  Direktori kustom tambahan untuk pemindaian (pisahkan koma)
  update          Perbarui FikPanel secara aman (checksum, backup, restart, health check, rollback)
  auto-update     Kelola pembaruan otomatis terjadwal: enable | disable | status
  doctor          Diagnosa integritas database, Nginx, dan perizinan sistem
  reset-password  Reset kata sandi admin (acak bila --password kosong) & keluarkan semua sesinya
                  Opsi: --user <nama>  --password <sandi>  --db <path>
  init            Buat akun administrator baru (kata sandi acak bila --password kosong)
  version, -v     Menampilkan informasi versi biner FikPanel
  help, -h        Menampilkan panduan bantuan perintah ini

Contoh Penggunaan:
  # Menjalankan daemon panel di port khusus
  fikpanel serve --port 1405 --db /opt/fikpanel/data/fikpanel.db

  # Memindai virtual host dan stack docker di server untuk diadopsi
  fikpanel discover --adopt --roots "/home/fikfikk/projects,/var/www"

  # Reset password akun administrator
  fikpanel reset-password --user root

  # Memperbarui biner ke rilis terbaru
  fikpanel update

  # Menampilkan panduan bantuan
  fikpanel --help
```

---

## Dokumentasi Lengkap
- [Dokumentasi Logika Bisnis & Siklus Hidup Entitas (documentation.md)](./documentation.md)
- [Spesifikasi REST API & Tugas Terjadwal (documentation-api.md)](./documentation-api.md)

---

## Lisensi & Perlindungan Hak Cipta
Didistribusikan di bawah lisensi **GNU Affero General Public License v3.0 (AGPL-3.0)**.  
Hak Cipta (C) 2026 **FikFikk (Fikri)**. Seluruh hak cipta dilindungi undang-undang.

> **Pemberitahuan Orisinalitas & Merek Dagang:**  
> Nama **FikPanel**, logo, serta identitas visual proyek adalah hak eksklusif **FikFikk**. Modifikasi atau redistribusi kode sumber wajib mempertahankan atribusi pencipta asli pada antarmuka pengguna dan tidak diperkenankan menggunakan merek FikPanel untuk produk turunan komersial tanpa izin tertulis.

<!-- Pair Extraordinaire Trigger -->
