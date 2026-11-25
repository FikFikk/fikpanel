<script lang="ts">
  import Chart from '../../lib/components/Chart.svelte';
  import LiveIOChart from './LiveIOChart.svelte';
  import { formatBytes } from '../../lib/api';
  import type { MetricPoint, SystemProcess } from '../../lib/types';

  interface Props {
    currentMetric: MetricPoint;
    history: MetricPoint[];
    topProcesses: SystemProcess[];
    loadingProcs: boolean;
    onRefreshProcs: () => void;
  }

  let { currentMetric, history, topProcesses, loadingProcs, onRefreshProcs }: Props = $props();

  // Fitur Freeze-Frame agar operator dapat membekukan lonjakan grafik live tanpa kehilangan data
  let paused = $state(false);
  let frozenMetric = $state<MetricPoint | null>(null);
  let frozenHistory = $state<MetricPoint[]>([]);

  function togglePause() {
    if (!paused) {
      frozenMetric = currentMetric;
      frozenHistory = [...history];
      paused = true;
    } else {
      paused = false;
      frozenMetric = null;
      frozenHistory = [];
    }
  }

  let displayMetric = $derived(paused && frozenMetric ? frozenMetric : currentMetric);
  let displayHistory = $derived(paused ? frozenHistory : history);

  let currentRAM = $derived(
    displayMetric.ram_total > 0
      ? (displayMetric.ram_used / displayMetric.ram_total) * 100
      : 0
  );

  let cpuUser = $derived(Math.min(100, Math.max(0, displayMetric.cpu_user || 0)));
  let cpuSys = $derived(Math.min(100 - cpuUser, Math.max(0, displayMetric.cpu_system || 0)));
  let cpuIO = $derived(Math.min(100 - cpuUser - cpuSys, Math.max(0, displayMetric.cpu_iowait || 0)));
  let cpuSteal = $derived(Math.min(100 - cpuUser - cpuSys - cpuIO, Math.max(0, displayMetric.cpu_steal || 0)));

  function coreFillClass(pct: number): string {
    if (pct >= 85) return 'mon-mini-bar__fill mon-mini-bar__fill--danger';
    if (pct >= 65) return 'mon-mini-bar__fill mon-mini-bar__fill--warn';
    return 'mon-mini-bar__fill';
  }
</script>

