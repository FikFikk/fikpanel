<script lang="ts">
  // Subkomponen modal manajemen DNS records Cloudflare
  import '../../styles/pages/waf/cf-dns-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { CloudflareZone, CloudflareDNSRecord } from '../../lib/types';

  interface Props {
    showDNSModal: boolean;
    selectedZone: CloudflareZone | null;
    serverIP: string;
    dnsEditId: string;
    dnsType: 'A' | 'AAAA' | 'CNAME' | 'TXT' | 'MX';
    dnsName: string;
    dnsContent: string;
    dnsProxied: boolean;
    savingDNS: boolean;
    dnsFilterTab: 'all' | 'points_here' | 'proxied';
    dnsSearchQuery: string;
    onPointZoneHere: (zone: CloudflareZone, proxied: boolean, targetHost?: string) => void;
    onSaveCFDNSRecord: () => void;
    onResetDNSForm: () => void;
    onToggleCFDNSProxy: (zone: CloudflareZone, rec: CloudflareDNSRecord) => void;
    onStartEditDNSRecord: (rec: CloudflareDNSRecord) => void;
    onDeleteCFDNSRecord: (zone: CloudflareZone, rec: CloudflareDNSRecord) => void;
  }

  let {
    showDNSModal = $bindable(),
    selectedZone,
    serverIP,
    dnsEditId,
    dnsType = $bindable(),
    dnsName = $bindable(),
    dnsContent = $bindable(),
    dnsProxied = $bindable(),
    savingDNS,
    dnsFilterTab = $bindable(),
    dnsSearchQuery = $bindable(),
    onPointZoneHere,
    onSaveCFDNSRecord,
    onResetDNSForm,
    onToggleCFDNSProxy,
    onStartEditDNSRecord,
    onDeleteCFDNSRecord,
  }: Props = $props();
</script>

