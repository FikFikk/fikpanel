<script lang="ts">
  import '../../styles/pages/phpmanager/config-file-tab.css';

  // Antarmuka properti untuk tab editor konfigurasi berkas php.ini
  interface Props {
    activeVersion: string;
    rawIniPath: string;
    isRawIniDirty: boolean;
    rawIniSearch: string;
    rawIniContent: string;
    rawIniLineNumbers: number[];
    rawIniTextarea?: HTMLTextAreaElement;
    loadingRawIni: boolean;
    savingRawIni: boolean;
    onPerformEditorSearch: (forward?: boolean) => void;
    onFetchRawIni: (version: string) => void;
    onHandleEditorKeydown: (e: KeyboardEvent) => void;
    onEditorScroll: (e: Event) => void;
    onSubmitRawIni: () => void;
  }

  let {
    activeVersion,
    rawIniPath,
    isRawIniDirty,
    rawIniSearch = $bindable(),
    rawIniContent = $bindable(),
    rawIniLineNumbers,
    rawIniTextarea = $bindable(),
    loadingRawIni,
    savingRawIni,
    onPerformEditorSearch,
    onFetchRawIni,
    onHandleEditorKeydown,
    onEditorScroll,
    onSubmitRawIni,
  }: Props = $props();
</script>

<!-- Tab 3: Configuration File (Editor php.ini ala Gambar 4 & 5) -->
<div class="pg-phpmanager-config-file-tab tab-pane full-height-pane">
  <!-- Tips Bar ala aaPanel Screenshot 4 -->
  <div class="editor-tips-bar flex justify-between items-center">
    <div class="tips-text">
      <span class="font-semibold text-accent"><i class="ri-lightbulb-line"></i> Tips:</span>
      <span><kbd>Ctrl+F</kbd> Cari kata kunci, <kbd>Ctrl+S</kbd> Simpan berkas, <kbd>Tab</kbd> Indentasi 4 spasi</span>
    </div>
    <div class="flex items-center gap-2">
      <span class="file-path-badge mono" title={rawIniPath}>
        <i class="ri-file-text-line"></i> {rawIniPath || 'php.ini'}
      </span>
      {#if isRawIniDirty}
        <span class="badge badge--warning">Belum Disimpan</span>
      {/if}
    </div>
  </div>

  <!-- Search Bar Interaktif -->
  <div class="editor-search-bar flex items-center gap-2">
    <div class="search-input-wrap flex-1">
      <i class="ri-search-line search-icon"></i>
      <input
        id="editor-search-input"
        type="text"
        class="editor-search-input"
        placeholder="Ketik kata kunci untuk mencari di php.ini (misal: memory_limit, upload_max, disable_functions)..."
        bind:value={rawIniSearch}
        onkeydown={(e) => {
          if (e.key === 'Enter') onPerformEditorSearch(!e.shiftKey);
        }}
      />
    </div>
    <button class="btn btn-ghost btn-sm" onclick={() => onPerformEditorSearch(false)} title="Cari sebelumnya (Shift+Enter)">
      <i class="ri-arrow-up-s-line"></i>
    </button>
    <button class="btn btn-ghost btn-sm" onclick={() => onPerformEditorSearch(true)} title="Cari selanjutnya (Enter)">
      <i class="ri-arrow-down-s-line"></i>
    </button>
    <button class="btn btn-ghost btn-sm" onclick={() => onFetchRawIni(activeVersion)} disabled={loadingRawIni} title="Muat ulang dari disk (Batalkan perubahan)">
      <i class="ri-restart-line"></i> Reset
    </button>
  </div>

  <!-- Textarea Code Editor dengan Nomor Baris Gutter (Screenshot 4 & 5) -->
  <div class="code-editor-wrapper flex-1">
    {#if loadingRawIni}
      <div class="editor-loading">
        <i class="ri-loader-4-line spin text-2xl"></i>
        <span>Membaca berkas php.ini...</span>
      </div>
    {:else}
      <div class="editor-gutter mono" id="editor-gutter">
        {#each rawIniLineNumbers as num}
          <div class="gutter-num">{num}</div>
        {/each}
      </div>
      <textarea
        bind:this={rawIniTextarea}
        class="editor-textarea mono"
        bind:value={rawIniContent}
        onkeydown={onHandleEditorKeydown}
        onscroll={onEditorScroll}
        spellcheck="false"
        wrap="off"
        placeholder="[PHP] ..."
      ></textarea>
    {/if}
  </div>

  <!-- Footer Editor ala aaPanel Screenshot 4 -->
  <div class="editor-footer flex justify-between items-center">
    <p class="editor-note text-xs text-subtle">
      • Berkas konfigurasi utama PHP {activeVersion}. Jangan mengedit sembarangan jika belum memahami format sintaks direktif PHP INI.
    </p>
    <button class="btn btn-primary" onclick={onSubmitRawIni} disabled={savingRawIni || loadingRawIni}>
      <i class="ri-save-line" aria-hidden="true"></i> {savingRawIni ? 'Menyimpan...' : 'Save'}
    </button>
  </div>
</div>
