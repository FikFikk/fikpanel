<script lang="ts">
  // Subkomponen modal pembuatan kontainer Docker baru dengan dukungan jaringan, batas RAM/CPU, dan auto-remove
  import '../../styles/pages/docker/create-container-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { DockerNetwork } from '../../lib/types';

  interface NewContainerForm {
    name: string;
    image: string;
    network: string;
    memory_limit_mb: number;
    cpu_limit: number;
    ports: Array<{ host: string; container: string; protocol: string }>;
    volumes: Array<{ host: string; container: string }>;
    envs: Array<{ key: string; value: string }>;
    restart_policy: string;
    auto_remove: boolean;
    cmd: string;
  }

  interface Props {
    showCreateContainerModal: boolean;
    newContainer: NewContainerForm;
    networks: DockerNetwork[];
    actionLoading: boolean;
    onSubmitCreateContainer: () => void;
    onAddPortMapping: () => void;
    onRemovePortMapping: (idx: number) => void;
    onAddVolumeMapping: () => void;
    onRemoveVolumeMapping: (idx: number) => void;
    onAddEnvParam: () => void;
    onRemoveEnvParam: (idx: number) => void;
  }

  let {
    showCreateContainerModal = $bindable(),
    newContainer = $bindable(),
    networks,
    actionLoading,
    onSubmitCreateContainer,
    onAddPortMapping,
    onRemovePortMapping,
    onAddVolumeMapping,
    onRemoveVolumeMapping,
    onAddEnvParam,
    onRemoveEnvParam,
  }: Props = $props();
</script>

<Modal
  open={showCreateContainerModal}
  title="Buat Kontainer Baru"
  size="lg"
  onclose={() => (showCreateContainerModal = false)}
