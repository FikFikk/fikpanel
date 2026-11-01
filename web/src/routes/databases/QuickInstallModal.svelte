<script lang="ts">
  /**
   * Subkomponen Modal Instalasi Cepat (Quick Install) Engine Database.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DatabaseEngineInfo } from '../../lib/types';
  import '../../styles/pages/databases/quick-install-modal.css';

  interface Props {
    open: boolean;
    installing: boolean;
    activeEngine: 'mysql' | 'postgresql' | 'mongodb' | 'redis';
    currentEngine: DatabaseEngineInfo;
    installVersion: string;
    installMode: string;
    getEngineIcon: (engine: string) => string;
    onClose: () => void;
    onSubmit: () => void;
  }

  let {
    open,
    installing,
    activeEngine,
    currentEngine,
    installVersion = $bindable(),
    installMode = $bindable(),
    getEngineIcon,
    onClose,
    onSubmit
  }: Props = $props();
</script>

<Modal
  {open}
  size="md"
  onclose={() => { if (!installing) onClose(); }}
>
  {#snippet header()}
    <div class="modal-title-with-icon">
      <i class="{getEngineIcon(activeEngine)} engine-install-icon" aria-hidden="true"></i>
      <div>
        <h3 class="modal__title">Instalasi {currentEngine.display_name}</h3>
        <p class="modal__subtitle">Pasang paket engine database ke server</p>
      </div>
    </div>
  {/snippet}

  <div class="pg-databases-quick-install-modal">
    <div class="form-group">
      <label for="install-ver" class="form-label">Pilih Versi</label>
      <select id="install-ver" class="form-select" bind:value={installVersion} disabled={installing}>
        {#if currentEngine.versions && currentEngine.versions.length > 0}
          {#each currentEngine.versions as v}
            <option value={v.version}>{v.label}</option>
          {/each}
        {:else}
          {#each currentEngine.supported_versions as ver}
            <option value={ver}>{ver}</option>
          {/each}
        {/if}
      </select>
    </div>

    <div class="form-group">
      <label for="install-mode" class="form-label">Metode Instalasi</label>
      <select id="install-mode" class="form-select" bind:value={installMode} disabled={installing}>
        <option value="native">Native Package (APT / Systemd Linux)</option>
        <option value="docker">Docker Container (Terisolasi & Mandiri)</option>
      </select>
    </div>

    <div class="plugin-desc-box">
      <p class="plugin-desc-title">Petunjuk Instalasi:</p>
      <ul class="plugin-desc-list">
        <li>Jika paket ini telah ada, berkas biner lama akan diperbarui secara aman.</li>
        <li>Port default ({currentEngine.port}) akan dikonfigurasi pada localhost (127.0.0.1).</li>
        <li>Proses instalasi membutuhkan waktu 1-3 menit, silakan tunggu hingga selesai.</li>
      </ul>
    </div>
  </div>

  {#snippet footer()}
    <button
      class="btn btn-outline"
      disabled={installing}
      onclick={onClose}
    >
      Batal
    </button>
    <button class="btn btn-primary" disabled={installing} onclick={onSubmit}>
      {#if installing}
        <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
        <span>Memulai...</span>
      {:else}
        <i class="ri-play-fill" aria-hidden="true"></i>
        <span>Mulai Instalasi (Install Now)</span>
      {/if}
    </button>
  {/snippet}
</Modal>

