<script lang="ts">
  // Subkomponen tab Local Image pada halaman Docker Management Suite
  import '../../styles/pages/docker/local-image.css';
  import type { DockerImage } from '../../lib/types';

  interface Props {
    images: DockerImage[];
    loadingImages: boolean;
    actionLoading: boolean;
    imageSearch: string;
    onOpenPullImage: () => void;
    onLoadImages: () => void;
    onOpenRunContainerFor: (img: DockerImage) => void;
    onDeleteImage: (img: DockerImage) => void;
  }

  let {
    images,
    loadingImages,
    actionLoading,
    imageSearch = $bindable(),
    onOpenPullImage,
    onLoadImages,
    onOpenRunContainerFor,
    onDeleteImage,
  }: Props = $props();
</script>

<div class="pg-docker-local-image tab-pane">
  <div class="toolbar">
    <div class="toolbar-left">
      <button class="btn btn--primary btn--sm" onclick={onOpenPullImage}>
        <i class="ri-download-cloud-line" aria-hidden="true"></i>
        <span>Tarik Citra Baru (Pull)</span>
      </button>
    </div>

    <div class="toolbar-right">
      <div class="search-box">
        <i class="ri-search-line search-icon" aria-hidden="true"></i>
        <input
          type="text"
          class="input input--sm"
          placeholder="Cari repositori atau tag..."
          bind:value={imageSearch}
        />
      </div>
      <button
        class="btn btn--secondary btn--sm btn--icon"
        title="Segarkan citra"
        aria-label="Segarkan citra"
        onclick={onLoadImages}
      >
        <i class="ri-refresh-line {loadingImages ? 'spin' : ''}" aria-hidden="true"></i>
      </button>
    </div>
  </div>

  <div class="table-wrap">
    <table class="table">
      <thead>
        <tr>
          <th>Repositori</th>
          <th>Tag</th>
          <th>Image ID</th>
          <th>Ukuran</th>
          <th>Terpakai</th>
          <th>Dibuat</th>
          <th class="col-w-180 text-right">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#if loadingImages}
          <tr>
            <td colspan="7" class="text-center py-6">
              <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
              <p class="text-muted mt-2">Memuat daftar citra lokal...</p>
            </td>
          </tr>
        {:else if images.length === 0}
          <tr>
            <td colspan="7" class="text-center py-8">
              <div class="empty-state">
                <i class="ri-hard-drive-2-line empty-icon" aria-hidden="true"></i>
                <p class="empty-title">Belum ada citra lokal</p>
                <p class="empty-desc">Tarik citra dari Docker Hub untuk mulai menggunakannya.</p>
                <button class="btn btn--primary btn--sm mt-3" onclick={onOpenPullImage}>
                  <i class="ri-download-cloud-line" aria-hidden="true"></i>
                  <span>Tarik Citra Sekarang</span>
                </button>
              </div>
            </td>
          </tr>
        {:else}
          {#each images as img}
            <tr>
              <td>
                <strong class="font-mono text-sm">{img.repository}</strong>
              </td>
              <td>
                <span class="badge badge--info font-mono">{img.tag}</span>
              </td>
              <td>
                <span class="font-mono text-xs text-muted">{img.id.slice(0, 12)}</span>
              </td>
              <td>
                <span class="text-xs font-semibold">{img.size_human}</span>
              </td>
              <td>
                {#if img.containers > 0}
                  <span class="badge badge--success">{img.containers} kontainer</span>
                {:else}
                  <span class="badge badge--secondary">0 (Unused)</span>
                {/if}
              </td>
              <td>
                <span class="text-xs text-muted">{new Date(img.created * 1000).toLocaleDateString()}</span>
              </td>
              <td class="text-right">
                <div class="action-btns-row">
                  <button
                    class="btn btn--secondary btn--sm"
                    title="Jalankan kontainer dari citra ini"
                    onclick={() => onOpenRunContainerFor(img)}
                  >
                    <i class="ri-play-line" aria-hidden="true"></i>
                    <span>Run</span>
                  </button>
                  <button
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Hapus Citra"
                    aria-label="Hapus Citra"
                    disabled={actionLoading}
                    onclick={() => onDeleteImage(img)}
                  >
                    <i class="ri-delete-bin-6-line text-danger" aria-hidden="true"></i>
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

