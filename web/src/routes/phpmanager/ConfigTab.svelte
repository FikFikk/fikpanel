<script lang="ts">
  import type { PHPVersionDetail } from '../../lib/types';
  import '../../styles/pages/phpmanager/config-tab.css';

  type ManageTab = 'service' | 'sites' | 'config' | 'raw_ini' | 'fpm_pool' | 'raw_fpm' | 'disabled_func' | 'extensions' | 'opcache' | 'logs' | 'phpinfo';

  // Antarmuka properti untuk tab konfigurasi visual parameter php.ini
  interface Props {
    activeVersion: string;
    activeDetail: PHPVersionDetail | null;
    formMaxExec: string;
    formMaxInput: string;
    formMemoryLimit: string;
    formUploadMax: string;
    formPostMax: string;
    formDisplayErrors: string;
    formErrorReporting: string;
    formTimezone: string;
    savingConfig: boolean;
    onSwitchTab: (tab: ManageTab) => void;
    onSubmitVisualConfig: () => void;
  }

  let {
    activeVersion,
    activeDetail,
    formMaxExec = $bindable(),
    formMaxInput = $bindable(),
    formMemoryLimit = $bindable(),
    formUploadMax = $bindable(),
    formPostMax = $bindable(),
    formDisplayErrors = $bindable(),
    formErrorReporting = $bindable(),
    formTimezone = $bindable(),
    savingConfig,
    onSwitchTab,
    onSubmitVisualConfig,
  }: Props = $props();
</script>

