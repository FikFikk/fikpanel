<script lang="ts">
  import '../../styles/pages/phpmanager/fpm-config-tab.css';
  import CodeMirrorEditor from '../../lib/components/ui/CodeMirrorEditor.svelte';

  // Antarmuka properti untuk tab editor konfigurasi pool FPM www.conf
  interface Props {
    activeVersion: string;
    rawFpmPath: string;
    isRawFpmDirty: boolean;
    rawFpmContent: string;
    loadingRawFpm: boolean;
    savingRawFpm: boolean;
    onFetchRawFpm: (version: string) => void;
    onSubmitRawFpm: () => void;
  }

  let {
    activeVersion,
    rawFpmPath,
    isRawFpmDirty,
    rawFpmContent = $bindable(),
    loadingRawFpm,
    savingRawFpm,
    onFetchRawFpm,
    onSubmitRawFpm,
  }: Props = $props();
</script>

<!-- Tab 5: Raw FPM www.conf -->
<div class="pg-phpmanager-fpm-config-tab tab-pane full-height-pane">
  <div class="editor-tips-bar flex justify-between items-center flex-wrap gap-2">
    <div class="tips-text">
      <span class="font-semibold text-accent"><i class="ri-settings-line" aria-hidden="true"></i> Pool Config:</span>
      <span><kbd>Ctrl+F</kbd> Cari, <kbd>Ctrl+S</kbd> Simpan berkas pool www.conf</span>
    </div>
    <div class="flex items-center gap-2 flex-wrap">
      <span class="file-path-badge mono">{rawFpmPath || 'pool.d/www.conf'}</span>
      {#if isRawFpmDirty}
        <span class="badge badge--warning">Belum Disimpan</span>
      {/if}
      <button
        type="button"
        class="btn btn--ghost btn--xs"
        onclick={() => onFetchRawFpm(activeVersion)}
        disabled={loadingRawFpm}
        title="Muat ulang www.conf dari disk"
      >
        <i class="ri-restart-line" aria-hidden="true"></i> Muat Ulang
      </button>
    </div>
  </div>

  <div class="code-editor-wrapper flex-1">
    {#if loadingRawFpm}
      <div class="editor-loading">
        <i class="ri-loader-4-line spin text-2xl"></i>
        <span>Membaca berkas pool www.conf...</span>
      </div>
    {:else}
      <CodeMirrorEditor
        bind:value={rawFpmContent}
        lang="ini"
        fill={true}
        onSave={onSubmitRawFpm}
      />
    {/if}
  </div>

  <div class="editor-footer flex justify-between items-center flex-wrap gap-2">
    <p class="editor-note text-xs text-subtle">
      Berkas pool FPM www.conf asli. Sintaks diuji via <code class="mono">php-fpm -t</code> dengan rollback otomatis bila tidak valid.
    </p>
    <button type="button" class="btn btn--primary" onclick={onSubmitRawFpm} disabled={savingRawFpm || loadingRawFpm}>
      <i class="ri-save-line" aria-hidden="true"></i> {savingRawFpm ? 'Menyimpan...' : 'Simpan & Reload'}
    </button>
  </div>
</div>
