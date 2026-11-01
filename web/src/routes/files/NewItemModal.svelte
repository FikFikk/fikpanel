<script lang="ts">
  /**
   * Subkomponen Modal Pembuatan File / Folder Baru untuk Halaman Files.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/files/new-item-modal.css';

  interface Props {
    open: boolean;
    newType: 'file' | 'dir';
    newName: string;
    currentPath: string;
    onClose: () => void;
    onSubmit: (e: Event) => void;
  }

  let {
    open,
    newType = $bindable(),
    newName = $bindable(),
    currentPath,
    onClose,
    onSubmit
  }: Props = $props();
</script>

<Modal
  {open}
  title={`Buat ${newType === 'dir' ? 'Folder' : 'Berkas'} Baru`}
  size="md"
  onclose={onClose}
>
  <form id="form-create-new" class="pg-files-new-item-modal" onsubmit={onSubmit}>
    <div class="field">
      <label class="field__label" for="f-new-type">Tipe Item</label>
      <div class="type-segmented">
        <button
          type="button"
          class="seg-btn {newType === 'file' ? 'active' : ''}"
          onclick={() => (newType = 'file')}
        >
          <i class="ri-file-line"></i> Berkas (File)
        </button>
        <button
          type="button"
          class="seg-btn {newType === 'dir' ? 'active' : ''}"
          onclick={() => (newType = 'dir')}
        >
          <i class="ri-folder-line"></i> Direktori (Folder)
        </button>
      </div>
    </div>
    <div class="field">
      <label class="field__label" for="f-new-name"
        >Nama {newType === 'dir' ? 'Folder' : 'Berkas'}</label
      >
      <input
        id="f-new-name"
        class="input"
        type="text"
        placeholder={newType === 'dir'
          ? 'contoh: config atau assets'
          : 'contoh: index.html atau .env'}
        bind:value={newName}
        required
      />
    </div>
    <div class="field-meta">
      Lokasi: <span class="mono">{currentPath}</span>
    </div>
  </form>
  {#snippet footer()}
    <button
      type="button"
      class="btn btn--ghost"
      onclick={onClose}
    >Batal</button>
    <button
      type="submit"
      form="form-create-new"
      class="btn btn--primary"
    >
      Buat Sekarang
    </button>
  {/snippet}
</Modal>

