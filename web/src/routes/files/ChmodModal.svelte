<script lang="ts">
  /**
   * Subkomponen Modal Atur Perizinan (Permission / chmod) untuk Halaman Files.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { FileItem } from '../../lib/types';
  import '../../styles/pages/files/chmod-modal.css';

  interface ChmodPermissions {
    ownerR: boolean;
    ownerW: boolean;
    ownerX: boolean;
    groupR: boolean;
    groupW: boolean;
    groupX: boolean;
    publicR: boolean;
    publicW: boolean;
    publicX: boolean;
  }

  interface Props {
    open: boolean;
    chmodTarget: FileItem | null;
    chmodPerms: ChmodPermissions;
    chmodOctal: string;
    onClose: () => void;
    onSave: () => void;
  }

  let {
    open,
    chmodTarget,
    chmodPerms = $bindable(),
    chmodOctal,
    onClose,
    onSave
  }: Props = $props();
</script>

<Modal
  {open}
  title="Atur Perizinan (Permission / chmod)"
  size="md"
  onclose={onClose}
>
  {#if chmodTarget}
    <div class="pg-files-chmod-modal">
      <div class="pmsn-body">
        <p class="target-name mono">{chmodTarget.name}</p>
        <div class="chmod-matrix">
          <div class="matrix-col">
            <h4>Owner</h4>
            <label
              ><input type="checkbox" bind:checked={chmodPerms.ownerR} /> Read (4)</label
            >
            <label
              ><input type="checkbox" bind:checked={chmodPerms.ownerW} /> Write (2)</label
            >
            <label
              ><input type="checkbox" bind:checked={chmodPerms.ownerX} /> Execute (1)</label
            >
          </div>
          <div class="matrix-col">
            <h4>Group</h4>
            <label
              ><input type="checkbox" bind:checked={chmodPerms.groupR} /> Read (4)</label
            >
            <label
              ><input type="checkbox" bind:checked={chmodPerms.groupW} /> Write (2)</label
            >
            <label
              ><input type="checkbox" bind:checked={chmodPerms.groupX} /> Execute (1)</label
            >
          </div>
          <div class="matrix-col">
            <h4>Public</h4>
            <label
              ><input type="checkbox" bind:checked={chmodPerms.publicR} /> Read (4)</label
            >
            <label
              ><input type="checkbox" bind:checked={chmodPerms.publicW} /> Write (2)</label
            >
            <label
              ><input type="checkbox" bind:checked={chmodPerms.publicX} /> Execute (1)</label
            >
          </div>
        </div>

        <div class="octal-display">
          <span>Nilai chmod:</span>
          <span class="octal-badge mono">{chmodOctal}</span>
        </div>
      </div>
    </div>
  {/if}
  {#snippet footer()}
    <button
      type="button"
      class="btn btn--ghost"
      onclick={onClose}
    >Batal</button>
    <button
      type="button"
      class="btn btn--primary"
      onclick={onSave}
    >
      Terapkan Izin
    </button>
  {/snippet}
</Modal>

