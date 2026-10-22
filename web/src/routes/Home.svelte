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

    <!-- CPU Usage (Indigo/Ungu) -->
    <Gauge
      percent={currentMetric.cpu}
      title="CPU Usage"
      subtitle="{currentMetric.cpu.toFixed(1)}% Active"
      detailValue="{overview?.system?.cpu_cores || 4} Cores"
      color="#6366f1"
      icon="ri-cpu-line"
      warningThreshold={85}
      showMiniBar={true}
    />

    <!-- RAM Usage (Cyan) -->
    <Gauge
      percent={ramPercent}
      title="Memory (RAM)"
      subtitle="{formatBytes(currentMetric.ram_used)} / {formatBytes(currentMetric.ram_total)}"
      detailValue="Sisa: {formatBytes(freeRAM)}"
      color="#06b6d4"
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
    background: rgba(17, 20, 29, 0.85);
    backdrop-filter: blur(16px);
    border: 1px solid rgba(255, 255, 255, 0.07);
    box-shadow: inset 0 1px 0 0 rgba(255, 255, 255, 0.08), 0 4px 20px rgba(0, 0, 0, 0.35);
    border-radius: 14px;
    padding: 1.25rem 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    transition: border-color 0.2s;
  }
  .bento-panel:hover {
    border-color: rgba(255, 255, 255, 0.12);
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
    font-size: 1rem;
    font-weight: 700;
    color: #f8fafc;
    letter-spacing: -0.01em;
  }
  .panel-subtext {
    font-size: 0.75rem;
    color: #94a3b8;
    margin-top: 0.15rem;
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .pulse-live-inline {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #10b981;
    box-shadow: 0 0 6px #10b981;
  }

  .badge-system {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.7rem;
    font-weight: 600;
    color: #10b981;
    background: rgba(16, 185, 129, 0.1);
    border: 1px solid rgba(16, 185, 129, 0.25);
    padding: 0.2rem 0.6rem;
    border-radius: 999px;
  }
  .dot-system {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: #10b981;
  }

  /* Quick Actions Buttons */
  .quick-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .btn-action {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.35rem 0.75rem;
    border-radius: 8px;
    font-size: 0.75rem;
    font-weight: 600;
    text-decoration: none;
    background: rgba(255, 255, 255, 0.04);
    color: #cbd5e1;
    border: 1px solid rgba(255, 255, 255, 0.08);
    transition: all 0.15s;
  }
  .btn-action:hover {
    background: rgba(255, 255, 255, 0.08);
    color: #f8fafc;
    border-color: rgba(255, 255, 255, 0.15);
    transform: translateY(-1px);
  }
  .btn-action-primary {
    background: rgba(16, 185, 129, 0.15);
    color: #10b981;
    border-color: rgba(16, 185, 129, 0.3);
  }
  .btn-action-primary:hover {
    background: #10b981;
    color: #090d16;
  }

  /* 4 Bento Sub-Cards */
  .bento-subcards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 0.85rem;
  }
  .subcard {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    background: rgba(0, 0, 0, 0.25);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 10px;
    padding: 0.9rem 1rem;
    text-decoration: none;
    transition: all 0.2s cubic-bezier(0.2, 0.8, 0.2, 1);
  }
  .subcard:hover {
    transform: translateY(-2px);
    border-color: rgba(255, 255, 255, 0.15);
    background: rgba(255, 255, 255, 0.03);
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.3);
  }
  .subcard-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.65rem;
  }
  .subcard-icon {
    width: 28px;
    height: 28px;
    border-radius: 7px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.95rem;
  }
  .icon-emerald {
    background: rgba(16, 185, 129, 0.12);
    color: #10b981;
    border: 1px solid rgba(16, 185, 129, 0.2);
  }
  .icon-indigo {
    background: rgba(99, 102, 241, 0.12);
    color: #818cf8;
    border: 1px solid rgba(99, 102, 241, 0.2);
  }
  .icon-cyan {
    background: rgba(6, 182, 212, 0.12);
    color: #06b6d4;
    border: 1px solid rgba(6, 182, 212, 0.2);
  }
  .icon-green {
    background: rgba(34, 197, 94, 0.12);
    color: #22c55e;
    border: 1px solid rgba(34, 197, 94, 0.2);
  }
  .subcard-arrow {
    color: #64748b;
    font-size: 0.85rem;
    transition: transform 0.2s, color 0.2s;
  }
  .subcard:hover .subcard-arrow {
    transform: translateX(3px);
    color: #f8fafc;
  }
  .subcard-title {
    font-size: 0.78rem;
    font-weight: 600;
    color: #94a3b8;
    margin-bottom: 0.2rem;
  }
  .subcard-stats {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.82rem;
    font-family: 'Fira Code', monospace;
    font-variant-numeric: tabular-nums;
  }
  .stat-highlight {
    font-weight: 700;
  }
  .stat-sep {
    color: #475569;
  }
  .stat-muted {
    color: #64748b;
    font-size: 0.75rem;
  }
  .text-emerald {
    color: #10b981;
  }
  .text-indigo {
    color: #818cf8;
  }
  .text-cyan {
    color: #06b6d4;
  }
  .pulse-emerald {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #10b981;
    box-shadow: 0 0 6px #10b981;
  }

  /* Traffic Pills */
  .traffic-pills {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .traffic-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.72rem;
    font-family: 'Fira Code', monospace;
    font-variant-numeric: tabular-nums;
    padding: 0.25rem 0.65rem;
    border-radius: 7px;
    border: 1px solid transparent;
  }
  .pill-in {
    background: rgba(16, 185, 129, 0.1);
    color: #10b981;
    border-color: rgba(16, 185, 129, 0.25);
  }
  .pill-out {
    background: rgba(6, 182, 212, 0.1);
    color: #06b6d4;
    border-color: rgba(6, 182, 212, 0.25);
  }

  /* Layanan Prioritas Table */
  .link-manage {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.78rem;
    font-weight: 600;
    color: #10b981;
    text-decoration: none;
    transition: color 0.15s;
  }
  .link-manage:hover {
    color: #34d399;
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
    color: #64748b;
    border-bottom: 1px solid rgba(255, 255, 255, 0.07);
  }
  td {
    padding: 0.75rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
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
    background: rgba(16, 185, 129, 0.12);
    color: #10b981;
  }
  .badge-fail {
    background: rgba(239, 68, 68, 0.12);
    color: #ef4444;
  }
  .dot-status {
    width: 5px;
    height: 5px;
    border-radius: 50%;
  }
  .dot-green {
    background: #10b981;
  }
  .dot-red {
    background: #ef4444;
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
    color: #cbd5e1;
    border: 1px solid rgba(255, 255, 255, 0.1);
  }
  .btn-ghost:hover {
    background: rgba(255, 255, 255, 0.06);
    color: #f8fafc;
  }
  .btn-accent {
    background: #10b981;
    color: #090d16;
  }
  .btn-danger {
    background: rgba(239, 68, 68, 0.15);
    color: #ef4444;
    border: 1px solid rgba(239, 68, 68, 0.25);
  }
</style>