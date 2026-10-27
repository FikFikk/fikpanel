package sites

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureErrorPages memastikan folder error_pages sistem ada dan berisi template standar FikPanel
func EnsureErrorPages(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("gagal membuat folder error pages: %w", err)
	}

	files := map[string]string{
		"403.html":     tpl403,
		"404.html":     tpl404,
		"50x.html":     tpl50x,
		"welcome.html": tplWelcome,
	}

	for name, content := range files {
		target := filepath.Join(dir, name)
		// Selalu perbarui agar template sistem tetap sinkron dengan rilis terbaru
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return fmt.Errorf("gagal menulis template %s: %w", name, err)
		}
	}
	return nil
}

// Tpl403 mengembalikan template HTML untuk galat 403
func Tpl403() string { return tpl403 }

// Tpl404 mengembalikan template HTML untuk galat 404
func Tpl404() string { return tpl404 }

// Tpl50x mengembalikan template HTML untuk galat 500/502/503/504
func Tpl50x() string { return tpl50x }

// TplWelcome mengembalikan template HTML halaman sambutan 200 OK
func TplWelcome() string { return RenderWelcomePage("contoh-domain.com") }

// RenderWelcomePage merender halaman sambutan dengan domain yang ditentukan
func RenderWelcomePage(domain string) string {
	if domain == "" {
		domain = "contoh-domain.com"
	}
	return strings.ReplaceAll(tplWelcome, "{{DOMAIN}}", domain)
}

