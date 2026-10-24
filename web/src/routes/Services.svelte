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
    color: #34d399;
    font-size: inherit;
    cursor: pointer;
    border-bottom: 1px dashed rgba(52, 211, 153, 0.4);
  }
  .unit-link:hover {
    color: #6ee7b7;
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
    color: #f1f5f9;
    letter-spacing: -0.01em;
  }
  .subtitle {
    font-size: 0.75rem;
    color: #8b95a5;
    margin-top: 0.15rem;
  }
  .panel {
    background: #10131b;
    border: 1px solid #1c2230;
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
    color: #64748b;
    border-bottom: 1px solid #1c2230;
  }
  td {
    padding: 0.75rem;
    border-bottom: 1px solid #161924;
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
    background: rgba(245, 158, 11, 0.1);
    color: #f59e0b;
    border: 1px solid rgba(245, 158, 11, 0.25);
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
    background: rgba(16, 185, 129, 0.08);
    color: #10b981;
    border: 1px solid rgba(16, 185, 129, 0.2);
  }
  .badge-fail {
    background: rgba(244, 63, 94, 0.08);
    color: #f43f5e;
    border: 1px solid rgba(244, 63, 94, 0.2);
  }
  .dot-status {
    width: 5px;
    height: 5px;
    border-radius: 50%;
  }
  .dot-green {
    background: #10b981;
  }
  .dot-red {
    background: #f43f5e;
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
    background: #131622;
    color: #cbd5e1;
    border: 1px solid #1e2434;
  }
  .btn-ghost:hover {
    background: #181d2a;
    color: #f1f5f9;
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
    background: #10b981;
    color: #05130b;
  }
  .btn-accent:hover {
    background: #059669;
  }
  .btn-danger {
    background: rgba(244, 63, 94, 0.1);
    color: #f43f5e;
    border: 1px solid rgba(244, 63, 94, 0.25);
  }
  .btn-danger:hover {
    background: rgba(244, 63, 94, 0.2);
  }
</style>