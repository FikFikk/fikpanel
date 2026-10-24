<script lang="ts">
  import Gauge from '../lib/components/Gauge.svelte';
  import Chart from '../lib/components/Chart.svelte';
  import { formatBytes } from '../lib/api';
  import type { MetricPoint, OverviewData, Service } from '../lib/types';

  interface Props {
    overview: OverviewData | null;
    currentMetric: MetricPoint;
    history: MetricPoint[];
    services: Service[];
    onSvcAction: (unit: string, action: string) => void;
  }

  let { overview, currentMetric, history, services, onSvcAction }: Props = $props();

  // Kalkulasi persentase metrik untuk gauge
  let ramPercent = $derived(
    currentMetric.ram_total > 0
      ? (currentMetric.ram_used / currentMetric.ram_total) * 100
      : 0
  );

  let diskPercent = $derived(
    currentMetric.disk_total > 0
      ? (currentMetric.disk_used / currentMetric.disk_total) * 100
      : 0
  );

  let loadPercent = $derived(
    overview?.system?.cpu_cores
      ? Math.min((currentMetric.load_avg / overview.system.cpu_cores) * 100, 100)
      : Math.min(currentMetric.load_avg * 25, 100)
  );

  // Sisa kapasitas Free RAM & Free Disk
  let freeRAM = $derived(
    currentMetric.ram_total > currentMetric.ram_used
      ? currentMetric.ram_total - currentMetric.ram_used
      : 0
  );

  let freeDisk = $derived(
    currentMetric.disk_total > currentMetric.disk_used
      ? currentMetric.disk_total - currentMetric.disk_used
      : 0
  );
</script>

