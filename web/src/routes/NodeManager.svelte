<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { PM2Process, NodeSystemStatus } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import '../styles/pages/nodemanager.css';

  let status = $state<NodeSystemStatus | null>(null);
  let processes = $state<PM2Process[]>([]);
  let loading = $state(true);

  // State modal tambah aplikasi
  let showAddModal = $state(false);
  let formName = $state('');
  let formScript = $state('index.js');
  let formCwd = $state('');
  let formInstances = $state('1');
  let formWatch = $state(false);
  let formArgs = $state('');
  let submittingApp = $state(false);

  // State modal log output
  let showLogModal = $state(false);
  let selectedProcess = $state<PM2Process | null>(null);
  let logContent = $state('');
  let loadingLogs = $state(false);

  async function loadData() {
    try {
      loading = true;
      const res = await apiFetch<{ ok: boolean; status: NodeSystemStatus; processes: PM2Process[] }>('/api/node/processes');
      if (res) {
        status = res.status || null;
        processes = res.processes || [];
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat status proses Node.js / PM2', 'error');
    } finally {
      loading = false;
    }
  }

  async function handleAction(p: PM2Process, action: 'restart' | 'stop' | 'start' | 'reload' | 'delete') {
    const actionLabels: Record<string, string> = {
      restart: 'Restart',
      stop: 'Hentikan',
      start: 'Jalankan',
      reload: 'Reload',
      delete: 'Hapus',
    };
    const actionLabel = actionLabels[action] || action;

    const ok = await confirmDialog({
      title: `${actionLabel} Proses PM2`,
      message: action === 'delete'
        ? `Apakah Anda yakin ingin menghapus proses "${p.name}" dari PM2? Proses akan dihentikan dan dihapus dari startup.`
        : `Apakah Anda yakin ingin mengeksekusi ${actionLabel.toLowerCase()} pada proses "${p.name}"?`,
      confirmText: actionLabel,
      tone: action === 'delete' || action === 'stop' ? 'danger' : 'default',
    });
    if (!ok) return;

    try {
      notify(`Memproses ${action} "${p.name}"...`, 'info');
      await apiFetch(`/api/node/processes/${p.id}/${action}${p.user ? `?user=${encodeURIComponent(p.user)}` : ''}`, { method: 'POST' });
      notify(`Proses "${p.name}" berhasil di-${action}`, 'success');
      await loadData();
    } catch (err: any) {
      notify(err.message || `Gagal ${action} proses`, 'error');
    }
  }

  async function handleStartApp() {
    if (!formName.trim() || !formScript.trim()) {
      notify('Nama aplikasi dan berkas script wajib diisi', 'warning');
      return;
    }

    try {
      submittingApp = true;
      await apiFetch('/api/node/processes/start', {
        method: 'POST',
        body: JSON.stringify({
          name: formName.trim(),
          script: formScript.trim(),
          cwd: formCwd.trim(),
          instances: formInstances,
          watch: formWatch,
          args: formArgs.trim(),
        }),
      });

      notify(`Aplikasi "${formName}" berhasil dijalankan dengan PM2`, 'success');
      showAddModal = false;
      formName = '';
      formScript = 'index.js';
      formCwd = '';
      formInstances = '1';
      formWatch = false;
      formArgs = '';
      await loadData();
    } catch (err: any) {
      notify(err.message || 'Gagal menjalankan aplikasi PM2', 'error');
    } finally {
      submittingApp = false;
    }
  }

  async function openLogs(p: PM2Process) {
    selectedProcess = p;
    showLogModal = true;
    await fetchLogs(p.id, p.user);
  }

  async function fetchLogs(id: number, user?: string) {
    try {
      loadingLogs = true;
      const res = await apiFetch<{ ok: boolean; logs: string }>(`/api/node/processes/${id}/logs?lines=80${user ? `&user=${encodeURIComponent(user)}` : ''}`);
      logContent = res.logs || 'Belum ada catatan log.';
    } catch (err: any) {
      logContent = `Gagal mengambil log: ${err.message}`;
    } finally {
      loadingLogs = false;
    }
  }

  function formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function formatUptime(uptimeMs: number): string {
    if (!uptimeMs || uptimeMs <= 0) return '-';
    const sec = Math.floor((Date.now() - uptimeMs) / 1000);
    if (sec < 60) return `${sec}d`;
    const min = Math.floor(sec / 60);
    if (min < 60) return `${min}m`;
    const hrs = Math.floor(min / 60);
    if (hrs < 24) return `${hrs}j ${min % 60}m`;
    const days = Math.floor(hrs / 24);
    return `${days}h ${hrs % 24}j`;
  }

  onMount(() => {
    loadData();
  });
</script>

<div class="pg-nodemanager node-view">
  <!-- Baris kepala halaman -->
  <header class="page-header">
    <div class="page-header__main">
      <h1 class="page-header__title">
        <i class="ri-nodejs-line"></i> Node.js & PM2 Process Manager
      </h1>
      <p class="page-header__sub">
        Pantau penggunaan CPU, konsumsi RAM, status uptime, dan kontrol siklus hidup aplikasi SSR / API Node.js secara nyata.
      </p>
    </div>
    <div class="page-header__actions">
      <button class="btn btn--ghost" onclick={loadData} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh
      </button>
      {#if status?.pm2_installed}
        <button class="btn btn--primary" onclick={() => (showAddModal = true)}>
          <i class="ri-add-line"></i> Tambah Aplikasi
        </button>
      {/if}
    </div>
  </header>

  <!-- Baris status lingkungan Node & PM2 -->
  <div class="env-status-grid">
    <div class="status-box">
      <div class="status-box-icon {status?.node_installed ? 'success' : 'muted'}">
        <i class="ri-terminal-line"></i>
      </div>
      <div class="status-box-info">
        <span class="status-box-label">Runtime Node.js</span>
        <span class="status-box-val">
          {#if status?.node_installed}
            <span class="dot-online"></span> {status.node_version}
          {:else}
            <span class="text-subtle">Belum Terdeteksi</span>
          {/if}
        </span>
      </div>
    </div>

    <div class="status-box">
      <div class="status-box-icon {status?.pm2_installed ? 'success' : 'danger'}">
        <i class="ri-cpu-line"></i>
      </div>
      <div class="status-box-info">
        <span class="status-box-label">PM2 Process Daemon</span>
        <span class="status-box-val">
          {#if status?.pm2_installed}
            <span class="dot-online"></span> Versi {status.pm2_version}
          {:else}
            <span class="text-danger">Belum Terpasang</span>
          {/if}
        </span>
      </div>
    </div>

    <div class="status-box">
      <div class="status-box-icon info">
        <i class="ri-apps-line"></i>
      </div>
      <div class="status-box-info">
        <span class="status-box-label">Aplikasi Aktif</span>
        <span class="status-box-val mono">
          {processes.filter((p) => p.status === 'online').length} / {processes.length} Online
        </span>
      </div>
    </div>
  </div>

  <!-- Peringatan jika PM2 belum terpasang -->
  {#if !loading && status && !status.pm2_installed}
    <div class="warning-banner">
      <div class="warning-icon">
        <i class="ri-alert-line"></i>
      </div>
      <div class="warning-body">
        <h3 class="warning-title">PM2 Process Manager Belum Terpasang</h3>
        <p class="warning-text">
          PM2 diperlukan untuk menjaga aplikasi Node.js tetap berjalan di latar belakang (daemon) dan melakukan restart otomatis saat aplikasi mengalami crash atau server reboot.
        </p>
        <div class="code-copy-box">
          <code>npm install -g pm2</code>
        </div>
      </div>
    </div>
  {/if}

  <!-- Tabel proses PM2 -->
  <div class="card">
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>ID / NAMA APLIKASI</th>
            <th>STATUS</th>
            <th>PID</th>
            <th>CPU</th>
            <th>MEMORI (RAM)</th>
            <th>UPTIME</th>
            <th>RESTART</th>
            <th>MODE</th>
            <th class="text-right">KONTROL</th>
          </tr>
        </thead>
        <tbody>
          {#if loading && processes.length === 0}
            <tr>
              <td colspan="9" class="table-empty">
                <i class="ri-loader-4-line spin"></i> Memeriksa proses PM2 sistem...
              </td>
            </tr>
          {:else if processes.length === 0}
            <tr>
              <td colspan="9">
                <div class="state state--empty">
                  <div class="state__icon"><i class="ri-nodejs-line"></i></div>
                  <div class="state__title">Tidak ada proses PM2 aktif</div>
                  <div class="state__desc">
                    {#if status?.pm2_installed}
                      Klik tombol <strong>"Tambah Aplikasi"</strong> di atas untuk menjalankan aplikasi Node.js baru.
                    {:else}
                      Pasang PM2 di server untuk mulai mengelola proses Node.js.
                    {/if}
                  </div>
                </div>
              </td>
            </tr>
          {:else}
            {#each processes as p (`${p.user || 'root'}-${p.id}-${p.name}`)}
              <tr>
                <td>
                  <div class="proc-name-col">
                    <span class="badge-id">#{p.id}</span>
                    <div class="proc-title-wrap">
                      <span class="font-bold">{p.name} {#if p.user}<span class="badge-mode">@{p.user}</span>{/if}</span>
                      {#if p.cwd || p.script}
                        <span class="text-subtle mono truncate" title="{p.cwd || ''}/{p.script || ''}">
                          {p.script || p.cwd}
                        </span>
                      {/if}
                    </div>
                  </div>
                </td>
                <td>
                  <span class="status-badge {p.status}">
                    <span class="status-dot {p.status}"></span>
                    {p.status.toUpperCase()}
                  </span>
                </td>
                <td class="mono text-muted">{p.pid || '-'}</td>
                <td class="mono font-medium">{p.cpu || 0}%</td>
                <td class="mono font-medium">{formatBytes(p.memory)}</td>
                <td class="mono text-muted">{formatUptime(p.uptime || 0)}</td>
                <td class="text-muted">{p.restart_count || 0}x</td>
                <td>
                  <span class="badge-mode">{p.mode || 'fork'}</span>
                </td>
                <td class="text-right actions-col">
                  {#if p.status === 'online'}
                    <button class="btn btn--icon btn--sm" aria-label="Restart aplikasi {p.name}" title="Restart Aplikasi" onclick={() => handleAction(p, 'restart')}>
                      <i class="ri-restart-line"></i>
                    </button>
                    <button class="btn btn--icon btn--sm btn--danger" aria-label="Hentikan aplikasi {p.name}" title="Hentikan Aplikasi" onclick={() => handleAction(p, 'stop')}>
                      <i class="ri-stop-circle-line"></i>
                    </button>
                  {:else}
                    <button class="btn btn--icon btn--sm btn--success" aria-label="Jalankan aplikasi {p.name}" title="Jalankan Aplikasi" onclick={() => handleAction(p, 'start')}>
                      <i class="ri-play-circle-line"></i>
                    </button>
                  {/if}
                  <button class="btn btn--icon btn--sm" aria-label="Lihat log {p.name}" title="Lihat Log Real-time" onclick={() => openLogs(p)}>
                    <i class="ri-file-list-3-line"></i>
                  </button>
                  <button class="btn btn--icon btn--sm btn--danger" aria-label="Hapus {p.name} dari PM2" title="Hapus dari PM2" onclick={() => handleAction(p, 'delete')}>
                    <i class="ri-delete-bin-line"></i>
                  </button>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>

  <!-- Modal tambah aplikasi PM2 -->
  <Modal
    open={showAddModal}
    title="Tambah Aplikasi Node.js / PM2"
    onclose={() => (showAddModal = false)}
  >
    <div class="add-form">
      <div class="field">
        <label class="field__label" for="appName">Nama Aplikasi <span class="required">*</span></label>
        <input
          id="appName"
          class="input"
          type="text"
          placeholder="misal: api-service, web-frontend"
          bind:value={formName}
        />
      </div>

      <div class="field">
        <label class="field__label" for="appScript">Berkas Entrypoint / Script <span class="required">*</span></label>
        <input
          id="appScript"
          class="input mono"
          type="text"
          placeholder="misal: server.js, index.js, dist/main.js, atau npm -- start"
          bind:value={formScript}
        />
        <span class="field__hint">Berkas JavaScript/TypeScript utama atau perintah script npm.</span>
      </div>

      <div class="field">
        <label class="field__label" for="appCwd">Working Directory (Folder Kerja)</label>
        <input
          id="appCwd"
          class="input mono"
          type="text"
          placeholder="misal: /var/www/my-api"
          bind:value={formCwd}
        />
      </div>

      <div class="form-row-2">
        <div class="field">
          <label class="field__label" for="appInstances">Jumlah Instance</label>
          <select id="appInstances" class="select" bind:value={formInstances}>
            <option value="1">1 (Single Fork Mode)</option>
            <option value="2">2 Instance (Cluster)</option>
            <option value="4">4 Instance (Cluster)</option>
            <option value="max">Max (Semua Core CPU)</option>
          </select>
        </div>

        <div class="field">
          <label class="field__label" for="appArgs">Argumen Tambahan</label>
          <input
            id="appArgs"
            class="input mono"
            type="text"
            placeholder="--port 3000 --env prod"
            bind:value={formArgs}
          />
        </div>
      </div>

      <div class="form-checkbox-wrap">
        <label class="checkbox-label">
          <input type="checkbox" bind:checked={formWatch} />
          <span>Aktifkan Watch & Reload (Otomatis restart saat berkas berubah)</span>
        </label>
      </div>
    </div>

    {#snippet footer()}
      <button class="btn btn--ghost" onclick={() => (showAddModal = false)}>Batal</button>
      <button class="btn btn--primary" onclick={handleStartApp} disabled={submittingApp}>
        {#if submittingApp}
          <i class="ri-loader-4-line spin"></i> Memulai...
        {:else}
          <i class="ri-play-line"></i> Jalankan Aplikasi
        {/if}
      </button>
    {/snippet}
  </Modal>

  <!-- Modal log output PM2 -->
  <Modal
    open={showLogModal}
    title="Log Real-time: {selectedProcess?.name || ''} (#{selectedProcess?.id ?? ''})"
    size="lg"
    onclose={() => (showLogModal = false)}
  >
    <div class="log-toolbar">
      <span class="text-subtle">Menampilkan 80 baris log stdout dan stderr gabungan terbaru.</span>
      <button class="btn btn--ghost btn--sm" onclick={() => selectedProcess && fetchLogs(selectedProcess.id, selectedProcess.user)} disabled={loadingLogs}>
        <i class="ri-refresh-line {loadingLogs ? 'spin' : ''}"></i> Refresh Log
      </button>
    </div>

    <div class="log-terminal">
      {#if loadingLogs}
        <div class="log-loading">
          <i class="ri-loader-4-line spin"></i> Mengambil catatan log PM2...
        </div>
      {:else}
        <pre class="log-pre">{logContent}</pre>
      {/if}
    </div>

    {#snippet footer()}
      <button class="btn btn--ghost" onclick={() => (showLogModal = false)}>Tutup</button>
    {/snippet}
  </Modal>
</div>
