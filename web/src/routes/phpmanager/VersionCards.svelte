<script lang="ts">
  import type { PHPVersionInfo } from '../../lib/types';
  import SitesList from './SitesList.svelte';
  import '../../styles/pages/phpmanager/version-cards.css';

  type ManageTab = 'service' | 'sites' | 'config' | 'raw_ini' | 'fpm_pool' | 'raw_fpm' | 'disabled_func' | 'extensions' | 'opcache' | 'logs' | 'phpinfo';

  // Antarmuka properti untuk grid kartu versi PHP
  interface Props {
    versions: PHPVersionInfo[];
    installingVer: string | null;
    restartingVer: string | null;
    onOpenManageModal: (version: string, tab: ManageTab) => void;
    onInstallPHP: (v: PHPVersionInfo) => void;
    onQuickRestartFPM: (version: string) => void;
  }

  let {
    versions,
    installingVer,
    restartingVer,
    onOpenManageModal,
    onInstallPHP,
    onQuickRestartFPM,
  }: Props = $props();

  // Pisahkan versi PHP terpasang dan yang tersedia agar runtime aktif tampil paling menonjol
  let installedVersions = $derived(versions.filter((v) => v.installed));
  let availableVersions = $derived(versions.filter((v) => !v.installed));
</script>

<!-- Grid Kartu Versi PHP Terpasang & Tersedia -->
<div class="pg-phpmanager-version-cards">
  {#if installedVersions.length > 0}
    <section class="php-section-block">
      <div class="php-section-head">
        <span class="php-section-title">
          <i class="ri-checkbox-circle-line" aria-hidden="true"></i>
          Runtime Terpasang ({installedVersions.length})
        </span>
      </div>
      <div class="php-versions-grid">
        {#each installedVersions as v (v.version)}
          <div class="v-card installed">
            <div class="v-head">
              <div class="flex items-center gap-2 flex-wrap">
                <span class="v-tag">PHP {v.version}</span>
                {#if v.running}
                  <span class="badge badge--success"><i class="ri-checkbox-circle-fill" aria-hidden="true"></i> RUNNING</span>
                {:else}
                  <span class="badge badge--danger"><i class="ri-stop-circle-line" aria-hidden="true"></i> STOPPED</span>
                {/if}
                {#if v.is_cli_default}
                  <span class="badge badge--info" title="Versi PHP CLI default (/usr/bin/php)">CLI Default</span>
                {/if}
              </div>
              <span class="fpm-badge">FPM</span>
            </div>

            <div class="v-meta">
              <div class="meta-row">
                <span class="meta-lbl">Socket:</span>
                <span class="mono meta-val truncate" title={v.fpm_socket}>{v.fpm_socket || '-'}</span>
              </div>
              <div class="meta-row">
                <span class="meta-lbl">Memory Limit:</span>
                <span class="meta-val font-semibold">{v.settings?.['memory_limit'] || '128M'}</span>
              </div>
              <div class="meta-row">
                <span class="meta-lbl">Upload Max / Exec:</span>
                <span class="meta-val">
                  {v.settings?.['upload_max_filesize'] || '2M'} · {v.settings?.['max_execution_time'] || '30'}s
                </span>
              </div>

              <!-- Bagian Situs Terhubung -->
              <SitesList
                version={v.version}
                sites={v.sites}
                sitesCount={v.sites_count}
                {onOpenManageModal}
              />
            </div>

            <div class="v-actions">
              <button
                type="button"
                class="btn btn--primary btn--sm flex-1"
                onclick={() => onOpenManageModal(v.version, 'config')}
                title="Kelola konfigurasi dan layanan PHP {v.version}"
              >
                <i class="ri-settings-4-line" aria-hidden="true"></i> Kelola
              </button>
              <button
                type="button"
                class="btn btn--ghost btn--sm"
                onclick={() => onOpenManageModal(v.version, 'raw_ini')}
                title="Buka Editor php.ini langsung"
              >
                <i class="ri-file-code-line" aria-hidden="true"></i> php.ini
              </button>
              <button
                type="button"
                class="btn btn--ghost btn--sm btn--icon"
                onclick={() => onQuickRestartFPM(v.version)}
                disabled={restartingVer === v.version}
                aria-label="Restart PHP {v.version} FPM"
                title="Restart layanan php{v.version}-fpm"
              >
                <i class="ri-restart-line {restartingVer === v.version ? 'spin' : ''}" aria-hidden="true"></i>
              </button>
            </div>
          </div>
        {/each}
      </div>
    </section>
  {/if}

  {#if availableVersions.length > 0}
    <section class="php-section-block">
      <div class="php-section-head">
        <span class="php-section-title php-section-title--muted">
          <i class="ri-download-cloud-2-line" aria-hidden="true"></i>
          Tersedia untuk Dipasang ({availableVersions.length})
        </span>
      </div>
      <div class="php-versions-grid">
        {#each availableVersions as v (v.version)}
          <div class="v-card not-installed">
            <div class="v-head">
              <div class="flex items-center gap-2">
                <span class="v-tag">PHP {v.version}</span>
                <span class="badge badge--neutral">BELUM TERPASANG</span>
              </div>
            </div>

            <div class="v-meta">
              <p class="text-subtle text-xs">
                Tersedia di repositori sistem operasi. Memasang suite FPM + CLI + ekstensi web inti via APT.
              </p>
            </div>

            <div class="v-actions">
              <button
                type="button"
                class="btn btn--ghost btn--sm w-full"
                onclick={() => onInstallPHP(v)}
                disabled={installingVer === v.version}
              >
                <i class="ri-download-2-line {installingVer === v.version ? 'spin' : ''}" aria-hidden="true"></i>
                {installingVer === v.version ? 'Memasang...' : `Pasang PHP ${v.version}`}
              </button>
            </div>
          </div>
        {/each}
      </div>
    </section>
  {/if}
</div>
