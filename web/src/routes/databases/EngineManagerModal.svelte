<script lang="ts">
  /**
   * Subkomponen Modal Database Engine Manager (Status, Ganti Versi, Konfigurasi my.cnf, Log Error).
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DatabaseEngineInfo } from '../../lib/types';
  import '../../styles/pages/databases/engine-manager-modal.css';

  interface Props {
    open: boolean;
    activeEngine: 'mysql' | 'postgresql' | 'mongodb' | 'redis';
    currentEngine: DatabaseEngineInfo;
    engineTab: 'service' | 'switch_version' | 'config' | 'logs';
    serviceActionLoading: boolean;
    switchingVersion: boolean;
    switchTargetVersion: string;
    switchBackupFirst: boolean;
    engineConfigPath: string;
    engineConfigContent: string;
    loadingEngineConfig: boolean;
    savingEngineConfig: boolean;
    engineLogsContent: string;
    loadingEngineLogs: boolean;
    getEngineIcon: (engine: string) => string;
    onServiceAction: (action: 'start' | 'stop' | 'restart' | 'reload') => void;
    onSubmitSwitchVersion: () => void;
    onFetchEngineConfig: () => void;
    onSaveEngineConfig: () => void;
    onFetchEngineLogs: () => void;
    onClose: () => void;
  }

  let {
    open,
    activeEngine,
    currentEngine,
    engineTab = $bindable(),
    serviceActionLoading,
    switchingVersion,
    switchTargetVersion = $bindable(),
    switchBackupFirst = $bindable(),
    engineConfigPath,
    engineConfigContent = $bindable(),
    loadingEngineConfig,
    savingEngineConfig,
    engineLogsContent,
    loadingEngineLogs,
    getEngineIcon,
    onServiceAction,
    onSubmitSwitchVersion,
    onFetchEngineConfig,
    onSaveEngineConfig,
    onFetchEngineLogs,
    onClose
  }: Props = $props();
</script>

<Modal
  {open}
  size="lg"
  onclose={() => { if (!switchingVersion) onClose(); }}
>
  {#snippet header()}
    <div class="modal-title-with-icon">
      <i class="{getEngineIcon(activeEngine)} engine-install-icon" aria-hidden="true"></i>
      <div>
        <h3 class="modal__title">{currentEngine.display_name} Manager</h3>
        <p class="modal__subtitle">Kelola status layanan, alih versi, berkas konfigurasi, dan log</p>
      </div>
    </div>
  {/snippet}

  <div class="pg-databases-engine-manager-modal">
    <!-- Navigasi Tab Internal Engine Manager -->
    <div class="tabs tabs--underline engine-manager-tabs">
      <button
        class="tabs__tab"
        class:is-active={engineTab === 'service'}
        onclick={() => (engineTab = 'service')}
      >
        <i class="ri-dashboard-line" aria-hidden="true"></i>
        <span>Layanan (Service)</span>
      </button>
      <button
        class="tabs__tab"
        class:is-active={engineTab === 'switch_version'}
        onclick={() => (engineTab = 'switch_version')}
      >
        <i class="ri-shuffle-line" aria-hidden="true"></i>
        <span>Ganti Versi</span>
      </button>
      <button
        class="tabs__tab"
        class:is-active={engineTab === 'config'}
        onclick={() => {
          engineTab = 'config';
          onFetchEngineConfig();
        }}
      >
        <i class="ri-file-settings-line" aria-hidden="true"></i>
        <span>Konfigurasi (my.cnf)</span>
      </button>
      <button
        class="tabs__tab"
        class:is-active={engineTab === 'logs'}
        onclick={() => {
          engineTab = 'logs';
          onFetchEngineLogs();
        }}
      >
        <i class="ri-file-list-3-line" aria-hidden="true"></i>
        <span>Log Galat</span>
      </button>
    </div>

    {#if engineTab === 'service'}
      <!-- Tab 1: Status Layanan -->
      <div class="service-panel">
        <div class="service-status-box">
          <div class="status-indicator-large">
            <span class="status-dot-large {currentEngine.running ? 'online' : 'offline'}"></span>
            <div>
              <h4 class="service-status-heading">
                Status: {currentEngine.running ? 'Aktif (Running)' : 'Berhenti (Stopped)'}
              </h4>
              <p class="service-status-sub">
                Unit: <code>{currentEngine.service_unit || activeEngine + '.service'}</code> • Port: <code>{currentEngine.port}</code>
              </p>
            </div>
          </div>
        </div>

        <div class="service-actions-grid">
          <button
            class="service-action-card"
            disabled={serviceActionLoading}
            onclick={() => onServiceAction('restart')}
          >
            <i class="ri-restart-line action-icon" aria-hidden="true"></i>
            <div class="action-info">
              <strong>Restart Layanan</strong>
              <span>Mulai ulang proses daemon</span>
            </div>
          </button>

          {#if currentEngine.running}
            <button
              class="service-action-card danger"
              disabled={serviceActionLoading}
              onclick={() => onServiceAction('stop')}
            >
              <i class="ri-stop-circle-line action-icon" aria-hidden="true"></i>
              <div class="action-info">
                <strong>Hentikan Layanan</strong>
                <span>Stop daemon {currentEngine.display_name}</span>
              </div>
            </button>
          {:else}
            <button
              class="service-action-card success"
              disabled={serviceActionLoading}
              onclick={() => onServiceAction('start')}
            >
              <i class="ri-play-circle-line action-icon" aria-hidden="true"></i>
              <div class="action-info">
                <strong>Nyalakan Layanan</strong>
                <span>Start daemon {currentEngine.display_name}</span>
              </div>
            </button>
          {/if}

          <button
            class="service-action-card"
            disabled={serviceActionLoading}
            onclick={() => onServiceAction('reload')}
          >
            <i class="ri-refresh-line action-icon" aria-hidden="true"></i>
            <div class="action-info">
              <strong>Reload Konfigurasi</strong>
              <span>Terapkan konfigurasi tanpa downtime</span>
            </div>
          </button>
        </div>
      </div>
    {:else if engineTab === 'switch_version'}
      <!-- Tab 2: Ganti Versi (Switch Version) -->
      <div class="switch-version-panel">
        <div class="version-warning-banner">
          <i class="ri-alert-fill warning-icon" aria-hidden="true"></i>
          <div>
            <strong>Perhatian Arsitektur SQL:</strong>
            <p>
              Satu server host Linux hanya dapat menjalankan 1 versi aktif native pada port 3306.
              Mengganti versi secara langsung tanpa backup dapat menyebabkan ketidakcocokan format direktori data InnoDB.
              FikPanel akan mengekspor backup seluruh database secara otomatis sebelum memasang biner versi target.
            </p>
          </div>
        </div>

        <div class="form-group mt-3">
          <label for="current-ver-info" class="form-label">Versi Saat Ini yang Aktif</label>
          <div class="current-version-badge">
            <i class="ri-checkbox-circle-fill active-check" aria-hidden="true"></i>
            <span>{currentEngine.display_name} <strong>{currentEngine.version || '10.11'}</strong> (Sedang Berjalan)</span>
          </div>
        </div>

        <div class="form-group">
          <label for="target-ver-select" class="form-label">Pilih Versi Target yang Ingin Dituju</label>
          <select id="target-ver-select" class="form-select" bind:value={switchTargetVersion} disabled={switchingVersion}>
            {#if currentEngine.versions && currentEngine.versions.length > 0}
              {#each currentEngine.versions as v}
                <option value={v.version}>
                  {v.label} {v.version === currentEngine.version ? '(Versi Saat Ini)' : ''}
                </option>
              {/each}
            {:else}
              {#each currentEngine.supported_versions as sv}
                <option value={sv}>{currentEngine.display_name} {sv}</option>
              {/each}
            {/if}
          </select>
        </div>

        <div class="checkbox-row mt-2">
          <label class="custom-checkbox-label">
            <input type="checkbox" bind:checked={switchBackupFirst} disabled={switchingVersion} />
            <span>
              <strong>Buat cadangan snapshot otomatis seluruh database</strong> (mysqldump --all-databases) sebelum beralih versi (Sangat Direkomendasikan).
            </span>
          </label>
        </div>

        <div class="switch-action-row mt-4">
          <button
            class="btn btn-primary"
            disabled={switchingVersion || switchTargetVersion === currentEngine.version}
            onclick={onSubmitSwitchVersion}
          >
            {#if switchingVersion}
              <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
              <span>Memproses Alih Versi...</span>
            {:else}
              <i class="ri-shuffle-line" aria-hidden="true"></i>
              <span>Beralih ke Versi Ini (Switch Version)</span>
            {/if}
          </button>
        </div>
      </div>
    {:else if engineTab === 'config'}
      <!-- Tab 3: Konfigurasi my.cnf -->
      <div class="config-panel">
        <div class="config-panel-header">
          <span class="config-path-badge">Berkas: <code>{engineConfigPath || '/etc/mysql/my.cnf'}</code></span>
          <button
            class="btn btn-outline btn-sm"
            disabled={loadingEngineConfig}
            onclick={onFetchEngineConfig}
          >
            <i class="ri-refresh-line" aria-hidden="true"></i>
            <span>Muat Ulang</span>
          </button>
        </div>

        <textarea
          class="config-editor-textarea"
          bind:value={engineConfigContent}
          disabled={loadingEngineConfig || savingEngineConfig}
          spellcheck="false"
        ></textarea>

        <div class="config-footer-row">
          <span class="config-hint">Simpan perubahan dan restart layanan untuk menerapkan pengaturan baru.</span>
          <button
            class="btn btn-primary"
            disabled={savingEngineConfig || loadingEngineConfig}
            onclick={onSaveEngineConfig}
          >
            {#if savingEngineConfig}
              <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
              <span>Menyimpan...</span>
            {:else}
              <i class="ri-save-line" aria-hidden="true"></i>
              <span>Simpan Konfigurasi</span>
            {/if}
          </button>
        </div>
      </div>
    {:else if engineTab === 'logs'}
      <!-- Tab 4: Log Galat -->
      <div class="logs-panel">
        <div class="config-panel-header">
          <span class="config-path-badge">Log Galat Terkini (Error Log)</span>
          <button
            class="btn btn-outline btn-sm"
            disabled={loadingEngineLogs}
            onclick={onFetchEngineLogs}
          >
            <i class="ri-refresh-line" aria-hidden="true"></i>
            <span>Segarkan Log</span>
          </button>
        </div>
        <pre class="logs-terminal-viewer">{engineLogsContent || 'Tidak ada catatan log galat.'}</pre>
      </div>
    {/if}
  </div>

  {#snippet footer()}
    <button
      class="btn btn-outline"
      disabled={switchingVersion}
      onclick={onClose}
    >
      Tutup
    </button>
  {/snippet}
</Modal>

