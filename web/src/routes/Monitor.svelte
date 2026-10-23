<script lang="ts">
  import Chart from '../lib/components/Chart.svelte';
  import { formatBytes } from '../lib/api';
  import type { MetricPoint } from '../lib/types';

  interface Props {
    currentMetric: MetricPoint;
    history: MetricPoint[];
  }

  let { currentMetric, history }: Props = $props();
</script>

<div class="monitor-container">
  <div class="head-bar">
    <div>
      <h2 class="title">Monitor Performa Server</h2>
      <p class="subtitle">Visualisasi metrik /proc & kernel via Server-Sent Events (SSE) realtime</p>
    </div>
    <div class="badge-live">
      <span class="live-dot"></span>
      <span>Live Streaming 1 Detik</span>
    </div>
  </div>

  <div class="panel">
    <div class="panel-head">
      <div>
        <h3 class="panel-title">Grafik Utilisasi CPU & RAM</h3>
        <p class="panel-sub">Buffer 60 detik terakhir &middot; Auto-scaling canvas render</p>
      </div>
      <div class="legend">
        <span class="l-item"><span class="l-dot" style="background:#38bdf8;"></span> CPU %</span>
        <span class="l-item"><span class="l-dot" style="background:#818cf8;"></span> RAM %</span>
      </div>
    </div>
    <Chart {history} height={260} />
  </div>

  <!-- Detail Metrik Cards -->
  <div class="metrics-grid">
    <div class="metric-card">
      <div class="m-label">CPU Core Usage</div>
      <div class="m-val" style="color: #38bdf8;">{currentMetric.cpu.toFixed(1)}%</div>
      <div class="m-sub">Load Avg: {currentMetric.load_avg.toFixed(2)}</div>
    </div>
    <div class="metric-card">
      <div class="m-label">Physical Memory (RAM)</div>
      <div class="m-val" style="color: #818cf8;">
        {currentMetric.ram_total > 0
          ? ((currentMetric.ram_used / currentMetric.ram_total) * 100).toFixed(1)
          : 0}%
      </div>
      <div class="m-sub">
        {formatBytes(currentMetric.ram_used)} / {formatBytes(currentMetric.ram_total)}
      </div>
    </div>
    <div class="metric-card">
      <div class="m-label">Disk Storage (/)</div>
      <div class="m-val" style="color: #10b981;">
        {currentMetric.disk_total > 0
          ? ((currentMetric.disk_used / currentMetric.disk_total) * 100).toFixed(1)
          : 0}%
      </div>
      <div class="m-sub">
        {formatBytes(currentMetric.disk_used)} / {formatBytes(currentMetric.disk_total)}
      </div>
    </div>
    <div class="metric-card">
      <div class="m-label">Network Throughput</div>
      <div class="m-val" style="color: #f59e0b; font-size: 1.15rem;">
        ↓ {formatBytes(currentMetric.net_in)}/s
      </div>
      <div class="m-sub">
        ↑ {formatBytes(currentMetric.net_out)}/s
      </div>
    </div>
  </div>
</div>

<style>
  .monitor-container {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .head-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
  }
  .title {
    font-size: 1.15rem;
    font-weight: 700;
    color: #f1f5f9;
    letter-spacing: -0.01em;
  }
  .subtitle {
    font-size: 0.75rem;
    color: #8b95a5;
    margin-top: 0.15rem;
  }
  .badge-live {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    background: rgba(16, 185, 129, 0.08);
    border: 1px solid rgba(16, 185, 129, 0.2);
    color: #10b981;
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.25rem 0.65rem;
    border-radius: 999px;
  }
  .live-dot {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: #10b981;
  }
  .panel {
    background: #10131b;
    border: 1px solid #1c2230;
    border-radius: 12px;
    padding: 1.25rem 1.35rem;
  }
  .panel-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
    flex-wrap: wrap;
    gap: 0.75rem;
  }
  .panel-title {
    font-size: 0.95rem;
    font-weight: 600;
    color: #f1f5f9;
  }
  .panel-sub {
    font-size: 0.73rem;
    color: #8b95a5;
    margin-top: 0.15rem;
  }
  .legend {
    display: flex;
    gap: 0.85rem;
    font-size: 0.75rem;
    color: #cbd5e1;
  }
  .l-item {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }
  .l-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 0.85rem;
  }
  .metric-card {
    background: #10131b;
    border: 1px solid #1c2230;
    border-radius: 12px;
    padding: 1.15rem 1.25rem;
  }
  .m-label {
    font-size: 0.7rem;
    font-weight: 600;
    text-transform: uppercase;
    color: #8b95a5;
    letter-spacing: 0.05em;
  }
  .m-val {
    font-size: 1.45rem;
    font-weight: 700;
    font-family: 'Fira Code', monospace;
    font-variant-numeric: tabular-nums;
    margin: 0.4rem 0 0.2rem;
  }
  .m-sub {
    font-size: 0.72rem;
    color: #64748b;
    font-family: 'Fira Code', monospace;
    font-variant-numeric: tabular-nums;
  }
</style>