// tplHeadStyles menyediakan CSS bersama untuk seluruh template galat dan welcome
const tplHeadStyles = `
    :root {
      --bg: #07080c;
      --bg-card: #0d0f17;
      --bg-surface: #131622;
      --border: rgba(255, 255, 255, 0.08);
      --border-subtle: rgba(255, 255, 255, 0.04);
      --text: #f1f5f9;
      --text-muted: #94a3b8;
      --text-dim: #64748b;
      --accent: #38bdf8;
      --warn: #f59e0b;
      --danger: #ef4444;
      --ok: #10b981;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: var(--bg);
      color: var(--text);
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Plus Jakarta Sans', sans-serif;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      padding: 1.5rem;
      background-image: 
        radial-gradient(circle at 50% 0%, rgba(30, 41, 59, 0.4) 0%, transparent 65%),
        linear-gradient(rgba(255, 255, 255, 0.018) 1px, transparent 1px),
        linear-gradient(90deg, rgba(255, 255, 255, 0.018) 1px, transparent 1px);
      background-size: 100% 100%, 32px 32px, 32px 32px;
    }
    .container {
      width: 100%;
      max-width: 720px;
      display: flex;
      flex-direction: column;
      gap: 1.25rem;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      gap: 0.5rem;
      padding: 0.35rem 0.75rem;
      border-radius: 9999px;
      background: var(--bg-surface);
      border: 1px solid var(--border);
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 0.74rem;
      color: var(--text-muted);
      width: fit-content;
    }
    .dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: var(--warn);
      box-shadow: 0 0 8px var(--warn);
    }
    .dot.danger { background: var(--danger); box-shadow: 0 0 8px var(--danger); }
    .dot.ok { background: var(--ok); box-shadow: 0 0 8px var(--ok); }
    .hero {
      display: flex;
      flex-direction: column;
      gap: 0.6rem;
    }
    .error-code {
      font-size: 3.5rem;
      font-weight: 800;
      letter-spacing: -0.05em;
      line-height: 1;
      color: var(--text);
      display: flex;
      align-items: baseline;
      gap: 0.75rem;
      flex-wrap: wrap;
    }
    .error-title {
      font-size: 1.35rem;
      font-weight: 600;
      color: var(--text-muted);
      letter-spacing: -0.02em;
    }
    .desc {
      font-size: 0.92rem;
      color: var(--text-muted);
      line-height: 1.6;
    }
    .desc code {
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      background: var(--bg-surface);
      padding: 0.15rem 0.4rem;
      border-radius: 4px;
      border: 1px solid var(--border);
      color: var(--accent);
    }
    .actions {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      flex-wrap: wrap;
      margin-top: 0.25rem;
    }
    button, a.btn {
      display: inline-flex;
      align-items: center;
      gap: 0.45rem;
      padding: 0.55rem 1rem;
      border-radius: 8px;
      font-size: 0.82rem;
      font-weight: 600;
      cursor: pointer;
      text-decoration: none;
      transition: all 0.15s ease;
      font-family: inherit;
    }
    .btn-main {
      background: #f8fafc;
      color: #0f172a;
      border: none;
    }
    .btn-main:hover {
      background: #ffffff;
      transform: translateY(-1px);
    }
    .btn-outline {
      background: var(--bg-card);
      color: var(--text);
      border: 1px solid var(--border);
    }
    .btn-outline:hover {
      background: var(--bg-surface);
      border-color: rgba(255,255,255,0.2);
    }
    .card {
      background: var(--bg-card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 1.25rem;
      display: flex;
      flex-direction: column;
      gap: 1rem;
    }
    .card-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 0.7rem;
      color: var(--text-dim);
      border-bottom: 1px solid var(--border-subtle);
      padding-bottom: 0.65rem;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    .telemetry-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: 0.85rem;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    }
    .tel-item {
      display: flex;
      flex-direction: column;
      gap: 0.25rem;
    }
    .tel-label {
      font-size: 0.66rem;
      color: var(--text-dim);
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }
    .tel-val {
      font-size: 0.8rem;
      color: var(--text);
      word-break: break-all;
    }
    .tel-val.warn { color: var(--warn); }
    .tel-val.danger { color: var(--danger); }
    .troubleshoot {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: 0.85rem;
      margin-top: 0.25rem;
    }
    .ts-box {
      background: var(--bg-surface);
      border: 1px solid var(--border-subtle);
      border-radius: 8px;
      padding: 0.85rem;
      display: flex;
      flex-direction: column;
      gap: 0.35rem;
    }
    .ts-num {
      font-family: ui-monospace, SFMono-Regular, monospace;
      font-size: 0.7rem;
      color: var(--accent);
      font-weight: 700;
    }
    .ts-title {
      font-size: 0.8rem;
      font-weight: 600;
      color: var(--text);
    }
    .ts-desc {
      font-size: 0.74rem;
      color: var(--text-muted);
      line-height: 1.45;
    }
    .toast {
      position: fixed;
      bottom: 2rem;
      background: #10b981;
      color: #064e3b;
      padding: 0.5rem 1rem;
      border-radius: 8px;
      font-size: 0.8rem;
      font-weight: 600;
      opacity: 0;
      pointer-events: none;
      transform: translateY(10px);
      transition: all 0.2s ease;
      z-index: 999;
    }
    .toast.show {
      opacity: 1;
      transform: translateY(0);
    }
    footer {
      font-size: 0.7rem;
      color: var(--text-dim);
      text-align: center;
      font-family: ui-monospace, SFMono-Regular, monospace;
    }
`

// tplScript menyediakan logika JS untuk diagnostik dinamis dan copy
const tplScript = `
  <div id="toast" class="toast">✓ Data diagnostik disalin ke clipboard</div>
  <script>
    const hostEl = document.getElementById('tel-host');
    const pathEl = document.getElementById('tel-path');
    const timeEl = document.getElementById('tel-time');
    const rayEl = document.getElementById('tel-ray');
    const descHostEl = document.getElementById('desc-host');

    const curHost = window.location.hostname || 'localhost';
    const curPath = window.location.pathname || '/';
    const rayId = Math.random().toString(16).substring(2, 10) + Math.random().toString(16).substring(2, 10);

    if (hostEl) hostEl.textContent = curHost;
    if (descHostEl) descHostEl.textContent = curHost;
    if (pathEl) pathEl.textContent = curPath;
    if (timeEl) timeEl.textContent = new Date().toISOString().replace('T', ' ').substring(0, 19) + ' UTC';
    if (rayEl) rayEl.textContent = rayId;

    function copyDiagnostic(code) {
      const payload = {
        status_code: code,
        host: curHost,
        path: curPath,
        time: new Date().toISOString(),
        user_agent: navigator.userAgent,
        trace_id: rayId,
        engine: 'Nginx / FikPanel Edge'
      };
      navigator.clipboard.writeText(JSON.stringify(payload, null, 2)).then(() => {
        const t = document.getElementById('toast');
        t.classList.add('show');
        setTimeout(() => t.classList.remove('show'), 2200);
      });
    }

    function goBack() {
      if (window.history.length > 1) {
        window.history.back();
      } else {
        window.location.href = '/';
      }
    }
  </script>
`

