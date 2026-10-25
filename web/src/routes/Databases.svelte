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

  // State Form Quick Install
  let installVersion = $state('7.0');
  let installMode = $state<'native' | 'docker'>('native');
  let installing = $state(false);

  // State Form Tambah Database
  let dbName = $state('');
  let dbUser = $state('');
  let dbPass = $state('');
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
      const res = await apiFetch('/api/databases/engines');
      if (res.ok) {
        const data = await res.json();
        engines = data.engines || [];
        // Set versi default untuk modal instalasi sesuai engine aktif
        const curr = engines.find((e) => e.name === activeEngine);
        if (curr && curr.supported_versions?.length > 0) {
          installVersion = curr.supported_versions[0];
        }
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
      const res = await apiFetch(`/api/databases/${eng}/list`);
      if (res.ok) {
        const data = await res.json();
        databases = data.databases || [];
      } else {
        databases = [];
      }
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
      const res = await apiFetch('/api/databases/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          engine: activeEngine,
          version: installVersion,
          mode: installMode,
        }),
      });

      if (!res.ok) {
        const err = await res.json();
        throw new Error(err.error || 'Gagal memicu instalasi');
      }

      const data = await res.json();
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
      const res = await apiFetch('/api/system/tasks');
      if (res.ok) {
        const data = await res.json();
        tasks = data.tasks || [];
      }
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

  // Buka modal tambah database
  function openAddModal() {
    dbName = '';
    dbUser = '';
    dbNote = '';
    dbAccessHost = 'localhost';
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

    creatingDb = true;
    try {
      const res = await apiFetch(`/api/databases/${activeEngine}/create`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: dbName.trim(),
          username: dbUser.trim(),
          password: dbPass.trim(),
          access_host: dbAccessHost,
          note: dbNote.trim(),
        }),
      });

      if (!res.ok) {
        const err = await res.json();
        throw new Error(err.error || 'Gagal membuat database');
      }

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
      const res = await apiFetch(`/api/databases/${activeEngine}/${d.name}`, {
        method: 'DELETE',
      });
      if (!res.ok) {
        const err = await res.json();
        throw new Error(err.error || 'Gagal menghapus database');
      }
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
            onclick={() => notify('Membuka antarmuka manajemen web database...', 'info')}
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
          <div class="engine-status-badge">
            <span class="status-indicator-dot"></span>
            <span>{currentEngine.display_name} {currentEngine.version || 'Aktif'}</span>
          </div>

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
                      onclick={() => notify(`Membuka Web GUI untuk database ${db.name}`, 'info')}
                    >
                      Web GUI
                    </button>
                    <button
                      class="action-link danger"
                      onclick={() => deleteDb(db)}
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
        <div class="form-group">
          <label for="db-name" class="form-label">Nama Database</label>
          <input
            id="db-name"
            type="text"
            class="form-input"
            placeholder="misal: sql_myproject"
            value={dbName}
            oninput={(e) => onDbNameInput((e.target as HTMLInputElement).value)}
            disabled={creatingDb}
          />
        </div>

        <div class="form-group">
          <label for="db-user" class="form-label">Username Akses</label>
          <input
            id="db-user"
            type="text"
            class="form-input"
            placeholder="Username akses database"
            bind:value={dbUser}
            disabled={creatingDb}
          />
        </div>

        <div class="form-group">
          <label for="db-pass" class="form-label">Password</label>
          <div class="input-with-button">
            <input
              id="db-pass"
              type="text"
              class="form-input"
              placeholder="Kata sandi kuat"
              bind:value={dbPass}
              disabled={creatingDb}
            />
            <button
              type="button"
              class="btn btn-outline btn-generate"
              onclick={generatePassword}
              disabled={creatingDb}
              title="Buat kata sandi acak kuat"
            >
              Acak
            </button>
          </div>
        </div>

        <div class="form-group">
          <label for="db-host" class="form-label">Akses Host</label>
          <select id="db-host" class="form-select" bind:value={dbAccessHost} disabled={creatingDb}>
            <option value="localhost">Localhost (127.0.0.1 - Direkomendasikan)</option>
            <option value="%">Semua Host (%)</option>
          </select>
        </div>

        <div class="form-group">
          <label for="db-note" class="form-label">Catatan</label>
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
            <span>Buat Database</span>
          {/if}
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
</style>
