<script lang="ts">
  /**
   * Subkomponen Modal Web GUI Database (phpMyAdmin / Adminer).
   */
  import Modal from '../../lib/components/ui/Modal.svelte';
  import '../../styles/pages/databases/web-gui-modal.css';

  interface WebGuiStatus {
    installed?: boolean;
    software?: string;
    available_domains?: Array<{ domain: string; ssl: boolean }>;
  }

  interface Props {
    open: boolean;
    webGuiStatus: WebGuiStatus | null;
    targetWebGuiDb: string;
    targetWebGuiUser: string;
    targetWebGuiPass: string;
    showWebGuiPassword: boolean;
    webGuiAccessMode: 'panel' | 'domain' | 'custom_port';
    selectedWebGuiDomain: string;
    customWebGuiPort: number;
    publicAccessEnabled: boolean;
    getComputedPublicUrl: () => string;
    onCopyText: (text: string, label: string) => void;
    onOpenPmaInstall: () => void;
    onOpenWebGuiSso: () => void;
    onToggleWebGuiPublic: (checked: boolean) => void;
    onOpenWebGuiPublic: () => void;
    onClose: () => void;
  }

  let {
    open,
    webGuiStatus,
    targetWebGuiDb,
    targetWebGuiUser,
    targetWebGuiPass,
    showWebGuiPassword = $bindable(),
    webGuiAccessMode = $bindable(),
    selectedWebGuiDomain = $bindable(),
    customWebGuiPort = $bindable(),
    publicAccessEnabled,
    getComputedPublicUrl,
    onCopyText,
    onOpenPmaInstall,
    onOpenWebGuiSso,
    onToggleWebGuiPublic,
    onOpenWebGuiPublic,
    onClose
  }: Props = $props();
</script>

<Modal
  {open}
  size="md"
  onclose={onClose}
