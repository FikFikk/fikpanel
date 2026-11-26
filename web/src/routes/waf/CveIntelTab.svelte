<script lang="ts">
  import type { CVEFeedReport, VirtualPatch } from '../../lib/types';
  import '../../styles/pages/waf/cve-intel.css';

  // Props untuk tab CVE Threat Intelligence & Virtual Patching 1-Klik
  interface Props {
    report: CVEFeedReport | null;
    loading: boolean;
    refreshing: boolean;
    togglingPatchId: string;
    onRefresh: (force: boolean) => void;
    onTogglePatch: (patchId: string, enabled: boolean) => void;
    onTestPatchPayload: (patch: VirtualPatch) => void;
    formatDateTime: (ts: number) => string;
  }

  let {
    report,
    loading,
    refreshing,
    togglingPatchId,
    onRefresh,
    onTogglePatch,
    onTestPatchPayload,
    formatDateTime,
  }: Props = $props();

  // State pencarian dan filter feed CVE
  let searchQuery = $state('');
  let productFilter = $state('all');
  let scopeFilter = $state<'all' | 'host' | 'kev' | 'patchable'>('all');

  // Daftar produk unik yang tersedia pada laporan CVE saat ini
  let availableProducts = $derived(
    (() => {
      const set = new Set<string>(['nginx', 'php', 'go', 'react', 'nextjs', 'laravel', 'wordpress', 'node', 'openssh', 'openssl']);
      for (const a of report?.advisories || []) {
        if (a.product) set.add(a.product.toLowerCase());
      }
      return Array.from(set).sort();
    })()
  );

  // Filter multi-dimensi terhadap daftar berita CVE
  let filteredAdvisories = $derived(
    (report?.advisories || []).filter((a) => {
      if (productFilter !== 'all' && a.product.toLowerCase() !== productFilter) return false;
      if (scopeFilter === 'host' && a.host_match === 'global') return false;
      if (scopeFilter === 'kev' && !a.is_kev) return false;
      if (scopeFilter === 'patchable' && !a.virtual_patch_id) return false;
      if (!searchQuery.trim()) return true;
      const q = searchQuery.toLowerCase();
      return (
        a.cve_id.toLowerCase().includes(q) ||
        a.title.toLowerCase().includes(q) ||
        a.description.toLowerCase().includes(q) ||
        a.product.toLowerCase().includes(q) ||
        (a.affected_range && a.affected_range.toLowerCase().includes(q))
      );
    })
  );

  // Label sumber feed yang mudah dipahami admin
  function sourceLabel(src: string): string {
    if (src === 'cisa_kev') return 'CISA KEV';
    if (src === 'github_advisory') return 'GitHub Advisory';
    return 'Kurasi Kritis';
  }
</script>

