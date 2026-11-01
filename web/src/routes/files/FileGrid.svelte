<script lang="ts">
  // Subkomponen tampilan kartu grid berkas dan direktori pada halaman File Manager
  import '../../styles/pages/files/file-grid.css';
  import { formatBytes } from '../../lib/api';
  import type { FileItem } from '../../lib/types';

  interface Props {
    paginatedItems: FileItem[];
    selectedPaths: Set<string>;
    getFileIcon: (item: FileItem) => { icon: string; color: string };
    onItemClick: (item: FileItem) => void;
    onRowContextMenu: (item: FileItem, e: MouseEvent) => void;
    onToggleSelectItem: (path: string) => void;
    onToggleOperateMenu: (item: FileItem, e: MouseEvent) => void;
  }

  let {
    paginatedItems,
    selectedPaths,
    getFileIcon,
    onItemClick,
    onRowContextMenu,
    onToggleSelectItem,
    onToggleOperateMenu,
  }: Props = $props();
</script>

<div class="pg-files-file-grid grid-container">
  {#each paginatedItems as item (item.path)}
    {@const iconMeta = getFileIcon(item)}
    <div
      class="grid-card {selectedPaths.has(item.path) ? 'selected-card' : ''}"
      role="button"
      tabindex="0"
      onclick={() => onItemClick(item)}
      onkeydown={(e) => e.key === 'Enter' && onItemClick(item)}
      oncontextmenu={(e) => onRowContextMenu(item, e)}
    >
      <div class="card-top">
        <input
          type="checkbox"
          checked={selectedPaths.has(item.path)}
          onclick={(e) => e.stopPropagation()}
          onchange={() => onToggleSelectItem(item.path)}
        />
        <button
          class="card-operate-btn"
          onclick={(e) => onToggleOperateMenu(item, e)}
          title="Menu Operasi"
        >
          <i class="ri-more-2-fill"></i>
        </button>
      </div>
      <i class="{iconMeta.icon} card-icon" style="color: {iconMeta.color};"></i>
      <span class="card-name" title={item.name}>{item.name}</span>
      <span class="card-size mono">{item.is_dir ? 'Directory' : formatBytes(item.size)}</span>
    </div>
  {/each}
</div>

