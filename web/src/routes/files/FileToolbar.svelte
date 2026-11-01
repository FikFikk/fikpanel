<script lang="ts">
  // Subkomponen toolbar aksi utama (Upload, New, Favorite, Operations, View Mode) pada halaman File Manager
  import '../../styles/pages/files/file-toolbar.css';

  export type ToolbarMenuName = 'upload' | 'new' | 'favorite' | 'operations' | null;

  export interface ClipboardState {
    action: 'copy' | 'cut';
    paths: string[];
    names: string[];
  }

  interface Props {
    activeToolbarMenu: ToolbarMenuName;
    customFavorites: { name: string; path: string }[];
    selectedPaths: Set<string>;
    clipboard: ClipboardState | null;
    viewMode: 'list' | 'grid';
    onToggleToolbarMenu: (name: 'upload' | 'new' | 'favorite' | 'operations', e: MouseEvent) => void;
    onCloseAllMenus: () => void;
    onOpenUploadModal: () => void;
    onOpenCloudStorage: (name: string) => void;
    onOpenRemoteModal: () => void;
    onOpenNewModal: (type: 'file' | 'dir') => void;
    onOpenSymlinkModal: () => void;
    onOpenSearchContentModal: () => void;
    onJumpFavorite: (path: string) => void;
    onOpenFavManageModal: () => void;
    onNotify: (msg: string) => void;
    onBatchDelete: () => void;
    onPasteClipboard: () => void;
  }

  let {
    activeToolbarMenu = $bindable(),
    customFavorites,
    selectedPaths,
    clipboard = $bindable(),
    viewMode = $bindable(),
    onToggleToolbarMenu,
    onCloseAllMenus,
    onOpenUploadModal,
    onOpenCloudStorage,
    onOpenRemoteModal,
    onOpenNewModal,
    onOpenSymlinkModal,
    onOpenSearchContentModal,
    onJumpFavorite,
    onOpenFavManageModal,
    onNotify,
    onBatchDelete,
    onPasteClipboard,
  }: Props = $props();
</script>

