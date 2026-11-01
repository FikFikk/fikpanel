<script lang="ts">
  import type { PHPVersionInfo } from '../../lib/types';
  import SitesList from './SitesList.svelte';
  import '../../styles/pages/phpmanager/version-cards.css';

  type ManageTab = 'service' | 'sites' | 'config' | 'raw_ini' | 'fpm_pool' | 'raw_fpm' | 'disabled_func' | 'extensions' | 'opcache' | 'logs' | 'phpinfo';

  // Antarmuka properti untuk grid kartu versi PHP
  interface Props {
    versions: PHPVersionInfo[];
    installingVer: string | null;
    onOpenManageModal: (version: string, tab: ManageTab) => void;
    onInstallPHP: (v: PHPVersionInfo) => void;
  }

  let {
    versions,
    installingVer,
    onOpenManageModal,
    onInstallPHP,
  }: Props = $props();
</script>

<!-- Grid Kartu Versi PHP Server (Screenshot aaPanel Style) -->
<div class="pg-phpmanager-version-cards versions-grid">
  {#each versions as v (v.version)}
    <div class="v-card {v.installed ? 'installed' : 'not-installed'}">
      <div class="v-head">
        <div class="flex items-center gap-2">
          <span class="v-tag">PHP {v.version}</span>
          {#if v.running}
            <span class="badge badge--success"><i class="ri-checkbox-circle-fill"></i> RUNNING</span>
          {:else if v.installed}
            <span class="badge badge--danger"><i class="ri-stop-circle-line"></i> STOPPED</span>
          {:else}
            <span class="badge badge--neutral">BELUM TERPASANG</span>
          {/if}
        </div>
        {#if v.installed}
          <span class="fpm-badge">FPM Service</span>
        {/if}
      </div>

      <div class="v-meta">
        {#if v.installed}
          <div class="meta-row">
            <span class="meta-lbl">Socket:</span>
            <span class="mono meta-val truncate" title={v.fpm_socket}>{v.fpm_socket || '-'}</span>
          </div>
          <div class="meta-row">
            <span class="meta-lbl">Memory Limit:</span>
            <span class="meta-val font-semibold">{v.settings?.['memory_limit'] || '256M'}</span>
          </div>
          <div class="meta-row">
            <span class="meta-lbl">Upload Max:</span>
            <span class="meta-val">{v.settings?.['upload_max_filesize'] || '64M'}</span>
          </div>

          <!-- Bagian Situs Terhubung (HestiaCP / aaPanel Style) -->
          <SitesList
            version={v.version}
            sites={v.sites}
            sitesCount={v.sites_count}
            {onOpenManageModal}
          />
        {:else}
          <p class="text-subtle text-xs">
            Tersedia di repositori sistem operasi. Dapat dipasang dalam 1-klik melalui antrean APT.
          </p>
        {/if}
      </div>

      <div class="v-actions">
        {#if v.installed}
          <button
            class="btn btn-primary btn-sm flex-1"
            onclick={() => onOpenManageModal(v.version, 'config')}
            title="Kelola konfigurasi dan layanan PHP {v.version}"
          >
            <i class="ri-settings-4-line" aria-hidden="true"></i> Kelola (Manage)
          </button>
          <button
            class="btn btn-ghost btn-sm"
            onclick={() => onOpenManageModal(v.version, 'raw_ini')}
            title="Buka Editor php.ini langsung"
          >
            <i class="ri-file-code-line" aria-hidden="true"></i> php.ini
          </button>
        {:else}
          <button
            class="btn btn-primary btn-sm w-full"
            onclick={() => onInstallPHP(v)}
            disabled={installingVer === v.version}
          >
            <i class="ri-download-2-line {installingVer === v.version ? 'spin' : ''}" aria-hidden="true"></i>
            {installingVer === v.version ? 'Memasang...' : `Pasang PHP ${v.version}`}
          </button>
        {/if}
      </div>
    </div>
  {/each}
</div>
