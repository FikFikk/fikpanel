<script lang="ts">
  import { onMount } from 'svelte';
  import type { MetricPoint } from '../types';

  interface Props {
    history: MetricPoint[];
    height?: number;
    currentCPU?: number;
    currentRAM?: number;
  }

  let { history = [], height = 220, currentCPU = 0, currentRAM = 0 }: Props = $props();

  const MAX_POINTS = 60;
  const COLOR_CPU = '#0ea5e9'; // Sky 500 - tegas & kontras tinggi
  const COLOR_RAM = '#6366f1'; // Indigo 500

  // Formatter waktu di-cache sekali agar tidak alokasi ulang saat mouse bergerak
  const timeFormatter = new Intl.DateTimeFormat('id-ID', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });

  let canvasEl: HTMLCanvasElement;
  let containerEl: HTMLDivElement;
  let ctx: CanvasRenderingContext2D | null = null;

  // Dimensi & cache gradient
  let viewW = $state(600);
  let cpuGrad: CanvasGradient | null = null;
  let ramGrad: CanvasGradient | null = null;

  // State Interaksi Hover (Hanya memicu update layer kompositor, bukan redraw kanvas)
  let isHovering = $state(false);
  let hoveredIndex = $state(-1);

  // Slice buffer terakhir maksimal 60 titik
  let visiblePoints = $derived(history.slice(-MAX_POINTS));

  // Kalkulasi statistik (Peak & nilai RAM) dalam satu kali loop (Single-Pass) without .map()
  let stats = $derived.by(() => {
    let maxCpu = currentCPU;
    let maxRam = currentRAM;
    let lastRamUsed = 0;
    let lastRamTotal = 0;

    for (let i = 0; i < history.length; i++) {
      const p = history[i];
      const cpu = p.cpu || 0;
      const ram = p.ram_total > 0 ? (p.ram_used / p.ram_total) * 100 : 0;
      if (cpu > maxCpu) maxCpu = cpu;
      if (ram > maxRam) maxRam = ram;
    }

    if (history.length > 0) {
      const last = history[history.length - 1];
      lastRamUsed = last.ram_used || 0;
      lastRamTotal = last.ram_total || 0;
    }

    return {
      peakCPU: maxCpu,
      peakRAM: maxRam,
      ramUsed: lastRamUsed,
      ramTotal: lastRamTotal,
    };
  });

  // Kalkulasi posisi & nilai titik yang sedang di-hover
  let activeHover = $derived.by(() => {
    if (!isHovering || hoveredIndex < 0 || hoveredIndex >= visiblePoints.length) {
      return null;
    }
    const pt = visiblePoints[hoveredIndex];
    const step = viewW / (MAX_POINTS - 1);
    const x = hoveredIndex * step;

    const cpuVal = Math.min(Math.max(pt.cpu || 0, 0), 100);
    const ramVal =
      pt.ram_total > 0
        ? Math.min(Math.max((pt.ram_used / pt.ram_total) * 100, 0), 100)
        : 0;

    const cpuY = height - (cpuVal / 100) * height;
    const ramY = height - (ramVal / 100) * height;

    // Posisi tooltip dibatasi agar tidak keluar dari tepi kiri/kanan panel
    const tooltipX = Math.min(Math.max(x, 76), viewW - 76);
    const topY = Math.min(cpuY, ramY);
    const tooltipY = Math.max(topY - 12, 68);

    return {
      x,
      cpuY,
      ramY,
      cpuVal,
      ramVal,
      ramUsed: pt.ram_used || 0,
      ramTotal: pt.ram_total || 0,
      timeStr: pt.timestamp ? timeFormatter.format(pt.timestamp * 1000) : '--:--:--',
      tooltipX,
      tooltipY,
    };
  });

  function formatBytes(val: number): string {
    if (!val || val <= 0) return '';
    // Deteksi otomatis apakah satuan dalam MB, GB, atau Bytes
    if (val > 1024 * 1024 * 1024) return `${(val / (1024 * 1024 * 1024)).toFixed(1)}G`;
    if (val > 1024 * 1024) return `${(val / (1024 * 1024)).toFixed(1)}G`;
    if (val > 1024) return `${(val / 1024).toFixed(1)}G`;
    return `${val.toFixed(1)}G`;
  }

  function buildGradients(h: number) {
    if (!ctx) return;
    cpuGrad = ctx.createLinearGradient(0, 0, 0, h);
    cpuGrad.addColorStop(0, 'rgba(14, 165, 233, 0.16)');
    cpuGrad.addColorStop(1, 'rgba(14, 165, 233, 0.0)');

    ramGrad = ctx.createLinearGradient(0, 0, 0, h);
    ramGrad.addColorStop(0, 'rgba(99, 102, 241, 0.14)');
    ramGrad.addColorStop(1, 'rgba(99, 102, 241, 0.0)');
  }

  function drawChart(pts: MetricPoint[], w: number, h: number) {
    if (!ctx || !canvasEl) return;

    ctx.clearRect(0, 0, w, h);

    // 1. Garis Grid Horizontal & Sub-Grid Presisi
    ctx.lineWidth = 1;
    for (let i = 0; i <= 4; i++) {
      // Snap ke 0.5px agar garis 1px terlihat tajam di layar standar
      const y = Math.floor((h / 4) * i) + 0.5;
      ctx.strokeStyle = i === 4 ? 'rgba(148, 163, 184, 0.14)' : 'rgba(148, 163, 184, 0.06)';
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(w, y);
      ctx.stroke();
    }

    // Garis Grid Vertikal (Interval 15 titik / 15 detik)
    const step = w / (MAX_POINTS - 1);
    ctx.strokeStyle = 'rgba(148, 163, 184, 0.04)';
    for (let i = 15; i < MAX_POINTS; i += 15) {
      const x = Math.floor(i * step) + 0.5;
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, h);
      ctx.stroke();
    }

    if (pts.length < 2) return;

    if (!cpuGrad || !ramGrad) buildGradients(h);

    // 2. Helper Penggambar Kurva
    function renderTrace(
      getValue: (p: MetricPoint) => number,
      strokeColor: string,
      fillGrad: CanvasGradient | null
    ) {
      if (!ctx) return;
      const lastX = (pts.length - 1) * step;

      ctx.beginPath();
      for (let i = 0; i < pts.length; i++) {
        const val = Math.min(Math.max(getValue(pts[i]), 0), 100);
        const x = i * step;
        const y = h - (val / 100) * h;
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }

      ctx.strokeStyle = strokeColor;
      ctx.lineWidth = 1.5;
      ctx.lineJoin = 'round';
      ctx.lineCap = 'round';
      ctx.stroke();

      if (fillGrad) {
        ctx.lineTo(lastX, h);
        ctx.lineTo(0, h);
        ctx.closePath();
        ctx.fillStyle = fillGrad;
        ctx.fill();
      }
    }

    // Gambar layer RAM terlebih dahulu, lalu CPU di atasnya
    renderTrace(
      (p) => (p.ram_total > 0 ? (p.ram_used / p.ram_total) * 100 : 0),
      COLOR_RAM,
      ramGrad
    );

    renderTrace((p) => p.cpu || 0, COLOR_CPU, cpuGrad);
  }

  function handlePointerMove(e: PointerEvent) {
    if (!canvasEl || visiblePoints.length < 2) return;
    const rect = canvasEl.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const step = rect.width / (MAX_POINTS - 1);
    const idx = Math.round(x / step);

    if (idx >= 0 && idx < visiblePoints.length) {
      isHovering = true;
      if (hoveredIndex !== idx) {
        hoveredIndex = idx;
      }
    } else {
      isHovering = false;
      hoveredIndex = -1;
    }
  }

  function handlePointerLeave() {
    isHovering = false;
    hoveredIndex = -1;
  }

  function syncCanvasSize() {
    if (!canvasEl || !containerEl) return;
    const dpr = window.devicePixelRatio || 1;
    const w = containerEl.clientWidth || 600;
    viewW = w;

    canvasEl.width = Math.round(w * dpr);
    canvasEl.height = Math.round(height * dpr);
    canvasEl.style.width = `${w}px`;
    canvasEl.style.height = `${height}px`;

    ctx = canvasEl.getContext('2d', { alpha: true, desynchronized: true });
    if (ctx) {
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      buildGradients(height);
      drawChart(visiblePoints, w, height);
    }
  }

  // Effect HANYA bergantung pada visiblePoints dan viewW, BUKAN state hover
  $effect(() => {
    drawChart(visiblePoints, viewW, height);
  });

  onMount(() => {
    syncCanvasSize();
    const ro = new ResizeObserver(() => {
      syncCanvasSize();
    });
    ro.observe(containerEl);
    return () => ro.disconnect();
  });
