<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch, formatBytes } from '../lib/api';
  import type { Site, SiteKind, SoftwareItem, SitesData, NginxStatus } from '../lib/types';
  import PageHeader from '../lib/components/ui/PageHeader.svelte';
  import NginxManager from '../lib/components/NginxManager.svelte';
  import AddSiteModal from '../lib/components/AddSiteModal.svelte';
  import SiteSettings from '../lib/components/SiteSettings.svelte';
  import SitesTable from './sites/SitesTable.svelte';
  import LogsModal from './sites/LogsModal.svelte';
  import GitPullModal from './sites/GitPullModal.svelte';
  import DeleteModal from './sites/DeleteModal.svelte';
  import '../styles/pages/sites.css';

  interface Props {
    onNotify: (msg: string) => void;
  }

  let { onNotify }: Props = $props();

  // State Situs dan Web Server
  let sites = $state<Site[]>([]);
  let webServer = $state<SoftwareItem>({
    name: 'nginx',
    display_name: 'Nginx Web Server',
    category: 'webserver',
    installed: true,
    version: '1.24.0',
    running: true,
    service_unit: 'nginx.service',
  });
  let loading = $state(true);
  let installingNginx = $state(false);
  let showNginxManager = $state(false);

  // Sinkronkan badge web server dengan status terbaru dari Nginx Manager
  function onNginxStatus(st: NginxStatus) {
    webServer = { ...webServer, installed: st.installed, running: st.running, version: st.version };
  }

  // Filter dinamis, kartu kesehatan, urutan, dan mode tampilan
  type HealthFilter = 'all' | 'online' | 'issues' | 'git';
  type SortMode = 'status' | 'domain' | 'traffic' | 'recent';

  let activeRuntime = $state<'all' | SiteKind>('all');
  let healthFilter = $state<HealthFilter>('all');
  let sortBy = $state<SortMode>('status');
  let viewMode = $state<'table' | 'grid'>('table');
  let searchQuery = $state('');
  let selectedIds = $state<Set<number>>(new Set());

  // Modal Tambah Situs & Pengaturan Situs (Domain, .env, Runtime, SSL, Konfigurasi)
  let showAddModal = $state(false);
  let settingsTarget = $state<{ id: number; tab: string } | null>(null);

  // Modal Log Akses / Error Situs (Real Nginx Logs + Auto-Refresh Live)
  let showLogModal = $state(false);
  let logTarget = $state<Site | null>(null);
  let logTab = $state<'access' | 'error'>('access');
  let logContent = $state('');
  let logPath = $state('');
  let logLoading = $state(false);
  let logLive = $state(true);

  // Modal Git Pull & Auto-Deploy (khusus situs yang terhubung Git)
  interface GitPullResponse {
    ok: boolean;
    domain: string;
    branch: string;
    repo: string;
    old_commit: string;
    new_commit: string;
    commit_message: string;
    updated: boolean;
    output: string;
    health_ok?: boolean;
    health_code?: number;
    health_latency_ms?: number;
    health_summary?: string;
  }
  let gitPullTarget = $state<Site | null>(null);
  let gitPullRunning = $state(false);
  let gitPullCustomCmd = $state('');
  let gitPullResult = $state<GitPullResponse | null>(null);
  let gitPullError = $state('');
  let openMoreSiteId = $state<number | null>(null);
  let renewingAllSSL = $state(false);
  let refreshingSites = $state(false);
  let backingUpSiteId = $state<number | null>(null);
  let restartingSiteId = $state<number | null>(null);
  let bulkBusy = $state(false);

  // Metadata tampilan untuk chip runtime dinamis
  const RUNTIME_META: Record<SiteKind, { label: string; icon: string; cls: string }> = {
    php: { label: 'PHP', icon: 'ri-code-s-slash-line', cls: 'icon-php' },
    node: { label: 'Node.js', icon: 'ri-nodejs-line', cls: 'icon-node' },
    go: { label: 'Go', icon: 'ri-braces-line', cls: 'icon-go' },
    python: { label: 'Python', icon: 'ri-terminal-box-line', cls: 'icon-python' },
    static: { label: 'Static / SPA', icon: 'ri-html5-line', cls: 'icon-static' },
    proxy: { label: 'Proxy', icon: 'ri-shuffle-line', cls: 'icon-proxy' },
  };

  function toggleMoreMenu(siteId: number, e: MouseEvent) {
    e.stopPropagation();
    openMoreSiteId = openMoreSiteId === siteId ? null : siteId;
  }

  // Daftar snippet perintah dinamis pada modal Git Pull
  const gitPullSnippets = [
    { label: '+ composer install', cmd: 'composer install --no-interaction --optimize-autoloader', phpOnly: true },
    { label: '+ php artisan migrate', cmd: 'php artisan migrate --force', phpOnly: true },
    { label: '+ artisan optimize', cmd: 'php artisan optimize && php artisan filament:optimize', phpOnly: true },
    { label: '+ queue:restart', cmd: 'php artisan queue:restart', phpOnly: true },
    { label: '+ npm run build', cmd: 'NODE_OPTIONS=--max-old-space-size=768 npm run build', phpOnly: false },
  ];

  function hasGitSnippet(snippetCmd: string): boolean {
    const parts = gitPullCustomCmd.split('&&').map((p) => p.trim());
    return parts.includes(snippetCmd);
  }

  function toggleGitSnippet(snippetCmd: string) {
    const branch = gitPullTarget?.git_branch || 'main';
    const basePull = `git fetch origin ${branch} && git reset --hard origin/${branch}`;
    const parts = gitPullCustomCmd
      .split('&&')
      .map((p) => p.trim())
      .filter(Boolean);
    const idx = parts.indexOf(snippetCmd);
    if (idx !== -1) {
      parts.splice(idx, 1);
    } else {
      if (parts.length === 0) parts.push(basePull);
      parts.push(snippetCmd);
    }
    gitPullCustomCmd = parts.join(' && ');
  }

  function resetGitPullCmd() {
    gitPullCustomCmd = '';
  }

  // Modal Konfirmasi Hapus Situs (Danger Zone + Anti-Paste + Auto-Backup)
  let deleteTarget = $state<Site | null>(null);
  let deleteConfirmInput = $state('');
  let deleteFilesOpt = $state(true);
  let deleteDBOpt = $state(true);
  let backupFirstOpt = $state(true);
  let deletePasteWarning = $state(false);
  let deletingSite = $state(false);

  // Hitung KPI kesehatan situs secara real-time
  let kpiCounts = $derived.by(() => {
    let online = 0;
    let issues = 0;
    let gitCount = 0;
    for (const s of sites) {
      const st = s.live_status || (s.enabled === 1 ? 'running' : 'stopped');
      if (st === 'running') online++;
      else if (st === 'down' || st === 'degraded') issues++;
      if (s.has_git) gitCount++;
    }
    return { total: sites.length, online, issues, git: gitCount };
  });

  // Bangun daftar chip runtime secara dinamis hanya untuk runtime yang benar-benar ada di server
  let runtimeChips = $derived.by(() => {
    const counts = new Map<SiteKind, number>();
    for (const s of sites) {
      counts.set(s.kind, (counts.get(s.kind) || 0) + 1);
    }
    const order: SiteKind[] = ['php', 'node', 'go', 'python', 'static', 'proxy'];
    return order
      .filter((k) => (counts.get(k) || 0) > 0)
      .map((k) => ({ kind: k, count: counts.get(k) || 0, ...RUNTIME_META[k] }));
  });

  // Reset filter runtime bila runtime yang dipilih sudah tidak memiliki situs
  $effect(() => {
    if (activeRuntime !== 'all' && !runtimeChips.some((c) => c.kind === activeRuntime)) {
      activeRuntime = 'all';
    }
  });

  // Peringkat prioritas status agar situs bermasalah langsung terlihat di atas saat diurutkan berdasarkan status
  function statusRank(s: Site): number {
    const st = s.live_status || (s.enabled === 1 ? 'running' : 'stopped');
    if (st === 'down') return 0;
    if (st === 'degraded') return 1;
    if (st === 'running') return 2;
    return 3;
  }

  // Filter & urutkan daftar situs berdasarkan kartu KPI, chip runtime, pencarian, dan sort selector
  let filteredSites = $derived.by(() => {
    let result = sites;

    if (healthFilter === 'online') {
      result = result.filter((s) => (s.live_status || (s.enabled === 1 ? 'running' : 'stopped')) === 'running');
    } else if (healthFilter === 'issues') {
      result = result.filter((s) => {
        const st = s.live_status || (s.enabled === 1 ? 'running' : 'stopped');
        return st === 'down' || st === 'degraded';
      });
    } else if (healthFilter === 'git') {
      result = result.filter((s) => Boolean(s.has_git));
    }

    if (activeRuntime !== 'all') {
      result = result.filter((s) => s.kind === activeRuntime);
    }

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      result = result.filter(
        (s) =>
          s.domain.toLowerCase().includes(q) ||
          s.aliases.toLowerCase().includes(q) ||
          (s.framework && s.framework.toLowerCase().includes(q)) ||
          (s.git_repo && s.git_repo.toLowerCase().includes(q)) ||
          (s.description && s.description.toLowerCase().includes(q)) ||
          (s.root_path && s.root_path.toLowerCase().includes(q)) ||
          (s.upstream && s.upstream.toLowerCase().includes(q)),
      );
    }

    const sorted = [...result];
    if (sortBy === 'status') {
      sorted.sort((a, b) => statusRank(a) - statusRank(b) || a.domain.localeCompare(b.domain));
    } else if (sortBy === 'domain') {
      sorted.sort((a, b) => a.domain.localeCompare(b.domain));
    } else if (sortBy === 'traffic') {
      sorted.sort((a, b) => (b.request_count ?? 0) - (a.request_count ?? 0) || a.domain.localeCompare(b.domain));
    } else {
      sorted.sort((a, b) => (b.updated_at || b.id) - (a.updated_at || a.id));
    }
    return sorted;
  });

  // Memuat daftar situs dan status Nginx dari server
  async function loadSites(silent = false) {
    if (!silent) loading = true;
    else refreshingSites = true;
    try {
      const data = await apiFetch<SitesData>('/api/sites');
      const nextSites = data.sites || [];
      sites = nextSites;
      if (selectedIds.size > 0) {
        const validIds = new Set(nextSites.map((item) => item.id));
        const pruned = new Set([...selectedIds].filter((id) => validIds.has(id)));
        if (pruned.size !== selectedIds.size) {
          selectedIds = pruned;
        }
      }
      if (data.web_server) {
        webServer = data.web_server;
      }
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat daftar situs');
    } finally {
      loading = false;
      refreshingSites = false;
    }
  }

  $effect(() => {
    if (!sites.some((s) => s.ssl_status === 'pending')) return;
    const t = setTimeout(() => loadSites(true), 3000);
    return () => clearTimeout(t);
  });

  // Periksa & perbarui SSL untuk seluruh situs yang perlu diperpanjang
  async function renewAllSSL() {
    if (renewingAllSSL) return;
    if (sites.length === 0) {
      onNotify('Belum ada situs yang terdaftar untuk diperiksa SSL-nya');
      return;
    }

    renewingAllSSL = true;
    try {
      const needRenew = sites.filter(
        (s) => s.ssl_mode !== 'cloudflare' && (s.ssl_status !== 'issued' || s.ssl_mode === 'none' || sslDaysLeft(s) <= 30),
      );

      if (needRenew.length === 0) {
        await loadSites(true);
        const minDays = Math.min(...sites.map((s) => sslDaysLeft(s)));
        onNotify(`Semua sertifikat SSL (${sites.length} situs) masih aktif & aman (${minDays} hari tersisa)`);
        return;
      }

      onNotify(`Memperbarui sertifikat SSL untuk ${needRenew.length} situs...`);
      let count = 0;
      for (const site of needRenew) {
        try {
          await apiFetch(`/api/sites/${site.id}/ssl`, {
            method: 'POST',
            body: JSON.stringify({
              enabled: true,
              force_https: site.force_https === 1,
              hsts: site.hsts === 1,
              issue: true,
              skip_dns: true,
            }),
          });
          count++;
        } catch {
          // Lanjutkan ke situs berikutnya jika satu situs gagal
        }
      }
      await loadSites(true);
      onNotify(`Pembaruan SSL berhasil dijalankan untuk ${count} dari ${needRenew.length} situs`);
    } finally {
      renewingAllSSL = false;
    }
  }

  function sslDaysLeft(s: Site) {
    return Math.max(0, Math.floor((s.ssl_expires * 1000 - Date.now()) / 86400000));
  }

  function openSettings(s: Site, tab: string = 'domain') {
    settingsTarget = { id: s.id, tab };
  }

  // Mulai ulang atau hidupkan proses aplikasi (PM2 / systemd / PHP-FPM) secara 1-klik langsung dari daftar situs
  async function restartSiteProcess(site: Site) {
    if (restartingSiteId !== null) return;
    restartingSiteId = site.id;
    try {
      const res = await apiFetch<{ ok: boolean; message?: string }>(`/api/sites/${site.id}/process/action`, {
        method: 'POST',
        body: JSON.stringify({ action: 'restart' }),
        timeoutMs: 95_000,
      });
      onNotify(res.message || `Proses aplikasi ${site.domain} berhasil dimulai ulang`);
      await loadSites(true);
    } catch (err: any) {
      onNotify(err.message || `Gagal memulai ulang aplikasi ${site.domain}`);
    } finally {
      restartingSiteId = null;
    }
  }

  async function installNginx() {
    installingNginx = true;
    try {
      await apiFetch('/api/system/software/install', {
        method: 'POST',
        body: JSON.stringify({ package: 'nginx' }),
      });
      onNotify('Perintah instalasi Nginx berhasil dikirim ke daemon');
      setTimeout(() => {
        loadSites();
        installingNginx = false;
      }, 1500);
    } catch (err: any) {
      onNotify(err.message || 'Gagal memulai instalasi Nginx');
      installingNginx = false;
    }
  }

  async function toggleSite(id: number) {
    try {
      await apiFetch(`/api/sites/${id}/toggle`, { method: 'POST' });
      await loadSites(true);
      onNotify('Status virtual host situs diperbarui');
    } catch (err: any) {
      onNotify(err.message || 'Gagal mengubah status');
    }
  }

  function getProjectRootDir(s: Site): string {
    const raw = (s.root_path || `/home/fikpanel/web/${s.domain}`).replace(/\/+$/, '');
    if (raw.endsWith('/public')) {
      return raw.slice(0, -7);
    }
    return raw;
  }

  function getPredictedDBName(s: Site): string {
    const base = s.domain
      .toLowerCase()
      .replace(/[^a-z0-9]/g, '_')
      .replace(/^_+|_+$/g, '')
      .slice(0, 12);
    return `${base || 'site'}_db`;
  }

  function openDeleteModal(site: Site) {
    deleteTarget = site;
    deleteConfirmInput = '';
    deleteFilesOpt = true;
    deleteDBOpt = true;
    backupFirstOpt = true;
    deletePasteWarning = false;
    deletingSite = false;
  }

  function closeDeleteModal() {
    if (deletingSite) return;
    deleteTarget = null;
    deleteConfirmInput = '';
    deletePasteWarning = false;
  }

  function handleBlockPaste(e: Event) {
    e.preventDefault();
    deletePasteWarning = true;
  }

  async function confirmDeleteSite() {
    if (!deleteTarget || deleteConfirmInput !== deleteTarget.domain || deletingSite) return;
    deletingSite = true;
    const targetDomain = deleteTarget.domain;
    const targetId = deleteTarget.id;
    try {
      const qs = new URLSearchParams({
        delete_files: deleteFilesOpt ? '1' : '0',
        delete_db: deleteDBOpt ? '1' : '0',
        backup_first: backupFirstOpt ? '1' : '0',
      }).toString();
      const res = await apiFetch<{
        ok: boolean;
        backup_file?: string;
        deleted_db?: string;
        deleted_files?: boolean;
      }>(`/api/sites/${targetId}?${qs}`, { method: 'DELETE' });

      deleteTarget = null;
      await loadSites();

      const summaryParts: string[] = [`Situs ${targetDomain} dihapus`];
      if (res.backup_file) summaryParts.push(`Backup: ${res.backup_file.split('/').pop()}`);
      if (res.deleted_db) summaryParts.push(`DB ${res.deleted_db} terhapus`);
      onNotify(summaryParts.join(' • '));
    } catch (err: any) {
      onNotify(err.message || 'Gagal menghapus situs');
    } finally {
      deletingSite = false;
    }
  }

  function openInFileManager(targetPath: string) {
    sessionStorage.setItem('fikpanel_open_path', targetPath);
    window.location.hash = '#/files';
    onNotify(`Membuka File Manager di direktori ${targetPath}`);
  }

  function openInTerminal(targetPath: string) {
    sessionStorage.setItem('fikpanel_term_path', targetPath);
    window.location.hash = '#/terminal';
    onNotify(`Membuka terminal pada direktori ${targetPath}`);
  }

  async function fetchSiteLogs(site: Site, type: 'access' | 'error', silent = false) {
    if (!silent) logLoading = true;
    try {
      const res = await apiFetch<{
        ok: boolean;
        content: string;
        log_path?: string;
        request_count?: number;
        request_source?: string;
      }>(`/api/sites/${site.id}/logs?type=${type}`);
      logContent = res.content || `Belum ada catatan ${type} log untuk ${site.domain}.`;
      logPath = res.log_path || '';
      if (typeof res.request_count === 'number') {
        const nextCount = res.request_count;
        const nextSource = res.request_source || site.request_source;
        if (logTarget && logTarget.id === site.id) {
          logTarget = { ...logTarget, request_count: nextCount, request_source: nextSource };
        }
        sites = sites.map((item) =>
          item.id === site.id ? { ...item, request_count: nextCount, request_source: nextSource } : item,
        );
      }
    } catch (err: any) {
      if (!silent) {
        logContent = `Gagal memuat log: ${err.message || 'Kesalahan jaringan'}`;
      }
    } finally {
      if (!silent) logLoading = false;
    }
  }

  function openLogViewer(site: Site) {
    logTarget = site;
    logTab = 'access';
    logPath = '';
    logLive = true;
    showLogModal = true;
    fetchSiteLogs(site, 'access');
  }

  function switchLogTab(tab: 'access' | 'error') {
    logTab = tab;
    if (logTarget) {
      fetchSiteLogs(logTarget, tab);
    }
  }

  $effect(() => {
    const targetId = logTarget?.id;
    const tab = logTab;
    if (!showLogModal || !targetId || !logLive) return;
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') {
        const current = sites.find((item) => item.id === targetId) || logTarget;
        if (current) {
          fetchSiteLogs(current, tab, true);
        }
      }
    }, 5000);
    return () => clearInterval(timer);
  });

  function openGitPullModal(site: Site) {
    gitPullTarget = site;
    gitPullRunning = false;
    gitPullCustomCmd = '';
    gitPullResult = null;
    gitPullError = '';
  }

  function closeGitPullModal() {
    if (gitPullRunning) return;
    gitPullTarget = null;
    gitPullResult = null;
    gitPullError = '';
  }

  async function executeGitPull(opts?: { auto_optimize?: boolean; maintenance_mode?: boolean; rebuild_assets?: boolean }) {
    if (!gitPullTarget || gitPullRunning) return;
    gitPullRunning = true;
    gitPullError = '';
    gitPullResult = null;
    try {
      const res = await apiFetch<GitPullResponse>(`/api/sites/${gitPullTarget.id}/git-pull`, {
        method: 'POST',
        body: JSON.stringify({
          command: gitPullCustomCmd.trim(),
          auto_optimize: opts?.auto_optimize,
          maintenance_mode: opts?.maintenance_mode,
          rebuild_assets: opts?.rebuild_assets,
        }),
        timeoutMs: 300_000,
      });
      gitPullResult = res;
      await loadSites(true);
      if (gitPullTarget) {
        const refreshed = sites.find((s) => s.id === gitPullTarget?.id);
        if (refreshed) gitPullTarget = refreshed;
      }
      const hcNote = res.health_summary ? ` · ${res.health_summary}` : '';
      onNotify(
        res.updated
          ? `Deploy ${res.domain} berhasil (${res.old_commit} → ${res.new_commit})${hcNote}`
          : `Deploy ${res.domain} selesai (#${res.new_commit})${hcNote}`,
      );
    } catch (err: any) {
      gitPullError = err.message || 'Gagal menjalankan Git Pull pada situs ini';
    } finally {
      gitPullRunning = false;
    }
  }

  async function triggerBackup(site: Site) {
    if (backingUpSiteId !== null || bulkBusy) return;
    backingUpSiteId = site.id;
    onNotify(`Membuat arsip cadangan untuk ${site.domain}...`);
    try {
      const res = await apiFetch<{
        ok: boolean;
        file_name: string;
        size_bytes: number;
        db_included?: boolean;
        backup_count: number;
      }>(`/api/sites/${site.id}/backup`, {
        method: 'POST',
        timeoutMs: 180_000,
      });
      await loadSites(true);
      const dbTag = res.db_included ? ' + DB' : '';
      onNotify(`Cadangan ${site.domain} tersimpan (${res.file_name} • ${formatBytes(res.size_bytes)}${dbTag})`);
    } catch (err: any) {
      onNotify(err?.message || `Gagal membuat cadangan situs ${site.domain}`);
    } finally {
      backingUpSiteId = null;
    }
  }

  function clearSelection() {
    selectedIds = new Set();
  }

  async function bulkBackupSites() {
    if (bulkBusy || selectedIds.size === 0) return;
    const targets = sites.filter((s) => selectedIds.has(s.id));
    if (targets.length === 0) return;
    bulkBusy = true;
    let okCount = 0;
    try {
      onNotify(`Memulai cadangan massal untuk ${targets.length} situs...`);
      for (const s of targets) {
        backingUpSiteId = s.id;
        try {
          await apiFetch(`/api/sites/${s.id}/backup`, { method: 'POST', timeoutMs: 180_000 });
          okCount++;
        } catch {
          // Lanjutkan ke situs berikutnya jika salah satu direktori gagal dicadangkan
        }
      }
      await loadSites(true);
      clearSelection();
      onNotify(`Cadangan massal selesai: ${okCount} dari ${targets.length} situs berhasil dicadangkan`);
    } finally {
      backingUpSiteId = null;
      bulkBusy = false;
    }
  }

  async function bulkRenewSSL() {
    if (bulkBusy || selectedIds.size === 0) return;
    const targets = sites.filter((s) => selectedIds.has(s.id) && s.ssl_mode !== 'cloudflare');
    if (targets.length === 0) {
      onNotify('Seluruh situs terpilih menggunakan Cloudflare Edge SSL');
      return;
    }
    bulkBusy = true;
    let okCount = 0;
    try {
      onNotify(`Memperbarui SSL untuk ${targets.length} situs terpilih...`);
      for (const site of targets) {
        try {
          await apiFetch(`/api/sites/${site.id}/ssl`, {
            method: 'POST',
            body: JSON.stringify({
              enabled: true,
              force_https: site.force_https === 1,
              hsts: site.hsts === 1,
              issue: true,
              skip_dns: true,
            }),
          });
          okCount++;
        } catch {
          // Lanjutkan pembaruan untuk situs terpilih lainnya
        }
      }
      await loadSites(true);
      clearSelection();
      onNotify(`Pembaruan SSL dijalankan untuk ${okCount} dari ${targets.length} situs terpilih`);
    } finally {
      bulkBusy = false;
    }
  }

  async function bulkToggleSites() {
    if (bulkBusy || selectedIds.size === 0) return;
    const targets = sites.filter((s) => selectedIds.has(s.id));
    if (targets.length === 0) return;
    bulkBusy = true;
    let okCount = 0;
    try {
      for (const site of targets) {
        try {
          await apiFetch(`/api/sites/${site.id}/toggle`, { method: 'POST' });
          okCount++;
        } catch {
          // Lanjutkan ke situs berikutnya jika toggle gagal
        }
      }
      await loadSites(true);
      clearSelection();
      onNotify(`Status ${okCount} dari ${targets.length} situs berhasil diubah`);
    } finally {
      bulkBusy = false;
    }
  }

  function toggleSelectAll(e: Event) {
    const checked = (e.target as HTMLInputElement).checked;
    if (checked) {
      selectedIds = new Set(filteredSites.map((s) => s.id));
    } else {
      selectedIds = new Set();
    }
  }

  function toggleSelectSite(id: number) {
    const next = new Set(selectedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    selectedIds = next;
  }

  onMount(() => {
    loadSites();

    // Segarkan otomatis status live tiap 12 detik saat tab aktif dan tidak sedang sibuk
    const pollTimer = setInterval(() => {
      if (
        document.visibilityState === 'visible' &&
        !loading &&
        !refreshingSites &&
        backingUpSiteId === null &&
        restartingSiteId === null &&
        !bulkBusy &&
        !deletingSite &&
        !gitPullRunning &&
        !showAddModal
      ) {
        loadSites(true);
      }
    }, 12_000);

    const handleVisibility = () => {
      if (document.visibilityState === 'visible' && !loading && !refreshingSites && !bulkBusy && backingUpSiteId === null) {
        loadSites(true);
      }
    };
    document.addEventListener('visibilitychange', handleVisibility);

    return () => {
      clearInterval(pollTimer);
      document.removeEventListener('visibilitychange', handleVisibility);
    };
  });
</script>

<svelte:window onclick={() => { if (openMoreSiteId !== null) openMoreSiteId = null; }} />

<div class="pg-sites sites-container">
  <!-- ═════════ 1. Header Halaman + Status Mesin Nginx + Aksi Utama ═════════ -->
  <PageHeader
    title="Website & Aplikasi"
    subtitle="Deploy multi-runtime (PHP/Filament, Node.js, Go, Python, SPA), Auto-SSL, .env, & manajemen proses 1-klik"
    icon="ri-global-line"
  >
    {#snippet actions()}
      {#if webServer.installed}
        <button
          id="nginx-badge-btn"
          type="button"
          class="server-badge clickable {webServer.running ? 'is-running' : 'is-stopped'}"
          title="Klik untuk membuka Nginx Manager"
          onclick={() => (showNginxManager = true)}
        >
          <span class="pulse-indicator"></span>
          <i class="ri-server-line" aria-hidden="true"></i>
          <span class="server-label">Nginx {webServer.version || '1.24.0'}</span>
          <span class="server-status-tag">{webServer.running ? 'Running' : 'Stopped'}</span>
          <i class="ri-settings-4-line badge-gear" aria-hidden="true"></i>
        </button>
      {:else}
        <div class="server-badge is-missing" title="Nginx belum terpasang">
          <i class="ri-alert-fill" aria-hidden="true"></i>
          <span>Nginx Belum Terpasang</span>
        </div>
      {/if}

      <button
        type="button"
        class="btn-renew-ssl"
        title="Periksa masa aktif & perbarui semua sertifikat SSL"
        disabled={renewingAllSSL}
        onclick={renewAllSSL}
      >
        <i class={renewingAllSSL ? 'ri-loader-4-line spin' : 'ri-shield-keyhole-line'} aria-hidden="true"></i>
        <span>{renewingAllSSL ? 'Memeriksa…' : 'Perbarui SSL'}</span>
      </button>

      <button
        type="button"
        class="btn-refresh-sites"
        title="Segarkan status live upstream & proses"
        disabled={loading || refreshingSites}
        onclick={() => loadSites(true)}
      >
        <i class={refreshingSites ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
      </button>

      <button type="button" class="btn-add-site" id="add-site-btn" onclick={() => (showAddModal = true)}>
        <i class="ri-rocket-2-line" aria-hidden="true"></i>
        <span>Deploy / Tambah Situs</span>
      </button>
    {/snippet}
  </PageHeader>

  <!-- ═════════ 2. 4 Kartu Filter Kesehatan Interaktif ═════════ -->
  {#if webServer.installed}
    <div class="sites-kpi-grid" role="group" aria-label="Filter cepat kesehatan situs">
      <button
        type="button"
        class="sites-kpi-card"
        class:is-active={healthFilter === 'all'}
        onclick={() => (healthFilter = 'all')}
      >
        <div class="kpi-top">
          <span class="kpi-label">Semua Situs</span>
          <i class="ri-apps-2-line kpi-icon" aria-hidden="true"></i>
        </div>
        <strong class="kpi-value mono">{kpiCounts.total}</strong>
        <small class="kpi-sub">{runtimeChips.length} runtime aktif di server</small>
      </button>

      <button
        type="button"
        class="sites-kpi-card tone-success"
        class:is-active={healthFilter === 'online'}
        onclick={() => (healthFilter = healthFilter === 'online' ? 'all' : 'online')}
      >
        <div class="kpi-top">
          <span class="kpi-label">Online &amp; Sehat</span>
          <i class="ri-checkbox-circle-line kpi-icon" aria-hidden="true"></i>
        </div>
        <strong class="kpi-value mono">{kpiCounts.online}</strong>
        <small class="kpi-sub">Upstream &amp; vhost merespons normal</small>
      </button>

      <button
        type="button"
        class="sites-kpi-card tone-danger"
        class:is-active={healthFilter === 'issues'}
        onclick={() => (healthFilter = healthFilter === 'issues' ? 'all' : 'issues')}
      >
        <div class="kpi-top">
          <span class="kpi-label">Bermasalah / Down</span>
          <i class="ri-error-warning-line kpi-icon" aria-hidden="true"></i>
        </div>
        <strong class="kpi-value mono">{kpiCounts.issues}</strong>
        <small class="kpi-sub">{kpiCounts.issues > 0 ? 'Klik untuk inspeksi & restart 1-klik' : 'Semua layanan berjalan lancar'}</small>
      </button>

      <button
        type="button"
        class="sites-kpi-card tone-accent"
        class:is-active={healthFilter === 'git'}
        onclick={() => (healthFilter = healthFilter === 'git' ? 'all' : 'git')}
      >
        <div class="kpi-top">
          <span class="kpi-label">Terhubung Git</span>
          <i class="ri-git-branch-line kpi-icon" aria-hidden="true"></i>
        </div>
        <strong class="kpi-value mono">{kpiCounts.git}</strong>
        <small class="kpi-sub">Auto-deploy, webhook &amp; rollback aktif</small>
      </button>
    </div>

    <!-- ═════════ 3. Bar Filter Runtime Dinamis + Pencarian + Urutan + Toggle View ═════════ -->
    <div class="sites-filter-bar">
      <div class="runtime-chips-wrap" role="tablist" aria-label="Filter runtime dinamis">
        <button
          type="button"
          role="tab"
          aria-selected={activeRuntime === 'all'}
          class="rt-chip"
          class:is-active={activeRuntime === 'all'}
          onclick={() => (activeRuntime = 'all')}
        >
          <span>Semua</span>
          <span class="rt-count mono">{sites.length}</span>
        </button>

        {#each runtimeChips as chip (chip.kind)}
          <button
            type="button"
            role="tab"
            aria-selected={activeRuntime === chip.kind}
            class="rt-chip"
            class:is-active={activeRuntime === chip.kind}
            onclick={() => (activeRuntime = activeRuntime === chip.kind ? 'all' : chip.kind)}
          >
            <i class="{chip.icon} {chip.cls}" aria-hidden="true"></i>
            <span>{chip.label}</span>
            <span class="rt-count mono">{chip.count}</span>
          </button>
        {/each}
      </div>

      <div class="filter-controls-right">
        <div class="search-box">
          <i class="ri-search-line search-icon" aria-hidden="true"></i>
          <input
            type="text"
            aria-label="Cari domain, framework, repo, atau port"
            placeholder="Cari domain, framework, port..."
            bind:value={searchQuery}
          />
          {#if searchQuery}
            <button
              type="button"
              class="btn-clear-search"
              aria-label="Hapus pencarian"
              title="Hapus pencarian"
              onclick={() => (searchQuery = '')}
            >&times;</button>
          {/if}
        </div>

        <select class="sort-select" aria-label="Urutkan situs" bind:value={sortBy}>
          <option value="status">Prioritas Status</option>
          <option value="domain">Domain (A–Z)</option>
          <option value="traffic">Trafik Tertinggi</option>
          <option value="recent">Terbaru</option>
        </select>

        <div class="view-toggle" role="group" aria-label="Mode tampilan">
          <button
            type="button"
            class="view-btn"
            class:is-active={viewMode === 'table'}
            aria-label="Tampilan tabel"
            title="Tampilan Tabel"
            onclick={() => (viewMode = 'table')}
          >
            <i class="ri-list-check-2" aria-hidden="true"></i>
          </button>
          <button
            type="button"
            class="view-btn"
            class:is-active={viewMode === 'grid'}
            aria-label="Tampilan kartu grid"
            title="Tampilan Kartu"
            onclick={() => (viewMode = 'grid')}
          >
            <i class="ri-layout-grid-line" aria-hidden="true"></i>
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ═════════ 4. Bar Aksi Massal (muncul saat >= 1 situs dicentang) ═════════ -->
  {#if selectedIds.size > 0 && webServer.installed}
    <div class="bulk-action-bar">
      <div class="bulk-action-info">
        <i class="ri-checkbox-multiple-line" aria-hidden="true"></i>
        <span><strong>{selectedIds.size}</strong> situs dipilih</span>
      </div>
      <div class="bulk-action-btns">
        <button type="button" class="btn-bulk" disabled={bulkBusy} onclick={bulkBackupSites}>
          <i class={bulkBusy ? 'ri-loader-4-line spin' : 'ri-archive-line'} aria-hidden="true"></i>
          <span>Backup Terpilih</span>
        </button>
        <button type="button" class="btn-bulk" disabled={bulkBusy} onclick={bulkRenewSSL}>
          <i class="ri-shield-keyhole-line" aria-hidden="true"></i>
          <span>Perbarui SSL</span>
        </button>
        <button type="button" class="btn-bulk" disabled={bulkBusy} onclick={bulkToggleSites}>
          <i class="ri-shut-down-line" aria-hidden="true"></i>
          <span>Toggle Vhost</span>
        </button>
        <button type="button" class="btn-bulk btn-bulk--ghost" disabled={bulkBusy} onclick={clearSelection}>
          <i class="ri-close-line" aria-hidden="true"></i>
          <span>Batal</span>
        </button>
      </div>
    </div>
  {/if}

  <!-- ═════════ 5. Tampilan Utama: Warning Nginx vs Tabel/Grid Situs ═════════ -->
  {#if !webServer.installed}
    <div class="dep-warning-card">
      <div class="dep-warn-glow"></div>
      <div class="dep-warn-content">
        <div class="dep-warn-icon-box">
          <i class="ri-error-warning-fill" aria-hidden="true"></i>
        </div>
        <div class="dep-warn-details">
          <h3>Nginx Web Server Belum Terpasang di Server!</h3>
          <p>
            FikPanel memerlukan Nginx untuk mengonfigurasi reverse proxy, merutekan port aplikasi (Node, Go, Python), mengelola berkas web statis, dan menerbitkan sertifikat SSL otomatis.
          </p>
          <div class="dep-warn-actions">
            <button
              type="button"
              class="btn btn-install-now"
              onclick={installNginx}
              disabled={installingNginx}
            >
              {#if installingNginx}
                <i class="ri-loader-4-line spin" aria-hidden="true"></i>
                <span>Memulai Pemasangan Nginx...</span>
              {:else}
                <i class="ri-download-cloud-2-line" aria-hidden="true"></i>
                <span>Pasang Nginx Sekarang (1-Klik)</span>
              {/if}
            </button>
            <button
              type="button"
              class="btn btn--ghost"
              onclick={() => (window.location.hash = '#/terminal')}
            >
              <i class="ri-terminal-box-line" aria-hidden="true"></i>
              <span>Pasang via Terminal</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  {:else}
    <SitesTable
      {loading}
      {filteredSites}
      {selectedIds}
      {backingUpSiteId}
      {restartingSiteId}
      {viewMode}
      bind:openMoreSiteId
      onToggleSelectAll={toggleSelectAll}
      onToggleSelectSite={toggleSelectSite}
      onToggleSite={toggleSite}
      onRestartProcess={restartSiteProcess}
      onTriggerBackup={triggerBackup}
      onOpenInFileManager={openInFileManager}
      onOpenInTerminal={openInTerminal}
      onOpenSettings={openSettings}
      onOpenGitPullModal={openGitPullModal}
      onOpenLogViewer={openLogViewer}
      onOpenDeleteModal={openDeleteModal}
      onToggleMoreMenu={toggleMoreMenu}
      onShowAddModal={() => (showAddModal = true)}
      {getProjectRootDir}
      {sslDaysLeft}
    />
  {/if}

  <!-- ═════════ MODAL: Tambah / Deploy Situs ═════════ -->
  {#if showAddModal}
    <AddSiteModal
      onClose={() => (showAddModal = false)}
      onCreated={() => loadSites(true)}
      onOpenSSL={(s) => { showAddModal = false; openSettings(s, 'ssl'); }}
      {onNotify}
    />
  {/if}

  <!-- ═════════ MODAL: Pengaturan Situs ═════════ -->
  {#if settingsTarget}
    {#key settingsTarget.id}
      <SiteSettings
        siteId={settingsTarget.id}
        initialTab={settingsTarget.tab}
        onClose={() => (settingsTarget = null)}
        onChanged={() => loadSites(true)}
        {onNotify}
      />
    {/key}
  {/if}

  <!-- ═════════ MODAL: Penampil Log Akses & Error ═════════ -->
  <LogsModal
    open={showLogModal && !!logTarget}
    {logTarget}
    {logTab}
    {logLoading}
    {logContent}
    {logPath}
    {logLive}
    onClose={() => (showLogModal = false)}
    onSwitchLogTab={switchLogTab}
    onRefreshLogs={fetchSiteLogs}
    onToggleLive={() => (logLive = !logLive)}
  />

  <!-- ═════════ MODAL: Git Pull & Auto-Deploy ═════════ -->
  <GitPullModal
    open={!!gitPullTarget}
    {gitPullTarget}
    bind:gitPullCustomCmd
    {gitPullRunning}
    {gitPullError}
    {gitPullResult}
    {gitPullSnippets}
    {hasGitSnippet}
    onClose={closeGitPullModal}
    onResetGitPullCmd={resetGitPullCmd}
    onToggleGitSnippet={toggleGitSnippet}
    onExecuteGitPull={executeGitPull}
    {onNotify}
    onSiteUpdated={() => loadSites(true)}
    {getProjectRootDir}
  />

  <!-- ═════════ MODAL: Konfirmasi Hapus Situs (Danger Zone) ═════════ -->
  <DeleteModal
    open={!!deleteTarget}
    {deleteTarget}
    bind:backupFirstOpt
    bind:deleteFilesOpt
    bind:deleteDBOpt
    bind:deleteConfirmInput
    {deletingSite}
    {deletePasteWarning}
    onClose={closeDeleteModal}
    onConfirmDelete={confirmDeleteSite}
    onBlockPaste={handleBlockPaste}
    {getProjectRootDir}
    {getPredictedDBName}
  />

  <!-- ═════════ MODAL: Nginx Manager ═════════ -->
  {#if showNginxManager}
    <NginxManager
      onClose={() => (showNginxManager = false)}
      {onNotify}
      onStatusChange={onNginxStatus}
    />
  {/if}
</div>
