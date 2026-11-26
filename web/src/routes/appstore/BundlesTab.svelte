<script lang="ts">
  import type { StackBundle } from '../../lib/types';

  // Props untuk tab Stack Bundles 1-Klik
  interface Props {
    bundles: StackBundle[];
    loading: boolean;
    onInstallBundle: (bundle: StackBundle) => void;
    onViewBundleLog: (bundle: StackBundle) => void;
  }

  let { bundles, loading, onInstallBundle, onViewBundleLog }: Props = $props();
</script>

{#if loading && bundles.length === 0}
  <div class="state state--loading">
    <span class="spinner" aria-hidden="true"></span>
    <p class="state__title">Menganalisis kesiapan Stack Bundles di server...</p>
  </div>
{:else}
  <div class="bundles-grid">
    {#each bundles as bundle (bundle.id)}
      {@const pct = bundle.total_count > 0 ? Math.round((bundle.installed_count / bundle.total_count) * 100) : 0}
      <article class="bundle-card card {bundle.ready ? 'is-ready' : ''}">
        <div class="bundle-head">
          <div class="app-card__icon-box">
            <i class="{bundle.icon} app-card__icon" aria-hidden="true"></i>
          </div>
          <div class="app-card__meta">
            <div class="bundle-title-row">
              <h3 class="app-card__name">{bundle.name}</h3>
              <span class="badge badge--pill {bundle.ready ? 'badge--success' : 'badge--info'}">
                {bundle.ready ? 'Siap Produksi' : bundle.badge}
              </span>
            </div>
            <span class="bundle-tagline">{bundle.tagline}</span>
          </div>
        </div>

        <p class="app-card__desc">{bundle.description}</p>

        <div class="bundle-progress-wrap">
          <div class="bundle-progress-label">
            <span>Kesiapan Komponen</span>
            <span class="mono">{bundle.installed_count} / {bundle.total_count} terpasang ({pct}%)</span>
          </div>
          <div class="bundle-progress-bar">
            <div class="bundle-progress-fill" style="--bar-w: {pct}%"></div>
          </div>
        </div>

        <div class="bundle-chips">
          {#each bundle.components as comp (comp.id)}
            <span class="bundle-comp-chip {comp.installed ? 'is-installed' : ''}" title={comp.installed ? `Terpasang (${comp.version})` : 'Belum terpasang'}>
              <i class={comp.installed ? 'ri-checkbox-circle-fill' : 'ri-checkbox-blank-circle-line'} aria-hidden="true"></i>
              <span>{comp.name.split(' ')[0]}</span>
              {#if comp.installed && comp.version}
                <span class="mono">{comp.version}</span>
              {/if}
            </span>
          {/each}
        </div>

        <div class="app-card__footer">
          <span class="text-xs text-muted">
            {#if bundle.ready}
              Seluruh komponen bundel aktif
            {:else}
              Kurang {bundle.missing_app_ids.length} komponen
            {/if}
          </span>

          <div class="app-card__actions">
            <button
              type="button"
              class="btn btn--ghost btn--sm btn--icon"
              onclick={() => onViewBundleLog(bundle)}
              aria-label="Lihat log bundel {bundle.name}"
              title="Lihat Log Bundel"
            >
              <i class="ri-file-text-line"></i>
            </button>

            {#if bundle.ready}
              <span class="badge badge--success badge--pill">
                <i class="ri-check-double-line"></i> Lengkap
              </span>
            {:else}
              <button
                type="button"
                class="btn btn--primary btn--sm"
                onclick={() => onInstallBundle(bundle)}
              >
                <i class="ri-download-cloud-2-line"></i> Pasang ({bundle.missing_app_ids.length})
              </button>
            {/if}
          </div>
        </div>
      </article>
    {/each}
  </div>
{/if}
