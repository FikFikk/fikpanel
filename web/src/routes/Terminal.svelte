<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { apiFetch } from '../lib/api';
  import { toast } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import type { TerminalServer, TerminalCommand, TerminalTab, TerminalHistoryItem, TerminalCompletionItem } from '../lib/types';
  import Modal from '../lib/components/ui/Modal.svelte';
  import '../styles/pages/terminal.css';

  // State auto-complete Tab & Tab 2x
  let completionMatches = $state<TerminalCompletionItem[]>([]);
  let tabPressCount = $state(0);
  let lastTabCommand = $state('');
  let completing = $state(false);

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

  // State multi-tab terminal
  let tabs = $state<TerminalTab[]>([
    {
      id: 'local-1',
      serverId: 0,
      name: 'Local server',
      host: 'fikpanel',
      username: 'root',
      cwd: '~',
      connected: true,
      commandInput: '',
      historyIndex: -1,
      executing: false,
      history: [
        {
          command: 'uname -a',
          output: 'Linux fikpanel-node 6.8.0-45-generic #45-Ubuntu SMP PREEMPT_DYNAMIC x86_64 GNU/Linux',
          time: '12:00:00',
          ok: true,
        },
        {
          command: 'uptime',
          output: ' 15:30:00 up 14 days,  3 users,  load average: 0.12, 0.08, 0.05',
          time: '12:00:01',
          ok: true,
        },
      ],
    },
  ]);
  let activeTabId = $state('local-1');

  // Tab aktif saat ini
  let activeTab = $derived(tabs.find((t) => t.id === activeTabId) ?? tabs[0]);

  // Daftar perintah tersaring berdasarkan input pencarian
  let filteredCommands = $derived(
    commands.filter((c) =>
      c.name.toLowerCase().includes(commandSearch.toLowerCase()) ||
      c.content.toLowerCase().includes(commandSearch.toLowerCase())
    )
  );

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
  let testConnResult = $state<{ ok: boolean; message: string } | null>(null);
  let savingServer = $state(false);

  // State Modal Add/Edit Command
  let showCommandModal = $state(false);
  let editingCommandId = $state<number | null>(null);
  let commandFormName = $state('');
  let commandFormContent = $state('');
  let savingCommand = $state(false);

  // Status pengujian koneksi inline di tabel server
  let testingServerId = $state<number | null>(null);

  // Elemen scroll terminal untuk autoscroll
  let terminalScrollEl = $state<HTMLDivElement | null>(null);
  let inputPromptEl = $state<HTMLInputElement | null>(null);

  // State prompt interaktif untuk perintah CLI bertahap (misal: php artisan make:filament-user)
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
      // Pintasan Ctrl+L untuk membersihkan terminal aktif
      if (e.ctrlKey && e.key.toLowerCase() === 'l') {
        e.preventDefault();
        clearActiveTerminal();
        return;
      }
      // Tombol Escape untuk menutup modal atau menu
      if (e.key === 'Escape') {
        if (showServerModal) showServerModal = false;
        if (showCommandModal) showCommandModal = false;
        if (showAddTabMenu) showAddTabMenu = false;
        return;
      }
      // Otomatis arahkan fokus ke input terminal saat pengguna mulai mengetik di halaman Terminal
      if (!showServerModal && !showCommandModal && inputPromptEl) {
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

    window.addEventListener('keydown', handleGlobalKeydown);
    return () => {
      window.removeEventListener('keydown', handleGlobalKeydown);
    };
  });

  // Ambil daftar server dari backend
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

  // Ambil daftar perintah dari backend
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

  // Gulir ke bawah pada layar terminal
  async function scrollToBottom() {
    await tick();
    if (terminalScrollEl) {
      terminalScrollEl.scrollTop = terminalScrollEl.scrollHeight;
    }
  }

  // Fokus kembali ke input command setelah siklus update DOM Svelte selesai
  async function focusInput() {
    if (inputPromptEl) {
      inputPromptEl.focus();
    }
    await tick();
    if (inputPromptEl) {
      inputPromptEl.focus();
      requestAnimationFrame(() => {
        inputPromptEl?.focus();
      });
    }
  }

  // Reset state kandidat auto-complete
  function resetCompletionState() {
    completionMatches = [];
    tabPressCount = 0;
  }

  // Inisialisasi direktori kerja awal (cwd) pada tab tanpa mengotori riwayat perintah
  async function initTabSession(tab: TerminalTab) {
    try {
      const res = await apiFetch<{
        ok: boolean;
        cwd?: string;
      }>('/api/terminal/complete', {
        method: 'POST',
        body: JSON.stringify({
          command: '',
          cwd: tab.cwd && tab.cwd !== '~' ? tab.cwd : '',
          server_id: tab.serverId,
        }),
      });
      if (res.cwd) {
        tab.cwd = res.cwd;
      }
    } catch {
      // Abaikan kegagalan inisialisasi direktori awal
    }
  }

  // Jalankan perintah di tab aktif
  async function runCommand(cmdText: string) {
    const trimmed = cmdText.trim();
    if (!activeTab || activeTab.executing) return;

    const currentTab = activeTab;
    resetCompletionState();

    // Jika sedang berada dalam mode prompt interaktif (make:filament-user), tangkap jawaban per langkah
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
        await runCommand(finalCmd);
        return;
      }
    }

    if (!trimmed) return;

    // Intersep php artisan make:filament-user tanpa flag agar memunculkan prompt interaktif bertahap
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

    // Tangani perintah clear/cls langsung di sisi klien seperti shell asli
    if (trimmed === 'clear' || trimmed === 'cls') {
      currentTab.history = [];
      currentTab.commandInput = '';
      currentTab.historyIndex = -1;
      focusInput();
      return;
    }

    currentTab.executing = true;
    const executedAtCwd = currentTab.cwd || '~';
    currentTab.commandInput = '';
    currentTab.historyIndex = -1;
    focusInput();

    try {
      const payload: { command: string; server_id?: number; cwd?: string } = {
        command: trimmed,
        cwd: currentTab.cwd && currentTab.cwd !== '~' ? currentTab.cwd : '',
      };
      if (currentTab.serverId > 0) {
        payload.server_id = currentTab.serverId;
      }

      const res = await apiFetch<{
        ok: boolean;
        command: string;
        output: string;
        cwd?: string;
        prompt_user?: string;
        prompt_host?: string;
      }>('/api/terminal/exec', {
        method: 'POST',
        body: JSON.stringify(payload),
        timeoutMs: 600_000,
      });

      const now = new Date().toLocaleTimeString();
      currentTab.history = [
        ...currentTab.history,
        {
          command: trimmed,
          output: res.output,
          time: now,
          ok: res.ok,
          cwd: executedAtCwd,
        },
      ];

      if (res.cwd) currentTab.cwd = res.cwd;
      if (res.prompt_user) currentTab.username = res.prompt_user;
      if (res.prompt_host) currentTab.host = res.prompt_host;
      currentTab.connected = true;

      currentTab.commandInput = '';
      currentTab.historyIndex = -1;
    } catch (err: any) {
      const now = new Date().toLocaleTimeString();
      currentTab.history = [
        ...currentTab.history,
        {
          command: trimmed,
          output: `Error: ${err.message || 'Gagal mengeksekusi perintah'}`,
          time: now,
          ok: false,
          cwd: executedAtCwd,
        },
      ];
      currentTab.connected = false;
    } finally {
      currentTab.executing = false;
      scrollToBottom();
      focusInput();
    }
  }

  // Cetak daftar kandidat ke riwayat terminal layaknya ls -la pada Tab 2x
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

  // Tangani auto-complete Tab (1x = lengkapi otomatis / LCP, 2x = cetak ls -la kandidat)
  async function handleTabAutocomplete() {
    if (!activeTab || activeTab.executing || completing) return;
    const currentTab = activeTab;

    // Jika pengguna menekan Tab ke-2 kalinya pada prefix ambigu yang sama -> cetak daftar ls -la
    if (
      tabPressCount >= 1 &&
      currentTab.commandInput === lastTabCommand &&
      completionMatches.length > 1
    ) {
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
          cwd: currentTab.cwd && currentTab.cwd !== '~' ? currentTab.cwd : '',
          server_id: currentTab.serverId,
        }),
      });

      if (res.cwd) {
        currentTab.cwd = res.cwd;
      }

      const matches = res.matches || [];
      if (matches.length === 0) {
        resetCompletionState();
      } else if (matches.length === 1) {
        // 1 kandidat tunggal -> langsung lengkapi otomatis (misal: cd da -> cd data/)
        currentTab.commandInput = res.completed_command;
        lastTabCommand = res.completed_command;
        resetCompletionState();
      } else {
        // Lebih dari 1 kandidat -> lengkapi sampai Longest Common Prefix (LCP), siapkan Tab ke-2
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

  // Tangani tombol Tab, Ctrl+C, Enter, dan Panah Atas/Bawah (seperti bash shell asli)
  function handleInputKeydown(e: KeyboardEvent) {
    if (!activeTab) return;

    if (e.key === 'Tab') {
      e.preventDefault();
      handleTabAutocomplete();
      return;
    }

    // Pintasan Ctrl+C untuk membatalkan baris perintah aktif jika tidak ada teks yang diblok
    if (e.ctrlKey && e.key.toLowerCase() === 'c') {
      const selectedText = window.getSelection()?.toString() || '';
      if (!selectedText) {
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
        activeTab.historyIndex = -1;
        interactivePrompt = null;
        resetCompletionState();
        scrollToBottom();
        return;
      }
    }

    if (e.key === 'Enter') {
      e.preventDefault();
      runCommand(activeTab.commandInput);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      resetCompletionState();
      const pastCommands = activeTab.history
        .filter((h) => !h.isCompletionList && !h.command.endsWith('^C'))
        .map((h) => h.command);
      if (pastCommands.length === 0) return;

      if (activeTab.historyIndex === -1) {
        activeTab.historyIndex = pastCommands.length - 1;
      } else if (activeTab.historyIndex > 0) {
        activeTab.historyIndex--;
      }
      activeTab.commandInput = pastCommands[activeTab.historyIndex];
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      resetCompletionState();
      const pastCommands = activeTab.history
        .filter((h) => !h.isCompletionList && !h.command.endsWith('^C'))
        .map((h) => h.command);
      if (activeTab.historyIndex !== -1) {
        if (activeTab.historyIndex < pastCommands.length - 1) {
          activeTab.historyIndex++;
          activeTab.commandInput = pastCommands[activeTab.historyIndex];
        } else {
          activeTab.historyIndex = -1;
          activeTab.commandInput = '';
        }
      }
    } else if (e.key === 'Escape') {
      resetCompletionState();
    } else if (!e.ctrlKey && !e.altKey && !e.metaKey && e.key.length === 1) {
      // Reset hitungan Tab saat pengguna mengetik karakter baru
      tabPressCount = 0;
    }
  }

  // Bersihkan layar output tab aktif
  function clearActiveTerminal() {
    if (activeTab) {
      activeTab.history = [];
      resetCompletionState();
      toast.info('Layar terminal dibersihkan');
      focusInput();
    }
  }

  // Buka tab baru untuk server lokal
  function openLocalTab() {
    const newId = `local-${Date.now()}`;
    const newTab: TerminalTab = {
      id: newId,
      serverId: 0,
      name: `Local server ${tabs.filter((t) => t.serverId === 0).length + 1}`,
      host: 'fikpanel',
      username: 'root',
      cwd: '~',
      connected: true,
      commandInput: '',
      historyIndex: -1,
      executing: false,
      history: [],
    };
    tabs = [...tabs, newTab];
    activeTabId = newId;
    showAddTabMenu = false;
    resetCompletionState();
    initTabSession(newTab);
    focusInput();
  }

  // Buka tab koneksi ke server remote
  function connectToServer(srv: TerminalServer) {
    // Cek apakah tab untuk server ini sudah dibuka
    const existing = tabs.find((t) => t.serverId === srv.id);
    if (existing) {
      activeTabId = existing.id;
      toast.info(`Beralih ke tab: ${srv.name || srv.host}`);
      showAddTabMenu = false;
      resetCompletionState();
      focusInput();
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
      connected: true,
      commandInput: '',
      historyIndex: -1,
      executing: false,
      history: [
        {
          command: '# Inisialisasi Sesi SSH',
          output: `Terhubung ke remote node: ${srv.username}@${srv.host}:${srv.port} (${srv.remarks || 'SSH Agent'})\nKetik perintah atau pilih dari panel Commands samping.`,
          time: new Date().toLocaleTimeString(),
          ok: true,
          cwd: '~',
        },
      ],
    };

    tabs = [...tabs, newTab];
    activeTabId = newId;
    showAddTabMenu = false;
    resetCompletionState();
    initTabSession(newTab);
    toast.success(`Terhubung ke ${srv.name || srv.host}`);
    focusInput();
  }

  // Tutup tab
  function closeTab(tabId: string, event: MouseEvent) {
    event.stopPropagation();
    if (tabs.length <= 1) {
      toast.warning('Minimal satu tab terminal harus tetap aktif');
      return;
    }

    const idx = tabs.findIndex((t) => t.id === tabId);
    tabs = tabs.filter((t) => t.id !== tabId);

    if (activeTabId === tabId) {
      const nextTab = tabs[Math.max(0, idx - 1)];
      activeTabId = nextTab.id;
    }
  }

  // Tempel perintah ke prompt input
  function pasteCommand(content: string) {
    if (activeTab) {
      activeTab.commandInput = content;
      toast.info(`Perintah ditempel ke input prompt`);
      focusInput();
    }
  }

  // Buka modal Tambah Server Baru
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

  // Buka modal Edit Server
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

  // Uji koneksi SSH dari modal
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
        message?: string;
        error?: string;
      }>('/api/terminal/servers/test', {
        method: 'POST',
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        testConnResult = {
          ok: true,
          message: `${res.message || 'Koneksi berhasil'} (Latensi: ${res.latency_ms || 0} ms) — ${res.banner || 'OK'}`,
        };
        toast.success(`Koneksi SSH berhasil (${res.latency_ms} ms)`);
      } else {
        testConnResult = {
          ok: false,
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

  // Uji koneksi SSH langsung dari baris tabel server
  async function testServerRow(srv: TerminalServer) {
    testingServerId = srv.id;
    try {
      const res = await apiFetch<{
        ok: boolean;
        latency_ms?: number;
        message?: string;
        banner?: string;
        error?: string;
      }>('/api/terminal/servers/test', {
        method: 'POST',
        body: JSON.stringify({ server_id: srv.id }),
      });

      if (res.ok) {
        toast.success(`Server [${srv.host}]: Koneksi normal (${res.latency_ms} ms)`);
      } else {
        toast.error(`Server [${srv.host}]: ${res.error || 'Koneksi terputus'}`);
      }
    } catch (err: any) {
      toast.error(`Gagal menguji server: ${err.message}`);
    } finally {
      testingServerId = null;
    }
  }

  // Simpan data server (Add atau Update)
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
        toast.success('Konfigurasi server berhasil diperbarui');
      } else {
        await apiFetch('/api/terminal/servers', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        toast.success('Server remote berhasil ditambahkan');
      }

      showServerModal = false;
      await fetchServers();
    } catch (err: any) {
      toast.error(`Gagal menyimpan server: ${err.message}`);
    } finally {
      savingServer = false;
    }
  }

  // Hapus server dengan konfirmasi
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

  // Buka modal Tambah Perintah
  function openAddCommandModal() {
    editingCommandId = null;
    commandFormName = '';
    commandFormContent = '';
    showCommandModal = true;
  }

  // Buka modal Edit Perintah
  function openEditCommandModal(cmd: TerminalCommand) {
    editingCommandId = cmd.id;
    commandFormName = cmd.name;
    commandFormContent = cmd.content;
    showCommandModal = true;
  }

  // Simpan perintah (Add atau Update)
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

  // Hapus perintah
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

  // Toggle layar penuh
  function toggleFullscreen() {
    isFullscreen = !isFullscreen;
    if (isFullscreen && terminalBoxEl) {
      if (terminalBoxEl.requestFullscreen) {
        terminalBoxEl.requestFullscreen().catch(() => {});
      }
    } else if (document.fullscreenElement) {
      document.exitFullscreen().catch(() => {});
    }
  }
</script>

<div class="pg-terminal terminal-page">
  <!-- Kontainer Utama Terminal & Split Panel -->
  <div
    class="terminal-layout"
    class:fullscreen={isFullscreen}
    bind:this={terminalBoxEl}
  >
    <!-- Kolom Kiri: Layar Terminal & Tab Bar -->
    <div class="terminal-main">
      <!-- Tab Bar atas (gaya aaPanel) -->
      <div class="terminal-tab-bar">
        <div class="tabs-scroll">
          {#each tabs as tab (tab.id)}
            <div
              class="term-tab"
              class:active={tab.id === activeTabId}
              onclick={() => {
                activeTabId = tab.id;
                focusInput();
              }}
              onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  activeTabId = tab.id;
                  focusInput();
                }
              }}
              role="tab"
              tabindex="0"
              aria-selected={tab.id === activeTabId}
            >
              <span
                class="dot"
                class:dot--success={tab.connected && !tab.executing}
                class:dot--danger={!tab.connected}
                class:dot--warning={tab.executing}
              ></span>
              <span class="tab-label" title={`${tab.name} (${tab.host})`}>
                {tab.name}
              </span>
              {#if tabs.length > 1}
                <button
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
          <div class="add-tab-wrapper">
            <button
              class="add-tab-btn"
              onclick={() => (showAddTabMenu = !showAddTabMenu)}
              title="Tambah sesi terminal / hubungkan server"
              aria-label="Tambah terminal"
            >
              <i class="ri-add-line"></i>
            </button>

            <!-- Dropdown Popover Pemilihan Tab -->
            {#if showAddTabMenu}
              <div class="add-tab-menu">
                <button class="menu-item" onclick={openLocalTab}>
                  <i class="ri-terminal-box-line menu-icon"></i>
                  <span>Terminal Lokal Baru</span>
                </button>

                {#if servers.length > 0}
                  <div class="menu-divider"></div>
                  <div class="menu-header">Hubungkan Server Remote:</div>
                  {#each servers as srv}
                    <button class="menu-item" onclick={() => connectToServer(srv)}>
                      <i class="ri-server-line menu-icon"></i>
                      <div class="menu-item-text">
                        <span class="srv-title">{srv.name || srv.host}</span>
                        <span class="srv-sub">{srv.username}@{srv.host}:{srv.port}</span>
                      </div>
                    </button>
                  {/each}
                {/if}

                <div class="menu-divider"></div>
                <button class="menu-item menu-item--primary" onclick={openAddServerModal}>
                  <i class="ri-add-circle-line menu-icon"></i>
                  <span>+ Tambah Server Baru...</span>
                </button>
              </div>
            {/if}
          </div>
        </div>

        <!-- Tombol Aksi Kanan Tab Bar -->
        <div class="tab-bar-actions">
          <button
            class="action-icon-btn"
            onclick={clearActiveTerminal}
            title="Bersihkan layar (Ctrl+L)"
            aria-label="Bersihkan layar"
          >
            <i class="ri-delete-bin-7-line"></i>
          </button>
          <button
            class="action-icon-btn"
            onclick={toggleFullscreen}
            title={isFullscreen ? 'Keluar layar penuh' : 'Layar penuh'}
            aria-label="Layar penuh"
          >
            <i class={isFullscreen ? 'ri-fullscreen-exit-line' : 'ri-fullscreen-line'}></i>
          </button>
        </div>
      </div>

      <!-- Area Tampilan Konsol Terminal -->
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
      <div
        class="terminal-viewport"
        bind:this={terminalScrollEl}
        onclick={focusInput}
        role="region"
        aria-label="Layar konsol terminal"
      >
        <!-- Banner Selamat Datang Ubuntu / Sesi -->
        <div class="terminal-welcome">
          <p class="welcome-line">
            <span class="text-accent">fikpanel v0.3.13</span> (SSH PTY Console Bridge &middot; {activeTab?.name})
          </p>
          <p class="welcome-meta">
            Host: <span class="text-fg">{activeTab?.host}</span> &middot;
            User: <span class="text-fg">{activeTab?.username}</span> &middot;
            Tipe: <span class="text-accent">{activeTab?.serverId === 0 ? 'Local Node' : 'Remote SSH Node'}</span>
          </p>
          <p class="welcome-hint">
            Ketik perintah shell di bawah atau klik <b>Run / Paste</b> pada panel pintasan Commands.
          </p>
        </div>

        <!-- Riwayat Eksekusi Perintah -->
        {#if activeTab}
          {#each activeTab.history as item}
            <div class="term-entry">
              <div class="prompt-line">
                <span class="prompt-user">{activeTab.username}@{activeTab.host}</span>:<span class="prompt-path">{item.cwd || activeTab.cwd || '~'}</span><span class="prompt-hash">#</span>
                <span class="prompt-cmd">{item.command}</span>
                <span class="prompt-time">{item.time}</span>
              </div>
              {#if item.output}
                <pre class="term-output" class:term-output--error={!item.ok}>{item.output}</pre>
              {/if}
            </div>
          {/each}

          <!-- Baris Input Prompt Aktif -->
          <div class="input-line">
            {#if interactivePrompt}
              <span class="prompt-user">
                {interactivePrompt.step === 'name'
                  ? '┌ Name (default: Super Admin):'
                  : interactivePrompt.step === 'email'
                    ? '├ Email address (default: admin@fikfikk.my.id):'
                    : '└ Password (default: admin123):'}
              </span>
            {:else}
              <span class="prompt-user">{activeTab.username}@{activeTab.host}</span>:<span class="prompt-path">{activeTab.cwd || '~'}</span><span class="prompt-hash">#</span>
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
            {#if activeTab.executing || completing}
              <span class="spinner spinner--sm"></span>
            {/if}
          </div>
        {/if}
      </div>
    </div>

    <!-- Tombol Pembagi Collapse / Expand Sidebar Kanan -->
    <button
      class="sidebar-toggle-btn"
      class:collapsed={!sidebarOpen}
      onclick={() => (sidebarOpen = !sidebarOpen)}
      title={sidebarOpen ? 'Sembunyikan panel samping' : 'Tampilkan panel samping'}
      aria-label="Toggle panel samping"
    >
      <i class={sidebarOpen ? 'ri-arrow-right-s-line' : 'ri-arrow-left-s-line'}></i>
    </button>

    <!-- Kolom Kanan: Server List & Commands -->
    {#if sidebarOpen}
      <aside class="terminal-sidebar">
        <!-- Tab Navigasi Sidebar (Server List & Commands) -->
        <div class="tabs tabs--underline sidebar-tabs-nav">
          <button
            class="tabs__tab"
            class:is-active={activeSidebarTab === 'servers'}
            onclick={() => (activeSidebarTab = 'servers')}
          >
            Server List
          </button>
          <button
            class="tabs__tab"
            class:is-active={activeSidebarTab === 'commands'}
            onclick={() => (activeSidebarTab = 'commands')}
          >
            Commands
          </button>
        </div>

        <!-- Konten Sidebar: SERVER LIST -->
        {#if activeSidebarTab === 'servers'}
          <div class="sidebar-content">
            <!-- Header Aksi: Add Server -->
            <div class="sidebar-actions-bar">
              <button class="btn btn--primary btn--sm" onclick={openAddServerModal}>
                <i class="ri-add-line"></i> Add server
              </button>
            </div>

            <!-- Tabel Daftar Server -->
            <div class="table-container">
              {#if loadingServers}
                <div class="state state--loading">
                  <span class="spinner"></span>
                  <span>Memuat daftar server...</span>
                </div>
              {:else if servers.length === 0}
                <div class="state state--empty">
                  <i class="ri-inbox-2-line state__icon"></i>
                  <span class="state__title">No Data</span>
                  <span class="state__desc">Belum ada remote server tersimpan.</span>
                </div>
              {:else}
                <table class="sidebar-table">
                  <thead>
                    <tr>
                      <th class="th-left">Server IP</th>
                      <th class="th-right">Operate</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each servers as srv (srv.id)}
                      <tr>
                        <td class="td-server-info">
                          <div class="srv-name-row">
                            <span class="srv-ip mono">{srv.host}:{srv.port}</span>
                            {#if srv.remarks}
                              <span class="badge badge--sm badge--secondary truncate max-w-100" title={srv.remarks}>
                                {srv.remarks}
                              </span>
                            {/if}
                          </div>
                          <span class="srv-user mono text-muted">{srv.username} &middot; {srv.auth_type}</span>
                        </td>
                        <td class="td-actions">
                          <div class="action-links">
                            <button
                              class="link-act link-act--connect"
                              onclick={() => connectToServer(srv)}
                              title="Buka sesi terminal di tab baru"
                            >
                              Connect
                            </button>
                            <button
                              class="link-act"
                              onclick={() => testServerRow(srv)}
                              disabled={testingServerId === srv.id}
                              title="Uji koneksi SSH"
                            >
                              {testingServerId === srv.id ? 'Testing...' : 'Test'}
                            </button>
                            <button
                              class="link-act"
                              onclick={() => openEditServerModal(srv)}
                              title="Edit konfigurasi"
                            >
                              Edit
                            </button>
                            <button
                              class="link-act link-act--danger"
                              onclick={() => deleteServer(srv)}
                              title="Hapus server"
                            >
                              Delete
                            </button>
                          </div>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              {/if}
            </div>
          </div>
        {/if}

        <!-- Konten Sidebar: COMMANDS -->
        {#if activeSidebarTab === 'commands'}
          <div class="sidebar-content">
            <!-- Header Aksi: Add command & Search Input -->
            <div class="sidebar-actions-bar">
              <button class="btn btn--primary btn--sm" onclick={openAddCommandModal}>
                <i class="ri-add-line"></i> Add command
              </button>
              <div class="search-input-wrap">
                <input
                  type="text"
                  bind:value={commandSearch}
                  placeholder="Command name"
                  class="search-input"
                />
                <i class="ri-search-line search-icon"></i>
              </div>
            </div>

            <!-- Tabel Daftar Command Shortcut -->
            <div class="table-container">
              {#if loadingCommands}
                <div class="state state--loading">
                  <span class="spinner"></span>
                  <span>Memuat pintasan...</span>
                </div>
              {:else if filteredCommands.length === 0}
                <div class="state state--empty">
                  <i class="ri-inbox-2-line state__icon"></i>
                  <span class="state__title">No Data</span>
                  <span class="state__desc">Tidak ada perintah yang cocok.</span>
                </div>
              {:else}
                <table class="sidebar-table">
                  <thead>
                    <tr>
                      <th class="th-left">Command name &darr;</th>
                      <th class="th-right">Operate</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each filteredCommands as cmd (cmd.id)}
                      <tr>
                        <td class="td-cmd-info">
                          <div class="cmd-name" title={cmd.content}>
                            {cmd.name}
                          </div>
                          <div class="cmd-preview mono truncate" title={cmd.content}>
                            {cmd.content}
                          </div>
                        </td>
                        <td class="td-actions">
                          <div class="action-links">
                            <button
                              class="link-act link-act--run"
                              onclick={() => runCommand(cmd.content)}
                              title="Langsung eksekusi di terminal aktif"
                            >
                              Run
                            </button>
                            <button
                              class="link-act link-act--paste"
                              onclick={() => pasteCommand(cmd.content)}
                              title="Tempel teks ke input terminal"
                            >
                              Paste
                            </button>
                            <button
                              class="link-act"
                              onclick={() => openEditCommandModal(cmd)}
                              title="Edit perintah"
                            >
                              Edit
                            </button>
                            <button
                              class="link-act link-act--danger"
                              onclick={() => deleteCommand(cmd)}
                              title="Hapus perintah"
                            >
                              Delete
                            </button>
                          </div>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              {/if}
            </div>
          </div>
        {/if}
      </aside>
    {/if}
  </div>

  <!-- MODAL: ADD / EDIT SERVER -->
  <Modal
    open={showServerModal}
    title={editingServerId ? 'Edit server' : 'Add server'}
    size="md"
    onclose={() => (showServerModal = false)}
  >
    <div class="modal-form-stack">
      <!-- Baris 1: Server IP & Port -->
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

      <!-- Baris 2: SSH account -->
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

      <!-- Baris 3: Verification (Pill switcher: Password / Private key) -->
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

      <!-- Baris 4: Password atau Private Key -->
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

      <!-- Baris 5: Remarks -->
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

      <!-- Tombol Test Connection -->
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
      <button
        type="button"
        class="btn btn--secondary"
        onclick={() => (showServerModal = false)}
      >
        Cancel
      </button>
      <button
        type="button"
        class="btn btn--primary"
        onclick={saveServer}
        disabled={savingServer}
      >
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
          placeholder="Please enter command content"
          required
        ></textarea>
      </div>
    </div>

    {#snippet footer()}
      <button
        type="button"
        class="btn btn--secondary"
        onclick={() => (showCommandModal = false)}
      >
        Cancel
      </button>
      <button
        type="button"
        class="btn btn--primary"
        onclick={saveCommand}
        disabled={savingCommand}
      >
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