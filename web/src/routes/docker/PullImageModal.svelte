<script lang="ts">
  // Subkomponen modal penarikan citra Docker (Pull Image)
  import '../../styles/pages/docker/pull-image-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';

  interface Props {
    showPullImageModal: boolean;
    pullImageName: string;
    pullImageTag: string;
    pulling: boolean;
    onSubmitPullImage: () => void;
  }

  let {
    showPullImageModal = $bindable(),
    pullImageName = $bindable(),
    pullImageTag = $bindable(),
    pulling,
    onSubmitPullImage,
  }: Props = $props();
</script>

<Modal
  open={showPullImageModal}
  title="Tarik Citra (Pull Image)"
  size="md"
  onclose={() => (showPullImageModal = false)}
>
  <form
    id="docker-pull-img-form"
    class="pg-docker-pull-image-modal docker-modal-form"
    onsubmit={(e) => {
      e.preventDefault();
      onSubmitPullImage();
    }}
  >
    <div class="field">
      <label class="field__label" for="pull-name">Nama Citra Docker *</label>
      <input
        id="pull-name"
        type="text"
        class="input font-mono"
        placeholder="misal: nginx, ubuntu, redis, postgres"
        bind:value={pullImageName}
        required
      />
    </div>

    <div class="field">
      <label class="field__label" for="pull-tag">Tag Versi (Default: latest)</label>
      <input
        id="pull-tag"
        type="text"
        class="input font-mono"
        placeholder="latest, alpine, 18-bookworm, dll."
        bind:value={pullImageTag}
      />
    </div>
  </form>

  {#snippet footer()}
    <button type="button" class="btn btn--secondary" onclick={() => (showPullImageModal = false)}>
      Batal
    </button>
    <button type="submit" form="docker-pull-img-form" class="btn btn--primary" disabled={pulling}>
      <i class="ri-download-line {pulling ? 'spin' : ''}" aria-hidden="true"></i>
      <span>{pulling ? 'Sedang Mengunduh...' : 'Mulai Tarik Citra'}</span>
    </button>
  {/snippet}
</Modal>

