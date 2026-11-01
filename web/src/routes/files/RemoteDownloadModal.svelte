<script lang="ts">
  /**
   * Subkomponen Modal Remote Download untuk Halaman Files.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/files/remote-download-modal.css';

  interface Props {
    open: boolean;
    remoteUrl: string;
    remoteFilename: string;
    currentPath: string;
    remoteDownloading: boolean;
    onClose: () => void;
    onSubmit: (e: Event) => void;
  }

  let {
    open,
    remoteUrl = $bindable(),
    remoteFilename = $bindable(),
    currentPath,
    remoteDownloading,
    onClose,
    onSubmit
  }: Props = $props();
</script>

<Modal
  {open}
  title="Remote Download (Unduh URL)"
  size="md"
  onclose={onClose}
>
  <form id="form-remote-dl" class="pg-files-remote-download-modal" onsubmit={onSubmit}>
    <div class="field">
      <label class="field__label" for="f-rem-url">URL Berkas</label>
      <input
        id="f-rem-url"
        class="input"
        type="text"
        placeholder="https://example.com/package.tar.gz"
        bind:value={remoteUrl}
        required
      />
    </div>
    <div class="field">
      <label class="field__label" for="f-rem-fname">Nama Berkas Simpan (Opsional)</label>
      <input
        id="f-rem-fname"
        class="input"
        type="text"
        placeholder="Kosongkan untuk nama otomatis dari URL"
        bind:value={remoteFilename}
      />
    </div>
    <div class="field-meta">
      Tujuan: <span class="mono">{currentPath}</span>
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
      form="form-remote-dl"
      class="btn btn--primary"
      disabled={remoteDownloading}
    >
      {remoteDownloading ? 'Mengunduh...' : 'Mulai Unduh'}
    </button>
  {/snippet}
</Modal>

