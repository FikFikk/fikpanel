<script lang="ts">
  /**
   * Subkomponen Bilah Aksi & Pencarian Database.
   */
  import type { DatabaseEngineInfo } from '../../lib/types';
  import '../../styles/pages/databases/action-bar.css';

  interface Props {
    autoBackup: boolean;
    searchQuery: string;
    currentEngine: DatabaseEngineInfo;
    onAddDb: () => void;
    onOpenRootPass: () => void;
    onOpenWebGui: () => void;
    onSync: () => void;
    onOpenEngineManager: () => void;
  }

  let {
    autoBackup = $bindable(),
    searchQuery = $bindable(),
    currentEngine,
    onAddDb,
    onOpenRootPass,
    onOpenWebGui,
    onSync,
    onOpenEngineManager
  }: Props = $props();
</script>

<div class="action-bar-card pg-databases-action-bar">
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
      <button class="btn btn-primary" onclick={onAddDb}>
        <i class="ri-add-line" aria-hidden="true"></i>
        <span>Tambah DB</span>
      </button>
      <button class="btn btn-outline" onclick={onOpenRootPass}>
        <i class="ri-key-2-line" aria-hidden="true"></i>
        <span>Password Root</span>
      </button>
      <button
        class="btn btn-outline"
        onclick={onOpenWebGui}
        title="Kelola akses web GUI database (phpMyAdmin / Adminer)"
      >
        <i class="ri-external-link-line" aria-hidden="true"></i>
        <span>Web GUI</span>
      </button>
      <button
        class="btn btn-outline"
        onclick={onSync}
      >
        <i class="ri-refresh-line" aria-hidden="true"></i>
        <span>Sinkronisasi</span>
      </button>
    </div>

    <div class="btn-group-right">
      <button
        class="engine-status-badge is-clickable"
        onclick={onOpenEngineManager}
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

