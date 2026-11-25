<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch, formatBytes } from '../lib/api';
  import type {
    HostOverview,
    SystemProcess,
    SystemProcessDetail,
    PortListening,
    MemoryOptimizationStatus,
    CleanerScanResult,
    CleanReport,
    DiskHogsResult,
    UpdatesOverview,
    DiagnosticsReport,
  } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import PageHeader from '../lib/components/ui/PageHeader.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import OverviewTab from './system/OverviewTab.svelte';
  import ProcessesTab from './system/ProcessesTab.svelte';
  import NetworkTab from './system/NetworkTab.svelte';
  import MemoryTab from './system/MemoryTab.svelte';
  import CleanerTab, { type CleanSelectionPayload } from './system/CleanerTab.svelte';
  import DiagnosticsTab from './system/DiagnosticsTab.svelte';
  import '../styles/pages/systemmanager.css';

  type SystemTabId = 'overview' | 'processes' | 'network' | 'memory' | 'cleaner' | 'diagnostics';

  let activeTab = $state<SystemTabId>('overview');
  let busyKey = $state('');

  // State tab Ringkasan & Disk
  let overview = $state<HostOverview | null>(null);
  let loadingOverview = $state(false);

  // State tab Task Manager (Live Proses)
  let processes = $state<SystemProcess[]>([]);
  let loadingProcs = $state(false);
  let procSort = $state<'cpu' | 'mem' | 'pid'>('cpu');
  let procSearch = $state('');
  let procFilter = $state<'all' | 'nonroot' | 'zombie'>('all');
  let pollSeconds = $state(0);

  // State tab Jaringan & Port
  let ports = $state<PortListening[]>([]);
  let connections = $state<Record<string, number>>({});
  let loadingPorts = $state(false);

  // State tab Swap & Memori
  let memory = $state<MemoryOptimizationStatus | null>(null);
  let loadingMemory = $state(false);
  let lastFreedRAM = $state<number | null>(null);

  // State tab Disk Cleaner & Disk Hogs
  let scanResult = $state<CleanerScanResult | null>(null);
  let cleanReport = $state<CleanReport | null>(null);
  let diskHogs = $state<DiskHogsResult | null>(null);
  let scanning = $state(false);
  let cleaning = $state(false);
  let analyzingHogs = $state(false);

  // State tab Updates & Diagnostik
  let updates = $state<UpdatesOverview | null>(null);
  let diagnostics = $state<DiagnosticsReport | null>(null);
  let loadingUpdates = $state(false);
  let applyingUpdates = $state(false);
  let loadingDiag = $state(false);

  // State Modal Process Inspector (/proc/<pid>)
  let inspectorOpen = $state(false);
  let inspectorLoading = $state(false);
  let inspectorDetail = $state<SystemProcessDetail | null>(null);
  let reniceValue = $state(0);

  // State Modal Safe Reboot Server
  let rebootModalOpen = $state(false);
  let rebootConfirmInput = $state('');
  let rebootSubmitting = $state(false);

  // ── Pengambilan Data Per Modul ────────────────────────────────────────────

  async function loadOverview(silent = false) {
    try {
      if (!silent) loadingOverview = true;
      const res = await apiFetch<{ ok: boolean; overview: HostOverview }>('/api/system/overview');
      overview = res?.overview || null;
    } catch (err: any) {
      if (!silent) notify(err?.message || 'Gagal memuat ringkasan host', 'error');
    } finally {
      if (!silent) loadingOverview = false;
    }
  }

  async function loadProcesses(silent = false) {
    try {
      if (!silent) loadingProcs = true;
      const params = new URLSearchParams({
        limit: '80',
        sort: procSort,
        filter: procFilter,
      });
      if (procSearch.trim()) params.set('search', procSearch.trim());
      const res = await apiFetch<{ ok: boolean; processes: SystemProcess[] }>(`/api/system/processes?${params.toString()}`);
      processes = res?.processes || [];
    } catch (err: any) {
      if (!silent) notify(err?.message || 'Gagal memuat proses sistem', 'error');
    } finally {
      if (!silent) loadingProcs = false;
    }
  }

  async function loadPorts(silent = false) {
    try {
      if (!silent) loadingPorts = true;
      const res = await apiFetch<{ ok: boolean; ports: PortListening[]; connections: Record<string, number> }>('/api/system/ports');
      ports = res?.ports || [];
      connections = res?.connections || {};
    } catch (err: any) {
      if (!silent) notify(err?.message || 'Gagal memuat status jaringan & port', 'error');
    } finally {
      if (!silent) loadingPorts = false;
    }
  }

  async function loadMemory(silent = false) {
    try {
      if (!silent) loadingMemory = true;
      const res = await apiFetch<{ ok: boolean; memory: MemoryOptimizationStatus }>('/api/system/memory');
      memory = res?.memory || null;
    } catch (err: any) {
      if (!silent) notify(err?.message || 'Gagal memuat informasi memori & swap', 'error');
    } finally {
      if (!silent) loadingMemory = false;
    }
  }

  async function scanJunk(silent = false) {
    try {
      scanning = true;
      const res = await apiFetch<{ ok: boolean; scan: CleanerScanResult }>('/api/system/cleaner/scan', {
        timeoutMs: 60_000,
      });
      scanResult = res?.scan || null;
      if (!silent) notify('Pemindaian kategori sampah selesai', 'info');
    } catch (err: any) {
      if (!silent) notify(err?.message || 'Gagal memindai berkas sampah', 'error');
    } finally {
      scanning = false;
    }
  }

  async function loadUpdates(refresh = false, silent = false) {
    try {
      loadingUpdates = true;
      const url = refresh ? '/api/system/updates?refresh=1' : '/api/system/updates';
      const res = await apiFetch<{ ok: boolean; updates: UpdatesOverview }>(url, {
        timeoutMs: refresh ? 60_000 : 30_000,
      });
      updates = res?.updates || null;
      if (refresh && !silent) notify('Indeks paket OS berhasil diperbarui', 'success');
    } catch (err: any) {
      if (!silent) notify(err?.message || 'Gagal memeriksa pembaruan OS', 'error');
    } finally {
      loadingUpdates = false;
    }
  }

  async function loadDiagnostics(silent = false) {
    try {
      loadingDiag = true;
      const res = await apiFetch<{ ok: boolean; diagnostics: DiagnosticsReport }>('/api/system/diagnostics');
      diagnostics = res?.diagnostics || null;
    } catch (err: any) {
      if (!silent) notify(err?.message || 'Gagal memuat log diagnostik kernel', 'error');
    } finally {
      loadingDiag = false;
    }
  }

  // Segarkan tab yang sedang aktif secara kontekstual
  async function refreshActiveTab() {
    switch (activeTab) {
      case 'overview':
        await loadOverview();
        break;
      case 'processes':
        await loadProcesses();
        break;
      case 'network':
        await loadPorts();
        break;
      case 'memory':
        await loadMemory();
        break;
      case 'cleaner':
        await scanJunk();
        break;
      case 'diagnostics':
        await Promise.all([loadUpdates(false), loadDiagnostics()]);
        break;
    }
  }

  function selectTab(tab: SystemTabId) {
    activeTab = tab;
    if (tab === 'overview' && !overview) loadOverview();
    if (tab === 'processes' && processes.length === 0) loadProcesses();
    if (tab === 'network' && ports.length === 0) loadPorts();
    if (tab === 'memory' && !memory) loadMemory();
    if (tab === 'cleaner' && !scanResult) scanJunk(true);
    if (tab === 'diagnostics') {
      if (!updates) loadUpdates(false, true);
      if (!diagnostics) loadDiagnostics(true);
    }
  }

  // Debounce pencarian proses agar tidak membanjiri API saat mengetik cepat
  let searchTimer: ReturnType<typeof setTimeout> | null = null;
  function handleProcessQueryChange() {
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      loadProcesses();
    }, 180);
  }

  // Auto-refresh berkala saat user mengaktifkan mode Live di tab Task Manager
  $effect(() => {
    const intervalSec = pollSeconds;
    const currentTab = activeTab;
    if (!intervalSec || currentTab !== 'processes') return;
    const id = setInterval(() => {
      loadProcesses(true);
    }, intervalSec * 1000);
    return () => clearInterval(id);
  });

  // ── Aksi Proses, Port, Swap, Cleaner, Updates & Reboot ────────────────────

  async function inspectProcess(p: SystemProcess) {
    inspectorOpen = true;
    inspectorLoading = true;
    inspectorDetail = null;
    try {
      const res = await apiFetch<{ ok: boolean; process: SystemProcessDetail }>(`/api/system/processes/${p.pid}`);
      inspectorDetail = res?.process || null;
      reniceValue = inspectorDetail?.nice ?? 0;
    } catch (err: any) {
      notify(err?.message || `Gagal membaca /proc/${p.pid}`, 'error');
      inspectorOpen = false;
    } finally {
      inspectorLoading = false;
    }
  }

  async function sendProcessSignal(pid: number, label: string, signal: 'TERM' | 'KILL' | 'HUP') {
    const titleMap = {
      TERM: 'Hentikan Proses (SIGTERM)',
      KILL: 'Paksa Hentikan Proses (SIGKILL)',
      HUP: 'Muat Ulang Konfigurasi (SIGHUP)',
    };
    const ok = await confirmDialog({
      title: titleMap[signal],
      message: `Kirim sinyal ${signal} ke proses #${pid} (${label})?`,
      confirmText: `Kirim ${signal}`,
      tone: signal === 'HUP' ? 'default' : 'danger',
    });
    if (!ok) return;

    try {
      busyKey = `kill:${pid}`;
      await apiFetch(`/api/system/processes/${pid}/kill?signal=${signal}`, { method: 'POST' });
      notify(`Sinyal ${signal} berhasil dikirim ke proses #${pid}`, 'success');
      if (signal !== 'HUP' && inspectorDetail?.pid === pid) {
        inspectorOpen = false;
      }
      await Promise.all([loadProcesses(true), activeTab === 'network' ? loadPorts(true) : Promise.resolve()]);
    } catch (err: any) {
      notify(err?.message || `Gagal mengirim sinyal ${signal} ke proses #${pid}`, 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleSaveRenice() {
    if (!inspectorDetail) return;
    const pid = inspectorDetail.pid;
    const prio = Math.max(-10, Math.min(19, Math.round(Number(reniceValue) || 0)));
    try {
      busyKey = `renice:${pid}`;
      await apiFetch(`/api/system/processes/${pid}/renice`, {
        method: 'POST',
        body: JSON.stringify({ priority: prio }),
      });
      inspectorDetail = { ...inspectorDetail, nice: prio };
      notify(`Prioritas nice proses #${pid} diperbarui ke ${prio}`, 'success');
    } catch (err: any) {
      notify(err?.message || 'Gagal mengubah prioritas proses', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleFreePort(p: PortListening) {
    if (!p.can_kill || p.pid <= 1) return;
    await sendProcessSignal(p.pid, `${p.program} di port :${p.port}`, 'TERM');
  }

  async function handleCreateSwap(sizeMB: number) {
    const ok = await confirmDialog({
      title: memory?.has_fikpanel_swap ? 'Ubah Ukuran Swapfile' : 'Buat Swapfile Baru',
      message: `Alokasikan swapfile sebesar ${sizeMB} MB di ${memory?.fikpanel_swap_path || '/swapfile.fikpanel'} dan daftarkan permanen ke /etc/fstab?`,
      confirmText: `Aktifkan Swap ${sizeMB} MB`,
      tone: 'default',
    });
    if (!ok) return;

    try {
      busyKey = 'mem:swap-create';
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/system/memory/swap', {
        method: 'POST',
        body: JSON.stringify({ size_mb: sizeMB }),
        timeoutMs: 180_000,
      });
      notify(res?.message || 'Swapfile berhasil diaktifkan', 'success');
      await Promise.all([loadMemory(), loadOverview(true)]);
    } catch (err: any) {
      notify(err?.message || 'Gagal membuat swapfile', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleRemoveSwap() {
    const ok = await confirmDialog({
      title: 'Hapus Swapfile FikPanel',
      message: 'Nonaktifkan (swapoff) dan hapus /swapfile.fikpanel beserta entrinya di /etc/fstab? Pastikan RAM bebas cukup menampung memori aktif.',
      confirmText: 'Hapus Swapfile',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      busyKey = 'mem:swap-del';
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/system/memory/swap', {
        method: 'DELETE',
        timeoutMs: 90_000,
      });
      notify(res?.message || 'Swapfile berhasil dihapus', 'success');
      await Promise.all([loadMemory(), loadOverview(true)]);
    } catch (err: any) {
      notify(err?.message || 'Gagal menghapus swapfile', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleSaveMemoryTuning(swappiness: number, vfsPressure: number) {
    try {
      busyKey = 'mem:tuning';
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/system/memory/tuning', {
        method: 'POST',
        body: JSON.stringify({ swappiness, vfs_cache_pressure: vfsPressure }),
      });
      notify(res?.message || 'Tuning kernel memori berhasil disimpan', 'success');
      await loadMemory(true);
    } catch (err: any) {
      notify(err?.message || 'Gagal menyimpan tuning memori kernel', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleDropCaches() {
    try {
      busyKey = 'mem:drop';
      const res = await apiFetch<{ ok: boolean; freed_bytes: number }>('/api/system/memory/drop-cache', {
        method: 'POST',
      });
      lastFreedRAM = res?.freed_bytes ?? 0;
      notify(`RAM page cache dilepaskan (${formatBytes(lastFreedRAM)})`, 'success');
      await Promise.all([loadMemory(true), loadOverview(true)]);
    } catch (err: any) {
      notify(err?.message || 'Gagal mengosongkan RAM page cache', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleCleanSystem(payload: CleanSelectionPayload) {
    const ok = await confirmDialog({
      title: 'Jalankan Pembersihan Sistem',
      message: 'Kategori sampah terpilih akan dibersihkan secara permanen dari disk host. Berkas aktif yang sedang digunakan aplikasi tetap aman.',
      confirmText: 'Bersihkan Sekarang',
      tone: 'default',
    });
    if (!ok) return;

    try {
      cleaning = true;
      const res = await apiFetch<{ ok: boolean; report: CleanReport; message: string }>('/api/system/cleaner/clean', {
        method: 'POST',
        body: JSON.stringify(payload),
        timeoutMs: 120_000,
      });
      cleanReport = res?.report || null;
      notify(
        `Pembersihan selesai — ruang dibebaskan: ${formatBytes(cleanReport?.reclaimed_bytes || 0)}`,
        'success'
      );
      await Promise.all([scanJunk(true), loadOverview(true)]);
    } catch (err: any) {
      notify(err?.message || 'Gagal menjalankan pembersihan sistem', 'error');
    } finally {
      cleaning = false;
    }
  }

  async function handleAnalyzeDiskHogs(targetPath: string) {
    try {
      analyzingHogs = true;
      const res = await apiFetch<{ ok: boolean; result: DiskHogsResult }>('/api/system/disk-hogs', {
        method: 'POST',
        body: JSON.stringify({ path: targetPath }),
        timeoutMs: 30_000,
      });
      diskHogs = res?.result || null;
    } catch (err: any) {
      notify(err?.message || 'Gagal menganalisis subdirektori terbesar', 'error');
    } finally {
      analyzingHogs = false;
    }
  }

  async function handleApplySecurityUpdates() {
    const ok = await confirmDialog({
      title: 'Pasang Patch Keamanan OS',
      message: `Pasang ${updates?.security_count || 0} pembaruan paket keamanan secara non-interaktif (--only-upgrade, konfigurasi lama dipertahankan)?`,
      confirmText: 'Pasang Patch Keamanan',
      tone: 'default',
    });
    if (!ok) return;

    try {
      applyingUpdates = true;
      notify('Memasang pembaruan keamanan OS, mohon tunggu...', 'info');
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/system/updates/apply-security', {
        method: 'POST',
        timeoutMs: 240_000,
      });
      notify(res?.message || 'Pembaruan keamanan berhasil dipasang', 'success');
      await Promise.all([loadUpdates(false, true), loadOverview(true)]);
    } catch (err: any) {
      notify(err?.message || 'Gagal memasang pembaruan keamanan OS', 'error');
    } finally {
      applyingUpdates = false;
    }
  }

  function openRebootModal() {
    rebootConfirmInput = '';
    rebootModalOpen = true;
  }

  async function handleConfirmReboot() {
    const expected = overview?.hostname || '';
    if (!expected || rebootConfirmInput.trim() !== expected) return;

    try {
      rebootSubmitting = true;
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/system/reboot', {
        method: 'POST',
        body: JSON.stringify({ confirm_hostname: rebootConfirmInput.trim() }),
      });
      notify(res?.message || 'Perintah reboot dikirim ke host server', 'warning');
      rebootModalOpen = false;
    } catch (err: any) {
      notify(err?.message || 'Gagal menjadwalkan reboot server', 'error');
    } finally {
      rebootSubmitting = false;
    }
  }

  onMount(() => {
    // Muat telemetri ringkasan & sinyal badge (updates & diagnostik) secara paralel saat membuka halaman
    loadOverview();
    loadUpdates(false, true);
    loadDiagnostics(true);
    return () => {
      if (searchTimer) clearTimeout(searchTimer);
    };
  });
</script>

<div class="pg-systemmanager">
  <PageHeader
    title="System & Performance Console"
    subtitle="Telemetri host, Task Manager /proc, jaringan & port, Swapfile manager, Disk Cleaner, dan diagnostik kernel OOM."
    icon="ri-cpu-line"
  >
    {#snippet actions()}
      <button type="button" class="btn btn--secondary btn--sm" onclick={refreshActiveTab}>
        <i
          class={loadingOverview || loadingProcs || loadingPorts || loadingMemory || scanning || loadingUpdates || loadingDiag
            ? 'ri-loader-4-line spin'
            : 'ri-refresh-line'}
          aria-hidden="true"
        ></i>
        <span>Segarkan Tab</span>
      </button>
      <button type="button" class="btn btn--danger btn--sm" onclick={openRebootModal}>
        <i class="ri-shut-down-line" aria-hidden="true"></i>
        <span>Reboot Host</span>
      </button>
    {/snippet}
  </PageHeader>

  <!-- Navigasi 6 Tab Operasional Sistem -->
  <div class="tabs" role="tablist" aria-label="Navigasi System & Performance Console">
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={activeTab === 'overview'}
      aria-selected={activeTab === 'overview'}
      onclick={() => selectTab('overview')}
    >
      <i class="ri-dashboard-3-line" aria-hidden="true"></i>
      <span>Ringkasan &amp; Disk</span>
      {#if overview?.needs_reboot}
        <span class="sys-tab-count sys-tab-count--danger">!</span>
      {/if}
    </button>

    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={activeTab === 'processes'}
      aria-selected={activeTab === 'processes'}
      onclick={() => selectTab('processes')}
    >
      <i class="ri-pulse-line" aria-hidden="true"></i>
      <span>Task Manager</span>
      {#if (overview?.zombie_procs || 0) > 0}
        <span class="sys-tab-count sys-tab-count--danger">{overview?.zombie_procs}Z</span>
      {/if}
    </button>

    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={activeTab === 'network'}
      aria-selected={activeTab === 'network'}
      onclick={() => selectTab('network')}
    >
      <i class="ri-broadcast-line" aria-hidden="true"></i>
      <span>Jaringan &amp; Port</span>
      {#if ports.length > 0}
        <span class="sys-tab-count">{ports.length}</span>
      {/if}
    </button>

    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={activeTab === 'memory'}
      aria-selected={activeTab === 'memory'}
      onclick={() => selectTab('memory')}
    >
      <i class="ri-swap-box-line" aria-hidden="true"></i>
      <span>Swap &amp; Memori</span>
    </button>

    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={activeTab === 'cleaner'}
      aria-selected={activeTab === 'cleaner'}
      onclick={() => selectTab('cleaner')}
    >
      <i class="ri-brush-line" aria-hidden="true"></i>
      <span>Disk Cleaner</span>
    </button>

    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={activeTab === 'diagnostics'}
      aria-selected={activeTab === 'diagnostics'}
      onclick={() => selectTab('diagnostics')}
    >
      <i class="ri-stethoscope-line" aria-hidden="true"></i>
      <span>Updates &amp; Diagnostik</span>
      {#if (updates?.security_count || 0) + (diagnostics?.oom_count || 0) > 0}
        <span class="sys-tab-count sys-tab-count--danger">
          {(updates?.security_count || 0) + (diagnostics?.oom_count || 0)}
        </span>
      {/if}
    </button>
  </div>

  <!-- Konten Tab Aktif -->
  {#if activeTab === 'overview'}
    <OverviewTab
      {overview}
      onOpenRebootModal={openRebootModal}
      onJumpTab={(tab) => selectTab(tab)}
    />
  {:else if activeTab === 'processes'}
    <ProcessesTab
      {processes}
      loading={loadingProcs}
      bind:sortBy={procSort}
      bind:search={procSearch}
      bind:filter={procFilter}
      bind:pollSeconds
      {busyKey}
      onChangeQuery={handleProcessQueryChange}
      onInspect={inspectProcess}
      onKill={(p, sig) => sendProcessSignal(p.pid, p.command, sig)}
    />
  {:else if activeTab === 'network'}
    <NetworkTab
      {ports}
      {connections}
      loading={loadingPorts}
      {busyKey}
      onRefresh={() => loadPorts()}
      onFreePort={handleFreePort}
    />
  {:else if activeTab === 'memory'}
    <MemoryTab
      {memory}
      {busyKey}
      {lastFreedRAM}
      onCreateSwap={handleCreateSwap}
      onRemoveSwap={handleRemoveSwap}
      onSaveTuning={handleSaveMemoryTuning}
      onDropCaches={handleDropCaches}
    />
  {:else if activeTab === 'cleaner'}
    <CleanerTab
      {scanResult}
      {cleanReport}
      {diskHogs}
      {scanning}
      {cleaning}
      {analyzingHogs}
      onScan={() => scanJunk()}
      onClean={handleCleanSystem}
      onAnalyzeHogs={handleAnalyzeDiskHogs}
    />
  {:else if activeTab === 'diagnostics'}
    <DiagnosticsTab
      {updates}
      {diagnostics}
      {loadingUpdates}
      {applyingUpdates}
      {loadingDiag}
      onCheckUpdates={(refresh) => loadUpdates(refresh)}
      onApplySecurityUpdates={handleApplySecurityUpdates}
      onRefreshDiag={() => loadDiagnostics()}
      onOpenRebootModal={openRebootModal}
    />
  {/if}

  <!-- Modal Process Inspector (/proc/<pid>) & Renice -->
  <Modal
    open={inspectorOpen}
    title={inspectorDetail ? `Inspeksi Proses #${inspectorDetail.pid} (${inspectorDetail.name})` : 'Inspeksi Proses'}
    subtitle="Telemetri langsung dari /proc/<pid> dan pengaturan prioritas CPU (renice)."
    size="lg"
    onclose={() => (inspectorOpen = false)}
  >
    {#if inspectorLoading}
      <div class="state state--loading">
        <i class="ri-loader-4-line spin" aria-hidden="true"></i>
        <div class="state__title">Membaca Metadata /proc...</div>
      </div>
    {:else if inspectorDetail}
      <div class="stack">
        {#if inspectorDetail.protected}
          <div class="alert alert--info" role="status">
            <i class="ri-shield-check-fill" aria-hidden="true"></i>
            <span>Proses ini termasuk daemon vital sistem sehingga tombol penghentian sinyal dinonaktifkan demi keamanan akses server.</span>
          </div>
        {/if}

        <div class="sys-inspector-grid">
          <div class="sys-spec-item">
            <span class="sys-spec-label">PID / PPID</span>
            <span class="sys-spec-val mono">#{inspectorDetail.pid} / Parent #{inspectorDetail.ppid}</span>
          </div>
          <div class="sys-spec-item">
            <span class="sys-spec-label">Pemilik &amp; Status</span>
            <span class="sys-spec-val mono">{inspectorDetail.user} · {inspectorDetail.state}</span>
          </div>
          <div class="sys-spec-item">
            <span class="sys-spec-label">Memori RSS / Swap</span>
            <span class="sys-spec-val mono">{formatBytes(inspectorDetail.rss_bytes)} / {formatBytes(inspectorDetail.swap_bytes)}</span>
          </div>
          <div class="sys-spec-item">
            <span class="sys-spec-label">Threads / Open FDs / OOM Score</span>
            <span class="sys-spec-val mono">{inspectorDetail.threads} thr · {inspectorDetail.fd_count} FD · OOM {inspectorDetail.oom_score}</span>
          </div>
        </div>

        <div class="field">
          <span class="field__label">Executable Binary (exe)</span>
          <div class="sys-code-box">{inspectorDetail.exe || '-'}</div>
        </div>

        <div class="field">
          <span class="field__label">Working Directory (cwd)</span>
          <div class="sys-code-box">{inspectorDetail.cwd || '-'}</div>
        </div>

        <div class="field">
          <span class="field__label">Command Line Lengkap (cmdline)</span>
          <div class="sys-code-box">{inspectorDetail.cmdline || '-'}</div>
        </div>

        <div class="field">
          <label class="field__label" for="proc-renice-input">Prioritas CPU / Nice (-10 tinggi s/d 19 rendah)</label>
          <div class="row wrap">
            <input
              id="proc-renice-input"
              type="number"
              min="-10"
              max="19"
              class="input input--mono grow"
              bind:value={reniceValue}
            />
            <button
              type="button"
              class="btn btn--secondary btn--sm"
              disabled={busyKey === `renice:${inspectorDetail.pid}`}
              onclick={handleSaveRenice}
            >
              {busyKey === `renice:${inspectorDetail.pid}` ? 'Menyimpan...' : 'Terapkan Nice'}
            </button>
          </div>
        </div>
      </div>
    {/if}

    {#snippet footer()}
      {#if inspectorDetail}
        <button
          type="button"
          class="btn btn--secondary btn--sm"
          disabled={inspectorDetail.protected || busyKey === `kill:${inspectorDetail.pid}`}
          onclick={() => inspectorDetail && sendProcessSignal(inspectorDetail.pid, inspectorDetail.name, 'HUP')}
        >
          Reload (SIGHUP)
        </button>
        <button
          type="button"
          class="btn btn--secondary btn--sm"
          disabled={inspectorDetail.protected || busyKey === `kill:${inspectorDetail.pid}`}
          onclick={() => inspectorDetail && sendProcessSignal(inspectorDetail.pid, inspectorDetail.name, 'TERM')}
        >
          Stop Sopan (SIGTERM)
        </button>
        <button
          type="button"
          class="btn btn--danger btn--sm"
          disabled={inspectorDetail.protected || busyKey === `kill:${inspectorDetail.pid}`}
          onclick={() => inspectorDetail && sendProcessSignal(inspectorDetail.pid, inspectorDetail.name, 'KILL')}
        >
          Force Kill (SIGKILL)
        </button>
      {/if}
    {/snippet}
  </Modal>

  <!-- Modal Safe Reboot Server dengan Konfirmasi Hostname Persis -->
  <Modal
    open={rebootModalOpen}
    title="Konfirmasi Reboot Host Server"
    subtitle="Seluruh koneksi web, SSH, dan panel akan terputus sementara selama server memulai ulang."
    size="sm"
    onclose={() => (rebootModalOpen = false)}
  >
    <div class="stack">
      <div class="alert alert--danger" role="alert">
        <i class="ri-error-warning-fill" aria-hidden="true"></i>
        <span>
          Pastikan tidak ada proses backup, migrasi database, atau pembaruan kernel yang masih berjalan.
        </span>
      </div>

      <div class="field">
        <label class="field__label" for="reboot-hostname-confirm">
          Ketik hostname persis <code class="mono">{overview?.hostname || '-'}</code> untuk melanjutkan
        </label>
        <input
          id="reboot-hostname-confirm"
          type="text"
          class="input input--mono"
          placeholder={overview?.hostname || 'hostname'}
          data-autofocus
          bind:value={rebootConfirmInput}
        />
      </div>
    </div>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary btn--sm" onclick={() => (rebootModalOpen = false)}>
        Batal
      </button>
      <button
        type="button"
        class="btn btn--danger btn--sm"
        disabled={rebootSubmitting || !overview?.hostname || rebootConfirmInput.trim() !== overview.hostname}
        onclick={handleConfirmReboot}
      >
        <i class="ri-shut-down-line" aria-hidden="true"></i>
        <span>{rebootSubmitting ? 'Menjadwalkan Reboot...' : 'Reboot Server Sekarang'}</span>
      </button>
    {/snippet}
  </Modal>
</div>
