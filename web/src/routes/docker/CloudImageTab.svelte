<script lang="ts">
  // Subkomponen tab Docker Hub Explorer (API v2 + Tag Picker arm64/amd64) & Dockerfile Quick Builder
  import '../../styles/pages/docker/cloud-image.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import CodeMirrorEditor from '../../lib/components/ui/CodeMirrorEditor.svelte';
  import { apiFetch } from '../../lib/api';
  import { notify } from '../../lib/stores/toast.svelte';
  import type { DockerHubSearchItem, DockerHubTagItem } from '../../lib/types';

  interface Props {
    hubQuery: string;
    hubResults: DockerHubSearchItem[];
    searchingHub: boolean;
    pulling: boolean;
    initialMode?: 'hub' | 'build';
    onSearchDockerHub: () => void;
    onStreamPullImage: (fullRef: string) => void;
    onBuildDockerfile: (payload: { tag: string; dockerfile: string; no_cache: boolean; pull: boolean }) => void;
  }

  let {
    hubQuery = $bindable(),
    hubResults,
    searchingHub,
    pulling,
    initialMode = 'hub',
    onSearchDockerHub,
    onStreamPullImage,
    onBuildDockerfile,
  }: Props = $props();

  let subMode = $state<'hub' | 'build'>('hub');
  $effect(() => {
    subMode = initialMode;
  });

  // Populer Docker Hub shortcuts
  const popularRepos = [
    'nginx',
    'postgres',
    'redis',
    'node',
    'golang',
    'python',
    'mariadb',
    'n8nio/n8n',
    'minio/minio',
    'caddy',
    'traefik',
  ];

  // State Tag Explorer Modal
  let tagModalOpen = $state(false);
  let tagModalRepo = $state('');
  let tagFilter = $state('');
  let tagsList = $state<DockerHubTagItem[]>([]);
  let loadingTags = $state(false);

  // State Dockerfile Quick Builder
  let buildTag = $state('my-custom-app:latest');
  let buildNoCache = $state(false);
  let buildPullBase = $state(true);
  let dockerfileContent = $state(`FROM nginx:alpine
LABEL maintainer="fikpanel"
RUN echo "<h1>Dibangun dari Docker Studio FikPanel</h1>" > /usr/share/nginx/html/index.html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`);

  const dockerfilePresets: Record<string, { label: string; tag: string; content: string }> = {
    nginx: {
      label: 'Nginx Static',
      tag: 'custom-nginx:latest',
      content: `FROM nginx:alpine
RUN echo "<h1>Hello from FikPanel Docker Studio</h1>" > /usr/share/nginx/html/index.html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`,
    },
    node: {
      label: 'Node.js 22 Alpine',
      tag: 'custom-node:latest',
      content: `FROM node:22-alpine
WORKDIR /app
RUN printf 'const http = require("http");\\nhttp.createServer((_, res) => res.end("Node.js OK")).listen(3000);\\n' > server.js
EXPOSE 3000
CMD ["node", "server.js"]
`,
    },
    python: {
      label: 'Python 3.12 HTTP',
      tag: 'custom-python:latest',
      content: `FROM python:3.12-alpine
WORKDIR /app
RUN echo "Hello from Python Container" > index.html
EXPOSE 8000
CMD ["python3", "-m", "http.server", "8000"]
`,
    },
    golang: {
      label: 'Go Multi-Stage',
      tag: 'custom-go:latest',
      content: `FROM golang:1.23-alpine AS builder
WORKDIR /src
RUN printf 'package main\\nimport ("fmt"; "net/http")\\nfunc main(){http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){fmt.Fprintln(w, "Go OK")}); http.ListenAndServe(":8080", nil)}\\n' > main.go
RUN CGO_ENABLED=0 go build -o /app/server main.go

FROM alpine:3.20
COPY --from=builder /app/server /usr/local/bin/server
EXPOSE 8080
CMD ["server"]
`,
    },
  };

  function formatPulls(n: number): string {
    if (!n || n <= 0) return '-';
    if (n >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(1)}B+`;
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M+`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K+`;
    return String(n);
  }

  function quickSearch(term: string) {
    hubQuery = term;
    onSearchDockerHub();
  }

  async function openTagExplorer(repo: string) {
    tagModalRepo = repo;
    tagFilter = '';
    tagModalOpen = true;
    await fetchRepoTags();
  }

  async function fetchRepoTags() {
    if (!tagModalRepo) return;
    loadingTags = true;
    try {
      const qs = new URLSearchParams({ repo: tagModalRepo });
      if (tagFilter.trim()) qs.set('q', tagFilter.trim());
      const res = await apiFetch<{ tags: DockerHubTagItem[] }>(`/api/docker/hub/tags?${qs.toString()}`);
      tagsList = res.tags || [];
    } catch (e: any) {
      notify(`Gagal memuat tag ${tagModalRepo}: ${e.message}`, 'error');
      tagsList = [];
    } finally {
      loadingTags = false;
    }
  }

  function pullSelectedTag(tagName: string) {
    tagModalOpen = false;
    onStreamPullImage(`${tagModalRepo}:${tagName}`);
  }

  function submitBuild(e: Event) {
    e.preventDefault();
    if (!buildTag.trim() || !dockerfileContent.trim()) {
      notify('Nama tag dan isi Dockerfile wajib diisi', 'warning');
      return;
    }
    onBuildDockerfile({
      tag: buildTag.trim(),
      dockerfile: dockerfileContent,
      no_cache: buildNoCache,
      pull: buildPullBase,
    });
  }
</script>

<div class="pg-docker-cloud-image tab-pane">
  <div class="mode-switcher">
    <div class="filter-pills">
      <button
        type="button"
        class="pill {subMode === 'hub' ? 'pill--active' : ''}"
        onclick={() => (subMode = 'hub')}
      >
        <i class="ri-global-line" aria-hidden="true"></i>
        Docker Hub Explorer & Tag Picker
      </button>
      <button
        type="button"
        class="pill {subMode === 'build' ? 'pill--active' : ''}"
        onclick={() => (subMode = 'build')}
      >
        <i class="ri-hammer-line" aria-hidden="true"></i>
        Dockerfile Quick Builder
      </button>
    </div>
  </div>

  {#if subMode === 'hub'}
    <div class="card">
      <div class="card__body">
        <form
          class="search-form-row"
          onsubmit={(e) => {
            e.preventDefault();
            onSearchDockerHub();
          }}
        >
          <input
            type="text"
            class="input flex-1"
            placeholder="Cari citra di Docker Hub (contoh: nginx, postgres, n8nio/n8n, minio/minio)..."
            aria-label="Cari citra Docker Hub"
            bind:value={hubQuery}
          />
          <button type="submit" class="btn btn--primary" disabled={searchingHub}>
            <i class="ri-search-line {searchingHub ? 'spin' : ''}" aria-hidden="true"></i>
            <span>{searchingHub ? 'Mencari...' : 'Cari Docker Hub'}</span>
          </button>
        </form>

        <div class="popular-chips">
          <span class="text-xs text-muted">Populer:</span>
          {#each popularRepos as pop}
            <button type="button" class="chip-btn" onclick={() => quickSearch(pop)}>
              {pop}
            </button>
          {/each}
        </div>
      </div>
    </div>

    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>Repositori</th>
            <th>Deskripsi</th>
            <th class="col-w-140">Popularitas</th>
            <th class="col-w-200 text-right">Unduh / Tag</th>
          </tr>
        </thead>
        <tbody>
          {#if searchingHub}
            <tr>
              <td colspan="4" class="text-center py-6">
                <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
                <p class="text-muted mt-2">Menghubungi Docker Hub Registry API v2...</p>
              </td>
            </tr>
          {:else if hubResults.length === 0}
            <tr>
              <td colspan="4" class="text-center py-8">
                <div class="empty-state">
                  <i class="ri-download-cloud-2-line empty-icon" aria-hidden="true"></i>
                  <p class="empty-title">Jelajahi Jutaan Citra Docker Hub</p>
                  <p class="empty-desc">Klik salah satu shortcut populer di atas atau ketik nama citra untuk memilih tag & arsitektur (arm64 / amd64).</p>
                </div>
              </td>
            </tr>
          {:else}
            {#each hubResults as res}
              <tr>
                <td>
                  <div class="hub-repo-head">
                    <strong class="font-mono text-sm">{res.name}</strong>
                    {#if res.is_official}
                      <span class="badge badge--success text-2xs">Official</span>
                    {/if}
                  </div>
                </td>
                <td>
                  <span class="text-xs text-muted">{res.description || '-'}</span>
                </td>
                <td>
                  <div class="hub-metrics">
                    <span title="Bintang Docker Hub">
                      <i class="ri-star-fill text-warning" aria-hidden="true"></i>
                      {res.star_count.toLocaleString()}
                    </span>
                    {#if res.pull_count > 0}
                      <span title="Total Pull">
                        <i class="ri-download-2-line text-info" aria-hidden="true"></i>
                        {formatPulls(res.pull_count)}
                      </span>
                    {/if}
                  </div>
                </td>
                <td class="text-right">
                  <div class="action-btns-row">
                    <button
                      type="button"
                      class="btn btn--secondary btn--sm"
                      onclick={() => openTagExplorer(res.name)}
                    >
                      <i class="ri-price-tag-3-line" aria-hidden="true"></i>
                      <span>Pilih Tag</span>
                    </button>
                    <button
                      type="button"
                      class="btn btn--primary btn--sm"
                      disabled={pulling}
                      onclick={() => onStreamPullImage(`${res.name}:latest`)}
                    >
                      <i class="ri-download-cloud-line" aria-hidden="true"></i>
                      <span>Pull</span>
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  {:else}
    <!-- Sub-mode Dockerfile Quick Builder -->
    <form class="card" onsubmit={submitBuild}>
      <div class="card__head">
        <div>
          <h2 class="card-title">
            <i class="ri-hammer-line text-accent" aria-hidden="true"></i>
            <span>Dockerfile Quick Builder</span>
          </h2>
          <span class="text-xs text-muted">Tulis Dockerfile lalu bangun citra lokal secara instan dengan streaming output build real-time.</span>
        </div>
        <div class="preset-row">
          {#each Object.values(dockerfilePresets) as preset}
            <button
              type="button"
              class="chip-btn"
              onclick={() => {
                buildTag = preset.tag;
                dockerfileContent = preset.content;
              }}
            >
              {preset.label}
            </button>
          {/each}
        </div>
      </div>

      <div class="card__body docker-modal-form">
        <div class="form-row items-center">
          <div class="field flex-1">
            <label class="field__label" for="build-image-tag">Nama Citra & Tag Target *</label>
            <input
              id="build-image-tag"
              type="text"
              class="input font-mono"
              placeholder="misal: my-api:1.0 atau custom-nginx:latest"
              bind:value={buildTag}
              required
            />
          </div>
          <div class="builder-options">
            <label class="builder-check">
              <input type="checkbox" bind:checked={buildPullBase} />
              <span>Selalu tarik base image terbaru (--pull)</span>
            </label>
            <label class="builder-check">
              <input type="checkbox" bind:checked={buildNoCache} />
              <span>Tanpa Cache (--no-cache)</span>
            </label>
          </div>
        </div>

        <div class="field">
          <span class="field__label">Isi Dockerfile *</span>
          <CodeMirrorEditor
            bind:value={dockerfileContent}
            lang="shell"
            minHeight="280px"
            maxHeight="52vh"
          />
        </div>

        <div class="flex-row justify-between items-center">
          <span class="text-xs text-muted">Build berjalan terisolasi di direktori temporer aman dengan progres live di panel konsol atas.</span>
          <button type="submit" class="btn btn--primary" disabled={pulling}>
            <i class="ri-play-fill" aria-hidden="true"></i>
            <span>{pulling ? 'Sedang Memproses...' : 'Build Citra Sekarang'}</span>
          </button>
        </div>
      </div>
    </form>
  {/if}

  <!-- Modal Tag Explorer Docker Hub (menampilkan ukuran & dukungan arm64/amd64) -->
  <Modal open={tagModalOpen} size="lg" onclose={() => (tagModalOpen = false)}>
    {#snippet header()}
      <div class="flex-row items-center gap-2 flex-1">
        <i class="ri-price-tag-3-line text-accent" aria-hidden="true"></i>
        <h2 class="modal__title">Pilih Tag: {tagModalRepo}</h2>
      </div>
    {/snippet}

    <div class="docker-modal-form">
      <form
        class="search-form-row"
        onsubmit={(e) => {
          e.preventDefault();
          fetchRepoTags();
        }}
      >
        <input
          type="text"
          class="input input--sm flex-1 font-mono"
          placeholder="Filter tag (misal: alpine, 16, bookworm, lts)..."
          aria-label="Filter tag Docker Hub"
          bind:value={tagFilter}
        />
        <button type="submit" class="btn btn--secondary btn--sm" disabled={loadingTags}>
          <i class="ri-search-line" aria-hidden="true"></i>
          <span>Filter</span>
        </button>
      </form>

      <div class="tags-list-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>Tag Versi</th>
              <th>Arsitektur</th>
              <th>Ukuran</th>
              <th class="col-w-120 text-right">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#if loadingTags}
              <tr>
                <td colspan="4" class="text-center py-6">
                  <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
                  <p class="text-muted mt-2">Mengambil daftar tag dari Docker Hub...</p>
                </td>
              </tr>
            {:else if tagsList.length === 0}
              <tr>
                <td colspan="4" class="text-center py-6 text-muted">Tidak ada tag ditemukan untuk filter tersebut.</td>
              </tr>
            {:else}
              {#each tagsList as t}
                <tr>
                  <td>
                    <strong class="font-mono text-xs">{t.name}</strong>
                  </td>
                  <td>
                    <div class="arch-chips">
                      {#each t.architectures || [] as arch}
                        <span class="badge {arch === 'arm64' ? 'badge--info' : 'badge--secondary'} text-2xs font-mono">
                          {arch}
                        </span>
                      {/each}
                    </div>
                  </td>
                  <td>
                    <span class="font-mono text-xs">{t.size_human || '-'}</span>
                  </td>
                  <td class="text-right">
                    <button
                      type="button"
                      class="btn btn--primary btn--sm"
                      disabled={pulling}
                      onclick={() => pullSelectedTag(t.name)}
                    >
                      <i class="ri-download-line" aria-hidden="true"></i>
                      <span>Pull</span>
                    </button>
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary btn--sm" onclick={() => (tagModalOpen = false)}>
        Tutup
      </button>
    {/snippet}
  </Modal>
</div>
