<script lang="ts">
  /**
   * Subkomponen Editor Berkas Teks Modern (FikPanel Code Studio) untuk Halaman Files.
   */
  import type { FileItem } from '../../lib/types';
  import '../../styles/pages/files/code-editor.css';

  interface Props {
    editorTarget: FileItem;
    editorContent: string;
    editorSaving: boolean;
    editorFontSize: number;
    editorWordWrap: boolean;
    editorIsFullscreen: boolean;
    editorCursorPos: { line: number; col: number };
    editorShowSearch: boolean;
    editorSearchQuery: string;
    editorReplaceQuery: string;
    editorSearchMatches: number[];
    editorSearchMatchIndex: number;
    editorTextareaEl: HTMLTextAreaElement | null;
    editorGutterEl: HTMLDivElement | null;
    editorIsDirty: boolean;
    editorLineCount: number;
    editorLines: number[];
    editorLangInfo: { label: string; icon: string; color: string };
    formatBytes: (bytes: number) => string;
    onClose: () => void;
    onSave: () => void;
    onRefresh: () => void;
    onJumpLine: () => void;
    onSearch: (dir: 'next' | 'prev') => void;
    onReplace: () => void;
    onReplaceAll: () => void;
    onScroll: () => void;
    onKeydown: (e: KeyboardEvent) => void;
    onUpdateCursor: () => void;
  }

  let {
    editorTarget,
    editorContent = $bindable(),
    editorSaving,
    editorFontSize = $bindable(),
    editorWordWrap = $bindable(),
    editorIsFullscreen = $bindable(),
    editorCursorPos,
    editorShowSearch = $bindable(),
    editorSearchQuery = $bindable(),
    editorReplaceQuery = $bindable(),
    editorSearchMatches,
    editorSearchMatchIndex,
    editorTextareaEl = $bindable(),
    editorGutterEl = $bindable(),
    editorIsDirty,
    editorLineCount,
    editorLines,
    editorLangInfo,
    formatBytes,
    onClose,
    onSave,
    onRefresh,
    onJumpLine,
    onSearch,
    onReplace,
    onReplaceAll,
    onScroll,
    onKeydown,
    onUpdateCursor
  }: Props = $props();
</script>

<div
  class="modal-backdrop editor-backdrop pg-files-code-editor"
  role="dialog"
  aria-modal="true"
  onclick={(e) => {
    if (e.target === e.currentTarget) onClose();
  }}
  onkeydown={(e) => {
    if (e.key === 'Escape') onClose();
  }}
  tabindex="-1"
