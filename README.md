# fikpanel

<p align="center">
  <strong>Server Control Panel Ringan, Modern, & Berkinerja Tinggi</strong><br>
  Dibangun dengan arsitektur <em>Single Static Binary</em> bertenaga <strong>Golang</strong> dan antarmuka <strong>Svelte 5</strong>.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go Version" />
  <img src="https://img.shields.io/badge/Svelte-5.0_(Runes)-FF3E00?style=flat&logo=svelte&logoColor=white" alt="Svelte 5" />
  <img src="https://img.shields.io/badge/Database-SQLite_WAL-003B57?style=flat&logo=sqlite&logoColor=white" alt="SQLite WAL" />
  <img src="https://img.shields.io/badge/Architecture-Single_Binary-10b981?style=flat" alt="Single Binary" />
  <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License" />
</p>

---

## Tentang fikpanel

**fikpanel** adalah web control panel manajemen server open-source yang dirancang untuk kecepatan ekstrem, penggunaan memori minimal (< 25 MB RAM saat idle), dan instalasi tanpa dependensi runtime rumit. Seluruh antarmuka web dikompilasi dan disematkan langsung (*embedded*) ke dalam satu biner statis Go.

Menggabungkan keandalan sistem backend **Golang** dengan reaktivitas generasi terbaru dari **Svelte 5 (Runes)**, fikpanel menghadirkan pengalaman kontrol server bergaya *Bento Dark Dashboard* yang responsif, mulus, dan bebas lag.

---

## Fitur Unggulan

### 1. Dashboard Bento Realtime
- **Penjadwal Tugas (Cron Jobs)**: Manajemen eksekusi skrip otomatis, pembersihan log berkala, trigger manual, dan riwayat terminal log per tugas.
- **Pencadangan & Snapshot (Backup S3 / Lokal)**: Cadangkan database MySQL / direktori website ke MinIO / S3 Object Storage atau arsip lokal secara instan.
- **Firewall Guard (UFW GUI)**: Monitoring dan kendali buka-tutup port TCP/UDP sistem operasi secara visual tanpa terminal SSH.
- **Node.js & PM2 Process Manager**: Monitor utilisasi CPU/RAM per proses Node, status uptime, dan kontrol siklus hidup (start/stop/restart) aplikasi.
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

### 5. Terminal Web & AI Copilot
- **Terminal Web Terintegrasi**: Eksekusi perintah sistem secara cepat dengan proteksi sanitasi perintah.
- **AI Copilot Diagnostik**: Asisten cerdas untuk mendiagnosa anomali server, konsumsi resource tinggi, dan saran pemecahan masalah.

---

## Arsitektur & Teknologi

| Lapisan | Teknologi | Peran & Alasan Pemilihan |
|---|---|---|
| **Backend Core** | **Go (Golang 1.22+)** | Stdlib-first, *cross-compile* instan, pemakaian RAM idle 8–15 MB, konkurensi goroutine efisien |
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
1. **Go 1.22** atau versi lebih baru.
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
go run ./cmd/fikpanel serve --dev --port 8443
```
> Server akan berjalan di `http://127.0.0.1:8443`.
> Pada mode `--dev`, akun pengujian lokal dibuat otomatis:
> - **Username**: `root`
> - **Password**: *(kosongkan / tekan Masuk)*

### 4. Build Single Binary Produksi
```bash
# Untuk Windows
go build -ldflags="-s -w" -o bin/fikpanel.exe ./cmd/fikpanel

# Untuk Linux (Cross-Compile)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/fikpanel ./cmd/fikpanel
```

---

## Perintah CLI

Biner `fikpanel` menyediakan utilitas CLI terintegrasi:

```text
fikpanel <perintah> [opsi]

Perintah:
  serve           Menjalankan server web control panel
                  Opsi: --port <port> (default: 8443)
                        --db <path>   (default: fikpanel.db)
                        --dev         (aktifkan auto-seed akun root)
  init            Inisialisasi kredensial awal administrator
  reset-password  Reset kata sandi pengguna secara aman
  doctor          Pemeriksaan diagnostik integritas database dan perizinan sistem
  version         Mencetak versi fikpanel
```

---

## Dokumentasi Lengkap
- [Dokumentasi Logika Bisnis & Siklus Hidup Entitas (documentation.md)](./documentation.md)
- [Spesifikasi REST API & Tugas Terjadwal (documentation-api.md)](./documentation-api.md)

---

## Lisensi
Didistribusikan di bawah lisensi MIT. Silakan gunakan dan kembangkan sesuai kebutuhan Anda.