<div class="dash-container">
  <!-- Baris 1: 4 Kartu Circular Gauge Padat, Hidup & Reaktif -->
  <div class="gauges-grid">
    <!-- Load Status (Hijau Emerald) -->
    <Gauge
      percent={loadPercent}
      title="Load Status"
      subtitle="{currentMetric.load_avg.toFixed(2)} / {overview?.system?.cpu_cores || 4} Cores"
      detailValue="{overview?.system?.cpu_cores || 4} Cores"
      color="#10b981"
      icon="ri-dashboard-2-line"
      warningThreshold={80}
    />

    <!-- CPU Usage (Sky Blue) -->
    <Gauge
      percent={currentMetric.cpu}
      title="CPU Usage"
      subtitle="{currentMetric.cpu.toFixed(1)}% Active"
      detailValue="{overview?.system?.cpu_cores || 4} Cores"
      color="#38bdf8"
      icon="ri-cpu-line"
      warningThreshold={85}
      showMiniBar={true}
    />

    <!-- RAM Usage (Indigo) -->
    <Gauge
      percent={ramPercent}
      title="Memory (RAM)"
      subtitle="{formatBytes(currentMetric.ram_used)} / {formatBytes(currentMetric.ram_total)}"
      detailValue="Sisa: {formatBytes(freeRAM)}"
      color="#818cf8"
      icon="ri-database-2-line"
      warningThreshold={85}
      showMiniBar={true}
    />

    <!-- Disk Usage (Amber/Oranye) -->
    <Gauge
      percent={diskPercent}
      title="Disk Storage (/)"
      subtitle="{formatBytes(currentMetric.disk_used)} / {formatBytes(currentMetric.disk_total)}"
      detailValue="Sisa: {formatBytes(freeDisk)}"
      color="#f59e0b"
      icon="ri-hard-drive-2-line"
      warningThreshold={85}
      showMiniBar={true}
    />
  </div>

  <!-- Baris 2: Interactive Bento Overview & Quick Actions -->
  <div class="bento-panel">
    <div class="panel-header-row">
      <div class="header-left">
        <h3 class="panel-heading">Overview Sistem</h3>
        <span class="badge-system">
          <span class="dot-system"></span>
          Sistem Terkelola
        </span>
      </div>

      <!-- Quick Action Buttons -->
      <div class="quick-actions">
        <a href="#/website" class="btn-action btn-action-primary">
          <i class="ri-add-line"></i>
          <span>Tambah Site</span>
        </a>
        <a href="#/services" class="btn-action">
          <i class="ri-server-line"></i>
          <span>Kelola Service</span>
        </a>
        <a href="#/terminal" class="btn-action">
          <i class="ri-terminal-line"></i>
          <span>Buka Terminal</span>
        </a>
      </div>
    </div>

    <!-- 4 Sub-Kartu Interaktif Bento -->
    <div class="bento-subcards">
      <!-- Sub-Card 1: Website (Sites) -->
      <a href="#/website" class="subcard subcard-sites">
        <div class="subcard-top">
          <div class="subcard-icon icon-emerald">
            <i class="ri-global-line"></i>
          </div>
          <span class="subcard-arrow"><i class="ri-arrow-right-line"></i></span>
        </div>
        <div class="subcard-body">
          <div class="subcard-title">Website (Sites)</div>
          <div class="subcard-stats">
            <span class="stat-highlight text-emerald">{overview?.counts?.sites_running || 0} Aktif</span>
            <span class="stat-sep">&middot;</span>
            <span class="stat-muted">{overview?.counts?.sites_total || 0} Total</span>
          </div>
        </div>
      </a>

      <!-- Sub-Card 2: Services (Systemd) -->
      <a href="#/services" class="subcard subcard-services">
        <div class="subcard-top">
          <div class="subcard-icon icon-indigo">
            <i class="ri-server-line"></i>
          </div>
          <span class="subcard-arrow"><i class="ri-arrow-right-line"></i></span>
        </div>
        <div class="subcard-body">
          <div class="subcard-title">Services (Systemd)</div>
          <div class="subcard-stats">
            <span class="pulse-emerald"></span>
            <span class="stat-highlight text-indigo">{overview?.counts?.services_active || 0} Berjalan</span>
            <span class="stat-sep">&middot;</span>
            <span class="stat-muted">{overview?.counts?.services_total || 0} Total</span>
          </div>
        </div>
      </a>

      <!-- Sub-Card 3: Database Internal -->
      <a href="#/settings" class="subcard subcard-db">
        <div class="subcard-top">
          <div class="subcard-icon icon-cyan">
            <i class="ri-database-2-line"></i>
          </div>
          <span class="subcard-arrow"><i class="ri-arrow-right-line"></i></span>
        </div>
        <div class="subcard-body">
          <div class="subcard-title">Database Internal</div>
          <div class="subcard-stats">
            <span class="stat-highlight text-cyan">SQLite (WAL)</span>
            <span class="stat-sep">&middot;</span>
            <span class="stat-muted">1 DB (panel.db)</span>
          </div>
        </div>
      </a>

      <!-- Sub-Card 4: Status Keamanan -->
      <a href="#/security" class="subcard subcard-security">
        <div class="subcard-top">
          <div class="subcard-icon icon-green">
            <i class="ri-shield-check-line"></i>
          </div>
          <span class="subcard-arrow"><i class="ri-arrow-right-line"></i></span>
        </div>
        <div class="subcard-body">
          <div class="subcard-title">Status Keamanan</div>
          <div class="subcard-stats">
            <span class="stat-highlight text-emerald">Risiko: 0</span>
            <span class="stat-sep">&middot;</span>
            <span class="stat-muted">Proteksi Penuh</span>
          </div>
        </div>
      </a>
    </div>
  </div>

  <!-- Baris 3: Kartu Monitoring Realtime CPU & RAM -->
  <div class="bento-panel">
    <div class="panel-header-row">
      <div>
        <h3 class="panel-heading">Monitoring Realtime (CPU & RAM)</h3>
        <p class="panel-subtext">
          <span class="pulse-live-inline"></span>
          Streaming SSE 1 detik &middot; Ring Buffer 300 titik
        </p>
      </div>

      <!-- Badge Kecepatan Jaringan -->
      <div class="traffic-pills">
        <div class="traffic-pill pill-in">
          <i class="ri-arrow-down-line"></i>
          <span>{formatBytes(currentMetric.net_in)}/s</span>
        </div>
        <div class="traffic-pill pill-out">
          <i class="ri-arrow-up-line"></i>
          <span>{formatBytes(currentMetric.net_out)}/s</span>
        </div>
      </div>
    </div>

    <!-- Komponen Chart dengan Crosshair & Interactive Tooltip -->
    <Chart
      {history}
      height={220}
      currentCPU={currentMetric.cpu}
      currentRAM={ramPercent}
    />
  </div>

  <!-- Baris 4: Layanan Sistem Prioritas -->
  <div class="bento-panel">
    <div class="panel-header-row">
      <div>
        <h3 class="panel-heading">Layanan Sistem Utama</h3>
        <p class="panel-subtext">Systemd units terdaftar dalam allowlist yang dapat dikontrol</p>
      </div>
      <a href="#/services" class="link-manage">
        <span>Kelola Semua Service</span>
        <i class="ri-arrow-right-line"></i>
      </a>
    </div>

    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Unit</th>
            <th>Label</th>
            <th>Status</th>
            <th>Memory</th>
            <th style="text-align: right;">Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#each services.slice(0, 5) as s}
            <tr>
              <td class="mono font-bold text-white">{s.unit}</td>
              <td class="text-muted">{s.label || '—'}</td>
              <td>
                <span class="badge {s.active_state === 'active' ? 'badge-ok' : 'badge-fail'}">
                  <span class="dot-status {s.active_state === 'active' ? 'dot-green' : 'dot-red'}"></span>
                  {s.active_state} ({s.sub_state})
                </span>
              </td>
              <td class="mono tabular">{formatBytes(s.memory_current)}</td>
              <td style="text-align: right;">
                <button
                  class="btn-sm btn-ghost"
                  onclick={() => onSvcAction(s.unit, 'restart')}
                >
                  Restart
                </button>
                <button
                  class="btn-sm {s.active_state === 'active' ? 'btn-danger' : 'btn-accent'}"
                  onclick={() => onSvcAction(s.unit, s.active_state === 'active' ? 'stop' : 'start')}
                >
                  {s.active_state === 'active' ? 'Stop' : 'Start'}
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
</div>

