<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../api';
  import { confirmDialog } from '../stores/confirm.svelte';
  import type { AIProvider, AIProviderKind } from '../types';
  import Modal from './ui/Modal.svelte';
  import '../../styles/pages/components/ai-provider-modal.css';

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

  onMount(() => {
    loadProviders();
  });
</script>

<Modal
  open={true}
  title="Pengaturan Provider AI"
  subtitle="Konfigurasi Omniroute, 9router, Claude, Gemini, OpenAI, atau Ollama lokal"
  size="xl"
  onclose={onClose}
>
    <!-- Isi Modal: Dua Kolom (Daftar Provider + Form Konfigurasi) -->
    <div class="cmp-ai-provider modal-grid">
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
</Modal>
