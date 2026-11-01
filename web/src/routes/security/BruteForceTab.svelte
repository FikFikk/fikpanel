<script lang="ts">
  import type {
    ProtectionOverview,
    BruteForceConfig,
    WhitelistIP,
    BlacklistIP,
    FailedLoginEvent,
  } from '../../lib/types';
  import '../../styles/pages/security/bruteforce.css';

  export type BruteForceSubTab = 'config' | 'whitelist' | 'blacklist' | 'history';

  interface Props {
    protection: ProtectionOverview | null;
    bruteCfg: BruteForceConfig | null;
    bruteSubTab: BruteForceSubTab;
    whitelist: WhitelistIP[];
    blacklist: BlacklistIP[];
    failedEvents: FailedLoginEvent[];
    onSelectSubTab: (tab: BruteForceSubTab) => void;
    onWhitelistCurrentIP: () => void;
    onSaveBruteConfig: () => void;
    onOpenAddWhitelist: () => void;
    onRemoveWhitelist: (ip: string) => void;
    onOpenAddBlacklist: () => void;
    onRemoveBlacklist: (ip: string) => void;
  }

  let {
    protection,
    bruteCfg = $bindable(),
    bruteSubTab,
    whitelist,
    blacklist,
    failedEvents,
    onSelectSubTab,
    onWhitelistCurrentIP,
    onSaveBruteConfig,
    onOpenAddWhitelist,
    onRemoveWhitelist,
    onOpenAddBlacklist,
    onRemoveBlacklist,
  }: Props = $props();
</script>

