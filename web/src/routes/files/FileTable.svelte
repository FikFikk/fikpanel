<script lang="ts">
  // Subkomponen tabel daftar berkas dan direktori pada halaman File Manager
  import '../../styles/pages/files/file-table.css';
  import { formatBytes } from '../../lib/api';
  import type { FileItem } from '../../lib/types';

  interface Props {
    paginatedItems: FileItem[];
    selectedPaths: Set<string>;
    activeMenuItem: FileItem | null;
    getFileIcon: (item: FileItem) => { icon: string; color: string };
    onToggleSelectAll: (e: Event) => void;
    onToggleSelectItem: (path: string) => void;
    onRowContextMenu: (item: FileItem, e: MouseEvent) => void;
    onItemClick: (item: FileItem) => void;
    onOpenChmodModal: (item: FileItem) => void;
    onOpenEditor: (item: FileItem) => void;
    onDownloadFile: (item: FileItem) => void;
    onDeleteSingleItem: (item: FileItem) => void;
    onToggleOperateMenu: (item: FileItem, e: MouseEvent) => void;
  }

  let {
    paginatedItems,
    selectedPaths,
    activeMenuItem,
    getFileIcon,
    onToggleSelectAll,
    onToggleSelectItem,
    onRowContextMenu,
    onItemClick,
    onOpenChmodModal,
    onOpenEditor,
    onDownloadFile,
    onDeleteSingleItem,
    onToggleOperateMenu,
  }: Props = $props();
</script>

<div class="pg-files-file-table table-wrap table-container">
  <table class="table file-table">
    <thead>
      <tr>
        <th class="col-check">
          <input
            type="checkbox"
            checked={paginatedItems.length > 0 && selectedPaths.size === paginatedItems.length}
            onchange={onToggleSelectAll}
          />
        </th>
        <th class="col-name">File Name &darr;</th>
        <th class="col-prot">Protected</th>
        <th class="col-pmsn">PMSN/Owner</th>
        <th class="col-size">Size &darr;</th>
        <th class="col-time">Modified Time &darr;</th>
        <th class="col-ps">Ps</th>
        <th class="col-ops">Operation</th>
      </tr>
    </thead>
    <tbody>
      {#each paginatedItems as item (item.path)}
        {@const iconMeta = getFileIcon(item)}
        <tr
          class="{selectedPaths.has(item.path) ? 'selected-row' : ''} {activeMenuItem?.path === item.path ? 'highlight-row' : ''}"
          oncontextmenu={(e) => onRowContextMenu(item, e)}
        >
          <td class="col-check">
            <input
              type="checkbox"
              checked={selectedPaths.has(item.path)}
              onchange={() => onToggleSelectItem(item.path)}
            />
          </td>
          <td class="col-name">
            <div
              class="name-cell"
              role="button"
              tabindex="0"
              onclick={() => onItemClick(item)}
              onkeydown={(e) => e.key === 'Enter' && onItemClick(item)}
            >
              <i class="{iconMeta.icon} item-icon" style="color: {iconMeta.color};"></i>
              <span class="item-name {item.is_dir ? 'dir-name' : ''}">{item.name}</span>
            </div>
          </td>
          <td class="col-prot">
            <span class="badge-prot">
              <i class="ri-lock-unlock-line"></i> Unprotected
            </span>
          </td>
          <td class="col-pmsn">
            <button
              class="pmsn-pill"
              onclick={() => onOpenChmodModal(item)}
              title="Klik untuk ubah perizinan (chmod)"
            >
              {item.mode.slice(-3)} / {item.owner}
            </button>
          </td>
          <td class="col-size mono">
            {#if item.is_dir}
              <span class="calc-text">Calculate</span>
            {:else}
              {formatBytes(item.size)}
            {/if}
          </td>
          <td class="col-time mono">{item.mod_time}</td>
          <td class="col-ps text-muted">{item.is_dir ? 'Directory' : item.ext || 'File'}</td>
          <td class="col-ops">
            <div class="row-operations">
              <!-- Pintasan cepat saat baris di-hover -->
              <div class="hover-shortcuts">
                {#if !item.is_dir}
                  <button class="quick-icon-btn" onclick={() => onOpenEditor(item)} title="Edit Cepat">
                    <i class="ri-edit-line"></i>
                  </button>
                  <button class="quick-icon-btn" onclick={() => onDownloadFile(item)} title="Unduh">
                    <i class="ri-download-line"></i>
                  </button>
                {:else}
                  <button class="quick-icon-btn" onclick={() => onItemClick(item)} title="Buka">
                    <i class="ri-folder-open-line"></i>
                  </button>
                {/if}
                <button class="quick-icon-btn link-danger" onclick={() => onDeleteSingleItem(item)} title="Hapus">
                  <i class="ri-delete-bin-line"></i>
                </button>
              </div>

              <!-- Tombol dropdown Operate -->
              <button
                class="btn-operate {activeMenuItem?.path === item.path ? 'is-active' : ''}"
                onclick={(e) => onToggleOperateMenu(item, e)}
                title="Menu Operasi"
              >
                <span>Operate</span>
                <i class="ri-arrow-down-s-line"></i>
              </button>
            </div>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

