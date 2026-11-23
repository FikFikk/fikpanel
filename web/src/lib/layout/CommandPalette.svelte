<script lang="ts">
  import { NAV_ITEMS, type NavItem } from './nav';
  import type { Service } from '../types';
  import { i18nState, setLocale, t } from '../stores/i18n.svelte';
  import { applyAppearance, appearanceState, setTheme, type AccentPreset } from '../stores/theme.svelte';
  import { confirmDialog } from '../stores/confirm.svelte';
  import { notify } from '../stores/toast.svelte';

  interface Props {
    open: boolean;
    current: string;
    services?: Service[];
    onClose: () => void;
    onSelect?: (id: string) => void;
    onSvcAction?: (unit: string, action: string) => void;
    onRefresh?: () => void;
    onLogout?: () => void;
  }

  let { open, current, services = [], onClose, onSelect, onSvcAction, onRefresh, onLogout }: Props = $props();

  let query = $state('');
  let activeIndex = $state(0);
  let inputEl = $state<HTMLInputElement | null>(null);

  // Kata kunci tambahan agar pencarian lintas bahasa & istilah teknis cepat ditemukan
  const ROUTE_KEYWORDS: Record<string, string> = {
    home: 'overview dashboard metrik cpu ram disk',
    website: 'sites domain vhost nginx ssl git deploy',
    php: 'fpm extensions runtime',
    appstore: 'install software packages',
    databases: 'mysql mariadb postgres redis mongodb phpmyadmin sql',
    files: 'file manager editor folder upload zip',
    docker: 'container compose image volume network',
    k8s: 'kubernetes k3s pod deployment',
    node: 'nodejs pm2 javascript',
    services: 'systemd daemon unit restart log',
    cron: 'schedule job crontab timer',
    terminal: 'ssh pty console shell bash',
    waf: 'firewall attack map traffic block',
    security: 'ufw port fail2ban ssh hardening',
    monitor: 'metrics chart history bandwidth',
    system: 'cleaner cache process port task',
    assistant: 'ai copilot llm chat',
    settings: 'theme palette 2fa totp tls ssl language backup export import update',
  };

  interface PaletteEntry {
    id: string;
    kind: 'nav' | 'action';
    label: string;
    group: string;
    icon: string;
    keywords: string;
    isCurrent?: boolean;
    swatchClass?: string;
    run: () => void;
  }

  const ACCENTS: Array<{ id: AccentPreset; name: string; swatchClass: string }> = [
    { id: 'lava', name: 'Lava', swatchClass: 'palette__swatch--lava' },
    { id: 'ocean', name: 'Ocean', swatchClass: 'palette__swatch--ocean' },
    { id: 'emerald', name: 'Emerald', swatchClass: 'palette__swatch--emerald' },
    { id: 'violet', name: 'Violet', swatchClass: 'palette__swatch--violet' },
    { id: 'crimson', name: 'Crimson', swatchClass: 'palette__swatch--crimson' },
    { id: 'amber', name: 'Amber', swatchClass: 'palette__swatch--amber' },
    { id: 'slate', name: 'Slate', swatchClass: 'palette__swatch--slate' },
  ];

  function labelForItem(item: NavItem): string {
    return t(`nav.item.${item.id}` as any);
  }
  function labelForGroup(groupId: string): string {
    return t(`nav.group.${groupId}` as any);
  }

  // Susun daftar gabungan navigasi halaman + aksi cepat (tema, palet, bahasa, layanan, refresh, logout)
  let entries = $derived.by<PaletteEntry[]>(() => {
    const navEntries: PaletteEntry[] = NAV_ITEMS.map((item) => ({
      id: `nav:${item.id}`,
      kind: 'nav',
      label: labelForItem(item),
      group: labelForGroup(item.group),
      icon: item.icon,
      keywords: `${item.label} ${item.id} ${ROUTE_KEYWORDS[item.id] || ''}`,
      isCurrent: item.id === current,
      run: () => {
        window.location.hash = `#/${item.id}`;
        onSelect?.(item.id);
      },
    }));

    const actionEntries: PaletteEntry[] = [
      {
        id: 'act:refresh',
        kind: 'action',
        label: t('shell.palette.act_refresh'),
        group: t('shell.palette.group_action'),
        icon: 'ri-refresh-line',
        keywords: 'refresh reload segarkan sync overview',
        run: () => {
          onRefresh?.();
          notify(t('shell.palette.act_refresh'), 'info');
        },
      },
      {
        id: 'act:lang',
        kind: 'action',
        label: t('shell.palette.act_lang'),
        group: t('shell.palette.group_action'),
        icon: 'ri-translate-2',
        keywords: 'language bahasa indonesia english id en i18n',
        run: () => {
          const next = i18nState.lang === 'id' ? 'en' : 'id';
          setLocale(next);
          notify(next === 'en' ? 'Language switched to English' : 'Bahasa diubah ke Bahasa Indonesia', 'success');
        },
      },
      {
        id: 'act:theme:dark',
        kind: 'action',
        label: t('shell.palette.act_theme_dark'),
        group: t('shell.palette.group_theme'),
        icon: 'ri-moon-line',
        keywords: 'theme tema dark gelap night',
        isCurrent: appearanceState.theme_mode === 'dark',
        run: () => setTheme('dark'),
      },
      {
        id: 'act:theme:light',
        kind: 'action',
        label: t('shell.palette.act_theme_light'),
        group: t('shell.palette.group_theme'),
        icon: 'ri-sun-line',
        keywords: 'theme tema light terang day',
        isCurrent: appearanceState.theme_mode === 'light',
        run: () => setTheme('light'),
      },
      {
        id: 'act:theme:auto',
        kind: 'action',
        label: t('shell.palette.act_theme_auto'),
        group: t('shell.palette.group_theme'),
        icon: 'ri-contrast-2-line',
        keywords: 'theme tema auto system otomatis',
        isCurrent: appearanceState.theme_mode === 'auto',
        run: () => setTheme('auto'),
      },
      ...ACCENTS.map<PaletteEntry>((acc) => ({
        id: `act:accent:${acc.id}`,
        kind: 'action',
        label: t('shell.palette.act_accent', { name: acc.name }),
        group: t('shell.palette.group_theme'),
        icon: 'ri-palette-line',
        swatchClass: acc.swatchClass,
        keywords: `palette warna color accent aksen ${acc.name} ${acc.id}`,
        isCurrent: appearanceState.accent_preset === acc.id,
        run: () => {
          applyAppearance({ accent_preset: acc.id });
          notify(t('shell.palette.act_accent', { name: acc.name }), 'success');
        },
      })),
    ];

    // Tambahkan aksi cepat restart untuk layanan sistem non-terproteksi yang aktif
    if (onSvcAction && services.length > 0) {
      const quickUnits = services.filter((s) => !s.protected).slice(0, 10);
      for (const svc of quickUnits) {
        actionEntries.push({
          id: `act:svc:restart:${svc.unit}`,
          kind: 'action',
          label: t('shell.palette.act_restart_svc', { unit: svc.label || svc.unit }),
          group: t('shell.palette.group_service'),
          icon: 'ri-restart-line',
          keywords: `restart service layanan ${svc.unit} ${svc.label || ''}`,
          run: () => {
            void confirmDialog({
              title: t('shell.palette.act_restart_svc', { unit: svc.unit }),
              message: `${svc.unit} (${svc.active_state})`,
              confirmText: 'Restart',
              tone: 'danger',
            }).then((ok) => {
              if (ok) onSvcAction?.(svc.unit, 'restart');
            });
          },
        });
      }
    }

    if (onLogout) {
      actionEntries.push({
        id: 'act:logout',
        kind: 'action',
        label: t('shell.palette.act_logout'),
        group: t('shell.palette.group_action'),
        icon: 'ri-logout-box-r-line',
        keywords: 'logout keluar sign out exit',
        run: () => onLogout?.(),
      });
    }

    return [...navEntries, ...actionEntries];
  });

  // Filter fuzzy ringan berdasarkan label, grup, dan kata kunci
  function matchQuery(entry: PaletteEntry, q: string): boolean {
    const clean = q.trim().toLowerCase();
    if (!clean) return true;
    const hay = `${entry.label} ${entry.group} ${entry.keywords}`.toLowerCase();
    if (hay.includes(clean)) return true;
    const parts = clean.split(/\s+/).filter(Boolean);
    if (parts.length > 1 && parts.every((p) => hay.includes(p))) return true;
    let qi = 0;
    for (let i = 0; i < hay.length && qi < clean.length; i++) {
      if (hay[i] === clean[qi]) qi++;
    }
    return qi === clean.length;
  }

  let filtered = $derived(entries.filter((e) => matchQuery(e, query)));

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

  function execute(entry: PaletteEntry) {
    onClose();
    entry.run();
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
      execute(filtered[activeIndex]);
    }
  }
</script>

{#if open}
  <div class="palette" role="dialog" aria-modal="true" aria-label={t('shell.search_menu')} tabindex="-1" onkeydown={handleKeydown}>
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
          {#each filtered as entry, idx (entry.id)}
            <button
              type="button"
              role="option"
              aria-selected={idx === activeIndex}
              class="palette__item"
              class:is-active={idx === activeIndex}
              class:is-current={entry.isCurrent}
              onmouseenter={() => (activeIndex = idx)}
              onclick={() => execute(entry)}
            >
              {#if entry.swatchClass}
                <span class={`palette__swatch ${entry.swatchClass}`} aria-hidden="true"></span>
              {:else}
                <i class="{entry.icon} palette__item-icon" aria-hidden="true"></i>
              {/if}
              <span class="palette__item-label">{entry.label}</span>
              {#if entry.isCurrent}
                <span class="badge badge--success">{t('shell.palette.active')}</span>
              {/if}
              <span class="palette__item-group">{entry.group}</span>
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
