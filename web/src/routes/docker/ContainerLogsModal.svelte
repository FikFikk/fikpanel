<script lang="ts">
  // Subkomponen modal pemantau log kontainer Docker dengan Live Follow (SSE), filter pencarian, dan unduh .log
  import '../../styles/pages/docker/container-logs-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import { copyText } from '../../lib/utils/browser';
  import { notify } from '../../lib/stores/toast.svelte';

  interface ContainerLogSelection {
    id: string;
    name: string;
    content: string;
    lines: number;
    timestamps?: boolean;
    isCompose?: boolean;
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

  let searchQuery = $state('');
  let liveFollow = $state(false);
  let autoScroll = $state(true);
  let preEl = $state<HTMLPreElement | null>(null);
  let streamES: EventSource | null = null;

  function stopStream() {
    if (streamES) {
      streamES.close();
      streamES = null;
    }
  }

  function startStream() {
    stopStream();
    if (!selectedContainerLogs.id || selectedContainerLogs.isCompose) return;

    const tsParam = selectedContainerLogs.timestamps ? '1' : '0';
    const url = `/api/docker/containers/${encodeURIComponent(selectedContainerLogs.id)}/logs/stream?lines=${selectedContainerLogs.lines}&timestamps=${tsParam}`;
    selectedContainerLogs = { ...selectedContainerLogs, content: '' };

    const es = new EventSource(url);
    streamES = es;

    es.onmessage = (ev) => {
      try {
        const line = JSON.parse(ev.data) as string;
        const prev = selectedContainerLogs.content;
        const next = prev ? `${prev}\n${line}` : line;
        // Batasi panjang buffer log di memori browser agar tetap ringan saat stream panjang
        const capped = next.length > 350000 ? next.slice(-300000) : next;
        selectedContainerLogs = { ...selectedContainerLogs, content: capped };
        if (autoScroll && preEl) {
          requestAnimationFrame(() => {
            if (preEl) preEl.scrollTop = preEl.scrollHeight;
          });
        }
      } catch {
        // Abaikan frame tidak valid
      }
    };

    es.addEventListener('done', () => {
      liveFollow = false;
      stopStream();
    });

    es.onerror = () => {
      liveFollow = false;
      stopStream();
    };
  }

  function toggleFollow() {
    if (liveFollow) {
      liveFollow = false;
      stopStream();
    } else {
      liveFollow = true;
      startStream();
    }
  }

  $effect(() => {
    if (!showLogsModal) {
      liveFollow = false;
      searchQuery = '';
      stopStream();
    }
    return () => stopStream();
  });

  let filteredContent = $derived.by(() => {
    const raw = selectedContainerLogs.content || '';
    const q = searchQuery.trim().toLowerCase();
    if (!q) return raw || '(Belum ada output log)';
    const matched = raw
      .split('\n')
      .filter((line) => line.toLowerCase().includes(q));
    return matched.length > 0 ? matched.join('\n') : `(Tidak ada baris yang cocok dengan "${searchQuery}")`;
  });

  async function handleCopy() {
    try {
      await copyText(filteredContent);
      notify('Log berhasil disalin', 'info');
    } catch {
      notify('Gagal menyalin log', 'error');
    }
  }

  function handleDownload() {
    const blob = new Blob([selectedContainerLogs.content || ''], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${selectedContainerLogs.name || 'container'}.log`;
    a.click();
    URL.revokeObjectURL(url);
  }
</script>

<Modal
  open={showLogsModal}
  size="xl"
  onclose={() => (showLogsModal = false)}
>
  {#snippet header()}
    <div class="flex-row items-center gap-2 flex-1">
      <i class="ri-file-list-3-line text-accent" aria-hidden="true"></i>
      <h2 class="modal__title">Log: {selectedContainerLogs.name}</h2>
      {#if liveFollow}
        <span class="badge badge--success">LIVE SSE</span>
      {/if}
    </div>
  {/snippet}

  <div class="pg-docker-container-logs-modal">
    <div class="logs-toolbar">
      <div class="logs-toolbar-group flex-1">
        <input
          type="text"
          class="input input--sm flex-1"
          placeholder="Cari teks di dalam log..."
          aria-label="Cari teks log"
          bind:value={searchQuery}
        />
        <select
          class="input input--sm w-28"
          aria-label="Jumlah baris log"
          bind:value={selectedContainerLogs.lines}
          onchange={() => (liveFollow ? startStream() : onRefreshContainerLogs())}
        >
          <option value={100}>100 baris</option>
          <option value={300}>300 baris</option>
          <option value={500}>500 baris</option>
          <option value={1000}>1000 baris</option>
        </select>
        {#if !selectedContainerLogs.isCompose}
          <label class="logs-check">
            <input
              type="checkbox"
              checked={Boolean(selectedContainerLogs.timestamps)}
              onchange={(e) => {
                selectedContainerLogs = { ...selectedContainerLogs, timestamps: e.currentTarget.checked };
                if (liveFollow) startStream();
                else onRefreshContainerLogs();
              }}
            />
            <span>Timestamp</span>
          </label>
        {/if}
        <label class="logs-check">
          <input type="checkbox" bind:checked={autoScroll} />
          <span>Auto-scroll</span>
        </label>
      </div>

      <div class="logs-toolbar-group">
        {#if !selectedContainerLogs.isCompose}
          <button
            type="button"
            class="btn {liveFollow ? 'btn--danger' : 'btn--primary'} btn--sm"
            onclick={toggleFollow}
          >
            <i class={liveFollow ? 'ri-stop-line' : 'ri-live-line'} aria-hidden="true"></i>
            <span>{liveFollow ? 'Hentikan Live' : 'Live Follow'}</span>
          </button>
        {/if}
        <button
          type="button"
          class="btn btn--secondary btn--sm btn--icon"
          title="Segarkan Log"
          aria-label="Segarkan Log"
          onclick={onRefreshContainerLogs}
        >
          <i class="ri-refresh-line {logsLoading ? 'spin' : ''}" aria-hidden="true"></i>
        </button>
        <button
          type="button"
          class="btn btn--secondary btn--sm btn--icon"
          title="Salin Log"
          aria-label="Salin Log"
          onclick={handleCopy}
        >
          <i class="ri-file-copy-line" aria-hidden="true"></i>
        </button>
        <button
          type="button"
          class="btn btn--secondary btn--sm btn--icon"
          title="Unduh .log"
          aria-label="Unduh berkas log"
          onclick={handleDownload}
        >
          <i class="ri-download-line" aria-hidden="true"></i>
        </button>
      </div>
    </div>

    <pre bind:this={preEl} class="terminal-log-viewer font-mono text-xs">{filteredContent}</pre>
  </div>

  {#snippet footer()}
    <span class="text-xs text-muted flex-1">
      {selectedContainerLogs.isCompose ? 'Stack Compose' : `Container ID: ${selectedContainerLogs.id.slice(0, 12)}`}
    </span>
    <button type="button" class="btn btn--secondary btn--sm" onclick={() => (showLogsModal = false)}>
      Tutup
    </button>
  {/snippet}
</Modal>
