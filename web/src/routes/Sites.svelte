<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { Site } from '../lib/types';

  interface Props {
    onNotify: (msg: string) => void;
  }

  let { onNotify }: Props = $props();

  let sites = $state<Site[]>([]);
  let loading = $state(true);
  let showModal = $state(false);

  // Form input state
  let domain = $state('');
  let kind = $state<'proxy' | 'static' | 'php'>('proxy');
  let upstream = $state('http://127.0.0.1:3000');
  let rootPath = $state('/var/www/html');
  let sslMode = $state('none');
  let forceHTTPS = $state(true);
  let submitting = $state(false);

  async function loadSites() {
    loading = true;
    try {
      const data = await apiFetch<{ sites: Site[] }>('/api/sites');
      sites = data.sites || [];
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat situs');
    } finally {
      loading = false;
    }
  }

  async function handleCreateSite(e: Event) {
    e.preventDefault();
    if (!domain.trim()) return;

    submitting = true;
    try {
      await apiFetch('/api/sites', {
        method: 'POST',
        body: JSON.stringify({
          domain: domain.trim(),
          kind,
          upstream: kind === 'proxy' ? upstream.trim() : '',
          root_path: kind !== 'proxy' ? rootPath.trim() : '',
          ssl_mode: sslMode,
          force_https: forceHTTPS,
        }),
      });

      onNotify(`Situs ${domain} berhasil ditambahkan`);
      showModal = false;
      domain = '';
      await loadSites();
    } catch (err: any) {
      onNotify(err.message || 'Gagal menambahkan situs');
    } finally {
      submitting = false;
    }
  }

  async function toggleSite(id: number) {
    try {
      await apiFetch(`/api/sites/${id}/toggle`, { method: 'POST' });
      await loadSites();
      onNotify('Status situs diperbarui');
    } catch (err: any) {
      onNotify(err.message || 'Gagal mengubah status');
    }
  }

  async function deleteSite(id: number, siteDomain: string) {
    if (!confirm(`Hapus konfigurasi situs ${siteDomain}?`)) return;
    try {
      await apiFetch(`/api/sites/${id}`, { method: 'DELETE' });
      await loadSites();
      onNotify(`Situs ${siteDomain} dihapus`);
    } catch (err: any) {
      onNotify(err.message || 'Gagal menghapus situs');
    }
  }

  onMount(() => {
    loadSites();
  });
</script>

