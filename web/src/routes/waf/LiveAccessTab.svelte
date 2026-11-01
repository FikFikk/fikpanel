<script lang="ts">
  import type { WAFOverview, WAFAccessLog } from '../../lib/types';
  import '../../styles/pages/waf/live-access.css';

  // Props untuk tab Live Access & Visitor Observatory WAF
  interface Props {
    overview: WAFOverview | null;
    filteredAccessLogs: WAFAccessLog[];
    loadingAccessLogs: boolean;
    availableAccessDomains: string[];
    accessSearchQuery: string;
    accessDomainFilter: string;
    accessThreatFilter: 'all' | 'threat' | 'normal';
    accessStatusFilter: 'all' | '2xx' | '3xx' | '4xx' | '5xx';
    accessMethodFilter: string;
    formatTime: (ts: number) => string;
    formatDateTime: (ts: number) => string;
    formatBytes: (bytes: number) => string;
    onLoadAccessLogs: () => void;
    onQuickWhitelistIP: (ip: string, reason?: string) => void;
    onQuickBlockIP: (ip: string, reason?: string) => void;
    onQuickBlockPath: (urlPath: string, reason?: string) => void;
    onSelectAccessLog: (log: WAFAccessLog) => void;
  }

  let {
    overview,
    filteredAccessLogs,
    loadingAccessLogs,
    availableAccessDomains,
    accessSearchQuery = $bindable(),
    accessDomainFilter = $bindable(),
    accessThreatFilter = $bindable(),
    accessStatusFilter = $bindable(),
    accessMethodFilter = $bindable(),
    formatTime,
    formatDateTime,
    formatBytes,
    onLoadAccessLogs,
    onQuickWhitelistIP,
    onQuickBlockIP,
    onQuickBlockPath,
    onSelectAccessLog,
  }: Props = $props();
</script>

