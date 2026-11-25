<script lang="ts">
  import { onMount } from 'svelte';
  import { formatBytes } from '../api';
  import type { MonitorSnapshot } from '../types';
  import { readToken } from '../utils/browser';
  import '../../styles/pages/components/history-chart.css';

  export type HistoryChartMode = 'resource' | 'load' | 'network' | 'disk_io' | 'disk_space';

  interface Props {
    snapshots: MonitorSnapshot[];
    height?: number;
    mode?: HistoryChartMode;
    thresholdValue?: number;
    thresholdLabel?: string;
  }

  let {
    snapshots = [],
    height = 280,
    mode = 'resource',
    thresholdValue = 0,
    thresholdLabel = '',
  }: Props = $props();

  const PAD_LEFT = 58;
  const PAD_RIGHT = 18;
  const PAD_TOP = 20;
  const PAD_BOTTOM = 28;

  let canvasEl: HTMLCanvasElement;
  let containerEl: HTMLDivElement;
  let ctx: CanvasRenderingContext2D | null = null;
  let viewW = $state(800);
  let themeVersion = $state(0);

  // State hover (hanya menggerakkan layer overlay CSS, tanpa redraw penuh kanvas)
  let isHovering = $state(false);
  let hoveredIndex = $state(-1);

  const timeFormatter = new Intl.DateTimeFormat('id-ID', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });

  // Hitung nilai maksimum sumbu Y berdasarkan mode aktif
  let computedMaxY = $derived.by(() => {
    if (mode === 'resource' || mode === 'disk_space') return 100;
    let maxY = 1.0;
    if (mode === 'load') {
      for (const s of snapshots) {
        if (s.load_1m > maxY) maxY = s.load_1m;
        if (s.load_5m > maxY) maxY = s.load_5m;
        if (s.load_15m > maxY) maxY = s.load_15m;
      }
      return Math.max(maxY * 1.2, 1.0);
    }
    if (mode === 'network') {
      maxY = 1024;
      for (const s of snapshots) {
        if (s.net_in > maxY) maxY = s.net_in;
        if (s.net_out > maxY) maxY = s.net_out;
      }
      return maxY * 1.2;
    }
    // disk_io
    maxY = 1024;
    for (const s of snapshots) {
      if (s.disk_read > maxY) maxY = s.disk_read;
      if (s.disk_write > maxY) maxY = s.disk_write;
    }
    return maxY * 1.2;
  });

  // Hitung posisi crosshair & data tooltip untuk indeks yang di-hover
  let activeHover = $derived.by(() => {
    if (!isHovering || hoveredIndex < 0 || hoveredIndex >= snapshots.length || snapshots.length < 2) {
      return null;
    }
    const plotW = Math.max(viewW - PAD_LEFT - PAD_RIGHT, 1);
    const x = PAD_LEFT + (hoveredIndex / (snapshots.length - 1)) * plotW;
    const tipX = Math.min(Math.max(x, 125), Math.max(viewW - 135, 125));
    return {
      x,
      tipX,
      snap: snapshots[hoveredIndex],
    };
  });

  $effect(() => {
    const _snaps = snapshots;
    const _m = mode;
    const _w = viewW;
    const _th = thresholdValue;
    const _tv = themeVersion;
    draw();
  });

  function draw() {
    if (!canvasEl || !ctx) return;
    const dpr = window.devicePixelRatio || 1;
    const w = canvasEl.width / dpr;
    const h = canvasEl.height / dpr;

    const colorMuted = readToken('--text-muted', 'slategray');
    const colorInfo = readToken('--info', 'dodgerblue');
    const colorViolet = readToken('--violet', 'mediumpurple');
    const colorAccent = readToken('--accent', 'coral');
    const colorWarning = readToken('--warning', 'orange');
    const colorDanger = readToken('--danger', 'crimson');
    const colorSuccess = readToken('--success', 'mediumseagreen');
    const fontMono = readToken('--font-mono', 'monospace');

    ctx.save();
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, w, h);

    const plotW = Math.max(w - PAD_LEFT - PAD_RIGHT, 1);
    const plotH = Math.max(h - PAD_TOP - PAD_BOTTOM, 1);
    const maxY = computedMaxY;

    // Grid horizontal & label sumbu Y
    ctx.strokeStyle = colorMuted;
    ctx.lineWidth = 1;
    ctx.fillStyle = colorMuted;
    ctx.font = `10px ${fontMono}`;
    ctx.textAlign = 'right';

    const steps = 4;
    for (let i = 0; i <= steps; i++) {
      const y = Math.floor(PAD_TOP + (plotH / steps) * i) + 0.5;
      const val = maxY - (maxY / steps) * i;

      ctx.globalAlpha = 0.14;
      ctx.beginPath();
      ctx.moveTo(PAD_LEFT, y);
      ctx.lineTo(PAD_LEFT + plotW, y);
      ctx.stroke();

      let label = val.toFixed(1);
      if (mode === 'resource' || mode === 'disk_space') label = `${val.toFixed(0)}%`;
      else if (mode === 'network' || mode === 'disk_io') label = `${formatBytes(val)}/s`;
      ctx.globalAlpha = 0.72;
      ctx.fillText(label, PAD_LEFT - 8, y + 3);
    }

    if (snapshots.length < 2) {
      ctx.globalAlpha = 0.65;
      ctx.textAlign = 'center';
      ctx.fillText(
        'Belum cukup sampel histori pada rentang waktu ini (minimal 2 titik).',
        PAD_LEFT + plotW / 2,
        PAD_TOP + plotH / 2
      );
      ctx.restore();
      return;
    }

    const getX = (idx: number) => PAD_LEFT + (idx / (snapshots.length - 1)) * plotW;
    const getY = (val: number) => PAD_TOP + plotH - (Math.min(Math.max(val, 0), maxY) / maxY) * plotH;

    // Label sumbu waktu X
    ctx.globalAlpha = 0.7;
    ctx.textAlign = 'center';
    const xTicks = Math.min(5, snapshots.length - 1);
    for (let t = 0; t <= xTicks; t++) {
      const idx = Math.round((t / xTicks) * (snapshots.length - 1));
      const x = getX(idx);
      const timeStr = timeFormatter.format(new Date(snapshots[idx].timestamp * 1000));
      ctx.fillText(timeStr, x, h - 8);
    }
    ctx.globalAlpha = 1;

    // Helper penggambar kurva + gradasi area halus
    function drawSeries(vals: number[], color: string, lineWidth: number, fillAlpha: number) {
      if (!ctx || vals.length < 2) return;
      ctx.beginPath();
      for (let i = 0; i < vals.length; i++) {
        const x = getX(i);
        const y = getY(vals[i]);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.strokeStyle = color;
      ctx.lineWidth = lineWidth;
      ctx.lineJoin = 'round';
      ctx.lineCap = 'round';
      ctx.stroke();

      if (fillAlpha > 0) {
        const grad = ctx.createLinearGradient(0, PAD_TOP, 0, PAD_TOP + plotH);
        grad.addColorStop(0, color);
        grad.addColorStop(1, 'transparent');
        ctx.lineTo(getX(vals.length - 1), PAD_TOP + plotH);
        ctx.lineTo(getX(0), PAD_TOP + plotH);
        ctx.closePath();
        ctx.save();
        ctx.globalAlpha = fillAlpha;
        ctx.fillStyle = grad;
        ctx.fill();
        ctx.restore();
      }
    }

    if (mode === 'resource') {
      drawSeries(
        snapshots.map((s) => (s.ram_total > 0 ? (s.ram_used / s.ram_total) * 100 : 0)),
        colorViolet,
        1.8,
        0.13
      );
      drawSeries(snapshots.map((s) => s.cpu), colorInfo, 2, 0.16);
      drawSeries(snapshots.map((s) => s.cpu_iowait || 0), colorWarning, 1.25, 0);
    } else if (mode === 'load') {
      drawSeries(snapshots.map((s) => s.load_15m), colorAccent, 1.5, 0.06);
      drawSeries(snapshots.map((s) => s.load_5m), colorViolet, 1.6, 0.08);
      drawSeries(snapshots.map((s) => s.load_1m), colorInfo, 2, 0.15);
    } else if (mode === 'network') {
      drawSeries(snapshots.map((s) => s.net_out), colorWarning, 1.8, 0.12);
      drawSeries(snapshots.map((s) => s.net_in), colorAccent, 2, 0.16);
    } else if (mode === 'disk_io') {
      drawSeries(snapshots.map((s) => s.disk_write || 0), colorWarning, 1.8, 0.12);
      drawSeries(snapshots.map((s) => s.disk_read || 0), colorSuccess, 2, 0.16);
    } else if (mode === 'disk_space') {
      drawSeries(
        snapshots.map((s) => (s.disk_total > 0 ? (s.disk_used / s.disk_total) * 100 : 0)),
        colorAccent,
        2,
        0.18
      );
    }

    // Gambar garis batas ambang alert putus-putus bila relevan (mode resource / disk_space)
    if ((mode === 'resource' || mode === 'disk_space') && thresholdValue > 0 && thresholdValue <= 100) {
      const ty = Math.floor(getY(thresholdValue)) + 0.5;
      ctx.save();
      ctx.strokeStyle = colorDanger;
      ctx.globalAlpha = 0.6;
      ctx.lineWidth = 1;
      ctx.setLineDash([5, 4]);
      ctx.beginPath();
      ctx.moveTo(PAD_LEFT, ty);
      ctx.lineTo(PAD_LEFT + plotW, ty);
      ctx.stroke();
      ctx.setLineDash([]);
      if (thresholdLabel) {
        ctx.fillStyle = colorDanger;
        ctx.globalAlpha = 0.85;
        ctx.textAlign = 'right';
        ctx.fillText(thresholdLabel, PAD_LEFT + plotW - 4, Math.max(ty - 4, PAD_TOP + 10));
      }
      ctx.restore();
    }

    ctx.restore();
  }

  function handlePointerMove(e: PointerEvent) {
    if (!containerEl || snapshots.length < 2) return;
    const rect = containerEl.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const plotW = Math.max(rect.width - PAD_LEFT - PAD_RIGHT, 1);

    if (x >= PAD_LEFT && x <= rect.width - PAD_RIGHT) {
      const ratio = (x - PAD_LEFT) / plotW;
      const idx = Math.round(ratio * (snapshots.length - 1));
      hoveredIndex = Math.max(0, Math.min(snapshots.length - 1, idx));
      isHovering = true;
    } else {
      isHovering = false;
      hoveredIndex = -1;
    }
  }

  function handlePointerLeave() {
    isHovering = false;
    hoveredIndex = -1;
  }

  function resize() {
    if (!containerEl || !canvasEl) return;
    const rect = containerEl.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    const w = Math.max(rect.width, 280);
    viewW = w;
    canvasEl.width = Math.round(w * dpr);
    canvasEl.height = Math.round(height * dpr);
    draw();
  }

  onMount(() => {
    ctx = canvasEl.getContext('2d');
    resize();
    const ro = new ResizeObserver(() => resize());
    ro.observe(containerEl);
    // Pantau perubahan tema gelap/terang agar warna kanvas langsung menyesuaikan
    const mo = new MutationObserver(() => {
      themeVersion++;
    });
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme', 'data-accent', 'class'] });
    return () => {
      ro.disconnect();
      mo.disconnect();
    };
  });
