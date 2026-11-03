<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DatabaseItem } from '../../lib/types';
  import '../../styles/pages/databases/user-password-modal.css';

  // Antarmuka properti komponen modal ubah password database
  interface Props {
    open: boolean;
    db: DatabaseItem | null;
    newPassword: string;
    saving: boolean;
    onGeneratePassword: () => void;
    onClose: () => void;
    onSubmit: () => void;
  }

  let {
    open,
    db,
    newPassword = $bindable(),
    saving,
    onGeneratePassword,
    onClose,
    onSubmit,
  }: Props = $props();

  let showPass = $state(false);
</script>

<Modal
  {open}
  title="Ubah Password Database"
  size="md"
  onclose={onClose}
>
  <div class="pg-databases-user-password-modal">
    <p class="modal__subtitle">
      Perbarui kata sandi untuk database <strong>{db?.name}</strong> (Pengguna: <code>{db?.username || db?.name}</code>).
    </p>

    <div class="form-group">
      <label for="db-user-pass" class="form-label">Kata Sandi Baru</label>
      <div class="input-with-button">
        <input
          id="db-user-pass"
          type={showPass ? 'text' : 'password'}
          class="form-input"
          placeholder="Masukkan kata sandi baru..."
          bind:value={newPassword}
        />
        <button
          type="button"
          class="btn btn-outline"
          aria-label={showPass ? 'Sembunyikan password' : 'Lihat password'}
          onclick={() => (showPass = !showPass)}
        >
          <i class={showPass ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
        </button>
        <button
          type="button"
          class="btn btn-outline"
          onclick={onGeneratePassword}
          title="Buat kata sandi acak kuat"
        >
          Acak
        </button>
      </div>
    </div>
  </div>

  {#snippet footer()}
    <button class="btn btn-outline" onclick={onClose} disabled={saving}>
      Batal
    </button>
    <button class="btn btn-primary" onclick={onSubmit} disabled={saving || !newPassword.trim()}>
      {#if saving}
        <span class="spinner" aria-hidden="true"></span>
        <span>Menyimpan...</span>
      {:else}
        <span>Simpan Password</span>
      {/if}
    </button>
  {/snippet}
</Modal>
