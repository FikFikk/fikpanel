<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { CronJob, CronLog } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  let jobs = $state<CronJob[]>([]);
  let loading = $state(true);
  let showCreateModal = $state(false);
  let showLogModal = $state(false);
  let selectedJob = $state<CronJob | null>(null);
  let logs = $state<CronLog[]>([]);
  let loadingLogs = $state(false);

  // Form state
  let formName = $state('');
  let formScheduleType = $state<'minute_n' | 'hourly' | 'daily' | 'weekly' | 'monthly' | 'custom'>('daily');
  let formSpec = $state('00:00');
  let formCommand = $state('');
  let formTimeout = $state(3600);
  let formSaveLog = $state(true);
  let submitting = $state(false);

  async function loadJobs() {
    try {
      loading = true;
      const res = await apiFetch<{ ok: boolean; jobs: CronJob[] }>('/api/cron/jobs');
      if (res && res.jobs) {
        jobs = res.jobs;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat daftar cron job', 'error');
    } finally {
      loading = false;
    }
  }

  async function handleCreateJob() {
    if (!formName.trim() || !formCommand.trim()) {
      notify('Nama tugas dan perintah eksekusi wajib diisi', 'warning');
      return;
    }

    try {
      submitting = true;
      await apiFetch('/api/cron/jobs', {
        method: 'POST',
        body: JSON.stringify({
          name: formName,
          schedule_type: formScheduleType,
          spec: formSpec,
          command: formCommand,
          timeout_sec: formTimeout,
          save_log: formSaveLog,
        }),
      });

      notify('Tugas cron berhasil ditambahkan', 'success');
      showCreateModal = false;
      resetForm();
      await loadJobs();
    } catch (err: any) {
      notify(err.message || 'Gagal menambahkan tugas cron', 'error');
    } finally {
      submitting = false;
    }
  }

  function resetForm() {
    formName = '';
    formScheduleType = 'daily';
    formSpec = '00:00';
    formCommand = '';
    formTimeout = 3600;
    formSaveLog = true;
  }

  async function toggleJob(job: CronJob) {
    try {
      await apiFetch(`/api/cron/jobs/${job.id}/toggle`, {
        method: 'POST',
        body: JSON.stringify({ enabled: !job.enabled }),
      });
      job.enabled = !job.enabled;
      notify(`Tugas ${job.enabled ? 'diaktifkan' : 'dinonaktifkan'}`, 'info');
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status', 'error');
    }
  }

  async function runJobNow(job: CronJob) {
    try {
      notify(`Menjalankan tugas "${job.name}"...`, 'info');
      const res = await apiFetch<{ ok: boolean; log: CronLog }>(`/api/cron/jobs/${job.id}/run`, {
        method: 'POST',
      });
      if (res && res.log) {
        job.last_status = res.log.status;
        job.last_run_at = res.log.finished_at;
        if (res.log.status === 'success') {
          notify(`Tugas "${job.name}" selesai dijalankan (Kode: ${res.log.exit_code})`, 'success');
        } else {
          notify(`Tugas "${job.name}" selesai dengan status ${res.log.status}`, 'warning');
        }
      }
    } catch (err: any) {
      notify(err.message || 'Gagal mengeksekusi tugas', 'error');
    }
  }

  async function deleteJob(job: CronJob) {
    const ok = await confirmDialog({
      title: 'Hapus Tugas Cron',
      message: `Apakah Anda yakin ingin menghapus tugas "${job.name}"? Riwayat eksekusi juga akan dihapus.`,
      confirmLabel: 'Hapus',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/cron/jobs/${job.id}`, { method: 'DELETE' });
      jobs = jobs.filter((j) => j.id !== job.id);
      notify('Tugas cron berhasil dihapus', 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus tugas', 'error');
    }
  }

  async function viewLogs(job: CronJob) {
    selectedJob = job;
    showLogModal = true;
    try {
      loadingLogs = true;
      const res = await apiFetch<{ ok: boolean; logs: CronLog[] }>(`/api/cron/jobs/${job.id}/logs`);
      logs = res?.logs || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat log', 'error');
    } finally {
      loadingLogs = false;
    }
  }

  function formatTimestamp(ts: number): string {
    if (!ts) return '-';
    const d = new Date(ts * 1000);
    return d.toLocaleString('id-ID', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  onMount(() => {
    loadJobs();
  });
</script>

<div class="cron-view">
  <!-- Header Bar -->
  <header class="page-head">
    <div class="page-title-wrap">
      <h1 class="page-title">
        <i class="ri-time-line"></i> Penjadwal Tugas (Cron)
      </h1>
      <p class="page-sub">
        Kelola jadwal eksekusi skrip otomatis, pembersihan log, dan maintenance server berkala.
      </p>
    </div>
    <div class="page-actions">
      <button class="btn btn-ghost" onclick={loadJobs} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh
      </button>
      <button class="btn btn-primary" onclick={() => (showCreateModal = true)}>
        <i class="ri-add-line"></i> Tambah Tugas
      </button>
    </div>
  </header>

  <!-- Table Card -->
  <div class="panel-card">
    <div class="table-wrap">
      <table class="data-table">
        <thead>
          <tr>
            <th>NAMA TUGAS</th>
            <th>JADWAL</th>
            <th>PERINTAH</th>
            <th>STATUS TERAKHIR</th>
            <th>TERAKHIR DIJALANKAN</th>
            <th class="text-right">AKSI</th>
          </tr>
        </thead>
        <tbody>
          {#if loading && jobs.length === 0}
            <tr>
              <td colspan="6" class="table-empty">
                <i class="ri-loader-4-line spin text-lg"></i> Memuat daftar tugas terjadwal...
              </td>
            </tr>
          {:else if jobs.length === 0}
            <tr>
              <td colspan="6" class="table-empty">
                Belum ada tugas cron yang didaftarkan.
              </td>
            </tr>
          {:else}
            {#each jobs as job (job.id)}
              <tr>
                <td>
                  <div class="job-name-col">
                    <span class="status-indicator {job.enabled ? 'active' : 'paused'}"></span>
                    <span class="font-bold">{job.name}</span>
                  </div>
                </td>
                <td>
                  <span class="badge-spec">{job.schedule_type}: {job.spec}</span>
                </td>
                <td class="col-cmd mono" title={job.command}>
                  {job.command}
                </td>
                <td>
                  {#if job.last_status === 'success'}
                    <span class="status-badge success"><i class="ri-checkbox-circle-fill"></i> Berhasil</span>
                  {:else if job.last_status === 'failed'}
                    <span class="status-badge failed"><i class="ri-close-circle-fill"></i> Gagal</span>
                  {:else if job.last_status === 'running'}
                    <span class="status-badge running"><i class="ri-loader-4-line spin"></i> Berjalan</span>
                  {:else}
                    <span class="text-muted">-</span>
                  {/if}
                </td>
                <td class="text-muted">
                  {formatTimestamp(job.last_run_at)}
                </td>
                <td class="text-right actions-col">
                  <button class="btn-icon" title="Jalankan Sekarang" onclick={() => runJobNow(job)}>
                    <i class="ri-play-line"></i>
                  </button>
                  <button class="btn-icon" title={job.enabled ? 'Jeda Tugas' : 'Aktifkan'} onclick={() => toggleJob(job)}>
                    <i class={job.enabled ? 'ri-pause-line' : 'ri-play-circle-line'}></i>
                  </button>
                  <button class="btn-icon" title="Riwayat Log" onclick={() => viewLogs(job)}>
                    <i class="ri-file-list-3-line"></i>
                  </button>
                  <button class="btn-icon danger" title="Hapus Tugas" onclick={() => deleteJob(job)}>
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
</div>

<!-- Modal Tambah Tugas -->
{#if showCreateModal}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) showCreateModal = false; }}
    onkeydown={(e) => { if (e.key === 'Escape') showCreateModal = false; }}
  >
    <div class="modal-card">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-time-line"></i>
          <h3>Tambah Tugas Terjadwal (Cron)</h3>
        </div>
        <button class="btn-close" onclick={() => (showCreateModal = false)}>&times;</button>
      </div>

      <form onsubmit={(e) => { e.preventDefault(); handleCreateJob(); }} class="form-body">
        <div class="form-group">
          <label for="form-name">Nama Tugas</label>
          <input id="form-name" type="text" bind:value={formName} placeholder="Misal: Backup Database Harian" required />
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label for="form-type">Tipe Jadwal</label>
            <select id="form-type" bind:value={formScheduleType}>
              <option value="daily">Setiap Hari (Daily)</option>
              <option value="hourly">Setiap Jam (Hourly)</option>
              <option value="minute_n">Per N Menit</option>
              <option value="weekly">Setiap Minggu (Weekly)</option>
              <option value="monthly">Setiap Bulan (Monthly)</option>
            </select>
          </div>
          <div class="form-group flex-1">
            <label for="form-spec">Parameter (Jam:Menit)</label>
            <input id="form-spec" type="text" bind:value={formSpec} placeholder="00:00 (HH:MM)" required />
          </div>
        </div>

        <div class="form-group">
          <label for="form-cmd">Perintah Eksekusi (Shell)</label>
          <textarea id="form-cmd" bind:value={formCommand} rows="3" class="mono" placeholder="/usr/local/bin/backup.sh atau docker system prune -f" required></textarea>
        </div>

        <div class="checkbox-row">
          <label class="custom-chk">
            <input type="checkbox" bind:checked={formSaveLog} />
            <span>Simpan riwayat log eksekusi</span>
          </label>
        </div>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" onclick={() => (showCreateModal = false)}>Batal</button>
          <button type="submit" class="btn btn-primary" disabled={submitting}>
            {submitting ? 'Menyimpan...' : 'Simpan Tugas'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- Modal Riwayat Log -->
{#if showLogModal && selectedJob}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) showLogModal = false; }}
    onkeydown={(e) => { if (e.key === 'Escape') showLogModal = false; }}
  >
    <div class="modal-card modal-wide">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-file-list-3-line"></i>
          <h3>Riwayat Eksekusi: {selectedJob.name}</h3>
        </div>
        <button class="btn-close" onclick={() => (showLogModal = false)}>&times;</button>
      </div>

      <div class="log-container">
        {#if loadingLogs}
          <div class="table-empty">
            <i class="ri-loader-4-line spin text-lg"></i> Memuat log eksekusi...
          </div>
        {:else if logs.length === 0}
          <div class="table-empty">
            Belum ada catatan eksekusi untuk tugas ini.
          </div>
        {:else}
          <div class="log-list">
            {#each logs as log (log.id)}
              <div class="log-item">
                <div class="log-meta">
                  <span class="status-badge {log.status}">
                    {log.status.toUpperCase()} (Kode: {log.exit_code})
                  </span>
                  <span class="text-muted">{formatTimestamp(log.started_at)} • {log.duration_ms}ms</span>
                </div>
                {#if log.output}
                  <pre class="log-output mono">{log.output}</pre>
                {:else}
                  <span class="text-subtle italic">Tidak ada output terminal.</span>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" onclick={() => (showLogModal = false)}>Tutup</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .cron-view {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .page-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .page-title {
    font-size: 1.4rem;
    font-weight: 700;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.6rem;
    margin: 0;
  }
  .page-sub {
    font-size: 0.85rem;
    color: var(--text-muted);
    margin: 0.25rem 0 0 0;
  }
  .page-actions {
    display: flex;
    gap: 0.5rem;
  }
  .panel-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    overflow: hidden;
  }
  .table-wrap {
    overflow-x: auto;
  }
  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
    text-align: left;
  }
  .data-table th {
    background: var(--bg-surface-soft);
    color: var(--text-subtle);
    padding: 0.75rem 1rem;
    font-size: 0.72rem;
    font-weight: 600;
    letter-spacing: 0.05em;
    border-bottom: 1px solid var(--border);
  }
  .data-table td {
    padding: 0.85rem 1rem;
    border-bottom: 1px solid var(--border-soft);
    color: var(--text);
  }
  .data-table tr:hover {
    background: var(--bg-hover);
  }
  .job-name-col {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .status-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
  .status-indicator.active {
    background: var(--accent);
    box-shadow: 0 0 8px var(--ring-accent);
  }
  .status-indicator.paused {
    background: var(--text-subtle);
  }
  .badge-spec {
    display: inline-block;
    padding: 0.2rem 0.5rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-family: var(--font-mono);
    font-size: 0.75rem;
  }
  .col-cmd {
    max-width: 260px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-muted);
  }
  .status-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.75rem;
    font-weight: 600;
  }
  .status-badge.success { color: var(--accent); }
  .status-badge.failed { color: var(--danger); }
  .status-badge.running { color: var(--warning); }
  .btn-icon {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0.35rem;
    border-radius: 6px;
    font-size: 1rem;
    transition: background 0.15s, color 0.15s;
  }
  .btn-icon:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .btn-icon.danger:hover {
    background: var(--tint-danger);
    color: var(--danger);
  }
  .text-right { text-align: right; }
  .mono { font-family: var(--font-mono); }
  .text-muted { color: var(--text-muted); }
  .text-subtle { color: var(--text-subtle); }
  .font-bold { font-weight: 600; }
  .spin { animation: spin 1s linear infinite; }
  @keyframes spin { 100% { transform: rotate(360deg); } }

  /* Modal Styling */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: var(--bg-overlay);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: var(--z-modal);
    padding: 1rem;
  }
  .modal-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    width: 100%;
    max-width: 520px;
    padding: 1.5rem;
    box-shadow: var(--shadow-modal);
  }
  .modal-card.modal-wide {
    max-width: 760px;
  }
  .modal-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.25rem;
  }
  .modal-title-wrap {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .modal-title-wrap i {
    font-size: 1.3rem;
    color: var(--primary);
  }
  .modal-title-wrap h3 {
    margin: 0;
    font-size: 1rem;
    color: var(--text);
  }
  .btn-close {
    background: transparent;
    border: none;
    color: var(--text-subtle);
    font-size: 1.4rem;
    cursor: pointer;
  }
  .form-body {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .form-row {
    display: flex;
    gap: 1rem;
  }
  .flex-1 { flex: 1; }
  .form-group label {
    display: block;
    font-size: 0.78rem;
    font-weight: 500;
    color: var(--text-muted);
    margin-bottom: 0.35rem;
  }
  .form-group input, .form-group select, .form-group textarea {
    width: 100%;
    padding: 0.6rem 0.75rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text);
    font-size: 0.82rem;
    outline: none;
    box-sizing: border-box;
  }
  .form-group input:focus, .form-group select:focus, .form-group textarea:focus {
    border-color: var(--primary);
  }
  .checkbox-row {
    padding-top: 0.25rem;
  }
  .custom-chk {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.8rem;
    color: var(--text-muted);
    cursor: pointer;
  }
  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
    margin-top: 1rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border-soft);
  }
  .log-container {
    max-height: 60vh;
    overflow-y: auto;
  }
  .log-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .log-item {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75rem;
  }
  .log-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.75rem;
    margin-bottom: 0.5rem;
  }
  .log-output {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.6rem;
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    font-size: 0.75rem;
    margin: 0;
    overflow-x: auto;
    white-space: pre-wrap;
    max-height: 180px;
  }
  .table-empty {
    text-align: center;
    padding: 2.5rem;
    color: var(--text-subtle);
  }
</style>
