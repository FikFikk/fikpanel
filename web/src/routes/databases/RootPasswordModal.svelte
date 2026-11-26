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
    saving?: boolean;
    onCopyText: (text: string, label: string) => void;
    onGenerate: () => void;
    onSave: () => void;
    onClose: () => void;
  }

  let {
    open,
    currentEngine,
    rootPassword = $bindable(),
    showRootPass = $bindable(),
    saving = false,
    onCopyText,
    onGenerate,
    onSave,
    onClose,
  }: Props = $props();
</script>

<!-- Modal Password Root / Administrator -->
<Modal
  {open}
  title={currentEngine.name === 'redis' ? 'Password Auth Redis (requirepass)' : 'Password Root / Administrator'}
  size="md"
  onclose={() => { if (!saving) onClose(); }}
>
  <div class="pg-databases-root-password-modal">
    <p class="modal__subtitle">
      Perubahan ini langsung diterapkan ke daemon <strong>{currentEngine.display_name}</strong> di server host (minimal 6 karakter). Pastikan Anda menyalin sandi baru sebelum menutup modal.
    </p>

    <div class="form-group">
      <label for="root-pass" class="form-label">Password Baru Administrator</label>
      <div class="input-with-button">
        <input
          id="root-pass"
          type={showRootPass ? 'text' : 'password'}
          class="form-input"
          placeholder="Masukkan kata sandi baru (min. 6 karakter)"
          bind:value={rootPassword}
          disabled={saving}
        />
        <button
          type="button"
          class="btn btn-outline"
          aria-label="Buat kata sandi acak"
          title="Buat kata sandi acak kuat"
          onclick={onGenerate}
          disabled={saving}
        >
          <i class="ri-refresh-line" aria-hidden="true"></i>
        </button>
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
    <button type="button" class="btn btn-outline" disabled={saving} onclick={onClose}>
      Tutup
    </button>
    <button type="button" class="btn btn-primary" disabled={saving || rootPassword.trim().length < 6} onclick={onSave}>
      {saving ? 'Menerapkan...' : 'Terapkan ke Server'}
    </button>
  {/snippet}
</Modal>
