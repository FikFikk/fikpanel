<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type {
    ClusterStatus,
    K8sNamespace,
    K8sPod,
    K8sDeployment,
    K8sService,
    K8sNode,
  } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import '../styles/pages/kubernetes.css';

  // State utama cluster
  let loading = $state(true);
  let status = $state<ClusterStatus | null>(null);
  let namespaces = $state<K8sNamespace[]>([]);
  let selectedNamespace = $state('default');
  let activeTab = $state<'workloads' | 'services' | 'manifest' | 'nodes'>('workloads');

  // State data workload
  let pods = $state<K8sPod[]>([]);
  let deployments = $state<K8sDeployment[]>([]);
  let services = $state<K8sService[]>([]);
  let nodes = $state<K8sNode[]>([]);

  // State modal scaling
  let showScaleModal = $state(false);
  let scaleTarget = $state<{ name: string; namespace: string; current: number }>({
    name: '',
    namespace: '',
    current: 1,
  });
  let scaleReplicas = $state(1);
  let scaling = $state(false);

  // State modal log pod
  let showLogsModal = $state(false);
  let logTarget = $state<{ name: string; namespace: string }>({ name: '', namespace: '' });
  let podLogs = $state('');
  let loadingLogs = $state(false);

  // State deploy manifest YAML
  let yamlContent = $state(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-web
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: demo-web
  template:
    metadata:
      labels:
        app: demo-web
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: demo-web-svc
  namespace: default
spec:
  type: ClusterIP
  selector:
    app: demo-web
  ports:
  - port: 80
    targetPort: 80`);
  let applyingYaml = $state(false);
  let yamlResult = $state<{ ok: boolean; message: string; output?: string } | null>(null);

  // State instalasi K3s
  let installingK3s = $state(false);

  // Muat status dan data cluster saat inisialisasi
  onMount(async () => {
    await refreshCluster();
  });

  async function refreshCluster() {
    loading = true;
    try {
      const res = await apiFetch<{ ok: boolean; status: ClusterStatus }>('/api/k8s/status');
      if (res && res.status) {
        status = res.status;
        if (status.available) {
          await Promise.all([
            loadNamespaces(),
            loadWorkloads(),
            loadServices(),
            loadNodes(),
          ]);
        }
      }
    } catch {
      notify('Gagal membaca status cluster Kubernetes', 'error');
    } finally {
      loading = false;
    }
  }

  async function loadNamespaces() {
    try {
      const res = await apiFetch<{ ok: boolean; namespaces: K8sNamespace[] }>('/api/k8s/namespaces');
      if (res && res.namespaces) {
        namespaces = res.namespaces;
      }
    } catch {
      // Namespace fallback jika API lambat
    }
  }

  async function loadWorkloads() {
    const nsQuery = selectedNamespace ? `?namespace=${encodeURIComponent(selectedNamespace)}` : '';
    try {
      const [podRes, depRes] = await Promise.all([
        apiFetch<{ ok: boolean; pods: K8sPod[] }>(`/api/k8s/pods${nsQuery}`),
        apiFetch<{ ok: boolean; deployments: K8sDeployment[] }>(`/api/k8s/deployments${nsQuery}`),
      ]);
      if (podRes && podRes.pods) pods = podRes.pods;
      if (depRes && depRes.deployments) deployments = depRes.deployments;
    } catch {
      notify('Gagal memuat daftar pods atau deployment', 'error');
    }
  }

  async function loadServices() {
    const nsQuery = selectedNamespace ? `?namespace=${encodeURIComponent(selectedNamespace)}` : '';
    try {
      const res = await apiFetch<{ ok: boolean; services: K8sService[] }>(`/api/k8s/services${nsQuery}`);
      if (res && res.services) services = res.services;
    } catch {
      // Abaikan error minor service
    }
  }

  async function loadNodes() {
    try {
      const res = await apiFetch<{ ok: boolean; nodes: K8sNode[] }>('/api/k8s/nodes');
      if (res && res.nodes) nodes = res.nodes;
    } catch {
      // Abaikan error minor node
    }
  }

  function handleNamespaceChange() {
    loadWorkloads();
    loadServices();
  }

  // Buka modal scale deployment
  function openScaleModal(dep: K8sDeployment) {
    scaleTarget = {
      name: dep.name,
      namespace: dep.namespace,
      current: dep.replicas,
    };
    scaleReplicas = dep.replicas;
    showScaleModal = true;
  }

  // Kirim permintaan scale deployment
  async function submitScale() {
    scaling = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(
        `/api/k8s/deployments/${encodeURIComponent(scaleTarget.name)}/scale`,
        {
          method: 'POST',
          body: JSON.stringify({
            namespace: scaleTarget.namespace,
            replicas: scaleReplicas,
          }),
        }
      );
      if (res && res.ok) {
        notify(res.message || 'Skala deployment berhasil diperbarui', 'success');
        showScaleModal = false;
        await loadWorkloads();
      }
    } catch (err: any) {
      notify(err?.message || 'Gagal mengubah skala deployment', 'error');
    } finally {
      scaling = false;
    }
  }

  // Buka modal pembacaan log pod
  async function openLogsModal(pod: K8sPod) {
    logTarget = { name: pod.name, namespace: pod.namespace };
    showLogsModal = true;
    podLogs = '';
    loadingLogs = true;
    try {
      const res = await apiFetch<{ ok: boolean; logs: string }>(
        `/api/k8s/pods/${encodeURIComponent(pod.name)}/logs?namespace=${encodeURIComponent(pod.namespace)}&tail=150`
      );
      if (res && res.logs) {
        podLogs = res.logs;
      } else {
        podLogs = '(Belum ada log output yang dihasilkan oleh pod ini)';
      }
    } catch (err: any) {
      podLogs = `Gagal membaca log: ${err?.message || 'Error tidak diketahui'}`;
    } finally {
      loadingLogs = false;
    }
  }

  // Hapus resource Kubernetes (pod atau deployment)
  async function deleteResource(kind: 'pod' | 'deployment' | 'service', name: string, ns: string) {
    const ok = await confirmDialog({
      title: `Hapus resource ${kind}?`,
      message: `Hapus resource ${kind} '${name}' pada namespace '${ns}'? Tindakan ini tidak dapat dibatalkan.`,
      confirmText: `Hapus ${kind}`,
      tone: 'danger',
    });
    if (!ok) return;

    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/k8s/resources', {
        method: 'DELETE',
        body: JSON.stringify({ kind, name, namespace: ns }),
      });
      if (res && res.ok) {
        notify(res.message || 'Resource berhasil dihapus', 'success');
        if (kind === 'service') loadServices();
        else loadWorkloads();
      }
    } catch (err: any) {
      notify(err?.message || 'Gagal menghapus resource', 'error');
    }
  }

  // Terapkan manifest YAML langsung ke cluster
  async function applyManifest() {
    if (!yamlContent.trim()) {
      notify('Konten manifest YAML tidak boleh kosong', 'warning');
      return;
    }

    applyingYaml = true;
    yamlResult = null;
    try {
      const res = await apiFetch<{ ok: boolean; message: string; output?: string }>('/api/k8s/apply', {
        method: 'POST',
        body: JSON.stringify({
          manifest: yamlContent,
          namespace: selectedNamespace,
        }),
      });
      if (res) {
        yamlResult = res;
        notify('Manifest YAML berhasil diterapkan ke cluster', 'success');
        await loadWorkloads();
        await loadServices();
      }
    } catch (err: any) {
      yamlResult = { ok: false, message: err?.message || 'Gagal menerapkan manifest' };
      notify(err?.message || 'Gagal menerapkan manifest YAML', 'error');
    } finally {
      applyingYaml = false;
    }
  }

  // Mulai pemasangan K3s otomatis di latar belakang
  async function triggerInstallK3s() {
    const ok = await confirmDialog({
      title: 'Pasang K3s Kubernetes?',
      message: 'Pasang K3s lightweight Kubernetes sekarang? Skrip resmi Rancher akan dijalankan di latar belakang.',
      confirmText: 'Pasang K3s',
      tone: 'default',
    });
    if (!ok) return;

    installingK3s = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/k8s/install-k3s', {
        method: 'POST',
      });
      if (res && res.ok) {
        notify(res.message, 'success');
      }
    } catch (err: any) {
      notify(err?.message || 'Gagal memulai instalasi K3s', 'error');
    } finally {
      installingK3s = false;
    }
  }
</script>

<div class="pg-kubernetes k8s-page">
  <div class="page-header">
    <div>
      <h1 class="page-header__title">
        <i class="ri-bubble-chart-line"></i> Kubernetes Orchestrator
      </h1>
      <p class="page-header__sub">
        Pusat kendali orkestrasi cluster Kubernetes & K3s host, pengelolaan Pod, Deployment, Service, dan deklarasi YAML.
      </p>
    </div>
    <div class="page-header__actions">
      <button class="btn btn--secondary" onclick={refreshCluster} disabled={loading}>
        {#if loading}
          <span class="spinner"></span>
        {:else}
          <i class="ri-refresh-line"></i>
        {/if}
        Refresh Cluster
      </button>
    </div>
  </div>

  {#if loading && !status}
    <div class="card state state--loading">
      <span class="spinner"></span>
      <p class="state__desc">Menghubungkan ke cluster Kubernetes host...</p>
    </div>
  {:else if !status?.available}
    <!-- Tampilan fallback saat Kubernetes belum aktif di server -->
    <div class="card uninstalled-card">
      <div class="uninstalled-icon">
        <i class="ri-bubble-chart-line"></i>
      </div>
      <h2>Kubernetes / K3s Belum Aktif di Host</h2>
      <p>
        Server ini belum memiliki cluster Kubernetes aktif. Anda dapat memasang <strong>K3s (Lightweight Kubernetes by Rancher)</strong> secara otomatis dengan 1-klik, atau menggunakan cluster yang sudah ada via <code>/etc/rancher/k3s/k3s.yaml</code>.
      </p>
      <div class="uninstalled-actions">
        <button class="btn btn--primary" onclick={triggerInstallK3s} disabled={installingK3s}>
          {#if installingK3s}
            <span class="spinner"></span> Memulai Pemasangan...
          {:else}
            <i class="ri-download-cloud-line"></i> Pasang K3s Otomatis (1-Klik)
          {/if}
        </button>
      </div>
      <div class="manual-guide">
        <span class="guide-title">Perintah Pemasangan Manual di Terminal:</span>
        <pre class="code-box"><code>curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644</code></pre>
      </div>
    </div>
  {:else}
    <!-- Ringkasan Status Cluster Kubernetes -->
    <div class="card status-overview-card">
      <div class="status-badge-row">
        <div class="status-indicator">
          <span class="dot dot--success dot--pulse"></span>
          <span class="status-name">Cluster Online</span>
        </div>
        <span class="badge badge--info">{status.cluster_type.toUpperCase()}</span>
      </div>

      <div class="stats-grid">
        <div class="stat-box">
          <span class="stat-label">Versi Kubernetes</span>
          <span class="stat-val">{status.server_version || 'v1.28+'}</span>
        </div>
        <div class="stat-box">
          <span class="stat-label">Total Worker Node</span>
          <span class="stat-val">{status.nodes_count} Node</span>
        </div>
        <div class="stat-box">
          <span class="stat-label">Total Pods Aktif</span>
          <span class="stat-val">{status.pods_count} Pods</span>
        </div>
        <div class="stat-box">
          <span class="stat-label">Control Plane</span>
          <span class="stat-val">Host Localhost</span>
        </div>
      </div>
    </div>

    <!-- Navigasi Sub-Tab dan Pemilih Namespace -->
    <div class="tab-toolbar">
      <div class="tabs tabs--pill" role="tablist">
        <button
          class="tabs__tab"
          class:is-active={activeTab === 'workloads'}
          onclick={() => (activeTab = 'workloads')}
          role="tab"
          aria-selected={activeTab === 'workloads'}
        >
          <i class="ri-apps-line"></i> Workloads (Pods & Deployments)
        </button>
        <button
          class="tabs__tab"
          class:is-active={activeTab === 'services'}
          onclick={() => (activeTab = 'services')}
          role="tab"
          aria-selected={activeTab === 'services'}
        >
          <i class="ri-links-line"></i> Services & Jaringan
        </button>
        <button
          class="tabs__tab"
          class:is-active={activeTab === 'manifest'}
          onclick={() => (activeTab = 'manifest')}
          role="tab"
          aria-selected={activeTab === 'manifest'}
        >
          <i class="ri-file-code-line"></i> Deploy Manifest (YAML)
        </button>
        <button
          class="tabs__tab"
          class:is-active={activeTab === 'nodes'}
          onclick={() => (activeTab = 'nodes')}
          role="tab"
          aria-selected={activeTab === 'nodes'}
        >
          <i class="ri-server-line"></i> Node Cluster ({nodes.length})
        </button>
      </div>

      <div class="namespace-filter">
        <label for="ns-select"><i class="ri-folder-shared-line"></i> Namespace:</label>
        <select
          id="ns-select"
          class="input namespace-select"
          bind:value={selectedNamespace}
          onchange={handleNamespaceChange}
        >
          <option value="">(Semua Namespace)</option>
          {#each namespaces as ns}
            <option value={ns.name}>{ns.name}</option>
          {/each}
          {#if namespaces.length === 0}
            <option value="default">default</option>
            <option value="kube-system">kube-system</option>
          {/if}
        </select>
      </div>
    </div>

    <!-- Konten Tab: Workloads -->
    {#if activeTab === 'workloads'}
      <div class="workloads-section">
        <!-- Tabel Deployments -->
        <div class="card section-card">
          <div class="card-title-row">
            <h3><i class="ri-stack-line"></i> Deployments Terdaftar ({deployments.length})</h3>
          </div>

          {#if deployments.length === 0}
            <div class="state state--empty">
              <p class="state__desc">Tidak ada deployment pada namespace '{selectedNamespace || 'semua'}'.</p>
            </div>
          {:else}
            <div class="table-wrap">
              <table class="table">
                <thead>
                  <tr>
                    <th>Nama</th>
                    <th>Namespace</th>
                    <th>Replicas</th>
                    <th>Umur</th>
                    <th>Citra Kontainer</th>
                    <th class="actions-col">Aksi</th>
                  </tr>
                </thead>
                <tbody>
                  {#each deployments as dep}
                    <tr>
                      <td class="font-bold">{dep.name}</td>
                      <td><span class="badge badge--neutral">{dep.namespace}</span></td>
                      <td>
                        <span class="badge {dep.ready_replicas === dep.replicas ? 'badge--success' : 'badge--warning'}">
                          {dep.ready_replicas} / {dep.replicas} Siap
                        </span>
                      </td>
                      <td>{dep.age}</td>
                      <td>
                        <div class="image-cell">
                          {#each dep.images as img}
                            <span class="badge badge--neutral mono">{img}</span>
                          {/each}
                        </div>
                      </td>
                      <td class="actions-col">
                        <div class="row-actions">
                          <button class="btn btn--sm btn--secondary" onclick={() => openScaleModal(dep)}>
                            <i class="ri-equalizer-line"></i> Scale
                          </button>
                          <button
                            class="btn btn--sm btn--icon btn--danger"
                            onclick={() => deleteResource('deployment', dep.name, dep.namespace)}
                            aria-label="Hapus deployment {dep.name}"
                          >
                            <i class="ri-delete-bin-line"></i>
                          </button>
                        </div>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </div>

        <!-- Tabel Pods Kontainer -->
        <div class="card section-card">
          <div class="card-title-row">
            <h3><i class="ri-terminal-box-line"></i> Pods Kontainer ({pods.length})</h3>
          </div>

          {#if pods.length === 0}
            <div class="state state--empty">
              <p class="state__desc">Tidak ada Pod yang berjalan pada namespace '{selectedNamespace || 'semua'}'.</p>
            </div>
          {:else}
            <div class="table-wrap">
              <table class="table">
                <thead>
                  <tr>
                    <th>Nama Pod</th>
                    <th>Namespace</th>
                    <th>Status</th>
                    <th>Ready</th>
                    <th>Restarts</th>
                    <th>Umur</th>
                    <th>IP Pod</th>
                    <th class="actions-col">Aksi</th>
                  </tr>
                </thead>
                <tbody>
                  {#each pods as pod}
                    <tr>
                      <td class="font-bold mono">{pod.name}</td>
                      <td><span class="badge badge--neutral">{pod.namespace}</span></td>
                      <td>
                        <span
                          class="badge badge--pill {pod.status === 'Running'
                            ? 'badge--success'
                            : pod.status === 'Pending'
                              ? 'badge--warning'
                              : 'badge--danger'}"
                        >
                          {pod.status}
                        </span>
                      </td>
                      <td>{pod.ready}</td>
                      <td>{pod.restarts}x</td>
                      <td>{pod.age}</td>
                      <td class="mono">{pod.ip || '-'}</td>
                      <td class="actions-col">
                        <div class="row-actions">
                          <button class="btn btn--sm btn--secondary" onclick={() => openLogsModal(pod)}>
                            <i class="ri-file-text-line"></i> Log
                          </button>
                          <button
                            class="btn btn--sm btn--icon btn--danger"
                            onclick={() => deleteResource('pod', pod.name, pod.namespace)}
                            aria-label="Hapus pod {pod.name}"
                          >
                            <i class="ri-delete-bin-line"></i>
                          </button>
                        </div>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Konten Tab: Services -->
    {#if activeTab === 'services'}
      <div class="card section-card">
        <div class="card-title-row">
          <h3><i class="ri-links-line"></i> Services Jaringan ({services.length})</h3>
        </div>

        {#if services.length === 0}
          <div class="state state--empty">
            <p class="state__desc">Tidak ada Service pada namespace '{selectedNamespace || 'semua'}'.</p>
          </div>
        {:else}
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>Nama Service</th>
                  <th>Namespace</th>
                  <th>Tipe</th>
                  <th>Cluster IP</th>
                  <th>External IP</th>
                  <th>Port Exposure</th>
                  <th>Umur</th>
                  <th class="actions-col">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {#each services as svc}
                  <tr>
                    <td class="font-bold mono">{svc.name}</td>
                    <td><span class="badge badge--neutral">{svc.namespace}</span></td>
                    <td><span class="badge badge--info">{svc.type}</span></td>
                    <td class="mono">{svc.cluster_ip}</td>
                    <td class="mono">{svc.external_ip}</td>
                    <td class="mono">{svc.ports}</td>
                    <td>{svc.age}</td>
                    <td class="actions-col">
                      <button
                        class="btn btn--sm btn--danger"
                        onclick={() => deleteResource('service', svc.name, svc.namespace)}
                      >
                        <i class="ri-delete-bin-line"></i> Hapus
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

    <!-- Konten Tab: Manifest YAML -->
    {#if activeTab === 'manifest'}
      <div class="card section-card">
        <div class="card-title-row">
          <h3><i class="ri-file-code-line"></i> Terapkan Manifest YAML (Kubectl Apply)</h3>
        </div>
        <p class="manifest-desc">
          Terapkan konfigurasi Pod, Deployment, Service, Ingress, atau ConfigMap secara deklaratif langsung ke cluster.
        </p>

        <div>
          <textarea
            class="input input--mono"
            bind:value={yamlContent}
            rows="18"
            placeholder="Ketik atau tempel konfigurasi YAML di sini..."
          ></textarea>
        </div>

        <div class="manifest-actions">
          <button class="btn btn--primary" onclick={applyManifest} disabled={applyingYaml}>
            {#if applyingYaml}
              <span class="spinner"></span> Menerapkan ke Cluster...
            {:else}
              <i class="ri-check-line"></i> Terapkan Manifest (Apply)
            {/if}
          </button>
        </div>

        {#if yamlResult}
          <div class="yaml-result-box" class:result-success={yamlResult.ok} class:result-error={!yamlResult.ok}>
            <span class="result-title">
              <i class={yamlResult.ok ? 'ri-checkbox-circle-line text-accent' : 'ri-error-warning-line text-danger'}></i>
              {yamlResult.message}
            </span>
            {#if yamlResult.output}
              <pre class="result-output"><code>{yamlResult.output}</code></pre>
            {/if}
          </div>
        {/if}
      </div>
    {/if}

    <!-- Konten Tab: Nodes -->
    {#if activeTab === 'nodes'}
      <div class="card section-card">
        <div class="card-title-row">
          <h3><i class="ri-server-line"></i> Daftar Node Cluster</h3>
        </div>

        {#if nodes.length === 0}
          <div class="state state--empty">
            <p class="state__desc">Tidak ada informasi node yang ditemukan.</p>
          </div>
        {:else}
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>Nama Node</th>
                  <th>Status</th>
                  <th>Role</th>
                  <th>Versi Kubelet</th>
                  <th>Sistem Operasi / Image</th>
                </tr>
              </thead>
              <tbody>
                {#each nodes as node}
                  <tr>
                    <td class="font-bold mono">{node.name}</td>
                    <td>
                      <span class="badge badge--pill {node.status === 'Ready' ? 'badge--success' : 'badge--neutral'}">
                        {node.status}
                      </span>
                    </td>
                    <td><span class="badge badge--neutral">{node.roles}</span></td>
                    <td class="mono">{node.version}</td>
                    <td>{node.os_image}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    {/if}
  {/if}

  <!-- Modal Scale Deployment -->
  <Modal
    open={showScaleModal}
    title="Ubah Skala Deployment"
    size="md"
    onclose={() => (showScaleModal = false)}
  >
    <p class="modal-info">
      Atur jumlah replika Pod yang berjalan untuk deployment <strong>{scaleTarget.name}</strong> pada namespace <strong>{scaleTarget.namespace}</strong>.
    </p>

    <div class="field">
      <label class="field__label" for="scale-input">Jumlah Replika (Replicas):</label>
      <div class="stepper-row">
        <button
          class="btn btn--secondary stepper-btn"
          onclick={() => { if (scaleReplicas > 0) scaleReplicas--; }}
        >
          -
        </button>
        <input
          id="scale-input"
          type="number"
          min="0"
          max="50"
          class="input scale-input"
          bind:value={scaleReplicas}
        />
        <button
          class="btn btn--secondary stepper-btn"
          onclick={() => scaleReplicas++}
        >
          +
        </button>
      </div>
    </div>

    {#snippet footer()}
      <button class="btn btn--secondary" onclick={() => (showScaleModal = false)}>Batal</button>
      <button class="btn btn--primary" onclick={submitScale} disabled={scaling}>
        {#if scaling}
          <span class="spinner"></span> Menyimpan...
        {:else}
          Simpan Skala
        {/if}
      </button>
    {/snippet}
  </Modal>

  <!-- Modal Pembacaan Log Pod -->
  <Modal
    open={showLogsModal}
    title={`Log Pod: ${logTarget.name}`}
    size="lg"
    onclose={() => (showLogsModal = false)}
  >
    {#if loadingLogs}
      <div class="state state--loading">
        <span class="spinner"></span>
        <span class="state__desc">Memuat baris log kontainer...</span>
      </div>
    {:else}
      <pre class="terminal-log-viewer"><code>{podLogs}</code></pre>
    {/if}

    {#snippet footer()}
      <button class="btn btn--secondary" onclick={() => openLogsModal({ name: logTarget.name, namespace: logTarget.namespace } as K8sPod)}>
        <i class="ri-refresh-line"></i> Refresh Log
      </button>
      <button class="btn btn--primary" onclick={() => (showLogsModal = false)}>Tutup</button>
    {/snippet}
  </Modal>
</div>
