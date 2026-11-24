<script lang="ts">
  import { onMount } from 'svelte';
  import type { OverviewData } from '../types';
  import { copyText } from '../utils/browser';
  import { notify } from '../stores/toast.svelte';
  import { appearanceState, cycleTheme, themeState } from '../stores/theme.svelte';
  import { dateLocale, i18nState, t, toggleLocale } from '../stores/i18n.svelte';

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
      notify(t('shell.ip_copied'), 'success');
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
    return d.toLocaleTimeString(dateLocale(), { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
  }

  function formatDate(d: Date): string {
    return d.toLocaleDateString(dateLocale(), { day: '2-digit', month: 'short', year: 'numeric' });
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
      ? t('shell.theme.light')
      : themeState.mode === 'auto'
        ? t('shell.theme.auto')
        : t('shell.theme.dark')
  );

  let envBadgeLabel = $derived(
    appearanceState.env_badge === 'production'
      ? t('shell.env.production')
      : appearanceState.env_badge === 'staging'
        ? t('shell.env.staging')
        : appearanceState.env_badge === 'development'
          ? t('shell.env.development')
          : ''
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
  <!-- Sisi Kiri: Menu Mobile, Brand Ringkas Mobile, IP Host, & OS -->
  <div class="topbar__left">
    <button
      type="button"
      class="topbar__menu"
      onclick={onToggleMenu}
      aria-label={menuOpen ? 'Tutup menu' : 'Buka menu'}
      title={menuOpen ? 'Tutup menu' : 'Buka menu'}
      aria-expanded={menuOpen}
      aria-controls="app-sidebar"
    >
      <i class={menuOpen ? 'ri-close-line' : 'ri-menu-line'} aria-hidden="true"></i>
    </button>

    <!-- Identitas ringkas yang hanya tampil pada layar ponsel sempit -->
    <div class="topbar__mobile-brand">
      <span class="dot dot--success" aria-hidden="true"></span>
      <span class="topbar__mobile-brand-name">{appearanceState.brand_name || 'fikpanel'}</span>
    </div>

    <div class="topbar__ip">
      <button type="button" class="topbar__ip-value" onclick={copyIP} title={t('shell.copy_ip')}>
        <span class="dot dot--success" aria-hidden="true"></span>
        <span class="mono tabular">{maskIP ? '•••.•••.•••.•••' : ip}</span>
        <i class="ri-file-copy-line" aria-hidden="true"></i>
      </button>
      <button
        type="button"
        class="topbar__ip-mask"
        onclick={() => (maskIP = !maskIP)}
        aria-label={maskIP ? t('shell.show_ip') : t('shell.hide_ip')}
        aria-pressed={maskIP}
        title={maskIP ? t('shell.show_ip') : t('shell.hide_ip')}
      >
        <i class={maskIP ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
      </button>
    </div>

    {#if envBadgeLabel}
      <span
        class="topbar__env-badge"
        class:topbar__env-badge--prod={appearanceState.env_badge === 'production'}
        class:topbar__env-badge--staging={appearanceState.env_badge === 'staging'}
        class:topbar__env-badge--dev={appearanceState.env_badge === 'development'}
      >
        {envBadgeLabel}
      </span>
    {/if}

    <span class="topbar__os" title={overview?.system?.os || 'Linux'}>
      <i class="ri-computer-line" aria-hidden="true"></i>
      <span class="topbar__os-text">{overview?.system?.os || 'Linux'}</span>
    </span>
  </div>

  <!-- Sisi Kanan: Command Palette, Live Global Clock Pill, Bahasa, Tema, & Menu Akun Admin -->
  <div class="topbar__right">
    {#if onOpenPalette}
      <button
        type="button"
        class="topbar__palette-btn"
        onclick={onOpenPalette}
        aria-label={t('shell.search_menu')}
        title={`${t('shell.search_menu')} (Ctrl+K)`}
      >
        <i class="ri-search-line" aria-hidden="true"></i>
        <span class="topbar__palette-label">{t('shell.search_short')}</span>
        <kbd>Ctrl K</kbd>
      </button>
    {/if}

    <!-- Live Global Server & Client Clock -->
    <div class="topbar__time-pill">
      <div class="topbar__time-item">
        <i class="ri-server-line topbar__time-icon" aria-hidden="true"></i>
        <span class="topbar__time-label">{t('shell.server_clock')}</span>
        <span class="mono tabular topbar__time-val">{formatDate(serverNow)}, {formatTime(serverNow)}</span>
        <span class="topbar__tz-badge">{serverTzName}</span>
      </div>

      <span class="topbar__time-sep">|</span>

      <div class="topbar__time-item topbar__client-time">
        <i class="ri-user-location-line topbar__time-icon" aria-hidden="true"></i>
        <span class="topbar__time-label">{t('shell.client_clock')}</span>
        <span class="mono tabular topbar__time-val">{formatTime(clientNow)}</span>
        <span class="topbar__tz-badge topbar__tz-badge--highlight">{getClientTimezoneLabel()}</span>
      </div>
    </div>

    <!-- Switcher Cepat Bahasa ID / EN -->
    <button
      type="button"
      class="topbar__lang-btn mono"
      onclick={toggleLocale}
      title={i18nState.lang === 'id' ? 'Switch language to English' : 'Ganti bahasa ke Bahasa Indonesia'}
      aria-label="Toggle language"
    >
      {i18nState.lang.toUpperCase()}
    </button>

    <!-- Toggle Tema (Dark / Light / Auto) -->
    <button
      type="button"
      class="topbar__theme-btn"
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
        aria-label={user}
        title={user}
      >
        <i class="ri-user-3-line" aria-hidden="true"></i>
        <span class="topbar__user-name">{user}</span>
        <i class="ri-arrow-down-s-line topbar__user-caret" aria-hidden="true"></i>
      </button>

      {#if userMenuOpen}
        <div class="topbar__user-menu" role="menu" aria-label={t('shell.signed_in_as')}>
          <div class="topbar__user-info">
            <span class="topbar__user-role">{t('shell.signed_in_as')}</span>
            <strong class="mono">{user}</strong>
          </div>

          <!-- Informasi host, jam server, & bahasa khusus layar mobile agar topbar tetap bersih -->
          <div class="topbar__mobile-meta">
            <div class="topbar__mobile-row">
              <button
                type="button"
                class="topbar__mobile-ip-btn"
                onclick={copyIP}
                title={t('shell.copy_ip')}
              >
                <span class="dot dot--success" aria-hidden="true"></span>
                <span class="mono tabular">{maskIP ? '•••.•••.•••.•••' : ip}</span>
                <i class="ri-file-copy-line" aria-hidden="true"></i>
              </button>
              <button
                type="button"
                class="topbar__mobile-mask-btn"
                onclick={() => (maskIP = !maskIP)}
                aria-label={maskIP ? t('shell.show_ip') : t('shell.hide_ip')}
                title={maskIP ? t('shell.show_ip') : t('shell.hide_ip')}
                aria-pressed={maskIP}
              >
                <i class={maskIP ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
              </button>
            </div>

            <div class="topbar__mobile-info">
              <i class="ri-computer-line" aria-hidden="true"></i>
              <span class="truncate">{overview?.system?.os || 'Linux'}</span>
            </div>

            <div class="topbar__mobile-info">
              <i class="ri-server-line" aria-hidden="true"></i>
              <span class="mono tabular">{formatDate(serverNow)}, {formatTime(serverNow)}</span>
              <span class="topbar__tz-badge">{serverTzName}</span>
            </div>

            <button
              type="button"
              role="menuitem"
              class="topbar__mobile-lang"
              onclick={toggleLocale}
            >
              <i class="ri-translate-2" aria-hidden="true"></i>
              <span>{i18nState.lang === 'id' ? 'Bahasa: Indonesia (ID)' : 'Language: English (EN)'}</span>
              <span class="topbar__tz-badge mono">{i18nState.lang.toUpperCase()}</span>
            </button>
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
              <span>{t('shell.logout')}</span>
            </button>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</header>
