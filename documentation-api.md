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
| Response | HTTP 200 `{"ok": true, "installed": true, "software": "phpMyAdmin / FikPanel Web Client", "port": 888, "panel_host": "127.0.0.1:8443", "available_domains": [{"domain": "kostash.my.id", "ssl": true, "url": "https://kostash.my.id/phpmyadmin/"}], "has_domains": true}` |
| Notes | Memeriksa ketersediaan antarmuka visual database, daftar domain situs aktif untuk akses via domain, dan port server. |

| Field | Detail |
|---|---|
| Method + Endpoint | `POST /api/databases/webgui/sso` |
| Auth Required | Yes (Cookie `panel_session`) |
| Headers | `Content-Type: application/json`, `X-Panel-Request: 1` |
| Query Params | Tidak ada |
| Request Body | `database` (string, opsional), `username` (string, opsional), `password` (string, opsional), `access_mode` (`"panel"` / `"domain"` / `"custom_port"`), `target_domain` (string, opsional mis. `"kostash.my.id"`), `custom_port` (int, opsional mis. 888) |
| Response | HTTP 200 `{"ok": true, "installed": true, "access_mode": "panel", "url": "http://127.0.0.1:8443/phpmyadmin/index.php?route=/database/structure&db=...&user=...&sso_token=...", "token": "pma_sso_..."}` |
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
| Method + Endpoint | `GET /pma/` atau `GET /phpmyadmin/` |
| Auth Required | Yes (Session cookie atau parameter query `sso_token`) |
| Headers | `Accept: text/html` |
| Query Params | `route` (string, mis. `/database/structure`), `db` (string, nama database), `user` (string, username database), `sso_token` (string, token SSO) |
| Request Body | Tidak ada |
| Response | HTTP 200 Dokumen HTML antarmuka phpMyAdmin resmi (logo sailboat, Language selector, kartu Log in dengan prefilled username, dan workspace tabel setelah login). |
| Notes | Berfungsi sebagai fallback lokal maupun antarmuka phpMyAdmin bawaan di port panel, sehingga tidak terjadi `ERR_CONNECTION_REFUSED` dan konsisten dengan phpMyAdmin resmi. |

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

## 2. Tugas Terjadwal (Cron Jobs & Background Tasks)

| Perintah / Fungsi | Frekuensi | Tujuan | Efek Samping |
|---|---|---|---|
| `sites.RunRenewLoop` | Setiap 12 jam (sejak panel start) | Memperpanjang sertifikat `issued` dengan sisa masa aktif < 30 hari via ACME HTTP-01 | Status sementara `pending`, file cert ditulis atomik, Nginx di-reload; audit `ssl.renew`; gagal = `failed` + `last_error` |
| `nginx.RunWatchdog` | Setiap 1 menit (sejak panel start) | Bila setting `daemon=true` dan Nginx mati, jalankan ulang setelah lolos `nginx -t` | Nginx otomatis start; dihentikan saat panel shutdown (context dibatalkan) |
| `store.CleanupRoutine` | 1x sehari (pukul 03:00) | Menghapus baris audit log berumur > 90 hari dan pesan obrolan AI berumur > 30 hari | Menjaga ukuran database `/var/lib/panel/panel.db` tetap ringkas di bawah 10 MB |
| `store.BackupRoutine` | 1x sehari (pukul 04:00) | Eksekusi `VACUUM INTO` untuk snapshot SQLite ke direktori `/var/lib/panel/backups/panel-<timestamp>.db` | Menahan penulisan DB selama hitungan milidetik; mempertahankan rotasi 7 file backup terbaru |
