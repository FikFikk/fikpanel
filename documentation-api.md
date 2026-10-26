# Dokumentasi Spesifikasi API & Tugas Terjadwal (fikpanel)

## 1. Daftar Endpoint API

### 1.0 Pemeriksaan Kesehatan (Health Check)

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /healthz` |
| Auth Required | No |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"status": "ok"}` |
| Notes | Endpoint liveness probe publik untuk load balancer, container orchestrator, atau pemantau status dev. |

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
| Method + Endpoint | `GET /api/overview` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"system": {"hostname": "ip-172-31-45-195", "os": "Ubuntu 24.04", "arch": "amd64", "version": "0.1.3", "uptime_sec": 3600}, "sites_count": 0, "databases_count": 0, "cron_count": 0, "latest_metric": {...}}` |
| Notes | Menyediakan ringkasan status sistem, ringkasan jumlah resource, metrik terkini, dan nomor versi rilis FikPanel yang aktif. |

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
| Method + Endpoint | `GET /api/sites/meta` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"php_versions": ["8.3", "8.2"], "server_ips": ["203.0.113.10"]}` |
| Notes | Versi PHP dideteksi dari `/etc/php/*/fpm`. IP server = env `FIKPANEL_PUBLIC_IP` + IP publik (api.ipify.org, cache 1 jam) + IP interface. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/sites/check-domains` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `domains` (string, wajib, multi-baris, maks 4096 karakter), `add_www` (bool), `exclude_id` (int, opsional, untuk edit), `dns` (bool, sertakan resolusi DNS) |
| Response | HTTP 200 `{"primary", "aliases": [], "checks": [{"input", "domain", "wildcard", "auto", "error"}], "dns": [{"domain", "ips", "points_here", "error"}], "server_ips": [], "valid"}` |
| Notes | Dipakai validasi realtime form. `valid=false` bila ada format salah, duplikat, domain dipakai situs lain, atau tidak ada domain non-wildcard. Status DNS tidak memengaruhi `valid`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/sites` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `domains` (string, wajib, satu per baris), `add_www` (bool), `kind` (string, wajib: proxy/static/php), `upstream` (string, wajib jika proxy), `root_path` (string, opsional, default `/var/www/<domain>/public_html`), `php_version` (string `X.Y`, untuk php), `description` (string, maks 200) |
| Response | HTTP 201 objek `Site` (`id`, `domain`, `aliases`, `kind`, `root_path`, `ssl_status`=none, ...); HTTP 400 validasi gagal |
| Notes | Tanpa opsi SSL (SSL diaktifkan setelah dibuat). Membuat web root + index.html, render vhost port 80 + lokasi ACME, `nginx -t`, reload. Gagal = baris DB dihapus & file dipulihkan. Dicatat audit `site.create`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/sites/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"site": Site, "domains": [], "certificate": {"status", "issuer", "not_before", "not_after", "domains", "signature", "key_type", "last_error"}, "ssl_job": {"running", "step", "ok", "message"}}`; HTTP 404 |
| Notes | Dipoll UI tiap 1,5 detik selama `ssl_job.running=true`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `PUT /api/sites/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Sama dengan `POST /api/sites` (payload lengkap) |
| Response | HTTP 200 `{"ok": true, "site": Site, "ssl_reset": bool}` |
| Notes | Bila domain/alias berubah saat sertifikat `issued`, sertifikat dihapus (`ssl_reset=true`). Pergantian domain utama ikut mengganti nama file vhost. Gagal diterapkan = data DB dikembalikan. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/sites/{id}/toggle` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "id": 2, "enabled": 0}` |
| Notes | Nonaktif = file di-rename `.conf.disabled` lalu reload; gagal = status dikembalikan. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/sites/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "id": 2}` |
| Notes | Menghapus file vhost (aktif & disabled), reload Nginx, lalu baris DB (sertifikat ikut terhapus via cascade). Isi web root tidak dihapus. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/sites/{id}/vhost` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"content": "server { ... }"}` |
| Notes | Preview hanya-baca hasil render template sesuai pengaturan situs & status sertifikat saat ini. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/sites/{id}/ssl` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `enabled` (bool), `force_https` (bool), `hsts` (bool, diabaikan bila `force_https=false`), `issue` (bool, paksa terbit ulang), `skip_dns` (bool, lewati prasyarat DNS) |
| Response | HTTP 200 `{"ok": true, "issuing": bool, "ssl_job": {...}}`; HTTP 400 DNS belum siap / wildcard; HTTP 409 job masih berjalan |
| Notes | Penerbitan berjalan async (ACME HTTP-01 via `x/crypto/acme`, staging bila `FIKPANEL_ACME_STAGING=1`). Otomatis menerbitkan bila `enabled=true` dan belum `issued`. Mode non-Linux disimulasikan ±4 detik. Audit `ssl.settings` & `ssl.issue`. |

---

### 1.3a Nginx Manager

