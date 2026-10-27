<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { apiFetch, apiFetchText } from '../api';
  import { confirmDialog } from '../stores/confirm.svelte';
  import ManagerModal from './ManagerModal.svelte';
  import type { DomainReport, ManagerTab, SiteDetail, SiteKind, SiteMeta } from '../types';

  interface Props {
    siteId: number;
    initialTab?: string;
    onClose: () => void;
    onChanged: () => void;
    onNotify: (msg: string) => void;
  }

  let { siteId, initialTab = 'ssl', onClose, onChanged, onNotify }: Props = $props();

  // Tab navigasi lengkap melampaui aaPanel & HestiaCP
  const TABS: ManagerTab[] = [
    { id: 'domain', label: 'Domain Manager', icon: 'ri-global-line' },
    { id: 'directory', label: 'Directory', icon: 'ri-folder-settings-line' },
    { id: 'limit_access', label: 'Limit Access', icon: 'ri-shield-keyhole-line' },
    { id: 'rewrite', label: 'URL Rewrite', icon: 'ri-shuffle-line' },
    { id: 'default_doc', label: 'Default Document', icon: 'ri-file-list-3-line' },
    { id: 'error_pages', label: 'Error Pages', icon: 'ri-file-warning-line' },
    { id: 'conf', label: 'Config', icon: 'ri-file-code-line' },
    { id: 'ssl', label: 'SSL', icon: 'ri-lock-line' },
    { id: 'php', label: 'PHP Version', icon: 'ri-terminal-window-line' },
    { id: 'proxy', label: 'Reverse Proxy', icon: 'ri-arrow-left-right-line' },
    { id: 'redirect', label: 'Redirect', icon: 'ri-guide-line' },
    { id: 'hotlink', label: 'Hotlink Protection', icon: 'ri-shield-check-line' },
    { id: 'maintenance', label: 'Maintenance Mode', icon: 'ri-tools-line' },
    { id: 'logs', label: 'Response Logs', icon: 'ri-file-text-line' }
  ];

  const STEPS = ['Memulai', 'Mendaftarkan akun ACME', 'Validasi HTTP-01 domain', 'Menerbitkan sertifikat', 'Menerapkan HTTPS ke Nginx', 'Selesai'];

  // State navigasi & detail situs
  let active = $state<string>(untrack(() => initialTab));
  let detail = $state<SiteDetail | null>(null);
  let meta = $state<SiteMeta>({ php_versions: [], server_ips: [] });
  let loadError = $state('');

  // 1. Tab Domain Manager
  let domainText = $state('');
  let domainReport = $state<DomainReport | null>(null);
  let domainChecking = $state(false);
  let domainSaving = $state(false);

  // 2. Tab Directory
  let siteDir = $state('');
  let runDir = $state('/public');
  let openBasedir = $state(true);
  let dirSaving = $state(false);

  // 3. Tab Limit Access
  let limitSubTab = $state<'auth' | 'deny' | 'traffic'>('auth');
  let limitPath = $state('/admin');
  let limitUser = $state('');
  let limitPass = $state('');
  let denyExt = $state('.env, .git, .sql, .bak, .yml, .yaml');
  let rateLimitEnabled = $state(false);
  let limitSaving = $state(false);

  // 4. Tab URL Rewrite
  let rewritePreset = $state('0.Current');
  let rewriteContent = $state('');
  let rewriteSaving = $state(false);

  // 5. Tab Default Document
  let defaultDocs = $state('index.php index.html index.htm default.html');
  let defaultDocSaving = $state(false);

  // Tab Error Pages
  let errorPreviewCode = $state('404');
  let errorPreviewHtml = $state('');
  let errorPreviewLoading = $state(false);

  // Memuat konten HTML template galat untuk pratinjau langsung
  async function loadErrorPreview(code: string) {
    errorPreviewCode = code;
    errorPreviewLoading = true;
    try {
      errorPreviewHtml = await apiFetchText(`/api/sites/error-pages/preview?code=${code}`);
    } catch (err) {
      errorPreviewHtml = `<div style="color:var(--danger-strong);padding:2rem;font-family:monospace;">Gagal terhubung: ${(err as Error).message}</div>`;
    } finally {
      errorPreviewLoading = false;
    }
  }

  // Muat pratinjau saat tab error_pages pertama kali dibuka
  $effect(() => {
    if (active === 'error_pages' && !errorPreviewHtml) {
      loadErrorPreview(errorPreviewCode);
    }
  });

  // 6. Tab Config (Raw Vhost)
  let vhost = $state('');
  let vhostLoading = $state(false);
  let vhostSaving = $state(false);

  // 7. Tab SSL
  let sslSubTab = $state<'current' | 'custom' | 'letsencrypt'>('current');
  let sslEnabled = $state(false);
  let sslForce = $state(true);
  let sslHSTS = $state(false);
  let skipDNS = $state(false);
  let sslSaving = $state(false);
  let dnsReport = $state<DomainReport | null>(null);
  let dnsLoading = $state(false);
  let customKey = $state('');
  let customCert = $state('');
  let customSaving = $state(false);
  let deployedKey = $state('');
  let deployedCert = $state('');

  // 8. Tab PHP Version
  let fPHP = $state('');
  let sessionIsolation = $state(false);
  let phpSaving = $state(false);

  // 9. Tab Reverse Proxy
  let fUpstream = $state('');
  let proxyWs = $state(true);
  let proxyTimeout = $state('60s');
  let proxySaving = $state(false);

  // 10. Tab Redirect
  let redirectType = $state('301');
  let redirectFrom = $state('/');
  let redirectTo = $state('');
  let redirectSaving = $state(false);

  // 11. Tab Hotlink Protection
  let hotlinkEnabled = $state(false);
  let hotlinkExts = $state('jpg, jpeg, png, gif, webp, svg, mp4');
  let hotlinkDomains = $state('');
  let hotlinkSaving = $state(false);

  // 12. Tab Maintenance Mode
  let maintenanceEnabled = $state(false);
  let maintenanceMsg = $state('Situs sedang dalam pemeliharaan berkala. Silakan kembali beberapa saat lagi.');
  let maintenanceSaving = $state(false);

  // 13. Tab Response Logs
  let logSubTab = $state<'access' | 'error'>('access');
  let logContent = $state('');
  let logLoading = $state(false);
  let logClearing = $state(false);

  let pollTimer: ReturnType<typeof setTimeout> | undefined;
  let domainTimer: ReturnType<typeof setTimeout> | undefined;
  let destroyed = false;

  let site = $derived(detail?.site);
  let cert = $derived(detail?.certificate);
  let job = $derived(detail?.ssl_job);
  let jobRunning = $derived(!!job?.running);
  let certIssued = $derived(cert?.status === 'issued');
  let certSANs = $derived.by<string[]>(() => {
    try {
      return JSON.parse(cert?.domains || '[]');
    } catch {
      return [];
    }
  });
  let daysLeft = $derived(cert?.not_after ? Math.floor((cert.not_after * 1000 - Date.now()) / 86400000) : 0);
  let dnsAllReady = $derived(!!dnsReport && dnsReport.dns.length > 0 && dnsReport.dns.every((d) => d.points_here));
  let hasWildcard = $derived((detail?.domains ?? []).some((d) => d.startsWith('*.')));

  // Sinkronkan form saat data diterima dari server
  function hydrate(d: SiteDetail) {
    detail = d;
    domainText = d.domains.join('\n');
    siteDir = d.site.root_path;
    if (d.site.root_path.endsWith('/public')) {
      runDir = '/public';
    } else if (d.site.root_path.endsWith('/public_html')) {
      runDir = '/public_html';
    } else if (d.site.root_path.endsWith('/dist')) {
      runDir = '/dist';
    } else if (d.site.root_path.endsWith('/build')) {
      runDir = '/build';
    } else {
      runDir = '/';
    }
    fUpstream = d.site.upstream || 'http://127.0.0.1:3000';
    fPHP = d.site.php_version || meta.php_versions[0] || '';
    sslEnabled = d.site.ssl_mode !== 'none';
    sslForce = d.site.force_https === 1;
    sslHSTS = d.site.hsts === 1;
    hotlinkDomains = d.domains.join(', ');
    rewriteContent = d.site.extra_conf || 'try_files $uri $uri/ /index.php?$query_string;';
  }

  async function load() {
    try {
      const d = await apiFetch<SiteDetail>(`/api/sites/${siteId}`);
      if (destroyed) return;
      loadError = '';
      hydrate(d);
      if (d.ssl_job?.running) schedulePoll();
    } catch (err) {
      loadError = (err as Error).message;
    }
  }

  // Polling status penerbitan SSL selama job berjalan di latar belakang
  function schedulePoll() {
    clearTimeout(pollTimer);
    pollTimer = setTimeout(async () => {
      try {
        const d = await apiFetch<SiteDetail>(`/api/sites/${siteId}`);
        if (destroyed) return;
        detail = d;
        if (d.ssl_job?.running) {
          schedulePoll();
          return;
        }
        hydrate(d);
        onChanged();
        onNotify(d.ssl_job?.ok ? `SSL ${d.site.domain} aktif` : `Penerbitan SSL gagal: ${d.ssl_job?.message || d.certificate?.last_error}`);
      } catch (err) {
        if (!destroyed) {
          onNotify(`Gagal memantau status SSL: ${(err as Error).message}`);
          schedulePoll();
        }
      }
    }, 1500);
  }

  async function checkDNS() {
    if (!detail) return;
    dnsLoading = true;
    try {
      dnsReport = await apiFetch<DomainReport>('/api/sites/check-domains', {
        method: 'POST',
        body: JSON.stringify({ domains: detail.domains.join('\n'), add_www: false, exclude_id: siteId, dns: true })
      });
    } catch (err) {
      onNotify(`Cek DNS gagal: ${(err as Error).message}`);
    } finally {
      dnsLoading = false;
    }
  }

  async function loadVhost() {
    vhostLoading = true;
    try {
      const r = await apiFetch<{ content: string }>(`/api/sites/${siteId}/vhost`);
      vhost = r.content;
    } catch (err) {
      vhost = `# Gagal memuat konfigurasi: ${(err as Error).message}`;
    } finally {
      vhostLoading = false;
    }
  }

  async function loadDeployedCerts() {
    try {
      const r = await apiFetch<{ cert: string; key: string }>(`/api/sites/${siteId}/ssl/certs`);
      deployedCert = r.cert || '/* Belum ada file sertifikat yang diterbitkan */';
      deployedKey = r.key || '/* Kunci privat tersimpan di server */';
    } catch {
      deployedCert = '';
      deployedKey = '';
    }
  }

  async function loadLogs(type: 'access' | 'error') {
    logLoading = true;
    try {
      const r = await apiFetch<{ content: string }>(`/api/sites/${siteId}/logs?type=${type}`);
      logContent = r.content || `(Log ${type} masih kosong)`;
    } catch (err) {
      logContent = `Gagal membaca log: ${(err as Error).message}`;
    } finally {
      logLoading = false;
    }
  }

  async function clearLogs() {
    if (!(await confirmDialog({
      title: 'Kosongkan log situs?',
      message: 'Semua riwayat catatan akses dan kesalahan untuk situs ini akan dibersihkan.',
      confirmText: 'Kosongkan',
      tone: 'danger'
    }))) return;
    logClearing = true;
    try {
      await apiFetch(`/api/sites/${siteId}/logs/clear`, { method: 'POST' });
      onNotify('Log akses dan error berhasil dikosongkan');
      await loadLogs(logSubTab);
    } catch (err) {
      onNotify(`Gagal membersihkan log: ${(err as Error).message}`);
    } finally {
      logClearing = false;
    }
  }

  $effect(() => {
    const tab = active;
    if (!detail) return;
    untrack(() => {
      if (tab === 'ssl') {
        if (!dnsReport && !dnsLoading) checkDNS();
        loadDeployedCerts();
      }
      if (tab === 'conf') loadVhost();
      if (tab === 'logs') loadLogs(logSubTab);
    });
  });

  onMount(() => {
    apiFetch<SiteMeta>('/api/sites/meta')
      .then((m) => {
        meta = m;
        if (!fPHP && m.php_versions.length) fPHP = m.php_versions[0];
      })
      .catch((err: Error) => onNotify(`Gagal memuat metadata: ${err.message}`));
    load();
    return () => {
      destroyed = true;
      clearTimeout(pollTimer);
      clearTimeout(domainTimer);
    };
  });

  // Susun payload PUT terpadu dengan nilai default yang aman
  function sitePayload(overrides: Record<string, unknown> = {}) {
    const s = detail!.site;
    return JSON.stringify({
      domains: detail!.domains.join('\n'),
      add_www: false,
      kind: s.kind,
      upstream: s.upstream,
      root_path: s.root_path,
      php_version: s.php_version,
      description: s.description,
      extra_conf: s.extra_conf,
      ...overrides
    });
  }

  function onDomainInput() {
    clearTimeout(domainTimer);
    domainChecking = true;
    domainTimer = setTimeout(async () => {
      try {
        domainReport = await apiFetch<DomainReport>('/api/sites/check-domains', {
          method: 'POST',
          body: JSON.stringify({ domains: domainText, add_www: false, exclude_id: siteId, dns: false })
        });
      } catch (err) {
        onNotify(`Validasi domain gagal: ${(err as Error).message}`);
      } finally {
        domainChecking = false;
      }
    }, 400);
  }

  async function saveDomains() {
    if (!detail) return;
    if (certIssued && !(await confirmDialog({
      title: 'Ubah domain situs?',
      message: 'Mengubah daftar domain akan mereset sertifikat SSL dan menonaktifkan HTTPS sampai sertifikat baru diterbitkan.',
      confirmText: 'Lanjutkan & Reset SSL',
      tone: 'danger',
    }))) return;
    domainSaving = true;
    try {
      const r = await apiFetch<{ ssl_reset: boolean }>(`/api/sites/${siteId}`, { method: 'PUT', body: sitePayload({ domains: domainText }) });
      onNotify(r.ssl_reset ? 'Domain diperbarui — terbitkan ulang SSL di tab SSL' : 'Domain diperbarui');
      domainReport = null;
      dnsReport = null;
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      domainSaving = false;
    }
  }

  async function saveDirectory() {
    if (!detail) return;
    dirSaving = true;
    try {
      let fullRoot = siteDir.trim();
      if (runDir !== '/' && !fullRoot.endsWith(runDir)) {
        fullRoot = fullRoot.replace(/\/+$/, '') + runDir;
      }
      await apiFetch(`/api/sites/${siteId}`, {
        method: 'PUT',
        body: sitePayload({ root_path: fullRoot })
      });
      onNotify('Direktori situs berhasil diperbarui & Nginx di-reload');
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      dirSaving = false;
    }
  }

  async function saveRewrite() {
    if (!detail) return;
    rewriteSaving = true;
    try {
      await apiFetch(`/api/sites/${siteId}`, {
        method: 'PUT',
        body: sitePayload({ extra_conf: rewriteContent.trim() })
      });
      onNotify('Aturan URL rewrite berhasil disimpan');
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      rewriteSaving = false;
    }
  }

  function applyRewritePreset(preset: string) {
    rewritePreset = preset;
    switch (preset) {
      case 'Laravel':
        rewriteContent = 'location / {\n    try_files $uri $uri/ /index.php?$query_string;\n}';
        break;
      case 'WordPress':
        rewriteContent = 'location / {\n    try_files $uri $uri/ /index.php?$args;\n}';
        break;
      case 'SPA':
        rewriteContent = 'location / {\n    try_files $uri $uri/ /index.html;\n}';
        break;
      case 'ThinkPHP':
        rewriteContent = 'location / {\n    if (!-e $request_filename) {\n        rewrite ^(.*)$ /index.php?s=$1 last;\n        break;\n    }\n}';
        break;
      case 'CodeIgniter':
        rewriteContent = 'location / {\n    try_files $uri $uri/ /index.php/$request_uri;\n}';
        break;
      case 'Nginx Default':
        rewriteContent = 'location / {\n    try_files $uri $uri/ =404;\n}';
        break;
      default:
        rewriteContent = detail?.site.extra_conf || 'try_files $uri $uri/ /index.php?$query_string;';
    }
  }

  async function saveDefaultDoc() {
    if (!detail) return;
    defaultDocSaving = true;
    try {
      const conf = `index ${defaultDocs.trim()};`;
      await apiFetch(`/api/sites/${siteId}`, {
        method: 'PUT',
        body: sitePayload({ extra_conf: conf })
      });
      onNotify('Dokumen bawaan disimpan');
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      defaultDocSaving = false;
    }
  }

  async function saveRawVhost() {
    if (!detail) return;
    vhostSaving = true;
    try {
      await apiFetch(`/api/sites/${siteId}/vhost`, {
        method: 'POST',
        body: JSON.stringify({ content: vhost })
      });
      onNotify('Konfigurasi vhost mentah berhasil disimpan & Nginx di-reload');
      await load();
      onChanged();
    } catch (err) {
      onNotify(`Gagal menerapkan vhost: ${(err as Error).message}`);
    } finally {
      vhostSaving = false;
    }
  }

  async function saveCustomSSL() {
    if (!detail) return;
    customSaving = true;
    try {
      await apiFetch(`/api/sites/${siteId}/ssl/custom`, {
        method: 'POST',
        body: JSON.stringify({ key: customKey.trim(), cert: customCert.trim() })
      });
      onNotify('Sertifikat kustom berhasil dipasang & HTTPS aktif');
      await load();
      await loadDeployedCerts();
      onChanged();
    } catch (err) {
      onNotify(`Gagal memasang sertifikat: ${(err as Error).message}`);
    } finally {
      customSaving = false;
    }
  }

  async function saveSSL(issue: boolean) {
    sslSaving = true;
    try {
      const r = await apiFetch<{ issuing: boolean }>(`/api/sites/${siteId}/ssl`, {
        method: 'POST',
        body: JSON.stringify({ enabled: sslEnabled, force_https: sslForce, hsts: sslHSTS && sslForce, issue, skip_dns: skipDNS })
      });
      await load();
      onChanged();
      if (r.issuing) {
        onNotify('Penerbitan sertifikat Let\'s Encrypt dimulai');
        schedulePoll();
      } else {
        onNotify(sslEnabled ? 'Pengaturan SSL disimpan' : 'SSL dinonaktifkan');
      }
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      sslSaving = false;
    }
  }

  async function switchPHP() {
    if (!detail) return;
    phpSaving = true;
    try {
      await apiFetch(`/api/sites/${siteId}`, {
        method: 'PUT',
        body: sitePayload({ kind: 'php', php_version: fPHP })
      });
      onNotify(`Versi PHP situs dialihkan ke PHP ${fPHP}`);
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      phpSaving = false;
    }
  }

  async function saveProxy() {
    if (!detail) return;
    proxySaving = true;
    try {
      await apiFetch(`/api/sites/${siteId}`, {
        method: 'PUT',
        body: sitePayload({ kind: 'proxy', upstream: fUpstream.trim() })
      });
      onNotify('Reverse proxy disimpan & Nginx di-reload');
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      proxySaving = false;
    }
  }

  async function saveRedirect() {
    if (!detail) return;
    redirectSaving = true;
    try {
      const rule = `rewrite ^${redirectFrom} ${redirectTo} ${redirectType === '301' ? 'permanent' : 'redirect'};`;
      await apiFetch(`/api/sites/${siteId}`, {
        method: 'PUT',
        body: sitePayload({ extra_conf: rule })
      });
      onNotify('Aturan pengalihan (redirect) berhasil disimpan');
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      redirectSaving = false;
    }
  }

  async function saveHotlink() {
    if (!detail) return;
    hotlinkSaving = true;
    try {
      let conf = '';
      if (hotlinkEnabled) {
        const exts = hotlinkExts.split(',').map((s) => s.trim()).filter(Boolean).join('|');
        conf = `location ~* \\.(${exts})$ {\n    valid_referers none blocked ${hotlinkDomains.replace(/,/g, ' ')};\n    if ($invalid_referer) {\n        return 403;\n    }\n}`;
      }
      await apiFetch(`/api/sites/${siteId}`, {
        method: 'PUT',
        body: sitePayload({ extra_conf: conf })
      });
      onNotify(hotlinkEnabled ? 'Perlindungan hotlink diaktifkan' : 'Perlindungan hotlink dinonaktifkan');
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      hotlinkSaving = false;
    }
  }

  async function saveMaintenance() {
    if (!detail) return;
    maintenanceSaving = true;
    try {
      let conf = '';
      if (maintenanceEnabled) {
        conf = `# Maintenance Mode\nreturn 503 "${maintenanceMsg.replace(/"/g, '\\"')}";`;
      }
      await apiFetch(`/api/sites/${siteId}`, {
        method: 'PUT',
        body: sitePayload({ extra_conf: conf })
      });
      onNotify(maintenanceEnabled ? 'Mode pemeliharaan diaktifkan' : 'Mode pemeliharaan dinonaktifkan');
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      maintenanceSaving = false;
    }
  }

  async function saveLimitAccess() {
    if (!detail) return;
    limitSaving = true;
    try {
      let conf = '';
      if (limitSubTab === 'auth') {
        conf = `# Limit Access Auth\nlocation ${limitPath.trim()} {\n    auth_basic "Area Terbatas";\n    auth_basic_user_file /etc/nginx/.htpasswd;\n}`;
      } else if (limitSubTab === 'deny') {
        const exts = denyExt.split(',').map((s) => s.trim().replace(/^\./, '')).filter(Boolean).join('|');
        conf = `# Deny Sensitive Files\nlocation ~* \\.(${exts})$ {\n    deny all;\n}`;
      } else if (limitSubTab === 'traffic' && rateLimitEnabled) {
        conf = `# Traffic Control Rate Limit\nlimit_req zone=one burst=20 nodelay;`;
      }
      await apiFetch(`/api/sites/${siteId}`, {
        method: 'PUT',
        body: sitePayload({ extra_conf: conf })
      });
      onNotify('Pengaturan pembatasan akses berhasil disimpan');
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      limitSaving = false;
    }
  }

  async function copyText(text: string, label: string) {
    try {
      await navigator.clipboard.writeText(text);
      onNotify(`${label} disalin`);
    } catch (err) {
      onNotify(`Gagal menyalin: ${(err as Error).message}`);
    }
  }

  function fmtDate(ts: number) {
    return ts ? new Date(ts * 1000).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';
  }
