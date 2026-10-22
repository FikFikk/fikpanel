<script lang="ts">
  import { onMount } from 'svelte';
  import type { MetricPoint } from '../types';

  interface Props {
    history: MetricPoint[];
    height?: number;
    currentCPU?: number;
    currentRAM?: number;
  }

  let { history = [], height = 230, currentCPU = 0, currentRAM = 0 }: Props = $props();

  let canvasEl: HTMLCanvasElement;
  let containerEl: HTMLDivElement;
  let ctx: CanvasRenderingContext2D | null = null;
  const maxPoints = 60;

  // State Tooltip Interaktif
  let isHovering = $state(false);
  let hoverX = $state(0);
  let hoverY = $state(0);
  let hoveredPoint = $state<MetricPoint | null>(null);
  let hoveredIndex = $state(0);

  // Kalkulasi Peak CPU & RAM dari buffer history
  let peakCPU = $derived(
    history.length > 0 ? Math.max(...history.map((p) => p.cpu || 0)) : currentCPU
  );
  let peakRAM = $derived(
    history.length > 0
      ? Math.max(
          ...history.map((p) => (p.ram_total > 0 ? (p.ram_used / p.ram_total) * 100 : 0))
        )
      : currentRAM
  );

  function renderChart() {
    if (!ctx || !canvasEl) return;
    const dpr = window.devicePixelRatio || 1;
    const w = canvasEl.width / dpr;
    const h = canvasEl.height / dpr;

    ctx.clearRect(0, 0, w, h);

    // Garis Grid Horizontal Lembut
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.05)';
    ctx.lineWidth = 1;
    for (let i = 0; i <= 4; i++) {
      const y = (h / 4) * i;
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(w, y);
      ctx.stroke();

      // Label Persentase Y-Axis
      ctx.fillStyle = 'rgba(148, 163, 184, 0.4)';
      ctx.font = '10px "Fira Code", monospace';
      ctx.fillText(`${100 - i * 25}%`, 8, y + 12);
    }

    if (history.length < 2) return;

    const pts = history.slice(-maxPoints);
    const step = w / (maxPoints - 1);

    // Fungsi menggambar kurva dengan vertical linearGradient glow
    function drawSeries(
      getValue: (p: MetricPoint) => number,
      strokeColor: string,
      topGradient: string,
      bottomGradient: string
    ) {
      if (!ctx) return;

      // Buat vertical linearGradient di canvas
      const grad = ctx.createLinearGradient(0, 0, 0, h);
      grad.addColorStop(0, topGradient);
      grad.addColorStop(1, bottomGradient);

      ctx.save();
      ctx.beginPath();
      pts.forEach((pt, i) => {
        const val = Math.min(Math.max(getValue(pt), 0), 100);
        const x = i * step;
        const y = h - (val / 100) * h;
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      });

      // Efek garis utama dengan drop-shadow tipis
      ctx.strokeStyle = strokeColor;
      ctx.lineWidth = 2;
      ctx.shadowColor = strokeColor;
      ctx.shadowBlur = 8;
      ctx.stroke();

      // Fill area gradasi ke bawah
      ctx.shadowColor = 'transparent';
      ctx.shadowBlur = 0;
      ctx.lineTo((pts.length - 1) * step, h);
      ctx.lineTo(0, h);
      ctx.closePath();
      ctx.fillStyle = grad;
      ctx.fill();
      ctx.restore();
    }

    // Series RAM (Cyan)
    drawSeries(
      (p) => (p.ram_total > 0 ? (p.ram_used / p.ram_total) * 100 : 0),
      '#06b6d4',
      'rgba(6, 182, 212, 0.16)',
      'rgba(6, 182, 212, 0.0)'
    );

    // Series CPU (Indigo)
    drawSeries(
      (p) => p.cpu || 0,
      '#818cf8',
      'rgba(99, 102, 241, 0.18)',
      'rgba(99, 102, 241, 0.0)'
    );

    // Garis Crosshair Interaktif jika sedang di-hover
    if (isHovering && hoveredIndex >= 0 && hoveredIndex < pts.length) {
      const crossX = hoveredIndex * step;
      ctx.save();
      ctx.beginPath();
      ctx.setLineDash([4, 4]);
      ctx.strokeStyle = 'rgba(255, 255, 255, 0.35)';
      ctx.lineWidth = 1;
      ctx.moveTo(crossX, 0);
      ctx.lineTo(crossX, h);
      ctx.stroke();

      // Titik indikator lingkaran pada posisi CPU & RAM
      const pt = pts[hoveredIndex];
      const cpuY = h - (Math.min(Math.max(pt.cpu || 0, 0), 100) / 100) * h;
      const ramVal = pt.ram_total > 0 ? (pt.ram_used / pt.ram_total) * 100 : 0;
      const ramY = h - (Math.min(Math.max(ramVal, 0), 100) / 100) * h;

      // Lingkaran CPU
      ctx.beginPath();
      ctx.arc(crossX, cpuY, 4, 0, Math.PI * 2);
      ctx.fillStyle = '#818cf8';
      ctx.fill();
      ctx.strokeStyle = '#0f172a';
      ctx.lineWidth = 1.5;
      ctx.stroke();

      // Lingkaran RAM
      ctx.beginPath();
      ctx.arc(crossX, ramY, 4, 0, Math.PI * 2);
      ctx.fillStyle = '#06b6d4';
      ctx.fill();
      ctx.strokeStyle = '#0f172a';
      ctx.lineWidth = 1.5;
      ctx.stroke();

      ctx.restore();
    }
  }

  function handleMouseMove(e: MouseEvent) {
    if (!canvasEl || history.length < 2) return;
    const rect = canvasEl.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;

    const pts = history.slice(-maxPoints);
    const step = rect.width / (maxPoints - 1);
    const index = Math.round(x / step);

    if (index >= 0 && index < pts.length) {
      isHovering = true;
      hoverX = Math.min(Math.max(x, 70), rect.width - 70);
      hoverY = y;
      hoveredIndex = index;
      hoveredPoint = pts[index];
      renderChart();
    }
  }

  function handleMouseLeave() {
    isHovering = false;
    hoveredPoint = null;
    renderChart();
  }

  function resizeCanvas() {
    if (!canvasEl || !containerEl) return;
    const dpr = window.devicePixelRatio || 1;
    const w = containerEl.clientWidth || 600;

    canvasEl.width = w * dpr;
    canvasEl.height = height * dpr;
    canvasEl.style.width = `${w}px`;
    canvasEl.style.height = `${height}px`;

    ctx = canvasEl.getContext('2d');
    if (ctx) {
      ctx.scale(dpr, dpr);
    }
    renderChart();
  }

  $effect(() => {
    if (history) {
      renderChart();
    }
  });

  onMount(() => {
    resizeCanvas();
    window.addEventListener('resize', resizeCanvas);
    return () => {
      window.removeEventListener('resize', resizeCanvas);
    };
  });
