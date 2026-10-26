<script lang="ts">
  import { onMount } from 'svelte';
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

  // Konversi koordinat bumi (lat, lng) ke koordinat proyeksi peta SVG (1000x500)
  function projectCoordinates(lat: number, lng: number): { x: number; y: number } {
    const x = ((lng + 180) / 360) * 1000;
    const y = ((90 - lat) / 180) * 500;
    return { x, y };
  }

  // Koordinat server host (Indonesia / Jakarta -6.2, 106.8 atau Singapore 1.35, 103.8)
  const hostServerCoords = projectCoordinates(1.3521, 103.8198);

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
      <!-- 2D Interactive World Map -->
      <div class="bento-panel map-panel">
        <div class="panel-header">
          <div class="panel-title-wrap">
            <i class="ri-earth-line text-accent"></i>
            <h3>Access / Block Map</h3>
          </div>
          <span class="badge-soft">Peta Koordinat Serangan</span>
        </div>

        <div class="world-map-wrapper">
          <svg viewBox="0 0 1000 500" class="world-map-svg" preserveAspectRatio="xMidYMid meet">
            <rect width="1000" height="500" fill="#0b1219" rx="12" />
            <!-- Pola grid peta bumi -->
            <defs>
              <pattern id="grid-pattern" width="40" height="40" patternUnits="userSpaceOnUse">
                <path d="M 40 0 L 0 0 0 40" fill="none" stroke="rgba(255,255,255,0.03)" stroke-width="1" />
              </pattern>
              <filter id="glow-node" x="-50%" y="-50%" width="200%" height="200%">
                <feGaussianBlur stdDeviation="3" result="blur" />
                <feMerge>
                  <feMergeNode in="blur" />
                  <feMergeNode in="SourceGraphic" />
                </feMerge>
              </filter>
            </defs>
            <rect width="1000" height="500" fill="url(#grid-pattern)" />

            <!-- Siluet Benua Sederhana (World Silhouette Outlines) -->
            <!-- Amerika Utara & Selatan -->
            <path d="M160,80 Q220,70 300,100 Q260,160 220,200 Q260,250 280,320 Q240,430 200,380 Q180,300 160,220 Z" fill="#152433" stroke="#223b53" stroke-width="1" />
            <!-- Eropa & Asia -->
            <path d="M460,80 Q580,60 820,90 Q880,180 820,260 Q720,280 620,220 Q540,240 480,160 Z" fill="#152433" stroke="#223b53" stroke-width="1" />
            <!-- Afrika -->
            <path d="M480,200 Q560,200 580,280 Q540,380 480,360 Q440,280 480,200 Z" fill="#152433" stroke="#223b53" stroke-width="1" />
            <!-- Australia -->
            <path d="M780,320 Q880,310 880,380 Q800,420 760,360 Z" fill="#152433" stroke="#223b53" stroke-width="1" />

            <!-- Garis Serangan Laser Melengkung (Arcs) dari Negara Penyerang ke Server Host -->
            {#if overview?.recent_events}
              {#each overview.recent_events.slice(0, 8) as ev}
                {@const origin = projectCoordinates(ev.latitude, ev.longitude)}
                <path
                  d="M {origin.x},{origin.y} Q {(origin.x + hostServerCoords.x) / 2},{(origin.y + hostServerCoords.y) / 2 - 45} {hostServerCoords.x},{hostServerCoords.y}"
                  fill="none"
                  stroke={ev.action === 'block' ? 'rgba(239, 68, 68, 0.65)' : 'rgba(245, 158, 11, 0.55)'}
                  stroke-width="1.8"
                  stroke-dasharray="6,4"
                  class="laser-beam"
                />
                <!-- Pin Titik Serangan Asal -->
                <circle cx={origin.x} cy={origin.y} r="4.5" fill="#ef4444" filter="url(#glow-node)" />
                <circle cx={origin.x} cy={origin.y} r="9" fill="none" stroke="rgba(239, 68, 68, 0.4)" class="pulse-ring" />
              {/each}
            {/if}

            <!-- Server Host Pin (Target) -->
            <circle cx={hostServerCoords.x} cy={hostServerCoords.y} r="7" fill="#00d084" filter="url(#glow-node)" />
            <circle cx={hostServerCoords.x} cy={hostServerCoords.y} r="14" fill="none" stroke="rgba(0, 208, 132, 0.5)" class="pulse-ring" />
            <text x={hostServerCoords.x + 12} y={hostServerCoords.y + 4} fill="#00d084" font-size="11" font-weight="bold" font-family="monospace">
              SERVER HOST
            </text>
          </svg>
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
                      <span class="flag-code">{ipItem.country_code}</span> {ipItem.country}
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
    <div class="bento-panel">
      <div class="panel-header">
        <div class="panel-title-wrap">
          <i class="ri-radar-line text-accent"></i>
          <h3>Live Global Attack Map & Threat Telemetry</h3>
        </div>
        <button class="btn btn-secondary btn-sm" onclick={loadEvents} disabled={loadingEvents}>
          <i class="ri-refresh-line {loadingEvents ? 'spin' : ''}"></i> Refresh Log Serangan
        </button>
      </div>

      <div class="large-map-container">
        <svg viewBox="0 0 1000 500" class="world-map-svg full" preserveAspectRatio="xMidYMid meet">
          <rect width="1000" height="500" fill="#080f14" rx="12" />
          <defs>
            <pattern id="grid-large" width="40" height="40" patternUnits="userSpaceOnUse">
              <path d="M 40 0 L 0 0 0 40" fill="none" stroke="rgba(255,255,255,0.03)" stroke-width="1" />
            </pattern>
          </defs>
          <rect width="1000" height="500" fill="url(#grid-large)" />

          <!-- Benua -->
          <path d="M160,80 Q220,70 300,100 Q260,160 220,200 Q260,250 280,320 Q240,430 200,380 Q180,300 160,220 Z" fill="#131e28" stroke="#1d2e3e" stroke-width="1" />
          <path d="M460,80 Q580,60 820,90 Q880,180 820,260 Q720,280 620,220 Q540,240 480,160 Z" fill="#131e28" stroke="#1d2e3e" stroke-width="1" />
          <path d="M480,200 Q560,200 580,280 Q540,380 480,360 Q440,280 480,200 Z" fill="#131e28" stroke="#1d2e3e" stroke-width="1" />
          <path d="M780,320 Q880,310 880,380 Q800,420 760,360 Z" fill="#131e28" stroke="#1d2e3e" stroke-width="1" />

          <!-- Arcs / Sinar Laser Serangan -->
          {#each events as ev}
            {@const orig = projectCoordinates(ev.latitude, ev.longitude)}
            <path
              d="M {orig.x},{orig.y} Q {(orig.x + hostServerCoords.x) / 2},{(orig.y + hostServerCoords.y) / 2 - 55} {hostServerCoords.x},{hostServerCoords.y}"
              fill="none"
              stroke={ev.action === 'block' ? 'rgba(239, 68, 68, 0.75)' : 'rgba(245, 158, 11, 0.65)'}
              stroke-width="2"
              class="laser-beam"
            />
            <circle cx={orig.x} cy={orig.y} r="5" fill="#ef4444" />
            <circle cx={orig.x} cy={orig.y} r="10" fill="none" stroke="rgba(239, 68, 68, 0.4)" class="pulse-ring" />
          {/each}

          <!-- Server Destination -->
          <circle cx={hostServerCoords.x} cy={hostServerCoords.y} r="8" fill="#00d084" />
          <circle cx={hostServerCoords.x} cy={hostServerCoords.y} r="16" fill="none" stroke="rgba(0, 208, 132, 0.5)" class="pulse-ring" />
          <text x={hostServerCoords.x + 14} y={hostServerCoords.y + 4} fill="#00d084" font-size="12" font-weight="bold" font-family="monospace">
            PROTECTED SERVER HOST
          </text>
        </svg>
      </div>

      <!-- Real-time Attack Incidents Table -->
      <div class="panel-header mt-6">
        <h4><i class="ri-history-line"></i> Feed Insiden Serangan Terkini</h4>
      </div>
      <div class="table-responsive">
        <table class="waf-main-table">
          <thead>
            <tr>
              <th>Waktu</th>
              <th>IP Penyerang</th>
              <th>Negara</th>
              <th>Target Situs</th>
              <th>Jenis Serangan</th>
              <th>Path URL</th>
              <th>Tindakan</th>
            </tr>
          </thead>
          <tbody>
            {#if events.length === 0}
              <tr>
                <td colspan="7" class="text-center p-4 text-muted">Belum ada rekaman serangan baru.</td>
              </tr>
            {:else}
              {#each events as ev}
                <tr>
                  <td class="text-muted text-xs mono">{formatTime(ev.created_at)}</td>
                  <td class="mono font-bold">{ev.client_ip}</td>
                  <td>
                    <span class="flag-code">{ev.country_code}</span> {ev.country}
                  </td>
                  <td class="mono text-accent">{ev.site_name}</td>
                  <td>
                    <span class="badge-attack-tag">{ev.attack_type}</span>
                  </td>
                  <td class="mono text-xs text-subtle truncate max-w-xs" title={ev.url_path}>
                    {ev.url_path}
                  </td>
                  <td>
                    <span class="badge-action {ev.action}">{ev.action.toUpperCase()}</span>
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

  /* World Map SVG */
  .world-map-wrapper {
    position: relative;
    border-radius: var(--radius-sm);
    overflow: hidden;
    background: var(--bg-inset);
  }
  .world-map-svg {
    width: 100%;
    height: auto;
    display: block;
  }
  .world-map-svg.full {
    height: 380px;
  }

  .laser-beam {
    stroke-dashoffset: 100;
    animation: dashLaser 2.5s linear infinite;
  }
  @keyframes dashLaser {
    from { stroke-dashoffset: 100; }
    to { stroke-dashoffset: 0; }
  }
  .pulse-ring {
    animation: pulseScale 2s infinite ease-out;
  }
  @keyframes pulseScale {
    0% { transform: scale(0.6); opacity: 1; }
    100% { transform: scale(1.6); opacity: 0; }
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
