<script lang="ts">
  // Subkomponen tab Kontainer Docker Studio: Bulk actions, Terminal Exec, Inspect, Live Logs, dan host-aware port links
  import '../../styles/pages/docker/container.css';
  import type { DockerContainer } from '../../lib/types';

  interface Props {
    containers: DockerContainer[];
    filteredContainers: DockerContainer[];
    loadingContainers: boolean;
    actionLoading: boolean;
    containerFilter: 'all' | 'running' | 'exited' | 'paused';
    containerSearch: string;
    onOpenCreateContainer: () => void;
    onLoadContainers: () => void;
    onControlContainer: (c: DockerContainer, action: 'start' | 'stop' | 'restart' | 'pause' | 'unpause' | 'remove') => void;
    onBulkContainers: (ids: string[], action: 'start' | 'stop' | 'restart' | 'remove') => Promise<void>;
    onOpenContainerLogs: (c: DockerContainer) => void;
    onOpenContainerShell: (c: DockerContainer) => void;
    onOpenContainerInspect: (c: DockerContainer) => void;
    formatDate: (ts: number) => string;
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
    onBulkContainers,
    onOpenContainerLogs,
    onOpenContainerShell,
    onOpenContainerInspect,
    formatDate,
  }: Props = $props();

  let selectedIds = $state<string[]>([]);

  function getContainerName(c: DockerContainer): string {
    const raw = c.name || c.names?.[0] || c.id.slice(0, 12);
    return raw.replace(/^\//, '');
  }

  function portHref(publicPort: number): string {
    const host = typeof window !== 'undefined' ? window.location.hostname : 'localhost';
    const proto = publicPort === 443 || publicPort === 8443 || publicPort === 9443 ? 'https' : 'http';
    return `${proto}://${host}:${publicPort}`;
  }

  let allVisibleSelected = $derived(
    filteredContainers.length > 0 && filteredContainers.every((c) => selectedIds.includes(c.id)),
  );

  function toggleSelectAll() {
    if (allVisibleSelected) {
      selectedIds = [];
    } else {
      selectedIds = filteredContainers.map((c) => c.id);
    }
  }

  function toggleOne(id: string) {
    if (selectedIds.includes(id)) {
      selectedIds = selectedIds.filter((x) => x !== id);
    } else {
      selectedIds = [...selectedIds, id];
    }
  }

  async function runBulk(action: 'start' | 'stop' | 'restart' | 'remove') {
    if (selectedIds.length === 0) return;
    await onBulkContainers(selectedIds, action);
    selectedIds = [];
  }
</script>

<div class="pg-docker-container tab-pane">
  <div class="toolbar">
    <div class="toolbar-left">
      <button type="button" class="btn btn--primary btn--sm" onclick={onOpenCreateContainer}>
        <i class="ri-add-line" aria-hidden="true"></i>
        <span>Buat Kontainer</span>
      </button>

      <div class="filter-pills">
        <button
          type="button"
          class="pill {containerFilter === 'all' ? 'pill--active' : ''}"
          onclick={() => (containerFilter = 'all')}
        >
          Semua ({containers.length})
        </button>
        <button
          type="button"
          class="pill {containerFilter === 'running' ? 'pill--active' : ''}"
          onclick={() => (containerFilter = 'running')}
        >
          Berjalan ({containers.filter((c) => c.state === 'running').length})
        </button>
        <button
          type="button"
          class="pill {containerFilter === 'exited' ? 'pill--active' : ''}"
          onclick={() => (containerFilter = 'exited')}
        >
          Berhenti ({containers.filter((c) => c.state !== 'running' && c.state !== 'paused').length})
        </button>
        <button
          type="button"
          class="pill {containerFilter === 'paused' ? 'pill--active' : ''}"
          onclick={() => (containerFilter = 'paused')}
        >
          Jeda ({containers.filter((c) => c.state === 'paused').length})
        </button>
      </div>
    </div>

    <div class="toolbar-right">
      <div class="search-box">
        <i class="ri-search-line search-icon" aria-hidden="true"></i>
        <input
          type="text"
          class="input input--sm"
          placeholder="Cari nama, citra, stack, ID..."
          aria-label="Cari kontainer"
          bind:value={containerSearch}
        />
      </div>
      <button
        type="button"
        class="btn btn--secondary btn--sm btn--icon"
        title="Segarkan daftar kontainer"
        aria-label="Segarkan daftar kontainer"
        onclick={onLoadContainers}
      >
        <i class="ri-refresh-line {loadingContainers ? 'spin' : ''}" aria-hidden="true"></i>
      </button>
    </div>
  </div>

  {#if selectedIds.length > 0}
    <div class="bulk-bar">
      <span class="text-xs font-semibold">
        {selectedIds.length} kontainer terpilih
      </span>
      <div class="bulk-actions">
        <button type="button" class="btn btn--secondary btn--sm" disabled={actionLoading} onclick={() => runBulk('start')}>
          <i class="ri-play-fill text-success" aria-hidden="true"></i>
          <span>Start</span>
        </button>
        <button type="button" class="btn btn--secondary btn--sm" disabled={actionLoading} onclick={() => runBulk('stop')}>
          <i class="ri-stop-fill text-warning" aria-hidden="true"></i>
          <span>Stop</span>
        </button>
        <button type="button" class="btn btn--secondary btn--sm" disabled={actionLoading} onclick={() => runBulk('restart')}>
          <i class="ri-restart-line text-info" aria-hidden="true"></i>
          <span>Restart</span>
        </button>
        <button type="button" class="btn btn--danger btn--sm" disabled={actionLoading} onclick={() => runBulk('remove')}>
          <i class="ri-delete-bin-6-line" aria-hidden="true"></i>
          <span>Hapus</span>
        </button>
        <button type="button" class="btn btn--ghost btn--sm" onclick={() => (selectedIds = [])}>
          Batal
        </button>
      </div>
    </div>
  {/if}

  <div class="table-wrap">
    <table class="table">
      <thead>
        <tr>
          <th class="col-check">
            <input
              type="checkbox"
              aria-label="Pilih semua kontainer"
              checked={allVisibleSelected}
              onchange={toggleSelectAll}
            />
          </th>
          <th class="col-w-100">Status</th>
          <th>Kontainer</th>
          <th>Citra (Image)</th>
          <th>Port Mapping</th>
          <th>CPU / RAM / Net</th>
          <th>Dibuat</th>
          <th class="col-w-240 text-right">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#if loadingContainers}
          <tr>
            <td colspan="8" class="text-center py-6">
              <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
              <p class="text-muted mt-2">Memuat daftar kontainer & metrik live...</p>
            </td>
          </tr>
        {:else if filteredContainers.length === 0}
          <tr>
            <td colspan="8" class="text-center py-8">
              <div class="empty-state">
                <i class="ri-inbox-2-line empty-icon" aria-hidden="true"></i>
                <p class="empty-title">Tidak ada kontainer ditemukan</p>
                <p class="empty-desc">Buat kontainer baru atau deploy aplikasi instan dari tab App Store 1-Klik.</p>
                <button type="button" class="btn btn--primary btn--sm mt-3" onclick={onOpenCreateContainer}>
                  <i class="ri-add-line" aria-hidden="true"></i>
                  <span>Buat Kontainer Pertama</span>
                </button>
              </div>
            </td>
          </tr>
        {:else}
          {#each filteredContainers as c (c.id)}
            {@const cName = getContainerName(c)}
            <tr>
              <td class="col-check">
                <input
                  type="checkbox"
                  aria-label="Pilih kontainer {cName}"
                  checked={selectedIds.includes(c.id)}
                  onchange={() => toggleOne(c.id)}
                />
              </td>
              <td>
                {#if c.state === 'running'}
                  <span class="badge badge--success">
                    <span class="status-dot status-dot--active"></span>
                    Running
                  </span>
                {:else if c.state === 'paused'}
                  <span class="badge badge--warning">Paused</span>
                {:else}
                  <span class="badge badge--secondary">{c.state || 'exited'}</span>
                {/if}
              </td>
              <td>
                <div class="cnt-name-row">
                  <strong class="text-sm font-semibold">{cName}</strong>
                  {#if c.compose_project}
                    <span class="badge badge--info text-2xs">{c.compose_project}</span>
                  {/if}
                </div>
                <div class="cnt-sub-meta">
                  <span class="font-mono">{c.id.slice(0, 12)}</span>
                  <span>·</span>
                  <span>{c.status}</span>
                </div>
              </td>
              <td>
                <span class="font-mono text-xs text-secondary">{c.image}</span>
              </td>
              <td>
                {#if c.ports && c.ports.length > 0}
                  <div class="ports-list">
                    {#each c.ports as p}
                      {#if p.public_port > 0}
                        <a
                          href={portHref(p.public_port)}
                          target="_blank"
                          rel="noopener noreferrer"
                          class="port-chip port-chip--link"
                          title="Buka di tab baru"
                        >
                          {p.public_port}:{p.private_port}/{p.type}
                          <i class="ri-external-link-line" aria-hidden="true"></i>
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
                <div class="telemetry-cell">
                  <span>CPU {c.cpu_percent ? `${c.cpu_percent.toFixed(1)}%` : '0.0%'}</span>
                  <span class="text-muted">{c.mem_usage || '-'}</span>
                  {#if c.net_io}
                    <span class="text-muted">Net: {c.net_io}</span>
                  {/if}
                </div>
              </td>
              <td>
                <span class="text-xs text-muted">{c.created_since || formatDate(c.created)}</span>
              </td>
              <td class="text-right">
                <div class="action-btns-row">
                  {#if c.state === 'running'}
                    <button
                      type="button"
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Terminal Shell (docker exec -it)"
                      aria-label="Terminal Shell Kontainer"
                      onclick={() => onOpenContainerShell(c)}
                    >
                      <i class="ri-terminal-line text-accent" aria-hidden="true"></i>
                    </button>
                  {/if}
                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Lihat & Stream Log"
                    aria-label="Lihat Log Kontainer"
                    onclick={() => onOpenContainerLogs(c)}
                  >
                    <i class="ri-file-list-3-line" aria-hidden="true"></i>
                  </button>
                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Inspeksi & Recreate"
                    aria-label="Inspeksi Kontainer"
                    onclick={() => onOpenContainerInspect(c)}
                  >
                    <i class="ri-search-eye-line" aria-hidden="true"></i>
                  </button>

                  {#if c.state === 'running'}
                    <button
                      type="button"
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Hentikan (Stop)"
                      aria-label="Hentikan Kontainer"
                      disabled={actionLoading}
                      onclick={() => onControlContainer(c, 'stop')}
                    >
                      <i class="ri-stop-fill text-warning" aria-hidden="true"></i>
                    </button>
                    <button
                      type="button"
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Restart"
                      aria-label="Restart Kontainer"
                      disabled={actionLoading}
                      onclick={() => onControlContainer(c, 'restart')}
                    >
                      <i class="ri-restart-line text-info" aria-hidden="true"></i>
                    </button>
                  {:else}
                    <button
                      type="button"
                      class="btn btn--ghost btn--sm btn--icon"
                      title="Jalankan (Start)"
                      aria-label="Jalankan Kontainer"
                      disabled={actionLoading}
                      onclick={() => onControlContainer(c, 'start')}
                    >
                      <i class="ri-play-fill text-success" aria-hidden="true"></i>
                    </button>
                  {/if}

                  <button
                    type="button"
                    class="btn btn--ghost btn--sm btn--icon"
                    title="Hapus Kontainer"
                    aria-label="Hapus Kontainer"
                    disabled={actionLoading}
                    onclick={() => onControlContainer(c, 'remove')}
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
