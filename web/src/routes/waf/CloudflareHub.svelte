<script lang="ts">
  import type { CloudflareConfig, CloudflareZone } from '../../lib/types';
  import '../../styles/pages/waf/cloudflare-hub.css';

  // Props untuk tab Cloudflare Hub WAF
  interface Props {
    cfConfig: CloudflareConfig | null;
    serverIP: string;
    allServerIPs: string[];
    loadingCF: boolean;
    cfTokenDiagnostic: string;
    showCFPermGuide: boolean;
    cfZoneFilter: 'all' | 'points_here' | 'local_sites';
    cfZones: CloudflareZone[];
    filteredCFZones: CloudflareZone[];
    expandedCFZones: Record<string, boolean>;
    onLoadCloudflare: () => void;
    onOpenConnectModal: () => void;
    onDisconnectCF: () => void;
    onOpenDNSModal: (zone: CloudflareZone, initialTab: 'all' | 'points_here' | 'proxied') => void;
    onOpenEdgeModal: (zone: CloudflareZone) => void;
    onUpdateCFZoneSetting: (zone: CloudflareZone, setting: string, value: string) => void;
    onToggleDevMode: (zone: CloudflareZone) => void;
    onPointZoneHere: (zone: CloudflareZone, proxied: boolean, targetHost?: string) => void;
    onPurgeCFCache: (zone: CloudflareZone) => void;
  }

  let {
    cfConfig,
    serverIP,
    allServerIPs,
    loadingCF,
    cfTokenDiagnostic,
    showCFPermGuide = $bindable(),
    cfZoneFilter = $bindable(),
    cfZones,
    filteredCFZones,
    expandedCFZones = $bindable(),
    onLoadCloudflare,
    onOpenConnectModal,
    onDisconnectCF,
    onOpenDNSModal,
    onOpenEdgeModal,
    onUpdateCFZoneSetting,
    onToggleDevMode,
    onPointZoneHere,
    onPurgeCFCache,
  }: Props = $props();
</script>

