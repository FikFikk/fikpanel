<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { PHPVersionInfo } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  let versions = $state<PHPVersionInfo[]>([]);
  let loading = $state(true);
  let selectedVer = $state<PHPVersionInfo | null>(null);
  let saving = $state(false);

  // Form settings edit
  let editMemoryLimit = $state('256M');
  let editUploadMax = $state('64M');
  let editPostMax = $state('64M');
  let editMaxExec = $state('300');

  let installingVer = $state<string | null>(null);

  async function loadPHPVersions() {
    try {
      loading = true;
      const res = await apiFetch<{ ok: boolean; versions: PHPVersionInfo[] }>('/api/php/versions');
      versions = res?.versions || [];
      if (versions.length > 0) {
        if (!selectedVer || !versions.some(v => v.version === selectedVer?.version)) {
          // Utamakan versi yang sudah terpasang
          const firstInstalled = versions.find((v) => v.installed) || versions[0];
          selectVer(firstInstalled);
        } else {
          const current = versions.find(v => v.version === selectedVer?.version);
          if (current) selectVer(current);
        }
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat status PHP', 'error');
    } finally {
      loading = false;
    }
  }

  function selectVer(v: PHPVersionInfo) {
    selectedVer = v;
    editMemoryLimit = v.settings?.['memory_limit'] || '256M';
    editUploadMax = v.settings?.['upload_max_filesize'] || '64M';
    editPostMax = v.settings?.['post_max_size'] || '64M';
    editMaxExec = v.settings?.['max_execution_time'] || '300';
  }

  async function installPHP(v: PHPVersionInfo) {
    const ok = await confirmDialog({
      title: `Pasang PHP ${v.version}`,
      message: `Pasang paket runtime PHP ${v.version} dan daemon php${v.version}-fpm melalui manajer paket APT? Proses berjalan di background.`,
      confirmLabel: 'Mulai Pasang',
      variant: 'primary',
    });
    if (!ok) return;

    try {
      installingVer = v.version;
      notify(`Memulai instalasi PHP ${v.version}...`, 'info');
      await apiFetch(`/api/appstore/install/php${v.version}`, { method: 'POST' });
      notify(`Instalasi PHP ${v.version} dimulai di background. Periksa menu App Store untuk melihat log instalasi.`, 'success');
      setTimeout(() => {
        loadPHPVersions();
      }, 5000);
    } catch (err: any) {
      notify(err.message || 'Gagal memicu instalasi PHP', 'error');
    } finally {
      installingVer = null;
    }
  }

  async function restartFPM(v: PHPVersionInfo) {
    const ok = await confirmDialog({
      title: 'Restart PHP-FPM',
      message: `Restart layanan php${v.version}-fpm? Situs yang menggunakan versi ini mungkin mengalami jeda sepersekian detik.`,
      confirmLabel: 'Restart',
      variant: 'primary',
    });
    if (!ok) return;

    try {
      notify(`Me-restart php${v.version}-fpm...`, 'info');
      await apiFetch(`/api/php/versions/${v.version}/restart`, { method: 'POST' });
      notify(`PHP ${v.version} FPM berhasil di-restart`, 'success');
      await loadPHPVersions();
    } catch (err: any) {
      notify(err.message || 'Gagal restart PHP-FPM', 'error');
    }
  }

  async function saveSettings() {
    if (!selectedVer) return;
    try {
      saving = true;
      notify(`Menyimpan konfigurasi php.ini untuk PHP ${selectedVer.version}...`, 'info');
      await apiFetch(`/api/php/versions/${selectedVer.version}/settings`, {
        method: 'POST',
        body: JSON.stringify({ key: 'memory_limit', value: editMemoryLimit }),
      });
      await apiFetch(`/api/php/versions/${selectedVer.version}/settings`, {
        method: 'POST',
        body: JSON.stringify({ key: 'upload_max_filesize', value: editUploadMax }),
      });
      await apiFetch(`/api/php/versions/${selectedVer.version}/settings`, {
        method: 'POST',
        body: JSON.stringify({ key: 'post_max_size', value: editPostMax }),
      });
      await apiFetch(`/api/php/versions/${selectedVer.version}/settings`, {
        method: 'POST',
        body: JSON.stringify({ key: 'max_execution_time', value: editMaxExec }),
      });

      notify('Konfigurasi php.ini berhasil disimpan & FPM di-reload', 'success');
      await loadPHPVersions();
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan konfigurasi PHP', 'error');
    } finally {
      saving = false;
    }
  }

  onMount(() => {
    loadPHPVersions();
  });
</script>

<div class="php-view">
  <!-- Header Bar -->
  <header class="page-head">
    <div class="page-title-wrap">
      <h1 class="page-title">
        <i class="ri-code-s-slash-line"></i> PHP Multi-Version & FPM Tuning
      </h1>
      <p class="page-sub">
        Deteksi otomatis versi PHP yang terinstal di host, pantau daemon FPM socket, dan atur parameter alokasi memori serta upload.
      </p>
    </div>
    <div class="page-actions">
      <button class="btn btn-ghost" onclick={loadPHPVersions} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh Status
      </button>
    </div>
  </header>

  <div class="php-layout">
    <!-- Version Selector Cards -->
    <div class="versions-grid">
      {#each versions as v (v.version)}
        <div
          class="v-card {selectedVer?.version === v.version ? 'selected' : ''} {!v.installed ? 'not-installed' : ''}"
          onclick={() => selectVer(v)}
          role="button"
          tabindex="0"
          onkeydown={(e) => { if (e.key === 'Enter') selectVer(v); }}
        >
          <div class="v-head">
            <span class="v-tag">PHP {v.version}</span>
            {#if v.running}
              <span class="status-pill running">FPM RUNNING</span>
            {:else if v.installed}
              <span class="status-pill stopped">STOPPED</span>
            {:else}
              <span class="status-pill uninstalled">BELUM TERPASANG</span>
            {/if}
          </div>
          <div class="v-meta">
            {#if v.installed}
              <span class="mono text-subtle text-xs truncate" title={v.fpm_socket}>{v.fpm_socket || 'Socket Aktif'}</span>
            {:else}
              <span class="text-subtle text-xs">Tersedia di repositori OS</span>
            {/if}
          </div>
          <div class="v-actions" onclick={(e) => e.stopPropagation()} role="presentation">
            {#if v.installed}
              <button class="btn btn-ghost btn-sm" onclick={() => restartFPM(v)}>
                <i class="ri-restart-line"></i> Restart FPM
              </button>
            {:else}
              <button class="btn btn-primary btn-sm" onclick={() => installPHP(v)} disabled={installingVer === v.version}>
                <i class="ri-download-2-line {installingVer === v.version ? 'spin' : ''}"></i> {installingVer === v.version ? 'Memasang...' : 'Pasang'}
              </button>
            {/if}
          </div>
        </div>
      {/each}
    </div>

    <!-- Tuning Panel / Uninstalled Banner -->
    {#if selectedVer && selectedVer.installed}
      <div class="tuning-panel">
        <div class="panel-head">
          <h3 class="panel-title flex items-center gap-2">
            <i class="ri-settings-3-line"></i> Parameter php.ini — PHP {selectedVer.version}
          </h3>
          <span class="mono text-muted text-xs">{selectedVer.iniPath}</span>
        </div>

        <div class="tuning-body">
          <div class="form-grid">
            <div class="form-group">
              <label for="mem-lim">Memory Limit (memory_limit)</label>
              <input id="mem-lim" type="text" bind:value={editMemoryLimit} placeholder="256M" class="mono" />
              <span class="hint">Alokasi RAM maksimum untuk setiap eksekusi script skrip PHP.</span>
            </div>

            <div class="form-group">
              <label for="max-up">Upload Max Filesize (upload_max_filesize)</label>
              <input id="max-up" type="text" bind:value={editUploadMax} placeholder="64M" class="mono" />
              <span class="hint">Batas ukuran maksimum berkas unggahan per permintaan HTTP.</span>
            </div>

            <div class="form-group">
              <label for="post-max">Post Max Size (post_max_size)</label>
              <input id="post-max" type="text" bind:value={editPostMax} placeholder="64M" class="mono" />
              <span class="hint">Batas muatan data form POST total (harus >= upload_max_filesize).</span>
            </div>

            <div class="form-group">
              <label for="max-exec">Max Execution Time (max_execution_time)</label>
              <input id="max-exec" type="text" bind:value={editMaxExec} placeholder="300" class="mono" />
              <span class="hint">Batas detik eksekusi skrip sebelum dipaksa berhenti oleh runtime.</span>
            </div>
          </div>

          <div class="flex justify-end pt-4 border-t border-neutral-800">
            <button class="btn btn-primary" onclick={saveSettings} disabled={saving}>
              {saving ? 'Menyimpan...' : 'Simpan & Reload PHP-FPM'}
            </button>
          </div>
        </div>
      </div>
    {:else if selectedVer && !selectedVer.installed}
      <div class="uninstalled-banner-card">
        <div class="banner-icon">
          <i class="ri-information-line"></i>
        </div>
        <div class="banner-content">
          <h4>PHP {selectedVer.version} Belum Terpasang di Server</h4>
          <p>
            Paket runtime PHP {selectedVer.version} dan ekstensi FPM belum terinstal di host Anda. Anda dapat memasang paket ini secara otomatis hanya dengan 1-klik melalui manajer paket sistem.
          </p>
          <div class="banner-actions">
            <button class="btn btn-primary" onclick={() => installPHP(selectedVer!)} disabled={installingVer === selectedVer.version}>
              <i class="ri-download-2-line {installingVer === selectedVer.version ? 'spin' : ''}"></i>
              {installingVer === selectedVer.version ? 'Sedang Memasang...' : `Pasang PHP ${selectedVer.version} via APT`}
            </button>
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .php-view {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .page-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .page-title {
    font-size: 1.4rem;
    font-weight: 700;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.6rem;
    margin: 0;
  }
  .page-sub {
    font-size: 0.85rem;
    color: var(--text-muted);
    margin: 0.25rem 0 0 0;
  }
  .page-actions {
    display: flex;
    gap: 0.5rem;
  }
  .php-layout {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .versions-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 1rem;
  }
  .v-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    padding: 1rem 1.25rem;
    cursor: pointer;
    transition: all 0.15s;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .v-card:hover {
    border-color: var(--border-hover);
    background: var(--bg-hover);
  }
  .v-card.selected {
    border-color: var(--accent);
    background: var(--bg-surface-soft);
    box-shadow: 0 0 14px var(--ring-accent);
  }
  .v-card.not-installed {
    opacity: 0.6;
  }
  .v-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .v-tag {
    font-size: 1rem;
    font-weight: 700;
    color: var(--text);
  }
  .status-pill {
    font-size: 0.65rem;
    font-weight: 700;
    padding: 0.15rem 0.45rem;
    border-radius: var(--radius-sm);
  }
  .status-pill.running {
    background: var(--tint-accent);
    color: var(--accent);
  }
  .status-pill.stopped {
    background: var(--tint-danger);
    color: var(--danger);
  }
  .status-pill.uninstalled {
    background: var(--bg-inset);
    color: var(--text-subtle);
  }
  .tuning-panel {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    overflow: hidden;
  }
  .panel-head {
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--border-soft);
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .panel-title {
    font-size: 1rem;
    font-weight: 700;
    margin: 0;
    color: var(--text);
  }
  .tuning-body {
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .form-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1.25rem;
  }
  .form-group label {
    display: block;
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--text-muted);
    margin-bottom: 0.35rem;
  }
  .form-group input {
    width: 100%;
    padding: 0.6rem 0.75rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    color: var(--text);
    font-size: 0.82rem;
    outline: none;
    box-sizing: border-box;
  }
  .hint {
    display: block;
    font-size: 0.7rem;
    color: var(--text-subtle);
    margin-top: 0.3rem;
  }
  .btn-sm {
    padding: 0.3rem 0.6rem;
    font-size: 0.75rem;
  }
  .flex { display: flex; }
  .justify-between { justify-content: space-between; }
  .items-center { align-items: center; }
  .gap-2 { gap: 0.5rem; }
  .mono { font-family: var(--font-mono); }
  .text-muted { color: var(--text-muted); }
  .uninstalled-banner-card {
    background: var(--bg-surface);
    border: 1px dashed var(--border);
    border-radius: var(--radius-xl);
    padding: 2rem;
    display: flex;
    align-items: flex-start;
    gap: 1.25rem;
  }
  .banner-icon {
    width: 44px;
    height: 44px;
    border-radius: 10px;
    background: var(--tint-info);
    color: var(--info);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.5rem;
    flex-shrink: 0;
  }
  .banner-content {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .banner-content h4 {
    margin: 0;
    font-size: 1.05rem;
    color: var(--text);
  }
  .banner-content p {
    margin: 0;
    font-size: 0.82rem;
    color: var(--text-muted);
    line-height: 1.5;
  }
  .banner-actions {
    margin-top: 0.75rem;
  }
  .spin { animation: spin 1s linear infinite; }
  @keyframes spin { 100% { transform: rotate(360deg); } }
</style>
