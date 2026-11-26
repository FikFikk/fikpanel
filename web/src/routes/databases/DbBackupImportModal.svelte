<script lang="ts">
  /**
   * Modal Manajemen Backup 1-Klik, Unduh, Restore, dan Impor Berkas .sql/.sql.gz per Database.
   */
  import { apiFetch, formatBytes } from '../../lib/api';
  import { notify } from '../../lib/stores/toast.svelte';
  import { confirmDialog } from '../../lib/stores/confirm.svelte';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DatabaseBackupFile, DatabaseItem } from '../../lib/types';
  import '../../styles/pages/databases/db-backup-import-modal.css';

  interface Props {
    open: boolean;
    engine: string;
    db: DatabaseItem | null;
    onChanged: () => void;
    onClose: () => void;
  }

  let { open, engine, db, onChanged, onClose }: Props = $props();

  let backups = $state<DatabaseBackupFile[]>([]);
  let loading = $state(false);
  let dumping = $state(false);
  let restoring = $state('');
  let importing = $state(false);
  let selectedFile = $state<File | null>(null);
  let fileInputEl = $state<HTMLInputElement | null>(null);

  $effect(() => {
    if (open && db) {
      selectedFile = null;
      if (fileInputEl) fileInputEl.value = '';
      loadBackups();
    }
  });

  // Muat daftar arsip .sql/.sql.gz yang tersimpan di server untuk database ini
  async function loadBackups() {
    if (!db) return;
    loading = true;
    try {
      const res = await apiFetch<{ ok: boolean; backups: DatabaseBackupFile[] }>(
        `/api/databases/${engine}/${encodeURIComponent(db.name)}/backups`
      );
      backups = res.backups || [];
    } catch {
      backups = [];
    } finally {
      loading = false;
    }
  }

  // Buat snapshot dump .sql.gz baru secara instan
  async function createDumpNow() {
    if (!db) return;
    dumping = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(
        `/api/databases/${engine}/${encodeURIComponent(db.name)}/dump`,
        { method: 'POST', timeoutMs: 180_000 }
      );
      notify(res.message || 'Cadangan database berhasil dibuat', 'success');
      await loadBackups();
      onChanged();
    } catch (e: any) {
      notify(e.message || 'Gagal mencadangkan database', 'error');
    } finally {
      dumping = false;
    }
  }

  // Pulihkan database dari salah satu berkas arsip cadangan tersimpan
  async function restoreFromFile(fileName: string) {
    if (!db) return;
    const ok = await confirmDialog({
      title: `Pulihkan Database ${db.name}`,
      message: `Data saat ini pada database "${db.name}" akan ditimpa oleh isi arsip "${fileName}". Lanjutkan pemulihan?`,
      confirmText: 'Pulihkan Sekarang',
      tone: 'danger',
    });
    if (!ok) return;

    restoring = fileName;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(
        `/api/databases/${engine}/${encodeURIComponent(db.name)}/restore`,
        {
          method: 'POST',
          body: JSON.stringify({ file_name: fileName }),
          timeoutMs: 300_000,
        }
      );
      notify(res.message || 'Database berhasil dipulihkan', 'success');
      onChanged();
    } catch (e: any) {
      notify(e.message || 'Gagal memulihkan database', 'error');
    } finally {
      restoring = '';
    }
  }

  // Unggah dan impor berkas .sql atau .sql.gz langsung ke database
  async function importUploadedSql() {
    if (!db || !selectedFile) {
      notify('Pilih berkas .sql atau .sql.gz terlebih dahulu', 'warning');
      return;
    }
    const ok = await confirmDialog({
      title: `Impor SQL ke ${db.name}`,
      message: `Berkas "${selectedFile.name}" (${formatBytes(selectedFile.size)}) akan dieksekusi langsung ke database "${db.name}". Lanjutkan?`,
      confirmText: 'Impor Sekarang',
      tone: 'danger',
    });
    if (!ok) return;

    importing = true;
    try {
      const form = new FormData();
      form.append('file', selectedFile);
      const res = await apiFetch<{ ok: boolean; message: string }>(
        `/api/databases/${engine}/${encodeURIComponent(db.name)}/restore`,
        {
          method: 'POST',
          body: form,
          timeoutMs: 300_000,
        }
      );
      notify(res.message || 'Impor berkas SQL berhasil', 'success');
      selectedFile = null;
      if (fileInputEl) fileInputEl.value = '';
      onChanged();
    } catch (e: any) {
      notify(e.message || 'Gagal mengimpor berkas SQL', 'error');
    } finally {
      importing = false;
    }
  }

  // Hapus satu berkas arsip dari server
  async function deleteArchive(fileName: string) {
    const ok = await confirmDialog({
      title: 'Hapus Arsip Cadangan',
      message: `Hapus berkas "${fileName}" secara permanen dari penyimpanan server?`,
      confirmText: 'Hapus',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/databases/backups/file?file=${encodeURIComponent(fileName)}`, {
        method: 'DELETE',
      });
      notify('Arsip cadangan berhasil dihapus', 'success');
      await loadBackups();
      onChanged();
    } catch (e: any) {
      notify(e.message || 'Gagal menghapus arsip', 'error');
    }
  }

  function formatTs(ts: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString('id-ID', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }
</script>

<Modal
  {open}
  title={db ? `Backup, Restore & Impor — ${db.name}` : 'Backup & Impor Database'}
  size="lg"
  onclose={() => {
    if (!dumping && !importing && !restoring) onClose();
  }}
>
  <div class="pg-databases-backup-import">
    <div class="backup-actions-grid">
      <!-- Kartu 1: Dump Instan -->
      <div class="backup-action-box">
        <h4 class="backup-box-title">
          <i class="ri-save-3-line" aria-hidden="true"></i>
          <span>Cadangkan Sekarang (.sql.gz)</span>
        </h4>
        <p class="backup-box-desc">
          Ekspor struktur, data, triggers, dan routines secara konsisten tanpa mengunci tabel produksi.
        </p>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          disabled={dumping || importing}
          onclick={createDumpNow}
        >
          <i class={dumping ? 'ri-loader-4-line spin' : 'ri-download-cloud-2-line'} aria-hidden="true"></i>
          <span>{dumping ? 'Membuat Snapshot...' : 'Buat Cadangan Sekarang'}</span>
        </button>
      </div>

      <!-- Kartu 2: Impor Berkas SQL -->
      <div class="backup-action-box">
        <h4 class="backup-box-title">
          <i class="ri-upload-2-line" aria-hidden="true"></i>
          <span>Impor Berkas .sql / .sql.gz</span>
        </h4>
        <p class="backup-box-desc">
          Unggah berkas dump SQL (maks. 128 MB) untuk langsung diimpor ke database ini.
        </p>
        <div class="import-file-row">
          <input
            bind:this={fileInputEl}
            type="file"
            class="import-file-input"
            accept=".sql,.gz"
            aria-label="Pilih berkas SQL atau SQL.GZ"
            onchange={(e) => {
              const target = e.currentTarget as HTMLInputElement;
              selectedFile = target.files?.[0] || null;
            }}
          />
          <button
            type="button"
            class="btn btn-outline btn-sm"
            disabled={!selectedFile || importing || dumping}
            onclick={importUploadedSql}
          >
            <i class={importing ? 'ri-loader-4-line spin' : 'ri-upload-cloud-line'} aria-hidden="true"></i>
            <span>{importing ? 'Mengimpor...' : 'Impor'}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Daftar Arsip Tersimpan -->
    <div class="backup-list-head">
      <h4 class="backup-list-title">Arsip Cadangan Tersimpan ({backups.length})</h4>
      <button
        type="button"
        class="btn btn-outline btn-sm"
        disabled={loading}
        onclick={loadBackups}
      >
        <i class={loading ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
        <span>Segarkan</span>
      </button>
    </div>

    {#if loading}
      <div class="backup-empty">Memuat daftar berkas cadangan...</div>
    {:else if backups.length === 0}
      <div class="backup-empty">
        Belum ada arsip cadangan tersimpan untuk database ini. Klik "Buat Cadangan Sekarang" di atas.
      </div>
    {:else}
      <div class="backup-files-list">
        {#each backups as file (file.file_name)}
          <div class="backup-file-row">
            <div class="backup-file-meta">
              <span class="backup-file-name">{file.file_name}</span>
              <span class="backup-file-sub">{formatBytes(file.size_bytes)} · {formatTs(file.created_at)}</span>
            </div>
            <div class="backup-file-ops">
              <a
                class="btn btn-outline btn-sm"
                href={`/api/databases/backups/download?file=${encodeURIComponent(file.file_name)}`}
                download={file.file_name}
                title="Unduh berkas cadangan"
              >
                <i class="ri-download-line" aria-hidden="true"></i>
                <span>Unduh</span>
              </a>
              <button
                type="button"
                class="btn btn-outline btn-sm"
                disabled={restoring !== ''}
                onclick={() => restoreFromFile(file.file_name)}
                title="Pulihkan database dari arsip ini"
              >
                <i class={restoring === file.file_name ? 'ri-loader-4-line spin' : 'ri-history-line'} aria-hidden="true"></i>
                <span>Restore</span>
              </button>
              <button
                type="button"
                class="btn btn-outline btn-sm"
                disabled={restoring !== ''}
                onclick={() => deleteArchive(file.file_name)}
                title="Hapus arsip ini"
                aria-label={`Hapus arsip ${file.file_name}`}
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
    <button
      type="button"
      class="btn btn-outline"
      disabled={dumping || importing || restoring !== ''}
      onclick={onClose}
    >
      Tutup
    </button>
  {/snippet}
</Modal>
