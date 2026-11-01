<script lang="ts">
  /**
   * Subkomponen Modal Manajemen Favorit untuk Halaman Files.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/files/favorites-modal.css';

  interface Props {
    open: boolean;
    customFavorites: { name: string; path: string }[];
    onClose: () => void;
    onAddCurrent: () => void;
    onRemoveFavorite: (index: number) => void;
  }

  let {
    open,
    customFavorites,
    onClose,
    onAddCurrent,
    onRemoveFavorite
  }: Props = $props();
</script>

<Modal
  {open}
  title="Favorite Management"
  size="md"
  onclose={onClose}
>
  <div class="pg-files-favorites-modal">
    <div class="fav-add-action">
      <button
        class="btn btn--secondary w-full"
        onclick={onAddCurrent}
      >
        + Tambahkan Folder Saat Ini ke Favorit
      </button>
    </div>
    <div class="fav-list">
      {#each customFavorites as fav, idx}
        <div class="fav-row">
          <div>
            <div class="fav-row-name font-bold">{fav.name}</div>
            <div class="fav-row-path mono text-muted">
              {fav.path || 'Root'}
            </div>
          </div>
          <button
            class="btn-icon link-danger"
            onclick={() => onRemoveFavorite(idx)}
            title="Hapus"
          >
            <i class="ri-delete-bin-line"></i>
          </button>
        </div>
      {/each}
    </div>
  </div>
  {#snippet footer()}
    <button
      class="btn btn--primary"
      onclick={onClose}
    >Selesai</button>
  {/snippet}
</Modal>

