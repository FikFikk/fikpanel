<script lang="ts">
  /**
   * Subkomponen Bilah Aksi, Auto Backup Terjadwal & Pencarian Database.
   */
  import type { AutoBackupStatus, DatabaseEngineInfo } from '../../lib/types';
  import '../../styles/pages/databases/action-bar.css';

  interface Props {
    autoBackupStatus: AutoBackupStatus;
    togglingAutoBackup: boolean;
    syncingHost: boolean;
    vaultUnlocked: boolean;
    vaultCountdownText: string;
    searchQuery: string;
    currentEngine: DatabaseEngineInfo;
    onToggleAutoBackup: (enabled: boolean) => void;
    onToggleVault: () => void;
    onAddDb: () => void;
    onOpenRootPass: () => void;
    onOpenWebGui: () => void;
    onSync: () => void;
    onOpenEngineManager: () => void;
  }

  let {
    autoBackupStatus,
    togglingAutoBackup,
    syncingHost,
    vaultUnlocked,
    vaultCountdownText,
    searchQuery = $bindable(),
    currentEngine,
    onToggleAutoBackup,
    onToggleVault,
    onAddDb,
    onOpenRootPass,
    onOpenWebGui,
    onSync,
    onOpenEngineManager
  }: Props = $props();
</script>

<div class="action-bar-card pg-databases-action-bar">
  <!-- Toggle Auto Backup Terintegrasi dengan Cron Manager -->
  <div class="auto-backup-row">
    <div class="auto-backup-left">
      <label class="toggle-switch">
        <input
          type="checkbox"
          checked={autoBackupStatus.enabled}
          disabled={togglingAutoBackup}
          aria-label="Aktifkan auto backup terjadwal"
          onchange={(e) => onToggleAutoBackup((e.currentTarget as HTMLInputElement).checked)}
        />
        <span class="slider"></span>
      </label>
      <span class="auto-backup-label">
        <strong>Auto Backup Terjadwal ({currentEngine.display_name}):</strong>
        {#if autoBackupStatus.enabled}
          Dump otomatis terkompresi aktif via Cron Manager setiap hari (retensi {autoBackupStatus.keep_days} hari).
        {:else}
          Aktifkan untuk menjadwalkan dump otomatis harian (.sql.gz) dengan pembersihan retensi otomatis.
        {/if}
      </span>
    </div>

    {#if autoBackupStatus.enabled}
      <span class="auto-backup-pill" title="Terhubung dengan Cron Manager FikPanel">
        <i class="ri-time-line" aria-hidden="true"></i>
        <span>Cron: {autoBackupStatus.schedule} · Simpan {autoBackupStatus.keep_days}h</span>
      </span>
    {/if}
  </div>

  <!-- Baris Tombol Aksi -->
  <div class="action-buttons-row">
    <div class="btn-group-left">
      <button type="button" class="btn btn-primary" onclick={onAddDb}>
        <i class="ri-add-line" aria-hidden="true"></i>
        <span>Tambah DB</span>
      </button>
      <button type="button" class="btn btn-outline" onclick={onOpenRootPass}>
        <i class="ri-key-2-line" aria-hidden="true"></i>
        <span>Password Root</span>
      </button>
      {#if currentEngine.name === 'mysql'}
        <button
          type="button"
          class="btn btn-outline"
          onclick={onOpenWebGui}
          title="Kelola akses phpMyAdmin / SSO"
        >
          <i class="ri-external-link-line" aria-hidden="true"></i>
          <span>Web GUI</span>
        </button>
      {/if}
      <button
        type="button"
        class="btn btn-outline"
        disabled={syncingHost}
        onclick={onSync}
        title="Pindai database langsung dari server host dan sinkronkan ke panel"
      >
        <i class={syncingHost ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
        <span>{syncingHost ? 'Menyinkronkan...' : 'Sinkron Host'}</span>
      </button>
    </div>

    <div class="btn-group-right">
      <button
        type="button"
        class="vault-status-pill"
        class:unlocked={vaultUnlocked}
        onclick={onToggleVault}
        title={vaultUnlocked
          ? 'Vault terbuka — klik untuk mengunci kembali segera'
          : 'Vault terkunci — klik untuk verifikasi Biometrik/Passkey atau Password Admin'}
      >
        <i class={vaultUnlocked ? 'ri-lock-unlock-line' : 'ri-lock-2-line'} aria-hidden="true"></i>
        <span>{vaultUnlocked ? `Vault Terbuka (${vaultCountdownText})` : 'Vault Terkunci'}</span>
      </button>

      <button
        type="button"
        class="engine-status-badge is-clickable"
        onclick={onOpenEngineManager}
        title="Kelola layanan, Processlist, RAM Tuner, konfigurasi, dan log"
      >
        <span class="status-indicator-dot {currentEngine.running ? 'running' : 'stopped'}"></span>
        <span>{currentEngine.display_name} {currentEngine.version || (currentEngine.running ? 'Aktif' : 'Berhenti')}</span>
        <i class="ri-settings-3-line badge-settings-icon" aria-hidden="true"></i>
      </button>

      <div class="search-box">
        <i class="ri-search-line search-icon" aria-hidden="true"></i>
        <input
          type="text"
          class="search-input"
          placeholder="Cari database atau user..."
          aria-label="Cari database atau username"
          bind:value={searchQuery}
        />
      </div>
    </div>
  </div>
</div>
