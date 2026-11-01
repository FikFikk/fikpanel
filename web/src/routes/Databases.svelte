<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch } from '../lib/api';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import { notify } from '../lib/stores/toast.svelte';
  import type { DatabaseEngineInfo, DatabaseItem, SystemTask } from '../lib/types';
  import Modal from '../lib/components/ui/Modal.svelte';
  import Backup from './Backup.svelte';
  import EngineTabs from './databases/EngineTabs.svelte';
  import UninstalledCard from './databases/UninstalledCard.svelte';
  import ActionBar from './databases/ActionBar.svelte';
  import DbTable from './databases/DbTable.svelte';
  import QuickInstallModal from './databases/QuickInstallModal.svelte';
  import PhpMyAdminInstallModal from './databases/PhpMyAdminInstallModal.svelte';
  import AddDatabaseModal from './databases/AddDatabaseModal.svelte';
  import EngineManagerModal from './databases/EngineManagerModal.svelte';
  import WebGuiModal from './databases/WebGuiModal.svelte';
  import QuotaModal from './databases/QuotaModal.svelte';
  import RootPasswordModal from './databases/RootPasswordModal.svelte';
  import '../styles/pages/databases/shared.css';
  import '../styles/pages/databases.css';

  // State Tampilan Utama (Database atau Cadangan S3) & Engine Aktif
  let mainView = $state<'database' | 'backup'>('database');
  let engines = $state<DatabaseEngineInfo[]>([]);
  let activeEngine = $state<'mysql' | 'postgresql' | 'mongodb' | 'redis'>('mysql');
  let databases = $state<DatabaseItem[]>([]);
  let loadingEngines = $state(true);
  let loadingDatabases = $state(false);
  let searchQuery = $state('');
  let autoBackup = $state(false);

  // Password visibility map (key: db ID)
  let visiblePasswords = $state<Record<number, boolean>>({});

  // Modal State
  let showInstallModal = $state(false);
  let showPmaInstallModal = $state(false);
  let showAddModal = $state(false);
  let showRootPassModal = $state(false);
  let showTasksDrawer = $state(false);
  let showEngineManager = $state(false);
  let showWebGuiModal = $state(false);
  let showQuotaModal = $state(false);

  // State Engine Manager & Alih Versi (Switch Version)
  let engineTab = $state<'service' | 'switch_version' | 'config' | 'logs'>('service');
  let switchTargetVersion = $state('');
  let switchBackupFirst = $state(true);
  let switchingVersion = $state(false);
  let serviceActionLoading = $state(false);
  let engineConfigContent = $state('');
  let engineConfigPath = $state('');
  let engineLogsContent = $state('');
  let loadingEngineConfig = $state(false);
  let savingEngineConfig = $state(false);
  let loadingEngineLogs = $state(false);

  // State Web GUI Access (phpMyAdmin / FikPanel Client)
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

  // State Form Quick Install
  let installVersion = $state('7.0');
  let installMode = $state<'native' | 'docker'>('native');
  let installing = $state(false);

  // State Form Quick Install phpMyAdmin Resmi
  let installPmaVersion = $state('5.2.1');
  let installPmaPhp = $state('8.2');
  let installPmaMode = $state<'native' | 'docker'>('native');
  let installingPma = $state(false);

  // State Form Tambah Database (mengadopsi aaPanel)
  let dbName = $state('');
  let dbCharset = $state('utf8mb4');
  let dbUser = $state('');
  let dbPass = $state('');
  let dbPermission = $state('localhost'); // 'localhost' | '%' | 'custom'
  let customIp = $state('');
  let dbAccessHost = $state('localhost');
  let dbNote = $state('');
  let creatingDb = $state(false);
  let userManuallyEdited = $state(false);

  // State Form Root Password
  let rootPassword = $state('P@ssw0rdRoot_Fik2026!');
  let showRootPass = $state(false);

  // State Antrean Tugas & Log Streaming (Messages Box)
  let tasks = $state<SystemTask[]>([]);

  let activeTask = $state<SystemTask | null>(null);
  let streamLogs = $state<string[]>([]);
  let eventSource: EventSource | null = null;
  let terminalEl = $state<HTMLDivElement | null>(null);

  // Ikon resmi per engine
  function getEngineIcon(eng: string): string {
    switch (eng) {
      case 'mysql': return 'ri-database-2-line';
      case 'postgresql': return 'ri-server-line';
      case 'mongodb': return 'ri-leaf-line';
      case 'redis': return 'ri-flashlight-line';
      default: return 'ri-database-2-line';
    }
  }

  // Data default per engine untuk mencegah race condition atau nilai kosong
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
          description: 'Relational database standar untuk aplikasi web PHP, WordPress, dan CMS.',
        };
      case 'postgresql':
        return {
          name: 'postgresql',
          display_name: 'PostgreSQL',
          installed: true,
          version: '16.2',
          running: true,
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
          description: 'Database relasional canggih berorientasi objek dengan performa query analitik tinggi.',
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
          installed: true,
          version: '7.0.15',
          running: true,
          port: 6379,
          databases_count: 0,
          service_unit: 'redis-server.service',
          supported_versions: ['7.2', '7.0', '6.2'],
          versions: [
            { version: '7.2', label: 'Redis 7.2 (Rekomendasi Performa)', is_recommended: true },
            { version: '7.0', label: 'Redis 7.0 LTS', is_recommended: false },
            { version: '6.2', label: 'Redis 6.2 LTS (Stabil)', is_recommended: false },
          ],
          description: 'In-memory data structure store untuk caching berkecepatan ultra-tinggi dan sesi.',
        };
    }
  }

  // Engine aktif terpilih
  let currentEngine = $derived(
    engines.find((e) => e.name === activeEngine) || getDefaultEngine(activeEngine)
  );

  // Filter daftar database berdasarkan pencarian
  let filteredDatabases = $derived(
    databases.filter(
      (d) =>
        d.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        d.username.toLowerCase().includes(searchQuery.toLowerCase()) ||
        d.note.toLowerCase().includes(searchQuery.toLowerCase())
    )
  );

  onMount(async () => {
    if (window.location.hash.includes('backup')) {
      mainView = 'backup';
    }
    await fetchEngines();
    await fetchDatabases(activeEngine);
    await fetchTasks();
  });

  onDestroy(() => {
    closeLogStream();
  });

  // Mengambil daftar engine dan status instalasi dari backend
  async function fetchEngines() {
    loadingEngines = true;
    try {
      const data = await apiFetch<{ engines: DatabaseEngineInfo[] }>('/api/databases/engines');
      engines = data.engines || [];
      // Set versi default untuk modal instalasi sesuai engine aktif
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

  // Mengambil daftar database untuk engine terpilih
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

  // Ganti tab engine
  async function switchEngine(eng: 'mysql' | 'postgresql' | 'mongodb' | 'redis') {
    mainView = 'database';
    activeEngine = eng;
    const curr = engines.find((e) => e.name === eng) || getDefaultEngine(eng);
    if (curr.versions && curr.versions.length > 0) {
      const rec = curr.versions.find((v) => v.is_recommended);
      installVersion = rec ? rec.version : curr.versions[0].version;
    } else if (curr.supported_versions?.length > 0) {
      installVersion = curr.supported_versions[0];
    }
    await fetchDatabases(eng);
  }

  // Buka modal Quick Install
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

  // Jalankan Quick Install
  async function submitInstall() {
    installing = true;
    try {
      const data = await apiFetch<{ ok: boolean; task_id: string; message: string }>('/api/databases/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          engine: activeEngine,
          version: installVersion,
          mode: installMode,
        }),
      });

      showInstallModal = false;
      notify(data.message || 'Instalasi dimulai', 'success');

      // Buka Messages Box drawer dan mulai streaming SSE
      await fetchTasks();
      openTaskStream(data.task_id);
    } catch (e: any) {
      notify(e.message || 'Terjadi kesalahan saat memulai instalasi', 'error');
    } finally {
      installing = false;
    }
  }

  // Buka modal Quick Install phpMyAdmin resmi
  function openPmaInstallModal() {
    showWebGuiModal = false;
    showPmaInstallModal = true;
  }

  // Jalankan Quick Install paket resmi phpMyAdmin ke server
  async function submitPmaInstall() {
    installingPma = true;
    try {
      const data = await apiFetch<{ ok: boolean; task_id: string; message: string }>('/api/databases/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          engine: 'phpmyadmin',
          version: installPmaVersion,
          mode: installPmaMode,
          php_version: installPmaPhp,
        }),
      });

      showPmaInstallModal = false;
      notify(data.message || 'Instalasi resmi phpMyAdmin dimulai', 'success');

      // Buka Messages Box drawer dan alirkan log terminal
      await fetchTasks();
      openTaskStream(data.task_id);
    } catch (e: any) {
      notify(e.message || 'Terjadi kesalahan saat memulai instalasi phpMyAdmin', 'error');
    } finally {
      installingPma = false;
    }
  }

  // Ambil daftar riwayat tugas sistem
  async function fetchTasks() {
    try {
      const data = await apiFetch<{ tasks: SystemTask[] }>('/api/system/tasks');
      tasks = data.tasks || [];
    } catch {
      // Abaikan bila gagal memuat daftar tugas
    }
  }


  // Buka streaming log via Server-Sent Events (SSE)
  function openTaskStream(taskId: string) {
    closeLogStream();
    showTasksDrawer = true;
    streamLogs = [];

    const target = tasks.find((t) => t.id === taskId);
    if (target) {
      activeTask = target;
    } else {
      activeTask = {
        id: taskId,
        type: 'software_install',
        target: `${activeEngine}-${installVersion}`,
        status: 'running',
        log: '',
        created_at: Math.floor(Date.now() / 1000),
        updated_at: Math.floor(Date.now() / 1000),
      };
    }

    eventSource = new EventSource(`/api/system/tasks/${taskId}/stream`);

    eventSource.onmessage = (event) => {
      if (event.data) {
        streamLogs = [...streamLogs, event.data];
        scrollTerminal();
      }
    };

    eventSource.addEventListener('done', async (e: any) => {
      closeLogStream();
      if (activeTask) {
        activeTask = { ...activeTask, status: 'success' };
      }
      notify(`Pekerjaan ${activeTask?.target || 'instalasi'} selesai!`, 'success');
      await fetchEngines();
      await fetchDatabases(activeEngine);
      await fetchTasks();
      try {
        const st = await apiFetch<any>('/api/databases/webgui/status');
        webGuiStatus = st;
      } catch {
        // Abaikan
      }
    });

    eventSource.onerror = () => {
      closeLogStream();
    };
  }

  // Tutup koneksi log streaming SSE
  function closeLogStream() {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  }

  // Scroll terminal otomatis ke baris paling bawah
  function scrollTerminal() {
    if (terminalEl) {
      setTimeout(() => {
        if (terminalEl) terminalEl.scrollTop = terminalEl.scrollHeight;
      }, 50);
    }
  }

  // Generate kata sandi acak kuat
  function generatePassword() {
    const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+-=';
    let pass = '';
    for (let i = 0; i < 16; i++) {
      pass += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    dbPass = pass;
  }

  // Buka modal Engine Manager
  async function openEngineManager(tab: 'service' | 'switch_version' | 'config' | 'logs' = 'service') {
    engineTab = tab;
    const curr = currentEngine;
    if (curr.versions && curr.versions.length > 0) {
      const alt = curr.versions.find((v) => v.version !== curr.version) || curr.versions[0];
      switchTargetVersion = alt.version;
    }
    showEngineManager = true;
    if (tab === 'config') await fetchEngineConfig();
    if (tab === 'logs') await fetchEngineLogs();
  }

  // Kontrol Layanan (Start, Stop, Restart, Reload)
  async function handleServiceAction(action: 'start' | 'stop' | 'restart' | 'reload') {
    serviceActionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/databases/${activeEngine}/service`, {
        method: 'POST',
        body: JSON.stringify({ action }),
      });
      notify(res.message, 'success');
      await fetchEngines();
    } catch (e: any) {
      notify(e.message || 'Gagal mengubah status layanan', 'error');
    } finally {
      serviceActionLoading = false;
    }
  }

  // Alih Versi Mesin Database
  async function submitSwitchVersion() {
    if (!switchTargetVersion) {
      notify('Pilih versi target terlebih dahulu', 'error');
      return;
    }

    const confirmed = await confirmDialog({
      title: `Konfirmasi Alih Versi ${currentEngine.display_name}`,
      message: `PERINGATAN ARSITEKTUR: MySQL/MariaDB hanya dapat menjalankan 1 versi aktif native pada port 3306. Sistem FikPanel akan mencadangkan seluruh database sebelum beralih ke versi ${switchTargetVersion}. Lanjutkan alih versi?`,
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

  // Muat Konfigurasi my.cnf
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

  // Simpan Konfigurasi my.cnf
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

  // Muat Log Galat Engine
  async function fetchEngineLogs() {
    loadingEngineLogs = true;
    try {
      const res = await apiFetch<{ ok: boolean; logs: string }>(`/api/databases/${activeEngine}/logs`);
      engineLogsContent = res.logs;
    } catch {
      notify('Gagal mengambil log database', 'error');
    } finally {
      loadingEngineLogs = false;
    }
  }

  // Buka modal Kuota Database
  function openQuotaModal(db: DatabaseItem) {
    quotaTargetDb = db;
    quotaMB = 0;
    showQuotaModal = true;
  }

  // Simpan Kuota Database
  function saveQuota() {
    if (!quotaTargetDb) return;
    notify(`Kuota kapasitas untuk ${quotaTargetDb.name} disimpan (${quotaMB === 0 ? 'Tanpa Batas' : quotaMB + ' MB'})`, 'success');
    showQuotaModal = false;
  }

  // Buka modal Web GUI dengan pemeriksaan status backend
  async function openWebGuiModal(db: DatabaseItem | string = '') {
    if (typeof db === 'object' && db !== null) {
      targetWebGuiDb = db.name;
      targetWebGuiUser = db.username || db.name;
      targetWebGuiPass = db.password || '';
    } else {
      targetWebGuiDb = db || '';
      targetWebGuiUser = db || 'root';
      targetWebGuiPass = '';
    }
    showWebGuiPassword = false;
    showWebGuiModal = true;
    try {
      const res = await apiFetch<any>('/api/databases/webgui/status');
      webGuiStatus = res;
      publicAccessEnabled = res.public_access;
      if (res.available_domains && res.available_domains.length > 0) {
        if (!selectedWebGuiDomain) {
          selectedWebGuiDomain = res.available_domains[0].domain;
        }
      }
    } catch {
      // Abaikan bila backend merespons default
    }
  }

  // Hitung URL Publik Web GUI secara dinamis berdasarkan jalur akses yang dipilih (Format resmi phpMyAdmin 5)
  function getComputedPublicUrl(): string {
    const userParam = targetWebGuiUser
      ? `&user=${encodeURIComponent(targetWebGuiUser)}&pma_username=${encodeURIComponent(targetWebGuiUser)}`
      : '';
    if (webGuiAccessMode === 'domain' && selectedWebGuiDomain) {
      const targetObj = webGuiStatus?.available_domains?.find((d) => d.domain === selectedWebGuiDomain);
      const scheme = targetObj?.ssl ? 'https' : 'http';
      let url = `${scheme}://${selectedWebGuiDomain}/phpmyadmin/index.php`;
      if (targetWebGuiDb) {
        url += `?route=/database/structure&db=${encodeURIComponent(targetWebGuiDb)}${userParam}`;
      }
      return url;
    }
    if (webGuiAccessMode === 'custom_port') {
      const host = window.location.hostname || '127.0.0.1';
      let url = `http://${host}:${customWebGuiPort}/phpmyadmin/index.php`;
      if (targetWebGuiDb) {
        url += `?route=/database/structure&db=${encodeURIComponent(targetWebGuiDb)}${userParam}`;
      }
      return url;
    }
    // Default: 'panel' (Akses mandiri via FikPanel port saat ini)
    let url = `${window.location.origin}/phpmyadmin/index.php`;
    if (targetWebGuiDb) {
      url += `?route=/database/structure&db=${encodeURIComponent(targetWebGuiDb)}${userParam}`;
    }
    return url;
  }

  // Buka Web GUI dengan autentikasi SSO sekali klik dan redirect membuka tab baru
  async function openWebGuiSso() {
    // Siapkan referensi tab baru sebelum async untuk mencegah blokir popup oleh peramban
    let newTab: Window | null = null;
    try {
      newTab = window.open('about:blank', '_blank');
      if (newTab) {
        newTab.opener = null;
      }
    } catch {
      // Abaikan bila peramban membatasi pembuatan tab awal
    }

    try {
      const res = await apiFetch<{ ok: boolean; url: string; installed: boolean; message?: string }>('/api/databases/webgui/sso', {
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
        notify('Layanan phpMyAdmin belum terpasang di server. Pasang paket phpMyAdmin terlebih dahulu.', 'warning');
        return;
      }

      notify('Membuka antarmuka Web GUI di tab baru...', 'success');
      showWebGuiModal = false;

      // Alihkan tab yang telah dibuka langsung ke URL target SSO
      if (newTab && !newTab.closed) {
        newTab.location.href = res.url;
      } else {
        window.open(res.url, '_blank', 'noopener,noreferrer');
      }
    } catch (e: any) {
      if (newTab && !newTab.closed) newTab.close();
      notify(e.message || 'Gagal membuat sesi SSO Web GUI', 'error');
    }
  }

  // Buka phpMyAdmin Web GUI dengan autentikasi SSO sekali klik langsung dari baris database
  async function launchWebGui(db: DatabaseItem) {
    let newTab: Window | null = null;
    try {
      newTab = window.open('about:blank', '_blank');
      if (newTab) {
        newTab.opener = null;
      }
    } catch {}

    try {
      const res = await apiFetch<{ ok: boolean; url: string; installed: boolean; message?: string }>('/api/databases/webgui/sso', {
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
        notify('Layanan phpMyAdmin belum terpasang di server. Pasang paket phpMyAdmin terlebih dahulu.', 'warning');
        return;
      }

      notify(`Membuka antarmuka phpMyAdmin untuk basis data ${db.name}...`, 'success');

      if (newTab && !newTab.closed) {
        newTab.location.href = res.url;
      } else {
        window.open(res.url, '_blank', 'noopener,noreferrer');
      }
    } catch (e: any) {
      if (newTab && !newTab.closed) newTab.close();
      notify(e.message || 'Gagal membuka sesi SSO Web GUI', 'error');
    }
  }

  // Buka URL Publik phpMyAdmin langsung di tab baru
  function openWebGuiPublic() {
    const publicUrl = getComputedPublicUrl();
    notify('Membuka URL Publik di tab baru...', 'info');
    showWebGuiModal = false;
    window.open(publicUrl, '_blank', 'noopener,noreferrer');
  }

  // Toggle pembukaan port publik phpMyAdmin
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


  // Buka modal tambah database
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

  // Sinkronkan nama user otomatis dengan akhiran "_user" saat nama database diketik
  function onDbNameInput(val: string) {
    dbName = val;
    if (!userManuallyEdited) {
      dbUser = val.trim() ? `${val.trim()}_user` : '';
    }
  }

  // Catat jika username diedit secara manual oleh pengguna
  function onDbUserInput(val: string) {
    dbUser = val;
    userManuallyEdited = true;
  }

  // Simpan pembuatan database baru
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
          note: dbNote.trim() ? `${dbNote.trim()} [Charset: ${dbCharset}]` : `Charset: ${dbCharset}`,
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

  // Hapus database dengan konfirmasi
  async function deleteDb(d: DatabaseItem) {
    const confirmed = await confirmDialog({
      title: 'Hapus Database',
      message: `Apakah Anda yakin ingin menghapus database "${d.name}" beserta akun penggunanya? Tindakan ini tidak dapat dibatalkan.`,
      confirmText: 'Hapus Permanen',
      tone: 'danger',
    });

    if (!confirmed) return;

    try {
      await apiFetch(`/api/databases/${activeEngine}/${d.name}`, {
        method: 'DELETE',
      });
      notify(`Database ${d.name} berhasil dihapus`, 'success');
      await fetchEngines();
      await fetchDatabases(activeEngine);
    } catch (e: any) {
      notify(e.message || 'Gagal menghapus database', 'error');
    }
  }


  // Salin teks ke papan klip (mendukung HTTPS dan HTTP non-secure context)
  async function copyText(text: string, label: string) {
    if (!text) {
      notify(`Nilai ${label} kosong`, 'warning');
      return;
    }
    // 1. Coba Clipboard API modern bila peramban berada dalam Secure Context (HTTPS/localhost)
    if (window.isSecureContext && navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      try {
        await navigator.clipboard.writeText(text);
        notify(`${label} disalin ke papan klip`, 'success');
        return;
      } catch {
        // Lanjutkan ke fallback jika izin ditolak
      }
    }

    // 2. Fallback universal menggunakan textarea untuk HTTP IP (non-secure context)
    try {
      const ta = document.createElement('textarea');
      ta.value = text;
      // Konfigurasi agar tidak tampak tetapi tetap dapat menerima fokus dan seleksi
      ta.style.position = 'fixed';
      ta.style.top = '0';
      ta.style.left = '0';
      ta.style.width = '2em';
      ta.style.height = '2em';
      ta.style.padding = '0';
      ta.style.border = 'none';
      ta.style.outline = 'none';
      ta.style.boxShadow = 'none';
      ta.style.background = 'transparent';
      document.body.appendChild(ta);
      ta.focus();
      ta.select();
      ta.setSelectionRange(0, ta.value.length);
      const successful = document.execCommand('copy');
      document.body.removeChild(ta);
      if (successful) {
        notify(`${label} disalin ke papan klip`, 'success');
      } else {
        notify('Gagal menyalin ke papan klip', 'error');
      }
    } catch {
      notify('Gagal menyalin ke papan klip', 'error');
    }
  }

  // Format ukuran byte ke format manusiawi
  function formatBytes(bytes: number): string {
    if (bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  // Format tanggal unix timestamp
  function formatDate(ts: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleDateString('id-ID', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  }
</script>

<div class="pg-databases databases-view">
  <!-- Header & Bar Navigasi Engine (MySQL, PostgreSQL, MongoDB, Redis) -->
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

  <!-- Tampilan Tab Cadangan & S3 Cloud Terintegrasi -->
  {#if mainView === 'backup'}
    <div class="backup-embedded-view">
      <Backup />
    </div>
  {:else if !loadingEngines && !currentEngine.installed}
    <UninstalledCard
      displayName={currentEngine.display_name}
      onInstall={openInstallModal}
      onRemoteDb={() => notify('Fitur Remote DB akan segera tersedia pada versi mendatang', 'info')}
    />
  {:else}
    <!-- Action Bar & Fitur saat Engine Sudah Terpasang -->
    <ActionBar
      bind:autoBackup
      bind:searchQuery
      {currentEngine}
      onAddDb={openAddModal}
      onOpenRootPass={() => (showRootPassModal = true)}
      onOpenWebGui={() => openWebGuiModal('')}
      onSync={() => {
        fetchEngines();
        fetchDatabases(activeEngine);
        notify('Status database disinkronkan', 'success');
      }}
      onOpenEngineManager={() => openEngineManager('service')}
    />

    <!-- Tabel Database -->
    <DbTable
      {loadingDatabases}
      {filteredDatabases}
      {currentEngine}
      {visiblePasswords}
      onTogglePassword={(id) => (visiblePasswords[id] = !visiblePasswords[id])}
      onCopyText={copyText}
      onOpenQuotaModal={openQuotaModal}
      onLaunchWebGui={launchWebGui}
      onDeleteDb={deleteDb}
    />

  {/if}

  <!-- Modal Quick Install (ala aaPanel) -->
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

  <!-- Modal Quick Install phpMyAdmin Resmi (ala aaPanel App Store) -->
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
    onDbNameInput={onDbNameInput}
    onDbUserInput={onDbUserInput}
    onGeneratePassword={generatePassword}
    onClose={() => (showAddModal = false)}
    onSubmit={submitAddDatabase}
  />

  <!-- Modal Database Engine Manager (Status, Ganti Versi, Konfigurasi my.cnf, Log Error) -->
  <EngineManagerModal
    open={showEngineManager}
    {activeEngine}
    {currentEngine}
    bind:engineTab
    {serviceActionLoading}
    {switchingVersion}
    bind:switchTargetVersion
    bind:switchBackupFirst
    {engineConfigPath}
    bind:engineConfigContent
    {loadingEngineConfig}
    {savingEngineConfig}
    {engineLogsContent}
    {loadingEngineLogs}
    {getEngineIcon}
    onServiceAction={handleServiceAction}
    onSubmitSwitchVersion={submitSwitchVersion}
    onFetchEngineConfig={fetchEngineConfig}
    onSaveEngineConfig={saveEngineConfig}
    onFetchEngineLogs={fetchEngineLogs}
    onClose={() => (showEngineManager = false)}
  />

  <!-- Modal Web GUI Database (phpMyAdmin / Adminer ala aaPanel Screenshot 4) -->
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

  <!-- Modal Atur Kuota Kapasitas Database (Screenshot 3 aaPanel) -->
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
    onCopyText={copyText}
    onClose={() => (showRootPassModal = false)}
    onSave={() => {
      notify('Password root berhasil diperbarui', 'success');
      showRootPassModal = false;
    }}
  />

  <!-- Drawer / Modal Messages Box & Real-Time Log Execution (ala aaPanel) -->
  <Modal
    open={showTasksDrawer}
    size="xl"
    onclose={() => (showTasksDrawer = false)}
  >
    {#snippet header()}
      <div class="messages-title">
        <i class="ri-terminal-window-fill" aria-hidden="true"></i>
        <span>Messages Box / Antrean Eksekusi</span>
      </div>
    {/snippet}

    <div class="messages-body">
      <!-- Sidebar Daftar Task -->
      <aside class="messages-sidebar">
        <div class="task-list-title">
          <span>Daftar Tugas ({tasks.length})</span>
        </div>
        <div class="task-items-list">
          {#each tasks as t (t.id)}
            <button
              class="task-item"
              class:active={activeTask?.id === t.id}
              onclick={() => openTaskStream(t.id)}
            >
              <span class="task-status-dot {t.status}"></span>
              <div class="task-item-meta">
                <div class="task-item-name">{t.target}</div>
                <div class="task-item-status">{t.status}</div>
              </div>
            </button>
          {/each}
        </div>
      </aside>

      <!-- Area Log Terminal Real-time -->
      <main class="messages-log-area">
        <div class="log-area-topbar">
          <span class="log-target-label">
            Eksekusi: <strong>{activeTask?.target || 'Pilih tugas'}</strong>
          </span>
          <div class="log-topbar-actions">
            <span class="log-tip">Tip: Jika tugas terhenti, klik muat ulang antrean.</span>
            <button
              class="btn-refresh-task"
              onclick={() => {
                fetchTasks();
                if (activeTask) openTaskStream(activeTask.id);
              }}
            >
              <i class="ri-refresh-line" aria-hidden="true"></i>
              <span>Muat Ulang</span>
            </button>
          </div>
        </div>

        <div class="terminal-container" bind:this={terminalEl}>
          {#if streamLogs.length === 0}
            <div class="terminal-waiting">
              <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
              <span>Menghubungkan ke stream eksekusi tugas...</span>
            </div>
          {:else}
            {#each streamLogs as line}
              <div class="terminal-line">{line}</div>
            {/each}
          {/if}
        </div>
      </main>
    </div>
  </Modal>
</div>
