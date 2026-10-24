<script lang="ts">
  import type { OverviewData } from '../types';
  import { copyText } from '../utils/browser';
  import { notify } from '../stores/toast.svelte';

  interface Props {
    overview: OverviewData | null;
    user: string;
    live: boolean;
    menuOpen: boolean;
    refreshing: boolean;
    onToggleMenu: () => void;
    onRefresh: () => void;
    onLogout: () => void;
  }

  let { overview, user, live, menuOpen, refreshing, onToggleMenu, onRefresh, onLogout }: Props = $props();

  // Preferensi sembunyikan IP disimpan agar aman saat screenshot/berbagi layar
  let maskIP = $state(localStorage.getItem('fp:mask-ip') === '1');
  $effect(() => {
    localStorage.setItem('fp:mask-ip', maskIP ? '1' : '0');
  });

  let ip = $derived(overview?.system?.ip || '127.0.0.1');

  async function copyIP() {
    try {
      await copyText(ip);
      notify('Alamat IP disalin', 'success');
    } catch (err) {
      notify(`Gagal menyalin IP: ${(err as Error).message}`, 'error');
    }
  }
</script>

<header class="topbar">
  <div class="row">
    <button
      class="btn btn--ghost btn--icon topbar__menu"
      onclick={onToggleMenu}
      aria-label={menuOpen ? 'Tutup menu' : 'Buka menu'}
      aria-expanded={menuOpen}
      aria-controls="app-sidebar"
    >
      <i class={menuOpen ? 'ri-close-line' : 'ri-menu-line'} aria-hidden="true"></i>
    </button>

    <div class="ip">
      <button class="ip__value" onclick={copyIP} title="Salin alamat IP">
        <span class="dot dot--success" aria-hidden="true"></span>
        <span class="mono tabular">{maskIP ? '•••.•••.•••.•••' : ip}</span>
        <i class="ri-file-copy-line" aria-hidden="true"></i>
      </button>
      <button
        class="ip__mask"
        onclick={() => (maskIP = !maskIP)}
        aria-label={maskIP ? 'Tampilkan IP' : 'Sembunyikan IP'}
        aria-pressed={maskIP}
        title={maskIP ? 'Tampilkan IP' : 'Sembunyikan IP untuk screenshot'}
      >
        <i class={maskIP ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
      </button>
    </div>

    <span class="badge topbar__os"><i class="ri-computer-line" aria-hidden="true"></i>{overview?.system?.os || 'Linux'}</span>

    <span class="badge badge--pill {live ? 'badge--success' : 'badge--warning'}" role="status">
      <span class="dot {live ? 'dot--success' : 'dot--warning dot--pulse'}" aria-hidden="true"></span>
      {live ? 'Live' : 'Menyambung ulang…'}
    </span>
  </div>

  <div class="row">
    <a href="#/terminal" class="btn btn--sm topbar__wide"><i class="ri-terminal-line" aria-hidden="true"></i> Terminal</a>
    <button class="btn btn--sm" onclick={onRefresh} disabled={refreshing} aria-label="Muat ulang data">
      <i class="ri-refresh-line" class:spin={refreshing} aria-hidden="true"></i>
      <span class="topbar__wide">Refresh</span>
    </button>
    <span class="topbar__user topbar__wide"><i class="ri-user-3-line" aria-hidden="true"></i>{user}</span>
    <button class="btn btn--sm btn--danger" onclick={onLogout}>
      <i class="ri-logout-box-r-line" aria-hidden="true"></i><span class="topbar__wide">Keluar</span>
    </button>
  </div>
</header>

<style>
  .topbar {
    position: sticky;
    top: 0;
    z-index: var(--z-sticky);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    height: var(--topbar-height);
    padding: 0 var(--space-6);
    background: color-mix(in srgb, var(--bg-surface) 88%, transparent);
    backdrop-filter: blur(10px);
    border-bottom: 1px solid var(--border-subtle);
  }
  .topbar__menu {
    display: none;
  }
  .ip {
    display: inline-flex;
    align-items: center;
    height: var(--control-sm);
    padding: 2px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--bg-raised);
  }
  .ip__value,
  .ip__mask {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    height: 100%;
    padding: 0 var(--space-2);
    border-radius: 4px;
    font-size: var(--text-xs);
    color: var(--text);
    transition: background var(--duration-fast) ease, color var(--duration-fast) ease;
  }
  .ip__value i,
  .ip__mask {
    color: var(--text-subtle);
  }
  .ip__value:hover,
  .ip__mask:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .topbar__user {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 0 var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-secondary);
  }
  @media (max-width: 900px) {
    .topbar__wide,
    .topbar__os { display: none; }
  }
  @media (max-width: 768px) {
    .topbar { padding: 0 var(--space-3); }
    .topbar__menu { display: inline-flex; }
  }
</style>
