<script lang="ts">
  // Subkomponen tab One-Click Install (Template) pada halaman Docker Management Suite
  import '../../styles/pages/docker/template.css';
  import type { OneClickTemplate } from '../../lib/types';

  interface Props {
    filteredTemplates: OneClickTemplate[];
    loadingTemplates: boolean;
    templateCategory: string;
    onOpenDeployTemplate: (tmpl: OneClickTemplate) => void;
  }

  let {
    filteredTemplates,
    loadingTemplates,
    templateCategory = $bindable(),
    onOpenDeployTemplate,
  }: Props = $props();
</script>

<div class="pg-docker-template tab-pane">
  <div class="template-header">
    <div class="category-pills">
      <button
        class="pill {templateCategory === 'all' ? 'pill--active' : ''}"
        onclick={() => (templateCategory = 'all')}
      >
        Semua Template
      </button>
      <button
        class="pill {templateCategory === 'database' ? 'pill--active' : ''}"
        onclick={() => (templateCategory = 'database')}
      >
        Database & Cache
      </button>
      <button
        class="pill {templateCategory === 'cms' ? 'pill--active' : ''}"
        onclick={() => (templateCategory = 'cms')}
      >
        CMS & Web App
      </button>
      <button
        class="pill {templateCategory === 'tools' ? 'pill--active' : ''}"
        onclick={() => (templateCategory = 'tools')}
      >
        Management Tools
      </button>
      <button
        class="pill {templateCategory === 'monitoring' ? 'pill--active' : ''}"
        onclick={() => (templateCategory = 'monitoring')}
      >
        Monitoring
      </button>
    </div>
  </div>

  {#if loadingTemplates}
    <div class="text-center py-10">
      <i class="ri-loader-4-line spin text-muted text-2xl" aria-hidden="true"></i>
      <p class="text-muted mt-2">Memuat katalog aplikasi...</p>
    </div>
  {:else}
    <div class="templates-grid">
      {#each filteredTemplates as tmpl}
        <div class="template-card card">
          <div class="template-card-top">
            <div class="template-icon">
              <i class={tmpl.icon} aria-hidden="true"></i>
            </div>
            <div class="template-info">
              <h3 class="template-name">{tmpl.name}</h3>
              <span class="badge badge--secondary text-2xs">{tmpl.category}</span>
            </div>
          </div>

          <p class="template-desc">{tmpl.description}</p>

          <div class="template-meta">
            <span class="text-xs text-muted">
              <i class="ri-links-line" aria-hidden="true"></i>
              Port default: <strong class="font-mono">{tmpl.default_port}</strong>
            </span>
            <span class="text-xs text-muted font-mono">{tmpl.image}</span>
          </div>

          <div class="template-actions">
            <button class="btn btn--primary btn--sm w-full" onclick={() => onOpenDeployTemplate(tmpl)}>
              <i class="ri-install-line" aria-hidden="true"></i>
              <span>Pasang Sekarang</span>
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

