<script lang="ts">
  import type { WAFOverview, WAFEvent, WAFAccessLog } from '../../lib/types';
  import '../../styles/pages/waf/overview.css';

  // Props untuk tab Overview WAF
  interface Props {
    overview: WAFOverview | null;
    events: WAFEvent[];
    accessLogs: WAFAccessLog[];
    attachingVhosts: boolean;
    overviewMapAction: (node: HTMLElement, eventsParam?: WAFEvent[]) => { update: () => void; destroy: () => void };
    formatBytes: (bytes: number) => string;
    formatTime: (ts: number) => string;
    onOpenAttackMap: () => void;
    onOpenCveIntel: () => void;
    onAttachVhosts: () => void;
    onInspectAttackIP: (ip: string) => void;
    onQuickBlockIP: (ip: string, reason?: string) => void;
    onFilterAccessByPath: (path: string) => void;
    onOpenLiveAccess: () => void;
    onSelectAccessLog: (log: WAFAccessLog) => void;
    onOpenEnforcementModal?: () => void;
  }

  let {
    overview,
    events,
    accessLogs,
    attachingVhosts,
    overviewMapAction,
    formatBytes,
    formatTime,
    onOpenAttackMap,
    onOpenCveIntel,
    onAttachVhosts,
    onInspectAttackIP,
    onQuickBlockIP,
    onFilterAccessByPath,
    onOpenLiveAccess,
    onSelectAccessLog,
    onOpenEnforcementModal,
  }: Props = $props();

  let unattachedVhosts = $derived(
    Math.max(0, (overview?.enforcement?.total_vhosts_count ?? 0) - (overview?.enforcement?.attached_vhosts_count ?? 0))
  );
</script>

