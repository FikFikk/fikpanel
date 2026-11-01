<script lang="ts">
  // Subkomponen tab Overview pada halaman Docker Management Suite
  import '../../styles/pages/docker/overview.css';
  import type { DockerStatus, DockerSystemInfo } from '../../lib/types';

  interface Props {
    status: DockerStatus | null;
    systemInfo: DockerSystemInfo | null;
    onOpenCreateContainer: () => void;
    onOpenPullImage: () => void;
    onSwitchToTemplate: () => void;
    onOpenCompose: () => void;
    onPruneVolumes: () => void;
  }

  let {
    status,
    systemInfo,
    onOpenCreateContainer,
    onOpenPullImage,
    onSwitchToTemplate,
    onOpenCompose,
    onPruneVolumes,
  }: Props = $props();
</script>

<div class="pg-docker-overview">
  <div class="overview-grid">
    <!-- Statistik Kartu Metrik -->
    <div class="stats-row">
      <div class="stat-card">
        <div class="stat-icon stat-icon--active">
          <i class="ri-play-circle-line" aria-hidden="true"></i>
        </div>
        <div class="stat-meta">
          <span class="stat-val">{status?.containers_running || 0}</span>
          <span class="stat-lbl">Running Containers</span>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon stat-icon--danger">
          <i class="ri-stop-circle-line" aria-hidden="true"></i>
        </div>
        <div class="stat-meta">
          <span class="stat-val">{status?.containers_stopped || 0}</span>
          <span class="stat-lbl">Stopped Containers</span>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon stat-icon--info">
          <i class="ri-hard-drive-line" aria-hidden="true"></i>
        </div>
        <div class="stat-meta">
          <span class="stat-val">{status?.images_total || 0}</span>
          <span class="stat-lbl">Local Images</span>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon stat-icon--violet">
          <i class="ri-database-2-line" aria-hidden="true"></i>
        </div>
        <div class="stat-meta">
          <span class="stat-val">{status?.volumes_total || 0}</span>
          <span class="stat-lbl">Mounted Volumes</span>
        </div>
      </div>
    </div>

    <!-- Quick Actions & Pintasan Cepat -->
    <div class="card quick-actions-card">
      <div class="card__head">
        <h2 class="card-title">
          <i class="ri-flashlight-line" aria-hidden="true"></i>
          <span>Pintasan Operasi Cepat</span>
        </h2>
      </div>
      <div class="card__body shortcuts-bar">
        <button class="btn btn--primary btn--sm" onclick={onOpenCreateContainer}>
          <i class="ri-add-line" aria-hidden="true"></i>
          <span>Buat Kontainer</span>
        </button>
        <button class="btn btn--secondary btn--sm" onclick={onOpenPullImage}>
          <i class="ri-download-line" aria-hidden="true"></i>
          <span>Tarik Citra (Pull)</span>
        </button>
        <button class="btn btn--secondary btn--sm" onclick={onSwitchToTemplate}>
          <i class="ri-magic-line" aria-hidden="true"></i>
          <span>Aplikasi 1-Klik</span>
        </button>
        <button class="btn btn--secondary btn--sm" onclick={onOpenCompose}>
          <i class="ri-file-code-line" aria-hidden="true"></i>
          <span>Stack Compose Baru</span>
        </button>
        <button class="btn btn--danger btn--sm" onclick={onPruneVolumes}>
          <i class="ri-delete-bin-line" aria-hidden="true"></i>
          <span>Bersihkan Volume Usang</span>
        </button>
      </div>
    </div>

    <!-- Host System Specs Table (Mirip aaPanel bottom table) -->
    <div class="card system-specs-card">
      <div class="card__head">
        <h2 class="card-title">
          <i class="ri-server-line" aria-hidden="true"></i>
          <span>Informasi Arsitektur & Spesifikasi Host</span>
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
  </div>
</div>

