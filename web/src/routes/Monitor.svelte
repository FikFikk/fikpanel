<script lang="ts">
  import { onMount } from 'svelte';
  import Chart from '../lib/components/Chart.svelte';
  import HistoryChart from '../lib/components/HistoryChart.svelte';
  import { formatBytes, apiFetch } from '../lib/api';
  import type { MetricPoint, MonitorSnapshot, MonitorConfig } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  interface Props {
    currentMetric: MetricPoint;
    history: MetricPoint[];
  }

  let { currentMetric, history }: Props = $props();

  // Tab mode: 'history' (Persistent aaPanel style) vs 'realtime' (Live SSE 60s)
  let activeTab = $state<'history' | 'realtime'>('history');

  // Histori persistent state
  let snapshots = $state<MonitorSnapshot[]>([]);
  let loadingHistory = $state(false);
  let timeRange = $state<'1h' | '6h' | '24h' | '7d' | '30d'>('24h');
  let chartMode = $state<'load' | 'resource' | 'network'>('load');

  // Monitor toggle config
  let monitorConfig = $state<MonitorConfig>({ enabled: true, save_days: 30 });
  let updatingConfig = $state(false);

  async function loadMonitorConfig() {
    try {
      const res = await apiFetch<{ ok: boolean; config: MonitorConfig }>('/api/monitor/config');
      if (res?.config) {
        monitorConfig = res.config;
      }
    } catch {
      // ignore
    }
  }

  async function toggleMonitor() {
    try {
      updatingConfig = true;
      const newStatus = !monitorConfig.enabled;
      await apiFetch('/api/monitor/config', {
        method: 'POST',
        body: JSON.stringify({
          enabled: newStatus,
          save_days: monitorConfig.save_days,
        }),
      });
      monitorConfig.enabled = newStatus;
      notify(
        newStatus
          ? 'Status Monitoring diaktifkan: Riwayat performa dicatat ke database'
          : 'Status Monitoring dimatikan: Perekaman dihentikan (kosong/tidak mencatat)',
        'info'
      );
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status monitoring', 'error');
    } finally {
      updatingConfig = false;
    }
  }

  async function loadHistory() {
    try {
      loadingHistory = true;
      const now = Math.floor(Date.now() / 1000);
      let duration = 86400; // 24h
      if (timeRange === '1h') duration = 3600;
      else if (timeRange === '6h') duration = 21600;
      else if (timeRange === '7d') duration = 86400 * 7;
      else if (timeRange === '30d') duration = 86400 * 30;

      const start = now - duration;
      const res = await apiFetch<{ ok: boolean; history: MonitorSnapshot[] }>(
        `/api/monitor/history?start=${start}&end=${now}`
      );
      snapshots = res?.history || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat histori monitoring', 'error');
    } finally {
      loadingHistory = false;
    }
  }

  async function clearHistory() {
    const ok = await confirmDialog({
      title: 'Kosongkan Riwayat Monitoring',
      message: 'Hapus seluruh rekaman histori metrik monitoring yang tersimpan di server?',
      confirmLabel: 'Kosongkan',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch('/api/monitor/history', { method: 'DELETE' });
      notify('Seluruh data histori monitoring berhasil dikosongkan', 'success');
      snapshots = [];
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus rekaman', 'error');
    }
  }

  $effect(() => {
    if (activeTab === 'history') {
      const _tr = timeRange;
      loadHistory();
    }
  });

  onMount(() => {
    loadMonitorConfig();
    loadHistory();
  });
</script>

<div class="monitor-container">
  <!-- Header Bar -->
  <div class="head-bar">
    <div>
      <h2 class="title">Monitor Performa & Beban Server</h2>
      <p class="subtitle">Pelacak metrik histori beban CPU, RAM, Network I/O, dan Load Average ala aaPanel</p>
    </div>

    <div class="monitor-controls">
      <!-- Status Switch Toggle ala aaPanel -->
      <div class="switch-box">
        <span class="switch-label">Status Monitoring:</span>
        <button
          class="switch-btn {monitorConfig.enabled ? 'active' : ''}"
          onclick={toggleMonitor}
          disabled={updatingConfig}
        >
          <span class="switch-knob"></span>
        </button>
        <span class="switch-text {monitorConfig.enabled ? 'text-accent' : 'text-subtle'}">
          {monitorConfig.enabled ? 'MENYALA' : 'MATI'}
        </span>
      </div>

      <div class="tab-toggle">
        <button
          class="sub-tab {activeTab === 'history' ? 'active' : ''}"
          onclick={() => (activeTab = 'history')}
        >
          <i class="ri-history-line"></i> Rekaman Histori
        </button>
        <button
          class="sub-tab {activeTab === 'realtime' ? 'active' : ''}"
          onclick={() => (activeTab = 'realtime')}
        >
          <i class="ri-pulse-line"></i> Live Streaming (1s)
        </button>
      </div>
    </div>
  </div>

  <!-- Detail Metrik Cards Overview -->
  <div class="metrics-grid">
    <div class="metric-card">
      <div class="m-label">CPU Core Usage</div>
      <div class="m-val" style="color: var(--info);">{currentMetric.cpu.toFixed(1)}%</div>
      <div class="m-sub">Load: {currentMetric.load_avg.toFixed(2)} (1m) &middot; {currentMetric.load_5m?.toFixed(2) || currentMetric.load_avg.toFixed(2)} (5m)</div>
    </div>
    <div class="metric-card">
      <div class="m-label">Physical Memory (RAM)</div>
      <div class="m-val" style="color: var(--violet);">
        {currentMetric.ram_total > 0
          ? ((currentMetric.ram_used / currentMetric.ram_total) * 100).toFixed(1)
          : '0'}%
      </div>
      <div class="m-sub">{formatBytes(currentMetric.ram_used)} / {formatBytes(currentMetric.ram_total)}</div>
    </div>
    <div class="metric-card">
      <div class="m-label">Jaringan Masuk (Inbound)</div>
      <div class="m-val" style="color: var(--accent);">{formatBytes(currentMetric.net_in)}/s</div>
      <div class="m-sub">Traffic download / request masuk</div>
    </div>
    <div class="metric-card">
      <div class="m-label">Jaringan Keluar (Outbound)</div>
      <div class="m-val" style="color: var(--warning);">{formatBytes(currentMetric.net_out)}/s</div>
      <div class="m-sub">Traffic upload / respon web</div>
    </div>
  </div>

  {#if activeTab === 'history'}
    <!-- Persistent History View (aaPanel style) -->
    <div class="panel">
      <div class="panel-head flex-wrap">
        <div>
          <h3 class="panel-title flex items-center gap-2">
            <i class="ri-time-line"></i> Grafik Rekaman Histori Server
          </h3>
          <p class="panel-sub">
            {#if monitorConfig.enabled}
              Mencatat data berkala setiap 60 detik. Retensi penyimpanan: {monitorConfig.save_days} hari.
            {:else}
              <span class="text-danger font-semibold">Monitoring saat ini non-aktif! Tidak ada data baru yang dicatat selama status mati.</span>
            {/if}
          </p>
        </div>

        <div class="filter-controls">
          <!-- Filter Jenis Metrik -->
          <div class="btn-group">
            <button
              class="filter-btn {chartMode === 'load' ? 'active' : ''}"
              onclick={() => (chartMode = 'load')}
            >
              Load Average
            </button>
            <button
              class="filter-btn {chartMode === 'resource' ? 'active' : ''}"
              onclick={() => (chartMode = 'resource')}
            >
              CPU & RAM
            </button>
            <button
              class="filter-btn {chartMode === 'network' ? 'active' : ''}"
              onclick={() => (chartMode = 'network')}
            >
              Network I/O
            </button>
          </div>

          <!-- Filter Rentang Waktu -->
          <div class="btn-group">
            <button class="filter-btn {timeRange === '1h' ? 'active' : ''}" onclick={() => (timeRange = '1h')}>1 Jam</button>
            <button class="filter-btn {timeRange === '6h' ? 'active' : ''}" onclick={() => (timeRange = '6h')}>6 Jam</button>
            <button class="filter-btn {timeRange === '24h' ? 'active' : ''}" onclick={() => (timeRange = '24h')}>Hari Ini</button>
            <button class="filter-btn {timeRange === '7d' ? 'active' : ''}" onclick={() => (timeRange = '7d')}>7 Hari</button>
            <button class="filter-btn {timeRange === '30d' ? 'active' : ''}" onclick={() => (timeRange = '30d')}>30 Hari</button>
          </div>

          <button class="btn btn-ghost btn-sm" onclick={loadHistory} disabled={loadingHistory} title="Refresh Rekaman">
            <i class="ri-refresh-line {loadingHistory ? 'spin' : ''}"></i>
          </button>
          <button class="btn btn-ghost btn-sm danger" onclick={clearHistory} title="Kosongkan Semua Catatan">
            <i class="ri-delete-bin-line"></i>
          </button>
        </div>
      </div>

      <!-- Legend Indicator -->
      <div class="chart-legend-row">
        {#if chartMode === 'load'}
          <span class="l-item"><span class="l-dot" style="background: var(--info);"></span> 1 Menit Load</span>
          <span class="l-item"><span class="l-dot" style="background: var(--violet);"></span> 5 Menit Load</span>
          <span class="l-item"><span class="l-dot" style="background: var(--accent);"></span> 15 Menit Load</span>
        {:else if chartMode === 'resource'}
          <span class="l-item"><span class="l-dot" style="background: var(--info);"></span> CPU Utilisasi %</span>
          <span class="l-item"><span class="l-dot" style="background: var(--violet);"></span> RAM Utilisasi %</span>
        {:else if chartMode === 'network'}
          <span class="l-item"><span class="l-dot" style="background: var(--accent);"></span> Inbound (Download)</span>
          <span class="l-item"><span class="l-dot" style="background: var(--warning);"></span> Outbound (Upload)</span>
        {/if}
        <span class="total-badge mono">Total Titik: {snapshots.length} sampel</span>
      </div>

      <div class="chart-box">
        <HistoryChart {snapshots} height={280} mode={chartMode} />
      </div>
    </div>
  {:else}
    <!-- Live Realtime View -->
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">Grafik Utilisasi CPU & RAM (Realtime SSE)</h3>
          <p class="panel-sub">Buffer 60 detik terakhir &middot; Auto-scaling canvas render langsung dari kernel</p>
        </div>
        <div class="legend">
          <span class="l-item"><span class="l-dot" style="background: var(--info);"></span> CPU %</span>
          <span class="l-item"><span class="l-dot" style="background: var(--violet);"></span> RAM %</span>
        </div>
      </div>
      <Chart {history} height={260} />
    </div>
  {/if}
</div>

<style>
  .monitor-container {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .head-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
  }
  .title {
    font-size: 1.4rem;
    font-weight: 700;
    color: var(--text);
    margin: 0;
  }
  .subtitle {
    font-size: 0.85rem;
    color: var(--text-muted);
    margin: 0.25rem 0 0 0;
  }
  .monitor-controls {
    display: flex;
    align-items: center;
    gap: 1.25rem;
    flex-wrap: wrap;
  }
  .switch-box {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    padding: 0.35rem 0.75rem;
    border-radius: var(--radius-full);
  }
  .switch-label {
    font-size: 0.78rem;
    color: var(--text-muted);
    font-weight: 600;
  }
  .switch-btn {
    width: 36px;
    height: 20px;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-full);
    position: relative;
    cursor: pointer;
    transition: background 0.2s, border-color 0.2s;
    padding: 0;
  }
  .switch-btn.active {
    background: var(--accent);
    border-color: var(--accent);
  }
  .switch-knob {
    width: 14px;
    height: 14px;
    background: var(--bg-surface);
    border-radius: 50%;
    position: absolute;
    top: 2px;
    left: 2px;
    transition: transform 0.2s;
  }
  .switch-btn.active .switch-knob {
    transform: translateX(16px);
  }
  .switch-text {
    font-size: 0.75rem;
    font-weight: 700;
    font-family: var(--font-mono);
  }
  .tab-toggle {
    display: flex;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 2px;
  }
  .sub-tab {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.4rem 0.75rem;
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.15s;
  }
  .sub-tab:hover {
    color: var(--text);
  }
  .sub-tab.active {
    background: var(--bg-surface-soft);
    color: var(--accent);
  }
  .panel {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    overflow: hidden;
  }
  .panel-head {
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--border-soft);
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
  }
  .panel-title {
    font-size: 1rem;
    font-weight: 700;
    margin: 0;
    color: var(--text);
  }
  .panel-sub {
    font-size: 0.75rem;
    color: var(--text-muted);
    margin: 0.25rem 0 0 0;
  }
  .filter-controls {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .btn-group {
    display: flex;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 2px;
  }
  .filter-btn {
    padding: 0.35rem 0.65rem;
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.15s;
  }
  .filter-btn:hover {
    color: var(--text);
  }
  .filter-btn.active {
    background: var(--accent);
    color: var(--text-inverse);
  }
  .chart-legend-row {
    display: flex;
    align-items: center;
    gap: 1.25rem;
    padding: 0.75rem 1.25rem;
    background: var(--bg-surface-soft);
    border-bottom: 1px solid var(--border-soft);
    font-size: 0.75rem;
    color: var(--text-muted);
  }
  .l-item {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .l-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
  .total-badge {
    margin-left: auto;
    font-size: 0.7rem;
    color: var(--text-subtle);
  }
  .chart-box {
    padding: 1.25rem;
  }
  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 1rem;
  }
  .metric-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .m-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-subtle);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .m-val {
    font-size: 1.6rem;
    font-weight: 800;
    font-family: var(--font-mono);
  }
  .m-sub {
    font-size: 0.72rem;
    color: var(--text-muted);
    font-family: var(--font-mono);
  }
  .btn-sm {
    padding: 0.35rem 0.65rem;
    font-size: 0.8rem;
  }
  .btn.danger:hover {
    color: var(--danger);
  }
  .mono { font-family: var(--font-mono); }
  .text-accent { color: var(--accent); }
  .text-subtle { color: var(--text-subtle); }
  .text-danger { color: var(--danger); }
  .font-semibold { font-weight: 600; }
  .flex { display: flex; }
  .items-center { align-items: center; }
  .gap-2 { gap: 0.5rem; }
  .spin { animation: spin 1s linear infinite; }
  @keyframes spin { 100% { transform: rotate(360deg); } }
</style>
