<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from './lib/api';
  import type { MetricPoint, OverviewData, Service } from './lib/types';

  import Home from './routes/Home.svelte';
  import Sites from './routes/Sites.svelte';
  import Services from './routes/Services.svelte';
  import Terminal from './routes/Terminal.svelte';
  import Monitor from './routes/Monitor.svelte';
  import Security from './routes/Security.svelte';
  import Assistant from './routes/Assistant.svelte';
  import Settings from './routes/Settings.svelte';
  import Files from './routes/Files.svelte';

  // State Otentikasi Pengguna
  let authenticated = $state(false);
  let currentUser = $state('root');
  let loginUser = $state('root');
  let loginPass = $state('');
  let loginError = $state('');
  let loginSubmitting = $state(false);

  // State Routing (Hash-based)
  let currentRoute = $state('home');

  // State Data Global
  let overview = $state<OverviewData | null>(null);
  let services = $state<Service[]>([]);
  let history = $state<MetricPoint[]>([]);
  let currentMetric = $state<MetricPoint>({
    timestamp: 0,
    cpu: 0,
    ram_used: 0,
    ram_total: 0,
    disk_used: 0,
    disk_total: 0,
    net_in: 0,
    net_out: 0,
    load_avg: 0,
  });

  // State Koneksi SSE
  let sseConnected = $state(false);
  let eventSource: EventSource | null = null;

  // State Toast Notifikasi
  let toastMessage = $state('');
  let toastVisible = $state(false);
  let toastTimer: any = null;

  function showToast(msg: string) {
    toastMessage = msg;
    toastVisible = true;
    if (toastTimer) clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      toastVisible = false;
    }, 2800);
  }

  // State Masking IP & Privasi
  let maskIP = $state(false);

  // Helper Salin Alamat IP
  async function copyIP(ip: string) {
    try {
      await navigator.clipboard.writeText(ip);
      showToast(`IP disalin: ${ip}`);
    } catch {
      showToast(`IP: ${ip}`);
    }
  }

  // Navigasi rute berdasarkan URL Hash
  function updateRoute() {
    const rawHash = window.location.hash.replace(/^#\/?/, '').split('?')[0];
    currentRoute = rawHash || 'home';
  }

  // Inisialisasi Aliran Metrik Server-Sent Events (SSE)
  function startSSE() {
    if (eventSource) eventSource.close();
    eventSource = new EventSource('/api/metrics/live');

    eventSource.addEventListener('history', (e) => {
      try {
        const hist: MetricPoint[] = JSON.parse(e.data);
        history = hist.slice(-60);
      } catch {}
    });

    eventSource.addEventListener('metric', (e) => {
      try {
        const pt: MetricPoint = JSON.parse(e.data);
        currentMetric = pt;
        history = [...history.slice(-59), pt];
        sseConnected = true;
      } catch {}
    });

    eventSource.onopen = () => {
      sseConnected = true;
    };

    eventSource.onerror = () => {
      sseConnected = false;
    };
  }

  async function loadInitialData() {
    try {
      const [ovData, svcData] = await Promise.all([
        apiFetch<OverviewData>('/api/overview'),
        apiFetch<{ services: Service[] }>('/api/services'),
      ]);
      overview = ovData;
      services = svcData.services || [];
      if (ovData.latest_metric) {
        currentMetric = ovData.latest_metric;
      }
    } catch {}
  }

  async function checkAuth() {
    try {
      const me = await apiFetch<{ username: string }>('/api/auth/me');
      authenticated = true;
      currentUser = me.username;
      startSSE();
      await loadInitialData();
    } catch {
      authenticated = false;
    }
  }

  async function handleLogin(e: Event) {
    e.preventDefault();
    loginError = '';
    loginSubmitting = true;

    try {
      const res = await apiFetch<{ ok: boolean; user: { username: string } }>('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({
          username: loginUser.trim(),
          password: loginPass,
        }),
      });

      if (res.ok) {
        authenticated = true;
        currentUser = res.user.username;
        showToast('Login berhasil!');
        startSSE();
        await loadInitialData();
      }
    } catch (err: any) {
      loginError = err.message || 'Kredensial login tidak valid';
    } finally {
      loginSubmitting = false;
    }
  }

  async function handleLogout() {
    try {
      await apiFetch('/api/auth/logout', { method: 'POST' });
    } catch {}
    if (eventSource) eventSource.close();
    authenticated = false;
    showToast('Sesi berakhir');
  }

  async function handleSvcAction(unit: string, action: string) {
    try {
      await apiFetch(`/api/services/${unit}/action`, {
        method: 'POST',
        body: JSON.stringify({ action }),
      });
      showToast(`Aksi ${action} pada ${unit} berhasil`);
      const svcData = await apiFetch<{ services: Service[] }>('/api/services');
      services = svcData.services || [];
    } catch (err: any) {
      showToast(`Gagal: ${err.message}`);
    }
  }

  onMount(() => {
    updateRoute();
    window.addEventListener('hashchange', updateRoute);
    checkAuth();
    return () => {
      window.removeEventListener('hashchange', updateRoute);
      if (eventSource) eventSource.close();
    };
  });
