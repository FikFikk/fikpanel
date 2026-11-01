<script lang="ts">
  /**
   * Subkomponen Modal Pencarian Isi Berkas (Search File Content) untuk Halaman Files.
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/files/search-content-modal.css';

  interface Props {
    open: boolean;
    contentSearchQuery: string;
    contentSearchResults: { file: string; line: number; text: string }[];
    searchingContent: boolean;
    currentPath: string;
    onClose: () => void;
    onSubmit: (e: Event) => void;
  }

  let {
    open,
    contentSearchQuery = $bindable(),
    contentSearchResults,
    searchingContent,
    currentPath,
    onClose,
    onSubmit
  }: Props = $props();
</script>

<Modal
  {open}
  title="Search File Content (Cari Isi Berkas)"
  size="lg"
  onclose={onClose}
>
  <div class="pg-files-search-content-modal">
    <form onsubmit={onSubmit} class="search-content-form">
      <div class="search-content-input-wrap">
        <input
          class="input"
          type="text"
          placeholder="Masukkan kata kunci yang ingin dicari di dalam teks berkas..."
          bind:value={contentSearchQuery}
          required
        />
        <button
          type="submit"
          class="btn btn--primary"
          disabled={searchingContent}
        >
          {searchingContent ? 'Mencari...' : 'Cari'}
        </button>
      </div>
    </form>
    <div class="search-results-box">
      {#if searchingContent}
        <div class="search-loading">Memindai berkas di {currentPath}...</div>
      {:else if contentSearchResults.length === 0}
        <div class="search-empty">
          Tidak ada hasil cocok atau belum dilakukan pencarian.
        </div>
      {:else}
        <div class="results-list">
          {#each contentSearchResults as res}
            <div class="result-row">
              <div class="res-head">
                <span class="res-file font-bold mono">{res.file}</span>
                <span class="res-line mono">Baris {res.line}</span>
              </div>
              <pre class="res-code mono">{res.text}</pre>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
  {#snippet footer()}
    <button
      class="btn btn--primary"
      onclick={onClose}
    >Tutup</button>
  {/snippet}
</Modal>

