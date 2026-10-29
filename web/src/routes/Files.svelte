<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch, formatBytes } from '../lib/api';
  import { copyText } from '../lib/utils/browser';
  import { confirmDialog, promptDialog } from '../lib/stores/confirm.svelte';
  import type { FileItem, FileListData } from '../lib/types';

  interface Props {
    onNotify: (msg: string) => void;
  }

  let { onNotify }: Props = $props();

  // State Tab Direktori (Multi-tab aaPanel)
  interface Tab {
    id: string;
    name: string;
    path: string;
  }

  let tabs = $state<Tab[]>([
    { id: '1', name: 'root', path: '' }
  ]);
  let activeTabId = $state<string>('1');

  // State Direktori & Berkas
  let currentPath = $state('');
  let parentPath = $state('');
  let items = $state<FileItem[]>([]);
  let totalDirs = $state(0);
  let totalFiles = $state(0);
  let totalSize = $state(0);
  let loading = $state(false);

  // State Tampilan & Pencarian
  let viewMode = $state<'list' | 'grid'>('list');
  let searchQuery = $state('');
  let selectedPaths = $state<Set<string>>(new Set());

  // State Clipboard Salin / Potong
  let clipboard = $state<{ action: 'copy' | 'cut'; paths: string[]; names: string[] } | null>(null);

  // State Menu Popup Tools / Function List
  let activeMenuItem = $state<FileItem | null>(null);
  let menuPos = $state<{ top: number; left: number }>({ top: 0, left: 0 });

  // State Dropdown Menu Toolbar (Upload, New, Favorite, Operations)
  let activeToolbarMenu = $state<'upload' | 'new' | 'favorite' | 'operations' | null>(null);

  // State Pagination
  let pageSize = $state(100);
  let currentPage = $state(1);

  // State Modal-Modal Aksi
  let showNewModal = $state(false);
  let newType = $state<'file' | 'dir'>('file');
  let newName = $state('');

  let showSymlinkModal = $state(false);
  let symlinkTarget = $state('');
  let symlinkName = $state('');
  let creatingSymlink = $state(false);

  let showRemoteModal = $state(false);
  let remoteUrl = $state('');
  let remoteFilename = $state('');
  let remoteDownloading = $state(false);

  let showSearchContentModal = $state(false);
  let contentSearchQuery = $state('');
  let contentSearchResults = $state<{ file: string; line: number; text: string }[]>([]);
  let searchingContent = $state(false);

  let showFavManageModal = $state(false);
  let customFavorites = $state<{ name: string; path: string }[]>([
    { name: 'Root Aplikasi', path: '' },
    { name: 'Web Frontend (/web)', path: 'web' },
    { name: 'Backend Core (/internal)', path: 'internal' },
    { name: 'CLI Entry (/cmd)', path: 'cmd' },
  ]);

  let showCloudStorageModal = $state(false);
  let selectedCloud = $state('');

  let showRenameModal = $state(false);
  let renameTarget = $state<FileItem | null>(null);
  let renameNewName = $state('');

  let showChmodModal = $state(false);
  let chmodTarget = $state<FileItem | null>(null);
  let chmodPerms = $state({
    ownerR: true, ownerW: true, ownerX: true,
    groupR: true, groupW: false, groupX: true,
    publicR: true, publicW: false, publicX: true,
  });

  // State Editor Berkas Modern (FikPanel Code Studio)
  let showEditorModal = $state(false);
  let editorTarget = $state<FileItem | null>(null);
  let editorContent = $state('');
  let editorInitialContent = $state('');
  let editorSaving = $state(false);
  let editorFontSize = $state<number>(13);
  let editorWordWrap = $state<boolean>(false);
  let editorIsFullscreen = $state<boolean>(false);
  let editorCursorPos = $state<{ line: number; col: number }>({ line: 1, col: 1 });
  let editorShowSearch = $state<boolean>(false);
  let editorSearchQuery = $state<string>('');
  let editorReplaceQuery = $state<string>('');
  let editorSearchMatches = $state<number[]>([]);
  let editorSearchMatchIndex = $state<number>(0);
  let editorTextareaEl = $state<HTMLTextAreaElement | null>(null);
  let editorGutterEl = $state<HTMLDivElement | null>(null);

  // Derived state untuk kalkulasi nomor baris, status kotor, dan bahasa
  let editorIsDirty = $derived(editorContent !== editorInitialContent);
  let editorLineCount = $derived(editorContent ? editorContent.split('\n').length : 1);
  let editorLines = $derived(Array.from({ length: editorLineCount }, (_, i) => i + 1));
  let editorLangInfo = $derived(editorTarget ? detectFileLanguage(editorTarget.name) : { label: 'Plain Text', icon: 'ri-file-text-line', color: '#94a3b8' });

  let showUploadModal = $state(false);
  let uploadFiles = $state<FileList | null>(null);
  let uploadSubmitting = $state(false);

  let showPropertiesModal = $state(false);
  let propTarget = $state<FileItem | null>(null);

  let showCompressModal = $state(false);
  let compressTarget = $state<FileItem | null>(null);
  let compressZipName = $state('');
  let compressing = $state(false);

  // Menghitung string mode numerik dari checkbox matrix PMSN (mis. 0755)
  let chmodOctal = $derived.by(() => {
    const o = (chmodPerms.ownerR ? 4 : 0) + (chmodPerms.ownerW ? 2 : 0) + (chmodPerms.ownerX ? 1 : 0);
    const g = (chmodPerms.groupR ? 4 : 0) + (chmodPerms.groupW ? 2 : 0) + (chmodPerms.groupX ? 1 : 0);
    const p = (chmodPerms.publicR ? 4 : 0) + (chmodPerms.publicW ? 2 : 0) + (chmodPerms.publicX ? 1 : 0);
    return `0${o}${g}${p}`;
  });

  // Breadcrumb part parser untuk Windows dan Linux
  let breadcrumbs = $derived.by(() => {
    if (!currentPath) return [];
    const isWindows = currentPath.includes('\\');
    const sep = isWindows ? '\\' : '/';
    const parts = currentPath.split(/[/\\]+/).filter(Boolean);

    let runningPath = '';
    return parts.map((part, index) => {
      if (index === 0 && isWindows && part.includes(':')) {
        runningPath = part + sep;
      } else if (index === 0 && !isWindows) {
        runningPath = '/' + part;
      } else {
        runningPath = runningPath ? runningPath + (runningPath.endsWith(sep) ? '' : sep) + part : part;
      }
      return {
        label: part,
        fullPath: runningPath,
      };
    });
  });

  // Filter item berdasarkan pencarian lokal bila query terisi
  let filteredItems = $derived.by(() => {
    if (!searchQuery.trim()) return items;
    const q = searchQuery.toLowerCase();
    return items.filter(it => it.name.toLowerCase().includes(q));
  });

  // Item yang sedang aktif di halaman saat ini
  let paginatedItems = $derived.by(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredItems.slice(start, start + pageSize);
  });

  let totalPages = $derived(Math.max(1, Math.ceil(filteredItems.length / pageSize)));

  // Memuat daftar direktori dari backend
  async function loadDirectory(targetPath: string = '') {
    loading = true;
    closeAllMenus();
    selectedPaths = new Set();
    try {
      const q = encodeURIComponent(targetPath);
      const data = await apiFetch<FileListData>(`/api/files/list?path=${q}`);
      currentPath = data.current_path;
      parentPath = data.parent_path;
      items = data.items || [];
      totalDirs = data.total_dirs;
      totalFiles = data.total_files;
      totalSize = data.total_size;
      currentPage = 1;

      // Update tab aktif saat ini
      const activeTab = tabs.find(t => t.id === activeTabId);
      if (activeTab) {
        activeTab.path = currentPath;
        const base = currentPath.split(/[/\\]+/).filter(Boolean).pop() || 'root';
        activeTab.name = base;
      }
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat direktori');
    } finally {
      loading = false;
    }
  }

  // Tambah tab baru
  function addTab(customPath: string = '') {
    const path = customPath || currentPath;
    const newId = String(Date.now());
    const base = path.split(/[/\\]+/).filter(Boolean).pop() || 'folder';
    tabs = [...tabs, { id: newId, name: base, path }];
    activeTabId = newId;
    loadDirectory(path);
  }

  // Tutup tab tertentu
  function closeTab(id: string, e: MouseEvent) {
    e.stopPropagation();
    if (tabs.length <= 1) return;
    const idx = tabs.findIndex(t => t.id === id);
    tabs = tabs.filter(t => t.id !== id);
    if (activeTabId === id) {
      const nextTab = tabs[Math.max(0, idx - 1)];
      activeTabId = nextTab.id;
      loadDirectory(nextTab.path);
    }
  }

  // Beralih ke tab lain
  function switchTab(t: Tab) {
    activeTabId = t.id;
    loadDirectory(t.path);
  }

  // Navigasi ke direktori anak atau membuka editor untuk berkas teks
  function handleItemClick(item: FileItem) {
    if (item.is_dir) {
      loadDirectory(item.path);
    } else {
      openEditor(item);
    }
  }

  // Navigasi kembali ke folder induk
  function handleGoParent() {
    if (parentPath) {
      loadDirectory(parentPath);
    }
  }

  // Toggle seleksi semua item
  function toggleSelectAll(e: Event) {
    const checked = (e.target as HTMLInputElement).checked;
    if (checked) {
      selectedPaths = new Set(paginatedItems.map(it => it.path));
    } else {
      selectedPaths = new Set();
    }
  }

  // Toggle seleksi satu item
  function toggleSelectItem(path: string) {
    const next = new Set(selectedPaths);
    if (next.has(path)) {
      next.delete(path);
    } else {
      next.add(path);
    }
    selectedPaths = next;
  }

  // ═════════ Dropdown Operate Menu Handlers ═════════
  function toggleOperateMenu(item: FileItem, e: MouseEvent) {
    e.stopPropagation();
    activeToolbarMenu = null;
    if (activeMenuItem?.path === item.path) {
      closeAllMenus();
      return;
    }

    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const menuWidth = 220;
    const menuHeight = 440;

    let left = rect.right - menuWidth;
    if (left < 10) left = 10;

    let top = rect.bottom + 4;
    if (top + menuHeight > window.innerHeight) {
      top = rect.top - menuHeight - 4;
      if (top < 10) top = 10;
    }

    menuPos = { top, left };
    activeMenuItem = item;
  }

  function handleRowContextMenu(item: FileItem, e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    activeToolbarMenu = null;
    const menuWidth = 220;
    const menuHeight = 440;

    let left = e.clientX;
    if (left + menuWidth > window.innerWidth) {
      left = window.innerWidth - menuWidth - 10;
    }

    let top = e.clientY;
    if (top + menuHeight > window.innerHeight) {
      top = window.innerHeight - menuHeight - 10;
    }

    menuPos = { top, left };
    activeMenuItem = item;
  }

  function closeAllMenus() {
    activeMenuItem = null;
    activeToolbarMenu = null;
  }

  function toggleToolbarMenu(name: 'upload' | 'new' | 'favorite' | 'operations', e: MouseEvent) {
    e.stopPropagation();
    activeMenuItem = null;
    if (activeToolbarMenu === name) {
      activeToolbarMenu = null;
    } else {
      activeToolbarMenu = name;
    }
  }

  // Salin Path ke Clipboard OS
  async function copyPathToClipboard(item: FileItem) {
    closeAllMenus();
    try {
      await copyText(item.path);
      onNotify(`Path disalin: ${item.name}`);
    } catch {
      onNotify(`Lokasi: ${item.path}`);
    }
  }

  // Set Clipboard Salin (Copy)
  function handleCopyItem(item: FileItem) {
    closeAllMenus();
    clipboard = {
      action: 'copy',
      paths: [item.path],
      names: [item.name],
    };
    onNotify(`"${item.name}" disalin ke clipboard`);
  }

  // Set Clipboard Potong (Cut)
  function handleCutItem(item: FileItem) {
    closeAllMenus();
    clipboard = {
      action: 'cut',
      paths: [item.path],
      names: [item.name],
    };
    onNotify(`"${item.name}" dipotong ke clipboard`);
  }

  // Tempel (Paste) Item dari Clipboard ke Direktori Saat Ini
  async function handlePasteClipboard() {
    if (!clipboard) return;
    try {
      const res = await apiFetch<{ processed: number; errors?: string[] }>('/api/files/paste', {
        method: 'POST',
        body: JSON.stringify({
          action: clipboard.action,
          source_paths: clipboard.paths,
          destination_dir: currentPath,
        }),
      });

      if (res.errors && res.errors.length > 0) {
        onNotify(`Peringatan: ${res.errors.join(', ')}`);
      } else {
        onNotify(`Berhasil ${clipboard.action === 'cut' ? 'memindahkan' : 'menyalin'} ${res.processed} item`);
      }

      clipboard = null;
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || 'Gagal memproses paste');
    }
  }

  // Buka Properties Dialog
  function openProperties(item: FileItem) {
    closeAllMenus();
    propTarget = item;
    showPropertiesModal = true;
  }

  // Buka Compress Dialog
  function openCompress(item: FileItem) {
    closeAllMenus();
    compressTarget = item;
    compressZipName = `${item.name}.zip`;
    showCompressModal = true;
  }

  async function handleExecuteCompress(e: Event) {
    e.preventDefault();
    if (!compressTarget || !compressZipName.trim()) return;

    compressing = true;
    try {
      await apiFetch('/api/files/compress', {
        method: 'POST',
        body: JSON.stringify({
          paths: [compressTarget.path],
          zip_name: compressZipName.trim(),
          dest_dir: currentPath,
        }),
      });

      showCompressModal = false;
      onNotify(`Berhasil membuat arsip ${compressZipName}`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || 'Gagal mengompresi berkas');
    } finally {
      compressing = false;
    }
  }

  // External Share (Salin Link Download)
  async function handleExternalShare(item: FileItem) {
    closeAllMenus();
    const downloadUrl = `${window.location.origin}/api/files/read?path=${encodeURIComponent(item.path)}&download=1`;
    try {
      await copyText(downloadUrl);
      onNotify(`Link unduhan eksternal disalin: ${item.name}`);
    } catch {
      onNotify(`Link: ${downloadUrl}`);
    }
  }

  // Modal Pembuatan File / Folder Baru
  function openNewModal(type: 'file' | 'dir') {
    closeAllMenus();
    newType = type;
    newName = '';
    showNewModal = true;
  }

  async function handleCreateNew(e: Event) {
    e.preventDefault();
    if (!newName.trim()) return;
    try {
      await apiFetch('/api/files/new', {
        method: 'POST',
        body: JSON.stringify({
          parent_path: currentPath,
          name: newName.trim(),
          type: newType,
        }),
      });
      showNewModal = false;
      onNotify(`${newType === 'dir' ? 'Folder' : 'Berkas'} ${newName} berhasil dibuat`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || 'Gagal membuat item baru');
    }
  }

  // Pembuatan Soft Link (Symlink)
  async function handleCreateSymlink(e: Event) {
    e.preventDefault();
    if (!symlinkTarget.trim() || !symlinkName.trim()) return;

    creatingSymlink = true;
    try {
      await apiFetch('/api/files/symlink', {
        method: 'POST',
        body: JSON.stringify({
          target_path: symlinkTarget.trim(),
          link_name: symlinkName.trim(),
          dest_dir: currentPath,
        }),
      });
      showSymlinkModal = false;
      symlinkTarget = '';
      symlinkName = '';
      onNotify(`Tautan simbolik ${symlinkName} berhasil dibuat`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || 'Gagal membuat tautan simbolik');
    } finally {
      creatingSymlink = false;
    }
  }

  // Remote Download
  async function handleRemoteDownload(e: Event) {
    e.preventDefault();
    if (!remoteUrl.trim()) return;

    remoteDownloading = true;
    try {
      await apiFetch('/api/files/remote-download', {
        method: 'POST',
        body: JSON.stringify({
          url: remoteUrl.trim(),
          dest_dir: currentPath,
          filename: remoteFilename.trim(),
        }),
      });
      showRemoteModal = false;
      remoteUrl = '';
      remoteFilename = '';
      onNotify('Berkas berhasil diunduh ke server');
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || 'Gagal mengunduh berkas');
    } finally {
      remoteDownloading = false;
    }
  }

  // Search File Content
  async function handleSearchContent(e: Event) {
    e.preventDefault();
    if (!contentSearchQuery.trim()) return;

    searchingContent = true;
    try {
      const res = await apiFetch<{ matches: { file: string; line: number; text: string }[] }>('/api/files/search-content', {
        method: 'POST',
        body: JSON.stringify({
          dir: currentPath,
          query: contentSearchQuery.trim(),
        }),
      });
      contentSearchResults = res.matches || [];
    } catch (err: any) {
      onNotify(err.message || 'Gagal mencari konten berkas');
    } finally {
      searchingContent = false;
    }
  }

  // Modal Cloud Storage
  function openCloudStorage(name: string) {
    closeAllMenus();
    selectedCloud = name;
    showCloudStorageModal = true;
  }

  // Manajemen Favorit
  function addCurrentToFavorites() {
    const base = currentPath.split(/[/\\]+/).filter(Boolean).pop() || 'Folder';
    customFavorites = [...customFavorites, { name: base, path: currentPath }];
    onNotify(`Folder ${base} ditambahkan ke favorit`);
  }

  function removeFavorite(idx: number) {
    customFavorites = customFavorites.filter((_, i) => i !== idx);
  }

  // Modal Ganti Nama (Rename)
  function openRenameModal(item: FileItem) {
    closeAllMenus();
    renameTarget = item;
    renameNewName = item.name;
    showRenameModal = true;
  }

  async function handleRename(e: Event) {
    e.preventDefault();
    if (!renameTarget || !renameNewName.trim()) return;
    try {
      await apiFetch('/api/files/rename', {
        method: 'POST',
        body: JSON.stringify({
          path: renameTarget.path,
          new_name: renameNewName.trim(),
        }),
      });
      showRenameModal = false;
      onNotify(`Berhasil mengubah nama menjadi ${renameNewName}`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || 'Gagal mengubah nama');
    }
  }

  // Modal Hak Akses (PMSN / chmod)
  function openChmodModal(item: FileItem) {
    closeAllMenus();
    chmodTarget = item;
    const mode = item.mode.slice(-3);
    if (mode.length === 3) {
      const o = parseInt(mode[0], 8) || 7;
      const g = parseInt(mode[1], 8) || 5;
      const p = parseInt(mode[2], 8) || 5;
      chmodPerms = {
        ownerR: (o & 4) !== 0, ownerW: (o & 2) !== 0, ownerX: (o & 1) !== 0,
        groupR: (g & 4) !== 0, groupW: (g & 2) !== 0, groupX: (g & 1) !== 0,
        publicR: (p & 4) !== 0, publicW: (p & 2) !== 0, publicX: (p & 1) !== 0,
      };
    }
    showChmodModal = true;
  }

  async function handleSaveChmod() {
    if (!chmodTarget) return;
    try {
      await apiFetch('/api/files/chmod', {
        method: 'POST',
        body: JSON.stringify({
          path: chmodTarget.path,
          mode: chmodOctal,
        }),
      });
      showChmodModal = false;
      onNotify(`Izin ${chmodTarget.name} diubah ke ${chmodOctal}`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || 'Gagal mengubah izin');
    }
  }

  // Deteksi bahasa dan format berkas berdasarkan nama/ekstensi
  function detectFileLanguage(filename: string): { label: string; icon: string; color: string } {
    const lower = filename.toLowerCase();
    if (lower === '.htaccess' || (lower.endsWith('.conf') && lower.includes('apache'))) {
      return { label: 'Apache Conf', icon: 'ri-server-line', color: '#f59e0b' };
    }
    if (lower.endsWith('.conf') || lower.includes('nginx')) {
      return { label: 'Nginx Conf', icon: 'ri-server-line', color: '#10b981' };
    }
    if (lower.endsWith('.go')) return { label: 'Go', icon: 'ri-code-s-slash-line', color: '#00add8' };
    if (lower.endsWith('.ts')) return { label: 'TypeScript', icon: 'ri-javascript-line', color: '#3178c6' };
    if (lower.endsWith('.js') || lower.endsWith('.mjs') || lower.endsWith('.cjs')) return { label: 'JavaScript', icon: 'ri-javascript-line', color: '#f7df1e' };
    if (lower.endsWith('.svelte')) return { label: 'Svelte', icon: 'ri-code-line', color: '#ff3e00' };
    if (lower.endsWith('.json')) return { label: 'JSON', icon: 'ri-braces-line', color: '#cbd5e1' };
    if (lower.endsWith('.html') || lower.endsWith('.htm')) return { label: 'HTML', icon: 'ri-html5-line', color: '#e34f26' };
    if (lower.endsWith('.css')) return { label: 'CSS', icon: 'ri-css3-line', color: '#264de4' };
    if (lower.endsWith('.scss') || lower.endsWith('.sass')) return { label: 'SCSS', icon: 'ri-css3-line', color: '#cc6699' };
    if (lower.endsWith('.php')) return { label: 'PHP', icon: 'ri-code-line', color: '#777bb4' };
    if (lower.endsWith('.py')) return { label: 'Python', icon: 'ri-terminal-box-line', color: '#3776ab' };
    if (lower.endsWith('.sh') || lower.endsWith('.bash')) return { label: 'Shell Script', icon: 'ri-terminal-line', color: '#4ade80' };
    if (lower.endsWith('.yml') || lower.endsWith('.yaml')) return { label: 'YAML', icon: 'ri-file-list-2-line', color: '#cb171e' };
    if (lower.endsWith('.sql')) return { label: 'SQL', icon: 'ri-database-2-line', color: '#00758f' };
    if (lower.endsWith('.md')) return { label: 'Markdown', icon: 'ri-markdown-line', color: '#0284c7' };
    if (lower.startsWith('.env')) return { label: 'Environment', icon: 'ri-key-2-line', color: '#eab308' };
    if (lower.endsWith('.xml')) return { label: 'XML', icon: 'ri-code-line', color: '#f97316' };
    return { label: 'Plain Text', icon: 'ri-file-text-line', color: '#94a3b8' };
  }

  // Buka editor modern untuk membaca dan menyunting berkas teks
  async function openEditor(item: FileItem) {
    closeAllMenus();
    editorTarget = item;
    editorContent = 'Memuat isi berkas...';
    editorInitialContent = '';
    showEditorModal = true;
    editorShowSearch = false;
    editorSearchQuery = '';
    editorReplaceQuery = '';
    editorSearchMatches = [];
    editorSearchMatchIndex = 0;
    try {
      const q = encodeURIComponent(item.path);
      const res = await apiFetch<{ content: string }>(`/api/files/read?path=${q}`);
      editorContent = res.content || '';
      editorInitialContent = editorContent;
      setTimeout(() => {
        if (editorTextareaEl) {
          editorTextareaEl.focus();
          editorTextareaEl.setSelectionRange(0, 0);
          updateEditorCursor();
        }
      }, 50);
    } catch (err: any) {
      onNotify(err.message || 'Gagal membaca berkas');
      showEditorModal = false;
    }
  }

  // Simpan konten editor ke disk server
  async function handleSaveEditor() {
    if (!editorTarget) return;
    editorSaving = true;
    try {
      await apiFetch('/api/files/save', {
        method: 'POST',
        body: JSON.stringify({
          path: editorTarget.path,
          content: editorContent,
        }),
      });
      editorInitialContent = editorContent;
      onNotify(`Berkas ${editorTarget.name} berhasil disimpan`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || 'Gagal menyimpan berkas');
    } finally {
      editorSaving = false;
    }
  }

  // Muat ulang konten berkas dari server
  async function refreshEditorContent() {
    if (!editorTarget) return;
    if (editorIsDirty && !(await confirmDialog({
      title: 'Muat ulang berkas dari server?',
      message: 'Perubahan yang belum Anda simpan di editor akan dibuang.',
      confirmText: 'Muat Ulang',
      tone: 'danger',
    }))) {
      return;
    }
    try {
      const q = encodeURIComponent(editorTarget.path);
      const res = await apiFetch<{ content: string }>(`/api/files/read?path=${q}`);
      editorContent = res.content || '';
      editorInitialContent = editorContent;
      onNotify('Isi berkas berhasil dimuat ulang');
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat ulang berkas');
    }
  }

  // Tutup editor dengan konfirmasi bila ada perubahan yang belum disimpan
  async function closeEditorModal() {
    if (editorIsDirty) {
      if (!(await confirmDialog({
        title: 'Tutup editor tanpa menyimpan?',
        message: 'Perubahan pada berkas ini belum disimpan ke server.',
        confirmText: 'Tutup Editor',
        tone: 'danger',
      }))) {
        return;
      }
    }
    showEditorModal = false;
    editorTarget = null;
    editorShowSearch = false;
  }

  // Perbarui posisi kursor (baris dan kolom)
  function updateEditorCursor() {
    if (!editorTextareaEl) return;
    const pos = editorTextareaEl.selectionStart || 0;
    const textBefore = editorContent.substring(0, pos);
    const lines = textBefore.split('\n');
    const line = lines.length;
    const col = lines[lines.length - 1].length + 1;
    editorCursorPos = { line, col };
  }

  // Sinkronisasi scroll antara gutter nomor baris dan textarea editor
  function handleEditorScroll() {
    if (!editorTextareaEl || !editorGutterEl) return;
    editorGutterEl.scrollTop = editorTextareaEl.scrollTop;
  }

  // Tangani shortcut keyboard editor (Ctrl+S, Ctrl+F, Ctrl+G, Tab)
  function handleEditorKeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault();
      handleSaveEditor();
      return;
    }
    if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
      e.preventDefault();
      editorShowSearch = !editorShowSearch;
      return;
    }
    if ((e.ctrlKey || e.metaKey) && e.key === 'g') {
      e.preventDefault();
      handleEditorJumpLine();
      return;
    }
    if (e.key === 'Tab') {
      e.preventDefault();
      if (!editorTextareaEl) return;
      const start = editorTextareaEl.selectionStart;
      const end = editorTextareaEl.selectionEnd;
      const spaces = '  '; // Indentasi 2 spasi standar
      editorContent = editorContent.substring(0, start) + spaces + editorContent.substring(end);
      setTimeout(() => {
        if (editorTextareaEl) {
          editorTextareaEl.selectionStart = editorTextareaEl.selectionEnd = start + spaces.length;
          updateEditorCursor();
        }
      }, 0);
    }
  }

  // Lompat ke baris tertentu sesuai input pengguna
  async function handleEditorJumpLine() {
    const targetStr = await promptDialog({
      title: 'Lompat ke Baris',
      label: `Nomor baris (1 - ${editorLineCount}):`,
      value: String(editorCursorPos.line),
      inputType: 'number',
      min: 1,
      max: editorLineCount,
      confirmText: 'Lompat',
    });
    if (!targetStr) return;
    const targetLine = parseInt(targetStr, 10);
    if (isNaN(targetLine) || targetLine < 1 || targetLine > editorLineCount) {
      onNotify('Nomor baris tidak valid');
      return;
    }
    const lines = editorContent.split('\n');
    let charIndex = 0;
    for (let i = 0; i < targetLine - 1; i++) {
      charIndex += lines[i].length + 1;
    }
    if (editorTextareaEl) {
      editorTextareaEl.focus();
      editorTextareaEl.setSelectionRange(charIndex, charIndex + (lines[targetLine - 1]?.length || 0));
      updateEditorCursor();
      const lineHeight = editorFontSize * 1.6;
      editorTextareaEl.scrollTop = Math.max(0, (targetLine - 5) * lineHeight);
    }
  }

  // Pencarian kata kunci di dalam textarea editor
  function searchInEditor(direction: 'next' | 'prev' = 'next') {
    if (!editorSearchQuery || !editorTextareaEl) return;
    const query = editorSearchQuery.toLowerCase();
    const contentLower = editorContent.toLowerCase();
    const matches: number[] = [];
    let idx = contentLower.indexOf(query);
    while (idx !== -1) {
      matches.push(idx);
      idx = contentLower.indexOf(query, idx + 1);
    }
    editorSearchMatches = matches;
    if (matches.length === 0) {
      editorSearchMatchIndex = 0;
      onNotify('Teks tidak ditemukan dalam berkas');
      return;
    }

    if (direction === 'next') {
      editorSearchMatchIndex = (editorSearchMatchIndex + 1) % matches.length;
    } else {
      editorSearchMatchIndex = (editorSearchMatchIndex - 1 + matches.length) % matches.length;
    }

    const matchChar = matches[editorSearchMatchIndex];
    editorTextareaEl.focus();
    editorTextareaEl.setSelectionRange(matchChar, matchChar + editorSearchQuery.length);
    updateEditorCursor();

    const linesBefore = editorContent.substring(0, matchChar).split('\n').length;
    const lineHeight = editorFontSize * 1.6;
    editorTextareaEl.scrollTop = Math.max(0, (linesBefore - 5) * lineHeight);
  }

  // Mengganti teks yang sedang dipilih
  function replaceInEditor() {
    if (!editorSearchQuery || !editorTextareaEl || editorSearchMatches.length === 0) return;
    const matchChar = editorSearchMatches[editorSearchMatchIndex];
    editorContent = editorContent.substring(0, matchChar) + editorReplaceQuery + editorContent.substring(matchChar + editorSearchQuery.length);
    searchInEditor('next');
  }

  // Mengganti semua kemunculan teks dalam berkas
  function replaceAllInEditor() {
    if (!editorSearchQuery) return;
    const parts = editorContent.split(editorSearchQuery);
    const count = parts.length - 1;
    if (count > 0) {
      editorContent = parts.join(editorReplaceQuery);
      onNotify(`${count} kemunculan teks berhasil diganti`);
      editorSearchMatches = [];
      editorSearchMatchIndex = 0;
    } else {
      onNotify('Teks tidak ditemukan untuk diganti');
    }
  }

  // Mengunduh Berkas
  function downloadFile(item: FileItem) {
    closeAllMenus();
    const q = encodeURIComponent(item.path);
    window.open(`/api/files/read?path=${q}&download=1`, '_blank');
  }

  // Hapus Satu Item
  async function deleteSingleItem(item: FileItem) {
    closeAllMenus();
    if (!(await confirmDialog({
      title: `Hapus ${item.is_dir ? 'folder' : 'berkas'} "${item.name}"?`,
      message: 'Item ini akan dihapus secara permanen dari server dan tidak dapat dipulihkan.',
      confirmText: 'Hapus Permanen',
      tone: 'danger',
    }))) return;
    try {
      await apiFetch('/api/files/delete', {
        method: 'POST',
        body: JSON.stringify({ paths: [item.path] }),
      });
      onNotify(`"${item.name}" berhasil dihapus`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || 'Gagal menghapus item');
    }
  }

  // Hapus Massal Berkas yang Terpilih
  async function handleBatchDelete() {
    if (selectedPaths.size === 0) return;
    if (!(await confirmDialog({
      title: `Hapus ${selectedPaths.size} item yang dipilih?`,
      message: 'Semua berkas dan folder yang terpilih akan dihapus permanen dari server.',
      confirmText: `Hapus ${selectedPaths.size} Item`,
      tone: 'danger',
    }))) return;
    try {
      await apiFetch('/api/files/delete', {
        method: 'POST',
        body: JSON.stringify({ paths: Array.from(selectedPaths) }),
      });
      onNotify(`${selectedPaths.size} item berhasil dihapus`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || 'Gagal menghapus batch item');
    }
  }

  // Unggah Berkas
  async function handleUploadSubmit(e: Event) {
    e.preventDefault();
    if (!uploadFiles || uploadFiles.length === 0) return;

    uploadSubmitting = true;
    try {
      const formData = new FormData();
      formData.append('destination', currentPath);
      for (let i = 0; i < uploadFiles.length; i++) {
        formData.append('files', uploadFiles[i]);
      }

      await apiFetch('/api/files/upload', {
        method: 'POST',
        body: formData,
        timeoutMs: 0,
      });

      showUploadModal = false;
      uploadFiles = null;
      onNotify('Berkas berhasil diunggah');
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || 'Galat saat upload');
    } finally {
      uploadSubmitting = false;
    }
  }

  // Pintasan Lokasi Favorit
  function jumpFavorite(path: string) {
    loadDirectory(path);
  }

  // Helper Ikon Berkas Berdasarkan Ekstensi
  function getFileIcon(item: FileItem): { icon: string; color: string } {
    if (item.is_dir) {
      return { icon: 'ri-folder-fill', color: '#f59e0b' };
    }
    const ext = item.ext.toLowerCase();
    switch (ext) {
      case '.go':
      case '.ts':
      case '.js':
      case '.svelte':
      case '.py':
      case '.php':
      case '.html':
      case '.css':
      case '.json':
        return { icon: 'ri-file-code-fill', color: '#10b981' };
      case '.png':
      case '.jpg':
      case '.jpeg':
      case '.svg':
      case '.webp':
      case '.gif':
        return { icon: 'ri-image-fill', color: '#ec4899' };
      case '.zip':
      case '.tar':
      case '.gz':
      case '.rar':
        return { icon: 'ri-file-zip-fill', color: '#f97316' };
      case '.md':
      case '.txt':
      case '.log':
      case '.conf':
      case '.env':
        return { icon: 'ri-file-text-fill', color: '#38bdf8' };
      default:
        return { icon: 'ri-file-3-fill', color: '#94a3b8' };
    }
  }

  onMount(() => {
    // Periksa apakah ada permintaan pembukaan direktori dari halaman lain (misal tombol aksi cepat situs)
    const openPath = sessionStorage.getItem('fikpanel_open_path');
    if (openPath) {
      sessionStorage.removeItem('fikpanel_open_path');
      loadDirectory(openPath);
    } else {
      loadDirectory();
    }
    window.addEventListener('click', closeAllMenus);
  });

  onDestroy(() => {
    window.removeEventListener('click', closeAllMenus);
  });
