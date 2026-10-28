<script lang="ts">
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

  // Kalkulasi persentase metrik untuk kartu resource
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

  // Evaluasi status metrik secara seragam tanpa warna pelangi berlebihan
  function metricLevel(pct: number, warnAt = 80): 'normal' | 'warn' | 'crit' {
    if (pct >= 92) return 'crit';
    if (pct >= warnAt) return 'warn';
    return 'normal';
  }

  function metricStatusText(pct: number, warnAt = 80): string {
    if (pct >= 92) return 'Kritis';
    if (pct >= warnAt) return 'Waspada';
    return 'Normal';
  }

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
  <!-- Baris 1: 4 Kartu Metrik Arsitektural (Tanpa Cincin Neon / AI Slop) -->
  <div class="metrics-grid">
    <!-- Kartu 1: System Load -->
    <div class="metric-card">
      <div class="metric-top">
        <span class="metric-label"><i class="ri-pulse-line"></i> System Load</span>
        <span class="metric-status {metricLevel(loadPercent, 80)}">{metricStatusText(loadPercent, 80)}</span>
      </div>
      <div class="metric-main">
        <span class="metric-value tabular">{loadPercent.toFixed(0)}<small>%</small></span>
        <span class="metric-cap tabular">{currentMetric.load_avg.toFixed(2)} / {overview?.system?.cpu_cores || 4} Cores</span>
      </div>
      <div class="metric-track" aria-hidden="true">
        <div
          class="metric-fill {metricLevel(loadPercent, 80)}"
          style="width: {Math.min(Math.max(loadPercent, 0), 100)}%;"
        ></div>
      </div>
      <div class="metric-foot">
        <span>Kapasitas inti</span>
        <span class="tabular">{overview?.system?.cpu_cores || 4} vCPU</span>
      </div>
    </div>

    <!-- Kartu 2: CPU Usage -->
    <div class="metric-card">
      <div class="metric-top">
        <span class="metric-label"><i class="ri-cpu-line"></i> CPU Usage</span>
        <span class="metric-status {metricLevel(currentMetric.cpu, 85)}">{metricStatusText(currentMetric.cpu, 85)}</span>
      </div>
      <div class="metric-main">
        <span class="metric-value tabular">{currentMetric.cpu.toFixed(1)}<small>%</small></span>
        <span class="metric-cap tabular">{currentMetric.cpu.toFixed(1)}% Active</span>
      </div>
      <div class="metric-track" aria-hidden="true">
        <div
          class="metric-fill {metricLevel(currentMetric.cpu, 85)}"
          style="width: {Math.min(Math.max(currentMetric.cpu, 0), 100)}%;"
        ></div>
      </div>
      <div class="metric-foot">
        <span>Alokasi prosesor</span>
        <span class="tabular">{overview?.system?.cpu_cores || 4} Cores</span>
      </div>
    </div>

    <!-- Kartu 3: Memory (RAM) -->
    <div class="metric-card">
      <div class="metric-top">
        <span class="metric-label"><i class="ri-database-2-line"></i> Memory (RAM)</span>
        <span class="metric-status {metricLevel(ramPercent, 85)}">{metricStatusText(ramPercent, 85)}</span>
      </div>
      <div class="metric-main">
        <span class="metric-value tabular">{ramPercent.toFixed(0)}<small>%</small></span>
        <span class="metric-cap tabular">{formatBytes(currentMetric.ram_used)} / {formatBytes(currentMetric.ram_total)}</span>
      </div>
      <div class="metric-track" aria-hidden="true">
        <div
          class="metric-fill {metricLevel(ramPercent, 85)}"
          style="width: {Math.min(Math.max(ramPercent, 0), 100)}%;"
        ></div>
      </div>
      <div class="metric-foot">
        <span>Tersedia</span>
        <span class="tabular">{formatBytes(freeRAM)}</span>
      </div>
    </div>

    <!-- Kartu 4: Disk Storage -->
    <div class="metric-card">
      <div class="metric-top">
        <span class="metric-label"><i class="ri-hard-drive-2-line"></i> Disk Storage (/)</span>
        <span class="metric-status {metricLevel(diskPercent, 85)}">{metricStatusText(diskPercent, 85)}</span>
      </div>
      <div class="metric-main">
        <span class="metric-value tabular">{diskPercent.toFixed(0)}<small>%</small></span>
        <span class="metric-cap tabular">{formatBytes(currentMetric.disk_used)} / {formatBytes(currentMetric.disk_total)}</span>
      </div>
      <div class="metric-track" aria-hidden="true">
        <div
          class="metric-fill {metricLevel(diskPercent, 85)}"
          style="width: {Math.min(Math.max(diskPercent, 0), 100)}%;"
        ></div>
      </div>
      <div class="metric-foot">
        <span>Ruang kosong</span>
        <span class="tabular">{formatBytes(freeDisk)}</span>
      </div>
    </div>
  </div>

  <!-- Baris 2: Ringkasan Infrastruktur & Aksi Cepat -->
  <div class="bento-panel">
    <div class="panel-header-row">
      <div class="header-left">
        <h3 class="panel-heading">Ringkasan Infrastruktur</h3>
      </div>

      <!-- Quick Action Buttons -->
      <div class="quick-actions">
        <button class="btn-action" onclick={openDiscoveryModal}>
          <i class="ri-radar-line"></i>
          <span>Pindai Host Server</span>
        </button>
        <a href="#/website" class="btn-action btn-action-primary">
          <i class="ri-add-line"></i>
          <span>Tambah Situs</span>
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

    <!-- 4 Sub-Kartu Ringkas & Monokromatik -->
    <div class="bento-subcards">
      <!-- Sub-Card 1: Website (Sites) -->
      <a href="#/website" class="subcard">
        <div class="subcard-top">
          <div class="subcard-icon">
            <i class="ri-global-line"></i>
          </div>
          <span class="subcard-arrow"><i class="ri-arrow-right-up-line"></i></span>
        </div>
        <div class="subcard-body">
          <div class="subcard-title">Website</div>
          <div class="subcard-stats">
            <span class="stat-highlight">{overview?.counts?.sites_running || 0} Aktif</span>
            <span class="stat-sep">&middot;</span>
            <span class="stat-muted">{overview?.counts?.sites_total || 0} Total</span>
          </div>
        </div>
      </a>

      <!-- Sub-Card 2: Services (Systemd) -->
      <a href="#/services" class="subcard">
        <div class="subcard-top">
          <div class="subcard-icon">
            <i class="ri-server-line"></i>
          </div>
          <span class="subcard-arrow"><i class="ri-arrow-right-up-line"></i></span>
        </div>
        <div class="subcard-body">
          <div class="subcard-title">Services (Systemd)</div>
          <div class="subcard-stats">
            <span class="stat-highlight">{overview?.counts?.services_active || 0} Berjalan</span>
            <span class="stat-sep">&middot;</span>
            <span class="stat-muted">{overview?.counts?.services_total || 0} Total</span>
          </div>
        </div>
      </a>

      <!-- Sub-Card 3: Database Internal -->
      <a href="#/databases" class="subcard">
        <div class="subcard-top">
          <div class="subcard-icon">
            <i class="ri-database-2-line"></i>
          </div>
          <span class="subcard-arrow"><i class="ri-arrow-right-up-line"></i></span>
        </div>
        <div class="subcard-body">
          <div class="subcard-title">Database Engine</div>
          <div class="subcard-stats">
            <span class="stat-highlight">SQLite WAL</span>
            <span class="stat-sep">&middot;</span>
            <span class="stat-muted">panel.db</span>
          </div>
        </div>
      </a>

      <!-- Sub-Card 4: Status Keamanan -->
      <a href="#/security" class="subcard">
        <div class="subcard-top">
          <div class="subcard-icon">
            <i class="ri-shield-check-line"></i>
          </div>
          <span class="subcard-arrow"><i class="ri-arrow-right-up-line"></i></span>
        </div>
        <div class="subcard-body">
          <div class="subcard-title">Keamanan & WAF</div>
          <div class="subcard-stats">
            <span class="stat-highlight">Proteksi Aktif</span>
            <span class="stat-sep">&middot;</span>
            <span class="stat-muted">0 Peringatan</span>
          </div>
        </div>
      </a>
    </div>
  </div>

  <!-- Baris 3: Kartu Monitoring Realtime CPU & RAM -->
  <div class="bento-panel">
    <div class="panel-header-row">
      <div>
        <h3 class="panel-heading">Beban CPU & Memori</h3>
        <p class="panel-subtext">Riwayat penggunaan sumber daya sistem 60 detik terakhir</p>
      </div>

      <!-- Indikator Trafik Jaringan Bersih -->
      <div class="traffic-pills">
        <div class="traffic-pill" title="Lalu lintas jaringan masuk (Inbound)">
          <i class="ri-arrow-down-line icon-in"></i>
          <span>{formatBytes(currentMetric.net_in)}/s</span>
        </div>
        <div class="traffic-pill" title="Lalu lintas jaringan keluar (Outbound)">
          <i class="ri-arrow-up-line icon-out"></i>
          <span>{formatBytes(currentMetric.net_out)}/s</span>
        </div>
      </div>
    </div>

    <!-- Komponen Chart dengan Crosshair & Interactive Tooltip -->
    <Chart
      {history}
      height={210}
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

  /* Baris 1: Grid 4 Kartu Metrik Arsitektural */
  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 0.85rem;
  }

  .metric-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 1rem 1.15rem;
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    transition: border-color 0.15s ease;
  }
  .metric-card:hover {
    border-color: var(--border-strong);
  }

  .metric-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }
  .metric-label {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.76rem;
    font-weight: 500;
    color: var(--text-muted);
  }
  .metric-label i {
    font-size: 0.88rem;
    color: var(--text-subtle);
  }
  .metric-status {
    font-size: 0.68rem;
    font-weight: 600;
    padding: 0.12rem 0.45rem;
    border-radius: 4px;
    background: var(--bg-raised);
    color: var(--text-muted);
    border: 1px solid var(--border);
  }
  .metric-status.warn {
    color: var(--warning);
    border-color: color-mix(in srgb, var(--warning) 30%, transparent);
    background: color-mix(in srgb, var(--warning) 8%, transparent);
  }
  .metric-status.crit {
    color: var(--danger-strong);
    border-color: color-mix(in srgb, var(--danger-strong) 30%, transparent);
    background: color-mix(in srgb, var(--danger-strong) 8%, transparent);
  }

  .metric-main {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.5rem;
    margin-top: 0.1rem;
  }
  .metric-value {
    font-size: 1.55rem;
    font-weight: 700;
    color: var(--text);
    letter-spacing: -0.03em;
    line-height: 1.1;
  }
  .metric-value small {
    font-size: 0.88rem;
    font-weight: 500;
    color: var(--text-muted);
    margin-left: 1px;
  }
  .metric-cap {
    font-size: 0.74rem;
    font-family: 'Fira Code', monospace;
    color: var(--text-secondary);
  }

  .metric-track {
    width: 100%;
    height: 4px;
    background: var(--bg-raised);
    border-radius: 999px;
    overflow: hidden;
  }
  .metric-fill {
    height: 100%;
    border-radius: 999px;
    background: var(--accent);
    transition: width 0.35s cubic-bezier(0.22, 1, 0.36, 1), background-color 0.2s ease;
  }
  .metric-fill.warn {
    background: var(--warning);
  }
  .metric-fill.crit {
    background: var(--danger-strong);
  }

  .metric-foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.71rem;
    color: var(--text-subtle);
  }

  /* Bento Card Surface */
  .bento-panel {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 1.1rem 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.95rem;
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
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--text);
    letter-spacing: -0.01em;
    margin: 0;
  }
  .panel-subtext {
    font-size: 0.73rem;
    color: var(--text-muted);
    margin: 0.15rem 0 0;
  }

  /* Quick Actions Buttons */
  .quick-actions {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    flex-wrap: wrap;
  }
  .btn-action {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.34rem 0.68rem;
    border-radius: 6px;
    font-size: 0.73rem;
    font-weight: 500;
    text-decoration: none;
    background: var(--bg-raised);
    color: var(--text-secondary);
    border: 1px solid var(--border);
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .btn-action:hover {
    background: var(--bg-hover);
    color: var(--text);
    border-color: var(--border-strong);
  }
  .btn-action-primary {
    background: var(--accent);
    color: var(--text-on-accent);
    border-color: var(--accent);
    font-weight: 600;
  }
  .btn-action-primary:hover {
    background: var(--accent-strong);
    color: var(--text-on-accent);
    border-color: var(--accent-strong);
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
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.85rem 0.95rem;
    text-decoration: none;
    transition: border-color 0.15s ease, background 0.15s ease;
  }
  .subcard:hover {
    border-color: var(--border-strong);
    background: var(--bg-hover);
  }
  .subcard-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.55rem;
  }
  .subcard-icon {
    width: 26px;
    height: 26px;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.88rem;
    color: var(--text-secondary);
    background: var(--bg-surface);
    border: 1px solid var(--border);
  }
  .subcard-arrow {
    color: var(--text-subtle);
    font-size: 0.82rem;
    transition: transform 0.15s ease, color 0.15s ease;
  }
  .subcard:hover .subcard-arrow {
    transform: translate(1px, -1px);
    color: var(--text);
  }
  .subcard-title {
    font-size: 0.73rem;
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
    color: var(--text);
  }
  .stat-sep {
    color: var(--text-disabled);
  }
  .stat-muted {
    color: var(--text-subtle);
    font-size: 0.73rem;
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
    padding: 0.22rem 0.55rem;
    border-radius: 6px;
    background: var(--bg-raised);
    color: var(--text-secondary);
    border: 1px solid var(--border);
  }
  .traffic-pill .icon-in {
    color: var(--accent);
  }
  .traffic-pill .icon-out {
    color: var(--text-muted);
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