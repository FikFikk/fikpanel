<script lang="ts">
  import '../../styles/pages/phpmanager/logs-tab.css';

  // Antarmuka properti untuk tab log PHP-FPM
  interface Props {
    activeVersion: string;
    logType: 'fpm' | 'slow';
    logContent: string;
    loadingLogs: boolean;
    onSelectLogType: (type: 'fpm' | 'slow') => void;
    onRefreshLogs: () => void;
    onClearLogs: () => void;
  }

  let {
    activeVersion,
    logType,
    logContent,
    loadingLogs,
    onSelectLogType,
    onRefreshLogs,
    onClearLogs,
  }: Props = $props();
</script>

<div class="tab-pane full-height-pane pg-phpmanager-logs-tab">
  <div class="flex justify-between items-center mb-3">
    <div class="flex gap-2">
      <button
        class="btn btn-sm {logType === 'fpm' ? 'btn-primary' : 'btn-ghost'}"
        onclick={() => onSelectLogType('fpm')}
      >
        <i class="ri-error-warning-line"></i> Error Log (php-fpm.log)
      </button>
      <button
        class="btn btn-sm {logType === 'slow' ? 'btn-primary' : 'btn-ghost'}"
        onclick={() => onSelectLogType('slow')}
      >
        <i class="ri-timer-line"></i> Slow Log
      </button>
    </div>
    <div class="flex gap-2">
      <button class="btn btn-ghost btn-sm" onclick={onRefreshLogs} disabled={loadingLogs}>
        <i class="ri-refresh-line {loadingLogs ? 'spin' : ''}"></i> Refresh
      </button>
      <button class="btn btn-danger btn-sm" onclick={onClearLogs}>
        <i class="ri-delete-bin-line"></i> Kosongkan Log
      </button>
    </div>
  </div>

  <div class="terminal-log-viewer flex-1">
    {#if loadingLogs}
      <div class="editor-loading">
        <i class="ri-loader-4-line spin text-2xl"></i>
        <span>Mengambil data log terbaru...</span>
      </div>
    {:else}
      <pre class="log-pre mono">{logContent}</pre>
    {/if}
  </div>
</div>
