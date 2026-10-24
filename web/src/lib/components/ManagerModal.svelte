<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { ManagerTab } from '../types';

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
  class="mm-backdrop"
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

<style>
  .mm-backdrop {
    position: fixed;
    inset: 0;
    z-index: 1000;
    display: grid;
    place-items: center;
    padding: 1.5rem;
    background: color-mix(in srgb, var(--bg-app) 72%, transparent);
    backdrop-filter: blur(6px);
    animation: mm-fade 0.18s ease-out;
  }
  .mm-card {
    width: min(1000px, 100%);
    height: min(720px, calc(100vh - 3rem));
    display: flex;
    flex-direction: column;
    background: linear-gradient(180deg, var(--bg-raised) 0%, var(--bg-surface) 100%);
    border: 1px solid var(--border);
    border-radius: 14px;
    box-shadow: 0 30px 80px -20px rgba(0, 0, 0, 0.7), 0 0 0 1px color-mix(in srgb, var(--accent) 4%, transparent);
    overflow: hidden;
    animation: mm-pop 0.22s cubic-bezier(0.2, 0.9, 0.3, 1.2);
  }
  .mm-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.9rem 1.2rem;
    border-bottom: 1px solid var(--border);
  }
  .mm-title {
    display: flex;
    align-items: center;
    gap: 0.7rem;
  }
  .mm-title h3 {
    font-size: 1rem;
    font-weight: 700;
    color: var(--text);
    margin: 0;
  }
  .mm-title p {
    font-size: 0.7rem;
    color: var(--text-subtle);
    margin: 0.1rem 0 0;
    font-family: 'Fira Code', monospace;
  }
  .mm-logo {
    display: grid;
    place-items: center;
  }
  .mm-close {
    width: 32px;
    height: 32px;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--bg-raised);
    color: var(--text-muted);
    font-size: 1.1rem;
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .mm-close:hover {
    color: var(--danger-strong);
    border-color: color-mix(in srgb, var(--danger-strong) 40%, transparent);
    transform: rotate(90deg);
  }
  .mm-body {
    flex: 1;
    display: flex;
    min-height: 0;
  }
  .mm-nav {
    width: 200px;
    flex-shrink: 0;
    padding: 0.6rem;
    border-right: 1px solid var(--border);
    background: color-mix(in srgb, var(--bg-inset) 45%, transparent);
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    overflow-y: auto;
  }
  .mm-nav-item {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.6rem 0.75rem;
    border: none;
    border-radius: 8px;
    background: transparent;
    color: var(--text-muted);
    font-size: 0.82rem;
    font-weight: 500;
    text-align: left;
    cursor: pointer;
    position: relative;
    transition: background 0.15s ease, color 0.15s ease;
  }
  .mm-nav-item i {
    font-size: 1rem;
  }
  .mm-nav-item:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .mm-nav-item.active {
    background: linear-gradient(90deg, color-mix(in srgb, var(--accent) 14%, transparent), color-mix(in srgb, var(--accent) 2%, transparent));
    color: var(--accent-strong);
  }
  .mm-nav-item.active::before {
    content: '';
    position: absolute;
    left: 0;
    top: 22%;
    bottom: 22%;
    width: 3px;
    border-radius: 2px;
    background: var(--accent);
    box-shadow: 0 0 10px var(--accent);
  }
  .mm-panel {
    flex: 1;
    min-width: 0;
    overflow-y: auto;
    padding: 1.3rem 1.5rem;
  }
  .mm-panel-inner {
    height: 100%;
    animation: mm-slide 0.2s ease-out;
  }
  @keyframes mm-fade {
    from { opacity: 0; }
  }
  @keyframes mm-pop {
    from { opacity: 0; transform: translateY(10px) scale(0.98); }
  }
  @keyframes mm-slide {
    from { opacity: 0; transform: translateX(6px); }
  }
  @media (max-width: 720px) {
    .mm-body { flex-direction: column; }
    .mm-nav {
      width: 100%;
      flex-direction: row;
      overflow-x: auto;
      border-right: none;
      border-bottom: 1px solid var(--border);
    }
    .mm-nav-item { white-space: nowrap; }
    .mm-nav-item.active::before { display: none; }
  }
</style>
