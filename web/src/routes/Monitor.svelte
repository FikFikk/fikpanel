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
        <span class="l-item"><span class="l-dot" style="background:#818cf8;"></span> CPU %</span>
        <span class="l-item"><span class="l-dot" style="background:#06b6d4;"></span> RAM %</span>
      </div>
    </div>
    <Chart {history} height={260} />
  </div>

  <!-- Detail Metrik Cards -->
  <div class="metrics-grid">
    <div class="metric-card">
      <div class="m-label">CPU Core Usage</div>
      <div class="m-val" style="color: #818cf8;">{currentMetric.cpu.toFixed(1)}%</div>
      <div class="m-sub">Load Avg: {currentMetric.load_avg.toFixed(2)}</div>
    </div>
    <div class="metric-card">
      <div class="m-label">Physical Memory (RAM)</div>
      <div class="m-val" style="color: #06b6d4;">
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
      <div class="m-val" style="color: #22c55e;">
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
    font-size: 1.25rem;
    font-weight: 700;
    color: #f3f4f6;
  }
  .subtitle {
    font-size: 0.8rem;
    color: #9ca3af;
    margin-top: 0.2rem;
  }
  .badge-live {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    background: rgba(34, 197, 94, 0.12);
    border: 1px solid rgba(34, 197, 94, 0.25);
    color: #22c55e;
    font-size: 0.75rem;
    font-weight: 600;
    padding: 0.35rem 0.75rem;
    border-radius: 999px;
  }
  .live-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: #22c55e;
    box-shadow: 0 0 8px #22c55e;
    animation: pulse 2s infinite;
  }
  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.4;
    }
  }
  .panel {
    background: rgba(28, 29, 34, 0.75);
    border: 1px solid rgba(255, 255, 255, 0.07);
    border-radius: 12px;
    padding: 1.5rem;
  }
  .panel-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.25rem;
    flex-wrap: wrap;
    gap: 0.75rem;
  }
  .panel-title {
    font-size: 1rem;
    font-weight: 700;
    color: #f3f4f6;
  }
  .panel-sub {
    font-size: 0.75rem;
    color: #9ca3af;
    margin-top: 0.2rem;
  }
  .legend {
    display: flex;
    gap: 1rem;
    font-size: 0.78rem;
    color: #d1d5db;
  }
  .l-item {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }
  .l-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 1.25rem;
  }
  .metric-card {
    background: rgba(28, 29, 34, 0.75);
    border: 1px solid rgba(255, 255, 255, 0.07);
    border-radius: 12px;
    padding: 1.25rem;
  }
  .m-label {
    font-size: 0.72rem;
    font-weight: 600;
    text-transform: uppercase;
    color: #9ca3af;
    letter-spacing: 0.05em;
  }
  .m-val {
    font-size: 1.6rem;
    font-weight: 700;
    font-family: 'Fira Code', monospace;
    margin: 0.5rem 0 0.25rem;
  }
  .m-sub {
    font-size: 0.75rem;
    color: #6b7280;
    font-family: 'Fira Code', monospace;
  }
</style>