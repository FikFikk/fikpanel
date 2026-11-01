<script lang="ts">
  import type { SSHStats, SystemAccount } from '../../lib/types';
  import '../../styles/pages/security/ssh.css';

  export type SshSubTab = 'basic' | 'accounts' | 'logs';

  interface Props {
    sshStats: SSHStats | null;
    accounts: SystemAccount[];
    sshSubTab: SshSubTab;
    sshPortInput: number;
    sshPasswordAuth: boolean;
    sshKeyAuth: boolean;
    sshPermitRoot: boolean;
    onSelectSubTab: (tab: SshSubTab) => void;
    onRestartSSH: () => void;
    onSaveSSHConfig: () => void;
    onToggleAccountLock: (acc: SystemAccount) => void;
  }

  let {
    sshStats,
    accounts,
    sshSubTab,
    sshPortInput = $bindable(),
    sshPasswordAuth = $bindable(),
    sshKeyAuth = $bindable(),
    sshPermitRoot = $bindable(),
    onSelectSubTab,
    onRestartSSH,
    onSaveSSHConfig,
    onToggleAccountLock,
  }: Props = $props();
</script>

<div class="pg-security-ssh tab-pane">
  <!-- Grid Atas: Status Layanan SSH & Statistik Login -->
  <div class="ssh-top-grid">
    <!-- Kartu Layanan SSH -->
    <div class="card ssh-service-card">
      <div class="card__head">
        <span class="card__title">SSH service</span>
        <label class="switch" aria-label="Toggle SSH Service">
          <input
            type="checkbox"
            role="switch"
            checked={sshStats ? sshStats.service_active : true}
            onchange={onRestartSSH}
          />
          <span class="switch__track"></span>
        </label>
      </div>
      <div class="card__body">
        <div class="ssh-status-tag">
          <span class="badge badge--success badge--pill">
            <span class="dot dot--success dot--pulse"></span> SSH service enabled
          </span>
        </div>
        <p class="text-subtle mt-2">Current status: <strong class="text-accent">Running</strong></p>
        <div class="mt-4">
          <button class="btn btn--secondary btn--sm" onclick={onRestartSSH}>
            <i class="ri-restart-line"></i> Restart SSH Daemon
          </button>
        </div>
      </div>
    </div>

    <!-- Kartu Statistik Percobaan Login SSH -->
    <div class="card ssh-stats-card">
      <div class="ssh-stat-boxes">
        <div class="stat-box success-box">
          <span class="stat-big-num">{sshStats?.successful_logins || 46}</span>
          <span class="stat-box-label">SSH login successful</span>
        </div>
        <div class="stat-box fail-box">
          <span class="stat-big-num">{sshStats?.failed_logins || 0}</span>
          <span class="stat-box-label">SSH login failed</span>
        </div>
      </div>

      <div class="ssh-timeline-wrap">
        <div class="timeline-row">
          <span class="timeline-label">Today</span>
          <div class="timeline-bar-bg">
            <div class="timeline-bar-fill timeline-bar-fill--75"></div>
          </div>
        </div>
        <div class="timeline-row">
          <span class="timeline-label">Yesterday</span>
          <div class="timeline-bar-bg">
            <div class="timeline-bar-fill timeline-bar-fill--45"></div>
          </div>
        </div>
        <div class="timeline-row">
          <span class="timeline-label">7 Days</span>
          <div class="timeline-bar-bg">
            <div class="timeline-bar-fill timeline-bar-fill--90"></div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Sub-tabs Navigasi SSH -->
  <div class="tabs mb-4">
    <button
      class="tabs__tab"
      class:is-active={sshSubTab === 'basic'}
      onclick={() => onSelectSubTab('basic')}
    >
      Basic setup
    </button>
    <button
      class="tabs__tab"
      class:is-active={sshSubTab === 'accounts'}
      onclick={() => onSelectSubTab('accounts')}
    >
      System Accounts
    </button>
    <button
      class="tabs__tab"
      class:is-active={sshSubTab === 'logs'}
      onclick={() => onSelectSubTab('logs')}
    >
      SSH login logs
    </button>
  </div>

  <!-- Konten Subtab SSH: Basic setup -->
  {#if sshSubTab === 'basic'}
    <div class="card p-4">
      <h3 class="section-title">SSH Basic Settings</h3>

      <div class="setting-item-row">
        <div class="setting-info">
          <span class="setting-name">SSH Password login</span>
          <p class="setting-desc">Allow SSH login with password</p>
        </div>
        <label class="switch" aria-label="Toggle Password Login">
          <input type="checkbox" role="switch" bind:checked={sshPasswordAuth} />
          <span class="switch__track"></span>
        </label>
      </div>

      <div class="setting-item-row">
        <div class="setting-info">
          <span class="setting-name">SSH key login</span>
          <p class="setting-desc">Allow SSH login with cryptographic key</p>
        </div>
        <label class="switch" aria-label="Toggle Key Login">
          <input type="checkbox" role="switch" bind:checked={sshKeyAuth} />
          <span class="switch__track"></span>
        </label>
      </div>

      <div class="setting-item-row">
        <div class="setting-info">
          <span class="setting-name">SSH port</span>
          <p class="setting-desc">Port used by the SSH protocol. The default value is 22</p>
        </div>
        <div class="port-input-wrap">
          <input type="number" class="input input--sm mono input-port-w" bind:value={sshPortInput} />
          <button class="btn btn--primary btn--sm" onclick={onSaveSSHConfig}>Save</button>
        </div>
      </div>

      <h3 class="section-title mt-6">Root Settings</h3>

      <div class="setting-item-row">
        <div class="setting-info">
          <span class="setting-name">Permit Root Login</span>
          <p class="setting-desc">Izinkan user root melakukan autentikasi langsung via SSH</p>
        </div>
        <label class="switch" aria-label="Toggle Root Login">
          <input type="checkbox" role="switch" bind:checked={sshPermitRoot} />
          <span class="switch__track"></span>
        </label>
      </div>
    </div>
  {/if}

  <!-- Konten Subtab SSH: System Accounts -->
  {#if sshSubTab === 'accounts'}
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>Username</th>
            <th>UID</th>
            <th>GID</th>
            <th>Shell</th>
            <th>Home Directory</th>
            <th>Status</th>
            <th class="text-right">Operate</th>
          </tr>
        </thead>
        <tbody>
          {#each accounts as acc (acc.username)}
            <tr>
              <td class="font-bold text-accent mono">{acc.username}</td>
              <td class="mono text-muted">{acc.uid}</td>
              <td class="mono text-muted">{acc.gid}</td>
              <td class="mono text-subtle">{acc.shell}</td>
              <td class="mono text-subtle">{acc.home_dir}</td>
              <td>
                <span class="badge {acc.locked ? 'badge--danger' : 'badge--success'}">
                  {acc.locked ? 'Locked' : 'Active'}
                </span>
              </td>
              <td class="text-right">
                <button class="btn btn--ghost btn--sm" onclick={() => onToggleAccountLock(acc)}>
                  {acc.locked ? 'Buka Kunci' : 'Kunci Akun'}
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <!-- Konten Subtab SSH: SSH login logs -->
  {#if sshSubTab === 'logs'}
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>Time</th>
            <th>User</th>
            <th>Remote IP</th>
            <th>Port</th>
            <th>Status</th>
            <th>Message</th>
          </tr>
        </thead>
        <tbody>
          {#if !sshStats || !sshStats.recent_logs || sshStats.recent_logs.length === 0}
            <tr>
              <td colspan="6" class="text-center text-muted p-4">Belum ada rekaman riwayat login SSH terbaru.</td>
            </tr>
          {:else}
            {#each sshStats.recent_logs as log}
              <tr>
                <td class="mono text-subtle">{new Date(log.timestamp * 1000).toLocaleString('id-ID')}</td>
                <td class="mono font-bold text-accent">{log.user}</td>
                <td class="mono text-muted">{log.ip}</td>
                <td class="mono text-subtle">{log.port}</td>
                <td>
                  <span class="badge {log.status === 'success' ? 'badge--success' : 'badge--danger'}">
                    {log.status.toUpperCase()}
                  </span>
                </td>
                <td class="text-subtle">{log.message}</td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  {/if}
</div>
