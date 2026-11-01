<script lang="ts">
  import '../../styles/pages/phpmanager/fpm-config-tab.css';

  // Antarmuka properti untuk tab editor konfigurasi pool FPM www.conf
  interface Props {
    rawFpmPath: string;
    rawFpmContent: string;
    loadingRawFpm: boolean;
    savingRawFpm: boolean;
    onHandleEditorKeydown: (e: KeyboardEvent) => void;
    onSubmitRawFpm: () => void;
  }

  let {
    rawFpmPath,
    rawFpmContent = $bindable(),
    loadingRawFpm,
    savingRawFpm,
    onHandleEditorKeydown,
    onSubmitRawFpm,
  }: Props = $props();
</script>

<!-- Tab 5: Raw FPM www.conf -->
<div class="pg-phpmanager-fpm-config-tab tab-pane full-height-pane">
  <div class="editor-tips-bar flex justify-between items-center">
    <div class="tips-text">
      <span class="font-semibold text-accent"><i class="ri-settings-line"></i> Pool Config:</span>
      <span><kbd>Ctrl+S</kbd> Simpan berkas pool www.conf</span>
    </div>
    <span class="file-path-badge mono">{rawFpmPath || 'pool.d/www.conf'}</span>
  </div>

  <div class="code-editor-wrapper flex-1">
    {#if loadingRawFpm}
      <div class="editor-loading">
        <i class="ri-loader-4-line spin text-2xl"></i>
        <span>Membaca berkas pool www.conf...</span>
      </div>
    {:else}
      <textarea
        class="editor-textarea mono p-4"
        bind:value={rawFpmContent}
        onkeydown={onHandleEditorKeydown}
        spellcheck="false"
        wrap="off"
      ></textarea>
    {/if}
  </div>

  <div class="editor-footer flex justify-between items-center">
    <p class="editor-note text-xs text-subtle">
      • Berkas pool FPM www.conf asli. Mengubah konfigurasi socket atau user worker di sini akan langsung di-reload ke daemon.
    </p>
    <button class="btn btn-primary" onclick={onSubmitRawFpm} disabled={savingRawFpm || loadingRawFpm}>
      <i class="ri-save-line"></i> {savingRawFpm ? 'Menyimpan...' : 'Save'}
    </button>
  </div>
</div>
