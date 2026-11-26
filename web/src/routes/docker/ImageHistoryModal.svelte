<script lang="ts">
  // Subkomponen modal Inspektor Layer (`docker image history`) & Retag Citra Lokal
  import '../../styles/pages/docker/image-history-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DockerImage, DockerImageHistoryItem } from '../../lib/types';

  interface Props {
    open: boolean;
    image: DockerImage | null;
    history: DockerImageHistoryItem[];
    loading: boolean;
    actionLoading: boolean;
    onClose: () => void;
    onRetag: (source: string, target: string) => void;
  }

  let { open, image, history, loading, actionLoading, onClose, onRetag }: Props = $props();
  let newTag = $state('');

  $effect(() => {
    if (open && image) {
      const repo = image.repository !== '<none>' ? image.repository : 'custom-image';
      newTag = `${repo}:backup`;
    }
  });

  function submitRetag(e: Event) {
    e.preventDefault();
    if (!image || !newTag.trim()) return;
    const src = image.repository !== '<none>' ? `${image.repository}:${image.tag}` : image.id;
    onRetag(src, newTag.trim());
  }
</script>

<Modal {open} size="xl" onclose={onClose}>
  {#snippet header()}
    <div class="flex-row items-center gap-2 flex-1">
      <i class="ri-stack-line text-accent" aria-hidden="true"></i>
      <h2 class="modal__title">
        Layers & Tag: {image ? `${image.repository}:${image.tag}` : '...'}
      </h2>
      {#if image}
        <span class="badge badge--info text-2xs">{image.size_human}</span>
      {/if}
    </div>
  {/snippet}

  <div class="pg-docker-image-history-modal">
    {#if image}
      <form class="retag-bar" onsubmit={submitRetag}>
        <span class="text-xs font-semibold">Retag Citra:</span>
        <input
          type="text"
          class="input input--sm flex-1 font-mono"
          placeholder="misal: repo-anda/app:v1.0"
          aria-label="Nama tag citra baru"
          bind:value={newTag}
          required
        />
        <button type="submit" class="btn btn--primary btn--sm" disabled={actionLoading}>
          <i class="ri-price-tag-3-line" aria-hidden="true"></i>
          <span>Buat Tag Baru</span>
        </button>
      </form>
    {/if}

    {#if loading}
      <div class="text-center py-8">
        <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
        <p class="text-muted mt-2">Membaca struktur layer citra...</p>
      </div>
    {:else}
      <div class="layers-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>Layer</th>
              <th>Instruksi Build</th>
              <th>Usia</th>
              <th class="text-right">Ukuran</th>
            </tr>
          </thead>
          <tbody>
            {#each history as item, idx}
              <tr>
                <td class="font-mono text-2xs text-muted">#{history.length - idx}</td>
                <td><div class="layer-cmd">{item.created_by || '-'}</div></td>
                <td class="text-xs text-muted">{item.created_since}</td>
                <td class="text-right font-mono text-xs">{item.size_human}</td>
              </tr>
            {:else}
              <tr>
                <td colspan="4" class="text-center py-6 text-muted">Riwayat layer tidak tersedia.</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>

  {#snippet footer()}
    <button type="button" class="btn btn--secondary btn--sm" onclick={onClose}>
      Tutup
    </button>
  {/snippet}
</Modal>
