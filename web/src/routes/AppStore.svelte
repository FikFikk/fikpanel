<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { AppPackage, InstallTask } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  let apps = $state<AppPackage[]>([]);
  let loading = $state(true);
  let activeCategory = $state<'all' | 'webserver' | 'runtime' | 'database' | 'tools'>('all');

  // Modal Log state
  let showLogModal = $state(false);
  let selectedApp = $state<AppPackage | null>(null);
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
      viewLogs(app);
    } catch (err: any) {
      notify(err.message || 'Gagal memulai instalasi', 'error');
    }
  }

  async function viewLogs(app: AppPackage) {
    selectedApp = app;
    showLogModal = true;
    pollTask(app.id);
  }

  async function pollTask(appID: string) {
    if (pollingInterval) clearInterval(pollingInterval);
    const fetchStatus = async () => {
      try {
        const res = await apiFetch<{ ok: boolean; task: InstallTask | null }>(`/api/appstore/tasks/${appID}`);
        currentTask = res?.task || null;
        if (currentTask && currentTask.status !== 'running') {
          clearInterval(pollingInterval);
          loadCatalog();
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
    return () => {
      if (pollingInterval) clearInterval(pollingInterval);
    };
  });
</script>

<div class="appstore-view">
  <!-- Header Bar -->
  <header class="page-head">
    <div class="page-title-wrap">
      <h1 class="page-title">
        <i class="ri-apps-2-line"></i> Toolkit & App Store Software Server
      </h1>
      <p class="page-sub">
        Pusat instalasi 1-klik untuk web server (Nginx), PHP multi-version (8.2/8.3), MySQL/MariaDB, Redis, Node.js, dan utility server.
      </p>
    </div>
    <div class="page-actions">
      <button class="btn btn-ghost" onclick={loadCatalog} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh Status
      </button>
    </div>
  </header>

  <!-- Filter Category Pills -->
  <div class="cat-nav">
    <button class="cat-pill {activeCategory === 'all' ? 'active' : ''}" onclick={() => (activeCategory = 'all')}>
      Semua Paket ({apps.length})
    </button>
    <button class="cat-pill {activeCategory === 'webserver' ? 'active' : ''}" onclick={() => (activeCategory = 'webserver')}>
      Web Server
    </button>
    <button class="cat-pill {activeCategory === 'runtime' ? 'active' : ''}" onclick={() => (activeCategory = 'runtime')}>
      Runtime / Bahasa (PHP/Node)
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
                <button class="btn btn-ghost btn-sm" onclick={() => viewLogs(app)}>
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
</div>

<!-- Modal Log Output Instalasi -->
{#if showLogModal && selectedApp}
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
          <h3>Proses Instalasi: {selectedApp.name}</h3>
        </div>
        <button class="btn-close" onclick={closeLogModal}>&times;</button>
      </div>

      <div class="log-body">
        <div class="status-banner {currentTask?.status || 'running'}">
          <span class="font-bold">STATUS:</span>
          {#if currentTask?.status === 'running'}
            <span class="flex items-center gap-sm"><i class="ri-loader-4-line spin"></i> Sedang mengunduh dan memasang paket APT...</span>
          {:else if currentTask?.status === 'success'}
            <span class="flex items-center gap-sm text-accent"><i class="ri-checkbox-circle-fill"></i> Paket berhasil terpasang dan siap digunakan!</span>
          {:else if currentTask?.status === 'failed'}
            <span class="flex items-center gap-sm text-danger"><i class="ri-close-circle-fill"></i> Terjadi kesalahan selama eksekusi apt.</span>
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
  .cat-nav {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
    border-bottom: 1px solid var(--border);
    padding-bottom: 0.75rem;
  }
  .cat-pill {
    padding: 0.45rem 0.85rem;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: var(--radius-full);
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.15s;
  }
  .cat-pill:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .cat-pill.active {
    background: var(--accent);
    color: var(--text-inverse);
    border-color: var(--accent);
  }
  .apps-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
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
    border-radius: var(--radius-lg);
    background: var(--bg-surface-soft);
    border: 1px solid var(--border-soft);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .app-icon {
    font-size: 1.4rem;
    color: var(--accent);
  }
  .app-meta {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .app-name {
    font-size: 0.95rem;
    font-weight: 700;
    color: var(--text);
  }
  .app-ver {
    font-size: 0.72rem;
    color: var(--text-subtle);
    font-family: var(--font-mono);
  }
  .app-desc {
    font-size: 0.8rem;
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
    font-size: 0.68rem;
    font-weight: 700;
  }
  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  .dot.running { background: var(--accent); box-shadow: 0 0 6px var(--ring-accent); }
  .dot.stopped { background: var(--warning); }
  .dot.uninstalled { background: var(--text-subtle); }
  .status-tag.running { color: var(--accent); }
  .status-tag.stopped { color: var(--warning); }
  .status-tag.uninstalled { color: var(--text-subtle); }

  .app-actions {
    display: flex;
    gap: 0.4rem;
  }
  .btn-sm {
    padding: 0.35rem 0.75rem;
    font-size: 0.75rem;
    border-radius: var(--radius-md);
  }
  .btn-outline-installed {
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-subtle);
    cursor: default;
  }
  .empty-loading {
    grid-column: 1 / -1;
    text-align: center;
    padding: 3rem;
    color: var(--text-subtle);
  }

  /* Modal */
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
    max-width: 520px;
    padding: 1.5rem;
    box-shadow: var(--shadow-modal);
  }
  .modal-card.modal-wide {
    max-width: 760px;
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
    justify-content: space-between;
    align-items: center;
    padding: 0.65rem 0.85rem;
    background: var(--bg-surface-soft);
    border: 1px solid var(--border-soft);
    border-radius: var(--radius-md);
    font-size: 0.78rem;
  }
  .log-terminal {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 0.85rem;
    color: var(--text-muted);
    font-size: 0.75rem;
    white-space: pre-wrap;
    max-height: 320px;
    overflow-y: auto;
    margin: 0;
  }
  .modal-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 1.25rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border-soft);
  }
  .flex { display: flex; }
  .items-center { align-items: center; }
  .gap-sm { gap: 0.375rem; }
  .mono { font-family: var(--font-mono); }
  .font-bold { font-weight: 700; }
  .text-accent { color: var(--accent); }
  .text-danger { color: var(--danger); }
  .spin { animation: spin 1s linear infinite; }
  @keyframes spin { 100% { transform: rotate(360deg); } }
</style>
