<script lang="ts">
  import { onMount, type Component } from 'svelte';
  import { apiFetch, UNAUTHORIZED_EVENT } from './lib/api';
  import type { MetricPoint, OverviewData, Service } from './lib/types';
  import { notify } from './lib/stores/toast.svelte';
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

  // Kembalikan komponen langsung bila sudah ada di cache agar tidak ada kedipan loading
  function loadRoute(id: string): Component<any> | Promise<Component<any>> {
    const cached = loaded.get(id);
    if (cached) return cached;
    return loaders[id]().then((m) => {
      loaded.set(id, m.default);
      return m.default;
    });
  }

  // State sesi
  let checking = $state(true);
  let authenticated = $state(false);
  let currentUser = $state('');

  // State navigasi & command palette
  let currentRoute = $state(routeFromHash(window.location.hash));
  let menuOpen = $state(false);
  let paletteOpen = $state(false);
  let routeAttempt = $state(0);

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
    document.title = `${item?.label ?? 'Dashboard'} · fikpanel`;
  });

  function onHashChange() {
    currentRoute = routeFromHash(window.location.hash);
    menuOpen = false;
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
    const onUnauthorized = () => {
      if (authenticated) endSession('Sesi berakhir, silakan masuk kembali', 'warning');
    };
    window.addEventListener('hashchange', onHashChange);
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized);

    apiFetch<{ username: string }>('/api/auth/me')
      .then((me) => enterPanel(me.username))
      .catch(() => (authenticated = false))
      .finally(() => (checking = false));

    return () => {
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
  <Login onSuccess={(u) => { enterPanel(u); notify(`Selamat datang, ${u}`, 'success'); }} />
{:else}
  <a href="#main-content" class="skip-link">Lewati ke konten</a>
  <div class="app-shell">
    <Sidebar
      current={currentRoute}
      open={menuOpen}
      version={overview?.system?.version}
      onNavigate={() => (menuOpen = false)}
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
        {#key `${currentRoute}:${routeAttempt}`}
          {#await loadRoute(currentRoute)}
            <div class="app-shell__loading" role="status">
              <div class="skeleton app-shell__sk-title"></div>
              <div class="skeleton app-shell__sk-card"></div>
              <div class="skeleton app-shell__sk-panel"></div>
              <span class="sr-only">Memuat halaman…</span>
            </div>
          {:then Page}
            <Page {...routeProps(currentRoute)} />
          {:catch err}
            <div class="empty-state" role="alert">
              <i class="ri-wifi-off-line" aria-hidden="true"></i>
              <p class="empty-state__title">Halaman gagal dimuat</p>
              <p>{err?.message ?? 'Periksa koneksi lalu coba lagi.'}</p>
              <button class="btn btn--primary" onclick={() => routeAttempt++}>
                <i class="ri-refresh-line" aria-hidden="true"></i> Coba lagi
              </button>
            </div>
          {/await}
        {/key}
      </main>
    </div>

    <CommandPalette
      open={paletteOpen}
      current={currentRoute}
      onClose={() => (paletteOpen = false)}
      onSelect={() => (menuOpen = false)}
    />
  </div>
{/if}

<ToastHost />
<ConfirmHost />
