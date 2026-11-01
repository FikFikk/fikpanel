<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { BackupConfig, BackupRecord } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import '../styles/pages/backup.css';

  let configs = $state<BackupConfig[]>([]);
  let loading = $state(true);
  let showCreateModal = $state(false);
  let showRecordModal = $state(false);
  let selectedConfig = $state<BackupConfig | null>(null);
  let records = $state<BackupRecord[]>([]);
  let loadingRecords = $state(false);

  // State form
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
      confirmText: 'Hapus',
      tone: 'danger',
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

<div class="pg-backup backup-view">
  <!-- Baris kepala halaman -->
  <header class="page-header">
    <div class="page-header__main">
      <h1 class="page-header__title">
        <i class="ri-inbox-archive-line"></i> Backup & Snapshot S3
      </h1>
      <p class="page-header__sub">
        Pencadangan database & file web otomatis ke penyimpanan lokal atau S3 / MinIO Object Storage.
      </p>
    </div>
    <div class="page-header__actions">
      <button class="btn btn--ghost" onclick={loadBackups} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh
      </button>
      <button class="btn btn--primary" onclick={() => (showCreateModal = true)}>
        <i class="ri-add-line"></i> Buat Backup
      </button>
    </div>
  </header>

  <!-- Kartu tabel konfigurasi -->
  <div class="card">
    <div class="table-wrap">
      <table class="table">
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
                <i class="ri-loader-4-line spin"></i> Memuat daftar backup...
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
                  <span class="badge badge--neutral">{cfg.target_type.toUpperCase()}</span>
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
                  <button class="btn btn--icon btn--sm" aria-label="Cadangkan sekarang {cfg.name}" title="Cadangkan Sekarang" onclick={() => runBackupNow(cfg)}>
                    <i class="ri-play-line"></i>
                  </button>
                  <button class="btn btn--icon btn--sm" aria-label="Daftar arsip file {cfg.name}" title="Daftar Arsip File" onclick={() => viewRecords(cfg)}>
                    <i class="ri-archive-line"></i>
                  </button>
                  <button class="btn btn--icon btn--sm btn--danger" aria-label="Hapus konfigurasi {cfg.name}" title="Hapus Konfigurasi" onclick={() => deleteConfig(cfg)}>
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

  <!-- Modal tambah backup -->
  <Modal
    open={showCreateModal}
    title="Buat Konfigurasi Pencadangan Baru"
    onclose={() => (showCreateModal = false)}
  >
    <form id="backup-create-form" onsubmit={(e) => { e.preventDefault(); handleCreateConfig(); }} class="form-body">
      <div class="field">
        <label for="b-name" class="field__label">Nama Backup</label>
        <input id="b-name" type="text" class="input" bind:value={formName} placeholder="Misal: Backup MySQL KostAsh Harian" required />
      </div>

      <div class="form-row">
        <div class="field grow">
          <label for="b-type" class="field__label">Target yang Dicadangkan</label>
          <select id="b-type" class="select" bind:value={formTargetType}>
            <option value="database">Database (MySQL / MariaDB)</option>
            <option value="site">Website (Direktori Web)</option>
            <option value="directory">Folder Tertentu</option>
          </select>
        </div>
        <div class="field grow">
          <label for="b-tname" class="field__label">Nama Target</label>
          <input id="b-tname" type="text" class="input" bind:value={formTargetName} placeholder="nama_db atau kostash.my.id" required />
        </div>
      </div>

      <div class="field">
        <label for="b-store" class="field__label">Tujuan Penyimpanan</label>
        <select id="b-store" class="select" bind:value={formStorageType}>
          <option value="s3">S3 / MinIO Object Storage (Cloud/Remote)</option>
          <option value="local">Penyimpanan Server Lokal (/var/backups)</option>
        </select>
      </div>

      {#if formStorageType === 's3'}
        <div class="form-row">
          <div class="field grow">
            <label for="b-ep" class="field__label">Endpoint S3 / MinIO</label>
            <input id="b-ep" type="text" class="input" bind:value={formS3Endpoint} placeholder="storage.fikfikk.my.id" />
          </div>
          <div class="field grow">
            <label for="b-bkt" class="field__label">Bucket S3</label>
            <input id="b-bkt" type="text" class="input" bind:value={formS3Bucket} placeholder="server-backups" />
          </div>
        </div>
        <div class="form-row">
          <div class="field grow">
            <label for="b-ak" class="field__label">Access Key</label>
            <input id="b-ak" type="text" class="input" bind:value={formS3AccessKey} placeholder="MinIO Username/Key" />
          </div>
          <div class="field grow">
            <label for="b-sk" class="field__label">Secret Key</label>
            <input id="b-sk" type="password" class="input" bind:value={formS3SecretKey} placeholder="MinIO Secret Key" />
          </div>
        </div>
      {/if}

      <div class="field">
        <label for="b-ret" class="field__label">Batas Simpan Salinan (Retensi)</label>
        <input id="b-ret" type="number" class="input" bind:value={formKeepCopies} min="1" max="50" />
      </div>
    </form>

    {#snippet footer()}
      <button type="button" class="btn btn--ghost" onclick={() => (showCreateModal = false)}>Batal</button>
      <button type="submit" form="backup-create-form" class="btn btn--primary" disabled={submitting}>
        {submitting ? 'Menyimpan...' : 'Simpan Konfigurasi'}
      </button>
    {/snippet}
  </Modal>

  <!-- Modal riwayat arsip -->
  <Modal
    open={showRecordModal && !!selectedConfig}
    title="Daftar Snapshot: {selectedConfig?.name || ''}"
    size="lg"
    onclose={() => (showRecordModal = false)}
  >
    <div class="log-container">
      {#if loadingRecords}
        <div class="table-empty">
          <i class="ri-loader-4-line spin"></i> Memuat daftar arsip...
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

    {#snippet footer()}
      <button type="button" class="btn btn--ghost" onclick={() => (showRecordModal = false)}>Tutup</button>
    {/snippet}
  </Modal>
</div>
