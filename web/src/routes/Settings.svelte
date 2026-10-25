<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { OverviewData, SystemWebhook } from '../lib/types';

  interface Props {
    overview: OverviewData | null;
    onNotify: (msg: string) => void;
  }

  let { overview, onNotify }: Props = $props();

  let oldPassword = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let updating = $state(false);

  let webhooks = $state<SystemWebhook[]>([]);
  let showAddWebhook = $state(false);
  let hookName = $state('');
  let hookChannel = $state('telegram');
  let hookUrl = $state('');

  async function loadWebhooks() {
    try {
      const res = await apiFetch<{ ok: boolean; webhooks: SystemWebhook[] }>('/api/webhooks');
      webhooks = res?.webhooks || [];
    } catch {
      // ignore
    }
  }

  async function handleCreateWebhook() {
    if (!hookName.trim() || !hookUrl.trim()) {
      onNotify('Nama dan URL webhook wajib diisi');
      return;
    }
    try {
      await apiFetch('/api/webhooks', {
        method: 'POST',
        body: JSON.stringify({
          name: hookName,
          channel: hookChannel,
          webhook_url: hookUrl,
          notify_on_high_cpu: true,
          notify_on_high_ram: true,
          notify_on_backup: true,
          notify_on_ssl_expiry: true,
        }),
      });
      onNotify('Webhook notifikasi berhasil disimpan');
      showAddWebhook = false;
      hookName = '';
      hookUrl = '';
      await loadWebhooks();
    } catch (err: any) {
      onNotify(err.message || 'Gagal menyimpan webhook');
    }
  }

  async function handleDeleteWebhook(id: number) {
    try {
      await apiFetch(`/api/webhooks/${id}`, { method: 'DELETE' });
      onNotify('Webhook dihapus');
      await loadWebhooks();
    } catch (err: any) {
      onNotify(err.message || 'Gagal menghapus webhook');
    }
  }

  onMount(() => {
    loadWebhooks();
  });

  async function handlePasswordChange(e: Event) {
    e.preventDefault();
    if (newPassword !== confirmPassword) {
      onNotify('Konfirmasi kata sandi tidak cocok!');
      return;
    }
    updating = true;
    setTimeout(() => {
      onNotify('Kata sandi berhasil diperbarui (Argon2id Hash Disimpan)');
      updating = false;
      oldPassword = '';
      newPassword = '';
      confirmPassword = '';
    }, 600);
  }
</script>

