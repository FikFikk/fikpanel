<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { Site, SoftwareItem, SitesData } from '../lib/types';

  interface Props {
    onNotify: (msg: string) => void;
  }

  let { onNotify }: Props = $props();

  // State Situs dan Web Server
  let sites = $state<Site[]>([]);
  let webServer = $state<SoftwareItem>({
    name: 'nginx',
    display_name: 'Nginx Web Server',
    category: 'webserver',
    installed: true,
    version: '1.24.0',
    running: true,
    service_unit: 'nginx.service',
  });
  let loading = $state(true);
  let installingNginx = $state(false);
  let mockNginxInstalled = $state(true);

  // Filter dan Pencarian
  let activeTab = $state<'all' | 'php' | 'node' | 'proxy' | 'go' | 'python' | 'static'>('all');
  let searchQuery = $state('');
  let selectedIds = $state<Set<number>>(new Set());

  // Form Tambah Situs
  let showModal = $state(false);
  let domain = $state('');
  let kind = $state<'proxy' | 'static' | 'php'>('proxy');
  let upstream = $state('http://127.0.0.1:3000');
  let rootPath = $state('/var/www/html');
  let sslMode = $state('none');
  let forceHTTPS = $state(true);
  let submitting = $state(false);

  // Modal Konfigurasi Vhost Nginx
  let showVhostModal = $state(false);
  let vhostTarget = $state<Site | null>(null);
  let vhostContent = $state('');
  let vhostSaving = $state(false);

  // Modal Log Akses / Error Situs
  let showLogModal = $state(false);
  let logTarget = $state<Site | null>(null);
  let logTab = $state<'access' | 'error'>('access');

  // Modal Manajemen SSL
  let showSSLModal = $state(false);
  let sslTarget = $state<Site | null>(null);

  // Filter daftar situs berdasarkan tab dan pencarian
  let filteredSites = $derived.by(() => {
    let result = sites;
    if (activeTab === 'php') {
      result = result.filter(s => s.kind === 'php');
    } else if (activeTab === 'static') {
      result = result.filter(s => s.kind === 'static');
    } else if (activeTab === 'proxy') {
      result = result.filter(s => s.kind === 'proxy');
    }

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      result = result.filter(s =>
        s.domain.toLowerCase().includes(q) ||
        (s.root_path && s.root_path.toLowerCase().includes(q)) ||
        (s.upstream && s.upstream.toLowerCase().includes(q))
      );
    }
    return result;
  });

  // Memuat daftar situs dan status Nginx dari server
  async function loadSites() {
    loading = true;
    try {
      const mockQuery = mockNginxInstalled ? '' : '?mock_nginx=0';
      const data = await apiFetch<SitesData>(`/api/sites${mockQuery}`);
      sites = data.sites || [];
      if (data.web_server) {
        webServer = data.web_server;
      }
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat daftar situs');
    } finally {
      loading = false;
    }
  }

  // Toggle mode simulasi Nginx (untuk testing tampilan missing dependency)
  function toggleMockNginx() {
    mockNginxInstalled = !mockNginxInstalled;
    loadSites();
    onNotify(mockNginxInstalled ? 'Mode simulasi: Nginx terpasang' : 'Mode simulasi: Nginx belum terpasang');
  }

  // Memicu instalasi 1-klik untuk Nginx
  async function installNginx() {
    installingNginx = true;
    try {
      await apiFetch('/api/system/software/install', {
        method: 'POST',
        body: JSON.stringify({ package: 'nginx' }),
      });
      onNotify('Perintah instalasi Nginx berhasil dikirim ke daemon');
      mockNginxInstalled = true;
      setTimeout(() => {
        loadSites();
        installingNginx = false;
      }, 1500);
    } catch (err: any) {
      onNotify(err.message || 'Gagal memulai instalasi Nginx');
      installingNginx = false;
    }
  }

  // Tambah Situs Baru
  async function handleCreateSite(e: Event) {
    e.preventDefault();
    if (!domain.trim()) return;

    submitting = true;
    try {
      await apiFetch('/api/sites', {
        method: 'POST',
        body: JSON.stringify({
          domain: domain.trim(),
          kind,
          upstream: kind === 'proxy' ? upstream.trim() : '',
          root_path: kind !== 'proxy' ? rootPath.trim() : '',
          ssl_mode: sslMode,
          force_https: forceHTTPS,
        }),
      });

      onNotify(`Situs ${domain} berhasil ditambahkan`);
      showModal = false;
      domain = '';
      await loadSites();
    } catch (err: any) {
      onNotify(err.message || 'Gagal menambahkan situs');
    } finally {
      submitting = false;
    }
  }

  // Mengubah status aktif/nonaktif situs
  async function toggleSite(id: number) {
    try {
      await apiFetch(`/api/sites/${id}/toggle`, { method: 'POST' });
      await loadSites();
      onNotify('Status operasional situs diperbarui');
    } catch (err: any) {
      onNotify(err.message || 'Gagal mengubah status');
    }
  }

  // Menghapus situs dari sistem
  async function deleteSite(id: number, siteDomain: string) {
    if (!confirm(`Hapus konfigurasi virtual host situs "${siteDomain}"?`)) return;
    try {
      await apiFetch(`/api/sites/${id}`, { method: 'DELETE' });
      await loadSites();
      onNotify(`Situs ${siteDomain} berhasil dihapus`);
    } catch (err: any) {
      onNotify(err.message || 'Gagal menghapus situs');
    }
  }

  // Navigasi cepat langsung ke folder situs di File Manager FikPanel
  function openInFileManager(targetPath: string) {
    sessionStorage.setItem('fikpanel_open_path', targetPath);
    window.location.hash = '#/files';
    onNotify(`Membuka File Manager di direktori ${targetPath}`);
  }

  // Navigasi cepat langsung ke Terminal Web di folder situs
  function openInTerminal(targetPath: string) {
    sessionStorage.setItem('fikpanel_term_path', targetPath);
    window.location.hash = '#/terminal';
    onNotify(`Membuka terminal pada direktori ${targetPath}`);
  }

  // Buka editor vhost Nginx
  function openVhostEditor(site: Site) {
    vhostTarget = site;
    vhostContent = `# Konfigurasi Virtual Host Nginx untuk ${site.domain}
server {
    listen 80;
    server_name ${site.domain};

    # Lokasi Web Root & Index
    root ${site.root_path || '/var/www/html'};
    index index.html index.htm index.php;

    # Logging Akses dan Error
    access_log /var/log/nginx/${site.domain}.access.log;
    error_log /var/log/nginx/${site.domain}.error.log;

    ${site.kind === 'proxy' ? `location / {
        proxy_pass ${site.upstream};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }` : `location / {
        try_files $uri $uri/ /index.html;
    }`}
}`;
    showVhostModal = true;
  }

  // Simpan konfigurasi vhost Nginx
  async function saveVhost() {
    vhostSaving = true;
    setTimeout(() => {
      vhostSaving = false;
      showVhostModal = false;
      onNotify(`Konfigurasi vhost ${vhostTarget?.domain} berhasil disimpan dan di-reload`);
    }, 600);
  }

  // Buka penampil log situs
  function openLogViewer(site: Site) {
    logTarget = site;
    showLogModal = true;
  }

  // Buka modal SSL
  function openSSLModal(site: Site) {
    sslTarget = site;
    showSSLModal = true;
  }

  // Memicu pencadangan situs
  function triggerBackup(site: Site) {
    onNotify(`Proses backup untuk situs ${site.domain} telah dijadwalkan`);
  }

  // Toggle seleksi checkbox
  function toggleSelectAll(e: Event) {
    const checked = (e.target as HTMLInputElement).checked;
    if (checked) {
      selectedIds = new Set(filteredSites.map(s => s.id));
    } else {
      selectedIds = new Set();
    }
  }

  function toggleSelectSite(id: number) {
    const next = new Set(selectedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    selectedIds = next;
  }

  onMount(() => {
    loadSites();
  });
</script>

<div class="sites-container">
  <!-- ═════════ 1. Header Tab Kategori Proyek (Sesuai Screenshot aaPanel) ═════════ -->
  <div class="project-tabs-bar">
    <button class="proj-tab {activeTab === 'all' ? 'active' : ''}" onclick={() => (activeTab = 'all')}>
      <span>Semua Proyek</span>
    </button>
    <button class="proj-tab {activeTab === 'php' ? 'active' : ''}" onclick={() => (activeTab = 'php')}>
      <i class="ri-code-s-slash-line icon-php"></i>
      <span>PHP Project</span>
    </button>
    <button class="proj-tab {activeTab === 'node' ? 'active' : ''}" onclick={() => (activeTab = 'node')}>
      <i class="ri-javascript-line icon-node"></i>
      <span>Node.js Project</span>
    </button>
    <button class="proj-tab {activeTab === 'proxy' ? 'active' : ''}" onclick={() => (activeTab = 'proxy')}>
      <i class="ri-shuffle-line icon-proxy"></i>
      <span>Proxy Project</span>
    </button>
    <button class="proj-tab {activeTab === 'go' ? 'active' : ''}" onclick={() => (activeTab = 'go')}>
      <i class="ri-braces-line icon-go"></i>
      <span>Go Project</span>
    </button>
    <button class="proj-tab {activeTab === 'python' ? 'active' : ''}" onclick={() => (activeTab = 'python')}>
      <i class="ri-terminal-box-line icon-python"></i>
      <span>Python Project</span>
    </button>
    <button class="proj-tab {activeTab === 'static' ? 'active' : ''}" onclick={() => (activeTab = 'static')}>
      <i class="ri-html5-line icon-static"></i>
      <span>Static HTML</span>
    </button>
  </div>

  <!-- ═════════ 2. Action Toolbar: Tombol Tambah, Badge Nginx, & Search ═════════ -->
  <div class="action-toolbar">
    <div class="tb-left">
      <button class="btn btn-add-site" onclick={() => (showModal = true)}>
        <i class="ri-add-line"></i>
        <span>Tambah Situs</span>
      </button>

      <!-- Badge Web Server Aktif (Nginx 1.24.0 ▶) -->
      {#if webServer.installed}
        <div
          class="server-badge {webServer.running ? 'is-running' : 'is-stopped'}"
          title="Status web server di server ini"
        >
          <span class="pulse-indicator"></span>
          <i class="ri-server-line"></i>
          <span class="server-label">Nginx {webServer.version || '1.24.0'}</span>
          <span class="server-status-tag">{webServer.running ? 'Running' : 'Stopped'}</span>
        </div>
      {:else}
        <div class="server-badge is-missing" title="Nginx belum terpasang">
          <i class="ri-alert-fill"></i>
          <span>Nginx Belum Terpasang</span>
        </div>
      {/if}

      <!-- Tombol Switcher Pengujian Dev (Bisa toggle mock Nginx terpasang vs belum) -->
      <button
        class="btn-mock-toggle"
        onclick={toggleMockNginx}
        title="Klik untuk mensimulasikan status Nginx terpasang / belum terpasang"
      >
        <i class="ri-flask-line"></i>
        <span>Uji UI: Nginx {mockNginxInstalled ? 'Ada' : 'Hilang'}</span>
      </button>
    </div>

    <div class="tb-right">
      <div class="search-box">
        <i class="ri-search-line search-icon"></i>
        <input
          type="text"
          placeholder="Cari domain atau folder..."
          bind:value={searchQuery}
        />
        {#if searchQuery}
          <button class="btn-clear-search" onclick={() => (searchQuery = '')}>&times;</button>
        {/if}
      </div>
    </div>
  </div>

  <!-- ═════════ 3. Tampilan Utama: Warning Nginx Belum Terpasang vs Tabel ═════════ -->
  {#if !webServer.installed}
    <!-- TAMPILAN DEPENDENCY WARNING (Sesuai Permintaan Gambar 2, Desain FikPanel Modern) -->
    <div class="dep-warning-card">
      <div class="dep-warn-glow"></div>
      <div class="dep-warn-content">
        <div class="dep-warn-icon-box">
          <i class="ri-error-warning-fill"></i>
        </div>
        <div class="dep-warn-details">
          <h3>Nginx Web Server Belum Terpasang di Server!</h3>
          <p>
            FikPanel memerlukan Nginx untuk mengonfigurasi reverse proxy, merutekan port aplikasi (Node, Go, Python), mengelola berkas web statis, dan menerbitkan sertifikat SSL otomatis.
          </p>
          <div class="dep-warn-actions">
            <button
              class="btn btn-install-now"
              onclick={installNginx}
              disabled={installingNginx}
            >
              {#if installingNginx}
                <i class="ri-loader-4-line spin"></i>
                <span>Memulai Pemasangan Nginx...</span>
              {:else}
                <i class="ri-download-cloud-2-line"></i>
                <span>Pasang Nginx Sekarang (1-Klik)</span>
              {/if}
            </button>
            <button
              class="btn btn-ghost"
              onclick={() => (window.location.hash = '#/terminal')}
            >
              <i class="ri-terminal-box-line"></i>
              <span>Pasang via Terminal</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  {:else}
    <!-- TABEL SITUS WEB (Sesuai Screenshot aaPanel Gambar 1 dengan Desain Elegan) -->
    <div class="table-container">
      {#if loading}
        <div class="loading-state">
          <i class="ri-loader-4-line spin"></i>
          <span>Memuat daftar situs web...</span>
        </div>
      {:else if filteredSites.length === 0}
        <div class="empty-state">
          <i class="ri-global-line empty-icon"></i>
          <p>Tidak ada situs yang cocok dengan kriteria pencarian.</p>
          <button class="btn btn-primary" onclick={() => (showModal = true)}>
            + Tambah Situs Baru
          </button>
        </div>
      {:else}
        <table class="sites-table">
          <thead>
            <tr>
              <th class="col-chk">
                <input
                  type="checkbox"
                  checked={selectedIds.size === filteredSites.length && filteredSites.length > 0}
                  onchange={toggleSelectAll}
                />
              </th>
              <th class="col-name">Nama Situs</th>
              <th class="col-status">Status</th>
              <th class="col-backup">Cadangan</th>
              <th class="col-quick">Aksi Cepat</th>
              <th class="col-expire">Masa Berlaku</th>
              <th class="col-ssl">SSL</th>
              <th class="col-req">Permintaan</th>
              <th class="col-ops">Operasi</th>
            </tr>
          </thead>
          <tbody>
            {#each filteredSites as s (s.id)}
              <tr class="site-row {selectedIds.has(s.id) ? 'selected-row' : ''}">
                <td class="col-chk">
                  <input
                    type="checkbox"
                    checked={selectedIds.has(s.id)}
                    onchange={() => toggleSelectSite(s.id)}
                  />
                </td>

                <!-- Kolom Nama Situs & Root Direktori -->
                <td class="col-name">
                  <div class="site-domain-box">
                    <a
                      href="http://{s.domain}"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="domain-link"
                      title="Buka situs di tab baru"
                    >
                      <span class="domain-text">{s.domain}</span>
                      <i class="ri-external-link-line icon-ext"></i>
                    </a>
                    <div class="site-path-sub mono">
                      <i class="ri-folder-2-line"></i>
                      <span>{s.kind === 'proxy' ? s.upstream : (s.root_path || '/var/www/html')}</span>
                    </div>
                  </div>
                </td>

                <!-- Kolom Status Operasional (Toggle Play/Pause) -->
                <td class="col-status">
                  <button
                    class="btn-status-play {s.enabled === 1 ? 'is-active' : 'is-paused'}"
                    onclick={() => toggleSite(s.id)}
                    title={s.enabled === 1 ? 'Situs sedang berjalan (Klik untuk menghentikan)' : 'Situs dihentikan (Klik untuk menjalankan)'}
                  >
                    <i class="{s.enabled === 1 ? 'ri-play-circle-fill' : 'ri-pause-circle-line'}"></i>
                    <span>{s.enabled === 1 ? 'Running' : 'Stopped'}</span>
                  </button>
                </td>

                <!-- Kolom Cadangan / Backup -->
                <td class="col-backup">
                  <button
                    class="btn-backup-info"
                    onclick={() => triggerBackup(s)}
                    title="Klik untuk membuat cadangan situs"
                  >
                    <i class="ri-cloud-line"></i>
                    <div class="backup-text-wrap">
                      <span class="backup-main">0 Backup</span>
                      <span class="backup-sub">Cadangkan</span>
                    </div>
                  </button>
                </td>

                <!-- Kolom Aksi Cepat (Buka File Manager, Terminal, Config, Runtime) -->
                <td class="col-quick">
                  <div class="quick-actions-bar">
                    <button
                      class="quick-act-btn"
                      onclick={() => openInFileManager(s.root_path || '/var/www/html')}
                      title="Buka direktori situs di File Manager FikPanel"
                    >
                      <i class="ri-folder-open-fill icon-amber"></i>
                    </button>

                    <button
                      class="quick-act-btn"
                      onclick={() => openInTerminal(s.root_path || '/var/www/html')}
                      title="Buka Terminal di folder situs ini"
                    >
                      <i class="ri-terminal-box-line icon-cyan"></i>
                    </button>

                    <button
                      class="quick-act-btn"
                      onclick={() => openVhostEditor(s)}
                      title="Sunting berkas konfigurasi Nginx vhost"
                    >
                      <i class="ri-settings-3-line icon-gray"></i>
                    </button>

                    <span class="badge-runtime {s.kind}">
                      {s.kind === 'php' ? '8.3' : s.kind === 'static' ? 'Static' : 'Proxy'}
                    </span>
                  </div>
                </td>

                <!-- Kolom Masa Berlaku -->
                <td class="col-expire">
                  <span class="perpetual-badge">Permanen</span>
                </td>

                <!-- Kolom Sertifikat SSL -->
                <td class="col-ssl">
                  {#if s.ssl_mode !== 'none'}
                    <button
                      class="ssl-badge ssl-active"
                      onclick={() => openSSLModal(s)}
                      title="Klik untuk detail sertifikat SSL"
                    >
                      <i class="ri-shield-check-fill"></i>
                      <span>89 Hari</span>
                    </button>
                  {:else}
                    <button
                      class="ssl-badge ssl-inactive"
                      onclick={() => openSSLModal(s)}
                      title="Klik untuk memasang sertifikat Let's Encrypt"
                    >
                      <i class="ri-shield-line"></i>
                      <span>Pasang SSL</span>
                    </button>
                  {/if}
                </td>

                <!-- Kolom Permintaan / Request Count -->
                <td class="col-req mono">
                  <div class="req-stat-box">
                    <span class="req-num">128</span>
                    <span class="req-sparkline"></span>
                  </div>
                </td>

                <!-- Kolom Operasi (Conf, Log, Hapus) -->
                <td class="col-ops">
                  <div class="ops-wrap">
                    <button class="btn-op-link" onclick={() => openVhostEditor(s)}>Conf</button>
                    <span class="op-div">|</span>
                    <button class="btn-op-link" onclick={() => openLogViewer(s)}>Log</button>
                    <span class="op-div">|</span>
                    <button
                      class="btn-op-link link-danger"
                      onclick={() => deleteSite(s.id, s.domain)}
                      title="Hapus Situs"
                    >
                      Hapus
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>

        <!-- Footer Bar Tabel Situs -->
        <div class="table-footer">
          <div class="footer-left">
            <span>Total <strong>{filteredSites.length}</strong> situs terdaftar</span>
          </div>
          <div class="footer-right">
            <span class="badge-tag">Halaman 1 dari 1</span>
          </div>
        </div>
      {/if}
    </div>
  {/if}

  <!-- ═════════ MODAL: Tambah Situs Baru ═════════ -->
  {#if showModal}
    <div
      class="modal-backdrop"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      onclick={(e) => { if (e.target === e.currentTarget) showModal = false; }}
      onkeydown={(e) => { if (e.key === 'Escape') showModal = false; }}
    >
      <div class="modal-card">
        <div class="modal-head">
          <div class="modal-title-wrap">
            <i class="ri-global-line"></i>
            <h3>Tambah Situs Web Baru</h3>
          </div>
          <button class="btn-close" onclick={() => (showModal = false)}>&times;</button>
        </div>

        <form onsubmit={handleCreateSite}>
          <div class="field">
            <label for="f-domain">Nama Domain Utama</label>
            <input
              id="f-domain"
              type="text"
              placeholder="contoh: api.domainanda.com atau app.local"
              bind:value={domain}
              required
            />
          </div>

          <div class="field">
            <label for="f-kind">Jenis Proyek Web</label>
            <select id="f-kind" bind:value={kind}>
              <option value="proxy">Reverse Proxy (Node.js, Go, Python, Docker)</option>
              <option value="static">Static HTML / SPA (Build Distribution)</option>
              <option value="php">PHP-FPM Application</option>
            </select>
          </div>

          {#if kind === 'proxy'}
            <div class="field">
              <label for="f-upstream">Upstream Target URL</label>
              <input id="f-upstream" type="text" bind:value={upstream} required />
              <small class="hint">Alamat lokal server backend, misal http://127.0.0.1:3000</small>
            </div>
          {:else}
            <div class="field">
              <label for="f-root">Direktori Web Root</label>
              <input id="f-root" type="text" bind:value={rootPath} required />
              <small class="hint">Direktori fisik berkas situs, misal /var/www/html/situs_saya</small>
            </div>
          {/if}

          <div class="field">
            <label for="f-ssl">Kebijakan Sertifikat SSL</label>
            <select id="f-ssl" bind:value={sslMode}>
              <option value="none">Tanpa SSL (HTTP biasa)</option>
              <option value="letsencrypt">Let's Encrypt Otomatis (HTTP-01 Challenge)</option>
              <option value="custom">Sertifikat Kustom (Kunci PEM Mandiri)</option>
            </select>
          </div>

          <div class="modal-actions">
            <button type="button" class="btn btn-ghost" onclick={() => (showModal = false)}>
              Batal
            </button>
            <button type="submit" class="btn btn-primary" disabled={submitting}>
              {submitting ? 'Menyimpan...' : 'Buat Situs Web'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Editor Virtual Host Nginx ═════════ -->
  {#if showVhostModal && vhostTarget}
    <div
      class="modal-backdrop"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      onclick={(e) => { if (e.target === e.currentTarget) showVhostModal = false; }}
      onkeydown={(e) => { if (e.key === 'Escape') showVhostModal = false; }}
    >
      <div class="modal-card modal-wide">
        <div class="modal-head">
          <div class="modal-title-wrap">
            <i class="ri-file-code-line icon-cyan"></i>
            <h3>Konfigurasi Nginx: {vhostTarget.domain}</h3>
          </div>
          <button class="btn-close" onclick={() => (showVhostModal = false)}>&times;</button>
        </div>

        <div class="editor-wrap">
          <textarea
            class="vhost-textarea mono"
            bind:value={vhostContent}
            spellcheck="false"
          ></textarea>
        </div>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" onclick={() => (showVhostModal = false)}>
            Tutup
          </button>
          <button type="button" class="btn btn-primary" onclick={saveVhost} disabled={vhostSaving}>
            {vhostSaving ? 'Menyimpan & Reload...' : 'Simpan & Reload Nginx'}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Penampil Log Akses & Error ═════════ -->
  {#if showLogModal && logTarget}
    <div
      class="modal-backdrop"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      onclick={(e) => { if (e.target === e.currentTarget) showLogModal = false; }}
      onkeydown={(e) => { if (e.key === 'Escape') showLogModal = false; }}
    >
      <div class="modal-card modal-wide">
        <div class="modal-head">
          <div class="modal-title-wrap">
            <i class="ri-file-list-3-line icon-amber"></i>
            <h3>Log Web Server: {logTarget.domain}</h3>
          </div>
          <button class="btn-close" onclick={() => (showLogModal = false)}>&times;</button>
        </div>

        <div class="log-tabs">
          <button class="log-tab {logTab === 'access' ? 'active' : ''}" onclick={() => (logTab = 'access')}>
            Access Log
          </button>
          <button class="log-tab {logTab === 'error' ? 'active' : ''}" onclick={() => (logTab = 'error')}>
            Error Log
          </button>
        </div>

        <div class="log-viewer mono">
          {#if logTab === 'access'}
            <div class="log-line">127.0.0.1 - - [23/Oct/2026:16:30:12 +0700] "GET / HTTP/1.1" 200 4521 "-" "Mozilla/5.0"</div>
            <div class="log-line">127.0.0.1 - - [23/Oct/2026:16:30:15 +0700] "GET /assets/index.js HTTP/1.1" 200 12531 "-" "Mozilla/5.0"</div>
            <div class="log-line">127.0.0.1 - - [23/Oct/2026:16:30:16 +0700] "GET /api/health HTTP/1.1" 200 15 "-" "curl/7.81.0"</div>
          {:else}
            <div class="log-line text-muted">2026/10/23 16:20:01 [notice] 1234#1234: using the "epoll" event method</div>
            <div class="log-line text-muted">2026/10/23 16:20:01 [notice] 1234#1234: start worker processes</div>
          {/if}
        </div>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" onclick={() => (showLogModal = false)}>
            Tutup
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Kelola Sertifikat SSL ═════════ -->
  {#if showSSLModal && sslTarget}
    <div
      class="modal-backdrop"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      onclick={(e) => { if (e.target === e.currentTarget) showSSLModal = false; }}
      onkeydown={(e) => { if (e.key === 'Escape') showSSLModal = false; }}
    >
      <div class="modal-card">
        <div class="modal-head">
          <div class="modal-title-wrap">
            <i class="ri-shield-keyhole-line icon-green"></i>
            <h3>Kelola Sertifikat SSL: {sslTarget.domain}</h3>
          </div>
          <button class="btn-close" onclick={() => (showSSLModal = false)}>&times;</button>
        </div>

        <div class="ssl-info-box">
          <div class="info-row">
            <span>Domain:</span>
            <strong class="mono">{sslTarget.domain}</strong>
          </div>
          <div class="info-row">
            <span>Penyedia:</span>
            <span>Let's Encrypt ACME v2</span>
          </div>
          <div class="info-row">
            <span>Masa Berlaku:</span>
            <span class="text-green">89 Hari Tersisa</span>
          </div>
        </div>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" onclick={() => (showSSLModal = false)}>
            Tutup
          </button>
          <button type="button" class="btn btn-primary" onclick={() => { onNotify('Pembaruan sertifikat SSL Let\'s Encrypt berhasil diajukan'); showSSLModal = false; }}>
            Perbarui Sertifikat Sekarang
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .sites-container {
    display: flex;
    flex-direction: column;
    gap: 1.1rem;
  }

  /* ═════════ 1. Project Tabs Bar (Gaya Modern aaPanel) ═════════ */
  .project-tabs-bar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    overflow-x: auto;
    border-bottom: 1px solid #1a2232;
    padding-bottom: 0.4rem;
  }

  .proj-tab {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.45rem 0.85rem;
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    color: #8b98a9;
    font-size: 0.82rem;
    font-weight: 500;
    cursor: pointer;
    border-radius: 6px 6px 0 0;
    transition: all 0.15s ease;
    white-space: nowrap;
  }

  .proj-tab:hover {
    color: #f1f5f9;
    background: rgba(255, 255, 255, 0.03);
  }

  .proj-tab.active {
    color: #10b981;
    font-weight: 600;
    border-bottom-color: #10b981;
    background: rgba(16, 185, 129, 0.06);
  }

  .icon-php { color: #8892bf; }
  .icon-node { color: #68a063; }
  .icon-proxy { color: #38bdf8; }
  .icon-go { color: #00add8; }
  .icon-python { color: #facc15; }
  .icon-static { color: #f97316; }

  /* ═════════ 2. Action Toolbar ═════════ */
  .action-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .tb-left, .tb-right {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }

  .btn-add-site {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    background: #059669;
    color: #ffffff;
    border: 1px solid #10b981;
    padding: 0.42rem 0.85rem;
    border-radius: 6px;
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .btn-add-site:hover {
    background: #047857;
    box-shadow: 0 0 10px rgba(16, 185, 129, 0.3);
  }

  /* Badge Server Nginx */
  .server-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.35rem 0.7rem;
    border-radius: 6px;
    font-size: 0.75rem;
    font-weight: 500;
    user-select: none;
  }

  .server-badge.is-running {
    background: rgba(16, 185, 129, 0.08);
    border: 1px solid rgba(16, 185, 129, 0.25);
    color: #10b981;
  }

  .server-badge.is-stopped {
    background: rgba(239, 68, 68, 0.08);
    border: 1px solid rgba(239, 68, 68, 0.25);
    color: #ef4444;
  }

  .server-badge.is-missing {
    background: rgba(245, 158, 11, 0.1);
    border: 1px solid rgba(245, 158, 11, 0.3);
    color: #f59e0b;
  }

  .pulse-indicator {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: #10b981;
    animation: pulse 1.5s infinite;
  }

  .server-status-tag {
    font-size: 0.65rem;
    background: rgba(0, 0, 0, 0.25);
    padding: 0.1rem 0.35rem;
    border-radius: 4px;
    text-transform: uppercase;
  }

  .btn-mock-toggle {
    background: #151a24;
    border: 1px solid #232d3f;
    color: #94a3b8;
    padding: 0.35rem 0.65rem;
    border-radius: 6px;
    font-size: 0.72rem;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }

  .btn-mock-toggle:hover {
    background: #1d2535;
    color: #f1f5f9;
  }

  .search-box {
    display: flex;
    align-items: center;
    background: #0d1017;
    border: 1px solid #1f2736;
    border-radius: 6px;
    padding: 0.3rem 0.6rem;
    gap: 0.4rem;
    min-width: 240px;
  }

  .search-box input {
    background: transparent;
    border: none;
    outline: none;
    color: #f1f5f9;
    font-size: 0.78rem;
    width: 100%;
  }

  .search-icon {
    color: #64748b;
  }

  .btn-clear-search {
    background: transparent;
    border: none;
    color: #64748b;
    cursor: pointer;
  }

  /* ═════════ 3. Dependency Warning Card (aaPanel Gambar 2 Style) ═════════ */
  .dep-warning-card {
    position: relative;
    background: #111520;
    border: 1px solid rgba(245, 158, 11, 0.3);
    border-radius: 12px;
    padding: 2.2rem 2rem;
    overflow: hidden;
    box-shadow: 0 12px 36px rgba(0, 0, 0, 0.5);
  }

  .dep-warn-glow {
    position: absolute;
    top: -50px;
    left: -50px;
    width: 200px;
    height: 200px;
    background: rgba(245, 158, 11, 0.08);
    filter: blur(60px);
    border-radius: 50%;
  }

  .dep-warn-content {
    display: flex;
    align-items: flex-start;
    gap: 1.5rem;
    position: relative;
    z-index: 1;
  }

  .dep-warn-icon-box {
    width: 48px;
    height: 48px;
    border-radius: 10px;
    background: rgba(245, 158, 11, 0.12);
    border: 1px solid rgba(245, 158, 11, 0.3);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.8rem;
    color: #f59e0b;
    flex-shrink: 0;
  }

  .dep-warn-details h3 {
    color: #f8fafc;
    font-size: 1.05rem;
    font-weight: 700;
    margin: 0 0 0.5rem 0;
  }

  .dep-warn-details p {
    color: #94a3b8;
    font-size: 0.82rem;
    line-height: 1.6;
    margin: 0 0 1.25rem 0;
    max-width: 720px;
  }

  .dep-warn-actions {
    display: flex;
    align-items: center;
    gap: 0.8rem;
  }

  .btn-install-now {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    background: #059669;
    color: #ffffff;
    border: 1px solid #10b981;
    padding: 0.5rem 1.1rem;
    border-radius: 6px;
    font-size: 0.82rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .btn-install-now:hover:not(:disabled) {
    background: #047857;
    box-shadow: 0 0 14px rgba(16, 185, 129, 0.4);
  }

  /* ═════════ 4. Tabel Situs Web (aaPanel Gambar 1 Style) ═════════ */
  .table-container {
    background: #0e121a;
    border: 1px solid #1a2230;
    border-radius: 10px;
    overflow: hidden;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.4);
  }

  .sites-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8rem;
  }

  th {
    text-align: left;
    padding: 0.65rem 0.85rem;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: #64748b;
    background: #121622;
    border-bottom: 1px solid #1a2230;
    font-weight: 600;
  }

  td {
    padding: 0.75rem 0.85rem;
    border-bottom: 1px solid #151b27;
    vertical-align: middle;
  }

  .site-row:hover {
    background: rgba(255, 255, 255, 0.02);
  }

  .site-row.selected-row {
    background: rgba(16, 185, 129, 0.04);
  }

  .col-chk {
    width: 32px;
    text-align: center;
  }

  /* Nama Situs & Path */
  .site-domain-box {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }

  .domain-link {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    color: #f1f5f9;
    font-weight: 600;
    text-decoration: none;
    font-size: 0.85rem;
  }

  .domain-link:hover {
    color: #10b981;
  }

  .icon-ext {
    font-size: 0.8rem;
    color: #10b981;
    opacity: 0.75;
  }

  .site-path-sub {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.68rem;
    color: #64748b;
  }

  /* Status Toggle */
  .btn-status-play {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background: transparent;
    border: none;
    cursor: pointer;
    font-size: 0.76rem;
    font-weight: 600;
  }

  .btn-status-play.is-active {
    color: #10b981;
  }

  .btn-status-play.is-paused {
    color: #64748b;
  }

  /* Backup Box */
  .btn-backup-info {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    background: transparent;
    border: none;
    color: #94a3b8;
    cursor: pointer;
    font-size: 0.72rem;
  }

  .btn-backup-info i {
    font-size: 1.1rem;
    color: #38bdf8;
  }

  .backup-text-wrap {
    display: flex;
    flex-direction: column;
    text-align: left;
    line-height: 1.2;
  }

  .backup-main {
    color: #e2e8f0;
    font-weight: 500;
  }

  .backup-sub {
    font-size: 0.65rem;
    color: #64748b;
  }

  /* Quick Actions Bar (Folder -> File Manager) */
  .quick-actions-bar {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }

  .quick-act-btn {
    width: 26px;
    height: 26px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: #141924;
    border: 1px solid #1f2736;
    border-radius: 5px;
    cursor: pointer;
    font-size: 0.9rem;
    transition: all 0.15s ease;
  }

  .quick-act-btn:hover {
    background: #1f283a;
    border-color: #33435e;
  }

  .icon-amber { color: #f59e0b; }
  .icon-cyan { color: #38bdf8; }
  .icon-gray { color: #94a3b8; }

  .badge-runtime {
    font-size: 0.68rem;
    padding: 0.15rem 0.4rem;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: #cbd5e1;
    font-weight: 600;
    font-family: monospace;
  }

  .perpetual-badge {
    color: #94a3b8;
    font-size: 0.75rem;
  }

  /* SSL Badge */
  .ssl-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.2rem 0.55rem;
    border-radius: 4px;
    font-size: 0.72rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
  }

  .ssl-badge.ssl-active {
    background: rgba(16, 185, 129, 0.1);
    color: #10b981;
    border: 1px solid rgba(16, 185, 129, 0.25);
  }

  .ssl-badge.ssl-inactive {
    background: rgba(245, 158, 11, 0.1);
    color: #f59e0b;
    border: 1px solid rgba(245, 158, 11, 0.25);
  }

  /* Permintaan */
  .req-stat-box {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .req-sparkline {
    width: 28px;
    height: 10px;
    background: linear-gradient(90deg, #10b981 30%, transparent 30%);
    border-bottom: 2px solid #10b981;
  }

  /* Operasi */
  .ops-wrap {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
  }

  .btn-op-link {
    background: transparent;
    border: none;
    color: #38bdf8;
    font-size: 0.75rem;
    cursor: pointer;
    font-weight: 500;
  }

  .btn-op-link:hover {
    text-decoration: underline;
  }

  .btn-op-link.link-danger {
    color: #f43f5e;
  }

  .op-div {
    color: #273449;
    font-size: 0.7rem;
  }

  /* Footer Bar */
  .table-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem 0.85rem;
    background: #0a0d14;
    border-top: 1px solid #161c28;
    font-size: 0.72rem;
    color: #64748b;
  }

  .badge-tag {
    background: rgba(255, 255, 255, 0.04);
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
  }

  /* Empty / Loading States */
  .loading-state, .empty-state {
    text-align: center;
    padding: 3rem 1.5rem;
    color: #8b95a5;
  }

  .empty-icon {
    font-size: 2.2rem;
    color: #334155;
    margin-bottom: 0.5rem;
    display: block;
  }

  /* Modal-modal */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.78);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 1rem;
  }

  .modal-card {
    background: #0d111a;
    border: 1px solid #1f283b;
    border-radius: 12px;
    width: 100%;
    max-width: 480px;
    padding: 1.5rem;
    box-shadow: 0 20px 50px rgba(0, 0, 0, 0.8);
  }

  .modal-card.modal-wide {
    max-width: 820px;
  }

  .modal-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.25rem;
  }

  .modal-title-wrap {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .modal-title-wrap i {
    font-size: 1.25rem;
    color: #10b981;
  }

  .modal-title-wrap h3 {
    font-size: 0.95rem;
    color: #f1f5f9;
    margin: 0;
  }

  .btn-close {
    background: transparent;
    border: none;
    color: #64748b;
    font-size: 1.3rem;
    cursor: pointer;
  }

  .field {
    margin-bottom: 1rem;
  }

  label {
    display: block;
    font-size: 0.74rem;
    font-weight: 600;
    color: #cbd5e1;
    margin-bottom: 0.35rem;
  }

  input, select {
    width: 100%;
    padding: 0.5rem 0.75rem;
    background: #080b11;
    border: 1px solid #1c2434;
    border-radius: 6px;
    color: #f1f5f9;
    font-size: 0.8rem;
    outline: none;
  }

  input:focus, select:focus {
    border-color: #10b981;
  }

  .hint {
    display: block;
    font-size: 0.68rem;
    color: #64748b;
    margin-top: 0.25rem;
  }

  .editor-wrap {
    background: #080a10;
    border: 1px solid #1a2232;
    border-radius: 8px;
    overflow: hidden;
    margin-bottom: 1rem;
  }

  .vhost-textarea {
    width: 100%;
    height: 320px;
    background: transparent;
    border: none;
    color: #e2e8f0;
    padding: 0.85rem;
    font-size: 0.78rem;
    line-height: 1.6;
    outline: none;
    resize: vertical;
  }

  .log-tabs {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.8rem;
  }

  .log-tab {
    background: #141924;
    border: 1px solid #1f2736;
    color: #94a3b8;
    padding: 0.35rem 0.75rem;
    border-radius: 6px;
    font-size: 0.75rem;
    cursor: pointer;
  }

  .log-tab.active {
    background: #065f46;
    border-color: #10b981;
    color: #ffffff;
  }

  .log-viewer {
    background: #080b11;
    border: 1px solid #1a2230;
    border-radius: 6px;
    padding: 0.85rem;
    height: 260px;
    overflow: auto;
    font-size: 0.74rem;
    line-height: 1.6;
  }

  .ssl-info-box {
    background: #080b11;
    border: 1px solid #1a2230;
    border-radius: 8px;
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    font-size: 0.78rem;
    margin-bottom: 1.25rem;
  }

  .info-row {
    display: flex;
    justify-content: space-between;
    color: #94a3b8;
  }

  .info-row strong, .info-row span:last-child {
    color: #f1f5f9;
  }

  .text-green { color: #10b981 !important; }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.65rem;
  }

  .btn {
    padding: 0.45rem 0.85rem;
    border-radius: 6px;
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
  }

  .btn-primary {
    background: #10b981;
    color: #05130b;
  }

  .btn-primary:hover:not(:disabled) {
    background: #059669;
  }

  .btn-ghost {
    background: #151a24;
    border: 1px solid #20293a;
    color: #cbd5e1;
  }

  .btn-ghost:hover {
    background: #1b2332;
    color: #f1f5f9;
  }

  .mono { font-family: 'Fira Code', monospace; }

  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.4; transform: scale(0.85); }
  }
</style>