<div class="pg-waf-cf-dns-modal">
  <!-- Modal Manajemen DNS Records (Full CRUD + 1-Click Local Site Sync) -->
  {#if showDNSModal && selectedZone}
    {@const allZoneRecs = selectedZone.dns_records || []}
    {@const filteredZoneRecs = allZoneRecs.filter((r) => {
      if (dnsFilterTab === 'points_here' && !r.points_to_server) return false;
      if (dnsFilterTab === 'proxied' && !r.proxied) return false;
      if (dnsSearchQuery.trim()) {
        const q = dnsSearchQuery.trim().toLowerCase();
        return r.name.toLowerCase().includes(q) || r.content.toLowerCase().includes(q) || r.type.toLowerCase().includes(q);
      }
      return true;
    })}
    <Modal
      open={showDNSModal && !!selectedZone}
      title={`Manajemen DNS Cloudflare: ${selectedZone.name}`}
      subtitle={`IP Publik Server Ini: ${serverIP} · Kelola record A, AAAA, CNAME, TXT, MX & Proxy Orange Cloud`}
      size="xl"
      onclose={() => (showDNSModal = false)}
    >
      <!-- Pintasan 1-Klik Arahkan Situs Lokal ke Server Ini -->
      {#if selectedZone.local_sites && selectedZone.local_sites.length > 0}
        <div class="cf-quick-sync-bar mb-3">
          <span class="text-xs font-semibold"><i class="ri-magic-line text-accent"></i> Pintasan Situs FikPanel:</span>
          {#each selectedZone.local_sites as ls}
            <button
              type="button"
              class="btn btn-xs btn-secondary mono"
              onclick={() => onPointZoneHere(selectedZone!, true, ls)}
              title="Arahkan {ls} ke {serverIP} (Proxied)"
            >
              <i class="ri-cloud-fill text-warning"></i> Point {ls} → {serverIP} (Proxied)
            </button>
            <button
              type="button"
              class="btn btn-xs btn-ghost mono"
              onclick={() => onPointZoneHere(selectedZone!, false, ls)}
              title="Arahkan {ls} ke {serverIP} (DNS Only)"
            >
              <i class="ri-cloud-line"></i> {ls} (DNS Only)
            </button>
          {/each}
        </div>
      {/if}

      <!-- Form Tambah / Ubah Record DNS -->
      <div class="cf-dns-form-card mb-4">
        <div class="text-xs font-semibold mb-2">
          <i class="ri-add-circle-line text-accent"></i>
          {dnsEditId ? `Ubah Record DNS (${dnsName})` : 'Tambah Record DNS Baru'}
        </div>
        <div class="cf-dns-form-grid">
          <select bind:value={dnsType} class="input-dark select-sm mono">
            <option value="A">A</option>
            <option value="AAAA">AAAA</option>
            <option value="CNAME">CNAME</option>
            <option value="TXT">TXT</option>
            <option value="MX">MX</option>
          </select>

          <input
            type="text"
            bind:value={dnsName}
            placeholder="Nama (mis. @ atau sub.{selectedZone.name})"
            class="input-dark mono text-xs"
          />

          <div class="cf-content-input-group">
            <input
              type="text"
              bind:value={dnsContent}
              placeholder={dnsType === 'A' ? `IPv4 (mis. ${serverIP})` : 'Target / Content'}
              class="input-dark mono text-xs"
            />
            {#if dnsType === 'A' && serverIP}
              <button
                type="button"
                class="btn btn-xs btn-secondary nowrap"
                onclick={() => (dnsContent = serverIP)}
                title="Gunakan IP Publik Server FikPanel ini"
              >
                Pakai IP Server
              </button>
            {/if}
          </div>

          <label class="custom-chk text-xs nowrap" title="Aktifkan Cloudflare CDN & WAF Proxy (Orange Cloud)">
            <input type="checkbox" bind:checked={dnsProxied} disabled={dnsType === 'TXT' || dnsType === 'MX'} />
            <span>Proxied ☁️</span>
          </label>

          <div class="cf-dns-form-btns">
            <button
              type="button"
              class="btn btn-primary btn-sm"
              onclick={onSaveCFDNSRecord}
              disabled={savingDNS || selectedZone.read_only}
            >
              {savingDNS ? 'Menyimpan...' : dnsEditId ? 'Update' : '+ Simpan'}
            </button>
            {#if dnsEditId}
              <button type="button" class="btn btn-ghost btn-sm" onclick={onResetDNSForm}>
                Batal
              </button>
            {/if}
          </div>
        </div>
      </div>

      <!-- Filter & Pencarian Cepat Record DNS di Modal -->
      <div class="cf-dns-toolbar">
        <div class="cf-filter-tabs">
          <button
            type="button"
            class="cf-filter-btn {dnsFilterTab === 'all' ? 'active' : ''}"
            onclick={() => (dnsFilterTab = 'all')}
          >
            Semua ({allZoneRecs.length})
          </button>
          <button
            type="button"
            class="cf-filter-btn {dnsFilterTab === 'points_here' ? 'active' : ''}"
            onclick={() => (dnsFilterTab = 'points_here')}
          >
            Mengarah ke Server Ini ({allZoneRecs.filter((r) => r.points_to_server).length})
          </button>
          <button
            type="button"
            class="cf-filter-btn {dnsFilterTab === 'proxied' ? 'active' : ''}"
            onclick={() => (dnsFilterTab = 'proxied')}
          >
            Proxied / Tunnel ({allZoneRecs.filter((r) => r.proxied).length})
          </button>
        </div>
        <input
          type="text"
          bind:value={dnsSearchQuery}
          placeholder="Cari subdomain atau IP..."
          class="input-dark cf-dns-search-input mono text-xs"
        />
      </div>

      <div class="table-responsive cf-dns-table-scroll">
        <table class="waf-main-table">
          <thead>
            <tr>
              <th>Type</th>
              <th>Name (Subdomain)</th>
              <th>Content / Target IP</th>
              <th>Proxy Status</th>
              <th>Status di Server Ini</th>
              <th class="text-right">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#if filteredZoneRecs.length === 0}
              <tr>
                <td colspan="6" class="text-center p-4 text-muted">Tidak ada rekaman DNS yang sesuai dengan filter.</td>
              </tr>
            {:else}
              {#each filteredZoneRecs as rec}
                <tr>
                  <td><span class="badge-soft mono">{rec.type}</span></td>
                  <td class="mono font-semibold text-xs">{rec.name}</td>
                  <td class="mono text-xs text-subtle break-all max-w-xs">{rec.content}</td>
                  <td>
                    <button
                      type="button"
                      class="btn-proxy-toggle {rec.proxied ? 'proxied' : 'dns-only'}"
                      disabled={selectedZone.read_only || (rec.type !== 'A' && rec.type !== 'AAAA' && rec.type !== 'CNAME')}
                      onclick={() => onToggleCFDNSProxy(selectedZone!, rec)}
                      title="Klik untuk beralih antara Proxied (Orange Cloud) dan DNS Only (Grey Cloud)"
                    >
                      {#if rec.proxied}
                        <span class="badge-orange-cloud"><i class="ri-cloud-fill"></i> Proxied (CDN)</span>
                      {:else}
                        <span class="badge-grey-cloud"><i class="ri-cloud-line"></i> DNS Only</span>
                      {/if}
                    </button>
                  </td>
                  <td>
                    {#if rec.points_to_server}
                      <div class="cf-server-match-cell">
                        <span class="badge-points-here"><i class="ri-check-line"></i> Mengarah ke Server Ini</span>
                        {#if rec.in_local_sites || (selectedZone.local_sites || []).includes(rec.name)}
                          <span class="cf-site-tag registered"><i class="ri-check-line"></i> Terdaftar di Website</span>
                        {:else}
                          <span class="cf-site-tag unregistered">Belum di Website</span>
                        {/if}
                      </div>
                    {:else if rec.proxied}
                      <span class="cf-source-tag warn"><i class="ri-git-branch-line"></i> Cloudflare Tunnel / Proxy Lain</span>
                    {:else}
                      <span class="text-muted text-xs">IP Server Lain ({rec.content})</span>
                    {/if}
                  </td>
                  <td class="text-right nowrap">
                    <button
                      type="button"
                      class="btn-icon-xs text-accent"
                      disabled={selectedZone.read_only}
                      onclick={() => onStartEditDNSRecord(rec)}
                      title="Edit Record DNS Ini"
                    >
                      <i class="ri-edit-line"></i>
                    </button>
                    <button
                      type="button"
                      class="btn-icon-xs text-danger"
                      disabled={selectedZone.read_only}
                      onclick={() => onDeleteCFDNSRecord(selectedZone!, rec)}
                      title="Hapus Record DNS Ini"
                    >
                      <i class="ri-delete-bin-line"></i>
                    </button>
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>

      {#snippet footer()}
        <button type="button" class="btn btn-ghost" onclick={() => (showDNSModal = false)}>Tutup</button>
      {/snippet}
    </Modal>
  {/if}
</div>

