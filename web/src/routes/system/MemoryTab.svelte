<script lang="ts">
  import { formatBytes } from '../../lib/api';
  import type { MemoryOptimizationStatus } from '../../lib/types';

  interface Props {
    memory: MemoryOptimizationStatus | null;
    busyKey: string;
    lastFreedRAM: number | null;
    onCreateSwap: (sizeMB: number) => void;
    onRemoveSwap: () => void;
    onSaveTuning: (swappiness: number, vfsPressure: number) => void;
    onDropCaches: () => void;
  }

  let {
    memory,
    busyKey,
    lastFreedRAM,
    onCreateSwap,
    onRemoveSwap,
    onSaveTuning,
    onDropCaches,
  }: Props = $props();

  let selectedSwapMB = $state(2048);
  let swappinessInput = $state(30);
  let vfsPressureInput = $state(100);

  $effect(() => {
    if (memory) {
      swappinessInput = memory.swappiness;
      vfsPressureInput = memory.vfs_cache_pressure;
    }
  });

  const swapPresets = [
    { mb: 1024, label: '1 GB', hint: 'VPS 1 GB RAM' },
    { mb: 2048, label: '2 GB', hint: 'VPS 2 GB RAM' },
    { mb: 4096, label: '4 GB', hint: 'Build Vite / Composer' },
    { mb: 8192, label: '8 GB', hint: 'Beban Berat / DB' },
  ];
</script>