// tpl404 adalah template 404 Halaman Tidak Ditemukan
const tpl404 = `<!doctype html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>404 Halaman Tidak Ditemukan</title>
  <style>` + tplHeadStyles + `</style>
</head>
<body>
  <div class="container">
    <div class="badge">
      <span class="dot"></span>
      <span>HTTP 404 NOT FOUND · Edge Router</span>
    </div>

    <div class="hero">
      <div class="error-code">
        <span>404</span>
        <span class="error-title">Halaman Tidak Ditemukan</span>
      </div>
      <p class="desc">
        URL atau berkas yang Anda minta tidak tersedia di direktori web hosting server <code id="desc-host">domain</code>.
      </p>
    </div>

    <div class="actions">
      <button class="btn-main" onclick="window.location.reload()">Muat Ulang Halaman</button>
      <button class="btn-outline" onclick="copyDiagnostic(404)">Salin Diagnostik</button>
      <button class="btn-outline" onclick="goBack()">Kembali</button>
    </div>

    <div class="card">
      <div class="card-head">
        <span>Server Telemetry & Diagnostic Payload</span>
        <span>Protocol: HTTP/2 · TLS 1.3</span>
      </div>
      <div class="telemetry-grid">
        <div class="tel-item">
          <span class="tel-label">Target Host</span>
          <span class="tel-val" id="tel-host">Memuat…</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Response Code</span>
          <span class="tel-val warn">404 Not Found</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Request Path</span>
          <span class="tel-val" id="tel-path">/</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Timestamp</span>
          <span class="tel-val" id="tel-time">—</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Trace / Ray ID</span>
          <span class="tel-val" id="tel-ray">—</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Web Server Engine</span>
          <span class="tel-val">Nginx / FikPanel Edge</span>
        </div>
      </div>
    </div>

    <div class="troubleshoot">
      <div class="ts-box">
        <span class="ts-num">01</span>
        <span class="ts-title">Akses File Manager</span>
        <span class="ts-desc">Pastikan berkas <code>index.html</code> atau <code>index.php</code> telah diunggah ke folder root situs.</span>
      </div>
      <div class="ts-box">
        <span class="ts-num">02</span>
        <span class="ts-title">Kustomisasi Halaman</span>
        <span class="ts-desc">Untuk memakai tampilan Anda sendiri, buat berkas <code>error/404.html</code> di root proyek.</span>
      </div>
      <div class="ts-box">
        <span class="ts-num">03</span>
        <span class="ts-title">Periksa URL Tujuan</span>
        <span class="ts-desc">Pastikan ejaan tautan atau rute yang dimasukkan pada peramban sudah tepat dan terdaftar.</span>
      </div>
    </div>

    <footer>FikPanel Hosting Engine · High Performance Edge Architecture</footer>
  </div>
` + tplScript + `
</body>
</html>
`

