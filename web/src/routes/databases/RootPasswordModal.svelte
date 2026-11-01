<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DatabaseEngineInfo } from '../../lib/types';
  import '../../styles/pages/databases/root-password-modal.css';

  // Antarmuka properti untuk modal password root database
  interface Props {
    open: boolean;
    currentEngine: DatabaseEngineInfo;
    rootPassword: string;
    showRootPass: boolean;
    onCopyText: (text: string, label: string) => void;
    onSave: () => void;
    onClose: () => void;
  }

  let {
    open,
    currentEngine,
    rootPassword = $bindable(),
    showRootPass = $bindable(),
    onCopyText,
    onSave,
    onClose,
  }: Props = $props();
</script>

<!-- Modal Password Root -->
<Modal
  {open}
  title="Password Root / Administrator"
  size="md"
  onclose={onClose}
>
  <div class="pg-databases-root-password-modal">
    <p class="modal__subtitle">
      Kredensial root administrator digunakan untuk menghubungkan panel dengan layanan engine {currentEngine.display_name}.
    </p>

    <div class="form-group">
      <label for="root-pass" class="form-label">Password Root Saat Ini</label>
      <div class="input-with-button">
        <input
          id="root-pass"
          type={showRootPass ? 'text' : 'password'}
          class="form-input"
          bind:value={rootPassword}
        />
        <button
          type="button"
          class="btn btn-outline"
          aria-label={showRootPass ? 'Sembunyikan password' : 'Lihat password'}
          onclick={() => (showRootPass = !showRootPass)}
        >
          <i class={showRootPass ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
        </button>
        <button
          type="button"
          class="btn btn-outline"
          onclick={() => onCopyText(rootPassword, 'Password root')}
        >
          Salin
        </button>
      </div>
    </div>
  </div>

  {#snippet footer()}
    <button class="btn btn-outline" onclick={onClose}>
      Tutup
    </button>
    <button class="btn btn-primary" onclick={onSave}>
      Simpan Perubahan
    </button>
  {/snippet}
</Modal>
