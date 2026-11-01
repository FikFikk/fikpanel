<script lang="ts">
  import { onMount } from 'svelte';
  import type { OverviewData } from '../types';
  import { copyText } from '../utils/browser';
  import { notify } from '../stores/toast.svelte';
  import { cycleTheme, themeState } from '../stores/theme.svelte';

  interface Props {
    overview: OverviewData | null;
    user?: string;
    menuOpen: boolean;
    onToggleMenu: () => void;
    onOpenPalette?: () => void;
    onLogout?: () => void;
  }

  let { overview, user = 'root', menuOpen, onToggleMenu, onOpenPalette, onLogout }: Props = $props();

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

  // Status dropdown menu pengguna
  let userMenuOpen = $state(false);
  let userWrapEl = $state<HTMLDivElement | null>(null);

  let themeIcon = $derived(
    themeState.mode === 'light'
      ? 'ri-sun-line'
      : themeState.mode === 'auto'
        ? 'ri-contrast-2-line'
        : 'ri-moon-line'
  );

  let themeLabel = $derived(
    themeState.mode === 'light'
      ? 'Tema: Terang (klik untuk Otomatis)'
      : themeState.mode === 'auto'
        ? 'Tema: Otomatis (klik untuk Gelap)'
        : 'Tema: Gelap (klik untuk Terang)'
  );

  onMount(() => {
    const timer = setInterval(() => {
      clientNow = new Date();
    }, 1000);

    const onDocClick = (e: MouseEvent) => {
      if (userMenuOpen && userWrapEl && !userWrapEl.contains(e.target as Node)) {
        userMenuOpen = false;
      }
    };
    const onDocKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && userMenuOpen) {
        userMenuOpen = false;
      }
    };
    document.addEventListener('click', onDocClick);
    document.addEventListener('keydown', onDocKey);

    return () => {
      clearInterval(timer);
      document.removeEventListener('click', onDocClick);
      document.removeEventListener('keydown', onDocKey);
    };
  });
</script>

<header class="topbar">
  <!-- Sisi Kiri: Menu Mobile, IP Host, & OS -->
  <div class="topbar__left">
    <button
      type="button"
      class="btn btn--ghost btn--icon topbar__menu"
      onclick={onToggleMenu}
      aria-label={menuOpen ? 'Tutup menu' : 'Buka menu'}
      aria-expanded={menuOpen}
      aria-controls="app-sidebar"
    >
      <i class={menuOpen ? 'ri-close-line' : 'ri-menu-line'} aria-hidden="true"></i>
    </button>

    <div class="topbar__ip">
      <button type="button" class="topbar__ip-value" onclick={copyIP} title="Salin alamat IP server">
        <span class="dot dot--success" aria-hidden="true"></span>
        <span class="mono tabular">{maskIP ? '•••.•••.•••.•••' : ip}</span>
        <i class="ri-file-copy-line" aria-hidden="true"></i>
      </button>
      <button
        type="button"
        class="topbar__ip-mask"
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

  <!-- Sisi Kanan: Command Palette, Live Global Clock Pill, Tema, & Menu Akun Admin -->
  <div class="topbar__right">
    {#if onOpenPalette}
      <button
        type="button"
        class="topbar__palette-btn"
        onclick={onOpenPalette}
        aria-label="Buka pencarian cepat (Ctrl+K)"
        title="Pencarian cepat (Ctrl+K)"
      >
        <i class="ri-search-line" aria-hidden="true"></i>
        <span class="topbar__palette-label">Cari…</span>
        <kbd>Ctrl K</kbd>
      </button>
    {/if}

    <!-- Live Global Server & Client Clock -->
    <div class="topbar__time-pill" title="Jam Server host (UTC) dan jam lokal Anda yang selalu tersinkronisasi">
      <div class="topbar__time-item">
        <i class="ri-server-line topbar__time-icon" aria-hidden="true"></i>
        <span class="topbar__time-label">Server:</span>
        <span class="mono tabular topbar__time-val">{formatDate(serverNow)}, {formatTime(serverNow)}</span>
        <span class="topbar__tz-badge">{serverTzName}</span>
      </div>

      <span class="topbar__time-sep">|</span>

      <div class="topbar__time-item topbar__client-time">
        <i class="ri-user-location-line topbar__time-icon" aria-hidden="true"></i>
        <span class="topbar__time-label">Anda:</span>
        <span class="mono tabular topbar__time-val">{formatTime(clientNow)}</span>
        <span class="topbar__tz-badge topbar__tz-badge--highlight">{getClientTimezoneLabel()}</span>
      </div>
    </div>

    <!-- Toggle Tema (Dark / Light / Auto) -->
    <button
      type="button"
      class="btn btn--ghost btn--icon btn--sm topbar__theme-btn"
      onclick={cycleTheme}
      aria-label={themeLabel}
      title={themeLabel}
    >
      <i class={themeIcon} aria-hidden="true"></i>
    </button>

    <!-- User Profile Menu -->
    <div class="topbar__user-wrap" bind:this={userWrapEl}>
      <button
        type="button"
        class="topbar__user"
        onclick={() => (userMenuOpen = !userMenuOpen)}
        aria-expanded={userMenuOpen}
        aria-haspopup="menu"
        title="Menu akun panel aktif"
      >
        <i class="ri-user-3-line" aria-hidden="true"></i>
        <span>{user}</span>
        <i class="ri-arrow-down-s-line topbar__user-caret" aria-hidden="true"></i>
      </button>

      {#if userMenuOpen}
        <div class="topbar__user-menu" role="menu" aria-label="Menu akun pengguna">
          <div class="topbar__user-info">
            <span class="topbar__user-role">Masuk sebagai</span>
            <strong class="mono">{user}</strong>
          </div>
          {#if onLogout}
            <button
              type="button"
              role="menuitem"
              class="topbar__user-logout"
              onclick={() => {
                userMenuOpen = false;
                onLogout();
              }}
            >
              <i class="ri-logout-box-r-line" aria-hidden="true"></i>
              <span>Keluar</span>
            </button>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</header>