<!-- Tab 2: Configuration (Visual ala Gambar 1) -->
<div class="pg-phpmanager-config-tab tab-pane">
  <div class="section-title-wrap flex justify-between items-center">
    <div>
      <h4 class="section-title">Configure Server: PHP {activeVersion}</h4>
      <p class="section-desc">Atur parameter dasar php.ini secara visual dengan penyesuaian nilai interaktif.</p>
    </div>
    <button class="btn btn-ghost btn-sm" onclick={() => onSwitchTab('raw_ini')}>
      <i class="ri-file-code-line"></i> Buka Editor php.ini
    </button>
  </div>

  <div class="visual-form">
    <!-- max_execution_time -->
    <div class="form-row">
      <div class="form-label-wrap">
        <label for="f-max-exec" class="f-label">max_execution_time</label>
        <span class="f-hint">Batas waktu eksekusi skrip dalam detik sebelum dipaksa berhenti.</span>
      </div>
      <div class="form-input-wrap">
        <input id="f-max-exec" type="text" class="form-input mono" bind:value={formMaxExec} placeholder="300" />
        <div class="quick-chips">
          <button type="button" class="chip" onclick={() => (formMaxExec = '60')}>60s</button>
          <button type="button" class="chip" onclick={() => (formMaxExec = '300')}>300s</button>
          <button type="button" class="chip" onclick={() => (formMaxExec = '600')}>600s</button>
          <button type="button" class="chip" onclick={() => (formMaxExec = '1200')}>1200s</button>
        </div>
      </div>
    </div>

    <!-- max_input_time -->
    <div class="form-row">
      <div class="form-label-wrap">
        <label for="f-max-input" class="f-label">max_input_time</label>
        <span class="f-hint">Waktu maksimum skrip mem-parsing data POST / GET dalam detik.</span>
      </div>
      <div class="form-input-wrap">
        <input id="f-max-input" type="text" class="form-input mono" bind:value={formMaxInput} placeholder="300" />
        <div class="quick-chips">
          <button type="button" class="chip" onclick={() => (formMaxInput = '60')}>60s</button>
          <button type="button" class="chip" onclick={() => (formMaxInput = '300')}>300s</button>
          <button type="button" class="chip" onclick={() => (formMaxInput = '600')}>600s</button>
        </div>
      </div>
    </div>

    <!-- memory_limit -->
    <div class="form-row">
      <div class="form-label-wrap">
        <label for="f-mem-limit" class="f-label">memory_limit</label>
        <span class="f-hint">Batas memori RAM maksimum yang diizinkan untuk setiap skrip PHP.</span>
      </div>
      <div class="form-input-wrap">
        <input id="f-mem-limit" type="text" class="form-input mono" bind:value={formMemoryLimit} placeholder="256M" />
        <div class="quick-chips">
          <button type="button" class="chip" onclick={() => (formMemoryLimit = '128M')}>128M</button>
          <button type="button" class="chip" onclick={() => (formMemoryLimit = '256M')}>256M</button>
          <button type="button" class="chip" onclick={() => (formMemoryLimit = '512M')}>512M</button>
          <button type="button" class="chip" onclick={() => (formMemoryLimit = '1024M')}>1024M</button>
          <button type="button" class="chip" onclick={() => (formMemoryLimit = '2048M')}>2048M</button>
        </div>
      </div>
    </div>

    <!-- upload_max_filesize -->
    <div class="form-row">
      <div class="form-label-wrap">
        <label for="f-upload-max" class="f-label">upload_max_filesize</label>
        <span class="f-hint">Ukuran maksimum sebuah berkas yang diunggah ke server.</span>
      </div>
      <div class="form-input-wrap">
        <input id="f-upload-max" type="text" class="form-input mono" bind:value={formUploadMax} placeholder="64M" />
        <div class="quick-chips">
          <button type="button" class="chip" onclick={() => (formUploadMax = '32M')}>32M</button>
          <button type="button" class="chip" onclick={() => (formUploadMax = '64M')}>64M</button>
          <button type="button" class="chip" onclick={() => (formUploadMax = '100M')}>100M</button>
          <button type="button" class="chip" onclick={() => (formUploadMax = '256M')}>256M</button>
          <button type="button" class="chip" onclick={() => (formUploadMax = '512M')}>512M</button>
        </div>
      </div>
    </div>

    <!-- post_max_size -->
    <div class="form-row">
      <div class="form-label-wrap">
        <label for="f-post-max" class="f-label">post_max_size</label>
        <span class="f-hint">Ukuran total muatan data POST HTTP (harus >= upload_max_filesize).</span>
      </div>
      <div class="form-input-wrap">
        <input id="f-post-max" type="text" class="form-input mono" bind:value={formPostMax} placeholder="64M" />
        <div class="quick-chips">
          <button type="button" class="chip" onclick={() => (formPostMax = '64M')}>64M</button>
          <button type="button" class="chip" onclick={() => (formPostMax = '100M')}>100M</button>
          <button type="button" class="chip" onclick={() => (formPostMax = '256M')}>256M</button>
          <button type="button" class="chip" onclick={() => (formPostMax = '512M')}>512M</button>
        </div>
      </div>
    </div>

    <!-- display_errors -->
    <div class="form-row">
      <div class="form-label-wrap">
        <label for="f-display-errors" class="f-label">display_errors</label>
        <span class="f-hint">Cetak galat langsung ke output browser (Gunakan Off di Production).</span>
      </div>
      <div class="form-input-wrap">
        <select id="f-display-errors" class="form-select" bind:value={formDisplayErrors}>
          <option value="Off">Off (Rekomendasi Aman)</option>
          <option value="On">On (Mode Debugging / Dev)</option>
        </select>
      </div>
    </div>

    <!-- error_reporting -->
    <div class="form-row">
      <div class="form-label-wrap">
        <label for="f-error-rep" class="f-label">error_reporting</label>
        <span class="f-hint">Tingkat keparahan galat yang dicatat oleh runtime PHP.</span>
      </div>
      <div class="form-input-wrap">
        <input id="f-error-rep" type="text" class="form-input mono" bind:value={formErrorReporting} />
        <div class="quick-chips">
          <button type="button" class="chip" onclick={() => (formErrorReporting = 'E_ALL & ~E_DEPRECATED & ~E_STRICT')}>Standard Prod</button>
          <button type="button" class="chip" onclick={() => (formErrorReporting = 'E_ALL')}>Semua (E_ALL)</button>
          <button type="button" class="chip" onclick={() => (formErrorReporting = '0')}>Mute (0)</button>
        </div>
      </div>
    </div>

    <!-- date.timezone -->
    <div class="form-row">
      <div class="form-label-wrap">
        <label for="f-tz" class="f-label">date.timezone</label>
        <span class="f-hint">Zona waktu standar untuk seluruh fungsi tanggal PHP.</span>
      </div>
      <div class="form-input-wrap">
        <div class="flex gap-2">
          <input id="f-tz" type="text" class="form-input mono flex-1" bind:value={formTimezone} placeholder="UTC" />
          <button type="button" class="btn btn-ghost btn-sm" onclick={() => (formTimezone = 'Asia/Jakarta')}>Asia/Jakarta</button>
          <button type="button" class="btn btn-ghost btn-sm" onclick={() => (formTimezone = 'UTC')}>UTC</button>
        </div>
      </div>
    </div>
  </div>

  <div class="form-footer flex justify-between items-center">
    <span class="text-xs text-subtle">
      Perubahan akan disimpan langsung ke <code class="mono">{activeDetail?.iniPath}</code> dan layanan FPM akan dimuat ulang.
    </span>
    <button class="btn btn-primary" onclick={onSubmitVisualConfig} disabled={savingConfig}>
      {savingConfig ? 'Menyimpan...' : 'Simpan & Reload PHP-FPM'}
    </button>
  </div>
</div>
