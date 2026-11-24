<script lang="ts">
  import type { PHPVersionDetail } from '../../lib/types';
  import '../../styles/pages/phpmanager/service-tab.css';

  // Antarmuka properti untuk tab status dan kontrol layanan PHP-FPM
  interface Props {
    activeVersion: string;
    activeDetail: PHPVersionDetail | null;
    settingDefaultCLI: boolean;
    onControlService: (action: 'start' | 'stop' | 'restart' | 'reload') => void;
    onSetDefaultCLI: (ver: string) => void;
  }

  let {
    activeVersion,
    activeDetail,
    settingDefaultCLI,
    onControlService,
    onSetDefaultCLI,
  }: Props = $props();
</script>

<!-- Tab 1: Service -->
<div class="pg-phpmanager-service-tab tab-pane">
  <div class="section-title-wrap">
    <h4 class="section-title">Status & Kontrol Layanan PHP-FPM</h4>
    <p class="section-desc">Kelola daemon FastCGI Process Manager dan versi CLI default untuk runtime PHP {activeVersion}.</p>
  </div>

  <div class="status-cards-grid">
    <div class="php-stat-box">
      <span class="stat-lbl">Daemon Service</span>
      <span class="stat-val mono">php{activeVersion}-fpm.service</span>
    </div>
    <div class="php-stat-box">
      <span class="stat-lbl">Master Process PID</span>
      <span class="stat-val mono">{activeDetail?.service_status?.pid || 'Tidak aktif'}</span>
    </div>
    <div class="php-stat-box">
      <span class="stat-lbl">Memori FPM (RSS)</span>
      <span class="stat-val mono">{activeDetail?.service_status?.memory || '-'}</span>
    </div>
    <div class="php-stat-box">
      <span class="stat-lbl">FastCGI Socket</span>
      <span class="stat-val mono text-xs truncate" title={activeDetail?.fpm_socket}>{activeDetail?.fpm_socket || '-'}</span>
    </div>
    <div class="php-stat-box">
      <span class="stat-lbl">Status Layanan</span>
      <span class="stat-val font-semibold {activeDetail?.running ? 'text-success' : 'text-danger'}">
        {activeDetail?.running ? 'Aktif (Running)' : 'Berhenti (Stopped)'}
      </span>
    </div>
    <div class="php-stat-box">
      <span class="stat-lbl">Binary CLI Default</span>
      <span class="stat-val font-semibold {activeDetail?.is_cli_default ? 'text-info' : 'text-muted'}">
        {activeDetail?.is_cli_default ? 'Aktif (/usr/bin/php)' : 'Bukan Default'}
      </span>
    </div>
  </div>

  <div class="actions-group-card">
    <h5 class="group-title">Operasi Layanan Systemd & CLI</h5>
    <div class="flex flex-wrap gap-2">
      <button type="button" class="btn btn--primary btn--sm" onclick={() => onControlService('restart')}>
        <i class="ri-restart-line" aria-hidden="true"></i> Restart Layanan
      </button>
      <button type="button" class="btn btn--ghost btn--sm" onclick={() => onControlService('reload')}>
        <i class="ri-refresh-line" aria-hidden="true"></i> Reload Konfigurasi
      </button>
      {#if !activeDetail?.is_cli_default}
        <button
          type="button"
          class="btn btn--ghost btn--sm"
          onclick={() => onSetDefaultCLI(activeVersion)}
          disabled={settingDefaultCLI}
        >
          <i class="ri-terminal-line" aria-hidden="true"></i>
          {settingDefaultCLI ? 'Menyetel...' : `Jadikan CLI Default (php${activeVersion})`}
        </button>
      {/if}
      {#if activeDetail?.running}
        <button type="button" class="btn btn--danger btn--sm" onclick={() => onControlService('stop')}>
          <i class="ri-stop-circle-line" aria-hidden="true"></i> Hentikan (Stop)
        </button>
      {:else}
        <button type="button" class="btn btn--primary btn--sm" onclick={() => onControlService('start')}>
          <i class="ri-play-circle-line" aria-hidden="true"></i> Jalankan (Start)
        </button>
      {/if}
    </div>
  </div>
</div>