<div class="pg-files-file-toolbar toolbar action-toolbar">
  <div class="tool-left">
    <!-- 1. Dropdown Upload -->
    <div class="tool-dropdown-wrap">
      <button
        class="tool-btn {activeToolbarMenu === 'upload' ? 'is-active' : ''}"
        onclick={(e) => onToggleToolbarMenu('upload', e)}
      >
        <i class="ri-upload-line"></i>
        <span>Upload &gt;</span>
      </button>

      {#if activeToolbarMenu === 'upload'}
        <div
          class="toolbar-dropdown-menu"
          role="menu"
          tabindex="-1"
          onclick={(e) => e.stopPropagation()}
          onkeydown={(e) => e.key === 'Escape' && onCloseAllMenus()}
        >
          <button
            class="tb-menu-item"
            onclick={() => {
              activeToolbarMenu = null;
              onOpenUploadModal();
            }}
          >
            <div class="tb-item-left">
              <i class="ri-upload-2-line"></i>
              <span>Upload File</span>
            </div>
          </button>
          <button class="tb-menu-item" onclick={() => onOpenCloudStorage('FTP Storage')}>
            <div class="tb-item-left">
              <i class="ri-server-line"></i>
              <span>FTP Storage</span>
            </div>
            <i class="ri-vip-diamond-fill badge-vip"></i>
          </button>
          <button class="tb-menu-item" onclick={() => onOpenCloudStorage('S3 Storage')}>
            <div class="tb-item-left">
              <i class="ri-cloud-line"></i>
              <span>S3 Storage</span>
            </div>
            <i class="ri-vip-diamond-fill badge-vip"></i>
          </button>
          <button class="tb-menu-item" onclick={() => onOpenCloudStorage('Google Drive')}>
            <div class="tb-item-left">
              <i class="ri-google-fill"></i>
              <span>Google Drive</span>
            </div>
            <i class="ri-vip-diamond-fill badge-vip"></i>
          </button>
          <button class="tb-menu-item" onclick={() => onOpenCloudStorage('Google Cloud Storage')}>
            <div class="tb-item-left">
              <i class="ri-cloudy-line"></i>
              <span>Google Cloud Storage</span>
            </div>
            <i class="ri-vip-diamond-fill badge-vip"></i>
          </button>
        </div>
      {/if}
    </div>

    <!-- 2. Remote Download -->
    <button class="tool-btn" onclick={onOpenRemoteModal}>
      <i class="ri-download-cloud-line icon-blue"></i>
      <span>Remote Download</span>
    </button>

    <!-- 3. Dropdown New -->
    <div class="tool-dropdown-wrap">
      <button
        class="tool-btn {activeToolbarMenu === 'new' ? 'is-active' : ''}"
        onclick={(e) => onToggleToolbarMenu('new', e)}
      >
        <i class="ri-folder-add-fill icon-amber"></i>
        <span>New &gt;</span>
      </button>

      {#if activeToolbarMenu === 'new'}
        <div
          class="toolbar-dropdown-menu"
          role="menu"
          tabindex="-1"
          onclick={(e) => e.stopPropagation()}
          onkeydown={(e) => e.key === 'Escape' && onCloseAllMenus()}
        >
          <button class="tb-menu-item" onclick={() => onOpenNewModal('dir')}>
            <div class="tb-item-left">
              <i class="ri-folder-fill icon-amber"></i>
              <span>New Directory</span>
            </div>
          </button>
          <button class="tb-menu-item" onclick={() => onOpenNewModal('file')}>
            <div class="tb-item-left">
              <i class="ri-file-3-line"></i>
              <span>New Blank File</span>
            </div>
          </button>
          <button
            class="tb-menu-item"
            onclick={() => {
              activeToolbarMenu = null;
              onOpenSymlinkModal();
            }}
          >
            <div class="tb-item-left">
              <i class="ri-links-line icon-blue"></i>
              <span>Soft Link</span>
            </div>
          </button>
        </div>
      {/if}
    </div>

    <!-- 4. Search File Content -->
    <button class="tool-btn" onclick={onOpenSearchContentModal}>
      <i class="ri-file-search-line"></i>
      <span>Search File Content</span>
    </button>

    <!-- 5. Dropdown Favorite -->
    <div class="tool-dropdown-wrap">
      <button
        class="tool-btn {activeToolbarMenu === 'favorite' ? 'is-active' : ''}"
        onclick={(e) => onToggleToolbarMenu('favorite', e)}
      >
        <i class="ri-star-fill icon-yellow"></i>
        <span>Favorite &gt;</span>
      </button>

      {#if activeToolbarMenu === 'favorite'}
        <div
          class="toolbar-dropdown-menu"
          role="menu"
          tabindex="-1"
          onclick={(e) => e.stopPropagation()}
          onkeydown={(e) => e.key === 'Escape' && onCloseAllMenus()}
        >
          {#each customFavorites as fav}
            <button
              class="tb-menu-item"
              onclick={() => {
                activeToolbarMenu = null;
                onJumpFavorite(fav.path);
              }}
            >
              <div class="tb-item-left">
                <i class="ri-folder-line icon-amber"></i>
                <span>{fav.name}</span>
              </div>
            </button>
          {/each}
          <div class="operate-divider"></div>
          <button
            class="tb-menu-item"
            onclick={() => {
              activeToolbarMenu = null;
              onOpenFavManageModal();
            }}
          >
            <div class="tb-item-left">
              <i class="ri-settings-3-line"></i>
              <span>Management</span>
            </div>
          </button>
        </div>
      {/if}
    </div>

    <!-- 6. Share List -->
    <button
      class="tool-btn"
      onclick={() => onNotify('Fitur Share List: tautan unduhan langsung terdaftar di menu Properties')}
    >
      <i class="ri-share-line icon-green"></i>
      <span>Share List</span>
    </button>

    <!-- 7. File Operations -->
    <div class="tool-dropdown-wrap">
      <button
        class="tool-btn {activeToolbarMenu === 'operations' ? 'is-active' : ''}"
        onclick={(e) => onToggleToolbarMenu('operations', e)}
      >
        <i class="ri-folder-settings-line icon-amber"></i>
        <span>File Operations &gt;</span>
      </button>

      {#if activeToolbarMenu === 'operations'}
        <div
          class="toolbar-dropdown-menu"
          role="menu"
          tabindex="-1"
          onclick={(e) => e.stopPropagation()}
          onkeydown={(e) => e.key === 'Escape' && onCloseAllMenus()}
        >
          <button
            class="tb-menu-item"
            disabled={selectedPaths.size === 0}
            onclick={() => {
              if (selectedPaths.size > 0) {
                clipboard = {
                  action: 'copy',
                  paths: Array.from(selectedPaths),
                  names: Array.from(selectedPaths).map((p) => p.split(/[/\\]+/).pop() || ''),
                };
                onNotify(`${selectedPaths.size} item disalin ke clipboard`);
                activeToolbarMenu = null;
              }
            }}
          >
            <div class="tb-item-left">
              <i class="ri-file-copy-line"></i>
              <span>Copy ({selectedPaths.size})</span>
            </div>
          </button>
          <button
            class="tb-menu-item"
            disabled={selectedPaths.size === 0}
            onclick={() => {
              if (selectedPaths.size > 0) {
                clipboard = {
                  action: 'cut',
                  paths: Array.from(selectedPaths),
                  names: Array.from(selectedPaths).map((p) => p.split(/[/\\]+/).pop() || ''),
                };
                onNotify(`${selectedPaths.size} item dipotong ke clipboard`);
                activeToolbarMenu = null;
              }
            }}
          >
            <div class="tb-item-left">
              <i class="ri-scissors-line icon-cyan"></i>
              <span>Cut ({selectedPaths.size})</span>
            </div>
          </button>
          <button
            class="tb-menu-item item-danger"
            disabled={selectedPaths.size === 0}
            onclick={() => {
              activeToolbarMenu = null;
              onBatchDelete();
            }}
          >
            <div class="tb-item-left">
              <i class="ri-delete-bin-line"></i>
              <span>Delete ({selectedPaths.size})</span>
            </div>
          </button>
        </div>
      {/if}
    </div>

    <!-- Tombol tempel bila clipboard aktif -->
    {#if clipboard}
      <div class="paste-indicator">
        <button
          class="tool-btn tool-primary"
          onclick={onPasteClipboard}
          title="Klik untuk menempel di direktori ini"
        >
          <i class="ri-clipboard-line"></i>
          <span>Paste ({clipboard.names.length} item)</span>
        </button>
        <button
          class="btn-cancel-clip"
          onclick={() => (clipboard = null)}
          title="Batalkan salin/potong"
        >&times;</button>
      </div>
    {/if}
  </div>

  <div class="tool-right">
    <div class="view-toggle">
      <button
        class="toggle-btn {viewMode === 'list' ? 'active' : ''}"
        onclick={() => (viewMode = 'list')}
        title="Tampilan Daftar"
      >
        <i class="ri-list-check"></i>
      </button>
      <button
        class="toggle-btn {viewMode === 'grid' ? 'active' : ''}"
        onclick={() => (viewMode = 'grid')}
        title="Tampilan Grid"
      >
        <i class="ri-grid-fill"></i>
      </button>
    </div>
  </div>
</div>

