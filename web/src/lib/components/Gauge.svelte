<script lang="ts">
  import '../../styles/pages/components/gauge.css';

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
    color = 'var(--info)',
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
    level === 'crit' ? 'var(--danger)' : level === 'warn' ? 'var(--warning)' : color
  );
  const status = $derived(STATUS[level]);
</script>

<section class="cmp-gauge gauge" data-level={level} style:--accent={accent}>
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
    style:--dial-size="{size}px"
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
        <div class="bar-fill" style:--bar-scale={clamped / 100}></div>
      </div>
      <div class="bar-limit" style:--bar-limit="{limit}%"></div>
    </div>
  {/if}
</section>