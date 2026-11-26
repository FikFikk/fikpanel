<script lang="ts">
  import '../../styles/pages/sites/sites-table.css';
  import type { Site } from '../../lib/types';

  // Antarmuka properti untuk tampilan tabel & kartu grid daftar situs web
  interface Props {
    loading: boolean;
    filteredSites: Site[];
    selectedIds: Set<number>;
    backingUpSiteId: number | null;
    restartingSiteId: number | null;
    viewMode: 'table' | 'grid';
    openMoreSiteId: number | null;
    onToggleSelectAll: (e: Event) => void;
    onToggleSelectSite: (id: number) => void;
    onToggleSite: (id: number) => void;
    onRestartProcess: (s: Site) => void;
    onTriggerBackup: (s: Site) => void;
    onOpenInFileManager: (dir: string) => void;
    onOpenInTerminal: (dir: string) => void;
    onOpenSettings: (s: Site, tab?: string) => void;
    onOpenGitPullModal: (s: Site) => void;
    onOpenLogViewer: (s: Site) => void;
    onOpenDeleteModal: (s: Site) => void;
    onToggleMoreMenu: (id: number, e: MouseEvent) => void;
    onShowAddModal: () => void;
    getProjectRootDir: (s: Site) => string;
    sslDaysLeft: (s: Site) => number;
  }

  let {
    loading,
    filteredSites,
    selectedIds,
    backingUpSiteId,
    restartingSiteId,
    viewMode,
    openMoreSiteId = $bindable(),
    onToggleSelectAll,
    onToggleSelectSite,
    onToggleSite,
    onRestartProcess,
    onTriggerBackup,
    onOpenInFileManager,
    onOpenInTerminal,
    onOpenSettings,
    onOpenGitPullModal,
    onOpenLogViewer,
    onOpenDeleteModal,
    onToggleMoreMenu,
    onShowAddModal,
    getProjectRootDir,
    sslDaysLeft,
  }: Props = $props();

  // Hitung nilai maksimum permintaan untuk lebar bilah proporsional nyata
  let maxRequests = $derived(Math.max(1, ...filteredSites.map((item) => item.request_count ?? 0)));

  // Ringkasan kesehatan live seluruh situs yang tampil pada footer
  let statusSummary = $derived.by(() => {
    let online = 0;
    let down = 0;
    let degraded = 0;
    let stopped = 0;
    for (const s of filteredSites) {
      const st = s.live_status || (s.enabled === 1 ? 'running' : 'stopped');
      if (st === 'running') online++;
      else if (st === 'down') down++;
      else if (st === 'degraded') degraded++;
      else stopped++;
    }
    return { online, down, degraded, stopped };
  });

  function siteHref(s: Site): string {
    const hasSSL = s.ssl_mode === 'cloudflare' || (s.ssl_status === 'issued' && s.ssl_mode !== 'none');
    return `${hasSSL ? 'https' : 'http'}://${s.domain}`;
  }

  function liveStatusMeta(s: Site): { cls: string; icon: string; label: string } {
    const st = s.live_status || (s.enabled === 1 ? 'running' : 'stopped');
    if (st === 'running') {
      return { cls: 'is-online', icon: 'ri-checkbox-circle-fill', label: 'Online' };
    }
    if (st === 'down') {
      return { cls: 'is-down', icon: 'ri-error-warning-fill', label: 'Down' };
    }
    if (st === 'degraded') {
      return { cls: 'is-degraded', icon: 'ri-alert-fill', label: 'Degraded' };
    }
    return { cls: 'is-paused', icon: 'ri-pause-circle-line', label: 'Stopped' };
  }

  // Format label runtime bebas duplikasi (mencegah 'Node.js (Node.js)' atau 'Go (Go)')
  function formatRuntimeLabel(s: Site): string {
    const fw = (s.framework || '').trim();
    const fwLow = fw.toLowerCase();
    if (s.kind === 'php') {
      const base = s.php_version ? `PHP ${s.php_version}` : 'PHP';
      if (fw && fwLow !== 'php') return `${base} · ${fw}`;
      return base;
    }
    if (s.kind === 'node') {
      if (fw && fwLow !== 'node' && fwLow !== 'node.js' && fwLow !== 'nodejs') return `Node.js · ${fw}`;
      return 'Node.js';
    }
    if (s.kind === 'go') {
      if (fw && fwLow !== 'go' && fwLow !== 'golang' && fwLow !== 'go service') return `Go · ${fw}`;
      return 'Go Service';
    }
    if (s.kind === 'python') {
      if (fw && fwLow !== 'python') return `Python · ${fw}`;
      return 'Python';
    }
    if (s.kind === 'static') {
      if (fw && fwLow !== 'static' && fwLow !== 'html') return fw;
      return 'Static';
    }
    if (fw && fwLow !== 'proxy') return fw;
    return 'Proxy';
  }

  // Target utama akurat: tampilkan upstream (port) untuk Node/Go/Python/Proxy alih-alih '/var/www/html'
  function formatPrimaryTarget(s: Site): { icon: string; text: string; title: string } {
    const hasUpstream = Boolean(s.upstream && s.upstream.trim());
    if (hasUpstream && (s.kind === 'proxy' || s.kind === 'node' || s.kind === 'go' || s.kind === 'python')) {
      const cleanUps = s.upstream.replace(/^https?:\/\//, '');
      const extraPath = s.root_path && s.root_path !== '/var/www/html' ? ` · ${s.root_path}` : '';
      return {
        icon: 'ri-plug-line',
        text: cleanUps,
        title: `Upstream: ${s.upstream}${extraPath}`,
      };
    }
    const dir = s.root_path || `/home/fikpanel/web/${s.domain}`;
    return {
      icon: 'ri-folder-2-line',
      text: dir,
      title: `Direktori root: ${dir}`,
    };
  }

  // Saring deskripsi adopsi otomatis berulang agar tampilan bersih
  function cleanUserDescription(desc?: string): string {
    const d = (desc || '').trim();
    if (!d) return '';
    if (d.toLowerCase().includes('diadopsi secara otomatis dari host server')) {
      return '';
    }
    return d;
  }

  // Periksa apakah situs mendukung tombol restart proses langsung (PM2 / systemd / PHP-FPM)
  function canRestartProcess(s: Site): boolean {
    return s.kind === 'node' || s.kind === 'go' || s.kind === 'python' || s.kind === 'php' || Boolean(s.process_manager);
  }

  function requestSourceHint(source?: string): string {
    switch (source) {
      case 'nginx':
        return 'Log akses khusus Nginx';
      case 'global_log':
        return 'Log akses global Nginx';
      case 'waf':
        return 'Catatan event WAF';
      case 'tunnel':
        return 'Cloudflare Tunnel / direct upstream (tidak melewati log Nginx)';
      default:
        return 'Belum ada catatan log akses Nginx';
    }
  }

  function requestBarPct(count?: number): number {
    const n = count ?? 0;
    if (n <= 0) return 0;
    return Math.max(8, Math.min(100, Math.round((n / maxRequests) * 100)));
  }
</script>

<div class="table-container pg-sites-table" class:is-grid-mode={viewMode === 'grid'}>
  {#if loading}
    <div class="state state--loading">
      <i class="ri-loader-4-line spin state__icon" aria-hidden="true"></i>
      <span class="state__title">Memuat daftar situs web...</span>
    </div>
  {:else if filteredSites.length === 0}
    <div class="state state--empty">
      <i class="ri-global-line state__icon" aria-hidden="true"></i>
      <p class="state__desc">Tidak ada situs yang cocok dengan filter atau pencarian saat ini.</p>
      <button type="button" class="btn btn--primary" onclick={onShowAddModal}>
        + Deploy / Tambah Situs Baru
      </button>
    </div>
  {:else if viewMode === 'grid'}
    <!-- ═════════ MODE KARTU GRID MODERN (PaaS View) ═════════ -->
    <div class="sites-card-grid">
      {#each filteredSites as s (s.id)}
        {@const stMeta = liveStatusMeta(s)}
        {@const target = formatPrimaryTarget(s)}
        {@const userDesc = cleanUserDescription(s.description)}
        {@const isRestarting = restartingSiteId === s.id}
        {@const isDown = stMeta.cls === 'is-down' || stMeta.cls === 'is-degraded'}
        <article class="site-card" class:is-down-card={isDown} class:selected-row={selectedIds.has(s.id)}>
          <div class="sc-head">
            <div class="sc-title-wrap">
              <a href={siteHref(s)} target="_blank" rel="noopener noreferrer" class="domain-link" title="Buka {siteHref(s)}">
                <span class="domain-text">{s.domain}</span>
                <i class="ri-external-link-line icon-ext" aria-hidden="true"></i>
              </a>
              <div class="site-path-sub mono" title={target.title}>
                <i class={target.icon} aria-hidden="true"></i>
                <span>{target.text}</span>
              </div>
            </div>
            <button
              type="button"
              class="btn-status-play {stMeta.cls}"
              onclick={() => onToggleSite(s.id)}
              title="{s.live_detail || 'Status situs'} — Klik untuk toggle vhost"
            >
              <i class={stMeta.icon} aria-hidden="true"></i>
              <span>{stMeta.label}</span>
            </button>
          </div>

          <div class="sc-badges">
            <button
              type="button"
              class="badge-runtime {s.kind}"
              onclick={() => onOpenSettings(s, s.kind === 'php' ? 'php' : s.kind === 'static' ? 'directory' : 'proxy')}
            >
              {formatRuntimeLabel(s)}
            </button>
            {#if s.process_manager}
              <span class="proc-pill mono" title="Process manager: {s.process_unit || s.process_manager}">
                {s.process_manager.toUpperCase()}
              </span>
            {/if}
            {#if s.queue_worker_enabled}
              <span class="proc-pill is-queue mono" title="Laravel Queue Worker aktif">QUEUE</span>
            {/if}
            {#if s.ssl_mode === 'cloudflare'}
              <button type="button" class="ssl-badge ssl-cloudflare" onclick={() => onOpenSettings(s, 'ssl')}>
                <i class="ri-cloud-line" aria-hidden="true"></i> Cloudflare
              </button>
            {:else if s.ssl_status === 'issued' && s.ssl_mode !== 'none'}
              <button type="button" class="ssl-badge ssl-active" class:ssl-expiring={sslDaysLeft(s) < 15} onclick={() => onOpenSettings(s, 'ssl')}>
                <i class="ri-shield-check-fill" aria-hidden="true"></i> {sslDaysLeft(s)} Hari
              </button>
            {:else}
              <button type="button" class="ssl-badge ssl-inactive" onclick={() => onOpenSettings(s, 'ssl')}>
                <i class="ri-shield-line" aria-hidden="true"></i> Pasang SSL
              </button>
            {/if}
          </div>

          {#if s.has_git}
            <button type="button" class="git-meta-pill mono" onclick={() => onOpenGitPullModal(s)}>
              <i class="ri-git-branch-line" aria-hidden="true"></i>
              <span>{s.git_repo || 'git'}</span>
              <span class="git-branch-tag">{s.git_branch || 'main'}</span>
              {#if s.git_commit}<span class="git-commit-hash">#{s.git_commit}</span>{/if}
            </button>
          {/if}

          {#if userDesc}
            <p class="sc-desc">{userDesc}</p>
          {/if}

          <div class="sc-foot">
            <div class="quick-actions-bar">
              <button type="button" class="quick-act-btn" onclick={() => onOpenInFileManager(getProjectRootDir(s))} aria-label="Buka berkas" title="Berkas">
                <i class="ri-folder-open-fill icon-amber" aria-hidden="true"></i>
              </button>
              <button type="button" class="quick-act-btn" onclick={() => onOpenInTerminal(getProjectRootDir(s))} aria-label="Buka terminal" title="Terminal">
                <i class="ri-terminal-box-line icon-cyan" aria-hidden="true"></i>
              </button>
              {#if s.kind !== 'static' && s.kind !== 'proxy'}
                <button type="button" class="quick-act-btn" onclick={() => onOpenSettings(s, 'env')} aria-label="Edit .env" title="Editor .env">
                  <i class="ri-key-2-line icon-gray" aria-hidden="true"></i>
                </button>
              {/if}
              <button type="button" class="quick-act-btn" onclick={() => onOpenLogViewer(s)} aria-label="Lihat log" title="Log Akses & Error">
                <i class="ri-file-list-3-line icon-gray" aria-hidden="true"></i>
              </button>
            </div>
            <div class="sc-foot-right">
              {#if isDown && canRestartProcess(s)}
                <button type="button" class="btn-op-restart" disabled={isRestarting} onclick={() => onRestartProcess(s)}>
                  <i class={isRestarting ? 'ri-loader-4-line spin' : 'ri-restart-line'} aria-hidden="true"></i>
                  <span>Restart</span>
                </button>
              {/if}
              {#if s.has_git}
                <button type="button" class="btn-op-git" onclick={() => onOpenGitPullModal(s)}>
                  <i class="ri-git-pull-request-line" aria-hidden="true"></i>
                  <span>Deploy</span>
                </button>
              {/if}
              <button type="button" class="btn-op-conf-pill" onclick={() => onOpenSettings(s, 'domain')}>
                <i class="ri-settings-4-line" aria-hidden="true"></i>
                <span>Kelola</span>
              </button>
            </div>
          </div>
        </article>
      {/each}
    </div>
  {:else}
    <!-- ═════════ MODE TABEL RINGKAS & INFORMATIF ═════════ -->
    <table class="sites-table">
      <thead>
        <tr>
          <th class="col-chk">
            <input
              type="checkbox"
              aria-label="Pilih semua situs"
              checked={selectedIds.size === filteredSites.length && filteredSites.length > 0}
              onchange={onToggleSelectAll}
            />
          </th>
          <th class="col-name">Situs &amp; Target</th>
          <th class="col-runtime">Runtime</th>
          <th class="col-status">Status</th>
          <th class="col-ssl">SSL</th>
          <th class="col-req">Trafik</th>
          <th class="col-ops">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#each filteredSites as s (s.id)}
          {@const stMeta = liveStatusMeta(s)}
          {@const target = formatPrimaryTarget(s)}
          {@const userDesc = cleanUserDescription(s.description)}
          {@const isBackingUp = backingUpSiteId === s.id}
          {@const isRestarting = restartingSiteId === s.id}
          {@const isDown = stMeta.cls === 'is-down' || stMeta.cls === 'is-degraded'}
          <tr class="site-row {selectedIds.has(s.id) ? 'selected-row' : ''}">
            <td class="col-chk">
              <input
                type="checkbox"
                aria-label="Pilih situs {s.domain}"
                checked={selectedIds.has(s.id)}
                onchange={() => onToggleSelectSite(s.id)}
              />
            </td>

            <!-- Kolom Situs, Target Upstream/Direktori & Metadata Git -->
            <td class="col-name">
              <div class="site-domain-box">
                <a
                  href={siteHref(s)}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="domain-link"
                  title="Buka {siteHref(s)} di tab baru"
                >
                  <span class="domain-text">{s.domain}</span>
                  <i class="ri-external-link-line icon-ext" aria-hidden="true"></i>
                </a>
                <div class="site-path-sub mono" title={target.title}>
                  <i class={target.icon} aria-hidden="true"></i>
                  <span>{target.text}</span>
                </div>
                {#if s.has_git || s.has_cicd}
                  <div class="site-git-meta mono">
                    {#if s.has_git}
                      <button
                        type="button"
                        class="git-meta-pill"
                        onclick={() => onOpenGitPullModal(s)}
                        title="Klik untuk Deploy, Webhook & Rollback ({s.git_repo || 'origin'} • {s.git_branch || 'main'})"
                      >
                        <i class="ri-git-branch-line" aria-hidden="true"></i>
                        <span>{s.git_repo || 'git'}</span>
                        <span class="git-branch-tag">{s.git_branch || 'main'}</span>
                        {#if s.git_commit}
                          <span class="git-commit-hash">#{s.git_commit}</span>
                        {/if}
                      </button>
                    {/if}
                    {#if s.has_cicd}
                      <span class="cicd-pill {s.cicd_provider || 'github-actions'}">
                        <i class={s.cicd_provider === 'actions-runner' ? 'ri-cpu-line' : 'ri-github-fill'} aria-hidden="true"></i>
                        <span>{s.cicd_provider === 'actions-runner' ? 'Action Runner' : s.cicd_provider === 'gitlab-ci' ? 'GitLab CI' : 'CI/CD'}</span>
                      </span>
                    {/if}
                  </div>
                {/if}
                {#if userDesc}
                  <div class="site-path-sub" title={userDesc}>
                    <i class="ri-sticky-note-line" aria-hidden="true"></i>
                    <span>{userDesc}</span>
                  </div>
                {/if}
              </div>
            </td>

            <!-- Kolom Runtime, Process Manager & Akses Cepat -->
            <td class="col-runtime">
              <div class="runtime-stack">
                <div class="runtime-badges-row">
                  <button
                    type="button"
                    class="badge-runtime {s.kind}"
                    onclick={() => onOpenSettings(s, s.kind === 'php' ? 'php' : s.kind === 'static' ? 'directory' : 'proxy')}
                    title="Atur runtime & port situs"
                  >
                    {formatRuntimeLabel(s)}
                  </button>
                  {#if s.process_manager}
                    <span class="proc-pill mono" title="Dikelola oleh {s.process_manager}: {s.process_unit || s.domain}">
                      {s.process_manager.toUpperCase()}
                    </span>
                  {/if}
                  {#if s.queue_worker_enabled}
                    <span class="proc-pill is-queue mono" title="Laravel Queue Worker aktif">QUEUE</span>
                  {/if}
                </div>
                <div class="quick-actions-bar">
                  <button
                    type="button"
                    class="quick-act-btn"
                    onclick={() => onOpenInFileManager(getProjectRootDir(s))}
                    aria-label="Buka berkas {s.domain}"
                    title="Buka di File Manager"
                  >
                    <i class="ri-folder-open-fill icon-amber" aria-hidden="true"></i>
                  </button>
                  <button
                    type="button"
                    class="quick-act-btn"
                    onclick={() => onOpenInTerminal(getProjectRootDir(s))}
                    aria-label="Buka terminal {s.domain}"
                    title="Buka di Terminal"
                  >
                    <i class="ri-terminal-box-line icon-cyan" aria-hidden="true"></i>
                  </button>
                  {#if s.kind !== 'static' && s.kind !== 'proxy'}
                    <button
                      type="button"
                      class="quick-act-btn"
                      onclick={() => onOpenSettings(s, 'env')}
                      aria-label="Edit berkas .env {s.domain}"
                      title="Editor .env Aman"
                    >
                      <i class="ri-key-2-line icon-gray" aria-hidden="true"></i>
                    </button>
                  {/if}
                  <button
                    type="button"
                    class="quick-act-btn"
                    disabled={backingUpSiteId !== null}
                    onclick={() => onTriggerBackup(s)}
                    aria-label="Cadangkan {s.domain}"
                    title="{s.backup_count ?? 0} arsip cadangan — klik untuk backup sekarang"
                  >
                    <i class={isBackingUp ? 'ri-loader-4-line spin icon-cyan' : 'ri-archive-line icon-gray'} aria-hidden="true"></i>
                  </button>
                </div>
              </div>
            </td>

            <!-- Kolom Status Operasional Live + Tombol Restart Proses jika Down -->
            <td class="col-status">
              <div class="status-cell-stack">
                <button
                  type="button"
                  class="btn-status-play {stMeta.cls}"
                  onclick={() => onToggleSite(s.id)}
                  title="{s.live_detail || (s.enabled === 1 ? 'Situs aktif' : 'Situs dihentikan')} — Klik untuk {s.enabled === 1 ? 'menghentikan' : 'menjalankan'} vhost"
                >
                  <i class={stMeta.icon} aria-hidden="true"></i>
                  <span>{stMeta.label}</span>
                </button>
                {#if isDown && canRestartProcess(s)}
                  <button
                    type="button"
                    class="btn-inline-restart"
                    disabled={isRestarting}
                    onclick={() => onRestartProcess(s)}
                    title="Hidupkan atau restart proses aplikasi ({s.process_manager || s.kind})"
                  >
                    <i class={isRestarting ? 'ri-loader-4-line spin' : 'ri-restart-line'} aria-hidden="true"></i>
                    <span>{isRestarting ? 'Memulai…' : 'Restart App'}</span>
                  </button>
                {/if}
              </div>
            </td>

            <!-- Kolom Sertifikat SSL -->
            <td class="col-ssl">
              {#if s.ssl_mode === 'cloudflare'}
                <button type="button" class="ssl-badge ssl-cloudflare" onclick={() => onOpenSettings(s, 'ssl')} title="Cloudflare Edge SSL Aktif">
                  <i class="ri-cloud-line" aria-hidden="true"></i>
                  <span>Cloudflare</span>
                </button>
              {:else if s.ssl_status === 'issued' && s.ssl_mode !== 'none'}
                <button
                  type="button"
                  class="ssl-badge ssl-active"
                  class:ssl-expiring={sslDaysLeft(s) < 15}
                  onclick={() => onOpenSettings(s, 'ssl')}
                  title="Berlaku sampai {new Date(s.ssl_expires * 1000).toLocaleDateString('id-ID')}"
                >
                  <i class="ri-shield-check-fill" aria-hidden="true"></i>
                  <span>{sslDaysLeft(s)} Hari</span>
                </button>
              {:else if s.ssl_status === 'pending'}
                <button type="button" class="ssl-badge ssl-pending" onclick={() => onOpenSettings(s, 'ssl')}>
                  <i class="ri-loader-4-line spin" aria-hidden="true"></i>
                  <span>Menerbitkan</span>
                </button>
              {:else if s.ssl_status === 'failed' && s.ssl_mode !== 'none'}
                <button type="button" class="ssl-badge ssl-failed" onclick={() => onOpenSettings(s, 'ssl')}>
                  <i class="ri-shield-cross-line" aria-hidden="true"></i>
                  <span>Gagal</span>
                </button>
              {:else}
                <button type="button" class="ssl-badge ssl-inactive" onclick={() => onOpenSettings(s, 'ssl')}>
                  <i class="ri-shield-line" aria-hidden="true"></i>
                  <span>Pasang SSL</span>
                </button>
              {/if}
            </td>

            <!-- Kolom Permintaan / Trafik Nyata -->
            <td class="col-req mono">
              {#if (s.request_count ?? 0) === 0 && s.request_source === 'tunnel'}
                <button
                  type="button"
                  class="req-tunnel-tag"
                  onclick={() => onOpenLogViewer(s)}
                  title="Trafik masuk via Cloudflare Tunnel langsung ke {s.upstream || 'upstream lokal'}"
                >
                  <i class="ri-route-line" aria-hidden="true"></i>
                  <span>Tunnel</span>
                </button>
              {:else}
                <button
                  type="button"
                  class="req-stat-box req-btn-trigger"
                  onclick={() => onOpenLogViewer(s)}
                  title="Total {(s.request_count ?? 0).toLocaleString('id-ID')} permintaan • {requestSourceHint(s.request_source)}"
                >
                  <span class="req-num">{(s.request_count ?? 0).toLocaleString('id-ID')}</span>
                  <span class="req-meter" aria-hidden="true">
                    <span class="req-meter__fill" style="--bar-w: {requestBarPct(s.request_count)}%"></span>
                  </span>
                </button>
              {/if}
            </td>

            <!-- Kolom Aksi (Deploy + Kelola Selalu Tersedia + More) -->
            <td class="col-ops">
              <div class="ops-wrap">
                {#if s.has_git}
                  <button
                    type="button"
                    class="btn-op-git"
                    onclick={() => onOpenGitPullModal(s)}
                    title="Pull, Auto-Deploy, Webhook & Rollback ({s.git_branch || 'main'})"
                  >
                    <i class="ri-git-pull-request-line" aria-hidden="true"></i>
                    <span>Deploy</span>
                  </button>
                {/if}

                <button
                  type="button"
                  class="btn-op-conf-pill"
                  onclick={() => onOpenSettings(s, 'domain')}
                  title="Pengaturan lengkap situs, .env, runtime, SSL & Nginx"
                >
                  <i class="ri-settings-4-line" aria-hidden="true"></i>
                  <span>Kelola</span>
                </button>

                <div class="op-more-wrap">
                  <button
                    type="button"
                    class="btn-op-more {openMoreSiteId === s.id ? 'is-open' : ''}"
                    onclick={(e) => onToggleMoreMenu(s.id, e)}
                    aria-label="Menu lainnya untuk {s.domain}"
                    title="Menu operasi lengkap"
                  >
                    <i class="ri-more-2-fill" aria-hidden="true"></i>
                  </button>
                  {#if openMoreSiteId === s.id}
                    <div class="op-more-menu" role="menu">
                      {#if canRestartProcess(s)}
                        <button
                          type="button"
                          class="op-more-item"
                          onclick={() => { openMoreSiteId = null; onRestartProcess(s); }}
                        >
                          <i class="ri-restart-line icon-cyan" aria-hidden="true"></i>
                          <div>
                            <b>Restart Proses Aplikasi</b>
                            <small>Muat ulang {s.process_manager ? s.process_manager.toUpperCase() : s.kind === 'php' ? 'PHP-FPM' : 'service'}</small>
                          </div>
                        </button>
                      {/if}
                      {#if s.kind !== 'static' && s.kind !== 'proxy'}
                        <button
                          type="button"
                          class="op-more-item"
                          onclick={() => { openMoreSiteId = null; onOpenSettings(s, 'env'); }}
                        >
                          <i class="ri-key-2-line icon-amber" aria-hidden="true"></i>
                          <div>
                            <b>Editor Berkas .env</b>
                            <small>Ubah variabel environment &amp; rahasia secara aman</small>
                          </div>
                        </button>
                      {/if}
                      <button
                        type="button"
                        class="op-more-item"
                        onclick={() => { openMoreSiteId = null; onOpenSettings(s, 'conf'); }}
                      >
                        <i class="ri-file-code-line icon-cyan" aria-hidden="true"></i>
                        <div>
                          <b>Konfigurasi Nginx (Vhost)</b>
                          <small>Sunting langsung virtual host dengan uji nginx -t</small>
                        </div>
                      </button>
                      <button
                        type="button"
                        class="op-more-item"
                        onclick={() => { openMoreSiteId = null; onOpenLogViewer(s); }}
                      >
                        <i class="ri-file-list-3-line icon-amber" aria-hidden="true"></i>
                        <div>
                          <b>Log Akses &amp; Error Nginx</b>
                          <small>Pantau lalu lintas HTTP &amp; catatan galat real-time</small>
                        </div>
                      </button>
                      <button
                        type="button"
                        class="op-more-item"
                        onclick={() => { openMoreSiteId = null; onTriggerBackup(s); }}
                      >
                        <i class="ri-archive-line icon-gray" aria-hidden="true"></i>
                        <div>
                          <b>Cadangkan Situs ({s.backup_count ?? 0})</b>
                          <small>Buat arsip .tar.gz proyek &amp; dump MySQL</small>
                        </div>
                      </button>
                      <div class="op-more-divider"></div>
                      <button
                        type="button"
                        class="op-more-item is-danger-item"
                        onclick={() => { openMoreSiteId = null; onOpenDeleteModal(s); }}
                      >
                        <i class="ri-delete-bin-line" aria-hidden="true"></i>
                        <div>
                          <b>Hapus Situs</b>
                          <small>Hapus vhost, service, database &amp; direktori</small>
                        </div>
                      </button>
                    </div>
                  {/if}
                </div>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}

  {#if !loading && filteredSites.length > 0}
    <!-- Footer Bar dengan Ringkasan Kesehatan Real-Time -->
    <div class="table-footer">
      <div class="footer-left">
        <span>Menampilkan <strong>{filteredSites.length}</strong> situs</span>
      </div>
      <div class="footer-right footer-status-summary">
        <span class="status-pill is-online">
          <i class="ri-checkbox-circle-fill" aria-hidden="true"></i>
          <span>{statusSummary.online} Online</span>
        </span>
        {#if statusSummary.down > 0}
          <span class="status-pill is-down">
            <i class="ri-error-warning-fill" aria-hidden="true"></i>
            <span>{statusSummary.down} Down</span>
          </span>
        {/if}
        {#if statusSummary.degraded > 0}
          <span class="status-pill is-degraded">
            <i class="ri-alert-fill" aria-hidden="true"></i>
            <span>{statusSummary.degraded} Degraded</span>
          </span>
        {/if}
        {#if statusSummary.stopped > 0}
          <span class="status-pill is-paused">
            <i class="ri-pause-circle-line" aria-hidden="true"></i>
            <span>{statusSummary.stopped} Stopped</span>
          </span>
        {/if}
      </div>
    </div>
  {/if}
</div>