</script>

<div
  class="cmp-history-chart chart-container"
  bind:this={containerEl}
  onpointermove={handlePointerMove}
  onpointerleave={handlePointerLeave}
  role="region"
  aria-label="Grafik rekaman histori metrik server"
>
  <canvas bind:this={canvasEl} style="--hc-h: {height}px;"></canvas>

  {#if activeHover}
    {@const s = activeHover.snap}
    <div class="hc-crosshair" style="--hc-x: {activeHover.x}px; --hc-top: {PAD_TOP}px; --hc-bottom: {PAD_BOTTOM}px;"></div>
    <div class="hover-tooltip" style="--tip-x: {activeHover.tipX}px;">
      <div class="tip-time mono">{timeFormatter.format(new Date(s.timestamp * 1000))}</div>
      {#if mode === 'resource'}
        <div class="tip-row"><span class="tip-dot tip-dot--info"></span> CPU: <b>{s.cpu.toFixed(1)}%</b></div>
        <div class="tip-row"><span class="tip-dot tip-dot--warning"></span> IOWait: <b>{(s.cpu_iowait || 0).toFixed(1)}%</b></div>
        <div class="tip-row">
          <span class="tip-dot tip-dot--violet"></span> RAM:
          <b>{s.ram_total > 0 ? ((s.ram_used / s.ram_total) * 100).toFixed(1) : 0}%</b> ({formatBytes(s.ram_used)})
        </div>
      {:else if mode === 'load'}
        <div class="tip-row"><span class="tip-dot tip-dot--info"></span> 1 Menit: <b>{s.load_1m.toFixed(2)}</b></div>
        <div class="tip-row"><span class="tip-dot tip-dot--violet"></span> 5 Menit: <b>{s.load_5m.toFixed(2)}</b></div>
        <div class="tip-row"><span class="tip-dot tip-dot--accent"></span> 15 Menit: <b>{s.load_15m.toFixed(2)}</b></div>
      {:else if mode === 'network'}
        <div class="tip-row"><span class="tip-dot tip-dot--accent"></span> Masuk: <b>{formatBytes(s.net_in)}/s</b></div>
        <div class="tip-row"><span class="tip-dot tip-dot--warning"></span> Keluar: <b>{formatBytes(s.net_out)}/s</b></div>
      {:else if mode === 'disk_io'}
        <div class="tip-row"><span class="tip-dot tip-dot--success"></span> Baca: <b>{formatBytes(s.disk_read || 0)}/s</b></div>
        <div class="tip-row"><span class="tip-dot tip-dot--warning"></span> Tulis: <b>{formatBytes(s.disk_write || 0)}/s</b></div>
      {:else if mode === 'disk_space'}
        <div class="tip-row">
          <span class="tip-dot tip-dot--accent"></span> Root Disk:
          <b>{s.disk_total > 0 ? ((s.disk_used / s.disk_total) * 100).toFixed(1) : 0}%</b>
        </div>
        <div class="tip-row mono">{formatBytes(s.disk_used)} / {formatBytes(s.disk_total)}</div>
      {/if}
    </div>
  {/if}
</div>
