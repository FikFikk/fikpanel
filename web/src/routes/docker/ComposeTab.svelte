<script lang="ts">
  // Subkomponen tab Docker Compose pada halaman Docker Management Suite
  import '../../styles/pages/docker/compose.css';
  import type { DockerComposeProject } from '../../lib/types';

  interface Props {
    composeProjects: DockerComposeProject[];
    loadingCompose: boolean;
    actionLoading: boolean;
    onOpenComposeModal: () => void;
    onLoadComposeProjects: () => void;
    onEditComposeProject: (proj: DockerComposeProject) => void;
    onComposeDown: (proj: DockerComposeProject) => void;
    onComposeUpForProject: (proj: DockerComposeProject) => void;
  }

  let {
    composeProjects,
    loadingCompose,
    actionLoading,
    onOpenComposeModal,
    onLoadComposeProjects,
    onEditComposeProject,
    onComposeDown,
    onComposeUpForProject,
  }: Props = $props();
</script>

<div class="pg-docker-compose tab-pane">
  <div class="toolbar">
    <div class="toolbar-left">
      <button class="btn btn--primary btn--sm" onclick={onOpenComposeModal}>
        <i class="ri-add-line" aria-hidden="true"></i>
        <span>Tambah Stack Compose</span>
      </button>
    </div>
    <div class="toolbar-right">
      <button
        class="btn btn--secondary btn--sm btn--icon"
        title="Segarkan Compose"
        aria-label="Segarkan Compose"
        onclick={onLoadComposeProjects}
      >
        <i class="ri-refresh-line {loadingCompose ? 'spin' : ''}" aria-hidden="true"></i>
      </button>
    </div>
  </div>

  <div class="table-wrap">
    <table class="table">
      <thead>
        <tr>
          <th>Nama Proyek</th>
          <th>Status</th>
          <th>Lokasi Berkas</th>
          <th>Kontainer Aktif</th>
          <th class="col-w-200 text-right">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#if loadingCompose}
          <tr>
            <td colspan="5" class="text-center py-6">
              <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
              <p class="text-muted mt-2">Memuat proyek compose...</p>
            </td>
          </tr>
        {:else if composeProjects.length === 0}
          <tr>
            <td colspan="5" class="text-center py-8">
              <div class="empty-state">
                <i class="ri-git-merge-line empty-icon" aria-hidden="true"></i>
                <p class="empty-title">Belum ada proyek Docker Compose</p>
                <p class="empty-desc">Definisikan stack multi-kontainer menggunakan format YAML standar.</p>
                <button class="btn btn--primary btn--sm mt-3" onclick={onOpenComposeModal}>
                  <i class="ri-add-line" aria-hidden="true"></i>
                  <span>Buat Stack Baru</span>
                </button>
              </div>
            </td>
          </tr>
        {:else}
          {#each composeProjects as proj}
            <tr>
              <td>
                <strong class="font-semibold text-sm">{proj.name}</strong>
              </td>
              <td>
                {#if proj.status === 'running'}
                  <span class="badge badge--success">
                    <span class="status-dot status-dot--active"></span>
                    Running
                  </span>
                {:else}
                  <span class="badge badge--secondary">Stopped</span>
                {/if}
              </td>
              <td>
                <span class="text-xs font-mono text-muted">{proj.config_path}</span>
              </td>
              <td>
                <span class="badge badge--info">{proj.containers_count} service</span>
              </td>
              <td class="text-right">
                <div class="action-btns-row">
                  <button
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Edit YAML Stack"
                    aria-label="Edit YAML Stack"
                    onclick={() => onEditComposeProject(proj)}
                  >
                    <i class="ri-edit-line" aria-hidden="true"></i>
                  </button>
                  {#if proj.status === 'running'}
                    <button
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Hentikan Stack (Down)"
                      aria-label="Hentikan Stack"
                      disabled={actionLoading}
                      onclick={() => onComposeDown(proj)}
                    >
                      <i class="ri-stop-circle-line text-warning" aria-hidden="true"></i>
                    </button>
                  {:else}
                    <button
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Jalankan Stack (Up)"
                      aria-label="Jalankan Stack"
                      disabled={actionLoading}
                      onclick={() => onComposeUpForProject(proj)}
                    >
                      <i class="ri-play-circle-line text-accent" aria-hidden="true"></i>
                    </button>
                  {/if}
                </div>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>

