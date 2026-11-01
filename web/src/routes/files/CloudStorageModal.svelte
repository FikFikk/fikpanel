<script lang="ts">
  /**
   * Subkomponen Modal Konektor Penyimpanan Awan (Cloud Storage) untuk Halaman Files.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/files/cloud-storage-modal.css';

  interface Props {
    open: boolean;
    selectedCloud: string;
    onClose: () => void;
    onConnect: () => void;
  }

  let {
    open,
    selectedCloud,
    onClose,
    onConnect
  }: Props = $props();
</script>

<Modal
  {open}
  title={`${selectedCloud} Connector`}
  size="md"
  onclose={onClose}
>
  <div class="pg-files-cloud-storage-modal">
    <i class="ri-cloud-line cloud-huge-icon"></i>
    <p>
      Konektor penyimpanan awan untuk <strong>{selectedCloud}</strong> telah terintegrasi.
    </p>
    <div class="field text-left">
      <label class="field__label" for="f-cloud-ep">Storage Bucket / Endpoint</label>
      <input
        id="f-cloud-ep"
        class="input"
        type="text"
        placeholder="misal: my-backup-bucket atau ftp.domain.com"
      />
    </div>
    <div class="field text-left">
      <label class="field__label" for="f-cloud-key">Access Key / Credential</label>
      <input
        id="f-cloud-key"
        class="input"
        type="password"
        placeholder="API Key / Password"
      />
    </div>
  </div>
  {#snippet footer()}
    <button
      class="btn btn--ghost"
      onclick={onClose}
    >Batal</button>
    <button
      class="btn btn--primary"
      onclick={onConnect}
    >
      Hubungkan Sekarang
    </button>
  {/snippet}
</Modal>

