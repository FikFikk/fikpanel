<script lang="ts">
  import { toasts, dismissToast, type ToastType } from '../../stores/toast.svelte';

  const ICON: Record<ToastType, string> = {
    success: 'ri-checkbox-circle-fill',
    info: 'ri-information-fill',
    warning: 'ri-error-warning-fill',
    error: 'ri-close-circle-fill',
  };
</script>

<!-- Dua region terpisah: error dibacakan segera (assertive), sisanya sopan (polite) -->
<div class="toast-stack" aria-live="polite" aria-relevant="additions">
  {#each toasts.items as t (t.id)}
    <div class="toast toast--{t.type}" role={t.type === 'error' ? 'alert' : 'status'}>
      <i class={ICON[t.type]} aria-hidden="true"></i>
      <span class="toast__msg">{t.message}</span>
      <button class="toast__close" onclick={() => dismissToast(t.id)} aria-label="Tutup notifikasi">
        <i class="ri-close-line" aria-hidden="true"></i>
      </button>
    </div>
  {/each}
</div>

<style>
  .toast-stack {
    position: fixed;
    right: var(--space-6);
    bottom: var(--space-6);
    z-index: var(--z-toast);
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: var(--space-2);
    pointer-events: none;
  }
  .toast {
    --tone: var(--info);
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    max-width: min(420px, calc(100vw - 2rem));
    padding: var(--space-3) var(--space-3) var(--space-3) var(--space-4);
    border-radius: var(--radius-lg);
    border: 1px solid color-mix(in srgb, var(--tone) 35%, var(--border));
    border-left: 3px solid var(--tone);
    background: var(--bg-raised);
    box-shadow: var(--shadow-md);
    color: var(--text);
    font-size: var(--text-sm);
    pointer-events: auto;
    animation: fp-pop var(--duration-base) var(--ease-spring);
  }
  .toast > i {
    margin-top: 1px;
    font-size: 1.1rem;
    color: var(--tone);
  }
  .toast__msg {
    flex: 1;
    line-height: var(--leading-normal);
    word-break: break-word;
  }
  .toast__close {
    display: grid;
    place-items: center;
    width: 22px;
    height: 22px;
    border-radius: var(--radius-sm);
    color: var(--text-subtle);
    transition: color var(--duration-fast) ease, background var(--duration-fast) ease;
  }
  .toast__close:hover {
    color: var(--text);
    background: var(--bg-hover);
  }
  .toast--success { --tone: var(--accent); }
  .toast--warning { --tone: var(--warning); }
  .toast--error   { --tone: var(--danger); }
  @media (max-width: 640px) {
    .toast-stack {
      left: var(--space-3);
      right: var(--space-3);
      bottom: var(--space-3);
      align-items: stretch;
    }
    .toast { max-width: none; }
  }
</style>
