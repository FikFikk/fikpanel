<script lang="ts">
  // Subkomponen tab Docker Compose Studio: auto-discovery stack host + lifecycle lengkap (Up/Restart/Pull/Stop/Down/Logs)
  import '../../styles/pages/docker/compose.css';
  import type { DockerComposeProject } from '../../lib/types';

  interface Props {
    composeProjects: DockerComposeProject[];
    loadingCompose: boolean;
    actionLoading: boolean;
    onOpenComposeModal: () => void;
    onLoadComposeProjects: () => void;
    onEditComposeProject: (proj: DockerComposeProject) => void;
    onComposeAction: (proj: DockerComposeProject, action: 'up' | 'down' | 'restart' | 'stop' | 'start' | 'pull') => void;
    onOpenComposeLogs: (proj: DockerComposeProject) => void;
  }

  let {
    composeProjects,
    loadingCompose,
    actionLoading,
    onOpenComposeModal,
    onLoadComposeProjects,
    onEditComposeProject,
    onComposeAction,
    onOpenComposeLogs,
  }: Props = $props();
</script>

<div class="pg-docker-compose tab-pane">
  <div class="toolbar">
    <div class="toolbar-left">
      <button type="button" class="btn btn--primary btn--sm" onclick={onOpenComposeModal}>
        <i class="ri-add-line" aria-hidden="true"></i>
        <span>Tambah Stack Compose</span>
      </button>
    </div>
    <div class="toolbar-right">
      <button
        type="button"
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
          <th>Nama Stack</th>
          <th>Status</th>
          <th>Lokasi Berkas YAML</th>
          <th>Layanan Aktif</th>
          <th class="col-w-240 text-right">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#if loadingCompose}
          <tr>
            <td colspan="5" class="text-center py-6">
              <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
              <p class="text-muted mt-2">Memindai seluruh stack Docker Compose di host...</p>
            </td>
          </tr>
        {:else if composeProjects.length === 0}
          <tr>
            <td colspan="5" class="text-center py-8">
              <div class="empty-state">
                <i class="ri-git-merge-line empty-icon" aria-hidden="true"></i>
                <p class="empty-title">Belum ada proyek Docker Compose</p>
                <p class="empty-desc">Definisikan stack multi-kontainer menggunakan YAML atau pasang dari App Store 1-Klik.</p>
                <button type="button" class="btn btn--primary btn--sm mt-3" onclick={onOpenComposeModal}>
                  <i class="ri-add-line" aria-hidden="true"></i>
                  <span>Buat Stack Baru</span>
                </button>
              </div>
            </td>
          </tr>
        {:else}
          {#each composeProjects as proj (proj.name)}
            <tr>
              <td>
                <div class="flex-row items-center gap-2">
                  <strong class="font-semibold text-sm">{proj.name}</strong>
                  {#if proj.managed}
                    <span class="badge badge--secondary text-2xs">FikPanel</span>
                  {:else}
                    <span class="badge badge--info text-2xs">Host</span>
                  {/if}
                </div>
              </td>
              <td>
                {#if proj.status === 'running'}
                  <span class="badge badge--success">
                    <span class="status-dot status-dot--active"></span>
                    Running
                  </span>
                {:else if proj.status === 'partial'}
                  <span class="badge badge--warning">Partial ({proj.running_count ?? 0}/{proj.containers_count})</span>
                {:else}
                  <span class="badge badge--secondary">Stopped</span>
                {/if}
              </td>
              <td>
                <span class="text-xs font-mono text-muted">{proj.config_path || '-'}</span>
              </td>
              <td>
                <span class="badge badge--info font-mono">
                  {proj.running_count ?? 0}/{proj.containers_count} kontainer
                </span>
              </td>
              <td class="text-right">
                <div class="action-btns-row">
                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Lihat Log Stack"
                    aria-label="Lihat Log Stack Compose"
                    onclick={() => onOpenComposeLogs(proj)}
                  >
                    <i class="ri-file-list-3-line" aria-hidden="true"></i>
                  </button>
                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Edit YAML Stack"
                    aria-label="Edit YAML Stack"
                    onclick={() => onEditComposeProject(proj)}
                  >
                    <i class="ri-edit-line" aria-hidden="true"></i>
                  </button>
                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Pull Image Terbaru & Recreate"
                    aria-label="Pull dan Perbarui Stack"
                    disabled={actionLoading}
                    onclick={() => onComposeAction(proj, 'pull')}
                  >
                    <i class="ri-download-cloud-2-line text-info" aria-hidden="true"></i>
                  </button>
                  {#if proj.status === 'running' || proj.status === 'partial'}
                    <button
                      type="button"
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Restart Stack"
                      aria-label="Restart Stack"
                      disabled={actionLoading}
                      onclick={() => onComposeAction(proj, 'restart')}
                    >
                      <i class="ri-restart-line text-info" aria-hidden="true"></i>
                    </button>
                    <button
                      type="button"
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Hentikan Stack (Down)"
                      aria-label="Hentikan Stack"
                      disabled={actionLoading}
                      onclick={() => onComposeAction(proj, 'down')}
                    >
                      <i class="ri-stop-circle-line text-warning" aria-hidden="true"></i>
                    </button>
                  {:else}
                    <button
                      type="button"
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Jalankan Stack (Up)"
                      aria-label="Jalankan Stack"
                      disabled={actionLoading}
                      onclick={() => onComposeAction(proj, 'up')}
                    >
                      <i class="ri-play-circle-line text-success" aria-hidden="true"></i>
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