</script>

{#if !authenticated}
  <!-- Ã¢â€¢ÂÃ¢â€¢ÂÃ¢â€¢Â Tampilan Login Ã¢â€¢ÂÃ¢â€¢ÂÃ¢â€¢Â -->
  <div class="login-wrapper">
    <div class="login-box">
      <div class="brand-logo">
        <svg viewBox="0 0 40 40" fill="none" width="48" height="48">
          <rect width="40" height="40" rx="10" fill="#22c55e" />
          <path d="M12 14h16M12 20h12M12 26h16" stroke="#0f172a" stroke-width="2.5" stroke-linecap="round" />
        </svg>
        <h1 class="login-title">fikpanel</h1>
        <p class="login-sub">Server Control Panel Ringan &middot; Svelte 5 + Go</p>
      </div>

      {#if loginError}
        <div class="err-box">{loginError}</div>
      {/if}

      <form onsubmit={handleLogin} class="login-form">
        <div class="field">
          <label for="l-user">Nama Pengguna</label>
          <input id="l-user" type="text" bind:value={loginUser} required />
        </div>

        <div class="field">
          <label for="l-pass">Kata Sandi</label>
          <input
            id="l-pass"
            type="password"
            placeholder="Kosongkan jika mode pengujian lokal"
            bind:value={loginPass}
          />
        </div>

        <button type="submit" class="btn-login" disabled={loginSubmitting}>
          {loginSubmitting ? 'Memverifikasi...' : 'Masuk ke Panel'}
        </button>

        <p class="dev-hint">
          Akun default: <code class="mono">root</code> (kata sandi kosong)
        </p>
      </form>
    </div>
  </div>
{:else}
  <!-- Ã¢â€¢ÂÃ¢â€¢ÂÃ¢â€¢Â Layout Dashboard Utama (Gaya aaPanel Sidebar) Ã¢â€¢ÂÃ¢â€¢ÂÃ¢â€¢Â -->
  <div class="app-layout">
    <!-- Left Sidebar -->
    <aside class="sidebar">
      <div class="sidebar-brand">
        <svg viewBox="0 0 32 32" fill="none" width="28" height="28">
          <rect width="32" height="32" rx="8" fill="#22c55e" />
          <path d="M9 11h14M9 16h10M9 21h14" stroke="#0f172a" stroke-width="2.2" stroke-linecap="round" />
        </svg>
        <span class="brand-text">fikpanel</span>
      </div>

      <nav class="nav-menu">
        <a href="#/home" class="nav-item {currentRoute === 'home' ? 'active' : ''}">
          <i class="ri-dashboard-3-line nav-icon"></i>
          <span>Home</span>
        </a>
        <a href="#/website" class="nav-item {currentRoute === 'website' ? 'active' : ''}">
          <i class="ri-global-line nav-icon"></i>
          <span>Website</span>
        </a>
        <a href="#/services" class="nav-item {currentRoute === 'services' ? 'active' : ''}">
          <i class="ri-server-line nav-icon"></i>
          <span>Services</span>
        </a>
        <a href="#/terminal" class="nav-item {currentRoute === 'terminal' ? 'active' : ''}">
          <i class="ri-terminal-box-line nav-icon"></i>
          <span>Terminal</span>
        </a>
        <a href="#/monitor" class="nav-item {currentRoute === 'monitor' ? 'active' : ''}">
          <i class="ri-line-chart-line nav-icon"></i>
          <span>Monitor</span>
        </a>
        <a href="#/files" class="nav-item {currentRoute === 'files' ? 'active' : ''}">
          <i class="ri-folder-3-line nav-icon"></i>
          <span>Files</span>
        </a>
        <a href="#/security" class="nav-item {currentRoute === 'security' ? 'active' : ''}">
          <i class="ri-shield-keyhole-line nav-icon"></i>
          <span>Security</span>
        </a>
        <a href="#/assistant" class="nav-item {currentRoute === 'assistant' ? 'active' : ''}">
          <i class="ri-sparkling-fill nav-icon"></i>
          <span>AI Assistant</span>
        </a>
        <a href="#/settings" class="nav-item {currentRoute === 'settings' ? 'active' : ''}">
          <i class="ri-settings-4-line nav-icon"></i>
          <span>Settings</span>
        </a>
      </nav>

      <div class="sidebar-foot">
        <div class="panel-ver">v0.1.0-dev &middot; Svelte 5</div>
      </div>
    </aside>

    <!-- Content Area -->
    <div class="main-wrapper">
      <!-- Top Header Bar (Gaya aaPanel) -->
      <header class="topbar">
        <div class="top-left">
          <!-- IP Pill dengan Click-to-Copy & Masking Toggle -->
          <div class="ip-container">
            <button
              class="ip-pill"
              onclick={() => copyIP(overview?.system?.ip || '127.0.0.1')}
              title="Klik untuk menyalin alamat IP"
            >
              <span class="dot-online"></span>
              <span class="mono tabular">{maskIP ? '• • • • • •' : (overview?.system?.ip || '127.0.0.1')}</span>
              <i class="ri-file-copy-line copy-icon"></i>
            </button>
            <button
              class="btn-eye"
              onclick={() => (maskIP = !maskIP)}
              title={maskIP ? 'Tampilkan IP' : 'Sembunyikan IP untuk screenshot'}
            >
              <i class={maskIP ? 'ri-eye-off-line' : 'ri-eye-line'}></i>
            </button>
          </div>

          <!-- Badge OS / Arsitektur -->
          <div class="os-badge">
            <i class="ri-computer-line"></i>
            <span>{overview?.system?.os || 'Ubuntu'}</span>
          </div>

          <!-- Badge Live dengan Animasi Ping Dua Lapis -->
          <div class="sse-pill {sseConnected ? 'sse-on' : 'sse-warn'}">
            <div class="live-ping-wrap">
              <span class="live-ping-outer"></span>
              <span class="live-ping-inner"></span>
            </div>
            <span>{sseConnected ? 'Live' : 'Reconnecting'}</span>
          </div>
        </div>

        <div class="top-right">
          <a href="#/terminal" class="top-action-btn" title="Buka Terminal">
            <i class="ri-terminal-line"></i> <span>Terminal</span>
          </a>
          <button class="top-action-btn" onclick={loadInitialData} title="Refresh Metrik">
            <i class="ri-refresh-line"></i> <span>Refresh</span>
          </button>
          <div class="user-badge">
            <i class="ri-user-3-line user-avatar"></i>
            <span class="user-name">{currentUser}</span>
          </div>
          <button class="btn-logout" onclick={handleLogout}>Keluar</button>
        </div>
      </header>

      <!-- Main Body View -->
      <main class="page-content">
        {#if currentRoute === 'home'}
          <Home
            {overview}
            {currentMetric}
            {history}
            {services}
            onSvcAction={handleSvcAction}
          />
        {:else if currentRoute === 'website'}
          <Sites onNotify={showToast} />
        {:else if currentRoute === 'services'}
          <Services
            {services}
            onSvcAction={handleSvcAction}
            onRefresh={loadInitialData}
            onNotify={showToast}
          />
        {:else if currentRoute === 'terminal'}
          <Terminal />
        {:else if currentRoute === 'monitor'}
          <Monitor {currentMetric} {history} />
        {:else if currentRoute === 'files'}
          <Files onNotify={showToast} />
        {:else if currentRoute === 'security'}
          <Security />
        {:else if currentRoute === 'assistant'}
          <Assistant onSvcAction={handleSvcAction} onNotify={showToast} />
        {:else if currentRoute === 'settings'}
          <Settings {overview} onNotify={showToast} />
        {/if}
      </main>
    </div>
  </div>
{/if}

<!-- Toast Notifikasi Global -->
<div class="toast {toastVisible ? 'toast-show' : ''}">
  {toastMessage}
</div>

<style>
  :global(*) {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
  }
  :global(body) {
    background: #090a0f !important;
    color: #f1f5f9;
    font-family: 'Plus Jakarta Sans', system-ui, -apple-system, sans-serif;
    min-height: 100vh;
    overflow-x: hidden;
  }
  .mono {
    font-family: 'Fira Code', monospace;
  }
  .tabular {
    font-variant-numeric: tabular-nums;
  }

  /* --- Halaman Login --- */
  .login-wrapper {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    padding: 1.5rem;
    background: #090a0f;
  }
  .login-box {
    background: #0d0f17;
    border: 1px solid #1c2230;
    border-radius: 12px;
    width: 100%;
    max-width: 380px;
    padding: 2rem;
    box-shadow: 0 16px 36px rgba(0, 0, 0, 0.6);
  }
  .brand-logo {
    text-align: center;
    margin-bottom: 1.5rem;
  }
  .login-title {
    font-size: 1.35rem;
    font-weight: 700;
    margin-top: 0.65rem;
    color: #f1f5f9;
    letter-spacing: -0.02em;
  }
  .login-sub {
    font-size: 0.78rem;
    color: #8b95a5;
    margin-top: 0.25rem;
  }
  .err-box {
    background: rgba(244, 63, 94, 0.08);
    border: 1px solid rgba(244, 63, 94, 0.25);
    color: #f43f5e;
    padding: 0.55rem 0.75rem;
    border-radius: 6px;
    font-size: 0.78rem;
    margin-bottom: 1rem;
  }
  .field {
    margin-bottom: 1rem;
  }
  label {
    display: block;
    font-size: 0.75rem;
    font-weight: 500;
    color: #cbd5e1;
    margin-bottom: 0.35rem;
  }
  input {
    width: 100%;
    padding: 0.6rem 0.8rem;
    background: #090b10;
    border: 1px solid #1c2230;
    border-radius: 6px;
    color: #f1f5f9;
    font-size: 0.85rem;
    outline: none;
    transition: border-color 0.15s ease;
  }
  input:focus {
    border-color: #38bdf8;
  }
  .btn-login {
    width: 100%;
    padding: 0.65rem;
    border-radius: 6px;
    background: #10b981;
    color: #05130b;
    font-size: 0.85rem;
    font-weight: 600;
    cursor: pointer;
    border: none;
    transition: background 0.15s ease;
    margin-top: 0.5rem;
  }
  .btn-login:hover {
    background: #059669;
  }
  .dev-hint {
    text-align: center;
    font-size: 0.7rem;
    color: #64748b;
    margin-top: 1.25rem;
  }
  .dev-hint code {
    color: #10b981;
    background: rgba(16, 185, 129, 0.08);
    padding: 0.1rem 0.3rem;
    border-radius: 4px;
  }

  /* --- Tata Letak Panel (Sidebar + Wrapper) --- */
  .app-layout {
    display: flex;
    min-height: 100vh;
    background: #090a0f;
  }
  .sidebar {
    width: 220px;
    background: #0d0f17;
    border-right: 1px solid #181b24;
    display: flex;
    flex-direction: column;
    position: fixed;
    top: 0;
    bottom: 0;
    left: 0;
    z-index: 50;
  }
  .sidebar-brand {
    height: 54px;
    display: flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0 1.25rem;
    border-bottom: 1px solid #181b24;
  }
  .brand-text {
    font-size: 1.05rem;
    font-weight: 700;
    letter-spacing: -0.02em;
    color: #f1f5f9;
  }
  .nav-menu {
    flex: 1;
    padding: 0.85rem 0.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    overflow-y: auto;
  }
  .nav-item {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0.55rem 0.75rem;
    border-radius: 6px;
    color: #8b95a5;
    text-decoration: none;
    font-size: 0.82rem;
    font-weight: 500;
    transition: background 0.15s ease, color 0.15s ease;
  }
  .nav-item:hover {
    background: #141722;
    color: #f1f5f9;
  }
  .nav-item.active {
    background: #161925;
    color: #f1f5f9;
    font-weight: 600;
    border-left: 2px solid #38bdf8;
  }
  .nav-item.active .nav-icon {
    color: #38bdf8;
  }
  .nav-icon {
    font-size: 1rem;
  }
  .sidebar-foot {
    padding: 0.75rem 1.25rem;
    border-top: 1px solid #181b24;
    font-size: 0.68rem;
    color: #545e6f;
  }

  .main-wrapper {
    flex: 1;
    margin-left: 220px;
    display: flex;
    flex-direction: column;
    min-height: 100vh;
  }

  /* --- Topbar Navigasi --- */
  .topbar {
    height: 54px;
    background: #0d0f17;
    border-bottom: 1px solid #181b24;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 1.5rem;
    position: sticky;
    top: 0;
    z-index: 40;
  }
  .top-left,
  .top-right {
    display: flex;
    align-items: center;
    gap: 0.65rem;
  }
  .ip-container {
    display: inline-flex;
    align-items: center;
    background: #11141e;
    border: 1px solid #1c2230;
    border-radius: 6px;
    padding: 0.1rem;
  }
  .ip-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    background: transparent;
    border: none;
    color: #f1f5f9;
    font-size: 0.75rem;
    padding: 0.2rem 0.55rem;
    cursor: pointer;
    border-radius: 4px;
    transition: background 0.15s ease;
  }
  .ip-pill:hover {
    background: #191e2b;
  }
  .dot-online {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: #10b981;
  }
  .copy-icon {
    font-size: 0.75rem;
    color: #64748b;
  }
  .btn-eye {
    background: transparent;
    border: none;
    color: #545e6f;
    padding: 0.2rem 0.4rem;
    cursor: pointer;
    font-size: 0.85rem;
    border-radius: 4px;
    transition: color 0.15s ease;
  }
  .btn-eye:hover {
    color: #cbd5e1;
  }
  .os-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background: #11141e;
    border: 1px solid #1c2230;
    color: #8b95a5;
    font-size: 0.72rem;
    padding: 0.22rem 0.6rem;
    border-radius: 6px;
  }
  .sse-pill {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.7rem;
    font-weight: 500;
    padding: 0.2rem 0.55rem;
    border-radius: 999px;
    background: #11141e;
    border: 1px solid #1c2230;
  }
  .sse-on {
    color: #10b981;
  }
  .sse-warn {
    color: #f59e0b;
  }
  .live-ping-wrap {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 6px;
    height: 6px;
  }
  .live-ping-outer {
    display: none;
  }
  .live-ping-inner {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: currentColor;
  }
  .top-action-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background: #11141e;
    border: 1px solid #1c2230;
    color: #cbd5e1;
    font-size: 0.72rem;
    padding: 0.28rem 0.6rem;
    border-radius: 6px;
    text-decoration: none;
    cursor: pointer;
    transition: background 0.15s ease, border-color 0.15s ease;
  }
  .top-action-btn:hover {
    background: #181d2a;
    border-color: #2b3448;
    color: #f1f5f9;
  }
  .user-badge {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.75rem;
    color: #cbd5e1;
    background: #11141e;
    border: 1px solid #1c2230;
    padding: 0.25rem 0.6rem;
    border-radius: 6px;
  }
  .user-avatar {
    font-size: 0.85rem;
    color: #8b95a5;
  }
  .btn-logout {
    background: rgba(244, 63, 94, 0.08);
    color: #f43f5e;
    border: 1px solid rgba(244, 63, 94, 0.2);
    padding: 0.28rem 0.65rem;
    border-radius: 6px;
    font-size: 0.72rem;
    font-weight: 500;
    cursor: pointer;
    transition: background 0.15s ease;
  }
  .btn-logout:hover {
    background: rgba(244, 63, 94, 0.16);
  }

  /* --- Konten Halaman Utama --- */
  .page-content {
    flex: 1;
    padding: 1.25rem 1.5rem;
    max-width: 1400px;
    width: 100%;
    margin: 0 auto;
  }

  /* --- Notifikasi Toast Global --- */
  .toast {
    position: fixed;
    bottom: 1.5rem;
    right: 1.5rem;
    background: #11141d;
    border: 1px solid #232a3c;
    color: #f1f5f9;
    padding: 0.65rem 1.15rem;
    border-radius: 6px;
    font-size: 0.8rem;
    font-weight: 500;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.6);
    opacity: 0;
    transform: translateY(8px);
    transition: all 0.2s ease;
    pointer-events: none;
    z-index: 999;
  }
  .toast-show {
    opacity: 1;
    transform: translateY(0);
    pointer-events: auto;
  }

  @media (max-width: 768px) {
    .sidebar {
      width: 54px;
    }
    .brand-text,
    .nav-item span:last-child,
    .sidebar-foot {
      display: none;
    }
    .nav-item {
      justify-content: center;
      padding: 0.65rem;
    }
    .main-wrapper {
      margin-left: 54px;
    }
  }
</style>
