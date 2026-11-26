<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type {
    AppPackage,
    StackBundle,
    InstallTask,
    GlobalPackage,
    UpgradablePackage,
    PackageDetail,
    DockerAppTemplate,
    DockerStatus,
  } from '../lib/types';
  import PageHeader from '../lib/components/ui/PageHeader.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import BundlesTab from './appstore/BundlesTab.svelte';
  import DockerAppsTab from './appstore/DockerAppsTab.svelte';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import '../styles/pages/appstore.css';

  // Tab utama App Store
  type StoreTab = 'catalog' | 'bundles' | 'docker_apps' | 'apt_queue';
  let activeTab = $state<StoreTab>('catalog');

  // Filter kategori katalog server
  type CategoryFilter = 'all' | 'installed' | 'updates' | 'webserver' | 'runtime' | 'database' | 'tools';
  let activeCategory = $state<CategoryFilter>('all');

  // Mode tampilan: Kartu (Grid) atau Tabel (Table)
  type ViewMode = 'grid' | 'table';
  let viewMode = $state<ViewMode>('grid');

  // State data utama
  let apps = $state<AppPackage[]>([]);
  let bundles = $state<StackBundle[]>([]);
  let tasks = $state<InstallTask[]>([]);
  let upgradable = $state<UpgradablePackage[]>([]);
  let dockerTemplates = $state<DockerAppTemplate[]>([]);
  let dockerStatus = $state<DockerStatus | null>(null);

  let loading = $state(true);
  let loadingDocker = $state(false);
  let syncingApt = $state(false);
  let busyServiceId = $state('');

  // State pencarian fuzzy & autocomplete
  let searchQuery = $state('');
  let searchFocused = $state(false);
  let searchSectionEl = $state<HTMLElement | null>(null);
  let searchHistory = $state<string[]>([]);
  let searchResults = $state<GlobalPackage[]>([]);
  let searching = $state(false);
  let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;

  // Modal Log Operasi APT Live
  let showLogModal = $state(false);
  let logModalTitle = $state('');
  let currentTask = $state<InstallTask | null>(null);
  let logTerminalEl = $state<HTMLElement | null>(null);
  let pollingInterval: ReturnType<typeof setInterval> | null = null;
  let bgTasksInterval: ReturnType<typeof setInterval> | null = null;

  // Modal Inspektor Detail Paket
  let showInspectModal = $state(false);
  let inspectLoading = $state(false);
  let inspectedApp = $state<AppPackage | null>(null);
  let inspectedDetail = $state<PackageDetail | null>(null);

  // Modal Deploy Aplikasi Docker 1-Klik
  let showDeployModal = $state(false);
  let deployTemplate = $state<DockerAppTemplate | null>(null);
  let deployProjectName = $state('');
  let deployPorts = $state<Record<string, string>>({});
  let deployEnv = $state<Record<string, string>>({});
  let deployingDocker = $state(false);

  // Daftar rekomendasi pencarian cepat
  const trendingTags = [
    { name: 'Nginx', icon: 'ri-server-line', query: 'nginx' },
    { name: 'PHP 8.4', icon: 'ri-code-s-slash-line', query: 'php8.4' },
    { name: 'Go', icon: 'ri-braces-line', query: 'golang' },
    { name: 'Cloudflared', icon: 'ri-cloud-line', query: 'cloudflared' },
    { name: 'Redis', icon: 'ri-flashlight-line', query: 'redis' },
    { name: 'Docker', icon: 'ri-box-3-line', query: 'docker' },
    { name: 'Python 3', icon: 'ri-code-box-line', query: 'python3' },
    { name: 'htop', icon: 'ri-pulse-line', query: 'htop' },
    { name: 'ffmpeg', icon: 'ri-film-line', query: 'ffmpeg' },
    { name: 'rclone', icon: 'ri-upload-cloud-2-line', query: 'rclone' },
  ];

  // Tutup popover autocomplete ketika klik di luar area kotak pencarian
  function handleWindowClick(e: MouseEvent) {
    if (searchFocused && searchSectionEl && !searchSectionEl.contains(e.target as Node)) {
      searchFocused = false;
    }
  }

  function loadHistory() {
    try {
      const saved = localStorage.getItem('fikpanel_appstore_history');
      if (saved) searchHistory = JSON.parse(saved);
    } catch {
      searchHistory = [];
    }
  }

  function saveSearchQuery(q: string) {
    const trimmed = q.trim();
    if (!trimmed || trimmed.length < 2) return;
    const next = [trimmed, ...searchHistory.filter((item) => item.toLowerCase() !== trimmed.toLowerCase())].slice(0, 8);
    searchHistory = next;
    try {
      localStorage.setItem('fikpanel_appstore_history', JSON.stringify(next));
    } catch {
      // Abaikan bila localStorage tidak tersedia
    }
  }

  function clearHistory() {
    searchHistory = [];
    try {
      localStorage.removeItem('fikpanel_appstore_history');
    } catch {
      // Abaikan
    }
  }

  function loadViewPreference() {
    try {
      const pref = localStorage.getItem('fikpanel_appstore_view_mode') as ViewMode | null;
      if (pref === 'grid' || pref === 'table') viewMode = pref;
    } catch {
      viewMode = 'grid';
    }
  }

  function setViewMode(mode: ViewMode) {
    viewMode = mode;
    try {
      localStorage.setItem('fikpanel_appstore_view_mode', mode);
    } catch {
      // Abaikan
    }
  }

  // Muat katalog server, bundel, tugas, dan paket upgradable secara paralel
  async function loadCatalog(forceRefresh = false) {
    try {
      loading = true;
      const qs = forceRefresh ? '?refresh=1' : '';
      const [catRes, bunRes, taskRes, upRes] = await Promise.all([
        apiFetch<{ ok: boolean; apps: AppPackage[] }>(`/api/appstore/catalog${qs}`),
        apiFetch<{ ok: boolean; bundles: StackBundle[] }>('/api/appstore/bundles').catch(() => null),
        apiFetch<{ ok: boolean; tasks: InstallTask[] }>('/api/appstore/tasks').catch(() => null),
        apiFetch<{ ok: boolean; packages: UpgradablePackage[] }>(`/api/appstore/upgradable${qs}`).catch(() => null),
      ]);
      apps = catRes?.apps || [];
      if (bunRes?.bundles) bundles = bunRes.bundles;
      if (taskRes?.tasks) tasks = taskRes.tasks;
      if (upRes?.packages) upgradable = upRes.packages;
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memuat katalog App Store', 'error');
    } finally {
      loading = false;
    }
  }

  // Muat daftar template & status Docker 1-Klik saat tab dibuka
  async function loadDockerApps() {
    try {
      loadingDocker = true;
      const [stRes, tplRes] = await Promise.all([
        apiFetch<{ installed?: boolean; running?: boolean } & DockerStatus>('/api/docker/status').catch(() => null),
        apiFetch<{ ok: boolean; templates: DockerAppTemplate[] }>('/api/docker/templates').catch(() => null),
      ]);
      if (stRes) dockerStatus = stRes;
      if (tplRes?.templates) dockerTemplates = tplRes.templates;
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memuat katalog Docker 1-klik', 'error');
    } finally {
      loadingDocker = false;
    }
  }

  // Sinkronisasi indeks APT nyata (apt-get update)
  async function runAptUpdate() {
    try {
      syncingApt = true;
      await apiFetch('/api/appstore/apt-update', { method: 'POST' });
      notify('Sinkronisasi indeks repositori APT dimulai', 'info');
      viewLogs('sys:apt-update', 'Sinkronisasi Repositori APT (apt-get update)');
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memulai apt-get update', 'error');
    } finally {
      syncingApt = false;
    }
  }

  // Pasang paket aplikasi katalog
  async function installApp(app: AppPackage) {
    const ok = await confirmDialog({
      title: `Pasang ${app.name}`,
      message: `Pasang "${app.name}" beserta dependensi APT (${app.apt_packages.join(', ')}) ke server?`,
      confirmText: 'Mulai Pasang',
      tone: 'default',
    });
    if (!ok) return;

    try {
      notify(`Memulai instalasi ${app.name}...`, 'info');
      await apiFetch(`/api/appstore/install/${encodeURIComponent(app.id)}`, { method: 'POST' });
      viewLogs(app.id, `Instalasi: ${app.name}`);
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memulai instalasi', 'error');
    }
  }

  // Upgrade 1-klik paket katalog atau paket APT global
  async function upgradeApp(appID: string, label: string, candidate?: string) {
    const ok = await confirmDialog({
      title: `Upgrade ${label}`,
      message: `Perbarui "${label}"${candidate ? ` ke versi ${candidate}` : ''} menggunakan konfigurasi aman non-interaktif (--force-confold)?`,
      confirmText: 'Mulai Upgrade',
      tone: 'default',
    });
    if (!ok) return;

    try {
      notify(`Menjadwalkan upgrade ${label}...`, 'info');
      const res = await apiFetch<{ ok: boolean; task_id?: string }>(`/api/appstore/upgrade/${encodeURIComponent(appID)}`, { method: 'POST' });
      viewLogs(res?.task_id || `upgrade:${appID}`, `Upgrade: ${label}`);
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memulai upgrade', 'error');
    }
  }

  // Copot paket aplikasi katalog (peringatan ekstra bila paket kritis)
  async function uninstallApp(app: AppPackage) {
    const extraWarn = app.critical
      ? ' PERINGATAN: Paket ini merupakan komponen inti server dan dapat memengaruhi situs aktif atau firewall!'
      : '';
    const ok = await confirmDialog({
      title: `Copot ${app.name}`,
      message: `Apakah Anda yakin ingin mencopot "${app.name}" dari server?${extraWarn}`,
      confirmText: 'Copot Paket',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      notify(`Memulai pencopotan ${app.name}...`, 'info');
      await apiFetch(`/api/appstore/uninstall/${encodeURIComponent(app.id)}`, { method: 'POST' });
      viewLogs(app.id, `Pencopotan: ${app.name}`);
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memulai pencopotan', 'error');
    }
  }

  // Kontrol daemon systemd langsung dari kartu / modal inspektor
  async function controlAppService(app: AppPackage, action: 'start' | 'stop' | 'restart') {
    if (action === 'stop' && app.critical) {
      const ok = await confirmDialog({
        title: `Hentikan Layanan ${app.name}`,
        message: `Menghentikan ${app.name} dapat memutus layanan aktif pada server. Lanjutkan?`,
        confirmText: 'Hentikan',
        tone: 'danger',
      });
      if (!ok) return;
    }

    try {
      busyServiceId = app.id;
      const res = await apiFetch<{ ok: boolean; message?: string }>(
        `/api/appstore/service/${encodeURIComponent(app.id)}/${action}`,
        { method: 'POST' }
      );
      notify(res?.message || `Layanan ${app.name} berhasil di-${action}`, 'success');
      await loadCatalog(true);
    } catch (err: unknown) {
      notify((err as Error).message || `Gagal ${action} layanan ${app.name}`, 'error');
    } finally {
      busyServiceId = '';
    }
  }

  // Pasang komponen kurang pada Stack Bundle 1-klik
  async function installBundle(bundle: StackBundle) {
    const ok = await confirmDialog({
      title: `Pasang Bundel ${bundle.name}`,
      message: `Memasang ${bundle.missing_app_ids.length} komponen yang belum tersedia (${bundle.missing_app_ids.join(', ')}) dalam satu antrean APT aman. Lanjutkan?`,
      confirmText: 'Pasang Bundel',
      tone: 'default',
    });
    if (!ok) return;

    try {
      const res = await apiFetch<{ ok: boolean; task_id?: string }>(
        `/api/appstore/bundles/${encodeURIComponent(bundle.id)}/install`,
        { method: 'POST' }
      );
      notify(`Instalasi bundel ${bundle.name} dimulai`, 'info');
      viewLogs(res?.task_id || `bundle:${bundle.id}`, `Stack Bundle: ${bundle.name}`);
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memulai instalasi bundel', 'error');
    }
  }

  // Buka inspektor metadata paket mendalam (apt-cache show + dpkg)
  async function openInspectModal(pkgName: string, app: AppPackage | null = null) {
    inspectedApp = app;
    inspectedDetail = null;
    showInspectModal = true;
    inspectLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; detail: PackageDetail }>(
        `/api/appstore/inspect/${encodeURIComponent(pkgName)}`
      );
      inspectedDetail = res?.detail || null;
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal membaca detail paket APT', 'error');
    } finally {
      inspectLoading = false;
    }
  }

  // Buka modal deploy Docker 1-klik
  function openDeployModal(tpl: DockerAppTemplate) {
    deployTemplate = tpl;
    deployProjectName = tpl.id;
    const ports: Record<string, string> = {};
    for (const p of tpl.default_ports || []) {
      ports[p] = p;
    }
    deployPorts = ports;
    deployEnv = { ...(tpl.default_env || {}) };
    showDeployModal = true;
  }

  // Jalankan deployment template Docker Compose 1-klik
  async function submitDeployTemplate() {
    if (!deployTemplate) return;
    try {
      deployingDocker = true;
      await apiFetch('/api/docker/templates/deploy', {
        method: 'POST',
        body: JSON.stringify({
          template_id: deployTemplate.id,
          project_name: deployProjectName.trim() || deployTemplate.id,
          ports: deployPorts,
          env: deployEnv,
        }),
      });
      notify(`Aplikasi ${deployTemplate.name} berhasil dideploy via Docker Compose!`, 'success');
      showDeployModal = false;
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal mendeploy template Docker', 'error');
    } finally {
      deployingDocker = false;
    }
  }

  // Pencarian paket sistem Linux global (APT batch cepat)
  async function searchGlobalPackages(query: string) {
    const q = query.trim();
    if (!q || q.length < 2) {
      searchResults = [];
      searching = false;
      return;
    }
    try {
      searching = true;
      const res = await apiFetch<{ ok: boolean; packages: GlobalPackage[] }>(
        `/api/appstore/search?q=${encodeURIComponent(q)}`
      );
      searchResults = res?.packages || [];
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal mencari paket OS', 'error');
    } finally {
      searching = false;
    }
  }

  async function installGlobalPackage(pkg: GlobalPackage) {
    const ok = await confirmDialog({
      title: `Pasang Paket ${pkg.name}`,
      message: `Pasang paket Linux "${pkg.name}" dari repositori sistem (apt-get)?`,
      confirmText: 'Pasang Sekarang',
      tone: 'default',
    });
    if (!ok) return;

    try {
      notify(`Memulai instalasi paket ${pkg.name}...`, 'info');
      await apiFetch(`/api/appstore/install/${encodeURIComponent(`pkg:${pkg.name}`)}`, { method: 'POST' });
      viewLogs(`pkg:${pkg.name}`, `Instalasi Paket APT: ${pkg.name}`);
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memulai instalasi', 'error');
    }
  }

  async function uninstallGlobalPackage(pkg: GlobalPackage) {
    const ok = await confirmDialog({
      title: `Copot Paket ${pkg.name}`,
      message: `Hapus paket "${pkg.name}" dari server via apt-get remove?`,
      confirmText: 'Copot Paket',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      notify(`Memulai pencopotan paket ${pkg.name}...`, 'info');
      await apiFetch(`/api/appstore/uninstall/${encodeURIComponent(`pkg:${pkg.name}`)}`, { method: 'POST' });
      viewLogs(`pkg:${pkg.name}`, `Pencopotan Paket APT: ${pkg.name}`);
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memulai pencopotan', 'error');
    }
  }

  function handleSearchInput(e: Event) {
    const val = (e.target as HTMLInputElement).value;
    searchQuery = val;
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
    searchDebounceTimer = setTimeout(() => {
      if (val.trim().length >= 2) {
        saveSearchQuery(val);
        searchGlobalPackages(val);
      } else {
        searchResults = [];
      }
    }, 400);
  }

  function selectTag(term: string) {
    searchQuery = term;
    searchFocused = false;
    saveSearchQuery(term);
    searchGlobalPackages(term);
  }

  function clearSearchInput() {
    searchQuery = '';
    searchResults = [];
    searchFocused = false;
  }

  // Tampilkan modal terminal streaming live
  function viewLogs(taskID: string, title: string) {
    logModalTitle = title;
    showLogModal = true;
    pollTask(taskID);
  }

  async function pollTask(taskID: string) {
    if (pollingInterval) clearInterval(pollingInterval);
    const fetchStatus = async () => {
      try {
        const res = await apiFetch<{ ok: boolean; task: InstallTask | null }>(
          `/api/appstore/tasks/${encodeURIComponent(taskID)}`
        );
        currentTask = res?.task || null;
        if (logTerminalEl) {
          logTerminalEl.scrollTop = logTerminalEl.scrollHeight;
        }
        if (currentTask && currentTask.status !== 'running' && currentTask.status !== 'queued') {
          clearInterval(pollingInterval!);
          loadCatalog(true);
          if (searchQuery.trim().length >= 2) {
            searchGlobalPackages(searchQuery);
          }
        }
      } catch {
        // Abaikan kegagalan polling sementara
      }
    };
    await fetchStatus();
    pollingInterval = setInterval(fetchStatus, 1500);
  }

  function closeLogModal() {
    showLogModal = false;
    if (pollingInterval) clearInterval(pollingInterval);
  }

  async function copyTaskLogs() {
    if (!currentTask?.logs) return;
    try {
      await navigator.clipboard.writeText(currentTask.logs);
      notify('Log terminal berhasil disalin', 'success');
    } catch {
      notify('Gagal menyalin ke clipboard', 'error');
    }
  }

  function navigateToManager(route?: string) {
    if (!route) return;
    window.location.hash = route;
  }

  function getCategoryLabel(cat: string): string {
    switch (cat) {
      case 'webserver': return 'Web Server';
      case 'runtime': return 'Runtime';
      case 'database': return 'Database & Cache';
      case 'tools': return 'Alat & Keamanan';
      default: return cat;
    }
  }

  function formatSizeKB(kb?: number): string {
    if (!kb || kb <= 0) return '';
    if (kb >= 1024 * 1024) return `${(kb / (1024 * 1024)).toFixed(1)} GB`;
    if (kb >= 1024) return `${(kb / 1024).toFixed(1)} MB`;
    return `${kb} KB`;
  }

  function formatTime(ts?: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleTimeString('id-ID', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
  }

  // Metrik KPI & filter reaktif
  let installedCount = $derived(apps.filter((a) => a.installed).length);
  let runningCount = $derived(apps.filter((a) => a.status === 'running').length);
  let catalogUpdatesCount = $derived(apps.filter((a) => a.update_available).length);
  let activeTasks = $derived(tasks.filter((t) => t.status === 'running' || t.status === 'queued'));

  let filteredApps = $derived(
    apps.filter((app) => {
      if (activeCategory === 'installed' && !app.installed) return false;
      if (activeCategory === 'updates' && !app.update_available) return false;
      if (
        activeCategory !== 'all' &&
        activeCategory !== 'installed' &&
        activeCategory !== 'updates' &&
        app.category !== activeCategory
      ) {
        return false;
      }
      const q = searchQuery.trim().toLowerCase();
      if (!q) return true;
      return (
        app.name.toLowerCase().includes(q) ||
        app.id.toLowerCase().includes(q) ||
        app.description.toLowerCase().includes(q) ||
        app.category.toLowerCase().includes(q)
      );
    })
  );

  onMount(() => {
    loadViewPreference();
    loadHistory();
    loadCatalog();

    // Sinkronkan antrean tugas secara ringan setiap 5 detik saat ada tugas aktif
    bgTasksInterval = setInterval(async () => {
      if (activeTasks.length === 0) return;
      try {
        const res = await apiFetch<{ ok: boolean; tasks: InstallTask[] }>('/api/appstore/tasks');
        if (res?.tasks) tasks = res.tasks;
      } catch {
        // Abaikan galat polling latar belakang
      }
    }, 5000);
  });

  onDestroy(() => {
    if (pollingInterval) clearInterval(pollingInterval);
    if (bgTasksInterval) clearInterval(bgTasksInterval);
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
  });
</script>

<svelte:window onclick={handleWindowClick} />

<div class="pg-appstore store-page">
  <!-- Header Standar Halaman App Store -->
  <PageHeader
    title="App Store & Paket Server"
    subtitle="Manajemen software server dengan deteksi versi nyata, Stack Bundles 1-klik, aplikasi Docker self-hosted, dan repositori APT."
    icon="ri-apps-2-line"
  >
    {#snippet actions()}
      <button
        type="button"
        class="btn btn--secondary btn--sm"
        onclick={runAptUpdate}
        disabled={syncingApt}
        title="Jalankan apt-get update untuk menyegarkan daftar versi & update repositori OS"
      >
        <i class="ri-download-cloud-line {syncingApt ? 'spin' : ''}"></i>
        Sync APT
      </button>
      <button
        type="button"
        class="btn btn--secondary btn--sm"
        onclick={() => loadCatalog(true)}
        disabled={loading}
        title="Pindai ulang versi aktual dan status layanan server"
      >
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i>
        Perbarui Daftar
      </button>
    {/snippet}
  </PageHeader>

  <!-- 4 Kartu KPI Ringkas -->
  <div class="as-kpi-grid">
    <div class="as-kpi-card">
      <div class="as-kpi-head">
        <span class="as-kpi-label">Katalog Server &amp; Bundel</span>
        <i class="ri-apps-2-line as-kpi-icon text-accent"></i>
      </div>
      <div class="as-kpi-val">{apps.length}</div>
      <div class="as-kpi-sub">{bundles.length} Stack Bundles · 12 Template Docker</div>
    </div>

    <div class="as-kpi-card is-accent">
      <div class="as-kpi-head">
        <span class="as-kpi-label">Terpasang &amp; Berjalan</span>
        <i class="ri-checkbox-circle-line as-kpi-icon text-accent"></i>
      </div>
      <div class="as-kpi-val text-accent">{installedCount}</div>
      <div class="as-kpi-sub">{runningCount} daemon systemd sedang aktif berjalan</div>
    </div>

    <div class="as-kpi-card {catalogUpdatesCount > 0 || upgradable.length > 0 ? 'is-warn' : ''}">
      <div class="as-kpi-head">
        <span class="as-kpi-label">Pembaruan APT Tersedia</span>
        <i class="ri-arrow-up-circle-line as-kpi-icon text-warning"></i>
      </div>
      <div class="as-kpi-val {upgradable.length > 0 ? 'text-warning' : ''}">{upgradable.length}</div>
      <div class="as-kpi-sub">{catalogUpdatesCount} di antaranya software katalog utama</div>
    </div>

    <div class="as-kpi-card">
      <div class="as-kpi-head">
        <span class="as-kpi-label">Antrean &amp; Tugas APT</span>
        <i class="ri-terminal-box-line as-kpi-icon text-info"></i>
      </div>
      <div class="as-kpi-val">{activeTasks.length}</div>
      <div class="as-kpi-sub">{tasks.length} total riwayat operasi sesi ini</div>
    </div>
  </div>

  <!-- Strip Tugas Aktif bila Ada Operasi APT Sedang Berjalan -->
  {#if activeTasks.length > 0}
    {@const firstTask = activeTasks[0]}
    <div class="active-task-strip">
      <div class="active-task-left">
        <span class="spinner" aria-hidden="true"></span>
        <span>
          <strong>{firstTask.title || firstTask.app_id}</strong> sedang {firstTask.status === 'queued' ? 'menunggu antrean APT' : 'diproses secara live'}...
        </span>
      </div>
      <button
        type="button"
        class="btn btn--primary btn--sm"
        onclick={() => viewLogs(firstTask.app_id, firstTask.title || firstTask.app_id)}
      >
        <i class="ri-terminal-line"></i> Buka Terminal Live
      </button>
    </div>
  {/if}

  <!-- Navigasi 4 Tab Utama -->
  <div class="tabs as-main-tabs" role="tablist" aria-label="Menu App Store">
    <button
      type="button"
      role="tab"
      aria-selected={activeTab === 'catalog'}
      class="tabs__tab"
      class:is-active={activeTab === 'catalog'}
      onclick={() => (activeTab = 'catalog')}
    >
      <i class="ri-server-line"></i> Katalog Server
      <span class="as-tab-count">{apps.length}</span>
    </button>
    <button
      type="button"
      role="tab"
      aria-selected={activeTab === 'bundles'}
      class="tabs__tab"
      class:is-active={activeTab === 'bundles'}
      onclick={() => (activeTab = 'bundles')}
    >
      <i class="ri-stack-line"></i> Stack Bundles 1-Klik
      <span class="as-tab-count">{bundles.length}</span>
    </button>
    <button
      type="button"
      role="tab"
      aria-selected={activeTab === 'docker_apps'}
      class="tabs__tab"
      class:is-active={activeTab === 'docker_apps'}
      onclick={() => { activeTab = 'docker_apps'; if (dockerTemplates.length === 0) loadDockerApps(); }}
    >
      <i class="ri-box-3-line"></i> Aplikasi Docker 1-Klik
      <span class="as-tab-count">{dockerTemplates.length || 12}</span>
    </button>
    <button
      type="button"
      role="tab"
      aria-selected={activeTab === 'apt_queue'}
      class="tabs__tab"
      class:is-active={activeTab === 'apt_queue'}
      onclick={() => (activeTab = 'apt_queue')}
    >
      <i class="ri-terminal-box-line"></i> Repositori APT &amp; Tugas
      {#if upgradable.length > 0}
        <span class="as-tab-count as-tab-count--warn">{upgradable.length} update</span>
      {/if}
    </button>
  </div>

  <!-- TAB 1: KATALOG SERVER -->
  {#if activeTab === 'catalog'}
    <div class="search-section" bind:this={searchSectionEl}>
      <div class="search-box card">
        <div class="input-wrap">
          <i class="ri-search-line input__icon" aria-hidden="true"></i>
          <input
            type="text"
            value={searchQuery}
            oninput={handleSearchInput}
            onfocus={() => (searchFocused = true)}
            placeholder="Cari software katalog atau paket Linux APT (misal: Nginx, PHP, Go, Cloudflared, htop, ffmpeg)..."
            class="input input--search search-field"
            aria-label="Cari aplikasi dan paket"
          />
          {#if searching}
            <span class="spinner search-spin" aria-label="Mencari"></span>
          {:else if searchQuery}
            <button type="button" class="btn btn--icon btn--sm btn--ghost btn-clear-search" onclick={clearSearchInput} aria-label="Bersihkan pencarian" title="Bersihkan pencarian">
              <i class="ri-close-line"></i>
            </button>
          {/if}
        </div>

        {#if searchFocused && !searchQuery}
          <div class="search-popover">
            {#if searchHistory.length > 0}
              <div class="popover-group">
                <div class="popover-title">
                  <span><i class="ri-history-line"></i> Riwayat Pencarian</span>
                  <button type="button" class="btn-clear-history" onclick={clearHistory}>Hapus Semua</button>
                </div>
                <div class="popover-chips">
                  {#each searchHistory as term}
                    <button type="button" class="chip-btn" onclick={() => selectTag(term)}>{term}</button>
                  {/each}
                </div>
              </div>
            {/if}

            <div class="popover-group">
              <div class="popover-title">
                <span><i class="ri-fire-line text-warning"></i> Populer &amp; Esensial</span>
              </div>
              <div class="popover-chips">
                {#each trendingTags as item}
                  <button type="button" class="chip-btn" onclick={() => selectTag(item.query)}>
                    <i class={item.icon}></i> {item.name}
                  </button>
                {/each}
              </div>
            </div>
          </div>
        {/if}
      </div>
    </div>

    <div class="toolbar-row">
      <div class="tabs tabs--pill" role="tablist" aria-label="Filter kategori software">
        <button type="button" class="tabs__tab" class:is-active={activeCategory === 'all'} onclick={() => (activeCategory = 'all')} role="tab" aria-selected={activeCategory === 'all'}>
          Semua ({apps.length})
        </button>
        <button type="button" class="tabs__tab" class:is-active={activeCategory === 'installed'} onclick={() => (activeCategory = 'installed')} role="tab" aria-selected={activeCategory === 'installed'}>
          <span class="dot dot--success"></span> Terpasang ({installedCount})
        </button>
        {#if catalogUpdatesCount > 0}
          <button type="button" class="tabs__tab" class:is-active={activeCategory === 'updates'} onclick={() => (activeCategory = 'updates')} role="tab" aria-selected={activeCategory === 'updates'}>
            <i class="ri-arrow-up-circle-line text-warning"></i> Update ({catalogUpdatesCount})
          </button>
        {/if}
        <button type="button" class="tabs__tab" class:is-active={activeCategory === 'webserver'} onclick={() => (activeCategory = 'webserver')} role="tab" aria-selected={activeCategory === 'webserver'}>
          Web Server
        </button>
        <button type="button" class="tabs__tab" class:is-active={activeCategory === 'runtime'} onclick={() => (activeCategory = 'runtime')} role="tab" aria-selected={activeCategory === 'runtime'}>
          Runtime &amp; Bahasa
        </button>
        <button type="button" class="tabs__tab" class:is-active={activeCategory === 'database'} onclick={() => (activeCategory = 'database')} role="tab" aria-selected={activeCategory === 'database'}>
          Database &amp; Cache
        </button>
        <button type="button" class="tabs__tab" class:is-active={activeCategory === 'tools'} onclick={() => (activeCategory = 'tools')} role="tab" aria-selected={activeCategory === 'tools'}>
          Alat &amp; Keamanan
        </button>
      </div>

      <div class="view-toggle segmented" role="radiogroup" aria-label="Pilih mode tampilan">
        <button type="button" class={viewMode === 'grid' ? 'active' : ''} onclick={() => setViewMode('grid')} aria-pressed={viewMode === 'grid'} aria-label="Tampilan Kartu" title="Tampilan Kartu">
          <i class="ri-layout-grid-line"></i>
        </button>
        <button type="button" class={viewMode === 'table' ? 'active' : ''} onclick={() => setViewMode('table')} aria-pressed={viewMode === 'table'} aria-label="Tampilan Tabel" title="Tampilan Tabel">
          <i class="ri-list-check-2"></i>
        </button>
      </div>
    </div>

    {#if loading && apps.length === 0}
      <div class="state state--loading">
        <span class="spinner" aria-hidden="true"></span>
        <p class="state__title">Memindai versi aktual dan layanan paket server...</p>
      </div>
    {:else if filteredApps.length === 0 && searchResults.length === 0}
      <div class="state state--empty card">
        <i class="ri-inbox-line" aria-hidden="true"></i>
        <p class="state__title">Tidak ditemukan aplikasi yang cocok</p>
        <p class="state__desc">
          {searchQuery ? `Tidak ada hasil untuk "${searchQuery}".` : 'Belum ada software dalam filter ini.'}
        </p>
        {#if searchQuery}
          <button type="button" class="btn btn--secondary btn--sm" onclick={clearSearchInput}>
            <i class="ri-arrow-go-back-line"></i> Bersihkan Pencarian
          </button>
        {/if}
      </div>
    {:else}
      {#if viewMode === 'grid'}
        <div class="apps-grid">
          {#each filteredApps as app (app.id)}
            <article class="app-card card {app.update_available ? 'has-update' : ''}">
              <div class="app-card__header">
                <div class="app-card__icon-box">
                  <i class="{app.icon} app-card__icon" aria-hidden="true"></i>
                </div>
                <div class="app-card__meta">
                  <h3 class="app-card__name" title={app.name}>{app.name}</h3>
                  <div class="app-card__subrow">
                    <span class="app-card__cat">{getCategoryLabel(app.category)}</span>
                  </div>
                </div>
              </div>

              <!-- Baris Metadata Versi Nyata, Update & Ukuran Disk (Selalu Sejajar Seragam) -->
              <div class="app-card__ver-row">
                <span class="ver-pill" title={app.installed_version ? `Versi terpasang: ${app.installed_version}` : `Versi katalog: ${app.version}`}>
                  {app.installed_version ? `v${app.installed_version}` : app.version}
                </span>
                {#if app.update_available && app.candidate_version}
                  <span class="ver-pill ver-pill--update" title="Versi baru tersedia di repositori APT">
                    <i class="ri-arrow-up-line"></i> v{app.candidate_version}
                  </span>
                {/if}
                {#if app.disk_size_kb}
                  <span class="ver-pill ver-pill--size">{formatSizeKB(app.disk_size_kb)}</span>
                {/if}
              </div>

              <p class="app-card__desc">{app.description}</p>

              <div class="app-card__footer">
                <div class="app-card__status">
                  {#if app.status === 'running'}
                    <span class="badge badge--success badge--pill">
                      <span class="dot dot--success dot--pulse"></span> Berjalan
                    </span>
                  {:else if app.status === 'stopped'}
                    <span class="badge badge--info badge--pill">
                      <span class="dot"></span> Terpasang
                    </span>
                  {:else}
                    <span class="badge badge--neutral badge--pill">
                      <span class="dot"></span> Belum Ada
                    </span>
                  {/if}
                </div>

                <div class="app-card__actions">
                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    onclick={() => openInspectModal(app.id, app)}
                    aria-label="Inspeksi detail {app.name}"
                    title="Inspeksi Detail & Dependensi"
                  >
                    <i class="ri-information-line"></i>
                  </button>

                  {#if app.installed}
                    {#if app.update_available}
                      <button
                        type="button"
                        class="btn btn--primary btn--sm"
                        onclick={() => upgradeApp(app.id, app.name, app.candidate_version)}
                        title="Upgrade ke versi {app.candidate_version}"
                      >
                        <i class="ri-arrow-up-circle-line"></i> Upgrade
                      </button>
                    {/if}

                    {#if app.service_unit}
                      <button
                        type="button"
                        class="btn btn--secondary btn--sm btn--icon"
                        disabled={busyServiceId === app.id}
                        onclick={() => controlAppService(app, app.status === 'running' ? 'restart' : 'start')}
                        aria-label="{app.status === 'running' ? 'Restart' : 'Jalankan'} layanan {app.name}"
                        title={app.status === 'running' ? 'Restart Layanan' : 'Jalankan Layanan'}
                      >
                        <i class="{app.status === 'running' ? 'ri-restart-line' : 'ri-play-line'} {busyServiceId === app.id ? 'spin' : ''}"></i>
                      </button>
                    {/if}

                    {#if app.manager_route}
                      <button
                        type="button"
                        class="btn btn--secondary btn--sm btn--icon"
                        onclick={() => navigateToManager(app.manager_route)}
                        aria-label="Buka manajer {app.name}"
                        title="Buka Pengaturan di Panel"
                      >
                        <i class="ri-settings-4-line"></i>
                      </button>
                    {/if}

                    <button
                      type="button"
                      class="btn btn--danger btn--sm"
                      onclick={() => uninstallApp(app)}
                      title="Copot dari server"
                    >
                      <i class="ri-delete-bin-line"></i> Copot
                    </button>
                  {:else}
                    <button type="button" class="btn btn--primary btn--sm" onclick={() => installApp(app)}>
                      <i class="ri-download-2-line"></i> Pasang
                    </button>
                  {/if}
                </div>
              </div>
            </article>
          {/each}
        </div>
      {:else}
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th class="col-software">NAMA SOFTWARE</th>
                <th class="col-category">KATEGORI</th>
                <th>DESKRIPSI &amp; VERSI</th>
                <th class="col-status">STATUS</th>
                <th class="text-right col-ops">OPERASI</th>
              </tr>
            </thead>
            <tbody>
              {#each filteredApps as app (app.id)}
                <tr>
                  <td>
                    <div class="table-app-meta">
                      <div class="table-icon-wrap">
                        <i class={app.icon} aria-hidden="true"></i>
                      </div>
                      <div>
                        <div class="table-app-name">{app.name}</div>
                        <span class="table-app-ver">
                          {app.installed_version ? `v${app.installed_version}` : app.version}
                          {#if app.update_available && app.candidate_version}
                            → v{app.candidate_version}
                          {/if}
                        </span>
                      </div>
                    </div>
                  </td>
                  <td><span class="badge badge--neutral">{getCategoryLabel(app.category)}</span></td>
                  <td class="text-muted text-xs">{app.description}</td>
                  <td>
                    {#if app.status === 'running'}
                      <span class="badge badge--success badge--pill"><span class="dot dot--success dot--pulse"></span> Berjalan</span>
                    {:else if app.status === 'stopped'}
                      <span class="badge badge--info badge--pill"><span class="dot"></span> Terpasang</span>
                    {:else}
                      <span class="badge badge--neutral badge--pill"><span class="dot"></span> Belum Ada</span>
                    {/if}
                  </td>
                  <td class="text-right">
                    <div class="table-actions">
                      <button type="button" class="btn btn--ghost btn--sm btn--icon" onclick={() => openInspectModal(app.id, app)} aria-label="Detail {app.name}" title="Detail Paket">
                        <i class="ri-information-line"></i>
                      </button>
                      {#if app.installed}
                        {#if app.update_available}
                          <button type="button" class="btn btn--primary btn--sm" onclick={() => upgradeApp(app.id, app.name, app.candidate_version)}>
                            <i class="ri-arrow-up-circle-line"></i> Upgrade
                          </button>
                        {/if}
                        {#if app.service_unit}
                          <button
                            type="button"
                            class="btn btn--secondary btn--sm btn--icon"
                            disabled={busyServiceId === app.id}
                            onclick={() => controlAppService(app, app.status === 'running' ? 'restart' : 'start')}
                            aria-label="Restart {app.name}"
                            title={app.status === 'running' ? 'Restart Layanan' : 'Jalankan Layanan'}
                          >
                            <i class={app.status === 'running' ? 'ri-restart-line' : 'ri-play-line'}></i>
                          </button>
                        {/if}
                        {#if app.manager_route}
                          <button type="button" class="btn btn--secondary btn--sm btn--icon" onclick={() => navigateToManager(app.manager_route)} aria-label="Pengaturan {app.name}" title="Buka Pengaturan">
                            <i class="ri-settings-4-line"></i>
                          </button>
                        {/if}
                        <button type="button" class="btn btn--danger btn--sm" onclick={() => uninstallApp(app)}>
                          <i class="ri-delete-bin-line"></i> Copot
                        </button>
                      {:else}
                        <button type="button" class="btn btn--primary btn--sm" onclick={() => installApp(app)}>
                          <i class="ri-download-2-line"></i> Pasang
                        </button>
                      {/if}
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}

      <!-- Hasil Pencarian Langsung Repositori APT Linux -->
      {#if searchQuery.trim().length >= 2}
        <section class="section-card card">
          <div class="section-card-head">
            <h3 class="section-card-title">
              <i class="ri-terminal-box-line text-accent"></i>
              Paket Repositori Linux Sistem (APT)
              <span class="badge badge--neutral badge--pill">{searchResults.length} hasil</span>
            </h3>
            {#if searching}
              <span class="text-xs text-subtle"><span class="spinner" aria-hidden="true"></span> Memindai APT...</span>
            {/if}
          </div>

          {#if searchResults.length > 0}
            <div class="table-wrap">
              <table class="table">
                <thead>
                  <tr>
                    <th class="col-pkg">NAMA PAKET</th>
                    <th>DESKRIPSI</th>
                    <th class="col-pkg-status">STATUS</th>
                    <th class="text-right col-pkg-action">AKSI</th>
                  </tr>
                </thead>
                <tbody>
                  {#each searchResults as pkg (pkg.name)}
                    <tr>
                      <td>
                        <div class="pkg-col">
                          <i class="ri-terminal-line text-accent"></i>
                          <div>
                            <div class="mono font-semibold text-accent">{pkg.name}</div>
                            {#if pkg.version}
                              <span class="table-app-ver">v{pkg.version}</span>
                            {/if}
                          </div>
                        </div>
                      </td>
                      <td class="text-muted text-xs">{pkg.description || 'Paket utilitas sistem Linux.'}</td>
                      <td>
                        {#if pkg.installed}
                          <span class="badge badge--success badge--pill"><span class="dot dot--success"></span> Terpasang</span>
                        {:else}
                          <span class="badge badge--neutral badge--pill"><span class="dot"></span> Belum Ada</span>
                        {/if}
                      </td>
                      <td class="text-right">
                        <div class="table-actions">
                          <button type="button" class="btn btn--ghost btn--sm btn--icon" onclick={() => openInspectModal(pkg.name, null)} aria-label="Inspeksi {pkg.name}" title="Inspeksi Detail Paket">
                            <i class="ri-information-line"></i>
                          </button>
                          {#if pkg.installed}
                            <button type="button" class="btn btn--danger btn--sm" onclick={() => uninstallGlobalPackage(pkg)}>
                              <i class="ri-delete-bin-line"></i> Copot
                            </button>
                          {:else}
                            <button type="button" class="btn btn--primary btn--sm" onclick={() => installGlobalPackage(pkg)}>
                              <i class="ri-download-2-line"></i> Pasang
                            </button>
                          {/if}
                        </div>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </section>
      {/if}
    {/if}

  <!-- TAB 2: STACK BUNDLES 1-KLIK -->
  {:else if activeTab === 'bundles'}
    <BundlesTab
      {bundles}
      {loading}
      onInstallBundle={installBundle}
      onViewBundleLog={(b) => viewLogs(`bundle:${b.id}`, `Log Bundel: ${b.name}`)}
    />

  <!-- TAB 3: APLIKASI DOCKER 1-KLIK -->
  {:else if activeTab === 'docker_apps'}
    <DockerAppsTab
      {dockerStatus}
      templates={dockerTemplates}
      loading={loadingDocker}
      onOpenDeployModal={openDeployModal}
      onInstallDocker={() => {
        const dApp = apps.find((a) => a.id === 'docker');
        if (dApp) installApp(dApp);
      }}
      onOpenDockerPage={() => navigateToManager('#/docker')}
    />

  <!-- TAB 4: REPOSITORI APT, PEMBARUAN & ANTREAN TUGAS -->
  {:else if activeTab === 'apt_queue'}
    <div class="apt-sections">
      <!-- Daftar Pembaruan Paket APT Tersedia -->
      <section class="section-card card">
        <div class="section-card-head">
          <h3 class="section-card-title">
            <i class="ri-arrow-up-circle-line text-warning"></i>
            Pembaruan Paket APT Tersedia ({upgradable.length})
          </h3>
          <button type="button" class="btn btn--secondary btn--sm" onclick={runAptUpdate} disabled={syncingApt}>
            <i class="ri-refresh-line {syncingApt ? 'spin' : ''}"></i> Cek Update (apt update)
          </button>
        </div>

        {#if upgradable.length === 0}
          <p class="text-xs text-muted">Seluruh paket APT terpantau mutakhir. Klik "Cek Update" untuk menyegarkan indeks repositori.</p>
        {:else}
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>PAKET</th>
                  <th>VERSI TERPASANG</th>
                  <th>VERSI KANDIDAT</th>
                  <th class="text-right">AKSI</th>
                </tr>
              </thead>
              <tbody>
                {#each upgradable.slice(0, 40) as up (up.name)}
                  <tr>
                    <td class="mono font-semibold text-accent">{up.name}</td>
                    <td class="mono text-xs text-muted">{up.current_version}</td>
                    <td><span class="ver-pill ver-pill--update">{up.candidate_version}</span></td>
                    <td class="text-right">
                      <div class="table-actions">
                        <button type="button" class="btn btn--ghost btn--sm btn--icon" onclick={() => openInspectModal(up.name, null)} aria-label="Inspeksi {up.name}" title="Detail Paket">
                          <i class="ri-information-line"></i>
                        </button>
                        <button type="button" class="btn btn--primary btn--sm" onclick={() => upgradeApp(`pkg:${up.name}`, up.name, up.candidate_version)}>
                          <i class="ri-arrow-up-circle-line"></i> Upgrade
                        </button>
                      </div>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </section>

      <!-- Antrean & Riwayat Operasi APT -->
      <section class="section-card card">
        <div class="section-card-head">
          <h3 class="section-card-title">
            <i class="ri-terminal-box-line text-accent"></i>
            Antrean &amp; Riwayat Operasi Paket ({tasks.length})
          </h3>
        </div>

        {#if tasks.length === 0}
          <p class="text-xs text-muted">Belum ada operasi instalasi, upgrade, atau pencopotan yang dijalankan pada sesi ini.</p>
        {:else}
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>OPERASI</th>
                  <th>JENIS</th>
                  <th>STATUS</th>
                  <th>WAKTU MULAI</th>
                  <th class="text-right">TERMINAL</th>
                </tr>
              </thead>
              <tbody>
                {#each tasks as tk (tk.app_id + tk.started_at)}
                  <tr>
                    <td class="font-semibold">{tk.title || tk.app_id}</td>
                    <td class="mono text-xs text-muted">{tk.action || 'apt'}</td>
                    <td>
                      {#if tk.status === 'running' || tk.status === 'queued'}
                        <span class="badge badge--warning badge--pill">{tk.status === 'queued' ? 'Antre' : 'Berjalan'}</span>
                      {:else if tk.status === 'success'}
                        <span class="badge badge--success badge--pill">Sukses</span>
                      {:else}
                        <span class="badge badge--danger badge--pill">Gagal</span>
                      {/if}
                    </td>
                    <td class="mono text-xs text-muted">{formatTime(tk.started_at)}</td>
                    <td class="text-right">
                      <button type="button" class="btn btn--secondary btn--sm" onclick={() => viewLogs(tk.app_id, tk.title || tk.app_id)}>
                        <i class="ri-terminal-line"></i> Lihat Log
                      </button>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </section>
    </div>
  {/if}

  <!-- Modal Inspektor Detail Paket APT & Kontrol Layanan -->
  <Modal
    open={showInspectModal}
    title={inspectedApp ? `Detail Software: ${inspectedApp.name}` : `Detail Paket APT: ${inspectedDetail?.name || ''}`}
    subtitle={inspectedDetail?.section ? `Kategori APT: ${inspectedDetail.section}` : 'Metadata langsung dari dpkg-query dan apt-cache show'}
    size="lg"
    onclose={() => (showInspectModal = false)}
  >
    {#if inspectLoading}
      <div class="state state--loading">
        <span class="spinner" aria-hidden="true"></span>
        <p class="state__title">Membaca metadata paket dari repositori APT...</p>
      </div>
    {:else if inspectedDetail}
      <div class="inspect-grid">
        <div class="inspect-cell">
          <span class="inspect-lbl">Paket Utama</span>
          <span class="inspect-val mono font-semibold">{inspectedDetail.name}</span>
        </div>
        <div class="inspect-cell">
          <span class="inspect-lbl">Versi Terpasang</span>
          <span class="inspect-val mono">{inspectedDetail.installed ? (inspectedDetail.installed_version || 'Terpasang') : 'Belum terpasang'}</span>
        </div>
        <div class="inspect-cell">
          <span class="inspect-lbl">Versi Kandidat APT</span>
          <span class="inspect-val mono">{inspectedDetail.candidate_version || '-'}</span>
        </div>
        <div class="inspect-cell">
          <span class="inspect-lbl">Ukuran Terpasang / Unduh</span>
          <span class="inspect-val mono">
            {formatSizeKB(inspectedDetail.installed_size_kb) || '-'} / {inspectedDetail.download_size || '-'}
          </span>
        </div>
      </div>

      {#if inspectedDetail.depends}
        <div class="inspect-cell">
          <span class="inspect-lbl">Dependensi</span>
          <span class="inspect-val mono">{inspectedDetail.depends}</span>
        </div>
      {/if}

      {#if inspectedApp?.apt_packages?.length}
        <div class="inspect-apt-chips">
          {#each inspectedApp.apt_packages as p}
            <span class="ver-pill">{p}</span>
          {/each}
        </div>
      {/if}

      {#if inspectedDetail.description}
        <div class="inspect-desc">{inspectedDetail.description}</div>
      {/if}
    {/if}

    {#snippet footer()}
      {#if inspectedApp?.installed && inspectedApp.service_unit}
        <button type="button" class="btn btn--secondary btn--sm" onclick={() => controlAppService(inspectedApp!, 'start')} disabled={busyServiceId === inspectedApp.id}>
          <i class="ri-play-line"></i> Start
        </button>
        <button type="button" class="btn btn--secondary btn--sm" onclick={() => controlAppService(inspectedApp!, 'restart')} disabled={busyServiceId === inspectedApp.id}>
          <i class="ri-restart-line"></i> Restart
        </button>
        <button type="button" class="btn btn--danger btn--sm" onclick={() => controlAppService(inspectedApp!, 'stop')} disabled={busyServiceId === inspectedApp.id}>
          <i class="ri-stop-circle-line"></i> Stop
        </button>
      {/if}
      {#if inspectedDetail?.update_available}
        <button
          type="button"
          class="btn btn--primary btn--sm"
          onclick={() => {
            showInspectModal = false;
            upgradeApp(inspectedApp ? inspectedApp.id : `pkg:${inspectedDetail!.name}`, inspectedApp?.name || inspectedDetail!.name, inspectedDetail!.candidate_version);
          }}
        >
          <i class="ri-arrow-up-circle-line"></i> Upgrade Sekarang
        </button>
      {/if}
      <button type="button" class="btn btn--ghost btn--sm" onclick={() => (showInspectModal = false)}>Tutup</button>
    {/snippet}
  </Modal>

  <!-- Modal Deploy Aplikasi Docker 1-Klik -->
  <Modal
    open={showDeployModal && !!deployTemplate}
    title={deployTemplate ? `Deploy 1-Klik: ${deployTemplate.name}` : 'Deploy Aplikasi Docker'}
    subtitle={deployTemplate ? `Image: ${deployTemplate.image}` : ''}
    size="md"
    onclose={() => (showDeployModal = false)}
  >
    {#if deployTemplate}
      <div class="docker-deploy-form">
        <div class="deploy-field">
          <label for="dk-proj-name">Nama Stack / Proyek Compose</label>
          <input id="dk-proj-name" type="text" class="input mono" bind:value={deployProjectName} placeholder={deployTemplate.id} />
        </div>

        {#if (deployTemplate.default_ports || []).length > 0}
          <div class="deploy-kv-grid">
            {#each deployTemplate.default_ports as origPort}
              <div class="deploy-field">
                <label for="dk-port-{origPort}">Pemetaan Port ({origPort})</label>
                <input id="dk-port-{origPort}" type="text" class="input mono" bind:value={deployPorts[origPort]} />
              </div>
            {/each}
          </div>
        {/if}

        {#if Object.keys(deployEnv).length > 0}
          <div class="deploy-kv-grid">
            {#each Object.keys(deployEnv) as envKey}
              <div class="deploy-field">
                <label for="dk-env-{envKey}">{envKey}</label>
                <input id="dk-env-{envKey}" type="text" class="input mono" bind:value={deployEnv[envKey]} />
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    {#snippet footer()}
      <button type="button" class="btn btn--ghost btn--sm" onclick={() => (showDeployModal = false)}>Batal</button>
      <button type="button" class="btn btn--primary btn--sm" onclick={submitDeployTemplate} disabled={deployingDocker}>
        <i class="ri-rocket-line {deployingDocker ? 'spin' : ''}"></i>
        {deployingDocker ? 'Mendeploy...' : 'Jalankan Docker Compose'}
      </button>
    {/snippet}
  </Modal>

  <!-- Modal Log Output Terminal APT Live -->
  <Modal
    open={showLogModal}
    title={logModalTitle || 'Eksekusi Paket Server'}
    size="lg"
    onclose={closeLogModal}
  >
    <div class="status-banner {currentTask?.status || 'running'}">
      <span class="font-bold">STATUS OPERASI:</span>
      {#if currentTask?.status === 'queued'}
        <span class="status-msg"><span class="spinner" aria-hidden="true"></span> Menunggu giliran antrean lock APT...</span>
      {:else if currentTask?.status === 'running'}
        <span class="status-msg"><span class="spinner" aria-hidden="true"></span> Memproses paket secara live di background...</span>
      {:else if currentTask?.status === 'success'}
        <span class="status-msg text-accent"><i class="ri-checkbox-circle-fill"></i> Operasi selesai dengan sukses!</span>
      {:else if currentTask?.status === 'failed'}
        <span class="status-msg text-danger"><i class="ri-close-circle-fill"></i> Terjadi kesalahan dalam eksekusi perintah APT.</span>
      {:else}
        <span class="status-msg text-muted">Belum ada catatan operasi untuk paket ini.</span>
      {/if}
    </div>

    <pre class="log-terminal mono" bind:this={logTerminalEl}>{currentTask?.logs || 'Menunggu keluaran terminal APT...'}</pre>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary btn--sm" onclick={copyTaskLogs} disabled={!currentTask?.logs}>
        <i class="ri-file-copy-line"></i> Salin Log
      </button>
      <button type="button" class="btn btn--primary btn--sm" onclick={closeLogModal}>Tutup</button>
    {/snippet}
  </Modal>
</div>
