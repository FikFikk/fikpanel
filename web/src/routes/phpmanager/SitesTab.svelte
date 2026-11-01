<script lang="ts">
  import type { PHPVersionDetail, PHPVersionInfo } from '../../lib/types';
  import '../../styles/pages/phpmanager/sites-tab.css';

  // Antarmuka properti untuk tab situs terhubung pada modal PHP Manager
  interface Props {
    activeVersion: string;
    activeDetail: PHPVersionDetail | null;
    versions: PHPVersionInfo[];
    switchingSiteId: number | null;
    onSwitchSitePHP: (siteId: number, domain: string, newVersion: string) => void;
  }

  let {
    activeVersion,
    activeDetail,
    versions,
    switchingSiteId,
    onSwitchSitePHP,
  }: Props = $props();
</script>

<!-- Tab: Situs Terhubung (HestiaCP / aaPanel Style) -->
<div class="pg-phpmanager-sites-tab tab-pane">
  <div class="section-title-wrap flex justify-between items-center">
    <div>
      <h4 class="section-title">Situs Terhubung: PHP {activeVersion}</h4>
      <p class="section-desc">
        Daftar seluruh virtual host web di server yang mengeksekusi skrip PHP melalui socket php{activeVersion}-fpm.
      </p>
    </div>
    <a href="#/sites" class="btn btn-ghost btn-sm">
      <i class="ri-external-link-line"></i> Buka Menu Websites
    </a>
  </div>

  {#if activeDetail?.sites && activeDetail.sites.length > 0}
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>Domain Situs</th>
            <th>Pengguna Linux</th>
            <th>Direktori Root</th>
            <th>Status</th>
            <th>Ganti Versi PHP</th>
          </tr>
        </thead>
        <tbody>
          {#each activeDetail.sites as site (site.id)}
            <tr>
              <td>
                <div class="flex items-center gap-2">
                  <span class="dot {site.enabled ? 'dot--success' : ''}"></span>
                  <span class="domain-title font-semibold">{site.domain}</span>
                  <a href="http://{site.domain}" target="_blank" rel="noopener noreferrer" class="link-external" title="Kunjungi website">
                    <i class="ri-external-link-line"></i>
                  </a>
                </div>
              </td>
              <td>
                <span class="user-pill"><i class="ri-user-line"></i> {site.user}</span>
              </td>
              <td>
                <span class="mono text-xs text-subtle truncate max-w-xs" title={site.root_path}>{site.root_path}</span>
              </td>
              <td>
                {#if site.enabled}
                  <span class="badge badge--success">Aktif</span>
                {:else}
                  <span class="badge badge--neutral">Nonaktif</span>
                {/if}
              </td>
              <td>
                <div class="inline-flex items-center gap-2">
                  <select
                    class="form-select form-select-sm"
                    value={activeVersion}
                    onchange={(e) => onSwitchSitePHP(site.id, site.domain, e.currentTarget.value)}
                    disabled={switchingSiteId === site.id}
                  >
                    {#each versions.filter((ver) => ver.installed) as pv (pv.version)}
                      <option value={pv.version}>PHP {pv.version} {pv.running ? '(FPM Aktif)' : ''}</option>
                    {/each}
                  </select>
                  {#if switchingSiteId === site.id}
                    <i class="ri-loader-4-line spin text-accent"></i>
                  {/if}
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {:else}
    <div class="state">
      <i class="ri-global-line state__icon"></i>
      <h5 class="state__title">Belum Ada Situs Terhubung</h5>
      <p class="state__desc">
        Saat ini belum ada virtual host yang dikonfigurasi untuk mengeksekusi skrip melalui PHP {activeVersion}. Anda dapat memilih versi ini saat menambah situs baru atau mengalihkan situs yang ada di menu Websites.
      </p>
      <div class="state__actions">
        <a href="#/sites" class="btn btn-primary btn-sm">
          <i class="ri-arrow-right-line"></i> Buka Menu Websites
        </a>
      </div>
    </div>
  {/if}
</div>
