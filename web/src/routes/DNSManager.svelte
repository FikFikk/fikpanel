<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { DNSZone, DNSRecord } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import '../styles/pages/dnsmanager.css';

  let zones = $state<DNSZone[]>([]);
  let loading = $state(true);
  let selectedZone = $state<DNSZone | null>(null);
  let records = $state<DNSRecord[]>([]);
  let loadingRecords = $state(false);

  // State modal dan form
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
      confirmText: 'Hapus Zona',
      tone: 'danger',
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

<div class="pg-dnsmanager dns-view">
  <!-- Baris kepala halaman -->
  <header class="page-header">
    <div class="page-header__main">
      <h1 class="page-header__title">
        <i class="ri-node-tree"></i> DNS Zone & Record Manager
      </h1>
      <p class="page-header__sub">
        Kelola authoritative zone file, A/AAAA, CNAME, TXT, dan MX records langsung dari panel.
      </p>
    </div>
    <div class="page-header__actions">
      <button class="btn btn--ghost" onclick={loadZones} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh
      </button>
      <button class="btn btn--primary" onclick={() => (showCreateZone = true)}>
        <i class="ri-add-line"></i> Tambah Zona Domain
      </button>
    </div>
  </header>

  <div class="dns-layout">
    <!-- Panel daftar zona -->
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
                <span class="zone-name truncate">{z.name}</span>
                <span class="zone-ttl">TTL: {z.ttl}s</span>
              </div>
              <button
                class="btn btn--icon btn--sm btn--danger"
                aria-label="Hapus zona {z.name}"
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

    <!-- Panel tabel rekod DNS -->
    <div class="card">
      {#if selectedZone}
        <div class="card__head flex justify-between items-center">
          <div>
            <h3 class="card__title flex items-center gap-2">
              <i class="ri-record-mail-line"></i> Rekod DNS: {selectedZone.name}
            </h3>
          </div>
          <button class="btn btn--primary" onclick={() => (showAddRecord = true)}>
            <i class="ri-add-line"></i> Tambah Rekod
          </button>
        </div>

        {#if showAddRecord}
          <div class="add-rec-box">
            <div class="rec-grid">
              <div class="field">
                <label for="rec-t" class="field__label">Tipe</label>
                <select id="rec-t" class="select" bind:value={recType}>
                  <option value="A">A (IPv4)</option>
                  <option value="AAAA">AAAA (IPv6)</option>
                  <option value="CNAME">CNAME (Alias)</option>
                  <option value="TXT">TXT (Verifikasi / SPF)</option>
                  <option value="MX">MX (Mail Server)</option>
                </select>
              </div>
              <div class="field">
                <label for="rec-n" class="field__label">Nama Host / Subdomain</label>
                <input id="rec-n" type="text" bind:value={recName} placeholder="@ atau www" class="input mono" />
              </div>
              <div class="field">
                <label for="rec-c" class="field__label">Nilai / Target Konten</label>
                <input id="rec-c" type="text" bind:value={recContent} placeholder="1.2.3.4 atau domain.com" class="input mono" />
              </div>
              <div class="field">
                <label for="rec-ttl" class="field__label">TTL (Detik)</label>
                <input id="rec-ttl" type="number" class="input" bind:value={recTTL} />
              </div>
            </div>
            <div class="rec-actions">
              <button class="btn btn--ghost" onclick={() => (showAddRecord = false)}>Batal</button>
              <button class="btn btn--primary" onclick={handleAddRecord}>Simpan Rekod</button>
            </div>
          </div>
        {/if}

        <div class="table-wrap">
          <table class="table">
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
                    <td><span class="badge badge--info">{r.type}</span></td>
                    <td class="mono font-bold">{r.name}</td>
                    <td class="mono text-muted content-cell truncate" title={r.content}>{r.content}</td>
                    <td class="mono">{r.priority || '-'}</td>
                    <td class="mono">{r.ttl}s</td>
                    <td class="text-right">
                      <button class="btn btn--icon btn--sm btn--danger" aria-label="Hapus rekod {r.name}" onclick={() => handleDeleteRecord(r)}>
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
        <div class="state state--empty">
          <div class="state__icon"><i class="ri-node-tree"></i></div>
          <div class="state__title">Pilih atau buat zona domain terlebih dahulu</div>
          <div class="state__desc">Pilih salah satu domain di sebelah kiri atau tambahkan zona baru untuk mengelola rekod DNS.</div>
        </div>
      {/if}
    </div>
  </div>

  <!-- Modal buat zona -->
  <Modal
    open={showCreateZone}
    title="Tambah Zona Domain Baru"
    size="sm"
    onclose={() => (showCreateZone = false)}
  >
    <form id="dns-create-zone-form" onsubmit={(e) => { e.preventDefault(); handleCreateZone(); }} class="form-body">
      <div class="field">
        <label for="z-name" class="field__label">Nama Domain (Zone Name)</label>
        <input id="z-name" type="text" class="input" bind:value={newZoneName} placeholder="misal: example.my.id" required />
      </div>
      <div class="field">
        <label for="z-ttl" class="field__label">Default TTL</label>
        <input id="z-ttl" type="number" class="input" bind:value={newZoneTTL} />
      </div>
    </form>

    {#snippet footer()}
      <button type="button" class="btn btn--ghost" onclick={() => (showCreateZone = false)}>Batal</button>
      <button type="submit" form="dns-create-zone-form" class="btn btn--primary">Buat Zona</button>
    {/snippet}
  </Modal>
</div>
