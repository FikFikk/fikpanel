package sites

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureErrorPages memastikan folder error_pages sistem ada dan berisi template standar FikPanel Edge
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
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return fmt.Errorf("gagal menulis template %s: %w", name, err)
		}
	}
	return nil
}

// Tpl403 mengembalikan template HTML untuk galat 403 Forbidden
func Tpl403() string { return tpl403 }

// Tpl404 mengembalikan template HTML untuk galat 404 Not Found
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

// buildCleanEdgePage menghasilkan dokumen HTML mandiri tanpa dependensi eksternal
func buildCleanEdgePage(code, statusLabel, badgeText, badgeSub, dotColor, textColor, title, desc, hint, defaultDomain string) string {
	return `<!DOCTYPE html>
<html class="dark" lang="id">
<head>
  <meta charset="utf-8"/>
  <meta content="width=device-width, initial-scale=1.0" name="viewport"/>
  <title>` + code + ` — ` + title + ` | FikPanel Edge</title>
  <style>
    /* Reset & Variabel Warna Obsidian */
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    :root {
      --bg: #090a0f;
      --surface: #11131a;
      --raised: #171a23;
      --border: rgba(255, 255, 255, 0.08);
      --border-hover: rgba(255, 255, 255, 0.16);
      --text: #e2e8f0;
      --text-muted: #94a3b8;
      --text-dim: #64748b;
      --white: #ffffff;
      --status-color: ` + textColor + `;
    }

    body {
      background-color: var(--bg);
      color: var(--text);
      font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      position: relative;
      overflow-x: hidden;
      line-height: 1.5;
      -webkit-font-smoothing: antialiased;
    }

    /* Pendaran Latar Belakang Ambien */
    .hero-glow {
      position: fixed;
      inset: 0;
      background: radial-gradient(circle at 50% 45%, rgba(99, 102, 241, 0.08) 0%, rgba(6, 182, 212, 0.03) 40%, transparent 75%);
      pointer-events: none;
      z-index: 0;
    }

    /* Kontainer Utama Terpusat Vertikal & Horizontal */
    .main-wrap {
      position: relative;
      z-index: 10;
      flex: 1;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      text-align: center;
      padding: 3rem 1.25rem;
      min-height: calc(100vh - 60px);
    }

    .hero-card {
      width: 100%;
      max-width: 40rem;
      margin: 0 auto;
      display: flex;
      flex-direction: column;
      align-items: center;
    }

    /* Animasi Conic Rainbow Spin */
    @keyframes spin-rainbow {
      0% { transform: rotate(0deg); }
      100% { transform: rotate(360deg); }
    }

    .rainbow-pill-wrap {
      position: relative;
      padding: 1.5px;
      border-radius: 9999px;
      overflow: hidden;
      box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5);
      margin-bottom: 1.5rem;
      display: inline-flex;
      align-items: center;
      justify-content: center;
    }

    .rainbow-conic-spin {
      position: absolute;
      inset: -150%;
      background: conic-gradient(
        hsl(0, 95%, 65%),
        hsl(45, 95%, 65%),
        hsl(90, 95%, 65%),
        hsl(150, 95%, 65%),
        hsl(210, 95%, 65%),
        hsl(270, 95%, 65%),
        hsl(330, 95%, 65%),
        hsl(360, 95%, 65%)
      );
      animation: spin-rainbow 5s linear infinite;
      opacity: 0.9;
    }

    .rainbow-glow-ambient {
      position: absolute;
      inset: -20px;
      background: conic-gradient(
        from 180deg at 50% 50%,
        rgba(255, 99, 132, 0.35),
        rgba(255, 205, 86, 0.35),
        rgba(75, 192, 192, 0.35),
        rgba(54, 162, 235, 0.35),
        rgba(153, 102, 255, 0.35),
        rgba(255, 99, 132, 0.35)
      );
      animation: spin-rainbow 8s linear infinite;
      filter: blur(24px);
      opacity: 0.5;
    }

    .badge-content {
      position: relative;
      z-index: 10;
      display: inline-flex;
      align-items: center;
      gap: 0.5rem;
      padding: 0.4rem 0.9rem;
      border-radius: 9999px;
      background: rgba(13, 15, 22, 0.96);
      backdrop-filter: blur(12px);
      font-family: 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
      font-size: 0.75rem;
      color: var(--text);
    }

    .badge-dot {
      width: 0.5rem;
      height: 0.5rem;
      border-radius: 50%;
      background: ` + dotColor + `;
      flex-shrink: 0;
      box-shadow: 0 0 8px ` + dotColor + `;
    }

    .badge-title {
      font-weight: 600;
      letter-spacing: 0.05em;
      text-transform: uppercase;
    }

    .badge-sub {
      color: var(--text-dim);
      font-size: 0.7rem;
    }

    /* Tipografi Utama */
    .error-code {
      font-size: clamp(4.5rem, 12vw, 6.5rem);
      font-weight: 800;
      letter-spacing: -0.04em;
      color: var(--white);
      font-family: 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
      line-height: 1;
      user-select: none;
      margin-bottom: 0.75rem;
    }

    .error-title {
      font-size: clamp(1.4rem, 4vw, 1.85rem);
      font-weight: 600;
      letter-spacing: -0.02em;
      color: var(--white);
      margin-bottom: 0.75rem;
    }

    .error-desc {
      color: var(--text-muted);
      font-size: 0.92rem;
      line-height: 1.6;
      max-width: 32rem;
      margin: 0 auto;
    }

    .error-desc code {
      font-family: 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
      color: #f1f5f9;
      background: rgba(255, 255, 255, 0.06);
      border: 1px solid var(--border);
      padding: 0.15rem 0.4rem;
      border-radius: 4px;
      font-size: 0.8rem;
    }

    /* Tombol Aksi */
    .btn-group {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: center;
      gap: 0.75rem;
      margin-top: 2rem;
    }

    .btn {
      display: inline-flex;
      align-items: center;
      gap: 0.45rem;
      padding: 0.55rem 1rem;
      border-radius: 6px;
      font-size: 0.78rem;
      font-family: 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
      font-weight: 500;
      cursor: pointer;
      text-decoration: none;
      transition: all 0.15s ease;
    }

    .btn-primary {
      background: var(--white);
      color: #090a0f;
      border: 1px solid var(--white);
      box-shadow: 0 4px 12px rgba(255, 255, 255, 0.1);
    }
    .btn-primary:hover {
      background: #e2e8f0;
      transform: translateY(-1px);
    }

    .btn-surface {
      background: var(--surface);
      color: var(--text);
      border: 1px solid var(--border);
    }
    .btn-surface:hover {
      background: var(--raised);
      color: var(--white);
      border-color: var(--border-hover);
    }

    .btn-ghost {
      background: transparent;
      color: var(--text-muted);
      border: 1px solid transparent;
    }
    .btn-ghost:hover {
      background: rgba(255, 255, 255, 0.04);
      color: var(--text);
      border-color: var(--border);
    }

    /* Accordion Telemetri Diagnostik */
    .telemetry-container {
      width: 100%;
      max-width: 36rem;
      margin-top: 2rem;
      text-align: left;
    }

    .telemetry-toggle {
      width: 100%;
      padding: 0.65rem 1rem;
      border-radius: 6px;
      border: 1px solid var(--border);
      background: rgba(17, 19, 26, 0.7);
      color: var(--text-muted);
      font-size: 0.78rem;
      font-family: 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
      display: flex;
      align-items: center;
      justify-content: space-between;
      cursor: pointer;
      backdrop-filter: blur(8px);
      transition: all 0.15s ease;
    }
    .telemetry-toggle:hover {
      background: var(--surface);
      color: var(--text);
      border-color: var(--border-hover);
    }

    .telemetry-toggle-left {
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }

    .telemetry-drawer {
      margin-top: 0.75rem;
      display: flex;
      flex-direction: column;
      gap: 0.75rem;
    }
    .telemetry-drawer.hidden {
      display: none;
    }

    .grid-4 {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.5rem;
      font-family: 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
    }
    @media (min-width: 640px) {
      .grid-4 { grid-template-columns: repeat(4, minmax(0, 1fr)); }
    }

    .tel-card {
      padding: 0.6rem 0.65rem;
      border-radius: 6px;
      border: 1px solid var(--border);
      background: rgba(17, 19, 26, 0.9);
      min-width: 0;
      overflow: hidden;
    }

    .tel-label {
      font-size: 0.65rem;
      color: var(--text-dim);
      text-transform: uppercase;
      display: block;
      margin-bottom: 0.2rem;
      letter-spacing: 0.03em;
    }

    .tel-val {
      color: var(--text);
      font-size: 0.72rem;
      font-weight: 500;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      display: block;
    }
    .tel-val.status {
      color: var(--status-color);
      font-weight: 600;
    }

    .info-box {
      padding: 0.75rem 0.9rem;
      border-radius: 6px;
      border: 1px solid var(--border);
      background: rgba(17, 19, 26, 0.8);
      font-family: 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
      font-size: 0.75rem;
      color: var(--text-muted);
    }

    .info-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      border-bottom: 1px solid rgba(255, 255, 255, 0.05);
      padding-bottom: 0.35rem;
      margin-bottom: 0.4rem;
      font-size: 0.7rem;
      color: var(--text-dim);
    }
    .info-header strong {
      color: var(--text);
      font-weight: 400;
    }

    .guide-box {
      padding: 0.85rem;
      border-radius: 6px;
      border: 1px solid var(--border);
      background: rgba(17, 19, 26, 0.5);
      font-family: 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
      font-size: 0.75rem;
      display: flex;
      flex-direction: column;
      gap: 0.45rem;
    }

    .guide-title {
      font-size: 0.65rem;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--text-dim);
      font-weight: 600;
      margin-bottom: 0.2rem;
    }

    .guide-step {
      display: flex;
      align-items: flex-start;
      gap: 0.5rem;
      color: var(--text-muted);
      line-height: 1.45;
    }
    .guide-num {
      color: #818cf8;
      font-weight: 700;
      flex-shrink: 0;
    }
    .guide-step strong { color: var(--text); }
    .guide-step code {
      color: #f1f5f9;
      background: rgba(255, 255, 255, 0.06);
      padding: 0.1rem 0.3rem;
      border-radius: 3px;
    }

    /* Footer */
    footer {
      width: 100%;
      border-top: 1px solid var(--border);
      background: rgba(9, 10, 15, 0.9);
      padding: 0.9rem 1.25rem;
      margin-top: auto;
      font-family: 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
      font-size: 0.72rem;
      color: var(--text-dim);
    }

    .footer-inner {
      max-width: 64rem;
      margin: 0 auto;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: space-between;
      gap: 0.5rem;
    }
    @media (min-width: 640px) {
      .footer-inner { flex-direction: row; }
    }

    .footer-left {
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }
    .footer-dot {
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background: #10b981;
      display: inline-block;
    }

    .footer-links {
      display: flex;
      align-items: center;
      gap: 1.25rem;
    }
    .footer-links a {
      color: var(--text-muted);
      text-decoration: none;
      transition: color 0.15s ease;
    }
    .footer-links a:hover {
      color: var(--text);
    }
  </style>
</head>
<body>
  <!-- Pendaran Latar Belakang -->
  <div class="hero-glow"></div>

  <!-- Area Konten Utama -->
  <main class="main-wrap">
    <div class="hero-card">
      <!-- Badge Pill Pelangi Berputar -->
      <div class="rainbow-pill-wrap">
        <div class="rainbow-conic-spin"></div>
        <div class="rainbow-glow-ambient"></div>
        <div class="badge-content">
          <span class="badge-dot"></span>
          <span class="badge-title">` + badgeText + `</span>
          <span style="color: var(--text-dim);">•</span>
          <span class="badge-sub">` + badgeSub + `</span>
        </div>
      </div>

      <!-- Tipografi Status & Galat -->
      <h1 class="error-code">` + code + `</h1>
      <h2 class="error-title">` + title + `</h2>
      <p class="error-desc">` + desc + `</p>

      <!-- Tombol Aksi Utama -->
      <div class="btn-group">
        <a class="btn btn-primary" href="/" onclick="goHomeOrBack(event)">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5M12 19l-7-7 7-7"/></svg>
          <span>Kembali ke Beranda</span>
        </a>
        <button class="btn btn-surface" onclick="window.location.reload()">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/><path d="M16 21h5v-5"/></svg>
          <span>Muat Ulang Halaman</span>
        </button>
        <button class="btn btn-ghost" onclick="copyDiagnostics()">
          <svg id="copy-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          <span id="copy-text">Salin Info Diagnostik</span>
        </button>
      </div>

      <!-- Accordion Telemetri Server (Tertutup Secara Bawaan) -->
      <div class="telemetry-container">
        <button class="telemetry-toggle" onclick="toggleTelemetry()">
          <div class="telemetry-toggle-left">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#818cf8" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
            <span>Tampilkan Detail Diagnostik &amp; Server</span>
          </div>
          <svg id="telemetry-chevron" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>
        </button>

        <!-- Laci Diagnostik Tertutup Default (hidden) -->
        <div class="telemetry-drawer hidden" id="telemetry-drawer">
          <div class="grid-4">
            <div class="tel-card">
              <span class="tel-label">Host</span>
              <span class="tel-val domain-host-label" title="` + defaultDomain + `">` + defaultDomain + `</span>
            </div>
            <div class="tel-card">
              <span class="tel-label">Response Code</span>
              <span class="tel-val status" title="` + statusLabel + `">` + statusLabel + `</span>
            </div>
            <div class="tel-card">
              <span class="tel-label">Edge Node</span>
              <span class="tel-val" title="sg-edge-01 (SIN)">sg-edge-01 (SIN)</span>
            </div>
            <div class="tel-card">
              <span class="tel-label">Trace ID</span>
              <span class="tel-val" id="trace-id" title="8f3d9b4c0e">8f3d9b4c0e</span>
            </div>
          </div>

          <div class="info-box">
            <div class="info-header">
              <span>Web Server Engine: <strong>Nginx 1.28.x / FikPanel Edge</strong></span>
              <span>TLS: <strong>1.3</strong></span>
            </div>
            <div>` + hint + `</div>
          </div>

          <div class="guide-box">
            <div class="guide-title">Panduan Pemilik Situs</div>
            <div class="guide-step">
              <span class="guide-num">1.</span>
              <span>Buka menu <strong>File Manager</strong> pada panel FikPanel dan navigasi ke direktori root target situs.</span>
            </div>
            <div class="guide-step">
              <span class="guide-num">2.</span>
              <span>Unggah berkas aplikasi atau build website ke direktori publik yang sesuai.</span>
            </div>
            <div class="guide-step">
              <span class="guide-num">3.</span>
              <span>Pastikan permission berkas diatur ke <code>0644</code> dan direktori ke <code>0755</code>.</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </main>

  <!-- Footer Status & Jam UTC -->
  <footer>
    <div class="footer-inner">
      <div class="footer-left">
        <span class="footer-dot"></span>
        <span style="color: var(--text-muted);">FikPanel Cloud Infrastructure</span>
        <span style="color: var(--border);">·</span>
        <span id="utc-time">UTC --:--:--</span>
      </div>
      <div class="footer-links">
        <a href="https://github.com/fikfikk/fikpanel" target="_blank" rel="noopener noreferrer">Dokumentasi Edge</a>
        <a href="https://github.com/fikfikk/fikpanel/issues" target="_blank" rel="noopener noreferrer">Dukungan Teknis</a>
      </div>
    </div>
  </footer>

  <script>
    // Buka/tutup laci diagnostik
    function toggleTelemetry() {
      const drawer = document.getElementById('telemetry-drawer');
      const chevron = document.getElementById('telemetry-chevron');
      if (!drawer || !chevron) return;
      const isHidden = drawer.classList.contains('hidden');
      if (isHidden) {
        drawer.classList.remove('hidden');
        chevron.style.transform = 'rotate(180deg)';
      } else {
        drawer.classList.add('hidden');
        chevron.style.transform = 'rotate(0deg)';
      }
    }

    // Jam UTC langsung
    function updateClock() {
      const now = new Date();
      const utcString = now.toUTCString().split(' ')[4] + ' UTC';
      const el = document.getElementById('utc-time');
      if (el) el.textContent = utcString;
    }
    setInterval(updateClock, 1000);
    updateClock();

    // Navigasi kembali ke riwayat sebelumnya jika berasal dari host yang sama
    function goHomeOrBack(e) {
      if (window.history.length > 1 && document.referrer && new URL(document.referrer).hostname === window.location.hostname) {
        e.preventDefault();
        window.history.back();
      }
    }

    // Salin info diagnostik
    function copyDiagnostics() {
      const code = '` + code + `';
      const trace = document.getElementById('trace-id')?.textContent.trim() || '8f3d9b4c0e';
      const host = window.location.hostname || '` + defaultDomain + `';
      const node = 'sg-edge-01 (SIN)';
      const utc = document.getElementById('utc-time')?.textContent.trim() || 'UTC';

      const payload = '[FikPanel Edge Diagnostic]\n' +
        'Status: HTTP ' + code + '\n' +
        'Host: ' + host + '\n' +
        'Node: ' + node + '\n' +
        'Trace-ID: ' + trace + '\n' +
        'Timestamp: ' + utc + '\n' +
        'Engine: Nginx/1.28.x (FikPanel Edge)';

      navigator.clipboard.writeText(payload).then(() => {
        const copyText = document.getElementById('copy-text');
        if (copyText) copyText.textContent = 'Info Tersalin!';
        setTimeout(() => {
          if (copyText) copyText.textContent = 'Salin Info Diagnostik';
        }, 2000);
      });
    }

    // Inisialisasi awal hostname dinamis & trace ID 10 karakter rapi
    document.addEventListener('DOMContentLoaded', () => {
      const curHost = window.location.hostname || '` + defaultDomain + `';
      document.querySelectorAll('.domain-host-label').forEach(el => {
        el.textContent = curHost;
      });

      // Hasilkan Trace ID 10 karakter hex agar pas di dalam kartu tanpa terpotong
      const randomHex = Math.random().toString(16).substring(2, 12);
      const traceEl = document.getElementById('trace-id');
      if (traceEl) {
        traceEl.textContent = randomHex;
        traceEl.title = randomHex;
      }
    });
  </script>
</body>
</html>`
}

