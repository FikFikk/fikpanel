<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch, formatBytes } from '../lib/api';
  import { copyText } from '../lib/utils/browser';
  import type { Service } from '../lib/types';
  import NginxManager from '../lib/components/NginxManager.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import '../styles/pages/services.css';

  interface Props {
    services?: Service[];
    onSvcAction?: (unit: string, action: string) => void;
    onRefresh?: () => void;
    onNotify: (msg: string, type?: 'success' | 'error' | 'warning' | 'info') => void;
  }

  let { services = [], onSvcAction, onRefresh, onNotify }: Props = $props();

  // State data layanan
  interface ServiceCounts {
    total: number;
    running: number;
    stopped: number;
    failed: number;
    total_memory: number;
    categories?: Record<string, number>;
  }

  let allServicesList = $state<Service[]>([]);
  let counts = $state<ServiceCounts>({
    total: 0,
    running: 0,
    stopped: 0,
    failed: 0,
    total_memory: 0,
    categories: {},
  });
  let loading = $state<boolean>(false);
  let actionLoading = $state<string | null>(null);

  // State filter & pencarian
  let scope = $state<'running' | 'all'>('running');
  let selectedCategory = $state<string>('all');
  let searchQuery = $state<string>('');

  // Filtering reaktif otomatis: instan 0ms tanpa loading spinner atau kedipan network
  let displayedServices = $derived.by(() => {
    let list = allServicesList;
    if (selectedCategory !== 'all') {
      list = list.filter((s) => s.category === selectedCategory);
    }
    const q = searchQuery.trim().toLowerCase();
    if (q) {
      list = list.filter(
        (s) =>
          s.unit.toLowerCase().includes(q) ||
          (s.label && s.label.toLowerCase().includes(q)) ||
          (s.description && s.description.toLowerCase().includes(q))
      );
    }
    return list;
  });

  // State modal Nginx
  let showNginxManager = $state<boolean>(false);

  // State modal Journalctl Logs
  let showLogModal = $state<boolean>(false);
  let logUnit = $state<string>('');
  let logLines = $state<number>(100);
  let logContent = $state<string>('');
  let loadingLogs = $state<boolean>(false);
  let copyLogSuccess = $state<boolean>(false);

  // State modal Technical Inspect
  let showInspectModal = $state<boolean>(false);
  let inspectUnit = $state<string>('');
  let inspectProps = $state<Record<string, string>>({});
  let loadingInspect = $state<boolean>(false);

  // Daftar tab kategori sistem
  const CATEGORIES = [
    { id: 'all', label: 'Semua Layanan', icon: 'ri-apps-2-line' },
    { id: 'web', label: 'Web & App Stack', icon: 'ri-global-line' },
    { id: 'database', label: 'Database & Cache', icon: 'ri-database-2-line' },
    { id: 'security', label: 'Keamanan & Firewall', icon: 'ri-shield-keyhole-line' },
    { id: 'supervisor', label: 'Supervisor & Worker', icon: 'ri-cpu-line' },
    { id: 'system', label: 'Sistem & Daemon', icon: 'ri-server-line' },
  ];

  // Muat seluruh daftar layanan dalam cakupan (scope) aktif dari API backend
  async function loadServices(silent = false) {
    if (!silent) loading = true;
    try {
      const params = new URLSearchParams();
      params.set('scope', scope);

      const res = await apiFetch<{
        services: Service[];
        counts: ServiceCounts;
        scope: string;
      }>(`/api/services?${params.toString()}`);

      allServicesList = res.services || [];
      if (res.counts) {
        counts = res.counts;
      }
    } catch (err) {
      onNotify(`Gagal memuat layanan: ${(err as Error).message}`, 'error');
    } finally {
      loading = false;
    }
  }

  // Eksekusi aksi lifecycle (start, stop, restart, reload, enable, disable)
  async function executeAction(unit: string, action: string) {
    actionLoading = `${unit}:${action}`;
    try {
      await apiFetch(`/api/services/${encodeURIComponent(unit)}/action`, {
        method: 'POST',
        body: JSON.stringify({ action }),
      });

      const actionLabels: Record<string, string> = {
        start: 'dijalankan',
        stop: 'dihentikan',
        restart: 'dimuat ulang (restart)',
        reload: 'dikonfigurasi ulang (reload)',
        enable: 'diaktifkan saat boot',
        disable: 'dinonaktifkan saat boot',
      };
      const actDesc = actionLabels[action] || action;
      onNotify(`Layanan ${unit} berhasil ${actDesc}`, 'success');

      // Refresh data lokal dan sinkronkan dengan parent overview
      await loadServices(true);
      if (onRefresh) onRefresh();
    } catch (err) {
      onNotify(`Gagal mengeksekusi ${action} pada ${unit}: ${(err as Error).message}`, 'error');
    } finally {
      actionLoading = null;
    }
  }

  // Buka modal log journalctl
  async function openLogModal(unit: string) {
    logUnit = unit;
    showLogModal = true;
    await fetchLogs();
  }

  // Ambil log journalctl real-time dari backend
  async function fetchLogs() {
    if (!logUnit) return;
    loadingLogs = true;
    try {
      const res = await apiFetch<{ ok: boolean; logs: string }>(
        `/api/services/${encodeURIComponent(logUnit)}/logs?lines=${logLines}`
      );
      logContent = res.logs || '(Tidak ada catatan log ditemukan untuk unit ini)';
    } catch (err) {
      logContent = `Gagal mengambil log: ${(err as Error).message}`;
    } finally {
      loadingLogs = false;
    }
  }

  // Salin teks log ke clipboard (mendukung HTTP non-HTTPS via fallback textarea)
  async function copyLogs() {
    if (!logContent) return;
    try {
      await copyText(logContent);
      copyLogSuccess = true;
      onNotify('Log berhasil disalin ke clipboard', 'success');
      setTimeout(() => {
        copyLogSuccess = false;
      }, 2000);
    } catch (err) {
      onNotify('Gagal menyalin log ke clipboard', 'error');
    }
  }

  // Buka modal inspeksi teknis metadata
  async function openInspectModal(unit: string) {
    inspectUnit = unit;
    showInspectModal = true;
    loadingInspect = true;
    inspectProps = {};
    try {
      const res = await apiFetch<{ ok: boolean; properties: Record<string, string> }>(
        `/api/services/${encodeURIComponent(unit)}/inspect`
      );
      inspectProps = res.properties || {};
    } catch (err) {
      onNotify(`Gagal memeriksa metadata unit: ${(err as Error).message}`, 'error');
    } finally {
      loadingInspect = false;
    }
  }

  // Hitung jumlah unit per kategori untuk label pill secara konsisten dan independen
  function getCategoryCount(catId: string): number {
    if (counts.categories && typeof counts.categories[catId] === 'number') {
      return counts.categories[catId];
    }
    if (catId === 'all') return counts.total || allServicesList.length;
    return allServicesList.filter((s) => s.category === catId).length;
  }

  // Muat data saat komponen pertama kali diinisialisasi
  onMount(() => {
    loadServices();
  });
