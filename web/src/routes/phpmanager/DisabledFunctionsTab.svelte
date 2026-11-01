<script lang="ts">
  import '../../styles/pages/phpmanager/disabled-functions-tab.css';

  // Antarmuka properti untuk tab fungsi PHP yang dinonaktifkan
  interface Props {
    disabledFuncs: string[];
    newFuncInput: string;
    savingDisabledFuncs: boolean;
    onApplyDisabledPreset: (preset: 'strict' | 'compat') => void;
    onAddDisabledFunction: () => void;
    onRemoveDisabledFunction: (fn: string) => void;
    onSubmitDisabledFunctions: () => void;
  }

  let {
    disabledFuncs = $bindable(),
    newFuncInput = $bindable(),
    savingDisabledFuncs,
    onApplyDisabledPreset,
    onAddDisabledFunction,
    onRemoveDisabledFunction,
    onSubmitDisabledFunctions,
  }: Props = $props();
</script>

<!-- Tab 6: Disabled Functions -->
<div class="pg-phpmanager-disabled-functions-tab tab-pane">
  <div class="section-title-wrap">
    <h4 class="section-title">Disabled Functions (Fungsi Dinonaktifkan)</h4>
    <p class="section-desc">
      Blokir eksekusi perintah shell dan fungsi berbahaya di tingkat PHP runtime untuk melindungi server dari serangan eksekusi kode berbahaya.
    </p>
  </div>

  <div class="flex justify-between items-center mb-3">
    <div class="flex gap-2">
      <button type="button" class="btn btn-ghost btn-sm" onclick={() => onApplyDisabledPreset('strict')}>
        <i class="ri-shield-check-line"></i> Preset Ketat (Enterprise)
      </button>
      <button type="button" class="btn btn-ghost btn-sm" onclick={() => onApplyDisabledPreset('compat')}>
        <i class="ri-apps-line"></i> Preset Kompatibel (Laravel/WP)
      </button>
      <button type="button" class="btn btn-ghost btn-sm text-danger" onclick={() => (disabledFuncs = [])}>
        <i class="ri-delete-bin-line"></i> Kosongkan Semua
      </button>
    </div>
  </div>

  <!-- Input Tambah Fungsi -->
  <div class="add-func-wrap flex gap-2 mb-4">
    <input
      type="text"
      class="form-input mono flex-1"
      placeholder="Ketik nama fungsi PHP (misal: exec, shell_exec, system, proc_open)..."
      bind:value={newFuncInput}
      onkeydown={(e) => {
        if (e.key === 'Enter') onAddDisabledFunction();
      }}
    />
    <button type="button" class="btn btn-primary" onclick={onAddDisabledFunction}>
      <i class="ri-add-line"></i> Tambahkan
    </button>
  </div>

  <!-- Visual Chips Container -->
  <div class="chips-container">
    {#if disabledFuncs.length === 0}
      <p class="text-subtle text-sm p-4 text-center">
        Tidak ada fungsi yang dinonaktifkan. Semua fungsi PHP dapat dieksekusi secara bebas.
      </p>
    {:else}
      {#each disabledFuncs as fn (fn)}
        <span class="func-chip">
          <span class="mono">{fn}</span>
          <button
            type="button"
            class="chip-remove"
            onclick={() => onRemoveDisabledFunction(fn)}
            title="Hapus {fn} dari daftar blokir"
            aria-label="Hapus fungsi {fn}"
          >
            <i class="ri-close-line"></i>
          </button>
        </span>
      {/each}
    {/if}
  </div>

  <div class="form-footer flex justify-between items-center mt-4">
    <span class="text-xs text-subtle">
      Total: {disabledFuncs.length} fungsi diblokir pada <code class="mono">disable_functions</code>.
    </span>
    <button class="btn btn-primary" onclick={onSubmitDisabledFunctions} disabled={savingDisabledFuncs}>
      {savingDisabledFuncs ? 'Menyimpan...' : 'Simpan & Terapkan ke php.ini'}
    </button>
  </div>
</div>
