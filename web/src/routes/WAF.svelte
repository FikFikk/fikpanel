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
  import Modal from '../lib/components/ui/Modal.svelte';
  import OverviewTab from './waf/OverviewTab.svelte';
  import LiveAccessTab from './waf/LiveAccessTab.svelte';
  import SiteProtectionTab from './waf/SiteProtectionTab.svelte';
  import AttackMapTab from './waf/AttackMapTab.svelte';
  import ListsTab from './waf/ListsTab.svelte';
  import CloudflareHub from './waf/CloudflareHub.svelte';
  import '../styles/pages/waf/shared.css';

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

<div class="pg-waf waf-container">
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
  <nav class="tabs waf-tabs-nav">
    <button
      class="tabs__tab waf-tab-btn {activeTab === 'overview' ? 'active' : ''}"
      class:is-active={activeTab === 'overview'}
      onclick={() => { activeTab = 'overview'; loadOverview(); }}
    >
      <i class="ri-dashboard-2-line"></i> Overview
    </button>
    <button
      class="tabs__tab waf-tab-btn {activeTab === 'live_access' ? 'active' : ''}"
      class:is-active={activeTab === 'live_access'}
      onclick={() => { activeTab = 'live_access'; loadAccessLogs(); loadOverview(true); }}
    >
      <i class="ri-pulse-line"></i> Lalu Lintas & Akses Detail
      <span class="tab-count-pill">{accessLogs.length || overview?.recent_access_logs?.length || 0}</span>
    </button>
    <button
      class="tabs__tab waf-tab-btn {activeTab === 'website' ? 'active' : ''}"
      class:is-active={activeTab === 'website'}
      onclick={() => { activeTab = 'website'; loadSites(); }}
    >
      <i class="ri-global-line"></i> WebSite ({sites.length})
    </button>
    <button
      class="tabs__tab waf-tab-btn {activeTab === 'attack_map' ? 'active' : ''}"
      class:is-active={activeTab === 'attack_map'}
      onclick={() => { activeTab = 'attack_map'; loadEvents(); }}
    >
      <i class="ri-radar-line"></i> Attack Map
    </button>
    <button
      class="tabs__tab waf-tab-btn {activeTab === 'blacklist' ? 'active' : ''}"
      class:is-active={activeTab === 'blacklist'}
      onclick={() => { activeTab = 'blacklist'; loadRules(); }}
    >
      <i class="ri-shield-keyhole-line"></i> Black/White List
    </button>
    <button
      class="tabs__tab waf-tab-btn {activeTab === 'cloudflare' ? 'active' : ''}"
      class:is-active={activeTab === 'cloudflare'}
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
    <OverviewTab
      {overview}
      {events}
      {accessLogs}
      {overviewMapAction}
      {formatBytes}
      {formatTime}
      onOpenAttackMap={() => { activeTab = 'attack_map'; loadEvents(); }}
      onInspectAttackIP={inspectAttackIP}
      onQuickBlockIP={quickBlockIP}
      onFilterAccessByPath={(path) => {
        activeTab = 'live_access';
        accessSearchQuery = path;
        loadAccessLogs();
      }}
      onOpenLiveAccess={() => { activeTab = 'live_access'; loadAccessLogs(); }}
      onSelectAccessLog={(log) => (selectedAccessLog = log)}
    />

  <!-- TAB 1B: LIVE HTTP ACCESS & VISITOR OBSERVATORY -->
  {:else if activeTab === 'live_access'}
    <LiveAccessTab
      {overview}
      {filteredAccessLogs}
      {loadingAccessLogs}
      {availableAccessDomains}
      bind:accessSearchQuery
      bind:accessDomainFilter
      bind:accessThreatFilter
      bind:accessStatusFilter
      bind:accessMethodFilter
      {formatTime}
      {formatDateTime}
      {formatBytes}
      onLoadAccessLogs={() => loadAccessLogs()}
      onQuickWhitelistIP={quickWhitelistIP}
      onQuickBlockIP={quickBlockIP}
      onQuickBlockPath={quickBlockPath}
      onSelectAccessLog={(log) => (selectedAccessLog = log)}
    />

  <!-- TAB 2: WEBSITE PROTECTION (Screenshot 3) -->
  {:else if activeTab === 'website'}
    <SiteProtectionTab
      bind:siteSearch
      {loadingSites}
      {filteredSites}
      onLoadSites={loadSites}
      onUpdateSite={(site) => apiFetch(`/api/waf/sites/${encodeURIComponent(site.site_name)}`, { method: 'POST', body: JSON.stringify(site) })}
      onToggleSiteStatus={toggleSiteStatus}
      onOpenSiteLog={() => { activeTab = 'attack_map'; loadEvents(); }}
      onEditSite={(site) => { editingSite = { ...site }; showSiteModal = true; }}
    />

  <!-- TAB 3: ATTACK MAP (Screenshot 4) -->
  {:else if activeTab === 'attack_map'}
    <AttackMapTab
      {events}
      {filteredEvents}
      {countryStats}
      {loadingEvents}
      bind:mapSearchQuery
      bind:attackTypeFilter
      bind:countryFilter
      bind:selectedEvent
      {attackMapAction}
      {formatTime}
      onLoadEvents={() => loadEvents()}
      onFlyToServer={flyToServer}
      onResetMapView={resetMapView}
      onFlyToCountry={flyToCountry}
      onQuickBlockIP={quickBlockIP}
    />

  <!-- TAB 4: BLACK / WHITE LIST (Screenshot 5) -->
  {:else if activeTab === 'blacklist'}
    <ListsTab
      bind:activeRuleType
      bind:ruleType
      bind:ruleStartIP
      bind:ruleEndIP
      bind:rulePattern
      bind:ruleRemarks
      {submittingRule}
      {loadingRules}
      {rules}
      onLoadRules={loadRules}
      onAddRule={handleAddRule}
      onDeleteRule={deleteRule}
    />

  <!-- TAB 5: CLOUDFLARE HUB -->
  {:else if activeTab === 'cloudflare'}
    <CloudflareHub
      {cfConfig}
      {serverIP}
      {allServerIPs}
      {loadingCF}
      {cfTokenDiagnostic}
      bind:showCFPermGuide
      bind:cfZoneFilter
      {cfZones}
      {filteredCFZones}
      bind:expandedCFZones
      onLoadCloudflare={loadCloudflare}
      onOpenConnectModal={() => (showCFConnectModal = true)}
      onDisconnectCF={handleDisconnectCF}
      onOpenDNSModal={(zone, initialTab) => {
        selectedZone = zone;
        resetDNSForm();
        dnsFilterTab = initialTab;
        dnsSearchQuery = '';
        showDNSModal = true;
      }}
      onOpenEdgeModal={(zone) => {
        selectedZone = zone;
        showCFSettingsModal = true;
      }}
      onUpdateCFZoneSetting={updateCFZoneSetting}
      onToggleDevMode={toggleDevMode}
      onPointZoneHere={pointZoneHere}
      onPurgeCFCache={purgeCFCache}
    />
  {/if}

  <!-- Modal Hubungkan Cloudflare Token / Global API Key -->
  <Modal
    open={showCFConnectModal}
    title="Hubungkan Akun Cloudflare (API Token / Global API Key)"
    subtitle="Mendukung Custom API Token maupun Global API Key + Email"
    size="lg"
    onclose={() => (showCFConnectModal = false)}
  >
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

    {#snippet footer()}
      <button type="button" class="btn btn-ghost" onclick={() => showCFConnectModal = false}>Batal</button>
      <button type="button" class="btn btn-primary" onclick={handleConnectCF} disabled={cfSubmitting}>
        {cfSubmitting ? 'Memverifikasi & Menyinkronkan...' : 'Verifikasi & Hubungkan'}
      </button>
    {/snippet}
  </Modal>

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
    <Modal
      open={showDNSModal && !!selectedZone}
      title={`Manajemen DNS Cloudflare: ${selectedZone.name}`}
      subtitle={`IP Publik Server Ini: ${serverIP} · Kelola record A, AAAA, CNAME, TXT, MX & Proxy Orange Cloud`}
      size="xl"
      onclose={() => (showDNSModal = false)}
    >
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

      {#snippet footer()}
        <button type="button" class="btn btn-ghost" onclick={() => showDNSModal = false}>Tutup</button>
      {/snippet}
    </Modal>
  {/if}

  <!-- Modal Pengaturan Edge & Keamanan Cloudflare -->
  {#if showCFSettingsModal && selectedZone}
    <Modal
      open={showCFSettingsModal && !!selectedZone}
      title={`Pengaturan Edge & Keamanan Cloudflare: ${selectedZone.name}`}
      subtitle="Konfigurasi SSL/TLS, DDoS Under Attack Mode, HTTPS Redirect, dan Optimasi Kecepatan"
      size="xl"
      onclose={() => (showCFSettingsModal = false)}
    >
      <div class="stack-gap">
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

      {#snippet footer()}
        <button type="button" class="btn btn-ghost" onclick={() => showCFSettingsModal = false}>Tutup</button>
      {/snippet}
    </Modal>
  {/if}

  <!-- Modal Ubah Opsi Situs WAF -->
  {#if showSiteModal && editingSite}
    <Modal
      open={showSiteModal && !!editingSite}
      title={`Modifikasi Proteksi WAF: ${editingSite.site_name}`}
      subtitle="Konfigurasi mitigasi bot, CC flood, dan CDN"
      size="md"
      onclose={() => (showSiteModal = false)}
    >
      <div class="stack-gap">
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

      {#snippet footer()}
        <button type="button" class="btn btn-ghost" onclick={() => showSiteModal = false}>Batal</button>
        <button type="button" class="btn btn-primary" onclick={saveSiteSettings}>Simpan Konfigurasi</button>
      {/snippet}
    </Modal>
  {/if}

  <!-- Modal Forensik Paket HTTP & Akses Mendalam -->
  {#if selectedAccessLog}
    <Modal
      open={!!selectedAccessLog}
      title={`Inspeksi Forensik Paket HTTP #${selectedAccessLog.id}`}
      subtitle={`${formatDateTime(selectedAccessLog.timestamp)} · ${selectedAccessLog.is_threat ? `Ancaman Terdeteksi: ${selectedAccessLog.attack_type}` : 'Lalu Lintas Pengunjung Normal'}`}
      size="xl"
      onclose={() => (selectedAccessLog = null)}
    >
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

      {#snippet footer()}
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
      {/snippet}
    </Modal>
  {/if}
</div>
