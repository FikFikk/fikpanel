<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { SystemProcess, PortListening, CleanerScanResult } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  let activeTab = $state<'processes' | 'ports' | 'cleaner'>('processes');

  // Processes state
  let processes = $state<SystemProcess[]>([]);
  let loadingProcs = $state(false);

  // Ports state
  let ports = $state<PortListening[]>([]);
  let loadingPorts = $state(false);

  // Cleaner state
  let scanResult = $state<CleanerScanResult | null>(null);
  let scanning = $state(false);
  let cleaning = $state(false);
  let optApt = $state(true);
  let optLogs = $state(true);
  let optTmp = $state(true);
  let optDropCaches = $state(true);

  async function loadProcesses() {
    try {
      loadingProcs = true;
      const res = await apiFetch<{ ok: boolean; processes: SystemProcess[] }>('/api/system/processes?limit=40');
      processes = res?.processes || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat proses sistem', 'error');
    } finally {
      loadingProcs = false;
    }
  }

  async function killProcess(p: SystemProcess) {
    const ok = await confirmDialog({
      title: 'Hentikan Proses',
      message: `Hentikan proses #${p.pid} (${p.command})?`,
      confirmLabel: 'Hentikan',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/system/processes/${p.pid}/kill?force=true`, { method: 'POST' });
      notify(`Proses #${p.pid} dihentikan`, 'success');
      await loadProcesses();
    } catch (err: any) {
      notify(err.message || 'Gagal menghentikan proses', 'error');
    }
  }

  async function loadPorts() {
    try {
      loadingPorts = true;
      const res = await apiFetch<{ ok: boolean; ports: PortListening[] }>('/api/system/ports');
      ports = res?.ports || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat daftar port listening', 'error');
    } finally {
      loadingPorts = false;
    }
  }

  async function scanJunk() {
    try {
      scanning = true;
      const res = await apiFetch<{ ok: boolean; scan: CleanerScanResult }>('/api/system/cleaner/scan');
      scanResult = res?.scan || null;
      notify('Pemindaian file sampah selesai', 'info');
    } catch (err: any) {
      notify(err.message || 'Gagal memindai sampah', 'error');
    } finally {
      scanning = false;
    }
  }

  async function cleanSystem() {
    const ok = await confirmDialog({
      title: 'Optimasi & Pembersihan Server',
      message: 'Bersihkan cache apt, arsip log lama, berkas temporary, dan lepaskan buffer RAM yang mengendap?',
      confirmLabel: 'Bersihkan Sekarang',
      variant: 'primary',
    });
    if (!ok) return;

    try {
      cleaning = true;
      notify('Membersihkan sistem...', 'info');
      await apiFetch('/api/system/cleaner/clean', {
        method: 'POST',
        body: JSON.stringify({
          clean_apt: optApt,
          clean_logs: optLogs,
          clean_tmp: optTmp,
          drop_caches: optDropCaches,
        }),
      });
      notify('Sistem berhasil dibersihkan dan RAM dioptimalkan', 'success');
      await scanJunk();
    } catch (err: any) {
      notify(err.message || 'Gagal membersihkan sistem', 'error');
    } finally {
      cleaning = false;
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
    scanJunk();
  });
</script>

<div class="sys-view">
  <!-- Header Bar -->
  <header class="page-head">
    <div class="page-title-wrap">
      <h1 class="page-title">
        <i class="ri-speed-up-line"></i> Task Manager & System Cleaner
      </h1>
      <p class="page-sub">
        Pantau proses sistem paling berat, audit port jaringan yang mendengarkan (listening), dan optimalkan ruang disk serta alokasi buffer RAM.
      </p>
    </div>
  </header>

  <!-- Navigation Tabs -->
  <div class="tabs-nav">
    <button
      class="tab-btn {activeTab === 'processes' ? 'active' : ''}"
      onclick={() => { activeTab = 'processes'; loadProcesses(); }}
    >
      <i class="ri-cpu-line"></i> Task Manager (Top Proses)
    </button>
    <button
      class="tab-btn {activeTab === 'ports' ? 'active' : ''}"
      onclick={() => { activeTab = 'ports'; loadPorts(); }}
    >
      <i class="ri-broadcast-line"></i> Port Listening Jaringan
    </button>
    <button
      class="tab-btn {activeTab === 'cleaner' ? 'active' : ''}"
      onclick={() => { activeTab = 'cleaner'; }}
    >
      <i class="ri-magic-line"></i> Pembersih Sistem & RAM
    </button>
  </div>

  <!-- Tab 1: Task Manager -->
  {#if activeTab === 'processes'}
    <div class="panel-card">
      <div class="panel-head flex justify-between items-center">
        <h3 class="panel-title flex items-center gap-2">
          <i class="ri-cpu-line"></i> Proses Teratas Pengguna Sumber Daya
        </h3>
        <button class="btn btn-ghost" onclick={loadProcesses} disabled={loadingProcs}>
          <i class="ri-refresh-line {loadingProcs ? 'spin' : ''}"></i> Refresh
        </button>
      </div>

      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>PID</th>
              <th>USER</th>
              <th>CPU %</th>
              <th>MEMORI (RAM)</th>
              <th>PERINTAH / PROGRAM</th>
              <th class="text-right">AKSI</th>
            </tr>
          </thead>
          <tbody>
            {#if loadingProcs && processes.length === 0}
              <tr><td colspan="6" class="table-empty">Memeriksa proses Linux...</td></tr>
            {:else if processes.length === 0}
              <tr><td colspan="6" class="table-empty">Tidak ada data proses.</td></tr>
            {:else}
              {#each processes as p (p.pid)}
                <tr>
                  <td class="mono font-bold text-white">#{p.pid}</td>
                  <td class="mono text-muted">{p.user}</td>
                  <td class="mono font-semibold text-accent">{p.cpu.toFixed(1)}%</td>
                  <td class="mono font-semibold">{p.memory.toFixed(1)} MB</td>
                  <td class="mono text-muted max-w-md truncate" title={p.command}>{p.command}</td>
                  <td class="text-right">
                    <button class="btn-icon danger" title="Matikan Proses" onclick={() => killProcess(p)}>
                      <i class="ri-close-circle-line"></i>
                    </button>
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- Tab 2: Port Listening -->
  {#if activeTab === 'ports'}
    <div class="panel-card">
      <div class="panel-head flex justify-between items-center">
        <h3 class="panel-title flex items-center gap-2">
          <i class="ri-broadcast-line"></i> Port Jaringan yang Terbuka (Listening)
        </h3>
        <button class="btn btn-ghost" onclick={loadPorts} disabled={loadingPorts}>
          <i class="ri-refresh-line {loadingPorts ? 'spin' : ''}"></i> Refresh
        </button>
      </div>

      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>PROTOKOL</th>
              <th>PORT</th>
              <th>BIND ADDRESS</th>
              <th>PROGRAM / SERVICE</th>
              <th>PID</th>
            </tr>
          </thead>
          <tbody>
            {#if loadingPorts && ports.length === 0}
              <tr><td colspan="5" class="table-empty">Memindai port terbuka...</td></tr>
            {:else if ports.length === 0}
              <tr><td colspan="5" class="table-empty">Tidak ada port yang mendengarkan.</td></tr>
            {:else}
              {#each ports as port (port.proto + port.port + port.address)}
                <tr>
                  <td><span class="badge-proto">{port.proto.toUpperCase()}</span></td>
                  <td class="mono font-bold text-accent">{port.port}</td>
                  <td class="mono text-muted">{port.address}</td>
                  <td class="mono font-semibold text-white">{port.program}</td>
                  <td class="mono text-subtle">{port.pid ? '#' + port.pid : '-'}</td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- Tab 3: Cleaner & RAM Optimization -->
  {#if activeTab === 'cleaner'}
    <div class="cleaner-grid">
      <div class="panel-card">
        <div class="panel-head">
          <h3 class="panel-title flex items-center gap-2">
            <i class="ri-magic-line"></i> Hasil Analisis Sampah Sistem
          </h3>
        </div>
        <div class="cleaner-body">
          <div class="stat-banner">
            <div class="stat-value">{formatBytes(scanResult?.total_bytes || 0)}</div>
            <div class="stat-desc">Total file cache & log yang aman untuk dibersihkan</div>
          </div>

          <div class="junk-items">
            <div class="junk-row">
              <span class="junk-label">Apt Package Cache (/var/cache/apt)</span>
              <span class="mono font-bold">{formatBytes(scanResult?.apt_cache_bytes || 0)}</span>
            </div>
            <div class="junk-row">
              <span class="junk-label">Rotated System Logs (*.gz, *.1)</span>
              <span class="mono font-bold">{formatBytes(scanResult?.system_log_bytes || 0)}</span>
            </div>
            <div class="junk-row">
              <span class="junk-label">Temporary Files (/tmp & /var/tmp)</span>
              <span class="mono font-bold">{formatBytes(scanResult?.tmp_bytes || 0)}</span>
            </div>
          </div>

          <div class="clean-opts">
            <label class="custom-chk">
              <input type="checkbox" bind:checked={optApt} />
              <span>Bersihkan Cache Paket APT</span>
            </label>
            <label class="custom-chk">
              <input type="checkbox" bind:checked={optLogs} />
              <span>Hapus Arsip Log Usang (.gz / .old)</span>
            </label>
            <label class="custom-chk">
              <input type="checkbox" bind:checked={optTmp} />
              <span>Bersihkan File Temp (> 3 Hari)</span>
            </label>
            <label class="custom-chk">
              <input type="checkbox" bind:checked={optDropCaches} />
              <span>Kosongkan Buffer & Cache RAM (Drop Caches)</span>
            </label>
          </div>

          <div class="clean-actions">
            <button class="btn btn-ghost" onclick={scanJunk} disabled={scanning}>
              <i class="ri-scan-line {scanning ? 'spin' : ''}"></i> Pindai Ulang
            </button>
            <button class="btn btn-primary" onclick={cleanSystem} disabled={cleaning}>
              <i class="ri-delete-bin-2-line"></i> {cleaning ? 'Sedang Membersihkan...' : 'Bersihkan Sekarang'}
            </button>
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .sys-view {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .page-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
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
  .tabs-nav {
    display: flex;
    gap: 0.5rem;
    border-bottom: 1px solid var(--border);
    padding-bottom: 0.5rem;
  }
  .tab-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.55rem 1rem;
    background: transparent;
    border: none;
    border-radius: var(--radius-md);
    color: var(--text-muted);
    font-size: 0.82rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s;
  }
  .tab-btn:hover {
    color: var(--text);
    background: var(--bg-hover);
  }
  .tab-btn.active {
    color: var(--accent);
    background: var(--tint-accent);
  }
  .panel-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    overflow: hidden;
  }
  .panel-head {
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--border-soft);
  }
  .panel-title {
    font-size: 1rem;
    font-weight: 700;
    margin: 0;
    color: var(--text);
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
  .badge-proto {
    display: inline-block;
    padding: 0.15rem 0.4rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 0.7rem;
    font-weight: 700;
    color: var(--accent);
  }
  .btn-icon {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0.35rem;
    border-radius: var(--radius-sm);
    font-size: 1.1rem;
  }
  .btn-icon.danger:hover {
    background: var(--tint-danger);
    color: var(--danger);
  }
  .cleaner-body {
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    max-width: 640px;
  }
  .stat-banner {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 1.25rem;
    text-align: center;
  }
  .stat-value {
    font-size: 2.2rem;
    font-weight: 800;
    color: var(--accent);
    font-family: var(--font-mono);
  }
  .stat-desc {
    font-size: 0.8rem;
    color: var(--text-muted);
    margin-top: 0.25rem;
  }
  .junk-items {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .junk-row {
    display: flex;
    justify-content: space-between;
    padding: 0.6rem 0.85rem;
    background: var(--bg-surface-soft);
    border-radius: var(--radius-md);
    font-size: 0.82rem;
  }
  .clean-opts {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.75rem;
    padding: 1rem;
    background: var(--bg-inset);
    border-radius: var(--radius-md);
  }
  .custom-chk {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.8rem;
    color: var(--text);
    cursor: pointer;
  }
  .clean-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
  }
  .text-right { text-align: right; }
  .mono { font-family: var(--font-mono); }
  .text-muted { color: var(--text-muted); }
  .text-subtle { color: var(--text-subtle); }
  .text-accent { color: var(--accent); }
  .font-bold { font-weight: 600; }
  .font-semibold { font-weight: 500; }
  .table-empty {
    text-align: center;
    padding: 2.5rem;
    color: var(--text-subtle);
  }
  .flex { display: flex; }
  .justify-between { justify-content: space-between; }
  .items-center { align-items: center; }
  .gap-2 { gap: 0.5rem; }
  .spin { animation: spin 1s linear infinite; }
  @keyframes spin { 100% { transform: rotate(360deg); } }
</style>
