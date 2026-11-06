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
    WAFEnforcementStatus,
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
  import CfConnectModal from './waf/CfConnectModal.svelte';
  import CfDnsModal from './waf/CfDnsModal.svelte';
  import CfEdgeModal from './waf/CfEdgeModal.svelte';
  import SiteOptionsModal from './waf/SiteOptionsModal.svelte';
  import ForensicsModal from './waf/ForensicsModal.svelte';
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
  let showEnforcementModal = $state(false);
  let enforcementStatus = $state<WAFEnforcementStatus | null>(null);
  let syncingWAF = $state(false);
  let togglingGlobalWAF = $state(false);
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

  // Toggle saklar Global WAF (Aktif / Nonaktif) secara dinamis
  async function toggleGlobalWAF() {
    const current = overview?.waf_global_enabled ?? true;
    const nextState = !current;
    try {
      togglingGlobalWAF = true;
      const res = await apiFetch<{
        ok: boolean;
        waf_global_enabled: boolean;
        enforcement?: WAFEnforcementStatus;
        message?: string;
      }>('/api/waf/global', {
        method: 'POST',
        body: JSON.stringify({ enabled: nextState }),
      });
      if (overview) {
        overview.waf_global_enabled = res?.waf_global_enabled ?? nextState;
        if (res?.enforcement) overview.enforcement = res.enforcement;
      }
      if (res?.enforcement) enforcementStatus = res.enforcement;
      notify(res?.message || `Mesin Global WAF ${nextState ? 'diaktifkan' : 'dinonaktifkan'}`, 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status Global WAF', 'error');
    } finally {
      togglingGlobalWAF = false;
    }
  }

  // Buka modal transparansi konfigurasi Nginx WAF
  async function openEnforcementInspector() {
    showEnforcementModal = true;
    try {
      const res = await apiFetch<{ ok: boolean; enforcement: WAFEnforcementStatus }>('/api/waf/enforcement');
      if (res?.enforcement) {
        enforcementStatus = res.enforcement;
        if (overview) overview.enforcement = res.enforcement;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat status konfigurasi WAF Nginx', 'error');
    }
  }

  // Paksa sinkronisasi ulang aturan WAF ke berkas konfigurasi Nginx
  async function syncWAFNginxNow() {
    try {
      syncingWAF = true;
      const res = await apiFetch<{ ok: boolean; enforcement: WAFEnforcementStatus; message?: string }>('/api/waf/sync', {
        method: 'POST',
      });
      if (res?.enforcement) {
        enforcementStatus = res.enforcement;
        if (overview) overview.enforcement = res.enforcement;
      }
      notify(res?.message || 'Konfigurasi WAF berhasil disinkronkan ke Nginx', 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal menyinkronkan konfigurasi WAF', 'error');
    } finally {
      syncingWAF = false;
    }
  }

  // Bersihkan seluruh riwayat insiden serangan WAF
  async function handleClearEvents() {
    const ok = await confirmDialog({
      title: 'Bersihkan Riwayat Insiden WAF',
      message: 'Seluruh rekaman insiden serangan pada Attack Map akan dihapus. Lanjutkan?',
      confirmLabel: 'Bersihkan Semua',
      variant: 'danger',
    });
    if (!ok) return;
    try {
      await apiFetch('/api/waf/events', { method: 'DELETE' });
      events = [];
      selectedEvent = null;
      await loadOverview(true);
      await loadSites();
      refreshAttackMapFn?.();
      notify('Seluruh riwayat insiden WAF berhasil dibersihkan', 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal membersihkan riwayat insiden WAF', 'error');
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

  // Koordinat & identitas server host Origin secara dinamis dari backend
  let hostServerLocation = $derived({
    lat: overview?.host_location?.latitude ?? -6.2088,
    lng: overview?.host_location?.longitude ?? 106.8456,
    ip: overview?.host_location?.ip || serverIP || '127.0.0.1',
    name: overview?.host_location?.label || 'FikPanel Origin Host',
  });
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
        .bindTooltip(hostServerLocation.name, { direction: 'top', className: 'dark-map-tooltip' })
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
        .bindTooltip(`${hostServerLocation.name} (IP ${hostServerLocation.ip})`, { direction: 'top', className: 'dark-map-tooltip' })
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
      <button
        type="button"
        class="badge-status-glow"
        onclick={toggleGlobalWAF}
        disabled={togglingGlobalWAF}
        title="Klik untuk mengaktifkan / menonaktifkan Global WAF"
      >
        <span class="dot-glow"></span>
        {overview?.waf_global_enabled === false ? 'Global WAF Nonaktif (Klik Aktifkan)' : 'Proteksi Mesin Aktif'}
      </button>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        onclick={openEnforcementInspector}
        title="Lihat konfigurasi Nginx WAF yang diterapkan secara langsung"
      >
        <i class="ri-file-code-line"></i> Aturan Nginx
      </button>
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
      onOpenEnforcementModal={openEnforcementInspector}
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
      onOpenSiteLog={(domain) => { activeTab = 'live_access'; accessDomainFilter = domain; loadAccessLogs(); }}
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
      onClearEvents={handleClearEvents}
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
  <CfConnectModal
    bind:showCFConnectModal
    bind:cfTokenInput
    bind:cfEmailInput
    {cfSubmitting}
    onConnectCF={handleConnectCF}
  />

  <!-- Modal Manajemen DNS Records (Full CRUD + 1-Click Local Site Sync) -->
  <CfDnsModal
    bind:showDNSModal
    {selectedZone}
    {serverIP}
    {dnsEditId}
    bind:dnsType
    bind:dnsName
    bind:dnsContent
    bind:dnsProxied
    {savingDNS}
    bind:dnsFilterTab
    bind:dnsSearchQuery
    onPointZoneHere={pointZoneHere}
    onSaveCFDNSRecord={saveCFDNSRecord}
    onResetDNSForm={resetDNSForm}
    onToggleCFDNSProxy={toggleCFDNSProxy}
    onStartEditDNSRecord={startEditDNSRecord}
    onDeleteCFDNSRecord={deleteCFDNSRecord}
  />

  <!-- Modal Pengaturan Edge & Keamanan Cloudflare -->
  <CfEdgeModal
    bind:showCFSettingsModal
    {selectedZone}
    onUpdateCFZoneSetting={updateCFZoneSetting}
    onToggleDevMode={toggleDevMode}
    onPurgeCFCache={purgeCFCache}
  />

  <!-- Modal Ubah Opsi Situs WAF -->
  <SiteOptionsModal
    bind:showSiteModal
    bind:editingSite
    onSaveSiteSettings={saveSiteSettings}
  />

  <!-- Modal Forensik Paket HTTP & Akses Mendalam -->
  <ForensicsModal
    bind:selectedAccessLog
    {formatDateTime}
    {formatBytes}
    onQuickWhitelistIP={quickWhitelistIP}
    onQuickBlockPath={quickBlockPath}
    onQuickBlockIP={quickBlockIP}
  />

  <!-- Modal Transparansi & Konfigurasi Aktif Nginx WAF -->
  {#if showEnforcementModal}
    {@const enf = enforcementStatus || overview?.enforcement}
    <Modal
      open={showEnforcementModal}
      title="Transparansi Penegakan Aturan WAF Nginx"
      subtitle={enf?.config_path ? `Berkas Aktif: ${enf.config_path}` : 'Sinkronisasi langsung aturan Black/White List & Signature OWASP ke Nginx'}
      size="xl"
      onclose={() => (showEnforcementModal = false)}
    >
      <div class="stack-gap">
        <div class="drawer-info-grid">
          <div class="info-row">
            <span class="info-lbl">Status Mesin Global WAF:</span>
            <span class="info-val font-bold {overview?.waf_global_enabled === false ? 'text-danger' : 'text-success'}">
              {overview?.waf_global_enabled === false ? 'DINONAKTIFKAN (Pass-Through)' : 'AKTIF (Enforcing)'}
            </span>
          </div>
          <div class="info-row">
            <span class="info-lbl">Lokasi Origin Server:</span>
            <span class="info-val mono">{hostServerLocation.name}</span>
          </div>
          <div class="info-row">
            <span class="info-lbl">Aturan Aktif & Situs Terproteksi:</span>
            <span class="info-val mono">
              {enf?.active_rules_count ?? overview?.total_rules_count ?? 0} Aturan · {enf?.protected_sites_count ?? sites.length} Situs Aktif
            </span>
          </div>
          <div class="info-row">
            <span class="info-lbl">Sinkronisasi Terakhir:</span>
            <span class="info-val mono text-xs">{enf?.last_synced_at ? formatDateTime(enf.last_synced_at) : '-'}</span>
          </div>
        </div>
        {#if enf?.last_sync_error}
          <div class="rule-box text-xs text-danger">{enf.last_sync_error}</div>
        {/if}
        <div class="info-col mt-3">
          <span class="info-lbl">Isi Konfigurasi Nginx WAF (`waf-rules.conf`):</span>
          <pre class="payload-box waf-config-preview mono text-xs">{enf?.config_preview || '# Memuat konfigurasi...'}</pre>
        </div>
      </div>
      {#snippet footer()}
        <button type="button" class="btn btn-secondary btn-sm" onclick={toggleGlobalWAF} disabled={togglingGlobalWAF}>
          <i class="ri-shut-down-line"></i> {overview?.waf_global_enabled === false ? 'Aktifkan Global WAF' : 'Matikan Global WAF'}
        </button>
        <button type="button" class="btn btn-primary btn-sm" onclick={syncWAFNginxNow} disabled={syncingWAF}>
          <i class="ri-refresh-line {syncingWAF ? 'spin' : ''}"></i> {syncingWAF ? 'Menyinkronkan...' : 'Sinkronkan Ulang ke Nginx'}
        </button>
        <button type="button" class="btn btn-ghost btn-sm" onclick={() => (showEnforcementModal = false)}>Tutup</button>
      {/snippet}
    </Modal>
  {/if}
</div>
