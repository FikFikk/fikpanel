<script lang="ts">
  import '../../styles/pages/phpmanager/phpinfo-tab.css';

  // Antarmuka properti untuk tab diagnostik phpinfo
  interface Props {
    activeVersion: string;
    phpInfoContent: string;
    loadingPhpInfo: boolean;
    onRefreshPhpInfo: () => void;
  }

  let {
    activeVersion,
    phpInfoContent,
    loadingPhpInfo,
    onRefreshPhpInfo,
  }: Props = $props();
</script>

<div class="tab-pane full-height-pane pg-phpmanager-phpinfo-tab">
  <div class="flex justify-between items-center mb-3">
    <div>
      <h4 class="section-title">Diagnostik phpinfo() — PHP {activeVersion}</h4>
      <p class="section-desc">Ringkasan konfigurasi internal, library dependensi, dan environment runtime.</p>
    </div>
    <button class="btn btn-ghost btn-sm" onclick={onRefreshPhpInfo} disabled={loadingPhpInfo}>
      <i class="ri-refresh-line {loadingPhpInfo ? 'spin' : ''}"></i> Refresh Info
    </button>
  </div>

  <div class="terminal-log-viewer flex-1">
    {#if loadingPhpInfo}
      <div class="editor-loading">
        <i class="ri-loader-4-line spin text-2xl"></i>
        <span>Mengeksekusi php -i...</span>
      </div>
    {:else}
      <pre class="log-pre mono">{phpInfoContent}</pre>
    {/if}
  </div>
</div>
