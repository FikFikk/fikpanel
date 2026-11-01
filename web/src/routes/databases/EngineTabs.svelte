<script lang="ts">
  /**
   * Subkomponen Navigasi Tab Engine Database & Tombol Antrean Tugas.
   */
  import type { DatabaseEngineInfo, SystemTask } from '../../lib/types';
  import '../../styles/pages/databases/engine-tabs.css';

  interface Props {
    mainView: 'database' | 'backup';
    activeEngine: 'mysql' | 'postgresql' | 'mongodb' | 'redis';
    engines: DatabaseEngineInfo[];
    tasks: SystemTask[];
    onSwitchEngine: (engine: 'mysql' | 'postgresql' | 'mongodb' | 'redis') => void;
    onSelectBackup: () => void;
    onOpenTasks: () => void;
  }

  let {
    mainView,
    activeEngine,
    engines,
    tasks,
    onSwitchEngine,
    onSelectBackup,
    onOpenTasks
  }: Props = $props();
</script>

<header class="engine-tabs-header pg-databases-engine-tabs">
  <div class="engine-tabs" role="tablist">
    <button
      class="engine-tab"
      class:active={mainView === 'database' && activeEngine === 'mysql'}
      role="tab"
      aria-selected={mainView === 'database' && activeEngine === 'mysql'}
      onclick={() => onSwitchEngine('mysql')}
    >
      <i class="ri-database-2-line tab-icon" aria-hidden="true"></i>
      <span>MySQL / MariaDB</span>
      {#if engines.find((e) => e.name === 'mysql')?.installed}
        <span class="engine-dot active" title="Layanan aktif"></span>
      {/if}
    </button>

    <button
      class="engine-tab"
      class:active={mainView === 'database' && activeEngine === 'postgresql'}
      role="tab"
      aria-selected={mainView === 'database' && activeEngine === 'postgresql'}
      onclick={() => onSwitchEngine('postgresql')}
    >
      <i class="ri-server-line tab-icon" aria-hidden="true"></i>
      <span>PostgreSQL</span>
      {#if engines.find((e) => e.name === 'postgresql')?.installed}
        <span class="engine-dot active" title="Layanan aktif"></span>
      {/if}
    </button>

    <button
      class="engine-tab"
      class:active={mainView === 'database' && activeEngine === 'mongodb'}
      role="tab"
      aria-selected={mainView === 'database' && activeEngine === 'mongodb'}
      onclick={() => onSwitchEngine('mongodb')}
    >
      <i class="ri-leaf-line tab-icon" aria-hidden="true"></i>
      <span>MongoDB</span>
      {#if engines.find((e) => e.name === 'mongodb')?.installed}
        <span class="engine-dot active" title="Layanan aktif"></span>
      {:else}
        <span class="engine-dot uninstalled" title="Belum terpasang"></span>
      {/if}
    </button>

    <button
      class="engine-tab"
      class:active={mainView === 'database' && activeEngine === 'redis'}
      role="tab"
      aria-selected={mainView === 'database' && activeEngine === 'redis'}
      onclick={() => onSwitchEngine('redis')}
    >
      <i class="ri-flashlight-line tab-icon" aria-hidden="true"></i>
      <span>Redis</span>
      {#if engines.find((e) => e.name === 'redis')?.installed}
        <span class="engine-dot active" title="Layanan aktif"></span>
      {/if}
    </button>

    <button
      class="engine-tab"
      class:active={mainView === 'backup'}
      role="tab"
      aria-selected={mainView === 'backup'}
      onclick={onSelectBackup}
    >
      <i class="ri-inbox-archive-line tab-icon" aria-hidden="true"></i>
      <span>Cadangan & S3 Cloud</span>
    </button>
  </div>

  <div class="header-tools">
    <button
      class="btn-task-queue"
      onclick={onOpenTasks}
      title="Buka Antrean Tugas & Log Instalasi"
    >
      <i class="ri-terminal-window-line" aria-hidden="true"></i>
      <span>Antrean Tugas</span>
      {#if tasks.some((t) => t.status === 'running')}
        <span class="badge-running-count">1</span>
      {/if}
    </button>
  </div>
</header>

