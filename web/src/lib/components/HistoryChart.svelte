<script lang="ts">
  import { onMount } from 'svelte';
  import type { MonitorSnapshot } from '../types';
  import { readToken } from '../utils/browser';
  import '../../styles/pages/components/history-chart.css';

  interface Props {
    snapshots: MonitorSnapshot[];
    height?: number;
    mode?: 'load' | 'resource' | 'network';
  }

  let { snapshots = [], height = 260, mode = 'load' }: Props = $props();

  let canvasEl: HTMLCanvasElement;
  let containerEl: HTMLDivElement;
  let ctx: CanvasRenderingContext2D | null = null;
  let viewW = $state(800);

  // State interaksi hover
  let isHovering = $state(false);
  let hoveredIndex = $state(-1);
  let mouseX = $state(0);

  const timeFormatter = new Intl.DateTimeFormat('id-ID', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });

  function formatBytes(bytes: number): string {
    if (!bytes) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  $effect(() => {
    // Gambar ulang setiap kali snapshots, mode, atau viewW berubah
    const _snaps = snapshots;
    const _m = mode;
    const _w = viewW;
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
    const colorAccent = readToken('--accent', 'teal');
    const colorWarning = readToken('--warning', 'orange');
    const fontMono = readToken('--font-mono', 'monospace');

    ctx.save();
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, w, h);

    const padLeft = 45;
    const padRight = 20;
    const padTop = 20;
    const padBottom = 30;

    const plotW = w - padLeft - padRight;
    const plotH = h - padTop - padBottom;

    // Grid garis horizontal & label
    ctx.strokeStyle = colorMuted;
    ctx.lineWidth = 1;
    ctx.fillStyle = colorMuted;
    ctx.font = `10px ${fontMono}`;
    ctx.textAlign = 'right';

    // Cari nilai maksimum sumbu Y
    let maxY = 1.0;
    if (mode === 'load') {
      for (const s of snapshots) {
        if (s.load_1m > maxY) maxY = s.load_1m;
        if (s.load_5m > maxY) maxY = s.load_5m;
        if (s.load_15m > maxY) maxY = s.load_15m;
      }
      maxY = Math.max(maxY * 1.2, 1.0);
    } else if (mode === 'resource') {
      maxY = 100; // Persen CPU & RAM
    } else if (mode === 'network') {
      for (const s of snapshots) {
        if (s.net_in > maxY) maxY = s.net_in;
        if (s.net_out > maxY) maxY = s.net_out;
      }
      maxY = Math.max(maxY * 1.2, 1024);
    }

    const steps = 4;
    for (let i = 0; i <= steps; i++) {
      const y = padTop + (plotH / steps) * i;
      const val = maxY - (maxY / steps) * i;

      ctx.globalAlpha = 0.14;
      ctx.beginPath();
      ctx.moveTo(padLeft, y);
      ctx.lineTo(padLeft + plotW, y);
      ctx.stroke();

      let label = val.toFixed(1);
      if (mode === 'resource') label = val.toFixed(0) + '%';
      else if (mode === 'network') label = formatBytes(val) + '/s';
      ctx.globalAlpha = 0.7;
      ctx.fillText(label, padLeft - 6, y + 3);
    }

    if (snapshots.length < 2) {
      ctx.globalAlpha = 0.6;
      ctx.textAlign = 'center';
      ctx.fillText('Belum ada data rekaman histori pada rentang waktu ini.', padLeft + plotW / 2, padTop + plotH / 2);
      ctx.restore();
      return;
    }

    // Helper kalkulasi koordinat titik
    const getX = (idx: number) => padLeft + (idx / (snapshots.length - 1)) * plotW;
    const getY = (val: number) => padTop + plotH - (Math.min(val, maxY) / maxY) * plotH;

    // Garis waktu bawah (X-axis ticks)
    ctx.globalAlpha = 0.7;
    ctx.textAlign = 'center';
    const xStep = Math.max(Math.floor(snapshots.length / 4), 1);
    for (let i = 0; i < snapshots.length; i += xStep) {
      const x = getX(i);
      const s = snapshots[i];
      const timeStr = timeFormatter.format(new Date(s.timestamp * 1000));
      ctx.fillText(timeStr, x, h - 10);
    }
    ctx.globalAlpha = 1;

    // Render kurva berdasarkan mode
    if (mode === 'load') {
      drawLine(snapshots.map((s) => s.load_1m), colorInfo, 2);
      drawLine(snapshots.map((s) => s.load_5m), colorViolet, 1.5);
      drawLine(snapshots.map((s) => s.load_15m), colorAccent, 1.5);
    } else if (mode === 'resource') {
      drawLine(snapshots.map((s) => s.cpu), colorInfo, 2);
      drawLine(snapshots.map((s) => (s.ram_total > 0 ? (s.ram_used / s.ram_total) * 100 : 0)), colorViolet, 2);
    } else if (mode === 'network') {
      drawLine(snapshots.map((s) => s.net_in), colorAccent, 2);
      drawLine(snapshots.map((s) => s.net_out), colorWarning, 2);
    }

    function drawLine(vals: number[], color: string, lineWidth: number) {
      if (!ctx) return;
      ctx.beginPath();
      ctx.strokeStyle = color;
      ctx.lineWidth = lineWidth;
      ctx.lineJoin = 'round';
      ctx.lineCap = 'round';

      for (let i = 0; i < vals.length; i++) {
        const x = getX(i);
        const y = getY(vals[i]);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.stroke();
    }

    // Gambar indikator hover
    if (isHovering && hoveredIndex >= 0 && hoveredIndex < snapshots.length) {
      const x = getX(hoveredIndex);
      ctx.globalAlpha = 0.45;
      ctx.beginPath();
      ctx.strokeStyle = colorMuted;
      ctx.setLineDash([4, 4]);
      ctx.moveTo(x, padTop);
      ctx.lineTo(x, padTop + plotH);
      ctx.stroke();
      ctx.setLineDash([]);
      ctx.globalAlpha = 1;
    }

    ctx.restore();
  }

  function handleMouseMove(e: MouseEvent) {
    if (!containerEl || snapshots.length < 2) return;
    const rect = containerEl.getBoundingClientRect();
    const x = e.clientX - rect.left;
    mouseX = x;

    const padLeft = 45;
    const padRight = 20;
    const plotW = rect.width - padLeft - padRight;

    if (x >= padLeft && x <= rect.width - padRight) {
      const ratio = (x - padLeft) / plotW;
      hoveredIndex = Math.round(ratio * (snapshots.length - 1));
      isHovering = true;
    } else {
      isHovering = false;
      hoveredIndex = -1;
    }
    draw();
  }

  function handleMouseLeave() {
    isHovering = false;
    hoveredIndex = -1;
    draw();
  }

  function resize() {
    if (!containerEl || !canvasEl) return;
    const rect = containerEl.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    viewW = rect.width;
    canvasEl.width = rect.width * dpr;
    canvasEl.height = height * dpr;
    draw();
  }

  onMount(() => {
    ctx = canvasEl.getContext('2d');
    resize();
    window.addEventListener('resize', resize);
    return () => window.removeEventListener('resize', resize);
  });
</script>

<div
  class="cmp-history-chart chart-container"
  bind:this={containerEl}
  onmousemove={handleMouseMove}
  onmouseleave={handleMouseLeave}
  role="presentation"
>
  <canvas bind:this={canvasEl} style="--hc-h: {height}px;"></canvas>

  {#if isHovering && hoveredIndex >= 0 && hoveredIndex < snapshots.length}
    {@const s = snapshots[hoveredIndex]}
    <div class="hover-tooltip" style="--tip-x: {Math.min(Math.max(mouseX, 120), viewW - 140)}px;">
      <div class="tip-time mono">{timeFormatter.format(new Date(s.timestamp * 1000))}</div>
      {#if mode === 'load'}
        <div class="tip-row"><span class="tip-dot tip-dot--info"></span> 1 Menit: <b>{s.load_1m.toFixed(2)}</b></div>
        <div class="tip-row"><span class="tip-dot tip-dot--violet"></span> 5 Menit: <b>{s.load_5m.toFixed(2)}</b></div>
        <div class="tip-row"><span class="tip-dot tip-dot--accent"></span> 15 Menit: <b>{s.load_15m.toFixed(2)}</b></div>
      {:else if mode === 'resource'}
        <div class="tip-row"><span class="tip-dot tip-dot--info"></span> CPU: <b>{s.cpu.toFixed(1)}%</b></div>
        <div class="tip-row"><span class="tip-dot tip-dot--violet"></span> RAM: <b>{s.ram_total > 0 ? ((s.ram_used / s.ram_total) * 100).toFixed(1) : 0}%</b> ({formatBytes(s.ram_used)})</div>
      {:else if mode === 'network'}
        <div class="tip-row"><span class="tip-dot tip-dot--accent"></span> Masuk: <b>{formatBytes(s.net_in)}/s</b></div>
        <div class="tip-row"><span class="tip-dot tip-dot--warning"></span> Keluar: <b>{formatBytes(s.net_out)}/s</b></div>
      {/if}
    </div>
  {/if}
</div>