</script>

<ManagerModal
  title={site ? site.domain : 'Memuat…'}
  subtitle={site ? `${site.kind.toUpperCase()} · ${site.kind === 'proxy' ? site.upstream : site.root_path}` : ''}
  tabs={TABS}
  bind:active
  {onClose}
>
  {#snippet logo()}
    <span class="ss-logo"><i class="ri-global-line"></i></span>
  {/snippet}
  {#snippet badge()}
    {#if site}
      <span class="ss-badge" class:on={certIssued} class:pending={cert?.status === 'pending'}>
        <i class={certIssued ? 'ri-lock-fill' : 'ri-lock-unlock-line'}></i>
        {certIssued ? 'HTTPS' : cert?.status === 'pending' ? 'Menerbitkan…' : 'HTTP'}
      </span>
    {/if}
  {/snippet}

  {#snippet children(tab: string)}
    {#if loadError}
      <div class="ss-empty"><i class="ri-error-warning-line"></i><p>{loadError}</p><button class="btn-ghost" onclick={load}>Coba lagi</button></div>
    {:else if !detail}
      <div class="ss-empty"><i class="ri-loader-4-line spin"></i><p>Memuat data situs…</p></div>

    <!-- 1. TAB: DOMAIN MANAGER -->
    {:else if tab === 'domain'}
      <section class="ss-sec">
        <div class="sec-title">
          <h4>Domain Manager</h4>
          <p class="ss-desc">Kelola domain utama dan alias. Satu domain per baris. Baris pertama menjadi domain primer.</p>
        </div>
        <textarea id="ss-domains" rows="6" spellcheck="false" bind:value={domainText} oninput={onDomainInput}></textarea>
        {#if domainReport?.checks?.length}
          <ul class="chips">
            {#each domainReport.checks as c, i (c.input + i)}
              <li class="chip" class:bad={!!c.error}>
                <i class={c.error ? 'ri-close-circle-fill' : 'ri-checkbox-circle-fill ok'}></i>
                <span class="mono">{c.domain || c.input}</span>
                {#if domainReport.primary === c.domain}<b class="tag">utama</b>{/if}
                {#if c.error}<small>{c.error}</small>{/if}
              </li>
            {/each}
          </ul>
        {/if}
        {#if certIssued}
          <div class="note warn"><i class="ri-error-warning-line"></i><span>Perubahan domain akan mereset sertifikat SSL untuk menjaga integritas keamanan HTTPS.</span></div>
        {/if}
        <div class="ss-actions">
          <button class="btn-primary" id="ss-save-domains" onclick={saveDomains}
            disabled={domainSaving || domainChecking || (domainReport !== null && !domainReport.valid) || domainText.trim() === detail.domains.join('\n')}>
            {#if domainSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-save-line"></i>{/if} Simpan Domain
          </button>
        </div>
      </section>

    <!-- 2. TAB: DIRECTORY -->
    {:else if tab === 'directory'}
      <section class="ss-sec">
        <div class="sec-title">
          <h4>Site Directory & Subdirectory</h4>
          <p class="ss-desc">Konfigurasi lokasi fisik proyek di server dan direktori eksekusi (running directory) web server.</p>
        </div>
        <label class="fld" for="ss-site-dir"><span>Site Directory (Root Proyek)</span>
          <input id="ss-site-dir" class="mono" type="text" bind:value={siteDir} placeholder="/www/sites/domain.com" />
        </label>
        <label class="fld" for="ss-run-dir"><span>Running Directory (Folder Publik Nginx)</span>
          <select id="ss-run-dir" bind:value={runDir}>
            <option value="/">/ (Root Proyek)</option>
            <option value="/public">/public (Laravel, Static Assets, Vue/Vite)</option>
            <option value="/public_html">/public_html (Legacy Hosting / Hestia)</option>
            <option value="/dist">/dist (Frontend Build Output)</option>
            <option value="/build">/build (Production Bundle)</option>
            <option value="/app">/app (Node.js / Go / Python Core)</option>
          </select>
        </label>
        <label class="toggle-row" for="ss-open-basedir">
          <div><b>Anti-XSS / PHP open_basedir</b><small>Membatasi eksekusi PHP hanya di dalam direktori situs untuk mencegah akses file sistem lain</small></div>
          <input id="ss-open-basedir" type="checkbox" bind:checked={openBasedir} /><span class="track"></span>
        </label>
        <div class="meta-card">
          <div class="meta-row"><span>Owner Izin:</span><b class="mono">www-data : www-data</b></div>
          <div class="meta-row"><span>Hak Akses Linux:</span><b class="mono">0755 (Direktori) / 0644 (Berkas)</b></div>
        </div>
        <div class="ss-actions">
          <button class="btn-primary" onclick={saveDirectory} disabled={dirSaving}>
            {#if dirSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-save-line"></i>{/if} Simpan Direktori
          </button>
        </div>
      </section>

    <!-- 3. TAB: LIMIT ACCESS -->
    {:else if tab === 'limit_access'}
      <section class="ss-sec">
        <div class="sub-nav">
          <button class="sub-btn" class:active={limitSubTab === 'auth'} onclick={() => (limitSubTab = 'auth')}>Limit Access (Auth)</button>
          <button class="sub-btn" class:active={limitSubTab === 'deny'} onclick={() => (limitSubTab = 'deny')}>Deny Access (Files)</button>
          <button class="sub-btn" class:active={limitSubTab === 'traffic'} onclick={() => (limitSubTab = 'traffic')}>Traffic Control</button>
        </div>
        {#if limitSubTab === 'auth'}
          <label class="fld" for="ss-limit-path"><span>Path Yang Dilindungi</span>
            <input id="ss-limit-path" class="mono" type="text" bind:value={limitPath} placeholder="/admin, /api/secret" />
          </label>
          <label class="fld" for="ss-limit-user"><span>Username</span>
            <input id="ss-limit-user" type="text" bind:value={limitUser} placeholder="admin" />
          </label>
          <label class="fld" for="ss-limit-pass"><span>Password</span>
            <input id="ss-limit-pass" type="password" bind:value={limitPass} placeholder="••••••••" />
          </label>
          <p class="hint-text">Setelah diterapkan, pengunjung wajib memasukkan akun untuk mengakses path tersebut.</p>
        {:else if limitSubTab === 'deny'}
          <label class="fld" for="ss-deny-ext"><span>Ekstensi Berkas Terlarang (Deny All)</span>
            <input id="ss-deny-ext" class="mono" type="text" bind:value={denyExt} placeholder=".env, .git, .sql, .bak" />
          </label>
          <p class="hint-text">Mencegah pengunjung publik mendownload berkas sensitif atau kode sumber tersembunyi.</p>
        {:else}
          <label class="toggle-row" for="ss-traffic-on">
            <div><b>Traffic Control (Rate Limiting)</b><small>Membatasi frekuensi permintaan per IP untuk melindungi server dari serangan brute force dan flood</small></div>
            <input id="ss-traffic-on" type="checkbox" bind:checked={rateLimitEnabled} /><span class="track"></span>
          </label>
        {/if}
        <div class="ss-actions">
          <button class="btn-primary" onclick={saveLimitAccess} disabled={limitSaving}>
            {#if limitSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-shield-keyhole-line"></i>{/if} Terapkan Aturan
          </button>
        </div>
      </section>

    <!-- 4. TAB: URL REWRITE -->
    {:else if tab === 'rewrite'}
      <section class="ss-sec fill">
        <div class="conf-head">
          <div class="fld-h">
            <span class="label-inline">Preset Framework:</span>
            <select bind:value={rewritePreset} onchange={(e) => applyRewritePreset(e.currentTarget.value)}>
              <option value="0.Current">0. Current Configuration</option>
              <option value="Laravel">Laravel</option>
              <option value="WordPress">WordPress</option>
              <option value="SPA">SPA (React / Vue / Vite)</option>
              <option value="ThinkPHP">ThinkPHP</option>
              <option value="CodeIgniter">CodeIgniter</option>
              <option value="Nginx Default">Nginx Default</option>
            </select>
          </div>
          <button class="btn-mini" onclick={() => copyText(rewriteContent, 'Aturan Rewrite')}><i class="ri-file-copy-line"></i> Salin</button>
        </div>
        <textarea class="code-editor" rows="10" spellcheck="false" bind:value={rewriteContent}></textarea>
        <p class="hint-text">• Pilih template framework Anda atau tulis aturan kustom Nginx. Sintaks akan divalidasi otomatis saat disimpan.</p>
        <div class="ss-actions">
          <button class="btn-primary" onclick={saveRewrite} disabled={rewriteSaving}>
            {#if rewriteSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-save-line"></i>{/if} Simpan Rewrite
          </button>
        </div>
      </section>

    <!-- 5. TAB: DEFAULT DOCUMENT -->
    {:else if tab === 'default_doc'}
      <section class="ss-sec">
        <div class="sec-title">
          <h4>Default Document (Index Files)</h4>
          <p class="ss-desc">Tentukan urutan berkas pembuka utama situs saat sebuah direktori diakses.</p>
        </div>
        <label class="fld" for="ss-default-doc"><span>Daftar Berkas Index (Dipisahkan Spasi)</span>
          <input id="ss-default-doc" class="mono" type="text" bind:value={defaultDocs} />
        </label>
        <p class="hint-text">Contoh: index.php index.html index.htm default.html</p>
        <div class="ss-actions">
          <button class="btn-primary" onclick={saveDefaultDoc} disabled={defaultDocSaving}>
            {#if defaultDocSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-save-line"></i>{/if} Simpan Dokumen
          </button>
        </div>
      </section>

    <!-- TAB: ERROR PAGES & DIAGNOSTICS -->
    {:else if tab === 'error_pages'}
      <section class="ss-sec">
        <div class="sec-title">
          <h4>Halaman Galat (Error Pages) & Diagnostik</h4>
          <p class="ss-desc">Konfigurasi dan pratinjau halaman galat saat pengunjung menemui kendala (404, 403, 502, 503). Nginx otomatis memprioritaskan berkas proyek jika tersedia.</p>
        </div>

        <div class="error-info-card">
          <div class="eic-header">
            <i class="ri-information-line"></i>
            <b>Cara Menggunakan Halaman Kustom Sendiri</b>
          </div>
          <p class="eic-text">
            Secara default, FikPanel melayani template galat gelap modern dengan telemetri diagnostik. Jika proyek Anda memiliki halaman galat sendiri, cukup letakkan berkas di root proyek:
          </p>
          <div class="eic-tags">
            <code>/error/404.html</code>
            <code>/error/403.html</code>
            <code>/error/50x.html</code>
            <code>/404.html</code>
          </div>
        </div>

        <div class="ep-nav-row">
          <div class="sub-nav">
            <button class="sub-btn" class:active={errorPreviewCode === '404'} onclick={() => loadErrorPreview('404')}>404 Not Found</button>
            <button class="sub-btn" class:active={errorPreviewCode === '403'} onclick={() => loadErrorPreview('403')}>403 Forbidden</button>
            <button class="sub-btn" class:active={errorPreviewCode === '502'} onclick={() => loadErrorPreview('502')}>502 Bad Gateway</button>
            <button class="sub-btn" class:active={errorPreviewCode === '200'} onclick={() => loadErrorPreview('200')}>200 Welcome</button>
          </div>
          <a
            href="/api/sites/error-pages/preview?code={errorPreviewCode}"
            target="_blank"
            rel="noopener noreferrer"
            class="ep-link-btn"
            title="Buka pratinjau penuh di tab baru"
          >
            <i class="ri-external-link-line"></i> Buka di Tab Baru
          </a>
        </div>

        <div class="ep-frame-wrap">
          <iframe
            srcdoc={errorPreviewHtml}
            title="Pratinjau Halaman Galat"
            class="ep-iframe"
          ></iframe>
        </div>
      </section>

    <!-- 6. TAB: CONFIG (RAW VHOST) -->
    {:else if tab === 'conf'}
      <section class="ss-sec fill">
        <div class="conf-head">
          <div>
            <h4>Vhost Nginx Mentah</h4>
            <p class="ss-desc">Edit langsung file konfigurasi Nginx vhost. Sintaks akan diuji otomatis via nginx -t.</p>
          </div>
          <div class="btn-group">
            <button class="btn-mini" onclick={loadVhost} disabled={vhostLoading}><i class="ri-refresh-line" class:spin={vhostLoading}></i> Refresh</button>
            <button class="btn-mini" onclick={() => copyText(vhost, 'Vhost Nginx')} disabled={!vhost}><i class="ri-file-copy-line"></i> Salin</button>
          </div>
        </div>
        <textarea class="code-editor" rows="14" spellcheck="false" bind:value={vhost}></textarea>
        <div class="ss-actions">
          <button class="btn-primary" onclick={saveRawVhost} disabled={vhostSaving || vhostLoading}>
            {#if vhostSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-check-line"></i>{/if} Simpan & Terapkan Vhost
          </button>
        </div>
      </section>

    <!-- 7. TAB: SSL -->
    {:else if tab === 'ssl'}
      <section class="ss-sec">
        <div class="sub-nav">
          <button class="sub-btn" class:active={sslSubTab === 'current'} onclick={() => (sslSubTab = 'current')}>Current Certs - [Deployed]</button>
          <button class="sub-btn" class:active={sslSubTab === 'custom'} onclick={() => (sslSubTab = 'custom')}>Custom Certificate</button>
          <button class="sub-btn" class:active={sslSubTab === 'letsencrypt'} onclick={() => (sslSubTab = 'letsencrypt')}>Let's Encrypt</button>
        </div>

        {#if sslSubTab === 'current'}
          <div class="cert-overview-card">
            <div class="cov-item"><span>Tipe Sertifikat:</span><b>{detail.site.ssl_mode === 'custom' ? 'Custom Certificate' : 'Let\'s Encrypt TLS'}</b></div>
            <div class="cov-item"><span>Brand / Penerbit:</span><b>{cert?.issuer || '—'}</b></div>
            <div class="cov-item"><span>Domain(s):</span><b class="mono">{certSANs.join(', ') || detail.domains.join(', ')}</b></div>
            <div class="cov-item"><span>Masa Berlaku:</span><b class:warn={daysLeft < 30}>{fmtDate(cert?.not_after ?? 0)} ({daysLeft} hari lagi)</b></div>
          </div>
          <div class="sub mt">
            <label class="toggle-row" for="ss-force-cur">
              <div><b>Paksa HTTPS</b><small>Redirect 301 seluruh trafik HTTP port 80 ke HTTPS port 443</small></div>
              <input id="ss-force-cur" type="checkbox" bind:checked={sslForce} onchange={() => saveSSL(false)} /><span class="track"></span>
            </label>
            <label class="toggle-row" for="ss-hsts-cur" class:dim={!sslForce}>
              <div><b>HSTS (HTTP Strict Transport Security)</b><small>Browser mengingat koneksi aman HTTPS selama 1 tahun</small></div>
              <input id="ss-hsts-cur" type="checkbox" bind:checked={sslHSTS} disabled={!sslForce} onchange={() => saveSSL(false)} /><span class="track"></span>
            </label>
          </div>
          <div class="dual-boxes">
            <div class="box-col">
              <span class="box-head">Private Key (KEY)</span>
              <textarea class="code-editor-sm" rows="6" readonly value={deployedKey}></textarea>
            </div>
            <div class="box-col">
              <span class="box-head">Certificate (CRT/PEM)</span>
              <textarea class="code-editor-sm" rows="6" readonly value={deployedCert}></textarea>
            </div>
          </div>
          <div class="ss-actions">
            {#if sslEnabled}
              <button class="btn-ghost" onclick={() => { sslEnabled = false; saveSSL(false); }} disabled={sslSaving}>Nonaktifkan SSL</button>
            {/if}
          </div>

        {:else if sslSubTab === 'custom'}
          <div class="dual-boxes">
            <div class="box-col">
              <span class="box-head">Private Key (KEY) *</span>
              <textarea class="code-editor-sm" rows="8" placeholder="-----BEGIN PRIVATE KEY-----&#10;..." bind:value={customKey}></textarea>
            </div>
            <div class="box-col">
              <span class="box-head">Certificate (CRT/PEM) *</span>
              <textarea class="code-editor-sm" rows="8" placeholder="-----BEGIN CERTIFICATE-----&#10;..." bind:value={customCert}></textarea>
            </div>
          </div>
          <p class="hint-text">• Tempelkan konten kunci privat (.key) dan sertifikat gabungan (.crt / fullchain.pem). Server akan memverifikasi pasangan kunci secara instan.</p>
          <div class="ss-actions">
            <button class="btn-primary" onclick={saveCustomSSL} disabled={customSaving || !customKey.trim() || !customCert.trim()}>
              {#if customSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-save-line"></i>{/if} Simpan & Pasang Sertifikat
            </button>
          </div>

        {:else}
          <label class="toggle-row" for="ss-ssl-on">
            <div><b>Aktifkan Sertifikat Otomatis Let's Encrypt</b><small>Sertifikat TLS gratis, diperpanjang otomatis setiap 60 hari</small></div>
            <input id="ss-ssl-on" type="checkbox" bind:checked={sslEnabled} disabled={jobRunning} /><span class="track"></span>
          </label>
          {#if sslEnabled}
            <div class="dns-box mt">
              <div class="dns-head">
                <span><i class="ri-route-line"></i> Kesiapan DNS Sebelum Validasi ACME</span>
                <button class="btn-mini" onclick={checkDNS} disabled={dnsLoading}><i class="ri-refresh-line" class:spin={dnsLoading}></i> Cek ulang</button>
              </div>
              {#if dnsReport}
                <ul class="dns-list">
                  {#each dnsReport.dns as d (d.domain)}
                    <li>
                      <i class={d.points_here ? 'ri-checkbox-circle-fill ok' : 'ri-error-warning-fill warn'}></i>
                      <span class="mono">{d.domain}</span>
                      <small class="mono">{d.error ? 'tidak ter-resolve' : (d.ips ?? []).join(', ')}</small>
                    </li>
                  {/each}
                </ul>
                <p class="ip-note">IP server: <span class="mono">{dnsReport.server_ips.join(', ') || '—'}</span></p>
                {#if !dnsAllReady}
                  <label class="check" for="ss-skip-dns">
                    <input id="ss-skip-dns" type="checkbox" bind:checked={skipDNS} />
                    Lewati cek DNS (server di balik Cloudflare / CDN / Proxy)
                  </label>
                {/if}
              {:else}
                <p class="ip-note"><i class="ri-loader-4-line spin"></i> Memeriksa DNS…</p>
              {/if}
            </div>
          {/if}
          {#if jobRunning && job}
            <div class="job mt">
              {#each STEPS as st, i (st)}
                {@const cur = STEPS.indexOf(job.step)}
                <div class="step" class:done={i < cur} class:now={i === cur}>
                  <i class={i < cur ? 'ri-check-line' : i === cur ? 'ri-loader-4-line spin' : 'ri-checkbox-blank-circle-line'}></i>{st}
                </div>
              {/each}
            </div>
          {:else if job && !job.ok && job.message}
            <div class="note bad mt"><i class="ri-close-circle-line"></i><span>{job.message}</span></div>
          {/if}
          <div class="ss-actions">
            {#if certIssued && sslEnabled}
              <button class="btn-ghost" onclick={() => saveSSL(true)} disabled={sslSaving || jobRunning}><i class="ri-refresh-line"></i> Terbitkan Ulang</button>
            {/if}
            <button class="btn-primary" onclick={() => saveSSL(false)} disabled={sslSaving || jobRunning}>
              {#if sslSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-shield-check-line"></i>{/if}
              {sslEnabled && !certIssued ? 'Simpan & Terbitkan SSL' : 'Simpan'}
            </button>
          </div>
        {/if}
      </section>

    <!-- 8. TAB: PHP VERSION -->
    {:else if tab === 'php'}
      <section class="ss-sec">
        <div class="sec-title">
          <h4>PHP Version Switcher</h4>
          <p class="ss-desc">Alihkan versi PHP yang mengeksekusi skrip situs ini melalui soket fastcgi_pass.</p>
        </div>
        <label class="fld" for="ss-php-ver"><span>Versi PHP-FPM</span>
          <select id="ss-php-ver" bind:value={fPHP}>
            {#each meta.php_versions as v (v)}
              <option value={v}>PHP-{v.replace('.', '')} ({v})</option>
            {:else}
              <option value="">Default PHP-FPM</option>
            {/each}
          </select>
        </label>
        <label class="toggle-row" for="ss-session-iso">
          <div><b>Session Isolation</b><small>Simpan session di folder terisolasi milik situs ini untuk mencegah kebocoran antar-situs</small></div>
          <input id="ss-session-iso" type="checkbox" bind:checked={sessionIsolation} /><span class="track"></span>
        </label>
        <div class="ss-actions">
          <button class="btn-primary" onclick={switchPHP} disabled={phpSaving}>
            {#if phpSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-refresh-line"></i>{/if} Switch PHP Version
          </button>
        </div>
      </section>

    <!-- 9. TAB: REVERSE PROXY -->
    {:else if tab === 'proxy'}
      <section class="ss-sec">
        <div class="sec-title">
          <h4>Reverse Proxy (Node.js / Go / Python / Container)</h4>
          <p class="ss-desc">Teruskan lalu lintas Nginx ke aplikasi backend lokal atau remote target.</p>
        </div>
        <label class="fld" for="ss-upstream-url"><span>Target Upstream URL</span>
          <input id="ss-upstream-url" class="mono" type="text" bind:value={fUpstream} placeholder="http://127.0.0.1:3000" />
        </label>
        <label class="fld" for="ss-proxy-timeout"><span>Proxy Timeout</span>
          <select id="ss-proxy-timeout" bind:value={proxyTimeout}>
            <option value="60s">60 detik (Biasa)</option>
            <option value="120s">120 detik (Aplikasi Berat)</option>
            <option value="300s">300 detik (Long Polling / Stream)</option>
          </select>
        </label>
        <label class="toggle-row" for="ss-proxy-ws">
          <div><b>WebSocket Upgrade Support</b><small>Meneruskan header Connection $http_connection dan Upgrade $http_upgrade</small></div>
          <input id="ss-proxy-ws" type="checkbox" bind:checked={proxyWs} /><span class="track"></span>
        </label>
        <div class="ss-actions">
          <button class="btn-primary" onclick={saveProxy} disabled={proxySaving}>
            {#if proxySaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-save-line"></i>{/if} Simpan Reverse Proxy
          </button>
        </div>
      </section>

    <!-- 10. TAB: REDIRECT -->
    {:else if tab === 'redirect'}
      <section class="ss-sec">
        <div class="sec-title">
          <h4>URL Redirect</h4>
          <p class="ss-desc">Atur pengalihan alamat HTTP secara otomatis dengan kode respons 301 atau 302.</p>
        </div>
        <label class="fld" for="ss-redir-type"><span>Jenis Pengalihan (Redirect Type)</span>
          <select id="ss-redir-type" bind:value={redirectType}>
            <option value="301">301 (Permanent Redirect)</option>
            <option value="302">302 (Temporary Redirect)</option>
          </select>
        </label>
        <label class="fld" for="ss-redir-from"><span>Path Asal</span>
          <input id="ss-redir-from" class="mono" type="text" bind:value={redirectFrom} placeholder="/old-path atau /" />
        </label>
        <label class="fld" for="ss-redir-to"><span>URL Tujuan</span>
          <input id="ss-redir-to" class="mono" type="text" bind:value={redirectTo} placeholder="https://domain-baru.com$request_uri" />
        </label>
        <div class="ss-actions">
          <button class="btn-primary" onclick={saveRedirect} disabled={redirectSaving || !redirectTo.trim()}>
            {#if redirectSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-guide-line"></i>{/if} Simpan Pengalihan
          </button>
        </div>
      </section>

    <!-- 11. TAB: HOTLINK PROTECTION -->
    {:else if tab === 'hotlink'}
      <section class="ss-sec">
        <div class="sec-title">
          <h4>Hotlink Protection</h4>
          <p class="ss-desc">Cegah website eksternal mencuri bandwidth dengan menautkan langsung gambar dan media dari situs Anda.</p>
        </div>
        <label class="toggle-row" for="ss-hotlink-on">
          <div><b>Aktifkan Anti-Hotlink</b><small>Hanya izinkan referrer dari domain terdaftar di bawah</small></div>
          <input id="ss-hotlink-on" type="checkbox" bind:checked={hotlinkEnabled} /><span class="track"></span>
        </label>
        <label class="fld" for="ss-hotlink-ext"><span>Ekstensi Yang Dilindungi</span>
          <input id="ss-hotlink-ext" class="mono" type="text" bind:value={hotlinkExts} />
        </label>
        <label class="fld" for="ss-hotlink-dom"><span>Whitelist Domain (Pisahkan Koma)</span>
          <input id="ss-hotlink-dom" class="mono" type="text" bind:value={hotlinkDomains} />
        </label>
        <div class="ss-actions">
          <button class="btn-primary" onclick={saveHotlink} disabled={hotlinkSaving}>
            {#if hotlinkSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-shield-check-line"></i>{/if} Simpan Hotlink
          </button>
        </div>
      </section>

    <!-- 12. TAB: MAINTENANCE MODE -->
    {:else if tab === 'maintenance'}
      <section class="ss-sec">
        <div class="sec-title">
          <h4>Maintenance Mode</h4>
          <p class="ss-desc">Kembalikan status HTTP 503 saat sedang melakukan pembaruan sistem atau migrasi berkas.</p>
        </div>
        <label class="toggle-row" for="ss-maint-on">
          <div><b>Aktifkan Mode Pemeliharaan</b><small>Pengunjung akan menerima pesan 503 Service Unavailable</small></div>
          <input id="ss-maint-on" type="checkbox" bind:checked={maintenanceEnabled} /><span class="track"></span>
        </label>
        <label class="fld" for="ss-maint-msg"><span>Pesan Pemeliharaan</span>
          <textarea id="ss-maint-msg" rows="4" bind:value={maintenanceMsg}></textarea>
        </label>
        <div class="ss-actions">
          <button class="btn-primary" onclick={saveMaintenance} disabled={maintenanceSaving}>
            {#if maintenanceSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-tools-line"></i>{/if} Terapkan Status
          </button>
        </div>
      </section>

    <!-- 13. TAB: RESPONSE LOGS -->
    {:else if tab === 'logs'}
      <section class="ss-sec fill">
        <div class="conf-head">
          <div class="sub-nav compact">
            <button class="sub-btn" class:active={logSubTab === 'access'} onclick={() => { logSubTab = 'access'; loadLogs('access'); }}>Access Log</button>
            <button class="sub-btn" class:active={logSubTab === 'error'} onclick={() => { logSubTab = 'error'; loadLogs('error'); }}>Error Log</button>
          </div>
          <div class="btn-group">
            <button class="btn-mini" onclick={() => loadLogs(logSubTab)} disabled={logLoading}><i class="ri-refresh-line" class:spin={logLoading}></i> Refresh</button>
            <button class="btn-mini" onclick={clearLogs} disabled={logClearing}><i class="ri-delete-bin-line"></i> Kosongkan</button>
            <button class="btn-mini" onclick={() => copyText(logContent, `Log ${logSubTab}`)}><i class="ri-file-copy-line"></i> Salin</button>
          </div>
        </div>
        <pre class="log-viewer">{logLoading ? 'Memuat log real-time…' : logContent}</pre>
      </section>
    {/if}
  {/snippet}
</ManagerModal>

<style>
  .ss-logo { width: 34px; height: 34px; border-radius: 9px; display: grid; place-items: center; background: color-mix(in srgb, var(--accent) 12%, transparent); color: var(--accent-strong); font-size: 1.1rem; }
  .ss-badge { display: inline-flex; align-items: center; gap: 0.3rem; padding: 0.2rem 0.55rem; border-radius: 99px; font-size: 0.7rem; font-weight: 600; background: var(--bg-active); color: var(--text-muted); }
  .ss-badge.on { background: color-mix(in srgb, var(--accent) 14%, transparent); color: var(--accent-strong); }
  .ss-badge.pending { background: color-mix(in srgb, var(--warning) 14%, transparent); color: var(--warning-strong); }
  .ss-empty { height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 0.6rem; color: var(--text-subtle); }
  .ss-empty i { font-size: 1.8rem; }
  .ss-sec { display: flex; flex-direction: column; gap: 0.85rem; max-width: 760px; }
  .ss-sec.fill { max-width: none; height: 100%; }
  .sec-title h4 { margin: 0; font-size: 0.95rem; color: var(--text); }
  .ss-desc { margin: 0.2rem 0 0; font-size: 0.76rem; color: var(--text-subtle); line-height: 1.45; }
  .hint-text { margin: -0.3rem 0 0; font-size: 0.73rem; color: var(--text-subtle); line-height: 1.4; }
  .ss-actions { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 0.2rem; }
  .mono { font-family: 'Fira Code', monospace; }
  .mt { margin-top: 0.6rem; }

  textarea, input[type='text'], input[type='password'], select {
    width: 100%; box-sizing: border-box; padding: 0.55rem 0.75rem; background: var(--bg-surface); border: 1px solid var(--border);
    border-radius: 8px; color: var(--text); font-size: 0.83rem; transition: border-color 0.15s ease, box-shadow 0.15s ease;
  }
  textarea { font-family: 'Fira Code', monospace; resize: vertical; line-height: 1.5; }
  textarea:focus, input[type='text']:focus, input[type='password']:focus, select:focus { outline: none; border-color: var(--accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 15%, transparent); }
  .fld { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.76rem; font-weight: 600; color: var(--text-secondary); }
  .fld-h { display: flex; align-items: center; gap: 0.6rem; }
  .label-inline { font-size: 0.78rem; font-weight: 600; color: var(--text-secondary); white-space: nowrap; }

  .sub-nav { display: flex; gap: 0.3rem; padding: 3px; background: var(--bg-inset); border: 1px solid var(--border); border-radius: 8px; width: fit-content; }
  .sub-nav.compact { padding: 2px; }
  .sub-btn { padding: 0.35rem 0.75rem; font-size: 0.75rem; font-weight: 600; border: none; border-radius: 6px; background: transparent; color: var(--text-muted); cursor: pointer; transition: all 0.15s ease; }
  .sub-btn:hover { color: var(--text); }
  .sub-btn.active { background: var(--bg-surface); color: var(--accent-strong); box-shadow: 0 2px 6px color-mix(in srgb, var(--bg-app) 50%, transparent); }

  .chips { list-style: none; margin: 0; padding: 0; display: flex; flex-wrap: wrap; gap: 0.4rem; }
  .chip { display: inline-flex; align-items: center; gap: 0.35rem; padding: 0.3rem 0.55rem; border-radius: 7px; background: var(--bg-raised); border: 1px solid var(--border); font-size: 0.74rem; color: var(--text-secondary); }
  .chip.bad { border-color: color-mix(in srgb, var(--danger-strong) 40%, transparent); color: var(--danger-strong); }
  .chip.bad i { color: var(--danger-strong); }
  .chip small { color: var(--danger-strong); }
  .tag { font-size: 0.62rem; padding: 0.05rem 0.35rem; border-radius: 4px; background: color-mix(in srgb, var(--accent) 12%, transparent); color: var(--accent-strong); text-transform: uppercase; }
  i.ok { color: var(--accent); }
  i.warn { color: var(--warning); }

  .toggle-row { position: relative; display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 0.75rem 0.9rem; border-radius: 10px; background: var(--bg-raised); border: 1px solid var(--border); cursor: pointer; }
  .toggle-row div { display: flex; flex-direction: column; gap: 0.15rem; }
  .toggle-row b { font-size: 0.83rem; color: var(--text); }
  .toggle-row small { font-size: 0.72rem; color: var(--text-subtle); }
  .toggle-row.dim { opacity: 0.5; }
  .toggle-row input { position: absolute; opacity: 0; pointer-events: none; }
  .track { width: 36px; height: 20px; border-radius: 99px; background: var(--bg-active); position: relative; flex-shrink: 0; transition: background 0.15s ease; }
  .track::after { content: ''; position: absolute; top: 3px; left: 3px; width: 14px; height: 14px; border-radius: 50%; background: var(--text-secondary); transition: transform 0.18s ease, background 0.18s ease; }
  .toggle-row input:checked + .track { background: color-mix(in srgb, var(--accent) 35%, transparent); }
  .toggle-row input:checked + .track::after { transform: translateX(16px); background: var(--accent-strong); }
  .sub { display: flex; flex-direction: column; gap: 0.5rem; padding-left: 1rem; border-left: 2px solid var(--border); }

  .meta-card { display: flex; flex-direction: column; gap: 0.35rem; padding: 0.75rem 0.9rem; border-radius: 9px; background: var(--bg-inset); border: 1px solid var(--border); font-size: 0.78rem; }
  .meta-row { display: flex; justify-content: space-between; align-items: center; }
  .meta-row span { color: var(--text-subtle); }
  .meta-row b { color: var(--text-secondary); }

  .error-info-card {
    display: flex; flex-direction: column; gap: 0.5rem; padding: 0.85rem 1rem;
    border-radius: 9px; background: color-mix(in srgb, var(--info) 6%, transparent);
    border: 1px solid color-mix(in srgb, var(--info) 20%, transparent);
    color: var(--text-secondary); font-size: 0.78rem; line-height: 1.5;
  }
  .eic-header { display: flex; align-items: center; gap: 0.45rem; color: var(--info); font-size: 0.82rem; }
  .eic-text { margin: 0; color: var(--text-muted); }
  .eic-tags { display: flex; gap: 0.4rem; flex-wrap: wrap; margin-top: 0.2rem; }
  .eic-tags code {
    font-family: 'Fira Code', monospace; font-size: 0.72rem; padding: 0.15rem 0.45rem;
    border-radius: 4px; background: var(--bg-surface); border: 1px solid var(--border);
    color: var(--accent-strong);
  }
  .ep-nav-row { display: flex; align-items: center; justify-content: space-between; gap: 0.8rem; flex-wrap: wrap; }
  .ep-link-btn {
    display: inline-flex; align-items: center; gap: 0.35rem; font-size: 0.75rem; font-weight: 600;
    color: var(--accent-strong); text-decoration: none; padding: 0.35rem 0.65rem; border-radius: 6px;
    background: var(--bg-raised); border: 1px solid var(--border); transition: all 0.15s ease;
  }
  .ep-link-btn:hover { background: var(--bg-active); border-color: var(--accent); }
  .ep-frame-wrap {
    width: 100%; height: 520px; border-radius: 9px; overflow: hidden;
    border: 1px solid var(--border); background: var(--bg-app);
  }
  .ep-iframe { width: 100%; height: 100%; border: none; display: block; }

  .cert-overview-card { display: grid; grid-template-columns: 1fr 1fr; gap: 0.6rem; padding: 0.85rem; border-radius: 10px; background: var(--bg-surface); border: 1px solid var(--border); font-size: 0.78rem; }
  .cov-item { display: flex; flex-direction: column; gap: 0.15rem; }
  .cov-item span { font-size: 0.72rem; color: var(--text-subtle); }
  .cov-item b { color: var(--accent-strong); word-break: break-all; }
  .cov-item b.warn { color: var(--warning-strong); }

  .dual-boxes { display: grid; grid-template-columns: 1fr 1fr; gap: 0.8rem; margin-top: 0.4rem; }
  .box-col { display: flex; flex-direction: column; gap: 0.3rem; }
  .box-head { font-size: 0.74rem; font-weight: 600; color: var(--text-secondary); }

  .dns-box { padding: 0.8rem 0.9rem; border-radius: 10px; border: 1px dashed var(--border-strong); display: flex; flex-direction: column; gap: 0.5rem; }
  .dns-head { display: flex; justify-content: space-between; align-items: center; font-size: 0.8rem; font-weight: 600; color: var(--text-secondary); }
  .dns-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.3rem; }
  .dns-list li { display: flex; align-items: center; gap: 0.45rem; font-size: 0.78rem; color: var(--text); }
  .dns-list small { color: var(--text-subtle); margin-left: auto; }
  .ip-note { margin: 0; font-size: 0.72rem; color: var(--text-subtle); }
  .check { display: flex; align-items: center; gap: 0.45rem; font-size: 0.75rem; color: var(--warning-strong); cursor: pointer; }
  .check input { accent-color: var(--warning); }

  .job { display: flex; flex-direction: column; gap: 0.35rem; padding: 0.8rem 0.9rem; border-radius: 10px; background: color-mix(in srgb, var(--accent) 5%, transparent); border: 1px solid color-mix(in srgb, var(--accent) 20%, transparent); }
  .step { display: flex; align-items: center; gap: 0.5rem; font-size: 0.78rem; color: var(--text-disabled); transition: color 0.2s ease; }
  .step.done { color: var(--accent-strong); }
  .step.now { color: var(--text); font-weight: 600; }

  .note { display: flex; gap: 0.55rem; padding: 0.65rem 0.8rem; border-radius: 9px; font-size: 0.76rem; line-height: 1.5; }
  .note.warn { background: color-mix(in srgb, var(--warning) 7%, transparent); border: 1px solid color-mix(in srgb, var(--warning) 25%, transparent); color: var(--warning-strong); }
  .note.bad { background: color-mix(in srgb, var(--danger-strong) 7%, transparent); border: 1px solid color-mix(in srgb, var(--danger-strong) 25%, transparent); color: var(--danger-strong); }

  .conf-head { display: flex; justify-content: space-between; align-items: center; gap: 0.8rem; }
  .conf-head h4 { margin: 0; font-size: 0.92rem; color: var(--text); }
  .btn-group { display: flex; gap: 0.4rem; }

  .code-editor { flex: 1; min-height: 240px; margin: 0; padding: 0.85rem 1rem; border-radius: 9px; background: var(--bg-inset); border: 1px solid var(--border); color: var(--accent-strong); font: 0.78rem/1.6 'Fira Code', monospace; white-space: pre; }
  .code-editor-sm { margin: 0; padding: 0.6rem 0.8rem; border-radius: 8px; background: var(--bg-inset); border: 1px solid var(--border); color: var(--accent-strong); font: 0.72rem/1.5 'Fira Code', monospace; white-space: pre; }
  .log-viewer { flex: 1; min-height: 280px; margin: 0; padding: 0.85rem 1rem; overflow: auto; border-radius: 9px; background: var(--bg-inset); border: 1px solid var(--border); color: var(--text-secondary); font: 0.74rem/1.6 'Fira Code', monospace; white-space: pre-wrap; word-break: break-all; }

  .btn-primary, .btn-ghost, .btn-mini { display: inline-flex; align-items: center; gap: 0.4rem; cursor: pointer; font-weight: 600; transition: all 0.15s ease; }
  .btn-primary { padding: 0.55rem 1rem; border: none; border-radius: 8px; font-size: 0.82rem; color: var(--text-on-accent); background: linear-gradient(135deg, var(--accent-strong), var(--accent)); box-shadow: 0 6px 18px -6px color-mix(in srgb, var(--accent) 60%, transparent); }
  .btn-primary:hover:not(:disabled) { transform: translateY(-1px); }
  .btn-primary:disabled, .btn-ghost:disabled, .btn-mini:disabled { opacity: 0.45; cursor: not-allowed; box-shadow: none; }
  .btn-ghost { padding: 0.55rem 1rem; border-radius: 8px; font-size: 0.82rem; background: transparent; border: 1px solid var(--border); color: var(--text-muted); }
  .btn-ghost:hover:not(:disabled) { color: var(--text); border-color: var(--border-strong); }
  .btn-mini { padding: 0.3rem 0.6rem; border-radius: 6px; font-size: 0.72rem; background: var(--bg-raised); border: 1px solid var(--border); color: var(--text-muted); }
  .btn-mini:hover:not(:disabled) { color: var(--accent-strong); border-color: color-mix(in srgb, var(--accent) 40%, transparent); }

  .spin { display: inline-block; animation: spin 0.8s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>
