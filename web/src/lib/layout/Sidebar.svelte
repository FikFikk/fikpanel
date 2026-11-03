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

  let { current, open, version = '0.3.11', onNavigate, onLogout, onOpenPalette }: Props = $props();

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
    <svg class="sidebar__brand-icon" viewBox="0 0 48 48" width="26" height="26" aria-hidden="true">
      <path fill="var(--accent)" fill-rule="evenodd" d="M10 0H38C43.52 0 48 4.48 48 10V38C48 43.52 43.52 48 38 48H17.5V10H10V48C4.48 48 0 43.52 0 38V10C0 4.48 4.48 0 10 0ZM20.5 10V16H38V10ZM20.5 19V28H31.5V19Z" />
    </svg>
    <svg class="sidebar__brand-wordmark" viewBox="0 0 373.60 90.10" width="84" height="20" role="img" aria-label="fikpanel">
      <path fill="var(--text)" d="M7.5 72.3V30.4H0V19.7H7.5V13.9Q7.5 9.6 8.95 6.25Q10.4 2.9 13.6 1Q16.8 -0.9 22.1 -0.9Q23.6 -0.9 25.25 -0.7Q26.9 -0.5 28.45 -0.15Q30 0.2 31.1 0.6V9.8H26.1Q23.7 9.8 22.55 11Q21.4 12.2 21.4 14.5V19.7H31.1V30.4H21.4V72.3Z M38.8 12V0H52.7V12ZM38.8 72.3V19.7H52.7V72.3Z M65.9 72.3H79.8V-0.1H65.9ZM85.36 34.98V52.09L87.4 50.1L99.5 72.3H115.4L96.5 40.3L114.5 19.7H98.2Z M123.1 90.1V19.7H134.5L135.7 26.8H136.4Q139 22.5 143.2 20.5Q147.4 18.5 152.5 18.5Q159.1 18.5 163.85 21.4Q168.6 24.3 171.15 30.4Q173.7 36.5 173.7 46.1Q173.7 55.4 171.1 61.5Q168.5 67.6 163.85 70.55Q159.2 73.5 153 73.5Q149.6 73.5 146.7 72.75Q143.8 72 141.5 70.4Q139.2 68.8 137.6 66.5H137V90.1ZM148.2 62.1Q152.4 62.1 154.85 60.45Q157.3 58.8 158.4 55.45Q159.5 52.1 159.5 47.2V44.9Q159.5 39.9 158.4 36.55Q157.3 33.2 154.85 31.55Q152.4 29.9 148.2 29.9Q144.3 29.9 141.8 31.7Q139.3 33.5 138.1 36.9Q136.9 40.3 136.9 45.1V46.8Q136.9 50.3 137.6 53.15Q138.3 56 139.65 58Q141 60 143.1 61.05Q145.2 62.1 148.2 62.1Z M197.2 73.5Q194.6 73.5 191.65 72.9Q188.7 72.3 186.1 70.75Q183.5 69.2 181.85 66.25Q180.2 63.3 180.2 58.7Q180.2 53.2 182.6 49.6Q185 46 189.45 43.95Q193.9 41.9 200.1 41.1Q206.3 40.3 213.8 40.3V36.4Q213.8 34.1 213.05 32.4Q212.3 30.7 210.45 29.7Q208.6 28.7 205.3 28.7Q202 28.7 199.95 29.5Q197.9 30.3 197.05 31.55Q196.2 32.8 196.2 34.3V35.5H182.7Q182.6 35 182.6 34.55Q182.6 34.1 182.6 33.4Q182.6 28.8 185.4 25.45Q188.2 22.1 193.3 20.3Q198.4 18.5 205.2 18.5Q212.5 18.5 217.45 20.4Q222.4 22.3 225.05 26Q227.7 29.7 227.7 35.3V59.2Q227.7 61 228.75 61.85Q229.8 62.7 231.1 62.7H234.4V71.8Q233.4 72.3 231.3 72.85Q229.2 73.4 226.2 73.4Q223.3 73.4 221.05 72.55Q218.8 71.7 217.3 70.15Q215.8 68.6 215.2 66.4H214.5Q212.8 68.5 210.35 70.1Q207.9 71.7 204.65 72.6Q201.4 73.5 197.2 73.5ZM201.9 63Q204.7 63 206.9 62.15Q209.1 61.3 210.6 59.85Q212.1 58.4 212.95 56.3Q213.8 54.2 213.8 51.8V48.9Q208.2 48.9 203.8 49.65Q199.4 50.4 196.85 52.2Q194.3 54 194.3 57.2Q194.3 59 195.2 60.3Q196.1 61.6 197.8 62.3Q199.5 63 201.9 63Z M241.3 72.3V19.7H252.8L253.9 26.9H254.6Q256.4 24.3 258.95 22.4Q261.5 20.5 264.7 19.5Q267.9 18.5 271.7 18.5Q276.9 18.5 280.8 20.4Q284.7 22.3 286.85 26.4Q289 30.5 289 37.1V72.3H275.1V39.3Q275.1 36.8 274.5 35.05Q273.9 33.3 272.75 32.15Q271.6 31 269.9 30.5Q268.2 30 266.1 30Q263 30 260.5 31.5Q258 33 256.6 35.6Q255.2 38.2 255.2 41.6V72.3Z M325 73.5Q316.3 73.5 310.4 70.55Q304.5 67.6 301.5 61.5Q298.5 55.4 298.5 46Q298.5 36.5 301.5 30.45Q304.5 24.4 310.35 21.45Q316.2 18.5 324.7 18.5Q332.7 18.5 338.2 21.35Q343.7 24.2 346.5 30.15Q349.3 36.1 349.3 45.5V49.1H312.6Q312.8 53.7 314.05 56.9Q315.3 60.1 317.95 61.65Q320.6 63.2 325 63.2Q327.4 63.2 329.35 62.6Q331.3 62 332.7 60.8Q334.1 59.6 334.9 57.8Q335.7 56 335.7 53.7H349.3Q349.3 58.7 347.5 62.4Q345.7 66.1 342.5 68.55Q339.3 71 334.85 72.25Q330.4 73.5 325 73.5ZM312.8 40.3H335Q335 37.3 334.25 35.1Q333.5 32.9 332.2 31.5Q330.9 30.1 329.05 29.45Q327.2 28.8 324.9 28.8Q321.1 28.8 318.55 30.05Q316 31.3 314.65 33.85Q313.3 36.4 312.8 40.3Z M359.7 72.3V0H373.6V72.3Z" />
    </svg>
    <span class="sr-only">fikpanel</span>
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
