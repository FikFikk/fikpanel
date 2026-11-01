<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch, formatBytes } from '../api';
  import { copyText as copyToClipboard } from '../utils/browser';
  import type { DomainReport, Site, SiteKind, SiteMeta } from '../types';
  import Modal from './ui/Modal.svelte';
  import '../../styles/pages/components/add-site-modal.css';

  interface Props {
    onClose: () => void;
    onCreated: (site: Site) => void;
    onOpenSSL: (site: Site) => void;
    onNotify: (msg: string) => void;
  }

  let { onClose, onCreated, onOpenSSL, onNotify }: Props = $props();

  // Navigasi 5 Tab Site Builder (mengungguli 5 tab aaPanel)
  type BuilderTab = 'create' | 'deploy' | 'git' | 'batch' | 'backup';

  const BUILDER_TABS: { id: BuilderTab; label: string; sub: string; icon: string }[] = [
    { id: 'create', label: 'Create site', sub: 'Multi-Runtime', icon: 'ri-global-line' },
    { id: 'deploy', label: 'One-click Deploy', sub: 'Template Siap Pakai', icon: 'ri-rocket-2-line' },
    { id: 'git', label: 'Create for Git', sub: 'GitHub & SSH Clone', icon: 'ri-git-branch-line' },
    { id: 'batch', label: 'Batch create', sub: 'Buat Massal', icon: 'ri-stack-line' },
    { id: 'backup', label: 'Create from backup', sub: 'Pulihkan Arsip', icon: 'ri-archive-drawer-line' }
  ];

  // Pilihan jenis runtime situs
  const KINDS: { id: SiteKind; label: string; desc: string; icon: string }[] = [
    { id: 'php', label: 'PHP', desc: 'Laravel, WordPress, CMS PHP', icon: 'ri-code-s-slash-line' },
    { id: 'node', label: 'Node.js', desc: 'Next.js, Express, Nuxt, Nest', icon: 'ri-nodejs-line' },
    { id: 'go', label: 'Golang', desc: 'Gin, Fiber, binary app', icon: 'ri-terminal-box-line' },
    { id: 'python', label: 'Python', desc: 'Django, FastAPI, Flask', icon: 'ri-code-box-line' },
    { id: 'static', label: 'Static', desc: 'HTML, SPA React/Vue build', icon: 'ri-html5-line' },
    { id: 'proxy', label: 'Custom Proxy', desc: 'Forward port / Docker upstream', icon: 'ri-share-forward-line' }
  ];

  const UPSTREAM_PRESETS = [
    { label: 'Node :3000', url: 'http://127.0.0.1:3000' },
    { label: 'Go :8080', url: 'http://127.0.0.1:8080' },
    { label: 'Python :8000', url: 'http://127.0.0.1:8000' },
    { label: 'Custom :5000', url: 'http://127.0.0.1:5000' }
  ];

  // Pilihan template One-Click Deploy
  const DEPLOY_TEMPLATES: {
    id: string;
    name: string;
    version: string;
    desc: string;
    kind: SiteKind;
    defaultUpstream?: string;
    needsDB: boolean;
    icon: string;
    badge: string;
  }[] = [
    {
      id: 'laravel',
      name: 'Laravel Starter',
      version: 'Laravel 11.x (PHP-FPM)',
      desc: 'Struktur public/index.php siap pakai beserta injeksi .env database MySQL otomatis.',
      kind: 'php',
      needsDB: true,
      icon: 'ri-code-s-slash-line',
      badge: 'PHP'
    },
    {
      id: 'wordpress',
      name: 'WordPress CMS',
      version: 'WordPress Latest',
      desc: 'Siapkan struktur WordPress beserta file konfigurasi wp-config-sample-fikpanel.php otomatis.',
      kind: 'php',
      needsDB: true,
      icon: 'ri-article-line',
      badge: 'PHP'
    },
    {
      id: 'express',
      name: 'Node.js Web / API',
      version: 'Node.js HTTP / Express (:3000)',
      desc: 'Proyek Node.js siap jalan dengan reverse proxy otomatis ke http://127.0.0.1:3000.',
      kind: 'node',
      defaultUpstream: 'http://127.0.0.1:3000',
      needsDB: false,
      icon: 'ri-nodejs-line',
      badge: 'Node.js'
    },
    {
      id: 'gin',
      name: 'Golang HTTP Service',
      version: 'Go 1.22+ HTTP (:8080)',
      desc: 'Proyek Golang performa tinggi dengan reverse proxy otomatis ke http://127.0.0.1:8080.',
      kind: 'go',
      defaultUpstream: 'http://127.0.0.1:8080',
      needsDB: false,
      icon: 'ri-terminal-box-line',
      badge: 'Golang'
    },
    {
      id: 'fastapi',
      name: 'Python FastAPI / WSGI',
      version: 'FastAPI + Uvicorn (:8000)',
      desc: 'Proyek Python web service dengan reverse proxy otomatis ke http://127.0.0.1:8000.',
      kind: 'python',
      defaultUpstream: 'http://127.0.0.1:8000',
      needsDB: false,
      icon: 'ri-code-box-line',
      badge: 'Python'
    },
    {
      id: 'spa',
      name: 'Modern Static / SPA',
      version: 'HTML5 + SPA Routing',
      desc: 'Landing page responsif dengan routing fallback index.html untuk React, Vue, atau Svelte.',
      kind: 'static',
      needsDB: false,
      icon: 'ri-html5-line',
      badge: 'Static'
    }
  ];

  // State tab & form utama
  let activeTab = $state<BuilderTab>('create');
  let domains = $state('');
  let addWWW = $state(true);
  let autoSSL = $state(false);
  let description = $state('');
  let kind = $state<SiteKind>('php');
  let user = $state('fikpanel');
  let upstream = $state('http://127.0.0.1:3000');
  let rootPath = $state('');
  let rootTouched = $state(false);
  let phpVersion = $state('');

  // State provisioning database MySQL otomatis
  let createDB = $state(false);
  let dbName = $state('');
  let dbUser = $state('');
  let dbPass = $state('');
  let dbTouched = $state(false);

  // State provisioning akun Super Admin otomatis (Laravel / Filament)
  let createAdmin = $state(true);
  let adminName = $state('Super Admin');
  let adminEmail = $state('');
  let adminPass = $state('admin12345');
  let adminEmailTouched = $state(false);
  let isFilamentRepo = $state(false);
  let isLaravelRepo = $state(false);

  // State One-Click Deploy
  let selectedTemplate = $state('laravel');

  // State Git / GitHub Integration
  interface GitStatus {
    connected: boolean;
    mode?: string;
    username?: string;
    name?: string;
    avatar_url?: string;
    scopes?: string;
  }
  interface GitRepoItem {
    name: string;
    full_name: string;
    private: boolean;
    html_url: string;
    clone_url: string;
    ssh_url: string;
    default_branch: string;
    description: string;
    language: string;
    updated_at: string;
  }

  let gitSource = $state<'github' | 'url'>('github');
  let gitStatus = $state<GitStatus>({ connected: false });
  let avatarBroken = $state(false);
  let gitConnectOpen = $state(false);
  let gitAuthMode = $state<'pat' | 'device' | 'public'>('pat');
  let gitTokenInput = $state('');
  let gitPublicUser = $state('');
  let gitClientId = $state('');
  let gitDeviceFlow = $state<{
    device_code: string;
    user_code: string;
    verification_uri: string;
    interval: number;
  } | null>(null);
  let gitDevicePolling = $state(false);
  let gitBusy = $state(false);
  let gitError = $state('');
  let gitRepos = $state<GitRepoItem[]>([]);
  let gitRepoFilter = $state('');
  let repoDropdownOpen = $state(false);
  let selectedGitRepoFull = $state('');
  let gitBranches = $state<string[]>([]);
  let gitRepoURL = $state('');
  let gitBranch = $state('main');
  let gitPostCmd = $state('');
  let sshPublicKey = $state('');
  let sshKeyLoading = $state(false);
  let sshKeyCopied = $state(false);

  // State Batch Create
  let batchLines = $state('');
  let batchResult = $state<{
    ok: boolean;
    created_count: number;
    total_lines: number;
    results: { line: string; domain: string; kind: string; ok: boolean; message: string }[];
  } | null>(null);

  // State Backup Restore
  interface BackupArchiveItem {
    name: string;
    path: string;
    size: number;
    mod_time: string;
  }
  let backupArchives = $state<BackupArchiveItem[]>([]);
  let backupLoading = $state(false);
  let selectedBackupArchive = $state('');
  let backupUploading = $state(false);

  // State validasi domain & meta
  let meta = $state<SiteMeta>({ php_versions: [], server_ips: [] });
  let report = $state<DomainReport | null>(null);
  let checking = $state(false);
  let checkError = $state('');
  let submitting = $state(false);
  let submitError = $state('');
  let created = $state<Site | null>(null);

  // State progress bar real-time saat proses pembuatan & deployment situs berjalan
  let deployPercent = $state(0);
  let deployStage = $state('init');
  let deployStepLabel = $state('Menyiapkan provisioning situs...');
  let deployElapsedSec = $state(0);
  let deployPollTimer: ReturnType<typeof setInterval> | undefined;
  let deployTickTimer: ReturnType<typeof setInterval> | undefined;

  let debounceTimer: ReturnType<typeof setTimeout> | undefined;
  let abortCtl: AbortController | undefined;

  // Buat password acak untuk database MySQL otomatis
  function generateRandomPass(len = 14): string {
    const chars = 'abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789';
    let out = '';
    for (let i = 0; i < len; i++) {
      out += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return out;
  }

  function sanitizeDBIdentifier(dom: string): string {
    const clean = (dom || '')
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '_')
      .replace(/^_+|_+$/g, '');
    return (clean || 'site_app').slice(0, 14);
  }

  // Root path mengikuti domain utama & standar direktori FikPanel (/home/<user>/<runtime>/<domain>)
  let defaultRoot = $derived.by(() => {
    const d = report?.primary || 'example.com';
    const u = user.trim() || 'fikpanel';
    switch (kind) {
      case 'php':
        return `/home/${u}/web/${d}/public`;
      case 'node':
        return `/home/${u}/node/${d}`;
      case 'go':
        return `/home/${u}/go/${d}`;
      case 'python':
        return `/home/${u}/python/${d}`;
      case 'static':
        return `/home/${u}/static/${d}/public`;
      case 'proxy':
        return '';
      default:
        return `/home/${u}/web/${d}/public`;
    }
  });

  $effect(() => {
    if (!rootTouched) rootPath = defaultRoot;
  });

  // Isi otomatis nama database & user MySQL berdasarkan domain utama
  $effect(() => {
    if (!dbTouched) {
      const base = sanitizeDBIdentifier(report?.primary || 'fikpanel_site');
      dbName = `${base}_db`;
      dbUser = `${base}_u`;
    }
  });

  // Isi otomatis email Super Admin berdasarkan domain utama jika belum diubah manual
  $effect(() => {
    if (!adminEmailTouched) {
      const dom = report?.primary || 'example.com';
      adminEmail = `admin@${dom}`;
    }
  });

  function selectKind(k: SiteKind) {
    kind = k;
    if (k === 'node' && (upstream === 'http://127.0.0.1:8080' || upstream === 'http://127.0.0.1:8000')) {
      upstream = 'http://127.0.0.1:3000';
    } else if (k === 'go' && (upstream === 'http://127.0.0.1:3000' || upstream === 'http://127.0.0.1:8000')) {
      upstream = 'http://127.0.0.1:8080';
    } else if (k === 'python' && (upstream === 'http://127.0.0.1:3000' || upstream === 'http://127.0.0.1:8080')) {
      upstream = 'http://127.0.0.1:8000';
    }
  }

  function switchTab(tabId: BuilderTab) {
    activeTab = tabId;
    submitError = '';
    if (tabId === 'deploy') {
      applyTemplatePreset(selectedTemplate);
    } else if (tabId === 'git') {
      if (!gitStatus.connected) loadGitStatus();
    } else if (tabId === 'backup') {
      loadBackupArchives();
    } else if (tabId === 'batch' && !batchLines.trim()) {
      batchLines = [
        'app1.example.com|/home/fikpanel/web/app1.example.com/public|php|8.3|1',
        'api.example.com|/home/fikpanel/node/api.example.com|node|http://127.0.0.1:3000|0',
        'static.example.com|/home/fikpanel/static/static.example.com/public|static|0|0'
      ].join('\n');
    }
  }

  function applyTemplatePreset(tplId: string) {
    selectedTemplate = tplId;
    const tpl = DEPLOY_TEMPLATES.find((t) => t.id === tplId);
    if (!tpl) return;
    selectKind(tpl.kind);
    if (tpl.defaultUpstream) {
      upstream = tpl.defaultUpstream;
    }
    createDB = tpl.needsDB;
    if (createDB && !dbPass) {
      dbPass = generateRandomPass(14);
    }
  }

  // Filter repositori GitHub
  let filteredGitRepos = $derived.by(() => {
    const q = gitRepoFilter.trim().toLowerCase();
    if (!q) return gitRepos;
    return gitRepos.filter(
      (r) =>
        r.full_name.toLowerCase().includes(q) ||
        (r.language || '').toLowerCase().includes(q) ||
        (r.description || '').toLowerCase().includes(q)
    );
  });

  let selectedRepoObj = $derived(gitRepos.find((r) => r.full_name === selectedGitRepoFull) || null);

  // Saat kata kunci pencarian repo diketik, otomatis pilih repo teratas jika hanya tersisa 1 hasil atau pilihan lama tidak ada di hasil filter
  function onRepoFilterInput() {
    if (
      filteredGitRepos.length > 0 &&
      (filteredGitRepos.length === 1 || !filteredGitRepos.some((r) => r.full_name === selectedGitRepoFull))
    ) {
      selectGitRepo(filteredGitRepos[0].full_name);
    }
  }

  // Pratinjau interaktif untuk tab Batch Create
  let parsedBatchRows = $derived.by(() => {
    return batchLines
      .split('\n')
      .map((l) => l.trim())
      .filter((l) => l !== '' && !l.startsWith('#'))
      .map((line) => {
        const parts = line.split('|').map((p) => p.trim());
        const dom = parts[0] || '';
        const path = parts[1] && parts[1] !== '1' && parts[1] !== '0' ? parts[1] : `(auto) /home/fikpanel/.../${dom}`;
        const p2 = (parts[2] || '').toLowerCase();
        let rowKind = 'php';
        if (['php', 'static', 'node', 'go', 'python', 'proxy'].includes(p2)) {
          rowKind = p2;
        }
        const target = parts[3] || (rowKind === 'php' ? phpVersion || '8.3' : '-');
        const hasDB = parts[3] === '1' || parts[4] === '1';
        return { domain: dom, path, kind: rowKind, target, hasDB };
      })
      .filter((r) => r.domain !== '');
  });

  let checks = $derived(report?.checks ?? []);
  let dnsMap = $derived(new Map((report?.dns ?? []).map((d) => [d.domain, d])));
  let canSubmit = $derived.by(() => {
    if (submitting) return false;
    if (activeTab === 'batch') {
      return parsedBatchRows.length > 0;
    }
    if (!report?.valid || checking || domains.trim() === '') return false;
    if (activeTab === 'git' && gitRepoURL.trim() === '') return false;
    if (activeTab === 'backup' && selectedBackupArchive.trim() === '') return false;
    if (kind === 'proxy') return upstream.trim() !== '';
    if (kind === 'static' || kind === 'php') return rootPath.trim() !== '';
    return rootPath.trim() !== '' && upstream.trim() !== '';
  });

  onMount(() => {
    dbPass = generateRandomPass(14);
    apiFetch<SiteMeta>('/api/sites/meta')
      .then((m) => {
        meta = m;
        if (!phpVersion && m.php_versions.length) phpVersion = m.php_versions[0];
      })
      .catch((err: Error) => onNotify(`Gagal memuat metadata situs: ${err.message}`));
    loadGitStatus();
    return () => {
      clearTimeout(debounceTimer);
      abortCtl?.abort();
      stopDeployProgressPolling();
    };
  });

  // Muat status koneksi akun GitHub dari server
  async function loadGitStatus() {
    try {
      const st = await apiFetch<GitStatus>('/api/sites/git/status');
      avatarBroken = false;
      gitStatus = st;
      if (st.connected) {
        await loadGitRepos();
      }
    } catch {
      gitStatus = { connected: false };
    }
  }

  // Ambil daftar repositori GitHub milik akun yang terhubung
  async function loadGitRepos() {
    gitBusy = true;
    gitError = '';
    try {
      const res = await apiFetch<{ repos: GitRepoItem[] }>('/api/sites/git/repos');
      gitRepos = res.repos || [];
      if (gitRepos.length > 0 && !selectedGitRepoFull) {
        await selectGitRepo(gitRepos[0].full_name);
      }
    } catch (err) {
      gitError = (err as Error).message || 'Gagal memuat repositori GitHub';
    } finally {
      gitBusy = false;
    }
  }

  // Saat user memilih repositori GitHub dari dropdown, isi otomatis URL, branch, dan deteksi runtime
  async function selectGitRepo(fullName: string) {
    selectedGitRepoFull = fullName;
    const found = gitRepos.find((r) => r.full_name === fullName);
    if (!found) return;

    gitRepoURL = found.clone_url;
    gitBranch = found.default_branch || 'main';
    if (!description && found.description) {
      description = found.description.slice(0, 200);
    }

    // Deteksi otomatis runtime proyek berdasarkan bahasa pemrograman repositori GitHub
    const lang = (found.language || '').toLowerCase();
    if (lang === 'php') {
      selectKind('php');
      createDB = true;
      createAdmin = true;
      if (!gitPostCmd) gitPostCmd = 'composer install --no-interaction --prefer-dist';
    } else if (lang === 'javascript' || lang === 'typescript') {
      selectKind('node');
      if (!gitPostCmd) gitPostCmd = 'npm install && npm run build';
    } else if (lang === 'go') {
      selectKind('go');
      if (!gitPostCmd) gitPostCmd = 'go build -o app .';
    } else if (lang === 'python') {
      selectKind('python');
    } else if (lang === 'html' || lang === 'css' || lang === 'vue' || lang === 'svelte') {
      selectKind('static');
    }

    try {
      const res = await apiFetch<{
        branches: string[];
        detected_kind?: SiteKind;
        detected_post_cmd?: string;
        needs_db?: boolean;
        is_laravel?: boolean;
        is_filament?: boolean;
      }>(`/api/sites/git/branches?repo=${encodeURIComponent(fullName)}`);
      gitBranches = res.branches || [gitBranch];
      if (gitBranches.length > 0 && !gitBranches.includes(gitBranch)) {
        gitBranch = gitBranches[0];
      }
      // Prioritaskan hasil inspeksi file proyek aktual (package.json, composer.json, go.mod, ecosystem.config.cjs)
      if (res.detected_kind) {
        selectKind(res.detected_kind);
      }
      if (res.detected_post_cmd) {
        gitPostCmd = res.detected_post_cmd;
      }
      if (res.needs_db) {
        createDB = true;
      }
      isFilamentRepo = !!res.is_filament;
      isLaravelRepo = !!(res.is_laravel || res.is_filament);
      if (isFilamentRepo || isLaravelRepo) {
        createAdmin = true;
      }
    } catch {
      gitBranches = [gitBranch];
    }
  }

  // Hubungkan akun GitHub (Token Akses PAT untuk Public & Private Repo, atau Username Publik)
  async function connectGitHubAccount() {
    gitBusy = true;
    gitError = '';
    try {
      const payload =
        gitAuthMode === 'pat'
          ? { mode: 'pat', token: gitTokenInput.trim() }
          : { mode: 'public', username: gitPublicUser.trim() };
      await apiFetch('/api/sites/git/connect', {
        method: 'POST',
        body: JSON.stringify(payload)
      });
      gitTokenInput = '';
      gitConnectOpen = false;
      await loadGitStatus();
      onNotify('Akun GitHub berhasil dihubungkan');
    } catch (err) {
      gitError = (err as Error).message || 'Gagal menghubungkan akun GitHub';
    } finally {
      gitBusy = false;
    }
  }

  // Mulai alur persetujuan browser GitHub OAuth Device Flow
  async function startGitHubDeviceFlow() {
    gitBusy = true;
    gitError = '';
    try {
      const res = await apiFetch<{
        device_code: string;
        user_code: string;
        verification_uri: string;
        interval: number;
      }>('/api/sites/git/device/start', {
        method: 'POST',
        body: JSON.stringify({ client_id: gitClientId.trim() })
      });
      gitDeviceFlow = res;
      if (res.verification_uri) {
        window.open(res.verification_uri, '_blank', 'noopener,noreferrer');
      }
    } catch (err) {
      gitError = (err as Error).message || 'Gagal memulai GitHub Device Flow';
    } finally {
      gitBusy = false;
    }
  }

  // Periksa status otorisasi GitHub OAuth Device Flow setelah user menyetujui di browser
  async function pollGitHubDeviceFlow() {
    if (!gitDeviceFlow?.device_code) return;
    gitDevicePolling = true;
    gitError = '';
    try {
      const res = await apiFetch<{ status: string; username?: string; error?: string }>(
        '/api/sites/git/device/poll',
        {
          method: 'POST',
          body: JSON.stringify({
            client_id: gitClientId.trim(),
            device_code: gitDeviceFlow.device_code
          })
        }
      );
      if (res.status === 'connected') {
        gitDeviceFlow = null;
        gitConnectOpen = false;
        await loadGitStatus();
        onNotify('Otorisasi GitHub berhasil disetujui!');
      } else if (res.status === 'pending') {
        gitError = 'Menunggu persetujuan Anda di halaman GitHub (klik Authorize lalu coba lagi)...';
      } else if (res.error) {
        gitError = res.error;
      }
    } catch (err) {
      gitError = (err as Error).message || 'Gagal memeriksa status otorisasi GitHub';
    } finally {
      gitDevicePolling = false;
    }
  }

  // Putuskan koneksi akun GitHub dari server
  async function disconnectGitHubAccount() {
    gitBusy = true;
    gitError = '';
    try {
      await apiFetch('/api/sites/git/disconnect', { method: 'POST', body: '{}' });
      gitStatus = { connected: false };
      gitRepos = [];
      selectedGitRepoFull = '';
      onNotify('Koneksi akun GitHub diputuskan');
    } catch (err) {
      gitError = (err as Error).message || 'Gagal memutuskan akun GitHub';
    } finally {
      gitBusy = false;
    }
  }

  // Buka halaman pembuatan Personal Access Token GitHub dengan scope repo yang sudah tercentang otomatis
  function openGitHubQuickTokenPage() {
    const url = 'https://github.com/settings/tokens/new?description=FikPanel+Server+Deploy&scopes=repo,read:user';
    window.open(url, '_blank', 'noopener,noreferrer');
  }

  // Ambil atau generate Server SSH Deploy Key (id_ed25519.pub)
  async function fetchServerSSHKey() {
    sshKeyLoading = true;
    gitError = '';
    try {
      const res = await apiFetch<{ public_key: string }>('/api/sites/git/ssh-key', {
        method: 'POST',
        body: '{}'
      });
      sshPublicKey = res.public_key || '';
    } catch (err) {
      gitError = (err as Error).message || 'Gagal memuat SSH Deploy Key server';
    } finally {
      sshKeyLoading = false;
    }
  }

  async function copySSHKey() {
    if (!sshPublicKey) return;
    try {
      await copyToClipboard(sshPublicKey);
      sshKeyCopied = true;
      setTimeout(() => (sshKeyCopied = false), 2000);
    } catch {
      // Abaikan bila clipboard tidak diizinkan
    }
  }

  // Muat daftar arsip backup lokal di server
  async function loadBackupArchives() {
    backupLoading = true;
    try {
      const res = await apiFetch<{ archives: BackupArchiveItem[] }>('/api/sites/backups/archives');
      backupArchives = res.archives || [];
      if (backupArchives.length > 0 && !selectedBackupArchive) {
        selectedBackupArchive = backupArchives[0].path;
      }
    } catch {
      backupArchives = [];
    } finally {
      backupLoading = false;
    }
  }

  // Unggah file arsip backup (.tar.gz / .zip) dari komputer ke server menggunakan apiFetch
  async function handleBackupFileUpload(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    backupUploading = true;
    submitError = '';
    try {
      const formData = new FormData();
      formData.append('file', file);
      const data = await apiFetch<{ path: string; name: string }>('/api/sites/backups/upload', {
        method: 'POST',
        body: formData,
        timeoutMs: 120_000
      });
      await loadBackupArchives();
      if (data.path) {
        selectedBackupArchive = data.path;
      }
      onNotify(`Arsip backup ${data.name || file.name} berhasil diunggah`);
    } catch (err) {
      submitError = (err as Error).message || 'Gagal mengunggah arsip backup';
    } finally {
      backupUploading = false;
      input.value = '';
    }
  }

  // Validasi realtime dengan debounce; request lama dibatalkan agar hasil tidak tertukar
  function scheduleCheck() {
    clearTimeout(debounceTimer);
    abortCtl?.abort();
    checkError = '';
    if (!domains.trim()) {
      report = null;
      checking = false;
      return;
    }
    checking = true;
    debounceTimer = setTimeout(runCheck, 450);
  }

  async function runCheck() {
    const ctl = new AbortController();
    abortCtl = ctl;
    try {
      report = await apiFetch<DomainReport>('/api/sites/check-domains', {
        method: 'POST',
        body: JSON.stringify({ domains, add_www: addWWW, dns: true }),
        signal: ctl.signal
      });
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      checkError = (err as Error).message;
      report = null;
    } finally {
      if (abortCtl === ctl) checking = false;
    }
  }

  function startDeployProgressPolling(targetDomain: string) {
    stopDeployProgressPolling();
    deployPercent = 6;
    deployStage = 'vhost';
    deployStepLabel = 'Membuat konfigurasi Virtual Host Nginx & direktori web...';
    deployElapsedSec = 0;
    const startMs = Date.now();

    deployTickTimer = setInterval(() => {
      deployElapsedSec = Math.floor((Date.now() - startMs) / 1000);
      // Batasi kenaikan persentase otomatis sesuai plafon tahap aktif agar sinkron dengan tahapan backend
      const stageCap =
        deployStage === 'vhost' ? 16 :
        deployStage === 'database' ? 26 :
        deployStage === 'git_clone' ? 42 :
        deployStage === 'env' ? 54 :
        deployStage === 'composer' ? 70 :
        deployStage === 'migrate' ? 82 :
        deployStage === 'filament' ? 88 :
        deployStage === 'admin' ? 94 : 98;
      if (deployPercent < stageCap) {
        deployPercent = Math.min(stageCap, deployPercent + 1);
      }
    }, 1000);

    deployPollTimer = setInterval(async () => {
      try {
        const res = await apiFetch<{
          ok: boolean;
          progress?: { domain: string; percent: number; stage: string; step_label: string; updated_at: number };
        }>(`/api/sites/deploy-progress?domain=${encodeURIComponent(targetDomain)}`, {
          timeoutMs: 3500
        });
        if (res?.progress) {
          if (res.progress.percent > deployPercent) {
            deployPercent = res.progress.percent;
          }
          if (res.progress.stage) deployStage = res.progress.stage;
          if (res.progress.step_label) deployStepLabel = res.progress.step_label;
        }
      } catch {
        // Abaikan error polling sementara selama proses deploy berjalan
      }
    }, 650);
  }

  function stopDeployProgressPolling() {
    clearInterval(deployPollTimer);
    clearInterval(deployTickTimer);
    deployPollTimer = undefined;
    deployTickTimer = undefined;
  }

  async function submit(e: Event) {
    e.preventDefault();
    if (!canSubmit) return;
    submitting = true;
    submitError = '';
    const primaryDomain = report?.primary || domains.split('\n')[0]?.trim() || '';
    startDeployProgressPolling(primaryDomain);

    // Penanganan khusus Tab 4: Batch Create
    if (activeTab === 'batch') {
      try {
        deployStepLabel = `Membuat ${parsedBatchRows.length} situs secara massal...`;
        const res = await apiFetch<{
          ok: boolean;
          created_count: number;
          total_lines: number;
          results: { line: string; domain: string; kind: string; ok: boolean; message: string }[];
        }>('/api/sites/batch', {
          method: 'POST',
          body: JSON.stringify({
            lines: batchLines,
            auto_ssl: autoSSL
          }),
          timeoutMs: 120_000
        });
        deployPercent = 100;
        batchResult = res;
        if (res.created_count > 0) {
          onNotify(`Berhasil membuat ${res.created_count} dari ${res.total_lines} situs secara massal`);
          onCreated({ domain: `${res.created_count} situs massal` } as Site);
        } else {
          const firstErr = (res.results || []).find((r) => !r.ok);
          submitError = firstErr?.message || 'Gagal membuat situs pada daftar Batch';
        }
      } catch (err) {
        submitError = (err as Error).message;
      } finally {
        stopDeployProgressPolling();
        submitting = false;
      }
      return;
    }

    try {
      const site = await apiFetch<Site>('/api/sites', {
        method: 'POST',
        body: JSON.stringify({
          domains,
          add_www: addWWW,
          kind,
          upstream: kind === 'proxy' || kind === 'node' || kind === 'go' || kind === 'python' ? upstream.trim() : '',
          root_path: kind === 'proxy' ? '' : rootPath.trim(),
          php_version: kind === 'php' ? phpVersion : '',
          user: user.trim() || 'fikpanel',
          description: description.trim(),
          auto_ssl: autoSSL,
          create_db: createDB,
          db_name: createDB ? dbName.trim() : '',
          db_user: createDB ? dbUser.trim() : '',
          db_pass: createDB ? dbPass : '',
          create_admin: kind === 'php' && createDB && createAdmin,
          admin_name: createAdmin ? adminName.trim() || 'Super Admin' : '',
          admin_email: createAdmin ? adminEmail.trim() : '',
          admin_pass: createAdmin ? adminPass : '',
          deploy_template: activeTab === 'deploy' ? selectedTemplate : '',
          git_repo_url: activeTab === 'git' ? gitRepoURL.trim() : '',
          git_branch: activeTab === 'git' ? gitBranch.trim() || 'main' : '',
          git_post_cmd: activeTab === 'git' ? gitPostCmd.trim() : '',
          backup_archive: activeTab === 'backup' ? selectedBackupArchive.trim() : ''
        }),
        timeoutMs: 240_000
      });
      deployPercent = 100;
      deployStepLabel = 'Deployment selesai! Situs siap digunakan.';
      created = site;
      onCreated(site);
      onNotify(`Situs ${site.domain} berhasil dibuat`);
    } catch (err) {
      submitError = (err as Error).message;
    } finally {
      stopDeployProgressPolling();
      submitting = false;
    }
  }

  let allPointHere = $derived((report?.dns ?? []).length > 0 && (report?.dns ?? []).every((d) => d.points_here));
