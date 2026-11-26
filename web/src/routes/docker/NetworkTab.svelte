<script lang="ts">
  // Subkomponen tab Network pada Docker Studio: subnet/gateway akurat, daftar kontainer terhubung, dan Prune
  import '../../styles/pages/docker/network.css';
  import type { DockerNetwork } from '../../lib/types';

  interface Props {
    networks: DockerNetwork[];
    loadingNetworks: boolean;
    actionLoading: boolean;
    onOpenCreateNetwork: () => void;
    onPruneNetworks: () => void;
    onLoadNetworks: () => void;
    onDeleteNetwork: (net: DockerNetwork) => void;
  }

  let {
    networks,
    loadingNetworks,
    actionLoading,
    onOpenCreateNetwork,
    onPruneNetworks,
    onLoadNetworks,
    onDeleteNetwork,
  }: Props = $props();
</script>

<div class="pg-docker-network tab-pane">
  <div class="toolbar">
    <div class="toolbar-left">
      <button type="button" class="btn btn--primary btn--sm" onclick={onOpenCreateNetwork}>
        <i class="ri-add-line" aria-hidden="true"></i>
        <span>Buat Jaringan Baru</span>
      </button>
      <button type="button" class="btn btn--danger btn--sm" disabled={actionLoading} onclick={onPruneNetworks}>
        <i class="ri-brush-line" aria-hidden="true"></i>
        <span>Bersihkan Tak Terpakai (Prune)</span>
      </button>
    </div>
    <div class="toolbar-right">
      <button
        type="button"
        class="btn btn--secondary btn--sm btn--icon"
        title="Segarkan jaringan"
        aria-label="Segarkan jaringan"
        onclick={onLoadNetworks}
      >
        <i class="ri-refresh-line {loadingNetworks ? 'spin' : ''}" aria-hidden="true"></i>
      </button>
    </div>
  </div>

  <div class="table-wrap">
    <table class="table">
      <thead>
        <tr>
          <th>Nama Network</th>
          <th>Driver</th>
          <th>Scope</th>
          <th>Subnet</th>
          <th>Gateway</th>
          <th>Kontainer Terhubung</th>
          <th class="col-w-100 text-right">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#if loadingNetworks}
          <tr>
            <td colspan="7" class="text-center py-6">
              <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
              <p class="text-muted mt-2">Memuat topologi jaringan Docker...</p>
            </td>
          </tr>
        {:else if networks.length === 0}
          <tr>
            <td colspan="7" class="text-center py-8">
              <div class="empty-state">
                <i class="ri-global-line empty-icon" aria-hidden="true"></i>
                <p class="empty-title">Tidak ada jaringan Docker terdeteksi</p>
              </div>
            </td>
          </tr>
        {:else}
          {#each networks as net (net.id)}
            <tr>
              <td>
                <strong class="font-mono text-sm">{net.name}</strong>
              </td>
              <td>
                <span class="badge badge--secondary font-mono">{net.driver}</span>
              </td>
              <td>
                <span class="text-xs text-muted">{net.scope}</span>
              </td>
              <td>
                <span class="font-mono text-xs">{net.subnet || '-'}</span>
              </td>
              <td>
                <span class="font-mono text-xs">{net.gateway || '-'}</span>
              </td>
              <td>
                <span
                  class="badge {net.containers_count > 0 ? 'badge--success' : 'badge--secondary'}"
                  title={(net.containers || []).join(', ') || 'Tidak ada kontainer terhubung'}
                >
                  {net.containers_count} kontainer
                </span>
              </td>
              <td class="text-right">
                {#if net.name !== 'bridge' && net.name !== 'host' && net.name !== 'none'}
                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Hapus Network"
                    aria-label="Hapus Network"
                    disabled={actionLoading}
                    onclick={() => onDeleteNetwork(net)}
                  >
                    <i class="ri-delete-bin-6-line text-danger" aria-hidden="true"></i>
                  </button>
                {:else}
                  <span class="text-xs text-muted">Sistem</span>
                {/if}
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>
