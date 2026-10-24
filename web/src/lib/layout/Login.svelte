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
        <div class="pw">
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
            class="btn btn--ghost btn--icon btn--sm pw__toggle"
            onclick={() => (showPassword = !showPassword)}
            aria-label={showPassword ? 'Sembunyikan kata sandi' : 'Tampilkan kata sandi'}
            aria-pressed={showPassword}
          >
            <i class={showPassword ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
          </button>
        </div>
        {#if capsLock}
          <span class="field__hint caps" id="login-caps"><i class="ri-arrow-up-line" aria-hidden="true"></i> Caps Lock aktif</span>
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

<style>
  .login {
    position: relative;
    display: grid;
    place-items: center;
    min-height: 100vh;
    padding: var(--space-6);
    overflow: hidden;
  }
  .login__glow {
    position: absolute;
    width: 520px;
    height: 520px;
    border-radius: 50%;
    background: radial-gradient(circle, color-mix(in srgb, var(--accent) 14%, transparent), transparent 65%);
    filter: blur(20px);
    pointer-events: none;
  }
  .login__card {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    width: min(380px, 100%);
    padding: var(--space-8);
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    box-shadow: var(--shadow-lg);
    animation: fp-pop var(--duration-slow) var(--ease-spring);
  }
  .login__brand {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    text-align: center;
  }
  .login__brand h1 {
    font-size: var(--text-xl);
    margin-top: var(--space-1);
  }
  .login__brand p {
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  .pw {
    position: relative;
  }
  .pw .input {
    padding-right: calc(var(--control-sm) + var(--space-3));
  }
  .pw__toggle {
    position: absolute;
    top: 50%;
    right: var(--space-1);
    transform: translateY(-50%);
  }
  .caps {
    color: var(--warning-strong);
  }
  .login__hint {
    text-align: center;
    font-size: var(--text-xs);
    color: var(--text-subtle);
  }
  .login__hint code {
    padding: 0 var(--space-1);
    border-radius: 4px;
    background: var(--tint-accent);
    color: var(--accent-strong);
  }
</style>
