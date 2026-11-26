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

  // State form konfigurasi backup baru
  let formName = $state('');
  let formTargetType = $state<'database' | 'site' | 'directory'>('database');
  let formTargetName = $state('');
  let formStorageType = $state<'local' | 's3'>('local');
  let formS3Endpoint = $state('');
  let formS3Bucket = $state('');
  let formS3Region = $state('us-east-1');
  let formS3AccessKey = $state('');
  let formS3SecretKey = $state('');
  let formKeepCopies = $state(7);
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
          name: formName.trim(),
          target_type: formTargetType,
          target_name: formTargetName.trim(),
          storage_type: formStorageType,
          s3_endpoint: formS3Endpoint.trim(),
          s3_bucket: formS3Bucket.trim(),
          s3_region: formS3Region.trim() || 'us-east-1',
          s3_access_key: formS3AccessKey.trim(),
          s3_secret_key: formS3SecretKey.trim(),
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
    formStorageType = 'local';
    formS3Endpoint = '';
    formS3Bucket = '';
    formS3Region = 'us-east-1';
    formS3AccessKey = '';
    formS3SecretKey = '';
    formKeepCopies = 7;
  }

  async function runBackupNow(cfg: BackupConfig) {
    try {
      notify(`Memulai proses backup "${cfg.name}"...`, 'info');
      await apiFetch(`/api/backups/${cfg.id}/run`, { method: 'POST' });
      notify(`Pencadangan "${cfg.name}" sedang diproses di latar belakang`, 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal menjalankan backup', 'error');
    }
  }

  async function deleteConfig(cfg: BackupConfig) {
    const ok = await confirmDialog({
      title: 'Hapus Konfigurasi Backup',
      message: `Hapus konfigurasi backup "${cfg.name}" beserta seluruh catatan riwayatnya?`,
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
    await refreshRecords(cfg.id);
  }

  async function refreshRecords(configId: number) {
    try {
      loadingRecords = true;
      const res = await apiFetch<{ ok: boolean; records: BackupRecord[] }>(`/api/backups/${configId}/records`);
      records = res?.records || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat arsip', 'error');
    } finally {
      loadingRecords = false;
    }
  }

  async function deleteRecord(rec: BackupRecord) {
    const ok = await confirmDialog({
      title: 'Hapus Snapshot Backup',
      message: `Hapus arsip "${rec.file_name}" dari daftar riwayat dan penyimpanan lokal?`,
      confirmText: 'Hapus',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/backups/records/${rec.id}`, { method: 'DELETE' });
      records = records.filter((r) => r.id !== rec.id);
      notify('Snapshot backup berhasil dihapus', 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus snapshot', 'error');
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
        <i class="ri-inbox-archive-line" aria-hidden="true"></i> Backup & Snapshot S3 Cloud
      </h1>
      <p class="page-header__sub">
        Pencadangan database (MySQL/PostgreSQL) & situs ke disk lokal atau S3 / MinIO / Cloudflare R2 (AWS SigV4) dengan retensi otomatis.
      </p>
    </div>
    <div class="page-header__actions">
      <button type="button" class="btn btn--ghost" onclick={loadBackups} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}" aria-hidden="true"></i> Segarkan
      </button>
      <button type="button" class="btn btn--primary" onclick={() => (showCreateModal = true)}>
        <i class="ri-add-line" aria-hidden="true"></i> Buat Kebijakan Backup
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
                <i class="ri-loader-4-line spin" aria-hidden="true"></i> Memuat daftar backup...
              </td>
            </tr>
          {:else if configs.length === 0}
            <tr>
              <td colspan="7" class="table-empty">
                Belum ada kebijakan backup. Klik "Buat Kebijakan Backup" untuk mulai mencadangkan database atau situs ke Lokal / S3 Cloud.
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
                    <i class={cfg.storage_type === 's3' ? 'ri-cloud-line' : 'ri-hard-drive-2-line'} aria-hidden="true"></i>
                    {cfg.storage_type === 's3' ? `S3 · ${cfg.s3_bucket || 'Cloud'}` : 'Lokal'}
                  </span>
                </td>
                <td>Simpan {cfg.keep_copies} Salinan</td>
                <td class="text-muted">{formatTimestamp(cfg.created_at)}</td>
                <td class="text-right actions-col">
                  <button type="button" class="btn btn--icon btn--sm" aria-label="Cadangkan sekarang {cfg.name}" title="Cadangkan Sekarang" onclick={() => runBackupNow(cfg)}>
                    <i class="ri-play-line" aria-hidden="true"></i>
                  </button>
                  <button type="button" class="btn btn--icon btn--sm" aria-label="Daftar arsip file {cfg.name}" title="Daftar Arsip File" onclick={() => viewRecords(cfg)}>
                    <i class="ri-archive-line" aria-hidden="true"></i>
                  </button>
                  <button type="button" class="btn btn--icon btn--sm btn--danger" aria-label="Hapus konfigurasi {cfg.name}" title="Hapus Konfigurasi" onclick={() => deleteConfig(cfg)}>
                    <i class="ri-delete-bin-line" aria-hidden="true"></i>
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
    title="Buat Kebijakan Pencadangan Baru"
    onclose={() => (showCreateModal = false)}
  >
    <form id="backup-create-form" onsubmit={(e) => { e.preventDefault(); handleCreateConfig(); }} class="form-body">
      <div class="field">
        <label for="b-name" class="field__label">Nama Kebijakan Backup</label>
        <input id="b-name" type="text" class="input" bind:value={formName} placeholder="Misal: Backup Database Produksi" required />
      </div>

      <div class="form-row">
        <div class="field grow">
          <label for="b-type" class="field__label">Tipe Target</label>
          <select id="b-type" class="select" bind:value={formTargetType}>
            <option value="database">Database (MySQL / MariaDB / pg:nama_db)</option>
            <option value="site">Website (Direktori /var/www)</option>
            <option value="directory">Folder Kustom (Absolute Path)</option>
          </select>
        </div>
        <div class="field grow">
          <label for="b-tname" class="field__label">Nama Target</label>
          <input id="b-tname" type="text" class="input" bind:value={formTargetName} placeholder="nama_db, * (semua), atau pg:app_db" required />
        </div>
      </div>

      <div class="field">
        <label for="b-store" class="field__label">Tujuan Penyimpanan</label>
        <select id="b-store" class="select" bind:value={formStorageType}>
          <option value="local">Penyimpanan Server Lokal (/var/backups/fikpanel)</option>
          <option value="s3">S3 / MinIO / Cloudflare R2 (AWS SigV4)</option>
        </select>
      </div>

      {#if formStorageType === 's3'}
        <div class="form-row">
          <div class="field grow">
            <label for="b-ep" class="field__label">Endpoint S3 / MinIO / R2</label>
            <input id="b-ep" type="text" class="input" bind:value={formS3Endpoint} placeholder="https://s3.domain.com atau kosongkan untuk AWS" />
          </div>
          <div class="field grow">
            <label for="b-bkt" class="field__label">Nama Bucket</label>
            <input id="b-bkt" type="text" class="input" bind:value={formS3Bucket} placeholder="server-backups" required />
          </div>
        </div>
        <div class="form-row">
          <div class="field grow">
            <label for="b-reg" class="field__label">Region</label>
            <input id="b-reg" type="text" class="input" bind:value={formS3Region} placeholder="us-east-1 atau auto (R2)" />
          </div>
          <div class="field grow">
            <label for="b-ak" class="field__label">Access Key ID</label>
            <input id="b-ak" type="text" class="input" bind:value={formS3AccessKey} placeholder="Access Key" required />
          </div>
        </div>
        <div class="field">
          <label for="b-sk" class="field__label">Secret Access Key</label>
          <input id="b-sk" type="password" class="input" bind:value={formS3SecretKey} placeholder="Secret Key" required />
        </div>
      {/if}

      <div class="field">
        <label for="b-ret" class="field__label">Batas Simpan Salinan (Retensi Otomatis)</label>
        <input id="b-ret" type="number" class="input" bind:value={formKeepCopies} min="1" max="50" />
      </div>
    </form>

    {#snippet footer()}
      <button type="button" class="btn btn--ghost" onclick={() => (showCreateModal = false)}>Batal</button>
      <button type="submit" form="backup-create-form" class="btn btn--primary" disabled={submitting}>
        {submitting ? 'Menyimpan...' : 'Simpan Kebijakan'}
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
          <i class="ri-loader-4-line spin" aria-hidden="true"></i> Memuat daftar arsip...
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
                <span class="rec-meta">
                  {formatBytes(rec.file_size_bytes)} · {formatTimestamp(rec.created_at)}
                  {#if rec.storage_path} · {rec.storage_path}{/if}
                </span>
                {#if rec.error_message}
                  <span class="rec-error">{rec.error_message}</span>
                {/if}
              </div>
              <div class="rec-right">
                <span class="rec-badge {rec.status}">{rec.status.toUpperCase()}</span>
                {#if rec.status === 'success'}
                  <a
                    class="btn btn--icon btn--sm"
                    href={`/api/backups/records/${rec.id}/download`}
                    download={rec.file_name}
                    title="Unduh arsip lokal"
                    aria-label={`Unduh ${rec.file_name}`}
                  >
                    <i class="ri-download-line" aria-hidden="true"></i>
                  </a>
                {/if}
                <button
                  type="button"
                  class="btn btn--icon btn--sm btn--danger"
                  onclick={() => deleteRecord(rec)}
                  title="Hapus snapshot"
                  aria-label={`Hapus snapshot ${rec.file_name}`}
                >
                  <i class="ri-delete-bin-line" aria-hidden="true"></i>
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    {#snippet footer()}
      {#if selectedConfig}
        <button type="button" class="btn btn--ghost" onclick={() => selectedConfig && refreshRecords(selectedConfig.id)}>
          <i class="ri-refresh-line" aria-hidden="true"></i> Segarkan
        </button>
      {/if}
      <button type="button" class="btn btn--ghost" onclick={() => (showRecordModal = false)}>Tutup</button>
    {/snippet}
  </Modal>
</div>
