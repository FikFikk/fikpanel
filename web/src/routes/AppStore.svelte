<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { AppPackage, InstallTask, GlobalPackage } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  // Store mode
  type StoreMode = 'curated' | 'global';
  let storeMode = $state<StoreMode>('curated');

  // Curated Catalog State
  let apps = $state<AppPackage[]>([]);
  let loading = $state(true);
  let activeCategory = $state<'all' | 'webserver' | 'runtime' | 'database' | 'tools'>('all');

  // Global Linux Package State
  let searchQuery = $state('');
  let searchResults = $state<GlobalPackage[]>([]);
  let searching = $state(false);
  let searchDebounceTimer: any = null;
  let initialSearched = $state(false);

  // Modal Log state
  let showLogModal = $state(false);
  let logModalTitle = $state('');
  let currentTask = $state<InstallTask | null>(null);
  let pollingInterval: any = null;

  async function loadCatalog() {
    try {
      loading = true;
      const res = await apiFetch<{ ok: boolean; apps: AppPackage[] }>('/api/appstore/catalog');
      apps = res?.apps || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat katalog App Store', 'error');
    } finally {
      loading = false;
    }
  }

  async function installApp(app: AppPackage) {
    const ok = await confirmDialog({
      title: `Pasang ${app.name}`,
      message: `Apakah Anda ingin memasang paket software "${app.name}" (${app.version}) ke server? Proses instalasi berjalan secara background.`,
      confirmLabel: 'Mulai Instalasi',
      variant: 'primary',
    });
    if (!ok) return;

    try {
      notify(`Memulai instalasi ${app.name}...`, 'info');
      await apiFetch(`/api/appstore/install/${app.id}`, { method: 'POST' });
      viewLogs(app.id, `Proses Instalasi: ${app.name}`);
    } catch (err: any) {
      notify(err.message || 'Gagal memulai instalasi', 'error');
    }
  }

  async function uninstallApp(app: AppPackage) {
    const ok = await confirmDialog({
      title: `Copot ${app.name}`,
      message: `Apakah Anda yakin ingin mencopot ${app.name} dari sistem? Perhatikan dependensi perangkat lunak sebelum mencopot.`,
      confirmLabel: 'Copot Paket',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      notify(`Memulai pencopotan ${app.name}...`, 'info');
      await apiFetch(`/api/appstore/uninstall/${app.id}`, { method: 'POST' });
      viewLogs(app.id, `Pencopotan: ${app.name}`);
    } catch (err: any) {
      notify(err.message || 'Gagal memulai pencopotan', 'error');
    }
  }

  // Global Package Search
  async function searchGlobalPackages(query: string) {
    const q = query.trim();
    if (!q) {
      searchResults = [];
      searching = false;
      return;
    }

    try {
      searching = true;
      initialSearched = true;
      const res = await apiFetch<{ ok: boolean; packages: GlobalPackage[] }>(`/api/appstore/search?q=${encodeURIComponent(q)}`);
      searchResults = res?.packages || [];
    } catch (err: any) {
      notify(err.message || 'Gagal mencari paket di repositori OS', 'error');
    } finally {
      searching = false;
    }
  }

  function handleSearchInput(e: Event) {
    const val = (e.target as HTMLInputElement).value;
    searchQuery = val;
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
    searchDebounceTimer = setTimeout(() => {
      searchGlobalPackages(val);
    }, 400);
  }

  function quickSearch(pkg: string) {
    searchQuery = pkg;
    searchGlobalPackages(pkg);
  }

  async function installGlobalPackage(pkg: GlobalPackage) {
    const ok = await confirmDialog({
      title: `Pasang Paket ${pkg.name}`,
      message: `Pasang paket "${pkg.name}" dari repositori sistem (APT)?`,
      confirmLabel: 'Pasang Sekarang',
      variant: 'primary',
    });
    if (!ok) return;

    try {
      notify(`Memulai instalasi paket ${pkg.name}...`, 'info');
      await apiFetch(`/api/appstore/install/pkg:${pkg.name}`, { method: 'POST' });
      viewLogs(`pkg:${pkg.name}`, `Instalasi Paket Linux: ${pkg.name}`);
    } catch (err: any) {
      notify(err.message || 'Gagal memulai instalasi', 'error');
    }
  }

  async function uninstallGlobalPackage(pkg: GlobalPackage) {
    const ok = await confirmDialog({
      title: `Copot Paket ${pkg.name}`,
      message: `Hapus paket "${pkg.name}" dari sistem via apt-get remove?`,
      confirmLabel: 'Copot Paket',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      notify(`Memulai pencopotan paket ${pkg.name}...`, 'info');
      await apiFetch(`/api/appstore/uninstall/pkg:${pkg.name}`, { method: 'POST' });
      viewLogs(`pkg:${pkg.name}`, `Pencopotan Paket: ${pkg.name}`);
    } catch (err: any) {
      notify(err.message || 'Gagal memulai pencopotan', 'error');
    }
  }

  function viewLogs(taskID: string, title: string) {
    logModalTitle = title;
    showLogModal = true;
    pollTask(taskID);
  }

  async function pollTask(taskID: string) {
    if (pollingInterval) clearInterval(pollingInterval);
    const fetchStatus = async () => {
      try {
        const res = await apiFetch<{ ok: boolean; task: InstallTask | null }>(`/api/appstore/tasks/${encodeURIComponent(taskID)}`);
        currentTask = res?.task || null;
        if (currentTask && currentTask.status !== 'running') {
          clearInterval(pollingInterval);
          loadCatalog();
          if (searchQuery.trim()) {
            searchGlobalPackages(searchQuery);
          }
        }
      } catch {
        // ignore
      }
    };
    await fetchStatus();
    pollingInterval = setInterval(fetchStatus, 2000);
  }

  function closeLogModal() {
    showLogModal = false;
    if (pollingInterval) clearInterval(pollingInterval);
  }

  let filteredApps = $derived(
    activeCategory === 'all' ? apps : apps.filter((a) => a.category === activeCategory)
  );

  onMount(() => {
    loadCatalog();
  });

  onDestroy(() => {
    if (pollingInterval) clearInterval(pollingInterval);
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
  });
</script>

<div class="appstore-view">
  <!-- Header Bar -->
  <header class="page-head">
    <div class="page-title-wrap">
      <h1 class="page-title">
        <i class="ri-apps-2-line"></i> App Store & Paket Sistem Linux
      </h1>
      <p class="page-sub">
        Instalasi 1-klik untuk web stack unggulan (Nginx, PHP 8.5/8.3, MariaDB, Redis, Node.js) dan akses ke seluruh repositori paket global Ubuntu / Debian.
      </p>
    </div>
    <div class="page-actions">
      <button class="btn btn-ghost" onclick={loadCatalog} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh Status
      </button>
    </div>
  </header>

  <!-- Navigation Tab Mode (Curated Stacks vs Global Linux APT) -->
  <div class="store-mode-tabs">
    <button
      class="mode-tab {storeMode === 'curated' ? 'active' : ''}"
      onclick={() => (storeMode = 'curated')}
    >
      <i class="ri-star-line"></i>
      <span>Katalog Unggulan Stack ({apps.length})</span>
    </button>
    <button
      class="mode-tab {storeMode === 'global' ? 'active' : ''}"
      onclick={() => { storeMode = 'global'; if (!initialSearched) searchGlobalPackages('htop'); }}
    >
      <i class="ri-ubuntu-line"></i>
      <span>Repositori Global Linux (APT Search)</span>
      <span class="badge-pill">Global OS</span>
    </button>
  </div>

  {#if storeMode === 'curated'}
    <!-- Filter Category Pills (Curated) -->
    <div class="cat-nav">
      <button class="cat-pill {activeCategory === 'all' ? 'active' : ''}" onclick={() => (activeCategory = 'all')}>
        Semua Paket ({apps.length})
      </button>
      <button class="cat-pill {activeCategory === 'webserver' ? 'active' : ''}" onclick={() => (activeCategory = 'webserver')}>
        Web Server
      </button>
      <button class="cat-pill {activeCategory === 'runtime' ? 'active' : ''}" onclick={() => (activeCategory = 'runtime')}>
        Runtime (PHP / Node.js)
      </button>
      <button class="cat-pill {activeCategory === 'database' ? 'active' : ''}" onclick={() => (activeCategory = 'database')}>
        Database & Cache
      </button>
      <button class="cat-pill {activeCategory === 'tools' ? 'active' : ''}" onclick={() => (activeCategory = 'tools')}>
        Toolkit & Security
      </button>
    </div>

    <!-- App Cards Grid -->
    <div class="apps-grid">
      {#if loading && apps.length === 0}
        <div class="empty-loading">
          <i class="ri-loader-4-line spin text-xl"></i> Memeriksa status paket sistem...
        </div>
      {:else}
        {#each filteredApps as app (app.id)}
          <div class="app-card">
            <div class="app-header">
              <div class="app-icon-wrap">
                <i class="{app.icon} app-icon"></i>
              </div>
              <div class="app-meta">
                <span class="app-name">{app.name}</span>
                <span class="app-ver">{app.version}</span>
              </div>
            </div>

            <p class="app-desc">{app.description}</p>

            <div class="app-footer">
              <div class="app-status">
                {#if app.status === 'running'}
                  <span class="status-tag running">
                    <span class="dot running"></span> RUNNING
                  </span>
                {:else if app.status === 'stopped'}
                  <span class="status-tag stopped">
                    <span class="dot stopped"></span> TERPASANG
                  </span>
                {:else}
                  <span class="status-tag uninstalled">
                    <span class="dot uninstalled"></span> BELUM ADA
                  </span>
                {/if}
              </div>

              <div class="app-actions">
                {#if app.installed}
                  <button class="btn btn-ghost btn-sm" onclick={() => viewLogs(app.id, `Log Operasi: ${app.name}`)}>
                    <i class="ri-file-text-line"></i> Log
                  </button>
                  <button class="btn btn-outline-installed btn-sm" disabled>
                    <i class="ri-check-line"></i> Terinstal
                  </button>
                {:else}
                  <button class="btn btn-primary btn-sm" onclick={() => installApp(app)}>
                    <i class="ri-download-2-line"></i> Pasang
                  </button>
                {/if}
              </div>
            </div>
          </div>
        {/each}
      {/if}
    </div>

  {:else}
    <!-- GLOBAL REPOSITORY SEARCH TAB -->
    <div class="global-search-container">
      <div class="search-box-card">
        <div class="search-input-wrap">
          <i class="ri-search-line search-icon"></i>
          <input
            type="text"
            value={searchQuery}
            oninput={handleSearchInput}
            placeholder="Ketik nama paket sistem Linux (misal: htop, ffmpeg, imagemagick, zip, supervisor, postgresql, ufw)..."
            class="search-input"
          />
          {#if searching}
            <i class="ri-loader-4-line spin search-spinner"></i>
          {/if}
        </div>

        <!-- Quick Tags -->
        <div class="quick-tags">
          <span class="quick-tags-label">Rekomendasi Cepat:</span>
          {#each ['htop', 'ffmpeg', 'imagemagick', 'zip', 'unzip', 'supervisor', 'curl', 'wget', 'tmux', 'postgresql', 'sqlite3', 'ufw'] as tag}
            <button
              type="button"
              class="tag-btn {searchQuery === tag ? 'active' : ''}"
              onclick={() => quickSearch(tag)}
            >
              {tag}
            </button>
          {/each}
        </div>
      </div>

      <!-- Search Results Table -->
      <div class="panel-card">
        <div class="table-wrap">
          <table class="data-table">
            <thead>
              <tr>
                <th style="width: 200px;">NAMA PAKET</th>
                <th>DESKRIPSI LENGKAP</th>
                <th style="width: 140px;">STATUS</th>
                <th class="text-right" style="width: 140px;">AKSI</th>
              </tr>
            </thead>
            <tbody>
              {#if searching && searchResults.length === 0}
                <tr>
                  <td colspan="4" class="table-empty">
                    <i class="ri-loader-4-line spin text-lg"></i> Mencari paket di repositori OS...
                  </td>
                </tr>
              {:else if searchResults.length === 0}
                <tr>
                  <td colspan="4" class="table-empty">
                    {searchQuery ? `Tidak ditemukan paket dengan nama "${searchQuery}".` : 'Ketik nama paket untuk mencari di seluruh repositori Ubuntu/Debian.'}
                  </td>
                </tr>
              {:else}
                {#each searchResults as pkg (pkg.name)}
                  <tr>
                    <td>
                      <div class="pkg-name-col">
                        <i class="ri-terminal-box-line text-accent"></i>
                        <span class="mono font-bold text-accent">{pkg.name}</span>
                      </div>
                    </td>
                    <td class="text-muted text-xs line-clamp">
                      {pkg.description || 'Paket utilitas sistem Linux.'}
                    </td>
                    <td>
                      {#if pkg.installed}
                        <span class="status-badge success">
                          <i class="ri-checkbox-circle-fill"></i> TERPASANG
                        </span>
                      {:else}
                        <span class="status-badge text-subtle">
                          <i class="ri-checkbox-blank-circle-line"></i> BELUM ADA
                        </span>
                      {/if}
                    </td>
                    <td class="text-right">
                      {#if pkg.installed}
                        <button class="btn btn-ghost danger btn-sm" onclick={() => uninstallGlobalPackage(pkg)}>
                          <i class="ri-delete-bin-line"></i> Copot
                        </button>
                      {:else}
                        <button class="btn btn-primary btn-sm" onclick={() => installGlobalPackage(pkg)}>
                          <i class="ri-download-2-line"></i> Pasang
                        </button>
                      {/if}
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  {/if}
</div>

<!-- Modal Log Output Instalasi / Pencopotan -->
{#if showLogModal}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) closeLogModal(); }}
    onkeydown={(e) => { if (e.key === 'Escape') closeLogModal(); }}
  >
    <div class="modal-card modal-wide">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-terminal-box-line"></i>
          <h3>{logModalTitle || 'Eksekusi Paket APT'}</h3>
        </div>
        <button class="btn-close" onclick={closeLogModal}>&times;</button>
      </div>

      <div class="log-body">
        <div class="status-banner {currentTask?.status || 'running'}">
          <span class="font-bold">STATUS:</span>
          {#if currentTask?.status === 'running'}
            <span class="flex items-center gap-sm"><i class="ri-loader-4-line spin"></i> Sedang memproses paket APT di background...</span>
          {:else if currentTask?.status === 'success'}
            <span class="flex items-center gap-sm text-accent"><i class="ri-checkbox-circle-fill"></i> Operasi selesai dengan sukses!</span>
          {:else if currentTask?.status === 'failed'}
            <span class="flex items-center gap-sm text-danger"><i class="ri-close-circle-fill"></i> Terjadi kesalahan selama eksekusi perintah apt.</span>
          {:else}
            <span>Menunggu status daemon...</span>
          {/if}
        </div>

        <pre class="log-terminal mono">{currentTask?.logs || 'Memulai eksekusi...'}</pre>
      </div>

      <div class="modal-actions">
        <button class="btn btn-ghost" onclick={closeLogModal}>Tutup</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .appstore-view {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .page-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .page-title {
    font-size: 1.4rem;
    font-weight: 700;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.6rem;
    margin: 0;
  }
  .page-sub {
    font-size: 0.85rem;
    color: var(--text-muted);
    margin: 0.25rem 0 0 0;
  }
  .page-actions {
    display: flex;
    gap: 0.5rem;
  }

  /* Store Mode Tabs */
  .store-mode-tabs {
    display: flex;
    gap: 0.5rem;
    border-bottom: 1px solid var(--border);
    padding-bottom: 0.75rem;
  }
  .mode-tab {
    background: transparent;
    border: 1px solid transparent;
    padding: 0.55rem 1.1rem;
    border-radius: 8px;
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--text-muted);
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    transition: all 0.15s;
  }
  .mode-tab:hover {
    color: var(--text);
    background: var(--bg-hover);
  }
  .mode-tab.active {
    background: var(--bg-surface);
    color: var(--accent);
    border-color: var(--border);
    box-shadow: var(--shadow-sm);
  }
  .badge-pill {
    background: var(--tint-accent);
    color: var(--accent);
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    font-size: 0.68rem;
    font-weight: 700;
  }

  .cat-nav {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .cat-pill {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    color: var(--text-muted);
    padding: 0.4rem 0.85rem;
    border-radius: 20px;
    font-size: 0.78rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s;
  }
  .cat-pill:hover {
    color: var(--text);
    border-color: var(--border-hover);
  }
  .cat-pill.active {
    background: var(--primary);
    color: white;
    border-color: var(--primary);
  }
  .apps-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 1.25rem;
  }
  .app-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 1rem;
    transition: transform 0.15s, border-color 0.15s;
  }
  .app-card:hover {
    border-color: var(--border-hover);
    transform: translateY(-2px);
  }
  .app-header {
    display: flex;
    align-items: center;
    gap: 0.85rem;
  }
  .app-icon-wrap {
    width: 44px;
    height: 44px;
    border-radius: 10px;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }
  .app-icon {
    font-size: 1.4rem;
    color: var(--primary);
  }
  .app-meta {
    display: flex;
    flex-direction: column;
  }
  .app-name {
    font-size: 0.92rem;
    font-weight: 700;
    color: var(--text);
  }
  .app-ver {
    font-size: 0.72rem;
    color: var(--text-subtle);
    font-family: var(--font-mono);
  }
  .app-desc {
    font-size: 0.78rem;
    color: var(--text-muted);
    line-height: 1.45;
    margin: 0;
    flex: 1;
  }
  .app-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-top: 0.75rem;
    border-top: 1px solid var(--border-soft);
  }
  .status-tag {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.72rem;
    font-weight: 600;
    font-family: var(--font-mono);
  }
  .status-tag.running { color: var(--accent); }
  .status-tag.stopped { color: var(--text-muted); }
  .status-tag.uninstalled { color: var(--text-subtle); }
  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  .dot.running { background: var(--accent); box-shadow: 0 0 6px var(--ring-accent); }
  .dot.stopped { background: var(--text-muted); }
  .dot.uninstalled { background: var(--text-subtle); }
  .app-actions {
    display: flex;
    gap: 0.4rem;
  }
  .btn-outline-installed {
    background: transparent;
    border: 1px solid var(--border);
    color: var(--accent);
    opacity: 0.85;
    cursor: default;
  }
  .btn-sm {
    padding: 0.35rem 0.75rem;
    font-size: 0.75rem;
  }

  /* Global Search Tab Styles */
  .global-search-container {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .search-box-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .search-input-wrap {
    position: relative;
    display: flex;
    align-items: center;
  }
  .search-icon {
    position: absolute;
    left: 1rem;
    font-size: 1.1rem;
    color: var(--text-subtle);
  }
  .search-input {
    width: 100%;
    padding: 0.75rem 1rem 0.75rem 2.75rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text);
    font-size: 0.88rem;
    outline: none;
    transition: border-color 0.15s;
  }
  .search-input:focus {
    border-color: var(--primary);
  }
  .search-spinner {
    position: absolute;
    right: 1rem;
    font-size: 1.1rem;
    color: var(--primary);
  }
  .quick-tags {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .quick-tags-label {
    font-size: 0.75rem;
    color: var(--text-subtle);
    font-weight: 500;
  }
  .tag-btn {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    color: var(--text-muted);
    padding: 0.2rem 0.55rem;
    border-radius: 6px;
    font-size: 0.72rem;
    font-family: var(--font-mono);
    cursor: pointer;
    transition: all 0.15s;
  }
  .tag-btn:hover {
    color: var(--text);
    border-color: var(--accent);
  }
  .tag-btn.active {
    background: var(--tint-accent);
    border-color: var(--accent);
    color: var(--accent);
    font-weight: bold;
  }

  /* Table styling */
  .panel-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    overflow: hidden;
  }
  .table-wrap {
    overflow-x: auto;
  }
  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
    text-align: left;
  }
  .data-table th {
    background: var(--bg-surface-soft);
    color: var(--text-subtle);
    padding: 0.75rem 1rem;
    font-size: 0.72rem;
    font-weight: 600;
    letter-spacing: 0.05em;
    border-bottom: 1px solid var(--border);
  }
  .data-table td {
    padding: 0.85rem 1rem;
    border-bottom: 1px solid var(--border-soft);
    color: var(--text);
  }
  .data-table tr:hover {
    background: var(--bg-hover);
  }
  .pkg-name-col {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .status-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.72rem;
    font-weight: 600;
  }
  .status-badge.success { color: var(--accent); }
  .text-right { text-align: right; }
  .mono { font-family: var(--font-mono); }
  .font-bold { font-weight: 600; }
  .text-accent { color: var(--accent); }
  .text-muted { color: var(--text-muted); }
  .text-subtle { color: var(--text-subtle); }
  .text-danger { color: var(--danger); }
  .gap-sm { gap: 0.4rem; }
  .empty-loading, .table-empty {
    text-align: center;
    padding: 2.5rem;
    color: var(--text-subtle);
  }
  .spin { animation: spin 1s linear infinite; }
  @keyframes spin { 100% { transform: rotate(360deg); } }

  /* Modal Log */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: var(--bg-overlay);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: var(--z-modal);
    padding: 1rem;
  }
  .modal-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    width: 100%;
    max-width: 760px;
    padding: 1.5rem;
    box-shadow: var(--shadow-modal);
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
    font-size: 1.3rem;
    color: var(--primary);
  }
  .modal-title-wrap h3 {
    margin: 0;
    font-size: 1rem;
    color: var(--text);
  }
  .btn-close {
    background: transparent;
    border: none;
    color: var(--text-subtle);
    font-size: 1.4rem;
    cursor: pointer;
  }
  .log-body {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .status-banner {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.75rem 1rem;
    border-radius: 8px;
    font-size: 0.82rem;
  }
  .status-banner.running {
    background: var(--tint-warning);
    border: 1px solid var(--border-strong);
    color: var(--warning);
  }
  .status-banner.success {
    background: var(--tint-accent);
    border: 1px solid var(--border-strong);
    color: var(--accent);
  }
  .status-banner.failed {
    background: var(--tint-danger);
    border: 1px solid var(--border-strong);
    color: var(--danger);
  }
  .log-terminal {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 1rem;
    border-radius: 8px;
    font-size: 0.75rem;
    color: var(--text-muted);
    max-height: 320px;
    overflow-y: auto;
    white-space: pre-wrap;
    margin: 0;
  }
  .modal-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 1rem;
  }
</style>
