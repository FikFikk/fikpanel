<script lang="ts">
  // Subkomponen modal penambahan kredensial registri privat Docker
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
</script>

<Modal
  open={showAddRegistryModal}
  title="Tambah Kredensial Registri Privat"
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
    <div class="field">
      <label class="field__label" for="reg-name">Nama Registri *</label>
      <input
        id="reg-name"
        type="text"
        class="input"
        placeholder="misal: Harbor Internal, Gitlab Registry"
        bind:value={newRegistry.name}
        required
      />
    </div>

    <div class="field">
      <label class="field__label" for="reg-url">URL Registri *</label>
      <input
        id="reg-url"
        type="text"
        class="input font-mono"
        placeholder="misal: registry.mycompany.com atau ghcr.io"
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
          placeholder="Username atau Token"
          bind:value={newRegistry.username}
        />
      </div>
      <div class="field flex-1">
        <label class="field__label" for="reg-pass">Password / Token Akses</label>
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
      <i class="ri-check-line" aria-hidden="true"></i>
      <span>Simpan Registri</span>
    </button>
  {/snippet}
</Modal>

