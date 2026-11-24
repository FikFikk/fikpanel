<script lang="ts">
  import { onMount, type Component } from 'svelte';
  import { apiFetch, UNAUTHORIZED_EVENT } from './lib/api';
  import type { MetricPoint, OverviewData, Service } from './lib/types';
  import { notify } from './lib/stores/toast.svelte';
  import { applyAppearance, appearanceState, type PanelAppearance } from './lib/stores/theme.svelte';
  import { i18nState, setLocale, t, type Locale } from './lib/stores/i18n.svelte';
  import { NAV_ITEMS, routeFromHash } from './lib/layout/nav';

  import Login from './lib/layout/Login.svelte';
  import Sidebar from './lib/layout/Sidebar.svelte';
  import Topbar from './lib/layout/Topbar.svelte';
  import CommandPalette from './lib/layout/CommandPalette.svelte';
  import ToastHost from './lib/components/ui/ToastHost.svelte';
  import ConfirmHost from './lib/components/ui/ConfirmHost.svelte';
  import Home from './routes/Home.svelte';

  // Halaman selain dashboard dimuat terpisah (code splitting) agar login & dashboard cepat tampil
  const loaders: Record<string, () => Promise<{ default: Component<any> }>> = {
    website: () => import('./routes/Sites.svelte'),
    waf: () => import('./routes/WAF.svelte'),
    databases: () => import('./routes/Databases.svelte'),
    docker: () => import('./routes/Docker.svelte'),
    k8s: () => import('./routes/Kubernetes.svelte'),
    cron: () => import('./routes/Cron.svelte'),
    backup: () => import('./routes/Backup.svelte'),
    node: () => import('./routes/NodeManager.svelte'),
    appstore: () => import('./routes/AppStore.svelte'),
    dns: () => import('./routes/DNSManager.svelte'),
    php: () => import('./routes/PHPManager.svelte'),
    system: () => import('./routes/SystemManager.svelte'),
    services: () => import('./routes/Services.svelte'),
    terminal: () => import('./routes/Terminal.svelte'),
    monitor: () => import('./routes/Monitor.svelte'),
    files: () => import('./routes/Files.svelte'),
    security: () => import('./routes/Security.svelte'),
    assistant: () => import('./routes/Assistant.svelte'),
    settings: () => import('./routes/Settings.svelte'),
  };
  const loaded = new Map<string, Component<any>>([['home', Home as Component<any>]]);
  const inflight = new Map<string, Promise<Component<any>>>();

  // Muat chunk rute dengan deduplikasi promise dan simpan ke cache memori
  function fetchRouteComponent(id: string): Promise<Component<any>> {
    const cached = loaded.get(id);
    if (cached) return Promise.resolve(cached);
    const pending = inflight.get(id);
    if (pending) return pending;
    const loader = loaders[id];
    if (!loader) return Promise.resolve(Home as Component<any>);
    const p = loader()
      .then((m) => {
        loaded.set(id, m.default);
        inflight.delete(id);
        return m.default;
      })
      .catch((err) => {
        inflight.delete(id);
        throw err;
      });
    inflight.set(id, p);
    return p;
  }

  // Prefetch chunk saat pengguna mengarahkan kursor ke item sidebar agar klik langsung instan
  function prefetchRoute(id: string) {
    if (loaded.has(id) || inflight.has(id) || !loaders[id]) return;
    fetchRouteComponent(id).catch(() => {
      // Abaikan kegagalan prefetch latar belakang; akan dicoba ulang saat diklik
    });
  }

  // State sesi
  let checking = $state(true);
  let authenticated = $state(false);
  let currentUser = $state('');
  let passkeysAvailable = $state(false);

  // State navigasi & command palette (ActivePage diganti hanya setelah chunk siap agar bebas kedip)
  const initialRoute = routeFromHash(window.location.hash);
  let currentRoute = $state(initialRoute);
  let activeRoute = $state(initialRoute);
  let ActivePage = $state<Component<any> | null>(loaded.get(initialRoute) ?? null);
  let routeError = $state('');
  let routeLoading = $state(!loaded.has(initialRoute));
  let menuOpen = $state(false);
  let paletteOpen = $state(false);
  let routeAttempt = $state(0);
  let navToken = 0;
  let skeletonTimer: ReturnType<typeof setTimeout> | undefined;

  // State data global
  let overview = $state<OverviewData | null>(null);
  let services = $state<Service[]>([]);
  let history = $state<MetricPoint[]>([]);
  let currentMetric = $state<MetricPoint>({
    timestamp: 0, cpu: 0, ram_used: 0, ram_total: 0, disk_used: 0, disk_total: 0, net_in: 0, net_out: 0, load_avg: 0,
  });
  let sseConnected = $state(false);
  let refreshing = $state(false);
  let eventSource: EventSource | null = null;

  $effect(() => {
    const item = NAV_ITEMS.find((n) => n.id === currentRoute);
    const label = item ? t(`nav.item.${item.id}` as any) : t('nav.item.home');
    const brand = appearanceState.brand_name || 'fikpanel';
    document.title = `${label} · ${brand}`;
  });

  // Pindah rute secara bersih: pre-resolve chunk sebelum swap agar halaman lama tidak berkedip/ketimpa
  async function activateRoute(nextRoute: string, force = false) {
    if (!force && nextRoute === activeRoute && ActivePage && !routeError) {
      currentRoute = nextRoute;
      menuOpen = false;
      return;
    }
    currentRoute = nextRoute;
    menuOpen = false;
    routeError = '';
    clearTimeout(skeletonTimer);

    const cached = loaded.get(nextRoute);
    if (cached) {
      navToken++;
      ActivePage = cached;
      activeRoute = nextRoute;
      routeLoading = false;
      window.scrollTo({ top: 0 });
      return;
    }

    const token = ++navToken;
    // Tampilkan skeleton hanya jika belum ada halaman aktif atau unduhan chunk > 180ms
    if (!ActivePage) {
      routeLoading = true;
    } else {
      skeletonTimer = setTimeout(() => {
        if (token === navToken) routeLoading = true;
      }, 180);
    }

    try {
      const comp = await fetchRouteComponent(nextRoute);
      if (token !== navToken) return;
      clearTimeout(skeletonTimer);
      ActivePage = comp;
      activeRoute = nextRoute;
      routeLoading = false;
      window.scrollTo({ top: 0 });
    } catch (err) {
      if (token !== navToken) return;
      clearTimeout(skeletonTimer);
      routeLoading = false;
      routeError = (err as Error)?.message ?? 'Periksa koneksi lalu coba lagi.';
    }
  }

  function onHashChange() {
    const nextRoute = routeFromHash(window.location.hash);
    activateRoute(nextRoute);
  }

  function retryRoute() {
    routeAttempt++;
    activateRoute(currentRoute, true);
  }

  // Aliran metrik realtime (Server-Sent Events); browser menyambung ulang otomatis bila putus
  function startSSE() {
    eventSource?.close();
    eventSource = new EventSource('/api/metrics/live');
    eventSource.addEventListener('history', (e) => {
      try {
        history = (JSON.parse((e as MessageEvent).data) as MetricPoint[]).slice(-60);
      } catch (err) {
        console.warn('Data histori metrik tidak valid', err);
      }
    });
    eventSource.addEventListener('metric', (e) => {
      try {
        const pt: MetricPoint = JSON.parse((e as MessageEvent).data);
        currentMetric = pt;
        history = [...history.slice(-59), pt];
        sseConnected = true;
      } catch (err) {
        console.warn('Data metrik tidak valid', err);
      }
    });
    eventSource.onopen = () => (sseConnected = true);
    eventSource.onerror = () => (sseConnected = false);
  }

  async function loadInitialData() {
    refreshing = true;
    try {
      const [ov, svc] = await Promise.all([
        apiFetch<OverviewData>('/api/overview'),
        apiFetch<{ services: Service[] }>('/api/services'),
      ]);
      overview = ov;
      services = svc.services || [];
      if (ov.latest_metric) currentMetric = ov.latest_metric;
    } catch (err) {
      notify(`Gagal memuat data server: ${(err as Error).message}`, 'error');
    } finally {
      refreshing = false;
    }
  }

  async function enterPanel(username: string) {
    authenticated = true;
    currentUser = username;
    startSSE();
    await loadInitialData();
  }

  function endSession(message: string, type: 'info' | 'warning' = 'info') {
    eventSource?.close();
    eventSource = null;
    authenticated = false;
    paletteOpen = false;
    overview = null;
    services = [];
    history = [];
    notify(message, type);
  }

  async function handleLogout() {
    try {
      await apiFetch('/api/auth/logout', { method: 'POST' });
    } catch (err) {
      console.warn('Logout server gagal, sesi lokal tetap diakhiri', err);
    }
    endSession('Anda telah keluar');
  }

  async function handleSvcAction(unit: string, action: string) {
    try {
      await apiFetch(`/api/services/${encodeURIComponent(unit)}/action`, {
        method: 'POST',
        body: JSON.stringify({ action }),
      });
      notify(`Aksi ${action} pada ${unit} berhasil`, 'success');
      const svc = await apiFetch<{ services: Service[] }>('/api/services');
      services = svc.services || [];
    } catch (err) {
      notify(`Gagal ${action} ${unit}: ${(err as Error).message}`, 'error');
    }
  }

  // Props tiap halaman (dipertahankan kompatibel dengan kontrak komponen lama)
  function routeProps(id: string): Record<string, unknown> {
    switch (id) {
      case 'home':
        return { overview, currentMetric, history, services, onSvcAction: handleSvcAction };
      case 'services':
        return { services, onSvcAction: handleSvcAction, onRefresh: loadInitialData, onNotify: notify };
      case 'monitor':
        return { currentMetric, history };
      case 'assistant':
        return { onSvcAction: handleSvcAction, onNotify: notify };
      case 'settings':
        return { overview, onNotify: notify };
      case 'terminal':
      case 'security':
        return {};
      default:
        return { onNotify: notify };
    }
  }

  function onWindowKey(e: KeyboardEvent) {
    if (authenticated && (e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      paletteOpen = !paletteOpen;
      return;
    }
    if (e.key === 'Escape' && menuOpen) {
      menuOpen = false;
    }
  }

  onMount(() => {
    // Sinkronkan preferensi tampilan, branding, dan bahasa dari server sebelum/sesudah login
    apiFetch<Partial<PanelAppearance> & { language?: Locale; passkeys_available?: boolean }>('/api/settings/public')
      .then((pub) => {
        if (!pub) return;
        passkeysAvailable = Boolean(pub.passkeys_available);
        applyAppearance({
          theme_mode: pub.theme_mode,
          accent_preset: pub.accent_preset,
          accent_custom: pub.accent_custom,
          surface_style: pub.surface_style,
          density: pub.density,
          radius_style: pub.radius_style,
          brand_name: pub.brand_name,
          env_badge: pub.env_badge,
        });
        if (pub.language === 'id' || pub.language === 'en') {
          setLocale(pub.language);
        }
      })
      .catch(() => {
        // Tetap gunakan cache localStorage bila jaringan belum siap
      });

    const onUnauthorized = () => {
      if (authenticated) endSession(i18nState.lang === 'en' ? 'Session expired, please sign in again' : 'Sesi berakhir, silakan masuk kembali', 'warning');
    };
    window.addEventListener('hashchange', onHashChange);
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized);
    if (!ActivePage) {
      activateRoute(currentRoute, true);
    }

    apiFetch<{ username: string }>('/api/auth/me')
      .then((me) => enterPanel(me.username))
      .catch(() => (authenticated = false))
      .finally(() => (checking = false));

    return () => {
      clearTimeout(skeletonTimer);
      window.removeEventListener('hashchange', onHashChange);
      window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized);
      eventSource?.close();
    };
  });
