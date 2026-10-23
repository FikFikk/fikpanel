# Dokumentasi Spesifikasi API & Tugas Terjadwal (fikpanel)

## 1. Daftar Endpoint API

### 1.1 Autentikasi & Sesi

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/auth/login` |
| Auth Required | No |
| Headers | `Content-Type: application/json` |
| Query Params | Tidak ada |
| Request Body | `username` (string, wajib), `password` (string, wajib), `totp_code` (string, opsional) |
| Response | HTTP 200 `{"ok": true, "user": {"id": 1, "username": "admin", "role": "admin"}}` + Cookie `panel_session` |
| Notes | Rate limited: 5 kegagalan per 15 menit per IP. Cookie disetel `HttpOnly; Secure; SameSite=Strict`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/auth/logout` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true}` |
| Notes | Menghapus token dari tabel `sessions` dan membersihkan cookie browser. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/auth/me` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"id": 1, "username": "admin", "role": "admin", "sudo_active": true}` |
| Notes | Mengembalikan profil pengguna aktif dan status window otorisasi sudo. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/auth/sudo` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `password` (string, wajib) |
| Response | HTTP 200 `{"ok": true, "sudo_until": 1771747800}` |
| Notes | Mengaktifkan window sudo selama 5 menit untuk eksekusi aksi sensitif/terminal root. |

---

### 1.2 Pemantauan Sistem (Monitoring & Metrik)

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/metrics/live` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Accept: text/event-stream` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 SSE stream data CPU, RAM, disk, network, dan load average setiap 1 detik |
| Notes | Ticker 1 detik hanya aktif bila terdapat subscriber SSE aktif. Buffer awal 300 titik (5 menit) dikirim saat inisiasi koneksi. |

---

### 1.3 Pengelolaan Situs (Sites & Virtual Host)

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/sites` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `mock_nginx` (string, opsional, `0` untuk simulasi belum terpasang) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"sites": [...], "web_server": {"name": "nginx", "installed": true, "version": "1.24.0", "running": true}}` |
| Notes | Mengembalikan seluruh virtual host terkelola beserta status ketersediaan dan runtime Nginx web server. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/system/software` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"software": [{"name": "nginx", "display_name": "Nginx Web Server", "installed": true, "version": "1.24.0", "running": true}]}` |
| Notes | Mendeteksi ketersediaan paket software utama (Nginx, Apache, MySQL, PHP, Node.js, Python) via binary path dan systemctl. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/system/software/install` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `package` (string, wajib: "nginx", "mysql", "php", dll) |
| Response | HTTP 200 `{"ok": true, "package": "nginx", "message": "Proses instalasi nginx berhasil dimulai"}` |
| Notes | Memicu pemasangan dependensi software sistem via task manager / daemon dan dicatat ke audit log. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/sites` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `domain` (string, wajib), `kind` (string, wajib: proxy/static/php), `upstream` (string, opsional), `root_path` (string, opsional), `ssl_mode` (string, default: none), `force_https` (int, default: 1), `extra_conf` (string, opsional) |
| Response | HTTP 201 `{"ok": true, "id": 2}` |
| Notes | Menjalankan validasi RFC1123, render file sementara, uji `nginx -t`, rename atomik, dan reload Nginx. Rollback otomatis jika uji sintaks gagal. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/sites/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true}` |
| Notes | Menghapus konfigurasi Nginx vhost, sertifikat terkait, dan me-reload Nginx. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/sites/{id}/ssl` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "issuer": "Let's Encrypt", "not_after": 1780000000}` |
| Notes | Memicu penerbitan sertifikat SSL via lego HTTP-01 webroot secara on-demand. |

---

### 1.4 Supervisor Layanan (Services)

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/services` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"services": [{"unit": "nginx.service", "active_state": "active", "sub_state": "running", "protected": 0}]}` |
| Notes | Menampilkan status unit yang terdaftar di tabel `managed_services`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/services/{unit}/action` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `action` (string, wajib: start/stop/restart/reload) |
| Response | HTTP 200 `{"ok": true, "unit": "nginx.service", "action": "restart"}` |
| Notes | Validasi allowlist nama unit. Jika `protected=1`, aksi stop membutuhkan verifikasi sudo aktif. |

---

### 1.5 Terminal Web & AI Copilot

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/terminal/ws` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Upgrade: websocket` |
| Query Params | Tidak ada |
| Request Body | Stream byte I/O PTY dan pesan JSON resize `{ "cols": 80, "rows": 24 }` |
| Response | HTTP 101 Switching Protocols |
| Notes | Membutuhkan sudo mode aktif jika membuka shell root. Maksimal 3 sesi simultan per user. Idle timeout 15 menit. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/ai/chat` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `Accept: text/event-stream` |
| Query Params | Tidak ada |
| Request Body | `conversation_id` (int, opsional), `prompt` (string, wajib) |
| Response | HTTP 200 SSE stream token jawaban, tool-call status, dan rekomendasi tombol aksi |
| Notes | Tool read-only (maksimal 8 panggilan). Data log dan konfigurasi diredaksi otomatis sebelum dikirim ke penyedia LLM. |

---

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/overview` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"system": {...}, "counts": {"sites_total": 0, "services_active": 3}, "latest_metric": {...}}` |
| Notes | Menyajikan ringkasan agregat server, spesifikasi kernel, jumlah website aktif, dan metrik terkini. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/sites/{id}/toggle` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "id": 1, "enabled": 0}` |
| Notes | Mengaktifkan atau menonaktifkan konfigurasi vhost situs secara on/off atomik. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/audit` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `limit` (int, default: 50) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"logs": [{"id": 1, "ts": 1771747200, "actor": "user", "action": "site.create", "ok": 1}]}` |
| Notes | Riwayat audit kejadian keamanan sistem. Retensi 90 hari. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/terminal/exec` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `command` (string, wajib) |
| Response | HTTP 200 `{"ok": true, "command": "uptime", "output": "..."}` |
| Notes | Konsol perintah terminal terisolasi aman dengan validasi allowlist perintah sistem. |

