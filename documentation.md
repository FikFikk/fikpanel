# Dokumentasi Logika Bisnis & Siklus Hidup Entitas (fikpanel)

## 1. Siklus Hidup Entitas

### 1.1 Siklus Hidup Site & Sertifikat SSL

```mermaid
flowchart TD
    Input[Input Domain Multi-baris + Jenis Situs] --> Validasi{Validasi RFC1123, Duplikat, Domain Terpakai}
    Validasi -- Gagal --> Tolak[Tolak HTTP 400 + Pesan per Domain]
    Validasi -- Sukses --> SimpanDB[Simpan Baris Site ssl_mode none]
    SimpanDB --> Root[Buat Web Root + index.html Sambutan]
    Root --> Render[Render Vhost Port 80 + Lokasi ACME]
    Render --> TestNginx{Uji Sintaks nginx -t}
    TestNginx -- Gagal --> Rollback[Pulihkan Snapshot File + Hapus Baris DB]
    TestNginx -- Sukses --> Aktif[Site Aktif HTTP]
    Aktif --> AktifkanSSL{User Aktifkan SSL di Pengaturan?}
    AktifkanSSL -- Tidak --> Aktif
    AktifkanSSL -- Ya --> CekDNS{Semua Domain Mengarah ke IP Server?}
    CekDNS -- Tidak dan tidak dilewati --> TolakDNS[Tolak HTTP 400 + Daftar Domain Bermasalah]
    CekDNS -- Ya / Dilewati --> Pending[Sertifikat pending - Job ACME HTTP-01]
    Pending -- Gagal --> Failed[Sertifikat failed + last_error]
    Pending -- Sukses --> Issued[Sertifikat issued]
    Issued --> Pasang[Render Blok 443 + Redirect + HSTS lalu Reload]
    Pasang --> AktifSSL[Site Aktif HTTPS]
    AktifSSL --> Renew{Cek 12 Jam: Sisa < 30 Hari?}
    Renew -- Ya --> Pending
    Renew -- Tidak --> AktifSSL
    AktifSSL --> UbahDomain{Domain / Alias Diubah?}
    UbahDomain -- Ya --> Reset[Hapus Sertifikat - Kembali ke HTTP]
    Reset --> AktifkanSSL
```

### 1.2 Siklus Hidup Sesi Pengguna & Sudo Mode

```mermaid
flowchart TD
    InputLogin[Input Kredensial Login] --> CekRateLimit{Gagal < 5 Kali / 15 Menit?}
    CekRateLimit -- Tidak --> Lockout[Kunci IP & Catat Audit Log]
    CekRateLimit -- Ya --> VerifikasiArgon2[Verifikasi Password Hash Argon2id]
    VerifikasiArgon2 -- Gagal --> CatatGagal[Tambah Counter Gagal Login]
    VerifikasiArgon2 -- Sukses --> CekTOTP{2FA / TOTP Aktif?}
    CekTOTP -- Ya --> ValidasiOTP{Validasi Kode 6 Digit}
    ValidasiOTP -- Gagal --> CatatGagal
    ValidasiOTP -- Sukses --> TerbitkanSesi[Buat Session Token 32 Byte Acak]
    CekTOTP -- Tidak --> TerbitkanSesi
    TerbitkanSesi --> SimpanHashSesi[Simpan SHA-256 Token ke DB]
    SimpanHashSesi --> SesiAktif[Sesi Berjalan Standar]
    
    SesiAktif --> AksesSensitif{Akses Aksi Sensitif / Root?}
    AksesSensitif -- Ya --> CekSudoWindow{Window Sudo <= 5 Menit?}
    CekSudoWindow -- Tidak --> MintaReAuth[Minta Password Ulang]
    MintaReAuth --> VerifikasiArgon2
    CekSudoWindow -- Ya --> BukaPTY[Minta Agent Buka PTY / Jalankan Aksi]
    AksesSensitif -- Tidak --> OperasiBiasa[Jalankan Request Read-Only]
    
    SesiAktif --> TimeoutLogout{Idle 15 Menit / Expired?}
    TimeoutLogout -- Ya --> HapusSesi[Hapus Sesi dari DB & Invalidate Cookie]
```

