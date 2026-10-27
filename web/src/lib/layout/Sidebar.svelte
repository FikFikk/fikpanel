<script lang="ts">
  import { NAV_ITEMS } from './nav';

  interface Props {
    current: string;
    open: boolean;
    version?: string;
    onNavigate: () => void;
  }

  let { current, open, version = '0.1.9', onNavigate }: Props = $props();

  const groups = [
    { id: 'utama', label: 'Kelola' },
    { id: 'sistem', label: 'Sistem' },
  ] as const;
</script>

<aside class="sidebar" class:open id="app-sidebar" aria-label="Navigasi utama">
  <div class="sidebar__brand">
    <svg viewBox="0 0 32 32" width="28" height="28" aria-hidden="true">
      <rect width="32" height="32" rx="8" fill="var(--accent)" />
      <path d="M9 11h14M9 16h10M9 21h14" stroke="var(--text-on-accent)" stroke-width="2.2" stroke-linecap="round" />
    </svg>
    <span class="sidebar__name">fikpanel</span>
  </div>

  <nav class="sidebar__nav">
    {#each groups as g (g.id)}
      <p class="sidebar__group">{g.label}</p>
      <ul>
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
              <span>{item.label}</span>
            </a>
          </li>
        {/each}
      </ul>
    {/each}
  </nav>

  <div class="sidebar__foot">{version.startsWith('v') ? version : `v${version}`}</div>
</aside>

<style>
  .sidebar {
    position: fixed;
    inset: 0 auto 0 0;
    z-index: var(--z-sidebar);
    display: flex;
    flex-direction: column;
    width: var(--sidebar-width);
    background: var(--bg-surface);
    border-right: 1px solid var(--border-subtle);
    transition: transform var(--duration-base) var(--ease-out);
  }
  .sidebar__brand {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    height: var(--topbar-height);
    padding: 0 var(--space-5);
    border-bottom: 1px solid var(--border-subtle);
  }
  .sidebar__name {
    font-size: var(--text-lg);
    font-weight: var(--weight-bold);
    letter-spacing: -0.02em;
  }
  .sidebar__nav {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-3) var(--space-2);
  }
  .sidebar__nav ul {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin-bottom: var(--space-3);
  }
  .sidebar__group {
    padding: var(--space-2) var(--space-3) var(--space-1);
    font-size: var(--text-2xs);
    font-weight: var(--weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-disabled);
  }
  .sidebar__item {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    height: 36px;
    padding: 0 var(--space-3);
    border-radius: var(--radius-md);
    color: var(--text-muted);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    text-decoration: none;
    transition: background var(--duration-fast) ease, color var(--duration-fast) ease;
  }
  .sidebar__item i {
    font-size: 1.05rem;
  }
  .sidebar__item:hover {
    background: var(--bg-hover);
    color: var(--text);
    text-decoration: none;
  }
  .sidebar__item[aria-current='page'] {
    background: linear-gradient(90deg, var(--tint-accent), transparent);
    color: var(--accent-strong);
    font-weight: var(--weight-semibold);
  }
  .sidebar__item[aria-current='page']::before {
    content: '';
    position: absolute;
    left: 0;
    top: 22%;
    bottom: 22%;
    width: 3px;
    border-radius: 2px;
    background: var(--accent);
  }
  .sidebar__foot {
    padding: var(--space-3) var(--space-5);
    border-top: 1px solid var(--border-subtle);
    font-size: var(--text-2xs);
    color: var(--text-disabled);
  }

  /* Tablet: sidebar ringkas ikon saja */
  @media (max-width: 1024px) and (min-width: 769px) {
    .sidebar { width: var(--sidebar-collapsed); }
    .sidebar__name,
    .sidebar__group,
    .sidebar__item span,
    .sidebar__foot { display: none; }
    .sidebar__brand { justify-content: center; padding: 0; }
    .sidebar__item { justify-content: center; padding: 0; }
  }
  /* Mobile: drawer tersembunyi, dibuka lewat tombol menu di topbar */
  @media (max-width: 768px) {
    .sidebar {
      transform: translateX(-100%);
      box-shadow: var(--shadow-lg);
    }
    .sidebar.open { transform: translateX(0); }
    .sidebar__item { height: var(--control-lg); }
  }
</style>
