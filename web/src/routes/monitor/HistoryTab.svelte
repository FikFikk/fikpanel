<script lang="ts">
  import HistoryChart, { type HistoryChartMode } from '../../lib/components/HistoryChart.svelte';
  import { formatBytes } from '../../lib/api';
  import type {
    MonitorConfig,
    MonitorHistorySummary,
    MonitorSnapshot,
    MonitorThresholds,
  } from '../../lib/types';

  interface Props {
    snapshots: MonitorSnapshot[];
    summary: MonitorHistorySummary | null;
    thresholds: MonitorThresholds;
    monitorConfig: MonitorConfig;
    loadingHistory: boolean;
    updatingConfig: boolean;
    timeRange: '1h' | '6h' | '24h' | '7d' | '30d';
    chartMode: HistoryChartMode;
    exportUrl: string;
    onRefresh: () => void;
    onClearHistory: () => void;
    onChangeRetention: (days: number) => void;
  }

  let {
    snapshots,
    summary,
    thresholds,
    monitorConfig,
    loadingHistory,
    updatingConfig,
    timeRange = $bindable(),
    chartMode = $bindable(),
    exportUrl,
    onRefresh,
    onClearHistory,
    onChangeRetention,
  }: Props = $props();

  let activeThresholdValue = $derived.by(() => {
    if (chartMode === 'resource') return thresholds.alert_cpu_pct || 85;
    if (chartMode === 'disk_space') return thresholds.alert_disk_pct || 90;
    return 0;
  });

  let activeThresholdLabel = $derived.by(() => {
    if (chartMode === 'resource') return `Ambang CPU ${thresholds.alert_cpu_pct || 85}%`;
    if (chartMode === 'disk_space') return `Ambang Disk ${thresholds.alert_disk_pct || 90}%`;
    return '';
  });
</script>

