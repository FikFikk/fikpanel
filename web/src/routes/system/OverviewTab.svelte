<script lang="ts">
  import { formatBytes } from '../../lib/api';
  import type { HostOverview } from '../../lib/types';

  interface Props {
    overview: HostOverview | null;
    onOpenRebootModal: () => void;
    onJumpTab: (tab: 'processes' | 'memory' | 'cleaner' | 'diagnostics') => void;
  }

  let { overview, onOpenRebootModal, onJumpTab }: Props = $props();

  function formatUptime(sec: number): string {
    if (!sec || sec <= 0) return '-';
    const d = Math.floor(sec / 86400);
    const h = Math.floor((sec % 86400) / 3600);
    const m = Math.floor((sec % 3600) / 60);
    if (d > 0) return `${d}h ${h}j ${m}m`;
    if (h > 0) return `${h}j ${m}m`;
    return `${m}m`;
  }

  let usedPct = $derived(
    overview && overview.mem_total_bytes > 0
      ? Math.min(100, Math.round((overview.mem_used_bytes / overview.mem_total_bytes) * 100))
      : 0
  );
  let cachePct = $derived(
    overview && overview.mem_total_bytes > 0
      ? Math.min(100 - usedPct, Math.round((overview.mem_cache_bytes / overview.mem_total_bytes) * 100))
      : 0
  );

  function pctFillClass(pct: number): string {
    if (pct >= 85) return 'sys-mini-bar__fill sys-mini-bar__fill--danger';
    if (pct >= 70) return 'sys-mini-bar__fill sys-mini-bar__fill--warn';
    return 'sys-mini-bar__fill';
  }
</script>

