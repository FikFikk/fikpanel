<script lang="ts">
  // Subkomponen modal pembuatan volume Docker baru
  import '../../styles/pages/docker/create-volume-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';

  interface NewVolumeForm {
    name: string;
    driver: string;
  }

  interface Props {
    showCreateVolumeModal: boolean;
    newVolume: NewVolumeForm;
    actionLoading: boolean;
    onSubmitCreateVolume: () => void;
  }

  let {
    showCreateVolumeModal = $bindable(),
    newVolume = $bindable(),
    actionLoading,
    onSubmitCreateVolume,
  }: Props = $props();
</script>

<Modal
  open={showCreateVolumeModal}
  title="Buat Volume Docker Baru"
  size="md"
  onclose={() => (showCreateVolumeModal = false)}
>
  <form
    id="docker-create-vol-form"
    class="pg-docker-create-volume-modal docker-modal-form"
    onsubmit={(e) => {
      e.preventDefault();
      onSubmitCreateVolume();
    }}
  >
    <div class="field">
      <label class="field__label" for="vol-name">Nama Volume *</label>
      <input
        id="vol-name"
        type="text"
        class="input font-mono"
        placeholder="misal: pg_data, redis_cache"
        bind:value={newVolume.name}
        required
      />
    </div>

    <div class="field">
      <label class="field__label" for="vol-driver">Driver</label>
      <select id="vol-driver" class="input" bind:value={newVolume.driver}>
        <option value="local">local (Penyimpanan host bawaan)</option>
      </select>
    </div>
  </form>

  {#snippet footer()}
    <button
      type="button"
      class="btn btn--secondary"
      onclick={() => (showCreateVolumeModal = false)}
    >
      Batal
    </button>
    <button
      type="submit"
      form="docker-create-vol-form"
      class="btn btn--primary"
      disabled={actionLoading}
    >
      <i class="ri-check-line" aria-hidden="true"></i>
      <span>Buat Volume</span>
    </button>
  {/snippet}
</Modal>

