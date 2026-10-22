<script lang="ts">
  import { formatBytes } from '../lib/api';
  import type { Service } from '../lib/types';

  interface Props {
    services: Service[];
    onSvcAction: (unit: string, action: string) => void;
    onRefresh: () => void;
  }

  let { services = [], onSvcAction, onRefresh }: Props = $props();
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
                <span class="mono font-bold">{s.unit}</span>
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

<style>
  .services-container {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .head-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
  }
  .title {
    font-size: 1.25rem;
    font-weight: 700;
    color: #f3f4f6;
  }
  .subtitle {
    font-size: 0.8rem;
    color: #9ca3af;
    margin-top: 0.2rem;
  }
  .panel {
    background: rgba(28, 29, 34, 0.75);
    border: 1px solid rgba(255, 255, 255, 0.07);
    border-radius: 12px;
    padding: 1.25rem 1.5rem;
  }
  .table-wrap {
    overflow-x: auto;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }
  th {
    text-align: left;
    padding: 0.75rem;
    font-size: 0.7rem;
    text-transform: uppercase;
    color: #9ca3af;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  }
  td {
    padding: 0.85rem 0.75rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  }
  .mono {
    font-family: 'Fira Code', monospace;
  }
  .shield-badge {
    display: inline-block;
    font-size: 0.65rem;
    font-weight: 700;
    padding: 0.15rem 0.4rem;
    border-radius: 4px;
    background: rgba(245, 158, 11, 0.15);
    color: #f59e0b;
    border: 1px solid rgba(245, 158, 11, 0.3);
    margin-left: 0.4rem;
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.2rem 0.55rem;
    border-radius: 4px;
    font-size: 0.72rem;
    font-weight: 600;
  }
  .badge-ok {
    background: rgba(34, 197, 94, 0.12);
    color: #22c55e;
  }
  .badge-fail {
    background: rgba(244, 63, 94, 0.12);
    color: #f43f5e;
  }
  .dot-status {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  .dot-green {
    background: #22c55e;
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
    padding: 0.5rem 1rem;
    border-radius: 8px;
    font-size: 0.82rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
  }
  .btn-ghost {
    background: transparent;
    color: #d1d5db;
    border: 1px solid rgba(255, 255, 255, 0.1);
  }
  .btn-sm {
    padding: 0.25rem 0.65rem;
    border-radius: 6px;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
  }
  .btn-accent {
    background: #22c55e;
    color: #0f172a;
  }
  .btn-danger {
    background: rgba(244, 63, 94, 0.15);
    color: #f43f5e;
    border: 1px solid rgba(244, 63, 94, 0.25);
  }
</style>