<div class="mon-pane">
  {#if !monitorConfig.enabled}
    <div class="alert alert--warning" role="alert">
      <i class="ri-pause-circle-line" aria-hidden="true"></i>
      <div>
        <strong>Perekaman Histori Sedang Nonaktif:</strong> Aktifkan sakelar <strong>Status Monitoring</strong> di kanan atas agar metrik server kembali direkam otomatis setiap 60 detik.
      </div>
    </div>
  {/if}

  <div class="card">
    <div class="mon-toolbar">
      <div class="mon-toolbar-group">
        <!-- Pemilih jenis grafik 5 dimensi -->
        <div class="tabs tabs--pill" role="tablist" aria-label="Jenis metrik histori">
          <button
            type="button"
            role="tab"
            class="tabs__tab"
            class:is-active={chartMode === 'resource'}
            aria-selected={chartMode === 'resource'}
            onclick={() => (chartMode = 'resource')}
          >
            CPU &amp; RAM
          </button>
          <button
            type="button"
            role="tab"
            class="tabs__tab"
            class:is-active={chartMode === 'load'}
            aria-selected={chartMode === 'load'}
            onclick={() => (chartMode = 'load')}
          >
            Load Avg
          </button>
          <button
            type="button"
            role="tab"
            class="tabs__tab"
            class:is-active={chartMode === 'network'}
            aria-selected={chartMode === 'network'}
            onclick={() => (chartMode = 'network')}
          >
            Network I/O
          </button>
          <button
            type="button"
            role="tab"
            class="tabs__tab"
            class:is-active={chartMode === 'disk_io'}
            aria-selected={chartMode === 'disk_io'}
            onclick={() => (chartMode = 'disk_io')}
          >
            Disk I/O
          </button>
          <button
            type="button"
            role="tab"
            class="tabs__tab"
            class:is-active={chartMode === 'disk_space'}
            aria-selected={chartMode === 'disk_space'}
            onclick={() => (chartMode = 'disk_space')}
          >
            Ruang Disk
          </button>
        </div>
      </div>

      <div class="mon-toolbar-group">
        <!-- Rentang waktu historis -->
        <div class="tabs tabs--pill" role="tablist" aria-label="Rentang waktu histori">
          <button type="button" role="tab" class="tabs__tab" class:is-active={timeRange === '1h'} aria-selected={timeRange === '1h'} onclick={() => (timeRange = '1h')}>1j</button>
          <button type="button" role="tab" class="tabs__tab" class:is-active={timeRange === '6h'} aria-selected={timeRange === '6h'} onclick={() => (timeRange = '6h')}>6j</button>
          <button type="button" role="tab" class="tabs__tab" class:is-active={timeRange === '24h'} aria-selected={timeRange === '24h'} onclick={() => (timeRange = '24h')}>24j</button>
          <button type="button" role="tab" class="tabs__tab" class:is-active={timeRange === '7d'} aria-selected={timeRange === '7d'} onclick={() => (timeRange = '7d')}>7h</button>
          <button type="button" role="tab" class="tabs__tab" class:is-active={timeRange === '30d'} aria-selected={timeRange === '30d'} onclick={() => (timeRange = '30d')}>30h</button>
        </div>

        <select
          class="select"
          aria-label="Retensi penyimpanan histori"
          value={monitorConfig.save_days}
          disabled={updatingConfig}
          onchange={(e) => onChangeRetention(Number((e.currentTarget as HTMLSelectElement).value))}
        >
          <option value={7}>Retensi: 7 Hari</option>
          <option value={14}>Retensi: 14 Hari</option>
          <option value={30}>Retensi: 30 Hari</option>
          <option value={60}>Retensi: 60 Hari</option>
          <option value={90}>Retensi: 90 Hari</option>
        </select>

        <a
          class="btn btn--secondary btn--sm"
          href={exportUrl}
          download
          title="Unduh data sampel mentah periode ini dalam format CSV"
        >
          <i class="ri-file-excel-2-line" aria-hidden="true"></i>
          <span>CSV</span>
        </a>

        <button
          type="button"
          class="btn btn--ghost btn--icon btn--sm"
          aria-label="Segarkan histori monitoring"
          title="Segarkan"
          disabled={loadingHistory}
          onclick={onRefresh}
        >
          <i class={loadingHistory ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
        </button>

        <button
          type="button"
          class="btn btn--ghost btn--icon btn--sm btn--danger"
          aria-label="Kosongkan semua catatan histori"
          title="Kosongkan Riwayat"
          onclick={onClearHistory}
        >
          <i class="ri-delete-bin-line" aria-hidden="true"></i>
        </button>
      </div>
    </div>

    <!-- Strip 6 Analitik Periode (Server-Side Avg / Peak / P95 / Total Transfer) -->
    <div class="mon-analytics-grid">
      <div class="mon-stat-pill">
        <span class="mon-stat-pill__label">CPU Avg / P95</span>
        <span class="mon-stat-pill__val">
          {summary ? `${summary.cpu_avg.toFixed(1)}% · ${summary.cpu_p95.toFixed(1)}%` : '-'}
        </span>
        <span class="mon-stat-pill__sub">
          Peak {summary ? `${summary.cpu_max.toFixed(1)}%` : '-'} · IOWait {summary ? `${summary.cpu_iowait_avg.toFixed(1)}%` : '-'}
        </span>
      </div>

      <div class="mon-stat-pill">
        <span class="mon-stat-pill__label">RAM Avg / Peak</span>
        <span class="mon-stat-pill__val">
          {summary ? `${summary.ram_avg_pct.toFixed(1)}% · ${summary.ram_max_pct.toFixed(1)}%` : '-'}
        </span>
        <span class="mon-stat-pill__sub">Ambang alert: {thresholds.alert_ram_pct}%</span>
      </div>

      <div class="mon-stat-pill">
        <span class="mon-stat-pill__label">Puncak Load 1m</span>
        <span class="mon-stat-pill__val">{summary ? summary.load_max.toFixed(2) : '-'}</span>
        <span class="mon-stat-pill__sub">Disk Root: {summary ? `${summary.disk_latest_pct.toFixed(1)}%` : '-'}</span>
      </div>

      <div class="mon-stat-pill">
        <span class="mon-stat-pill__label">Volume Trafik Masuk</span>
        <span class="mon-stat-pill__val">{summary ? formatBytes(summary.net_in_total_bytes) : '-'}</span>
        <span class="mon-stat-pill__sub">Peak {summary ? `${formatBytes(summary.net_in_peak)}/s` : '-'}</span>
      </div>

      <div class="mon-stat-pill">
        <span class="mon-stat-pill__label">Volume Trafik Keluar</span>
        <span class="mon-stat-pill__val">{summary ? formatBytes(summary.net_out_total_bytes) : '-'}</span>
        <span class="mon-stat-pill__sub">Peak {summary ? `${formatBytes(summary.net_out_peak)}/s` : '-'}</span>
      </div>

      <div class="mon-stat-pill">
        <span class="mon-stat-pill__label">Puncak Disk I/O</span>
        <span class="mon-stat-pill__val">
          {summary ? `${formatBytes(summary.disk_read_peak)}/s` : '-'}
        </span>
        <span class="mon-stat-pill__sub">Tulis {summary ? `${formatBytes(summary.disk_write_peak)}/s` : '-'}</span>
      </div>
    </div>

    <!-- Baris legenda warna kurva -->
    <div class="mon-legend-row">
      {#if chartMode === 'resource'}
        <span class="mon-legend-item"><span class="mon-dot mon-dot--info"></span> CPU Total %</span>
        <span class="mon-legend-item"><span class="mon-dot mon-dot--violet"></span> RAM Terpakai %</span>
        <span class="mon-legend-item"><span class="mon-dot mon-dot--warning"></span> CPU IOWait %</span>
        <span class="mon-legend-item"><span class="mon-dot mon-dot--danger"></span> Garis Ambang Alert</span>
      {:else if chartMode === 'load'}
        <span class="mon-legend-item"><span class="mon-dot mon-dot--info"></span> Load 1m</span>
        <span class="mon-legend-item"><span class="mon-dot mon-dot--violet"></span> Load 5m</span>
        <span class="mon-legend-item"><span class="mon-dot mon-dot--accent"></span> Load 15m</span>
      {:else if chartMode === 'network'}
        <span class="mon-legend-item"><span class="mon-dot mon-dot--accent"></span> Inbound (Download)</span>
        <span class="mon-legend-item"><span class="mon-dot mon-dot--warning"></span> Outbound (Upload)</span>
      {:else if chartMode === 'disk_io'}
        <span class="mon-legend-item"><span class="mon-dot mon-dot--success"></span> Disk Read /s</span>
        <span class="mon-legend-item"><span class="mon-dot mon-dot--warning"></span> Disk Write /s</span>
      {:else}
        <span class="mon-legend-item"><span class="mon-dot mon-dot--accent"></span> Kapasitas Disk Root (/) %</span>
        <span class="mon-legend-item"><span class="mon-dot mon-dot--danger"></span> Garis Ambang Alert</span>
      {/if}

      <span class="mon-sample-badge mono">
        {#if summary && summary.raw_count > summary.returned_count}
          Downsampled: {summary.raw_count} → {summary.returned_count} titik (peak-preserved)
        {:else}
          Total Sampel: {snapshots.length} titik
        {/if}
      </span>
    </div>

    <div class="mon-chart-box">
      <HistoryChart
        {snapshots}
        height={300}
        mode={chartMode}
        thresholdValue={activeThresholdValue}
        thresholdLabel={activeThresholdLabel}
      />
    </div>
  </div>
</div>
