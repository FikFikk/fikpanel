<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../api';
  import { confirmDialog } from '../stores/confirm.svelte';
  import type { AIProvider, AIProviderKind } from '../types';

  interface Props {
    onClose: () => void;
    onNotify: (msg: string) => void;
    onProviderChange?: () => void;
  }

  let { onClose, onNotify, onProviderChange }: Props = $props();

  let providers = $state<AIProvider[]>([]);
  let loading = $state(true);
  let testing = $state(false);
  let saving = $state(false);
  let testResult = $state<{ ok: boolean; latency?: number; message: string } | null>(null);

  // State form input
  let editingId = $state<number | null>(null);
  let formName = $state('');
  let formKind = $state<AIProviderKind>('openai_compat');
  let formBaseURL = $state('https://api.omniroute.ai/v1');
  let formModel = $state('gpt-4o');
  let formAPIKey = $state('');
  let formIsDefault = $state(false);
  let showAPIKey = $state(false);

  // Template preset populer
  const presets = [
    {
      label: 'Omniroute',
      kind: 'openai_compat' as AIProviderKind,
      baseUrl: 'https://api.omniroute.ai/v1',
      model: 'gpt-4o',
    },
    {
      label: '9router',
      kind: 'openai_compat' as AIProviderKind,
      baseUrl: 'https://api.9router.com/v1',
      model: 'gpt-4o',
    },
    {
      label: 'OpenAI',
      kind: 'openai_compat' as AIProviderKind,
      baseUrl: 'https://api.openai.com/v1',
      model: 'gpt-4o',
    },
    {
      label: 'Claude (Anthropic)',
      kind: 'anthropic' as AIProviderKind,
      baseUrl: 'https://api.anthropic.com/v1',
      model: 'claude-3-5-sonnet-20241022',
    },
    {
      label: 'Google Gemini',
      kind: 'gemini' as AIProviderKind,
      baseUrl: 'https://generativelanguage.googleapis.com/v1beta/openai/',
      model: 'gemini-1.5-flash',
    },
    {
      label: 'Ollama (Lokal)',
      kind: 'openai_compat' as AIProviderKind,
      baseUrl: 'http://localhost:11434/v1',
      model: 'llama3',
    },
  ];

  function applyPreset(p: typeof presets[0]) {
    formName = p.label;
    formKind = p.kind;
    formBaseURL = p.baseUrl;
    formModel = p.model;
    testResult = null;
  }

  async function loadProviders() {
    loading = true;
    try {
      const res = await apiFetch<{ ok: boolean; providers: AIProvider[] }>('/api/ai/providers');
      providers = res.providers || [];
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat daftar provider');
    } finally {
      loading = false;
    }
  }

  function startEdit(p: AIProvider) {
    editingId = p.id;
    formName = p.name;
    formKind = p.kind;
    formBaseURL = p.base_url;
    formModel = p.model;
    formAPIKey = ''; // Kosongkan agar tidak mengekspos kunci asli
    formIsDefault = p.is_default;
    testResult = null;
  }

  function resetForm() {
    editingId = null;
    formName = '';
    formKind = 'openai_compat';
    formBaseURL = 'https://api.omniroute.ai/v1';
    formModel = 'gpt-4o';
    formAPIKey = '';
    formIsDefault = providers.length === 0;
    testResult = null;
  }

  async function testConnection() {
    if (!formBaseURL.trim() || !formModel.trim()) {
      onNotify('Base URL dan Model wajib diisi untuk uji koneksi');
      return;
    }

    testing = true;
    testResult = null;
    try {
      const res = await apiFetch<{ ok: boolean; latency_ms: number; reply: string }>(
        '/api/ai/providers/test',
        {
          method: 'POST',
          body: JSON.stringify({
            id: editingId,
            kind: formKind,
            base_url: formBaseURL.trim(),
            model: formModel.trim(),
            api_key: formAPIKey.trim(),
          }),
        }
      );
      testResult = {
        ok: true,
        latency: res.latency_ms,
        message: `Koneksi Berhasil (${res.latency_ms} ms) · Respon: "${res.reply}"`,
      };
      onNotify(`Uji koneksi ke ${formName || formModel} berhasil!`);
    } catch (err: any) {
      testResult = {
        ok: false,
        message: err.message || 'Gagal terhubung ke provider',
      };
      onNotify(`Uji koneksi gagal: ${err.message || 'Koneksi ditolak'}`);
    } finally {
      testing = false;
    }
  }

  async function saveProvider(e: Event) {
    e.preventDefault();
    if (!formName.trim() || !formBaseURL.trim() || !formModel.trim()) {
      onNotify('Nama, Base URL, dan Model tidak boleh kosong');
      return;
    }

    saving = true;
    try {
      if (editingId) {
        await apiFetch(`/api/ai/providers/${editingId}`, {
          method: 'PUT',
          body: JSON.stringify({
            name: formName.trim(),
            kind: formKind,
            base_url: formBaseURL.trim(),
            model: formModel.trim(),
            api_key: formAPIKey.trim(),
            is_default: formIsDefault,
          }),
        });
        onNotify(`Provider ${formName} berhasil diperbarui`);
      } else {
        await apiFetch('/api/ai/providers', {
          method: 'POST',
          body: JSON.stringify({
            name: formName.trim(),
            kind: formKind,
            base_url: formBaseURL.trim(),
            model: formModel.trim(),
            api_key: formAPIKey.trim(),
            is_default: formIsDefault,
          }),
        });
        onNotify(`Provider ${formName} berhasil ditambahkan`);
      }

      resetForm();
      await loadProviders();
      onProviderChange?.();
    } catch (err: any) {
      onNotify(err.message || 'Gagal menyimpan provider');
    } finally {
      saving = false;
    }
  }

  async function setDefault(p: AIProvider) {
    try {
      await apiFetch(`/api/ai/providers/${p.id}/default`, { method: 'POST' });
      onNotify(`${p.name} dijadikan provider aktif default`);
      await loadProviders();
      onProviderChange?.();
    } catch (err: any) {
      onNotify(err.message || 'Gagal mengubah provider default');
    }
  }

  async function deleteProvider(p: AIProvider) {
    const ok = await confirmDialog({
      title: `Hapus Provider "${p.name}"?`,
      message: 'Konfigurasi provider ini akan dihapus dari panel. Jika ini adalah provider default, sistem akan otomatis beralih.',
      confirmText: 'Hapus Provider',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/ai/providers/${p.id}`, { method: 'DELETE' });
      onNotify(`Provider ${p.name} dihapus`);
      if (editingId === p.id) resetForm();
      await loadProviders();
      onProviderChange?.();
    } catch (err: any) {
      onNotify(err.message || 'Gagal menghapus provider');
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      onClose();
    }
  }

  onMount(() => {
    loadProviders();
  });
</script>

<svelte:window onkeydown={handleKeydown} />

<div
  class="modal-backdrop"
  role="presentation"
  onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}
>
  <div class="modal-card" role="dialog" aria-modal="true" aria-labelledby="provider-modal-title">
    <!-- Header Modal -->
    <div class="modal-head">
      <div class="head-left">
        <span class="head-icon" aria-hidden="true"><i class="ri-settings-3-line"></i></span>
        <div>
          <h2 id="provider-modal-title">Pengaturan Provider AI</h2>
          <p class="head-sub">Konfigurasi Omniroute, 9router, Claude, Gemini, OpenAI, atau Ollama lokal</p>
        </div>
      </div>
      <button class="btn btn--icon btn--ghost" onclick={onClose} aria-label="Tutup modal" title="Tutup">
        <i class="ri-close-line"></i>
      </button>
    </div>

    <!-- Isi Modal: Dua Kolom (Daftar Provider + Form Konfigurasi) -->
    <div class="modal-grid">
      <!-- Kolom Kiri: Daftar Provider -->
      <div class="provider-list-col">
        <div class="col-header">
          <span class="col-title">Provider Terdaftar ({providers.length})</span>
          <button class="btn btn--sm btn--secondary" onclick={resetForm}>
            <i class="ri-add-line"></i> Tambah Baru
          </button>
        </div>

        {#if loading}
          <div class="loading-state">
            <span class="spinner"></span>
            <span>Memuat data provider...</span>
          </div>
        {:else if providers.length === 0}
          <div class="empty-state">
            <i class="ri-robot-2-line empty-icon"></i>
            <div class="empty-title">Belum Ada Provider</div>
            <div class="empty-desc">Sistem saat ini berjalan dalam mode offline rule-based. Tambahkan provider pertama Anda di formulir samping.</div>
          </div>
        {:else}
          <div class="provider-cards">
            {#each providers as p}
              <div class="p-card" class:p-active={p.is_default} class:p-editing={editingId === p.id}>
                <div class="p-head">
                  <div class="p-info">
                    <span class="p-name">{p.name}</span>
                    <span class="p-model mono">{p.model}</span>
                  </div>
                  {#if p.is_default}
                    <span class="badge badge--success badge--pill"><i class="ri-checkbox-circle-line"></i> Aktif</span>
                  {/if}
                </div>
                <div class="p-url mono truncate" title={p.base_url}>
                  <i class="ri-links-line"></i> {p.base_url}
                </div>
                <div class="p-actions">
                  {#if !p.is_default}
                    <button class="btn btn--sm btn--ghost" onclick={() => setDefault(p)} title="Jadikan aktif">
                      Set Default
                    </button>
                  {/if}
                  <button class="btn btn--sm btn--secondary" onclick={() => startEdit(p)} title="Edit konfigurasi">
                    <i class="ri-edit-line"></i> Edit
                  </button>
                  <button class="btn btn--sm btn--danger btn--icon" onclick={() => deleteProvider(p)} aria-label="Hapus provider" title="Hapus">
                    <i class="ri-delete-bin-line"></i>
                  </button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <!-- Kolom Kanan: Formulir Tambah / Edit -->
      <div class="form-col">
        <div class="col-header">
          <span class="col-title">{editingId ? 'Edit Provider' : 'Tambah Provider Kustom'}</span>
          {#if editingId}
            <button class="btn btn--sm btn--ghost" onclick={resetForm}>Batal</button>
          {/if}
        </div>

        <!-- Tombol Preset Cepat -->
        <div class="preset-box">
          <span class="preset-label">Template Preset Cepat:</span>
          <div class="preset-chips">
            {#each presets as ps}
              <button type="button" class="preset-chip" onclick={() => applyPreset(ps)}>
                {ps.label}
              </button>
            {/each}
          </div>
        </div>

        <form onsubmit={saveProvider} class="p-form">
          <label class="field">
            <span class="field__label">Nama Provider</span>
            <input
              type="text"
              class="input"
              placeholder="Contoh: Omniroute Utama, 9router Fast, Claude 3.5"
              bind:value={formName}
              required
            />
          </label>

          <div class="form-row">
            <label class="field field--grow">
              <span class="field__label">Protokol AI</span>
              <select class="input" bind:value={formKind}>
                <option value="openai_compat">OpenAI-Compatible (Omniroute, 9router, OpenAI, Ollama)</option>
                <option value="anthropic">Anthropic Claude Native (/v1/messages)</option>
                <option value="gemini">Google Gemini (OpenAI/Native)</option>
              </select>
            </label>

            <label class="field field--grow">
              <span class="field__label">Nama Model</span>
              <input
                type="text"
                class="input input--mono"
                placeholder="mis. gpt-4o, claude-3-5-sonnet, llama3"
                bind:value={formModel}
                required
              />
            </label>
          </div>

          <label class="field">
            <span class="field__label">Base URL Endpoint</span>
            <input
              type="url"
              class="input input--mono"
              placeholder="https://api.omniroute.ai/v1"
              bind:value={formBaseURL}
              required
            />
            <span class="field__hint">
              Untuk Omniroute/9router, gunakan endpoint gateway v1 Anda. Untuk Ollama lokal: <code>http://localhost:11434/v1</code>.
            </span>
          </label>

          <label class="field">
            <div class="field-label-row">
              <span class="field__label">API Key</span>
              <button
                type="button"
                class="btn-text"
                onclick={() => (showAPIKey = !showAPIKey)}
              >
                <i class={showAPIKey ? 'ri-eye-off-line' : 'ri-eye-line'}></i>
                {showAPIKey ? 'Sembunyikan' : 'Perlihatkan'}
              </button>
            </div>
            <div class="input-wrap">
              <input
                type={showAPIKey ? 'text' : 'password'}
                class="input input--mono"
                placeholder={editingId ? 'Kosongkan jika tidak ingin mengubah kunci tersimpan' : 'sk-...'}
                bind:value={formAPIKey}
                autocomplete="off"
                spellcheck="false"
              />
            </div>
            <span class="field__hint">Kunci disimpan aman di SQLite lokal dan disensor otomatis pada audit log.</span>
          </label>

          <label class="switch-row">
            <input type="checkbox" bind:checked={formIsDefault} />
            <span class="switch-text">Jadikan provider aktif default</span>
          </label>

          {#if testResult}
            <div class="alert {testResult.ok ? 'alert--success' : 'alert--danger'}">
              <i class={testResult.ok ? 'ri-checkbox-circle-line' : 'ri-error-warning-line'}></i>
              <div class="alert-content">
                <span class="alert-text">{testResult.message}</span>
              </div>
            </div>
          {/if}

          <div class="form-foot">
            <button
              type="button"
              class="btn btn--secondary"
              onclick={testConnection}
              disabled={testing || saving || !formBaseURL.trim() || !formModel.trim()}
            >
              {#if testing}
                <span class="spinner"></span> Menguji...
              {:else}
                <i class="ri-flashlight-line"></i> Uji Koneksi
              {/if}
            </button>

            <button
              type="submit"
              class="btn btn--primary"
              disabled={saving || testing}
            >
              {#if saving}
                <span class="spinner"></span> Menyimpan...
              {:else}
                <i class="ri-save-line"></i> {editingId ? 'Perbarui Provider' : 'Simpan Provider'}
              {/if}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    z-index: var(--z-modal);
    display: grid;
    place-items: center;
    padding: var(--space-4);
    background: var(--bg-overlay);
    backdrop-filter: blur(4px);
    animation: fp-fade var(--duration-base) ease-out;
  }

  .modal-card {
    width: min(960px, 100%);
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    box-shadow: var(--shadow-lg);
    animation: fp-pop var(--duration-slow) var(--ease-spring);
    overflow: hidden;
  }

  .modal-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4) var(--space-5);
    background: var(--bg-raised);
    border-bottom: 1px solid var(--border-subtle);
  }

  .head-left {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .head-icon {
    display: grid;
    place-items: center;
    width: 38px;
    height: 38px;
    border-radius: var(--radius-md);
    background: var(--tint-violet);
    color: var(--violet);
    font-size: 1.25rem;
  }

  .modal-head h2 {
    font-size: var(--text-lg);
    font-weight: var(--weight-bold);
    color: var(--text);
  }

  .head-sub {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .modal-grid {
    display: grid;
    grid-template-columns: 340px 1fr;
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  .provider-list-col {
    display: flex;
    flex-direction: column;
    padding: var(--space-4);
    background: var(--bg-inset);
    border-right: 1px solid var(--border-subtle);
    overflow-y: auto;
  }

  .form-col {
    display: flex;
    flex-direction: column;
    padding: var(--space-5);
    overflow-y: auto;
    background: var(--bg-surface);
  }

  .col-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-3);
  }

  .col-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .provider-cards {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .p-card {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    transition: all var(--duration-fast);
  }

  .p-card.p-active {
    border-color: var(--accent);
    background: color-mix(in srgb, var(--accent) 4%, var(--bg-surface));
  }

  .p-card.p-editing {
    border-color: var(--violet);
  }

  .p-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .p-info {
    display: flex;
    flex-direction: column;
  }

  .p-name {
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--text);
  }

  .p-model {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .p-url {
    font-size: var(--text-2xs);
    color: var(--text-subtle);
  }

  .p-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-1);
    margin-top: var(--space-1);
    padding-top: var(--space-2);
    border-top: 1px solid var(--border-subtle);
  }

  .preset-box {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3);
    background: var(--bg-raised);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    margin-bottom: var(--space-4);
  }

  .preset-label {
    font-size: var(--text-xs);
    color: var(--text-muted);
    font-weight: var(--weight-medium);
  }

  .preset-chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .preset-chip {
    padding: var(--space-1) var(--space-2);
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    color: var(--text-secondary);
    cursor: pointer;
    transition: all var(--duration-fast);
  }

  .preset-chip:hover {
    color: var(--text);
    border-color: var(--accent);
    background: var(--tint-accent);
  }

  .p-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .form-row {
    display: flex;
    gap: var(--space-3);
  }

  .field--grow {
    flex: 1;
    min-width: 0;
  }

  .field-label-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .btn-text {
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: var(--text-xs);
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 0;
  }

  .btn-text:hover {
    color: var(--text);
  }

  .switch-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    cursor: pointer;
    font-size: var(--text-sm);
    color: var(--text-secondary);
  }

  .form-foot {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
    margin-top: var(--space-2);
    padding-top: var(--space-4);
    border-top: 1px solid var(--border-subtle);
  }

  .loading-state {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-muted);
    padding: var(--space-4);
  }

  .empty-icon {
    font-size: 2rem;
    color: var(--text-subtle);
    margin-bottom: var(--space-1);
  }

  @media (max-width: 768px) {
    .modal-grid {
      grid-template-columns: 1fr;
    }
    .provider-list-col {
      max-height: 200px;
      border-right: none;
      border-bottom: 1px solid var(--border-subtle);
    }
    .form-row {
      flex-direction: column;
    }
  }
</style>
