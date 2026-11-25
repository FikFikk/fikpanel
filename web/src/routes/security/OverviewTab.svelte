<script lang="ts">
  import type { ProtectionOverview, ProtectionItem, ScanFinding, SecurityScanReport } from '../../lib/types';
  import type { SecurityTabId } from './SecurityTabs.svelte';
  import '../../styles/pages/security/overview.css';

  interface Props {
    protection: ProtectionOverview | null;
    scanReport: SecurityScanReport | null;
    isScanning: boolean;
    busyKey: string;
    onRunScan: () => void;
    onTogglePillar: (pillar: ProtectionItem) => void;
    onNavigateTarget: (target: string) => void;
    onOpenScanCategory: (category: string) => void;
    onApplyQuickFix: (finding: ScanFinding) => void;
  }

  let {
    protection,
    scanReport,
    isScanning,
    busyKey,
    onRunScan,
    onTogglePillar,
    onNavigateTarget,
    onOpenScanCategory,
    onApplyQuickFix,
  }: Props = $props();

  // Perhitungan keliling lingkaran radial gauge (r = 56)
  const RADIUS = 56;
  const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

  let scoreValue = $derived(scanReport ? Math.max(0, Math.min(100, scanReport.score)) : 0);
  let dashOffset = $derived(
    scanReport ? CIRCUMFERENCE * (1 - scoreValue / 100) : CIRCUMFERENCE * 0.75
  );
  let scoreTone = $derived<'safe' | 'warn' | 'danger'>(
    !scanReport ? 'warn' : scoreValue >= 80 ? 'safe' : scoreValue >= 55 ? 'warn' : 'danger'
  );

  // Urutkan temuan aktif: critical terlebih dahulu, lalu warning
  let actionableFindings = $derived.by(() => {
    if (!scanReport?.findings) return [];
    return scanReport.findings
      .filter((f) => f.severity !== 'safe')
      .sort((a, b) => (a.severity === 'critical' ? -1 : 1) - (b.severity === 'critical' ? -1 : 1));
  });

  function formatUnixTime(ts: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString('id-ID', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  function pillarDotClass(p: ProtectionItem): string {
    if (!p.installed) return 'dot';
    return p.enabled ? 'dot dot--success' : 'dot dot--warning';
  }

  function pillarBadgeClass(p: ProtectionItem): string {
    if (!p.installed) return 'badge badge--neutral';
    return p.enabled ? 'badge badge--success' : 'badge badge--warning';
  }
</script>

<div class="pg-security-overview">
  {#if protection && !protection.is_linux}
    <div class="alert alert--info" role="status">
      <i class="ri-information-line" aria-hidden="true"></i>
      <div>
        <strong>Mode Pengembangan Non-Linux:</strong> Kontrol kernel <code>sysctl</code>, <code>ufw</code>, <code>sshd</code>, dan <code>fail2ban</code> aktif penuh saat FikPanel berjalan di host server Linux. Data persistensi SQLite (Brute Force, Whitelist/Blacklist, Audit &amp; FIM) tetap aktif.
      </div>
    </div>
  {/if}

  <!-- Grid Hero: Radial Score Audit & 6 Pilar Proteksi Real -->
  <div class="ov-hero-grid">
    <div class="card ov-score-card">
      <div class="ov-gauge" class:ov-gauge--scanning={isScanning}>
        <svg class="ov-gauge__svg" viewBox="0 0 136 136" aria-hidden="true">
          <circle class="ov-gauge__track" cx="68" cy="68" r={RADIUS} />
          <circle
            class="ov-gauge__fill ov-gauge__fill--{scoreTone}"
            cx="68"
            cy="68"
            r={RADIUS}
            stroke-dasharray={CIRCUMFERENCE}
            stroke-dashoffset={dashOffset}
          />
        </svg>
        <div class="ov-gauge__center">
          {#if scanReport}
            <span class="ov-gauge__value">{scanReport.score}</span>
            <span class="ov-gauge__label">dari 100</span>
          {:else}
            <span class="ov-gauge__value">--</span>
            <span class="ov-gauge__label">Belum Audit</span>
          {/if}
        </div>
      </div>

      <div class="ov-score-meta">
        {#if scanReport}
          <span
            class="badge badge--pill"
            class:badge--success={scoreTone === 'safe'}
            class:badge--warning={scoreTone === 'warn'}
            class:badge--danger={scoreTone === 'danger'}
          >
            {scanReport.rating}
          </span>
          <h2 class="ov-score-title">Postur Keamanan Server</h2>
          <p class="ov-score-sub">
            {scanReport.baseline_pass} dari {scanReport.baseline_total} kontrol lulus · Dipindai {formatUnixTime(scanReport.scanned_at)}
          </p>
        {:else}
          <span class="badge badge--neutral badge--pill">Menunggu Pemindaian</span>
          <h2 class="ov-score-title">Belum Ada Laporan Audit</h2>
          <p class="ov-score-sub">
            Jalankan audit menyeluruh untuk memeriksa firewall, port database, SSH, SSL, dan potensi webshell.
          </p>
        {/if}
      </div>

      <div class="ov-score-actions">
        <button
          type="button"
          class="btn btn--primary"
          disabled={isScanning}
          onclick={onRunScan}
        >
          <i class={isScanning ? 'ri-loader-4-line spin' : 'ri-radar-line'} aria-hidden="true"></i>
          <span>{isScanning ? 'Memindai Server...' : scanReport ? 'Pindai Ulang Sekarang' : 'Mulai Audit Keamanan'}</span>
        </button>
      </div>
    </div>

    <!-- 6 Pilar Proteksi Aktual -->
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-shield-star-line" aria-hidden="true"></i>
          <span>Status Pilar Perlindungan</span>
        </span>
        <span class="badge badge--pill" class:badge--success={(protection?.enabled_count || 0) >= 4} class:badge--warning={(protection?.enabled_count || 0) < 4}>
          {protection ? `${protection.enabled_count}/${protection.total_count} Aktif` : 'Memuat...'}
        </span>
      </div>

      <div class="card__body">
        <div class="ov-pillars-list">
          {#each protection?.pillars || [] as pillar (pillar.key)}
            <div class="ov-pillar-row">
              <div class="ov-pillar-info">
                <div class="ov-pillar-head">
                  <span class={pillarDotClass(pillar)}></span>
                  <span class="ov-pillar-name">{pillar.name}</span>
                  <span class={pillarBadgeClass(pillar)}>{pillar.status_text}</span>
                </div>
                <span class="ov-pillar-summary">{pillar.summary}</span>
              </div>

              <div class="ov-pillar-ctrl">
                {#if pillar.toggleable}
                  <label class="switch" aria-label={`Ubah status ${pillar.name}`}>
                    <input
                      type="checkbox"
                      role="switch"
                      checked={pillar.enabled}
                      disabled={busyKey === `pillar:${pillar.key}`}
                      onchange={() => onTogglePillar(pillar)}
                    />
                    <span class="switch__track"></span>
                  </label>
                {/if}
                <button
                  type="button"
                  class="btn btn--ghost btn--icon btn--sm"
                  title={`Kelola ${pillar.name}`}
                  aria-label={`Kelola ${pillar.name}`}
                  onclick={() => onNavigateTarget(pillar.target_tab)}
                >
                  <i class="ri-arrow-right-up-line" aria-hidden="true"></i>
                </button>
              </div>
            </div>
          {/each}
        </div>
      </div>
    </div>
  </div>

  <!-- Ringkasan Login Terakhir (SSH & Panel Aktual) -->
  <div class="ov-login-grid">
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-terminal-line" aria-hidden="true"></i>
          <span>Login SSH Terakhir</span>
        </span>
        <span class="badge {protection?.last_ssh_login.available ? 'badge--success' : 'badge--neutral'}">
          {protection?.last_ssh_login.available ? 'Tercatat' : 'Belum Ada Log'}
        </span>
      </div>
      <div class="card__body">
        {#if protection?.last_ssh_login.available}
          <div class="ov-login-kv">
            <div class="ov-kv-item">
              <span class="ov-kv-label">Alamat IP</span>
              <span class="ov-kv-val mono">{protection.last_ssh_login.ip}</span>
            </div>
            <div class="ov-kv-item">
              <span class="ov-kv-label">Pengguna</span>
              <span class="ov-kv-val mono">{protection.last_ssh_login.user}</span>
            </div>
            <div class="ov-kv-item">
              <span class="ov-kv-label">Waktu</span>
              <span class="ov-kv-val mono">{protection.last_ssh_login.time}</span>
            </div>
            <div class="ov-kv-item">
              <span class="ov-kv-label">Port Sumber</span>
              <span class="ov-kv-val mono">{protection.last_ssh_login.port || '-'}</span>
            </div>
          </div>
        {:else}
          <p class="text-xs text-muted">Belum ditemukan catatan sesi login SSH sukses dalam 7 hari terakhir.</p>
        {/if}
      </div>
    </div>

    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-admin-line" aria-hidden="true"></i>
          <span>Login Panel Terakhir</span>
        </span>
        <span class="badge {protection?.last_panel_login.available ? 'badge--success' : 'badge--neutral'}">
          {protection?.last_panel_login.available ? 'Tercatat' : 'Belum Ada Log'}
        </span>
      </div>
      <div class="card__body">
        {#if protection?.last_panel_login.available}
          <div class="ov-login-kv">
            <div class="ov-kv-item">
              <span class="ov-kv-label">Alamat IP</span>
              <span class="ov-kv-val mono">{protection.last_panel_login.ip}</span>
            </div>
            <div class="ov-kv-item">
              <span class="ov-kv-label">Akun Panel</span>
              <span class="ov-kv-val mono">{protection.last_panel_login.user || '-'}</span>
            </div>
            <div class="ov-kv-item">
              <span class="ov-kv-label">Waktu</span>
              <span class="ov-kv-val mono">{protection.last_panel_login.time}</span>
            </div>
            <div class="ov-kv-item">
              <span class="ov-kv-label">IP Anda Saat Ini</span>
              <span class="ov-kv-val mono">
                {protection.client_ip} {protection.is_client_whitelisted ? '(Whitelist)' : ''}
              </span>
            </div>
          </div>
        {:else}
          <p class="text-xs text-muted">Belum ditemukan riwayat sesi login panel.</p>
        {/if}
      </div>
    </div>
  </div>

  <!-- 4 Kartu Kategori Hasil Pemindaian -->
  <div class="ov-scan-grid">
    <button type="button" class="card ov-scan-card" onclick={() => onOpenScanCategory('risk')}>
      <div class="ov-scan-card__top">
        <span class="ov-scan-card__title">Risiko Konfigurasi</span>
        <span class="badge {scanReport && scanReport.risk_count > 0 ? 'badge--danger' : 'badge--success'}">
          {scanReport ? (scanReport.risk_count > 0 ? 'Perlu Tindakan' : 'Aman') : 'Menunggu'}
        </span>
      </div>
      <span class="ov-scan-card__num">{scanReport ? scanReport.risk_count : '-'}</span>
      <div class="ov-scan-card__foot">
        <span>Lihat detail temuan</span>
        <i class="ri-arrow-right-line" aria-hidden="true"></i>
      </div>
    </button>

    <button type="button" class="card ov-scan-card" onclick={() => onOpenScanCategory('malware')}>
      <div class="ov-scan-card__top">
        <span class="ov-scan-card__title">Malware &amp; Webshell</span>
        <span class="badge {scanReport && scanReport.malware_count > 0 ? 'badge--danger' : 'badge--success'}">
          {scanReport ? (scanReport.malware_count > 0 ? 'Ancaman' : 'Bersih') : 'Menunggu'}
        </span>
      </div>
      <span class="ov-scan-card__num">{scanReport ? scanReport.malware_count : '-'}</span>
      <div class="ov-scan-card__foot">
        <span>Lihat hasil webroot</span>
        <i class="ri-arrow-right-line" aria-hidden="true"></i>
      </div>
    </button>

    <button type="button" class="card ov-scan-card" onclick={() => onOpenScanCategory('vuln')}>
      <div class="ov-scan-card__top">
        <span class="ov-scan-card__title">Kerentanan &amp; Izin</span>
        <span class="badge {scanReport && scanReport.vuln_count > 0 ? 'badge--warning' : 'badge--success'}">
          {scanReport ? (scanReport.vuln_count > 0 ? 'Ditemukan' : 'Aman') : 'Menunggu'}
        </span>
      </div>
      <span class="ov-scan-card__num">{scanReport ? scanReport.vuln_count : '-'}</span>
      <div class="ov-scan-card__foot">
        <span>Periksa celah &amp; SSL</span>
        <i class="ri-arrow-right-line" aria-hidden="true"></i>
      </div>
    </button>

    <button type="button" class="card ov-scan-card" onclick={() => onOpenScanCategory('baseline')}>
      <div class="ov-scan-card__top">
        <span class="ov-scan-card__title">Standar Baseline</span>
        <span class="badge badge--info">
          {scanReport ? `${scanReport.baseline_pass}/${scanReport.baseline_total}` : 'Menunggu'}
        </span>
      </div>
      <span class="ov-scan-card__num">
        {scanReport && scanReport.baseline_total > 0
          ? `${Math.round((scanReport.baseline_pass / scanReport.baseline_total) * 100)}%`
          : '-'}
      </span>
      <div class="ov-scan-card__foot">
        <span>Tinjau seluruh poin</span>
        <i class="ri-arrow-right-line" aria-hidden="true"></i>
      </div>
    </button>
  </div>

  <!-- Daftar Rekomendasi Tindakan -->
  <div class="card">
    <div class="card__head">
      <span class="card__title">
        <i class="ri-list-check-3" aria-hidden="true"></i>
        <span>Rekomendasi Perbaikan</span>
      </span>
      <span class="badge badge--pill" class:badge--danger={actionableFindings.length > 0} class:badge--success={scanReport !== null && actionableFindings.length === 0}>
        {scanReport ? `${actionableFindings.length} item perlu ditangani` : 'Belum dipindai'}
      </span>
    </div>

    <div class="card__body">
      {#if !scanReport}
        <div class="empty-state">
          <i class="ri-shield-keyhole-line" aria-hidden="true"></i>
          <div class="empty-state__title">Belum Ada Hasil Audit</div>
          <p class="state__desc">
            Klik "Mulai Audit Keamanan" untuk mengevaluasi UFW, port database, SSH, Fail2Ban, TLS panel, 2FA, FIM, dan webroot situs Anda.
          </p>
        </div>
      {:else if actionableFindings.length === 0}
        <div class="empty-state">
          <i class="ri-checkbox-circle-line" aria-hidden="true"></i>
          <div class="empty-state__title">Seluruh Pemeriksaan Memenuhi Standar</div>
          <p class="state__desc">
            Tidak ditemukan risiko konfigurasi, kerentanan izin berkas, maupun indikasi webshell pada pemindaian terakhir.
          </p>
        </div>
      {:else}
        <div class="ov-rec-list">
          {#each actionableFindings as finding (finding.id)}
            <div
              class="ov-rec-item"
              class:ov-rec-item--critical={finding.severity === 'critical'}
              class:ov-rec-item--warning={finding.severity === 'warning'}
            >
              <div class="ov-rec-main">
                <div class="ov-rec-title-row">
                  <span class="badge {finding.severity === 'critical' ? 'badge--danger' : 'badge--warning'}">
                    {finding.severity === 'critical' ? 'KRITIS' : 'PERINGATAN'}
                  </span>
                  <span class="ov-rec-title">{finding.title}</span>
                </div>
                <p class="ov-rec-detail">{finding.detail}</p>
                {#if finding.fix_hint && finding.fix_hint !== '-'}
                  <span class="ov-rec-hint">Saran: {finding.fix_hint}</span>
                {/if}
              </div>

              <div class="ov-rec-actions">
                {#if finding.fixable && finding.action_key}
                  <button
                    type="button"
                    class="btn btn--primary btn--sm"
                    disabled={busyKey === `fix:${finding.action_key}`}
                    onclick={() => onApplyQuickFix(finding)}
                  >
                    <i class="ri-magic-line" aria-hidden="true"></i>
                    <span>{busyKey === `fix:${finding.action_key}` ? 'Menerapkan...' : 'Terapkan Perbaikan'}</span>
                  </button>
                {/if}
                <button
                  type="button"
                  class="btn btn--secondary btn--sm"
                  onclick={() => onNavigateTarget(finding.target_tab)}
                >
                  <span>Kelola</span>
                  <i class="ri-arrow-right-s-line" aria-hidden="true"></i>
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</div>
