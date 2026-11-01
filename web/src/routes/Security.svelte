<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import { copyText } from '../lib/utils/browser';
  import type {
    PortRule,
    FirewallExtendedStatus,
    SSHStats,
    SystemAccount,
    ServerSecurityOverview,
    BruteForceConfig,
    WhitelistIP,
    BlacklistIP,
    FailedLoginEvent,
    ProtectionOverview,
    HardeningStatus,
    SecurityScanReport,
    Fail2BanStatus,
  } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import SecurityTabs from './security/SecurityTabs.svelte';
  import OverviewTab from './security/OverviewTab.svelte';
  import FirewallTab from './security/FirewallTab.svelte';
  import SshTab from './security/SshTab.svelte';
  import ServerSecurityTab from './security/ServerSecurityTab.svelte';
  import BruteForceTab from './security/BruteForceTab.svelte';
  import AntiIntrusionTab from './security/AntiIntrusionTab.svelte';
  import ScanDetectionTab from './security/ScanDetectionTab.svelte';
  import '../styles/pages/security/shared.css';

  // Tab navigasi aktif
  let activeTab = $state<'overview' | 'firewall' | 'ssh' | 'server_security' | 'bruteforce' | 'anti_intrusion' | 'scan_detection'>('overview');

  // State data utama modul keamanan
  let loading = $state(true);
  let protection = $state<ProtectionOverview | null>(null);
  let fwStatus = $state<FirewallExtendedStatus | null>(null);
  let sshStats = $state<SSHStats | null>(null);
  let accounts = $state<SystemAccount[]>([]);
  let serverSec = $state<ServerSecurityOverview | null>(null);
  let bruteCfg = $state<BruteForceConfig | null>(null);
  let whitelist = $state<WhitelistIP[]>([]);
  let blacklist = $state<BlacklistIP[]>([]);
  let failedEvents = $state<FailedLoginEvent[]>([]);
  let hardening = $state<HardeningStatus | null>(null);
  let f2bStatus = $state<Fail2BanStatus | null>(null);
  let scanReport = $state<SecurityScanReport | null>(null);
  let isScanning = $state(false);

  // Subtab per modul
  let sshSubTab = $state<'basic' | 'accounts' | 'logs'>('basic');
  let bruteSubTab = $state<'config' | 'whitelist' | 'blacklist' | 'history'>('config');
  let scanCategoryFilter = $state<string>('all');

  // Filter firewall & Subtab ala AWS EC2 Security Groups
  let fwDirectionFilter = $state<'all' | 'inbound' | 'outbound'>('all');
  let fwPortSearch = $state('');
  let fwSubTab = $state<'inbound' | 'outbound' | 'settings'>('inbound');
  let isEditingInbound = $state(false);
  let draftRules = $state<PortRule[]>([]);
  let showActionsDropdown = $state(false);

  // Template preset tipe aturan port ala AWS EC2 Security Group
  const ruleTypePresets = [
    { type: 'Custom TCP', proto: 'tcp', port: '', desc: 'Custom TCP port' },
    { type: 'Custom UDP', proto: 'udp', port: '', desc: 'Custom UDP port' },
    { type: 'Custom ICMP - IPv4', proto: 'icmp', port: 'All', desc: 'Custom ICMP Ping/Echo' },
    { type: 'All TCP', proto: 'tcp', port: '0 - 65535', desc: 'All TCP ports' },
    { type: 'All UDP', proto: 'udp', port: '0 - 65535', desc: 'All UDP ports' },
    { type: 'All traffic', proto: 'All', port: 'All', desc: 'All inbound traffic' },
    { type: 'SSH', proto: 'tcp', port: '22', desc: 'SSH remote management' },
    { type: 'HTTP', proto: 'tcp', port: '80', desc: 'WebServer (HTTP)' },
    { type: 'HTTPS', proto: 'tcp', port: '443', desc: 'WebServer (HTTPS)' },
    { type: 'FikPanel Web', proto: 'tcp', port: '1405', desc: 'FikPanel Secure Management Panel' },
    { type: 'MySQL/Aurora', proto: 'tcp', port: '3306', desc: 'MySQL/MariaDB Database Service' },
    { type: 'PostgreSQL', proto: 'tcp', port: '5432', desc: 'PostgreSQL Database Service' },
    { type: 'Redis', proto: 'tcp', port: '6379', desc: 'Redis In-Memory Key-Value Store' },
    { type: 'DNS (UDP)', proto: 'udp', port: '53', desc: 'DNS Name Resolution Service' },
    { type: 'DNS (TCP)', proto: 'tcp', port: '53', desc: 'DNS Zone Transfer Service' },
    { type: 'SMTP', proto: 'tcp', port: '25', desc: 'SMTP Mail Delivery' },
    { type: 'SMTPS', proto: 'tcp', port: '465', desc: 'Secure SMTPS Email Submission' },
    { type: 'phpMyAdmin', proto: 'tcp', port: '888', desc: 'phpMyAdmin Web Interface' },
  ];

  function isPresetFixed(type?: string): boolean {
    if (!type) return false;
    return ['SSH', 'HTTP', 'HTTPS', 'MySQL/Aurora', 'PostgreSQL', 'Redis', 'DNS (UDP)', 'DNS (TCP)', 'SMTP', 'SMTPS', 'phpMyAdmin', 'All TCP', 'All UDP', 'All traffic'].includes(type);
  }

  function isPresetFixedPort(type?: string): boolean {
    if (!type) return false;
    return ['SSH', 'HTTP', 'HTTPS', 'MySQL/Aurora', 'PostgreSQL', 'Redis', 'DNS (UDP)', 'DNS (TCP)', 'SMTP', 'SMTPS', 'phpMyAdmin', 'All traffic', 'Custom ICMP - IPv4'].includes(type);
  }

  function detectTypeFromPort(port: string, proto: string): string {
    const p = (proto || '').toLowerCase();
    switch (port) {
      case '22': return 'SSH';
      case '80': return 'HTTP';
      case '443': return 'HTTPS';
      case '1405': return 'FikPanel Web';
      case '3306': return 'MySQL/Aurora';
      case '5432': return 'PostgreSQL';
      case '6379': return 'Redis';
      case '53': return p === 'udp' ? 'DNS (UDP)' : 'DNS (TCP)';
      case '25': return 'SMTP';
      case '465': return 'SMTPS';
      case '888': return 'phpMyAdmin';
      default: return p === 'udp' ? 'Custom UDP' : 'Custom TCP';
    }
  }

  function startEditInbound() {
    if (!fwStatus || !fwStatus.rules) return;
    draftRules = fwStatus.rules.map(r => ({
      ...r,
      rule_id: r.rule_id || `sgr-${Math.random().toString(16).substring(2, 10)}${Math.random().toString(16).substring(2, 11)}`,
      rule_type: r.rule_type || detectTypeFromPort(r.port, r.protocol),
      ip_version: r.ip_version || 'IPv4',
      source_ip: r.source_ip || '0.0.0.0/0',
    }));
    isEditingInbound = true;
  }

  function addDraftRule() {
    draftRules = [
      ...draftRules,
      {
        number: draftRules.length + 1,
        rule_id: '-',
        rule_type: 'Custom TCP',
        ip_version: 'IPv4',
        protocol: 'tcp',
        port: '',
        status: 'Not Listening',
        strategy: 'Allow',
        direction: 'Inbound',
        source_ip: '0.0.0.0/0',
        remarks: '',
        add_time: '--',
      },
    ];
  }

  function removeDraftRule(index: number) {
    draftRules = draftRules.filter((_, i) => i !== index);
  }

  function handleRuleTypeChange(index: number, newType: string) {
    const preset = ruleTypePresets.find(p => p.type === newType);
    if (!preset) return;
    const rule = draftRules[index];
    rule.rule_type = preset.type;
    rule.protocol = preset.proto;
    if (preset.port !== '') {
      rule.port = preset.port;
    }
    if (!rule.remarks || rule.remarks === 'Custom port rule' || rule.remarks === '') {
      rule.remarks = preset.desc;
    }
  }

  function handleSourceTypeChange(index: number, newType: string) {
    const rule = draftRules[index];
    if (newType === 'Anywhere-IPv4') {
      rule.source_ip = '0.0.0.0/0';
    } else if (newType === 'My IP') {
      rule.source_ip = protection?.client_ip ? `${protection.client_ip}/32` : '0.0.0.0/0';
    }
  }

  async function saveDraftRules() {
    try {
      for (const r of draftRules) {
        if (!r.port || r.port === '0') {
          notify('Port range tidak boleh kosong', 'warning');
          return;
        }
      }
      await apiFetch('/api/firewall/rules/batch', {
        method: 'POST',
        body: JSON.stringify({ rules: draftRules }),
      });
      notify('Aturan inbound firewall berhasil disimpan', 'success');
      isEditingInbound = false;
      await loadFirewall();
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan aturan firewall', 'error');
    }
  }

  // Form modal tambah aturan port
  let showAddPortModal = $state(false);
  let newPort = $state('');
  let newProto = $state('tcp');
  let newStrategy = $state('Allow');
  let newDirection = $state('Inbound');
  let newSourceIP = $state('All');
  let newRemarks = $state('');

  // Form modal tambah whitelist / blacklist
  let showAddWhitelistModal = $state(false);
  let newWhiteIP = $state('');
  let newWhiteRemarks = $state('');
  let showAddBlacklistModal = $state(false);
  let newBlackIP = $state('');
  let newBlackReason = $state('');
  let newBlackDuration = $state(1440);

  // Form SSH Basic
  let sshPortInput = $state(22);
  let sshPasswordAuth = $state(true);
  let sshKeyAuth = $state(false);
  let sshPermitRoot = $state(true);

  // Memuat data agregat dasbor ikhtisar keamanan
  async function loadOverview() {
    try {
      const res = await apiFetch<{ ok: boolean; overview: ProtectionOverview }>('/api/security/overview');
      if (res && res.overview) {
        protection = res.overview;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat ikhtisar perlindungan', 'error');
    }
  }

  // Memuat status lengkap firewall UFW dan aturan port
  async function loadFirewall() {
    try {
      const res = await apiFetch<{ ok: boolean; status: FirewallExtendedStatus }>('/api/firewall/extended');
      if (res && res.status) {
        fwStatus = res.status;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat status firewall', 'error');
    }
  }

  // Memuat metrik SSH dan konfigurasi aktif
  async function loadSSH() {
    try {
      const [resStats, resAccounts] = await Promise.all([
        apiFetch<{ ok: boolean; stats: SSHStats }>('/api/security/ssh/stats'),
        apiFetch<{ ok: boolean; accounts: SystemAccount[] }>('/api/security/ssh/accounts'),
      ]);
      if (resStats && resStats.stats) {
        sshStats = resStats.stats;
        sshPortInput = resStats.stats.port;
        sshPasswordAuth = resStats.stats.password_auth;
        sshKeyAuth = resStats.stats.key_auth;
        sshPermitRoot = resStats.stats.permit_root_login;
      }
      if (resAccounts && resAccounts.accounts) {
        accounts = resAccounts.accounts;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat metrik SSH', 'error');
    }
  }

  // Memuat evaluasi 10 poin keamanan server
  async function loadServerSecurity() {
    try {
      const res = await apiFetch<{ ok: boolean; security: ServerSecurityOverview }>('/api/security/server-security');
      if (res && res.security) {
        serverSec = res.security;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat audit keamanan server', 'error');
    }
  }

  // Memuat data proteksi brute force
  async function loadBruteForce() {
    try {
      const res = await apiFetch<{
        ok: boolean;
        config: BruteForceConfig;
        whitelist: WhitelistIP[];
        blacklist: BlacklistIP[];
        history: FailedLoginEvent[];
      }>('/api/security/bruteforce');
      if (res) {
        bruteCfg = res.config;
        whitelist = res.whitelist || [];
        blacklist = res.blacklist || [];
        failedEvents = res.history || [];
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat modul brute force', 'error');
    }
  }

  // Memuat modul anti-intrusi dan hardening sistem
  async function loadHardeningAndF2B() {
    try {
      const [resF2B, resHard] = await Promise.all([
        apiFetch<{ ok: boolean; fail2ban: Fail2BanStatus }>('/api/security/fail2ban'),
        apiFetch<{ ok: boolean; hardening: HardeningStatus }>('/api/security/hardening'),
      ]);
      if (resF2B && resF2B.fail2ban) {
        f2bStatus = resF2B.fail2ban;
      }
      if (resHard && resHard.hardening) {
        hardening = resHard.hardening;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat hardening dan Fail2Ban', 'error');
    }
  }

  // Memuat seluruh data saat inisialisasi awal
  async function refreshAll() {
    loading = true;
    try {
      await Promise.all([
        loadOverview(),
        loadFirewall(),
        loadSSH(),
        loadServerSecurity(),
        loadBruteForce(),
        loadHardeningAndF2B(),
      ]);
    } finally {
      loading = false;
    }
  }

  // Menjalankan pemindaian kerentanan mendalam menyeluruh
  async function runSecurityScan() {
    isScanning = true;
    try {
      const res = await apiFetch<{ ok: boolean; report: SecurityScanReport }>('/api/security/scan', {
        method: 'POST',
      });
      if (res && res.report) {
        scanReport = res.report;
        notify('Pemindaian keamanan server selesai dijalankan', 'success');
      }
    } catch (err: any) {
      notify(err.message || 'Gagal menjalankan pemindaian keamanan', 'error');
    } finally {
      isScanning = false;
    }
  }

  // Mengubah status aktif firewall UFW
  async function toggleFirewall() {
    if (!fwStatus) return;
    const action = fwStatus.enabled ? 'nonaktifkan' : 'aktifkan';
    const ok = await confirmDialog({
      title: `${action.toUpperCase()} Firewall`,
      message: `Apakah Anda yakin ingin me-${action} firewall UFW server?`,
      confirmText: action,
      tone: fwStatus.enabled ? 'danger' : 'default',
    });
    if (!ok) return;

    try {
      await apiFetch('/api/firewall/toggle', {
        method: 'POST',
        body: JSON.stringify({ enabled: !fwStatus.enabled }),
      });
      notify(`Firewall berhasil di-${action}`, 'success');
      await loadFirewall();
      await loadOverview();
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status firewall', 'error');
    }
  }

  // Mengubah status pemblokiran ping ICMP
  async function toggleICMP() {
    if (!fwStatus) return;
    const newBlockState = !fwStatus.block_icmp;
    try {
      await apiFetch('/api/firewall/icmp', {
        method: 'POST',
        body: JSON.stringify({ block: newBlockState }),
      });
      fwStatus.block_icmp = newBlockState;
      notify(newBlockState ? 'ICMP Ping berhasil diblokir' : 'ICMP Ping diizinkan kembali', 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah pengaturan ICMP', 'error');
    }
  }

  // Membersihkan berkas log situs web
  async function clearSiteLogs() {
    const ok = await confirmDialog({
      title: 'Bersihkan Log Situs',
      message: 'Kosongkan seluruh berkas log akses dan error webserver (/var/log/nginx)?',
      confirmText: 'Bersihkan',
      tone: 'default',
    });
    if (!ok) return;

    try {
      await apiFetch('/api/firewall/logs/clear', { method: 'POST' });
      notify('Berkas log web berhasil dikosongkan', 'success');
      await loadFirewall();
    } catch (err: any) {
      notify(err.message || 'Gagal membersihkan log web', 'error');
    }
  }

  // Menambah aturan port firewall baru
  async function handleAddPortRule() {
    if (!newPort.trim()) {
      notify('Nomor port wajib diisi', 'warning');
      return;
    }
    try {
      await apiFetch('/api/firewall/rules/extended', {
        method: 'POST',
        body: JSON.stringify({
          port: newPort.trim(),
          protocol: newProto,
          strategy: newStrategy,
          direction: newDirection,
          source_ip: newSourceIP.trim() || 'All',
          remarks: newRemarks.trim() || 'Custom port rule',
        }),
      });
      notify(`Aturan port ${newPort}/${newProto} berhasil disimpan`, 'success');
      showAddPortModal = false;
      newPort = '';
      newRemarks = '';
      await loadFirewall();
    } catch (err: any) {
      notify(err.message || 'Gagal menambah aturan port', 'error');
    }
  }

  // Menghapus aturan port firewall
  async function handleDeletePortRule(rule: PortRule) {
    const ok = await confirmDialog({
      title: 'Hapus Aturan Firewall',
      message: `Hapus aturan port ${rule.port}/${rule.protocol} (${rule.strategy})?`,
      confirmText: 'Hapus',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/firewall/rules/${rule.number}`, { method: 'DELETE' });
      notify('Aturan port firewall berhasil dihapus', 'success');
      await loadFirewall();
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus aturan', 'error');
    }
  }

  // Menyimpan konfigurasi dasar SSH
  async function handleSaveSSHConfig() {
    try {
      await apiFetch('/api/security/ssh', {
        method: 'POST',
        body: JSON.stringify({
          port: Number(sshPortInput),
          password_auth: sshPasswordAuth,
          permit_root_login: sshPermitRoot,
        }),
      });
      notify('Konfigurasi SSH berhasil disimpan dan di-reload', 'success');
      await loadSSH();
      await loadOverview();
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan konfigurasi SSH', 'error');
    }
  }

  // Memulai ulang layanan daemon SSH
  async function handleRestartSSH() {
    const ok = await confirmDialog({
      title: 'Restart Layanan SSH',
      message: 'Apakah Anda yakin ingin memulai ulang daemon SSH? Sesi aktif tidak akan terputus.',
      confirmText: 'Restart',
      tone: 'default',
    });
    if (!ok) return;

    try {
      await apiFetch('/api/security/ssh/restart', { method: 'POST' });
      notify('Layanan daemon SSH berhasil direstart', 'success');
      await loadSSH();
    } catch (err: any) {
      notify(err.message || 'Gagal merestart layanan SSH', 'error');
    }
  }

  // Mengubah status kunci akun sistem pengguna Linux
  async function handleToggleAccountLock(acc: SystemAccount) {
    const action = acc.locked ? 'Buka Kunci' : 'Kunci Akun';
    const ok = await confirmDialog({
      title: `${action} Pengguna ${acc.username}`,
      message: `Apakah Anda yakin ingin me-${action.toLowerCase()} akun ${acc.username}?`,
      confirmText: action,
      tone: acc.locked ? 'default' : 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/security/ssh/accounts/${acc.username}/toggle`, {
        method: 'POST',
        body: JSON.stringify({ lock: !acc.locked }),
      });
      notify(`Status akun ${acc.username} berhasil diubah`, 'success');
      await loadSSH();
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status akun', 'error');
    }
  }

  // Menerapkan perbaikan otomatis audit keamanan server
  async function handleApplySecurityFix(actionKey: string) {
    try {
      await apiFetch('/api/security/server-security/fix', {
        method: 'POST',
        body: JSON.stringify({ action_key: actionKey }),
      });
      notify('Perbaikan audit keamanan berhasil diterapkan', 'success');
      await loadServerSecurity();
      await loadOverview();
    } catch (err: any) {
      notify(err.message || 'Gagal menerapkan perbaikan', 'error');
    }
  }

  // Menyimpan konfigurasi brute force
  async function handleSaveBruteConfig() {
    if (!bruteCfg) return;
    try {
      await apiFetch('/api/security/bruteforce/config', {
        method: 'POST',
        body: JSON.stringify(bruteCfg),
      });
      notify('Konfigurasi proteksi brute force berhasil disimpan', 'success');
      await loadBruteForce();
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan konfigurasi brute force', 'error');
    }
  }

  // Menambahkan IP klien saat ini langsung ke whitelist
  async function handleWhitelistCurrentIP() {
    if (!protection?.client_ip) return;
    try {
      await apiFetch('/api/security/bruteforce/whitelist', {
        method: 'POST',
        body: JSON.stringify({
          ip: protection.client_ip,
          remarks: 'Alamat IP Pengelola Saat Ini',
        }),
      });
      notify(`IP ${protection.client_ip} berhasil ditambahkan ke whitelist`, 'success');
      await loadBruteForce();
      await loadOverview();
    } catch (err: any) {
      notify(err.message || 'Gagal menambahkan IP ke whitelist', 'error');
    }
  }

  // Menambahkan IP baru ke whitelist via form modal
  async function handleAddWhitelist() {
    if (!newWhiteIP.trim()) {
      notify('Alamat IP wajib diisi', 'warning');
      return;
    }
    try {
      await apiFetch('/api/security/bruteforce/whitelist', {
        method: 'POST',
        body: JSON.stringify({
          ip: newWhiteIP.trim(),
          remarks: newWhiteRemarks.trim() || 'Whitelist Operator',
        }),
      });
      notify('Alamat IP berhasil ditambahkan ke whitelist', 'success');
      showAddWhitelistModal = false;
      newWhiteIP = '';
      newWhiteRemarks = '';
      await loadBruteForce();
    } catch (err: any) {
      notify(err.message || 'Gagal menambahkan IP ke whitelist', 'error');
    }
  }

  // Menghapus IP dari whitelist
  async function handleRemoveWhitelist(ip: string) {
    try {
      await apiFetch(`/api/security/bruteforce/whitelist/${ip}`, { method: 'DELETE' });
      notify('Alamat IP berhasil dihapus dari whitelist', 'success');
      await loadBruteForce();
      await loadOverview();
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus IP whitelist', 'error');
    }
  }

  // Menambahkan IP ke blacklist
  async function handleAddBlacklist() {
    if (!newBlackIP.trim()) {
      notify('Alamat IP wajib diisi', 'warning');
      return;
    }
    try {
      await apiFetch('/api/security/bruteforce/blacklist', {
        method: 'POST',
        body: JSON.stringify({
          ip: newBlackIP.trim(),
          reason: newBlackReason.trim() || 'Manual block by admin',
          duration_minutes: Number(newBlackDuration) || 1440,
        }),
      });
      notify('Alamat IP berhasil diblokir', 'success');
      showAddBlacklistModal = false;
      newBlackIP = '';
      newBlackReason = '';
      await loadBruteForce();
    } catch (err: any) {
      notify(err.message || 'Gagal menambahkan IP ke blacklist', 'error');
    }
  }

  // Membuka blokir IP dari blacklist
  async function handleRemoveBlacklist(ip: string) {
    try {
      await apiFetch(`/api/security/bruteforce/blacklist/${ip}`, { method: 'DELETE' });
      notify('Pemblokiran IP berhasil dicabut', 'success');
      await loadBruteForce();
    } catch (err: any) {
      notify(err.message || 'Gagal mencabut pemblokiran IP', 'error');
    }
  }

  // Mengubah pembatasan eksekusi compiler sistem
  async function handleToggleCompilerAccess() {
    if (!hardening) return;
    const newBlock = !hardening.compiler_blocked;
    try {
      await apiFetch('/api/security/hardening/compiler', {
        method: 'POST',
        body: JSON.stringify({ block: newBlock }),
      });
      hardening.compiler_blocked = newBlock;
      notify(newBlock ? 'Akses compiler berhasil dibatasi ke root' : 'Akses compiler dibuka kembali', 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah perizinan compiler', 'error');
    }
  }

  // Membuka blokir IP dari jail Fail2Ban
  async function handleUnbanF2B(jail: string, ip: string) {
    try {
      await apiFetch('/api/security/fail2ban/unban', {
        method: 'POST',
        body: JSON.stringify({ jail, ip }),
      });
      notify(`IP ${ip} berhasil di-unban dari jail ${jail}`, 'success');
      await loadHardeningAndF2B();
    } catch (err: any) {
      notify(err.message || 'Gagal melakukan unban IP', 'error');
    }
  }

  // Filter aturan port firewall
  let filteredPortRules = $derived.by(() => {
    if (!fwStatus || !fwStatus.rules) return [];
    return fwStatus.rules.filter((r) => {
      if (fwDirectionFilter === 'inbound' && r.direction.toLowerCase() !== 'inbound') return false;
      if (fwDirectionFilter === 'outbound' && r.direction.toLowerCase() !== 'outbound') return false;
      if (fwPortSearch.trim()) {
        const q = fwPortSearch.trim().toLowerCase();
        const matchPort = r.port.toLowerCase().includes(q);
        const matchType = (r.rule_type || '').toLowerCase().includes(q);
        const matchID = (r.rule_id || '').toLowerCase().includes(q);
        const matchDesc = (r.remarks || '').toLowerCase().includes(q);
        if (!matchPort && !matchType && !matchID && !matchDesc) return false;
      }
      return true;
    });
  });

  // Filter hasil pemindaian keamanan
  let filteredScanFindings = $derived.by(() => {
    if (!scanReport || !scanReport.findings) return [];
    if (scanCategoryFilter === 'all') return scanReport.findings;
    return scanReport.findings.filter((f) => f.category === scanCategoryFilter);
  });

  onMount(() => {
    refreshAll();
  });
</script>

<div class="pg-security sec-page">
  <!-- Tab Navigasi Atas (7 Modul Keamanan Tingkat Enterprise) -->
  <SecurityTabs {activeTab} onSelect={(t) => (activeTab = t)} />

  <!-- TAB 1: OVERVIEW -->
  {#if activeTab === 'overview'}
    <OverviewTab
      {isScanning}
      {scanReport}
      {protection}
      {fwStatus}
      {sshStats}
      {hardening}
      {f2bStatus}
      onRefresh={refreshAll}
      onRunScan={runSecurityScan}
      onToggleFirewall={toggleFirewall}
      onRestartSSH={handleRestartSSH}
      onToggleTamperProof={async () => {
        if (!hardening) return;
        await apiFetch('/api/security/hardening/tamper', {
          method: 'POST',
          body: JSON.stringify({ enable: !hardening.tamper_proof_enabled }),
        });
        await loadHardeningAndF2B();
      }}
      onSelectTab={(t) => (activeTab = t)}
      onOpenScanCategory={(cat) => {
        activeTab = 'scan_detection';
        scanCategoryFilter = cat;
      }}
    />
  {/if}

  <!-- TAB 2: FIREWALL (AWS EC2 Security Group Style) -->
  {#if activeTab === 'firewall'}
    <FirewallTab
      {fwStatus}
      {fwSubTab}
      bind:fwPortSearch
      {isEditingInbound}
      {showActionsDropdown}
      {filteredPortRules}
      bind:draftRules
      {ruleTypePresets}
      {detectTypeFromPort}
      {isPresetFixed}
      {isPresetFixedPort}
      onSelectSubTab={(t) => (fwSubTab = t)}
      onToggleActionsDropdown={() => (showActionsDropdown = !showActionsDropdown)}
      onCloseActionsDropdown={() => (showActionsDropdown = false)}
      onStartEditInbound={startEditInbound}
      onCancelEditInbound={() => (isEditingInbound = false)}
      onToggleFirewall={toggleFirewall}
      onToggleICMP={toggleICMP}
      onClearSiteLogs={clearSiteLogs}
      onLoadFirewall={loadFirewall}
      onDeletePortRule={handleDeletePortRule}
      onAddDraftRule={addDraftRule}
      onRemoveDraftRule={removeDraftRule}
      onRuleTypeChange={handleRuleTypeChange}
      onSourceTypeChange={handleSourceTypeChange}
      onSaveDraftRules={saveDraftRules}
    />
  {/if}

  <!-- TAB 3: SSH -->
  {#if activeTab === 'ssh'}
    <SshTab
      {sshStats}
      {accounts}
      {sshSubTab}
      bind:sshPortInput
      bind:sshPasswordAuth
      bind:sshKeyAuth
      bind:sshPermitRoot
      onSelectSubTab={(t) => (sshSubTab = t)}
      onRestartSSH={handleRestartSSH}
      onSaveSSHConfig={handleSaveSSHConfig}
      onToggleAccountLock={handleToggleAccountLock}
    />
  {/if}

  <!-- TAB 4: SERVER SECURITY (10 Poin Evaluasi & Skor) -->
  {#if activeTab === 'server_security'}
    <ServerSecurityTab {serverSec} onApplySecurityFix={handleApplySecurityFix} />
  {/if}

  <!-- TAB 5: BRUTE FORCE PROTECTION -->
  {#if activeTab === 'bruteforce'}
    <BruteForceTab
      {protection}
      bind:bruteCfg
      {bruteSubTab}
      {whitelist}
      {blacklist}
      {failedEvents}
      onSelectSubTab={(t) => (bruteSubTab = t)}
      onWhitelistCurrentIP={handleWhitelistCurrentIP}
      onSaveBruteConfig={handleSaveBruteConfig}
      onOpenAddWhitelist={() => (showAddWhitelistModal = true)}
      onRemoveWhitelist={handleRemoveWhitelist}
      onOpenAddBlacklist={() => (showAddBlacklistModal = true)}
      onRemoveBlacklist={handleRemoveBlacklist}
    />
  {/if}

  <!-- TAB 6: ANTI INTRUSION & HARDENING -->
  {#if activeTab === 'anti_intrusion'}
    <AntiIntrusionTab
      {f2bStatus}
      {hardening}
      {fwStatus}
      onUnbanF2B={handleUnbanF2B}
      onToggleCompilerAccess={handleToggleCompilerAccess}
      onToggleICMP={toggleICMP}
    />
  {/if}

  <!-- TAB 7: SCAN DETECTION -->
  {#if activeTab === 'scan_detection'}
    <ScanDetectionTab
      {isScanning}
      {scanReport}
      {scanCategoryFilter}
      {filteredScanFindings}
      onRunScan={runSecurityScan}
      onSelectCategory={(cat) => (scanCategoryFilter = cat)}
    />
  {/if}

  <!-- MODAL: ADD PORT RULE -->
  <Modal
    open={showAddPortModal}
    title="Tambah Aturan Port Firewall"
    size="sm"
    onclose={() => (showAddPortModal = false)}
  >
    <div class="field mb-3">
      <label class="field__label" for="rule-port">Nomor Port / Range Port</label>
      <input
        id="rule-port"
        type="text"
        class="input input--mono"
        placeholder="Contoh: 8080 atau 3000-4000"
        bind:value={newPort}
      />
    </div>

    <div class="field mb-3">
      <label class="field__label" for="rule-proto">Protokol</label>
      <select id="rule-proto" class="input" bind:value={newProto}>
        <option value="tcp">TCP</option>
        <option value="udp">UDP</option>
        <option value="any">TCP/UDP (ANY)</option>
      </select>
    </div>

    <div class="field mb-3">
      <label class="field__label" for="rule-strategy">Strategi Aksi</label>
      <select id="rule-strategy" class="input" bind:value={newStrategy}>
        <option value="Allow">ALLOW (Izinkan)</option>
        <option value="Deny">DENY (Tolak / Blokir)</option>
      </select>
    </div>

    <div class="field mb-3">
      <label class="field__label" for="rule-direction">Arah Paket (Direction)</label>
      <select id="rule-direction" class="input" bind:value={newDirection}>
        <option value="Inbound">Inbound (Masuk)</option>
        <option value="Outbound">Outbound (Keluar)</option>
      </select>
    </div>

    <div class="field mb-3">
      <label class="field__label" for="rule-source">Sumber IP</label>
      <input
        id="rule-source"
        type="text"
        class="input input--mono"
        placeholder="All atau misal 192.168.1.0/24"
        bind:value={newSourceIP}
      />
    </div>

    <div class="field mb-3">
      <label class="field__label" for="rule-remarks">Keterangan / Catatan</label>
      <input
        id="rule-remarks"
        type="text"
        class="input"
        placeholder="Contoh: Port API Backend Go"
        bind:value={newRemarks}
      />
    </div>

    {#snippet footer()}
      <button class="btn btn--secondary" onclick={() => (showAddPortModal = false)}>Batal</button>
      <button class="btn btn--primary" onclick={handleAddPortRule}>Terapkan Aturan</button>
    {/snippet}
  </Modal>

  <!-- MODAL: ADD WHITELIST IP -->
  <Modal
    open={showAddWhitelistModal}
    title="Tambah IP ke Whitelist"
    size="sm"
    onclose={() => (showAddWhitelistModal = false)}
  >
    <div class="field mb-3">
      <label class="field__label" for="white-ip">Alamat IP / Subnet</label>
      <input
        id="white-ip"
        type="text"
        class="input input--mono"
        placeholder="Contoh: 202.59.205.41"
        bind:value={newWhiteIP}
      />
    </div>
    <div class="field mb-3">
      <label class="field__label" for="white-remarks">Catatan Keterangan</label>
      <input
        id="white-remarks"
        type="text"
        class="input"
        placeholder="Contoh: Kantor Utama Fikri"
        bind:value={newWhiteRemarks}
      />
    </div>

    {#snippet footer()}
      <button class="btn btn--secondary" onclick={() => (showAddWhitelistModal = false)}>Batal</button>
      <button class="btn btn--primary" onclick={handleAddWhitelist}>Simpan Whitelist</button>
    {/snippet}
  </Modal>

  <!-- MODAL: ADD BLACKLIST IP -->
  <Modal
    open={showAddBlacklistModal}
    title="Blokir Alamat IP (Blacklist)"
    size="sm"
    onclose={() => (showAddBlacklistModal = false)}
  >
    <div class="field mb-3">
      <label class="field__label" for="black-ip">Alamat IP</label>
      <input
        id="black-ip"
        type="text"
        class="input input--mono"
        placeholder="Contoh: 185.220.101.44"
        bind:value={newBlackIP}
      />
    </div>
    <div class="field mb-3">
      <label class="field__label" for="black-reason">Alasan Pemblokiran</label>
      <input
        id="black-reason"
        type="text"
        class="input"
        placeholder="Contoh: Serangan brute force berulang"
        bind:value={newBlackReason}
      />
    </div>
    <div class="field mb-3">
      <label class="field__label" for="black-dur">Durasi Blokir (Menit)</label>
      <input
        id="black-dur"
        type="number"
        class="input input--mono"
        bind:value={newBlackDuration}
      />
    </div>

    {#snippet footer()}
      <button class="btn btn--secondary" onclick={() => (showAddBlacklistModal = false)}>Batal</button>
      <button class="btn btn--danger" onclick={handleAddBlacklist}>Blokir IP</button>
    {/snippet}
  </Modal>
</div>

