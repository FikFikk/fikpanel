<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch } from '../lib/api';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import { notify } from '../lib/stores/toast.svelte';
  import type {
    DockerStatus,
    DockerSystemInfo,
    DockerDaemonConfig,
    DockerContainer,
    DockerImage,
    DockerVolume,
    DockerNetwork,
    DockerComposeProject,
    OneClickTemplate,
    DockerRegistry,
    SystemTask,
  } from '../lib/types';
  import Modal from '../lib/components/ui/Modal.svelte';
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
  import PullImageModal from './docker/PullImageModal.svelte';
  import ContainerLogsModal from './docker/ContainerLogsModal.svelte';
  import '../styles/pages/docker/shared.css';

  // Daftar Tab Navigasi Utama (10 Tab Lengkap Sesuai aaPanel Suite)
  type DockerTab =
    | 'overview'
    | 'container'
    | 'template'
    | 'cloud_image'
    | 'local_image'
    | 'compose'
    | 'network'
    | 'volume'
    | 'repository'
    | 'settings';

  let activeTab = $state<DockerTab>('overview');

  // State Utama Engine & Sistem
  let status = $state<DockerStatus | null>(null);
  let systemInfo = $state<DockerSystemInfo | null>(null);
  let daemonConfig = $state<DockerDaemonConfig | null>(null);
  let loadingStatus = $state(true);
  let actionLoading = $state(false);

  // State Data Tiap Tab
  let containers = $state<DockerContainer[]>([]);
  let loadingContainers = $state(false);
  let containerFilter = $state<'all' | 'running' | 'stopped' | 'paused'>('all');
  let containerSearch = $state('');

  let images = $state<DockerImage[]>([]);
  let loadingImages = $state(false);
  let imageSearch = $state('');

  let cloudSearchQuery = $state('');
  let cloudSearching = $state(false);
  let cloudResults = $state<
    Array<{
      name: string;
      description: string;
      stars: number;
      is_official: boolean;
      is_automated: boolean;
    }>
  >([]);

  let volumes = $state<DockerVolume[]>([]);
  let loadingVolumes = $state(false);

  let networks = $state<DockerNetwork[]>([]);
  let loadingNetworks = $state(false);

  let composeProjects = $state<DockerComposeProject[]>([]);
  let loadingCompose = $state(false);

  let templates = $state<OneClickTemplate[]>([]);
  let loadingTemplates = $state(false);
  let templateCategory = $state<string>('all');

  let registries = $state<DockerRegistry[]>([]);
  let loadingRegistries = $state(false);

  // Modal State
  let showCreateContainerModal = $state(false);
  let showPullImageModal = $state(false);
  let showLogsModal = $state(false);
  let showComposeModal = $state(false);
  let showCreateNetworkModal = $state(false);
  let showCreateVolumeModal = $state(false);
  let showAddRegistryModal = $state(false);
  let showDeployTemplateModal = $state(false);
  let showConfigModal = $state(false);
  let showLogRotationModal = $state(false);

  // Data Seleksi / Form Modal
  let selectedContainerLogs = $state<{ id: string; name: string; content: string; lines: number }>({
    id: '',
    name: '',
    content: '',
    lines: 200,
  });
  let logsLoading = $state(false);

  // Form Kontainer Baru
  let newContainer = $state({
    name: '',
    image: '',
    ports: [{ host: '', container: '', protocol: 'tcp' }],
    volumes: [{ host: '', container: '' }],
    envs: [{ key: '', value: '' }],
    restart_policy: 'unless-stopped',
    auto_remove: false,
    cmd: '',
  });

  // Form Tarik Citra
  let pullImageName = $state('');
  let pullImageTag = $state('latest');
  let pulling = $state(false);

  // Form Compose
  let composeForm = $state({
    name: '',
    yaml: `version: '3.8'\nservices:\n  app:\n    image: nginx:alpine\n    ports:\n      - "8080:80"\n    restart: unless-stopped\n`,
  });

  // Form Network
  let newNetwork = $state({
    name: '',
    driver: 'bridge',
    subnet: '172.28.0.0/16',
    gateway: '172.28.0.1',
  });

  // Form Volume
  let newVolume = $state({
    name: '',
    driver: 'local',
  });

  // Form Registri
  let newRegistry = $state({
    name: '',
    url: '',
    username: '',
    password: '',
  });

  // Form Deploy Template
  let selectedTemplate = $state<OneClickTemplate | null>(null);
  let deployForm = $state<{
    name: string;
    port: number;
    volume_path: string;
    env_values: Record<string, string>;
  }>({
    name: '',
    port: 80,
    volume_path: '',
    env_values: {},
  });

  // Editor Konfigurasi daemon.json
  let rawJsonConfig = $state('');
  let savingConfig = $state(false);

  // Polling interval
  let timer: any = null;

  onMount(async () => {
    await fetchStatus();
    await fetchSystemInfo();
    await fetchDaemonConfig();
    await switchTab(activeTab);

    // Refresh metrik status setiap 15 detik
    timer = setInterval(() => {
      fetchStatus(false);
    }, 15000);
  });

  onDestroy(() => {
    if (timer) clearInterval(timer);
  });

  // ── Pemuatan Data Server ────────────────────────────────────────────────────
  async function fetchStatus(showLoading = true) {
    if (showLoading) loadingStatus = true;
    try {
      const res = await apiFetch<DockerStatus>('/api/docker/status');
      status = res;
    } catch (err: any) {
      console.error('Gagal mengambil status Docker:', err);
    } finally {
      if (showLoading) loadingStatus = false;
    }
  }

  async function fetchSystemInfo() {
    try {
      const res = await apiFetch<DockerSystemInfo>('/api/docker/system-info');
      systemInfo = res;
    } catch (err: any) {
      console.error('Gagal mengambil spesifikasi host Docker:', err);
    }
  }

  async function fetchDaemonConfig() {
    try {
      const res = await apiFetch<DockerDaemonConfig>('/api/docker/config');
      daemonConfig = res;
      rawJsonConfig = res.raw_json || '{\n}';
    } catch (err: any) {
      console.error('Gagal mengambil daemon.json Docker:', err);
    }
  }

  async function switchTab(tab: DockerTab) {
    activeTab = tab;
    if (tab === 'container') await loadContainers();
    else if (tab === 'local_image') await loadImages();
    else if (tab === 'template') await loadTemplates();
    else if (tab === 'volume') await loadVolumes();
    else if (tab === 'network') await loadNetworks();
    else if (tab === 'compose') await loadComposeProjects();
    else if (tab === 'repository') await loadRegistries();
    else if (tab === 'settings') {
      await fetchDaemonConfig();
      await fetchSystemInfo();
    }
  }

  // ── Operasi Kontainer ───────────────────────────────────────────────────────
  async function loadContainers() {
    loadingContainers = true;
    try {
      const res = await apiFetch<{ ok: boolean; containers: DockerContainer[] }>('/api/docker/containers');
      containers = res.containers || [];
    } catch (err: any) {
      notify('Gagal memuat kontainer: ' + (err.message || 'Error server'), 'error');
    } finally {
      loadingContainers = false;
    }
  }

  async function controlContainer(id: string, action: 'start' | 'stop' | 'restart' | 'pause' | 'unpause' | 'delete') {
    if (action === 'delete') {
      const confirmed = await confirmDialog({
        title: 'Hapus Kontainer',
        message: `Hapus kontainer "${id.slice(0, 12)}"? Kontainer yang dihapus tidak dapat dipulihkan.`,
        confirmText: 'Hapus Permanen',
        tone: 'danger',
      });
      if (!confirmed) return;
    }

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/docker/containers/${id}/${action}`, {
        method: 'POST',
      });
      notify(res.message || `Aksi ${action} berhasil`, 'success');
      await loadContainers();
      await fetchStatus(false);
    } catch (err: any) {
      notify(`Gagal ${action} kontainer: ` + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function openContainerLogs(c: DockerContainer) {
    const name = c.names[0]?.replace(/^\//, '') || c.id.slice(0, 12);
    selectedContainerLogs = { id: c.id, name, content: '', lines: 200 };
    showLogsModal = true;
    await refreshContainerLogs();
  }

  async function refreshContainerLogs() {
    if (!selectedContainerLogs.id) return;
    logsLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; logs: string }>(
        `/api/docker/containers/${selectedContainerLogs.id}/logs?lines=${selectedContainerLogs.lines}`
      );
      selectedContainerLogs.content = res.logs || 'Belum ada catatan log dari kontainer.';
    } catch (err: any) {
      selectedContainerLogs.content = 'Gagal memuat log: ' + (err.message || 'Error');
    } finally {
      logsLoading = false;
    }
  }

  function addPortMapping() {
    newContainer.ports.push({ host: '', container: '', protocol: 'tcp' });
  }
  function removePortMapping(idx: number) {
    newContainer.ports = newContainer.ports.filter((_, i) => i !== idx);
  }

  function addVolumeMapping() {
    newContainer.volumes.push({ host: '', container: '' });
  }
  function removeVolumeMapping(idx: number) {
    newContainer.volumes = newContainer.volumes.filter((_, i) => i !== idx);
  }

  function addEnvParam() {
    newContainer.envs.push({ key: '', value: '' });
  }
  function removeEnvParam(idx: number) {
    newContainer.envs = newContainer.envs.filter((_, i) => i !== idx);
  }

  async function submitCreateContainer() {
    if (!newContainer.name.trim() || !newContainer.image.trim()) {
      notify('Nama kontainer dan citra wajib diisi', 'warning');
      return;
    }

    const ports: string[] = [];
    for (const p of newContainer.ports) {
      if (p.host.trim() && p.container.trim()) {
        ports.push(`${p.host.trim()}:${p.container.trim()}/${p.protocol}`);
      }
    }

    const vols: string[] = [];
    for (const v of newContainer.volumes) {
      if (v.host.trim() && v.container.trim()) {
        vols.push(`${v.host.trim()}:${v.container.trim()}`);
      }
    }

    const envs: string[] = [];
    for (const e of newContainer.envs) {
      if (e.key.trim()) {
        envs.push(`${e.key.trim()}=${e.value.trim()}`);
      }
    }

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/containers', {
        method: 'POST',
        body: JSON.stringify({
          name: newContainer.name.trim(),
          image: newContainer.image.trim(),
          ports,
          volumes: vols,
          environment: envs,
          restart_policy: newContainer.restart_policy,
          auto_remove: newContainer.auto_remove,
          command: newContainer.cmd.trim(),
        }),
      });

      notify(res.message || 'Kontainer berhasil dibuat', 'success');
      showCreateContainerModal = false;
      // Reset form
      newContainer = {
        name: '',
        image: '',
        ports: [{ host: '', container: '', protocol: 'tcp' }],
        volumes: [{ host: '', container: '' }],
        envs: [{ key: '', value: '' }],
        restart_policy: 'unless-stopped',
        auto_remove: false,
        cmd: '',
      };
      await loadContainers();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal membuat kontainer: ' + (err.message || 'Error server'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  // ── Operasi Citra (Images) ──────────────────────────────────────────────────
  async function loadImages() {
    loadingImages = true;
    try {
      const res = await apiFetch<{ ok: boolean; images: DockerImage[] }>('/api/docker/images');
      images = res.images || [];
    } catch (err: any) {
      notify('Gagal memuat daftar citra: ' + (err.message || 'Error'), 'error');
    } finally {
      loadingImages = false;
    }
  }

  async function submitPullImage() {
    if (!pullImageName.trim()) {
      notify('Nama citra wajib diisi', 'warning');
      return;
    }
    const fullTag = pullImageTag.trim() ? `${pullImageName.trim()}:${pullImageTag.trim()}` : pullImageName.trim();
    pulling = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/images/pull', {
        method: 'POST',
        body: JSON.stringify({ image: fullTag }),
      });
      notify(res.message || `Citra ${fullTag} berhasil diunduh`, 'success');
      showPullImageModal = false;
      pullImageName = '';
      pullImageTag = 'latest';
      await loadImages();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal mengunduh citra: ' + (err.message || 'Error'), 'error');
    } finally {
      pulling = false;
    }
  }

  async function deleteImage(img: DockerImage) {
    const confirmed = await confirmDialog({
      title: 'Hapus Citra Docker',
      message: `Hapus citra "${img.repository}:${img.tag}" (${img.size_human})?`,
      confirmText: 'Hapus Citra',
      tone: 'danger',
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/docker/images/${img.id}`, {
        method: 'DELETE',
      });
      notify(res.message || 'Citra berhasil dihapus', 'success');
      await loadImages();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal menghapus citra: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function searchCloudImages() {
    if (!cloudSearchQuery.trim()) {
      notify('Ketik kata kunci pencarian Docker Hub', 'warning');
      return;
    }
    cloudSearching = true;
    try {
      const res = await apiFetch<{ ok: boolean; results: any[] }>(
        `/api/docker/images/search?q=${encodeURIComponent(cloudSearchQuery.trim())}`
      );
      cloudResults = res.results || [];
      if (cloudResults.length === 0) {
        notify('Tidak ditemukan citra dengan kata kunci tersebut', 'info');
      }
    } catch (err: any) {
      notify('Gagal mencari di Docker Hub: ' + (err.message || 'Error'), 'error');
    } finally {
      cloudSearching = false;
    }
  }

  function openPullModalFor(repoName: string) {
    pullImageName = repoName;
    pullImageTag = 'latest';
    showPullImageModal = true;
  }

  function openRunContainerFor(img: DockerImage) {
    const full = `${img.repository}:${img.tag}`;
    newContainer.image = full;
    newContainer.name = img.repository.split('/').pop()?.replace(/[^a-zA-Z0-9_-]/g, '') || 'app';
    showCreateContainerModal = true;
  }

  // ── Operasi Volume ──────────────────────────────────────────────────────────
  async function loadVolumes() {
    loadingVolumes = true;
    try {
      const res = await apiFetch<{ ok: boolean; volumes: DockerVolume[] }>('/api/docker/volumes');
      volumes = res.volumes || [];
    } catch (err: any) {
      notify('Gagal memuat volume: ' + (err.message || 'Error'), 'error');
    } finally {
      loadingVolumes = false;
    }
  }

  async function submitCreateVolume() {
    if (!newVolume.name.trim()) {
      notify('Nama volume wajib diisi', 'warning');
      return;
    }
    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/volumes', {
        method: 'POST',
        body: JSON.stringify({ name: newVolume.name.trim(), driver: newVolume.driver }),
      });
      notify(res.message || 'Volume berhasil dibuat', 'success');
      showCreateVolumeModal = false;
      newVolume.name = '';
      await loadVolumes();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal membuat volume: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function deleteVolume(v: DockerVolume) {
    const confirmed = await confirmDialog({
      title: 'Hapus Volume',
      message: `Hapus volume "${v.name}"? Data persisten di dalamnya akan hilang permanen!`,
      confirmText: 'Hapus Volume',
      tone: 'danger',
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/docker/volumes/${v.name}`, {
        method: 'DELETE',
      });
      notify(res.message || 'Volume berhasil dihapus', 'success');
      await loadVolumes();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal menghapus volume: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function pruneVolumes() {
    const confirmed = await confirmDialog({
      title: 'Bersihkan Volume Tak Terpakai',
      message: 'Apakah Anda yakin ingin menghapus seluruh volume yang saat ini tidak terkait dengan kontainer manapun?',
      confirmText: 'Prune All Unused',
      tone: 'danger',
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/volumes/prune', {
        method: 'POST',
      });
      notify(res.message || 'Volume tak terpakai berhasil dibersihkan', 'success');
      await loadVolumes();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal membersihkan volume: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  // ── Operasi Network ─────────────────────────────────────────────────────────
  async function loadNetworks() {
    loadingNetworks = true;
    try {
      const res = await apiFetch<{ ok: boolean; networks: DockerNetwork[] }>('/api/docker/networks');
      networks = res.networks || [];
    } catch (err: any) {
      notify('Gagal memuat jaringan Docker: ' + (err.message || 'Error'), 'error');
    } finally {
      loadingNetworks = false;
    }
  }

  async function submitCreateNetwork() {
    if (!newNetwork.name.trim()) {
      notify('Nama jaringan wajib diisi', 'warning');
      return;
    }
    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/networks', {
        method: 'POST',
        body: JSON.stringify(newNetwork),
      });
      notify(res.message || 'Jaringan berhasil dibuat', 'success');
      showCreateNetworkModal = false;
      newNetwork.name = '';
      await loadNetworks();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal membuat jaringan: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function deleteNetwork(net: DockerNetwork) {
    const confirmed = await confirmDialog({
      title: 'Hapus Jaringan Docker',
      message: `Hapus jaringan "${net.name}"? Kontainer yang terhubung ke jaringan ini akan terputus.`,
      confirmText: 'Hapus Jaringan',
      tone: 'danger',
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/docker/networks/${net.id}`, {
        method: 'DELETE',
      });
      notify(res.message || 'Jaringan berhasil dihapus', 'success');
      await loadNetworks();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal menghapus jaringan: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  // ── Operasi Docker Compose ──────────────────────────────────────────────────
  async function loadComposeProjects() {
    loadingCompose = true;
    try {
      const res = await apiFetch<{ ok: boolean; projects: DockerComposeProject[] }>('/api/docker/compose');
      composeProjects = res.projects || [];
    } catch (err: any) {
      notify('Gagal memuat proyek compose: ' + (err.message || 'Error'), 'error');
    } finally {
      loadingCompose = false;
    }
  }

  async function submitComposeUp() {
    if (!composeForm.name.trim() || !composeForm.yaml.trim()) {
      notify('Nama proyek dan konfigurasi YAML wajib diisi', 'warning');
      return;
    }
    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/compose/up', {
        method: 'POST',
        body: JSON.stringify(composeForm),
      });
      notify(res.message || 'Stack Compose berhasil dijalankan', 'success');
      showComposeModal = false;
      await loadComposeProjects();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal menjalankan Compose: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function composeDown(project: DockerComposeProject) {
    const confirmed = await confirmDialog({
      title: 'Hentikan Stack Compose',
      message: `Hentikan seluruh kontainer dan jaringan dari proyek "${project.name}"?`,
      confirmText: 'Hentikan (Down)',
      tone: 'danger',
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/compose/down', {
        method: 'POST',
        body: JSON.stringify({ name: project.name }),
      });
      notify(res.message || 'Stack Compose berhasil dihentikan', 'success');
      await loadComposeProjects();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal menghentikan Compose: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  function editComposeProject(p: DockerComposeProject) {
    composeForm.name = p.name;
    composeForm.yaml = p.yaml_content;
    showComposeModal = true;
  }

  // ── Operasi One-Click Install Template ──────────────────────────────────────
  async function loadTemplates() {
    loadingTemplates = true;
    try {
      const res = await apiFetch<{ ok: boolean; templates: OneClickTemplate[] }>('/api/docker/templates');
      templates = res.templates || [];
    } catch (err: any) {
      notify('Gagal memuat katalog template: ' + (err.message || 'Error'), 'error');
    } finally {
      loadingTemplates = false;
    }
  }

  function openDeployTemplate(tmpl: OneClickTemplate) {
    selectedTemplate = tmpl;
    const initialEnv: Record<string, string> = {};
    for (const p of tmpl.env_params) {
      initialEnv[p.key] = p.default;
    }
    deployForm = {
      name: `${tmpl.id}-app`,
      port: tmpl.default_port,
      volume_path: tmpl.volume_mount ? `/www/dk_project/${tmpl.id}/data` : '',
      env_values: initialEnv,
    };
    showDeployTemplateModal = true;
  }

  async function submitDeployTemplate() {
    if (!selectedTemplate) return;
    if (!deployForm.name.trim()) {
      notify('Nama aplikasi wajib diisi', 'warning');
      return;
    }

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/templates/deploy', {
        method: 'POST',
        body: JSON.stringify({
          template_id: selectedTemplate.id,
          name: deployForm.name.trim(),
          port: deployForm.port,
          volume_path: deployForm.volume_path.trim(),
          env_values: deployForm.env_values,
        }),
      });

      notify(res.message || 'Aplikasi berhasil dipasang dan berjalan', 'success');
      showDeployTemplateModal = false;
      await fetchStatus(false);
      await switchTab('container');
    } catch (err: any) {
      notify('Gagal memasang aplikasi: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  // ── Operasi Registri ────────────────────────────────────────────────────────
  async function loadRegistries() {
    loadingRegistries = true;
    try {
      const res = await apiFetch<{ ok: boolean; registries: DockerRegistry[] }>('/api/docker/registries');
      registries = res.registries || [];
    } catch (err: any) {
      notify('Gagal memuat registri: ' + (err.message || 'Error'), 'error');
    } finally {
      loadingRegistries = false;
    }
  }

  async function submitAddRegistry() {
    if (!newRegistry.name.trim() || !newRegistry.url.trim()) {
      notify('Nama dan URL registri wajib diisi', 'warning');
      return;
    }
    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/registries', {
        method: 'POST',
        body: JSON.stringify(newRegistry),
      });
      notify(res.message || 'Registri berhasil ditambahkan', 'success');
      showAddRegistryModal = false;
      newRegistry = { name: '', url: '', username: '', password: '' };
      await loadRegistries();
    } catch (err: any) {
      notify('Gagal menambahkan registri: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function deleteRegistry(reg: DockerRegistry) {
    const confirmed = await confirmDialog({
      title: 'Hapus Registri Privat',
      message: `Hapus kredensial registri "${reg.name}" (${reg.url})?`,
      confirmText: 'Hapus Registri',
      tone: 'danger',
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/docker/registries/${reg.id}`, {
        method: 'DELETE',
      });
      notify(res.message || 'Registri berhasil dihapus', 'success');
      await loadRegistries();
    } catch (err: any) {
      notify('Gagal menghapus registri: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  // ── Operasi Daemon & Pengaturan (Settings aaPanel) ───────────────────────────
  async function controlDockerService(action: 'start' | 'stop' | 'restart' | 'repair') {
    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/docker/service/${action}`, {
        method: 'POST',
      });
      notify(res.message || `Layanan Docker di-${action}`, 'success');
      await fetchStatus(true);
    } catch (err: any) {
      notify(`Gagal ${action} layanan Docker: ` + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function installDockerEngine() {
    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; task_id: string; message: string }>('/api/docker/install', {
        method: 'POST',
      });
      notify(res.message || 'Tugas instalasi Docker dikirim ke Messages Box', 'success');
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal memulai instalasi Docker: ' + (err.message || 'Error'), 'error');
    } finally {
      actionLoading = false;
    }
  }

  async function saveDaemonConfig() {
    if (!daemonConfig) return;
    savingConfig = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/config', {
        method: 'POST',
        body: JSON.stringify(daemonConfig),
      });
      notify(res.message || 'Konfigurasi Docker daemon.json disimpan', 'success');
      showConfigModal = false;
      showLogRotationModal = false;
      await fetchDaemonConfig();
    } catch (err: any) {
      notify('Gagal menyimpan konfigurasi: ' + (err.message || 'Error format JSON'), 'error');
    } finally {
      savingConfig = false;
    }
  }

  async function saveRawJsonConfig() {
    if (!daemonConfig) return;
    try {
      // Validasi sintaks JSON
      JSON.parse(rawJsonConfig);
    } catch (e: any) {
      notify('Sintaks JSON tidak valid: ' + e.message, 'error');
      return;
    }

    daemonConfig.raw_json = rawJsonConfig;
    await saveDaemonConfig();
  }

  async function uninstallDocker() {
    const confirmed = await confirmDialog({
      title: 'Uninstall Docker Engine & Data',
      message: 'PERINGATAN TINGKAT TINGGI: Menghapus Docker akan menghentikan seluruh kontainer, menghapus seluruh citra, volume, dan berkas konfigurasi daemon. Anda yakin ingin melanjutkan?',
      confirmText: 'Uninstall Docker Sekarang',
      tone: 'danger',
    });
    if (!confirmed) return;

    notify('Proses uninstall Docker dimulai di latar belakang...', 'warning');
    await controlDockerService('stop');
  }

  // Filter kontainer
  let filteredContainers = $derived(
    containers.filter((c) => {
      const matchSearch =
        !containerSearch ||
        c.names.some((n) => n.toLowerCase().includes(containerSearch.toLowerCase())) ||
        c.image.toLowerCase().includes(containerSearch.toLowerCase()) ||
        c.id.toLowerCase().includes(containerSearch.toLowerCase());

      if (!matchSearch) return false;
      if (containerFilter === 'all') return true;
      if (containerFilter === 'running') return c.state === 'running';
      if (containerFilter === 'stopped') return c.state === 'exited' || c.state === 'stopped';
      if (containerFilter === 'paused') return c.state === 'paused';
      return true;
    })
  );

  // Filter templates
  let filteredTemplates = $derived(
    templates.filter((t) => {
      if (templateCategory === 'all') return true;
      return t.category.toLowerCase() === templateCategory.toLowerCase();
    })
  );
