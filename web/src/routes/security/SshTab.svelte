<script lang="ts">
  import type { SSHStats, SSHAuthorizedKey, SystemAccount } from '../../lib/types';
  import '../../styles/pages/security/ssh.css';

  export type SshSubTab = 'config' | 'keys' | 'accounts' | 'logs';

  interface Props {
    sshStats: SSHStats | null;
    sshKeys: SSHAuthorizedKey[];
    accounts: SystemAccount[];
    sshSubTab: SshSubTab;
    sshPortInput: number;
    sshPasswordAuth: boolean;
    sshPubkeyAuth: boolean;
    sshPermitRoot: string;
    sshMaxAuthTries: number;
    newPublicKey: string;
    busyKey: string;
    onSelectSubTab: (tab: SshSubTab) => void;
    onRestartSSH: () => void;
    onSaveSSHConfig: () => void;
    onAddSSHKey: () => void;
    onRemoveSSHKey: (key: SSHAuthorizedKey) => void;
    onToggleAccountLock: (acc: SystemAccount) => void;
  }

  let {
    sshStats,
    sshKeys,
    accounts,
    sshSubTab,
    sshPortInput = $bindable(),
    sshPasswordAuth = $bindable(),
    sshPubkeyAuth = $bindable(),
    sshPermitRoot = $bindable(),
    sshMaxAuthTries = $bindable(),
    newPublicKey = $bindable(),
    busyKey,
    onSelectSubTab,
    onRestartSSH,
    onSaveSSHConfig,
    onAddSSHKey,
    onRemoveSSHKey,
    onToggleAccountLock,
  }: Props = $props();

  let logFilter = $state<'all' | 'success' | 'failed'>('all');
  let logSearch = $state('');

  // Skala maksimum harian untuk proporsi grafik batang 7 hari
  let maxDailyCount = $derived.by(() => {
    const days = sshStats?.daily_stats || [];
    let max = 1;
    for (const d of days) {
      if (d.success > max) max = d.success;
      if (d.failed > max) max = d.failed;
    }
    return max;
  });

  function barPct(val: number): string {
    if (val <= 0) return '3%';
    return `${Math.max(8, Math.round((val / maxDailyCount) * 100))}%`;
  }

  let filteredLogs = $derived.by(() => {
    const list = sshStats?.recent_logs || [];
    const q = logSearch.trim().toLowerCase();
    return list.filter((l) => {
      if (logFilter !== 'all' && l.status !== logFilter) return false;
      if (q) {
        return (
          l.user.toLowerCase().includes(q) ||
          l.ip.toLowerCase().includes(q) ||
          l.message.toLowerCase().includes(q)
        );
      }
      return true;
    });
  });

  function lockBadgeText(acc: SystemAccount): string {
    if (!acc.locked) return 'Aktif';
    if (acc.lock_reason === 'passwd_locked') return 'Terkunci (passwd)';
    return 'No-Login Shell';
  }
</script>

