<script lang="ts">
  // Subkomponen bar multi-tab direktori pada halaman File Manager
  import '../../styles/pages/files/dir-tabs.css';

  export interface DirTab {
    id: string;
    name: string;
    path: string;
  }

  interface Props {
    tabs: DirTab[];
    activeTabId: string;
    onSwitchTab: (tab: DirTab) => void;
    onCloseTab: (id: string, e: MouseEvent) => void;
    onAddTab: () => void;
  }

  let {
    tabs,
    activeTabId,
    onSwitchTab,
    onCloseTab,
    onAddTab,
  }: Props = $props();
</script>

<div class="pg-files-dir-tabs tabs-bar">
  <div class="tab-list">
    {#each tabs as t (t.id)}
      <div
        class="tab-item {activeTabId === t.id ? 'active' : ''}"
        role="button"
        tabindex="0"
        onclick={() => onSwitchTab(t)}
        onkeydown={(e) => e.key === 'Enter' && onSwitchTab(t)}
      >
        <i class="ri-folder-fill tab-icon"></i>
        <span class="tab-title">{t.name}</span>
        {#if tabs.length > 1}
          <button
            class="tab-close"
            onclick={(e) => onCloseTab(t.id, e)}
            title="Tutup tab"
          >&times;</button>
        {/if}
      </div>
    {/each}
    <button class="btn-add-tab" onclick={() => onAddTab()} title="Buka tab baru">
      <i class="ri-add-line"></i>
    </button>
  </div>
</div>