>
  {#snippet header()}
    <div class="modal-title-with-icon">
      <i class="ri-terminal-box-line engine-install-icon" aria-hidden="true"></i>
      <div>
        <h3 class="modal__title">Web GUI Database (phpMyAdmin / Adminer)</h3>
        <p class="modal__subtitle">Antarmuka visual manajemen database, tabel, dan eksekusi query SQL</p>
      </div>
    </div>
  {/snippet}

  <div class="pg-databases-web-gui-modal webgui-info-card">
    <div class="webgui-badge-row">
      {#if webGuiStatus?.installed}
        <span class="badge-status-online">● Layanan Aktif</span>
        <span class="badge-php-version">{webGuiStatus?.software || 'phpMyAdmin 5.2.1'}</span>
      {:else}
        <span class="badge-status-not-installed">● Belum Terpasang</span>
        <span class="badge-php-version">phpMyAdmin Resmi</span>
      {/if}
      {#if targetWebGuiDb}
        <span class="badge-target-db">Basis Data: <strong>{targetWebGuiDb}</strong></span>
      {/if}
    </div>

    {#if !webGuiStatus?.installed}
      <div class="webgui-not-installed-box mt-3">
        <div class="not-installed-content">
          <i class="ri-alert-line not-installed-icon" aria-hidden="true"></i>
          <div class="not-installed-info">
            <h4 class="not-installed-title">Layanan phpMyAdmin Belum Terpasang</h4>
            <p class="not-installed-desc">
              Paket resmi phpMyAdmin belum dipasang pada server ini. Silakan pasang paket phpMyAdmin terlebih dahulu melalui Quick Install agar dapat mengelola basis data secara visual.
            </p>
          </div>
        </div>
        <div class="not-installed-actions mt-3">
          <button
            type="button"
            class="btn btn-primary"
            onclick={onOpenPmaInstall}
          >
            <i class="ri-download-cloud-line" aria-hidden="true"></i>
            <span>Quick Install phpMyAdmin Sekarang</span>
          </button>
        </div>
      </div>
    {:else}
      {#if targetWebGuiUser || targetWebGuiPass}
        <div class="webgui-credentials-box">
          <div class="credentials-box-header">
            <i class="ri-key-2-line" aria-hidden="true"></i>
            <span class="credentials-box-title">Kredensial Login phpMyAdmin</span>
          </div>
          <div class="credentials-box-body">
            <div class="credential-row">
              <span class="credential-label">Username:</span>
              <code class="credential-value">{targetWebGuiUser || targetWebGuiDb}</code>
              <button
                type="button"
                class="btn-icon"
                onclick={() => onCopyText(targetWebGuiUser || targetWebGuiDb, 'Username')}
                title="Salin Username"
                aria-label="Salin Username"
              >
                <i class="ri-file-copy-line" aria-hidden="true"></i>
              </button>
            </div>
            {#if targetWebGuiPass}
              <div class="credential-row">
                <span class="credential-label">Password:</span>
                <code class="credential-value">
                  {showWebGuiPassword ? targetWebGuiPass : '••••••••••••'}
                </code>
                <button
                  type="button"
                  class="btn-icon"
                  onclick={() => (showWebGuiPassword = !showWebGuiPassword)}
                  title={showWebGuiPassword ? 'Sembunyikan Sandi' : 'Tampilkan Sandi'}
                  aria-label="Toggle Tampilan Sandi"
                >
                  <i class={showWebGuiPassword ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
                </button>
                <button
                  type="button"
                  class="btn-icon"
                  onclick={() => onCopyText(targetWebGuiPass, 'Password')}
                  title="Salin Password"
                  aria-label="Salin Password"
                >
                  <i class="ri-file-copy-line" aria-hidden="true"></i>
                </button>
              </div>
            {/if}
          </div>
          <p class="credential-hint">
            <i class="ri-information-line" aria-hidden="true"></i>
            <span>Formulir <strong>Log in</strong> phpMyAdmin otomatis terisi dengan Username di atas. Masukkan Password jika diminta.</span>
          </p>
        </div>
      {/if}

      <p class="webgui-desc">
        Pilih jalur akses yang sesuai dengan konfigurasi server Anda:
      </p>

      <!-- Pilihan Jalur Akses (Access Mode Selector) -->
      <div class="webgui-mode-selector mt-2">
        <button
          type="button"
          class="mode-tab-btn"
          class:active={webGuiAccessMode === 'panel'}
          onclick={() => (webGuiAccessMode = 'panel')}
        >
          <i class="ri-dashboard-line" aria-hidden="true"></i>
          <span>Melalui FikPanel (Independen)</span>
        </button>
        <button
          type="button"
          class="mode-tab-btn"
          class:active={webGuiAccessMode === 'domain'}
          onclick={() => (webGuiAccessMode = 'domain')}
        >
          <i class="ri-global-line" aria-hidden="true"></i>
          <span>Melalui Domain Situs</span>
        </button>
        <button
          type="button"
          class="mode-tab-btn"
          class:active={webGuiAccessMode === 'custom_port'}
          onclick={() => (webGuiAccessMode = 'custom_port')}
        >
          <i class="ri-terminal-box-line" aria-hidden="true"></i>
          <span>Port Standalone Khusus</span>
        </button>
      </div>

      <!-- Konfigurasi sesuai Mode Terpilih -->
      {#if webGuiAccessMode === 'domain'}
        <div class="mode-config-box mt-3">
          {#if webGuiStatus?.available_domains && webGuiStatus.available_domains.length > 0}
            <label for="webgui-domain-select" class="form-label">Pilih Domain Situs yang Terpasang:</label>
            <select id="webgui-domain-select" class="form-input" bind:value={selectedWebGuiDomain}>
              {#each webGuiStatus.available_domains as d}
                <option value={d.domain}>{d.domain} ({d.ssl ? 'HTTPS:443' : 'HTTP:80'})</option>
              {/each}
            </select>
            <p class="mode-help-text">
              Mengakses phpMyAdmin via subpath domain <code>/{selectedWebGuiDomain}/phpmyadmin</code> pada port standar web (80/443).
            </p>
          {:else}
            <div class="mode-alert-box">
              <i class="ri-alert-line" aria-hidden="true"></i>
              <span>Belum ada situs terdaftar di menu Sites. Mengalihkan ke mode FikPanel Independen yang langsung aktif.</span>
            </div>
          {/if}
        </div>
      {:else if webGuiAccessMode === 'custom_port'}
        <div class="mode-config-box mt-3">
          <label for="webgui-port-input" class="form-label">Nomor Port Khusus phpMyAdmin:</label>
          <div class="input-with-unit">
            <input
              id="webgui-port-input"
              type="number"
              class="form-input"
              bind:value={customWebGuiPort}
              placeholder="888"
              min="1"
              max="65535"
            />
            <span class="unit-badge">TCP</span>
          </div>
          <p class="mode-help-text">
            Gunakan mode ini jika Anda menjalankan daemon phpMyAdmin pada port standalone (mis. 888 atau 8080). Pastikan port telah dibuka di firewall VPS.
          </p>
        </div>
      {:else}
        <div class="mode-config-box mt-3">
          <div class="mode-panel-info">
            <i class="ri-shield-check-line mode-panel-icon" aria-hidden="true"></i>
            <div>
              <span class="mode-panel-title">Akses Mandiri FikPanel (Rekomendasi Utama)</span>
              <p class="mode-help-text">
                Berjalan langsung di port panel (<code>{typeof window !== 'undefined' ? window.location.origin : ''}/phpmyadmin/index.php</code>). Tidak terikat ke domain manapun, bebas dari port 888, dan tidak perlu membuka port baru di firewall VPS atau cloud security group.
              </p>
            </div>
          </div>
        </div>
      {/if}

      <div class="webgui-cards-container mt-3">
        <!-- Opsi 1: Akses Bebas Sandi (Password-Free Access / SSO) -->
        <div class="webgui-choice-card highlight">
          <div class="choice-card-header">
            <span class="badge-new-sso">FITUR UTAMA</span>
            <h4 class="choice-card-title">Akses Bebas Sandi (One-Click SSO)</h4>
          </div>
          <p class="choice-card-desc">
            Masuk langsung ke antarmuka web database tanpa perlu mengetik ulang root password secara manual.
            Menggunakan token sesi panel yang aman dan otomatis kedaluwarsa.
          </p>
          <div class="webgui-public-url-box mt-2">
            <div class="public-url-wrapper">
              <span class="public-url-label">Target URL:</span>
              <code class="public-url-text">{getComputedPublicUrl()}</code>
            </div>
          </div>
          <button
            class="btn btn-primary mt-2"
            onclick={onOpenWebGuiSso}
          >
            <i class="ri-external-link-line" aria-hidden="true"></i>
            <span>Buka Akses Bebas Sandi (Tab Baru)</span>
          </button>
        </div>

        <!-- Opsi 2: Akses Publik (Public Access) -->
        <div class="webgui-choice-card">
          <div class="choice-card-header">
            <h4 class="choice-card-title">Akses Publik (Public Access)</h4>
            <label class="toggle-switch">
              <input
                type="checkbox"
                checked={publicAccessEnabled}
                onchange={(e) => onToggleWebGuiPublic((e.target as HTMLInputElement).checked)}
              />
              <span class="slider"></span>
            </label>
          </div>
          <p class="choice-card-desc">
            Membuka port / URL antarmuka web database ke internet publik untuk diakses via URL domain/IP.
            Matikan akses publik bila tidak digunakan untuk meningkatkan keamanan server dari serangan brute-force.
          </p>
          {#if publicAccessEnabled}
            <div class="webgui-public-url-box mt-2">
              <div class="public-url-wrapper">
                <span class="public-url-label">URL Publik:</span>
                <code class="public-url-text">{getComputedPublicUrl()}</code>
              </div>
              <button
                type="button"
                class="btn-icon"
                onclick={() => onCopyText(getComputedPublicUrl(), 'URL Web GUI')}
                title="Salin URL Publik"
                aria-label="Salin URL Publik"
              >
                <i class="ri-file-copy-line" aria-hidden="true"></i>
              </button>
            </div>
            <button
              class="btn btn-outline mt-2"
              onclick={onOpenWebGuiPublic}
            >
              <i class="ri-global-line" aria-hidden="true"></i>
              <span>Buka URL Publik (Tab Baru)</span>
            </button>
          {/if}
        </div>
      </div>

      <!-- Penjelasan Komprehensif Port 888 vs Domain vs Panel -->
      <div class="webgui-environment-hint mt-3">
        <i class="ri-information-line hint-icon" aria-hidden="true"></i>
        <div class="hint-content">
          <span class="hint-title">Mengapa phpMyAdmin Tidak Wajib Port 888?</span>
          <p class="hint-text">
            Di industri hosting (seperti cPanel, DirectAdmin, atau CloudPanel), phpMyAdmin umumnya diakses melalui domain situs (mis. <code>kostash.my.id/phpmyadmin</code>) pada port standar 80/443 dengan SSL HTTPS, atau disajikan langsung melalui port panel itu sendiri. Port 888 adalah default warisan aaPanel yang sering terblokir firewall cloud. FikPanel memberikan fleksibilitas penuh: Anda bisa mengakses via domain situs, port khusus, atau langsung melalui FikPanel Built-in Client tanpa hambatan port!
          </p>
        </div>
      </div>
    {/if}
  </div>

  {#snippet footer()}
    <button class="btn btn-outline" onclick={onClose}>
      Tutup
    </button>
  {/snippet}
</Modal>

