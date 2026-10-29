<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch, formatBytes } from '../lib/api';
  import { copyText } from '../lib/utils/browser';
  import type { Service } from '../lib/types';
  import NginxManager from '../lib/components/NginxManager.svelte';

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

<div class="services-container">
  <!-- Baris Judul & Aksi Global -->
  <div class="head-bar">
    <div class="head-title-wrap">
      <div class="title-with-pill">
        <h2 class="title">Layanan Sistem &amp; Supervisor</h2>
        <span class="system-badge">Systemd Linux</span>
      </div>
      <p class="subtitle">
        Pantau seluruh proses latar belakang host, supervisor worker, status boot startup, dan log real-time
      </p>
    </div>

    <div class="head-actions">
      <!-- Pengatur Cakupan Layanan (Scope Switcher) -->
      <div class="scope-switch" role="group" aria-label="Cakupan Layanan">
        <button
          type="button"
          class="scope-btn {scope === 'running' ? 'active' : ''}"
          onclick={() => {
            scope = 'running';
            loadServices();
          }}
          title="Tampilkan hanya layanan yang aktif berjalan pada memori"
        >
          <span class="dot-indicator green"></span>
          Hanya Berjalan
        </button>
        <button
          type="button"
          class="scope-btn {scope === 'all' ? 'active' : ''}"
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
        class="btn btn-ghost refresh-btn"
        onclick={() => loadServices()}
        disabled={loading}
        title="Muat ulang status layanan sekarang"
      >
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i>
        Refresh
      </button>
    </div>
  </div>

  <!-- Ringkasan Statistik Metrik Cepat -->
  <div class="stats-grid">
    <div class="stat-card">
      <div class="stat-icon icon-total">
        <i class="ri-server-line"></i>
      </div>
      <div class="stat-info">
        <span class="stat-label">Total Terdeteksi</span>
        <span class="stat-value">{counts.total}</span>
        <span class="stat-desc">Unit layanan sistem</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-icon icon-running">
        <i class="ri-play-circle-line"></i>
      </div>
      <div class="stat-info">
        <span class="stat-label">Aktif Berjalan</span>
        <span class="stat-value text-accent">{counts.running}</span>
        <span class="stat-desc">Proses runtime aktif</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-icon icon-stopped">
        <i class="ri-stop-circle-line"></i>
      </div>
      <div class="stat-info">
        <span class="stat-label">Nonaktif / Exited</span>
        <span class="stat-value text-muted">{counts.stopped}</span>
        <span class="stat-desc">Layanan berhenti</span>
      </div>
    </div>

    <div class="stat-card {counts.failed > 0 ? 'card-failed-alert' : ''}">
      <div class="stat-icon icon-failed">
        <i class="ri-error-warning-line"></i>
      </div>
      <div class="stat-info">
        <span class="stat-label">Bermasalah (Failed)</span>
        <span class="stat-value {counts.failed > 0 ? 'text-danger' : 'text-muted'}">{counts.failed}</span>
        <span class="stat-desc">{counts.failed > 0 ? 'Perlu tindakan segera!' : 'Seluruh layanan stabil'}</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-icon icon-ram">
        <i class="ri-dashboard-3-line"></i>
      </div>
      <div class="stat-info">
        <span class="stat-label">Total RAM Layanan</span>
        <span class="stat-value text-info mono">{formatBytes(counts.total_memory)}</span>
        <span class="stat-desc">Akumulasi penggunaan RSS</span>
      </div>
    </div>
  </div>

  <!-- Panel Utama: Tab Kategori, Pencarian, & Tabel Layanan -->
  <div class="panel">
    <!-- Baris Filter Kategori & Pencarian -->
    <div class="filter-bar">
      <div class="tabs-scroll">
        {#each CATEGORIES as cat}
          <button
            type="button"
            class="cat-tab {selectedCategory === cat.id ? 'active' : ''}"
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

      <div class="search-box">
        <i class="ri-search-line search-icon"></i>
        <input
          type="text"
          placeholder="Cari nama unit, deskripsi, atau proses..."
          bind:value={searchQuery}
          class="search-input"
        />
        {#if searchQuery}
          <button class="search-clear" onclick={() => { searchQuery = ''; }}>
            <i class="ri-close-line"></i>
          </button>
        {/if}
      </div>
    </div>

    <!-- Tabel Data Layanan Super Detail -->
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th style="min-width: 220px;">Unit / Daemon</th>
            <th style="width: 140px;">Kategori</th>
            <th style="width: 150px;">Status Aktif</th>
            <th style="width: 130px;">Boot Startup</th>
            <th style="width: 110px;">PID / Thread</th>
            <th style="width: 140px;">Penggunaan RAM</th>
            <th style="width: 100px;">Uptime</th>
            <th style="text-align: right; min-width: 220px;">Aksi Kontrol</th>
          </tr>
        </thead>
        <tbody>
          {#if loading && allServicesList.length === 0}
            <tr>
              <td colspan="8" class="empty-state">
                <i class="ri-loader-4-line spin"></i>
                <p>Memindai seluruh unit layanan sistem...</p>
              </td>
            </tr>
          {:else if displayedServices.length === 0}
            <tr>
              <td colspan="8" class="empty-state">
                <i class="ri-inbox-line"></i>
                <p>Tidak ada unit layanan yang sesuai dengan kriteria filter ({selectedCategory !== 'all' ? selectedCategory : 'semua'}).</p>
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
                        <span class="mono font-bold text-main">{s.unit}</span>
                      {/if}

                      {#if s.protected}
                        <span class="shield-badge" title="Unit vital sistem: dilindungi dari penghentian paksa">
                          <i class="ri-shield-fill"></i> PROTECTED
                        </span>
                      {/if}

                      {#if s.category === 'supervisor'}
                        <span class="supervisor-badge" title="Supervisor worker daemon">
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
                    <span class="badge badge-ok">
                      <span class="dot-status dot-green"></span>
                      active ({s.sub_state})
                    </span>
                  {:else if s.active_state === 'failed'}
                    <span class="badge badge-fail">
                      <span class="dot-status dot-red pulse-dot"></span>
                      failed ({s.sub_state})
                    </span>
                  {:else if s.active_state === 'activating'}
                    <span class="badge badge-warn">
                      <span class="dot-status dot-amber"></span>
                      activating
                    </span>
                  {:else}
                    <span class="badge badge-inactive">
                      <span class="dot-status dot-gray"></span>
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
                    <span class="mono font-bold text-main">{formatBytes(s.memory_current)}</span>
                    {#if s.memory_peak && s.memory_peak > s.memory_current}
                      <span class="ram-peak text-subtle mono" title="Puncak penggunaan memori tercatat">
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
                <td style="text-align: right;">
                  <div class="actions-cell">
                    <!-- Tombol Restart -->
                    <button
                      class="btn-sm btn-ghost"
                      onclick={() => executeAction(s.unit, 'restart')}
                      disabled={actionLoading === `${s.unit}:restart`}
                      title="Muat ulang proses layanan (restart)"
                    >
                      <i class="ri-restart-line {actionLoading === `${s.unit}:restart` ? 'spin' : ''}"></i>
                      Restart
                    </button>

                    <!-- Tombol Start / Stop -->
                    {#if s.active_state === 'active'}
                      <button
                        class="btn-sm btn-danger"
                        onclick={() => executeAction(s.unit, 'stop')}
                        disabled={s.protected || actionLoading === `${s.unit}:stop`}
                        title={s.protected ? 'Layanan vital sistem tidak boleh dihentikan' : 'Hentikan layanan sekarang'}
                      >
                        <i class="ri-stop-fill {actionLoading === `${s.unit}:stop` ? 'spin' : ''}"></i>
                        Stop
                      </button>
                    {:else}
                      <button
                        class="btn-sm btn-accent"
                        onclick={() => executeAction(s.unit, 'start')}
                        disabled={actionLoading === `${s.unit}:start`}
                        title="Jalankan layanan sekarang"
                      >
                        <i class="ri-play-fill {actionLoading === `${s.unit}:start` ? 'spin' : ''}"></i>
                        Start
                      </button>
                    {/if}

                    <!-- Tombol Lihat Log Journalctl -->
                    <button
                      class="btn-sm btn-tool"
                      onclick={() => openLogModal(s.unit)}
                      title="Buka log journalctl real-time"
                    >
                      <i class="ri-file-list-2-line"></i>
                      Log
                    </button>

                    <!-- Tombol Inspeksi Metadata Teknis -->
                    <button
                      class="btn-sm btn-tool"
                      onclick={() => openInspectModal(s.unit)}
                      title="Periksa metadata teknis systemctl"
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
{#if showLogModal}
  <div class="modal-backdrop" onclick={() => (showLogModal = false)} role="presentation">
    <div class="modal-card modal-lg" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <h2 class="modal-title">
            <i class="ri-terminal-box-line"></i> Log Real-time: <span class="mono text-accent">{logUnit}</span>
          </h2>
          <span class="text-subtle text-xs">Jurnal log peristiwa sistem via journalctl</span>
        </div>

        <div class="modal-head-tools">
          <div class="limit-selector">
            <span class="text-xs text-muted">Baris:</span>
            <select
              bind:value={logLines}
              onchange={() => fetchLogs()}
              class="select-sm"
              disabled={loadingLogs}
            >
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={200}>200</option>
              <option value={500}>500</option>
            </select>
          </div>

          <button
            class="btn btn-ghost btn-sm"
            onclick={copyLogs}
            disabled={loadingLogs || !logContent}
            title="Salin seluruh isi log ke clipboard"
          >
            <i class="{copyLogSuccess ? 'ri-check-line text-accent' : 'ri-file-copy-line'}"></i>
            {copyLogSuccess ? 'Tersalin!' : 'Salin'}
          </button>

          <button
            class="btn btn-ghost btn-sm"
            onclick={() => fetchLogs()}
            disabled={loadingLogs}
            title="Segarkan log"
          >
            <i class="ri-refresh-line {loadingLogs ? 'spin' : ''}"></i>
            Refresh
          </button>

          <button class="modal-close" onclick={() => (showLogModal = false)} aria-label="Tutup modal">
            <i class="ri-close-line"></i>
          </button>
        </div>
      </div>

      <div class="modal-body p-0">
        <div class="log-terminal">
          {#if loadingLogs}
            <div class="log-loading">
              <i class="ri-loader-4-line spin"></i> Membaca log journalctl dari host sistem...
            </div>
          {:else}
            <pre class="log-pre">{logContent}</pre>
          {/if}
        </div>
      </div>

      <div class="modal-foot">
        <span class="text-subtle text-xs">
          Menampilkan catatan eksekusi, runtime output, dan sinyal error unit {logUnit}.
        </span>
        <button class="btn btn-ghost" onclick={() => (showLogModal = false)}>Tutup</button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal Inspeksi Metadata Teknis (Systemctl Show) -->
{#if showInspectModal}
  <div class="modal-backdrop" onclick={() => (showInspectModal = false)} role="presentation">
    <div class="modal-card modal-md" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <h2 class="modal-title">
            <i class="ri-information-line"></i> Inspeksi Teknis: <span class="mono text-accent">{inspectUnit}</span>
          </h2>
          <span class="text-subtle text-xs">Informasi arsitektur runtime dari systemctl show</span>
        </div>

        <button class="modal-close" onclick={() => (showInspectModal = false)} aria-label="Tutup modal">
          <i class="ri-close-line"></i>
        </button>
      </div>

      <div class="modal-body">
        {#if loadingInspect}
          <div class="inspect-loading">
            <i class="ri-loader-4-line spin"></i> Mengambil konfigurasi teknis unit...
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
      </div>

      <div class="modal-foot">
        <span class="text-subtle text-xs">Data diekstrak langsung dari bus systemd cgroup kernel.</span>
        <button class="btn btn-ghost" onclick={() => (showInspectModal = false)}>Tutup</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .services-container {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  /* ═══ Header Bar ═══ */
  .head-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
  }
  .head-title-wrap {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .title-with-pill {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .title {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--text);
    letter-spacing: -0.01em;
  }
  .system-badge {
    font-size: 0.68rem;
    font-weight: 700;
    padding: 0.15rem 0.5rem;
    border-radius: 9999px;
    background: color-mix(in srgb, var(--accent) 12%, transparent);
    color: var(--accent);
    border: 1px solid color-mix(in srgb, var(--accent) 25%, transparent);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .subtitle {
    font-size: 0.78rem;
    color: var(--text-muted);
  }
  .head-actions {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    flex-wrap: wrap;
  }

  /* ═══ Scope Switcher ═══ */
  .scope-switch {
    display: inline-flex;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.2rem;
    gap: 0.2rem;
  }
  .scope-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.35rem 0.75rem;
    border-radius: 6px;
    font-size: 0.75rem;
    font-weight: 600;
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .scope-btn:hover {
    color: var(--text);
  }
  .scope-btn.active {
    background: var(--bg-raised);
    color: var(--text);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  }
  .dot-indicator {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  .dot-indicator.green {
    background: var(--accent);
    box-shadow: 0 0 6px var(--accent);
  }

  /* ═══ Stats Cards Grid ═══ */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
    gap: 0.85rem;
  }
  .stat-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 0.9rem 1rem;
    display: flex;
    align-items: center;
    gap: 0.85rem;
    transition: transform 0.15s ease, border-color 0.15s ease;
  }
  .stat-card:hover {
    border-color: color-mix(in srgb, var(--accent) 30%, var(--border));
  }
  .card-failed-alert {
    border-color: color-mix(in srgb, var(--danger-strong) 40%, var(--border));
    background: color-mix(in srgb, var(--danger-strong) 5%, var(--bg-surface));
  }
  .stat-icon {
    width: 40px;
    height: 40px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.2rem;
    flex-shrink: 0;
  }
  .icon-total {
    background: color-mix(in srgb, var(--text-muted) 15%, transparent);
    color: var(--text);
  }
  .icon-running {
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    color: var(--accent);
  }
  .icon-stopped {
    background: color-mix(in srgb, var(--text-subtle) 15%, transparent);
    color: var(--text-subtle);
  }
  .icon-failed {
    background: color-mix(in srgb, var(--danger-strong) 15%, transparent);
    color: var(--danger-strong);
  }
  .icon-ram {
    background: var(--tint-info);
    color: var(--info);
  }
  .stat-info {
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .stat-label {
    font-size: 0.68rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
  }
  .stat-value {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--text);
    line-height: 1.2;
    margin: 0.1rem 0;
  }
  .stat-desc {
    font-size: 0.68rem;
    color: var(--text-subtle);
    white-space: nowrap;
    text-overflow: ellipsis;
    overflow: hidden;
  }

  /* ═══ Panel Utama ═══ */
  .panel {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 1.15rem 1.35rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  /* ═══ Filter Bar & Tabs ═══ */
  .filter-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
    border-bottom: 1px solid var(--border-subtle);
    padding-bottom: 0.85rem;
  }
  .tabs-scroll {
    display: flex;
    gap: 0.4rem;
    overflow-x: auto;
    padding-bottom: 0.2rem;
  }
  .cat-tab {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.4rem 0.75rem;
    border-radius: 8px;
    font-size: 0.76rem;
    font-weight: 600;
    background: transparent;
    border: 1px solid transparent;
    color: var(--text-secondary);
    cursor: pointer;
    white-space: nowrap;
    transition: all 0.15s ease;
  }
  .cat-tab:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .cat-tab.active {
    background: var(--bg-raised);
    border-color: var(--border);
    color: var(--accent);
  }
  .cat-count-pill {
    font-size: 0.65rem;
    padding: 0.1rem 0.4rem;
    border-radius: 9999px;
    background: var(--bg-hover);
    color: var(--text-subtle);
  }
  .cat-tab.active .cat-count-pill {
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    color: var(--accent);
  }

  /* ═══ Search Box ═══ */
  .search-box {
    position: relative;
    display: flex;
    align-items: center;
    min-width: 260px;
    flex: 1;
    max-width: 360px;
  }
  .search-icon {
    position: absolute;
    left: 0.75rem;
    color: var(--text-subtle);
    pointer-events: none;
    font-size: 0.9rem;
  }
  .search-input {
    width: 100%;
    padding: 0.45rem 2rem 0.45rem 2.2rem;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--bg-base);
    color: var(--text);
    font-size: 0.78rem;
    outline: none;
    transition: border-color 0.15s ease;
  }
  .search-input:focus {
    border-color: var(--accent);
  }
  .search-clear {
    position: absolute;
    right: 0.5rem;
    background: none;
    border: none;
    color: var(--text-subtle);
    cursor: pointer;
    padding: 0.2rem;
    border-radius: 4px;
  }
  .search-clear:hover {
    color: var(--text);
  }

  /* ═══ Tabel Data ═══ */
  .table-wrap {
    overflow-x: auto;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }
  th {
    text-align: left;
    padding: 0.65rem 0.75rem;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-subtle);
    border-bottom: 1px solid var(--border);
  }
  td {
    padding: 0.75rem;
    border-bottom: 1px solid var(--border-subtle);
    vertical-align: middle;
  }
  tr.row-failed {
    background: color-mix(in srgb, var(--danger-strong) 4%, transparent);
  }

  /* ═══ Unit Cell ═══ */
  .unit-cell {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .unit-title-row {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .unit-link {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background: none;
    border: none;
    padding: 0;
    color: var(--accent-strong);
    font-size: inherit;
    cursor: pointer;
    border-bottom: 1px dashed color-mix(in srgb, var(--accent-strong) 40%, transparent);
  }
  .unit-link:hover {
    color: var(--accent-strong);
    border-bottom-style: solid;
  }
  .unit-desc-row {
    font-size: 0.72rem;
    max-width: 320px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .unit-cmd-row {
    font-size: 0.65rem;
    max-width: 320px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* ═══ Badges & Pills ═══ */
  .shield-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
    font-size: 0.6rem;
    font-weight: 700;
    padding: 0.1rem 0.35rem;
    border-radius: 4px;
    background: color-mix(in srgb, var(--warning) 12%, transparent);
    color: var(--warning);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, transparent);
  }
  .supervisor-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
    font-size: 0.6rem;
    font-weight: 700;
    padding: 0.1rem 0.35rem;
    border-radius: 4px;
    background: var(--tint-violet);
    color: var(--violet);
    border: 1px solid color-mix(in srgb, var(--violet) 30%, transparent);
  }
  .category-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    padding: 0.15rem 0.5rem;
    border-radius: 6px;
    font-size: 0.7rem;
    font-weight: 600;
    background: var(--bg-hover);
    color: var(--text-secondary);
  }
  .cat-web {
    background: var(--tint-info);
    color: var(--info);
  }
  .cat-database {
    background: var(--tint-warning);
    color: var(--warning-strong);
  }
  .cat-security {
    background: var(--tint-accent);
    color: var(--accent-strong);
  }
  .cat-supervisor {
    background: var(--tint-violet);
    color: var(--violet);
  }
  .cat-system {
    background: color-mix(in srgb, var(--text-subtle) 15%, transparent);
    color: var(--text-muted);
  }

  .badge {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.2rem 0.55rem;
    border-radius: 6px;
    font-size: 0.7rem;
    font-weight: 600;
  }
  .badge-ok {
    background: color-mix(in srgb, var(--accent) 10%, transparent);
    color: var(--accent);
    border: 1px solid color-mix(in srgb, var(--accent) 25%, transparent);
  }
  .badge-fail {
    background: color-mix(in srgb, var(--danger-strong) 12%, transparent);
    color: var(--danger-strong);
    border: 1px solid color-mix(in srgb, var(--danger-strong) 30%, transparent);
  }
  .badge-warn {
    background: color-mix(in srgb, var(--warning) 12%, transparent);
    color: var(--warning);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, transparent);
  }
  .badge-inactive {
    background: var(--bg-hover);
    color: var(--text-subtle);
    border: 1px solid var(--border);
  }
  .dot-status {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  .dot-green {
    background: var(--accent);
  }
  .dot-red {
    background: var(--danger-strong);
  }
  .dot-amber {
    background: var(--warning);
  }
  .dot-gray {
    background: var(--text-subtle);
  }
  .pulse-dot {
    animation: pulseRed 1.8s infinite;
  }
  @keyframes pulseRed {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.4; transform: scale(1.3); }
  }

  /* ═══ Boot Startup Badge & Toggle ═══ */
  .boot-cell {
    display: flex;
    align-items: center;
  }
  .boot-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.15rem 0.45rem;
    border-radius: 5px;
    font-size: 0.68rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
    transition: all 0.15s ease;
  }
  .boot-enabled {
    background: color-mix(in srgb, var(--accent) 12%, transparent);
    color: var(--accent);
    border: 1px solid color-mix(in srgb, var(--accent) 25%, transparent);
  }
  .boot-enabled:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 20%, transparent);
  }
  .boot-disabled {
    background: var(--bg-hover);
    color: var(--text-subtle);
    border: 1px solid var(--border);
  }
  .boot-disabled:hover:not(:disabled) {
    background: var(--bg-raised);
    color: var(--text);
  }
  .boot-static {
    font-size: 0.68rem;
    color: var(--text-subtle);
  }
  .boot-badge:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  /* ═══ PID & RAM Cells ═══ */
  .pid-cell {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .tasks-pill {
    font-size: 0.65rem;
    color: var(--text-subtle);
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
  }
  .ram-cell {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .ram-peak {
    font-size: 0.65rem;
  }
  .uptime-text {
    font-size: 0.75rem;
    color: var(--text-secondary);
  }

  /* ═══ Actions Cell ═══ */
  .actions-cell {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 0.35rem;
  }
  .btn {
    padding: 0.45rem 0.85rem;
    border-radius: 6px;
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    transition: background 0.15s ease;
  }
  .btn-ghost {
    background: var(--bg-raised);
    color: var(--text-secondary);
    border: 1px solid var(--border);
  }
  .btn-ghost:hover:not(:disabled) {
    background: var(--bg-hover);
    color: var(--text);
  }
  .btn-sm {
    padding: 0.25rem 0.55rem;
    border-radius: 5px;
    font-size: 0.72rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    transition: all 0.15s ease;
  }
  .btn-accent {
    background: var(--accent);
    color: var(--text-on-accent);
  }
  .btn-accent:hover:not(:disabled) {
    background: var(--accent-hover);
  }
  .btn-danger {
    background: color-mix(in srgb, var(--danger-strong) 12%, transparent);
    color: var(--danger-strong);
    border: 1px solid color-mix(in srgb, var(--danger-strong) 30%, transparent);
  }
  .btn-danger:hover:not(:disabled) {
    background: color-mix(in srgb, var(--danger-strong) 22%, transparent);
  }
  .btn-tool {
    background: var(--bg-hover);
    color: var(--text-muted);
    border: 1px solid var(--border);
  }
  .btn-tool:hover:not(:disabled) {
    background: var(--bg-raised);
    color: var(--text);
  }
  .btn-sm:disabled, .btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* ═══ Empty State ═══ */
  .empty-state {
    text-align: center;
    padding: 3rem 1rem !important;
    color: var(--text-subtle);
  }
  .empty-state i {
    font-size: 2rem;
    margin-bottom: 0.5rem;
    display: block;
  }

  /* ═══ Modal Generic ═══ */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.65);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1050;
    padding: 1.5rem;
  }
  .modal-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    width: 100%;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.4);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .modal-md {
    max-width: 650px;
  }
  .modal-lg {
    max-width: 850px;
  }
  .modal-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--border);
    background: var(--bg-raised);
  }
  .modal-title-wrap {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .modal-title {
    font-size: 1rem;
    font-weight: 700;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .modal-head-tools {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .limit-selector {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }
  .select-sm {
    background: var(--bg-base);
    color: var(--text);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 0.2rem 0.4rem;
    font-size: 0.72rem;
  }
  .modal-close {
    background: none;
    border: none;
    color: var(--text-subtle);
    font-size: 1.25rem;
    cursor: pointer;
    padding: 0.2rem;
    border-radius: 4px;
  }
  .modal-close:hover {
    color: var(--text);
  }
  .modal-body {
    padding: 1.25rem;
    max-height: 70vh;
    overflow-y: auto;
  }
  .modal-body.p-0 {
    padding: 0;
  }
  .modal-foot {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.85rem 1.25rem;
    border-top: 1px solid var(--border);
    background: var(--bg-raised);
  }

  /* ═══ Terminal Log Viewer ═══ */
  .log-terminal {
    background: var(--bg-inset);
    min-height: 400px;
    max-height: 550px;
    overflow-y: auto;
    padding: 1rem 1.25rem;
  }
  .log-pre {
    margin: 0;
    font-family: 'Fira Code', monospace;
    font-size: 0.75rem;
    line-height: 1.5;
    color: var(--text-secondary);
    white-space: pre-wrap;
    word-break: break-all;
  }
  .log-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    height: 350px;
    color: var(--text-subtle);
    font-size: 0.85rem;
  }

  /* ═══ Inspect Grid ═══ */
  .inspect-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    height: 200px;
    color: var(--text-subtle);
  }
  .inspect-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.85rem;
  }
  .inspect-item {
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.65rem 0.85rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .inspect-item.full-width {
    grid-column: 1 / -1;
  }
  .inspect-key {
    font-size: 0.68rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-subtle);
  }
  .inspect-val {
    font-size: 0.82rem;
    color: var(--text);
    word-break: break-all;
  }
  .code-snippet {
    background: var(--bg-hover);
    padding: 0.4rem 0.6rem;
    border-radius: 4px;
    border: 1px solid var(--border-subtle);
    font-size: 0.75rem;
  }

  /* ═══ Utilitas & Font ═══ */
  .mono {
    font-family: 'Fira Code', monospace;
  }
  .font-bold {
    font-weight: 700;
  }
  .text-main {
    color: var(--text);
  }
  .text-accent {
    color: var(--accent);
  }
  .text-muted {
    color: var(--text-muted);
  }
  .text-subtle {
    color: var(--text-subtle);
  }
  .text-danger {
    color: var(--danger-strong);
  }
  .text-info {
    color: var(--info);
  }
  .text-xs {
    font-size: 0.72rem;
  }
  .spin {
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>