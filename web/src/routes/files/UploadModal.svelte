<script lang="ts">
  /**
   * Subkomponen Modal Unggah Berkas untuk Halaman Files.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/files/upload-modal.css';

  interface Props {
    open: boolean;
    currentPath: string;
    uploadFiles: FileList | null;
    uploadSubmitting: boolean;
    onClose: () => void;
    onSubmit: (e: Event) => void;
  }

  let {
    open,
    currentPath,
    uploadFiles = $bindable(),
    uploadSubmitting,
    onClose,
    onSubmit
  }: Props = $props();
</script>

<Modal
  {open}
  title="Upload Berkas ke Server"
  size="md"
  onclose={onClose}
>
  <form id="form-upload-files" class="pg-files-upload-modal" onsubmit={onSubmit}>
    <div class="field">
      <label class="field__label" for="f-upload-dest">Direktori Tujuan</label>
      <input
        id="f-upload-dest"
        class="input"
        type="text"
        value={currentPath}
        disabled
      />
    </div>
    <div class="field">
      <label class="field__label" for="f-upload-input">Pilih Berkas</label>
      <input
        id="f-upload-input"
        class="input"
        type="file"
        multiple
        onchange={(e) => (uploadFiles = (e.target as HTMLInputElement).files)}
        required
      />
    </div>
  </form>
  {#snippet footer()}
    <button
      type="button"
      class="btn btn--ghost"
      onclick={onClose}>Batal</button
    >
    <button
      type="submit"
      form="form-upload-files"
      class="btn btn--primary"
      disabled={uploadSubmitting}
    >
      {uploadSubmitting ? "Mengunggah..." : "Mulai Upload"}
    </button>
  {/snippet}
</Modal>

