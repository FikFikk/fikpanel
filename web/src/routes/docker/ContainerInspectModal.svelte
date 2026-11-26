<script lang="ts">
  // Subkomponen modal Inspektor Mendalam Kontainer (`docker inspect`) + Recreate/Update Image
  import '../../styles/pages/docker/container-inspect-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DockerContainerInspect } from '../../lib/types';
  import { copyText } from '../../lib/utils/browser';
  import { notify } from '../../lib/stores/toast.svelte';

  interface Props {
    open: boolean;
    loading: boolean;
    actionLoading: boolean;
    inspectData: DockerContainerInspect | null;
    onClose: () => void;
    onRecreate: (id: string, pullLatest: boolean) => void;
  }

  let { open, loading, actionLoading, inspectData, onClose, onRecreate }: Props = $props();

  let subTab = $state<'summary' | 'env' | 'mounts' | 'networks' | 'labels'>('summary');
  let revealedKeys = $state<Record<string, boolean>>({});
  let envSearch = $state('');

  $effect(() => {
    if (!open) {
      revealedKeys = {};
      envSearch = '';
      subTab = 'summary';
    }
  });

  let filteredEnvs = $derived(
    (inspectData?.env || []).filter((item) => {
      if (!envSearch.trim()) return true;
      const q = envSearch.toLowerCase();
      return item.key.toLowerCase().includes(q) || (!item.secret && item.value.toLowerCase().includes(q));
    }),
  );

  function toggleReveal(key: string) {
    revealedKeys = { ...revealedKeys, [key]: !revealedKeys[key] };
  }

  async function copyEnv(key: string, value: string) {
    try {
      await copyText(`${key}=${value}`);
      notify(`Disalin: ${key}`, 'info');
    } catch {
      notify('Gagal menyalin ke clipboard', 'error');
    }
  }
</script>

