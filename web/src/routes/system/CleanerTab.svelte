<script lang="ts">
  import { formatBytes } from '../../lib/api';
  import type { CleanerScanResult, CleanReport, DiskHogsResult } from '../../lib/types';

  export interface CleanSelectionPayload {
    clean_apt: boolean;
    clean_journal: boolean;
    clean_logs: boolean;
    clean_tmp: boolean;
    clean_dev_cache: boolean;
    clean_docker: boolean;
    drop_caches: boolean;
  }

  interface Props {
    scanResult: CleanerScanResult | null;
    cleanReport: CleanReport | null;
    diskHogs: DiskHogsResult | null;
    scanning: boolean;
    cleaning: boolean;
    analyzingHogs: boolean;
    onScan: () => void;
    onClean: (payload: CleanSelectionPayload) => void;
    onAnalyzeHogs: (targetPath: string) => void;
  }

  let {
    scanResult,
    cleanReport,
    diskHogs,
    scanning,
    cleaning,
    analyzingHogs,
    onScan,
    onClean,
    onAnalyzeHogs,
  }: Props = $props();

  // Pilihan kategori sampah bawaan yang paling aman untuk pembersihan rutin
  let selected = $state<Record<string, boolean>>({
    clean_apt: true,
    clean_journal: true,
    clean_logs: true,
    clean_tmp: true,
    clean_dev_cache: false,
    clean_docker: false,
  });
  let dropCaches = $state(false);
  let hogTarget = $state('/');

  const allowedRoots = [
    { path: '/', label: '/ (Root Filesystem)' },
    { path: '/var', label: '/var (Logs, DB, Cache)' },
    { path: '/var/log', label: '/var/log (Berkas Log)' },
    { path: '/www', label: '/www (Website & Aplikasi)' },
    { path: '/home', label: '/home (Direktori User)' },
    { path: '/opt', label: '/opt (Layanan Eksternal)' },
    { path: '/usr', label: '/usr (Biner & Pustaka OS)' },
  ];

  // Hitung estimasi total ukuran kategori yang dicentang dan tersedia
  let selectedBytes = $derived.by(() => {
    if (!scanResult?.categories) return 0;
    return scanResult.categories.reduce((acc, cat) => {
      if (cat.available && selected[cat.key]) return acc + cat.bytes;
      return acc;
    }, 0);
  });

  let anySelected = $derived.by(() => {
    if (dropCaches) return true;
    if (!scanResult?.categories) return false;
    return scanResult.categories.some((cat) => cat.available && selected[cat.key]);
  });

  function submitClean() {
    const avail = new Set((scanResult?.categories || []).filter((c) => c.available).map((c) => c.key));
    onClean({
      clean_apt: avail.has('clean_apt') && !!selected.clean_apt,
      clean_journal: avail.has('clean_journal') && !!selected.clean_journal,
      clean_logs: avail.has('clean_logs') && !!selected.clean_logs,
      clean_tmp: avail.has('clean_tmp') && !!selected.clean_tmp,
      clean_dev_cache: avail.has('clean_dev_cache') && !!selected.clean_dev_cache,
      clean_docker: avail.has('clean_docker') && !!selected.clean_docker,
      drop_caches: dropCaches,
    });
  }

  function hogFillClass(pct: number): string {
    if (pct >= 45) return 'sys-mini-bar__fill sys-mini-bar__fill--danger';
    if (pct >= 20) return 'sys-mini-bar__fill sys-mini-bar__fill--warn';
    return 'sys-mini-bar__fill sys-mini-bar__fill--info';
  }
</script>

