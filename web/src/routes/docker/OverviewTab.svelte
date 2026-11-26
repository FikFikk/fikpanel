<script lang="ts">
  // Subkomponen tab Overview Docker Studio: KPI, visualisasi `docker system df`, System Prune, dan spesifikasi host
  import '../../styles/pages/docker/overview.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DockerStatus, DockerSystemInfo } from '../../lib/types';

  interface PrunePayload {
    containers: boolean;
    images: boolean;
    all_images: boolean;
    volumes: boolean;
    networks: boolean;
    build_cache: boolean;
  }

  interface Props {
    status: DockerStatus | null;
    systemInfo: DockerSystemInfo | null;
    actionLoading: boolean;
    onOpenCreateContainer: () => void;
    onOpenPullImage: () => void;
    onGoHubBuilder: () => void;
    onGoTemplate: () => void;
    onOpenComposeModal: () => void;
    onSystemPrune: (opts: PrunePayload) => Promise<void>;
  }

  let {
    status,
    systemInfo,
    actionLoading,
    onOpenCreateContainer,
    onOpenPullImage,
    onGoHubBuilder,
    onGoTemplate,
    onOpenComposeModal,
    onSystemPrune,
  }: Props = $props();

  let showPruneModal = $state(false);
  let pruneOpts = $state<PrunePayload>({
    containers: true,
    images: true,
    all_images: false,
    volumes: false,
    networks: true,
    build_cache: true,
  });

  function diskPct(part: number, total: number): number {
    if (!total || total <= 0 || !part || part <= 0) return 0;
    return Math.min(100, Math.max(4, Math.round((part / total) * 100)));
  }

  async function submitPrune() {
    await onSystemPrune(pruneOpts);
    showPruneModal = false;
  }
</script>

