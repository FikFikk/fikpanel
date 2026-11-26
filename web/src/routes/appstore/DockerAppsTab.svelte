<script lang="ts">
  import type { DockerAppTemplate, DockerStatus } from '../../lib/types';

  // Props untuk tab Aplikasi Self-Hosted 1-Klik (Docker)
  interface Props {
    dockerStatus: DockerStatus | null;
    templates: DockerAppTemplate[];
    loading: boolean;
    onOpenDeployModal: (tpl: DockerAppTemplate) => void;
    onInstallDocker: () => void;
    onOpenDockerPage: () => void;
  }

  let {
    dockerStatus,
    templates,
    loading,
    onOpenDeployModal,
    onInstallDocker,
    onOpenDockerPage,
  }: Props = $props();

  let catFilter = $state('all');
  let search = $state('');

  let filteredTemplates = $derived(
    templates.filter((tpl) => {
      if (catFilter !== 'all' && tpl.category !== catFilter) return false;
      const q = search.trim().toLowerCase();
      if (!q) return true;
      return (
        tpl.name.toLowerCase().includes(q) ||
        tpl.description.toLowerCase().includes(q) ||
        tpl.image.toLowerCase().includes(q)
      );
    })
  );

  // Label kategori aplikasi kontainer
  function catLabel(cat: string): string {
    switch (cat) {
      case 'web': return 'Web & CMS';
      case 'database': return 'Database & Storage';
      case 'monitoring': return 'Monitoring';
      case 'tool': return 'Otomasi & Alat';
      default: return cat;
    }
  }
</script>

{#if dockerStatus && (!dockerStatus.installed || !dockerStatus.running)}
  <div class="docker-banner">
    <div class="docker-banner-info">
      <i class="ri-box-3-line" aria-hidden="true"></i>
      <div>
        <div class="table-app-name">
          {!dockerStatus.installed ? 'Docker Engine Belum Terpasang' : 'Daemon Docker Sedang Tidak Berjalan'}
        </div>
        <p class="app-card__desc">
          Aplikasi self-hosted 1-klik (WordPress, n8n, Gitea, MinIO, Grafana, Vaultwarden, Uptime Kuma) membutuhkan Docker CE &amp; Compose aktif.
        </p>
      </div>
    </div>
    <div class="app-card__actions">
      {#if !dockerStatus.installed}
        <button type="button" class="btn btn--primary btn--sm" onclick={onInstallDocker}>
          <i class="ri-download-2-line"></i> Pasang Docker CE Sekarang
        </button>
      {:else}
        <button type="button" class="btn btn--primary btn--sm" onclick={onOpenDockerPage}>
          <i class="ri-box-3-line"></i> Kelola di Menu Docker
        </button>
      {/if}
    </div>
  </div>
{/if}

<div class="toolbar-row">
  <div class="tabs tabs--pill" role="tablist" aria-label="Filter kategori aplikasi Docker">
    <button type="button" class="tabs__tab" class:is-active={catFilter === 'all'} onclick={() => (catFilter = 'all')}>
      Semua ({templates.length})
    </button>
    <button type="button" class="tabs__tab" class:is-active={catFilter === 'web'} onclick={() => (catFilter = 'web')}>
      Web &amp; CMS
    </button>
    <button type="button" class="tabs__tab" class:is-active={catFilter === 'tool'} onclick={() => (catFilter = 'tool')}>
      Otomasi &amp; Alat
    </button>
    <button type="button" class="tabs__tab" class:is-active={catFilter === 'database'} onclick={() => (catFilter = 'database')}>
      Database &amp; Storage
    </button>
    <button type="button" class="tabs__tab" class:is-active={catFilter === 'monitoring'} onclick={() => (catFilter = 'monitoring')}>
      Monitoring
    </button>
  </div>

  <div class="input-wrap">
    <i class="ri-search-line input__icon" aria-hidden="true"></i>
    <input
      type="text"
      bind:value={search}
      placeholder="Cari n8n, Gitea, MinIO, Grafana..."
      aria-label="Cari aplikasi Docker 1-klik"
      class="input input--search"
    />
  </div>
</div>

{#if loading && templates.length === 0}
  <div class="state state--loading">
    <span class="spinner" aria-hidden="true"></span>
    <p class="state__title">Memuat katalog aplikasi Docker 1-klik...</p>
  </div>
{:else if filteredTemplates.length === 0}
  <div class="state state--empty card">
    <i class="ri-box-3-line" aria-hidden="true"></i>
    <p class="state__title">Tidak ada template yang cocok</p>
  </div>
{:else}
  <div class="apps-grid">
    {#each filteredTemplates as tpl (tpl.id)}
      <article class="app-card card">
        <div class="app-card__header">
          <div class="app-card__icon-box">
            <i class="{tpl.icon} app-card__icon" aria-hidden="true"></i>
          </div>
          <div class="app-card__meta">
            <h3 class="app-card__name">{tpl.name}</h3>
            <span class="app-card__cat">{catLabel(tpl.category)}</span>
          </div>
        </div>

        <div class="docker-meta-list">
          <span class="ver-pill">{tpl.image}</span>
          {#each tpl.default_ports || [] as port}
            <span class="ver-pill ver-pill--size">Port {port}</span>
          {/each}
        </div>

        <p class="app-card__desc">{tpl.description}</p>

        <div class="app-card__footer">
          <span class="text-xs text-muted">Docker Compose</span>
          <button
            type="button"
            class="btn btn--primary btn--sm"
            onclick={() => onOpenDeployModal(tpl)}
            disabled={dockerStatus ? !dockerStatus.installed : false}
          >
            <i class="ri-rocket-line"></i> Deploy 1-Klik
          </button>
        </div>
      </article>
    {/each}
  </div>
{/if}
