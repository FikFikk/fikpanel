<script lang="ts">
  // Subkomponen tab Registries pada Docker Studio: daftar registri persisten dengan status autentikasi docker login
  import '../../styles/pages/docker/repository.css';
  import type { DockerRegistry } from '../../lib/types';

  interface Props {
    registries: DockerRegistry[];
    loadingRegistries: boolean;
    actionLoading: boolean;
    onOpenAddRegistry: () => void;
    onLoadRegistries: () => void;
    onDeleteRegistry: (reg: DockerRegistry) => void;
  }

  let {
    registries,
    loadingRegistries,
    actionLoading,
    onOpenAddRegistry,
    onLoadRegistries,
    onDeleteRegistry,
  }: Props = $props();
</script>

<div class="pg-docker-repository tab-pane">
  <div class="toolbar">
    <div class="toolbar-left">
      <button type="button" class="btn btn--primary btn--sm" onclick={onOpenAddRegistry}>
        <i class="ri-add-line" aria-hidden="true"></i>
        <span>Hubungkan Registri Privat</span>
      </button>
    </div>
    <div class="toolbar-right">
      <button
        type="button"
        class="btn btn--secondary btn--sm btn--icon"
        title="Segarkan registri"
        aria-label="Segarkan registri"
        onclick={onLoadRegistries}
      >
        <i class="ri-refresh-line {loadingRegistries ? 'spin' : ''}" aria-hidden="true"></i>
      </button>
    </div>
  </div>

  <div class="table-wrap">
    <table class="table">
      <thead>
        <tr>
          <th>Nama Registri</th>
          <th>URL Host</th>
          <th>Username</th>
          <th>Status</th>
          <th class="col-w-100 text-right">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#if loadingRegistries}
          <tr>
            <td colspan="5" class="text-center py-6">
              <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
              <p class="text-muted mt-2">Memuat daftar registri...</p>
            </td>
          </tr>
        {:else if registries.length === 0}
          <tr>
            <td colspan="5" class="text-center py-8">
              <div class="empty-state">
                <i class="ri-shield-keyhole-line empty-icon" aria-hidden="true"></i>
                <p class="empty-title">Tidak ada registri privat terdaftar</p>
                <p class="empty-desc">Hubungkan GHCR, GitLab Registry, Harbor, atau akun Docker Hub privat Anda.</p>
              </div>
            </td>
          </tr>
        {:else}
          {#each registries as reg (reg.id)}
            <tr>
              <td>
                <strong class="text-sm font-semibold">{reg.name}</strong>
              </td>
              <td>
                <span class="font-mono text-xs text-muted">{reg.url}</span>
              </td>
              <td>
                <span class="text-xs font-mono">{reg.username || '-'}</span>
              </td>
              <td>
                {#if reg.is_default}
                  <span class="badge badge--info">Default Publik</span>
                {:else if reg.logged_in}
                  <span class="badge badge--success">Terautentikasi</span>
                {:else}
                  <span class="badge badge--secondary">Terdaftar</span>
                {/if}
              </td>
              <td class="text-right">
                {#if !reg.is_default}
                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Logout & Hapus Registri"
                    aria-label="Hapus Registri"
                    disabled={actionLoading}
                    onclick={() => onDeleteRegistry(reg)}
                  >
                    <i class="ri-delete-bin-6-line text-danger" aria-hidden="true"></i>
                  </button>
                {:else}
                  <span class="text-xs text-muted">-</span>
                {/if}
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>