<div class="pg-waf-overview">
  <!-- Peringatan Transparan bila Ada Vhost Nginx yang Belum Meng-include waf-rules.conf -->
  {#if unattachedVhosts > 0}
    <div class="vhost-attach-banner">
      <div class="vhost-attach-info">
        <i class="ri-error-warning-line"></i>
        <div>
          <div class="vhost-attach-title">
            Aturan WAF Baru Terpasang di {overview?.enforcement?.attached_vhosts_count ?? 0} dari {overview?.enforcement?.total_vhosts_count ?? 0} Vhost Nginx
          </div>
          <div class="vhost-attach-desc">
            {unattachedVhosts} berkas virtual host belum menyertakan <code class="mono">include waf-rules.conf</code> sehingga proteksi masih berjalan di mode pemantauan log. Pasang otomatis dengan uji <code class="mono">nginx -t</code> &amp; rollback aman.
          </div>
        </div>
      </div>
      <button
        type="button"
        class="btn btn--primary btn--sm"
        onclick={onAttachVhosts}
        disabled={attachingVhosts}
      >
        <i class="ri-plug-line {attachingVhosts ? 'spin' : ''}"></i>
        {attachingVhosts ? 'Memasang ke Vhost...' : 'Pasang ke Semua Vhost Nginx'}
      </button>
    </div>
  {/if}

  <!-- 4 Kartu KPI Utama (Rata Kiri Presisi) -->
  <div class="overview-grid">
    <div class="waf-kpi-card">
      <div class="waf-kpi-head">
        <span class="waf-kpi-label">Permintaan Hari Ini</span>
        <i class="ri-bar-chart-box-line waf-kpi-icon text-accent"></i>
      </div>
      <div class="waf-kpi-num">{(overview?.today_requests ?? 0).toLocaleString()}</div>
      <div class="waf-kpi-sub">
        Kemarin: {(overview?.yesterday_requests ?? 0).toLocaleString()} · Total Log: <strong>{(overview?.total_requests_all_time ?? 0).toLocaleString()}</strong>
      </div>
    </div>

    <div class="waf-kpi-card is-danger">
      <div class="waf-kpi-head">
        <span class="waf-kpi-label">Ancaman Dicegat Hari Ini</span>
        <i class="ri-shield-cross-line waf-kpi-icon text-danger"></i>
      </div>
      <div class="waf-kpi-num text-danger">{(overview?.malicious_requests ?? 0).toLocaleString()}</div>
      <div class="waf-kpi-sub">
        <span class="trend-badge down"><i class="ri-shield-flash-line"></i> Total: {(overview?.total_events_all_time ?? events.length).toLocaleString()}</span>
        Kemarin: {(overview?.yesterday_malicious ?? 0).toLocaleString()}
      </div>
    </div>

    <div class="waf-kpi-card">
      <div class="waf-kpi-head">
        <span class="waf-kpi-label">QPS &amp; Trafik Live</span>
        <i class="ri-speed-line waf-kpi-icon text-info"></i>
      </div>
      <div class="waf-kpi-num">{overview?.realtime_qps ?? 0} <span class="waf-kpi-unit">/dtk</span></div>
      <div class="waf-kpi-sub">
        Rasio Cegat: {(overview?.intercept_rate ?? 0).toFixed(2)}% · Bandwidth: <strong>{formatBytes(overview?.total_bandwidth_bytes ?? 0)}</strong>
      </div>
    </div>

    <div class="waf-kpi-card">
      <div class="waf-kpi-head">
        <span class="waf-kpi-label">Latensi Origin HTTP</span>
        <i class="ri-timer-line waf-kpi-icon text-accent"></i>
      </div>
      <div class="waf-kpi-num text-accent">{overview?.origin_latency_ms ?? 0} <span class="waf-kpi-unit">ms</span></div>
      <div class="waf-kpi-sub">Waktu respons HEAD ke Nginx lokal</div>
    </div>
  </div>

  <!-- Strip Ringkas Status Pertahanan Proaktif (CVE, Virtual Patch, Auto-Ban, Nginx) -->
  <div class="bento-panel shield-summary-strip">
    <div class="shield-pills">
      <span class="shield-pill {(overview?.active_virtual_patches_count ?? 0) > 0 ? 'ok' : ''}">
        <i class="ri-shield-star-line"></i>
        Virtual Patch: <strong>{overview?.active_virtual_patches_count ?? 0} / 10</strong>
      </span>
      <span class="shield-pill {(overview?.cve_host_matched_count ?? 0) > 0 ? 'warn' : ''}">
        <i class="ri-bug-line"></i>
        CVE Relevan Host: <strong>{overview?.cve_host_matched_count ?? 0}</strong>
      </span>
      <span class="shield-pill {overview?.autoban?.enabled ? 'ok' : ''}">
        <i class="ri-forbid-2-line"></i>
        Auto-Ban: <strong>{overview?.autoban?.enabled ? `Aktif (${overview.autoban.threshold}x/${overview.autoban.window_minutes}m)` : 'Nonaktif'}</strong>
      </span>
      <span class="shield-pill {overview?.enforcement?.nginx_enforced ? 'ok' : 'warn'}">
        <i class="ri-server-line"></i>
        Vhost Terproteksi: <strong>{overview?.enforcement?.attached_vhosts_count ?? 0} / {overview?.enforcement?.total_vhosts_count ?? 0}</strong>
      </span>
    </div>
    <div class="shield-actions">
      <button type="button" class="btn btn--secondary btn--sm" onclick={onOpenCveIntel}>
        <i class="ri-radar-line text-warning"></i> Berita CVE &amp; Virtual Patch
      </button>
      {#if onOpenEnforcementModal}
        <button type="button" class="btn btn--secondary btn--sm" onclick={onOpenEnforcementModal}>
          <i class="ri-file-shield-2-line text-accent"></i> Aturan Nginx ({overview?.enforcement?.active_rules_count ?? overview?.total_rules_count ?? 0})
        </button>
      {/if}
    </div>
  </div>

  <!-- Distribusi Kode Respons HTTP & Telemetri Paket Nginx -->
  {#if overview?.status_breakdown}
    {@const sb = overview.status_breakdown}
    {@const totalSt = Math.max(sb.s2xx + sb.s3xx + sb.s4xx + sb.s5xx, 1)}
    <div class="bento-panel status-telemetry-strip">
      <div class="telemetry-strip-head">
        <div class="strip-title">
          <i class="ri-pulse-line text-accent"></i>
          <span>Distribusi Kode Respons HTTP (Log Akses Nginx)</span>
        </div>
        <div class="strip-badges">
          <span class="status-pill s2xx">
            <span class="dot"></span> 2xx Normal: <strong>{sb.s2xx.toLocaleString()}</strong> ({((sb.s2xx / totalSt) * 100).toFixed(1)}%)
          </span>
          <span class="status-pill s3xx">
            <span class="dot"></span> 3xx Redirect: <strong>{sb.s3xx.toLocaleString()}</strong> ({((sb.s3xx / totalSt) * 100).toFixed(1)}%)
          </span>
          <span class="status-pill s4xx">
            <span class="dot"></span> 4xx Client/Probe: <strong>{sb.s4xx.toLocaleString()}</strong> ({((sb.s4xx / totalSt) * 100).toFixed(1)}%)
          </span>
          <span class="status-pill s5xx">
            <span class="dot"></span> 5xx Server Error: <strong>{sb.s5xx.toLocaleString()}</strong> ({((sb.s5xx / totalSt) * 100).toFixed(1)}%)
          </span>
        </div>
      </div>
      <div class="status-progress-bar">
        <div class="seg seg-2xx" style="--seg-w: {(sb.s2xx / totalSt) * 100}%" title="HTTP 2xx: {sb.s2xx}"></div>
        <div class="seg seg-3xx" style="--seg-w: {(sb.s3xx / totalSt) * 100}%" title="HTTP 3xx: {sb.s3xx}"></div>
        <div class="seg seg-4xx" style="--seg-w: {(sb.s4xx / totalSt) * 100}%" title="HTTP 4xx: {sb.s4xx}"></div>
        <div class="seg seg-5xx" style="--seg-w: {(sb.s5xx / totalSt) * 100}%" title="HTTP 5xx: {sb.s5xx}"></div>
      </div>
    </div>
  {/if}

  <!-- Peta Serangan & Tabel Top Attack IPs -->
  <div class="bento-two-col">
    <div class="bento-panel map-panel">
      <div class="panel-header">
        <div class="panel-title-wrap">
          <i class="ri-earth-line text-accent"></i>
          <h3>Peta Akses &amp; Pemblokiran</h3>
        </div>
        <div class="panel-header-badges">
          <span class="badge-soft">
            <span class="live-mini-dot"></span> {(overview?.total_events_all_time ?? events.length).toLocaleString()} Insiden Tercatat
          </span>
          <button type="button" class="btn-link text-xs text-accent" onclick={onOpenAttackMap}>
            Buka Peta Penuh <i class="ri-arrow-right-line"></i>
          </button>
        </div>
      </div>

      <div class="world-map-wrapper">
        <div class="overview-leaflet-container" use:overviewMapAction={overview?.recent_events}></div>
      </div>
    </div>

    <div class="bento-panel">
      <div class="panel-header">
        <div class="panel-title-wrap">
          <i class="ri-skull-line text-danger"></i>
          <h3>IP Penyerang Teratas</h3>
        </div>
        <span class="text-xs text-muted">IP Publik · Klik Inspeksi/Blokir</span>
      </div>

      <div class="table-container-compact">
        <table class="waf-table-compact">
          <thead>
            <tr>
              <th>Alamat IP</th>
              <th>Jumlah</th>
              <th>Atribusi Lokasi &amp; ISP</th>
              <th class="text-right">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#if !overview?.top_attack_ips || overview.top_attack_ips.length === 0}
              <tr>
                <td colspan="4" class="text-center text-muted">Belum ada rekaman IP penyerang publik</td>
              </tr>
            {:else}
              {#each overview.top_attack_ips as ipItem}
                <tr>
                  <td class="mono font-semibold">{ipItem.ip}</td>
                  <td>
                    <span class="badge-attack-count">{ipItem.count}</span>
                  </td>
                  <td class="text-subtle">
                    <div class="attribution-main">
                      <span class="flag-code">{ipItem.country_code || '??'}</span>
                      <span class="country-name">{ipItem.country || 'Tidak Diketahui'}{ipItem.city ? ` · ${ipItem.city}` : ''}</span>
                    </div>
                    {#if ipItem.isp}
                      <div class="attribution-isp mono" title={ipItem.isp}>{ipItem.isp}</div>
                    {/if}
                  </td>
                  <td class="text-right nowrap">
                    <button
                      type="button"
                      class="btn-icon-xs text-accent"
                      title="Inspeksi Vektor Serangan IP Ini"
                      aria-label="Inspeksi vektor serangan {ipItem.ip}"
                      onclick={() => onInspectAttackIP(ipItem.ip)}
                    >
                      <i class="ri-search-eye-line"></i>
                    </button>
                    <button
                      type="button"
                      class="btn-icon-xs text-danger"
                      title="Blokir IP ke Blacklist"
                      aria-label="Blokir IP {ipItem.ip}"
                      onclick={() => onQuickBlockIP(ipItem.ip, `Top Attacker (${ipItem.count}x)`)}
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

  <!-- 3 Widget Peringkat Bawah -->
  <div class="bottom-three-grid">
    <div class="bento-panel">
      <div class="panel-header">
        <h4><i class="ri-shield-alert-line text-danger"></i> Domain Paling Diserang</h4>
        <span class="text-2xs text-muted">Top 10 Target</span>
      </div>
      <div class="rank-list">
        {#if !overview?.attacked_domains || overview.attacked_domains.length === 0}
          <div class="text-xs text-muted">Belum ada serangan tercatat</div>
        {:else}
          {#each overview.attacked_domains as d, idx}
            <div class="rank-item">
              <span class="rank-num {idx === 0 ? 'first' : ''}">{idx + 1}</span>
              <span class="rank-name mono">{d.domain}</span>
              <span class="rank-val badge-danger-soft">{d.count} blok</span>
            </div>
          {/each}
        {/if}
      </div>
    </div>

    <div class="bento-panel">
      <div class="panel-header">
        <h4><i class="ri-line-chart-line text-accent"></i> Peringkat Trafik Domain</h4>
        <span class="text-2xs text-muted">Per Vhost</span>
      </div>
      <div class="rank-list">
        {#if !overview?.traffic_ranking || overview.traffic_ranking.length === 0}
          <div class="text-xs text-muted">Belum ada log trafik situs</div>
        {:else}
          {#each overview.traffic_ranking.slice(0, 10) as tr, idx}
            <div class="rank-item rank-item-rich">
              <span class="rank-num">{idx + 1}</span>
              <div class="rank-rich-body">
                <div class="rank-rich-top">
                  <span class="rank-name mono font-semibold">{tr.domain}</span>
                  <span class="rank-val text-accent">{tr.requests.toLocaleString()} req</span>
                </div>
                <div class="rank-rich-meta mono text-2xs">
                  <span><i class="ri-hard-drive-2-line"></i> {formatBytes(tr.bytes_sent ?? tr.bandwidth_bytes ?? 0)}</span>
                  {#if tr.status_2xx !== undefined}
                    <span class="text-accent">2xx:{tr.status_2xx}</span>
                    <span class="text-warning">4xx:{tr.status_4xx || 0}</span>
                    {#if (tr.threats || 0) > 0}
                      <span class="text-danger">{tr.threats} ancaman</span>
                    {/if}
                  {/if}
                </div>
              </div>
            </div>
          {/each}
        {/if}
      </div>
    </div>

    <div class="bento-panel">
      <div class="panel-header">
        <h4><i class="ri-file-search-line text-info"></i> Endpoint Paling Diakses</h4>
        <span class="text-2xs text-muted">Klik untuk filter log</span>
      </div>
      <div class="rank-list rank-list-scroll">
        {#if !overview?.visited_pages || overview.visited_pages.length === 0}
          <div class="text-xs text-muted">Belum ada riwayat akses endpoint</div>
        {:else}
          {#each overview.visited_pages as vp, idx}
            <button
              type="button"
              class="rank-item rank-item-clickable {vp.is_threat ? 'rank-threat-row' : ''}"
              onclick={() => onFilterAccessByPath(vp.path)}
              title="Klik untuk melihat seluruh rekaman akses ke {vp.path}"
            >
              <span class="rank-num {vp.is_threat ? 'first' : ''}">{idx + 1}</span>
              <div class="rank-rich-body">
                <div class="rank-rich-top">
                  <span class="rank-name mono text-xs truncate">
                    {#if vp.method}
                      <span class="method-mini {vp.method.toLowerCase()}">{vp.method}</span>
                    {/if}
                    {vp.path}
                  </span>
                  <span class="rank-val badge-soft">{vp.count}x</span>
                </div>
                <div class="rank-rich-meta mono text-2xs">
                  <span class="text-subtle">{vp.domain || vp.site_name || 'default'}</span>
                  {#if vp.last_status || vp.status_code}
                    {@const stCode = vp.last_status || vp.status_code || 200}
                    <span class="status-mini s{Math.floor(stCode / 100)}xx">HTTP {stCode}</span>
                  {/if}
                  {#if vp.bytes_sent || vp.bytes_total}
                    <span>{formatBytes(vp.bytes_sent ?? vp.bytes_total ?? 0)}</span>
                  {/if}
                  {#if vp.is_threat}
                    <span class="text-danger">PROBED</span>
                  {/if}
                </div>
              </div>
            </button>
          {/each}
        {/if}
      </div>
    </div>
  </div>

  <!-- Pratinjau Observatorium Lalu Lintas HTTP Langsung -->
  <div class="bento-panel">
    <div class="panel-header">
      <div class="panel-title-wrap">
        <i class="ri-radar-line text-accent"></i>
        <div>
          <h3>Aliran Lalu Lintas HTTP &amp; Deteksi Ancaman Live</h3>
          <p class="text-xs text-subtle">Permintaan aktual pada log Nginx (pengunjung normal maupun pemindaian bot/eksploitasi)</p>
        </div>
      </div>
      <div class="panel-header-badges">
        <button type="button" class="btn btn--primary btn--sm" onclick={onOpenLiveAccess}>
          <i class="ri-external-link-line"></i> Buka Log Penuh ({accessLogs.length || overview?.recent_access_logs?.length || 0})
        </button>
      </div>
    </div>

    <div class="table-responsive">
      <table class="waf-main-table">
        <thead>
          <tr>
            <th>Waktu</th>
            <th>Status</th>
            <th>IP Klien &amp; Lokasi</th>
            <th>Domain &amp; URL</th>
            <th>Ukuran</th>
            <th>Perangkat / User-Agent</th>
            <th>Klasifikasi WAF</th>
            <th class="text-right">Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#if (overview?.recent_access_logs || accessLogs).length === 0}
            <tr>
              <td colspan="8" class="text-center text-muted">Belum ada aliran akses HTTP terbaru</td>
            </tr>
          {:else}
            {#each (overview?.recent_access_logs || accessLogs).slice(0, 12) as log}
              <tr class="{log.is_threat ? 'row-threat-highlight' : ''}">
                <td class="mono text-xs text-muted nowrap">{formatTime(log.timestamp || log.created_at)}</td>
                <td>
                  <span class="http-status-badge s{Math.floor(log.status_code / 100)}xx">{log.status_code}</span>
                </td>
                <td>
                  <div class="mono font-semibold text-xs">{log.client_ip}</div>
                  <div class="text-2xs text-subtle">
                    <span class="flag-code">{log.country_code || '??'}</span> {log.city ? `${log.city}, ` : ''}{log.country || '-'}
                  </div>
                </td>
                <td>
                  <div class="mono text-xs">
                    <span class="method-mini {log.method.toLowerCase()}">{log.method}</span>
                    <span class="text-accent font-semibold">{log.site_name}</span>
                  </div>
                  <div class="mono text-xs text-subtle url-cell-truncate" title={log.url_path}>{log.url_path}</div>
                </td>
                <td class="mono text-xs text-subtle nowrap">{formatBytes(log.bytes_sent)}</td>
                <td>
                  <div class="text-xs font-medium">{log.browser} · {log.os}</div>
                  <div class="mono text-2xs text-subtle ua-cell-truncate" title={log.user_agent}>{log.user_agent || '-'}</div>
                </td>
                <td>
                  {#if log.is_threat}
                    <span class="badge-attack-tag">{log.attack_type || 'Threat Probe'}</span>
                  {:else}
                    <span class="badge-normal-traffic"><i class="ri-check-line"></i> Normal</span>
                  {/if}
                </td>
                <td class="text-right nowrap">
                  <button
                    type="button"
                    class="btn-icon-xs text-accent"
                    title="Inspeksi Paket HTTP Lengkap"
                    aria-label="Inspeksi paket dari {log.client_ip}"
                    onclick={() => onSelectAccessLog(log)}
                  >
                    <i class="ri-file-search-line"></i>
                  </button>
                  <button
                    type="button"
                    class="btn-icon-xs text-danger"
                    title="Blokir IP ke Blacklist"
                    aria-label="Blokir IP {log.client_ip}"
                    onclick={() => onQuickBlockIP(log.client_ip, log.attack_type || `Akses ke ${log.url_path}`)}
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
