<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { SystemProcess, PortListening, CleanerScanResult } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import '../styles/pages/systemmanager.css';

  let activeTab = $state<'processes' | 'ports' | 'cleaner'>('processes');

  // State daftar proses
  let processes = $state<SystemProcess[]>([]);
  let loadingProcs = $state(false);

  // State daftar port
  let ports = $state<PortListening[]>([]);
  let loadingPorts = $state(false);

  // State pembersih sistem
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
      confirmText: 'Hentikan',
      tone: 'danger',
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
      confirmText: 'Bersihkan Sekarang',
      tone: 'default',
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

<div class="pg-systemmanager sys-view">
  <!-- Baris kepala halaman -->
  <header class="page-header">
    <div class="page-header__main">
      <h1 class="page-header__title">
        <i class="ri-speed-up-line"></i> Task Manager & System Cleaner
      </h1>
      <p class="page-header__sub">
        Pantau proses sistem paling berat, audit port jaringan yang mendengarkan (listening), dan optimalkan ruang disk serta alokasi buffer RAM.
      </p>
    </div>
  </header>

  <!-- Navigasi tab -->
  <div class="tabs" role="tablist">
    <button
      role="tab"
      aria-selected={activeTab === 'processes'}
      class="tabs__tab"
      class:is-active={activeTab === 'processes'}
      onclick={() => { activeTab = 'processes'; loadProcesses(); }}
    >
      <i class="ri-cpu-line"></i> Task Manager (Top Proses)
    </button>
    <button
      role="tab"
      aria-selected={activeTab === 'ports'}
      class="tabs__tab"
      class:is-active={activeTab === 'ports'}
      onclick={() => { activeTab = 'ports'; loadPorts(); }}
    >
      <i class="ri-broadcast-line"></i> Port Listening Jaringan
    </button>
    <button
      role="tab"
      aria-selected={activeTab === 'cleaner'}
      class="tabs__tab"
      class:is-active={activeTab === 'cleaner'}
      onclick={() => { activeTab = 'cleaner'; }}
    >
      <i class="ri-magic-line"></i> Pembersih Sistem & RAM
    </button>
  </div>

  <!-- Tab 1: Task Manager -->
  {#if activeTab === 'processes'}
    <div class="card">
      <div class="card__head flex justify-between items-center">
        <h3 class="card__title flex items-center gap-2">
          <i class="ri-cpu-line"></i> Proses Teratas Pengguna Sumber Daya
        </h3>
        <button class="btn btn--ghost" onclick={loadProcesses} disabled={loadingProcs}>
          <i class="ri-refresh-line {loadingProcs ? 'spin' : ''}"></i> Refresh
        </button>
      </div>

      <div class="table-wrap">
        <table class="table">
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
                  <td class="mono font-bold">#{p.pid}</td>
                  <td class="mono text-muted">{p.user}</td>
                  <td class="mono font-medium text-accent">{p.cpu.toFixed(1)}%</td>
                  <td class="mono font-medium">{p.memory.toFixed(1)} MB</td>
                  <td class="mono text-muted cmd-cell truncate" title={p.command}>{p.command}</td>
                  <td class="text-right">
                    <button class="btn btn--icon btn--sm btn--danger" aria-label="Matikan proses #{p.pid}" title="Matikan Proses" onclick={() => killProcess(p)}>
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
    <div class="card">
      <div class="card__head flex justify-between items-center">
        <h3 class="card__title flex items-center gap-2">
          <i class="ri-broadcast-line"></i> Port Jaringan yang Terbuka (Listening)
        </h3>
        <button class="btn btn--ghost" onclick={loadPorts} disabled={loadingPorts}>
          <i class="ri-refresh-line {loadingPorts ? 'spin' : ''}"></i> Refresh
        </button>
      </div>

      <div class="table-wrap">
        <table class="table">
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
                  <td><span class="badge badge--info">{port.proto.toUpperCase()}</span></td>
                  <td class="mono font-bold text-accent">{port.port}</td>
                  <td class="mono text-muted">{port.address}</td>
                  <td class="mono font-medium">{port.program}</td>
                  <td class="mono text-subtle">{port.pid ? '#' + port.pid : '-'}</td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- Tab 3: Pembersih sistem & optimasi RAM -->
  {#if activeTab === 'cleaner'}
    <div class="cleaner-grid">
      <div class="card">
        <div class="card__head">
          <h3 class="card__title flex items-center gap-2">
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
            <button class="btn btn--ghost" onclick={scanJunk} disabled={scanning}>
              <i class="ri-scan-line {scanning ? 'spin' : ''}"></i> Pindai Ulang
            </button>
            <button class="btn btn--primary" onclick={cleanSystem} disabled={cleaning}>
              <i class="ri-delete-bin-2-line"></i> {cleaning ? 'Sedang Membersihkan...' : 'Bersihkan Sekarang'}
            </button>
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>
