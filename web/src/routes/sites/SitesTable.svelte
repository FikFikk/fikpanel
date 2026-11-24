<script lang="ts">
  import '../../styles/pages/sites/sites-table.css';
  import type { Site } from '../../lib/types';

  // Antarmuka properti untuk tabel daftar situs web
  interface Props {
    loading: boolean;
    filteredSites: Site[];
    selectedIds: Set<number>;
    backingUpSiteId: number | null;
    openMoreSiteId: number | null;
    onToggleSelectAll: (e: Event) => void;
    onToggleSelectSite: (id: number) => void;
    onToggleSite: (id: number) => void;
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
    openMoreSiteId = $bindable(),
    onToggleSelectAll,
    onToggleSelectSite,
    onToggleSite,
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

  // Hitung nilai maksimum permintaan di tabel untuk lebar bilah proporsional nyata
  let maxRequests = $derived(Math.max(1, ...filteredSites.map((item) => item.request_count ?? 0)));

  // Ringkasan kesehatan live seluruh situs yang tampil pada footer tabel
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

  // Tentukan skema URL yang tepat berdasarkan status SSL aktif situs
  function siteHref(s: Site): string {
    const hasSSL = s.ssl_mode === 'cloudflare' || (s.ssl_status === 'issued' && s.ssl_mode !== 'none');
    return `${hasSSL ? 'https' : 'http'}://${s.domain}`;
  }

  // Petakan status live probe ke kelas warna semantik, ikon, dan label
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

  // Label sumber data jumlah permintaan yang jujur untuk tooltip
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

  // Persentase lebar bar proporsional terhadap situs dengan trafik tertinggi di daftar
  function requestBarPct(count?: number): number {
    const n = count ?? 0;
    if (n <= 0) return 0;
    return Math.max(8, Math.min(100, Math.round((n / maxRequests) * 100)));
  }
</script>

<div class="table-container pg-sites-table">
  {#if loading}
    <div class="state state--loading">
      <i class="ri-loader-4-line spin state__icon"></i>
      <span class="state__title">Memuat daftar situs web...</span>
    </div>
  {:else if filteredSites.length === 0}
    <div class="state state--empty">
      <i class="ri-global-line state__icon"></i>
      <p class="state__desc">Tidak ada situs yang cocok dengan kriteria pencarian.</p>
      <button class="btn btn--primary" onclick={onShowAddModal}>
        + Tambah Situs Baru
      </button>
    </div>
  {:else}
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
          <th class="col-name">Nama Situs</th>
          <th class="col-status">Status</th>
          <th class="col-backup">Cadangan</th>
          <th class="col-quick">Aksi Cepat</th>
          <th class="col-ssl">SSL</th>
          <th class="col-req">Permintaan</th>
          <th class="col-ops">Operasi</th>
        </tr>
      </thead>
      <tbody>
        {#each filteredSites as s (s.id)}
          {@const stMeta = liveStatusMeta(s)}
          {@const isBackingUp = backingUpSiteId === s.id}
          <tr class="site-row {selectedIds.has(s.id) ? 'selected-row' : ''}">
            <td class="col-chk">
              <input
                type="checkbox"
                aria-label="Pilih situs {s.domain}"
                checked={selectedIds.has(s.id)}
                onchange={() => onToggleSelectSite(s.id)}
              />
            </td>

            <!-- Kolom Nama Situs & Root Direktori -->
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
                  <i class="ri-external-link-line icon-ext"></i>
                </a>
                <div class="site-path-sub mono">
                  <i class="ri-folder-2-line"></i>
                  <span>{s.kind === 'proxy' ? s.upstream : (s.root_path || '/var/www/html')}</span>
                </div>
                {#if s.has_git || s.has_cicd}
                  <div class="site-git-meta mono">
                    {#if s.has_git}
                      <button
                        type="button"
                        class="git-meta-pill"
                        onclick={() => onOpenGitPullModal(s)}
                        title="Proyek terhubung ke Git ({s.git_repo || 'origin'} • branch {s.git_branch || 'main'}). Klik untuk Git Pull & Auto-Deploy."
                      >
                        <i class="ri-git-branch-line"></i>
                        <span>{s.git_repo || 'git'}</span>
                        <span class="git-branch-tag">{s.git_branch || 'main'}</span>
                        {#if s.git_commit}
                          <span class="git-commit-hash">#{s.git_commit}</span>
                        {/if}
                      </button>
                    {/if}
                    {#if s.has_cicd}
                      <span
                        class="cicd-pill {s.cicd_provider || 'github-actions'}"
                        title={s.cicd_provider === 'actions-runner'
                          ? 'Dideploy / dijalankan melalui GitHub Actions Self-Hosted Runner di server'
                          : (s.cicd_provider === 'github-actions'
                            ? 'Pipeline CI/CD GitHub Actions terdeteksi (.github/workflows)'
                            : (s.cicd_provider === 'gitlab-ci'
                              ? 'Pipeline GitLab CI terdeteksi (.gitlab-ci.yml)'
                              : `Pipeline CI/CD: ${s.cicd_provider}`))}
                      >
                        {#if s.cicd_provider === 'actions-runner'}
                          <i class="ri-cpu-line"></i>
                          <span>Action Runner</span>
                        {:else if s.cicd_provider === 'github-actions'}
                          <i class="ri-github-fill"></i>
                          <span>GitHub Actions</span>
                        {:else if s.cicd_provider === 'gitlab-ci'}
                          <i class="ri-gitlab-line"></i>
                          <span>GitLab CI</span>
                        {:else}
                          <i class="ri-rocket-2-line"></i>
                          <span>CI/CD</span>
                        {/if}
                      </span>
                    {/if}
                  </div>
                {/if}
                {#if s.description}
                  <div class="site-path-sub" title={s.description}>
                    <i class="ri-sticky-note-line"></i>
                    <span>{s.description}</span>
                  </div>
                {/if}
              </div>
            </td>

            <!-- Kolom Status Operasional Live (Probe TCP/PHP-FPM/Nginx) -->
            <td class="col-status">
              <button
                type="button"
                class="btn-status-play {stMeta.cls}"
                onclick={() => onToggleSite(s.id)}
                title="{s.live_detail || (s.enabled === 1 ? 'Situs aktif' : 'Situs dihentikan')} — Klik untuk {s.enabled === 1 ? 'menghentikan' : 'menjalankan'} vhost"
              >
                <i class={stMeta.icon}></i>
                <span>{stMeta.label}</span>
              </button>
            </td>

            <!-- Kolom Cadangan / Backup Nyata 1-Klik -->
            <td class="col-backup">
              <button
                type="button"
                class="btn-backup-info"
                disabled={backingUpSiteId !== null}
                onclick={() => onTriggerBackup(s)}
                title={isBackingUp
                  ? `Sedang membuat arsip cadangan ${s.domain}...`
                  : `Klik untuk membuat arsip cadangan .tar.gz langsung ke server (${s.backup_count ?? 0} arsip tersimpan)`}
              >
                <i class={isBackingUp ? 'ri-loader-4-line spin' : 'ri-archive-line'}></i>
                <div class="backup-text-wrap">
                  <span class="backup-main">{s.backup_count ?? 0} Backup</span>
                  <span class="backup-sub">{isBackingUp ? 'Memproses...' : 'Cadangkan'}</span>
                </div>
              </button>
            </td>

            <!-- Kolom Aksi Cepat (Buka File Manager, Terminal, Config, Runtime) -->
            <td class="col-quick">
              <div class="quick-actions-bar">
                <button
                  type="button"
                  class="quick-act-btn"
                  onclick={() => onOpenInFileManager(getProjectRootDir(s))}
                  aria-label="Buka direktori proyek situs di File Manager FikPanel"
                  title="Buka direktori proyek situs di File Manager FikPanel"
                >
                  <i class="ri-folder-open-fill icon-amber"></i>
                </button>

                <button
                  type="button"
                  class="quick-act-btn"
                  onclick={() => onOpenInTerminal(getProjectRootDir(s))}
                  aria-label="Buka Terminal di folder root proyek situs ini"
                  title="Buka Terminal di folder root proyek situs ini"
                >
                  <i class="ri-terminal-box-line icon-cyan"></i>
                </button>

                <button
                  type="button"
                  class="quick-act-btn"
                  onclick={() => onOpenSettings(s, 'domain')}
                  aria-label="Pengaturan situs (domain, direktori, SSL, konfigurasi)"
                  title="Pengaturan situs (domain, direktori, SSL, konfigurasi)"
                >
                  <i class="ri-settings-3-line icon-gray"></i>
                </button>

                <button
                  type="button"
                  class="badge-runtime {s.kind}"
                  onclick={() => onOpenSettings(s, s.kind === 'php' ? 'php' : s.kind === 'static' ? 'directory' : 'proxy')}
                  title="Klik untuk mengatur runtime & konfigurasi proyek ini"
                >
                  {#if s.kind === 'php'}
                    {s.framework ? `PHP (${s.framework})` : (s.php_version ? `PHP ${s.php_version}` : 'PHP')}
                  {:else if s.kind === 'node'}
                    {s.framework ? `Node.js (${s.framework})` : 'Node.js'}
                  {:else if s.kind === 'go'}
                    {s.framework ? s.framework : 'Go Service'}
                  {:else if s.kind === 'python'}
                    {s.framework ? `Python (${s.framework})` : 'Python'}
                  {:else if s.kind === 'static'}
                    Static
                  {:else}
                    {s.framework ? s.framework : 'Proxy'}
                  {/if}
                </button>
              </div>
            </td>

            <!-- Kolom Sertifikat SSL -->
            <td class="col-ssl">
              {#if s.ssl_mode === 'cloudflare'}
                <button
                  type="button"
                  class="ssl-badge ssl-cloudflare"
                  onclick={() => onOpenSettings(s, 'ssl')}
                  title="Cloudflare Edge SSL Aktif"
                >
                  <i class="ri-cloud-line"></i>
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
                  <i class="ri-shield-check-fill"></i>
                  <span>{sslDaysLeft(s)} Hari</span>
                </button>
              {:else if s.ssl_status === 'pending'}
                <button type="button" class="ssl-badge ssl-pending" onclick={() => onOpenSettings(s, 'ssl')} title="Sertifikat sedang diterbitkan">
                  <i class="ri-loader-4-line spin"></i>
                  <span>Menerbitkan</span>
                </button>
              {:else if s.ssl_status === 'failed' && s.ssl_mode !== 'none'}
                <button type="button" class="ssl-badge ssl-failed" onclick={() => onOpenSettings(s, 'ssl')} title="Penerbitan gagal, klik untuk detail">
                  <i class="ri-shield-cross-line"></i>
                  <span>Gagal</span>
                </button>
              {:else}
                <button
                  type="button"
                  class="ssl-badge ssl-inactive"
                  onclick={() => onOpenSettings(s, 'ssl')}
                  title="Klik untuk memasang sertifikat Let's Encrypt"
                >
                  <i class="ri-shield-line"></i>
                  <span>Pasang SSL</span>
                </button>
              {/if}
            </td>

            <!-- Kolom Permintaan / Request Count Nyata -->
            <td class="col-req mono">
              {#if (s.request_count ?? 0) === 0 && s.request_source === 'tunnel'}
                <button
                  type="button"
                  class="req-tunnel-tag"
                  onclick={() => onOpenLogViewer(s)}
                  title="Trafik masuk via Cloudflare Tunnel langsung ke {s.upstream || 'upstream lokal'} tanpa melewati access log Nginx (Klik untuk lihat log)"
                >
                  <i class="ri-route-line"></i>
                  <span>Tunnel</span>
                </button>
              {:else}
                <button
                  type="button"
                  class="req-stat-box req-btn-trigger"
                  onclick={() => onOpenLogViewer(s)}
                  title="Total {(s.request_count ?? 0).toLocaleString('id-ID')} permintaan • Sumber: {requestSourceHint(s.request_source)} (Klik untuk lihat Access Log)"
                >
                  <span class="req-num">{(s.request_count ?? 0).toLocaleString('id-ID')}</span>
                  <span class="req-meter" aria-hidden="true">
                    <span class="req-meter__fill" style="width: {requestBarPct(s.request_count)}%"></span>
                  </span>
                </button>
              {/if}
            </td>

            <!-- Kolom Operasi -->
            <td class="col-ops">
              <div class="ops-wrap">
                {#if s.has_git}
                  <button
                    type="button"
                    class="btn-op-git"
                    onclick={() => onOpenGitPullModal(s)}
                    title="Tarik pembaruan kode dari Git ({s.git_repo || 'origin'} : {s.git_branch || 'main'})"
                  >
                    <i class="ri-git-pull-request-line"></i>
                    <span>Git Pull</span>
                  </button>
                {:else}
                  <button
                    type="button"
                    class="btn-op-conf-pill"
                    onclick={() => onOpenSettings(s, 'conf')}
                    title="Konfigurasi Virtual Host & Situs"
                  >
                    <i class="ri-settings-4-line"></i>
                    <span>Conf</span>
                  </button>
                {/if}

                <div class="op-more-wrap">
                  <button
                    type="button"
                    class="btn-op-more {openMoreSiteId === s.id ? 'is-open' : ''}"
                    onclick={(e) => onToggleMoreMenu(s.id, e)}
                    title="Menu operasi situs lengkap"
                  >
                    <span>More</span>
                    <i class="ri-arrow-down-s-line"></i>
                  </button>
                  {#if openMoreSiteId === s.id}
                    <div class="op-more-menu" role="menu">
                      <button
                        type="button"
                        class="op-more-item"
                        onclick={() => { openMoreSiteId = null; onOpenSettings(s, 'conf'); }}
                      >
                        <i class="ri-file-code-line icon-cyan"></i>
                        <div>
                          <b>Konfigurasi Situs (Conf)</b>
                          <small>Pengaturan vhost Nginx, PHP, SSL &amp; domain</small>
                        </div>
                      </button>
                      <button
                        type="button"
                        class="op-more-item"
                        onclick={() => { openMoreSiteId = null; onOpenLogViewer(s); }}
                      >
                        <i class="ri-file-list-3-line icon-amber"></i>
                        <div>
                          <b>Log Akses &amp; Error (Log)</b>
                          <small>Pantau lalu lintas HTTP &amp; catatan galat</small>
                        </div>
                      </button>
                      <button
                        type="button"
                        class="op-more-item"
                        onclick={() => { openMoreSiteId = null; onOpenInTerminal(getProjectRootDir(s)); }}
                      >
                        <i class="ri-terminal-box-line icon-gray"></i>
                        <div>
                          <b>Buka Terminal Proyek</b>
                          <small class="mono">{getProjectRootDir(s)}</small>
                        </div>
                      </button>
                      <div class="op-more-divider"></div>
                      <button
                        type="button"
                        class="op-more-item is-danger-item"
                        onclick={() => { openMoreSiteId = null; onOpenDeleteModal(s); }}
                      >
                        <i class="ri-delete-bin-line"></i>
                        <div>
                          <b>Hapus Situs</b>
                          <small>Hapus vhost, database &amp; direktori proyek</small>
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

    <!-- Footer Bar Tabel Situs dengan Ringkasan Kesehatan Real-Time -->
    <div class="table-footer">
      <div class="footer-left">
        <span>Total <strong>{filteredSites.length}</strong> situs ditampilkan</span>
      </div>
      <div class="footer-right footer-status-summary">
        <span class="status-pill is-online">
          <i class="ri-checkbox-circle-fill"></i>
          <span>{statusSummary.online} Online</span>
        </span>
        {#if statusSummary.down > 0}
          <span class="status-pill is-down">
            <i class="ri-error-warning-fill"></i>
            <span>{statusSummary.down} Down</span>
          </span>
        {/if}
        {#if statusSummary.degraded > 0}
          <span class="status-pill is-degraded">
            <i class="ri-alert-fill"></i>
            <span>{statusSummary.degraded} Degraded</span>
          </span>
        {/if}
        {#if statusSummary.stopped > 0}
          <span class="status-pill is-paused">
            <i class="ri-pause-circle-line"></i>
            <span>{statusSummary.stopped} Stopped</span>
          </span>
        {/if}
      </div>
    </div>
  {/if}
</div>
