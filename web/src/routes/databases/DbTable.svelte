<script lang="ts">
  /**
   * Subkomponen Tabel Daftar Database dengan Bulk Action Bar, Proteksi DB Sistem, SQL Studio, dan 1-Klik Backup/Copy .env.
   */
  import { formatBytes } from '../../lib/api';
  import type { DatabaseEngineInfo, DatabaseItem } from '../../lib/types';
  import '../../styles/pages/databases/db-table.css';

  interface Props {
    loadingDatabases: boolean;
    filteredDatabases: DatabaseItem[];
    currentEngine: DatabaseEngineInfo;
    vaultUnlocked: boolean;
    visiblePasswords: Record<number, boolean>;
    selectedNames: string[];
    bulkWorking: boolean;
    onToggleSelect: (name: string) => void;
    onToggleSelectAll: () => void;
    onClearSelection: () => void;
    onBulkBackup: () => void;
    onBulkDelete: () => void;
    onTogglePassword: (db: DatabaseItem) => void;
    onCopyPassword: (db: DatabaseItem) => void;
    onCopyEnv: (db: DatabaseItem) => void;
    onOpenSqlStudio: (db: DatabaseItem) => void;
    onOpenBackupModal: (db: DatabaseItem) => void;
    onOpenQuotaModal: (db: DatabaseItem) => void;
    onOpenPasswordModal: (db: DatabaseItem) => void;
    onLaunchWebGui: (db: DatabaseItem) => void;
    onDeleteDb: (db: DatabaseItem) => void;
  }

  let {
    loadingDatabases,
    filteredDatabases,
    currentEngine,
    vaultUnlocked,
    visiblePasswords,
    selectedNames,
    bulkWorking,
    onToggleSelect,
    onToggleSelectAll,
    onClearSelection,
    onBulkBackup,
    onBulkDelete,
    onTogglePassword,
    onCopyPassword,
    onCopyEnv,
    onOpenSqlStudio,
    onOpenBackupModal,
    onOpenQuotaModal,
    onOpenPasswordModal,
    onLaunchWebGui,
    onDeleteDb
  }: Props = $props();

  let allSelected = $derived(
    filteredDatabases.length > 0 && filteredDatabases.every((d) => selectedNames.includes(d.name))
  );

  // Saring catatan boilerplate adopsi otomatis agar tabel tidak penuh teks berulang
  function cleanNote(note?: string): string {
    if (!note) return '';
    if (note.includes('Diadopsi secara otomatis dari host server oleh FikPanel')) return '';
    return note;
  }
</script>

