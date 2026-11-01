<script lang="ts">
  // Subkomponen tab Cloud Image (Docker Hub) pada halaman Docker Management Suite
  import '../../styles/pages/docker/cloud-image.css';

  interface CloudImageResult {
    name: string;
    description: string;
    stars: number;
    is_official: boolean;
    is_automated: boolean;
  }

  interface Props {
    cloudSearchQuery: string;
    cloudSearching: boolean;
    cloudResults: CloudImageResult[];
    onSearchCloudImages: () => void;
    onOpenPullModalFor: (repoName: string) => void;
  }

  let {
    cloudSearchQuery = $bindable(),
    cloudSearching,
    cloudResults,
    onSearchCloudImages,
    onOpenPullModalFor,
  }: Props = $props();
</script>

<div class="pg-docker-cloud-image tab-pane">
  <div class="card cloud-search-card">
    <div class="card__head">
      <h2 class="card-title">
        <i class="ri-cloud-line" aria-hidden="true"></i>
        <span>Cari Citra Resmi di Docker Hub Registry</span>
      </h2>
    </div>
    <div class="card__body">
      <form
        class="search-form-row"
        onsubmit={(e) => {
          e.preventDefault();
          onSearchCloudImages();
        }}
      >
        <div class="field flex-1 mb-0">
          <input
            type="text"
            class="input"
            placeholder="Ketik nama citra (misal: nginx, redis, node, python, golang, wordpress)..."
            bind:value={cloudSearchQuery}
          />
        </div>
        <button class="btn btn--primary" type="submit" disabled={cloudSearching}>
          <i class="ri-search-2-line {cloudSearching ? 'spin' : ''}" aria-hidden="true"></i>
          <span>Cari di Hub</span>
        </button>
      </form>
    </div>
  </div>

  {#if cloudResults.length > 0}
    <div class="table-wrap mt-4">
      <table class="table">
        <thead>
          <tr>
            <th>Citra Docker Hub</th>
            <th>Deskripsi</th>
            <th class="col-w-100">Bintang</th>
            <th class="col-w-120">Lencana</th>
            <th class="col-w-120 text-right">Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#each cloudResults as r}
            <tr>
              <td>
                <div class="cell-primary font-semibold font-mono">{r.name}</div>
              </td>
              <td>
                <p class="text-xs text-secondary line-clamp-2">{r.description || 'Tanpa deskripsi'}</p>
              </td>
              <td>
                <span class="badge badge--secondary">
                  <i class="ri-star-fill text-warning" aria-hidden="true"></i>
                  {r.stars}
                </span>
              </td>
              <td>
                {#if r.is_official}
                  <span class="badge badge--success" title="Citra resmi Docker">Official</span>
                {:else if r.is_automated}
                  <span class="badge badge--info" title="Build otomatis">Automated</span>
                {:else}
                  <span class="badge badge--secondary">Community</span>
                {/if}
              </td>
              <td class="text-right">
                <button
                  class="btn btn--primary btn--sm"
                  onclick={() => onOpenPullModalFor(r.name)}
                >
                  <i class="ri-download-cloud-line" aria-hidden="true"></i>
                  <span>Tarik (Pull)</span>
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

