<script lang="ts">
  import type {
    BruteForceConfig,
    SecurityIPRule,
    SecurityLoginEvent,
  } from '../../lib/types';
  import '../../styles/pages/security/bruteforce.css';

  export type BruteForceSubTab = 'config' | 'whitelist' | 'blacklist' | 'history';

  interface Props {
    clientIP: string;
    isClientWhitelisted: boolean;
    bruteCfg: BruteForceConfig | null;
    bruteSubTab: BruteForceSubTab;
    whitelist: SecurityIPRule[];
    blacklist: SecurityIPRule[];
    loginEvents: SecurityLoginEvent[];
    busyKey: string;
    onSelectSubTab: (tab: BruteForceSubTab) => void;
    onWhitelistCurrentIP: () => void;
    onSaveBruteConfig: () => void;
    onOpenAddWhitelist: () => void;
    onRemoveWhitelist: (rule: SecurityIPRule) => void;
    onOpenAddBlacklist: () => void;
    onRemoveBlacklist: (rule: SecurityIPRule) => void;
  }

  let {
    clientIP,
    isClientWhitelisted,
    bruteCfg = $bindable(),
    bruteSubTab,
    whitelist,
    blacklist,
    loginEvents,
    busyKey,
    onSelectSubTab,
    onWhitelistCurrentIP,
    onSaveBruteConfig,
    onOpenAddWhitelist,
    onRemoveWhitelist,
    onOpenAddBlacklist,
    onRemoveBlacklist,
  }: Props = $props();

  let historyFilter = $state<'all' | 'failed' | 'banned' | 'success'>('all');

  let filteredEvents = $derived.by(() => {
    if (historyFilter === 'all') return loginEvents;
    return loginEvents.filter((ev) => ev.status === historyFilter);
  });

  function formatUnix(ts: number): string {
    if (!ts) return 'Permanen';
    return new Date(ts * 1000).toLocaleString('id-ID');
  }

  function statusBadge(st: string): { cls: string; label: string } {
    switch (st) {
      case 'success':
        return { cls: 'badge badge--success', label: 'SUKSES' };
      case 'banned':
        return { cls: 'badge badge--danger', label: 'AUTO-BAN' };
      default:
        return { cls: 'badge badge--warning', label: 'GAGAL' };
    }
  }
</script>

