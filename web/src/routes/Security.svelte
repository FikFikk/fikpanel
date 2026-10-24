<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { AuditLog } from '../lib/types';

  let logs = $state<AuditLog[]>([]);
  let loading = $state(true);

  async function loadAuditLogs() {
    loading = true;
    try {
      const data = await apiFetch<{ logs: AuditLog[] }>('/api/audit');
      logs = data.logs || [];
    } catch {
      // Abaikan jika gagal
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadAuditLogs();
  });
</script>

<div class="sec-container">
  <div class="head-bar">
    <div>
      <h2 class="title">Keamanan & Audit Log</h2>
      <p class="subtitle">Mekanisme proteksi berlapis dan riwayat aktivitas sistem tercatat</p>
    </div>
    <button class="btn btn-ghost" onclick={loadAuditLogs}>Refresh Log</button>
  </div>

  <!-- Security Mechanisms Cards -->
  <div class="cards-grid">
    <div class="sec-card">
      <div class="s-icon" style="color: var(--accent);"><i class="ri-shield-check-line"></i></div>
      <div class="s-title">Argon2id Hashing</div>
      <div class="s-desc">Memory cost 19 MiB &middot; Proteksi brute-force GPU</div>
      <div class="s-status">AKTIF</div>
    </div>
    <div class="sec-card">
      <div class="s-icon" style="color: var(--info);"><i class="ri-key-2-line"></i></div>
      <div class="s-title">Session Hardening</div>
      <div class="s-desc">32-byte crypto/rand &middot; Cookie HttpOnly; SameSite=Strict</div>
      <div class="s-status">AKTIF</div>
    </div>
    <div class="sec-card">
      <div class="s-icon" style="color: var(--violet);"><i class="ri-lock-password-line"></i></div>
      <div class="s-title">CSRF Header Shield</div>
      <div class="s-desc">Wajib header X-Panel-Request: 1 pada seluruh request mutasi</div>
      <div class="s-status">AKTIF</div>
    </div>
    <div class="sec-card">
      <div class="s-icon" style="color: var(--warning);"><i class="ri-timer-flash-line"></i></div>
      <div class="s-title">Login Rate Limiter</div>
      <div class="s-desc">Batas 5 kegagalan / 15 menit per alamat IP</div>
      <div class="s-status">AKTIF</div>
    </div>
  </div>

  <!-- Audit Log Table -->
  <div class="panel">
    <div class="panel-head">
      <h3 class="panel-title">Riwayat Kejadian Audit (Audit Logs)</h3>
      <span class="count-tag">{logs.length} Log Terakhir</span>
    </div>

    {#if loading}
      <div class="loading">Memuat riwayat audit...</div>
    {:else if logs.length === 0}
      <div class="empty">Belum ada aktivitas tercatat di audit log.</div>
    {:else}
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Waktu</th>
              <th>Aktor</th>
              <th>Aksi</th>
              <th>Target</th>
              <th>Hasil</th>
              <th>Detail</th>
            </tr>
          </thead>
          <tbody>
            {#each logs as log}
              <tr>
                <td class="mono text-muted">
                  {new Date(log.ts * 1000).toLocaleString('id-ID')}
                </td>
                <td>
                  <span class="actor-tag">{log.actor}</span>
                </td>
                <td class="mono font-bold">{log.action}</td>
                <td class="mono text-muted">{log.target || 'â€”'}</td>
                <td>
                  <span class="res-badge {log.ok === 1 ? 'res-ok' : 'res-fail'}">
                    {log.ok === 1 ? 'SUKSES' : 'GAGAL'}
                  </span>
                </td>
                <td class="text-muted">{log.detail || 'â€”'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</div>

<style>
  .sec-container {
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
    color: var(--text);
  }
  .subtitle {
    font-size: 0.8rem;
    color: var(--text-muted);
    margin-top: 0.2rem;
  }
  .cards-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 1.25rem;
  }
  .sec-card {
    background: color-mix(in srgb, var(--bg-hover) 75%, transparent);
    border: 1px solid color-mix(in srgb, var(--text) 7%, transparent);
    border-radius: 12px;
    padding: 1.25rem;
    position: relative;
  }
  .s-icon {
    font-size: 1.5rem;
    margin-bottom: 0.5rem;
  }
  .s-title {
    font-size: 0.9rem;
    font-weight: 700;
    color: var(--text);
  }
  .s-desc {
    font-size: 0.72rem;
    color: var(--text-muted);
    margin: 0.35rem 0 0.75rem;
    line-height: 1.4;
  }
  .s-status {
    display: inline-block;
    font-size: 0.65rem;
    font-weight: 700;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    color: var(--accent);
    border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
  }
  .panel {
    background: color-mix(in srgb, var(--bg-hover) 75%, transparent);
    border: 1px solid color-mix(in srgb, var(--text) 7%, transparent);
    border-radius: 12px;
    padding: 1.5rem;
  }
  .panel-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.25rem;
  }
  .panel-title {
    font-size: 1rem;
    font-weight: 700;
    color: var(--text);
  }
  .count-tag {
    font-size: 0.72rem;
    font-family: 'Fira Code', monospace;
    color: var(--text-muted);
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
    color: var(--text-muted);
    border-bottom: 1px solid color-mix(in srgb, var(--text) 8%, transparent);
  }
  td {
    padding: 0.85rem 0.75rem;
    border-bottom: 1px solid color-mix(in srgb, var(--text) 4%, transparent);
  }
  .mono {
    font-family: 'Fira Code', monospace;
  }
  .actor-tag {
    font-size: 0.7rem;
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    background: color-mix(in srgb, var(--text) 6%, transparent);
    color: var(--text-secondary);
    text-transform: uppercase;
    font-weight: 600;
  }
  .res-badge {
    font-size: 0.68rem;
    font-weight: 700;
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
  }
  .res-ok {
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    color: var(--accent);
  }
  .res-fail {
    background: color-mix(in srgb, var(--danger-strong) 15%, transparent);
    color: var(--danger-strong);
  }
  .btn {
    padding: 0.45rem 0.9rem;
    border-radius: 8px;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
  }
  .btn-ghost {
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid color-mix(in srgb, var(--text) 10%, transparent);
  }
</style>