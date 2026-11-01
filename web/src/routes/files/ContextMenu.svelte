<script lang="ts">
  /**
   * Subkomponen Menu Konteks / Dropdown Operasi Berkas untuk Halaman Files.
   */
  import type { FileItem } from '../../lib/types';
  import '../../styles/pages/files/context-menu.css';

  interface Props {
    activeMenuItem: FileItem;
    menuPos: { top: number; left: number };
    onClose: () => void;
    onOpenEditor: (item: FileItem) => void;
    onDownloadFile: (item: FileItem) => void;
    onExternalShare: (item: FileItem) => void;
    onAddToFavorites: (item: FileItem) => void;
    onOpenCloudStorage: (provider: string) => void;
    onOpenChmodModal: (item: FileItem) => void;
    onItemClick: (item: FileItem) => void;
    onAddTab: (path: string) => void;
    onCopyItem: (item: FileItem) => void;
    onCopyPath: (item: FileItem) => void;
    onCutItem: (item: FileItem) => void;
    onOpenRenameModal: (item: FileItem) => void;
    onDeleteSingleItem: (item: FileItem) => void;
    onOpenCompress: (item: FileItem) => void;
    onOpenProperties: (item: FileItem) => void;
  }

  let {
    activeMenuItem,
    menuPos,
    onClose,
    onOpenEditor,
    onDownloadFile,
    onExternalShare,
    onAddToFavorites,
    onOpenCloudStorage,
    onOpenChmodModal,
    onItemClick,
    onAddTab,
    onCopyItem,
    onCopyPath,
    onCutItem,
    onOpenRenameModal,
    onDeleteSingleItem,
    onOpenCompress,
    onOpenProperties
  }: Props = $props();
</script>

<div
  class="pg-files-context-menu"
  role="menu"
  tabindex="-1"
  style="top: {menuPos.top}px; left: {menuPos.left}px;"
  onclick={(e) => e.stopPropagation()}
  onkeydown={(e) => e.key === 'Escape' && onClose()}
>
  {#if !activeMenuItem.is_dir}
    <!-- Opsi Khusus Berkas (File): Tanpa Open/New Window, diawali dengan Edit -->
    <!-- Edit -->
    <button
      class="operate-item"
      onclick={() => {
        const it = activeMenuItem;
        onClose();
        onOpenEditor(it);
      }}
    >
      <i class="ri-edit-line op-icon icon-blue"></i>
      <span>Edit</span>
    </button>

    <!-- Download -->
    <button
      class="operate-item"
      onclick={() => {
        const it = activeMenuItem;
        onClose();
        onDownloadFile(it);
      }}
    >
      <i class="ri-download-line op-icon icon-blue"></i>
      <span>Download</span>
    </button>

    <!-- External Share -->
    <button
      class="operate-item"
      onclick={() => onExternalShare(activeMenuItem)}
    >
      <i class="ri-share-forward-line op-icon icon-green"></i>
      <span>External Share</span>
    </button>

    <!-- Add to Favorites -->
    <button
      class="operate-item"
      onclick={() => {
        const it = activeMenuItem;
        onClose();
        onAddToFavorites(it);
      }}
    >
      <i class="ri-star-fill op-icon icon-yellow"></i>
      <span>Add to Favorites</span>
    </button>

    <!-- Upload to Cloud Storage -->
    <button
      class="operate-item"
      onclick={() => {
        onClose();
        onOpenCloudStorage('S3 Storage');
      }}
    >
      <div class="op-label-row">
        <i class="ri-upload-cloud-2-line op-icon"></i>
        <span>Upload to Cloud Storage</span>
      </div>
      <i class="ri-arrow-right-s-line op-chevron"></i>
    </button>

    <!-- Permission -->
    <button
      class="operate-item"
      onclick={() => onOpenChmodModal(activeMenuItem)}
    >
      <i class="ri-folder-lock-fill op-icon icon-amber"></i>
      <span>Permission</span>
    </button>
  {:else}
    <!-- Opsi Khusus Direktori (Folder): Ada Open dan Open in New Window -->
    <!-- Open -->
    <button
      class="operate-item"
      onclick={() => {
        const it = activeMenuItem;
        onClose();
        onItemClick(it);
      }}
    >
      <i class="ri-folder-open-fill op-icon icon-amber"></i>
      <span>Open</span>
    </button>

    <!-- Open in New Window / Tab -->
    <button
      class="operate-item"
      onclick={() => {
        const it = activeMenuItem;
        onClose();
        onAddTab(it.path);
      }}
    >
      <i class="ri-external-link-line op-icon icon-blue"></i>
      <span>Open in New Window</span>
    </button>

    <!-- Add to Favorites -->
    <button
      class="operate-item"
      onclick={() => {
        const it = activeMenuItem;
        onClose();
        onAddToFavorites(it);
      }}
    >
      <i class="ri-star-fill op-icon icon-yellow"></i>
      <span>Add to Favorites</span>
    </button>

    <!-- Permission -->
    <button
      class="operate-item"
      onclick={() => onOpenChmodModal(activeMenuItem)}
    >
      <i class="ri-folder-lock-fill op-icon icon-amber"></i>
      <span>Permission</span>
    </button>
  {/if}

  <div class="operate-divider"></div>

  <!-- Copy -->
  <button
    class="operate-item"
    onclick={() => onCopyItem(activeMenuItem)}
  >
    <i class="ri-file-copy-line op-icon"></i>
    <span>Copy</span>
  </button>

  <!-- Copy Path -->
  <button
    class="operate-item"
    onclick={() => onCopyPath(activeMenuItem)}
  >
    <i class="ri-clipboard-line op-icon"></i>
    <span>Copy Path</span>
  </button>

  <!-- Cut -->
  <button
    class="operate-item"
    onclick={() => onCutItem(activeMenuItem)}
  >
    <i class="ri-scissors-line op-icon icon-cyan"></i>
    <span>Cut</span>
  </button>

  <!-- Rename -->
  <button
    class="operate-item"
    onclick={() => onOpenRenameModal(activeMenuItem)}
  >
    <i class="ri-edit-box-line op-icon icon-blue"></i>
    <span>Rename</span>
  </button>

  <!-- Delete -->
  <button
    class="operate-item item-danger"
    onclick={() => onDeleteSingleItem(activeMenuItem)}
  >
    <i class="ri-delete-bin-line op-icon"></i>
    <span>Delete</span>
  </button>

  <div class="operate-divider"></div>

  <!-- Compress -->
  <button
    class="operate-item"
    onclick={() => onOpenCompress(activeMenuItem)}
  >
    <i class="ri-file-zip-fill op-icon icon-cyan"></i>
    <span>Compress</span>
  </button>

  <!-- Properties -->
  <button
    class="operate-item"
    onclick={() => onOpenProperties(activeMenuItem)}
  >
    <i class="ri-information-line op-icon icon-blue"></i>
    <span>Properties</span>
  </button>
</div>