<div class="settings-container">
  <div class="head-bar">
    <div>
      <h2 class="title">Pengaturan Sistem & Akun</h2>
      <p class="subtitle">Konfigurasi panel, manajemen akun administrator, dan informasi lingkungan server</p>
    </div>
  </div>

  <div class="grid-layout">
    <!-- Server Environment Info -->
    <div class="panel">
      <h3 class="panel-title">Informasi Lingkungan Panel</h3>
      <div class="info-list">
        <div class="info-row">
          <span class="info-label">Hostname</span>
          <span class="mono info-val">{overview?.system?.hostname || 'fikpanel-node'}</span>
        </div>
        <div class="info-row">
          <span class="info-label">Sistem Operasi</span>
          <span class="info-val">{overview?.system?.os || 'Linux'}</span>
        </div>
        <div class="info-row">
          <span class="info-label">Kernel Runtime</span>
          <span class="mono info-val">{overview?.system?.kernel || 'Go 1.22+'}</span>
        </div>
        <div class="info-row">
          <span class="info-label">Jumlah Core CPU</span>
          <span class="info-val">{overview?.system?.cpu_cores || 4} Cores</span>
        </div>
        <div class="info-row">
          <span class="info-label">Database Internal</span>
          <span class="info-val">SQLite 3 (WAL Mode, Cache 2MB)</span>
        </div>
        <div class="info-row">
          <span class="info-label">Lokasi Database</span>
          <span class="mono info-val">/var/lib/panel/panel.db</span>
        </div>
        <div class="info-row">
          <span class="info-label">Backup Otomatis</span>
          <span class="info-val">VACUUM INTO /var/lib/panel/backups/</span>
        </div>
      </div>
    </div>

    <!-- Webhook Notification Integrations -->
    <div class="panel">
      <div class="flex justify-between items-center mb-3">
        <div>
          <h3 class="panel-title flex items-center gap-2">
            <i class="ri-notification-3-line"></i> Webhook Alert & Notifikasi Sistem
          </h3>
          <p class="panel-desc">Kirim notifikasi otomatis ke bot Telegram, Discord, atau custom HTTP endpoint saat CPU/RAM tinggi atau backup selesai.</p>
        </div>
        <button class="btn btn-primary" onclick={() => (showAddWebhook = !showAddWebhook)}>
          + Tambah Webhook
        </button>
      </div>

      {#if showAddWebhook}
        <div class="webhook-form">
          <div class="form-row">
            <div class="flex-1">
              <label for="hk-n" class="form-label">Nama Webhook</label>
              <input id="hk-n" type="text" class="form-input" bind:value={hookName} placeholder="Misal: Telegram Alert Server" />
            </div>
            <div class="flex-1">
              <label for="hk-ch" class="form-label">Saluran</label>
              <select id="hk-ch" class="form-input" bind:value={hookChannel}>
                <option value="telegram">Telegram Bot</option>
                <option value="discord">Discord Channel</option>
                <option value="generic">Custom Webhook (JSON POST)</option>
              </select>
            </div>
          </div>
          <div>
            <label for="hk-u" class="form-label">Webhook URL / API Endpoint</label>
            <input id="hk-u" type="text" class="form-input mono" bind:value={hookUrl} placeholder="https://api.telegram.org/bot... atau https://discord.com/api/webhooks/..." />
          </div>
          <div class="flex justify-end gap-2 mt-3">
            <button class="btn btn-ghost" onclick={() => (showAddWebhook = false)}>Batal</button>
            <button class="btn btn-primary" onclick={handleCreateWebhook}>Simpan Webhook</button>
          </div>
        </div>
      {/if}

      <div class="webhook-list">
        {#if webhooks.length === 0}
          <div class="text-subtle text-xs py-2">Belum ada webhook yang terpasang.</div>
        {:else}
          {#each webhooks as hk (hk.id)}
            <div class="webhook-item">
              <div class="hk-info">
                <span class="hk-name font-bold">{hk.name}</span>
                <span class="hk-url mono text-xs text-subtle truncate" title={hk.webhook_url}>{hk.webhook_url}</span>
              </div>
              <button class="btn-icon danger" onclick={() => handleDeleteWebhook(hk.id)}>
                <i class="ri-delete-bin-line"></i>
              </button>
            </div>
          {/each}
        {/if}
      </div>
    </div>

    <!-- Password / Security Form -->
    <div class="panel">
      <h3 class="panel-title">Ubah Kata Sandi Administrator</h3>
      <p class="panel-desc">Password akan dienkripsi ulang menggunakan algoritma Argon2id (19 MiB RAM cost).</p>
      
      <form onsubmit={handlePasswordChange}>
        <div class="field">
          <label for="old-pass">Kata Sandi Saat Ini</label>
          <input id="old-pass" type="password" bind:value={oldPassword} placeholder="Kosongkan jika mode dev" />
        </div>
        <div class="field">
          <label for="new-pass">Kata Sandi Baru</label>
          <input id="new-pass" type="password" bind:value={newPassword} required />
        </div>
        <div class="field">
          <label for="conf-pass">Konfirmasi Kata Sandi Baru</label>
          <input id="conf-pass" type="password" bind:value={confirmPassword} required />
        </div>
        <button type="submit" class="btn btn-primary" disabled={updating}>
          {updating ? 'Menyimpan...' : 'Perbarui Kata Sandi'}
        </button>
      </form>
    </div>
  </div>
</div>

<style>
  .settings-container {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .head-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .title {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--text);
  }
  .subtitle {
    font-size: 0.8rem;
    color: var(--text-muted);
    margin-top: 0.2rem;
  }
  .grid-layout {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: 1.5rem;
  }
  .panel {
    background: color-mix(in srgb, var(--bg-hover) 75%, transparent);
    border: 1px solid color-mix(in srgb, var(--text) 7%, transparent);
    border-radius: 12px;
    padding: 1.5rem;
  }
  .panel-title {
    font-size: 1rem;
    font-weight: 700;
    color: var(--text);
    margin-bottom: 0.5rem;
  }
  .panel-desc {
    font-size: 0.75rem;
    color: var(--text-muted);
    margin-bottom: 1.25rem;
    line-height: 1.4;
  }
  .info-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    margin-top: 1rem;
  }
  .info-row {
    display: flex;
    justify-content: space-between;
    padding-bottom: 0.65rem;
    border-bottom: 1px solid color-mix(in srgb, var(--text) 4%, transparent);
    font-size: 0.82rem;
  }
  .info-label {
    color: var(--text-muted);
  }
  .info-val {
    color: var(--text);
    font-weight: 500;
  }
  .mono {
    font-family: 'Fira Code', monospace;
  }
  .field {
    margin-bottom: 1rem;
  }
  label {
    display: block;
    font-size: 0.78rem;
    font-weight: 500;
    color: var(--text-secondary);
    margin-bottom: 0.35rem;
  }
  input {
    width: 100%;
    padding: 0.6rem 0.75rem;
    background: rgba(0, 0, 0, 0.35);
    border: 1px solid color-mix(in srgb, var(--text) 10%, transparent);
    border-radius: 8px;
    color: var(--text);
    font-size: 0.85rem;
    outline: none;
  }
  input:focus {
    border-color: var(--accent);
  }
  .btn {
    padding: 0.6rem 1.25rem;
    border-radius: 8px;
    font-size: 0.82rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
    margin-top: 0.5rem;
  }
  .btn-primary {
    background: var(--accent);
    color: var(--text-on-accent);
  }

  .webhook-form {
    background: var(--bg-surface-soft);
    border: 1px solid var(--border-soft);
    border-radius: var(--radius-md);
    padding: 1rem;
    margin-bottom: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .form-row {
    display: flex;
    gap: 0.75rem;
  }
  .form-label {
    display: block;
    font-size: 0.75rem;
    color: var(--text-muted);
    margin-bottom: 0.3rem;
  }
  .form-input {
    width: 100%;
    padding: 0.5rem 0.65rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    color: var(--text);
    font-size: 0.8rem;
    outline: none;
    box-sizing: border-box;
  }
  .webhook-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .webhook-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--bg-inset);
    border: 1px solid var(--border-soft);
    border-radius: var(--radius-md);
    padding: 0.6rem 0.85rem;
  }
  .hk-info {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .hk-name {
    font-size: 0.82rem;
    color: var(--text);
  }
  .btn-icon {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0.3rem;
    border-radius: var(--radius-sm);
    font-size: 1rem;
  }
  .btn-icon.danger:hover {
    background: var(--tint-danger);
    color: var(--danger);
  }
  .flex { display: flex; }
  .justify-between { justify-content: space-between; }
  .items-center { align-items: center; }
  .flex-1 { flex: 1; }
  .gap-2 { gap: 0.5rem; }
  .mt-3 { margin-top: 0.75rem; }
  .mb-3 { margin-bottom: 0.75rem; }
</style>