<script lang="ts">
  import type { SystemProcess } from '../../lib/types';

  interface Props {
    processes: SystemProcess[];
    loading: boolean;
    sortBy: 'cpu' | 'mem' | 'pid';
    search: string;
    filter: 'all' | 'nonroot' | 'zombie';
    pollSeconds: number;
    busyKey: string;
    onChangeQuery: () => void;
    onInspect: (p: SystemProcess) => void;
    onKill: (p: SystemProcess, signal: 'TERM' | 'KILL') => void;
  }

  let {
    processes,
    loading,
    sortBy = $bindable(),
    search = $bindable(),
    filter = $bindable(),
    pollSeconds = $bindable(),
    busyKey,
    onChangeQuery,
    onInspect,
    onKill,
  }: Props = $props();

  function stateBadge(st: string): { cls: string; label: string } {
    switch (st) {
      case 'R':
        return { cls: 'badge badge--success', label: 'Running' };
      case 'Z':
        return { cls: 'badge badge--danger', label: 'Zombie' };
      case 'D':
        return { cls: 'badge badge--warning', label: 'I/O Wait' };
      case 'T':
        return { cls: 'badge badge--warning', label: 'Stopped' };
      default:
        return { cls: 'badge badge--neutral', label: 'Sleep' };
    }
  }
</script>

<div class="sys-pane">
  <div class="card">
    <div class="sys-toolbar">
      <div class="sys-toolbar-group">
        <div class="input-wrap sys-search">
          <i class="ri-search-line input__icon" aria-hidden="true"></i>
          <input
            type="text"
            class="input input--search"
            placeholder="Cari PID, user, perintah..."
            bind:value={search}
            oninput={onChangeQuery}
          />
        </div>

        <select class="select" aria-label="Urutkan proses" bind:value={sortBy} onchange={onChangeQuery}>
          <option value="cpu">Urut: CPU Tertinggi</option>
          <option value="mem">Urut: RAM Tertinggi</option>
          <option value="pid">Urut: PID</option>
        </select>

        <select class="select" aria-label="Filter proses" bind:value={filter} onchange={onChangeQuery}>
          <option value="all">Semua Proses</option>
          <option value="nonroot">Hanya Non-Root</option>
          <option value="zombie">Hanya Zombie (Z)</option>
        </select>
      </div>

      <div class="sys-toolbar-group">
        <select class="select" aria-label="Interval auto-refresh" bind:value={pollSeconds}>
          <option value={0}>Auto-Refresh: Mati</option>
          <option value={3}>Live: Tiap 3 Detik</option>
          <option value={5}>Live: Tiap 5 Detik</option>
        </select>
        <button type="button" class="btn btn--secondary btn--sm" disabled={loading} onclick={onChangeQuery}>
          <i class={loading ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
          <span>Segarkan</span>
        </button>
      </div>
    </div>

    <div class="table-wrap">
      <table class="table table--dense">
        <thead>
          <tr>
            <th>PID</th>
            <th>User</th>
            <th>Status</th>
            <th>CPU %</th>
            <th>RAM (RSS)</th>
            <th>Threads</th>
            <th>Uptime</th>
            <th>Perintah (Klik untuk Inspeksi /proc)</th>
            <th class="sys-col-right">Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#if loading && processes.length === 0}
            <tr>
              <td colspan="9">
                <div class="state state--loading">
                  <i class="ri-loader-4-line spin" aria-hidden="true"></i>
                  <div class="state__title">Membaca Tabel Proses Sistem...</div>
                </div>
              </td>
            </tr>
          {:else if processes.length === 0}
            <tr>
              <td colspan="9">
                <div class="empty-state">
                  <i class="ri-cpu-line" aria-hidden="true"></i>
                  <div class="empty-state__title">Tidak Ada Proses Ditemukan</div>
                  <p class="state__desc">
                    Ubah kata kunci pencarian atau pastikan FikPanel berjalan pada host Linux untuk membaca proses <code>ps</code> &amp; <code>/proc</code>.
                  </p>
                </div>
              </td>
            </tr>
          {:else}
            {#each processes as p (p.pid)}
              {@const st = stateBadge(p.state)}
              <tr>
                <td class="mono">
                  <strong>#{p.pid}</strong>
                  {#if p.protected}
                    <i class="ri-shield-check-fill text-subtle" title="Proses vital sistem dilindungi"></i>
                  {/if}
                </td>
                <td class="mono text-muted">{p.user}</td>
                <td><span class={st.cls}>{st.label}</span></td>
                <td>
                  <div class="sys-mini-bar-wrap">
                    <div class="sys-mini-bar">
                      <div
                        class="sys-mini-bar__fill"
                        class:sys-mini-bar__fill--danger={p.cpu >= 80}
                        class:sys-mini-bar__fill--warn={p.cpu >= 40 && p.cpu < 80}
                        style="--bar-w: {Math.min(100, Math.round(p.cpu))}%"
                      ></div>
                    </div>
                    <span class="mono tabular">{p.cpu.toFixed(1)}%</span>
                  </div>
                </td>
                <td class="mono tabular">
                  <strong>{p.memory.toFixed(1)} MB</strong>
                  <span class="text-subtle">({p.mem_pct.toFixed(1)}%)</span>
                </td>
                <td class="mono text-subtle tabular">{p.threads}</td>
                <td class="mono text-subtle">{p.elapsed}</td>
                <td>
                  <button
                    type="button"
                    class="sys-cmd-cell mono"
                    title={p.command}
                    onclick={() => onInspect(p)}
                  >
                    {p.command}
                  </button>
                </td>
                <td class="sys-col-right">
                  <div class="sys-row-actions">
                    <button
                      type="button"
                      class="btn btn--ghost btn--icon btn--sm"
                      title={`Inspeksi detail proses #${p.pid}`}
                      aria-label={`Inspeksi detail proses #${p.pid}`}
                      onclick={() => onInspect(p)}
                    >
                      <i class="ri-search-eye-line" aria-hidden="true"></i>
                    </button>
                    <button
                      type="button"
                      class="btn btn--secondary btn--icon btn--sm"
                      title={p.protected ? 'Proses vital dilindungi' : `Hentikan sopan SIGTERM #${p.pid}`}
                      aria-label={`Hentikan proses #${p.pid} secara sopan`}
                      disabled={p.protected || busyKey === `kill:${p.pid}`}
                      onclick={() => onKill(p, 'TERM')}
                    >
                      <i class="ri-stop-circle-line" aria-hidden="true"></i>
                    </button>
                    <button
                      type="button"
                      class="btn btn--danger btn--icon btn--sm"
                      title={p.protected ? 'Proses vital dilindungi' : `Paksa hentikan SIGKILL #${p.pid}`}
                      aria-label={`Paksa hentikan proses #${p.pid}`}
                      disabled={p.protected || busyKey === `kill:${p.pid}`}
                      onclick={() => onKill(p, 'KILL')}
                    >
                      <i class="ri-close-circle-line" aria-hidden="true"></i>
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</div>
