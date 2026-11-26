<script lang="ts">
  // Halaman Utama Docker Studio — Dedicated Container Workspace (Cloudflare Zero Trust / Docker Desktop Style)
  import '../styles/pages/docker/shared.css';
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import type {
    DockerStatus,
    DockerSystemInfo,
    DockerDaemonConfig,
    DockerContainer,
    DockerContainerInspect,
    DockerImage,
    DockerImageHistoryItem,
    DockerHubSearchItem,
    DockerVolume,
    DockerNetwork,
    DockerComposeProject,
    OneClickTemplate,
    DockerRegistry,
  } from '../lib/types';

  import OverviewTab from './docker/OverviewTab.svelte';
  import ContainerTab from './docker/ContainerTab.svelte';
  import TemplateTab from './docker/TemplateTab.svelte';
  import CloudImageTab from './docker/CloudImageTab.svelte';
  import LocalImageTab from './docker/LocalImageTab.svelte';
  import ComposeTab from './docker/ComposeTab.svelte';
  import NetworkTab from './docker/NetworkTab.svelte';
  import VolumeTab from './docker/VolumeTab.svelte';
  import RepositoryTab from './docker/RepositoryTab.svelte';
  import SettingsTab from './docker/SettingsTab.svelte';
  import CreateContainerModal from './docker/CreateContainerModal.svelte';
  import DeployTemplateModal from './docker/DeployTemplateModal.svelte';
  import PullImageModal from './docker/PullImageModal.svelte';
  import ComposeModal from './docker/ComposeModal.svelte';
  import CreateNetworkModal from './docker/CreateNetworkModal.svelte';
  import CreateVolumeModal from './docker/CreateVolumeModal.svelte';
  import AddRegistryModal from './docker/AddRegistryModal.svelte';
  import ContainerLogsModal from './docker/ContainerLogsModal.svelte';
  import ContainerShellModal from './docker/ContainerShellModal.svelte';
  import ContainerInspectModal from './docker/ContainerInspectModal.svelte';
  import ImageHistoryModal from './docker/ImageHistoryModal.svelte';

  interface Props {
    onNotify?: (msg: string, type?: 'success' | 'error' | 'info' | 'warning') => void;
    studioFullscreen?: boolean;
    currentUser?: string;
  }

  let { studioFullscreen = false }: Props = $props();

  type TabId =
    | 'overview'
    | 'container'
    | 'compose'
    | 'template'
    | 'local-image'
    | 'cloud-image'
    | 'volume'
    | 'network'
    | 'repository'
    | 'settings';

  const sectionMeta: Record<TabId, { title: string; subtitle: string; icon: string }> = {
    overview: {
      title: 'Overview & Disk Telemetry',
      subtitle: 'Ringkasan kesehatan engine Docker, pemakaian ruang disk aktual, dan spesifikasi host.',
      icon: 'ri-dashboard-3-line',
    },
    container: {
      title: 'Containers',
      subtitle: 'Kelola kontainer aktif & berhenti, buka terminal exec PTY, inspeksi env/mounts, dan streaming log live.',
      icon: 'ri-terminal-box-line',
    },
    compose: {
      title: 'Compose Stacks',
      subtitle: 'Auto-discovery seluruh stack Docker Compose di host dengan editor YAML dan kontrol siklus hidup penuh.',
      icon: 'ri-git-merge-line',
    },
    template: {
      title: 'App Store 1-Klik',
      subtitle: 'Pasang 15 layanan produksi siap pakai (WordPress, n8n, MinIO, Postgres, Redis, Grafana, dll.) dalam satu klik.',
      icon: 'ri-rocket-2-line',
    },
    'local-image': {
      title: 'Local Images',
      subtitle: 'Kelola citra tersimpan lokal, inspeksi layer build, retag, ekspor/impor .tar, dan bersihkan citra dangling.',
      icon: 'ri-stack-line',
    },
    'cloud-image': {
      title: 'Docker Hub & Dockerfile Builder',
      subtitle: 'Jelajahi Docker Hub API v2 dengan pemilihan tag & arsitektur (arm64/amd64) serta build citra langsung dari Dockerfile.',
      icon: 'ri-download-cloud-2-line',
    },
    volume: {
      title: 'Volumes',
      subtitle: 'Pantau ukuran aktual dan keterikatan penyimpanan data persisten kontainer.',
      icon: 'ri-hard-drive-2-line',
    },
    network: {
      title: 'Networks',
      subtitle: 'Kelola jaringan bridge/overlay virtual, subnet CIDR, gateway, dan kontainer terhubung.',
      icon: 'ri-global-line',
    },
    repository: {
      title: 'Private Registries',
      subtitle: 'Kelola autentikasi docker login persisten ke GHCR, GitLab Registry, Harbor, dan Docker Hub privat.',
      icon: 'ri-shield-keyhole-line',
    },
    settings: {
      title: 'Engine Settings',
      subtitle: 'Konfigurasi /etc/docker/daemon.json, registry mirror, log rotation, live restore, dan kontrol layanan systemd.',
      icon: 'ri-settings-3-line',
    },
  };

  let activeTab = $state<TabId>('overview');
  let mobileSidebarOpen = $state(false);
  let cloudInitialMode = $state<'hub' | 'build'>('hub');

  // State global status & sistem
  let status = $state<DockerStatus | null>(null);
  let systemInfo = $state<DockerSystemInfo | null>(null);
  let daemonConfig = $state<DockerDaemonConfig | null>(null);
  let loadingStatus = $state(true);

  // State data per modul
  let containers = $state<DockerContainer[]>([]);
  let loadingContainers = $state(false);
  let containerFilter = $state<'all' | 'running' | 'exited' | 'paused'>('all');
  let containerSearch = $state('');

  let templates = $state<OneClickTemplate[]>([]);
  let loadingTemplates = $state(false);
  let templateCategory = $state<string>('all');
  let templateSearch = $state('');

  let hubQuery = $state('');
  let hubResults = $state<DockerHubSearchItem[]>([]);
  let searchingHub = $state(false);

  let images = $state<DockerImage[]>([]);
  let loadingImages = $state(false);

  let composeProjects = $state<DockerComposeProject[]>([]);
  let loadingCompose = $state(false);

  let networks = $state<DockerNetwork[]>([]);
  let loadingNetworks = $state(false);

  let volumes = $state<DockerVolume[]>([]);
  let loadingVolumes = $state(false);

  let registries = $state<DockerRegistry[]>([]);
  let loadingRegistries = $state(false);

  // State Live Streaming Task Console (Pull / Build / Install)
  let streamTask = $state<{
    open: boolean;
    taskId: string;
    title: string;
    status: 'running' | 'success' | 'failed';
    logs: string;
  }>({
    open: false,
    taskId: '',
    title: '',
    status: 'running',
    logs: '',
  });
  let taskES: EventSource | null = null;
  let streamPreEl = $state<HTMLPreElement | null>(null);

  // State Modal & Form
  let showCreateContainerModal = $state(false);
  let showDeployTemplateModal = $state(false);
  let selectedTemplate = $state<OneClickTemplate | null>(null);
  let showPullImageModal = $state(false);
  let showComposeModal = $state(false);
  let showCreateNetworkModal = $state(false);
  let showCreateVolumeModal = $state(false);
  let showAddRegistryModal = $state(false);
  let showLogsModal = $state(false);
  let showConfigModal = $state(false);
  let showLogRotationModal = $state(false);

  // State Modal Baru: Shell PTY, Container Inspector, Image History
  let shellTarget = $state<{ open: boolean; id: string; name: string }>({
    open: false,
    id: '',
    name: '',
  });
  let inspectOpen = $state(false);
  let inspectLoading = $state(false);
  let inspectData = $state<DockerContainerInspect | null>(null);

  let historyOpen = $state(false);
  let historyLoading = $state(false);
  let historyImage = $state<DockerImage | null>(null);
  let historyItems = $state<DockerImageHistoryItem[]>([]);

  // State Operasi & Loading
  let installingDocker = $state(false);
  let actionLoading = $state(false);
  let pulling = $state(false);
  let savingConfig = $state(false);
  let logsLoading = $state(false);

  let newContainer = $state({
    name: '',
    image: 'nginx:alpine',
    network: 'default',
    memory_limit_mb: 0,
    cpu_limit: 0,
    ports: [{ host: '8080', container: '80', protocol: 'tcp' }],
    volumes: [{ host: '', container: '' }],
    envs: [{ key: '', value: '' }],
    restart_policy: 'unless-stopped',
    auto_remove: false,
    cmd: '',
  });

  let deployForm = $state({
    name: '',
    port: 8080,
    volume_path: '',
    env_values: {} as Record<string, string>,
  });

  let pullImageName = $state('');
  let pullImageTag = $state('latest');

  let composeForm = $state({
    name: '',
    yaml: `services:
  web:
    image: nginx:alpine
    restart: unless-stopped
    ports:
      - "8085:80"
`,
  });

  let newNetwork = $state({
    name: '',
    driver: 'bridge',
    subnet: '',
    gateway: '',
  });

  let newVolume = $state({
    name: '',
    driver: 'local',
  });

  let newRegistry = $state({
    name: '',
    url: '',
    username: '',
    password: '',
  });

  let selectedContainerLogs = $state<{
    id: string;
    name: string;
    content: string;
    lines: number;
    timestamps?: boolean;
    isCompose?: boolean;
    configPath?: string;
  }>({
    id: '',
    name: '',
    content: '',
    lines: 200,
    timestamps: false,
    isCompose: false,
    configPath: '',
  });

  let rawJsonConfig = $state('');

  // ── Pemetaan Nama Kontainer Aman ───────────────────────────────────────────
  function getContainerName(c: DockerContainer): string {
    return (c.name || c.names?.[0] || c.id.slice(0, 12)).replace(/^\//, '');
  }

  // ── Pemuatan Data ──────────────────────────────────────────────────────────
  async function loadCoreStatus() {
    loadingStatus = true;
    try {
      const [stRes, sysRes] = await Promise.all([
        apiFetch<DockerStatus>('/api/docker/status'),
        apiFetch<DockerSystemInfo>('/api/docker/system-info'),
      ]);
      status = stRes;
      systemInfo = sysRes;
    } catch (e: any) {
      notify(`Gagal memeriksa status Docker: ${e.message}`, 'error');
    } finally {
      loadingStatus = false;
    }
  }

  async function loadContainers() {
    loadingContainers = true;
    try {
      const res = await apiFetch<{ containers: DockerContainer[] }>('/api/docker/containers');
      containers = res.containers || [];
    } catch (e: any) {
      notify(`Gagal memuat kontainer: ${e.message}`, 'error');
    } finally {
      loadingContainers = false;
    }
  }

  async function loadTemplates() {
    loadingTemplates = true;
    try {
      const res = await apiFetch<{ templates: OneClickTemplate[] }>('/api/docker/templates');
      templates = res.templates || [];
    } catch (e: any) {
      notify(`Gagal memuat katalog template: ${e.message}`, 'error');
    } finally {
      loadingTemplates = false;
    }
  }

  async function loadImages() {
    loadingImages = true;
    try {
      const res = await apiFetch<{ images: DockerImage[] }>('/api/docker/images');
      images = res.images || [];
    } catch (e: any) {
      notify(`Gagal memuat citra lokal: ${e.message}`, 'error');
    } finally {
      loadingImages = false;
    }
  }

  async function loadComposeProjects() {
    loadingCompose = true;
    try {
      const res = await apiFetch<{ projects: DockerComposeProject[] }>('/api/docker/compose');
      composeProjects = res.projects || [];
    } catch (e: any) {
      notify(`Gagal memuat proyek Compose: ${e.message}`, 'error');
    } finally {
      loadingCompose = false;
    }
  }

  async function loadNetworks() {
    loadingNetworks = true;
    try {
      const res = await apiFetch<{ networks: DockerNetwork[] }>('/api/docker/networks');
      networks = res.networks || [];
    } catch (e: any) {
      notify(`Gagal memuat jaringan: ${e.message}`, 'error');
    } finally {
      loadingNetworks = false;
    }
  }

  async function loadVolumes() {
    loadingVolumes = true;
    try {
      const res = await apiFetch<{ volumes: DockerVolume[] }>('/api/docker/volumes');
      volumes = res.volumes || [];
    } catch (e: any) {
      notify(`Gagal memuat volume: ${e.message}`, 'error');
    } finally {
      loadingVolumes = false;
    }
  }

  async function loadRegistries() {
    loadingRegistries = true;
    try {
      const res = await apiFetch<{ registries: DockerRegistry[] }>('/api/docker/registries');
      registries = res.registries || [];
    } catch (e: any) {
      notify(`Gagal memuat registri: ${e.message}`, 'error');
    } finally {
      loadingRegistries = false;
    }
  }

  async function loadDaemonConfig() {
    try {
      const res = await apiFetch<DockerDaemonConfig>('/api/docker/config');
      daemonConfig = res;
      rawJsonConfig = res.raw_json || '{}';
    } catch (e: any) {
      notify(`Gagal membaca daemon.json: ${e.message}`, 'error');
    }
  }

  async function refreshCurrentTab() {
    await loadCoreStatus();
    switch (activeTab) {
      case 'container':
        await loadContainers();
        break;
      case 'template':
        await loadTemplates();
        break;
      case 'local-image':
        await loadImages();
        break;
      case 'compose':
        await loadComposeProjects();
        break;
      case 'network':
        await loadNetworks();
        break;
      case 'volume':
        await loadVolumes();
        break;
      case 'repository':
        await loadRegistries();
        break;
      case 'settings':
        await loadDaemonConfig();
        await loadRegistries();
        break;
    }
  }

  function switchTab(tab: TabId, hubMode?: 'hub' | 'build') {
    if (hubMode) cloudInitialMode = hubMode;
    activeTab = tab;
    mobileSidebarOpen = false;
    if (tab === 'container') loadContainers();
    else if (tab === 'template' && templates.length === 0) loadTemplates();
    else if (tab === 'local-image') loadImages();
    else if (tab === 'compose') loadComposeProjects();
    else if (tab === 'network') loadNetworks();
    else if (tab === 'volume') loadVolumes();
    else if (tab === 'repository') loadRegistries();
    else if (tab === 'settings') {
      loadDaemonConfig();
      loadRegistries();
    } else if (tab === 'cloud-image' && hubResults.length === 0 && !hubQuery) {
      hubQuery = 'nginx';
      searchDockerHub();
    }
  }

  function openFullscreenStudioTab() {
    const base = `${window.location.origin}${window.location.pathname}`;
    window.open(`${base}#/docker?studio=1`, '_blank', 'noopener');
  }

  function backToMainPanel() {
    window.location.hash = '#/home';
  }

  // ── Live Streaming SSE Task Console (Pull, Build, Install) ────────────────
  function closeStreamTaskES() {
    if (taskES) {
      taskES.close();
      taskES = null;
    }
  }

  function attachStreamTask(taskId: string, title: string, onDoneRefresh: () => void) {
    closeStreamTaskES();
    streamTask = {
      open: true,
      taskId,
      title,
      status: 'running',
      logs: '',
    };

    const es = new EventSource(`/api/docker/tasks/${encodeURIComponent(taskId)}/stream`);
    taskES = es;

    es.onmessage = (ev) => {
      try {
        const line = JSON.parse(ev.data) as string;
        streamTask = {
          ...streamTask,
          logs: streamTask.logs ? `${streamTask.logs}\n${line}` : line,
        };
        requestAnimationFrame(() => {
          if (streamPreEl) streamPreEl.scrollTop = streamPreEl.scrollHeight;
        });
      } catch {
        // Abaikan frame yang bukan JSON
      }
    };

    es.addEventListener('done', (ev) => {
      try {
        const meta = JSON.parse((ev as MessageEvent).data) as { status: string; error?: string };
        const ok = meta.status === 'success';
        streamTask = { ...streamTask, status: ok ? 'success' : 'failed' };
        if (ok) {
          notify(`${title} selesai!`, 'success');
          onDoneRefresh();
        } else {
          notify(`${title} gagal: ${meta.error || 'lihat log di konsol'}`, 'error');
        }
      } catch {
        streamTask = { ...streamTask, status: 'failed' };
      } finally {
        pulling = false;
        installingDocker = false;
        closeStreamTaskES();
      }
    });

    es.onerror = () => {
      if (streamTask.status === 'running') {
        streamTask = { ...streamTask, status: 'failed' };
        pulling = false;
        installingDocker = false;
      }
      closeStreamTaskES();
    };
  }

  // ── Instalasi Docker Engine Asli (Streaming SSE) ──────────────────────────
  async function handleInstallDocker() {
    const ok = await confirmDialog({
      title: 'Instal Docker Engine Resmi',
      message: 'FikPanel akan menjalankan installer resmi get.docker.com untuk memasang Docker Engine CE dan Docker Compose Plugin pada host Linux ini. Lanjutkan?',
      confirmLabel: 'Mulai Instalasi',
    });
    if (!ok) return;

    installingDocker = true;
    try {
      const res = await apiFetch<{ task_id: string }>('/api/docker/install', { method: 'POST' });
      if (res.task_id) {
        attachStreamTask(res.task_id, 'Instalasi Docker Engine CE', () => {
          loadCoreStatus();
        });
      }
    } catch (e: any) {
      installingDocker = false;
      notify(`Gagal memulai instalasi Docker: ${e.message}`, 'error');
    }
  }

  // ── System Prune Disk ─────────────────────────────────────────────────────
  async function handleSystemPrune(opts: {
    containers: boolean;
    images: boolean;
    all_images: boolean;
    volumes: boolean;
    networks: boolean;
    build_cache: boolean;
  }) {
    actionLoading = true;
    try {
      const res = await apiFetch<{ message?: string }>('/api/docker/system/prune', {
        method: 'POST',
        body: JSON.stringify(opts),
      });
      notify(res.message || 'Pembersihan disk Docker selesai', 'success');
      await Promise.all([loadCoreStatus(), loadImages(), loadContainers(), loadVolumes(), loadNetworks()]);
    } catch (e: any) {
      notify(`Pembersihan disk gagal: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  // ── Aksi Kontainer ────────────────────────────────────────────────────────
  async function controlContainer(c: DockerContainer, action: 'start' | 'stop' | 'restart' | 'pause' | 'unpause' | 'remove') {
    const cName = getContainerName(c);
    if (action === 'remove') {
      const ok = await confirmDialog({
        title: 'Hapus Kontainer',
        message: `Hapus permanen kontainer "${cName}"? Data di luar volume persisten akan hilang.`,
        confirmLabel: 'Hapus Kontainer',
        tone: 'danger',
      });
      if (!ok) return;
    }

    actionLoading = true;
    try {
      await apiFetch(`/api/docker/containers/${encodeURIComponent(c.id)}/${action}`, { method: 'POST' });
      notify(`Kontainer ${cName} berhasil di-${action}`, 'success');
      await Promise.all([loadContainers(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal ${action} ${cName}: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function handleBulkContainers(ids: string[], action: 'start' | 'stop' | 'restart' | 'remove') {
    if (action === 'remove') {
      const ok = await confirmDialog({
        title: `Hapus ${ids.length} Kontainer`,
        message: `Yakin ingin menghapus permanen ${ids.length} kontainer terpilih sekaligus?`,
        confirmLabel: 'Hapus Semua',
        tone: 'danger',
      });
      if (!ok) return;
    }

    actionLoading = true;
    try {
      const res = await apiFetch<{ result: { succeeded: string[]; failed: Record<string, string> } }>(
        '/api/docker/containers/bulk',
        {
          method: 'POST',
          body: JSON.stringify({ ids, action }),
        },
      );
      const okCnt = res.result?.succeeded?.length || 0;
      const failCnt = Object.keys(res.result?.failed || {}).length;
      if (failCnt > 0) {
        notify(`Bulk ${action}: ${okCnt} berhasil, ${failCnt} gagal`, 'warning');
      } else {
        notify(`Bulk ${action} berhasil pada ${okCnt} kontainer`, 'success');
      }
      await Promise.all([loadContainers(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal menjalankan bulk ${action}: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  function openContainerShell(c: DockerContainer) {
    shellTarget = {
      open: true,
      id: c.id,
      name: getContainerName(c),
    };
  }

  async function openContainerInspect(c: DockerContainer) {
    inspectOpen = true;
    inspectLoading = true;
    inspectData = null;
    try {
      inspectData = await apiFetch<DockerContainerInspect>(`/api/docker/containers/${encodeURIComponent(c.id)}/inspect`);
    } catch (e: any) {
      notify(`Gagal inspeksi kontainer: ${e.message}`, 'error');
      inspectOpen = false;
    } finally {
      inspectLoading = false;
    }
  }

  async function handleRecreateContainer(id: string, pullLatest: boolean) {
    const ok = await confirmDialog({
      title: pullLatest ? 'Pull & Update Kontainer' : 'Buat Ulang Kontainer',
      message: pullLatest
        ? 'Docker akan menarik citra terbaru terlebih dahulu lalu menghentikan dan membangun ulang kontainer dengan konfigurasi persis sama. Lanjutkan?'
        : 'Kontainer lama akan dihentikan dan dibuat ulang dengan konfigurasi yang identik. Lanjutkan?',
      confirmLabel: pullLatest ? 'Pull & Recreate' : 'Recreate',
    });
    if (!ok) return;

    actionLoading = true;
    try {
      await apiFetch(`/api/docker/containers/${encodeURIComponent(id)}/recreate`, {
        method: 'POST',
        body: JSON.stringify({ pull_latest: pullLatest }),
        timeoutMs: 0,
      });
      notify('Kontainer berhasil dibuat ulang!', 'success');
      inspectOpen = false;
      await Promise.all([loadContainers(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal recreate kontainer: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function openContainerLogs(c: DockerContainer) {
    selectedContainerLogs = {
      id: c.id,
      name: getContainerName(c),
      content: 'Memuat log kontainer...',
      lines: 200,
      timestamps: false,
      isCompose: false,
      configPath: '',
    };
    showLogsModal = true;
    await refreshContainerLogs();
  }

  async function openComposeLogs(proj: DockerComposeProject) {
    selectedContainerLogs = {
      id: proj.name,
      name: `Stack: ${proj.name}`,
      content: 'Memuat log stack Compose...',
      lines: 200,
      timestamps: false,
      isCompose: true,
      configPath: proj.config_path || '',
    };
    showLogsModal = true;
    await refreshContainerLogs();
  }

  async function refreshContainerLogs() {
    if (!selectedContainerLogs.id) return;
    logsLoading = true;
    try {
      if (selectedContainerLogs.isCompose) {
        const qs = new URLSearchParams({
          name: selectedContainerLogs.id,
          lines: String(selectedContainerLogs.lines),
        });
        if (selectedContainerLogs.configPath) qs.set('config_path', selectedContainerLogs.configPath);
        const res = await apiFetch<{ logs: string }>(`/api/docker/compose/logs?${qs.toString()}`);
        selectedContainerLogs = { ...selectedContainerLogs, content: res.logs || '(Log stack kosong)' };
      } else {
        const ts = selectedContainerLogs.timestamps ? '1' : '0';
        const res = await apiFetch<{ logs: string }>(
          `/api/docker/containers/${encodeURIComponent(selectedContainerLogs.id)}/logs?lines=${selectedContainerLogs.lines}&timestamps=${ts}`,
        );
        selectedContainerLogs = { ...selectedContainerLogs, content: res.logs || '(Belum ada output log)' };
      }
    } catch (e: any) {
      selectedContainerLogs = { ...selectedContainerLogs, content: `Gagal mengambil log: ${e.message}` };
    } finally {
      logsLoading = false;
    }
  }

  function addPortMapping() {
    newContainer.ports = [...newContainer.ports, { host: '', container: '', protocol: 'tcp' }];
  }
  function removePortMapping(idx: number) {
    newContainer.ports = newContainer.ports.filter((_, i) => i !== idx);
  }
  function addVolumeMapping() {
    newContainer.volumes = [...newContainer.volumes, { host: '', container: '' }];
  }
  function removeVolumeMapping(idx: number) {
    newContainer.volumes = newContainer.volumes.filter((_, i) => i !== idx);
  }
  function addEnvParam() {
    newContainer.envs = [...newContainer.envs, { key: '', value: '' }];
  }
  function removeEnvParam(idx: number) {
    newContainer.envs = newContainer.envs.filter((_, i) => i !== idx);
  }

  async function openCreateContainerModal(prefillImage?: string) {
    if (networks.length === 0) {
      loadNetworks();
    }
    if (prefillImage) {
      newContainer.image = prefillImage;
      const baseName = prefillImage.split('/').pop()?.split(':')[0] || 'container';
      newContainer.name = `${baseName}-${Math.floor(Math.random() * 900 + 100)}`;
    }
    showCreateContainerModal = true;
  }

  async function submitCreateContainer() {
    if (!newContainer.name.trim() || !newContainer.image.trim()) {
      notify('Nama kontainer dan citra wajib diisi', 'warning');
      return;
    }

    actionLoading = true;
    try {
      const payload = {
        name: newContainer.name.trim(),
        image: newContainer.image.trim(),
        network: newContainer.network,
        memory_limit_mb: Number(newContainer.memory_limit_mb) || 0,
        cpu_limit: Number(newContainer.cpu_limit) || 0,
        ports: newContainer.ports
          .filter((p) => p.host.trim() && p.container.trim())
          .map((p) => `${p.host.trim()}:${p.container.trim()}/${p.protocol}`),
        volumes: newContainer.volumes
          .filter((v) => v.host.trim() && v.container.trim())
          .map((v) => `${v.host.trim()}:${v.container.trim()}`),
        environment: newContainer.envs
          .filter((e) => e.key.trim())
          .map((e) => `${e.key.trim()}=${e.value}`),
        restart_policy: newContainer.restart_policy,
        auto_remove: newContainer.auto_remove,
        command: newContainer.cmd.trim(),
      };

      await apiFetch('/api/docker/containers', {
        method: 'POST',
        body: JSON.stringify(payload),
        timeoutMs: 0,
      });

      notify(`Kontainer ${newContainer.name} berhasil dibuat dan dijalankan!`, 'success');
      showCreateContainerModal = false;
      newContainer = {
        name: '',
        image: 'nginx:alpine',
        network: 'default',
        memory_limit_mb: 0,
        cpu_limit: 0,
        ports: [{ host: '8080', container: '80', protocol: 'tcp' }],
        volumes: [{ host: '', container: '' }],
        envs: [{ key: '', value: '' }],
        restart_policy: 'unless-stopped',
        auto_remove: false,
        cmd: '',
      };
      activeTab = 'container';
      await Promise.all([loadContainers(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal membuat kontainer: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  // ── Aksi One-Click Template ────────────────────────────────────────────────
  function openDeployTemplate(tmpl: OneClickTemplate) {
    selectedTemplate = tmpl;
    const defaultEnvs: Record<string, string> = {};
    for (const p of tmpl.env_params || []) {
      defaultEnvs[p.key] = p.default || '';
    }
    deployForm = {
      name: `${tmpl.id}-${Math.floor(Math.random() * 900 + 100)}`,
      port: tmpl.default_port || 8080,
      volume_path: `/www/dk_project/${tmpl.id}_data`,
      env_values: defaultEnvs,
    };
    showDeployTemplateModal = true;
  }

  async function submitDeployTemplate() {
    if (!selectedTemplate) return;
    if (!deployForm.name.trim()) {
      notify('Nama proyek kontainer wajib diisi', 'warning');
      return;
    }

    actionLoading = true;
    try {
      await apiFetch('/api/docker/templates/deploy', {
        method: 'POST',
        body: JSON.stringify({
          template_id: selectedTemplate.id,
          project_name: deployForm.name.trim(),
          name: deployForm.name.trim(),
          port: Number(deployForm.port),
          volume_path: deployForm.volume_path.trim(),
          env_values: deployForm.env_values,
        }),
        timeoutMs: 0,
      });
      notify(`Aplikasi ${selectedTemplate.name} berhasil dipasang dan dijalankan!`, 'success');
      showDeployTemplateModal = false;
      activeTab = 'container';
      await Promise.all([loadContainers(), loadComposeProjects(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal memasang ${selectedTemplate.name}: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  // ── Aksi Docker Hub, Streaming Pull & Dockerfile Build ────────────────────
  async function searchDockerHub() {
    if (!hubQuery.trim()) return;
    searchingHub = true;
    try {
      const res = await apiFetch<{ results: DockerHubSearchItem[] }>(
        `/api/docker/images/search?q=${encodeURIComponent(hubQuery.trim())}`,
      );
      hubResults = res.results || [];
    } catch (e: any) {
      notify(`Pencarian Docker Hub gagal: ${e.message}`, 'error');
    } finally {
      searchingHub = false;
    }
  }

  async function startStreamPull(fullImageRef: string) {
    const cleanRef = fullImageRef.trim();
    if (!cleanRef) {
      notify('Nama citra wajib diisi', 'warning');
      return;
    }
    pulling = true;
    showPullImageModal = false;
    try {
      const res = await apiFetch<{ task_id: string }>('/api/docker/images/pull/stream', {
        method: 'POST',
        body: JSON.stringify({ image: cleanRef }),
      });
      attachStreamTask(res.task_id, `Pull Citra ${cleanRef}`, () => {
        loadImages();
        loadCoreStatus();
      });
    } catch (e: any) {
      pulling = false;
      notify(`Gagal memulai pull ${cleanRef}: ${e.message}`, 'error');
    }
  }

  async function handleBuildDockerfile(payload: { tag: string; dockerfile: string; no_cache: boolean; pull: boolean }) {
    pulling = true;
    try {
      const res = await apiFetch<{ task_id: string }>('/api/docker/images/build', {
        method: 'POST',
        body: JSON.stringify(payload),
      });
      attachStreamTask(res.task_id, `Build Citra ${payload.tag}`, () => {
        loadImages();
        loadCoreStatus();
      });
    } catch (e: any) {
      pulling = false;
      notify(`Gagal memulai build Dockerfile: ${e.message}`, 'error');
    }
  }

  async function openImageHistory(img: DockerImage) {
    historyImage = img;
    historyOpen = true;
    historyLoading = true;
    historyItems = [];
    try {
      const ref = img.repository !== '<none>' && img.tag !== '<none>' ? `${img.repository}:${img.tag}` : img.id;
      const res = await apiFetch<{ history: DockerImageHistoryItem[] }>(
        `/api/docker/images/${encodeURIComponent(ref)}/history`,
      );
      historyItems = res.history || [];
    } catch (e: any) {
      notify(`Gagal memuat layer citra: ${e.message}`, 'error');
    } finally {
      historyLoading = false;
    }
  }

  async function handleRetagImage(source: string, target: string) {
    actionLoading = true;
    try {
      await apiFetch('/api/docker/images/tag', {
        method: 'POST',
        body: JSON.stringify({ source, target }),
      });
      notify(`Tag baru ${target} berhasil dibuat`, 'success');
      await loadImages();
    } catch (e: any) {
      notify(`Gagal membuat tag: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function handlePruneDanglingImages() {
    const ok = await confirmDialog({
      title: 'Prune Citra Dangling',
      message: 'Hapus seluruh citra yatim (<none>:<none>) yang tidak terpakai untuk membebaskan ruang disk?',
      confirmLabel: 'Prune Sekarang',
      tone: 'danger',
    });
    if (!ok) return;

    actionLoading = true;
    try {
      await apiFetch('/api/docker/images/prune', {
        method: 'POST',
        body: JSON.stringify({ all: false }),
      });
      notify('Citra dangling berhasil dibersihkan', 'success');
      await Promise.all([loadImages(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal membersihkan citra: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function handleImportImageTar(file: File) {
    actionLoading = true;
    notify(`Mengimpor arsip citra ${file.name}...`, 'info');
    try {
      const fd = new FormData();
      fd.append('file', file);
      await apiFetch('/api/docker/images/load', {
        method: 'POST',
        body: fd,
        timeoutMs: 0,
      });
      notify(`Arsip citra ${file.name} berhasil dimuat!`, 'success');
      await Promise.all([loadImages(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal mengimpor tar citra: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function deleteImage(img: DockerImage) {
    const label = `${img.repository}:${img.tag}`;
    const ok = await confirmDialog({
      title: 'Hapus Citra Lokal',
      message: `Hapus citra ${label} (${img.size_human}) dari penyimpanan lokal?`,
      confirmLabel: 'Hapus Citra',
      tone: 'danger',
    });
    if (!ok) return;

    actionLoading = true;
    try {
      await apiFetch(`/api/docker/images/${encodeURIComponent(img.id)}?force=true`, { method: 'DELETE' });
      notify(`Citra ${label} berhasil dihapus`, 'success');
      await Promise.all([loadImages(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal menghapus citra: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  // ── Aksi Compose ──────────────────────────────────────────────────────────
  async function submitComposeUp() {
    if (!composeForm.name.trim() || !composeForm.yaml.trim()) {
      notify('Nama proyek dan isi YAML wajib diisi', 'warning');
      return;
    }
    actionLoading = true;
    try {
      await apiFetch('/api/docker/compose/up', {
        method: 'POST',
        body: JSON.stringify({ name: composeForm.name.trim(), yaml_content: composeForm.yaml }),
        timeoutMs: 0,
      });
      notify(`Stack Compose ${composeForm.name} berhasil dijalankan!`, 'success');
      showComposeModal = false;
      await Promise.all([loadComposeProjects(), loadContainers(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal menjalankan stack Compose: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function handleComposeAction(
    proj: DockerComposeProject,
    action: 'up' | 'down' | 'restart' | 'stop' | 'start' | 'pull',
  ) {
    if (action === 'down') {
      const ok = await confirmDialog({
        title: 'Hentikan Stack Compose (Down)',
        message: `Hentikan dan hapus seluruh kontainer dalam stack "${proj.name}"?`,
        confirmLabel: 'Hentikan (Down)',
        tone: 'danger',
      });
      if (!ok) return;
    }

    actionLoading = true;
    try {
      await apiFetch('/api/docker/compose/action', {
        method: 'POST',
        body: JSON.stringify({
          name: proj.name,
          config_path: proj.config_path || '',
          action,
        }),
        timeoutMs: 0,
      });
      notify(`Stack ${proj.name} berhasil di-${action}`, 'success');
      await Promise.all([loadComposeProjects(), loadContainers(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Aksi ${action} stack ${proj.name} gagal: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  // ── Aksi Network, Volume, Registry & Daemon Settings ──────────────────────
  async function submitCreateNetwork() {
    if (!newNetwork.name.trim()) {
      notify('Nama jaringan wajib diisi', 'warning');
      return;
    }
    actionLoading = true;
    try {
      await apiFetch('/api/docker/networks', {
        method: 'POST',
        body: JSON.stringify(newNetwork),
      });
      notify(`Jaringan ${newNetwork.name} berhasil dibuat`, 'success');
      showCreateNetworkModal = false;
      newNetwork = { name: '', driver: 'bridge', subnet: '', gateway: '' };
      await Promise.all([loadNetworks(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal membuat jaringan: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function deleteNetwork(net: DockerNetwork) {
    const ok = await confirmDialog({
      title: 'Hapus Jaringan Docker',
      message: `Hapus jaringan "${net.name}"?`,
      confirmLabel: 'Hapus',
      tone: 'danger',
    });
    if (!ok) return;

    actionLoading = true;
    try {
      await apiFetch(`/api/docker/networks/${encodeURIComponent(net.id)}`, { method: 'DELETE' });
      notify(`Jaringan ${net.name} berhasil dihapus`, 'success');
      await Promise.all([loadNetworks(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal menghapus jaringan: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function pruneNetworks() {
    const ok = await confirmDialog({
      title: 'Bersihkan Jaringan Tak Terpakai',
      message: 'Hapus semua jaringan kustom Docker yang tidak terhubung ke kontainer aktif?',
      confirmLabel: 'Prune Jaringan',
      tone: 'danger',
    });
    if (!ok) return;

    actionLoading = true;
    try {
      await apiFetch('/api/docker/networks/prune', { method: 'POST' });
      notify('Jaringan tak terpakai berhasil dibersihkan', 'success');
      await Promise.all([loadNetworks(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal prune jaringan: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function submitCreateVolume() {
    if (!newVolume.name.trim()) {
      notify('Nama volume wajib diisi', 'warning');
      return;
    }
    actionLoading = true;
    try {
      await apiFetch('/api/docker/volumes', {
        method: 'POST',
        body: JSON.stringify(newVolume),
      });
      notify(`Volume ${newVolume.name} berhasil dibuat`, 'success');
      showCreateVolumeModal = false;
      newVolume = { name: '', driver: 'local' };
      await Promise.all([loadVolumes(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal membuat volume: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function deleteVolume(v: DockerVolume) {
    const ok = await confirmDialog({
      title: 'Hapus Volume Docker',
      message: `Hapus permanen volume "${v.name}"? Seluruh data di dalamnya akan hilang.`,
      confirmLabel: 'Hapus Volume',
      tone: 'danger',
    });
    if (!ok) return;

    actionLoading = true;
    try {
      await apiFetch(`/api/docker/volumes/${encodeURIComponent(v.name)}`, { method: 'DELETE' });
      notify(`Volume ${v.name} berhasil dihapus`, 'success');
      await Promise.all([loadVolumes(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal menghapus volume: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function pruneVolumes() {
    const ok = await confirmDialog({
      title: 'Bersihkan Volume Tidak Terpakai',
      message: 'Hapus seluruh volume Docker yang tidak terpasang pada kontainer mana pun?',
      confirmLabel: 'Prune Volume',
      tone: 'danger',
    });
    if (!ok) return;

    actionLoading = true;
    try {
      await apiFetch('/api/docker/volumes/prune', { method: 'POST' });
      notify('Volume usang berhasil dibersihkan', 'success');
      await Promise.all([loadVolumes(), loadCoreStatus()]);
    } catch (e: any) {
      notify(`Gagal prune volume: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function submitAddRegistry() {
    if (!newRegistry.name.trim() || !newRegistry.url.trim()) {
      notify('Nama dan URL registri wajib diisi', 'warning');
      return;
    }
    actionLoading = true;
    try {
      await apiFetch('/api/docker/registries', {
        method: 'POST',
        body: JSON.stringify(newRegistry),
      });
      notify(`Registri ${newRegistry.name} berhasil terhubung dan disimpan`, 'success');
      showAddRegistryModal = false;
      newRegistry = { name: '', url: '', username: '', password: '' };
      await loadRegistries();
    } catch (e: any) {
      notify(`Gagal menghubungkan registri: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function deleteRegistry(reg: DockerRegistry) {
    const ok = await confirmDialog({
      title: 'Logout & Hapus Registri',
      message: `Keluar dari sesi login dan hapus registri "${reg.name}"?`,
      confirmLabel: 'Hapus',
      tone: 'danger',
    });
    if (!ok) return;

    actionLoading = true;
    try {
      await apiFetch(`/api/docker/registries/${encodeURIComponent(reg.id)}`, { method: 'DELETE' });
      notify(`Registri ${reg.name} dihapus`, 'success');
      await loadRegistries();
    } catch (e: any) {
      notify(`Gagal menghapus registri: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function controlDockerService(action: 'start' | 'stop' | 'restart' | 'repair') {
    actionLoading = true;
    try {
      await apiFetch(`/api/docker/service/${action}`, { method: 'POST' });
      notify(`Layanan Docker Engine berhasil di-${action}`, 'success');
      await loadCoreStatus();
    } catch (e: any) {
      notify(`Gagal ${action} Docker Engine: ${e.message}`, 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function saveDaemonConfig() {
    if (!daemonConfig) return;
    savingConfig = true;
    try {
      // Kosongkan raw_json saat menyimpan lewat tombol pengaturan terstruktur agar backend memperbarui field JSON dengan benar
      const payload = { ...daemonConfig, raw_json: '' };
      await apiFetch('/api/docker/config', {
        method: 'POST',
        body: JSON.stringify(payload),
      });
      notify('Konfigurasi daemon.json berhasil disimpan! Restart Docker bila diperlukan.', 'success');
      showLogRotationModal = false;
      await loadDaemonConfig();
    } catch (e: any) {
      notify(`Gagal menyimpan konfigurasi: ${e.message}`, 'error');
    } finally {
      savingConfig = false;
    }
  }

  async function saveRawJsonConfig() {
    savingConfig = true;
    try {
      JSON.parse(rawJsonConfig);
      await apiFetch('/api/docker/config', {
        method: 'POST',
        body: JSON.stringify({ ...(daemonConfig || {}), raw_json: rawJsonConfig }),
      });
      notify('Berkas /etc/docker/daemon.json berhasil diperbarui!', 'success');
      showConfigModal = false;
      await loadDaemonConfig();
    } catch (e: any) {
      notify(`JSON tidak valid atau gagal menyimpan: ${e.message}`, 'error');
    } finally {
      savingConfig = false;
    }
  }

  async function handleUninstallDocker() {
    const ok = await confirmDialog({
      title: 'Hapus Instalasi Docker Engine',
      message: 'Tindakan ini menghentikan layanan Docker dan sangat berisiko pada server produksi. Untuk keamanan data volume, jalankan perintah uninstall paket langsung lewat Terminal SSH.',
      confirmLabel: 'Mengerti',
      tone: 'danger',
    });
    if (ok) {
      notify('Gunakan Terminal untuk menghapus paket docker-ce secara manual demi mencegah kehilangan data volume.', 'info');
    }
  }

  // ── Filter Reaktif ────────────────────────────────────────────────────────
  let filteredContainers = $derived(
    containers.filter((c) => {
      if (containerFilter === 'running' && c.state !== 'running') return false;
      if (containerFilter === 'exited' && (c.state === 'running' || c.state === 'paused')) return false;
      if (containerFilter === 'paused' && c.state !== 'paused') return false;
      if (containerSearch.trim()) {
        const q = containerSearch.toLowerCase();
        const name = getContainerName(c).toLowerCase();
        const matchName = name.includes(q);
        const matchImage = (c.image || '').toLowerCase().includes(q);
        const matchId = (c.id || '').toLowerCase().includes(q);
        const matchStack = (c.compose_project || '').toLowerCase().includes(q);
        return matchName || matchImage || matchId || matchStack;
      }
      return true;
    }),
  );

  let filteredTemplates = $derived(
    templates.filter((t) => {
      if (templateCategory !== 'all' && t.category !== templateCategory) return false;
      if (templateSearch.trim()) {
        const q = templateSearch.toLowerCase();
        return (
          t.name.toLowerCase().includes(q) ||
          t.description.toLowerCase().includes(q) ||
          t.image.toLowerCase().includes(q)
        );
      }
      return true;
    }),
  );

  function formatDate(ts: number): string {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString('id-ID', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  onMount(() => {
    loadCoreStatus();
    loadContainers();
    loadTemplates();
    return () => closeStreamTaskES();
  });

  let activeMeta = $derived(sectionMeta[activeTab]);
</script>

<div class="pg-docker {studioFullscreen ? 'pg-docker--fullscreen' : ''}">
  {#if mobileSidebarOpen}
    <div
      class="studio-scrim"
      role="presentation"
      onclick={() => (mobileSidebarOpen = false)}
    ></div>
  {/if}

  <div class="studio-shell">
    <!-- Sidebar Khusus Docker Studio (Gaya Dedicated Dashboard Cloudflare Zero Trust / Docker Desktop) -->
    <aside class="studio-sidebar {mobileSidebarOpen ? 'studio-sidebar--open' : ''}" aria-label="Navigasi Docker Studio">
      <div class="studio-brand">
        <div class="studio-brand-mark">
          <i class="ri-box-3-fill" aria-hidden="true"></i>
        </div>
        <div class="studio-brand-info">
          <span class="studio-brand-title">Docker Studio</span>
          <span class="studio-brand-sub">
            {#if status?.running}
              <span class="status-dot status-dot--active"></span>
              <span>v{status.version || '-'}</span>
              {#if status.compose_installed}
                <span>· Compose {status.compose_version}</span>
              {/if}
            {:else if status?.installed}
              <span class="status-dot status-dot--danger"></span>
              <span>Daemon Stopped</span>
            {:else}
              <span class="status-dot status-dot--warning"></span>
              <span>Not Installed</span>
            {/if}
          </span>
        </div>
      </div>

      <nav class="studio-nav">
        <div class="studio-nav-group">
          <span class="studio-nav-label">Ringkasan</span>
          <button
            type="button"
            class="studio-nav-item {activeTab === 'overview' ? 'studio-nav-item--active' : ''}"
            onclick={() => switchTab('overview')}
          >
            <i class="ri-dashboard-3-line studio-nav-icon" aria-hidden="true"></i>
            <span class="studio-nav-text">Overview</span>
          </button>
        </div>

        <div class="studio-nav-group">
          <span class="studio-nav-label">Workloads</span>
          <button
            type="button"
            class="studio-nav-item {activeTab === 'container' ? 'studio-nav-item--active' : ''}"
            onclick={() => switchTab('container')}
          >
            <i class="ri-terminal-box-line studio-nav-icon" aria-hidden="true"></i>
            <span class="studio-nav-text">Containers</span>
            <span class="studio-nav-count">{status?.containers_running ?? 0}/{status?.containers_total ?? 0}</span>
          </button>
          <button
            type="button"
            class="studio-nav-item {activeTab === 'compose' ? 'studio-nav-item--active' : ''}"
            onclick={() => switchTab('compose')}
          >
            <i class="ri-git-merge-line studio-nav-icon" aria-hidden="true"></i>
            <span class="studio-nav-text">Compose Stacks</span>
            <span class="studio-nav-count">{status?.compose_stacks_total ?? composeProjects.length}</span>
          </button>
          <button
            type="button"
            class="studio-nav-item {activeTab === 'template' ? 'studio-nav-item--active' : ''}"
            onclick={() => switchTab('template')}
          >
            <i class="ri-rocket-2-line studio-nav-icon" aria-hidden="true"></i>
            <span class="studio-nav-text">App Store 1-Klik</span>
            <span class="studio-nav-count">{templates.length || 15}</span>
          </button>
        </div>

        <div class="studio-nav-group">
          <span class="studio-nav-label">Images & Build</span>
          <button
            type="button"
            class="studio-nav-item {activeTab === 'local-image' ? 'studio-nav-item--active' : ''}"
            onclick={() => switchTab('local-image')}
          >
            <i class="ri-stack-line studio-nav-icon" aria-hidden="true"></i>
            <span class="studio-nav-text">Local Images</span>
            <span class="studio-nav-count">{status?.images_total ?? images.length}</span>
          </button>
          <button
            type="button"
            class="studio-nav-item {activeTab === 'cloud-image' ? 'studio-nav-item--active' : ''}"
            onclick={() => switchTab('cloud-image', 'hub')}
          >
            <i class="ri-download-cloud-2-line studio-nav-icon" aria-hidden="true"></i>
            <span class="studio-nav-text">Docker Hub & Build</span>
          </button>
        </div>

        <div class="studio-nav-group">
          <span class="studio-nav-label">Infrastruktur</span>
          <button
            type="button"
            class="studio-nav-item {activeTab === 'volume' ? 'studio-nav-item--active' : ''}"
            onclick={() => switchTab('volume')}
          >
            <i class="ri-hard-drive-2-line studio-nav-icon" aria-hidden="true"></i>
            <span class="studio-nav-text">Volumes</span>
            <span class="studio-nav-count">{status?.volumes_total ?? volumes.length}</span>
          </button>
          <button
            type="button"
            class="studio-nav-item {activeTab === 'network' ? 'studio-nav-item--active' : ''}"
            onclick={() => switchTab('network')}
          >
            <i class="ri-global-line studio-nav-icon" aria-hidden="true"></i>
            <span class="studio-nav-text">Networks</span>
            <span class="studio-nav-count">{status?.networks_total ?? networks.length}</span>
          </button>
          <button
            type="button"
            class="studio-nav-item {activeTab === 'repository' ? 'studio-nav-item--active' : ''}"
            onclick={() => switchTab('repository')}
          >
            <i class="ri-shield-keyhole-line studio-nav-icon" aria-hidden="true"></i>
            <span class="studio-nav-text">Registries</span>
            <span class="studio-nav-count">{registries.length || 1}</span>
          </button>
          <button
            type="button"
            class="studio-nav-item {activeTab === 'settings' ? 'studio-nav-item--active' : ''}"
            onclick={() => switchTab('settings')}
          >
            <i class="ri-settings-3-line studio-nav-icon" aria-hidden="true"></i>
            <span class="studio-nav-text">Engine Settings</span>
          </button>
        </div>
      </nav>

      <div class="studio-sidebar-foot">
        {#if studioFullscreen}
          <button type="button" class="btn btn--secondary btn--sm w-full" onclick={backToMainPanel}>
            <i class="ri-arrow-left-line" aria-hidden="true"></i>
            <span>Kembali ke FikPanel</span>
          </button>
        {:else}
          <button type="button" class="btn btn--secondary btn--sm w-full" onclick={openFullscreenStudioTab}>
            <i class="ri-external-link-line" aria-hidden="true"></i>
            <span>Buka di Tab Penuh</span>
          </button>
        {/if}
      </div>
    </aside>

    <!-- Area Konten Utama Docker Studio -->
    <div class="studio-main">
      <!-- Banner Instalasi bila Docker belum terpasang -->
      {#if !loadingStatus && status && !status.installed}
        <div class="floating-banner" role="alert">
          <div class="banner-content">
            <i class="ri-error-warning-fill banner-icon" aria-hidden="true"></i>
            <span>Docker Engine belum terpasang pada server ini. Instal Docker CE & Compose Plugin resmi sekarang?</span>
          </div>
          <button
            type="button"
            class="btn btn--primary btn--sm"
            disabled={installingDocker}
            onclick={handleInstallDocker}
          >
            <i class="ri-download-cloud-2-line {installingDocker ? 'spin' : ''}" aria-hidden="true"></i>
            <span>{installingDocker ? 'Memasang Docker...' : 'Instal Docker CE Resmi'}</span>
          </button>
        </div>
      {/if}

      <!-- Header Bagian Aktif -->
      <header class="studio-header">
        <div class="studio-header-left">
          <button
            type="button"
            class="btn btn--secondary btn--sm btn--icon studio-mobile-toggle"
            aria-label="Buka menu Docker Studio"
            onclick={() => (mobileSidebarOpen = !mobileSidebarOpen)}
          >
            <i class="ri-menu-2-line" aria-hidden="true"></i>
          </button>
          <div>
            <h1 class="studio-title">
              <i class="{activeMeta.icon} text-accent" aria-hidden="true"></i>
              <span>{activeMeta.title}</span>
            </h1>
            <p class="studio-subtitle">{activeMeta.subtitle}</p>
          </div>
        </div>

        <div class="studio-header-actions">
          <button type="button" class="btn btn--primary btn--sm" onclick={() => openCreateContainerModal()}>
            <i class="ri-add-line" aria-hidden="true"></i>
            <span>Kontainer</span>
          </button>
          <button type="button" class="btn btn--secondary btn--sm" onclick={() => (showPullImageModal = true)}>
            <i class="ri-download-line" aria-hidden="true"></i>
            <span>Pull Image</span>
          </button>
          {#if !studioFullscreen}
            <button
              type="button"
              class="btn btn--secondary btn--sm btn--icon"
              title="Buka Docker Studio di tab penuh"
              aria-label="Buka Docker Studio di tab penuh"
              onclick={openFullscreenStudioTab}
            >
              <i class="ri-external-link-line" aria-hidden="true"></i>
            </button>
          {/if}
          <button
            type="button"
            class="btn btn--secondary btn--sm btn--icon"
            title="Segarkan data"
            aria-label="Segarkan data Docker"
            disabled={loadingStatus}
            onclick={refreshCurrentTab}
          >
            <i class="ri-refresh-line {loadingStatus ? 'spin' : ''}" aria-hidden="true"></i>
          </button>
        </div>
      </header>

      <!-- Konsol Progres Live Streaming SSE (muncul saat Pull Image / Build Dockerfile / Install Docker) -->
      {#if streamTask.open}
        <section class="stream-console-card" aria-label="Konsol Progres Live Docker">
          <div class="stream-console-head">
            <div class="stream-console-title">
              {#if streamTask.status === 'running'}
                <i class="ri-loader-4-line spin text-accent" aria-hidden="true"></i>
              {:else if streamTask.status === 'success'}
                <i class="ri-checkbox-circle-fill text-success" aria-hidden="true"></i>
              {:else}
                <i class="ri-close-circle-fill text-danger" aria-hidden="true"></i>
              {/if}
              <span>{streamTask.title}</span>
              <span class="badge {streamTask.status === 'running' ? 'badge--warning' : streamTask.status === 'success' ? 'badge--success' : 'badge--danger'} text-2xs">
                {streamTask.status === 'running' ? 'STREAMING' : streamTask.status.toUpperCase()}
              </span>
            </div>
            <button
              type="button"
              class="btn btn--ghost btn--sm btn--icon"
              title="Tutup konsol progres"
              aria-label="Tutup konsol progres"
              onclick={() => (streamTask = { ...streamTask, open: false })}
            >
              <i class="ri-close-line" aria-hidden="true"></i>
            </button>
          </div>
          <pre bind:this={streamPreEl} class="stream-console-body">{streamTask.logs || 'Menunggu output dari Docker daemon...'}</pre>
        </section>
      {/if}

      <!-- Tampilan Submodul Aktif -->
      {#if activeTab === 'overview'}
        <OverviewTab
          {status}
          {systemInfo}
          {actionLoading}
          onOpenCreateContainer={() => openCreateContainerModal()}
          onOpenPullImage={() => (showPullImageModal = true)}
          onGoHubBuilder={() => switchTab('cloud-image', 'build')}
          onGoTemplate={() => switchTab('template')}
          onOpenComposeModal={() => (showComposeModal = true)}
          onSystemPrune={handleSystemPrune}
        />
      {:else if activeTab === 'container'}
        <ContainerTab
          {containers}
          {filteredContainers}
          {loadingContainers}
          {actionLoading}
          bind:containerFilter
          bind:containerSearch
          onOpenCreateContainer={() => openCreateContainerModal()}
          onLoadContainers={loadContainers}
          onControlContainer={controlContainer}
          onBulkContainers={handleBulkContainers}
          onOpenContainerLogs={openContainerLogs}
          onOpenContainerShell={openContainerShell}
          onOpenContainerInspect={openContainerInspect}
          {formatDate}
        />
      {:else if activeTab === 'compose'}
        <ComposeTab
          {composeProjects}
          {loadingCompose}
          {actionLoading}
          onOpenComposeModal={() => (showComposeModal = true)}
          onLoadComposeProjects={loadComposeProjects}
          onEditComposeProject={(proj) => {
            composeForm = { name: proj.name, yaml: proj.yaml_content || '' };
            showComposeModal = true;
          }}
          onComposeAction={handleComposeAction}
          onOpenComposeLogs={openComposeLogs}
        />
      {:else if activeTab === 'template'}
        <TemplateTab
          {filteredTemplates}
          {loadingTemplates}
          bind:templateCategory
          bind:templateSearch
          onOpenDeployTemplate={openDeployTemplate}
        />
      {:else if activeTab === 'local-image'}
        <LocalImageTab
          {images}
          {loadingImages}
          {actionLoading}
          onOpenPullImage={() => (showPullImageModal = true)}
          onLoadImages={loadImages}
          onRunImageAsContainer={(img) => openCreateContainerModal(`${img.repository}:${img.tag}`)}
          onInspectImageHistory={openImageHistory}
          onPruneDanglingImages={handlePruneDanglingImages}
          onImportImageTar={handleImportImageTar}
          onDeleteImage={deleteImage}
          {formatDate}
        />
      {:else if activeTab === 'cloud-image'}
        <CloudImageTab
          bind:hubQuery
          {hubResults}
          {searchingHub}
          {pulling}
          initialMode={cloudInitialMode}
          onSearchDockerHub={searchDockerHub}
          onStreamPullImage={startStreamPull}
          onBuildDockerfile={handleBuildDockerfile}
        />
      {:else if activeTab === 'volume'}
        <VolumeTab
          {volumes}
          {loadingVolumes}
          {actionLoading}
          onOpenCreateVolume={() => (showCreateVolumeModal = true)}
          onPruneVolumes={pruneVolumes}
          onLoadVolumes={loadVolumes}
          onDeleteVolume={deleteVolume}
        />
      {:else if activeTab === 'network'}
        <NetworkTab
          {networks}
          {loadingNetworks}
          {actionLoading}
          onOpenCreateNetwork={() => (showCreateNetworkModal = true)}
          onPruneNetworks={pruneNetworks}
          onLoadNetworks={loadNetworks}
          onDeleteNetwork={deleteNetwork}
        />
      {:else if activeTab === 'repository'}
        <RepositoryTab
          {registries}
          {loadingRegistries}
          {actionLoading}
          onOpenAddRegistry={() => (showAddRegistryModal = true)}
          onLoadRegistries={loadRegistries}
          onDeleteRegistry={deleteRegistry}
        />
      {:else if activeTab === 'settings'}
        <SettingsTab
          {status}
          {systemInfo}
          bind:daemonConfig
          {registries}
          {actionLoading}
          {savingConfig}
          bind:showConfigModal
          bind:showLogRotationModal
          bind:rawJsonConfig
          onControlDockerService={controlDockerService}
          onSaveDaemonConfig={saveDaemonConfig}
          onSaveRawJsonConfig={saveRawJsonConfig}
          onOpenLogRotationModal={() => (showLogRotationModal = true)}
          onOpenConfigModal={() => {
            rawJsonConfig = daemonConfig?.raw_json || '{}';
            showConfigModal = true;
          }}
          onUninstallDocker={handleUninstallDocker}
        />
      {/if}
    </div>
  </div>

  <!-- Kumpulan Modal Docker Studio -->
  <CreateContainerModal
    bind:showCreateContainerModal
    bind:newContainer
    {networks}
    {actionLoading}
    onSubmitCreateContainer={submitCreateContainer}
    onAddPortMapping={addPortMapping}
    onRemovePortMapping={removePortMapping}
    onAddVolumeMapping={addVolumeMapping}
    onRemoveVolumeMapping={removeVolumeMapping}
    onAddEnvParam={addEnvParam}
    onRemoveEnvParam={removeEnvParam}
  />

  <DeployTemplateModal
    bind:showDeployTemplateModal
    {selectedTemplate}
    bind:deployForm
    {actionLoading}
    onSubmitDeployTemplate={submitDeployTemplate}
  />

  <PullImageModal
    bind:showPullImageModal
    bind:pullImageName
    bind:pullImageTag
    {pulling}
    onSubmitPullImage={() => {
      const name = pullImageName.trim();
      const tag = pullImageTag.trim() || 'latest';
      if (!name) return;
      const full = name.includes(':') ? name : `${name}:${tag}`;
      startStreamPull(full);
    }}
  />

  <ComposeModal
    bind:showComposeModal
    bind:composeForm
    {actionLoading}
    onSubmitComposeUp={submitComposeUp}
  />

  <CreateNetworkModal
    bind:showCreateNetworkModal
    bind:newNetwork
    {actionLoading}
    onSubmitCreateNetwork={submitCreateNetwork}
  />

  <CreateVolumeModal
    bind:showCreateVolumeModal
    bind:newVolume
    {actionLoading}
    onSubmitCreateVolume={submitCreateVolume}
  />

  <AddRegistryModal
    bind:showAddRegistryModal
    bind:newRegistry
    {actionLoading}
    onSubmitAddRegistry={submitAddRegistry}
  />

  <ContainerLogsModal
    bind:showLogsModal
    bind:selectedContainerLogs
    {logsLoading}
    onRefreshContainerLogs={refreshContainerLogs}
  />

  <ContainerShellModal
    open={shellTarget.open}
    containerId={shellTarget.id}
    containerName={shellTarget.name}
    onClose={() => (shellTarget = { ...shellTarget, open: false })}
  />

  <ContainerInspectModal
    open={inspectOpen}
    loading={inspectLoading}
    {actionLoading}
    {inspectData}
    onClose={() => (inspectOpen = false)}
    onRecreate={handleRecreateContainer}
  />

  <ImageHistoryModal
    open={historyOpen}
    image={historyImage}
    history={historyItems}
    loading={historyLoading}
    {actionLoading}
    onClose={() => (historyOpen = false)}
    onRetag={handleRetagImage}
  />
</div>
