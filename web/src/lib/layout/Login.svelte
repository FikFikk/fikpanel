<script lang="ts">
  import { apiFetch } from '../api';

  interface Props {
    onSuccess: (username: string) => void;
  }

  let { onSuccess }: Props = $props();

  let username = $state('root');
  let password = $state('');
  let showPassword = $state(false);
  let capsLock = $state(false);
  let error = $state('');
  let submitting = $state(false);

  // Hint akun default hanya tampil saat pengembangan, tidak pernah di build produksi
  const isDev = import.meta.env.DEV;

  async function submit(e: Event) {
    e.preventDefault();
    if (submitting) return;
    error = '';
    submitting = true;
    try {
      const res = await apiFetch<{ ok: boolean; user: { username: string } }>('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username: username.trim(), password }),
      });
      if (res.ok) {
        password = '';
        onSuccess(res.user.username);
      }
    } catch (err) {
      error = (err as Error).message || 'Kredensial login tidak valid';
      password = '';
    } finally {
      submitting = false;
    }
  }

  function onKey(e: KeyboardEvent) {
    capsLock = e.getModifierState?.('CapsLock') ?? false;
  }
</script>

<main class="login">
  <div class="login__glow" aria-hidden="true"></div>
  <section class="login__card" aria-labelledby="login-title">
    <header class="login__brand">
      <svg viewBox="0 0 40 40" width="48" height="48" aria-hidden="true">
        <rect width="40" height="40" rx="10" fill="var(--accent)" />
        <path d="M12 14h16M12 20h12M12 26h16" stroke="var(--text-on-accent)" stroke-width="2.5" stroke-linecap="round" />
      </svg>
      <h1 id="login-title">fikpanel</h1>
      <p>Masuk untuk mengelola server Anda</p>
    </header>

    {#if error}
      <div class="alert alert--danger" role="alert">
        <i class="ri-error-warning-line" aria-hidden="true"></i>
        <span>{error}</span>
      </div>
    {/if}

    <form class="stack" onsubmit={submit} novalidate>
      <label class="field">
        <span class="field__label">Nama pengguna</span>
        <input class="input" id="login-user" type="text" bind:value={username} autocomplete="username" autocapitalize="off" spellcheck="false" required />
      </label>

      <div class="field">
        <label class="field__label" for="login-pass">Kata sandi</label>
        <div class="login__pw">
          <input
            class="input"
            id="login-pass"
            type={showPassword ? 'text' : 'password'}
            bind:value={password}
            autocomplete="current-password"
            onkeydown={onKey}
            onkeyup={onKey}
            aria-describedby={capsLock ? 'login-caps' : undefined}
          />
          <button
            type="button"
            class="btn btn--ghost btn--icon btn--sm login__pw-toggle"
            onclick={() => (showPassword = !showPassword)}
            aria-label={showPassword ? 'Sembunyikan kata sandi' : 'Tampilkan kata sandi'}
            aria-pressed={showPassword}
          >
            <i class={showPassword ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
          </button>
        </div>
        {#if capsLock}
          <span class="field__hint login__caps" id="login-caps"><i class="ri-arrow-up-line" aria-hidden="true"></i> Caps Lock aktif</span>
        {/if}
      </div>

      <button type="submit" class="btn btn--primary btn--lg btn--block" disabled={submitting || !username.trim()} id="login-submit">
        {#if submitting}<span class="spinner" aria-hidden="true"></span> Memverifikasi…{:else}Masuk ke panel{/if}
      </button>

      {#if isDev}
        <p class="login__hint">Mode dev: akun <code>root</code> dengan kata sandi kosong</p>
      {/if}
    </form>
  </section>
</main>
