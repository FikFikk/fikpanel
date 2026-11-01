<script lang="ts">
  /**
   * Subkomponen Modal Kompresi Berkas ke Zip untuk Halaman Files.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { FileItem } from '../../lib/types';
  import '../../styles/pages/files/compress-modal.css';

  interface Props {
    open: boolean;
    compressTarget: FileItem | null;
    compressZipName: string;
    currentPath: string;
    compressing: boolean;
    onClose: () => void;
    onSubmit: (e: Event) => void;
  }

  let {
    open,
    compressTarget,
    compressZipName = $bindable(),
    currentPath,
    compressing,
    onClose,
    onSubmit
  }: Props = $props();
</script>

<Modal
  {open}
  title="Kompresi Berkas ke Zip"
  size="md"
  onclose={onClose}
>
  {#if compressTarget}
    <form id="form-compress" class="pg-files-compress-modal" onsubmit={onSubmit}>
      <div class="field">
        <label class="field__label" for="f-zip-src">Item Sumber</label>
        <input
          id="f-zip-src"
          class="input"
          type="text"
          value={compressTarget.name}
          disabled
        />
      </div>
      <div class="field">
        <label class="field__label" for="f-zip-name">Nama Arsip Zip</label>
        <input
          id="f-zip-name"
          class="input"
          type="text"
          bind:value={compressZipName}
          required
        />
      </div>
      <div class="field-meta">
        Lokasi output: <span class="mono">{currentPath}</span>
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
      form="form-compress"
      class="btn btn--primary"
      disabled={compressing}
    >
      {compressing ? 'Mengompresi...' : 'Mulai Kompresi'}
    </button>
  {/snippet}
</Modal>

