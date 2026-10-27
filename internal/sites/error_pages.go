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
		// Selalu perbarui agar template sistem tetap sinkron dengan rilis terbaru
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

// buildEdgePageTemplate menghasilkan dokumen HTML tunggal lengkap persis sesuai mockup FikPanel Edge
func buildEdgePageTemplate(defaultCode, defaultDomain string) string {
	return `<!DOCTYPE html>
<html class="dark" lang="id">
<head>
  <meta charset="utf-8"/>
  <meta content="width=device-width, initial-scale=1.0" name="viewport"/>
  <title>` + defaultCode + ` — Halaman Status | FikPanel Edge</title>
  <script src="https://cdn.tailwindcss.com?plugins=forms,container-queries"></script>
  <link href="https://fonts.googleapis.com" rel="preconnect"/>
  <link crossorigin="" href="https://fonts.gstatic.com" rel="preconnect"/>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet"/>
  <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css" rel="stylesheet"/>
  <script id="tailwind-config">
    tailwind.config = {
      darkMode: "class",
      theme: {
        extend: {
          colors: {
            obsidian: {
              DEFAULT: '#090a0f',
              surface: '#11131a',
              raised: '#171a23',
              border: 'rgba(255, 255, 255, 0.08)',
              borderHover: 'rgba(255, 255, 255, 0.16)',
            }
          },
          fontFamily: {
            sans: ['Inter', '-apple-system', 'BlinkMacSystemFont', 'sans-serif'],
            mono: ['"JetBrains Mono"', 'SFMono-Regular', 'Menlo', 'monospace']
          }
        }
      }
    };
  </script>
  <style>
    /* Vignette halus dan pendaran latar belakang radial */
    .hero-glow {
      background: radial-gradient(circle at 50% 45%, rgba(99, 102, 241, 0.08) 0%, rgba(6, 182, 212, 0.03) 40%, transparent 75%);
    }

    /* Animasi putaran gradasi konik pelangi (rainbow conic spin) */
    @keyframes spin-rainbow {
      0% {
        transform: rotate(0deg);
      }
      100% {
        transform: rotate(360deg);
      }
    }

    .rainbow-conic-spin {
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
    }

    .rainbow-glow-ambient {
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
    }
  </style>
</head>
<body class="min-h-screen flex flex-col bg-[#090a0f] text-slate-200 font-sans selection:bg-indigo-500/30 selection:text-white antialiased relative overflow-x-hidden">
  <!-- Vignette Halus & Pendaran Ambien -->
  <div class="fixed inset-0 hero-glow pointer-events-none z-0"></div>

  <!-- Kontainer Utama -->
  <div class="relative z-10 flex flex-col min-h-screen">
    <!-- Header Minimalis Bersih FikPanel Edge -->
    <header class="w-full border-b border-obsidian-border bg-[#090a0f]/80 backdrop-blur-md sticky top-0 z-50">
      <div class="max-w-6xl mx-auto px-4 sm:px-6 h-14 flex items-center justify-between gap-4">
        <!-- Kiri: Logo & Identitas Host -->
        <div class="flex items-center space-x-3 shrink-0">
          <a class="flex items-center space-x-2 group" href="#">
            <span class="w-6 h-6 rounded bg-slate-800 border border-white/10 flex items-center justify-center text-slate-300 font-mono text-[11px] font-semibold group-hover:border-white/25 transition">
              <i class="fa-solid fa-cube text-[10px] text-indigo-400"></i>
            </span>
            <span class="font-medium text-sm tracking-tight text-white font-mono">FikPanel</span>
          </a>
          <span class="text-[11px] font-mono px-1.5 py-0.5 rounded bg-white/5 text-slate-400 border border-white/5">v3.2 Edge</span>
          <span class="hidden md:inline-flex items-center text-[11px] font-mono text-slate-400 px-2 py-0.5 rounded bg-slate-900/60 border border-white/5">
            <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 mr-1.5"></span>
            <span class="domain-host-label">` + defaultDomain + `</span>
          </span>
        </div>

        <!-- Tengah: Switcher Status Minimalis -->
        <div class="hidden lg:flex items-center bg-obsidian-surface border border-obsidian-border rounded-full p-1 font-mono text-xs shadow-inner">
          <button class="code-btn px-2.5 py-1 rounded-full text-slate-400 hover:text-slate-200 transition" data-code="403" onclick="setErrorCode('403')">403</button>
          <button class="code-btn px-2.5 py-1 rounded-full text-slate-400 hover:text-slate-200 transition" data-code="404" onclick="setErrorCode('404')">404</button>
          <button class="code-btn px-2.5 py-1 rounded-full text-slate-400 hover:text-slate-200 transition" data-code="500" onclick="setErrorCode('500')">500</button>
          <button class="code-btn px-2.5 py-1 rounded-full text-slate-400 hover:text-slate-200 transition" data-code="502" onclick="setErrorCode('502')">502</button>
          <button class="code-btn px-2.5 py-1 rounded-full text-slate-400 hover:text-slate-200 transition" data-code="503" onclick="setErrorCode('503')">503</button>
          <button class="code-btn px-2.5 py-1 rounded-full text-slate-400 hover:text-slate-200 transition" data-code="ready" onclick="setErrorCode('ready')">Ready</button>
        </div>

        <!-- Kanan: Tautan Dokumentasi & Panel -->
        <div class="flex items-center space-x-2 shrink-0">
          <a class="hidden sm:inline-flex items-center space-x-1.5 text-xs font-mono text-slate-400 hover:text-slate-200 px-2.5 py-1.5 rounded transition" href="https://github.com/fikfikk/fikpanel" target="_blank" rel="noopener noreferrer">
            <i class="fa-regular fa-file-lines text-[11px]"></i>
            <span>Dokumentasi</span>
          </a>
          <a id="panel-link" class="inline-flex items-center space-x-1.5 text-xs font-mono text-slate-200 hover:text-white bg-white/5 hover:bg-white/10 border border-white/10 px-3 py-1.5 rounded-md transition" href="#">
            <span>Buka Panel</span>
            <i class="fa-solid fa-arrow-up-right-from-square text-[10px] text-slate-400"></i>
          </a>
        </div>
      </div>

      <!-- Switcher Bar Khusus Layar Seluler -->
      <div class="lg:hidden flex items-center justify-between px-4 py-2 border-t border-obsidian-border bg-obsidian-surface/60 text-xs font-mono overflow-x-auto gap-1">
        <span class="text-[11px] text-slate-500 uppercase shrink-0 mr-1">Status:</span>
        <div class="flex items-center gap-1">
          <button class="code-btn-m px-2.5 py-0.5 rounded-full text-slate-400 bg-white/5" data-code="403" onclick="setErrorCode('403')">403</button>
          <button class="code-btn-m px-2.5 py-0.5 rounded-full text-slate-400 bg-white/5" data-code="404" onclick="setErrorCode('404')">404</button>
          <button class="code-btn-m px-2.5 py-0.5 rounded-full text-slate-400 bg-white/5" data-code="500" onclick="setErrorCode('500')">500</button>
          <button class="code-btn-m px-2.5 py-0.5 rounded-full text-slate-400 bg-white/5" data-code="502" onclick="setErrorCode('502')">502</button>
          <button class="code-btn-m px-2.5 py-0.5 rounded-full text-slate-400 bg-white/5" data-code="503" onclick="setErrorCode('503')">503</button>
          <button class="code-btn-m px-2.5 py-0.5 rounded-full text-slate-400 bg-white/5" data-code="ready" onclick="setErrorCode('ready')">200</button>
        </div>
      </div>
    </header>

    <!-- Konten Pahlawan (Hero) Rata Tengah -->
    <main class="flex-1 min-h-[calc(100vh-140px)] flex flex-col items-center justify-center text-center px-4 sm:px-6 py-12">
      <div class="max-w-2xl w-full mx-auto flex flex-col items-center justify-center">
        <!-- Badge Pill Berputar Bergradasi Pelangi -->
        <div class="relative p-[1.5px] rounded-full overflow-hidden shadow-lg mb-6 group inline-flex items-center justify-center">
          <div class="absolute inset-[-150%] rainbow-conic-spin opacity-90 group-hover:opacity-100 transition-opacity"></div>
          <div class="absolute inset-[-20px] rainbow-glow-ambient opacity-50 group-hover:opacity-80 transition-opacity"></div>
          <div class="relative z-10 inline-flex items-center space-x-2 px-3.5 py-1.5 rounded-full bg-[#0d0f16]/95 backdrop-blur-md text-xs font-mono text-slate-300" id="badge-wrapper">
            <span class="w-2 h-2 rounded-full bg-amber-400 shrink-0" id="badge-dot"></span>
            <span class="tracking-wide uppercase font-medium" id="badge-text">HTTP 404 NOT FOUND</span>
            <span class="text-slate-600">•</span>
            <span class="text-slate-400 text-[11px]" id="badge-sub">Edge Router</span>
          </div>
        </div>

        <!-- Tipografi Judul & Deskripsi Galat -->
        <div class="space-y-3 max-w-xl mx-auto">
          <div class="relative inline-block">
            <h1 class="text-7xl sm:text-8xl font-bold tracking-tight text-white font-mono select-none" id="error-title-code">
              ` + defaultCode + `
            </h1>
          </div>
          <h2 class="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-100" id="main-domain-heading">
            Halaman Tidak Ditemukan
          </h2>
          <p class="text-slate-400 text-sm sm:text-base leading-relaxed pt-1 max-w-lg mx-auto" id="error-description">
            URL atau berkas yang Anda minta tidak tersedia di direktori web hosting server <code class="font-mono text-slate-300 bg-white/5 px-1.5 py-0.5 rounded border border-white/5 text-xs domain-host-label">` + defaultDomain + `</code>.
          </p>
        </div>

        <!-- Tombol Aksi Utama -->
        <div class="flex flex-wrap items-center justify-center gap-3 mt-8">
          <a class="px-4 py-2.5 rounded-md bg-white hover:bg-slate-200 text-slate-950 font-medium text-xs font-mono transition shadow-md flex items-center space-x-2" href="/" onclick="goHomeOrBack(event)">
            <i class="fa-solid fa-arrow-left text-[11px]"></i>
            <span>Kembali ke Beranda</span>
          </a>
          <button class="px-4 py-2.5 rounded-md bg-obsidian-surface hover:bg-obsidian-raised text-slate-200 hover:text-white border border-obsidian-border font-medium text-xs font-mono transition flex items-center space-x-2" onclick="window.location.reload()">
            <i class="fa-solid fa-rotate-right text-[11px] text-slate-400"></i>
            <span>Muat Ulang Halaman</span>
          </button>
          <button class="px-4 py-2.5 rounded-md bg-transparent hover:bg-white/5 text-slate-400 hover:text-slate-200 border border-transparent hover:border-obsidian-border font-medium text-xs font-mono transition flex items-center space-x-2" id="btn-copy-diag" onclick="copyDiagnostics()">
            <i class="fa-regular fa-copy text-[11px]" id="copy-icon"></i>
            <span id="copy-text">Salin Info Diagnostik</span>
          </button>
        </div>

        <!-- Accordion Telemetri Diagnostik yang Dapat Dilipat -->
        <div class="w-full max-w-xl mt-8">
          <button class="w-full py-2.5 px-4 rounded-md border border-obsidian-border bg-obsidian-surface/60 hover:bg-obsidian-surface text-slate-400 hover:text-slate-200 text-xs font-mono flex items-center justify-between transition group backdrop-blur-sm" id="telemetry-toggle-btn" onclick="toggleTelemetry()">
            <span class="flex items-center space-x-2">
              <i class="fa-solid fa-terminal text-[11px] text-indigo-400"></i>
              <span>Tampilkan Detail Diagnostik &amp; Konfigurasi Server</span>
            </span>
            <i class="fa-solid fa-chevron-down text-[10px] text-slate-500 group-hover:text-slate-300 transition-transform duration-200" id="telemetry-chevron"></i>
          </button>

          <!-- Isi Laci Panel Telemetri -->
          <div class="hidden text-left mt-3 space-y-3" id="telemetry-drawer">
            <!-- Grid 4 Kartu Telemetri Cepat -->
            <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs font-mono">
              <div class="p-2.5 rounded border border-obsidian-border bg-obsidian-surface/90">
                <span class="text-[10px] text-slate-500 block uppercase">Host</span>
                <span class="text-slate-200 font-medium truncate block domain-host-label" title="` + defaultDomain + `">` + defaultDomain + `</span>
              </div>
              <div class="p-2.5 rounded border border-obsidian-border bg-obsidian-surface/90">
                <span class="text-[10px] text-slate-500 block uppercase">Response Code</span>
                <span class="text-amber-400 font-medium block" id="card-status-code">404 Not Found</span>
              </div>
              <div class="p-2.5 rounded border border-obsidian-border bg-obsidian-surface/90">
                <span class="text-[10px] text-slate-500 block uppercase">Edge Node</span>
                <span class="text-slate-300 block" id="card-edge-node">sg-edge-01 (SIN)</span>
              </div>
              <div class="p-2.5 rounded border border-obsidian-border bg-obsidian-surface/90">
                <span class="text-[10px] text-slate-500 block uppercase">Trace ID</span>
                <span class="text-slate-300 font-medium truncate block" id="trace-id">8f3d9b4c0e21a7</span>
              </div>
            </div>

            <!-- Bilah Engine Web Server & Petunjuk Pemecahan Masalah -->
            <div class="p-3 rounded border border-obsidian-border bg-obsidian-surface/80 text-xs font-mono text-slate-300">
              <div class="flex items-center justify-between text-[11px] text-slate-500 pb-1 mb-1 border-b border-white/5">
                <span>Web Server Engine: <strong class="text-slate-300 font-normal">Nginx 1.28.x / FikPanel Edge</strong></span>
                <span>TLS: <strong class="text-slate-300 font-normal">1.3</strong></span>
              </div>
              <div class="text-slate-400 text-xs mt-1 leading-relaxed" id="troubleshoot-hint">
                Pastikan berkas <code class="text-slate-200 bg-white/5 px-1 py-0.5 rounded">index.html</code> atau <code class="text-slate-200 bg-white/5 px-1 py-0.5 rounded">index.php</code> telah diunggah ke folder root target di FikPanel File Manager.
              </div>
            </div>

            <!-- Panduan Ringkas Pemilik Situs -->
            <div class="p-3.5 rounded border border-obsidian-border bg-obsidian-surface/50 space-y-2 text-xs font-mono">
              <div class="text-[10px] uppercase tracking-wider text-slate-500 font-semibold mb-1">Panduan Pemilik Situs</div>
              <div class="flex items-start space-x-2 text-slate-400">
                <span class="text-indigo-400 font-bold shrink-0">1.</span>
                <span>Buka menu <strong>File Manager</strong> pada panel FikPanel dan navigasi ke direktori root situs yang dituju.</span>
              </div>
              <div class="flex items-start space-x-2 text-slate-400">
                <span class="text-indigo-400 font-bold shrink-0">2.</span>
                <span>Unggah berkas aplikasi atau build website ke direktori publik yang sesuai.</span>
              </div>
              <div class="flex items-start space-x-2 text-slate-400">
                <span class="text-indigo-400 font-bold shrink-0">3.</span>
                <span>Pastikan permission berkas diatur ke <code class="text-slate-300">0644</code> dan direktori ke <code class="text-slate-300">0755</code>.</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- Footer Status & Metadata -->
    <footer class="w-full border-t border-obsidian-border bg-[#090a0f]/90 py-4 mt-auto">
      <div class="max-w-6xl mx-auto px-4 sm:px-6 flex flex-col sm:flex-row items-center justify-between text-xs text-slate-500 font-mono gap-3">
        <div class="flex items-center space-x-2">
          <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 inline-block"></span>
          <span class="text-slate-400">FikPanel Cloud Infrastructure</span>
          <span class="text-slate-700">·</span>
          <span class="text-slate-500" id="utc-time">UTC --:--:--</span>
        </div>
        <div class="flex items-center space-x-5 text-slate-400">
          <a class="hover:text-slate-200 transition" href="#">Status Jaringan</a>
          <a class="hover:text-slate-200 transition" href="#">Dokumentasi Edge</a>
          <a class="hover:text-slate-200 transition" href="#">Dukungan Teknis</a>
        </div>
      </div>
    </footer>
  </div>

  <!-- Skrip Interaktif & Manajemen Status Halaman -->
  <script>
    // Buka/tutup laci informasi diagnostik
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

    // Jam UTC langsung (Live UTC Clock)
    function updateClock() {
      const now = new Date();
      const utcString = now.toUTCString().split(' ')[4] + ' UTC';
      const el = document.getElementById('utc-time');
      if (el) el.textContent = utcString;
    }
    setInterval(updateClock, 1000);
    updateClock();

    // Navigasi kembali ke halaman sebelumnya atau beranda
    function goHomeOrBack(e) {
      if (window.history.length > 1 && document.referrer && new URL(document.referrer).hostname === window.location.hostname) {
        e.preventDefault();
        window.history.back();
      }
    }

    // Konfigurasi status galat dan halaman siap pakai
    const errorConfigs = {
      '403': {
        code: '403',
        badge: 'HTTP 403 FORBIDDEN',
        badgeSub: 'Security Guard',
        badgeColor: 'bg-rose-400',
        textColor: 'text-rose-400',
        title: 'Akses Ditolak (Forbidden)',
        desc: 'Server memahami permintaan Anda, namun menolak untuk memberikan izin akses ke berkas atau direktori ini di host ',
        statusCard: '403 Forbidden',
        hint: 'Periksa permission folder (standar 0755) atau periksa apakah aturan pembatasan akses / basic auth memblokir IP/request Anda.'
      },
      '404': {
        code: '404',
        badge: 'HTTP 404 NOT FOUND',
        badgeSub: 'Edge Router',
        badgeColor: 'bg-amber-400',
        textColor: 'text-amber-400',
        title: 'Halaman Tidak Ditemukan',
        desc: 'URL atau berkas yang Anda minta tidak tersedia di direktori web hosting server ',
        statusCard: '404 Not Found',
        hint: 'Pastikan berkas index.html atau index.php telah diunggah ke folder root target di FikPanel File Manager.'
      },
      '500': {
        code: '500',
        badge: 'HTTP 500 INTERNAL SERVER ERROR',
        badgeSub: 'Runtime Engine',
        badgeColor: 'bg-rose-400',
        textColor: 'text-rose-400',
        title: 'Kesalahan Internal Server',
        desc: 'Terjadi kegagalan pemrosesan skrip pada server atau kesalahan sintaks pada berkas konfigurasi runtime aplikasi di host ',
        statusCard: '500 Internal Error',
        hint: 'Periksa log galat di menu Response Logs situs, periksa sintaks berkas konfigurasi, atau aktifkan log debugging aplikasi.'
      },
      '502': {
        code: '502',
        badge: 'HTTP 502 BAD GATEWAY',
        badgeSub: 'Upstream Offline',
        badgeColor: 'bg-rose-400',
        textColor: 'text-rose-400',
        title: 'Bad Gateway (Koneksi Upstream Gagal)',
        desc: 'Server gateway Nginx menerima respon yang tidak valid dari proses backend hulu (PHP-FPM, Node.js port, atau backend daemon) di host ',
        statusCard: '502 Bad Gateway',
        hint: 'Pastikan backend service (PHP-FPM pool atau application daemon port) berjalan aktif dan memori RAM server tidak kehabisan kuota.'
      },
      '503': {
        code: '503',
        badge: 'HTTP 503 SERVICE UNAVAILABLE',
        badgeSub: 'Maintenance Mode',
        badgeColor: 'bg-amber-400',
        textColor: 'text-amber-400',
        title: 'Layanan Sedang Pemeliharaan',
        desc: 'Server saat ini tidak dapat menangani permintaan karena beban lalu lintas puncak sementara atau sedang dalam mode pemeliharaan di host ',
        statusCard: '503 Maintenance',
        hint: 'Tunggu beberapa saat atau periksa status penggunaan resource di FikPanel untuk memastikan batas alokasi CPU/RAM tidak tercapai.'
      },
      'ready': {
        code: '200',
        badge: 'HTTP 200 OK — SITE READY',
        badgeSub: 'Virtual Host Ready',
        badgeColor: 'bg-emerald-400',
        textColor: 'text-emerald-400',
        title: 'Situs Berhasil Dikonfigurasi',
        desc: 'Selamat! Domain telah aktif pada klaster FikPanel Edge. Silakan mulai mengunggah file produksi website Anda ke folder publik di host ',
        statusCard: '200 OK (Active)',
        hint: 'Hapus file halaman placeholder ini atau ganti dengan berkas aplikasi website Anda pada direktori publik / root situs.'
      }
    };

    function setErrorCode(key) {
      const data = errorConfigs[key];
      if (!data) return;

      const host = window.location.hostname || '` + defaultDomain + `';
      document.title = data.code + ' — ' + data.title + ' | FikPanel Edge';

      const errTitle = document.getElementById('error-title-code');
      if (errTitle) errTitle.textContent = data.code;

      const badgeText = document.getElementById('badge-text');
      if (badgeText) badgeText.textContent = data.badge;

      const badgeSub = document.getElementById('badge-sub');
      if (badgeSub) badgeSub.textContent = data.badgeSub;

      const mainHeading = document.getElementById('main-domain-heading');
      if (mainHeading) mainHeading.textContent = data.title;

      const descEl = document.getElementById('error-description');
      if (descEl) {
        descEl.innerHTML = data.desc + '<code class="font-mono text-slate-300 bg-white/5 px-1.5 py-0.5 rounded border border-white/5 text-xs domain-host-label">' + host + '</code>.';
      }

      const cardStatus = document.getElementById('card-status-code');
      if (cardStatus) {
        cardStatus.textContent = data.statusCard;
        cardStatus.className = data.textColor + ' font-medium block';
      }

      const badgeDot = document.getElementById('badge-dot');
      if (badgeDot) {
        badgeDot.className = 'w-2 h-2 rounded-full ' + data.badgeColor + ' shrink-0';
      }

      const hintEl = document.getElementById('troubleshoot-hint');
      if (hintEl) hintEl.innerHTML = data.hint;

      // Perbarui Trace ID acak
      const randomHex = Math.random().toString(16).substring(2, 10) + Math.random().toString(16).substring(2, 8);
      const traceEl = document.getElementById('trace-id');
      if (traceEl) traceEl.textContent = randomHex;

      // Perbarui tombol desktop switcher
      document.querySelectorAll('.code-btn').forEach(btn => {
        if (btn.getAttribute('data-code') === key) {
          btn.className = 'code-btn px-2.5 py-1 rounded-full bg-slate-800 text-white font-medium border border-white/10 shadow-sm';
        } else {
          btn.className = 'code-btn px-2.5 py-1 rounded-full text-slate-400 hover:text-slate-200 transition';
        }
      });

      // Perbarui tombol mobile switcher
      document.querySelectorAll('.code-btn-m').forEach(btn => {
        if (btn.getAttribute('data-code') === key) {
          btn.className = 'code-btn-m px-2.5 py-0.5 rounded-full bg-slate-800 text-white font-medium border border-white/10';
        } else {
          btn.className = 'code-btn-m px-2.5 py-0.5 rounded-full text-slate-400 bg-white/5';
        }
      });
    }

    // Salin diagnostik ke clipboard pengguna
    function copyDiagnostics() {
      const code = document.getElementById('error-title-code')?.textContent.trim() || '` + defaultCode + `';
      const trace = document.getElementById('trace-id')?.textContent.trim() || '8f3d9b4c0e21a7';
      const host = window.location.hostname || '` + defaultDomain + `';
      const node = document.getElementById('card-edge-node')?.textContent.trim() || 'sg-edge-01 (SIN)';
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
        const copyIcon = document.getElementById('copy-icon');
        if (copyText) copyText.textContent = 'Info Tersalin!';
        if (copyIcon) copyIcon.className = 'fa-solid fa-check text-[11px] text-emerald-400';
        setTimeout(() => {
          if (copyText) copyText.textContent = 'Salin Info Diagnostik';
          if (copyIcon) copyIcon.className = 'fa-regular fa-copy text-[11px]';
        }, 2000);
      });
    }

    // Inisialisasi awal saat DOM siap dimuat
    document.addEventListener('DOMContentLoaded', () => {
      const curHost = window.location.hostname || '` + defaultDomain + `';
      document.querySelectorAll('.domain-host-label').forEach(el => {
        el.textContent = curHost;
      });

      // Arahkan tautan panel ke port default 1405
      const panelLink = document.getElementById('panel-link');
      if (panelLink) {
        panelLink.href = window.location.protocol + '//' + curHost + ':1405';
      }

      // Deteksi query param code (?code=403 / 502 / ready)
      const urlParams = new URLSearchParams(window.location.search);
      const reqCode = urlParams.get('code');
      if (reqCode && errorConfigs[reqCode]) {
        setErrorCode(reqCode);
      } else {
        setErrorCode('` + defaultCode + `');
      }
    });
  </script>
</body>
</html>`
}

var (
	tpl403     = buildEdgePageTemplate("403", "contoh-domain.com")
	tpl404     = buildEdgePageTemplate("404", "contoh-domain.com")
	tpl50x     = buildEdgePageTemplate("502", "contoh-domain.com")
	tplWelcome = buildEdgePageTemplate("ready", "{{DOMAIN}}")
)
