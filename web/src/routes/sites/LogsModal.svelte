<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { Site } from '../../lib/types';
  import '../../styles/pages/sites/logs-modal.css';

  // Antarmuka properti untuk modal penampil log akses & error situs
  interface Props {
    open: boolean;
    logTarget: Site | null;
    logTab: 'access' | 'error';
    logLoading: boolean;
    logContent: string;
    onClose: () => void;
    onSwitchLogTab: (tab: 'access' | 'error') => void;
    onRefreshLogs: (target: Site, tab: 'access' | 'error') => void;
  }

  let {
    open,
    logTarget,
    logTab,
    logLoading,
    logContent,
    onClose,
    onSwitchLogTab,
    onRefreshLogs,
  }: Props = $props();
</script>

<Modal
  {open}
  title={logTarget ? `Log Web Server: ${logTarget.domain}` : 'Log Web Server'}
  size="xl"
  onclose={onClose}
>
  {#if logTarget}
    <div class="pg-sites-logs-modal">
      <div class="log-tabs">
        <button
          type="button"
          class="log-tab {logTab === 'access' ? 'active' : ''}"
          onclick={() => onSwitchLogTab('access')}
        >
          Access Log ({(logTarget.request_count ?? 0).toLocaleString('id-ID')} Req)
        </button>
        <button
          type="button"
          class="log-tab {logTab === 'error' ? 'active' : ''}"
          onclick={() => onSwitchLogTab('error')}
        >
          Error Log
        </button>
        <button
          type="button"
          class="log-tab"
          onclick={() => logTarget && onRefreshLogs(logTarget, logTab)}
          disabled={logLoading}
        >
          <i class="ri-refresh-line" class:spin={logLoading}></i> Segarkan
        </button>
      </div>

      <div class="log-viewer mono">
        {#if logLoading}
          <div class="log-line text-muted">
            <i class="ri-loader-4-line spin"></i> Memuat catatan log {logTab} dari server...
          </div>
        {:else}
          <pre class="log-pre-raw">{logContent}</pre>
        {/if}
      </div>
    </div>
  {/if}

  {#snippet footer()}
    <button type="button" class="btn btn--ghost" onclick={onClose}>
      Tutup
    </button>
  {/snippet}
</Modal>

