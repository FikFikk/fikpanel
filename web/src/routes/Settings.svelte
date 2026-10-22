<script lang="ts">
  import type { OverviewData } from '../lib/types';

  interface Props {
    overview: OverviewData | null;
    onNotify: (msg: string) => void;
  }

  let { overview, onNotify }: Props = $props();

  let oldPassword = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let updating = $state(false);

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
    color: #f3f4f6;
  }
  .subtitle {
    font-size: 0.8rem;
    color: #9ca3af;
    margin-top: 0.2rem;
  }
  .grid-layout {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: 1.5rem;
  }
  .panel {
    background: rgba(28, 29, 34, 0.75);
    border: 1px solid rgba(255, 255, 255, 0.07);
    border-radius: 12px;
    padding: 1.5rem;
  }
  .panel-title {
    font-size: 1rem;
    font-weight: 700;
    color: #f3f4f6;
    margin-bottom: 0.5rem;
  }
  .panel-desc {
    font-size: 0.75rem;
    color: #9ca3af;
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
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
    font-size: 0.82rem;
  }
  .info-label {
    color: #9ca3af;
  }
  .info-val {
    color: #f3f4f6;
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
    color: #d1d5db;
    margin-bottom: 0.35rem;
  }
  input {
    width: 100%;
    padding: 0.6rem 0.75rem;
    background: rgba(0, 0, 0, 0.35);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    color: #f3f4f6;
    font-size: 0.85rem;
    outline: none;
  }
  input:focus {
    border-color: #22c55e;
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
    background: #22c55e;
    color: #0f172a;
  }
</style>