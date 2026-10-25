<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { DNSZone, DNSRecord } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  let zones = $state<DNSZone[]>([]);
  let loading = $state(true);
  let selectedZone = $state<DNSZone | null>(null);
  let records = $state<DNSRecord[]>([]);
  let loadingRecords = $state(false);

  // Modal / Form state
  let showCreateZone = $state(false);
  let showAddRecord = $state(false);
  let newZoneName = $state('');
  let newZoneTTL = $state(3600);

  let recType = $state('A');
  let recName = $state('@');
  let recContent = $state('');
  let recPriority = $state(0);
  let recTTL = $state(3600);

  async function loadZones() {
    try {
      loading = true;
      const res = await apiFetch<{ ok: boolean; zones: DNSZone[] }>('/api/dns/zones');
      zones = res?.zones || [];
      if (zones.length > 0 && !selectedZone) {
        selectZone(zones[0]);
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat zona DNS', 'error');
    } finally {
      loading = false;
    }
  }

  async function selectZone(z: DNSZone) {
    selectedZone = z;
    try {
      loadingRecords = true;
      const res = await apiFetch<{ ok: boolean; records: DNSRecord[] }>(`/api/dns/zones/${z.id}/records`);
      records = res?.records || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat rekod DNS', 'error');
    } finally {
      loadingRecords = false;
    }
  }

  async function handleCreateZone() {
    if (!newZoneName.trim()) {
      notify('Nama domain zona wajib diisi', 'warning');
      return;
    }
    try {
      await apiFetch('/api/dns/zones', {
        method: 'POST',
        body: JSON.stringify({ name: newZoneName, ttl: newZoneTTL }),
      });
      notify(`Zona DNS "${newZoneName}" berhasil dibuat`, 'success');
      showCreateZone = false;
      newZoneName = '';
      await loadZones();
    } catch (err: any) {
      notify(err.message || 'Gagal membuat zona DNS', 'error');
    }
  }

  async function handleDeleteZone(z: DNSZone) {
    const ok = await confirmDialog({
      title: 'Hapus Zona DNS',
      message: `Hapus seluruh zona DNS "${z.name}" beserta semua rekodnya?`,
      confirmLabel: 'Hapus Zona',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/dns/zones/${z.id}`, { method: 'DELETE' });
      notify('Zona DNS berhasil dihapus', 'success');
      if (selectedZone?.id === z.id) selectedZone = null;
      await loadZones();
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus zona', 'error');
    }
  }

  async function handleAddRecord() {
    if (!selectedZone || !recName.trim() || !recContent.trim()) {
      notify('Parameter rekod belum lengkap', 'warning');
      return;
    }
    try {
      await apiFetch('/api/dns/records', {
        method: 'POST',
        body: JSON.stringify({
          zone_id: selectedZone.id,
          type: recType,
          name: recName,
          content: recContent,
          priority: recPriority,
          ttl: recTTL,
        }),
      });
      notify('Rekod DNS berhasil ditambahkan', 'success');
      showAddRecord = false;
      recName = '@';
      recContent = '';
      await selectZone(selectedZone);
    } catch (err: any) {
      notify(err.message || 'Gagal menambahkan rekod DNS', 'error');
    }
  }

  async function handleDeleteRecord(r: DNSRecord) {
    try {
      await apiFetch(`/api/dns/records/${r.id}`, { method: 'DELETE' });
      notify('Rekod DNS dihapus', 'success');
      if (selectedZone) await selectZone(selectedZone);
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus rekod', 'error');
    }
  }

  onMount(() => {
    loadZones();
  });
</script>

<div class="dns-view">
  <!-- Header Bar -->
  <header class="page-head">
    <div class="page-title-wrap">
      <h1 class="page-title">
        <i class="ri-node-tree"></i> DNS Zone & Record Manager
      </h1>
      <p class="page-sub">
        Kelola authoritative zone file, A/AAAA, CNAME, TXT, dan MX records langsung dari panel.
      </p>
    </div>
    <div class="page-actions">
      <button class="btn btn-ghost" onclick={loadZones} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh
      </button>
      <button class="btn btn-primary" onclick={() => (showCreateZone = true)}>
        <i class="ri-add-line"></i> Tambah Zona Domain
      </button>
    </div>
  </header>

  <div class="dns-layout">
    <!-- Sidebar Zones -->
    <div class="zones-panel">
      <div class="zones-head">DAFTAR ZONA DOMAIN ({zones.length})</div>
      <div class="zones-list">
        {#if loading && zones.length === 0}
          <div class="empty-note">Memuat zona...</div>
        {:else if zones.length === 0}
          <div class="empty-note">Belum ada domain zona terdaftar.</div>
        {:else}
          {#each zones as z (z.id)}
            <div
              class="zone-item {selectedZone?.id === z.id ? 'active' : ''}"
              onclick={() => selectZone(z)}
              role="button"
              tabindex="0"
              onkeydown={(e) => { if (e.key === 'Enter') selectZone(z); }}
            >
              <div class="zone-info">
                <span class="zone-name">{z.name}</span>
                <span class="zone-ttl">TTL: {z.ttl}s</span>
              </div>
              <button
                class="btn-icon danger"
                title="Hapus Zona"
                onclick={(e) => { e.stopPropagation(); handleDeleteZone(z); }}
              >
                <i class="ri-delete-bin-line"></i>
              </button>
            </div>
          {/each}
        {/if}
      </div>
    </div>

    <!-- Records Table -->
    <div class="records-panel">
      {#if selectedZone}
        <div class="panel-head flex justify-between items-center">
          <div>
            <h3 class="panel-title flex items-center gap-2">
              <i class="ri-record-mail-line"></i> Rekod DNS: {selectedZone.name}
            </h3>
          </div>
          <button class="btn btn-primary" onclick={() => (showAddRecord = true)}>
            <i class="ri-add-line"></i> Tambah Rekod
          </button>
        </div>

        {#if showAddRecord}
          <div class="add-rec-box">
            <div class="rec-grid">
              <div>
                <label for="rec-t">Tipe</label>
                <select id="rec-t" bind:value={recType}>
                  <option value="A">A (IPv4)</option>
                  <option value="AAAA">AAAA (IPv6)</option>
                  <option value="CNAME">CNAME (Alias)</option>
                  <option value="TXT">TXT (Verifikasi / SPF)</option>
                  <option value="MX">MX (Mail Server)</option>
                </select>
              </div>
              <div>
                <label for="rec-n">Nama Host / Subdomain</label>
                <input id="rec-n" type="text" bind:value={recName} placeholder="@ atau www" class="mono" />
              </div>
              <div>
                <label for="rec-c">Nilai / Target Konten</label>
                <input id="rec-c" type="text" bind:value={recContent} placeholder="1.2.3.4 atau domain.com" class="mono" />
              </div>
              <div>
                <label for="rec-ttl">TTL (Detik)</label>
                <input id="rec-ttl" type="number" bind:value={recTTL} />
              </div>
            </div>
            <div class="flex justify-end gap-2 mt-3">
              <button class="btn btn-ghost" onclick={() => (showAddRecord = false)}>Batal</button>
              <button class="btn btn-primary" onclick={handleAddRecord}>Simpan Rekod</button>
            </div>
          </div>
        {/if}

        <div class="table-wrap">
          <table class="data-table">
            <thead>
              <tr>
                <th>TIPE</th>
                <th>NAMA HOST</th>
                <th>NILAI / TARGET</th>
                <th>PRIORITAS</th>
                <th>TTL</th>
                <th class="text-right">HAPUS</th>
              </tr>
            </thead>
            <tbody>
              {#if loadingRecords}
                <tr><td colspan="6" class="table-empty">Memuat rekod DNS...</td></tr>
              {:else if records.length === 0}
                <tr><td colspan="6" class="table-empty">Belum ada rekod untuk zona ini.</td></tr>
              {:else}
                {#each records as r (r.id)}
                  <tr>
                    <td><span class="badge-type">{r.type}</span></td>
                    <td class="mono font-bold text-white">{r.name}</td>
                    <td class="mono text-muted max-w-xs truncate" title={r.content}>{r.content}</td>
                    <td class="mono">{r.priority || '-'}</td>
                    <td class="mono">{r.ttl}s</td>
                    <td class="text-right">
                      <button class="btn-icon danger" onclick={() => handleDeleteRecord(r)}>
                        <i class="ri-delete-bin-line"></i>
                      </button>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      {:else}
        <div class="empty-state">
          <i class="ri-node-tree empty-icon"></i>
          <p>Pilih atau buat zona domain terlebih dahulu untuk mengelola rekod DNS.</p>
        </div>
      {/if}
    </div>
  </div>
</div>

<!-- Modal Buat Zona -->
{#if showCreateZone}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) showCreateZone = false; }}
    onkeydown={(e) => { if (e.key === 'Escape') showCreateZone = false; }}
  >
    <div class="modal-card">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-node-tree"></i>
          <h3>Tambah Zona Domain Baru</h3>
        </div>
        <button class="btn-close" onclick={() => (showCreateZone = false)}>&times;</button>
      </div>

      <form onsubmit={(e) => { e.preventDefault(); handleCreateZone(); }} class="form-body">
        <div class="form-group">
          <label for="z-name">Nama Domain (Zone Name)</label>
          <input id="z-name" type="text" bind:value={newZoneName} placeholder="misal: example.my.id" required />
        </div>
        <div class="form-group">
          <label for="z-ttl">Default TTL</label>
          <input id="z-ttl" type="number" bind:value={newZoneTTL} />
        </div>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" onclick={() => (showCreateZone = false)}>Batal</button>
          <button type="submit" class="btn btn-primary">Buat Zona</button>
        </div>
      </form>
    </div>
  </div>
{/if}

<style>
  .dns-view {
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
  .dns-layout {
    display: grid;
    grid-template-columns: 280px 1fr;
    gap: 1.25rem;
  }
  .zones-panel {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    overflow: hidden;
    height: fit-content;
  }
  .zones-head {
    padding: 0.75rem 1rem;
    background: var(--bg-surface-soft);
    border-bottom: 1px solid var(--border);
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--text-subtle);
    letter-spacing: 0.05em;
  }
  .zones-list {
    display: flex;
    flex-direction: column;
  }
  .zone-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.85rem 1rem;
    border-bottom: 1px solid var(--border-soft);
    cursor: pointer;
    transition: background 0.15s;
  }
  .zone-item:hover {
    background: var(--bg-hover);
  }
  .zone-item.active {
    background: var(--bg-active);
    border-left: 3px solid var(--accent);
  }
  .zone-info {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .zone-name {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--text);
  }
  .zone-ttl {
    font-size: 0.7rem;
    color: var(--text-subtle);
  }
  .records-panel {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    overflow: hidden;
  }
  .panel-head {
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--border-soft);
  }
  .panel-title {
    font-size: 1rem;
    font-weight: 700;
    margin: 0;
    color: var(--text);
  }
  .add-rec-box {
    padding: 1rem 1.25rem;
    background: var(--bg-surface-soft);
    border-bottom: 1px solid var(--border-soft);
  }
  .rec-grid {
    display: grid;
    grid-template-columns: 140px 180px 1fr 100px;
    gap: 0.75rem;
  }
  .rec-grid label {
    display: block;
    font-size: 0.75rem;
    color: var(--text-muted);
    margin-bottom: 0.3rem;
  }
  .rec-grid input, .rec-grid select {
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
  .table-wrap {
    overflow-x: auto;
  }
  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
    text-align: left;
  }
  .data-table th {
    background: var(--bg-surface-soft);
    color: var(--text-subtle);
    padding: 0.75rem 1rem;
    font-size: 0.72rem;
    font-weight: 600;
    letter-spacing: 0.05em;
    border-bottom: 1px solid var(--border);
  }
  .data-table td {
    padding: 0.85rem 1rem;
    border-bottom: 1px solid var(--border-soft);
    color: var(--text);
  }
  .badge-type {
    display: inline-block;
    padding: 0.15rem 0.45rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 0.72rem;
    font-weight: 700;
    color: var(--accent);
  }
  .btn-icon {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0.35rem;
    border-radius: var(--radius-sm);
    font-size: 1rem;
  }
  .btn-icon.danger:hover {
    background: var(--tint-danger);
    color: var(--danger);
  }
  .text-right { text-align: right; }
  .mono { font-family: var(--font-mono); }
  .text-muted { color: var(--text-muted); }
  .text-subtle { color: var(--text-subtle); }
  .font-bold { font-weight: 600; }
  .empty-note {
    padding: 1.5rem;
    text-align: center;
    font-size: 0.8rem;
    color: var(--text-subtle);
  }
  .empty-state {
    padding: 4rem 2rem;
    text-align: center;
    color: var(--text-muted);
  }
  .empty-icon {
    font-size: 2.5rem;
    color: var(--text-subtle);
    display: block;
    margin-bottom: 0.5rem;
  }
  .table-empty {
    text-align: center;
    padding: 2.5rem;
    color: var(--text-subtle);
  }

  /* Modal */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: var(--bg-overlay);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: var(--z-modal);
    padding: 1rem;
  }
  .modal-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    width: 100%;
    max-width: 480px;
    padding: 1.5rem;
    box-shadow: var(--shadow-modal);
  }
  .modal-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.25rem;
  }
  .modal-title-wrap {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .modal-title-wrap i {
    font-size: 1.3rem;
    color: var(--primary);
  }
  .modal-title-wrap h3 {
    margin: 0;
    font-size: 1rem;
    color: var(--text);
  }
  .btn-close {
    background: transparent;
    border: none;
    color: var(--text-subtle);
    font-size: 1.4rem;
    cursor: pointer;
  }
  .form-body {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .form-group label {
    display: block;
    font-size: 0.78rem;
    font-weight: 500;
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
  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
    margin-top: 1rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border-soft);
  }
  .flex { display: flex; }
  .justify-between { justify-content: space-between; }
  .items-center { align-items: center; }
  .gap-2 { gap: 0.5rem; }
  .mt-3 { margin-top: 0.75rem; }
  .spin { animation: spin 1s linear infinite; }
  @keyframes spin { 100% { transform: rotate(360deg); } }
</style>
