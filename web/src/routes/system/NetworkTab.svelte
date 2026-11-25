<script lang="ts">
  import { copyText } from '../../lib/utils/browser';
  import { notify } from '../../lib/stores/toast.svelte';
  import type { PortListening } from '../../lib/types';

  interface Props {
    ports: PortListening[];
    connections: Record<string, number>;
    loading: boolean;
    busyKey: string;
    onRefresh: () => void;
    onFreePort: (p: PortListening) => void;
  }

  let { ports, connections, loading, busyKey, onRefresh, onFreePort }: Props = $props();

  let search = $state('');
  let protoFilter = $state<'all' | 'tcp' | 'udp'>('all');
  let scopeFilter = $state<'all' | 'public' | 'local'>('all');

  let filteredPorts = $derived.by(() => {
    const q = search.trim().toLowerCase();
    return ports.filter((p) => {
      if (protoFilter === 'tcp' && !p.proto.startsWith('tcp')) return false;
      if (protoFilter === 'udp' && !p.proto.startsWith('udp')) return false;
      if (scopeFilter === 'public' && !p.exposed_public) return false;
      if (scopeFilter === 'local' && p.exposed_public) return false;
      if (q) {
        return (
          p.port.toLowerCase().includes(q) ||
          p.program.toLowerCase().includes(q) ||
          p.address.toLowerCase().includes(q) ||
          p.service.toLowerCase().includes(q) ||
          String(p.pid).includes(q)
        );
      }
      return true;
    });
  });

  async function copyEndpoint(p: PortListening) {
    await copyText(`${p.address}:${p.port}`);
    notify(`Alamat ${p.address}:${p.port} disalin`, 'info');
  }
</script>

<div class="sys-pane">
  <!-- Ringkasan Distribusi Koneksi TCP Real-Time (ss -tan) -->
  <div class="sys-conn-strip">
    <div class="sys-conn-pill">
      <span class="dot dot--success"></span>
      <span class="sys-conn-pill__label">ESTABLISHED</span>
      <span class="sys-conn-pill__num">{connections['ESTABLISHED'] ?? 0}</span>
    </div>
    <div class="sys-conn-pill">
      <span class="dot dot--info"></span>
      <span class="sys-conn-pill__label">LISTEN</span>
      <span class="sys-conn-pill__num">{connections['LISTEN'] ?? 0}</span>
    </div>
    <div class="sys-conn-pill">
      <span class="dot dot--warning"></span>
      <span class="sys-conn-pill__label">TIME-WAIT</span>
      <span class="sys-conn-pill__num">{connections['TIME-WAIT'] ?? 0}</span>
    </div>
    <div class="sys-conn-pill">
      <span class="dot" class:dot--danger={(connections['SYN-RECV'] ?? 0) > 20}></span>
      <span class="sys-conn-pill__label">SYN-RECV</span>
      <span class="sys-conn-pill__num">{connections['SYN-RECV'] ?? 0}</span>
    </div>
    <div class="sys-conn-pill">
      <span class="dot"></span>
      <span class="sys-conn-pill__label">CLOSE-WAIT</span>
      <span class="sys-conn-pill__num">{connections['CLOSE-WAIT'] ?? 0}</span>
    </div>
    <div class="sys-conn-pill">
      <span class="sys-conn-pill__label">Total Soket TCP</span>
      <span class="sys-conn-pill__num">{connections['TOTAL'] ?? 0}</span>
    </div>
  </div>

  <!-- Tabel Port Listening -->
  <div class="card">
    <div class="sys-toolbar">
      <div class="sys-toolbar-group">
        <div class="input-wrap sys-search">
          <i class="ri-search-line input__icon" aria-hidden="true"></i>
          <input
            type="text"
            class="input input--search"
            placeholder="Cari port, program, PID, alamat..."
            bind:value={search}
          />
        </div>
        <select class="select" aria-label="Filter protokol" bind:value={protoFilter}>
          <option value="all">Semua Protokol</option>
          <option value="tcp">TCP / TCP6</option>
          <option value="udp">UDP / UDP6</option>
        </select>
        <select class="select" aria-label="Filter eksposur" bind:value={scopeFilter}>
          <option value="all">Semua Eksposur</option>
          <option value="public">Publik (0.0.0.0 / ::)</option>
          <option value="local">Lokal (Loopback)</option>
        </select>
      </div>
      <div class="sys-toolbar-group">
        <button type="button" class="btn btn--secondary btn--sm" disabled={loading} onclick={onRefresh}>
          <i class={loading ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
          <span>Segarkan</span>
        </button>
      </div>
    </div>

    <div class="table-wrap">
      <table class="table table--dense">
        <thead>
          <tr>
            <th>Protokol</th>
            <th>Port</th>
            <th>Bind Address</th>
            <th>Layanan</th>
            <th>Program / PID</th>
            <th>Eksposur</th>
            <th class="sys-col-right">Aksi Cepat</th>
          </tr>
        </thead>
        <tbody>
          {#if filteredPorts.length === 0}
            <tr>
              <td colspan="7">
                <div class="empty-state">
                  <i class="ri-broadcast-line" aria-hidden="true"></i>
                  <div class="empty-state__title">Tidak Ada Port Listening</div>
                  <p class="state__desc">
                    Data soket jaringan aktual (`ss -tulnp`) tersedia saat berjalan di server Linux.
                  </p>
                </div>
              </td>
            </tr>
          {:else}
            {#each filteredPorts as p (`${p.proto}-${p.address}-${p.port}`)}
              <tr>
                <td><span class="badge badge--info mono">{p.proto.toUpperCase()}</span></td>
                <td class="mono"><strong>:{p.port}</strong></td>
                <td class="mono text-muted">{p.address}</td>
                <td><span class="badge badge--neutral">{p.service}</span></td>
                <td class="mono">
                  <strong>{p.program}</strong>
                  {#if p.pid > 0}
                    <span class="text-subtle"> (#{p.pid})</span>
                  {/if}
                </td>
                <td>
                  {#if p.exposed_public}
                    <span class="badge badge--warning">Publik</span>
                  {:else}
                    <span class="badge badge--success">Lokal</span>
                  {/if}
                </td>
                <td class="sys-col-right">
                  <div class="sys-row-actions">
                    <button
                      type="button"
                      class="btn btn--ghost btn--icon btn--sm"
                      title="Salin alamat:port"
                      aria-label="Salin alamat dan port"
                      onclick={() => copyEndpoint(p)}
                    >
                      <i class="ri-file-copy-line" aria-hidden="true"></i>
                    </button>
                    <button
                      type="button"
                      class="btn btn--danger btn--sm"
                      disabled={!p.can_kill || busyKey === `kill:${p.pid}`}
                      title={p.can_kill ? `Hentikan proses #${p.pid} untuk membebaskan port :${p.port}` : 'Proses sistem dilindungi atau PID tidak tersedia'}
                      onclick={() => onFreePort(p)}
                    >
                      Bebaskan Port
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