<Modal {open} size="xl" onclose={onClose}>
  {#snippet header()}
    <div class="flex-row items-center gap-2 flex-1">
      <i class="ri-search-eye-line text-accent" aria-hidden="true"></i>
      <h2 class="modal__title">Inspeksi Kontainer: {inspectData?.name || '...'}</h2>
      {#if inspectData?.compose_project}
        <span class="badge badge--info text-2xs">Stack: {inspectData.compose_project}</span>
      {/if}
    </div>
  {/snippet}

  <div class="pg-docker-inspect-modal">
    {#if loading}
      <div class="text-center py-8">
        <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
        <p class="text-muted mt-2">Membaca metadata docker inspect...</p>
      </div>
    {:else if inspectData}
      <div class="inspect-tabs">
        <button
          type="button"
          class="inspect-tab {subTab === 'summary' ? 'inspect-tab--active' : ''}"
          onclick={() => (subTab = 'summary')}
        >
          Ringkasan
        </button>
        <button
          type="button"
          class="inspect-tab {subTab === 'env' ? 'inspect-tab--active' : ''}"
          onclick={() => (subTab = 'env')}
        >
          Environment ({inspectData.env?.length || 0})
        </button>
        <button
          type="button"
          class="inspect-tab {subTab === 'mounts' ? 'inspect-tab--active' : ''}"
          onclick={() => (subTab = 'mounts')}
        >
          Mounts ({inspectData.mounts?.length || 0})
        </button>
        <button
          type="button"
          class="inspect-tab {subTab === 'networks' ? 'inspect-tab--active' : ''}"
          onclick={() => (subTab = 'networks')}
        >
          Networks & Ports ({inspectData.networks?.length || 0})
        </button>
        <button
          type="button"
          class="inspect-tab {subTab === 'labels' ? 'inspect-tab--active' : ''}"
          onclick={() => (subTab = 'labels')}
        >
          Labels ({Object.keys(inspectData.labels || {}).length})
        </button>
      </div>

      {#if subTab === 'summary'}
        <div class="inspect-grid">
          <div class="inspect-kv">
            <span class="inspect-k">Container ID</span>
            <span class="inspect-v font-mono">{inspectData.id.slice(0, 18)}</span>
          </div>
          <div class="inspect-kv">
            <span class="inspect-k">Status & PID</span>
            <span class="inspect-v">
              {inspectData.state} {inspectData.pid ? `(PID ${inspectData.pid})` : ''}
              {inspectData.health_status ? `· Health: ${inspectData.health_status}` : ''}
            </span>
          </div>
          <div class="inspect-kv">
            <span class="inspect-k">Citra (Image)</span>
            <span class="inspect-v font-mono">{inspectData.image}</span>
          </div>
          <div class="inspect-kv">
            <span class="inspect-k">Restart Policy / Count</span>
            <span class="inspect-v font-mono">{inspectData.restart_policy || 'no'} ({inspectData.restart_count}x)</span>
          </div>
          <div class="inspect-kv">
            <span class="inspect-k">Hostname & Working Dir</span>
            <span class="inspect-v font-mono">{inspectData.hostname || '-'} · {inspectData.working_dir || '/'}</span>
          </div>
          <div class="inspect-kv">
            <span class="inspect-k">Storage Driver / Platform</span>
            <span class="inspect-v font-mono">{inspectData.driver || '-'} · {inspectData.platform || 'linux'}</span>
          </div>
          <div class="inspect-kv">
            <span class="inspect-k">Entrypoint</span>
            <span class="inspect-v font-mono">{(inspectData.entrypoint || []).join(' ') || '-'}</span>
          </div>
          <div class="inspect-kv">
            <span class="inspect-k">Command</span>
            <span class="inspect-v font-mono">{(inspectData.cmd || []).join(' ') || '-'}</span>
          </div>
        </div>

        <div class="inspect-recreate-box">
          <div>
            <strong class="text-sm">Perbarui atau Buat Ulang Kontainer (Recreate)</strong>
            <p class="text-xs text-muted">
              Menarik versi terbaru citra <code class="font-mono">{inspectData.image}</code> lalu membuat ulang kontainer dengan seluruh port, volume, jaringan, dan env yang identik.
            </p>
          </div>
          <div class="flex-row gap-2">
            <button
              type="button"
              class="btn btn--secondary btn--sm"
              disabled={actionLoading}
              onclick={() => onRecreate(inspectData.id, false)}
            >
              <i class="ri-restart-line" aria-hidden="true"></i>
              <span>Recreate Saja</span>
            </button>
            <button
              type="button"
              class="btn btn--primary btn--sm"
              disabled={actionLoading}
              onclick={() => onRecreate(inspectData.id, true)}
            >
              <i class="ri-download-cloud-2-line" aria-hidden="true"></i>
              <span>Pull & Update</span>
            </button>
          </div>
        </div>
      {:else if subTab === 'env'}
        <div class="flex-row gap-2 items-center">
          <input
            type="text"
            class="input input--sm flex-1"
            placeholder="Filter nama variabel lingkungan..."
            aria-label="Filter environment variable"
            bind:value={envSearch}
          />
        </div>
        <div class="inspect-scroll">
          <table class="table">
            <thead>
              <tr>
                <th>Key</th>
                <th>Value</th>
              </tr>
            </thead>
            <tbody>
              {#each filteredEnvs as item}
                <tr>
                  <td class="font-mono text-xs font-semibold">{item.key}</td>
                  <td class="font-mono text-xs">
                    <div class="env-val-cell">
                      <span>
                        {#if item.secret && !revealedKeys[item.key]}
                          ••••••••••••
                        {:else}
                          {item.value}
                        {/if}
                      </span>
                      <div class="action-btns-row">
                        {#if item.secret}
                          <button
                            type="button"
                            class="btn btn--ghost btn--sm btn--icon"
                            title={revealedKeys[item.key] ? 'Sembunyikan' : 'Tampilkan rahasia'}
                            aria-label={revealedKeys[item.key] ? 'Sembunyikan rahasia' : 'Tampilkan rahasia'}
                            onclick={() => toggleReveal(item.key)}
                          >
                            <i class={revealedKeys[item.key] ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
                          </button>
                        {/if}
                        <button
                          type="button"
                          class="btn btn--ghost btn--sm btn--icon"
                          title="Salin KEY=VALUE"
                          aria-label="Salin environment variable"
                          onclick={() => copyEnv(item.key, item.value)}
                        >
                          <i class="ri-file-copy-line" aria-hidden="true"></i>
                        </button>
                      </div>
                    </div>
                  </td>
                </tr>
              {:else}
                <tr>
                  <td colspan="2" class="text-center py-6 text-muted">Tidak ada variabel lingkungan ditemukan.</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else if subTab === 'mounts'}
        <div class="inspect-scroll">
          <table class="table">
            <thead>
              <tr>
                <th>Tipe</th>
                <th>Sumber (Host / Volume)</th>
                <th>Tujuan Kontainer</th>
                <th>Akses</th>
              </tr>
            </thead>
            <tbody>
              {#each inspectData.mounts || [] as mt}
                <tr>
                  <td><span class="badge badge--secondary font-mono">{mt.type}</span></td>
                  <td class="font-mono text-xs">{mt.name || mt.source}</td>
                  <td class="font-mono text-xs">{mt.destination}</td>
                  <td>
                    <span class="badge {mt.rw ? 'badge--success' : 'badge--warning'}">
                      {mt.rw ? 'RW' : 'RO'}
                    </span>
                  </td>
                </tr>
              {:else}
                <tr>
                  <td colspan="4" class="text-center py-6 text-muted">Kontainer ini tidak memiliki volume atau bind mount.</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else if subTab === 'networks'}
        <div class="inspect-scroll">
          <table class="table">
            <thead>
              <tr>
                <th>Jaringan</th>
                <th>IP Address</th>
                <th>Gateway</th>
                <th>MAC Address</th>
              </tr>
            </thead>
            <tbody>
              {#each inspectData.networks || [] as net}
                <tr>
                  <td class="font-mono text-xs font-semibold">{net.name}</td>
                  <td class="font-mono text-xs">{net.ip_address || '-'}</td>
                  <td class="font-mono text-xs">{net.gateway || '-'}</td>
                  <td class="font-mono text-xs text-muted">{net.mac_address || '-'}</td>
                </tr>
              {:else}
                <tr>
                  <td colspan="4" class="text-center py-6 text-muted">Tidak terhubung ke jaringan apa pun.</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else if subTab === 'labels'}
        <div class="inspect-scroll">
          <table class="table">
            <thead>
              <tr>
                <th>Label Key</th>
                <th>Value</th>
              </tr>
            </thead>
            <tbody>
              {#each Object.entries(inspectData.labels || {}) as [k, v]}
                <tr>
                  <td class="font-mono text-xs font-semibold">{k}</td>
                  <td class="font-mono text-xs">{v}</td>
                </tr>
              {:else}
                <tr>
                  <td colspan="2" class="text-center py-6 text-muted">Tidak ada label pada kontainer ini.</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    {/if}
  </div>

  {#snippet footer()}
    <button type="button" class="btn btn--secondary btn--sm" onclick={onClose}>
      Tutup
    </button>
  {/snippet}
</Modal>