>
  <div class="studio-window {editorIsFullscreen ? 'is-fullscreen' : ''}">
    <!-- Bar Judul: Tab Berkas Aktif + Breadcrumb + Kontrol Jendela -->
    <div class="studio-titlebar">
      <div class="studio-tab-bar">
        <div class="studio-tab active">
          <i
            class="{editorLangInfo.icon} tab-icon"
            style="color: {editorLangInfo.color};"
          ></i>
          <span class="tab-title">{editorTarget.name}</span>
          {#if editorIsDirty}
            <span class="dirty-dot" title="Ada perubahan belum disimpan">●</span>
          {/if}
        </div>
        <div class="studio-path-crumbs mono" title={editorTarget.path}>
          <i class="ri-folder-2-line"></i>
          <span>{editorTarget.path}</span>
        </div>
      </div>

      <div class="studio-win-controls">
        <button
          class="win-ctrl-btn"
          onclick={() => (editorIsFullscreen = !editorIsFullscreen)}
          title={editorIsFullscreen ? 'Keluar Layar Penuh' : 'Layar Penuh'}
        >
          <i
            class={editorIsFullscreen
              ? 'ri-fullscreen-exit-line'
              : 'ri-fullscreen-line'}
          ></i>
        </button>
        <button
          class="win-ctrl-btn btn-win-close"
          onclick={onClose}
          title="Tutup Editor"
        >
          <i class="ri-close-line"></i>
        </button>
      </div>
    </div>

    <!-- Toolbar Studio Modern -->
    <div class="studio-toolbar">
      <div class="tb-left">
        <button
          class="studio-btn studio-btn-save {editorIsDirty ? 'glow-green' : ''}"
          onclick={onSave}
          disabled={editorSaving}
          title="Simpan berkas (Ctrl+S)"
        >
          {#if editorSaving}
            <i class="ri-loader-4-line spin"></i>
            <span>Menyimpan...</span>
          {:else}
            <i class="ri-save-3-line"></i>
            <span>Simpan</span>
            <span class="kbd-hint">Ctrl+S</span>
          {/if}
        </button>

        <button
          class="studio-btn"
          onclick={onRefresh}
          title="Muat ulang isi berkas dari server (Refresh)"
        >
          <i class="ri-refresh-line"></i>
          <span>Refresh</span>
        </button>

        <button
          class="studio-btn {editorShowSearch ? 'active' : ''}"
          onclick={() => (editorShowSearch = !editorShowSearch)}
          title="Cari & Ganti Teks (Ctrl+F)"
        >
          <i class="ri-search-line"></i>
          <span>Cari & Ganti</span>
        </button>

        <button
          class="studio-btn"
          onclick={onJumpLine}
          title="Lompat ke Baris (Ctrl+G)"
        >
          <i class="ri-arrow-right-up-line"></i>
          <span>Lompat Baris</span>
        </button>
      </div>

      <div class="tb-right">
        <!-- Pilihan Ukuran Font -->
        <div class="tool-group">
          <span class="tool-label">Font:</span>
          <button
            class="font-step-btn"
            onclick={() => (editorFontSize = Math.max(11, editorFontSize - 1))}
            title="Perkecil Font"
          >A-</button>
          <span class="font-size-val mono">{editorFontSize}px</span>
          <button
            class="font-step-btn"
            onclick={() => (editorFontSize = Math.min(22, editorFontSize + 1))}
            title="Perbesar Font"
          >A+</button>
        </div>

        <!-- Toggle Word Wrap -->
        <button
          class="studio-btn {editorWordWrap ? 'active' : ''}"
          onclick={() => (editorWordWrap = !editorWordWrap)}
          title="Toggle Word Wrap"
        >
          <i class="ri-text-wrap"></i>
          <span>Wrap</span>
        </button>
      </div>
    </div>

    <!-- Panel Pencarian dan Penggantian Teks Floating -->
    {#if editorShowSearch}
      <div class="studio-search-bar">
        <div class="search-row">
          <div class="search-input-wrap">
            <i class="ri-search-line search-icon"></i>
            <input
              type="text"
              placeholder="Cari kata kunci..."
              bind:value={editorSearchQuery}
              onkeydown={(e) => {
                if (e.key === 'Enter') onSearch(e.shiftKey ? 'prev' : 'next');
                if (e.key === 'Escape') editorShowSearch = false;
              }}
            />
            {#if editorSearchMatches.length > 0}
              <span class="match-count mono"
                >{editorSearchMatchIndex + 1} dari {editorSearchMatches.length}</span
              >
            {:else if editorSearchQuery}
              <span class="match-count mono text-muted">0 hasil</span>
            {/if}
          </div>

          <div class="search-nav-btns">
            <button
              class="search-action-btn"
              onclick={() => onSearch('prev')}
              title="Sebelumnya (Shift+Enter)"
            >
              <i class="ri-arrow-up-s-line"></i>
            </button>
            <button
              class="search-action-btn"
              onclick={() => onSearch('next')}
              title="Berikutnya (Enter)"
            >
              <i class="ri-arrow-down-s-line"></i>
            </button>
          </div>

          <div class="replace-input-wrap">
            <i class="ri-find-replace-line search-icon"></i>
            <input
              type="text"
              placeholder="Ganti dengan..."
              bind:value={editorReplaceQuery}
              onkeydown={(e) => {
                if (e.key === 'Enter') onReplace();
                if (e.key === 'Escape') editorShowSearch = false;
              }}
            />
          </div>

          <button
            class="search-action-btn replace-btn"
            onclick={onReplace}
            title="Ganti yang dipilih"
          >
            <span>Ganti</span>
          </button>
          <button
            class="search-action-btn replace-btn"
            onclick={onReplaceAll}
            title="Ganti semua kemunculan"
          >
            <span>Ganti Semua</span>
          </button>

          <button
            class="search-close-btn"
            onclick={() => (editorShowSearch = false)}
            title="Tutup Pencarian"
          >
            <i class="ri-close-line"></i>
          </button>
        </div>
      </div>
    {/if}

    <!-- Workspace: Gutter Nomor Baris + Textarea Editor -->
    <div class="studio-body">
      <div class="studio-gutter mono" bind:this={editorGutterEl}>
        {#each editorLines as lineNum}
          <div
            class="gutter-line {editorCursorPos.line === lineNum
              ? 'active-line'
              : ''}"
          >
            {lineNum}
          </div>
        {/each}
      </div>

      <div class="studio-canvas">
        <textarea
          class="studio-textarea mono"
          bind:this={editorTextareaEl}
          bind:value={editorContent}
          style="font-size: {editorFontSize}px; white-space: {editorWordWrap
            ? 'pre-wrap'
            : 'pre'};"
          spellcheck="false"
          onscroll={onScroll}
          onkeydown={onKeydown}
          onclick={onUpdateCursor}
          onkeyup={onUpdateCursor}
          onselect={onUpdateCursor}
        ></textarea>
      </div>
    </div>

    <!-- Status Bar Bawah Studio -->
    <div class="studio-statusbar mono">
      <div class="sb-left">
        <span class="sb-item" title="Path berkas">
          <i class="ri-file-text-line"></i>
          <span class="sb-text">{editorTarget.path}</span>
        </span>
        <span class="sb-divider">|</span>
        <span class="sb-item">{editorLineCount} baris</span>
        <span class="sb-item">{editorContent.length} karakter</span>
        <span class="sb-item">{formatBytes(editorTarget.size)}</span>
      </div>

      <div class="sb-right">
        <span class="sb-item cursor-pos"
          >Ln {editorCursorPos.line}, Col {editorCursorPos.col}</span
        >
        <span class="sb-divider">|</span>
        <span class="sb-item">LF</span>
        <span class="sb-divider">|</span>
        <span class="sb-item">Spasi: 2</span>
        <span class="sb-divider">|</span>
        <span class="sb-item">UTF-8</span>
        <span class="sb-divider">|</span>
        <span
          class="sb-item lang-tag"
          style="color: {editorLangInfo.color};"
        >
          <i class={editorLangInfo.icon}></i>
          <span>{editorLangInfo.label}</span>
        </span>
        <span class="sb-divider">|</span>
        {#if editorSaving}
          <span class="save-status saving">
            <i class="ri-loader-4-line spin"></i>
            <span>Menyimpan...</span>
          </span>
        {:else if editorIsDirty}
          <span class="save-status unsaved">
            <span class="dirty-indicator">●</span>
            <span>Belum disimpan</span>
          </span>
        {:else}
          <span class="save-status saved">
            <i class="ri-checkbox-circle-fill"></i>
            <span>Tersimpan</span>
          </span>
        {/if}
      </div>
    </div>
  </div>
</div>

