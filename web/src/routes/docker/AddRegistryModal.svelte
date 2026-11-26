<script lang="ts">
  // Subkomponen modal penambahan & login kredensial registri privat Docker dengan preset GHCR/GitLab/Docker Hub
  import '../../styles/pages/docker/add-registry-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';

  interface NewRegistryForm {
    name: string;
    url: string;
    username: string;
    password: string;
  }

  interface Props {
    showAddRegistryModal: boolean;
    newRegistry: NewRegistryForm;
    actionLoading: boolean;
    onSubmitAddRegistry: () => void;
  }

  let {
    showAddRegistryModal = $bindable(),
    newRegistry = $bindable(),
    actionLoading,
    onSubmitAddRegistry,
  }: Props = $props();

  function applyPreset(name: string, url: string) {
    newRegistry = { ...newRegistry, name, url };
  }
</script>

<Modal
  open={showAddRegistryModal}
  title="Hubungkan Registri Privat (docker login)"
  size="md"
  onclose={() => (showAddRegistryModal = false)}
>
  <form
    id="docker-add-reg-form"
    class="pg-docker-add-registry-modal docker-modal-form"
    onsubmit={(e) => {
      e.preventDefault();
      onSubmitAddRegistry();
    }}
  >
    <div class="flex-row gap-2">
      <button type="button" class="btn btn--secondary btn--sm" onclick={() => applyPreset('GitHub GHCR', 'ghcr.io')}>
        GHCR
      </button>
      <button type="button" class="btn btn--secondary btn--sm" onclick={() => applyPreset('GitLab Registry', 'registry.gitlab.com')}>
        GitLab
      </button>
      <button type="button" class="btn btn--secondary btn--sm" onclick={() => applyPreset('Docker Hub Privat', 'https://index.docker.io/v1/')}>
        Docker Hub
      </button>
    </div>

    <div class="field">
      <label class="field__label" for="reg-name">Nama Registri *</label>
      <input
        id="reg-name"
        type="text"
        class="input"
        placeholder="misal: GitHub Container Registry, Harbor Internal"
        bind:value={newRegistry.name}
        required
      />
    </div>

    <div class="field">
      <label class="field__label" for="reg-url">URL Host Registri *</label>
      <input
        id="reg-url"
        type="text"
        class="input font-mono"
        placeholder="misal: ghcr.io atau registry.mycompany.com"
        bind:value={newRegistry.url}
        required
      />
    </div>

    <div class="form-row">
      <div class="field flex-1">
        <label class="field__label" for="reg-user">Username</label>
        <input
          id="reg-user"
          type="text"
          class="input"
          placeholder="Username akun registri"
          bind:value={newRegistry.username}
        />
      </div>
      <div class="field flex-1">
        <label class="field__label" for="reg-pass">Password / Personal Access Token</label>
        <input
          id="reg-pass"
          type="password"
          class="input font-mono"
          placeholder="••••••••"
          bind:value={newRegistry.password}
        />
      </div>
    </div>
  </form>

  {#snippet footer()}
    <button
      type="button"
      class="btn btn--secondary"
      onclick={() => (showAddRegistryModal = false)}
    >
      Batal
    </button>
    <button
      type="submit"
      form="docker-add-reg-form"
      class="btn btn--primary"
      disabled={actionLoading}
    >
      <i class="ri-login-box-line" aria-hidden="true"></i>
      <span>Login & Simpan Registri</span>
    </button>
  {/snippet}
</Modal>
