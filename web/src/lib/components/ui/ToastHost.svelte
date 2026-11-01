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
<div class="toast-host toast-stack" aria-live="polite" aria-relevant="additions">
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