</script>

<div class="files-view">
  <!-- ═════════ 1. Tab Bar Direktori (Multi-Tab aaPanel) ═════════ -->
  <div class="tabs-bar">
    <div class="tab-list">
      {#each tabs as t (t.id)}
        <div
          class="tab-item {activeTabId === t.id ? 'active' : ''}"
          role="button"
          tabindex="0"
          onclick={() => switchTab(t)}
          onkeydown={(e) => e.key === 'Enter' && switchTab(t)}
        >
          <i class="ri-folder-fill tab-icon"></i>
          <span class="tab-title">{t.name}</span>
          {#if tabs.length > 1}
            <button
              class="tab-close"
              onclick={(e) => closeTab(t.id, e)}
              title="Tutup tab"
            >&times;</button>
          {/if}
        </div>
      {/each}
      <button class="btn-add-tab" onclick={() => addTab()} title="Buka tab baru">
        <i class="ri-add-line"></i>
      </button>
    </div>
  </div>

  <!-- ═════════ 2. Breadcrumbs & Search Bar ═════════ -->
  <div class="nav-bar">
    <div class="nav-left">
      <!-- Tombol Kembali ke Induk -->
      <button
        class="btn-icon"
        onclick={handleGoParent}
        disabled={!parentPath}
        title="Kembali ke folder induk"
      >
        <i class="ri-arrow-left-line"></i>
      </button>

      <!-- Jalur Breadcrumbs -->
      <div class="breadcrumb-trail">
        <i class="ri-folder-3-fill path-icon"></i>
        <button class="crumb-btn" onclick={() => loadDirectory('')}>root</button>
        {#each breadcrumbs as crumb}
          <span class="crumb-sep">&gt;</span>
          <button class="crumb-btn" onclick={() => loadDirectory(crumb.fullPath)}>
            {crumb.label}
          </button>
        {/each}
      </div>

      <!-- Tombol Refresh -->
      <button class="btn-icon" onclick={() => loadDirectory(currentPath)} title="Refresh">
        <i class="ri-refresh-line"></i>
      </button>
    </div>

    <!-- Pencarian Berkas Instan -->
    <div class="nav-right">
      <div class="search-box">
        <i class="ri-search-line search-icon"></i>
        <input
          type="text"
          placeholder="Cari berkas..."
          bind:value={searchQuery}
        />
        {#if searchQuery}
          <button
            class="btn-clear-search"
            onclick={() => (searchQuery = '')}
            title="Reset pencarian"
          >
            <i class="ri-close-line"></i>
          </button>
        {/if}
      </div>
    </div>
  </div>

  <!-- ═════════ 3. Toolbar Aksi (Action Toolbar aaPanel Asli) ═════════ -->
  <div class="action-toolbar">
    <div class="tool-left">
      <!-- 1. Dropdown Upload > -->
      <div class="tool-dropdown-wrap">
        <button
          class="tool-btn {activeToolbarMenu === 'upload' ? 'is-active' : ''}"
          onclick={(e) => toggleToolbarMenu('upload', e)}
        >
          <i class="ri-upload-line"></i>
          <span>Upload &gt;</span>
        </button>

        {#if activeToolbarMenu === 'upload'}
          <div
            class="toolbar-dropdown-menu"
            role="menu"
            tabindex="-1"
            onclick={(e) => e.stopPropagation()}
            onkeydown={(e) => e.key === 'Escape' && closeAllMenus()}
          >
            <button class="tb-menu-item" onclick={() => { activeToolbarMenu = null; showUploadModal = true; }}>
              <div class="tb-item-left">
                <i class="ri-upload-2-line"></i>
                <span>Upload File</span>
              </div>
            </button>
            <button class="tb-menu-item" onclick={() => openCloudStorage('FTP Storage')}>
              <div class="tb-item-left">
                <i class="ri-server-line"></i>
                <span>FTP Storage</span>
              </div>
              <i class="ri-vip-diamond-fill badge-vip"></i>
            </button>
            <button class="tb-menu-item" onclick={() => openCloudStorage('S3 Storage')}>
              <div class="tb-item-left">
                <i class="ri-cloud-line"></i>
                <span>S3 Storage</span>
              </div>
              <i class="ri-vip-diamond-fill badge-vip"></i>
            </button>
            <button class="tb-menu-item" onclick={() => openCloudStorage('Google Drive')}>
              <div class="tb-item-left">
                <i class="ri-google-fill"></i>
                <span>Google Drive</span>
              </div>
              <i class="ri-vip-diamond-fill badge-vip"></i>
            </button>
            <button class="tb-menu-item" onclick={() => openCloudStorage('Google Cloud Storage')}>
              <div class="tb-item-left">
                <i class="ri-cloudy-line"></i>
                <span>Google Cloud Storage</span>
              </div>
              <i class="ri-vip-diamond-fill badge-vip"></i>
            </button>
          </div>
        {/if}
      </div>

      <!-- 2. Remote Download -->
      <button class="tool-btn" onclick={() => (showRemoteModal = true)}>
        <i class="ri-download-cloud-line icon-blue"></i>
        <span>Remote Download</span>
      </button>

      <!-- 3. Dropdown New > -->
      <div class="tool-dropdown-wrap">
        <button
          class="tool-btn {activeToolbarMenu === 'new' ? 'is-active' : ''}"
          onclick={(e) => toggleToolbarMenu('new', e)}
        >
          <i class="ri-folder-add-fill icon-amber"></i>
          <span>New &gt;</span>
        </button>

        {#if activeToolbarMenu === 'new'}
          <div
            class="toolbar-dropdown-menu"
            role="menu"
            tabindex="-1"
            onclick={(e) => e.stopPropagation()}
            onkeydown={(e) => e.key === 'Escape' && closeAllMenus()}
          >
            <button class="tb-menu-item" onclick={() => openNewModal('dir')}>
              <div class="tb-item-left">
                <i class="ri-folder-fill icon-amber"></i>
                <span>New Directory</span>
              </div>
            </button>
            <button class="tb-menu-item" onclick={() => openNewModal('file')}>
              <div class="tb-item-left">
                <i class="ri-file-3-line"></i>
                <span>New Blank File</span>
              </div>
            </button>
            <button class="tb-menu-item" onclick={() => { activeToolbarMenu = null; showSymlinkModal = true; }}>
              <div class="tb-item-left">
                <i class="ri-links-line icon-blue"></i>
                <span>Soft Link</span>
              </div>
            </button>
          </div>
        {/if}
      </div>

      <!-- 4. Search File Content -->
      <button class="tool-btn" onclick={() => (showSearchContentModal = true)}>
        <i class="ri-file-search-line"></i>
        <span>Search File Content</span>
      </button>

      <!-- 5. Dropdown Favorite > -->
      <div class="tool-dropdown-wrap">
        <button
          class="tool-btn {activeToolbarMenu === 'favorite' ? 'is-active' : ''}"
          onclick={(e) => toggleToolbarMenu('favorite', e)}
        >
          <i class="ri-star-fill icon-yellow"></i>
          <span>Favorite &gt;</span>
        </button>

        {#if activeToolbarMenu === 'favorite'}
          <div
            class="toolbar-dropdown-menu"
            role="menu"
            tabindex="-1"
            onclick={(e) => e.stopPropagation()}
            onkeydown={(e) => e.key === 'Escape' && closeAllMenus()}
          >
            {#each customFavorites as fav}
              <button class="tb-menu-item" onclick={() => { activeToolbarMenu = null; jumpFavorite(fav.path); }}>
                <div class="tb-item-left">
                  <i class="ri-folder-line icon-amber"></i>
                  <span>{fav.name}</span>
                </div>
              </button>
            {/each}
            <div class="operate-divider"></div>
            <button class="tb-menu-item" onclick={() => { activeToolbarMenu = null; showFavManageModal = true; }}>
              <div class="tb-item-left">
                <i class="ri-settings-3-line"></i>
                <span>Management</span>
              </div>
            </button>
          </div>
        {/if}
      </div>

      <!-- 6. Share List -->
      <button class="tool-btn" onclick={() => onNotify('Fitur Share List: tautan unduhan langsung terdaftar di menu Properties')}>
        <i class="ri-share-line icon-green"></i>
        <span>Share List</span>
      </button>

      <!-- 7. File Operations > -->
      <div class="tool-dropdown-wrap">
        <button
          class="tool-btn {activeToolbarMenu === 'operations' ? 'is-active' : ''}"
          onclick={(e) => toggleToolbarMenu('operations', e)}
        >
          <i class="ri-folder-settings-line icon-amber"></i>
          <span>File Operations &gt;</span>
        </button>

        {#if activeToolbarMenu === 'operations'}
          <div
            class="toolbar-dropdown-menu"
            role="menu"
            tabindex="-1"
            onclick={(e) => e.stopPropagation()}
            onkeydown={(e) => e.key === 'Escape' && closeAllMenus()}
          >
            <button
              class="tb-menu-item"
              disabled={selectedPaths.size === 0}
              onclick={() => {
                if (selectedPaths.size > 0) {
                  clipboard = { action: 'copy', paths: Array.from(selectedPaths), names: Array.from(selectedPaths).map(p => p.split(/[/\\]+/).pop() || '') };
                  onNotify(`${selectedPaths.size} item disalin ke clipboard`);
                  activeToolbarMenu = null;
                }
              }}
            >
              <div class="tb-item-left">
                <i class="ri-file-copy-line"></i>
                <span>Copy ({selectedPaths.size})</span>
              </div>
            </button>
            <button
              class="tb-menu-item"
              disabled={selectedPaths.size === 0}
              onclick={() => {
                if (selectedPaths.size > 0) {
                  clipboard = { action: 'cut', paths: Array.from(selectedPaths), names: Array.from(selectedPaths).map(p => p.split(/[/\\]+/).pop() || '') };
                  onNotify(`${selectedPaths.size} item dipotong ke clipboard`);
                  activeToolbarMenu = null;
                }
              }}
            >
              <div class="tb-item-left">
                <i class="ri-scissors-line icon-cyan"></i>
                <span>Cut ({selectedPaths.size})</span>
              </div>
            </button>
            <button
              class="tb-menu-item item-danger"
              disabled={selectedPaths.size === 0}
              onclick={() => { activeToolbarMenu = null; handleBatchDelete(); }}
            >
              <div class="tb-item-left">
                <i class="ri-delete-bin-line"></i>
                <span>Delete ({selectedPaths.size})</span>
              </div>
            </button>
          </div>
        {/if}
      </div>

      <!-- Paste Button jika Clipboard Aktif -->
      {#if clipboard}
        <div class="paste-indicator">
          <button class="tool-btn tool-primary" onclick={handlePasteClipboard} title="Klik untuk menempel di direktori ini">
            <i class="ri-clipboard-line"></i>
            <span>Paste ({clipboard.names.length} item)</span>
          </button>
          <button class="btn-cancel-clip" onclick={() => (clipboard = null)} title="Batalkan salin/potong">&times;</button>
        </div>
      {/if}
    </div>

    <div class="tool-right">
      <div class="view-toggle">
        <button
          class="toggle-btn {viewMode === 'list' ? 'active' : ''}"
          onclick={() => (viewMode = 'list')}
          title="Tampilan Daftar"
        >
          <i class="ri-list-check"></i>
        </button>
        <button
          class="toggle-btn {viewMode === 'grid' ? 'active' : ''}"
          onclick={() => (viewMode = 'grid')}
          title="Tampilan Grid"
        >
          <i class="ri-grid-fill"></i>
        </button>
      </div>
    </div>
  </div>

  <!-- ═════════ 4. Area Konten File (Table / Grid) ═════════ -->
  <div class="file-content-panel">
    {#if loading}
      <div class="state-box">
        <i class="ri-loader-4-line spin-icon"></i>
        <p>Memuat direktori...</p>
      </div>
    {:else if filteredItems.length === 0}
      <div class="state-box">
        <i class="ri-folder-open-line empty-icon"></i>
        <p>Direktori ini kosong atau berkas tidak ditemukan.</p>
        <div class="empty-actions">
          <button class="btn btn-outline" onclick={() => openNewModal('file')}>+ Buat Berkas</button>
          <button class="btn btn-outline" onclick={() => openNewModal('dir')}>+ Buat Folder</button>
          <button class="btn btn-primary" onclick={() => (showUploadModal = true)}>Upload Berkas</button>
        </div>
      </div>
    {:else if viewMode === 'list'}
      <!-- Tampilan Tabel List Gaya aaPanel -->
      <div class="table-container">
        <table class="file-table">
          <thead>
            <tr>
              <th class="col-check">
                <input
                  type="checkbox"
                  checked={paginatedItems.length > 0 && selectedPaths.size === paginatedItems.length}
                  onchange={toggleSelectAll}
                />
              </th>
              <th class="col-name">File Name &darr;</th>
              <th class="col-prot">Protected</th>
              <th class="col-pmsn">PMSN/Owner</th>
              <th class="col-size">Size &darr;</th>
              <th class="col-time">Modified Time &darr;</th>
              <th class="col-ps">Ps</th>
              <th class="col-ops">Operation</th>
            </tr>
          </thead>
          <tbody>
            {#each paginatedItems as item (item.path)}
              {@const iconMeta = getFileIcon(item)}
              <tr
                class="{selectedPaths.has(item.path) ? 'selected-row' : ''} {activeMenuItem?.path === item.path ? 'highlight-row' : ''}"
                oncontextmenu={(e) => handleRowContextMenu(item, e)}
              >
                <td class="col-check">
                  <input
                    type="checkbox"
                    checked={selectedPaths.has(item.path)}
                    onchange={() => toggleSelectItem(item.path)}
                  />
                </td>
                <td class="col-name">
                  <div
                    class="name-cell"
                    role="button"
                    tabindex="0"
                    onclick={() => handleItemClick(item)}
                    onkeydown={(e) => e.key === 'Enter' && handleItemClick(item)}
                  >
                    <i class="{iconMeta.icon} item-icon" style="color: {iconMeta.color};"></i>
                    <span class="item-name {item.is_dir ? 'dir-name' : ''}">{item.name}</span>
                  </div>
                </td>
                <td class="col-prot">
                  <span class="badge-prot">
                    <i class="ri-lock-unlock-line"></i> Unprotected
                  </span>
                </td>
                <td class="col-pmsn">
                  <button
                    class="pmsn-pill"
                    onclick={() => openChmodModal(item)}
                    title="Klik untuk ubah perizinan (chmod)"
                  >
                    {item.mode.slice(-3)} / {item.owner}
                  </button>
                </td>
                <td class="col-size mono">
                  {#if item.is_dir}
                    <span class="calc-text">Calculate</span>
                  {:else}
                    {formatBytes(item.size)}
                  {/if}
                </td>
                <td class="col-time mono">{item.mod_time}</td>
                <td class="col-ps text-muted">{item.is_dir ? 'Directory' : item.ext || 'File'}</td>
                <td class="col-ops">
                  <div class="row-operations">
                    <!-- Hover Quick Shortcuts (hanya terlihat saat baris di-hover) -->
                    <div class="hover-shortcuts">
                      {#if !item.is_dir}
                        <button class="quick-icon-btn" onclick={() => openEditor(item)} title="Edit Cepat">
                          <i class="ri-edit-line"></i>
                        </button>
                        <button class="quick-icon-btn" onclick={() => downloadFile(item)} title="Unduh">
                          <i class="ri-download-line"></i>
                        </button>
                      {:else}
                        <button class="quick-icon-btn" onclick={() => handleItemClick(item)} title="Buka">
                          <i class="ri-folder-open-line"></i>
                        </button>
                      {/if}
                      <button class="quick-icon-btn link-danger" onclick={() => deleteSingleItem(item)} title="Hapus">
                        <i class="ri-delete-bin-line"></i>
                      </button>
                    </div>

                    <!-- Tombol Dropdown Operate (Sesuai Screenshot aaPanel) -->
                    <button
                      class="btn-operate {activeMenuItem?.path === item.path ? 'is-active' : ''}"
                      onclick={(e) => toggleOperateMenu(item, e)}
                      title="Menu Operasi"
                    >
                      <span>Operate</span>
                      <i class="ri-arrow-down-s-line"></i>
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <!-- Tampilan Grid Bento -->
      <div class="grid-container">
        {#each paginatedItems as item (item.path)}
          {@const iconMeta = getFileIcon(item)}
          <div
            class="grid-card {selectedPaths.has(item.path) ? 'selected-card' : ''}"
            role="button"
            tabindex="0"
            onclick={() => handleItemClick(item)}
            onkeydown={(e) => e.key === 'Enter' && handleItemClick(item)}
            oncontextmenu={(e) => handleRowContextMenu(item, e)}
          >
            <div class="card-top">
              <input
                type="checkbox"
                checked={selectedPaths.has(item.path)}
                onclick={(e) => e.stopPropagation()}
                onchange={() => toggleSelectItem(item.path)}
              />
              <button
                class="card-operate-btn"
                onclick={(e) => toggleOperateMenu(item, e)}
                title="Menu Operasi"
              >
                <i class="ri-more-2-fill"></i>
              </button>
            </div>
            <i class="{iconMeta.icon} card-icon" style="color: {iconMeta.color};"></i>
            <span class="card-name" title={item.name}>{item.name}</span>
            <span class="card-size mono">{item.is_dir ? 'Directory' : formatBytes(item.size)}</span>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- ═════════ 5. Footer & Status Bar ═════════ -->
  <div class="footer-bar">
    <div class="foot-left">
      <span>Total <strong>{totalDirs}</strong> directories, <strong>{totalFiles}</strong> files, File size: <strong class="mono">{formatBytes(totalSize)}</strong></span>
    </div>

    <div class="foot-right">
      <div class="pager">
        <button
          class="btn-page"
          disabled={currentPage <= 1}
          onclick={() => currentPage--}
        >&lt;</button>
        <span class="page-num">{currentPage}</span>
        <button
          class="btn-page"
          disabled={currentPage >= totalPages}
          onclick={() => currentPage++}
        >&gt;</button>
      </div>

      <select class="select-pagesize" bind:value={pageSize}>
        <option value={50}>50 / page</option>
        <option value={100}>100 / page</option>
        <option value={200}>200 / page</option>
      </select>

      <span class="goto-box">Goto <input type="number" min="1" max={totalPages} bind:value={currentPage} /></span>
      <span class="total-badge">Total {filteredItems.length}</span>
    </div>
  </div>

  <!-- ═════════ DROPDOWN OPERATE MENU (Popup Sesuai Screenshot aaPanel) ═════════ -->
  {#if activeMenuItem}
    <div
      class="operate-dropdown"
      role="menu"
      tabindex="-1"
      style="top: {menuPos.top}px; left: {menuPos.left}px;"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.key === 'Escape' && closeAllMenus()}
    >
      {#if !activeMenuItem.is_dir}
        <!-- Opsi Khusus Berkas (File): Tanpa Open/New Window, diawali dengan Edit -->
        <!-- Edit -->
        <button class="operate-item" onclick={() => { const it = activeMenuItem!; closeAllMenus(); openEditor(it); }}>
          <i class="ri-edit-line op-icon icon-blue"></i>
          <span>Edit</span>
        </button>

        <!-- Download -->
        <button class="operate-item" onclick={() => { const it = activeMenuItem!; closeAllMenus(); downloadFile(it); }}>
          <i class="ri-download-line op-icon icon-blue"></i>
          <span>Download</span>
        </button>

        <!-- External Share -->
        <button class="operate-item" onclick={() => handleExternalShare(activeMenuItem!)}>
          <i class="ri-share-forward-line op-icon icon-green"></i>
          <span>External Share</span>
        </button>

        <!-- Add to Favorites -->
        <button class="operate-item" onclick={() => { const it = activeMenuItem!; closeAllMenus(); customFavorites = [...customFavorites, { name: it.name, path: it.path }]; onNotify(`"${it.name}" ditambahkan ke favorit`); }}>
          <i class="ri-star-fill op-icon icon-yellow"></i>
          <span>Add to Favorites</span>
        </button>

        <!-- Upload to Cloud Storage -->
        <button class="operate-item" onclick={() => { closeAllMenus(); openCloudStorage('S3 Storage'); }}>
          <div class="op-label-row">
            <i class="ri-upload-cloud-2-line op-icon"></i>
            <span>Upload to Cloud Storage</span>
          </div>
          <i class="ri-arrow-right-s-line op-chevron"></i>
        </button>

        <!-- Permission -->
        <button class="operate-item" onclick={() => openChmodModal(activeMenuItem!)}>
          <i class="ri-folder-lock-fill op-icon icon-amber"></i>
          <span>Permission</span>
        </button>
      {:else}
        <!-- Opsi Khusus Direktori (Folder): Ada Open dan Open in New Window -->
        <!-- Open -->
        <button class="operate-item" onclick={() => { const it = activeMenuItem!; closeAllMenus(); handleItemClick(it); }}>
          <i class="ri-folder-open-fill op-icon icon-amber"></i>
          <span>Open</span>
        </button>

        <!-- Open in New Window / Tab -->
        <button class="operate-item" onclick={() => { const it = activeMenuItem!; closeAllMenus(); addTab(it.path); }}>
          <i class="ri-external-link-line op-icon icon-blue"></i>
          <span>Open in New Window</span>
        </button>

        <!-- Add to Favorites -->
        <button class="operate-item" onclick={() => { const it = activeMenuItem!; closeAllMenus(); customFavorites = [...customFavorites, { name: it.name, path: it.path }]; onNotify(`"${it.name}" ditambahkan ke favorit`); }}>
          <i class="ri-star-fill op-icon icon-yellow"></i>
          <span>Add to Favorites</span>
        </button>

        <!-- Permission -->
        <button class="operate-item" onclick={() => openChmodModal(activeMenuItem!)}>
          <i class="ri-folder-lock-fill op-icon icon-amber"></i>
          <span>Permission</span>
        </button>
      {/if}

      <div class="operate-divider"></div>

      <!-- Copy -->
      <button class="operate-item" onclick={() => handleCopyItem(activeMenuItem!)}>
        <i class="ri-file-copy-line op-icon"></i>
        <span>Copy</span>
      </button>

      <!-- Copy Path -->
      <button class="operate-item" onclick={() => copyPathToClipboard(activeMenuItem!)}>
        <i class="ri-clipboard-line op-icon"></i>
        <span>Copy Path</span>
      </button>

      <!-- Cut -->
      <button class="operate-item" onclick={() => handleCutItem(activeMenuItem!)}>
        <i class="ri-scissors-line op-icon icon-cyan"></i>
        <span>Cut</span>
      </button>

      <!-- Rename -->
      <button class="operate-item" onclick={() => openRenameModal(activeMenuItem!)}>
        <i class="ri-edit-box-line op-icon icon-blue"></i>
        <span>Rename</span>
      </button>

      <!-- Delete -->
      <button class="operate-item item-danger" onclick={() => deleteSingleItem(activeMenuItem!)}>
        <i class="ri-delete-bin-line op-icon"></i>
        <span>Delete</span>
      </button>

      <div class="operate-divider"></div>

      <!-- Compress -->
      <button class="operate-item" onclick={() => openCompress(activeMenuItem!)}>
        <i class="ri-file-zip-fill op-icon icon-cyan"></i>
        <span>Compress</span>
      </button>

      <!-- Properties -->
      <button class="operate-item" onclick={() => openProperties(activeMenuItem!)}>
        <i class="ri-information-line op-icon icon-blue"></i>
        <span>Properties</span>
      </button>
    </div>
  {/if}

  <!-- ═════════ MODAL: Remote Download ═════════ -->
  {#if showRemoteModal}
    <div class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <h3>Remote Download (Unduh URL)</h3>
          <button class="btn-close" onclick={() => (showRemoteModal = false)}>&times;</button>
        </div>
        <form onsubmit={handleRemoteDownload}>
          <div class="field">
            <label for="f-rem-url">URL Berkas</label>
            <input
              id="f-rem-url"
              type="text"
              placeholder="https://example.com/package.tar.gz"
              bind:value={remoteUrl}
              required
            />
          </div>
          <div class="field">
            <label for="f-rem-fname">Nama Berkas Simpan (Opsional)</label>
            <input
              id="f-rem-fname"
              type="text"
              placeholder="Kosongkan untuk nama otomatis dari URL"
              bind:value={remoteFilename}
            />
          </div>
          <div class="field-meta">
            Tujuan: <span class="mono">{currentPath}</span>
          </div>
          <div class="modal-foot">
            <button type="button" class="btn btn-ghost" onclick={() => (showRemoteModal = false)}>Batal</button>
            <button type="submit" class="btn btn-primary" disabled={remoteDownloading}>
              {remoteDownloading ? 'Mengunduh...' : 'Mulai Unduh'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Soft Link (Symlink) ═════════ -->
  {#if showSymlinkModal}
    <div class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <h3>Buat Soft Link (Symbolic Link)</h3>
          <button class="btn-close" onclick={() => (showSymlinkModal = false)}>&times;</button>
        </div>
        <form onsubmit={handleCreateSymlink}>
          <div class="field">
            <label for="f-sym-tgt">Path Target (Asal Berkas/Folder)</label>
            <input
              id="f-sym-tgt"
              type="text"
              placeholder="/var/log/nginx atau C:\data\app"
              bind:value={symlinkTarget}
              required
            />
          </div>
          <div class="field">
            <label for="f-sym-name">Nama Tautan Link</label>
            <input
              id="f-sym-name"
              type="text"
              placeholder="contoh: nginx_logs_link"
              bind:value={symlinkName}
              required
            />
          </div>
          <div class="field-meta">
            Lokasi pembuatan: <span class="mono">{currentPath}</span>
          </div>
          <div class="modal-foot">
            <button type="button" class="btn btn-ghost" onclick={() => (showSymlinkModal = false)}>Batal</button>
            <button type="submit" class="btn btn-primary" disabled={creatingSymlink}>
              {creatingSymlink ? 'Membuat...' : 'Buat Tautan'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Search File Content ═════════ -->
  {#if showSearchContentModal}
    <div class="modal-backdrop">
      <div class="modal-card modal-wide">
        <div class="modal-header">
          <h3>Search File Content (Cari Isi Berkas)</h3>
          <button class="btn-close" onclick={() => (showSearchContentModal = false)}>&times;</button>
        </div>
        <form onsubmit={handleSearchContent} class="search-content-form">
          <div class="search-content-input-wrap">
            <input
              type="text"
              placeholder="Masukkan kata kunci yang ingin dicari di dalam teks berkas..."
              bind:value={contentSearchQuery}
              required
            />
            <button type="submit" class="btn btn-primary" disabled={searchingContent}>
              {searchingContent ? 'Mencari...' : 'Cari'}
            </button>
          </div>
        </form>
        <div class="search-results-box">
          {#if searchingContent}
            <div class="search-loading">Memindai berkas di {currentPath}...</div>
          {:else if contentSearchResults.length === 0}
            <div class="search-empty">Tidak ada hasil cocok atau belum dilakukan pencarian.</div>
          {:else}
            <div class="results-list">
              {#each contentSearchResults as res}
                <div class="result-row">
                  <div class="res-head">
                    <span class="res-file font-bold mono">{res.file}</span>
                    <span class="res-line mono">Baris {res.line}</span>
                  </div>
                  <pre class="res-code mono">{res.text}</pre>
                </div>
              {/each}
            </div>
          {/if}
        </div>
        <div class="modal-foot">
          <button class="btn btn-primary" onclick={() => (showSearchContentModal = false)}>Tutup</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Manajemen Favorit ═════════ -->
  {#if showFavManageModal}
    <div class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <h3>Favorite Management</h3>
          <button class="btn-close" onclick={() => (showFavManageModal = false)}>&times;</button>
        </div>
        <div class="fav-manager-body">
          <div class="fav-add-action">
            <button class="btn btn-outline w-full" onclick={addCurrentToFavorites}>
              + Tambahkan Folder Saat Ini ke Favorit
            </button>
          </div>
          <div class="fav-list">
            {#each customFavorites as fav, idx}
              <div class="fav-row">
                <div>
                  <div class="fav-row-name font-bold">{fav.name}</div>
                  <div class="fav-row-path mono text-muted">{fav.path || 'Root'}</div>
                </div>
                <button class="btn-icon link-danger" onclick={() => removeFavorite(idx)} title="Hapus">
                  <i class="ri-delete-bin-line"></i>
                </button>
              </div>
            {/each}
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn btn-primary" onclick={() => (showFavManageModal = false)}>Selesai</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Cloud Storage Connector ═════════ -->
  {#if showCloudStorageModal}
    <div class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <h3>{selectedCloud} Connector</h3>
          <button class="btn-close" onclick={() => (showCloudStorageModal = false)}>&times;</button>
        </div>
        <div class="cloud-info-body">
          <i class="ri-cloud-line cloud-huge-icon"></i>
          <p>Konektor penyimpanan awan untuk <strong>{selectedCloud}</strong> telah terintegrasi.</p>
          <div class="field">
            <label for="f-cloud-ep">Storage Bucket / Endpoint</label>
            <input id="f-cloud-ep" type="text" placeholder="misal: my-backup-bucket atau ftp.domain.com" />
          </div>
          <div class="field">
            <label for="f-cloud-key">Access Key / Credential</label>
            <input id="f-cloud-key" type="password" placeholder="API Key / Password" />
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn btn-ghost" onclick={() => (showCloudStorageModal = false)}>Batal</button>
          <button class="btn btn-primary" onclick={() => { showCloudStorageModal = false; onNotify(`Konfigurasi ${selectedCloud} tersimpan`); }}>
            Hubungkan Sekarang
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Properties (Detail Metadata Berkas) ═════════ -->
  {#if showPropertiesModal && propTarget}
    <div class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <h3>Properties: {propTarget.name}</h3>
          <button class="btn-close" onclick={() => (showPropertiesModal = false)}>&times;</button>
        </div>
        <div class="prop-table">
          <div class="prop-row">
            <span class="prop-label">Name:</span>
            <span class="prop-val font-bold">{propTarget.name}</span>
          </div>
          <div class="prop-row">
            <span class="prop-label">Full Path:</span>
            <span class="prop-val mono select-all">{propTarget.path}</span>
          </div>
          <div class="prop-row">
            <span class="prop-label">Type:</span>
            <span class="prop-val">{propTarget.is_dir ? 'Directory (Folder)' : `File (${propTarget.ext || 'Plain'})`}</span>
          </div>
          <div class="prop-row">
            <span class="prop-label">Size:</span>
            <span class="prop-val mono">{formatBytes(propTarget.size)} ({propTarget.size} bytes)</span>
          </div>
          <div class="prop-row">
            <span class="prop-label">PMSN / Permissions:</span>
            <span class="prop-val mono badge-green">{propTarget.mode}</span>
          </div>
          <div class="prop-row">
            <span class="prop-label">Owner:</span>
            <span class="prop-val mono">{propTarget.owner}</span>
          </div>
          <div class="prop-row">
            <span class="prop-label">Modified Time:</span>
            <span class="prop-val mono">{propTarget.mod_time}</span>
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn btn-primary" onclick={() => (showPropertiesModal = false)}>Tutup</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Kompresi Arsip Zip ═════════ -->
  {#if showCompressModal && compressTarget}
    <div class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <h3>Kompresi Berkas ke Zip</h3>
          <button class="btn-close" onclick={() => (showCompressModal = false)}>&times;</button>
        </div>
        <form onsubmit={handleExecuteCompress}>
          <div class="field">
            <label for="f-zip-src">Item Sumber</label>
            <input id="f-zip-src" type="text" value={compressTarget.name} disabled />
          </div>
          <div class="field">
            <label for="f-zip-name">Nama Arsip Zip</label>
            <input id="f-zip-name" type="text" bind:value={compressZipName} required />
          </div>
          <div class="field-meta">
            Lokasi output: <span class="mono">{currentPath}</span>
          </div>
          <div class="modal-foot">
            <button type="button" class="btn btn-ghost" onclick={() => (showCompressModal = false)}>Batal</button>
            <button type="submit" class="btn btn-primary" disabled={compressing}>
              {compressing ? 'Mengompresi...' : 'Mulai Kompresi'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Buat File / Folder Baru ═════════ -->
  {#if showNewModal}
    <div class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <h3>Buat {newType === 'dir' ? 'Folder' : 'Berkas'} Baru</h3>
          <button class="btn-close" onclick={() => (showNewModal = false)}>&times;</button>
        </div>
        <form onsubmit={handleCreateNew}>
          <div class="field">
            <label for="f-new-type">Tipe Item</label>
            <div class="type-segmented">
              <button
                type="button"
                class="seg-btn {newType === 'file' ? 'active' : ''}"
                onclick={() => (newType = 'file')}
              >
                <i class="ri-file-line"></i> Berkas (File)
              </button>
              <button
                type="button"
                class="seg-btn {newType === 'dir' ? 'active' : ''}"
                onclick={() => (newType = 'dir')}
              >
                <i class="ri-folder-line"></i> Direktori (Folder)
              </button>
            </div>
          </div>
          <div class="field">
            <label for="f-new-name">Nama {newType === 'dir' ? 'Folder' : 'Berkas'}</label>
            <input
              id="f-new-name"
              type="text"
              placeholder={newType === 'dir' ? 'contoh: config atau assets' : 'contoh: index.html atau .env'}
              bind:value={newName}
              required
            />
          </div>
          <div class="field-meta">
            Lokasi: <span class="mono">{currentPath}</span>
          </div>
          <div class="modal-foot">
            <button type="button" class="btn btn-ghost" onclick={() => (showNewModal = false)}>Batal</button>
            <button type="submit" class="btn btn-primary">Buat Sekarang</button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Ganti Nama (Rename) ═════════ -->
  {#if showRenameModal && renameTarget}
    <div class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <h3>Ganti Nama (Rename)</h3>
          <button class="btn-close" onclick={() => (showRenameModal = false)}>&times;</button>
        </div>
        <form onsubmit={handleRename}>
          <div class="field">
            <label for="f-old-name">Nama Saat Ini</label>
            <input id="f-old-name" type="text" value={renameTarget.name} disabled />
          </div>
          <div class="field">
            <label for="f-new-rname">Nama Baru</label>
            <input id="f-new-rname" type="text" bind:value={renameNewName} required />
          </div>
          <div class="modal-foot">
            <button type="button" class="btn btn-ghost" onclick={() => (showRenameModal = false)}>Batal</button>
            <button type="submit" class="btn btn-primary">Simpan Nama Baru</button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Hak Akses / PMSN (chmod) ═════════ -->
  {#if showChmodModal && chmodTarget}
    <div class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <h3>Atur Perizinan (Permission / chmod)</h3>
          <button class="btn-close" onclick={() => (showChmodModal = false)}>&times;</button>
        </div>
        <div class="pmsn-body">
          <p class="target-name mono">{chmodTarget.name}</p>
          <div class="chmod-matrix">
            <div class="matrix-col">
              <h4>Owner</h4>
              <label><input type="checkbox" bind:checked={chmodPerms.ownerR} /> Read (4)</label>
              <label><input type="checkbox" bind:checked={chmodPerms.ownerW} /> Write (2)</label>
              <label><input type="checkbox" bind:checked={chmodPerms.ownerX} /> Execute (1)</label>
            </div>
            <div class="matrix-col">
              <h4>Group</h4>
              <label><input type="checkbox" bind:checked={chmodPerms.groupR} /> Read (4)</label>
              <label><input type="checkbox" bind:checked={chmodPerms.groupW} /> Write (2)</label>
              <label><input type="checkbox" bind:checked={chmodPerms.groupX} /> Execute (1)</label>
            </div>
            <div class="matrix-col">
              <h4>Public</h4>
              <label><input type="checkbox" bind:checked={chmodPerms.publicR} /> Read (4)</label>
              <label><input type="checkbox" bind:checked={chmodPerms.publicW} /> Write (2)</label>
              <label><input type="checkbox" bind:checked={chmodPerms.publicX} /> Execute (1)</label>
            </div>
          </div>

          <div class="octal-display">
            <span>Nilai chmod:</span>
            <span class="octal-badge mono">{chmodOctal}</span>
          </div>

          <div class="modal-foot">
            <button type="button" class="btn btn-ghost" onclick={() => (showChmodModal = false)}>Batal</button>
            <button type="button" class="btn btn-primary" onclick={handleSaveChmod}>Terapkan Izin</button>
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Editor Berkas Teks ═════════ -->
  <!-- ═════════ MODAL: Modern Code Studio Editor ═════════ -->
  {#if showEditorModal && editorTarget}
    <div
      class="modal-backdrop editor-backdrop"
      role="dialog"
      aria-modal="true"
      onclick={(e) => { if (e.target === e.currentTarget) closeEditorModal(); }}
      onkeydown={(e) => { if (e.key === 'Escape') closeEditorModal(); }}
      tabindex="-1"
    >
      <div class="studio-window {editorIsFullscreen ? 'is-fullscreen' : ''}">
        <!-- Bar Judul: Tab Berkas Aktif + Breadcrumb + Kontrol Jendela -->
        <div class="studio-titlebar">
          <div class="studio-tab-bar">
            <div class="studio-tab active">
              <i class="{editorLangInfo.icon} tab-icon" style="color: {editorLangInfo.color};"></i>
              <span class="tab-title">{editorTarget.name}</span>
              {#if editorIsDirty}
                <span class="dirty-dot" title="Ada perubahan belum disimpan">●</span>
              {/if}
            </div>
            <div class="studio-path-crumbs mono" title={editorTarget.path}>
              <i class="ri-folder-2-line"></i>
              <span>{editorTarget.path}</span>
            </div>
          </div>

          <div class="studio-win-controls">
            <button
              class="win-ctrl-btn"
              onclick={() => (editorIsFullscreen = !editorIsFullscreen)}
              title={editorIsFullscreen ? 'Keluar Layar Penuh' : 'Layar Penuh'}
            >
              <i class="{editorIsFullscreen ? 'ri-fullscreen-exit-line' : 'ri-fullscreen-line'}"></i>
            </button>
            <button
              class="win-ctrl-btn btn-win-close"
              onclick={closeEditorModal}
              title="Tutup Editor"
            >
              <i class="ri-close-line"></i>
            </button>
          </div>
        </div>

        <!-- Toolbar Studio Modern -->
        <div class="studio-toolbar">
          <div class="tb-left">
            <button
              class="studio-btn studio-btn-save {editorIsDirty ? 'glow-green' : ''}"
              onclick={handleSaveEditor}
              disabled={editorSaving}
              title="Simpan berkas (Ctrl+S)"
            >
              {#if editorSaving}
                <i class="ri-loader-4-line spin"></i>
                <span>Menyimpan...</span>
              {:else}
                <i class="ri-save-3-line"></i>
                <span>Simpan</span>
                <span class="kbd-hint">Ctrl+S</span>
              {/if}
            </button>

            <button
              class="studio-btn"
              onclick={refreshEditorContent}
              title="Muat ulang isi berkas dari server (Refresh)"
            >
              <i class="ri-refresh-line"></i>
              <span>Refresh</span>
            </button>

            <button
              class="studio-btn {editorShowSearch ? 'active' : ''}"
              onclick={() => (editorShowSearch = !editorShowSearch)}
              title="Cari & Ganti Teks (Ctrl+F)"
            >
              <i class="ri-search-line"></i>
              <span>Cari & Ganti</span>
            </button>

            <button
              class="studio-btn"
              onclick={handleEditorJumpLine}
              title="Lompat ke Baris (Ctrl+G)"
            >
              <i class="ri-arrow-right-up-line"></i>
              <span>Lompat Baris</span>
            </button>
          </div>

          <div class="tb-right">
            <!-- Pilihan Ukuran Font -->
            <div class="tool-group">
              <span class="tool-label">Font:</span>
              <button
                class="font-step-btn"
                onclick={() => editorFontSize = Math.max(11, editorFontSize - 1)}
                title="Perkecil Font"
              >A-</button>
              <span class="font-size-val mono">{editorFontSize}px</span>
              <button
                class="font-step-btn"
                onclick={() => editorFontSize = Math.min(22, editorFontSize + 1)}
                title="Perbesar Font"
              >A+</button>
            </div>

            <!-- Toggle Word Wrap -->
            <button
              class="studio-btn {editorWordWrap ? 'active' : ''}"
              onclick={() => (editorWordWrap = !editorWordWrap)}
              title="Toggle Word Wrap"
            >
              <i class="ri-text-wrap"></i>
              <span>Wrap</span>
            </button>
          </div>
        </div>

        <!-- Panel Pencarian dan Penggantian Teks Floating -->
        {#if editorShowSearch}
          <div class="studio-search-bar">
            <div class="search-row">
              <div class="search-input-wrap">
                <i class="ri-search-line search-icon"></i>
                <input
                  type="text"
                  placeholder="Cari kata kunci..."
                  bind:value={editorSearchQuery}
                  onkeydown={(e) => {
                    if (e.key === 'Enter') searchInEditor(e.shiftKey ? 'prev' : 'next');
                    if (e.key === 'Escape') editorShowSearch = false;
                  }}
                />
                {#if editorSearchMatches.length > 0}
                  <span class="match-count mono">{editorSearchMatchIndex + 1} dari {editorSearchMatches.length}</span>
                {:else if editorSearchQuery}
                  <span class="match-count mono text-muted">0 hasil</span>
                {/if}
              </div>

              <div class="search-nav-btns">
                <button class="search-action-btn" onclick={() => searchInEditor('prev')} title="Sebelumnya (Shift+Enter)">
                  <i class="ri-arrow-up-s-line"></i>
                </button>
                <button class="search-action-btn" onclick={() => searchInEditor('next')} title="Berikutnya (Enter)">
                  <i class="ri-arrow-down-s-line"></i>
                </button>
              </div>

              <div class="replace-input-wrap">
                <i class="ri-find-replace-line search-icon"></i>
                <input
                  type="text"
                  placeholder="Ganti dengan..."
                  bind:value={editorReplaceQuery}
                  onkeydown={(e) => {
                    if (e.key === 'Enter') replaceInEditor();
                    if (e.key === 'Escape') editorShowSearch = false;
                  }}
                />
              </div>

              <button class="search-action-btn replace-btn" onclick={replaceInEditor} title="Ganti yang dipilih">
                <span>Ganti</span>
              </button>
              <button class="search-action-btn replace-btn" onclick={replaceAllInEditor} title="Ganti semua kemunculan">
                <span>Ganti Semua</span>
              </button>

              <button class="search-close-btn" onclick={() => (editorShowSearch = false)} title="Tutup Pencarian">
                <i class="ri-close-line"></i>
              </button>
            </div>
          </div>
        {/if}

        <!-- Workspace: Gutter Nomor Baris + Textarea Editor -->
        <div class="studio-body">
          <div class="studio-gutter mono" bind:this={editorGutterEl}>
            {#each editorLines as lineNum}
              <div class="gutter-line {editorCursorPos.line === lineNum ? 'active-line' : ''}">{lineNum}</div>
            {/each}
          </div>

          <div class="studio-canvas">
            <textarea
              class="studio-textarea mono"
              bind:this={editorTextareaEl}
              bind:value={editorContent}
              style="font-size: {editorFontSize}px; white-space: {editorWordWrap ? 'pre-wrap' : 'pre'};"
              spellcheck="false"
              onscroll={handleEditorScroll}
              onkeydown={handleEditorKeydown}
              onclick={updateEditorCursor}
              onkeyup={updateEditorCursor}
              onselect={updateEditorCursor}
            ></textarea>
          </div>
        </div>

        <!-- Status Bar Bawah Studio -->
        <div class="studio-statusbar mono">
          <div class="sb-left">
            <span class="sb-item" title="Path berkas">
              <i class="ri-file-text-line"></i>
              <span class="sb-text">{editorTarget.path}</span>
            </span>
            <span class="sb-divider">|</span>
            <span class="sb-item">{editorLineCount} baris</span>
            <span class="sb-item">{editorContent.length} karakter</span>
            <span class="sb-item">{formatBytes(editorTarget.size)}</span>
          </div>

          <div class="sb-right">
            <span class="sb-item cursor-pos">Ln {editorCursorPos.line}, Col {editorCursorPos.col}</span>
            <span class="sb-divider">|</span>
            <span class="sb-item">LF</span>
            <span class="sb-divider">|</span>
            <span class="sb-item">Spasi: 2</span>
            <span class="sb-divider">|</span>
            <span class="sb-item">UTF-8</span>
            <span class="sb-divider">|</span>
            <span class="sb-item lang-tag" style="color: {editorLangInfo.color};">
              <i class="{editorLangInfo.icon}"></i>
              <span>{editorLangInfo.label}</span>
            </span>
            <span class="sb-divider">|</span>
            {#if editorSaving}
              <span class="save-status saving">
                <i class="ri-loader-4-line spin"></i>
                <span>Menyimpan...</span>
              </span>
            {:else if editorIsDirty}
              <span class="save-status unsaved">
                <span class="dirty-indicator">●</span>
                <span>Belum disimpan</span>
              </span>
            {:else}
              <span class="save-status saved">
                <i class="ri-checkbox-circle-fill"></i>
                <span>Tersimpan</span>
              </span>
            {/if}
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- ═════════ MODAL: Upload Berkas ═════════ -->
  {#if showUploadModal}
    <div class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <h3>Upload Berkas ke Server</h3>
          <button class="btn-close" onclick={() => (showUploadModal = false)}>&times;</button>
        </div>
        <form onsubmit={handleUploadSubmit}>
          <div class="field">
            <label for="f-upload-dest">Direktori Tujuan</label>
            <input id="f-upload-dest" type="text" value={currentPath} disabled />
          </div>
          <div class="field">
            <label for="f-upload-input">Pilih Berkas</label>
            <input
              id="f-upload-input"
              type="file"
              multiple
              onchange={(e) => (uploadFiles = (e.target as HTMLInputElement).files)}
              required
            />
          </div>
          <div class="modal-foot">
            <button type="button" class="btn btn-ghost" onclick={() => (showUploadModal = false)}>Batal</button>
            <button type="submit" class="btn btn-primary" disabled={uploadSubmitting}>
              {uploadSubmitting ? 'Mengunggah...' : 'Mulai Upload'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}
</div>

<style>
  .files-view {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    height: 100%;
  }

  /* 1. Multi-Tab Bar */
  .tabs-bar {
    display: flex;
    align-items: center;
    background: var(--bg-surface);
    border-radius: 8px;
    padding: 0.25rem 0.5rem;
    border: 1px solid var(--border);
  }
  .tab-list {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    overflow-x: auto;
  }
  .tab-item {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    background: var(--bg-hover);
    color: var(--text-muted);
    padding: 0.35rem 0.75rem;
    border-radius: 6px;
    font-size: 0.8rem;
    cursor: pointer;
    border: 1px solid transparent;
    user-select: none;
    transition: all 0.15s ease;
  }
  .tab-item:hover {
    color: var(--text);
    background: var(--bg-active);
  }
  .tab-item.active {
    background: var(--bg-raised);
    color: var(--accent);
    border-color: color-mix(in srgb, var(--accent) 27%, transparent);
    font-weight: 600;
  }
  .tab-icon {
    font-size: 0.95rem;
    color: var(--warning);
  }
  .tab-close {
    background: transparent;
    border: none;
    color: var(--text-subtle);
    font-size: 1rem;
    cursor: pointer;
    line-height: 1;
    padding: 0 0.15rem;
  }
  .tab-close:hover {
    color: var(--danger-strong);
  }
  .btn-add-tab {
    background: transparent;
    border: 1px dashed var(--border-strong);
    color: var(--text-muted);
    padding: 0.3rem 0.5rem;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.9rem;
    display: flex;
    align-items: center;
  }
  .btn-add-tab:hover {
    background: var(--bg-active);
    color: var(--text);
  }

  /* 2. Breadcrumbs & Search */
  .nav-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.4rem 0.6rem;
  }
  .nav-left {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex: 1;
    min-width: 0;
  }
  .btn-icon {
    background: var(--bg-active);
    border: 1px solid var(--border-strong);
    color: var(--text-secondary);
    width: 28px;
    height: 28px;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
  }
  .btn-icon:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .btn-icon:not(:disabled):hover {
    background: var(--border-strong);
    color: var(--text);
  }
  .breadcrumb-trail {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.25rem 0.6rem;
    flex: 1;
    overflow-x: auto;
    white-space: nowrap;
    font-size: 0.8rem;
  }
  .path-icon {
    color: var(--warning);
    font-size: 0.95rem;
  }
  .crumb-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0.1rem 0.25rem;
    border-radius: 4px;
    font-size: 0.8rem;
  }
  .crumb-btn:hover {
    color: var(--accent);
    background: var(--bg-active);
  }
  .crumb-sep {
    color: var(--text-disabled);
    font-size: 0.75rem;
  }
  .nav-right {
    display: flex;
    align-items: center;
  }
  .search-box {
    display: flex;
    align-items: center;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0 0.5rem;
    transition: border-color 0.15s ease, box-shadow 0.15s ease;
  }
  .search-box:focus-within {
    border-color: color-mix(in srgb, var(--accent) 40%, transparent);
    box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 20%, transparent);
  }
  .search-icon {
    color: var(--text-subtle);
    font-size: 0.85rem;
  }
  .search-box input {
    background: transparent;
    border: none;
    padding: 0.35rem 0.5rem;
    color: var(--text);
    font-size: 0.78rem;
    outline: none;
    width: 200px;
  }
  .btn-clear-search {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0.1rem;
    display: flex;
    align-items: center;
  }
  .btn-clear-search:hover {
    color: var(--text);
  }

  /* 3. Action Toolbar */
  .action-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.35rem 0.6rem;
    flex-wrap: wrap;
  }
  .tool-left, .tool-right {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .tool-dropdown-wrap {
    position: relative;
    display: inline-block;
  }
  .tool-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background: var(--bg-hover);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 0.75rem;
    padding: 0.35rem 0.65rem;
    border-radius: 6px;
    cursor: pointer;
    font-weight: 500;
    transition: all 0.15s ease;
  }
  .tool-btn:hover {
    background: var(--bg-active);
    color: var(--text);
    border-color: var(--border-strong);
  }
  .tool-btn.is-active {
    background: var(--bg-raised);
    border-color: var(--accent);
    color: var(--accent);
    font-weight: 600;
  }
  .toolbar-dropdown-menu {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    background: var(--bg-raised);
    border: 1px solid var(--border-strong);
    border-radius: 8px;
    box-shadow: 0 16px 40px rgba(0, 0, 0, 0.75);
    min-width: 200px;
    z-index: 999;
    padding: 0.35rem 0;
    display: flex;
    flex-direction: column;
    animation: fadeIn 0.12s ease;
  }
  .tb-menu-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.65rem;
    padding: 0.45rem 0.85rem;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    font-size: 0.78rem;
    cursor: pointer;
    text-align: left;
    width: 100%;
    transition: background 0.1s ease, color 0.1s ease;
  }
  .tb-menu-item:hover:not(:disabled) {
    background: var(--bg-active);
    color: var(--text);
  }
  .tb-menu-item:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .tb-menu-item.item-danger:hover:not(:disabled) {
    background: var(--bg-active);
    color: var(--danger-strong);
  }
  .tb-item-left {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .badge-vip {
    color: var(--warning);
    font-size: 0.8rem;
  }

  .tool-primary {
    background: var(--bg-raised);
    border-color: color-mix(in srgb, var(--accent) 33%, transparent);
    color: var(--accent);
    font-weight: 600;
  }
  .tool-primary:hover {
    background: var(--bg-active);
    color: var(--accent-strong);
  }
  .paste-indicator {
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
  }
  .btn-cancel-clip {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    font-size: 1rem;
    padding: 0 0.2rem;
  }
  .btn-cancel-clip:hover {
    color: var(--danger-strong);
  }

  /* View Mode Switcher */
  .view-toggle {
    display: flex;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    overflow: hidden;
  }
  .toggle-btn {
    background: transparent;
    border: none;
    color: var(--text-subtle);
    padding: 0.25rem 0.5rem;
    cursor: pointer;
    font-size: 0.85rem;
  }
  .toggle-btn.active {
    background: var(--accent);
    color: var(--text-on-accent);
  }

  /* 4. Content Panel & Table */
  .file-content-panel {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    min-height: 480px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .table-container {
    overflow-x: auto;
    width: 100%;
  }
  .file-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.78rem;
    text-align: left;
  }
  .file-table th {
    background: var(--bg-raised);
    color: var(--text-muted);
    padding: 0.55rem 0.75rem;
    font-weight: 600;
    border-bottom: 1px solid var(--border);
    user-select: none;
  }
  .file-table td {
    padding: 0.45rem 0.75rem;
    border-bottom: 1px solid var(--border-subtle);
    color: var(--text-secondary);
  }
  .file-table tr:hover {
    background: var(--bg-hover);
  }
  .selected-row {
    background: var(--bg-raised) !important;
  }
  .highlight-row {
    background: var(--bg-active) !important;
  }
  .col-check {
    width: 32px;
    text-align: center;
  }
  .col-name {
    min-width: 240px;
  }
  .name-cell {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
  }
  .name-cell:hover .item-name {
    color: var(--accent);
    text-decoration: underline;
  }
  .item-icon {
    font-size: 1.1rem;
    flex-shrink: 0;
  }
  .item-name {
    color: var(--text);
    font-weight: 500;
  }
  .dir-name {
    font-weight: 600;
  }
  .badge-prot {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    color: var(--text-muted);
    font-size: 0.72rem;
  }
  .pmsn-pill {
    background: var(--bg-hover);
    border: 1px solid var(--border);
    color: var(--accent);
    font-family: 'Fira Code', monospace;
    font-size: 0.72rem;
    padding: 0.15rem 0.4rem;
    border-radius: 4px;
    cursor: pointer;
  }
  .pmsn-pill:hover {
    border-color: var(--accent);
  }
  .calc-text {
    color: var(--accent);
    cursor: pointer;
  }
  .col-ops {
    width: 170px;
    text-align: right;
  }

  /* Operation Row Styling */
  .row-operations {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.4rem;
    position: relative;
  }
  .hover-shortcuts {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.15s ease, visibility 0.15s ease;
  }
  .file-table tr:hover .hover-shortcuts {
    opacity: 1;
    visibility: visible;
  }
  .quick-icon-btn {
    background: var(--bg-active);
    border: 1px solid var(--border-strong);
    color: var(--text-secondary);
    width: 24px;
    height: 24px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    font-size: 0.78rem;
    transition: all 0.12s ease;
  }
  .quick-icon-btn:hover {
    background: var(--bg-active);
    color: var(--text);
    border-color: var(--border-strong);
  }
  .quick-icon-btn.link-danger:hover {
    background: var(--bg-active);
    color: var(--danger-strong);
    border-color: var(--danger);
  }

  .btn-operate {
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
    background: transparent;
    border: 1px solid transparent;
    color: var(--accent);
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    transition: all 0.12s ease;
  }
  .btn-operate:hover, .btn-operate.is-active {
    background: color-mix(in srgb, var(--accent) 10%, transparent);
    border-color: color-mix(in srgb, var(--accent) 27%, transparent);
  }
  .btn-operate i {
    font-size: 0.9rem;
    transition: transform 0.15s ease;
  }
  .btn-operate.is-active i {
    transform: rotate(180deg);
  }

  /* Floating Dropdown Menu */
  .operate-dropdown {
    position: fixed;
    background: var(--bg-raised);
    border: 1px solid var(--border-strong);
    border-radius: 8px;
    box-shadow: 0 16px 40px rgba(0, 0, 0, 0.75);
    min-width: 215px;
    z-index: 9999;
    padding: 0.35rem 0;
    display: flex;
    flex-direction: column;
    animation: fadeIn 0.12s ease;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(-4px); }
    to { opacity: 1; transform: translateY(0); }
  }
  .operate-item {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0.45rem 0.85rem;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    font-size: 0.78rem;
    cursor: pointer;
    text-align: left;
    width: 100%;
    transition: background 0.1s ease, color 0.1s ease;
  }
  .operate-item:hover {
    background: var(--bg-active);
    color: var(--text);
  }
  .operate-item.item-danger:hover {
    background: var(--bg-active);
    color: var(--danger-strong);
  }
  .operate-divider {
    height: 1px;
    background: var(--bg-active);
    margin: 0.3rem 0;
  }
  .op-icon {
    font-size: 1rem;
    width: 18px;
    text-align: center;
  }
  .icon-amber { color: var(--warning); }
  .icon-blue { color: var(--info); }
  .icon-green { color: var(--accent); }
  .icon-cyan { color: var(--info); }
  .icon-yellow { color: var(--warning); }

  /* Grid Mode */
  .grid-container {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
    gap: 0.75rem;
    padding: 1rem;
  }
  .grid-card {
    background: var(--bg-hover);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.6rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    cursor: pointer;
    transition: all 0.15s ease;
    position: relative;
  }
  .grid-card:hover {
    background: var(--bg-active);
    border-color: var(--border-strong);
  }
  .selected-card {
    border-color: var(--accent);
    background: var(--bg-raised);
  }
  .card-top {
    width: 100%;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .card-operate-btn {
    background: transparent;
    border: none;
    color: var(--text-subtle);
    cursor: pointer;
    font-size: 1rem;
    padding: 0 0.2rem;
  }
  .card-operate-btn:hover {
    color: var(--accent);
  }
  .card-icon {
    font-size: 2.2rem;
    margin: 0.5rem 0;
  }
  .card-name {
    font-size: 0.75rem;
    color: var(--text);
    text-align: center;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    width: 100%;
  }
  .card-size {
    font-size: 0.68rem;
    color: var(--text-muted);
    margin-top: 0.2rem;
  }

  /* Loading & Empty States */
  .state-box {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 4rem 1rem;
    color: var(--text-subtle);
    gap: 0.75rem;
  }
  .spin-icon {
    font-size: 1.8rem;
    animation: spin 1s linear infinite;
    color: var(--accent);
  }
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
  .empty-icon {
    font-size: 3rem;
    color: var(--text-disabled);
  }
  .empty-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.5rem;
  }

  /* 5. Footer Bar */
  .footer-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.5rem 0.75rem;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: 8px;
    font-size: 0.75rem;
    color: var(--text-muted);
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .foot-left {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .foot-left strong {
    color: var(--text);
  }
  .foot-right {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .pager {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }
  .btn-page {
    background: var(--bg-hover);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    width: 24px;
    height: 24px;
    border-radius: 4px;
    cursor: pointer;
  }
  .btn-page:disabled {
    opacity: 0.35;
    cursor: not-allowed;
  }
  .page-num {
    background: var(--bg-hover);
    border: 1px solid var(--border);
    color: var(--accent);
    font-weight: 600;
    padding: 0.15rem 0.55rem;
    border-radius: 4px;
  }
  .select-pagesize {
    background: var(--bg-hover);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    border-radius: 4px;
    padding: 0.15rem 0.4rem;
    font-size: 0.72rem;
  }
  .goto-box {
    display: flex;
    align-items: center;
    gap: 0.3rem;
  }
  .goto-box input {
    width: 44px;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text);
    padding: 0.1rem 0.3rem;
    font-size: 0.72rem;
    text-align: center;
  }
  .total-badge {
    background: var(--bg-active);
    color: var(--text-muted);
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    font-size: 0.72rem;
  }

  /* Modal Styles */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.75);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10000;
    backdrop-filter: blur(4px);
    padding: 1rem;
  }
  .modal-card {
    background: var(--bg-raised);
    border: 1px solid var(--border-strong);
    border-radius: 10px;
    width: 100%;
    max-width: 460px;
    padding: 1.25rem;
    box-shadow: 0 16px 36px rgba(0, 0, 0, 0.6);
  }
  .modal-wide {
    max-width: 680px;
  }
  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
    padding-bottom: 0.6rem;
    border-bottom: 1px solid var(--border);
  }
  .modal-header h3 {
    font-size: 1rem;
    color: var(--text);
  }
  .btn-close {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 1.3rem;
    cursor: pointer;
  }
  .field {
    margin-bottom: 1rem;
  }
  .field label {
    display: block;
    font-size: 0.75rem;
    color: var(--text-muted);
    margin-bottom: 0.35rem;
  }
  .field input[type="text"], .field input[type="file"], .field input[type="password"] {
    width: 100%;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.45rem 0.65rem;
    color: var(--text);
    font-size: 0.8rem;
    outline: none;
  }
  .field input:disabled {
    opacity: 0.6;
    background: var(--bg-inset);
  }
  .type-segmented {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.5rem;
  }
  .seg-btn {
    background: var(--bg-active);
    border: 1px solid var(--border-strong);
    color: var(--text-muted);
    padding: 0.4rem;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.78rem;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
  }
  .seg-btn.active {
    background: color-mix(in srgb, var(--accent) 13%, transparent);
    border-color: var(--accent);
    color: var(--accent);
    font-weight: 600;
  }
  .field-meta {
    font-size: 0.72rem;
    color: var(--text-subtle);
    margin-bottom: 1rem;
    word-break: break-all;
  }
  .modal-foot {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 1rem;
  }
  .btn {
    padding: 0.45rem 0.85rem;
    border-radius: 6px;
    font-size: 0.78rem;
    cursor: pointer;
    font-weight: 600;
    border: none;
  }
  .btn-primary {
    background: var(--accent);
    color: var(--text-on-accent);
  }
  .btn-primary:hover {
    background: var(--accent);
  }
  .btn-ghost {
    background: transparent;
    color: var(--text-muted);
  }
  .btn-ghost:hover {
    color: var(--text);
    background: var(--bg-active);
  }
  .btn-outline {
    background: transparent;
    border: 1px solid var(--border-strong);
    color: var(--text-secondary);
  }
  .w-full {
    width: 100%;
  }

  /* Search File Content */
  .search-content-form {
    margin-bottom: 1rem;
  }
  .search-content-input-wrap {
    display: flex;
    gap: 0.5rem;
  }
  .search-content-input-wrap input {
    flex: 1;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.45rem 0.65rem;
    color: var(--text);
    font-size: 0.8rem;
  }
  .search-results-box {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 6px;
    max-height: 320px;
    overflow-y: auto;
    padding: 0.5rem;
  }
  .search-loading, .search-empty {
    text-align: center;
    padding: 2rem;
    color: var(--text-subtle);
    font-size: 0.78rem;
  }
  .results-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .result-row {
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 0.45rem 0.65rem;
  }
  .res-head {
    display: flex;
    justify-content: space-between;
    margin-bottom: 0.25rem;
  }
  .res-file {
    color: var(--info);
    font-size: 0.75rem;
  }
  .res-line {
    color: var(--accent);
    font-size: 0.72rem;
  }
  .res-code {
    background: var(--bg-inset);
    padding: 0.35rem 0.5rem;
    border-radius: 4px;
    color: var(--text-secondary);
    font-size: 0.75rem;
    overflow-x: auto;
    margin: 0;
  }

  /* Favorite Manager */
  .fav-manager-body {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .fav-list {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    max-height: 240px;
    overflow-y: auto;
  }
  .fav-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--bg-surface);
    padding: 0.4rem 0.65rem;
    border-radius: 6px;
    border: 1px solid var(--border);
  }
  .fav-row-name {
    font-size: 0.78rem;
    color: var(--text);
  }
  .fav-row-path {
    font-size: 0.7rem;
  }

  /* Cloud Storage Body */
  .cloud-info-body {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    text-align: center;
  }
  .cloud-huge-icon {
    font-size: 3rem;
    color: var(--info);
    margin: 0.5rem 0;
  }
  .cloud-info-body p {
    font-size: 0.8rem;
    color: var(--text-muted);
  }

  /* Properties Table */
  .prop-table {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.75rem;
    font-size: 0.78rem;
  }
  .prop-row {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    border-bottom: 1px solid var(--border-subtle);
    padding-bottom: 0.4rem;
  }
  .prop-label {
    color: var(--text-subtle);
    font-size: 0.7rem;
    text-transform: uppercase;
  }
  .prop-val {
    color: var(--text);
    word-break: break-all;
  }
  .badge-green {
    color: var(--accent);
    font-weight: 700;
  }

  /* PMSN Matrix */
  .pmsn-body {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .target-name {
    font-size: 0.8rem;
    color: var(--info);
    background: var(--bg-surface);
    padding: 0.35rem 0.5rem;
    border-radius: 4px;
    word-break: break-all;
  }
  .chmod-matrix {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 0.75rem;
    background: var(--bg-surface);
    padding: 0.75rem;
    border-radius: 6px;
    border: 1px solid var(--border);
  }
  .matrix-col h4 {
    font-size: 0.75rem;
    color: var(--text);
    margin-bottom: 0.4rem;
    border-bottom: 1px solid var(--border);
    padding-bottom: 0.2rem;
  }
  .matrix-col label {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.72rem;
    color: var(--text-secondary);
    margin-bottom: 0.3rem;
    cursor: pointer;
  }
  .octal-display {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.8rem;
    color: var(--text-muted);
  }
  .octal-badge {
    background: var(--bg-raised);
    color: var(--accent);
    border: 1px solid color-mix(in srgb, var(--accent) 27%, transparent);
    padding: 0.2rem 0.6rem;
    border-radius: 4px;
    font-weight: 700;
  }

  /* ═════════ MODERN CODE STUDIO EDITOR STYLES ═════════ */
  .editor-backdrop {
    background: color-mix(in srgb, var(--bg-app) 85%, transparent);
    backdrop-filter: blur(10px);
    z-index: 1050;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .studio-window {
    background: var(--bg-surface);
    border: 1px solid var(--border-strong);
    border-radius: 12px;
    width: 95vw;
    max-width: 1440px;
    height: 88vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 25px 60px -15px rgba(0, 0, 0, 0.9), 0 0 0 1px color-mix(in srgb, var(--accent) 12%, transparent);
    overflow: hidden;
    transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .studio-window.is-fullscreen {
    width: 100vw;
    max-width: 100vw;
    height: 100vh;
    border-radius: 0;
    border: none;
  }

  /* Bar Judul Atas: Tab + Breadcrumb + Kontrol */
  .studio-titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 0.8rem;
    height: 42px;
    background: var(--bg-raised);
    border-bottom: 1px solid var(--border);
    user-select: none;
  }

  .studio-tab-bar {
    display: flex;
    align-items: center;
    gap: 1rem;
    overflow: hidden;
  }

  .studio-tab {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.35rem 0.8rem;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-bottom: none;
    border-top: 2px solid var(--accent);
    border-radius: 6px 6px 0 0;
    color: var(--text);
    font-size: 0.82rem;
    font-weight: 500;
  }

  .studio-tab .tab-icon {
    font-size: 1rem;
  }

  .studio-tab .dirty-dot {
    color: var(--warning);
    font-size: 0.75rem;
    margin-left: 2px;
    animation: pulse 1.5s infinite;
  }

  .studio-path-crumbs {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.72rem;
    color: var(--text-subtle);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .studio-win-controls {
    display: flex;
    align-items: center;
    gap: 0.3rem;
  }

  .win-ctrl-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 6px;
    color: var(--text-muted);
    cursor: pointer;
    font-size: 1.05rem;
    transition: all 0.15s ease;
  }

  .win-ctrl-btn:hover {
    background: var(--bg-active);
    color: var(--text);
  }

  .win-ctrl-btn.btn-win-close:hover {
    background: var(--danger-strong);
    color: var(--text);
  }

  /* Toolbar Studio Aksi */
  .studio-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.35rem 0.8rem;
    background: var(--bg-raised);
    border-bottom: 1px solid var(--border);
    gap: 0.8rem;
  }

  .studio-toolbar .tb-left,
  .studio-toolbar .tb-right {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .studio-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.3rem 0.65rem;
    background: var(--bg-hover);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text-secondary);
    font-size: 0.76rem;
    cursor: pointer;
    font-weight: 500;
    transition: all 0.15s ease;
  }

  .studio-btn:hover {
    background: var(--bg-active);
    color: var(--text);
    border-color: var(--border-strong);
  }

  .studio-btn.active {
    background: var(--bg-hover);
    border-color: var(--accent);
    color: var(--accent);
  }

  .studio-btn-save {
    background: var(--accent-hover);
    border-color: var(--accent-hover);
    color: var(--text);
  }

  .studio-btn-save:hover:not(:disabled) {
    background: var(--accent-hover);
    border-color: var(--accent);
    color: var(--text);
  }

  .studio-btn-save.glow-green {
    background: var(--accent-hover);
    border-color: var(--accent);
    box-shadow: 0 0 10px color-mix(in srgb, var(--accent) 35%, transparent);
  }

  .studio-btn-save:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .kbd-hint {
    background: rgba(0, 0, 0, 0.25);
    padding: 0.1rem 0.35rem;
    border-radius: 4px;
    font-size: 0.65rem;
    font-family: monospace;
    opacity: 0.85;
  }

  .tool-group {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    background: var(--bg-hover);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.15rem 0.4rem;
  }

  .tool-label {
    font-size: 0.7rem;
    color: var(--text-subtle);
    margin-right: 2px;
  }

  .font-step-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 0.75rem;
    padding: 0.15rem 0.35rem;
    border-radius: 4px;
    cursor: pointer;
  }

  .font-step-btn:hover {
    background: var(--border-strong);
    color: var(--text);
  }

  .font-size-val {
    font-size: 0.72rem;
    color: var(--info);
    min-width: 32px;
    text-align: center;
  }

  /* Floating Search & Replace Bar */
  .studio-search-bar {
    background: var(--bg-hover);
    border-bottom: 1px solid var(--border);
    padding: 0.4rem 0.8rem;
    animation: slideDown 0.18s ease-out;
  }

  .search-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .search-input-wrap,
  .replace-input-wrap {
    display: flex;
    align-items: center;
    background: var(--bg-inset);
    border: 1px solid var(--border-strong);
    border-radius: 6px;
    padding: 0.25rem 0.5rem;
    gap: 0.4rem;
  }

  .search-input-wrap {
    flex: 1;
    min-width: 200px;
  }

  .replace-input-wrap {
    flex: 1;
    min-width: 200px;
  }

  .search-input-wrap input,
  .replace-input-wrap input {
    background: transparent;
    border: none;
    outline: none;
    color: var(--text);
    font-size: 0.78rem;
    width: 100%;
  }

  .search-icon {
    color: var(--text-subtle);
    font-size: 0.9rem;
  }

  .match-count {
    font-size: 0.7rem;
    color: var(--accent);
    white-space: nowrap;
    padding-left: 0.3rem;
  }

  .search-nav-btns {
    display: flex;
    gap: 2px;
  }

  .search-action-btn {
    background: var(--bg-active);
    border: 1px solid var(--border-strong);
    color: var(--text-secondary);
    border-radius: 5px;
    padding: 0.25rem 0.5rem;
    font-size: 0.75rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .search-action-btn:hover {
    background: var(--border-strong);
    color: var(--text);
  }

  .replace-btn {
    font-size: 0.72rem;
    padding: 0.3rem 0.6rem;
  }

  .search-close-btn {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 1.1rem;
    cursor: pointer;
    padding: 0.2rem;
  }

  .search-close-btn:hover {
    color: var(--text);
  }

  /* Body Studio: Gutter + Textarea Canvas */
  .studio-body {
    flex: 1;
    display: flex;
    background: var(--bg-inset);
    overflow: hidden;
    position: relative;
  }

  .studio-gutter {
    width: 52px;
    background: var(--bg-surface);
    border-right: 1px solid var(--border);
    padding: 0.75rem 0.4rem 0.75rem 0;
    text-align: right;
    color: var(--text-disabled);
    font-size: 0.8rem;
    line-height: 1.6;
    user-select: none;
    overflow: hidden;
  }

  .gutter-line {
    padding-right: 0.4rem;
  }

  .gutter-line.active-line {
    color: var(--accent);
    font-weight: 600;
    background: color-mix(in srgb, var(--accent) 8%, transparent);
  }

  .studio-canvas {
    flex: 1;
    display: flex;
    overflow: hidden;
    position: relative;
  }

  .studio-textarea {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    color: var(--text);
    font-family: 'Fira Code', 'JetBrains Mono', Consolas, monospace;
    line-height: 1.6;
    padding: 0.75rem 0.9rem;
    resize: none;
    tab-size: 2;
    caret-color: var(--accent);
    overflow: auto;
  }

  /* Status Bar Bawah Studio */
  .studio-statusbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.3rem 0.8rem;
    background: var(--bg-inset);
    border-top: 1px solid var(--border);
    font-size: 0.7rem;
    color: var(--text-subtle);
    user-select: none;
  }

  .sb-left,
  .sb-right {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }

  .sb-item {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
  }

  .sb-divider {
    color: var(--text-on-accent);
  }

  .sb-text {
    max-width: 320px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .cursor-pos {
    color: var(--text-muted);
    font-weight: 500;
  }

  .lang-tag {
    font-weight: 500;
    background: color-mix(in srgb, var(--text) 4%, transparent);
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
  }

  .save-status {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    font-weight: 500;
  }

  .save-status.saved {
    color: var(--accent);
  }

  .save-status.unsaved {
    color: var(--warning);
  }

  .save-status.saving {
    color: var(--info);
  }

  .dirty-indicator {
    font-size: 0.7rem;
    animation: pulse 1.5s infinite;
  }

  @keyframes slideDown {
    from {
      opacity: 0;
      transform: translateY(-8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