</script>

<div class="chart-wrapper">
  <!-- Baris Sub-Header: Live Stat Pills & Peak Indicators -->
  <div class="stat-pills-bar">
    <div class="pill-group">
      <!-- CPU Pill -->
      <div class="stat-badge badge-cpu">
        <span class="legend-dot dot-cpu"></span>
        <span class="badge-label">CPU:</span>
        <span class="badge-num">{currentCPU.toFixed(1)}%</span>
        <span class="badge-peak">Peak {peakCPU.toFixed(1)}%</span>
      </div>

      <!-- RAM Pill -->
      <div class="stat-badge badge-ram">
        <span class="legend-dot dot-ram"></span>
        <span class="badge-label">RAM:</span>
        <span class="badge-num">{currentRAM.toFixed(1)}%</span>
        <span class="badge-peak">Peak {peakRAM.toFixed(1)}%</span>
      </div>
    </div>
  </div>

  <!-- Kontainer Kanvas & Tooltip Interaktif -->
  <div
    class="canvas-container"
    bind:this={containerEl}
    onmousemove={handleMouseMove}
    onmouseleave={handleMouseLeave}
    role="region"
    aria-label="Interactive performance chart"
  >
    <canvas bind:this={canvasEl}></canvas>

    {#if isHovering && hoveredPoint}
      {@const hoveredRAM =
        hoveredPoint.ram_total > 0
          ? ((hoveredPoint.ram_used / hoveredPoint.ram_total) * 100).toFixed(1)
          : '0.0'}
      <div
        class="chart-tooltip"
        style="left: {hoverX}px; top: {Math.max(hoverY - 65, 10)}px;"
      >
        <div class="tooltip-time">
          {new Date(hoveredPoint.timestamp * 1000).toLocaleTimeString('id-ID', {
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
          })}
        </div>
        <div class="tooltip-row">
          <span class="tooltip-dot dot-cpu"></span>
          <span>CPU:</span>
          <span class="mono-bold">{(hoveredPoint.cpu || 0).toFixed(1)}%</span>
        </div>
        <div class="tooltip-row">
          <span class="tooltip-dot dot-ram"></span>
          <span>RAM:</span>
          <span class="mono-bold">{hoveredRAM}%</span>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .chart-wrapper {
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
    width: 100%;
  }

  /* Stat Pills Bar */
  .stat-pills-bar {
    display: flex;
    justify-content: flex-end;
    align-items: center;
  }
  .pill-group {
    display: flex;
    gap: 0.65rem;
    flex-wrap: wrap;
  }
  .stat-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.25rem 0.65rem;
    border-radius: 8px;
    font-size: 0.72rem;
    font-family: 'Fira Code', monospace;
    font-variant-numeric: tabular-nums;
    border: 1px solid transparent;
  }
  .badge-cpu {
    background: rgba(99, 102, 241, 0.1);
    color: #cbd5e1;
    border-color: rgba(99, 102, 241, 0.25);
  }
  .badge-ram {
    background: rgba(6, 182, 212, 0.1);
    color: #cbd5e1;
    border-color: rgba(6, 182, 212, 0.25);
  }
  .badge-num {
    font-weight: 700;
    color: #f8fafc;
  }
  .badge-peak {
    color: #94a3b8;
    font-size: 0.68rem;
    margin-left: 0.2rem;
    border-left: 1px solid rgba(255, 255, 255, 0.1);
    padding-left: 0.4rem;
  }
  .legend-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
  }
  .dot-cpu {
    background: #818cf8;
    box-shadow: 0 0 6px #818cf8;
  }
  .dot-ram {
    background: #06b6d4;
    box-shadow: 0 0 6px #06b6d4;
  }

  /* Canvas Container */
  .canvas-container {
    position: relative;
    width: 100%;
    overflow: hidden;
    cursor: crosshair;
  }
  canvas {
    display: block;
    width: 100%;
  }

  /* Floating Interactive Tooltip */
  .chart-tooltip {
    position: absolute;
    transform: translate(-50%, -100%);
    background: rgba(15, 23, 42, 0.95);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(255, 255, 255, 0.12);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.6);
    border-radius: 8px;
    padding: 0.45rem 0.65rem;
    pointer-events: none;
    z-index: 20;
    min-width: 110px;
    font-size: 0.72rem;
    font-family: 'Fira Code', monospace;
    font-variant-numeric: tabular-nums;
  }
  .tooltip-time {
    color: #94a3b8;
    font-size: 0.65rem;
    margin-bottom: 0.25rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
    padding-bottom: 0.2rem;
  }
  .tooltip-row {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    color: #cbd5e1;
    margin-top: 0.15rem;
  }
  .tooltip-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  .mono-bold {
    font-weight: 700;
    color: #f8fafc;
    margin-left: auto;
  }
</style>