<div class="sites-container">
  <div class="head-bar">
    <div>
      <h2 class="title">Website / Virtual Hosts</h2>
      <p class="subtitle">Kelola reverse proxy, situs statis, PHP upstream, dan sertifikat SSL Nginx</p>
    </div>
    <button class="btn btn-primary" onclick={() => (showModal = true)}>
      + Tambah Situs
    </button>
  </div>

  <div class="panel">
    {#if loading}
      <div class="loading">Memuat daftar situs...</div>
    {:else if sites.length === 0}
      <div class="empty">
        <p>Belum ada situs yang ditambahkan.</p>
        <button class="btn btn-outline" onclick={() => (showModal = true)}>Tambah Situs Pertama</button>
      </div>
    {:else}
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Domain</th>
              <th>Jenis</th>
              <th>Target / Web Root</th>
              <th>SSL</th>
              <th>Status</th>
              <th style="text-align: right;">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#each sites as s}
              <tr>
                <td class="domain-col">
                  <span class="mono font-bold">{s.domain}</span>
                </td>
                <td>
                  <span class="type-pill">{s.kind}</span>
                </td>
                <td class="mono text-muted">{s.kind === 'proxy' ? s.upstream : s.root_path}</td>
                <td>
                  <span class="ssl-badge {s.ssl_mode !== 'none' ? 'ssl-on' : 'ssl-off'}">
                    {s.ssl_mode === 'none' ? 'Off' : s.ssl_mode}
                  </span>
                </td>
                <td>
                  <button
                    class="status-toggle {s.enabled === 1 ? 'is-active' : 'is-disabled'}"
                    onclick={() => toggleSite(s.id)}
                  >
                    {s.enabled === 1 ? 'Aktif' : 'Nonaktif'}
                  </button>
                </td>
                <td style="text-align: right;">
                  <button class="btn-sm btn-danger" onclick={() => deleteSite(s.id, s.domain)}>
                    Hapus
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>

  <!-- Modal Tambah Situs -->
  {#if showModal}
    <div class="modal-backdrop">
      <div class="modal">
        <div class="modal-head">
          <h3>Tambah Situs Web Baru</h3>
          <button class="btn-close" onclick={() => (showModal = false)}>&times;</button>
        </div>
        <form onsubmit={handleCreateSite}>
          <div class="field">
            <label for="f-domain">Nama Domain</label>
            <input
              id="f-domain"
              type="text"
              placeholder="contoh: api.fikri.dev atau app.local"
              bind:value={domain}
              required
            />
          </div>

          <div class="field">
            <label for="f-kind">Jenis Situs</label>
            <select id="f-kind" bind:value={kind}>
              <option value="proxy">Reverse Proxy (Node/Go/Python/Docker)</option>
              <option value="static">Static HTML/SPA (Build Dist)</option>
              <option value="php">PHP-FPM</option>
            </select>
          </div>

          {#if kind === 'proxy'}
            <div class="field">
              <label for="f-upstream">Upstream Target URL</label>
              <input id="f-upstream" type="text" bind:value={upstream} required />
              <small class="hint">Alamat lokal aplikasi, mis. http://127.0.0.1:3000</small>
            </div>
          {:else}
            <div class="field">
              <label for="f-root">Direktori Web Root</label>
              <input id="f-root" type="text" bind:value={rootPath} required />
            </div>
          {/if}

          <div class="field">
            <label for="f-ssl">Mode Sertifikat SSL</label>
            <select id="f-ssl" bind:value={sslMode}>
              <option value="none">Tanpa SSL (HTTP)</option>
              <option value="letsencrypt">Let's Encrypt (Otomatis HTTP-01)</option>
              <option value="custom">Sertifikat Kustom (PEM)</option>
            </select>
          </div>

          <div class="modal-actions">
            <button type="button" class="btn btn-ghost" onclick={() => (showModal = false)}>
              Batal
            </button>
            <button type="submit" class="btn btn-primary" disabled={submitting}>
              {submitting ? 'Menyimpan...' : 'Simpan Situs'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}
</div>

<style>
  .sites-container {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .head-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
  }
  .title {
    font-size: 1.15rem;
    font-weight: 700;
    color: #f1f5f9;
    letter-spacing: -0.01em;
  }
  .subtitle {
    font-size: 0.75rem;
    color: #8b95a5;
    margin-top: 0.15rem;
  }
  .panel {
    background: #10131b;
    border: 1px solid #1c2230;
    border-radius: 12px;
    padding: 1.15rem 1.35rem;
  }
  .loading,
  .empty {
    text-align: center;
    padding: 2.5rem 1rem;
    color: #8b95a5;
    font-size: 0.85rem;
  }
  .table-wrap {
    overflow-x: auto;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }
  th {
    text-align: left;
    padding: 0.65rem 0.75rem;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: #64748b;
    border-bottom: 1px solid #1c2230;
  }
  td {
    padding: 0.75rem;
    border-bottom: 1px solid #161924;
  }
  .mono {
    font-family: 'Fira Code', monospace;
  }
  .type-pill {
    font-size: 0.68rem;
    text-transform: uppercase;
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid rgba(255, 255, 255, 0.06);
    color: #cbd5e1;
    font-weight: 600;
  }
  .ssl-badge {
    font-size: 0.68rem;
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    font-weight: 600;
  }
  .ssl-on {
    background: rgba(16, 185, 129, 0.08);
    color: #10b981;
    border: 1px solid rgba(16, 185, 129, 0.2);
  }
  .ssl-off {
    background: rgba(148, 163, 184, 0.08);
    color: #8b95a5;
    border: 1px solid rgba(148, 163, 184, 0.15);
  }
  .status-toggle {
    cursor: pointer;
    border: none;
    padding: 0.2rem 0.55rem;
    border-radius: 4px;
    font-size: 0.72rem;
    font-weight: 600;
    transition: background 0.15s ease;
  }
  .is-active {
    background: rgba(16, 185, 129, 0.1);
    color: #10b981;
    border: 1px solid rgba(16, 185, 129, 0.25);
  }
  .is-disabled {
    background: rgba(148, 163, 184, 0.08);
    color: #8b95a5;
    border: 1px solid rgba(148, 163, 184, 0.15);
  }
  .btn {
    padding: 0.45rem 0.85rem;
    border-radius: 6px;
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
    transition: background 0.15s ease;
  }
  .btn-primary {
    background: #10b981;
    color: #05130b;
  }
  .btn-primary:hover {
    background: #059669;
  }
  .btn-sm {
    padding: 0.25rem 0.55rem;
    border-radius: 4px;
    font-size: 0.72rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
  }
  .btn-danger {
    background: rgba(244, 63, 94, 0.1);
    color: #f43f5e;
    border: 1px solid rgba(244, 63, 94, 0.25);
  }
  .btn-danger:hover {
    background: rgba(244, 63, 94, 0.2);
  }
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.75);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
    padding: 1rem;
  }
  .modal {
    background: #0d0f17;
    border: 1px solid #1c2230;
    border-radius: 12px;
    width: 100%;
    max-width: 460px;
    padding: 1.5rem;
    box-shadow: 0 16px 36px rgba(0, 0, 0, 0.6);
  }
  .modal-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.25rem;
  }
  .modal-head h3 {
    font-size: 1rem;
    color: #f1f5f9;
  }
  .btn-close {
    background: transparent;
    border: none;
    color: #8b95a5;
    font-size: 1.25rem;
    cursor: pointer;
  }
  .field {
    margin-bottom: 0.95rem;
  }
  label {
    display: block;
    font-size: 0.75rem;
    font-weight: 500;
    color: #cbd5e1;
    margin-bottom: 0.35rem;
  }
  input,
  select {
    width: 100%;
    padding: 0.55rem 0.75rem;
    background: #090b10;
    border: 1px solid #1c2230;
    border-radius: 6px;
    color: #f1f5f9;
    font-size: 0.82rem;
    outline: none;
    transition: border-color 0.15s ease;
  }
  input:focus,
  select:focus {
    border-color: #38bdf8;
  }
  .hint {
    display: block;
    font-size: 0.68rem;
    color: #8b95a5;
    margin-top: 0.25rem;
  }
  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.65rem;
    margin-top: 1.35rem;
  }
  .btn-ghost {
    background: #131622;
    border: 1px solid #1e2434;
    color: #cbd5e1;
  }
  .btn-ghost:hover {
    background: #181d2a;
    color: #f1f5f9;
  }
</style>