<div class="pg-security-bruteforce tab-pane">
  <!-- Peringatan IP Belum Masuk Whitelist -->
  {#if protection && !protection.is_client_whitelisted}
    <div class="alert alert--danger mb-4">
      <i class="ri-error-warning-fill"></i>
      <div class="flex-1">
        Your current IP address [<strong class="mono">{protection.client_ip}</strong>] is not on the whitelist.
      </div>
      <button class="btn btn--primary btn--sm" onclick={onWhitelistCurrentIP}>
        Add to Whitelist
      </button>
    </div>
  {/if}

  <!-- Master Switch Proteksi Brute Force -->
  <div class="card p-4 mb-4 flex justify-between items-center">
    <div>
      <h3 class="font-bold text-md">FikPanel Brute Force Protection</h3>
      <p class="text-subtle text-xs mt-1">Proteksi otomatis terhadap serangan login berulang pada akun panel dan port SSH</p>
    </div>
    <label class="switch" aria-label="Toggle Master Brute Force">
      <input
        type="checkbox"
        role="switch"
        checked={bruteCfg ? bruteCfg.enabled : true}
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
  <div class="tabs mb-4">
    <button
      class="tabs__tab"
      class:is-active={bruteSubTab === 'config'}
      onclick={() => onSelectSubTab('config')}
    >
      Configuration
    </button>
    <button
      class="tabs__tab"
      class:is-active={bruteSubTab === 'whitelist'}
      onclick={() => onSelectSubTab('whitelist')}
    >
      WhiteList
    </button>
    <button
      class="tabs__tab"
      class:is-active={bruteSubTab === 'blacklist'}
      onclick={() => onSelectSubTab('blacklist')}
    >
      BlackList
    </button>
    <button
      class="tabs__tab"
      class:is-active={bruteSubTab === 'history'}
      onclick={() => onSelectSubTab('history')}
    >
      History Reports
    </button>
  </div>

  <!-- Subtab 1: Configuration -->
  {#if bruteSubTab === 'config' && bruteCfg}
    <div class="card p-4">
      <!-- Username-based Protection -->
      <div class="brute-section">
        <h4 class="font-bold text-sm">Username-based Protection (Based on fikpanel)</h4>
        <p class="text-subtle text-xs mt-1">
          Username-based protection tracks login attempts for user accounts. When disabled, will not lock user accounts, but existing account locks will remain.
        </p>
        <div class="flex items-center gap-3 mt-3">
          <span class="text-xs text-muted">Status:</span>
          <label class="switch" aria-label="Toggle Username Protection">
            <input type="checkbox" role="switch" bind:checked={bruteCfg.username_protection} />
            <span class="switch__track"></span>
          </label>
        </div>
      </div>

      <div class="fw-divider my-4"></div>

      <!-- IP Address-based Protection -->
      <div class="brute-section">
        <h4 class="font-bold text-sm">IP Address-based Protection (Based on ssh & panel)</h4>
        <p class="text-subtle text-xs mt-1">
          IP Address-based protection tracks login attempts from specific IP addresses. When disabled, will not block IP addresses, but existing blocks will remain.
        </p>
        <div class="flex items-center gap-3 mt-3">
          <span class="text-xs text-muted">Status:</span>
          <label class="switch" aria-label="Toggle IP Protection">
            <input type="checkbox" role="switch" bind:checked={bruteCfg.ip_protection} />
            <span class="switch__track"></span>
          </label>
        </div>
      </div>

      <div class="fw-divider my-4"></div>

      <!-- Login History Retention -->
      <div class="brute-section">
        <h4 class="font-bold text-sm">Login History</h4>
        <p class="text-subtle text-xs mt-1">Duration for Retaining Failed Logins (in minutes)</p>
        <div class="flex items-center gap-2 mt-2">
          <input
            type="number"
            class="input input--sm mono input-retention-w"
            bind:value={bruteCfg.retention_minutes}
          />
          <button class="btn btn--primary btn--sm" onclick={onSaveBruteConfig}>Save Configuration</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Subtab 2: WhiteList -->
  {#if bruteSubTab === 'whitelist'}
    <div class="card p-4">
      <div class="flex justify-between items-center mb-4">
        <span class="font-bold text-sm">Daftar Putih IP (Tidak Pernah Diblokir)</span>
        <button class="btn btn--primary btn--sm" onclick={onOpenAddWhitelist}>
          <i class="ri-add-line"></i> Add to Whitelist
        </button>
      </div>
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>IP Address</th>
              <th>Remarks</th>
              <th>Added Time</th>
              <th class="text-right">Operate</th>
            </tr>
          </thead>
          <tbody>
            {#if whitelist.length === 0}
              <tr><td colspan="4" class="text-center text-muted p-4">Belum ada IP pada daftar putih.</td></tr>
            {:else}
              {#each whitelist as w (w.ip)}
                <tr>
                  <td class="mono font-bold text-accent">{w.ip}</td>
                  <td class="text-subtle">{w.remarks}</td>
                  <td class="mono text-subtle">{new Date(w.added_at * 1000).toLocaleString('id-ID')}</td>
                  <td class="text-right">
                    <button class="btn btn--ghost btn--sm text-danger" onclick={() => onRemoveWhitelist(w.ip)}>
                      Remove
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

  <!-- Subtab 3: BlackList -->
  {#if bruteSubTab === 'blacklist'}
    <div class="card p-4">
      <div class="flex justify-between items-center mb-4">
        <span class="font-bold text-sm">Daftar Hitam IP (Diblokir Sistem)</span>
        <button class="btn btn--danger btn--sm" onclick={onOpenAddBlacklist}>
          <i class="ri-forbid-line"></i> Add to Blacklist
        </button>
      </div>
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>IP Address</th>
              <th>Reason</th>
              <th>Blocked At</th>
              <th>Expires At</th>
              <th class="text-right">Operate</th>
            </tr>
          </thead>
          <tbody>
            {#if blacklist.length === 0}
              <tr><td colspan="5" class="text-center text-muted p-4">Tidak ada IP yang sedang diblokir.</td></tr>
            {:else}
              {#each blacklist as b (b.ip)}
                <tr>
                  <td class="mono font-bold text-danger">{b.ip}</td>
                  <td class="text-subtle">{b.reason}</td>
                  <td class="mono text-subtle">{new Date(b.added_at * 1000).toLocaleString('id-ID')}</td>
                  <td class="mono text-subtle">{new Date(b.expires_at * 1000).toLocaleString('id-ID')}</td>
                  <td class="text-right">
                    <button class="btn btn--secondary btn--sm" onclick={() => onRemoveBlacklist(b.ip)}>
                      Unban
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

  <!-- Subtab 4: History Reports -->
  {#if bruteSubTab === 'history'}
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>Time</th>
            <th>Target User</th>
            <th>Source IP</th>
            <th>Origin</th>
            <th>Reason</th>
          </tr>
        </thead>
        <tbody>
          {#if failedEvents.length === 0}
            <tr><td colspan="5" class="text-center text-muted p-4">Tidak ada riwayat percobaan gagal tercatat.</td></tr>
          {:else}
            {#each failedEvents as ev}
              <tr>
                <td class="mono text-subtle">{new Date(ev.time * 1000).toLocaleString('id-ID')}</td>
                <td class="mono font-bold text-accent">{ev.username}</td>
                <td class="mono text-danger">{ev.ip}</td>
                <td><span class="badge badge--pill">{ev.source.toUpperCase()}</span></td>
                <td class="text-subtle">{ev.reason}</td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  {/if}
</div>
