<script lang="ts">
  import type {
    ProtectionOverview,
    FirewallExtendedStatus,
    SSHStats,
    HardeningStatus,
    Fail2BanStatus,
    SecurityScanReport,
  } from '../../lib/types';
  import type { SecurityTabId } from './SecurityTabs.svelte';
  import '../../styles/pages/security/overview.css';

  interface Props {
    isScanning: boolean;
    scanReport: SecurityScanReport | null;
    protection: ProtectionOverview | null;
    fwStatus: FirewallExtendedStatus | null;
    sshStats: SSHStats | null;
    hardening: HardeningStatus | null;
    f2bStatus: Fail2BanStatus | null;
    onRefresh: () => void;
    onRunScan: () => void;
    onToggleFirewall: () => void;
    onRestartSSH: () => void;
    onToggleTamperProof: () => void;
    onSelectTab: (tab: SecurityTabId) => void;
    onOpenScanCategory: (category: string) => void;
  }

  let {
    isScanning,
    scanReport,
    protection,
    fwStatus,
    sshStats,
    hardening,
    f2bStatus,
    onRefresh,
    onRunScan,
    onToggleFirewall,
    onRestartSSH,
    onToggleTamperProof,
    onSelectTab,
    onOpenScanCategory,
  }: Props = $props();
</script>

<div class="pg-security-overview tab-pane">
  <header class="pane-header">
    <div>
      <h2 class="pane-title">Security Overview</h2>
      <p class="pane-subtitle">Monitor your server security posture and respond to threats before they impact your services.</p>
    </div>
    <div class="header-right">
      <span class="badge badge--success badge--pill">
        <span class="dot dot--success dot--pulse"></span> System running
      </span>
      <button class="btn btn--secondary btn--sm" onclick={onRefresh} aria-label="Refresh Data Keamanan">
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
          onclick={onRunScan}
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
                  onchange={onToggleFirewall}
                />
                <span class="switch__track"></span>
              </label>
              <button class="btn btn--ghost btn--icon" title="Pengaturan Firewall" onclick={() => onSelectTab('firewall')}>
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
                  onchange={onRestartSSH}
                />
                <span class="switch__track"></span>
              </label>
              <button class="btn btn--ghost btn--icon" title="Pengaturan SSH" onclick={() => onSelectTab('ssh')}>
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
              <button class="btn btn--ghost btn--icon" title="Pengaturan WAF" onclick={() => onSelectTab('anti_intrusion')}>
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
                  onchange={onToggleTamperProof}
                />
                <span class="switch__track"></span>
              </label>
              <button class="btn btn--ghost btn--icon" title="Pengaturan Hardening" onclick={() => onSelectTab('anti_intrusion')}>
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
              <button class="btn btn--ghost btn--icon" title="Pengaturan Intrusion" onclick={() => onSelectTab('anti_intrusion')}>
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
              <button class="btn btn--ghost btn--icon" title="Detail Fail2Ban" onclick={() => onSelectTab('anti_intrusion')}>
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
          <button class="link-btn" onclick={() => onOpenScanCategory('risk')}>
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
          <button class="link-btn" onclick={() => onOpenScanCategory('malware')}>
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
          <button class="link-btn" onclick={() => onOpenScanCategory('vuln')}>
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
          <button class="link-btn" onclick={() => onOpenScanCategory('baseline')}>
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
              <button class="btn btn--secondary btn--sm" onclick={() => onSelectTab('server_security')}>
                Perbaiki Sekarang
              </button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </section>
</div>

