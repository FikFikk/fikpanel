<script lang="ts">
  // Subkomponen tab Local Images Docker Studio: ukuran aktual, status pemakaian, Layers/Retag, Export/Import .tar, dan Prune
  import '../../styles/pages/docker/local-image.css';
  import type { DockerImage } from '../../lib/types';

  interface Props {
    images: DockerImage[];
    loadingImages: boolean;
    actionLoading: boolean;
    onOpenPullImage: () => void;
    onLoadImages: () => void;
    onRunImageAsContainer: (img: DockerImage) => void;
    onInspectImageHistory: (img: DockerImage) => void;
    onPruneDanglingImages: () => void;
    onImportImageTar: (file: File) => void;
    onDeleteImage: (img: DockerImage) => void;
    formatDate: (ts: number) => string;
  }

  let {
    images,
    loadingImages,
    actionLoading,
    onOpenPullImage,
    onLoadImages,
    onRunImageAsContainer,
    onInspectImageHistory,
    onPruneDanglingImages,
    onImportImageTar,
    onDeleteImage,
    formatDate,
  }: Props = $props();

  let imageSearch = $state('');
  let tarInputEl = $state<HTMLInputElement | null>(null);

  let filteredImages = $derived.by(() => {
    const q = imageSearch.trim().toLowerCase();
    if (!q) return images;
    return images.filter(
      (img) =>
        img.repository.toLowerCase().includes(q) ||
        img.tag.toLowerCase().includes(q) ||
        img.id.toLowerCase().includes(q),
    );
  });

  function exportImageHref(img: DockerImage): string {
    const ref = img.repository !== '<none>' && img.tag !== '<none>' ? `${img.repository}:${img.tag}` : img.id;
    return `/api/docker/images/export?ref=${encodeURIComponent(ref)}`;
  }

  function handleTarSelected(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (file) {
      onImportImageTar(file);
      input.value = '';
    }
  }
</script>

<div class="pg-docker-local-image tab-pane">
  <div class="toolbar">
    <div class="toolbar-left">
      <button type="button" class="btn btn--primary btn--sm" onclick={onOpenPullImage}>
        <i class="ri-download-line" aria-hidden="true"></i>
        <span>Tarik Citra Baru</span>
      </button>
      <button
        type="button"
        class="btn btn--secondary btn--sm"
        disabled={actionLoading}
        onclick={() => tarInputEl?.click()}
      >
        <i class="ri-upload-2-line" aria-hidden="true"></i>
        <span>Import .tar</span>
      </button>
      <input
        bind:this={tarInputEl}
        type="file"
        accept=".tar,.tar.gz,.tgz"
        class="hidden-file-input"
        aria-label="Unggah arsip tar citra Docker"
        onchange={handleTarSelected}
      />
      <button
        type="button"
        class="btn btn--danger btn--sm"
        disabled={actionLoading}
        onclick={onPruneDanglingImages}
      >
        <i class="ri-brush-line" aria-hidden="true"></i>
        <span>Prune Dangling</span>
      </button>
    </div>

    <div class="toolbar-right">
      <div class="search-box">
        <i class="ri-search-line search-icon" aria-hidden="true"></i>
        <input
          type="text"
          class="input input--sm"
          placeholder="Cari citra atau tag..."
          aria-label="Cari citra lokal"
          bind:value={imageSearch}
        />
      </div>
      <button
        type="button"
        class="btn btn--secondary btn--sm btn--icon"
        title="Segarkan citra lokal"
        aria-label="Segarkan citra lokal"
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
          <th>Status Pemakaian</th>
          <th>Image ID</th>
          <th>Ukuran</th>
          <th>Dibuat</th>
          <th class="col-w-200 text-right">Aksi</th>
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
        {:else if filteredImages.length === 0}
          <tr>
            <td colspan="7" class="text-center py-8">
              <div class="empty-state">
                <i class="ri-stack-line empty-icon" aria-hidden="true"></i>
                <p class="empty-title">Belum ada citra lokal</p>
                <p class="empty-desc">Tarik citra dari Docker Hub Explorer atau bangun langsung dari Dockerfile.</p>
                <button type="button" class="btn btn--primary btn--sm mt-3" onclick={onOpenPullImage}>
                  <i class="ri-download-line" aria-hidden="true"></i>
                  <span>Tarik Citra Pertama</span>
                </button>
              </div>
            </td>
          </tr>
        {:else}
          {#each filteredImages as img (img.id + ':' + img.repository + ':' + img.tag)}
            <tr>
              <td>
                <div class="img-repo-cell">
                  <strong class="font-mono text-sm">{img.repository}</strong>
                  {#if img.dangling}
                    <span class="badge badge--warning text-2xs">Dangling</span>
                  {/if}
                </div>
              </td>
              <td>
                <span class="badge badge--info font-mono">{img.tag}</span>
              </td>
              <td>
                {#if img.containers > 0}
                  <span class="badge badge--success">{img.containers} kontainer</span>
                {:else}
                  <span class="badge badge--secondary">Bebas</span>
                {/if}
              </td>
              <td>
                <span class="font-mono text-xs text-muted">{img.id.replace('sha256:', '').slice(0, 12)}</span>
              </td>
              <td>
                <span class="text-xs font-mono font-semibold">{img.size_human}</span>
              </td>
              <td>
                <span class="text-xs text-muted">{img.created_since || formatDate(img.created)}</span>
              </td>
              <td class="text-right">
                <div class="action-btns-row">
                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Jalankan sebagai Kontainer"
                    aria-label="Jalankan Citra sebagai Kontainer"
                    onclick={() => onRunImageAsContainer(img)}
                  >
                    <i class="ri-play-circle-line text-success" aria-hidden="true"></i>
                  </button>
                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Inspeksi Layers & Retag"
                    aria-label="Inspeksi Layers dan Retag Citra"
                    onclick={() => onInspectImageHistory(img)}
                  >
                    <i class="ri-stack-line text-info" aria-hidden="true"></i>
                  </button>
                  <a
                    href={exportImageHref(img)}
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Ekspor sebagai .tar"
                    aria-label="Ekspor citra sebagai berkas tar"
                  >
                    <i class="ri-download-2-line" aria-hidden="true"></i>
                  </a>
                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Hapus Citra Lokal"
                    aria-label="Hapus Citra Lokal"
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
