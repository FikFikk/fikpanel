<script lang="ts">
  // Subkomponen modal pemasangan One-Click Template Docker
  import '../../styles/pages/docker/deploy-template-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { OneClickTemplate } from '../../lib/types';

  interface DeployForm {
    name: string;
    port: number;
    volume_path: string;
    env_values: Record<string, string>;
  }

  interface Props {
    showDeployTemplateModal: boolean;
    selectedTemplate: OneClickTemplate | null;
    deployForm: DeployForm;
    actionLoading: boolean;
    onSubmitDeployTemplate: () => void;
  }

  let {
    showDeployTemplateModal = $bindable(),
    selectedTemplate,
    deployForm = $bindable(),
    actionLoading,
    onSubmitDeployTemplate,
  }: Props = $props();
</script>

<Modal
  open={showDeployTemplateModal && Boolean(selectedTemplate)}
  size="lg"
  onclose={() => (showDeployTemplateModal = false)}
>
  {#snippet header()}
    {#if selectedTemplate}
      <div class="flex-row items-center gap-3">
        <i class="{selectedTemplate.icon} text-xl text-accent" aria-hidden="true"></i>
        <div>
          <h2 class="modal__title">Pasang {selectedTemplate.name}</h2>
          <span class="text-xs text-muted font-mono">{selectedTemplate.image}</span>
        </div>
      </div>
    {/if}
  {/snippet}

  {#if selectedTemplate}
    <form
      id="docker-deploy-tmpl-form"
      class="pg-docker-deploy-template-modal docker-modal-form"
      onsubmit={(e) => {
        e.preventDefault();
        onSubmitDeployTemplate();
      }}
    >
      <div class="form-row">
        <div class="field flex-1">
          <label class="field__label" for="deploy-name">Nama Kontainer *</label>
          <input
            id="deploy-name"
            type="text"
            class="input font-mono"
            bind:value={deployForm.name}
            required
          />
        </div>
        <div class="field flex-1">
          <label class="field__label" for="deploy-port">Port Host Terbuka *</label>
          <input
            id="deploy-port"
            type="number"
            class="input font-mono"
            bind:value={deployForm.port}
            required
          />
        </div>
      </div>

      {#if selectedTemplate.volume_mount}
        <div class="field">
          <label class="field__label" for="deploy-vol">Direktori Penyimpanan Host</label>
          <input
            id="deploy-vol"
            type="text"
            class="input font-mono"
            bind:value={deployForm.volume_path}
          />
          <span class="field__hint">
            Akan dipetakan ke direktori kontainer: <code class="font-mono">{selectedTemplate.volume_mount}</code>
          </span>
        </div>
      {/if}

      {#if selectedTemplate.env_params && selectedTemplate.env_params.length > 0}
        <div class="form-section">
          <span class="font-semibold text-sm">Parameter Konfigurasi Lingkungan</span>
          <div class="mt-2 space-y-2">
            {#each selectedTemplate.env_params as p}
              <div class="field mb-2">
                <label class="field__label" for="env-{p.key}">
                  {p.label} <code class="font-mono text-2xs">({p.key})</code>
                </label>
                <input
                  id="env-{p.key}"
                  type={p.type === 'password' ? 'password' : 'text'}
                  class="input font-mono"
                  bind:value={deployForm.env_values[p.key]}
                />
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </form>
  {/if}

  {#snippet footer()}
    <button
      type="button"
      class="btn btn--secondary"
      onclick={() => (showDeployTemplateModal = false)}
    >
      Batal
    </button>
    <button
      type="submit"
      form="docker-deploy-tmpl-form"
      class="btn btn--primary"
      disabled={actionLoading}
    >
      <i class="ri-check-line" aria-hidden="true"></i>
      <span>Mulai Pasang & Jalankan</span>
    </button>
  {/snippet}
</Modal>

