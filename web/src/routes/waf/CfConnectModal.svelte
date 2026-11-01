<script lang="ts">
  // Subkomponen modal koneksi API Token / Global API Key Cloudflare
  import '../../styles/pages/waf/cf-connect-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';

  interface Props {
    showCFConnectModal: boolean;
    cfTokenInput: string;
    cfEmailInput: string;
    cfSubmitting: boolean;
    onConnectCF: () => void;
  }

  let {
    showCFConnectModal = $bindable(),
    cfTokenInput = $bindable(),
    cfEmailInput = $bindable(),
    cfSubmitting,
    onConnectCF,
  }: Props = $props();
</script>

<div class="pg-waf-cf-connect-modal">
  <!-- Modal Hubungkan Cloudflare Token / Global API Key -->
  <Modal
    open={showCFConnectModal}
    title="Hubungkan Akun Cloudflare (API Token / Global API Key)"
    subtitle="Mendukung Custom API Token maupun Global API Key + Email"
    size="lg"
    onclose={() => (showCFConnectModal = false)}
  >
    <div class="cf-token-guide-box mb-4">
      <div class="font-semibold text-xs mb-1">
        <i class="ri-shield-keyhole-line text-accent"></i> Panduan Izin API Token Cloudflare (Read vs Edit):
      </div>
      <ul class="cf-guide-list text-xs text-subtle">
        <li>
          <strong>Mengapa perlu izin <code>Edit</code>?</strong> Izin <code>Read</code> saja hanya dapat membaca daftar domain & DNS, tetapi <strong>tidak dapat</strong> mengubah DNS 1-klik, menambah subdomain, mengubah mode SSL, menyalakan <em>Under Attack Mode</em>, atau <em>Purge Cache</em>.
        </li>
        <li>
          Saat membuat token di <strong>dash.cloudflare.com/profile/api-tokens</strong> (<code>Create Custom Token</code>), tambahkan 4 baris izin berikut:
          <div class="mono text-2xs mt-1 p-2 bg-inset-box">
            1. Zone → Zone → Read (Wajib agar daftar domain terdeteksi)<br/>
            2. Zone → DNS → Edit (Untuk tambah/ubah/point record DNS & toggle Proxy)<br/>
            3. Zone → Zone Settings → Edit (Untuk ubah Mode SSL, Under Attack, Always HTTPS, Brotli)<br/>
            4. Zone → Cache Purge → Purge (Untuk bersihkan cache CDN)<br/>
            *Pada bagian Zone Resources, pilih: Include → All zones
          </div>
        </li>
        <li>
          <strong>Alternatif Paling Mudah:</strong> Anda juga dapat memasukkan <strong>Global API Key</strong> pada kolom Token di bawah disertai <strong>Email Akun Cloudflare</strong> Anda (otomatis memiliki akses penuh ke seluruh domain tanpa perlu mengatur permission satu per satu).
        </li>
      </ul>
    </div>

    <div class="form-group">
      <label for="cf-token">Cloudflare API Token atau Global API Key (Wajib)</label>
      <input
        id="cf-token"
        type="password"
        bind:value={cfTokenInput}
        placeholder="Tempel API Token (40+ karakter) atau Global API Key (37 karakter hex)..."
        class="input-dark mono"
      />
    </div>

    <div class="form-group mt-4">
      <label for="cf-email">Email Akun Cloudflare (Wajib jika memakai Global API Key, Opsional untuk API Token)</label>
      <input
        id="cf-email"
        type="email"
        bind:value={cfEmailInput}
        placeholder="user@example.com"
        class="input-dark"
      />
    </div>

    {#snippet footer()}
      <button type="button" class="btn btn-ghost" onclick={() => (showCFConnectModal = false)}>Batal</button>
      <button type="button" class="btn btn-primary" onclick={onConnectCF} disabled={cfSubmitting}>
        {cfSubmitting ? 'Memverifikasi & Menyinkronkan...' : 'Verifikasi & Hubungkan'}
      </button>
    {/snippet}
  </Modal>
</div>

