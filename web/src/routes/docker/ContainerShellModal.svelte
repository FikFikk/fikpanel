<script lang="ts">
  // Subkomponen modal Terminal Interaktif Kontainer (`docker exec -it`) berbasis xterm.js + WebSocket PTY
  import '../../styles/pages/docker/container-shell-modal.css';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import { readToken } from '../../lib/utils/browser';

  interface Props {
    open: boolean;
    containerId: string;
    containerName: string;
    onClose: () => void;
  }

  let { open, containerId, containerName, onClose }: Props = $props();

  let hostEl = $state<HTMLDivElement | null>(null);
  let selectedShell = $state('auto');
  let connState = $state<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting');
  let statusMsg = $state('');

  let term: Terminal | null = null;
  let fitAddon: FitAddon | null = null;
  let ws: WebSocket | null = null;
  let resizeObserver: ResizeObserver | null = null;

  function buildTheme() {
    const bg = readToken('--bg-inset', '');
    const fg = readToken('--text', '');
    const accent = readToken('--accent', '');
    const success = readToken('--success', '');
    const warning = readToken('--warning', '');
    const danger = readToken('--danger', '');
    const info = readToken('--info', '');
    const activeBg = readToken('--bg-active', '');
    return {
      background: bg,
      foreground: fg,
      cursor: accent,
      cursorAccent: bg,
      selectionBackground: activeBg,
      red: danger,
      green: success,
      yellow: warning,
      blue: info,
      cyan: info,
    };
  }

  function cleanupSession() {
    resizeObserver?.disconnect();
    resizeObserver = null;
    if (ws) {
      try {
        ws.close();
      } catch {
        // Abaikan error penutupan socket lama
      }
      ws = null;
    }
    if (term) {
      try {
        term.dispose();
      } catch {
        // Abaikan error saat membersihkan instance xterm
      }
      term = null;
      fitAddon = null;
    }
  }

  function connectSocket() {
    if (!term || !containerId) return;
    if (ws) {
      try {
        ws.close();
      } catch {
        // Abaikan error penutupan socket lama
      }
      ws = null;
    }

    connState = 'connecting';
    statusMsg = '';

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const cols = term.cols || 110;
    const rows = term.rows || 28;
    const url = `${proto}//${window.location.host}/api/docker/containers/${encodeURIComponent(containerId)}/exec/ws?shell=${encodeURIComponent(selectedShell)}&cols=${cols}&rows=${rows}`;

    const socket = new WebSocket(url);
    socket.binaryType = 'arraybuffer';
    ws = socket;

    socket.onopen = () => {
      if (ws !== socket) return;
      connState = 'connected';
      fitAddon?.fit();
      if (term) {
        socket.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
        term.focus();
      }
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
          if (msg.type === 'error') {
            connState = 'error';
            statusMsg = msg.message || 'Gagal membuka shell di dalam kontainer';
            term.writeln(`\r\n[Error] ${statusMsg}`);
          } else if (msg.type === 'exit') {
            connState = 'disconnected';
            statusMsg = `Sesi shell kontainer berakhir (exit code ${msg.code ?? 0})`;
          }
        } catch {
          term.write(ev.data);
        }
      }
    };

    socket.onerror = () => {
      if (ws !== socket) return;
      connState = 'error';
      if (!statusMsg) statusMsg = 'Koneksi WebSocket exec kontainer terputus';
    };

    socket.onclose = () => {
      if (ws !== socket) return;
      if (connState !== 'error') {
        connState = 'disconnected';
        if (!statusMsg) statusMsg = 'Sesi shell kontainer ditutup';
      }
    };
  }

  function mountTerminal() {
    cleanupSession();
    if (!hostEl || !containerId) return;

    const fontFamily = readToken('--font-mono', 'ui-monospace, monospace');
    const instance = new Terminal({
      cursorBlink: true,
      fontFamily,
      fontSize: 13,
      lineHeight: 1.38,
      scrollback: 3000,
      theme: buildTheme(),
    });
    const fit = new FitAddon();
    instance.loadAddon(fit);
    instance.open(hostEl);

    term = instance;
    fitAddon = fit;

    const encoder = new TextEncoder();
    instance.onData((data) => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(encoder.encode(data));
      }
    });
    instance.onResize(({ cols, rows }) => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols, rows }));
      }
    });

    resizeObserver = new ResizeObserver(() => {
      try {
        fit.fit();
      } catch {
        // Abaikan error fit sementara
      }
    });
    resizeObserver.observe(hostEl);

    requestAnimationFrame(() => {
      try {
        fit.fit();
      } catch {
        // Abaikan error fit awal
      }
      connectSocket();
    });
  }

  $effect(() => {
    if (open && hostEl && containerId) {
      mountTerminal();
      return () => cleanupSession();
    } else {
      cleanupSession();
    }
  });

  function handleReconnect() {
    term?.clear();
    connectSocket();
  }

  function handleClear() {
    term?.clear();
    term?.focus();
  }
</script>

<Modal
  {open}
  size="xl"
  onclose={onClose}
>
  {#snippet header()}
    <div class="flex-row items-center gap-2 flex-1">
      <i class="ri-terminal-box-line text-accent" aria-hidden="true"></i>
      <h2 class="modal__title">Terminal Kontainer: {containerName}</h2>
      <span class="badge badge--secondary font-mono text-2xs">{containerId.slice(0, 12)}</span>
    </div>
  {/snippet}

  <div class="pg-docker-shell-modal">
    <div class="shell-toolbar">
      <div class="shell-toolbar-left">
        {#if connState === 'connected'}
          <span class="badge badge--success">Terhubung</span>
        {:else if connState === 'connecting'}
          <span class="badge badge--warning">Menghubungkan...</span>
        {:else}
          <span class="badge badge--danger">Terputus</span>
        {/if}

        <select
          class="input input--sm"
          aria-label="Pilih shell kontainer"
          bind:value={selectedShell}
          onchange={handleReconnect}
        >
          <option value="auto">Auto (bash / ash / sh)</option>
          <option value="/bin/bash">/bin/bash</option>
          <option value="/bin/sh">/bin/sh</option>
          <option value="/bin/ash">/bin/ash (Alpine)</option>
        </select>
      </div>

      <div class="shell-toolbar-right">
        <button type="button" class="btn btn--secondary btn--sm" onclick={handleClear}>
          <i class="ri-eraser-line" aria-hidden="true"></i>
          <span>Bersihkan</span>
        </button>
        <button type="button" class="btn btn--secondary btn--sm" onclick={handleReconnect}>
          <i class="ri-refresh-line" aria-hidden="true"></i>
          <span>Sambung Ulang</span>
        </button>
      </div>
    </div>

    <div class="shell-viewport">
      <div class="shell-host" bind:this={hostEl}></div>

      {#if connState === 'disconnected' || connState === 'error'}
        <div class="shell-banner {connState === 'error' ? 'shell-banner--error' : ''}">
          <span>{statusMsg || 'Sesi terminal berakhir'}</span>
          <button type="button" class="btn btn--primary btn--sm" onclick={handleReconnect}>
            <i class="ri-refresh-line" aria-hidden="true"></i>
            <span>Hubungkan Kembali</span>
          </button>
        </div>
      {/if}
    </div>
  </div>

  {#snippet footer()}
    <span class="text-xs text-muted flex-1">
      Eksekusi langsung via <code class="font-mono">docker exec -it</code> dengan PTY asli (mendukung top, htop, nano, bash completion).
    </span>
    <button type="button" class="btn btn--secondary btn--sm" onclick={onClose}>
      Tutup
    </button>
  {/snippet}
</Modal>
