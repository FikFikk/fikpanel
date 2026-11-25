<script lang="ts">
  import type {
    Fail2BanStatus,
    HardeningStatus,
    FIMCheckResult,
    SecurityScanReport,
    ScanFinding,
  } from '../../lib/types';
  import '../../styles/pages/security/hardening.css';

  export type HardeningSubTab = 'scanner' | 'fail2ban' | 'kernel' | 'fim';

  interface Props {
    hardeningSubTab: HardeningSubTab;
    scanReport: SecurityScanReport | null;
    scanCategoryFilter: string;
    isScanning: boolean;
    f2bStatus: Fail2BanStatus | null;
    hardening: HardeningStatus | null;
    fimResult: FIMCheckResult | null;
    busyKey: string;
    onSelectSubTab: (tab: HardeningSubTab) => void;
    onSelectScanCategory: (cat: string) => void;
    onRunScan: () => void;
    onApplyQuickFix: (finding: ScanFinding) => void;
    onNavigateTarget: (target: string) => void;
    onControlF2B: (action: 'start' | 'stop' | 'restart') => void;
    onBanF2B: (jail: string, ip: string) => void;
    onUnbanF2B: (jail: string, ip: string) => void;
    onToggleSysctl: (key: string, enabled: boolean) => void;
    onToggleCompiler: () => void;
    onCreateFIMBaseline: () => void;
    onCheckFIM: () => void;
    onDisableFIM: () => void;
  }

  let {
    hardeningSubTab,
    scanReport,
    scanCategoryFilter,
    isScanning,
    f2bStatus,
    hardening,
    fimResult,
    busyKey,
    onSelectSubTab,
    onSelectScanCategory,
    onRunScan,
    onApplyQuickFix,
    onNavigateTarget,
    onControlF2B,
    onBanF2B,
    onUnbanF2B,
    onToggleSysctl,
    onToggleCompiler,
    onCreateFIMBaseline,
    onCheckFIM,
    onDisableFIM,
  }: Props = $props();

  let manualBanJail = $state('');
  let manualBanIP = $state('');

  // Pilih jail pertama otomatis saat daftar jail Fail2Ban dimuat
  $effect(() => {
    if (!manualBanJail && f2bStatus?.jails && f2bStatus.jails.length > 0) {
      manualBanJail = f2bStatus.jails[0].name;
    }
  });

  let findings = $derived(scanReport?.findings || []);
  let filteredFindings = $derived.by(() => {
    if (scanCategoryFilter === 'all') return findings;
    return findings.filter((f) => f.category === scanCategoryFilter);
  });

  let catCounts = $derived.by(() => {
    const counts = { all: findings.length, risk: 0, malware: 0, vuln: 0, baseline: 0 };
    for (const f of findings) {
      if (f.category === 'risk') counts.risk++;
      else if (f.category === 'malware') counts.malware++;
      else if (f.category === 'vuln') counts.vuln++;
      else if (f.category === 'baseline') counts.baseline++;
    }
    return counts;
  });

  function formatUnix(ts: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString('id-ID');
  }

  function shortHash(h: string): string {
    if (!h) return '-';
    return h.length > 16 ? `${h.slice(0, 12)}…` : h;
  }

  function submitManualBan() {
    if (!manualBanJail || !manualBanIP.trim()) return;
    onBanF2B(manualBanJail, manualBanIP.trim());
    manualBanIP = '';
  }
</script>