<div class="pg-waf-cve">
  <!-- Strip Stack Host yang Terdeteksi Otomatis -->
  <section class="cve-stack-banner">
    <div class="cve-stack-head">
      <div class="cve-stack-title">
        <i class="ri-cpu-line text-accent"></i>
        <span>Stack Runtime &amp; Framework Terdeteksi pada Server Ini</span>
      </div>
      <div class="cve-source-meta">
        {#if report?.sources_status}
          {#each Object.entries(report.sources_status) as [srcKey, srcVal]}
            <span class="cve-source-chip {srcVal.startsWith('ok') || srcVal === 'cache' ? 'ok' : ''}">
              <i class="ri-rss-line"></i> {sourceLabel(srcKey)}: {srcVal}
            </span>
          {/each}
        {/if}
        {#if report?.last_fetched_at}
          <span class="mono">Pembaruan: {formatDateTime(report.last_fetched_at)}</span>
        {/if}
      </div>
    </div>

    <div class="cve-stack-list">
      {#if !report?.installed_stack || report.installed_stack.length === 0}
        <span class="text-xs text-muted">Memindai biner runtime &amp; framework proyek host...</span>
      {:else}
        {#each report.installed_stack as st}
          <span class="stack-pill">
            <strong>{st.label}</strong>
            <span class="stack-pill__ver">{st.version || 'terpasang'}</span>
          </span>
        {/each}
      {/if}
    </div>
  </section>

  <!-- 4 Kartu Ringkasan Intelijen Kerentanan -->
  <div class="cve-kpi-grid">
    <div class="cve-kpi-card">
      <div class="cve-kpi-head">
        <span class="cve-kpi-label">Total Berita CVE Dipantau</span>
        <i class="ri-bug-line cve-kpi-icon text-info"></i>
      </div>
      <div class="cve-kpi-val">{report?.total_count ?? 0}</div>
      <div class="cve-kpi-sub">CISA KEV + GitHub Advisories + Katalog Web</div>
    </div>

    <div class="cve-kpi-card is-danger">
      <div class="cve-kpi-head">
        <span class="cve-kpi-label">CISA KEV (Aktif Dieksploitasi)</span>
        <i class="ri-alarm-warning-line cve-kpi-icon text-danger"></i>
      </div>
      <div class="cve-kpi-val text-danger">{report?.kev_count ?? 0}</div>
      <div class="cve-kpi-sub">Kerentanan prioritas tinggi dieksploitasi di alam liar</div>
    </div>

    <div class="cve-kpi-card is-warning">
      <div class="cve-kpi-head">
        <span class="cve-kpi-label">Relevan Stack Server Ini</span>
        <i class="ri-radar-line cve-kpi-icon text-warning"></i>
      </div>
      <div class="cve-kpi-val text-warning">{report?.host_matched_count ?? 0}</div>
      <div class="cve-kpi-sub">Cocok dengan runtime / framework terpasang di host</div>
    </div>

    <div class="cve-kpi-card is-accent">
      <div class="cve-kpi-head">
        <span class="cve-kpi-label">Virtual Patch Nginx Aktif</span>
        <i class="ri-shield-check-line cve-kpi-icon text-accent"></i>
      </div>
      <div class="cve-kpi-val text-accent">
        {report?.active_patches_count ?? 0} / {report?.virtual_patches?.length ?? 10}
      </div>
      <div class="cve-kpi-sub">Mitigasi langsung di lapisan Nginx tanpa tunggu upgrade</div>
    </div>
  </div>

  <!-- Toolbar Pencarian & Filter CVE -->
  <div class="cve-toolbar">
    <div class="cve-toolbar-left">
      <div class="cve-search-wrap">
        <i class="ri-search-line"></i>
        <input
          type="text"
          class="cve-search-input"
          bind:value={searchQuery}
          placeholder="Cari CVE-2025-55182, PHP2Shell, React2Shell, Nginx, Go..."
          aria-label="Cari berita CVE"
        />
      </div>

      <select class="cve-select" bind:value={productFilter} aria-label="Filter produk CVE">
        <option value="all">Semua Stack / Produk</option>
        {#each availableProducts as p}
          <option value={p}>{p.toUpperCase()}</option>
        {/each}
      </select>

      <select class="cve-select" bind:value={scopeFilter} aria-label="Filter relevansi CVE">
        <option value="all">Semua Kerentanan ({report?.advisories?.length ?? 0})</option>
        <option value="host">Relevan Stack Host ({report?.host_matched_count ?? 0})</option>
        <option value="kev">Hanya CISA KEV ({report?.kev_count ?? 0})</option>
        <option value="patchable">Tersedia Virtual Patch 1-Klik</option>
      </select>
    </div>

    <button
      type="button"
      class="btn btn--primary btn--sm"
      onclick={() => onRefresh(true)}
      disabled={refreshing || loading}
    >
      <i class="ri-refresh-line {refreshing || loading ? 'spin' : ''}"></i>
      {refreshing ? 'Menarik Feed Live...' : 'Segarkan Feed Live'}
    </button>
  </div>

  <!-- Grid 2 Kolom: Feed CVE (Kiri) + Virtual Patch Shield 1-Klik (Kanan) -->
  <div class="cve-main-grid">
    <!-- Kolom Kiri: Daftar Berita Kerentanan CVE -->
    <div class="cve-feed-list">
      {#if loading && !report}
        <div class="cve-empty">
          <i class="ri-loader-4-line spin"></i> Memuat berita kerentanan CVE dari CISA KEV &amp; GitHub Security Advisories...
        </div>
      {:else if filteredAdvisories.length === 0}
        <div class="cve-empty">
          Tidak ada entri CVE yang cocok dengan filter saat ini. Coba ubah kata kunci atau filter produk.
        </div>
      {:else}
        {#each filteredAdvisories as adv (adv.cve_id)}
          <article
            class="cve-card {adv.virtual_patch_active
              ? 'is-patched'
              : adv.host_match === 'affected'
                ? 'is-affected'
                : adv.host_match === 'installed_stack'
                  ? 'is-stack'
                  : ''}"
          >
            <div class="cve-card-top">
              <div class="cve-badges">
                <span class="cve-id-badge">{adv.cve_id}</span>
                <span class="sev-pill {adv.severity}">
                  {adv.severity}{adv.cvss_score ? ` · CVSS ${adv.cvss_score.toFixed(1)}` : ''}
                </span>
                <span class="prod-pill">{adv.product}</span>
                {#if adv.is_kev}
                  <span class="kev-pill"><i class="ri-fire-fill"></i> CISA KEV</span>
                {/if}
                {#if adv.host_match === 'affected'}
                  <span class="host-match-pill affected">
                    <i class="ri-error-warning-line"></i> Risiko di Server Ini ({adv.installed_version || 'terpasang'})
                  </span>
                {:else if adv.host_match === 'installed_stack'}
                  <span class="host-match-pill installed_stack">
                    <i class="ri-server-line"></i> Stack Terpasang ({adv.installed_version || 'aktif'})
                  </span>
                {/if}
              </div>
              <span class="cve-date">{adv.published_at || '-'}</span>
            </div>

            <h4 class="cve-title">{adv.title}</h4>
            <p class="cve-desc">{adv.description}</p>

            <div class="cve-meta-bar mono">
              {#if adv.affected_range}
                <span class="cve-meta-item">
                  <i class="ri-alert-line text-warning"></i> Terdampak: <strong>{adv.affected_range}</strong>
                </span>
              {/if}
              {#if adv.patched_version}
                <span class="cve-meta-item">
                  <i class="ri-checkbox-circle-line text-accent"></i> Versi Aman: <strong>{adv.patched_version}</strong>
                </span>
              {/if}
              <span class="cve-meta-item">
                <i class="ri-database-2-line"></i> {sourceLabel(adv.source)}
              </span>
            </div>

            <div class="cve-card-footer">
              <a
                href={adv.reference_url}
                target="_blank"
                rel="noopener noreferrer"
                class="cve-ref-link"
              >
                <i class="ri-external-link-line"></i> Detail NVD / Advisory Resmi
              </a>

              {#if adv.virtual_patch_id}
                <button
                  type="button"
                  class="btn btn--sm {adv.virtual_patch_active ? 'btn--secondary' : 'btn--primary'}"
                  disabled={togglingPatchId === adv.virtual_patch_id}
                  onclick={() => onTogglePatch(adv.virtual_patch_id!, !adv.virtual_patch_active)}
                >
                  {#if togglingPatchId === adv.virtual_patch_id}
                    <i class="ri-loader-4-line spin"></i> Menyinkronkan...
                  {:else if adv.virtual_patch_active}
                    <i class="ri-shield-check-fill text-accent"></i> Virtual Patch Aktif (Nonaktifkan)
                  {:else}
                    <i class="ri-shield-flash-line"></i> Pasang Virtual Patch 1-Klik
                  {/if}
                </button>
              {:else}
                <span class="text-xs text-muted">
                  <i class="ri-information-line"></i> Mitigasi melalui update paket OS / biner
                </span>
              {/if}
            </div>
          </article>
        {/each}
      {/if}
    </div>

    <!-- Kolom Kanan: Katalog Virtual Patch 1-Klik -->
    <aside class="vp-panel">
      <div class="vp-panel-head">
        <h3><i class="ri-shield-star-line text-accent"></i> Virtual Patch Shield (Nginx)</h3>
        <p>
          Aktifkan aturan blokir eksploit CVE populer secara instan di <code class="mono">waf-rules.conf</code> sambil menunggu jadwal upgrade aplikasi.
        </p>
      </div>

      <div class="vp-list">
        {#each report?.virtual_patches || [] as vp (vp.id)}
          <div class="vp-item {vp.enabled ? 'is-enabled' : ''}">
            <div class="vp-item-head">
              <div>
                <div class="vp-item-title">{vp.title}</div>
                <div class="vp-cve-tags">
                  {#each vp.cve_ids as cid}
                    <span class="vp-cve-tag">{cid}</span>
                  {/each}
                  <span class="sev-pill {vp.severity}">{vp.severity}</span>
                </div>
              </div>

              <label class="switch" aria-label="Aktifkan Virtual Patch {vp.title}">
                <input
                  type="checkbox"
                  checked={vp.enabled}
                  disabled={togglingPatchId === vp.id}
                  onchange={() => onTogglePatch(vp.id, !vp.enabled)}
                />
                <span class="slider"></span>
              </label>
            </div>

            <p class="vp-item-desc">{vp.mitigation}</p>

            <div class="vp-item-actions">
              <span class="mono text-2xs text-subtle">
                {vp.enabled ? 'Aktif di Nginx WAF' : 'Siap dipasang'}
              </span>
              <button
                type="button"
                class="btn-link"
                onclick={() => onTestPatchPayload(vp)}
                title="Uji payload eksploitasi ini di Simulator WAF"
              >
                <i class="ri-flask-line"></i> Uji Payload
              </button>
            </div>
          </div>
        {/each}
      </div>
    </aside>
  </div>
</div>
