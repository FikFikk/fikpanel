<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch, formatBytes } from '../lib/api';
  import { copyText } from '../lib/utils/browser';
  import { confirmDialog, promptDialog } from '../lib/stores/confirm.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import DirTabs from './files/DirTabs.svelte';
  import PathBar from './files/PathBar.svelte';
  import FileToolbar from './files/FileToolbar.svelte';
  import FileTable from './files/FileTable.svelte';
  import FileGrid from './files/FileGrid.svelte';
  import StatusBar from './files/StatusBar.svelte';
  import ContextMenu from './files/ContextMenu.svelte';
  import CodeEditor from './files/CodeEditor.svelte';
  import RemoteDownloadModal from './files/RemoteDownloadModal.svelte';
  import SymlinkModal from './files/SymlinkModal.svelte';
  import SearchContentModal from './files/SearchContentModal.svelte';
  import FavoritesModal from './files/FavoritesModal.svelte';
  import CloudStorageModal from './files/CloudStorageModal.svelte';
  import PropertiesModal from './files/PropertiesModal.svelte';
  import CompressModal from './files/CompressModal.svelte';
  import NewItemModal from './files/NewItemModal.svelte';
  import RenameModal from './files/RenameModal.svelte';
  import ChmodModal from './files/ChmodModal.svelte';
  import type { FileItem, FileListData } from '../lib/types';
  import '../styles/pages/files/shared.css';

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

  let tabs = $state<Tab[]>([{ id: "1", name: "root", path: "" }]);
  let activeTabId = $state<string>("1");

  // State Direktori & Berkas
  let currentPath = $state("");
  let parentPath = $state("");
  let items = $state<FileItem[]>([]);
  let totalDirs = $state(0);
  let totalFiles = $state(0);
  let totalSize = $state(0);
  let loading = $state(false);

  // State Tampilan & Pencarian
  let viewMode = $state<"list" | "grid">("list");
  let searchQuery = $state("");
  let selectedPaths = $state<Set<string>>(new Set());

  // State Clipboard Salin / Potong
  let clipboard = $state<{
    action: "copy" | "cut";
    paths: string[];
    names: string[];
  } | null>(null);

  // State Menu Popup Tools / Function List
  let activeMenuItem = $state<FileItem | null>(null);
  let menuPos = $state<{ top: number; left: number }>({ top: 0, left: 0 });

  // State Dropdown Menu Toolbar (Upload, New, Favorite, Operations)
  let activeToolbarMenu = $state<
    "upload" | "new" | "favorite" | "operations" | null
  >(null);

  // State Pagination
  let pageSize = $state(100);
  let currentPage = $state(1);

  // State Modal-Modal Aksi
  let showNewModal = $state(false);
  let newType = $state<"file" | "dir">("file");
  let newName = $state("");

  let showSymlinkModal = $state(false);
  let symlinkTarget = $state("");
  let symlinkName = $state("");
  let creatingSymlink = $state(false);

  let showRemoteModal = $state(false);
  let remoteUrl = $state("");
  let remoteFilename = $state("");
  let remoteDownloading = $state(false);

  let showSearchContentModal = $state(false);
  let contentSearchQuery = $state("");
  let contentSearchResults = $state<
    { file: string; line: number; text: string }[]
  >([]);
  let searchingContent = $state(false);

  let showFavManageModal = $state(false);
  let customFavorites = $state<{ name: string; path: string }[]>([
    { name: "Root Aplikasi", path: "" },
    { name: "Web Frontend (/web)", path: "web" },
    { name: "Backend Core (/internal)", path: "internal" },
    { name: "CLI Entry (/cmd)", path: "cmd" },
  ]);

  let showCloudStorageModal = $state(false);
  let selectedCloud = $state("");

  let showRenameModal = $state(false);
  let renameTarget = $state<FileItem | null>(null);
  let renameNewName = $state("");

  let showChmodModal = $state(false);
  let chmodTarget = $state<FileItem | null>(null);
  let chmodPerms = $state({
    ownerR: true,
    ownerW: true,
    ownerX: true,
    groupR: true,
    groupW: false,
    groupX: true,
    publicR: true,
    publicW: false,
    publicX: true,
  });

  // State Editor Berkas Modern (FikPanel Code Studio)
  let showEditorModal = $state(false);
  let editorTarget = $state<FileItem | null>(null);
  let editorContent = $state("");
  let editorInitialContent = $state("");
  let editorSaving = $state(false);
  let editorFontSize = $state<number>(13);
  let editorWordWrap = $state<boolean>(false);
  let editorIsFullscreen = $state<boolean>(false);
  let editorCursorPos = $state<{ line: number; col: number }>({
    line: 1,
    col: 1,
  });
  let editorShowSearch = $state<boolean>(false);
  let editorSearchQuery = $state<string>("");
  let editorReplaceQuery = $state<string>("");
  let editorSearchMatches = $state<number[]>([]);
  let editorSearchMatchIndex = $state<number>(0);
  let editorTextareaEl = $state<HTMLTextAreaElement | null>(null);
  let editorGutterEl = $state<HTMLDivElement | null>(null);

  // Derived state untuk kalkulasi nomor baris, status kotor, dan bahasa
  let editorIsDirty = $derived(editorContent !== editorInitialContent);
  let editorLineCount = $derived(
    editorContent ? editorContent.split("\n").length : 1,
  );
  let editorLines = $derived(
    Array.from({ length: editorLineCount }, (_, i) => i + 1),
  );
  let editorLangInfo = $derived(
    editorTarget
      ? detectFileLanguage(editorTarget.name)
      : {
          label: "Plain Text",
          icon: "ri-file-text-line",
          color: "var(--text-muted)",
        },
  );

  let showUploadModal = $state(false);
  let uploadFiles = $state<FileList | null>(null);
  let uploadSubmitting = $state(false);

  let showPropertiesModal = $state(false);
  let propTarget = $state<FileItem | null>(null);

  let showCompressModal = $state(false);
  let compressTarget = $state<FileItem | null>(null);
  let compressZipName = $state("");
  let compressing = $state(false);

  // Menghitung string mode numerik dari checkbox matrix PMSN (mis. 0755)
  let chmodOctal = $derived.by(() => {
    const o =
      (chmodPerms.ownerR ? 4 : 0) +
      (chmodPerms.ownerW ? 2 : 0) +
      (chmodPerms.ownerX ? 1 : 0);
    const g =
      (chmodPerms.groupR ? 4 : 0) +
      (chmodPerms.groupW ? 2 : 0) +
      (chmodPerms.groupX ? 1 : 0);
    const p =
      (chmodPerms.publicR ? 4 : 0) +
      (chmodPerms.publicW ? 2 : 0) +
      (chmodPerms.publicX ? 1 : 0);
    return `0${o}${g}${p}`;
  });

  // Breadcrumb part parser untuk Windows dan Linux
  let breadcrumbs = $derived.by(() => {
    if (!currentPath) return [];
    const isWindows = currentPath.includes("\\");
    const sep = isWindows ? "\\" : "/";
    const parts = currentPath.split(/[/\\]+/).filter(Boolean);

    let runningPath = "";
    return parts.map((part, index) => {
      if (index === 0 && isWindows && part.includes(":")) {
        runningPath = part + sep;
      } else if (index === 0 && !isWindows) {
        runningPath = "/" + part;
      } else {
        runningPath = runningPath
          ? runningPath + (runningPath.endsWith(sep) ? "" : sep) + part
          : part;
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
    return items.filter((it) => it.name.toLowerCase().includes(q));
  });

  // Item yang sedang aktif di halaman saat ini
  let paginatedItems = $derived.by(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredItems.slice(start, start + pageSize);
  });

  let totalPages = $derived(
    Math.max(1, Math.ceil(filteredItems.length / pageSize)),
  );

  // Memuat daftar direktori dari backend
  async function loadDirectory(targetPath: string = "") {
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
      const activeTab = tabs.find((t) => t.id === activeTabId);
      if (activeTab) {
        activeTab.path = currentPath;
        const base =
          currentPath
            .split(/[/\\]+/)
            .filter(Boolean)
            .pop() || "root";
        activeTab.name = base;
      }
    } catch (err: any) {
      onNotify(err.message || "Gagal memuat direktori");
    } finally {
      loading = false;
    }
  }

  // Tambah tab baru
  function addTab(customPath: string = "") {
    const path = customPath || currentPath;
    const newId = String(Date.now());
    const base =
      path
        .split(/[/\\]+/)
        .filter(Boolean)
        .pop() || "folder";
    tabs = [...tabs, { id: newId, name: base, path }];
    activeTabId = newId;
    loadDirectory(path);
  }

  // Tutup tab tertentu
  function closeTab(id: string, e: MouseEvent) {
    e.stopPropagation();
    if (tabs.length <= 1) return;
    const idx = tabs.findIndex((t) => t.id === id);
    tabs = tabs.filter((t) => t.id !== id);
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
      selectedPaths = new Set(paginatedItems.map((it) => it.path));
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

  function toggleToolbarMenu(
    name: "upload" | "new" | "favorite" | "operations",
    e: MouseEvent,
  ) {
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
      action: "copy",
      paths: [item.path],
      names: [item.name],
    };
    onNotify(`"${item.name}" disalin ke clipboard`);
  }

  // Set Clipboard Potong (Cut)
  function handleCutItem(item: FileItem) {
    closeAllMenus();
    clipboard = {
      action: "cut",
      paths: [item.path],
      names: [item.name],
    };
    onNotify(`"${item.name}" dipotong ke clipboard`);
  }

  // Tempel (Paste) Item dari Clipboard ke Direktori Saat Ini
  async function handlePasteClipboard() {
    if (!clipboard) return;
    try {
      const res = await apiFetch<{ processed: number; errors?: string[] }>(
        "/api/files/paste",
        {
          method: "POST",
          body: JSON.stringify({
            action: clipboard.action,
            source_paths: clipboard.paths,
            destination_dir: currentPath,
          }),
        },
      );

      if (res.errors && res.errors.length > 0) {
        onNotify(`Peringatan: ${res.errors.join(", ")}`);
      } else {
        onNotify(
          `Berhasil ${clipboard.action === "cut" ? "memindahkan" : "menyalin"} ${res.processed} item`,
        );
      }

      clipboard = null;
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || "Gagal memproses paste");
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
      await apiFetch("/api/files/compress", {
        method: "POST",
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
      onNotify(err.message || "Gagal mengompresi berkas");
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
  function openNewModal(type: "file" | "dir") {
    closeAllMenus();
    newType = type;
    newName = "";
    showNewModal = true;
  }

  async function handleCreateNew(e: Event) {
    e.preventDefault();
    if (!newName.trim()) return;
    try {
      await apiFetch("/api/files/new", {
        method: "POST",
        body: JSON.stringify({
          parent_path: currentPath,
          name: newName.trim(),
          type: newType,
        }),
      });
      showNewModal = false;
      onNotify(
        `${newType === "dir" ? "Folder" : "Berkas"} ${newName} berhasil dibuat`,
      );
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || "Gagal membuat item baru");
    }
  }

  // Pembuatan Soft Link (Symlink)
  async function handleCreateSymlink(e: Event) {
    e.preventDefault();
    if (!symlinkTarget.trim() || !symlinkName.trim()) return;

    creatingSymlink = true;
    try {
      await apiFetch("/api/files/symlink", {
        method: "POST",
        body: JSON.stringify({
          target_path: symlinkTarget.trim(),
          link_name: symlinkName.trim(),
          dest_dir: currentPath,
        }),
      });
      showSymlinkModal = false;
      symlinkTarget = "";
      symlinkName = "";
      onNotify(`Tautan simbolik ${symlinkName} berhasil dibuat`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || "Gagal membuat tautan simbolik");
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
      await apiFetch("/api/files/remote-download", {
        method: "POST",
        body: JSON.stringify({
          url: remoteUrl.trim(),
          dest_dir: currentPath,
          filename: remoteFilename.trim(),
        }),
      });
      showRemoteModal = false;
      remoteUrl = "";
      remoteFilename = "";
      onNotify("Berkas berhasil diunduh ke server");
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || "Gagal mengunduh berkas");
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
      const res = await apiFetch<{
        matches: { file: string; line: number; text: string }[];
      }>("/api/files/search-content", {
        method: "POST",
        body: JSON.stringify({
          dir: currentPath,
          query: contentSearchQuery.trim(),
        }),
      });
      contentSearchResults = res.matches || [];
    } catch (err: any) {
      onNotify(err.message || "Gagal mencari konten berkas");
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
    const base =
      currentPath
        .split(/[/\\]+/)
        .filter(Boolean)
        .pop() || "Folder";
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
      await apiFetch("/api/files/rename", {
        method: "POST",
        body: JSON.stringify({
          path: renameTarget.path,
          new_name: renameNewName.trim(),
        }),
      });
      showRenameModal = false;
      onNotify(`Berhasil mengubah nama menjadi ${renameNewName}`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || "Gagal mengubah nama");
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
        ownerR: (o & 4) !== 0,
        ownerW: (o & 2) !== 0,
        ownerX: (o & 1) !== 0,
        groupR: (g & 4) !== 0,
        groupW: (g & 2) !== 0,
        groupX: (g & 1) !== 0,
        publicR: (p & 4) !== 0,
        publicW: (p & 2) !== 0,
        publicX: (p & 1) !== 0,
      };
    }
    showChmodModal = true;
  }

  async function handleSaveChmod() {
    if (!chmodTarget) return;
    try {
      await apiFetch("/api/files/chmod", {
        method: "POST",
        body: JSON.stringify({
          path: chmodTarget.path,
          mode: chmodOctal,
        }),
      });
      showChmodModal = false;
      onNotify(`Izin ${chmodTarget.name} diubah ke ${chmodOctal}`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || "Gagal mengubah izin");
    }
  }

  // Deteksi bahasa dan format berkas berdasarkan nama/ekstensi
  function detectFileLanguage(filename: string): {
    label: string;
    icon: string;
    color: string;
  } {
    const lower = filename.toLowerCase();
    if (
      lower === ".htaccess" ||
      (lower.endsWith(".conf") && lower.includes("apache"))
    ) {
      return {
        label: "Apache Conf",
        icon: "ri-server-line",
        color: "var(--warning)",
      };
    }
    if (lower.endsWith(".conf") || lower.includes("nginx")) {
      return {
        label: "Nginx Conf",
        icon: "ri-server-line",
        color: "var(--accent)",
      };
    }
    if (lower.endsWith(".go"))
      return {
        label: "Go",
        icon: "ri-code-s-slash-line",
        color: "var(--info)",
      };
    if (lower.endsWith(".ts"))
      return {
        label: "TypeScript",
        icon: "ri-javascript-line",
        color: "var(--info)",
      };
    if (
      lower.endsWith(".js") ||
      lower.endsWith(".mjs") ||
      lower.endsWith(".cjs")
    )
      return {
        label: "JavaScript",
        icon: "ri-javascript-line",
        color: "var(--warning)",
      };
    if (lower.endsWith(".svelte"))
      return { label: "Svelte", icon: "ri-code-line", color: "var(--danger)" };
    if (lower.endsWith(".json"))
      return {
        label: "JSON",
        icon: "ri-braces-line",
        color: "var(--text-secondary)",
      };
    if (lower.endsWith(".html") || lower.endsWith(".htm"))
      return { label: "HTML", icon: "ri-html5-line", color: "var(--warning)" };
    if (lower.endsWith(".css"))
      return { label: "CSS", icon: "ri-css3-line", color: "var(--info)" };
    if (lower.endsWith(".scss") || lower.endsWith(".sass"))
      return { label: "SCSS", icon: "ri-css3-line", color: "var(--violet)" };
    if (lower.endsWith(".php"))
      return { label: "PHP", icon: "ri-code-line", color: "var(--violet)" };
    if (lower.endsWith(".py"))
      return {
        label: "Python",
        icon: "ri-terminal-box-line",
        color: "var(--info)",
      };
    if (lower.endsWith(".sh") || lower.endsWith(".bash"))
      return {
        label: "Shell Script",
        icon: "ri-terminal-line",
        color: "var(--accent-strong)",
      };
    if (lower.endsWith(".yml") || lower.endsWith(".yaml"))
      return {
        label: "YAML",
        icon: "ri-file-list-2-line",
        color: "var(--danger)",
      };
    if (lower.endsWith(".sql"))
      return { label: "SQL", icon: "ri-database-2-line", color: "var(--info)" };
    if (lower.endsWith(".md"))
      return {
        label: "Markdown",
        icon: "ri-markdown-line",
        color: "var(--info)",
      };
    if (lower.startsWith(".env"))
      return {
        label: "Environment",
        icon: "ri-key-2-line",
        color: "var(--warning)",
      };
    if (lower.endsWith(".xml"))
      return { label: "XML", icon: "ri-code-line", color: "var(--warning)" };
    return {
      label: "Plain Text",
      icon: "ri-file-text-line",
      color: "var(--text-muted)",
    };
  }

  // Buka editor modern untuk membaca dan menyunting berkas teks
  async function openEditor(item: FileItem) {
    closeAllMenus();
    editorTarget = item;
    editorContent = "Memuat isi berkas...";
    editorInitialContent = "";
    showEditorModal = true;
    editorShowSearch = false;
    editorSearchQuery = "";
    editorReplaceQuery = "";
    editorSearchMatches = [];
    editorSearchMatchIndex = 0;
    try {
      const q = encodeURIComponent(item.path);
      const res = await apiFetch<{ content: string }>(
        `/api/files/read?path=${q}`,
      );
      editorContent = res.content || "";
      editorInitialContent = editorContent;
      setTimeout(() => {
        if (editorTextareaEl) {
          editorTextareaEl.focus();
          editorTextareaEl.setSelectionRange(0, 0);
          updateEditorCursor();
        }
      }, 50);
    } catch (err: any) {
      onNotify(err.message || "Gagal membaca berkas");
      showEditorModal = false;
    }
  }

  // Simpan konten editor ke disk server
  async function handleSaveEditor() {
    if (!editorTarget) return;
    editorSaving = true;
    try {
      await apiFetch("/api/files/save", {
        method: "POST",
        body: JSON.stringify({
          path: editorTarget.path,
          content: editorContent,
        }),
      });
      editorInitialContent = editorContent;
      onNotify(`Berkas ${editorTarget.name} berhasil disimpan`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || "Gagal menyimpan berkas");
    } finally {
      editorSaving = false;
    }
  }

  // Muat ulang konten berkas dari server
  async function refreshEditorContent() {
    if (!editorTarget) return;
    if (
      editorIsDirty &&
      !(await confirmDialog({
        title: "Muat ulang berkas dari server?",
        message: "Perubahan yang belum Anda simpan di editor akan dibuang.",
        confirmText: "Muat Ulang",
        tone: "danger",
      }))
    ) {
      return;
    }
    try {
      const q = encodeURIComponent(editorTarget.path);
      const res = await apiFetch<{ content: string }>(
        `/api/files/read?path=${q}`,
      );
      editorContent = res.content || "";
      editorInitialContent = editorContent;
      onNotify("Isi berkas berhasil dimuat ulang");
    } catch (err: any) {
      onNotify(err.message || "Gagal memuat ulang berkas");
    }
  }

  // Tutup editor dengan konfirmasi bila ada perubahan yang belum disimpan
  async function closeEditorModal() {
    if (editorIsDirty) {
      if (
        !(await confirmDialog({
          title: "Tutup editor tanpa menyimpan?",
          message: "Perubahan pada berkas ini belum disimpan ke server.",
          confirmText: "Tutup Editor",
          tone: "danger",
        }))
      ) {
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
    const lines = textBefore.split("\n");
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
    if ((e.ctrlKey || e.metaKey) && e.key === "s") {
      e.preventDefault();
      handleSaveEditor();
      return;
    }
    if ((e.ctrlKey || e.metaKey) && e.key === "f") {
      e.preventDefault();
      editorShowSearch = !editorShowSearch;
      return;
    }
    if ((e.ctrlKey || e.metaKey) && e.key === "g") {
      e.preventDefault();
      handleEditorJumpLine();
      return;
    }
    if (e.key === "Tab") {
      e.preventDefault();
      if (!editorTextareaEl) return;
      const start = editorTextareaEl.selectionStart;
      const end = editorTextareaEl.selectionEnd;
      const spaces = "  "; // Indentasi 2 spasi standar
      editorContent =
        editorContent.substring(0, start) +
        spaces +
        editorContent.substring(end);
      setTimeout(() => {
        if (editorTextareaEl) {
          editorTextareaEl.selectionStart = editorTextareaEl.selectionEnd =
            start + spaces.length;
          updateEditorCursor();
        }
      }, 0);
    }
  }

  // Lompat ke baris tertentu sesuai input pengguna
  async function handleEditorJumpLine() {
    const targetStr = await promptDialog({
      title: "Lompat ke Baris",
      label: `Nomor baris (1 - ${editorLineCount}):`,
      value: String(editorCursorPos.line),
      inputType: "number",
      min: 1,
      max: editorLineCount,
      confirmText: "Lompat",
    });
    if (!targetStr) return;
    const targetLine = parseInt(targetStr, 10);
    if (isNaN(targetLine) || targetLine < 1 || targetLine > editorLineCount) {
      onNotify("Nomor baris tidak valid");
      return;
    }
    const lines = editorContent.split("\n");
    let charIndex = 0;
    for (let i = 0; i < targetLine - 1; i++) {
      charIndex += lines[i].length + 1;
    }
    if (editorTextareaEl) {
      editorTextareaEl.focus();
      editorTextareaEl.setSelectionRange(
        charIndex,
        charIndex + (lines[targetLine - 1]?.length || 0),
      );
      updateEditorCursor();
      const lineHeight = editorFontSize * 1.6;
      editorTextareaEl.scrollTop = Math.max(0, (targetLine - 5) * lineHeight);
    }
  }

  // Pencarian kata kunci di dalam textarea editor
  function searchInEditor(direction: "next" | "prev" = "next") {
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
      onNotify("Teks tidak ditemukan dalam berkas");
      return;
    }

    if (direction === "next") {
      editorSearchMatchIndex = (editorSearchMatchIndex + 1) % matches.length;
    } else {
      editorSearchMatchIndex =
        (editorSearchMatchIndex - 1 + matches.length) % matches.length;
    }

    const matchChar = matches[editorSearchMatchIndex];
    editorTextareaEl.focus();
    editorTextareaEl.setSelectionRange(
      matchChar,
      matchChar + editorSearchQuery.length,
    );
    updateEditorCursor();

    const linesBefore = editorContent
      .substring(0, matchChar)
      .split("\n").length;
    const lineHeight = editorFontSize * 1.6;
    editorTextareaEl.scrollTop = Math.max(0, (linesBefore - 5) * lineHeight);
  }

  // Mengganti teks yang sedang dipilih
  function replaceInEditor() {
    if (
      !editorSearchQuery ||
      !editorTextareaEl ||
      editorSearchMatches.length === 0
    )
      return;
    const matchChar = editorSearchMatches[editorSearchMatchIndex];
    editorContent =
      editorContent.substring(0, matchChar) +
      editorReplaceQuery +
      editorContent.substring(matchChar + editorSearchQuery.length);
    searchInEditor("next");
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
      onNotify("Teks tidak ditemukan untuk diganti");
    }
  }

  // Mengunduh Berkas
  function downloadFile(item: FileItem) {
    closeAllMenus();
    const q = encodeURIComponent(item.path);
    window.open(`/api/files/read?path=${q}&download=1`, "_blank");
  }

  // Hapus Satu Item
  async function deleteSingleItem(item: FileItem) {
    closeAllMenus();
    if (
      !(await confirmDialog({
        title: `Hapus ${item.is_dir ? "folder" : "berkas"} "${item.name}"?`,
        message:
          "Item ini akan dihapus secara permanen dari server dan tidak dapat dipulihkan.",
        confirmText: "Hapus Permanen",
        tone: "danger",
      }))
    )
      return;
    try {
      await apiFetch("/api/files/delete", {
        method: "POST",
        body: JSON.stringify({ paths: [item.path] }),
      });
      onNotify(`"${item.name}" berhasil dihapus`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || "Gagal menghapus item");
    }
  }

  // Hapus Massal Berkas yang Terpilih
  async function handleBatchDelete() {
    if (selectedPaths.size === 0) return;
    if (
      !(await confirmDialog({
        title: `Hapus ${selectedPaths.size} item yang dipilih?`,
        message:
          "Semua berkas dan folder yang terpilih akan dihapus permanen dari server.",
        confirmText: `Hapus ${selectedPaths.size} Item`,
        tone: "danger",
      }))
    )
      return;
    try {
      await apiFetch("/api/files/delete", {
        method: "POST",
        body: JSON.stringify({ paths: Array.from(selectedPaths) }),
      });
      onNotify(`${selectedPaths.size} item berhasil dihapus`);
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || "Gagal menghapus batch item");
    }
  }

  // Unggah Berkas
  async function handleUploadSubmit(e: Event) {
    e.preventDefault();
    if (!uploadFiles || uploadFiles.length === 0) return;

    uploadSubmitting = true;
    try {
      const formData = new FormData();
      formData.append("destination", currentPath);
      for (let i = 0; i < uploadFiles.length; i++) {
        formData.append("files", uploadFiles[i]);
      }

      await apiFetch("/api/files/upload", {
        method: "POST",
        body: formData,
        timeoutMs: 0,
      });

      showUploadModal = false;
      uploadFiles = null;
      onNotify("Berkas berhasil diunggah");
      await loadDirectory(currentPath);
    } catch (err: any) {
      onNotify(err.message || "Galat saat upload");
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
      return { icon: "ri-folder-fill", color: "var(--warning)" };
    }
    const ext = item.ext.toLowerCase();
    switch (ext) {
      case ".go":
      case ".ts":
      case ".js":
      case ".svelte":
      case ".py":
      case ".php":
      case ".html":
      case ".css":
      case ".json":
        return { icon: "ri-file-code-fill", color: "var(--accent)" };
      case ".png":
      case ".jpg":
      case ".jpeg":
      case ".svg":
      case ".webp":
      case ".gif":
        return { icon: "ri-image-fill", color: "var(--violet)" };
      case ".zip":
      case ".tar":
      case ".gz":
      case ".rar":
        return { icon: "ri-file-zip-fill", color: "var(--warning)" };
      case ".md":
      case ".txt":
      case ".log":
      case ".conf":
      case ".env":
        return { icon: "ri-file-text-fill", color: "var(--info)" };
      default:
        return { icon: "ri-file-3-fill", color: "var(--text-muted)" };
    }
  }

  onMount(() => {
    // Periksa apakah ada permintaan pembukaan direktori dari halaman lain (misal tombol aksi cepat situs)
    const openPath = sessionStorage.getItem("fikpanel_open_path");
    if (openPath) {
      sessionStorage.removeItem("fikpanel_open_path");
      loadDirectory(openPath);
    } else {
      loadDirectory();
    }
    window.addEventListener("click", closeAllMenus);
  });

  onDestroy(() => {
    window.removeEventListener("click", closeAllMenus);
  });