<div class="pg-security-hardening">
  <!-- Sub-tabs Hardening & Scanner -->
  <div class="tabs tabs--underline" role="tablist">
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={hardeningSubTab === 'scanner'}
      aria-selected={hardeningSubTab === 'scanner'}
      onclick={() => onSelectSubTab('scanner')}
    >
      <i class="ri-radar-line" aria-hidden="true"></i>
      <span>Hasil Pemindaian ({catCounts.all})</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={hardeningSubTab === 'fail2ban'}
      aria-selected={hardeningSubTab === 'fail2ban'}
      onclick={() => onSelectSubTab('fail2ban')}
    >
      <i class="ri-shield-flash-line" aria-hidden="true"></i>
      <span>Fail2Ban IPS</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={hardeningSubTab === 'kernel'}
      aria-selected={hardeningSubTab === 'kernel'}
      onclick={() => onSelectSubTab('kernel')}
    >
      <i class="ri-cpu-line" aria-hidden="true"></i>
      <span>Kernel &amp; Compiler</span>
    </button>
    <button
      type="button"
      role="tab"
      class="tabs__tab"
      class:is-active={hardeningSubTab === 'fim'}
      aria-selected={hardeningSubTab === 'fim'}
      onclick={() => onSelectSubTab('fim')}
    >
      <i class="ri-file-shield-2-line" aria-hidden="true"></i>
      <span>File Integrity (FIM)</span>
    </button>
  </div>

  <!-- SUBTAB 1: Detail Hasil Pemindaian Keamanan -->
  {#if hardeningSubTab === 'scanner'}
    <div class="card">
      <div class="sec-toolbar">
        <div class="sec-toolbar-left">
          <div class="segmented">
            <button type="button" class:active={scanCategoryFilter === 'all'} onclick={() => onSelectScanCategory('all')}>
              Semua ({catCounts.all})
            </button>
            <button type="button" class:active={scanCategoryFilter === 'risk'} onclick={() => onSelectScanCategory('risk')}>
              Risiko ({catCounts.risk})
            </button>
            <button type="button" class:active={scanCategoryFilter === 'malware'} onclick={() => onSelectScanCategory('malware')}>
              Malware ({catCounts.malware})
            </button>
            <button type="button" class:active={scanCategoryFilter === 'vuln'} onclick={() => onSelectScanCategory('vuln')}>
              Kerentanan ({catCounts.vuln})
            </button>
            <button type="button" class:active={scanCategoryFilter === 'baseline'} onclick={() => onSelectScanCategory('baseline')}>
              Baseline ({catCounts.baseline})
            </button>
          </div>
        </div>
        <div class="sec-toolbar-right">
          {#if scanReport}
            <span class="badge badge--info mono">Skor: {scanReport.score}/100 ({scanReport.rating})</span>
          {/if}
          <button type="button" class="btn btn--primary btn--sm" disabled={isScanning} onclick={onRunScan}>
            <i class={isScanning ? 'ri-loader-4-line spin' : 'ri-radar-line'} aria-hidden="true"></i>
            <span>{isScanning ? 'Memindai...' : 'Jalankan Pemindaian Baru'}</span>
          </button>
        </div>
      </div>

      <div class="card__body">
        {#if !scanReport}
          <div class="empty-state">
            <i class="ri-radar-line" aria-hidden="true"></i>
            <div class="empty-state__title">Pemindaian Belum Pernah Dijalankan</div>
            <p class="state__desc">
              Klik "Jalankan Pemindaian Baru" untuk mengaudit UFW, eksposur database, SSH, Fail2Ban, TLS panel, 2FA, FIM, SSL situs, dan webroot PHP.
            </p>
            <div class="state__actions">
              <button type="button" class="btn btn--primary" disabled={isScanning} onclick={onRunScan}>
                Mulai Pemindaian Sekarang
              </button>
            </div>
          </div>
        {:else if filteredFindings.length === 0}
          <div class="empty-state">
            <i class="ri-checkbox-circle-line" aria-hidden="true"></i>
            <div class="empty-state__title">Tidak Ada Temuan pada Kategori Ini</div>
            <p class="state__desc">
              Pemindaian terakhir ({formatUnix(scanReport.scanned_at)}) tidak mencatat masalah pada kategori filter yang dipilih.
            </p>
          </div>
        {:else}
          <div class="hd-findings">
            {#each filteredFindings as finding (finding.id)}
              <div
                class="card hd-finding-card"
                class:hd-finding-card--critical={finding.severity === 'critical'}
                class:hd-finding-card--warning={finding.severity === 'warning'}
              >
                <div class="hd-finding-head">
                  <span
                    class="badge"
                    class:badge--danger={finding.severity === 'critical'}
                    class:badge--warning={finding.severity === 'warning'}
                    class:badge--success={finding.severity === 'safe'}
                  >
                    {finding.severity === 'critical' ? 'KRITIS' : finding.severity === 'warning' ? 'PERINGATAN' : 'AMAN'}
                  </span>
                  <span class="hd-finding-title">{finding.title}</span>
                  <span class="badge badge--neutral badge--pill mono hd-finding-cat">{finding.category}</span>
                </div>

                <p class="hd-finding-detail">{finding.detail}</p>

                {#if finding.fix_hint && finding.fix_hint !== '-'}
                  <div class="hd-finding-fix">
                    <div class="hd-finding-fix-text">
                      <i class="ri-lightbulb-line" aria-hidden="true"></i>
                      <span>Solusi: <strong class="mono">{finding.fix_hint}</strong></span>
                    </div>
                    <div class="row">
                      {#if finding.fixable && finding.action_key}
                        <button
                          type="button"
                          class="btn btn--primary btn--sm"
                          disabled={busyKey === `fix:${finding.action_key}`}
                          onclick={() => onApplyQuickFix(finding)}
                        >
                          Terapkan
                        </button>
                      {/if}
                      <button
                        type="button"
                        class="btn btn--secondary btn--sm"
                        onclick={() => onNavigateTarget(finding.target_tab)}
                      >
                        Buka Modul
                      </button>
                    </div>
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {/if}

  <!-- SUBTAB 2: Fail2Ban Intrusion Prevention -->
  {#if hardeningSubTab === 'fail2ban'}
    <div class="card">
      <div class="card__head">
        <div class="row">
          <span class="card__title">
            <i class="ri-shield-flash-line" aria-hidden="true"></i>
            <span>Fail2Ban Intrusion Prevention</span>
          </span>
          <span
            class="badge badge--pill"
            class:badge--success={f2bStatus?.running}
            class:badge--danger={f2bStatus?.installed && !f2bStatus?.running}
            class:badge--neutral={!f2bStatus?.installed}
          >
            {!f2bStatus?.installed ? 'Belum Terpasang' : f2bStatus.running ? 'Aktif (Running)' : 'Berhenti (Stopped)'}
          </span>
        </div>

        <div class="row">
          {#if f2bStatus?.installed}
            {#if !f2bStatus.running}
              <button
                type="button"
                class="btn btn--primary btn--sm"
                disabled={busyKey === 'f2b:service'}
                onclick={() => onControlF2B('start')}
              >
                <i class="ri-play-line" aria-hidden="true"></i>
                <span>Start</span>
              </button>
            {:else}
              <button
                type="button"
                class="btn btn--secondary btn--sm"
                disabled={busyKey === 'f2b:service'}
                onclick={() => onControlF2B('restart')}
              >
                <i class="ri-restart-line" aria-hidden="true"></i>
                <span>Restart</span>
              </button>
              <button
                type="button"
                class="btn btn--danger btn--sm"
                disabled={busyKey === 'f2b:service'}
                onclick={() => onControlF2B('stop')}
              >
                <i class="ri-stop-line" aria-hidden="true"></i>
                <span>Stop</span>
              </button>
            {/if}
          {/if}
        </div>
      </div>

      {#if f2bStatus?.running && f2bStatus.jails.length > 0}
        <div class="hd-f2b-ban-bar">
          <div class="field hd-f2b-ban-field">
            <label class="field__label" for="f2b-jail-sel">Jail Target</label>
            <select id="f2b-jail-sel" class="select" bind:value={manualBanJail}>
              {#each f2bStatus.jails as j (j.name)}
                <option value={j.name}>{j.name}</option>
              {/each}
            </select>
          </div>
          <div class="field hd-f2b-ban-field">
            <label class="field__label" for="f2b-ban-ip">Ban IP Manual</label>
            <input
              id="f2b-ban-ip"
              type="text"
              class="input input--mono"
              placeholder="Contoh: 185.220.101.44"
              bind:value={manualBanIP}
            />
          </div>
          <button
            type="button"
            class="btn btn--danger btn--sm"
            disabled={!manualBanIP.trim() || busyKey === 'f2b:ban'}
            onclick={submitManualBan}
          >
            <i class="ri-forbid-line" aria-hidden="true"></i>
            <span>Ban IP</span>
          </button>
        </div>
      {/if}

      {#if !f2bStatus?.installed}
        <div class="card__body">
          <div class="empty-state">
            <i class="ri-shield-cross-line" aria-hidden="true"></i>
            <div class="empty-state__title">Fail2Ban Belum Terpasang</div>
            <p class="state__desc">
              {f2bStatus?.is_linux
                ? 'Pasang paket fail2ban via Terminal (`apt install -y fail2ban`) untuk pemblokiran otomatis serangan brute-force SSH & Nginx.'
                : 'Fail2Ban hanya beroperasi pada server Linux.'}
            </p>
          </div>
        </div>
      {:else if !f2bStatus.jails || f2bStatus.jails.length === 0}
        <div class="card__body">
          <div class="empty-state">
            <i class="ri-shield-line" aria-hidden="true"></i>
            <div class="empty-state__title">Tidak Ada Jail Aktif</div>
            <p class="state__desc">
              {f2bStatus.running
                ? 'Fail2Ban berjalan tetapi belum ada jail yang diaktifkan pada /etc/fail2ban/jail.local.'
                : 'Nyalakan layanan Fail2Ban terlebih dahulu untuk melihat daftar jail aktif.'}
            </p>
          </div>
        </div>
      {:else}
        <div class="table-wrap">
          <table class="table table--dense">
            <thead>
              <tr>
                <th>Nama Jail</th>
                <th>Gagal Saat Ini</th>
                <th>Total Gagal</th>
                <th>Diblokir</th>
                <th>Daftar IP Terblokir (Klik × untuk Unban)</th>
              </tr>
            </thead>
            <tbody>
              {#each f2bStatus.jails as j (j.name)}
                <tr>
                  <td class="mono"><strong>{j.name}</strong></td>
                  <td class="mono tabular">{j.currently_failed}</td>
                  <td class="mono tabular">{j.total_failed}</td>
                  <td>
                    <span class="badge {j.banned_count > 0 ? 'badge--danger' : 'badge--success'}">
                      {j.banned_count} IP
                    </span>
                  </td>
                  <td>
                    {#if !j.banned_ips || j.banned_ips.length === 0}
                      <span class="text-subtle text-xs">Tidak ada IP terblokir</span>
                    {:else}
                      <div class="hd-ip-chips">
                        {#each j.banned_ips as ip (ip)}
                          <span class="hd-ip-chip">
                            <span>{ip}</span>
                            <button
                              type="button"
                              class="hd-ip-chip__btn"
                              title={`Unban ${ip} dari jail ${j.name}`}
                              aria-label={`Unban ${ip} dari jail ${j.name}`}
                              disabled={busyKey === `f2b:unban:${ip}`}
                              onclick={() => onUnbanF2B(j.name, ip)}
                            >
                              <i class="ri-close-line" aria-hidden="true"></i>
                            </button>
                          </span>
                        {/each}
                      </div>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}

  <!-- SUBTAB 3: Kernel Sysctl & Compiler Hardening -->
  {#if hardeningSubTab === 'kernel'}
    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-cpu-line" aria-hidden="true"></i>
          <span>Pengerasan Kernel Linux (Live /proc/sys &amp; Persisten sysctl.d)</span>
        </span>
        <span class="badge badge--info mono">/etc/sysctl.d/99-fikpanel-hardening.conf</span>
      </div>
      <div class="card__body">
        <div class="sec-setting-list">
          {#each hardening?.sysctl_params || [] as param (param.key)}
            <div class="sec-setting-row">
              <div class="sec-setting-info">
                <span class="sec-setting-title">
                  <span>{param.title}</span>
                  <span class="hd-sysctl-key">{param.key} = {param.current_value || '?'}</span>
                  {#if param.persisted}
                    <span class="badge badge--success">Persisten</span>
                  {/if}
                </span>
                <span class="sec-setting-desc">{param.description}</span>
              </div>
              <div class="sec-setting-ctrl">
                <label class="switch" aria-label={`Ubah ${param.title}`}>
                  <input
                    type="checkbox"
                    role="switch"
                    checked={param.enabled}
                    disabled={busyKey === `sysctl:${param.key}`}
                    onchange={() => onToggleSysctl(param.key, !param.enabled)}
                  />
                  <span class="switch__track"></span>
                </label>
              </div>
            </div>
          {/each}
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card__head">
        <span class="card__title">
          <i class="ri-code-box-line" aria-hidden="true"></i>
          <span>Pembatasan Akses Compiler Sistem</span>
        </span>
        <span class="badge {hardening?.compiler_blocked ? 'badge--success' : 'badge--neutral'}">
          {hardening?.compiler_blocked ? 'Dibatasi (chmod 700)' : 'Terbuka (0755)'}
        </span>
      </div>
      <div class="card__body">
        <div class="sec-setting-row">
          <div class="sec-setting-info">
            <span class="sec-setting-title">Batasi Eksekusi Compiler untuk Non-Root</span>
            <span class="sec-setting-desc">
              Mengubah izin biner compiler ke <code>0700</code> agar skrip web/user non-root tidak dapat mengompilasi exploit lokal di server.
            </span>
            {#if hardening?.compilers_found && hardening.compilers_found.length > 0}
              <div class="hd-compiler-chips">
                {#each hardening.compilers_found as c}
                  <span class="badge badge--neutral mono">{c}</span>
                {/each}
              </div>
            {/if}
          </div>
          <div class="sec-setting-ctrl">
            <label class="switch" aria-label="Batasi akses compiler untuk non-root">
              <input
                type="checkbox"
                role="switch"
                checked={hardening?.compiler_blocked || false}
                disabled={busyKey === 'hardening:compiler'}
                onchange={onToggleCompiler}
              />
              <span class="switch__track"></span>
            </label>
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- SUBTAB 4: File Integrity Monitor (FIM / Anti-Tamper Real) -->
  {#if hardeningSubTab === 'fim'}
    <div class="sec-kpi-grid">
      <div class="stat-card">
        <div class="stat-card__icon">
          <i class="ri-file-shield-2-line" aria-hidden="true"></i>
        </div>
        <div class="stat-card__body">
          <span class="stat-card__label">Total Berkas Dipantau</span>
          <span class="stat-card__value">{fimResult?.total_files ?? 0}</span>
          <span class="stat-card__sub">{fimResult?.enabled ? 'Baseline SHA-256 aktif' : 'Belum diinisialisasi'}</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-card__icon stat-card__icon--info">
          <i class="ri-checkbox-circle-line" aria-hidden="true"></i>
        </div>
        <div class="stat-card__body">
          <span class="stat-card__label">Integritas Cocok</span>
          <span class="stat-card__value">{fimResult?.ok_count ?? 0}</span>
          <span class="stat-card__sub">Hash &amp; izin identik</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-card__icon stat-card__icon--danger">
          <i class="ri-error-warning-line" aria-hidden="true"></i>
        </div>
        <div class="stat-card__body">
          <span class="stat-card__label">Berkas Berubah</span>
          <span class="stat-card__value">{fimResult?.modified_count ?? 0}</span>
          <span class="stat-card__sub">Mismatch SHA-256 / chmod</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-card__icon stat-card__icon--warning">
          <i class="ri-file-damage-line" aria-hidden="true"></i>
        </div>
        <div class="stat-card__body">
          <span class="stat-card__label">Berkas Hilang</span>
          <span class="stat-card__value">{fimResult?.missing_count ?? 0}</span>
          <span class="stat-card__sub">Terhapus dari disk</span>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="sec-toolbar">
        <div class="sec-toolbar-left">
          <span class="card__title">File Integrity Monitor (Checksum SHA-256)</span>
          {#if fimResult?.last_baseline_at}
            <span class="badge badge--neutral mono">Baseline: {formatUnix(fimResult.last_baseline_at)}</span>
          {/if}
        </div>
        <div class="sec-toolbar-right">
          {#if fimResult?.enabled}
            <button
              type="button"
              class="btn btn--secondary btn--sm"
              disabled={busyKey === 'fim:check'}
              onclick={onCheckFIM}
            >
              <i class="ri-search-eye-line" aria-hidden="true"></i>
              <span>{busyKey === 'fim:check' ? 'Memeriksa...' : 'Verifikasi Sekarang'}</span>
            </button>
            <button
              type="button"
              class="btn btn--ghost btn--sm"
              disabled={busyKey === 'fim:disable'}
              onclick={onDisableFIM}
            >
              Nonaktifkan
            </button>
          {/if}
          <button
            type="button"
            class="btn btn--primary btn--sm"
            disabled={busyKey === 'fim:baseline'}
            onclick={onCreateFIMBaseline}
          >
            <i class="ri-fingerprint-line" aria-hidden="true"></i>
            <span>{busyKey === 'fim:baseline' ? 'Menghitung Hash...' : fimResult?.enabled ? 'Perbarui Baseline' : 'Buat Baseline SHA-256'}</span>
          </button>
        </div>
      </div>

      {#if !fimResult || !fimResult.enabled}
        <div class="card__body">
          <div class="empty-state">
            <i class="ri-file-shield-line" aria-hidden="true"></i>
            <div class="empty-state__title">File Integrity Monitor Belum Aktif</div>
            <p class="state__desc">
              Buat snapshot baseline SHA-256 berkas sistem kritis (<code>/etc/passwd</code>, <code>/etc/sudoers</code>, <code>sshd_config</code>, konfigurasi Nginx) untuk mendeteksi modifikasi tidak sah secara akurat.
            </p>
          </div>
        </div>
      {:else}
        <div class="table-wrap">
          <table class="table table--dense">
            <thead>
              <tr>
                <th>Jalur Berkas</th>
                <th>Status</th>
                <th>Izin</th>
                <th>Ukuran</th>
                <th>SHA-256 Baseline</th>
                <th>SHA-256 Aktual</th>
              </tr>
            </thead>
            <tbody>
              {#each fimResult.items as item (item.path)}
                <tr>
                  <td class="mono">{item.path}</td>
                  <td>
                    <span
                      class="badge"
                      class:badge--success={item.status === 'ok'}
                      class:badge--danger={item.status === 'modified'}
                      class:badge--warning={item.status === 'missing'}
                    >
                      {item.status === 'ok' ? 'VALID' : item.status === 'modified' ? 'BERUBAH' : 'HILANG'}
                    </span>
                  </td>
                  <td class="mono text-subtle">{item.mode}</td>
                  <td class="mono text-subtle">{item.size} B</td>
                  <td class="hd-fim-hash" title={item.expected_hash}>{shortHash(item.expected_hash)}</td>
                  <td class="hd-fim-hash" title={item.actual_hash}>{shortHash(item.actual_hash)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}
</div>
