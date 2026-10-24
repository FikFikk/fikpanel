<script lang="ts">
  import { formatBytes } from '../lib/api';
  import type { Service } from '../lib/types';
  import NginxManager from '../lib/components/NginxManager.svelte';

  interface Props {
    services: Service[];
    onSvcAction: (unit: string, action: string) => void;
    onRefresh: () => void;
    onNotify: (msg: string) => void;
  }

  let { services = [], onSvcAction, onRefresh, onNotify }: Props = $props();

  let showNginxManager = $state(false);
</script>

<div class="services-container">
  <div class="head-bar">
    <div>
      <h2 class="title">Layanan Sistem (Systemd Units)</h2>
      <p class="subtitle">Unit terkelola dalam allowlist &middot; Supervisor via privileged agent RPC</p>
    </div>
    <button class="btn btn-ghost" onclick={onRefresh}>Refresh</button>
  </div>

  <div class="panel">
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Unit</th>
            <th>Deskripsi / Label</th>
            <th>Status Aktif</th>
            <th>PID</th>
            <th>Penggunaan Memori</th>
            <th style="text-align: right;">Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#each services as s}
            <tr>
              <td>
                {#if s.unit === 'nginx.service'}
                  <button id="svc-nginx-manage" class="unit-link mono font-bold" onclick={() => (showNginxManager = true)} title="Buka Nginx Manager">
                    {s.unit} <i class="ri-settings-4-line"></i>
                  </button>
                {:else}
                  <span class="mono font-bold">{s.unit}</span>
                {/if}
                {#if s.protected}
                  <span class="shield-badge">PROTECTED</span>
                {/if}
              </td>
              <td class="text-muted">{s.label || '—'}</td>
              <td>
                <span class="badge {s.active_state === 'active' ? 'badge-ok' : 'badge-fail'}">
                  <span class="dot-status {s.active_state === 'active' ? 'dot-green' : 'dot-red'}"></span>
                  {s.active_state} ({s.sub_state})
                </span>
              </td>
              <td class="mono">{s.main_pid > 0 ? s.main_pid : '—'}</td>
              <td class="mono">{formatBytes(s.memory_current)}</td>
              <td style="text-align: right;">
                <div class="actions-cell">
                  <button class="btn-sm btn-ghost" onclick={() => onSvcAction(s.unit, 'restart')}>
                    Restart
                  </button>
                  <button
                    class="btn-sm {s.active_state === 'active' ? 'btn-danger' : 'btn-accent'}"
                    onclick={() => onSvcAction(s.unit, s.active_state === 'active' ? 'stop' : 'start')}
                  >
                    {s.active_state === 'active' ? 'Stop' : 'Start'}
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
</div>

{#if showNginxManager}
  <NginxManager
    onClose={() => { showNginxManager = false; onRefresh(); }}
    {onNotify}
  />
{/if}

<style>
  .unit-link {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background: none;
    border: none;
    padding: 0;
    color: var(--accent-strong);
    font-size: inherit;
    cursor: pointer;
    border-bottom: 1px dashed color-mix(in srgb, var(--accent-strong) 40%, transparent);
  }
  .unit-link:hover {
    color: var(--accent-strong);
    border-bottom-style: solid;
  }
  .services-container {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .head-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
  }
  .title {
    font-size: 1.15rem;
    font-weight: 700;
    color: var(--text);
    letter-spacing: -0.01em;
  }
  .subtitle {
    font-size: 0.75rem;
    color: var(--text-muted);
    margin-top: 0.15rem;
  }
  .panel {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 1.15rem 1.35rem;
  }
  .table-wrap {
    overflow-x: auto;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }
  th {
    text-align: left;
    padding: 0.65rem 0.75rem;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-subtle);
    border-bottom: 1px solid var(--border);
  }
  td {
    padding: 0.75rem;
    border-bottom: 1px solid var(--border-subtle);
  }
  .mono {
    font-family: 'Fira Code', monospace;
  }
  .shield-badge {
    display: inline-block;
    font-size: 0.62rem;
    font-weight: 700;
    padding: 0.12rem 0.35rem;
    border-radius: 4px;
    background: color-mix(in srgb, var(--warning) 10%, transparent);
    color: var(--warning);
    border: 1px solid color-mix(in srgb, var(--warning) 25%, transparent);
    margin-left: 0.4rem;
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    font-size: 0.7rem;
    font-weight: 600;
  }
  .badge-ok {
    background: color-mix(in srgb, var(--accent) 8%, transparent);
    color: var(--accent);
    border: 1px solid color-mix(in srgb, var(--accent) 20%, transparent);
  }
  .badge-fail {
    background: color-mix(in srgb, var(--danger-strong) 8%, transparent);
    color: var(--danger-strong);
    border: 1px solid color-mix(in srgb, var(--danger-strong) 20%, transparent);
  }
  .dot-status {
    width: 5px;
    height: 5px;
    border-radius: 50%;
  }
  .dot-green {
    background: var(--accent);
  }
  .dot-red {
    background: var(--danger-strong);
  }
  .actions-cell {
    display: flex;
    justify-content: flex-end;
    gap: 0.4rem;
  }
  .btn {
    padding: 0.45rem 0.85rem;
    border-radius: 6px;
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
    transition: background 0.15s ease;
  }
  .btn-ghost {
    background: var(--bg-raised);
    color: var(--text-secondary);
    border: 1px solid var(--border);
  }
  .btn-ghost:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .btn-sm {
    padding: 0.25rem 0.55rem;
    border-radius: 4px;
    font-size: 0.72rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
    transition: background 0.15s ease;
  }
  .btn-accent {
    background: var(--accent);
    color: var(--text-on-accent);
  }
  .btn-accent:hover {
    background: var(--accent-hover);
  }
  .btn-danger {
    background: color-mix(in srgb, var(--danger-strong) 10%, transparent);
    color: var(--danger-strong);
    border: 1px solid color-mix(in srgb, var(--danger-strong) 25%, transparent);
  }
  .btn-danger:hover {
    background: color-mix(in srgb, var(--danger-strong) 20%, transparent);
  }
</style>