<div class="sys-pane">
  {#if overview && !overview.is_linux}
    <div class="alert alert--info" role="status">
      <i class="ri-information-line" aria-hidden="true"></i>
      <div>
        <strong>Mode Pengembangan Non-Linux:</strong> Metrik `/proc`, partisi disk Linux, Swapfile manager, APT updates, dan diagnostik kernel aktif penuh saat FikPanel berjalan pada server Linux.
      </div>
    </div>
  {/if}

  {#if overview?.needs_reboot}
    <div class="alert alert--warning" role="alert">
      <i class="ri-restart-line" aria-hidden="true"></i>
      <div class="grow">
        <strong>Reboot Disarankan:</strong> Sistem mendeteksi berkas <code>/var/run/reboot-required</code> pasca pembaruan kernel atau pustaka sistem.
      </div>
      <button type="button" class="btn btn--danger btn--sm" onclick={onOpenRebootModal}>
        Reboot Server
      </button>
    </div>
  {/if}

  {#if (overview?.oom_kills_recent || 0) > 0}
    <div class="alert alert--danger" role="alert">
      <i class="ri-error-warning-fill" aria-hidden="true"></i>
      <div class="grow">
        <strong>Deteksi OOM Killer:</strong> Tercatat <strong>{overview?.oom_kills_recent}</strong> kejadian proses dihentikan karena kehabisan memori di kernel log. Pertimbangkan menambah Swapfile atau memeriksa log diagnostik.
      </div>
      <button type="button" class="btn btn--secondary btn--sm" onclick={() => onJumpTab('memory')}>
        Kelola Swap
      </button>
      <button type="button" class="btn btn--secondary btn--sm" onclick={() => onJumpTab('diagnostics')}>
        Lihat Log OOM
      </button>
    </div>
  {/if}

  <!-- 4 Kartu Metrik Live Host -->
  <div class="sys-kpi-grid">
    <div class="stat-card">
      <div class="stat-card__icon">
        <i class="ri-cpu-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Load Average (1m / 5m / 15m)</span>
        <span class="stat-card__value mono">
          {overview ? `${overview.load_1.toFixed(2)} · ${overview.load_5.toFixed(2)}` : '-'}
        </span>
        <span class="stat-card__sub">15m: {overview ? overview.load_15.toFixed(2) : '-'} · {overview?.cpu_cores || 0} Core</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--info">
        <i class="ri-ram-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Memori RAM Terpakai</span>
        <span class="stat-card__value">{overview ? `${overview.mem_used_pct.toFixed(1)}%` : '-'}</span>
        <span class="stat-card__sub">
          {overview ? `${formatBytes(overview.mem_used_bytes)} / ${formatBytes(overview.mem_total_bytes)}` : '-'}
        </span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--violet">
        <i class="ri-swap-box-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Memori Swap</span>
        <span class="stat-card__value">
          {overview && overview.swap_total_bytes > 0 ? `${overview.swap_used_pct.toFixed(1)}%` : 'Tidak Ada'}
        </span>
        <span class="stat-card__sub">
          {overview && overview.swap_total_bytes > 0
            ? `${formatBytes(overview.swap_used_bytes)} / ${formatBytes(overview.swap_total_bytes)}`
            : 'Buat Swapfile di tab Swap & Memori'}
        </span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon" class:stat-card__icon--danger={(overview?.zombie_procs || 0) > 0} class:stat-card__icon--warning={(overview?.zombie_procs || 0) === 0}>
        <i class="ri-pulse-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Proses &amp; Uptime</span>
        <span class="stat-card__value">{overview?.total_procs ?? 0}</span>
        <span class="stat-card__sub">
          {overview?.running_procs ?? 0} aktif · {overview?.zombie_procs ?? 0} zombie · Up {formatUptime(overview?.uptime_sec || 0)}
        </span>
      </div>
    </div>
  </div>

  <!-- Identitas Hardware & Pecahan Alokasi Memori -->
  <div class="sys-two-col">
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-server-line" aria-hidden="true"></i>
          <span>Spesifikasi Host &amp; Sistem Operasi</span>
        </span>
        <button type="button" class="btn btn--danger btn--sm" onclick={onOpenRebootModal}>
          <i class="ri-shut-down-line" aria-hidden="true"></i>
          <span>Reboot</span>
        </button>
      </div>
      <div class="card__body">
        <div class="sys-spec-grid">
          <div class="sys-spec-item">
            <span class="sys-spec-label">Hostname</span>
            <span class="sys-spec-val mono" title={overview?.hostname || ''}>{overview?.hostname || '-'}</span>
          </div>
          <div class="sys-spec-item">
            <span class="sys-spec-label">Sistem Operasi</span>
            <span class="sys-spec-val" title={overview?.os || ''}>{overview?.os || '-'}</span>
          </div>
          <div class="sys-spec-item">
            <span class="sys-spec-label">Kernel</span>
            <span class="sys-spec-val mono" title={overview?.kernel || ''}>{overview?.kernel || '-'}</span>
          </div>
          <div class="sys-spec-item">
            <span class="sys-spec-label">Arsitektur</span>
            <span class="sys-spec-val mono">{overview?.arch || '-'}</span>
          </div>
          <div class="sys-spec-item">
            <span class="sys-spec-label">Prosesor</span>
            <span class="sys-spec-val" title={overview?.cpu_model || ''}>{overview?.cpu_model || '-'}</span>
          </div>
          <div class="sys-spec-item">
            <span class="sys-spec-label">Uptime Host</span>
            <span class="sys-spec-val mono">{formatUptime(overview?.uptime_sec || 0)}</span>
          </div>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-pie-chart-2-line" aria-hidden="true"></i>
          <span>Pecahan Alokasi Memori RAM</span>
        </span>
        <button type="button" class="btn btn--secondary btn--sm" onclick={() => onJumpTab('memory')}>
          Optimasi RAM &amp; Swap
        </button>
      </div>
      <div class="card__body">
        <div class="sys-mem-bar" aria-label="Pecahan penggunaan RAM">
          <div class="sys-mem-seg sys-mem-seg--used" style="--seg-used: {usedPct}%"></div>
          <div class="sys-mem-seg sys-mem-seg--cache" style="--seg-cache: {cachePct}%"></div>
        </div>
        <div class="sys-mem-legend">
          <span class="sys-legend-item">
            <span class="dot dot--danger"></span>
            <span>Terpakai: <strong class="mono">{formatBytes(overview?.mem_used_bytes || 0)}</strong></span>
          </span>
          <span class="sys-legend-item">
            <span class="dot dot--info"></span>
            <span>Buffers/Cache: <strong class="mono">{formatBytes(overview?.mem_cache_bytes || 0)}</strong></span>
          </span>
          <span class="sys-legend-item">
            <span class="dot dot--success"></span>
            <span>Tersedia: <strong class="mono">{formatBytes(overview?.mem_avail_bytes || 0)}</strong></span>
          </span>
        </div>
      </div>
    </div>
  </div>

  <!-- Tabel Partisi Disk & Pemantauan Inode -->
  <div class="card">
    <div class="card__head">
      <span class="card__title">
        <i class="ri-hard-drive-2-line" aria-hidden="true"></i>
        <span>Partisi Disk &amp; Kesehatan Inode</span>
      </span>
      <button type="button" class="btn btn--secondary btn--sm" onclick={() => onJumpTab('cleaner')}>
        <i class="ri-pie-chart-line" aria-hidden="true"></i>
        <span>Analisis Direktori Besar</span>
      </button>
    </div>

    <div class="table-wrap">
      <table class="table table--dense">
        <thead>
          <tr>
            <th>Mountpoint</th>
            <th>Perangkat</th>
            <th>Tipe FS</th>
            <th>Terpakai / Total</th>
            <th>Kapasitas Disk</th>
            <th>Penggunaan Inode</th>
          </tr>
        </thead>
        <tbody>
          {#if !overview?.disks || overview.disks.length === 0}
            <tr>
              <td colspan="6">
                <div class="empty-state">
                  <i class="ri-hard-drive-line" aria-hidden="true"></i>
                  <div class="empty-state__title">Data Partisi Tidak Tersedia</div>
                  <p class="state__desc">Inspeksi partisi blok &amp; inode (`df -B1` / `df -i`) aktif pada server Linux.</p>
                </div>
              </td>
            </tr>
          {:else}
            {#each overview.disks as d (d.mountpoint)}
              <tr>
                <td class="mono"><strong>{d.mountpoint}</strong></td>
                <td class="mono text-muted">{d.device}</td>
                <td><span class="badge badge--neutral mono">{d.fs_type}</span></td>
                <td class="mono text-subtle">
                  {formatBytes(d.used_bytes)} / {formatBytes(d.total_bytes)}
                </td>
                <td>
                  <div class="sys-mini-bar-wrap">
                    <div class="sys-mini-bar">
                      <div class={pctFillClass(d.used_pct)} style="--bar-w: {Math.min(100, Math.round(d.used_pct))}%"></div>
                    </div>
                    <span class="mono tabular">{d.used_pct.toFixed(1)}%</span>
                  </div>
                </td>
                <td>
                  <div class="sys-mini-bar-wrap">
                    <div class="sys-mini-bar">
                      <div class={pctFillClass(d.inodes_pct)} style="--bar-w: {Math.min(100, Math.round(d.inodes_pct))}%"></div>
                    </div>
                    <span class="mono tabular">{d.inodes_pct.toFixed(1)}%</span>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</div>
