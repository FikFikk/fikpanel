<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch, formatBytes } from '../lib/api';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import { notify } from '../lib/stores/toast.svelte';
  import PageHeader from '../lib/components/ui/PageHeader.svelte';
  import type {
    AutoBackupStatus,
    DatabaseEngineInfo,
    DatabaseItem,
    DatabaseProcessItem,
    DatabaseTelemetry,
    SystemTask,
  } from '../lib/types';
  import Backup from './Backup.svelte';
  import EngineTabs from './databases/EngineTabs.svelte';
  import UninstalledCard from './databases/UninstalledCard.svelte';
  import ActionBar from './databases/ActionBar.svelte';
  import DbTable from './databases/DbTable.svelte';
  import RedisConsole from './databases/RedisConsole.svelte';
  import SqlStudioModal from './databases/SqlStudioModal.svelte';
  import DbBackupImportModal from './databases/DbBackupImportModal.svelte';
  import QuickInstallModal from './databases/QuickInstallModal.svelte';
  import PhpMyAdminInstallModal from './databases/PhpMyAdminInstallModal.svelte';
  import AddDatabaseModal from './databases/AddDatabaseModal.svelte';
  import EngineManagerModal from './databases/EngineManagerModal.svelte';
  import WebGuiModal from './databases/WebGuiModal.svelte';
  import QuotaModal from './databases/QuotaModal.svelte';
  import RootPasswordModal from './databases/RootPasswordModal.svelte';
  import UserPasswordModal from './databases/UserPasswordModal.svelte';
  import TasksDrawer from './databases/TasksDrawer.svelte';
  import '../styles/pages/databases/shared.css';

  // State tampilan utama & engine aktif
  let mainView = $state<'database' | 'backup'>('database');
  let engines = $state<DatabaseEngineInfo[]>([]);
  let activeEngine = $state<'mysql' | 'postgresql' | 'mongodb' | 'redis'>('mysql');
  let databases = $state<DatabaseItem[]>([]);
  let telemetry = $state<DatabaseTelemetry | null>(null);
  let loadingEngines = $state(true);
  let loadingDatabases = $state(false);
  let syncingHost = $state(false);
  let searchQuery = $state('');

  // State Auto-Backup Cron per engine
  let autoBackupStatus = $state<AutoBackupStatus>({
    enabled: false,
    schedule: '0 2 * * *',
    keep_days: 7,
    cron_job_id: 0,
    last_status: '',
    last_run_at: 0,
  });
  let togglingAutoBackup = $state(false);

  // State seleksi massal (Bulk Actions)
  let selectedNames = $state<string[]>([]);
  let bulkWorking = $state(false);

  // Peta visibilitas password per ID database
  let visiblePasswords = $state<Record<number, boolean>>({});

  // State modal
  let showInstallModal = $state(false);
  let showPmaInstallModal = $state(false);
  let showAddModal = $state(false);
  let showRootPassModal = $state(false);
  let showUserPassModal = $state(false);
  let selectedUserPassDb = $state<DatabaseItem | null>(null);
  let userPassValue = $state('');
  let savingUserPass = $state(false);
  let showTasksDrawer = $state(false);
  let showEngineManager = $state(false);
  let showWebGuiModal = $state(false);
  let showQuotaModal = $state(false);

  // State modal SQL Studio & Backup/Impor
  let showSqlStudio = $state(false);
  let sqlStudioDb = $state<DatabaseItem | null>(null);
  let showBackupImportModal = $state(false);
  let backupImportDb = $state<DatabaseItem | null>(null);

  // State Engine Manager (Service, Processlist, RAM Tuner, Config, Logs, Switch Version)
  let engineTab = $state<'service' | 'processlist' | 'switch_version' | 'config' | 'logs'>('service');
  let switchTargetVersion = $state('');
  let switchBackupFirst = $state(true);
  let switchingVersion = $state(false);
  let serviceActionLoading = $state(false);
  let tuningLoading = $state(false);
  let engineConfigContent = $state('');
  let engineConfigPath = $state('');
  let engineLogsContent = $state('');
  let engineLogsSource = $state('');
  let loadingEngineConfig = $state(false);
  let savingEngineConfig = $state(false);
  let loadingEngineLogs = $state(false);
  let processes = $state<DatabaseProcessItem[]>([]);
  let loadingProcesses = $state(false);

  // State Web GUI (phpMyAdmin)
  let publicAccessEnabled = $state(false);
  let targetWebGuiDb = $state('');
  let targetWebGuiUser = $state('');
  let targetWebGuiPass = $state('');
  let showWebGuiPassword = $state(false);
  let webGuiStatus = $state<{
    installed: boolean;
    software: string;
    port: number;
    panel_host?: string;
    php_version: string;
    public_access: boolean;
    available_domains?: Array<{ domain: string; ssl: boolean; url: string }>;
    has_domains?: boolean;
  } | null>(null);
  let webGuiAccessMode = $state<'panel' | 'domain' | 'custom_port'>('panel');
  let selectedWebGuiDomain = $state('');
  let customWebGuiPort = $state(888);

  // State Kuota Database
  let quotaTargetDb = $state<DatabaseItem | null>(null);
  let quotaMB = $state(0);
  let savingQuota = $state(false);

  // State Quick Install Engine
  let installVersion = $state('7.0');
  let installMode = $state<'native' | 'docker'>('native');
  let installing = $state(false);

  // State Quick Install phpMyAdmin
  let installPmaVersion = $state('5.2.1');
  let installPmaPhp = $state('8.2');
  let installPmaMode = $state<'native' | 'docker'>('native');
  let installingPma = $state(false);

  // State Form Tambah Database
  let dbName = $state('');
  let dbCharset = $state('utf8mb4');
  let dbUser = $state('');
  let dbPass = $state('');
  let dbPermission = $state('localhost');
  let customIp = $state('');
  let dbNote = $state('');
  let creatingDb = $state(false);
  let userManuallyEdited = $state(false);

  // State Form Password Root (kosong secara default demi keamanan)
  let rootPassword = $state('');
  let showRootPass = $state(false);
  let savingRootPass = $state(false);

  // State Antrean Tugas & Log Streaming SSE
  let tasks = $state<SystemTask[]>([]);
  let activeTask = $state<SystemTask | null>(null);
  let streamLogs = $state<string[]>([]);
  let eventSource: EventSource | null = null;
  let terminalEl = $state<HTMLDivElement | null>(null);

  function getEngineIcon(eng: string): string {
    switch (eng) {
      case 'mysql': return 'ri-database-2-line';
      case 'postgresql': return 'ri-server-line';
      case 'mongodb': return 'ri-leaf-line';
      case 'redis': return 'ri-flashlight-line';
      default: return 'ri-database-2-line';
    }
  }

  function getDefaultEngine(eng: 'mysql' | 'postgresql' | 'mongodb' | 'redis'): DatabaseEngineInfo {
    switch (eng) {
      case 'mysql':
        return {
          name: 'mysql',
          display_name: 'MySQL / MariaDB',
          installed: true,
          version: '10.11.6-MariaDB',
          running: true,
          port: 3306,
          databases_count: 0,
          service_unit: 'mariadb.service',
          supported_versions: ['10.11', '11.4', '8.0', '8.4'],
          versions: [
            { version: '10.11', label: 'MariaDB 10.11 LTS (Rekomendasi Hosting)', is_recommended: true },
            { version: '11.4', label: 'MariaDB 11.4 LTS (Fitur Terbaru)', is_recommended: false },
            { version: '8.0', label: 'MySQL 8.0 LTS (Oracle Standar)', is_recommended: false },
            { version: '8.4', label: 'MySQL 8.4 LTS (Enterprise)', is_recommended: false },
          ],
          description: 'Relational database standar untuk aplikasi web PHP, Laravel, WordPress, dan CMS.',
        };
      case 'postgresql':
        return {
          name: 'postgresql',
          display_name: 'PostgreSQL',
          installed: false,
          version: '',
          running: false,
          port: 5432,
          databases_count: 0,
          service_unit: 'postgresql.service',
          supported_versions: ['16', '15', '17', '14'],
          versions: [
            { version: '16', label: 'PostgreSQL 16 (Rekomendasi Stabil)', is_recommended: true },
            { version: '15', label: 'PostgreSQL 15 LTS', is_recommended: false },
            { version: '17', label: 'PostgreSQL 17 (Fitur Terbaru)', is_recommended: false },
            { version: '14', label: 'PostgreSQL 14 LTS', is_recommended: false },
          ],
          description: 'Database relasional modern berorientasi objek dengan performa query analitik tinggi.',
        };
      case 'mongodb':
        return {
          name: 'mongodb',
          display_name: 'MongoDB',
          installed: false,
          version: '',
          running: false,
          port: 27017,
          databases_count: 0,
          service_unit: 'mongod.service',
          supported_versions: ['7.0', '6.0', '8.0', '5.0'],
          versions: [
            { version: '7.0', label: 'MongoDB 7.0 LTS (Rekomendasi Stabil)', is_recommended: true },
            { version: '6.0', label: 'MongoDB 6.0 LTS (Kompatibilitas Luas)', is_recommended: false },
            { version: '8.0', label: 'MongoDB 8.0 (Modern Engine)', is_recommended: false },
            { version: '5.0', label: 'MongoDB 5.0 (Legacy)', is_recommended: false },
          ],
          description: 'Database dokumen NoSQL terdistribusi untuk struktur data JSON / BSON fleksibel.',
        };
      case 'redis':
        return {
          name: 'redis',
          display_name: 'Redis',
          installed: false,
          version: '',
          running: false,
          port: 6379,
          databases_count: 0,
          service_unit: 'redis-server.service',
          supported_versions: ['7.2', '7.0', '6.2'],
          versions: [
            { version: '7.2', label: 'Redis 7.2 (Rekomendasi Performa)', is_recommended: true },
            { version: '7.0', label: 'Redis 7.0 LTS', is_recommended: false },
            { version: '6.2', label: 'Redis 6.2 LTS (Stabil)', is_recommended: false },
          ],
          description: 'In-memory data structure store untuk caching berkecepatan ultra-tinggi, queue, dan sesi.',
        };
    }
  }

  let currentEngine = $derived(
    engines.find((e) => e.name === activeEngine) || getDefaultEngine(activeEngine)
  );

  let filteredDatabases = $derived(
    databases.filter(
      (d) =>
        d.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        d.username.toLowerCase().includes(searchQuery.toLowerCase()) ||
        (d.note || '').toLowerCase().includes(searchQuery.toLowerCase())
    )
  );

  let totalDbBytes = $derived(
    databases.reduce((acc, item) => acc + (item.size_bytes || 0), 0)
  );

  let totalBackupsCount = $derived(
    databases.reduce((acc, item) => acc + (item.backup_count || 0), 0)
  );

  onMount(async () => {
    if (window.location.hash.includes('backup')) {
      mainView = 'backup';
    }
    await fetchEngines();
    await loadEngineData(activeEngine);
    await fetchTasks();
  });

  onDestroy(() => {
    closeLogStream();
  });

  // Muat seluruh data untuk satu engine (daftar DB, telemetri, status auto-backup) secara paralel
  async function loadEngineData(eng: 'mysql' | 'postgresql' | 'mongodb' | 'redis') {
    selectedNames = [];
    const jobs: Promise<any>[] = [fetchTelemetry(eng)];
    if (eng !== 'redis') {
      jobs.push(fetchDatabases(eng), fetchAutoBackupStatus(eng));
    }
    await Promise.all(jobs);
  }

  async function fetchEngines() {
    loadingEngines = true;
    try {
      const data = await apiFetch<{ engines: DatabaseEngineInfo[] }>('/api/databases/engines');
      engines = data.engines || [];
      const curr = engines.find((e) => e.name === activeEngine);
      if (curr && curr.supported_versions?.length > 0) {
        installVersion = curr.supported_versions[0];
      }
    } catch {
      notify('Gagal memuat status mesin database', 'error');
    } finally {
      loadingEngines = false;
    }
  }

  async function fetchDatabases(eng: string) {
    loadingDatabases = true;
    try {
      const data = await apiFetch<{ databases: DatabaseItem[] }>(`/api/databases/${eng}/list`);
      databases = data.databases || [];
    } catch {
      databases = [];
    } finally {
      loadingDatabases = false;
    }
  }

  async function fetchTelemetry(eng: string) {
    try {
      const data = await apiFetch<{ ok: boolean; telemetry: DatabaseTelemetry }>(`/api/databases/${eng}/telemetry`);
      telemetry = data.telemetry || null;
    } catch {
      telemetry = null;
    }
  }

  async function fetchAutoBackupStatus(eng: string) {
    try {
      const data = await apiFetch<AutoBackupStatus & { ok: boolean }>(`/api/databases/${eng}/auto-backup`);
      autoBackupStatus = {
        enabled: !!data.enabled,
        schedule: data.schedule || '0 2 * * *',
        keep_days: data.keep_days || 7,
        cron_job_id: data.cron_job_id || 0,
        last_status: data.last_status || '',
        last_run_at: data.last_run_at || 0,
      };
    } catch {
      autoBackupStatus = { enabled: false, schedule: '0 2 * * *', keep_days: 7, cron_job_id: 0, last_status: '', last_run_at: 0 };
    }
  }

  // Toggle Auto-Backup nyata yang terhubung ke Cron Manager FikPanel
  async function toggleAutoBackup(enabled: boolean) {
    togglingAutoBackup = true;
    try {
      const res = await apiFetch<AutoBackupStatus & { ok: boolean; message: string }>(
        `/api/databases/${activeEngine}/auto-backup`,
        {
          method: 'POST',
          body: JSON.stringify({
            enabled,
            schedule: autoBackupStatus.schedule || '0 2 * * *',
            keep_days: autoBackupStatus.keep_days || 7,
          }),
        }
      );
      autoBackupStatus = {
        ...autoBackupStatus,
        enabled: res.enabled,
        schedule: res.schedule,
        keep_days: res.keep_days,
        cron_job_id: res.cron_job_id,
      };
      notify(res.message || 'Pengaturan auto-backup diperbarui', 'success');
    } catch (e: any) {
      notify(e.message || 'Gagal memperbarui jadwal auto-backup', 'error');
    } finally {
      togglingAutoBackup = false;
    }
  }

  // Sinkronisasi database nyata dari server host ke panel
  async function syncFromHost() {
    syncingHost = true;
    try {
      const res = await apiFetch<{ ok: boolean; discovered: number; updated: number; message: string }>(
        `/api/databases/${activeEngine}/sync`,
        { method: 'POST' }
      );
      notify(res.message || 'Sinkronisasi host selesai', 'success');
      await fetchEngines();
      await loadEngineData(activeEngine);
    } catch (e: any) {
      notify(e.message || 'Gagal menyinkronkan database dari host', 'error');
    } finally {
      syncingHost = false;
    }
  }

  async function switchEngine(eng: 'mysql' | 'postgresql' | 'mongodb' | 'redis') {
    mainView = 'database';
    activeEngine = eng;
    searchQuery = '';
    const curr = engines.find((e) => e.name === eng) || getDefaultEngine(eng);
    if (curr.versions && curr.versions.length > 0) {
      const rec = curr.versions.find((v) => v.is_recommended);
      installVersion = rec ? rec.version : curr.versions[0].version;
    } else if (curr.supported_versions?.length > 0) {
      installVersion = curr.supported_versions[0];
    }
    await loadEngineData(eng);
  }

  // Salin blok .env & URL koneksi 1-klik untuk Laravel / Node / Go
  function copyConnectionEnv(db: DatabaseItem) {
    const pass = db.password || '<PASSWORD_BELUM_DISETEL>';
    const host = db.location === '%' || !db.location ? '127.0.0.1' : db.location;
    let snippet = '';
    if (activeEngine === 'postgresql') {
      snippet = [
        `DB_CONNECTION=pgsql`,
        `DB_HOST=${host}`,
        `DB_PORT=5432`,
        `DB_DATABASE=${db.name}`,
        `DB_USERNAME=${db.username}`,
        `DB_PASSWORD=${pass}`,
        `# DATABASE_URL=postgresql://${db.username}:${encodeURIComponent(pass)}@${host}:5432/${db.name}`,
      ].join('\n');
    } else {
      snippet = [
        `DB_CONNECTION=mysql`,
        `DB_HOST=${host}`,
        `DB_PORT=3306`,
        `DB_DATABASE=${db.name}`,
        `DB_USERNAME=${db.username}`,
        `DB_PASSWORD=${pass}`,
        `# DATABASE_URL=mysql://${db.username}:${encodeURIComponent(pass)}@${host}:3306/${db.name}`,
      ].join('\n');
    }
    copyText(snippet, `Konfigurasi .env (${db.name})`);
  }

  // Aksi seleksi massal (Bulk Actions)
  function toggleSelect(name: string) {
    if (selectedNames.includes(name)) {
      selectedNames = selectedNames.filter((n) => n !== name);
    } else {
      selectedNames = [...selectedNames, name];
    }
  }

  function toggleSelectAll() {
    if (filteredDatabases.length > 0 && filteredDatabases.every((d) => selectedNames.includes(d.name))) {
      selectedNames = [];
    } else {
      selectedNames = filteredDatabases.map((d) => d.name);
    }
  }

  async function bulkBackupSelected() {
    if (selectedNames.length === 0) return;
    bulkWorking = true;
    let okCount = 0;
    for (const name of selectedNames) {
      try {
        await apiFetch(`/api/databases/${activeEngine}/${encodeURIComponent(name)}/dump`, {
          method: 'POST',
          timeoutMs: 180_000,
        });
        okCount++;
      } catch {
        // Lanjutkan ke item berikutnya
      }
    }
    bulkWorking = false;
    notify(`Backup massal selesai: ${okCount}/${selectedNames.length} database berhasil dicadangkan`, 'success');
    selectedNames = [];
    await fetchDatabases(activeEngine);
  }

  async function bulkDeleteSelected() {
    const deletable = databases.filter((d) => selectedNames.includes(d.name) && !d.is_system);
    if (deletable.length === 0) {
      notify('Tidak ada database yang dapat dihapus (database sistem dilindungi)', 'warning');
      return;
    }
    const ok = await confirmDialog({
      title: `Hapus ${deletable.length} Database Terpilih`,
      message: `Database berikut akan dihapus permanen beserta akun penggunanya: ${deletable.map((d) => d.name).join(', ')}. Lanjutkan?`,
      confirmText: `Hapus ${deletable.length} Database`,
      tone: 'danger',
    });
    if (!ok) return;

    bulkWorking = true;
    let okCount = 0;
    for (const d of deletable) {
      try {
        await apiFetch(`/api/databases/${activeEngine}/${encodeURIComponent(d.name)}`, { method: 'DELETE' });
        okCount++;
      } catch {
        // Lanjutkan ke item berikutnya
      }
    }
    bulkWorking = false;
    notify(`${okCount} database berhasil dihapus`, 'success');
    selectedNames = [];
    await fetchEngines();
    await fetchDatabases(activeEngine);
  }

  function openInstallModal() {
    const curr = currentEngine;
    if (curr.versions && curr.versions.length > 0) {
      const rec = curr.versions.find((v) => v.is_recommended);
      installVersion = rec ? rec.version : curr.versions[0].version;
    } else if (curr.supported_versions?.length > 0) {
      installVersion = curr.supported_versions[0];
    }
    showInstallModal = true;
  }

  async function submitInstall() {
    installing = true;
    try {
      const data = await apiFetch<{ ok: boolean; task_id: string; message: string }>('/api/databases/install', {
        method: 'POST',
        body: JSON.stringify({
          engine: activeEngine,
          version: installVersion,
          mode: installMode,
        }),
      });

      showInstallModal = false;
      notify(data.message || 'Instalasi dimulai', 'success');
      await fetchTasks();
      openTaskStream(data.task_id);
    } catch (e: any) {
      notify(e.message || 'Terjadi kesalahan saat memulai instalasi', 'error');
    } finally {
      installing = false;
    }
  }

  function openPmaInstallModal() {
    showWebGuiModal = false;
    showPmaInstallModal = true;
  }

  async function submitPmaInstall() {
    installingPma = true;
    try {
      const data = await apiFetch<{ ok: boolean; task_id: string; message: string }>('/api/databases/install', {
        method: 'POST',
        body: JSON.stringify({
          engine: 'phpmyadmin',
          version: installPmaVersion,
          mode: installPmaMode,
          php_version: installPmaPhp,
        }),
      });

      showPmaInstallModal = false;
      notify(data.message || 'Instalasi resmi phpMyAdmin dimulai', 'success');
      await fetchTasks();
      openTaskStream(data.task_id);
    } catch (e: any) {
      notify(e.message || 'Terjadi kesalahan saat memulai instalasi phpMyAdmin', 'error');
    } finally {
      installingPma = false;
    }
  }

  async function fetchTasks() {
    try {
      const data = await apiFetch<{ tasks: SystemTask[] }>('/api/system/tasks');
      tasks = data.tasks || [];
    } catch {
      // Abaikan bila gagal memuat riwayat tugas
    }
  }

  function openTaskStream(taskId: string) {
    closeLogStream();
    showTasksDrawer = true;
    streamLogs = [];

    const target = tasks.find((t) => t.id === taskId);
    activeTask = target || {
      id: taskId,
      type: 'software_install',
      target: `${activeEngine}-${installVersion}`,
      status: 'running',
      log: '',
      created_at: Math.floor(Date.now() / 1000),
      updated_at: Math.floor(Date.now() / 1000),
    };

    eventSource = new EventSource(`/api/system/tasks/${taskId}/stream`);
    eventSource.onmessage = (event) => {
      if (event.data) {
        streamLogs = [...streamLogs, event.data];
        scrollTerminal();
      }
    };

    eventSource.addEventListener('done', async (ev: MessageEvent) => {
      closeLogStream();
      let finalStatus: 'success' | 'failed' = 'success';
      try {
        const parsed = JSON.parse(ev.data || '{}');
        if (parsed.status === 'failed') finalStatus = 'failed';
      } catch {
        // Default sukses
      }
      if (activeTask) {
        activeTask = { ...activeTask, status: finalStatus };
      }
      notify(
        finalStatus === 'success'
          ? `Pekerjaan ${activeTask?.target || 'instalasi'} selesai!`
          : `Pekerjaan ${activeTask?.target || 'instalasi'} gagal — periksa log antrean`,
        finalStatus === 'success' ? 'success' : 'error'
      );
      await fetchEngines();
      await loadEngineData(activeEngine);
      await fetchTasks();
    });

    eventSource.onerror = () => {
      closeLogStream();
    };
  }

  function closeLogStream() {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  }

  function scrollTerminal() {
    if (terminalEl) {
      setTimeout(() => {
        if (terminalEl) terminalEl.scrollTop = terminalEl.scrollHeight;
      }, 50);
    }
  }

  function randomStrongPassword(): string {
    const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^*_-';
    let pass = '';
    for (let i = 0; i < 18; i++) {
      pass += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return pass;
  }

  function generatePassword() {
    dbPass = randomStrongPassword();
  }

  async function openEngineManager(tab: 'service' | 'processlist' | 'switch_version' | 'config' | 'logs' = 'service') {
    engineTab = tab;
    const curr = currentEngine;
    if (curr.versions && curr.versions.length > 0) {
      const alt = curr.versions.find((v) => v.version !== curr.version) || curr.versions[0];
      switchTargetVersion = alt.version;
    }
    showEngineManager = true;
    if (tab === 'processlist') await fetchProcesses();
    if (tab === 'config') await fetchEngineConfig();
    if (tab === 'logs') await fetchEngineLogs();
  }

  async function handleServiceAction(action: 'start' | 'stop' | 'restart' | 'reload') {
    serviceActionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/databases/${activeEngine}/service`, {
        method: 'POST',
        body: JSON.stringify({ action }),
      });
      notify(res.message, 'success');
      await fetchEngines();
      await fetchTelemetry(activeEngine);
    } catch (e: any) {
      notify(e.message || 'Gagal mengubah status layanan', 'error');
    } finally {
      serviceActionLoading = false;
    }
  }

  async function applyRamTuning(profile: '1gb' | '2gb' | '4gb' | '8gb') {
    const ok = await confirmDialog({
      title: `Terapkan RAM Tuner (${profile.toUpperCase()})`,
      message: `Konfigurasi performa ${currentEngine.display_name} akan dioptimalkan untuk server RAM ${profile.toUpperCase()} dan layanan akan di-restart singkat. Lanjutkan?`,
      confirmText: 'Terapkan Profil',
      tone: 'default',
    });
    if (!ok) return;

    tuningLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/databases/${activeEngine}/tune`, {
        method: 'POST',
        body: JSON.stringify({ profile }),
      });
      notify(res.message || 'Profil RAM berhasil diterapkan', 'success');
      await fetchEngines();
      await fetchTelemetry(activeEngine);
    } catch (e: any) {
      notify(e.message || 'Gagal menerapkan profil tuning RAM', 'error');
    } finally {
      tuningLoading = false;
    }
  }

  async function fetchProcesses() {
    loadingProcesses = true;
    try {
      const res = await apiFetch<{ ok: boolean; processes: DatabaseProcessItem[] }>(`/api/databases/${activeEngine}/processlist`);
      processes = res.processes || [];
    } catch (e: any) {
      processes = [];
      notify(e.message || 'Gagal memuat daftar proses database', 'error');
    } finally {
      loadingProcesses = false;
    }
  }

  async function killProcess(pid: number) {
    const ok = await confirmDialog({
      title: `Hentikan Proses PID ${pid}`,
      message: `Koneksi/kueri dengan PID ${pid} pada ${currentEngine.display_name} akan dihentikan paksa. Lanjutkan?`,
      confirmText: 'Hentikan (Kill)',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/databases/${activeEngine}/processlist/${pid}/kill`, {
        method: 'POST',
      });
      notify(res.message || `PID ${pid} dihentikan`, 'success');
      await fetchProcesses();
    } catch (e: any) {
      notify(e.message || 'Gagal menghentikan proses database', 'error');
    }
  }

  async function submitSwitchVersion() {
    if (!switchTargetVersion) {
      notify('Pilih versi target terlebih dahulu', 'error');
      return;
    }

    const confirmed = await confirmDialog({
      title: `Konfirmasi Alih Versi ${currentEngine.display_name}`,
      message: `Sistem akan membuat cadangan penuh sebelum menyelaraskan ${currentEngine.display_name} ke versi ${switchTargetVersion}. Lanjutkan?`,
      confirmText: 'Mulai Alih Versi',
      tone: 'danger',
    });
    if (!confirmed) return;

    switchingVersion = true;
    try {
      const res = await apiFetch<{ ok: boolean; task_id: string; message: string }>(`/api/databases/${activeEngine}/switch-version`, {
        method: 'POST',
        body: JSON.stringify({
          version: switchTargetVersion,
          backup_first: switchBackupFirst,
        }),
      });
      showEngineManager = false;
      notify(res.message, 'success');
      await fetchTasks();
      openTaskStream(res.task_id);
    } catch (e: any) {
      notify(e.message || 'Gagal memulai alih versi', 'error');
    } finally {
      switchingVersion = false;
    }
  }

  async function fetchEngineConfig() {
    loadingEngineConfig = true;
    try {
      const res = await apiFetch<{ ok: boolean; path: string; content: string }>(`/api/databases/${activeEngine}/config`);
      engineConfigPath = res.path;
      engineConfigContent = res.content;
    } catch {
      notify('Gagal memuat berkas konfigurasi', 'error');
    } finally {
      loadingEngineConfig = false;
    }
  }

  async function saveEngineConfig() {
    savingEngineConfig = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/databases/${activeEngine}/config`, {
        method: 'POST',
        body: JSON.stringify({ content: engineConfigContent }),
      });
      notify(res.message, 'success');
    } catch (e: any) {
      notify(e.message || 'Gagal menyimpan konfigurasi', 'error');
    } finally {
      savingEngineConfig = false;
    }
  }

  async function fetchEngineLogs() {
    loadingEngineLogs = true;
    try {
      const res = await apiFetch<{ ok: boolean; source: string; logs: string }>(`/api/databases/${activeEngine}/logs`);
      engineLogsContent = res.logs;
      engineLogsSource = res.source || 'journalctl';
    } catch {
      notify('Gagal mengambil log database', 'error');
    } finally {
      loadingEngineLogs = false;
    }
  }

  // Simpan kata sandi Root nyata ke server
  async function saveRootPassword() {
    if (rootPassword.trim().length < 6) {
      notify('Kata sandi root minimal 6 karakter', 'warning');
      return;
    }
    savingRootPass = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/databases/${activeEngine}/root-password`, {
        method: 'POST',
        body: JSON.stringify({ password: rootPassword.trim() }),
      });
      notify(res.message || 'Password root berhasil diperbarui', 'success');
      showRootPassModal = false;
      rootPassword = '';
    } catch (e: any) {
      notify(e.message || 'Gagal memperbarui kata sandi root', 'error');
    } finally {
      savingRootPass = false;
    }
  }

  function openQuotaModal(db: DatabaseItem) {
    quotaTargetDb = db;
    quotaMB = db.quota_mb || 0;
    showQuotaModal = true;
  }

  // Simpan kuota kapasitas database ke backend secara persisten
  async function saveQuota() {
    if (!quotaTargetDb) return;
    savingQuota = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(
        `/api/databases/${activeEngine}/${encodeURIComponent(quotaTargetDb.name)}/quota`,
        {
          method: 'PUT',
          body: JSON.stringify({ quota_mb: Number(quotaMB) || 0 }),
        }
      );
      notify(res.message || 'Kuota kapasitas disimpan', 'success');
      showQuotaModal = false;
      await fetchDatabases(activeEngine);
    } catch (e: any) {
      notify(e.message || 'Gagal menyimpan kuota database', 'error');
    } finally {
      savingQuota = false;
    }
  }

  function openUserPasswordModal(db: DatabaseItem) {
    selectedUserPassDb = db;
    userPassValue = db.password || '';
    showUserPassModal = true;
  }

  function generateUserPassword() {
    userPassValue = randomStrongPassword();
  }

  async function submitUserPassword() {
    if (!selectedUserPassDb || !userPassValue.trim()) return;
    savingUserPass = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(
        `/api/databases/${activeEngine}/${encodeURIComponent(selectedUserPassDb.name)}/password`,
        {
          method: 'PUT',
          body: JSON.stringify({ password: userPassValue.trim() }),
        }
      );
      notify(res.message || 'Password database berhasil diperbarui', 'success');
      showUserPassModal = false;
      await fetchDatabases(activeEngine);
    } catch (e: any) {
      notify(e.message || 'Gagal memperbarui password database', 'error');
    } finally {
      savingUserPass = false;
    }
  }

  async function openWebGuiModal(db: DatabaseItem | string = '') {
    if (typeof db === 'object' && db !== null) {
      targetWebGuiDb = db.name;
      targetWebGuiUser = db.username || db.name;
      targetWebGuiPass = db.password || '';
    } else {
      targetWebGuiDb = (typeof db === 'string' ? db : '') || '';
      targetWebGuiUser = (typeof db === 'string' ? db : '') || 'root';
      targetWebGuiPass = '';
    }
    showWebGuiPassword = false;
    showWebGuiModal = true;
    try {
      const res = await apiFetch<any>('/api/databases/webgui/status');
      webGuiStatus = res;
      publicAccessEnabled = res.public_access;
      if (res.available_domains && res.available_domains.length > 0 && !selectedWebGuiDomain) {
        selectedWebGuiDomain = res.available_domains[0].domain;
      }
    } catch {
      // Abaikan bila status default
    }
  }

  function getComputedPublicUrl(): string {
    const userParam = targetWebGuiUser
      ? `&user=${encodeURIComponent(targetWebGuiUser)}&pma_username=${encodeURIComponent(targetWebGuiUser)}`
      : '';
    if (webGuiAccessMode === 'domain' && selectedWebGuiDomain) {
      const targetObj = webGuiStatus?.available_domains?.find((d) => d.domain === selectedWebGuiDomain);
      const scheme = targetObj?.ssl ? 'https' : 'http';
      let url = `${scheme}://${selectedWebGuiDomain}/phpmyadmin/index.php`;
      if (targetWebGuiDb) url += `?route=/database/structure&db=${encodeURIComponent(targetWebGuiDb)}${userParam}`;
      return url;
    }
    if (webGuiAccessMode === 'custom_port') {
      const host = window.location.hostname || '127.0.0.1';
      let url = `http://${host}:${customWebGuiPort}/phpmyadmin/index.php`;
      if (targetWebGuiDb) url += `?route=/database/structure&db=${encodeURIComponent(targetWebGuiDb)}${userParam}`;
      return url;
    }
    let url = `${window.location.origin}/phpmyadmin/index.php`;
    if (targetWebGuiDb) url += `?route=/database/structure&db=${encodeURIComponent(targetWebGuiDb)}${userParam}`;
    return url;
  }

  async function openWebGuiSso() {
    let newTab: Window | null = null;
    try {
      newTab = window.open('about:blank', '_blank');
      if (newTab) newTab.opener = null;
    } catch {
      // Abaikan jika popup dibatasi
    }
    try {
      const res = await apiFetch<{ ok: boolean; url: string; installed: boolean }>('/api/databases/webgui/sso', {
        method: 'POST',
        body: JSON.stringify({
          database: targetWebGuiDb,
          username: targetWebGuiUser,
          password: targetWebGuiPass,
          access_mode: webGuiAccessMode,
          target_domain: selectedWebGuiDomain,
          custom_port: customWebGuiPort,
        }),
      });
      if (!res.installed) {
        if (newTab && !newTab.closed) newTab.close();
        notify('Layanan phpMyAdmin belum terpasang. Pasang terlebih dahulu melalui Quick Install.', 'warning');
        return;
      }
      showWebGuiModal = false;
      if (newTab && !newTab.closed) newTab.location.href = res.url;
      else window.open(res.url, '_blank', 'noopener,noreferrer');
    } catch (e: any) {
      if (newTab && !newTab.closed) newTab.close();
      notify(e.message || 'Gagal membuat sesi SSO Web GUI', 'error');
    }
  }

  async function launchWebGui(db: DatabaseItem) {
    let newTab: Window | null = null;
    try {
      newTab = window.open('about:blank', '_blank');
      if (newTab) newTab.opener = null;
    } catch {
      // Abaikan
    }
    try {
      const res = await apiFetch<{ ok: boolean; url: string; installed: boolean }>('/api/databases/webgui/sso', {
        method: 'POST',
        body: JSON.stringify({
          database: db.name,
          username: db.username || db.name,
          password: db.password || '',
          access_mode: 'panel',
        }),
      });
      if (!res.installed) {
        if (newTab && !newTab.closed) newTab.close();
        notify('Layanan phpMyAdmin belum terpasang. Gunakan SQL Studio bawaan atau pasang phpMyAdmin.', 'warning');
        return;
      }
      if (newTab && !newTab.closed) newTab.location.href = res.url;
      else window.open(res.url, '_blank', 'noopener,noreferrer');
    } catch (e: any) {
      if (newTab && !newTab.closed) newTab.close();
      notify(e.message || 'Gagal membuka phpMyAdmin SSO', 'error');
    }
  }

  function openWebGuiPublic() {
    const publicUrl = getComputedPublicUrl();
    showWebGuiModal = false;
    window.open(publicUrl, '_blank', 'noopener,noreferrer');
  }

  async function onToggleWebGuiPublic(enabled: boolean) {
    publicAccessEnabled = enabled;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/databases/webgui/toggle-public', {
        method: 'POST',
        body: JSON.stringify({ enabled }),
      });
      notify(res.message, 'success');
    } catch (e: any) {
      notify(e.message || 'Gagal mengubah status akses publik', 'error');
    }
  }

  function openAddModal() {
    dbName = '';
    dbCharset = 'utf8mb4';
    dbUser = '';
    userManuallyEdited = false;
    dbPermission = 'localhost';
    customIp = '';
    dbNote = '';
    generatePassword();
    showAddModal = true;
  }

  function onDbNameInput(val: string) {
    dbName = val;
    if (!userManuallyEdited) {
      dbUser = val.trim() ? `${val.trim()}_user` : '';
    }
  }

  function onDbUserInput(val: string) {
    dbUser = val;
    userManuallyEdited = true;
  }

  async function submitAddDatabase() {
    if (!dbName.trim() || !dbUser.trim() || !dbPass.trim()) {
      notify('Nama database, username, dan password wajib diisi', 'error');
      return;
    }
    const host = dbPermission === 'custom' ? (customIp.trim() || 'localhost') : dbPermission;
    creatingDb = true;
    try {
      await apiFetch(`/api/databases/${activeEngine}/create`, {
        method: 'POST',
        body: JSON.stringify({
          name: dbName.trim(),
          username: dbUser.trim(),
          password: dbPass.trim(),
          access_host: host,
          note: dbNote.trim(),
        }),
      });
      notify(`Database ${dbName} berhasil dibuat`, 'success');
      showAddModal = false;
      await fetchEngines();
      await fetchDatabases(activeEngine);
    } catch (e: any) {
      notify(e.message || 'Gagal membuat database', 'error');
    } finally {
      creatingDb = false;
    }
  }

  async function deleteDb(d: DatabaseItem) {
    if (d.is_system) {
      notify(`Database sistem "${d.name}" dilindungi dari penghapusan`, 'warning');
      return;
    }
    const confirmed = await confirmDialog({
      title: 'Hapus Database',
      message: `Hapus database "${d.name}" beserta akun penggunanya secara permanen? Tindakan ini tidak dapat dibatalkan.`,
      confirmText: 'Hapus Permanen',
      tone: 'danger',
    });
    if (!confirmed) return;

    try {
      await apiFetch(`/api/databases/${activeEngine}/${encodeURIComponent(d.name)}`, { method: 'DELETE' });
      notify(`Database ${d.name} berhasil dihapus`, 'success');
      await fetchEngines();
      await fetchDatabases(activeEngine);
    } catch (e: any) {
      notify(e.message || 'Gagal menghapus database', 'error');
    }
  }

  async function copyText(text: string, label: string) {
    if (!text) {
      notify(`Nilai ${label} kosong`, 'warning');
      return;
    }
    if (window.isSecureContext && navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      try {
        await navigator.clipboard.writeText(text);
        notify(`${label} disalin ke papan klip`, 'success');
        return;
      } catch {
        // Lanjut ke fallback textarea
      }
    }
    try {
      const ta = document.createElement('textarea');
      ta.value = text;
      ta.className = 'sr-only';
      document.body.appendChild(ta);
      ta.focus();
      ta.select();
      const ok = document.execCommand('copy');
      document.body.removeChild(ta);
      notify(ok ? `${label} disalin ke papan klip` : 'Gagal menyalin ke papan klip', ok ? 'success' : 'error');
    } catch {
      notify('Gagal menyalin ke papan klip', 'error');
    }
  }

  function formatUptime(sec?: number): string {
    if (!sec || sec <= 0) return '-';
    const d = Math.floor(sec / 86400);
    const h = Math.floor((sec % 86400) / 3600);
    const m = Math.floor((sec % 3600) / 60);
    if (d > 0) return `${d}h ${h}j`;
    if (h > 0) return `${h}j ${m}m`;
    return `${m}m`;
  }
