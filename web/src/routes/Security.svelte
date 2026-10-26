<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
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
      confirmLabel: action,
      variant: fwStatus.enabled ? 'danger' : 'primary',
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
      confirmLabel: 'Bersihkan',
      variant: 'primary',
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
      confirmLabel: 'Hapus',
      variant: 'danger',
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
      confirmLabel: 'Restart',
      variant: 'primary',
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
      confirmLabel: action,
      variant: acc.locked ? 'primary' : 'danger',
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

<div class="sec-page">
  <!-- Top Navigation Tabs (10 Modul Keamanan Tingkat Enterprise) -->
  <nav class="sec-nav-bar" aria-label="Tab Navigasi Keamanan">
    <div class="sec-tabs">
      <button
        class="sec-tab-btn"
        class:active={activeTab === 'overview'}
        onclick={() => (activeTab = 'overview')}
      >
        Overview
      </button>
      <button
        class="sec-tab-btn"
        class:active={activeTab === 'firewall'}
        onclick={() => (activeTab = 'firewall')}
      >
        Firewall
      </button>
      <button
        class="sec-tab-btn"
        class:active={activeTab === 'ssh'}
        onclick={() => (activeTab = 'ssh')}
      >
        SSH
      </button>
      <button
        class="sec-tab-btn"
        class:active={activeTab === 'server_security'}
        onclick={() => (activeTab = 'server_security')}
      >
        Server security
      </button>
      <button
        class="sec-tab-btn"
        class:active={activeTab === 'bruteforce'}
        onclick={() => (activeTab = 'bruteforce')}
      >
        Brute force protection
      </button>
      <button
        class="sec-tab-btn"
        class:active={activeTab === 'anti_intrusion'}
        onclick={() => (activeTab = 'anti_intrusion')}
      >
        Anti Intrusion & Hardening
      </button>
      <button
        class="sec-tab-btn"
        class:active={activeTab === 'scan_detection'}
        onclick={() => (activeTab = 'scan_detection')}
      >
        Scan Detection
      </button>
    </div>
  </nav>

  <!-- TAB 1: OVERVIEW -->
  {#if activeTab === 'overview'}
    <div class="tab-pane">
      <header class="pane-header">
        <div>
          <h2 class="pane-title">Security Overview</h2>
          <p class="pane-subtitle">Monitor your server security posture and respond to threats before they impact your services.</p>
        </div>
        <div class="header-right">
          <span class="badge badge--success badge--pill">
            <span class="dot dot--success dot--pulse"></span> System running
          </span>
          <button class="btn btn--secondary btn--sm" onclick={refreshAll} aria-label="Refresh Data Keamanan">
            <i class="ri-refresh-line"></i> Refresh
          </button>
        </div>
      </header>

      <!-- Grid Atas: Status Pemindaian & Status Proteksi 6 Modul -->
      <div class="overview-top-grid">
        <!-- Kartu Kiri: Indikator Pemindaian Keamanan -->
        <div class="card scan-posture-card">
          <div class="radar-visual">
            <div class="radar-circle" class:spinning={isScanning}>
              <i class="ri-radar-line radar-icon"></i>
            </div>
          </div>
          <div class="scan-posture-content">
            {#if scanReport}
              <div class="scan-score-badge">
                <span class="score-number">{scanReport.score}</span>
                <span class="score-total">/ 100</span>
              </div>
              <h3 class="posture-title">Skor Keamanan Sistem</h3>
              <p class="posture-desc">
                Terakhir dipindai: {new Date(scanReport.scanned_at * 1000).toLocaleString('id-ID')}
              </p>
            {:else}
              <h3 class="posture-title">Not scanned</h3>
              <p class="posture-desc">No security scan has been run yet. Click Start Scan to begin.</p>
            {/if}
            <button
              class="btn btn--primary"
              disabled={isScanning}
              onclick={runSecurityScan}
            >
              <i class="ri-play-circle-line"></i>
              {isScanning ? 'Memindai Sistem...' : scanReport ? 'Rescan System' : 'Start Scan'}
            </button>
          </div>
        </div>

        <!-- Kartu Kanan: Status Proteksi (Enabled X/6) -->
        <div class="card protection-status-card">
          <div class="card__head">
            <span class="card__title">Protection Status</span>
            <span class="badge badge--success badge--pill">
              Enabled {protection ? protection.enabled_count : 2}/6
            </span>
          </div>
          <div class="card__body">
            <div class="protection-list">
              <!-- Firewall Guard -->
              <div class="prot-row">
                <div class="prot-info">
                  <div class="prot-title-wrap">
                    <span class="dot dot--success"></span>
                    <span class="prot-name">Firewall</span>
                  </div>
                  <p class="prot-sub">{protection?.firewall.summary || 'Allowed ports: 22, 80, 443, 3306, 24441'}</p>
                </div>
                <div class="prot-actions">
                  <label class="switch" aria-label="Toggle Firewall">
                    <input
                      type="checkbox"
                      role="switch"
                      checked={fwStatus ? fwStatus.enabled : false}
                      onchange={toggleFirewall}
                    />
                    <span class="switch__track"></span>
                  </label>
                  <button class="btn btn--ghost btn--icon" title="Pengaturan Firewall" onclick={() => (activeTab = 'firewall')}>
                    <i class="ri-settings-3-line"></i>
                  </button>
                </div>
              </div>

              <!-- SSH Protection -->
              <div class="prot-row">
                <div class="prot-info">
                  <div class="prot-title-wrap">
                    <span class="dot dot--success"></span>
                    <span class="prot-name">SSH Protection</span>
                  </div>
                  <p class="prot-sub">{protection?.ssh_protection.summary || 'Current port: 22 · Key login: Disabled · Password login: Enabled'}</p>
                </div>
                <div class="prot-actions">
                  <label class="switch" aria-label="Toggle SSH Protection">
                    <input
                      type="checkbox"
                      role="switch"
                      checked={sshStats ? sshStats.service_active : true}
                      onchange={handleRestartSSH}
                    />
                    <span class="switch__track"></span>
                  </label>
                  <button class="btn btn--ghost btn--icon" title="Pengaturan SSH" onclick={() => (activeTab = 'ssh')}>
                    <i class="ri-settings-3-line"></i>
                  </button>
                </div>
              </div>

              <!-- Nginx WAF -->
              <div class="prot-row">
                <div class="prot-info">
                  <div class="prot-title-wrap">
                    <span class="dot dot--success"></span>
                    <span class="prot-name">Nginx WAF</span>
                  </div>
                  <p class="prot-sub">Layer 7 HTTP Web Application Firewall Guard</p>
                </div>
                <div class="prot-actions">
                  <label class="switch" aria-label="Toggle Nginx WAF">
                    <input type="checkbox" role="switch" checked={true} />
                    <span class="switch__track"></span>
                  </label>
                  <button class="btn btn--ghost btn--icon" title="Pengaturan WAF" onclick={() => (activeTab = 'anti_intrusion')}>
                    <i class="ri-settings-3-line"></i>
                  </button>
                </div>
              </div>

              <!-- Tamper-proof for Enterprise -->
              <div class="prot-row">
                <div class="prot-info">
                  <div class="prot-title-wrap">
                    <span class="dot" class:dot--success={hardening?.tamper_proof_enabled}></span>
                    <span class="prot-name">Tamper-proof for Enterprise</span>
                  </div>
                  <p class="prot-sub">Integritas berkas webroot dan pencegahan injeksi backdoor</p>
                </div>
                <div class="prot-actions">
                  <label class="switch" aria-label="Toggle Anti-Tamper">
                    <input
                      type="checkbox"
                      role="switch"
                      checked={hardening ? hardening.tamper_proof_enabled : false}
                      onchange={async () => {
                        if (!hardening) return;
                        await apiFetch('/api/security/hardening/tamper', {
                          method: 'POST',
                          body: JSON.stringify({ enable: !hardening.tamper_proof_enabled }),
                        });
                        await loadHardeningAndF2B();
                      }}
                    />
                    <span class="switch__track"></span>
                  </label>
                  <button class="btn btn--ghost btn--icon" title="Pengaturan Hardening" onclick={() => (activeTab = 'anti_intrusion')}>
                    <i class="ri-settings-3-line"></i>
                  </button>
                </div>
              </div>

              <!-- Intrusion Detection -->
              <div class="prot-row">
                <div class="prot-info">
                  <div class="prot-title-wrap">
                    <span class="dot dot--success"></span>
                    <span class="prot-name">Intrusion Detection</span>
                  </div>
                  <p class="prot-sub">Deteksi pemindaian port dan anomali koneksi waktu-nyata</p>
                </div>
                <div class="prot-actions">
                  <label class="switch" aria-label="Toggle Intrusion Detection">
                    <input type="checkbox" role="switch" checked={true} />
                    <span class="switch__track"></span>
                  </label>
                  <button class="btn btn--ghost btn--icon" title="Pengaturan Intrusion" onclick={() => (activeTab = 'anti_intrusion')}>
                    <i class="ri-settings-3-line"></i>
                  </button>
                </div>
              </div>

              <!-- Fail2Ban Daemon -->
              <div class="prot-row">
                <div class="prot-info">
                  <div class="prot-title-wrap">
                    <span class="dot" class:dot--success={f2bStatus?.running}></span>
                    <span class="prot-name">Fail2Ban</span>
                  </div>
                  <p class="prot-sub">{f2bStatus?.running ? `${f2bStatus.jails.length} Jail aktif memantau brute-force` : 'Layanan Fail2Ban belum aktif'}</p>
                </div>
                <div class="prot-actions">
                  <label class="switch" aria-label="Toggle Fail2Ban">
                    <input type="checkbox" role="switch" checked={f2bStatus ? f2bStatus.running : false} />
                    <span class="switch__track"></span>
                  </label>
                  <button class="btn btn--ghost btn--icon" title="Detail Fail2Ban" onclick={() => (activeTab = 'anti_intrusion')}>
                    <i class="ri-settings-3-line"></i>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Bagian Tengah: 4 Kartu Hasil Pemindaian (Scan Overview) -->
      <section class="scan-summary-section">
        <div class="sec-subhead">
          <h3 class="sec-subtitle">Security Scan</h3>
          <span class="badge badge--info badge--pill">Scan Overview</span>
        </div>

        <div class="scan-cards-grid">
          <!-- Security Risk -->
          <div class="card scan-stat-card">
            <div class="card__head">
              <span class="stat-category-title">Security Risk</span>
              <span class="badge {scanReport && scanReport.risk_count > 0 ? 'badge--danger' : 'badge--success'}">
                {scanReport ? `${scanReport.risk_count} Risks` : 'No data'}
              </span>
            </div>
            <div class="card__body">
              <p class="stat-status-text">
                {scanReport ? (scanReport.risk_count > 0 ? `${scanReport.risk_count} risiko konfigurasi ditemukan` : 'No security risks') : 'Menunggu pemindaian'}
              </p>
              <button class="link-btn" onclick={() => { activeTab = 'scan_detection'; scanCategoryFilter = 'risk'; }}>
                Details &rarr;
              </button>
            </div>
          </div>

          <!-- Malware Detection -->
          <div class="card scan-stat-card">
            <div class="card__head">
              <span class="stat-category-title">Malware Detection</span>
              <span class="badge {scanReport && scanReport.malware_count > 0 ? 'badge--danger' : 'badge--success'}">
                {scanReport ? `${scanReport.malware_count} Threats` : 'No data'}
              </span>
            </div>
            <div class="card__body">
              <p class="stat-status-text">
                {scanReport ? (scanReport.malware_count > 0 ? 'Indikasi backdoor PHP terdeteksi' : 'No threats detected') : 'Menunggu pemindaian'}
              </p>
              <button class="link-btn" onclick={() => { activeTab = 'scan_detection'; scanCategoryFilter = 'malware'; }}>
                Details &rarr;
              </button>
            </div>
          </div>

          <!-- Website Vulnerability -->
          <div class="card scan-stat-card">
            <div class="card__head">
              <span class="stat-category-title">Website Vulnerability</span>
              <span class="badge {scanReport && scanReport.vuln_count > 0 ? 'badge--warning' : 'badge--success'}">
                {scanReport ? `${scanReport.vuln_count} Vulns` : 'No data'}
              </span>
            </div>
            <div class="card__body">
              <p class="stat-status-text">
                {scanReport ? (scanReport.vuln_count > 0 ? 'Ditemukan berkas izin terbuka (.env)' : 'Website vulnerability scan result') : 'Menunggu pemindaian'}
              </p>
              <button class="link-btn" onclick={() => { activeTab = 'scan_detection'; scanCategoryFilter = 'vuln'; }}>
                Details &rarr;
              </button>
            </div>
          </div>

          <!-- Baseline -->
          <div class="card scan-stat-card">
            <div class="card__head">
              <span class="stat-category-title">Baseline</span>
              <span class="badge badge--info">
                {scanReport ? `${scanReport.baseline_pass}/${scanReport.baseline_total} Passed` : 'Waiting'}
              </span>
            </div>
            <div class="card__body">
              <p class="stat-status-text">
                {scanReport ? `${scanReport.baseline_pass} standar konfigurasi terpenuhi` : 'Waiting for first scan'}
              </p>
              <button class="link-btn" onclick={() => { activeTab = 'scan_detection'; scanCategoryFilter = 'baseline'; }}>
                Details &rarr;
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- Rekomendasi Tindakan (Recommended Actions) -->
      <section class="actions-section">
        <div class="sec-subhead">
          <h3 class="sec-subtitle">Recommended Actions</h3>
          <span class="badge badge--pill">
            {scanReport ? `${scanReport.findings.filter(f => f.severity !== 'safe').length} Rekomendasi` : 'Waiting for first scan'}
          </span>
        </div>

        {#if !scanReport || scanReport.findings.filter(f => f.severity !== 'safe').length === 0}
          <div class="card p-4">
            <p class="text-subtle text-center">
              Belum ada rekomendasi aktif. Jalankan "Start Scan" untuk mengecek postur keamanan sistem secara otomatis.
            </p>
          </div>
        {:else}
          <div class="recommendations-list">
            {#each scanReport.findings.filter(f => f.severity !== 'safe') as finding}
              <div class="card rec-card">
                <div class="rec-info">
                  <div class="rec-title-wrap">
                    <span class="badge {finding.severity === 'critical' ? 'badge--danger' : 'badge--warning'}">
                      {finding.severity.toUpperCase()}
                    </span>
                    <span class="rec-title">{finding.title}</span>
                  </div>
                  <p class="rec-desc">{finding.detail}</p>
                  <p class="rec-hint">Saran perbaikan: {finding.fixHint}</p>
                </div>
                <div class="rec-action-btn">
                  <button class="btn btn--secondary btn--sm" onclick={() => (activeTab = 'server_security')}>
                    Perbaiki Sekarang
                  </button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </section>
    </div>
  {/if}

  <!-- TAB 2: FIREWALL (AWS EC2 Security Group Style) -->
  {#if activeTab === 'firewall'}
    <div class="tab-pane">
      {#if !isEditingInbound}
        <!-- AWS EC2 Header Breadcrumb & Actions Bar -->
        <div class="aws-header-bar">
          <div class="aws-title-wrap">
            <h2 class="aws-main-title">sg-0645a2456451e6a9f - launch-wizard-fikpanel</h2>
          </div>
          <div class="aws-header-actions">
            <div class="dropdown-wrap" style="position: relative;">
              <button class="btn btn--secondary btn--sm aws-actions-btn" onclick={() => (showActionsDropdown = !showActionsDropdown)}>
                Actions <i class="ri-arrow-down-s-line"></i>
              </button>
              {#if showActionsDropdown}
                <div class="dropdown-menu aws-dropdown-menu">
                  <button class="dropdown-item" onclick={() => { showActionsDropdown = false; startEditInbound(); }}>
                    <i class="ri-edit-line"></i> Edit inbound rules
                  </button>
                  <button class="dropdown-item" onclick={() => { showActionsDropdown = false; toggleFirewall(); }}>
                    <i class="ri-shield-flash-line"></i> {fwStatus?.enabled ? 'Disable UFW' : 'Enable UFW'}
                  </button>
                  <button class="dropdown-item" onclick={() => { showActionsDropdown = false; toggleICMP(); }}>
                    <i class="ri-radar-line"></i> {fwStatus?.block_icmp ? 'Allow ICMP Ping' : 'Block ICMP Ping'}
                  </button>
                  <button class="dropdown-item" onclick={() => { showActionsDropdown = false; clearSiteLogs(); }}>
                    <i class="ri-delete-bin-line"></i> Clear web logs
                  </button>
                </div>
              {/if}
            </div>
          </div>
        </div>

        <!-- AWS EC2 Details Card -->
        <div class="card aws-details-card">
          <div class="aws-card-title">Details</div>
          <div class="aws-details-grid">
            <div class="aws-detail-item">
              <span class="aws-label">Security group name</span>
              <div class="aws-value-copy">
                <span class="aws-val-text font-bold">launch-wizard-fikpanel</span>
                <button class="copy-icon-btn" title="Copy name" onclick={() => { navigator.clipboard.writeText('launch-wizard-fikpanel'); notify('Security group name copied', 'info'); }}>
                  <i class="ri-file-copy-line"></i>
                </button>
              </div>
            </div>

            <div class="aws-detail-item">
              <span class="aws-label">Security group ID</span>
              <div class="aws-value-copy">
                <span class="aws-val-text mono font-semibold">sg-0645a2456451e6a9f</span>
                <button class="copy-icon-btn" title="Copy ID" onclick={() => { navigator.clipboard.writeText('sg-0645a2456451e6a9f'); notify('Security group ID copied', 'info'); }}>
                  <i class="ri-file-copy-line"></i>
                </button>
              </div>
            </div>

            <div class="aws-detail-item">
              <span class="aws-label">Description</span>
              <span class="aws-val-text text-muted">Linux Host Firewall (UFW) managed by FikPanel</span>
            </div>

            <div class="aws-detail-item">
              <span class="aws-label">VPC ID / Network Host</span>
              <span class="aws-val-text text-accent mono">vpc-0630356aa20ed371f (Host IP)</span>
            </div>

            <div class="aws-detail-item">
              <span class="aws-label">Owner</span>
              <div class="aws-value-copy">
                <span class="aws-val-text mono">691687039084</span>
                <button class="copy-icon-btn" title="Copy Owner" onclick={() => { navigator.clipboard.writeText('691687039084'); notify('Owner ID copied', 'info'); }}>
                  <i class="ri-file-copy-line"></i>
                </button>
              </div>
            </div>

            <div class="aws-detail-item">
              <span class="aws-label">Inbound rules count</span>
              <span class="aws-val-text font-bold text-accent">{fwStatus?.port_rules_count || 0} Permission entries</span>
            </div>

            <div class="aws-detail-item">
              <span class="aws-label">Outbound rules count</span>
              <span class="aws-val-text">1 Permission entry (All traffic allowed)</span>
            </div>

            <div class="aws-detail-item">
              <span class="aws-label">UFW Daemon Status</span>
              <span class="badge {fwStatus?.enabled ? 'badge--success' : 'badge--ghost'}">
                <span class="dot {fwStatus?.enabled ? 'dot--success' : ''}"></span>
                {fwStatus?.enabled ? 'Active (Running)' : 'Disabled'}
              </span>
            </div>
          </div>
        </div>

        <!-- AWS EC2 Sub-Navigation Tabs Bar -->
        <div class="aws-tabs-bar">
          <button
            class="aws-subtab-btn"
            class:active={fwSubTab === 'inbound'}
            onclick={() => (fwSubTab = 'inbound')}
          >
            Inbound rules
          </button>
          <button
            class="aws-subtab-btn"
            class:active={fwSubTab === 'outbound'}
            onclick={() => (fwSubTab = 'outbound')}
          >
            Outbound rules
          </button>
          <button
            class="aws-subtab-btn"
            class:active={fwSubTab === 'settings'}
            onclick={() => (fwSubTab = 'settings')}
          >
            Firewall Settings & Logs
          </button>
        </div>

        <!-- SUBTAB 1: Inbound rules Table -->
        {#if fwSubTab === 'inbound'}
          <div class="aws-table-container">
            <div class="aws-table-header-row">
              <div class="aws-table-title">
                Inbound rules ({filteredPortRules.length})
              </div>
              <div class="aws-table-actions">
                <div class="search-box">
                  <input
                    type="text"
                    class="input input--sm aws-search-input"
                    placeholder="Search rules, port, type..."
                    bind:value={fwPortSearch}
                  />
                </div>
                <button class="btn btn--ghost btn--icon" title="Refresh" onclick={loadFirewall}>
                  <i class="ri-refresh-line"></i>
                </button>
                <button class="btn btn--primary btn--sm aws-edit-rules-btn" onclick={startEditInbound}>
                  Edit inbound rules
                </button>
              </div>
            </div>

            <div class="table-wrap aws-table-wrap">
              <table class="table aws-table">
                <thead>
                  <tr>
                    <th style="width: 32px;"><input type="checkbox" aria-label="Select all" /></th>
                    <th>Name</th>
                    <th>Security group rule ID</th>
                    <th>IP version</th>
                    <th>Type</th>
                    <th>Protocol</th>
                    <th>Port range</th>
                    <th>Source</th>
                    <th>Description</th>
                    <th>Status</th>
                    <th class="text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {#if filteredPortRules.length === 0}
                    <tr>
                      <td colspan="11" class="text-center text-muted p-4">
                        Tidak ada aturan inbound yang sesuai dengan kriteria pencarian.
                      </td>
                    </tr>
                  {:else}
                    {#each filteredPortRules as r (r.number)}
                      <tr>
                        <td><input type="checkbox" aria-label="Select row" /></td>
                        <td class="text-subtle">-</td>
                        <td class="mono font-semibold text-accent" style="font-size: 0.8rem;">
                          {r.rule_id || `sgr-${r.port}00000000`}
                        </td>
                        <td>IPv4</td>
                        <td>
                          <span class="badge badge--pill badge--ghost font-semibold">
                            {r.rule_type || detectTypeFromPort(r.port, r.protocol)}
                          </span>
                        </td>
                        <td class="mono uppercase font-semibold">{r.protocol}</td>
                        <td class="mono font-bold text-white">{r.port}</td>
                        <td class="mono text-muted">{r.source_ip || '0.0.0.0/0'}</td>
                        <td class="text-subtle text-truncate" style="max-width: 200px;" title={r.remarks}>
                          {r.remarks || '-'}
                        </td>
                        <td>
                          <span class="badge {r.status.includes('Listening') ? 'badge--success' : 'badge--ghost'}">
                            <span class="dot {r.status.includes('Listening') ? 'dot--success' : ''}"></span>
                            {r.status}
                          </span>
                        </td>
                        <td class="text-right">
                          <button class="btn btn--ghost btn--sm text-danger" onclick={() => handleDeletePortRule(r)}>
                            Delete
                          </button>
                        </td>
                      </tr>
                    {/each}
                  {/if}
                </tbody>
              </table>
            </div>
          </div>
        {/if}

        <!-- SUBTAB 2: Outbound rules Table -->
        {#if fwSubTab === 'outbound'}
          <div class="aws-table-container">
            <div class="aws-table-header-row">
              <div class="aws-table-title">
                Outbound rules (1)
              </div>
            </div>

            <div class="table-wrap aws-table-wrap">
              <table class="table aws-table">
                <thead>
                  <tr>
                    <th>Security group rule ID</th>
                    <th>IP version</th>
                    <th>Type</th>
                    <th>Protocol</th>
                    <th>Port range</th>
                    <th>Destination</th>
                    <th>Description</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td class="mono font-semibold text-accent" style="font-size: 0.8rem;">sgr-0938bfe43810c9e71</td>
                    <td>IPv4</td>
                    <td><span class="badge badge--pill badge--ghost">All traffic</span></td>
                    <td class="mono uppercase">All</td>
                    <td class="mono">All</td>
                    <td class="mono">0.0.0.0/0</td>
                    <td class="text-subtle">Allow all outbound IPv4 traffic</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        {/if}

        <!-- SUBTAB 3: Firewall Settings & Logs -->
        {#if fwSubTab === 'settings'}
          <div class="card p-4">
            <h4 class="font-bold text-white mb-3">Pengaturan Tingkat Lanjut UFW & Logging</h4>
            <div class="fw-controls-row">
              <div class="fw-toggle-group">
                <span class="fw-control-label">Status Firewall UFW</span>
                <label class="switch" aria-label="Toggle Firewall UFW">
                  <input
                    type="checkbox"
                    role="switch"
                    checked={fwStatus ? fwStatus.enabled : false}
                    onchange={toggleFirewall}
                  />
                  <span class="switch__track"></span>
                </label>
              </div>

              <div class="fw-divider"></div>

              <div class="fw-toggle-group">
                <span class="fw-control-label">Blokir Ping ICMP Echo</span>
                <label class="switch" aria-label="Toggle Block ICMP Ping">
                  <input
                    type="checkbox"
                    role="switch"
                    checked={fwStatus ? fwStatus.block_icmp : false}
                    onchange={toggleICMP}
                  />
                  <span class="switch__track"></span>
                </label>
              </div>

              <div class="fw-divider"></div>

              <div class="fw-logs-group">
                <span class="fw-control-label">Berkas Log Web:</span>
                <span class="logs-path mono">{fwStatus?.site_logs_path || '/var/log/nginx'}</span>
                <span class="logs-size badge badge--info">{fwStatus?.site_logs_size || '0 KB'}</span>
                <button class="btn btn--secondary btn--sm" onclick={clearSiteLogs}>Bersihkan Log</button>
              </div>
            </div>
          </div>
        {/if}

      {:else}
        <!-- ======================================================== -->
        <!-- AWS EC2: EDIT INBOUND RULES FULL-VIEW MULTI-ROW BUILDER -->
        <!-- ======================================================== -->
        <div class="aws-edit-view">
          <div class="aws-edit-header">
            <h2 class="aws-edit-title">Edit inbound rules <span class="aws-info-link">Info</span></h2>
            <p class="aws-edit-sub">
              Inbound rules control the incoming traffic that's allowed to reach the instance.
            </p>
          </div>

          <div class="card aws-edit-card">
            <div class="aws-card-top-head">
              <span class="aws-card-heading">Inbound rules</span>
              <span class="aws-info-link">Info</span>
            </div>

            <div class="table-wrap aws-edit-table-wrap">
              <table class="aws-edit-table">
                <thead>
                  <tr>
                    <th style="min-width: 170px;">Security group rule ID</th>
                    <th style="min-width: 180px;">Type <span class="aws-info-link">Info</span></th>
                    <th style="min-width: 100px;">Protocol <span class="aws-info-link">Info</span></th>
                    <th style="min-width: 110px;">Port range <span class="aws-info-link">Info</span></th>
                    <th style="min-width: 130px;">Source type <span class="aws-info-link">Info</span></th>
                    <th style="min-width: 170px;">Source <span class="aws-info-link">Info</span></th>
                    <th style="min-width: 200px;">Description - optional <span class="aws-info-link">Info</span></th>
                    <th style="width: 90px;"></th>
                  </tr>
                </thead>
                <tbody>
                  {#each draftRules as rule, i}
                    <tr class="aws-edit-row">
                      <!-- Rule ID -->
                      <td>
                        <span class="mono text-muted text-xs">{rule.rule_id || '-'}</span>
                      </td>

                      <!-- Type Preset Dropdown -->
                      <td>
                        <select
                          class="input input--sm aws-input-select"
                          value={rule.rule_type || detectTypeFromPort(rule.port, rule.protocol)}
                          onchange={(e) => handleRuleTypeChange(i, (e.target as HTMLSelectElement).value)}
                        >
                          {#each ruleTypePresets as p}
                            <option value={p.type}>{p.type}</option>
                          {/each}
                        </select>
                      </td>

                      <!-- Protocol -->
                      <td>
                        <input
                          type="text"
                          class="input input--sm mono aws-input-disabled"
                          value={rule.protocol.toUpperCase()}
                          disabled={isPresetFixed(rule.rule_type)}
                          readonly={isPresetFixed(rule.rule_type)}
                        />
                      </td>

                      <!-- Port range -->
                      <td>
                        <input
                          type="text"
                          class="input input--sm mono"
                          class:aws-input-disabled={isPresetFixedPort(rule.rule_type)}
                          bind:value={rule.port}
                          placeholder="22, 80..."
                          disabled={isPresetFixedPort(rule.rule_type)}
                          readonly={isPresetFixedPort(rule.rule_type)}
                        />
                      </td>

                      <!-- Source type -->
                      <td>
                        <select
                          class="input input--sm aws-input-select"
                          onchange={(e) => handleSourceTypeChange(i, (e.target as HTMLSelectElement).value)}
                        >
                          <option value="Anywhere-IPv4" selected={rule.source_ip === '0.0.0.0/0' || rule.source_ip === 'All'}>Anywhere-IPv4</option>
                          <option value="Custom" selected={rule.source_ip !== '0.0.0.0/0' && rule.source_ip !== 'All'}>Custom</option>
                          <option value="My IP">My IP</option>
                        </select>
                      </td>

                      <!-- Source Input with Badge Chip -->
                      <td>
                        <div class="aws-source-input-wrap">
                          <div class="source-chip-box">
                            <span class="source-chip">
                              {rule.source_ip || '0.0.0.0/0'}
                              <button
                                type="button"
                                class="chip-remove"
                                onclick={() => (rule.source_ip = '0.0.0.0/0')}
                                title="Reset"
                              >
                                &times;
                              </button>
                            </span>
                          </div>
                        </div>
                      </td>

                      <!-- Description -->
                      <td>
                        <input
                          type="text"
                          class="input input--sm"
                          bind:value={rule.remarks}
                          placeholder="Keterangan aturan port..."
                        />
                      </td>

                      <!-- Delete Row Button -->
                      <td class="text-right">
                        <button
                          type="button"
                          class="btn btn--sm aws-btn-delete"
                          onclick={() => removeDraftRule(i)}
                        >
                          Delete
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>

            <!-- Add Rule Button -->
            <div class="aws-add-rule-row">
              <button type="button" class="btn btn--sm aws-btn-add-rule" onclick={addDraftRule}>
                <i class="ri-add-line"></i> Add rule
              </button>
            </div>
          </div>

          <!-- Alert Callout -->
          <div class="aws-warning-banner">
            <i class="ri-alert-line warning-icon"></i>
            <span class="warning-text">
              <strong>Rules with source of 0.0.0.0/0 or ::/0 allow all IP addresses to access your instance.</strong>
              We recommend setting security group rules to allow access from known IP addresses only.
            </span>
          </div>

          <!-- Sticky Bottom Actions -->
          <div class="aws-edit-footer-actions">
            <button type="button" class="btn btn--secondary" onclick={() => (isEditingInbound = false)}>
              Cancel
            </button>
            <button type="button" class="btn btn--primary aws-save-rules-btn" onclick={saveDraftRules}>
              Save rules
            </button>
          </div>
        </div>
      {/if}
    </div>
  {/if}

  <!-- TAB 3: SSH -->
  {#if activeTab === 'ssh'}
    <div class="tab-pane">
      <!-- Grid Atas: Status Layanan SSH & Statistik Login -->
      <div class="ssh-top-grid">
        <!-- Kartu Layanan SSH -->
        <div class="card ssh-service-card">
          <div class="card__head">
            <span class="card__title">SSH service</span>
            <label class="switch" aria-label="Toggle SSH Service">
              <input
                type="checkbox"
                role="switch"
                checked={sshStats ? sshStats.service_active : true}
                onchange={handleRestartSSH}
              />
              <span class="switch__track"></span>
            </label>
          </div>
          <div class="card__body">
            <div class="ssh-status-tag">
              <span class="badge badge--success badge--pill">
                <span class="dot dot--success dot--pulse"></span> SSH service enabled
              </span>
            </div>
            <p class="text-subtle mt-2">Current status: <strong class="text-accent">Running</strong></p>
            <div class="mt-4">
              <button class="btn btn--secondary btn--sm" onclick={handleRestartSSH}>
                <i class="ri-restart-line"></i> Restart SSH Daemon
              </button>
            </div>
          </div>
        </div>

        <!-- Kartu Statistik Percobaan Login SSH -->
        <div class="card ssh-stats-card">
          <div class="ssh-stat-boxes">
            <div class="stat-box success-box">
              <span class="stat-big-num">{sshStats?.successful_logins || 46}</span>
              <span class="stat-box-label">SSH login successful</span>
            </div>
            <div class="stat-box fail-box">
              <span class="stat-big-num">{sshStats?.failed_logins || 0}</span>
              <span class="stat-box-label">SSH login failed</span>
            </div>
          </div>

          <div class="ssh-timeline-wrap">
            <div class="timeline-row">
              <span class="timeline-label">Today</span>
              <div class="timeline-bar-bg">
                <div class="timeline-bar-fill" style="width: 75%;"></div>
              </div>
            </div>
            <div class="timeline-row">
              <span class="timeline-label">Yesterday</span>
              <div class="timeline-bar-bg">
                <div class="timeline-bar-fill" style="width: 45%;"></div>
              </div>
            </div>
            <div class="timeline-row">
              <span class="timeline-label">7 Days</span>
              <div class="timeline-bar-bg">
                <div class="timeline-bar-fill" style="width: 90%;"></div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Sub-tabs Navigasi SSH -->
      <div class="sub-nav-tabs">
        <button
          class="sub-tab-btn"
          class:active={sshSubTab === 'basic'}
          onclick={() => (sshSubTab = 'basic')}
        >
          Basic setup
        </button>
        <button
          class="sub-tab-btn"
          class:active={sshSubTab === 'accounts'}
          onclick={() => (sshSubTab = 'accounts')}
        >
          System Accounts
        </button>
        <button
          class="sub-tab-btn"
          class:active={sshSubTab === 'logs'}
          onclick={() => (sshSubTab = 'logs')}
        >
          SSH login logs
        </button>
      </div>

      <!-- Konten Subtab SSH: Basic setup -->
      {#if sshSubTab === 'basic'}
        <div class="card p-4">
          <h3 class="section-title">SSH Basic Settings</h3>

          <div class="setting-item-row">
            <div class="setting-info">
              <span class="setting-name">SSH Password login</span>
              <p class="setting-desc">Allow SSH login with password</p>
            </div>
            <label class="switch" aria-label="Toggle Password Login">
              <input type="checkbox" role="switch" bind:checked={sshPasswordAuth} />
              <span class="switch__track"></span>
            </label>
          </div>

          <div class="setting-item-row">
            <div class="setting-info">
              <span class="setting-name">SSH key login</span>
              <p class="setting-desc">Allow SSH login with cryptographic key</p>
            </div>
            <label class="switch" aria-label="Toggle Key Login">
              <input type="checkbox" role="switch" bind:checked={sshKeyAuth} />
              <span class="switch__track"></span>
            </label>
          </div>

          <div class="setting-item-row">
            <div class="setting-info">
              <span class="setting-name">SSH port</span>
              <p class="setting-desc">Port used by the SSH protocol. The default value is 22</p>
            </div>
            <div class="port-input-wrap">
              <input type="number" class="input input--sm mono" style="width: 110px;" bind:value={sshPortInput} />
              <button class="btn btn--primary btn--sm" onclick={handleSaveSSHConfig}>Save</button>
            </div>
          </div>

          <h3 class="section-title mt-6">Root Settings</h3>

          <div class="setting-item-row">
            <div class="setting-info">
              <span class="setting-name">Permit Root Login</span>
              <p class="setting-desc">Izinkan user root melakukan autentikasi langsung via SSH</p>
            </div>
            <label class="switch" aria-label="Toggle Root Login">
              <input type="checkbox" role="switch" bind:checked={sshPermitRoot} />
              <span class="switch__track"></span>
            </label>
          </div>
        </div>
      {/if}

      <!-- Konten Subtab SSH: System Accounts -->
      {#if sshSubTab === 'accounts'}
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Username</th>
                <th>UID</th>
                <th>GID</th>
                <th>Shell</th>
                <th>Home Directory</th>
                <th>Status</th>
                <th class="text-right">Operate</th>
              </tr>
            </thead>
            <tbody>
              {#each accounts as acc (acc.username)}
                <tr>
                  <td class="font-bold text-accent mono">{acc.username}</td>
                  <td class="mono text-muted">{acc.uid}</td>
                  <td class="mono text-muted">{acc.gid}</td>
                  <td class="mono text-subtle">{acc.shell}</td>
                  <td class="mono text-subtle">{acc.home_dir}</td>
                  <td>
                    <span class="badge {acc.locked ? 'badge--danger' : 'badge--success'}">
                      {acc.locked ? 'Locked' : 'Active'}
                    </span>
                  </td>
                  <td class="text-right">
                    <button class="btn btn--ghost btn--sm" onclick={() => handleToggleAccountLock(acc)}>
                      {acc.locked ? 'Buka Kunci' : 'Kunci Akun'}
                    </button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}

      <!-- Konten Subtab SSH: SSH login logs -->
      {#if sshSubTab === 'logs'}
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Time</th>
                <th>User</th>
                <th>Remote IP</th>
                <th>Port</th>
                <th>Status</th>
                <th>Message</th>
              </tr>
            </thead>
            <tbody>
              {#if !sshStats || !sshStats.recent_logs || sshStats.recent_logs.length === 0}
                <tr>
                  <td colspan="6" class="text-center text-muted p-4">Belum ada rekaman riwayat login SSH terbaru.</td>
                </tr>
              {:else}
                {#each sshStats.recent_logs as log}
                  <tr>
                    <td class="mono text-subtle">{new Date(log.timestamp * 1000).toLocaleString('id-ID')}</td>
                    <td class="mono font-bold text-accent">{log.user}</td>
                    <td class="mono text-muted">{log.ip}</td>
                    <td class="mono text-subtle">{log.port}</td>
                    <td>
                      <span class="badge {log.status === 'success' ? 'badge--success' : 'badge--danger'}">
                        {log.status.toUpperCase()}
                      </span>
                    </td>
                    <td class="text-subtle">{log.message}</td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}

  <!-- TAB 4: SERVER SECURITY (10 Poin Evaluasi & Skor) -->
  {#if activeTab === 'server_security'}
    <div class="tab-pane">
      <!-- Grid Atas: Rekap Login Terakhir SSH & Panel -->
      <div class="login-info-grid">
        <div class="card ssh-login-card">
          <div class="card__head">
            <span class="card__title">SSH login</span>
            <span class="badge badge--success">Success</span>
          </div>
          <div class="card__body">
            <p class="mono text-sm">IP: <strong class="text-accent">{serverSec?.last_ssh_login.ip || '127.0.0.1'}</strong></p>
            <p class="mono text-sm mt-1">Time: {serverSec?.last_ssh_login.time || '--'}</p>
            <p class="mono text-sm mt-1 text-subtle">Port: {serverSec?.last_ssh_login.port || 22}</p>
          </div>
        </div>

        <div class="card panel-login-card">
          <div class="card__head">
            <span class="card__title">Panel login</span>
            <span class="badge badge--success">Success</span>
          </div>
          <div class="card__body">
            <p class="mono text-sm">IP: <strong class="text-accent">{serverSec?.last_panel_login.ip || '127.0.0.1'}</strong></p>
            <p class="mono text-sm mt-1">Time: {serverSec?.last_panel_login.time || '--'}</p>
            <p class="mono text-sm mt-1 text-subtle">Port: {serverSec?.last_panel_login.port || 24441}</p>
          </div>
        </div>
      </div>

      <!-- Banner Skor Keselamatan (Safety rating: 40/100 Insecure / 85/100 Secure) -->
      <div class="safety-score-bar card">
        <div class="score-meta">
          <span class="safety-label">Safety rating:</span>
          <span class="score-display {serverSec && serverSec.score >= 80 ? 'text-accent' : 'text-danger'}">
            {serverSec?.score || 40} / 100
          </span>
          <span class="badge {serverSec && serverSec.score >= 80 ? 'badge--success' : 'badge--danger'}">
            {serverSec?.rating || 'Insecure'}
          </span>
        </div>
        <div>
          <button class="btn btn--primary btn--sm" onclick={() => handleApplySecurityFix('change_ssh_port')}>
            <i class="ri-shield-check-line"></i> Security configuration
          </button>
        </div>
      </div>

      <!-- Grid 10 Poin Evaluasi Keamanan -->
      <div class="checklist-grid">
        {#each serverSec?.checklist || [] as item}
          <div class="card checklist-card" class:card-danger={item.status === 'danger'}>
            <div class="checklist-head">
              <div class="checklist-title-wrap">
                <i class="{item.status === 'safe' ? 'ri-checkbox-circle-fill text-accent' : 'ri-close-circle-fill text-danger'} check-icon"></i>
                <span class="check-title">{item.title}</span>
              </div>
              <span class="badge {item.status === 'safe' ? 'badge--success' : 'badge--danger'}">
                {item.status.toUpperCase()}
              </span>
            </div>
            <p class="check-desc">{item.description}</p>
            {#if item.fixable && item.status !== 'safe'}
              <div class="mt-3">
                <button class="btn btn--secondary btn--sm" onclick={() => handleApplySecurityFix(item.action_key)}>
                  Perbaiki Sekarang
                </button>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- TAB 5: BRUTE FORCE PROTECTION -->
  {#if activeTab === 'bruteforce'}
    <div class="tab-pane">
      <!-- Peringatan IP Belum Masuk Whitelist -->
      {#if protection && !protection.is_client_whitelisted}
        <div class="alert alert--danger mb-4">
          <i class="ri-error-warning-fill"></i>
          <div class="flex-1">
            Your current IP address [<strong class="mono">{protection.client_ip}</strong>] is not on the whitelist.
          </div>
          <button class="btn btn--primary btn--sm" onclick={handleWhitelistCurrentIP}>
            Add to Whitelist
          </button>
        </div>
      {/if}

      <!-- Master Switch Proteksi Brute Force -->
      <div class="card p-4 mb-4 flex justify-between items-center">
        <div>
          <h3 class="font-bold text-md">FikPanel Brute Force Protection</h3>
          <p class="text-subtle text-xs mt-1">Proteksi otomatis terhadap serangan login berulang pada akun panel dan port SSH</p>
        </div>
        <label class="switch" aria-label="Toggle Master Brute Force">
          <input
            type="checkbox"
            role="switch"
            checked={bruteCfg ? bruteCfg.enabled : true}
            onchange={() => {
              if (bruteCfg) {
                bruteCfg.enabled = !bruteCfg.enabled;
                handleSaveBruteConfig();
              }
            }}
          />
          <span class="switch__track"></span>
        </label>
      </div>

      <!-- Sub-tabs Brute Force -->
      <div class="sub-nav-tabs">
        <button
          class="sub-tab-btn"
          class:active={bruteSubTab === 'config'}
          onclick={() => (bruteSubTab = 'config')}
        >
          Configuration
        </button>
        <button
          class="sub-tab-btn"
          class:active={bruteSubTab === 'whitelist'}
          onclick={() => (bruteSubTab = 'whitelist')}
        >
          WhiteList
        </button>
        <button
          class="sub-tab-btn"
          class:active={bruteSubTab === 'blacklist'}
          onclick={() => (bruteSubTab = 'blacklist')}
        >
          BlackList
        </button>
        <button
          class="sub-tab-btn"
          class:active={bruteSubTab === 'history'}
          onclick={() => (bruteSubTab = 'history')}
        >
          History Reports
        </button>
      </div>

      <!-- Subtab 1: Configuration -->
      {#if bruteSubTab === 'config' && bruteCfg}
        <div class="card p-4">
          <!-- Username-based Protection -->
          <div class="brute-section">
            <h4 class="font-bold text-sm">Username-based Protection (Based on fikpanel)</h4>
            <p class="text-subtle text-xs mt-1">
              Username-based protection tracks login attempts for user accounts. When disabled, will not lock user accounts, but existing account locks will remain.
            </p>
            <div class="flex items-center gap-3 mt-3">
              <span class="text-xs text-muted">Status:</span>
              <label class="switch" aria-label="Toggle Username Protection">
                <input type="checkbox" role="switch" bind:checked={bruteCfg.username_protection} />
                <span class="switch__track"></span>
              </label>
            </div>
          </div>

          <div class="fw-divider my-4"></div>

          <!-- IP Address-based Protection -->
          <div class="brute-section">
            <h4 class="font-bold text-sm">IP Address-based Protection (Based on ssh & panel)</h4>
            <p class="text-subtle text-xs mt-1">
              IP Address-based protection tracks login attempts from specific IP addresses. When disabled, will not block IP addresses, but existing blocks will remain.
            </p>
            <div class="flex items-center gap-3 mt-3">
              <span class="text-xs text-muted">Status:</span>
              <label class="switch" aria-label="Toggle IP Protection">
                <input type="checkbox" role="switch" bind:checked={bruteCfg.ip_protection} />
                <span class="switch__track"></span>
              </label>
            </div>
          </div>

          <div class="fw-divider my-4"></div>

          <!-- Login History Retention -->
          <div class="brute-section">
            <h4 class="font-bold text-sm">Login History</h4>
            <p class="text-subtle text-xs mt-1">Duration for Retaining Failed Logins (in minutes)</p>
            <div class="flex items-center gap-2 mt-2">
              <input
                type="number"
                class="input input--sm mono"
                style="width: 120px;"
                bind:value={bruteCfg.retention_minutes}
              />
              <button class="btn btn--primary btn--sm" onclick={handleSaveBruteConfig}>Save Configuration</button>
            </div>
          </div>
        </div>
      {/if}

      <!-- Subtab 2: WhiteList -->
      {#if bruteSubTab === 'whitelist'}
        <div class="card p-4">
          <div class="flex justify-between items-center mb-4">
            <span class="font-bold text-sm">Daftar Putih IP (Tidak Pernah Diblokir)</span>
            <button class="btn btn--primary btn--sm" onclick={() => (showAddWhitelistModal = true)}>
              <i class="ri-add-line"></i> Add to Whitelist
            </button>
          </div>
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>IP Address</th>
                  <th>Remarks</th>
                  <th>Added Time</th>
                  <th class="text-right">Operate</th>
                </tr>
              </thead>
              <tbody>
                {#if whitelist.length === 0}
                  <tr><td colspan="4" class="text-center text-muted p-4">Belum ada IP pada daftar putih.</td></tr>
                {:else}
                  {#each whitelist as w (w.ip)}
                    <tr>
                      <td class="mono font-bold text-accent">{w.ip}</td>
                      <td class="text-subtle">{w.remarks}</td>
                      <td class="mono text-subtle">{new Date(w.added_at * 1000).toLocaleString('id-ID')}</td>
                      <td class="text-right">
                        <button class="btn btn--ghost btn--sm text-danger" onclick={() => handleRemoveWhitelist(w.ip)}>
                          Remove
                        </button>
                      </td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>
        </div>
      {/if}

      <!-- Subtab 3: BlackList -->
      {#if bruteSubTab === 'blacklist'}
        <div class="card p-4">
          <div class="flex justify-between items-center mb-4">
            <span class="font-bold text-sm">Daftar Hitam IP (Diblokir Sistem)</span>
            <button class="btn btn--danger btn--sm" onclick={() => (showAddBlacklistModal = true)}>
              <i class="ri-forbid-line"></i> Add to Blacklist
            </button>
          </div>
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>IP Address</th>
                  <th>Reason</th>
                  <th>Blocked At</th>
                  <th>Expires At</th>
                  <th class="text-right">Operate</th>
                </tr>
              </thead>
              <tbody>
                {#if blacklist.length === 0}
                  <tr><td colspan="5" class="text-center text-muted p-4">Tidak ada IP yang sedang diblokir.</td></tr>
                {:else}
                  {#each blacklist as b (b.ip)}
                    <tr>
                      <td class="mono font-bold text-danger">{b.ip}</td>
                      <td class="text-subtle">{b.reason}</td>
                      <td class="mono text-subtle">{new Date(b.added_at * 1000).toLocaleString('id-ID')}</td>
                      <td class="mono text-subtle">{new Date(b.expires_at * 1000).toLocaleString('id-ID')}</td>
                      <td class="text-right">
                        <button class="btn btn--secondary btn--sm" onclick={() => handleRemoveBlacklist(b.ip)}>
                          Unban
                        </button>
                      </td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>
        </div>
      {/if}

      <!-- Subtab 4: History Reports -->
      {#if bruteSubTab === 'history'}
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Time</th>
                <th>Target User</th>
                <th>Source IP</th>
                <th>Origin</th>
                <th>Reason</th>
              </tr>
            </thead>
            <tbody>
              {#if failedEvents.length === 0}
                <tr><td colspan="5" class="text-center text-muted p-4">Tidak ada riwayat percobaan gagal tercatat.</td></tr>
              {:else}
                {#each failedEvents as ev}
                  <tr>
                    <td class="mono text-subtle">{new Date(ev.time * 1000).toLocaleString('id-ID')}</td>
                    <td class="mono font-bold text-accent">{ev.username}</td>
                    <td class="mono text-danger">{ev.ip}</td>
                    <td><span class="badge badge--pill">{ev.source.toUpperCase()}</span></td>
                    <td class="text-subtle">{ev.reason}</td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}

  <!-- TAB 6: ANTI INTRUSION & HARDENING -->
  {#if activeTab === 'anti_intrusion'}
    <div class="tab-pane">
      <!-- Fail2Ban Daemon Status & Jails -->
      <div class="card p-4 mb-6">
        <div class="card__head mb-3">
          <span class="card__title"><i class="ri-shield-flash-line"></i> Fail2Ban Intrusion Prevention Jails</span>
          <span class="badge {f2bStatus?.running ? 'badge--success' : 'badge--danger'}">
            {f2bStatus?.running ? 'Running' : 'Stopped'}
          </span>
        </div>

        {#if !f2bStatus || !f2bStatus.jails || f2bStatus.jails.length === 0}
          <p class="text-muted text-sm">Tidak ada jail Fail2Ban yang sedang aktif memantau.</p>
        {:else}
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>Jail Name</th>
                  <th>Currently Failed</th>
                  <th>Total Failed</th>
                  <th>Banned IPs</th>
                  <th class="text-right">Operate</th>
                </tr>
              </thead>
              <tbody>
                {#each f2bStatus.jails as j}
                  <tr>
                    <td class="mono font-bold text-accent">{j.name}</td>
                    <td class="mono text-muted">{j.currently_failed}</td>
                    <td class="mono text-muted">{j.total_failed}</td>
                    <td>
                      {#if j.banned_ips.length === 0}
                        <span class="text-subtle">None</span>
                      {:else}
                        {#each j.banned_ips as ip}
                          <span class="badge badge--danger mono mr-1">{ip}</span>
                        {/each}
                      {/if}
                    </td>
                    <td class="text-right">
                      {#each j.banned_ips as ip}
                        <button class="btn btn--secondary btn--sm" onclick={() => handleUnbanF2B(j.name, ip)}>
                          Unban {ip}
                        </button>
                      {/each}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>

      <!-- Pengerasan Sistem Operasi & Akses Compiler -->
      <div class="card p-4">
        <h3 class="section-title mb-4">Kernel & System Hardening</h3>

        <!-- Compiler Access -->
        <div class="setting-item-row">
          <div class="setting-info">
            <span class="setting-name">Compiler Access (gcc, make)</span>
            <p class="setting-desc">Batasi izin eksekusi compiler (chmod 700) hanya untuk root agar malware tidak dapat melakukan kompilasi biner di server</p>
          </div>
          <label class="switch" aria-label="Toggle Compiler Access">
            <input
              type="checkbox"
              role="switch"
              checked={hardening ? hardening.compiler_blocked : false}
              onchange={handleToggleCompilerAccess}
            />
            <span class="switch__track"></span>
          </label>
        </div>

        <!-- ICMP Ping Block -->
        <div class="setting-item-row">
          <div class="setting-info">
            <span class="setting-name">Block ICMP Ping Requests</span>
            <p class="setting-desc">Cegah server merespons ping ICMP dari jaringan publik (net.ipv4.icmp_echo_ignore_all = 1)</p>
          </div>
          <label class="switch" aria-label="Toggle ICMP Hardening">
            <input
              type="checkbox"
              role="switch"
              checked={fwStatus ? fwStatus.block_icmp : false}
              onchange={toggleICMP}
            />
            <span class="switch__track"></span>
          </label>
        </div>

        <!-- SYN Flood Protection -->
        <div class="setting-item-row">
          <div class="setting-info">
            <span class="setting-name">TCP SYN Flood Cookies Protection</span>
            <p class="setting-desc">Aktifkan perlindungan mitigasi serangan DDoS paket SYN setengah terbuka (net.ipv4.tcp_syncookies = 1)</p>
          </div>
          <span class="badge badge--success">AKTIF</span>
        </div>

        <!-- IP Spoofing Prevention -->
        <div class="setting-item-row">
          <div class="setting-info">
            <span class="setting-name">IP Spoofing Reverse Path Filter</span>
            <p class="setting-desc">Verifikasi alamat sumber paket untuk mencegah spoofing (net.ipv4.conf.all.rp_filter = 1)</p>
          </div>
          <span class="badge badge--success">AKTIF</span>
        </div>
      </div>
    </div>
  {/if}

  <!-- TAB 7: SCAN DETECTION -->
  {#if activeTab === 'scan_detection'}
    <div class="tab-pane">
      <header class="pane-header">
        <div>
          <h2 class="pane-title">Scan Detection & Vulnerability Analysis</h2>
          <p class="pane-subtitle">Hasil pemindaian heuristik mendalam terhadap berkas web, izin berkas kritis, dan konfigurasi server.</p>
        </div>
        <button class="btn btn--primary" disabled={isScanning} onclick={runSecurityScan}>
          <i class="ri-radar-line"></i> {isScanning ? 'Memindai...' : 'Jalankan Pemindaian Baru'}
        </button>
      </header>

      <!-- Kategori Filter Hasil -->
      <div class="segmented mb-4">
        <button class:active={scanCategoryFilter === 'all'} onclick={() => (scanCategoryFilter = 'all')}>
          Semua Temuan
        </button>
        <button class:active={scanCategoryFilter === 'risk'} onclick={() => (scanCategoryFilter = 'risk')}>
          Security Risks
        </button>
        <button class:active={scanCategoryFilter === 'malware'} onclick={() => (scanCategoryFilter = 'malware')}>
          Malware / Webshell
        </button>
        <button class:active={scanCategoryFilter === 'vuln'} onclick={() => (scanCategoryFilter = 'vuln')}>
          Vulnerabilities
        </button>
        <button class:active={scanCategoryFilter === 'baseline'} onclick={() => (scanCategoryFilter = 'baseline')}>
          Baseline Checks
        </button>
      </div>

      {#if !scanReport || filteredScanFindings.length === 0}
        <div class="card p-8 text-center text-muted">
          <i class="ri-shield-check-line" style="font-size: 2.5rem; color: var(--accent);"></i>
          <p class="mt-2 font-bold text-white">Tidak ada temuan pada kategori ini.</p>
          <p class="text-xs text-subtle mt-1">Sistem berjalan dengan konfigurasi yang memenuhi standar keamanan.</p>
        </div>
      {:else}
        <div class="findings-list">
          {#each filteredScanFindings as finding}
            <div class="card finding-card">
              <div class="finding-head">
                <span class="badge {finding.severity === 'critical' ? 'badge--danger' : finding.severity === 'warning' ? 'badge--warning' : 'badge--success'}">
                  {finding.severity.toUpperCase()}
                </span>
                <span class="finding-title">{finding.title}</span>
                <span class="badge badge--pill mono text-subtle ml-auto">{finding.category}</span>
              </div>
              <p class="finding-detail">{finding.detail}</p>
              {#if finding.fix_hint && finding.fix_hint !== '-'}
                <div class="finding-fix">
                  <i class="ri-lightbulb-line text-warning"></i>
                  <span>Solusi: <strong class="mono">{finding.fix_hint}</strong></span>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

<!-- MODAL: ADD PORT RULE -->
{#if showAddPortModal}
  <div class="modal-backdrop" onclick={() => (showAddPortModal = false)}>
    <div class="modal-content" onclick={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <h3 class="modal-title">Tambah Aturan Port Firewall</h3>
        <button class="btn btn--ghost btn--icon" onclick={() => (showAddPortModal = false)} aria-label="Tutup Modal">
          <i class="ri-close-line"></i>
        </button>
      </div>
      <div class="modal-body">
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
      </div>
      <div class="modal-footer">
        <button class="btn btn--secondary" onclick={() => (showAddPortModal = false)}>Batal</button>
        <button class="btn btn--primary" onclick={handleAddPortRule}>Terapkan Aturan</button>
      </div>
    </div>
  </div>
{/if}

<!-- MODAL: ADD WHITELIST IP -->
{#if showAddWhitelistModal}
  <div class="modal-backdrop" onclick={() => (showAddWhitelistModal = false)}>
    <div class="modal-content" onclick={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <h3 class="modal-title">Tambah IP ke Whitelist</h3>
        <button class="btn btn--ghost btn--icon" onclick={() => (showAddWhitelistModal = false)} aria-label="Tutup Modal">
          <i class="ri-close-line"></i>
        </button>
      </div>
      <div class="modal-body">
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
      </div>
      <div class="modal-footer">
        <button class="btn btn--secondary" onclick={() => (showAddWhitelistModal = false)}>Batal</button>
        <button class="btn btn--primary" onclick={handleAddWhitelist}>Simpan Whitelist</button>
      </div>
    </div>
  </div>
{/if}

<!-- MODAL: ADD BLACKLIST IP -->
{#if showAddBlacklistModal}
  <div class="modal-backdrop" onclick={() => (showAddBlacklistModal = false)}>
    <div class="modal-content" onclick={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <h3 class="modal-title">Blokir Alamat IP (Blacklist)</h3>
        <button class="btn btn--ghost btn--icon" onclick={() => (showAddBlacklistModal = false)} aria-label="Tutup Modal">
          <i class="ri-close-line"></i>
        </button>
      </div>
      <div class="modal-body">
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
      </div>
      <div class="modal-footer">
        <button class="btn btn--secondary" onclick={() => (showAddBlacklistModal = false)}>Batal</button>
        <button class="btn btn--danger" onclick={handleAddBlacklist}>Blokir IP</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .sec-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  /* Bilah Tab Navigasi Atas */
  .sec-nav-bar {
    border-bottom: 1px solid var(--border);
    padding-bottom: var(--space-1);
    overflow-x: auto;
  }
  .sec-tabs {
    display: flex;
    gap: var(--space-1);
    min-width: max-content;
  }
  .sec-tab-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    padding: var(--space-2) var(--space-4);
    border-radius: var(--radius-md);
    cursor: pointer;
    transition: color var(--duration-fast) ease, background var(--duration-fast) ease;
  }
  .sec-tab-btn:hover {
    color: var(--text);
    background: var(--bg-hover);
  }
  .sec-tab-btn.active {
    color: var(--accent-strong);
    background: var(--tint-accent);
    font-weight: var(--weight-bold);
  }

  /* Header Halaman / Pane */
  .pane-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-3);
    margin-bottom: var(--space-4);
  }
  .pane-title {
    font-size: var(--text-xl);
    font-weight: var(--weight-bold);
    color: var(--text);
  }
  .pane-subtitle {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin-top: var(--space-1);
  }
  .header-right {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  /* Grid Atas Overview */
  .overview-top-grid {
    display: grid;
    grid-template-columns: 320px 1fr;
    gap: var(--space-4);
    margin-bottom: var(--space-6);
  }
  @media (max-width: 960px) {
    .overview-top-grid {
      grid-template-columns: 1fr;
    }
  }

  .scan-posture-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: var(--space-6);
  }
  .radar-visual {
    margin-bottom: var(--space-4);
  }
  .radar-circle {
    width: 88px;
    height: 88px;
    border-radius: var(--radius-full);
    border: 2px dashed color-mix(in srgb, var(--accent) 45%, transparent);
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--tint-accent);
  }
  .radar-circle.spinning {
    animation: fp-spin 2s linear infinite;
  }
  .radar-icon {
    font-size: 2.2rem;
    color: var(--accent-strong);
  }
  .posture-title {
    font-size: var(--text-lg);
    font-weight: var(--weight-bold);
    color: var(--text);
    margin-bottom: var(--space-1);
  }
  .posture-desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin-bottom: var(--space-4);
    line-height: var(--leading-normal);
  }
  .scan-score-badge {
    margin-bottom: var(--space-2);
  }
  .score-number {
    font-size: var(--text-2xl);
    font-weight: var(--weight-bold);
    color: var(--accent-strong);
    font-family: var(--font-mono);
  }
  .score-total {
    font-size: var(--text-sm);
    color: var(--text-subtle);
  }

  /* Daftar Proteksi 6 Modul */
  .protection-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .prot-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--space-2) var(--space-1);
    border-bottom: 1px solid var(--border-subtle);
  }
  .prot-row:last-child {
    border-bottom: none;
  }
  .prot-title-wrap {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .prot-name {
    font-size: var(--text-sm);
    font-weight: var(--weight-bold);
    color: var(--text);
  }
  .prot-sub {
    font-size: var(--text-2xs);
    color: var(--text-muted);
    margin-top: 2px;
  }
  .prot-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  /* Ringkasan 4 Kartu Pemindaian */
  .scan-summary-section {
    margin-bottom: var(--space-6);
  }
  .sec-subhead {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: var(--space-3);
  }
  .sec-subtitle {
    font-size: var(--text-md);
    font-weight: var(--weight-bold);
    color: var(--text);
  }
  .scan-cards-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--space-3);
  }
  .stat-category-title {
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    color: var(--text-secondary);
  }
  .stat-status-text {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin-bottom: var(--space-3);
  }
  .link-btn {
    background: transparent;
    border: none;
    color: var(--accent-strong);
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    cursor: pointer;
    padding: 0;
  }
  .link-btn:hover {
    text-decoration: underline;
  }

  /* Rekomendasi Tindakan */
  .recommendations-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .rec-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--space-3) var(--space-4);
  }
  .rec-title-wrap {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: 2px;
  }
  .rec-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-bold);
    color: var(--text);
  }
  .rec-desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .rec-hint {
    font-size: var(--text-2xs);
    color: var(--text-subtle);
    font-family: var(--font-mono);
    margin-top: 2px;
  }

  /* Firewall Elements */
  .fw-top-card {
    padding: var(--space-3) var(--space-4);
    margin-bottom: var(--space-3);
  }
  .fw-controls-row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-4);
  }
  .fw-toggle-group, .fw-logs-group {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .fw-control-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    color: var(--text-secondary);
  }
  .fw-divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  .logs-path {
    font-size: var(--text-xs);
    color: var(--accent-strong);
  }

  .fw-metric-bar {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-1);
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin-bottom: var(--space-3);
  }
  .metric-sep {
    color: var(--border-strong);
  }

  .fw-action-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-3);
    margin-bottom: var(--space-3);
  }
  .action-left, .action-right {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  /* ── AWS EC2 Security Groups UI Styling ──────────────────────────────────── */
  .aws-header-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: var(--space-3);
  }
  .aws-main-title {
    font-size: var(--text-lg);
    font-weight: var(--weight-bold);
    color: var(--text);
  }
  .aws-actions-btn {
    border-radius: var(--radius-full);
    padding: var(--space-1) var(--space-3);
  }
  .aws-dropdown-menu {
    position: absolute;
    right: 0;
    top: 100%;
    margin-top: var(--space-1);
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    box-shadow: 0 10px 25px rgba(0,0,0,0.5);
    z-index: 50;
    min-width: 200px;
    padding: var(--space-1);
  }
  .dropdown-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    font-size: var(--text-xs);
    color: var(--text-muted);
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
  }
  .dropdown-item:hover {
    color: var(--text);
    background: var(--bg-hover);
  }

  /* Details Card */
  .aws-details-card {
    padding: var(--space-4) var(--space-5);
    margin-bottom: var(--space-4);
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
  }
  .aws-card-title {
    font-size: var(--text-md);
    font-weight: var(--weight-bold);
    color: var(--text);
    margin-bottom: var(--space-3);
  }
  .aws-details-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--space-4);
  }
  .aws-detail-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .aws-label {
    font-size: var(--text-2xs);
    font-weight: var(--weight-semibold);
    color: var(--text-subtle);
    text-transform: capitalize;
  }
  .aws-val-text {
    font-size: var(--text-xs);
    color: var(--text);
  }
  .aws-value-copy {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .copy-icon-btn {
    background: transparent;
    border: none;
    color: var(--accent-strong);
    cursor: pointer;
    font-size: 0.9rem;
    padding: 0;
  }
  .copy-icon-btn:hover {
    color: var(--text);
  }

  /* AWS Sub-Navigation Tabs */
  .aws-tabs-bar {
    display: flex;
    gap: var(--space-4);
    border-bottom: 2px solid var(--border-subtle);
    margin-bottom: var(--space-4);
  }
  .aws-subtab-btn {
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    margin-bottom: -2px;
    padding: var(--space-2) var(--space-1);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
    cursor: pointer;
    transition: all var(--duration-fast) ease;
  }
  .aws-subtab-btn:hover {
    color: var(--text);
  }
  .aws-subtab-btn.active {
    color: var(--accent-strong);
    border-bottom-color: var(--accent-strong);
    font-weight: var(--weight-bold);
  }

  /* Table Container */
  .aws-table-container {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    overflow: hidden;
  }
  .aws-table-header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
    flex-wrap: wrap;
    gap: var(--space-3);
  }
  .aws-table-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-bold);
    color: var(--text);
  }
  .aws-table-actions {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .aws-search-input {
    min-width: 240px;
    border-radius: var(--radius-full);
  }
  .aws-edit-rules-btn {
    border-radius: var(--radius-full);
    font-weight: var(--weight-semibold);
    padding: 6px 18px;
  }
  .aws-table th {
    white-space: nowrap;
    font-size: var(--text-2xs);
    font-weight: var(--weight-semibold);
  }
  .aws-table td {
    font-size: var(--text-xs);
  }

  /* Edit View Multi-Row Builder */
  .aws-edit-view {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .aws-edit-header {
    margin-bottom: var(--space-1);
  }
  .aws-edit-title {
    font-size: var(--text-xl);
    font-weight: var(--weight-bold);
    color: var(--text);
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .aws-info-link {
    font-size: var(--text-xs);
    color: var(--accent-strong);
    cursor: pointer;
    font-weight: normal;
  }
  .aws-info-link:hover {
    text-decoration: underline;
  }
  .aws-edit-sub {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin-top: 2px;
  }
  .aws-edit-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    padding: var(--space-4);
    border-radius: var(--radius-lg);
  }
  .aws-card-top-head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: var(--space-3);
  }
  .aws-card-heading {
    font-size: var(--text-md);
    font-weight: var(--weight-bold);
    color: var(--text);
  }
  .aws-edit-table-wrap {
    overflow-x: auto;
    margin-bottom: var(--space-4);
  }
  .aws-edit-table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--text-xs);
  }
  .aws-edit-table th {
    text-align: left;
    padding: var(--space-2) var(--space-3);
    color: var(--text-secondary);
    font-weight: var(--weight-semibold);
    border-bottom: 1px solid var(--border);
    font-size: var(--text-2xs);
  }
  .aws-edit-row td {
    padding: var(--space-2) var(--space-3);
    vertical-align: middle;
    border-bottom: 1px solid var(--border-subtle);
  }
  .aws-input-select {
    cursor: pointer;
  }
  .aws-input-disabled {
    opacity: 0.55;
    background: var(--bg-subtle) !important;
    cursor: not-allowed;
  }
  .aws-source-input-wrap {
    display: flex;
    align-items: center;
  }
  .source-chip-box {
    display: flex;
    align-items: center;
  }
  .source-chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background: var(--tint-accent);
    color: var(--accent-strong);
    border: 1px solid var(--accent-subtle);
    border-radius: var(--radius-sm);
    padding: 3px 8px;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }
  .chip-remove {
    background: transparent;
    border: none;
    color: var(--accent-strong);
    cursor: pointer;
    font-size: 1rem;
    line-height: 1;
    padding: 0;
  }
  .chip-remove:hover {
    color: var(--danger-strong);
  }
  .aws-btn-delete {
    border: 1px solid var(--accent-strong);
    color: var(--accent-strong);
    border-radius: var(--radius-full);
    background: transparent;
    font-weight: var(--weight-semibold);
    padding: 4px 14px;
    cursor: pointer;
    transition: all var(--duration-fast) ease;
  }
  .aws-btn-delete:hover {
    background: var(--tint-accent);
  }
  .aws-add-rule-row {
    padding-top: var(--space-2);
  }
  .aws-btn-add-rule {
    border: 1px solid var(--accent-strong);
    color: var(--accent-strong);
    border-radius: var(--radius-full);
    background: transparent;
    font-weight: var(--weight-semibold);
    padding: 6px 18px;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    transition: all var(--duration-fast) ease;
  }
  .aws-btn-add-rule:hover {
    background: var(--tint-accent);
  }
  .aws-warning-banner {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    background: color-mix(in srgb, var(--warning) 12%, transparent);
    border-left: 4px solid var(--warning);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-md);
    font-size: var(--text-xs);
    color: var(--text);
  }
  .warning-icon {
    color: var(--warning);
    font-size: 1.1rem;
    flex-shrink: 0;
    margin-top: 2px;
  }
  .aws-edit-footer-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
    margin-top: var(--space-2);
  }
  .aws-save-rules-btn {
    border-radius: var(--radius-full);
    padding: 6px 22px;
    font-weight: var(--weight-bold);
  }

  /* SSH Elements */
  .ssh-top-grid {
    display: grid;
    grid-template-columns: 320px 1fr;
    gap: var(--space-4);
    margin-bottom: var(--space-4);
  }
  @media (max-width: 860px) {
    .ssh-top-grid {
      grid-template-columns: 1fr;
    }
  }
  .ssh-stat-boxes {
    display: flex;
    gap: var(--space-3);
    padding: var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
  }
  .stat-box {
    flex: 1;
    padding: var(--space-3);
    border-radius: var(--radius-md);
    text-align: center;
  }
  .success-box {
    background: var(--tint-accent);
    border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
  }
  .fail-box {
    background: var(--tint-danger);
    border: 1px solid color-mix(in srgb, var(--danger) 30%, transparent);
  }
  .stat-big-num {
    font-size: var(--text-xl);
    font-weight: var(--weight-bold);
    font-family: var(--font-mono);
    display: block;
  }
  .success-box .stat-big-num { color: var(--accent-strong); }
  .fail-box .stat-big-num { color: var(--danger-strong); }
  .stat-box-label {
    font-size: var(--text-2xs);
    color: var(--text-muted);
  }

  .ssh-timeline-wrap {
    padding: var(--space-3) var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .timeline-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    font-size: var(--text-xs);
  }
  .timeline-label {
    width: 70px;
    color: var(--text-muted);
  }
  .timeline-bar-bg {
    flex: 1;
    height: 6px;
    background: var(--bg-inset);
    border-radius: var(--radius-full);
    overflow: hidden;
  }
  .timeline-bar-fill {
    height: 100%;
    background: var(--accent);
    border-radius: var(--radius-full);
  }

  .sub-nav-tabs {
    display: flex;
    gap: var(--space-2);
    border-bottom: 1px solid var(--border);
    padding-bottom: var(--space-1);
    margin-bottom: var(--space-4);
  }
  .sub-tab-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-sm);
    cursor: pointer;
  }
  .sub-tab-btn.active {
    color: var(--accent-strong);
    background: var(--tint-accent);
  }

  .section-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-bold);
    color: var(--text);
    margin-bottom: var(--space-3);
  }
  .setting-item-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--space-3) 0;
    border-bottom: 1px solid var(--border-subtle);
  }
  .setting-item-row:last-child {
    border-bottom: none;
  }
  .setting-name {
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--text);
  }
  .setting-desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin-top: 2px;
  }
  .port-input-wrap {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }

  /* Server Security Grid */
  .login-info-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-4);
    margin-bottom: var(--space-4);
  }
  @media (max-width: 720px) {
    .login-info-grid {
      grid-template-columns: 1fr;
    }
  }
  .safety-score-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--space-4);
    margin-bottom: var(--space-4);
  }
  .score-meta {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .safety-label {
    font-size: var(--text-sm);
    font-weight: var(--weight-bold);
    color: var(--text);
  }
  .score-display {
    font-size: var(--text-xl);
    font-weight: var(--weight-bold);
    font-family: var(--font-mono);
  }

  .checklist-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: var(--space-3);
  }
  .checklist-card {
    padding: var(--space-4);
  }
  .card-danger {
    border-color: color-mix(in srgb, var(--danger) 40%, transparent);
    background: color-mix(in srgb, var(--tint-danger) 25%, var(--bg-surface));
  }
  .checklist-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: var(--space-2);
  }
  .checklist-title-wrap {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .check-icon {
    font-size: 1.1rem;
  }
  .check-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-bold);
    color: var(--text);
  }
  .check-desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: var(--leading-normal);
  }

  /* Modal Form */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: var(--bg-overlay);
    z-index: var(--z-modal);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-4);
  }
  .modal-content {
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    width: 100%;
    max-width: 460px;
    overflow: hidden;
    box-shadow: var(--shadow-lg);
  }
  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
  }
  .modal-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-bold);
    color: var(--text);
  }
  .modal-body {
    padding: var(--space-4);
  }
  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    border-top: 1px solid var(--border-subtle);
  }

  /* Scan Detection Findings */
  .findings-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .finding-card {
    padding: var(--space-4);
  }
  .finding-head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: var(--space-2);
  }
  .finding-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-bold);
    color: var(--text);
  }
  .finding-detail {
    font-size: var(--text-xs);
    color: var(--text-secondary);
    margin-bottom: var(--space-2);
  }
  .finding-fix {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-2xs);
    color: var(--text-muted);
    background: var(--bg-inset);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
  }

  /* Utilitas */
  .mono { font-family: var(--font-mono); }
  .text-accent { color: var(--accent-strong); }
  .text-danger { color: var(--danger-strong); }
  .text-warning { color: var(--warning-strong); }
  .text-muted { color: var(--text-muted); }
  .text-subtle { color: var(--text-subtle); }
  .text-right { text-align: right; }
  .text-center { text-align: center; }
  .font-bold { font-weight: var(--weight-bold); }
  .flex { display: flex; }
  .flex-1 { flex: 1; }
  .justify-between { justify-content: space-between; }
  .items-center { align-items: center; }
  .gap-2 { gap: var(--space-2); }
  .gap-3 { gap: var(--space-3); }
  .mt-1 { margin-top: var(--space-1); }
  .mt-2 { margin-top: var(--space-2); }
  .mt-3 { margin-top: var(--space-3); }
  .mt-4 { margin-top: var(--space-4); }
  .mt-6 { margin-top: var(--space-6); }
  .mb-2 { margin-bottom: var(--space-2); }
  .mb-3 { margin-bottom: var(--space-3); }
  .mb-4 { margin-bottom: var(--space-4); }
  .mb-6 { margin-bottom: var(--space-6); }
  .mr-1 { margin-right: var(--space-1); }
  .p-4 { padding: var(--space-4); }
  .p-8 { padding: var(--space-8); }
  @keyframes fp-spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>