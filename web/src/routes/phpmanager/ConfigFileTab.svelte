<script lang="ts">
  import '../../styles/pages/phpmanager/config-file-tab.css';
  import CodeMirrorEditor from '../../lib/components/ui/CodeMirrorEditor.svelte';

  // Antarmuka properti untuk tab editor konfigurasi berkas php.ini
  interface Props {
    activeVersion: string;
    rawIniPath: string;
    isRawIniDirty: boolean;
    rawIniContent: string;
    loadingRawIni: boolean;
    savingRawIni: boolean;
    onFetchRawIni: (version: string) => void;
    onSubmitRawIni: () => void;
  }

  let {
    activeVersion,
    rawIniPath,
    isRawIniDirty,
    rawIniContent = $bindable(),
    loadingRawIni,
    savingRawIni,
    onFetchRawIni,
    onSubmitRawIni,
  }: Props = $props();
</script>

<!-- Tab 3: Configuration File (Editor php.ini berbasis CodeMirror 6) -->
<div class="pg-phpmanager-config-file-tab tab-pane full-height-pane">
  <!-- Tips Bar & Status Berkas -->
  <div class="editor-tips-bar flex justify-between items-center flex-wrap gap-2">
    <div class="tips-text">
      <span class="font-semibold text-accent"><i class="ri-lightbulb-line" aria-hidden="true"></i> Tips:</span>
      <span><kbd>Ctrl+F</kbd> Cari, <kbd>Ctrl+H</kbd> Ganti, <kbd>Ctrl+S</kbd> Simpan</span>
    </div>
    <div class="flex items-center gap-2 flex-wrap">
      <span class="file-path-badge mono" title={rawIniPath}>
        <i class="ri-file-text-line" aria-hidden="true"></i> {rawIniPath || 'php.ini'}
      </span>
      {#if isRawIniDirty}
        <span class="badge badge--warning">Belum Disimpan</span>
      {/if}
      <button
        type="button"
        class="btn btn--ghost btn--xs"
        onclick={() => onFetchRawIni(activeVersion)}
        disabled={loadingRawIni}
        title="Muat ulang dari disk (Batalkan perubahan)"
      >
        <i class="ri-restart-line" aria-hidden="true"></i> Muat Ulang
      </button>
    </div>
  </div>

  <!-- Editor CodeMirror 6 -->
  <div class="code-editor-wrapper flex-1">
    {#if loadingRawIni}
      <div class="editor-loading">
        <i class="ri-loader-4-line spin text-2xl"></i>
        <span>Membaca berkas php.ini...</span>
      </div>
    {:else}
      <CodeMirrorEditor
        bind:value={rawIniContent}
        lang="ini"
        fill={true}
        placeholder="[PHP] ..."
        onSave={onSubmitRawIni}
      />
    {/if}
  </div>

  <!-- Footer Editor -->
  <div class="editor-footer flex justify-between items-center flex-wrap gap-2">
    <p class="editor-note text-xs text-subtle">
      Berkas utama PHP {activeVersion}. Sintaks diverifikasi otomatis via <code class="mono">php-fpm -t</code> dengan rollback cadangan bila gagal.
    </p>
    <button type="button" class="btn btn--primary" onclick={onSubmitRawIni} disabled={savingRawIni || loadingRawIni}>
      <i class="ri-save-line" aria-hidden="true"></i> {savingRawIni ? 'Menyimpan...' : 'Simpan & Reload'}
    </button>
  </div>
</div>
