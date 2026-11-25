<script lang="ts">
  import type { UptimeCheck, UptimeMonitor } from '../../lib/types';

  interface Props {
    monitors: UptimeMonitor[];
    loading: boolean;
    importing: boolean;
    checkingAll: boolean;
    checkingId: number | null;
    togglingId: number | null;
    onRefresh: () => void;
    onCheckAll: () => void;
    onImportSites: () => void;
    onAdd: () => void;
    onEdit: (m: UptimeMonitor) => void;
    onCheckNow: (m: UptimeMonitor) => void;
    onToggleEnabled: (m: UptimeMonitor) => void;
    onDelete: (m: UptimeMonitor) => void;
    onInspect: (m: UptimeMonitor) => void;
  }

  let {
    monitors,
    loading,
    importing,
    checkingAll,
    checkingId,
    togglingId,
    onRefresh,
    onCheckAll,
    onImportSites,
    onAdd,
    onEdit,
    onCheckNow,
    onToggleEnabled,
    onDelete,
    onInspect,
  }: Props = $props();

  let search = $state('');
  let statusFilter = $state<'all' | 'up' | 'down' | 'ssl' | 'paused'>('all');

  let stats = $derived.by(() => {
    const total = monitors.length;
    let up = 0;
    let down = 0;
    let sslWarn = 0;
    let latSum = 0;
    let latCount = 0;
    for (const m of monitors) {
      if (!m.enabled) continue;
      if (m.last_status === 'up') up++;
      else if (m.last_status === 'down') down++;
      if (m.ssl_days_remaining >= 0 && m.ssl_days_remaining <= 14) sslWarn++;
      if (m.last_latency_ms > 0) {
        latSum += m.last_latency_ms;
        latCount++;
      }
    }
    return {
      total,
      up,
      down,
      sslWarn,
      avgLatency: latCount > 0 ? Math.round(latSum / latCount) : 0,
    };
  });

  let filteredMonitors = $derived.by(() => {
    const q = search.trim().toLowerCase();
    return monitors.filter((m) => {
      if (statusFilter === 'up' && (!m.enabled || m.last_status !== 'up')) return false;
      if (statusFilter === 'down' && (!m.enabled || m.last_status !== 'down')) return false;
      if (statusFilter === 'ssl' && !(m.ssl_days_remaining >= 0 && m.ssl_days_remaining <= 14)) return false;
      if (statusFilter === 'paused' && m.enabled) return false;
      if (q) {
        return (
          m.name.toLowerCase().includes(q) ||
          m.target.toLowerCase().includes(q) ||
          m.monitor_type.toLowerCase().includes(q)
        );
      }
      return true;
    });
  });

  function buildHeartbeatSlots(checks: UptimeCheck[] | undefined): Array<UptimeCheck | null> {
    const list = checks || [];
    const totalSlots = 30;
    if (list.length >= totalSlots) return list.slice(-totalSlots);
    return [...Array<UptimeCheck | null>(totalSlots - list.length).fill(null), ...list];
  }

  function formatCheckTooltip(c: UptimeCheck | null): string {
    if (!c) return 'Belum ada data pemeriksaan';
    const dt = new Date(c.checked_at * 1000).toLocaleString('id-ID', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      day: '2-digit',
      month: 'short',
    });
    const st = c.status === 'up' ? 'ONLINE' : 'OFFLINE';
    const code = c.status_code > 1 ? ` · HTTP ${c.status_code}` : '';
    const err = c.error_msg ? ` · ${c.error_msg}` : '';
    return `${dt} — ${st}${code} (${c.latency_ms} ms)${err}`;
  }

  function formatLastChecked(ts: number): string {
    if (!ts) return 'Belum dicek';
    return new Date(ts * 1000).toLocaleTimeString('id-ID', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  function sslBadgeClass(days: number): string {
    if (days <= 3) return 'badge badge--danger';
    if (days <= 14) return 'badge badge--warning';
    return 'badge badge--success';
  }
</script>

<div class="mon-pane">
  <!-- KPI Uptime & SSL -->
  <div class="mon-kpi-grid">
    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--info">
        <i class="ri-radar-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Total Target Monitor</span>
        <span class="stat-card__value">{stats.total}</span>
        <span class="stat-card__sub">HTTP/HTTPS &amp; TCP terdaftar</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--success">
        <i class="ri-checkbox-circle-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Layanan Online</span>
        <span class="stat-card__value">{stats.up}</span>
        <span class="stat-card__sub">Merespons normal sesuai kriteria</span>
      </div>
    </div>

    <div class="stat-card">
      <div
        class="stat-card__icon"
        class:stat-card__icon--danger={stats.down > 0}
        class:stat-card__icon--warning={stats.down === 0}
      >
        <i class="ri-alarm-warning-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Sedang Gangguan</span>
        <span class="stat-card__value">{stats.down}</span>
        <span class="stat-card__sub">Tercatat di log insiden &amp; webhook</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--violet">
        <i class="ri-speed-up-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Rata-rata Latensi</span>
        <span class="stat-card__value mono">{stats.avgLatency} ms</span>
        <span class="stat-card__sub">
          {stats.sslWarn > 0 ? `${stats.sslWarn} sertifikat SSL ≤ 14 hari` : 'Seluruh sertifikat SSL aman'}
        </span>
      </div>
    </div>
  </div>

  <!-- Daftar Target Uptime & SSL -->
  <div class="card">
    <div class="mon-toolbar">
      <div class="mon-toolbar-group">
        <div class="input-wrap mon-search">
          <i class="ri-search-line input__icon" aria-hidden="true"></i>
          <input
            type="text"
            class="input input--search"
            placeholder="Cari nama atau URL target..."
            bind:value={search}
          />
        </div>

        <select class="select" aria-label="Filter status uptime" bind:value={statusFilter}>
          <option value="all">Semua Status ({monitors.length})</option>
          <option value="up">Online ({stats.up})</option>
          <option value="down">Offline ({stats.down})</option>
          <option value="ssl">SSL Kritis ≤14h ({stats.sslWarn})</option>
          <option value="paused">Dijeda</option>
        </select>
      </div>

      <div class="mon-toolbar-group">
        <a
          class="btn btn--secondary btn--sm"
          href="/status"
          target="_blank"
          rel="noopener noreferrer"
          title="Buka halaman status publik tanpa login"
        >
          <i class="ri-external-link-line" aria-hidden="true"></i>
          <span>/status</span>
        </a>
        <button
          type="button"
          class="btn btn--secondary btn--sm"
          disabled={checkingAll || monitors.length === 0}
          onclick={onCheckAll}
        >
          <i class={checkingAll ? 'ri-loader-4-line spin' : 'ri-flashlight-line'} aria-hidden="true"></i>
          <span>{checkingAll ? 'Memeriksa...' : 'Cek Semua'}</span>
        </button>
        <button
          type="button"
          class="btn btn--secondary btn--sm"
          disabled={importing}
          onclick={onImportSites}
        >
          <i class="ri-download-cloud-2-line" aria-hidden="true"></i>
          <span>{importing ? 'Mengimpor...' : 'Impor Situs'}</span>
        </button>
        <button
          type="button"
          class="btn btn--ghost btn--icon btn--sm"
          aria-label="Segarkan daftar monitor uptime"
          title="Segarkan"
          disabled={loading}
          onclick={onRefresh}
        >
          <i class={loading ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
        </button>
        <button type="button" class="btn btn--primary btn--sm" onclick={onAdd}>
          <i class="ri-add-line" aria-hidden="true"></i>
          <span>Tambah Monitor</span>
        </button>
      </div>
    </div>

    <div class="mon-uptime-list">
      {#if loading && monitors.length === 0}
        <div class="state state--loading">
          <i class="ri-loader-4-line spin" aria-hidden="true"></i>
          <div class="state__title">Memuat Daftar Monitor Uptime...</div>
        </div>
      {:else if filteredMonitors.length === 0}
        <div class="empty-state">
          <i class="ri-heart-pulse-line" aria-hidden="true"></i>
          <div class="empty-state__title">
            {monitors.length === 0 ? 'Belum Ada Target Uptime Monitor' : 'Tidak Ada Target Sesuai Filter'}
          </div>
          <p class="state__desc">
            {monitors.length === 0
              ? 'Tambahkan URL website/API atau port TCP untuk memantau ketersediaan 24/7 beserta masa berlaku SSL.'
              : 'Ubah kata kunci pencarian atau filter status untuk melihat monitor lainnya.'}
          </p>
          {#if monitors.length === 0}
            <div class="row wrap">
              <button type="button" class="btn btn--primary btn--sm" onclick={onAdd}>
                <i class="ri-add-line" aria-hidden="true"></i>
                <span>Tambah Monitor Pertama</span>
              </button>
              <button type="button" class="btn btn--secondary btn--sm" disabled={importing} onclick={onImportSites}>
                <i class="ri-download-cloud-2-line" aria-hidden="true"></i>
                <span>Impor dari Daftar Situs</span>
              </button>
            </div>
          {/if}
        </div>
      {:else}
        {#each filteredMonitors as mon (mon.id)}
          {@const slots = buildHeartbeatSlots(mon.recent_checks)}
          <div class="mon-uptime-card" class:is-paused={!mon.enabled} class:is-down={mon.enabled && mon.last_status === 'down'}>
            <div class="mon-uptime-top">
              <div class="mon-uptime-ident">
                <span
                  class="mon-status-dot"
                  class:is-up={mon.enabled && mon.last_status === 'up'}
                  class:is-down={mon.enabled && mon.last_status === 'down'}
                  class:is-paused={!mon.enabled}
                  aria-hidden="true"
                ></span>
                <div class="mon-uptime-titles">
                  <div class="row wrap">
                    <strong class="mon-uptime-name">{mon.name}</strong>
                    <span class="badge badge--neutral mono">{mon.monitor_type.toUpperCase()}</span>
                    {#if !mon.enabled}
                      <span class="badge badge--warning">Dijeda</span>
                    {:else if mon.last_status === 'up'}
                      <span class="badge badge--success">Online</span>
                    {:else if mon.last_status === 'down'}
                      <span class="badge badge--danger">Offline</span>
                    {:else}
                      <span class="badge badge--neutral">Menunggu</span>
                    {/if}
                    {#if mon.public_visible}
                      <span class="badge badge--info" title="Tampil di halaman /status">Publik</span>
                    {/if}
                    {#if mon.notify_webhook}
                      <span class="badge badge--neutral" title="Notifikasi Webhook aktif">
                        <i class="ri-notification-3-line" aria-hidden="true"></i> Alert
                      </span>
                    {/if}
                  </div>
                  <div class="mon-uptime-target mono">{mon.target}</div>
                </div>
              </div>

              <div class="mon-toolbar-group">
                {#if mon.ssl_days_remaining >= 0}
                  <span
                    class={sslBadgeClass(mon.ssl_days_remaining)}
                    title={mon.ssl_issuer ? `Penerbit SSL: ${mon.ssl_issuer}` : 'Sisa masa berlaku sertifikat TLS'}
                  >
                    <i class="ri-lock-line" aria-hidden="true"></i> SSL {mon.ssl_days_remaining}h
                  </span>
                {/if}
                <span class="mon-uptime-pill mono" title="Persentase uptime 24 jam terakhir">
                  {mon.uptime_24h_pct.toFixed(2)}%
                </span>
                <span class="mon-uptime-pill mono" title="Latensi pemeriksaan terakhir">
                  {mon.last_latency_ms} ms
                </span>
              </div>
            </div>

            <!-- Strip 30-bar heartbeat -->
            <div class="mon-hb-strip" aria-label="Riwayat 30 pemeriksaan terakhir">
              {#each slots as check, idx (check ? check.id : `empty-${mon.id}-${idx}`)}
                <span
                  class="mon-hb-bar"
                  class:mon-hb-bar--up={check?.status === 'up'}
                  class:mon-hb-bar--down={check?.status === 'down'}
                  title={formatCheckTooltip(check)}
                ></span>
              {/each}
            </div>

            {#if mon.enabled && mon.last_status === 'down' && mon.last_error}
              <div class="alert alert--danger" role="alert">
                <i class="ri-error-warning-line" aria-hidden="true"></i>
                <span class="mono text-xs">{mon.last_error}</span>
              </div>
            {/if}

            <div class="mon-uptime-foot">
              <div class="row wrap text-xs text-subtle mono">
                <span>Interval {mon.interval_sec}d</span>
                <span>·</span>
                <span>Cek: {formatLastChecked(mon.last_checked_at)}</span>
                {#if mon.avg_latency_ms > 0}
                  <span>·</span>
                  <span>Avg 24j: {mon.avg_latency_ms} ms</span>
                {/if}
                {#if mon.keyword}
                  <span>·</span>
                  <span>Keyword: "{mon.keyword}"</span>
                {/if}
              </div>

              <div class="mon-toolbar-group">
                <button
                  type="button"
                  class="btn btn--secondary btn--sm"
                  onclick={() => onInspect(mon)}
                >
                  <i class="ri-line-chart-line" aria-hidden="true"></i>
                  <span>Detail &amp; Latensi</span>
                </button>
                <button
                  type="button"
                  class="btn btn--ghost btn--icon btn--sm"
                  disabled={checkingId === mon.id}
                  aria-label={`Periksa ${mon.name} sekarang`}
                  title="Periksa Sekarang"
                  onclick={() => onCheckNow(mon)}
                >
                  <i class={checkingId === mon.id ? 'ri-loader-4-line spin' : 'ri-flashlight-line'} aria-hidden="true"></i>
                </button>
                <button
                  type="button"
                  class="btn btn--ghost btn--icon btn--sm"
                  disabled={togglingId === mon.id}
                  aria-label={mon.enabled ? `Jeda ${mon.name}` : `Aktifkan ${mon.name}`}
                  title={mon.enabled ? 'Jeda Monitor' : 'Aktifkan Monitor'}
                  onclick={() => onToggleEnabled(mon)}
                >
                  <i class={mon.enabled ? 'ri-pause-line' : 'ri-play-line'} aria-hidden="true"></i>
                </button>
                <button
                  type="button"
                  class="btn btn--ghost btn--icon btn--sm"
                  aria-label={`Edit ${mon.name}`}
                  title="Edit Monitor"
                  onclick={() => onEdit(mon)}
                >
                  <i class="ri-edit-line" aria-hidden="true"></i>
                </button>
                <button
                  type="button"
                  class="btn btn--ghost btn--icon btn--sm btn--danger"
                  aria-label={`Hapus ${mon.name}`}
                  title="Hapus Monitor"
                  onclick={() => onDelete(mon)}
                >
                  <i class="ri-delete-bin-line" aria-hidden="true"></i>
                </button>
              </div>
            </div>
          </div>
        {/each}
      {/if}
    </div>
  </div>
</div>
