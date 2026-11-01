<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { Site } from '../../lib/types';
  import '../../styles/pages/sites/delete-modal.css';

  // Antarmuka properti untuk modal konfirmasi hapus situs
  interface Props {
    open: boolean;
    deleteTarget: Site | null;
    backupFirstOpt: boolean;
    deleteFilesOpt: boolean;
    deleteDBOpt: boolean;
    deleteConfirmInput: string;
    deletingSite: boolean;
    deletePasteWarning: boolean;
    onClose: () => void;
    onConfirmDelete: () => void;
    onBlockPaste: (e: Event) => void;
    getProjectRootDir: (s: Site) => string;
    getPredictedDBName: (s: Site) => string;
  }

  let {
    open,
    deleteTarget,
    backupFirstOpt = $bindable(),
    deleteFilesOpt = $bindable(),
    deleteDBOpt = $bindable(),
    deleteConfirmInput = $bindable(),
    deletingSite,
    deletePasteWarning,
    onClose,
    onConfirmDelete,
    onBlockPaste,
    getProjectRootDir,
    getPredictedDBName,
  }: Props = $props();
</script>

<Modal
  {open}
  title={deleteTarget ? `Hapus Situs "${deleteTarget.domain}"?` : 'Hapus Situs'}
  size="lg"
  onclose={onClose}
>
  {#if deleteTarget}
    <div class="pg-sites-delete-modal">
      <span class="danger-sub-caption">Tindakan destruktif &amp; permanen pada server produksi</span>

      <!-- Banner Peringatan Dampak -->
      <div class="danger-alert-banner">
        <i class="ri-alarm-warning-line"></i>
        <div>
          <strong>Peringatan Penting Sebelum Menghapus!</strong>
          <p>
            Konfigurasi virtual host Nginx, sertifikat SSL, serta komponen yang Anda centang di bawah akan dihapus secara permanen.
            Jika database dan kode proyek ini penting, pastikan opsi <strong>Auto-Backup Sebelum Hapus</strong> tetap aktif.
          </p>
        </div>
      </div>

      <!-- Opsi Granular Dampak Penghapusan & Cadangan Otomatis -->
      <div class="delete-impact-list">
        <label class="delete-opt-item opt-backup {backupFirstOpt ? 'is-checked' : ''}">
          <input type="checkbox" bind:checked={backupFirstOpt} disabled={deletingSite} />
          <div class="opt-body">
            <div class="opt-title-row">
              <span class="opt-title">Auto-Backup Proyek &amp; Database Sebelum Dihapus</span>
              <span class="opt-badge badge-safe">Disarankan</span>
            </div>
            <span class="opt-desc">
              Mencadangkan dump SQL + seluruh direktori proyek ke <code class="mono">/var/backups/fikpanel/{deleteTarget.domain}-pre-delete-*.tar.gz</code> agar dapat dipulihkan kapan saja lewat tab <b>Create from backup</b>.
            </span>
          </div>
        </label>

        <label class="delete-opt-item opt-destructive {deleteFilesOpt ? 'is-checked-danger' : ''}">
          <input type="checkbox" bind:checked={deleteFilesOpt} disabled={deletingSite} />
          <div class="opt-body">
            <div class="opt-title-row">
              <span class="opt-title">Hapus Direktori Proyek &amp; File Web Root</span>
              <span class="opt-badge badge-danger">Permanen</span>
            </div>
            <span class="opt-desc">
              Menghapus seluruh berkas kode sumber di <code class="mono">{getProjectRootDir(deleteTarget)}</code>
            </span>
          </div>
        </label>

        <label class="delete-opt-item opt-destructive {deleteDBOpt ? 'is-checked-danger' : ''}">
          <input type="checkbox" bind:checked={deleteDBOpt} disabled={deletingSite} />
          <div class="opt-body">
            <div class="opt-title-row">
              <span class="opt-title">Hapus Database &amp; User MySQL Terkait</span>
              <span class="opt-badge badge-danger">DROP DATABASE</span>
            </div>
            <span class="opt-desc">
              Menghapus skema database (<code class="mono">{getPredictedDBName(deleteTarget)}</code> / sesuai <code class="mono">.env</code>) beserta user MySQL dari server fisik.
            </span>
          </div>
        </label>
      </div>

      <!-- Verifikasi Ketik Manual (Anti Copy-Paste) -->
      <div class="delete-verify-box">
        <label for="confirm-domain-input" class="verify-label">
          Untuk konfirmasi, ketik manual nama domain <strong class="mono select-none">{deleteTarget.domain}</strong> di bawah ini:
        </label>
        <input
          id="confirm-domain-input"
          type="text"
          class="verify-input mono"
          placeholder="Ketik manual {deleteTarget.domain} (tidak bisa paste)"
          bind:value={deleteConfirmInput}
          autocomplete="off"
          autocorrect="off"
          autocapitalize="off"
          spellcheck="false"
          disabled={deletingSite}
          onpaste={onBlockPaste}
          ondrop={onBlockPaste}
          oncontextmenu={(e) => e.preventDefault()}
          onkeydown={(e) => {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'v') {
              onBlockPaste(e);
            } else if (e.key === 'Enter' && deleteConfirmInput === deleteTarget?.domain) {
              onConfirmDelete();
            }
          }}
        />
        {#if deletePasteWarning}
          <div class="paste-blocked-hint">
            <i class="ri-keyboard-box-line"></i>
            <span>Demi keamanan, fitur Copy-Paste dinonaktifkan. Silakan ketik nama domain secara manual dengan keyboard.</span>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  {#snippet footer()}
    <button type="button" class="btn btn--ghost" onclick={onClose} disabled={deletingSite}>
      Batal
    </button>
    <button
      type="button"
      class="btn btn--danger"
      disabled={!deleteTarget || deleteConfirmInput !== deleteTarget.domain || deletingSite}
      onclick={onConfirmDelete}
    >
      {#if deletingSite}
        <i class="ri-loader-4-line spin"></i>
        <span>{backupFirstOpt ? 'Mencadangkan & Menghapus...' : 'Menghapus Situs...'}</span>
      {:else}
        <i class="ri-delete-bin-2-line"></i>
        <span>Saya Mengerti, Hapus Situs Ini</span>
      {/if}
    </button>
  {/snippet}
</Modal>