<div class="pg-docker-overview tab-pane overview-grid">
  <!-- 4 Kartu Metrik Utama -->
  <div class="stats-row">
    <div class="stat-card">
      <div class="stat-icon stat-icon--active">
        <i class="ri-play-circle-line" aria-hidden="true"></i>
      </div>
      <div class="stat-meta">
        <span class="stat-val">{status?.containers_running ?? 0} / {status?.containers_total ?? 0}</span>
        <span class="stat-lbl">Kontainer Aktif</span>
        <span class="stat-sub">{status?.containers_stopped ?? 0} berhenti · {status?.containers_paused ?? 0} jeda</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-icon stat-icon--info">
        <i class="ri-stack-line" aria-hidden="true"></i>
      </div>
      <div class="stat-meta">
        <span class="stat-val">{status?.images_total ?? 0}</span>
        <span class="stat-lbl">Local Images</span>
        <span class="stat-sub">{status?.disk_usage?.images?.size_human || '0 B'} di disk</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-icon stat-icon--violet">
        <i class="ri-hard-drive-2-line" aria-hidden="true"></i>
      </div>
      <div class="stat-meta">
        <span class="stat-val">{status?.volumes_total ?? 0}</span>
        <span class="stat-lbl">Docker Volumes</span>
        <span class="stat-sub">{status?.disk_usage?.volumes?.size_human || '0 B'} terpakai</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-icon stat-icon--accent">
        <i class="ri-git-merge-line" aria-hidden="true"></i>
      </div>
      <div class="stat-meta">
        <span class="stat-val">{status?.compose_stacks_total ?? 0}</span>
        <span class="stat-lbl">Compose Stacks</span>
        <span class="stat-sub">{status?.networks_total ?? 0} jaringan virtual</span>
      </div>
    </div>
  </div>

  <!-- Visualisasi Pemakaian Disk Docker (`docker system df`) -->
  <div class="card">
    <div class="card__head">
      <div>
        <h2 class="card-title">
          <i class="ri-pie-chart-2-line text-accent" aria-hidden="true"></i>
          <span>Pemakaian Ruang Disk Docker</span>
        </h2>
        {#if status?.disk_usage}
          <span class="text-xs text-muted">
            Total terpakai <strong class="font-mono">{status.disk_usage.total_size_human}</strong> ·
            Dapat dibebaskan <strong class="font-mono text-warning">{status.disk_usage.total_reclaimable_human}</strong>
          </span>
        {/if}
      </div>
      <button
        type="button"
        class="btn btn--danger btn--sm"
        disabled={actionLoading}
        onclick={() => (showPruneModal = true)}
      >
        <i class="ri-brush-line" aria-hidden="true"></i>
        <span>Bersihkan Disk (Prune)</span>
      </button>
    </div>
    <div class="card__body">
      {#if status?.disk_usage}
        {@const df = status.disk_usage}
        <div class="disk-grid">
          <div class="disk-item">
            <div class="disk-item-top">
              <span class="disk-item-name">Images ({df.images.total_count})</span>
              <span class="disk-item-size">{df.images.size_human}</span>
            </div>
            <div class="disk-bar">
              <div
                class="disk-bar-fill disk-bar-fill--info"
                style="--bar-pct: {diskPct(df.images.size_bytes, df.total_size_bytes)}%"
              ></div>
            </div>
            <div class="disk-item-meta">
              <span>{df.images.active_count} aktif dipakai</span>
              <span>Bebas: {df.images.reclaimable_human}</span>
            </div>
          </div>

          <div class="disk-item">
            <div class="disk-item-top">
              <span class="disk-item-name">Containers ({df.containers.total_count})</span>
              <span class="disk-item-size">{df.containers.size_human}</span>
            </div>
            <div class="disk-bar">
              <div
                class="disk-bar-fill"
                style="--bar-pct: {diskPct(df.containers.size_bytes, df.total_size_bytes)}%"
              ></div>
            </div>
            <div class="disk-item-meta">
              <span>{df.containers.active_count} berjalan</span>
              <span>Bebas: {df.containers.reclaimable_human}</span>
            </div>
          </div>

          <div class="disk-item">
            <div class="disk-item-top">
              <span class="disk-item-name">Local Volumes ({df.volumes.total_count})</span>
              <span class="disk-item-size">{df.volumes.size_human}</span>
            </div>
            <div class="disk-bar">
              <div
                class="disk-bar-fill disk-bar-fill--violet"
                style="--bar-pct: {diskPct(df.volumes.size_bytes, df.total_size_bytes)}%"
              ></div>
            </div>
            <div class="disk-item-meta">
              <span>{df.volumes.active_count} terpasang</span>
              <span>Bebas: {df.volumes.reclaimable_human}</span>
            </div>
          </div>

          <div class="disk-item">
            <div class="disk-item-top">
              <span class="disk-item-name">Build Cache ({df.build_cache.total_count})</span>
              <span class="disk-item-size">{df.build_cache.size_human}</span>
            </div>
            <div class="disk-bar">
              <div
                class="disk-bar-fill disk-bar-fill--warning"
                style="--bar-pct: {diskPct(df.build_cache.size_bytes, df.total_size_bytes)}%"
              ></div>
            </div>
            <div class="disk-item-meta">
              <span>{df.build_cache.active_count} aktif</span>
              <span>Bebas: {df.build_cache.reclaimable_human}</span>
            </div>
          </div>
        </div>
      {:else}
        <p class="text-xs text-muted">Data pemakaian disk akan tampil setelah Docker Engine berjalan.</p>
      {/if}
    </div>
  </div>

  <!-- Pintasan Operasi Cepat -->
  <div class="card">
    <div class="card__head">
      <h2 class="card-title">
        <i class="ri-flashlight-line text-accent" aria-hidden="true"></i>
        <span>Pintasan Operasi Cepat</span>
      </h2>
    </div>
    <div class="card__body shortcuts-bar">
      <button type="button" class="btn btn--primary btn--sm" onclick={onOpenCreateContainer}>
        <i class="ri-add-line" aria-hidden="true"></i>
        <span>Buat Kontainer</span>
      </button>
      <button type="button" class="btn btn--secondary btn--sm" onclick={onOpenPullImage}>
        <i class="ri-download-line" aria-hidden="true"></i>
        <span>Tarik Citra (Pull)</span>
      </button>
      <button type="button" class="btn btn--secondary btn--sm" onclick={onGoHubBuilder}>
        <i class="ri-hammer-line" aria-hidden="true"></i>
        <span>Build Dockerfile</span>
      </button>
      <button type="button" class="btn btn--secondary btn--sm" onclick={onGoTemplate}>
        <i class="ri-rocket-2-line" aria-hidden="true"></i>
        <span>App Store 1-Klik (15)</span>
      </button>
      <button type="button" class="btn btn--secondary btn--sm" onclick={onOpenComposeModal}>
        <i class="ri-file-code-line" aria-hidden="true"></i>
        <span>Stack Compose Baru</span>
      </button>
    </div>
  </div>

  <!-- Informasi Arsitektur & Spesifikasi Host -->
  <div class="card">
    <div class="card__head">
      <h2 class="card-title">
        <i class="ri-cpu-line text-accent" aria-hidden="true"></i>
        <span>Informasi Arsitektur & Spesifikasi Host</span>
      </h2>
    </div>
    <div class="card__body p-0">
      <div class="specs-grid">
        <div class="spec-cell">
          <span class="spec-k">Hostname</span>
          <span class="spec-v font-mono">{systemInfo?.hostname || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Sistem Operasi</span>
          <span class="spec-v">{systemInfo?.os_version || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Arsitektur</span>
          <span class="spec-v font-mono">{systemInfo?.architecture || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Kernel Version</span>
          <span class="spec-v font-mono">{systemInfo?.kernel_version || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">CPU Core</span>
          <span class="spec-v">{systemInfo?.cpu_cores || 1} Cores</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Total Memori Host</span>
          <span class="spec-v font-mono">{systemInfo?.memory_size || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Docker Engine</span>
          <span class="spec-v font-mono">{systemInfo?.docker_version || status?.version || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Docker Compose</span>
          <span class="spec-v font-mono">{systemInfo?.compose_version || status?.compose_version || '-'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Storage Driver</span>
          <span class="spec-v font-mono">{systemInfo?.storage_driver || status?.storage_driver || 'overlay2'}</span>
        </div>
        <div class="spec-cell">
          <span class="spec-k">Data Directory</span>
          <span class="spec-v font-mono">{systemInfo?.data_directory || status?.root_dir || '/var/lib/docker'}</span>
        </div>
      </div>
    </div>
  </div>

  <!-- Modal Pembersihan Disk Selektif (System Prune) -->
  <Modal open={showPruneModal} title="Bersihkan Ruang Disk Docker (System Prune)" size="md" onclose={() => (showPruneModal = false)}>
    <div class="prune-list">
      <label class="prune-option">
        <input type="checkbox" bind:checked={pruneOpts.containers} />
        <div class="prune-option-text">
          <strong class="text-sm">Kontainer Berhenti (Stopped)</strong>
          <span class="text-xs text-muted">Menghapus kontainer dengan status exited/dead yang tidak berjalan.</span>
        </div>
      </label>

      <label class="prune-option">
        <input type="checkbox" bind:checked={pruneOpts.images} />
        <div class="prune-option-text">
          <strong class="text-sm">Citra Dangling (&lt;none&gt;:&lt;none&gt;)</strong>
          <span class="text-xs text-muted">Menghapus layer citra yatim bekas build atau pembaruan citra.</span>
        </div>
      </label>

      <label class="prune-option">
        <input type="checkbox" bind:checked={pruneOpts.all_images} />
        <div class="prune-option-text">
          <strong class="text-sm text-warning">Semua Citra Tak Terpakai (Prune -a)</strong>
          <span class="text-xs text-muted">Menghapus seluruh citra lokal yang tidak sedang dipakai kontainer mana pun.</span>
        </div>
      </label>

      <label class="prune-option">
        <input type="checkbox" bind:checked={pruneOpts.build_cache} />
        <div class="prune-option-text">
          <strong class="text-sm">Docker Build Cache</strong>
          <span class="text-xs text-muted">Membersihkan cache layer hasil kompilasi Dockerfile.</span>
        </div>
      </label>

      <label class="prune-option">
        <input type="checkbox" bind:checked={pruneOpts.networks} />
        <div class="prune-option-text">
          <strong class="text-sm">Jaringan Tak Terpakai</strong>
          <span class="text-xs text-muted">Menghapus jaringan kustom yang tidak terhubung ke kontainer aktif.</span>
        </div>
      </label>

      <label class="prune-option">
        <input type="checkbox" bind:checked={pruneOpts.volumes} />
        <div class="prune-option-text">
          <strong class="text-sm text-danger">Volume Tidak Terpasang (Hati-hati)</strong>
          <span class="text-xs text-muted">Menghapus permanen volume data yang tidak sedang di-mount ke kontainer.</span>
        </div>
      </label>
    </div>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary" onclick={() => (showPruneModal = false)}>
        Batal
      </button>
      <button type="button" class="btn btn--danger" disabled={actionLoading} onclick={submitPrune}>
        <i class="ri-delete-bin-6-line" aria-hidden="true"></i>
        <span>Mulai Bersihkan Sekarang</span>
      </button>
    {/snippet}
  </Modal>
</div>
