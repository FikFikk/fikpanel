<script lang="ts">
  /**
   * Subkomponen Modal Ganti Nama (Rename) untuk Halaman Files.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { FileItem } from '../../lib/types';
  import '../../styles/pages/files/rename-modal.css';

  interface Props {
    open: boolean;
    renameTarget: FileItem | null;
    renameNewName: string;
    onClose: () => void;
    onSubmit: (e: Event) => void;
  }

  let {
    open,
    renameTarget,
    renameNewName = $bindable(),
    onClose,
    onSubmit
  }: Props = $props();
</script>

<Modal
  {open}
  title="Ganti Nama (Rename)"
  size="md"
  onclose={onClose}
>
  {#if renameTarget}
    <form id="form-rename" class="pg-files-rename-modal" onsubmit={onSubmit}>
      <div class="field">
        <label class="field__label" for="f-old-name">Nama Saat Ini</label>
        <input
          id="f-old-name"
          class="input"
          type="text"
          value={renameTarget.name}
          disabled
        />
      </div>
      <div class="field">
        <label class="field__label" for="f-new-rname">Nama Baru</label>
        <input
          id="f-new-rname"
          class="input"
          type="text"
          bind:value={renameNewName}
          required
        />
      </div>
    </form>
  {/if}
  {#snippet footer()}
    <button
      type="button"
      class="btn btn--ghost"
      onclick={onClose}
    >Batal</button>
    <button
      type="submit"
      form="form-rename"
      class="btn btn--primary"
    >
      Simpan Nama Baru
    </button>
  {/snippet}
</Modal>

