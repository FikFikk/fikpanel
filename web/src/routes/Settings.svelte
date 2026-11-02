<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { OverviewData, SystemWebhook } from '../lib/types';
  import '../styles/pages/settings.css';

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
      // Abaikan kesalahan pemuatan awal webhook
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
    if (newPassword.length < 4) {
      onNotify('Kata sandi baru minimal 4 karakter!');
      return;
    }
    updating = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/auth/password', {
        method: 'POST',
        body: JSON.stringify({
          old_password: oldPassword,
          new_password: newPassword,
        }),
      });
      onNotify(res.message || 'Kata sandi berhasil diperbarui (Argon2id tersimpan)');
      oldPassword = '';
      newPassword = '';
      confirmPassword = '';
    } catch (err: any) {
      onNotify(err.message || 'Gagal memperbarui kata sandi');
    } finally {
      updating = false;
    }
  }
</script>

<div class="pg-settings settings-container">
  <div class="page-header">
    <div class="page-header__main">
      <h2 class="page-header__title">Pengaturan Sistem & Akun</h2>
      <p class="page-header__sub">Konfigurasi panel, manajemen akun administrator, dan informasi lingkungan server</p>
    </div>
  </div>

  <div class="grid-layout">
    <!-- Informasi lingkungan server -->
    <div class="card">
      <div class="card__head">
        <h3 class="card__title">
          <i class="ri-server-line" aria-hidden="true"></i> Informasi Lingkungan Panel
        </h3>
      </div>
      <div class="card__body">
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
    </div>

    <!-- Integrasi notifikasi webhook -->
    <div class="card">
      <div class="card__head">
        <h3 class="card__title">
          <i class="ri-notification-3-line" aria-hidden="true"></i> Webhook Alert & Notifikasi Sistem
        </h3>
        <button class="btn btn--primary btn--sm" onclick={() => (showAddWebhook = !showAddWebhook)}>
          + Tambah Webhook
        </button>
      </div>
      <div class="card__body">
        <p class="panel-desc">Kirim notifikasi otomatis ke bot Telegram, Discord, atau custom HTTP endpoint saat CPU/RAM tinggi atau backup selesai.</p>

        {#if showAddWebhook}
          <div class="webhook-form">
            <div class="form-row">
              <div class="field grow">
                <label for="hk-n" class="field__label">Nama Webhook</label>
                <input id="hk-n" type="text" class="input" bind:value={hookName} placeholder="Misal: Telegram Alert Server" />
              </div>
              <div class="field grow">
                <label for="hk-ch" class="field__label">Saluran</label>
                <select id="hk-ch" class="select" bind:value={hookChannel}>
                  <option value="telegram">Telegram Bot</option>
                  <option value="discord">Discord Channel</option>
                  <option value="generic">Custom Webhook (JSON POST)</option>
                </select>
              </div>
            </div>
            <div class="field">
              <label for="hk-u" class="field__label">Webhook URL / API Endpoint</label>
              <input id="hk-u" type="text" class="input mono" bind:value={hookUrl} placeholder="https://api.telegram.org/bot... atau https://discord.com/api/webhooks/..." />
            </div>
            <div class="flex justify-between items-center gap-2 mt-2">
              <span></span>
              <div class="flex gap-2">
                <button class="btn btn--ghost" onclick={() => (showAddWebhook = false)}>Batal</button>
                <button class="btn btn--primary" onclick={handleCreateWebhook}>Simpan Webhook</button>
              </div>
            </div>
          </div>
        {/if}

        <div class="webhook-list">
          {#if webhooks.length === 0}
            <div class="text-subtle">Belum ada webhook yang terpasang.</div>
          {:else}
            {#each webhooks as hk (hk.id)}
              <div class="webhook-item">
                <div class="hk-info">
                  <span class="hk-name font-bold">{hk.name}</span>
                  <span class="hk-url mono text-subtle truncate" title={hk.webhook_url}>{hk.webhook_url}</span>
                </div>
                <button class="btn btn--icon btn--sm btn--danger" aria-label="Hapus webhook {hk.name}" onclick={() => handleDeleteWebhook(hk.id)}>
                  <i class="ri-delete-bin-line"></i>
                </button>
              </div>
            {/each}
          {/if}
        </div>
      </div>
    </div>

    <!-- Form ubah kata sandi -->
    <div class="card">
      <div class="card__head">
        <h3 class="card__title">
          <i class="ri-lock-password-line" aria-hidden="true"></i> Ubah Kata Sandi Administrator
        </h3>
      </div>
      <div class="card__body">
        <p class="panel-desc">Password akan dienkripsi ulang menggunakan algoritma Argon2id (19 MiB RAM cost).</p>

        <form class="pass-form" onsubmit={handlePasswordChange}>
          <div class="field">
            <label for="old-pass" class="field__label">Kata Sandi Saat Ini</label>
            <input id="old-pass" type="password" class="input" bind:value={oldPassword} placeholder="Kosongkan jika mode dev / saat ini tanpa sandi" autocomplete="current-password" />
          </div>
          <div class="field">
            <label for="new-pass" class="field__label">Kata Sandi Baru</label>
            <input id="new-pass" type="password" class="input" bind:value={newPassword} required minlength="4" autocomplete="new-password" />
          </div>
          <div class="field">
            <label for="conf-pass" class="field__label">Konfirmasi Kata Sandi Baru</label>
            <input id="conf-pass" type="password" class="input" bind:value={confirmPassword} required minlength="4" autocomplete="new-password" />
          </div>
          <div>
            <button type="submit" class="btn btn--primary" disabled={updating}>
              {updating ? 'Menyimpan...' : 'Perbarui Kata Sandi'}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</div>
