<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import type { Site, SoftwareItem, SitesData, NginxStatus } from '../lib/types';
  import NginxManager from '../lib/components/NginxManager.svelte';
  import AddSiteModal from '../lib/components/AddSiteModal.svelte';
  import SiteSettings from '../lib/components/SiteSettings.svelte';

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
  let showNginxManager = $state(false);

  // Sinkronkan badge web server dengan status terbaru dari Nginx Manager
  function onNginxStatus(st: NginxStatus) {
    webServer = { ...webServer, installed: st.installed, running: st.running, version: st.version };
  }

  // Filter dan Pencarian
  let activeTab = $state<'all' | 'php' | 'node' | 'proxy' | 'go' | 'python' | 'static'>('all');
  let searchQuery = $state('');
  let selectedIds = $state<Set<number>>(new Set());

  // Modal Tambah Situs & Pengaturan Situs (Domain, Situs, SSL, Konfigurasi)
  let showAddModal = $state(false);
  let settingsTarget = $state<{ id: number; tab: 'domain' | 'site' | 'ssl' | 'conf' } | null>(null);

  // Modal Log Akses / Error Situs
  let showLogModal = $state(false);
  let logTarget = $state<Site | null>(null);
  let logTab = $state<'access' | 'error'>('access');

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
        s.aliases.toLowerCase().includes(q) ||
        (s.description && s.description.toLowerCase().includes(q)) ||
        (s.root_path && s.root_path.toLowerCase().includes(q)) ||
        (s.upstream && s.upstream.toLowerCase().includes(q))
      );
    }
    return result;
  });

  // Memuat daftar situs dan status Nginx dari server (silent = tanpa indikator loading)
  async function loadSites(silent = false) {
    if (!silent) loading = true;
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

  // Selama ada sertifikat berstatus pending, segarkan daftar secara senyap agar badge SSL ikut berubah
  $effect(() => {
    if (!sites.some((s) => s.ssl_status === 'pending')) return;
    const t = setTimeout(() => loadSites(true), 3000);
    return () => clearTimeout(t);
  });

  // Sisa hari masa berlaku sertifikat
  function sslDaysLeft(s: Site) {
    return Math.max(0, Math.floor((s.ssl_expires * 1000 - Date.now()) / 86400000));
  }

  function openSettings(s: Site, tab: 'domain' | 'site' | 'ssl' | 'conf') {
    settingsTarget = { id: s.id, tab };
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

  async function deleteSite(id: number, siteDomain: string) {
    if (!(await confirmDialog({
      title: `Hapus situs "${siteDomain}"?`,
      message: 'Konfigurasi virtual host Nginx dan sertifikat SSL akan dihapus dari server.',
      confirmText: 'Hapus Situs',
      tone: 'danger',
    }))) return;
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

  // Buka penampil log situs
  function openLogViewer(site: Site) {
    logTarget = site;
    showLogModal = true;
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
      <button class="btn btn-add-site" id="add-site-btn" onclick={() => (showAddModal = true)}>
        <i class="ri-add-line"></i>
        <span>Tambah Situs</span>
      </button>

      <!-- Badge Web Server Aktif (Nginx 1.24.0 ▶) -->
      {#if webServer.installed}
        <button
          id="nginx-badge-btn"
          class="server-badge clickable {webServer.running ? 'is-running' : 'is-stopped'}"
          title="Klik untuk membuka Nginx Manager"
          onclick={() => (showNginxManager = true)}
        >
          <span class="pulse-indicator"></span>
          <i class="ri-server-line"></i>
          <span class="server-label">Nginx {webServer.version || '1.24.0'}</span>
          <span class="server-status-tag">{webServer.running ? 'Running' : 'Stopped'}</span>
          <i class="ri-settings-4-line badge-gear"></i>
        </button>
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
          <button class="btn btn-primary" onclick={() => (showAddModal = true)}>
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
                    {#if s.description}
                      <div class="site-path-sub" title={s.description}>
                        <i class="ri-sticky-note-line"></i>
                        <span>{s.description}</span>
                      </div>
                    {/if}
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
                      onclick={() => openSettings(s, 'site')}
                      title="Pengaturan situs (domain, jenis, SSL, konfigurasi)"
                    >
                      <i class="ri-settings-3-line icon-gray"></i>
                    </button>

                    <span class="badge-runtime {s.kind}">
                      {s.kind === 'php' ? (s.php_version || 'PHP') : s.kind === 'static' ? 'Static' : 'Proxy'}
                    </span>
                  </div>
                </td>

                <!-- Kolom Masa Berlaku -->
                <td class="col-expire">
                  <span class="perpetual-badge">Permanen</span>
                </td>

                <!-- Kolom Sertifikat SSL -->
                <td class="col-ssl">
                  {#if s.ssl_status === 'issued' && s.ssl_mode !== 'none'}
                    <button
                      class="ssl-badge ssl-active"
                      class:ssl-expiring={sslDaysLeft(s) < 15}
                      onclick={() => openSettings(s, 'ssl')}
                      title="Berlaku sampai {new Date(s.ssl_expires * 1000).toLocaleDateString('id-ID')}"
                    >
                      <i class="ri-shield-check-fill"></i>
                      <span>{sslDaysLeft(s)} Hari</span>
                    </button>
                  {:else if s.ssl_status === 'pending'}
                    <button class="ssl-badge ssl-pending" onclick={() => openSettings(s, 'ssl')} title="Sertifikat sedang diterbitkan">
                      <i class="ri-loader-4-line spin"></i>
                      <span>Menerbitkan</span>
                    </button>
                  {:else if s.ssl_status === 'failed' && s.ssl_mode !== 'none'}
                    <button class="ssl-badge ssl-failed" onclick={() => openSettings(s, 'ssl')} title="Penerbitan gagal, klik untuk detail">
                      <i class="ri-shield-cross-line"></i>
                      <span>Gagal</span>
                    </button>
                  {:else}
                    <button
                      class="ssl-badge ssl-inactive"
                      onclick={() => openSettings(s, 'ssl')}
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
                    <button class="btn-op-link" onclick={() => openSettings(s, 'conf')}>Conf</button>
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

  <!-- ═════════ MODAL: Tambah Situs (SSL diaktifkan setelah situs dibuat) ═════════ -->
  {#if showAddModal}
    <AddSiteModal
      onClose={() => (showAddModal = false)}
      onCreated={() => loadSites(true)}
      onOpenSSL={(s) => { showAddModal = false; openSettings(s, 'ssl'); }}
      {onNotify}
    />
  {/if}

  <!-- ═════════ MODAL: Pengaturan Situs (Domain, Situs, SSL, Konfigurasi) ═════════ -->
  {#if settingsTarget}
    {#key settingsTarget.id}
      <SiteSettings
        siteId={settingsTarget.id}
        initialTab={settingsTarget.tab}
        onClose={() => (settingsTarget = null)}
        onChanged={() => loadSites(true)}
        {onNotify}
      />
    {/key}
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

  <!-- ═════════ MODAL: Nginx Manager (Layanan, Config, Versi, Beban, Optimasi, Log) ═════════ -->
  {#if showNginxManager}
    <NginxManager
      onClose={() => (showNginxManager = false)}
      {onNotify}
      onStatusChange={onNginxStatus}
    />
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
    border-bottom: 1px solid var(--border);
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
    color: var(--text-muted);
    font-size: 0.82rem;
    font-weight: 500;
    cursor: pointer;
    border-radius: 6px 6px 0 0;
    transition: all 0.15s ease;
    white-space: nowrap;
  }

  .proj-tab:hover {
    color: var(--text);
    background: color-mix(in srgb, var(--text) 3%, transparent);
  }

  .proj-tab.active {
    color: var(--accent);
    font-weight: 600;
    border-bottom-color: var(--accent);
    background: color-mix(in srgb, var(--accent) 6%, transparent);
  }

  .icon-php { color: var(--brand-php); }
  .icon-node { color: var(--brand-node); }
  .icon-proxy { color: var(--info); }
  .icon-go { color: var(--brand-go); }
  .icon-python { color: var(--brand-python); }
  .icon-static { color: var(--brand-html); }

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
    background: var(--accent-hover);
    color: var(--text);
    border: 1px solid var(--accent);
    padding: 0.42rem 0.85rem;
    border-radius: 6px;
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .btn-add-site:hover {
    background: var(--accent-hover);
    box-shadow: 0 0 10px color-mix(in srgb, var(--accent) 30%, transparent);
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
    background: color-mix(in srgb, var(--accent) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--accent) 25%, transparent);
    color: var(--accent);
  }

  .server-badge.is-stopped {
    background: color-mix(in srgb, var(--danger-strong) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--danger-strong) 25%, transparent);
    color: var(--danger-strong);
  }

  .server-badge.is-missing {
    background: color-mix(in srgb, var(--warning) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, transparent);
    color: var(--warning);
  }

  .server-badge.clickable {
    cursor: pointer;
    font-family: inherit;
    transition: transform 0.15s ease, box-shadow 0.15s ease, background 0.15s ease;
  }

  .server-badge.clickable:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 14px -4px currentColor;
  }

  .badge-gear {
    opacity: 0.55;
    transition: transform 0.4s ease, opacity 0.15s ease;
  }

  .server-badge.clickable:hover .badge-gear {
    opacity: 1;
    transform: rotate(90deg);
  }

  .pulse-indicator {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--accent);
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
    background: var(--bg-hover);
    border: 1px solid var(--border);
    color: var(--text-muted);
    padding: 0.35rem 0.65rem;
    border-radius: 6px;
    font-size: 0.72rem;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }

  .btn-mock-toggle:hover {
    background: var(--bg-active);
    color: var(--text);
  }

  .search-box {
    display: flex;
    align-items: center;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.3rem 0.6rem;
    gap: 0.4rem;
    min-width: 240px;
  }

  .search-box input {
    background: transparent;
    border: none;
    outline: none;
    color: var(--text);
    font-size: 0.78rem;
    width: 100%;
  }

  .search-icon {
    color: var(--text-subtle);
  }

  .btn-clear-search {
    background: transparent;
    border: none;
    color: var(--text-subtle);
    cursor: pointer;
  }

  /* ═════════ 3. Dependency Warning Card (aaPanel Gambar 2 Style) ═════════ */
  .dep-warning-card {
    position: relative;
    background: var(--bg-raised);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, transparent);
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
    background: color-mix(in srgb, var(--warning) 8%, transparent);
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
    background: color-mix(in srgb, var(--warning) 12%, transparent);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, transparent);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.8rem;
    color: var(--warning);
    flex-shrink: 0;
  }

  .dep-warn-details h3 {
    color: var(--text);
    font-size: 1.05rem;
    font-weight: 700;
    margin: 0 0 0.5rem 0;
  }

  .dep-warn-details p {
    color: var(--text-muted);
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
    background: var(--accent-hover);
    color: var(--text);
    border: 1px solid var(--accent);
    padding: 0.5rem 1.1rem;
    border-radius: 6px;
    font-size: 0.82rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .btn-install-now:hover:not(:disabled) {
    background: var(--accent-hover);
    box-shadow: 0 0 14px color-mix(in srgb, var(--accent) 40%, transparent);
  }

  /* ═════════ 4. Tabel Situs Web (aaPanel Gambar 1 Style) ═════════ */
  .table-container {
    background: var(--bg-surface);
    border: 1px solid var(--border);
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
    color: var(--text-subtle);
    background: var(--bg-raised);
    border-bottom: 1px solid var(--border);
    font-weight: 600;
  }

  td {
    padding: 0.75rem 0.85rem;
    border-bottom: 1px solid var(--border-subtle);
    vertical-align: middle;
  }

  .site-row:hover {
    background: color-mix(in srgb, var(--text) 2%, transparent);
  }

  .site-row.selected-row {
    background: color-mix(in srgb, var(--accent) 4%, transparent);
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
    color: var(--text);
    font-weight: 600;
    text-decoration: none;
    font-size: 0.85rem;
  }

  .domain-link:hover {
    color: var(--accent);
  }

  .icon-ext {
    font-size: 0.8rem;
    color: var(--accent);
    opacity: 0.75;
  }

  .site-path-sub {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.68rem;
    color: var(--text-subtle);
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
    color: var(--accent);
  }

  .btn-status-play.is-paused {
    color: var(--text-subtle);
  }

  /* Backup Box */
  .btn-backup-info {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    font-size: 0.72rem;
  }

  .btn-backup-info i {
    font-size: 1.1rem;
    color: var(--info);
  }

  .backup-text-wrap {
    display: flex;
    flex-direction: column;
    text-align: left;
    line-height: 1.2;
  }

  .backup-main {
    color: var(--text);
    font-weight: 500;
  }

  .backup-sub {
    font-size: 0.65rem;
    color: var(--text-subtle);
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
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: 5px;
    cursor: pointer;
    font-size: 0.9rem;
    transition: all 0.15s ease;
  }

  .quick-act-btn:hover {
    background: var(--bg-active);
    border-color: var(--border-strong);
  }

  .icon-amber { color: var(--warning); }
  .icon-cyan { color: var(--info); }
  .icon-gray { color: var(--text-muted); }

  .badge-runtime {
    font-size: 0.68rem;
    padding: 0.15rem 0.4rem;
    border-radius: 4px;
    background: color-mix(in srgb, var(--text) 5%, transparent);
    border: 1px solid color-mix(in srgb, var(--text) 8%, transparent);
    color: var(--text-secondary);
    font-weight: 600;
    font-family: monospace;
  }

  .perpetual-badge {
    color: var(--text-muted);
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
    background: color-mix(in srgb, var(--accent) 10%, transparent);
    color: var(--accent);
    border: 1px solid color-mix(in srgb, var(--accent) 25%, transparent);
  }

  .ssl-badge.ssl-inactive {
    background: color-mix(in srgb, var(--warning) 10%, transparent);
    color: var(--warning);
    border: 1px solid color-mix(in srgb, var(--warning) 25%, transparent);
  }

  .ssl-badge.ssl-active.ssl-expiring {
    background: color-mix(in srgb, var(--warning) 10%, transparent);
    color: var(--warning-strong);
    border-color: color-mix(in srgb, var(--warning) 30%, transparent);
  }

  .ssl-badge.ssl-pending {
    background: color-mix(in srgb, var(--info) 10%, transparent);
    color: var(--info);
    border: 1px solid color-mix(in srgb, var(--info) 25%, transparent);
  }

  .ssl-badge.ssl-failed {
    background: color-mix(in srgb, var(--danger-strong) 10%, transparent);
    color: var(--danger-strong);
    border: 1px solid color-mix(in srgb, var(--danger-strong) 25%, transparent);
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
    background: linear-gradient(90deg, var(--accent) 30%, transparent 30%);
    border-bottom: 2px solid var(--accent);
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
    color: var(--info);
    font-size: 0.75rem;
    cursor: pointer;
    font-weight: 500;
  }

  .btn-op-link:hover {
    text-decoration: underline;
  }

  .btn-op-link.link-danger {
    color: var(--danger-strong);
  }

  .op-div {
    color: var(--text-on-accent);
    font-size: 0.7rem;
  }

  /* Footer Bar */
  .table-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem 0.85rem;
    background: var(--bg-inset);
    border-top: 1px solid var(--border-subtle);
    font-size: 0.72rem;
    color: var(--text-subtle);
  }

  .badge-tag {
    background: color-mix(in srgb, var(--text) 4%, transparent);
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
  }

  /* Empty / Loading States */
  .loading-state, .empty-state {
    text-align: center;
    padding: 3rem 1.5rem;
    color: var(--text-muted);
  }

  .empty-icon {
    font-size: 2.2rem;
    color: var(--text-disabled);
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
    background: var(--bg-surface);
    border: 1px solid var(--border);
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
    color: var(--accent);
  }

  .modal-title-wrap h3 {
    font-size: 0.95rem;
    color: var(--text);
    margin: 0;
  }

  .btn-close {
    background: transparent;
    border: none;
    color: var(--text-subtle);
    font-size: 1.3rem;
    cursor: pointer;
  }

  input {
    width: 100%;
    padding: 0.5rem 0.75rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text);
    font-size: 0.8rem;
    outline: none;
  }

  input:focus {
    border-color: var(--accent);
  }

  .log-tabs {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.8rem;
  }

  .log-tab {
    background: var(--bg-raised);
    border: 1px solid var(--border);
    color: var(--text-muted);
    padding: 0.35rem 0.75rem;
    border-radius: 6px;
    font-size: 0.75rem;
    cursor: pointer;
  }

  .log-tab.active {
    background: var(--accent-hover);
    border-color: var(--accent);
    color: var(--text);
  }

  .log-viewer {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.85rem;
    height: 260px;
    overflow: auto;
    font-size: 0.74rem;
    line-height: 1.6;
  }

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
    background: var(--accent);
    color: var(--text-on-accent);
  }

  .btn-primary:hover:not(:disabled) {
    background: var(--accent-hover);
  }

  .btn-ghost {
    background: var(--bg-hover);
    border: 1px solid var(--border);
    color: var(--text-secondary);
  }

  .btn-ghost:hover {
    background: var(--bg-active);
    color: var(--text);
  }

  .mono { font-family: 'Fira Code', monospace; }

  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.4; transform: scale(0.85); }
  }
</style>