Semua endpoint: Auth Required = Yes (Cookie `panel_session`); request mutasi wajib header `Content-Type: application/json` + `X-Panel-Request: 1`; body maks 3 MB.

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/software/nginx/status` |
| Query Params / Body | Tidak ada |
| Response | HTTP 200 `{"installed", "running", "version", "pid", "uptime", "config_ok", ...}` |
| Notes | Sumber status badge Nginx di halaman Sites & Services. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/software/nginx/service` |
| Request Body | `action` (string, wajib: start/stop/restart/reload) |
| Response | HTTP 200 `{"ok": true, "status": {...}}`; HTTP 400 bila `nginx -t` gagal |
| Notes | start/restart/reload didahului uji sintaks `nginx -t`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET` / `POST /api/software/nginx/settings` |
| Request Body | `daemon` (bool, watchdog auto-start), `alert` (bool) |
| Response | HTTP 200 `{"settings": {"daemon", "alert"}}` |
| Notes | Disimpan pada tabel `settings`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET` / `POST /api/software/nginx/config` |
| Request Body | `content` (string, wajib, isi `nginx.conf`) |
| Response | GET: `{"path", "content"}`; POST: `{"ok": true}`; HTTP 400 sintaks salah |
| Notes | File sementara diuji, backup rotasi 10 file, rename atomik, reload. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/software/nginx/versions`, `POST /api/software/nginx/versions/switch` |
| Request Body | `version` (string, wajib, salah satu versi tersedia) |
| Response | GET: `{"current", "available": [], "job"}`; POST: `{"ok", "target"}` |
| Notes | Ganti versi via apt sebagai job async; progres dibaca dari `job`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/software/nginx/load` |
| Response | HTTP 200 metrik `stub_status` (active, accepts, handled, requests, reading, writing, waiting) + proses worker |
| Notes | Butuh `stub_status` pada localhost; data simulasi di non-Linux. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET` / `POST /api/software/nginx/optimize` |
| Request Body | Directive optimasi (`worker_processes`, `worker_connections`, `keepalive_timeout`, `client_max_body_size`, `server_names_hash_bucket_size`, gzip, dll.) |
| Response | GET: nilai saat ini; POST: `{"ok": true}`; HTTP 400 nilai di luar batas |
| Notes | `server_names_hash_bucket_size` wajib kelipatan 32. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/software/nginx/error-log`, `POST /api/software/nginx/error-log/clear` |
| Query Params | `lines` (int, opsional, jumlah baris akhir) |
| Response | GET: `{"path", "lines", "content"}`; POST: `{"ok": true}` |
| Notes | Clear = truncate file, bukan hapus. |

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
| Notes | Menampilkan status unit yang terdaftar di tabel `managed_services` dengan auto-discovery dinamis unit sistem nyata terpasang (Nginx, MySQL, PHP-FPM, Docker, Redis, SSH, dll). |

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

### 1.7 AI Copilot & Manajemen Provider LLM

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/ai/providers` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "providers": [{"id": 1, "name": "Omniroute", "kind": "openai_compat", "base_url": "https://...", "model": "gpt-4o", "api_key": "sk-••••••", "has_key": true, "is_default": true}]}` |
| Notes | Seluruh kunci API disensor otomatis demi keamanan (hanya menampilkan 3 karakter awal dan 4 karakter akhir). |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/ai/providers` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `name` (string, wajib), `kind` (string, wajib: `openai_compat`/`anthropic`/`gemini`), `base_url` (string, wajib), `model` (string, wajib), `api_key` (string, opsional), `is_default` (bool) |
| Response | HTTP 200 `{"ok": true, "provider": {...}}`; HTTP 400 validasi gagal |
| Notes | Provider pertama yang didaftarkan otomatis berstatus default. Mendukung gateway kustom (Omniroute, 9router, OpenAI, Groq, Ollama). |

| Field | Detail |
|---|---|
| Method + Endpoint | `PUT /api/ai/providers/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `name` (string), `kind` (string), `base_url` (string), `model` (string), `api_key` (string, kosongkan jika tidak diubah), `is_default` (bool) |
| Response | HTTP 200 `{"ok": true}` |
| Notes | Jika field `api_key` dikirim string kosong, kunci yang tersimpan sebelumnya tetap dipertahankan. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/ai/providers/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true}` |
| Notes | Menghapus provider. Bila provider yang dihapus berstatus default, sistem otomatis mengalihkan status default ke provider lain yang tersisa. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/ai/providers/{id}/default` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true}` |
| Notes | Menyetel provider sebagai provider aktif utama. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/ai/providers/test` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `id` (int, opsional), `kind` (string, wajib), `base_url` (string, wajib), `model` (string, wajib), `api_key` (string, opsional) |
| Response | HTTP 200 `{"ok": true, "latency_ms": 142, "reply": "PONG"}`; HTTP 400 koneksi gagal |
| Notes | Melakukan ping verifikasi konektivitas ke LLM sebelum konfigurasi disimpan. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/ai/conversations` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "conversations": [{"id": 1, "title": "Cek Nginx", "updated_at": 1792896000, "message_count": 4}]}` |
| Notes | Daftar percakapan terurut berdasarkan aktivitas waktu terbaru (`updated_at` descending). |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/ai/conversations` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `title` (string, opsional, default: 'Percakapan Baru') |
| Response | HTTP 200 `{"ok": true, "conversation": {...}}` |
| Notes | Membuat sesi percakapan kosong baru secara manual. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/ai/conversations/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true}` |
| Notes | Menghapus sesi percakapan dan seluruh pesan di dalamnya (CASCADE). |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/ai/conversations` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true}` |
| Notes | Menghapus seluruh riwayat percakapan milik pengguna aktif. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/ai/conversations/{id}/messages` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "conversation": {...}, "messages": [{"id": 1, "role": "user", "content": "...", "ts": 1792896000}]}` |
| Notes | Mengambil kronologi riwayat pesan dalam satu percakapan. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/ai/chat` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `conversation_id` (int, opsional), `message` (string, wajib), `provider_id` (int, opsional) |
| Response | HTTP 200 `{"ok": true, "conversation_id": 1, "response": "...", "provider_name": "Omniroute", "model": "gpt-4o", "actions": [{"label": "Restart Nginx", "action": "service.restart", "target": "nginx.service"}]}` |
| Notes | Jika `conversation_id` 0, otomatis membuat percakapan baru dengan judul dari 35 karakter awal pesan. Bila provider gagal/offline, otomatis fallback ke diagnostik lokal. Seluruh pesan diredaksi dari pola secret (BR-04). |

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

### 1.10 Header Keamanan HTTP Global (Security Headers)

Setiap respon dari backend panel menyematkan serangkaian header pengamanan HTTP berlapis:

| Header | Nilai | Penjelasan |
|---|---|---|
| `X-Content-Type-Options` | `nosniff` | Mencegah MIME sniffing oleh peramban |
| `X-Frame-Options` | `DENY` | Mencegah serangan clickjacking (halaman dilarang dimuat dalam iframe) |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Mengurangi kebocoran data URL rujukan |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` | Menonaktifkan sensor perangkat sensitif di aplikasi |
| `Cross-Origin-Opener-Policy` | `same-origin` | Isolasi proses peramban untuk mencegah eksploitasi Spectre/meltdown |
| `Cross-Origin-Resource-Policy` | `same-origin` | Mencegah aset dibaca oleh domain asing |
| `Content-Security-Policy` *(statis)* | `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'` | Menjamin hanya skrip, font, dan aset internal yang dapat dieksekusi |

