<script lang="ts">
  import Chart from '../lib/components/Chart.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import { formatBytes, apiFetch } from '../lib/api';
  import { notify } from '../lib/stores/toast.svelte';
  import type { MetricPoint, OverviewData, Service, DiscoveryResult } from '../lib/types';
  import '../styles/pages/home.css';

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
          ssl_cert_path: vh.ssl_cert_path,
          ssl_key_path: vh.ssl_key_path,
          ssl_issuer: vh.ssl_issuer,
          ssl_not_after: vh.ssl_not_after,
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

<div class="pg-home dash-container">
  <!-- Baris 1: Kartu Status (.stat-card: System Load & Uptime, CPU, RAM, Disk, Layanan, Ancaman) -->
  <div class="metrics-grid">
    <!-- Kartu 1: System Load & Uptime -->
    <div class="stat-card stat-card--metric">
      <div
        class="stat-card__icon {metricLevel(loadPercent, 80) === 'crit'
          ? 'stat-card__icon--danger'
          : metricLevel(loadPercent, 80) === 'warn'
            ? 'stat-card__icon--warning'
            : ''}"
      >
        <i class="ri-pulse-line"></i>
      </div>
      <div class="stat-card__body">
        <div class="stat-head">
          <span class="stat-card__label">System Load & Uptime</span>
          <span
            class="badge {metricLevel(loadPercent, 80) === 'crit'
              ? 'badge--danger'
              : metricLevel(loadPercent, 80) === 'warn'
                ? 'badge--warning'
                : 'badge--neutral'}"
          >
            {metricStatusText(loadPercent, 80)}
          </span>
        </div>
        <div class="stat-main">
          <span class="stat-card__value">{loadPercent.toFixed(0)}<small>%</small></span>
          <span class="metric-cap tabular">{currentMetric.load_avg.toFixed(2)} / {overview?.system?.cpu_cores || 4} Cores</span>
        </div>
        <div class="progress progress--sm" aria-hidden="true">
          <div
            class="progress__bar {metricLevel(loadPercent, 80) === 'crit'
              ? 'progress__bar--danger'
              : metricLevel(loadPercent, 80) === 'warn'
                ? 'progress__bar--warning'
                : ''}"
            style="--w: {Math.min(Math.max(loadPercent, 0), 100)}%;"
          ></div>
        </div>
        <span class="stat-card__sub">
          Kapasitas {overview?.system?.cpu_cores || 4} vCPU &middot; Uptime {overview?.system?.uptime_sec
            ? `${Math.floor(overview.system.uptime_sec / 3600)}j ${Math.floor((overview.system.uptime_sec % 3600) / 60)}m`
            : '—'}
        </span>
      </div>
    </div>

    <!-- Kartu 2: CPU Usage -->
    <div class="stat-card stat-card--metric">
      <div
        class="stat-card__icon {metricLevel(currentMetric.cpu, 85) === 'crit'
          ? 'stat-card__icon--danger'
          : metricLevel(currentMetric.cpu, 85) === 'warn'
            ? 'stat-card__icon--warning'
            : ''}"
      >
        <i class="ri-cpu-line"></i>
      </div>
      <div class="stat-card__body">
        <div class="stat-head">
          <span class="stat-card__label">CPU Usage</span>
          <span
            class="badge {metricLevel(currentMetric.cpu, 85) === 'crit'
              ? 'badge--danger'
              : metricLevel(currentMetric.cpu, 85) === 'warn'
                ? 'badge--warning'
                : 'badge--neutral'}"
          >
            {metricStatusText(currentMetric.cpu, 85)}
          </span>
        </div>
        <div class="stat-main">
          <span class="stat-card__value">{currentMetric.cpu.toFixed(1)}<small>%</small></span>
          <span class="metric-cap tabular">{currentMetric.cpu.toFixed(1)}% Active</span>
        </div>
        <div class="progress progress--sm" aria-hidden="true">
          <div
            class="progress__bar {metricLevel(currentMetric.cpu, 85) === 'crit'
              ? 'progress__bar--danger'
              : metricLevel(currentMetric.cpu, 85) === 'warn'
                ? 'progress__bar--warning'
                : ''}"
            style="--w: {Math.min(Math.max(currentMetric.cpu, 0), 100)}%;"
          ></div>
        </div>
        <span class="stat-card__sub">Alokasi prosesor &middot; {overview?.system?.cpu_cores || 4} Cores</span>
      </div>
    </div>

    <!-- Kartu 3: Memory (RAM) -->
    <div class="stat-card stat-card--metric">
      <div
        class="stat-card__icon {metricLevel(ramPercent, 85) === 'crit'
          ? 'stat-card__icon--danger'
          : metricLevel(ramPercent, 85) === 'warn'
            ? 'stat-card__icon--warning'
            : 'stat-card__icon--info'}"
      >
        <i class="ri-database-2-line"></i>
      </div>
      <div class="stat-card__body">
        <div class="stat-head">
          <span class="stat-card__label">Memory (RAM)</span>
          <span
            class="badge {metricLevel(ramPercent, 85) === 'crit'
              ? 'badge--danger'
              : metricLevel(ramPercent, 85) === 'warn'
                ? 'badge--warning'
                : 'badge--neutral'}"
          >
            {metricStatusText(ramPercent, 85)}
          </span>
        </div>
        <div class="stat-main">
          <span class="stat-card__value">{ramPercent.toFixed(0)}<small>%</small></span>
          <span class="metric-cap tabular">{formatBytes(currentMetric.ram_used)} / {formatBytes(currentMetric.ram_total)}</span>
        </div>
        <div class="progress progress--sm" aria-hidden="true">
          <div
            class="progress__bar {metricLevel(ramPercent, 85) === 'crit'
              ? 'progress__bar--danger'
              : metricLevel(ramPercent, 85) === 'warn'
                ? 'progress__bar--warning'
                : 'progress__bar--info'}"
            style="--w: {Math.min(Math.max(ramPercent, 0), 100)}%;"
          ></div>
        </div>
        <span class="stat-card__sub">Tersedia {formatBytes(freeRAM)}</span>
      </div>
    </div>

    <!-- Kartu 4: Disk Storage -->
    <div class="stat-card stat-card--metric">
      <div
        class="stat-card__icon {metricLevel(diskPercent, 85) === 'crit'
          ? 'stat-card__icon--danger'
          : metricLevel(diskPercent, 85) === 'warn'
            ? 'stat-card__icon--warning'
            : 'stat-card__icon--violet'}"
      >
        <i class="ri-hard-drive-2-line"></i>
      </div>
      <div class="stat-card__body">
        <div class="stat-head">
          <span class="stat-card__label">Disk Storage (/)</span>
          <span
            class="badge {metricLevel(diskPercent, 85) === 'crit'
              ? 'badge--danger'
              : metricLevel(diskPercent, 85) === 'warn'
                ? 'badge--warning'
                : 'badge--neutral'}"
          >
            {metricStatusText(diskPercent, 85)}
          </span>
        </div>
        <div class="stat-main">
          <span class="stat-card__value">{diskPercent.toFixed(0)}<small>%</small></span>
          <span class="metric-cap tabular">{formatBytes(currentMetric.disk_used)} / {formatBytes(currentMetric.disk_total)}</span>
        </div>
        <div class="progress progress--sm" aria-hidden="true">
          <div
            class="progress__bar {metricLevel(diskPercent, 85) === 'crit'
              ? 'progress__bar--danger'
              : metricLevel(diskPercent, 85) === 'warn'
                ? 'progress__bar--warning'
                : ''}"
            style="--w: {Math.min(Math.max(diskPercent, 0), 100)}%;"
          ></div>
        </div>
        <span class="stat-card__sub">Ruang kosong {formatBytes(freeDisk)}</span>
      </div>
    </div>

    <!-- Kartu 5: Layanan Sistem -->
    <div class="stat-card stat-card--metric">
      <div class="stat-card__icon stat-card__icon--info">
        <i class="ri-server-line"></i>
      </div>
      <div class="stat-card__body">
        <div class="stat-head">
          <span class="stat-card__label">Layanan & Website</span>
          <span class="badge badge--success">{overview?.counts?.services_active || 0} Berjalan</span>
        </div>
        <div class="stat-main">
          <span class="stat-card__value">{overview?.counts?.services_active || 0}<small>/{overview?.counts?.services_total || 0}</small></span>
          <span class="metric-cap tabular">{overview?.counts?.sites_running || 0}/{overview?.counts?.sites_total || 0} Situs</span>
        </div>
        <span class="stat-card__sub">Systemd units & virtual host aktif</span>
      </div>
    </div>

    <!-- Kartu 6: Status Keamanan & Ancaman -->
    <div class="stat-card stat-card--metric">
      <div class="stat-card__icon {(overview?.counts?.security_risk || 0) > 0 ? 'stat-card__icon--warning' : ''}">
        <i class="ri-shield-check-line"></i>
      </div>
      <div class="stat-card__body">
        <div class="stat-head">
          <span class="stat-card__label">Status Ancaman</span>
          <span class="badge {(overview?.counts?.security_risk || 0) > 0 ? 'badge--warning' : 'badge--success'}">
            {(overview?.counts?.security_risk || 0) > 0 ? 'Waspada' : 'Proteksi Aktif'}
          </span>
        </div>
        <div class="stat-main">
          <span class="stat-card__value">{overview?.counts?.security_risk || 0}</span>
          <span class="metric-cap tabular">Peringatan</span>
        </div>
        <span class="stat-card__sub">Keamanan host & WAF terpantau</span>
      </div>
    </div>
  </div>

  <!-- Baris 2: Ringkasan Infrastruktur & Aksi Cepat -->
  <div class="card bento-panel">
    <div class="panel-header-row">
      <div class="header-left">
        <h3 class="panel-heading">Ringkasan Infrastruktur</h3>
      </div>

      <!-- Tombol Aksi Cepat -->
      <div class="quick-actions">
        <button class="btn btn--sm btn--secondary" onclick={openDiscoveryModal}>
          <i class="ri-radar-line"></i>
          <span>Pindai Host Server</span>
        </button>
        <a href="#/website" class="btn btn--sm btn--primary">
          <i class="ri-add-line"></i>
          <span>Tambah Situs</span>
        </a>
        <a href="#/services" class="btn btn--sm btn--secondary">
          <i class="ri-server-line"></i>
          <span>Kelola Service</span>
        </a>
        <a href="#/terminal" class="btn btn--sm btn--secondary">
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
            <span class="stat-muted">{overview?.counts?.security_risk || 0} Peringatan</span>
          </div>
        </div>
      </a>
    </div>
  </div>

  <!-- Baris 3: Kartu Monitoring Realtime CPU & RAM -->
  <div class="card bento-panel">
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
  <div class="card bento-panel">
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

    {#if services.length === 0}
      <div class="state state--empty">
        <i class="ri-server-line"></i>
        <div class="state__title">Belum Ada Layanan Terdaftar</div>
        <div class="state__desc">Tidak ada systemd unit yang dapat ditampilkan pada daftar layanan utama saat ini.</div>
      </div>
    {:else}
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>Unit</th>
              <th>Label</th>
              <th>Status</th>
              <th>Memory</th>
              <th class="text-right">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#each services.slice(0, 5) as s}
              <tr>
                <td class="mono font-bold">{s.unit}</td>
                <td class="text-muted">{s.label || '—'}</td>
                <td>
                  <span class="badge {s.active_state === 'active' ? 'badge--success' : 'badge--danger'}">
                    <span class="dot {s.active_state === 'active' ? 'dot--success' : 'dot--danger'}"></span>
                    {s.active_state} ({s.sub_state})
                  </span>
                </td>
                <td class="mono tabular">{formatBytes(s.memory_current)}</td>
                <td class="text-right">
                  <div class="svc-actions">
                    <button
                      class="btn btn--sm btn--ghost"
                      onclick={() => onSvcAction(s.unit, 'restart')}
                    >
                      Restart
                    </button>
                    <button
                      class="btn btn--sm {s.active_state === 'active' ? 'btn--danger' : 'btn--primary'}"
                      onclick={() => onSvcAction(s.unit, s.active_state === 'active' ? 'stop' : 'start')}
                    >
                      {s.active_state === 'active' ? 'Stop' : 'Start'}
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>

  <!-- Modal Host Infrastructure Auto-Discovery -->
  <Modal
    open={showDiscoveryModal}
    title="Host Infrastructure Auto-Discovery"
    size="xl"
    onclose={() => (showDiscoveryModal = false)}
  >
    <p class="discovery-sub">Deteksi dinamis Nginx virtual host, Docker compose, PM2, dan Kubernetes di server</p>

    <div class="discovery-filter-row">
      <input
        type="text"
        class="input grow"
        bind:value={customRootsInput}
        placeholder="Direktori kustom tambahan (contoh: /home/fikfikk/projects, /var/www)"
      />
      <button class="btn btn--secondary" onclick={runHostScan} disabled={scanningHost}>
        {#if scanningHost}
          <span class="spinner"></span> Memindai...
        {:else}
          <i class="ri-refresh-line"></i> Pindai Ulang
        {/if}
      </button>
    </div>

    {#if scanningHost && !discoveryResult}
      <div class="state state--loading">
        <span class="spinner"></span>
        <div class="state__desc">Menelusuri Nginx virtual hosts, proyek docker, dan proses PM2...</div>
      </div>
    {:else if discoveryResult}
      <div class="discovery-results">
        <!-- Ringkasan K8s -->
        <div class="discovery-summary-banner">
          <div class="summary-item">
            <span class="sum-label">Kubernetes Host</span>
            <span class="sum-val">
              {#if discoveryResult.kubernetes_detected}
                <span class="badge badge--success"><i class="ri-checkbox-circle-fill"></i> {discoveryResult.kubernetes_type.toUpperCase()} Terdeteksi</span>
              {:else}
                <span class="badge badge--neutral">Belum Terdeteksi</span>
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
            <div class="table-wrap">
              <table class="table table--dense">
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
                      <td class="mono">{vh.root_dir || (vh.proxy_pass ? `Proxy: ${vh.proxy_pass}` : '-')}</td>
                      <td>{vh.port}</td>
                      <td>{vh.ssl ? 'Ya' : 'Tidak'}</td>
                      <td>
                        {#if vh.is_adopted}
                          <span class="badge badge--success">Terdaftar</span>
                        {:else}
                          <span class="badge badge--info">Host Asli (Bisa Diadopsi)</span>
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
            <div class="table-wrap">
              <table class="table table--dense">
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
                      <td class="mono">{dp.working_dir}</td>
                      <td class="mono">{dp.compose_path}</td>
                      <td>
                        <span class="badge {dp.running ? 'badge--success' : 'badge--neutral'}">
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
            <div class="table-wrap">
              <table class="table table--dense">
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
                      <td class="mono">{app.script}</td>
                      <td class="mono">{app.working_dir}</td>
                      <td>
                        <span class="badge {app.status === 'online' ? 'badge--success' : 'badge--neutral'}">
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

    {#snippet footer()}
      <button class="btn btn--secondary" onclick={() => (showDiscoveryModal = false)}>Tutup</button>
      {#if discoveryResult && discoveryResult.nginx_vhosts.some((v) => !v.is_adopted)}
        <button class="btn btn--primary" onclick={adoptDiscoveredSites} disabled={adopting}>
          {#if adopting}
            <span class="spinner"></span> Mengadopsi...
          {:else}
            <i class="ri-download-2-line"></i> Adopsi Seluruh Situs ke FikPanel
          {/if}
        </button>
      {/if}
    {/snippet}
  </Modal>
</div>