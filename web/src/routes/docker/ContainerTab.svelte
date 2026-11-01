<script lang="ts">
  // Subkomponen tab Container pada halaman Docker Management Suite
  import '../../styles/pages/docker/container.css';
  import type { DockerContainer } from '../../lib/types';

  interface Props {
    containers: DockerContainer[];
    filteredContainers: DockerContainer[];
    loadingContainers: boolean;
    actionLoading: boolean;
    containerFilter: 'all' | 'running' | 'stopped' | 'paused';
    containerSearch: string;
    onOpenCreateContainer: () => void;
    onLoadContainers: () => void;
    onControlContainer: (id: string, action: 'start' | 'stop' | 'restart' | 'pause' | 'unpause' | 'delete') => void;
    onOpenContainerLogs: (c: DockerContainer) => void;
  }

  let {
    containers,
    filteredContainers,
    loadingContainers,
    actionLoading,
    containerFilter = $bindable(),
    containerSearch = $bindable(),
    onOpenCreateContainer,
    onLoadContainers,
    onControlContainer,
    onOpenContainerLogs,
  }: Props = $props();
</script>

<div class="pg-docker-container tab-pane">
  <!-- Filter Toolbar -->
  <div class="toolbar">
    <div class="toolbar-left">
      <button class="btn btn--primary btn--sm" onclick={onOpenCreateContainer}>
        <i class="ri-add-line" aria-hidden="true"></i>
        <span>Buat Kontainer</span>
      </button>

      <div class="filter-pills" role="radiogroup" aria-label="Filter status kontainer">
        <button
          class="pill {containerFilter === 'all' ? 'pill--active' : ''}"
          onclick={() => (containerFilter = 'all')}
        >
          Semua ({containers.length})
        </button>
        <button
          class="pill {containerFilter === 'running' ? 'pill--active' : ''}"
          onclick={() => (containerFilter = 'running')}
        >
          Berjalan ({containers.filter((c) => c.state === 'running').length})
        </button>
        <button
          class="pill {containerFilter === 'stopped' ? 'pill--active' : ''}"
          onclick={() => (containerFilter = 'stopped')}
        >
          Berhenti ({containers.filter((c) => c.state === 'exited' || c.state === 'stopped').length})
        </button>
        <button
          class="pill {containerFilter === 'paused' ? 'pill--active' : ''}"
          onclick={() => (containerFilter = 'paused')}
        >
          Ditangguhkan ({containers.filter((c) => c.state === 'paused').length})
        </button>
      </div>
    </div>

    <div class="toolbar-right">
      <div class="search-box">
        <i class="ri-search-line search-icon" aria-hidden="true"></i>
        <input
          type="text"
          class="input input--sm"
          placeholder="Cari nama, citra, atau ID..."
          bind:value={containerSearch}
        />
      </div>
      <button
        class="btn btn--secondary btn--sm btn--icon"
        title="Segarkan daftar kontainer"
        aria-label="Segarkan daftar kontainer"
        onclick={onLoadContainers}
      >
        <i class="ri-refresh-line {loadingContainers ? 'spin' : ''}" aria-hidden="true"></i>
      </button>
    </div>
  </div>

  <!-- Tabel Kontainer -->
  <div class="table-wrap">
    <table class="table">
      <thead>
        <tr>
          <th class="col-w-140">Status</th>
          <th>Nama Kontainer</th>
          <th>Citra</th>
          <th>Port Mapping</th>
          <th class="col-w-120">CPU / Mem</th>
          <th class="col-w-140">Dibuat</th>
          <th class="col-w-200 text-right">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#if loadingContainers}
          <tr>
            <td colspan="7" class="text-center py-6">
              <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
              <p class="text-muted mt-2">Memuat daftar kontainer...</p>
            </td>
          </tr>
        {:else if filteredContainers.length === 0}
          <tr>
            <td colspan="7" class="text-center py-8">
              <div class="empty-state">
                <i class="ri-box-3-line empty-icon" aria-hidden="true"></i>
                <p class="empty-title">Tidak ada kontainer</p>
                <p class="empty-desc">Buat kontainer baru atau jalankan template aplikasi 1-klik untuk memulai.</p>
                <button class="btn btn--primary btn--sm mt-3" onclick={onOpenCreateContainer}>
                  <i class="ri-add-line" aria-hidden="true"></i>
                  <span>Tambah Kontainer Baru</span>
                </button>
              </div>
            </td>
          </tr>
        {:else}
          {#each filteredContainers as c}
            <tr>
              <td>
                {#if c.state === 'running'}
                  <span class="badge badge--success">
                    <span class="status-dot status-dot--active"></span>
                    Running
                  </span>
                {:else if c.state === 'paused'}
                  <span class="badge badge--warning">
                    <span class="status-dot status-dot--warning"></span>
                    Paused
                  </span>
                {:else}
                  <span class="badge badge--danger">
                    <span class="status-dot status-dot--danger"></span>
                    Exited
                  </span>
                {/if}
              </td>
              <td>
                <div class="cell-primary font-semibold">
                  {c.names[0]?.replace(/^\//, '') || c.id.slice(0, 12)}
                </div>
                <div class="cell-sub font-mono text-xs">{c.id.slice(0, 12)}</div>
              </td>
              <td>
                <span class="badge badge--secondary font-mono text-xs">{c.image}</span>
              </td>
              <td>
                {#if c.ports && c.ports.length > 0}
                  <div class="ports-list">
                    {#each c.ports as p}
                      {#if p.public_port > 0}
                        <a
                          href="http://localhost:{p.public_port}"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="port-chip port-chip--link"
                          title="Buka http://localhost:{p.public_port}"
                        >
                          <i class="ri-external-link-line" aria-hidden="true"></i>
                          {p.public_port}:{p.private_port}
                        </a>
                      {:else}
                        <span class="port-chip">{p.private_port}/{p.type}</span>
                      {/if}
                    {/each}
                  </div>
                {:else}
                  <span class="text-muted text-xs">-</span>
                {/if}
              </td>
              <td>
                <div class="text-xs font-mono">{c.cpu_percent.toFixed(1)}% CPU</div>
                <div class="text-xs text-muted font-mono">{c.mem_usage || '0 MB'}</div>
              </td>
              <td>
                <span class="text-xs text-muted">{new Date(c.created * 1000).toLocaleDateString()}</span>
              </td>
              <td class="text-right">
                <div class="action-btns-row">
                  {#if c.state === 'running'}
                    <button
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Hentikan (Stop)"
                      aria-label="Hentikan kontainer"
                      disabled={actionLoading}
                      onclick={() => onControlContainer(c.id, 'stop')}
                    >
                      <i class="ri-stop-fill text-warning" aria-hidden="true"></i>
                    </button>
                    <button
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Muat Ulang (Restart)"
                      aria-label="Muat ulang kontainer"
                      disabled={actionLoading}
                      onclick={() => onControlContainer(c.id, 'restart')}
                    >
                      <i class="ri-restart-line text-info" aria-hidden="true"></i>
                    </button>
                  {:else}
                    <button
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Mulai (Start)"
                      aria-label="Mulai kontainer"
                      disabled={actionLoading}
                      onclick={() => onControlContainer(c.id, 'start')}
                    >
                      <i class="ri-play-fill text-accent" aria-hidden="true"></i>
                    </button>
                  {/if}

                  <button
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Lihat Log Kontainer"
                    aria-label="Lihat log kontainer"
                    onclick={() => onOpenContainerLogs(c)}
                  >
                    <i class="ri-file-list-3-line" aria-hidden="true"></i>
                  </button>

                  <button
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Hapus Kontainer"
                    aria-label="Hapus kontainer"
                    disabled={actionLoading}
                    onclick={() => onControlContainer(c.id, 'delete')}
                  >
                    <i class="ri-delete-bin-6-line text-danger" aria-hidden="true"></i>
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>
