<script lang="ts">
  import Gauge from '../lib/components/Gauge.svelte';
  import Chart from '../lib/components/Chart.svelte';
  import { formatBytes, apiFetch } from '../lib/api';
  import { notify } from '../lib/stores/toast.svelte';
  import type { MetricPoint, OverviewData, Service, DiscoveryResult } from '../lib/types';

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

  // State Host Discovery & Adoption
  let showDiscoveryModal = $state(false);
  let scanningHost = $state(false);
  let adopting = $state(false);
  let discoveryResult = $state<DiscoveryResult | null>(null);
  let customRootsInput = $state('');

  async function openDiscoveryModal() {
    showDiscoveryModal = true;
    if (!discoveryResult) {
      await runHostScan();
    }
  }

  async function runHostScan() {
    scanningHost = true;
    try {
      const roots = customRootsInput
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);

      const res = await apiFetch<{ ok: boolean; result: DiscoveryResult }>('/api/discovery/scan', {
        method: 'POST',
        body: JSON.stringify({ custom_roots: roots }),
      });
      if (res && res.result) {
        discoveryResult = res.result;
        notify('Pemindaian infrastruktur server selesai', 'success');
      }
    } catch (err: any) {
      notify(err?.message || 'Gagal memindai server', 'error');
    } finally {
      scanningHost = false;
    }
  }

  async function adoptDiscoveredSites() {
    if (!discoveryResult || discoveryResult.nginx_vhosts.length === 0) return;
    adopting = true;
    try {
      const sitesToAdopt = discoveryResult.nginx_vhosts.map((vh) => {
        const domains = vh.server_name.split(/\s+/).filter(Boolean);
        return {
          primary_domain: domains[0] || vh.server_name,
          aliases: domains.slice(1),
          document_root: vh.root_dir,
          has_php: vh.config_file.includes('php') || vh.port === 80 || vh.port === 443,
          proxy_pass: vh.proxy_pass,
          has_ssl: vh.ssl,
        };
      });

      const res = await apiFetch<{ ok: boolean; adopted_sites: number; message: string }>('/api/discovery/adopt', {
        method: 'POST',
        body: JSON.stringify({ sites: sitesToAdopt }),
      });
      if (res && res.ok) {
        notify(res.message || 'Situs berhasil diadopsi ke FikPanel', 'success');
        showDiscoveryModal = false;
        window.location.hash = '#/website';
      }
    } catch (err: any) {
      notify(err?.message || 'Gagal mengadopsi situs', 'error');
    } finally {
      adopting = false;
    }
  }
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
        <button class="btn-action btn-action-radar" onclick={openDiscoveryModal}>
          <i class="ri-radar-line"></i>
          <span>Pindai Host Server</span>
        </button>
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