>
  <form
    id="docker-create-cnt-form"
    class="pg-docker-create-container-modal docker-modal-form"
    onsubmit={(e) => {
      e.preventDefault();
      onSubmitCreateContainer();
    }}
  >
    <div class="form-row">
      <div class="field flex-1">
        <label class="field__label" for="cnt-name">Nama Kontainer *</label>
        <input
          id="cnt-name"
          type="text"
          class="input"
          placeholder="misal: my-nginx-app"
          bind:value={newContainer.name}
          required
        />
      </div>
      <div class="field flex-1">
        <label class="field__label" for="cnt-image">Nama Citra (Image) *</label>
        <input
          id="cnt-image"
          type="text"
          class="input font-mono"
          placeholder="misal: nginx:alpine atau redis:7"
          bind:value={newContainer.image}
          required
        />
      </div>
    </div>

    <div class="form-row">
      <div class="field flex-1">
        <label class="field__label" for="cnt-net">Jaringan (Network)</label>
        <select id="cnt-net" class="input" bind:value={newContainer.network}>
          <option value="default">Default (bridge)</option>
          {#each networks as n}
            <option value={n.name}>{n.name} ({n.driver})</option>
          {/each}
        </select>
      </div>
      <div class="field w-28">
        <label class="field__label" for="cnt-ram">Batas RAM (MB)</label>
        <input
          id="cnt-ram"
          type="number"
          min="0"
          class="input font-mono"
          placeholder="0 = bebas"
          bind:value={newContainer.memory_limit_mb}
        />
      </div>
      <div class="field w-28">
        <label class="field__label" for="cnt-cpu">Batas CPU</label>
        <input
          id="cnt-cpu"
          type="number"
          min="0"
          step="0.25"
          class="input font-mono"
          placeholder="0 = bebas"
          bind:value={newContainer.cpu_limit}
        />
      </div>
    </div>

    <!-- Pemetaan port host ke kontainer -->
    <div class="form-section">
      <div class="form-section-head">
        <span class="font-semibold text-sm">Pemetaan Port (Host : Container)</span>
        <button type="button" class="btn btn--secondary btn--sm" onclick={onAddPortMapping}>
          <i class="ri-add-line" aria-hidden="true"></i>
          <span>Tambah Port</span>
        </button>
      </div>
      {#each newContainer.ports as p, idx}
        <div class="mapping-row">
          <input
            type="text"
            class="input input--sm flex-1 font-mono"
            placeholder="Port Host (misal: 8080)"
            aria-label="Port host"
            bind:value={p.host}
          />
          <span class="text-muted">:</span>
          <input
            type="text"
            class="input input--sm flex-1 font-mono"
            placeholder="Port Kontainer (misal: 80)"
            aria-label="Port kontainer"
            bind:value={p.container}
          />
          <select class="input input--sm w-24" aria-label="Protokol port" bind:value={p.protocol}>
            <option value="tcp">TCP</option>
            <option value="udp">UDP</option>
          </select>
          <button
            type="button"
            class="btn btn--ghost btn--sm btn--icon"
            title="Hapus baris port"
            aria-label="Hapus baris port"
            onclick={() => onRemovePortMapping(idx)}
          >
            <i class="ri-delete-bin-line text-danger" aria-hidden="true"></i>
          </button>
        </div>
      {/each}
    </div>

    <!-- Pemetaan volume host ke kontainer -->
    <div class="form-section">
      <div class="form-section-head">
        <span class="font-semibold text-sm">Pemetaan Volume / Direktori Host</span>
        <button type="button" class="btn btn--secondary btn--sm" onclick={onAddVolumeMapping}>
          <i class="ri-add-line" aria-hidden="true"></i>
          <span>Tambah Mount</span>
        </button>
      </div>
      {#each newContainer.volumes as v, idx}
        <div class="mapping-row">
          <input
            type="text"
            class="input input--sm flex-1 font-mono"
            placeholder="Path Host atau Nama Volume"
            aria-label="Sumber volume host"
            bind:value={v.host}
          />
          <span class="text-muted">:</span>
          <input
            type="text"
            class="input input--sm flex-1 font-mono"
            placeholder="Path Kontainer (misal: /data)"
            aria-label="Tujuan volume kontainer"
            bind:value={v.container}
          />
          <button
            type="button"
            class="btn btn--ghost btn--sm btn--icon"
            title="Hapus mount"
            aria-label="Hapus mount"
            onclick={() => onRemoveVolumeMapping(idx)}
          >
            <i class="ri-delete-bin-line text-danger" aria-hidden="true"></i>
          </button>
        </div>
      {/each}
    </div>

    <!-- Variabel lingkungan kontainer -->
    <div class="form-section">
      <div class="form-section-head">
        <span class="font-semibold text-sm">Environment Variables</span>
        <button type="button" class="btn btn--secondary btn--sm" onclick={onAddEnvParam}>
          <i class="ri-add-line" aria-hidden="true"></i>
          <span>Tambah Env</span>
        </button>
      </div>
      {#each newContainer.envs as e, idx}
        <div class="mapping-row">
          <input
            type="text"
            class="input input--sm flex-1 font-mono"
            placeholder="KEY (misal: NODE_ENV)"
            aria-label="Nama variabel env"
            bind:value={e.key}
          />
          <span class="text-muted">=</span>
          <input
            type="text"
            class="input input--sm flex-1 font-mono"
            placeholder="VALUE (misal: production)"
            aria-label="Nilai variabel env"
            bind:value={e.value}
          />
          <button
            type="button"
            class="btn btn--ghost btn--sm btn--icon"
            title="Hapus env"
            aria-label="Hapus env"
            onclick={() => onRemoveEnvParam(idx)}
          >
            <i class="ri-delete-bin-line text-danger" aria-hidden="true"></i>
          </button>
        </div>
      {/each}
    </div>

    <!-- Kebijakan restart dan custom command -->
    <div class="form-row">
      <div class="field flex-1">
        <label class="field__label" for="cnt-restart">Kebijakan Restart</label>
        <select id="cnt-restart" class="input" disabled={newContainer.auto_remove} bind:value={newContainer.restart_policy}>
          <option value="unless-stopped">Unless Stopped (Direkomendasikan)</option>
          <option value="always">Always Restart</option>
          <option value="on-failure">On Failure Only</option>
          <option value="no">Do Not Restart (No)</option>
        </select>
      </div>
      <div class="field flex-1">
        <label class="field__label" for="cnt-cmd">Custom Command (Opsional)</label>
        <input
          id="cnt-cmd"
          type="text"
          class="input font-mono"
          placeholder="misal: redis-server --appendonly yes"
          bind:value={newContainer.cmd}
        />
      </div>
    </div>

    <label class="flex-row items-center gap-2 text-xs text-secondary">
      <input type="checkbox" bind:checked={newContainer.auto_remove} />
      <span>Hapus kontainer otomatis saat berhenti (<code>--rm</code>)</span>
    </label>
  </form>

  {#snippet footer()}
    <button
      type="button"
      class="btn btn--secondary"
      onclick={() => (showCreateContainerModal = false)}
    >
      Batal
    </button>
    <button
      type="submit"
      form="docker-create-cnt-form"
      class="btn btn--primary"
      disabled={actionLoading}
    >
      <i class="ri-check-line" aria-hidden="true"></i>
      <span>Buat & Jalankan Kontainer</span>
    </button>
  {/snippet}
</Modal>