</script>

<div class="telemetry-panel">
  <!-- Header Strip Bergaya Server Control Panel -->
  <div class="telemetry-bar">
    <div class="telemetry-status">
      <span class="pulse-indicator"></span>
      <span class="status-label">REALTIME TELEMETRY</span>
      <span class="status-window">{visiblePoints.length}s / {MAX_POINTS}s</span>
    </div>

    <div class="metric-readouts">
      <!-- Readout CPU -->
      <div class="metric-cell">
        <span class="metric-accent accent-cpu"></span>
        <span class="metric-name">CPU</span>
        <span
          class="metric-val"
          class:val-warn={currentCPU >= 75 && currentCPU < 90}
          class:val-crit={currentCPU >= 90}
        >
          {currentCPU.toFixed(1)}%
        </span>
        <span class="metric-sub">PK {stats.peakCPU.toFixed(1)}%</span>
      </div>

      <!-- Readout RAM -->
      <div class="metric-cell">
        <span class="metric-accent accent-ram"></span>
        <span class="metric-name">RAM</span>
        <span
          class="metric-val"
          class:val-warn={currentRAM >= 80 && currentRAM < 92}
          class:val-crit={currentRAM >= 92}
        >
          {currentRAM.toFixed(1)}%
        </span>
        {#if stats.ramTotal > 0}
          <span class="metric-sub">
            {formatBytes(stats.ramUsed)}/{formatBytes(stats.ramTotal)}
          </span>
        {:else}
          <span class="metric-sub">PK {stats.peakRAM.toFixed(1)}%</span>
        {/if}
      </div>
    </div>
  </div>

  <!-- Area Kanvas & Overlay Kompositor GPU -->
  <div
    class="viewport"
    style="height: {height}px;"
    bind:this={containerEl}
    onpointermove={handlePointerMove}
    onpointerleave={handlePointerLeave}
    role="region"
    aria-label="Server resource telemetry chart"
  >
    <!-- Label Y-Axis Statis di Layer CSS (Tidak perlu digambar ulang di Canvas) -->
    <div class="y-axis" aria-hidden="true">
      <span>100%</span>
      <span>75%</span>
      <span>50%</span>
      <span>25%</span>
      <span>0%</span>
    </div>

    <canvas bind:this={canvasEl}></canvas>

    <!-- Overlay Crosshair & Titik (Zero-Canvas-Redraw, GPU Accelerated) -->
    {#if activeHover}
      <div
        class="crosshair-line"
        style="transform: translate3d({activeHover.x}px, 0, 0);"
      ></div>

      <div
        class="hover-node node-cpu"
        style="transform: translate3d({activeHover.x}px, {activeHover.cpuY}px, 0);"
      ></div>

      <div
        class="hover-node node-ram"
        style="transform: translate3d({activeHover.x}px, {activeHover.ramY}px, 0);"
      ></div>

      <div
        class="hud-tooltip"
        style="transform: translate3d({activeHover.tooltipX}px, {activeHover.tooltipY}px, 0) translate(-50%, -100%);"
      >
        <div class="hud-header">
          <span>TIME</span>
          <span class="hud-time">{activeHover.timeStr}</span>
        </div>
        <div class="hud-row">
          <span class="hud-mark accent-cpu"></span>
          <span class="hud-key">CPU</span>
          <span class="hud-num">{activeHover.cpuVal.toFixed(1)}%</span>
        </div>
        <div class="hud-row">
          <span class="hud-mark accent-ram"></span>
          <span class="hud-key">RAM</span>
          <span class="hud-num">{activeHover.ramVal.toFixed(1)}%</span>
        </div>
      </div>
    {/if}
  </div>

  <!-- Sub-Footer Time Axis -->
  <div class="x-axis" aria-hidden="true">
    <span>-60s</span>
    <span>-45s</span>
    <span>-30s</span>
    <span>-15s</span>
    <span>NOW</span>
  </div>
</div>

<style>
  .telemetry-panel {
    display: flex;
    flex-direction: column;
    width: 100%;
    background: #0b0f17;
    border: 1px solid #1e293b;
    border-radius: 6px;
    overflow: hidden;
    font-family: 'JetBrains Mono', 'Fira Code', ui-monospace, SFMono-Regular, monospace;
    font-variant-numeric: tabular-nums;
    user-select: none;
  }

  /* Header Bar */
  .telemetry-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.5rem;
    padding: 0.45rem 0.75rem;
    background: #0f1521;
    border-bottom: 1px solid #1e293b;
  }

  .telemetry-status {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.68rem;
    letter-spacing: 0.04em;
    color: #64748b;
  }

  .pulse-indicator {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #10b981;
    box-shadow: 0 0 6px rgba(16, 185, 129, 0.6);
  }

  .status-label {
    font-weight: 600;
    color: #94a3b8;
  }

  .status-window {
    padding: 0.1rem 0.35rem;
    background: #161f30;
    border: 1px solid #1e293b;
    border-radius: 3px;
    color: #64748b;
    font-size: 0.64rem;
  }

  /* Readout Cells */
  .metric-readouts {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .metric-cell {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.2rem 0.55rem;
    background: #0b0f17;
    border: 1px solid #1e293b;
    border-radius: 4px;
    font-size: 0.72rem;
  }

  .metric-accent {
    width: 3px;
    height: 12px;
    border-radius: 1px;
  }

  .accent-cpu {
    background: #0ea5e9;
  }

  .accent-ram {
    background: #6366f1;
  }

  .metric-name {
    color: #94a3b8;
    font-weight: 500;
  }

  .metric-val {
    font-weight: 700;
    color: #f8fafc;
    min-width: 42px;
    text-align: right;
  }

  .val-warn {
    color: #f59e0b;
  }

  .val-crit {
    color: #ef4444;
  }

  .metric-sub {
    color: #64748b;
    font-size: 0.65rem;
    padding-left: 0.4rem;
    border-left: 1px solid #1e293b;
  }

  /* Chart Viewport */
  .viewport {
    position: relative;
    width: 100%;
    overflow: hidden;
    cursor: crosshair;
    background: #0b0f17;
  }

  canvas {
    display: block;
    width: 100%;
    height: 100%;
  }

  /* Static Y-Axis Overlay */
  .y-axis {
    position: absolute;
    top: 0;
    left: 6px;
    bottom: 0;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    padding: 4px 0;
    pointer-events: none;
    z-index: 2;
    font-size: 0.62rem;
    color: rgba(148, 163, 184, 0.4);
  }

  /* GPU-Accelerated Crosshair & Nodes */
  .crosshair-line {
    position: absolute;
    top: 0;
    left: 0;
    bottom: 0;
    width: 1px;
    background: rgba(226, 232, 240, 0.25);
    pointer-events: none;
    will-change: transform;
    z-index: 3;
  }

  .hover-node {
    position: absolute;
    top: -4px;
    left: -4px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    border: 1.5px solid #0b0f17;
    pointer-events: none;
    will-change: transform;
    z-index: 4;
  }

  .node-cpu {
    background: #0ea5e9;
  }

  .node-ram {
    background: #6366f1;
  }

  /* HUD Tooltip */
  .hud-tooltip {
    position: absolute;
    top: 0;
    left: 0;
    min-width: 124px;
    background: rgba(11, 15, 23, 0.94);
    border: 1px solid #334155;
    border-radius: 4px;
    padding: 0.35rem 0.5rem;
    pointer-events: none;
    will-change: transform;
    z-index: 10;
    font-size: 0.68rem;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  }

  .hud-header {
    display: flex;
    justify-content: space-between;
    color: #64748b;
    font-size: 0.62rem;
    padding-bottom: 0.2rem;
    margin-bottom: 0.2rem;
    border-bottom: 1px solid #1e293b;
  }

  .hud-time {
    color: #cbd5e1;
    font-weight: 600;
  }

  .hud-row {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    margin-top: 0.15rem;
  }

  .hud-mark {
    width: 2px;
    height: 9px;
    border-radius: 1px;
  }

  .hud-key {
    color: #94a3b8;
  }

  .hud-num {
    margin-left: auto;
    font-weight: 700;
    color: #f8fafc;
  }

  /* Footer X-Axis */
  .x-axis {
    display: flex;
    justify-content: space-between;
    padding: 0.25rem 0.65rem;
    background: #0d121c;
    border-top: 1px solid #1e293b;
    font-size: 0.62rem;
    color: #475569;
  }
</style>