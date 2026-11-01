<script lang="ts">
  import '../../styles/pages/phpmanager/opcache-tab.css';

  // Antarmuka properti untuk tab optimasi OPcache
  interface Props {
    opcacheEnable: string;
    opcacheMem: string;
    opcacheMaxFiles: string;
    savingOpcache: boolean;
    onSubmitOpcache: () => void;
  }

  let {
    opcacheEnable = $bindable(),
    opcacheMem = $bindable(),
    opcacheMaxFiles = $bindable(),
    savingOpcache,
    onSubmitOpcache,
  }: Props = $props();
</script>

<div class="tab-pane pg-phpmanager-opcache-tab">
  <div class="section-title-wrap">
    <h4 class="section-title">Optimization (Zend OPcache)</h4>
    <p class="section-desc">Optimasi bytecode compiler caching untuk mempercepat load time aplikasi berbasis PHP hingga 3x lipat.</p>
  </div>

  <div class="visual-form">
    <div class="form-row">
      <div class="form-label-wrap">
        <label for="opc-en" class="f-label">opcache.enable</label>
        <span class="f-hint">Aktifkan Zend OPcache bytecode compiler cache.</span>
      </div>
      <div class="form-input-wrap">
        <select id="opc-en" class="form-select" bind:value={opcacheEnable}>
          <option value="1">1 (Aktif - Sangat Disarankan)</option>
          <option value="0">0 (Nonaktif)</option>
        </select>
      </div>
    </div>

    <div class="form-row">
      <div class="form-label-wrap">
        <label for="opc-mem" class="f-label">opcache.memory_consumption</label>
        <span class="f-hint">Alokasi memori RAM terdedikasi untuk penyimpanan bytecode cache (MB).</span>
      </div>
      <div class="form-input-wrap">
        <input id="opc-mem" type="text" class="form-input mono" bind:value={opcacheMem} placeholder="128" />
        <div class="quick-chips">
          <button type="button" class="chip" onclick={() => (opcacheMem = '64')}>64M</button>
          <button type="button" class="chip" onclick={() => (opcacheMem = '128')}>128M</button>
          <button type="button" class="chip" onclick={() => (opcacheMem = '256')}>256M</button>
          <button type="button" class="chip" onclick={() => (opcacheMem = '512')}>512M</button>
        </div>
      </div>
    </div>

    <div class="form-row">
      <div class="form-label-wrap">
        <label for="opc-files" class="f-label">opcache.max_accelerated_files</label>
        <span class="f-hint">Jumlah maksimum file skrip PHP yang dapat disimpan di dalam tabel hash cache.</span>
      </div>
      <div class="form-input-wrap">
        <input id="opc-files" type="text" class="form-input mono" bind:value={opcacheMaxFiles} placeholder="10000" />
      </div>
    </div>
  </div>

  <div class="form-footer flex justify-end">
    <button class="btn btn-primary" onclick={onSubmitOpcache} disabled={savingOpcache}>
      {savingOpcache ? 'Menyimpan...' : 'Simpan & Terapkan OPcache'}
    </button>
  </div>
</div>
