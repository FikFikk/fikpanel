<script lang="ts">
  import type { MonitorAlert, MonitorThresholds } from '../../lib/types';

  interface Props {
    alerts: MonitorAlert[];
    unackedCount: number;
    webhooksCount: number;
    thresholds: MonitorThresholds;
    loading: boolean;
    savingThresholds: boolean;
    onRefresh: () => void;
    onSaveThresholds: (t: MonitorThresholds) => void;
    onAck: (id: number) => void;
    onAckAll: () => void;
    onClearAll: () => void;
  }

  let {
    alerts,
    unackedCount,
    webhooksCount,
    thresholds,
    loading,
    savingThresholds,
    onRefresh,
    onSaveThresholds,
    onAck,
    onAckAll,
    onClearAll,
  }: Props = $props();

  let sourceFilter = $state<'all' | 'cpu' | 'ram' | 'disk' | 'uptime' | 'ssl'>('all');
  let onlyUnacked = $state(false);

  // State form lokal yang disinkronkan dari thresholds server
  let cpuPct = $state(85);
  let ramPct = $state(85);
  let diskPct = $state(90);
  let sslDays = $state(14);
  let cooldownMin = $state(15);

  $effect(() => {
    cpuPct = thresholds.alert_cpu_pct;
    ramPct = thresholds.alert_ram_pct;
    diskPct = thresholds.alert_disk_pct;
    sslDays = thresholds.alert_ssl_days;
    cooldownMin = thresholds.alert_cooldown_min;
  });

  let filteredAlerts = $derived.by(() => {
    return alerts.filter((a) => {
      if (sourceFilter !== 'all' && a.source !== sourceFilter) return false;
      if (onlyUnacked && a.acknowledged) return false;
      return true;
    });
  });

  function formatTime(ts: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString('id-ID', {
      day: '2-digit',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  function severityBadge(sev: string): { cls: string; label: string } {
    if (sev === 'critical') return { cls: 'badge badge--danger', label: 'KRITIS' };
    if (sev === 'recovered') return { cls: 'badge badge--success', label: 'PULIH' };
    return { cls: 'badge badge--warning', label: 'PERINGATAN' };
  }

  function submitThresholds(e: SubmitEvent) {
    e.preventDefault();
    onSaveThresholds({
      alert_cpu_pct: Number(cpuPct) || 85,
      alert_ram_pct: Number(ramPct) || 85,
      alert_disk_pct: Number(diskPct) || 90,
      alert_ssl_days: Number(sslDays) || 14,
      alert_cooldown_min: Number(cooldownMin) || 15,
    });
  }
</script>

<div class="mon-pane">
  <div class="mon-two-col">
    <!-- Timeline Alert & Insiden Terpadu (CPU / RAM / Disk / Uptime / SSL) -->
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-alarm-warning-line" aria-hidden="true"></i>
          <span>Log Alert &amp; Insiden Terpadu</span>
        </span>
        <div class="mon-toolbar-group">
          {#if unackedCount > 0}
            <button type="button" class="btn btn--secondary btn--sm" onclick={onAckAll}>
              <i class="ri-check-double-line" aria-hidden="true"></i>
              <span>Tandai Semua Dibaca ({unackedCount})</span>
            </button>
          {/if}
          <button
            type="button"
            class="btn btn--ghost btn--icon btn--sm"
            aria-label="Segarkan riwayat alert"
            title="Segarkan"
            disabled={loading}
            onclick={onRefresh}
          >
            <i class={loading ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
          </button>
          <button
            type="button"
            class="btn btn--ghost btn--icon btn--sm btn--danger"
            aria-label="Kosongkan riwayat alert"
            title="Kosongkan Log"
            disabled={alerts.length === 0}
            onclick={onClearAll}
          >
            <i class="ri-delete-bin-line" aria-hidden="true"></i>
          </button>
        </div>
      </div>

      <div class="mon-toolbar">
        <div class="mon-toolbar-group">
          <select class="select" aria-label="Filter sumber alert" bind:value={sourceFilter}>
            <option value="all">Semua Sumber</option>
            <option value="cpu">CPU</option>
            <option value="ram">RAM</option>
            <option value="disk">Disk</option>
            <option value="uptime">Uptime</option>
            <option value="ssl">SSL</option>
          </select>
          <label class="mon-check-row">
            <input type="checkbox" bind:checked={onlyUnacked} />
            <span>Hanya belum dibaca</span>
          </label>
        </div>
        <span class="text-xs text-subtle mono">{filteredAlerts.length} peristiwa</span>
      </div>

      <div class="card__body">
        {#if loading && alerts.length === 0}
          <div class="state state--loading">
            <i class="ri-loader-4-line spin" aria-hidden="true"></i>
            <div class="state__title">Memuat Riwayat Insiden...</div>
          </div>
        {:else if filteredAlerts.length === 0}
          <div class="empty-state">
            <i class="ri-shield-check-line" aria-hidden="true"></i>
            <div class="empty-state__title">Tidak Ada Alert atau Insiden</div>
            <p class="state__desc">
              Setiap lonjakan CPU/RAM/Disk melewati ambang batas maupun insiden Uptime/SSL akan tercatat otomatis di sini beserta status pemulihannya.
            </p>
          </div>
        {:else}
          <div class="mon-alert-list">
            {#each filteredAlerts as item (item.id)}
              {@const sev = severityBadge(item.severity)}
              <div
                class="mon-alert-row"
                class:is-unread={!item.acknowledged && item.severity !== 'critical'}
                class:is-crit={!item.acknowledged && item.severity === 'critical'}
              >
                <div class="stack-sm grow">
                  <div class="row wrap">
                    <span class={sev.cls}>{sev.label}</span>
                    <span class="badge badge--neutral mono">{item.source.toUpperCase()}</span>
                    <span class="mono text-xs text-muted">{item.target}</span>
                    {#if item.notified}
                      <span class="badge badge--info" title="Alert berhasil diteruskan ke webhook">
                        <i class="ri-send-plane-line" aria-hidden="true"></i> Webhook
                      </span>
                    {/if}
                    <span class="mono text-xs text-subtle">{formatTime(item.created_at)}</span>
                  </div>
                  <div class="mon-alert-msg">{item.message}</div>
                </div>
                {#if !item.acknowledged}
                  <button
                    type="button"
                    class="btn btn--ghost btn--icon btn--sm"
                    aria-label="Tandai alert dibaca"
                    title="Tandai Dibaca"
                    onclick={() => onAck(item.id)}
                  >
                    <i class="ri-check-line" aria-hidden="true"></i>
                  </button>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>

    <!-- Konfigurasi Ambang Batas Alert & Status Webhook -->
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-equalizer-line" aria-hidden="true"></i>
          <span>Ambang Batas Alert Otomatis</span>
        </span>
        <span class={webhooksCount > 0 ? 'badge badge--success' : 'badge badge--warning'}>
          {webhooksCount} Webhook Aktif
        </span>
      </div>

      <form class="card__body stack" id="monitor-thresholds-form" onsubmit={submitThresholds}>
        {#if webhooksCount === 0}
          <div class="alert alert--info" role="status">
            <i class="ri-notification-off-line" aria-hidden="true"></i>
            <div class="grow">
              Belum ada Webhook (Telegram / Discord / Custom) terdaftar. Insiden tetap tercatat di sini, atau tambahkan Webhook di Pengaturan.
            </div>
            <a href="#/settings" class="btn btn--secondary btn--sm">Atur Webhook</a>
          </div>
        {/if}

        <p class="text-xs text-muted">
          Mesin evaluasi memeriksa metrik setiap 60 detik dengan hysteresis pemulihan 5% agar tidak terjadi spam notifikasi. Nilai ini tersinkron otomatis dengan halaman Pengaturan.
        </p>

        <div class="mon-form-grid">
          <div class="field">
            <label class="field__label" for="th-cpu">Ambang CPU (%)</label>
            <input id="th-cpu" type="number" min="50" max="99" class="input input--mono" bind:value={cpuPct} />
          </div>
          <div class="field">
            <label class="field__label" for="th-ram">Ambang RAM (%)</label>
            <input id="th-ram" type="number" min="50" max="99" class="input input--mono" bind:value={ramPct} />
          </div>
          <div class="field">
            <label class="field__label" for="th-disk">Ambang Disk Root (%)</label>
            <input id="th-disk" type="number" min="50" max="99" class="input input--mono" bind:value={diskPct} />
          </div>
          <div class="field">
            <label class="field__label" for="th-ssl">Peringatan SSL (Hari)</label>
            <input id="th-ssl" type="number" min="1" max="90" class="input input--mono" bind:value={sslDays} />
          </div>
        </div>

        <div class="field">
          <label class="field__label" for="th-cooldown">Jeda Cooldown Antar Alert (Menit)</label>
          <input id="th-cooldown" type="number" min="1" max="1440" class="input input--mono" bind:value={cooldownMin} />
          <span class="field__hint">Jeda minimal sebelum alert berulang dikirim lagi untuk sumber yang sama (default 15 menit).</span>
        </div>
      </form>

      <div class="card__foot">
        <a href="#/settings" class="btn btn--secondary btn--sm">Kelola Webhook</a>
        <button type="submit" form="monitor-thresholds-form" class="btn btn--primary btn--sm" disabled={savingThresholds}>
          <i class={savingThresholds ? 'ri-loader-4-line spin' : 'ri-save-line'} aria-hidden="true"></i>
          <span>{savingThresholds ? 'Menyimpan...' : 'Simpan Ambang Alert'}</span>
        </button>
      </div>
    </div>
  </div>
</div>
