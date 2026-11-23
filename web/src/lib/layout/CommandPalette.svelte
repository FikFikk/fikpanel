<script lang="ts">
  import { NAV_GROUPS, NAV_ITEMS, type NavItem } from './nav';
  import { t } from '../stores/i18n.svelte';

  interface Props {
    open: boolean;
    current: string;
    onClose: () => void;
    onSelect?: (id: string) => void;
  }

  let { open, current, onClose, onSelect }: Props = $props();

  let query = $state('');
  let activeIndex = $state(0);
  let inputEl = $state<HTMLInputElement | null>(null);

  function labelForItem(item: NavItem): string {
    return t(`nav.item.${item.id}` as any);
  }
  function labelForGroup(groupId: string): string {
    return t(`nav.group.${groupId}` as any);
  }

  // Filter fuzzy sederhana berdasarkan label terjemahan, id rute, atau nama grup
  function matchQuery(item: NavItem, q: string): boolean {
    const clean = q.trim().toLowerCase();
    if (!clean) return true;
    const hay = `${labelForItem(item)} ${item.label} ${item.id} ${labelForGroup(item.group)}`.toLowerCase();
    if (hay.includes(clean)) return true;
    let qi = 0;
    for (let i = 0; i < hay.length && qi < clean.length; i++) {
      if (hay[i] === clean[qi]) qi++;
    }
    return qi === clean.length;
  }

  let filtered = $derived(NAV_ITEMS.filter((item) => matchQuery(item, query)));

  // Reset indeks dan fokuskan input setiap kali palette dibuka
  $effect(() => {
    if (open) {
      query = '';
      activeIndex = 0;
      queueMicrotask(() => inputEl?.focus());
    }
  });

  // Jaga agar indeks aktif selalu berada dalam rentang hasil filter
  $effect(() => {
    if (activeIndex >= filtered.length) {
      activeIndex = Math.max(0, filtered.length - 1);
    }
  });

  function choose(item: NavItem) {
    window.location.hash = `#/${item.id}`;
    onSelect?.(item.id);
    onClose();
  }

  function handleKeydown(e: KeyboardEvent) {
    if (!open) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (filtered.length > 0) {
        activeIndex = (activeIndex + 1) % filtered.length;
      }
      return;
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (filtered.length > 0) {
        activeIndex = (activeIndex - 1 + filtered.length) % filtered.length;
      }
      return;
    }
    if (e.key === 'Enter' && filtered[activeIndex]) {
      e.preventDefault();
      choose(filtered[activeIndex]);
    }
  }
</script>

{#if open}
  <div class="palette" role="dialog" aria-modal="true" aria-label="Pencarian cepat menu" tabindex="-1" onkeydown={handleKeydown}>
    <div class="palette__backdrop" role="presentation" onclick={onClose}></div>

    <div class="palette__panel">
      <div class="palette__search">
        <i class="ri-search-line palette__search-icon" aria-hidden="true"></i>
        <input
          bind:this={inputEl}
          bind:value={query}
          type="text"
          class="palette__input"
          placeholder={t('shell.palette.placeholder')}
          aria-label={t('shell.search_menu')}
          autocomplete="off"
          spellcheck="false"
        />
        <kbd class="palette__kbd">Esc</kbd>
      </div>

      <div class="palette__list" role="listbox" aria-label={t('shell.search_menu')}>
        {#if filtered.length === 0}
          <div class="palette__empty">
            <i class="ri-compass-3-line" aria-hidden="true"></i>
            <span>{t('shell.palette.empty', { query })}</span>
          </div>
        {:else}
          {#each filtered as item, idx (item.id)}
            <button
              type="button"
              role="option"
              aria-selected={idx === activeIndex}
              class="palette__item"
              class:is-active={idx === activeIndex}
              class:is-current={item.id === current}
              onmouseenter={() => (activeIndex = idx)}
              onclick={() => choose(item)}
            >
              <i class="{item.icon} palette__item-icon" aria-hidden="true"></i>
              <span class="palette__item-label">{labelForItem(item)}</span>
              {#if item.id === current}
                <span class="badge badge--success">{t('shell.palette.active')}</span>
              {/if}
              <span class="palette__item-group">{labelForGroup(item.group)}</span>
            </button>
          {/each}
        {/if}
      </div>

      <div class="palette__foot">
        <span><kbd>↑</kbd> <kbd>↓</kbd> {t('shell.palette.nav')}</span>
        <span><kbd>Enter</kbd> {t('shell.palette.open')}</span>
        <span><kbd>Esc</kbd> {t('shell.palette.close')}</span>
      </div>
    </div>
  </div>
{/if}
