<script lang="ts">
  /**
   * Subkomponen Modal Soft Link (Symbolic Link) untuk Halaman Files.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/files/symlink-modal.css';

  interface Props {
    open: boolean;
    symlinkTarget: string;
    symlinkName: string;
    currentPath: string;
    creatingSymlink: boolean;
    onClose: () => void;
    onSubmit: (e: Event) => void;
  }

  let {
    open,
    symlinkTarget = $bindable(),
    symlinkName = $bindable(),
    currentPath,
    creatingSymlink,
    onClose,
    onSubmit
  }: Props = $props();
</script>

<Modal
  {open}
  title="Buat Soft Link (Symbolic Link)"
  size="md"
  onclose={onClose}
>
  <form id="form-symlink" class="pg-files-symlink-modal" onsubmit={onSubmit}>
    <div class="field">
      <label class="field__label" for="f-sym-tgt">Path Target (Asal Berkas/Folder)</label>
      <input
        id="f-sym-tgt"
        class="input"
        type="text"
        placeholder="/var/log/nginx atau C:\data\app"
        bind:value={symlinkTarget}
        required
      />
    </div>
    <div class="field">
      <label class="field__label" for="f-sym-name">Nama Tautan Link</label>
      <input
        id="f-sym-name"
        class="input"
        type="text"
        placeholder="contoh: nginx_logs_link"
        bind:value={symlinkName}
        required
      />
    </div>
    <div class="field-meta">
      Lokasi pembuatan: <span class="mono">{currentPath}</span>
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
      form="form-symlink"
      class="btn btn--primary"
      disabled={creatingSymlink}
    >
      {creatingSymlink ? 'Membuat...' : 'Buat Tautan'}
    </button>
  {/snippet}
</Modal>

