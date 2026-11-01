<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DatabaseItem } from '../../lib/types';
  import '../../styles/pages/databases/quota-modal.css';

  // Antarmuka properti untuk modal atur kuota database
  interface Props {
    open: boolean;
    quotaTargetDb: DatabaseItem | null;
    quotaMB: number;
    savingQuota: boolean;
    onFormatBytes: (bytes: number) => string;
    onClose: () => void;
    onSave: () => void;
  }

  let {
    open,
    quotaTargetDb,
    quotaMB = $bindable(),
    savingQuota,
    onFormatBytes,
    onClose,
    onSave,
  }: Props = $props();
</script>

<!-- Modal Atur Kuota Kapasitas Database (Screenshot 3 aaPanel) -->
<Modal
  {open}
  title={quotaTargetDb ? `Atur Kuota Baru [${quotaTargetDb.name}]` : 'Atur Kuota Baru'}
  size="md"
  onclose={() => {
    if (!savingQuota) onClose();
  }}
>
  {#if quotaTargetDb}
    <div class="pg-databases-quota-modal">
      <div class="form-group">
        <label for="quota-used-info" class="form-label">Kapasitas Terpakai Saat Ini</label>
        <div class="input-with-unit">
          <input
            id="quota-used-info"
            type="text"
            class="form-input"
            value={onFormatBytes(quotaTargetDb.size_bytes)}
            disabled
          />
        </div>
      </div>

      <div class="form-group">
        <label for="quota-val-input" class="form-label">Kapasitas Batas Kuota (Quota)</label>
        <div class="input-with-unit">
          <input
            id="quota-val-input"
            type="number"
            class="form-input"
            min="0"
            bind:value={quotaMB}
            disabled={savingQuota}
          />
          <span class="unit-badge">MB</span>
        </div>
      </div>

      <div class="quota-reminder-box">
        <p class="reminder-line">
          • <strong>Catatan:</strong> Fitur pembatasan kuota kapasitas database menggunakan kuota sistem file OS (XFS / ext4 prjquota).
        </p>
        <p class="reminder-line">
          • Untuk menonaktifkan atau membatalkan batasan kuota, isi nilai dengan <strong>"0"</strong>.
        </p>
      </div>
    </div>
  {/if}

  {#snippet footer()}
    <button class="btn btn-outline" disabled={savingQuota} onclick={onClose}>
      Batal
    </button>
    <button class="btn btn-primary" disabled={savingQuota} onclick={onSave}>
      Konfirmasi Kuota
    </button>
  {/snippet}
</Modal>
