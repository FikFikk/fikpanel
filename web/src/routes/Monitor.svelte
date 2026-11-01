<script lang="ts">
  import { onMount } from 'svelte';
  import Chart from '../lib/components/Chart.svelte';
  import HistoryChart from '../lib/components/HistoryChart.svelte';
  import { formatBytes, apiFetch } from '../lib/api';
  import type { MetricPoint, MonitorSnapshot, MonitorConfig } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import '../styles/pages/monitor.css';

  interface Props {
    currentMetric: MetricPoint;
    history: MetricPoint[];
  }

  let { currentMetric, history }: Props = $props();

  // Mode tab: 'history' (persisten ala aaPanel) vs 'realtime' (streaming SSE 60 detik)
  let activeTab = $state<'history' | 'realtime'>('history');

  // State histori persisten
  let snapshots = $state<MonitorSnapshot[]>([]);
  let loadingHistory = $state(false);
  let timeRange = $state<'1h' | '6h' | '24h' | '7d' | '30d'>('24h');
  let chartMode = $state<'load' | 'resource' | 'network'>('load');

  // Konfigurasi toggle monitoring
  let monitorConfig = $state<MonitorConfig>({ enabled: true, save_days: 30 });
  let updatingConfig = $state(false);

  async function loadMonitorConfig() {
    try {
      const res = await apiFetch<{ ok: boolean; config: MonitorConfig }>('/api/monitor/config');
      if (res?.config) {
        monitorConfig = res.config;
      }
    } catch {
      // Abaikan kesalahan pemuatan awal konfigurasi
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
      let duration = 86400; // 24 jam
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
      confirmText: 'Kosongkan',
      tone: 'danger',
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

  const load5m = $derived(
    (currentMetric as MetricPoint & { load_5m?: number }).load_5m ?? currentMetric.load_avg
  );

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

<div class="pg-monitor monitor-container">
  <!-- Baris kepala halaman -->
  <div class="page-header">
    <div class="page-header__main">
      <h2 class="page-header__title">Monitor Performa & Beban Server</h2>
      <p class="page-header__sub">Pelacak metrik histori beban CPU, RAM, Network I/O, dan Load Average ala aaPanel</p>
    </div>

    <div class="monitor-controls">
      <!-- Sakelar status monitoring -->
      <div class="switch-box">
        <span class="switch-label">Status Monitoring:</span>
        <button
          class="switch-btn {monitorConfig.enabled ? 'active' : ''}"
          aria-label="Ubah status perekaman monitoring"
          onclick={toggleMonitor}
          disabled={updatingConfig}
        >
          <span class="switch-knob"></span>
        </button>
        <span class="switch-text {monitorConfig.enabled ? 'text-accent' : 'text-subtle'}">
          {monitorConfig.enabled ? 'MENYALA' : 'MATI'}
        </span>
      </div>

      <div class="tabs tabs--pill" role="tablist">
        <button
          role="tab"
          aria-selected={activeTab === 'history'}
          class="tabs__tab"
          class:is-active={activeTab === 'history'}
          onclick={() => (activeTab = 'history')}
        >
          <i class="ri-history-line"></i> Rekaman Histori
        </button>
        <button
          role="tab"
          aria-selected={activeTab === 'realtime'}
          class="tabs__tab"
          class:is-active={activeTab === 'realtime'}
          onclick={() => (activeTab = 'realtime')}
        >
          <i class="ri-pulse-line"></i> Live Streaming (1s)
        </button>
      </div>
    </div>
  </div>

  <!-- Ringkasan kartu metrik -->
  <div class="metrics-grid">
    <div class="metric-card">
      <div class="m-label">CPU Core Usage</div>
      <div class="m-val m-val--info">{currentMetric.cpu.toFixed(1)}%</div>
      <div class="m-sub">Load: {currentMetric.load_avg.toFixed(2)} (1m) &middot; {load5m.toFixed(2)} (5m)</div>
    </div>
    <div class="metric-card">
      <div class="m-label">Physical Memory (RAM)</div>
      <div class="m-val m-val--violet">
        {currentMetric.ram_total > 0
          ? ((currentMetric.ram_used / currentMetric.ram_total) * 100).toFixed(1)
          : '0'}%
      </div>
      <div class="m-sub">{formatBytes(currentMetric.ram_used)} / {formatBytes(currentMetric.ram_total)}</div>
    </div>
    <div class="metric-card">
      <div class="m-label">Jaringan Masuk (Inbound)</div>
      <div class="m-val text-accent">{formatBytes(currentMetric.net_in)}/s</div>
      <div class="m-sub">Traffic download / request masuk</div>
    </div>
    <div class="metric-card">
      <div class="m-label">Jaringan Keluar (Outbound)</div>
      <div class="m-val text-warning">{formatBytes(currentMetric.net_out)}/s</div>
      <div class="m-sub">Traffic upload / respon web</div>
    </div>
  </div>

  {#if activeTab === 'history'}
    <!-- Tampilan histori persisten -->
    <div class="card">
      <div class="card__head flex-wrap">
        <div>
          <h3 class="card__title flex items-center gap-2">
            <i class="ri-time-line"></i> Grafik Rekaman Histori Server
          </h3>
          <p class="panel-sub">
            {#if monitorConfig.enabled}
              Mencatat data berkala setiap 60 detik. Retensi penyimpanan: {monitorConfig.save_days} hari.
            {:else}
              <span class="text-danger font-bold">Monitoring saat ini non-aktif! Tidak ada data baru yang dicatat selama status mati.</span>
            {/if}
          </p>
        </div>

        <div class="filter-controls">
          <!-- Filter jenis metrik -->
          <div class="tabs tabs--pill" role="tablist">
            <button
              role="tab"
              aria-selected={chartMode === 'load'}
              class="tabs__tab"
              class:is-active={chartMode === 'load'}
              onclick={() => (chartMode = 'load')}
            >
              Load Average
            </button>
            <button
              role="tab"
              aria-selected={chartMode === 'resource'}
              class="tabs__tab"
              class:is-active={chartMode === 'resource'}
              onclick={() => (chartMode = 'resource')}
            >
              CPU & RAM
            </button>
            <button
              role="tab"
              aria-selected={chartMode === 'network'}
              class="tabs__tab"
              class:is-active={chartMode === 'network'}
              onclick={() => (chartMode = 'network')}
            >
              Network I/O
            </button>
          </div>

          <!-- Filter rentang waktu -->
          <div class="tabs tabs--pill" role="tablist">
            <button role="tab" aria-selected={timeRange === '1h'} class="tabs__tab" class:is-active={timeRange === '1h'} onclick={() => (timeRange = '1h')}>1 Jam</button>
            <button role="tab" aria-selected={timeRange === '6h'} class="tabs__tab" class:is-active={timeRange === '6h'} onclick={() => (timeRange = '6h')}>6 Jam</button>
            <button role="tab" aria-selected={timeRange === '24h'} class="tabs__tab" class:is-active={timeRange === '24h'} onclick={() => (timeRange = '24h')}>Hari Ini</button>
            <button role="tab" aria-selected={timeRange === '7d'} class="tabs__tab" class:is-active={timeRange === '7d'} onclick={() => (timeRange = '7d')}>7 Hari</button>
            <button role="tab" aria-selected={timeRange === '30d'} class="tabs__tab" class:is-active={timeRange === '30d'} onclick={() => (timeRange = '30d')}>30 Hari</button>
          </div>

          <button class="btn btn--ghost btn--sm" aria-label="Refresh rekaman histori" onclick={loadHistory} disabled={loadingHistory} title="Refresh Rekaman">
            <i class="ri-refresh-line {loadingHistory ? 'spin' : ''}"></i>
          </button>
          <button class="btn btn--ghost btn--sm btn--danger" aria-label="Kosongkan semua catatan histori" onclick={clearHistory} title="Kosongkan Semua Catatan">
            <i class="ri-delete-bin-line"></i>
          </button>
        </div>
      </div>

      <!-- Baris legenda grafik -->
      <div class="chart-legend-row">
        {#if chartMode === 'load'}
          <span class="l-item"><span class="l-dot l-dot--info"></span> 1 Menit Load</span>
          <span class="l-item"><span class="l-dot l-dot--violet"></span> 5 Menit Load</span>
          <span class="l-item"><span class="l-dot l-dot--accent"></span> 15 Menit Load</span>
        {:else if chartMode === 'resource'}
          <span class="l-item"><span class="l-dot l-dot--info"></span> CPU Utilisasi %</span>
          <span class="l-item"><span class="l-dot l-dot--violet"></span> RAM Utilisasi %</span>
        {:else if chartMode === 'network'}
          <span class="l-item"><span class="l-dot l-dot--accent"></span> Inbound (Download)</span>
          <span class="l-item"><span class="l-dot l-dot--warning"></span> Outbound (Upload)</span>
        {/if}
        <span class="total-badge mono">Total Titik: {snapshots.length} sampel</span>
      </div>

      <div class="chart-box">
        <HistoryChart {snapshots} height={280} mode={chartMode} />
      </div>
    </div>
  {:else}
    <!-- Tampilan live realtime -->
    <div class="card">
      <div class="card__head">
        <div>
          <h3 class="card__title">Grafik Utilisasi CPU & RAM (Realtime SSE)</h3>
          <p class="panel-sub">Buffer 60 detik terakhir &middot; Auto-scaling canvas render langsung dari kernel</p>
        </div>
        <div class="legend">
          <span class="l-item"><span class="l-dot l-dot--info"></span> CPU %</span>
          <span class="l-item"><span class="l-dot l-dot--violet"></span> RAM %</span>
        </div>
      </div>
      <Chart {history} height={260} />
    </div>
  {/if}
</div>