<div class="table-card pg-databases-db-table">
  {#if selectedNames.length > 0}
    <div class="bulk-bar">
      <span class="bulk-info">{selectedNames.length} database dipilih</span>
      <div class="bulk-actions">
        <button
          type="button"
          class="btn btn-primary btn-sm"
          disabled={bulkWorking}
          onclick={onBulkBackup}
        >
          <i class={bulkWorking ? 'ri-loader-4-line spin' : 'ri-download-cloud-2-line'} aria-hidden="true"></i>
          <span>Backup Massal (.sql.gz)</span>
        </button>
        <button
          type="button"
          class="btn btn-outline btn-sm"
          disabled={bulkWorking}
          onclick={onBulkDelete}
        >
          <i class="ri-delete-bin-line" aria-hidden="true"></i>
          <span>Hapus Terpilih</span>
        </button>
        <button
          type="button"
          class="btn btn-outline btn-sm"
          disabled={bulkWorking}
          onclick={onClearSelection}
        >
          Batal
        </button>
      </div>
    </div>
  {/if}

  {#if loadingDatabases}
    <div class="state state--loading">
      <span class="spinner" aria-hidden="true"></span>
      <span>Memuat daftar database...</span>
    </div>
  {:else if filteredDatabases.length === 0}
    <div class="state">
      <i class="ri-database-2-line state__icon" aria-hidden="true"></i>
      <p class="state__title">Belum ada database pada {currentEngine.display_name}</p>
      <p class="state__desc">Klik "Tambah DB" untuk membuat baru atau "Sinkron Host" untuk memindai database yang sudah ada.</p>
    </div>
  {:else}
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th class="col-check">
              <input
                type="checkbox"
                checked={allSelected}
                onchange={onToggleSelectAll}
                aria-label="Pilih semua database"
              />
            </th>
            <th>Nama Database</th>
            <th>Username</th>
            <th>Password</th>
            <th>Ukuran</th>
            <th>Cadangan</th>
            <th>Host</th>
            <th class="text-right">Operasi</th>
          </tr>
        </thead>
        <tbody>
          {#each filteredDatabases as db (db.id)}
            {@const noteText = cleanNote(db.note)}
            <tr>
              <td class="col-check">
                <input
                  type="checkbox"
                  checked={selectedNames.includes(db.name)}
                  onchange={() => onToggleSelect(db.name)}
                  aria-label={`Pilih ${db.name}`}
                />
              </td>
              <td>
                <div class="db-name-cell">
                  <div class="db-name-top">
                    <strong>{db.name}</strong>
                    {#if db.is_system}
                      <span class="badge-system" title="Database utilitas sistem yang dilindungi">Sistem</span>
                    {/if}
                  </div>
                  {#if noteText}
                    <span class="db-note-sub" title={noteText}>{noteText}</span>
                  {/if}
                </div>
              </td>
              <td class="col-mono">{db.username}</td>
              <td class="col-mono">
                <div class="pass-wrap">
                  {#if db.has_password || db.password}
                    <span>{vaultUnlocked && visiblePasswords[db.id] && db.password ? db.password : '••••••••'}</span>
                    <button
                      type="button"
                      class="btn-icon"
                      onclick={() => onTogglePassword(db)}
                      title={vaultUnlocked && visiblePasswords[db.id] ? 'Sembunyikan' : 'Verifikasi Vault & Lihat password'}
                      aria-label={vaultUnlocked && visiblePasswords[db.id] ? 'Sembunyikan password' : 'Lihat password'}
                    >
                      <i class={vaultUnlocked && visiblePasswords[db.id] ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
                    </button>
                    <button
                      type="button"
                      class="btn-icon"
                      onclick={() => onCopyPassword(db)}
                      title={vaultUnlocked ? 'Salin password' : 'Verifikasi Vault & Salin password'}
                      aria-label="Salin password"
                    >
                      <i class={vaultUnlocked ? 'ri-file-copy-line' : 'ri-lock-line'} aria-hidden="true"></i>
                    </button>
                  {:else}
                    <span class="text-muted">Belum disetel</span>
                    <button
                      type="button"
                      class="btn-icon"
                      onclick={() => onOpenPasswordModal(db)}
                      title="Catat atau atur kata sandi sekarang"
                      aria-label={`Atur kata sandi ${db.name}`}
                    >
                      <i class="ri-key-line" aria-hidden="true"></i>
                    </button>
                  {/if}
                </div>
              </td>
              <td class="col-nowrap">
                <span>{formatBytes(db.size_bytes)}</span>
                {#if db.quota_mb && db.quota_mb > 0}
                  <span class="badge-quota">Batas: {db.quota_mb} MB</span>
                {/if}
              </td>
              <td class="col-nowrap">
                <button
                  type="button"
                  class="badge-backup {db.backup_count > 0 ? 'exists' : 'empty'}"
                  onclick={() => onOpenBackupModal(db)}
                  title="Kelola cadangan, unduh, atau impor .sql"
                >
                  <i class={db.backup_count > 0 ? 'ri-checkbox-circle-line' : 'ri-archive-line'} aria-hidden="true"></i>
                  <span>{db.backup_count > 0 ? `${db.backup_count} arsip` : 'Cadangkan'}</span>
                </button>
              </td>
              <td class="col-mono">{db.location || 'localhost'}</td>
              <td class="col-actions">
                <div class="ops-group">
                  {#if currentEngine.name === 'mysql' || currentEngine.name === 'postgresql'}
                    <button
                      type="button"
                      class="op-chip primary"
                      onclick={() => onOpenSqlStudio(db)}
                      title="Buka SQL Studio & Penjelajah Tabel bawaan"
                    >
                      <i class="ri-terminal-box-line" aria-hidden="true"></i>
                      <span>SQL Studio</span>
                    </button>
                  {/if}

                  <button
                    type="button"
                    class="op-chip"
                    onclick={() => onCopyEnv(db)}
                    title={vaultUnlocked
                      ? 'Salin blok konfigurasi .env (Laravel / DSN)'
                      : 'Verifikasi Biometrik/Passkey atau Password Admin untuk menyalin .env'}
                    aria-label={`Salin konfigurasi .env untuk ${db.name}`}
                  >
                    <i class={vaultUnlocked ? 'ri-code-s-slash-line' : 'ri-lock-line'} aria-hidden="true"></i>
                    <span>.env</span>
                  </button>

                  {#if currentEngine.name === 'mysql'}
                    <button
                      type="button"
                      class="op-chip"
                      onclick={() => onLaunchWebGui(db)}
                      title="Buka phpMyAdmin SSO"
                      aria-label={`Buka phpMyAdmin untuk ${db.name}`}
                    >
                      <i class="ri-external-link-line" aria-hidden="true"></i>
                      <span>PMA</span>
                    </button>
                  {/if}

                  <button
                    type="button"
                    class="op-chip"
                    onclick={() => onOpenPasswordModal(db)}
                    title="Ubah kata sandi pengguna"
                    aria-label={`Ubah kata sandi ${db.name}`}
                  >
                    <i class="ri-key-2-line" aria-hidden="true"></i>
                  </button>

                  <button
                    type="button"
                    class="op-chip"
                    onclick={() => onOpenQuotaModal(db)}
                    title="Atur kuota kapasitas"
                    aria-label={`Atur kuota ${db.name}`}
                  >
                    <i class="ri-pie-chart-line" aria-hidden="true"></i>
                  </button>

                  <button
                    type="button"
                    class="op-chip danger"
                    disabled={!!db.is_system}
                    onclick={() => onDeleteDb(db)}
                    title={db.is_system ? 'Database sistem dilindungi dari penghapusan' : 'Hapus database'}
                    aria-label={`Hapus database ${db.name}`}
                  >
                    <i class="ri-delete-bin-line" aria-hidden="true"></i>
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
