<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { Site, SoftwareItem, SitesData, NginxStatus } from '../lib/types';
  import NginxManager from '../lib/components/NginxManager.svelte';
  import AddSiteModal from '../lib/components/AddSiteModal.svelte';
  import SiteSettings from '../lib/components/SiteSettings.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import '../styles/pages/sites.css';

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
  let gitPullCustomCmd = $state('');
  let gitPullResult = $state<GitPullResponse | null>(null);
  let gitPullError = $state('');
  let openMoreSiteId = $state<number | null>(null);
  let renewingAllSSL = $state(false);

  function toggleMoreMenu(siteId: number, e: MouseEvent) {
    e.stopPropagation();
    openMoreSiteId = openMoreSiteId === siteId ? null : siteId;
  }

  // Daftar snippet perintah dinamis yang dapat ditambahkan/dihapus dengan 1 klik pada modal Git Pull
  const gitPullSnippets = [
    { label: '+ composer install', cmd: 'composer install --no-interaction --optimize-autoloader', phpOnly: true },
    { label: '+ php artisan migrate', cmd: 'php artisan migrate --force', phpOnly: true },
    { label: '+ php artisan optimize:clear', cmd: 'php artisan optimize:clear', phpOnly: true },
    { label: '+ filament:assets', cmd: 'php artisan filament:assets', phpOnly: true },
    { label: '+ npm run build', cmd: 'npm install && npm run build', phpOnly: false },
  ];

  // Memeriksa apakah snippet tertentu sudah ada di dalam rangkaian perintah Git Pull
  function hasGitSnippet(snippetCmd: string): boolean {
    const parts = gitPullCustomCmd.split('&&').map((p) => p.trim());
    return parts.includes(snippetCmd);
  }

  // Menambah atau menghapus snippet perintah dari input dinamis Git Pull
  function toggleGitSnippet(snippetCmd: string) {
    const parts = gitPullCustomCmd
      .split('&&')
      .map((p) => p.trim())
      .filter(Boolean);
    const idx = parts.indexOf(snippetCmd);
    if (idx !== -1) {
      parts.splice(idx, 1);
    } else {
      parts.push(snippetCmd);
    }
    const branch = gitPullTarget?.git_branch || 'main';
    gitPullCustomCmd = parts.length > 0 ? parts.join(' && ') : `git pull origin ${branch}`;
  }

  function resetGitPullCmd() {
    const branch = gitPullTarget?.git_branch || 'main';
    gitPullCustomCmd = `git pull origin ${branch}`;
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

  // Memeriksa dan memperbarui sertifikat SSL untuk seluruh situs terdaftar
  async function renewAllSSL() {
    if (renewingAllSSL) return;
    if (sites.length === 0) {
      onNotify('Belum ada situs yang terdaftar untuk diperiksa SSL-nya');
      return;
    }

    renewingAllSSL = true;
    try {
      // Cari situs yang SSL-nya belum aktif, gagal, atau sisa masa berlakunya <= 30 hari
      const needRenew = sites.filter(
        (s) => s.ssl_status !== 'issued' || s.ssl_mode === 'none' || sslDaysLeft(s) <= 30,
      );

      if (needRenew.length === 0) {
        await loadSites(true);
        const minDays = Math.min(...sites.map((s) => sslDaysLeft(s)));
        onNotify(
          `Semua sertifikat SSL (${sites.length} situs) masih aktif & aman (${minDays} hari tersisa)`,
        );
        return;
      }

      onNotify(`Memperbarui sertifikat SSL untuk ${needRenew.length} situs...`);
      let count = 0;
      for (const site of needRenew) {
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
          // Lanjutkan ke situs berikutnya jika satu situs gagal
        }
      }
      await loadSites(true);
      onNotify(`Pembaruan SSL berhasil dijalankan untuk ${count} dari ${needRenew.length} situs`);
    } finally {
      renewingAllSSL = false;
    }
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

  // Buka modal konfirmasi Git Pull dengan perintah dinamis yang dapat diedit
  function openGitPullModal(site: Site) {
    gitPullTarget = site;
    gitPullRunning = false;
    const branch = site.git_branch || 'main';
    if (site.kind === 'php') {
      gitPullCustomCmd = `git pull origin ${branch} && php artisan optimize:clear`;
    } else if (site.kind === 'node') {
      gitPullCustomCmd = `git pull origin ${branch} && npm install && npm run build`;
    } else {
      gitPullCustomCmd = `git pull origin ${branch}`;
    }
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
          command: gitPullCustomCmd.trim(),
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

<div class="pg-sites sites-container">
  <!-- ═════════ 1. Header Tab Kategori Proyek + Status Mesin Nginx di Kanan ═════════ -->
  <div class="project-tabs-bar">
    <div class="project-tabs-list">
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

    <!-- Badge Engine Web Server di Ujung Kanan Bar Kategori -->
    <div class="project-tabs-right">
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
  </div>

  <!-- ═════════ 2. Action Toolbar: Tombol Tambah, Perbarui SSL Semua, & Search ═════════ -->
  <div class="action-toolbar">
    <div class="tb-left">
      <button class="btn btn-add-site" id="add-site-btn" onclick={() => (showAddModal = true)}>
        <i class="ri-add-line"></i>
        <span>Tambah Situs</span>
      </button>

      <button
        class="btn btn-renew-ssl"
        title="Periksa masa aktif & perbarui semua sertifikat SSL"
        disabled={renewingAllSSL}
        onclick={renewAllSSL}
      >
        <i class={renewingAllSSL ? 'ri-loader-4-line spin' : 'ri-shield-keyhole-line'}></i>
        <span>{renewingAllSSL ? 'Memeriksa SSL...' : 'Perbarui SSL Semua'}</span>
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
              class="btn btn--ghost"
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
    <!-- TABEL SITUS WEB -->
    <div class="table-container">
      {#if loading}
        <div class="state state--loading">
          <i class="ri-loader-4-line spin state__icon"></i>
          <span class="state__title">Memuat daftar situs web...</span>
        </div>
      {:else if filteredSites.length === 0}
        <div class="state state--empty">
          <i class="ri-global-line state__icon"></i>
          <p class="state__desc">Tidak ada situs yang cocok dengan kriteria pencarian.</p>
          <button class="btn btn--primary" onclick={() => (showAddModal = true)}>
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

                <!-- Kolom Permintaan / Request Count -->
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

                <!-- Kolom Operasi -->
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
            <span class="badge">Halaman 1 dari 1</span>
          </div>
        </div>
      {/if}
    </div>
  {/if}

  <!-- ═════════ MODAL: Tambah Situs ═════════ -->
  {#if showAddModal}
    <AddSiteModal
      onClose={() => (showAddModal = false)}
      onCreated={() => loadSites(true)}
      onOpenSSL={(s) => { showAddModal = false; openSettings(s, 'ssl'); }}
      {onNotify}
    />
  {/if}

  <!-- ═════════ MODAL: Pengaturan Situs ═════════ -->
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
  <Modal
    open={showLogModal && !!logTarget}
    title={logTarget ? `Log Web Server: ${logTarget.domain}` : 'Log Web Server'}
    size="xl"
    onclose={() => (showLogModal = false)}
  >
    {#if logTarget}
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
    {/if}

    {#snippet footer()}
      <button type="button" class="btn btn--ghost" onclick={() => (showLogModal = false)}>
        Tutup
      </button>
    {/snippet}
  </Modal>

  <!-- ═════════ MODAL: Git Pull & Auto-Deploy ═════════ -->
  <Modal
    open={!!gitPullTarget}
    title={gitPullTarget ? `Git Pull & Sinkronisasi: ${gitPullTarget.domain}` : 'Git Pull'}
    size="lg"
    onclose={closeGitPullModal}
  >
    {#if gitPullTarget}
      <span class="danger-sub-caption">Tarik commit terbaru dari repositori remote tanpa menimpa file .env lokal</span>

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

      <!-- Konfirmasi & Editor Perintah Dinamis Git Pull -->
      <div class="git-cmd-section">
        <div class="git-cmd-label-row">
          <span class="git-cmd-label">
            <i class="ri-terminal-box-line"></i>
            Perintah Eksekusi (Dapat Diedit Dinamis)
          </span>
          <button
            type="button"
            class="git-chip-reset"
            onclick={resetGitPullCmd}
            disabled={gitPullRunning}
            title="Kembalikan ke perintah git pull murni"
          >
            Reset ke git pull
          </button>
        </div>

        <div class="git-cmd-input-wrap">
          <span class="git-cmd-prompt mono">$</span>
          <input
            type="text"
            class="git-cmd-input mono"
            bind:value={gitPullCustomCmd}
            disabled={gitPullRunning}
            placeholder="git pull origin {gitPullTarget.git_branch || 'main'} && php artisan optimize:clear"
          />
        </div>

        <!-- Tombol Chip Tambah/Hapus Perintah Cepat -->
        <div class="git-chips-bar">
          <span class="git-chips-hint">Klik untuk tambah/hapus:</span>
          {#each gitPullSnippets.filter((sn) => !sn.phpOnly || gitPullTarget?.kind === 'php') as sn}
            <button
              type="button"
              class="git-snippet-chip mono {hasGitSnippet(sn.cmd) ? 'is-active' : ''}"
              onclick={() => toggleGitSnippet(sn.cmd)}
              disabled={gitPullRunning}
              title={sn.cmd}
            >
              <i class={hasGitSnippet(sn.cmd) ? 'ri-check-line' : 'ri-add-line'}></i>
              <span>{sn.label.replace(/^\+\s*/, '')}</span>
            </button>
          {/each}
        </div>

        <div class="git-confirm-note">
          <i class="ri-shield-check-line"></i>
          <span>
            Dijalankan di <code class="mono">{getProjectRootDir(gitPullTarget)}</code>. File <code class="mono">.env</code> lokal dicadangkan otomatis &amp; izin <code class="mono">www-data</code> tetap terjaga.
          </span>
        </div>
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
    {/if}

    {#snippet footer()}
      <button type="button" class="btn btn--ghost" onclick={closeGitPullModal} disabled={gitPullRunning}>
        Tutup
      </button>
      <button
        type="button"
        class="btn btn--primary"
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
    {/snippet}
  </Modal>

  <!-- ═════════ MODAL: Konfirmasi Hapus Situs (Danger Zone) ═════════ -->
  <Modal
    open={!!deleteTarget}
    title={deleteTarget ? `Hapus Situs "${deleteTarget.domain}"?` : 'Hapus Situs'}
    size="lg"
    onclose={closeDeleteModal}
  >
    {#if deleteTarget}
      <span class="danger-sub-caption">Tindakan destruktif &amp; permanen pada server produksi</span>

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
            } else if (e.key === 'Enter' && deleteConfirmInput === deleteTarget?.domain) {
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
    {/if}

    {#snippet footer()}
      <button type="button" class="btn btn--ghost" onclick={closeDeleteModal} disabled={deletingSite}>
        Batal
      </button>
      <button
        type="button"
        class="btn btn--danger"
        disabled={!deleteTarget || deleteConfirmInput !== deleteTarget.domain || deletingSite}
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
    {/snippet}
  </Modal>

  <!-- ═════════ MODAL: Nginx Manager ═════════ -->
  {#if showNginxManager}
    <NginxManager
      onClose={() => (showNginxManager = false)}
      {onNotify}
      onStatusChange={onNginxStatus}
    />
  {/if}
</div>
