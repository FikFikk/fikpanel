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
  {#if showSiteModal && editingSite}
    <Modal
      open={showSiteModal && !!editingSite}
      title={`Konfigurasi Proteksi WAF: ${editingSite.site_name}`}
      subtitle="Atur pemblokiran bot, rate limiting Nginx, dan dukungan Real-IP CDN"
      size="md"
      onclose={() => (showSiteModal = false)}
    >
      <div class="stack-gap">
        <div class="form-check-group">
          <label class="custom-chk">
            <input type="checkbox" bind:checked={editingSite.fake_spider} />
            <span>Blokir Fake Spider &amp; Bad Crawlers di Nginx</span>
          </label>
        </div>

        <div class="form-check-group">
          <label class="custom-chk">
            <input type="checkbox" bind:checked={editingSite.using_cdn} />
            <span>Situs di Balik CDN / Cloudflare (Aktifkan Real-IP CF-Connecting-IP)</span>
          </label>
        </div>

        <div class="form-check-group">
          <label class="custom-chk">
            <input type="checkbox" bind:checked={editingSite.crawl_protection} />
            <span>Aktifkan Anti-Crawl / Anti-Scraping (Scrapy, wget, python-requests)</span>
          </label>
        </div>

        <div class="form-check-group">
          <label class="custom-chk">
            <input type="checkbox" bind:checked={editingSite.rate_limit} />
            <span>Aktifkan Pembatasan Laju Permintaan (Nginx limit_req)</span>
          </label>
        </div>

        <div class="form-group">
          <label for="smart-cc">Ambang Batas Smart CC (Permintaan / Detik per IP)</label>
          <input
            id="smart-cc"
            type="number"
            bind:value={editingSite.smart_cc}
            min="5"
            max="10000"
            class="input-dark mono"
          />
        </div>
      </div>

      {#snippet footer()}
        <button type="button" class="btn btn--ghost btn--sm" onclick={() => (showSiteModal = false)}>Batal</button>
        <button type="button" class="btn btn--primary btn--sm" onclick={onSaveSiteSettings}>Simpan Konfigurasi</button>
      {/snippet}
    </Modal>
  {/if}
</div>
