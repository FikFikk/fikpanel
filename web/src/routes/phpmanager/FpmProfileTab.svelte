<script lang="ts">
  import '../../styles/pages/phpmanager/fpm-profile-tab.css';

  type ManageTab = 'service' | 'sites' | 'config' | 'raw_ini' | 'fpm_pool' | 'raw_fpm' | 'disabled_func' | 'extensions' | 'opcache' | 'logs' | 'phpinfo';

  // Antarmuka properti untuk tab profil alokasi worker process FastCGI
  interface Props {
    fpmPm: string;
    fpmMaxChildren: string;
    fpmStartServers: string;
    fpmMinSpare: string;
    fpmMaxSpare: string;
    fpmMaxRequests: string;
    savingFpmPool: boolean;
    onSwitchTab: (tab: ManageTab) => void;
    onApplyFpmRamPreset: (gb: number) => void;
    onSubmitFpmPool: () => void;
  }

  let {
    fpmPm = $bindable(),
    fpmMaxChildren = $bindable(),
    fpmStartServers = $bindable(),
    fpmMinSpare = $bindable(),
    fpmMaxSpare = $bindable(),
    fpmMaxRequests = $bindable(),
    savingFpmPool,
    onSwitchTab,
    onApplyFpmRamPreset,
    onSubmitFpmPool,
  }: Props = $props();
</script>

<!-- Tab 4: FPM Profile -->
<div class="pg-phpmanager-fpm-profile-tab tab-pane">
  <div class="section-title-wrap flex justify-between items-center">
    <div>
      <h4 class="section-title">FPM Profile (Pool: www.conf)</h4>
      <p class="section-desc">Konfigurasi alokasi worker process FastCGI untuk performa maksimal dan efisiensi RAM.</p>
    </div>
    <button class="btn btn-ghost btn-sm" onclick={() => onSwitchTab('raw_fpm')}>
      <i class="ri-file-settings-line"></i> Buka www.conf Mentah
    </button>
  </div>

  <!-- Preset RAM 1-Klik -->
  <div class="preset-box">
    <span class="preset-title">Rekomendasi Preset Cepat Berdasarkan Kapasitas RAM:</span>
    <div class="flex flex-wrap gap-2 mt-2">
      <button type="button" class="btn btn-ghost btn-sm" onclick={() => onApplyFpmRamPreset(1)}>RAM 1 GB</button>
      <button type="button" class="btn btn-ghost btn-sm" onclick={() => onApplyFpmRamPreset(2)}>RAM 2 GB</button>
      <button type="button" class="btn btn-ghost btn-sm" onclick={() => onApplyFpmRamPreset(4)}>RAM 4 GB</button>
      <button type="button" class="btn btn-ghost btn-sm" onclick={() => onApplyFpmRamPreset(8)}>RAM 8 GB+</button>
    </div>
  </div>

  <div class="visual-form mt-4">
    <div class="form-row">
      <div class="form-label-wrap">
        <label for="fpm-pm" class="f-label">pm (Process Manager)</label>
        <span class="f-hint">Metode pembuatan proses worker (dynamic sangat disarankan untuk fleksibilitas).</span>
      </div>
      <div class="form-input-wrap">
        <select id="fpm-pm" class="form-select" bind:value={fpmPm}>
          <option value="dynamic">dynamic (Menyesuaikan beban trafik secara dinamis)</option>
          <option value="static">static (Jumlah worker konstan di memori)</option>
          <option value="ondemand">ondemand (Worker hanya dibuat saat request datang)</option>
        </select>
      </div>
    </div>

    <div class="form-row">
      <div class="form-label-wrap">
        <label for="fpm-mc" class="f-label">pm.max_children</label>
        <span class="f-hint">Jumlah maksimum proses worker bersamaan yang diizinkan aktif.</span>
      </div>
      <div class="form-input-wrap">
        <input id="fpm-mc" type="text" class="form-input mono" bind:value={fpmMaxChildren} />
      </div>
    </div>

    <div class="form-row">
      <div class="form-label-wrap">
        <label for="fpm-ss" class="f-label">pm.start_servers</label>
        <span class="f-hint">Jumlah worker yang langsung dinyalakan saat layanan PHP-FPM start.</span>
      </div>
      <div class="form-input-wrap">
        <input id="fpm-ss" type="text" class="form-input mono" bind:value={fpmStartServers} />
      </div>
    </div>

    <div class="form-row">
      <div class="form-label-wrap">
        <label for="fpm-min-s" class="f-label">pm.min_spare_servers</label>
        <span class="f-hint">Batas minimum worker idle yang selalu siap melayani request baru.</span>
      </div>
      <div class="form-input-wrap">
        <input id="fpm-min-s" type="text" class="form-input mono" bind:value={fpmMinSpare} />
      </div>
    </div>

    <div class="form-row">
      <div class="form-label-wrap">
        <label for="fpm-max-s" class="f-label">pm.max_spare_servers</label>
        <span class="f-hint">Batas maksimum worker idle sebelum dipangkas oleh master process.</span>
      </div>
      <div class="form-input-wrap">
        <input id="fpm-max-s" type="text" class="form-input mono" bind:value={fpmMaxSpare} />
      </div>
    </div>

    <div class="form-row">
      <div class="form-label-wrap">
        <label for="fpm-mr" class="f-label">pm.max_requests</label>
        <span class="f-hint">Daur ulang worker setelah melayani X request untuk mencegah memory leak.</span>
      </div>
      <div class="form-input-wrap">
        <input id="fpm-mr" type="text" class="form-input mono" bind:value={fpmMaxRequests} />
      </div>
    </div>
  </div>

  <div class="form-footer flex justify-end">
    <button class="btn btn-primary" onclick={onSubmitFpmPool} disabled={savingFpmPool}>
      {savingFpmPool ? 'Menyimpan...' : 'Simpan & Reload FPM'}
    </button>
  </div>
</div>