<div class="sys-pane">
  <!-- KPI Memori & Swap Live -->
  <div class="sys-kpi-grid">
    <div class="stat-card">
      <div class="stat-card__icon">
        <i class="ri-ram-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">RAM Tersedia</span>
        <span class="stat-card__value">{formatBytes(memory?.mem_avail_bytes || 0)}</span>
        <span class="stat-card__sub">dari total {formatBytes(memory?.mem_total_bytes || 0)}</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--info">
        <i class="ri-database-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">RAM Page Cache</span>
        <span class="stat-card__value">{formatBytes(memory?.mem_cache_bytes || 0)}</span>
        <span class="stat-card__sub">Buffers + Cached + SReclaimable</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--violet">
        <i class="ri-swap-box-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Total Swap Aktif</span>
        <span class="stat-card__value">{formatBytes(memory?.swap_total_bytes || 0)}</span>
        <span class="stat-card__sub">Terpakai: {formatBytes(memory?.swap_used_bytes || 0)}</span>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-card__icon stat-card__icon--warning">
        <i class="ri-equalizer-line" aria-hidden="true"></i>
      </div>
      <div class="stat-card__body">
        <span class="stat-card__label">Kernel Swappiness</span>
        <span class="stat-card__value mono">{memory?.swappiness ?? 60}</span>
        <span class="stat-card__sub">VFS Cache Pressure: {memory?.vfs_cache_pressure ?? 100}</span>
      </div>
    </div>
  </div>

  <div class="sys-two-col">
    <!-- 1-Klik Swapfile Manager -->
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-swap-box-line" aria-hidden="true"></i>
          <span>1-Klik Swapfile Manager</span>
        </span>
        <span class="badge badge--info mono">{memory?.fikpanel_swap_path || '/swapfile.fikpanel'}</span>
      </div>
      <div class="card__body stack">
        <p class="text-xs text-muted">
          Cegah OOM Killer saat menjalankan build Vite, Composer, atau lonjakan trafik database. Swapfile otomatis diformat (<code>0600</code>), diaktifkan, dan didaftarkan persisten di <code>/etc/fstab</code>.
        </p>

        <div class="sys-preset-grid">
          {#each swapPresets as preset}
            <button
              type="button"
              class="sys-preset-btn"
              class:is-selected={selectedSwapMB === preset.mb}
              onclick={() => (selectedSwapMB = preset.mb)}
            >
              <span>{preset.label}</span>
              <small>{preset.hint}</small>
            </button>
          {/each}
        </div>

        <div class="field">
          <label class="field__label" for="swap-size-mb">Ukuran Kustom (MB)</label>
          <input
            id="swap-size-mb"
            type="number"
            min="512"
            max="16384"
            step="256"
            class="input input--mono"
            bind:value={selectedSwapMB}
          />
          <span class="field__hint">
            Ruang kosong root '/': <strong class="mono">{formatBytes(memory?.root_free_bytes || 0)}</strong> (wajib tersisa minimal 2 GB).
          </span>
        </div>
      </div>
      <div class="card__foot">
        {#if memory?.has_fikpanel_swap}
          <button
            type="button"
            class="btn btn--danger btn--sm"
            disabled={busyKey === 'mem:swap-del'}
            onclick={onRemoveSwap}
          >
            {busyKey === 'mem:swap-del' ? 'Menghapus...' : 'Hapus Swapfile'}
          </button>
        {/if}
        <button
          type="button"
          class="btn btn--primary btn--sm"
          disabled={busyKey === 'mem:swap-create'}
          onclick={() => onCreateSwap(Number(selectedSwapMB))}
        >
          <i class="ri-add-circle-line" aria-hidden="true"></i>
          <span>
            {busyKey === 'mem:swap-create'
              ? 'Mengalokasikan Swapfile...'
              : memory?.has_fikpanel_swap
                ? 'Ubah Ukuran Swapfile'
                : 'Buat & Aktifkan Swapfile'}
          </span>
        </button>
      </div>
    </div>

    <!-- Tuning Memori Kernel & Drop RAM Cache -->
    <div class="stack">
      <div class="card">
        <div class="card__head">
          <span class="card__title">
            <i class="ri-equalizer-line" aria-hidden="true"></i>
            <span>Tuning Kernel Memori (Persisten sysctl.d)</span>
          </span>
        </div>
        <div class="card__body stack">
          <div class="field">
            <label class="field__label" for="vm-swappiness">
              vm.swappiness (0–100)
            </label>
            <input
              id="vm-swappiness"
              type="number"
              min="0"
              max="100"
              class="input input--mono"
              bind:value={swappinessInput}
            />
            <div class="row wrap">
              <button type="button" class="btn btn--secondary btn--sm" onclick={() => (swappinessInput = 10)}>
                10 (Database / Produksi)
              </button>
              <button type="button" class="btn btn--secondary btn--sm" onclick={() => (swappinessInput = 30)}>
                30 (Seimbang)
              </button>
              <button type="button" class="btn btn--secondary btn--sm" onclick={() => (swappinessInput = 60)}>
                60 (Bawaan Linux)
              </button>
            </div>
          </div>

          <div class="field">
            <label class="field__label" for="vm-vfs">
              vm.vfs_cache_pressure (10–500)
            </label>
            <input
              id="vm-vfs"
              type="number"
              min="10"
              max="500"
              class="input input--mono"
              bind:value={vfsPressureInput}
            />
            <span class="field__hint">Nilai 50 mempertahankan cache direktori/inode lebih lama (bagus untuk Laravel/WordPress).</span>
          </div>
        </div>
        <div class="card__foot">
          <button
            type="button"
            class="btn btn--primary btn--sm"
            disabled={busyKey === 'mem:tuning'}
            onclick={() => onSaveTuning(Number(swappinessInput), Number(vfsPressureInput))}
          >
            {busyKey === 'mem:tuning' ? 'Menerapkan...' : 'Terapkan Tuning Kernel'}
          </button>
        </div>
      </div>

      <div class="card">
        <div class="card__head">
          <span class="card__title">
            <i class="ri-flashlight-line" aria-hidden="true"></i>
            <span>Bebaskan RAM Page Cache</span>
          </span>
        </div>
        <div class="card__body stack">
          <p class="text-xs text-muted">
            Menjalankan <code>sync</code> lalu melepas page, dentry, dan inode cache kernel secara aman tanpa mengganggu aplikasi yang sedang berjalan.
          </p>
          {#if lastFreedRAM !== null}
            <div class="alert alert--success" role="status">
              <i class="ri-checkbox-circle-line" aria-hidden="true"></i>
              <span>RAM page cache berhasil dibersihkan — ruang memori dibebaskan: <strong class="mono">{formatBytes(lastFreedRAM)}</strong>.</span>
            </div>
          {/if}
        </div>
        <div class="card__foot">
          <button
            type="button"
            class="btn btn--secondary btn--sm"
            disabled={busyKey === 'mem:drop'}
            onclick={onDropCaches}
          >
            <i class="ri-flashlight-line" aria-hidden="true"></i>
            <span>{busyKey === 'mem:drop' ? 'Mengosongkan Cache...' : 'Drop RAM Cache Sekarang'}</span>
          </button>
        </div>
      </div>
    </div>
  </div>

  <!-- Tabel Perangkat Swap Aktif (/proc/swaps) -->
  <div class="card">
    <div class="card__head">
      <span class="card__title">
        <i class="ri-list-check-2" aria-hidden="true"></i>
        <span>Daftar Perangkat Swap Aktif (/proc/swaps)</span>
      </span>
    </div>
    <div class="table-wrap">
      <table class="table table--dense">
        <thead>
          <tr>
            <th>Lokasi / Perangkat</th>
            <th>Tipe</th>
            <th>Kapasitas</th>
            <th>Terpakai</th>
            <th>Prioritas</th>
          </tr>
        </thead>
        <tbody>
          {#if !memory?.swaps || memory.swaps.length === 0}
            <tr>
              <td colspan="5">
                <div class="empty-state">
                  <i class="ri-swap-box-line" aria-hidden="true"></i>
                  <div class="empty-state__title">Tidak Ada Swap Aktif</div>
                  <p class="state__desc">Gunakan 1-Klik Swapfile Manager di atas untuk menambahkan memori cadangan pada VPS Anda.</p>
                </div>
              </td>
            </tr>
          {:else}
            {#each memory.swaps as sw (sw.filename)}
              <tr>
                <td class="mono"><strong>{sw.filename}</strong></td>
                <td><span class="badge badge--neutral">{sw.type}</span></td>
                <td class="mono">{formatBytes(sw.size_bytes)}</td>
                <td class="mono">{formatBytes(sw.used_bytes)}</td>
                <td class="mono text-subtle">{sw.priority}</td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</div>
