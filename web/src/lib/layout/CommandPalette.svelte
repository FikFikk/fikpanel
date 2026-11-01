<script lang="ts">
  import { NAV_GROUPS, NAV_ITEMS, type NavItem } from './nav';

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

  const groupLabels = new Map(NAV_GROUPS.map((g) => [g.id, g.label]));

  // Filter fuzzy sederhana berdasarkan label, id rute, atau nama grup
  function matchQuery(item: NavItem, q: string): boolean {
    const clean = q.trim().toLowerCase();
    if (!clean) return true;
    const hay = `${item.label} ${item.id} ${groupLabels.get(item.group) ?? ''}`.toLowerCase();
    if (hay.includes(clean)) return true;
    // Pencocokan karakter berurutan (fuzzy subsequence)
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
          placeholder="Cari halaman atau menu (mis. Website, Docker, WAF)…"
          aria-label="Kata kunci pencarian menu"
          autocomplete="off"
          spellcheck="false"
        />
        <kbd class="palette__kbd">Esc</kbd>
      </div>

      <div class="palette__list" role="listbox" aria-label="Hasil pencarian menu">
        {#if filtered.length === 0}
          <div class="palette__empty">
            <i class="ri-Compass-3-line" aria-hidden="true"></i>
            <span>Tidak ada menu yang cocok dengan "{query}"</span>
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
              <span class="palette__item-label">{item.label}</span>
              {#if item.id === current}
                <span class="badge badge--success">Aktif</span>
              {/if}
              <span class="palette__item-group">{groupLabels.get(item.group)}</span>
            </button>
          {/each}
        {/if}
      </div>

      <div class="palette__foot">
        <span><kbd>↑</kbd> <kbd>↓</kbd> Navigasi</span>
        <span><kbd>Enter</kbd> Buka halaman</span>
        <span><kbd>Esc</kbd> Tutup</span>
      </div>
    </div>
  </div>
{/if}
