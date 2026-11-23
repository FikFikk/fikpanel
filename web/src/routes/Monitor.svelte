<script lang="ts">
  import { onMount } from 'svelte';
  import Chart from '../lib/components/Chart.svelte';
  import HistoryChart from '../lib/components/HistoryChart.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import { formatBytes, apiFetch } from '../lib/api';
  import type {
    MetricPoint,
    MonitorSnapshot,
    MonitorConfig,
    UptimeMonitor,
    UptimeCheck,
  } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import '../styles/pages/monitor.css';

  interface Props {
    currentMetric: MetricPoint;
    history: MetricPoint[];
  }

  let { currentMetric, history }: Props = $props();

  // Mode tab: 'history' (persisten), 'realtime' (SSE 60 detik), dan 'uptime' (Uptime & SSL Monitor)
  let activeTab = $state<'history' | 'realtime' | 'uptime'>('history');

  // State histori persisten
  let snapshots = $state<MonitorSnapshot[]>([]);
  let loadingHistory = $state(false);
  let timeRange = $state<'1h' | '6h' | '24h' | '7d' | '30d'>('24h');
  let chartMode = $state<'load' | 'resource' | 'network'>('load');

  // Konfigurasi toggle monitoring sistem
  let monitorConfig = $state<MonitorConfig>({ enabled: true, save_days: 30 });
  let updatingConfig = $state(false);

  // State Uptime & SSL Monitor
  let uptimeMonitors = $state<UptimeMonitor[]>([]);
  let loadingUptime = $state(false);
  let importingSites = $state(false);
  let checkingId = $state<number | null>(null);
  let togglingId = $state<number | null>(null);

  // State modal tambah/edit monitor
  let showMonitorModal = $state(false);
  let editingMonitorId = $state<number | null>(null);
  let savingMonitor = $state(false);
  let formName = $state('');
  let formType = $state<'http' | 'tcp'>('http');
  let formTarget = $state('');
  let formInterval = $state(60);
  let formTimeout = $state(10);
  let formExpectedStatus = $state(0);
  let formKeyword = $state('');
  let formNotifyWebhook = $state(true);
  let formPublicVisible = $state(true);
  let formEnabled = $state(true);

  // Ringkasan metrik Uptime & SSL
  const uptimeStats = $derived.by(() => {
    const total = uptimeMonitors.length;
    let up = 0;
    let down = 0;
    let sslWarn = 0;
    let latSum = 0;
    let latCount = 0;
    for (const m of uptimeMonitors) {
      if (!m.enabled) continue;
      if (m.last_status === 'up') up++;
      else if (m.last_status === 'down') down++;
      if (m.ssl_days_remaining >= 0 && m.ssl_days_remaining <= 14) sslWarn++;
      if (m.last_latency_ms > 0) {
        latSum += m.last_latency_ms;
        latCount++;
      }
    }
    return {
      total,
      up,
      down,
      sslWarn,
      avgLatency: latCount > 0 ? Math.round(latSum / latCount) : 0,
    };
  });

  async function loadMonitorConfig() {
    try {
      const res = await apiFetch<{ ok: boolean; config: MonitorConfig }>('/api/monitor/config');
      if (res?.config) {
        monitorConfig = res.config;
      }
    } catch {
      // Abaikan kesalahan pemuatan awal konfigurasi
    }
  }

  async function toggleMonitor() {
    try {
      updatingConfig = true;
      const newStatus = !monitorConfig.enabled;
      await apiFetch('/api/monitor/config', {
        method: 'POST',
        body: JSON.stringify({
          enabled: newStatus,
          save_days: monitorConfig.save_days,
        }),
      });
      monitorConfig.enabled = newStatus;
      notify(
        newStatus
          ? 'Status Monitoring diaktifkan: Riwayat performa dicatat ke database'
          : 'Status Monitoring dimatikan: Perekaman dihentikan (kosong/tidak mencatat)',
        'info'
      );
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status monitoring', 'error');
    } finally {
      updatingConfig = false;
    }
  }

  async function loadHistory() {
    try {
      loadingHistory = true;
      const now = Math.floor(Date.now() / 1000);
      let duration = 86400; // 24 jam
      if (timeRange === '1h') duration = 3600;
      else if (timeRange === '6h') duration = 21600;
      else if (timeRange === '7d') duration = 86400 * 7;
      else if (timeRange === '30d') duration = 86400 * 30;

      const start = now - duration;
      const res = await apiFetch<{ ok: boolean; history: MonitorSnapshot[] }>(
        `/api/monitor/history?start=${start}&end=${now}`
      );
      snapshots = res?.history || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat histori monitoring', 'error');
    } finally {
      loadingHistory = false;
    }
  }

  async function clearHistory() {
    const ok = await confirmDialog({
      title: 'Kosongkan Riwayat Monitoring',
      message: 'Hapus seluruh rekaman histori metrik monitoring yang tersimpan di server?',
      confirmText: 'Kosongkan',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch('/api/monitor/history', { method: 'DELETE' });
      notify('Seluruh data histori monitoring berhasil dikosongkan', 'success');
      snapshots = [];
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus rekaman', 'error');
    }
  }

  // ── Logika Uptime & SSL Monitor ───────────────────────────────────────────
  async function loadUptimeMonitors(silent = false) {
    try {
      if (!silent) loadingUptime = true;
      const res = await apiFetch<{ ok: boolean; monitors: UptimeMonitor[] }>('/api/uptime/monitors');
      uptimeMonitors = res?.monitors || [];
    } catch (err: any) {
      if (!silent) {
        notify(err.message || 'Gagal memuat daftar monitor uptime', 'error');
      }
    } finally {
      if (!silent) loadingUptime = false;
    }
  }

  function openAddMonitor() {
    editingMonitorId = null;
    formName = '';
    formType = 'http';
    formTarget = '';
    formInterval = 60;
    formTimeout = 10;
    formExpectedStatus = 0;
    formKeyword = '';
    formNotifyWebhook = true;
    formPublicVisible = true;
    formEnabled = true;
    showMonitorModal = true;
  }

  function openEditMonitor(m: UptimeMonitor) {
    editingMonitorId = m.id;
    formName = m.name;
    formType = m.monitor_type === 'tcp' ? 'tcp' : 'http';
    formTarget = m.target;
    formInterval = m.interval_sec || 60;
    formTimeout = m.timeout_sec || 10;
    formExpectedStatus = m.expected_status ?? 0;
    formKeyword = m.keyword || '';
    formNotifyWebhook = m.notify_webhook;
    formPublicVisible = m.public_visible;
    formEnabled = m.enabled;
    showMonitorModal = true;
  }

  async function saveMonitor(e: SubmitEvent) {
    e.preventDefault();
    if (!formName.trim() || !formTarget.trim()) {
      notify('Nama monitor dan target URL/Host wajib diisi', 'error');
      return;
    }
    try {
      savingMonitor = true;
      const payload = {
        name: formName.trim(),
        monitor_type: formType,
        target: formTarget.trim(),
        interval_sec: Number(formInterval) || 60,
        timeout_sec: Number(formTimeout) || 10,
        expected_status: formType === 'http' ? Number(formExpectedStatus) || 0 : 0,
        keyword: formType === 'http' ? formKeyword.trim() : '',
        notify_webhook: formNotifyWebhook,
        public_visible: formPublicVisible,
        enabled: formEnabled,
      };
      if (editingMonitorId) {
        await apiFetch(`/api/uptime/monitors/${editingMonitorId}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        });
        notify('Konfigurasi monitor diperbarui', 'success');
      } else {
        await apiFetch('/api/uptime/monitors', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        notify('Monitor baru ditambahkan dan pemeriksaan awal dimulai', 'success');
      }
      showMonitorModal = false;
      await loadUptimeMonitors();
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan monitor', 'error');
    } finally {
      savingMonitor = false;
    }
  }

  async function checkMonitorNow(m: UptimeMonitor) {
    try {
      checkingId = m.id;
      const res = await apiFetch<{ ok: boolean; monitor?: UptimeMonitor }>(
        `/api/uptime/monitors/${m.id}/check`,
        { method: 'POST' }
      );
      if (res?.monitor) {
        uptimeMonitors = uptimeMonitors.map((item) => (item.id === m.id ? res.monitor! : item));
        const st = res.monitor.last_status === 'up' ? 'ONLINE' : 'OFFLINE';
        notify(
          `${m.name}: ${st} (${res.monitor.last_latency_ms} ms)`,
          res.monitor.last_status === 'up' ? 'success' : 'error'
        );
      } else {
        await loadUptimeMonitors(true);
      }
    } catch (err: any) {
      notify(err.message || 'Gagal menjalankan pemeriksaan langsung', 'error');
    } finally {
      checkingId = null;
    }
  }

  async function toggleMonitorEnabled(m: UptimeMonitor) {
    try {
      togglingId = m.id;
      const nextEnabled = !m.enabled;
      const res = await apiFetch<{ ok: boolean; monitor?: UptimeMonitor }>(
        `/api/uptime/monitors/${m.id}`,
        {
          method: 'PUT',
          body: JSON.stringify({
            name: m.name,
            target: m.target,
            monitor_type: m.monitor_type,
            interval_sec: m.interval_sec,
            timeout_sec: m.timeout_sec,
            expected_status: m.expected_status,
            keyword: m.keyword,
            notify_webhook: m.notify_webhook,
            public_visible: m.public_visible,
            enabled: nextEnabled,
          }),
        }
      );
      if (res?.monitor) {
        uptimeMonitors = uptimeMonitors.map((item) => (item.id === m.id ? res.monitor! : item));
      } else {
        await loadUptimeMonitors(true);
      }
      notify(nextEnabled ? `${m.name} diaktifkan kembali` : `${m.name} dijeda`, 'info');
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status monitor', 'error');
    } finally {
      togglingId = null;
    }
  }

  async function deleteMonitor(m: UptimeMonitor) {
    const ok = await confirmDialog({
      title: 'Hapus Monitor Uptime',
      message: `Hapus monitor "${m.name}" (${m.target}) beserta seluruh riwayat heartbeat-nya?`,
      confirmText: 'Hapus Monitor',
      tone: 'danger',
    });
    if (!ok) return;
    try {
      await apiFetch(`/api/uptime/monitors/${m.id}`, { method: 'DELETE' });
      uptimeMonitors = uptimeMonitors.filter((item) => item.id !== m.id);
      notify(`Monitor "${m.name}" dihapus`, 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus monitor', 'error');
    }
  }

  async function importSitesToUptime() {
    try {
      importingSites = true;
      const res = await apiFetch<{ ok: boolean; added: number; message?: string }>(
        '/api/uptime/import-sites',
        { method: 'POST' }
      );
      notify(res?.message || `${res?.added ?? 0} situs diimpor ke Uptime Monitor`, 'success');
      await loadUptimeMonitors();
    } catch (err: any) {
      notify(err.message || 'Gagal mengimpor situs', 'error');
    } finally {
      importingSites = false;
    }
  }

  // Bangun tepat 30 slot bar heartbeat (kosong di kiri, kronologis terbaru di kanan)
  function buildHeartbeatSlots(checks: UptimeCheck[] | undefined): Array<UptimeCheck | null> {
    const list = checks || [];
    const totalSlots = 30;
    if (list.length >= totalSlots) {
      return list.slice(-totalSlots);
    }
    const pad = Array<UptimeCheck | null>(totalSlots - list.length).fill(null);
    return [...pad, ...list];
  }

  function formatCheckTooltip(c: UptimeCheck | null): string {
    if (!c) return 'Belum ada data pemeriksaan';
    const dt = new Date(c.checked_at * 1000).toLocaleString('id-ID', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      day: '2-digit',
      month: 'short',
    });
    const st = c.status === 'up' ? 'ONLINE' : 'OFFLINE';
    const code = c.status_code > 1 ? ` · HTTP ${c.status_code}` : '';
    const err = c.error_msg ? ` · ${c.error_msg}` : '';
    return `${dt} — ${st}${code} (${c.latency_ms} ms)${err}`;
  }

  function formatLastChecked(ts: number): string {
    if (!ts) return 'Belum dicek';
    return new Date(ts * 1000).toLocaleTimeString('id-ID', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  function sslBadgeClass(days: number): string {
    if (days <= 3) return 'badge--danger';
    if (days <= 14) return 'badge--warning';
    return 'badge--success';
  }

  const load5m = $derived(
    (currentMetric as MetricPoint & { load_5m?: number }).load_5m ?? currentMetric.load_avg
  );

  $effect(() => {
    if (activeTab === 'history') {
      const _tr = timeRange;
      loadHistory();
    }
  });

  // Segarkan data Uptime otomatis setiap 20 detik selama tab Uptime dibuka
  $effect(() => {
    if (activeTab !== 'uptime') return;
    loadUptimeMonitors(uptimeMonitors.length > 0);
    const timer = setInterval(() => {
      loadUptimeMonitors(true);
    }, 20000);
    return () => clearInterval(timer);
  });

  onMount(() => {
    loadMonitorConfig();
    loadHistory();
    loadUptimeMonitors(true);
  });
</script>

<div class="pg-monitor monitor-container">
  <!-- Baris kepala halaman -->
  <div class="page-header">
    <div class="page-header__main">
      <h2 class="page-header__title">Monitor Performa, Uptime &amp; SSL</h2>
      <p class="page-header__sub">
        Pelacak metrik histori beban server, streaming real-time, serta pemantau Uptime HTTP/TCP &amp; masa berlaku SSL
      </p>
    </div>

    <div class="monitor-controls">
      <!-- Sakelar status monitoring metrik sistem -->
      {#if activeTab !== 'uptime'}
        <div class="switch-box">
          <span class="switch-label">Status Monitoring:</span>
          <button
            class="switch-btn {monitorConfig.enabled ? 'active' : ''}"
            aria-label="Ubah status perekaman monitoring"
            onclick={toggleMonitor}
            disabled={updatingConfig}
          >
            <span class="switch-knob"></span>
          </button>
          <span class="switch-text {monitorConfig.enabled ? 'text-accent' : 'text-subtle'}">
            {monitorConfig.enabled ? 'MENYALA' : 'MATI'}
          </span>
        </div>
      {/if}

      <div class="tabs tabs--pill" role="tablist">
        <button
          role="tab"
          aria-selected={activeTab === 'history'}
          class="tabs__tab"
          class:is-active={activeTab === 'history'}
          onclick={() => (activeTab = 'history')}
        >
          <i class="ri-history-line"></i> Rekaman Histori
        </button>
        <button
          role="tab"
          aria-selected={activeTab === 'realtime'}
          class="tabs__tab"
          class:is-active={activeTab === 'realtime'}
          onclick={() => (activeTab = 'realtime')}
        >
          <i class="ri-pulse-line"></i> Live Streaming (1s)
        </button>
        <button
          role="tab"
          aria-selected={activeTab === 'uptime'}
          class="tabs__tab"
          class:is-active={activeTab === 'uptime'}
          onclick={() => (activeTab = 'uptime')}
        >
          <i class="ri-heart-pulse-line"></i> Uptime &amp; SSL
          {#if uptimeStats.down > 0}
            <span class="badge badge--danger">{uptimeStats.down}</span>
          {/if}
        </button>
      </div>
    </div>
  </div>

  {#if activeTab !== 'uptime'}
    <!-- Ringkasan kartu metrik sistem -->
    <div class="metrics-grid">
      <div class="metric-card">
        <div class="m-label">CPU Core Usage</div>
        <div class="m-val m-val--info">{currentMetric.cpu.toFixed(1)}%</div>
        <div class="m-sub">Load: {currentMetric.load_avg.toFixed(2)} (1m) &middot; {load5m.toFixed(2)} (5m)</div>
      </div>
      <div class="metric-card">
        <div class="m-label">Physical Memory (RAM)</div>
        <div class="m-val m-val--violet">
          {currentMetric.ram_total > 0
            ? ((currentMetric.ram_used / currentMetric.ram_total) * 100).toFixed(1)
            : '0'}%
        </div>
        <div class="m-sub">{formatBytes(currentMetric.ram_used)} / {formatBytes(currentMetric.ram_total)}</div>
      </div>
      <div class="metric-card">
        <div class="m-label">Jaringan Masuk (Inbound)</div>
        <div class="m-val text-accent">{formatBytes(currentMetric.net_in)}/s</div>
        <div class="m-sub">Traffic download / request masuk</div>
      </div>
      <div class="metric-card">
        <div class="m-label">Jaringan Keluar (Outbound)</div>
        <div class="m-val text-warning">{formatBytes(currentMetric.net_out)}/s</div>
        <div class="m-sub">Traffic upload / respon web</div>
      </div>
    </div>
  {:else}
    <!-- Ringkasan KPI Uptime & SSL -->
    <div class="metrics-grid">
      <div class="metric-card">
        <div class="m-label">Total Target</div>
        <div class="m-val m-val--info">{uptimeStats.total}</div>
        <div class="m-sub">Monitor HTTP/HTTPS &amp; TCP terdaftar</div>
      </div>
      <div class="metric-card">
        <div class="m-label">Layanan Online</div>
        <div class="m-val m-val--success">{uptimeStats.up}</div>
        <div class="m-sub">Merespons normal sesuai kriteria</div>
      </div>
      <div class="metric-card">
        <div class="m-label">Sedang Gangguan</div>
        <div class="m-val {uptimeStats.down > 0 ? 'm-val--danger' : 'm-val--muted'}">{uptimeStats.down}</div>
        <div class="m-sub">Memicu notifikasi webhook otomatis</div>
      </div>
      <div class="metric-card">
        <div class="m-label">Rata-rata Latensi</div>
        <div class="m-val m-val--violet">{uptimeStats.avgLatency} ms</div>
        <div class="m-sub">
          {#if uptimeStats.sslWarn > 0}
            <span class="text-warning">{uptimeStats.sslWarn} SSL mendekati kedaluwarsa</span>
          {:else}
            Semua sertifikat SSL aman
          {/if}
        </div>
      </div>
    </div>
  {/if}

  {#if activeTab === 'history'}
    <!-- Tampilan histori persisten -->
    <div class="card">
      <div class="card__head flex-wrap">
        <div>
          <h3 class="card__title flex items-center gap-2">
            <i class="ri-time-line"></i> Grafik Rekaman Histori Server
          </h3>
          <p class="panel-sub">
            {#if monitorConfig.enabled}
              Mencatat data berkala setiap 60 detik. Retensi penyimpanan: {monitorConfig.save_days} hari.
            {:else}
              <span class="text-danger font-bold">Monitoring saat ini non-aktif! Tidak ada data baru yang dicatat selama status mati.</span>
            {/if}
          </p>
        </div>

        <div class="filter-controls">
          <!-- Filter jenis metrik -->
          <div class="tabs tabs--pill" role="tablist">
            <button
              role="tab"
              aria-selected={chartMode === 'load'}
              class="tabs__tab"
              class:is-active={chartMode === 'load'}
              onclick={() => (chartMode = 'load')}
            >
              Load Average
            </button>
            <button
              role="tab"
              aria-selected={chartMode === 'resource'}
              class="tabs__tab"
              class:is-active={chartMode === 'resource'}
              onclick={() => (chartMode = 'resource')}
            >
              CPU &amp; RAM
            </button>
            <button
              role="tab"
              aria-selected={chartMode === 'network'}
              class="tabs__tab"
              class:is-active={chartMode === 'network'}
              onclick={() => (chartMode = 'network')}
            >
              Network I/O
            </button>
          </div>

          <!-- Filter rentang waktu -->
          <div class="tabs tabs--pill" role="tablist">
            <button role="tab" aria-selected={timeRange === '1h'} class="tabs__tab" class:is-active={timeRange === '1h'} onclick={() => (timeRange = '1h')}>1 Jam</button>
            <button role="tab" aria-selected={timeRange === '6h'} class="tabs__tab" class:is-active={timeRange === '6h'} onclick={() => (timeRange = '6h')}>6 Jam</button>
            <button role="tab" aria-selected={timeRange === '24h'} class="tabs__tab" class:is-active={timeRange === '24h'} onclick={() => (timeRange = '24h')}>Hari Ini</button>
            <button role="tab" aria-selected={timeRange === '7d'} class="tabs__tab" class:is-active={timeRange === '7d'} onclick={() => (timeRange = '7d')}>7 Hari</button>
            <button role="tab" aria-selected={timeRange === '30d'} class="tabs__tab" class:is-active={timeRange === '30d'} onclick={() => (timeRange = '30d')}>30 Hari</button>
          </div>

          <button class="btn btn--ghost btn--sm" aria-label="Refresh rekaman histori" onclick={loadHistory} disabled={loadingHistory} title="Refresh Rekaman">
            <i class="ri-refresh-line {loadingHistory ? 'spin' : ''}"></i>
          </button>
          <button class="btn btn--ghost btn--sm btn--danger" aria-label="Kosongkan semua catatan histori" onclick={clearHistory} title="Kosongkan Semua Catatan">
            <i class="ri-delete-bin-line"></i>
          </button>
        </div>
      </div>

      <!-- Baris legenda grafik -->
      <div class="chart-legend-row">
        {#if chartMode === 'load'}
          <span class="l-item"><span class="l-dot l-dot--info"></span> 1 Menit Load</span>
          <span class="l-item"><span class="l-dot l-dot--violet"></span> 5 Menit Load</span>
          <span class="l-item"><span class="l-dot l-dot--accent"></span> 15 Menit Load</span>
        {:else if chartMode === 'resource'}
          <span class="l-item"><span class="l-dot l-dot--info"></span> CPU Utilisasi %</span>
          <span class="l-item"><span class="l-dot l-dot--violet"></span> RAM Utilisasi %</span>
        {:else if chartMode === 'network'}
          <span class="l-item"><span class="l-dot l-dot--accent"></span> Inbound (Download)</span>
          <span class="l-item"><span class="l-dot l-dot--warning"></span> Outbound (Upload)</span>
        {/if}
        <span class="total-badge mono">Total Titik: {snapshots.length} sampel</span>
      </div>

      <div class="chart-box">
        <HistoryChart {snapshots} height={280} mode={chartMode} />
      </div>
    </div>
  {:else if activeTab === 'realtime'}
    <!-- Tampilan live realtime -->
    <div class="card">
      <div class="card__head">
        <div>
          <h3 class="card__title">Grafik Utilisasi CPU &amp; RAM (Realtime SSE)</h3>
          <p class="panel-sub">Buffer 60 detik terakhir &middot; Auto-scaling canvas render langsung dari kernel</p>
        </div>
        <div class="legend">
          <span class="l-item"><span class="l-dot l-dot--info"></span> CPU %</span>
          <span class="l-item"><span class="l-dot l-dot--violet"></span> RAM %</span>
        </div>
      </div>
      <Chart {history} height={260} />
    </div>
  {:else}
    <!-- Tampilan Uptime & SSL Monitor -->
    <div class="card">
      <div class="card__head flex-wrap">
        <div>
          <h3 class="card__title flex items-center gap-2">
            <i class="ri-shield-check-line"></i> Pemantau Uptime &amp; Sertifikat SSL
          </h3>
          <p class="panel-sub">
            Pemeriksaan HTTP/HTTPS/TCP terjadwal dengan strip 30-heartbeat, peringatan SSL, webhook Telegram/Discord, dan halaman status publik.
          </p>
        </div>

        <div class="filter-controls">
          <a
            class="btn btn--ghost btn--sm"
            href="/status"
            target="_blank"
            rel="noopener noreferrer"
            title="Buka halaman status publik tanpa login"
          >
            <i class="ri-external-link-line"></i> Status Publik (/status)
          </a>
          <button
            class="btn btn--ghost btn--sm"
            onclick={importSitesToUptime}
            disabled={importingSites}
            title="Impor otomatis seluruh situs aktif FikPanel"
          >
            <i class="ri-download-cloud-2-line"></i> {importingSites ? 'Mengimpor...' : 'Impor dari Situs'}
          </button>
          <button
            class="btn btn--ghost btn--sm"
            aria-label="Segarkan daftar monitor uptime"
            title="Segarkan"
            onclick={() => loadUptimeMonitors(false)}
            disabled={loadingUptime}
          >
            <i class="ri-refresh-line {loadingUptime ? 'spin' : ''}"></i>
          </button>
          <button class="btn btn--primary btn--sm" onclick={openAddMonitor}>
            <i class="ri-add-line"></i> Tambah Monitor
          </button>
        </div>
      </div>

      <div class="uptime-body">
        {#if loadingUptime && uptimeMonitors.length === 0}
          <div class="uptime-empty">
            <i class="ri-loader-4-line spin"></i>
            <p>Memuat daftar monitor uptime...</p>
          </div>
        {:else if uptimeMonitors.length === 0}
          <div class="uptime-empty">
            <i class="ri-heart-pulse-line uptime-empty__icon"></i>
            <h4>Belum Ada Target Uptime Monitor</h4>
            <p>
              Tambahkan URL situs/API atau port TCP untuk mulai memantau ketersediaan 24/7 beserta sisa masa aktif sertifikat SSL.
            </p>
            <div class="uptime-empty__actions">
              <button class="btn btn--primary btn--sm" onclick={openAddMonitor}>
                <i class="ri-add-line"></i> Tambah Monitor Pertama
              </button>
              <button class="btn btn--ghost btn--sm" onclick={importSitesToUptime} disabled={importingSites}>
                <i class="ri-download-cloud-2-line"></i> Impor dari Daftar Situs
              </button>
            </div>
          </div>
        {:else}
          <div class="uptime-list">
            {#each uptimeMonitors as mon (mon.id)}
              {@const slots = buildHeartbeatSlots(mon.recent_checks)}
              <div class="uptime-card" class:is-paused={!mon.enabled} class:is-down={mon.enabled && mon.last_status === 'down'}>
                <!-- Baris header kartu monitor -->
                <div class="uptime-card__top">
                  <div class="uptime-card__ident">
                    <span
                      class="uptime-status-dot"
                      class:is-up={mon.enabled && mon.last_status === 'up'}
                      class:is-down={mon.enabled && mon.last_status === 'down'}
                      class:is-paused={!mon.enabled}
                      aria-hidden="true"
                    ></span>
                    <div class="uptime-card__titles">
                      <div class="uptime-card__name-row">
                        <strong class="uptime-card__name">{mon.name}</strong>
                        <span class="badge badge--muted">{mon.monitor_type.toUpperCase()}</span>
                        {#if !mon.enabled}
                          <span class="badge badge--warning">Dijeda</span>
                        {:else if mon.last_status === 'up'}
                          <span class="badge badge--success">Online</span>
                        {:else if mon.last_status === 'down'}
                          <span class="badge badge--danger">Offline</span>
                        {:else}
                          <span class="badge badge--muted">Menunggu</span>
                        {/if}
                        {#if mon.public_visible}
                          <span class="badge badge--info" title="Tampil di halaman /status">Publik</span>
                        {/if}
                        {#if mon.notify_webhook}
                          <span class="badge badge--muted" title="Notifikasi Webhook aktif">
                            <i class="ri-notification-3-line"></i> Alert
                          </span>
                        {/if}
                      </div>
                      <div class="uptime-card__target mono">{mon.target}</div>
                    </div>
                  </div>

                  <div class="uptime-card__stats">
                    {#if mon.ssl_days_remaining >= 0}
                      <span
                        class="badge {sslBadgeClass(mon.ssl_days_remaining)}"
                        title={mon.ssl_issuer ? `Penerbit SSL: ${mon.ssl_issuer}` : 'Masa berlaku sertifikat TLS'}
                      >
                        <i class="ri-lock-line"></i> SSL {mon.ssl_days_remaining}h
                      </span>
                    {/if}
                    <span class="uptime-pill mono" title="Persentase uptime 24 jam terakhir">
                      {mon.uptime_24h_pct.toFixed(2)}%
                    </span>
                    <span class="uptime-pill mono" title="Latensi pemeriksaan terakhir">
                      {mon.last_latency_ms} ms
                    </span>
                  </div>
                </div>

                <!-- Strip 30-bar heartbeat ala Uptime Kuma -->
                <div class="hb-strip" aria-label="Riwayat 30 pemeriksaan terakhir">
                  {#each slots as check, idx (check ? check.id : `empty-${mon.id}-${idx}`)}
                    <span
                      class="hb-bar"
                      class:hb-bar--up={check?.status === 'up'}
                      class:hb-bar--down={check?.status === 'down'}
                      title={formatCheckTooltip(check)}
                    ></span>
                  {/each}
                </div>

                {#if mon.enabled && mon.last_status === 'down' && mon.last_error}
                  <div class="uptime-card__error mono">
                    <i class="ri-error-warning-line"></i> {mon.last_error}
                  </div>
                {/if}

                <!-- Baris metadata & aksi cepat -->
                <div class="uptime-card__foot">
                  <div class="uptime-card__meta mono">
                    <span>Interval {mon.interval_sec}d</span>
                    <span>&middot;</span>
                    <span>Cek terakhir: {formatLastChecked(mon.last_checked_at)}</span>
                    {#if mon.avg_latency_ms > 0}
                      <span>&middot;</span>
                      <span>Avg 24j: {mon.avg_latency_ms} ms</span>
                    {/if}
                    {#if mon.keyword}
                      <span>&middot;</span>
                      <span>Keyword: "{mon.keyword}"</span>
                    {/if}
                  </div>

                  <div class="uptime-card__actions">
                    <button
                      class="btn btn--ghost btn--sm"
                      onclick={() => checkMonitorNow(mon)}
                      disabled={checkingId === mon.id}
                      aria-label="Periksa monitor sekarang"
                      title="Periksa Sekarang"
                    >
                      <i class="ri-flashlight-line {checkingId === mon.id ? 'spin' : ''}"></i>
                    </button>
                    <button
                      class="btn btn--ghost btn--sm"
                      onclick={() => toggleMonitorEnabled(mon)}
                      disabled={togglingId === mon.id}
                      aria-label={mon.enabled ? 'Jeda monitor' : 'Aktifkan monitor'}
                      title={mon.enabled ? 'Jeda Monitor' : 'Aktifkan Monitor'}
                    >
                      <i class={mon.enabled ? 'ri-pause-line' : 'ri-play-line'}></i>
                    </button>
                    <button
                      class="btn btn--ghost btn--sm"
                      onclick={() => openEditMonitor(mon)}
                      aria-label="Edit monitor"
                      title="Edit Monitor"
                    >
                      <i class="ri-edit-line"></i>
                    </button>
                    <button
                      class="btn btn--ghost btn--sm btn--danger"
                      onclick={() => deleteMonitor(mon)}
                      aria-label="Hapus monitor"
                      title="Hapus Monitor"
                    >
                      <i class="ri-delete-bin-line"></i>
                    </button>
                  </div>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<!-- Modal Tambah / Edit Uptime Monitor -->
<Modal
  open={showMonitorModal}
  title={editingMonitorId ? 'Edit Monitor Uptime & SSL' : 'Tambah Monitor Uptime & SSL'}
  subtitle="Pantau ketersediaan HTTP/HTTPS (termasuk sisa masa aktif SSL) atau port TCP secara otomatis."
  size="md"
  onclose={() => (showMonitorModal = false)}
>
  <form id="uptime-monitor-form" class="uptime-form" onsubmit={saveMonitor}>
    <div class="uptime-form__grid">
      <label class="uptime-field">
        <span class="uptime-field__label">Nama Layanan</span>
        <input
          class="input"
          type="text"
          bind:value={formName}
          placeholder="Contoh: API Produksi / Website Utama"
          required
        />
      </label>

      <label class="uptime-field">
        <span class="uptime-field__label">Tipe Pemeriksaan</span>
        <select class="input" bind:value={formType}>
          <option value="http">HTTP / HTTPS (+ Inspeksi SSL)</option>
          <option value="tcp">TCP Port Dial</option>
        </select>
      </label>
    </div>

    <label class="uptime-field">
      <span class="uptime-field__label">
        {formType === 'tcp' ? 'Target Host:Port' : 'Target URL (HTTP/HTTPS)'}
      </span>
      <input
        class="input mono"
        type="text"
        bind:value={formTarget}
        placeholder={formType === 'tcp' ? '127.0.0.1:3306' : 'https://domainanda.com/healthz'}
        required
      />
    </label>

    <div class="uptime-form__grid">
      <label class="uptime-field">
        <span class="uptime-field__label">Interval Pemeriksaan</span>
        <select class="input" bind:value={formInterval}>
          <option value={30}>Setiap 30 detik</option>
          <option value={60}>Setiap 1 menit (Rekomendasi)</option>
          <option value={180}>Setiap 3 menit</option>
          <option value={300}>Setiap 5 menit</option>
          <option value={900}>Setiap 15 menit</option>
        </select>
      </label>

      <label class="uptime-field">
        <span class="uptime-field__label">Timeout (detik)</span>
        <input class="input" type="number" min="2" max="30" bind:value={formTimeout} />
      </label>
    </div>

    {#if formType === 'http'}
      <div class="uptime-form__grid">
        <label class="uptime-field">
          <span class="uptime-field__label">Kode HTTP Diharapkan (0 = 2xx–3xx)</span>
          <input class="input" type="number" min="0" max="599" bind:value={formExpectedStatus} />
        </label>

        <label class="uptime-field">
          <span class="uptime-field__label">Kata Kunci Respons (Opsional)</span>
          <input class="input" type="text" bind:value={formKeyword} placeholder="Contoh: status:ok" />
        </label>
      </div>
    {/if}

    <div class="uptime-checks">
      <label class="uptime-check">
        <input type="checkbox" bind:checked={formNotifyWebhook} />
        <span>Kirim peringatan ke Webhook aktif (Telegram / Discord / Custom) saat DOWN, Pulih, atau SSL kritis</span>
      </label>
      <label class="uptime-check">
        <input type="checkbox" bind:checked={formPublicVisible} />
        <span>Tampilkan pada Halaman Status Publik (<code>/status</code>) tanpa perlu login</span>
      </label>
      <label class="uptime-check">
        <input type="checkbox" bind:checked={formEnabled} />
        <span>Aktifkan pemantauan segera setelah disimpan</span>
      </label>
    </div>
  </form>

  {#snippet footer()}
    <button type="button" class="btn btn--ghost" onclick={() => (showMonitorModal = false)}>
      Batal
    </button>
    <button type="submit" form="uptime-monitor-form" class="btn btn--primary" disabled={savingMonitor}>
      <i class="ri-save-line"></i> {savingMonitor ? 'Menyimpan...' : 'Simpan Monitor'}
    </button>
  {/snippet}
</Modal>
