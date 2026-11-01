<script lang="ts">
  // Subkomponen modal modifikasi proteksi opsi situs WAF
  import '../../styles/pages/waf/site-options-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { WAFSite } from '../../lib/types';

  interface Props {
    showSiteModal: boolean;
    editingSite: WAFSite | null;
    onSaveSiteSettings: () => void;
  }

  let {
    showSiteModal = $bindable(),
    editingSite = $bindable(),
    onSaveSiteSettings,
  }: Props = $props();
</script>

<div class="pg-waf-site-options-modal">
  <!-- Modal Ubah Opsi Situs WAF -->
  {#if showSiteModal && editingSite}
    <Modal
      open={showSiteModal && !!editingSite}
      title={`Modifikasi Proteksi WAF: ${editingSite.site_name}`}
      subtitle="Konfigurasi mitigasi bot, CC flood, dan CDN"
      size="md"
      onclose={() => (showSiteModal = false)}
    >
      <div class="stack-gap">
        <div class="form-check-group">
          <label class="custom-chk">
            <input type="checkbox" bind:checked={editingSite.fake_spider} />
            <span>Blokir Fake Spider & Malicious Crawlers</span>
          </label>
        </div>

        <div class="form-check-group">
          <label class="custom-chk">
            <input type="checkbox" bind:checked={editingSite.using_cdn} />
            <span>Situs Menggunakan CDN / Reverse Proxy Cloudflare</span>
          </label>
        </div>

        <div class="form-check-group">
          <label class="custom-chk">
            <input type="checkbox" bind:checked={editingSite.crawl_protection} />
            <span>Aktifkan Crawl Protection (Anti-Scraping)</span>
          </label>
        </div>

        <div class="form-check-group">
          <label class="custom-chk">
            <input type="checkbox" bind:checked={editingSite.rate_limit} />
            <span>Aktifkan Waiting Room / Rate Limiting</span>
          </label>
        </div>

        <div class="form-group mt-3">
          <label for="smart-cc">Ambang Batas Smart CC Defense (Permintaan / Detik)</label>
          <input
            id="smart-cc"
            type="number"
            bind:value={editingSite.smart_cc}
            min="10"
            max="10000"
            class="input-dark"
          />
        </div>
      </div>

      {#snippet footer()}
        <button type="button" class="btn btn-ghost" onclick={() => (showSiteModal = false)}>Batal</button>
        <button type="button" class="btn btn-primary" onclick={onSaveSiteSettings}>Simpan Konfigurasi</button>
      {/snippet}
    </Modal>
  {/if}
</div>