</script>

<svelte:window onkeydown={onWindowKey} />

{#if checking}
  <div class="app-shell__boot" role="status" aria-live="polite">
    <span class="spinner" aria-hidden="true"></span>
    <span class="sr-only">Memeriksa sesi…</span>
  </div>
{:else if !authenticated}
  <Login {passkeysAvailable} onSuccess={(u) => { enterPanel(u); notify(`Selamat datang, ${u}`, 'success'); }} />
{:else}
  <a href="#main-content" class="skip-link">Lewati ke konten</a>
  <div class="app-shell">
    <Sidebar
      current={currentRoute}
      open={menuOpen}
      version={overview?.system?.version}
      onNavigate={() => (menuOpen = false)}
      onPrefetch={prefetchRoute}
      onLogout={handleLogout}
      onOpenPalette={() => (paletteOpen = true)}
    />
    {#if menuOpen}
      <div class="app-shell__scrim" role="presentation" onclick={() => (menuOpen = false)}></div>
    {/if}

    <div class="app-shell__main">
      <Topbar
        {overview}
        user={currentUser}
        {menuOpen}
        onToggleMenu={() => (menuOpen = !menuOpen)}
        onOpenPalette={() => (paletteOpen = true)}
        onLogout={handleLogout}
      />

      <main class="app-shell__content" id="main-content" tabindex="-1">
        {#if routeError}
          <div class="empty-state" role="alert">
            <i class="ri-wifi-off-line" aria-hidden="true"></i>
            <p class="empty-state__title">Halaman gagal dimuat</p>
            <p>{routeError}</p>
            <button class="btn btn--primary" onclick={retryRoute}>
              <i class="ri-refresh-line" aria-hidden="true"></i> Coba lagi
            </button>
          </div>
        {:else if routeLoading}
          <div class="app-shell__loading" role="status">
            <div class="skeleton app-shell__sk-title"></div>
            <div class="skeleton app-shell__sk-card"></div>
            <div class="skeleton app-shell__sk-panel"></div>
            <span class="sr-only">Memuat halaman…</span>
          </div>
        {:else if ActivePage}
          {@const Page = ActivePage}
          {#key `${activeRoute}:${routeAttempt}`}
            <div class="app-shell__view">
              <Page {...routeProps(activeRoute)} />
            </div>
          {/key}
        {/if}
      </main>
    </div>

    <CommandPalette
      open={paletteOpen}
      current={currentRoute}
      {services}
      onClose={() => (paletteOpen = false)}
      onSelect={() => (menuOpen = false)}
      onSvcAction={handleSvcAction}
      onRefresh={loadInitialData}
      onLogout={handleLogout}
    />
  </div>
{/if}

<ToastHost />
<ConfirmHost />
