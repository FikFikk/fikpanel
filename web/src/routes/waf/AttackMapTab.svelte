<script lang="ts">
  import type { WAFEvent } from '../../lib/types';
  import '../../styles/pages/waf/attack-map.css';

  // Props untuk tab Attack Map & Forensic Telemetry WAF
  interface Props {
    events: WAFEvent[];
    filteredEvents: WAFEvent[];
    countryStats: { code: string; name: string; count: number; lat: number; lng: number }[];
    loadingEvents: boolean;
    mapSearchQuery: string;
    attackTypeFilter: string;
    countryFilter: string;
    selectedEvent: WAFEvent | null;
    attackMapAction: (node: HTMLElement, eventsParam?: WAFEvent[]) => { update: () => void; destroy: () => void };
    formatTime: (ts: number) => string;
    onLoadEvents: () => void;
    onFlyToServer: () => void;
    onResetMapView: () => void;
    onFlyToCountry: (lat: number, lng: number) => void;
    onQuickBlockIP: (ip: string, reason?: string) => void;
    onClearEvents?: () => void;
  }

  let {
    events,
    filteredEvents,
    countryStats,
    loadingEvents,
    mapSearchQuery = $bindable(),
    attackTypeFilter = $bindable(),
    countryFilter = $bindable(),
    selectedEvent = $bindable(),
    attackMapAction,
    formatTime,
    onLoadEvents,
    onFlyToServer,
    onResetMapView,
    onFlyToCountry,
    onQuickBlockIP,
    onClearEvents,
  }: Props = $props();
</script>

