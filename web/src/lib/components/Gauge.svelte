<script lang="ts">
  // Properti Gauge radial modern dengan ambient glow & reactive warning
  interface Props {
    percent: number;
    title: string;
    subtitle: string;
    color?: string;
    icon?: string;
    warningThreshold?: number;
    detailLabel?: string;
    detailValue?: string;
    showMiniBar?: boolean;
    size?: number;
  }

  let {
    percent = 0,
    title = '',
    subtitle = '',
    color = '#10b981',
    icon = 'ri-dashboard-line',
    warningThreshold = 85,
    detailLabel = '',
    detailValue = '',
    showMiniBar = false,
    size = 110,
  }: Props = $props();

  const strokeWidth = 8;
  let radius = $derived((size - strokeWidth) / 2);
  let circumference = $derived(2 * Math.PI * radius);

  let clampedPercent = $derived(Math.min(Math.max(percent, 0), 100));
  let strokeDashoffset = $derived(circumference - (clampedPercent / 100) * circumference);
  let isWarning = $derived(clampedPercent >= warningThreshold);
  let activeColor = $derived(isWarning ? '#ef4444' : color);
</script>

<div class="gauge-card {isWarning ? 'card-warning' : ''}" style="--accent-color: {activeColor};">
  <!-- Baris Atas: Icon + Judul + Status Badge -->
  <div class="card-top">
    <div class="title-wrap">
      <span class="icon-chip" style="color: {activeColor}; background: {activeColor}15; border-color: {activeColor}30;">
        <i class="{icon}"></i>
      </span>
      <span class="gauge-title">{title}</span>
    </div>

    {#if isWarning}
      <span class="badge-warn">
        <span class="pulse-warn"></span>
        High
      </span>
    {:else}
      <span class="badge-normal">
        <span class="dot-normal" style="background: {activeColor};"></span>
        Normal
      </span>
    {/if}
  </div>

  <!-- Area Tengah: SVG Gauge Lingkaran dengan Ambient Radial Glow -->
  <div class="gauge-center">
    <div class="ambient-glow" style="background: radial-gradient(circle, {activeColor}22 0%, transparent 70%);"></div>

    <div class="svg-wrapper" style="width: {size}px; height: {size}px;">
      <svg width={size} height={size}>
        <!-- Lingkaran latar belakang (Track) -->
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="transparent"
          stroke="rgba(255, 255, 255, 0.06)"
          stroke-width={strokeWidth}
        />
        <!-- Lingkaran progress dengan drop-shadow menyala -->
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="transparent"
          stroke={activeColor}
          stroke-width={strokeWidth}
          stroke-dasharray={circumference}
          stroke-dashoffset={strokeDashoffset}
          stroke-linecap="round"
          transform="rotate(-90 {size / 2} {size / 2})"
          class="progress-ring"
          style="filter: drop-shadow(0 0 6px {activeColor}60);"
        />
      </svg>
      <!-- Angka Persentase Tegas di Tengah -->
      <div class="gauge-number" style="color: {activeColor};">
        <span class="number-val">{clampedPercent.toFixed(0)}</span><span class="number-unit">%</span>
      </div>
    </div>
  </div>

  <!-- Baris Bawah: Detail Metrik & Mini Progress Bar -->
  <div class="card-bottom">
    <div class="metric-text-row">
      <span class="metric-subtitle">{subtitle}</span>
      {#if detailValue}
        <span class="detail-badge">{detailValue}</span>
      {/if}
    </div>

    {#if showMiniBar}
      <div class="mini-bar-track">
        <div
          class="mini-bar-fill"
          style="width: {clampedPercent}%; background: {activeColor}; box-shadow: 0 0 8px {activeColor}80;"
        ></div>
      </div>
    {/if}
  </div>
</div>

<style>
  .gauge-card {
    position: relative;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    background: rgba(17, 20, 29, 0.85);
    backdrop-filter: blur(16px);
    border: 1px solid rgba(255, 255, 255, 0.07);
    box-shadow: inset 0 1px 0 0 rgba(255, 255, 255, 0.08), 0 4px 20px rgba(0, 0, 0, 0.35);
    border-radius: 14px;
    padding: 1.15rem 1.25rem;
    transition: transform 0.25s cubic-bezier(0.2, 0.8, 0.2, 1), border-color 0.25s, box-shadow 0.25s;
    overflow: hidden;
  }
  .gauge-card:hover {
    transform: translateY(-2px);
    border-color: rgba(255, 255, 255, 0.16);
    box-shadow: inset 0 1px 0 0 rgba(255, 255, 255, 0.12), 0 10px 30px rgba(0, 0, 0, 0.45), 0 0 15px -3px var(--accent-color);
  }
  .gauge-card.card-warning {
    border-color: rgba(239, 68, 68, 0.35);
  }

  /* Header Kartu */
  .card-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
  }
  .title-wrap {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .icon-chip {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    border-radius: 7px;
    font-size: 0.95rem;
    border: 1px solid transparent;
  }
  .gauge-title {
    font-size: 0.88rem;
    font-weight: 600;
    color: #f3f4f6;
    letter-spacing: -0.01em;
  }

  .badge-normal {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.7rem;
    font-weight: 500;
    color: #9ca3af;
    background: rgba(255, 255, 255, 0.04);
    padding: 0.2rem 0.55rem;
    border-radius: 999px;
    border: 1px solid rgba(255, 255, 255, 0.06);
  }
  .dot-normal {
    width: 5px;
    height: 5px;
    border-radius: 50%;
  }
  .badge-warn {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.7rem;
    font-weight: 600;
    color: #ef4444;
    background: rgba(239, 68, 68, 0.12);
    border: 1px solid rgba(239, 68, 68, 0.3);
    padding: 0.2rem 0.55rem;
    border-radius: 999px;
  }
  .pulse-warn {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #ef4444;
    box-shadow: 0 0 8px #ef4444;
    animation: pingWarn 1.6s infinite;
  }
  @keyframes pingWarn {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.4; transform: scale(1.3); }
  }

  /* Center Area */
  .gauge-center {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0.75rem 0;
  }
  .ambient-glow {
    position: absolute;
    width: 130px;
    height: 130px;
    border-radius: 50%;
    pointer-events: none;
    z-index: 0;
  }
  .svg-wrapper {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1;
  }
  .progress-ring {
    transition: stroke-dashoffset 0.6s cubic-bezier(0.22, 1, 0.36, 1);
  }
  .gauge-number {
    position: absolute;
    display: flex;
    align-items: baseline;
    font-family: 'Fira Code', monospace;
    font-variant-numeric: tabular-nums;
  }
  .number-val {
    font-size: 1.55rem;
    font-weight: 700;
    letter-spacing: -0.03em;
  }
  .number-unit {
    font-size: 0.85rem;
    font-weight: 600;
    margin-left: 1px;
    opacity: 0.85;
  }

  /* Footer Kartu */
  .card-bottom {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    margin-top: 0.25rem;
  }
  .metric-text-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-family: 'Fira Code', monospace;
    font-variant-numeric: tabular-nums;
  }
  .metric-subtitle {
    font-size: 0.75rem;
    color: #94a3b8;
  }
  .detail-badge {
    font-size: 0.7rem;
    color: #cbd5e1;
    background: rgba(255, 255, 255, 0.05);
    padding: 0.15rem 0.45rem;
    border-radius: 5px;
    border: 1px solid rgba(255, 255, 255, 0.06);
  }
  .mini-bar-track {
    width: 100%;
    height: 4px;
    background: rgba(255, 255, 255, 0.06);
    border-radius: 999px;
    overflow: hidden;
  }
  .mini-bar-fill {
    height: 100%;
    border-radius: 999px;
    transition: width 0.6s cubic-bezier(0.22, 1, 0.36, 1);
  }
</style>