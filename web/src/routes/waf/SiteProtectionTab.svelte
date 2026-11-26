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
    <div class="search-input-wrap">
      <i class="ri-search-line search-icon"></i>
      <input
        type="text"
        bind:value={siteSearch}
        placeholder="Cari nama domain situs..."
        aria-label="Cari nama domain situs"
        class="input-dark search-input"
      />
    </div>
    <button type="button" class="btn btn--secondary btn--sm" onclick={onLoadSites} disabled={loadingSites}>
      <i class="ri-refresh-line {loadingSites ? 'spin' : ''}"></i> Muat Ulang
    </button>
  </div>

  <div class="table-responsive">
    <table class="waf-main-table">
      <thead>
        <tr>
          <th>Domain Situs</th>
          <th>Total Intersepsi</th>
          <th>Fake Spider</th>
          <th>Pakai CDN</th>
          <th>Smart CC</th>
          <th>Anti-Crawl</th>
          <th>Rate Limit</th>
          <th>Status WAF</th>
          <th class="text-right">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#if loadingSites}
          <tr>
            <td colspan="9" class="text-center text-muted">
              <i class="ri-loader-4-line spin"></i> Memuat daftar situs...
            </td>
          </tr>
        {:else if filteredSites.length === 0}
          <tr>
            <td colspan="9" class="text-center text-muted">
              Tidak ada situs yang cocok. Tambahkan situs pada menu Website terlebih dahulu.
            </td>
          </tr>
        {:else}
          {#each filteredSites as site}
            <tr>
              <td class="text-accent mono font-semibold">{site.site_name}</td>
              <td>
                <span class="badge-intercept-count">{site.total_interceptions}</span>
              </td>
              <td>
                <input
                  type="checkbox"
                  aria-label="Blokir Fake Spider untuk {site.site_name}"
                  bind:checked={site.fake_spider}
                  onchange={() => onUpdateSite(site)}
                />
              </td>
              <td>
                <input
                  type="checkbox"
                  aria-label="Situs {site.site_name} memakai CDN Cloudflare"
                  bind:checked={site.using_cdn}
                  onchange={() => onUpdateSite(site)}
                />
              </td>
              <td>
                <span class="badge-tag mono">{site.smart_cc} req/s</span>
              </td>
              <td>
                <span class="badge-pill {site.crawl_protection ? 'active' : 'disabled'}">
                  {site.crawl_protection ? 'Aktif' : 'Nonaktif'}
                </span>
              </td>
              <td>
                <span class="badge-pill {site.rate_limit ? 'active' : 'disabled'}">
                  {site.rate_limit ? 'Aktif' : 'Nonaktif'}
                </span>
              </td>
              <td>
                <label class="switch" aria-label="Toggle WAF untuk {site.site_name}">
                  <input type="checkbox" checked={site.status} onchange={() => onToggleSiteStatus(site)} />
                  <span class="slider"></span>
                </label>
              </td>
              <td class="text-right nowrap">
                <button type="button" class="btn-link" onclick={() => onOpenSiteLog(site.site_name)}>
                  Log
                </button>
                <button type="button" class="btn-link" onclick={() => onEditSite(site)}>
                  Ubah
                </button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>

  <div class="info-footer-note">
    <i class="ri-information-line"></i>
    <span>Saat opsi <strong>Pakai CDN</strong> dicentang pada salah satu situs, FikPanel otomatis menulis rentang <code class="mono">set_real_ip_from</code> Cloudflare &amp; <code class="mono">real_ip_header CF-Connecting-IP</code> ke konfigurasi Nginx agar IP asli pengunjung selalu terdeteksi.</span>
  </div>
</div>
