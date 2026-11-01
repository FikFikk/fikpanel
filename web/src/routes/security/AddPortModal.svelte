<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/security/add-port-modal.css';

  interface Props {
    open: boolean;
    newPort: string;
    newProto: string;
    newStrategy: string;
    newDirection: string;
    newSourceIP: string;
    newRemarks: string;
    onClose: () => void;
    onSubmit: () => void;
  }

  let {
    open,
    newPort = $bindable(),
    newProto = $bindable(),
    newStrategy = $bindable(),
    newDirection = $bindable(),
    newSourceIP = $bindable(),
    newRemarks = $bindable(),
    onClose,
    onSubmit,
  }: Props = $props();
</script>

<Modal
  {open}
  title="Tambah Aturan Port Firewall"
  size="sm"
  onclose={onClose}
>
  <div class="pg-security-add-port-modal">
    <div class="field mb-3">
      <label class="field__label" for="rule-port">Nomor Port / Range Port</label>
      <input
        id="rule-port"
        type="text"
        class="input input--mono"
        placeholder="Contoh: 8080 atau 3000-4000"
        bind:value={newPort}
      />
    </div>

    <div class="field mb-3">
      <label class="field__label" for="rule-proto">Protokol</label>
      <select id="rule-proto" class="input" bind:value={newProto}>
        <option value="tcp">TCP</option>
        <option value="udp">UDP</option>
        <option value="any">TCP/UDP (ANY)</option>
      </select>
    </div>

    <div class="field mb-3">
      <label class="field__label" for="rule-strategy">Strategi Aksi</label>
      <select id="rule-strategy" class="input" bind:value={newStrategy}>
        <option value="Allow">ALLOW (Izinkan)</option>
        <option value="Deny">DENY (Tolak / Blokir)</option>
      </select>
    </div>

    <div class="field mb-3">
      <label class="field__label" for="rule-direction">Arah Paket (Direction)</label>
      <select id="rule-direction" class="input" bind:value={newDirection}>
        <option value="Inbound">Inbound (Masuk)</option>
        <option value="Outbound">Outbound (Keluar)</option>
      </select>
    </div>

    <div class="field mb-3">
      <label class="field__label" for="rule-source">Sumber IP</label>
      <input
        id="rule-source"
        type="text"
        class="input input--mono"
        placeholder="All atau misal 192.168.1.0/24"
        bind:value={newSourceIP}
      />
    </div>

    <div class="field mb-3">
      <label class="field__label" for="rule-remarks">Keterangan / Catatan</label>
      <input
        id="rule-remarks"
        type="text"
        class="input"
        placeholder="Contoh: Port API Backend Go"
        bind:value={newRemarks}
      />
    </div>
  </div>

  {#snippet footer()}
    <button class="btn btn--secondary" onclick={onClose}>Batal</button>
    <button class="btn btn--primary" onclick={onSubmit}>Terapkan Aturan</button>
  {/snippet}
</Modal>