### 1.3 Siklus Operasi Manajemen Berkas (File Manager)

```mermaid
flowchart TD
    ReqFile[Permintaan Operasi Berkas] --> ValPath{Validasi & Sanitasi Path}
    ValPath -- Path Traversal / Ilegal --> TolakReq[Tolak HTTP 400 Bad Request]
    ValPath -- Path Valid --> CekAksi{Jenis Aksi}
    
    CekAksi -- Baca / List --> CekExist{Berkas / Folder Ada?}
    CekExist -- Tidak --> Ret404[Kembalikan HTTP 404]
    CekExist -- Ya --> RenderResp[Kirim Metadata / Konten]
    
    CekAksi -- Tulis / Edit / Unggah --> CekBatasUkuran{Ukuran <= Batas?}
    CekBatasUkuran -- Terlalu Besar --> TolakUkuran[Tolak HTTP 400 Ukuran Melebihi Batas]
    CekBatasUkuran -- Lolos --> TulisDisk[Tulis Berkas ke Disk]
    TulisDisk --> CatatAudit[Rekam Riwayat ke Audit Log]
    
    CekAksi -- Hapus / Chmod --> CekProteksiRoot{Target Root Drive?}
    CekProteksiRoot -- Ya --> TolakAksiRoot[Blokir Operasi Demi Integritas Sistem]
    CekProteksiRoot -- Tidak --> EksekusiOS[Eksekusi via OS File System]
    EksekusiOS --> CatatAudit
```

---

## 2. Aturan Bisnis Entitas (Entity Rules)

### 2.1 Entitas `users`

| Kolom | Aturan Validasi | Catatan Bisnis |
|---|---|---|
| `id` | Primary Key, Auto Increment | Identitas unik akun pengguna |
| `username` | Panjang 3-32 karakter, alphanumeric, wajib unik | Nama akun login operator panel |
| `password_hash` | Wajib string ter-encode Argon2id valid | Hash menggunakan parameter memori 19 MiB |
| `totp_secret` | BLOB terenkripsi AES-256-GCM, opsional (bisa NULL) | Kunci rahasia 2FA, hanya didekripsi saat validasi |
| `role` | Wajib bernilai `'admin'` atau `'viewer'` | Viewer hanya memiliki akses read-only |
| `created_at` | Timestamp Unix integer, tidak boleh null | Waktu pendaftaran akun |
| `last_login_at` | Timestamp Unix integer, nullable | Diperbarui saat proses autentikasi berhasil |

### 2.2 Entitas `sites`

| Kolom | Aturan Validasi | Catatan Bisnis |
|---|---|---|
| `id` | Primary Key, Auto Increment | Identifikasi unik site |
| `domain` | Wajib RFC1123, non-wildcard, unik (termasuk terhadap alias situs lain) | Domain utama = baris non-wildcard pertama; juga nama file vhost |
| `aliases` | JSON Array domain valid, tanpa duplikat | Boleh wildcard (`*.x.com`); `www.` ditambahkan otomatis untuk domain apex bila opsi aktif |
| `kind` | Wajib salah satu: `'proxy'`, `'static'`, `'php'` | Aplikasi Node/Go/Python memakai `proxy` |
| `upstream` | Wajib URL `http(s)://host[:port]` jika `kind='proxy'` | Karakter `; { } ' "` dan spasi ditolak |
| `root_path` | Wajib path absolut jika `kind='static'/'php'` | Default `/var/www/<domain>/public_html`; `..`, karakter khusus, dan direktori sistem (`/`, `/etc`, `/usr`, `/root`, dst.) ditolak |
| `php_version` | Format `X.Y`, hanya untuk `kind='php'` | Menentukan socket `/run/php/phpX.Y-fpm.sock` |
| `description` | Maks 200 karakter, opsional | Catatan bebas untuk operator |
| `ssl_mode` | `'none'` atau `'letsencrypt'` | Selalu `none` saat dibuat; diubah hanya dari tab SSL |
| `force_https` | Boolean (0/1), default 1 | Redirect 301 ke HTTPS, aktif hanya bila sertifikat `issued` |
| `hsts` | Boolean (0/1), default 0 | Hanya bisa 1 bila `force_https=1`; header `max-age=31536000` |
| `extra_conf` | String konfigurasi Nginx bebas | Snippet wajib lolos uji sintaks `nginx -t` |
| `enabled` | Boolean (0 atau 1), default 1 | 0 = file vhost di-rename menjadi `.conf.disabled` |
| `updated_at` | Timestamp Unix integer | Waktu perubahan konfigurasi terakhir |

