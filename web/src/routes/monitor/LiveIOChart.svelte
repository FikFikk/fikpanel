<script lang="ts">
  import { onMount } from 'svelte';
  import { formatBytes } from '../../lib/api';
  import type { MetricPoint } from '../../lib/types';
  import { readToken } from '../../lib/utils/browser';

  interface Props {
    history: MetricPoint[];
    mode: 'network' | 'disk_io';
    height?: number;
  }

  let { history = [], mode, height = 185 }: Props = $props();

  const MAX_POINTS = 60;
  let canvasEl: HTMLCanvasElement;
  let containerEl: HTMLDivElement;
  let ctx: CanvasRenderingContext2D | null = null;
  let viewW = $state(420);
  let themeVersion = $state(0);

  let visiblePoints = $derived(history.slice(-MAX_POINTS));

  let peakStats = $derived.by(() => {
    let maxA = 0;
    let maxB = 0;
    for (const p of visiblePoints) {
      const a = mode === 'network' ? p.net_in || 0 : p.disk_read || 0;
      const b = mode === 'network' ? p.net_out || 0 : p.disk_write || 0;
      if (a > maxA) maxA = a;
      if (b > maxB) maxB = b;
    }
    return { maxA, maxB };
  });

  function draw() {
    if (!canvasEl || !ctx) return;
    const dpr = window.devicePixelRatio || 1;
    const w = canvasEl.width / dpr;
    const h = canvasEl.height / dpr;

    const colorMuted = readToken('--text-muted', 'slategray');
    const colorA = mode === 'network' ? readToken('--accent', 'coral') : readToken('--success', 'mediumseagreen');
    const colorB = readToken('--warning', 'orange');
    const fontMono = readToken('--font-mono', 'monospace');

    const padLeft = 56;
    const padRight = 12;
    const padTop = 14;
    const padBottom = 20;
    const plotW = Math.max(w - padLeft - padRight, 1);
    const plotH = Math.max(h - padTop - padBottom, 1);

    ctx.save();
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, w, h);

    const maxY = Math.max(peakStats.maxA, peakStats.maxB, 1024) * 1.2;

    // Grid horizontal & label kecepatan
    ctx.strokeStyle = colorMuted;
    ctx.fillStyle = colorMuted;
    ctx.font = `10px ${fontMono}`;
    ctx.textAlign = 'right';
    ctx.lineWidth = 1;

    for (let i = 0; i <= 3; i++) {
      const y = Math.floor(padTop + (plotH / 3) * i) + 0.5;
      const val = maxY - (maxY / 3) * i;
      ctx.globalAlpha = 0.14;
      ctx.beginPath();
      ctx.moveTo(padLeft, y);
      ctx.lineTo(padLeft + plotW, y);
      ctx.stroke();

      ctx.globalAlpha = 0.7;
      ctx.fillText(`${formatBytes(val)}/s`, padLeft - 6, y + 3);
    }
    ctx.globalAlpha = 1;

    if (visiblePoints.length < 2) {
      ctx.globalAlpha = 0.6;
      ctx.textAlign = 'center';
      ctx.fillText('Menunggu aliran data SSE...', padLeft + plotW / 2, padTop + plotH / 2);
      ctx.restore();
      return;
    }

    const step = plotW / (MAX_POINTS - 1);
    const getX = (i: number) => padLeft + i * step;
    const getY = (val: number) => padTop + plotH - (Math.min(Math.max(val, 0), maxY) / maxY) * plotH;

    function drawTrace(vals: number[], color: string, alpha: number) {
      if (!ctx || vals.length < 2) return;
      ctx.beginPath();
      for (let i = 0; i < vals.length; i++) {
        const x = getX(i);
        const y = getY(vals[i]);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.strokeStyle = color;
      ctx.lineWidth = 1.8;
      ctx.lineJoin = 'round';
      ctx.lineCap = 'round';
      ctx.stroke();

      const grad = ctx.createLinearGradient(0, padTop, 0, padTop + plotH);
      grad.addColorStop(0, color);
      grad.addColorStop(1, 'transparent');
      ctx.lineTo(getX(vals.length - 1), padTop + plotH);
      ctx.lineTo(getX(0), padTop + plotH);
      ctx.closePath();
      ctx.save();
      ctx.globalAlpha = alpha;
      ctx.fillStyle = grad;
      ctx.fill();
      ctx.restore();
    }

    const seriesA = visiblePoints.map((p) => (mode === 'network' ? p.net_in || 0 : p.disk_read || 0));
    const seriesB = visiblePoints.map((p) => (mode === 'network' ? p.net_out || 0 : p.disk_write || 0));
    drawTrace(seriesB, colorB, 0.12);
    drawTrace(seriesA, colorA, 0.16);

    ctx.restore();
  }

  function resize() {
    if (!containerEl || !canvasEl) return;
    const rect = containerEl.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    const w = Math.max(rect.width, 240);
    viewW = w;
    canvasEl.width = Math.round(w * dpr);
    canvasEl.height = Math.round(height * dpr);
    draw();
  }

  $effect(() => {
    const _pts = visiblePoints;
    const _w = viewW;
    const _tv = themeVersion;
    draw();
  });

  onMount(() => {
    ctx = canvasEl.getContext('2d');
    resize();
    const ro = new ResizeObserver(() => resize());
    ro.observe(containerEl);
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

<div bind:this={containerEl}>
  <canvas
    bind:this={canvasEl}
    class="mon-live-io-canvas"
    style="--io-h: {height}px;"
    aria-label={mode === 'network' ? 'Grafik live kecepatan jaringan' : 'Grafik live kecepatan baca tulis disk'}
  ></canvas>
</div>
