<script lang="ts">
  import type { PHPVersionDetail } from '../../lib/types';
  import '../../styles/pages/phpmanager/service-tab.css';

  // Antarmuka properti untuk tab status dan kontrol layanan PHP-FPM
  interface Props {
    activeVersion: string;
    activeDetail: PHPVersionDetail | null;
    onControlService: (action: 'start' | 'stop' | 'restart' | 'reload') => void;
  }

  let {
    activeVersion,
    activeDetail,
    onControlService,
  }: Props = $props();
</script>

<!-- Tab 1: Service -->
<div class="pg-phpmanager-service-tab tab-pane">
  <div class="section-title-wrap">
    <h4 class="section-title">Status & Kontrol Layanan PHP-FPM</h4>
    <p class="section-desc">Kelola daemon FastCGI Process Manager untuk runtime PHP {activeVersion}.</p>
  </div>

  <div class="status-cards-grid">
    <div class="php-stat-box">
      <span class="stat-lbl">Daemon Service</span>
      <span class="stat-val mono">php{activeVersion}-fpm.service</span>
    </div>
    <div class="php-stat-box">
      <span class="stat-lbl">Master Process PID</span>
      <span class="stat-val mono">{activeDetail?.serviceStatus?.pid || 'Tidak aktif'}</span>
    </div>
    <div class="php-stat-box">
      <span class="stat-lbl">FastCGI Socket</span>
      <span class="stat-val mono text-xs truncate" title={activeDetail?.fpmSocket}>{activeDetail?.fpmSocket || '-'}</span>
    </div>
    <div class="php-stat-box">
      <span class="stat-lbl">Status Layanan</span>
      <span class="stat-val font-semibold {activeDetail?.running ? 'text-accent' : 'text-danger'}">
        {activeDetail?.running ? 'Aktif (Running)' : 'Berhenti (Stopped)'}
      </span>
    </div>
  </div>

  <div class="actions-group-card">
    <h5 class="group-title">Operasi Layanan Systemd</h5>
    <div class="flex flex-wrap gap-2">
      <button class="btn btn-primary" onclick={() => onControlService('restart')}>
        <i class="ri-restart-line"></i> Restart Layanan
      </button>
      <button class="btn btn-ghost" onclick={() => onControlService('reload')}>
        <i class="ri-refresh-line"></i> Reload Konfigurasi
      </button>
      {#if activeDetail?.running}
        <button class="btn btn-danger" onclick={() => onControlService('stop')}>
          <i class="ri-stop-circle-line"></i> Hentikan (Stop)
        </button>
      {:else}
        <button class="btn btn-primary" onclick={() => onControlService('start')}>
          <i class="ri-play-circle-line"></i> Jalankan (Start)
        </button>
      {/if}
    </div>
  </div>
</div>