</script>

<div class="pg-files files-view">
  <!-- ═════════ 1. Tab Bar Direktori (Multi-Tab aaPanel) ═════════ -->
  <DirTabs
    {tabs}
    {activeTabId}
    onSwitchTab={switchTab}
    onCloseTab={closeTab}
    onAddTab={() => addTab()}
  />

  <!-- ═════════ 2. Breadcrumbs & Search Bar ═════════ -->
  <PathBar
    {currentPath}
    {parentPath}
    {breadcrumbs}
    bind:searchQuery
    onGoParent={handleGoParent}
    onLoadDirectory={loadDirectory}
  />

  <!-- ═════════ 3. Toolbar Aksi (Action Toolbar aaPanel Asli) ═════════ -->
  <div class="toolbar action-toolbar">
    <div class="tool-left">
      <!-- 1. Dropdown Upload > -->
      <div class="tool-dropdown-wrap">
        <button
          class="tool-btn {activeToolbarMenu === 'upload' ? 'is-active' : ''}"
          onclick={(e) => toggleToolbarMenu("upload", e)}
        >
          <i class="ri-upload-line"></i>
          <span>Upload &gt;</span>
        </button>

        {#if activeToolbarMenu === "upload"}
          <div
            class="toolbar-dropdown-menu"
            role="menu"
            tabindex="-1"
            onclick={(e) => e.stopPropagation()}
            onkeydown={(e) => e.key === "Escape" && closeAllMenus()}
          >
            <button
              class="tb-menu-item"
              onclick={() => {
                activeToolbarMenu = null;
                showUploadModal = true;
              }}
            >
              <div class="tb-item-left">
                <i class="ri-upload-2-line"></i>
                <span>Upload File</span>
              </div>
            </button>
            <button
              class="tb-menu-item"
              onclick={() => openCloudStorage("FTP Storage")}
            >
              <div class="tb-item-left">
                <i class="ri-server-line"></i>
                <span>FTP Storage</span>
              </div>
              <i class="ri-vip-diamond-fill badge-vip"></i>
            </button>
            <button
              class="tb-menu-item"
              onclick={() => openCloudStorage("S3 Storage")}
            >
              <div class="tb-item-left">
                <i class="ri-cloud-line"></i>
                <span>S3 Storage</span>
              </div>
              <i class="ri-vip-diamond-fill badge-vip"></i>
            </button>
            <button
              class="tb-menu-item"
              onclick={() => openCloudStorage("Google Drive")}
            >
              <div class="tb-item-left">
                <i class="ri-google-fill"></i>
                <span>Google Drive</span>
              </div>
              <i class="ri-vip-diamond-fill badge-vip"></i>
            </button>
            <button
              class="tb-menu-item"
              onclick={() => openCloudStorage("Google Cloud Storage")}
            >
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
          onclick={(e) => toggleToolbarMenu("new", e)}
        >
          <i class="ri-folder-add-fill icon-amber"></i>
          <span>New &gt;</span>
        </button>

        {#if activeToolbarMenu === "new"}
          <div
            class="toolbar-dropdown-menu"
            role="menu"
            tabindex="-1"
            onclick={(e) => e.stopPropagation()}
            onkeydown={(e) => e.key === "Escape" && closeAllMenus()}
          >
            <button class="tb-menu-item" onclick={() => openNewModal("dir")}>
              <div class="tb-item-left">
                <i class="ri-folder-fill icon-amber"></i>
                <span>New Directory</span>
              </div>
            </button>
            <button class="tb-menu-item" onclick={() => openNewModal("file")}>
              <div class="tb-item-left">
                <i class="ri-file-3-line"></i>
                <span>New Blank File</span>
              </div>
            </button>
            <button
              class="tb-menu-item"
              onclick={() => {
                activeToolbarMenu = null;
                showSymlinkModal = true;
              }}
            >
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
          onclick={(e) => toggleToolbarMenu("favorite", e)}
        >
          <i class="ri-star-fill icon-yellow"></i>
          <span>Favorite &gt;</span>
        </button>

        {#if activeToolbarMenu === "favorite"}
          <div
            class="toolbar-dropdown-menu"
            role="menu"
            tabindex="-1"
            onclick={(e) => e.stopPropagation()}
            onkeydown={(e) => e.key === "Escape" && closeAllMenus()}
          >
            {#each customFavorites as fav}
              <button
                class="tb-menu-item"
                onclick={() => {
                  activeToolbarMenu = null;
                  jumpFavorite(fav.path);
                }}
              >
                <div class="tb-item-left">
                  <i class="ri-folder-line icon-amber"></i>
                  <span>{fav.name}</span>
                </div>
              </button>
            {/each}
            <div class="operate-divider"></div>
            <button
              class="tb-menu-item"
              onclick={() => {
                activeToolbarMenu = null;
                showFavManageModal = true;
              }}
            >
              <div class="tb-item-left">
                <i class="ri-settings-3-line"></i>
                <span>Management</span>
              </div>
            </button>
          </div>
        {/if}
      </div>

      <!-- 6. Share List -->
      <button
        class="tool-btn"
        onclick={() =>
          onNotify(
            "Fitur Share List: tautan unduhan langsung terdaftar di menu Properties",
          )}
      >
        <i class="ri-share-line icon-green"></i>
        <span>Share List</span>
      </button>

      <!-- 7. File Operations > -->
      <div class="tool-dropdown-wrap">
        <button
          class="tool-btn {activeToolbarMenu === 'operations'
            ? 'is-active'
            : ''}"
          onclick={(e) => toggleToolbarMenu("operations", e)}
        >
          <i class="ri-folder-settings-line icon-amber"></i>
          <span>File Operations &gt;</span>
        </button>

        {#if activeToolbarMenu === "operations"}
          <div
            class="toolbar-dropdown-menu"
            role="menu"
            tabindex="-1"
            onclick={(e) => e.stopPropagation()}
            onkeydown={(e) => e.key === "Escape" && closeAllMenus()}
          >
            <button
              class="tb-menu-item"
              disabled={selectedPaths.size === 0}
              onclick={() => {
                if (selectedPaths.size > 0) {
                  clipboard = {
                    action: "copy",
                    paths: Array.from(selectedPaths),
                    names: Array.from(selectedPaths).map(
                      (p) => p.split(/[/\\]+/).pop() || "",
                    ),
                  };
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
                  clipboard = {
                    action: "cut",
                    paths: Array.from(selectedPaths),
                    names: Array.from(selectedPaths).map(
                      (p) => p.split(/[/\\]+/).pop() || "",
                    ),
                  };
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
              onclick={() => {
                activeToolbarMenu = null;
                handleBatchDelete();
              }}
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
          <button
            class="tool-btn tool-primary"
            onclick={handlePasteClipboard}
            title="Klik untuk menempel di direktori ini"
          >
            <i class="ri-clipboard-line"></i>
            <span>Paste ({clipboard.names.length} item)</span>
          </button>
          <button
            class="btn-cancel-clip"
            onclick={() => (clipboard = null)}
            title="Batalkan salin/potong">&times;</button
          >
        </div>
      {/if}
    </div>

    <div class="tool-right">
      <div class="view-toggle">
        <button
          class="toggle-btn {viewMode === 'list' ? 'active' : ''}"
          onclick={() => (viewMode = "list")}
          title="Tampilan Daftar"
        >
          <i class="ri-list-check"></i>
        </button>
        <button
          class="toggle-btn {viewMode === 'grid' ? 'active' : ''}"
          onclick={() => (viewMode = "grid")}
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
      <div class="state state--loading state-box">
        <i class="ri-loader-4-line spin-icon"></i>
        <p>Memuat direktori...</p>
      </div>
    {:else if filteredItems.length === 0}
      <div class="state state--empty state-box">
        <i class="ri-folder-open-line empty-icon"></i>
        <p>Direktori ini kosong atau berkas tidak ditemukan.</p>
        <div class="empty-actions">
          <button
            class="btn btn--secondary btn-outline"
            onclick={() => openNewModal("file")}>+ Buat Berkas</button
          >
          <button
            class="btn btn--secondary btn-outline"
            onclick={() => openNewModal("dir")}>+ Buat Folder</button
          >
          <button
            class="btn btn--primary btn-primary"
            onclick={() => (showUploadModal = true)}>Upload Berkas</button
          >
        </div>
      </div>
    {:else if viewMode === "list"}
      <!-- Tampilan Tabel List Gaya aaPanel -->
      <div class="table-wrap table-container">
        <table class="table file-table">
          <thead>
            <tr>
              <th class="col-check">
                <input
                  type="checkbox"
                  checked={paginatedItems.length > 0 &&
                    selectedPaths.size === paginatedItems.length}
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
                class="{selectedPaths.has(item.path)
                  ? 'selected-row'
                  : ''} {activeMenuItem?.path === item.path
                  ? 'highlight-row'
                  : ''}"
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
                    onkeydown={(e) =>
                      e.key === "Enter" && handleItemClick(item)}
                  >
                    <i
                      class="{iconMeta.icon} item-icon"
                      style="color: {iconMeta.color};"
                    ></i>
                    <span class="item-name {item.is_dir ? 'dir-name' : ''}"
                      >{item.name}</span
                    >
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
                <td class="col-ps text-muted"
                  >{item.is_dir ? "Directory" : item.ext || "File"}</td
                >
                <td class="col-ops">
                  <div class="row-operations">
                    <!-- Hover Quick Shortcuts (hanya terlihat saat baris di-hover) -->
                    <div class="hover-shortcuts">
                      {#if !item.is_dir}
                        <button
                          class="quick-icon-btn"
                          onclick={() => openEditor(item)}
                          title="Edit Cepat"
                        >
                          <i class="ri-edit-line"></i>
                        </button>
                        <button
                          class="quick-icon-btn"
                          onclick={() => downloadFile(item)}
                          title="Unduh"
                        >
                          <i class="ri-download-line"></i>
                        </button>
                      {:else}
                        <button
                          class="quick-icon-btn"
                          onclick={() => handleItemClick(item)}
                          title="Buka"
                        >
                          <i class="ri-folder-open-line"></i>
                        </button>
                      {/if}
                      <button
                        class="quick-icon-btn link-danger"
                        onclick={() => deleteSingleItem(item)}
                        title="Hapus"
                      >
                        <i class="ri-delete-bin-line"></i>
                      </button>
                    </div>

                    <!-- Tombol Dropdown Operate (Sesuai Screenshot aaPanel) -->
                    <button
                      class="btn-operate {activeMenuItem?.path === item.path
                        ? 'is-active'
                        : ''}"
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
            class="grid-card {selectedPaths.has(item.path)
              ? 'selected-card'
              : ''}"
            role="button"
            tabindex="0"
            onclick={() => handleItemClick(item)}
            onkeydown={(e) => e.key === "Enter" && handleItemClick(item)}
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
            <i
              class="{iconMeta.icon} card-icon"
              style="color: {iconMeta.color};"
            ></i>
            <span class="card-name" title={item.name}>{item.name}</span>
            <span class="card-size mono"
              >{item.is_dir ? "Directory" : formatBytes(item.size)}</span
            >
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- ═════════ 5. Footer & Status Bar ═════════ -->
  <StatusBar
    {totalDirs}
    {totalFiles}
    {totalSize}
    bind:currentPage
    bind:pageSize
    {totalPages}
    totalFilteredCount={filteredItems.length}
    {formatBytes}
  />

  <!-- ═════════ DROPDOWN OPERATE MENU (Popup Sesuai Screenshot aaPanel) ═════════ -->
  {#if activeMenuItem}
    <ContextMenu
      {activeMenuItem}
      {menuPos}
      onClose={closeAllMenus}
      onOpenEditor={openEditor}
      onDownloadFile={downloadFile}
      onExternalShare={handleExternalShare}
      onAddToFavorites={(it) => {
        customFavorites = [...customFavorites, { name: it.name, path: it.path }];
        onNotify(`"${it.name}" ditambahkan ke favorit`);
      }}
      onOpenCloudStorage={openCloudStorage}
      onOpenChmodModal={openChmodModal}
      onItemClick={handleItemClick}
      onAddTab={addTab}
      onCopyItem={handleCopyItem}
      onCopyPath={copyPathToClipboard}
      onCutItem={handleCutItem}
      onOpenRenameModal={openRenameModal}
      onDeleteSingleItem={deleteSingleItem}
      onOpenCompress={openCompress}
      onOpenProperties={openProperties}
    />
  {/if}

  <!-- ═════════ MODAL: Remote Download ═════════ -->
  <RemoteDownloadModal
    open={showRemoteModal}
    bind:remoteUrl
    bind:remoteFilename
    {currentPath}
    {remoteDownloading}
    onClose={() => (showRemoteModal = false)}
    onSubmit={handleRemoteDownload}
  />

  <!-- ═════════ MODAL: Soft Link (Symlink) ═════════ -->
  <SymlinkModal
    open={showSymlinkModal}
    bind:symlinkTarget
    bind:symlinkName
    {currentPath}
    {creatingSymlink}
    onClose={() => (showSymlinkModal = false)}
    onSubmit={handleCreateSymlink}
  />

  <!-- ═════════ MODAL: Search File Content ═════════ -->
  <SearchContentModal
    open={showSearchContentModal}
    bind:contentSearchQuery
    {contentSearchResults}
    {searchingContent}
    {currentPath}
    onClose={() => (showSearchContentModal = false)}
    onSubmit={handleSearchContent}
  />

  <!-- ═════════ MODAL: Manajemen Favorit ═════════ -->
  <FavoritesModal
    open={showFavManageModal}
    {customFavorites}
    onClose={() => (showFavManageModal = false)}
    onAddCurrent={addCurrentToFavorites}
    onRemoveFavorite={removeFavorite}
  />

  <!-- ═════════ MODAL: Cloud Storage Connector ═════════ -->
  <CloudStorageModal
    open={showCloudStorageModal}
    {selectedCloud}
    onClose={() => (showCloudStorageModal = false)}
    onConnect={() => {
      showCloudStorageModal = false;
      onNotify(`Konfigurasi ${selectedCloud} tersimpan`);
    }}
  />

  <!-- ═════════ MODAL: Properties (Detail Metadata Berkas) ═════════ -->
  <PropertiesModal
    open={showPropertiesModal && !!propTarget}
    {propTarget}
    {formatBytes}
    onClose={() => (showPropertiesModal = false)}
  />

  <!-- ═════════ MODAL: Kompresi Arsip Zip ═════════ -->
  <CompressModal
    open={showCompressModal && !!compressTarget}
    {compressTarget}
    bind:compressZipName
    {currentPath}
    {compressing}
    onClose={() => (showCompressModal = false)}
    onSubmit={handleExecuteCompress}
  />

  <!-- ═════════ MODAL: Buat File / Folder Baru ═════════ -->
  <NewItemModal
    open={showNewModal}
    bind:newType
    bind:newName
    {currentPath}
    onClose={() => (showNewModal = false)}
    onSubmit={handleCreateNew}
  />

  <!-- ═════════ MODAL: Ganti Nama (Rename) ═════════ -->
  <RenameModal
    open={showRenameModal && !!renameTarget}
    {renameTarget}
    bind:renameNewName
    onClose={() => (showRenameModal = false)}
    onSubmit={handleRename}
  />

  <!-- ═════════ MODAL: Hak Akses / PMSN (chmod) ═════════ -->
  <ChmodModal
    open={showChmodModal && !!chmodTarget}
    {chmodTarget}
    bind:chmodPerms
    {chmodOctal}
    onClose={() => (showChmodModal = false)}
    onSave={handleSaveChmod}
  />

  <!-- ═════════ MODAL: Editor Berkas Teks ═════════ -->
  <!-- ═════════ MODAL: Modern Code Studio Editor ═════════ -->
  {#if showEditorModal && editorTarget}
    <CodeEditor
      {editorTarget}
      bind:editorContent
      {editorSaving}
      bind:editorFontSize
      bind:editorWordWrap
      bind:editorIsFullscreen
      {editorCursorPos}
      bind:editorShowSearch
      bind:editorSearchQuery
      bind:editorReplaceQuery
      {editorSearchMatches}
      {editorSearchMatchIndex}
      bind:editorTextareaEl
      bind:editorGutterEl
      {editorIsDirty}
      {editorLineCount}
      {editorLines}
      {editorLangInfo}
      {formatBytes}
      onClose={closeEditorModal}
      onSave={handleSaveEditor}
      onRefresh={refreshEditorContent}
      onJumpLine={handleEditorJumpLine}
      onSearch={searchInEditor}
      onReplace={replaceInEditor}
      onReplaceAll={replaceAllInEditor}
      onScroll={handleEditorScroll}
      onKeydown={handleEditorKeydown}
      onUpdateCursor={updateEditorCursor}
    />
  {/if}

  <!-- ═════════ MODAL: Upload Berkas ═════════ -->
  <Modal
    open={showUploadModal}
    title="Upload Berkas ke Server"
    size="md"
    onclose={() => (showUploadModal = false)}
  >
    <form id="form-upload-files" onsubmit={handleUploadSubmit}>
      <div class="field">
        <label class="field__label" for="f-upload-dest">Direktori Tujuan</label>
        <input
          id="f-upload-dest"
          class="input"
          type="text"
          value={currentPath}
          disabled
        />
      </div>
      <div class="field">
        <label class="field__label" for="f-upload-input">Pilih Berkas</label>
        <input
          id="f-upload-input"
          class="input"
          type="file"
          multiple
          onchange={(e) => (uploadFiles = (e.target as HTMLInputElement).files)}
          required
        />
      </div>
    </form>
    {#snippet footer()}
      <button
        type="button"
        class="btn btn--ghost"
        onclick={() => (showUploadModal = false)}>Batal</button
      >
      <button
        type="submit"
        form="form-upload-files"
        class="btn btn--primary"
        disabled={uploadSubmitting}
      >
        {uploadSubmitting ? "Mengunggah..." : "Mulai Upload"}
      </button>
    {/snippet}
  </Modal>
</div>
