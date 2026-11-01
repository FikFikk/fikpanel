<script lang="ts">
  /**
   * Subkomponen Bar Status & Paginasi untuk Halaman Files.
   */
  import '../../styles/pages/files/status-bar.css';

  interface Props {
    totalDirs: number;
    totalFiles: number;
    totalSize: number;
    currentPage: number;
    pageSize: number;
    totalPages: number;
    totalFilteredCount: number;
    formatBytes: (bytes: number) => string;
  }

  let {
    totalDirs,
    totalFiles,
    totalSize,
    currentPage = $bindable(),
    pageSize = $bindable(),
    totalPages,
    totalFilteredCount,
    formatBytes
  }: Props = $props();
</script>

<div class="pg-files-status-bar">
  <div class="foot-left">
    <span>Total <strong>{totalDirs}</strong> directories, <strong>{totalFiles}</strong> files, File size: <strong class="mono">{formatBytes(totalSize)}</strong></span>
  </div>

  <div class="foot-right">
    <div class="pager">
      <button
        class="btn-page"
        disabled={currentPage <= 1}
        onclick={() => currentPage--}
      >&lt;</button>
      <span class="page-num">{currentPage}</span>
      <button
        class="btn-page"
        disabled={currentPage >= totalPages}
        onclick={() => currentPage++}
      >&gt;</button>
    </div>

    <select class="select-pagesize" bind:value={pageSize}>
      <option value={50}>50 / page</option>
      <option value={100}>100 / page</option>
      <option value={200}>200 / page</option>
    </select>

    <span class="goto-box">Goto <input type="number" min="1" max={totalPages} bind:value={currentPage} /></span>
    <span class="total-badge">Total {totalFilteredCount}</span>
  </div>
</div>

