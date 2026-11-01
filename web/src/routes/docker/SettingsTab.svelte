<script lang="ts">
  // Subkomponen tab Settings beserta modal konfigurasi daemon pada halaman Docker Management Suite
  import '../../styles/pages/docker/settings.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type {
    DockerStatus,
    DockerSystemInfo,
    DockerDaemonConfig,
    DockerRegistry,
  } from '../../lib/types';

  interface Props {
    status: DockerStatus | null;
    systemInfo: DockerSystemInfo | null;
    daemonConfig: DockerDaemonConfig | null;
    registries: DockerRegistry[];
    actionLoading: boolean;
    savingConfig: boolean;
    showConfigModal: boolean;
    showLogRotationModal: boolean;
    rawJsonConfig: string;
    onControlDockerService: (action: 'start' | 'stop' | 'restart' | 'repair') => void;
    onSaveDaemonConfig: () => void;
    onSaveRawJsonConfig: () => void;
    onOpenLogRotationModal: () => void;
    onOpenConfigModal: () => void;
    onUninstallDocker: () => void;
  }

  let {
    status,
    systemInfo,
    daemonConfig = $bindable(),
    registries,
    actionLoading,
    savingConfig,
    showConfigModal = $bindable(),
    showLogRotationModal = $bindable(),
    rawJsonConfig = $bindable(),
    onControlDockerService,
    onSaveDaemonConfig,
    onSaveRawJsonConfig,
    onOpenLogRotationModal,
    onOpenConfigModal,
    onUninstallDocker,
  }: Props = $props();
</script>

