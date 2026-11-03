<script lang="ts">
  /**
   * Subkomponen Tabel Daftar Database.
   */
  import { formatBytes } from '../../lib/api';
  import type { DatabaseEngineInfo, DatabaseItem } from '../../lib/types';
  import '../../styles/pages/databases/db-table.css';

  interface Props {
    loadingDatabases: boolean;
    filteredDatabases: DatabaseItem[];
    currentEngine: DatabaseEngineInfo;
    visiblePasswords: Record<number, boolean>;
    onTogglePassword: (id: number) => void;
    onCopyText: (text: string, label: string) => void;
    onOpenQuotaModal: (db: DatabaseItem) => void;
    onOpenPasswordModal: (db: DatabaseItem) => void;
    onLaunchWebGui: (db: DatabaseItem) => void;
    onDeleteDb: (db: DatabaseItem) => void;
  }

  let {
    loadingDatabases,
    filteredDatabases,
    currentEngine,
    visiblePasswords,
    onTogglePassword,
    onCopyText,
    onOpenQuotaModal,
    onOpenPasswordModal,
    onLaunchWebGui,
    onDeleteDb
  }: Props = $props();
</script>

<div class="table-card pg-databases-db-table">
  {#if loadingDatabases}
    <div class="state state--loading">
      <span class="spinner" aria-hidden="true"></span>
      <span>Memuat data database...</span>
    </div>
  {:else if filteredDatabases.length === 0}
    <div class="state">
      <i class="ri-database-2-line state__icon" aria-hidden="true"></i>
      <p class="state__title">Belum ada database pada {currentEngine.display_name}</p>
      <p class="state__desc">Klik tombol "Tambah DB" di atas untuk membuat database baru.</p>
    </div>
  {:else}
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th class="col-check"><input type="checkbox" aria-label="Pilih semua" /></th>
            <th>Nama Database</th>
            <th>Username</th>
            <th>Password</th>
            <th>Ukuran</th>
            <th>Backup</th>
            <th>Lokasi</th>
            <th>Catatan</th>
            <th class="text-right">Operasi</th>
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
                {#if db.password}
                  {#if visiblePasswords[db.id]}
                    <span>{db.password}</span>
                  {:else}
                    <span>••••••••••••</span>
                  {/if}
                  <button
                    class="btn-icon"
                    onclick={() => onTogglePassword(db.id)}
                    title={visiblePasswords[db.id] ? 'Sembunyikan' : 'Lihat'}
                  >
                    <i class={visiblePasswords[db.id] ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
                  </button>
                  <button
                    class="btn-icon"
                    onclick={() => onCopyText(db.password, 'Password')}
                    title="Salin password"
                  >
                    <i class="ri-file-copy-line" aria-hidden="true"></i>
                  </button>
                {:else}
                  <span class="text-muted">(Belum disetel)</span>
                  <button
                    class="btn-icon"
                    onclick={() => onOpenPasswordModal(db)}
                    title="Atur kata sandi sekarang"
                  >
                    <i class="ri-key-line" aria-hidden="true"></i>
                  </button>
                {/if}
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
                  onclick={() => onOpenQuotaModal(db)}
                  title="Atur kuota kapasitas database"
                >
                  Kuota
                </button>
                <button
                  class="action-link"
                  onclick={() => onOpenPasswordModal(db)}
                  title="Ubah atau atur kata sandi pengguna database ini"
                >
                  Sandi
                </button>
                <button
                  class="action-link"
                  onclick={() => onLaunchWebGui(db)}
                  title="Buka phpMyAdmin langsung ke basis data ini (Akses Bebas Sandi)"
                >
                  Web GUI
                </button>
                <button
                  class="action-link danger"
                  onclick={() => onDeleteDb(db)}
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

