<script lang="ts">
  import type { SiteItem } from '../../lib/types';
  import '../../styles/pages/phpmanager/sites-list.css';

  type ManageTab = 'service' | 'sites' | 'config' | 'raw_ini' | 'fpm_pool' | 'raw_fpm' | 'disabled_func' | 'extensions' | 'opcache' | 'logs' | 'phpinfo';

  // Antarmuka properti untuk daftar ringkas situs terhubung pada kartu versi PHP
  interface Props {
    version: string;
    sites?: SiteItem[];
    sitesCount?: number;
    onOpenManageModal: (version: string, tab: ManageTab) => void;
  }

  let {
    version,
    sites = [],
    sitesCount = 0,
    onOpenManageModal,
  }: Props = $props();
</script>

<!-- Bagian Situs Terhubung (HestiaCP / aaPanel Style) -->
<div class="pg-phpmanager-sites-list connected-section">
  <div class="connected-header">
    <span class="connected-label">
      <i class="ri-global-line"></i> Situs Terhubung
    </span>
    {#if sitesCount > 0}
      <button
        type="button"
        class="site-count-badge"
        onclick={() => onOpenManageModal(version, 'sites')}
        title="Buka daftar situs terhubung"
      >
        {sitesCount} Situs
      </button>
    {:else}
      <span class="site-count-zero">0 Situs</span>
    {/if}
  </div>

  {#if sites && sites.length > 0}
    <div class="site-chips-list">
      {#each sites.slice(0, 3) as site (site.id)}
        <a
          href="#/sites"
          class="site-chip"
          title="Situs: {site.domain} | Pengguna: {site.user} | Path: {site.root_path}"
        >
          <span class="site-user-badge"><i class="ri-user-line"></i> {site.user}</span>
          <span class="site-domain-text">{site.domain}</span>
        </a>
      {/each}
      {#if sites.length > 3}
        <button
          type="button"
          class="site-chip site-chip-more"
          onclick={() => onOpenManageModal(version, 'sites')}
          title="Lihat semua {sites.length} situs yang terhubung"
        >
          +{sites.length - 3} lainnya…
        </button>
      {/if}
    </div>
  {:else}
    <p class="site-empty-hint">Belum ada situs yang terhubung</p>
  {/if}
</div>