<style>
  .dash-container {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  /* Baris 1: Grid 4 Gauges */
  .gauges-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 1rem;
  }

  /* Bento Card Surface */
  .bento-panel {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 1.15rem 1.35rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    transition: border-color 0.15s ease;
  }
  .bento-panel:hover {
    border-color: var(--border);
  }

  /* Header Row di Panel */
  .panel-header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.75rem;
  }
  .header-left {
    display: flex;
    align-items: center;
    gap: 0.65rem;
  }
  .panel-heading {
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--text);
    letter-spacing: -0.01em;
  }
  .panel-subtext {
    font-size: 0.73rem;
    color: var(--text-muted);
    margin-top: 0.15rem;
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .pulse-live-inline {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: var(--accent);
  }

  .badge-system {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.68rem;
    font-weight: 600;
    color: var(--accent);
    background: color-mix(in srgb, var(--accent) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--accent) 20%, transparent);
    padding: 0.15rem 0.55rem;
    border-radius: 999px;
  }
  .dot-system {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: var(--accent);
  }

  /* Quick Actions Buttons */
  .quick-actions {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }
  .btn-action {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.32rem 0.65rem;
    border-radius: 6px;
    font-size: 0.72rem;
    font-weight: 500;
    text-decoration: none;
    background: var(--bg-raised);
    color: var(--text-secondary);
    border: 1px solid var(--border);
    transition: all 0.15s ease;
  }
  .btn-action:hover {
    background: var(--bg-hover);
    color: var(--text);
    border-color: var(--border-strong);
  }
  .btn-action-primary {
    background: color-mix(in srgb, var(--accent) 12%, transparent);
    color: var(--accent);
    border-color: color-mix(in srgb, var(--accent) 25%, transparent);
  }
  .btn-action-primary:hover {
    background: var(--accent);
    color: var(--text-on-accent);
  }

  /* 4 Bento Sub-Cards */
  .bento-subcards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 0.75rem;
  }
  .subcard {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    padding: 0.85rem 0.95rem;
    text-decoration: none;
    transition: border-color 0.15s ease, background 0.15s ease;
  }
  .subcard:hover {
    border-color: var(--border);
    background: var(--bg-raised);
  }
  .subcard-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.65rem;
  }
  .subcard-icon {
    width: 26px;
    height: 26px;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.9rem;
    background: var(--bg-raised);
    border: 1px solid var(--border);
  }
  .icon-emerald {
    color: var(--accent);
  }
  .icon-indigo {
    color: var(--violet);
  }
  .icon-cyan {
    color: var(--info);
  }
  .icon-green {
    color: var(--accent);
  }
  .subcard-arrow {
    color: var(--text-subtle);
    font-size: 0.8rem;
    transition: transform 0.15s ease, color 0.15s ease;
  }
  .subcard:hover .subcard-arrow {
    transform: translateX(2px);
    color: var(--text-secondary);
  }
  .subcard-title {
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-muted);
    margin-bottom: 0.2rem;
  }
  .subcard-stats {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.8rem;
    font-family: 'Fira Code', monospace;
    font-variant-numeric: tabular-nums;
  }
  .stat-highlight {
    font-weight: 600;
  }
  .stat-sep {
    color: var(--text-disabled);
  }
  .stat-muted {
    color: var(--text-subtle);
    font-size: 0.73rem;
  }
  .text-emerald {
    color: var(--accent);
  }
  .text-indigo {
    color: var(--violet);
  }
  .text-cyan {
    color: var(--info);
  }
  .pulse-emerald {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: var(--accent);
  }

  /* Traffic Pills */
  .traffic-pills {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }
  .traffic-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.72rem;
    font-family: 'Fira Code', monospace;
    font-variant-numeric: tabular-nums;
    padding: 0.2rem 0.55rem;
    border-radius: 6px;
    border: 1px solid transparent;
  }
  .pill-in {
    background: color-mix(in srgb, var(--accent) 8%, transparent);
    color: var(--accent);
    border-color: color-mix(in srgb, var(--accent) 20%, transparent);
  }
  .pill-out {
    background: color-mix(in srgb, var(--info) 8%, transparent);
    color: var(--info);
    border-color: color-mix(in srgb, var(--info) 20%, transparent);
  }

  /* Layanan Prioritas Table */
  .link-manage {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--accent);
    text-decoration: none;
    transition: color 0.15s;
  }
  .link-manage:hover {
    color: var(--accent-strong);
  }
  .table-wrap {
    overflow-x: auto;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }
  th {
    text-align: left;
    padding: 0.65rem 0.75rem;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-subtle);
    border-bottom: 1px solid color-mix(in srgb, var(--text) 7%, transparent);
  }
  td {
    padding: 0.75rem;
    border-bottom: 1px solid color-mix(in srgb, var(--text) 4%, transparent);
  }
  .mono {
    font-family: 'Fira Code', monospace;
  }
  .tabular {
    font-variant-numeric: tabular-nums;
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.2rem 0.55rem;
    border-radius: 5px;
    font-size: 0.72rem;
    font-weight: 600;
  }
  .badge-ok {
    background: color-mix(in srgb, var(--accent) 12%, transparent);
    color: var(--accent);
  }
  .badge-fail {
    background: color-mix(in srgb, var(--danger-strong) 12%, transparent);
    color: var(--danger-strong);
  }
  .dot-status {
    width: 5px;
    height: 5px;
    border-radius: 50%;
  }
  .dot-green {
    background: var(--accent);
  }
  .dot-red {
    background: var(--danger-strong);
  }
  .btn-sm {
    padding: 0.25rem 0.65rem;
    border-radius: 6px;
    font-size: 0.72rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
    transition: all 0.15s;
  }
  .btn-ghost {
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid color-mix(in srgb, var(--text) 10%, transparent);
  }
  .btn-ghost:hover {
    background: color-mix(in srgb, var(--text) 6%, transparent);
    color: var(--text);
  }
  .btn-accent {
    background: var(--accent);
    color: var(--text-on-accent);
  }
  .btn-danger {
    background: color-mix(in srgb, var(--danger-strong) 15%, transparent);
    color: var(--danger-strong);
    border: 1px solid color-mix(in srgb, var(--danger-strong) 25%, transparent);
  }
</style>