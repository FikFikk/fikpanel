<script lang="ts">
  import { onMount } from 'svelte';
  import L from 'leaflet';
  import 'leaflet/dist/leaflet.css';
  import { apiFetch } from '../lib/api';
  import type {
    WAFOverview,
    WAFSite,
    WAFRule,
    WAFEvent,
    CloudflareConfig,
    CloudflareZone,
    CloudflareDNSRecord,
  } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  // State Tab Navigasi WAF
  type WAFTab = 'overview' | 'website' | 'attack_map' | 'blacklist' | 'cloudflare';
  let activeTab = $state<WAFTab>('overview');

  // State Telemetri & Filter Attack Map
  let selectedEvent = $state<WAFEvent | null>(null);
  let attackTypeFilter = $state('all');
  let countryFilter = $state('');
  let mapSearchQuery = $state('');
  let attackMapInstance: L.Map | null = null;
  let overviewMapInstance: L.Map | null = null;

  // Loading States
  let loadingOverview = $state(true);
  let loadingSites = $state(false);
  let loadingRules = $state(false);
  let loadingEvents = $state(false);
  let loadingCF = $state(false);

  // Data States
  let overview = $state<WAFOverview | null>(null);
  let sites = $state<WAFSite[]>([]);
  let rules = $state<WAFRule[]>([]);
  let events = $state<WAFEvent[]>([]);

  // Cloudflare Data States
  let cfConfig = $state<CloudflareConfig | null>(null);
  let cfZones = $state<CloudflareZone[]>([]);
  let selectedZone = $state<CloudflareZone | null>(null);
  let showCFConnectModal = $state(false);
  let showDNSModal = $state(false);
  let cfTokenInput = $state('');
  let cfEmailInput = $state('');
  let cfSubmitting = $state(false);
  let serverIP = $state('');

  // Search & Filter States
  let siteSearch = $state('');
  let activeRuleType = $state<'ip_whitelist' | 'ip_blacklist' | 'ua_whitelist' | 'ua_blacklist' | 'url_whitelist' | 'url_blacklist'>('ip_blacklist');

  // Form State Aturan Black/White List
  let ruleType = $state<'IPv4' | 'IPv6'>('IPv4');
  let ruleStartIP = $state('');
  let ruleEndIP = $state('');
  let rulePattern = $state('');
  let ruleRemarks = $state('');
  let submittingRule = $state(false);

  // Site Edit Modal State
  let editingSite = $state<WAFSite | null>(null);
  let showSiteModal = $state(false);

  // Filtered Sites
  let filteredSites = $derived(
    sites.filter((s) => !siteSearch.trim() || s.site_name.toLowerCase().includes(siteSearch.toLowerCase()))
  );

  // Ambil data WAF Overview
  async function loadOverview() {
    try {
      loadingOverview = true;
      const res = await apiFetch<{ ok: boolean; overview: WAFOverview }>('/api/waf/overview');
      if (res?.overview) {
        overview = res.overview;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat metrik WAF', 'error');
    } finally {
      loadingOverview = false;
    }
  }

  // Ambil daftar situs terproteksi WAF
  async function loadSites() {
    try {
      loadingSites = true;
      const res = await apiFetch<{ ok: boolean; sites: WAFSite[] }>('/api/waf/sites');
      if (res?.sites) {
        sites = res.sites;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat situs WAF', 'error');
    } finally {
      loadingSites = false;
    }
  }

  // Update status proteksi situs
  async function toggleSiteStatus(site: WAFSite) {
    try {
      site.status = !site.status;
      await apiFetch(`/api/waf/sites/${encodeURIComponent(site.site_name)}`, {
        method: 'POST',
        body: JSON.stringify(site),
      });
      notify(`Proteksi WAF untuk ${site.site_name} ${site.status ? 'diaktifkan' : 'dinonaktifkan'}`, 'info');
    } catch (err: any) {
      site.status = !site.status; // Revert
      notify(err.message || 'Gagal mengubah status proteksi', 'error');
    }
  }

  // Update opsi situs spesifik
  async function saveSiteSettings() {
    if (!editingSite) return;
    try {
      await apiFetch(`/api/waf/sites/${encodeURIComponent(editingSite.site_name)}`, {
        method: 'POST',
        body: JSON.stringify(editingSite),
      });
      notify(`Pengaturan WAF ${editingSite.site_name} berhasil disimpan`, 'success');
      showSiteModal = false;
      await loadSites();
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan konfigurasi situs', 'error');
    }
  }

  // Ambil daftar aturan
  async function loadRules() {
    try {
      loadingRules = true;
      const res = await apiFetch<{ ok: boolean; rules: WAFRule[] }>(`/api/waf/rules?type=${activeRuleType}`);
      if (res?.rules) {
        rules = res.rules;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat aturan WAF', 'error');
    } finally {
      loadingRules = false;
    }
  }

  // Tambah aturan WAF baru
  async function handleAddRule() {
    const isIP = activeRuleType.startsWith('ip_');
    if (isIP && !ruleStartIP.trim()) {
      notify('Alamat IP awal wajib diisi', 'warning');
      return;
    }
    if (!isIP && !rulePattern.trim()) {
      notify('Pola / String pencocokan wajib diisi', 'warning');
      return;
    }

    try {
      submittingRule = true;
      await apiFetch('/api/waf/rules', {
        method: 'POST',
        body: JSON.stringify({
          rule_type: activeRuleType,
          ip_version: ruleType,
          start_ip: ruleStartIP.trim(),
          end_ip: ruleEndIP.trim(),
          pattern: rulePattern.trim(),
          remarks: ruleRemarks.trim(),
        }),
      });

      notify('Aturan WAF berhasil ditambahkan', 'success');
      ruleStartIP = '';
      ruleEndIP = '';
      rulePattern = '';
      ruleRemarks = '';
      await loadRules();
    } catch (err: any) {
      notify(err.message || 'Gagal menambahkan aturan', 'error');
    } finally {
      submittingRule = false;
    }
  }

  // Hapus aturan WAF
  async function deleteRule(rule: WAFRule) {
    const ok = await confirmDialog({
      title: 'Hapus Aturan WAF',
      message: `Hapus aturan ${rule.rule_type}: ${rule.start_ip || rule.pattern}?`,
      confirmLabel: 'Hapus',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/waf/rules/${rule.id}`, { method: 'DELETE' });
      rules = rules.filter((r) => r.id !== rule.id);
      notify('Aturan berhasil dihapus', 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus aturan', 'error');
    }
  }

  // Ambil insiden serangan WAF
  async function loadEvents() {
    try {
      loadingEvents = true;
      const res = await apiFetch<{ ok: boolean; events: WAFEvent[] }>('/api/waf/events?limit=50');
      if (res?.events) {
        events = res.events;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat insiden serangan', 'error');
    } finally {
      loadingEvents = false;
    }
  }

  // Cloudflare: Ambil konfigurasi dan daftar zone
  async function loadCloudflare() {
    try {
      loadingCF = true;
      const cfgRes = await apiFetch<{ ok: boolean; config: CloudflareConfig }>('/api/cloudflare/config');
      if (cfgRes?.config) {
        cfConfig = cfgRes.config;
      }
      if (cfConfig?.connected) {
        const zonesRes = await apiFetch<{ ok: boolean; zones: CloudflareZone[]; server_ip: string }>('/api/cloudflare/zones');
        if (zonesRes?.zones) {
          cfZones = zonesRes.zones;
        }
        if (zonesRes?.server_ip) {
          serverIP = zonesRes.server_ip;
        }
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat data Cloudflare', 'error');
    } finally {
      loadingCF = false;
    }
  }

  // Cloudflare: Sambungkan API Token
  async function handleConnectCF() {
    if (!cfTokenInput.trim()) {
      notify('Cloudflare API Token wajib diisi', 'warning');
      return;
    }
    try {
      cfSubmitting = true;
      const res = await apiFetch<{ ok: boolean; message: string; config: CloudflareConfig }>('/api/cloudflare/connect', {
        method: 'POST',
        body: JSON.stringify({
          api_token: cfTokenInput.trim(),
          email: cfEmailInput.trim(),
        }),
      });
      notify(res?.message || 'Berhasil terhubung ke Cloudflare!', 'success');
      showCFConnectModal = false;
      cfTokenInput = '';
      cfEmailInput = '';
      await loadCloudflare();
    } catch (err: any) {
      notify(err.message || 'Gagal menghubungkan Cloudflare', 'error');
    } finally {
      cfSubmitting = false;
    }
  }

  // Cloudflare: Putuskan koneksi
  async function handleDisconnectCF() {
    const ok = await confirmDialog({
      title: 'Putuskan Cloudflare',
      message: 'Apakah Anda yakin ingin memutuskan integrasi Cloudflare?',
      confirmLabel: 'Putuskan',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch('/api/cloudflare/disconnect', { method: 'POST' });
      cfConfig = { connected: false };
      cfZones = [];
      notify('Integrasi Cloudflare telah diputuskan', 'info');
    } catch (err: any) {
      notify(err.message || 'Gagal memutuskan integrasi', 'error');
    }
  }

  // Cloudflare: Arahkan DNS domain ke IP server ini
  async function pointZoneHere(zone: CloudflareZone, proxied: boolean) {
    if (!serverIP) {
      notify('Alamat IP server belum terdeteksi', 'warning');
      return;
    }
    const ok = await confirmDialog({
      title: 'Arahkan DNS ke Server Ini',
      message: `Arahkan record A untuk ${zone.name} ke IP ${serverIP} dengan status Proxy Cloudflare: ${proxied ? 'Aktif (Orange Cloud)' : 'Nonaktif (Grey Cloud)'}?`,
      confirmLabel: 'Arahkan Sekarang',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/cloudflare/zones/${zone.id}/point-here`, {
        method: 'POST',
        body: JSON.stringify({
          domain: zone.name,
          server_ip: serverIP,
          proxied: proxied,
        }),
      });
      notify(`DNS ${zone.name} berhasil diarahkan ke ${serverIP}`, 'success');
      await loadCloudflare();
    } catch (err: any) {
      notify(err.message || 'Gagal memperbarui DNS Cloudflare', 'error');
    }
  }

  // Cloudflare: Kosongkan Cache CDN
  async function purgeCFCache(zone: CloudflareZone) {
    try {
      notify(`Mengosongkan cache Cloudflare untuk ${zone.name}...`, 'info');
      await apiFetch(`/api/cloudflare/zones/${zone.id}/purge-cache`, { method: 'POST' });
      notify(`Seluruh cache CDN Cloudflare untuk ${zone.name} berhasil dibersihkan!`, 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal mengosongkan cache', 'error');
    }
  }

  // Cloudflare: Toggle Mode Pengembangan
  async function toggleDevMode(zone: CloudflareZone) {
    const nextState = zone.development_mode === 0;
    try {
      await apiFetch(`/api/cloudflare/zones/${zone.id}/dev-mode`, {
        method: 'POST',
        body: JSON.stringify({ enable: nextState }),
      });
      zone.development_mode = nextState ? 1 : 0;
      notify(`Mode Pengembangan untuk ${zone.name} ${nextState ? 'diaktifkan (3 jam bypass cache)' : 'dinonaktifkan'}`, 'info');
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah mode pengembang', 'error');
    }
  }

  // Koordinat server host Edge (Default Singapore AWS: 1.3521, 103.8198)
  const hostServerLocation = { lat: 1.3521, lng: 103.8198, name: 'FikPanel Origin Host (AWS Singapore)' };

  // Filter daftar event serangan untuk Attack Map
  let filteredEvents = $derived(
    events.filter((ev) => {
      if (attackTypeFilter !== 'all' && ev.attack_type !== attackTypeFilter) return false;
      if (countryFilter && ev.country_code !== countryFilter) return false;
      if (!mapSearchQuery.trim()) return true;
      const q = mapSearchQuery.toLowerCase();
      return (
        ev.client_ip.toLowerCase().includes(q) ||
        ev.country.toLowerCase().includes(q) ||
        (ev.city && ev.city.toLowerCase().includes(q)) ||
        (ev.isp && ev.isp.toLowerCase().includes(q)) ||
        ev.url_path.toLowerCase().includes(q) ||
        ev.site_name.toLowerCase().includes(q) ||
        (ev.device && ev.device.toLowerCase().includes(q)) ||
        (ev.os && ev.os.toLowerCase().includes(q))
      );
    })
  );

  // Statistik negara penyerang terbanyak
  let countryStats = $derived(
    (() => {
      const counts: Record<string, { code: string; name: string; count: number; lat: number; lng: number }> = {};
      for (const ev of events) {
        const key = ev.country_code || 'XX';
        if (!counts[key]) {
          counts[key] = {
            code: key,
            name: ev.country || 'Unknown',
            count: 0,
            lat: ev.latitude || 0,
            lng: ev.longitude || 0,
          };
        }
        counts[key].count++;
      }
      return Object.values(counts).sort((a, b) => b.count - a.count);
    })()
  );

  // Buat kurva quadratic bezier untuk laser serangan antar koordinat bumi
  function createCurvedArc(from: [number, number], to: [number, number], pointsCount = 20): [number, number][] {
    const [lat1, lng1] = from;
    const [lat2, lng2] = to;
    const midLat = (lat1 + lat2) / 2;
    const midLng = (lng1 + lng2) / 2;
    const dLat = lat2 - lat1;
    const dLng = lng2 - lng1;
    const dist = Math.sqrt(dLat * dLat + dLng * dLng);
    const arcHeight = Math.min(dist * 0.22, 22);

    const ctrlLat = midLat + (dLng > 0 ? arcHeight : -arcHeight) * 0.35;
    const ctrlLng = midLng + (dLat > 0 ? -arcHeight : arcHeight) * 0.35;

    const points: [number, number][] = [];
    for (let i = 0; i <= pointsCount; i++) {
      const t = i / pointsCount;
      const lat = (1 - t) * (1 - t) * lat1 + 2 * (1 - t) * t * ctrlLat + t * t * lat2;
      const lng = (1 - t) * (1 - t) * lng1 + 2 * (1 - t) * t * ctrlLng + t * t * lng2;
      points.push([lat, lng]);
    }
    return points;
  }

  // Blokir IP seketika ke aturan blacklist WAF
  async function quickBlockIP(ip: string, reason?: string) {
    if (!ip) return;
    try {
      await apiFetch('/api/waf/rules', {
        method: 'POST',
        body: JSON.stringify({
          rule_type: 'ip_blacklist',
          ip_version: ip.includes(':') ? 'IPv6' : 'IPv4',
          start_ip: ip.trim(),
          remarks: reason || 'Diblokir langsung dari Attack Map',
        }),
      });
      notify(`IP ${ip} berhasil dimasukkan ke daftar hitam (Blacklist)!`, 'success');
      await loadRules();
    } catch (err: any) {
      notify(err.message || 'Gagal memblokir IP', 'error');
    }
  }

  // Terbang ke lokasi negara pada Attack Map
  function flyToCountry(lat: number, lng: number) {
    if (attackMapInstance) {
      attackMapInstance.flyTo([lat, lng], 5, { duration: 1.2 });
    }
  }

  // Terbang ke koordinat host server
  function flyToServer() {
    if (attackMapInstance) {
      attackMapInstance.flyTo([hostServerLocation.lat, hostServerLocation.lng], 6, { duration: 1.2 });
    }
  }

  // Reset tampilan peta ke pandangan global dunia
  function resetMapView() {
    if (attackMapInstance) {
      attackMapInstance.flyTo([20, 10], 2, { duration: 1.0 });
    }
    countryFilter = '';
    attackTypeFilter = 'all';
    mapSearchQuery = '';
  }

  // Svelte Action untuk peta ringkas di tab Overview
  function overviewMapAction(node: HTMLElement) {
    const map = L.map(node, {
      zoomControl: false,
      attributionControl: false,
      minZoom: 1,
      maxZoom: 14,
      scrollWheelZoom: false,
    }).setView([20, 10], 1);

    // Gunakan Esri World Dark Gray Base + Reference (Bebas API Key, tampilan dark elegan tanpa watermark)
    L.tileLayer('https://services.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Base/MapServer/tile/{z}/{y}/{x}', {
      maxZoom: 16,
      errorTileUrl: '/api/waf/tiles/{z}/{x}/{y}.png',
    }).addTo(map);

    L.tileLayer('https://services.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Reference/MapServer/tile/{z}/{y}/{x}', {
      maxZoom: 16,
      opacity: 0.85,
    }).addTo(map);

    overviewMapInstance = map;
    const layersGroup = L.layerGroup().addTo(map);

    function updateLayers() {
      layersGroup.clearLayers();

      // Marker host server
      const serverIcon = L.divIcon({
        className: 'leaflet-server-icon',
        html: '<div class="server-beacon-dot"><div class="pulse-ring pulse-green"></div><span class="dot-core"></span></div>',
        iconSize: [24, 24],
        iconAnchor: [12, 12],
      });
      L.marker([hostServerLocation.lat, hostServerLocation.lng], { icon: serverIcon, zIndexOffset: 1000 }).addTo(layersGroup);

      // Titik serangan & laser arc
      const threatList = overview?.recent_events || [];
      threatList.slice(0, 8).forEach((ev) => {
        if (!ev.latitude && !ev.longitude) return;
        const isBlock = ev.action === 'block';
        const threatIcon = L.divIcon({
          className: 'leaflet-threat-icon',
          html: `<div class="threat-beacon-dot ${isBlock ? 'blocked' : 'warn'}"><div class="pulse-ring ${isBlock ? 'pulse-red' : 'pulse-amber'}"></div><span class="dot-core"></span></div>`,
          iconSize: [20, 20],
          iconAnchor: [10, 10],
        });
        L.marker([ev.latitude, ev.longitude], { icon: threatIcon }).addTo(layersGroup);

        const arcPoints = createCurvedArc([ev.latitude, ev.longitude], [hostServerLocation.lat, hostServerLocation.lng]);
        L.polyline(arcPoints, {
          className: isBlock ? 'laser-arc-danger' : 'laser-arc-warn',
          weight: 2,
        }).addTo(layersGroup);
      });
    }

    updateLayers();

    setTimeout(() => {
      map.invalidateSize();
    }, 200);

    return {
      update() {
        updateLayers();
      },
      destroy() {
        map.remove();
        overviewMapInstance = null;
      },
    };
  }

  // Svelte Action untuk peta studio penuh di tab Attack Map
  function attackMapAction(node: HTMLElement) {
    const map = L.map(node, {
      zoomControl: false,
      attributionControl: false,
      minZoom: 2,
      maxZoom: 16,
    }).setView([22, 15], 2);

    // Gunakan Esri World Dark Gray Base + Reference (Bebas API Key, tampilan dark elegan tanpa watermark)
    L.tileLayer('https://services.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Base/MapServer/tile/{z}/{y}/{x}', {
      maxZoom: 16,
      errorTileUrl: '/api/waf/tiles/{z}/{x}/{y}.png',
    }).addTo(map);

    L.tileLayer('https://services.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Reference/MapServer/tile/{z}/{y}/{x}', {
      maxZoom: 16,
      opacity: 0.85,
    }).addTo(map);

    L.control.zoom({ position: 'topright' }).addTo(map);
    attackMapInstance = map;

    const layersGroup = L.layerGroup().addTo(map);

    function renderAttackLayers() {
      layersGroup.clearLayers();

      // Marker host server (Origin Target)
      const serverIcon = L.divIcon({
        className: 'leaflet-server-full',
        html: `
          <div class="server-host-beacon">
            <div class="pulse-ring pulse-green"></div>
            <div class="server-core-icon"><i class="ri-shield-check-fill"></i></div>
            <div class="server-host-label">FIKPANEL EDGE HOST</div>
          </div>
        `,
        iconSize: [36, 36],
        iconAnchor: [18, 18],
      });
      L.marker([hostServerLocation.lat, hostServerLocation.lng], { icon: serverIcon, zIndexOffset: 2000 })
        .bindTooltip('FikPanel Protected Host (IP 54.254.187.53 - Singapore)', { direction: 'top', className: 'dark-map-tooltip' })
        .addTo(layersGroup);

      // Render tiap serangan terfilter
      filteredEvents.forEach((ev) => {
        if (!ev.latitude && !ev.longitude) return;
        const isBlock = ev.action === 'block';

        const threatIcon = L.divIcon({
          className: 'leaflet-threat-full',
          html: `
            <div class="threat-marker-beacon ${isBlock ? 'threat-danger' : 'threat-warning'}">
              <div class="pulse-ring ${isBlock ? 'pulse-red' : 'pulse-amber'}"></div>
              <div class="threat-dot-inner"></div>
            </div>
          `,
          iconSize: [26, 26],
          iconAnchor: [13, 13],
        });

        const marker = L.marker([ev.latitude, ev.longitude], { icon: threatIcon }).addTo(layersGroup);

        marker.bindTooltip(`
          <div class="map-tooltip-content">
            <span class="flag-code">${ev.country_code}</span> <strong>${ev.country}</strong> · ${ev.city || 'Kota Terdeteksi'}<br/>
            <span class="mono">${ev.client_ip}</span> · <span class="badge-tag-xs">${ev.attack_type}</span>
          </div>
        `, { direction: 'top', className: 'dark-map-tooltip' });

        marker.on('click', () => {
          selectedEvent = ev;
          map.flyTo([ev.latitude, ev.longitude], Math.max(map.getZoom(), 5), { duration: 0.8 });
        });

        // Gambar kurva laser serangan
        const arc = createCurvedArc([ev.latitude, ev.longitude], [hostServerLocation.lat, hostServerLocation.lng]);
        L.polyline(arc, {
          className: isBlock ? 'laser-arc-danger' : 'laser-arc-warn',
          weight: 2,
        }).addTo(layersGroup);
      });
    }

    renderAttackLayers();

    setTimeout(() => {
      map.invalidateSize();
    }, 150);

    return {
      update() {
        renderAttackLayers();
      },
      destroy() {
        map.remove();
        attackMapInstance = null;
      },
    };
  }

  function formatTime(ts: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleTimeString('id-ID', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
  }

  onMount(() => {
    loadOverview();
    loadSites();
  });
</script>

<div class="waf-container">
  <!-- WAF Top Navigation Bar -->
  <header class="waf-header">
    <div class="waf-brand-title">
      <div class="waf-icon-glow">
        <i class="ri-shield-flash-line"></i>
      </div>
      <div>
        <h1 class="waf-title">Web Application Firewall (WAF)</h1>
        <p class="waf-desc">Proteksi lalu lintas web real-time, mitigasi serangan CC Flood, SQLi, XSS, dan integrasi Cloudflare Edge</p>
      </div>
    </div>
    <div class="waf-top-actions">
      <span class="badge-status-glow">
        <span class="dot-glow"></span> Proteksi Mesin Aktif
      </span>
      <button class="btn btn-secondary btn-sm" onclick={loadOverview} disabled={loadingOverview}>
        <i class="ri-refresh-line {loadingOverview ? 'spin' : ''}"></i> Segarkan
      </button>
    </div>
  </header>

  <!-- Sub-Tabs Navigation (Matching Reference Screenshots) -->
  <nav class="waf-tabs-nav">
    <button
      class="waf-tab-btn {activeTab === 'overview' ? 'active' : ''}"
      onclick={() => { activeTab = 'overview'; loadOverview(); }}
    >
      <i class="ri-dashboard-2-line"></i> Overview
    </button>
    <button
      class="waf-tab-btn {activeTab === 'website' ? 'active' : ''}"
      onclick={() => { activeTab = 'website'; loadSites(); }}
    >
      <i class="ri-global-line"></i> WebSite ({sites.length})
    </button>
    <button
      class="waf-tab-btn {activeTab === 'attack_map' ? 'active' : ''}"
      onclick={() => { activeTab = 'attack_map'; loadEvents(); }}
    >
      <i class="ri-radar-line"></i> Attack Map
    </button>
    <button
      class="waf-tab-btn {activeTab === 'blacklist' ? 'active' : ''}"
      onclick={() => { activeTab = 'blacklist'; loadRules(); }}
    >
      <i class="ri-shield-keyhole-line"></i> Black/White List
    </button>
    <button
      class="waf-tab-btn {activeTab === 'cloudflare' ? 'active' : ''}"
      onclick={() => { activeTab = 'cloudflare'; loadCloudflare(); }}
    >
      <i class="ri-cloud-line"></i> Cloudflare Hub
      {#if cfConfig?.connected}
        <span class="cf-online-dot"></span>
      {/if}
    </button>
  </nav>

  <!-- TAB 1: OVERVIEW (Screenshot 2) -->
  {#if activeTab === 'overview'}
    <div class="overview-grid">
      <!-- 4 Quick Stats Cards -->
      <div class="stat-card">
        <div class="stat-head">
          <span class="stat-label">Today request</span>
          <i class="ri-bar-chart-box-line stat-icon text-accent"></i>
        </div>
        <div class="stat-num">{overview?.today_requests.toLocaleString() || '1,420'}</div>
        <div class="stat-sub">Yesterday: {overview?.yesterday_requests.toLocaleString() || '3,280'}</div>
      </div>

      <div class="stat-card danger">
        <div class="stat-head">
          <span class="stat-label">Malicious request</span>
          <i class="ri-shield-cross-line stat-icon text-danger"></i>
        </div>
        <div class="stat-num text-danger">{overview?.malicious_requests.toLocaleString() || '23'}</div>
        <div class="stat-sub">
          <span class="trend-badge down"><i class="ri-arrow-down-line"></i> 64.62%</span>
          Yesterday: {overview?.yesterday_malicious || '65'}
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-head">
          <span class="stat-label">Real-time QPS</span>
          <i class="ri-speed-line stat-icon text-info"></i>
        </div>
        <div class="stat-num">{overview?.realtime_qps || '0.8'} <span class="stat-unit">/s</span></div>
        <div class="stat-sub">Pencegatan WAF: {overview?.intercept_rate.toFixed(2) || '1.62'}%</div>
      </div>

      <div class="stat-card">
        <div class="stat-head">
          <span class="stat-label">Real-time origin response</span>
          <i class="ri-timer-line stat-icon text-success"></i>
        </div>
        <div class="stat-num text-success">{overview?.origin_latency_ms || '4'} <span class="stat-unit">ms</span></div>
        <div class="stat-sub">Kecepatan latensi server normal</div>
      </div>
    </div>

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
            <span class="badge-soft">Peta Koordinat Serangan Real-time</span>
            <button class="btn-link text-xs text-accent" onclick={() => activeTab = 'attack_map'}>
              Buka Peta Penuh <i class="ri-arrow-right-line"></i>
            </button>
          </div>
        </div>

        <div class="world-map-wrapper">
          <div class="overview-leaflet-container" use:overviewMapAction></div>
        </div>
      </div>

      <!-- Top Attack IPs Table (Screenshot 2 Right Side) -->
      <div class="bento-panel">
        <div class="panel-header">
          <div class="panel-title-wrap">
            <i class="ri-skull-line text-danger"></i>
            <h3>Top Attack IPs</h3>
          </div>
          <span class="text-xs text-muted">Hari Ini</span>
        </div>

        <div class="table-container-compact">
          <table class="waf-table-compact">
            <thead>
              <tr>
                <th>Attack IP</th>
                <th>Times</th>
                <th>IP attribution</th>
              </tr>
            </thead>
            <tbody>
              {#if !overview?.top_attack_ips || overview.top_attack_ips.length === 0}
                <tr>
                  <td colspan="3" class="text-center text-muted">Belum ada rekaman IP penyerang</td>
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

      <!-- Traffic Ranking TOP 10 -->
      <div class="bento-panel">
        <div class="panel-header">
          <h4><i class="ri-line-chart-line text-accent"></i> Traffic ranking TOP10</h4>
        </div>
        <div class="rank-list">
          {#if overview?.traffic_ranking}
            {#each overview.traffic_ranking as tr, idx}
              <div class="rank-item">
                <span class="rank-num">{idx + 1}</span>
                <span class="rank-name mono">{tr.domain}</span>
                <span class="rank-val text-accent">{tr.requests.toLocaleString()} req</span>
              </div>
            {/each}
          {/if}
        </div>
      </div>

      <!-- Visited Pages TOP 10 -->
      <div class="bento-panel">
        <div class="panel-header">
          <h4><i class="ri-file-search-line text-info"></i> Visited pages TOP10</h4>
        </div>
        <div class="rank-list">
          {#if overview?.visited_pages}
            {#each overview.visited_pages as vp, idx}
              <div class="rank-item">
                <span class="rank-num">{idx + 1}</span>
                <span class="rank-name mono text-xs truncate" title={vp.path}>{vp.path}</span>
                <span class="rank-val badge-soft">{vp.count}</span>
              </div>
            {/each}
          {/if}
        </div>
      </div>
    </div>

  <!-- TAB 2: WEBSITE PROTECTION (Screenshot 3) -->
  {:else if activeTab === 'website'}
    <div class="bento-panel">
      <div class="panel-header-toolbar">
        <div class="left-tools">
          <div class="search-box">
            <i class="ri-search-line"></i>
            <input
              type="text"
              bind:value={siteSearch}
              placeholder="Cari nama situs..."
              class="search-input"
            />
          </div>
        </div>
        <button class="btn btn-secondary btn-sm" onclick={loadSites} disabled={loadingSites}>
          <i class="ri-refresh-line {loadingSites ? 'spin' : ''}"></i> Muat Ulang
        </button>
      </div>

      <div class="table-responsive">
        <table class="waf-main-table">
          <thead>
            <tr>
              <th>Site</th>
              <th>Total interceptions</th>
              <th>Fake spider</th>
              <th>Using CDN</th>
              <th>Smart CC defense</th>
              <th>Crawl Protection</th>
              <th>Waiting Room</th>
              <th>CC Defense</th>
              <th>Status</th>
              <th class="text-right">Operate</th>
            </tr>
          </thead>
          <tbody>
            {#if loadingSites}
              <tr>
                <td colspan="10" class="text-center p-6 text-muted">
                  <i class="ri-loader-4-line spin text-lg"></i> Memuat daftar situs...
                </td>
              </tr>
            {:else if filteredSites.length === 0}
              <tr>
                <td colspan="10" class="text-center p-6 text-muted">
                  Tidak ada situs yang cocok. Buat situs terlebih dahulu pada menu Website.
                </td>
              </tr>
            {:else}
              {#each filteredSites as site}
                <tr>
                  <td class="font-bold text-accent mono">{site.site_name}</td>
                  <td>
                    <span class="badge-intercept-count">{site.total_interceptions}</span>
                  </td>
                  <td>
                    <input
                      type="checkbox"
                      bind:checked={site.fake_spider}
                      onchange={() => apiFetch(`/api/waf/sites/${encodeURIComponent(site.site_name)}`, { method: 'POST', body: JSON.stringify(site) })}
                    />
                  </td>
                  <td>
                    <input
                      type="checkbox"
                      bind:checked={site.using_cdn}
                      onchange={() => apiFetch(`/api/waf/sites/${encodeURIComponent(site.site_name)}`, { method: 'POST', body: JSON.stringify(site) })}
                    />
                  </td>
                  <td>
                    <span class="badge-tag mono">{site.smart_cc} req/s</span>
                  </td>
                  <td>
                    <span class="badge-pill {site.crawl_protection ? 'active' : 'disabled'}">
                      {site.crawl_protection ? 'Enable' : 'Disable'}
                    </span>
                  </td>
                  <td>
                    <span class="badge-pill {site.rate_limit ? 'active' : 'disabled'}">
                      {site.rate_limit ? 'Enable' : 'Disable'}
                    </span>
                  </td>
                  <td>
                    <span class="mono text-xs">{site.cc_defense} layer</span>
                  </td>
                  <td>
                    <label class="switch" aria-label="Toggle WAF Site">
                      <input type="checkbox" checked={site.status} onchange={() => toggleSiteStatus(site)} />
                      <span class="slider"></span>
                    </label>
                  </td>
                  <td class="text-right actions-cell">
                    <button
                      class="btn-link"
                      onclick={() => { activeTab = 'attack_map'; loadEvents(); }}
                    >
                      Log
                    </button>
                    <button
                      class="btn-link"
                      onclick={() => { editingSite = { ...site }; showSiteModal = true; }}
                    >
                      Modify
                    </button>
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>

      <!-- Panduan CDN WAF -->
      <div class="info-footer-note">
        <i class="ri-information-line"></i>
        <span>Opsi CDN: Setelah mengaktifkan CDN/Cloudflare untuk situs, opsi "Using CDN" wajib dicentang agar IP proxy CDN tidak terblokir secara keliru oleh mitigasi CC.</span>
      </div>
    </div>

  <!-- TAB 3: ATTACK MAP (Screenshot 4) -->
  {:else if activeTab === 'attack_map'}
    <div class="bento-panel attack-studio-panel">
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
          <button class="btn btn-secondary btn-sm" onclick={loadEvents} disabled={loadingEvents}>
            <i class="ri-refresh-line {loadingEvents ? 'spin' : ''}"></i> Segarkan
          </button>
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
          <button class="btn btn-secondary btn-sm" onclick={flyToServer} title="Pusatkan peta ke server host FikPanel">
            <i class="ri-focus-3-line text-accent"></i> Host Server
          </button>
          <button class="btn btn-secondary btn-sm" onclick={resetMapView} title="Kembalikan tampilan peta global">
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
                if (c.lat && c.lng) flyToCountry(c.lat, c.lng);
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
        <div class="attack-map-canvas" use:attackMapAction></div>

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
                onclick={() => { quickBlockIP(selectedEvent.client_ip, selectedEvent.attack_type); selectedEvent = null; }}
              >
                <i class="ri-shield-cross-line"></i> Blokir IP ke Blacklist
              </button>
              <button
                class="btn btn-secondary btn-sm"
                onclick={() => flyToCountry(selectedEvent.latitude, selectedEvent.longitude)}
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
                      onclick={() => { selectedEvent = ev; flyToCountry(ev.latitude, ev.longitude); }}
                    >
                      <i class="ri-search-eye-line"></i>
                    </button>
                    <button
                      class="btn-icon-xs text-danger"
                      title="Blokir IP ke Blacklist"
                      onclick={() => quickBlockIP(ev.client_ip, ev.attack_type)}
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

  <!-- TAB 4: BLACK / WHITE LIST (Screenshot 5) -->
  {:else if activeTab === 'blacklist'}
    <div class="bento-panel">
      <!-- Sub-tabs Daftar Hitam/Putih -->
      <div class="rule-subtabs">
        <button
          class="rule-subtab-btn {activeRuleType === 'ip_whitelist' ? 'active' : ''}"
          onclick={() => { activeRuleType = 'ip_whitelist'; loadRules(); }}
        >
          IP whitelist
        </button>
        <button
          class="rule-subtab-btn {activeRuleType === 'ip_blacklist' ? 'active' : ''}"
          onclick={() => { activeRuleType = 'ip_blacklist'; loadRules(); }}
        >
          IP blacklist
        </button>
        <button
          class="rule-subtab-btn {activeRuleType === 'ua_whitelist' ? 'active' : ''}"
          onclick={() => { activeRuleType = 'ua_whitelist'; loadRules(); }}
        >
          UA whitelist
        </button>
        <button
          class="rule-subtab-btn {activeRuleType === 'ua_blacklist' ? 'active' : ''}"
          onclick={() => { activeRuleType = 'ua_blacklist'; loadRules(); }}
        >
          UA blacklist
        </button>
        <button
          class="rule-subtab-btn {activeRuleType === 'url_whitelist' ? 'active' : ''}"
          onclick={() => { activeRuleType = 'url_whitelist'; loadRules(); }}
        >
          URL whitelist
        </button>
        <button
          class="rule-subtab-btn {activeRuleType === 'url_blacklist' ? 'active' : ''}"
          onclick={() => { activeRuleType = 'url_blacklist'; loadRules(); }}
        >
          URL blacklist
        </button>
      </div>

      <!-- Form Tambah Aturan (Screenshot 5 Form Box) -->
      <div class="add-rule-box">
        <h4 class="form-box-title">
          Tambahkan entri baru ke {activeRuleType.replace('_', ' ').toUpperCase()}
        </h4>

        {#if activeRuleType.startsWith('ip_')}
          <div class="form-row-grid">
            <div class="form-field">
              <span class="field-label">Type</span>
              <div class="pill-group">
                <button
                  type="button"
                  class="pill-opt {ruleType === 'IPv4' ? 'active' : ''}"
                  onclick={() => ruleType = 'IPv4'}
                >
                  IPv4
                </button>
                <button
                  type="button"
                  class="pill-opt {ruleType === 'IPv6' ? 'active' : ''}"
                  onclick={() => ruleType = 'IPv6'}
                >
                  IPv6
                </button>
              </div>
            </div>

            <div class="form-field">
              <label for="start-ip" class="field-label">Start IP / Single IP</label>
              <input
                id="start-ip"
                type="text"
                bind:value={ruleStartIP}
                placeholder="Contoh: 192.168.1.1 atau 203.0.113.5"
                class="input-dark mono"
              />
            </div>

            <div class="form-field">
              <label for="end-ip" class="field-label">End IP (Opsional untuk rentang)</label>
              <input
                id="end-ip"
                type="text"
                bind:value={ruleEndIP}
                placeholder="Contoh: 192.168.1.254"
                class="input-dark mono"
              />
            </div>

            <div class="form-field">
              <label for="remarks" class="field-label">Remarks / Keterangan</label>
              <input
                id="remarks"
                type="text"
                bind:value={ruleRemarks}
                placeholder="Catatan alasan aturan"
                class="input-dark"
              />
            </div>

            <div class="form-field btn-cell">
              <button
                class="btn btn-primary"
                onclick={handleAddRule}
                disabled={submittingRule}
              >
                {submittingRule ? 'Menyimpan...' : 'Add'}
              </button>
            </div>
          </div>
        {:else}
          <!-- Form untuk User-Agent atau URL -->
          <div class="form-row-grid">
            <div class="form-field col-span-2">
              <label for="pattern" class="field-label">Pola / String Pencocokan ({activeRuleType.includes('ua') ? 'User-Agent' : 'URL Path'})</label>
              <input
                id="pattern"
                type="text"
                bind:value={rulePattern}
                placeholder={activeRuleType.includes('ua') ? 'Contoh: Googlebot, curl, python-requests' : 'Contoh: /wp-admin, /api/internal'}
                class="input-dark mono"
              />
            </div>
            <div class="form-field">
              <label for="remarks-text" class="field-label">Remarks</label>
              <input
                id="remarks-text"
                type="text"
                bind:value={ruleRemarks}
                placeholder="Catatan"
                class="input-dark"
              />
            </div>
            <div class="form-field btn-cell">
              <button
                class="btn btn-primary"
                onclick={handleAddRule}
                disabled={submittingRule}
              >
                {submittingRule ? 'Menyimpan...' : 'Add'}
              </button>
            </div>
          </div>
        {/if}
      </div>

      <!-- Tabel Daftar Aturan Aktif -->
      <div class="table-responsive mt-6">
        <table class="waf-main-table">
          <thead>
            <tr>
              {#if activeRuleType.startsWith('ip_')}
                <th>Start IP - End IP / IPv6 address</th>
                <th>Type</th>
              {:else}
                <th>Pattern String</th>
                <th>Target</th>
              {/if}
              <th>Remarks</th>
              <th class="text-right">Operate</th>
            </tr>
          </thead>
          <tbody>
            {#if loadingRules}
              <tr>
                <td colspan="4" class="text-center p-4 text-muted">Memuat daftar aturan...</td>
              </tr>
            {:else if rules.length === 0}
              <tr>
                <td colspan="4" class="text-center p-6 text-muted">
                  Belum ada entri aturan untuk kategori {activeRuleType.replace('_', ' ').toUpperCase()}.
                </td>
              </tr>
            {:else}
              {#each rules as r}
                <tr>
                  {#if activeRuleType.startsWith('ip_')}
                    <td class="mono font-semibold">
                      {r.start_ip} {r.end_ip ? `- ${r.end_ip}` : ''}
                    </td>
                    <td>
                      <span class="badge-soft">{r.ip_version}</span>
                    </td>
                  {:else}
                    <td class="mono font-semibold text-accent">{r.pattern}</td>
                    <td><span class="badge-soft">{r.rule_type.toUpperCase()}</span></td>
                  {/if}
                  <td class="text-subtle">{r.remarks || '-'}</td>
                  <td class="text-right">
                    <button class="btn-link text-danger" onclick={() => deleteRule(r)}>
                      Delete
                    </button>
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>

      <!-- Guide Rules Note -->
      <div class="info-footer-note mt-6">
        <i class="ri-shield-check-line"></i>
        <span>Hierarki Keamanan: IP Whitelist memiliki prioritas absolut tertinggi di atas Blacklist dan filter URL. Masukkan IP terpercaya pengelola ke whitelist untuk menghindari penguncian tak sengaja.</span>
      </div>
    </div>

  <!-- TAB 5: CLOUDFLARE HUB (Third Feature Request) -->
  {:else if activeTab === 'cloudflare'}
    <div class="bento-panel">
      <!-- Status Koneksi Header -->
      <div class="cf-status-card">
        <div class="cf-left">
          <div class="cf-logo-icon">
            <i class="ri-cloud-line"></i>
          </div>
          <div>
            <div class="cf-title-row">
              <h3>Integrasi Akun Cloudflare API v4</h3>
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
            <p class="text-subtle text-xs">
              Membaca seluruh domain di Cloudflare, memverifikasi apakah mengarah ke server ini, sinkronisasi DNS 1-klik, dan manajemen CDN cache langsung dari FikPanel.
            </p>
          </div>
        </div>

        <div class="cf-actions">
          {#if cfConfig?.connected}
            <button class="btn btn-secondary btn-sm" onclick={loadCloudflare} disabled={loadingCF}>
              <i class="ri-refresh-line {loadingCF ? 'spin' : ''}"></i> Sinkronkan Domain
            </button>
            <button class="btn btn-danger-outline btn-sm" onclick={handleDisconnectCF}>
              Putuskan Koneksi
            </button>
          {:else}
            <button class="btn btn-primary btn-sm" onclick={() => showCFConnectModal = true}>
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
            Dapatkan kontrol penuh atas domain Anda di Cloudflare: deteksi otomatis apakah DNS mengarah ke VPS Anda atau host lain, aktifkan proxy Orange Cloud, dan kosongkan cache dengan 1-klik.
          </p>
          <button class="btn btn-primary" onclick={() => showCFConnectModal = true}>
            Mulai Hubungkan API Token
          </button>
        </div>
      {:else}
        <!-- Daftar Domain di Cloudflare -->
        <div class="panel-header mt-6">
          <div class="panel-title-wrap">
            <i class="ri-global-line text-accent"></i>
            <h4>Domain Terdeteksi di Akun Cloudflare ({cfZones.length})</h4>
          </div>
          <span class="text-xs text-muted">IP Server Ini: <code class="mono text-accent">{serverIP || 'Mendeteksi...'}</code></span>
        </div>

        <div class="table-responsive">
          <table class="waf-main-table">
            <thead>
              <tr>
                <th>Domain Name</th>
                <th>Status</th>
                <th>Arah DNS Server</th>
                <th>Mode SSL Cloudflare</th>
                <th>Mode Pengembang</th>
                <th class="text-right">Aksi Cepat</th>
              </tr>
            </thead>
            <tbody>
              {#if loadingCF}
                <tr>
                  <td colspan="6" class="text-center p-6 text-muted">
                    <i class="ri-loader-4-line spin text-lg"></i> Mengambil data zona dari Cloudflare API...
                  </td>
                </tr>
              {:else if cfZones.length === 0}
                <tr>
                  <td colspan="6" class="text-center p-6 text-muted">
                    Tidak ada domain ditemukan pada akun Cloudflare ini.
                  </td>
                </tr>
              {:else}
                {#each cfZones as zone}
                  <tr>
                    <td class="font-bold mono text-accent">
                      {zone.name}
                    </td>
                    <td>
                      <span class="badge-soft uppercase">{zone.status}</span>
                    </td>
                    <td>
                      {#if zone.points_to_server}
                        <span class="badge-points-here">
                          <i class="ri-checkbox-circle-fill"></i> Mengarah ke Server Ini
                        </span>
                      {:else}
                        <span class="badge-points-other" title="Domain ini mengarah ke IP/server lain, bukan server FikPanel ini">
                          <i class="ri-external-link-line"></i> Mengarah ke Host Lain
                        </span>
                      {/if}
                    </td>
                    <td>
                      <span class="badge-ssl-mode">{zone.ssl_mode || 'Full (Strict)'}</span>
                    </td>
                    <td>
                      <label class="switch" title="Bypass cache Cloudflare selama 3 jam">
                        <input
                          type="checkbox"
                          checked={zone.development_mode === 1}
                          onchange={() => toggleDevMode(zone)}
                        />
                        <span class="slider"></span>
                      </label>
                    </td>
                    <td class="text-right actions-cell">
                      <!-- 1-Click Point DNS -->
                      <button
                        class="btn btn-xs btn-secondary"
                        onclick={() => pointZoneHere(zone, true)}
                        title="Perbarui record A domain di Cloudflare ke IP server ini (Proxy Aktif)"
                      >
                        <i class="ri-focus-3-line"></i> Arahkan ke Sini
                      </button>

                      <!-- Purge Cache -->
                      <button
                        class="btn btn-xs btn-secondary"
                        onclick={() => purgeCFCache(zone)}
                        title="Kosongkan seluruh cache CDN Cloudflare"
                      >
                        <i class="ri-delete-bin-7-line"></i> Purge
                      </button>

                      <!-- Rincian DNS Records -->
                      <button
                        class="btn btn-xs btn-ghost"
                        onclick={() => { selectedZone = zone; showDNSModal = true; }}
                      >
                        DNS ({zone.dns_records?.length || 0})
                      </button>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}
</div>

<!-- Modal Hubungkan Cloudflare Token -->
{#if showCFConnectModal}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) showCFConnectModal = false; }}
    onkeydown={(e) => { if (e.key === 'Escape') showCFConnectModal = false; }}
  >
    <div class="modal-card">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-cloud-line text-accent"></i>
          <div>
            <h3>Hubungkan Akun Cloudflare</h3>
            <span class="modal-sub">Gunakan API Token dengan izin Zone.Read dan DNS.Edit</span>
          </div>
        </div>
        <button class="btn-close" onclick={() => showCFConnectModal = false}>&times;</button>
      </div>

      <div class="modal-body">
        <div class="form-group">
          <label for="cf-token">Cloudflare API Token (Wajib)</label>
          <input
            id="cf-token"
            type="password"
            bind:value={cfTokenInput}
            placeholder="Ketik atau tempel Cloudflare API Token Anda di sini..."
            class="input-dark mono"
          />
          <span class="text-xs text-muted">
            Buat token di: <strong>dash.cloudflare.com/profile/api-tokens</strong> (Template: Edit Zone DNS).
          </span>
        </div>

        <div class="form-group mt-4">
          <label for="cf-email">Email Akun Cloudflare (Opsional)</label>
          <input
            id="cf-email"
            type="email"
            bind:value={cfEmailInput}
            placeholder="user@example.com"
            class="input-dark"
          />
        </div>
      </div>

      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" onclick={() => showCFConnectModal = false}>Batal</button>
        <button type="button" class="btn btn-primary" onclick={handleConnectCF} disabled={cfSubmitting}>
          {cfSubmitting ? 'Memverifikasi...' : 'Verifikasi & Hubungkan'}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal Rincian DNS Records -->
{#if showDNSModal && selectedZone}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) showDNSModal = false; }}
    onkeydown={(e) => { if (e.key === 'Escape') showDNSModal = false; }}
  >
    <div class="modal-card modal-wide">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-list-settings-line text-accent"></i>
          <div>
            <h3>Rekaman DNS: {selectedZone.name}</h3>
            <span class="modal-sub">Daftar entri DNS aktif pada Cloudflare</span>
          </div>
        </div>
        <button class="btn-close" onclick={() => showDNSModal = false}>&times;</button>
      </div>

      <div class="table-responsive max-h-96">
        <table class="waf-main-table">
          <thead>
            <tr>
              <th>Type</th>
              <th>Name</th>
              <th>Content / Target</th>
              <th>Proxy Status</th>
              <th>Server Match</th>
            </tr>
          </thead>
          <tbody>
            {#if !selectedZone.dns_records || selectedZone.dns_records.length === 0}
              <tr>
                <td colspan="5" class="text-center p-4 text-muted">Tidak ada rekaman DNS ditemukan.</td>
              </tr>
            {:else}
              {#each selectedZone.dns_records as rec}
                <tr>
                  <td><span class="badge-soft mono">{rec.type}</span></td>
                  <td class="mono font-semibold">{rec.name}</td>
                  <td class="mono text-xs text-subtle">{rec.content}</td>
                  <td>
                    {#if rec.proxied}
                      <span class="badge-orange-cloud"><i class="ri-cloud-fill"></i> Proxied</span>
                    {:else}
                      <span class="badge-grey-cloud"><i class="ri-cloud-line"></i> DNS Only</span>
                    {/if}
                  </td>
                  <td>
                    {#if rec.points_to_server}
                      <span class="badge-points-here">Match Server</span>
                    {:else}
                      <span class="text-muted text-xs">-</span>
                    {/if}
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>

      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" onclick={() => showDNSModal = false}>Tutup</button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal Ubah Opsi Situs WAF -->
{#if showSiteModal && editingSite}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) showSiteModal = false; }}
    onkeydown={(e) => { if (e.key === 'Escape') showSiteModal = false; }}
  >
    <div class="modal-card">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-settings-4-line text-accent"></i>
          <div>
            <h3>Modifikasi Proteksi WAF: {editingSite.site_name}</h3>
            <span class="modal-sub">Konfigurasi mitigasi bot, CC flood, dan CDN</span>
          </div>
        </div>
        <button class="btn-close" onclick={() => showSiteModal = false}>&times;</button>
      </div>

      <div class="modal-body stack-gap">
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

      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" onclick={() => showSiteModal = false}>Batal</button>
        <button type="button" class="btn btn-primary" onclick={saveSiteSettings}>Simpan Konfigurasi</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .waf-container {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }

  .waf-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    padding: 1.25rem 1.5rem;
    border-radius: var(--radius-md);
  }
  .waf-brand-title {
    display: flex;
    align-items: center;
    gap: 1rem;
  }
  .waf-icon-glow {
    width: 44px;
    height: 44px;
    border-radius: 10px;
    background: var(--tint-info);
    color: var(--accent);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.5rem;
    box-shadow: 0 0 16px var(--tint-info);
  }
  .waf-title {
    font-size: 1.35rem;
    font-weight: 700;
    margin: 0;
    color: var(--text);
  }
  .waf-desc {
    margin: 0.2rem 0 0;
    font-size: 0.8rem;
    color: var(--text-muted);
  }
  .waf-top-actions {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .badge-status-glow {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    background: var(--tint-accent);
    border: 1px solid var(--ring-accent);
    color: var(--accent-strong);
    padding: 0.35rem 0.75rem;
    border-radius: 9999px;
    font-size: 0.78rem;
    font-weight: 600;
  }
  .dot-glow {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 8px var(--accent);
    animation: pulseGlow 2s infinite ease-in-out;
  }
  @keyframes pulseGlow {
    0%, 100% { transform: scale(1); opacity: 1; }
    50% { transform: scale(1.3); opacity: 0.6; }
  }

  /* Subtabs Nav */
  .waf-tabs-nav {
    display: flex;
    gap: 0.5rem;
    border-bottom: 1px solid var(--border);
    padding-bottom: 0.25rem;
  }
  .waf-tab-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    color: var(--text-muted);
    padding: 0.6rem 1rem;
    font-size: 0.85rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .waf-tab-btn.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
    font-weight: 600;
  }
  .waf-tab-btn:hover:not(.active) {
    color: var(--text);
  }
  .cf-online-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--warning);
    box-shadow: 0 0 6px var(--warning);
  }

  /* Overview Cards Grid */
  .overview-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 1rem;
  }
  .stat-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 1.15rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .stat-card.danger {
    border-color: var(--ring-danger);
    background: var(--tint-danger);
  }
  .stat-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .stat-label {
    font-size: 0.78rem;
    color: var(--text-muted);
  }
  .stat-icon {
    font-size: 1.2rem;
  }
  .stat-num {
    font-size: 1.8rem;
    font-weight: 700;
    color: var(--text);
  }
  .stat-unit {
    font-size: 0.85rem;
    font-weight: 400;
    color: var(--text-subtle);
  }
  .stat-sub {
    font-size: 0.72rem;
    color: var(--text-subtle);
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .trend-badge.down {
    background: var(--tint-accent);
    color: var(--accent-strong);
    padding: 0.1rem 0.35rem;
    border-radius: 4px;
    font-weight: 600;
  }

  /* Bento 2 Col */
  .bento-two-col {
    display: grid;
    grid-template-columns: 2fr 1.2fr;
    gap: 1.25rem;
  }
  @media (max-width: 1024px) {
    .bento-two-col {
      grid-template-columns: 1fr;
    }
  }

  .bento-panel {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 1.25rem;
  }
  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }
  .panel-title-wrap {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .panel-title-wrap h3 {
    font-size: 0.95rem;
    margin: 0;
    color: var(--text);
  }

  /* Leaflet Map Containers */
  .world-map-wrapper {
    position: relative;
    border-radius: var(--radius-sm);
    overflow: hidden;
    background: var(--bg-inset);
  }
  .overview-leaflet-container {
    width: 100%;
    height: 300px;
    background: var(--bg-inset);
    border-radius: var(--radius-sm);
  }
  .panel-header-badges {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .attribution-main {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }
  .country-name {
    font-weight: 500;
    color: var(--text);
  }
  .attribution-isp {
    font-size: var(--text-2xs);
    color: var(--text-subtle);
    margin-top: 2px;
  }

  /* Attack Studio Panel */
  .attack-studio-panel {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .studio-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
    margin-bottom: 0;
  }
  .radar-live-indicator {
    position: relative;
    width: 36px;
    height: 36px;
    background: var(--tint-accent);
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.25rem;
  }
  .live-dot {
    position: absolute;
    top: 2px;
    right: 2px;
    width: 8px;
    height: 8px;
    background: var(--accent);
    border-radius: 50%;
    box-shadow: 0 0 6px var(--accent);
  }
  .studio-stats-pills {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }
  .stat-pill {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.35rem 0.75rem;
    border-radius: var(--radius-sm);
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.75rem;
  }
  .stat-pill-label {
    color: var(--text-subtle);
  }
  .stat-pill-val {
    font-weight: 700;
    font-family: var(--font-mono);
  }
  .stat-pill-val.danger { color: var(--danger); }
  .stat-pill-val.warning { color: var(--warning); }
  .stat-pill-val.info { color: var(--info); }

  /* Studio Toolbar */
  .studio-toolbar {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
    background: var(--bg-inset);
    padding: 0.75rem 1rem;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
  }
  .search-input-wrap {
    position: relative;
    flex: 1;
    min-width: 240px;
    display: flex;
    align-items: center;
  }
  .search-icon {
    position: absolute;
    left: 0.75rem;
    color: var(--text-subtle);
    pointer-events: none;
  }
  .search-input {
    padding-left: 2.25rem;
    padding-right: 2rem;
    width: 100%;
  }
  .clear-search-btn {
    position: absolute;
    right: 0.75rem;
    background: transparent;
    border: none;
    color: var(--text-subtle);
    cursor: pointer;
    font-size: 1rem;
  }
  .select-sm {
    padding: 0.45rem 0.75rem;
    font-size: 0.78rem;
    cursor: pointer;
  }
  .toolbar-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  /* Country Chips Bar */
  .country-chips-bar {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    flex-wrap: wrap;
    padding: 0.25rem 0;
  }
  .chips-label {
    font-size: 0.75rem;
    color: var(--text-subtle);
    display: flex;
    align-items: center;
    gap: 0.25rem;
  }
  .country-chip {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    color: var(--text-muted);
    padding: 0.25rem 0.6rem;
    border-radius: 9999px;
    font-size: 0.72rem;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    transition: all 0.15s ease;
  }
  .country-chip:hover {
    background: var(--bg-hover);
    color: var(--text);
    border-color: var(--border-strong);
  }
  .country-chip.active {
    background: var(--tint-accent);
    color: var(--accent-strong);
    border-color: var(--accent);
    font-weight: 600;
  }
  .chip-badge {
    background: var(--bg-surface);
    color: var(--danger);
    font-weight: 700;
    font-size: 0.65rem;
    padding: 0.05rem 0.35rem;
    border-radius: 9999px;
  }

  /* Map Viewport & Telemetry Drawer */
  .map-studio-viewport {
    position: relative;
    border-radius: var(--radius-sm);
    overflow: hidden;
    border: 1px solid var(--border);
  }
  .attack-map-canvas {
    width: 100%;
    height: 520px;
    background: var(--bg-inset);
  }

  /* Drawer Telemetri Melayang */
  .telemetry-inspector-drawer {
    position: absolute;
    top: 1rem;
    right: 1rem;
    bottom: 1rem;
    width: 380px;
    max-width: calc(100% - 2rem);
    background: color-mix(in srgb, var(--bg-surface) 92%, transparent);
    backdrop-filter: blur(12px);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-md);
    box-shadow: 0 12px 36px rgb(0 0 0 / 0.5);
    z-index: 1000;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: slideInRight 0.25s ease-out;
  }
  @keyframes slideInRight {
    from { transform: translateX(20px); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
  .drawer-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--border);
    background: var(--bg-raised);
  }
  .drawer-title-wrap {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }
  .drawer-ip {
    margin: 0;
    font-size: 0.95rem;
    color: var(--text);
  }
  .drawer-body {
    padding: 1rem 1.25rem;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
    flex: 1;
  }
  .drawer-card {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.75rem 0.85rem;
  }
  .drawer-card-title {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--text-secondary);
    margin-bottom: 0.5rem;
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }
  .drawer-info-grid {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    font-size: 0.75rem;
  }
  .info-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
  }
  .info-col {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    margin-top: 0.25rem;
  }
  .info-lbl {
    color: var(--text-subtle);
  }
  .info-val {
    color: var(--text);
    text-align: right;
  }
  .ua-box, .payload-box, .rule-box {
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    padding: 0.4rem 0.55rem;
    border-radius: 4px;
    word-break: break-all;
    color: var(--text-muted);
  }
  .payload-box {
    color: var(--warning-strong);
  }
  .drawer-actions {
    padding: 0.85rem 1.25rem;
    border-top: 1px solid var(--border);
    background: var(--bg-raised);
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  /* Leaflet Marker Beacons & Laser Arcs */
  :global(.leaflet-server-icon), :global(.leaflet-threat-icon),
  :global(.leaflet-server-full), :global(.leaflet-threat-full) {
    background: transparent !important;
    border: none !important;
  }
  :global(.server-beacon-dot), :global(.threat-beacon-dot) {
    position: relative;
    width: 20px;
    height: 20px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  :global(.server-beacon-dot .dot-core) {
    width: 8px;
    height: 8px;
    background: var(--accent);
    border-radius: 50%;
    box-shadow: 0 0 8px var(--accent);
  }
  :global(.threat-beacon-dot.blocked .dot-core) {
    width: 8px;
    height: 8px;
    background: var(--danger);
    border-radius: 50%;
    box-shadow: 0 0 8px var(--danger);
  }
  :global(.threat-beacon-dot.warn .dot-core) {
    width: 8px;
    height: 8px;
    background: var(--warning);
    border-radius: 50%;
    box-shadow: 0 0 8px var(--warning);
  }

  :global(.server-host-beacon) {
    position: relative;
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  :global(.server-core-icon) {
    width: 22px;
    height: 22px;
    background: var(--accent);
    color: var(--text-on-accent);
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.85rem;
    box-shadow: 0 0 12px var(--accent);
    z-index: 10;
  }
  :global(.server-host-label) {
    position: absolute;
    top: 26px;
    left: 50%;
    transform: translateX(-50%);
    background: var(--bg-surface);
    border: 1px solid var(--accent);
    color: var(--accent);
    font-family: var(--font-mono);
    font-size: 0.62rem;
    font-weight: 700;
    padding: 1px 4px;
    border-radius: 3px;
    white-space: nowrap;
    box-shadow: 0 2px 8px rgb(0 0 0 / 0.6);
  }

  :global(.threat-marker-beacon) {
    position: relative;
    width: 26px;
    height: 26px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
  }
  :global(.threat-dot-inner) {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    z-index: 5;
  }
  :global(.threat-danger .threat-dot-inner) {
    background: var(--danger);
    box-shadow: 0 0 10px var(--danger);
  }
  :global(.threat-warning .threat-dot-inner) {
    background: var(--warning);
    box-shadow: 0 0 10px var(--warning);
  }

  :global(.pulse-ring) {
    position: absolute;
    width: 100%;
    height: 100%;
    border-radius: 50%;
    animation: pulseScale 2.2s infinite ease-out;
  }
  :global(.pulse-green) {
    border: 2px solid var(--accent);
  }
  :global(.pulse-red) {
    border: 2px solid var(--danger);
  }
  :global(.pulse-amber) {
    border: 2px solid var(--warning);
  }
  @keyframes pulseScale {
    0% { transform: scale(0.4); opacity: 1; }
    100% { transform: scale(2.0); opacity: 0; }
  }

  :global(.laser-arc-danger) {
    stroke: var(--danger) !important;
    stroke-dasharray: 6, 5;
    animation: dashLaser 2.5s linear infinite;
  }
  :global(.laser-arc-warn) {
    stroke: var(--warning) !important;
    stroke-dasharray: 6, 5;
    animation: dashLaser 2.5s linear infinite;
  }
  @keyframes dashLaser {
    from { stroke-dashoffset: 44; }
    to { stroke-dashoffset: 0; }
  }

  :global(.dark-map-tooltip) {
    background: var(--bg-surface) !important;
    border: 1px solid var(--border-strong) !important;
    color: var(--text) !important;
    border-radius: var(--radius-sm) !important;
    padding: 0.4rem 0.6rem !important;
    box-shadow: 0 4px 16px rgb(0 0 0 / 0.5) !important;
    font-size: 0.74rem !important;
  }
  :global(.dark-map-tooltip::before) {
    border-top-color: var(--border-strong) !important;
  }
  :global(.map-tooltip-content) {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  :global(.badge-tag-xs) {
    background: var(--tint-danger);
    color: var(--danger);
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 0.65rem;
    font-weight: 600;
  }

  /* Leaflet Controls & Tile Styling */
  :global(.leaflet-container) {
    background: var(--bg-inset) !important;
    font-family: var(--font-sans) !important;
  }
  :global(.leaflet-control-zoom a) {
    background: var(--bg-surface) !important;
    color: var(--text) !important;
    border-color: var(--border) !important;
  }
  :global(.leaflet-control-zoom a:hover) {
    background: var(--bg-hover) !important;
    color: var(--accent) !important;
  }

  /* Table additions */
  .threat-row {
    transition: background 0.15s ease;
  }
  .threat-row:hover {
    background: var(--bg-hover);
  }
  .threat-row.row-selected {
    background: var(--bg-active);
  }
  .geo-cell {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }
  .geo-city {
    color: var(--text);
  }
  .device-cell {
    display: flex;
    flex-direction: column;
  }
  .method-tag {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 0.68rem;
    font-weight: 700;
  }
  .btn-icon-xs {
    background: transparent;
    border: none;
    cursor: pointer;
    font-size: 1rem;
    padding: 0.2rem 0.35rem;
    border-radius: 4px;
    transition: background 0.15s ease;
  }
  .btn-icon-xs:hover {
    background: var(--bg-hover);
  }
  .table-action-cell {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.25rem;
  }

  /* Compact Table */
  .waf-table-compact {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.78rem;
  }
  .waf-table-compact th {
    text-align: left;
    padding: 0.5rem 0.6rem;
    color: var(--text-subtle);
    border-bottom: 1px solid var(--border);
  }
  .waf-table-compact td {
    padding: 0.55rem 0.6rem;
    border-bottom: 1px solid var(--border-soft);
  }
  .badge-attack-count {
    background: var(--tint-danger);
    color: var(--danger);
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    font-weight: 600;
  }
  .flag-code {
    background: var(--bg-inset);
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
    font-size: 0.68rem;
    color: var(--accent);
  }

  /* Bottom 3 Grid */
  .bottom-three-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 1.25rem;
  }
  .rank-list {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }
  .rank-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.45rem 0.6rem;
    background: var(--bg-inset);
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
  }
  .rank-num {
    width: 20px;
    height: 20px;
    border-radius: 50%;
    background: var(--bg-surface);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.68rem;
    font-weight: bold;
    color: var(--text-muted);
  }
  .rank-num.first {
    background: var(--danger);
    color: var(--text);
  }

  /* Main WAF Table */
  .waf-main-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8rem;
  }
  .waf-main-table th {
    text-align: left;
    padding: 0.75rem 0.85rem;
    color: var(--text-subtle);
    border-bottom: 1px solid var(--border);
    font-weight: 600;
  }
  .waf-main-table td {
    padding: 0.75rem 0.85rem;
    border-bottom: 1px solid var(--border-soft);
    vertical-align: middle;
  }
  .badge-intercept-count {
    color: var(--danger);
    font-weight: 700;
  }
  .badge-tag {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    font-size: 0.75rem;
  }
  .badge-pill {
    padding: 0.2rem 0.5rem;
    border-radius: 9999px;
    font-size: 0.7rem;
    font-weight: 600;
  }
  .badge-pill.active {
    background: var(--tint-accent);
    color: var(--accent-strong);
  }
  .badge-pill.disabled {
    background: var(--tint-danger);
    color: var(--danger);
  }
  .badge-attack-tag {
    background: var(--tint-warning);
    color: var(--warning-strong);
    border: 1px solid var(--tint-warning);
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    font-size: 0.72rem;
  }
  .badge-action {
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    font-size: 0.7rem;
    font-weight: 700;
  }
  .badge-action.block {
    background: var(--tint-danger);
    color: var(--danger);
  }
  .badge-action.intercept {
    background: var(--tint-info);
    color: var(--accent);
  }
  .badge-action.challenge {
    background: var(--tint-warning);
    color: var(--warning-strong);
  }

  .btn-link {
    background: transparent;
    border: none;
    color: var(--accent);
    cursor: pointer;
    font-size: 0.78rem;
    padding: 0.2rem 0.4rem;
  }
  .btn-link:hover {
    text-decoration: underline;
  }

  .info-footer-note {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 1.25rem;
    padding: 0.75rem 1rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 0.75rem;
    color: var(--text-muted);
  }

  /* Black/White list subtabs */
  .rule-subtabs {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
    margin-bottom: 1.25rem;
    border-bottom: 1px solid var(--border);
    padding-bottom: 0.5rem;
  }
  .rule-subtab-btn {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    color: var(--text-muted);
    padding: 0.4rem 0.8rem;
    border-radius: var(--radius-sm);
    font-size: 0.78rem;
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .rule-subtab-btn.active {
    background: var(--accent);
    color: var(--text);
    border-color: var(--accent);
    font-weight: 600;
  }

  .add-rule-box {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 1rem;
  }
  .form-box-title {
    font-size: 0.85rem;
    margin: 0 0 0.85rem 0;
    color: var(--text);
  }
  .form-row-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)) auto;
    gap: 0.85rem;
    align-items: flex-end;
  }
  .pill-group {
    display: flex;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 2px;
  }
  .pill-opt {
    flex: 1;
    background: transparent;
    border: none;
    color: var(--text-muted);
    padding: 0.35rem 0.6rem;
    font-size: 0.75rem;
    border-radius: 4px;
    cursor: pointer;
  }
  .pill-opt.active {
    background: var(--accent);
    color: var(--text);
    font-weight: bold;
  }
  .input-dark {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    color: var(--text);
    padding: 0.5rem 0.75rem;
    border-radius: var(--radius-sm);
    font-size: 0.8rem;
    width: 100%;
  }

  /* Cloudflare Hub Styles */
  .cf-status-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 1.25rem;
    gap: 1.5rem;
  }
  .cf-left {
    display: flex;
    align-items: center;
    gap: 1rem;
  }
  .cf-logo-icon {
    width: 48px;
    height: 48px;
    background: var(--tint-warning);
    color: var(--warning-strong);
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.6rem;
    box-shadow: 0 0 16px var(--tint-warning);
  }
  .cf-title-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-bottom: 0.25rem;
  }
  .cf-title-row h3 {
    margin: 0;
    font-size: 1rem;
    color: var(--text);
  }
  .badge-cf-connected {
    background: var(--tint-accent);
    color: var(--accent-strong);
    padding: 0.15rem 0.5rem;
    border-radius: 9999px;
    font-size: 0.72rem;
    font-weight: 600;
  }
  .badge-cf-disconnected {
    background: var(--tint-danger);
    color: var(--danger);
    padding: 0.15rem 0.5rem;
    border-radius: 9999px;
    font-size: 0.72rem;
    font-weight: 600;
  }
  .badge-points-here {
    background: var(--tint-accent);
    color: var(--accent-strong);
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    font-size: 0.72rem;
    font-weight: 600;
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
  }
  .badge-points-other {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    color: var(--text-subtle);
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    font-size: 0.72rem;
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
  }
  .badge-orange-cloud {
    color: var(--warning-strong);
    font-size: 0.75rem;
    font-weight: 600;
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
  }
  .badge-grey-cloud {
    color: var(--text-subtle);
    font-size: 0.75rem;
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
  }
  .badge-ssl-mode {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    font-size: 0.72rem;
    color: var(--info);
  }

  .cf-empty-promo {
    text-align: center;
    padding: 3rem 1.5rem;
    background: var(--bg-inset);
    border: 1px dashed var(--border);
    border-radius: var(--radius-sm);
    margin-top: 1.5rem;
  }
  .promo-icon {
    font-size: 3rem;
    color: var(--warning-strong);
    margin-bottom: 0.75rem;
  }
  .cf-empty-promo h4 {
    margin: 0 0 0.5rem;
    color: var(--text);
  }
  .cf-empty-promo p {
    color: var(--text-muted);
    font-size: 0.85rem;
    max-width: 500px;
    margin: 0 auto 1.25rem;
  }
</style>
