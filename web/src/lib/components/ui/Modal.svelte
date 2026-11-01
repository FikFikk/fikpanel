<script lang="ts">
  import { tick, type Snippet } from 'svelte';

  export type ModalSize = 'sm' | 'md' | 'lg' | 'xl';

  interface Props {
    open: boolean;
    title?: string;
    subtitle?: string;
    size?: ModalSize;
    onclose: () => void;
    header?: Snippet;
    children?: Snippet;
    footer?: Snippet;
  }

  let {
    open,
    title = '',
    subtitle = '',
    size = 'md',
    onclose,
    header,
    children,
    footer,
  }: Props = $props();

  const titleId = `fp-modal-title-${Math.random().toString(36).slice(2, 9)}`;

  let panelEl = $state<HTMLDivElement | null>(null);
  let previousFocus: HTMLElement | null = null;

  // Kunci scroll body, simpan fokus pemicu, dan pindahkan fokus ke dalam modal saat dibuka
  $effect(() => {
    if (!open) return;

    previousFocus = document.activeElement as HTMLElement | null;
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';

    tick().then(() => {
      if (!panelEl) return;
      const target =
        panelEl.querySelector<HTMLElement>('[data-autofocus]') ??
        panelEl.querySelector<HTMLElement>(
          'input:not(:disabled), select:not(:disabled), textarea:not(:disabled), button:not(:disabled):not(.modal__close)'
        ) ??
        panelEl.querySelector<HTMLElement>('button:not(:disabled)') ??
        panelEl;
      target.focus();
    });

    return () => {
      document.body.style.overflow = prevOverflow;
      previousFocus?.focus?.();
      previousFocus = null;
    };
  });

  function handleKeydown(e: KeyboardEvent) {
    if (!open) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      onclose();
      return;
    }
    if (e.key !== 'Tab' || !panelEl) return;
    const focusables = panelEl.querySelectorAll<HTMLElement>(
      'a[href], button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])'
    );
    if (!focusables.length) {
      e.preventDefault();
      return;
    }
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      onclose();
    }
  }
</script>

{#if open}
  <div
    class="modal modal--{size}"
    role="presentation"
    onclick={handleBackdropClick}
    onkeydown={handleKeydown}
  >
    <div
      bind:this={panelEl}
      class="modal__panel modal--{size}"
      role="dialog"
      aria-modal="true"
      aria-labelledby={title ? titleId : undefined}
      tabindex="-1"
    >
      {#if title || header}
        <header class="modal__head">
          {#if header}
            {@render header()}
          {:else}
            <div class="modal__head-text">
              <h2 class="modal__title" id={titleId}>{title}</h2>
              {#if subtitle}
                <p class="modal__sub">{subtitle}</p>
              {/if}
            </div>
          {/if}
          <button
            type="button"
            class="modal__close"
            onclick={onclose}
            aria-label="Tutup dialog"
            title="Tutup (Esc)"
          >
            <i class="ri-close-line" aria-hidden="true"></i>
          </button>
        </header>
      {/if}

      <div class="modal__body">
        {@render children?.()}
      </div>

      {#if footer}
        <footer class="modal__foot">
          {@render footer()}
        </footer>
      {/if}
    </div>
  </div>
{/if}
