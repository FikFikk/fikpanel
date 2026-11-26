<script lang="ts">
  /**
   * Modal Database Engine Manager: Kontrol Layanan, 1-Klik RAM Tuner, Processlist & Kill Query, Ganti Versi, Konfigurasi (CodeMirror), dan Log Galat.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import CodeMirrorEditor from '../../lib/components/ui/CodeMirrorEditor.svelte';
  import type { DatabaseEngineInfo, DatabaseProcessItem } from '../../lib/types';
  import '../../styles/pages/databases/engine-manager-modal.css';

  interface Props {
    open: boolean;
    activeEngine: 'mysql' | 'postgresql' | 'mongodb' | 'redis';
    currentEngine: DatabaseEngineInfo;
    engineTab: 'service' | 'processlist' | 'switch_version' | 'config' | 'logs';
    serviceActionLoading: boolean;
    tuningLoading: boolean;
    switchingVersion: boolean;
    switchTargetVersion: string;
    switchBackupFirst: boolean;
    engineConfigPath: string;
    engineConfigContent: string;
    loadingEngineConfig: boolean;
    savingEngineConfig: boolean;
    engineLogsContent: string;
    engineLogsSource: string;
    loadingEngineLogs: boolean;
    processes: DatabaseProcessItem[];
    loadingProcesses: boolean;
    getEngineIcon: (engine: string) => string;
    onServiceAction: (action: 'start' | 'stop' | 'restart' | 'reload') => void;
    onApplyRamTuning: (profile: '1gb' | '2gb' | '4gb' | '8gb') => void;
    onFetchProcesses: () => void;
    onKillProcess: (pid: number) => void;
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
    tuningLoading,
    switchingVersion,
    switchTargetVersion = $bindable(),
    switchBackupFirst = $bindable(),
    engineConfigPath,
    engineConfigContent = $bindable(),
    loadingEngineConfig,
    savingEngineConfig,
    engineLogsContent,
    engineLogsSource,
    loadingEngineLogs,
    processes,
    loadingProcesses,
    getEngineIcon,
    onServiceAction,
    onApplyRamTuning,
    onFetchProcesses,
    onKillProcess,
    onSubmitSwitchVersion,
    onFetchEngineConfig,
    onSaveEngineConfig,
    onFetchEngineLogs,
    onClose
  }: Props = $props();

  const tunerProfiles: Array<{ id: '1gb' | '2gb' | '4gb' | '8gb'; label: string; desc: string }> = [
    { id: '1gb', label: 'Server 1 GB', desc: 'Buffer 256M · 80 conn' },
    { id: '2gb', label: 'Server 2 GB', desc: 'Buffer 512M · 150 conn' },
    { id: '4gb', label: 'Server 4 GB', desc: 'Buffer 1.5G · 250 conn' },
    { id: '8gb', label: 'Server 8 GB+', desc: 'Buffer 4G · 400 conn' },
  ];
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
        <p class="modal__subtitle">Layanan systemd, RAM Tuner, Processlist, Konfigurasi & Log Nyata</p>
      </div>
    </div>
  {/snippet}

  <div class="pg-databases-engine-manager-modal">
    <!-- Navigasi Tab Internal Engine Manager -->
    <div class="tabs tabs--underline engine-manager-tabs">
      <button
        type="button"
        class="tabs__tab"
        class:is-active={engineTab === 'service'}
        onclick={() => (engineTab = 'service')}
      >
        <i class="ri-dashboard-line" aria-hidden="true"></i>
        <span>Layanan & Tuning</span>
      </button>

      {#if activeEngine === 'mysql' || activeEngine === 'postgresql'}
        <button
          type="button"
          class="tabs__tab"
          class:is-active={engineTab === 'processlist'}
          onclick={() => {
            engineTab = 'processlist';
            onFetchProcesses();
          }}
        >
          <i class="ri-pulse-line" aria-hidden="true"></i>
          <span>Processlist</span>
        </button>
      {/if}

      <button
        type="button"
        class="tabs__tab"
        class:is-active={engineTab === 'config'}
        onclick={() => {
          engineTab = 'config';
          onFetchEngineConfig();
        }}
      >
        <i class="ri-file-settings-line" aria-hidden="true"></i>
        <span>Konfigurasi</span>
      </button>

      <button
        type="button"
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

      <button
        type="button"
        class="tabs__tab"
        class:is-active={engineTab === 'switch_version'}
        onclick={() => (engineTab = 'switch_version')}
      >
        <i class="ri-shuffle-line" aria-hidden="true"></i>
        <span>Ganti Versi</span>
      </button>
    </div>

    {#if engineTab === 'service'}
      <!-- Tab 1: Kontrol Layanan + 1-Klik RAM Tuner -->
      <div class="service-panel">
        <div class="service-status-box">
          <div class="status-indicator-large">
            <span class="status-dot-large {currentEngine.running ? 'online' : 'offline'}"></span>
            <div>
              <h4 class="service-status-heading">
                Status: {currentEngine.running ? 'Aktif (Running)' : 'Berhenti (Stopped)'}
              </h4>
              <p class="service-status-sub">
                Unit: <code>{currentEngine.service_unit || activeEngine + '.service'}</code> · Port: <code>{currentEngine.port}</code> · Versi: <code>{currentEngine.version || '-'}</code>
              </p>
            </div>
          </div>
        </div>

        <div class="service-actions-grid">
          <button
            type="button"
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
              type="button"
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
              type="button"
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
            type="button"
            class="service-action-card"
            disabled={serviceActionLoading}
            onclick={() => onServiceAction('reload')}
          >
            <i class="ri-refresh-line action-icon" aria-hidden="true"></i>
            <div class="action-info">
              <strong>Reload Konfigurasi</strong>
              <span>Muat ulang pengaturan layanan</span>
            </div>
          </button>
        </div>

        {#if activeEngine !== 'mongodb'}
          <div class="tuner-section">
            <h4 class="tuner-title">
              <i class="ri-speed-up-line" aria-hidden="true"></i>
              <span>1-Klik RAM Tuner ({currentEngine.display_name})</span>
            </h4>
            <p class="tuner-desc">
              Terapkan profil optimasi memori & koneksi optimal sesuai kapasitas RAM server Anda secara otomatis (menulis drop-in config & restart layanan).
            </p>
            <div class="tuner-grid">
              {#each tunerProfiles as prof (prof.id)}
                <button
                  type="button"
                  class="tuner-card"
                  disabled={tuningLoading || serviceActionLoading}
                  onclick={() => onApplyRamTuning(prof.id)}
                >
                  <strong>{prof.label}</strong>
                  <span>{prof.desc}</span>
                </button>
              {/each}
            </div>
          </div>
        {/if}
      </div>

    {:else if engineTab === 'processlist'}
      <!-- Tab 2: Processlist & Kill Query -->
      <div class="process-panel">
        <div class="config-panel-header">
          <span class="config-path-badge">Koneksi & Kueri Aktif ({processes.length} sesi)</span>
          <button
            type="button"
            class="btn btn-outline btn-sm"
            disabled={loadingProcesses}
            onclick={onFetchProcesses}
          >
            <i class={loadingProcesses ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
            <span>Segarkan</span>
          </button>
        </div>

        <div class="process-scroll">
          <table class="process-table">
            <thead>
              <tr>
                <th>PID</th>
                <th>User</th>
                <th>DB</th>
                <th>Durasi</th>
                <th>Status</th>
                <th>Kueri</th>
                <th>Aksi</th>
              </tr>
            </thead>
            <tbody>
              {#if processes.length === 0}
                <tr>
                  <td colspan="7">Tidak ada proses aktif selain sesi sistem.</td>
                </tr>
              {:else}
                {#each processes as proc (proc.pid)}
                  <tr>
                    <td><code>{proc.pid}</code></td>
                    <td>{proc.user}</td>
                    <td>{proc.db}</td>
                    <td>{proc.time_sec}d</td>
                    <td>{proc.command || proc.state}</td>
                    <td class="process-query" title={proc.query}>{proc.query || '-'}</td>
                    <td>
                      <button
                        type="button"
                        class="btn btn-outline btn-sm"
                        onclick={() => onKillProcess(proc.pid)}
                        title={`Hentikan proses PID ${proc.pid}`}
                      >
                        Kill
                      </button>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>

    {:else if engineTab === 'switch_version'}
      <!-- Tab 3: Ganti Versi -->
      <div class="switch-version-panel">
        <div class="version-warning-banner">
          <i class="ri-alert-fill warning-icon" aria-hidden="true"></i>
          <div>
            <strong>Perhatian Arsitektur Database:</strong>
            <p>
              FikPanel akan mengekspor cadangan penuh seluruh database (.sql.gz) sebelum menyelaraskan layanan ke versi target untuk menjaga integritas data.
            </p>
          </div>
        </div>

        <div class="form-group">
          <span class="form-label">Versi Saat Ini yang Aktif</span>
          <div class="current-version-badge">
            <i class="ri-checkbox-circle-fill active-check" aria-hidden="true"></i>
            <span>{currentEngine.display_name} <strong>{currentEngine.version || '-'}</strong></span>
          </div>
        </div>

        <div class="form-group">
          <label for="target-ver-select" class="form-label">Pilih Versi Target</label>
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

        <div class="checkbox-row">
          <label class="custom-checkbox-label">
            <input type="checkbox" bind:checked={switchBackupFirst} disabled={switchingVersion} />
            <span>
              <strong>Buat cadangan snapshot penuh otomatis</strong> sebelum beralih versi (Sangat Direkomendasikan).
            </span>
          </label>
        </div>

        <div class="switch-action-row">
          <button
            type="button"
            class="btn btn-primary"
            disabled={switchingVersion || switchTargetVersion === currentEngine.version}
            onclick={onSubmitSwitchVersion}
          >
            {#if switchingVersion}
              <i class="ri-loader-4-line spin" aria-hidden="true"></i>
              <span>Memproses Alih Versi...</span>
            {:else}
              <i class="ri-shuffle-line" aria-hidden="true"></i>
              <span>Beralih ke Versi Ini</span>
            {/if}
          </button>
        </div>
      </div>

    {:else if engineTab === 'config'}
      <!-- Tab 4: Konfigurasi Disk Aktual dengan CodeMirror -->
      <div class="config-panel">
        <div class="config-panel-header">
          <span class="config-path-badge">Berkas: <code>{engineConfigPath || '-'}</code></span>
          <button
            type="button"
            class="btn btn-outline btn-sm"
            disabled={loadingEngineConfig}
            onclick={onFetchEngineConfig}
          >
            <i class={loadingEngineConfig ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
            <span>Muat Ulang</span>
          </button>
        </div>

        <div class="config-editor-wrap">
          <CodeMirrorEditor
            bind:value={engineConfigContent}
            lang={activeEngine === 'mongodb' ? 'yaml' : 'ini'}
            minHeight="280px"
            maxHeight="360px"
            readonly={loadingEngineConfig || savingEngineConfig}
            onSave={onSaveEngineConfig}
          />
        </div>

        <div class="config-footer-row">
          <span class="config-hint">Salinan .bak dibuat otomatis sebelum menulis. Restart layanan setelah menyimpan.</span>
          <button
            type="button"
            class="btn btn-primary"
            disabled={savingEngineConfig || loadingEngineConfig}
            onclick={onSaveEngineConfig}
          >
            <i class={savingEngineConfig ? 'ri-loader-4-line spin' : 'ri-save-line'} aria-hidden="true"></i>
            <span>{savingEngineConfig ? 'Menyimpan...' : 'Simpan Konfigurasi'}</span>
          </button>
        </div>
      </div>

    {:else if engineTab === 'logs'}
      <!-- Tab 5: Log Galat Nyata (journalctl / error.log) -->
      <div class="logs-panel">
        <div class="config-panel-header">
          <span class="config-path-badge">Sumber Log: <code>{engineLogsSource || 'journalctl'}</code></span>
          <button
            type="button"
            class="btn btn-outline btn-sm"
            disabled={loadingEngineLogs}
            onclick={onFetchEngineLogs}
          >
            <i class={loadingEngineLogs ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
            <span>Segarkan Log</span>
          </button>
        </div>
        <pre class="logs-terminal-viewer">{engineLogsContent || 'Tidak ada catatan log galat.'}</pre>
      </div>
    {/if}
  </div>

  {#snippet footer()}
    <button
      type="button"
      class="btn btn-outline"
      disabled={switchingVersion}
      onclick={onClose}
    >
      Tutup
    </button>
  {/snippet}
</Modal>
