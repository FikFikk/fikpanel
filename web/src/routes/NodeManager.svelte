<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { PM2Process, NodeSystemStatus } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  let status = $state<NodeSystemStatus | null>(null);
  let processes = $state<PM2Process[]>([]);
  let loading = $state(true);

  // State Modal Tambah Aplikasi
  let showAddModal = $state(false);
  let formName = $state('');
  let formScript = $state('index.js');
  let formCwd = $state('');
  let formInstances = $state('1');
  let formWatch = $state(false);
  let formArgs = $state('');
  let submittingApp = $state(false);

  // State Modal Log Output
  let showLogModal = $state(false);
  let selectedProcess = $state<PM2Process | null>(null);
  let logContent = $state('');
  let loadingLogs = $state(false);

  async function loadData() {
    try {
      loading = true;
      const res = await apiFetch<{ ok: boolean; status: NodeSystemStatus; processes: PM2Process[] }>('/api/node/processes');
      if (res) {
        status = res.status || null;
        processes = res.processes || [];
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat status proses Node.js / PM2', 'error');
    } finally {
      loading = false;
    }
  }

  async function handleAction(p: PM2Process, action: 'restart' | 'stop' | 'start' | 'reload' | 'delete') {
    const actionLabels: Record<string, string> = {
      restart: 'Restart',
      stop: 'Hentikan',
      start: 'Jalankan',
      reload: 'Reload',
      delete: 'Hapus',
    };
    const actionLabel = actionLabels[action] || action;

    const ok = await confirmDialog({
      title: `${actionLabel} Proses PM2`,
      message: action === 'delete'
        ? `Apakah Anda yakin ingin menghapus proses "${p.name}" dari PM2? Proses akan dihentikan dan dihapus dari startup.`
        : `Apakah Anda yakin ingin mengeksekusi ${actionLabel.toLowerCase()} pada proses "${p.name}"?`,
      confirmLabel: actionLabel,
      variant: action === 'delete' || action === 'stop' ? 'danger' : 'primary',
    });
    if (!ok) return;

    try {
      notify(`Memproses ${action} "${p.name}"...`, 'info');
      await apiFetch(`/api/node/processes/${p.id}/${action}`, { method: 'POST' });
      notify(`Proses "${p.name}" berhasil di-${action}`, 'success');
      await loadData();
    } catch (err: any) {
      notify(err.message || `Gagal ${action} proses`, 'error');
    }
  }

  async function handleStartApp() {
    if (!formName.trim() || !formScript.trim()) {
      notify('Nama aplikasi dan berkas script wajib diisi', 'warning');
      return;
    }

    try {
      submittingApp = true;
      await apiFetch('/api/node/processes/start', {
        method: 'POST',
        body: JSON.stringify({
          name: formName.trim(),
          script: formScript.trim(),
          cwd: formCwd.trim(),
          instances: formInstances,
          watch: formWatch,
          args: formArgs.trim(),
        }),
      });

      notify(`Aplikasi "${formName}" berhasil dijalankan dengan PM2`, 'success');
      showAddModal = false;
      formName = '';
      formScript = 'index.js';
      formCwd = '';
      formInstances = '1';
      formWatch = false;
      formArgs = '';
      await loadData();
    } catch (err: any) {
      notify(err.message || 'Gagal menjalankan aplikasi PM2', 'error');
    } finally {
      submittingApp = false;
    }
  }

  async function openLogs(p: PM2Process) {
    selectedProcess = p;
    showLogModal = true;
    await fetchLogs(p.id);
  }

  async function fetchLogs(id: number) {
    try {
      loadingLogs = true;
      const res = await apiFetch<{ ok: boolean; logs: string }>(`/api/node/processes/${id}/logs?lines=80`);
      logContent = res.logs || 'Belum ada catatan log.';
    } catch (err: any) {
      logContent = `Gagal mengambil log: ${err.message}`;
    } finally {
      loadingLogs = false;
    }
  }

  function formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function formatUptime(uptimeMs: number): string {
    if (!uptimeMs || uptimeMs <= 0) return '-';
    const sec = Math.floor((Date.now() - uptimeMs) / 1000);
    if (sec < 60) return `${sec}d`;
    const min = Math.floor(sec / 60);
    if (min < 60) return `${min}m`;
    const hrs = Math.floor(min / 60);
    if (hrs < 24) return `${hrs}j ${min % 60}m`;
    const days = Math.floor(hrs / 24);
    return `${days}h ${hrs % 24}j`;
  }

  onMount(() => {
    loadData();
  });
</script>

<div class="node-view">
  <!-- Header Bar -->
  <header class="page-head">
    <div class="page-title-wrap">
      <h1 class="page-title">
        <i class="ri-nodejs-line"></i> Node.js & PM2 Process Manager
      </h1>
      <p class="page-sub">
        Pantau penggunaan CPU, konsumsi RAM, status uptime, dan kontrol siklus hidup aplikasi SSR / API Node.js secara nyata.
      </p>
    </div>
    <div class="page-actions">
      <button class="btn btn-ghost" onclick={loadData} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh
      </button>
      {#if status?.pm2_installed}
        <button class="btn btn-primary" onclick={() => (showAddModal = true)}>
          <i class="ri-add-line"></i> Tambah Aplikasi
        </button>
      {/if}
    </div>
  </header>

  <!-- Status Bar Lingkungan Node & PM2 -->
  <div class="env-status-grid">
    <div class="status-box">
      <div class="status-box-icon {status?.node_installed ? 'success' : 'muted'}">
        <i class="ri-terminal-line"></i>
      </div>
      <div class="status-box-info">
        <span class="status-box-label">Runtime Node.js</span>
        <span class="status-box-val">
          {#if status?.node_installed}
            <span class="dot-online"></span> {status.node_version}
          {:else}
            <span class="text-subtle">Belum Terdeteksi</span>
          {/if}
        </span>
      </div>
    </div>

    <div class="status-box">
      <div class="status-box-icon {status?.pm2_installed ? 'success' : 'danger'}">
        <i class="ri-cpu-line"></i>
      </div>
      <div class="status-box-info">
        <span class="status-box-label">PM2 Process Daemon</span>
        <span class="status-box-val">
          {#if status?.pm2_installed}
            <span class="dot-online"></span> Versi {status.pm2_version}
          {:else}
            <span class="text-danger">Belum Terpasang</span>
          {/if}
        </span>
      </div>
    </div>

    <div class="status-box">
      <div class="status-box-icon info">
        <i class="ri-apps-line"></i>
      </div>
      <div class="status-box-info">
        <span class="status-box-label">Aplikasi Aktif</span>
        <span class="status-box-val font-mono">
          {processes.filter((p) => p.status === 'online').length} / {processes.length} Online
        </span>
      </div>
    </div>
  </div>

  <!-- Peringatan jika PM2 Belum Terpasang -->
  {#if !loading && status && !status.pm2_installed}
    <div class="warning-banner">
      <div class="warning-icon">
        <i class="ri-alert-line"></i>
      </div>
      <div class="warning-body">
        <h3 class="warning-title">PM2 Process Manager Belum Terpasang</h3>
        <p class="warning-text">
          PM2 diperlukan untuk menjaga aplikasi Node.js tetap berjalan di latar belakang (daemon) dan melakukan restart otomatis saat aplikasi mengalami crash atau server reboot.
        </p>
        <div class="code-copy-box">
          <code>npm install -g pm2</code>
        </div>
      </div>
    </div>
  {/if}

  <!-- Tabel Proses PM2 Nyata -->
  <div class="panel-card">
    <div class="table-wrap">
      <table class="data-table">
        <thead>
          <tr>
            <th>ID / NAMA APLIKASI</th>
            <th>STATUS</th>
            <th>PID</th>
            <th>CPU</th>
            <th>MEMORI (RAM)</th>
            <th>UPTIME</th>
            <th>RESTART</th>
            <th>MODE</th>
            <th class="text-right">KONTROL</th>
          </tr>
        </thead>
        <tbody>
          {#if loading && processes.length === 0}
            <tr>
              <td colspan="9" class="table-empty">
                <i class="ri-loader-4-line spin text-lg"></i> Memeriksa proses PM2 sistem...
              </td>
            </tr>
          {:else if processes.length === 0}
            <tr>
              <td colspan="9" class="table-empty">
                <div class="empty-state-wrap">
                  <i class="ri-nodejs-line empty-icon"></i>
                  <p class="empty-title">Tidak ada proses PM2 aktif</p>
                  <p class="empty-sub">
                    {#if status?.pm2_installed}
                      Klik tombol <strong>"Tambah Aplikasi"</strong> di atas untuk menjalankan aplikasi Node.js baru.
                    {:else}
                      Pasang PM2 di server untuk mulai mengelola proses Node.js.
                    {/if}
                  </p>
                </div>
              </td>
            </tr>
          {:else}
            {#each processes as p (p.id)}
              <tr>
                <td>
                  <div class="proc-name-col">
                    <span class="badge-id">#{p.id}</span>
                    <div class="proc-title-wrap">
                      <span class="font-bold text-white">{p.name}</span>
                      {#if p.cwd || p.script}
                        <span class="text-subtle text-xs mono truncate" title="{p.cwd || ''}/{p.script || ''}">
                          {p.script || p.cwd}
                        </span>
                      {/if}
                    </div>
                  </div>
                </td>
                <td>
                  <span class="status-badge {p.status}">
                    <span class="status-dot {p.status}"></span>
                    {p.status.toUpperCase()}
                  </span>
                </td>
                <td class="mono text-muted">{p.pid || '-'}</td>
                <td class="mono font-semibold">{p.cpu || 0}%</td>
                <td class="mono font-semibold">{formatBytes(p.memory)}</td>
                <td class="mono text-muted">{formatUptime(p.uptime || 0)}</td>
                <td class="text-muted">{p.restart_count || 0}x</td>
                <td>
                  <span class="badge-mode">{p.mode || 'fork'}</span>
                </td>
                <td class="text-right actions-col">
                  {#if p.status === 'online'}
                    <button class="btn-icon" title="Restart Aplikasi" onclick={() => handleAction(p, 'restart')}>
                      <i class="ri-restart-line"></i>
                    </button>
                    <button class="btn-icon danger" title="Hentikan Aplikasi" onclick={() => handleAction(p, 'stop')}>
                      <i class="ri-stop-circle-line"></i>
                    </button>
                  {:else}
                    <button class="btn-icon success" title="Jalankan Aplikasi" onclick={() => handleAction(p, 'start')}>
                      <i class="ri-play-circle-line"></i>
                    </button>
                  {/if}
                  <button class="btn-icon" title="Lihat Log Real-time" onclick={() => openLogs(p)}>
                    <i class="ri-file-list-3-line"></i>
                  </button>
                  <button class="btn-icon danger" title="Hapus dari PM2" onclick={() => handleAction(p, 'delete')}>
                    <i class="ri-delete-bin-line"></i>
                  </button>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</div>

<!-- Modal Tambah Aplikasi PM2 -->
{#if showAddModal}
  <div class="modal-backdrop" onclick={() => (showAddModal = false)} role="presentation">
    <div class="modal-card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
      <div class="modal-head">
        <h2 class="modal-title"><i class="ri-add-circle-line"></i> Tambah Aplikasi Node.js / PM2</h2>
        <button class="modal-close" onclick={() => (showAddModal = false)}>
          <i class="ri-close-line"></i>
        </button>
      </div>

      <div class="modal-body">
        <div class="form-group">
          <label class="form-label" for="appName">Nama Aplikasi <span class="required">*</span></label>
          <input
            id="appName"
            class="form-input"
            type="text"
            placeholder="misal: api-service, web-frontend"
            bind:value={formName}
          />
        </div>

        <div class="form-group">
          <label class="form-label" for="appScript">Berkas Entrypoint / Script <span class="required">*</span></label>
          <input
            id="appScript"
            class="form-input mono"
            type="text"
            placeholder="misal: server.js, index.js, dist/main.js, atau npm -- start"
            bind:value={formScript}
          />
          <span class="form-hint">Berkas JavaScript/TypeScript utama atau perintah script npm.</span>
        </div>

        <div class="form-group">
          <label class="form-label" for="appCwd">Working Directory (Folder Kerja)</label>
          <input
            id="appCwd"
            class="form-input mono"
            type="text"
            placeholder="misal: /var/www/my-api"
            bind:value={formCwd}
          />
        </div>

        <div class="form-row-2">
          <div class="form-group">
            <label class="form-label" for="appInstances">Jumlah Instance</label>
            <select id="appInstances" class="form-select" bind:value={formInstances}>
              <option value="1">1 (Single Fork Mode)</option>
              <option value="2">2 Instance (Cluster)</option>
              <option value="4">4 Instance (Cluster)</option>
              <option value="max">Max (Semua Core CPU)</option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label" for="appArgs">Argumen Tambahan</label>
            <input
              id="appArgs"
              class="form-input mono"
              type="text"
              placeholder="--port 3000 --env prod"
              bind:value={formArgs}
            />
          </div>
        </div>

        <div class="form-checkbox-wrap">
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={formWatch} />
            <span>Aktifkan Watch & Reload (Otomatis restart saat berkas berubah)</span>
          </label>
        </div>
      </div>

      <div class="modal-foot">
        <button class="btn btn-ghost" onclick={() => (showAddModal = false)}>Batal</button>
        <button class="btn btn-primary" onclick={handleStartApp} disabled={submittingApp}>
          {#if submittingApp}
            <i class="ri-loader-4-line spin"></i> Memulai...
          {:else}
            <i class="ri-play-line"></i> Jalankan Aplikasi
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal Log Output PM2 -->
{#if showLogModal}
  <div class="modal-backdrop" onclick={() => (showLogModal = false)} role="presentation">
    <div class="modal-card modal-lg" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
      <div class="modal-head">
        <h2 class="modal-title">
          <i class="ri-terminal-box-line"></i> Log Real-time: {selectedProcess?.name} (#{selectedProcess?.id})
        </h2>
        <div class="modal-head-tools">
          <button class="btn btn-ghost btn-sm" onclick={() => selectedProcess && fetchLogs(selectedProcess.id)} disabled={loadingLogs}>
            <i class="ri-refresh-line {loadingLogs ? 'spin' : ''}"></i> Refresh Log
          </button>
          <button class="modal-close" onclick={() => (showLogModal = false)}>
            <i class="ri-close-line"></i>
          </button>
        </div>
      </div>

      <div class="modal-body p-0">
        <div class="log-terminal">
          {#if loadingLogs}
            <div class="log-loading">
              <i class="ri-loader-4-line spin"></i> Mengambil catatan log PM2...
            </div>
          {:else}
            <pre class="log-pre">{logContent}</pre>
          {/if}
        </div>
      </div>

      <div class="modal-foot">
        <span class="text-subtle text-xs">Menampilkan 80 baris log stdout dan stderr gabungan terbaru.</span>
        <button class="btn btn-ghost" onclick={() => (showLogModal = false)}>Tutup</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .node-view {
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

  /* Environment status bar */
  .env-status-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 1rem;
  }
  .status-box {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 1rem;
    display: flex;
    align-items: center;
    gap: 0.85rem;
  }
  .status-box-icon {
    width: 2.5rem;
    height: 2.5rem;
    border-radius: var(--radius-md);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.3rem;
  }
  .status-box-icon.success {
    background: var(--tint-accent);
    color: var(--accent);
  }
  .status-box-icon.danger {
    background: var(--tint-danger);
    color: var(--danger);
  }
  .status-box-icon.info {
    background: var(--tint-accent);
    color: var(--accent);
  }
  .status-box-icon.muted {
    background: var(--bg-inset);
    color: var(--text-subtle);
  }
  .status-box-info {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .status-box-label {
    font-size: 0.72rem;
    color: var(--text-subtle);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .status-box-val {
    font-size: 0.92rem;
    font-weight: 600;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .dot-online {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 8px var(--ring-accent);
  }

  /* Warning banner */
  .warning-banner {
    background: var(--tint-danger);
    border: 1px solid var(--border-danger);
    border-radius: var(--radius-lg);
    padding: 1.25rem;
    display: flex;
    gap: 1rem;
  }
  .warning-icon {
    font-size: 1.8rem;
    color: var(--danger);
    line-height: 1;
  }
  .warning-body {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .warning-title {
    margin: 0;
    font-size: 0.98rem;
    font-weight: 700;
    color: var(--text);
  }
  .warning-text {
    margin: 0;
    font-size: 0.85rem;
    color: var(--text-muted);
    line-height: 1.4;
  }
  .code-copy-box {
    margin-top: 0.4rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.5rem 0.85rem;
    border-radius: var(--radius-sm);
    width: fit-content;
    font-family: var(--font-mono);
    font-size: 0.85rem;
    color: var(--accent);
  }

  /* Table styling */
  .panel-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
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
  .proc-name-col {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .proc-title-wrap {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
  }
  .badge-id {
    font-size: 0.7rem;
    font-family: var(--font-mono);
    padding: 0.15rem 0.4rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text-subtle);
  }
  .badge-mode {
    font-size: 0.7rem;
    font-family: var(--font-mono);
    padding: 0.15rem 0.45rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text-subtle);
  }
  .status-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.72rem;
    font-weight: 700;
  }
  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  .status-dot.online {
    background: var(--accent);
    box-shadow: 0 0 8px var(--ring-accent);
  }
  .status-dot.stopped {
    background: var(--text-subtle);
  }
  .status-dot.errored {
    background: var(--danger);
  }
  .status-badge.online { color: var(--accent); }
  .status-badge.stopped { color: var(--text-subtle); }
  .status-badge.errored { color: var(--danger); }

  .btn-icon {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0.35rem;
    border-radius: var(--radius-sm);
    font-size: 1.1rem;
    transition: background 0.15s, color 0.15s;
  }
  .btn-icon:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .btn-icon.success:hover {
    background: var(--tint-accent);
    color: var(--accent);
  }
  .btn-icon.danger:hover {
    background: var(--tint-danger);
    color: var(--danger);
  }

  .empty-state-wrap {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.4rem;
    padding: 2.5rem 1rem;
  }
  .empty-icon {
    font-size: 2.8rem;
    color: var(--text-subtle);
    opacity: 0.6;
  }
  .empty-title {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
    color: var(--text);
  }
  .empty-sub {
    margin: 0;
    font-size: 0.85rem;
    color: var(--text-muted);
    max-width: 420px;
    line-height: 1.4;
  }

  /* Modals */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.7);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 1rem;
  }
  .modal-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    width: 100%;
    max-width: 520px;
    box-shadow: var(--shadow-lg);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .modal-card.modal-lg {
    max-width: 800px;
  }
  .modal-head {
    padding: 1.25rem 1.5rem;
    border-bottom: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .modal-title {
    margin: 0;
    font-size: 1.1rem;
    font-weight: 700;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .modal-head-tools {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .modal-close {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 1.3rem;
    cursor: pointer;
    border-radius: var(--radius-sm);
    padding: 0.2rem;
  }
  .modal-close:hover {
    color: var(--text);
  }
  .modal-body {
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    max-height: 70vh;
    overflow-y: auto;
  }
  .modal-body.p-0 {
    padding: 0;
  }
  .modal-foot {
    padding: 1rem 1.5rem;
    border-top: 1px solid var(--border);
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 0.75rem;
    background: var(--bg-surface-soft);
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .form-row-2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.85rem;
  }
  .form-label {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--text);
  }
  .required {
    color: var(--danger);
  }
  .form-hint {
    font-size: 0.72rem;
    color: var(--text-subtle);
  }
  .form-input, .form-select {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 0.6rem 0.85rem;
    font-size: 0.85rem;
    color: var(--text);
    outline: none;
    transition: border-color 0.15s;
  }
  .form-input:focus, .form-select:focus {
    border-color: var(--accent);
  }
  .form-checkbox-wrap {
    margin-top: 0.25rem;
  }
  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.82rem;
    color: var(--text);
    cursor: pointer;
  }

  /* Terminal log styling */
  .log-terminal {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    min-height: 380px;
    max-height: 480px;
    overflow: auto;
    padding: 1rem 1.25rem;
  }
  .log-pre {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 0.8rem;
    line-height: 1.5;
    color: var(--text);
    white-space: pre-wrap;
    word-break: break-all;
  }
  .log-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 380px;
    gap: 0.6rem;
    color: var(--text-subtle);
    font-size: 0.9rem;
  }

  .text-right { text-align: right; }
  .mono { font-family: var(--font-mono); }
  .text-muted { color: var(--text-muted); }
  .text-subtle { color: var(--text-subtle); }
  .text-danger { color: var(--danger); }
  .font-bold { font-weight: 600; }
  .font-semibold { font-weight: 500; }
  .truncate {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 260px;
    display: inline-block;
  }
  .spin { animation: spin 1s linear infinite; }
  @keyframes spin { 100% { transform: rotate(360deg); } }
  .table-empty {
    text-align: center;
    padding: 2.5rem;
    color: var(--text-subtle);
  }
  .btn-sm {
    padding: 0.35rem 0.65rem;
    font-size: 0.78rem;
  }
</style>

