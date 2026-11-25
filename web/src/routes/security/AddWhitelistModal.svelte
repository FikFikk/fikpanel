<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/security/modals.css';

  interface Props {
    open: boolean;
    clientIP: string;
    newWhiteIP: string;
    newWhiteRemarks: string;
    busy: boolean;
    onClose: () => void;
    onSubmit: () => void;
  }

  let {
    open,
    clientIP,
    newWhiteIP = $bindable(),
    newWhiteRemarks = $bindable(),
    busy,
    onClose,
    onSubmit,
  }: Props = $props();
</script>

<Modal
  {open}
  title="Tambah IP ke Whitelist"
  subtitle="Alamat IP atau subnet CIDR yang masuk daftar putih tidak akan terkena rate-limit maupun auto-ban."
  size="sm"
  onclose={onClose}
>
  <div class="pg-security-modal">
    <div class="field">
      <label class="field__label" for="white-ip">Alamat IP / Subnet CIDR</label>
      <div class="sec-input-inline">
        <input
          id="white-ip"
          type="text"
          class="input input--mono"
          placeholder="Contoh: 202.59.205.41 atau 10.0.0.0/24"
          bind:value={newWhiteIP}
        />
        {#if clientIP}
          <button
            type="button"
            class="btn btn--secondary btn--sm"
            onclick={() => (newWhiteIP = clientIP)}
          >
            IP Saya
          </button>
        {/if}
      </div>
    </div>

    <div class="field">
      <label class="field__label" for="white-remarks">Catatan Keterangan</label>
      <input
        id="white-remarks"
        type="text"
        class="input"
        placeholder="Contoh: IP Kantor / VPN Administrator"
        bind:value={newWhiteRemarks}
      />
    </div>
  </div>

  {#snippet footer()}
    <button type="button" class="btn btn--secondary" disabled={busy} onclick={onClose}>Batal</button>
    <button type="button" class="btn btn--primary" disabled={busy} onclick={onSubmit}>
      {busy ? 'Menyimpan...' : 'Simpan ke Whitelist'}
    </button>
  {/snippet}
</Modal>
