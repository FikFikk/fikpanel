<script lang="ts">
  import { onMount } from 'svelte';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import { SearchAddon } from '@xterm/addon-search';
  import { WebLinksAddon } from '@xterm/addon-web-links';
  import { WebglAddon } from '@xterm/addon-webgl';
  import { copyText, openExternal, readToken } from '../utils/browser';
  import { toast } from '../stores/toast.svelte';

  interface Props {
    serverId: number;
    active: boolean;
    onStatusChange?: (
      status: 'connecting' | 'connected' | 'disconnected' | 'error',
      meta?: {
        version?: string;
        fingerprint?: string;
        error?: string;
        hostKeyMismatch?: boolean;
      },
    ) => void;
    onCommandLine?: (cmd: string) => void;
    onRequestSearch?: () => void;
    onRequestHistory?: () => void;
    onRequestResetHostKey?: () => void;
  }

  let {
    serverId,
    active,
    onStatusChange,
    onCommandLine,
    onRequestSearch,
    onRequestHistory,
    onRequestResetHostKey,
  }: Props = $props();

  let hostEl = $state<HTMLDivElement | null>(null);
  let status = $state<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting');
  let disconnectMsg = $state('');
  let hostKeyMismatch = $state(false);

  let term: Terminal | null = null;
  let fitAddon: FitAddon | null = null;
  let searchAddon: SearchAddon | null = null;
  let webglAddon: WebglAddon | null = null;
  let ws: WebSocket | null = null;
  let resizeObserver: ResizeObserver | null = null;
  let themeObserver: MutationObserver | null = null;
  let lineBuffer = '';
  let destroyed = false;

  // Bangun tema xterm.js secara dinamis dari CSS design tokens agar konsisten di mode gelap & terang
  function buildXtermTheme() {
    const bg = readToken('--bg-inset', '#0b0d14');
    const fg = readToken('--text', '#f1f5f9');
    const muted = readToken('--text-muted', '#94a3b8');
    const subtle = readToken('--text-subtle', '#64748b');
    const accent = readToken('--accent', '#ff5a2b');
    const success = readToken('--success', '#10b981');
    const successStrong = readToken('--success-strong', '#34d399');
    const warning = readToken('--warning', '#f59e0b');
    const warningStrong = readToken('--warning-strong', '#fbbf24');
    const danger = readToken('--danger', '#f43f5e');
    const dangerStrong = readToken('--danger-strong', '#fb7185');
    const info = readToken('--info', '#38bdf8');
    const violet = readToken('--violet', '#818cf8');
    const activeBg = readToken('--bg-active', '#1e2433');

    return {
      background: bg,
      foreground: fg,
      cursor: accent,
      cursorAccent: bg,
      selectionBackground: activeBg,
      selectionForeground: fg,
      black: bg,
      red: danger,
      green: success,
      yellow: warning,
      blue: info,
      magenta: violet,
      cyan: info,
      white: muted,
      brightBlack: subtle,
      brightRed: dangerStrong,
      brightGreen: successStrong,
      brightYellow: warningStrong,
      brightBlue: info,
      brightMagenta: violet,
      brightCyan: info,
      brightWhite: fg,
    };
  }

  function updateStatus(
    next: 'connecting' | 'connected' | 'disconnected' | 'error',
    meta?: { version?: string; fingerprint?: string; error?: string; hostKeyMismatch?: boolean },
  ) {
    status = next;
    if (meta?.error) disconnectMsg = meta.error;
    if (meta?.hostKeyMismatch !== undefined) hostKeyMismatch = meta.hostKeyMismatch;
    onStatusChange?.(next, meta);
  }

  function sendResize() {
    if (!term || !ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
  }

  function connectWS() {
    if (destroyed || !term) return;
    if (ws) {
      try {
        ws.close();
      } catch {
        // Abaikan error saat menutup socket lama
      }
      ws = null;
    }

    disconnectMsg = '';
    hostKeyMismatch = false;
    updateStatus('connecting');

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const cols = term.cols || 120;
    const rows = term.rows || 32;
    const url = `${proto}//${window.location.host}/api/terminal/ws?server_id=${serverId}&cols=${cols}&rows=${rows}`;

    const socket = new WebSocket(url);
    socket.binaryType = 'arraybuffer';
    ws = socket;

    socket.onopen = () => {
      if (ws !== socket) return;
      updateStatus('connected');
      fit();
      sendResize();
    };

    socket.onmessage = (ev) => {
      if (ws !== socket || !term) return;
      if (ev.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(ev.data));
        return;
      }
      if (typeof ev.data === 'string') {
        try {
          const msg = JSON.parse(ev.data);
          if (msg.type === 'meta') {
            updateStatus('connected', {
              version: msg.version,
              fingerprint: msg.fingerprint,
            });
          } else if (msg.type === 'exit') {
            const code = typeof msg.code === 'number' ? msg.code : 0;
            disconnectMsg = `Sesi shell berakhir (exit code ${code})`;
            updateStatus('disconnected', { error: disconnectMsg });
          } else if (msg.type === 'error') {
            disconnectMsg = msg.message || 'Koneksi terminal gagal';
            hostKeyMismatch = Boolean(msg.host_key_mismatch);
            term.writeln(`\r\n\x1b[31m[Error] ${disconnectMsg}\x1b[0m`);
            updateStatus('error', {
              error: disconnectMsg,
              hostKeyMismatch,
            });
          }
        } catch {
          term.write(ev.data);
        }
      }
    };

    socket.onerror = () => {
      if (ws !== socket) return;
      if (!disconnectMsg) {
        disconnectMsg = 'Koneksi WebSocket PTY terputus';
      }
      updateStatus('error', { error: disconnectMsg, hostKeyMismatch });
    };

    socket.onclose = () => {
      if (ws !== socket) return;
      if (status !== 'error') {
        if (!disconnectMsg) disconnectMsg = 'Sesi terminal terputus';
        updateStatus('disconnected', { error: disconnectMsg });
      }
    };
  }

  // Lacak perintah satu baris yang diketik user untuk memperkaya riwayat persisten (Ctrl+R / History)
  function trackTypedInput(data: string) {
    for (const ch of data) {
      if (ch === '\r' || ch === '\n') {
        const cleaned = lineBuffer.trim();
        if (cleaned.length > 0 && cleaned.length < 400) {
          onCommandLine?.(cleaned);
        }
        lineBuffer = '';
      } else if (ch === '\x7f' || ch === '\b') {
        lineBuffer = lineBuffer.slice(0, -1);
      } else if (ch === '\x03' || ch === '\x15') {
        lineBuffer = '';
      } else if (ch >= ' ' && ch <= '~') {
        lineBuffer += ch;
      }
    }
  }

  onMount(() => {
    if (!hostEl) return;

    const fontFamily = readToken('--font-mono', 'Fira Code, ui-monospace, monospace');
    const instance = new Terminal({
      cursorBlink: true,
      cursorStyle: 'block',
      fontFamily,
      fontSize: 13,
      lineHeight: 1.38,
      scrollback: 5000,
      convertEol: false,
      allowProposedApi: true,
      rightClickSelectsWord: false,
      theme: buildXtermTheme(),
    });

    const fitInstance = new FitAddon();
    const searchInstance = new SearchAddon();
    const linksInstance = new WebLinksAddon((_event, uri) => {
      openExternal(uri);
    });

    instance.loadAddon(fitInstance);
    instance.loadAddon(searchInstance);
    instance.loadAddon(linksInstance);
    instance.open(hostEl);

    // Aktifkan akselerasi GPU WebGL untuk rendering 60 FPS; fallback otomatis ke renderer DOM bila tidak didukung
    try {
      const gl = new WebglAddon();
      gl.onContextLoss(() => {
        try {
          gl.dispose();
        } catch {
          // Abaikan error saat melepas konteks WebGL yang hilang
        }
        webglAddon = null;
      });
      instance.loadAddon(gl);
      webglAddon = gl;
    } catch {
      // Browser/GPU tidak mendukung WebGL2 — tetap memakai renderer bawaan xterm.js
      webglAddon = null;
    }

    term = instance;
    fitAddon = fitInstance;
    searchAddon = searchInstance;

    // Tangani shortcut copy/paste (Ctrl+C copy jika ada teks diblok, SIGINT jika kosong), Ctrl+Shift+F, Alt+R
    instance.attachCustomKeyEventHandler((e: KeyboardEvent) => {
      if (e.type !== 'keydown') return true;

      // Ctrl+C: salin teks jika ada seleksi, kirim SIGINT asli ke PTY jika tidak ada seleksi
      if (e.ctrlKey && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'c') {
        if (instance.hasSelection()) {
          const selected = instance.getSelection();
          if (selected) {
            copyText(selected)
              .then(() => toast.info('Teks terpilih disalin'))
              .catch(() => toast.error('Gagal menyalin teks'));
            instance.clearSelection();
          }
          return false;
        }
        return true;
      }

      // Ctrl+Shift+C: selalu salin seleksi teks
      if (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === 'c') {
        const selected = instance.getSelection();
        if (selected) {
          copyText(selected)
            .then(() => toast.info('Teks terpilih disalin'))
            .catch(() => toast.error('Gagal menyalin teks'));
        }
        return false;
      }

      // Ctrl+Shift+F: buka bilah pencarian scrollback terminal
      if (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === 'f') {
        e.preventDefault();
        onRequestSearch?.();
        return false;
      }

      // Alt+R: buka pencari riwayat perintah persisten (tanpa menimpa Ctrl+R asli bash)
      if (e.altKey && !e.ctrlKey && e.key.toLowerCase() === 'r') {
        e.preventDefault();
        onRequestHistory?.();
        return false;
      }

      return true;
    });

    const encoder = new TextEncoder();
    instance.onData((data) => {
      if (!ws || ws.readyState !== WebSocket.OPEN) return;
      trackTypedInput(data);
      ws.send(encoder.encode(data));
    });

    instance.onResize(({ cols, rows }) => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols, rows }));
      }
    });

    // Sesuaikan ukuran kolom & baris PTY secara otomatis setiap ukuran kontainer berubah
    resizeObserver = new ResizeObserver(() => {
      if (active) {
        fit();
      }
    });
    resizeObserver.observe(hostEl);

    // Sinkronkan palet warna xterm saat pengguna mengganti tema, aksen palet, atau nuansa latar di Settings
    themeObserver = new MutationObserver(() => {
      if (term) {
        term.options.theme = buildXtermTheme();
      }
    });
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme', 'data-accent', 'data-surface', 'style'],
    });

    requestAnimationFrame(() => {
      fit();
      connectWS();
      if (active) instance.focus();
    });

    // Hitung ulang dimensi cell setelah webfont monospace selesai dimuat agar jumlah baris PTY selalu presisi
    if (typeof document !== 'undefined' && document.fonts?.ready) {
      document.fonts.ready.then(() => {
        if (!destroyed && active) {
          fit();
        }
      });
    }

    return () => {
      destroyed = true;
      resizeObserver?.disconnect();
      themeObserver?.disconnect();
      if (ws) {
        try {
          ws.close();
        } catch {
          // Abaikan error saat menutup socket
        }
        ws = null;
      }
      try {
        webglAddon?.dispose();
      } catch {
        // Abaikan error pelepasan addon WebGL saat unmount
      }
      webglAddon = null;
      instance.dispose();
      term = null;
      fitAddon = null;
      searchAddon = null;
    };
  });

  $effect(() => {
    if (active && term) {
      requestAnimationFrame(() => {
        fit();
        term?.focus();
      });
    }
  });

  // ── API publik komponen yang dipanggil oleh halaman induk Terminal.svelte ──

  export function fit() {
    if (!fitAddon || !term || !hostEl || hostEl.clientWidth === 0 || hostEl.clientHeight === 0) return;
    try {
      const buf = term.buffer.active;
      const atBottom = buf.viewportY >= buf.baseY;
      fitAddon.fit();
      if (atBottom) {
        term.scrollToBottom();
      }
      sendResize();
    } catch {
      // Abaikan fit saat elemen belum siap diukur
    }
  }

  export function focus() {
    term?.focus();
  }

  export function sendInput(text: string) {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      toast.warning('Sesi PTY belum terhubung');
      return;
    }
    ws.send(new TextEncoder().encode(text));
    term?.focus();
  }

  export function runCommand(cmd: string) {
    const clean = cmd.trim();
    if (!clean) return;
    onCommandLine?.(clean);
    sendInput(clean + '\r');
  }

  export function clear() {
    term?.clear();
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(new TextEncoder().encode('\x0c'));
    }
    term?.focus();
  }

  export function reconnect() {
    term?.clear();
    connectWS();
    term?.focus();
  }

  export function findNext(query: string, opts?: { caseSensitive?: boolean; regex?: boolean }): boolean {
    if (!searchAddon || !query) return false;
    return searchAddon.findNext(query, {
      caseSensitive: opts?.caseSensitive ?? false,
      regex: opts?.regex ?? false,
      incremental: false,
    });
  }

  export function findPrevious(query: string, opts?: { caseSensitive?: boolean; regex?: boolean }): boolean {
    if (!searchAddon || !query) return false;
    return searchAddon.findPrevious(query, {
      caseSensitive: opts?.caseSensitive ?? false,
      regex: opts?.regex ?? false,
    });
  }

  export function clearSearch() {
    searchAddon?.clearDecorations();
  }

  export function exportLog(): string {
    if (!term) return '';
    const buf = term.buffer.active;
    const lines: string[] = [];
    for (let i = 0; i < buf.length; i++) {
      const line = buf.getLine(i);
      if (line) {
        lines.push(line.translateToString(true));
      }
    }
    // Pangkas baris kosong berlebih di bagian paling bawah buffer
    while (lines.length > 0 && lines[lines.length - 1] === '') {
      lines.pop();
    }
    return lines.join('\n');
  }
</script>

<div class="xterm-shell">
  <div class="xterm-canvas-host" bind:this={hostEl}></div>

  {#if status === 'connecting'}
    <div class="xterm-status-bar">
      <span class="spinner spinner--sm"></span>
      <span>Menghubungkan sesi PTY interaktif...</span>
    </div>
  {:else if status === 'disconnected' || status === 'error'}
    <div class="xterm-status-bar" class:xterm-status-bar--error={status === 'error'}>
      <i class={status === 'error' ? 'ri-error-warning-line' : 'ri-plug-line'}></i>
      <span class="xterm-status-msg">{disconnectMsg || 'Sesi terminal terputus'}</span>
      <div class="xterm-status-actions">
        {#if hostKeyMismatch && serverId > 0 && onRequestResetHostKey}
          <button type="button" class="btn btn--danger btn--sm" onclick={onRequestResetHostKey}>
            <i class="ri-key-2-line"></i> Reset Host Key
          </button>
        {/if}
        <button type="button" class="btn btn--primary btn--sm" onclick={reconnect}>
          <i class="ri-refresh-line"></i> Reconnect
        </button>
      </div>
    </div>
  {/if}
</div>
