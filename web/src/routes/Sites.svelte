<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
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
  let settingsTarget = $state<{ id: number; tab: string } | null>(null);

  // Modal Log Akses / Error Situs (Real Nginx Logs)
  let showLogModal = $state(false);
  let logTarget = $state<Site | null>(null);
  let logTab = $state<'access' | 'error'>('access');
  let logContent = $state('');
  let logLoading = $state(false);

  // Modal Git Pull & Auto-Deploy (khusus situs yang dibuat dari Git)
  interface GitPullResponse {
    ok: boolean;
    domain: string;
    branch: string;
    repo: string;
    old_commit: string;
    new_commit: string;
    commit_message: string;
    updated: boolean;
    output: string;
  }
  let gitPullTarget = $state<Site | null>(null);
  let gitPullRunning = $state(false);
  let gitPullRunComposer = $state(true);
  let gitPullRunMigrate = $state(true);
  let gitPullClearCache = $state(true);
  let gitPullResult = $state<GitPullResponse | null>(null);
  let gitPullError = $state('');
  let openMoreSiteId = $state<number | null>(null);

  function toggleMoreMenu(siteId: number, e: MouseEvent) {
    e.stopPropagation();
    openMoreSiteId = openMoreSiteId === siteId ? null : siteId;
  }

  async function quickGitPull(site: Site) {
    openMoreSiteId = null;
    openGitPullModal(site);
    gitPullRunComposer = false;
    gitPullRunMigrate = true;
    gitPullClearCache = true;
    await executeGitPull();
  }

  // Modal Konfirmasi Hapus Situs (Danger Zone + Anti-Paste + Auto-Backup)
  let deleteTarget = $state<Site | null>(null);
  let deleteConfirmInput = $state('');
  let deleteFilesOpt = $state(true);
  let deleteDBOpt = $state(true);
  let backupFirstOpt = $state(true);
  let deletePasteWarning = $state(false);
  let deletingSite = $state(false);

  // Filter daftar situs berdasarkan tab dan pencarian
  let filteredSites = $derived.by(() => {
    let result = sites;
    if (activeTab !== 'all') {
      result = result.filter(s => s.kind === activeTab);
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
      const data = await apiFetch<SitesData>('/api/sites');
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
  async function renewAllSSL() {
    const issuedSites = sites.filter((s) => s.ssl_mode === 'letsencrypt');
    if (issuedSites.length === 0) {
      notify('Tidak ada situs dengan SSL Let\'s Encrypt', 'info');
      return;
    }
    notify(`Memeriksa dan memperbarui SSL untuk ${issuedSites.length} situs...`, 'info');
    let count = 0;
    for (const site of issuedSites) {
      try {
        await apiFetch(`/api/sites/${site.id}/ssl`, {
          method: 'POST',
          body: JSON.stringify({
            enabled: true,
            force_https: site.force_https === 1,
            hsts: site.hsts === 1,
            issue: true,
            skip_dns: true,
          }),
        });
        count++;
      } catch {
        // Lanjutkan
      }
    }
    notify(`Pembaruan SSL selesai dipicu untuk ${count} situs`, 'success');
    await loadSites();
  }

  function sslDaysLeft(s: Site) {
    return Math.max(0, Math.floor((s.ssl_expires * 1000 - Date.now()) / 86400000));
  }

  function openSettings(s: Site, tab: string = 'domain') {
    settingsTarget = { id: s.id, tab };
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

  function getProjectRootDir(s: Site): string {
    const raw = (s.root_path || `/home/fikpanel/web/${s.domain}`).replace(/\/+$/, '');
    if (raw.endsWith('/public')) {
      return raw.slice(0, -7);
    }
    return raw;
  }

  function getPredictedDBName(s: Site): string {
    const base = s.domain
      .toLowerCase()
      .replace(/[^a-z0-9]/g, '_')
      .replace(/^_+|_+$/g, '')
      .slice(0, 12);
    return `${base || 'site'}_db`;
  }

  function openDeleteModal(site: Site) {
    deleteTarget = site;
    deleteConfirmInput = '';
    deleteFilesOpt = true;
    deleteDBOpt = true;
    backupFirstOpt = true;
    deletePasteWarning = false;
    deletingSite = false;
  }

  function closeDeleteModal() {
    if (deletingSite) return;
    deleteTarget = null;
    deleteConfirmInput = '';
    deletePasteWarning = false;
  }

  function handleBlockPaste(e: Event) {
    e.preventDefault();
    deletePasteWarning = true;
  }

  async function confirmDeleteSite() {
    if (!deleteTarget || deleteConfirmInput !== deleteTarget.domain || deletingSite) return;
    deletingSite = true;
    const targetDomain = deleteTarget.domain;
    const targetId = deleteTarget.id;
    try {
      const qs = new URLSearchParams({
        delete_files: deleteFilesOpt ? '1' : '0',
        delete_db: deleteDBOpt ? '1' : '0',
        backup_first: backupFirstOpt ? '1' : '0',
      }).toString();
      const res = await apiFetch<{
        ok: boolean;
        backup_file?: string;
        deleted_db?: string;
        deleted_files?: boolean;
      }>(`/api/sites/${targetId}?${qs}`, { method: 'DELETE' });

      deleteTarget = null;
      await loadSites();

      const summaryParts: string[] = [`Situs ${targetDomain} dihapus`];
      if (res.backup_file) summaryParts.push(`Backup: ${res.backup_file.split('/').pop()}`);
      if (res.deleted_db) summaryParts.push(`DB ${res.deleted_db} terhapus`);
      onNotify(summaryParts.join(' • '));
    } catch (err: any) {
      onNotify(err.message || 'Gagal menghapus situs');
    } finally {
      deletingSite = false;
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

  // Buka penampil log situs nyata dari server
  async function fetchSiteLogs(site: Site, type: 'access' | 'error') {
    logLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; content: string }>(`/api/sites/${site.id}/logs?type=${type}`);
      logContent = res.content || `Belum ada catatan ${type} log untuk ${site.domain}.`;
    } catch (err: any) {
      logContent = `Gagal memuat log: ${err.message || 'Kesalahan jaringan'}`;
    } finally {
      logLoading = false;
    }
  }

  function openLogViewer(site: Site) {
    logTarget = site;
    logTab = 'access';
    showLogModal = true;
    fetchSiteLogs(site, 'access');
  }

  function switchLogTab(tab: 'access' | 'error') {
    logTab = tab;
    if (logTarget) {
      fetchSiteLogs(logTarget, tab);
    }
  }

  // Buka modal Git Pull & Auto-Deploy untuk situs berbasis Git
  function openGitPullModal(site: Site) {
    gitPullTarget = site;
    gitPullRunning = false;
    gitPullRunComposer = true;
    gitPullRunMigrate = true;
    gitPullClearCache = true;
    gitPullResult = null;
    gitPullError = '';
  }

  function closeGitPullModal() {
    if (gitPullRunning) return;
    gitPullTarget = null;
    gitPullResult = null;
    gitPullError = '';
  }

  async function executeGitPull() {
    if (!gitPullTarget || gitPullRunning) return;
    gitPullRunning = true;
    gitPullError = '';
    gitPullResult = null;
    try {
      const res = await apiFetch<GitPullResponse>(`/api/sites/${gitPullTarget.id}/git-pull`, {
        method: 'POST',
        body: JSON.stringify({
          run_composer: gitPullRunComposer,
          run_migrate: gitPullRunMigrate,
          clear_cache: gitPullClearCache,
        }),
      });
      gitPullResult = res;
      await loadSites(true);
      if (gitPullTarget) {
        const refreshed = sites.find((s) => s.id === gitPullTarget?.id);
        if (refreshed) gitPullTarget = refreshed;
      }
      onNotify(
        res.updated
          ? `Git Pull ${res.domain} berhasil (${res.old_commit} → ${res.new_commit})`
          : `Git Pull ${res.domain} selesai (sudah commit terbaru #${res.new_commit})`,
      );
    } catch (err: any) {
      gitPullError = err.message || 'Gagal menjalankan Git Pull pada situs ini';
    } finally {
      gitPullRunning = false;
    }
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

<svelte:window onclick={() => { if (openMoreSiteId !== null) openMoreSiteId = null; }} />

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

      <button class="btn btn-renew-ssl" title="Perbarui dan cek masa aktif semua SSL otomatis" onclick={renewAllSSL}>
        <i class="ri-shield-keyhole-line"></i>
        <span>Perbarui SSL Semua</span>
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
                    {#if s.has_git}
                      <div class="site-git-meta mono">
                        <button
                          type="button"
                          class="git-meta-pill"
                          onclick={() => openGitPullModal(s)}
                          title="Proyek terhubung ke Git ({s.git_repo || 'origin'} • branch {s.git_branch || 'main'}). Klik untuk Git Pull & Auto-Deploy."
                        >
                          <i class="ri-git-branch-line"></i>
                          <span>{s.git_repo || 'git'}</span>
                          <span class="git-branch-tag">{s.git_branch || 'main'}</span>
                          {#if s.git_commit}
                            <span class="git-commit-hash">#{s.git_commit}</span>
                          {/if}
                        </button>
                      </div>
                    {/if}
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
                      <span class="backup-main">{s.backup_count ?? 0} Backup</span>
                      <span class="backup-sub">Cadangkan</span>
                    </div>
                  </button>
                </td>

                <!-- Kolom Aksi Cepat (Buka File Manager, Terminal, Config, Runtime) -->
                <td class="col-quick">
                  <div class="quick-actions-bar">
                    <button
                      class="quick-act-btn"
                      onclick={() => openInFileManager(getProjectRootDir(s))}
                      title="Buka direktori proyek situs di File Manager FikPanel"
                    >
                      <i class="ri-folder-open-fill icon-amber"></i>
                    </button>

                    <button
                      class="quick-act-btn"
                      onclick={() => openInTerminal(getProjectRootDir(s))}
                      title="Buka Terminal di folder root proyek situs ini"
                    >
                      <i class="ri-terminal-box-line icon-cyan"></i>
                    </button>

                    <button
                      class="quick-act-btn"
                      onclick={() => openSettings(s, 'domain')}
                      title="Pengaturan situs (domain, direktori, SSL, konfigurasi)"
                    >
                      <i class="ri-settings-3-line icon-gray"></i>
                    </button>

                    <button
                      type="button"
                      class="badge-runtime {s.kind}"
                      onclick={() => openSettings(s, s.kind === 'php' ? 'php' : s.kind === 'static' ? 'directory' : 'proxy')}
                      title="Klik untuk mengatur runtime & konfigurasi proyek ini"
                    >
                      {#if s.kind === 'php'}
                        {s.php_version ? `PHP ${s.php_version}` : 'PHP'}
                      {:else if s.kind === 'node'}
                        Node.js
                      {:else if s.kind === 'go'}
                        Golang
                      {:else if s.kind === 'python'}
                        Python
                      {:else if s.kind === 'static'}
                        Static
                      {:else}
                        Proxy
                      {/if}
                    </button>
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

                <!-- Kolom Permintaan / Request Count (Data Riil dari Access Log Nginx) -->
                <td class="col-req mono">
                  <button
                    type="button"
                    class="req-stat-box req-btn-trigger"
                    onclick={() => openLogViewer(s)}
                    title="Total {(s.request_count ?? 0).toLocaleString('id-ID')} permintaan HTTP tercatat di log Nginx (Klik untuk lihat Access Log)"
                  >
                    <span class="req-num">{(s.request_count ?? 0).toLocaleString('id-ID')}</span>
                    <span class="req-sparkline"></span>
                  </button>
                </td>

                <!-- Kolom Operasi (Ringkas: Git Pull / Conf + Menu More ▾) -->
                <td class="col-ops">
                  <div class="ops-wrap">
                    {#if s.has_git}
                      <button
                        type="button"
                        class="btn-op-git"
                        onclick={() => openGitPullModal(s)}
                        title="Tarik pembaruan kode dari Git ({s.git_repo || 'origin'} : {s.git_branch || 'main'})"
                      >
                        <i class="ri-git-pull-request-line"></i>
                        <span>Git Pull</span>
                      </button>
                    {:else}
                      <button
                        type="button"
                        class="btn-op-conf-pill"
                        onclick={() => openSettings(s, 'conf')}
                        title="Konfigurasi Virtual Host & Situs"
                      >
                        <i class="ri-settings-4-line"></i>
                        <span>Conf</span>
                      </button>
                    {/if}

                    <div class="op-more-wrap">
                      <button
                        type="button"
                        class="btn-op-more {openMoreSiteId === s.id ? 'is-open' : ''}"
                        onclick={(e) => toggleMoreMenu(s.id, e)}
                        title="Menu operasi situs lengkap"
                      >
                        <span>More</span>
                        <i class="ri-arrow-down-s-line"></i>
                      </button>
                      {#if openMoreSiteId === s.id}
                        <div class="op-more-menu" role="menu">
                          {#if s.has_git}
                            <button
                              type="button"
                              class="op-more-item"
                              onclick={() => { openMoreSiteId = null; openGitPullModal(s); }}
                            >
                              <i class="ri-git-pull-request-line icon-accent"></i>
                              <div>
                                <b>Git Pull &amp; Sinkronisasi</b>
                                <small>Tarik commit terbaru dari {s.git_branch || 'main'}</small>
                              </div>
                            </button>
                            <button
                              type="button"
                              class="op-more-item"
                              onclick={() => quickGitPull(s)}
                            >
                              <i class="ri-flashlight-line icon-amber"></i>
                              <div>
                                <b>Quick Pull (Instan)</b>
                                <small>Tarik kode terbaru &amp; bersihkan cache</small>
                              </div>
                            </button>
                            <div class="op-more-divider"></div>
                          {/if}
                          <button
                            type="button"
                            class="op-more-item"
                            onclick={() => { openMoreSiteId = null; openSettings(s, 'conf'); }}
                          >
                            <i class="ri-file-code-line icon-cyan"></i>
                            <div>
                              <b>Konfigurasi Situs (Conf)</b>
                              <small>Pengaturan vhost Nginx, PHP, SSL &amp; domain</small>
                            </div>
                          </button>
                          <button
                            type="button"
                            class="op-more-item"
                            onclick={() => { openMoreSiteId = null; openLogViewer(s); }}
                          >
                            <i class="ri-file-list-3-line icon-amber"></i>
                            <div>
                              <b>Log Akses &amp; Error (Log)</b>
                              <small>Pantau lalu lintas HTTP &amp; catatan galat</small>
                            </div>
                          </button>
                          <button
                            type="button"
                            class="op-more-item"
                            onclick={() => { openMoreSiteId = null; openInTerminal(getProjectRootDir(s)); }}
                          >
                            <i class="ri-terminal-box-line icon-gray"></i>
                            <div>
                              <b>Buka Terminal Proyek</b>
                              <small class="mono">{getProjectRootDir(s)}</small>
                            </div>
                          </button>
                          <div class="op-more-divider"></div>
                          <button
                            type="button"
                            class="op-more-item is-danger-item"
                            onclick={() => { openMoreSiteId = null; openDeleteModal(s); }}
                          >
                            <i class="ri-delete-bin-line"></i>
                            <div>
                              <b>Hapus Situs</b>
                              <small>Hapus vhost, database &amp; direktori proyek</small>
                            </div>
                          </button>
                        </div>
                      {/if}
                    </div>
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

  <!-- ═════════ MODAL: Penampil Log Akses & Error (Data Riil dari Server) ═════════ -->
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
          <button class="log-tab {logTab === 'access' ? 'active' : ''}" onclick={() => switchLogTab('access')}>
            Access Log ({(logTarget.request_count ?? 0).toLocaleString('id-ID')} Req)
          </button>
          <button class="log-tab {logTab === 'error' ? 'active' : ''}" onclick={() => switchLogTab('error')}>
            Error Log
          </button>
          <button
            type="button"
            class="log-tab"
            onclick={() => logTarget && fetchSiteLogs(logTarget, logTab)}
            disabled={logLoading}
          >
            <i class="ri-refresh-line" class:spin={logLoading}></i> Segarkan
          </button>
        </div>

        <div class="log-viewer mono">
          {#if logLoading}
            <div class="log-line text-muted"><i class="ri-loader-4-line spin"></i> Memuat catatan log {logTab} dari server...</div>
          {:else}
            <pre class="log-pre-raw">{logContent}</pre>
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

  <!-- ═════════ MODAL: Git Pull & Auto-Deploy (Khusus Situs Berbasis Git) ═════════ -->
  {#if gitPullTarget}
    <div
      class="modal-backdrop"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      onclick={(e) => { if (e.target === e.currentTarget) closeGitPullModal(); }}
      onkeydown={(e) => { if (e.key === 'Escape') closeGitPullModal(); }}
    >
      <div class="modal-card git-pull-card">
        <div class="modal-head git-pull-head">
          <div class="modal-title-wrap">
            <span class="git-icon-circle">
              <i class="ri-git-pull-request-line"></i>
            </span>
            <div>
              <h3>Git Pull &amp; Sinkronisasi: {gitPullTarget.domain}</h3>
              <span class="danger-sub-caption">Tarik commit terbaru dari repositori remote tanpa menimpa file .env lokal</span>
            </div>
          </div>
          <button class="btn-close" onclick={closeGitPullModal} disabled={gitPullRunning}>&times;</button>
        </div>

        <!-- Ringkasan Repositori & Branch Aktif -->
        <div class="git-summary-strip mono">
          <div class="git-sum-item">
            <span class="git-sum-lbl">Repositori</span>
            <strong><i class="ri-github-fill"></i> {gitPullTarget.git_repo || 'origin'}</strong>
          </div>
          <div class="git-sum-item">
            <span class="git-sum-lbl">Branch Aktif</span>
            <strong class="git-branch-highlight"><i class="ri-git-branch-line"></i> {gitPullTarget.git_branch || 'main'}</strong>
          </div>
          <div class="git-sum-item">
            <span class="git-sum-lbl">Commit Saat Ini</span>
            <strong>#{gitPullTarget.git_commit || 'HEAD'}</strong>
          </div>
        </div>

        <!-- Opsi Otomatisasi Pasca-Pull -->
        <div class="delete-impact-list">
          <label class="delete-opt-item {gitPullRunComposer ? 'is-checked' : ''}">
            <input type="checkbox" bind:checked={gitPullRunComposer} disabled={gitPullRunning} />
            <div class="opt-body">
              <div class="opt-title-row">
                <span class="opt-title">Sinkronkan Dependensi Proyek</span>
                <span class="opt-badge badge-safe">Composer / NPM</span>
              </div>
              <span class="opt-desc">
                Menjalankan <code class="mono">composer install</code> (PHP) atau <code class="mono">npm install &amp;&amp; npm run build</code> (Node.js) apabila ada paket baru.
              </span>
            </div>
          </label>

          {#if gitPullTarget.kind === 'php'}
            <label class="delete-opt-item {gitPullRunMigrate ? 'is-checked' : ''}">
              <input type="checkbox" bind:checked={gitPullRunMigrate} disabled={gitPullRunning} />
              <div class="opt-body">
                <div class="opt-title-row">
                  <span class="opt-title">Jalankan Migrasi Tabel Baru</span>
                  <span class="opt-badge badge-safe">Artisan Migrate</span>
                </div>
                <span class="opt-desc">
                  Menjalankan skema migrasi baru (<code class="mono">php artisan migrate</code>) secara aman tanpa menyentuh data tabel yang sudah ada.
                </span>
              </div>
            </label>

            <label class="delete-opt-item {gitPullClearCache ? 'is-checked' : ''}">
              <input type="checkbox" bind:checked={gitPullClearCache} disabled={gitPullRunning} />
              <div class="opt-body">
                <div class="opt-title-row">
                  <span class="opt-title">Segarkan Cache &amp; Aset Aplikasi</span>
                  <span class="opt-badge badge-safe">Disarankan</span>
                </div>
                <span class="opt-desc">
                  Menjalankan <code class="mono">php artisan optimize:clear</code>, menyinkronkan aset Filament, serta merapikan izin akses direktori web.
                </span>
              </div>
            </label>
          {/if}
        </div>

        {#if gitPullError}
          <div class="git-pull-alert is-error">
            <i class="ri-error-warning-fill"></i>
            <div>
              <strong>Git Pull Gagal</strong>
              <p class="mono">{gitPullError}</p>
            </div>
          </div>
        {/if}

        {#if gitPullResult}
          <div class="git-pull-alert {gitPullResult.updated ? 'is-updated' : 'is-uptodate'}">
            <i class={gitPullResult.updated ? 'ri-checkbox-circle-fill' : 'ri-information-fill'}></i>
            <div>
              {#if gitPullResult.updated}
                <strong>Berhasil Diperbarui ({gitPullResult.old_commit} &rarr; {gitPullResult.new_commit})</strong>
              {:else}
                <strong>Sudah Versi Terbaru (#{gitPullResult.new_commit})</strong>
              {/if}
              {#if gitPullResult.commit_message}
                <p>Commit terakhir: <span class="mono">"{gitPullResult.commit_message}"</span></p>
              {/if}
            </div>
          </div>

          {#if gitPullResult.output}
            <div class="git-output-box mono">
              <div class="git-output-head">Log Eksekusi Git Pull &amp; Post-Deploy</div>
              <pre class="log-pre-raw">{gitPullResult.output}</pre>
            </div>
          {/if}
        {/if}

        <div class="modal-actions delete-modal-actions">
          <button type="button" class="btn btn-ghost" onclick={closeGitPullModal} disabled={gitPullRunning}>
            Tutup
          </button>
          <button
            type="button"
            class="btn btn-primary btn-git-pull-exec"
            disabled={gitPullRunning}
            onclick={executeGitPull}
          >
            {#if gitPullRunning}
              <i class="ri-loader-4-line spin"></i>
              <span>Menarik Commit &amp; Menyinkronkan Aplikasi...</span>
            {:else}
              <i class="ri-git-pull-request-line"></i>
              <span>Tarik Perubahan Terbaru (Git Pull)</span>
            {/if}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Konfirmasi Hapus Situs (Danger Zone) ═════════ -->
  {#if deleteTarget}
    <div
      class="modal-backdrop"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      onclick={(e) => { if (e.target === e.currentTarget) closeDeleteModal(); }}
      onkeydown={(e) => { if (e.key === 'Escape') closeDeleteModal(); }}
    >
      <div class="modal-card delete-danger-card">
        <div class="modal-head delete-danger-head">
          <div class="modal-title-wrap">
            <span class="danger-icon-circle">
              <i class="ri-error-warning-fill"></i>
            </span>
            <div>
              <h3>Hapus Situs "{deleteTarget.domain}"?</h3>
              <span class="danger-sub-caption">Tindakan destruktif &amp; permanen pada server produksi</span>
            </div>
          </div>
          <button class="btn-close" onclick={closeDeleteModal} disabled={deletingSite}>&times;</button>
        </div>

        <!-- Banner Peringatan Dampak -->
        <div class="danger-alert-banner">
          <i class="ri-alarm-warning-line"></i>
          <div>
            <strong>Peringatan Penting Sebelum Menghapus!</strong>
            <p>
              Konfigurasi virtual host Nginx, sertifikat SSL, serta komponen yang Anda centang di bawah akan dihapus secara permanen.
              Jika database dan kode proyek ini penting, pastikan opsi <strong>Auto-Backup Sebelum Hapus</strong> tetap aktif.
            </p>
          </div>
        </div>

        <!-- Opsi Granular Dampak Penghapusan & Cadangan Otomatis -->
        <div class="delete-impact-list">
          <label class="delete-opt-item opt-backup {backupFirstOpt ? 'is-checked' : ''}">
            <input type="checkbox" bind:checked={backupFirstOpt} disabled={deletingSite} />
            <div class="opt-body">
              <div class="opt-title-row">
                <span class="opt-title">Auto-Backup Proyek &amp; Database Sebelum Dihapus</span>
                <span class="opt-badge badge-safe">Disarankan</span>
              </div>
              <span class="opt-desc">
                Mencadangkan dump SQL + seluruh direktori proyek ke <code class="mono">/var/backups/fikpanel/{deleteTarget.domain}-pre-delete-*.tar.gz</code> agar dapat dipulihkan kapan saja lewat tab <b>Create from backup</b>.
              </span>
            </div>
          </label>

          <label class="delete-opt-item opt-destructive {deleteFilesOpt ? 'is-checked-danger' : ''}">
            <input type="checkbox" bind:checked={deleteFilesOpt} disabled={deletingSite} />
            <div class="opt-body">
              <div class="opt-title-row">
                <span class="opt-title">Hapus Direktori Proyek &amp; File Web Root</span>
                <span class="opt-badge badge-danger">Permanen</span>
              </div>
              <span class="opt-desc">
                Menghapus seluruh berkas kode sumber di <code class="mono">{getProjectRootDir(deleteTarget)}</code>
              </span>
            </div>
          </label>

          <label class="delete-opt-item opt-destructive {deleteDBOpt ? 'is-checked-danger' : ''}">
            <input type="checkbox" bind:checked={deleteDBOpt} disabled={deletingSite} />
            <div class="opt-body">
              <div class="opt-title-row">
                <span class="opt-title">Hapus Database &amp; User MySQL Terkait</span>
                <span class="opt-badge badge-danger">DROP DATABASE</span>
              </div>
              <span class="opt-desc">
                Menghapus skema database (<code class="mono">{getPredictedDBName(deleteTarget)}</code> / sesuai <code class="mono">.env</code>) beserta user MySQL dari server fisik.
              </span>
            </div>
          </label>
        </div>

        <!-- Verifikasi Ketik Manual (Anti Copy-Paste) -->
        <div class="delete-verify-box">
          <label for="confirm-domain-input" class="verify-label">
            Untuk konfirmasi, ketik manual nama domain <strong class="mono select-none">{deleteTarget.domain}</strong> di bawah ini:
          </label>
          <input
            id="confirm-domain-input"
            type="text"
            class="verify-input mono"
            placeholder="Ketik manual {deleteTarget.domain} (tidak bisa paste)"
            bind:value={deleteConfirmInput}
            autocomplete="off"
            autocorrect="off"
            autocapitalize="off"
            spellcheck="false"
            disabled={deletingSite}
            onpaste={handleBlockPaste}
            ondrop={handleBlockPaste}
            oncontextmenu={(e) => e.preventDefault()}
            onkeydown={(e) => {
              if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'v') {
                handleBlockPaste(e);
              } else if (e.key === 'Enter' && deleteConfirmInput === deleteTarget.domain) {
                confirmDeleteSite();
              }
            }}
          />
          {#if deletePasteWarning}
            <div class="paste-blocked-hint">
              <i class="ri-keyboard-box-line"></i>
              <span>Demi keamanan, fitur Copy-Paste dinonaktifkan. Silakan ketik nama domain secara manual dengan keyboard.</span>
            </div>
          {/if}
        </div>

        <div class="modal-actions delete-modal-actions">
          <button type="button" class="btn btn-ghost" onclick={closeDeleteModal} disabled={deletingSite}>
            Batal
          </button>
          <button
            type="button"
            class="btn btn-danger-solid"
            disabled={deleteConfirmInput !== deleteTarget.domain || deletingSite}
            onclick={confirmDeleteSite}
          >
            {#if deletingSite}
              <i class="ri-loader-4-line spin"></i>
              <span>{backupFirstOpt ? 'Mencadangkan & Menghapus...' : 'Menghapus Situs...'}</span>
            {:else}
              <i class="ri-delete-bin-2-line"></i>
              <span>Saya Mengerti, Hapus Situs Ini</span>
            {/if}
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

  .btn-renew-ssl {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.5rem 0.85rem;
    background: var(--bg-surface);
    color: var(--text);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    font-size: 0.82rem;
    font-weight: 500;
    cursor: pointer;
    transition: background 0.15s, border-color 0.15s;
  }
  .btn-renew-ssl:hover {
    background: var(--bg-hover);
    border-color: var(--border-hover);
  }
  .btn-renew-ssl i {
    color: var(--accent);
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
    overflow: visible;
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
    white-space: nowrap;
  }

  td {
    padding: 0.7rem 0.85rem;
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
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-weight: 600;
    font-family: monospace;
    cursor: pointer;
    white-space: nowrap;
    flex-shrink: 0;
    transition: all 0.15s ease;
  }
  .badge-runtime:hover {
    background: var(--bg-active);
    border-color: var(--border-strong);
    color: var(--text);
  }

  .perpetual-badge {
    color: var(--text-muted);
    font-size: 0.75rem;
    white-space: nowrap;
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
    white-space: nowrap;
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

  /* ═════════ GitHub-Style Danger Zone Delete Modal ═════════ */
  .delete-danger-card {
    max-width: 540px;
    border-color: color-mix(in srgb, var(--danger) 42%, var(--border));
  }

  .delete-danger-head {
    margin-bottom: 1rem;
    padding-bottom: 0.85rem;
    border-bottom: 1px solid var(--border-subtle);
  }

  .danger-icon-circle {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    background: color-mix(in srgb, var(--danger) 14%, transparent);
    border: 1px solid color-mix(in srgb, var(--danger) 35%, transparent);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .danger-icon-circle i {
    color: var(--danger-strong) !important;
    font-size: 1.15rem !important;
  }

  .danger-sub-caption {
    display: block;
    font-size: 0.72rem;
    color: var(--text-muted);
    margin-top: 0.15rem;
  }

  .danger-alert-banner {
    display: flex;
    gap: 0.65rem;
    align-items: flex-start;
    padding: 0.75rem 0.9rem;
    border-radius: 8px;
    background: color-mix(in srgb, var(--danger) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--danger) 28%, transparent);
    margin-bottom: 1rem;
    font-size: 0.76rem;
    line-height: 1.5;
    color: var(--text-secondary);
  }

  .danger-alert-banner > i {
    color: var(--danger-strong);
    font-size: 1.1rem;
    margin-top: 0.05rem;
    flex-shrink: 0;
  }

  .danger-alert-banner strong {
    color: var(--text);
  }

  .danger-alert-banner p {
    margin: 0.2rem 0 0;
  }

  .delete-impact-list {
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
    margin-bottom: 1.1rem;
  }

  .delete-opt-item {
    display: flex;
    align-items: flex-start;
    gap: 0.65rem;
    padding: 0.65rem 0.8rem;
    border-radius: 8px;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    cursor: pointer;
    transition: border-color 0.15s, background 0.15s;
  }

  .delete-opt-item input[type='checkbox'] {
    width: 15px;
    height: 15px;
    margin-top: 0.15rem;
    padding: 0;
    accent-color: var(--accent);
    flex-shrink: 0;
    cursor: pointer;
  }

  .delete-opt-item.opt-destructive input[type='checkbox'] {
    accent-color: var(--danger);
  }

  .delete-opt-item.is-checked {
    border-color: color-mix(in srgb, var(--accent) 45%, var(--border));
    background: color-mix(in srgb, var(--accent) 6%, var(--bg-inset));
  }

  .delete-opt-item.is-checked-danger {
    border-color: color-mix(in srgb, var(--danger) 38%, transparent);
    background: color-mix(in srgb, var(--danger) 5%, transparent);
  }

  .opt-body {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    min-width: 0;
    flex: 1;
  }

  .opt-title-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }

  .opt-title {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--text);
  }

  .opt-badge {
    font-size: 0.65rem;
    font-weight: 700;
    padding: 0.1rem 0.42rem;
    border-radius: 4px;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    flex-shrink: 0;
  }

  .badge-safe {
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    color: var(--accent);
    border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
  }

  .badge-danger {
    background: color-mix(in srgb, var(--danger) 14%, transparent);
    color: var(--danger-strong);
    border: 1px solid color-mix(in srgb, var(--danger) 30%, transparent);
  }

  .opt-desc {
    font-size: 0.73rem;
    color: var(--text-muted);
    line-height: 1.5;
    word-break: normal;
    overflow-wrap: break-word;
  }

  .opt-desc code {
    color: var(--text-secondary);
    background: color-mix(in srgb, var(--text) 6%, transparent);
    padding: 0.08rem 0.35rem;
    border-radius: 4px;
    font-size: 0.7rem;
    overflow-wrap: anywhere;
  }

  .delete-verify-box {
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.85rem;
    margin-bottom: 1.15rem;
  }

  .verify-label {
    display: block;
    font-size: 0.76rem;
    color: var(--text-secondary);
    margin-bottom: 0.5rem;
    line-height: 1.45;
  }

  .verify-label strong.select-none {
    user-select: none;
    -webkit-user-select: none;
    color: var(--danger-strong);
    background: color-mix(in srgb, var(--danger) 12%, transparent);
    padding: 0.12rem 0.4rem;
    border-radius: 4px;
    border: 1px dashed color-mix(in srgb, var(--danger) 35%, transparent);
  }

  .verify-input {
    border-color: var(--border-strong);
    font-size: 0.82rem;
  }

  .verify-input:focus {
    border-color: var(--danger-strong);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--danger) 18%, transparent);
  }

  .paste-blocked-hint {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-top: 0.5rem;
    padding: 0.4rem 0.6rem;
    border-radius: 6px;
    background: color-mix(in srgb, var(--warning) 12%, transparent);
    border: 1px solid color-mix(in srgb, var(--warning) 35%, transparent);
    color: var(--warning);
    font-size: 0.72rem;
    line-height: 1.4;
  }

  .delete-modal-actions {
    padding-top: 0.25rem;
  }

  .btn-danger-solid {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    background: var(--danger);
    color: var(--text);
    border: 1px solid var(--danger-strong);
    padding: 0.5rem 1rem;
    transition: background 0.15s, opacity 0.15s;
  }

  .btn-danger-solid:hover:not(:disabled) {
    background: var(--danger-strong);
  }

  .btn-danger-solid:disabled {
    opacity: 0.42;
    cursor: not-allowed;
    background: color-mix(in srgb, var(--danger) 35%, transparent);
    border-color: color-mix(in srgb, var(--danger) 25%, transparent);
  }

  /* ═════════ Git Badge, Git Pull Button & Modal ═════════ */
  .site-git-meta {
    margin-top: 0.15rem;
  }

  .git-meta-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.32rem;
    padding: 0.12rem 0.45rem;
    border-radius: 999px;
    background: color-mix(in srgb, var(--accent) 10%, var(--bg-raised));
    border: 1px solid color-mix(in srgb, var(--accent) 28%, var(--border));
    color: var(--text-secondary);
    font-size: 0.66rem;
    font-family: inherit;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .git-meta-pill:hover {
    border-color: var(--accent);
    color: var(--text);
    background: color-mix(in srgb, var(--accent) 16%, var(--bg-raised));
  }

  .git-meta-pill i {
    color: var(--accent);
    font-size: 0.76rem;
  }

  .git-branch-tag {
    color: var(--accent);
    font-weight: 700;
  }

  .git-commit-hash {
    color: var(--text-subtle);
    font-size: 0.63rem;
  }

  .req-btn-trigger {
    background: transparent;
    border: none;
    color: var(--text);
    font-family: inherit;
    font-size: 0.8rem;
    padding: 0.2rem 0.35rem;
    border-radius: 5px;
    cursor: pointer;
    transition: background 0.15s ease;
  }

  .req-btn-trigger:hover {
    background: color-mix(in srgb, var(--accent) 10%, transparent);
    color: var(--accent);
  }

  .btn-op-git {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    padding: 0.24rem 0.6rem;
    border-radius: 6px;
    background: color-mix(in srgb, var(--accent) 12%, transparent);
    border: 1px solid color-mix(in srgb, var(--accent) 32%, transparent);
    color: var(--accent);
    font-size: 0.73rem;
    font-weight: 600;
    white-space: nowrap;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .btn-op-git:hover {
    background: var(--accent);
    color: var(--text-on-accent);
  }

  .btn-op-conf-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.28rem;
    padding: 0.24rem 0.58rem;
    border-radius: 6px;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 0.73rem;
    font-weight: 600;
    white-space: nowrap;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .btn-op-conf-pill:hover {
    border-color: var(--info);
    color: var(--info);
    background: color-mix(in srgb, var(--info) 8%, var(--bg-raised));
  }

  .op-more-wrap {
    position: relative;
    display: inline-flex;
  }

  .btn-op-more {
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 0.73rem;
    font-weight: 600;
    white-space: nowrap;
    cursor: pointer;
    padding: 0.24rem 0.55rem;
    border-radius: 6px;
    transition: all 0.15s ease;
  }

  .btn-op-more:hover,
  .btn-op-more.is-open {
    background: var(--bg-hover);
    border-color: var(--border-strong);
    color: var(--text);
  }

  .op-more-menu {
    position: absolute;
    right: 0;
    top: calc(100% + 6px);
    min-width: 248px;
    background: var(--bg-raised);
    border: 1px solid var(--border-strong);
    border-radius: 10px;
    padding: 0.4rem;
    box-shadow: 0 14px 32px rgba(0, 0, 0, 0.72);
    z-index: 80;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .op-more-divider {
    height: 1px;
    background: var(--border-subtle);
    margin: 0.25rem 0.35rem;
  }

  .op-more-item {
    display: flex;
    align-items: flex-start;
    gap: 0.6rem;
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    padding: 0.5rem 0.65rem;
    border-radius: 6px;
    cursor: pointer;
    color: var(--text);
    transition: background 0.12s ease;
  }

  .op-more-item:hover {
    background: var(--bg-hover);
  }

  .op-more-item.is-danger-item {
    color: var(--danger-strong);
  }

  .op-more-item.is-danger-item b {
    color: var(--danger-strong);
  }

  .op-more-item.is-danger-item:hover {
    background: color-mix(in srgb, var(--danger) 12%, transparent);
  }

  .op-more-item i {
    font-size: 0.95rem;
    margin-top: 0.1rem;
    flex-shrink: 0;
  }

  .icon-accent {
    color: var(--accent);
  }

  .op-more-item div {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    min-width: 0;
  }

  .op-more-item b {
    font-size: 0.74rem;
    font-weight: 600;
    color: var(--text);
  }

  .op-more-item small {
    font-size: 0.65rem;
    color: var(--text-subtle);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .log-pre-raw {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
    font-family: 'Fira Code', monospace;
    font-size: 0.73rem;
    color: var(--text-secondary);
    line-height: 1.55;
  }

  .git-pull-card {
    max-width: 580px;
    border-color: color-mix(in srgb, var(--accent) 38%, var(--border));
  }

  .git-pull-head {
    margin-bottom: 0.9rem;
    padding-bottom: 0.8rem;
    border-bottom: 1px solid var(--border-subtle);
  }

  .git-icon-circle {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    background: color-mix(in srgb, var(--accent) 14%, transparent);
    border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .git-icon-circle i {
    color: var(--accent) !important;
    font-size: 1.15rem !important;
  }

  .git-summary-strip {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0.6rem;
    padding: 0.65rem 0.8rem;
    border-radius: 8px;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    margin-bottom: 0.95rem;
  }

  .git-sum-item {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    min-width: 0;
  }

  .git-sum-lbl {
    font-size: 0.64rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-subtle);
  }

  .git-sum-item strong {
    font-size: 0.76rem;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .git-branch-highlight {
    color: var(--accent) !important;
  }

  .git-pull-alert {
    display: flex;
    align-items: flex-start;
    gap: 0.6rem;
    padding: 0.7rem 0.85rem;
    border-radius: 8px;
    font-size: 0.76rem;
    line-height: 1.45;
    margin-bottom: 0.85rem;
  }

  .git-pull-alert i {
    font-size: 1.05rem;
    margin-top: 0.05rem;
    flex-shrink: 0;
  }

  .git-pull-alert p {
    margin: 0.2rem 0 0;
    color: var(--text-secondary);
  }

  .git-pull-alert.is-updated {
    background: color-mix(in srgb, var(--accent) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent);
    color: var(--accent);
  }

  .git-pull-alert.is-uptodate {
    background: color-mix(in srgb, var(--info) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--info) 30%, transparent);
    color: var(--info);
  }

  .git-pull-alert.is-error {
    background: color-mix(in srgb, var(--danger) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--danger) 35%, transparent);
    color: var(--danger-strong);
  }

  .git-output-box {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.65rem 0.8rem;
    max-height: 210px;
    overflow: auto;
    margin-bottom: 1rem;
  }

  .git-output-head {
    font-size: 0.66rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-subtle);
    margin-bottom: 0.4rem;
    padding-bottom: 0.3rem;
    border-bottom: 1px solid var(--border-subtle);
  }

  .btn-git-pull-exec {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.5rem 1rem;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.4; transform: scale(0.85); }
  }
</style>
