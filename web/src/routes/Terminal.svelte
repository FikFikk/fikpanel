<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { apiFetch, apiStreamSSE } from '../lib/api';
  import { copyText } from '../lib/utils/browser';
  import { parseAnsiSegments, splitHighlightMatches, stripAnsi } from '../lib/utils/ansi';
  import {
    clearTerminalHistoryStore,
    downloadTextFile,
    extractCommandVariables,
    hasCommandVariables,
    interpolateCommand,
    loadTerminalHistory,
    recordTerminalHistory,
    type CommandTemplateVar,
  } from '../lib/utils/terminalHelpers';
  import { toast } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import { t } from '../lib/stores/i18n.svelte';
  import type {
    TerminalServer,
    TerminalCommand,
    TerminalTab,
    TerminalHistoryItem,
    TerminalCompletionItem,
    TerminalMode,
  } from '../lib/types';
  import Modal from '../lib/components/ui/Modal.svelte';
  import XtermConsole from '../lib/components/XtermConsole.svelte';
  import '../styles/pages/terminal.css';

  // State auto-complete Tab & Tab 2x (mode Console)
  let completionMatches = $state<TerminalCompletionItem[]>([]);
  let tabPressCount = $state(0);
  let lastTabCommand = $state('');
  let completing = $state(false);

  // Versi panel dinamis dari backend (menghindari hardcode versi)
  let panelVersion = $state('');

  // State umpan balik salin teks per blok output (mode Console)
  let copiedEntryKey = $state<string | null>(null);
  let copyFeedbackTimer: ReturnType<typeof setTimeout> | null = null;

  // Koordinat awal pointer di layar Console untuk membedakan klik biasa vs drag seleksi teks
  let viewportPointerDown = { x: 0, y: 0 };

  // Peta AbortController per tab untuk menghentikan paksa proses SSE saat Ctrl+C / tombol Stop
  const activeStreamControllers = new Map<string, AbortController>();

  // Referensi instance XtermConsole per tab (tetap hidup saat berpindah tab)
  let xtermRefs: Record<string, ReturnType<typeof XtermConsole> | undefined> = {};

  // State daftar server dan perintah
  let servers = $state<TerminalServer[]>([]);
  let commands = $state<TerminalCommand[]>([]);
  let loadingServers = $state(false);
  let loadingCommands = $state(false);

  // State sidebar kanan (Server List & Commands)
  let sidebarOpen = $state(true);
  let activeSidebarTab = $state<'servers' | 'commands'>('commands');
  let commandSearch = $state('');

  // State layar penuh (fullscreen mode)
  let isFullscreen = $state(false);
  let terminalBoxEl: HTMLDivElement | null = null;
  let addTabWrapperEl: HTMLDivElement | null = null;

  // State pencarian output terminal (Ctrl+Shift+F) — mendukung PTY & Console
  let showSearchBar = $state(false);
  let terminalSearchQuery = $state('');
  let searchCaseSensitive = $state(false);
  let searchInputEl = $state<HTMLInputElement | null>(null);

  // State Reverse Search riwayat perintah persisten (Ctrl+R di Console / Alt+R di PTY)
  let showHistoryPicker = $state(false);
  let historyPickerQuery = $state('');
  let historyPickerIndex = $state(0);
  let historyInputEl = $state<HTMLInputElement | null>(null);
  let persistedHistory = $state<string[]>([]);

  // State modal pengisian variabel dinamis command ({{var:default}})
  let showVarModal = $state(false);
  let pendingTemplateCmd = $state<TerminalCommand | null>(null);
  let pendingTemplateAction = $state<'run' | 'paste'>('run');
  let pendingTemplateVars = $state<CommandTemplateVar[]>([]);
  let pendingVarValues = $state<Record<string, string>>({});

  // State multi-tab terminal (default mode PTY interaktif, mulai langsung di '~')
  let tabs = $state<TerminalTab[]>([
    {
      id: 'local-1',
      serverId: 0,
      name: 'Local server',
      host: 'fikpanel',
      username: 'root',
      cwd: '~',
      mode: 'pty',
      connected: true,
      ptyStatus: 'connecting',
      commandInput: '',
      draftInput: '',
      historyIndex: -1,
      executing: false,
      history: [],
    },
  ]);
  let activeTabId = $state('local-1');

  // Tab aktif saat ini
  let activeTab = $derived(tabs.find((t) => t.id === activeTabId) ?? tabs[0]);

  // Kunci scope penyimpanan riwayat perintah per server
  function historyScopeKey(tab: TerminalTab | undefined): string {
    return tab && tab.serverId > 0 ? `srv-${tab.serverId}` : 'local';
  }

  // Muat ulang riwayat persisten saat tab aktif berganti
  $effect(() => {
    persistedHistory = loadTerminalHistory(historyScopeKey(activeTab));
  });

  // Daftar perintah tersaring di sidebar kanan
  let filteredCommands = $derived(
    commands.filter(
      (c) =>
        c.name.toLowerCase().includes(commandSearch.toLowerCase()) ||
        c.content.toLowerCase().includes(commandSearch.toLowerCase()),
    ),
  );

  // Daftar kandidat riwayat perintah terbalik (terbaru di atas) untuk overlay Ctrl+R / Alt+R
  let filteredHistoryItems = $derived.by(() => {
    const q = historyPickerQuery.trim().toLowerCase();
    const reversed = [...persistedHistory].reverse();
    if (!q) return reversed.slice(0, 40);
    return reversed.filter((cmd) => cmd.toLowerCase().includes(q)).slice(0, 40);
  });

  // Hitung total kecocokan pencarian di mode Console
  let consoleSearchMatchCount = $derived.by(() => {
    const q = terminalSearchQuery.trim();
    if (!q || !activeTab || activeTab.mode !== 'console') return 0;
    const needle = searchCaseSensitive ? q : q.toLowerCase();
    let total = 0;
    for (const item of activeTab.history) {
      const hayCmd = searchCaseSensitive ? item.command : item.command.toLowerCase();
      const hayOut = searchCaseSensitive ? stripAnsi(item.output) : stripAnsi(item.output).toLowerCase();
      total += countOccurrences(hayCmd, needle) + countOccurrences(hayOut, needle);
    }
    return total;
  });

  // Pratinjau perintah final setelah substitusi variabel {{var}}
  let resolvedTemplatePreview = $derived(
    pendingTemplateCmd ? interpolateCommand(pendingTemplateCmd.content, pendingVarValues) : '',
  );

  function countOccurrences(haystack: string, needle: string): number {
    if (!needle || !haystack) return 0;
    let count = 0;
    let pos = 0;
    while ((pos = haystack.indexOf(needle, pos)) !== -1) {
      count++;
      pos += needle.length;
    }
    return count;
  }

  // State popover Tambah Tab
  let showAddTabMenu = $state(false);

  // State Modal Add/Edit Server
  let showServerModal = $state(false);
  let editingServerId = $state<number | null>(null);
  let serverFormHost = $state('');
  let serverFormPort = $state(22);
  let serverFormUser = $state('root');
  let serverFormAuthType = $state<'password' | 'key'>('password');
  let serverFormPassword = $state('');
  let serverFormPrivateKey = $state('');
  let serverFormPassphrase = $state('');
  let serverFormRemarks = $state('');
  let showPasswordPlain = $state(false);
  let testingConnection = $state(false);
  let testConnResult = $state<{ ok: boolean; message: string; hostKeyMismatch?: boolean } | null>(null);
  let savingServer = $state(false);

  // State Modal Add/Edit Command
  let showCommandModal = $state(false);
  let editingCommandId = $state<number | null>(null);
  let commandFormName = $state('');
  let commandFormContent = $state('');
  let savingCommand = $state(false);

  // Status pengujian koneksi inline di daftar server
  let testingServerId = $state<number | null>(null);

  // Elemen scroll dan input Console mode
  let terminalScrollEl = $state<HTMLDivElement | null>(null);
  let inputPromptEl = $state<HTMLInputElement | null>(null);

  // State prompt interaktif untuk perintah CLI bertahap di mode Console
  let interactivePrompt = $state<{
    baseCmd: string;
    step: 'name' | 'email' | 'password';
    name: string;
    email: string;
  } | null>(null);

  onMount(() => {
    fetchServers();
    fetchCommands();
    if (tabs[0]) {
      initTabSession(tabs[0]);
    }

    const handleGlobalKeydown = (e: KeyboardEvent) => {
      // Pintasan Ctrl+Shift+F untuk membuka bilah pencarian output (PTY & Console)
      if (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === 'f') {
        e.preventDefault();
        openSearchBar();
        return;
      }

      // Pintasan Alt+R untuk membuka pencari riwayat perintah persisten (PTY & Console)
      if (e.altKey && !e.ctrlKey && e.key.toLowerCase() === 'r') {
        e.preventDefault();
        openHistoryPicker();
        return;
      }

      // Pintasan Ctrl+R khusus mode Console (pada mode PTY dibiarkan untuk reverse-i-search bash asli)
      if (e.ctrlKey && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'r') {
        if (activeTab?.mode === 'console' && !showServerModal && !showCommandModal && !showVarModal) {
          e.preventDefault();
          openHistoryPicker();
          return;
        }
      }

      // Pintasan Ctrl+C saat proses SSE sedang berjalan di mode Console -> langsung hentikan proses server
      if (
        e.ctrlKey &&
        !e.shiftKey &&
        e.key.toLowerCase() === 'c' &&
        activeTab?.mode === 'console' &&
        activeTab.executing
      ) {
        const selected = window.getSelection()?.toString() || '';
        if (!selected) {
          e.preventDefault();
          stopRunningStream(activeTab);
          return;
        }
      }

      // Pintasan Ctrl+Shift+C untuk menyalin seleksi teks di mode Console
      if (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === 'c' && activeTab?.mode === 'console') {
        const selected = window.getSelection()?.toString() || '';
        if (selected) {
          e.preventDefault();
          copyText(selected)
            .then(() => toast.info('Teks terpilih disalin ke clipboard'))
            .catch(() => toast.error('Gagal menyalin teks'));
        }
        return;
      }

      // Pintasan Ctrl+L untuk membersihkan layar di mode Console
      if (e.ctrlKey && !e.shiftKey && e.key.toLowerCase() === 'l' && activeTab?.mode === 'console') {
        e.preventDefault();
        clearActiveTerminal();
        return;
      }

      // Tombol Escape untuk menutup overlay/modal aktif
      if (e.key === 'Escape') {
        if (showSearchBar) {
          closeSearchBar();
          return;
        }
        if (showHistoryPicker) {
          showHistoryPicker = false;
          focusActiveTerminal();
          return;
        }
        if (showVarModal) {
          showVarModal = false;
          return;
        }
        if (showServerModal) showServerModal = false;
        if (showCommandModal) showCommandModal = false;
        if (showAddTabMenu) showAddTabMenu = false;
        return;
      }

      // Otomatis arahkan fokus ke input Console saat pengguna mengetik di halaman Terminal (khusus mode Console)
      if (
        activeTab?.mode === 'console' &&
        !showServerModal &&
        !showCommandModal &&
        !showVarModal &&
        !showSearchBar &&
        !showHistoryPicker &&
        inputPromptEl
      ) {
        const activeEl = document.activeElement;
        const isTypingInOtherField =
          activeEl &&
          activeEl !== inputPromptEl &&
          (activeEl.tagName === 'INPUT' ||
            activeEl.tagName === 'TEXTAREA' ||
            activeEl.tagName === 'SELECT' ||
            (activeEl as HTMLElement).isContentEditable);
        if (
          !isTypingInOtherField &&
          activeEl !== inputPromptEl &&
          !e.ctrlKey &&
          !e.altKey &&
          !e.metaKey &&
          (e.key.length === 1 || e.key === 'Backspace' || e.key === 'Tab' || e.key === 'ArrowUp' || e.key === 'ArrowDown')
        ) {
          inputPromptEl.focus();
        }
      }
    };

    // Tutup popover tambah tab saat klik di luar area menu
    const handleGlobalPointerDown = (e: PointerEvent) => {
      if (showAddTabMenu && addTabWrapperEl && !addTabWrapperEl.contains(e.target as Node)) {
        showAddTabMenu = false;
      }
    };

    // Sinkronkan state fullscreen jika pengguna keluar lewat tombol Esc browser
    const handleFullscreenChange = () => {
      isFullscreen = Boolean(document.fullscreenElement);
      refitActiveXterm();
    };

    window.addEventListener('keydown', handleGlobalKeydown);
    window.addEventListener('pointerdown', handleGlobalPointerDown);
    document.addEventListener('fullscreenchange', handleFullscreenChange);
    return () => {
      window.removeEventListener('keydown', handleGlobalKeydown);
      window.removeEventListener('pointerdown', handleGlobalPointerDown);
      document.removeEventListener('fullscreenchange', handleFullscreenChange);
      if (copyFeedbackTimer) clearTimeout(copyFeedbackTimer);
      for (const ctl of activeStreamControllers.values()) {
        ctl.abort();
      }
      activeStreamControllers.clear();
    };
  });

  // Ambil daftar server remote dari backend
  async function fetchServers() {
    loadingServers = true;
    try {
      const data = await apiFetch<TerminalServer[]>('/api/terminal/servers');
      servers = data || [];
    } catch (err: any) {
      toast.error(`Gagal memuat server: ${err.message || err}`);
    } finally {
      loadingServers = false;
    }
  }

  // Ambil daftar pintasan perintah dari backend
  async function fetchCommands() {
    loadingCommands = true;
    try {
      const data = await apiFetch<TerminalCommand[]>('/api/terminal/commands');
      commands = data || [];
    } catch (err: any) {
      toast.error(`Gagal memuat perintah: ${err.message || err}`);
    } finally {
      loadingCommands = false;
    }
  }

  // Gulir ke bawah pada layar Console
  async function scrollToBottom() {
    await tick();
    if (terminalScrollEl) {
      terminalScrollEl.scrollTop = terminalScrollEl.scrollHeight;
    }
  }

  // Fokus kembali ke input Console tanpa mengganggu posisi scroll bila diminta
  async function focusInput(preventScroll = false) {
    await tick();
    if (inputPromptEl) {
      inputPromptEl.focus({ preventScroll });
    }
  }

  // Fokuskan terminal aktif (baik PTY xterm.js maupun input Console)
  function focusActiveTerminal() {
    if (!activeTab) return;
    if (activeTab.mode === 'pty') {
      xtermRefs[activeTab.id]?.focus();
    } else {
      focusInput();
    }
  }

  // Sesuaikan ulang dimensi xterm aktif setelah sidebar/fullscreen berubah
  function refitActiveXterm() {
    if (!activeTab || activeTab.mode !== 'pty') return;
    requestAnimationFrame(() => {
      xtermRefs[activeTab.id]?.fit();
    });
  }

  // Cek apakah ada teks yang sedang diseleksi di layar Console
  function hasActiveViewportSelection(): boolean {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || sel.toString().length === 0) return false;
    if (!terminalScrollEl) return true;
    return (
      (sel.anchorNode !== null && terminalScrollEl.contains(sel.anchorNode)) ||
      (sel.focusNode !== null && terminalScrollEl.contains(sel.focusNode))
    );
  }

  function handleViewportMouseDown(e: MouseEvent) {
    viewportPointerDown = { x: e.clientX, y: e.clientY };
  }

  // Fokus ke input Console hanya pada klik biasa agar stabilo seleksi teks tetap utuh saat mouse dilepas
  function handleViewportClick(e: MouseEvent) {
    const movedDist = Math.hypot(e.clientX - viewportPointerDown.x, e.clientY - viewportPointerDown.y);
    if (movedDist > 4) return;
    if (hasActiveViewportSelection()) return;
    const target = e.target as HTMLElement | null;
    if (target?.closest('button, a, input, textarea')) return;
    focusInput(true);
  }

  // Kembalikan simbol prompt standar SSH Linux ('#' untuk root, '$' untuk non-root)
  function promptSymbol(username: string): string {
    return username === 'root' ? '#' : '$';
  }

  // Format durasi eksekusi perintah secara ringkas
  function formatDuration(ms?: number): string {
    if (ms === undefined || ms < 0) return '';
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
  }

  // Salin output bersih (bebas kode ANSI) dari satu blok riwayat Console
  async function copyEntry(item: TerminalHistoryItem, key: string, e: MouseEvent) {
    e.stopPropagation();
    const raw = item.output ? stripAnsi(item.output) : item.command;
    if (!raw) return;
    try {
      await copyText(raw);
      copiedEntryKey = key;
      if (copyFeedbackTimer) clearTimeout(copyFeedbackTimer);
      copyFeedbackTimer = setTimeout(() => {
        copiedEntryKey = null;
      }, 1500);
    } catch {
      toast.error('Gagal menyalin ke clipboard');
    }
  }

  async function placeCaretAtEnd() {
    await tick();
    if (inputPromptEl) {
      const len = inputPromptEl.value.length;
      inputPromptEl.setSelectionRange(len, len);
    }
  }

  function resetCompletionState() {
    completionMatches = [];
    tabPressCount = 0;
  }

  // Catat perintah ke riwayat persisten localStorage untuk tab tertentu
  function saveCommandToHistory(tab: TerminalTab, cmd: string) {
    const updated = recordTerminalHistory(historyScopeKey(tab), cmd);
    if (activeTab && tab.id === activeTab.id) {
      persistedHistory = updated;
    }
  }

  // Inisialisasi direktori awal ('~') dan versi panel tanpa memindai direktori
  async function initTabSession(tab: TerminalTab) {
    try {
      const res = await apiFetch<{
        ok: boolean;
        cwd?: string;
        panel_version?: string;
      }>('/api/terminal/complete', {
        method: 'POST',
        body: JSON.stringify({
          command: '',
          cwd: tab.cwd || '~',
          server_id: tab.serverId,
          init_only: true,
        }),
      });
      if (res.cwd) {
        tab.cwd = res.cwd;
      }
      if (res.panel_version) {
        panelVersion = res.panel_version;
      }
    } catch {
      // Abaikan kegagalan inisialisasi ringan
    }
  }

  // Ganti mode tab aktif antara PTY (interaktif WebSocket) dan Console (streaming SSE)
  function switchTabMode(nextMode: TerminalMode) {
    if (!activeTab || activeTab.mode === nextMode) return;
    activeTab.mode = nextMode;
    if (nextMode === 'pty') {
      requestAnimationFrame(() => {
        xtermRefs[activeTab.id]?.fit();
        xtermRefs[activeTab.id]?.focus();
      });
    } else {
      scrollToBottom();
      focusInput();
    }
  }

  // Hentikan paksa proses SSE yang sedang berjalan di tab (Ctrl+C / tombol Stop)
  function stopRunningStream(tab: TerminalTab) {
    const ctl = activeStreamControllers.get(tab.id);
    if (ctl) {
      ctl.abort();
      activeStreamControllers.delete(tab.id);
      toast.info('Mengirim sinyal henti (SIGINT) ke proses...');
    }
  }

  // Jalankan perintah di tab aktif (otomatis ke PTY jika mode PTY, atau SSE streaming jika mode Console)
  async function dispatchCommandToActiveTab(cmdText: string, mode: 'run' | 'paste') {
    if (!activeTab) return;
    const clean = cmdText.trim();
    if (!clean) return;

    if (activeTab.mode === 'pty') {
      const xterm = xtermRefs[activeTab.id];
      if (!xterm) {
        toast.warning('Sesi PTY sedang diinisialisasi');
        return;
      }
      if (mode === 'run') {
        xterm.runCommand(clean);
      } else {
        xterm.sendInput(clean);
        toast.info('Perintah ditempel ke terminal PTY');
      }
      return;
    }

    if (mode === 'paste') {
      activeTab.commandInput = clean;
      toast.info('Perintah ditempel ke input prompt');
      focusInput();
      placeCaretAtEnd();
    } else {
      await runConsoleCommand(clean);
    }
  }

  // Jalankan perintah di mode Console menggunakan SSE live streaming (dengan dukungan Stop/Ctrl+C)
  async function runConsoleCommand(cmdText: string) {
    const trimmed = cmdText.trim();
    if (!activeTab || activeTab.executing) return;

    const currentTab = activeTab;
    resetCompletionState();

    // Mode prompt interaktif bertahap untuk php artisan make:filament-user
    if (interactivePrompt) {
      const now = new Date().toLocaleTimeString();
      if (interactivePrompt.step === 'name') {
        const val = trimmed || 'Super Admin';
        interactivePrompt.name = val;
        interactivePrompt.step = 'email';
        currentTab.history = [
          ...currentTab.history,
          { command: `┌ Name: ${val}`, output: '', time: now, ok: true, cwd: currentTab.cwd || '~' },
        ];
        currentTab.commandInput = '';
        scrollToBottom();
        focusInput();
        return;
      }
      if (interactivePrompt.step === 'email') {
        const val = trimmed || 'admin@fikfikk.my.id';
        interactivePrompt.email = val;
        interactivePrompt.step = 'password';
        currentTab.history = [
          ...currentTab.history,
          { command: `├ Email address: ${val}`, output: '', time: now, ok: true, cwd: currentTab.cwd || '~' },
        ];
        currentTab.commandInput = '';
        scrollToBottom();
        focusInput();
        return;
      }
      if (interactivePrompt.step === 'password') {
        const passVal = trimmed || 'admin123';
        const finalCmd = `${interactivePrompt.baseCmd} --name=${JSON.stringify(interactivePrompt.name)} --email=${JSON.stringify(interactivePrompt.email)} --password=${JSON.stringify(passVal)} --no-interaction`;
        interactivePrompt = null;
        currentTab.commandInput = '';
        await runConsoleCommand(finalCmd);
        return;
      }
    }

    if (!trimmed) return;

    if (trimmed.includes('make:filament-user') && !trimmed.includes('--name')) {
      const now = new Date().toLocaleTimeString();
      currentTab.history = [
        ...currentTab.history,
        {
          command: trimmed,
          output: 'Mode Interaktif Filament User — ketik jawaban di bawah lalu tekan Enter (Ctrl+C untuk batal):',
          time: now,
          ok: true,
          cwd: currentTab.cwd || '~',
        },
      ];
      interactivePrompt = { baseCmd: trimmed, step: 'name', name: '', email: '' };
      currentTab.commandInput = '';
      scrollToBottom();
      focusInput();
      return;
    }

    if (trimmed === 'clear' || trimmed === 'cls') {
      currentTab.history = [];
      currentTab.commandInput = '';
      currentTab.draftInput = '';
      currentTab.historyIndex = -1;
      focusInput();
      return;
    }

    saveCommandToHistory(currentTab, trimmed);

    currentTab.executing = true;
    const executedAtCwd = currentTab.cwd || '~';
    currentTab.commandInput = '';
    currentTab.draftInput = '';
    currentTab.historyIndex = -1;

    // Tambahkan entri streaming langsung agar output mengalir real-time baris per baris
    const entryIndex = currentTab.history.length;
    currentTab.history = [
      ...currentTab.history,
      {
        command: trimmed,
        output: '',
        time: new Date().toLocaleTimeString(),
        ok: true,
        cwd: executedAtCwd,
        streaming: true,
      },
    ];
    scrollToBottom();

    const ctl = new AbortController();
    activeStreamControllers.set(currentTab.id, ctl);

    try {
      const payload: { command: string; server_id?: number; cwd?: string } = {
        command: trimmed,
        cwd: currentTab.cwd || '~',
      };
      if (currentTab.serverId > 0) {
        payload.server_id = currentTab.serverId;
      }

      await apiStreamSSE(
        '/api/terminal/exec/stream',
        payload,
        {
          onEvent: (eventName, data) => {
            const entry = currentTab.history[entryIndex];
            if (!entry) return;

            if (eventName === 'chunk' && typeof data?.text === 'string') {
              entry.output += data.text;
              scrollToBottom();
            } else if (eventName === 'error') {
              const msg = data?.message || 'Eksekusi perintah gagal';
              entry.output += (entry.output ? '\n' : '') + msg;
              entry.ok = false;
              entry.streaming = false;
              currentTab.connected = false;
              scrollToBottom();
            } else if (eventName === 'done') {
              entry.ok = Boolean(data?.ok);
              entry.exitCode = typeof data?.exit_code === 'number' ? data.exit_code : entry.ok ? 0 : 1;
              entry.durationMs = typeof data?.duration_ms === 'number' ? data.duration_ms : undefined;
              entry.streaming = false;
              if (data?.cwd) currentTab.cwd = data.cwd;
              if (data?.prompt_user) currentTab.username = data.prompt_user;
              if (data?.prompt_host) currentTab.host = data.prompt_host;
              if (data?.fingerprint) currentTab.fingerprint = data.fingerprint;
              currentTab.connected = true;
              scrollToBottom();
            }
          },
        },
        ctl.signal,
      );
    } catch (err: any) {
      const entry = currentTab.history[entryIndex];
      if (entry) {
        if (err?.name === 'AbortError') {
          entry.output += (entry.output ? '\n' : '') + '^C [Dihentikan oleh pengguna]';
          entry.ok = false;
          entry.exitCode = 130;
        } else {
          let errorMsg = `Error: ${err?.message || 'Gagal mengeksekusi perintah'}`;
          if (
            trimmed.startsWith('fikpanel update') ||
            trimmed.includes('systemctl restart fikpanel') ||
            trimmed.includes('restart fikpanel')
          ) {
            errorMsg +=
              '\n[Info] Layanan FikPanel sedang dimuat ulang di latar belakang. Silakan refresh halaman dalam beberapa detik.';
          }
          entry.output += (entry.output ? '\n' : '') + errorMsg;
          entry.ok = false;
          currentTab.connected = false;
        }
        entry.streaming = false;
      }
    } finally {
      activeStreamControllers.delete(currentTab.id);
      const entry = currentTab.history[entryIndex];
      if (entry && entry.streaming) {
        entry.streaming = false;
      }
      currentTab.executing = false;
      scrollToBottom();
      focusInput();
    }
  }

  // Cetak daftar kandidat auto-complete pada Tab 2x (mode Console)
  function triggerDoubleTabListing() {
    if (!activeTab || completionMatches.length === 0) return;
    const now = new Date().toLocaleTimeString();
    const detailLines = completionMatches.map((m) => m.detail).join('\n');
    activeTab.history = [
      ...activeTab.history,
      {
        command: activeTab.commandInput,
        output: detailLines,
        time: now,
        ok: true,
        cwd: activeTab.cwd || '~',
        isCompletionList: true,
      },
    ];
    scrollToBottom();
    focusInput();
  }

  // Tangani auto-complete Tab di mode Console
  async function handleTabAutocomplete() {
    if (!activeTab || activeTab.executing || completing) return;
    const currentTab = activeTab;

    if (tabPressCount >= 1 && currentTab.commandInput === lastTabCommand && completionMatches.length > 1) {
      tabPressCount++;
      triggerDoubleTabListing();
      return;
    }

    completing = true;
    try {
      const res = await apiFetch<{
        ok: boolean;
        cwd?: string;
        prefix: string;
        common_prefix: string;
        completed_command: string;
        dirs_only: boolean;
        matches: TerminalCompletionItem[];
      }>('/api/terminal/complete', {
        method: 'POST',
        body: JSON.stringify({
          command: currentTab.commandInput,
          cwd: currentTab.cwd || '~',
          server_id: currentTab.serverId,
        }),
      });

      if (res.cwd) currentTab.cwd = res.cwd;

      const matches = res.matches || [];
      if (matches.length === 0) {
        resetCompletionState();
      } else if (matches.length === 1) {
        currentTab.commandInput = res.completed_command;
        lastTabCommand = res.completed_command;
        resetCompletionState();
      } else {
        currentTab.commandInput = res.completed_command;
        lastTabCommand = res.completed_command;
        completionMatches = matches;
        tabPressCount = 1;
      }
    } catch {
      // Abaikan error jaringan sementara saat auto-complete
    } finally {
      completing = false;
      scrollToBottom();
      focusInput();
    }
  }

  // Gabungkan riwayat persisten + riwayat sesi tab untuk navigasi ArrowUp/ArrowDown mode Console
  function getCombinedCommandHistory(tab: TerminalTab): string[] {
    const merged: string[] = [...persistedHistory];
    for (const h of tab.history) {
      if (h.isCompletionList || h.command.endsWith('^C')) continue;
      const c = h.command.trim();
      if (!c) continue;
      if (merged[merged.length - 1] !== c) {
        merged.push(c);
      }
    }
    return merged;
  }

  // Tangani keyboard input di mode Console
  function handleInputKeydown(e: KeyboardEvent) {
    if (!activeTab) return;

    if (e.key === 'Tab') {
      e.preventDefault();
      handleTabAutocomplete();
      return;
    }

    if (e.ctrlKey && !e.shiftKey && e.key.toLowerCase() === 'c') {
      if (activeTab.executing) {
        e.preventDefault();
        stopRunningStream(activeTab);
        return;
      }
      const selectedText = window.getSelection()?.toString() || '';
      const hasInputSelection =
        inputPromptEl !== null &&
        inputPromptEl.selectionStart !== null &&
        inputPromptEl.selectionEnd !== null &&
        inputPromptEl.selectionStart !== inputPromptEl.selectionEnd;
      if (!selectedText && !hasInputSelection) {
        e.preventDefault();
        const now = new Date().toLocaleTimeString();
        activeTab.history = [
          ...activeTab.history,
          {
            command: `${activeTab.commandInput}^C`,
            output: '',
            time: now,
            ok: true,
            cwd: activeTab.cwd || '~',
          },
        ];
        activeTab.commandInput = '';
        activeTab.draftInput = '';
        activeTab.historyIndex = -1;
        interactivePrompt = null;
        resetCompletionState();
        scrollToBottom();
        return;
      }
    }

    if (e.key === 'Enter') {
      e.preventDefault();
      runConsoleCommand(activeTab.commandInput);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      resetCompletionState();
      const pastCommands = getCombinedCommandHistory(activeTab);
      if (pastCommands.length === 0) return;

      if (activeTab.historyIndex === -1) {
        activeTab.draftInput = activeTab.commandInput;
        activeTab.historyIndex = pastCommands.length - 1;
      } else if (activeTab.historyIndex > 0) {
        activeTab.historyIndex--;
      }
      activeTab.commandInput = pastCommands[activeTab.historyIndex];
      placeCaretAtEnd();
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      resetCompletionState();
      const pastCommands = getCombinedCommandHistory(activeTab);
      if (activeTab.historyIndex !== -1) {
        if (activeTab.historyIndex < pastCommands.length - 1) {
          activeTab.historyIndex++;
          activeTab.commandInput = pastCommands[activeTab.historyIndex];
        } else {
          activeTab.historyIndex = -1;
          activeTab.commandInput = activeTab.draftInput || '';
        }
        placeCaretAtEnd();
      }
    } else if (e.key === 'Escape') {
      resetCompletionState();
    } else if (!e.ctrlKey && !e.altKey && !e.metaKey && e.key.length === 1) {
      tabPressCount = 0;
    }
  }

  // Bersihkan layar terminal aktif (PTY maupun Console)
  function clearActiveTerminal() {
    if (!activeTab) return;
    if (activeTab.mode === 'pty') {
      xtermRefs[activeTab.id]?.clear();
    } else {
      activeTab.history = [];
      resetCompletionState();
      focusInput();
    }
    toast.info('Layar terminal dibersihkan');
  }

  // ── Pencarian Output (Ctrl+Shift+F) ──
  async function openSearchBar() {
    showSearchBar = true;
    await tick();
    searchInputEl?.focus();
    searchInputEl?.select();
  }

  function closeSearchBar() {
    showSearchBar = false;
    terminalSearchQuery = '';
    if (activeTab?.mode === 'pty') {
      xtermRefs[activeTab.id]?.clearSearch();
    }
    focusActiveTerminal();
  }

  function runTerminalSearch(direction: 'next' | 'prev') {
    const q = terminalSearchQuery.trim();
    if (!q || !activeTab) return;
    if (activeTab.mode === 'pty') {
      const xterm = xtermRefs[activeTab.id];
      const found =
        direction === 'next'
          ? xterm?.findNext(q, { caseSensitive: searchCaseSensitive })
          : xterm?.findPrevious(q, { caseSensitive: searchCaseSensitive });
      if (!found) {
        toast.info('Teks tidak ditemukan di buffer terminal');
      }
    } else {
      // Lompat ke mark hasil pencarian berikutnya di layar Console
      const marks = terminalScrollEl?.querySelectorAll('mark.search-mark');
      if (marks && marks.length > 0) {
        const target = direction === 'next' ? marks[0] : marks[marks.length - 1];
        target.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    }
  }

  // ── Export Log Sesi (.txt) ──
  function exportActiveLog() {
    if (!activeTab) return;
    let content = '';
    if (activeTab.mode === 'pty') {
      content = xtermRefs[activeTab.id]?.exportLog() || '';
    } else {
      content = activeTab.history
        .map((item) => {
          const header = `${activeTab.username}@${activeTab.host}:${item.cwd || '~'}${promptSymbol(activeTab.username)} ${item.command}  [${item.time}]`;
          const body = item.output ? stripAnsi(item.output) : '';
          return body ? `${header}\n${body}` : header;
        })
        .join('\n\n');
    }

    if (!content.trim()) {
      toast.warning('Belum ada output terminal untuk diekspor');
      return;
    }

    const safeName = activeTab.name.toLowerCase().replace(/[^a-z0-9-_]+/g, '-');
    const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
    downloadTextFile(`fikpanel-${safeName}-${stamp}.txt`, content);
    toast.success('Log terminal berhasil diunduh (.txt)');
  }

  // ── Reverse History Picker (Ctrl+R / Alt+R) ──
  async function openHistoryPicker() {
    persistedHistory = loadTerminalHistory(historyScopeKey(activeTab));
    historyPickerQuery = '';
    historyPickerIndex = 0;
    showHistoryPicker = true;
    await tick();
    historyInputEl?.focus();
  }

  function handleHistoryPickerKeydown(e: KeyboardEvent) {
    const items = filteredHistoryItems;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (items.length > 0) historyPickerIndex = (historyPickerIndex + 1) % items.length;
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (items.length > 0) historyPickerIndex = (historyPickerIndex - 1 + items.length) % items.length;
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const chosen = items[historyPickerIndex];
      if (chosen) {
        showHistoryPicker = false;
        dispatchCommandToActiveTab(chosen, 'run');
      }
    } else if (e.key === 'Tab') {
      e.preventDefault();
      const chosen = items[historyPickerIndex];
      if (chosen) {
        showHistoryPicker = false;
        dispatchCommandToActiveTab(chosen, 'paste');
      }
    } else if (e.key === 'Escape') {
      e.preventDefault();
      showHistoryPicker = false;
      focusActiveTerminal();
    }
  }

  async function clearPersistedHistory() {
    const ok = await confirmDialog({
      title: 'Hapus Riwayat Perintah?',
      message: `Seluruh riwayat perintah tersimpan untuk "${activeTab?.name || 'tab ini'}" akan dibersihkan dari browser.`,
      confirmText: 'Hapus Riwayat',
      tone: 'danger',
    });
    if (!ok) return;
    clearTerminalHistoryStore(historyScopeKey(activeTab));
    persistedHistory = [];
    toast.success('Riwayat perintah dibersihkan');
  }

  // ── Shortcut Command dengan Dukungan Template Variabel {{var:default}} ──
  function triggerSavedCommand(cmd: TerminalCommand, action: 'run' | 'paste') {
    const vars = extractCommandVariables(cmd.content);
    if (vars.length === 0) {
      dispatchCommandToActiveTab(cmd.content, action);
      return;
    }
    pendingTemplateCmd = cmd;
    pendingTemplateAction = action;
    pendingTemplateVars = vars;
    const initVals: Record<string, string> = {};
    for (const v of vars) {
      initVals[v.name] = v.defaultValue;
    }
    pendingVarValues = initVals;
    showVarModal = true;
  }

  function confirmTemplateCommand() {
    if (!pendingTemplateCmd) return;
    const missing = pendingTemplateVars.find((v) => !(pendingVarValues[v.name] || '').trim());
    if (missing) {
      toast.warning(`Variabel "${missing.name}" wajib diisi`);
      return;
    }
    const finalCmd = interpolateCommand(pendingTemplateCmd.content, pendingVarValues);
    const action = pendingTemplateAction;
    showVarModal = false;
    pendingTemplateCmd = null;
    dispatchCommandToActiveTab(finalCmd, action);
  }

  // ── Manajemen Tab Terminal ──
  function openLocalTab() {
    const newId = `local-${Date.now()}`;
    const newTab: TerminalTab = {
      id: newId,
      serverId: 0,
      name: `Local server ${tabs.filter((t) => t.serverId === 0).length + 1}`,
      host: 'fikpanel',
      username: 'root',
      cwd: '~',
      mode: 'pty',
      connected: true,
      ptyStatus: 'connecting',
      commandInput: '',
      draftInput: '',
      historyIndex: -1,
      executing: false,
      history: [],
    };
    tabs = [...tabs, newTab];
    activeTabId = newId;
    showAddTabMenu = false;
    resetCompletionState();
    initTabSession(newTab);
  }

  function connectToServer(srv: TerminalServer) {
    const existing = tabs.find((t) => t.serverId === srv.id);
    if (existing) {
      activeTabId = existing.id;
      toast.info(`Beralih ke tab: ${srv.name || srv.host}`);
      showAddTabMenu = false;
      resetCompletionState();
      focusActiveTerminal();
      return;
    }

    const newId = `server-${srv.id}-${Date.now()}`;
    const newTab: TerminalTab = {
      id: newId,
      serverId: srv.id,
      name: srv.name || srv.host,
      host: srv.host,
      username: srv.username || 'root',
      cwd: '~',
      mode: 'pty',
      connected: true,
      ptyStatus: 'connecting',
      fingerprint: srv.host_key_fingerprint,
      commandInput: '',
      draftInput: '',
      historyIndex: -1,
      executing: false,
      history: [],
    };

    tabs = [...tabs, newTab];
    activeTabId = newId;
    showAddTabMenu = false;
    resetCompletionState();
    toast.success(`Membuka sesi SSH ke ${srv.name || srv.host}`);
  }

  function closeTab(tabId: string, event: MouseEvent) {
    event.stopPropagation();
    if (tabs.length <= 1) {
      toast.warning('Minimal satu tab terminal harus tetap aktif');
      return;
    }

    const ctl = activeStreamControllers.get(tabId);
    if (ctl) {
      ctl.abort();
      activeStreamControllers.delete(tabId);
    }
    delete xtermRefs[tabId];

    const idx = tabs.findIndex((t) => t.id === tabId);
    tabs = tabs.filter((t) => t.id !== tabId);

    if (activeTabId === tabId) {
      const nextTab = tabs[Math.max(0, idx - 1)];
      activeTabId = nextTab.id;
    }
  }

  // ── Reset Host Key TOFU untuk Remote Server ──
  async function resetServerHostKey(srv: TerminalServer, autoReconnectTabId?: string) {
    const ok = await confirmDialog({
      title: 'Reset Host Key SSH (TOFU)?',
      message: `Sidik jari host key tersimpan untuk "${srv.host}:${srv.port}" akan dihapus. Lakukan ini hanya jika VPS baru di-rebuild atau kunci host SSH memang diganti secara sah.`,
      confirmText: 'Reset Host Key',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/terminal/servers/${srv.id}/host-key`, {
        method: 'DELETE',
      });
      toast.success(`Host key ${srv.host}:${srv.port} berhasil direset`);
      await fetchServers();
      if (autoReconnectTabId) {
        xtermRefs[autoReconnectTabId]?.reconnect();
      }
    } catch (err: any) {
      toast.error(`Gagal mereset host key: ${err.message}`);
    }
  }

  // ── Modal Server Remote CRUD ──
  function openAddServerModal() {
    editingServerId = null;
    serverFormHost = '';
    serverFormPort = 22;
    serverFormUser = 'root';
    serverFormAuthType = 'password';
    serverFormPassword = '';
    serverFormPrivateKey = '';
    serverFormPassphrase = '';
    serverFormRemarks = '';
    testConnResult = null;
    showPasswordPlain = false;
    showServerModal = true;
    showAddTabMenu = false;
  }

  function openEditServerModal(srv: TerminalServer) {
    editingServerId = srv.id;
    serverFormHost = srv.host;
    serverFormPort = srv.port || 22;
    serverFormUser = srv.username || 'root';
    serverFormAuthType = srv.auth_type || 'password';
    serverFormPassword = '';
    serverFormPrivateKey = '';
    serverFormPassphrase = '';
    serverFormRemarks = srv.remarks || '';
    testConnResult = null;
    showPasswordPlain = false;
    showServerModal = true;
  }

  async function testServerConnection() {
    if (!serverFormHost.trim()) {
      toast.warning('Alamat Server IP/Host wajib diisi');
      return;
    }

    testingConnection = true;
    testConnResult = null;

    try {
      const payload: any = {
        host: serverFormHost.trim(),
        port: Number(serverFormPort) || 22,
        username: serverFormUser.trim() || 'root',
        auth_type: serverFormAuthType,
      };

      if (serverFormAuthType === 'password') {
        payload.password = serverFormPassword;
      } else {
        payload.private_key = serverFormPrivateKey;
        payload.passphrase = serverFormPassphrase;
      }

      if (editingServerId && !serverFormPassword && !serverFormPrivateKey) {
        payload.server_id = editingServerId;
      }

      const res = await apiFetch<{
        ok: boolean;
        latency_ms?: number;
        banner?: string;
        fingerprint?: string;
        host_key_mismatch?: boolean;
        message?: string;
        error?: string;
      }>('/api/terminal/servers/test', {
        method: 'POST',
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        const fpNote = res.fingerprint ? ` · ${res.fingerprint}` : '';
        testConnResult = {
          ok: true,
          message: `${res.message || 'Koneksi berhasil'} — ${res.banner || 'OK'}${fpNote}`,
        };
        toast.success(`Koneksi SSH berhasil (${res.latency_ms} ms)`);
        await fetchServers();
      } else {
        testConnResult = {
          ok: false,
          hostKeyMismatch: Boolean(res.host_key_mismatch),
          message: res.error || 'Koneksi gagal terhubung',
        };
        toast.error(`Koneksi gagal: ${res.error}`);
      }
    } catch (err: any) {
      testConnResult = {
        ok: false,
        message: err.message || 'Gagal menghubungi server',
      };
      toast.error(`Error pengujian: ${err.message}`);
    } finally {
      testingConnection = false;
    }
  }

  async function testServerRow(srv: TerminalServer) {
    testingServerId = srv.id;
    try {
      const res = await apiFetch<{
        ok: boolean;
        latency_ms?: number;
        fingerprint?: string;
        host_key_mismatch?: boolean;
        error?: string;
      }>('/api/terminal/servers/test', {
        method: 'POST',
        body: JSON.stringify({ server_id: srv.id }),
      });

      if (res.ok) {
        toast.success(`Server [${srv.host}]: Koneksi normal (${res.latency_ms} ms)`);
        await fetchServers();
      } else {
        toast.error(`Server [${srv.host}]: ${res.error || 'Koneksi terputus'}`);
      }
    } catch (err: any) {
      toast.error(`Gagal menguji server: ${err.message}`);
    } finally {
      testingServerId = null;
    }
  }

  async function saveServer() {
    if (!serverFormHost.trim()) {
      toast.warning('Alamat Server IP/Host wajib diisi');
      return;
    }

    savingServer = true;
    try {
      const payload: any = {
        name: serverFormRemarks.trim() || serverFormHost.trim(),
        host: serverFormHost.trim(),
        port: Number(serverFormPort) || 22,
        username: serverFormUser.trim() || 'root',
        auth_type: serverFormAuthType,
        remarks: serverFormRemarks.trim(),
      };

      if (serverFormAuthType === 'password') {
        payload.password = serverFormPassword;
      } else {
        payload.private_key = serverFormPrivateKey;
        payload.passphrase = serverFormPassphrase;
      }

      if (editingServerId) {
        await apiFetch(`/api/terminal/servers/${editingServerId}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        });
        toast.success('Konfigurasi server diperbarui (kredensial terenkripsi AES-256)');
      } else {
        await apiFetch('/api/terminal/servers', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        toast.success('Server remote ditambahkan (kredensial terenkripsi AES-256)');
      }

      showServerModal = false;
      await fetchServers();
    } catch (err: any) {
      toast.error(`Gagal menyimpan server: ${err.message}`);
    } finally {
      savingServer = false;
    }
  }

  async function deleteServer(srv: TerminalServer) {
    const ok = await confirmDialog({
      title: 'Hapus Server Remote?',
      message: `Konfigurasi server "${srv.name || srv.host}" (${srv.host}:${srv.port}) akan dihapus dari panel.`,
      confirmText: 'Hapus Server',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/terminal/servers/${srv.id}`, {
        method: 'DELETE',
      });
      toast.success('Server berhasil dihapus');
      await fetchServers();
    } catch (err: any) {
      toast.error(`Gagal menghapus server: ${err.message}`);
    }
  }

  // ── Modal Shortcut Command CRUD ──
  function openAddCommandModal() {
    editingCommandId = null;
    commandFormName = '';
    commandFormContent = '';
    showCommandModal = true;
  }

  function openEditCommandModal(cmd: TerminalCommand) {
    editingCommandId = cmd.id;
    commandFormName = cmd.name;
    commandFormContent = cmd.content;
    showCommandModal = true;
  }

  async function saveCommand() {
    if (!commandFormName.trim() || !commandFormContent.trim()) {
      toast.warning('Nama dan konten perintah wajib diisi');
      return;
    }

    savingCommand = true;
    try {
      const payload = {
        name: commandFormName.trim(),
        content: commandFormContent.trim(),
      };

      if (editingCommandId) {
        await apiFetch(`/api/terminal/commands/${editingCommandId}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        });
        toast.success('Pintasan perintah berhasil diperbarui');
      } else {
        await apiFetch('/api/terminal/commands', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        toast.success('Pintasan perintah berhasil ditambahkan');
      }

      showCommandModal = false;
      await fetchCommands();
    } catch (err: any) {
      toast.error(`Gagal menyimpan perintah: ${err.message}`);
    } finally {
      savingCommand = false;
    }
  }

  async function deleteCommand(cmd: TerminalCommand) {
    const ok = await confirmDialog({
      title: 'Hapus Pintasan Perintah?',
      message: `Pintasan perintah "${cmd.name}" akan dihapus.`,
      confirmText: 'Hapus',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/terminal/commands/${cmd.id}`, {
        method: 'DELETE',
      });
      toast.success('Perintah berhasil dihapus');
      await fetchCommands();
    } catch (err: any) {
      toast.error(`Gagal menghapus perintah: ${err.message}`);
    }
  }

  // Toggle layar penuh dan refit PTY
  function toggleFullscreen() {
    isFullscreen = !isFullscreen;
    if (isFullscreen && terminalBoxEl) {
      if (terminalBoxEl.requestFullscreen) {
        terminalBoxEl.requestFullscreen().catch(() => {});
      }
    } else if (document.fullscreenElement) {
      document.exitFullscreen().catch(() => {});
    }
    refitActiveXterm();
  }

  function toggleSidebar() {
    sidebarOpen = !sidebarOpen;
    refitActiveXterm();
  }
</script>

<div class="pg-terminal terminal-page">
  <!-- Kontainer Utama Terminal & Split Panel -->
  <div class="terminal-layout" class:fullscreen={isFullscreen} bind:this={terminalBoxEl}>
    <!-- Kolom Kiri: Layar Terminal & Tab Bar -->
    <div class="terminal-main">
      <!-- Tab Bar atas -->
      <div class="terminal-tab-bar">
        <div class="tabs-scroll">
          {#each tabs as tab (tab.id)}
            <div
              class="term-tab"
              class:active={tab.id === activeTabId}
              onclick={() => {
                activeTabId = tab.id;
                focusActiveTerminal();
              }}
              onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  activeTabId = tab.id;
                  focusActiveTerminal();
                }
              }}
              role="tab"
              tabindex="0"
              aria-selected={tab.id === activeTabId}
            >
              <span
                class="dot"
                class:dot--success={tab.connected && !tab.executing && tab.ptyStatus !== 'error' && tab.ptyStatus !== 'disconnected'}
                class:dot--danger={!tab.connected || tab.ptyStatus === 'error' || tab.ptyStatus === 'disconnected'}
                class:dot--warning={tab.executing || (tab.mode === 'pty' && tab.ptyStatus === 'connecting')}
              ></span>
              <span class="tab-label" title={`${tab.name} (${tab.username}@${tab.host})`}>
                {tab.name}
              </span>
              {#if tabs.length > 1}
                <button
                  type="button"
                  class="tab-close-btn"
                  onclick={(e) => closeTab(tab.id, e)}
                  title="Tutup tab"
                  aria-label="Tutup tab"
                >
                  <i class="ri-close-line"></i>
                </button>
              {/if}
            </div>
          {/each}

          <!-- Tombol + untuk Buka Tab Baru atau Pilih Server -->
          <div class="add-tab-wrapper" bind:this={addTabWrapperEl}>
            <button
              type="button"
              class="add-tab-btn"
              onclick={() => (showAddTabMenu = !showAddTabMenu)}
              title="Tambah sesi terminal / hubungkan server"
              aria-label="Tambah terminal"
            >
              <i class="ri-add-line"></i>
            </button>

            {#if showAddTabMenu}
              <div class="add-tab-menu">
                <button type="button" class="menu-item" onclick={openLocalTab}>
                  <i class="ri-terminal-box-line menu-icon"></i>
                  <span>Terminal Lokal Baru</span>
                </button>

                {#if servers.length > 0}
                  <div class="menu-divider"></div>
                  <div class="menu-header">{t('terminal.connect_remote')}</div>
                  {#each servers as srv}
                    <button type="button" class="menu-item" onclick={() => connectToServer(srv)}>
                      <i class="ri-server-line menu-icon"></i>
                      <div class="menu-item-text">
                        <span class="srv-title">{srv.name || srv.host}</span>
                        <span class="srv-sub">{srv.username}@{srv.host}:{srv.port}</span>
                      </div>
                    </button>
                  {/each}
                {/if}

                <div class="menu-divider"></div>
                <button type="button" class="menu-item menu-item--primary" onclick={openAddServerModal}>
                  <i class="ri-add-circle-line menu-icon"></i>
                  <span>{t('terminal.add_server_menu')}</span>
                </button>
              </div>
            {/if}
          </div>
        </div>

        <!-- Toolbar Kanan Tab Bar: Mode Switcher, Search, History, Export, Clear, Fullscreen -->
        <div class="tab-bar-actions">
          {#if activeTab}
            <div class="mode-switch" role="group" aria-label="Mode terminal">
              <button
                type="button"
                class="mode-pill"
                class:is-active={activeTab.mode === 'pty'}
                onclick={() => switchTabMode('pty')}
                title="PTY Interactive (WebSocket + xterm.js: nano, vim, htop, tail -f, truecolor)"
              >
                PTY
              </button>
              <button
                type="button"
                class="mode-pill"
                class:is-active={activeTab.mode === 'console'}
                onclick={() => switchTabMode('console')}
                title="Console Mode (SSE streaming, copy per block, Tab auto-complete, stop process)"
              >
                Console
              </button>
            </div>
          {/if}

          <button
            type="button"
            class="action-icon-btn"
            class:is-active={showSearchBar}
            onclick={() => (showSearchBar ? closeSearchBar() : openSearchBar())}
            title={t('terminal.search_btn')}
            aria-label={t('terminal.search_btn')}
          >
            <i class="ri-search-line"></i>
          </button>

          <button
            type="button"
            class="action-icon-btn"
            class:is-active={showHistoryPicker}
            onclick={() => (showHistoryPicker ? (showHistoryPicker = false) : openHistoryPicker())}
            title={t('terminal.history_btn')}
            aria-label={t('terminal.history_btn')}
          >
            <i class="ri-history-line"></i>
          </button>

          <button
            type="button"
            class="action-icon-btn"
            onclick={exportActiveLog}
            title={t('terminal.export_btn')}
            aria-label={t('terminal.export_btn')}
          >
            <i class="ri-download-2-line"></i>
          </button>

          <button
            type="button"
            class="action-icon-btn"
            onclick={clearActiveTerminal}
            title={t('terminal.clear_btn')}
            aria-label={t('terminal.clear_btn')}
          >
            <i class="ri-delete-bin-7-line"></i>
          </button>

          <button
            type="button"
            class="action-icon-btn"
            onclick={toggleFullscreen}
            title={isFullscreen ? t('terminal.fullscreen_exit') : t('terminal.fullscreen_enter')}
            aria-label={t('terminal.fullscreen_enter')}
          >
            <i class={isFullscreen ? 'ri-fullscreen-exit-line' : 'ri-fullscreen-line'}></i>
          </button>
        </div>
      </div>

      <!-- Bilah Pencarian Output Terminal (Ctrl+Shift+F) -->
      {#if showSearchBar}
        <div class="term-overlay-bar">
          <i class="ri-search-line overlay-bar-icon"></i>
          <input
            type="text"
            class="overlay-bar-input mono"
            bind:this={searchInputEl}
            bind:value={terminalSearchQuery}
            placeholder={t('terminal.search_placeholder')}
            onkeydown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                runTerminalSearch(e.shiftKey ? 'prev' : 'next');
              } else if (e.key === 'Escape') {
                e.preventDefault();
                closeSearchBar();
              }
            }}
          />
          {#if activeTab?.mode === 'console' && terminalSearchQuery.trim()}
            <span class="overlay-bar-meta">{t('terminal.search_matches', { count: consoleSearchMatchCount })}</span>
          {/if}
          <button
            type="button"
            class="overlay-bar-btn"
            class:is-active={searchCaseSensitive}
            onclick={() => (searchCaseSensitive = !searchCaseSensitive)}
            title="Peka huruf besar/kecil (Case Sensitive)"
            aria-label="Peka huruf besar/kecil"
          >
            Aa
          </button>
          <button
            type="button"
            class="overlay-bar-btn"
            onclick={() => runTerminalSearch('prev')}
            title="Hasil sebelumnya (Shift+Enter)"
            aria-label="Hasil sebelumnya"
          >
            <i class="ri-arrow-up-s-line"></i>
          </button>
          <button
            type="button"
            class="overlay-bar-btn"
            onclick={() => runTerminalSearch('next')}
            title="Hasil berikutnya (Enter)"
            aria-label="Hasil berikutnya"
          >
            <i class="ri-arrow-down-s-line"></i>
          </button>
          <button
            type="button"
            class="overlay-bar-btn"
            onclick={closeSearchBar}
            title="Tutup pencarian (Esc)"
            aria-label="Tutup pencarian"
          >
            <i class="ri-close-line"></i>
          </button>
        </div>
      {/if}

      <!-- Overlay Reverse Search Riwayat Perintah Persisten (Ctrl+R / Alt+R) -->
      {#if showHistoryPicker}
        <div class="history-picker-panel">
          <div class="history-picker-head">
            <i class="ri-history-line overlay-bar-icon"></i>
            <input
              type="text"
              class="overlay-bar-input mono"
              bind:this={historyInputEl}
              bind:value={historyPickerQuery}
              oninput={() => (historyPickerIndex = 0)}
              onkeydown={handleHistoryPickerKeydown}
              placeholder={t('terminal.history_placeholder')}
            />
            {#if persistedHistory.length > 0}
              <button
                type="button"
                class="overlay-bar-btn overlay-bar-btn--danger"
                onclick={clearPersistedHistory}
                title={t('common.delete')}
                aria-label={t('common.delete')}
              >
                <i class="ri-delete-bin-line"></i>
              </button>
            {/if}
            <button
              type="button"
              class="overlay-bar-btn"
              onclick={() => {
                showHistoryPicker = false;
                focusActiveTerminal();
              }}
              title={t('shell.palette.close')}
              aria-label={t('shell.palette.close')}
            >
              <i class="ri-close-line"></i>
            </button>
          </div>

          <div class="history-picker-list" role="listbox" aria-label={t('terminal.history_btn')}>
            {#if filteredHistoryItems.length === 0}
              <div class="history-picker-empty">{t('terminal.history_empty')}</div>
            {:else}
              {#each filteredHistoryItems as cmd, idx}
                <div
                  class="history-picker-row"
                  class:is-selected={idx === historyPickerIndex}
                  role="option"
                  tabindex="-1"
                  aria-selected={idx === historyPickerIndex}
                  onmouseenter={() => (historyPickerIndex = idx)}
                >
                  <button
                    type="button"
                    class="history-picker-cmd mono truncate"
                    onclick={() => {
                      showHistoryPicker = false;
                      dispatchCommandToActiveTab(cmd, 'run');
                    }}
                    title={cmd}
                  >
                    {cmd}
                  </button>
                  <div class="history-picker-actions">
                    <button
                      type="button"
                      class="link-act link-act--run"
                      onclick={() => {
                        showHistoryPicker = false;
                        dispatchCommandToActiveTab(cmd, 'run');
                      }}
                    >
                      Run
                    </button>
                    <button
                      type="button"
                      class="link-act link-act--paste"
                      onclick={() => {
                        showHistoryPicker = false;
                        dispatchCommandToActiveTab(cmd, 'paste');
                      }}
                    >
                      Paste
                    </button>
                  </div>
                </div>
              {/each}
            {/if}
          </div>
        </div>
      {/if}

      <!-- Area Viewport Utama: Menjaga seluruh sesi PTY tab tetap hidup di latar belakang -->
      <div class="terminal-stage">
        {#each tabs as tab (tab.id)}
          {@const isTabPtyVisible = tab.id === activeTabId && tab.mode === 'pty'}
          <div class="pty-tab-pane" class:is-hidden={!isTabPtyVisible}>
            <XtermConsole
              bind:this={xtermRefs[tab.id]}
              serverId={tab.serverId}
              active={isTabPtyVisible}
              onStatusChange={(st, meta) => {
                tab.ptyStatus = st;
                tab.connected = st === 'connected' || st === 'connecting';
                if (meta?.version) panelVersion = meta.version;
                if (meta?.fingerprint) {
                  tab.fingerprint = meta.fingerprint;
                  fetchServers();
                }
              }}
              onCommandLine={(cmd) => saveCommandToHistory(tab, cmd)}
              onRequestSearch={openSearchBar}
              onRequestHistory={openHistoryPicker}
              onRequestResetHostKey={() => {
                const srv = servers.find((s) => s.id === tab.serverId);
                if (srv) resetServerHostKey(srv, tab.id);
              }}
            />
          </div>
        {/each}

        <!-- Mode Console: SSE Live Streaming + ANSI Color + Stop Proses + Per-Blok Copy -->
        {#if activeTab && activeTab.mode === 'console'}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
          <div
            class="terminal-viewport"
            bind:this={terminalScrollEl}
            onmousedown={handleViewportMouseDown}
            onclick={handleViewportClick}
            role="region"
            aria-label="Layar konsol terminal"
          >
            <!-- Banner Sesi Ringkas -->
            <div class="terminal-welcome">
              <p class="welcome-line">
                <span class="text-accent">fikpanel{panelVersion ? ` v${panelVersion}` : ''}</span>
                <span class="welcome-sep">&middot;</span>
                <span>{activeTab.name}</span>
                <span class="welcome-badge">
                  {activeTab.serverId === 0 ? 'Local SSE Console' : 'Remote SSH Console'}
                </span>
                {#if activeTab.fingerprint}
                  <span class="welcome-fp mono" title={`Host key terverifikasi (TOFU): ${activeTab.fingerprint}`}>
                    <i class="ri-shield-check-line"></i> {activeTab.fingerprint.slice(0, 18)}…
                  </span>
                {/if}
              </p>
              <p class="welcome-meta">
                <span>{activeTab.username}@{activeTab.host}</span>
                <span class="welcome-sep">&middot;</span>
                <span>{t('terminal.welcome_hint')}</span>
              </p>
            </div>

            <!-- Riwayat Eksekusi Perintah (dengan Streaming & Warna ANSI) -->
            {#each activeTab.history as item, idx (`${activeTab.id}-${idx}`)}
              {@const entryKey = `${activeTab.id}-${idx}`}
              {@const segments = item.output ? parseAnsiSegments(item.output) : []}
              <div class="term-entry">
                <div class="prompt-line">
                  <span class="prompt-prefix"
                    ><span class="prompt-user">{activeTab.username}@{activeTab.host}</span
                    ><span class="prompt-sep">:</span
                    ><span class="prompt-path">{item.cwd || activeTab.cwd || '~'}</span
                    ><span class="prompt-hash">{promptSymbol(activeTab.username)}</span></span
                  >
                  <span class="prompt-cmd">
                    {#if showSearchBar && terminalSearchQuery.trim()}
                      {#each splitHighlightMatches(item.command, terminalSearchQuery) as part}
                        {#if part.match}
                          <mark class="search-mark">{part.text}</mark>
                        {:else}
                          {part.text}
                        {/if}
                      {/each}
                    {:else}
                      {item.command}
                    {/if}
                  </span>
                  <div class="prompt-meta">
                    {#if item.streaming}
                      <span class="entry-status-chip entry-status-chip--running">
                        <span class="spinner spinner--xs"></span> live
                      </span>
                    {:else if item.exitCode !== undefined && item.exitCode !== 0}
                      <span class="entry-status-chip entry-status-chip--err">exit {item.exitCode}</span>
                    {/if}
                    {#if item.durationMs !== undefined && !item.streaming}
                      <span class="prompt-duration">{formatDuration(item.durationMs)}</span>
                    {/if}
                    <button
                      type="button"
                      class="entry-copy-btn"
                      class:is-copied={copiedEntryKey === entryKey}
                      onclick={(e) => copyEntry(item, entryKey, e)}
                      title={item.output ? 'Salin output' : 'Salin perintah'}
                      aria-label={item.output ? 'Salin output' : 'Salin perintah'}
                    >
                      <i class={copiedEntryKey === entryKey ? 'ri-check-line' : 'ri-file-copy-line'}></i>
                    </button>
                    <span class="prompt-time">{item.time}</span>
                  </div>
                </div>
                {#if item.output}
                  <pre class="term-output" class:term-output--error={!item.ok && !item.streaming}>{#each segments as seg}{#if showSearchBar && terminalSearchQuery.trim()}<span class={seg.className}>{#each splitHighlightMatches(seg.text, terminalSearchQuery) as part}{#if part.match}<mark class="search-mark">{part.text}</mark>{:else}{part.text}{/if}{/each}</span>{:else if seg.className}<span class={seg.className}>{seg.text}</span>{:else}{seg.text}{/if}{/each}</pre>
                {/if}
              </div>
            {/each}

            <!-- Baris Input Prompt Aktif -->
            <div class="input-line">
              {#if interactivePrompt}
                <span class="prompt-user prompt-interactive">
                  {interactivePrompt.step === 'name'
                    ? '┌ Name (default: Super Admin):'
                    : interactivePrompt.step === 'email'
                      ? '├ Email address (default: admin@fikfikk.my.id):'
                      : '└ Password (default: admin123):'}
                </span>
              {:else}
                <span class="prompt-prefix"
                  ><span class="prompt-user">{activeTab.username}@{activeTab.host}</span
                  ><span class="prompt-sep">:</span
                  ><span class="prompt-path">{activeTab.cwd || '~'}</span
                  ><span class="prompt-hash">{promptSymbol(activeTab.username)}</span></span
                >
              {/if}
              <!-- svelte-ignore a11y_autofocus -->
              <input
                type={interactivePrompt?.step === 'password' ? 'password' : 'text'}
                bind:this={inputPromptEl}
                bind:value={activeTab.commandInput}
                onkeydown={handleInputKeydown}
                readonly={activeTab.executing}
                spellcheck="false"
                autocomplete="off"
                autofocus
              />
              {#if activeTab.executing}
                <button
                  type="button"
                  class="stop-stream-btn"
                  onclick={() => stopRunningStream(activeTab)}
                  title="Hentikan proses berjalan (Ctrl+C)"
                >
                  <i class="ri-stop-circle-line"></i> Stop
                </button>
              {:else if completing}
                <span class="spinner spinner--sm"></span>
              {/if}
            </div>
          </div>
        {/if}
      </div>
    </div>

    <!-- Tombol Pembagi Collapse / Expand Sidebar Kanan -->
    <button
      type="button"
      class="sidebar-toggle-btn"
      class:collapsed={!sidebarOpen}
      onclick={toggleSidebar}
      title={sidebarOpen ? 'Sembunyikan panel samping' : 'Tampilkan panel samping'}
      aria-label="Toggle panel samping"
    >
      <i class={sidebarOpen ? 'ri-arrow-right-s-line' : 'ri-arrow-left-s-line'}></i>
    </button>

    <!-- Kolom Kanan: Server List & Commands -->
    {#if sidebarOpen}
      <aside class="terminal-sidebar">
        <!-- Tab Navigasi Sidebar -->
        <div class="tabs tabs--underline sidebar-tabs-nav">
          <button
            type="button"
            class="tabs__tab"
            class:is-active={activeSidebarTab === 'servers'}
            onclick={() => (activeSidebarTab = 'servers')}
          >
            {t('terminal.tab_servers')}
          </button>
          <button
            type="button"
            class="tabs__tab"
            class:is-active={activeSidebarTab === 'commands'}
            onclick={() => (activeSidebarTab = 'commands')}
          >
            {t('terminal.tab_commands')}
          </button>
        </div>

        <!-- Konten Sidebar: SERVER LIST -->
        {#if activeSidebarTab === 'servers'}
          <div class="sidebar-content">
            <div class="sidebar-actions-bar">
              <button type="button" class="btn btn--primary btn--sm" onclick={openAddServerModal}>
                <i class="ri-add-line"></i> {t('terminal.add_server')}
              </button>
              <span class="sidebar-count">{servers.length} server</span>
            </div>

            <div class="sidebar-list-wrap">
              {#if loadingServers}
                <div class="state state--loading">
                  <span class="spinner"></span>
                  <span>{t('common.loading')}</span>
                </div>
              {:else if servers.length === 0}
                <div class="state state--empty">
                  <i class="ri-inbox-2-line state__icon"></i>
                  <span class="state__title">No Data</span>
                  <span class="state__desc">{t('terminal.empty_servers')}</span>
                </div>
              {:else}
                <div class="sidebar-list">
                  {#each servers as srv (srv.id)}
                    <div class="sidebar-item">
                      <div class="sidebar-item-head">
                        <div class="sidebar-item-title-wrap">
                          <span class="srv-ip mono truncate" title={`${srv.host}:${srv.port}`}>
                            {srv.host}:{srv.port}
                          </span>
                          {#if srv.remarks}
                            <span class="badge badge--sm badge--secondary truncate max-w-100" title={srv.remarks}>
                              {srv.remarks}
                            </span>
                          {/if}
                        </div>
                        <div class="action-links">
                          <button
                            type="button"
                            class="link-act link-act--connect"
                            onclick={() => connectToServer(srv)}
                            title="Buka sesi PTY SSH di tab baru"
                          >
                            Connect
                          </button>
                          <button
                            type="button"
                            class="link-act link-act--paste"
                            onclick={() => testServerRow(srv)}
                            disabled={testingServerId === srv.id}
                            title="Uji koneksi SSH"
                          >
                            {testingServerId === srv.id ? '...' : 'Test'}
                          </button>
                          {#if srv.host_key_fingerprint}
                            <button
                              type="button"
                              class="link-act link-act--icon"
                              onclick={() => resetServerHostKey(srv)}
                              title={`Reset Host Key TOFU (${srv.host_key_fingerprint})`}
                              aria-label="Reset Host Key"
                            >
                              <i class="ri-key-2-line"></i>
                            </button>
                          {/if}
                          <button
                            type="button"
                            class="link-act link-act--icon"
                            onclick={() => openEditServerModal(srv)}
                            title="Edit server"
                            aria-label="Edit server"
                          >
                            <i class="ri-pencil-line"></i>
                          </button>
                          <button
                            type="button"
                            class="link-act link-act--icon link-act--danger"
                            onclick={() => deleteServer(srv)}
                            title="Hapus server"
                            aria-label="Hapus server"
                          >
                            <i class="ri-delete-bin-line"></i>
                          </button>
                        </div>
                      </div>
                      <div class="srv-meta-row">
                        <span class="srv-user mono text-muted truncate">
                          {srv.username} &middot; {srv.auth_type}
                        </span>
                        {#if srv.host_key_fingerprint}
                          <span
                            class="srv-tofu-badge mono"
                            title={`Host key terverifikasi (TOFU): ${srv.host_key_fingerprint}`}
                          >
                            <i class="ri-shield-check-line"></i> TOFU
                          </span>
                        {/if}
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        {/if}

        <!-- Konten Sidebar: COMMANDS -->
        {#if activeSidebarTab === 'commands'}
          <div class="sidebar-content">
            <div class="sidebar-actions-bar">
              <button type="button" class="btn btn--primary btn--sm" onclick={openAddCommandModal}>
                <i class="ri-add-line"></i> {t('terminal.add_cmd')}
              </button>
              <div class="search-input-wrap">
                <input
                  type="text"
                  bind:value={commandSearch}
                  placeholder={t('terminal.search_cmd')}
                  class="search-input"
                />
                {#if commandSearch}
                  <button
                    type="button"
                    class="search-clear-btn"
                    onclick={() => (commandSearch = '')}
                    title={t('common.reset')}
                    aria-label={t('common.reset')}
                  >
                    <i class="ri-close-line"></i>
                  </button>
                {:else}
                  <i class="ri-search-line search-icon"></i>
                {/if}
              </div>
            </div>

            <div class="sidebar-list-wrap">
              {#if loadingCommands}
                <div class="state state--loading">
                  <span class="spinner"></span>
                  <span>{t('common.loading')}</span>
                </div>
              {:else if filteredCommands.length === 0}
                <div class="state state--empty">
                  <i class="ri-inbox-2-line state__icon"></i>
                  <span class="state__title">No Data</span>
                  <span class="state__desc">{t('terminal.empty_commands')}</span>
                </div>
              {:else}
                <div class="sidebar-list">
                  {#each filteredCommands as cmd (cmd.id)}
                    {@const isTemplate = hasCommandVariables(cmd.content)}
                    <div class="sidebar-item">
                      <div class="sidebar-item-head">
                        <div class="cmd-name-wrap">
                          <span class="cmd-name" title={cmd.name}>{cmd.name}</span>
                          {#if isTemplate}
                            <span class="var-chip mono" title="Memiliki parameter variabel dinamis">
                              {'{{}}'}
                            </span>
                          {/if}
                        </div>
                        <div class="action-links">
                          <button
                            type="button"
                            class="link-act link-act--run"
                            onclick={() => triggerSavedCommand(cmd, 'run')}
                            title="Eksekusi di terminal aktif"
                          >
                            Run
                          </button>
                          <button
                            type="button"
                            class="link-act link-act--paste"
                            onclick={() => triggerSavedCommand(cmd, 'paste')}
                            title="Tempel teks ke terminal aktif"
                          >
                            Paste
                          </button>
                          <button
                            type="button"
                            class="link-act link-act--icon"
                            onclick={() => openEditCommandModal(cmd)}
                            title="Edit perintah"
                            aria-label="Edit perintah"
                          >
                            <i class="ri-pencil-line"></i>
                          </button>
                          <button
                            type="button"
                            class="link-act link-act--icon link-act--danger"
                            onclick={() => deleteCommand(cmd)}
                            title="Hapus perintah"
                            aria-label="Hapus perintah"
                          >
                            <i class="ri-delete-bin-line"></i>
                          </button>
                        </div>
                      </div>
                      <button
                        type="button"
                        class="cmd-preview mono truncate"
                        onclick={() => triggerSavedCommand(cmd, 'paste')}
                        title={`Klik untuk tempel: ${cmd.content}`}
                      >
                        {cmd.content}
                      </button>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        {/if}
      </aside>
    {/if}
  </div>

  <!-- MODAL: ISI VARIABEL TEMPLATE COMMAND -->
  <Modal
    open={showVarModal}
    title={pendingTemplateCmd ? `Parameter: ${pendingTemplateCmd.name}` : 'Isi Variabel Perintah'}
    size="md"
    onclose={() => (showVarModal = false)}
  >
    <div class="modal-form-stack">
      {#each pendingTemplateVars as v (v.name)}
        <div class="form-row">
          <label class="form-label mono" for={`tpl-var-${v.name}`}>{v.name}</label>
          <input
            id={`tpl-var-${v.name}`}
            type="text"
            class="input mono"
            bind:value={pendingVarValues[v.name]}
            placeholder={v.defaultValue ? `Default: ${v.defaultValue}` : `Masukkan nilai ${v.name}`}
            onkeydown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                confirmTemplateCommand();
              }
            }}
          />
        </div>
      {/each}

      <div class="var-preview-box">
        <span class="var-preview-label">Pratinjau perintah:</span>
        <code class="var-preview-code mono">{resolvedTemplatePreview}</code>
      </div>
    </div>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary" onclick={() => (showVarModal = false)}>
        Batal
      </button>
      <button type="button" class="btn btn--primary" onclick={confirmTemplateCommand}>
        {pendingTemplateAction === 'run' ? 'Jalankan Perintah' : 'Tempel ke Terminal'}
      </button>
    {/snippet}
  </Modal>

  <!-- MODAL: ADD / EDIT SERVER -->
  <Modal
    open={showServerModal}
    title={editingServerId ? 'Edit server' : 'Add server'}
    size="md"
    onclose={() => (showServerModal = false)}
  >
    <div class="modal-form-stack">
      <div class="form-row">
        <label class="form-label" for="srv-ip">Server IP</label>
        <div class="ip-port-group">
          <input
            id="srv-ip"
            type="text"
            class="input mono"
            bind:value={serverFormHost}
            placeholder="Please enter server IP"
            required
          />
          <input
            type="number"
            class="input mono port-input"
            bind:value={serverFormPort}
            placeholder="22"
            min="1"
            max="65535"
          />
        </div>
      </div>

      <div class="form-row">
        <label class="form-label" for="srv-user">SSH account</label>
        <input
          id="srv-user"
          type="text"
          class="input mono"
          bind:value={serverFormUser}
          placeholder="root"
        />
      </div>

      <div class="form-row">
        <span class="form-label">Verification</span>
        <div class="tabs tabs--pill">
          <button
            type="button"
            class="tabs__tab"
            class:is-active={serverFormAuthType === 'password'}
            onclick={() => (serverFormAuthType = 'password')}
          >
            Password
          </button>
          <button
            type="button"
            class="tabs__tab"
            class:is-active={serverFormAuthType === 'key'}
            onclick={() => (serverFormAuthType = 'key')}
          >
            Private key
          </button>
        </div>
      </div>

      {#if serverFormAuthType === 'password'}
        <div class="form-row">
          <label class="form-label" for="srv-pass">Password</label>
          <div class="password-input-wrap">
            <input
              id="srv-pass"
              type={showPasswordPlain ? 'text' : 'password'}
              class="input mono"
              bind:value={serverFormPassword}
              placeholder={editingServerId ? '(Biarkan kosong jika tidak diubah)' : 'Please enter SSH password'}
            />
            <button
              type="button"
              class="eye-btn"
              onclick={() => (showPasswordPlain = !showPasswordPlain)}
              aria-label="Lihat password"
              title="Lihat password"
            >
              <i class={showPasswordPlain ? 'ri-eye-off-line' : 'ri-eye-line'}></i>
            </button>
          </div>
        </div>
      {:else}
        <div class="form-row">
          <label class="form-label" for="srv-key">Private key</label>
          <textarea
            id="srv-key"
            rows="4"
            class="input mono key-textarea"
            bind:value={serverFormPrivateKey}
            placeholder="Paste OpenSSH private key (-----BEGIN OPENSSH PRIVATE KEY-----)..."
          ></textarea>
        </div>
        <div class="form-row">
          <label class="form-label" for="srv-passphrase">Passphrase</label>
          <input
            id="srv-passphrase"
            type="password"
            class="input mono"
            bind:value={serverFormPassphrase}
            placeholder="Passphrase kunci privat (opsional jika ada)"
          />
        </div>
      {/if}

      <div class="form-row">
        <label class="form-label" for="srv-remarks">Remarks</label>
        <input
          id="srv-remarks"
          type="text"
          class="input"
          bind:value={serverFormRemarks}
          placeholder="Please enter remarks, can be blank"
        />
      </div>

      <div class="test-conn-area">
        <button
          type="button"
          class="btn btn--secondary btn--sm"
          onclick={testServerConnection}
          disabled={testingConnection}
        >
          {#if testingConnection}
            <span class="spinner spinner--sm"></span>
            <span>Testing connection...</span>
          {:else}
            <span>Test connection</span>
          {/if}
        </button>

        {#if testConnResult}
          <div
            class="badge"
            class:badge--success={testConnResult.ok}
            class:badge--danger={!testConnResult.ok}
          >
            <i class={testConnResult.ok ? 'ri-checkbox-circle-fill' : 'ri-error-warning-fill'}></i>
            <span>{testConnResult.message}</span>
          </div>
        {/if}
      </div>
    </div>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary" onclick={() => (showServerModal = false)}>
        Cancel
      </button>
      <button type="button" class="btn btn--primary" onclick={saveServer} disabled={savingServer}>
        {#if savingServer}
          <span class="spinner spinner--sm"></span>
          <span>Menyimpan...</span>
        {:else}
          <span>Confirm</span>
        {/if}
      </button>
    {/snippet}
  </Modal>

  <!-- MODAL: ADD / EDIT COMMAND -->
  <Modal
    open={showCommandModal}
    title={editingCommandId ? 'Edit command' : 'Add command'}
    size="md"
    onclose={() => (showCommandModal = false)}
  >
    <div class="modal-form-stack">
      <div class="form-row">
        <label class="form-label" for="cmd-name">Name</label>
        <input
          id="cmd-name"
          type="text"
          class="input"
          bind:value={commandFormName}
          placeholder="Please enter command name"
          required
        />
      </div>

      <div class="form-row">
        <label class="form-label" for="cmd-content">Content</label>
        <textarea
          id="cmd-content"
          rows="6"
          class="input mono content-textarea"
          bind:value={commandFormContent}
          placeholder={'Contoh: docker logs --tail {{lines:100}} {{container}}'}
          required
        ></textarea>
      </div>

      <p class="cmd-template-hint">
        Tips: gunakan <code>{'{{nama}}'}</code> atau <code>{'{{nama:default}}'}</code> untuk membuat form parameter cepat sebelum eksekusi.
      </p>
    </div>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary" onclick={() => (showCommandModal = false)}>
        Cancel
      </button>
      <button type="button" class="btn btn--primary" onclick={saveCommand} disabled={savingCommand}>
        {#if savingCommand}
          <span class="spinner spinner--sm"></span>
          <span>Menyimpan...</span>
        {:else}
          <span>Confirm</span>
        {/if}
      </button>
    {/snippet}
  </Modal>
</div>