<div class="pg-waf-cloudflare-hub bento-panel cf-hub-container">
  <!-- Status Koneksi Header -->
  <div class="cf-status-card">
    <div class="cf-left">
      <div class="cf-logo-icon">
        <i class="ri-cloud-line"></i>
      </div>
      <div class="cf-head-copy">
        <div class="cf-title-row">
          <h3>Integrasi Akun Cloudflare API v4 &amp; Edge Control</h3>
          {#if cfConfig?.connected}
            <span class="badge-cf-connected">
              <i class="ri-checkbox-circle-fill"></i> Terhubung ({cfConfig.account_name})
            </span>
          {:else}
            <span class="badge-cf-disconnected">
              <i class="ri-close-circle-line"></i> Belum Terhubung
            </span>
          {/if}
        </div>
        <p class="cf-head-desc">
          Kelola seluruh domain Cloudflare, deteksi resolusi DNS ke IP Publik VPS (<span class="mono">{serverIP || '54.254.187.53'}</span>), manajemen record DNS (A/CNAME/TXT/MX), Mode SSL, Under Attack Mode, Always Use HTTPS, dan Purge Cache CDN.
        </p>
      </div>
    </div>

    <div class="cf-actions">
      {#if cfConfig?.connected}
        <button class="btn btn-secondary btn-sm" onclick={onLoadCloudflare} disabled={loadingCF}>
          <i class="ri-refresh-line {loadingCF ? 'spin' : ''}"></i> Sinkronkan Domain
        </button>
        <button class="btn btn-secondary btn-sm" onclick={onOpenConnectModal}>
          <i class="ri-key-2-line"></i> Ganti Token / Izin
        </button>
        <button class="btn btn-danger-outline btn-sm" onclick={onDisconnectCF}>
          <i class="ri-link-unlink-m"></i> Putuskan
        </button>
      {:else}
        <button class="btn btn-primary btn-sm" onclick={onOpenConnectModal}>
          <i class="ri-link"></i> Hubungkan Cloudflare
        </button>
      {/if}
    </div>
  </div>

  {#if !cfConfig?.connected}
    <div class="cf-empty-promo">
      <i class="ri-cloud-windy-line promo-icon"></i>
      <h4>Sinkronkan Domain Cloudflare Anda</h4>
      <p>
        Dapatkan kontrol penuh atas domain Anda di Cloudflare: deteksi otomatis apakah DNS mengarah ke IP Publik VPS Anda ({serverIP || '54.254.187.53'}), kelola record A/CNAME/TXT, ubah mode SSL &amp; Under Attack Mode, serta kosongkan cache dengan 1-klik.
      </p>
      <button class="btn btn-primary" onclick={onOpenConnectModal}>
        Mulai Hubungkan API Token / Global Key
      </button>
    </div>
  {:else}
    <!-- Diagnostik Izin API Token (Collapsible & Terstruktur) -->
    {#if cfTokenDiagnostic}
      <div class="cf-diagnostic-banner">
        <div class="cf-diag-compact-bar">
          <div class="cf-diag-left">
            <div class="cf-diag-icon-box">
              <i class="ri-radar-line"></i>
            </div>
            <div class="cf-diag-summary">
              <div class="cf-diag-headline">
                <strong>Mode DNS Live Aktif</strong>
                <span class="cf-diag-pill">Read-Only Fallback</span>
              </div>
              <p class="cf-diag-subtext">
                Endpoint Cloudflare <code>/zones</code> mengembalikan 0 zona pada token saat ini, sehingga FikPanel otomatis memindai seluruh subdomain aktif dari DNS publik &amp; situs lokal.
              </p>
            </div>
          </div>
          <div class="cf-diag-right">
            <button
              type="button"
              class="btn btn-sm btn-secondary cf-guide-toggle-btn {showCFPermGuide ? 'active' : ''}"
              onclick={() => (showCFPermGuide = !showCFPermGuide)}
            >
              <i class="ri-shield-keyhole-line text-accent"></i>
              <span>Panduan Izin Token</span>
              <i class={showCFPermGuide ? 'ri-arrow-up-s-line' : 'ri-arrow-down-s-line'}></i>
            </button>
            <button
              type="button"
              class="btn btn-sm btn-primary"
              onclick={onOpenConnectModal}
            >
              <i class="ri-key-2-line"></i> Perbarui Token / Key
            </button>
          </div>
        </div>

        {#if showCFPermGuide}
          <div class="cf-diag-expanded">
            <div class="cf-diag-note-row">
              <i class="ri-information-line text-warning"></i>
              <span>{cfTokenDiagnostic}</span>
            </div>
            <div class="cf-perm-grid">
              <div class="cf-perm-box highlight">
                <div class="cf-perm-head">
                  <span class="cf-perm-step">Langkah 1</span>
                  <span class="cf-perm-badge req">Wajib</span>
                </div>
                <div class="cf-perm-title">Daftar Semua Zona Muncul</div>
                <div class="cf-perm-path">
                  <code>Zone</code> <i class="ri-arrow-right-s-line"></i> <code>Zone</code> <i class="ri-arrow-right-s-line"></i> <code>Read</code>
                </div>
                <div class="cf-perm-foot">Setel <code>Zone Resources: Include All Zones</code></div>
              </div>
              <div class="cf-perm-box">
                <div class="cf-perm-head">
                  <span class="cf-perm-step">Langkah 2</span>
                  <span class="cf-perm-badge">DNS Manager</span>
                </div>
                <div class="cf-perm-title">Tambah / Ubah / Point DNS</div>
                <div class="cf-perm-path">
                  <code>Zone</code> <i class="ri-arrow-right-s-line"></i> <code>DNS</code> <i class="ri-arrow-right-s-line"></i> <code>Edit</code>
                </div>
                <div class="cf-perm-foot">Izin <code>Read</code> hanya dapat melihat daftar record</div>
              </div>
              <div class="cf-perm-box">
                <div class="cf-perm-head">
                  <span class="cf-perm-step">Langkah 3</span>
                  <span class="cf-perm-badge">Edge Control</span>
                </div>
                <div class="cf-perm-title">Ubah SSL, DDoS &amp; HTTPS</div>
                <div class="cf-perm-path">
                  <code>Zone</code> <i class="ri-arrow-right-s-line"></i> <code>Zone Settings</code> <i class="ri-arrow-right-s-line"></i> <code>Edit</code>
                </div>
                <div class="cf-perm-foot">Diperlukan untuk mengubah SSL &amp; Under Attack Mode</div>
              </div>
              <div class="cf-perm-box">
                <div class="cf-perm-head">
                  <span class="cf-perm-step">Langkah 4</span>
                  <span class="cf-perm-badge">CDN Cache</span>
                </div>
                <div class="cf-perm-title">Purge Cache Instan</div>
                <div class="cf-perm-path">
                  <code>Zone</code> <i class="ri-arrow-right-s-line"></i> <code>Cache Purge</code> <i class="ri-arrow-right-s-line"></i> <code>Purge</code>
                </div>
                <div class="cf-perm-foot">Mengizinkan pembersihan cache aset CDN 1-klik</div>
              </div>
            </div>
          </div>
        {/if}
      </div>
    {/if}

    <!-- Filter & Informasi IP Server -->
    <div class="cf-toolbar-row">
      <div class="cf-filter-tabs">
        <button
          type="button"
          class="cf-filter-btn {cfZoneFilter === 'all' ? 'active' : ''}"
          onclick={() => (cfZoneFilter = 'all')}
        >
          Semua Zona ({cfZones.length})
        </button>
        <button
          type="button"
          class="cf-filter-btn {cfZoneFilter === 'points_here' ? 'active' : ''}"
          onclick={() => (cfZoneFilter = 'points_here')}
        >
          <i class="ri-checkbox-circle-line text-accent"></i> Mengarah ke Server Ini ({cfZones.filter((z) => z.points_to_server).length})
        </button>
        <button
          type="button"
          class="cf-filter-btn {cfZoneFilter === 'local_sites' ? 'active' : ''}"
          onclick={() => (cfZoneFilter = 'local_sites')}
        >
          <i class="ri-hard-drive-2-line text-info"></i> Domain Situs FikPanel ({cfZones.filter((z) => (z.local_sites?.length || 0) > 0).length})
        </button>
      </div>

      <div class="cf-ip-badges">
        <span class="cf-ip-pill" title="IP Publik IPv4 yang digunakan untuk pencocokan record A di Cloudflare">
          <i class="ri-global-line text-accent"></i> IP Publik Server: <strong class="mono">{serverIP || 'Mendeteksi...'}</strong>
        </span>
        {#if allServerIPs.length > 1}
          <span class="cf-ip-pill subtle mono" title="IP Internal VPC / Interface LAN">
            VPC/LAN: <strong>{allServerIPs.slice(1).join(', ')}</strong>
          </span>
        {/if}
      </div>
    </div>

    <div class="table-responsive cf-table-wrap">
      <table class="waf-main-table cf-zones-table">
        <thead>
          <tr>
            <th>Domain Zona &amp; Status</th>
            <th>Domain Mengarah ke Server Ini ({serverIP})</th>
            <th>SSL/TLS &amp; DDoS Level</th>
            <th>Proteksi Edge</th>
            <th class="text-right">Kontrol &amp; DNS</th>
          </tr>
        </thead>
        <tbody>
          {#if loadingCF}
            <tr>
              <td colspan="5" class="text-center p-6 text-muted">
                <i class="ri-loader-4-line spin text-lg"></i> Mengambil data zona &amp; memindai DNS Cloudflare...
              </td>
            </tr>
          {:else if filteredCFZones.length === 0}
            <tr>
              <td colspan="5" class="text-center p-6 text-muted">
                Tidak ada domain yang sesuai dengan filter saat ini.
              </td>
            </tr>
          {:else}
            {#each filteredCFZones as zone}
              {@const pointingRecs = (zone.dns_records || []).filter((r) => r.points_to_server)}
              {@const otherRecs = (zone.dns_records || []).filter((r) => !r.points_to_server)}
              {@const isExpanded = !!expandedCFZones[zone.id]}
              <tr>
                <td class="cf-col-zone">
                  <div class="cf-zone-name-row">
                    <span class="font-bold mono text-accent text-sm">{zone.name}</span>
                    <span class="badge-cf-connected uppercase">{zone.status}</span>
                    {#if zone.source === 'local_dns_fallback'}
                      <span class="cf-source-tag warn" title="Terdeteksi dari DNS publik & situs lokal (tambahkan izin Zone:Read untuk mode edit penuh)">
                        <i class="ri-radar-line"></i> DNS Live
                      </span>
                    {:else}
                      <span class="cf-source-tag api">
                        <i class="ri-cloud-line"></i> API v4
                      </span>
                    {/if}
                  </div>
                  {#if zone.name_servers && zone.name_servers.length > 0}
                    <div class="cf-ns-line mono" title={zone.name_servers.join(', ')}>
                      <i class="ri-server-line"></i> {zone.name_servers.map((ns) => ns.replace('.ns.cloudflare.com', '')).join(', ')}.ns.cloudflare.com
                    </div>
                  {/if}
                </td>

                <td class="cf-col-domains">
                  {#if pointingRecs.length > 0}
                    <div class="cf-pointing-head">
                      <span class="badge-points-here">
                        <i class="ri-checkbox-circle-fill"></i> {pointingRecs.length} Domain Mengarah ke Server Ini
                      </span>
                      {#if otherRecs.length > 0}
                        <button
                          type="button"
                          class="cf-collapse-link"
                          onclick={() => onOpenDNSModal(zone, 'proxied')}
                          title="Subdomain lain memakai Cloudflare Tunnel (main-tunnel) atau mengarah ke server lain. Klik untuk melihat detailnya di manajemen DNS."
                        >
                          <i class="ri-git-branch-line"></i> {otherRecs.length} via Tunnel / Host Lain
                        </button>
                      {/if}
                    </div>
                    <div class="cf-matched-list">
                      {#each (isExpanded ? pointingRecs : pointingRecs.slice(0, 3)) as pr}
                        <div class="cf-matched-row mono">
                          <span class="cf-host-name">{pr.name}</span>
                          <span class="cf-ip-arrow">→ {pr.content}</span>
                          {#if pr.in_local_sites || (zone.local_sites || []).includes(pr.name)}
                            <span class="cf-site-tag registered" title="Sudah terdaftar di menu Website FikPanel">
                              <i class="ri-check-line"></i> Terdaftar di Website
                            </span>
                          {:else}
                            <span class="cf-site-tag unregistered" title="Mengarah ke IP server ini di Cloudflare, namun belum dibuat di menu Website">
                              Belum di Website
                            </span>
                          {/if}
                        </div>
                      {/each}
                      {#if pointingRecs.length > 3}
                        <button
                          type="button"
                          class="cf-collapse-link self-start"
                          onclick={() => (expandedCFZones[zone.id] = !expandedCFZones[zone.id])}
                        >
                          {isExpanded ? 'Ringkas tampilan' : `+${pointingRecs.length - 3} domain lainnya`}
                        </button>
                      {/if}
                    </div>
                  {:else}
                    <span class="badge-points-other" title="Domain ini mengarah ke IP/server lain atau Cloudflare Tunnel">
                      <i class="ri-external-link-line"></i> Mengarah ke Tunnel / Host Lain
                    </span>
                  {/if}
                </td>

                <td class="nowrap cf-col-ssl">
                  <div class="cf-dual-select-stack">
                    <div class="cf-mini-select-row">
                      <span class="cf-mini-lbl">SSL:</span>
                      <select
                        class="input-dark select-cf mono"
                        value={zone.ssl_mode || 'full'}
                        disabled={zone.read_only}
                        onchange={(e) => onUpdateCFZoneSetting(zone, 'ssl', (e.currentTarget as HTMLSelectElement).value)}
                        title={zone.read_only ? 'Tambahkan izin Zone Settings:Edit pada API Token untuk mengubah SSL' : 'Mode Enkripsi SSL/TLS Cloudflare'}
                      >
                        <option value="off">Off</option>
                        <option value="flexible">Flexible</option>
                        <option value="full">Full</option>
                        <option value="strict">Full (Strict)</option>
                      </select>
                    </div>
                    <div class="cf-mini-select-row">
                      <span class="cf-mini-lbl">DDoS:</span>
                      <select
                        class="input-dark select-cf {zone.security_level === 'under_attack' ? 'sec-uam' : ''}"
                        value={zone.security_level || 'medium'}
                        disabled={zone.read_only}
                        onchange={(e) => onUpdateCFZoneSetting(zone, 'security_level', (e.currentTarget as HTMLSelectElement).value)}
                        title={zone.read_only ? 'Tambahkan izin Zone Settings:Edit pada API Token untuk mengubah Security Level' : 'Tingkat Tantangan Keamanan & Mitigasi DDoS Cloudflare'}
                      >
                        <option value="under_attack">🔥 Under Attack!</option>
                        <option value="high">High</option>
                        <option value="medium">Medium</option>
                        <option value="low">Low</option>
                        <option value="essentially_off">Essentially Off</option>
                      </select>
                    </div>
                  </div>
                </td>

                <td class="nowrap cf-col-edge">
                  <div class="cf-edge-pills">
                    <span class="badge-toggle {zone.always_use_https ? 'active' : ''}" title="Always Use HTTPS">
                      HTTPS: {zone.always_use_https ? 'ON' : 'OFF'}
                    </span>
                    <span class="badge-toggle {zone.brotli ? 'active' : ''}" title="Brotli Compression">
                      Brotli: {zone.brotli ? 'ON' : 'OFF'}
                    </span>
                    <button
                      type="button"
                      class="badge-toggle warn {zone.development_mode === 1 ? 'active' : ''}"
                      disabled={zone.read_only}
                      onclick={() => onToggleDevMode(zone)}
                      title="Klik untuk toggle Development Mode (Bypass Cache 3 Jam)"
                    >
                      Dev: {zone.development_mode === 1 ? 'ON' : 'OFF'}
                    </button>
                  </div>
                </td>

                <td class="text-right actions-cell cf-col-actions">
                  <div class="cf-action-grid">
                    <button
                      type="button"
                      class="cf-act-btn primary-dns"
                      onclick={() => onOpenDNSModal(zone, 'points_here')}
                      title="Kelola Record DNS (Menampilkan domain server ini & subdomain Tunnel)"
                    >
                      <i class="ri-list-settings-line"></i>
                      <span>Kelola DNS</span>
                      <span class="cf-dns-count">{pointingRecs.length}/{zone.dns_records?.length || 0}</span>
                    </button>
                    <button
                      type="button"
                      class="cf-act-btn"
                      onclick={() => onOpenEdgeModal(zone)}
                      title="Pengaturan Edge Cloudflare (SSL, Under Attack Mode, Always HTTPS, Brotli, TLS 1.3)"
                    >
                      <i class="ri-equalizer-line"></i>
                      <span>Edge Config</span>
                    </button>
                    <button
                      type="button"
                      class="cf-act-btn"
                      onclick={() => onPointZoneHere(zone, true)}
                      title="Arahkan record A domain ke IP Publik server ini ({serverIP})"
                    >
                      <i class="ri-focus-3-line"></i>
                      <span>Point IP</span>
                    </button>
                    <button
                      type="button"
                      class="cf-act-btn danger-subtle"
                      onclick={() => onPurgeCFCache(zone)}
                      title="Kosongkan seluruh cache CDN Cloudflare"
                    >
                      <i class="ri-delete-bin-7-line"></i>
                      <span>Purge Cache</span>
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  {/if}
</div>