// tpl403 adalah template 403 Akses Ditolak
const tpl403 = `<!doctype html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>403 Akses Ditolak</title>
  <style>` + tplHeadStyles + `</style>
</head>
<body>
  <div class="container">
    <div class="badge">
      <span class="dot danger"></span>
      <span>HTTP 403 FORBIDDEN · Security Guard</span>
    </div>

    <div class="hero">
      <div class="error-code">
        <span>403</span>
        <span class="error-title">Akses Ditolak</span>
      </div>
      <p class="desc">
        Server menolak permintaan Anda untuk mengakses direktori atau berkas di <code id="desc-host">domain</code> karena pembatasan hak akses.
      </p>
    </div>

    <div class="actions">
      <button class="btn-main" onclick="window.location.reload()">Muat Ulang Halaman</button>
      <button class="btn-outline" onclick="copyDiagnostic(403)">Salin Diagnostik</button>
      <button class="btn-outline" onclick="goBack()">Kembali</button>
    </div>

    <div class="card">
      <div class="card-head">
        <span>Server Telemetry & Diagnostic Payload</span>
        <span>Protocol: HTTP/2 · TLS 1.3</span>
      </div>
      <div class="telemetry-grid">
        <div class="tel-item">
          <span class="tel-label">Target Host</span>
          <span class="tel-val" id="tel-host">Memuat…</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Response Code</span>
          <span class="tel-val danger">403 Forbidden</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Request Path</span>
          <span class="tel-val" id="tel-path">/</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Timestamp</span>
          <span class="tel-val" id="tel-time">—</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Trace / Ray ID</span>
          <span class="tel-val" id="tel-ray">—</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Web Server Engine</span>
          <span class="tel-val">Nginx / FikPanel Edge</span>
        </div>
      </div>
    </div>

    <div class="troubleshoot">
      <div class="ts-box">
        <span class="ts-num">01</span>
        <span class="ts-title">Izin Berkas Linux</span>
        <span class="ts-desc">Pastikan folder memiliki izin akses <code>0755</code> dan berkas <code>0644</code> milik user/group yang benar.</span>
      </div>
      <div class="ts-box">
        <span class="ts-num">02</span>
        <span class="ts-title">Berkas Indeks Kosong</span>
        <span class="ts-desc">Nginx menonaktifkan directory listing otomatis jika berkas <code>index.html</code> atau <code>index.php</code> tidak ditemukan.</span>
      </div>
      <div class="ts-box">
        <span class="ts-num">03</span>
        <span class="ts-title">Aturan Akses Nginx</span>
        <span class="ts-desc">Periksa tab Limit Access atau konfigurasi vhost jika ada pemblokiran IP atau otentikasi basic auth.</span>
      </div>
    </div>

    <footer>FikPanel Hosting Engine · High Performance Edge Architecture</footer>
  </div>
` + tplScript + `
</body>
</html>
`

// tpl50x adalah template 500/502/503/504 Server & Gateway Error
const tpl50x = `<!doctype html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>502 Gateway Bermasalah</title>
  <style>` + tplHeadStyles + `</style>
</head>
<body>
  <div class="container">
    <div class="badge">
      <span class="dot danger"></span>
      <span id="badge-text">HTTP 502 BAD GATEWAY · Upstream Offline</span>
    </div>

    <div class="hero">
      <div class="error-code">
        <span id="code-val">502</span>
        <span class="error-title" id="title-val">Gateway Bermasalah</span>
      </div>
      <p class="desc" id="desc-text">
        Web server Nginx tidak dapat terhubung ke aplikasi backend di server <code id="desc-host">domain</code>. Layanan aplikasi kemungkinan sedang dimatikan atau belum aktif.
      </p>
    </div>

    <div class="actions">
      <button class="btn-main" onclick="window.location.reload()">Muat Ulang Halaman</button>
      <button class="btn-outline" onclick="copyDiagnostic(502)">Salin Diagnostik</button>
      <button class="btn-outline" onclick="goBack()">Kembali</button>
    </div>

    <div class="card">
      <div class="card-head">
        <span>Server Telemetry & Diagnostic Payload</span>
        <span>Protocol: HTTP/2 · TLS 1.3</span>
      </div>
      <div class="telemetry-grid">
        <div class="tel-item">
          <span class="tel-label">Target Host</span>
          <span class="tel-val" id="tel-host">Memuat…</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Response Code</span>
          <span class="tel-val danger" id="tel-code">502 Bad Gateway</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Request Path</span>
          <span class="tel-val" id="tel-path">/</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Timestamp</span>
          <span class="tel-val" id="tel-time">—</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Trace / Ray ID</span>
          <span class="tel-val" id="tel-ray">—</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Web Server Engine</span>
          <span class="tel-val">Nginx / FikPanel Edge</span>
        </div>
      </div>
    </div>

    <div class="troubleshoot">
      <div class="ts-box">
        <span class="ts-num">01</span>
        <span class="ts-title">Aplikasi Upstream</span>
        <span class="ts-desc">Pastikan servis aplikasi (Node.js, Golang, Python, atau PHP-FPM) sedang berjalan dan listening pada port target.</span>
      </div>
      <div class="ts-box">
        <span class="ts-num">02</span>
        <span class="ts-title">Periksa Log Server</span>
        <span class="ts-desc">Periksa tab Response Logs di menu situs FikPanel untuk melihat catatan galat stderr dari aplikasi backend.</span>
      </div>
      <div class="ts-box">
        <span class="ts-num">03</span>
        <span class="ts-title">Halaman Pemeliharaan</span>
        <span class="ts-desc">Untuk mode maintenance terjadwal, Anda dapat mengaktifkan fitur Maintenance Mode dari pengaturan situs.</span>
      </div>
    </div>

    <footer>FikPanel Hosting Engine · High Performance Edge Architecture</footer>
  </div>
` + tplScript + `
</body>
</html>
`

