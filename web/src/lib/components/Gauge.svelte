<script lang="ts">
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

  type Level = 'ok' | 'warn' | 'crit';

  let {
    percent = 0,
    title = '',
    subtitle = '',
    color = '#0ea5e9',
    icon = 'ri-dashboard-line',
    warningThreshold = 85,
    detailLabel = '',
    detailValue = '',
    showMiniBar = false,
    size = 120,
  }: Props = $props();

  // Status dibedakan lewat teks + ikon + warna, supaya tidak bergantung pada warna saja.
  const STATUS: Record<Level, { label: string; icon: string }> = {
    ok: { label: 'Normal', icon: 'ri-checkbox-circle-line' },
    warn: { label: 'Waspada', icon: 'ri-error-warning-line' },
    crit: { label: 'Kritis', icon: 'ri-alert-fill' },
  };

  const STROKE = 8;
  const TICK_OVERSHOOT = 2;

  const center = $derived(size / 2);
  const radius = $derived((size - STROKE) / 2 - TICK_OVERSHOOT);

  const clamped = $derived(Math.min(Math.max(percent, 0), 100));
  const limit = $derived(Math.min(Math.max(warningThreshold, 0), 100));
  const shown = $derived(Math.round(clamped));

  const level: Level = $derived(
    clamped >= limit
      ? 'crit'
      : clamped >= Math.max(limit - 15, 65)
        ? 'warn'
        : 'ok'
  );

  const accent = $derived(
    level === 'crit' ? '#ef4444' : level === 'warn' ? '#f59e0b' : color
  );
  const status = $derived(STATUS[level]);
</script>