### 2.3 Entitas `certificates`

| Kolom | Aturan Validasi | Catatan Bisnis |
|---|---|---|
| `site_id` | Foreign Key ke `sites(id)`, Unik, On Delete Cascade | Satu sertifikat per site |
| `status` | `'none'`, `'pending'`, `'issued'`, `'failed'` | Hanya `issued` yang mengaktifkan blok 443 |
| `issuer` | String | Penerbit (misal: Let's Encrypt E6) |
| `domains` | JSON Array | SAN yang tercakup sertifikat (Issued To) |
| `not_before` / `not_after` | Timestamp Unix integer | Masa berlaku sertifikat |
| `signature` / `key_type` | String | Algoritma tanda tangan & jenis kunci (ECDSA P-256) |
| `last_error` | Text, nullable | Pesan kegagalan penerbitan / renew terakhir |
| `renewed_at` | Timestamp Unix integer, nullable | Waktu terakhir penerbitan berhasil |

### 2.4 Entitas `managed_services`

| Kolom | Aturan Validasi | Catatan Bisnis |
|---|---|---|
| `unit` | Primary Key, pola `*.service`, regex aman | Allowlist unit systemd yang diizinkan dikontrol |
| `label` | String deskriptif, nullable | Nama manusia untuk unit layanan |
| `protected` | Boolean (0 atau 1), default 0 | Nilai 1 (seperti sshd, panel) wajib konfirmasi ganda |

### 2.5 Entitas `audit_log`

| Kolom | Aturan Validasi | Catatan Bisnis |
|---|---|---|
| `id` | Primary Key, Auto Increment | Urutan peristiwa audit |
| `ts` | Timestamp Unix integer, wajib terindeks | Waktu presisi eksekusi perintah |
| `user_id` | Foreign Key ke `users(id)`, On Delete Set Null | Operator yang melakukan aksi (NULL jika system/AI) |
| `actor` | Wajib salah satu: `'user'`, `'system'`, `'ai'` | Pengklasifikasi sumber pemicu aksi |
| `action` | String nama aksi terformat (`kategori.verb`) | Nama operasi, misal `site.create`, `service.restart` |
| `target` | String identifier target | Nama unit service, domain site, atau PID |
| `ok` | Boolean (0 atau 1) | Hasil status keberhasilan aksi |
| `detail` | String teks maks 4096 byte | Rangkuman output atau pesan galat |

---

## 3. Matriks Transisi Status Entitas

### 3.1 Transisi Status Site

| Status Awal | Pemicu (Trigger) | Status Akhir | Aturan Bisnis & Efek Samping |
|---|---|---|---|
| `None` | User membuat site & `nginx -t` lolos | `Active` | Web root + vhost port 80 dibuat, Nginx di-reload |
| `None` | Web root / vhost gagal diterapkan | `None` | Snapshot file dipulihkan dan baris DB dihapus |
| `Active` | User klik Nonaktifkan | `Disabled` | File di-rename `.conf.disabled`, Nginx di-reload |
| `Disabled`| User klik Aktifkan | `Active` | File vhost dipulihkan dan Nginx di-reload |
| `Active` | User mengubah pengaturan (PUT) | `Active` | Gagal uji sintaks = data DB & file dikembalikan |
| `Active`/`Disabled` | User menghapus site | `Deleted` | Vhost & sertifikat dihapus; isi web root dipertahankan |

### 3.2 Transisi Status Sertifikat SSL

| Status Awal | Pemicu (Trigger) | Status Akhir | Aturan Bisnis & Efek Samping |
|---|---|---|---|
| `none` | User aktifkan SSL & DNS siap / dilewati | `pending` | Job ACME HTTP-01 berjalan di background |
| `none`/`failed` | Domain wildcard terdaftar | (ditolak) | HTTP-01 tidak mendukung wildcard |
| `pending` | Job sedang berjalan & request baru | (ditolak) | HTTP 409, satu job per site |
| `pending` | Validasi & penerbitan sukses | `issued` | File cert ditulis atomik, blok 443 + redirect + HSTS diterapkan |
| `pending` | Validasi gagal | `failed` | `last_error` disimpan, situs tetap HTTP |
| `failed` | User klik terbitkan ulang | `pending` | Sama seperti penerbitan pertama |
| `issued` | Sisa masa aktif < 30 hari (loop 12 jam) | `pending` → `issued` | Perpanjangan otomatis, Nginx di-reload |
| `issued` | Domain / alias situs diubah | `none` | Record sertifikat dihapus, HTTPS nonaktif sampai terbit ulang |
| `issued` | User nonaktifkan SSL | `issued` | `ssl_mode=none`, blok 443 dilepas; sertifikat tetap tersimpan |

### 3.3 Transisi Status Layanan (Systemd Unit)

| Status Awal | Pemicu (Trigger) | Status Akhir | Aturan Bisnis & Efek Samping |
|---|---|---|---|
| `inactive` | Permintaan start | `active` | Agent mengeksekusi `systemctl start <unit>` |
| `active` | Permintaan stop | `inactive` | Ditolak jika unit `protected=1` tanpa konfirmasi |
| `active` | Permintaan restart | `active` | Agent mengeksekusi `systemctl restart <unit>` |
| `failed` | Permintaan restart | `active` | Mencoba pemulihan unit yang crash |
| `active` | Unit mengalami crash | `failed` | Terdeteksi oleh loop polling / supervisor |

---

## 4. Batasan Bisnis & Pemicu Otomatis (Triggers)

| Kode Aturan | Pemicu | Batasan Bisnis | Tindakan Sistem |
|---|---|---|---|
| **BR-01** | Mutasi Data Sensitif | Membutuhkan status Sudo Aktif | Tolak jika verifikasi sandi terakhir > 5 menit lalu |
| **BR-02** | Login Gagal | Maksimal 5 kegagalan per 15 menit per IP | Kunci autentikasi dari IP terkait selama 15 menit |
| **BR-03** | Eksekusi Alat AI | AI memproses permintaan diagnosa | Batasi hanya fungsi read-only, maks 8 iterasi |
| **BR-04** | Redaksi Log | Data log diambil sebelum dikirim ke AI | Hapus otomatis pola token, password, dan kunci privat |
| **BR-05** | Perpanjangan SSL | Pengecekan sertifikat tiap 12 jam | Picu penerbitan ulang ACME jika sisa masa aktif < 30 hari |
| **BR-06** | Retensi Audit & Chat| Job pembersihan berkala tengah malam | Hapus audit > 90 hari dan pesan AI > 30 hari |
| **BR-07** | Operasi Berkas Kritis | Upaya penghapusan root volume / traversal | Blokir path ilegal dan cegah penghapusan root drive |
| **BR-08** | Editor Teks Berkas | Perubahan konten berkas teks | Deteksi dirty state, konfirmasi sebelum tutup, simpan dengan `Ctrl+S` |
| **BR-09** | Ketersediaan Web Server | Akses menu Sites tanpa Nginx | Tampilkan interactive dependency warning card dan opsi 1-klik instalasi |
| **BR-10** | Live-Reload Pengembangan | File backend Go atau frontend Svelte berubah pada mode dev | Air otomatis kompilasi ulang & restart biner Go; Vite memperbarui komponen UI seketika via HMR tanpa restart manual |
| **BR-11** | SSL Setelah Situs Dibuat | Form tambah situs tidak menyediakan opsi SSL | SSL hanya diaktifkan dari tab SSL setelah vhost port 80 + lokasi ACME aktif |
| **BR-12** | Atomisitas Pembuatan Situs | Web root, vhost, atau `nginx -t` gagal | Baris DB dihapus dan file vhost dikembalikan ke snapshot sebelumnya |
| **BR-13** | Validasi Domain Realtime | Input domain di form (debounce ±450 ms) | Cek format, duplikat, domain terpakai, dan resolusi DNS vs IP server (DNS hanya informatif saat membuat situs) |
| **BR-14** | Prasyarat DNS untuk SSL | Penerbitan Let's Encrypt | Semua domain wajib mengarah ke IP server, kecuali opsi "lewati cek DNS" (NAT/CDN) dicentang |
| **BR-15** | Reset Sertifikat | Domain / alias situs diubah saat sertifikat `issued` | Sertifikat dihapus, situs kembali HTTP, UI meminta terbit ulang |
| **BR-16** | HSTS | Opsi HSTS dicentang | Hanya tersimpan bila Paksa HTTPS aktif |
| **BR-17** | Watchdog Nginx | Opsi daemon Nginx aktif | Nginx yang mati otomatis dinyalakan ulang setiap 1 menit setelah lolos `nginx -t` |
| **BR-18** | Simpan Konfigurasi Nginx | Edit `nginx.conf` dari Nginx Manager | Uji sintaks file sementara, backup rotasi 10 file, rename atomik, lalu reload |
| **BR-19** | Standar Desain UI & Token | Perubahan atau pembuatan komponen antarmuka | Nilai visual wajib berasal dari token terpusat (`tokens.css`). Dilarang memakai warna mentah, font eksternal, atau CDN. Divalidasi oleh `npm run lint:ui`. |
| **BR-20** | Dialog Konfirmasi Destruktif | Aksi penghapusan berkas, situs, stop service, dan reset SSL | Wajib menggunakan `confirmDialog()` atau `promptDialog()` berbasis Svelte runes; dilarang menggunakan fungsi dialog native browser (`confirm/alert/prompt`). |
| **BR-21** | Hardening Header Keamanan | Pengiriman respon HTTP dan frontend statis | Menyematkan `Content-Security-Policy` ketat, `Permissions-Policy`, `COOP`, `CORP`, `X-Content-Type-Options: nosniff`, dan `X-Frame-Options: DENY`. |
| **BR-22** | Klien API Terpusat & Anti-CSRF | Seluruh request HTTP frontend | Wajib melalui `apiFetch` relatif same-origin; menyematkan otomatis header `X-Panel-Request: 1` pada mutasi data dan penanganan 401 otomatis. |
| **BR-23** | Kustomisasi Provider LLM | Pengguna mengonfigurasi AI Copilot | Mendukung OpenAI-compatible (Omniroute, 9router, OpenAI, Groq, Ollama), Anthropic Claude, dan Google Gemini dengan Base URL bebas; API key disensor pada respon dan log. |
| **BR-24** | Persistensi Riwayat Percakapan AI | Pengguna mengirim dan membaca pesan AI | Riwayat disimpan dalam tabel `ai_conversations` dan `ai_messages`; menyertakan judul otomatis dan konteks memori maksimal 16 pesan terakhir. |
| **BR-25** | Fallback Diagnostik Server Offline | Provider eksternal tidak aktif atau gagal | AI Copilot otomatis beralih ke engine diagnostik lokal berbasis status real-time Nginx, CPU, RAM, dan Disk tanpa melempar kegagalan fatal. |


