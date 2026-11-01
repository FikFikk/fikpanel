<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/security/add-whitelist-modal.css';

  interface Props {
    open: boolean;
    newWhiteIP: string;
    newWhiteRemarks: string;
    onClose: () => void;
    onSubmit: () => void;
  }

  let {
    open,
    newWhiteIP = $bindable(),
    newWhiteRemarks = $bindable(),
    onClose,
    onSubmit,
  }: Props = $props();
</script>

<Modal
  {open}
  title="Tambah IP ke Whitelist"
  size="sm"
  onclose={onClose}
>
  <div class="pg-security-add-whitelist-modal">
    <div class="field mb-3">
      <label class="field__label" for="white-ip">Alamat IP / Subnet</label>
      <input
        id="white-ip"
        type="text"
        class="input input--mono"
        placeholder="Contoh: 202.59.205.41"
        bind:value={newWhiteIP}
      />
    </div>
    <div class="field mb-3">
      <label class="field__label" for="white-remarks">Catatan Keterangan</label>
      <input
        id="white-remarks"
        type="text"
        class="input"
        placeholder="Contoh: Kantor Utama Fikri"
        bind:value={newWhiteRemarks}
      />
    </div>
  </div>

  {#snippet footer()}
    <button class="btn btn--secondary" onclick={onClose}>Batal</button>
    <button class="btn btn--primary" onclick={onSubmit}>Simpan Whitelist</button>
  {/snippet}
</Modal>
