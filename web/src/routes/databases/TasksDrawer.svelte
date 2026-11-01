<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { SystemTask } from '../../lib/types';
  import '../../styles/pages/databases/tasks-drawer.css';

  // Antarmuka properti untuk drawer antrean eksekusi tugas
  interface Props {
    open: boolean;
    tasks: SystemTask[];
    activeTask: SystemTask | null;
    streamLogs: string[];
    terminalEl?: HTMLDivElement | null;
    onOpenTaskStream: (id: string) => void;
    onRefreshTasks: () => void;
    onClose: () => void;
  }

  let {
    open,
    tasks,
    activeTask,
    streamLogs,
    terminalEl = $bindable(null),
    onOpenTaskStream,
    onRefreshTasks,
    onClose,
  }: Props = $props();
</script>

<!-- Drawer / Modal Messages Box & Real-Time Log Execution (ala aaPanel) -->
<Modal
  {open}
  size="xl"
  onclose={onClose}
>
  {#snippet header()}
    <div class="pg-databases-tasks-title">
      <i class="ri-terminal-window-fill" aria-hidden="true"></i>
      <span>Messages Box / Antrean Eksekusi</span>
    </div>
  {/snippet}

  <div class="pg-databases-tasks-drawer">
    <!-- Sidebar Daftar Task -->
    <aside class="messages-sidebar">
      <div class="task-list-title">
        <span>Daftar Tugas ({tasks.length})</span>
      </div>
      <div class="task-items-list">
        {#each tasks as t (t.id)}
          <button
            class="task-item"
            class:active={activeTask?.id === t.id}
            onclick={() => onOpenTaskStream(t.id)}
          >
            <span class="task-status-dot {t.status}"></span>
            <div class="task-item-meta">
              <div class="task-item-name">{t.target}</div>
              <div class="task-item-status">{t.status}</div>
            </div>
          </button>
        {/each}
      </div>
    </aside>

    <!-- Area Log Terminal Real-time -->
    <main class="messages-log-area">
      <div class="log-area-topbar">
        <span class="log-target-label">
          Eksekusi: <strong>{activeTask?.target || 'Pilih tugas'}</strong>
        </span>
        <div class="log-topbar-actions">
          <span class="log-tip">Tip: Jika tugas terhenti, klik muat ulang antrean.</span>
          <button
            class="btn-refresh-task"
            onclick={onRefreshTasks}
          >
            <i class="ri-refresh-line" aria-hidden="true"></i>
            <span>Muat Ulang</span>
          </button>
        </div>
      </div>

      <div class="terminal-container" bind:this={terminalEl}>
        {#if streamLogs.length === 0}
          <div class="terminal-waiting">
            <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
            <span>Menghubungkan ke stream eksekusi tugas...</span>
          </div>
        {:else}
          {#each streamLogs as line}
            <div class="terminal-line">{line}</div>
          {/each}
        {/if}
      </div>
    </main>
  </div>
</Modal>