var (
	tpl404 = buildCleanEdgePage(
		"404",
		"404 Not Found",
		"HTTP 404 NOT FOUND",
		"Edge Router",
		"#fbbf24",
		"#fbbf24",
		"Halaman Tidak Ditemukan",
		`URL atau berkas yang Anda minta tidak tersedia di direktori web hosting server <code class="domain-host-label">contoh-domain.com</code>. Pastikan alamat URL yang dimasukkan sudah benar.`,
		`Pastikan berkas <code style="color:#f1f5f9;background:rgba(255,255,255,0.06);padding:0.1rem 0.3rem;border-radius:3px;">index.html</code> atau <code style="color:#f1f5f9;background:rgba(255,255,255,0.06);padding:0.1rem 0.3rem;border-radius:3px;">index.php</code> telah diunggah ke folder root target di FikPanel File Manager.`,
		"contoh-domain.com",
	)

	tpl403 = buildCleanEdgePage(
		"403",
		"403 Forbidden",
		"HTTP 403 FORBIDDEN",
		"Security Guard",
		"#fb7185",
		"#fb7185",
		"Akses Ditolak (Forbidden)",
		`Server memahami permintaan Anda, namun menolak untuk memberikan izin akses ke berkas atau direktori ini di host <code class="domain-host-label">contoh-domain.com</code>.`,
		`Periksa permission folder (standar 0755) atau periksa apakah aturan pembatasan akses / basic auth memblokir IP/request Anda.`,
		"contoh-domain.com",
	)

	tpl50x = buildCleanEdgePage(
		"502",
		"502 Bad Gateway",
		"HTTP 502 BAD GATEWAY",
		"Upstream Offline",
		"#fb7185",
		"#fb7185",
		"Bad Gateway (Koneksi Upstream Gagal)",
		`Server gateway Nginx menerima respon yang tidak valid dari proses backend hulu (PHP-FPM, Node.js port, atau backend daemon) di host <code class="domain-host-label">contoh-domain.com</code>.`,
		`Pastikan backend service (PHP-FPM pool atau application daemon port) berjalan aktif dan memori RAM server tidak melampaui kuota.`,
		"contoh-domain.com",
	)

	tplWelcome = buildCleanEdgePage(
		"200",
		"200 OK",
		"HTTP 200 OK — SITE READY",
		"Virtual Host Ready",
		"#34d399",
		"#34d399",
		"Situs Berhasil Dikonfigurasi",
		`Selamat! Domain telah aktif pada klaster FikPanel Edge. Silakan mulai mengunggah file produksi website Anda ke folder publik di host <code class="domain-host-label">{{DOMAIN}}</code>.`,
		`Hapus file halaman placeholder ini atau ganti dengan berkas aplikasi website Anda pada direktori publik / root situs.`,
		"{{DOMAIN}}",
	)
)
