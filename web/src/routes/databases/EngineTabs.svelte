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

  const engineItems: Array<{ key: 'mysql' | 'postgresql' | 'mongodb' | 'redis'; label: string; icon: string }> = [
    { key: 'mysql', label: 'MySQL / MariaDB', icon: 'ri-database-2-line' },
    { key: 'postgresql', label: 'PostgreSQL', icon: 'ri-server-line' },
    { key: 'mongodb', label: 'MongoDB', icon: 'ri-leaf-line' },
    { key: 'redis', label: 'Redis', icon: 'ri-flashlight-line' },
  ];

  // Tentukan kelas warna semantik untuk indikator status engine
  function getDotClass(eng?: DatabaseEngineInfo): string {
    if (!eng || !eng.installed) return 'uninstalled';
    return eng.running ? 'running' : 'stopped';
  }

  function getDotTitle(eng?: DatabaseEngineInfo): string {
    if (!eng || !eng.installed) return 'Belum terpasang';
    return eng.running ? `Berjalan (${eng.version || 'aktif'})` : 'Terpasang namun berhenti';
  }
</script>

<nav class="engine-tabs-header pg-databases-engine-tabs" aria-label="Navigasi Engine Database">
  <div class="engine-tabs" role="tablist">
    {#each engineItems as item (item.key)}
      {@const info = engines.find((e) => e.name === item.key)}
      <button
        type="button"
        class="engine-tab"
        class:active={mainView === 'database' && activeEngine === item.key}
        role="tab"
        aria-selected={mainView === 'database' && activeEngine === item.key}
        onclick={() => onSwitchEngine(item.key)}
      >
        <i class="{item.icon} tab-icon" aria-hidden="true"></i>
        <span>{item.label}</span>
        {#if info?.installed && info.databases_count > 0}
          <span class="tab-count">{info.databases_count}</span>
        {/if}
        <span class="engine-dot {getDotClass(info)}" title={getDotTitle(info)}></span>
      </button>
    {/each}

    <button
      type="button"
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
      type="button"
      class="btn-task-queue"
      onclick={onOpenTasks}
      title="Buka Antrean Tugas & Log Instalasi"
    >
      <i class="ri-terminal-window-line" aria-hidden="true"></i>
      <span>Antrean Tugas</span>
      {#if tasks.some((t) => t.status === 'running')}
        <span class="badge-running-count">
          {tasks.filter((t) => t.status === 'running').length}
        </span>
      {/if}
    </button>
  </div>
</nav>