</script>

<div class="pg-databases databases-view">
  <!-- Header Halaman Utama -->
  <PageHeader
    title="Database & Penyimpanan"
    subtitle="Pusat kendali MySQL/MariaDB, PostgreSQL, MongoDB, Redis Live Console, SQL Studio, dan Cadangan S3 Cloud."
    icon="ri-database-2-line"
  />

  <!-- Bar Navigasi Tab Engine -->
  <EngineTabs
    {mainView}
    {activeEngine}
    {engines}
    {tasks}
    onSwitchEngine={switchEngine}
    onSelectBackup={() => { mainView = 'backup'; }}
    onOpenTasks={() => {
      showTasksDrawer = true;
      fetchTasks();
    }}
  />

  {#if mainView === 'backup'}
    <div class="backup-embedded-view">
      <Backup />
    </div>
  {:else if !loadingEngines && !currentEngine.installed}
    <UninstalledCard
      displayName={currentEngine.display_name}
      onInstall={openInstallModal}
      onRemoteDb={() => notify('Gunakan form Tambah DB dengan Remote Host untuk mengelola koneksi jarak jauh', 'info')}
    />
  {:else}
    <!-- 4 Kartu Telemetri Real-Time Engine -->
    <section class="telemetry-kpi-grid" aria-label="Telemetri Engine Database">
      {#if activeEngine === 'redis'}
        <div class="kpi-card">
          <div class="kpi-head">
            <span>Total Kunci Redis</span>
            <i class="ri-key-2-line kpi-icon" aria-hidden="true"></i>
          </div>
          <div class="kpi-value">{(telemetry?.total_keys ?? 0).toLocaleString('id-ID')}</div>
          <div class="kpi-sub">
            {telemetry?.redis_databases?.length || 0} keyspace aktif · Port {currentEngine.port}
          </div>
        </div>
        <div class="kpi-card">
          <div class="kpi-head">
            <span>Memori Terpakai</span>
            <i class="ri-ram-line kpi-icon" aria-hidden="true"></i>
          </div>
          <div class="kpi-value">{formatBytes(telemetry?.memory_used_bytes ?? 0)}</div>
          <div class="kpi-sub">
            Batas: {telemetry?.memory_limit_bytes ? formatBytes(telemetry.memory_limit_bytes) : 'Tanpa batas'}
          </div>
        </div>
        <div class="kpi-card">
          <div class="kpi-head">
            <span>Cache Hit Ratio</span>
            <i class="ri-speed-up-line kpi-icon" aria-hidden="true"></i>
          </div>
          <div class="kpi-value">{(telemetry?.cache_hit_pct ?? 100).toFixed(1)}%</div>
          <div class="kpi-sub">{(telemetry?.qps ?? 0).toFixed(0)} ops/detik</div>
        </div>
        <div class="kpi-card">
          <div class="kpi-head">
            <span>Koneksi & Uptime</span>
            <i class="ri-pulse-line kpi-icon" aria-hidden="true"></i>
          </div>
          <div class="kpi-value">{telemetry?.connections_active ?? 0} klien</div>
          <div class="kpi-sub">Uptime: {formatUptime(telemetry?.uptime_sec)} · v{currentEngine.version || '-'}</div>
        </div>
      {:else}
        <div class="kpi-card">
          <div class="kpi-head">
            <span>Total Database</span>
            <i class="ri-database-2-line kpi-icon" aria-hidden="true"></i>
          </div>
          <div class="kpi-value">{databases.length} DB</div>
          <div class="kpi-sub">Total ukuran: {formatBytes(totalDbBytes)} · {totalBackupsCount} arsip</div>
        </div>
        <div class="kpi-card">
          <div class="kpi-head">
            <span>Koneksi Aktif</span>
            <i class="ri-links-line kpi-icon" aria-hidden="true"></i>
          </div>
          <div class="kpi-value">
            {telemetry?.connections_active ?? 0} / {telemetry?.connections_max ?? '-'}
          </div>
          <div class="kpi-sub">Threads berjalan: {telemetry?.threads_running ?? 0} · Port {currentEngine.port}</div>
        </div>
        <div class="kpi-card">
          <div class="kpi-head">
            <span>Cache / Buffer Hit</span>
            <i class="ri-speed-up-line kpi-icon" aria-hidden="true"></i>
          </div>
          <div class="kpi-value">{(telemetry?.cache_hit_pct ?? 100).toFixed(1)}%</div>
          <div class="kpi-sub">
            {(telemetry?.qps ?? 0).toFixed(1)} q/s · {(telemetry?.queries_total ?? 0).toLocaleString('id-ID')} kueri
          </div>
        </div>
        <div class="kpi-card">
          <div class="kpi-head">
            <span>Slow Queries & Uptime</span>
            <i class="ri-timer-flash-line kpi-icon" aria-hidden="true"></i>
          </div>
          <div class="kpi-value">{telemetry?.slow_queries ?? 0} lambat</div>
          <div class="kpi-sub">Uptime: {formatUptime(telemetry?.uptime_sec)} · v{currentEngine.version || '-'}</div>
        </div>
      {/if}
    </section>

    {#if activeEngine === 'redis'}
      <!-- Konsol Redis Live Key Browser -->
      <RedisConsole
        {currentEngine}
        {telemetry}
        onCopyText={copyText}
        onOpenEngineManager={() => openEngineManager('service')}
        onOpenRootPass={() => {
          rootPassword = '';
          showRootPassModal = true;
        }}
        onRefreshTelemetry={() => fetchTelemetry('redis')}
      />
    {:else}
      <!-- Bilah Aksi & Auto Backup -->
      <ActionBar
        {autoBackupStatus}
        {togglingAutoBackup}
        {syncingHost}
        bind:searchQuery
        {currentEngine}
        onToggleAutoBackup={toggleAutoBackup}
        onAddDb={openAddModal}
        onOpenRootPass={() => {
          rootPassword = '';
          showRootPassModal = true;
        }}
        onOpenWebGui={() => openWebGuiModal('')}
        onSync={syncFromHost}
        onOpenEngineManager={() => openEngineManager('service')}
      />

      <!-- Tabel Database dengan SQL Studio, Backup/Import, .env & Bulk Actions -->
      <DbTable
        {loadingDatabases}
        {filteredDatabases}
        {currentEngine}
        {visiblePasswords}
        {selectedNames}
        {bulkWorking}
        onToggleSelect={toggleSelect}
        onToggleSelectAll={toggleSelectAll}
        onClearSelection={() => (selectedNames = [])}
        onBulkBackup={bulkBackupSelected}
        onBulkDelete={bulkDeleteSelected}
        onTogglePassword={(id) => (visiblePasswords[id] = !visiblePasswords[id])}
        onCopyText={copyText}
        onCopyEnv={copyConnectionEnv}
        onOpenSqlStudio={(db) => {
          sqlStudioDb = db;
          showSqlStudio = true;
        }}
        onOpenBackupModal={(db) => {
          backupImportDb = db;
          showBackupImportModal = true;
        }}
        onOpenQuotaModal={openQuotaModal}
        onOpenPasswordModal={openUserPasswordModal}
        onLaunchWebGui={launchWebGui}
        onDeleteDb={deleteDb}
      />
    {/if}
  {/if}

  <!-- Modal SQL Studio & Penjelajah Tabel -->
  <SqlStudioModal
    open={showSqlStudio && Boolean(sqlStudioDb)}
    engine={activeEngine}
    db={sqlStudioDb}
    onClose={() => (showSqlStudio = false)}
  />

  <!-- Modal Backup 1-Klik, Restore & Impor SQL -->
  <DbBackupImportModal
    open={showBackupImportModal && Boolean(backupImportDb)}
    engine={activeEngine}
    db={backupImportDb}
    onChanged={() => fetchDatabases(activeEngine)}
    onClose={() => (showBackupImportModal = false)}
  />

  <!-- Modal Quick Install Engine -->
  <QuickInstallModal
    open={showInstallModal}
    {installing}
    {activeEngine}
    {currentEngine}
    bind:installVersion
    bind:installMode
    {getEngineIcon}
    onClose={() => (showInstallModal = false)}
    onSubmit={submitInstall}
  />

  <!-- Modal Quick Install phpMyAdmin -->
  <PhpMyAdminInstallModal
    open={showPmaInstallModal}
    {installingPma}
    bind:installPmaVersion
    bind:installPmaPhp
    bind:installPmaMode
    onClose={() => (showPmaInstallModal = false)}
    onSubmit={submitPmaInstall}
  />

  <!-- Modal Tambah Database -->
  <AddDatabaseModal
    open={showAddModal}
    {creatingDb}
    {currentEngine}
    {dbName}
    bind:dbCharset
    {dbUser}
    bind:dbPass
    bind:dbPermission
    bind:customIp
    bind:dbNote
    {onDbNameInput}
    {onDbUserInput}
    onGeneratePassword={generatePassword}
    onClose={() => (showAddModal = false)}
    onSubmit={submitAddDatabase}
  />

  <!-- Modal Engine Manager Lengkap -->
  <EngineManagerModal
    open={showEngineManager}
    {activeEngine}
    {currentEngine}
    bind:engineTab
    {serviceActionLoading}
    {tuningLoading}
    {switchingVersion}
    bind:switchTargetVersion
    bind:switchBackupFirst
    {engineConfigPath}
    bind:engineConfigContent
    {loadingEngineConfig}
    {savingEngineConfig}
    {engineLogsContent}
    {engineLogsSource}
    {loadingEngineLogs}
    {processes}
    {loadingProcesses}
    {getEngineIcon}
    onServiceAction={handleServiceAction}
    onApplyRamTuning={applyRamTuning}
    onFetchProcesses={fetchProcesses}
    onKillProcess={killProcess}
    onSubmitSwitchVersion={submitSwitchVersion}
    onFetchEngineConfig={fetchEngineConfig}
    onSaveEngineConfig={saveEngineConfig}
    onFetchEngineLogs={fetchEngineLogs}
    onClose={() => (showEngineManager = false)}
  />

  <!-- Modal Web GUI phpMyAdmin -->
  <WebGuiModal
    open={showWebGuiModal}
    {webGuiStatus}
    {targetWebGuiDb}
    {targetWebGuiUser}
    {targetWebGuiPass}
    bind:showWebGuiPassword
    bind:webGuiAccessMode
    bind:selectedWebGuiDomain
    bind:customWebGuiPort
    {publicAccessEnabled}
    {getComputedPublicUrl}
    onCopyText={copyText}
    onOpenPmaInstall={openPmaInstallModal}
    onOpenWebGuiSso={openWebGuiSso}
    onToggleWebGuiPublic={onToggleWebGuiPublic}
    onOpenWebGuiPublic={openWebGuiPublic}
    onClose={() => (showWebGuiModal = false)}
  />

  <!-- Modal Kuota Database -->
  <QuotaModal
    open={showQuotaModal && Boolean(quotaTargetDb)}
    {quotaTargetDb}
    bind:quotaMB
    {savingQuota}
    onFormatBytes={formatBytes}
    onClose={() => (showQuotaModal = false)}
    onSave={saveQuota}
  />

  <!-- Modal Password Root -->
  <RootPasswordModal
    open={showRootPassModal}
    {currentEngine}
    bind:rootPassword
    bind:showRootPass
    saving={savingRootPass}
    onCopyText={copyText}
    onGenerate={() => (rootPassword = randomStrongPassword())}
    onClose={() => (showRootPassModal = false)}
    onSave={saveRootPassword}
  />

  <!-- Modal Ubah Password Pengguna -->
  <UserPasswordModal
    open={showUserPassModal && Boolean(selectedUserPassDb)}
    db={selectedUserPassDb}
    bind:newPassword={userPassValue}
    saving={savingUserPass}
    onGeneratePassword={generateUserPassword}
    onClose={() => (showUserPassModal = false)}
    onSubmit={submitUserPassword}
  />

  <!-- Drawer Antrean Tugas & Log Streaming -->
  <TasksDrawer
    open={showTasksDrawer}
    {tasks}
    {activeTask}
    {streamLogs}
    bind:terminalEl
    onOpenTaskStream={openTaskStream}
    onRefreshTasks={() => {
      fetchTasks();
      if (activeTask) openTaskStream(activeTask.id);
    }}
    onClose={() => (showTasksDrawer = false)}
  />
</div>
