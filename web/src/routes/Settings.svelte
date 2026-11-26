<script lang="ts">
  import { onMount } from 'svelte';
  import QRCode from 'qrcode';
  import { apiFetch, formatBytes } from '../lib/api';
  import { copyText } from '../lib/utils/browser';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import {
    appearanceState,
    applyAppearance,
    DEFAULT_APPEARANCE,
    type AccentPreset,
    type DensityStyle,
    type EnvBadge,
    type RadiusStyle,
    type SurfaceStyle,
    type ThemeMode,
  } from '../lib/stores/theme.svelte';
  import { dateLocale, i18nState, setLocale, t, type Locale } from '../lib/stores/i18n.svelte';
  import type { AuditLog, DNSProviderMeta, OverviewData, SystemWebhook } from '../lib/types';
  import Modal from '../lib/components/ui/Modal.svelte';
  import {
    getWebAuthnEnvironment,
    registerPasskeyFlow,
    suggestPasskeyDeviceName,
    type PasskeyItem,
  } from '../lib/utils/webauthn';
  import '../styles/pages/settings.css';

  interface Props {
    overview: OverviewData | null;
    onNotify: (msg: string, type?: 'success' | 'error' | 'warning' | 'info') => void;
  }

  let { overview, onNotify }: Props = $props();

  type SettingsTabId = 'appearance' | 'security' | 'network' | 'system' | 'webhooks' | 'mcp' | 'backup';
  let activeTab = $state<SettingsTabId>('appearance');

  // ── State Preferensi Panel Terpadu (disimpan di SQLite settings) ──
  interface FullPreferences {
    theme_mode: ThemeMode;
    accent_preset: AccentPreset;
    accent_custom: string;
    surface_style: SurfaceStyle;
    density: DensityStyle;
    radius_style: RadiusStyle;
    brand_name: string;
    env_badge: EnvBadge;
    language: Locale;
    session_hours: number;
    ip_allowlist: string[];
    alert_cpu_pct: number;
    alert_ram_pct: number;
    alert_disk_pct: number;
    alert_ssl_days: number;
    alert_cooldown_min: number;
  }

  let prefs = $state<FullPreferences>({
    ...DEFAULT_APPEARANCE,
    theme_mode: appearanceState.theme_mode,
    accent_preset: appearanceState.accent_preset,
    accent_custom: appearanceState.accent_custom,
    surface_style: appearanceState.surface_style,
    density: appearanceState.density,
    radius_style: appearanceState.radius_style,
    brand_name: appearanceState.brand_name,
    env_badge: appearanceState.env_badge,
    language: i18nState.lang,
    session_hours: 24,
    ip_allowlist: [],
    alert_cpu_pct: 85,
    alert_ram_pct: 85,
    alert_disk_pct: 90,
    alert_ssl_days: 14,
    alert_cooldown_min: 15,
  });
  let ipAllowlistText = $state('');
  let clientIP = $state('');
  let savingPrefs = $state(false);

  // Daftar preset palet warna aksen kurasi
  const ACCENT_PRESETS: Array<{ id: AccentPreset; label: string; swatchClass: string }> = [
    { id: 'lava', label: 'Lava', swatchClass: 'swatch--lava' },
    { id: 'ocean', label: 'Ocean', swatchClass: 'swatch--ocean' },
    { id: 'emerald', label: 'Emerald', swatchClass: 'swatch--emerald' },
    { id: 'violet', label: 'Violet', swatchClass: 'swatch--violet' },
    { id: 'crimson', label: 'Crimson', swatchClass: 'swatch--crimson' },
    { id: 'amber', label: 'Amber', swatchClass: 'swatch--amber' },
    { id: 'slate', label: 'Slate', swatchClass: 'swatch--slate' },
  ];

  // ── Tab B: Kata Sandi, Multi-Akun, 2FA TOTP, Sesi Aktif, & Audit Log ──
  interface PanelUserItem {
    id: number;
    username: string;
    role: string;
    totp_enabled: boolean;
    last_login_at: number;
    is_current: boolean;
  }
  let panelUsers = $state<PanelUserItem[]>([]);
  let currentUserId = $state(0);
  let currentUsername = $state('');
  let targetUserId = $state(0);
  let isChangingOtherUser = $derived(targetUserId > 0 && currentUserId > 0 && targetUserId !== currentUserId);

  let oldPassword = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let showPassFields = $state(false);
  let updatingPass = $state(false);

  let totpEnabled = $state(false);
  let totpRecoveryRemaining = $state(0);
  let totpRecoveryPct = $derived(Math.min(100, Math.round((totpRecoveryRemaining / 8) * 100)));
  let totpSetupSecret = $state('');
  let totpQrDataUrl = $state('');
  let totpVerifyCode = $state('');
  let totpRecoveryCodes = $state<string[]>([]);
  let totpModalMode = $state<null | 'disable' | 'regen'>(null);
  let totpModalSecret = $state('');
  let totpBusy = $state(false);

  interface ActiveSession {
    id: string;
    ip: string;
    user_agent: string;
    created_at: number;
    expires_at: number;
    is_current: boolean;
  }
  let activeSessions = $state<ActiveSession[]>([]);
  let loadingSessions = $state(false);
  let auditLogs = $state<AuditLog[]>([]);

  // State Passkeys / WebAuthn
  const webauthnEnv = getWebAuthnEnvironment();
  let passkeys = $state<PasskeyItem[]>([]);
  let showPasskeyModal = $state(false);
  let passkeyNameInput = $state('');
  let passkeyBusy = $state(false);

  // Indikator kekuatan kata sandi live
  let passwordStrength = $derived.by(() => {
    const p = newPassword;
    if (!p) return { score: 0, label: '', tone: '' };
    let score = 0;
    if (p.length >= 8) score++;
    if (p.length >= 12) score++;
    if (/[A-Z]/.test(p) && /[a-z]/.test(p)) score++;
    if (/\d/.test(p) && /[^A-Za-z0-9]/.test(p)) score++;
    if (score <= 1) return { score: 25, label: t('settings.security.strength_weak'), tone: 'danger' };
    if (score <= 2) return { score: 55, label: t('settings.security.strength_fair'), tone: 'warning' };
    if (score === 3) return { score: 80, label: t('settings.security.strength_strong'), tone: 'info' };
    return { score: 100, label: t('settings.security.strength_very_strong'), tone: 'success' };
  });

  // ── Tab C: Panel HTTPS / TLS ──
  interface TLSStatus {
    enabled: boolean;
    self_signed: boolean;
    persistent: boolean;
    subject: string;
    issuer: string;
    dns_names: string[];
    not_before: number;
    not_after: number;
    days_remaining: number;
    fingerprint: string;
    cert_path: string;
    key_path: string;
  }
  let tlsInfo = $state<TLSStatus | null>(null);
  let customCertPEM = $state('');
  let customKeyPEM = $state('');
  let savingTLS = $state(false);
  let dnsProviders = $state<DNSProviderMeta[]>([]);
  let acmeDomain = $state('');
  let acmeProvider = $state('cloudflare');
  let acmeIncludeWildcard = $state(false);
  let acmeCreds = $state<Record<string, string>>({});
  let acmeEditCreds = $state(false);
  let issuingACME = $state(false);
  let activeAcmeProvider = $derived(dnsProviders.find((p) => p.id === acmeProvider) ?? null);

  // ── Tab D: Sistem, Hostname, Timezone, & Update Center ──
  interface SystemEnvInfo {
    hostname: string;
    os_pretty: string;
    kernel: string;
    go_version: string;
    arch: string;
    platform: string;
    cpu_cores: number;
    uptime_sec: number;
    version: string;
    db_path: string;
    db_size_bytes: number;
    backup_dir: string;
    timezone: string;
    tz_abbr: string;
    client_ip: string;
  }
  let sysInfo = $state<SystemEnvInfo | null>(null);
  let timezones = $state<string[]>([]);
  let hostnameInput = $state('');
  let timezoneInput = $state('');
  let savingHost = $state(false);
  let savingTz = $state(false);

  interface UpdaterStatusResp {
    current_version: string;
    platform: string;
    auto_update: {
      installed: boolean;
      enabled: boolean;
      active: boolean;
      disabled_by_admin: boolean;
      next_run: string;
      last_run: string;
    };
    last_status: {
      run_id?: string;
      state?: string;
      phase?: string;
      from?: string;
      to?: string;
      message?: string;
      started_at?: number;
      finished_at?: number;
    };
    log_tail: string;
  }
  let updaterInfo = $state<UpdaterStatusResp | null>(null);
  let checkReleaseResult = $state<{
    checked: boolean;
    ok: boolean;
    latest_tag?: string;
    update_available?: boolean;
    release_url?: string;
    error?: string;
  } | null>(null);
  let checkingUpdate = $state(false);
  let runningUpdate = $state(false);
  let togglingAutoUpdate = $state(false);
  let showUpdateLog = $state(false);

  // ── Tab E: Webhook Pintar ──
  let webhooks = $state<SystemWebhook[]>([]);
  let showWebhookForm = $state(false);
  let editingWebhookId = $state<number | null>(null);
  let hookName = $state('');
  let hookChannel = $state('telegram');
  let hookUrl = $state('');
  let hookTgToken = $state('');
  let hookTgChatId = $state('');
  let hookCpu = $state(true);
  let hookRam = $state(true);
  let hookBackup = $state(true);
  let hookSsl = $state(true);
  let savingWebhook = $state(false);
  let testingWebhookId = $state<number | null>(null);

  // ── Tab F: Migrasi Export / Import JSON ──
  let importingConfig = $state(false);
  let importFileEl = $state<HTMLInputElement | null>(null);

  // ── Tab G: MCP Server (Fokus Google Antigravity) ──
  interface MCPTokenItem {
    id: number;
    name: string;
    token_prefix: string;
    scope: 'readonly' | 'operator' | string;
    enabled: boolean;
    call_count: number;
    last_used_at: number;
    last_used_ip: string;
    created_at: number;
  }
  interface MCPToolItem {
    name: string;
    scope: 'readonly' | 'operator' | string;
    title: string;
    description: string;
  }
  interface MCPPromptItem {
    name: string;
    title: string;
    description: string;
  }
  let mcpTokens = $state<MCPTokenItem[]>([]);
  let mcpTools = $state<MCPToolItem[]>([]);
  let mcpPrompts = $state<MCPPromptItem[]>([]);
  let mcpToolFilter = $state<'all' | 'readonly' | 'operator'>('all');
  let mcpToolQuery = $state('');
  let mcpNewName = $state('Google Antigravity');
  let mcpNewScope = $state<'readonly' | 'operator'>('readonly');
  let mcpCreating = $state(false);
  let mcpPlainToken = $state('');
  let mcpBusyId = $state<number | null>(null);

  const filteredMcpTools = $derived.by(() => {
    const q = mcpToolQuery.trim().toLowerCase();
    return mcpTools.filter((tl) => {
      if (mcpToolFilter !== 'all' && tl.scope !== mcpToolFilter) return false;
      if (!q) return true;
      return (
        tl.name.toLowerCase().includes(q) ||
        tl.title.toLowerCase().includes(q) ||
        tl.description.toLowerCase().includes(q)
      );
    });
  });

  const mcpEndpointUrl = $derived(
    typeof window !== 'undefined' ? `${window.location.origin}/mcp` : 'https://panel.domain.com/mcp'
  );

  const antigravityConfigJson = $derived(
    JSON.stringify(
      {
        mcpServers: {
          fikpanel: {
            url: mcpEndpointUrl,
            headers: {
              Authorization: `Bearer ${mcpPlainToken || '<PASTE_FIKPANEL_MCP_TOKEN>'}`,
            },
          },
        },
      },
      null,
      2
    )
  );

  // ── Pemuatan Data Awal ──
  async function loadPreferences() {
    try {
      const res = await apiFetch<{ ok: boolean; preferences: FullPreferences; client_ip?: string }>('/api/settings/preferences');
      if (res?.preferences) {
        prefs = { ...prefs, ...res.preferences };
        ipAllowlistText = (res.preferences.ip_allowlist || []).join('\n');
        applyAppearance({
          theme_mode: prefs.theme_mode,
          accent_preset: prefs.accent_preset,
          accent_custom: prefs.accent_custom,
          surface_style: prefs.surface_style,
          density: prefs.density,
          radius_style: prefs.radius_style,
          brand_name: prefs.brand_name,
          env_badge: prefs.env_badge,
        });
        setLocale(prefs.language);
      }
      if (res?.client_ip) clientIP = res.client_ip;
    } catch {
      // Gunakan state cache lokal bila gagal memuat preferensi
    }
  }

  async function loadSecurityData() {
    loadingSessions = true;
    try {
      const [twofa, sess, aud, usr, pks] = await Promise.all([
        apiFetch<{ enabled: boolean; recovery_remaining: number }>('/api/auth/2fa/status'),
        apiFetch<{ sessions: ActiveSession[] }>('/api/auth/sessions'),
        apiFetch<{ logs: AuditLog[] }>('/api/audit'),
        apiFetch<{ current_user: string; current_id: number; users: PanelUserItem[] }>('/api/auth/users'),
        apiFetch<{ passkeys: PasskeyItem[] }>('/api/auth/passkeys'),
      ]);
      totpEnabled = Boolean(twofa?.enabled);
      totpRecoveryRemaining = twofa?.recovery_remaining || 0;
      activeSessions = sess?.sessions || [];
      auditLogs = (aud?.logs || []).slice(0, 20);
      passkeys = pks?.passkeys || [];
      if (usr) {
        panelUsers = usr.users || [];
        currentUserId = usr.current_id || 0;
        currentUsername = usr.current_user || '';
        if (targetUserId === 0 && currentUserId > 0) {
          targetUserId = currentUserId;
        }
      }
    } catch {
      // Abaikan kegagalan sebagian data keamanan
    } finally {
      loadingSessions = false;
    }
  }

  async function loadTLS() {
    try {
      const [res, provRes] = await Promise.all([
        apiFetch<{ tls: TLSStatus; acme_domain?: string; acme_provider?: string; client_ip?: string }>('/api/settings/tls'),
        apiFetch<{ ok: boolean; providers: DNSProviderMeta[] }>('/api/ssl/dns-providers').catch(() => ({ ok: false, providers: [] })),
      ]);
      tlsInfo = res?.tls || null;
      if (res?.client_ip) clientIP = res.client_ip;
      dnsProviders = provRes?.providers || [];
      if (res?.acme_domain) {
        acmeDomain = res.acme_domain;
      } else if (!acmeDomain && typeof window !== 'undefined') {
        const host = window.location.hostname;
        // Isi otomatis domain FQDN panel bila bukan IP atau localhost
        if (host && host !== 'localhost' && host.includes('.') && !/^\d+(\.\d+){3}$/.test(host) && !host.includes(':')) {
          acmeDomain = host;
        }
      }
      if (res?.acme_provider) {
        acmeProvider = res.acme_provider;
      }
    } catch {
      // Abaikan kesalahan muat TLS
    }
  }

  async function loadSystemAndUpdater() {
    try {
      const [sysRes, updRes] = await Promise.all([
        apiFetch<{ system: SystemEnvInfo; timezones: string[] }>('/api/settings/system'),
        apiFetch<UpdaterStatusResp>('/api/settings/updater/status'),
      ]);
      if (sysRes?.system) {
        sysInfo = sysRes.system;
        hostnameInput = sysRes.system.hostname || '';
        timezoneInput = sysRes.system.timezone || 'UTC';
        if (sysRes.system.client_ip) clientIP = sysRes.system.client_ip;
      }
      timezones = sysRes?.timezones || ['UTC', 'Asia/Jakarta'];
      updaterInfo = updRes || null;
    } catch {
      // Abaikan kesalahan muat info sistem
    }
  }

  async function loadWebhooks() {
    try {
      const res = await apiFetch<{ ok: boolean; webhooks: SystemWebhook[] }>('/api/webhooks');
      webhooks = res?.webhooks || [];
    } catch {
      // Abaikan kesalahan pemuatan webhook
    }
  }

  async function loadMCP() {
    try {
      const res = await apiFetch<{
        ok: boolean;
        tokens: MCPTokenItem[];
        tools: MCPToolItem[];
        prompts?: MCPPromptItem[];
      }>('/api/mcp/tokens');
      mcpTokens = res?.tokens || [];
      mcpTools = res?.tools || [];
      mcpPrompts = res?.prompts || [];
    } catch {
      // Abaikan kesalahan pemuatan token MCP
    }
  }

  onMount(() => {
    loadPreferences();
    loadSecurityData();
    loadTLS();
    loadSystemAndUpdater();
    loadWebhooks();
    loadMCP();
  });

  // ── Aksi Tab A + G: Live Preview & Simpan Tampilan / Bahasa ──
  function previewAppearancePatch(patch: Partial<FullPreferences>) {
    prefs = { ...prefs, ...patch };
    applyAppearance({
      theme_mode: prefs.theme_mode,
      accent_preset: prefs.accent_preset,
      accent_custom: prefs.accent_custom,
      surface_style: prefs.surface_style,
      density: prefs.density,
      radius_style: prefs.radius_style,
      brand_name: prefs.brand_name,
      env_badge: prefs.env_badge,
    });
    if (patch.language) {
      setLocale(patch.language);
    }
  }

  async function saveAllPreferences(customMessage?: string) {
    savingPrefs = true;
    try {
      const allowList = ipAllowlistText
        .split(/\r?\n|,/)
        .map((s) => s.trim())
        .filter(Boolean);
      const payload: FullPreferences = {
        ...prefs,
        ip_allowlist: allowList,
      };
      const res = await apiFetch<{ ok: boolean; preferences: FullPreferences; message?: string }>('/api/settings/preferences', {
        method: 'PUT',
        body: JSON.stringify(payload),
      });
      if (res?.preferences) {
        prefs = res.preferences;
        ipAllowlistText = (res.preferences.ip_allowlist || []).join('\n');
        applyAppearance({
          theme_mode: prefs.theme_mode,
          accent_preset: prefs.accent_preset,
          accent_custom: prefs.accent_custom,
          surface_style: prefs.surface_style,
          density: prefs.density,
          radius_style: prefs.radius_style,
          brand_name: prefs.brand_name,
          env_badge: prefs.env_badge,
        });
        setLocale(prefs.language);
      }
      onNotify(customMessage || res?.message || 'Pengaturan disimpan', 'success');
    } catch (err: any) {
      onNotify(err?.message || 'Gagal menyimpan pengaturan', 'error');
    } finally {
      savingPrefs = false;
    }
  }

  async function restoreDefaultAppearance() {
    previewAppearancePatch({ ...DEFAULT_APPEARANCE, language: 'id' });
    await saveAllPreferences(i18nState.lang === 'en' ? 'Default appearance restored' : 'Tampilan dikembalikan ke default');
  }

  // ── Aksi Tab B: Kata Sandi, 2FA, & Sesi ──
  async function handlePasswordChange(e: Event) {
    e.preventDefault();
    if (newPassword !== confirmPassword) {
      onNotify('Konfirmasi kata sandi tidak cocok!', 'warning');
      return;
    }
    if (newPassword.length < 8) {
      onNotify('Kata sandi baru minimal 8 karakter!', 'warning');
      return;
    }
    updatingPass = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/auth/password', {
        method: 'POST',
        body: JSON.stringify({
          old_password: oldPassword,
          new_password: newPassword,
          target_user_id: targetUserId || undefined,
        }),
      });
      onNotify(res.message || 'Kata sandi berhasil diperbarui', 'success');
      oldPassword = '';
      newPassword = '';
      confirmPassword = '';
      await loadSecurityData();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal memperbarui kata sandi', 'error');
    } finally {
      updatingPass = false;
    }
  }

  async function start2FASetup() {
    totpBusy = true;
    totpRecoveryCodes = [];
    try {
      const res = await apiFetch<{ secret: string; otpauth_uri: string }>('/api/auth/2fa/setup', { method: 'POST' });
      totpSetupSecret = res.secret;
      totpVerifyCode = '';
      totpQrDataUrl = await QRCode.toDataURL(res.otpauth_uri, { width: 196, margin: 2 });
    } catch (err: any) {
      onNotify(err?.message || 'Gagal memulai setup 2FA', 'error');
    } finally {
      totpBusy = false;
    }
  }

  async function confirmEnable2FA() {
    if (!totpVerifyCode.trim()) return;
    totpBusy = true;
    try {
      const res = await apiFetch<{ ok: boolean; recovery_codes: string[]; message: string }>('/api/auth/2fa/enable', {
        method: 'POST',
        body: JSON.stringify({ code: totpVerifyCode.trim() }),
      });
      totpEnabled = true;
      totpSetupSecret = '';
      totpQrDataUrl = '';
      totpVerifyCode = '';
      totpRecoveryCodes = res.recovery_codes || [];
      totpRecoveryRemaining = totpRecoveryCodes.length;
      onNotify(res.message || '2FA berhasil diaktifkan', 'success');
      await loadSecurityData();
    } catch (err: any) {
      onNotify(err?.message || 'Verifikasi kode 2FA gagal', 'error');
    } finally {
      totpBusy = false;
    }
  }

  function openTotpModal(mode: 'disable' | 'regen') {
    totpModalSecret = '';
    totpModalMode = mode;
  }

  function closeTotpModal() {
    if (totpBusy) return;
    totpModalMode = null;
    totpModalSecret = '';
  }

  async function submitTotpModal(e: Event) {
    e.preventDefault();
    const secret = totpModalSecret.trim();
    if (!secret) {
      onNotify(t('settings.security.totp_verify_placeholder'), 'warning');
      return;
    }

    const isSixDigits = /^\d{6}$/.test(secret);
    const body = isSixDigits ? { code: secret } : { password: secret };
    const mode = totpModalMode;

    totpBusy = true;
    try {
      if (mode === 'disable') {
        const res = await apiFetch<{ ok: boolean; message: string }>('/api/auth/2fa/disable', {
          method: 'POST',
          body: JSON.stringify(body),
        });
        totpEnabled = false;
        totpRecoveryCodes = [];
        totpRecoveryRemaining = 0;
        totpModalMode = null;
        totpModalSecret = '';
        onNotify(res.message || '2FA dinonaktifkan', 'success');
      } else if (mode === 'regen') {
        const res = await apiFetch<{ ok: boolean; recovery_codes: string[]; recovery_remaining: number; message: string }>(
          '/api/auth/2fa/recovery-regenerate',
          {
            method: 'POST',
            body: JSON.stringify(body),
          },
        );
        totpRecoveryCodes = res.recovery_codes || [];
        totpRecoveryRemaining = res.recovery_remaining ?? totpRecoveryCodes.length;
        totpModalMode = null;
        totpModalSecret = '';
        onNotify(res.message || 'Kode pemulihan baru dibuat', 'success');
      }
      await loadSecurityData();
    } catch (err: any) {
      onNotify(err?.message || 'Verifikasi 2FA gagal', 'error');
    } finally {
      totpBusy = false;
    }
  }

  async function revokeSingleSession(sess: ActiveSession) {
    const ok = await confirmDialog({
      title: 'Cabut Sesi Perangkat?',
      message: `Perangkat dengan IP ${sess.ip || '-'} akan langsung dikeluarkan dari panel.`,
      confirmText: 'Cabut Sesi',
      tone: 'danger',
    });
    if (!ok) return;
    try {
      await apiFetch(`/api/auth/sessions/${encodeURIComponent(sess.id)}`, { method: 'DELETE' });
      onNotify('Sesi berhasil dicabut', 'success');
      await loadSecurityData();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal mencabut sesi', 'error');
    }
  }

  async function revokeOtherSessions() {
    const ok = await confirmDialog({
      title: 'Keluarkan Semua Perangkat Lain?',
      message: 'Seluruh sesi login selain browser yang sedang Anda gunakan akan dicabut seketika.',
      confirmText: 'Keluarkan Semua',
      tone: 'danger',
    });
    if (!ok) return;
    try {
      const res = await apiFetch<{ message: string }>('/api/auth/sessions/revoke-others', { method: 'POST' });
      onNotify(res.message || 'Sesi lain telah dikeluarkan', 'success');
      await loadSecurityData();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal mengeluarkan sesi lain', 'error');
    }
  }

  // ── Aksi Tab B (Passkeys / WebAuthn) ──
  function openPasskeyRegisterModal() {
    passkeyNameInput = suggestPasskeyDeviceName();
    showPasskeyModal = true;
  }

  async function handleRegisterPasskey(e: Event) {
    e.preventDefault();
    if (passkeyBusy) return;
    passkeyBusy = true;
    try {
      const res = await registerPasskeyFlow(passkeyNameInput);
      showPasskeyModal = false;
      passkeyNameInput = '';
      onNotify(res.message || 'Passkey berhasil didaftarkan', 'success');
      await loadSecurityData();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal mendaftarkan Passkey', 'error');
    } finally {
      passkeyBusy = false;
    }
  }

  async function handleDeletePasskey(pk: PasskeyItem) {
    const ok = await confirmDialog({
      title: 'Hapus Perangkat Passkey?',
      message: `Perangkat "${pk.name}" tidak lagi bisa digunakan untuk login tanpa kata sandi.`,
      confirmText: t('common.delete'),
      tone: 'danger',
    });
    if (!ok) return;
    try {
      const res = await apiFetch<{ message: string }>(`/api/auth/passkeys/${pk.id}`, { method: 'DELETE' });
      onNotify(res.message || 'Passkey dihapus', 'success');
      await loadSecurityData();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal menghapus Passkey', 'error');
    }
  }

  // ── Aksi Tab C: TLS Custom & IP Allowlist ──
  async function issuePanelACMETLS() {
    const dom = acmeDomain.trim();
    if (!dom) {
      onNotify('Domain FQDN panel wajib diisi (contoh: panel.domain.com)', 'warning');
      return;
    }
    const credsPayload: Record<string, string> = {};
    for (const [k, v] of Object.entries(acmeCreds)) {
      if (v.trim()) credsPayload[k] = v.trim();
    }
    issuingACME = true;
    try {
      const res = await apiFetch<{ tls: TLSStatus; message: string }>('/api/settings/tls/acme', {
        method: 'POST',
        body: JSON.stringify({
          domain: dom,
          dns_provider: acmeProvider,
          include_wildcard: acmeIncludeWildcard,
          credentials: Object.keys(credsPayload).length > 0 ? credsPayload : undefined,
        }),
      });
      tlsInfo = res.tls;
      acmeCreds = {};
      acmeEditCreds = false;
      onNotify(res.message || 'Auto-SSL Panel berhasil diterbitkan & aktif', 'success');
      await loadTLS();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal menerbitkan Auto-SSL Panel via DNS-01', 'error');
    } finally {
      issuingACME = false;
    }
  }

  async function installCustomTLS() {
    if (!customCertPEM.trim() || !customKeyPEM.trim()) {
      onNotify('Sertifikat PEM dan Private Key PEM wajib diisi', 'warning');
      return;
    }
    savingTLS = true;
    try {
      const res = await apiFetch<{ tls: TLSStatus; message: string }>('/api/settings/tls/custom', {
        method: 'POST',
        body: JSON.stringify({ cert_pem: customCertPEM, key_pem: customKeyPEM }),
      });
      tlsInfo = res.tls;
      customCertPEM = '';
      customKeyPEM = '';
      onNotify(res.message || 'Sertifikat TLS dipasang', 'success');
    } catch (err: any) {
      onNotify(err?.message || 'Gagal memasang sertifikat TLS', 'error');
    } finally {
      savingTLS = false;
    }
  }

  async function resetSelfSignedTLS() {
    const ok = await confirmDialog({
      title: 'Buat Ulang Sertifikat Self-Signed?',
      message: 'Sertifikat HTTPS panel saat ini akan diganti dengan sertifikat self-signed ECDSA P-256 baru.',
      confirmText: 'Buat Ulang',
      tone: 'danger',
    });
    if (!ok) return;
    savingTLS = true;
    try {
      const res = await apiFetch<{ tls: TLSStatus; message: string }>('/api/settings/tls/self-signed', { method: 'POST' });
      tlsInfo = res.tls;
      onNotify(res.message || 'Sertifikat self-signed dibuat ulang', 'success');
    } catch (err: any) {
      onNotify(err?.message || 'Gagal membuat sertifikat self-signed', 'error');
    } finally {
      savingTLS = false;
    }
  }

  function appendMyIPToAllowlist() {
    if (!clientIP) return;
    const lines = ipAllowlistText
      .split(/\r?\n/)
      .map((l) => l.trim())
      .filter(Boolean);
    if (!lines.includes(clientIP)) {
      lines.push(clientIP);
      ipAllowlistText = lines.join('\n');
    }
  }

  // ── Aksi Tab D: Hostname, Timezone, Update Center, Restart ──
  async function saveHostname() {
    if (!hostnameInput.trim()) return;
    savingHost = true;
    try {
      const res = await apiFetch<{ message: string }>('/api/settings/system/hostname', {
        method: 'POST',
        body: JSON.stringify({ hostname: hostnameInput.trim() }),
      });
      onNotify(res.message || 'Hostname diperbarui', 'success');
      await loadSystemAndUpdater();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal mengubah hostname', 'error');
    } finally {
      savingHost = false;
    }
  }

  async function saveTimezone() {
    if (!timezoneInput.trim()) return;
    savingTz = true;
    try {
      const res = await apiFetch<{ message: string }>('/api/settings/system/timezone', {
        method: 'POST',
        body: JSON.stringify({ timezone: timezoneInput.trim() }),
      });
      onNotify(res.message || 'Timezone diperbarui', 'success');
      await loadSystemAndUpdater();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal mengubah timezone', 'error');
    } finally {
      savingTz = false;
    }
  }

  async function checkForUpdate() {
    checkingUpdate = true;
    try {
      const res = await apiFetch<{
        ok: boolean;
        latest_tag?: string;
        update_available?: boolean;
        release_url?: string;
        error?: string;
      }>('/api/settings/updater/check');
      checkReleaseResult = { checked: true, ...res };
      if (!res.ok) {
        onNotify(res.error || 'Gagal memeriksa rilis terbaru', 'warning');
      } else if (res.update_available) {
        onNotify(`Pembaruan tersedia: ${res.latest_tag}`, 'info');
      } else {
        onNotify(`FikPanel sudah berada di versi terbaru (${res.latest_tag || 'latest'})`, 'success');
      }
    } catch (err: any) {
      onNotify(err?.message || 'Gagal menghubungi GitHub Releases', 'error');
    } finally {
      checkingUpdate = false;
    }
  }

  async function triggerPanelUpdate() {
    const target = checkReleaseResult?.latest_tag || 'latest';
    const ok = await confirmDialog({
      title: `Perbarui FikPanel ke ${target}?`,
      message: 'Updater akan mencadangkan biner & database, memverifikasi SHA-256, me-restart panel beberapa detik, dan rollback otomatis jika gagal health check.',
      confirmText: 'Perbarui Sekarang',
      tone: 'default',
    });
    if (!ok) return;

    runningUpdate = true;
    try {
      const res = await apiFetch<{ message: string }>('/api/settings/updater/run', {
        method: 'POST',
        body: JSON.stringify({ target }),
      });
      onNotify(res.message || 'Pembaruan dimulai di latar belakang', 'success');
      setTimeout(() => loadSystemAndUpdater(), 3500);
    } catch (err: any) {
      onNotify(err?.message || 'Gagal memulai pembaruan', 'error');
    } finally {
      runningUpdate = false;
    }
  }

  async function toggleAutoUpdate(nextEnabled: boolean) {
    togglingAutoUpdate = true;
    try {
      const res = await apiFetch<{ message: string }>('/api/settings/updater/autoupdate', {
        method: 'POST',
        body: JSON.stringify({ enabled: nextEnabled }),
      });
      onNotify(res.message || 'Status auto-update diperbarui', 'success');
      await loadSystemAndUpdater();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal mengubah auto-update', 'error');
    } finally {
      togglingAutoUpdate = false;
    }
  }

  async function triggerRestartPanel() {
    const ok = await confirmDialog({
      title: 'Restart Layanan FikPanel?',
      message: 'Koneksi panel akan terputus selama ±2–3 detik. Website & layanan server lain tetap berjalan normal.',
      confirmText: 'Restart Panel',
      tone: 'danger',
    });
    if (!ok) return;
    try {
      const res = await apiFetch<{ message: string }>('/api/settings/system/restart', { method: 'POST' });
      onNotify(res.message || 'Layanan panel sedang dimulai ulang…', 'info');
    } catch (err: any) {
      onNotify(err?.message || 'Gagal me-restart panel', 'error');
    }
  }

  // ── Aksi Tab E: Webhook CRUD + Test Kirim ──
  function openNewWebhookForm() {
    editingWebhookId = null;
    hookName = '';
    hookChannel = 'telegram';
    hookUrl = '';
    hookTgToken = '';
    hookTgChatId = '';
    hookCpu = true;
    hookRam = true;
    hookBackup = true;
    hookSsl = true;
    showWebhookForm = true;
  }

  function openEditWebhookForm(hk: SystemWebhook) {
    editingWebhookId = hk.id;
    hookName = hk.name;
    hookChannel = hk.channel || 'generic';
    if (hk.webhook_url.startsWith('tg://')) {
      const parts = hk.webhook_url.slice(5).split('/');
      hookTgToken = parts[0] || '';
      hookTgChatId = parts[1] || '';
      hookUrl = '';
    } else {
      hookUrl = hk.webhook_url;
      hookTgToken = '';
      hookTgChatId = '';
    }
    hookCpu = hk.notify_on_high_cpu;
    hookRam = hk.notify_on_high_ram;
    hookBackup = hk.notify_on_backup;
    hookSsl = hk.notify_on_ssl_expiry;
    showWebhookForm = true;
  }

  async function handleSaveWebhook() {
    let finalUrl = hookUrl.trim();
    if (hookChannel === 'telegram' && !finalUrl && hookTgToken.trim() && hookTgChatId.trim()) {
      finalUrl = `tg://${hookTgToken.trim()}/${hookTgChatId.trim()}`;
    }
    if (!hookName.trim() || !finalUrl) {
      onNotify('Nama dan URL / kredensial Telegram webhook wajib diisi', 'warning');
      return;
    }

    savingWebhook = true;
    try {
      const payload = {
        name: hookName.trim(),
        channel: hookChannel,
        webhook_url: finalUrl,
        notify_on_high_cpu: hookCpu,
        notify_on_high_ram: hookRam,
        notify_on_backup: hookBackup,
        notify_on_ssl_expiry: hookSsl,
      };
      if (editingWebhookId) {
        await apiFetch(`/api/webhooks/${editingWebhookId}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        });
        onNotify('Webhook berhasil diperbarui', 'success');
      } else {
        await apiFetch('/api/webhooks', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        onNotify('Webhook notifikasi berhasil disimpan', 'success');
      }
      showWebhookForm = false;
      await loadWebhooks();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal menyimpan webhook', 'error');
    } finally {
      savingWebhook = false;
    }
  }

  async function handleTestWebhook(hk: SystemWebhook) {
    testingWebhookId = hk.id;
    try {
      const res = await apiFetch<{ ok: boolean; message?: string; error?: string }>(`/api/webhooks/${hk.id}/test`, {
        method: 'POST',
      });
      if (res.ok) {
        onNotify(res.message || `Uji coba ke "${hk.name}" berhasil`, 'success');
      } else {
        onNotify(res.error || `Uji coba ke "${hk.name}" gagal`, 'error');
      }
    } catch (err: any) {
      onNotify(err?.message || 'Gagal mengirim uji coba webhook', 'error');
    } finally {
      testingWebhookId = null;
    }
  }

  async function handleDeleteWebhook(hk: SystemWebhook) {
    const ok = await confirmDialog({
      title: 'Hapus Webhook Notifikasi?',
      message: `Webhook "${hk.name}" tidak lagi menerima peringatan dari panel.`,
      confirmText: 'Hapus Webhook',
      tone: 'danger',
    });
    if (!ok) return;
    try {
      await apiFetch(`/api/webhooks/${hk.id}`, { method: 'DELETE' });
      onNotify('Webhook dihapus', 'success');
      await loadWebhooks();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal menghapus webhook', 'error');
    }
  }

  // ── Aksi Tab G: Manajemen Token MCP (Google Antigravity) ──
  async function handleCreateMCPToken(e: SubmitEvent) {
    e.preventDefault();
    if (!mcpNewName.trim()) {
      onNotify('Nama perangkat / klien MCP wajib diisi', 'warning');
      return;
    }
    mcpCreating = true;
    try {
      const res = await apiFetch<{ ok: boolean; token: string; message?: string }>('/api/mcp/tokens', {
        method: 'POST',
        body: JSON.stringify({
          name: mcpNewName.trim(),
          scope: mcpNewScope,
        }),
      });
      mcpPlainToken = res.token || '';
      onNotify(res.message || 'Token MCP berhasil dibuat', 'success');
      await loadMCP();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal membuat token MCP', 'error');
    } finally {
      mcpCreating = false;
    }
  }

  async function handleToggleMCPToken(tok: MCPTokenItem) {
    mcpBusyId = tok.id;
    try {
      await apiFetch(`/api/mcp/tokens/${tok.id}`, {
        method: 'PUT',
        body: JSON.stringify({
          name: tok.name,
          scope: tok.scope,
          enabled: !tok.enabled,
        }),
      });
      onNotify(tok.enabled ? `Token "${tok.name}" dijeda` : `Token "${tok.name}" diaktifkan kembali`, 'info');
      await loadMCP();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal mengubah status token MCP', 'error');
    } finally {
      mcpBusyId = null;
    }
  }

  async function handleDeleteMCPToken(tok: MCPTokenItem) {
    const ok = await confirmDialog({
      title: 'Cabut Token MCP?',
      message: `Klien Antigravity yang memakai token "${tok.name}" (${tok.token_prefix}) akan langsung kehilangan akses ke /mcp.`,
      confirmText: 'Cabut Token',
      tone: 'danger',
    });
    if (!ok) return;
    mcpBusyId = tok.id;
    try {
      await apiFetch(`/api/mcp/tokens/${tok.id}`, { method: 'DELETE' });
      onNotify(`Token "${tok.name}" berhasil dicabut`, 'success');
      await loadMCP();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal mencabut token MCP', 'error');
    } finally {
      mcpBusyId = null;
    }
  }

  // ── Aksi Tab F: Snapshot DB & Export/Import Profil JSON ──
  function triggerDownload(url: string) {
    const a = document.createElement('a');
    a.href = url;
    a.download = '';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  }

  async function handleImportFileSelected(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    importingConfig = true;
    try {
      const text = await file.text();
      const parsed = JSON.parse(text);
      const res = await apiFetch<{ ok: boolean; message: string; preferences?: FullPreferences }>('/api/settings/config/import', {
        method: 'POST',
        body: JSON.stringify(parsed),
      });
      onNotify(res.message || 'Konfigurasi berhasil diimpor', 'success');
      await loadPreferences();
      await loadWebhooks();
    } catch (err: any) {
      onNotify(err?.message || 'Gagal mengimpor berkas konfigurasi JSON', 'error');
    } finally {
      importingConfig = false;
      input.value = '';
    }
  }

  // ── Helper Format Ringkas ──
  function formatUnix(ts?: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString(dateLocale(), {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  function formatUptime(sec?: number): string {
    if (!sec || sec <= 0) return '-';
    const d = Math.floor(sec / 86400);
    const h = Math.floor((sec % 86400) / 3600);
    const m = Math.floor((sec % 3600) / 60);
    if (d > 0) return `${d}d ${h}h ${m}m`;
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
  }

  function summarizeUA(ua: string): string {
    if (!ua) return 'Unknown Client';
    if (ua.includes('Edg/')) return 'Microsoft Edge';
    if (ua.includes('Chrome/')) return 'Google Chrome';
    if (ua.includes('Firefox/')) return 'Mozilla Firefox';
    if (ua.includes('Safari/') && !ua.includes('Chrome/')) return 'Safari';
    if (ua.includes('curl')) return 'cURL CLI';
    return ua.slice(0, 36);
  }

  async function copyQuick(text: string, label?: string) {
    try {
      await copyText(text);
      onNotify(label || t('common.copied'), 'success');
    } catch {
      onNotify('Gagal menyalin teks', 'error');
    }
  }

  const NAV_TABS: Array<{ id: SettingsTabId; icon: string; key: Parameters<typeof t>[0] }> = [
    { id: 'appearance', icon: 'ri-palette-line', key: 'settings.tab.appearance' },
    { id: 'security', icon: 'ri-shield-user-line', key: 'settings.tab.security' },
    { id: 'network', icon: 'ri-lock-2-line', key: 'settings.tab.network' },
    { id: 'system', icon: 'ri-cpu-line', key: 'settings.tab.system' },
    { id: 'webhooks', icon: 'ri-notification-3-line', key: 'settings.tab.webhooks' },
    { id: 'mcp', icon: 'ri-plug-2-line', key: 'settings.tab.mcp' },
    { id: 'backup', icon: 'ri-database-2-line', key: 'settings.tab.backup' },
  ];
</script>

<div class="pg-settings">
  <div class="page-header">
    <div class="page-header__main">
      <h2 class="page-header__title">{t('settings.title')}</h2>
      <p class="page-header__sub">{t('settings.subtitle')}</p>
    </div>
  </div>

  <!-- Tata Letak 2 Kolom: Navigasi Kategori Vertikal + Area Konten Terfokus -->
  <div class="settings-shell">
    <nav class="settings-nav" aria-label={t('settings.title')}>
      {#each NAV_TABS as tab (tab.id)}
        <button
          type="button"
          class="settings-nav__item"
          class:is-active={activeTab === tab.id}
          onclick={() => (activeTab = tab.id)}
        >
          <i class={tab.icon} aria-hidden="true"></i>
          <span>{t(tab.key)}</span>
        </button>
      {/each}
    </nav>

    <div class="settings-body">
      <!-- ═══ TAB 1: TAMPILAN, BRANDING & BAHASA (A + G) ═══ -->
      {#if activeTab === 'appearance'}
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-palette-line" aria-hidden="true"></i> {t('settings.appearance.theme_title')}
            </h3>
            <button type="button" class="btn btn--ghost btn--sm" onclick={restoreDefaultAppearance} disabled={savingPrefs}>
              <i class="ri-restart-line" aria-hidden="true"></i> {t('settings.appearance.reset_btn')}
            </button>
          </div>
          <div class="card__body stack">
            <p class="panel-desc">{t('settings.appearance.theme_desc')}</p>

            <!-- Mode Tema -->
            <div class="setting-group">
              <span class="field__label">{t('settings.appearance.mode')}</span>
              <div class="seg-row">
                {#each (['dark', 'light', 'auto'] as ThemeMode[]) as mode}
                  <button
                    type="button"
                    class="seg-card"
                    class:is-active={prefs.theme_mode === mode}
                    onclick={() => previewAppearancePatch({ theme_mode: mode })}
                  >
                    <i class={mode === 'dark' ? 'ri-moon-line' : mode === 'light' ? 'ri-sun-line' : 'ri-contrast-2-line'} aria-hidden="true"></i>
                    <span>{t(`settings.appearance.mode_${mode}` as any)}</span>
                  </button>
                {/each}
              </div>
            </div>

            <!-- Palet Warna Aksen + Custom Hex Picker -->
            <div class="setting-group">
              <span class="field__label">{t('settings.appearance.accent')}</span>
              <div class="palette-grid">
                {#each ACCENT_PRESETS as p (p.id)}
                  <button
                    type="button"
                    class="swatch-btn"
                    class:is-active={prefs.accent_preset === p.id}
                    onclick={() => previewAppearancePatch({ accent_preset: p.id })}
                  >
                    <span class={`swatch-dot ${p.swatchClass}`} aria-hidden="true"></span>
                    <span>{p.label}</span>
                  </button>
                {/each}
                <label class="swatch-btn swatch-btn--custom" class:is-active={prefs.accent_preset === 'custom'}>
                  <input
                    type="color"
                    class="color-picker-native"
                    value={prefs.accent_custom}
                    oninput={(e) =>
                      previewAppearancePatch({
                        accent_preset: 'custom',
                        accent_custom: (e.target as HTMLInputElement).value,
                      })}
                    aria-label={t('settings.appearance.custom_color')}
                  />
                  <span class="mono">{prefs.accent_preset === 'custom' ? prefs.accent_custom : 'Custom'}</span>
                </label>
              </div>
            </div>

            <!-- Nuansa Latar Gelap, Kerapatan, & Sudut -->
            <div class="prefs-triple-grid">
              <div class="setting-group">
                <span class="field__label">{t('settings.appearance.surface')}</span>
                <div class="pill-choice-row">
                  {#each (['obsidian', 'midnight', 'carbon'] as SurfaceStyle[]) as surf}
                    <button
                      type="button"
                      class="choice-chip"
                      class:is-active={prefs.surface_style === surf}
                      onclick={() => previewAppearancePatch({ surface_style: surf })}
                    >
                      {t(`settings.appearance.surface_${surf}` as any)}
                    </button>
                  {/each}
                </div>
              </div>

              <div class="setting-group">
                <span class="field__label">{t('settings.appearance.density')}</span>
                <div class="pill-choice-row">
                  {#each (['comfortable', 'compact'] as DensityStyle[]) as dens}
                    <button
                      type="button"
                      class="choice-chip"
                      class:is-active={prefs.density === dens}
                      onclick={() => previewAppearancePatch({ density: dens })}
                    >
                      {t(`settings.appearance.density_${dens}` as any)}
                    </button>
                  {/each}
                </div>
              </div>

              <div class="setting-group">
                <span class="field__label">{t('settings.appearance.radius')}</span>
                <div class="pill-choice-row">
                  {#each (['sharp', 'default', 'rounded'] as RadiusStyle[]) as rad}
                    <button
                      type="button"
                      class="choice-chip"
                      class:is-active={prefs.radius_style === rad}
                      onclick={() => previewAppearancePatch({ radius_style: rad })}
                    >
                      {t(`settings.appearance.radius_${rad}` as any)}
                    </button>
                  {/each}
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Kartu White-Label Branding & Multi-Language -->
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-translate-2" aria-hidden="true"></i> {t('settings.appearance.branding_title')}
            </h3>
          </div>
          <div class="card__body stack">
            <p class="panel-desc">{t('settings.appearance.branding_desc')}</p>

            <div class="form-grid-2">
              <div class="field">
                <label class="field__label" for="pref-brand">{t('settings.appearance.brand_name')}</label>
                <input
                  id="pref-brand"
                  type="text"
                  class="input"
                  maxlength="40"
                  value={prefs.brand_name}
                  oninput={(e) => previewAppearancePatch({ brand_name: (e.target as HTMLInputElement).value })}
                  placeholder="fikpanel"
                />
              </div>

              <div class="field">
                <label class="field__label" for="pref-env">{t('settings.appearance.env_badge')}</label>
                <select
                  id="pref-env"
                  class="input"
                  value={prefs.env_badge}
                  onchange={(e) => previewAppearancePatch({ env_badge: (e.target as HTMLSelectElement).value as EnvBadge })}
                >
                  <option value="">{t('settings.appearance.env_none')}</option>
                  <option value="production">PRODUCTION</option>
                  <option value="staging">STAGING</option>
                  <option value="development">DEVELOPMENT</option>
                </select>
              </div>
            </div>

            <div class="setting-group">
              <span class="field__label">{t('settings.appearance.language')}</span>
              <div class="seg-row seg-row--compact">
                <button
                  type="button"
                  class="seg-card"
                  class:is-active={prefs.language === 'id'}
                  onclick={() => previewAppearancePatch({ language: 'id' })}
                >
                  <span class="mono">ID</span>
                  <span>Bahasa Indonesia</span>
                </button>
                <button
                  type="button"
                  class="seg-card"
                  class:is-active={prefs.language === 'en'}
                  onclick={() => previewAppearancePatch({ language: 'en' })}
                >
                  <span class="mono">EN</span>
                  <span>English</span>
                </button>
              </div>
            </div>

            <div class="card-foot-actions">
              <button type="button" class="btn btn--primary" onclick={() => saveAllPreferences()} disabled={savingPrefs}>
                {#if savingPrefs}<span class="spinner spinner--sm"></span> {t('common.saving')}{:else}{t('settings.appearance.save_btn')}{/if}
              </button>
            </div>
          </div>
        </div>

      <!-- ═══ TAB 2: KEAMANAN AKUN, 2FA & SESI AKTIF (B) ═══ -->
      {:else if activeTab === 'security'}
        <div class="two-col-cards">
          <!-- Ubah Kata Sandi dengan Indikator Kekuatan & Dukungan Multi-Akun Adaptif -->
          <div class="card">
            <div class="card__head">
              <h3 class="card__title">
                <i class="ri-lock-password-line" aria-hidden="true"></i> {t('settings.security.pass_title')}
              </h3>
              <div class="row-actions">
                {#if currentUsername}
                  <span class="account-pill mono">
                    <i class="ri-user-3-line" aria-hidden="true"></i> {currentUsername}
                  </span>
                {/if}
                <button
                  type="button"
                  class="btn btn--ghost btn--icon btn--sm"
                  onclick={() => (showPassFields = !showPassFields)}
                  aria-label={showPassFields ? t('login.hide_password') : t('login.show_password')}
                  title={showPassFields ? t('login.hide_password') : t('login.show_password')}
                >
                  <i class={showPassFields ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
                </button>
              </div>
            </div>
            <div class="card__body stack">
              <p class="panel-desc">{t('settings.security.pass_desc')}</p>
              <form class="pass-form" onsubmit={handlePasswordChange}>
                {#if panelUsers.length > 1}
                  <div class="field">
                    <label for="pass-target-user" class="field__label">{t('settings.security.target_user')}</label>
                    <select id="pass-target-user" class="input" bind:value={targetUserId}>
                      {#each panelUsers as u (u.id)}
                        <option value={u.id}>
                          {u.username} · {u.role}{u.is_current ? ` ${t('settings.security.target_self')}` : ''}
                        </option>
                      {/each}
                    </select>
                  </div>
                {/if}
                <div class="field">
                  <label for="old-pass" class="field__label">
                    {isChangingOtherUser ? t('settings.security.admin_pass_verify') : t('settings.security.old_pass')}
                  </label>
                  <input id="old-pass" type={showPassFields ? 'text' : 'password'} class="input" bind:value={oldPassword} autocomplete="current-password" />
                </div>
                <div class="field">
                  <label for="new-pass" class="field__label">{t('settings.security.new_pass')}</label>
                  <input id="new-pass" type={showPassFields ? 'text' : 'password'} class="input" bind:value={newPassword} required minlength="8" autocomplete="new-password" />
                  {#if newPassword}
                    <div class="strength-bar-wrap">
                      <div class="strength-track">
                        <div class={`strength-fill strength-fill--${passwordStrength.tone}`} style={`--strength: ${passwordStrength.score}%`}></div>
                      </div>
                      <span class={`strength-label strength-label--${passwordStrength.tone}`}>{passwordStrength.label}</span>
                    </div>
                  {/if}
                </div>
                <div class="field">
                  <label for="conf-pass" class="field__label">{t('settings.security.conf_pass')}</label>
                  <input id="conf-pass" type={showPassFields ? 'text' : 'password'} class="input" bind:value={confirmPassword} required minlength="8" autocomplete="new-password" />
                </div>
                <div class="card-foot-actions">
                  <button type="submit" class="btn btn--primary" disabled={updatingPass}>
                    {updatingPass ? t('common.saving') : t('settings.security.pass_btn')}
                  </button>
                </div>
              </form>
            </div>
          </div>

          <!-- 2FA TOTP Authenticator -->
          <div class="card">
            <div class="card__head">
              <h3 class="card__title">
                <i class="ri-smartphone-line" aria-hidden="true"></i> {t('settings.security.totp_title')}
              </h3>
              <span class="badge" class:badge--success={totpEnabled} class:badge--neutral={!totpEnabled}>
                {#if totpEnabled}<span class="dot dot--success" aria-hidden="true"></span>{/if}
                {totpEnabled ? t('common.enabled') : t('common.disabled')}
              </span>
            </div>
            <div class="card__body stack">
              {#if !totpEnabled}
                <p class="panel-desc">{t('settings.security.totp_desc')}</p>
              {/if}

              {#if totpRecoveryCodes.length > 0}
                <div class="recovery-box">
                  <div class="recovery-box__head">
                    <strong>{t('settings.security.totp_recovery_title')}</strong>
                    <div class="row-actions">
                      <button
                        type="button"
                        class="btn btn--secondary btn--sm"
                        onclick={() => copyQuick(totpRecoveryCodes.join('\n'), t('common.copied'))}
                      >
                        <i class="ri-file-copy-line" aria-hidden="true"></i> {t('common.copy')}
                      </button>
                      <button
                        type="button"
                        class="btn btn--ghost btn--icon btn--sm"
                        onclick={() => (totpRecoveryCodes = [])}
                        aria-label={t('common.close')}
                        title={t('common.close')}
                      >
                        <i class="ri-close-line" aria-hidden="true"></i>
                      </button>
                    </div>
                  </div>
                  <p class="panel-desc">{t('settings.security.totp_recovery_desc')}</p>
                  <div class="recovery-grid mono">
                    {#each totpRecoveryCodes as code}
                      <span class="recovery-chip">{code}</span>
                    {/each}
                  </div>
                </div>
              {/if}

              {#if !totpEnabled && !totpSetupSecret}
                <div class="card-foot-actions">
                  <button type="button" class="btn btn--primary" onclick={start2FASetup} disabled={totpBusy}>
                    <i class="ri-qr-code-line" aria-hidden="true"></i> {t('settings.security.totp_start')}
                  </button>
                </div>
              {:else if totpSetupSecret}
                <div class="totp-setup-box">
                  <p class="panel-desc">{t('settings.security.totp_scan')}</p>
                  <div class="totp-qr-row">
                    {#if totpQrDataUrl}
                      <img src={totpQrDataUrl} alt="QR Code 2FA TOTP" class="totp-qr-img" />
                    {/if}
                    <div class="totp-manual-col">
                      <span class="field__label">{t('settings.security.totp_manual')}</span>
                      <div class="copy-code-row">
                        <code class="mono">{totpSetupSecret}</code>
                        <button
                          type="button"
                          class="btn btn--ghost btn--icon btn--sm"
                          onclick={() => copyQuick(totpSetupSecret)}
                          aria-label={t('common.copy')}
                          title={t('common.copy')}
                        >
                          <i class="ri-file-copy-line" aria-hidden="true"></i>
                        </button>
                      </div>
                      <div class="field">
                        <label for="totp-verify" class="field__label">OTP (6 digit)</label>
                        <input
                          id="totp-verify"
                          type="text"
                          class="input mono"
                          maxlength="6"
                          inputmode="numeric"
                          bind:value={totpVerifyCode}
                          placeholder="123456"
                        />
                      </div>
                      <div class="row-actions">
                        <button type="button" class="btn btn--primary btn--sm" onclick={confirmEnable2FA} disabled={totpBusy || totpVerifyCode.trim().length < 6}>
                          {t('settings.security.totp_verify_btn')}
                        </button>
                        <button type="button" class="btn btn--ghost btn--sm" onclick={() => { totpSetupSecret = ''; totpQrDataUrl = ''; }}>
                          {t('common.cancel')}
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              {:else if totpEnabled}
                <!-- Status Banner Hijau Terlindungi -->
                <div class="totp-protected-banner">
                  <span class="totp-protected-banner__icon" aria-hidden="true">
                    <i class="ri-shield-check-fill"></i>
                  </span>
                  <div class="totp-protected-banner__body">
                    <span class="totp-protected-banner__title">{t('settings.security.totp_protected_title')}</span>
                    <span class="totp-protected-banner__desc">{t('settings.security.totp_protected_desc')}</span>
                  </div>
                </div>

                <!-- Ringkasan Konfigurasi 2FA Aktif -->
                <div class="info-list">
                  <div class="info-row">
                    <span class="info-label">{t('settings.security.totp_method')}</span>
                    <span class="info-val">Authenticator App (RFC 6238)</span>
                  </div>
                  <div class="info-row">
                    <span class="info-label">{t('settings.security.totp_account')}</span>
                    <span class="mono info-val">{currentUsername || 'admin'}</span>
                  </div>
                  <div class="info-row">
                    <span class="info-label">{t('settings.security.totp_recovery_codes')}</span>
                    <div class="totp-recovery-inline">
                      <div class="recovery-meter" aria-hidden="true">
                        <div
                          class="recovery-meter__bar"
                          class:recovery-meter__bar--warn={totpRecoveryRemaining > 0 && totpRecoveryRemaining <= 3}
                          class:recovery-meter__bar--danger={totpRecoveryRemaining === 0}
                          style={`--rem-pct: ${totpRecoveryPct}%`}
                        ></div>
                      </div>
                      <span class="mono info-val">{t('settings.security.totp_recovery_count', { count: totpRecoveryRemaining })}</span>
                    </div>
                  </div>
                </div>

                <!-- Tombol Aksi Rapi (Membuka Modal Popup Konfirmasi) -->
                <div class="card-foot-actions">
                  <button type="button" class="btn btn--secondary btn--sm" onclick={() => openTotpModal('regen')} disabled={totpBusy}>
                    <i class="ri-key-2-line" aria-hidden="true"></i> {t('settings.security.totp_regen_btn')}
                  </button>
                  <button type="button" class="btn btn--danger btn--sm" onclick={() => openTotpModal('disable')} disabled={totpBusy}>
                    <i class="ri-shield-cross-line" aria-hidden="true"></i> {t('settings.security.totp_disable_btn')}
                  </button>
                </div>
              {/if}
            </div>
          </div>
        </div>

        <!-- Kartu Passkeys / Login Tanpa Kata Sandi (WebAuthn FIDO2) -->
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-fingerprint-line" aria-hidden="true"></i> {t('settings.security.passkey_title')}
            </h3>
            <button
              type="button"
              class="btn btn--primary btn--sm"
              onclick={openPasskeyRegisterModal}
              disabled={!webauthnEnv.validOrigin || passkeyBusy}
            >
              {t('settings.security.passkey_add_btn')}
            </button>
          </div>
          <div class="card__body stack">
            <p class="panel-desc">{t('settings.security.passkey_desc')}</p>

            {#if webauthnEnv.reason === 'raw_ip'}
              <div class="alert alert--warning">
                <i class="ri-error-warning-line" aria-hidden="true"></i>
                <span>{t('settings.security.passkey_raw_ip_warn')}</span>
              </div>
            {:else if webauthnEnv.reason === 'insecure'}
              <div class="alert alert--warning">
                <i class="ri-lock-unlock-line" aria-hidden="true"></i>
                <span>{t('settings.security.passkey_insecure_warn')}</span>
              </div>
            {/if}

            <div class="item-stack">
              {#if passkeys.length === 0}
                <div class="text-subtle">{t('settings.security.passkey_empty')}</div>
              {:else}
                {#each passkeys as pk (pk.id)}
                  <div class="list-row-item">
                    <div class="list-row-item__main">
                      <div class="list-row-item__title">
                        <i class="ri-key-2-line" aria-hidden="true"></i>
                        <strong>{pk.name}</strong>
                        {#if pk.transports}
                          {#each pk.transports.split(',').filter(Boolean) as tr}
                            <span class="badge badge--info">{tr}</span>
                          {/each}
                        {/if}
                      </div>
                      <div class="list-row-item__sub">
                        {formatUnix(pk.created_at)} &middot; {t('settings.security.passkey_last_used')} {pk.last_used_at ? formatUnix(pk.last_used_at) : t('settings.security.passkey_never_used')}
                      </div>
                    </div>
                    <button
                      type="button"
                      class="btn btn--danger btn--icon btn--sm"
                      onclick={() => handleDeletePasskey(pk)}
                      aria-label={`${t('common.delete')} ${pk.name}`}
                      title={`${t('common.delete')} ${pk.name}`}
                    >
                      <i class="ri-delete-bin-line" aria-hidden="true"></i>
                    </button>
                  </div>
                {/each}
              {/if}
            </div>
          </div>
        </div>

        <!-- Daftar Sesi Perangkat Aktif -->
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-device-line" aria-hidden="true"></i> {t('settings.security.sessions_title')}
            </h3>
            <div class="row-actions">
              <button type="button" class="btn btn--ghost btn--sm" onclick={loadSecurityData} disabled={loadingSessions}>
                <i class="ri-refresh-line" aria-hidden="true"></i> {t('common.refresh')}
              </button>
              {#if activeSessions.length > 1}
                <button type="button" class="btn btn--danger btn--sm" onclick={revokeOtherSessions}>
                  {t('settings.security.sessions_revoke_others')}
                </button>
              {/if}
            </div>
          </div>
          <div class="card__body">
            <p class="panel-desc">{t('settings.security.sessions_desc')}</p>
            <div class="item-stack">
              {#each activeSessions as sess (sess.id)}
                <div class="list-row-item">
                  <div class="list-row-item__main">
                    <div class="list-row-item__title">
                      <span class="mono">{sess.ip || '127.0.0.1'}</span>
                      <span>&middot;</span>
                      <span>{summarizeUA(sess.user_agent)}</span>
                      {#if sess.is_current}
                        <span class="badge badge--success">{t('settings.security.sessions_current')}</span>
                      {/if}
                    </div>
                    <div class="list-row-item__sub">
                      Login: {formatUnix(sess.created_at)} &middot; Exp: {formatUnix(sess.expires_at)}
                    </div>
                  </div>
                  {#if !sess.is_current}
                    <button type="button" class="btn btn--danger btn--sm" onclick={() => revokeSingleSession(sess)}>
                      {t('settings.security.sessions_revoke')}
                    </button>
                  {/if}
                </div>
              {/each}
            </div>
          </div>
        </div>

        <!-- Ringkasan Audit Log Keamanan -->
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-file-shield-2-line" aria-hidden="true"></i> {t('settings.security.audit_title')}
            </h3>
          </div>
          <div class="card__body">
            <div class="table-wrap">
              <table class="table audit-table">
                <thead>
                  <tr>
                    <th>{t('settings.security.audit_time')}</th>
                    <th>{t('settings.security.audit_action')}</th>
                    <th>{t('settings.security.audit_target')}</th>
                    <th>{t('settings.security.audit_status')}</th>
                    <th>{t('settings.security.audit_detail')}</th>
                  </tr>
                </thead>
                <tbody>
                  {#if auditLogs.length === 0}
                    <tr><td colspan="5" class="text-subtle">{t('settings.security.audit_empty')}</td></tr>
                  {:else}
                    {#each auditLogs as log (log.id)}
                      <tr>
                        <td class="mono nowrap">{formatUnix(log.ts)}</td>
                        <td class="mono">{log.action}</td>
                        <td class="mono truncate max-w-140" title={log.target}>{log.target}</td>
                        <td>
                          <span class="badge" class:badge--success={log.ok === 1} class:badge--danger={log.ok !== 1}>
                            {log.ok === 1 ? 'OK' : 'FAIL'}
                          </span>
                        </td>
                        <td class="truncate max-w-260" title={log.detail}>{log.detail}</td>
                      </tr>
                    {/each}
                  {/if}
                </tbody>
              </table>
            </div>
          </div>
        </div>

      <!-- ═══ TAB 3: JARINGAN PANEL, HTTPS/TLS & AKSES (C) ═══ -->
      {:else if activeTab === 'network'}
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-lock-2-line" aria-hidden="true"></i> {t('settings.network.tls_title')}
            </h3>
            {#if tlsInfo}
              <span class="badge" class:badge--success={tlsInfo.enabled && !tlsInfo.self_signed} class:badge--info={tlsInfo.enabled && tlsInfo.self_signed} class:badge--warning={!tlsInfo.enabled}>
                {#if !tlsInfo.enabled}
                  {t('settings.network.tls_http_only')}
                {:else if tlsInfo.self_signed}
                  {t('settings.network.tls_self_signed')}
                {:else}
                  {t('settings.network.tls_custom')}
                {/if}
              </span>
            {/if}
          </div>
          <div class="card__body stack">
            <p class="panel-desc">{t('settings.network.tls_desc')}</p>

            {#if tlsInfo?.enabled}
              <div class="info-list">
                <div class="info-row">
                  <span class="info-label">Subject / Issuer</span>
                  <span class="mono info-val">{tlsInfo.subject || 'fikpanel'} ({tlsInfo.issuer || '-'})</span>
                </div>
                <div class="info-row">
                  <span class="info-label">{t('settings.network.tls_valid_until')}</span>
                  <span class="mono info-val">
                    {formatUnix(tlsInfo.not_after)}
                    <span class="badge badge--sm" class:badge--success={tlsInfo.days_remaining > 30} class:badge--warning={tlsInfo.days_remaining <= 30}>
                      {t('settings.network.tls_days', { count: tlsInfo.days_remaining })}
                    </span>
                  </span>
                </div>
                <div class="info-row">
                  <span class="info-label">SHA-256 Fingerprint</span>
                  <div class="copy-code-row">
                    <span class="mono info-val truncate max-w-320" title={tlsInfo.fingerprint}>{tlsInfo.fingerprint || '-'}</span>
                    {#if tlsInfo.fingerprint}
                      <button
                        type="button"
                        class="btn btn--ghost btn--icon btn--sm"
                        onclick={() => copyQuick(tlsInfo?.fingerprint || '', 'Fingerprint SHA-256 disalin')}
                        aria-label={t('common.copy')}
                        title={t('common.copy')}
                      >
                        <i class="ri-file-copy-line" aria-hidden="true"></i>
                      </button>
                    {/if}
                  </div>
                </div>
              </div>

              <div class="acme-panel-box">
                <div class="acme-panel-box__head">
                  <span class="acme-panel-box__title">
                    <i class="ri-shield-star-line" aria-hidden="true"></i>
                    {t('settings.network.acme_title')}
                  </span>
                  {#if activeAcmeProvider?.configured}
                    <span class="badge badge--success badge--sm">
                      {activeAcmeProvider.source === 'cloudflare_hub'
                        ? t('settings.network.acme_cf_hub')
                        : t('settings.network.acme_saved')}
                    </span>
                  {:else}
                    <span class="badge badge--warning badge--sm">{t('settings.network.acme_unconfigured')}</span>
                  {/if}
                </div>
                <p class="panel-desc">{t('settings.network.acme_desc')}</p>

                <div class="form-grid-2">
                  <div class="field">
                    <label for="acme-panel-domain" class="field__label">{t('settings.network.acme_domain')}</label>
                    <input
                      id="acme-panel-domain"
                      type="text"
                      class="input mono"
                      bind:value={acmeDomain}
                      placeholder="panel.domainanda.com"
                      disabled={issuingACME}
                    />
                  </div>
                  <div class="field">
                    <label for="acme-panel-provider" class="field__label">{t('settings.network.acme_provider')}</label>
                    <select
                      id="acme-panel-provider"
                      class="input"
                      bind:value={acmeProvider}
                      onchange={() => { acmeCreds = {}; acmeEditCreds = false; }}
                      disabled={issuingACME}
                    >
                      {#if dnsProviders.length > 0}
                        {#each dnsProviders as prov (prov.id)}
                          <option value={prov.id}>{prov.name}{prov.configured ? ' ✓' : ''}</option>
                        {/each}
                      {:else}
                        <option value="cloudflare">Cloudflare DNS</option>
                        <option value="digitalocean">DigitalOcean DNS</option>
                        <option value="porkbun">Porkbun DNS</option>
                        <option value="namecheap">Namecheap DNS</option>
                        <option value="duckdns">DuckDNS</option>
                      {/if}
                    </select>
                  </div>
                </div>

                {#if activeAcmeProvider}
                  <p class="panel-desc">{activeAcmeProvider.description}</p>
                  {#if activeAcmeProvider.configured && !acmeEditCreds}
                    <div>
                      <button type="button" class="btn btn--ghost btn--sm" onclick={() => (acmeEditCreds = true)} disabled={issuingACME}>
                        <i class="ri-key-2-line" aria-hidden="true"></i> {t('settings.network.acme_change_creds')}
                      </button>
                    </div>
                  {:else}
                    <div class="form-grid-2">
                      {#each activeAcmeProvider.fields as fld (fld.key)}
                        <div class="field">
                          <label for="acme-cred-{fld.key}" class="field__label">{fld.label}{fld.required ? ' *' : ''}</label>
                          <input
                            id="acme-cred-{fld.key}"
                            type={fld.secret ? 'password' : 'text'}
                            class="input mono"
                            placeholder={fld.placeholder}
                            value={acmeCreds[fld.key] ?? ''}
                            oninput={(e) => { acmeCreds = { ...acmeCreds, [fld.key]: (e.currentTarget as HTMLInputElement).value }; }}
                            disabled={issuingACME}
                          />
                        </div>
                      {/each}
                    </div>
                  {/if}
                {/if}

                <label class="acme-panel-box__check" for="acme-panel-wildcard">
                  <input id="acme-panel-wildcard" type="checkbox" bind:checked={acmeIncludeWildcard} disabled={issuingACME} />
                  <span>{t('settings.network.acme_wildcard')}</span>
                </label>

                <div class="card-foot-actions">
                  <button type="button" class="btn btn--primary" onclick={issuePanelACMETLS} disabled={issuingACME || savingTLS || !acmeDomain.trim()}>
                    {#if issuingACME}
                      <i class="ri-loader-4-line spin" aria-hidden="true"></i> {t('settings.network.acme_issuing')}
                    {:else}
                      <i class="ri-shield-check-line" aria-hidden="true"></i> {t('settings.network.acme_issue_btn')}
                    {/if}
                  </button>
                </div>
              </div>

              <div class="tls-divider-label">{t('settings.network.tls_manual_title')}</div>

              <div class="form-grid-2">
                <div class="field">
                  <label for="tls-cert" class="field__label">Certificate / Fullchain (PEM)</label>
                  <textarea id="tls-cert" rows="5" class="input mono" bind:value={customCertPEM} placeholder="-----BEGIN CERTIFICATE-----"></textarea>
                </div>
                <div class="field">
                  <label for="tls-key" class="field__label">Private Key (PEM)</label>
                  <textarea id="tls-key" rows="5" class="input mono" bind:value={customKeyPEM} placeholder="-----BEGIN PRIVATE KEY-----"></textarea>
                </div>
              </div>

              <div class="card-foot-actions">
                <button type="button" class="btn btn--primary" onclick={installCustomTLS} disabled={savingTLS || issuingACME}>
                  {savingTLS ? t('common.saving') : t('settings.network.tls_install_btn')}
                </button>
                <button type="button" class="btn btn--secondary" onclick={resetSelfSignedTLS} disabled={savingTLS || issuingACME}>
                  {t('settings.network.tls_reset_btn')}
                </button>
              </div>
            {/if}
          </div>
        </div>

        <!-- Kebijakan Durasi Sesi & IP Allowlist -->
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-shield-keyhole-line" aria-hidden="true"></i> {t('settings.network.access_title')}
            </h3>
          </div>
          <div class="card__body stack">
            <p class="panel-desc">{t('settings.network.access_desc')}</p>

            <div class="field max-w-280">
              <label for="pref-session-hrs" class="field__label">{t('settings.network.session_hours')}</label>
              <select id="pref-session-hrs" class="input" bind:value={prefs.session_hours}>
                <option value={6}>{t('settings.network.session_6h')}</option>
                <option value={12}>{t('settings.network.session_12h')}</option>
                <option value={24}>{t('settings.network.session_24h')}</option>
                <option value={72}>{t('settings.network.session_3d')}</option>
                <option value={168}>{t('settings.network.session_7d')}</option>
                <option value={720}>{t('settings.network.session_30d')}</option>
              </select>
            </div>

            <div class="field">
              <div class="label-with-helper">
                <label for="pref-allowlist" class="field__label">{t('settings.network.ip_allowlist')}</label>
                {#if clientIP}
                  <div class="ip-helper">
                    <span class="text-subtle">{t('settings.network.ip_current')} <code class="mono">{clientIP}</code></span>
                    <button type="button" class="btn btn--ghost btn--sm" onclick={appendMyIPToAllowlist}>
                      {t('settings.network.ip_add_mine')}
                    </button>
                  </div>
                {/if}
              </div>
              <textarea
                id="pref-allowlist"
                rows="4"
                class="input mono"
                bind:value={ipAllowlistText}
                placeholder={t('settings.network.ip_placeholder')}
              ></textarea>
            </div>

            <div class="card-foot-actions">
              <button type="button" class="btn btn--primary" onclick={() => saveAllPreferences()} disabled={savingPrefs}>
                {savingPrefs ? t('common.saving') : t('settings.network.save_access')}
              </button>
            </div>
          </div>
        </div>

      <!-- ═══ TAB 4: SISTEM, HOSTNAME, TIMEZONE & UPDATE CENTER (D) ═══ -->
      {:else if activeTab === 'system'}
        <div class="two-col-cards">
          <!-- Informasi Lingkungan Nyata (Anti-Hardcode) -->
          <div class="card">
            <div class="card__head">
              <h3 class="card__title">
                <i class="ri-server-line" aria-hidden="true"></i> {t('settings.system.env_title')}
              </h3>
              <button type="button" class="btn btn--ghost btn--icon btn--sm" onclick={loadSystemAndUpdater} aria-label={t('common.refresh')} title={t('common.refresh')}>
                <i class="ri-refresh-line" aria-hidden="true"></i>
              </button>
            </div>
            <div class="card__body">
              <div class="info-list">
                <div class="info-row">
                  <span class="info-label">Hostname</span>
                  <span class="mono info-val">{sysInfo?.hostname || overview?.system?.hostname || '-'}</span>
                </div>
                <div class="info-row">
                  <span class="info-label">{t('settings.system.os')}</span>
                  <span class="info-val">{sysInfo?.os_pretty || overview?.system?.os || '-'}</span>
                </div>
                <div class="info-row">
                  <span class="info-label">Kernel</span>
                  <span class="mono info-val">{sysInfo?.kernel || overview?.system?.kernel || '-'}</span>
                </div>
                <div class="info-row">
                  <span class="info-label">Runtime / CPU</span>
                  <span class="mono info-val">{sysInfo?.go_version || '-'} &middot; {sysInfo?.cpu_cores || overview?.system?.cpu_cores || 0} Cores</span>
                </div>
                <div class="info-row">
                  <span class="info-label">Host Uptime</span>
                  <span class="mono info-val">{formatUptime(sysInfo?.uptime_sec)}</span>
                </div>
                <div class="info-row">
                  <span class="info-label">Database SQLite</span>
                  <span class="mono info-val truncate max-w-260" title={sysInfo?.db_path}>
                    {sysInfo?.db_path || '-'} ({formatBytes(sysInfo?.db_size_bytes || 0)})
                  </span>
                </div>
                <div class="info-row">
                  <span class="info-label">{t('settings.system.panel_version')}</span>
                  <span class="badge badge--info mono">v{sysInfo?.version || overview?.system?.version || '0.7.6'}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Pengaturan Hostname & Timezone + Power -->
          <div class="card">
            <div class="card__head">
              <h3 class="card__title">
                <i class="ri-time-zone-line" aria-hidden="true"></i> {t('settings.system.hostname_tz_title')}
              </h3>
            </div>
            <div class="card__body stack">
              <p class="panel-desc">{t('settings.system.hostname_tz_desc')}</p>

              <div class="field">
                <label for="sys-hostname" class="field__label">Hostname</label>
                <div class="inline-action-row">
                  <input id="sys-hostname" type="text" class="input mono" bind:value={hostnameInput} />
                  <button type="button" class="btn btn--secondary btn--sm" onclick={saveHostname} disabled={savingHost}>
                    {savingHost ? t('common.saving') : t('common.save')}
                  </button>
                </div>
              </div>

              <div class="field">
                <label for="sys-tz" class="field__label">Timezone ({sysInfo?.tz_abbr || 'UTC'})</label>
                <div class="inline-action-row">
                  <select id="sys-tz" class="input mono" bind:value={timezoneInput}>
                    {#if timezoneInput && !timezones.includes(timezoneInput)}
                      <option value={timezoneInput}>{timezoneInput}</option>
                    {/if}
                    {#each timezones as tz}
                      <option value={tz}>{tz}</option>
                    {/each}
                  </select>
                  <button type="button" class="btn btn--secondary btn--sm" onclick={saveTimezone} disabled={savingTz}>
                    {savingTz ? t('common.saving') : t('common.save')}
                  </button>
                </div>
              </div>

              <div class="danger-strip">
                <span class="text-subtle">{t('settings.system.daemon_label')}</span>
                <button type="button" class="btn btn--danger btn--sm" onclick={triggerRestartPanel}>
                  <i class="ri-restart-line" aria-hidden="true"></i> {t('settings.system.restart_panel')}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Pusat Pembaruan FikPanel (Update Center GUI) -->
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-download-cloud-2-line" aria-hidden="true"></i> {t('settings.system.updater_title')}
            </h3>
            <div class="row-actions">
              <button type="button" class="btn btn--secondary btn--sm" onclick={checkForUpdate} disabled={checkingUpdate}>
                {#if checkingUpdate}<span class="spinner spinner--sm"></span> {t('settings.system.checking')}{:else}<i class="ri-refresh-line" aria-hidden="true"></i> {t('settings.system.check_update')}{/if}
              </button>
              {#if checkReleaseResult?.update_available}
                <button type="button" class="btn btn--primary btn--sm" onclick={triggerPanelUpdate} disabled={runningUpdate}>
                  <i class="ri-rocket-line" aria-hidden="true"></i> {t('settings.system.update_now')} ({checkReleaseResult.latest_tag})
                </button>
              {/if}
            </div>
          </div>
          <div class="card__body stack">
            <p class="panel-desc">{t('settings.system.updater_desc')}</p>

            {#if checkReleaseResult?.checked}
              <div class="alert" class:alert--info={checkReleaseResult.update_available} class:alert--success={checkReleaseResult.ok && !checkReleaseResult.update_available} class:alert--danger={!checkReleaseResult.ok}>
                <i class={checkReleaseResult.ok ? 'ri-checkbox-circle-line' : 'ri-error-warning-line'} aria-hidden="true"></i>
                <span>
                  {#if !checkReleaseResult.ok}
                    {checkReleaseResult.error}
                  {:else if checkReleaseResult.update_available}
                    {t('settings.system.update_available')} <strong class="mono">{checkReleaseResult.latest_tag}</strong>
                    {#if checkReleaseResult.release_url}
                      &middot; <a href={checkReleaseResult.release_url} target="_blank" rel="noopener noreferrer">Release Notes</a>
                    {/if}
                  {:else}
                    {t('settings.system.up_to_date')} (<strong class="mono">{checkReleaseResult.latest_tag}</strong>).
                  {/if}
                </span>
              </div>
            {/if}

            <div class="list-row-item">
              <div class="list-row-item__main">
                <div class="list-row-item__title">
                  <span>{t('settings.system.autoupdate')}</span>
                  <span class="badge" class:badge--success={updaterInfo?.auto_update?.enabled} class:badge--neutral={!updaterInfo?.auto_update?.enabled}>
                    {updaterInfo?.auto_update?.enabled ? t('common.enabled') : t('common.disabled')}
                  </span>
                </div>
                <div class="list-row-item__sub">
                  {#if updaterInfo?.auto_update?.next_run}
                    Next: {updaterInfo.auto_update.next_run}
                  {:else}
                    Timer systemd `fikpanel-autoupdate.timer`
                  {/if}
                </div>
              </div>
              <button
                type="button"
                class="btn btn--secondary btn--sm"
                onclick={() => toggleAutoUpdate(!updaterInfo?.auto_update?.enabled)}
                disabled={togglingAutoUpdate}
              >
                {updaterInfo?.auto_update?.enabled ? t('settings.system.disable_btn') : t('settings.system.enable_btn')}
              </button>
            </div>

            {#if updaterInfo?.last_status?.state}
              <div class="info-row">
                <span class="info-label">{t('settings.system.last_update_status')}</span>
                <span class="mono info-val">
                  {updaterInfo.last_status.from || '-'} &rarr; {updaterInfo.last_status.to || updaterInfo.last_status.state}
                  ({updaterInfo.last_status.state})
                </span>
              </div>
            {/if}

            {#if updaterInfo?.log_tail}
              <div>
                <button type="button" class="btn btn--ghost btn--sm" onclick={() => (showUpdateLog = !showUpdateLog)}>
                  <i class={showUpdateLog ? 'ri-arrow-up-s-line' : 'ri-arrow-down-s-line'} aria-hidden="true"></i>
                  {showUpdateLog ? t('settings.system.hide_log') : t('settings.system.show_log')}
                </button>
                {#if showUpdateLog}
                  <pre class="updater-log mono">{updaterInfo.log_tail}</pre>
                {/if}
              </div>
            {/if}
          </div>
        </div>

      <!-- ═══ TAB 5: WEBHOOK PINTAR & AMBANG BATAS ALERT (E) ═══ -->
      {:else if activeTab === 'webhooks'}
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-notification-3-line" aria-hidden="true"></i> {t('settings.webhooks.title')}
            </h3>
            <button type="button" class="btn btn--primary btn--sm" onclick={openNewWebhookForm}>
              {t('settings.webhooks.add_btn')}
            </button>
          </div>
          <div class="card__body stack">
            <p class="panel-desc">{t('settings.webhooks.desc')}</p>

            {#if showWebhookForm}
              <div class="webhook-form">
                <div class="form-grid-2">
                  <div class="field">
                    <label for="hk-n" class="field__label">{t('settings.webhooks.name')}</label>
                    <input id="hk-n" type="text" class="input" bind:value={hookName} placeholder={t('settings.webhooks.name_placeholder')} />
                  </div>
                  <div class="field">
                    <label for="hk-ch" class="field__label">{t('settings.webhooks.channel')}</label>
                    <select id="hk-ch" class="input" bind:value={hookChannel}>
                      <option value="telegram">Telegram Bot</option>
                      <option value="discord">Discord Channel (Embed)</option>
                      <option value="generic">Custom Webhook (JSON POST)</option>
                    </select>
                  </div>
                </div>

                {#if hookChannel === 'telegram'}
                  <div class="form-grid-2">
                    <div class="field">
                      <label for="hk-tg-tok" class="field__label">{t('settings.webhooks.tg_token')}</label>
                      <input id="hk-tg-tok" type="text" class="input mono" bind:value={hookTgToken} placeholder="123456:ABC-DEF..." />
                    </div>
                    <div class="field">
                      <label for="hk-tg-cid" class="field__label">{t('settings.webhooks.tg_chat')}</label>
                      <input id="hk-tg-cid" type="text" class="input mono" bind:value={hookTgChatId} placeholder="-1001234567890" />
                    </div>
                  </div>
                {/if}

                <div class="field">
                  <label for="hk-u" class="field__label">
                    {hookChannel === 'telegram' ? t('settings.webhooks.tg_url_alt') : 'Webhook URL'}
                  </label>
                  <input
                    id="hk-u"
                    type="text"
                    class="input mono"
                    bind:value={hookUrl}
                    placeholder={hookChannel === 'discord' ? 'https://discord.com/api/webhooks/...' : 'https://...'}
                  />
                </div>

                <div class="trigger-checks">
                  <label class="check-pill"><input type="checkbox" bind:checked={hookCpu} /> {t('settings.webhooks.trigger_cpu')}</label>
                  <label class="check-pill"><input type="checkbox" bind:checked={hookRam} /> {t('settings.webhooks.trigger_ram')}</label>
                  <label class="check-pill"><input type="checkbox" bind:checked={hookBackup} /> {t('settings.webhooks.trigger_backup')}</label>
                  <label class="check-pill"><input type="checkbox" bind:checked={hookSsl} /> {t('settings.webhooks.trigger_ssl')}</label>
                </div>

                <div class="card-foot-actions">
                  <button type="button" class="btn btn--ghost btn--sm" onclick={() => (showWebhookForm = false)}>{t('common.cancel')}</button>
                  <button type="button" class="btn btn--primary btn--sm" onclick={handleSaveWebhook} disabled={savingWebhook}>
                    {savingWebhook ? t('common.saving') : t('common.save')}
                  </button>
                </div>
              </div>
            {/if}

            <div class="item-stack">
              {#if webhooks.length === 0}
                <div class="text-subtle">{t('settings.webhooks.empty')}</div>
              {:else}
                {#each webhooks as hk (hk.id)}
                  <div class="list-row-item">
                    <div class="list-row-item__main">
                      <div class="list-row-item__title">
                        <strong>{hk.name}</strong>
                        <span class="badge badge--info">{hk.channel}</span>
                      </div>
                      <div class="list-row-item__sub mono truncate" title={hk.webhook_url}>{hk.webhook_url}</div>
                      <div class="trigger-badges">
                        {#if hk.notify_on_high_cpu}<span class="badge badge--neutral">CPU</span>{/if}
                        {#if hk.notify_on_high_ram}<span class="badge badge--neutral">RAM</span>{/if}
                        {#if hk.notify_on_backup}<span class="badge badge--neutral">Backup</span>{/if}
                        {#if hk.notify_on_ssl_expiry}<span class="badge badge--neutral">SSL</span>{/if}
                      </div>
                    </div>
                    <div class="row-actions">
                      <button
                        type="button"
                        class="btn btn--secondary btn--sm"
                        onclick={() => handleTestWebhook(hk)}
                        disabled={testingWebhookId === hk.id}
                      >
                        {testingWebhookId === hk.id ? t('common.testing') : t('common.test')}
                      </button>
                      <button
                        type="button"
                        class="btn btn--ghost btn--icon btn--sm"
                        onclick={() => openEditWebhookForm(hk)}
                        aria-label={`Edit ${hk.name}`}
                        title={`Edit ${hk.name}`}
                      >
                        <i class="ri-pencil-line" aria-hidden="true"></i>
                      </button>
                      <button
                        type="button"
                        class="btn btn--danger btn--icon btn--sm"
                        onclick={() => handleDeleteWebhook(hk)}
                        aria-label={`Hapus ${hk.name}`}
                        title={`Hapus ${hk.name}`}
                      >
                        <i class="ri-delete-bin-line" aria-hidden="true"></i>
                      </button>
                    </div>
                  </div>
                {/each}
              {/if}
            </div>
          </div>
        </div>

        <!-- Ambang Batas Alert Global -->
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-equalizer-line" aria-hidden="true"></i> {t('settings.webhooks.threshold_title')}
            </h3>
          </div>
          <div class="card__body stack">
            <p class="panel-desc">{t('settings.webhooks.threshold_desc')}</p>
            <div class="threshold-grid">
              <div class="field">
                <label for="th-cpu" class="field__label">CPU Alert (%)</label>
                <input id="th-cpu" type="number" class="input mono" min="50" max="99" bind:value={prefs.alert_cpu_pct} />
              </div>
              <div class="field">
                <label for="th-ram" class="field__label">RAM Alert (%)</label>
                <input id="th-ram" type="number" class="input mono" min="50" max="99" bind:value={prefs.alert_ram_pct} />
              </div>
              <div class="field">
                <label for="th-disk" class="field__label">Disk Alert (%)</label>
                <input id="th-disk" type="number" class="input mono" min="50" max="99" bind:value={prefs.alert_disk_pct} />
              </div>
              <div class="field">
                <label for="th-ssl" class="field__label">{t('settings.webhooks.ssl_days')}</label>
                <input id="th-ssl" type="number" class="input mono" min="1" max="90" bind:value={prefs.alert_ssl_days} />
              </div>
              <div class="field">
                <label for="th-cool" class="field__label">{t('settings.webhooks.cooldown_min')}</label>
                <input id="th-cool" type="number" class="input mono" min="1" max="1440" bind:value={prefs.alert_cooldown_min} />
              </div>
            </div>
            <div class="card-foot-actions">
              <button type="button" class="btn btn--primary" onclick={() => saveAllPreferences()} disabled={savingPrefs}>
                {savingPrefs ? t('common.saving') : t('settings.webhooks.save_threshold')}
              </button>
            </div>
          </div>
        </div>

      <!-- ═══ TAB 6: MCP SERVER & INTEGRASI GOOGLE ANTIGRAVITY ═══ -->
      {:else if activeTab === 'mcp'}
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-plug-2-line" aria-hidden="true"></i> {t('settings.mcp.hero_title')}
            </h3>
            <div class="row-actions">
              <span class="badge badge--info mono">Streamable HTTP</span>
              <span class="badge badge--success">{t('settings.mcp.ready_badge')}</span>
            </div>
          </div>
          <div class="card__body stack">
            <p class="panel-desc">{t('settings.mcp.hero_desc')}</p>

            <!-- Daftar klien MCP: fokus Google Antigravity, klien lain Coming Soon -->
            <div class="mcp-clients-row">
              <span class="mcp-client-pill mcp-client-pill--active">
                <i class="ri-sparkling-2-fill" aria-hidden="true"></i>
                Google Antigravity
                <span class="mcp-client-pill__tag">{t('settings.mcp.ready_badge')}</span>
              </span>
              <span class="mcp-client-pill">
                <i class="ri-terminal-window-line" aria-hidden="true"></i>
                Cursor
                <span class="mcp-client-pill__tag">{t('settings.mcp.coming_soon')}</span>
              </span>
              <span class="mcp-client-pill">
                <i class="ri-chat-3-line" aria-hidden="true"></i>
                Claude Desktop
                <span class="mcp-client-pill__tag">{t('settings.mcp.coming_soon')}</span>
              </span>
              <span class="mcp-client-pill">
                <i class="ri-code-s-slash-line" aria-hidden="true"></i>
                VS Code Copilot
                <span class="mcp-client-pill__tag">{t('settings.mcp.coming_soon')}</span>
              </span>
            </div>

            <!-- URL Endpoint MCP -->
            <div class="field">
              <span class="field__label">{t('settings.mcp.endpoint_label')}</span>
              <div class="inline-action-row">
                <input type="text" class="input mono" value={mcpEndpointUrl} readonly aria-label={t('settings.mcp.endpoint_label')} />
                <button
                  type="button"
                  class="btn btn--secondary btn--sm"
                  onclick={() => copyQuick(mcpEndpointUrl, 'Endpoint MCP disalin')}
                >
                  <i class="ri-file-copy-line" aria-hidden="true"></i> {t('common.copy')}
                </button>
              </div>
            </div>

            {#if typeof window !== 'undefined' && (window.location.protocol !== 'https:' || tlsInfo?.self_signed)}
              <div class="alert alert--warning">
                <i class="ri-information-line" aria-hidden="true"></i>
                <span>{t('settings.mcp.self_signed_warn')}</span>
              </div>
            {/if}

            <!-- Blok konfigurasi mcp_config.json siap salin untuk Google Antigravity -->
            <div class="mcp-snippet-box">
              <div class="mcp-snippet-box__head">
                <strong class="field__label">{t('settings.mcp.config_title')}</strong>
                <button
                  type="button"
                  class="btn btn--primary btn--sm"
                  onclick={() => copyQuick(antigravityConfigJson, 'Konfigurasi mcp_config.json Antigravity disalin')}
                >
                  <i class="ri-file-copy-line" aria-hidden="true"></i> {t('settings.mcp.copy_json')}
                </button>
              </div>
              <pre class="mcp-code mono">{antigravityConfigJson}</pre>
              <ol class="mcp-steps">
                <li>{t('settings.mcp.step_1')}</li>
                <li>{t('settings.mcp.config_path_hint')}</li>
                <li>{t('settings.mcp.step_3')}</li>
              </ol>
            </div>
          </div>
        </div>

        <!-- Kartu Manajemen Token MCP -->
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-key-2-line" aria-hidden="true"></i> {t('settings.mcp.tokens_title')}
            </h3>
            <span class="badge badge--muted mono">{mcpTokens.length}</span>
          </div>
          <div class="card__body stack">
            <p class="panel-desc">{t('settings.mcp.tokens_desc')}</p>

            {#if mcpPlainToken}
              <div class="recovery-box">
                <div class="recovery-box__head">
                  <strong>{t('settings.mcp.new_token_title')}</strong>
                  <div class="row-actions">
                    <button
                      type="button"
                      class="btn btn--primary btn--xs"
                      onclick={() => copyQuick(antigravityConfigJson, 'JSON Antigravity dengan token baru disalin')}
                    >
                      <i class="ri-file-copy-line" aria-hidden="true"></i> {t('settings.mcp.copy_json')}
                    </button>
                    <button
                      type="button"
                      class="btn btn--secondary btn--xs"
                      onclick={() => copyQuick(mcpPlainToken, 'Token Bearer MCP disalin')}
                    >
                      <i class="ri-key-line" aria-hidden="true"></i> {t('common.copy')}
                    </button>
                    <button
                      type="button"
                      class="btn btn--ghost btn--xs"
                      onclick={() => (mcpPlainToken = '')}
                      aria-label={t('common.close')}
                      title={t('common.close')}
                    >
                      <i class="ri-close-line" aria-hidden="true"></i>
                    </button>
                  </div>
                </div>
                <p class="panel-desc">{t('settings.mcp.new_token_desc')}</p>
                <div class="copy-code-row">
                  <code class="mono">{mcpPlainToken}</code>
                </div>
              </div>
            {/if}

            <form class="form-grid-2" onsubmit={handleCreateMCPToken}>
              <div class="field">
                <label for="mcp-tok-name" class="field__label">{t('settings.mcp.token_name')}</label>
                <input
                  id="mcp-tok-name"
                  type="text"
                  class="input"
                  maxlength="60"
                  bind:value={mcpNewName}
                  placeholder={t('settings.mcp.token_name_placeholder')}
                  required
                />
              </div>
              <div class="field">
                <label for="mcp-tok-scope" class="field__label">{t('settings.mcp.token_scope')}</label>
                <select id="mcp-tok-scope" class="input" bind:value={mcpNewScope}>
                  <option value="readonly">{t('settings.mcp.scope_readonly')}</option>
                  <option value="operator">{t('settings.mcp.scope_operator')}</option>
                </select>
              </div>
              <div class="card-foot-actions">
                <button type="submit" class="btn btn--primary btn--sm" disabled={mcpCreating}>
                  {mcpCreating ? t('common.saving') : t('settings.mcp.create_btn')}
                </button>
              </div>
            </form>

            {#if mcpTokens.length === 0}
              <p class="panel-desc">{t('settings.mcp.empty_tokens')}</p>
            {:else}
              <div class="item-stack">
                {#each mcpTokens as tok (tok.id)}
                  <div class="list-row-item">
                    <div class="list-row-item__main">
                      <div class="list-row-item__title">
                        <strong>{tok.name}</strong>
                        <span class="mono text-subtle">{tok.token_prefix}</span>
                        <span class="badge {tok.scope === 'operator' ? 'badge--warning' : 'badge--info'}">
                          {tok.scope === 'operator' ? 'Operator' : 'Read-Only'}
                        </span>
                        <span class="badge {tok.enabled ? 'badge--success' : 'badge--muted'}">
                          {tok.enabled ? t('common.enabled') : t('common.disabled')}
                        </span>
                      </div>
                      <div class="list-row-item__sub mono">
                        {t('settings.mcp.calls', { count: tok.call_count })}
                        &middot;
                        {tok.last_used_at
                          ? `${formatUnix(tok.last_used_at)}${tok.last_used_ip ? ` (${tok.last_used_ip})` : ''}`
                          : t('settings.mcp.never_used')}
                      </div>
                    </div>
                    <div class="row-actions">
                      <button
                        type="button"
                        class="btn btn--ghost btn--xs"
                        onclick={() => handleToggleMCPToken(tok)}
                        disabled={mcpBusyId === tok.id}
                        aria-label={tok.enabled ? 'Nonaktifkan token MCP' : 'Aktifkan token MCP'}
                        title={tok.enabled ? 'Jeda Token' : 'Aktifkan Token'}
                      >
                        <i class={tok.enabled ? 'ri-pause-line' : 'ri-play-line'} aria-hidden="true"></i>
                      </button>
                      <button
                        type="button"
                        class="btn btn--ghost btn--xs btn--danger"
                        onclick={() => handleDeleteMCPToken(tok)}
                        disabled={mcpBusyId === tok.id}
                        aria-label="Cabut token MCP"
                        title="Cabut Token"
                      >
                        <i class="ri-delete-bin-line" aria-hidden="true"></i>
                      </button>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </div>

        <!-- Katalog Tools MCP -->
        <div class="card">
          <div class="card__head">
            <h3 class="card__title">
              <i class="ri-tools-line" aria-hidden="true"></i> {t('settings.mcp.tools_title')}
            </h3>
            <span class="badge badge--muted mono">{filteredMcpTools.length} / {mcpTools.length} tools</span>
          </div>
          <div class="card__body stack">
            <p class="panel-desc">{t('settings.mcp.tools_desc')}</p>
            <div class="mcp-filter-bar">
              <div class="pill-choice-row">
                <button
                  type="button"
                  class="choice-chip"
                  class:is-active={mcpToolFilter === 'all'}
                  onclick={() => (mcpToolFilter = 'all')}
                >
                  {t('settings.mcp.filter_all')} ({mcpTools.length})
                </button>
                <button
                  type="button"
                  class="choice-chip"
                  class:is-active={mcpToolFilter === 'readonly'}
                  onclick={() => (mcpToolFilter = 'readonly')}
                >
                  Read-Only ({mcpTools.filter((x) => x.scope === 'readonly').length})
                </button>
                <button
                  type="button"
                  class="choice-chip"
                  class:is-active={mcpToolFilter === 'operator'}
                  onclick={() => (mcpToolFilter = 'operator')}
                >
                  Operator ({mcpTools.filter((x) => x.scope === 'operator').length})
                </button>
              </div>
              <input
                type="search"
                class="input mcp-filter-search"
                bind:value={mcpToolQuery}
                placeholder={t('settings.mcp.filter_search')}
                aria-label={t('settings.mcp.filter_search')}
              />
            </div>
            <div class="mcp-tools-grid">
              {#each filteredMcpTools as tl (tl.name)}
                <div class="mcp-tool-card">
                  <div class="mcp-tool-card__head">
                    <code class="mcp-tool-card__name mono">{tl.name}</code>
                    <span class="badge {tl.scope === 'operator' ? 'badge--warning' : 'badge--info'}">
                      {tl.scope === 'operator' ? 'Operator' : 'Read-Only'}
                    </span>
                  </div>
                  <div class="mcp-tool-card__desc">{tl.description}</div>
                </div>
              {/each}
            </div>
          </div>
        </div>

        {#if mcpPrompts.length > 0}
          <!-- Katalog MCP Runbook Prompts -->
          <div class="card">
            <div class="card__head">
              <h3 class="card__title">
                <i class="ri-magic-line" aria-hidden="true"></i> {t('settings.mcp.prompts_title')}
              </h3>
              <span class="badge badge--muted mono">{mcpPrompts.length} prompts</span>
            </div>
            <div class="card__body stack">
              <p class="panel-desc">{t('settings.mcp.prompts_desc')}</p>
              <div class="mcp-tools-grid">
                {#each mcpPrompts as pr (pr.name)}
                  <div class="mcp-tool-card">
                    <div class="mcp-tool-card__head">
                      <code class="mcp-tool-card__name mono">{pr.name}</code>
                      <span class="badge badge--success">Prompt</span>
                    </div>
                    <div class="mcp-tool-card__desc">{pr.description}</div>
                  </div>
                {/each}
              </div>
            </div>
          </div>
        {/if}

      <!-- ═══ TAB 7: BACKUP SNAPSHOT SQLITE & MIGRASI PROFIL JSON (F) ═══ -->
      {:else if activeTab === 'backup'}
        <div class="two-col-cards">
          <div class="card">
            <div class="card__head">
              <h3 class="card__title">
                <i class="ri-database-2-line" aria-hidden="true"></i> {t('settings.backup.db_title')}
              </h3>
            </div>
            <div class="card__body stack">
              <p class="panel-desc">{t('settings.backup.db_desc')}</p>
              <div class="info-list">
                <div class="info-row">
                  <span class="info-label">{t('settings.backup.db_path')}</span>
                  <span class="mono info-val truncate max-w-220" title={sysInfo?.db_path}>{sysInfo?.db_path || '-'}</span>
                </div>
                <div class="info-row">
                  <span class="info-label">{t('settings.backup.db_size')}</span>
                  <span class="mono info-val">{formatBytes(sysInfo?.db_size_bytes || 0)}</span>
                </div>
              </div>
              <div class="card-foot-actions">
                <button type="button" class="btn btn--primary" onclick={() => triggerDownload('/api/settings/backup/db')}>
                  <i class="ri-download-2-line" aria-hidden="true"></i> {t('settings.backup.db_download')}
                </button>
              </div>
            </div>
          </div>

          <div class="card">
            <div class="card__head">
              <h3 class="card__title">
                <i class="ri-exchange-line" aria-hidden="true"></i> {t('settings.backup.migrate_title')}
              </h3>
            </div>
            <div class="card__body stack">
              <p class="panel-desc">{t('settings.backup.migrate_desc')}</p>
              <input
                bind:this={importFileEl}
                type="file"
                accept="application/json,.json"
                class="sr-only"
                onchange={handleImportFileSelected}
              />
              <div class="card-foot-actions">
                <button type="button" class="btn btn--primary" onclick={() => triggerDownload('/api/settings/config/export')}>
                  <i class="ri-file-download-line" aria-hidden="true"></i> {t('settings.backup.export_btn')}
                </button>
                <button type="button" class="btn btn--secondary" onclick={() => importFileEl?.click()} disabled={importingConfig}>
                  <i class="ri-file-upload-line" aria-hidden="true"></i> {importingConfig ? t('common.saving') : t('settings.backup.import_btn')}
                </button>
              </div>
            </div>
          </div>
        </div>
      {/if}
    </div>
  </div>

  <!-- Modal Konfirmasi Verifikasi Aksi 2FA (Nonaktifkan / Buat Ulang Kode Pemulihan) -->
  <Modal
    open={totpModalMode !== null}
    size="sm"
    title={totpModalMode === 'disable'
      ? t('settings.security.totp_disable_modal_title')
      : t('settings.security.totp_regen_title')}
    onclose={closeTotpModal}
  >
    <form id="totp-modal-form" class="modal-form-stack" onsubmit={submitTotpModal}>
      <div class="alert" class:alert--danger={totpModalMode === 'disable'} class:alert--warning={totpModalMode === 'regen'}>
        <i class={totpModalMode === 'disable' ? 'ri-shield-cross-line' : 'ri-key-2-line'} aria-hidden="true"></i>
        <span>
          {totpModalMode === 'disable'
            ? t('settings.security.totp_disable_modal_desc')
            : t('settings.security.totp_regen_desc')}
        </span>
      </div>

      <div class="field">
        <label for="totp-modal-secret" class="field__label">{t('settings.security.totp_verify_label')}</label>
        <input
          id="totp-modal-secret"
          type="password"
          class="input"
          bind:value={totpModalSecret}
          placeholder={t('settings.security.totp_verify_placeholder')}
          autocomplete="one-time-code"
          data-autofocus
          required
        />
      </div>
    </form>

    {#snippet footer()}
      <button type="button" class="btn btn--ghost" onclick={closeTotpModal} disabled={totpBusy}>
        {t('common.cancel')}
      </button>
      <button
        type="submit"
        form="totp-modal-form"
        class={totpModalMode === 'disable' ? 'btn btn--danger' : 'btn btn--primary'}
        disabled={totpBusy || !totpModalSecret.trim()}
      >
        {#if totpBusy}
          <span class="spinner spinner--sm"></span> {t('common.saving')}
        {:else if totpModalMode === 'disable'}
          {t('settings.security.totp_disable_btn')}
        {:else}
          {t('settings.security.totp_regen_btn')}
        {/if}
      </button>
    {/snippet}
  </Modal>

  <!-- Modal Pendaftaran Passkey / WebAuthn Baru -->
  <Modal
    open={showPasskeyModal}
    size="sm"
    title={t('settings.security.passkey_modal_title')}
    onclose={() => { if (!passkeyBusy) showPasskeyModal = false; }}
  >
    <form id="passkey-register-form" class="modal-form-stack" onsubmit={handleRegisterPasskey}>
      <div class="alert alert--info">
        <i class="ri-fingerprint-line" aria-hidden="true"></i>
        <span>{t('settings.security.passkey_modal_desc')}</span>
      </div>
      <div class="field">
        <label for="passkey-name-in" class="field__label">{t('settings.security.passkey_name_label')}</label>
        <input
          id="passkey-name-in"
          type="text"
          class="input"
          maxlength="60"
          bind:value={passkeyNameInput}
          placeholder={t('settings.security.passkey_name_placeholder')}
          data-autofocus
          required
        />
      </div>
    </form>

    {#snippet footer()}
      <button type="button" class="btn btn--ghost" onclick={() => (showPasskeyModal = false)} disabled={passkeyBusy}>
        {t('common.cancel')}
      </button>
      <button type="submit" form="passkey-register-form" class="btn btn--primary" disabled={passkeyBusy || !passkeyNameInput.trim()}>
        {#if passkeyBusy}
          <span class="spinner spinner--sm"></span> {t('common.loading')}
        {:else}
          <i class="ri-fingerprint-line" aria-hidden="true"></i> {t('settings.security.passkey_register_btn')}
        {/if}
      </button>
    {/snippet}
  </Modal>
</div>
