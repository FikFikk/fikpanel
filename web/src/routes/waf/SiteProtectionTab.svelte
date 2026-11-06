<script lang="ts">
  import type { WAFSite } from '../../lib/types';
  import '../../styles/pages/waf/site-protection.css';

  // Props untuk tab Proteksi Situs WAF
  interface Props {
    siteSearch: string;
    loadingSites: boolean;
    filteredSites: WAFSite[];
    onLoadSites: () => void;
    onUpdateSite: (site: WAFSite) => void;
    onToggleSiteStatus: (site: WAFSite) => void;
    onOpenSiteLog: (domain: string) => void;
    onEditSite: (site: WAFSite) => void;
  }

  let {
    siteSearch = $bindable(),
    loadingSites,
    filteredSites,
    onLoadSites,
    onUpdateSite,
    onToggleSiteStatus,
    onOpenSiteLog,
    onEditSite,
  }: Props = $props();
</script>

<div class="pg-waf-site-protection bento-panel">
  <div class="panel-header-toolbar">
    <div class="left-tools">
      <div class="search-box">
        <i class="ri-search-line"></i>
        <input
          type="text"
          bind:value={siteSearch}
          placeholder="Cari nama situs..."
          class="search-input"
        />
      </div>
    </div>
    <button class="btn btn-secondary btn-sm" onclick={onLoadSites} disabled={loadingSites}>
      <i class="ri-refresh-line {loadingSites ? 'spin' : ''}"></i> Muat Ulang
    </button>
  </div>

  <div class="table-responsive">
    <table class="waf-main-table">
      <thead>
        <tr>
          <th>Site</th>
          <th>Total interceptions</th>
          <th>Fake spider</th>
          <th>Using CDN</th>
          <th>Smart CC defense</th>
          <th>Crawl Protection</th>
          <th>Waiting Room</th>
          <th>CC Defense</th>
          <th>Status</th>
          <th class="text-right">Operate</th>
        </tr>
      </thead>
      <tbody>
        {#if loadingSites}
          <tr>
            <td colspan="10" class="text-center p-6 text-muted">
              <i class="ri-loader-4-line spin text-lg"></i> Memuat daftar situs...
            </td>
          </tr>
        {:else if filteredSites.length === 0}
          <tr>
            <td colspan="10" class="text-center p-6 text-muted">
              Tidak ada situs yang cocok. Buat situs terlebih dahulu pada menu Website.
            </td>
          </tr>
        {:else}
          {#each filteredSites as site}
            <tr>
              <td class="font-bold text-accent mono">{site.site_name}</td>
              <td>
                <span class="badge-intercept-count">{site.total_interceptions}</span>
              </td>
              <td>
                <input
                  type="checkbox"
                  bind:checked={site.fake_spider}
                  onchange={() => onUpdateSite(site)}
                />
              </td>
              <td>
                <input
                  type="checkbox"
                  bind:checked={site.using_cdn}
                  onchange={() => onUpdateSite(site)}
                />
              </td>
              <td>
                <span class="badge-tag mono">{site.smart_cc} req/s</span>
              </td>
              <td>
                <span class="badge-pill {site.crawl_protection ? 'active' : 'disabled'}">
                  {site.crawl_protection ? 'Enable' : 'Disable'}
                </span>
              </td>
              <td>
                <span class="badge-pill {site.rate_limit ? 'active' : 'disabled'}">
                  {site.rate_limit ? 'Enable' : 'Disable'}
                </span>
              </td>
              <td>
                <span class="mono text-xs">{site.cc_defense} layer</span>
              </td>
              <td>
                <label class="switch" aria-label="Toggle WAF Site">
                  <input type="checkbox" checked={site.status} onchange={() => onToggleSiteStatus(site)} />
                  <span class="slider"></span>
                </label>
              </td>
              <td class="text-right actions-cell">
                <button
                  class="btn-link"
                  onclick={() => onOpenSiteLog(site.site_name)}
                >
                  Log
                </button>
                <button
                  class="btn-link"
                  onclick={() => onEditSite(site)}
                >
                  Modify
                </button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>

  <!-- Panduan CDN WAF -->
  <div class="info-footer-note">
    <i class="ri-information-line"></i>
    <span>Opsi CDN: Setelah mengaktifkan CDN/Cloudflare untuk situs, opsi "Using CDN" wajib dicentang agar IP proxy CDN tidak terblokir secara keliru oleh mitigasi CC.</span>
  </div>
</div>

