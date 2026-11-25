<script lang="ts">
  import type { FirewallExtendedStatus, FirewallRule, FirewallListeningPort } from '../../lib/types';
  import '../../styles/pages/security/firewall.css';

  export type FirewallSubTab = 'inbound' | 'outbound' | 'listening' | 'settings';

  interface Props {
    fwStatus: FirewallExtendedStatus | null;
    fwSubTab: FirewallSubTab;
    fwSearch: string;
    selectedRuleNums: number[];
    busyKey: string;
    onSelectSubTab: (tab: FirewallSubTab) => void;
    onRefresh: () => void;
    onToggleFirewall: () => void;
    onToggleICMP: () => void;
    onClearSiteLogs: () => void;
    onSaveDefaultPolicy: (incoming: string, outgoing: string) => void;
    onOpenAddModal: (preset?: { port?: string; protocol?: string; direction?: string; remarks?: string }) => void;
    onDeleteRule: (rule: FirewallRule) => void;
    onBatchDeleteRules: (numbers: number[]) => void;
  }

  let {
    fwStatus,
    fwSubTab,
    fwSearch = $bindable(),
    selectedRuleNums = $bindable(),
    busyKey,
    onSelectSubTab,
    onRefresh,
    onToggleFirewall,
    onToggleICMP,
    onClearSiteLogs,
    onSaveDefaultPolicy,
    onOpenAddModal,
    onDeleteRule,
    onBatchDeleteRules,
  }: Props = $props();

  let actionFilter = $state<'all' | 'ALLOW' | 'DENY' | 'LIMIT'>('all');
  let defaultIncoming = $state('deny');
  let defaultOutgoing = $state('allow');

  // Sinkronkan pilihan kebijakan default saat status UFW diperbarui
  $effect(() => {
    if (fwStatus) {
      if (fwStatus.default_incoming && fwStatus.default_incoming !== 'unknown') {
        defaultIncoming = fwStatus.default_incoming;
      }
      if (fwStatus.default_outgoing && fwStatus.default_outgoing !== 'unknown') {
        defaultOutgoing = fwStatus.default_outgoing;
      }
    }
  });

  let filteredRules = $derived.by(() => {
    const list = fwStatus?.rules || [];
    const targetDir = fwSubTab === 'outbound' ? 'outbound' : 'inbound';
    const q = fwSearch.trim().toLowerCase();

    return list.filter((r) => {
      if (r.direction.toLowerCase() !== targetDir) return false;
      if (actionFilter !== 'all' && r.action !== actionFilter) return false;
      if (q) {
        return (
          r.port.toLowerCase().includes(q) ||
          r.service.toLowerCase().includes(q) ||
          r.from.toLowerCase().includes(q) ||
          r.remarks.toLowerCase().includes(q) ||
          r.protocol.toLowerCase().includes(q)
        );
      }
      return true;
    });
  });

  let filteredListening = $derived.by(() => {
    const list = fwStatus?.listening_ports || [];
    const q = fwSearch.trim().toLowerCase();
    if (!q) return list;
    return list.filter(
      (p) =>
        p.port.toLowerCase().includes(q) ||
        p.process.toLowerCase().includes(q) ||
        p.service.toLowerCase().includes(q) ||
        p.address.toLowerCase().includes(q)
    );
  });

  let allVisibleSelected = $derived(
    filteredRules.length > 0 && filteredRules.every((r) => selectedRuleNums.includes(r.number))
  );

  function toggleSelectAll() {
    if (allVisibleSelected) {
      selectedRuleNums = [];
    } else {
      selectedRuleNums = filteredRules.map((r) => r.number);
    }
  }

  function toggleSelectOne(num: number) {
    if (selectedRuleNums.includes(num)) {
      selectedRuleNums = selectedRuleNums.filter((n) => n !== num);
    } else {
      selectedRuleNums = [...selectedRuleNums, num];
    }
  }

  function actionBadgeClass(action: string): string {
    switch (action) {
      case 'ALLOW':
        return 'badge badge--success';
      case 'DENY':
      case 'REJECT':
        return 'badge badge--danger';
      case 'LIMIT':
        return 'badge badge--warning';
      default:
        return 'badge badge--neutral';
    }
  }

  function prefillFromListening(lp: FirewallListeningPort) {
    const proto = lp.protocol.startsWith('udp') ? 'udp' : 'tcp';
    onOpenAddModal({
      port: lp.port,
      protocol: proto,
      direction: 'in',
      remarks: lp.process ? `Layanan ${lp.process}` : lp.service,
    });
  }