<section class="gauge" data-level={level} style:--accent={accent}>
  <header class="head">
    <h3 class="title">
      <i class={icon} aria-hidden="true"></i>
      <span>{title}</span>
    </h3>

    <span class="status">
      <i class={status.icon} aria-hidden="true"></i>
      {status.label}
    </span>
  </header>

  <div
    class="dial"
    style:width="{size}px"
    style:height="{size}px"
    role="meter"
    aria-label={title}
    aria-valuemin="0"
    aria-valuemax="100"
    aria-valuenow={shown}
    aria-valuetext="{shown}%, {status.label}"
  >
    <svg width={size} height={size} viewBox="0 0 {size} {size}" aria-hidden="true">
      <circle
        cx={center}
        cy={center}
        r={radius}
        fill="none"
        stroke="var(--g-track)"
        stroke-width={STROKE}
      />

      <circle
        class="progress"
        cx={center}
        cy={center}
        r={radius}
        fill="none"
        stroke="var(--accent)"
        stroke-width={STROKE}
        pathLength="100"
        stroke-dasharray="100"
        stroke-dashoffset={100 - clamped}
        transform="rotate(-90 {center} {center})"
      />

      <!-- Tanda batas peringatan pada ring -->
      <line
        class="limit-tick"
        x1={center}
        y1={center - radius - STROKE / 2 - TICK_OVERSHOOT}
        x2={center}
        y2={center - radius + STROKE / 2 + TICK_OVERSHOOT}
        transform="rotate({limit * 3.6} {center} {center})"
      />
    </svg>

    <div class="readout">
      <span class="value">{shown}</span><span class="unit">%</span>
    </div>
  </div>

  <dl class="facts">
    <div class="fact">
      <dt>{detailLabel || 'Penggunaan'}</dt>
      <dd class="fact-main">{subtitle || '-'}</dd>
    </div>

    {#if detailValue}
      <div class="fact">
        <dt class="sr-only">Keterangan</dt>
        <dd class="fact-note">{detailValue}</dd>
      </div>
    {/if}

    <div class="fact">
      <dt>Batas peringatan</dt>
      <dd>{limit}%</dd>
    </div>
  </dl>

  {#if showMiniBar}
    <div class="bar" aria-hidden="true">
      <div class="bar-track">
        <div class="bar-fill" style:transform="scaleX({clamped / 100})"></div>
      </div>
      <div class="bar-limit" style:left="{limit}%"></div>
    </div>
  {/if}
</section>

<style>
  .gauge {
    /* Bisa dioverride dari parent: --gauge-surface, --gauge-border, dst. */
    --g-surface: var(--gauge-surface, #0e131b);
    --g-border: var(--gauge-border, #232d3b);
    --g-track: var(--gauge-track, #1c2533);
    --g-text: var(--gauge-text, #e8edf4);
    --g-muted: var(--gauge-text-muted, #9aa6b8);

    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: 1rem;
    padding: 1rem;
    background: var(--g-surface);
    border: 1px solid var(--g-border);
    border-radius: 10px;
    color: var(--g-text);
    font-family: inherit;
    font-variant-numeric: tabular-nums;
    transition: border-color 0.2s ease;
  }

  .gauge[data-level='warn'] {
    border-color: color-mix(in srgb, var(--accent) 40%, var(--g-border));
  }

  .gauge[data-level='crit'] {
    border-color: color-mix(in srgb, var(--accent) 55%, var(--g-border));
  }

  /* Header */
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .title {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin: 0;
    min-width: 0;
    font-size: 0.9375rem;
    font-weight: 600;
    line-height: 1.3;
  }

  .title i {
    flex: none;
    font-size: 1.125rem;
    color: var(--g-muted);
  }

  .title span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* Status: tenang saat normal, menonjol saat waspada/kritis */
  .status {
    display: inline-flex;
    flex: none;
    align-items: center;
    gap: 0.3rem;
    padding: 0.2rem 0.5rem;
    border-radius: 6px;
    font-size: 0.75rem;
    font-weight: 500;
    line-height: 1.2;
    color: var(--g-muted);
  }

  .status i {
    font-size: 0.9375rem;
  }

  .gauge[data-level='ok'] .status i {
    color: var(--accent);
  }

  .gauge[data-level='warn'] .status,
  .gauge[data-level='crit'] .status {
    color: var(--accent);
    background: color-mix(in srgb, var(--accent) 14%, transparent);
    font-weight: 600;
  }

  /* Dial */
  .dial {
    position: relative;
    align-self: center;
    flex: none;
  }

  .progress {
    transition:
      stroke-dashoffset 0.3s cubic-bezier(0.22, 1, 0.36, 1),
      stroke 0.2s ease;
  }

  .limit-tick {
    stroke: var(--g-text);
    stroke-width: 2;
    opacity: 0.55;
  }

  .readout {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: baseline;
    justify-content: center;
    padding-top: 0;
    align-content: center;
    line-height: 1;
  }

  .readout {
    align-items: center;
  }

  .value {
    font-size: 1.875rem;
    font-weight: 600;
    letter-spacing: -0.02em;
  }

  .unit {
    margin-left: 0.125rem;
    font-size: 1rem;
    font-weight: 500;
    color: var(--g-muted);
  }

  /* Data */
  .facts {
    display: flex;
    flex-direction: column;
    gap: 0.375rem;
    margin: 0;
  }

  .fact {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.75rem;
    font-size: 0.8125rem;
    line-height: 1.4;
  }

  .fact dt {
    color: var(--g-muted);
  }

  .fact dd {
    margin: 0;
    text-align: right;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .fact-main {
    font-size: 0.9375rem;
    font-weight: 600;
  }

  .fact-note {
    width: 100%;
    color: var(--g-muted);
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }

  /* Mini bar */
  .bar {
    position: relative;
    display: flex;
    align-items: center;
    height: 8px;
  }

  .bar-track {
    width: 100%;
    height: 6px;
    border-radius: 3px;
    background: var(--g-track);
    overflow: hidden;
  }

  .bar-fill {
    width: 100%;
    height: 100%;
    background: var(--accent);
    transform-origin: left center;
    will-change: transform;
    transition: transform 0.3s cubic-bezier(0.22, 1, 0.36, 1);
  }

  .bar-limit {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 2px;
    margin-left: -1px;
    background: var(--g-text);
    opacity: 0.55;
    border-radius: 1px;
  }

  @media (prefers-reduced-motion: reduce) {
    .progress,
    .bar-fill,
    .gauge {
      transition: none;
    }
  }
</style>