---

### 1.11 Manajemen Database & Antrean Tugas (Databases & Task Queue)

Semua endpoint: Auth Required = Yes (Cookie `panel_session`); request mutasi wajib menyertakan header `Content-Type: application/json` dan `X-Panel-Request: 1`.

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/databases/engines` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json` |
| Query Params | `mock_db` (string, opsional, `1` / `0` untuk simulasi dev) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"engines": [{"name": "mysql", "display_name": "MySQL / MariaDB", "installed": true, "version": "10.11.6", "running": true, "port": 3306}, {"name": "mongodb", "installed": false, ...}]}` |
| Notes | Digunakan oleh frontend untuk merender status badge dan menentukan apakah antarmuka menampilkan state "Belum Terkonfigurasi" atau tabel database. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/databases/install` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `engine` (string, wajib: `mongodb` / `mysql` / `postgresql` / `redis`), `version` (string, wajib: `6.0` / `7.0` / `8.0` dll), `mode` (string: `native` / `docker`) |
| Response | HTTP 202 `{"ok": true, "task_id": "task_abc123", "message": "Proses instalasi dimulai"}` |
| Notes | Mendaftarkan pekerjaan instalasi ke `task_queue` SQLite dan mengeksekusi worker di latar belakang. Mengembalikan `task_id` untuk streaming log. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/system/tasks/:id/stream` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Accept: text/event-stream` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 SSE `data: {"type": "log", "chunk": "..."}\n\ndata: {"type": "status", "status": "success"}\n\n` |
| Notes | Mengalirkan output terminal eksekusi instalasi secara real-time ke modal Task Manager / Messages Box di browser. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/databases/:engine/list` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json` |
| Query Params | `search` (string, opsional) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"databases": [{"name": "app_db", "user": "app_user", "size_bytes": 10485760, "backup_count": 2, "location": "localhost", "note": "Production DB"}]}` |
| Notes | Menghasilkan daftar database aktif dari engine terkait. Menolak jika engine belum terpasang dengan HTTP 412 Precondition Failed. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/databases/:engine/create` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `name` (string, wajib), `username` (string, wajib), `password` (string, wajib), `access_host` (string, default `localhost`), `note` (string, opsional) |
| Response | HTTP 201 `{"ok": true, "database": "app_db", "user": "app_user"}` |
| Notes | Membuat database, akun user, dan memberikan hak akses grant secara otomatis. Dicatat ke audit log. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/databases/:engine/:name` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Database berhasil dihapus"}` |
| Notes | Menghapus database dan user terkait. Memerlukan konfirmasi modal destruktif di frontend. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/databases/:engine/service` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `action` (string, wajib: `start`, `stop`, `restart`, `reload`) |
| Response | HTTP 200 `{"ok": true, "engine": "mysql", "action": "restart", "running": true, "message": "Layanan berhasil di-restart"}` |
| Notes | Mengontrol status proses daemon database pada host OS systemd. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/databases/:engine/switch-version` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `version` (string, wajib), `backup_first` (bool, default `true`) |
| Response | HTTP 202 `{"ok": true, "task_id": "task_switch_mysql_...", "message": "Proses migrasi versi sedang berjalan..."}` |
| Notes | Menjalankan tugas migrasi versi di latar belakang. Jika `backup_first=true`, sistem mengekspor cadangan snapshot seluruh database otomatis sebelum mengganti paket biner. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/databases/:engine/config` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "engine": "mysql", "path": "/etc/mysql/my.cnf", "content": "..."}` |
| Notes | Mengambil isi berkas konfigurasi utama mesin database. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/databases/:engine/config` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `content` (string, wajib) |
| Response | HTTP 200 `{"ok": true, "message": "Konfigurasi berhasil disimpan"}` |
| Notes | Menyimpan pembaruan konfigurasi daemon dan mencatat aksi ke riwayat audit. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/databases/:engine/logs` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "engine": "mysql", "logs": "..."}` |
| Notes | Menyajikan potongan log galat (error log) terbaru mesin database. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/databases/webgui/status` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "installed": true, "software": "phpMyAdmin / FikPanel Web Client", "port": 888, "panel_host": "127.0.0.1:1405", "available_domains": [{"domain": "kostash.my.id", "ssl": true, "url": "https://kostash.my.id/phpmyadmin/"}], "has_domains": true}` |
| Notes | Memeriksa ketersediaan antarmuka visual database, daftar domain situs aktif untuk akses via domain, dan port server. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/databases/webgui/sso` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `database` (string, opsional), `username` (string, opsional), `password` (string, opsional), `access_mode` (`"panel"` / `"domain"` / `"custom_port"`), `target_domain` (string, opsional mis. `"kostash.my.id"`), `custom_port` (int, opsional mis. 888) |
| Response | HTTP 200 `{"ok": true, "installed": true, "access_mode": "panel", "url": "http://127.0.0.1:1405/phpmyadmin/index.php?route=/database/structure&db=...&user=...&sso_token=...", "token": "pma_sso_..."}` |
| Notes | Menghasilkan token login otomatis sekali pakai (One-Click SSO) dengan parameter rute resmi phpMyAdmin 5. Mendukung mode domain situs (port 80/443), mode panel independen (`/phpmyadmin/index.php` pada port panel), atau port khusus. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/databases/webgui/toggle-public` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `enabled` (bool, wajib) |
| Response | HTTP 200 `{"ok": true, "public_access": true, "message": "Akses publik phpMyAdmin berhasil diaktifkan"}` |
| Notes | Mengatur pembukaan port ke jaringan publik atau membatasinya hanya untuk akses lokal / panel. |

| Field | Detail |
|---|---|
| Method + Endpoint | `ALL /pma/` atau `ALL /phpmyadmin/` atau `ALL /phpMyAdmin/` |
| Auth Required | Yes (Session cookie atau parameter query `sso_token`) |
| Headers | `Accept: text/html, application/json, text/css, */*` |
| Query Params | `route` (string, mis. `/database/structure`), `db` (string, nama database), `user` (string, username database), `pma_username` (string), `sso_token` (string, token SSO) |
| Request Body | Mendukung seluruh payload HTTP (GET, POST form submission login phpMyAdmin, AJAX query) |
| Response | HTTP 200 Mengalirkan (proxying) dokumen HTML resmi, aset tema, CSS, skrip JavaScript, serta cookie sesi asli dari daemon phpMyAdmin fisik (`127.0.0.1:8885`). Mengembalikan HTTP 503 jika paket belum terpasang. |
| Notes | Proxy transparan langsung ke daemon resmi phpMyAdmin tanpa mock/clone buatan. Menangani normalisasi casing folder Windows (`phpMyAdmin`) dan Linux (`phpmyadmin`), serta pemetaan cookie path otomatis. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /pma/api/tables` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "tables": ["users", "sites", "databases", ...]}` |
| Notes | Mengambil daftar tabel yang ada dalam database sistem aktif. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /pma/api/query` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json` |
| Query Params | Tidak ada |
| Request Body | `query` (string SQL, wajib) |
| Response | HTTP 200 `{"ok": true, "columns": ["id", "username"], "rows": [[1, "admin"]], "count": 1, "time_ms": 0.8}` |
| Notes | Mengeksekusi perintah SQL secara interaktif dan mengembalikan hasil grid data. |

---

### 1.9 Manajemen Docker & Kontainerisasi (10 Modul Lengkap)

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/status` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"installed": true, "compose_installed": true, "running": true, "version": "26.1.4", "compose_version": "v2.27.1", "containers_total": 4, "containers_running": 3, "images_total": 6, "volumes_total": 5, "networks_total": 3}` |
| Notes | Mengembalikan status kesehatan daemon Docker, versi biner, dan agregasi kuantitas resource. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/system-info` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"hostname": "node-01", "os_version": "Ubuntu 24.04 LTS", "architecture": "x86_64", "kernel_version": "6.8.0-31", "cpu_cores": 8, "memory_size": "16 GB", "unix_address": "unix:///var/run/docker.sock", "data_directory": "/var/lib/docker"}` |
| Notes | Mengambil spesifikasi lengkap host sistem untuk tab Overview dan Settings. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/config` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"registry_mirrors": ["https://hub-mirror.c.163.com"], "compose_path": "/usr/local/bin/docker-compose", "log_rotation": true, "log_max_size": "10m", "log_max_file": "3", "raw_json": "{\n  \"log-driver\": \"json-file\"\n}"}` |
| Notes | Membaca berkas konfigurasi daemon resmi `/etc/docker/daemon.json`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/config` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `DockerDaemonConfig` (objek konfigurasi daemon.json lengkap atau `raw_json`) |
| Response | HTTP 200 `{"ok": true, "message": "Konfigurasi Docker berhasil disimpan"}` |
| Notes | Menyimpan dan menerapkan konfigurasi daemon ke disk serta mencatat audit log. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/service/{action}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Path param `{action}`: `start`, `stop`, `restart`, `repair` |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Layanan Docker berhasil di-restart"}` |
| Notes | Mengontrol unit systemd daemon Docker host. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/install` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "task_id": "task_docker_install_1771747900", "message": "Tugas instalasi Docker telah dikirim ke antrean tugas (Messages Box)"}` |
| Notes | Mengotomatisasi instalasi paket Docker CE dan Docker Compose melalui background task worker. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/containers` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "containers": [{"id": "a1b2c3d4", "names": ["/web"], "image": "nginx:alpine", "state": "running", "ports": [{"public_port": 8080, "private_port": 80}], "cpu_percent": 0.5, "mem_usage": "24.5 MB"}]}` |
| Notes | Mengambil daftar seluruh kontainer aktif maupun berhenti beserta metrik utilisasinya. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/containers` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `name` (string), `image` (string), `ports` (array string), `volumes` (array string), `environment` (array string), `restart_policy` (string), `command` (string) |
| Response | HTTP 200 `{"ok": true, "message": "Kontainer berhasil dibuat dan dijalankan"}` |
| Notes | Membuat kontainer baru dan langsung menjalankannya dengan opsi port mapping dan volume persisten. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/containers/{id}/{action}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Path param `{id}`: ID kontainer; `{action}`: `start`, `stop`, `restart`, `pause`, `unpause`, `delete` |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Aksi restart pada kontainer berhasil"}` |
| Notes | Mengontrol siklus hidup kontainer; aksi delete memerlukan konfirmasi modal pengguna. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/containers/{id}/logs` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `lines` (int, default: 200) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "logs": "[2026-10-25 20:00:00] Starting service..."}` |
| Notes | Mengambil cuplikan log kontainer dari driver log daemon. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/images` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "images": [{"id": "sha256:...", "repository": "nginx", "tag": "alpine", "size_human": "45.2 MB", "containers": 1}]}` |
| Notes | Menampilkan seluruh citra yang telah tersimpan di repositori lokal. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/images/pull` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `image` (string, misal: `"redis:7-alpine"`) |
| Response | HTTP 200 `{"ok": true, "message": "Citra redis:7-alpine berhasil diunduh"}` |
| Notes | Menarik citra resmi dari registri Docker Hub atau registri privat aktif. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/docker/images/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | `force` (bool, opsional) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Citra berhasil dihapus"}` |
| Notes | Menghapus citra dari penyimpanan lokal host. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/images/search` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `q` (string kata kunci pencarian) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "results": [{"name": "postgres", "description": "The PostgreSQL object-relational database...", "stars": 14000, "is_official": true}]}` |
| Notes | Melakukan kueri langsung ke Docker Hub Registry API untuk menemukan citra komunitas dan resmi. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/volumes` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "volumes": [{"name": "db_data", "driver": "local", "mountpoint": "/var/lib/docker/volumes/db_data/_data", "size_human": "240 MB", "ref_count": 1}]}` |
| Notes | Mengembalikan daftar volume persisten host beserta referensi keterikatannya. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/volumes` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `name` (string, wajib), `driver` (string, default: `"local"`) |
| Response | HTTP 200 `{"ok": true, "message": "Volume berhasil dibuat"}` |
| Notes | Menginisialisasi volume baru pada sistem berkas host. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/docker/volumes/{name}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Volume berhasil dihapus"}` |
| Notes | Menghapus volume persisten dari sistem berkas host setelah konfirmasi pengguna. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/volumes/prune` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Volume tak terpakai berhasil dibersihkan"}` |
| Notes | Menjalankan perintah prune untuk mengosongkan seluruh volume yang tidak lagi digunakan kontainer. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/networks` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "networks": [{"id": "net1", "name": "app-net", "driver": "bridge", "subnet": "172.28.0.0/16", "gateway": "172.28.0.1", "containers_count": 2}]}` |
| Notes | Menampilkan seluruh jaringan virtual Docker aktif. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/networks` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `name` (string, wajib), `driver` (string), `subnet` (string), `gateway` (string) |
| Response | HTTP 200 `{"ok": true, "message": "Network berhasil dibuat"}` |
| Notes | Membuat jaringan virtual baru dengan isolasi subnet khusus. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/docker/networks/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Network berhasil dihapus"}` |
| Notes | Menghapus jaringan virtual non-bawaan dari host. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/compose` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "projects": [{"name": "web-stack", "status": "running", "config_path": "/www/dk_project/web-stack/docker-compose.yml", "containers_count": 2}]}` |
| Notes | Mengembalikan seluruh direktori proyek Docker Compose terdaftar di `/www/dk_project`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/compose/up` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `name` (string, wajib), `yaml` (string YAML, wajib) |
| Response | HTTP 200 `{"ok": true, "message": "Stack Compose berhasil dijalankan"}` |
| Notes | Menyimpan berkas `docker-compose.yml` ke direktori proyek dan mengeksekusi `docker compose up -d`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/compose/down` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `name` (string, wajib) |
| Response | HTTP 200 `{"ok": true, "message": "Stack Compose berhasil dihentikan"}` |
| Notes | Mengeksekusi `docker compose down` pada direktori proyek. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/templates` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "templates": [{"id": "wordpress", "name": "WordPress + MariaDB", "category": "cms", "default_port": 8080, "env_params": [...]}]}` |
| Notes | Mengambil katalog template aplikasi 1-klik terisolasi (WordPress, Redis, Postgres, MySQL, Nginx, Portainer, Uptime Kuma, dll.). |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/templates/deploy` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `template_id` (string), `name` (string), `port` (int), `volume_path` (string), `env_values` (object) |
| Response | HTTP 200 `{"ok": true, "message": "Aplikasi berhasil dipasang dan berjalan"}` |
| Notes | Memasang dan menyalakan aplikasi instan dengan parameter port dan volume yang dikonfigurasi pengguna. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/docker/registries` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "registries": [{"id": "reg_default", "name": "Docker Hub Official", "url": "https://index.docker.io/v1/", "is_default": true}]}` |
| Notes | Mengembalikan daftar kredensial registri privat yang telah didaftarkan. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/docker/registries` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `name` (string, wajib), `url` (string, wajib), `username` (string), `password` (string) |
| Response | HTTP 200 `{"ok": true, "message": "Registri berhasil ditambahkan"}` |
| Notes | Mendaftarkan akun autentikasi registri kontainer privat baru. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/docker/registries/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Registri berhasil dihapus"}` |
| Notes | Menghapus registri privat terdaftar. |

### System Process & Task Manager

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/system/processes` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `limit` (int, opsional, default: 30) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "processes": [{"pid": 1234, "user": "www-data", "cpu": 1.5, "memory": 45.2, "command": "php-fpm: pool www"}]}` |
| Notes | Mengambil daftar proses sistem aktif yang diurutkan berdasarkan konsumsi CPU tertinggi. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/system/processes/{pid}/kill` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | `force` (bool, opsional, default: false) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "pid": 1234}` |
| Notes | Mengirim sinyal SIGTERM atau SIGKILL (`force=true`) untuk menghentikan proses target. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/system/ports` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "ports": [{"proto": "tcp", "port": "80", "address": "0.0.0.0", "program": "nginx", "pid": 800}]}` |
| Notes | Memeriksa seluruh soket jaringan aktif dan layanan yang mendengarkan (listening). |

### System Cleaner & Optimization

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/system/cleaner/scan` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "scan": {"apt_cache_bytes": 104857600, "system_log_bytes": 52428800, "tmp_bytes": 20971520, "docker_reclaim_bytes": 0, "total_bytes": 178257920}}` |
| Notes | Memindai estimasi ruang disk yang dapat dibebaskan dari APT cache, rotasi berkas log usang, dan berkas `/tmp`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/system/cleaner/clean` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `clean_apt` (bool), `clean_logs` (bool), `clean_tmp` (bool), `drop_caches` (bool) |
| Response | HTTP 200 `{"ok": true, "message": "Pembersihan sistem berhasil diselesaikan"}` |
| Notes | Mengeksekusi pembersihan selektif dan sinkronisasi disk cache (`drop_caches`). |

### Software App Store & Toolkit Installer

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/appstore/catalog` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "apps": [{"id": "redis", "name": "Redis Server", "category": "database", "installed": true, "version": "7.0.15"}]}` |
| Notes | Mengembalikan katalog aplikasi bawaan OS dan toolkit 1-klik beserta status instalasinya. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/appstore/install/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "task_id": "app_inst_123"}` |
| Notes | Memulai instalasi paket aplikasi di latar belakang via sistem antrean task worker. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/appstore/tasks/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "task": {"id": "app_inst_123", "status": "running", "progress": 80, "log": "..."}}` |
| Notes | Memeriksa kemajuan eksekusi tugas instalasi aplikasi. |

### Persistent System Monitor History & Control

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/monitor/config` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "config": {"enabled": true, "retention_days": 30}}` |
| Notes | Mengambil konfigurasi perekaman metrik persisten historis (mirip fitur aaPanel monitor). |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/monitor/config` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `enabled` (bool, wajib), `retention_days` (int, 1-365) |
| Response | HTTP 200 `{"ok": true, "config": {"enabled": true, "retention_days": 30}}` |
| Notes | Mengaktifkan/menonaktifkan perekam interval background metrik CPU, RAM, Disk, dan Jaringan. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/monitor/history` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `range` (string: `"1h"`, `"6h"`, `"24h"`, `"7d"`, `"30d"`) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "snapshots": [{"timestamp": 1740000000, "cpu": 12.4, "ram_used": 1024, "ram_total": 4096, "disk_used": 20480, "disk_total": 81920, "net_in": 10240, "net_out": 20480, "load_1m": 0.45}]}` |
| Notes | Mengambil deret waktu metrik untuk visualisasi grafik multi-rentang. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/monitor/history` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Riwayat pemantauan berhasil dibersihkan"}` |
| Notes | Mengosongkan seluruh tabel rekaman snapshot metrik persisten. |

### Keamanan, Firewall, & SSH Hardening

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/firewall/status` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "status": {"active": true, "backend": "ufw", "rules": [{"number": 1, "port": "22", "proto": "tcp", "action": "ALLOW", "from": "Anywhere"}]}}` |
| Notes | Menampilkan status UFW/iptables dan daftar aturan port aktif. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/firewall/toggle` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `enabled` (bool, wajib) |
| Response | HTTP 200 `{"ok": true, "enabled": true}` |
| Notes | Menyalakan atau mematikan layanan firewall UFW host. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/firewall/rules` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `port` (string, misal `"1405"`), `proto` (string: `"tcp"`, `"udp"`, atau `"any"`), `action` (string: `"allow"` atau `"deny"`) |
| Response | HTTP 200 `{"ok": true}` |
| Notes | Menambahkan aturan penerusan port baru pada firewall. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/firewall/rules/{number}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true}` |
| Notes | Menghapus aturan firewall berdasarkan nomor urut index rule UFW. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/security/ssh` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "ssh": {"port": 22, "password_auth": true, "permit_root_login": false}}` |
| Notes | Membaca konfigurasi `/etc/ssh/sshd_config` aktif pada server. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/security/ssh` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `port` (int), `password_auth` (bool), `permit_root_login` (bool) |
| Response | HTTP 200 `{"ok": true, "message": "Konfigurasi SSH berhasil disimpan dan di-reload"}` |
| Notes | Memodifikasi parameter port dan autentikasi `sshd_config`, kemudian melakukan reload layanan SSH. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/security/fail2ban` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "fail2ban": {"active": true, "jails": ["sshd"], "banned_ips": [{"jail": "sshd", "ip": "192.168.1.100"}]}}` |
| Notes | Memeriksa status proteksi brute-force Fail2Ban beserta daftar IP yang sedang diblokir. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/security/fail2ban/unban` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `jail` (string, misal `"sshd"`), `ip` (string IP address) |
| Response | HTTP 200 `{"ok": true}` |
| Notes | Membuka blokir (unban) alamat IP tertentu dari jail Fail2Ban. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/security/scan` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "report": {"score": 85, "checks": [{"name": "SSH Root Login", "status": "passed", "risk": "low"}]}}` |
| Notes | Menjalankan inspeksi audit keamanan menyeluruh terhadap sistem operasi, port terbuka, hak akses berkas, dan konfigurasi server. |

### Node.js & PM2 Process Manager

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/node/status` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "status": {"node_installed": true, "node_version": "v24.7.0", "pm2_installed": true, "pm2_version": "7.1.1"}}` |
| Notes | Memeriksa ketersediaan biner runtime Node.js dan daemon PM2 di host. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/node/processes` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "status": {...}, "processes": [{"id": 0, "name": "my-app", "status": "online", "pid": 1234, "cpu": 0.5, "memory": 45000000, "uptime": 1740000000, "restart_count": 0, "mode": "fork"}]}` |
| Notes | Mengambil daftar proses nyata dari output `pm2 jlist` (mengembalikan array kosong jika PM2 belum terpasang atau belum ada proses). |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/node/processes/start` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `name` (string, wajib), `script` (string, wajib, misal `"server.js"`), `cwd` (string), `instances` (string, default `"1"`), `watch` (bool), `args` (string) |
| Response | HTTP 200 `{"ok": true, "message": "Aplikasi PM2 berhasil dijalankan"}` |
| Notes | Menjalankan aplikasi Node.js baru dengan PM2 dan menyimpan konfigurasi ke `pm2 save`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/node/processes/{id}/{action}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "target": "0", "action": "restart"}` |
| Notes | Mengontrol siklus hidup proses PM2: `restart`, `stop`, `start`, `reload`, atau `delete` (otomatis mengeksekusi `pm2 save` saat menghapus). |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/node/processes/{id}/logs` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `lines` (int, default: 60) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "target": "0", "logs": "..."}` |
| Notes | Mengambil baris log gabungan stdout dan stderr langsung dari daemon PM2. |

---

### 1.14 Host Infrastructure Auto-Discovery & Reverse Adoption

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/discovery/config` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "roots": ["/home/fikfikk/projects", "/var/www"]}` |
| Notes | Mengambil daftar direktori kustom yang dikonfigurasi untuk pemindaian file compose dan proyek host. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/discovery/config` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `roots` (array of string, contoh: `["/home/fikfikk/projects"]`) |
| Response | HTTP 200 `{"ok": true, "message": "...", "roots": [...]}` |
| Notes | Menyimpan direktori pencarian kustom ke tabel `settings`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/discovery/scan` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `custom_roots` (opsional, array of string) |
| Response | HTTP 200 `{"ok": true, "result": {"nginx_vhosts": [...], "docker_projects": [...], "pm2_apps": [...], "kubernetes_detected": bool, "kubernetes_type": "k3s"}}` |
| Notes | Memindai menyeluruh konfigurasi vhost Nginx host (`/etc/nginx/**`), stack docker compose, proses PM2 aktif, dan status cluster K8s. Vhost yang sudah ada di database otomatis ditandai `is_adopted: true`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/discovery/adopt` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `sites` (array of object: `primary_domain`, `aliases`, `document_root`, `has_php`, `proxy_pass`, `has_ssl`) |
| Response | HTTP 200 `{"ok": true, "adopted_sites": 2, "message": "..."}` |
| Notes | Mengadopsi vhost host yang belum terdaftar ke dalam tabel `sites` SQLite dengan mempertahankan path document root asli tanpa merombak struktur folder server. |

---

### 1.15 Kubernetes & K3s Orchestrator

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/k8s/status` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Response | HTTP 200 `{"ok": true, "status": {"available": true, "cluster_type": "k3s", "server_version": "v1.28.2", "nodes_count": 1, "pods_count": 5, "nodes": [...]}}` |
| Notes | Memeriksa ketersediaan koneksi ke cluster K3s, MicroK8s, atau Kubernetes standar. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/k8s/nodes` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Response | HTTP 200 `{"ok": true, "nodes": [{"name": "node-1", "status": "Ready", "roles": "control-plane,master", "version": "v1.28.2", "os_image": "Ubuntu 22.04"}]}` |
| Notes | Menampilkan ringkasan status node worker dan control plane cluster. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/k8s/namespaces` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Response | HTTP 200 `{"ok": true, "namespaces": [{"name": "default", "status": "Active", "age": "10d"}]}` |
| Notes | Menampilkan daftar seluruh namespace isolasi cluster. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/k8s/pods` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `namespace` (opsional string, kosong = semua namespace) |
| Response | HTTP 200 `{"ok": true, "pods": [{"name": "web-pod-xxx", "namespace": "default", "status": "Running", "ready": "1/1", "restarts": 0, "age": "2h", "ip": "10.42.0.12"}]}` |
| Notes | Mengambil daftar kontainer pod yang berjalan di cluster. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/k8s/deployments` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `namespace` (opsional string) |
| Response | HTTP 200 `{"ok": true, "deployments": [{"name": "web-app", "namespace": "default", "ready_replicas": 2, "replicas": 2, "age": "3d", "images": ["nginx:alpine"]}]}` |
| Notes | Mengambil daftar beban kerja deployment yang dapat di-scale. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/k8s/services` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `namespace` (opsional string) |
| Response | HTTP 200 `{"ok": true, "services": [{"name": "web-svc", "namespace": "default", "type": "ClusterIP", "cluster_ip": "10.43.0.1", "external_ip": "-", "ports": "80/TCP"}]}` |
| Notes | Menampilkan abstraksi exposure service jaringan internal dan eksternal. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/k8s/deployments/{name}/scale` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `namespace` (string, default: "default"), `replicas` (int, non-negatif) |
| Response | HTTP 200 `{"ok": true, "message": "Deployment demo-web berhasil di-scale menjadi 3 replika"}` |
| Notes | Mengubah jumlah replika Pod yang berjalan secara instan via `kubectl scale`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/k8s/apply` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `manifest` (string YAML deklaratif, wajib), `namespace` (opsional string) |
| Response | HTTP 200 `{"ok": true, "message": "Manifest berhasil diterapkan ke cluster", "output": "..."}` |
| Notes | Menerapkan definisi resource YAML langsung ke cluster via `kubectl apply -f -`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/k8s/pods/{name}/logs` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `namespace` (string, default: "default"), `container` (opsional string), `tail` (int, default: 100) |
| Response | HTTP 200 `{"ok": true, "logs": "..."}` |
| Notes | Membaca riwayat baris log kontainer pod secara real-time via `kubectl logs`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/k8s/resources` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `kind` (string, wajib: `"pod"`, `"deployment"`, atau `"service"`), `name` (string, wajib), `namespace` (string) |
| Response | HTTP 200 `{"ok": true, "message": "Resource deployment 'web' berhasil dihapus"}` |
| Notes | Menghapus resource dari cluster via `kubectl delete <kind> <name> -n <namespace>`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/k8s/install-k3s` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Instalasi K3s telah dimulai di latar belakang..."}` |
| Notes | Menjalankan skrip resmi Rancher K3s (`curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644`) di background. |

### 1.13 Penjadwal Tugas (Cron Manager)

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/cron/jobs` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "jobs": [...], "server_time": {"unix": 1771747800, "iso": "...", "timezone_name": "UTC", "offset_sec": 0}}` |
| Notes | Mengembalikan daftar tugas cron aktif beserta waktu server dan offset untuk sinkronisasi timezone klien. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/cron/jobs` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `name` (string, wajib), `schedule_type` (string), `spec` (string crontab 5-kolom, wajib), `command` (string, wajib), `timeout_sec` (int), `save_log` (bool) |
| Response | HTTP 201 `{"ok": true, "job": {...}}` |
| Notes | Menjadwalkan tugas cron baru. Mendukung pola 5-kolom crontab (`m h dom mon dow`). |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/cron/jobs/{id}/toggle` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `enabled` (bool, wajib) |
| Response | HTTP 200 `{"ok": true, "id": 1, "enabled": false}` |
| Notes | Mengaktifkan atau menjeda eksekusi cron job tertentu tanpa menghapus data. |

| Field | Detail |
|---|---|
| Method + Endpoint | `DELETE /api/cron/jobs/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "id": 1}` |
| Notes | Menghapus tugas cron dan riwayat log eksekusinya. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/cron/jobs/{id}/run` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "log": {"status": "success", "exit_code": 0, "duration_ms": 120, "output": "..."}}` |
| Notes | Memicu eksekusi tugas cron secara instan di luar siklus jadwal dan mencatat hasil eksekusi. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/cron/jobs/{id}/logs` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "logs": [...]}` |
| Notes | Mengembalikan hingga 30 riwayat eksekusi terakhir dari tugas cron tertentu. |

### 1.14 PHP Multi-Version & FPM Tuning

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/php/versions` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "versions": [{"version": "8.5", "installed": true, "running": true, "fpm_socket": "/run/php/php8.5-fpm.sock", "settings": {...}}]}` |
| Notes | Memindai versi PHP dinamis di host OS (8.5, 8.4, 8.3, dst.) beserta status FPM socket dan konfigurasi php.ini. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/php/versions/{version}/settings` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `key` (string, wajib: `memory_limit`, `upload_max_filesize`, dll), `value` (string, wajib) |
| Response | HTTP 200 `{"ok": true, "version": "8.5", "key": "memory_limit", "value": "512M"}` |
| Notes | Memperbarui parameter direktif pada berkas `php.ini` FPM dan me-reload layanan secara otomatis. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/php/versions/{version}/restart` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "PHP 8.5 FPM berhasil direstart"}` |
| Notes | Me-restart daemon FPM versi spesifik via systemctl (`php8.5-fpm`). |

### 1.15 App Store & Repositori Global Linux

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/appstore/catalog` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "apps": [...]}` |
| Notes | Mengembalikan katalog paket unggulan web stack (Nginx, PHP, MariaDB, Redis, Node, dll.) dengan status deteksi real-time. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/appstore/search` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | `q` (string, wajib: kata kunci pencarian paket Linux) |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "packages": [{"name": "htop", "description": "...", "installed": true}]}` |
| Notes | Mencari paket di seluruh repositori resmi OS (APT) via `apt-cache search` dan memeriksa status via `dpkg-query`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/appstore/install/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Instalasi telah dimulai di background"}` |
| Notes | Menjalankan instalasi paket software di latar belakang via APT. Mendukung prefix `pkg:<nama_paket>` untuk paket global. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/appstore/uninstall/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "message": "Pencopotan paket telah dimulai di background"}` |
| Notes | Menghapus paket dari sistem via `apt-get remove -y` di latar belakang. Mendukung prefix `pkg:<nama_paket>`. |

| Field | Detail |
|---|---|
| Method + Endpoint | `GET /api/appstore/tasks/{id}` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | Tidak ada |
| Query Params | Tidak ada |
| Request Body | Tidak ada |
| Response | HTTP 200 `{"ok": true, "task": {"app_id": "...", "status": "running/success/failed", "logs": "..."}}` |
| Notes | Membaca output baris terminal instalasi/pencopotan paket software yang sedang berjalan di latar belakang. |

---

## 2. Tugas Terjadwal (Cron Jobs & Background Tasks)

| `fikpanel renew-ssl` (Default Cron Job) | Setiap hari pukul 02:00 (`0 2 * * *`) | Memindai seluruh domain dan memperpanjang sertifikat SSL Let's Encrypt / ACME yang tersisa < 30 hari | Menulis sertifikat atomik, reload Nginx, mencatat output eksekusi ke `cron_logs` dan audit log `ssl.renew` |
| `sites.RunRenewLoop` | Setiap 12 jam (sejak panel start & saat boot) | Memperpanjang sertifikat `issued` dengan sisa masa aktif < 30 hari via ACME HTTP-01 | Status sementara `pending`, file cert ditulis atomik, Nginx di-reload; audit `ssl.renew`; gagal = `failed` + `last_error` |
| `nginx.RunWatchdog` | Setiap 1 menit (sejak panel start) | Bila setting `daemon=true` dan Nginx mati, jalankan ulang setelah lolos `nginx -t` | Nginx otomatis start; dihentikan saat panel shutdown (context dibatalkan) |
| `srv.StartMonitoringRecorder` | Setiap 60 detik (background ticker) & setiap 24 jam (purge) | Merekam metrik CPU, RAM, Disk, Jaringan jika fitur monitor aktif, serta membersihkan snapshot metrik melebihi masa retensi | Menyimpan data historis ke tabel `monitor_snapshots`; menghapus snapshot lama secara berkala |
| `srv.cronMgr.Start` | Setiap menit (sesuai spesifikasi cron) | Mengeksekusi antrean tugas cron kustom pengguna yang terdaftar di database | Menjalankan skrip shell/perintah cadangan atau pemeliharaan berkala |
| `store.CleanupRoutine` | 1x sehari (pukul 03:00) | Menghapus baris audit log berumur > 90 hari dan pesan obrolan AI berumur > 30 hari | Menjaga ukuran database `/var/lib/panel/panel.db` tetap ringkas di bawah 10 MB |
| `store.BackupRoutine` | 1x sehari (pukul 04:00) | Eksekusi `VACUUM INTO` untuk snapshot SQLite ke direktori `/var/lib/panel/backups/panel-<timestamp>.db` | Menahan penulisan DB selama hitungan milidetik; mempertahankan rotasi 7 file backup terbaru |