<div class="pg-docker-settings tab-pane settings-suite">
  <div class="card settings-card">
    <div class="card__body settings-rows">
      <!-- 1. Service Status -->
      <div class="setting-row">
        <div class="setting-label">
          <span>Service status</span>
        </div>
        <div class="setting-ctrl">
          <div class="status-indicator-group">
            {#if status?.running}
              <span class="badge badge--success">
                <span class="status-dot status-dot--active"></span>
                Running
              </span>
            {:else}
              <span class="badge badge--danger">
                <span class="status-dot status-dot--danger"></span>
                Stopped
              </span>
            {/if}

            <button
              class="btn btn--secondary btn--sm"
              disabled={actionLoading}
              onclick={() => onControlDockerService('start')}
            >
              Start up
            </button>
            <button
              class="btn btn--secondary btn--sm"
              disabled={actionLoading}
              onclick={() => onControlDockerService('restart')}
            >
              Restart
            </button>
            <button
              class="btn btn--secondary btn--sm"
              disabled={actionLoading}
              onclick={() => onControlDockerService('repair')}
            >
              Repair
            </button>
          </div>
        </div>
      </div>

      <!-- 2. Accelerate URL (Registry Mirrors) -->
      <div class="setting-row">
        <div class="setting-label">
          <span>Accelerate URL</span>
        </div>
        <div class="setting-ctrl">
          <div class="mirror-box">
            <textarea
              class="input font-mono text-xs"
              rows="3"
              placeholder="https://hub-mirror.c.163.com&#10;https://mirror.baidubce.com&#10;Satu URL per baris..."
              value={daemonConfig?.registry_mirrors?.join('\n') || ''}
              onchange={(e) => {
                if (!daemonConfig) return;
                daemonConfig.registry_mirrors = e.currentTarget.value
                  .split('\n')
                  .map((s) => s.trim())
                  .filter(Boolean);
              }}
            ></textarea>
            <p class="setting-hint">Accelerate URL, one URL per line</p>
            <button
              class="btn btn--primary btn--sm mt-2"
              disabled={savingConfig}
              onclick={onSaveDaemonConfig}
            >
              Simpan Accelerate URL
            </button>
          </div>
        </div>
      </div>

      <!-- 3. Docker Compose Path -->
      <div class="setting-row">
        <div class="setting-label">
          <span>Docker compose path</span>
        </div>
        <div class="setting-ctrl">
          <div class="flex-row gap-2">
            <input
              type="text"
              class="input input--sm flex-1 font-mono"
              value={daemonConfig?.compose_path || '/usr/local/bin/docker-compose'}
              onchange={(e) => {
                if (daemonConfig) daemonConfig.compose_path = e.currentTarget.value.trim();
              }}
            />
            <button class="btn btn--secondary btn--sm" onclick={onSaveDaemonConfig}>
              Save
            </button>
          </div>
        </div>
      </div>

      <!-- 4. Private Registry Default -->
      <div class="setting-row">
        <div class="setting-label">
          <span>Private Registry</span>
        </div>
        <div class="setting-ctrl">
          <div class="flex-row gap-2">
            <select
              class="input input--sm flex-1"
              value={daemonConfig?.private_registry || 'official'}
              onchange={(e) => {
                if (daemonConfig) daemonConfig.private_registry = e.currentTarget.value;
                onSaveDaemonConfig();
              }}
            >
              <option value="official">Docker Hub Official (Default)</option>
              {#each registries as r}
                <option value={r.url}>{r.name} ({r.url})</option>
              {/each}
            </select>
          </div>
        </div>
      </div>

      <!-- 5. Log Rotation -->
      <div class="setting-row">
        <div class="setting-label">
          <span>Log Rotation</span>
        </div>
        <div class="setting-ctrl flex-row items-center gap-4">
          <label class="switch">
            <input
              type="checkbox"
              checked={daemonConfig?.log_rotation ?? true}
              onchange={(e) => {
                if (daemonConfig) {
                  daemonConfig.log_rotation = e.currentTarget.checked;
                  onSaveDaemonConfig();
                }
              }}
            />
            <span class="slider"></span>
          </label>
          <button
            class="btn btn--secondary btn--sm"
            onclick={onOpenLogRotationModal}
          >
            Modify
          </button>
          <span class="text-xs text-muted font-mono">
            max-size: {daemonConfig?.log_max_size || '10m'}, max-file: {daemonConfig?.log_max_file || '3'}
          </span>
        </div>
      </div>

      <!-- 6. IPv6 -->
      <div class="setting-row">
        <div class="setting-label">
          <span>IPv6</span>
        </div>
        <div class="setting-ctrl flex-row items-center gap-4">
          <label class="switch">
            <input
              type="checkbox"
              checked={daemonConfig?.ipv6 ?? false}
              onchange={(e) => {
                if (daemonConfig) {
                  daemonConfig.ipv6 = e.currentTarget.checked;
                  onSaveDaemonConfig();
                }
              }}
            />
            <span class="slider"></span>
          </label>
          <span class="text-xs text-muted font-mono">
            {daemonConfig?.ipv6_subnet || 'fd00::/80'}
          </span>
        </div>
      </div>

      <!-- 7. iptables -->
      <div class="setting-row">
        <div class="setting-label">
          <span>iptables</span>
        </div>
        <div class="setting-ctrl flex-row items-center gap-4">
          <label class="switch">
            <input
              type="checkbox"
              checked={daemonConfig?.iptables ?? true}
              onchange={(e) => {
                if (daemonConfig) {
                  daemonConfig.iptables = e.currentTarget.checked;
                  onSaveDaemonConfig();
                }
              }}
            />
            <span class="slider"></span>
          </label>
          <span class="text-xs text-muted">
            Aktifkan manajemen aturan paket iptables otomatis oleh Docker daemon
          </span>
        </div>
      </div>

      <!-- 8. live_restore -->
      <div class="setting-row">
        <div class="setting-label">
          <span>live_restore</span>
        </div>
        <div class="setting-ctrl flex-row items-center gap-4">
          <label class="switch">
            <input
              type="checkbox"
              checked={daemonConfig?.live_restore ?? true}
              onchange={(e) => {
                if (daemonConfig) {
                  daemonConfig.live_restore = e.currentTarget.checked;
                  onSaveDaemonConfig();
                }
              }}
            />
            <span class="slider"></span>
          </label>
          <span class="text-xs text-muted">
            Jaga kontainer tetap berjalan saat daemon Docker sedang restart atau crash
          </span>
        </div>
      </div>

      <!-- 9. Configuration file -->
      <div class="setting-row">
        <div class="setting-label">
          <span>Configuration file</span>
        </div>
        <div class="setting-ctrl">
          <button
            class="btn btn--secondary btn--sm"
            onclick={onOpenConfigModal}
          >
            <i class="ri-file-code-line" aria-hidden="true"></i>
            <span>Open configuration file</span>
          </button>
        </div>
      </div>

      <!-- 10. One-Click Install Dir -->
      <div class="setting-row">
        <div class="setting-label">
          <span>One-Click Install Dir</span>
        </div>
        <div class="setting-ctrl">
          <div class="flex-row gap-2">
            <input
              type="text"
              class="input input--sm flex-1 font-mono"
              value={daemonConfig?.one_click_dir || '/www/dk_project'}
              onchange={(e) => {
                if (daemonConfig) daemonConfig.one_click_dir = e.currentTarget.value.trim();
              }}
            />
            <button class="btn btn--secondary btn--sm" onclick={onSaveDaemonConfig}>
              Save
            </button>
          </div>
        </div>
      </div>

      <!-- 11. Uninstall Docker -->
      <div class="setting-row border-top-danger">
        <div class="setting-label">
          <span class="text-danger font-semibold">Uninstall Docker</span>
        </div>
        <div class="setting-ctrl">
          <button
            class="btn btn--danger btn--sm"
            disabled={actionLoading}
            onclick={onUninstallDocker}
          >
            <i class="ri-delete-bin-fill" aria-hidden="true"></i>
            <span>Uninstall Docker</span>
          </button>
        </div>
      </div>
    </div>
  </div>

  <!-- Tabel Spesifikasi Bawah Settings (Sesuai Screenshot aaPanel) -->
  <div class="card system-specs-card mt-4">
    <div class="card__head">
      <h2 class="card-title">
        <i class="ri-server-line" aria-hidden="true"></i>
        <span>Hardware & Environment Parameters</span>
      </h2>
    </div>
    <div class="card__body p-0">
      <div class="specs-grid">
        <div class="spec-cell">
          <span class="spec-k">Hostname</span>
          <span class="spec-v">{systemInfo?.hostname || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">System Version</span>
          <span class="spec-v">{systemInfo?.os_version || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Architecture</span>
          <span class="spec-v">{systemInfo?.architecture || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Kernel Version</span>
          <span class="spec-v">{systemInfo?.kernel_version || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">CPU Core</span>
          <span class="spec-v">{systemInfo?.cpu_cores || 1} Cores</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Memory Size</span>
          <span class="spec-v">{systemInfo?.memory_size || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Docker Version</span>
          <span class="spec-v">{systemInfo?.docker_version || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Docker Compose</span>
          <span class="spec-v">{systemInfo?.compose_version || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Unix Address</span>
          <span class="spec-v font-mono">{systemInfo?.unix_address || 'unix:///var/run/docker.sock'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Data Directory</span>
          <span class="spec-v font-mono">{systemInfo?.data_directory || '/var/lib/docker'}</span>
        </div>
      </div>
    </div>
  </div>

  <!-- Modal file konfigurasi /etc/docker/daemon.json -->
  <Modal
    open={showConfigModal}
    title="Configuration file (/etc/docker/daemon.json)"
    size="lg"
    onclose={() => (showConfigModal = false)}
  >
    <textarea
      class="json-editor-textarea font-mono text-xs"
      rows="16"
      bind:value={rawJsonConfig}
    ></textarea>

    {#snippet footer()}
      <button class="btn btn--secondary" onclick={() => (showConfigModal = false)}>
        Batal
      </button>
      <button class="btn btn--primary" disabled={savingConfig} onclick={onSaveRawJsonConfig}>
        <i class="ri-save-line" aria-hidden="true"></i>
        <span>Simpan & Terapkan Konfigurasi</span>
      </button>
    {/snippet}
  </Modal>

  <!-- Modal pengaturan rotasi log -->
  <Modal
    open={showLogRotationModal && Boolean(daemonConfig)}
    title="Modifikasi Log Rotation"
    size="md"
    onclose={() => (showLogRotationModal = false)}
  >
    {#if daemonConfig}
      <div class="docker-modal-form">
        <div class="field">
          <label class="field__label" for="log-max-size">Max Size per File (max-size)</label>
          <input
            id="log-max-size"
            type="text"
            class="input font-mono"
            placeholder="misal: 10m atau 50m"
            bind:value={daemonConfig.log_max_size}
          />
        </div>

        <div class="field">
          <label class="field__label" for="log-max-file">Jumlah File Rotasi (max-file)</label>
          <input
            id="log-max-file"
            type="text"
            class="input font-mono"
            placeholder="misal: 3 atau 5"
            bind:value={daemonConfig.log_max_file}
          />
        </div>
      </div>
    {/if}

    {#snippet footer()}
      <button class="btn btn--secondary" onclick={() => (showLogRotationModal = false)}>
        Batal
      </button>
      <button class="btn btn--primary" disabled={savingConfig} onclick={onSaveDaemonConfig}>
        <i class="ri-check-line" aria-hidden="true"></i>
        <span>Simpan Rotasi Log</span>
      </button>
    {/snippet}
  </Modal>
</div>
