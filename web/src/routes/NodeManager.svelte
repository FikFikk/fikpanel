<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { PM2Process } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  let processes = $state<PM2Process[]>([]);
  let loading = $state(true);

  async function loadProcesses() {
    try {
      loading = true;
      const res = await apiFetch<{ ok: boolean; processes: PM2Process[] }>('/api/node/processes');
      if (res && res.processes) {
        processes = res.processes;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal mengambil proses Node.js / PM2', 'error');
    } finally {
      loading = false;
    }
  }

  async function handleAction(p: PM2Process, action: 'restart' | 'stop' | 'start') {
    const actionLabel = action === 'restart' ? 'Restart' : action === 'stop' ? 'Hentikan' : 'Jalankan';
    const ok = await confirmDialog({
      title: `${actionLabel} Proses`,
      message: `Apakah Anda yakin ingin melakukan ${actionLabel.toLowerCase()} pada proses "${p.name}"?`,
      confirmLabel: actionLabel,
      variant: action === 'stop' ? 'danger' : 'primary',
    });
    if (!ok) return;

    try {
      notify(`Memproses ${action} ${p.name}...`, 'info');
      await apiFetch(`/api/node/processes/${p.id}/${action}`, { method: 'POST' });
      notify(`Proses "${p.name}" berhasil di-${action}`, 'success');
      await loadProcesses();
    } catch (err: any) {
      notify(err.message || `Gagal ${action} proses`, 'error');
    }
  }

  function formatBytes(bytes: number): string {
    if (!bytes) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  onMount(() => {
    loadProcesses();
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
        Pantau penggunaan CPU, konsumsi RAM, status uptime, dan kontrol siklus hidup aplikasi SSR / API Node.js.
      </p>
    </div>
    <div class="page-actions">
      <button class="btn btn-ghost" onclick={loadProcesses} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh
      </button>
    </div>
  </header>

  <!-- Table Card -->
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
            <th>RESTART</th>
            <th>USER</th>
            <th class="text-right">KONTROL</th>
          </tr>
        </thead>
        <tbody>
          {#if loading && processes.length === 0}
            <tr>
              <td colspan="8" class="table-empty">
                <i class="ri-loader-4-line spin text-lg"></i> Memeriksa proses PM2...
              </td>
            </tr>
          {:else if processes.length === 0}
            <tr>
              <td colspan="8" class="table-empty">
                Tidak ada aplikasi Node.js / PM2 yang aktif di sistem.
              </td>
            </tr>
          {:else}
            {#each processes as p (p.id)}
              <tr>
                <td>
                  <div class="proc-name-col">
                    <span class="badge-id">#{p.id}</span>
                    <span class="font-bold text-white">{p.name}</span>
                    {#if p.version}
                      <span class="text-subtle text-xs">v{p.version}</span>
                    {/if}
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
                <td class="text-muted">{p.restart_count || 0} kali</td>
                <td class="mono text-muted">{p.user || 'root'}</td>
                <td class="text-right actions-col">
                  {#if p.status === 'online'}
                    <button class="btn-icon" title="Restart Aplikasi" onclick={() => handleAction(p, 'restart')}>
                      <i class="ri-restart-line"></i>
                    </button>
                    <button class="btn-icon danger" title="Hentikan Aplikasi" onclick={() => handleAction(p, 'stop')}>
                      <i class="ri-stop-circle-line"></i>
                    </button>
                  {:else}
                    <button class="btn-icon success" title="Nyalakan Aplikasi" onclick={() => handleAction(p, 'start')}>
                      <i class="ri-play-circle-line"></i>
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
    gap: 0.5rem;
  }
  .badge-id {
    font-size: 0.7rem;
    font-family: var(--font-mono);
    padding: 0.1rem 0.35rem;
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
  .text-right { text-align: right; }
  .mono { font-family: var(--font-mono); }
  .text-muted { color: var(--text-muted); }
  .text-subtle { color: var(--text-subtle); }
  .font-bold { font-weight: 600; }
  .font-semibold { font-weight: 500; }
  .spin { animation: spin 1s linear infinite; }
  @keyframes spin { 100% { transform: rotate(360deg); } }
  .table-empty {
    text-align: center;
    padding: 2.5rem;
    color: var(--text-subtle);
  }
</style>