</script>

<div class="pg-services services-container">
  <!-- Baris Judul & Aksi Global -->
  <div class="page-header">
    <div>
      <div class="title-with-pill">
        <h2 class="page-header__title">Layanan Sistem &amp; Supervisor</h2>
        <span class="badge badge--success badge--pill">Systemd Linux</span>
      </div>
      <p class="page-header__sub">
        Pantau seluruh proses latar belakang host, supervisor worker, status boot startup, dan log real-time
      </p>
    </div>

    <div class="page-header__actions">
      <!-- Pengatur Cakupan Layanan (Scope Switcher) -->
      <div class="tabs tabs--pill" role="group" aria-label="Cakupan Layanan">
        <button
          type="button"
          class="tabs__tab"
          class:is-active={scope === 'running'}
          onclick={() => {
            scope = 'running';
            loadServices();
          }}
          title="Tampilkan hanya layanan yang aktif berjalan pada memori"
        >
          <span class="dot dot--success"></span>
          Hanya Berjalan
        </button>
        <button
          type="button"
          class="tabs__tab"
          class:is-active={scope === 'all'}
          onclick={() => {
            scope = 'all';
            loadServices();
          }}
          title="Tampilkan semua unit layanan yang terpasang di sistem operasi"
        >
          <i class="ri-list-check-2"></i>
          Semua Terpasang
        </button>
      </div>

      <button
        class="btn btn--secondary btn--sm"
        onclick={() => loadServices()}
        disabled={loading}
        title="Muat ulang status layanan sekarang"
      >
        {#if loading}
          <span class="spinner"></span>
        {:else}
          <i class="ri-refresh-line"></i>
        {/if}
        Refresh
      </button>
    </div>
  </div>

  <!-- Ringkasan Statistik Metrik Cepat -->
  <div class="stats-grid">
    <div class="stat-card">
      <div class="stat-card__icon">
        <i class="ri-server-line"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Total Terdeteksi</span>
        <span class="stat-card__value">{counts.total}</span>
        <span class="stat-card__sub">Unit layanan sistem</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon">
        <i class="ri-play-circle-line"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Aktif Berjalan</span>
        <span class="stat-card__value text-accent">{counts.running}</span>
        <span class="stat-card__sub">Proses runtime aktif</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon">
        <i class="ri-stop-circle-line"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Nonaktif / Exited</span>
        <span class="stat-card__value text-muted">{counts.stopped}</span>
        <span class="stat-card__sub">Layanan berhenti</span>
      </div>
    </div>

    <div class="stat-card {counts.failed > 0 ? 'card-failed-alert' : ''}">
      <div class="stat-card__icon stat-card__icon--danger">
        <i class="ri-error-warning-line"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Bermasalah (Failed)</span>
        <span class="stat-card__value {counts.failed > 0 ? 'text-danger' : 'text-muted'}">{counts.failed}</span>
        <span class="stat-card__sub">{counts.failed > 0 ? 'Perlu tindakan segera!' : 'Seluruh layanan stabil'}</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--info">
        <i class="ri-dashboard-3-line"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Total RAM Layanan</span>
        <span class="stat-card__value text-info mono">{formatBytes(counts.total_memory)}</span>
        <span class="stat-card__sub">Akumulasi penggunaan RSS</span>
      </div>
    </div>
  </div>

  <!-- Panel Utama: Tab Kategori, Pencarian, & Tabel Layanan -->
  <div class="card services-panel">
    <!-- Baris Filter Kategori & Pencarian -->
    <div class="filter-bar">
      <div class="tabs tabs--pill">
        {#each CATEGORIES as cat}
          <button
            type="button"
            class="tabs__tab"
            class:is-active={selectedCategory === cat.id}
            onclick={() => {
              selectedCategory = cat.id;
            }}
          >
            <i class="{cat.icon}"></i>
            <span>{cat.label}</span>
            <span class="cat-count-pill">{getCategoryCount(cat.id)}</span>
          </button>
        {/each}
      </div>

      <div class="input-wrap search-box">
        <i class="ri-search-line input__icon"></i>
        <input
          type="text"
          placeholder="Cari nama unit, deskripsi, atau proses..."
          bind:value={searchQuery}
          class="input input--search"
        />
        {#if searchQuery}
          <button class="btn btn--icon btn--sm btn--ghost search-clear" onclick={() => { searchQuery = ''; }} aria-label="Bersihkan pencarian">
            <i class="ri-close-line"></i>
          </button>
        {/if}
      </div>
    </div>

    <!-- Tabel Data Layanan Super Detail -->
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th class="col-unit">Unit / Daemon</th>
            <th class="col-cat">Kategori</th>
            <th class="col-status">Status Aktif</th>
            <th class="col-boot">Boot Startup</th>
            <th class="col-pid">PID / Thread</th>
            <th class="col-ram">Penggunaan RAM</th>
            <th class="col-uptime">Uptime</th>
            <th class="text-right col-actions">Aksi Kontrol</th>
          </tr>
        </thead>
        <tbody>
          {#if loading && allServicesList.length === 0}
            <tr>
              <td colspan="8">
                <div class="state state--loading">
                  <span class="spinner"></span>
                  <p class="state__desc">Memindai seluruh unit layanan sistem...</p>
                </div>
              </td>
            </tr>
          {:else if displayedServices.length === 0}
            <tr>
              <td colspan="8">
                <div class="state state--empty">
                  <i class="ri-inbox-line"></i>
                  <p class="state__desc">Tidak ada unit layanan yang sesuai dengan kriteria filter ({selectedCategory !== 'all' ? selectedCategory : 'semua'}).</p>
                </div>
              </td>
            </tr>
          {:else}
            {#each displayedServices as s (s.unit)}
              <tr class="{s.active_state === 'failed' ? 'row-failed' : ''}">
                <!-- Kolom Unit & Label Deskriptif -->
                <td>
                  <div class="unit-cell">
                    <div class="unit-title-row">
                      {#if s.unit === 'nginx.service'}
                        <button
                          class="unit-link mono font-bold"
                          onclick={() => (showNginxManager = true)}
                          title="Buka panel konfigurasi Nginx Manager"
                        >
                          {s.unit} <i class="ri-settings-4-line"></i>
                        </button>
                      {:else}
                        <span class="mono font-bold">{s.unit}</span>
                      {/if}

                      {#if s.protected}
                        <span class="badge badge--warning" title="Unit vital sistem: dilindungi dari penghentian paksa">
                          <i class="ri-shield-fill"></i> PROTECTED
                        </span>
                      {/if}

                      {#if s.category === 'supervisor'}
                        <span class="badge badge--violet" title="Supervisor worker daemon">
                          <i class="ri-cpu-line"></i> SUPERVISOR
                        </span>
                      {/if}
                    </div>

                    <div class="unit-desc-row text-muted" title={s.description || s.label}>
                      {s.label || s.description || 'Layanan latar belakang sistem'}
                    </div>

                    {#if s.exec_start}
                      <div class="unit-cmd-row mono text-subtle" title={s.exec_start}>
                        <i class="ri-terminal-line"></i> {s.exec_start}
                      </div>
                    {/if}
                  </div>
                </td>

                <!-- Kolom Kategori -->
                <td>
                  <span class="category-pill cat-{s.category || 'system'}">
                    {#if s.category === 'web'}
                      <i class="ri-global-line"></i>
                    {:else if s.category === 'database'}
                      <i class="ri-database-2-line"></i>
                    {:else if s.category === 'security'}
                      <i class="ri-shield-keyhole-line"></i>
                    {:else if s.category === 'supervisor'}
                      <i class="ri-cpu-line"></i>
                    {:else}
                      <i class="ri-server-line"></i>
                    {/if}
                    {s.category_label || 'Sistem'}
                  </span>
                </td>

                <!-- Kolom Status Aktif -->
                <td>
                  {#if s.active_state === 'active'}
                    <span class="badge badge--success">
                      <span class="dot dot--success"></span>
                      active ({s.sub_state})
                    </span>
                  {:else if s.active_state === 'failed'}
                    <span class="badge badge--danger">
                      <span class="dot dot--danger dot--pulse"></span>
                      failed ({s.sub_state})
                    </span>
                  {:else if s.active_state === 'activating'}
                    <span class="badge badge--warning">
                      <span class="dot dot--warning"></span>
                      activating
                    </span>
                  {:else}
                    <span class="badge badge--neutral">
                      <span class="dot"></span>
                      {s.active_state} ({s.sub_state})
                    </span>
                  {/if}
                </td>

                <!-- Kolom Boot Startup (UnitFileState) & Toggle -->
                <td>
                  <div class="boot-cell">
                    {#if s.unit_file_state === 'enabled'}
                      <button
                        class="boot-badge boot-enabled"
                        onclick={() => executeAction(s.unit, 'disable')}
                        disabled={s.protected || actionLoading === `${s.unit}:disable`}
                        title={s.protected ? 'Unit dilindungi: tidak dapat dinonaktifkan dari startup' : 'Klik untuk menonaktifkan startup saat boot'}
                      >
                        <i class="ri-check-line"></i> enabled
                      </button>
                    {:else if s.unit_file_state === 'disabled'}
                      <button
                        class="boot-badge boot-disabled"
                        onclick={() => executeAction(s.unit, 'enable')}
                        disabled={actionLoading === `${s.unit}:enable`}
                        title="Klik untuk mengaktifkan otomatis saat server boot"
                      >
                        <i class="ri-close-line"></i> disabled
                      </button>
                    {:else if s.unit_file_state}
                      <span class="boot-badge boot-static" title="Dikelola oleh dependensi static/masked">
                        {s.unit_file_state}
                      </span>
                    {:else}
                      <span class="text-subtle">—</span>
                    {/if}
                  </div>
                </td>

                <!-- Kolom PID & Tasks / Threads -->
                <td>
                  <div class="pid-cell">
                    <span class="mono font-bold">{s.main_pid > 0 ? s.main_pid : '—'}</span>
                    {#if s.tasks_current && s.tasks_current > 0}
                      <span class="tasks-pill" title="{s.tasks_current} worker threads / subtasks aktif">
                        <i class="ri-git-fork-line"></i> {s.tasks_current} thr
                      </span>
                    {/if}
                  </div>
                </td>

                <!-- Kolom Penggunaan RAM -->
                <td>
                  <div class="ram-cell">
                    <span class="mono font-bold">{formatBytes(s.memory_current)}</span>
                    {#if s.memory_peak && s.memory_peak > s.memory_current}
                      <span class="ram-peak mono" title="Puncak penggunaan memori tercatat">
                        Puncak: {formatBytes(s.memory_peak)}
                      </span>
                    {/if}
                  </div>
                </td>

                <!-- Kolom Uptime -->
                <td>
                  <span class="uptime-text">{s.uptime || '—'}</span>
                </td>

                <!-- Kolom Aksi Kontrol -->
                <td class="text-right">
                  <div class="actions-cell">
                    <!-- Tombol Restart -->
                    <button
                      class="btn btn--sm btn--ghost"
                      onclick={() => executeAction(s.unit, 'restart')}
                      disabled={actionLoading === `${s.unit}:restart`}
                      title="Muat ulang proses layanan (restart)"
                    >
                      {#if actionLoading === `${s.unit}:restart`}
                        <span class="spinner"></span>
                      {:else}
                        <i class="ri-restart-line"></i>
                      {/if}
                      Restart
                    </button>

                    <!-- Tombol Start / Stop -->
                    {#if s.active_state === 'active'}
                      <button
                        class="btn btn--sm btn--danger"
                        onclick={() => executeAction(s.unit, 'stop')}
                        disabled={s.protected || actionLoading === `${s.unit}:stop`}
                        title={s.protected ? 'Layanan vital sistem tidak boleh dihentikan' : 'Hentikan layanan sekarang'}
                      >
                        {#if actionLoading === `${s.unit}:stop`}
                          <span class="spinner"></span>
                        {:else}
                          <i class="ri-stop-fill"></i>
                        {/if}
                        Stop
                      </button>
                    {:else}
                      <button
                        class="btn btn--sm btn--primary"
                        onclick={() => executeAction(s.unit, 'start')}
                        disabled={actionLoading === `${s.unit}:start`}
                        title="Jalankan layanan sekarang"
                      >
                        {#if actionLoading === `${s.unit}:start`}
                          <span class="spinner"></span>
                        {:else}
                          <i class="ri-play-fill"></i>
                        {/if}
                        Start
                      </button>
                    {/if}

                    <!-- Tombol Lihat Log Journalctl -->
                    <button
                      class="btn btn--sm btn--secondary"
                      onclick={() => openLogModal(s.unit)}
                      title="Buka log journalctl real-time"
                    >
                      <i class="ri-file-list-2-line"></i>
                      Log
                    </button>

                    <!-- Tombol Inspeksi Metadata Teknis -->
                    <button
                      class="btn btn--sm btn--icon btn--secondary"
                      onclick={() => openInspectModal(s.unit)}
                      title="Periksa metadata teknis systemctl"
                      aria-label="Inspeksi teknis {s.unit}"
                    >
                      <i class="ri-information-line"></i>
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>

  <!-- Modal Nginx Manager Khusus -->
  {#if showNginxManager}
    <NginxManager
      onClose={() => {
        showNginxManager = false;
        loadServices();
      }}
      {onNotify}
    />
  {/if}

  <!-- Modal Live Journalctl Logs -->
  <Modal
    open={showLogModal}
    title={`Log Real-time: ${logUnit}`}
    size="lg"
    onclose={() => (showLogModal = false)}
  >
    <div class="log-toolbar">
      <span class="text-subtle text-xs">Jurnal log peristiwa sistem via journalctl</span>
      <div class="log-tools">
        <div class="limit-selector">
          <span class="text-xs text-muted">Baris:</span>
          <select
            bind:value={logLines}
            onchange={() => fetchLogs()}
            class="input select-sm"
            disabled={loadingLogs}
          >
            <option value={50}>50</option>
            <option value={100}>100</option>
            <option value={200}>200</option>
            <option value={500}>500</option>
          </select>
        </div>

        <button
          class="btn btn--secondary btn--sm"
          onclick={copyLogs}
          disabled={loadingLogs || !logContent}
          title="Salin seluruh isi log ke clipboard"
        >
          <i class="{copyLogSuccess ? 'ri-check-line text-accent' : 'ri-file-copy-line'}"></i>
          {copyLogSuccess ? 'Tersalin!' : 'Salin'}
        </button>

        <button
          class="btn btn--secondary btn--sm"
          onclick={() => fetchLogs()}
          disabled={loadingLogs}
          title="Segarkan log"
        >
          {#if loadingLogs}
            <span class="spinner"></span>
          {:else}
            <i class="ri-refresh-line"></i>
          {/if}
          Refresh
        </button>
      </div>
    </div>

    <div class="log-terminal">
      {#if loadingLogs}
        <div class="state state--loading">
          <span class="spinner"></span>
          <span class="state__desc">Membaca log journalctl dari host sistem...</span>
        </div>
      {:else}
        <pre class="log-pre">{logContent}</pre>
      {/if}
    </div>

    {#snippet footer()}
      <div class="modal-foot-row">
        <span class="text-subtle text-xs">
          Menampilkan catatan eksekusi, runtime output, dan sinyal error unit {logUnit}.
        </span>
        <button class="btn btn--secondary" onclick={() => (showLogModal = false)}>Tutup</button>
      </div>
    {/snippet}
  </Modal>

  <!-- Modal Inspeksi Metadata Teknis (Systemctl Show) -->
  <Modal
    open={showInspectModal}
    title={`Inspeksi Teknis: ${inspectUnit}`}
    size="md"
    onclose={() => (showInspectModal = false)}
  >
    {#if loadingInspect}
      <div class="state state--loading">
        <span class="spinner"></span>
        <span class="state__desc">Mengambil konfigurasi teknis unit...</span>
      </div>
    {:else}
      <div class="inspect-grid">
        <div class="inspect-item full-width">
          <span class="inspect-key">ExecStart (Perintah Utama)</span>
          <span class="inspect-val mono code-snippet">{inspectProps.ExecStart || '—'}</span>
        </div>

        <div class="inspect-item">
          <span class="inspect-key">Main PID</span>
          <span class="inspect-val mono">{inspectProps.MainPID || '—'}</span>
        </div>

        <div class="inspect-item">
          <span class="inspect-key">Tasks Current</span>
          <span class="inspect-val mono">{inspectProps.TasksCurrent || '—'}</span>
        </div>

        <div class="inspect-item">
          <span class="inspect-key">Unit File State</span>
          <span class="inspect-val mono">{inspectProps.UnitFileState || '—'}</span>
        </div>

        <div class="inspect-item">
          <span class="inspect-key">Restart Policy</span>
          <span class="inspect-val mono">{inspectProps.Restart || '—'}</span>
        </div>

        <div class="inspect-item">
          <span class="inspect-key">Load State</span>
          <span class="inspect-val mono">{inspectProps.LoadState || '—'}</span>
        </div>

        <div class="inspect-item">
          <span class="inspect-key">Active &amp; Sub State</span>
          <span class="inspect-val mono">{inspectProps.ActiveState} ({inspectProps.SubState})</span>
        </div>

        <div class="inspect-item full-width">
          <span class="inspect-key">Active Enter Timestamp</span>
          <span class="inspect-val mono">{inspectProps.ActiveEnterTimestamp || '—'}</span>
        </div>

        <div class="inspect-item full-width">
          <span class="inspect-key">Unit File Path</span>
          <span class="inspect-val mono text-subtle">{inspectProps.FragmentPath || `/etc/systemd/system/${inspectUnit}`}</span>
        </div>
      </div>
    {/if}

    {#snippet footer()}
      <div class="modal-foot-row">
        <span class="text-subtle text-xs">Data diekstrak langsung dari bus systemd cgroup kernel.</span>
        <button class="btn btn--secondary" onclick={() => (showInspectModal = false)}>Tutup</button>
      </div>
    {/snippet}
  </Modal>
</div>