// tplWelcome adalah template selamat datang ketika situs baru dibuat (200 OK)
const tplWelcome = `<!doctype html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{DOMAIN}} · Siap Digunakan</title>
  <style>` + tplHeadStyles + `</style>
</head>
<body>
  <div class="container">
    <div class="badge">
      <span class="dot ok"></span>
      <span>HTTP 200 OK · Virtual Host Ready</span>
    </div>

    <div class="hero">
      <div class="error-code">
        <span>{{DOMAIN}}</span>
      </div>
      <div class="error-title" style="color: var(--ok);">Situs Berhasil Dibuat & Aktif</div>
      <p class="desc">
        Virtual host untuk domain <code>{{DOMAIN}}</code> telah terdaftar di Nginx dan siap melayani pengunjung. Berkas sambutan bawaan ini dapat Anda ganti dengan proyek website Anda.
      </p>
    </div>

    <div class="card">
      <div class="card-head">
        <span>Host Environment & Server Status</span>
        <span>Engine: Nginx · FikPanel</span>
      </div>
      <div class="telemetry-grid">
        <div class="tel-item">
          <span class="tel-label">Domain Utama</span>
          <span class="tel-val">{{DOMAIN}}</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Status Server</span>
          <span class="tel-val" style="color: var(--ok);">Online · Siap Digunakan</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Protokol Trafik</span>
          <span class="tel-val">HTTP/2 · TLS 1.3</span>
        </div>
        <div class="tel-item">
          <span class="tel-label">Direktori Berkas</span>
          <span class="tel-val">Root Proyek / public</span>
        </div>
      </div>
    </div>

    <div class="troubleshoot">
      <div class="ts-box">
        <span class="ts-num">01</span>
        <span class="ts-title">Unggah Berkas</span>
        <span class="ts-desc">Buka menu File Manager di FikPanel, lalu unggah berkas HTML, PHP, atau build aset ke direktori situs ini.</span>
      </div>
      <div class="ts-box">
        <span class="ts-num">02</span>
        <span class="ts-title">Aktifkan SSL</span>
        <span class="ts-desc">Amankan situs Anda dengan sertifikat Let's Encrypt gratis sekali klik melalui menu Pengaturan Situs → SSL.</span>
      </div>
      <div class="ts-box">
        <span class="ts-num">03</span>
        <span class="ts-title">Halaman Kustom</span>
        <span class="ts-desc">Untuk menyediakan halaman galat sendiri, letakkan <code>error/404.html</code> atau <code>404.html</code> di folder proyek.</span>
      </div>
    </div>

    <footer>FikPanel Hosting Engine · High Performance Edge Architecture</footer>
  </div>
</body>
</html>
`