</script>

<div class="pg-security-firewall">
  <!-- Ringkasan Host Firewall Real (Tanpa Metadata Palsu) -->
  <div class="sec-kpi-grid">
    <div class="stat-card">
      <div class="stat-card__icon" class:stat-card__icon--danger={!fwStatus?.enabled}>
        <i class="ri-shield-flash-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Status UFW Host</span>
        <span class="stat-card__value">{fwStatus?.enabled ? 'Aktif' : 'Nonaktif'}</span>
        <span class="stat-card__sub mono">{fwStatus?.hostname || '-'}</span>
      </div>
      <label class="switch" aria-label="Aktifkan atau nonaktifkan UFW Firewall">
        <input
          type="checkbox"
          role="switch"
          checked={fwStatus?.enabled || false}
          disabled={busyKey === 'fw:toggle'}
          onchange={onToggleFirewall}
        />
        <span class="switch__track"></span>
      </label>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--info">
        <i class="ri-arrow-left-right-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Kebijakan Default</span>
        <span class="stat-card__value mono">
          {(fwStatus?.default_incoming || 'deny').toUpperCase()} / {(fwStatus?.default_outgoing || 'allow').toUpperCase()}
        </span>
        <span class="stat-card__sub">Masuk (In) / Keluar (Out)</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--violet">
        <i class="ri-list-settings-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Aturan Terdaftar</span>
        <span class="stat-card__value">{(fwStatus?.inbound_count || 0) + (fwStatus?.outbound_count || 0)}</span>
        <span class="stat-card__sub">
          {fwStatus?.inbound_count || 0} Inbound · {fwStatus?.outbound_count || 0} Outbound
        </span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--warning">
        <i class="ri-radar-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Port Aktif (Listening)</span>
        <span class="stat-card__value">{fwStatus?.listening_count || 0}</span>
        <span class="stat-card__sub">
          SSH :{fwStatus?.ssh_port || 22} · Panel :{fwStatus?.panel_port || 1405}
        </span>
      </div>
    </div>
  </div>

  {#if fwStatus && fwStatus.is_linux && !fwStatus.installed}
    <div class="alert alert--warning" role="alert">
      <i class="ri-error-warning-line" aria-hidden="true"></i>
      <div>
        Paket <code>ufw</code> belum terpasang pada server Linux ini. Jalankan <code>apt install ufw</code> melalui Terminal agar manajemen aturan firewall aktif.
      </div>
    </div>
  {/if}

  <!-- Sub-tab Firewall -->
  <div class="tabs tabs--underline" role="tablist">
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={fwSubTab === 'inbound'}
      aria-selected={fwSubTab === 'inbound'}
      onclick={() => { selectedRuleNums = []; onSelectSubTab('inbound'); }}
    >
      <i class="ri-login-box-line" aria-hidden="true"></i>
      <span>Aturan Inbound ({fwStatus?.inbound_count || 0})</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={fwSubTab === 'outbound'}
      aria-selected={fwSubTab === 'outbound'}
      onclick={() => { selectedRuleNums = []; onSelectSubTab('outbound'); }}
    >
      <i class="ri-logout-box-line" aria-hidden="true"></i>
      <span>Aturan Outbound ({fwStatus?.outbound_count || 0})</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={fwSubTab === 'listening'}
      aria-selected={fwSubTab === 'listening'}
      onclick={() => onSelectSubTab('listening')}
    >
      <i class="ri-broadcast-line" aria-hidden="true"></i>
      <span>Port Listening ({fwStatus?.listening_count || 0})</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={fwSubTab === 'settings'}
      aria-selected={fwSubTab === 'settings'}
      onclick={() => onSelectSubTab('settings')}
    >
      <i class="ri-equalizer-line" aria-hidden="true"></i>
      <span>Kebijakan &amp; Log</span>
    </button>
  </div>

  <!-- SUBTAB 1 & 2: Tabel Aturan Inbound / Outbound -->
  {#if fwSubTab === 'inbound' || fwSubTab === 'outbound'}
    <div class="card">
      <div class="sec-toolbar">
        <div class="sec-toolbar-left">
          <div class="input-wrap sec-search">
            <i class="ri-search-line input__icon" aria-hidden="true"></i>
            <input
              type="text"
              class="input input--search"
              placeholder="Cari port, layanan, IP, catatan..."
              bind:value={fwSearch}
            />
          </div>
          <select class="select" aria-label="Filter aksi aturan" bind:value={actionFilter}>
            <option value="all">Semua Aksi</option>
            <option value="ALLOW">ALLOW</option>
            <option value="DENY">DENY</option>
            <option value="LIMIT">LIMIT</option>
          </select>
        </div>

        <div class="sec-toolbar-right">
          {#if selectedRuleNums.length > 0}
            <button
              type="button"
              class="btn btn--danger btn--sm"
              disabled={busyKey === 'fw:batch-del'}
              onclick={() => onBatchDeleteRules(selectedRuleNums)}
            >
              <i class="ri-delete-bin-line" aria-hidden="true"></i>
              <span>Hapus Terpilih ({selectedRuleNums.length})</span>
            </button>
          {/if}
          <button
            type="button"
            class="btn btn--ghost btn--icon btn--sm"
            title="Muat ulang daftar aturan"
            aria-label="Muat ulang daftar aturan"
            onclick={onRefresh}
          >
            <i class="ri-refresh-line" aria-hidden="true"></i>
          </button>
          <button
            type="button"
            class="btn btn--primary btn--sm"
            onclick={() => onOpenAddModal({ direction: fwSubTab === 'outbound' ? 'out' : 'in' })}
          >
            <i class="ri-add-line" aria-hidden="true"></i>
            <span>Tambah Aturan</span>
          </button>
        </div>
      </div>

      <div class="table-wrap">
        <table class="table table--dense">
          <thead>
            <tr>
              <th class="fw-col-check">
                <input
                  type="checkbox"
                  checked={allVisibleSelected}
                  onchange={toggleSelectAll}
                  aria-label="Pilih semua aturan"
                />
              </th>
              <th class="fw-col-num">#</th>
              <th>Aksi</th>
              <th>Port &amp; Layanan</th>
              <th>Protokol</th>
              <th>Versi</th>
              <th>{fwSubTab === 'outbound' ? 'Tujuan' : 'Sumber IP'}</th>
              <th>Status Soket</th>
              <th>Catatan</th>
              <th class="col-right">Operasi</th>
            </tr>
          </thead>
          <tbody>
            {#if filteredRules.length === 0}
              <tr>
                <td colspan="10">
                  <div class="empty-state">
                    <i class="ri-shield-line" aria-hidden="true"></i>
                    <div class="empty-state__title">
                      Belum Ada Aturan {fwSubTab === 'outbound' ? 'Outbound' : 'Inbound'}
                    </div>
                    <p class="state__desc">
                      {fwSubTab === 'outbound'
                        ? `Trafik keluar saat ini mengikuti kebijakan default (${(fwStatus?.default_outgoing || 'allow').toUpperCase()}).`
                        : 'Tambahkan aturan port atau ubah kata kunci pencarian untuk menampilkan aturan UFW.'}
                    </p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each filteredRules as r (r.number)}
                <tr aria-selected={selectedRuleNums.includes(r.number)}>
                  <td>
                    <input
                      type="checkbox"
                      checked={selectedRuleNums.includes(r.number)}
                      onchange={() => toggleSelectOne(r.number)}
                      aria-label={`Pilih aturan nomor ${r.number}`}
                    />
                  </td>
                  <td class="mono text-subtle">{r.number}</td>
                  <td>
                    <span class={actionBadgeClass(r.action)}>{r.action}</span>
                  </td>
                  <td>
                    <div class="fw-port-cell">
                      <span class="fw-port-num">{r.port}</span>
                      <span class="badge badge--neutral">{r.service}</span>
                      {#if r.protected}
                        <i
                          class="ri-lock-2-fill fw-protected-icon"
                          title="Port kritikal (SSH / FikPanel) dilindungi dari penghapusan tidak sengaja"
                        ></i>
                      {/if}
                    </div>
                  </td>
                  <td class="mono">{r.protocol.toUpperCase()}</td>
                  <td class="mono text-subtle">{r.ip_version}</td>
                  <td class="mono">{r.from}</td>
                  <td>
                    {#if r.listening}
                      <span class="badge badge--success" title={r.process ? `Proses: ${r.process}` : 'Soket aktif'}>
                        <span class="dot dot--success"></span>
                        <span>{r.process ? `Listening (${r.process})` : 'Listening'}</span>
                      </span>
                    {:else}
                      <span class="badge badge--neutral">Idle</span>
                    {/if}
                  </td>
                  <td class="fw-remarks text-muted" title={r.remarks}>{r.remarks || '-'}</td>
                  <td class="col-right">
                    <button
                      type="button"
                      class="btn btn--ghost btn--sm"
                      disabled={busyKey === `fw:del:${r.number}`}
                      onclick={() => onDeleteRule(r)}
                    >
                      Hapus
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

  <!-- SUBTAB 3: Daftar Soket Listening Aktual (ss -tulnp) -->
  {#if fwSubTab === 'listening'}
    <div class="card">
      <div class="sec-toolbar">
        <div class="sec-toolbar-left">
          <div class="input-wrap sec-search">
            <i class="ri-search-line input__icon" aria-hidden="true"></i>
            <input
              type="text"
              class="input input--search"
              placeholder="Cari port, proses, alamat bind..."
              bind:value={fwSearch}
            />
          </div>
        </div>
        <div class="sec-toolbar-right">
          <button
            type="button"
            class="btn btn--secondary btn--sm"
            onclick={onRefresh}
          >
            <i class="ri-refresh-line" aria-hidden="true"></i>
            <span>Segarkan Soket</span>
          </button>
        </div>
      </div>

      <div class="table-wrap">
        <table class="table table--dense">
          <thead>
            <tr>
              <th>Port</th>
              <th>Protokol</th>
              <th>Alamat Bind</th>
              <th>Layanan</th>
              <th>Proses / PID</th>
              <th>Eksposur</th>
              <th class="col-right">Aksi Cepat</th>
            </tr>
          </thead>
          <tbody>
            {#if filteredListening.length === 0}
              <tr>
                <td colspan="7">
                  <div class="empty-state">
                    <i class="ri-broadcast-line" aria-hidden="true"></i>
                    <div class="empty-state__title">Tidak Ada Data Port Listening</div>
                    <p class="state__desc">
                      Inspeksi soket <code>ss -tulnp</code> hanya menampilkan layanan aktif pada host server Linux.
                    </p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each filteredListening as lp (`${lp.protocol}-${lp.address}-${lp.port}`)}
                <tr>
                  <td class="mono fw-port-num">{lp.port}</td>
                  <td class="mono">{lp.protocol.toUpperCase()}</td>
                  <td class="mono text-muted">{lp.address}</td>
                  <td><span class="badge badge--neutral">{lp.service}</span></td>
                  <td class="mono">
                    {lp.process || 'unknown'}{#if lp.pid > 0} <span class="text-subtle">({lp.pid})</span>{/if}
                  </td>
                  <td>
                    {#if lp.exposed_public}
                      <span class="badge badge--warning">Publik (All Interfaces)</span>
                    {:else}
                      <span class="badge badge--success">Lokal (Loopback)</span>
                    {/if}
                  </td>
                  <td class="col-right">
                    <button
                      type="button"
                      class="btn btn--secondary btn--sm"
                      onclick={() => prefillFromListening(lp)}
                    >
                      Buat Aturan
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

  <!-- SUBTAB 4: Pengaturan Kebijakan Default UFW, ICMP & Log Web -->
  {#if fwSubTab === 'settings'}
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-equalizer-line" aria-hidden="true"></i>
          <span>Kebijakan Default UFW</span>
        </span>
      </div>
      <div class="card__body">
        <div class="fw-policy-grid">
          <div class="field">
            <label class="field__label" for="fw-def-in">Default Incoming Policy</label>
            <select id="fw-def-in" class="select" bind:value={defaultIncoming}>
              <option value="deny">DENY — Tolak semua koneksi masuk kecuali diizinkan (Direkomendasikan)</option>
              <option value="reject">REJECT — Tolak dengan respons ICMP unreachable</option>
              <option value="allow">ALLOW — Izinkan semua koneksi masuk (Berisiko)</option>
            </select>
          </div>
          <div class="field">
            <label class="field__label" for="fw-def-out">Default Outgoing Policy</label>
            <select id="fw-def-out" class="select" bind:value={defaultOutgoing}>
              <option value="allow">ALLOW — Izinkan trafik keluar dari server (Direkomendasikan)</option>
              <option value="deny">DENY — Blokir trafik keluar kecuali diizinkan</option>
              <option value="reject">REJECT — Tolak trafik keluar secara eksplisit</option>
            </select>
          </div>
        </div>
        <div class="fw-policy-foot">
          <span class="text-xs text-muted">
            Saat UFW diaktifkan, port SSH ({fwStatus?.ssh_port || 22}) dan FikPanel ({fwStatus?.panel_port || 1405}) otomatis dijaga tetap terbuka.
          </span>
          <button
            type="button"
            class="btn btn--primary btn--sm"
            disabled={busyKey === 'fw:policy'}
            onclick={() => onSaveDefaultPolicy(defaultIncoming, defaultOutgoing)}
          >
            {busyKey === 'fw:policy' ? 'Menyimpan...' : 'Simpan Kebijakan Default'}
          </button>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-settings-4-line" aria-hidden="true"></i>
          <span>Kontrol Jaringan &amp; Pembersihan Log Web</span>
        </span>
      </div>
      <div class="card__body">
        <div class="sec-setting-list">
          <div class="sec-setting-row">
            <div class="sec-setting-info">
              <span class="sec-setting-title">Blokir Ping ICMP Echo</span>
              <span class="sec-setting-desc">
                Mengatur <code>net.ipv4.icmp_echo_ignore_all = 1</code> secara persisten di <code>/etc/sysctl.d/99-fikpanel-hardening.conf</code>.
              </span>
            </div>
            <div class="sec-setting-ctrl">
              <label class="switch" aria-label="Blokir Ping ICMP Echo">
                <input
                  type="checkbox"
                  role="switch"
                  checked={fwStatus?.block_icmp || false}
                  disabled={busyKey === 'fw:icmp'}
                  onchange={onToggleICMP}
                />
                <span class="switch__track"></span>
              </label>
            </div>
          </div>

          <div class="sec-setting-row">
            <div class="sec-setting-info">
              <span class="sec-setting-title">
                <span>Berkas Log Webserver</span>
                <span class="badge badge--info">{fwStatus?.site_logs_size || '0 B'}</span>
              </span>
              <span class="sec-setting-desc">
                Direktori log: <span class="fw-log-path">{fwStatus?.site_logs_path || '/var/log/nginx'}</span> — dikosongkan aman dengan truncate tanpa restart Nginx.
              </span>
            </div>
            <div class="sec-setting-ctrl">
              <button
                type="button"
                class="btn btn--secondary btn--sm"
                disabled={busyKey === 'fw:clear-logs'}
                onclick={onClearSiteLogs}
              >
                <i class="ri-delete-bin-6-line" aria-hidden="true"></i>
                <span>{busyKey === 'fw:clear-logs' ? 'Membersihkan...' : 'Kosongkan Log'}</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>
