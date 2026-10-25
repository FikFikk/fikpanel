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
      notify('Gagal memuat kontainer: ' + (err.message || 'Error server'), 'danger');
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
        danger: true,
      });
      if (!confirmed) return;
    }

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/docker/containers/${id}/${action}`, {
        method: 'POST',
      });
      notify(res.message || `Aksi ${action} berhasil`, 'accent');
      await loadContainers();
      await fetchStatus(false);
    } catch (err: any) {
      notify(`Gagal ${action} kontainer: ` + (err.message || 'Error'), 'danger');
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

      notify(res.message || 'Kontainer berhasil dibuat', 'accent');
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
      notify('Gagal membuat kontainer: ' + (err.message || 'Error server'), 'danger');
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
      notify('Gagal memuat daftar citra: ' + (err.message || 'Error'), 'danger');
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
      notify(res.message || `Citra ${fullTag} berhasil diunduh`, 'accent');
      showPullImageModal = false;
      pullImageName = '';
      pullImageTag = 'latest';
      await loadImages();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal mengunduh citra: ' + (err.message || 'Error'), 'danger');
    } finally {
      pulling = false;
    }
  }

  async function deleteImage(img: DockerImage) {
    const confirmed = await confirmDialog({
      title: 'Hapus Citra Docker',
      message: `Hapus citra "${img.repository}:${img.tag}" (${img.size_human})?`,
      confirmText: 'Hapus Citra',
      danger: true,
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/docker/images/${img.id}`, {
        method: 'DELETE',
      });
      notify(res.message || 'Citra berhasil dihapus', 'accent');
      await loadImages();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal menghapus citra: ' + (err.message || 'Error'), 'danger');
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
      notify('Gagal mencari di Docker Hub: ' + (err.message || 'Error'), 'danger');
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
      notify('Gagal memuat volume: ' + (err.message || 'Error'), 'danger');
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
      notify(res.message || 'Volume berhasil dibuat', 'accent');
      showCreateVolumeModal = false;
      newVolume.name = '';
      await loadVolumes();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal membuat volume: ' + (err.message || 'Error'), 'danger');
    } finally {
      actionLoading = false;
    }
  }

  async function deleteVolume(v: DockerVolume) {
    const confirmed = await confirmDialog({
      title: 'Hapus Volume',
      message: `Hapus volume "${v.name}"? Data persisten di dalamnya akan hilang permanen!`,
      confirmText: 'Hapus Volume',
      danger: true,
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/docker/volumes/${v.name}`, {
        method: 'DELETE',
      });
      notify(res.message || 'Volume berhasil dihapus', 'accent');
      await loadVolumes();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal menghapus volume: ' + (err.message || 'Error'), 'danger');
    } finally {
      actionLoading = false;
    }
  }

  async function pruneVolumes() {
    const confirmed = await confirmDialog({
      title: 'Bersihkan Volume Tak Terpakai',
      message: 'Apakah Anda yakin ingin menghapus seluruh volume yang saat ini tidak terkait dengan kontainer manapun?',
      confirmText: 'Prune All Unused',
      danger: true,
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/volumes/prune', {
        method: 'POST',
      });
      notify(res.message || 'Volume tak terpakai berhasil dibersihkan', 'accent');
      await loadVolumes();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal membersihkan volume: ' + (err.message || 'Error'), 'danger');
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
      notify('Gagal memuat jaringan Docker: ' + (err.message || 'Error'), 'danger');
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
      notify(res.message || 'Jaringan berhasil dibuat', 'accent');
      showCreateNetworkModal = false;
      newNetwork.name = '';
      await loadNetworks();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal membuat jaringan: ' + (err.message || 'Error'), 'danger');
    } finally {
      actionLoading = false;
    }
  }

  async function deleteNetwork(net: DockerNetwork) {
    const confirmed = await confirmDialog({
      title: 'Hapus Jaringan Docker',
      message: `Hapus jaringan "${net.name}"? Kontainer yang terhubung ke jaringan ini akan terputus.`,
      confirmText: 'Hapus Jaringan',
      danger: true,
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/docker/networks/${net.id}`, {
        method: 'DELETE',
      });
      notify(res.message || 'Jaringan berhasil dihapus', 'accent');
      await loadNetworks();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal menghapus jaringan: ' + (err.message || 'Error'), 'danger');
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
      notify('Gagal memuat proyek compose: ' + (err.message || 'Error'), 'danger');
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
      notify(res.message || 'Stack Compose berhasil dijalankan', 'accent');
      showComposeModal = false;
      await loadComposeProjects();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal menjalankan Compose: ' + (err.message || 'Error'), 'danger');
    } finally {
      actionLoading = false;
    }
  }

  async function composeDown(project: DockerComposeProject) {
    const confirmed = await confirmDialog({
      title: 'Hentikan Stack Compose',
      message: `Hentikan seluruh kontainer dan jaringan dari proyek "${project.name}"?`,
      confirmText: 'Hentikan (Down)',
      danger: true,
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/docker/compose/down', {
        method: 'POST',
        body: JSON.stringify({ name: project.name }),
      });
      notify(res.message || 'Stack Compose berhasil dihentikan', 'accent');
      await loadComposeProjects();
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal menghentikan Compose: ' + (err.message || 'Error'), 'danger');
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
      notify('Gagal memuat katalog template: ' + (err.message || 'Error'), 'danger');
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

      notify(res.message || 'Aplikasi berhasil dipasang dan berjalan', 'accent');
      showDeployTemplateModal = false;
      await fetchStatus(false);
      await switchTab('container');
    } catch (err: any) {
      notify('Gagal memasang aplikasi: ' + (err.message || 'Error'), 'danger');
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
      notify('Gagal memuat registri: ' + (err.message || 'Error'), 'danger');
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
      notify(res.message || 'Registri berhasil ditambahkan', 'accent');
      showAddRegistryModal = false;
      newRegistry = { name: '', url: '', username: '', password: '' };
      await loadRegistries();
    } catch (err: any) {
      notify('Gagal menambahkan registri: ' + (err.message || 'Error'), 'danger');
    } finally {
      actionLoading = false;
    }
  }

  async function deleteRegistry(reg: DockerRegistry) {
    const confirmed = await confirmDialog({
      title: 'Hapus Registri Privat',
      message: `Hapus kredensial registri "${reg.name}" (${reg.url})?`,
      confirmText: 'Hapus Registri',
      danger: true,
    });
    if (!confirmed) return;

    actionLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/docker/registries/${reg.id}`, {
        method: 'DELETE',
      });
      notify(res.message || 'Registri berhasil dihapus', 'accent');
      await loadRegistries();
    } catch (err: any) {
      notify('Gagal menghapus registri: ' + (err.message || 'Error'), 'danger');
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
      notify(res.message || `Layanan Docker di-${action}`, 'accent');
      await fetchStatus(true);
    } catch (err: any) {
      notify(`Gagal ${action} layanan Docker: ` + (err.message || 'Error'), 'danger');
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
      notify(res.message || 'Tugas instalasi Docker dikirim ke Messages Box', 'accent');
      await fetchStatus(false);
    } catch (err: any) {
      notify('Gagal memulai instalasi Docker: ' + (err.message || 'Error'), 'danger');
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
      notify(res.message || 'Konfigurasi Docker daemon.json disimpan', 'accent');
      showConfigModal = false;
      showLogRotationModal = false;
      await fetchDaemonConfig();
    } catch (err: any) {
      notify('Gagal menyimpan konfigurasi: ' + (err.message || 'Error format JSON'), 'danger');
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
      notify('Sintaks JSON tidak valid: ' + e.message, 'danger');
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
      danger: true,
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

<div class="docker-page">
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
  <nav class="nav-tabs-bar" aria-label="Navigasi Docker">
    <button
      class="tab-item {activeTab === 'overview' ? 'tab-item--active' : ''}"
      onclick={() => switchTab('overview')}
    >
      <i class="ri-pie-chart-2-line" aria-hidden="true"></i>
      <span>Overview</span>
    </button>
    <button
      class="tab-item {activeTab === 'container' ? 'tab-item--active' : ''}"
      onclick={() => switchTab('container')}
    >
      <i class="ri-terminal-box-line" aria-hidden="true"></i>
      <span>Container</span>
      {#if status?.containers_total}
        <span class="tab-count">{status.containers_total}</span>
      {/if}
    </button>
    <button
      class="tab-item {activeTab === 'template' ? 'tab-item--active' : ''}"
      onclick={() => switchTab('template')}
    >
      <i class="ri-magic-line" aria-hidden="true"></i>
      <span>One-Click Install</span>
    </button>
    <button
      class="tab-item {activeTab === 'cloud_image' ? 'tab-item--active' : ''}"
      onclick={() => switchTab('cloud_image')}
    >
      <i class="ri-cloud-line" aria-hidden="true"></i>
      <span>Cloud image</span>
    </button>
    <button
      class="tab-item {activeTab === 'local_image' ? 'tab-item--active' : ''}"
      onclick={() => switchTab('local_image')}
    >
      <i class="ri-hard-drive-2-line" aria-hidden="true"></i>
      <span>Local image</span>
      {#if status?.images_total}
        <span class="tab-count">{status.images_total}</span>
      {/if}
    </button>
    <button
      class="tab-item {activeTab === 'compose' ? 'tab-item--active' : ''}"
      onclick={() => switchTab('compose')}
    >
      <i class="ri-git-merge-line" aria-hidden="true"></i>
      <span>Docker Compose</span>
    </button>
    <button
      class="tab-item {activeTab === 'network' ? 'tab-item--active' : ''}"
      onclick={() => switchTab('network')}
    >
      <i class="ri-global-line" aria-hidden="true"></i>
      <span>Network</span>
      {#if status?.networks_total}
        <span class="tab-count">{status.networks_total}</span>
      {/if}
    </button>
    <button
      class="tab-item {activeTab === 'volume' ? 'tab-item--active' : ''}"
      onclick={() => switchTab('volume')}
    >
      <i class="ri-database-line" aria-hidden="true"></i>
      <span>Volume</span>
      {#if status?.volumes_total}
        <span class="tab-count">{status.volumes_total}</span>
      {/if}
    </button>
    <button
      class="tab-item {activeTab === 'repository' ? 'tab-item--active' : ''}"
      onclick={() => switchTab('repository')}
    >
      <i class="ri-shield-keyhole-line" aria-hidden="true"></i>
      <span>Repository</span>
    </button>
    <button
      class="tab-item {activeTab === 'settings' ? 'tab-item--active' : ''}"
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
      <div class="overview-grid">
        <!-- Statistik Kartu Metrik -->
        <div class="stats-row">
          <div class="stat-card">
            <div class="stat-icon stat-icon--active">
              <i class="ri-play-circle-line" aria-hidden="true"></i>
            </div>
            <div class="stat-meta">
              <span class="stat-val">{status?.containers_running || 0}</span>
              <span class="stat-lbl">Running Containers</span>
            </div>
          </div>

          <div class="stat-card">
            <div class="stat-icon stat-icon--danger">
              <i class="ri-stop-circle-line" aria-hidden="true"></i>
            </div>
            <div class="stat-meta">
              <span class="stat-val">{status?.containers_stopped || 0}</span>
              <span class="stat-lbl">Stopped Containers</span>
            </div>
          </div>

          <div class="stat-card">
            <div class="stat-icon stat-icon--info">
              <i class="ri-hard-drive-line" aria-hidden="true"></i>
            </div>
            <div class="stat-meta">
              <span class="stat-val">{status?.images_total || 0}</span>
              <span class="stat-lbl">Local Images</span>
            </div>
          </div>

          <div class="stat-card">
            <div class="stat-icon stat-icon--violet">
              <i class="ri-database-2-line" aria-hidden="true"></i>
            </div>
            <div class="stat-meta">
              <span class="stat-val">{status?.volumes_total || 0}</span>
              <span class="stat-lbl">Mounted Volumes</span>
            </div>
          </div>
        </div>

        <!-- Quick Actions & Pintasan Cepat -->
        <div class="card quick-actions-card">
          <div class="card__head">
            <h2 class="card-title">
              <i class="ri-flashlight-line" aria-hidden="true"></i>
              <span>Pintasan Operasi Cepat</span>
            </h2>
          </div>
          <div class="card__body shortcuts-bar">
            <button class="btn btn--primary btn--sm" onclick={() => (showCreateContainerModal = true)}>
              <i class="ri-add-line" aria-hidden="true"></i>
              <span>Buat Kontainer</span>
            </button>
            <button class="btn btn--secondary btn--sm" onclick={() => (showPullImageModal = true)}>
              <i class="ri-download-line" aria-hidden="true"></i>
              <span>Tarik Citra (Pull)</span>
            </button>
            <button class="btn btn--secondary btn--sm" onclick={() => switchTab('template')}>
              <i class="ri-magic-line" aria-hidden="true"></i>
              <span>Aplikasi 1-Klik</span>
            </button>
            <button class="btn btn--secondary btn--sm" onclick={() => (showComposeModal = true)}>
              <i class="ri-file-code-line" aria-hidden="true"></i>
              <span>Stack Compose Baru</span>
            </button>
            <button class="btn btn--danger btn--sm" onclick={pruneVolumes}>
              <i class="ri-delete-bin-line" aria-hidden="true"></i>
              <span>Bersihkan Volume Usang</span>
            </button>
          </div>
        </div>

        <!-- Host System Specs Table (Mirip aaPanel bottom table) -->
        <div class="card system-specs-card">
          <div class="card__head">
            <h2 class="card-title">
              <i class="ri-server-line" aria-hidden="true"></i>
              <span>Informasi Arsitektur & Spesifikasi Host</span>
            </h2>
          </div>
          <div class="card__body p-0">
            <div class="specs-grid">
              <div class="spec-cell">
                <span class="spec-k">Hostname</span>
                <span class="spec-v">{systemInfo?.hostname || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">System Version</span>
                <span class="spec-v">{systemInfo?.os_version || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Architecture</span>
                <span class="spec-v">{systemInfo?.architecture || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Kernel Version</span>
                <span class="spec-v">{systemInfo?.kernel_version || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">CPU Core</span>
                <span class="spec-v">{systemInfo?.cpu_cores || 1} Cores</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Memory Size</span>
                <span class="spec-v">{systemInfo?.memory_size || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Docker Version</span>
                <span class="spec-v">{systemInfo?.docker_version || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Docker Compose</span>
                <span class="spec-v">{systemInfo?.compose_version || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Unix Address</span>
                <span class="spec-v font-mono">{systemInfo?.unix_address || 'unix:///var/run/docker.sock'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Data Directory</span>
                <span class="spec-v font-mono">{systemInfo?.data_directory || '/var/lib/docker'}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    {/if}

    <!-- ══════════════ TAB 2: CONTAINER ══════════════ -->
    {#if activeTab === 'container'}
      <div class="tab-pane">
        <!-- Filter Toolbar -->
        <div class="toolbar">
          <div class="toolbar-left">
            <button class="btn btn--primary btn--sm" onclick={() => (showCreateContainerModal = true)}>
              <i class="ri-add-line" aria-hidden="true"></i>
              <span>Buat Kontainer</span>
            </button>

            <div class="filter-pills" role="radiogroup" aria-label="Filter status kontainer">
              <button
                class="pill {containerFilter === 'all' ? 'pill--active' : ''}"
                onclick={() => (containerFilter = 'all')}
              >
                Semua ({containers.length})
              </button>
              <button
                class="pill {containerFilter === 'running' ? 'pill--active' : ''}"
                onclick={() => (containerFilter = 'running')}
              >
                Berjalan ({containers.filter((c) => c.state === 'running').length})
              </button>
              <button
                class="pill {containerFilter === 'stopped' ? 'pill--active' : ''}"
                onclick={() => (containerFilter = 'stopped')}
              >
                Berhenti ({containers.filter((c) => c.state === 'exited' || c.state === 'stopped').length})
              </button>
              <button
                class="pill {containerFilter === 'paused' ? 'pill--active' : ''}"
                onclick={() => (containerFilter = 'paused')}
              >
                Ditangguhkan ({containers.filter((c) => c.state === 'paused').length})
              </button>
            </div>
          </div>

          <div class="toolbar-right">
            <div class="search-box">
              <i class="ri-search-line search-icon" aria-hidden="true"></i>
              <input
                type="text"
                class="input input--sm"
                placeholder="Cari nama, citra, atau ID..."
                bind:value={containerSearch}
              />
            </div>
            <button
              class="btn btn--secondary btn--sm btn--icon"
              title="Segarkan daftar kontainer"
              aria-label="Segarkan daftar kontainer"
              onclick={loadContainers}
            >
              <i class="ri-refresh-line {loadingContainers ? 'spin' : ''}" aria-hidden="true"></i>
            </button>
          </div>
        </div>

        <!-- Tabel Kontainer -->
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th style="width: 140px;">Status</th>
                <th>Nama Kontainer</th>
                <th>Citra</th>
                <th>Port Mapping</th>
                <th style="width: 120px;">CPU / Mem</th>
                <th style="width: 140px;">Dibuat</th>
                <th style="width: 200px; text-align: right;">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {#if loadingContainers}
                <tr>
                  <td colspan="7" class="text-center py-6">
                    <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
                    <p class="text-muted mt-2">Memuat daftar kontainer...</p>
                  </td>
                </tr>
              {:else if filteredContainers.length === 0}
                <tr>
                  <td colspan="7" class="text-center py-8">
                    <div class="empty-state">
                      <i class="ri-box-3-line empty-icon" aria-hidden="true"></i>
                      <p class="empty-title">Tidak ada kontainer</p>
                      <p class="empty-desc">Buat kontainer baru atau jalankan template aplikasi 1-klik untuk memulai.</p>
                      <button class="btn btn--primary btn--sm mt-3" onclick={() => (showCreateContainerModal = true)}>
                        <i class="ri-add-line" aria-hidden="true"></i>
                        <span>Tambah Kontainer Baru</span>
                      </button>
                    </div>
                  </td>
                </tr>
              {:else}
                {#each filteredContainers as c}
                  <tr>
                    <td>
                      {#if c.state === 'running'}
                        <span class="badge badge--success">
                          <span class="status-dot status-dot--active"></span>
                          Running
                        </span>
                      {:else if c.state === 'paused'}
                        <span class="badge badge--warning">
                          <span class="status-dot status-dot--warning"></span>
                          Paused
                        </span>
                      {:else}
                        <span class="badge badge--danger">
                          <span class="status-dot status-dot--danger"></span>
                          Exited
                        </span>
                      {/if}
                    </td>
                    <td>
                      <div class="cell-primary font-semibold">
                        {c.names[0]?.replace(/^\//, '') || c.id.slice(0, 12)}
                      </div>
                      <div class="cell-sub font-mono text-xs">{c.id.slice(0, 12)}</div>
                    </td>
                    <td>
                      <span class="badge badge--secondary font-mono text-xs">{c.image}</span>
                    </td>
                    <td>
                      {#if c.ports && c.ports.length > 0}
                        <div class="ports-list">
                          {#each c.ports as p}
                            {#if p.public_port > 0}
                              <a
                                href="http://localhost:{p.public_port}"
                                target="_blank"
                                rel="noopener noreferrer"
                                class="port-chip port-chip--link"
                                title="Buka http://localhost:{p.public_port}"
                              >
                                <i class="ri-external-link-line" aria-hidden="true"></i>
                                {p.public_port}:{p.private_port}
                              </a>
                            {:else}
                              <span class="port-chip">{p.private_port}/{p.type}</span>
                            {/if}
                          {/each}
                        </div>
                      {:else}
                        <span class="text-muted text-xs">-</span>
                      {/if}
                    </td>
                    <td>
                      <div class="text-xs font-mono">{c.cpu_percent.toFixed(1)}% CPU</div>
                      <div class="text-xs text-muted font-mono">{c.mem_usage || '0 MB'}</div>
                    </td>
                    <td>
                      <span class="text-xs text-muted">{new Date(c.created * 1000).toLocaleDateString()}</span>
                    </td>
                    <td style="text-align: right;">
                      <div class="action-btns-row">
                        {#if c.state === 'running'}
                          <button
                            class="btn btn--ghost btn--sm btn--icon"
                            title="Hentikan (Stop)"
                            aria-label="Hentikan kontainer"
                            disabled={actionLoading}
                            onclick={() => controlContainer(c.id, 'stop')}
                          >
                            <i class="ri-stop-fill text-warning" aria-hidden="true"></i>
                          </button>
                          <button
                            class="btn btn--ghost btn--sm btn--icon"
                            title="Muat Ulang (Restart)"
                            aria-label="Muat ulang kontainer"
                            disabled={actionLoading}
                            onclick={() => controlContainer(c.id, 'restart')}
                          >
                            <i class="ri-restart-line text-info" aria-hidden="true"></i>
                          </button>
                        {:else}
                          <button
                            class="btn btn--ghost btn--sm btn--icon"
                            title="Mulai (Start)"
                            aria-label="Mulai kontainer"
                            disabled={actionLoading}
                            onclick={() => controlContainer(c.id, 'start')}
                          >
                            <i class="ri-play-fill text-accent" aria-hidden="true"></i>
                          </button>
                        {/if}

                        <button
                          class="btn btn--ghost btn--sm btn--icon"
                          title="Lihat Log Kontainer"
                          aria-label="Lihat log kontainer"
                          onclick={() => openContainerLogs(c)}
                        >
                          <i class="ri-file-list-3-line" aria-hidden="true"></i>
                        </button>

                        <button
                          class="btn btn--ghost btn--sm btn--icon"
                          title="Hapus Kontainer"
                          aria-label="Hapus kontainer"
                          disabled={actionLoading}
                          onclick={() => controlContainer(c.id, 'delete')}
                        >
                          <i class="ri-delete-bin-6-line text-danger" aria-hidden="true"></i>
                        </button>
                      </div>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    {/if}

    <!-- ══════════════ TAB 3: ONE-CLICK INSTALL ══════════════ -->
    {#if activeTab === 'template'}
      <div class="tab-pane">
        <div class="template-header">
          <div class="category-pills">
            <button
              class="pill {templateCategory === 'all' ? 'pill--active' : ''}"
              onclick={() => (templateCategory = 'all')}
            >
              Semua Template
            </button>
            <button
              class="pill {templateCategory === 'database' ? 'pill--active' : ''}"
              onclick={() => (templateCategory = 'database')}
            >
              Database & Cache
            </button>
            <button
              class="pill {templateCategory === 'cms' ? 'pill--active' : ''}"
              onclick={() => (templateCategory = 'cms')}
            >
              CMS & Web App
            </button>
            <button
              class="pill {templateCategory === 'tools' ? 'pill--active' : ''}"
              onclick={() => (templateCategory = 'tools')}
            >
              Management Tools
            </button>
            <button
              class="pill {templateCategory === 'monitoring' ? 'pill--active' : ''}"
              onclick={() => (templateCategory = 'monitoring')}
            >
              Monitoring
            </button>
          </div>
        </div>

        {#if loadingTemplates}
          <div class="text-center py-10">
            <i class="ri-loader-4-line spin text-muted text-2xl" aria-hidden="true"></i>
            <p class="text-muted mt-2">Memuat katalog aplikasi...</p>
          </div>
        {:else}
          <div class="templates-grid">
            {#each filteredTemplates as tmpl}
              <div class="template-card card">
                <div class="template-card-top">
                  <div class="template-icon">
                    <i class={tmpl.icon} aria-hidden="true"></i>
                  </div>
                  <div class="template-info">
                    <h3 class="template-name">{tmpl.name}</h3>
                    <span class="badge badge--secondary text-2xs">{tmpl.category}</span>
                  </div>
                </div>

                <p class="template-desc">{tmpl.description}</p>

                <div class="template-meta">
                  <span class="text-xs text-muted">
                    <i class="ri-links-line" aria-hidden="true"></i>
                    Port default: <strong class="font-mono">{tmpl.default_port}</strong>
                  </span>
                  <span class="text-xs text-muted font-mono">{tmpl.image}</span>
                </div>

                <div class="template-actions">
                  <button class="btn btn--primary btn--sm w-full" onclick={() => openDeployTemplate(tmpl)}>
                    <i class="ri-install-line" aria-hidden="true"></i>
                    <span>Pasang Sekarang</span>
                  </button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    <!-- ══════════════ TAB 4: CLOUD IMAGE (DOCKER HUB) ══════════════ -->
    {#if activeTab === 'cloud_image'}
      <div class="tab-pane">
        <div class="card cloud-search-card">
          <div class="card__head">
            <h2 class="card-title">
              <i class="ri-cloud-line" aria-hidden="true"></i>
              <span>Cari Citra Resmi di Docker Hub Registry</span>
            </h2>
          </div>
          <div class="card__body">
            <form
              class="search-form-row"
              onsubmit={(e) => {
                e.preventDefault();
                searchCloudImages();
              }}
            >
              <div class="field flex-1 mb-0">
                <input
                  type="text"
                  class="input"
                  placeholder="Ketik nama citra (misal: nginx, redis, node, python, golang, wordpress)..."
                  bind:value={cloudSearchQuery}
                />
              </div>
              <button class="btn btn--primary" type="submit" disabled={cloudSearching}>
                <i class="ri-search-2-line {cloudSearching ? 'spin' : ''}" aria-hidden="true"></i>
                <span>Cari di Hub</span>
              </button>
            </form>
          </div>
        </div>

        {#if cloudResults.length > 0}
          <div class="table-wrap mt-4">
            <table class="table">
              <thead>
                <tr>
                  <th>Citra Docker Hub</th>
                  <th>Deskripsi</th>
                  <th style="width: 100px;">Bintang</th>
                  <th style="width: 120px;">Lencana</th>
                  <th style="width: 120px; text-align: right;">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {#each cloudResults as r}
                  <tr>
                    <td>
                      <div class="cell-primary font-semibold font-mono">{r.name}</div>
                    </td>
                    <td>
                      <p class="text-xs text-secondary line-clamp-2">{r.description || 'Tanpa deskripsi'}</p>
                    </td>
                    <td>
                      <span class="badge badge--secondary">
                        <i class="ri-star-fill text-warning" aria-hidden="true"></i>
                        {r.stars}
                      </span>
                    </td>
                    <td>
                      {#if r.is_official}
                        <span class="badge badge--success" title="Citra resmi Docker">Official</span>
                      {:else if r.is_automated}
                        <span class="badge badge--info" title="Build otomatis">Automated</span>
                      {:else}
                        <span class="badge badge--secondary">Community</span>
                      {/if}
                    </td>
                    <td style="text-align: right;">
                      <button
                        class="btn btn--primary btn--sm"
                        onclick={() => openPullModalFor(r.name)}
                      >
                        <i class="ri-download-cloud-line" aria-hidden="true"></i>
                        <span>Tarik (Pull)</span>
                      </button>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    {/if}

    <!-- ══════════════ TAB 5: LOCAL IMAGE ══════════════ -->
    {#if activeTab === 'local_image'}
      <div class="tab-pane">
        <div class="toolbar">
          <div class="toolbar-left">
            <button class="btn btn--primary btn--sm" onclick={() => (showPullImageModal = true)}>
              <i class="ri-download-cloud-line" aria-hidden="true"></i>
              <span>Tarik Citra Baru (Pull)</span>
            </button>
          </div>

          <div class="toolbar-right">
            <div class="search-box">
              <i class="ri-search-line search-icon" aria-hidden="true"></i>
              <input
                type="text"
                class="input input--sm"
                placeholder="Cari repositori atau tag..."
                bind:value={imageSearch}
              />
            </div>
            <button
              class="btn btn--secondary btn--sm btn--icon"
              title="Segarkan citra"
              aria-label="Segarkan citra"
              onclick={loadImages}
            >
              <i class="ri-refresh-line {loadingImages ? 'spin' : ''}" aria-hidden="true"></i>
            </button>
          </div>
        </div>

        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Repositori</th>
                <th>Tag</th>
                <th>Image ID</th>
                <th>Ukuran</th>
                <th>Terpakai</th>
                <th>Dibuat</th>
                <th style="width: 180px; text-align: right;">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {#if loadingImages}
                <tr>
                  <td colspan="7" class="text-center py-6">
                    <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
                    <p class="text-muted mt-2">Memuat daftar citra lokal...</p>
                  </td>
                </tr>
              {:else if images.length === 0}
                <tr>
                  <td colspan="7" class="text-center py-8">
                    <div class="empty-state">
                      <i class="ri-hard-drive-2-line empty-icon" aria-hidden="true"></i>
                      <p class="empty-title">Belum ada citra lokal</p>
                      <p class="empty-desc">Tarik citra dari Docker Hub untuk mulai menggunakannya.</p>
                      <button class="btn btn--primary btn--sm mt-3" onclick={() => (showPullImageModal = true)}>
                        <i class="ri-download-cloud-line" aria-hidden="true"></i>
                        <span>Tarik Citra Sekarang</span>
                      </button>
                    </div>
                  </td>
                </tr>
              {:else}
                {#each images as img}
                  <tr>
                    <td>
                      <strong class="font-mono text-sm">{img.repository}</strong>
                    </td>
                    <td>
                      <span class="badge badge--info font-mono">{img.tag}</span>
                    </td>
                    <td>
                      <span class="font-mono text-xs text-muted">{img.id.slice(0, 12)}</span>
                    </td>
                    <td>
                      <span class="text-xs font-semibold">{img.size_human}</span>
                    </td>
                    <td>
                      {#if img.containers > 0}
                        <span class="badge badge--success">{img.containers} kontainer</span>
                      {:else}
                        <span class="badge badge--secondary">0 (Unused)</span>
                      {/if}
                    </td>
                    <td>
                      <span class="text-xs text-muted">{new Date(img.created * 1000).toLocaleDateString()}</span>
                    </td>
                    <td style="text-align: right;">
                      <div class="action-btns-row">
                        <button
                          class="btn btn--secondary btn--sm"
                          title="Jalankan kontainer dari citra ini"
                          onclick={() => openRunContainerFor(img)}
                        >
                          <i class="ri-play-line" aria-hidden="true"></i>
                          <span>Run</span>
                        </button>
                        <button
                          class="btn btn--ghost btn--sm btn--icon"
                          title="Hapus Citra"
                          aria-label="Hapus Citra"
                          disabled={actionLoading}
                          onclick={() => deleteImage(img)}
                        >
                          <i class="ri-delete-bin-6-line text-danger" aria-hidden="true"></i>
                        </button>
                      </div>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    {/if}

    <!-- ══════════════ TAB 6: DOCKER COMPOSE ══════════════ -->
    {#if activeTab === 'compose'}
      <div class="tab-pane">
        <div class="toolbar">
          <div class="toolbar-left">
            <button class="btn btn--primary btn--sm" onclick={() => (showComposeModal = true)}>
              <i class="ri-add-line" aria-hidden="true"></i>
              <span>Tambah Stack Compose</span>
            </button>
          </div>
          <div class="toolbar-right">
            <button
              class="btn btn--secondary btn--sm btn--icon"
              title="Segarkan Compose"
              aria-label="Segarkan Compose"
              onclick={loadComposeProjects}
            >
              <i class="ri-refresh-line {loadingCompose ? 'spin' : ''}" aria-hidden="true"></i>
            </button>
          </div>
        </div>

        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Nama Proyek</th>
                <th>Status</th>
                <th>Lokasi Berkas</th>
                <th>Kontainer Aktif</th>
                <th style="width: 200px; text-align: right;">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {#if loadingCompose}
                <tr>
                  <td colspan="5" class="text-center py-6">
                    <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
                    <p class="text-muted mt-2">Memuat proyek compose...</p>
                  </td>
                </tr>
              {:else if composeProjects.length === 0}
                <tr>
                  <td colspan="5" class="text-center py-8">
                    <div class="empty-state">
                      <i class="ri-git-merge-line empty-icon" aria-hidden="true"></i>
                      <p class="empty-title">Belum ada proyek Docker Compose</p>
                      <p class="empty-desc">Definisikan stack multi-kontainer menggunakan format YAML standar.</p>
                      <button class="btn btn--primary btn--sm mt-3" onclick={() => (showComposeModal = true)}>
                        <i class="ri-add-line" aria-hidden="true"></i>
                        <span>Buat Stack Baru</span>
                      </button>
                    </div>
                  </td>
                </tr>
              {:else}
                {#each composeProjects as proj}
                  <tr>
                    <td>
                      <strong class="font-semibold text-sm">{proj.name}</strong>
                    </td>
                    <td>
                      {#if proj.status === 'running'}
                        <span class="badge badge--success">
                          <span class="status-dot status-dot--active"></span>
                          Running
                        </span>
                      {:else}
                        <span class="badge badge--secondary">Stopped</span>
                      {/if}
                    </td>
                    <td>
                      <span class="text-xs font-mono text-muted">{proj.config_path}</span>
                    </td>
                    <td>
                      <span class="badge badge--info">{proj.containers_count} service</span>
                    </td>
                    <td style="text-align: right;">
                      <div class="action-btns-row">
                        <button
                          class="btn btn--ghost btn--sm btn--icon"
                          title="Edit YAML Stack"
                          aria-label="Edit YAML Stack"
                          onclick={() => editComposeProject(proj)}
                        >
                          <i class="ri-edit-line" aria-hidden="true"></i>
                        </button>
                        {#if proj.status === 'running'}
                          <button
                            class="btn btn--ghost btn--sm btn--icon"
                            title="Hentikan Stack (Down)"
                            aria-label="Hentikan Stack"
                            disabled={actionLoading}
                            onclick={() => composeDown(proj)}
                          >
                            <i class="ri-stop-circle-line text-warning" aria-hidden="true"></i>
                          </button>
                        {:else}
                          <button
                            class="btn btn--ghost btn--sm btn--icon"
                            title="Jalankan Stack (Up)"
                            aria-label="Jalankan Stack"
                            disabled={actionLoading}
                            onclick={() => {
                              composeForm.name = proj.name;
                              composeForm.yaml = proj.yaml_content;
                              submitComposeUp();
                            }}
                          >
                            <i class="ri-play-circle-line text-accent" aria-hidden="true"></i>
                          </button>
                        {/if}
                      </div>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    {/if}

    <!-- ══════════════ TAB 7: NETWORK ══════════════ -->
    {#if activeTab === 'network'}
      <div class="tab-pane">
        <div class="toolbar">
          <div class="toolbar-left">
            <button class="btn btn--primary btn--sm" onclick={() => (showCreateNetworkModal = true)}>
              <i class="ri-add-line" aria-hidden="true"></i>
              <span>Buat Jaringan Baru</span>
            </button>
          </div>
          <div class="toolbar-right">
            <button
              class="btn btn--secondary btn--sm btn--icon"
              title="Segarkan jaringan"
              aria-label="Segarkan jaringan"
              onclick={loadNetworks}
            >
              <i class="ri-refresh-line {loadingNetworks ? 'spin' : ''}" aria-hidden="true"></i>
            </button>
          </div>
        </div>

        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Nama Network</th>
                <th>Driver</th>
                <th>Scope</th>
                <th>Subnet</th>
                <th>Gateway</th>
                <th>Kontainer Terhubung</th>
                <th style="width: 100px; text-align: right;">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {#if loadingNetworks}
                <tr>
                  <td colspan="7" class="text-center py-6">
                    <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
                    <p class="text-muted mt-2">Memuat daftar jaringan...</p>
                  </td>
                </tr>
              {:else if networks.length === 0}
                <tr>
                  <td colspan="7" class="text-center py-8">
                    <div class="empty-state">
                      <i class="ri-global-line empty-icon" aria-hidden="true"></i>
                      <p class="empty-title">Tidak ada jaringan tambahan</p>
                    </div>
                  </td>
                </tr>
              {:else}
                {#each networks as net}
                  <tr>
                    <td>
                      <strong class="font-mono text-sm">{net.name}</strong>
                    </td>
                    <td>
                      <span class="badge badge--secondary font-mono">{net.driver}</span>
                    </td>
                    <td>
                      <span class="text-xs text-muted">{net.scope}</span>
                    </td>
                    <td>
                      <span class="font-mono text-xs">{net.subnet || '-'}</span>
                    </td>
                    <td>
                      <span class="font-mono text-xs">{net.gateway || '-'}</span>
                    </td>
                    <td>
                      <span class="badge badge--info">{net.containers_count} kontainer</span>
                    </td>
                    <td style="text-align: right;">
                      {#if net.name !== 'bridge' && net.name !== 'host' && net.name !== 'none'}
                        <button
                          class="btn btn--ghost btn--sm btn--icon"
                          title="Hapus Network"
                          aria-label="Hapus Network"
                          disabled={actionLoading}
                          onclick={() => deleteNetwork(net)}
                        >
                          <i class="ri-delete-bin-6-line text-danger" aria-hidden="true"></i>
                        </button>
                      {:else}
                        <span class="text-xs text-muted">Sistem</span>
                      {/if}
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    {/if}

    <!-- ══════════════ TAB 8: VOLUME ══════════════ -->
    {#if activeTab === 'volume'}
      <div class="tab-pane">
        <div class="toolbar">
          <div class="toolbar-left">
            <button class="btn btn--primary btn--sm" onclick={() => (showCreateVolumeModal = true)}>
              <i class="ri-add-line" aria-hidden="true"></i>
              <span>Buat Volume Baru</span>
            </button>
            <button class="btn btn--danger btn--sm" onclick={pruneVolumes}>
              <i class="ri-delete-bin-line" aria-hidden="true"></i>
              <span>Bersihkan Tak Terpakai (Prune)</span>
            </button>
          </div>
          <div class="toolbar-right">
            <button
              class="btn btn--secondary btn--sm btn--icon"
              title="Segarkan volume"
              aria-label="Segarkan volume"
              onclick={loadVolumes}
            >
              <i class="ri-refresh-line {loadingVolumes ? 'spin' : ''}" aria-hidden="true"></i>
            </button>
          </div>
        </div>

        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Nama Volume</th>
                <th>Driver</th>
                <th>Mountpoint</th>
                <th>Ukuran</th>
                <th>Ref Count</th>
                <th style="width: 100px; text-align: right;">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {#if loadingVolumes}
                <tr>
                  <td colspan="6" class="text-center py-6">
                    <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
                    <p class="text-muted mt-2">Memuat daftar volume...</p>
                  </td>
                </tr>
              {:else if volumes.length === 0}
                <tr>
                  <td colspan="6" class="text-center py-8">
                    <div class="empty-state">
                      <i class="ri-database-line empty-icon" aria-hidden="true"></i>
                      <p class="empty-title">Belum ada volume Docker</p>
                    </div>
                  </td>
                </tr>
              {:else}
                {#each volumes as v}
                  <tr>
                    <td>
                      <strong class="font-mono text-sm">{v.name}</strong>
                    </td>
                    <td>
                      <span class="badge badge--secondary font-mono">{v.driver}</span>
                    </td>
                    <td>
                      <span class="text-xs font-mono text-muted">{v.mountpoint}</span>
                    </td>
                    <td>
                      <span class="text-xs font-semibold">{v.size_human}</span>
                    </td>
                    <td>
                      {#if v.ref_count > 0}
                        <span class="badge badge--success">{v.ref_count} terpasang</span>
                      {:else}
                        <span class="badge badge--secondary">0 (Bebas)</span>
                      {/if}
                    </td>
                    <td style="text-align: right;">
                      <button
                        class="btn btn--ghost btn--sm btn--icon"
                        title="Hapus Volume"
                        aria-label="Hapus Volume"
                        disabled={actionLoading}
                        onclick={() => deleteVolume(v)}
                      >
                        <i class="ri-delete-bin-6-line text-danger" aria-hidden="true"></i>
                      </button>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    {/if}

    <!-- ══════════════ TAB 9: REPOSITORY (PRIVATE REGISTRY) ══════════════ -->
    {#if activeTab === 'repository'}
      <div class="tab-pane">
        <div class="toolbar">
          <div class="toolbar-left">
            <button class="btn btn--primary btn--sm" onclick={() => (showAddRegistryModal = true)}>
              <i class="ri-add-line" aria-hidden="true"></i>
              <span>Tambah Registri Privat</span>
            </button>
          </div>
          <div class="toolbar-right">
            <button
              class="btn btn--secondary btn--sm btn--icon"
              title="Segarkan registri"
              aria-label="Segarkan registri"
              onclick={loadRegistries}
            >
              <i class="ri-refresh-line {loadingRegistries ? 'spin' : ''}" aria-hidden="true"></i>
            </button>
          </div>
        </div>

        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Nama Registri</th>
                <th>URL Host</th>
                <th>Username</th>
                <th>Status Default</th>
                <th style="width: 100px; text-align: right;">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {#if loadingRegistries}
                <tr>
                  <td colspan="5" class="text-center py-6">
                    <i class="ri-loader-4-line spin text-muted text-xl" aria-hidden="true"></i>
                    <p class="text-muted mt-2">Memuat daftar registri...</p>
                  </td>
                </tr>
              {:else if registries.length === 0}
                <tr>
                  <td colspan="5" class="text-center py-8">
                    <div class="empty-state">
                      <i class="ri-shield-keyhole-line empty-icon" aria-hidden="true"></i>
                      <p class="empty-title">Tidak ada registri privat terdaftar</p>
                      <p class="empty-desc">Docker Hub resmi digunakan secara default.</p>
                    </div>
                  </td>
                </tr>
              {:else}
                {#each registries as reg}
                  <tr>
                    <td>
                      <strong class="text-sm font-semibold">{reg.name}</strong>
                    </td>
                    <td>
                      <span class="font-mono text-xs text-muted">{reg.url}</span>
                    </td>
                    <td>
                      <span class="text-xs">{reg.username || '-'}</span>
                    </td>
                    <td>
                      {#if reg.is_default}
                        <span class="badge badge--success">Default</span>
                      {:else}
                        <span class="badge badge--secondary">Alternatif</span>
                      {/if}
                    </td>
                    <td style="text-align: right;">
                      {#if !reg.is_default}
                        <button
                          class="btn btn--ghost btn--sm btn--icon"
                          title="Hapus Registri"
                          aria-label="Hapus Registri"
                          disabled={actionLoading}
                          onclick={() => deleteRegistry(reg)}
                        >
                          <i class="ri-delete-bin-6-line text-danger" aria-hidden="true"></i>
                        </button>
                      {:else}
                        <span class="text-xs text-muted">-</span>
                      {/if}
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    {/if}

    <!-- ══════════════ TAB 10: SETTINGS (PERSIS TAMPILAN SCREENSHOT AAPANEL) ══════════════ -->
    {#if activeTab === 'settings'}
      <div class="tab-pane settings-suite">
        <div class="card settings-card">
          <div class="card__body settings-rows">
            <!-- 1. Service Status -->
            <div class="setting-row">
              <div class="setting-label">
                <span>Service status</span>
              </div>
              <div class="setting-ctrl">
                <div class="status-indicator-group">
                  {#if status?.running}
                    <span class="badge badge--success">
                      <span class="status-dot status-dot--active"></span>
                      Running
                    </span>
                  {:else}
                    <span class="badge badge--danger">
                      <span class="status-dot status-dot--danger"></span>
                      Stopped
                    </span>
                  {/if}

                  <button
                    class="btn btn--secondary btn--sm"
                    disabled={actionLoading}
                    onclick={() => controlDockerService('start')}
                  >
                    Start up
                  </button>
                  <button
                    class="btn btn--secondary btn--sm"
                    disabled={actionLoading}
                    onclick={() => controlDockerService('restart')}
                  >
                    Restart
                  </button>
                  <button
                    class="btn btn--secondary btn--sm"
                    disabled={actionLoading}
                    onclick={() => controlDockerService('repair')}
                  >
                    Repair
                  </button>
                </div>
              </div>
            </div>

            <!-- 2. Accelerate URL (Registry Mirrors) -->
            <div class="setting-row">
              <div class="setting-label">
                <span>Accelerate URL</span>
              </div>
              <div class="setting-ctrl">
                <div class="mirror-box">
                  <textarea
                    class="input font-mono text-xs"
                    rows="3"
                    placeholder="https://hub-mirror.c.163.com&#10;https://mirror.baidubce.com&#10;Satu URL per baris..."
                    value={daemonConfig?.registry_mirrors?.join('\n') || ''}
                    onchange={(e) => {
                      if (!daemonConfig) return;
                      daemonConfig.registry_mirrors = e.currentTarget.value
                        .split('\n')
                        .map((s) => s.trim())
                        .filter(Boolean);
                    }}
                  ></textarea>
                  <p class="setting-hint">Accelerate URL, one URL per line</p>
                  <button
                    class="btn btn--primary btn--sm mt-2"
                    disabled={savingConfig}
                    onclick={saveDaemonConfig}
                  >
                    Simpan Accelerate URL
                  </button>
                </div>
              </div>
            </div>

            <!-- 3. Docker Compose Path -->
            <div class="setting-row">
              <div class="setting-label">
                <span>Docker compose path</span>
              </div>
              <div class="setting-ctrl">
                <div class="flex-row gap-2">
                  <input
                    type="text"
                    class="input input--sm flex-1 font-mono"
                    value={daemonConfig?.compose_path || '/usr/local/bin/docker-compose'}
                    onchange={(e) => {
                      if (daemonConfig) daemonConfig.compose_path = e.currentTarget.value.trim();
                    }}
                  />
                  <button class="btn btn--secondary btn--sm" onclick={saveDaemonConfig}>
                    Save
                  </button>
                </div>
              </div>
            </div>

            <!-- 4. Private Registry Default -->
            <div class="setting-row">
              <div class="setting-label">
                <span>Private Registry</span>
              </div>
              <div class="setting-ctrl">
                <div class="flex-row gap-2">
                  <select
                    class="input input--sm flex-1"
                    value={daemonConfig?.private_registry || 'official'}
                    onchange={(e) => {
                      if (daemonConfig) daemonConfig.private_registry = e.currentTarget.value;
                      saveDaemonConfig();
                    }}
                  >
                    <option value="official">Docker Hub Official (Default)</option>
                    {#each registries as r}
                      <option value={r.url}>{r.name} ({r.url})</option>
                    {/each}
                  </select>
                </div>
              </div>
            </div>

            <!-- 5. Log Rotation -->
            <div class="setting-row">
              <div class="setting-label">
                <span>Log Rotation</span>
              </div>
              <div class="setting-ctrl flex-row items-center gap-4">
                <label class="switch">
                  <input
                    type="checkbox"
                    checked={daemonConfig?.log_rotation ?? true}
                    onchange={(e) => {
                      if (daemonConfig) {
                        daemonConfig.log_rotation = e.currentTarget.checked;
                        saveDaemonConfig();
                      }
                    }}
                  />
                  <span class="slider"></span>
                </label>
                <button
                  class="btn btn--secondary btn--sm"
                  onclick={() => (showLogRotationModal = true)}
                >
                  Modify
                </button>
                <span class="text-xs text-muted font-mono">
                  max-size: {daemonConfig?.log_max_size || '10m'}, max-file: {daemonConfig?.log_max_file || '3'}
                </span>
              </div>
            </div>

            <!-- 6. IPv6 -->
            <div class="setting-row">
              <div class="setting-label">
                <span>IPv6</span>
              </div>
              <div class="setting-ctrl flex-row items-center gap-4">
                <label class="switch">
                  <input
                    type="checkbox"
                    checked={daemonConfig?.ipv6 ?? false}
                    onchange={(e) => {
                      if (daemonConfig) {
                        daemonConfig.ipv6 = e.currentTarget.checked;
                        saveDaemonConfig();
                      }
                    }}
                  />
                  <span class="slider"></span>
                </label>
                <span class="text-xs text-muted font-mono">
                  {daemonConfig?.ipv6_subnet || 'fd00::/80'}
                </span>
              </div>
            </div>

            <!-- 7. iptables -->
            <div class="setting-row">
              <div class="setting-label">
                <span>iptables</span>
              </div>
              <div class="setting-ctrl flex-row items-center gap-4">
                <label class="switch">
                  <input
                    type="checkbox"
                    checked={daemonConfig?.iptables ?? true}
                    onchange={(e) => {
                      if (daemonConfig) {
                        daemonConfig.iptables = e.currentTarget.checked;
                        saveDaemonConfig();
                      }
                    }}
                  />
                  <span class="slider"></span>
                </label>
                <span class="text-xs text-muted">
                  Aktifkan manajemen aturan paket iptables otomatis oleh Docker daemon
                </span>
              </div>
            </div>

            <!-- 8. live_restore -->
            <div class="setting-row">
              <div class="setting-label">
                <span>live_restore</span>
              </div>
              <div class="setting-ctrl flex-row items-center gap-4">
                <label class="switch">
                  <input
                    type="checkbox"
                    checked={daemonConfig?.live_restore ?? true}
                    onchange={(e) => {
                      if (daemonConfig) {
                        daemonConfig.live_restore = e.currentTarget.checked;
                        saveDaemonConfig();
                      }
                    }}
                  />
                  <span class="slider"></span>
                </label>
                <span class="text-xs text-muted">
                  Jaga kontainer tetap berjalan saat daemon Docker sedang restart atau crash
                </span>
              </div>
            </div>

            <!-- 9. Configuration file -->
            <div class="setting-row">
              <div class="setting-label">
                <span>Configuration file</span>
              </div>
              <div class="setting-ctrl">
                <button
                  class="btn btn--secondary btn--sm"
                  onclick={() => {
                    rawJsonConfig = daemonConfig?.raw_json || '{\n}';
                    showConfigModal = true;
                  }}
                >
                  <i class="ri-file-code-line" aria-hidden="true"></i>
                  <span>Open configuration file</span>
                </button>
              </div>
            </div>

            <!-- 10. One-Click Install Dir -->
            <div class="setting-row">
              <div class="setting-label">
                <span>One-Click Install Dir</span>
              </div>
              <div class="setting-ctrl">
                <div class="flex-row gap-2">
                  <input
                    type="text"
                    class="input input--sm flex-1 font-mono"
                    value={daemonConfig?.one_click_dir || '/www/dk_project'}
                    onchange={(e) => {
                      if (daemonConfig) daemonConfig.one_click_dir = e.currentTarget.value.trim();
                    }}
                  />
                  <button class="btn btn--secondary btn--sm" onclick={saveDaemonConfig}>
                    Save
                  </button>
                </div>
              </div>
            </div>

            <!-- 11. Uninstall Docker -->
            <div class="setting-row border-top-danger">
              <div class="setting-label">
                <span class="text-danger font-semibold">Uninstall Docker</span>
              </div>
              <div class="setting-ctrl">
                <button
                  class="btn btn--danger btn--sm"
                  disabled={actionLoading}
                  onclick={uninstallDocker}
                >
                  <i class="ri-delete-bin-fill" aria-hidden="true"></i>
                  <span>Uninstall Docker</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Tabel Spesifikasi Bawah Settings (Sesuai Screenshot aaPanel) -->
        <div class="card system-specs-card mt-4">
          <div class="card__head">
            <h2 class="card-title">
              <i class="ri-server-line" aria-hidden="true"></i>
              <span>Hardware & Environment Parameters</span>
            </h2>
          </div>
          <div class="card__body p-0">
            <div class="specs-grid">
              <div class="spec-cell">
                <span class="spec-k">Hostname</span>
                <span class="spec-v">{systemInfo?.hostname || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">System Version</span>
                <span class="spec-v">{systemInfo?.os_version || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Architecture</span>
                <span class="spec-v">{systemInfo?.architecture || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Kernel Version</span>
                <span class="spec-v">{systemInfo?.kernel_version || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">CPU Core</span>
                <span class="spec-v">{systemInfo?.cpu_cores || 1} Cores</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Memory Size</span>
                <span class="spec-v">{systemInfo?.memory_size || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Docker Version</span>
                <span class="spec-v">{systemInfo?.docker_version || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Docker Compose</span>
                <span class="spec-v">{systemInfo?.compose_version || '-'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Unix Address</span>
                <span class="spec-v font-mono">{systemInfo?.unix_address || 'unix:///var/run/docker.sock'}</span>
              </div>
              <div class="spec-cell">
                <span class="spec-k">Data Directory</span>
                <span class="spec-v font-mono">{systemInfo?.data_directory || '/var/lib/docker'}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    {/if}
  </main>

  <!-- ══════════════ MODAL: BUAT KONTAINER BARU ══════════════ -->
  {#if showCreateContainerModal}
    <div class="modal-backdrop" onclick={() => (showCreateContainerModal = false)} role="presentation">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div class="modal modal--lg" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="modal__head">
          <h2 class="modal__title">
            <i class="ri-add-box-line" aria-hidden="true"></i>
            <span>Buat Kontainer Baru</span>
          </h2>
          <button
            class="modal__close btn btn--ghost btn--sm btn--icon"
            aria-label="Tutup dialog"
            title="Tutup dialog"
            onclick={() => (showCreateContainerModal = false)}
          >
            <i class="ri-close-line" aria-hidden="true"></i>
          </button>
        </div>

        <form class="modal__body" onsubmit={(e) => { e.preventDefault(); submitCreateContainer(); }}>
          <div class="form-row">
            <div class="field flex-1">
              <label class="field__label" for="cnt-name">Nama Kontainer *</label>
              <input
                id="cnt-name"
                type="text"
                class="input"
                placeholder="misal: my-nginx-app"
                bind:value={newContainer.name}
                required
              />
            </div>
            <div class="field flex-1">
              <label class="field__label" for="cnt-image">Nama Citra (Image) *</label>
              <input
                id="cnt-image"
                type="text"
                class="input font-mono"
                placeholder="misal: nginx:alpine atau redis:7"
                bind:value={newContainer.image}
                required
              />
            </div>
          </div>

          <!-- Port Mapping -->
          <div class="form-section">
            <div class="form-section-head">
              <span class="font-semibold text-sm">Pemetaan Port (Host : Container)</span>
              <button type="button" class="btn btn--secondary btn--sm" onclick={addPortMapping}>
                <i class="ri-add-line" aria-hidden="true"></i>
                <span>Tambah Port</span>
              </button>
            </div>
            {#each newContainer.ports as p, idx}
              <div class="mapping-row">
                <input
                  type="text"
                  class="input input--sm flex-1 font-mono"
                  placeholder="Port Host (misal: 8080)"
                  bind:value={p.host}
                />
                <span class="text-muted">:</span>
                <input
                  type="text"
                  class="input input--sm flex-1 font-mono"
                  placeholder="Port Kontainer (misal: 80)"
                  bind:value={p.container}
                />
                <select class="input input--sm w-24" bind:value={p.protocol}>
                  <option value="tcp">TCP</option>
                  <option value="udp">UDP</option>
                </select>
                <button
                  type="button"
                  class="btn btn--ghost btn--sm btn--icon"
                  title="Hapus baris port"
                  aria-label="Hapus baris port"
                  onclick={() => removePortMapping(idx)}
                >
                  <i class="ri-delete-bin-line text-danger" aria-hidden="true"></i>
                </button>
              </div>
            {/each}
          </div>

          <!-- Volume Mapping -->
          <div class="form-section">
            <div class="form-section-head">
              <span class="font-semibold text-sm">Pemetaan Volume / Direktori Host</span>
              <button type="button" class="btn btn--secondary btn--sm" onclick={addVolumeMapping}>
                <i class="ri-add-line" aria-hidden="true"></i>
                <span>Tambah Mount</span>
              </button>
            </div>
            {#each newContainer.volumes as v, idx}
              <div class="mapping-row">
                <input
                  type="text"
                  class="input input--sm flex-1 font-mono"
                  placeholder="Path Host (misal: /www/data/html)"
                  bind:value={v.host}
                />
                <span class="text-muted">:</span>
                <input
                  type="text"
                  class="input input--sm flex-1 font-mono"
                  placeholder="Path Kontainer (misal: /usr/share/nginx/html)"
                  bind:value={v.container}
                />
                <button
                  type="button"
                  class="btn btn--ghost btn--sm btn--icon"
                  title="Hapus mount"
                  aria-label="Hapus mount"
                  onclick={() => removeVolumeMapping(idx)}
                >
                  <i class="ri-delete-bin-line text-danger" aria-hidden="true"></i>
                </button>
              </div>
            {/each}
          </div>

          <!-- Environment Variables -->
          <div class="form-section">
            <div class="form-section-head">
              <span class="font-semibold text-sm">Environment Variables</span>
              <button type="button" class="btn btn--secondary btn--sm" onclick={addEnvParam}>
                <i class="ri-add-line" aria-hidden="true"></i>
                <span>Tambah Env</span>
              </button>
            </div>
            {#each newContainer.envs as e, idx}
              <div class="mapping-row">
                <input
                  type="text"
                  class="input input--sm flex-1 font-mono"
                  placeholder="KEY (misal: NODE_ENV)"
                  bind:value={e.key}
                />
                <span class="text-muted">=</span>
                <input
                  type="text"
                  class="input input--sm flex-1 font-mono"
                  placeholder="VALUE (misal: production)"
                  bind:value={e.value}
                />
                <button
                  type="button"
                  class="btn btn--ghost btn--sm btn--icon"
                  title="Hapus env"
                  aria-label="Hapus env"
                  onclick={() => removeEnvParam(idx)}
                >
                  <i class="ri-delete-bin-line text-danger" aria-hidden="true"></i>
                </button>
              </div>
            {/each}
          </div>

          <!-- Restart Policy -->
          <div class="form-row">
            <div class="field flex-1">
              <label class="field__label" for="cnt-restart">Kebijakan Restart</label>
              <select id="cnt-restart" class="input" bind:value={newContainer.restart_policy}>
                <option value="unless-stopped">Unless Stopped (Direkomendasikan)</option>
                <option value="always">Always Restart</option>
                <option value="on-failure">On Failure Only</option>
                <option value="no">Do Not Restart (No)</option>
              </select>
            </div>
            <div class="field flex-1">
              <label class="field__label" for="cnt-cmd">Custom Command (Opsional)</label>
              <input
                id="cnt-cmd"
                type="text"
                class="input font-mono"
                placeholder="misal: npm start atau redis-server --appendonly yes"
                bind:value={newContainer.cmd}
              />
            </div>
          </div>

          <div class="modal__foot">
            <button
              type="button"
              class="btn btn--secondary"
              onclick={() => (showCreateContainerModal = false)}
            >
              Batal
            </button>
            <button type="submit" class="btn btn--primary" disabled={actionLoading}>
              <i class="ri-check-line" aria-hidden="true"></i>
              <span>Buat & Jalankan Kontainer</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ══════════════ MODAL: TARIK CITRA (PULL IMAGE) ══════════════ -->
  {#if showPullImageModal}
    <div class="modal-backdrop" onclick={() => (showPullImageModal = false)} role="presentation">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div class="modal" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="modal__head">
          <h2 class="modal__title">
            <i class="ri-download-cloud-line" aria-hidden="true"></i>
            <span>Tarik Citra (Pull Image)</span>
          </h2>
          <button
            class="modal__close btn btn--ghost btn--sm btn--icon"
            aria-label="Tutup dialog"
            title="Tutup dialog"
            onclick={() => (showPullImageModal = false)}
          >
            <i class="ri-close-line" aria-hidden="true"></i>
          </button>
        </div>

        <form class="modal__body" onsubmit={(e) => { e.preventDefault(); submitPullImage(); }}>
          <div class="field">
            <label class="field__label" for="pull-name">Nama Citra Docker *</label>
            <input
              id="pull-name"
              type="text"
              class="input font-mono"
              placeholder="misal: nginx, ubuntu, redis, postgres"
              bind:value={pullImageName}
              required
            />
          </div>

          <div class="field">
            <label class="field__label" for="pull-tag">Tag Versi (Default: latest)</label>
            <input
              id="pull-tag"
              type="text"
              class="input font-mono"
              placeholder="latest, alpine, 18-bookworm, dll."
              bind:value={pullImageTag}
            />
          </div>

          <div class="modal__foot">
            <button type="button" class="btn btn--secondary" onclick={() => (showPullImageModal = false)}>
              Batal
            </button>
            <button type="submit" class="btn btn--primary" disabled={pulling}>
              <i class="ri-download-line {pulling ? 'spin' : ''}" aria-hidden="true"></i>
              <span>{pulling ? 'Sedang Mengunduh...' : 'Mulai Tarik Citra'}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ══════════════ MODAL: LOG KONTAINER ══════════════ -->
  {#if showLogsModal}
    <div class="modal-backdrop" onclick={() => (showLogsModal = false)} role="presentation">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div class="modal modal--lg" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="modal__head">
          <h2 class="modal__title">
            <i class="ri-file-list-3-line" aria-hidden="true"></i>
            <span>Log Kontainer: {selectedContainerLogs.name}</span>
          </h2>
          <div class="flex-row items-center gap-2">
            <select
              class="input input--sm w-28"
              bind:value={selectedContainerLogs.lines}
              onchange={refreshContainerLogs}
            >
              <option value={100}>100 baris</option>
              <option value={200}>200 baris</option>
              <option value={500}>500 baris</option>
              <option value={1000}>1000 baris</option>
            </select>
            <button
              class="btn btn--secondary btn--sm btn--icon"
              title="Segarkan Log"
              aria-label="Segarkan Log"
              onclick={refreshContainerLogs}
            >
              <i class="ri-refresh-line {logsLoading ? 'spin' : ''}" aria-hidden="true"></i>
            </button>
            <button
              class="modal__close btn btn--ghost btn--sm btn--icon"
              aria-label="Tutup log"
              title="Tutup log"
              onclick={() => (showLogsModal = false)}
            >
              <i class="ri-close-line" aria-hidden="true"></i>
            </button>
          </div>
        </div>

        <div class="modal__body p-0">
          <pre class="terminal-log-viewer font-mono text-xs">{selectedContainerLogs.content}</pre>
        </div>

        <div class="modal__foot">
          <span class="text-xs text-muted">ID: {selectedContainerLogs.id.slice(0, 12)}</span>
          <button class="btn btn--secondary btn--sm" onclick={() => (showLogsModal = false)}>
            Tutup
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ══════════════ MODAL: DOCKER COMPOSE EDITOR ══════════════ -->
  {#if showComposeModal}
    <div class="modal-backdrop" onclick={() => (showComposeModal = false)} role="presentation">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div class="modal modal--lg" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="modal__head">
          <h2 class="modal__title">
            <i class="ri-git-merge-line" aria-hidden="true"></i>
            <span>Stack Docker Compose</span>
          </h2>
          <button
            class="modal__close btn btn--ghost btn--sm btn--icon"
            aria-label="Tutup dialog"
            title="Tutup dialog"
            onclick={() => (showComposeModal = false)}
          >
            <i class="ri-close-line" aria-hidden="true"></i>
          </button>
        </div>

        <form class="modal__body" onsubmit={(e) => { e.preventDefault(); submitComposeUp(); }}>
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

          <div class="modal__foot">
            <button type="button" class="btn btn--secondary" onclick={() => (showComposeModal = false)}>
              Batal
            </button>
            <button type="submit" class="btn btn--primary" disabled={actionLoading}>
              <i class="ri-play-line" aria-hidden="true"></i>
              <span>Deploy Stack (Compose Up)</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ══════════════ MODAL: BUAT JARINGAN ══════════════ -->
  {#if showCreateNetworkModal}
    <div class="modal-backdrop" onclick={() => (showCreateNetworkModal = false)} role="presentation">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div class="modal" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="modal__head">
          <h2 class="modal__title">
            <i class="ri-global-line" aria-hidden="true"></i>
            <span>Buat Jaringan Virtual Baru</span>
          </h2>
          <button
            class="modal__close btn btn--ghost btn--sm btn--icon"
            aria-label="Tutup dialog"
            title="Tutup dialog"
            onclick={() => (showCreateNetworkModal = false)}
          >
            <i class="ri-close-line" aria-hidden="true"></i>
          </button>
        </div>

        <form class="modal__body" onsubmit={(e) => { e.preventDefault(); submitCreateNetwork(); }}>
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

          <div class="modal__foot">
            <button type="button" class="btn btn--secondary" onclick={() => (showCreateNetworkModal = false)}>
              Batal
            </button>
            <button type="submit" class="btn btn--primary" disabled={actionLoading}>
              <i class="ri-check-line" aria-hidden="true"></i>
              <span>Buat Jaringan</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ══════════════ MODAL: BUAT VOLUME ══════════════ -->
  {#if showCreateVolumeModal}
    <div class="modal-backdrop" onclick={() => (showCreateVolumeModal = false)} role="presentation">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div class="modal" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="modal__head">
          <h2 class="modal__title">
            <i class="ri-database-line" aria-hidden="true"></i>
            <span>Buat Volume Docker Baru</span>
          </h2>
          <button
            class="modal__close btn btn--ghost btn--sm btn--icon"
            aria-label="Tutup dialog"
            title="Tutup dialog"
            onclick={() => (showCreateVolumeModal = false)}
          >
            <i class="ri-close-line" aria-hidden="true"></i>
          </button>
        </div>

        <form class="modal__body" onsubmit={(e) => { e.preventDefault(); submitCreateVolume(); }}>
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

          <div class="modal__foot">
            <button type="button" class="btn btn--secondary" onclick={() => (showCreateVolumeModal = false)}>
              Batal
            </button>
            <button type="submit" class="btn btn--primary" disabled={actionLoading}>
              <i class="ri-check-line" aria-hidden="true"></i>
              <span>Buat Volume</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ══════════════ MODAL: REGISTRI PRIVAT ══════════════ -->
  {#if showAddRegistryModal}
    <div class="modal-backdrop" onclick={() => (showAddRegistryModal = false)} role="presentation">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div class="modal" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="modal__head">
          <h2 class="modal__title">
            <i class="ri-shield-keyhole-line" aria-hidden="true"></i>
            <span>Tambah Kredensial Registri Privat</span>
          </h2>
          <button
            class="modal__close btn btn--ghost btn--sm btn--icon"
            aria-label="Tutup dialog"
            title="Tutup dialog"
            onclick={() => (showAddRegistryModal = false)}
          >
            <i class="ri-close-line" aria-hidden="true"></i>
          </button>
        </div>

        <form class="modal__body" onsubmit={(e) => { e.preventDefault(); submitAddRegistry(); }}>
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

          <div class="modal__foot">
            <button type="button" class="btn btn--secondary" onclick={() => (showAddRegistryModal = false)}>
              Batal
            </button>
            <button type="submit" class="btn btn--primary" disabled={actionLoading}>
              <i class="ri-check-line" aria-hidden="true"></i>
              <span>Simpan Registri</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ══════════════ MODAL: DEPLOY ONE-CLICK TEMPLATE ══════════════ -->
  {#if showDeployTemplateModal && selectedTemplate}
    <div class="modal-backdrop" onclick={() => (showDeployTemplateModal = false)} role="presentation">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div class="modal modal--lg" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="modal__head">
          <div class="flex-row items-center gap-3">
            <i class="{selectedTemplate.icon} text-xl text-accent" aria-hidden="true"></i>
            <div>
              <h2 class="modal__title">Pasang {selectedTemplate.name}</h2>
              <span class="text-xs text-muted font-mono">{selectedTemplate.image}</span>
            </div>
          </div>
          <button
            class="modal__close btn btn--ghost btn--sm btn--icon"
            aria-label="Tutup dialog"
            title="Tutup dialog"
            onclick={() => (showDeployTemplateModal = false)}
          >
            <i class="ri-close-line" aria-hidden="true"></i>
          </button>
        </div>

        <form class="modal__body" onsubmit={(e) => { e.preventDefault(); submitDeployTemplate(); }}>
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

          <div class="modal__foot">
            <button type="button" class="btn btn--secondary" onclick={() => (showDeployTemplateModal = false)}>
              Batal
            </button>
            <button type="submit" class="btn btn--primary" disabled={actionLoading}>
              <i class="ri-check-line" aria-hidden="true"></i>
              <span>Mulai Pasang & Jalankan</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- ══════════════ MODAL: DAEMON.JSON CONFIGURATION FILE ══════════════ -->
  {#if showConfigModal}
    <div class="modal-backdrop" onclick={() => (showConfigModal = false)} role="presentation">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div class="modal modal--lg" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="modal__head">
          <h2 class="modal__title">
            <i class="ri-file-code-line" aria-hidden="true"></i>
            <span>Configuration file (/etc/docker/daemon.json)</span>
          </h2>
          <button
            class="modal__close btn btn--ghost btn--sm btn--icon"
            aria-label="Tutup dialog"
            title="Tutup dialog"
            onclick={() => (showConfigModal = false)}
          >
            <i class="ri-close-line" aria-hidden="true"></i>
          </button>
        </div>

        <div class="modal__body p-0">
          <textarea
            class="json-editor-textarea font-mono text-xs"
            rows="16"
            bind:value={rawJsonConfig}
          ></textarea>
        </div>

        <div class="modal__foot">
          <button class="btn btn--secondary" onclick={() => (showConfigModal = false)}>
            Batal
          </button>
          <button class="btn btn--primary" disabled={savingConfig} onclick={saveRawJsonConfig}>
            <i class="ri-save-line" aria-hidden="true"></i>
            <span>Simpan & Terapkan Konfigurasi</span>
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ══════════════ MODAL: LOG ROTATION SETTINGS ══════════════ -->
  {#if showLogRotationModal && daemonConfig}
    <div class="modal-backdrop" onclick={() => (showLogRotationModal = false)} role="presentation">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div class="modal" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div class="modal__head">
          <h2 class="modal__title">
            <i class="ri-settings-line" aria-hidden="true"></i>
            <span>Modifikasi Log Rotation</span>
          </h2>
          <button
            class="modal__close btn btn--ghost btn--sm btn--icon"
            aria-label="Tutup dialog"
            title="Tutup dialog"
            onclick={() => (showLogRotationModal = false)}
          >
            <i class="ri-close-line" aria-hidden="true"></i>
          </button>
        </div>

        <div class="modal__body">
          <div class="field">
            <label class="field__label" for="log-max-size">Max Size per File (max-size)</label>
            <input
              id="log-max-size"
              type="text"
              class="input font-mono"
              placeholder="misal: 10m atau 50m"
              bind:value={daemonConfig.log_max_size}
            />
          </div>

          <div class="field">
            <label class="field__label" for="log-max-file">Jumlah File Rotasi (max-file)</label>
            <input
              id="log-max-file"
              type="text"
              class="input font-mono"
              placeholder="misal: 3 atau 5"
              bind:value={daemonConfig.log_max_file}
            />
          </div>
        </div>

        <div class="modal__foot">
          <button class="btn btn--secondary" onclick={() => (showLogRotationModal = false)}>
            Batal
          </button>
          <button class="btn btn--primary" disabled={savingConfig} onclick={saveDaemonConfig}>
            <i class="ri-check-line" aria-hidden="true"></i>
            <span>Simpan Rotasi Log</span>
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  /* ════════════════════════════════════════════════════════════════════════════
     GAYA KHUSUS MODUL DOCKER MANAGEMENT SUITE — 100% DESIGN TOKENS COMPLIANT
     ════════════════════════════════════════════════════════════════════════════ */
  .docker-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    min-height: 100%;
  }

  /* Peringatan Melayang Persis aaPanel */
  .floating-banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    background: var(--tint-warning);
    border: 1px solid color-mix(in srgb, var(--warning) 35%, transparent);
    border-radius: var(--radius-md);
    padding: var(--space-3) var(--space-4);
    box-shadow: var(--shadow-sm);
  }
  .banner-content {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    color: var(--warning-strong);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }
  .banner-icon {
    font-size: 1.25rem;
  }
  .banner-btn {
    flex-shrink: 0;
  }

  /* Header Halaman */
  .page-head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: var(--space-4);
    flex-wrap: wrap;
  }
  .title-cluster {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }
  .page-title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xl);
    font-weight: var(--weight-bold);
    color: var(--text);
    margin: 0;
  }
  .title-icon {
    color: var(--accent);
  }
  .badges-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .status-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: var(--radius-full);
    margin-right: var(--space-1);
  }
  .status-dot--active {
    background: var(--accent);
    box-shadow: 0 0 6px var(--accent);
  }
  .status-dot--danger {
    background: var(--danger);
  }
  .status-dot--warning {
    background: var(--warning);
  }
  .page-subtitle {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin: var(--space-1) 0 0 0;
  }

  /* 10 Tab Navigasi Bar */
  .nav-tabs-bar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-1);
    overflow-x: auto;
    scrollbar-width: none;
  }
  .nav-tabs-bar::-webkit-scrollbar {
    display: none;
  }
  .tab-item {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    border: none;
    background: transparent;
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    white-space: nowrap;
    cursor: pointer;
    transition: background var(--duration-fast) ease, color var(--duration-fast) ease;
  }
  .tab-item:hover {
    color: var(--text);
    background: var(--bg-hover);
  }
  .tab-item--active {
    background: var(--bg-active);
    color: var(--accent-strong);
    font-weight: var(--weight-semibold);
    box-shadow: inset 0 0 0 1px var(--border-strong);
  }
  .tab-count {
    display: inline-block;
    padding: 1px var(--space-1);
    border-radius: var(--radius-full);
    background: var(--bg-inset);
    color: var(--text-secondary);
    font-size: var(--text-2xs);
    font-weight: var(--weight-bold);
  }

  /* Toolbar */
  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    margin-bottom: var(--space-3);
    flex-wrap: wrap;
  }
  .toolbar-left,
  .toolbar-right {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .filter-pills {
    display: inline-flex;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 2px;
  }
  .pill {
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-sm);
    border: none;
    background: transparent;
    color: var(--text-muted);
    font-size: var(--text-xs);
    cursor: pointer;
    transition: background var(--duration-fast) ease, color var(--duration-fast) ease;
  }
  .pill:hover {
    color: var(--text);
  }
  .pill--active {
    background: var(--bg-active);
    color: var(--text);
    font-weight: var(--weight-semibold);
  }
  .search-box {
    position: relative;
    display: flex;
    align-items: center;
  }
  .search-icon {
    position: absolute;
    left: var(--space-2);
    color: var(--text-muted);
    font-size: 0.9rem;
    pointer-events: none;
  }
  .search-box .input {
    padding-left: var(--space-6);
    width: 220px;
  }

  /* Overview Tab Layout */
  .overview-grid {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .stats-row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--space-3);
  }
  .stat-card {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4);
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
  }
  .stat-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    border-radius: var(--radius-md);
    font-size: 1.4rem;
  }
  .stat-icon--active {
    background: var(--tint-accent);
    color: var(--accent-strong);
  }
  .stat-icon--danger {
    background: var(--tint-danger);
    color: var(--danger-strong);
  }
  .stat-icon--info {
    background: var(--tint-info);
    color: var(--info);
  }
  .stat-icon--violet {
    background: var(--tint-violet);
    color: var(--violet);
  }
  .stat-meta {
    display: flex;
    flex-direction: column;
  }
  .stat-val {
    font-size: var(--text-2xl);
    font-weight: var(--weight-bold);
    color: var(--text);
    line-height: var(--leading-tight);
  }
  .stat-lbl {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .shortcuts-bar {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  /* Hardware Specs Grid (aaPanel bottom table) */
  .specs-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    border-top: 1px solid var(--border-subtle);
  }
  .spec-cell {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
    border-right: 1px solid var(--border-subtle);
  }
  .spec-k {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .spec-v {
    font-size: var(--text-xs);
    color: var(--text);
    font-weight: var(--weight-medium);
  }

  /* Port Chips */
  .ports-list {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }
  .port-chip {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    padding: 1px var(--space-2);
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-xs);
    font-size: var(--text-2xs);
    font-family: var(--font-mono);
    color: var(--text-secondary);
  }
  .port-chip--link {
    color: var(--info);
    text-decoration: none;
  }
  .port-chip--link:hover {
    border-color: var(--info);
  }
  .action-btns-row {
    display: inline-flex;
    align-items: center;
    gap: 2px;
  }

  /* One-Click Template Cards */
  .templates-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: var(--space-3);
  }
  .template-card {
    display: flex;
    flex-direction: column;
    padding: var(--space-4);
    gap: var(--space-3);
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    transition: transform var(--duration-fast) ease, border-color var(--duration-fast) ease;
  }
  .template-card:hover {
    border-color: var(--border-strong);
    transform: translateY(-2px);
  }
  .template-card-top {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .template-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 42px;
    height: 42px;
    border-radius: var(--radius-md);
    background: var(--bg-inset);
    border: 1px solid var(--border);
    font-size: 1.5rem;
    color: var(--accent);
  }
  .template-name {
    font-size: var(--text-md);
    font-weight: var(--weight-bold);
    color: var(--text);
    margin: 0;
  }
  .template-desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin: 0;
    line-height: var(--leading-normal);
    flex: 1;
  }
  .template-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-top: 1px solid var(--border-subtle);
    padding-top: var(--space-2);
  }

  /* Cloud Search Form */
  .search-form-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  /* Settings Suite (Persis Screenshot) */
  .settings-rows {
    display: flex;
    flex-direction: column;
    gap: 0;
    padding: 0;
  }
  .setting-row {
    display: flex;
    align-items: flex-start;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
  }
  .setting-label {
    width: 220px;
    flex-shrink: 0;
    font-size: var(--text-sm);
    color: var(--text-secondary);
    padding-top: var(--space-1);
  }
  .setting-ctrl {
    flex: 1;
  }
  .status-indicator-group {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .mirror-box {
    display: flex;
    flex-direction: column;
    max-width: 500px;
  }
  .setting-hint {
    font-size: var(--text-2xs);
    color: var(--text-muted);
    margin-top: var(--space-1);
  }
  .border-top-danger {
    background: color-mix(in srgb, var(--danger) 5%, transparent);
  }

  /* Modals */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: var(--bg-overlay);
    backdrop-filter: blur(4px);
    z-index: var(--z-modal);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-4);
  }
  .modal {
    background: var(--bg-surface);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-lg);
    width: 100%;
    max-width: 560px;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .modal--lg {
    max-width: 760px;
  }
  .modal__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
    background: var(--bg-raised);
  }
  .modal__title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-md);
    font-weight: var(--weight-bold);
    color: var(--text);
    margin: 0;
  }
  .modal__body {
    padding: var(--space-4);
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .modal__foot {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    border-top: 1px solid var(--border);
    background: var(--bg-raised);
  }
  .form-row {
    display: flex;
    gap: var(--space-3);
  }
  .form-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    padding: var(--space-3);
    background: var(--bg-inset);
  }
  .form-section-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .mapping-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .terminal-log-viewer {
    background: var(--bg-inset);
    color: var(--text-secondary);
    padding: var(--space-4);
    margin: 0;
    max-height: 480px;
    overflow-y: auto;
    white-space: pre-wrap;
    word-break: break-all;
    border-bottom: 1px solid var(--border);
  }
  .json-editor-textarea {
    width: 100%;
    background: var(--bg-inset);
    color: var(--text);
    border: none;
    padding: var(--space-4);
    resize: vertical;
    outline: none;
  }

  /* Utility classes */
  .spin {
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
  .font-mono {
    font-family: var(--font-mono);
  }
  .text-xs {
    font-size: var(--text-xs);
  }
  .text-2xs {
    font-size: var(--text-2xs);
  }
  .text-muted {
    color: var(--text-muted);
  }
  .text-secondary {
    color: var(--text-secondary);
  }
  .text-accent {
    color: var(--accent);
  }
  .text-warning {
    color: var(--warning);
  }
  .text-danger {
    color: var(--danger);
  }
  .text-info {
    color: var(--info);
  }
  .flex-row {
    display: flex;
  }
  .items-center {
    align-items: center;
  }
  .gap-2 {
    gap: var(--space-2);
  }
  .gap-4 {
    gap: var(--space-4);
  }
  .flex-1 {
    flex: 1;
  }
  .w-full {
    width: 100%;
  }
  .w-24 {
    width: 6rem;
  }
  .w-28 {
    width: 7rem;
  }
  .p-0 {
    padding: 0 !important;
  }
  .mt-2 {
    margin-top: var(--space-2);
  }
  .mt-3 {
    margin-top: var(--space-3);
  }
  .mt-4 {
    margin-top: var(--space-4);
  }
  .py-6 {
    padding-top: var(--space-6);
    padding-bottom: var(--space-6);
  }
  .py-8 {
    padding-top: var(--space-8);
    padding-bottom: var(--space-8);
  }
  .py-10 {
    padding-top: 2.5rem;
    padding-bottom: 2.5rem;
  }
  .text-center {
    text-align: center;
  }
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
  }
  .empty-icon {
    font-size: 2.5rem;
    color: var(--text-disabled);
  }
  .empty-title {
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--text);
    margin: var(--space-2) 0 var(--space-1) 0;
  }
  .empty-desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin: 0;
  }
</style>
