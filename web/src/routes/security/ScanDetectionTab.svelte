<script lang="ts">
  import type { SecurityScanReport, ScanFinding } from '../../lib/types';
  import '../../styles/pages/security/scan-detection.css';

  interface Props {
    isScanning: boolean;
    scanReport: SecurityScanReport | null;
    scanCategoryFilter: string;
    filteredScanFindings: ScanFinding[];
    onRunScan: () => void;
    onSelectCategory: (category: string) => void;
  }

  let {
    isScanning,
    scanReport,
    scanCategoryFilter,
    filteredScanFindings,
    onRunScan,
    onSelectCategory,
  }: Props = $props();
</script>

<div class="pg-security-scan-detection tab-pane">
  <header class="pane-header">
    <div>
      <h2 class="pane-title">Scan Detection & Vulnerability Analysis</h2>
      <p class="pane-subtitle">Hasil pemindaian heuristik mendalam terhadap berkas web, izin berkas kritis, dan konfigurasi server.</p>
    </div>
    <button class="btn btn--primary" disabled={isScanning} onclick={onRunScan}>
      <i class="ri-radar-line"></i> {isScanning ? 'Memindai...' : 'Jalankan Pemindaian Baru'}
    </button>
  </header>

  <!-- Kategori Filter Hasil -->
  <div class="segmented mb-4">
    <button class:active={scanCategoryFilter === 'all'} onclick={() => onSelectCategory('all')}>
      Semua Temuan
    </button>
    <button class:active={scanCategoryFilter === 'risk'} onclick={() => onSelectCategory('risk')}>
      Security Risks
    </button>
    <button class:active={scanCategoryFilter === 'malware'} onclick={() => onSelectCategory('malware')}>
      Malware / Webshell
    </button>
    <button class:active={scanCategoryFilter === 'vuln'} onclick={() => onSelectCategory('vuln')}>
      Vulnerabilities
    </button>
    <button class:active={scanCategoryFilter === 'baseline'} onclick={() => onSelectCategory('baseline')}>
      Baseline Checks
    </button>
  </div>

  {#if !scanReport || filteredScanFindings.length === 0}
    <div class="card p-8 text-center text-muted">
      <i class="ri-shield-check-line scan-empty-icon"></i>
      <p class="mt-2 font-bold text-white">Tidak ada temuan pada kategori ini.</p>
      <p class="text-xs text-subtle mt-1">Sistem berjalan dengan konfigurasi yang memenuhi standar keamanan.</p>
    </div>
  {:else}
    <div class="findings-list">
      {#each filteredScanFindings as finding}
        <div class="card finding-card">
          <div class="finding-head">
            <span class="badge {finding.severity === 'critical' ? 'badge--danger' : finding.severity === 'warning' ? 'badge--warning' : 'badge--success'}">
              {finding.severity.toUpperCase()}
            </span>
            <span class="finding-title">{finding.title}</span>
            <span class="badge badge--pill mono text-subtle ml-auto">{finding.category}</span>
          </div>
          <p class="finding-detail">{finding.detail}</p>
          {#if finding.fix_hint && finding.fix_hint !== '-'}
            <div class="finding-fix">
              <i class="ri-lightbulb-line text-warning"></i>
              <span>Solusi: <strong class="mono">{finding.fix_hint}</strong></span>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