<div class="pg-security-bruteforce">
  <!-- Peringatan IP Admin Belum Masuk Whitelist -->
  {#if clientIP && !isClientWhitelisted}
    <div class="alert alert--warning" role="status">
      <i class="ri-error-warning-fill" aria-hidden="true"></i>
      <div class="grow">
        Alamat IP Anda saat ini (<strong class="mono">{clientIP}</strong>) belum terdaftar di Whitelist. Tambahkan agar tidak terkena auto-ban bila salah mengetik kata sandi.
      </div>
      <button
        type="button"
        class="btn btn--primary btn--sm"
        disabled={busyKey === 'bf:wl-self'}
        onclick={onWhitelistCurrentIP}
      >
        {busyKey === 'bf:wl-self' ? 'Menyimpan...' : 'Whitelist IP Saya'}
      </button>
    </div>
  {/if}

  <!-- Master Switch Proteksi Brute Force -->
  <div class="card bf-master-card">
    <div class="bf-master-info">
      <div class="row">
        <span class="bf-master-title">FikPanel Brute Force &amp; IP Guard</span>
        <span class="badge {bruteCfg?.enabled ? 'badge--success' : 'badge--warning'}">
          {bruteCfg?.enabled ? 'Aktif & Menegakkan' : 'Nonaktif'}
        </span>
      </div>
      <span class="bf-master-desc">
        Terhubung langsung ke endpoint login dan middleware HTTP panel. Daftar IP dan riwayat tersimpan persisten di SQLite.
      </span>
    </div>
    <label class="switch" aria-label="Aktifkan Proteksi Brute Force">
      <input
        type="checkbox"
        role="switch"
        checked={bruteCfg?.enabled ?? true}
        disabled={busyKey === 'bf:save'}
        onchange={() => {
          if (bruteCfg) {
            bruteCfg.enabled = !bruteCfg.enabled;
            onSaveBruteConfig();
          }
        }}
      />
      <span class="switch__track"></span>
    </label>
  </div>

  <!-- Sub-tabs Brute Force -->
  <div class="tabs tabs--underline" role="tablist">
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={bruteSubTab === 'config'}
      aria-selected={bruteSubTab === 'config'}
      onclick={() => onSelectSubTab('config')}
    >
      <i class="ri-equalizer-line" aria-hidden="true"></i>
      <span>Konfigurasi</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={bruteSubTab === 'whitelist'}
      aria-selected={bruteSubTab === 'whitelist'}
      onclick={() => onSelectSubTab('whitelist')}
    >
      <i class="ri-checkbox-circle-line" aria-hidden="true"></i>
      <span>Whitelist ({whitelist.length})</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={bruteSubTab === 'blacklist'}
      aria-selected={bruteSubTab === 'blacklist'}
      onclick={() => onSelectSubTab('blacklist')}
    >
      <i class="ri-forbid-2-line" aria-hidden="true"></i>
      <span>Blacklist ({blacklist.length})</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={bruteSubTab === 'history'}
      aria-selected={bruteSubTab === 'history'}
      onclick={() => onSelectSubTab('history')}
    >
      <i class="ri-history-line" aria-hidden="true"></i>
      <span>Riwayat Login ({loginEvents.length})</span>
    </button>
  </div>

  <!-- SUBTAB 1: Konfigurasi Lengkap -->
  {#if bruteSubTab === 'config' && bruteCfg}
    <div class="card">
      <div class="card__body stack">
        <div class="bf-config-grid">
          <!-- Proteksi Berbasis Username -->
          <div class="bf-panel-box">
            <div class="bf-panel-head">
              <span class="bf-panel-title">Proteksi Berbasis Username</span>
              <label class="switch" aria-label="Aktifkan proteksi berbasis username">
                <input type="checkbox" role="switch" bind:checked={bruteCfg.username_protection} />
                <span class="switch__track"></span>
              </label>
            </div>
            <p class="text-xs text-muted">
              Mengunci sementara percobaan login pada username yang gagal berulang kali dalam jendela 15 menit.
            </p>
            <div class="field">
              <label class="field__label" for="bf-user-retries">Batas Maksimal Gagal per Username</label>
              <input
                id="bf-user-retries"
                type="number"
                min="3"
                max="50"
                class="input input--mono"
                disabled={!bruteCfg.username_protection}
                bind:value={bruteCfg.username_max_retries}
              />
            </div>
          </div>

          <!-- Proteksi Berbasis IP -->
          <div class="bf-panel-box">
            <div class="bf-panel-head">
              <span class="bf-panel-title">Proteksi Berbasis Alamat IP</span>
              <label class="switch" aria-label="Aktifkan proteksi berbasis IP">
                <input type="checkbox" role="switch" bind:checked={bruteCfg.ip_protection} />
                <span class="switch__track"></span>
              </label>
            </div>
            <p class="text-xs text-muted">
              Memasukkan alamat IP penyerang ke Blacklist secara otomatis apabila melampaui ambang gagal login.
            </p>
            <div class="bf-input-row">
              <div class="field">
                <label class="field__label" for="bf-ip-retries">Maks Gagal per IP</label>
                <input
                  id="bf-ip-retries"
                  type="number"
                  min="3"
                  max="100"
                  class="input input--mono"
                  disabled={!bruteCfg.ip_protection}
                  bind:value={bruteCfg.ip_max_retries}
                />
              </div>
              <div class="field">
                <label class="field__label" for="bf-ip-ban-dur">Durasi Auto-Ban (Menit)</label>
                <input
                  id="bf-ip-ban-dur"
                  type="number"
                  min="1"
                  max="43200"
                  class="input input--mono"
                  disabled={!bruteCfg.ip_protection}
                  bind:value={bruteCfg.ip_ban_duration_minutes}
                />
              </div>
            </div>
          </div>
        </div>

        <!-- Opsi Retensi, UFW Sync & Notifikasi Webhook -->
        <div class="sec-setting-list">
          <div class="sec-setting-row">
            <div class="sec-setting-info">
              <span class="sec-setting-title">Retensi Riwayat Login (Menit)</span>
              <span class="sec-setting-desc">Lama penyimpanan log percobaan login keamanan sebelum dipangkas otomatis.</span>
            </div>
            <div class="sec-setting-ctrl">
              <input
                type="number"
                min="15"
                max="43200"
                class="input input--mono"
                aria-label="Retensi riwayat login dalam menit"
                bind:value={bruteCfg.retention_minutes}
              />
            </div>
          </div>

          <div class="sec-setting-row">
            <div class="sec-setting-info">
              <span class="sec-setting-title">Auto-Sinkron Ban IP ke Firewall UFW (Linux)</span>
              <span class="sec-setting-desc">
                Saat IP terkena auto-ban, otomatis menyisipkan aturan <code>ufw insert 1 deny from IP</code> (nonaktif secara default demi keamanan).
              </span>
            </div>
            <div class="sec-setting-ctrl">
              <label class="switch" aria-label="Sinkronkan auto-ban ke UFW">
                <input type="checkbox" role="switch" bind:checked={bruteCfg.auto_sync_ufw} />
                <span class="switch__track"></span>
              </label>
            </div>
          </div>

          <div class="sec-setting-row">
            <div class="sec-setting-info">
              <span class="sec-setting-title">Webhook Alert saat Auto-Ban Brute Force</span>
              <span class="sec-setting-desc">
                Kirim notifikasi instan ke seluruh channel Webhook aktif (Discord/Telegram/Slack) saat IP diblokir otomatis.
              </span>
            </div>
            <div class="sec-setting-ctrl">
              <label class="switch" aria-label="Notifikasi webhook saat auto-ban">
                <input type="checkbox" role="switch" bind:checked={bruteCfg.notify_on_ban} />
                <span class="switch__track"></span>
              </label>
            </div>
          </div>

          <div class="sec-setting-row">
            <div class="sec-setting-info">
              <span class="sec-setting-title">Webhook Alert saat Login Panel Baru</span>
              <span class="sec-setting-desc">
                Kirim peringatan setiap kali ada autentikasi sukses baru ke dasbor FikPanel.
              </span>
            </div>
            <div class="sec-setting-ctrl">
              <label class="switch" aria-label="Notifikasi webhook saat login panel baru">
                <input type="checkbox" role="switch" bind:checked={bruteCfg.notify_on_login} />
                <span class="switch__track"></span>
              </label>
            </div>
          </div>
        </div>
      </div>

      <div class="card__foot">
        <button
          type="button"
          class="btn btn--primary"
          disabled={busyKey === 'bf:save'}
          onclick={onSaveBruteConfig}
        >
          <i class="ri-save-line" aria-hidden="true"></i>
          <span>{busyKey === 'bf:save' ? 'Menyimpan...' : 'Simpan Konfigurasi'}</span>
        </button>
      </div>
    </div>
  {/if}

  <!-- SUBTAB 2: Whitelist -->
  {#if bruteSubTab === 'whitelist'}
    <div class="card">
      <div class="sec-toolbar">
        <div class="sec-toolbar-left">
          <span class="card__title">Daftar Putih IP / CIDR (Bebas Rate-Limit &amp; Auto-Ban)</span>
        </div>
        <div class="sec-toolbar-right">
          <button type="button" class="btn btn--primary btn--sm" onclick={onOpenAddWhitelist}>
            <i class="ri-add-line" aria-hidden="true"></i>
            <span>Tambah Whitelist</span>
          </button>
        </div>
      </div>

      <div class="table-wrap">
        <table class="table table--dense">
          <thead>
            <tr>
              <th>Alamat IP / CIDR</th>
              <th>Keterangan</th>
              <th>Sumber</th>
              <th>Ditambahkan</th>
              <th class="col-right">Operasi</th>
            </tr>
          </thead>
          <tbody>
            {#if whitelist.length === 0}
              <tr>
                <td colspan="5">
                  <div class="empty-state">
                    <i class="ri-checkbox-circle-line" aria-hidden="true"></i>
                    <div class="empty-state__title">Belum Ada Entri Whitelist</div>
                  </div>
                </td>
              </tr>
            {:else}
              {#each whitelist as w (w.id)}
                <tr>
                  <td class="mono"><strong>{w.ip}</strong></td>
                  <td class="text-muted">{w.remarks || '-'}</td>
                  <td><span class="badge badge--neutral">{w.source}</span></td>
                  <td class="mono text-subtle">{formatUnix(w.added_at)}</td>
                  <td class="col-right">
                    <button
                      type="button"
                      class="btn btn--ghost btn--sm"
                      disabled={w.source === 'system' || busyKey === `bf:del-wl:${w.id}`}
                      onclick={() => onRemoveWhitelist(w)}
                    >
                      {w.source === 'system' ? 'Bawaan' : 'Hapus'}
                    </button>
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- SUBTAB 3: Blacklist -->
  {#if bruteSubTab === 'blacklist'}
    <div class="card">
      <div class="sec-toolbar">
        <div class="sec-toolbar-left">
          <span class="card__title">Daftar Hitam IP / CIDR (Ditolak Aktif oleh Middleware)</span>
        </div>
        <div class="sec-toolbar-right">
          <button type="button" class="btn btn--danger btn--sm" onclick={onOpenAddBlacklist}>
            <i class="ri-forbid-2-line" aria-hidden="true"></i>
            <span>Blokir IP Manual</span>
          </button>
        </div>
      </div>

      <div class="table-wrap">
        <table class="table table--dense">
          <thead>
            <tr>
              <th>Alamat IP / CIDR</th>
              <th>Alasan</th>
              <th>Cakupan</th>
              <th>Diblokir Sejak</th>
              <th>Kedaluwarsa</th>
              <th class="col-right">Operasi</th>
            </tr>
          </thead>
          <tbody>
            {#if blacklist.length === 0}
              <tr>
                <td colspan="6">
                  <div class="empty-state">
                    <i class="ri-shield-check-line" aria-hidden="true"></i>
                    <div class="empty-state__title">Tidak Ada IP Diblokir Saat Ini</div>
                    <p class="state__desc">
                      IP penyerang yang gagal login berulang atau diblokir manual akan tampil di sini.
                    </p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each blacklist as b (b.id)}
                <tr>
                  <td class="mono"><strong>{b.ip}</strong></td>
                  <td class="text-muted">{b.remarks || '-'}</td>
                  <td>
                    <span class="badge {b.sync_ufw ? 'badge--danger' : 'badge--warning'}">
                      {b.sync_ufw ? 'Panel + UFW' : 'Panel HTTP'}
                    </span>
                  </td>
                  <td class="mono text-subtle">{formatUnix(b.added_at)}</td>
                  <td class="mono text-subtle">{formatUnix(b.expires_at)}</td>
                  <td class="col-right">
                    <button
                      type="button"
                      class="btn btn--secondary btn--sm"
                      disabled={busyKey === `bf:del-bl:${b.id}`}
                      onclick={() => onRemoveBlacklist(b)}
                    >
                      Cabut Blokir
                    </button>
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- SUBTAB 4: Riwayat Login Persisten -->
  {#if bruteSubTab === 'history'}
    <div class="card">
      <div class="sec-toolbar">
        <div class="sec-toolbar-left">
          <div class="segmented">
            <button type="button" class:active={historyFilter === 'all'} onclick={() => (historyFilter = 'all')}>
              Semua ({loginEvents.length})
            </button>
            <button type="button" class:active={historyFilter === 'failed'} onclick={() => (historyFilter = 'failed')}>
              Gagal
            </button>
            <button type="button" class:active={historyFilter === 'banned'} onclick={() => (historyFilter = 'banned')}>
              Auto-Ban
            </button>
            <button type="button" class:active={historyFilter === 'success'} onclick={() => (historyFilter = 'success')}>
              Sukses
            </button>
          </div>
        </div>
      </div>

      <div class="table-wrap">
        <table class="table table--dense">
          <thead>
            <tr>
              <th>Waktu</th>
              <th>Status</th>
              <th>Username</th>
              <th>IP Sumber</th>
              <th>Sumber</th>
              <th>Keterangan</th>
            </tr>
          </thead>
          <tbody>
            {#if filteredEvents.length === 0}
              <tr>
                <td colspan="6">
                  <div class="empty-state">
                    <i class="ri-history-line" aria-hidden="true"></i>
                    <div class="empty-state__title">Belum Ada Catatan Autentikasi</div>
                    <p class="state__desc">
                      Percobaan login panel (sukses, gagal, maupun auto-ban) dicatat secara real-time ke SQLite.
                    </p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each filteredEvents as ev (ev.id)}
                {@const badge = statusBadge(ev.status)}
                <tr>
                  <td class="mono text-subtle">{formatUnix(ev.time)}</td>
                  <td><span class={badge.cls}>{badge.label}</span></td>
                  <td class="mono"><strong>{ev.username || '-'}</strong></td>
                  <td class="mono">{ev.ip}</td>
                  <td><span class="badge badge--neutral">{ev.source.toUpperCase()}</span></td>
                  <td class="text-muted">{ev.reason}</td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  {/if}
</div>