<!-- Modal Host Infrastructure Auto-Discovery -->
{#if showDiscoveryModal}
  <div class="modal-backdrop">
    <div class="modal-card modal-lg">
      <div class="modal-header">
        <div class="modal-title-group">
          <h3><i class="ri-radar-line"></i> Host Infrastructure Auto-Discovery</h3>
          <p class="modal-sub">Deteksi dinamis Nginx virtual host, Docker compose, PM2, dan Kubernetes di server</p>
        </div>
        <button class="close-btn" onclick={() => (showDiscoveryModal = false)}>
          <i class="ri-close-line"></i>
        </button>
      </div>

      <div class="modal-body">
        <div class="discovery-filter-row">
          <input
            type="text"
            class="input-roots"
            bind:value={customRootsInput}
            placeholder="Direktori kustom tambahan (contoh: /home/fikfikk/projects, /var/www)"
          />
          <button class="btn btn-secondary" onclick={runHostScan} disabled={scanningHost}>
            {#if scanningHost}
              <i class="ri-loader-4-line spin"></i> Memindai...
            {:else}
              <i class="ri-refresh-line"></i> Pindai Ulang
            {/if}
          </button>
        </div>

        {#if scanningHost && !discoveryResult}
          <div class="discovery-loading">
            <i class="ri-loader-4-line spin"></i>
            <p>Menelusuri Nginx virtual hosts, proyek docker, dan proses PM2...</p>
          </div>
        {:else if discoveryResult}
          <div class="discovery-results">
            <!-- Ringkasan K8s -->
            <div class="discovery-summary-banner">
              <div class="summary-item">
                <span class="sum-label">Kubernetes Host</span>
                <span class="sum-val">
                  {#if discoveryResult.kubernetes_detected}
                    <span class="badge-k8s active"><i class="ri-checkbox-circle-fill"></i> {discoveryResult.kubernetes_type.toUpperCase()} Terdeteksi</span>
                  {:else}
                    <span class="badge-k8s inactive">Belum Terdeteksi</span>
                  {/if}
                </span>
              </div>
              <div class="summary-item">
                <span class="sum-label">Virtual Hosts</span>
                <span class="sum-val">{discoveryResult.nginx_vhosts.length} vhost</span>
              </div>
              <div class="summary-item">
                <span class="sum-label">Docker Projects</span>
                <span class="sum-val">{discoveryResult.docker_projects.length} stack</span>
              </div>
              <div class="summary-item">
                <span class="sum-label">PM2 Node.js</span>
                <span class="sum-val">{discoveryResult.pm2_apps.length} apps</span>
              </div>
            </div>

            <!-- Virtual Hosts Nginx -->
            <div class="disc-section">
              <h4><i class="ri-global-line"></i> Virtual Host Nginx Ditemukan ({discoveryResult.nginx_vhosts.length})</h4>
              {#if discoveryResult.nginx_vhosts.length === 0}
                <p class="disc-empty">Tidak ada vhost Nginx terpisah yang ditemukan di /etc/nginx/.</p>
              {:else}
                <div class="disc-table-wrapper">
                  <table class="disc-table">
                    <thead>
                      <tr>
                        <th>Domain / Server Name</th>
                        <th>Web Root Directory</th>
                        <th>Port</th>
                        <th>SSL</th>
                        <th>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each discoveryResult.nginx_vhosts as vh}
                        <tr>
                          <td class="font-bold">{vh.server_name}</td>
                          <td class="code-font">{vh.root_dir || (vh.proxy_pass ? `Proxy: ${vh.proxy_pass}` : '-')}</td>
                          <td>{vh.port}</td>
                          <td>{vh.ssl ? 'Ya' : 'Tidak'}</td>
                          <td>
                            {#if vh.is_adopted}
                              <span class="tag-adopted">Terdaftar</span>
                            {:else}
                              <span class="tag-unadopted">Host Asli (Bisa Diadopsi)</span>
                            {/if}
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              {/if}
            </div>

            <!-- Docker Compose Projects -->
            <div class="disc-section">
              <h4><i class="ri-box-3-line"></i> Docker Compose Stacks ({discoveryResult.docker_projects.length})</h4>
              {#if discoveryResult.docker_projects.length === 0}
                <p class="disc-empty">Tidak ada berkas compose yang ditemukan di direktori yang dipindai.</p>
              {:else}
                <div class="disc-table-wrapper">
                  <table class="disc-table">
                    <thead>
                      <tr>
                        <th>Nama Proyek</th>
                        <th>Working Directory</th>
                        <th>Berkas Compose</th>
                        <th>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each discoveryResult.docker_projects as dp}
                        <tr>
                          <td class="font-bold">{dp.project_name}</td>
                          <td class="code-font">{dp.working_dir}</td>
                          <td class="code-font">{dp.compose_path}</td>
                          <td>
                            <span class="tag-status" class:running={dp.running}>
                              {dp.running ? 'Berjalan' : 'Tersedia'}
                            </span>
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              {/if}
            </div>

            <!-- PM2 Apps -->
            <div class="disc-section">
              <h4><i class="ri-nodejs-line"></i> Aplikasi PM2 Node.js ({discoveryResult.pm2_apps.length})</h4>
              {#if discoveryResult.pm2_apps.length === 0}
                <p class="disc-empty">Tidak ada proses PM2 aktif di host.</p>
              {:else}
                <div class="disc-table-wrapper">
                  <table class="disc-table">
                    <thead>
                      <tr>
                        <th>Nama Aplikasi</th>
                        <th>Script</th>
                        <th>Working Directory</th>
                        <th>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each discoveryResult.pm2_apps as app}
                        <tr>
                          <td class="font-bold">{app.name}</td>
                          <td class="code-font">{app.script}</td>
                          <td class="code-font">{app.working_dir}</td>
                          <td>
                            <span class="tag-status" class:running={app.status === 'online'}>
                              {app.status}
                            </span>
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              {/if}
            </div>
          </div>
        {/if}
      </div>

      <div class="modal-footer">
        <button class="btn btn-secondary" onclick={() => (showDiscoveryModal = false)}>Tutup</button>
        {#if discoveryResult && discoveryResult.nginx_vhosts.some((v) => !v.is_adopted)}
          <button class="btn btn-primary" onclick={adoptDiscoveredSites} disabled={adopting}>
            {#if adopting}
              <i class="ri-loader-4-line spin"></i> Mengadopsi...
            {:else}
              <i class="ri-download-2-line"></i> Adopsi Seluruh Situs ke FikPanel
            {/if}
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}

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

  .btn-action-radar {
    background: color-mix(in srgb, var(--primary) 12%, transparent);
    color: var(--primary);
    border-color: color-mix(in srgb, var(--primary) 25%, transparent);
  }
  .btn-action-radar:hover {
    background: var(--primary);
    color: var(--text-on-accent);
  }

  /* Modal Discovery Styling */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .modal-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    width: 90%;
    max-width: 500px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .modal-lg {
    max-width: 900px;
    max-height: 85vh;
  }
  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--border);
  }
  .modal-title-group h3 {
    margin: 0;
    font-size: 1.15rem;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .modal-sub {
    margin: 0.25rem 0 0;
    font-size: 0.8rem;
    color: var(--text-muted);
  }
  .close-btn {
    background: transparent;
    border: none;
    font-size: 1.25rem;
    color: var(--text-muted);
    cursor: pointer;
  }
  .modal-body {
    padding: 1.25rem;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .discovery-filter-row {
    display: flex;
    gap: 0.75rem;
  }
  .input-roots {
    flex: 1;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.5rem 0.75rem;
    color: var(--text);
    font-size: 0.85rem;
  }
  .discovery-loading {
    padding: 3rem 1rem;
    text-align: center;
    color: var(--text-muted);
    font-size: 1rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.75rem;
  }
  .discovery-summary-banner {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 0.75rem;
    background: var(--bg-inset);
    padding: 1rem;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
  }
  .summary-item {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .sum-label {
    font-size: 0.75rem;
    color: var(--text-muted);
  }
  .sum-val {
    font-size: 1.1rem;
    font-weight: 700;
    color: var(--text);
  }
  .badge-k8s {
    font-size: 0.8rem;
    font-weight: 600;
  }
  .badge-k8s.active {
    color: var(--success);
  }
  .badge-k8s.inactive {
    color: var(--text-muted);
  }
  .disc-section {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .disc-section h4 {
    margin: 0;
    font-size: 0.95rem;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .disc-empty {
    margin: 0;
    font-size: 0.82rem;
    color: var(--text-muted);
    font-style: italic;
  }
  .disc-table-wrapper {
    overflow-x: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
  }
  .disc-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }
  .disc-table th {
    padding: 0.5rem 0.65rem;
    background: var(--bg-inset);
    border-bottom: 1px solid var(--border);
    color: var(--text-muted);
    text-align: left;
    font-size: 0.75rem;
    text-transform: uppercase;
  }
  .disc-table td {
    padding: 0.6rem 0.65rem;
    border-bottom: 1px solid var(--border);
    color: var(--text);
  }
  .font-bold {
    font-weight: 600;
  }
  .code-font {
    font-family: var(--font-mono);
    font-size: 0.78rem;
  }
  .tag-adopted {
    background: color-mix(in srgb, var(--success) 15%, transparent);
    color: var(--success);
    font-size: 0.72rem;
    padding: 0.15rem 0.45rem;
    border-radius: var(--radius-sm);
    font-weight: 600;
  }
  .tag-unadopted {
    background: color-mix(in srgb, var(--primary) 15%, transparent);
    color: var(--primary);
    font-size: 0.72rem;
    padding: 0.15rem 0.45rem;
    border-radius: var(--radius-sm);
    font-weight: 600;
  }
  .tag-status {
    background: var(--bg-inset);
    color: var(--text-muted);
    font-size: 0.72rem;
    padding: 0.15rem 0.45rem;
    border-radius: var(--radius-sm);
    font-weight: 600;
  }
  .tag-status.running {
    background: color-mix(in srgb, var(--success) 15%, transparent);
    color: var(--success);
  }
  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
    padding: 1rem 1.25rem;
    border-top: 1px solid var(--border);
    background: var(--bg-inset);
  }
  .spin {
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>