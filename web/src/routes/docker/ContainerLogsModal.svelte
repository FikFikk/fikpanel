<script lang="ts">
  // Subkomponen modal pemantau log kontainer Docker
  import '../../styles/pages/docker/container-logs-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';

  interface ContainerLogSelection {
    id: string;
    name: string;
    content: string;
    lines: number;
  }

  interface Props {
    showLogsModal: boolean;
    selectedContainerLogs: ContainerLogSelection;
    logsLoading: boolean;
    onRefreshContainerLogs: () => void;
  }

  let {
    showLogsModal = $bindable(),
    selectedContainerLogs = $bindable(),
    logsLoading,
    onRefreshContainerLogs,
  }: Props = $props();
</script>

<Modal
  open={showLogsModal}
  size="lg"
  onclose={() => (showLogsModal = false)}
>
  {#snippet header()}
    <div class="flex-row items-center gap-3 flex-1">
      <h2 class="modal__title flex-1">
        <i class="ri-file-list-3-line" aria-hidden="true"></i>
        <span>Log Kontainer: {selectedContainerLogs.name}</span>
      </h2>
      <select
        class="input input--sm w-28"
        bind:value={selectedContainerLogs.lines}
        onchange={onRefreshContainerLogs}
      >
        <option value={100}>100 baris</option>
        <option value={200}>200 baris</option>
        <option value={500}>500 baris</option>
        <option value={1000}>1000 baris</option>
      </select>
      <button
        class="btn btn--secondary btn--sm btn--icon"
        title="Segarkan Log"
        aria-label="Segarkan Log"
        onclick={onRefreshContainerLogs}
      >
        <i class="ri-refresh-line {logsLoading ? 'spin' : ''}" aria-hidden="true"></i>
      </button>
    </div>
  {/snippet}

  <pre class="pg-docker-container-logs-modal terminal-log-viewer font-mono text-xs">{selectedContainerLogs.content}</pre>

  {#snippet footer()}
    <span class="text-xs text-muted flex-1">ID: {selectedContainerLogs.id.slice(0, 12)}</span>
    <button class="btn btn--secondary btn--sm" onclick={() => (showLogsModal = false)}>
      Tutup
    </button>
  {/snippet}
</Modal>

