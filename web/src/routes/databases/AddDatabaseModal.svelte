<script lang="ts">
  /**
   * Subkomponen Modal Tambah Database.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DatabaseEngineInfo } from '../../lib/types';
  import '../../styles/pages/databases/add-database-modal.css';

  interface Props {
    open: boolean;
    creatingDb: boolean;
    currentEngine: DatabaseEngineInfo;
    dbName: string;
    dbCharset: string;
    dbUser: string;
    dbPass: string;
    dbPermission: string;
    customIp: string;
    dbNote: string;
    onDbNameInput: (val: string) => void;
    onDbUserInput: (val: string) => void;
    onGeneratePassword: () => void;
    onClose: () => void;
    onSubmit: () => void;
  }

  let {
    open,
    creatingDb,
    currentEngine,
    dbName,
    dbCharset = $bindable(),
    dbUser,
    dbPass = $bindable(),
    dbPermission = $bindable(),
    customIp = $bindable(),
    dbNote = $bindable(),
    onDbNameInput,
    onDbUserInput,
    onGeneratePassword,
    onClose,
    onSubmit
  }: Props = $props();
</script>

<Modal
  {open}
  title="Tambah Database {currentEngine.display_name}"
  size="md"
  onclose={() => { if (!creatingDb) onClose(); }}
>
  <div class="pg-databases-add-database-modal">
    <!-- DB Name & Charset (Screenshot 2 aaPanel) -->
    <div class="form-group">
      <label for="db-name" class="form-label">Nama Database & Charset</label>
      <div class="input-with-select">
        <input
          id="db-name"
          type="text"
          class="form-input flex-1"
          placeholder="Nama database baru"
          value={dbName}
          oninput={(e) => onDbNameInput((e.target as HTMLInputElement).value)}
          disabled={creatingDb}
        />
        <select class="form-select charset-select" bind:value={dbCharset} disabled={creatingDb}>
          <option value="utf8mb4">utf8mb4 (Rekomendasi)</option>
          <option value="utf8">utf8</option>
          <option value="latin1">latin1</option>
          <option value="gbk">gbk</option>
        </select>
      </div>
    </div>

    <!-- Username -->
    <div class="form-group">
      <label for="db-user" class="form-label">Username Pengguna</label>
      <input
        id="db-user"
        type="text"
        class="form-input"
        placeholder="Username database"
        value={dbUser}
        oninput={(e) => onDbUserInput((e.target as HTMLInputElement).value)}
        disabled={creatingDb}
      />
    </div>

    <!-- Password dengan tombol Refresh acak di samping input -->
    <div class="form-group">
      <label for="db-pass" class="form-label">Password</label>
      <div class="input-with-button">
        <input
          id="db-pass"
          type="text"
          class="form-input"
          placeholder="Kata sandi akses"
          bind:value={dbPass}
          disabled={creatingDb}
        />
        <button
          type="button"
          class="btn btn-outline btn-refresh-pass"
          onclick={onGeneratePassword}
          disabled={creatingDb}
          title="Acak ulang password otomatis"
        >
          <i class="ri-refresh-line" aria-hidden="true"></i>
        </button>
      </div>
    </div>

    <!-- Hak Akses (Permission) -->
    <div class="form-group">
      <label for="db-perm" class="form-label">Hak Akses (Permission)</label>
      <select id="db-perm" class="form-select" bind:value={dbPermission} disabled={creatingDb}>
        <option value="localhost">Local server (127.0.0.1)</option>
        <option value="%">Semua Host (Public / %)</option>
        <option value="custom">IP Tertentu (Spesifik)</option>
      </select>
      {#if dbPermission === 'custom'}
        <input
          type="text"
          class="form-input mt-2"
          placeholder="Masukkan alamat IP (mis. 192.168.1.100)"
          bind:value={customIp}
          disabled={creatingDb}
        />
      {/if}
    </div>

    <!-- Add to Server Lokasi (LocalServer) -->
    <div class="form-group">
      <div class="add-to-header">
        <label for="add-to-select" class="form-label">Tambahkan Ke (Add to)</label>
        <span class="remote-servers-link">Remote servers</span>
      </div>
      <select id="add-to-select" class="form-select" disabled>
        <option value="local">LocalServer (127.0.0.1)</option>
      </select>
    </div>

    <div class="form-group">
      <label for="db-note" class="form-label">Catatan (Opsional)</label>
      <input
        id="db-note"
        type="text"
        class="form-input"
        placeholder="Keterangan singkat database ini"
        bind:value={dbNote}
        disabled={creatingDb}
      />
    </div>
  </div>

  {#snippet footer()}
    <button class="btn btn-outline" disabled={creatingDb} onclick={onClose}>
      Batal
    </button>
    <button class="btn btn-primary" disabled={creatingDb} onclick={onSubmit}>
      {#if creatingDb}
        <i class="ri-loader-4-line spinner" aria-hidden="true"></i>
        <span>Menyimpan...</span>
      {:else}
        <span>Konfirmasi Buat DB</span>
      {/if}
    </button>
  {/snippet}
</Modal>

