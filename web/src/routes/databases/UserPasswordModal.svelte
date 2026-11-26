<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DatabaseItem } from '../../lib/types';
  import '../../styles/pages/databases/user-password-modal.css';

  // Antarmuka properti komponen modal kredensial & password pengguna database
  interface Props {
    open: boolean;
    db: DatabaseItem | null;
    username: string;
    newPassword: string;
    vaultOnly: boolean;
    saving: boolean;
    onGeneratePassword: () => void;
    onClose: () => void;
    onSubmit: () => void;
  }

  let {
    open,
    db,
    username = $bindable(),
    newPassword = $bindable(),
    vaultOnly = $bindable(),
    saving,
    onGeneratePassword,
    onClose,
    onSubmit,
  }: Props = $props();

  let showPass = $state(false);
</script>

<Modal
  {open}
  title="Kredensial & Password Database"
  size="md"
  onclose={() => { if (!saving) onClose(); }}
>
  <div class="pg-databases-user-password-modal">
    <p class="modal__subtitle">
      Kelola username dan kata sandi untuk database <strong>{db?.name}</strong>.
    </p>

    <div class="form-group">
      <label for="db-user-name" class="form-label">Username Database</label>
      <input
        id="db-user-name"
        type="text"
        class="form-input"
        placeholder="Misal: qr-fik atau root"
        bind:value={username}
        disabled={saving}
      />
    </div>

    <div class="form-group">
      <label for="db-user-pass" class="form-label">
        {vaultOnly ? 'Kata Sandi Eksisting (untuk disimpan di Vault Panel)' : 'Kata Sandi Baru'}
      </label>
      <div class="input-with-button">
        <input
          id="db-user-pass"
          type={showPass ? 'text' : 'password'}
          class="form-input"
          placeholder="Masukkan kata sandi..."
          bind:value={newPassword}
          disabled={saving}
        />
        <button
          type="button"
          class="btn btn-outline"
          aria-label={showPass ? 'Sembunyikan password' : 'Lihat password'}
          onclick={() => (showPass = !showPass)}
        >
          <i class={showPass ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
        </button>
        {#if !vaultOnly}
          <button
            type="button"
            class="btn btn-outline"
            onclick={onGeneratePassword}
            title="Buat kata sandi acak kuat"
            disabled={saving}
          >
            Acak
          </button>
        {/if}
      </div>
    </div>

    <label class="vault-option-box">
      <input type="checkbox" bind:checked={vaultOnly} disabled={saving} />
      <span>
        <strong>Hanya catat di vault panel (jangan ubah password di server database)</strong>
        Gunakan opsi ini bila aplikasi produksi Anda sudah berjalan memakai password eksisting agar koneksi aplikasi tidak terputus.
      </span>
    </label>
  </div>

  {#snippet footer()}
    <button type="button" class="btn btn-outline" onclick={onClose} disabled={saving}>
      Batal
    </button>
    <button type="button" class="btn btn-primary" onclick={onSubmit} disabled={saving || !newPassword.trim()}>
      {#if saving}
        <span class="spinner" aria-hidden="true"></span>
        <span>Menyimpan...</span>
      {:else}
        <span>{vaultOnly ? 'Simpan ke Vault Panel' : 'Ubah di Server & Panel'}</span>
      {/if}
    </button>
  {/snippet}
</Modal>
