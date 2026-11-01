<script lang="ts">
  // Subkomponen modal pembuatan jaringan virtual Docker baru
  import '../../styles/pages/docker/create-network-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';

  interface NewNetworkForm {
    name: string;
    driver: string;
    subnet: string;
    gateway: string;
  }

  interface Props {
    showCreateNetworkModal: boolean;
    newNetwork: NewNetworkForm;
    actionLoading: boolean;
    onSubmitCreateNetwork: () => void;
  }

  let {
    showCreateNetworkModal = $bindable(),
    newNetwork = $bindable(),
    actionLoading,
    onSubmitCreateNetwork,
  }: Props = $props();
</script>

<Modal
  open={showCreateNetworkModal}
  title="Buat Jaringan Virtual Baru"
  size="md"
  onclose={() => (showCreateNetworkModal = false)}
>
  <form
    id="docker-create-net-form"
    class="pg-docker-create-network-modal docker-modal-form"
    onsubmit={(e) => {
      e.preventDefault();
      onSubmitCreateNetwork();
    }}
  >
    <div class="field">
      <label class="field__label" for="net-name">Nama Network *</label>
      <input
        id="net-name"
        type="text"
        class="input font-mono"
        placeholder="misal: backend-net"
        bind:value={newNetwork.name}
        required
      />
    </div>

    <div class="field">
      <label class="field__label" for="net-driver">Driver</label>
      <select id="net-driver" class="input" bind:value={newNetwork.driver}>
        <option value="bridge">bridge (Default isolasi lokal)</option>
        <option value="host">host (Gunakan network host langsung)</option>
        <option value="overlay">overlay (Multi-host swarm)</option>
        <option value="macvlan">macvlan</option>
      </select>
    </div>

    <div class="form-row">
      <div class="field flex-1">
        <label class="field__label" for="net-subnet">Subnet CIDR</label>
        <input
          id="net-subnet"
          type="text"
          class="input font-mono"
          placeholder="172.28.0.0/16"
          bind:value={newNetwork.subnet}
        />
      </div>
      <div class="field flex-1">
        <label class="field__label" for="net-gw">Gateway IP</label>
        <input
          id="net-gw"
          type="text"
          class="input font-mono"
          placeholder="172.28.0.1"
          bind:value={newNetwork.gateway}
        />
      </div>
    </div>
  </form>

  {#snippet footer()}
    <button
      type="button"
      class="btn btn--secondary"
      onclick={() => (showCreateNetworkModal = false)}
    >
      Batal
    </button>
    <button
      type="submit"
      form="docker-create-net-form"
      class="btn btn--primary"
      disabled={actionLoading}
    >
      <i class="ri-check-line" aria-hidden="true"></i>
      <span>Buat Jaringan</span>
    </button>
  {/snippet}
</Modal>

