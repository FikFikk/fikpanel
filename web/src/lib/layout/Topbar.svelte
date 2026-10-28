<script lang="ts">
  import { onMount } from 'svelte';
  import type { OverviewData } from '../types';
  import { copyText } from '../utils/browser';
  import { notify } from '../stores/toast.svelte';

  interface Props {
    overview: OverviewData | null;
    user?: string;
    menuOpen: boolean;
    onToggleMenu: () => void;
  }

  let { overview, user = 'root', menuOpen, onToggleMenu }: Props = $props();

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

  // Live Clock untuk Server Time & Client Time
  let clientNow = $state(new Date());
  let serverDeltaMs = $state(0);
  let serverTzName = $state('UTC');

  $effect(() => {
    if (overview?.server_time?.unix) {
      const srvMs = overview.server_time.unix * 1000;
      serverDeltaMs = srvMs - Date.now();
      if (overview.server_time.timezone_name) {
        serverTzName = overview.server_time.timezone_name;
      }
    }
  });

  let serverNow = $derived(new Date(clientNow.getTime() + serverDeltaMs));

  function getClientTimezoneLabel(): string {
    const offsetMin = -new Date().getTimezoneOffset();
    const sign = offsetMin >= 0 ? '+' : '-';
    const hrs = Math.floor(Math.abs(offsetMin) / 60);
    const isWIB = offsetMin === 420;
    return isWIB ? 'WIB (UTC+7)' : `UTC${sign}${hrs}`;
  }

  function formatTime(d: Date): string {
    return d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
  }

  function formatDate(d: Date): string {
    return d.toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' });
  }

  onMount(() => {
    const timer = setInterval(() => {
      clientNow = new Date();
    }, 1000);
    return () => clearInterval(timer);
  });
</script>

<header class="topbar">
  <!-- Sisi Kiri: Menu Mobile, IP Host, & OS -->
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
      <button class="ip__value" onclick={copyIP} title="Salin alamat IP server">
        <span class="dot dot--success" aria-hidden="true"></span>
        <span class="mono tabular">{maskIP ? '•••.•••.•••.•••' : ip}</span>
        <i class="ri-file-copy-line" aria-hidden="true"></i>
      </button>
      <button
        class="ip__mask"
        onclick={() => (maskIP = !maskIP)}
        aria-label={maskIP ? 'Tampilkan IP' : 'Sembunyikan IP'}
        aria-pressed={maskIP}
        title={maskIP ? 'Tampilkan IP' : 'Sembunyikan IP untuk tangkapan layar'}
      >
        <i class={maskIP ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
      </button>
    </div>

    <span class="badge topbar__os">
      <i class="ri-computer-line" aria-hidden="true"></i>
      {overview?.system?.os || 'Linux'}
    </span>
  </div>

  <!-- Sisi Kanan: Live Global Clock Pill & Info Akun Admin -->
  <div class="row right-row">
    <!-- Live Global Server & Client Clock -->
    <div class="time-pill" title="Jam Server host (UTC) dan jam lokal Anda yang selalu tersinkronisasi">
      <div class="time-item">
        <i class="ri-server-line time-icon"></i>
        <span class="time-label">Server:</span>
        <span class="mono tabular time-val">{formatDate(serverNow)}, {formatTime(serverNow)}</span>
        <span class="tz-badge">{serverTzName}</span>
      </div>

      <span class="time-sep">|</span>

      <div class="time-item client-time">
        <i class="ri-user-location-line time-icon"></i>
        <span class="time-label">Anda:</span>
        <span class="mono tabular time-val">{formatTime(clientNow)}</span>
        <span class="tz-badge tz-badge--highlight">{getClientTimezoneLabel()}</span>
      </div>
    </div>

    <!-- User Profile Badge -->
    <span class="topbar__user" title="Akun panel aktif">
      <i class="ri-user-3-line" aria-hidden="true"></i>
      <span>{user}</span>
    </span>
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
  .row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .right-row {
    gap: var(--space-3);
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

  /* ═══ Live Clock Pill Global ═══ */
  .time-pill {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.2rem 0.65rem;
    font-size: var(--text-xs);
    color: var(--text);
  }
  .time-item {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }
  .time-icon {
    color: var(--accent-strong);
    font-size: 0.85rem;
  }
  .time-label {
    color: var(--text-muted);
    font-size: 0.68rem;
    font-weight: var(--weight-medium);
  }
  .time-val {
    color: var(--text);
    font-weight: var(--weight-semibold);
    font-size: 0.72rem;
  }
  .tabular {
    font-variant-numeric: tabular-nums;
  }
  .time-sep {
    color: var(--border-strong);
    user-select: none;
    font-size: 0.75rem;
  }
  .tz-badge {
    font-size: 0.62rem;
    padding: 0.05rem 0.35rem;
    border-radius: 4px;
    background: var(--bg-hover);
    color: var(--text-subtle);
    font-weight: var(--weight-semibold);
  }
  .tz-badge--highlight {
    background: var(--tint-accent);
    color: var(--accent-strong);
    border: 1px solid color-mix(in srgb, var(--accent) 25%, transparent);
  }

  /* ═══ User Pill ═══ */
  .topbar__user {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: 0.25rem 0.65rem;
    height: var(--control-sm);
    border-radius: var(--radius-sm);
    background: var(--bg-raised);
    border: 1px solid var(--border);
    font-size: var(--text-xs);
    color: var(--text-secondary);
    font-weight: var(--weight-semibold);
  }
  .topbar__user i {
    color: var(--accent);
  }

  /* ═══ Responsif ═══ */
  @media (max-width: 1024px) {
    .client-time,
    .time-sep {
      display: none;
    }
  }
  @media (max-width: 820px) {
    .time-pill,
    .topbar__os {
      display: none;
    }
  }
  @media (max-width: 768px) {
    .topbar {
      padding: 0 var(--space-3);
    }
    .topbar__menu {
      display: inline-flex;
    }
  }
</style>
