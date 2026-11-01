<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { ManagerTab } from '../types';
  import '../../styles/pages/components/manager-modal.css';

  interface Props {
    title: string;
    subtitle?: string;
    logo?: Snippet;
    tabs: ManagerTab[];
    active: string;
    badge?: Snippet;
    onClose: () => void;
    children: Snippet<[string]>;
  }

  let { title, subtitle = '', logo, tabs, active = $bindable(), badge, onClose, children }: Props = $props();

  // Tutup modal dengan tombol Escape dari mana pun di halaman
  function onWindowKey(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
  }
</script>

<svelte:window onkeydown={onWindowKey} />

<div
  class="cmp-manager-modal mm-backdrop"
  role="presentation"
  onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}
>
  <div class="mm-card" role="dialog" aria-modal="true" aria-label={title}>
    <header class="mm-head">
      <div class="mm-title">
        {#if logo}<span class="mm-logo">{@render logo()}</span>{/if}
        <div>
          <h3>{title}</h3>
          {#if subtitle}<p>{subtitle}</p>{/if}
        </div>
        {#if badge}{@render badge()}{/if}
      </div>
      <button class="mm-close" onclick={onClose} aria-label="Tutup" id="mm-close-btn">
        <i class="ri-close-line"></i>
      </button>
    </header>

    <div class="mm-body">
      <nav class="mm-nav" aria-label="Menu {title}">
        {#each tabs as t (t.id)}
          <button
            id="mm-tab-{t.id}"
            class="mm-nav-item"
            class:active={active === t.id}
            onclick={() => (active = t.id)}
          >
            <i class={t.icon}></i>
            <span>{t.label}</span>
          </button>
        {/each}
      </nav>

      <section class="mm-panel">
        {#key active}
          <div class="mm-panel-inner">
            {@render children(active)}
          </div>
        {/key}
      </section>
    </div>
  </div>
</div>
