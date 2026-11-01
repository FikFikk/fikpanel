<script lang="ts">
  // Subkomponen modal editor stack Docker Compose
  import '../../styles/pages/docker/compose-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';

  interface ComposeForm {
    name: string;
    yaml: string;
  }

  interface Props {
    showComposeModal: boolean;
    composeForm: ComposeForm;
    actionLoading: boolean;
    onSubmitComposeUp: () => void;
  }

  let {
    showComposeModal = $bindable(),
    composeForm = $bindable(),
    actionLoading,
    onSubmitComposeUp,
  }: Props = $props();
</script>

<Modal
  open={showComposeModal}
  title="Stack Docker Compose"
  size="lg"
  onclose={() => (showComposeModal = false)}
>
  <form
    id="docker-compose-form"
    class="pg-docker-compose-modal docker-modal-form"
    onsubmit={(e) => {
      e.preventDefault();
      onSubmitComposeUp();
    }}
  >
    <div class="field">
      <label class="field__label" for="compose-name">Nama Proyek / Stack *</label>
      <input
        id="compose-name"
        type="text"
        class="input font-mono"
        placeholder="misal: wordpress-stack atau microservices"
        bind:value={composeForm.name}
        required
      />
    </div>

    <div class="field">
      <label class="field__label" for="compose-yaml">Konfigurasi docker-compose.yml *</label>
      <textarea
        id="compose-yaml"
        class="input font-mono text-xs"
        rows="12"
        bind:value={composeForm.yaml}
        required
      ></textarea>
    </div>
  </form>

  {#snippet footer()}
    <button type="button" class="btn btn--secondary" onclick={() => (showComposeModal = false)}>
      Batal
    </button>
    <button
      type="submit"
      form="docker-compose-form"
      class="btn btn--primary"
      disabled={actionLoading}
    >
      <i class="ri-play-line" aria-hidden="true"></i>
      <span>Deploy Stack (Compose Up)</span>
    </button>
  {/snippet}
</Modal>

