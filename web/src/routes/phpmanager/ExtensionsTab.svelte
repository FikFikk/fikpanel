<script lang="ts">
  import '../../styles/pages/phpmanager/extensions-tab.css';
  import type { PHPExtensionInfo } from '../../lib/types';

  // Antarmuka properti untuk tab instalasi ekstensi PHP
  interface Props {
    activeVersion: string;
    extSearch: string;
    filteredExtensions: PHPExtensionInfo[];
    actionExtName: string | null;
    onInstallExtension: (ext: PHPExtensionInfo) => void;
    onUninstallExtension: (ext: PHPExtensionInfo) => void;
    onOpenSshModal: (ext: PHPExtensionInfo) => void;
  }

  let {
    activeVersion,
    extSearch = $bindable(),
    filteredExtensions,
    actionExtName,
    onInstallExtension,
    onUninstallExtension,
    onOpenSshModal,
  }: Props = $props();
</script>

<div class="tab-pane pg-phpmanager-extensions-tab">
  <div class="section-title-wrap flex justify-between items-center">
    <div>
      <h4 class="section-title">Install Extensions (Ekstensi PHP {activeVersion})</h4>
      <p class="section-desc">Status modul dan pustaka ekstensi PHP populer di server.</p>
    </div>
    <div class="search-mini-wrap">
      <i class="ri-search-line"></i>
      <input
        type="text"
        class="search-mini-input"
        placeholder="Cari ekstensi (redis, opcache, imagick)..."
        bind:value={extSearch}
      />
    </div>
  </div>

  <div class="table-wrap extensions-table-wrap">
    <table class="table">
      <thead>
        <tr>
          <th>Nama Ekstensi</th>
          <th>Kategori</th>
          <th>Deskripsi</th>
          <th>Status</th>
          <th>Paket APT</th>
          <th>Tindakan & Opsi SSH</th>
        </tr>
      </thead>
      <tbody>
        {#each filteredExtensions as ext (ext.name)}
          <tr>
            <td class="font-bold mono text-text">{ext.name}</td>
            <td>
              <span class="cat-pill {ext.type}">{ext.type}</span>
            </td>
            <td class="text-xs text-muted">{ext.description}</td>
            <td>
              {#if ext.installed}
                <span class="badge badge--success"><i class="ri-check-line"></i> Terpasang</span>
              {:else}
                <span class="badge badge--neutral">Belum ada</span>
              {/if}
            </td>
            <td>
              <code class="apt-hint mono">{ext.package_name || 'php' + activeVersion + '-' + ext.name}</code>
            </td>
            <td>
              <div class="ext-action-group flex items-center gap-2">
                {#if ext.installed}
                  <span class="badge-active-runtime">
                    <i class="ri-checkbox-circle-fill"></i> Aktif
                  </span>
                  {#if !ext.is_core}
                    <button
                      type="button"
                      class="btn btn-xs btn-danger"
                      onclick={() => onUninstallExtension(ext)}
                      disabled={actionExtName === ext.name}
                      title="Copot paket ekstensi {ext.name}"
                    >
                      {#if actionExtName === ext.name}
                        <i class="ri-loader-4-line spin"></i>
                      {:else}
                        <i class="ri-delete-bin-line"></i> Copot
                      {/if}
                    </button>
                  {/if}
                {:else}
                  <button
                    type="button"
                    class="btn btn-xs btn-primary"
                    onclick={() => onInstallExtension(ext)}
                    disabled={actionExtName === ext.name}
                    title="Pasang ekstensi {ext.name} secara otomatis (1-Klik via Panel)"
                  >
                    {#if actionExtName === ext.name}
                      <i class="ri-loader-4-line spin"></i> Memasang...
                    {:else}
                      <i class="ri-download-2-line"></i> Install
                    {/if}
                  </button>
                {/if}

                <button
                  type="button"
                  class="btn btn-xs btn-ghost"
                  onclick={() => onOpenSshModal(ext)}
                  title="Lihat & Salin Perintah Instalasi via Terminal / SSH"
                >
                  <i class="ri-terminal-box-line"></i> Perintah SSH
                </button>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