---

### 1.6 Manajemen Berkas (File Manager)

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/files/list` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `path` (string, opsional, default: root aplikasi), `search` (string, opsional) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"current_path": "...", "parent_path": "...", "items": [...], "total_dirs": 7, "total_files": 13, "total_size": 981571}` |
| Notes | Menampilkan isi direktori dengan metadata nama, ukuran, permissions (mode), owner, waktu modifikasi, dan ekstensi. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/files/read` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `path` (string, wajib), `download` (int, opsional, `1` untuk attachment) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "path": "...", "content": "..."}` atau stream file biner |
| Notes | Membaca konten teks untuk editor internal (batas 5 MB) atau streaming unduhan langsung. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/files/save` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `path` (string, wajib), `content` (string, wajib) |
| Response | HTTP 200 `{"ok": true, "message": "berkas berhasil disimpan", "path": "..."}` |
| Notes | Menyimpan perubahan isi berkas teks langsung ke disk dengan pencatatan audit log otomatis. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/files/new` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `parent_path` (string, wajib), `name` (string, wajib), `type` (string: "file" atau "dir") |
| Response | HTTP 200 `{"ok": true, "path": "...", "name": "...", "type": "file"}` |
| Notes | Membuat berkas baru atau folder baru dengan sanitasi karakter ilegal. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/files/rename` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `path` (string, wajib), `new_name` (string, wajib) |
| Response | HTTP 200 `{"ok": true, "path": "...", "name": "..."}` |
| Notes | Mengganti nama berkas atau folder dengan proteksi penimpaan file yang ada. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/files/delete` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `paths` (array string, wajib) |
| Response | HTTP 200 `{"ok": true, "deleted": 1, "errors": []}` |
| Notes | Menghapus satu atau beberapa berkas/folder secara rekursif dengan proteksi terhadap root drive. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/files/chmod` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `path` (string, wajib), `mode` (string oktal, misal "0755") |
| Response | HTTP 200 `{"ok": true, "path": "...", "mode": "0755"}` |
| Notes | Mengubah izin hak akses berkas (POSIX chmod) via angka oktal. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/files/upload` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `multipart/form-data`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Form fields: `destination` (string), `files` (file multipart binary) |
| Response | HTTP 200 `{"ok": true, "uploaded": 2, "files": ["app.js", "style.css"]}` |
| Notes | Mengunggah satu atau beberapa berkas secara bersamaan (maks 100 MB). |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/files/paste` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `action` (string: "copy" atau "cut"), `source_paths` (array string), `destination_dir` (string) |
| Response | HTTP 200 `{"ok": true, "processed": 1, "action": "copy", "errors": []}` |
| Notes | Operasi penempelan berkas hasil salin atau potong (move) dengan penanganan fallback direktori rekursif. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/files/compress` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `paths` (array string), `zip_name` (string), `dest_dir` (string) |
| Response | HTTP 200 `{"ok": true, "path": "...", "name": "archive.zip"}` |
| Notes | Mengompresi daftar berkas dan folder secara rekursif menjadi berkas arsip .zip via stdlib archive/zip. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/files/remote-download` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `url` (string, wajib), `dest_dir` (string, wajib), `filename` (string, opsional) |
| Response | HTTP 200 `{"ok": true, "path": "...", "filename": "...", "bytes": 102400}` |
| Notes | Mengunduh berkas dari URL publik langsung ke direktori target di server. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/files/symlink` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `target_path` (string, wajib), `link_name` (string, wajib), `dest_dir` (string, wajib) |
| Response | HTTP 200 `{"ok": true, "path": "...", "name": "..."}` |
| Notes | Membuat tautan simbolik (soft link) POSIX/OS filesystem. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/files/search-content` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `dir` (string, wajib), `query` (string, wajib) |
| Response | HTTP 200 `{"ok": true, "matches": [{"file": "...", "line": 10, "text": "..."}], "count": 1}` |
| Notes | Memindai isi berkas teks di dalam direktori target untuk menemukan kata kunci dengan batas hasil 50 baris. |

---

## 2. Tugas Terjadwal (Cron Jobs & Background Tasks)

| Perintah / Fungsi | Frekuensi | Tujuan | Efek Samping |
|---|---|---|---|
| `acme.RenewLoop` | 1x sehari (pukul 02:00) | Memeriksa seluruh sertifikat terdaftar di `certificates`; memperpanjang via Lego HTTP-01 jika sisa masa aktif < 30 hari | Me-reload Nginx saat sertifikat baru terpasang; mencatat error pada DB jika DNS/jaringan gagal |
| `store.CleanupRoutine` | 1x sehari (pukul 03:00) | Menghapus baris audit log berumur > 90 hari dan pesan obrolan AI berumur > 30 hari | Menjaga ukuran database `/var/lib/panel/panel.db` tetap ringkas di bawah 10 MB |
| `store.BackupRoutine` | 1x sehari (pukul 04:00) | Eksekusi `VACUUM INTO` untuk snapshot SQLite ke direktori `/var/lib/panel/backups/panel-<timestamp>.db` | Menahan penulisan DB selama hitungan milidetik; mempertahankan rotasi 7 file backup terbaru |
