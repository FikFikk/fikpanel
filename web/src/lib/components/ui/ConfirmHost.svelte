<script lang="ts">
  import { tick } from 'svelte';
  import { confirmState, resolveConfirm } from '../../stores/confirm.svelte';

  let typed = $state('');
  let dialogEl = $state<HTMLDivElement | null>(null);
  let restoreFocus: HTMLElement | null = null;

  let canConfirm = $derived(!confirmState.requireText || typed === confirmState.requireText);

  // Simpan fokus sebelumnya, pindahkan fokus ke dialog, dan kembalikan saat ditutup
  $effect(() => {
    if (confirmState.open) {
      restoreFocus = document.activeElement as HTMLElement | null;
      typed = '';
      tick().then(() => {
        const first = dialogEl?.querySelector<HTMLElement>(confirmState.requireText || confirmState.prompt ? 'input' : '[data-autofocus]');
        first?.focus();
      });
    } else if (restoreFocus) {
      restoreFocus.focus?.();
      restoreFocus = null;
    }
  });

  // Jebak fokus Tab di dalam dialog agar tidak lari ke halaman di belakang
  function onKeydown(e: KeyboardEvent) {
    if (!confirmState.open) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      resolveConfirm(false);
      return;
    }
    if (e.key !== 'Tab' || !dialogEl) return;
    const items = dialogEl.querySelectorAll<HTMLElement>('button:not(:disabled), input');
    if (!items.length) return;
    const first = items[0];
    const last = items[items.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }

  function submit(e: Event) {
    e.preventDefault();
    if (canConfirm) resolveConfirm(true);
  }
</script>

<svelte:window onkeydowncapture={onKeydown} />

{#if confirmState.open}
  <div class="confirm cd-backdrop" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) resolveConfirm(false); }}>
    <div
      bind:this={dialogEl}
      class="cd-card"
      role="alertdialog"
      aria-modal="true"
      aria-labelledby="cd-title"
      aria-describedby={confirmState.message ? 'cd-msg' : undefined}
    >
      <form onsubmit={submit}>
        <div class="cd-body">
          <span class="cd-icon" class:danger={confirmState.tone === 'danger'} aria-hidden="true">
            <i class={confirmState.tone === 'danger' ? 'ri-error-warning-line' : 'ri-question-line'}></i>
          </span>
          <div class="cd-text">
            <h2 id="cd-title">{confirmState.title}</h2>
            {#if confirmState.message}<p id="cd-msg">{confirmState.message}</p>{/if}
            {#if confirmState.requireText}
              <label class="field">
                <span class="field__label">Ketik <code class="mono">{confirmState.requireText}</code> untuk melanjutkan</span>
                <input class="input input--mono" bind:value={typed} autocomplete="off" spellcheck="false" />
              </label>
            {/if}
            {#if confirmState.prompt}
              <label class="field">
                <span class="field__label">{confirmState.prompt.label}</span>
                <input
                  type={confirmState.prompt.inputType ?? 'text'}
                  min={confirmState.prompt.min}
                  max={confirmState.prompt.max}
                  placeholder={confirmState.prompt.placeholder}
                  class="input input--mono"
                  bind:value={confirmState.prompt.value}
                  autocomplete="off"
                  spellcheck="false"
                />
              </label>
            {/if}
          </div>
        </div>
        <div class="cd-foot">
          <button type="button" class="btn btn--ghost" onclick={() => resolveConfirm(false)} data-autofocus={confirmState.tone === 'danger' ? '' : undefined}>
            {confirmState.cancelText}
          </button>
          <button
            type="submit"
            class="btn {confirmState.tone === 'danger' ? 'btn--danger' : 'btn--primary'}"
            disabled={!canConfirm}
            data-autofocus={confirmState.tone === 'danger' ? undefined : ''}
          >
            {confirmState.confirmText}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