<div class="pg-waf-live-access">
  <!-- Top Visitors / Active Clients Telemetry -->
  {#if overview?.top_visitors && overview.top_visitors.length > 0}
    <div class="bento-panel">
      <div class="panel-header">
        <div class="panel-title-wrap">
          <i class="ri-user-location-line text-accent"></i>
          <div>
            <h3>Top Pengakses & Klien Aktif (Agregasi IP Pengunjung & Scanner)</h3>
            <p class="text-xs text-subtle">Rincian setiap alamat IP yang mengakses server beserta geolokasi, ISP, perangkat, total bandwidth, dan path terakhir yang diakses</p>
          </div>
        </div>
        <span class="badge-soft">{overview.top_visitors.length} Klien Teratas</span>
      </div>

      <div class="table-responsive">
        <table class="waf-main-table">
          <thead>
            <tr>
              <th>Alamat IP Klien</th>
              <th>Geolokasi & ISP</th>
              <th>Perangkat / OS / Peramban</th>
              <th>Total Request</th>
              <th>Total Bandwidth</th>
              <th>Status Ancaman</th>
              <th>Endpoint Terakhir Diakses</th>
              <th class="text-right">Kontrol Cepat</th>
            </tr>
          </thead>
          <tbody>
            {#each overview.top_visitors as vis}
              <tr class="{vis.threats > 0 ? 'row-threat-highlight' : ''}">
                <td>
                  <button
                    type="button"
                    class="btn-link mono font-bold"
                    onclick={() => accessSearchQuery = vis.ip}
                    title="Filter log untuk IP ini"
                  >
                    {vis.ip}
                  </button>
                  <div class="text-2xs text-muted mono">Terakhir: {formatTime(vis.last_seen)}</div>
                </td>
                <td>
                  <div class="geo-cell">
                    <span class="flag-code">{vis.country_code}</span>
                    <span class="font-medium text-xs">{vis.city ? `${vis.city}, ` : ''}{vis.country}</span>
                  </div>
                  <div class="mono text-2xs text-subtle truncate max-w-xs">{vis.isp}</div>
                </td>
                <td>
                  <div class="text-xs font-semibold">{vis.device}</div>
                  <div class="text-2xs text-subtle">{vis.os} · {vis.browser}</div>
                </td>
                <td class="mono font-bold text-accent">{vis.requests.toLocaleString()} req</td>
                <td class="mono text-xs">{formatBytes(vis.bytes_sent)}</td>
                <td>
                  {#if vis.threats > 0}
                    <span class="badge-danger-soft">{vis.threats} serangan/probe</span>
                  {:else}
                    <span class="badge-normal-traffic">Bersih (0 ancaman)</span>
                  {/if}
                </td>
                <td class="mono text-xs text-subtle truncate max-w-xs" title={vis.last_path}>{vis.last_path}</td>
                <td class="text-right nowrap">
                  <button
                    class="btn-icon-xs text-accent"
                    title="Filter semua request dari IP ini"
                    onclick={() => accessSearchQuery = vis.ip}
                  >
                    <i class="ri-filter-3-line"></i>
                  </button>
                  <button
                    class="btn-icon-xs text-success"
                    title="Masukkan IP ke Whitelist"
                    onclick={() => onQuickWhitelistIP(vis.ip, 'Trusted Visitor')}
                  >
                    <i class="ri-shield-check-line"></i>
                  </button>
                  <button
                    class="btn-icon-xs text-danger"
                    title="Blokir IP ke Blacklist"
                    onclick={() => onQuickBlockIP(vis.ip, `Visitor Activity (${vis.threats} threats)`)}
                  >
                    <i class="ri-shield-cross-line"></i>
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- Full Real-Time Access Log Explorer -->
  <div class="bento-panel">
    <div class="panel-header">
      <div class="panel-title-wrap">
        <i class="ri-terminal-window-line text-accent"></i>
        <div>
          <h3>Inspektor Log Akses HTTP & Paket Masuk Real-Time ({filteredAccessLogs.length} Rekaman)</h3>
          <p class="text-xs text-subtle">Pantau secara transparan seluruh halaman yang dibuka, API yang dipanggil, payload biner, referer, hingga User-Agent lengkap</p>
        </div>
      </div>
      <button class="btn btn-secondary btn-sm" onclick={onLoadAccessLogs} disabled={loadingAccessLogs}>
        <i class="ri-refresh-line {loadingAccessLogs ? 'spin' : ''}"></i> Muat Ulang Log
      </button>
    </div>

    <!-- Multi-Filter Toolbar -->
    <div class="studio-toolbar mt-3">
      <div class="search-input-wrap">
        <i class="ri-search-line search-icon"></i>
        <input
          type="text"
          bind:value={accessSearchQuery}
          placeholder="Cari berdasarkan IP, URL path, domain, kota, ISP, browser, atau User-Agent..."
          class="input-dark search-input"
        />
        {#if accessSearchQuery}
          <button class="clear-search-btn" aria-label="Bersihkan" onclick={() => accessSearchQuery = ''}>&times;</button>
        {/if}
      </div>

      <div class="toolbar-selects">
        <select bind:value={accessDomainFilter} class="input-dark select-sm">
          <option value="all">Semua Domain</option>
          {#each availableAccessDomains as dom}
            <option value={dom}>{dom}</option>
          {/each}
        </select>

        <select bind:value={accessThreatFilter} class="input-dark select-sm">
          <option value="all">Semua Lalu Lintas</option>
          <option value="threat">Hanya Ancaman / Probe</option>
          <option value="normal">Hanya Pengunjung Normal</option>
        </select>

        <select bind:value={accessStatusFilter} class="input-dark select-sm">
          <option value="all">Semua Status HTTP</option>
          <option value="2xx">HTTP 2xx (OK)</option>
          <option value="3xx">HTTP 3xx (Redirect)</option>
          <option value="4xx">HTTP 4xx (Not Found / Blocked)</option>
          <option value="5xx">HTTP 5xx (Server Error)</option>
        </select>

        <select bind:value={accessMethodFilter} class="input-dark select-sm">
          <option value="all">Semua Metode</option>
          <option value="GET">GET</option>
          <option value="POST">POST</option>
          <option value="PUT">PUT / PATCH</option>
          <option value="DELETE">DELETE</option>
          <option value="CONNECT">CONNECT / PROXY</option>
          <option value="HEAD">HEAD</option>
        </select>
      </div>
    </div>

    <div class="table-responsive mt-4">
      <table class="waf-main-table">
        <thead>
          <tr>
            <th>Waktu</th>
            <th>Status</th>
            <th>Metode & Situs</th>
            <th>URL Path / Endpoint Diakses</th>
            <th>Alamat IP & Geolokasi</th>
            <th>Klien & Perangkat</th>
            <th>Ukuran</th>
            <th>Analisis WAF</th>
            <th class="text-right">Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#if loadingAccessLogs && filteredAccessLogs.length === 0}
            <tr>
              <td colspan="9" class="text-center p-6 text-muted">
                <i class="ri-loader-4-line spin text-lg"></i> Memindai log akses Nginx real-time...
              </td>
            </tr>
          {:else if filteredAccessLogs.length === 0}
            <tr>
              <td colspan="9" class="text-center p-6 text-muted">
                Tidak ada rekaman akses HTTP yang cocok dengan kriteria filter di atas.
              </td>
            </tr>
          {:else}
            {#each filteredAccessLogs as log}
              <tr class="{log.is_threat ? 'row-threat-highlight' : ''}">
                <td class="mono text-xs text-muted nowrap">{formatDateTime(log.timestamp)}</td>
                <td>
                  <span class="http-status-badge s{Math.floor(log.status_code / 100)}xx">{log.status_code}</span>
                </td>
                <td class="nowrap">
                  <span class="method-mini {log.method.toLowerCase()}">{log.method}</span>
                  <span class="mono text-xs font-semibold text-accent">{log.site_name}</span>
                </td>
                <td>
                  <div class="mono text-xs font-medium break-all max-w-md" title={log.url_path}>{log.url_path}</div>
                  {#if log.referer}
                    <div class="mono text-2xs text-subtle truncate max-w-md">Ref: {log.referer}</div>
                  {/if}
                </td>
                <td>
                  <div class="mono text-xs font-bold">{log.client_ip}</div>
                  <div class="text-2xs text-subtle">
                    <span class="flag-code">{log.country_code}</span> {log.city ? `${log.city}, ` : ''}{log.country}
                  </div>
                  {#if log.isp}
                    <div class="mono text-2xs text-muted truncate max-w-xs">{log.isp}</div>
                  {/if}
                </td>
                <td>
                  <div class="text-xs font-medium">{log.device} · {log.os}</div>
                  <div class="text-2xs text-subtle">{log.browser} · <span class="mono">{log.protocol}</span></div>
                </td>
                <td class="mono text-xs nowrap">{formatBytes(log.bytes_sent)}</td>
                <td>
                  {#if log.is_threat}
                    <span class="badge-attack-tag">{log.attack_type}</span>
                    <div class="text-2xs text-danger mt-05">{log.action === 'block' ? 'Diblokir / 40x' : 'Peringatan'}</div>
                  {:else}
                    <span class="badge-normal-traffic"><i class="ri-shield-check-line"></i> Aman</span>
                  {/if}
                </td>
                <td class="text-right nowrap">
                  <button
                    class="btn-icon-xs text-accent"
                    title="Lihat Forensik Detail Request Ini"
                    onclick={() => onSelectAccessLog(log)}
                  >
                    <i class="ri-search-eye-line"></i>
                  </button>
                  <button
                    class="btn-icon-xs text-warning"
                    title="Blokir Path URL Ini"
                    onclick={() => onQuickBlockPath(log.url_path, `Blokir endpoint dari ${log.client_ip}`)}
                  >
                    <i class="ri-forbid-2-line"></i>
                  </button>
                  <button
                    class="btn-icon-xs text-danger"
                    title="Blokir IP ke Blacklist"
                    onclick={() => onQuickBlockIP(log.client_ip, log.attack_type || `Akses mencurigakan ke ${log.url_path}`)}
                  >
                    <i class="ri-shield-cross-line"></i>
                  </button>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</div>

