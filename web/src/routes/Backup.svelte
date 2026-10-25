<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { BackupConfig, BackupRecord } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  let configs = $state<BackupConfig[]>([]);
  let loading = $state(true);
  let showCreateModal = $state(false);
  let showRecordModal = $state(false);
  let selectedConfig = $state<BackupConfig | null>(null);
  let records = $state<BackupRecord[]>([]);
  let loadingRecords = $state(false);

  // Form state
  let formName = $state('');
  let formTargetType = $state<'database' | 'site' | 'directory'>('database');
  let formTargetName = $state('');
  let formStorageType = $state<'local' | 's3'>('s3');
  let formS3Endpoint = $state('storage.fikfikk.my.id');
  let formS3Bucket = $state('server-backups');
  let formS3AccessKey = $state('');
  let formS3SecretKey = $state('');
  let formKeepCopies = $state(5);
  let submitting = $state(false);

  async function loadBackups() {
    try {
      loading = true;
      const res = await apiFetch<{ ok: boolean; configs: BackupConfig[] }>('/api/backups');
      if (res && res.configs) {
        configs = res.configs;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat konfigurasi backup', 'error');
    } finally {
      loading = false;
    }
  }

  async function handleCreateConfig() {
    if (!formName.trim() || !formTargetName.trim()) {
      notify('Nama konfigurasi dan nama target wajib diisi', 'warning');
      return;
    }

    try {
      submitting = true;
      await apiFetch('/api/backups', {
        method: 'POST',
        body: JSON.stringify({
          name: formName,
          target_type: formTargetType,
          target_name: formTargetName,
          storage_type: formStorageType,
          s3_endpoint: formS3Endpoint,
          s3_bucket: formS3Bucket,
          s3_access_key: formS3AccessKey,
          s3_secret_key: formS3SecretKey,
          keep_copies: formKeepCopies,
        }),
      });

      notify('Konfigurasi backup berhasil dibuat', 'success');
      showCreateModal = false;
      resetForm();
      await loadBackups();
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan konfigurasi backup', 'error');
    } finally {
      submitting = false;
    }
  }

  function resetForm() {
    formName = '';
    formTargetType = 'database';
    formTargetName = '';
    formStorageType = 's3';
    formS3Endpoint = 'storage.fikfikk.my.id';
    formS3Bucket = 'server-backups';
    formS3AccessKey = '';
    formS3SecretKey = '';
    formKeepCopies = 5;
  }

  async function runBackupNow(cfg: BackupConfig) {
    try {
      notify(`Memulai proses backup "${cfg.name}"...`, 'info');
      await apiFetch(`/api/backups/${cfg.id}/run`, { method: 'POST' });
      notify(`Pencadangan "${cfg.name}" berjalan di background`, 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal menjalankan backup', 'error');
    }
  }

  async function deleteConfig(cfg: BackupConfig) {
    const ok = await confirmDialog({
      title: 'Hapus Backup',
      message: `Hapus konfigurasi backup "${cfg.name}"? Semua arsip riwayat juga akan dihapus.`,
      confirmLabel: 'Hapus',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/backups/${cfg.id}`, { method: 'DELETE' });
      configs = configs.filter((c) => c.id !== cfg.id);
      notify('Konfigurasi backup dihapus', 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus backup', 'error');
    }
  }

  async function viewRecords(cfg: BackupConfig) {
    selectedConfig = cfg;
    showRecordModal = true;
    try {
      loadingRecords = true;
      const res = await apiFetch<{ ok: boolean; records: BackupRecord[] }>(`/api/backups/${cfg.id}/records`);
      records = res?.records || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat arsip', 'error');
    } finally {
      loadingRecords = false;
    }
  }

  function formatBytes(bytes: number): string {
    if (!bytes) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
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
    });
  }

  onMount(() => {
    loadBackups();
  });
</script>

<div class="backup-view">
  <!-- Header Bar -->
  <header class="page-head">
    <div class="page-title-wrap">
      <h1 class="page-title">
        <i class="ri-inbox-archive-line"></i> Backup & Snapshot S3
      </h1>
      <p class="page-sub">
        Pencadangan database & file web otomatis ke penyimpanan lokal atau S3 / MinIO Object Storage.
      </p>
    </div>
    <div class="page-actions">
      <button class="btn btn-ghost" onclick={loadBackups} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh
      </button>
      <button class="btn btn-primary" onclick={() => (showCreateModal = true)}>
        <i class="ri-add-line"></i> Buat Backup
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
            <th>TIPE TARGET</th>
            <th>NAMA TARGET</th>
            <th>PENYIMPANAN</th>
            <th>RETENSI</th>
            <th>DIBUAT PADA</th>
            <th class="text-right">AKSI</th>
          </tr>
        </thead>
        <tbody>
          {#if loading && configs.length === 0}
            <tr>
              <td colspan="7" class="table-empty">
                <i class="ri-loader-4-line spin text-lg"></i> Memuat daftar backup...
              </td>
            </tr>
          {:else if configs.length === 0}
            <tr>
              <td colspan="7" class="table-empty">
                Belum ada konfigurasi backup. Buat snapshot pertama Anda.
              </td>
            </tr>
          {:else}
            {#each configs as cfg (cfg.id)}
              <tr>
                <td>
                  <span class="font-bold">{cfg.name}</span>
                </td>
                <td>
                  <span class="badge-type">{cfg.target_type.toUpperCase()}</span>
                </td>
                <td class="mono">{cfg.target_name}</td>
                <td>
                  <span class="storage-badge {cfg.storage_type}">
                    <i class={cfg.storage_type === 's3' ? 'ri-cloud-line' : 'ri-hard-drive-2-line'}></i>
                    {cfg.storage_type === 's3' ? 'MinIO / S3' : 'Lokal'}
                  </span>
                </td>
                <td>Simpan {cfg.keep_copies} Salinan</td>
                <td class="text-muted">{formatTimestamp(cfg.created_at)}</td>
                <td class="text-right actions-col">
                  <button class="btn-icon" title="Cadangkan Sekarang" onclick={() => runBackupNow(cfg)}>
                    <i class="ri-play-line"></i>
                  </button>
                  <button class="btn-icon" title="Daftar Arsip File" onclick={() => viewRecords(cfg)}>
                    <i class="ri-archive-line"></i>
                  </button>
                  <button class="btn-icon danger" title="Hapus Konfigurasi" onclick={() => deleteConfig(cfg)}>
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

<!-- Modal Tambah Backup -->
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
          <i class="ri-inbox-archive-line"></i>
          <h3>Buat Konfigurasi Pencadangan Baru</h3>
        </div>
        <button class="btn-close" onclick={() => (showCreateModal = false)}>&times;</button>
      </div>

      <form onsubmit={(e) => { e.preventDefault(); handleCreateConfig(); }} class="form-body">
        <div class="form-group">
          <label for="b-name">Nama Backup</label>
          <input id="b-name" type="text" bind:value={formName} placeholder="Misal: Backup MySQL KostAsh Harian" required />
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label for="b-type">Target yang Dicadangkan</label>
            <select id="b-type" bind:value={formTargetType}>
              <option value="database">Database (MySQL / MariaDB)</option>
              <option value="site">Website (Direktori Web)</option>
              <option value="directory">Folder Tertentu</option>
            </select>
          </div>
          <div class="form-group flex-1">
            <label for="b-tname">Nama Target</label>
            <input id="b-tname" type="text" bind:value={formTargetName} placeholder="nama_db atau kostash.my.id" required />
          </div>
        </div>

        <div class="form-group">
          <label for="b-store">Tujuan Penyimpanan</label>
          <select id="b-store" bind:value={formStorageType}>
            <option value="s3">S3 / MinIO Object Storage (Cloud/Remote)</option>
            <option value="local">Penyimpanan Server Lokal (/var/backups)</option>
          </select>
        </div>

        {#if formStorageType === 's3'}
          <div class="form-row">
            <div class="form-group flex-1">
              <label for="b-ep">Endpoint S3 / MinIO</label>
              <input id="b-ep" type="text" bind:value={formS3Endpoint} placeholder="storage.fikfikk.my.id" />
            </div>
            <div class="form-group flex-1">
              <label for="b-bkt">Bucket S3</label>
              <input id="b-bkt" type="text" bind:value={formS3Bucket} placeholder="server-backups" />
            </div>
          </div>
          <div class="form-row">
            <div class="form-group flex-1">
              <label for="b-ak">Access Key</label>
              <input id="b-ak" type="text" bind:value={formS3AccessKey} placeholder="MinIO Username/Key" />
            </div>
            <div class="form-group flex-1">
              <label for="b-sk">Secret Key</label>
              <input id="b-sk" type="password" bind:value={formS3SecretKey} placeholder="MinIO Secret Key" />
            </div>
          </div>
        {/if}

        <div class="form-group">
          <label for="b-ret">Batas Simpan Salinan (Retensi)</label>
          <input id="b-ret" type="number" bind:value={formKeepCopies} min="1" max="50" />
        </div>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" onclick={() => (showCreateModal = false)}>Batal</button>
          <button type="submit" class="btn btn-primary" disabled={submitting}>
            {submitting ? 'Menyimpan...' : 'Simpan Konfigurasi'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- Modal Riwayat Arsip -->
{#if showRecordModal && selectedConfig}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) showRecordModal = false; }}
    onkeydown={(e) => { if (e.key === 'Escape') showRecordModal = false; }}
  >
    <div class="modal-card modal-wide">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-archive-line"></i>
          <h3>Daftar Snapshot: {selectedConfig.name}</h3>
        </div>
        <button class="btn-close" onclick={() => (showRecordModal = false)}>&times;</button>
      </div>

      <div class="log-container">
        {#if loadingRecords}
          <div class="table-empty">
            <i class="ri-loader-4-line spin text-lg"></i> Memuat daftar arsip...
          </div>
        {:else if records.length === 0}
          <div class="table-empty">
            Belum ada arsip pencadangan untuk tugas ini.
          </div>
        {:else}
          <div class="record-list">
            {#each records as rec (rec.id)}
              <div class="rec-item">
                <div class="rec-info">
                  <span class="rec-name mono">{rec.file_name}</span>
                  <span class="rec-meta">{formatBytes(rec.file_size_bytes)} • {formatTimestamp(rec.created_at)}</span>
                </div>
                <div class="rec-badge {rec.status}">
                  {rec.status.toUpperCase()}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" onclick={() => (showRecordModal = false)}>Tutup</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .backup-view {
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
    border-radius: var(--radius-xl);
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
  .badge-type {
    display: inline-block;
    padding: 0.15rem 0.45rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 0.7rem;
    font-weight: 600;
  }
  .storage-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.75rem;
    font-weight: 500;
  }
  .storage-badge.s3 { color: var(--accent); }
  .storage-badge.local { color: var(--text-muted); }
  .btn-icon {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0.35rem;
    border-radius: var(--radius-sm);
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
    max-width: 720px;
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
  .form-group input, .form-group select {
    width: 100%;
    padding: 0.6rem 0.75rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    color: var(--text);
    font-size: 0.82rem;
    outline: none;
    box-sizing: border-box;
  }
  .form-group input:focus, .form-group select:focus {
    border-color: var(--primary);
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
    max-height: 55vh;
    overflow-y: auto;
  }
  .record-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .rec-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 0.75rem 1rem;
  }
  .rec-info {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .rec-name {
    font-size: 0.8rem;
    color: var(--text);
    font-weight: 500;
  }
  .rec-meta {
    font-size: 0.72rem;
    color: var(--text-subtle);
  }
  .rec-badge {
    font-size: 0.7rem;
    font-weight: 700;
    padding: 0.2rem 0.5rem;
    border-radius: var(--radius-sm);
  }
  .rec-badge.success {
    background: var(--tint-accent);
    color: var(--accent);
  }
  .rec-badge.failed {
    background: var(--tint-danger);
    color: var(--danger);
  }
  .table-empty {
    text-align: center;
    padding: 2.5rem;
    color: var(--text-subtle);
  }
</style>
