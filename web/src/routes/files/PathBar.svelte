<script lang="ts">
  // Subkomponen bar navigasi path breadcrumbs dan pencarian berkas instan pada halaman File Manager
  import '../../styles/pages/files/path-bar.css';

  export interface BreadcrumbItem {
    label: string;
    fullPath: string;
  }

  interface Props {
    currentPath: string;
    parentPath: string;
    breadcrumbs: BreadcrumbItem[];
    searchQuery: string;
    onGoParent: () => void;
    onLoadDirectory: (path: string) => void;
  }

  let {
    currentPath,
    parentPath,
    breadcrumbs,
    searchQuery = $bindable(),
    onGoParent,
    onLoadDirectory,
  }: Props = $props();
</script>

<div class="pg-files-path-bar nav-bar">
  <div class="nav-left">
    <!-- Tombol kembali ke folder induk -->
    <button
      class="btn-icon"
      onclick={onGoParent}
      disabled={!parentPath}
      title="Kembali ke folder induk"
    >
      <i class="ri-arrow-left-line"></i>
    </button>

    <!-- Jalur breadcrumbs -->
    <div class="breadcrumb-trail">
      <i class="ri-folder-3-fill path-icon"></i>
      <button class="crumb-btn" onclick={() => onLoadDirectory('')}>root</button>
      {#each breadcrumbs as crumb}
        <span class="crumb-sep">&gt;</span>
        <button class="crumb-btn" onclick={() => onLoadDirectory(crumb.fullPath)}>
          {crumb.label}
        </button>
      {/each}
    </div>

    <!-- Tombol segarkan direktori -->
    <button class="btn-icon" onclick={() => onLoadDirectory(currentPath)} title="Refresh">
      <i class="ri-refresh-line"></i>
    </button>
  </div>

  <!-- Pencarian berkas instan -->
  <div class="nav-right">
    <div class="search-box">
      <i class="ri-search-line search-icon"></i>
      <input
        type="text"
        placeholder="Cari berkas..."
        bind:value={searchQuery}
      />
      {#if searchQuery}
        <button
          class="btn-clear-search"
          onclick={() => (searchQuery = '')}
          title="Reset pencarian"
        >
          <i class="ri-close-line"></i>
        </button>
      {/if}
    </div>
  </div>
</div>

