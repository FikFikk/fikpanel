<script lang="ts">
  // Subkomponen tab Volume pada halaman Docker Management Suite
  import '../../styles/pages/docker/volume.css';
  import type { DockerVolume } from '../../lib/types';

  interface Props {
    volumes: DockerVolume[];
    loadingVolumes: boolean;
    actionLoading: boolean;
    onOpenCreateVolume: () => void;
    onPruneVolumes: () => void;
    onLoadVolumes: () => void;
    onDeleteVolume: (v: DockerVolume) => void;
  }

  let {
    volumes,
    loadingVolumes,
    actionLoading,
    onOpenCreateVolume,
    onPruneVolumes,
    onLoadVolumes,
    onDeleteVolume,
  }: Props = $props();
</script>

<div class="pg-docker-volume tab-pane">
  <div class="toolbar">
    <div class="toolbar-left">
      <button class="btn btn--primary btn--sm" onclick={onOpenCreateVolume}>
        <i class="ri-add-line" aria-hidden="true"></i>
        <span>Buat Volume Baru</span>
      </button>
      <button class="btn btn--danger btn--sm" onclick={onPruneVolumes}>
        <i class="ri-delete-bin-line" aria-hidden="true"></i>
        <span>Bersihkan Tak Terpakai (Prune)</span>
      </button>
    </div>
    <div class="toolbar-right">
      <button
        class="btn btn--secondary btn--sm btn--icon"
        title="Segarkan volume"
        aria-label="Segarkan volume"
        onclick={onLoadVolumes}
      >
        <i class="ri-refresh-line {loadingVolumes ? 'spin' : ''}" aria-hidden="true"></i>
      </button>
    </div>
  </div>

  <div class="table-wrap">
    <table class="table">
      <thead>
        <tr>
          <th>Nama Volume</th>
          <th>Driver</th>
          <th>Mountpoint</th>
          <th>Ukuran</th>
          <th>Ref Count</th>
          <th class="col-w-100 text-right">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#if loadingVolumes}
          <tr>
            <td colspan="6" class="text-center py-6">
              <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
              <p class="text-muted mt-2">Memuat daftar volume...</p>
            </td>
          </tr>
        {:else if volumes.length === 0}
          <tr>
            <td colspan="6" class="text-center py-8">
              <div class="empty-state">
                <i class="ri-database-line empty-icon" aria-hidden="true"></i>
                <p class="empty-title">Belum ada volume Docker</p>
              </div>
            </td>
          </tr>
        {:else}
          {#each volumes as v}
            <tr>
              <td>
                <strong class="font-mono text-sm">{v.name}</strong>
              </td>
              <td>
                <span class="badge badge--secondary font-mono">{v.driver}</span>
              </td>
              <td>
                <span class="text-xs font-mono text-muted">{v.mountpoint}</span>
              </td>
              <td>
                <span class="text-xs font-semibold">{v.size_human}</span>
              </td>
              <td>
                {#if v.ref_count > 0}
                  <span class="badge badge--success">{v.ref_count} terpasang</span>
                {:else}
                  <span class="badge badge--secondary">0 (Bebas)</span>
                {/if}
              </td>
              <td class="text-right">
                <button
                  class="btn btn--ghost btn--sm btn--icon"
                  title="Hapus Volume"
                  aria-label="Hapus Volume"
                  disabled={actionLoading}
                  onclick={() => onDeleteVolume(v)}
                >
                  <i class="ri-delete-bin-6-line text-danger" aria-hidden="true"></i>
                </button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>