<div class="pg-waf-attack-map bento-panel attack-studio-panel">
  <!-- Studio Header & Quick Intelligence Stats -->
  <div class="panel-header studio-header">
    <div class="panel-title-wrap">
      <div class="radar-live-indicator">
        <span class="live-dot"></span>
        <i class="ri-radar-line text-accent"></i>
      </div>
      <div>
        <h3>Live Global Threat Map & Forensic Telemetry</h3>
        <p class="text-xs text-subtle">Pemetaan geolokasi presisi serangan web, identifikasi perangkat, dan penindakan 1-klik</p>
      </div>
    </div>

    <div class="studio-stats-pills">
      <div class="stat-pill">
        <span class="stat-pill-label">Total Serangan</span>
        <span class="stat-pill-val danger">{events.length}</span>
      </div>
      <div class="stat-pill">
        <span class="stat-pill-label">IP Unik</span>
        <span class="stat-pill-val warning">{new Set(events.map((e) => e.client_ip)).size}</span>
      </div>
      <div class="stat-pill">
        <span class="stat-pill-label">Negara Asal</span>
        <span class="stat-pill-val info">{countryStats.length}</span>
      </div>
      <button class="btn btn-secondary btn-sm" onclick={onLoadEvents} disabled={loadingEvents}>
        <i class="ri-refresh-line {loadingEvents ? 'spin' : ''}"></i> Segarkan
      </button>
      {#if onClearEvents && events.length > 0}
        <button class="btn btn-secondary btn-sm text-danger" onclick={onClearEvents} title="Bersihkan seluruh riwayat insiden serangan">
          <i class="ri-delete-bin-line"></i> Bersihkan Log
        </button>
      {/if}
    </div>
  </div>

  <!-- Studio Filter Toolbar -->
  <div class="studio-toolbar">
    <div class="search-input-wrap">
      <i class="ri-search-line search-icon"></i>
      <input
        type="text"
        bind:value={mapSearchQuery}
        placeholder="Cari IP, negara, kota, ISP, path target, atau perangkat..."
        class="input-dark search-input"
      />
      {#if mapSearchQuery}
        <button class="clear-search-btn" aria-label="Bersihkan pencarian" onclick={() => mapSearchQuery = ''}>&times;</button>
      {/if}
    </div>

    <div class="toolbar-selects">
      <select bind:value={attackTypeFilter} class="input-dark select-sm">
        <option value="all">Semua Jenis Serangan</option>
        <option value="SQL Injection">SQL Injection</option>
        <option value="Sensitive File Probe">Sensitive File Probe</option>
        <option value="Vulnerability Scanner">Vulnerability Scanner</option>
        <option value="Path Traversal">Path Traversal</option>
        <option value="XSS Attack">XSS Attack</option>
        <option value="CC Flood">CC Flood</option>
        <option value="Remote Code Execution (RCE)">RCE / Binary Exploit</option>
        <option value="Bad Bot">Bad Bot</option>
        <option value="Blocked Probe">Blocked Probe</option>
        <option value="Malicious Probe">Malicious Probe</option>
      </select>
    </div>

    <div class="toolbar-actions">
      <button class="btn btn-secondary btn-sm" onclick={onFlyToServer} title="Pusatkan peta ke server host FikPanel">
        <i class="ri-focus-3-line text-accent"></i> Host Server
      </button>
      <button class="btn btn-secondary btn-sm" onclick={onResetMapView} title="Kembalikan tampilan peta global">
        <i class="ri-globe-line"></i> Reset Zoom
      </button>
    </div>
  </div>

  <!-- Country Breakdown Chips -->
  {#if countryStats.length > 0}
    <div class="country-chips-bar">
      <span class="chips-label"><i class="ri-map-pin-line"></i> Asal Serangan:</span>
      <button
        class="country-chip {!countryFilter ? 'active' : ''}"
        onclick={() => countryFilter = ''}
      >
        Semua ({events.length})
      </button>
      {#each countryStats.slice(0, 8) as c}
        <button
          class="country-chip {countryFilter === c.code ? 'active' : ''}"
          onclick={() => {
            countryFilter = countryFilter === c.code ? '' : c.code;
            if (c.lat && c.lng) onFlyToCountry(c.lat, c.lng);
          }}
        >
          <span class="flag-code">{c.code}</span>
          <span>{c.name}</span>
          <span class="chip-badge">{c.count}</span>
        </button>
      {/each}
    </div>
  {/if}

  <!-- Interactive Map Studio & Telemetry Drawer Container -->
  <div class="map-studio-viewport">
    <!-- Canvas Peta Leaflet (Dukungan zoom penuh hingga level jalan) -->
    <div class="attack-map-canvas" use:attackMapAction={filteredEvents}></div>

    <!-- Telemetry Inspector Drawer (Muncul saat marker atau baris log diklik) -->
    {#if selectedEvent}
      <div class="telemetry-inspector-drawer">
        <div class="drawer-header">
          <div class="drawer-title-wrap">
            <span class="badge-action {selectedEvent.action}">{selectedEvent.action.toUpperCase()}</span>
            <div>
              <h4 class="drawer-ip mono">{selectedEvent.client_ip}</h4>
              <span class="drawer-type text-subtle text-xs">{selectedEvent.attack_type}</span>
            </div>
          </div>
          <button class="btn-close" aria-label="Tutup inspeksi" onclick={() => selectedEvent = null}>&times;</button>
        </div>

        <div class="drawer-body">
          <!-- 1. Geolokasi Presisi -->
          <div class="drawer-card">
            <div class="drawer-card-title"><i class="ri-map-pin-2-line text-accent"></i> Lokasi & Geolokasi Presisi</div>
            <div class="drawer-info-grid">
              <div class="info-row">
                <span class="info-lbl">Negara:</span>
                <span class="info-val"><span class="flag-code">{selectedEvent.country_code}</span> {selectedEvent.country}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Provinsi/Region:</span>
                <span class="info-val">{selectedEvent.region || '-'}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Kota:</span>
                <span class="info-val font-semibold">{selectedEvent.city || '-'}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Koordinat GPS:</span>
                <span class="info-val mono text-xs">{selectedEvent.latitude.toFixed(4)}, {selectedEvent.longitude.toFixed(4)}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">ISP / AS Provider:</span>
                <span class="info-val text-xs text-subtle">{selectedEvent.isp || 'Autonomous System'}</span>
              </div>
            </div>
          </div>

          <!-- 2. Fingerprint Perangkat & Klien -->
          <div class="drawer-card">
            <div class="drawer-card-title"><i class="ri-computer-line text-info"></i> Fingerprint Perangkat & Klien</div>
            <div class="drawer-info-grid">
              <div class="info-row">
                <span class="info-lbl">Perangkat:</span>
                <span class="info-val">{selectedEvent.device || 'Desktop (PC)'}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Sistem Operasi:</span>
                <span class="info-val">{selectedEvent.os || 'Linux'}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Peramban/Alat:</span>
                <span class="info-val font-semibold">{selectedEvent.browser || 'Unknown'}</span>
              </div>
              <div class="info-col">
                <span class="info-lbl">Raw User-Agent:</span>
                <div class="ua-box mono text-2xs">{selectedEvent.user_agent || '-'}</div>
              </div>
            </div>
          </div>

          <!-- 3. Forensik Vektor Serangan -->
          <div class="drawer-card">
            <div class="drawer-card-title"><i class="ri-shield-flash-line text-danger"></i> Forensik Vektor Serangan</div>
            <div class="drawer-info-grid">
              <div class="info-row">
                <span class="info-lbl">Target Situs:</span>
                <span class="info-val mono text-accent">{selectedEvent.site_name}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Metode HTTP:</span>
                <span class="info-val mono font-bold">{selectedEvent.method || 'GET'}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Status Kode:</span>
                <span class="info-val mono">{selectedEvent.status_code}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Waktu Deteksi:</span>
                <span class="info-val text-xs mono">{formatTime(selectedEvent.created_at)}</span>
              </div>
              <div class="info-col">
                <span class="info-lbl">Path URL / Query:</span>
                <div class="payload-box mono text-xs">{selectedEvent.url_path}</div>
              </div>
              <div class="info-col">
                <span class="info-lbl">Aturan Pencegatan:</span>
                <div class="rule-box text-xs">{selectedEvent.rule_name || 'Firewall Policy Rule Engine'}</div>
              </div>
            </div>
          </div>
        </div>

        <div class="drawer-actions">
          <button
            class="btn btn-danger btn-sm w-full"
            onclick={() => { if (selectedEvent) { onQuickBlockIP(selectedEvent.client_ip, selectedEvent.attack_type); selectedEvent = null; } }}
          >
            <i class="ri-shield-cross-line"></i> Blokir IP ke Blacklist
          </button>
          <button
            class="btn btn-secondary btn-sm"
            onclick={() => { if (selectedEvent) onFlyToCountry(selectedEvent.latitude, selectedEvent.longitude); }}
          >
            <i class="ri-focus-3-line"></i> Pusatkan Peta ke Lokasi
          </button>
        </div>
      </div>
    {/if}
  </div>

  <!-- Real-time Attack Incidents Table -->
  <div class="panel-header mt-6">
    <div class="panel-title-wrap">
      <i class="ri-history-line text-accent"></i>
      <h4>Feed Insiden Serangan Terkini ({filteredEvents.length} peristiwa)</h4>
    </div>
  </div>
  <div class="table-responsive">
    <table class="waf-main-table">
      <thead>
        <tr>
          <th>Waktu</th>
          <th>IP Penyerang & ISP</th>
          <th>Lokasi Presisi</th>
          <th>Perangkat / OS</th>
          <th>Target Situs</th>
          <th>Jenis Serangan</th>
          <th>Path URL</th>
          <th>Tindakan</th>
          <th class="text-right">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#if filteredEvents.length === 0}
          <tr>
            <td colspan="9" class="text-center p-6 text-muted">
              Tidak ada rekaman serangan yang sesuai dengan filter pencarian.
            </td>
          </tr>
        {:else}
          {#each filteredEvents as ev}
            <tr class="threat-row {selectedEvent?.id === ev.id ? 'row-selected' : ''}">
              <td class="text-muted text-xs mono">{formatTime(ev.created_at)}</td>
              <td>
                <div class="mono font-bold">{ev.client_ip}</div>
                {#if ev.isp}
                  <div class="text-2xs text-subtle truncate max-w-xs">{ev.isp}</div>
                {/if}
              </td>
              <td>
                <div class="geo-cell">
                  <span class="flag-code">{ev.country_code}</span>
                  <span class="geo-city font-medium">{ev.city ? `${ev.city}, ` : ''}{ev.country}</span>
                </div>
              </td>
              <td>
                <div class="device-cell">
                  <span class="text-xs font-semibold">{ev.device || 'Desktop'}</span>
                  <div class="text-2xs text-subtle">{ev.os || 'Linux'} · {ev.browser || 'Browser'}</div>
                </div>
              </td>
              <td class="mono text-accent">
                <span class="method-tag">{ev.method || 'GET'}</span> {ev.site_name}
              </td>
              <td>
                <span class="badge-attack-tag">{ev.attack_type}</span>
              </td>
              <td class="mono text-xs text-subtle truncate max-w-xs" title={ev.url_path}>
                {ev.url_path}
              </td>
              <td>
                <span class="badge-action {ev.action}">{ev.action.toUpperCase()}</span>
              </td>
              <td class="text-right table-action-cell">
                <button
                  class="btn-icon-xs text-accent"
                  title="Inspeksi Telemetri Detail"
                  onclick={() => { selectedEvent = ev; onFlyToCountry(ev.latitude, ev.longitude); }}
                >
                  <i class="ri-search-eye-line"></i>
                </button>
                <button
                  class="btn-icon-xs text-danger"
                  title="Blokir IP ke Blacklist"
                  onclick={() => onQuickBlockIP(ev.client_ip, ev.attack_type)}
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
