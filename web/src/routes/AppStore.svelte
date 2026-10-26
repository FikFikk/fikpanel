<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { AppPackage, InstallTask, GlobalPackage } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  // Kategori filter aplikasi
  type CategoryFilter = 'all' | 'installed' | 'webserver' | 'runtime' | 'database' | 'tools';
  let activeCategory = $state<CategoryFilter>('all');

  // Mode tampilan: Kartu (Grid) atau Tabel (Table)
  type ViewMode = 'grid' | 'table';
  let viewMode = $state<ViewMode>('grid');

  // Data katalog software
  let apps = $state<AppPackage[]>([]);
  let loading = $state(true);

  // Pencarian fuzzy & autocomplete
  let searchQuery = $state('');
  let searchFocused = $state(false);
  let searchHistory = $state<string[]>([]);
  let searchResults = $state<GlobalPackage[]>([]);
  let searching = $state(false);
  let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;

  // Modal Log Operasi APT
  let showLogModal = $state(false);
  let logModalTitle = $state('');
  let currentTask = $state<InstallTask | null>(null);
  let pollingInterval: ReturnType<typeof setInterval> | null = null;

  // Daftar software populer untuk rekomendasi cepat
  const trendingTags = [
    { name: 'Nginx', icon: 'ri-server-line', query: 'nginx' },
    { name: 'PHP 8.3', icon: 'ri-code-s-slash-line', query: 'php' },
    { name: 'MariaDB', icon: 'ri-database-2-line', query: 'mariadb' },
    { name: 'Redis', icon: 'ri-flashlight-line', query: 'redis' },
    { name: 'Docker', icon: 'ri-instance-line', query: 'docker' },
    { name: 'Node.js', icon: 'ri-nodejs-line', query: 'node' },
    { name: 'Composer', icon: 'ri-terminal-box-line', query: 'composer' },
    { name: 'Fail2ban', icon: 'ri-shield-keyhole-line', query: 'fail2ban' },
    { name: 'Supervisor', icon: 'ri-cpu-line', query: 'supervisor' },
    { name: 'htop', icon: 'ri-pulse-line', query: 'htop' },
    { name: 'ffmpeg', icon: 'ri-video-line', query: 'ffmpeg' },
  ];

  // Muat riwayat pencarian dari penyimpanan lokal browser
  function loadHistory() {
    try {
      const saved = localStorage.getItem('fikpanel_appstore_history');
      if (saved) {
        searchHistory = JSON.parse(saved);
      }
    } catch {
      searchHistory = [];
    }
  }

  // Simpan kata kunci pencarian ke riwayat
  function saveSearchQuery(q: string) {
    const trimmed = q.trim();
    if (!trimmed || trimmed.length < 2) return;
    const next = [trimmed, ...searchHistory.filter((item) => item.toLowerCase() !== trimmed.toLowerCase())].slice(0, 8);
    searchHistory = next;
    try {
      localStorage.setItem('fikpanel_appstore_history', JSON.stringify(next));
    } catch {
      // Abaikan jika penyimpanan lokal tidak tersedia
    }
  }

  // Hapus seluruh riwayat pencarian
  function clearHistory() {
    searchHistory = [];
    try {
      localStorage.removeItem('fikpanel_appstore_history');
    } catch {
      // Abaikan
    }
  }

  // Muat preferensi mode tampilan (Grid / Table)
  function loadViewPreference() {
    try {
      const pref = localStorage.getItem('fikpanel_appstore_view_mode') as ViewMode | null;
      if (pref === 'grid' || pref === 'table') {
        viewMode = pref;
      }
    } catch {
      viewMode = 'grid';
    }
  }

  // Ganti mode tampilan dan simpan preferensi
  function setViewMode(mode: ViewMode) {
    viewMode = mode;
    try {
      localStorage.setItem('fikpanel_appstore_view_mode', mode);
    } catch {
      // Abaikan
    }
  }

  // Muat status seluruh katalog aplikasi dari API
  async function loadCatalog() {
    try {
      loading = true;
      const res = await apiFetch<{ ok: boolean; apps: AppPackage[] }>('/api/appstore/catalog');
      apps = res?.apps || [];
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memuat katalog App Store', 'error');
    } finally {
      loading = false;
    }
  }

  // Pasang paket aplikasi katalog
  async function installApp(app: AppPackage) {
    const ok = await confirmDialog({
      title: `Pasang ${app.name}`,
      message: `Pasang "${app.name}" (${app.version}) ke sistem server? Proses instalasi berjalan di background.`,
      confirmLabel: 'Mulai Pasang',
      variant: 'primary',
    });
    if (!ok) return;

    try {
      notify(`Memulai instalasi ${app.name}...`, 'info');
      await apiFetch(`/api/appstore/install/${app.id}`, { method: 'POST' });
      viewLogs(app.id, `Instalasi: ${app.name}`);
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memulai instalasi', 'error');
    }
  }

  // Copot paket aplikasi katalog
  async function uninstallApp(app: AppPackage) {
    const ok = await confirmDialog({
      title: `Copot ${app.name}`,
      message: `Apakah Anda yakin ingin mencopot "${app.name}" dari server? Pastikan dependensi web tidak terdampak.`,
      confirmLabel: 'Copot Paket',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      notify(`Memulai pencopotan ${app.name}...`, 'info');
      await apiFetch(`/api/appstore/uninstall/${app.id}`, { method: 'POST' });
      viewLogs(app.id, `Pencopotan: ${app.name}`);
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memulai pencopotan', 'error');
    }
  }

  // Pencarian paket sistem Linux global (APT)
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

  // Pasang paket Linux sistem global (APT)
  async function installGlobalPackage(pkg: GlobalPackage) {
    const ok = await confirmDialog({
      title: `Pasang Paket ${pkg.name}`,
      message: `Pasang paket Linux "${pkg.name}" dari repositori sistem (apt-get)?`,
      confirmLabel: 'Pasang Sekarang',
      variant: 'primary',
    });
    if (!ok) return;

    try {
      notify(`Memulai instalasi paket ${pkg.name}...`, 'info');
      await apiFetch(`/api/appstore/install/pkg:${pkg.name}`, { method: 'POST' });
      viewLogs(`pkg:${pkg.name}`, `Instalasi Paket Linux: ${pkg.name}`);
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memulai instalasi', 'error');
    }
  }

  // Copot paket Linux sistem global (APT)
  async function uninstallGlobalPackage(pkg: GlobalPackage) {
    const ok = await confirmDialog({
      title: `Copot Paket ${pkg.name}`,
      message: `Hapus paket "${pkg.name}" dari server via apt-get remove?`,
      confirmLabel: 'Copot Paket',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      notify(`Memulai pencopotan paket ${pkg.name}...`, 'info');
      await apiFetch(`/api/appstore/uninstall/pkg:${pkg.name}`, { method: 'POST' });
      viewLogs(`pkg:${pkg.name}`, `Pencopotan Paket: ${pkg.name}`);
    } catch (err: unknown) {
      notify((err as Error).message || 'Gagal memulai pencopotan', 'error');
    }
  }

  // Tangani input pencarian dengan debounce
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
    }, 450);
  }

  // Pilih tag cepat dari dropdown autocomplete
  function selectTag(term: string) {
    searchQuery = term;
    searchFocused = false;
    saveSearchQuery(term);
    searchGlobalPackages(term);
  }

  // Bersihkan kolom pencarian
  function clearSearchInput() {
    searchQuery = '';
    searchResults = [];
    searchFocused = false;
  }

  // Tampilkan modal log operasi proses
  function viewLogs(taskID: string, title: string) {
    logModalTitle = title;
    showLogModal = true;
    pollTask(taskID);
  }

  // Polling status task instalasi/pencopotan APT
  async function pollTask(taskID: string) {
    if (pollingInterval) clearInterval(pollingInterval);
    const fetchStatus = async () => {
      try {
        const res = await apiFetch<{ ok: boolean; task: InstallTask | null }>(
          `/api/appstore/tasks/${encodeURIComponent(taskID)}`
        );
        currentTask = res?.task || null;
        if (currentTask && currentTask.status !== 'running') {
          clearInterval(pollingInterval!);
          loadCatalog();
          if (searchQuery.trim()) {
            searchGlobalPackages(searchQuery);
          }
        }
      } catch {
        // Abaikan kegagalan sementara selama polling
      }
    };
    await fetchStatus();
    pollingInterval = setInterval(fetchStatus, 2000);
  }

  // Tutup modal log
  function closeLogModal() {
    showLogModal = false;
    if (pollingInterval) clearInterval(pollingInterval);
  }

  // Cek apakah software memiliki halaman manajemen khusus di panel
  function hasDedicatedManager(id: string): boolean {
    return ['nginx', 'php8.5', 'php8.3', 'php8.2', 'mariadb', 'postgresql', 'nodejs', 'docker', 'certbot', 'fail2ban', 'ufw', 'supervisor'].includes(id);
  }

  // Navigasi ke halaman manajemen khusus perangkat lunak
  function navigateToManager(id: string) {
    if (id === 'nginx') window.location.hash = '#website';
    else if (id.startsWith('php')) window.location.hash = '#php';
    else if (id === 'mariadb' || id === 'postgresql') window.location.hash = '#databases';
    else if (id === 'nodejs') window.location.hash = '#node';
    else if (id === 'docker') window.location.hash = '#docker';
    else if (id === 'certbot') window.location.hash = '#dns';
    else if (id === 'fail2ban' || id === 'ufw') window.location.hash = '#security';
    else if (id === 'supervisor') window.location.hash = '#cron';
  }

  // Label kategori ramah pengguna
  function getCategoryLabel(cat: string): string {
    switch (cat) {
      case 'webserver': return 'Web Server';
      case 'runtime': return 'Runtime';
      case 'database': return 'Database & Cache';
      case 'tools': return 'Alat & Keamanan';
      default: return cat;
    }
  }

  // Hitung jumlah aplikasi terpasang
  let installedCount = $derived(apps.filter((a) => a.installed).length);

  // Filter aplikasi katalog berdasarkan kategori aktif dan pencarian
  let filteredApps = $derived(
    apps.filter((app) => {
      // Filter kategori
      if (activeCategory === 'installed' && !app.installed) return false;
      if (activeCategory !== 'all' && activeCategory !== 'installed' && app.category !== activeCategory) {
        return false;
      }
      // Filter pencarian
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
  });

  onDestroy(() => {
    if (pollingInterval) clearInterval(pollingInterval);
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
  });
</script>

<div class="store-page">
  <!-- Header Bar -->
  <header class="store-head">
    <div class="store-head__info">
      <h1 class="store-title">
        <i class="ri-apps-2-line"></i> App Store & Paket Server
      </h1>
      <p class="store-sub">
        Manajemen instalasi web stack, database, runtime, dan paket sistem Linux dengan 1-klik.
      </p>
    </div>
    <div class="store-head__actions">
      <button class="btn btn--secondary btn--sm" onclick={loadCatalog} disabled={loading} title="Perbarui status daftar paket">
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Perbarui Daftar
      </button>
    </div>
  </header>

  <!-- Search Card dengan Dropdown Autocomplete & Riwayat -->
  <div class="search-section">
    <div class="search-box card">
      <div class="search-input-wrap">
        <i class="ri-search-line search-icon" aria-hidden="true"></i>
        <input
          type="text"
          value={searchQuery}
          oninput={handleSearchInput}
          onfocus={() => (searchFocused = true)}
          placeholder="Cari aplikasi atau paket sistem (misal: Nginx, PHP, MariaDB, Docker, htop, ffmpeg)..."
          class="input search-field"
          aria-label="Cari aplikasi dan paket"
        />
        {#if searching}
          <span class="spinner search-spin" aria-label="Mencari"></span>
        {:else if searchQuery}
          <button class="btn-clear-search" onclick={clearSearchInput} aria-label="Bersihkan pencarian">
            <i class="ri-close-line"></i>
          </button>
        {/if}
      </div>

      <!-- Dropdown Autocomplete: Riwayat & Trending Populer -->
      {#if searchFocused && !searchQuery}
        <div class="search-popover">
          {#if searchHistory.length > 0}
            <div class="popover-group">
              <div class="popover-title">
                <span><i class="ri-history-line"></i> Riwayat Pencarian</span>
                <button class="btn-clear-history" onclick={clearHistory}>Hapus Semua</button>
              </div>
              <div class="popover-chips">
                {#each searchHistory as term}
                  <button type="button" class="chip-btn" onclick={() => selectTag(term)}>
                    {term}
                  </button>
                {/each}
              </div>
            </div>
          {/if}

          <div class="popover-group">
            <div class="popover-title">
              <span><i class="ri-fire-line text-warning"></i> Rekomendasi & Populer</span>
            </div>
            <div class="popover-chips">
              {#each trendingTags as item}
                <button type="button" class="chip-btn" onclick={() => selectTag(item.query)}>
                  <i class="{item.icon}"></i> {item.name}
                </button>
              {/each}
            </div>
          </div>
        </div>
      {/if}
    </div>
  </div>

  <!-- Toolbar: Filter Kategori & View Switcher (Grid / Table) -->
  <div class="toolbar-row">
    <!-- Filter Kategori Kiri (Pills) -->
    <div class="filter-pills" role="tablist" aria-label="Filter kategori software">
      <button
        class="filter-pill {activeCategory === 'all' ? 'active' : ''}"
        onclick={() => (activeCategory = 'all')}
        role="tab"
        aria-selected={activeCategory === 'all'}
      >
        Semua ({apps.length})
      </button>
      <button
        class="filter-pill {activeCategory === 'installed' ? 'active' : ''}"
        onclick={() => (activeCategory = 'installed')}
        role="tab"
        aria-selected={activeCategory === 'installed'}
      >
        <span class="dot dot--success"></span>
        Terpasang ({installedCount})
      </button>
      <button
        class="filter-pill {activeCategory === 'webserver' ? 'active' : ''}"
        onclick={() => (activeCategory = 'webserver')}
        role="tab"
        aria-selected={activeCategory === 'webserver'}
      >
        Web Server
      </button>
      <button
        class="filter-pill {activeCategory === 'runtime' ? 'active' : ''}"
        onclick={() => (activeCategory = 'runtime')}
        role="tab"
        aria-selected={activeCategory === 'runtime'}
      >
        Runtime & Bahasa
      </button>
      <button
        class="filter-pill {activeCategory === 'database' ? 'active' : ''}"
        onclick={() => (activeCategory = 'database')}
        role="tab"
        aria-selected={activeCategory === 'database'}
      >
        Database & Cache
      </button>
      <button
        class="filter-pill {activeCategory === 'tools' ? 'active' : ''}"
        onclick={() => (activeCategory = 'tools')}
        role="tab"
        aria-selected={activeCategory === 'tools'}
      >
        Alat & Keamanan
      </button>
    </div>

    <!-- Toggle View Mode Kanan: Grid vs Table -->
    <div class="view-toggle segmented" role="radiogroup" aria-label="Pilih mode tampilan">
      <button
        type="button"
        class={viewMode === 'grid' ? 'active' : ''}
        onclick={() => setViewMode('grid')}
        aria-checked={viewMode === 'grid'}
        aria-label="Tampilan Kartu (Grid)"
        title="Tampilan Kartu"
      >
        <i class="ri-layout-grid-line"></i>
      </button>
      <button
        type="button"
        class={viewMode === 'table' ? 'active' : ''}
        onclick={() => setViewMode('table')}
        aria-checked={viewMode === 'table'}
        aria-label="Tampilan Tabel (List)"
        title="Tampilan Tabel"
      >
        <i class="ri-list-check-2"></i>
      </button>
    </div>
  </div>

  <!-- Area Konten Aplikasi Utama -->
  {#if loading && apps.length === 0}
    <div class="empty-state">
      <span class="spinner" aria-hidden="true"></span>
      <p class="empty-state__title">Memeriksa repositori dan paket sistem...</p>
    </div>
  {:else if filteredApps.length === 0 && searchResults.length === 0}
    <div class="empty-state card">
      <i class="ri-inbox-line" aria-hidden="true"></i>
      <p class="empty-state__title">Tidak ditemukan aplikasi yang cocok</p>
      <p class="text-subtle">
        {searchQuery ? `Tidak ada hasil untuk kata kunci "${searchQuery}".` : 'Belum ada software dalam kategori ini.'}
      </p>
      {#if searchQuery}
        <button class="btn btn--secondary btn--sm" onclick={clearSearchInput}>
          <i class="ri-arrow-go-back-line"></i> Bersihkan Filter
        </button>
      {/if}
    </div>
  {:else}
    <!-- MODE 1: GRID / KARTU (Sesuai Referensi Gambar 2) -->
    {#if viewMode === 'grid'}
      <div class="apps-grid">
        {#each filteredApps as app (app.id)}
          <div class="app-card card">
            <!-- Header Kartu: Ikon, Nama, Versi, Kategori -->
            <div class="app-card__header">
              <div class="app-card__icon-box">
                <i class="{app.icon} app-card__icon"></i>
              </div>
              <div class="app-card__meta">
                <div class="app-card__title-row">
                  <h3 class="app-card__name">{app.name}</h3>
                  <span class="badge badge--pill badge--secondary app-card__ver">{app.version}</span>
                </div>
                <span class="app-card__cat">{getCategoryLabel(app.category)}</span>
              </div>
            </div>

            <!-- Deskripsi Kartu -->
            <p class="app-card__desc">{app.description}</p>

            <!-- Footer Kartu: Status & Aksi -->
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
                  <span class="badge badge--secondary badge--pill">
                    <span class="dot"></span> Belum Ada
                  </span>
                {/if}
              </div>

              <div class="app-card__actions">
                {#if app.installed}
                  <!-- Tombol Pengaturan Khusus jika didukung -->
                  {#if hasDedicatedManager(app.id)}
                    <button
                      class="btn btn--secondary btn--sm btn--icon"
                      onclick={() => navigateToManager(app.id)}
                      aria-label="Buka Pengaturan {app.name}"
                      title="Buka Pengaturan"
                    >
                      <i class="ri-settings-4-line"></i>
                    </button>
                  {/if}

                  <button
                    class="btn btn--ghost btn--sm btn--icon"
                    onclick={() => viewLogs(app.id, `Log Operasi: ${app.name}`)}
                    aria-label="Lihat Log {app.name}"
                    title="Lihat Log Operasi"
                  >
                    <i class="ri-file-text-line"></i>
                  </button>

                  <button
                    class="btn btn--danger btn--sm"
                    onclick={() => uninstallApp(app)}
                    title="Copot dari sistem"
                  >
                    <i class="ri-delete-bin-line"></i> Copot
                  </button>
                {:else}
                  <button class="btn btn--primary btn--sm" onclick={() => installApp(app)}>
                    <i class="ri-download-2-line"></i> Pasang
                  </button>
                {/if}
              </div>
            </div>
          </div>
        {/each}
      </div>
    {/if}

    <!-- MODE 2: TABEL / LIST (Sesuai Referensi Gambar 1) -->
    {#if viewMode === 'table'}
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th style="min-width: 220px;">NAMA SOFTWARE</th>
              <th style="width: 150px;">KATEGORI</th>
              <th>DESKRIPSI & PERUNTUKAN</th>
              <th style="width: 130px;">STATUS</th>
              <th class="text-right" style="min-width: 180px;">OPERASI</th>
            </tr>
          </thead>
          <tbody>
            {#each filteredApps as app (app.id)}
              <tr>
                <td>
                  <div class="table-app-meta">
                    <div class="table-icon-wrap">
                      <i class="{app.icon}"></i>
                    </div>
                    <div>
                      <div class="table-app-name">{app.name}</div>
                      <span class="table-app-ver">{app.version}</span>
                    </div>
                  </div>
                </td>
                <td>
                  <span class="badge badge--secondary">{getCategoryLabel(app.category)}</span>
                </td>
                <td class="text-muted text-xs">
                  {app.description}
                </td>
                <td>
                  {#if app.status === 'running'}
                    <span class="badge badge--success badge--pill">
                      <span class="dot dot--success dot--pulse"></span> Berjalan
                    </span>
                  {:else if app.status === 'stopped'}
                    <span class="badge badge--info badge--pill">
                      <span class="dot"></span> Terpasang
                    </span>
                  {:else}
                    <span class="badge badge--secondary badge--pill">
                      <span class="dot"></span> Belum Ada
                    </span>
                  {/if}
                </td>
                <td class="text-right">
                  <div class="table-actions">
                    {#if app.installed}
                      {#if hasDedicatedManager(app.id)}
                        <button
                          class="btn btn--secondary btn--sm btn--icon"
                          onclick={() => navigateToManager(app.id)}
                          aria-label="Pengaturan {app.name}"
                          title="Buka Pengaturan"
                        >
                          <i class="ri-settings-4-line"></i>
                        </button>
                      {/if}
                      <button
                        class="btn btn--ghost btn--sm btn--icon"
                        onclick={() => viewLogs(app.id, `Log Operasi: ${app.name}`)}
                        aria-label="Lihat Log {app.name}"
                        title="Lihat Log"
                      >
                        <i class="ri-file-text-line"></i>
                      </button>
                      <button
                        class="btn btn--danger btn--sm"
                        onclick={() => uninstallApp(app)}
                        title="Copot dari server"
                      >
                        <i class="ri-delete-bin-line"></i> Copot
                      </button>
                    {:else}
                      <button class="btn btn--primary btn--sm" onclick={() => installApp(app)}>
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

    <!-- HASIL PENCARIAN REPOSITORI LINUX SISTEM (APT) JIKA ADA PENCARIAN -->
    {#if searchQuery.trim().length >= 2}
      <div class="global-results-section">
        <div class="global-results-head">
          <div class="global-head-title">
            <i class="ri-terminal-box-line text-accent"></i>
            <h3>Paket Repositori Linux Sistem (APT)</h3>
            <span class="badge badge--secondary badge--pill">{searchResults.length} hasil</span>
          </div>
          {#if searching}
            <span class="text-xs text-subtle flex items-center gap-xs">
              <span class="spinner" aria-hidden="true"></span> Mencari repositori OS...
            </span>
          {/if}
        </div>

        {#if searchResults.length > 0}
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th style="width: 220px;">NAMA PAKET</th>
                  <th>DESKRIPSI LENGKAP</th>
                  <th style="width: 140px;">STATUS</th>
                  <th class="text-right" style="width: 130px;">AKSI</th>
                </tr>
              </thead>
              <tbody>
                {#each searchResults as pkg (pkg.name)}
                  <tr>
                    <td>
                      <div class="pkg-col">
                        <i class="ri-terminal-line text-accent"></i>
                        <span class="mono font-semibold text-accent">{pkg.name}</span>
                      </div>
                    </td>
                    <td class="text-muted text-xs">
                      {pkg.description || 'Paket utilitas sistem Linux.'}
                    </td>
                    <td>
                      {#if pkg.installed}
                        <span class="badge badge--success badge--pill">
                          <span class="dot dot--success"></span> Terpasang
                        </span>
                      {:else}
                        <span class="badge badge--secondary badge--pill">
                          <span class="dot"></span> Belum Ada
                        </span>
                      {/if}
                    </td>
                    <td class="text-right">
                      {#if pkg.installed}
                        <button class="btn btn--danger btn--sm" onclick={() => uninstallGlobalPackage(pkg)}>
                          <i class="ri-delete-bin-line"></i> Copot
                        </button>
                      {:else}
                        <button class="btn btn--primary btn--sm" onclick={() => installGlobalPackage(pkg)}>
                          <i class="ri-download-2-line"></i> Pasang
                        </button>
                      {/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    {/if}
  {/if}
</div>

<!-- Modal Log Output Terminal Real-time -->
{#if showLogModal}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) closeLogModal(); }}
    onkeydown={(e) => { if (e.key === 'Escape') closeLogModal(); }}
  >
    <div class="modal-card card">
      <div class="modal-card__head">
        <div class="modal-card__title">
          <i class="ri-terminal-box-line"></i>
          <h3>{logModalTitle || 'Eksekusi Paket Server'}</h3>
        </div>
        <button class="btn-close" onclick={closeLogModal} aria-label="Tutup modal">&times;</button>
      </div>

      <div class="modal-card__body">
        <div class="status-banner {currentTask?.status || 'running'}">
          <span class="font-bold">STATUS OPERASI:</span>
          {#if currentTask?.status === 'running'}
            <span class="status-msg">
              <span class="spinner" aria-hidden="true"></span> Sedang memproses paket di background...
            </span>
          {:else if currentTask?.status === 'success'}
            <span class="status-msg text-accent">
              <i class="ri-checkbox-circle-fill"></i> Operasi selesai dengan sukses!
            </span>
          {:else if currentTask?.status === 'failed'}
            <span class="status-msg text-danger">
              <i class="ri-close-circle-fill"></i> Terjadi kesalahan dalam eksekusi perintah apt.
            </span>
          {:else}
            <span>Menunggu daemon...</span>
          {/if}
        </div>

        <pre class="log-terminal mono">{currentTask?.logs || 'Memulai eksekusi...'}</pre>
      </div>

      <div class="modal-card__foot">
        <button class="btn btn--secondary" onclick={closeLogModal}>Tutup</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .store-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  /* Header Halaman */
  .store-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }
  .store-title {
    font-size: var(--text-lg);
    font-weight: var(--weight-bold);
    color: var(--text);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
  }
  .store-title i {
    color: var(--accent);
  }
  .store-sub {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin: var(--space-1) 0 0 0;
  }
  .store-head__actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  /* Search Area */
  .search-section {
    position: relative;
    z-index: var(--z-dropdown);
  }
  .search-box {
    position: relative;
    padding: var(--space-2) var(--space-3);
    background: var(--bg-surface);
  }
  .search-input-wrap {
    position: relative;
    display: flex;
    align-items: center;
  }
  .search-icon {
    position: absolute;
    left: var(--space-3);
    color: var(--text-subtle);
    font-size: 1.15rem;
    pointer-events: none;
  }
  .search-field {
    padding-left: calc(var(--space-3) * 2 + 1rem);
    padding-right: 2.5rem;
    height: var(--control-lg);
    background: var(--bg-inset);
    border-color: var(--border);
    font-size: var(--text-sm);
  }
  .search-spin {
    position: absolute;
    right: var(--space-3);
    color: var(--accent);
  }
  .btn-clear-search {
    position: absolute;
    right: var(--space-3);
    background: transparent;
    border: none;
    color: var(--text-subtle);
    cursor: pointer;
    font-size: 1.1rem;
    padding: var(--space-1);
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--radius-sm);
  }
  .btn-clear-search:hover {
    color: var(--text);
  }

  /* Popover Autocomplete */
  .search-popover {
    position: absolute;
    top: calc(100% + var(--space-2));
    left: 0;
    right: 0;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-modal);
    padding: var(--space-3) var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    z-index: var(--z-modal);
  }
  .popover-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .popover-title {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: var(--text-2xs);
    font-weight: var(--weight-semibold);
    color: var(--text-subtle);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .btn-clear-history {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: var(--text-2xs);
    cursor: pointer;
    text-decoration: underline;
  }
  .btn-clear-history:hover {
    color: var(--danger-strong);
  }
  .popover-chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .chip-btn {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-1) var(--space-3);
    font-size: var(--text-xs);
    color: var(--text-secondary);
    cursor: pointer;
    transition: all var(--duration-fast) ease;
  }
  .chip-btn:hover {
    background: var(--bg-hover);
    color: var(--text);
    border-color: var(--accent);
  }
  .chip-btn i {
    color: var(--accent);
  }

  /* Toolbar: Filter Pills & View Switcher */
  .toolbar-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }
  .filter-pills {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .filter-pill {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    color: var(--text-muted);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-full);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    transition: all var(--duration-fast) ease;
  }
  .filter-pill:hover {
    color: var(--text);
    border-color: var(--border-strong);
    background: var(--bg-hover);
  }
  .filter-pill.active {
    background: var(--tint-accent);
    color: var(--accent-strong);
    border-color: var(--accent);
    font-weight: var(--weight-semibold);
  }

  /* View Switcher */
  .view-toggle {
    margin-left: auto;
  }
  .view-toggle button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 1.1rem;
    width: 34px;
    padding: 0;
  }

  /* Apps Grid Layout */
  .apps-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: var(--space-4);
  }

  /* App Card */
  .app-card {
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: var(--space-3);
    transition: transform var(--duration-fast) ease, border-color var(--duration-fast) ease;
  }
  .app-card:hover {
    border-color: var(--border-strong);
    transform: translateY(-2px);
  }
  .app-card__header {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
  }
  .app-card__icon-box {
    width: 48px;
    height: 48px;
    border-radius: var(--radius-lg);
    background: var(--bg-inset);
    border: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }
  .app-card__icon {
    font-size: 1.6rem;
    color: var(--accent);
  }
  .app-card__meta {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    flex: 1;
  }
  .app-card__title-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    justify-content: space-between;
    flex-wrap: wrap;
  }
  .app-card__name {
    font-size: var(--text-sm);
    font-weight: var(--weight-bold);
    color: var(--text);
    margin: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .app-card__ver {
    font-family: var(--font-mono);
  }
  .app-card__cat {
    font-size: var(--text-2xs);
    color: var(--text-subtle);
  }
  .app-card__desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: var(--leading-relaxed);
    margin: 0;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    flex: 1;
  }
  .app-card__footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding-top: var(--space-3);
    border-top: 1px solid var(--border-subtle);
  }
  .app-card__actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  /* Table Specific Styles */
  .table-app-meta {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .table-icon-wrap {
    width: 34px;
    height: 34px;
    border-radius: var(--radius-md);
    background: var(--bg-inset);
    border: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }
  .table-icon-wrap i {
    font-size: 1.2rem;
    color: var(--accent);
  }
  .table-app-name {
    font-weight: var(--weight-semibold);
    color: var(--text);
  }
  .table-app-ver {
    font-size: var(--text-2xs);
    color: var(--text-subtle);
    font-family: var(--font-mono);
  }
  .table-actions {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
  }

  /* Linux OS Package Results */
  .global-results-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    margin-top: var(--space-2);
  }
  .global-results-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-top: var(--space-2);
    border-top: 1px solid var(--border);
  }
  .global-head-title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .global-head-title h3 {
    margin: 0;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--text);
  }
  .pkg-col {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  /* Modal Log Styles */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: var(--bg-overlay);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: var(--z-modal);
    padding: var(--space-4);
  }
  .modal-card {
    width: 100%;
    max-width: 760px;
    background: var(--bg-surface);
    box-shadow: var(--shadow-modal);
  }
  .modal-card__head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
  }
  .modal-card__title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .modal-card__title i {
    font-size: 1.25rem;
    color: var(--accent);
  }
  .modal-card__title h3 {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text);
  }
  .btn-close {
    background: transparent;
    border: none;
    color: var(--text-subtle);
    font-size: 1.5rem;
    line-height: 1;
    cursor: pointer;
    padding: var(--space-1);
    border-radius: var(--radius-sm);
  }
  .btn-close:hover {
    color: var(--text);
  }
  .modal-card__body {
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .status-banner {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-md);
    font-size: var(--text-xs);
  }
  .status-banner.running {
    background: var(--tint-warning);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, transparent);
    color: var(--warning-strong);
  }
  .status-banner.success {
    background: var(--tint-accent);
    border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
    color: var(--accent-strong);
  }
  .status-banner.failed {
    background: var(--tint-danger);
    border: 1px solid color-mix(in srgb, var(--danger) 30%, transparent);
    color: var(--danger-strong);
  }
  .status-msg {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
  }
  .log-terminal {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    font-size: var(--text-2xs);
    color: var(--text-secondary);
    max-height: 320px;
    overflow-y: auto;
    white-space: pre-wrap;
    word-break: break-all;
    margin: 0;
    line-height: var(--leading-relaxed);
  }
  .modal-card__foot {
    display: flex;
    justify-content: flex-end;
    padding: var(--space-3) var(--space-4);
    border-top: 1px solid var(--border-subtle);
  }

  /* Utility Helpers */
  .text-right { text-align: right; }
  .mono { font-family: var(--font-mono); }
  .font-semibold { font-weight: var(--weight-semibold); }
  .font-bold { font-weight: var(--weight-bold); }
  .text-accent { color: var(--accent-strong); }
  .text-muted { color: var(--text-muted); }
  .text-subtle { color: var(--text-subtle); }
  .text-danger { color: var(--danger-strong); }
  .text-warning { color: var(--warning-strong); }
  .text-xs { font-size: var(--text-xs); }
  .flex { display: flex; }
  .items-center { align-items: center; }
  .gap-xs { gap: var(--space-1); }
</style>
