<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/security/modals.css';

  interface Props {
    open: boolean;
    newBlackIP: string;
    newBlackReason: string;
    newBlackDuration: number;
    newBlackSyncUFW: boolean;
    busy: boolean;
    onClose: () => void;
    onSubmit: () => void;
  }

  let {
    open,
    newBlackIP = $bindable(),
    newBlackReason = $bindable(),
    newBlackDuration = $bindable(),
    newBlackSyncUFW = $bindable(),
    busy,
    onClose,
    onSubmit,
  }: Props = $props();

  const durationPresets = [
    { label: '1 Jam', mins: 60 },
    { label: '24 Jam', mins: 1440 },
    { label: '7 Hari', mins: 10080 },
    { label: 'Permanen', mins: 0 },
  ];
</script>

<Modal
  {open}
  title="Blokir Alamat IP (Blacklist)"
  subtitle="Alamat IP akan langsung ditolak (HTTP 403) pada seluruh endpoint panel dan login."
  size="sm"
  onclose={onClose}
>
  <div class="pg-security-modal">
    <div class="field">
      <label class="field__label" for="black-ip">Alamat IP / Subnet CIDR</label>
      <input
        id="black-ip"
        type="text"
        class="input input--mono"
        placeholder="Contoh: 185.220.101.44 atau 194.26.29.0/24"
        bind:value={newBlackIP}
      />
    </div>

    <div class="field">
      <label class="field__label" for="black-reason">Alasan Pemblokiran</label>
      <input
        id="black-reason"
        type="text"
        class="input"
        placeholder="Contoh: Serangan brute force berulang"
        bind:value={newBlackReason}
      />
    </div>

    <div class="field">
      <label class="field__label" for="black-dur">Durasi Blokir (Menit, 0 = Permanen)</label>
      <input
        id="black-dur"
        type="number"
        min="0"
        class="input input--mono"
        bind:value={newBlackDuration}
      />
      <div class="sec-preset-row">
        {#each durationPresets as d}
          <button
            type="button"
            class="btn btn--secondary btn--sm"
            onclick={() => (newBlackDuration = d.mins)}
          >
            {d.label}
          </button>
        {/each}
      </div>
    </div>

    <label class="sec-check-box">
      <input type="checkbox" bind:checked={newBlackSyncUFW} />
      <span>
        <strong>Sinkronkan ke Firewall UFW (Linux)</strong> — sekaligus menyisipkan aturan <code>ufw insert 1 deny from IP</code> agar trafik ke seluruh port ikut terblokir.
      </span>
    </label>
  </div>

  {#snippet footer()}
    <button type="button" class="btn btn--secondary" disabled={busy} onclick={onClose}>Batal</button>
    <button type="button" class="btn btn--danger" disabled={busy} onclick={onSubmit}>
      {busy ? 'Memblokir...' : 'Blokir Alamat IP'}
    </button>
  {/snippet}
</Modal>
