<script lang="ts">
  import type { WAFOverview, WAFEvent, WAFAccessLog } from '../../lib/types';
  import '../../styles/pages/waf/overview.css';

  // Props untuk tab Overview WAF
  interface Props {
    overview: WAFOverview | null;
    events: WAFEvent[];
    accessLogs: WAFAccessLog[];
    overviewMapAction: (node: HTMLElement, eventsParam?: WAFEvent[]) => { update: () => void; destroy: () => void };
    formatBytes: (bytes: number) => string;
    formatTime: (ts: number) => string;
    onOpenAttackMap: () => void;
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
    overviewMapAction,
    formatBytes,
    formatTime,
    onOpenAttackMap,
    onInspectAttackIP,
    onQuickBlockIP,
    onFilterAccessByPath,
    onOpenLiveAccess,
    onSelectAccessLog,
    onOpenEnforcementModal,
  }: Props = $props();
</script>

<div class="pg-waf-overview">
  <div class="overview-grid">
    <!-- 4 Quick Stats Cards -->
    <div class="stat-card">
      <div class="stat-head">
        <span class="stat-label">Today request</span>
        <i class="ri-bar-chart-box-line stat-icon text-accent"></i>
      </div>
      <div class="stat-num">{(overview?.today_requests ?? 0).toLocaleString()}</div>
      <div class="stat-sub">
        Yesterday: {(overview?.yesterday_requests ?? 0).toLocaleString()} · Total Log: <strong>{(overview?.total_requests_all_time ?? 0).toLocaleString()}</strong>
      </div>
    </div>

    <div class="stat-card danger">
      <div class="stat-head">
        <span class="stat-label">Malicious request</span>
        <i class="ri-shield-cross-line stat-icon text-danger"></i>
      </div>
      <div class="stat-num text-danger">{(overview?.malicious_requests ?? 0).toLocaleString()}</div>
      <div class="stat-sub">
        <span class="trend-badge down"><i class="ri-shield-flash-line"></i> Dicegat</span>
        Yesterday: {(overview?.yesterday_malicious ?? 0).toLocaleString()}
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-head">
        <span class="stat-label">Real-time QPS & Bandwidth</span>
        <i class="ri-speed-line stat-icon text-info"></i>
      </div>
      <div class="stat-num">{overview?.realtime_qps ?? 0.4} <span class="stat-unit">/s</span></div>
      <div class="stat-sub">
        Pencegatan: {(overview?.intercept_rate ?? 0).toFixed(2)}% · Trafik: <strong>{formatBytes(overview?.total_bandwidth_bytes ?? 0)}</strong>
      </div>
    </div>

    <div class="stat-card">
      <div class="stat-head">
        <span class="stat-label">Real-time origin response</span>
        <i class="ri-timer-line stat-icon text-success"></i>
      </div>
      <div class="stat-num text-success">{overview?.origin_latency_ms ?? 4} <span class="stat-unit">ms</span></div>
      <div class="stat-sub">Kecepatan latensi origin Nginx normal</div>
    </div>
  </div>

  <!-- HTTP Status Breakdown & Deep Telemetry Bar -->
  {#if overview?.status_breakdown}
    {@const sb = overview.status_breakdown}
    {@const totalSt = Math.max(sb.s2xx + sb.s3xx + sb.s4xx + sb.s5xx, 1)}
    <div class="bento-panel status-telemetry-strip">
      <div class="telemetry-strip-head">
        <div class="strip-title">
          <i class="ri-pulse-line text-accent"></i>
          <span>Distribusi Kode Respons HTTP & Telemetri Paket Nginx (Real-Time Log)</span>
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
          {#if onOpenEnforcementModal}
            <button type="button" class="btn btn-secondary btn-sm" onclick={onOpenEnforcementModal}>
              <i class="ri-file-shield-2-line text-accent"></i> Lihat Aturan Nginx WAF ({overview?.enforcement?.active_rules_count ?? overview?.total_rules_count ?? 0})
            </button>
          {/if}
        </div>
      </div>
      <div class="status-progress-bar">
        <div class="seg seg-2xx" style="width: {(sb.s2xx / totalSt) * 100}%" title="HTTP 2xx: {sb.s2xx}"></div>
        <div class="seg seg-3xx" style="width: {(sb.s3xx / totalSt) * 100}%" title="HTTP 3xx: {sb.s3xx}"></div>
        <div class="seg seg-4xx" style="width: {(sb.s4xx / totalSt) * 100}%" title="HTTP 4xx: {sb.s4xx}"></div>
        <div class="seg seg-5xx" style="width: {(sb.s5xx / totalSt) * 100}%" title="HTTP 5xx: {sb.s5xx}"></div>
      </div>
    </div>
  {/if}

  <!-- Map & Top Attack Table Section -->
  <div class="bento-two-col">
    <!-- 2D Interactive World Map (Leaflet Dark Matter) -->
    <div class="bento-panel map-panel">
      <div class="panel-header">
        <div class="panel-title-wrap">
          <i class="ri-earth-line text-accent"></i>
          <h3>Access / Block Map</h3>
        </div>
        <div class="panel-header-badges">
          <span class="badge-soft">
            <span class="live-mini-dot"></span> {(overview?.recent_events?.length || events.length || 0)} Vektor Serangan Aktif
          </span>
          <button class="btn-link text-xs text-accent" onclick={onOpenAttackMap}>
            Buka Peta Penuh <i class="ri-arrow-right-line"></i>
          </button>
        </div>
      </div>

      <div class="world-map-wrapper">
        <div class="overview-leaflet-container" use:overviewMapAction={overview?.recent_events}></div>
      </div>
    </div>

    <!-- Top Attack IPs Table -->
    <div class="bento-panel">
      <div class="panel-header">
        <div class="panel-title-wrap">
          <i class="ri-skull-line text-danger"></i>
          <h3>Top Attack IPs</h3>
        </div>
        <span class="text-xs text-muted">Klik untuk Inspeksi / Blokir</span>
      </div>

      <div class="table-container-compact">
        <table class="waf-table-compact">
          <thead>
            <tr>
              <th>Attack IP</th>
              <th>Times</th>
              <th>IP attribution</th>
              <th class="text-right">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#if !overview?.top_attack_ips || overview.top_attack_ips.length === 0}
              <tr>
                <td colspan="4" class="text-center text-muted">Belum ada rekaman IP penyerang</td>
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
                      <span class="flag-code">{ipItem.country_code}</span> 
                      <span class="country-name">{ipItem.country}{ipItem.city ? ` · ${ipItem.city}` : ''}</span>
                    </div>
                    {#if ipItem.isp}
                      <div class="attribution-isp mono truncate max-w-xs">{ipItem.isp}</div>
                    {/if}
                  </td>
                  <td class="text-right nowrap">
                    <button
                      class="btn-icon-xs text-accent"
                      title="Inspeksi Vektor Serangan IP Ini"
                      onclick={() => onInspectAttackIP(ipItem.ip)}
                    >
                      <i class="ri-search-eye-line"></i>
                    </button>
                    <button
                      class="btn-icon-xs text-danger"
                      title="Blokir IP ke Blacklist"
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

  <!-- 3 Bottom Widget Cards: Attacked domains, Traffic ranking, Visited pages -->
  <div class="bottom-three-grid">
    <!-- Attacked Domains TOP 10 -->
    <div class="bento-panel">
      <div class="panel-header">
        <h4><i class="ri-shield-alert-line text-danger"></i> Attacked domains TOP10</h4>
        <span class="text-2xs text-muted">Target Serangan</span>
      </div>
      <div class="rank-list">
        {#if overview?.attacked_domains}
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

    <!-- Traffic Ranking TOP 10 (Enhanced with Bandwidth & Status Breakdown) -->
    <div class="bento-panel">
      <div class="panel-header">
        <h4><i class="ri-line-chart-line text-accent"></i> Traffic ranking TOP10</h4>
        <span class="text-2xs text-muted">Per Domain Vhost</span>
      </div>
      <div class="rank-list">
        {#if overview?.traffic_ranking}
          {#each overview.traffic_ranking as tr, idx}
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
                    <span class="text-success">2xx:{tr.status_2xx}</span>
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

    <!-- Visited Pages TOP 15 (Enhanced with Method, Domain, Status, Bytes) -->
    <div class="bento-panel">
      <div class="panel-header">
        <h4><i class="ri-file-search-line text-info"></i> Visited pages & Endpoints</h4>
        <span class="text-2xs text-muted">Klik untuk filter log</span>
      </div>
      <div class="rank-list rank-list-scroll">
        {#if overview?.visited_pages}
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
                    <span class="text-danger font-bold">PROBED</span>
                  {/if}
                </div>
              </div>
            </button>
          {/each}
        {/if}
      </div>
    </div>
  </div>

  <!-- Live HTTP Access & Threat Stream Preview directly inside Overview -->
  <div class="bento-panel mt-2">
    <div class="panel-header">
      <div class="panel-title-wrap">
        <i class="ri-radar-line text-accent"></i>
        <div>
          <h3>Live HTTP Access & Threat Observatory (Semua Endpoint yang Diakses)</h3>
          <p class="text-xs text-subtle">Menampilkan lalu lintas HTTP nyata yang masuk ke Nginx (baik pengunjung normal maupun percobaan eksploitasi bot/scanner)</p>
        </div>
      </div>
      <div class="panel-header-badges">
        <button
          class="btn btn-primary btn-sm"
          onclick={onOpenLiveAccess}
        >
          <i class="ri-External-link-line"></i> Buka Observatorium Penuh ({accessLogs.length || overview?.recent_access_logs?.length || 0} Log)
        </button>
      </div>
    </div>

    <div class="table-responsive">
      <table class="waf-main-table">
        <thead>
          <tr>
            <th>Waktu</th>
            <th>Status</th>
            <th>Klien IP & Lokasi</th>
            <th>Domain & Endpoint URL</th>
            <th>Ukuran</th>
            <th>Perangkat / User-Agent</th>
            <th>Klasifikasi WAF</th>
            <th class="text-right">Inspeksi / Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#each (overview?.recent_access_logs || accessLogs).slice(0, 12) as log}
            <tr class="{log.is_threat ? 'row-threat-highlight' : ''}">
              <td class="mono text-xs text-muted nowrap">{formatTime(log.timestamp || log.created_at)}</td>
              <td>
                <span class="http-status-badge s{Math.floor(log.status_code / 100)}xx">{log.status_code}</span>
              </td>
              <td>
                <div class="mono font-semibold text-xs">{log.client_ip}</div>
                <div class="text-2xs text-subtle">
                  <span class="flag-code">{log.country_code}</span> {log.city ? `${log.city}, ` : ''}{log.country}
                </div>
              </td>
              <td>
                <div class="mono text-xs">
                  <span class="method-mini {log.method.toLowerCase()}">{log.method}</span>
                  <span class="text-accent font-semibold">{log.site_name}</span>
                </div>
                <div class="mono text-xs text-subtle truncate max-w-md" title={log.url_path}>{log.url_path}</div>
              </td>
              <td class="mono text-xs text-subtle nowrap">{formatBytes(log.bytes_sent)}</td>
              <td>
                <div class="text-xs font-medium">{log.browser} · {log.os}</div>
                <div class="mono text-2xs text-subtle truncate max-w-xs" title={log.user_agent}>{log.user_agent || '-'}</div>
              </td>
              <td>
                {#if log.is_threat}
                  <span class="badge-attack-tag">{log.attack_type || 'Threat Probe'}</span>
                {:else}
                  <span class="badge-normal-traffic"><i class="ri-check-line"></i> Normal Traffic</span>
                {/if}
              </td>
              <td class="text-right nowrap">
                <button
                  class="btn-icon-xs text-accent"
                  title="Inspeksi Paket HTTP Lengkap"
                  onclick={() => onSelectAccessLog(log)}
                >
                  <i class="ri-file-search-line"></i>
                </button>
                <button
                  class="btn-icon-xs text-danger"
                  title="Blokir IP ke Blacklist"
                  onclick={() => onQuickBlockIP(log.client_ip, log.attack_type || `Akses ke ${log.url_path}`)}
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
</div>