</script>

<Modal
  open={true}
  title={created ? 'Situs berhasil dibuat' : 'Add site — Support batch & dynamic site building'}
  subtitle={created
    ? created.domain
    : 'Provisioning vhost Nginx, Multi-Runtime, GitHub Private/Public Repo, Database MySQL & Auto-SSL'}
  size="xl"
  onclose={() => {
    if (!submitting) onClose();
  }}
>
  <div class="cmp-add-site">
    {#if !created}
      <nav class="tabs tabs--underline builder-tabs" aria-label="Mode Pembuatan Situs">
        {#each BUILDER_TABS as t (t.id)}
          <button
            type="button"
            class="tabs__tab builder-tab"
            class:active={activeTab === t.id}
            class:is-active={activeTab === t.id}
            onclick={() => switchTab(t.id)}
          >
            <div class="tab-top">
              <i class={t.icon}></i>
              <span>{t.label}</span>
            </div>
            <small>{t.sub}</small>
          </button>
        {/each}
      </nav>
    {/if}

    {#if created}
      <!-- Langkah 2: ringkasan + ajakan aktivasi SSL -->
      <div class="as-body as-done">
        <div class="done-hero">
          <div class="done-ring"><i class="ri-check-line"></i></div>
          <p>Vhost <b>{created.domain}</b> aktif di port 80 dan siap menerima trafik.</p>
        </div>
        <dl class="done-grid">
          <dt>Jenis</dt><dd>{KINDS.find((k) => k.id === created!.kind)?.label}</dd>
          {#if created.kind === 'proxy'}
            <dt>Upstream</dt><dd class="mono">{created.upstream}</dd>
          {:else if created.kind === 'node' || created.kind === 'go' || created.kind === 'python'}
            <dt>Direktori</dt><dd class="mono">{created.root_path}</dd>
            <dt>Upstream</dt><dd class="mono">{created.upstream}</dd>
          {:else}
            <dt>Direktori</dt><dd class="mono">{created.root_path}</dd>
          {/if}
          {#if created.php_version}<dt>PHP</dt><dd>{created.php_version}</dd>{/if}
          {#if createDB}<dt>Database MySQL</dt><dd class="mono">{dbName} (user: {dbUser})</dd>{/if}
          {#if createDB && createAdmin && kind === 'php'}<dt>Super Admin</dt><dd class="mono">{adminEmail} (pass: {adminPass})</dd>{/if}
          <dt>Domain</dt><dd class="mono">{[created.domain, ...JSON.parse(created.aliases || '[]')].join(', ')}</dd>
        </dl>
        <div class="ssl-cta" class:warn={!allPointHere}>
          <i class={allPointHere ? 'ri-shield-check-line' : 'ri-error-warning-line'}></i>
          <div>
            <b>{autoSSL ? 'Penerbitan SSL otomatis telah dijadwalkan' : 'Langkah berikutnya: aktifkan SSL'}</b>
            <span>
              {autoSSL
                ? 'Let\'s Encrypt sedang memvalidasi HTTP-01 di latar belakang. Anda juga dapat memantau detailnya di menu SSL.'
                : allPointHere
                  ? 'Semua domain sudah mengarah ke server ini, Let\'s Encrypt siap diterbitkan.'
                  : 'Sebagian domain belum mengarah ke server ini. Perbarui DNS A record terlebih dahulu agar validasi HTTP-01 berhasil.'}
            </span>
          </div>
        </div>
        <footer class="as-foot">
          <button class="btn btn-ghost" onclick={onClose} id="as-done-btn">Selesai</button>
          <button class="btn btn-primary" onclick={() => onOpenSSL(created!)} id="as-ssl-btn">
            <i class="ri-lock-line"></i> Kelola / Aktifkan SSL
          </button>
        </footer>
      </div>
    {:else}
      <form class="as-body" onsubmit={submit}>
        <div class="as-scroll" class:is-submitting={submitting}>
          <!-- ========================================================= -->
          <!-- TAB 4: BATCH CREATE (Pembuatan Banyak Situs Sekaligus)     -->
          <!-- ========================================================= -->
          {#if activeTab === 'batch'}
            <div class="batch-head-card">
              <div>
                <b>Format Batch Site Building (Mendukung Format FikPanel & aaPanel)</b>
                <p>
                  Satu baris per situs: <code>domain|root_path|kind|php_atau_upstream|create_db(1/0)</code>
                </p>
              </div>
              <label class="ssl-inline-check">
                <input type="checkbox" bind:checked={autoSSL} />
                <span>Apply for SSL (Semua)</span>
              </label>
            </div>

            <label class="fld" for="as-batch-lines">
              <span class="fld-label">Daftar Situs Massal <em>pisahkan kolom dengan karakter pipa (|)</em></span>
              <textarea
                id="as-batch-lines"
                rows="6"
                spellcheck="false"
                placeholder={'app1.contoh.com|/home/fikpanel/web/app1.contoh.com/public|php|8.3|1\napi.contoh.com|/home/fikpanel/node/api.contoh.com|node|http://127.0.0.1:3000|0'}
                bind:value={batchLines}
              ></textarea>
            </label>

            {#if parsedBatchRows.length > 0}
              <div class="batch-preview">
                <div class="batch-preview-head">
                  <span>Pratinjau Interaktif ({parsedBatchRows.length} situs siap dibuat)</span>
                  <span class="tag-accent">Real-time Parser</span>
                </div>
                <div class="batch-table-scroll">
                  <table class="batch-table">
                    <thead>
                      <tr>
                        <th>Domain</th>
                        <th>Runtime</th>
                        <th>Website Path</th>
                        <th>PHP / Upstream</th>
                        <th>Database</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each parsedBatchRows as row, idx (row.domain + idx)}
                        <tr>
                          <td class="mono">{row.domain}</td>
                          <td><span class="tag-accent">{row.kind.toUpperCase()}</span></td>
                          <td class="mono muted-cell">{row.path}</td>
                          <td class="mono">{row.target}</td>
                          <td>{row.hasDB ? 'MySQL Auto' : 'Tidak'}</td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              </div>
            {/if}

            {#if batchResult}
              <div class="batch-result-list">
                {#each batchResult.results as res, i (res.domain + i)}
                  <div class="batch-res-item" class:bad={!res.ok}>
                    <i class={res.ok ? 'ri-checkbox-circle-fill' : 'ri-close-circle-fill'}></i>
                    <strong class="mono">{res.domain}</strong>
                    <span>{res.message}</span>
                  </div>
                {/each}
              </div>
            {/if}

          {:else}
            <!-- ========================================================= -->
            <!-- TAB 2: ONE-CLICK DEPLOY TEMPLATE SELECTOR                 -->
            <!-- ========================================================= -->
            {#if activeTab === 'deploy'}
              <div class="fld">
                <span class="fld-label">Template Deploy <em>pilih framework/CMS siap pakai</em></span>
                <div class="tpl-grid">
                  {#each DEPLOY_TEMPLATES as tpl (tpl.id)}
                    <button
                      type="button"
                      class="tpl-card"
                      class:active={selectedTemplate === tpl.id}
                      onclick={() => applyTemplatePreset(tpl.id)}
                    >
                      <div class="tpl-head">
                        <span class="tpl-title"><i class={tpl.icon}></i> <b>{tpl.name}</b></span>
                        <span class="tag-accent">{tpl.badge}</span>
                      </div>
                      <small class="mono tpl-ver">{tpl.version}</small>
                      <span class="tpl-desc">{tpl.desc}</span>
                    </button>
                  {/each}
                </div>
              </div>
            {/if}

            <!-- ========================================================= -->
            <!-- DOMAIN INPUT + APPLY FOR SSL + DNS VALIDATOR              -->
            <!-- ========================================================= -->
            <label class="fld" for="as-domains">
              <div class="row-between">
                <span class="fld-label">
                  Domain name <em>satu domain per baris, default port 80 (mendukung *.domain.com)</em>
                </span>
                <label class="ssl-inline-check">
                  <input type="checkbox" bind:checked={autoSSL} />
                  <span><i class="ri-shield-check-line"></i> Apply for SSL</span>
                </label>
              </div>
              <textarea
                id="as-domains"
                rows="3"
                spellcheck="false"
                placeholder={'contoh.com\nblog.contoh.com\n*.app.contoh.com'}
                bind:value={domains}
                oninput={scheduleCheck}
              ></textarea>
            </label>

            <div class="row-between">
              <label class="switch" for="as-www">
                <input id="as-www" type="checkbox" bind:checked={addWWW} onchange={scheduleCheck} />
                <span class="track"></span>
                <span>Tambahkan <code>www.</code> otomatis untuk domain utama</span>
              </label>
              {#if checking}
                <span class="hint"><i class="ri-loader-4-line spin"></i> Memeriksa DNS…</span>
              {:else if report}
                <span class="hint" class:ok={report.valid} class:bad={!report.valid}>
                  <i class={report.valid ? 'ri-checkbox-circle-line' : 'ri-close-circle-line'}></i>
                  {report.valid ? `${checks.filter((c) => !c.error).length} domain valid` : 'Periksa kembali domain'}
                </span>
              {/if}
            </div>

            {#if checkError}
              <p class="err-line"><i class="ri-error-warning-line"></i> {checkError}</p>
            {/if}

            {#if checks.length}
              <ul class="chips">
                {#each checks as c, i (c.input + i)}
                  {@const dns = dnsMap.get(c.domain)}
                  <li
                    class="chip"
                    class:bad={!!c.error}
                    class:auto={c.auto}
                    title={c.error || dns?.error || (dns?.ips ?? []).join(', ')}
                  >
                    {#if c.error}
                      <i class="ri-close-circle-fill"></i>
                    {:else if dns?.points_here}
                      <i class="ri-checkbox-circle-fill ok"></i>
                    {:else if dns}
                      <i class="ri-error-warning-fill warn"></i>
                    {:else}
                      <i class="ri-checkbox-blank-circle-line"></i>
                    {/if}
                    <span class="mono">{c.domain || c.input}</span>
                    {#if report?.primary === c.domain}<b class="tag">utama</b>{/if}
                    {#if c.auto}<b class="tag">auto</b>{/if}
                    {#if c.wildcard}<b class="tag">wildcard</b>{/if}
                    {#if c.error}
                      <small>{c.error}</small>
                    {:else if dns && !dns.points_here}
                      <small>{dns.error ? 'DNS belum ada' : `→ ${(dns.ips ?? []).join(', ')}`}</small>
                    {/if}
                  </li>
                {/each}
              </ul>
              {#if report?.server_ips.length}
                <p class="ip-note">
                  <i class="ri-server-line"></i> IP server: <span class="mono">{report.server_ips.join(', ')}</span>
                  — pastikan A record mengarah ke IP ini bila mencentang <b>Apply for SSL</b>.
                </p>
              {/if}
            {/if}

            <!-- ========================================================= -->
            <!-- TAB 3: CREATE FOR GIT / GITHUB INTEGRATION                -->
            <!-- ========================================================= -->
            {#if activeTab === 'git'}
              <div class="git-box">
                <div class="row-between">
                  <span class="fld-label">Repository source</span>
                  <div class="seg-group">
                    <button
                      type="button"
                      class="seg-btn"
                      class:active={gitSource === 'github'}
                      onclick={() => (gitSource = 'github')}
                    >
                      <i class="ri-github-fill"></i> GitHub Account (Public & Private)
                    </button>
                    <button
                      type="button"
                      class="seg-btn"
                      class:active={gitSource === 'url'}
                      onclick={() => (gitSource = 'url')}
                    >
                      <i class="ri-git-repository-private-line"></i> Repository URL / SSH Key
                    </button>
                  </div>
                </div>

                {#if gitSource === 'github'}
                  {#if !gitStatus.connected}
                    <div class="gh-card">
                      <div class="gh-card-left">
                        <span class="gh-logo"><i class="ri-github-fill"></i></span>
                        <div>
                          <b>Not connected</b>
                          <small>
                            Otorisasi sekali untuk mengakses repositori GitHub Anda. Mendukung penuh <b>Public & Private Repositories</b>.
                          </small>
                        </div>
                      </div>
                      <button
                        type="button"
                        class="btn btn-primary btn-sm"
                        onclick={() => (gitConnectOpen = !gitConnectOpen)}
                      >
                        <i class="ri-github-fill"></i>
                        {gitConnectOpen ? 'Tutup Panel' : 'Connect GitHub'}
                      </button>
                    </div>
                  {:else}
                    <div class="gh-card connected">
                      <div class="gh-card-left">
                        {#if gitStatus.avatar_url && !avatarBroken}
                          <img
                            src={gitStatus.avatar_url}
                            alt={gitStatus.username}
                            class="gh-avatar"
                            onerror={() => (avatarBroken = true)}
                          />
                        {:else}
                          <span class="gh-logo"><i class="ri-github-fill"></i></span>
                        {/if}
                        <div>
                          <div class="gh-user-line">
                            <b>@{gitStatus.username}</b>
                            {#if gitStatus.name}<span>({gitStatus.name})</span>{/if}
                            <span class="tag-accent">
                              {gitStatus.mode === 'token' || gitStatus.mode === 'pat'
                                ? 'Full Access · Public & Private'
                                : 'Public Repos'}
                            </span>
                          </div>
                          <small>{gitRepos.length} repositori ditemukan · Pilih repo untuk auto-detect bahasa & branch.</small>
                        </div>
                      </div>
                      <div class="gh-btn-row">
                        <button type="button" class="btn btn-ghost btn-sm" onclick={loadGitRepos} disabled={gitBusy}>
                          <i class="ri-refresh-line"></i> Refresh
                        </button>
                        <button type="button" class="btn btn-ghost btn-sm" onclick={disconnectGitHubAccount} disabled={gitBusy}>
                          Disconnect
                        </button>
                      </div>
                    </div>
                  {/if}

                  {#if gitConnectOpen && !gitStatus.connected}
                    <div class="gh-connect-drawer">
                      <div class="gh-mode-switch">
                        <button
                          type="button"
                          class="gh-mode-btn"
                          class:active={gitAuthMode === 'pat'}
                          onclick={() => (gitAuthMode = 'pat')}
                        >
                          1. Token Akses 1-Klik (Public & Private)
                        </button>
                        <button
                          type="button"
                          class="gh-mode-btn"
                          class:active={gitAuthMode === 'device'}
                          onclick={() => (gitAuthMode = 'device')}
                        >
                          2. Persetujuan Browser (Device Flow)
                        </button>
                        <button
                          type="button"
                          class="gh-mode-btn"
                          class:active={gitAuthMode === 'public'}
                          onclick={() => (gitAuthMode = 'public')}
                        >
                          3. Username Publik (Tanpa Token)
                        </button>
                      </div>

                      {#if gitAuthMode === 'pat'}
                        <p class="gh-help">
                          Berbeda dengan aaPanel yang hanya menampilkan repo publik, mode Token Akses menghubungkan server Anda langsung ke API GitHub untuk mengakses <b>repositori Private & Public</b>:
                        </p>
                        <div class="gh-inline-step">
                          <button type="button" class="btn btn-ghost btn-sm" onclick={openGitHubQuickTokenPage}>
                            <i class="ri-external-link-line"></i> 1. Buka GitHub & Buat Token Otomatis
                          </button>
                          <small>Scope <code>repo</code> sudah tercentang otomatis, cukup klik <b>Generate token</b> di GitHub.</small>
                        </div>
                        <div class="gh-input-group">
                          <input
                            type="password"
                            class="mono"
                            placeholder="Tempel Personal Access Token GitHub (ghp_... atau github_pat_...)"
                            bind:value={gitTokenInput}
                          />
                          <button
                            type="button"
                            class="btn btn-primary btn-sm"
                            disabled={gitBusy || !gitTokenInput.trim()}
                            onclick={connectGitHubAccount}
                          >
                            {gitBusy ? 'Memverifikasi…' : 'Hubungkan'}
                          </button>
                        </div>
                      {:else if gitAuthMode === 'device'}
                        <p class="gh-help">
                          Otorisasi via layar persetujuan resmi GitHub (<code>github.com/login/device</code>) menggunakan Client ID GitHub OAuth App Anda tanpa memerlukan Callback URL:
                        </p>
                        <div class="gh-input-group">
                          <input
                            type="text"
                            class="mono"
                            placeholder="Masukkan GitHub OAuth App Client ID (contoh: Iv1.8a61f...)"
                            bind:value={gitClientId}
                          />
                          <button
                            type="button"
                            class="btn btn-primary btn-sm"
                            disabled={gitBusy || !gitClientId.trim()}
                            onclick={startGitHubDeviceFlow}
                          >
                            Minta Kode Persetujuan
                          </button>
                        </div>
                        {#if gitDeviceFlow}
                          <div class="device-approval-card">
                            <div>
                              <small>Masukkan kode persetujuan ini di GitHub:</small>
                              <div class="device-code-badge mono">{gitDeviceFlow.user_code}</div>
                            </div>
                            <div class="gh-btn-row">
                              <a
                                href={gitDeviceFlow.verification_uri}
                                target="_blank"
                                rel="noopener noreferrer"
                                class="btn btn-ghost btn-sm"
                              >
                                Buka GitHub ↗
                              </a>
                              <button
                                type="button"
                                class="btn btn-primary btn-sm"
                                disabled={gitDevicePolling}
                                onclick={pollGitHubDeviceFlow}
                              >
                                {gitDevicePolling ? 'Memeriksa…' : 'Saya Sudah Klik Authorize'}
                              </button>
                            </div>
                          </div>
                        {/if}
                      {:else}
                        <p class="gh-help">
                          Tanpa login atau token: masukkan username atau organisasi GitHub untuk langsung memuat seluruh repositori publiknya:
                        </p>
                        <div class="gh-input-group">
                          <input
                            type="text"
                            class="mono"
                            placeholder="Ketik username / organisasi GitHub (contoh: FikFikk atau laravel)"
                            bind:value={gitPublicUser}
                          />
                          <button
                            type="button"
                            class="btn btn-primary btn-sm"
                            disabled={gitBusy || !gitPublicUser.trim()}
                            onclick={connectGitHubAccount}
                          >
                            {gitBusy ? 'Memuat…' : 'Muat Repo Publik'}
                          </button>
                        </div>
                      {/if}
                    </div>
                  {/if}

                  {#if gitError}
                    <p class="err-line"><i class="ri-error-warning-line"></i> {gitError}</p>
                  {/if}

                  {#if gitStatus.connected}
                    <div class="grid-repo">
                      <div class="fld repo-combo-wrap">
                        <div class="row-between">
                          <span class="fld-label">
                            Repository <em>({filteredGitRepos.length} dari {gitRepos.length} repo)</em>
                          </span>
                        </div>

                        <div class="repo-combobox" class:open={repoDropdownOpen}>
                          <div class="repo-combo-bar">
                            <i class="ri-search-line repo-search-icon"></i>
                            <input
                              id="as-gh-repo"
                              type="text"
                              class="repo-combo-input"
                              placeholder={selectedRepoObj ? selectedRepoObj.full_name : 'Ketik untuk cari repo (mis. gci, laravel)...'}
                              bind:value={gitRepoFilter}
                              onfocus={() => (repoDropdownOpen = true)}
                              oninput={() => {
                                repoDropdownOpen = true;
                                onRepoFilterInput();
                              }}
                              onkeydown={(e) => {
                                if (e.key === 'Enter') {
                                  e.preventDefault();
                                  if (filteredGitRepos.length > 0) {
                                    selectGitRepo(filteredGitRepos[0].full_name);
                                  }
                                  repoDropdownOpen = false;
                                } else if (e.key === 'Escape') {
                                  e.stopPropagation();
                                  repoDropdownOpen = false;
                                }
                              }}
                            />
                            {#if gitRepoFilter}
                              <button
                                type="button"
                                class="repo-clear-btn"
                                title="Bersihkan pencarian"
                                onclick={() => {
                                  gitRepoFilter = '';
                                  repoDropdownOpen = true;
                                }}
                              >
                                <i class="ri-close-circle-fill"></i>
                              </button>
                            {/if}
                            <button
                              type="button"
                              class="repo-toggle-btn"
                              aria-label="Daftar Repositori"
                              onclick={() => (repoDropdownOpen = !repoDropdownOpen)}
                            >
                              <i class={repoDropdownOpen ? 'ri-arrow-up-s-line' : 'ri-arrow-down-s-line'}></i>
                            </button>
                          </div>

                          {#if selectedRepoObj && !repoDropdownOpen}
                            <button
                              type="button"
                              class="repo-selected-pill"
                              onclick={() => (repoDropdownOpen = true)}
                              title="Klik untuk mengganti repositori"
                            >
                              <span class="repo-vis-badge" class:priv={selectedRepoObj.private}>
                                <i class={selectedRepoObj.private ? 'ri-lock-2-fill' : 'ri-global-line'}></i>
                                {selectedRepoObj.private ? 'Private' : 'Public'}
                              </span>
                              <strong class="mono repo-pill-name">{selectedRepoObj.full_name}</strong>
                              {#if selectedRepoObj.language}
                                <span class="repo-lang-tag">{selectedRepoObj.language}</span>
                              {/if}
                            </button>
                          {/if}

                          {#if repoDropdownOpen}
                            <!-- svelte-ignore a11y_click_events_have_key_events -->
                            <!-- svelte-ignore a11y_no_static_element_interactions -->
                            <div class="repo-dropdown-backdrop" onclick={() => (repoDropdownOpen = false)}></div>
                            <div class="repo-dropdown-menu" role="listbox">
                              {#if filteredGitRepos.length === 0}
                                <div class="repo-empty-item">
                                  Tidak ada repositori yang cocok dengan "<b>{gitRepoFilter}</b>"
                                </div>
                              {:else}
                                {#each filteredGitRepos as r (r.full_name)}
                                  <button
                                    type="button"
                                    role="option"
                                    aria-selected={selectedGitRepoFull === r.full_name}
                                    class="repo-option-item"
                                    class:active={selectedGitRepoFull === r.full_name}
                                    onclick={() => {
                                      selectGitRepo(r.full_name);
                                      repoDropdownOpen = false;
                                    }}
                                  >
                                    <div class="repo-opt-left">
                                      <span class="repo-vis-badge" class:priv={r.private}>
                                        <i class={r.private ? 'ri-lock-2-fill' : 'ri-global-line'}></i>
                                        {r.private ? 'Private' : 'Public'}
                                      </span>
                                      <span class="mono repo-opt-name">{r.full_name}</span>
                                    </div>
                                    <div class="repo-opt-right">
                                      {#if r.language}
                                        <span class="repo-lang-tag">{r.language}</span>
                                      {/if}
                                      {#if selectedGitRepoFull === r.full_name}
                                        <i class="ri-check-line repo-check-icon"></i>
                                      {/if}
                                    </div>
                                  </button>
                                {/each}
                              {/if}
                            </div>
                          {/if}
                        </div>
                      </div>

                      <label class="fld" for="as-gh-branch">
                        <span class="fld-label">Branch</span>
                        {#if gitBranches.length > 0}
                          <select id="as-gh-branch" bind:value={gitBranch}>
                            {#each gitBranches as br (br)}
                              <option value={br}>{br}</option>
                            {/each}
                          </select>
                        {:else}
                          <input id="as-gh-branch" class="mono" type="text" bind:value={gitBranch} placeholder="main" />
                        {/if}
                      </label>
                    </div>
                  {/if}
                {:else}
                  <!-- Mode Repository URL / SSH Deploy Key -->
                  <div class="grid-repo">
                    <label class="fld" for="as-git-url">
                      <span class="fld-label">Repository URL (HTTPS atau SSH)</span>
                      <input
                        id="as-git-url"
                        class="mono"
                        type="text"
                        placeholder="https://github.com/user/repo.git atau git@github.com:user/repo.git"
                        bind:value={gitRepoURL}
                      />
                    </label>
                    <label class="fld" for="as-git-branch">
                      <span class="fld-label">Branch</span>
                      <input id="as-git-branch" class="mono" type="text" placeholder="main" bind:value={gitBranch} />
                    </label>
                  </div>

                  <div class="ssh-key-card">
                    <div class="row-between">
                      <div>
                        <b>Server SSH Deploy Key (<code>~/.ssh/id_ed25519.pub</code>)</b>
                        <small>Tambahkan kunci ini ke <em>GitHub → Repo Settings → Deploy Keys</em> untuk clone repo privat via SSH.</small>
                      </div>
                      <div class="gh-btn-row">
                        <button type="button" class="btn btn-ghost btn-sm" onclick={fetchServerSSHKey} disabled={sshKeyLoading}>
                          {sshKeyLoading ? 'Memuat…' : 'Generate / Lihat SSH Key'}
                        </button>
                        {#if sshPublicKey}
                          <button type="button" class="btn btn-primary btn-sm" onclick={copySSHKey}>
                            {sshKeyCopied ? 'Tersalin!' : 'Salin Kunci'}
                          </button>
                        {/if}
                      </div>
                    </div>
                    {#if sshPublicKey}
                      <pre class="ssh-pre mono">{sshPublicKey}</pre>
                    {/if}
                  </div>
                {/if}

                <label class="fld" for="as-git-post">
                  <div class="row-between">
                    <span class="fld-label">Post-Clone Build Command <em>opsional</em></span>
                    <div class="presets">
                      <button type="button" class="preset" onclick={() => (gitPostCmd = 'composer install --no-interaction --prefer-dist')}>
                        composer install
                      </button>
                      <button type="button" class="preset" onclick={() => (gitPostCmd = 'npm install && npm run build')}>
                        npm build
                      </button>
                      <button type="button" class="preset" onclick={() => (gitPostCmd = 'go build -o app .')}>
                        go build
                      </button>
                    </div>
                  </div>
                  <input
                    id="as-git-post"
                    class="mono"
                    type="text"
                    placeholder="composer install --no-dev && php artisan migrate --force"
                    bind:value={gitPostCmd}
                  />
                </label>

                {#if kind === 'php'}
                  <div class="admin-auto-divider"></div>
                  <div class="row-between">
                    <label class="check-inline" for="as-git-create-admin">
                      <input id="as-git-create-admin" type="checkbox" bind:checked={createAdmin} />
                      <span><i class="ri-shield-user-line"></i> Buat Akun Super Admin Otomatis (Laravel / Filament Shield)</span>
                      {#if isFilamentRepo}
                        <span class="filament-pill">Filament Terdeteksi</span>
                      {:else if isLaravelRepo}
                        <span class="filament-pill">Laravel Terdeteksi</span>
                      {/if}
                    </label>
                    {#if createAdmin}
                      <button type="button" class="preset" onclick={() => (adminPass = generateRandomPass(12))}>
                        Acak Password Admin
                      </button>
                    {/if}
                  </div>
                  {#if createAdmin}
                    <div class="grid-3">
                      <label class="fld" for="as-git-admin-name">
                        <span class="fld-label">Admin Name (Username)</span>
                        <input
                          id="as-git-admin-name"
                          type="text"
                          placeholder="Super Admin"
                          bind:value={adminName}
                        />
                      </label>
                      <label class="fld" for="as-git-admin-email">
                        <span class="fld-label">Email Login</span>
                        <input
                          id="as-git-admin-email"
                          class="mono"
                          type="email"
                          placeholder="admin@domain.com"
                          bind:value={adminEmail}
                          oninput={() => (adminEmailTouched = true)}
                        />
                      </label>
                      <label class="fld" for="as-git-admin-pass">
                        <span class="fld-label">Password Login</span>
                        <input
                          id="as-git-admin-pass"
                          class="mono"
                          type="text"
                          placeholder="Minimal 4 karakter"
                          bind:value={adminPass}
                        />
                      </label>
                    </div>
                  {/if}
                {/if}
              </div>
            {/if}

            <!-- ========================================================= -->
            <!-- TAB 5: CREATE FROM BACKUP ARCHIVE                         -->
            <!-- ========================================================= -->
            {#if activeTab === 'backup'}
              <div class="git-box">
                <div class="row-between">
                  <span class="fld-label">Sumber Arsip Backup (<code>.tar.gz</code>, <code>.tgz</code>, <code>.zip</code>)</span>
                  <div class="gh-btn-row">
                    <button type="button" class="btn btn-ghost btn-sm" onclick={loadBackupArchives} disabled={backupLoading}>
                      <i class="ri-refresh-line"></i> {backupLoading ? 'Memindai…' : 'Pindai Server'}
                    </button>
                    <label class="btn btn-primary btn-sm upload-lbl">
                      <i class="ri-upload-2-line"></i>
                      <span>{backupUploading ? 'Mengunggah…' : 'Upload Arsip'}</span>
                      <input
                        type="file"
                        accept=".tar.gz,.tgz,.zip"
                        onchange={handleBackupFileUpload}
                        disabled={backupUploading}
                        hidden
                      />
                    </label>
                  </div>
                </div>

                {#if backupArchives.length > 0}
                  <label class="fld" for="as-backup-sel">
                    <span class="fld-label">Arsip Ditemukan di Server ({backupArchives.length})</span>
                    <select id="as-backup-sel" bind:value={selectedBackupArchive}>
                      {#each backupArchives as arc (arc.path)}
                        <option value={arc.path}>
                          {arc.name} ({formatBytes(arc.size)}) — {arc.path}
                        </option>
                      {/each}
                    </select>
                  </label>
                {/if}

                <label class="fld" for="as-backup-path">
                  <span class="fld-label">Path Absolut Arsip Backup</span>
                  <input
                    id="as-backup-path"
                    class="mono"
                    type="text"
                    placeholder="/var/backups/fikpanel/site-backup.tar.gz"
                    bind:value={selectedBackupArchive}
                  />
                </label>
              </div>
            {/if}

            <!-- ========================================================= -->
            <!-- PILIHAN JENIS SITUS / RUNTIME (TAB 1, 3, 5)               -->
            <!-- ========================================================= -->
            {#if activeTab !== 'deploy'}
              <div class="row-between">
                <span class="fld-label">Jenis situs (Runtime)</span>
                <div class="user-inline">
                  <span class="fld-label">User Linux:</span>
                  <input id="as-user" class="mono user-input" type="text" placeholder="fikpanel" bind:value={user} />
                </div>
              </div>
              <div class="kinds" role="radiogroup" aria-label="Jenis situs">
                {#each KINDS as k (k.id)}
                  <button
                    type="button"
                    role="radio"
                    aria-checked={kind === k.id}
                    id="as-kind-{k.id}"
                    class="kind"
                    class:active={kind === k.id}
                    onclick={() => selectKind(k.id)}
                  >
                    <i class={k.icon}></i>
                    <b>{k.label}</b>
                    <small>{k.desc}</small>
                  </button>
                {/each}
              </div>
            {/if}

            <!-- Website Path + PHP Version / Upstream Port -->
            {#if kind === 'proxy'}
              <label class="fld" for="as-upstream">
                <span class="fld-label">Upstream URL</span>
                <input id="as-upstream" class="mono" type="text" placeholder="http://127.0.0.1:3000" bind:value={upstream} />
              </label>
              <div class="presets">
                {#each UPSTREAM_PRESETS as p (p.label)}
                  <button type="button" class="preset" class:active={upstream === p.url} onclick={() => (upstream = p.url)}>
                    {p.label}
                  </button>
                {/each}
              </div>
            {:else if kind === 'node' || kind === 'go' || kind === 'python'}
              <label class="fld" for="as-root">
                <span class="fld-label">
                  Website Path (Direktori proyek)
                  {#if rootTouched && defaultRoot}
                    <button type="button" class="link" onclick={() => { rootTouched = false; }}>reset</button>
                  {/if}
                </span>
                <input
                  id="as-root"
                  class="mono"
                  type="text"
                  placeholder="/home/fikpanel/{kind}/contoh.com"
                  bind:value={rootPath}
                  oninput={() => (rootTouched = true)}
                />
              </label>
              <label class="fld" for="as-upstream">
                <span class="fld-label">Upstream Aplikasi (Reverse Proxy Nginx)</span>
                <input id="as-upstream" class="mono" type="text" placeholder="http://127.0.0.1:3000" bind:value={upstream} />
              </label>
              <div class="presets">
                {#each UPSTREAM_PRESETS as p (p.label)}
                  <button type="button" class="preset" class:active={upstream === p.url} onclick={() => (upstream = p.url)}>
                    {p.label}
                  </button>
                {/each}
              </div>
            {:else}
              <div class="grid-2" class:single={kind === 'static'}>
                <label class="fld" for="as-root">
                  <span class="fld-label">
                    Website Path (Direktori root)
                    {#if rootTouched && defaultRoot}
                      <button type="button" class="link" onclick={() => { rootTouched = false; }}>reset</button>
                    {/if}
                  </span>
                  <input
                    id="as-root"
                    class="mono"
                    type="text"
                    placeholder="/home/fikpanel/web/contoh.com/public"
                    bind:value={rootPath}
                    oninput={() => (rootTouched = true)}
                  />
                </label>
                {#if kind === 'php'}
                  <label class="fld" for="as-php">
                    <span class="fld-label">PHP version</span>
                    <select id="as-php" bind:value={phpVersion}>
                      {#if meta.php_versions.length === 0}
                        <option value="">Default FPM</option>
                      {:else}
                        {#each meta.php_versions as v (v)}
                          <option value={v}>PHP {v}</option>
                        {/each}
                      {/if}
                    </select>
                  </label>
                {/if}
              </div>
            {/if}

            <!-- Baris Deskripsi & Pembuatan Database Otomatis -->
            <div class="grid-2">
              <label class="fld" for="as-desc">
                <span class="fld-label">Description <em>opsional</em></span>
                <input id="as-desc" type="text" maxlength="200" placeholder="Catatan proyek / produksi" bind:value={description} />
              </label>

              <label class="fld" for="as-db-select">
                <span class="fld-label">Database</span>
                <select
                  id="as-db-select"
                  value={createDB ? 'mysql' : 'none'}
                  onchange={(e) => (createDB = (e.currentTarget as HTMLSelectElement).value === 'mysql')}
                >
                  <option value="none">Not create</option>
                  <option value="mysql">MySQL (utf8mb4 — Auto Create)</option>
                </select>
              </label>
            </div>

            {#if createDB}
              <div class="db-auto-box">
                <div class="row-between">
                  <span class="fld-label"><i class="ri-database-2-line"></i> Other Config — MySQL Database Otomatis (utf8mb4)</span>
                  <button type="button" class="preset" onclick={() => (dbPass = generateRandomPass(14))}>
                    Acak Password
                  </button>
                </div>
                <div class="grid-3">
                  <label class="fld" for="as-db-name">
                    <span class="fld-label">Database Name</span>
                    <input
                      id="as-db-name"
                      class="mono"
                      type="text"
                      bind:value={dbName}
                      oninput={() => (dbTouched = true)}
                    />
                  </label>
                  <label class="fld" for="as-db-user">
                    <span class="fld-label">Username</span>
                    <input
                      id="as-db-user"
                      class="mono"
                      type="text"
                      bind:value={dbUser}
                      oninput={() => (dbTouched = true)}
                    />
                  </label>
                  <label class="fld" for="as-db-pass">
                    <span class="fld-label">Password</span>
                    <input id="as-db-pass" class="mono" type="text" bind:value={dbPass} />
                  </label>
                </div>

                {#if kind === 'php'}
                  <div class="admin-auto-divider"></div>
                  <div class="row-between">
                    <label class="check-inline" for="as-create-admin">
                      <input id="as-create-admin" type="checkbox" bind:checked={createAdmin} />
                      <span><i class="ri-shield-user-line"></i> Buat Akun Super Admin Otomatis (Laravel / Filament Shield)</span>
                      {#if isFilamentRepo}
                        <span class="filament-pill">Filament Terdeteksi</span>
                      {:else if isLaravelRepo}
                        <span class="filament-pill">Laravel Terdeteksi</span>
                      {/if}
                    </label>
                    {#if createAdmin}
                      <button type="button" class="preset" onclick={() => (adminPass = generateRandomPass(12))}>
                        Acak Password Admin
                      </button>
                    {/if}
                  </div>
                  {#if createAdmin}
                    <div class="grid-3">
                      <label class="fld" for="as-admin-name">
                        <span class="fld-label">Admin Name</span>
                        <input
                          id="as-admin-name"
                          type="text"
                          placeholder="Super Admin"
                          bind:value={adminName}
                        />
                      </label>
                      <label class="fld" for="as-admin-email">
                        <span class="fld-label">Email Login</span>
                        <input
                          id="as-admin-email"
                          class="mono"
                          type="email"
                          placeholder="admin@domain.com"
                          bind:value={adminEmail}
                          oninput={() => (adminEmailTouched = true)}
                        />
                      </label>
                      <label class="fld" for="as-admin-pass">
                        <span class="fld-label">Password Login</span>
                        <input
                          id="as-admin-pass"
                          class="mono"
                          type="text"
                          placeholder="Minimal 4 karakter"
                          bind:value={adminPass}
                        />
                      </label>
                    </div>
                  {/if}
                {/if}
              </div>
            {/if}
          {/if}

          {#if submitError}
            <p class="err-line"><i class="ri-error-warning-line"></i> {submitError}</p>
          {/if}
        </div>

        {#if submitting}
          <div class="deploy-progress-panel" role="status" aria-live="polite">
            <div class="deploy-prog-top">
              <div class="deploy-prog-left">
                <i class="ri-loader-4-line spin deploy-prog-icon"></i>
                <div class="deploy-prog-text">
                  <strong>{deployStepLabel}</strong>
                  <small>
                    Tahap aktif: <span class="mono">{deployStage}</span> · Waktu berjalan: <span class="mono">{deployElapsedSec}d</span>
                  </small>
                </div>
              </div>
              <div class="deploy-prog-pct mono">{deployPercent}%</div>
            </div>
            <div class="deploy-prog-track">
              <div class="deploy-prog-fill" style="width: {deployPercent}%"></div>
            </div>
            <div class="stepper deploy-prog-steps">
              <span class="stepper__step dp-step" class:done={deployPercent >= 15} class:active={deployPercent < 15}>
                <i class={deployPercent >= 15 ? 'ri-checkbox-circle-fill' : 'ri-loader-4-line spin'}></i> Vhost Nginx
              </span>
              {#if createDB}
                <span class="stepper__step dp-step" class:done={deployPercent >= 25} class:active={deployPercent >= 15 && deployPercent < 25}>
                  <i class={deployPercent >= 25 ? 'ri-checkbox-circle-fill' : deployPercent >= 15 ? 'ri-loader-4-line spin' : 'ri-checkbox-blank-circle-line'}></i> MySQL DB
                </span>
              {/if}
              {#if activeTab === 'git'}
                <span class="stepper__step dp-step" class:done={deployPercent >= 50} class:active={deployPercent >= 25 && deployPercent < 50}>
                  <i class={deployPercent >= 50 ? 'ri-checkbox-circle-fill' : deployPercent >= 25 ? 'ri-loader-4-line spin' : 'ri-checkbox-blank-circle-line'}></i> Git Clone & .env
                </span>
                <span class="stepper__step dp-step" class:done={deployPercent >= 88} class:active={deployPercent >= 50 && deployPercent < 88}>
                  <i class={deployPercent >= 88 ? 'ri-checkbox-circle-fill' : deployPercent >= 50 ? 'ri-loader-4-line spin' : 'ri-checkbox-blank-circle-line'}></i> Composer & Migrasi
                </span>
              {/if}
              {#if kind === 'php' && createDB && createAdmin}
                <span class="stepper__step dp-step" class:done={deployPercent >= 94} class:active={deployPercent >= 88 && deployPercent < 94}>
                  <i class={deployPercent >= 94 ? 'ri-checkbox-circle-fill' : deployPercent >= 88 ? 'ri-loader-4-line spin' : 'ri-checkbox-blank-circle-line'}></i> Super Admin
                </span>
              {/if}
              {#if autoSSL}
                <span class="stepper__step dp-step" class:done={deployPercent >= 99} class:active={deployPercent >= 94 && deployPercent < 99}>
                  <i class={deployPercent >= 99 ? 'ri-checkbox-circle-fill' : deployPercent >= 94 ? 'ri-loader-4-line spin' : 'ri-checkbox-blank-circle-line'}></i> Auto-SSL
                </span>
              {/if}
            </div>
          </div>
        {/if}

        <footer class="as-foot">
          <button type="button" class="btn btn-ghost" onclick={onClose} disabled={submitting} id="as-cancel-btn">
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" disabled={!canSubmit} id="as-submit-btn">
            {#if submitting}
              <i class="ri-loader-4-line spin"></i> Memproses… {deployPercent}%
            {:else if activeTab === 'batch'}
              <i class="ri-stack-line"></i> Confirm Batch ({parsedBatchRows.length})
            {:else}
              <i class="ri-add-line"></i> Confirm
            {/if}
          </button>
        </footer>
      </form>
    {/if}
  </div>
</Modal>