</script>

<div class="pg-docker docker-page">
  <!-- Peringatan Melayang Bila Docker / Compose Belum Terpasang (Screenshot Match) -->
  {#if status && (!status.installed || !status.compose_installed)}
    <aside class="floating-banner" aria-label="Status instalasi Docker">
      <div class="banner-content">
        <i class="ri-alert-fill banner-icon" aria-hidden="true"></i>
        <span>Docker or docker-compose not installed, please install before using</span>
      </div>
      <button
        class="btn btn--primary btn--sm banner-btn"
        disabled={actionLoading}
        onclick={installDockerEngine}
      >
        <i class="ri-download-cloud-2-line" aria-hidden="true"></i>
        <span>Install Docker Suite</span>
      </button>
    </aside>
  {/if}

  <!-- Header Modul & Status Ringkas -->
  <header class="page-head">
    <div class="head-left">
      <div class="title-cluster">
        <h1 class="page-title">
          <i class="ri-box-3-fill title-icon" aria-hidden="true"></i>
          <span>Docker Management Suite</span>
        </h1>
        <div class="badges-row">
          {#if status?.running}
            <span class="badge badge--success">
              <span class="status-dot status-dot--active"></span>
              Docker Running ({status.version || 'v26.1'})
            </span>
          {:else if status?.installed}
            <span class="badge badge--danger">
              <span class="status-dot status-dot--danger"></span>
              Docker Stopped
            </span>
          {:else}
            <span class="badge badge--warning">
              <span class="status-dot status-dot--warning"></span>
              Not Installed
            </span>
          {/if}

          {#if status?.compose_installed}
            <span class="badge badge--info">
              <i class="ri-stack-line" aria-hidden="true"></i>
              Compose {status.compose_version || 'v2.27'}
            </span>
          {/if}
        </div>
      </div>
      <p class="page-subtitle">
        Pusat kendali kontainerisasi lengkap, manajemen citra lokal & cloud, docker-compose, volume, jaringan, serta template 1-klik terisolasi.
      </p>
    </div>

    <div class="head-actions">
      <button
        class="btn btn--secondary btn--sm"
        title="Muat ulang seluruh data"
        disabled={loadingStatus}
        onclick={() => {
          fetchStatus();
          switchTab(activeTab);
        }}
      >
        <i class="ri-refresh-line {loadingStatus ? 'spin' : ''}" aria-hidden="true"></i>
        <span>Segarkan</span>
      </button>
    </div>
  </header>

  <!-- 10 Tab Navigasi Horizontal Sesuai aaPanel Suite -->
  <nav class="tabs nav-tabs-bar" aria-label="Navigasi Docker">
    <button
      class="tabs__tab"
      class:is-active={activeTab === 'overview'}
      onclick={() => switchTab('overview')}
    >
      <i class="ri-pie-chart-2-line" aria-hidden="true"></i>
      <span>Overview</span>
    </button>
    <button
      class="tabs__tab"
      class:is-active={activeTab === 'container'}
      onclick={() => switchTab('container')}
    >
      <i class="ri-terminal-box-line" aria-hidden="true"></i>
      <span>Container</span>
      {#if status?.containers_total}
        <span class="tab-count">{status.containers_total}</span>
      {/if}
    </button>
    <button
      class="tabs__tab"
      class:is-active={activeTab === 'template'}
      onclick={() => switchTab('template')}
    >
      <i class="ri-magic-line" aria-hidden="true"></i>
      <span>One-Click Install</span>
    </button>
    <button
      class="tabs__tab"
      class:is-active={activeTab === 'cloud_image'}
      onclick={() => switchTab('cloud_image')}
    >
      <i class="ri-cloud-line" aria-hidden="true"></i>
      <span>Cloud image</span>
    </button>
    <button
      class="tabs__tab"
      class:is-active={activeTab === 'local_image'}
      onclick={() => switchTab('local_image')}
    >
      <i class="ri-hard-drive-2-line" aria-hidden="true"></i>
      <span>Local image</span>
      {#if status?.images_total}
        <span class="tab-count">{status.images_total}</span>
      {/if}
    </button>
    <button
      class="tabs__tab"
      class:is-active={activeTab === 'compose'}
      onclick={() => switchTab('compose')}
    >
      <i class="ri-git-merge-line" aria-hidden="true"></i>
      <span>Docker Compose</span>
    </button>
    <button
      class="tabs__tab"
      class:is-active={activeTab === 'network'}
      onclick={() => switchTab('network')}
    >
      <i class="ri-global-line" aria-hidden="true"></i>
      <span>Network</span>
      {#if status?.networks_total}
        <span class="tab-count">{status.networks_total}</span>
      {/if}
    </button>
    <button
      class="tabs__tab"
      class:is-active={activeTab === 'volume'}
      onclick={() => switchTab('volume')}
    >
      <i class="ri-database-line" aria-hidden="true"></i>
      <span>Volume</span>
      {#if status?.volumes_total}
        <span class="tab-count">{status.volumes_total}</span>
      {/if}
    </button>
    <button
      class="tabs__tab"
      class:is-active={activeTab === 'repository'}
      onclick={() => switchTab('repository')}
    >
      <i class="ri-shield-keyhole-line" aria-hidden="true"></i>
      <span>Repository</span>
    </button>
    <button
      class="tabs__tab"
      class:is-active={activeTab === 'settings'}
      onclick={() => switchTab('settings')}
    >
      <i class="ri-settings-3-line" aria-hidden="true"></i>
      <span>Settings</span>
    </button>
  </nav>

  <!-- KONTEN TAB -->
  <main class="tab-body">
    <!-- ══════════════ TAB 1: OVERVIEW ══════════════ -->
    {#if activeTab === 'overview'}
      <OverviewTab
        {status}
        {systemInfo}
        onOpenCreateContainer={() => (showCreateContainerModal = true)}
        onOpenPullImage={() => (showPullImageModal = true)}
        onSwitchToTemplate={() => switchTab('template')}
        onOpenCompose={() => (showComposeModal = true)}
        onPruneVolumes={pruneVolumes}
      />
    {/if}

    <!-- ══════════════ TAB 2: CONTAINER ══════════════ -->
    {#if activeTab === 'container'}
      <ContainerTab
        {containers}
        {filteredContainers}
        {loadingContainers}
        {actionLoading}
        bind:containerFilter
        bind:containerSearch
        onOpenCreateContainer={() => (showCreateContainerModal = true)}
        onLoadContainers={loadContainers}
        onControlContainer={controlContainer}
        onOpenContainerLogs={openContainerLogs}
      />
    {/if}

    <!-- ══════════════ TAB 3: ONE-CLICK INSTALL ══════════════ -->
    {#if activeTab === 'template'}
      <TemplateTab
        {filteredTemplates}
        {loadingTemplates}
        bind:templateCategory
        onOpenDeployTemplate={openDeployTemplate}
      />
    {/if}

    <!-- ══════════════ TAB 4: CLOUD IMAGE (DOCKER HUB) ══════════════ -->
    {#if activeTab === 'cloud_image'}
      <CloudImageTab
        bind:cloudSearchQuery
        {cloudSearching}
        {cloudResults}
        onSearchCloudImages={searchCloudImages}
        onOpenPullModalFor={openPullModalFor}
      />
    {/if}

    <!-- ══════════════ TAB 5: LOCAL IMAGE ══════════════ -->
    {#if activeTab === 'local_image'}
      <LocalImageTab
        {images}
        {loadingImages}
        {actionLoading}
        bind:imageSearch
        onOpenPullImage={() => (showPullImageModal = true)}
        onLoadImages={loadImages}
        onOpenRunContainerFor={openRunContainerFor}
        onDeleteImage={deleteImage}
      />
    {/if}

    <!-- ══════════════ TAB 6: DOCKER COMPOSE ══════════════ -->
    {#if activeTab === 'compose'}
      <ComposeTab
        {composeProjects}
        {loadingCompose}
        {actionLoading}
        onOpenComposeModal={() => (showComposeModal = true)}
        onLoadComposeProjects={loadComposeProjects}
        onEditComposeProject={editComposeProject}
        onComposeDown={composeDown}
        onComposeUpForProject={(proj) => {
          composeForm.name = proj.name;
          composeForm.yaml = proj.yaml_content;
          submitComposeUp();
        }}
      />
    {/if}

    <!-- ══════════════ TAB 7: NETWORK ══════════════ -->
    {#if activeTab === 'network'}
      <NetworkTab
        {networks}
        {loadingNetworks}
        {actionLoading}
        onOpenCreateNetwork={() => (showCreateNetworkModal = true)}
        onLoadNetworks={loadNetworks}
        onDeleteNetwork={deleteNetwork}
      />
    {/if}

    <!-- ══════════════ TAB 8: VOLUME ══════════════ -->
    {#if activeTab === 'volume'}
      <VolumeTab
        {volumes}
        {loadingVolumes}
        {actionLoading}
        onOpenCreateVolume={() => (showCreateVolumeModal = true)}
        onPruneVolumes={pruneVolumes}
        onLoadVolumes={loadVolumes}
        onDeleteVolume={deleteVolume}
      />
    {/if}

    <!-- ══════════════ TAB 9: REPOSITORY (PRIVATE REGISTRY) ══════════════ -->
    {#if activeTab === 'repository'}
      <RepositoryTab
        {registries}
        {loadingRegistries}
        {actionLoading}
        onOpenAddRegistry={() => (showAddRegistryModal = true)}
        onLoadRegistries={loadRegistries}
        onDeleteRegistry={deleteRegistry}
      />
    {/if}

    <!-- ══════════════ TAB 10: SETTINGS (PERSIS TAMPILAN SCREENSHOT AAPANEL) ══════════════ -->
    {#if activeTab === 'settings'}
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
          rawJsonConfig = daemonConfig?.raw_json || '{\n}';
          showConfigModal = true;
        }}
        onUninstallDocker={uninstallDocker}
      />
    {/if}
  </main>

  <!-- ══════════════ MODAL: BUAT KONTAINER BARU ══════════════ -->
  <CreateContainerModal
    bind:showCreateContainerModal
    bind:newContainer
    {actionLoading}
    onSubmitCreateContainer={submitCreateContainer}
    onAddPortMapping={addPortMapping}
    onRemovePortMapping={removePortMapping}
    onAddVolumeMapping={addVolumeMapping}
    onRemoveVolumeMapping={removeVolumeMapping}
    onAddEnvParam={addEnvParam}
    onRemoveEnvParam={removeEnvParam}
  />

  <!-- ══════════════ MODAL: TARIK CITRA (PULL IMAGE) ══════════════ -->
  <PullImageModal
    bind:showPullImageModal
    bind:pullImageName
    bind:pullImageTag
    {pulling}
    onSubmitPullImage={submitPullImage}
  />

  <!-- ══════════════ MODAL: LOG KONTAINER ══════════════ -->
  <ContainerLogsModal
    bind:showLogsModal
    bind:selectedContainerLogs
    {logsLoading}
    onRefreshContainerLogs={refreshContainerLogs}
  />

  <!-- ══════════════ MODAL: DOCKER COMPOSE EDITOR ══════════════ -->
  <Modal
    open={showComposeModal}
    title="Stack Docker Compose"
    size="lg"
    onclose={() => (showComposeModal = false)}
  >
    <form id="docker-compose-form" class="docker-modal-form" onsubmit={(e) => { e.preventDefault(); submitComposeUp(); }}>
      <div class="field">
        <label class="field__label" for="compose-name">Nama Proyek / Stack *</label>
        <input
          id="compose-name"
          type="text"
          class="input font-mono"
          placeholder="misal: wordpress-stack atau microservices"
          bind:value={composeForm.name}
          required
        />
      </div>

      <div class="field">
        <label class="field__label" for="compose-yaml">Konfigurasi docker-compose.yml *</label>
        <textarea
          id="compose-yaml"
          class="input font-mono text-xs"
          rows="12"
          bind:value={composeForm.yaml}
          required
        ></textarea>
      </div>
    </form>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary" onclick={() => (showComposeModal = false)}>
        Batal
      </button>
      <button type="submit" form="docker-compose-form" class="btn btn--primary" disabled={actionLoading}>
        <i class="ri-play-line" aria-hidden="true"></i>
        <span>Deploy Stack (Compose Up)</span>
      </button>
    {/snippet}
  </Modal>

  <!-- ══════════════ MODAL: BUAT JARINGAN ══════════════ -->
  <Modal
    open={showCreateNetworkModal}
    title="Buat Jaringan Virtual Baru"
    size="md"
    onclose={() => (showCreateNetworkModal = false)}
  >
    <form id="docker-create-net-form" class="docker-modal-form" onsubmit={(e) => { e.preventDefault(); submitCreateNetwork(); }}>
      <div class="field">
        <label class="field__label" for="net-name">Nama Network *</label>
        <input
          id="net-name"
          type="text"
          class="input font-mono"
          placeholder="misal: backend-net"
          bind:value={newNetwork.name}
          required
        />
      </div>

      <div class="field">
        <label class="field__label" for="net-driver">Driver</label>
        <select id="net-driver" class="input" bind:value={newNetwork.driver}>
          <option value="bridge">bridge (Default isolasi lokal)</option>
          <option value="host">host (Gunakan network host langsung)</option>
          <option value="overlay">overlay (Multi-host swarm)</option>
          <option value="macvlan">macvlan</option>
        </select>
      </div>

      <div class="form-row">
        <div class="field flex-1">
          <label class="field__label" for="net-subnet">Subnet CIDR</label>
          <input
            id="net-subnet"
            type="text"
            class="input font-mono"
            placeholder="172.28.0.0/16"
            bind:value={newNetwork.subnet}
          />
        </div>
        <div class="field flex-1">
          <label class="field__label" for="net-gw">Gateway IP</label>
          <input
            id="net-gw"
            type="text"
            class="input font-mono"
            placeholder="172.28.0.1"
            bind:value={newNetwork.gateway}
          />
        </div>
      </div>
    </form>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary" onclick={() => (showCreateNetworkModal = false)}>
        Batal
      </button>
      <button type="submit" form="docker-create-net-form" class="btn btn--primary" disabled={actionLoading}>
        <i class="ri-check-line" aria-hidden="true"></i>
        <span>Buat Jaringan</span>
      </button>
    {/snippet}
  </Modal>

  <!-- ══════════════ MODAL: BUAT VOLUME ══════════════ -->
  <Modal
    open={showCreateVolumeModal}
    title="Buat Volume Docker Baru"
    size="md"
    onclose={() => (showCreateVolumeModal = false)}
  >
    <form id="docker-create-vol-form" class="docker-modal-form" onsubmit={(e) => { e.preventDefault(); submitCreateVolume(); }}>
      <div class="field">
        <label class="field__label" for="vol-name">Nama Volume *</label>
        <input
          id="vol-name"
          type="text"
          class="input font-mono"
          placeholder="misal: pg_data, redis_cache"
          bind:value={newVolume.name}
          required
        />
      </div>

      <div class="field">
        <label class="field__label" for="vol-driver">Driver</label>
        <select id="vol-driver" class="input" bind:value={newVolume.driver}>
          <option value="local">local (Penyimpanan host bawaan)</option>
        </select>
      </div>
    </form>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary" onclick={() => (showCreateVolumeModal = false)}>
        Batal
      </button>
      <button type="submit" form="docker-create-vol-form" class="btn btn--primary" disabled={actionLoading}>
        <i class="ri-check-line" aria-hidden="true"></i>
        <span>Buat Volume</span>
      </button>
    {/snippet}
  </Modal>

  <!-- ══════════════ MODAL: REGISTRI PRIVAT ══════════════ -->
  <Modal
    open={showAddRegistryModal}
    title="Tambah Kredensial Registri Privat"
    size="md"
    onclose={() => (showAddRegistryModal = false)}
  >
    <form id="docker-add-reg-form" class="docker-modal-form" onsubmit={(e) => { e.preventDefault(); submitAddRegistry(); }}>
      <div class="field">
        <label class="field__label" for="reg-name">Nama Registri *</label>
        <input
          id="reg-name"
          type="text"
          class="input"
          placeholder="misal: Harbor Internal, Gitlab Registry"
          bind:value={newRegistry.name}
          required
        />
      </div>

      <div class="field">
        <label class="field__label" for="reg-url">URL Registri *</label>
        <input
          id="reg-url"
          type="text"
          class="input font-mono"
          placeholder="misal: registry.mycompany.com atau ghcr.io"
          bind:value={newRegistry.url}
          required
        />
      </div>

      <div class="form-row">
        <div class="field flex-1">
          <label class="field__label" for="reg-user">Username</label>
          <input
            id="reg-user"
            type="text"
            class="input"
            placeholder="Username atau Token"
            bind:value={newRegistry.username}
          />
        </div>
        <div class="field flex-1">
          <label class="field__label" for="reg-pass">Password / Token Akses</label>
          <input
            id="reg-pass"
            type="password"
            class="input font-mono"
            placeholder="••••••••"
            bind:value={newRegistry.password}
          />
        </div>
      </div>
    </form>

    {#snippet footer()}
      <button type="button" class="btn btn--secondary" onclick={() => (showAddRegistryModal = false)}>
        Batal
      </button>
      <button type="submit" form="docker-add-reg-form" class="btn btn--primary" disabled={actionLoading}>
        <i class="ri-check-line" aria-hidden="true"></i>
        <span>Simpan Registri</span>
      </button>
    {/snippet}
  </Modal>

  <!-- ══════════════ MODAL: DEPLOY ONE-CLICK TEMPLATE ══════════════ -->
  <Modal
    open={showDeployTemplateModal && Boolean(selectedTemplate)}
    size="lg"
    onclose={() => (showDeployTemplateModal = false)}
  >
    {#snippet header()}
      {#if selectedTemplate}
        <div class="flex-row items-center gap-3">
          <i class="{selectedTemplate.icon} text-xl text-accent" aria-hidden="true"></i>
          <div>
            <h2 class="modal__title">Pasang {selectedTemplate.name}</h2>
            <span class="text-xs text-muted font-mono">{selectedTemplate.image}</span>
          </div>
        </div>
      {/if}
    {/snippet}

    {#if selectedTemplate}
      <form id="docker-deploy-tmpl-form" class="docker-modal-form" onsubmit={(e) => { e.preventDefault(); submitDeployTemplate(); }}>
        <div class="form-row">
          <div class="field flex-1">
            <label class="field__label" for="deploy-name">Nama Kontainer *</label>
            <input
              id="deploy-name"
              type="text"
              class="input font-mono"
              bind:value={deployForm.name}
              required
            />
          </div>
          <div class="field flex-1">
            <label class="field__label" for="deploy-port">Port Host Terbuka *</label>
            <input
              id="deploy-port"
              type="number"
              class="input font-mono"
              bind:value={deployForm.port}
              required
            />
          </div>
        </div>

        {#if selectedTemplate.volume_mount}
          <div class="field">
            <label class="field__label" for="deploy-vol">Direktori Penyimpanan Host</label>
            <input
              id="deploy-vol"
              type="text"
              class="input font-mono"
              bind:value={deployForm.volume_path}
            />
            <span class="field__hint">
              Akan dipetakan ke direktori kontainer: <code class="font-mono">{selectedTemplate.volume_mount}</code>
            </span>
          </div>
        {/if}

        {#if selectedTemplate.env_params && selectedTemplate.env_params.length > 0}
          <div class="form-section">
            <span class="font-semibold text-sm">Parameter Konfigurasi Lingkungan</span>
            <div class="mt-2 space-y-2">
              {#each selectedTemplate.env_params as p}
                <div class="field mb-2">
                  <label class="field__label" for="env-{p.key}">
                    {p.label} <code class="font-mono text-2xs">({p.key})</code>
                  </label>
                  <input
                    id="env-{p.key}"
                    type={p.type === 'password' ? 'password' : 'text'}
                    class="input font-mono"
                    bind:value={deployForm.env_values[p.key]}
                  />
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </form>
    {/if}

    {#snippet footer()}
      <button type="button" class="btn btn--secondary" onclick={() => (showDeployTemplateModal = false)}>
        Batal
      </button>
      <button type="submit" form="docker-deploy-tmpl-form" class="btn btn--primary" disabled={actionLoading}>
        <i class="ri-check-line" aria-hidden="true"></i>
        <span>Mulai Pasang & Jalankan</span>
      </button>
    {/snippet}
  </Modal>
</div>
