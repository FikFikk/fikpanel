<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/security/modals.css';

  interface Props {
    open: boolean;
    clientIP: string;
    port: string;
    protocol: string;
    action: string;
    direction: string;
    sourceIP: string;
    remarks: string;
    busy: boolean;
    onClose: () => void;
    onSubmit: () => void;
  }

  let {
    open,
    clientIP,
    port = $bindable(),
    protocol = $bindable(),
    action = $bindable(),
    direction = $bindable(),
    sourceIP = $bindable(),
    remarks = $bindable(),
    busy,
    onClose,
    onSubmit,
  }: Props = $props();

  const quickPresets = [
    { label: 'HTTP (80)', port: '80', proto: 'tcp', note: 'Webserver HTTP' },
    { label: 'HTTPS (443)', port: '443', proto: 'tcp', note: 'Webserver HTTPS' },
    { label: 'SSH (22)', port: '22', proto: 'tcp', note: 'Akses SSH' },
    { label: 'MySQL (3306)', port: '3306', proto: 'tcp', note: 'MySQL / MariaDB' },
    { label: 'PostgreSQL (5432)', port: '5432', proto: 'tcp', note: 'PostgreSQL' },
    { label: 'Redis (6379)', port: '6379', proto: 'tcp', note: 'Redis Cache' },
  ];

  function applyPreset(p: { port: string; proto: string; note: string }) {
    port = p.port;
    protocol = p.proto;
    if (!remarks.trim()) {
      remarks = p.note;
    }
  }
</script>

<Modal
  {open}
  title="Tambah Aturan Firewall (UFW)"
  subtitle="Terapkan aturan filter paket masuk atau keluar beserta pembatasan IP/CIDR."
  size="md"
  onclose={onClose}
>
  <div class="pg-security-modal">
    <div class="field">
      <span class="field__label">Preset Layanan Cepat</span>
      <div class="sec-preset-row">
        {#each quickPresets as preset}
          <button
            type="button"
            class="btn btn--secondary btn--sm"
            onclick={() => applyPreset(preset)}
          >
            {preset.label}
          </button>
        {/each}
      </div>
    </div>

    <div class="sec-modal-grid">
      <div class="field">
        <label class="field__label" for="fw-add-dir">Arah Trafik</label>
        <select id="fw-add-dir" class="select" bind:value={direction}>
          <option value="in">Inbound (Masuk)</option>
          <option value="out">Outbound (Keluar)</option>
        </select>
      </div>

      <div class="field">
        <label class="field__label" for="fw-add-act">Aksi</label>
        <select id="fw-add-act" class="select" bind:value={action}>
          <option value="allow">ALLOW (Izinkan)</option>
          <option value="deny">DENY (Tolak Diam-Diam)</option>
          <option value="limit">LIMIT (Rate-Limit Anti Brute Force)</option>
          <option value="reject">REJECT (Tolak Eksplisit)</option>
        </select>
      </div>
    </div>

    <div class="sec-modal-grid">
      <div class="field">
        <label class="field__label" for="fw-add-port">Port / Rentang Port</label>
        <input
          id="fw-add-port"
          type="text"
          class="input input--mono"
          placeholder="Contoh: 8080 atau 3000-4000"
          bind:value={port}
        />
        <span class="field__hint">Kosongkan bila ingin memblokir seluruh port dari satu IP spesifik.</span>
      </div>

      <div class="field">
        <label class="field__label" for="fw-add-proto">Protokol</label>
        <select id="fw-add-proto" class="select" bind:value={protocol}>
          <option value="tcp">TCP</option>
          <option value="udp">UDP</option>
          <option value="any">TCP &amp; UDP (ANY)</option>
        </select>
      </div>
    </div>

    <div class="field">
      <label class="field__label" for="fw-add-src">
        {direction === 'out' ? 'Alamat IP / CIDR Tujuan' : 'Alamat IP / CIDR Sumber'}
      </label>
      <div class="sec-input-inline">
        <input
          id="fw-add-src"
          type="text"
          class="input input--mono"
          placeholder="0.0.0.0/0 (Semua) atau 203.0.113.10/32"
          bind:value={sourceIP}
        />
        {#if clientIP}
          <button
            type="button"
            class="btn btn--secondary btn--sm"
            onclick={() => (sourceIP = `${clientIP}/32`)}
          >
            IP Saya
          </button>
        {/if}
      </div>
    </div>

    <div class="field">
      <label class="field__label" for="fw-add-remarks">Catatan / Komentar UFW <em>(opsional)</em></label>
      <input
        id="fw-add-remarks"
        type="text"
        class="input"
        placeholder="Contoh: Akses API internal kantor"
        bind:value={remarks}
      />
    </div>
  </div>

  {#snippet footer()}
    <button type="button" class="btn btn--secondary" disabled={busy} onclick={onClose}>Batal</button>
    <button type="button" class="btn btn--primary" disabled={busy} onclick={onSubmit}>
      {busy ? 'Menerapkan...' : 'Terapkan Aturan'}
    </button>
  {/snippet}
</Modal>
