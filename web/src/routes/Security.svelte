<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type {
    FirewallExtendedStatus,
    FirewallRule,
    SSHStats,
    SSHAuthorizedKey,
    SystemAccount,
    BruteForceConfig,
    SecurityIPRule,
    SecurityLoginEvent,
    ProtectionOverview,
    ProtectionItem,
    HardeningStatus,
    FIMCheckResult,
    SecurityScanReport,
    ScanFinding,
    Fail2BanStatus,
  } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import PageHeader from '../lib/components/ui/PageHeader.svelte';
  import SecurityTabs, { type SecurityTabId } from './security/SecurityTabs.svelte';
  import OverviewTab from './security/OverviewTab.svelte';
  import FirewallTab, { type FirewallSubTab } from './security/FirewallTab.svelte';
  import SshTab, { type SshSubTab } from './security/SshTab.svelte';
  import BruteForceTab, { type BruteForceSubTab } from './security/BruteForceTab.svelte';
  import HardeningTab, { type HardeningSubTab } from './security/HardeningTab.svelte';
  import AddPortModal from './security/AddPortModal.svelte';
  import AddWhitelistModal from './security/AddWhitelistModal.svelte';
  import AddBlacklistModal from './security/AddBlacklistModal.svelte';
  import '../styles/pages/security/shared.css';

  // Tab & sub-tab aktif
  let activeTab = $state<SecurityTabId>('overview');
  let fwSubTab = $state<FirewallSubTab>('inbound');
  let sshSubTab = $state<SshSubTab>('config');
  let bruteSubTab = $state<BruteForceSubTab>('config');
  let hardeningSubTab = $state<HardeningSubTab>('scanner');
  let scanCategoryFilter = $state<string>('all');

  // State data utama Security Center
  let loading = $state(true);
  let busyKey = $state('');
  let isScanning = $state(false);
  let protection = $state<ProtectionOverview | null>(null);
  let fwStatus = $state<FirewallExtendedStatus | null>(null);
  let sshStats = $state<SSHStats | null>(null);
  let sshKeys = $state<SSHAuthorizedKey[]>([]);
  let accounts = $state<SystemAccount[]>([]);
  let bruteCfg = $state<BruteForceConfig | null>(null);
  let clientIP = $state('');
  let isClientWhitelisted = $state(false);
  let whitelist = $state<SecurityIPRule[]>([]);
  let blacklist = $state<SecurityIPRule[]>([]);
  let loginEvents = $state<SecurityLoginEvent[]>([]);
  let hardening = $state<HardeningStatus | null>(null);
  let f2bStatus = $state<Fail2BanStatus | null>(null);
  let fimResult = $state<FIMCheckResult | null>(null);
  let scanReport = $state<SecurityScanReport | null>(null);

  // Pencarian & pilihan massal Firewall
  let fwSearch = $state('');
  let selectedRuleNums = $state<number[]>([]);

  // Form SSH
  let sshPortInput = $state(22);
  let sshPasswordAuth = $state(true);
  let sshPubkeyAuth = $state(true);
  let sshPermitRoot = $state('prohibit-password');
  let sshMaxAuthTries = $state(6);
  let newPublicKey = $state('');

  // Modal Tambah Aturan Firewall
  let showAddPortModal = $state(false);
  let newPort = $state('');
  let newProto = $state('tcp');
  let newAction = $state('allow');
  let newDirection = $state('in');
  let newSourceIP = $state('0.0.0.0/0');
  let newRemarks = $state('');

  // Modal Whitelist & Blacklist
  let showAddWhitelistModal = $state(false);
  let newWhiteIP = $state('');
  let newWhiteRemarks = $state('');
  let showAddBlacklistModal = $state(false);
  let newBlackIP = $state('');
  let newBlackReason = $state('');
  let newBlackDuration = $state(1440);
  let newBlackSyncUFW = $state(false);

  // Metrik badge untuk bilah tab atas
  let issuesCount = $derived(
    scanReport ? scanReport.risk_count + scanReport.malware_count + scanReport.vuln_count : 0
  );
  let rulesCount = $derived(
    fwStatus ? fwStatus.inbound_count + fwStatus.outbound_count : 0
  );
  let f2bBannedCount = $derived.by(() => {
    if (!f2bStatus?.jails) return 0;
    return f2bStatus.jails.reduce((acc, j) => acc + (j.banned_count || 0), 0);
  });

  // ── Loader API Modular ────────────────────────────────────────────────────

  async function loadOverview() {
    try {
      const res = await apiFetch<{ ok: boolean; overview: ProtectionOverview }>('/api/security/overview');
      if (res?.overview) {
        protection = res.overview;
        clientIP = res.overview.client_ip || clientIP;
        isClientWhitelisted = res.overview.is_client_whitelisted;
        if (res.overview.latest_scan) {
          scanReport = res.overview.latest_scan;
        }
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat ikhtisar keamanan', 'error');
    }
  }

  async function loadFirewall() {
    try {
      const res = await apiFetch<{ ok: boolean; status: FirewallExtendedStatus }>('/api/firewall/extended');
      if (res?.status) {
        fwStatus = res.status;
        selectedRuleNums = [];
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat status firewall', 'error');
    }
  }

  async function loadSSH() {
    try {
      const [resStats, resKeys, resAccounts] = await Promise.all([
        apiFetch<{ ok: boolean; stats: SSHStats }>('/api/security/ssh/stats'),
        apiFetch<{ ok: boolean; keys: SSHAuthorizedKey[] }>('/api/security/ssh/keys'),
        apiFetch<{ ok: boolean; accounts: SystemAccount[] }>('/api/security/ssh/accounts'),
      ]);
      if (resStats?.stats) {
        sshStats = resStats.stats;
        sshPortInput = resStats.stats.config.port;
        sshPasswordAuth = resStats.stats.config.password_auth;
        sshPubkeyAuth = resStats.stats.config.pubkey_auth;
        sshPermitRoot = resStats.stats.config.permit_root_login || 'prohibit-password';
        sshMaxAuthTries = resStats.stats.config.max_auth_tries || 6;
      }
      if (resKeys?.keys) {
        sshKeys = resKeys.keys;
      }
      if (resAccounts?.accounts) {
        accounts = resAccounts.accounts;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat modul SSH', 'error');
    }
  }

  async function loadBruteForce() {
    try {
      const res = await apiFetch<{
        ok: boolean;
        client_ip: string;
        is_client_whitelisted: boolean;
        config: BruteForceConfig;
        whitelist: SecurityIPRule[];
        blacklist: SecurityIPRule[];
        history: SecurityLoginEvent[];
      }>('/api/security/bruteforce');
      if (res) {
        clientIP = res.client_ip || clientIP;
        isClientWhitelisted = res.is_client_whitelisted;
        bruteCfg = res.config;
        whitelist = res.whitelist || [];
        blacklist = res.blacklist || [];
        loginEvents = res.history || [];
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat modul Brute Force', 'error');
    }
  }

  async function loadHardening() {
    try {
      const [resF2B, resHard, resFIM] = await Promise.all([
        apiFetch<{ ok: boolean; fail2ban: Fail2BanStatus }>('/api/security/fail2ban'),
        apiFetch<{ ok: boolean; hardening: HardeningStatus }>('/api/security/hardening'),
        apiFetch<{ ok: boolean; fim: FIMCheckResult }>('/api/security/hardening/fim'),
      ]);
      if (resF2B?.fail2ban) f2bStatus = resF2B.fail2ban;
      if (resHard?.hardening) hardening = resHard.hardening;
      if (resFIM?.fim) fimResult = resFIM.fim;
    } catch (err: any) {
      notify(err.message || 'Gagal memuat modul hardening & Fail2Ban', 'error');
    }
  }

  async function refreshAll() {
    loading = true;
    try {
      await Promise.all([
        loadOverview(),
        loadFirewall(),
        loadSSH(),
        loadBruteForce(),
        loadHardening(),
      ]);
    } finally {
      loading = false;
    }
  }

  // ── Navigasi Lintas Tab & Lintas Halaman ──────────────────────────────────

  function handleNavigateTarget(target: string) {
    if (target === 'waf') {
      window.location.hash = '#/waf';
      return;
    }
    if (target === 'settings') {
      window.location.hash = '#/settings';
      return;
    }
    if (target === 'overview' || target === 'firewall' || target === 'ssh' || target === 'bruteforce' || target === 'hardening') {
      activeTab = target;
    }
  }

  function handleOpenScanCategory(cat: string) {
    scanCategoryFilter = cat;
    hardeningSubTab = 'scanner';
    activeTab = 'hardening';
  }

  // ── Aksi Audit & Quick Fix Aman ───────────────────────────────────────────

  async function runSecurityScan() {
    isScanning = true;
    try {
      const res = await apiFetch<{ ok: boolean; report: SecurityScanReport }>('/api/security/scan', {
        method: 'POST',
      });
      if (res?.report) {
        scanReport = res.report;
        notify(`Audit selesai — Skor keamanan: ${res.report.score}/100 (${res.report.rating})`, 'success');
        await loadOverview();
      }
    } catch (err: any) {
      notify(err.message || 'Gagal menjalankan audit keamanan', 'error');
    } finally {
      isScanning = false;
    }
  }

  async function handleApplyQuickFix(finding: ScanFinding) {
    if (!finding.action_key) return;
    const ok = await confirmDialog({
      title: 'Terapkan Perbaikan Keamanan',
      message: `Jalankan perbaikan otomatis untuk "${finding.title}"?`,
      confirmText: 'Terapkan Perbaikan',
      tone: 'default',
    });
    if (!ok) return;

    busyKey = `fix:${finding.action_key}`;
    try {
      const res = await apiFetch<{ ok: boolean; message: string; report?: SecurityScanReport }>('/api/security/fix', {
        method: 'POST',
        body: JSON.stringify({ action_key: finding.action_key }),
      });
      if (res?.report) scanReport = res.report;
      notify(res?.message || 'Perbaikan keamanan berhasil diterapkan', 'success');
      await Promise.all([loadOverview(), loadFirewall(), loadBruteForce(), loadHardening()]);
    } catch (err: any) {
      notify(err.message || 'Gagal menerapkan perbaikan', 'error');
    } finally {
      busyKey = '';
    }
  }

  // Toggle langsung pada 6 pilar Overview (hanya pilar yang mendukung toggle nyata)
  async function handleTogglePillar(pillar: ProtectionItem) {
    if (pillar.key === 'firewall') {
      await toggleFirewall();
      return;
    }
    if (pillar.key === 'waf') {
      busyKey = 'pillar:waf';
      try {
        await apiFetch('/api/waf/global', {
          method: 'POST',
          body: JSON.stringify({ enabled: !pillar.enabled }),
        });
        notify(!pillar.enabled ? 'Nginx WAF diaktifkan' : 'Nginx WAF dinonaktifkan', 'success');
        await loadOverview();
      } catch (err: any) {
        notify(err.message || 'Gagal mengubah status Nginx WAF', 'error');
      } finally {
        busyKey = '';
      }
      return;
    }
    if (pillar.key === 'bruteforce' && bruteCfg) {
      bruteCfg.enabled = !pillar.enabled;
      await handleSaveBruteConfig();
      await loadOverview();
      return;
    }
    if (pillar.key === 'fail2ban') {
      await handleControlF2B(pillar.enabled ? 'stop' : 'start');
    }
  }

  // ── Aksi Firewall (UFW) ───────────────────────────────────────────────────

  async function toggleFirewall() {
    if (!fwStatus) return;
    const enable = !fwStatus.enabled;
    const ok = await confirmDialog({
      title: enable ? 'Aktifkan Firewall UFW' : 'Nonaktifkan Firewall UFW',
      message: enable
        ? `Aktifkan UFW? FikPanel otomatis memastikan port SSH (${fwStatus.ssh_port}) dan panel (${fwStatus.panel_port}) tetap diizinkan agar Anda tidak terkunci.`
        : 'Menonaktifkan UFW akan membuka seluruh port jaringan tanpa filter firewall. Lanjutkan?',
      confirmText: enable ? 'Aktifkan UFW' : 'Nonaktifkan UFW',
      tone: enable ? 'default' : 'danger',
    });
    if (!ok) return;

    busyKey = 'fw:toggle';
    try {
      await apiFetch('/api/firewall/toggle', {
        method: 'POST',
        body: JSON.stringify({ enabled: enable }),
      });
      notify(enable ? 'Firewall UFW berhasil diaktifkan' : 'Firewall UFW dinonaktifkan', 'success');
      await Promise.all([loadFirewall(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status UFW', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleSaveDefaultPolicy(incoming: string, outgoing: string) {
    busyKey = 'fw:policy';
    try {
      await apiFetch('/api/firewall/default-policy', {
        method: 'POST',
        body: JSON.stringify({ incoming, outgoing }),
      });
      notify('Kebijakan default UFW berhasil diperbarui', 'success');
      await loadFirewall();
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan kebijakan default UFW', 'error');
    } finally {
      busyKey = '';
    }
  }

  function handleOpenAddPortModal(preset?: { port?: string; protocol?: string; direction?: string; remarks?: string }) {
    newPort = preset?.port || '';
    newProto = preset?.protocol || 'tcp';
    newAction = 'allow';
    newDirection = preset?.direction || 'in';
    newSourceIP = '0.0.0.0/0';
    newRemarks = preset?.remarks || '';
    showAddPortModal = true;
  }

  async function handleAddPortRule() {
    busyKey = 'fw:add';
    try {
      await apiFetch('/api/firewall/rules/extended', {
        method: 'POST',
        body: JSON.stringify({
          port: newPort.trim(),
          protocol: newProto,
          action: newAction,
          direction: newDirection,
          source_ip: newSourceIP.trim() || '0.0.0.0/0',
          remarks: newRemarks.trim(),
        }),
      });
      notify('Aturan firewall berhasil diterapkan', 'success');
      showAddPortModal = false;
      await Promise.all([loadFirewall(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal menambah aturan firewall', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleDeleteRule(rule: FirewallRule, force = false) {
    if (!force) {
      const ok = await confirmDialog({
        title: `Hapus Aturan Firewall #${rule.number}`,
        message: rule.protected
          ? `PERINGATAN: Port ${rule.port} (${rule.service}) adalah port kritikal akses SSH/Panel. Yakin ingin menghapus aturan #${rule.number}?`
          : `Hapus aturan ${rule.action} port ${rule.port}/${rule.protocol} (${rule.from})?`,
        confirmText: 'Hapus Aturan',
        tone: 'danger',
      });
      if (!ok) return;
    }

    busyKey = `fw:del:${rule.number}`;
    try {
      const qs = force ? '?force=true' : '';
      await apiFetch(`/api/firewall/rules/${rule.number}${qs}`, { method: 'DELETE' });
      notify(`Aturan firewall #${rule.number} berhasil dihapus`, 'success');
      await Promise.all([loadFirewall(), loadOverview()]);
    } catch (err: any) {
      const msg = err?.message || 'Gagal menghapus aturan firewall';
      if (!force && msg.includes('proteksi lockout')) {
        const forceOk = await confirmDialog({
          title: 'Konfirmasi Paksa (Risiko Lockout)',
          message: msg,
          confirmText: 'Tetap Hapus Paksa',
          tone: 'danger',
        });
        if (forceOk) {
          busyKey = '';
          await handleDeleteRule(rule, true);
          return;
        }
      } else {
        notify(msg, 'error');
      }
    } finally {
      busyKey = '';
    }
  }

  async function handleBatchDeleteRules(numbers: number[], force = false) {
    if (numbers.length === 0) return;
    if (!force) {
      const ok = await confirmDialog({
        title: `Hapus ${numbers.length} Aturan Firewall`,
        message: `Apakah Anda yakin ingin menghapus ${numbers.length} aturan UFW yang dipilih secara permanen?`,
        confirmText: `Hapus ${numbers.length} Aturan`,
        tone: 'danger',
      });
      if (!ok) return;
    }

    busyKey = 'fw:batch-del';
    try {
      await apiFetch('/api/firewall/rules/batch-delete', {
        method: 'POST',
        body: JSON.stringify({ numbers, force }),
      });
      notify(`${numbers.length} aturan firewall berhasil dihapus`, 'success');
      await Promise.all([loadFirewall(), loadOverview()]);
    } catch (err: any) {
      const msg = err?.message || 'Gagal menghapus aturan terpilih';
      if (!force && msg.includes('proteksi lockout')) {
        const forceOk = await confirmDialog({
          title: 'Konfirmasi Paksa (Risiko Lockout)',
          message: msg,
          confirmText: 'Tetap Hapus Paksa',
          tone: 'danger',
        });
        if (forceOk) {
          busyKey = '';
          await handleBatchDeleteRules(numbers, true);
          return;
        }
      } else {
        notify(msg, 'error');
      }
    } finally {
      busyKey = '';
    }
  }

  async function toggleICMP() {
    if (!fwStatus) return;
    const nextBlock = !fwStatus.block_icmp;
    busyKey = 'fw:icmp';
    try {
      await apiFetch('/api/firewall/icmp', {
        method: 'POST',
        body: JSON.stringify({ block: nextBlock }),
      });
      notify(nextBlock ? 'ICMP Ping berhasil diblokir (persisten)' : 'ICMP Ping diizinkan kembali', 'success');
      await Promise.all([loadFirewall(), loadHardening()]);
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah pengaturan ICMP', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function clearSiteLogs() {
    const ok = await confirmDialog({
      title: 'Kosongkan Log Webserver',
      message: `Kosongkan seluruh berkas .log di ${fwStatus?.site_logs_path || '/var/log/nginx'}? Operasi ini aman tanpa restart Nginx.`,
      confirmText: 'Kosongkan Log',
      tone: 'default',
    });
    if (!ok) return;

    busyKey = 'fw:clear-logs';
    try {
      await apiFetch('/api/firewall/logs/clear', { method: 'POST' });
      notify('Berkas log webserver berhasil dikosongkan', 'success');
      await loadFirewall();
    } catch (err: any) {
      notify(err.message || 'Gagal mengosongkan log web', 'error');
    } finally {
      busyKey = '';
    }
  }

  // ── Aksi SSH, Authorized Keys & Akun Sistem ───────────────────────────────

  async function handleSaveSSHConfig(force = false) {
    if (!force) {
      const ok = await confirmDialog({
        title: 'Terapkan Konfigurasi OpenSSH',
        message: `Simpan konfigurasi (Port ${sshPortInput}, PasswordAuth: ${sshPasswordAuth ? 'Ya' : 'Tidak'}, PermitRootLogin: ${sshPermitRoot})? Sistem akan memvalidasi sintaksis via sshd -t sebelum reload.`,
        confirmText: 'Validasi & Terapkan',
        tone: !sshPasswordAuth ? 'danger' : 'default',
      });
      if (!ok) return;
    }

    busyKey = 'ssh:save';
    try {
      const res = await apiFetch<{ ok: boolean; message?: string }>('/api/security/ssh', {
        method: 'POST',
        body: JSON.stringify({
          port: Number(sshPortInput),
          password_auth: sshPasswordAuth,
          pubkey_auth: sshPubkeyAuth,
          permit_root_login: sshPermitRoot,
          max_auth_tries: Number(sshMaxAuthTries) || 6,
          force,
        }),
      });
      notify(res?.message || 'Konfigurasi SSH berhasil diperbarui', 'success');
      await Promise.all([loadSSH(), loadFirewall(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan konfigurasi SSH', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleRestartSSH() {
    const ok = await confirmDialog({
      title: 'Restart Layanan SSH',
      message: 'Mulai ulang daemon OpenSSH sekarang? Sesi terminal aktif saat ini umumnya tetap tersambung.',
      confirmText: 'Restart SSH',
      tone: 'default',
    });
    if (!ok) return;

    busyKey = 'ssh:restart';
    try {
      await apiFetch('/api/security/ssh/restart', { method: 'POST' });
      notify('Layanan daemon SSH berhasil dimulai ulang', 'success');
      await Promise.all([loadSSH(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal merestart SSH', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleAddSSHKey() {
    if (!newPublicKey.trim()) return;
    busyKey = 'ssh:add-key';
    try {
      await apiFetch('/api/security/ssh/keys', {
        method: 'POST',
        body: JSON.stringify({ public_key: newPublicKey.trim() }),
      });
      notify('SSH Public Key berhasil ditambahkan ke authorized_keys', 'success');
      newPublicKey = '';
      await Promise.all([loadSSH(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal menambahkan SSH Key', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleRemoveSSHKey(key: SSHAuthorizedKey) {
    const ok = await confirmDialog({
      title: 'Hapus SSH Public Key',
      message: `Hapus kunci ${key.comment || key.fingerprint} dari authorized_keys? Perangkat dengan kunci ini tidak lagi bisa login SSH tanpa password.`,
      confirmText: 'Hapus SSH Key',
      tone: 'danger',
    });
    if (!ok) return;

    busyKey = `ssh:del-key:${key.id}`;
    try {
      await apiFetch(`/api/security/ssh/keys/${key.id}`, { method: 'DELETE' });
      notify('SSH Public Key berhasil dihapus', 'success');
      await Promise.all([loadSSH(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus SSH Key', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleToggleAccountLock(acc: SystemAccount) {
    const willLock = !acc.locked;
    const ok = await confirmDialog({
      title: willLock ? `Kunci Akun ${acc.username}` : `Buka Kunci Akun ${acc.username}`,
      message: willLock
        ? `Kunci autentikasi password untuk akun sistem "${acc.username}" (passwd -l)?`
        : `Buka kembali kunci autentikasi untuk akun sistem "${acc.username}" (passwd -u)?`,
      confirmText: willLock ? 'Kunci Akun' : 'Buka Kunci',
      tone: willLock ? 'danger' : 'default',
    });
    if (!ok) return;

    busyKey = `ssh:acc:${acc.username}`;
    try {
      await apiFetch(`/api/security/ssh/accounts/${acc.username}/toggle`, {
        method: 'POST',
        body: JSON.stringify({ lock: willLock }),
      });
      notify(`Status akun ${acc.username} berhasil diperbarui`, 'success');
      await loadSSH();
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status akun', 'error');
    } finally {
      busyKey = '';
    }
  }

  // ── Aksi Brute Force, Whitelist & Blacklist Persisten ─────────────────────

  async function handleSaveBruteConfig() {
    if (!bruteCfg) return;
    busyKey = 'bf:save';
    try {
      const res = await apiFetch<{ ok: boolean; config?: BruteForceConfig }>('/api/security/bruteforce/config', {
        method: 'POST',
        body: JSON.stringify(bruteCfg),
      });
      if (res?.config) bruteCfg = res.config;
      notify('Konfigurasi Brute Force berhasil disimpan', 'success');
      await Promise.all([loadBruteForce(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan konfigurasi Brute Force', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleWhitelistCurrentIP() {
    if (!clientIP) return;
    busyKey = 'bf:wl-self';
    try {
      await apiFetch('/api/security/bruteforce/whitelist', {
        method: 'POST',
        body: JSON.stringify({ ip: clientIP, remarks: 'IP Administrator Saat Ini' }),
      });
      notify(`IP ${clientIP} berhasil dimasukkan ke Whitelist`, 'success');
      await Promise.all([loadBruteForce(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal menambahkan IP ke Whitelist', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleAddWhitelist() {
    if (!newWhiteIP.trim()) {
      notify('Alamat IP atau CIDR wajib diisi', 'warning');
      return;
    }
    busyKey = 'bf:add-wl';
    try {
      await apiFetch('/api/security/bruteforce/whitelist', {
        method: 'POST',
        body: JSON.stringify({
          ip: newWhiteIP.trim(),
          remarks: newWhiteRemarks.trim() || 'Whitelist Operator',
        }),
      });
      notify('IP berhasil ditambahkan ke Whitelist', 'success');
      showAddWhitelistModal = false;
      newWhiteIP = '';
      newWhiteRemarks = '';
      await Promise.all([loadBruteForce(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal menambah Whitelist', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleRemoveWhitelist(rule: SecurityIPRule) {
    const ok = await confirmDialog({
      title: 'Hapus dari Whitelist',
      message: `Hapus ${rule.ip} dari daftar putih?`,
      confirmText: 'Hapus',
      tone: 'danger',
    });
    if (!ok) return;

    busyKey = `bf:del-wl:${rule.id}`;
    try {
      await apiFetch('/api/security/bruteforce/whitelist/remove', {
        method: 'POST',
        body: JSON.stringify({ id: rule.id, ip: rule.ip }),
      });
      notify(`IP ${rule.ip} dihapus dari Whitelist`, 'success');
      await Promise.all([loadBruteForce(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus Whitelist', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleAddBlacklist() {
    if (!newBlackIP.trim()) {
      notify('Alamat IP atau CIDR wajib diisi', 'warning');
      return;
    }
    busyKey = 'bf:add-bl';
    try {
      await apiFetch('/api/security/bruteforce/blacklist', {
        method: 'POST',
        body: JSON.stringify({
          ip: newBlackIP.trim(),
          reason: newBlackReason.trim() || 'Pemblokiran manual administrator',
          duration_minutes: Number(newBlackDuration) || 0,
          sync_ufw: newBlackSyncUFW,
        }),
      });
      notify(`Alamat IP ${newBlackIP.trim()} berhasil diblokir`, 'success');
      showAddBlacklistModal = false;
      newBlackIP = '';
      newBlackReason = '';
      newBlackSyncUFW = false;
      await Promise.all([loadBruteForce(), loadFirewall(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal memblokir IP', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleRemoveBlacklist(rule: SecurityIPRule) {
    const ok = await confirmDialog({
      title: 'Cabut Pemblokiran IP',
      message: `Buka kembali akses untuk ${rule.ip}?`,
      confirmText: 'Cabut Blokir',
      tone: 'default',
    });
    if (!ok) return;

    busyKey = `bf:del-bl:${rule.id}`;
    try {
      await apiFetch('/api/security/bruteforce/blacklist/remove', {
        method: 'POST',
        body: JSON.stringify({ id: rule.id, ip: rule.ip }),
      });
      notify(`Blokir IP ${rule.ip} berhasil dicabut`, 'success');
      await Promise.all([loadBruteForce(), loadFirewall(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal mencabut blokir IP', 'error');
    } finally {
      busyKey = '';
    }
  }

  // ── Aksi Fail2Ban, Kernel Sysctl, Compiler & FIM ──────────────────────────

  async function handleControlF2B(action: 'start' | 'stop' | 'restart') {
    if (action === 'stop') {
      const ok = await confirmDialog({
        title: 'Hentikan Fail2Ban',
        message: 'Hentikan daemon Fail2Ban? Pemblokiran otomatis serangan SSH/Nginx sementara tidak aktif.',
        confirmText: 'Hentikan Fail2Ban',
        tone: 'danger',
      });
      if (!ok) return;
    }

    busyKey = 'f2b:service';
    try {
      await apiFetch('/api/security/fail2ban/service', {
        method: 'POST',
        body: JSON.stringify({ action }),
      });
      notify(`Perintah ${action} Fail2Ban berhasil dijalankan`, 'success');
      await Promise.all([loadHardening(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || `Gagal menjalankan ${action} Fail2Ban`, 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleBanF2B(jail: string, ip: string) {
    busyKey = 'f2b:ban';
    try {
      await apiFetch('/api/security/fail2ban/ban', {
        method: 'POST',
        body: JSON.stringify({ jail, ip }),
      });
      notify(`IP ${ip} berhasil diblokir pada jail ${jail}`, 'success');
      await Promise.all([loadHardening(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal memblokir IP di Fail2Ban', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleUnbanF2B(jail: string, ip: string) {
    busyKey = `f2b:unban:${ip}`;
    try {
      await apiFetch('/api/security/fail2ban/unban', {
        method: 'POST',
        body: JSON.stringify({ jail, ip }),
      });
      notify(`IP ${ip} berhasil di-unban dari jail ${jail}`, 'success');
      await Promise.all([loadHardening(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal melakukan unban IP', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleToggleSysctl(key: string, enabled: boolean) {
    busyKey = `sysctl:${key}`;
    try {
      await apiFetch('/api/security/hardening/sysctl', {
        method: 'POST',
        body: JSON.stringify({ key, enabled }),
      });
      notify(`Parameter kernel ${key} berhasil diperbarui secara persisten`, 'success');
      await Promise.all([loadHardening(), loadFirewall()]);
    } catch (err: any) {
      notify(err.message || `Gagal memperbarui ${key}`, 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleToggleCompiler() {
    if (!hardening) return;
    const nextBlock = !hardening.compiler_blocked;
    busyKey = 'hardening:compiler';
    try {
      await apiFetch('/api/security/hardening/compiler', {
        method: 'POST',
        body: JSON.stringify({ block: nextBlock }),
      });
      notify(nextBlock ? 'Akses compiler dibatasi khusus root (0700)' : 'Akses compiler dikembalikan (0755)', 'success');
      await loadHardening();
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah izin compiler', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleCreateFIMBaseline() {
    busyKey = 'fim:baseline';
    try {
      const res = await apiFetch<{ ok: boolean; fim: FIMCheckResult; message?: string }>(
        '/api/security/hardening/fim/baseline',
        { method: 'POST' }
      );
      if (res?.fim) fimResult = res.fim;
      notify(res?.message || 'Baseline SHA-256 FIM berhasil dibuat', 'success');
      await Promise.all([loadHardening(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal membuat baseline FIM', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleCheckFIM() {
    busyKey = 'fim:check';
    try {
      const res = await apiFetch<{ ok: boolean; fim: FIMCheckResult }>('/api/security/hardening/fim');
      if (res?.fim) {
        fimResult = res.fim;
        const drift = res.fim.modified_count + res.fim.missing_count;
        notify(
          drift > 0
            ? `Peringatan FIM: ${res.fim.modified_count} berkas berubah, ${res.fim.missing_count} hilang`
            : `Integritas valid: seluruh ${res.fim.total_files} berkas cocok dengan baseline`,
          drift > 0 ? 'warning' : 'success'
        );
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memverifikasi integritas FIM', 'error');
    } finally {
      busyKey = '';
    }
  }

  async function handleDisableFIM() {
    const ok = await confirmDialog({
      title: 'Nonaktifkan File Integrity Monitor',
      message: 'Hapus snapshot baseline SHA-256 tersimpan dan hentikan pemantauan FIM?',
      confirmText: 'Nonaktifkan FIM',
      tone: 'danger',
    });
    if (!ok) return;

    busyKey = 'fim:disable';
    try {
      await apiFetch('/api/security/hardening/fim', { method: 'DELETE' });
      notify('File Integrity Monitor dinonaktifkan', 'success');
      await Promise.all([loadHardening(), loadOverview()]);
    } catch (err: any) {
      notify(err.message || 'Gagal menonaktifkan FIM', 'error');
    } finally {
      busyKey = '';
    }
  }

  onMount(() => {
    refreshAll();
  });
</script>

<div class="pg-security">
  <!-- Header Halaman Standar FikPanel -->
  <PageHeader
    title="Security Center"
    subtitle="Pusat kendali firewall UFW, hardening SSH, proteksi brute force persisten, Fail2Ban, kernel sysctl, dan audit kerentanan host."
    icon="ri-shield-keyhole-line"
  >
    {#snippet actions()}
      <button
        type="button"
        class="btn btn--secondary btn--sm"
        disabled={loading}
        onclick={refreshAll}
      >
        <i class="ri-refresh-line" aria-hidden="true"></i>
        <span>Segarkan</span>
      </button>
      <button
        type="button"
        class="btn btn--primary btn--sm"
        disabled={isScanning}
        onclick={runSecurityScan}
      >
        <i class={isScanning ? 'ri-loader-4-line spin' : 'ri-radar-line'} aria-hidden="true"></i>
        <span>{isScanning ? 'Memindai...' : 'Jalankan Audit'}</span>
      </button>
    {/snippet}
  </PageHeader>

  <!-- Navigasi 5 Tab Terstruktur -->
  <SecurityTabs
    {activeTab}
    {issuesCount}
    {rulesCount}
    blacklistCount={blacklist.length}
    {f2bBannedCount}
    onSelect={(t) => (activeTab = t)}
  />

  {#if loading && !protection}
    <div class="sec-skeleton-grid" aria-busy="true" aria-label="Memuat data keamanan">
      <div class="skeleton sec-skeleton-card"></div>
      <div class="skeleton sec-skeleton-card"></div>
    </div>
  {:else}
    <div class="sec-tab-pane">
      {#if activeTab === 'overview'}
        <OverviewTab
          {protection}
          {scanReport}
          {isScanning}
          {busyKey}
          onRunScan={runSecurityScan}
          onTogglePillar={handleTogglePillar}
          onNavigateTarget={handleNavigateTarget}
          onOpenScanCategory={handleOpenScanCategory}
          onApplyQuickFix={handleApplyQuickFix}
        />
      {:else if activeTab === 'firewall'}
        <FirewallTab
          {fwStatus}
          {fwSubTab}
          bind:fwSearch
          bind:selectedRuleNums
          {busyKey}
          onSelectSubTab={(t) => (fwSubTab = t)}
          onRefresh={loadFirewall}
          onToggleFirewall={toggleFirewall}
          onToggleICMP={toggleICMP}
          onClearSiteLogs={clearSiteLogs}
          onSaveDefaultPolicy={handleSaveDefaultPolicy}
          onOpenAddModal={handleOpenAddPortModal}
          onDeleteRule={(r) => handleDeleteRule(r, false)}
          onBatchDeleteRules={(nums) => handleBatchDeleteRules(nums, false)}
        />
      {:else if activeTab === 'ssh'}
        <SshTab
          {sshStats}
          {sshKeys}
          {accounts}
          {sshSubTab}
          bind:sshPortInput
          bind:sshPasswordAuth
          bind:sshPubkeyAuth
          bind:sshPermitRoot
          bind:sshMaxAuthTries
          bind:newPublicKey
          {busyKey}
          onSelectSubTab={(t) => (sshSubTab = t)}
          onRestartSSH={handleRestartSSH}
          onSaveSSHConfig={() => handleSaveSSHConfig(false)}
          onAddSSHKey={handleAddSSHKey}
          onRemoveSSHKey={handleRemoveSSHKey}
          onToggleAccountLock={handleToggleAccountLock}
        />
      {:else if activeTab === 'bruteforce'}
        <BruteForceTab
          {clientIP}
          {isClientWhitelisted}
          bind:bruteCfg
          {bruteSubTab}
          {whitelist}
          {blacklist}
          {loginEvents}
          {busyKey}
          onSelectSubTab={(t) => (bruteSubTab = t)}
          onWhitelistCurrentIP={handleWhitelistCurrentIP}
          onSaveBruteConfig={handleSaveBruteConfig}
          onOpenAddWhitelist={() => (showAddWhitelistModal = true)}
          onRemoveWhitelist={handleRemoveWhitelist}
          onOpenAddBlacklist={() => (showAddBlacklistModal = true)}
          onRemoveBlacklist={handleRemoveBlacklist}
        />
      {:else if activeTab === 'hardening'}
        <HardeningTab
          {hardeningSubTab}
          {scanReport}
          {scanCategoryFilter}
          {isScanning}
          {f2bStatus}
          {hardening}
          {fimResult}
          {busyKey}
          onSelectSubTab={(t) => (hardeningSubTab = t)}
          onSelectScanCategory={(c) => (scanCategoryFilter = c)}
          onRunScan={runSecurityScan}
          onApplyQuickFix={handleApplyQuickFix}
          onNavigateTarget={handleNavigateTarget}
          onControlF2B={handleControlF2B}
          onBanF2B={handleBanF2B}
          onUnbanF2B={handleUnbanF2B}
          onToggleSysctl={handleToggleSysctl}
          onToggleCompiler={handleToggleCompiler}
          onCreateFIMBaseline={handleCreateFIMBaseline}
          onCheckFIM={handleCheckFIM}
          onDisableFIM={handleDisableFIM}
        />
      {/if}
    </div>
  {/if}

  <!-- Modal Tambah Aturan Firewall -->
  <AddPortModal
    open={showAddPortModal}
    {clientIP}
    bind:port={newPort}
    bind:protocol={newProto}
    bind:action={newAction}
    bind:direction={newDirection}
    bind:sourceIP={newSourceIP}
    bind:remarks={newRemarks}
    busy={busyKey === 'fw:add'}
    onClose={() => (showAddPortModal = false)}
    onSubmit={handleAddPortRule}
  />

  <!-- Modal Tambah Whitelist IP -->
  <AddWhitelistModal
    open={showAddWhitelistModal}
    {clientIP}
    bind:newWhiteIP
    bind:newWhiteRemarks
    busy={busyKey === 'bf:add-wl'}
    onClose={() => (showAddWhitelistModal = false)}
    onSubmit={handleAddWhitelist}
  />

  <!-- Modal Tambah Blacklist IP -->
  <AddBlacklistModal
    open={showAddBlacklistModal}
    bind:newBlackIP
    bind:newBlackReason
    bind:newBlackDuration
    bind:newBlackSyncUFW
    busy={busyKey === 'bf:add-bl'}
    onClose={() => (showAddBlacklistModal = false)}
    onSubmit={handleAddBlacklist}
  />
</div>
