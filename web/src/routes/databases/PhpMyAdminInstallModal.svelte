<script lang="ts">
  /**
   * Subkomponen Modal Quick Install phpMyAdmin Resmi.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/databases/phpmyadmin-install-modal.css';

  interface Props {
    open: boolean;
    installingPma: boolean;
    installPmaVersion: string;
    installPmaPhp: string;
    installPmaMode: string;
    onClose: () => void;
    onSubmit: () => void;
  }

  let {
    open,
    installingPma,
    installPmaVersion = $bindable(),
    installPmaPhp = $bindable(),
    installPmaMode = $bindable(),
    onClose,
    onSubmit
  }: Props = $props();
</script>

<Modal
  {open}
  size="md"
  onclose={() => { if (!installingPma) onClose(); }}
>
  {#snippet header()}
    <div class="modal-title-with-icon">
      <i class="ri-database-2-line engine-install-icon" aria-hidden="true"></i>
      <div>
        <h3 class="modal__title">Quick Install phpMyAdmin Resmi</h3>
        <p class="modal__subtitle">Pasang paket resmi phpMyAdmin dari repositori ke server</p>
      </div>
    </div>
  {/snippet}

  <div class="pg-databases-pma-install-modal">
    <div class="form-group">
      <label for="pma-install-ver" class="form-label">Pilih Versi phpMyAdmin</label>
      <select id="pma-install-ver" class="form-select" bind:value={installPmaVersion} disabled={installingPma}>
        <option value="5.2.1">phpMyAdmin 5.2.1 (Rekomendasi Stabil - PHP 7.2 - 8.3)</option>
        <option value="5.1.4">phpMyAdmin 5.1.4 (LTS Kompatibilitas Luas)</option>
      </select>
    </div>

    <div class="form-group">
      <label for="pma-install-php" class="form-label">Versi PHP Runtime</label>
      <select id="pma-install-php" class="form-select" bind:value={installPmaPhp} disabled={installingPma}>
        <option value="8.2">PHP 8.2 (FPM - Rekomendasi Keamanan)</option>
        <option value="8.1">PHP 8.1 (FPM - Standar Stabil)</option>
        <option value="8.0">PHP 8.0 (FPM - Legacy)</option>
      </select>
    </div>

    <div class="form-group">
      <span class="form-label">Metode Pemasangan</span>
      <div class="install-mode-cards">
        <label class="mode-card" class:selected={installPmaMode === 'native'}>
          <input
            type="radio"
            name="pma-install-mode"
            value="native"
            bind:group={installPmaMode}
            disabled={installingPma}
          />
          <div class="mode-card-content">
            <div class="mode-card-title">
              <i class="ri-download-cloud-line" aria-hidden="true"></i>
              <span>Fast Install (Rekomendasi)</span>
            </div>
            <p class="mode-card-desc">
              Mengunduh paket resmi phpMyAdmin dari situs resmi dan mengonfigurasi FastCGI Nginx secara otomatis.
            </p>
          </div>
        </label>
        <label class="mode-card" class:selected={installPmaMode === 'docker'}>
          <input
            type="radio"
            name="pma-install-mode"
            value="docker"
            bind:group={installPmaMode}
            disabled={installingPma}
          />
          <div class="mode-card-content">
            <div class="mode-card-title">
              <i class="ri-ubuntu-line" aria-hidden="true"></i>
              <span>Container (Docker)</span>
            </div>
            <p class="mode-card-desc">
              Menjalankan instance image phpmyadmin/phpmyadmin terisolasi dalam container Docker.
            </p>
          </div>
        </label>
      </div>
    </div>

    <div class="install-notice">
      <i class="ri-information-line" aria-hidden="true"></i>
      <span>
        Instalasi berlangsung di latar belakang melalui Antrean Tugas (Messages Box). Anda dapat terus menggunakan panel saat proses berjalan.
      </span>
    </div>
  </div>

  {#snippet footer()}
    <button
      class="btn btn-outline"
      disabled={installingPma}
      onclick={onClose}
    >
      Batal
    </button>
    <button class="btn btn-primary" disabled={installingPma} onclick={onSubmit}>
      {#if installingPma}
        <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
        <span>Memulai...</span>
      {:else}
        <i class="ri-play-fill" aria-hidden="true"></i>
        <span>Mulai Instalasi (Install Now)</span>
      {/if}
    </button>
  {/snippet}
</Modal>

