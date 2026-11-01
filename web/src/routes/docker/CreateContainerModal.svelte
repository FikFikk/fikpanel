<script lang="ts">
  // Subkomponen modal pembuatan kontainer Docker baru
  import '../../styles/pages/docker/create-container-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';

  interface NewContainerForm {
    name: string;
    image: string;
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
            bind:value={p.host}
          />
          <span class="text-muted">:</span>
          <input
            type="text"
            class="input input--sm flex-1 font-mono"
            placeholder="Port Kontainer (misal: 80)"
            bind:value={p.container}
          />
          <select class="input input--sm w-24" bind:value={p.protocol}>
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
            placeholder="Path Host (misal: /www/data/html)"
            bind:value={v.host}
          />
          <span class="text-muted">:</span>
          <input
            type="text"
            class="input input--sm flex-1 font-mono"
            placeholder="Path Kontainer (misal: /usr/share/nginx/html)"
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
            bind:value={e.key}
          />
          <span class="text-muted">=</span>
          <input
            type="text"
            class="input input--sm flex-1 font-mono"
            placeholder="VALUE (misal: production)"
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
        <select id="cnt-restart" class="input" bind:value={newContainer.restart_policy}>
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
          placeholder="misal: npm start atau redis-server --appendonly yes"
          bind:value={newContainer.cmd}
        />
      </div>
    </div>
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

