<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { apiFetch } from '../lib/api';
  import { toast } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import type { TerminalServer, TerminalCommand, TerminalTab, TerminalHistoryItem, TerminalCompletionItem } from '../lib/types';

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

<div class="terminal-page">
  <!-- Container Utama Terminal & Split Panel -->
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
              role="tab"
              tabindex="0"
              aria-selected={tab.id === activeTabId}
            >
              <span
                class="status-dot"
                class:online={tab.connected}
                class:offline={!tab.connected}
                class:running={tab.executing}
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
            <span class="text-accent">fikpanel v0.2.0</span> (SSH PTY Console Bridge &middot; {activeTab?.name})
          </p>
          <p class="welcome-meta">
            Host: <span class="text-white">{activeTab?.host}</span> &middot;
            User: <span class="text-white">{activeTab?.username}</span> &middot;
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

    <!-- Kolom Kanan: Server List & Commands (Sesuai Screenshot 1 & 4) -->
    {#if sidebarOpen}
      <aside class="terminal-sidebar">
        <!-- Tab Navigasi Sidebar (Server List & Commands) -->
        <div class="sidebar-tabs">
          <button
            class="sidebar-tab-btn"
            class:active={activeSidebarTab === 'servers'}
            onclick={() => (activeSidebarTab = 'servers')}
          >
            Server List
          </button>
          <button
            class="sidebar-tab-btn"
            class:active={activeSidebarTab === 'commands'}
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
                <div class="sidebar-loading">
                  <span class="spinner"></span>
                  <span>Memuat daftar server...</span>
                </div>
              {:else if servers.length === 0}
                <div class="empty-state">
                  <i class="ri-inbox-2-line empty-icon"></i>
                  <span class="empty-title">No Data</span>
                  <span class="empty-desc">Belum ada remote server tersimpan.</span>
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
            <!-- Header Aksi: Add command & Search Input (Sesuai Gambar 4) -->
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
                <div class="sidebar-loading">
                  <span class="spinner"></span>
                  <span>Memuat pintasan...</span>
                </div>
              {:else if filteredCommands.length === 0}
                <div class="empty-state">
                  <i class="ri-inbox-2-line empty-icon"></i>
                  <span class="empty-title">No Data</span>
                  <span class="empty-desc">Tidak ada perintah yang cocok.</span>
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
</div>

<!-- ═══════════════════════════════════════════════════════════════════════════ -->
<!-- MODAL: ADD / EDIT SERVER (Persis Gambar 2) -->
<!-- ═══════════════════════════════════════════════════════════════════════════ -->
{#if showServerModal}
  <div class="modal-backdrop" onclick={() => (showServerModal = false)} role="presentation">
    <div class="modal-panel" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
      <div class="modal-header">
        <h3 class="modal-title">{editingServerId ? 'Edit server' : 'Add server'}</h3>
        <button
          class="modal-close-btn"
          onclick={() => (showServerModal = false)}
          aria-label="Tutup modal"
        >
          <i class="ri-close-line"></i>
        </button>
      </div>

      <div class="modal-body">
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
          <div class="verification-switcher">
            <button
              type="button"
              class="switch-pill"
              class:active={serverFormAuthType === 'password'}
              onclick={() => (serverFormAuthType = 'password')}
            >
              Password
            </button>
            <button
              type="button"
              class="switch-pill"
              class:active={serverFormAuthType === 'key'}
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
              class="test-badge"
              class:test-badge--success={testConnResult.ok}
              class:test-badge--error={!testConnResult.ok}
            >
              <i class={testConnResult.ok ? 'ri-checkbox-circle-fill' : 'ri-error-warning-fill'}></i>
              <span>{testConnResult.message}</span>
            </div>
          {/if}
        </div>
      </div>

      <div class="modal-footer">
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
      </div>
    </div>
  </div>
{/if}

<!-- ═══════════════════════════════════════════════════════════════════════════ -->
<!-- MODAL: ADD / EDIT COMMAND (Persis Gambar 3) -->
<!-- ═══════════════════════════════════════════════════════════════════════════ -->
{#if showCommandModal}
  <div class="modal-backdrop" onclick={() => (showCommandModal = false)} role="presentation">
    <div class="modal-panel modal-panel--md" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
      <div class="modal-header">
        <h3 class="modal-title">{editingCommandId ? 'Edit command' : 'Add command'}</h3>
        <button
          class="modal-close-btn"
          onclick={() => (showCommandModal = false)}
          aria-label="Tutup modal"
        >
          <i class="ri-close-line"></i>
        </button>
      </div>

      <div class="modal-body">
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

      <div class="modal-footer">
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
      </div>
    </div>
  </div>
{/if}

<style>
  /* ── Layout Kontainer Utama ────────────────────────────────────────────── */
  .terminal-page {
    display: flex;
    flex-direction: column;
    height: calc(100vh - 120px);
    min-height: 580px;
  }

  .terminal-layout {
    display: flex;
    flex: 1;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    overflow: hidden;
    position: relative;
    box-shadow: var(--shadow-md);
  }

  .terminal-layout.fullscreen {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: var(--z-modal);
    border-radius: 0;
    border: none;
    height: 100vh;
  }

  /* ── Area Terminal Utama ───────────────────────────────────────────────── */
  .terminal-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--bg-inset);
  }

  /* ── Tab Bar Gaya aaPanel ──────────────────────────────────────────────── */
  .terminal-tab-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border);
    padding: 0 var(--space-2);
    height: 38px;
    user-select: none;
  }

  .tabs-scroll {
    display: flex;
    align-items: center;
    gap: 4px;
    overflow-x: auto;
    scrollbar-width: none;
    flex: 1;
    min-width: 0;
  }
  .tabs-scroll::-webkit-scrollbar {
    display: none;
  }

  .term-tab {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background: var(--bg-raised);
    color: var(--text-muted);
    border: 1px solid var(--border);
    border-bottom: none;
    border-radius: var(--radius-sm) var(--radius-sm) 0 0;
    padding: 0.35rem 0.75rem;
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    cursor: pointer;
    white-space: nowrap;
    max-width: 180px;
    transition: background var(--duration-fast) ease, color var(--duration-fast) ease;
  }
  .term-tab:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .term-tab.active {
    background: var(--bg-inset);
    color: var(--text);
    border-color: var(--border-strong);
    border-top: 2px solid var(--accent);
  }

  .tab-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .tab-close-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 2px;
    border-radius: 3px;
    margin-left: 2px;
  }
  .tab-close-btn:hover {
    background: var(--tint-danger);
    color: var(--danger-strong);
  }

  /* Status Indicator Dot */
  .status-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .status-dot.online {
    background: var(--accent);
    box-shadow: 0 0 6px var(--tint-accent);
  }
  .status-dot.offline {
    background: var(--danger);
  }
  .status-dot.running {
    background: var(--warning);
    animation: fp-pulse 1.2s infinite ease-in-out;
  }

  /* Tombol + Tambah Tab */
  .add-tab-wrapper {
    position: relative;
    display: inline-block;
  }
  .add-tab-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--accent);
    font-size: 1rem;
    cursor: pointer;
    transition: background var(--duration-fast) ease, border-color var(--duration-fast) ease;
  }
  .add-tab-btn:hover {
    background: var(--tint-accent);
    border-color: var(--accent);
  }

  /* Popover Menu Tambah Tab */
  .add-tab-menu {
    position: absolute;
    top: 32px;
    left: 0;
    z-index: var(--z-dropdown);
    min-width: 240px;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
    padding: var(--space-1) 0;
  }
  .menu-header {
    font-size: var(--text-2xs);
    text-transform: uppercase;
    color: var(--text-subtle);
    padding: var(--space-1) var(--space-3);
    letter-spacing: 0.05em;
  }
  .menu-divider {
    height: 1px;
    background: var(--border-subtle);
    margin: var(--space-1) 0;
  }
  .menu-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    background: transparent;
    border: none;
    color: var(--text-secondary);
    font-size: var(--text-xs);
    text-align: left;
    cursor: pointer;
    transition: background var(--duration-fast) ease, color var(--duration-fast) ease;
  }
  .menu-item:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .menu-item--primary {
    color: var(--accent);
    font-weight: var(--weight-semibold);
  }
  .menu-icon {
    font-size: 1rem;
  }
  .menu-item-text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .srv-title {
    font-weight: var(--weight-medium);
  }
  .srv-sub {
    font-size: var(--text-2xs);
    color: var(--text-muted);
    font-family: var(--font-mono);
  }

  /* Tombol Aksi Kanan */
  .tab-bar-actions {
    display: flex;
    align-items: center;
    gap: 4px;
    padding-left: var(--space-2);
  }
  .action-icon-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    width: 26px;
    height: 26px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--radius-sm);
    cursor: pointer;
    font-size: 0.95rem;
    transition: background var(--duration-fast) ease, color var(--duration-fast) ease;
  }
  .action-icon-btn:hover {
    background: var(--bg-hover);
    color: var(--text);
  }

  /* ── Layar Terminal Konsol ─────────────────────────────────────────────── */
  .terminal-viewport {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-4) var(--space-5);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    line-height: var(--leading-normal);
    background: var(--bg-inset);
    cursor: text;
  }

  .terminal-welcome {
    margin-bottom: var(--space-4);
    padding-bottom: var(--space-3);
    border-bottom: 1px dashed var(--border-subtle);
  }
  .welcome-line {
    font-weight: var(--weight-semibold);
    color: var(--text);
  }
  .welcome-meta {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin-top: 2px;
  }
  .welcome-hint {
    font-size: var(--text-2xs);
    color: var(--text-subtle);
    margin-top: var(--space-1);
  }

  .term-entry {
    margin-bottom: var(--space-3);
  }
  .prompt-line {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: 2px;
  }
  .prompt-user {
    color: var(--accent);
    font-weight: var(--weight-semibold);
  }
  .prompt-path {
    color: var(--violet);
    font-weight: var(--weight-semibold);
  }
  .prompt-hash {
    color: var(--text-muted);
    margin-right: var(--space-2);
  }
  .prompt-cmd {
    color: var(--text);
    font-weight: var(--weight-medium);
  }
  .prompt-time {
    color: var(--text-subtle);
    font-size: var(--text-2xs);
    margin-left: auto;
  }

  .term-output {
    margin: var(--space-1) 0 var(--space-2);
    padding-left: var(--space-2);
    color: var(--text-secondary);
    white-space: pre-wrap;
    word-break: break-all;
    font-size: var(--text-sm);
    line-height: var(--leading-normal);
  }
  .term-output--error {
    color: var(--danger-strong);
  }

  .input-line {
    display: flex;
    align-items: center;
    margin-top: var(--space-2);
    padding: var(--space-1) 0;
  }
  .input-line input {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    color: var(--text);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    margin-left: 2px;
    caret-color: var(--accent);
  }

  /* ── Tombol Pembagi Collapse/Expand Sidebar ────────────────────────────── */
  .sidebar-toggle-btn {
    width: 14px;
    background: var(--bg-surface);
    border: none;
    border-left: 1px solid var(--border);
    border-right: 1px solid var(--border);
    color: var(--text-muted);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.95rem;
    transition: background var(--duration-fast) ease, color var(--duration-fast) ease;
    padding: 0;
    z-index: 5;
  }
  .sidebar-toggle-btn:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .sidebar-toggle-btn.collapsed {
    border-right: none;
  }

  /* ── Sidebar Kanan (Server List & Commands) ────────────────────────────── */
  .terminal-sidebar {
    width: 330px;
    background: var(--bg-surface);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
  }

  .sidebar-tabs {
    display: flex;
    border-bottom: 1px solid var(--border);
    background: var(--bg-app);
  }
  .sidebar-tab-btn {
    flex: 1;
    padding: var(--space-2) var(--space-2);
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    cursor: pointer;
    text-align: center;
    transition: color var(--duration-fast) ease, border-color var(--duration-fast) ease, background var(--duration-fast) ease;
  }
  .sidebar-tab-btn:hover {
    color: var(--text);
  }
  .sidebar-tab-btn.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
    background: var(--bg-surface);
  }

  .sidebar-content {
    display: flex;
    flex-direction: column;
    flex: 1;
    overflow: hidden;
  }

  /* Header Aksi: Add Button & Search */
  .sidebar-actions-bar {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-bottom: 1px solid var(--border);
    background: var(--bg-surface);
  }

  .search-input-wrap {
    position: relative;
    flex: 1;
  }
  .search-input {
    width: 100%;
    height: 28px;
    padding: 0 var(--space-6) 0 var(--space-2);
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text);
    font-size: var(--text-xs);
  }
  .search-input:focus {
    outline: none;
    border-color: var(--accent);
  }
  .search-icon {
    position: absolute;
    right: 7px;
    top: 50%;
    transform: translateY(-50%);
    color: var(--text-subtle);
    font-size: 0.85rem;
    pointer-events: none;
  }

  /* Tabel Sidebar */
  .table-container {
    flex: 1;
    overflow-y: auto;
  }

  .sidebar-table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--text-xs);
  }
  .sidebar-table th {
    padding: var(--space-2) var(--space-3);
    background: var(--bg-raised);
    color: var(--text-subtle);
    font-weight: var(--weight-semibold);
    font-size: var(--text-2xs);
    border-bottom: 1px solid var(--border);
  }
  .th-left {
    text-align: left;
  }
  .th-right {
    text-align: right;
  }

  .sidebar-table td {
    padding: var(--space-2) var(--space-3);
    border-bottom: 1px solid var(--border-subtle);
    vertical-align: middle;
  }
  .sidebar-table tbody tr:hover {
    background: var(--bg-hover);
  }

  .td-server-info,
  .td-cmd-info {
    max-width: 170px;
  }

  .srv-name-row {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-wrap: wrap;
  }
  .srv-ip {
    color: var(--text);
    font-weight: var(--weight-medium);
  }
  .srv-user {
    font-size: var(--text-2xs);
    display: block;
    margin-top: 2px;
  }

  .cmd-name {
    color: var(--text);
    font-weight: var(--weight-semibold);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .cmd-preview {
    color: var(--text-muted);
    font-size: var(--text-2xs);
    margin-top: 2px;
  }

  .td-actions {
    text-align: right;
    white-space: nowrap;
  }
  .action-links {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    justify-content: flex-end;
  }

  .link-act {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: var(--text-2xs);
    cursor: pointer;
    padding: 2px 4px;
    border-radius: 3px;
    transition: background var(--duration-fast) ease, color var(--duration-fast) ease;
  }
  .link-act:hover {
    color: var(--text);
    background: var(--bg-hover);
  }
  .link-act--connect {
    color: var(--accent);
    font-weight: var(--weight-semibold);
  }
  .link-act--run {
    color: var(--accent);
    font-weight: var(--weight-semibold);
  }
  .link-act--paste {
    color: var(--info);
  }
  .link-act--danger:hover {
    color: var(--danger);
  }

  .sidebar-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-6) var(--space-4);
    color: var(--text-muted);
    font-size: var(--text-xs);
  }

  /* ── Modal Dialogs ─────────────────────────────────────────────────────── */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: var(--bg-overlay);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: var(--z-modal);
    padding: var(--space-4);
  }

  .modal-panel {
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    width: 100%;
    max-width: 520px;
    box-shadow: var(--shadow-lg);
    overflow: hidden;
    animation: modal-appear var(--duration-base) ease-out;
  }
  .modal-panel--md {
    max-width: 540px;
  }

  @keyframes modal-appear {
    from {
      opacity: 0;
      transform: scale(0.96) translateY(-8px);
    }
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
  }
  .modal-title {
    font-size: var(--text-lg);
    font-weight: var(--weight-semibold);
    color: var(--text);
    margin: 0;
  }
  .modal-close-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 1.25rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--radius-sm);
    padding: 2px;
  }
  .modal-close-btn:hover {
    color: var(--text);
  }

  .modal-body {
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .form-row {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }
  .form-label {
    width: 110px;
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    color: var(--text-secondary);
    flex-shrink: 0;
  }

  .ip-port-group {
    display: flex;
    gap: var(--space-2);
    flex: 1;
  }
  .port-input {
    width: 80px;
    flex-shrink: 0;
  }

  /* Segmented Pill Switcher (Password / Private Key) */
  .verification-switcher {
    display: inline-flex;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 2px;
  }
  .switch-pill {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    padding: 0.35rem 0.85rem;
    border-radius: 4px;
    cursor: pointer;
    transition: background var(--duration-fast) ease, color var(--duration-fast) ease;
  }
  .switch-pill.active {
    background: var(--accent);
    color: var(--text-on-accent);
    font-weight: var(--weight-semibold);
  }

  .password-input-wrap {
    position: relative;
    flex: 1;
  }
  .eye-btn {
    position: absolute;
    right: 8px;
    top: 50%;
    transform: translateY(-50%);
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    font-size: 0.95rem;
  }
  .eye-btn:hover {
    color: var(--text);
  }

  .key-textarea,
  .content-textarea {
    resize: vertical;
    font-size: var(--text-xs);
    line-height: var(--leading-normal);
  }

  .test-conn-area {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin-top: var(--space-1);
    padding-left: 110px;
    flex-wrap: wrap;
  }

  .test-badge {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: var(--text-2xs);
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
  }
  .test-badge--success {
    background: var(--tint-accent);
    color: var(--accent-strong);
    border: 1px solid var(--tint-accent);
  }
  .test-badge--error {
    background: var(--tint-danger);
    color: var(--danger-strong);
    border: 1px solid var(--tint-danger);
  }

  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background: var(--bg-surface);
    border-top: 1px solid var(--border);
  }

  /* Utility font & colors */
  .mono {
    font-family: var(--font-mono);
  }
  .text-white {
    color: var(--text);
  }
  .text-accent {
    color: var(--accent);
  }
  .max-w-100 {
    max-width: 100px;
  }
</style>