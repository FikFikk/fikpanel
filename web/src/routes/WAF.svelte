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
    WAFAccessLog,
    CloudflareConfig,
    CloudflareZone,
    CloudflareDNSRecord,
  } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  // State Tab Navigasi WAF
  type WAFTab = 'overview' | 'live_access' | 'website' | 'attack_map' | 'blacklist' | 'cloudflare';
  let activeTab = $state<WAFTab>('overview');

  // State Telemetri & Filter Attack Map
  let selectedEvent = $state<WAFEvent | null>(null);
  let selectedAccessLog = $state<WAFAccessLog | null>(null);
  let attackTypeFilter = $state('all');
  let countryFilter = $state('');
  let mapSearchQuery = $state('');
  let attackMapInstance: L.Map | null = null;
  let overviewMapInstance: L.Map | null = null;
  let refreshOverviewMapFn: (() => void) | null = null;
  let refreshAttackMapFn: (() => void) | null = null;

  // State Filter Observatorium Lalu Lintas & Log Akses Real-time
  let accessLogs = $state<WAFAccessLog[]>([]);
  let loadingAccessLogs = $state(false);
  let accessDomainFilter = $state('all');
  let accessStatusFilter = $state<'all' | '2xx' | '3xx' | '4xx' | '5xx'>('all');
  let accessThreatFilter = $state<'all' | 'threat' | 'normal'>('all');
  let accessMethodFilter = $state('all');
  let accessSearchQuery = $state('');
  let autoRefresh = $state(true);

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
  let showCFSettingsModal = $state(false);
  let cfTokenInput = $state('');
  let cfEmailInput = $state('');
  let cfSubmitting = $state(false);
  let serverIP = $state('');
  let allServerIPs = $state<string[]>([]);
  let cfTokenDiagnostic = $state('');
  let cfZoneFilter = $state<'all' | 'points_here' | 'local_sites'>('all');
  let showCFPermGuide = $state(false);
  let expandedCFZones = $state<Record<string, boolean>>({});
  let dnsFilterTab = $state<'all' | 'points_here' | 'proxied'>('all');
  let dnsSearchQuery = $state('');

  // Form State Manajemen DNS Record Cloudflare
  let dnsEditId = $state('');
  let dnsType = $state<'A' | 'AAAA' | 'CNAME' | 'TXT' | 'MX'>('A');
  let dnsName = $state('');
  let dnsContent = $state('');
  let dnsProxied = $state(true);
  let dnsTTL = $state(1);
  let savingDNS = $state(false);

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
  async function loadOverview(silent = false) {
    try {
      if (!silent) loadingOverview = true;
      const res = await apiFetch<{ ok: boolean; overview: WAFOverview }>('/api/waf/overview');
      if (res?.overview) {
        overview = res.overview;
        if (res.overview.recent_access_logs && accessLogs.length === 0) {
          accessLogs = res.overview.recent_access_logs;
        }
        // Pastikan peta Overview langsung menggambar ulang titik & laser serangan setelah data tiba
        setTimeout(() => {
          overviewMapInstance?.invalidateSize();
          refreshOverviewMapFn?.();
        }, 40);
      }
    } catch (err: any) {
      if (!silent) notify(err.message || 'Gagal memuat metrik WAF', 'error');
    } finally {
      if (!silent) loadingOverview = false;
    }
  }

  // Ambil log akses HTTP lengkap secara real-time (normal + ancaman)
  async function loadAccessLogs(silent = false) {
    try {
      if (!silent) loadingAccessLogs = true;
      const res = await apiFetch<{ ok: boolean; logs: WAFAccessLog[] }>('/api/waf/access-logs?limit=250');
      if (res?.logs) {
        accessLogs = res.logs;
      }
    } catch (err: any) {
      if (!silent) notify(err.message || 'Gagal memuat log akses HTTP', 'error');
    } finally {
      if (!silent) loadingAccessLogs = false;
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
  async function loadEvents(silent = false) {
    try {
      if (!silent) loadingEvents = true;
      const res = await apiFetch<{ ok: boolean; events: WAFEvent[] }>('/api/waf/events?limit=80');
      if (res?.events) {
        events = res.events;
        setTimeout(() => {
          attackMapInstance?.invalidateSize();
          refreshAttackMapFn?.();
        }, 40);
      }
    } catch (err: any) {
      if (!silent) notify(err.message || 'Gagal memuat insiden serangan', 'error');
    } finally {
      if (!silent) loadingEvents = false;
    }
  }

  // Filter zona Cloudflare berdasarkan pilihan tab scope
  let filteredCFZones = $derived(
    cfZones.filter((z) => {
      if (cfZoneFilter === 'points_here') return !!z.points_to_server;
      if (cfZoneFilter === 'local_sites') return (z.local_sites?.length || 0) > 0;
      return true;
    })
  );

  // Cloudflare: Ambil konfigurasi dan daftar zone beserta diagnostik izin token
  async function loadCloudflare() {
    try {
      loadingCF = true;
      const cfgRes = await apiFetch<{ ok: boolean; config: CloudflareConfig }>('/api/cloudflare/config');
      if (cfgRes?.config) {
        cfConfig = cfgRes.config;
      }
      if (cfConfig?.connected) {
        const zonesRes = await apiFetch<{
          ok: boolean;
          zones: CloudflareZone[];
          server_ip: string;
          all_server_ips?: string[];
          token_diagnostic?: string;
        }>('/api/cloudflare/zones');
        if (zonesRes?.zones) {
          cfZones = zonesRes.zones;
          if (selectedZone) {
            const updated = zonesRes.zones.find((z) => z.id === selectedZone?.id || z.name === selectedZone?.name);
            if (updated) selectedZone = updated;
          }
        }
        if (zonesRes?.server_ip) {
          serverIP = zonesRes.server_ip;
        }
        if (zonesRes?.all_server_ips) {
          allServerIPs = zonesRes.all_server_ips;
        }
        cfTokenDiagnostic = zonesRes?.token_diagnostic || '';
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat data Cloudflare', 'error');
    } finally {
      loadingCF = false;
    }
  }

  // Cloudflare: Sambungkan API Token atau Global API Key
  async function handleConnectCF() {
    if (!cfTokenInput.trim()) {
      notify('Cloudflare API Token atau Global API Key wajib diisi', 'warning');
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
      cfTokenDiagnostic = '';
      notify('Integrasi Cloudflare telah diputuskan', 'info');
    } catch (err: any) {
      notify(err.message || 'Gagal memutuskan integrasi', 'error');
    }
  }

  // Cloudflare: Arahkan DNS domain ke IP publik server ini
  async function pointZoneHere(zone: CloudflareZone, proxied: boolean, targetHost?: string) {
    if (!serverIP) {
      notify('Alamat IP publik server belum terdeteksi', 'warning');
      return;
    }
    const hostToPoint = targetHost || zone.name;
    const ok = await confirmDialog({
      title: 'Arahkan DNS ke Server Ini',
      message: `Arahkan record A untuk ${hostToPoint} ke IP Publik Server ${serverIP} dengan status Proxy Cloudflare: ${proxied ? 'Aktif (Orange Cloud)' : 'Nonaktif (DNS Only / Grey Cloud)'}?`,
      confirmLabel: 'Arahkan Sekarang',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/cloudflare/zones/${zone.id}/point-here`, {
        method: 'POST',
        body: JSON.stringify({
          domain: hostToPoint,
          server_ip: serverIP,
          proxied: proxied,
        }),
      });
      notify(`DNS ${hostToPoint} berhasil diarahkan ke ${serverIP}`, 'success');
      await loadCloudflare();
    } catch (err: any) {
      notify(err.message || 'Gagal memperbarui DNS Cloudflare', 'error');
    }
  }

  // Cloudflare: Ubah pengaturan zona (ssl, security_level, always_use_https, automatic_https_rewrites, brotli, tls_1_3)
  async function updateCFZoneSetting(zone: CloudflareZone, setting: string, value: string) {
    try {
      await apiFetch(`/api/cloudflare/zones/${zone.id}/settings`, {
        method: 'POST',
        body: JSON.stringify({ setting, value }),
      });
      notify(`Pengaturan ${setting} pada ${zone.name} berhasil diubah menjadi ${value}`, 'success');
      await loadCloudflare();
    } catch (err: any) {
      notify(err.message || `Gagal memperbarui pengaturan ${setting}`, 'error');
      await loadCloudflare();
    }
  }

  // Cloudflare: Reset form DNS Record
  function resetDNSForm(defaultHost = '') {
    dnsEditId = '';
    dnsType = 'A';
    dnsName = defaultHost || selectedZone?.name || '';
    dnsContent = serverIP || '';
    dnsProxied = true;
    dnsTTL = 1;
  }

  // Cloudflare: Isi form untuk edit DNS Record
  function startEditDNSRecord(rec: CloudflareDNSRecord) {
    dnsEditId = rec.id;
    dnsType = (rec.type as any) || 'A';
    dnsName = rec.name;
    dnsContent = rec.content;
    dnsProxied = rec.proxied;
    dnsTTL = rec.ttl || 1;
  }

  // Cloudflare: Simpan (Tambah / Update) DNS Record
  async function saveCFDNSRecord() {
    if (!selectedZone) return;
    if (!dnsName.trim() || !dnsContent.trim()) {
      notify('Nama host dan target IP / Content wajib diisi', 'warning');
      return;
    }
    try {
      savingDNS = true;
      await apiFetch(`/api/cloudflare/zones/${selectedZone.id}/dns`, {
        method: 'POST',
        body: JSON.stringify({
          id: dnsEditId,
          type: dnsType,
          name: dnsName.trim(),
          content: dnsContent.trim(),
          proxied: dnsProxied,
          ttl: Number(dnsTTL) || 1,
        }),
      });
      notify(`Record DNS ${dnsType} ${dnsName} berhasil disimpan`, 'success');
      resetDNSForm();
      await loadCloudflare();
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan DNS record', 'error');
    } finally {
      savingDNS = false;
    }
  }

  // Cloudflare: Toggle status Proxy (Orange Cloud) pada 1 record DNS
  async function toggleCFDNSProxy(zone: CloudflareZone, rec: CloudflareDNSRecord) {
    try {
      await apiFetch(`/api/cloudflare/zones/${zone.id}/dns`, {
        method: 'POST',
        body: JSON.stringify({
          id: rec.id,
          type: rec.type,
          name: rec.name,
          content: rec.content,
          proxied: !rec.proxied,
          ttl: rec.ttl || 1,
        }),
      });
      rec.proxied = !rec.proxied;
      notify(`Status Proxy Cloudflare untuk ${rec.name}: ${rec.proxied ? 'Proxied (Orange Cloud)' : 'DNS Only (Grey Cloud)'}`, 'info');
      await loadCloudflare();
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status Proxy DNS', 'error');
    }
  }

  // Cloudflare: Hapus DNS Record
  async function deleteCFDNSRecord(zone: CloudflareZone, rec: CloudflareDNSRecord) {
    const ok = await confirmDialog({
      title: 'Hapus Record DNS',
      message: `Hapus record ${rec.type} "${rec.name}" → "${rec.content}" dari Cloudflare?`,
      confirmLabel: 'Hapus Record',
      variant: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/cloudflare/zones/${zone.id}/dns/${rec.id}`, {
        method: 'DELETE',
      });
      notify(`Record DNS ${rec.name} berhasil dihapus`, 'success');
      await loadCloudflare();
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus DNS record', 'error');
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

  // Daftar domain unik yang muncul di log akses
  let availableAccessDomains = $derived(
    (() => {
      const set = new Set<string>();
      for (const s of sites) set.add(s.site_name);
      for (const l of accessLogs) if (l.site_name) set.add(l.site_name);
      if (overview?.traffic_ranking) {
        for (const tr of overview.traffic_ranking) if (tr.domain) set.add(tr.domain);
      }
      return Array.from(set);
    })()
  );

  // Filter multi-dimensi untuk log akses HTTP real-time
  let filteredAccessLogs = $derived(
    accessLogs.filter((log) => {
      if (accessDomainFilter !== 'all' && log.site_name !== accessDomainFilter) return false;
      if (accessThreatFilter === 'threat' && !log.is_threat) return false;
      if (accessThreatFilter === 'normal' && log.is_threat) return false;
      if (accessMethodFilter !== 'all' && log.method !== accessMethodFilter) return false;
      if (accessStatusFilter === '2xx' && (log.status_code < 200 || log.status_code >= 300)) return false;
      if (accessStatusFilter === '3xx' && (log.status_code < 300 || log.status_code >= 400)) return false;
      if (accessStatusFilter === '4xx' && (log.status_code < 400 || log.status_code >= 500)) return false;
      if (accessStatusFilter === '5xx' && log.status_code < 500) return false;
      if (!accessSearchQuery.trim()) return true;
      const q = accessSearchQuery.toLowerCase();
      return (
        log.client_ip.toLowerCase().includes(q) ||
        log.url_path.toLowerCase().includes(q) ||
        log.site_name.toLowerCase().includes(q) ||
        (log.country && log.country.toLowerCase().includes(q)) ||
        (log.city && log.city.toLowerCase().includes(q)) ||
        (log.isp && log.isp.toLowerCase().includes(q)) ||
        (log.browser && log.browser.toLowerCase().includes(q)) ||
        (log.os && log.os.toLowerCase().includes(q)) ||
        (log.attack_type && log.attack_type.toLowerCase().includes(q)) ||
        (log.user_agent && log.user_agent.toLowerCase().includes(q))
      );
    })
  );

  // Format ukuran byte menjadi KB / MB / GB
  function formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const val = bytes / Math.pow(1024, i);
    return `${val >= 10 || i === 0 ? val.toFixed(0) : val.toFixed(2)} ${units[i]}`;
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
          remarks: reason || 'Diblokir langsung dari observatorium WAF',
        }),
      });
      notify(`IP ${ip} berhasil dimasukkan ke daftar hitam (Blacklist)!`, 'success');
      await loadRules();
    } catch (err: any) {
      notify(err.message || 'Gagal memblokir IP', 'error');
    }
  }

  // Masukkan IP ke daftar putih (Whitelist) seketika
  async function quickWhitelistIP(ip: string, reason?: string) {
    if (!ip) return;
    try {
      await apiFetch('/api/waf/rules', {
        method: 'POST',
        body: JSON.stringify({
          rule_type: 'ip_whitelist',
          ip_version: ip.includes(':') ? 'IPv6' : 'IPv4',
          start_ip: ip.trim(),
          remarks: reason || 'Diizinkan dari observatorium lalu lintas WAF',
        }),
      });
      notify(`IP ${ip} berhasil dimasukkan ke daftar putih (Whitelist)!`, 'success');
      await loadRules();
    } catch (err: any) {
      notify(err.message || 'Gagal menambahkan IP ke Whitelist', 'error');
    }
  }

  // Blokir pola URL path berbahaya ke URL Blacklist
  async function quickBlockPath(urlPath: string, reason?: string) {
    if (!urlPath) return;
    const cleanPath = urlPath.split('?')[0].trim();
    if (!cleanPath || cleanPath === '/') {
      notify('Tidak dapat memblokir root path "/"', 'warning');
      return;
    }
    try {
      await apiFetch('/api/waf/rules', {
        method: 'POST',
        body: JSON.stringify({
          rule_type: 'url_blacklist',
          pattern: cleanPath,
          remarks: reason || 'Blokir URL dari inspeksi paket HTTP',
        }),
      });
      notify(`Path URL "${cleanPath}" berhasil dimasukkan ke URL Blacklist!`, 'success');
      await loadRules();
    } catch (err: any) {
      notify(err.message || 'Gagal memblokir path URL', 'error');
    }
  }

  // Buka inspeksi untuk IP tertentu di Attack Map
  async function inspectAttackIP(ip: string) {
    activeTab = 'attack_map';
    mapSearchQuery = ip;
    await loadEvents();
    const match = events.find((e) => e.client_ip === ip);
    if (match) {
      selectedEvent = match;
      if (match.latitude && match.longitude) {
        setTimeout(() => flyToCountry(match.latitude, match.longitude), 150);
      }
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

  // Reaktif: Gambar ulang peta Overview setiap kali data recent_events berubah
  $effect(() => {
    const _evCount = overview?.recent_events?.length ?? 0;
    if (_evCount >= 0 && refreshOverviewMapFn) {
      refreshOverviewMapFn();
    }
  });

  // Reaktif: Gambar ulang peta Attack Map setiap kali filteredEvents berubah
  $effect(() => {
    const _fCount = filteredEvents.length;
    if (_fCount >= 0 && refreshAttackMapFn) {
      refreshAttackMapFn();
    }
  });

  // Svelte Action untuk peta ringkas di tab Overview (stabil saat browser di-refresh)
  function overviewMapAction(node: HTMLElement, _eventsParam?: WAFEvent[]) {
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
      L.marker([hostServerLocation.lat, hostServerLocation.lng], { icon: serverIcon, zIndexOffset: 1000 })
        .bindTooltip('FikPanel Origin Host (AWS Singapore)', { direction: 'top', className: 'dark-map-tooltip' })
        .addTo(layersGroup);

      // Titik serangan & laser arc (ambil dari overview.recent_events atau fallback ke events)
      const threatList = (overview?.recent_events && overview.recent_events.length > 0)
        ? overview.recent_events
        : events;

      threatList.slice(0, 24).forEach((ev) => {
        if (!ev.latitude && !ev.longitude) return;
        const isBlock = ev.action === 'block';
        const threatIcon = L.divIcon({
          className: 'leaflet-threat-icon',
          html: `<div class="threat-beacon-dot ${isBlock ? 'blocked' : 'warn'}"><div class="pulse-ring ${isBlock ? 'pulse-red' : 'pulse-amber'}"></div><span class="dot-core"></span></div>`,
          iconSize: [20, 20],
          iconAnchor: [10, 10],
        });
        const marker = L.marker([ev.latitude, ev.longitude], { icon: threatIcon }).addTo(layersGroup);

        marker.bindTooltip(`
          <div class="map-tooltip-content">
            <span class="flag-code">${ev.country_code}</span> <strong>${ev.country}</strong> · ${ev.city || 'Detected'}<br/>
            <span class="mono">${ev.client_ip}</span> · <span class="badge-tag-xs">${ev.attack_type}</span><br/>
            <span class="mono text-2xs text-subtle">${ev.method || 'GET'} ${ev.url_path}</span>
          </div>
        `, { direction: 'top', className: 'dark-map-tooltip' });

        marker.on('click', () => {
          selectedEvent = ev;
          activeTab = 'attack_map';
          loadEvents(true);
        });

        const arcPoints = createCurvedArc([ev.latitude, ev.longitude], [hostServerLocation.lat, hostServerLocation.lng]);
        L.polyline(arcPoints, {
          className: isBlock ? 'laser-arc-danger' : 'laser-arc-warn',
          weight: 2.2,
        }).addTo(layersGroup);
      });
    }

    refreshOverviewMapFn = updateLayers;
    updateLayers();

    // Pantau perubahan ukuran elemen agar ukuran canvas Leaflet selalu presisi setelah F5 / refresh
    const resizeObs = new ResizeObserver(() => {
      map.invalidateSize();
      updateLayers();
    });
    resizeObs.observe(node);

    setTimeout(() => {
      map.invalidateSize();
      updateLayers();
    }, 150);

    return {
      update() {
        map.invalidateSize();
        updateLayers();
      },
      destroy() {
        resizeObs.disconnect();
        if (refreshOverviewMapFn === updateLayers) refreshOverviewMapFn = null;
        map.remove();
        overviewMapInstance = null;
      },
    };
  }

  // Svelte Action untuk peta studio penuh di tab Attack Map
  function attackMapAction(node: HTMLElement, _eventsParam?: WAFEvent[]) {
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
          weight: 2.2,
        }).addTo(layersGroup);
      });
    }

    refreshAttackMapFn = renderAttackLayers;
    renderAttackLayers();

    const resizeObs = new ResizeObserver(() => {
      map.invalidateSize();
      renderAttackLayers();
    });
    resizeObs.observe(node);

    setTimeout(() => {
      map.invalidateSize();
      renderAttackLayers();
    }, 150);

    return {
      update() {
        map.invalidateSize();
        renderAttackLayers();
      },
      destroy() {
        resizeObs.disconnect();
        if (refreshAttackMapFn === renderAttackLayers) refreshAttackMapFn = null;
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

  function formatDateTime(ts: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString('id-ID', {
      day: '2-digit',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
  }

  onMount(() => {
    loadOverview();
    loadSites();
    loadEvents(true);
    loadAccessLogs(true);

    // Interval pembaruan otomatis real-time setiap 6 detik jika autoRefresh aktif
    const timer = setInterval(() => {
      if (!autoRefresh) return;
      if (activeTab === 'overview') {
        loadOverview(true);
      } else if (activeTab === 'live_access') {
        loadAccessLogs(true);
        loadOverview(true);
      } else if (activeTab === 'attack_map') {
        loadEvents(true);
      }
    }, 6000);

    return () => clearInterval(timer);
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
      <button
        type="button"
        class="btn-live-pulse {autoRefresh ? 'active' : ''}"
        onclick={() => autoRefresh = !autoRefresh}
        title="Pembaruan telemetri & peta otomatis setiap 6 detik"
      >
        <span class="pulse-dot {autoRefresh ? 'live' : ''}"></span>
        {autoRefresh ? 'Live Stream (6d)' : 'Live Dijeda'}
      </button>
      <span class="badge-status-glow">
        <span class="dot-glow"></span> Proteksi Mesin Aktif
      </span>
      <button class="btn btn-secondary btn-sm" onclick={() => { loadOverview(); loadAccessLogs(); loadEvents(); }} disabled={loadingOverview}>
        <i class="ri-refresh-line {loadingOverview ? 'spin' : ''}"></i> Segarkan
      </button>
    </div>
  </header>

  <!-- Sub-Tabs Navigation -->
  <nav class="waf-tabs-nav">
    <button
      class="waf-tab-btn {activeTab === 'overview' ? 'active' : ''}"
      onclick={() => { activeTab = 'overview'; loadOverview(); }}
    >
      <i class="ri-dashboard-2-line"></i> Overview
    </button>
    <button
      class="waf-tab-btn {activeTab === 'live_access' ? 'active' : ''}"
      onclick={() => { activeTab = 'live_access'; loadAccessLogs(); loadOverview(true); }}
    >
      <i class="ri-pulse-line"></i> Lalu Lintas & Akses Detail
      <span class="tab-count-pill">{accessLogs.length || overview?.recent_access_logs?.length || 0}</span>
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

  <!-- TAB 1: OVERVIEW -->
  {#if activeTab === 'overview'}
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
            <button class="btn-link text-xs text-accent" onclick={() => { activeTab = 'attack_map'; loadEvents(); }}>
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
                        onclick={() => inspectAttackIP(ipItem.ip)}
                      >
                        <i class="ri-search-eye-line"></i>
                      </button>
                      <button
                        class="btn-icon-xs text-danger"
                        title="Blokir IP ke Blacklist"
                        onclick={() => quickBlockIP(ipItem.ip, `Top Attacker (${ipItem.count}x)`)}
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
                    <span><i class="ri-hard-drive-2-line"></i> {formatBytes(tr.bytes_sent || 0)}</span>
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
                onclick={() => {
                  activeTab = 'live_access';
                  accessSearchQuery = vp.path;
                  loadAccessLogs();
                }}
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
                    <span class="text-subtle">{vp.domain || 'default'}</span>
                    {#if vp.last_status}
                      <span class="status-mini s{Math.floor(vp.last_status / 100)}xx">HTTP {vp.last_status}</span>
                    {/if}
                    {#if vp.bytes_sent}
                      <span>{formatBytes(vp.bytes_sent)}</span>
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
            onclick={() => { activeTab = 'live_access'; loadAccessLogs(); }}
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
                <td class="mono text-xs text-muted nowrap">{formatTime(log.timestamp)}</td>
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
                    onclick={() => selectedAccessLog = log}
                  >
                    <i class="ri-file-search-line"></i>
                  </button>
                  <button
                    class="btn-icon-xs text-danger"
                    title="Blokir IP ke Blacklist"
                    onclick={() => quickBlockIP(log.client_ip, log.attack_type || `Akses ke ${log.url_path}`)}
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

  <!-- TAB 1B: LIVE HTTP ACCESS & VISITOR OBSERVATORY -->
  {:else if activeTab === 'live_access'}
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
                      onclick={() => quickWhitelistIP(vis.ip, 'Trusted Visitor')}
                    >
                      <i class="ri-shield-check-line"></i>
                    </button>
                    <button
                      class="btn-icon-xs text-danger"
                      title="Blokir IP ke Blacklist"
                      onclick={() => quickBlockIP(vis.ip, `Visitor Activity (${vis.threats} threats)`)}
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
        <button class="btn btn-secondary btn-sm" onclick={() => loadAccessLogs()} disabled={loadingAccessLogs}>
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
                      onclick={() => selectedAccessLog = log}
                    >
                      <i class="ri-search-eye-line"></i>
                    </button>
                    <button
                      class="btn-icon-xs text-warning"
                      title="Blokir Path URL Ini"
                      onclick={() => quickBlockPath(log.url_path, `Blokir endpoint dari ${log.client_ip}`)}
                    >
                      <i class="ri-forbid-2-line"></i>
                    </button>
                    <button
                      class="btn-icon-xs text-danger"
                      title="Blokir IP ke Blacklist"
                      onclick={() => quickBlockIP(log.client_ip, log.attack_type || `Akses mencurigakan ke ${log.url_path}`)}
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

  <!-- TAB 5: CLOUDFLARE HUB -->
  {:else if activeTab === 'cloudflare'}
    <div class="bento-panel cf-hub-container">
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
            <button class="btn btn-secondary btn-sm" onclick={loadCloudflare} disabled={loadingCF}>
              <i class="ri-refresh-line {loadingCF ? 'spin' : ''}"></i> Sinkronkan Domain
            </button>
            <button class="btn btn-secondary btn-sm" onclick={() => (showCFConnectModal = true)}>
              <i class="ri-key-2-line"></i> Ganti Token / Izin
            </button>
            <button class="btn btn-danger-outline btn-sm" onclick={handleDisconnectCF}>
              <i class="ri-link-unlink-m"></i> Putuskan
            </button>
          {:else}
            <button class="btn btn-primary btn-sm" onclick={() => (showCFConnectModal = true)}>
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
          <button class="btn btn-primary" onclick={() => (showCFConnectModal = true)}>
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
                  onclick={() => (showCFConnectModal = true)}
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
                              onclick={() => {
                                selectedZone = zone;
                                resetDNSForm();
                                dnsFilterTab = 'proxied';
                                dnsSearchQuery = '';
                                showDNSModal = true;
                              }}
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
                            onchange={(e) => updateCFZoneSetting(zone, 'ssl', (e.currentTarget as HTMLSelectElement).value)}
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
                            onchange={(e) => updateCFZoneSetting(zone, 'security_level', (e.currentTarget as HTMLSelectElement).value)}
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
                          onclick={() => toggleDevMode(zone)}
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
                          onclick={() => { selectedZone = zone; resetDNSForm(); dnsFilterTab = 'points_here'; dnsSearchQuery = ''; showDNSModal = true; }}
                          title="Kelola Record DNS (Menampilkan domain server ini & subdomain Tunnel)"
                        >
                          <i class="ri-list-settings-line"></i>
                          <span>Kelola DNS</span>
                          <span class="cf-dns-count">{pointingRecs.length}/{zone.dns_records?.length || 0}</span>
                        </button>
                        <button
                          type="button"
                          class="cf-act-btn"
                          onclick={() => { selectedZone = zone; showCFSettingsModal = true; }}
                          title="Pengaturan Edge Cloudflare (SSL, Under Attack Mode, Always HTTPS, Brotli, TLS 1.3)"
                        >
                          <i class="ri-equalizer-line"></i>
                          <span>Edge Config</span>
                        </button>
                        <button
                          type="button"
                          class="cf-act-btn"
                          onclick={() => pointZoneHere(zone, true)}
                          title="Arahkan record A domain ke IP Publik server ini ({serverIP})"
                        >
                          <i class="ri-focus-3-line"></i>
                          <span>Point IP</span>
                        </button>
                        <button
                          type="button"
                          class="cf-act-btn danger-subtle"
                          onclick={() => purgeCFCache(zone)}
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
  {/if}
</div>

<!-- Modal Hubungkan Cloudflare Token / Global API Key -->
{#if showCFConnectModal}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) showCFConnectModal = false; }}
    onkeydown={(e) => { if (e.key === 'Escape') showCFConnectModal = false; }}
  >
    <div class="modal-card modal-wide">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-cloud-line text-accent"></i>
          <div>
            <h3>Hubungkan Akun Cloudflare (API Token / Global API Key)</h3>
            <span class="modal-sub">Mendukung Custom API Token maupun Global API Key + Email</span>
          </div>
        </div>
        <button class="btn-close" onclick={() => showCFConnectModal = false}>&times;</button>
      </div>

      <div class="modal-body">
        <div class="cf-token-guide-box mb-4">
          <div class="font-semibold text-xs mb-1">
            <i class="ri-shield-keyhole-line text-accent"></i> Panduan Izin API Token Cloudflare (Read vs Edit):
          </div>
          <ul class="cf-guide-list text-xs text-subtle">
            <li>
              <strong>Mengapa perlu izin <code>Edit</code>?</strong> Izin <code>Read</code> saja hanya dapat membaca daftar domain & DNS, tetapi <strong>tidak dapat</strong> mengubah DNS 1-klik, menambah subdomain, mengubah mode SSL, menyalakan <em>Under Attack Mode</em>, atau <em>Purge Cache</em>.
            </li>
            <li>
              Saat membuat token di <strong>dash.cloudflare.com/profile/api-tokens</strong> (<code>Create Custom Token</code>), tambahkan 4 baris izin berikut:
              <div class="mono text-2xs mt-1 p-2 bg-inset-box">
                1. Zone → Zone → Read (Wajib agar daftar domain terdeteksi)<br/>
                2. Zone → DNS → Edit (Untuk tambah/ubah/point record DNS & toggle Proxy)<br/>
                3. Zone → Zone Settings → Edit (Untuk ubah Mode SSL, Under Attack, Always HTTPS, Brotli)<br/>
                4. Zone → Cache Purge → Purge (Untuk bersihkan cache CDN)<br/>
                *Pada bagian Zone Resources, pilih: Include → All zones
              </div>
            </li>
            <li>
              <strong>Alternatif Paling Mudah:</strong> Anda juga dapat memasukkan <strong>Global API Key</strong> pada kolom Token di bawah disertai <strong>Email Akun Cloudflare</strong> Anda (otomatis memiliki akses penuh ke seluruh domain tanpa perlu mengatur permission satu per satu).
            </li>
          </ul>
        </div>

        <div class="form-group">
          <label for="cf-token">Cloudflare API Token atau Global API Key (Wajib)</label>
          <input
            id="cf-token"
            type="password"
            bind:value={cfTokenInput}
            placeholder="Tempel API Token (40+ karakter) atau Global API Key (37 karakter hex)..."
            class="input-dark mono"
          />
        </div>

        <div class="form-group mt-4">
          <label for="cf-email">Email Akun Cloudflare (Wajib jika memakai Global API Key, Opsional untuk API Token)</label>
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
          {cfSubmitting ? 'Memverifikasi & Menyinkronkan...' : 'Verifikasi & Hubungkan'}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal Manajemen DNS Records (Full CRUD + 1-Click Local Site Sync) -->
{#if showDNSModal && selectedZone}
  {@const allZoneRecs = selectedZone.dns_records || []}
  {@const filteredZoneRecs = allZoneRecs.filter((r) => {
    if (dnsFilterTab === 'points_here' && !r.points_to_server) return false;
    if (dnsFilterTab === 'proxied' && !r.proxied) return false;
    if (dnsSearchQuery.trim()) {
      const q = dnsSearchQuery.trim().toLowerCase();
      return r.name.toLowerCase().includes(q) || r.content.toLowerCase().includes(q) || r.type.toLowerCase().includes(q);
    }
    return true;
  })}
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
            <h3>Manajemen DNS Cloudflare: {selectedZone.name}</h3>
            <span class="modal-sub">
              IP Publik Server Ini: <strong class="mono text-accent">{serverIP}</strong> · Kelola record A, AAAA, CNAME, TXT, MX & Proxy Orange Cloud
            </span>
          </div>
        </div>
        <button class="btn-close" onclick={() => showDNSModal = false}>&times;</button>
      </div>

      <div class="modal-body">
        <!-- Pintasan 1-Klik Arahkan Situs Lokal ke Server Ini -->
        {#if selectedZone.local_sites && selectedZone.local_sites.length > 0}
          <div class="cf-quick-sync-bar mb-3">
            <span class="text-xs font-semibold"><i class="ri-magic-line text-accent"></i> Pintasan Situs FikPanel:</span>
            {#each selectedZone.local_sites as ls}
              <button
                type="button"
                class="btn btn-xs btn-secondary mono"
                onclick={() => pointZoneHere(selectedZone!, true, ls)}
                title="Arahkan {ls} ke {serverIP} (Proxied)"
              >
                <i class="ri-cloud-fill text-warning"></i> Point {ls} → {serverIP} (Proxied)
              </button>
              <button
                type="button"
                class="btn btn-xs btn-ghost mono"
                onclick={() => pointZoneHere(selectedZone!, false, ls)}
                title="Arahkan {ls} ke {serverIP} (DNS Only)"
              >
                <i class="ri-cloud-line"></i> {ls} (DNS Only)
              </button>
            {/each}
          </div>
        {/if}

        <!-- Form Tambah / Ubah Record DNS -->
        <div class="cf-dns-form-card mb-4">
          <div class="text-xs font-semibold mb-2">
            <i class="ri-add-circle-line text-accent"></i>
            {dnsEditId ? `Ubah Record DNS (${dnsName})` : 'Tambah Record DNS Baru'}
          </div>
          <div class="cf-dns-form-grid">
            <select bind:value={dnsType} class="input-dark select-sm mono">
              <option value="A">A</option>
              <option value="AAAA">AAAA</option>
              <option value="CNAME">CNAME</option>
              <option value="TXT">TXT</option>
              <option value="MX">MX</option>
            </select>

            <input
              type="text"
              bind:value={dnsName}
              placeholder="Nama (mis. @ atau sub.{selectedZone.name})"
              class="input-dark mono text-xs"
            />

            <div class="cf-content-input-group">
              <input
                type="text"
                bind:value={dnsContent}
                placeholder={dnsType === 'A' ? `IPv4 (mis. ${serverIP})` : 'Target / Content'}
                class="input-dark mono text-xs"
              />
              {#if dnsType === 'A' && serverIP}
                <button
                  type="button"
                  class="btn btn-xs btn-secondary nowrap"
                  onclick={() => dnsContent = serverIP}
                  title="Gunakan IP Publik Server FikPanel ini"
                >
                  Pakai IP Server
                </button>
              {/if}
            </div>

            <label class="custom-chk text-xs nowrap" title="Aktifkan Cloudflare CDN & WAF Proxy (Orange Cloud)">
              <input type="checkbox" bind:checked={dnsProxied} disabled={dnsType === 'TXT' || dnsType === 'MX'} />
              <span>Proxied ☁️</span>
            </label>

            <div class="cf-dns-form-btns">
              <button
                type="button"
                class="btn btn-primary btn-sm"
                onclick={saveCFDNSRecord}
                disabled={savingDNS || selectedZone.read_only}
              >
                {savingDNS ? 'Menyimpan...' : dnsEditId ? 'Update' : '+ Simpan'}
              </button>
              {#if dnsEditId}
                <button type="button" class="btn btn-ghost btn-sm" onclick={() => resetDNSForm()}>
                  Batal
                </button>
              {/if}
            </div>
          </div>
        </div>

        <!-- Filter & Pencarian Cepat Record DNS di Modal -->
        <div class="cf-dns-toolbar">
          <div class="cf-filter-tabs">
            <button
              type="button"
              class="cf-filter-btn {dnsFilterTab === 'all' ? 'active' : ''}"
              onclick={() => (dnsFilterTab = 'all')}
            >
              Semua ({allZoneRecs.length})
            </button>
            <button
              type="button"
              class="cf-filter-btn {dnsFilterTab === 'points_here' ? 'active' : ''}"
              onclick={() => (dnsFilterTab = 'points_here')}
            >
              Mengarah ke Server Ini ({allZoneRecs.filter((r) => r.points_to_server).length})
            </button>
            <button
              type="button"
              class="cf-filter-btn {dnsFilterTab === 'proxied' ? 'active' : ''}"
              onclick={() => (dnsFilterTab = 'proxied')}
            >
              Proxied / Tunnel ({allZoneRecs.filter((r) => r.proxied).length})
            </button>
          </div>
          <input
            type="text"
            bind:value={dnsSearchQuery}
            placeholder="Cari subdomain atau IP..."
            class="input-dark cf-dns-search-input mono text-xs"
          />
        </div>

        <div class="table-responsive cf-dns-table-scroll">
          <table class="waf-main-table">
            <thead>
              <tr>
                <th>Type</th>
                <th>Name (Subdomain)</th>
                <th>Content / Target IP</th>
                <th>Proxy Status</th>
                <th>Status di Server Ini</th>
                <th class="text-right">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {#if filteredZoneRecs.length === 0}
                <tr>
                  <td colspan="6" class="text-center p-4 text-muted">Tidak ada rekaman DNS yang sesuai dengan filter.</td>
                </tr>
              {:else}
                {#each filteredZoneRecs as rec}
                  <tr>
                    <td><span class="badge-soft mono">{rec.type}</span></td>
                    <td class="mono font-semibold text-xs">{rec.name}</td>
                    <td class="mono text-xs text-subtle break-all max-w-xs">{rec.content}</td>
                    <td>
                      <button
                        type="button"
                        class="btn-proxy-toggle {rec.proxied ? 'proxied' : 'dns-only'}"
                        disabled={selectedZone.read_only || (rec.type !== 'A' && rec.type !== 'AAAA' && rec.type !== 'CNAME')}
                        onclick={() => toggleCFDNSProxy(selectedZone!, rec)}
                        title="Klik untuk beralih antara Proxied (Orange Cloud) dan DNS Only (Grey Cloud)"
                      >
                        {#if rec.proxied}
                          <span class="badge-orange-cloud"><i class="ri-cloud-fill"></i> Proxied (CDN)</span>
                        {:else}
                          <span class="badge-grey-cloud"><i class="ri-cloud-line"></i> DNS Only</span>
                        {/if}
                      </button>
                    </td>
                    <td>
                      {#if rec.points_to_server}
                        <div class="cf-server-match-cell">
                          <span class="badge-points-here"><i class="ri-check-line"></i> Mengarah ke Server Ini</span>
                          {#if rec.in_local_sites || (selectedZone.local_sites || []).includes(rec.name)}
                            <span class="cf-site-tag registered"><i class="ri-check-line"></i> Terdaftar di Website</span>
                          {:else}
                            <span class="cf-site-tag unregistered">Belum di Website</span>
                          {/if}
                        </div>
                      {:else if rec.proxied}
                        <span class="cf-source-tag warn"><i class="ri-git-branch-line"></i> Cloudflare Tunnel / Proxy Lain</span>
                      {:else}
                        <span class="text-muted text-xs">IP Server Lain ({rec.content})</span>
                      {/if}
                    </td>
                    <td class="text-right nowrap">
                      <button
                        type="button"
                        class="btn-icon-xs text-accent"
                        disabled={selectedZone.read_only}
                        onclick={() => startEditDNSRecord(rec)}
                        title="Edit Record DNS Ini"
                      >
                        <i class="ri-edit-line"></i>
                      </button>
                      <button
                        type="button"
                        class="btn-icon-xs text-danger"
                        disabled={selectedZone.read_only}
                        onclick={() => deleteCFDNSRecord(selectedZone!, rec)}
                        title="Hapus Record DNS Ini"
                      >
                        <i class="ri-delete-bin-line"></i>
                      </button>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>

      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" onclick={() => showDNSModal = false}>Tutup</button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal Pengaturan Edge & Keamanan Cloudflare -->
{#if showCFSettingsModal && selectedZone}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) showCFSettingsModal = false; }}
    onkeydown={(e) => { if (e.key === 'Escape') showCFSettingsModal = false; }}
  >
    <div class="modal-card modal-wide">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-shield-star-line text-accent"></i>
          <div>
            <h3>Pengaturan Edge & Keamanan Cloudflare: {selectedZone.name}</h3>
            <span class="modal-sub">Konfigurasi SSL/TLS, DDoS Under Attack Mode, HTTPS Redirect, dan Optimasi Kecepatan</span>
          </div>
        </div>
        <button class="btn-close" onclick={() => showCFSettingsModal = false}>&times;</button>
      </div>

      <div class="modal-body stack-gap">
        <div class="forensic-modal-grid">
          <!-- Kolom 1: SSL/TLS & HTTPS -->
          <div class="drawer-card">
            <div class="drawer-card-title"><i class="ri-lock-password-line text-accent"></i> Enkripsi SSL/TLS & HTTPS</div>
            <div class="stack-gap mt-2">
              <div class="form-group">
                <label for="cf-ssl-select">Mode Enkripsi SSL/TLS</label>
                <select
                  id="cf-ssl-select"
                  class="input-dark"
                  value={selectedZone.ssl_mode || 'full'}
                  disabled={selectedZone.read_only}
                  onchange={(e) => updateCFZoneSetting(selectedZone!, 'ssl', (e.currentTarget as HTMLSelectElement).value)}
                >
                  <option value="off">Off — Tidak mengenkripsi trafik</option>
                  <option value="flexible">Flexible — HTTPS ke Cloudflare, HTTP ke Origin Server</option>
                  <option value="full">Full — End-to-end HTTPS (Mendukung Self-Signed)</option>
                  <option value="strict">Full (Strict) — End-to-end HTTPS dengan Sertifikat Valid (Let's Encrypt)</option>
                </select>
              </div>

              <label class="custom-chk">
                <input
                  type="checkbox"
                  checked={selectedZone.always_use_https}
                  disabled={selectedZone.read_only}
                  onchange={(e) => updateCFZoneSetting(selectedZone!, 'always_use_https', (e.currentTarget as HTMLInputElement).checked ? 'on' : 'off')}
                />
                <span><strong>Always Use HTTPS</strong> — Alihkan semua permintaan <code>http://</code> ke <code>https://</code> di Edge</span>
              </label>

              <label class="custom-chk">
                <input
                  type="checkbox"
                  checked={selectedZone.auto_https_rewrites}
                  disabled={selectedZone.read_only}
                  onchange={(e) => updateCFZoneSetting(selectedZone!, 'automatic_https_rewrites', (e.currentTarget as HTMLInputElement).checked ? 'on' : 'off')}
                />
                <span><strong>Automatic HTTPS Rewrites</strong> — Perbaiki tautan mixed-content HTTP secara otomatis</span>
              </label>

              <label class="custom-chk">
                <input
                  type="checkbox"
                  checked={selectedZone.tls_1_3}
                  disabled={selectedZone.read_only}
                  onchange={(e) => updateCFZoneSetting(selectedZone!, 'tls_1_3', (e.currentTarget as HTMLInputElement).checked ? 'on' : 'off')}
                />
                <span><strong>TLS 1.3</strong> — Aktifkan protokol enkripsi TLS 1.3 terbaru & tercepat</span>
              </label>
            </div>
          </div>

          <!-- Kolom 2: Proteksi DDoS & Cache CDN -->
          <div class="drawer-card">
            <div class="drawer-card-title"><i class="ri-radar-line text-danger"></i> Mitigasi DDoS & Performa Edge CDN</div>
            <div class="stack-gap mt-2">
              <div class="form-group">
                <label for="cf-sec-select">Security Level & Mode DDoS</label>
                <select
                  id="cf-sec-select"
                  class="input-dark"
                  value={selectedZone.security_level || 'medium'}
                  disabled={selectedZone.read_only}
                  onchange={(e) => updateCFZoneSetting(selectedZone!, 'security_level', (e.currentTarget as HTMLSelectElement).value)}
                >
                  <option value="under_attack">🔥 I'm Under Attack! — Tampilkan tantangan JS untuk semua pengunjung</option>
                  <option value="high">High — Tantang semua pengunjung berisiko dalam 14 hari terakhir</option>
                  <option value="medium">Medium — Tantang pengunjung berisiko sedang & tinggi</option>
                  <option value="low">Low — Hanya tantang pengunjung paling berbahaya</option>
                  <option value="essentially_off">Essentially Off — Hanya tantang bot paling parah</option>
                </select>
              </div>

              <label class="custom-chk">
                <input
                  type="checkbox"
                  checked={selectedZone.brotli}
                  disabled={selectedZone.read_only}
                  onchange={(e) => updateCFZoneSetting(selectedZone!, 'brotli', (e.currentTarget as HTMLInputElement).checked ? 'on' : 'off')}
                />
                <span><strong>Brotli Compression</strong> — Percepat pemuatan halaman dengan kompresi Brotli</span>
              </label>

              <label class="custom-chk">
                <input
                  type="checkbox"
                  checked={selectedZone.development_mode === 1}
                  disabled={selectedZone.read_only}
                  onchange={() => toggleDevMode(selectedZone!)}
                />
                <span><strong>Development Mode (Bypass Cache 3 Jam)</strong> — Lewati cache CDN saat menguji kode baru</span>
              </label>

              <div class="pt-2">
                <button
                  type="button"
                  class="btn btn-secondary btn-sm w-full"
                  disabled={selectedZone.read_only}
                  onclick={() => purgeCFCache(selectedZone!)}
                >
                  <i class="ri-delete-bin-7-line text-warning"></i> Kosongkan Seluruh Cache CDN (Purge Everything)
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" onclick={() => showCFSettingsModal = false}>Tutup</button>
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

<!-- Modal Forensik Paket HTTP & Akses Mendalam -->
{#if selectedAccessLog}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) selectedAccessLog = null; }}
    onkeydown={(e) => { if (e.key === 'Escape') selectedAccessLog = null; }}
  >
    <div class="modal-card modal-wide">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-radar-line {selectedAccessLog.is_threat ? 'text-danger' : 'text-accent'}"></i>
          <div>
            <h3>Inspeksi Forensik Paket HTTP #{selectedAccessLog.id}</h3>
            <span class="modal-sub">
              {formatDateTime(selectedAccessLog.timestamp)} · {selectedAccessLog.is_threat ? `Ancaman Terdeteksi: ${selectedAccessLog.attack_type}` : 'Lalu Lintas Pengunjung Normal'}
            </span>
          </div>
        </div>
        <button class="btn-close" onclick={() => selectedAccessLog = null}>&times;</button>
      </div>

      <div class="modal-body">
        <div class="forensic-modal-grid">
          <!-- Kolom 1: Informasi Request & Target -->
          <div class="drawer-card">
            <div class="drawer-card-title"><i class="ri-links-line text-accent"></i> Detail Permintaan HTTP & Endpoint</div>
            <div class="drawer-info-grid">
              <div class="info-row">
                <span class="info-lbl">Domain Vhost:</span>
                <span class="info-val mono font-bold text-accent">{selectedAccessLog.site_name}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Metode & Protokol:</span>
                <span class="info-val mono">
                  <span class="method-mini {selectedAccessLog.method.toLowerCase()}">{selectedAccessLog.method}</span>
                  {selectedAccessLog.protocol}
                </span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Kode Status HTTP:</span>
                <span class="info-val">
                  <span class="http-status-badge s{Math.floor(selectedAccessLog.status_code / 100)}xx">{selectedAccessLog.status_code}</span>
                </span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Ukuran Payload Dikirim:</span>
                <span class="info-val mono">{formatBytes(selectedAccessLog.bytes_sent)} ({selectedAccessLog.bytes_sent.toLocaleString()} bytes)</span>
              </div>
              <div class="info-col">
                <span class="info-lbl">URL Path / Payload Lengkap:</span>
                <div class="payload-box mono text-xs">{selectedAccessLog.url_path}</div>
              </div>
              {#if selectedAccessLog.referer}
                <div class="info-col">
                  <span class="info-lbl">HTTP Referer:</span>
                  <div class="ua-box mono text-2xs">{selectedAccessLog.referer}</div>
                </div>
              {/if}
            </div>
          </div>

          <!-- Kolom 2: Identitas Klien, Geolokasi & Perangkat -->
          <div class="drawer-card">
            <div class="drawer-card-title"><i class="ri-map-pin-user-line text-info"></i> Atribusi IP, Geolokasi & Perangkat</div>
            <div class="drawer-info-grid">
              <div class="info-row">
                <span class="info-lbl">Alamat IP Klien:</span>
                <span class="info-val mono font-bold">{selectedAccessLog.client_ip}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Negara & Kota:</span>
                <span class="info-val">
                  <span class="flag-code">{selectedAccessLog.country_code}</span>
                  {selectedAccessLog.city ? `${selectedAccessLog.city}, ` : ''}{selectedAccessLog.region ? `${selectedAccessLog.region}, ` : ''}{selectedAccessLog.country}
                </span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Koordinat GPS:</span>
                <span class="info-val mono text-xs">{selectedAccessLog.latitude.toFixed(4)}, {selectedAccessLog.longitude.toFixed(4)}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">ISP / Jaringan ASN:</span>
                <span class="info-val text-xs">{selectedAccessLog.isp || 'Autonomous System'}</span>
              </div>
              <div class="info-row">
                <span class="info-lbl">Perangkat & OS:</span>
                <span class="info-val">{selectedAccessLog.device} · {selectedAccessLog.os} · {selectedAccessLog.browser}</span>
              </div>
              <div class="info-col">
                <span class="info-lbl">Raw User-Agent Header:</span>
                <div class="ua-box mono text-2xs">{selectedAccessLog.user_agent || '(Kosong / Raw Socket)'}</div>
              </div>
              <div class="info-col">
                <span class="info-lbl">Keputusan Mesin WAF:</span>
                <div class="rule-box text-xs">
                  {#if selectedAccessLog.is_threat}
                    <strong class="text-danger">{selectedAccessLog.attack_type}</strong> — {selectedAccessLog.rule_name || 'Signature Inspection Engine'} ({selectedAccessLog.action.toUpperCase()})
                  {:else}
                    <strong class="text-success">Normal HTTP Request</strong> — Tidak ditemukan pola SQLi, XSS, LFI, atau scanner berbahaya.
                  {/if}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="modal-actions">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          onclick={() => {
            if (selectedAccessLog) {
              quickWhitelistIP(selectedAccessLog.client_ip, 'Whitelisted via Packet Forensic Modal');
              selectedAccessLog = null;
            }
          }}
        >
          <i class="ri-shield-check-line text-success"></i> Whitelist IP
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          onclick={() => {
            if (selectedAccessLog) {
              quickBlockPath(selectedAccessLog.url_path, `Blocked endpoint from ${selectedAccessLog.client_ip}`);
              selectedAccessLog = null;
            }
          }}
        >
          <i class="ri-forbid-2-line text-warning"></i> Blokir URL Path Ini
        </button>
        <button
          type="button"
          class="btn btn-danger btn-sm"
          onclick={() => {
            if (selectedAccessLog) {
              quickBlockIP(selectedAccessLog.client_ip, selectedAccessLog.attack_type || `Forensic block (${selectedAccessLog.url_path})`);
              selectedAccessLog = null;
            }
          }}
        >
          <i class="ri-shield-cross-line"></i> Blokir IP ke Blacklist
        </button>
        <button type="button" class="btn btn-ghost btn-sm" onclick={() => selectedAccessLog = null}>Tutup</button>
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
  .cf-hub-container {
    display: flex;
    flex-direction: column;
    gap: 1.15rem;
  }
  .cf-status-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 1.25rem 1.4rem;
    gap: 1.5rem;
    flex-wrap: wrap;
  }
  .cf-left {
    display: flex;
    align-items: flex-start;
    gap: 1rem;
    flex: 1;
    min-width: 280px;
  }
  .cf-head-copy {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    min-width: 0;
  }
  .cf-head-desc {
    margin: 0;
    font-size: 0.78rem;
    color: var(--text-subtle);
    line-height: 1.55;
    max-width: 72ch;
  }
  .cf-actions {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-shrink: 0;
    flex-wrap: wrap;
  }
  .cf-logo-icon {
    width: 46px;
    height: 46px;
    flex-shrink: 0;
    background: var(--tint-warning);
    color: var(--warning-strong);
    border: 1px solid var(--ring-warning);
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.5rem;
    box-shadow: 0 0 16px var(--tint-warning);
  }
  .cf-title-row {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    flex-wrap: wrap;
  }
  .cf-title-row h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 700;
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

  /* Live Pulse Toggle & Observatory Styles */
  .btn-live-pulse {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    color: var(--text-subtle);
    padding: 0.35rem 0.75rem;
    border-radius: 9999px;
    font-size: 0.76rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
  }
  .btn-live-pulse.active {
    background: var(--tint-accent);
    border-color: var(--ring-accent);
    color: var(--accent-strong);
  }
  .pulse-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--text-muted);
  }
  .pulse-dot.live {
    background: var(--accent);
    box-shadow: 0 0 8px var(--accent);
    animation: pulseGlow 1.5s infinite ease-in-out;
  }

  .tab-count-pill {
    background: var(--tint-accent);
    color: var(--accent-strong);
    padding: 0.08rem 0.45rem;
    border-radius: 9999px;
    font-size: 0.68rem;
    font-weight: 700;
  }

  .status-telemetry-strip {
    padding: 0.9rem 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
  }
  .telemetry-strip-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.75rem;
  }
  .strip-title {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--text);
  }
  .strip-badges {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.6rem;
  }
  .status-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.2rem 0.55rem;
    border-radius: 6px;
    font-size: 0.72rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    color: var(--text-subtle);
  }
  .status-pill .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
  }
  .status-pill.s2xx .dot { background: var(--accent); }
  .status-pill.s3xx .dot { background: var(--info); }
  .status-pill.s4xx .dot { background: var(--warning); }
  .status-pill.s5xx .dot { background: var(--danger); }

  .status-progress-bar {
    height: 7px;
    width: 100%;
    background: var(--bg-inset);
    border-radius: 9999px;
    overflow: hidden;
    display: flex;
    gap: 2px;
  }
  .status-progress-bar .seg {
    height: 100%;
    transition: width 0.4s ease;
  }
  .seg-2xx { background: var(--accent); }
  .seg-3xx { background: var(--info); }
  .seg-4xx { background: var(--warning); }
  .seg-5xx { background: var(--danger); }

  .live-mini-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--danger);
    box-shadow: 0 0 6px var(--danger);
    margin-right: 4px;
    vertical-align: middle;
  }

  .rank-item-rich {
    align-items: flex-start;
    padding: 0.55rem 0.65rem;
  }
  .rank-rich-body {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .rank-rich-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
  }
  .rank-rich-meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.55rem;
    color: var(--text-muted);
  }
  .rank-list-scroll {
    max-height: 320px;
    overflow-y: auto;
    padding-right: 2px;
  }
  .rank-item-clickable {
    width: 100%;
    text-align: left;
    background: var(--bg-inset);
    border: 1px solid transparent;
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .rank-item-clickable:hover {
    border-color: var(--ring-accent);
    background: var(--tint-accent);
  }
  .rank-threat-row {
    border-left: 2px solid var(--danger);
  }

  .method-mini {
    display: inline-block;
    padding: 0.08rem 0.35rem;
    border-radius: 4px;
    font-size: 0.65rem;
    font-weight: 700;
    text-transform: uppercase;
    margin-right: 0.25rem;
    background: var(--tint-info);
    color: var(--info);
  }
  .method-mini.post {
    background: var(--tint-accent);
    color: var(--accent-strong);
  }
  .method-mini.delete,
  .method-mini.connect,
  .method-mini.raw {
    background: var(--tint-danger);
    color: var(--danger-strong);
  }

  .status-mini,
  .http-status-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 0.14rem 0.45rem;
    border-radius: 4px;
    font-family: var(--font-mono);
    font-size: 0.72rem;
    font-weight: 700;
  }
  .s2xx {
    background: var(--tint-accent);
    color: var(--accent-strong);
    border: 1px solid var(--ring-accent);
  }
  .s3xx {
    background: var(--tint-info);
    color: var(--info);
    border: 1px solid var(--ring-info);
  }
  .s4xx {
    background: var(--tint-warning);
    color: var(--warning-strong);
    border: 1px solid var(--ring-warning);
  }
  .s5xx {
    background: var(--tint-danger);
    color: var(--danger-strong);
    border: 1px solid var(--ring-danger);
  }

  .badge-normal-traffic {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    background: var(--tint-accent);
    color: var(--accent-strong);
    border: 1px solid var(--ring-accent);
    padding: 0.18rem 0.5rem;
    border-radius: 4px;
    font-size: 0.7rem;
    font-weight: 600;
  }
  .row-threat-highlight {
    background: var(--tint-danger);
  }
  .forensic-modal-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }
  @media (max-width: 900px) {
    .forensic-modal-grid {
      grid-template-columns: 1fr;
    }
  }
  .nowrap {
    white-space: nowrap;
  }
  .break-all {
    word-break: break-all;
  }
  .mt-2 {
    margin-top: 0.5rem;
  }
  .mt-05 {
    margin-top: 0.15rem;
  }

  /* Komponen Modal Overlay & Card Global di WAF */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    z-index: 1200;
    background: rgb(0 0 0 / 0.72);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1.25rem;
  }
  .modal-card {
    background: var(--bg-surface);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-md);
    width: 100%;
    max-width: 600px;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 20px 50px rgb(0 0 0 / 0.6);
    overflow: hidden;
  }
  .modal-card.modal-wide {
    max-width: 920px;
  }
  .modal-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--border);
    background: var(--bg-inset);
  }
  .modal-title-wrap {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }
  .modal-title-wrap i {
    font-size: 1.35rem;
  }
  .modal-title-wrap h3 {
    margin: 0;
    font-size: 0.98rem;
    font-weight: 700;
    color: var(--text);
  }
  .modal-sub {
    display: block;
    font-size: 0.74rem;
    color: var(--text-muted);
    margin-top: 0.12rem;
  }
  .btn-close {
    background: transparent;
    border: 1px solid transparent;
    color: var(--text-muted);
    font-size: 1.35rem;
    line-height: 1;
    width: 30px;
    height: 30px;
    border-radius: 6px;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }
  .btn-close:hover {
    background: var(--bg-hover);
    color: var(--text);
    border-color: var(--border);
  }
  .modal-body {
    padding: 1.15rem 1.25rem;
    overflow-y: auto;
    flex: 1;
  }
  .modal-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.6rem;
    padding: 0.85rem 1.25rem;
    border-top: 1px solid var(--border);
    background: var(--bg-inset);
  }

  /* Komponen Cloudflare Hub & Diagnostik Izin Token (Collapsible) */
  .cf-diagnostic-banner {
    background: var(--bg-inset);
    border: 1px solid var(--ring-warning);
    border-left: 3px solid var(--warning-strong);
    border-radius: 12px;
    padding: 1rem 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .cf-diag-compact-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1.25rem;
    flex-wrap: wrap;
  }
  .cf-diag-left {
    display: flex;
    align-items: flex-start;
    gap: 0.85rem;
    flex: 1;
    min-width: 260px;
  }
  .cf-diag-icon-box {
    width: 36px;
    height: 36px;
    flex-shrink: 0;
    border-radius: 9px;
    background: var(--tint-warning);
    border: 1px solid var(--ring-warning);
    color: var(--warning-strong);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.1rem;
  }
  .cf-diag-summary {
    display: flex;
    flex-direction: column;
    gap: 0.22rem;
    min-width: 0;
  }
  .cf-diag-headline {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .cf-diag-headline strong {
    font-size: 0.84rem;
    color: var(--text);
    font-weight: 700;
  }
  .cf-diag-pill {
    font-size: 0.66rem;
    font-weight: 600;
    padding: 0.1rem 0.45rem;
    border-radius: 9999px;
    background: var(--tint-warning);
    color: var(--warning-strong);
    border: 1px solid var(--ring-warning);
  }
  .cf-diag-subtext {
    margin: 0;
    font-size: 0.76rem;
    color: var(--text-muted);
    line-height: 1.5;
  }
  .cf-diag-subtext code {
    font-family: var(--font-mono);
    font-size: 0.72rem;
    color: var(--warning-strong);
    background: var(--bg-surface);
    padding: 0.08rem 0.35rem;
    border-radius: 4px;
    border: 1px solid var(--border);
  }
  .cf-diag-right {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-shrink: 0;
    flex-wrap: wrap;
  }
  .cf-guide-toggle-btn.active {
    border-color: var(--ring-accent);
    background: var(--tint-accent);
    color: var(--accent-strong);
  }
  .cf-diag-expanded {
    border-top: 1px solid var(--border);
    padding-top: 0.95rem;
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
  }
  .cf-diag-note-row {
    display: flex;
    align-items: flex-start;
    gap: 0.5rem;
    font-size: 0.75rem;
    color: var(--text-muted);
    line-height: 1.5;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.65rem 0.85rem;
  }
  .cf-perm-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(215px, 1fr));
    gap: 0.8rem;
  }
  .cf-perm-box {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 0.85rem 0.95rem;
    display: flex;
    flex-direction: column;
    gap: 0.38rem;
  }
  .cf-perm-box.highlight {
    border-color: var(--ring-accent);
    background: var(--tint-accent);
  }
  .cf-perm-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.4rem;
  }
  .cf-perm-step {
    font-size: 0.66rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-subtle);
  }
  .cf-perm-badge {
    font-size: 0.64rem;
    font-weight: 600;
    padding: 0.08rem 0.4rem;
    border-radius: 4px;
    background: var(--bg-inset);
    color: var(--text-muted);
    border: 1px solid var(--border);
  }
  .cf-perm-badge.req {
    background: var(--tint-accent);
    color: var(--accent-strong);
    border-color: var(--ring-accent);
  }
  .cf-perm-title {
    font-size: 0.78rem;
    font-weight: 700;
    color: var(--text);
  }
  .cf-perm-path {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.22rem;
    font-size: 0.7rem;
    color: var(--text-muted);
  }
  .cf-perm-path code {
    font-family: var(--font-mono);
    font-size: 0.68rem;
    background: var(--bg-inset);
    color: var(--text);
    border: 1px solid var(--border);
    padding: 0.1rem 0.38rem;
    border-radius: 4px;
  }
  .cf-perm-foot {
    font-size: 0.69rem;
    color: var(--text-subtle);
    line-height: 1.4;
  }
  .cf-perm-foot code {
    font-family: var(--font-mono);
    color: var(--accent-strong);
  }

  .cf-toolbar-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .cf-filter-tabs {
    display: inline-flex;
    align-items: center;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 9px;
    padding: 0.22rem;
    gap: 0.25rem;
    flex-wrap: wrap;
  }
  .cf-filter-btn {
    border: 1px solid transparent;
    background: transparent;
    color: var(--text-muted);
    font-size: 0.74rem;
    font-weight: 600;
    padding: 0.34rem 0.75rem;
    border-radius: 7px;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    transition: all 0.15s ease;
  }
  .cf-filter-btn:hover {
    color: var(--text);
  }
  .cf-filter-btn.active {
    background: var(--tint-accent);
    color: var(--accent-strong);
    border-color: var(--ring-accent);
  }

  .cf-ip-badges {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .cf-ip-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.38rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.34rem 0.7rem;
    border-radius: 8px;
    font-size: 0.73rem;
    color: var(--text-muted);
  }
  .cf-ip-pill strong {
    color: var(--accent-strong);
  }
  .cf-ip-pill.subtle strong {
    color: var(--text);
  }

  .cf-table-wrap {
    border: 1px solid var(--border);
    border-radius: 10px;
    overflow-x: auto;
  }
  .cf-zones-table {
    width: 100%;
  }
  .cf-zones-table th {
    padding: 0.75rem 0.9rem;
    white-space: nowrap;
  }
  .cf-zones-table td {
    vertical-align: top;
    padding: 0.95rem 0.9rem;
  }
  .cf-col-zone {
    min-width: 185px;
    max-width: 240px;
  }
  .cf-col-domains {
    min-width: 280px;
  }
  .cf-col-actions {
    width: 220px;
  }
  .cf-zone-name-row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.4rem;
    margin-bottom: 0.38rem;
  }
  .cf-ns-line {
    font-size: 0.68rem;
    color: var(--text-subtle);
    display: flex;
    align-items: center;
    gap: 0.3rem;
    line-height: 1.4;
    word-break: break-all;
  }
  .cf-source-tag {
    display: inline-flex;
    align-items: center;
    gap: 0.22rem;
    font-size: 0.66rem;
    font-weight: 600;
    padding: 0.1rem 0.42rem;
    border-radius: 4px;
    white-space: nowrap;
  }
  .cf-source-tag.warn {
    background: var(--tint-warning);
    color: var(--warning-strong);
    border: 1px solid var(--ring-warning);
  }
  .cf-source-tag.api {
    background: var(--tint-info);
    color: var(--info);
    border: 1px solid var(--ring-info);
  }

  .cf-pointing-head {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    flex-wrap: wrap;
    margin-bottom: 0.5rem;
  }
  .cf-collapse-link {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    color: var(--text-muted);
    font-size: 0.68rem;
    font-weight: 600;
    padding: 0.18rem 0.52rem;
    border-radius: 5px;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    white-space: nowrap;
    transition: all 0.15s ease;
  }
  .cf-collapse-link:hover {
    border-color: var(--ring-info);
    color: var(--info);
    background: var(--tint-info);
  }

  .cf-matched-list {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .cf-matched-row {
    display: inline-flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.42rem;
    font-size: 0.73rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.26rem 0.6rem;
    border-radius: 6px;
    width: fit-content;
  }
  .cf-host-name {
    color: var(--text);
    font-weight: 600;
  }
  .cf-ip-arrow {
    color: var(--text-muted);
    font-size: 0.69rem;
  }
  .cf-site-tag {
    display: inline-flex;
    align-items: center;
    gap: 0.15rem;
    font-family: var(--font-sans);
    font-size: 0.64rem;
    font-weight: 600;
    padding: 0.08rem 0.4rem;
    border-radius: 4px;
  }
  .cf-site-tag.registered {
    background: var(--tint-accent);
    color: var(--accent-strong);
    border: 1px solid var(--ring-accent);
  }
  .cf-site-tag.unregistered {
    background: var(--tint-info);
    color: var(--info);
    border: 1px solid var(--ring-info);
  }

  .cf-dual-select-stack {
    display: flex;
    flex-direction: column;
    gap: 0.38rem;
  }
  .cf-mini-select-row {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }
  .cf-mini-lbl {
    font-size: 0.68rem;
    font-weight: 600;
    color: var(--text-muted);
    width: 32px;
  }
  .select-cf {
    min-width: 112px;
    width: auto;
    background: var(--bg-inset);
    color: var(--text);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.26rem 0.48rem;
    font-size: 0.72rem;
    font-weight: 600;
    cursor: pointer;
  }
  .select-cf.sec-uam {
    background: var(--tint-danger);
    border-color: var(--ring-danger);
    color: var(--danger-strong);
  }
  .select-cf:disabled {
    opacity: 0.65;
    cursor: not-allowed;
  }

  .cf-edge-pills {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0.28rem;
  }
  .cf-action-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(98px, auto));
    gap: 0.42rem;
    justify-content: end;
  }
  .cf-act-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.32rem;
    background: var(--bg-inset);
    color: var(--text);
    border: 1px solid var(--border);
    border-radius: 7px;
    padding: 0.34rem 0.58rem;
    font-size: 0.71rem;
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
    transition: all 0.15s ease;
  }
  .cf-act-btn:hover {
    border-color: var(--border-strong);
    background: var(--bg-elevated);
  }
  .cf-act-btn.primary-dns {
    background: var(--tint-accent);
    color: var(--accent-strong);
    border-color: var(--ring-accent);
  }
  .cf-act-btn.primary-dns:hover {
    background: var(--accent);
    color: var(--text-on-accent);
  }
  .cf-dns-count {
    font-family: var(--font-mono);
    font-size: 0.64rem;
    padding: 0.04rem 0.32rem;
    border-radius: 4px;
    background: var(--bg-surface);
    color: var(--accent-strong);
    border: 1px solid var(--ring-accent);
  }
  .cf-act-btn.danger-subtle:hover {
    border-color: var(--ring-danger);
    color: var(--danger-strong);
    background: var(--tint-danger);
  }

  .badge-toggle {
    display: inline-flex;
    align-items: center;
    gap: 0.22rem;
    background: var(--bg-inset);
    color: var(--text-muted);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 0.2rem 0.45rem;
    font-size: 0.68rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .badge-toggle:hover:not(:disabled) {
    border-color: var(--border-strong);
    color: var(--text);
  }
  .badge-toggle.active {
    background: var(--tint-accent);
    color: var(--accent-strong);
    border-color: var(--ring-accent);
  }
  .badge-toggle.warn.active {
    background: var(--tint-warning);
    color: var(--warning-strong);
    border-color: var(--ring-warning);
  }
  .badge-toggle:disabled {
    opacity: 0.6;
    cursor: default;
  }

  .cf-token-guide-box {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75rem 0.85rem;
  }
  .cf-guide-list {
    margin: 0;
    padding-left: 1.1rem;
    font-size: 0.73rem;
    color: var(--text-muted);
    line-height: 1.55;
  }
  .cf-guide-list code {
    color: var(--text);
    font-family: var(--font-mono);
    font-size: 0.7rem;
  }

  .bg-inset-box {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.65rem 0.85rem;
  }
  .cf-quick-sync-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.6rem;
    flex-wrap: wrap;
    background: var(--tint-info);
    border: 1px solid var(--ring-info);
    border-radius: 8px;
    padding: 0.55rem 0.75rem;
    margin-bottom: 0.75rem;
  }
  .cf-dns-form-card {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75rem 0.85rem;
    margin-bottom: 0.75rem;
  }
  .cf-dns-form-grid {
    display: grid;
    grid-template-columns: 100px 1.2fr 1.4fr 105px auto;
    gap: 0.55rem;
    align-items: center;
  }
  @media (max-width: 900px) {
    .cf-dns-form-grid {
      grid-template-columns: 1fr 1fr;
    }
  }
  .cf-content-input-group {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }
  .cf-dns-form-btns {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .cf-dns-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    flex-wrap: wrap;
    margin-bottom: 0.65rem;
  }
  .cf-dns-search-input {
    max-width: 240px;
    padding: 0.35rem 0.65rem;
  }
  .cf-dns-table-scroll {
    max-height: 360px;
    overflow-y: auto;
    border: 1px solid var(--border);
    border-radius: 8px;
  }
  .cf-server-match-cell {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    flex-wrap: wrap;
  }
  .btn-proxy-toggle {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    background: var(--bg-inset);
    color: var(--text-muted);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 0.2rem 0.5rem;
    font-size: 0.68rem;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-proxy-toggle.proxied {
    background: var(--tint-warning);
    color: var(--warning-strong);
    border-color: var(--ring-warning);
  }
  .btn-proxy-toggle:disabled {
    opacity: 0.6;
    cursor: default;
  }
</style>
