<script lang="ts">
  import { NAV_GROUPS, NAV_ITEMS, type NavGroupId } from './nav';

  interface Props {
    current: string;
    open: boolean;
    version?: string;
    onNavigate: () => void;
    onLogout?: () => void;
    onOpenPalette?: () => void;
  }

  let { current, open, version = '0.3.4', onNavigate, onLogout, onOpenPalette }: Props = $props();

  const COLLAPSED_KEY = 'fp:nav-collapsed';

  function loadCollapsed(): Record<string, boolean> {
    try {
      const raw = localStorage.getItem(COLLAPSED_KEY);
      if (raw) {
        const parsed = JSON.parse(raw);
        if (parsed && typeof parsed === 'object') return parsed as Record<string, boolean>;
      }
    } catch {
      // Abaikan bila data localStorage rusak
    }
    return {};
  }

  let collapsed = $state<Record<string, boolean>>(loadCollapsed());

  function toggleGroup(id: NavGroupId) {
    collapsed = { ...collapsed, [id]: !collapsed[id] };
    try {
      localStorage.setItem(COLLAPSED_KEY, JSON.stringify(collapsed));
    } catch {
      // Abaikan kegagalan simpan localStorage
    }
  }
</script>

<aside class="sidebar" class:open id="app-sidebar" aria-label="Navigasi utama">
  <div class="sidebar__brand">
    <svg viewBox="0 0 32 32" width="28" height="28" aria-hidden="true">
      <rect width="32" height="32" rx="8" fill="var(--accent)" />
      <path d="M9 11h14M9 16h10M9 21h14" stroke="var(--text-on-accent)" stroke-width="2.2" stroke-linecap="round" />
    </svg>
    <span class="sidebar__name">fikpanel</span>
  </div>

  {#if onOpenPalette}
    <div class="sidebar__search">
      <button
        type="button"
        class="sidebar__search-btn"
        onclick={onOpenPalette}
        aria-label="Buka pencarian cepat (Ctrl+K)"
        title="Cari menu cepat (Ctrl+K)"
      >
        <i class="ri-search-line" aria-hidden="true"></i>
        <span class="sidebar__search-text">Cari menu…</span>
        <kbd class="sidebar__search-kbd">Ctrl K</kbd>
      </button>
    </div>
  {/if}

  <nav class="sidebar__nav">
    {#each NAV_GROUPS as g (g.id)}
      <div class="sidebar__section">
        <button
          type="button"
          class="sidebar__group-btn"
          aria-expanded={!collapsed[g.id]}
          onclick={() => toggleGroup(g.id)}
          title="Lipat atau buka grup {g.label}"
        >
          <span>{g.label}</span>
          <i
            class="ri-arrow-down-s-line sidebar__group-arrow"
            class:is-collapsed={Boolean(collapsed[g.id])}
            aria-hidden="true"
          ></i>
        </button>

        <ul class="sidebar__list" class:is-collapsed={Boolean(collapsed[g.id])}>
          {#each NAV_ITEMS.filter((n) => n.group === g.id) as item (item.id)}
            <li>
              <a
                href="#/{item.id}"
                class="sidebar__item"
                aria-current={current === item.id ? 'page' : undefined}
                title={item.label}
                onclick={onNavigate}
              >
                <i class={item.icon} aria-hidden="true"></i>
                <span class="sidebar__item-label">{item.label}</span>
                {#if item.badge}
                  <span class="badge sidebar__badge">{item.badge}</span>
                {/if}
              </a>
            </li>
          {/each}
        </ul>
      </div>
    {/each}
  </nav>

  <div class="sidebar__foot">
    <span class="sidebar__version">{version.startsWith('v') ? version : `v${version}`}</span>
    {#if onLogout}
      <button type="button" class="sidebar__logout-btn" onclick={onLogout} title="Keluar dari sesi FikPanel">
        <i class="ri-logout-box-r-line" aria-hidden="true"></i>
        <span>Keluar</span>
      </button>
    {/if}
  </div>
</aside>
