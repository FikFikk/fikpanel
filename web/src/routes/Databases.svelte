<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch } from '../lib/api';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import { notify } from '../lib/stores/toast.svelte';
  import type { DatabaseEngineInfo, DatabaseItem, SystemTask } from '../lib/types';

  // State Engine dan Tab Aktif
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

  // State Web GUI Access (phpMyAdmin / Adminer)
  let publicAccessEnabled = $state(false);

  // State Kuota Database
  let quotaTargetDb = $state<DatabaseItem | null>(null);
  let quotaMB = $state(0);
  let savingQuota = $state(false);

  // State Form Quick Install
  let installVersion = $state('7.0');
  let installMode = $state<'native' | 'docker'>('native');
  let installing = $state(false);

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
      notify(`Instalasi ${currentEngine.display_name} selesai!`, 'success');
      await fetchEngines();
      await fetchDatabases(activeEngine);
      await fetchTasks();
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
      danger: true,
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

  // Buka modal tambah database
  function openAddModal() {
    dbName = '';
    dbCharset = 'utf8mb4';
    dbUser = '';
    dbPermission = 'localhost';
    customIp = '';
    dbNote = '';
    generatePassword();
    showAddModal = true;
  }

  // Sinkronkan nama user otomatis saat nama database diketik
  function onDbNameInput(val: string) {
    dbName = val;
    if (!dbUser || dbUser === val.slice(0, -1)) {
      dbUser = val;
    }
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
      danger: true,
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


  // Salin teks ke papan klip
  async function copyText(text: string, label: string) {
    try {
      await navigator.clipboard.writeText(text);
      notify(`${label} disalin ke papan klip`, 'success');
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

<div class="databases-view">
  <!-- Header & Bar Navigasi Engine (MySQL, PostgreSQL, MongoDB, Redis) -->
  <header class="engine-tabs-header">
    <div class="engine-tabs" role="tablist">
      <button
        class="engine-tab"
        class:active={activeEngine === 'mysql'}
        role="tab"
        aria-selected={activeEngine === 'mysql'}
        onclick={() => switchEngine('mysql')}
      >
        <i class="ri-database-2-line tab-icon" aria-hidden="true"></i>
        <span>MySQL / MariaDB</span>
        {#if engines.find((e) => e.name === 'mysql')?.installed}
          <span class="engine-dot active" title="Layanan aktif"></span>
        {/if}
      </button>

      <button
        class="engine-tab"
        class:active={activeEngine === 'postgresql'}
        role="tab"
        aria-selected={activeEngine === 'postgresql'}
        onclick={() => switchEngine('postgresql')}
      >
        <i class="ri-server-line tab-icon" aria-hidden="true"></i>
        <span>PostgreSQL</span>
        {#if engines.find((e) => e.name === 'postgresql')?.installed}
          <span class="engine-dot active" title="Layanan aktif"></span>
        {/if}
      </button>

      <button
        class="engine-tab"
        class:active={activeEngine === 'mongodb'}
        role="tab"
        aria-selected={activeEngine === 'mongodb'}
        onclick={() => switchEngine('mongodb')}
      >
        <i class="ri-leaf-line tab-icon" aria-hidden="true"></i>
        <span>MongoDB</span>
        {#if engines.find((e) => e.name === 'mongodb')?.installed}
          <span class="engine-dot active" title="Layanan aktif"></span>
        {:else}
          <span class="engine-dot uninstalled" title="Belum terpasang"></span>
        {/if}
      </button>

      <button
        class="engine-tab"
        class:active={activeEngine === 'redis'}
        role="tab"
        aria-selected={activeEngine === 'redis'}
        onclick={() => switchEngine('redis')}
      >
        <i class="ri-flashlight-line tab-icon" aria-hidden="true"></i>
        <span>Redis</span>
        {#if engines.find((e) => e.name === 'redis')?.installed}
          <span class="engine-dot active" title="Layanan aktif"></span>
        {/if}
      </button>
    </div>

    <div class="header-tools">
      <button
        class="btn-task-queue"
        onclick={() => {
          showTasksDrawer = true;
          fetchTasks();
        }}
        title="Buka Antrean Tugas & Log Instalasi"
      >
        <i class="ri-terminal-window-line" aria-hidden="true"></i>
        <span>Antrean Tugas</span>
        {#if tasks.some((t) => t.status === 'running')}
          <span class="badge-running-count">1</span>
        {/if}
      </button>
    </div>
  </header>

  <!-- Jika Mesin Database BELUM Terpasang (State Edukatif ala aaPanel) -->
  {#if !loadingEngines && !currentEngine.installed}
    <div class="uninstalled-card">
      <div class="uninstalled-card__body">
        <div class="alert-icon-box">
          <i class="ri-error-warning-fill" aria-hidden="true"></i>
        </div>
        <div class="uninstalled-card__content">
          <h2 class="uninstalled-title">
            Server database {currentEngine.display_name} belum terkonfigurasi
          </h2>
          <p class="uninstalled-desc">
            Layanan {currentEngine.display_name} belum dipasang pada server ini. Anda dapat menginstal
            paket resmi secara otomatis dengan Quick Install atau menghubungkan database eksternal / remote.
          </p>
          <div class="uninstalled-actions">
            <button class="btn btn-primary" onclick={openInstallModal}>
              <i class="ri-download-cloud-line" aria-hidden="true"></i>
              <span>Pasang Sekarang (Quick Install)</span>
            </button>
            <button
              class="btn btn-outline"
              onclick={() => notify('Fitur Remote DB akan segera tersedia pada versi mendatang', 'info')}
            >
              <i class="ri-external-link-line" aria-hidden="true"></i>
              <span>Hubungkan Remote DB</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  {:else}
    <!-- Action Bar & Fitur saat Engine Sudah Terpasang -->
    <div class="action-bar-card">
      <!-- Toggle Auto Backup (ala aaPanel) -->
      <div class="auto-backup-row">
        <label class="toggle-switch">
          <input type="checkbox" bind:checked={autoBackup} />
          <span class="slider"></span>
        </label>
        <span class="auto-backup-label">
          <strong>Auto Backup Database:</strong>
          Setelah menambahkan database, Anda dapat mengaktifkan backup otomatis terjadwal untuk keamanan data.
        </span>
      </div>

      <!-- Action Buttons -->
      <div class="action-buttons-row">
        <div class="btn-group-left">
          <button class="btn btn-primary" onclick={openAddModal}>
            <i class="ri-add-line" aria-hidden="true"></i>
            <span>Tambah DB</span>
          </button>
          <button class="btn btn-outline" onclick={() => (showRootPassModal = true)}>
            <i class="ri-key-2-line" aria-hidden="true"></i>
            <span>Password Root</span>
          </button>
          <button
            class="btn btn-outline"
            onclick={() => (showWebGuiModal = true)}
            title="Kelola akses web GUI database (phpMyAdmin / Adminer)"
          >
            <i class="ri-external-link-line" aria-hidden="true"></i>
            <span>Web GUI</span>
          </button>
          <button
            class="btn btn-outline"
            onclick={() => {
              fetchEngines();
              fetchDatabases(activeEngine);
              notify('Status database disinkronkan', 'success');
            }}
          >
            <i class="ri-refresh-line" aria-hidden="true"></i>
            <span>Sinkronisasi</span>
          </button>
        </div>

        <div class="btn-group-right">
          <button
            class="engine-status-badge is-clickable"
            onclick={() => openEngineManager('service')}
            title="Kelola status layanan, alih versi, dan berkas konfigurasi"
          >
            <span class="status-indicator-dot"></span>
            <span>{currentEngine.display_name} {currentEngine.version || 'Aktif'}</span>
            <i class="ri-play-fill badge-play-icon" aria-hidden="true"></i>
          </button>

          <div class="search-box">
            <i class="ri-search-line search-icon" aria-hidden="true"></i>
            <input
              type="text"
              class="search-input"
              placeholder="Cari database..."
              bind:value={searchQuery}
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Tabel Database -->
    <div class="table-card">
      {#if loadingDatabases}
        <div class="table-loading">
          <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
          <span>Memuat data database...</span>
        </div>
      {:else if filteredDatabases.length === 0}
        <div class="table-empty">
          <i class="ri-database-2-line empty-icon" aria-hidden="true"></i>
          <p class="empty-title">Belum ada database pada {currentEngine.display_name}</p>
          <p class="empty-sub">Klik tombol "Tambah DB" di atas untuk membuat database baru.</p>
        </div>
      {:else}
        <div class="table-responsive">
          <table class="data-table">
            <thead>
              <tr>
                <th style="width: 40px;"><input type="checkbox" aria-label="Pilih semua" /></th>
                <th>Nama Database</th>
                <th>Username</th>
                <th>Password</th>
                <th>Ukuran</th>
                <th>Backup</th>
                <th>Lokasi</th>
                <th>Catatan</th>
                <th style="text-align: right;">Operasi</th>
              </tr>
            </thead>
            <tbody>
              {#each filteredDatabases as db (db.id)}
                <tr>
                  <td><input type="checkbox" aria-label={`Pilih ${db.name}`} /></td>
                  <td class="col-name">
                    <strong>{db.name}</strong>
                  </td>
                  <td class="col-mono">{db.username}</td>
                  <td class="col-mono col-pass">
                    {#if visiblePasswords[db.id]}
                      <span>{db.password}</span>
                    {:else}
                      <span>••••••••••••</span>
                    {/if}
                    <button
                      class="btn-icon"
                      onclick={() => (visiblePasswords[db.id] = !visiblePasswords[db.id])}
                      title={visiblePasswords[db.id] ? 'Sembunyikan' : 'Lihat'}
                    >
                      <i class={visiblePasswords[db.id] ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
                    </button>
                    <button
                      class="btn-icon"
                      onclick={() => copyText(db.password, 'Password')}
                      title="Salin password"
                    >
                      <i class="ri-file-copy-line" aria-hidden="true"></i>
                    </button>
                  </td>
                  <td>{formatBytes(db.size_bytes)}</td>
                  <td>
                    {#if db.backup_count > 0}
                      <span class="badge-backup exists">Ada ({db.backup_count})</span>
                    {:else}
                      <span class="badge-backup empty">Belum ada</span>
                    {/if}
                  </td>
                  <td>{db.location}</td>
                  <td class="col-note">{db.note || '-'}</td>
                  <td class="col-actions">
                    <button
                      class="action-link"
                      onclick={() => openQuotaModal(db)}
                      title="Atur kuota kapasitas database"
                    >
                      Kuota
                    </button>
                    <button
                      class="action-link"
                      onclick={() => (showWebGuiModal = true)}
                      title="Buka phpMyAdmin Web GUI"
                    >
                      Web GUI
                    </button>
                    <button
                      class="action-link danger"
                      onclick={() => deleteDb(db)}
                      title="Hapus database ini"
                    >
                      Hapus
                    </button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}
</div>

<!-- Modal Quick Install (ala aaPanel) -->
{#if showInstallModal}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget && !installing) showInstallModal = false; }}
  >
    <div class="modal-card" role="dialog" aria-modal="true" tabindex="-1">
      <div class="modal-header">
        <div class="modal-title-with-icon">
          <i class="{getEngineIcon(activeEngine)} engine-install-icon" aria-hidden="true"></i>
          <div>
            <h3 class="modal-title">Instalasi {currentEngine.display_name}</h3>
            <p class="modal-subtitle">Pasang paket engine database ke server</p>
          </div>
        </div>
        <button
          class="modal-close-btn"
          disabled={installing}
          onclick={() => (showInstallModal = false)}
          aria-label="Tutup"
        >
          <i class="ri-close-line" aria-hidden="true"></i>
        </button>
      </div>

      <div class="modal-body">
        <div class="form-group">
          <label for="install-ver" class="form-label">Pilih Versi</label>
          <select id="install-ver" class="form-select" bind:value={installVersion} disabled={installing}>
            {#if currentEngine.versions && currentEngine.versions.length > 0}
              {#each currentEngine.versions as v}
                <option value={v.version}>{v.label}</option>
              {/each}
            {:else}
              {#each currentEngine.supported_versions as ver}
                <option value={ver}>{ver}</option>
              {/each}
            {/if}
          </select>
        </div>

        <div class="form-group">
          <label for="install-mode" class="form-label">Metode Instalasi</label>
          <select id="install-mode" class="form-select" bind:value={installMode} disabled={installing}>
            <option value="native">Native Package (APT / Systemd Linux)</option>
            <option value="docker">Docker Container (Terisolasi & Mandiri)</option>
          </select>
        </div>

        <div class="plugin-desc-box">
          <p class="plugin-desc-title">Petunjuk Instalasi:</p>
          <ul class="plugin-desc-list">
            <li>Jika paket ini telah ada, berkas biner lama akan diperbarui secara aman.</li>
            <li>Port default ({currentEngine.port}) akan dikonfigurasi pada localhost (127.0.0.1).</li>
            <li>Proses instalasi membutuhkan waktu 1-3 menit, silakan tunggu hingga selesai.</li>
          </ul>
        </div>
      </div>

      <div class="modal-footer">
        <button
          class="btn btn-outline"
          disabled={installing}
          onclick={() => (showInstallModal = false)}
        >
          Batal
        </button>
        <button class="btn btn-primary" disabled={installing} onclick={submitInstall}>
          {#if installing}
            <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
            <span>Memulai...</span>
          {:else}
            <i class="ri-play-fill" aria-hidden="true"></i>
            <span>Mulai Instalasi (Install Now)</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal Tambah Database -->
{#if showAddModal}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget && !creatingDb) showAddModal = false; }}
  >
    <div class="modal-card" role="dialog" aria-modal="true" tabindex="-1">
      <div class="modal-header">
        <h3 class="modal-title">Tambah Database {currentEngine.display_name}</h3>
        <button
          class="modal-close-btn"
          disabled={creatingDb}
          onclick={() => (showAddModal = false)}
          aria-label="Tutup"
        >
          <i class="ri-close-line" aria-hidden="true"></i>
        </button>
      </div>

      <div class="modal-body">
        <!-- DB Name & Charset (Screenshot 2 aaPanel) -->
        <div class="form-group">
          <label for="db-name" class="form-label">Nama Database & Charset</label>
          <div class="input-with-select">
            <input
              id="db-name"
              type="text"
              class="form-input flex-1"
              placeholder="Nama database baru"
              value={dbName}
              oninput={(e) => onDbNameInput((e.target as HTMLInputElement).value)}
              disabled={creatingDb}
            />
            <select class="form-select charset-select" bind:value={dbCharset} disabled={creatingDb}>
              <option value="utf8mb4">utf8mb4 (Rekomendasi)</option>
              <option value="utf8">utf8</option>
              <option value="latin1">latin1</option>
              <option value="gbk">gbk</option>
            </select>
          </div>
        </div>

        <!-- Username -->
        <div class="form-group">
          <label for="db-user" class="form-label">Username Pengguna</label>
          <input
            id="db-user"
            type="text"
            class="form-input"
            placeholder="Username database"
            bind:value={dbUser}
            disabled={creatingDb}
          />
        </div>

        <!-- Password dengan tombol Refresh acak di samping input -->
        <div class="form-group">
          <label for="db-pass" class="form-label">Password</label>
          <div class="input-with-button">
            <input
              id="db-pass"
              type="text"
              class="form-input"
              placeholder="Kata sandi akses"
              bind:value={dbPass}
              disabled={creatingDb}
            />
            <button
              type="button"
              class="btn btn-outline btn-refresh-pass"
              onclick={generatePassword}
              disabled={creatingDb}
              title="Acak ulang password otomatis"
            >
              <i class="ri-refresh-line" aria-hidden="true"></i>
            </button>
          </div>
        </div>

        <!-- Hak Akses (Permission) -->
        <div class="form-group">
          <label for="db-perm" class="form-label">Hak Akses (Permission)</label>
          <select id="db-perm" class="form-select" bind:value={dbPermission} disabled={creatingDb}>
            <option value="localhost">Local server (127.0.0.1)</option>
            <option value="%">Semua Host (Public / %)</option>
            <option value="custom">IP Tertentu (Spesifik)</option>
          </select>
          {#if dbPermission === 'custom'}
            <input
              type="text"
              class="form-input mt-2"
              placeholder="Masukkan alamat IP (mis. 192.168.1.100)"
              bind:value={customIp}
              disabled={creatingDb}
            />
          {/if}
        </div>

        <!-- Add to Server Lokasi (LocalServer) -->
        <div class="form-group">
          <div class="add-to-header">
            <label for="add-to-select" class="form-label">Tambahkan Ke (Add to)</label>
            <span class="remote-servers-link">Remote servers</span>
          </div>
          <select id="add-to-select" class="form-select" disabled>
            <option value="local">LocalServer (127.0.0.1)</option>
          </select>
        </div>

        <div class="form-group">
          <label for="db-note" class="form-label">Catatan (Opsional)</label>
          <input
            id="db-note"
            type="text"
            class="form-input"
            placeholder="Keterangan singkat database ini"
            bind:value={dbNote}
            disabled={creatingDb}
          />
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-outline" disabled={creatingDb} onclick={() => (showAddModal = false)}>
          Batal
        </button>
        <button class="btn btn-primary" disabled={creatingDb} onclick={submitAddDatabase}>
          {#if creatingDb}
            <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
            <span>Menyimpan...</span>
          {:else}
            <span>Konfirmasi Buat DB</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal Database Engine Manager (Status, Ganti Versi, Konfigurasi my.cnf, Log Error) -->
{#if showEngineManager}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget && !switchingVersion) showEngineManager = false; }}
  >
    <div class="modal-card engine-manager-modal" role="dialog" aria-modal="true" tabindex="-1">
      <div class="modal-header">
        <div class="modal-title-with-icon">
          <i class="{getEngineIcon(activeEngine)} engine-install-icon" aria-hidden="true"></i>
          <div>
            <h3 class="modal-title">{currentEngine.display_name} Manager</h3>
            <p class="modal-subtitle">Kelola status layanan, alih versi, berkas konfigurasi, dan log</p>
          </div>
        </div>
        <button
          class="modal-close-btn"
          disabled={switchingVersion}
          onclick={() => (showEngineManager = false)}
          aria-label="Tutup"
        >
          <i class="ri-close-line" aria-hidden="true"></i>
        </button>
      </div>

      <!-- Navigasi Tab Internal Engine Manager -->
      <div class="engine-manager-tabs">
        <button
          class="em-tab-btn"
          class:active={engineTab === 'service'}
          onclick={() => (engineTab = 'service')}
        >
          <i class="ri-dashboard-line" aria-hidden="true"></i>
          <span>Layanan (Service)</span>
        </button>
        <button
          class="em-tab-btn"
          class:active={engineTab === 'switch_version'}
          onclick={() => (engineTab = 'switch_version')}
        >
          <i class="ri-shuffle-line" aria-hidden="true"></i>
          <span>Ganti Versi</span>
        </button>
        <button
          class="em-tab-btn"
          class:active={engineTab === 'config'}
          onclick={() => {
            engineTab = 'config';
            fetchEngineConfig();
          }}
        >
          <i class="ri-file-settings-line" aria-hidden="true"></i>
          <span>Konfigurasi (my.cnf)</span>
        </button>
        <button
          class="em-tab-btn"
          class:active={engineTab === 'logs'}
          onclick={() => {
            engineTab = 'logs';
            fetchEngineLogs();
          }}
        >
          <i class="ri-file-list-3-line" aria-hidden="true"></i>
          <span>Log Galat</span>
        </button>
      </div>

      <div class="modal-body">
        {#if engineTab === 'service'}
          <!-- Tab 1: Status Layanan -->
          <div class="service-panel">
            <div class="service-status-box">
              <div class="status-indicator-large">
                <span class="status-dot-large {currentEngine.running ? 'online' : 'offline'}"></span>
                <div>
                  <h4 class="service-status-heading">
                    Status: {currentEngine.running ? 'Aktif (Running)' : 'Berhenti (Stopped)'}
                  </h4>
                  <p class="service-status-sub">
                    Unit: <code>{currentEngine.service_unit || activeEngine + '.service'}</code> • Port: <code>{currentEngine.port}</code>
                  </p>
                </div>
              </div>
            </div>

            <div class="service-actions-grid">
              <button
                class="service-action-card"
                disabled={serviceActionLoading}
                onclick={() => handleServiceAction('restart')}
              >
                <i class="ri-restart-line action-icon" aria-hidden="true"></i>
                <div class="action-info">
                  <strong>Restart Layanan</strong>
                  <span>Mulai ulang proses daemon</span>
                </div>
              </button>

              {#if currentEngine.running}
                <button
                  class="service-action-card danger"
                  disabled={serviceActionLoading}
                  onclick={() => handleServiceAction('stop')}
                >
                  <i class="ri-stop-circle-line action-icon" aria-hidden="true"></i>
                  <div class="action-info">
                    <strong>Hentikan Layanan</strong>
                    <span>Stop daemon {currentEngine.display_name}</span>
                  </div>
                </button>
              {:else}
                <button
                  class="service-action-card success"
                  disabled={serviceActionLoading}
                  onclick={() => handleServiceAction('start')}
                >
                  <i class="ri-play-circle-line action-icon" aria-hidden="true"></i>
                  <div class="action-info">
                    <strong>Nyalakan Layanan</strong>
                    <span>Start daemon {currentEngine.display_name}</span>
                  </div>
                </button>
              {/if}

              <button
                class="service-action-card"
                disabled={serviceActionLoading}
                onclick={() => handleServiceAction('reload')}
              >
                <i class="ri-refresh-line action-icon" aria-hidden="true"></i>
                <div class="action-info">
                  <strong>Reload Konfigurasi</strong>
                  <span>Terapkan konfigurasi tanpa downtime</span>
                </div>
              </button>
            </div>
          </div>
        {:else if engineTab === 'switch_version'}
          <!-- Tab 2: Ganti Versi (Switch Version) -->
          <div class="switch-version-panel">
            <div class="version-warning-banner">
              <i class="ri-alert-fill warning-icon" aria-hidden="true"></i>
              <div>
                <strong>Perhatian Arsitektur SQL:</strong>
                <p>
                  Satu server host Linux hanya dapat menjalankan 1 versi aktif native pada port 3306.
                  Mengganti versi secara langsung tanpa backup dapat menyebabkan ketidakcocokan format direktori data InnoDB.
                  FikPanel akan mengekspor backup seluruh database secara otomatis sebelum memasang biner versi target.
                </p>
              </div>
            </div>

            <div class="form-group mt-3">
              <label for="current-ver-info" class="form-label">Versi Saat Ini yang Aktif</label>
              <div class="current-version-badge">
                <i class="ri-checkbox-circle-fill active-check" aria-hidden="true"></i>
                <span>{currentEngine.display_name} <strong>{currentEngine.version || '10.11'}</strong> (Sedang Berjalan)</span>
              </div>
            </div>

            <div class="form-group">
              <label for="target-ver-select" class="form-label">Pilih Versi Target yang Ingin Dituju</label>
              <select id="target-ver-select" class="form-select" bind:value={switchTargetVersion} disabled={switchingVersion}>
                {#if currentEngine.versions && currentEngine.versions.length > 0}
                  {#each currentEngine.versions as v}
                    <option value={v.version}>
                      {v.label} {v.version === currentEngine.version ? '(Versi Saat Ini)' : ''}
                    </option>
                  {/each}
                {:else}
                  {#each currentEngine.supported_versions as sv}
                    <option value={sv}>{currentEngine.display_name} {sv}</option>
                  {/each}
                {/if}
              </select>
            </div>

            <div class="checkbox-row mt-2">
              <label class="custom-checkbox-label">
                <input type="checkbox" bind:checked={switchBackupFirst} disabled={switchingVersion} />
                <span>
                  <strong>Buat cadangan snapshot otomatis seluruh database</strong> (mysqldump --all-databases) sebelum beralih versi (Sangat Direkomendasikan).
                </span>
              </label>
            </div>

            <div class="switch-action-row mt-4">
              <button
                class="btn btn-primary"
                disabled={switchingVersion || switchTargetVersion === currentEngine.version}
                onclick={submitSwitchVersion}
              >
                {#if switchingVersion}
                  <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
                  <span>Memproses Alih Versi...</span>
                {:else}
                  <i class="ri-shuffle-line" aria-hidden="true"></i>
                  <span>Beralih ke Versi Ini (Switch Version)</span>
                {/if}
              </button>
            </div>
          </div>
        {:else if engineTab === 'config'}
          <!-- Tab 3: Konfigurasi my.cnf -->
          <div class="config-panel">
            <div class="config-panel-header">
              <span class="config-path-badge">Berkas: <code>{engineConfigPath || '/etc/mysql/my.cnf'}</code></span>
              <button
                class="btn btn-outline btn-sm"
                disabled={loadingEngineConfig}
                onclick={fetchEngineConfig}
              >
                <i class="ri-refresh-line" aria-hidden="true"></i>
                <span>Muat Ulang</span>
              </button>
            </div>

            <textarea
              class="config-editor-textarea"
              bind:value={engineConfigContent}
              disabled={loadingEngineConfig || savingEngineConfig}
              spellcheck="false"
            ></textarea>

            <div class="config-footer-row">
              <span class="config-hint">Simpan perubahan dan restart layanan untuk menerapkan pengaturan baru.</span>
              <button
                class="btn btn-primary"
                disabled={savingEngineConfig || loadingEngineConfig}
                onclick={saveEngineConfig}
              >
                {#if savingEngineConfig}
                  <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
                  <span>Menyimpan...</span>
                {:else}
                  <i class="ri-save-line" aria-hidden="true"></i>
                  <span>Simpan Konfigurasi</span>
                {/if}
              </button>
            </div>
          </div>
        {:else if engineTab === 'logs'}
          <!-- Tab 4: Log Galat -->
          <div class="logs-panel">
            <div class="config-panel-header">
              <span class="config-path-badge">Log Galat Terkini (Error Log)</span>
              <button
                class="btn btn-outline btn-sm"
                disabled={loadingEngineLogs}
                onclick={fetchEngineLogs}
              >
                <i class="ri-refresh-line" aria-hidden="true"></i>
                <span>Segarkan Log</span>
              </button>
            </div>
            <pre class="logs-terminal-viewer">{engineLogsContent || 'Tidak ada catatan log galat.'}</pre>
          </div>
        {/if}
      </div>

      <div class="modal-footer">
        <button
          class="btn btn-outline"
          disabled={switchingVersion}
          onclick={() => (showEngineManager = false)}
        >
          Tutup
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal Web GUI Database (phpMyAdmin / Adminer ala aaPanel Screenshot 4) -->
{#if showWebGuiModal}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget) showWebGuiModal = false; }}
  >
    <div class="modal-card" role="dialog" aria-modal="true" tabindex="-1">
      <div class="modal-header">
        <div class="modal-title-with-icon">
          <i class="ri-terminal-box-line engine-install-icon" aria-hidden="true"></i>
          <div>
            <h3 class="modal-title">Web GUI Database (phpMyAdmin / Adminer)</h3>
            <p class="modal-subtitle">Antarmuka visual manajemen database, tabel, dan eksekusi query SQL</p>
          </div>
        </div>
        <button
          class="modal-close-btn"
          onclick={() => (showWebGuiModal = false)}
          aria-label="Tutup"
        >
          <i class="ri-close-line" aria-hidden="true"></i>
        </button>
      </div>

      <div class="modal-body">
        <div class="webgui-info-card">
          <div class="webgui-badge-row">
            <span class="badge-status-online">● Layanan Aktif (Port 888)</span>
            <span class="badge-php-version">PHP 8.2 (FPM)</span>
          </div>
          <p class="webgui-desc">
            Pilih metode akses ke antarmuka web database Anda:
          </p>
        </div>

        <div class="webgui-cards-container">
          <!-- Opsi 1: Akses Bebas Sandi (Password-Free Access / SSO) -->
          <div class="webgui-choice-card highlight">
            <div class="choice-card-header">
              <span class="badge-new-sso">FITUR UTAMA</span>
              <h4 class="choice-card-title">Akses Bebas Sandi (One-Click SSO)</h4>
            </div>
            <p class="choice-card-desc">
              Masuk langsung ke antarmuka phpMyAdmin tanpa perlu mengetik ulang root password secara manual.
              Menggunakan token sesi panel yang aman dan otomatis kedaluwarsa.
            </p>
            <button
              class="btn btn-primary mt-2"
              onclick={() => {
                notify('Membuka phpMyAdmin dengan Akses Bebas Sandi (SSO)...', 'success');
                showWebGuiModal = false;
              }}
            >
              <i class="ri-external-link-line" aria-hidden="true"></i>
              <span>Buka Akses Bebas Sandi</span>
            </button>
          </div>

          <!-- Opsi 2: Akses Publik (Public Access) -->
          <div class="webgui-choice-card">
            <div class="choice-card-header">
              <h4 class="choice-card-title">Akses Publik (Public Access)</h4>
              <label class="toggle-switch">
                <input type="checkbox" bind:checked={publicAccessEnabled} />
                <span class="slider"></span>
              </label>
            </div>
            <p class="choice-card-desc">
              Membuka port phpMyAdmin ke internet publik untuk diakses via URL domain/IP.
              Matikan akses publik bila tidak digunakan untuk meningkatkan keamanan server dari serangan brute-force.
            </p>
            {#if publicAccessEnabled}
              <button
                class="btn btn-outline mt-2"
                onclick={() => {
                  notify('Membuka URL Publik phpMyAdmin...', 'info');
                  showWebGuiModal = false;
                }}
              >
                <i class="ri-global-line" aria-hidden="true"></i>
                <span>Buka URL Publik</span>
              </button>
            {/if}
          </div>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-outline" onclick={() => (showWebGuiModal = false)}>
          Tutup
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal Atur Kuota Kapasitas Database (Screenshot 3 aaPanel) -->
{#if showQuotaModal && quotaTargetDb}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget && !savingQuota) showQuotaModal = false; }}
  >
    <div class="modal-card" role="dialog" aria-modal="true" tabindex="-1">
      <div class="modal-header">
        <h3 class="modal-title">Atur Kuota Baru [{quotaTargetDb.name}]</h3>
        <button
          class="modal-close-btn"
          disabled={savingQuota}
          onclick={() => (showQuotaModal = false)}
          aria-label="Tutup"
        >
          <i class="ri-close-line" aria-hidden="true"></i>
        </button>
      </div>

      <div class="modal-body">
        <div class="form-group">
          <label for="quota-used-info" class="form-label">Kapasitas Terpakai Saat Ini</label>
          <div class="input-with-unit">
            <input
              id="quota-used-info"
              type="text"
              class="form-input"
              value={formatBytes(quotaTargetDb.size_bytes)}
              disabled
            />
          </div>
        </div>

        <div class="form-group">
          <label for="quota-val-input" class="form-label">Kapasitas Batas Kuota (Quota)</label>
          <div class="input-with-unit">
            <input
              id="quota-val-input"
              type="number"
              class="form-input"
              min="0"
              bind:value={quotaMB}
              disabled={savingQuota}
            />
            <span class="unit-badge">MB</span>
          </div>
        </div>

        <div class="quota-reminder-box">
          <p class="reminder-line">
            • <strong>Catatan:</strong> Fitur pembatasan kuota kapasitas database menggunakan kuota sistem file OS (XFS / ext4 prjquota).
          </p>
          <p class="reminder-line">
            • Untuk menonaktifkan atau membatalkan batasan kuota, isi nilai dengan <strong>"0"</strong>.
          </p>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-outline" disabled={savingQuota} onclick={() => (showQuotaModal = false)}>
          Batal
        </button>
        <button class="btn btn-primary" disabled={savingQuota} onclick={saveQuota}>
          Konfirmasi Kuota
        </button>
      </div>
    </div>
  </div>
{/if}


<!-- Modal Password Root -->
{#if showRootPassModal}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget) showRootPassModal = false; }}
  >
    <div class="modal-card" role="dialog" aria-modal="true" tabindex="-1">
      <div class="modal-header">
        <h3 class="modal-title">Password Root / Administrator</h3>
        <button
          class="modal-close-btn"
          onclick={() => (showRootPassModal = false)}
          aria-label="Tutup"
        >
          <i class="ri-close-line" aria-hidden="true"></i>
        </button>
      </div>

      <div class="modal-body">
        <p class="modal-subtitle">
          Kredensial root administrator digunakan untuk menghubungkan panel dengan layanan engine {currentEngine.display_name}.
        </p>

        <div class="form-group">
          <label for="root-pass" class="form-label">Password Root Saat Ini</label>
          <div class="input-with-button">
            <input
              id="root-pass"
              type={showRootPass ? 'text' : 'password'}
              class="form-input"
              bind:value={rootPassword}
            />
            <button
              type="button"
              class="btn btn-outline"
              aria-label={showRootPass ? 'Sembunyikan password' : 'Lihat password'}
              onclick={() => (showRootPass = !showRootPass)}
            >
              <i class={showRootPass ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
            </button>
            <button
              type="button"
              class="btn btn-outline"
              onclick={() => copyText(rootPassword, 'Password root')}
            >
              Salin
            </button>
          </div>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-outline" onclick={() => (showRootPassModal = false)}>
          Tutup
        </button>
        <button
          class="btn btn-primary"
          onclick={() => {
            notify('Password root berhasil diperbarui', 'success');
            showRootPassModal = false;
          }}
        >
          Simpan Perubahan
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Drawer / Modal Messages Box & Real-Time Log Execution (ala aaPanel) -->
{#if showTasksDrawer}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => { if (e.target === e.currentTarget) showTasksDrawer = false; }}
  >
    <div class="messages-box-modal" role="dialog" aria-modal="true" tabindex="-1">
      <div class="messages-header">
        <div class="messages-title">
          <i class="ri-terminal-window-fill" aria-hidden="true"></i>
          <span>Messages Box / Antrean Eksekusi</span>
        </div>
        <button
          class="modal-close-btn"
          onclick={() => (showTasksDrawer = false)}
          aria-label="Tutup"
        >
          <i class="ri-close-line" aria-hidden="true"></i>
        </button>
      </div>

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
              {#each streamLogs as line, i}
                <div class="terminal-line">{line}</div>
              {/each}
            {/if}
          </div>
        </main>
      </div>
    </div>
  </div>
{/if}

<style>
  .databases-view {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
    padding: 1.5rem;
  }

  /* Header Tabs Bar */
  .engine-tabs-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid var(--border-color);
    padding-bottom: 0.5rem;
  }

  .engine-tabs {
    display: flex;
    gap: 0.5rem;
  }

  .engine-tab {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.625rem 1.125rem;
    background: transparent;
    border: none;
    border-radius: var(--radius-md);
    color: var(--text-muted);
    font-size: 0.9375rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .engine-tab:hover {
    color: var(--text-primary);
    background: var(--bg-hover);
  }

  .engine-tab.active {
    color: var(--accent);
    background: var(--bg-surface);
    box-shadow: var(--shadow-sm);
  }

  .tab-icon {
    font-size: 1.125rem;
  }

  .engine-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }

  .engine-dot.active {
    background: var(--accent);
    box-shadow: 0 0 6px var(--accent);
  }

  .engine-dot.uninstalled {
    background: var(--danger);
  }

  .btn-task-queue {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.875rem;
    border-radius: var(--radius-md);
    background: var(--bg-surface);
    border: 1px solid var(--border);
    color: var(--text);
    font-size: 0.875rem;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .btn-task-queue:hover {
    background: var(--bg-hover);
    border-color: var(--accent);
  }

  .badge-running-count {
    background: var(--accent);
    color: var(--text-on-accent);
    font-size: 0.75rem;
    font-weight: 700;
    padding: 0.125rem 0.375rem;
    border-radius: 999px;
  }

  /* Uninstalled State Card (aaPanel Screenshot 2) */
  .uninstalled-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 2.5rem 2rem;
    margin: 1.5rem 0;
  }

  .uninstalled-card__body {
    display: flex;
    gap: 1.5rem;
    align-items: flex-start;
  }

  .alert-icon-box {
    font-size: 2.5rem;
    color: var(--warning);
    line-height: 1;
  }

  .uninstalled-title {
    font-size: 1.25rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 0.5rem 0;
  }

  .uninstalled-desc {
    color: var(--text-muted);
    font-size: 0.9375rem;
    line-height: 1.5;
    margin: 0 0 1.25rem 0;
    max-width: 650px;
  }

  .uninstalled-actions {
    display: flex;
    gap: 0.75rem;
  }

  /* Action Bar Card (aaPanel Screenshot 1) */
  .action-bar-card {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    background: var(--bg-surface);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    padding: 1rem 1.25rem;
  }

  .auto-backup-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding-bottom: 0.75rem;
    border-bottom: 1px solid var(--border-color);
  }

  .auto-backup-label {
    font-size: 0.875rem;
    color: var(--text-muted);
  }

  .auto-backup-label strong {
    color: var(--text-primary);
  }

  /* Toggle Switch */
  .toggle-switch {
    position: relative;
    display: inline-block;
    width: 36px;
    height: 20px;
  }

  .toggle-switch input {
    opacity: 0;
    width: 0;
    height: 0;
  }

  .slider {
    position: absolute;
    cursor: pointer;
    inset: 0;
    background-color: var(--bg-canvas);
    border: 1px solid var(--border-color);
    transition: 0.3s;
    border-radius: 20px;
  }

  .slider:before {
    position: absolute;
    content: '';
    height: 14px;
    width: 14px;
    left: 2px;
    bottom: 2px;
    background-color: var(--text-muted);
    transition: 0.3s;
    border-radius: 50%;
  }

  input:checked + .slider {
    background-color: var(--accent);
    border-color: var(--accent);
  }

  input:checked + .slider:before {
    transform: translateX(16px);
    background-color: var(--text-on-accent);
  }

  .action-buttons-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.75rem;
  }

  .btn-group-left {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .btn-group-right {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .engine-status-badge {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.375rem 0.75rem;
    background: var(--bg-canvas);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--text-primary);
  }

  .status-indicator-dot {
    width: 7px;
    height: 7px;
    background: var(--status-online);
    border-radius: 50%;
    box-shadow: 0 0 5px var(--status-online);
  }

  .search-box {
    position: relative;
    display: flex;
    align-items: center;
  }

  .search-icon {
    position: absolute;
    left: 0.625rem;
    color: var(--text-muted);
  }

  .search-input {
    background: var(--bg-canvas);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    color: var(--text-primary);
    padding: 0.4375rem 0.625rem 0.4375rem 2rem;
    font-size: 0.875rem;
  }

  .search-input:focus {
    outline: none;
    border-color: var(--accent);
  }

  /* Tabel Data */
  .table-card {
    background: var(--bg-surface);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    overflow: hidden;
  }

  .table-responsive {
    overflow-x: auto;
  }

  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.875rem;
  }

  .data-table th {
    background: var(--bg-canvas);
    color: var(--text-muted);
    font-weight: 500;
    text-align: left;
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--border-color);
  }

  .data-table td {
    padding: 0.875rem 1rem;
    border-bottom: 1px solid var(--border-color);
    color: var(--text-primary);
  }

  .col-mono {
    font-family: var(--font-mono, monospace);
    font-size: 0.8125rem;
  }

  .col-pass {
    display: flex;
    align-items: center;
    gap: 0.375rem;
  }

  .btn-icon {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0.125rem 0.25rem;
  }

  .btn-icon:hover {
    color: var(--text-primary);
  }

  .badge-backup {
    font-size: 0.75rem;
    padding: 0.125rem 0.375rem;
    border-radius: 4px;
  }

  .badge-backup.exists {
    background: var(--tint-accent);
    color: var(--accent);
  }

  .badge-backup.empty {
    background: var(--bg-hover);
    color: var(--text-muted);
  }

  .action-link {
    background: transparent;
    border: none;
    color: var(--accent);
    cursor: pointer;
    font-size: 0.8125rem;
    padding: 0 0.25rem;
  }

  .action-link:hover {
    text-decoration: underline;
  }

  .action-link.danger {
    color: var(--status-offline);
  }

  .table-empty,
  .table-loading {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 4rem 2rem;
    color: var(--text-muted);
    gap: 0.75rem;
  }

  .empty-icon {
    font-size: 3rem;
  }

  .spinner {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  /* Tombol Global */
  .btn {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.5rem 1rem;
    border-radius: var(--radius-md);
    font-size: 0.875rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .btn-primary {
    background: var(--accent);
    color: var(--text-on-accent);
    border: 1px solid var(--accent);
  }

  .btn-primary:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .btn-outline {
    background: transparent;
    border: 1px solid var(--border-color);
    color: var(--text-primary);
  }

  .btn-outline:hover:not(:disabled) {
    background: var(--bg-hover);
    border-color: var(--text-muted);
  }

  .btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  /* Modals */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: var(--bg-overlay);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: var(--z-modal);
  }

  .modal-card {
    background: var(--bg-surface);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-lg);
    width: 100%;
    max-width: 520px;
    box-shadow: var(--shadow-lg);
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1.25rem 1.5rem;
    border-bottom: 1px solid var(--border-color);
  }

  .modal-title-with-icon {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .engine-install-icon {
    font-size: 2rem;
    color: var(--accent);
  }

  .modal-title {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .modal-subtitle {
    font-size: 0.8125rem;
    color: var(--text-muted);
    margin: 0;
  }

  .modal-close-btn {
    background: transparent;
    border: none;
    font-size: 1.25rem;
    color: var(--text-muted);
    cursor: pointer;
  }

  .modal-close-btn:hover {
    color: var(--text-primary);
  }

  .modal-body {
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
    padding: 1rem 1.5rem;
    border-top: 1px solid var(--border-color);
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 0.375rem;
  }

  .form-label {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--text-primary);
  }

  .form-input,
  .form-select {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 0.5rem 0.75rem;
    color: var(--text);
    font-size: 0.875rem;
    color-scheme: dark;
  }

  .form-select option {
    color-scheme: dark;
    background-color: var(--bg-surface);
    color: var(--text);
    padding: 0.5rem 0.75rem;
  }

  .form-select option:checked {
    background-color: var(--bg-active);
    color: var(--accent);
  }

  .form-input:focus,
  .form-select:focus {
    outline: none;
    border-color: var(--accent);
  }

  .input-with-button {
    display: flex;
    gap: 0.5rem;
  }

  .input-with-button .form-input {
    flex: 1;
  }

  .plugin-desc-box {
    background: var(--bg-canvas);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    padding: 0.875rem 1rem;
    font-size: 0.8125rem;
    color: var(--text-muted);
  }

  .plugin-desc-title {
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 0.5rem 0;
  }

  .plugin-desc-list {
    margin: 0;
    padding-left: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  /* Messages Box Modal (aaPanel Screenshot 4) */
  .messages-box-modal {
    background: var(--bg-surface);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-lg);
    width: 90%;
    max-width: 860px;
    height: 540px;
    display: flex;
    flex-direction: column;
    box-shadow: var(--shadow-lg);
    overflow: hidden;
  }

  .messages-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.875rem 1.25rem;
    background: var(--bg-canvas);
    border-bottom: 1px solid var(--border-color);
  }

  .messages-title {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-weight: 600;
    color: var(--text-primary);
  }

  .messages-body {
    display: flex;
    flex: 1;
    overflow: hidden;
  }

  .messages-sidebar {
    width: 220px;
    background: var(--bg-surface);
    border-right: 1px solid var(--border-color);
    display: flex;
    flex-direction: column;
  }

  .task-list-title {
    padding: 0.75rem 1rem;
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--text-muted);
    border-bottom: 1px solid var(--border-color);
  }

  .task-items-list {
    flex: 1;
    overflow-y: auto;
  }

  .task-item {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
    padding: 0.75rem 1rem;
    background: transparent;
    border: none;
    border-bottom: 1px solid var(--border-color);
    text-align: left;
    cursor: pointer;
    transition: background 0.2s;
  }

  .task-item:hover {
    background: var(--bg-hover);
  }

  .task-item.active {
    background: var(--bg-canvas);
  }

  .task-status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--text-muted);
  }

  .task-status-dot.running {
    background: var(--status-online);
    box-shadow: 0 0 6px var(--status-online);
  }

  .task-status-dot.success {
    background: var(--status-online);
  }

  .task-status-dot.failed {
    background: var(--status-offline);
  }

  .task-item-name {
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--text-primary);
  }

  .task-item-status {
    font-size: 0.6875rem;
    color: var(--text-muted);
    text-transform: capitalize;
  }

  .messages-log-area {
    flex: 1;
    display: flex;
    flex-direction: column;
    background: var(--bg-inset);
  }

  .log-area-topbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.625rem 1rem;
    background: var(--bg-canvas);
    border-bottom: 1px solid var(--border-color);
    font-size: 0.8125rem;
  }

  .log-target-label {
    color: var(--text-primary);
  }

  .log-topbar-actions {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .log-tip {
    font-size: 0.75rem;
    color: var(--text-muted);
  }

  .btn-refresh-task {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    background: transparent;
    border: 1px solid var(--border-color);
    border-radius: 4px;
    color: var(--text-primary);
    padding: 0.25rem 0.5rem;
    font-size: 0.75rem;
    cursor: pointer;
  }

  .btn-refresh-task:hover {
    background: var(--bg-hover);
  }

  .terminal-container {
    flex: 1;
    padding: 1rem;
    overflow-y: auto;
    font-family: var(--font-mono, monospace);
    font-size: 0.8125rem;
    color: var(--text-secondary);
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .terminal-waiting {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    color: var(--text-muted);
  }

  .terminal-line {
    line-height: 1.4;
  }

  /* Clickable Engine Status Badge (ala aaPanel) */
  .engine-status-badge.is-clickable {
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .engine-status-badge.is-clickable:hover {
    background: var(--bg-hover);
    border-color: var(--accent);
  }

  .badge-play-icon {
    font-size: 0.75rem;
    color: var(--accent);
    margin-left: 0.125rem;
  }

  /* Form Tambah DB Input with Select / Refresh */
  .input-with-select {
    display: flex;
    gap: 0.5rem;
  }

  .input-with-select .flex-1 {
    flex: 1;
  }

  .charset-select {
    width: 170px;
    flex-shrink: 0;
  }

  .btn-refresh-pass {
    padding: 0.5rem 0.75rem;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .add-to-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .remote-servers-link {
    font-size: 0.8125rem;
    color: var(--accent);
    cursor: pointer;
  }

  .remote-servers-link:hover {
    text-decoration: underline;
  }

  /* Engine Manager Modal */
  .engine-manager-modal {
    max-width: 720px;
  }

  .engine-manager-tabs {
    display: flex;
    border-bottom: 1px solid var(--border-color);
    background: var(--bg-canvas);
    padding: 0 1rem;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .em-tab-btn {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.75rem 1rem;
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    color: var(--text-muted);
    font-size: 0.875rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .em-tab-btn:hover {
    color: var(--text-primary);
  }

  .em-tab-btn.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
    font-weight: 600;
  }

  /* Service Tab */
  .service-panel {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  .service-status-box {
    background: var(--bg-canvas);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    padding: 1.25rem;
  }

  .status-indicator-large {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .status-dot-large {
    width: 14px;
    height: 14px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .status-dot-large.online {
    background: var(--status-online);
    box-shadow: 0 0 10px var(--status-online);
  }

  .status-dot-large.offline {
    background: var(--status-offline);
    box-shadow: 0 0 10px var(--status-offline);
  }

  .service-status-heading {
    margin: 0 0 0.25rem 0;
    font-size: 1rem;
    font-weight: 600;
    color: var(--text-primary);
  }

  .service-status-sub {
    margin: 0;
    font-size: 0.8125rem;
    color: var(--text-muted);
  }

  .service-actions-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 0.75rem;
  }

  .service-action-card {
    display: flex;
    align-items: center;
    gap: 0.875rem;
    padding: 1rem;
    background: var(--bg-canvas);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    cursor: pointer;
    text-align: left;
    transition: all 0.2s ease;
    color: var(--text-primary);
  }

  .service-action-card:hover:not(:disabled) {
    background: var(--bg-hover);
    border-color: var(--accent);
  }

  .service-action-card.danger:hover:not(:disabled) {
    border-color: var(--status-offline);
  }

  .service-action-card.success:hover:not(:disabled) {
    border-color: var(--status-online);
  }

  .service-action-card .action-icon {
    font-size: 1.5rem;
    color: var(--accent);
  }

  .service-action-card.danger .action-icon {
    color: var(--status-offline);
  }

  .service-action-card.success .action-icon {
    color: var(--status-online);
  }

  .action-info {
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
  }

  .action-info strong {
    font-size: 0.875rem;
  }

  .action-info span {
    font-size: 0.75rem;
    color: var(--text-muted);
  }

  /* Switch Version Tab */
  .switch-version-panel {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .version-warning-banner {
    display: flex;
    gap: 0.875rem;
    padding: 1rem;
    background: var(--bg-inset);
    border-left: 4px solid var(--warning);
    border-radius: var(--radius-md);
    font-size: 0.8125rem;
    line-height: 1.5;
    color: var(--text-primary);
  }

  .version-warning-banner .warning-icon {
    font-size: 1.375rem;
    color: var(--warning);
    flex-shrink: 0;
  }

  .version-warning-banner p {
    margin: 0.25rem 0 0 0;
    color: var(--text-muted);
  }

  .current-version-badge {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.625rem 0.875rem;
    background: var(--bg-canvas);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    font-size: 0.875rem;
    color: var(--text-primary);
  }

  .active-check {
    color: var(--status-online);
    font-size: 1.125rem;
  }

  .custom-checkbox-label {
    display: flex;
    align-items: flex-start;
    gap: 0.5rem;
    font-size: 0.8125rem;
    color: var(--text-muted);
    cursor: pointer;
  }

  .custom-checkbox-label input {
    margin-top: 0.125rem;
  }

  .custom-checkbox-label strong {
    color: var(--text-primary);
  }

  /* Config & Logs */
  .config-panel,
  .logs-panel {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .config-panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .config-path-badge {
    font-size: 0.8125rem;
    color: var(--text-muted);
  }

  .config-path-badge code {
    color: var(--accent);
    background: var(--bg-canvas);
    padding: 0.125rem 0.375rem;
    border-radius: 4px;
  }

  .config-editor-textarea {
    width: 100%;
    height: 280px;
    background: var(--bg-inset);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    padding: 0.75rem 1rem;
    font-family: var(--font-mono, monospace);
    font-size: 0.8125rem;
    color: var(--text-primary);
    line-height: 1.5;
    resize: vertical;
    white-space: pre;
  }

  .config-editor-textarea:focus {
    outline: none;
    border-color: var(--accent);
  }

  .config-footer-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
  }

  .config-hint {
    font-size: 0.75rem;
    color: var(--text-muted);
  }

  .logs-terminal-viewer {
    background: var(--bg-inset);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    padding: 1rem;
    font-family: var(--font-mono, monospace);
    font-size: 0.8125rem;
    color: var(--text-secondary);
    line-height: 1.5;
    height: 280px;
    overflow-y: auto;
    white-space: pre-wrap;
    word-break: break-all;
    margin: 0;
  }

  /* Web GUI Modal (Screenshot 4) */
  .webgui-info-card {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .webgui-badge-row {
    display: flex;
    gap: 0.5rem;
  }

  .badge-status-online {
    background: var(--tint-accent);
    color: var(--accent);
    font-size: 0.75rem;
    font-weight: 600;
    padding: 0.25rem 0.625rem;
    border-radius: 999px;
  }

  .badge-php-version {
    background: var(--bg-canvas);
    border: 1px solid var(--border-color);
    color: var(--text-muted);
    font-size: 0.75rem;
    padding: 0.25rem 0.5rem;
    border-radius: 999px;
  }

  .webgui-desc {
    font-size: 0.875rem;
    color: var(--text-muted);
    margin: 0;
  }

  .webgui-cards-container {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    margin-top: 0.5rem;
  }

  .webgui-choice-card {
    background: var(--bg-canvas);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .webgui-choice-card.highlight {
    border-color: var(--accent);
    background: var(--bg-surface);
  }

  .choice-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .badge-new-sso {
    background: var(--accent);
    color: var(--text-on-accent);
    font-size: 0.6875rem;
    font-weight: 700;
    padding: 0.125rem 0.375rem;
    border-radius: 4px;
    letter-spacing: 0.05em;
  }

  .choice-card-title {
    margin: 0;
    font-size: 0.9375rem;
    font-weight: 600;
    color: var(--text-primary);
  }

  .choice-card-desc {
    margin: 0;
    font-size: 0.8125rem;
    color: var(--text-muted);
    line-height: 1.4;
  }

  /* Quota Modal */
  .input-with-unit {
    position: relative;
    display: flex;
    align-items: center;
  }

  .input-with-unit .form-input {
    width: 100%;
    padding-right: 3rem;
  }

  .unit-badge {
    position: absolute;
    right: 0.75rem;
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--text-muted);
  }

  .quota-reminder-box {
    background: var(--bg-inset);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    padding: 0.875rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.375rem;
  }

  .reminder-line {
    margin: 0;
    font-size: 0.75rem;
    color: var(--text-muted);
    line-height: 1.4;
  }

  .reminder-line strong {
    color: var(--warning);
  }

  .mt-2 { margin-top: 0.5rem; }
  .mt-3 { margin-top: 0.75rem; }
  .mt-4 { margin-top: 1rem; }
  .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.75rem; }
</style>
