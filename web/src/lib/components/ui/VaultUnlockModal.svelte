<script lang="ts">
  /**
   * Modal Verifikasi Step-Up Keamanan Vault: mendukung Passkey/Biometrik FIDO2 (Windows Hello, Touch ID, QR HP)
   * serta verifikasi kata sandi administrator FikPanel sebelum membuka atau menyalin rahasia (.env / password).
   */
  import Modal from './Modal.svelte';
  import { apiFetch } from '../../api';
  import { getWebAuthnEnvironment, unlockVaultWithPasskeyFlow } from '../../utils/webauthn';
  import '../../../styles/pages/databases/vault-unlock-modal.css';

  interface Props {
    open: boolean;
    reason?: string;
    hasPasskeys?: boolean;
    username?: string;
    onUnlocked: (expiresAt: number) => void;
    onClose: () => void;
  }

  let {
    open,
    reason = 'Verifikasi identitas Anda untuk membuka atau menyalin kredensial rahasia.',
    hasPasskeys = false,
    username = '',
    onUnlocked,
    onClose,
  }: Props = $props();

  let adminPassword = $state('');
  let showPass = $state(false);
  let verifyingPasskey = $state(false);
  let verifyingPassword = $state(false);
  let errorMsg = $state('');

  let webAuthnEnv = $derived(getWebAuthnEnvironment());
  let canUsePasskey = $derived(hasPasskeys && webAuthnEnv.supported && webAuthnEnv.validOrigin);

  // Reset form setiap kali modal dibuka
  $effect(() => {
    if (open) {
      adminPassword = '';
      showPass = false;
      errorMsg = '';
    }
  });

  // Buka kunci Vault menggunakan Biometrik / Passkey (FIDO2 UserVerification Required)
  async function handleUnlockPasskey() {
    verifyingPasskey = true;
    errorMsg = '';
    try {
      const res = await unlockVaultWithPasskeyFlow();
      if (res.unlocked) {
        onUnlocked(res.expires_at);
      }
    } catch (e: any) {
      errorMsg = e.message || 'Verifikasi Biometrik / Passkey gagal';
    } finally {
      verifyingPasskey = false;
    }
  }

  // Buka kunci Vault menggunakan kata sandi login administrator FikPanel
  async function handleUnlockPassword(e?: Event) {
    e?.preventDefault();
    if (!adminPassword.trim()) {
      errorMsg = 'Masukkan kata sandi administrator panel';
      return;
    }
    verifyingPassword = true;
    errorMsg = '';
    try {
      const res = await apiFetch<{ ok: boolean; unlocked: boolean; expires_at: number }>(
        '/api/auth/vault/unlock/password',
        {
          method: 'POST',
          body: JSON.stringify({ password: adminPassword }),
        }
      );
      if (res.unlocked) {
        adminPassword = '';
        onUnlocked(res.expires_at);
      }
    } catch (err: any) {
      errorMsg = err.message || 'Kata sandi administrator tidak cocok';
    } finally {
      verifyingPassword = false;
    }
  }
</script>

<Modal
  {open}
  title="Verifikasi Keamanan Vault"
  size="sm"
  onclose={() => {
    if (!verifyingPasskey && !verifyingPassword) onClose();
  }}
>
  <div class="pg-vault-unlock-modal">
    <div class="vault-banner">
      <i class="ri-shield-keyhole-line vault-shield-icon" aria-hidden="true"></i>
      <div>
        <h4 class="vault-banner-title">Akses Kredensial Terkunci</h4>
        <p class="vault-banner-desc">{reason}</p>
      </div>
    </div>

    {#if errorMsg}
      <div class="vault-error" role="alert">
        <i class="ri-error-warning-line" aria-hidden="true"></i>
        <span>{errorMsg}</span>
      </div>
    {/if}

    {#if canUsePasskey}
      <button
        type="button"
        class="btn btn-primary vault-passkey-btn"
        disabled={verifyingPasskey || verifyingPassword}
        onclick={handleUnlockPasskey}
      >
        <i class={verifyingPasskey ? 'ri-loader-4-line spin' : 'ri-fingerprint-line'} aria-hidden="true"></i>
        <span>{verifyingPasskey ? 'Menunggu Verifikasi Biometrik...' : 'Buka dengan Biometrik / Passkey'}</span>
      </button>

      <div class="vault-divider">atau gunakan kata sandi admin</div>
    {/if}

    <form id="vault-unlock-form" onsubmit={handleUnlockPassword}>
      <div class="form-group">
        <label for="vault-admin-pass" class="form-label">
          Kata Sandi Panel {username ? `(${username})` : ''}
        </label>
        <div class="vault-pass-row">
          <input
            id="vault-admin-pass"
            type={showPass ? 'text' : 'password'}
            class="form-input"
            placeholder="Masukkan kata sandi login FikPanel..."
            autocomplete="current-password"
            bind:value={adminPassword}
            disabled={verifyingPassword || verifyingPasskey}
          />
          <button
            type="button"
            class="btn btn-outline"
            aria-label={showPass ? 'Sembunyikan kata sandi' : 'Lihat kata sandi'}
            onclick={() => (showPass = !showPass)}
          >
            <i class={showPass ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
          </button>
        </div>
      </div>
    </form>

    <p class="vault-foot-hint">
      <i class="ri-time-line" aria-hidden="true"></i>
      <span>Vault akan terbuka selama 5 menit untuk sesi ini dan dapat dikunci kembali kapan saja.</span>
    </p>
  </div>

  {#snippet footer()}
    <button
      type="button"
      class="btn btn-outline"
      disabled={verifyingPasskey || verifyingPassword}
      onclick={onClose}
    >
      Batal
    </button>
    <button
      type="submit"
      form="vault-unlock-form"
      class="btn btn-primary"
      disabled={verifyingPasskey || verifyingPassword || !adminPassword.trim()}
    >
      <i class={verifyingPassword ? 'ri-loader-4-line spin' : 'ri-lock-unlock-line'} aria-hidden="true"></i>
      <span>{verifyingPassword ? 'Memverifikasi...' : 'Buka Kunci Vault'}</span>
    </button>
  {/snippet}
</Modal>
