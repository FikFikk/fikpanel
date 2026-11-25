<script lang="ts">
  import { onMount } from 'svelte';
  import PageHeader from '../lib/components/ui/PageHeader.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import type { HistoryChartMode } from '../lib/components/HistoryChart.svelte';
  import HistoryTab from './monitor/HistoryTab.svelte';
  import RealtimeTab from './monitor/RealtimeTab.svelte';
  import UptimeTab from './monitor/UptimeTab.svelte';
  import AlertsTab from './monitor/AlertsTab.svelte';
  import { formatBytes, apiFetch } from '../lib/api';
  import type {
    MetricPoint,
    MonitorAlert,
    MonitorConfig,
    MonitorHistorySummary,
    MonitorSnapshot,
    MonitorThresholds,
    SystemProcess,
    UptimeMonitor,
    UptimeMonitorHistory,
  } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import '../styles/pages/monitor.css';

  interface Props {
    currentMetric: MetricPoint;
    history: MetricPoint[];
  }

  let { currentMetric, history }: Props = $props();

  type MonitorTabId = 'history' | 'realtime' | 'uptime' | 'alerts';
  let activeTab = $state<MonitorTabId>('history');

  // State histori persisten & analitik
  let snapshots = $state<MonitorSnapshot[]>([]);
  let summary = $state<MonitorHistorySummary | null>(null);
  let loadingHistory = $state(false);
  let timeRange = $state<'1h' | '6h' | '24h' | '7d' | '30d'>('24h');
  let chartMode = $state<HistoryChartMode>('resource');
  let historyWindowStart = $state(0);
  let historyWindowEnd = $state(0);

  // Konfigurasi perekaman & ambang batas alert
  let monitorConfig = $state<MonitorConfig>({ enabled: true, save_days: 30 });
  let updatingConfig = $state(false);
  let thresholds = $state<MonitorThresholds>({
    alert_cpu_pct: 85,
    alert_ram_pct: 85,
    alert_disk_pct: 90,
    alert_ssl_days: 14,
    alert_cooldown_min: 15,
  });

  // State Top 5 proses aktif untuk tab Live Telemetry
  let topProcesses = $state<SystemProcess[]>([]);
  let loadingProcs = $state(false);

  // State Uptime & SSL Monitor
  let uptimeMonitors = $state<UptimeMonitor[]>([]);
  let loadingUptime = $state(false);
  let importingSites = $state(false);
  let checkingAll = $state(false);
  let checkingId = $state<number | null>(null);
  let togglingId = $state<number | null>(null);

  // State Modal Detail & Latensi Uptime
  let detailOpen = $state(false);
  let detailLoading = $state(false);
  let uptimeDetail = $state<UptimeMonitorHistory | null>(null);

  // State Modal Tambah/Edit Uptime Monitor
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

  // State Alert & Insiden Terpadu
  let alerts = $state<MonitorAlert[]>([]);
  let unackedCount = $state(0);
  let webhooksCount = $state(0);
  let loadingAlerts = $state(false);
  let savingThresholds = $state(false);

  // Nilai derivasi KPI global header
  let ramPct = $derived(
    currentMetric.ram_total > 0 ? (currentMetric.ram_used / currentMetric.ram_total) * 100 : 0
  );
  let diskPct = $derived(
    currentMetric.disk_total > 0 ? (currentMetric.disk_used / currentMetric.disk_total) * 100 : 0
  );
  let load5m = $derived(currentMetric.load_5m ?? currentMetric.load_avg);
  let downCount = $derived(
    uptimeMonitors.filter((m) => m.enabled && m.last_status === 'down').length
  );
  let exportUrl = $derived(
    `/api/monitor/history/export?start=${historyWindowStart}&end=${historyWindowEnd}`
  );

  // Path SVG untuk grafik latensi di modal detail Uptime
  let detailSvgPaths = $derived.by(() => {
    const checks = (uptimeDetail?.checks || []).filter((c) => c.latency_ms >= 0);
    if (checks.length < 2) return null;
    const w = 520;
    const h = 110;
    const pad = 8;
    const maxLat = Math.max(...checks.map((c) => c.latency_ms), 50) * 1.15;
    const pts = checks.map((c, i) => {
      const x = pad + (i / (checks.length - 1)) * (w - pad * 2);
      const y = h - pad - (c.latency_ms / maxLat) * (h - pad * 2);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    });
    const line = `M ${pts.join(' L ')}`;
    const area = `${line} L ${(w - pad).toFixed(1)},${(h - pad).toFixed(1)} L ${pad.toFixed(1)},${(h - pad).toFixed(1)} Z`;
    return { line, area, maxLat: Math.round(maxLat) };
  });

  // ── Pengambilan Data ──────────────────────────────────────────────────────

  async function loadMonitorConfig() {
    try {
      const res = await apiFetch<{ ok: boolean; config: MonitorConfig }>('/api/monitor/config');
      if (res?.config) monitorConfig = res.config;
    } catch {
      // Abaikan kegagalan awal agar UI tetap tampil
    }
  }

  async function updateMonitorConfig(enabled: boolean, saveDays: number) {
    try {
      updatingConfig = true;
      const res = await apiFetch<{ ok: boolean; config?: MonitorConfig; message?: string }>(
        '/api/monitor/config',
        {
          method: 'POST',
          body: JSON.stringify({ enabled, save_days: saveDays }),
        }
      );
      monitorConfig = res?.config || { enabled, save_days: saveDays };
      notify(res?.message || 'Konfigurasi monitoring diperbarui', 'success');
    } catch (err: any) {
      notify(err?.message || 'Gagal memperbarui konfigurasi monitoring', 'error');
    } finally {
      updatingConfig = false;
    }
  }

  async function loadHistory() {
    try {
      loadingHistory = true;
      const now = Math.floor(Date.now() / 1000);
      let duration = 86400;
      if (timeRange === '1h') duration = 3600;
      else if (timeRange === '6h') duration = 21600;
      else if (timeRange === '7d') duration = 86400 * 7;
      else if (timeRange === '30d') duration = 86400 * 30;

      const start = now - duration;
      historyWindowStart = start;
      historyWindowEnd = now;
      const res = await apiFetch<{
        ok: boolean;
        history: MonitorSnapshot[];
        summary?: MonitorHistorySummary;
        thresholds?: MonitorThresholds;
      }>(`/api/monitor/history?start=${start}&end=${now}&max_points=360`);
      snapshots = res?.history || [];
      summary = res?.summary || null;
      if (res?.thresholds) thresholds = res.thresholds;
    } catch (err: any) {
      notify(err?.message || 'Gagal memuat histori monitoring', 'error');
    } finally {
      loadingHistory = false;
    }
  }

  async function clearHistory() {
    const ok = await confirmDialog({
      title: 'Kosongkan Riwayat Monitoring',
      message: 'Hapus seluruh rekaman histori metrik server dari database SQLite?',
      confirmText: 'Kosongkan Riwayat',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch('/api/monitor/history', { method: 'DELETE' });
      snapshots = [];
      summary = null;
      notify('Seluruh data histori monitoring berhasil dikosongkan', 'success');
    } catch (err: any) {
      notify(err?.message || 'Gagal mengosongkan histori', 'error');
    }
  }

  async function loadTopProcesses(silent = false) {
    try {
      if (!silent) loadingProcs = true;
      const res = await apiFetch<{ ok: boolean; processes: SystemProcess[] }>(
        '/api/system/processes?limit=5&sort=cpu'
      );
      topProcesses = res?.processes || [];
    } catch {
      // Abaikan saat non-Linux atau proses sementara sibuk
    } finally {
      if (!silent) loadingProcs = false;
    }
  }

  async function loadUptimeMonitors(silent = false) {
    try {
      if (!silent) loadingUptime = true;
      const res = await apiFetch<{ ok: boolean; monitors: UptimeMonitor[] }>('/api/uptime/monitors');
      uptimeMonitors = res?.monitors || [];
    } catch (err: any) {
      if (!silent) notify(err?.message || 'Gagal memuat monitor uptime', 'error');
    } finally {
      if (!silent) loadingUptime = false;
    }
  }

  async function loadAlerts(silent = false) {
    try {
      if (!silent) loadingAlerts = true;
      const res = await apiFetch<{
        ok: boolean;
        alerts: MonitorAlert[];
        unacked_count: number;
        webhooks_count: number;
        thresholds?: MonitorThresholds;
      }>('/api/monitor/alerts?limit=100');
      alerts = res?.alerts || [];
      unackedCount = res?.unacked_count ?? 0;
      webhooksCount = res?.webhooks_count ?? 0;
      if (res?.thresholds) thresholds = res.thresholds;
    } catch (err: any) {
      if (!silent) notify(err?.message || 'Gagal memuat riwayat alert', 'error');
    } finally {
      if (!silent) loadingAlerts = false;
    }
  }

  // ── Aksi Uptime & SSL Monitor ─────────────────────────────────────────────

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
      notify('Nama layanan dan target URL/Host wajib diisi', 'error');
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
        notify('Konfigurasi monitor berhasil diperbarui', 'success');
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
      notify(err?.message || 'Gagal menyimpan monitor', 'error');
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
      loadAlerts(true);
    } catch (err: any) {
      notify(err?.message || 'Gagal menjalankan pemeriksaan langsung', 'error');
    } finally {
      checkingId = null;
    }
  }

  async function checkAllMonitorsNow() {
    try {
      checkingAll = true;
      const res = await apiFetch<{ ok: boolean; monitors?: UptimeMonitor[]; message?: string }>(
        '/api/uptime/check-all',
        { method: 'POST', timeoutMs: 45_000 }
      );
      if (res?.monitors) uptimeMonitors = res.monitors;
      notify(res?.message || 'Seluruh monitor selesai diperiksa', 'success');
      loadAlerts(true);
    } catch (err: any) {
      notify(err?.message || 'Gagal memeriksa seluruh monitor', 'error');
    } finally {
      checkingAll = false;
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
      notify(err?.message || 'Gagal mengubah status monitor', 'error');
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
      notify(err?.message || 'Gagal menghapus monitor', 'error');
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
      notify(err?.message || 'Gagal mengimpor situs', 'error');
    } finally {
      importingSites = false;
    }
  }

  async function openUptimeDetail(m: UptimeMonitor) {
    detailOpen = true;
    detailLoading = true;
    uptimeDetail = null;
    try {
      const res = await apiFetch<{ ok: boolean; detail: UptimeMonitorHistory }>(
        `/api/uptime/monitors/${m.id}/history?limit=120`
      );
      uptimeDetail = res?.detail || null;
    } catch (err: any) {
      notify(err?.message || 'Gagal memuat detail latensi monitor', 'error');
      detailOpen = false;
    } finally {
      detailLoading = false;
    }
  }

  // ── Aksi Alert & Ambang Batas ─────────────────────────────────────────────

  async function saveAlertThresholds(next: MonitorThresholds) {
    try {
      savingThresholds = true;
      const res = await apiFetch<{ ok: boolean; thresholds?: MonitorThresholds; message?: string }>(
        '/api/monitor/alerts/thresholds',
        {
          method: 'POST',
          body: JSON.stringify(next),
        }
      );
      if (res?.thresholds) thresholds = res.thresholds;
      notify(res?.message || 'Ambang batas alert diperbarui', 'success');
    } catch (err: any) {
      notify(err?.message || 'Gagal menyimpan ambang alert', 'error');
    } finally {
      savingThresholds = false;
    }
  }

  async function ackAlert(id: number) {
    try {
      await apiFetch(`/api/monitor/alerts/${id}/ack`, { method: 'POST' });
      alerts = alerts.map((a) => (a.id === id ? { ...a, acknowledged: true } : a));
      unackedCount = Math.max(0, unackedCount - 1);
    } catch (err: any) {
      notify(err?.message || 'Gagal menandai alert', 'error');
    }
  }

  async function ackAllAlerts() {
    try {
      await apiFetch('/api/monitor/alerts/ack-all', { method: 'POST' });
      alerts = alerts.map((a) => ({ ...a, acknowledged: true }));
      unackedCount = 0;
      notify('Seluruh alert ditandai sudah dibaca', 'success');
    } catch (err: any) {
      notify(err?.message || 'Gagal menandai seluruh alert', 'error');
    }
  }

  async function clearAllAlerts() {
    const ok = await confirmDialog({
      title: 'Kosongkan Riwayat Alert & Insiden',
      message: 'Hapus seluruh catatan log alert resource dan insiden Uptime/SSL dari database?',
      confirmText: 'Kosongkan Log',
      tone: 'danger',
    });
    if (!ok) return;
    try {
      await apiFetch('/api/monitor/alerts', { method: 'DELETE' });
      alerts = [];
      unackedCount = 0;
      notify('Riwayat alert & insiden berhasil dikosongkan', 'success');
    } catch (err: any) {
      notify(err?.message || 'Gagal mengosongkan riwayat alert', 'error');
    }
  }

  function formatCheckTime(ts: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString('id-ID', {
      day: '2-digit',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  // Muat ulang histori otomatis saat rentang waktu berubah
  $effect(() => {
    if (activeTab === 'history') {
      const _tr = timeRange;
      loadHistory();
    }
  });

  // Auto-refresh daftar top proses setiap 5 detik saat tab Live Telemetry dibuka
  $effect(() => {
    if (activeTab !== 'realtime') return;
    loadTopProcesses(topProcesses.length > 0);
    const id = setInterval(() => loadTopProcesses(true), 5000);
    return () => clearInterval(id);
  });

  // Auto-refresh Uptime setiap 20 detik saat tab Uptime dibuka
  $effect(() => {
    if (activeTab !== 'uptime') return;
    loadUptimeMonitors(uptimeMonitors.length > 0);
    const id = setInterval(() => loadUptimeMonitors(true), 20000);
    return () => clearInterval(id);
  });

  // Muat riwayat alert saat berpindah ke tab Alerts
  $effect(() => {
    if (activeTab === 'alerts') {
      loadAlerts(alerts.length > 0);
    }
  });

  onMount(() => {
    loadMonitorConfig();
    loadUptimeMonitors(true);
    loadAlerts(true);
  });
</script>

<div class="pg-monitor">
  <PageHeader
    title="Observability, Uptime &amp; Alert Hub"
    subtitle="Analitik histori downsampled, telemetri live CPU/IOWait/Per-Core/Disk I/O, Uptime &amp; SSL, serta mesin alert ambang otomatis."
    icon="ri-line-chart-line"
  >
    {#snippet actions()}
      <div class="mon-switch-box">
        <span class="mon-switch-label">Perekaman 60d:</span>
        <button
          type="button"
          class="mon-switch-btn"
          class:is-on={monitorConfig.enabled}
          aria-label="Aktifkan atau nonaktifkan perekaman histori monitoring"
          disabled={updatingConfig}
          onclick={() => updateMonitorConfig(!monitorConfig.enabled, monitorConfig.save_days)}
        >
          <span class="mon-switch-knob"></span>
        </button>
        <span class="mon-switch-state {monitorConfig.enabled ? 'text-accent' : 'text-subtle'}">
          {monitorConfig.enabled ? 'AKTIF' : 'MATI'}
        </span>
      </div>
    {/snippet}
  </PageHeader>

  <!-- Navigasi 4 Tab Terpadu -->
  <div class="tabs" role="tablist" aria-label="Navigasi Observability Hub">
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={activeTab === 'history'}
      aria-selected={activeTab === 'history'}
      onclick={() => (activeTab = 'history')}
    >
      <i class="ri-history-line" aria-hidden="true"></i>
      <span>Rekaman Histori</span>
    </button>

    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={activeTab === 'realtime'}
      aria-selected={activeTab === 'realtime'}
      onclick={() => (activeTab = 'realtime')}
    >
      <i class="ri-pulse-line" aria-hidden="true"></i>
      <span>Live Telemetry (1s)</span>
    </button>

    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={activeTab === 'uptime'}
      aria-selected={activeTab === 'uptime'}
      onclick={() => (activeTab = 'uptime')}
    >
      <i class="ri-heart-pulse-line" aria-hidden="true"></i>
      <span>Uptime &amp; SSL</span>
      {#if downCount > 0}
        <span class="mon-tab-count mon-tab-count--danger">{downCount}</span>
      {:else if uptimeMonitors.length > 0}
        <span class="mon-tab-count">{uptimeMonitors.length}</span>
      {/if}
    </button>

    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={activeTab === 'alerts'}
      aria-selected={activeTab === 'alerts'}
      onclick={() => (activeTab = 'alerts')}
    >
      <i class="ri-alarm-warning-line" aria-hidden="true"></i>
      <span>Alert &amp; Insiden</span>
      {#if unackedCount > 0}
        <span class="mon-tab-count mon-tab-count--danger">{unackedCount}</span>
      {/if}
    </button>
  </div>

  <!-- Strip 4 Kartu Live Telemetry saat berada di tab Histori atau Live -->
  {#if activeTab === 'history' || activeTab === 'realtime'}
    <div class="mon-kpi-grid">
      <div class="stat-card">
        <div
          class="stat-card__icon"
          class:stat-card__icon--danger={currentMetric.cpu >= thresholds.alert_cpu_pct}
          class:stat-card__icon--info={currentMetric.cpu < thresholds.alert_cpu_pct}
        >
          <i class="ri-cpu-line" aria-hidden="true"></i>
        </div>
        <div class="stat-card__body">
          <span class="stat-card__label">CPU &amp; Load Average</span>
          <span class="stat-card__value mono">{(currentMetric.cpu || 0).toFixed(1)}%</span>
          <span class="stat-card__sub">
            Load {currentMetric.load_avg.toFixed(2)} · {load5m.toFixed(2)} · IOWait {(currentMetric.cpu_iowait || 0).toFixed(1)}%
          </span>
        </div>
      </div>

      <div class="stat-card">
        <div
          class="stat-card__icon"
          class:stat-card__icon--danger={ramPct >= thresholds.alert_ram_pct}
          class:stat-card__icon--violet={ramPct < thresholds.alert_ram_pct}
        >
          <i class="ri-ram-line" aria-hidden="true"></i>
        </div>
        <div class="stat-card__body">
          <span class="stat-card__label">Memori RAM Fisik</span>
          <span class="stat-card__value mono">{ramPct.toFixed(1)}%</span>
          <span class="stat-card__sub">
            {formatBytes(currentMetric.ram_used)} / {formatBytes(currentMetric.ram_total)}
          </span>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-card__icon">
          <i class="ri-global-line" aria-hidden="true"></i>
        </div>
        <div class="stat-card__body">
          <span class="stat-card__label">Jaringan (In / Out)</span>
          <span class="stat-card__value mono">{formatBytes(currentMetric.net_in || 0)}/s</span>
          <span class="stat-card__sub">Keluar: {formatBytes(currentMetric.net_out || 0)}/s</span>
        </div>
      </div>

      <div class="stat-card">
        <div
          class="stat-card__icon"
          class:stat-card__icon--danger={diskPct >= thresholds.alert_disk_pct}
          class:stat-card__icon--warning={diskPct < thresholds.alert_disk_pct}
        >
          <i class="ri-hard-drive-2-line" aria-hidden="true"></i>
        </div>
        <div class="stat-card__body">
          <span class="stat-card__label">Disk Root &amp; Throughput</span>
          <span class="stat-card__value mono">{diskPct.toFixed(1)}%</span>
          <span class="stat-card__sub">
            R {formatBytes(currentMetric.disk_read || 0)}/s · W {formatBytes(currentMetric.disk_write || 0)}/s
          </span>
        </div>
      </div>
    </div>
  {/if}

  <!-- Konten Tab Aktif -->
  {#if activeTab === 'history'}
    <HistoryTab
      {snapshots}
      {summary}
      {thresholds}
      {monitorConfig}
      {loadingHistory}
      {updatingConfig}
      bind:timeRange
      bind:chartMode
      {exportUrl}
      onRefresh={loadHistory}
      onClearHistory={clearHistory}
      onChangeRetention={(days) => updateMonitorConfig(monitorConfig.enabled, days)}
    />
  {:else if activeTab === 'realtime'}
    <RealtimeTab
      {currentMetric}
      {history}
      {topProcesses}
      {loadingProcs}
      onRefreshProcs={() => loadTopProcesses(false)}
    />
  {:else if activeTab === 'uptime'}
    <UptimeTab
      monitors={uptimeMonitors}
      loading={loadingUptime}
      importing={importingSites}
      {checkingAll}
      {checkingId}
      {togglingId}
      onRefresh={() => loadUptimeMonitors(false)}
      onCheckAll={checkAllMonitorsNow}
      onImportSites={importSitesToUptime}
      onAdd={openAddMonitor}
      onEdit={openEditMonitor}
      onCheckNow={checkMonitorNow}
      onToggleEnabled={toggleMonitorEnabled}
      onDelete={deleteMonitor}
      onInspect={openUptimeDetail}
    />
  {:else}
    <AlertsTab
      {alerts}
      {unackedCount}
      {webhooksCount}
      {thresholds}
      loading={loadingAlerts}
      {savingThresholds}
      onRefresh={() => loadAlerts(false)}
      onSaveThresholds={saveAlertThresholds}
      onAck={ackAlert}
      onAckAll={ackAllAlerts}
      onClearAll={clearAllAlerts}
    />
  {/if}

  <!-- Modal Tambah / Edit Uptime Monitor -->
  <Modal
    open={showMonitorModal}
    title={editingMonitorId ? 'Edit Monitor Uptime & SSL' : 'Tambah Monitor Uptime & SSL'}
    subtitle="Pantau ketersediaan HTTP/HTTPS (termasuk sisa masa aktif SSL) atau port TCP secara otomatis."
    size="md"
    onclose={() => (showMonitorModal = false)}
  >
    <form id="uptime-monitor-form" class="stack" onsubmit={saveMonitor}>
      <div class="mon-form-grid">
        <div class="field">
          <label class="field__label" for="mon-name">Nama Layanan</label>
          <input
            id="mon-name"
            class="input"
            type="text"
            bind:value={formName}
            placeholder="Contoh: API Produksi / Website Utama"
            required
          />
        </div>

        <div class="field">
          <label class="field__label" for="mon-type">Tipe Pemeriksaan</label>
          <select id="mon-type" class="select" bind:value={formType}>
            <option value="http">HTTP / HTTPS (+ Inspeksi SSL)</option>
            <option value="tcp">TCP Port Dial</option>
          </select>
        </div>
      </div>

      <div class="field">
        <label class="field__label" for="mon-target">
          {formType === 'tcp' ? 'Target Host:Port' : 'Target URL (HTTP/HTTPS)'}
        </label>
        <input
          id="mon-target"
          class="input input--mono"
          type="text"
          bind:value={formTarget}
          placeholder={formType === 'tcp' ? '127.0.0.1:3306' : 'https://domainanda.com/healthz'}
          required
        />
      </div>

      <div class="mon-form-grid">
        <div class="field">
          <label class="field__label" for="mon-interval">Interval Pemeriksaan</label>
          <select id="mon-interval" class="select" bind:value={formInterval}>
            <option value={30}>Setiap 30 detik</option>
            <option value={60}>Setiap 1 menit (Rekomendasi)</option>
            <option value={180}>Setiap 3 menit</option>
            <option value={300}>Setiap 5 menit</option>
            <option value={900}>Setiap 15 menit</option>
          </select>
        </div>

        <div class="field">
          <label class="field__label" for="mon-timeout">Timeout (detik)</label>
          <input id="mon-timeout" class="input input--mono" type="number" min="2" max="30" bind:value={formTimeout} />
        </div>
      </div>

      {#if formType === 'http'}
        <div class="mon-form-grid">
          <div class="field">
            <label class="field__label" for="mon-code">Kode HTTP Diharapkan (0 = 2xx–3xx)</label>
            <input id="mon-code" class="input input--mono" type="number" min="0" max="599" bind:value={formExpectedStatus} />
          </div>

          <div class="field">
            <label class="field__label" for="mon-kw">Kata Kunci Respons (Opsional)</label>
            <input id="mon-kw" class="input" type="text" bind:value={formKeyword} placeholder="Contoh: status:ok" />
          </div>
        </div>
      {/if}

      <div class="mon-checks-box">
        <label class="mon-check-row">
          <input type="checkbox" bind:checked={formNotifyWebhook} />
          <span>Kirim peringatan ke Webhook aktif saat DOWN, Pulih, atau SSL mendekati kedaluwarsa</span>
        </label>
        <label class="mon-check-row">
          <input type="checkbox" bind:checked={formPublicVisible} />
          <span>Tampilkan pada Halaman Status Publik (<code>/status</code>) tanpa perlu login</span>
        </label>
        <label class="mon-check-row">
          <input type="checkbox" bind:checked={formEnabled} />
          <span>Aktifkan pemantauan segera setelah disimpan</span>
        </label>
      </div>
    </form>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary btn--sm" onclick={() => (showMonitorModal = false)}>
        Batal
      </button>
      <button type="submit" form="uptime-monitor-form" class="btn btn--primary btn--sm" disabled={savingMonitor}>
        <i class="ri-save-line" aria-hidden="true"></i>
        <span>{savingMonitor ? 'Menyimpan...' : 'Simpan Monitor'}</span>
      </button>
    {/snippet}
  </Modal>

  <!-- Modal Detail & Analitik Latensi Uptime Monitor -->
  <Modal
    open={detailOpen}
    title={uptimeDetail?.monitor ? `Detail Latensi: ${uptimeDetail.monitor.name}` : 'Detail Monitor Uptime'}
    subtitle={uptimeDetail?.monitor?.target || 'Analitik persentil latensi dan histori pemeriksaan terbaru.'}
    size="lg"
    onclose={() => (detailOpen = false)}
  >
    {#if detailLoading}
      <div class="state state--loading">
        <i class="ri-loader-4-line spin" aria-hidden="true"></i>
        <div class="state__title">Memuat Analitik Latensi &amp; Riwayat Pemeriksaan...</div>
      </div>
    {:else if uptimeDetail}
      <div class="stack">
        <div class="mon-kpi-grid">
          <div class="mon-stat-pill">
            <span class="mon-stat-pill__label">SLA 24 Jam / 7 Hari</span>
            <span class="mon-stat-pill__val">
              {uptimeDetail.uptime_24h_pct.toFixed(2)}% · {uptimeDetail.uptime_7d_pct.toFixed(2)}%
            </span>
            <span class="mon-stat-pill__sub">{uptimeDetail.down_count_24h} gagal dalam 24j</span>
          </div>
          <div class="mon-stat-pill">
            <span class="mon-stat-pill__label">Latensi Min / Avg</span>
            <span class="mon-stat-pill__val">{uptimeDetail.min_latency_ms} / {uptimeDetail.avg_latency_ms} ms</span>
            <span class="mon-stat-pill__sub">Dari {uptimeDetail.checks.length} cek terakhir</span>
          </div>
          <div class="mon-stat-pill">
            <span class="mon-stat-pill__label">Latensi P95</span>
            <span class="mon-stat-pill__val">{uptimeDetail.p95_latency_ms} ms</span>
            <span class="mon-stat-pill__sub">Persentil ke-95</span>
          </div>
          <div class="mon-stat-pill">
            <span class="mon-stat-pill__label">Latensi Puncak (Max)</span>
            <span class="mon-stat-pill__val">{uptimeDetail.max_latency_ms} ms</span>
            <span class="mon-stat-pill__sub">
              {uptimeDetail.monitor.ssl_days_remaining >= 0
                ? `SSL ${uptimeDetail.monitor.ssl_days_remaining} hari`
                : 'Tanpa TLS'}
            </span>
          </div>
        </div>

        {#if detailSvgPaths}
          <div class="mon-lat-svg-wrap">
            <div class="row-between text-xs text-subtle mono">
              <span>Tren Latensi ({uptimeDetail.checks.length} pemeriksaan terakhir)</span>
              <span>Skala Maks: {detailSvgPaths.maxLat} ms</span>
            </div>
            <svg viewBox="0 0 520 110" class="mon-lat-svg" preserveAspectRatio="none" aria-label="Grafik tren latensi">
              <path d={detailSvgPaths.area} class="mon-lat-area"></path>
              <path d={detailSvgPaths.line} class="mon-lat-path"></path>
            </svg>
          </div>
        {/if}

        <div class="table-wrap">
          <table class="table table--dense">
            <thead>
              <tr>
                <th>Waktu</th>
                <th>Status</th>
                <th class="mon-col-right">Kode</th>
                <th class="mon-col-right">Latensi</th>
                <th>Catatan</th>
              </tr>
            </thead>
            <tbody>
              {#each [...uptimeDetail.checks].reverse().slice(0, 25) as chk (chk.id)}
                <tr>
                  <td class="mono text-xs">{formatCheckTime(chk.checked_at)}</td>
                  <td>
                    {#if chk.status === 'up'}
                      <span class="badge badge--success">ONLINE</span>
                    {:else}
                      <span class="badge badge--danger">OFFLINE</span>
                    {/if}
                  </td>
                  <td class="mono mon-col-right">{chk.status_code > 1 ? chk.status_code : '-'}</td>
                  <td class="mono tabular mon-col-right">{chk.latency_ms} ms</td>
                  <td class="mono text-xs text-muted">{chk.error_msg || 'OK'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {/if}

    {#snippet footer()}
      <button type="button" class="btn btn--secondary btn--sm" onclick={() => (detailOpen = false)}>
        Tutup
      </button>
    {/snippet}
  </Modal>
</div>
