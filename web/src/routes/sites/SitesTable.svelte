<script lang="ts">
  import '../../styles/pages/sites/sites-table.css';
  import type { Site } from '../../lib/types';

  // Antarmuka properti untuk tabel daftar situs web
  interface Props {
    loading: boolean;
    filteredSites: Site[];
    selectedIds: Set<number>;
    openMoreSiteId: number | null;
    onToggleSelectAll: () => void;
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
              checked={selectedIds.size === filteredSites.length && filteredSites.length > 0}
              onchange={onToggleSelectAll}
            />
          </th>
          <th class="col-name">Nama Situs</th>
          <th class="col-status">Status</th>
          <th class="col-backup">Cadangan</th>
          <th class="col-quick">Aksi Cepat</th>
          <th class="col-expire">Masa Berlaku</th>
          <th class="col-ssl">SSL</th>
          <th class="col-req">Permintaan</th>
          <th class="col-ops">Operasi</th>
        </tr>
      </thead>
      <tbody>
        {#each filteredSites as s (s.id)}
          <tr class="site-row {selectedIds.has(s.id) ? 'selected-row' : ''}">
            <td class="col-chk">
              <input
                type="checkbox"
                checked={selectedIds.has(s.id)}
                onchange={() => onToggleSelectSite(s.id)}
              />
            </td>

            <!-- Kolom Nama Situs & Root Direktori -->
            <td class="col-name">
              <div class="site-domain-box">
                <a
                  href="http://{s.domain}"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="domain-link"
                  title="Buka situs di tab baru"
                >
                  <span class="domain-text">{s.domain}</span>
                  <i class="ri-external-link-line icon-ext"></i>
                </a>
                <div class="site-path-sub mono">
                  <i class="ri-folder-2-line"></i>
                  <span>{s.kind === 'proxy' ? s.upstream : (s.root_path || '/var/www/html')}</span>
                </div>
                {#if s.has_git}
                  <div class="site-git-meta mono">
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

            <!-- Kolom Status Operasional (Toggle Play/Pause) -->
            <td class="col-status">
              <button
                class="btn-status-play {s.enabled === 1 ? 'is-active' : 'is-paused'}"
                onclick={() => onToggleSite(s.id)}
                title={s.enabled === 1 ? 'Situs sedang berjalan (Klik untuk menghentikan)' : 'Situs dihentikan (Klik untuk menjalankan)'}
              >
                <i class="{s.enabled === 1 ? 'ri-play-circle-fill' : 'ri-pause-circle-line'}"></i>
                <span>{s.enabled === 1 ? 'Running' : 'Stopped'}</span>
              </button>
            </td>

            <!-- Kolom Cadangan / Backup -->
            <td class="col-backup">
              <button
                class="btn-backup-info"
                onclick={() => onTriggerBackup(s)}
                title="Klik untuk membuat cadangan situs"
              >
                <i class="ri-cloud-line"></i>
                <div class="backup-text-wrap">
                  <span class="backup-main">{s.backup_count ?? 0} Backup</span>
                  <span class="backup-sub">Cadangkan</span>
                </div>
              </button>
            </td>

            <!-- Kolom Aksi Cepat (Buka File Manager, Terminal, Config, Runtime) -->
            <td class="col-quick">
              <div class="quick-actions-bar">
                <button
                  class="quick-act-btn"
                  onclick={() => onOpenInFileManager(getProjectRootDir(s))}
                  title="Buka direktori proyek situs di File Manager FikPanel"
                >
                  <i class="ri-folder-open-fill icon-amber"></i>
                </button>

                <button
                  class="quick-act-btn"
                  onclick={() => onOpenInTerminal(getProjectRootDir(s))}
                  title="Buka Terminal di folder root proyek situs ini"
                >
                  <i class="ri-terminal-box-line icon-cyan"></i>
                </button>

                <button
                  class="quick-act-btn"
                  onclick={() => onOpenSettings(s, 'domain')}
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
                    {s.php_version ? `PHP ${s.php_version}` : 'PHP'}
                  {:else if s.kind === 'node'}
                    Node.js
                  {:else if s.kind === 'go'}
                    Golang
                  {:else if s.kind === 'python'}
                    Python
                  {:else if s.kind === 'static'}
                    Static
                  {:else}
                    Proxy
                  {/if}
                </button>
              </div>
            </td>

            <!-- Kolom Masa Berlaku -->
            <td class="col-expire">
              <span class="perpetual-badge">Permanen</span>
            </td>

            <!-- Kolom Sertifikat SSL -->
            <td class="col-ssl">
              {#if s.ssl_status === 'issued' && s.ssl_mode !== 'none'}
                <button
                  class="ssl-badge ssl-active"
                  class:ssl-expiring={sslDaysLeft(s) < 15}
                  onclick={() => onOpenSettings(s, 'ssl')}
                  title="Berlaku sampai {new Date(s.ssl_expires * 1000).toLocaleDateString('id-ID')}"
                >
                  <i class="ri-shield-check-fill"></i>
                  <span>{sslDaysLeft(s)} Hari</span>
                </button>
              {:else if s.ssl_status === 'pending'}
                <button class="ssl-badge ssl-pending" onclick={() => onOpenSettings(s, 'ssl')} title="Sertifikat sedang diterbitkan">
                  <i class="ri-loader-4-line spin"></i>
                  <span>Menerbitkan</span>
                </button>
              {:else if s.ssl_status === 'failed' && s.ssl_mode !== 'none'}
                <button class="ssl-badge ssl-failed" onclick={() => onOpenSettings(s, 'ssl')} title="Penerbitan gagal, klik untuk detail">
                  <i class="ri-shield-cross-line"></i>
                  <span>Gagal</span>
                </button>
              {:else}
                <button
                  class="ssl-badge ssl-inactive"
                  onclick={() => onOpenSettings(s, 'ssl')}
                  title="Klik untuk memasang sertifikat Let's Encrypt"
                >
                  <i class="ri-shield-line"></i>
                  <span>Pasang SSL</span>
                </button>
              {/if}
            </td>

            <!-- Kolom Permintaan / Request Count -->
            <td class="col-req mono">
              <button
                type="button"
                class="req-stat-box req-btn-trigger"
                onclick={() => onOpenLogViewer(s)}
                title="Total {(s.request_count ?? 0).toLocaleString('id-ID')} permintaan HTTP tercatat di log Nginx (Klik untuk lihat Access Log)"
              >
                <span class="req-num">{(s.request_count ?? 0).toLocaleString('id-ID')}</span>
                <span class="req-sparkline"></span>
              </button>
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

    <!-- Footer Bar Tabel Situs -->
    <div class="table-footer">
      <div class="footer-left">
        <span>Total <strong>{filteredSites.length}</strong> situs terdaftar</span>
      </div>
      <div class="footer-right">
        <span class="badge">Halaman 1 dari 1</span>
      </div>
    </div>
  {/if}
</div>