<div class="sys-pane">
  <div class="sys-two-col">
    <!-- Pembersih Sampah Disk 6 Kategori -->
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-brush-line" aria-hidden="true"></i>
          <span>Smart Disk Cleaner</span>
        </span>
        <button type="button" class="btn btn--secondary btn--sm" disabled={scanning || cleaning} onclick={onScan}>
          <i class={scanning ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
          <span>{scanning ? 'Memindai...' : 'Pindai Ulang'}</span>
        </button>
      </div>

      <div class="card__body stack">
        <div class="alert alert--info" role="status">
          <i class="ri-hard-drive-2-line" aria-hidden="true"></i>
          <div class="grow">
            Total ruang dapat direklamasi: <strong class="mono">{formatBytes(scanResult?.total_bytes || 0)}</strong>
            {#if selectedBytes > 0}
              · Terpilih: <strong class="mono">{formatBytes(selectedBytes)}</strong>
            {/if}
          </div>
        </div>

        {#if cleanReport}
          <div class={cleanReport.errors.length > 0 ? 'alert alert--warning' : 'alert alert--success'} role="status">
            <i class={cleanReport.errors.length > 0 ? 'ri-alert-line' : 'ri-checkbox-circle-line'} aria-hidden="true"></i>
            <div class="stack-sm grow">
              <div>
                <strong>Hasil Pembersihan Terakhir:</strong>
                Disk dibebaskan <strong class="mono">{formatBytes(cleanReport.reclaimed_bytes)}</strong>
                {#if cleanReport.ram_freed_bytes > 0}
                  · RAM cache dibebaskan <strong class="mono">{formatBytes(cleanReport.ram_freed_bytes)}</strong>
                {/if}
              </div>
              {#if cleanReport.actions.length > 0}
                <div class="text-xs">{cleanReport.actions.join(' · ')}</div>
              {/if}
              {#if cleanReport.errors.length > 0}
                <div class="text-xs">Catatan galat: {cleanReport.errors.join(' · ')}</div>
              {/if}
            </div>
          </div>
        {/if}

        <div class="sys-cleaner-list">
          {#if scanResult?.categories && scanResult.categories.length > 0}
            {#each scanResult.categories as cat (cat.key)}
              <label class="sys-cleaner-row">
                <div class="sys-cleaner-left">
                  <input
                    type="checkbox"
                    disabled={!cat.available || cleaning}
                    bind:checked={selected[cat.key]}
                  />
                  <div class="sys-cleaner-meta">
                    <div class="row wrap">
                      <span class="sys-cleaner-title">{cat.label}</span>
                      <span class="badge badge--neutral mono">{cat.path}</span>
                      {#if !cat.available}
                        <span class="badge badge--warning">Tidak Tersedia</span>
                      {/if}
                    </div>
                    <span class="sys-cleaner-desc">{cat.description}</span>
                  </div>
                </div>
                <span class="sys-cleaner-size">{formatBytes(cat.bytes)}</span>
              </label>
            {/each}
          {:else}
            <div class="empty-state">
              <i class="ri-brush-line" aria-hidden="true"></i>
              <div class="empty-state__title">Belum Ada Hasil Pemindaian</div>
              <p class="state__desc">Klik <strong>Pindai Ulang</strong> untuk menghitung ukuran cache dan log lama di host.</p>
            </div>
          {/if}

          <!-- Opsi tambahan pengosongan RAM page cache pasca pembersihan -->
          <label class="sys-cleaner-row">
            <div class="sys-cleaner-left">
              <input type="checkbox" disabled={!scanResult?.is_linux || cleaning} bind:checked={dropCaches} />
              <div class="sys-cleaner-meta">
                <div class="row wrap">
                  <span class="sys-cleaner-title">Sekaligus Lepaskan RAM Page Cache</span>
                  <span class="badge badge--info mono">sync + drop_caches</span>
                </div>
                <span class="sys-cleaner-desc">
                  Opsional: menyinkronkan buffer disk lalu mengosongkan page, dentry, dan inode cache kernel secara aman.
                </span>
              </div>
            </div>
            <span class="sys-cleaner-size text-subtle">RAM</span>
          </label>
        </div>
      </div>

      <div class="card__foot">
        <button
          type="button"
          class="btn btn--primary btn--sm"
          disabled={!anySelected || cleaning || scanning || !scanResult?.is_linux}
          onclick={submitClean}
        >
          <i class={cleaning ? 'ri-loader-4-line spin' : 'ri-delete-bin-6-line'} aria-hidden="true"></i>
          <span>{cleaning ? 'Membersihkan Sistem...' : 'Bersihkan Kategori Terpilih'}</span>
        </button>
      </div>
    </div>

    <!-- Top Disk Hogs Analyzer (du -x -B1 --max-depth=1) -->
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-pie-chart-line" aria-hidden="true"></i>
          <span>Top Disk Hogs Analyzer</span>
        </span>
        {#if diskHogs}
          <span class="badge badge--info mono">Total: {formatBytes(diskHogs.total_bytes)}</span>
        {/if}
      </div>

      <div class="sys-toolbar">
        <div class="sys-toolbar-group grow">
          <select class="select grow" aria-label="Pilih direktori root analisis" bind:value={hogTarget}>
            {#each allowedRoots as r (r.path)}
              <option value={r.path}>{r.label}</option>
            {/each}
          </select>
        </div>
        <button
          type="button"
          class="btn btn--primary btn--sm"
          disabled={analyzingHogs}
          onclick={() => onAnalyzeHogs(hogTarget)}
        >
          <i class={analyzingHogs ? 'ri-loader-4-line spin' : 'ri-search-eye-line'} aria-hidden="true"></i>
          <span>{analyzingHogs ? 'Menganalisis...' : 'Analisis Direktori'}</span>
        </button>
      </div>

      <div class="table-wrap">
        <table class="table table--dense">
          <thead>
            <tr>
              <th>Subdirektori</th>
              <th class="sys-col-right">Ukuran</th>
              <th>Porsi</th>
            </tr>
          </thead>
          <tbody>
            {#if analyzingHogs}
              <tr>
                <td colspan="3">
                  <div class="state state--loading">
                    <i class="ri-loader-4-line spin" aria-hidden="true"></i>
                    <div class="state__title">Menghitung Ukuran Subdirektori {hogTarget}...</div>
                  </div>
                </td>
              </tr>
            {:else if !diskHogs || diskHogs.entries.length === 0}
              <tr>
                <td colspan="3">
                  <div class="empty-state">
                    <i class="ri-folder-chart-line" aria-hidden="true"></i>
                    <div class="empty-state__title">
                      {diskHogs ? 'Tidak Ada Subdirektori Terukur' : 'Temukan Penyebab Disk Penuh'}
                    </div>
                    <p class="state__desc">
                      Pilih root direktori (misalnya <code>/var</code> atau <code>/www</code>) lalu klik <strong>Analisis Direktori</strong> untuk melihat 15 subdirektori terbesar.
                    </p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each diskHogs.entries as entry (entry.path)}
                <tr>
                  <td class="mono break-all" title={entry.path}><strong>{entry.path}</strong></td>
                  <td class="mono tabular sys-col-right">{formatBytes(entry.size_bytes)}</td>
                  <td>
                    <div class="sys-mini-bar-wrap">
                      <div class="sys-mini-bar">
                        <div
                          class={hogFillClass(entry.percent)}
                          style="--bar-w: {Math.min(100, Math.round(entry.percent))}%"
                        ></div>
                      </div>
                      <span class="mono tabular">{entry.percent.toFixed(1)}%</span>
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
</div>