<div class="mon-pane">
  <!-- Grafik CPU & RAM Realtime SSE (Fixed currentCPU & currentRAM props + Freeze-Frame) -->
  <div class="card">
    <div class="card__head">
      <span class="card__title">
        <i class="ri-pulse-line" aria-hidden="true"></i>
        <span>Telemetri CPU &amp; RAM Realtime (SSE 1 Detik)</span>
      </span>
      <div class="mon-toolbar-group">
        <span class={paused ? 'badge badge--warning' : 'badge badge--success'}>
          {paused ? 'Tampilan Dibekukan' : 'Live 1s'}
        </span>
        <button type="button" class="btn btn--secondary btn--sm" onclick={togglePause}>
          <i class={paused ? 'ri-play-line' : 'ri-pause-line'} aria-hidden="true"></i>
          <span>{paused ? 'Lanjutkan Stream' : 'Jeda Grafik'}</span>
        </button>
      </div>
    </div>

    <div class="mon-chart-box">
      <Chart
        history={displayHistory}
        height={240}
        currentCPU={displayMetric.cpu || 0}
        {currentRAM}
      />
    </div>
  </div>

  <!-- Pecahan Beban CPU (User / System / IOWait / Steal) & Grid Per-Core -->
  <div class="mon-two-col">
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-cpu-line" aria-hidden="true"></i>
          <span>Pecahan Beban CPU (Kernel /proc/stat)</span>
        </span>
        <span class="badge badge--info mono">{(displayMetric.cpu || 0).toFixed(1)}%</span>
      </div>
      <div class="card__body stack">
        <div class="mon-cpu-stack" aria-label="Pecahan CPU User, System, IOWait, dan Steal">
          <div class="mon-cpu-seg mon-cpu-seg--user" style="--seg-user: {cpuUser}%"></div>
          <div class="mon-cpu-seg mon-cpu-seg--sys" style="--seg-sys: {cpuSys}%"></div>
          <div class="mon-cpu-seg mon-cpu-seg--iowait" style="--seg-iowait: {cpuIO}%"></div>
          <div class="mon-cpu-seg mon-cpu-seg--steal" style="--seg-steal: {cpuSteal}%"></div>
        </div>

        <div class="row wrap">
          <span class="mon-legend-item text-xs">
            <span class="mon-dot mon-dot--info"></span>
            <span>User: <strong class="mono">{cpuUser.toFixed(1)}%</strong></span>
          </span>
          <span class="mon-legend-item text-xs">
            <span class="mon-dot mon-dot--violet"></span>
            <span>System: <strong class="mono">{cpuSys.toFixed(1)}%</strong></span>
          </span>
          <span class="mon-legend-item text-xs">
            <span class="mon-dot mon-dot--warning"></span>
            <span>IOWait: <strong class="mono">{cpuIO.toFixed(1)}%</strong></span>
          </span>
          <span class="mon-legend-item text-xs">
            <span class="mon-dot mon-dot--danger"></span>
            <span>Steal (VPS): <strong class="mono">{cpuSteal.toFixed(1)}%</strong></span>
          </span>
        </div>

        {#if cpuIO >= 15}
          <div class="alert alert--warning" role="status">
            <i class="ri-hard-drive-2-line" aria-hidden="true"></i>
            <span>IOWait tinggi ({cpuIO.toFixed(1)}%): CPU banyak menunggu operasi baca/tulis disk atau database.</span>
          </div>
        {:else if cpuSteal >= 10}
          <div class="alert alert--danger" role="status">
            <i class="ri-cloud-windy-line" aria-hidden="true"></i>
            <span>CPU Steal tinggi ({cpuSteal.toFixed(1)}%): hypervisor VPS host sedang padat oleh tenant lain.</span>
          </div>
        {/if}

        {#if displayMetric.per_core && displayMetric.per_core.length > 0}
          <div class="mon-cores-grid">
            {#each displayMetric.per_core as pct, idx (idx)}
              <div class="mon-core-item">
                <div class="mon-core-top">
                  <span>CPU{idx}</span>
                  <strong>{pct.toFixed(0)}%</strong>
                </div>
                <div class="mon-mini-bar">
                  <div class={coreFillClass(pct)} style="--bar-w: {Math.min(100, Math.round(pct))}%"></div>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>

    <!-- Live Top 5 Konsumen CPU & RAM untuk korelasi lonjakan langsung -->
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-bar-chart-horizontal-line" aria-hidden="true"></i>
          <span>Top 5 Konsumen Resource Aktif</span>
        </span>
        <div class="mon-toolbar-group">
          <button
            type="button"
            class="btn btn--ghost btn--icon btn--sm"
            aria-label="Segarkan daftar konsumen proses"
            title="Segarkan"
            disabled={loadingProcs}
            onclick={onRefreshProcs}
          >
            <i class={loadingProcs ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
          </button>
          <a href="#/system" class="btn btn--secondary btn--sm">Buka Task Manager</a>
        </div>
      </div>

      <div class="table-wrap">
        <table class="table table--dense">
          <thead>
            <tr>
              <th>PID</th>
              <th>User</th>
              <th class="mon-col-right">CPU %</th>
              <th class="mon-col-right">RAM</th>
              <th>Perintah</th>
            </tr>
          </thead>
          <tbody>
            {#if topProcesses.length === 0}
              <tr>
                <td colspan="5">
                  <div class="empty-state">
                    <i class="ri-cpu-line" aria-hidden="true"></i>
                    <div class="empty-state__title">Data Proses Tidak Tersedia</div>
                    <p class="state__desc">Korelasi proses teratas aktif otomatis saat FikPanel berjalan pada server Linux.</p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each topProcesses as p (p.pid)}
                <tr>
                  <td class="mono"><strong>#{p.pid}</strong></td>
                  <td class="mono text-muted">{p.user}</td>
                  <td class="mono tabular mon-col-right"><strong>{p.cpu.toFixed(1)}%</strong></td>
                  <td class="mono tabular mon-col-right">{p.memory.toFixed(1)} MB</td>
                  <td class="mono truncate" title={p.command}>{p.command}</td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  </div>

  <!-- Grafik Live Network I/O & Live Disk I/O (60 detik) -->
  <div class="mon-two-col">
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-global-line" aria-hidden="true"></i>
          <span>Live Network I/O (60 Detik)</span>
        </span>
        <div class="mon-toolbar-group text-xs mono">
          <span class="mon-legend-item">
            <span class="mon-dot mon-dot--accent"></span> In {formatBytes(displayMetric.net_in || 0)}/s
          </span>
          <span class="mon-legend-item">
            <span class="mon-dot mon-dot--warning"></span> Out {formatBytes(displayMetric.net_out || 0)}/s
          </span>
        </div>
      </div>
      <div class="mon-chart-box">
        <LiveIOChart history={displayHistory} mode="network" height={185} />
      </div>
    </div>

    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-hard-drive-2-line" aria-hidden="true"></i>
          <span>Live Disk I/O Throughput (60 Detik)</span>
        </span>
        <div class="mon-toolbar-group text-xs mono">
          <span class="mon-legend-item">
            <span class="mon-dot mon-dot--success"></span> Baca {formatBytes(displayMetric.disk_read || 0)}/s
          </span>
          <span class="mon-legend-item">
            <span class="mon-dot mon-dot--warning"></span> Tulis {formatBytes(displayMetric.disk_write || 0)}/s
          </span>
        </div>
      </div>
      <div class="mon-chart-box">
        <LiveIOChart history={displayHistory} mode="disk_io" height={185} />
      </div>
    </div>
  </div>
</div>
