<script lang="ts">
  import type { UpdatesOverview, DiagnosticsReport } from '../../lib/types';

  interface Props {
    updates: UpdatesOverview | null;
    diagnostics: DiagnosticsReport | null;
    loadingUpdates: boolean;
    applyingUpdates: boolean;
    loadingDiag: boolean;
    onCheckUpdates: (refresh: boolean) => void;
    onApplySecurityUpdates: () => void;
    onRefreshDiag: () => void;
    onOpenRebootModal: () => void;
  }

  let {
    updates,
    diagnostics,
    loadingUpdates,
    applyingUpdates,
    loadingDiag,
    onCheckUpdates,
    onApplySecurityUpdates,
    onRefreshDiag,
    onOpenRebootModal,
  }: Props = $props();

  let updateFilter = $state<'all' | 'security'>('all');
  let diagFilter = $state<'all' | 'oom' | 'error' | 'warning'>('all');

  let filteredUpdates = $derived.by(() => {
    const list = updates?.updates || [];
    if (updateFilter === 'security') return list.filter((u) => u.is_security);
    return list;
  });

  let filteredEntries = $derived.by(() => {
    const list = diagnostics?.entries || [];
    if (diagFilter === 'oom') return list.filter((e) => e.is_oom);
    if (diagFilter === 'error') return list.filter((e) => e.level === 'error' || e.level === 'oom');
    if (diagFilter === 'warning') return list.filter((e) => e.level === 'warning');
    return list;
  });

  function formatCheckedAt(ts: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleTimeString('id-ID', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  function diagLevelBadge(level: string, isOOM: boolean): { cls: string; label: string } {
    if (isOOM || level === 'oom') return { cls: 'badge badge--danger', label: 'OOM KILLER' };
    if (level === 'error') return { cls: 'badge badge--danger', label: 'ERROR' };
    return { cls: 'badge badge--warning', label: 'WARNING' };
  }
</script>

<div class="sys-pane">
  <!-- KPI Pembaruan OS & Kesehatan Kernel -->
  <div class="sys-kpi-grid">
    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--info">
        <i class="ri-download-cloud-2-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Total Update Paket OS</span>
        <span class="stat-card__value">{updates?.total_count ?? 0}</span>
        <span class="stat-card__sub">Manajer: {updates?.package_manager || '-'} · Cek {formatCheckedAt(updates?.checked_at || 0)}</span>
      </div>
    </div>

    <div class="stat-card">
      <div
        class="stat-card__icon"
        class:stat-card__icon--danger={(updates?.security_count || 0) > 0}
        class:stat-card__icon--success={(updates?.security_count || 0) === 0}
      >
        <i class="ri-shield-keyhole-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Patch Keamanan Tertunda</span>
        <span class="stat-card__value">{updates?.security_count ?? 0}</span>
        <span class="stat-card__sub">
          {(updates?.security_count || 0) > 0 ? 'Disarankan segera dipasang' : 'Semua patch keamanan mutakhir'}
        </span>
      </div>
    </div>

    <div class="stat-card">
      <div
        class="stat-card__icon"
        class:stat-card__icon--danger={(diagnostics?.oom_count || 0) > 0}
        class:stat-card__icon--success={(diagnostics?.oom_count || 0) === 0}
      >
        <i class="ri-skull-2-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Insiden OOM Killer</span>
        <span class="stat-card__value">{diagnostics?.oom_count ?? 0}</span>
        <span class="stat-card__sub">
          {(diagnostics?.oom_count || 0) > 0 ? 'Proses pernah dihentikan kernel' : 'Tidak ada insiden kehabisan RAM'}
        </span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--warning">
        <i class="ri-stethoscope-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Error &amp; Warning Log</span>
        <span class="stat-card__value">{(diagnostics?.error_count ?? 0) + (diagnostics?.warn_count ?? 0)}</span>
        <span class="stat-card__sub">{diagnostics?.error_count ?? 0} error · {diagnostics?.warn_count ?? 0} warning</span>
      </div>
    </div>
  </div>

  {#if updates?.needs_reboot}
    <div class="alert alert--warning" role="alert">
      <i class="ri-restart-line" aria-hidden="true"></i>
      <div class="grow">
        <strong>Reboot Diperlukan:</strong> Pembaruan kernel atau pustaka inti sebelumnya memerlukan restart host agar sepenuhnya aktif.
      </div>
      <button type="button" class="btn btn--danger btn--sm" onclick={onOpenRebootModal}>
        Reboot Server
      </button>
    </div>
  {/if}

  <!-- Kartu Daftar Pending OS Updates -->
  <div class="card">
    <div class="card__head">
      <span class="card__title">
        <i class="ri-box-3-line" aria-hidden="true"></i>
        <span>Pembaruan Paket OS &amp; Patch Keamanan</span>
      </span>
      <div class="sys-toolbar-group">
        <select class="select" aria-label="Filter paket pembaruan" bind:value={updateFilter}>
          <option value="all">Semua Paket ({updates?.total_count ?? 0})</option>
          <option value="security">Hanya Keamanan ({updates?.security_count ?? 0})</option>
        </select>
        <button
          type="button"
          class="btn btn--secondary btn--sm"
          disabled={loadingUpdates || applyingUpdates}
          onclick={() => onCheckUpdates(true)}
        >
          <i class={loadingUpdates ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
          <span>{loadingUpdates ? 'Memeriksa APT...' : 'Cek Pembaruan (apt update)'}</span>
        </button>
        <button
          type="button"
          class="btn btn--primary btn--sm"
          disabled={applyingUpdates || loadingUpdates || !updates?.is_linux || (updates?.security_count || 0) === 0}
          onclick={onApplySecurityUpdates}
        >
          <i class={applyingUpdates ? 'ri-loader-4-line spin' : 'ri-shield-flash-line'} aria-hidden="true"></i>
          <span>{applyingUpdates ? 'Memasang Patch...' : 'Pasang Patch Keamanan'}</span>
        </button>
      </div>
    </div>

    <div class="table-wrap">
      <table class="table table--dense">
        <thead>
          <tr>
            <th>Nama Paket</th>
            <th>Versi Terpasang</th>
            <th>Versi Baru</th>
            <th>Repositori</th>
            <th>Kategori</th>
          </tr>
        </thead>
        <tbody>
          {#if loadingUpdates && !updates}
            <tr>
              <td colspan="5">
                <div class="state state--loading">
                  <i class="ri-loader-4-line spin" aria-hidden="true"></i>
                  <div class="state__title">Memeriksa Daftar Pembaruan OS...</div>
                </div>
              </td>
            </tr>
          {:else if filteredUpdates.length === 0}
            <tr>
              <td colspan="5">
                <div class="empty-state">
                  <i class="ri-checkbox-circle-line" aria-hidden="true"></i>
                  <div class="empty-state__title">
                    {updateFilter === 'security'
                      ? 'Tidak Ada Patch Keamanan Tertunda'
                      : 'Seluruh Paket OS Sudah Mutakhir'}
                  </div>
                  <p class="state__desc">
                    {updates?.is_linux
                      ? 'Klik Cek Pembaruan (apt update) untuk menyegarkan indeks repositori terbaru dari mirror Ubuntu/Debian.'
                      : 'Pemeriksaan pembaruan paket APT aktif otomatis saat FikPanel berjalan di server Linux.'}
                  </p>
                </div>
              </td>
            </tr>
          {:else}
            {#each filteredUpdates as pkg (pkg.name)}
              <tr>
                <td class="mono"><strong>{pkg.name}</strong></td>
                <td class="mono text-muted">{pkg.current_version}</td>
                <td class="mono">{pkg.new_version}</td>
                <td class="mono text-subtle">{pkg.source}</td>
                <td>
                  {#if pkg.is_security}
                    <span class="badge badge--danger">Keamanan</span>
                  {:else}
                    <span class="badge badge--neutral">Reguler</span>
                  {/if}
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>

  <!-- Kartu Log Diagnostik Kernel (dmesg) & Systemd Error (journalctl) -->
  <div class="card">
    <div class="card__head">
      <span class="card__title">
        <i class="ri-terminal-window-line" aria-hidden="true"></i>
        <span>Diagnostik Kernel (dmesg) &amp; Systemd Error</span>
      </span>
      <div class="sys-toolbar-group">
        <select class="select" aria-label="Filter level diagnostik" bind:value={diagFilter}>
          <option value="all">Semua Level ({diagnostics?.entries?.length ?? 0})</option>
          <option value="oom">OOM Killer ({diagnostics?.oom_count ?? 0})</option>
          <option value="error">Error &amp; OOM ({diagnostics?.error_count ?? 0})</option>
          <option value="warning">Warning ({diagnostics?.warn_count ?? 0})</option>
        </select>
        <button type="button" class="btn btn--secondary btn--sm" disabled={loadingDiag} onclick={onRefreshDiag}>
          <i class={loadingDiag ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
          <span>Segarkan Log</span>
        </button>
      </div>
    </div>

    <div class="card__body">
      {#if loadingDiag && !diagnostics}
        <div class="state state--loading">
          <i class="ri-loader-4-line spin" aria-hidden="true"></i>
          <div class="state__title">Membaca Log Diagnostik Kernel &amp; Systemd...</div>
        </div>
      {:else if filteredEntries.length === 0}
        <div class="empty-state">
          <i class="ri-heart-pulse-line" aria-hidden="true"></i>
          <div class="empty-state__title">Tidak Ada Anomali Ditemukan</div>
          <p class="state__desc">
            {diagnostics?.is_linux
              ? 'Tidak tercatat pesan error, warning, atau insiden OOM Killer untuk filter saat ini.'
              : 'Pembacaan log kernel dmesg dan journalctl aktif pada host server Linux.'}
          </p>
        </div>
      {:else}
        <div class="sys-diag-list">
          {#each filteredEntries as item, i (`${item.source}-${item.timestamp}-${i}`)}
            {@const badge = diagLevelBadge(item.level, item.is_oom)}
            <div class="sys-diag-row" class:sys-diag-row--oom={item.is_oom}>
              <div class="row wrap">
                <span class={badge.cls}>{badge.label}</span>
                <span class="badge badge--neutral mono">{item.source}</span>
                {#if item.timestamp}
                  <span class="mono text-subtle text-xs">{item.timestamp}</span>
                {/if}
              </div>
              <div class="sys-diag-msg">{item.message}</div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</div>
