<script lang="ts">
  /**
   * Subkomponen Modal Detail Properti / Metadata Berkas untuk Halaman Files.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { FileItem } from '../../lib/types';
  import '../../styles/pages/files/properties-modal.css';

  interface Props {
    open: boolean;
    propTarget: FileItem | null;
    formatBytes: (bytes: number) => string;
    onClose: () => void;
  }

  let {
    open,
    propTarget,
    formatBytes,
    onClose
  }: Props = $props();
</script>

<Modal
  {open}
  title={`Properties: ${propTarget?.name || ''}`}
  size="md"
  onclose={onClose}
>
  {#if propTarget}
    <div class="pg-files-properties-modal">
      <div class="prop-table">
        <div class="prop-row">
          <span class="prop-label">Name:</span>
          <span class="prop-val font-bold">{propTarget.name}</span>
        </div>
        <div class="prop-row">
          <span class="prop-label">Full Path:</span>
          <span class="prop-val mono select-all">{propTarget.path}</span>
        </div>
        <div class="prop-row">
          <span class="prop-label">Type:</span>
          <span class="prop-val"
            >{propTarget.is_dir
              ? 'Directory (Folder)'
              : `File (${propTarget.ext || 'Plain'})`}</span
          >
        </div>
        <div class="prop-row">
          <span class="prop-label">Size:</span>
          <span class="prop-val mono"
            >{formatBytes(propTarget.size)} ({propTarget.size} bytes)</span
          >
        </div>
        <div class="prop-row">
          <span class="prop-label">PMSN / Permissions:</span>
          <span class="prop-val mono badge-green">{propTarget.mode}</span>
        </div>
        <div class="prop-row">
          <span class="prop-label">Owner:</span>
          <span class="prop-val mono">{propTarget.owner}</span>
        </div>
        <div class="prop-row">
          <span class="prop-label">Modified Time:</span>
          <span class="prop-val mono">{propTarget.mod_time}</span>
        </div>
      </div>
    </div>
  {/if}
  {#snippet footer()}
    <button
      class="btn btn--primary"
      onclick={onClose}
    >Tutup</button>
  {/snippet}
</Modal>

