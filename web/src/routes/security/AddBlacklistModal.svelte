<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/security/add-blacklist-modal.css';

  interface Props {
    open: boolean;
    newBlackIP: string;
    newBlackReason: string;
    newBlackDuration: number;
    onClose: () => void;
    onSubmit: () => void;
  }

  let {
    open,
    newBlackIP = $bindable(),
    newBlackReason = $bindable(),
    newBlackDuration = $bindable(),
    onClose,
    onSubmit,
  }: Props = $props();
</script>

<Modal
  {open}
  title="Blokir Alamat IP (Blacklist)"
  size="sm"
  onclose={onClose}
>
  <div class="pg-security-add-blacklist-modal">
    <div class="field mb-3">
      <label class="field__label" for="black-ip">Alamat IP</label>
      <input
        id="black-ip"
        type="text"
        class="input input--mono"
        placeholder="Contoh: 185.220.101.44"
        bind:value={newBlackIP}
      />
    </div>
    <div class="field mb-3">
      <label class="field__label" for="black-reason">Alasan Pemblokiran</label>
      <input
        id="black-reason"
        type="text"
        class="input"
        placeholder="Contoh: Serangan brute force berulang"
        bind:value={newBlackReason}
      />
    </div>
    <div class="field mb-3">
      <label class="field__label" for="black-dur">Durasi Blokir (Menit)</label>
      <input
        id="black-dur"
        type="number"
        class="input input--mono"
        bind:value={newBlackDuration}
      />
    </div>
  </div>

  {#snippet footer()}
    <button class="btn btn--secondary" onclick={onClose}>Batal</button>
    <button class="btn btn--danger" onclick={onSubmit}>Blokir IP</button>
  {/snippet}
</Modal>