<div class="pg-security-ssh">
  <!-- Grid Atas: Status Layanan SSH & Statistik Login 7 Hari Real -->
  <div class="ssh-top-grid">
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-terminal-box-line" aria-hidden="true"></i>
          <span>Layanan OpenSSH</span>
        </span>
        <span class="badge {sshStats?.config.active_service ? 'badge--success' : 'badge--warning'} badge--pill">
          <span class="dot {sshStats?.config.active_service ? 'dot--success dot--pulse' : 'dot--warning'}"></span>
          <span>{sshStats?.config.active_service ? 'Berjalan' : sshStats?.config.is_linux ? 'Berhenti' : 'Non-Linux'}</span>
        </span>
      </div>

      <div class="card__body ssh-service-meta">
        <div class="ssh-meta-kv">
          <div class="ssh-meta-box">
            <span class="ssh-meta-label">Port Aktif</span>
            <span class="ssh-meta-val">{sshStats?.config.port || 22}</span>
          </div>
          <div class="ssh-meta-box">
            <span class="ssh-meta-label">SSH Keys</span>
            <span class="ssh-meta-val">{sshKeys.length} kunci</span>
          </div>
          <div class="ssh-meta-box">
            <span class="ssh-meta-label">Root Login</span>
            <span class="ssh-meta-val">{sshStats?.config.permit_root_login || '-'}</span>
          </div>
          <div class="ssh-meta-box">
            <span class="ssh-meta-label">Sumber Konfig</span>
            <span class="ssh-meta-val truncate" title={sshStats?.config.config_source || ''}>
              {sshStats?.config.config_source || '-'}
            </span>
          </div>
        </div>

        <button
          type="button"
          class="btn btn--secondary btn--sm"
          disabled={busyKey === 'ssh:restart'}
          onclick={onRestartSSH}
        >
          <i class="ri-restart-line" aria-hidden="true"></i>
          <span>{busyKey === 'ssh:restart' ? 'Memulai ulang...' : 'Restart Daemon SSH'}</span>
        </button>
      </div>
    </div>

    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-bar-chart-2-line" aria-hidden="true"></i>
          <span>Telemetri Autentikasi SSH (7 Hari)</span>
        </span>
        <span class="badge badge--neutral mono">{sshStats?.log_source || '-'}</span>
      </div>

      <div class="card__body">
        <div class="ssh-stats-grid">
          <div class="ssh-stat-tile ssh-stat-tile--ok">
            <span class="ssh-stat-num">{sshStats?.successful_logins ?? 0}</span>
            <span class="ssh-stat-label">Login Sukses (7 hari)</span>
            <span class="ssh-stat-sub">Hari ini: {sshStats?.today_success ?? 0} · Kemarin: {sshStats?.yesterday_success ?? 0}</span>
          </div>
          <div class="ssh-stat-tile ssh-stat-tile--fail">
            <span class="ssh-stat-num">{sshStats?.failed_logins ?? 0}</span>
            <span class="ssh-stat-label">Percobaan Gagal (7 hari)</span>
            <span class="ssh-stat-sub">Hari ini: {sshStats?.today_fail ?? 0} · Kemarin: {sshStats?.yesterday_fail ?? 0}</span>
          </div>
        </div>

        <!-- Grafik batang harian 7 hari dari data log aktual -->
        <div class="ssh-bars" aria-label="Grafik autentikasi SSH 7 hari terakhir">
          {#each sshStats?.daily_stats || [] as day (day.date)}
            <div class="ssh-bar-col" title={`${day.label}: ${day.success} sukses, ${day.failed} gagal`}>
              <div class="ssh-bar-track">
                <span class="ssh-bar-stick ssh-bar-stick--ok" style="--bar-ok: {barPct(day.success)}"></span>
                <span class="ssh-bar-stick ssh-bar-stick--fail" style="--bar-fail: {barPct(day.failed)}"></span>
              </div>
              <span class="ssh-bar-day">{day.label}</span>
            </div>
          {/each}
        </div>
      </div>
    </div>
  </div>

  {#if sshStats?.config.cloud_init_override}
    <div class="alert alert--warning" role="status">
      <i class="ri-information-line" aria-hidden="true"></i>
      <div>
        <strong>Override Cloud-Init Terdeteksi:</strong> Berkas <code>{sshStats.config.cloud_init_file}</code> turut mengatur kebijakan SSH. FikPanel menulis konfigurasi ke <code>/etc/ssh/sshd_config.d/01-fikpanel.conf</code> agar menjadi prioritas pertama (first-match-wins).
      </div>
    </div>
  {/if}

  <!-- Sub-tabs SSH -->
  <div class="tabs tabs--underline" role="tablist">
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={sshSubTab === 'config'}
      aria-selected={sshSubTab === 'config'}
      onclick={() => onSelectSubTab('config')}
    >
      <i class="ri-settings-3-line" aria-hidden="true"></i>
      <span>Konfigurasi SSH</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={sshSubTab === 'keys'}
      aria-selected={sshSubTab === 'keys'}
      onclick={() => onSelectSubTab('keys')}
    >
      <i class="ri-key-2-line" aria-hidden="true"></i>
      <span>SSH Authorized Keys ({sshKeys.length})</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={sshSubTab === 'accounts'}
      aria-selected={sshSubTab === 'accounts'}
      onclick={() => onSelectSubTab('accounts')}
    >
      <i class="ri-user-settings-line" aria-hidden="true"></i>
      <span>Akun Sistem ({accounts.length})</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={sshSubTab === 'logs'}
      aria-selected={sshSubTab === 'logs'}
      onclick={() => onSelectSubTab('logs')}
    >
      <i class="ri-file-list-3-line" aria-hidden="true"></i>
      <span>Log Autentikasi ({sshStats?.recent_logs.length || 0})</span>
    </button>
  </div>

  <!-- SUBTAB 1: Konfigurasi SSH -->
  {#if sshSubTab === 'config'}
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-shield-keyhole-line" aria-hidden="true"></i>
          <span>Hardening Daemon OpenSSH</span>
        </span>
        <span class="badge badge--info">Validasi otomatis sshd -t + Auto-Rollback</span>
      </div>

      <div class="card__body">
        {#if !sshPasswordAuth && sshKeys.length === 0}
          <div class="alert alert--danger" role="alert">
            <i class="ri-error-warning-fill" aria-hidden="true"></i>
            <div>
              <strong>Peringatan Lockout:</strong> Belum ada SSH Public Key di <code>authorized_keys</code>. Tambahkan kunci publik pada tab <strong>SSH Authorized Keys</strong> terlebih dahulu sebelum menonaktifkan login kata sandi.
            </div>
          </div>
        {/if}

        <div class="ssh-form-grid">
          <div class="field">
            <label class="field__label" for="ssh-port-input">Port SSH</label>
            <input
              id="ssh-port-input"
              type="number"
              min="1"
              max="65535"
              class="input input--mono"
              bind:value={sshPortInput}
            />
            <span class="field__hint">Port baru otomatis dibuka di UFW bila UFW aktif.</span>
          </div>

          <div class="field">
            <label class="field__label" for="ssh-root-select">Permit Root Login</label>
            <select id="ssh-root-select" class="select" bind:value={sshPermitRoot}>
              <option value="prohibit-password">prohibit-password (Hanya SSH Key — Direkomendasikan)</option>
              <option value="no">no (Tolak semua login root SSH)</option>
              <option value="yes">yes (Izinkan password &amp; key — Berisiko)</option>
            </select>
          </div>

          <div class="field">
            <label class="field__label" for="ssh-max-tries">Max Auth Tries</label>
            <input
              id="ssh-max-tries"
              type="number"
              min="1"
              max="20"
              class="input input--mono"
              bind:value={sshMaxAuthTries}
            />
            <span class="field__hint">Batas percobaan salah per koneksi (saran: 3–6).</span>
          </div>
        </div>

        <div class="sec-setting-list">
          <div class="sec-setting-row">
            <div class="sec-setting-info">
              <span class="sec-setting-title">Autentikasi Kata Sandi (PasswordAuthentication)</span>
              <span class="sec-setting-desc">
                Matikan jika Anda sudah memasang SSH Key untuk menghentikan hampir seluruh serangan brute-force SSH.
              </span>
            </div>
            <div class="sec-setting-ctrl">
              <label class="switch" aria-label="Aktifkan PasswordAuthentication">
                <input type="checkbox" role="switch" bind:checked={sshPasswordAuth} />
                <span class="switch__track"></span>
              </label>
            </div>
          </div>

          <div class="sec-setting-row">
            <div class="sec-setting-info">
              <span class="sec-setting-title">Autentikasi Kunci Publik (PubkeyAuthentication)</span>
              <span class="sec-setting-desc">
                Mengizinkan login kriptografi menggunakan pasangan kunci SSH (Ed25519 / RSA / ECDSA).
              </span>
            </div>
            <div class="sec-setting-ctrl">
              <label class="switch" aria-label="Aktifkan PubkeyAuthentication">
                <input type="checkbox" role="switch" bind:checked={sshPubkeyAuth} />
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
          disabled={busyKey === 'ssh:save'}
          onclick={onSaveSSHConfig}
        >
          <i class="ri-save-line" aria-hidden="true"></i>
          <span>{busyKey === 'ssh:save' ? 'Memvalidasi & Menyimpan...' : 'Simpan & Validasi (sshd -t)'}</span>
        </button>
      </div>
    </div>
  {/if}

  <!-- SUBTAB 2: Manajemen SSH Authorized Keys -->
  {#if sshSubTab === 'keys'}
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-key-2-line" aria-hidden="true"></i>
          <span>Kunci Publik Terotorisasi (~/.ssh/authorized_keys)</span>
        </span>
      </div>

      <div class="ssh-key-form">
        <div class="field">
          <label class="field__label" for="ssh-pubkey-input">Tambah SSH Public Key Baru</label>
          <textarea
            id="ssh-pubkey-input"
            rows="2"
            class="input input--mono"
            placeholder="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... admin@laptop"
            bind:value={newPublicKey}
          ></textarea>
          <span class="field__hint">Format divalidasi kriptografis (Ed25519, RSA, ECDSA) dan ditulis dengan izin berkas 0600.</span>
        </div>
        <div class="row-end">
          <button
            type="button"
            class="btn btn--primary btn--sm"
            disabled={busyKey === 'ssh:add-key' || !newPublicKey.trim()}
            onclick={onAddSSHKey}
          >
            <i class="ri-add-line" aria-hidden="true"></i>
            <span>{busyKey === 'ssh:add-key' ? 'Menyimpan...' : 'Tambahkan SSH Key'}</span>
          </button>
        </div>
      </div>

      <div class="table-wrap">
        <table class="table table--dense">
          <thead>
            <tr>
              <th>Tipe</th>
              <th>Komentar / Label</th>
              <th>SHA-256 Fingerprint</th>
              <th>Cuplikan Key</th>
              <th class="col-right">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#if sshKeys.length === 0}
              <tr>
                <td colspan="5">
                  <div class="empty-state">
                    <i class="ri-key-line" aria-hidden="true"></i>
                    <div class="empty-state__title">Belum Ada SSH Public Key</div>
                    <p class="state__desc">
                      Tempelkan kunci publik SSH Anda di formulir atas agar dapat login tanpa kata sandi dengan aman.
                    </p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each sshKeys as k (k.id)}
                <tr>
                  <td><span class="badge badge--info mono">{k.type}</span></td>
                  <td>{k.comment || '-'}</td>
                  <td class="ssh-key-fp">{k.fingerprint}</td>
                  <td class="mono text-subtle truncate">{k.preview}</td>
                  <td class="col-right">
                    <button
                      type="button"
                      class="btn btn--ghost btn--sm"
                      disabled={busyKey === `ssh:del-key:${k.id}`}
                      onclick={() => onRemoveSSHKey(k)}
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

  <!-- SUBTAB 3: Akun Sistem Linux -->
  {#if sshSubTab === 'accounts'}
    <div class="card">
      <div class="table-wrap">
        <table class="table table--dense">
          <thead>
            <tr>
              <th>Username</th>
              <th>UID / GID</th>
              <th>Shell</th>
              <th>Home Directory</th>
              <th>Status Aktual</th>
              <th class="col-right">Operasi</th>
            </tr>
          </thead>
          <tbody>
            {#if accounts.length === 0}
              <tr>
                <td colspan="6">
                  <div class="empty-state">
                    <i class="ri-user-line" aria-hidden="true"></i>
                    <div class="empty-state__title">Daftar Akun Sistem Tidak Tersedia</div>
                    <p class="state__desc">
                      Pembacaan <code>/etc/passwd</code> dan status kunci <code>/etc/shadow</code> hanya berjalan pada server Linux.
                    </p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each accounts as acc (acc.username)}
                <tr>
                  <td class="mono">
                    <strong>{acc.username}</strong>
                    {#if acc.is_protected}
                      <span class="badge badge--warning">Dilindungi</span>
                    {/if}
                  </td>
                  <td class="mono text-muted">{acc.uid} / {acc.gid}</td>
                  <td class="mono text-subtle">{acc.shell}</td>
                  <td class="mono text-subtle">{acc.home_dir}</td>
                  <td>
                    <span class="badge {acc.locked ? 'badge--danger' : 'badge--success'}">
                      {lockBadgeText(acc)}
                    </span>
                  </td>
                  <td class="col-right">
                    <button
                      type="button"
                      class="btn btn--secondary btn--sm"
                      disabled={acc.is_protected || busyKey === `ssh:acc:${acc.username}`}
                      title={acc.is_protected ? 'Akun root dilindungi dari penguncian' : ''}
                      onclick={() => onToggleAccountLock(acc)}
                    >
                      {acc.locked ? 'Buka Kunci' : 'Kunci Akun'}
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

  <!-- SUBTAB 4: Log Autentikasi SSH -->
  {#if sshSubTab === 'logs'}
    <div class="card">
      <div class="sec-toolbar">
        <div class="sec-toolbar-left">
          <div class="input-wrap sec-search">
            <i class="ri-search-line input__icon" aria-hidden="true"></i>
            <input
              type="text"
              class="input input--search"
              placeholder="Cari user, IP sumber, pesan..."
              bind:value={logSearch}
            />
          </div>
          <select class="select" aria-label="Filter status log SSH" bind:value={logFilter}>
            <option value="all">Semua Status</option>
            <option value="success">Sukses</option>
            <option value="failed">Gagal</option>
          </select>
        </div>
        <span class="badge badge--neutral mono">{filteredLogs.length} baris</span>
      </div>

      <div class="table-wrap">
        <table class="table table--dense">
          <thead>
            <tr>
              <th>Waktu</th>
              <th>User</th>
              <th>IP Sumber</th>
              <th>Port</th>
              <th>Metode</th>
              <th>Status</th>
              <th>Detail Pesan</th>
            </tr>
          </thead>
          <tbody>
            {#if filteredLogs.length === 0}
              <tr>
                <td colspan="7">
                  <div class="empty-state">
                    <i class="ri-file-list-3-line" aria-hidden="true"></i>
                    <div class="empty-state__title">Tidak Ada Riwayat Log SSH</div>
                    <p class="state__desc">
                      Belum ada rekaman autentikasi SSH pada <code>journalctl</code> / <code>auth.log</code> yang cocok dengan filter ini.
                    </p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each filteredLogs as log, i (`${log.timestamp}-${log.ip}-${i}`)}
                <tr>
                  <td class="mono text-subtle">{new Date(log.timestamp * 1000).toLocaleString('id-ID')}</td>
                  <td class="mono"><strong>{log.user}</strong></td>
                  <td class="mono">{log.ip}</td>
                  <td class="mono text-subtle">{log.port || '-'}</td>
                  <td class="mono text-muted">{log.method}</td>
                  <td>
                    <span class="badge {log.status === 'success' ? 'badge--success' : 'badge--danger'}">
                      {log.status === 'success' ? 'SUKSES' : 'GAGAL'}
                    </span>
                  </td>
                  <td class="text-muted">{log.message}</td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  {/if}
</div>
