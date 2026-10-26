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

<div class="k8s-page">
  <div class="page-header">
    <div>
      <h1 class="page-title">
        <i class="ri-bubble-chart-line"></i> Kubernetes Orchestrator
      </h1>
      <p class="page-desc">
        Pusat kendaliorkestrasi cluster Kubernetes & K3s host, pengelolaan Pod, Deployment, Service, dan deklarasi YAML.
      </p>
    </div>
    <div class="header-actions">
      <button class="btn btn-secondary" onclick={refreshCluster} disabled={loading}>
        <i class="ri-refresh-line" class:spin={loading}></i> Refresh Cluster
      </button>
    </div>
  </div>

  {#if loading && !status}
    <div class="card loading-card">
      <i class="ri-loader-4-line spin"></i>
      <p>Menghubungkan ke cluster Kubernetes host...</p>
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
        <button class="btn btn-primary" onclick={triggerInstallK3s} disabled={installingK3s}>
          {#if installingK3s}
            <i class="ri-loader-4-line spin"></i> Memulai Pemasangan...
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
          <span class="dot-online"></span>
          <span class="status-name">Cluster Online</span>
        </div>
        <span class="cluster-type-tag">{status.cluster_type.toUpperCase()}</span>
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
      <div class="tab-buttons">
        <button
          class="tab-btn"
          class:active={activeTab === 'workloads'}
          onclick={() => (activeTab = 'workloads')}
        >
          <i class="ri-apps-line"></i> Workloads (Pods & Deployments)
        </button>
        <button
          class="tab-btn"
          class:active={activeTab === 'services'}
          onclick={() => (activeTab = 'services')}
        >
          <i class="ri-links-line"></i> Services & Jaringan
        </button>
        <button
          class="tab-btn"
          class:active={activeTab === 'manifest'}
          onclick={() => (activeTab = 'manifest')}
        >
          <i class="ri-file-code-line"></i> Deploy Manifest (YAML)
        </button>
        <button
          class="tab-btn"
          class:active={activeTab === 'nodes'}
          onclick={() => (activeTab = 'nodes')}
        >
          <i class="ri-server-line"></i> Node Cluster ({nodes.length})
        </button>
      </div>

      <div class="namespace-filter">
        <label for="ns-select"><i class="ri-folder-shared-line"></i> Namespace:</label>
        <select
          id="ns-select"
          class="select-input"
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
            <p class="empty-state">Tidak ada deployment pada namespace '{selectedNamespace || 'semua'}'.</p>
          {:else}
            <div class="table-container">
              <table class="data-table">
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
                      <td class="bold-cell">{dep.name}</td>
                      <td><span class="badge-subtle">{dep.namespace}</span></td>
                      <td>
                        <span class="replica-badge" class:ready={dep.ready_replicas === dep.replicas}>
                          {dep.ready_replicas} / {dep.replicas} Siap
                        </span>
                      </td>
                      <td>{dep.age}</td>
                      <td class="image-cell">
                        {#each dep.images as img}
                          <span class="image-tag">{img}</span>
                        {/each}
                      </td>
                      <td class="actions-col">
                        <button class="btn btn-sm btn-secondary" onclick={() => openScaleModal(dep)}>
                          <i class="ri-equalizer-line"></i> Scale
                        </button>
                        <button
                          class="btn btn-sm btn-danger"
                          onclick={() => deleteResource('deployment', dep.name, dep.namespace)}
                        >
                          <i class="ri-delete-bin-line"></i>
                        </button>
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
            <p class="empty-state">Tidak ada Pod yang berjalan pada namespace '{selectedNamespace || 'semua'}'.</p>
          {:else}
            <div class="table-container">
              <table class="data-table">
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
                      <td class="bold-cell code-font">{pod.name}</td>
                      <td><span class="badge-subtle">{pod.namespace}</span></td>
                      <td>
                        <span
                          class="status-pill"
                          class:running={pod.status === 'Running'}
                          class:pending={pod.status === 'Pending'}
                          class:failed={pod.status !== 'Running' && pod.status !== 'Pending'}
                        >
                          {pod.status}
                        </span>
                      </td>
                      <td>{pod.ready}</td>
                      <td>{pod.restarts}x</td>
                      <td>{pod.age}</td>
                      <td class="code-font">{pod.ip || '-'}</td>
                      <td class="actions-col">
                        <button class="btn btn-sm btn-secondary" onclick={() => openLogsModal(pod)}>
                          <i class="ri-file-text-line"></i> Log
                        </button>
                        <button
                          class="btn btn-sm btn-danger"
                          onclick={() => deleteResource('pod', pod.name, pod.namespace)}
                        >
                          <i class="ri-delete-bin-line"></i>
                        </button>
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
          <p class="empty-state">Tidak ada Service pada namespace '{selectedNamespace || 'semua'}'.</p>
        {:else}
          <div class="table-container">
            <table class="data-table">
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
                    <td class="bold-cell code-font">{svc.name}</td>
                    <td><span class="badge-subtle">{svc.namespace}</span></td>
                    <td><span class="badge-type">{svc.type}</span></td>
                    <td class="code-font">{svc.cluster_ip}</td>
                    <td class="code-font">{svc.external_ip}</td>
                    <td class="code-font">{svc.ports}</td>
                    <td>{svc.age}</td>
                    <td class="actions-col">
                      <button
                        class="btn btn-sm btn-danger"
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

        <div class="yaml-editor-wrapper">
          <textarea
            class="yaml-textarea"
            bind:value={yamlContent}
            rows="18"
            placeholder="Ketik atau tempel konfigurasi YAML di sini..."
          ></textarea>
        </div>

        <div class="manifest-actions">
          <button class="btn btn-primary" onclick={applyManifest} disabled={applyingYaml}>
            {#if applyingYaml}
              <i class="ri-loader-4-line spin"></i> Menerapkan ke Cluster...
            {:else}
              <i class="ri-check-line"></i> Terapkan Manifest (Apply)
            {/if}
          </button>
        </div>

        {#if yamlResult}
          <div class="yaml-result-box" class:result-success={yamlResult.ok} class:result-error={!yamlResult.ok}>
            <span class="result-title">
              <i class={yamlResult.ok ? 'ri-checkbox-circle-line' : 'ri-error-warning-line'}></i>
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
          <p class="empty-state">Tidak ada informasi node yang ditemukan.</p>
        {:else}
          <div class="table-container">
            <table class="data-table">
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
                    <td class="bold-cell code-font">{node.name}</td>
                    <td>
                      <span class="status-pill" class:running={node.status === 'Ready'}>
                        {node.status}
                      </span>
                    </td>
                    <td><span class="badge-subtle">{node.roles}</span></td>
                    <td class="code-font">{node.version}</td>
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
</div>

<!-- Modal Scale Deployment -->
{#if showScaleModal}
  <div class="modal-backdrop">
    <div class="modal-card">
      <div class="modal-header">
        <h3><i class="ri-equalizer-line"></i> Ubah Skala Deployment</h3>
        <button class="close-btn" onclick={() => (showScaleModal = false)}>
          <i class="ri-close-line"></i>
        </button>
      </div>
      <div class="modal-body">
        <p class="modal-info">
          Atur jumlah replika Pod yang berjalan untuk deployment <strong>{scaleTarget.name}</strong> pada namespace <strong>{scaleTarget.namespace}</strong>.
        </p>

        <div class="form-group">
          <label for="scale-input">Jumlah Replika (Replicas):</label>
          <div class="stepper-row">
            <button
              class="btn btn-secondary stepper-btn"
              onclick={() => { if (scaleReplicas > 0) scaleReplicas--; }}
            >
              -
            </button>
            <input
              id="scale-input"
              type="number"
              min="0"
              max="50"
              class="scale-input"
              bind:value={scaleReplicas}
            />
            <button
              class="btn btn-secondary stepper-btn"
              onclick={() => scaleReplicas++}
            >
              +
            </button>
          </div>
        </div>
      </div>
      <div class="modal-footer">
        <button class="btn btn-secondary" onclick={() => (showScaleModal = false)}>Batal</button>
        <button class="btn btn-primary" onclick={submitScale} disabled={scaling}>
          {#if scaling}
            <i class="ri-loader-4-line spin"></i> Menyimpan...
          {:else}
            Simpan Skala
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal Pembacaan Log Pod -->
{#if showLogsModal}
  <div class="modal-backdrop">
    <div class="modal-card modal-lg">
      <div class="modal-header">
        <h3><i class="ri-terminal-box-line"></i> Log Pod: {logTarget.name}</h3>
        <button class="close-btn" onclick={() => (showLogsModal = false)}>
          <i class="ri-close-line"></i>
        </button>
      </div>
      <div class="modal-body">
        {#if loadingLogs}
          <div class="log-loading">
            <i class="ri-loader-4-line spin"></i> Memuat baris log kontainer...
          </div>
        {:else}
          <pre class="terminal-log-viewer"><code>{podLogs}</code></pre>
        {/if}
      </div>
      <div class="modal-footer">
        <button class="btn btn-secondary" onclick={() => openLogsModal({ name: logTarget.name, namespace: logTarget.namespace } as K8sPod)}>
          <i class="ri-refresh-line"></i> Refresh Log
        </button>
        <button class="btn btn-primary" onclick={() => (showLogsModal = false)}>Tutup</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .k8s-page {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    padding-bottom: 2rem;
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
  }

  .page-title {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .page-desc {
    margin: 0.35rem 0 0;
    font-size: 0.875rem;
    color: var(--text-muted);
  }

  .header-actions {
    display: flex;
    gap: 0.75rem;
  }

  .card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 1.25rem;
  }

  .loading-card {
    text-align: center;
    padding: 3rem;
    color: var(--text-muted);
    font-size: 1.1rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1rem;
  }

  .uninstalled-card {
    text-align: center;
    padding: 3.5rem 2rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1rem;
  }

  .uninstalled-icon {
    font-size: 3.5rem;
    color: var(--primary);
    background: var(--bg-inset);
    width: 5rem;
    height: 5rem;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
  }

  .uninstalled-card h2 {
    margin: 0;
    font-size: 1.35rem;
    color: var(--text);
  }

  .uninstalled-card p {
    max-width: 650px;
    margin: 0;
    color: var(--text-muted);
    line-height: 1.6;
    font-size: 0.95rem;
  }

  .manual-guide {
    margin-top: 1.5rem;
    text-align: left;
    width: 100%;
    max-width: 600px;
  }

  .guide-title {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .code-box {
    margin-top: 0.5rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.75rem 1rem;
    font-family: var(--font-mono);
    font-size: 0.85rem;
    color: var(--text);
    overflow-x: auto;
  }

  .status-overview-card {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  .status-badge-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .status-indicator {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-weight: 600;
    font-size: 0.95rem;
    color: var(--text);
  }

  .dot-online {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--success);
    box-shadow: 0 0 8px var(--success);
  }

  .cluster-type-tag {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.25rem 0.65rem;
    border-radius: var(--radius-sm);
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--primary);
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 1rem;
  }

  .stat-box {
    background: var(--bg-inset);
    padding: 1rem;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .stat-label {
    font-size: 0.8rem;
    color: var(--text-muted);
  }

  .stat-val {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--text);
  }

  .tab-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
    border-bottom: 1px solid var(--border);
    padding-bottom: 0.75rem;
  }

  .tab-buttons {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .tab-btn {
    background: transparent;
    border: 1px solid transparent;
    color: var(--text-muted);
    padding: 0.5rem 0.85rem;
    border-radius: var(--radius-sm);
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 0.4rem;
    transition: all 0.15s ease;
  }

  .tab-btn:hover {
    color: var(--text);
    background: var(--bg-inset);
  }

  .tab-btn.active {
    color: var(--text);
    background: var(--bg-card);
    border-color: var(--border);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  }

  .namespace-filter {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.875rem;
    color: var(--text-muted);
  }

  .select-input {
    background: var(--bg-card);
    border: 1px solid var(--border);
    color: var(--text);
    padding: 0.35rem 0.65rem;
    border-radius: var(--radius-sm);
    font-size: 0.875rem;
  }

  .section-card {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .card-title-row h3 {
    margin: 0;
    font-size: 1.05rem;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .empty-state {
    color: var(--text-muted);
    font-style: italic;
    margin: 0.5rem 0;
  }

  .table-container {
    overflow-x: auto;
  }

  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.875rem;
    text-align: left;
  }

  .data-table th {
    padding: 0.65rem 0.75rem;
    border-bottom: 1px solid var(--border);
    color: var(--text-muted);
    font-weight: 600;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .data-table td {
    padding: 0.75rem;
    border-bottom: 1px solid var(--border);
    color: var(--text);
  }

  .bold-cell {
    font-weight: 600;
  }

  .code-font {
    font-family: var(--font-mono);
    font-size: 0.82rem;
  }

  .actions-col {
    text-align: right;
    white-space: nowrap;
  }

  .badge-subtle {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.15rem 0.45rem;
    border-radius: var(--radius-sm);
    font-size: 0.75rem;
    color: var(--text-muted);
  }

  .badge-type {
    background: var(--bg-inset);
    color: var(--primary);
    font-weight: 600;
    padding: 0.15rem 0.5rem;
    border-radius: var(--radius-sm);
    font-size: 0.75rem;
  }

  .replica-badge {
    background: var(--bg-inset);
    padding: 0.2rem 0.5rem;
    border-radius: var(--radius-sm);
    font-weight: 600;
    font-size: 0.75rem;
    color: var(--warning);
  }

  .replica-badge.ready {
    color: var(--success);
  }

  .status-pill {
    display: inline-block;
    padding: 0.2rem 0.55rem;
    border-radius: 9999px;
    font-size: 0.75rem;
    font-weight: 600;
    background: var(--bg-inset);
    color: var(--text-muted);
  }

  .status-pill.running {
    background: color-mix(in srgb, var(--success) 15%, transparent);
    color: var(--success);
  }

  .status-pill.pending {
    background: color-mix(in srgb, var(--warning) 15%, transparent);
    color: var(--warning);
  }

  .status-pill.failed {
    background: color-mix(in srgb, var(--danger) 15%, transparent);
    color: var(--danger);
  }

  .image-cell {
    display: flex;
    gap: 0.35rem;
    flex-wrap: wrap;
  }

  .image-tag {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    background: var(--bg-inset);
    padding: 0.15rem 0.4rem;
    border-radius: var(--radius-sm);
    color: var(--text-muted);
  }

  .manifest-desc {
    margin: 0;
    font-size: 0.875rem;
    color: var(--text-muted);
  }

  .yaml-editor-wrapper {
    margin-top: 0.5rem;
  }

  .yaml-textarea {
    width: 100%;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text);
    font-family: var(--font-mono);
    font-size: 0.85rem;
    padding: 0.85rem;
    box-sizing: border-box;
    line-height: 1.5;
    resize: vertical;
  }

  .manifest-actions {
    display: flex;
    justify-content: flex-end;
  }

  .yaml-result-box {
    margin-top: 1rem;
    padding: 1rem;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: var(--bg-inset);
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .yaml-result-box.result-success {
    border-color: var(--success);
  }

  .yaml-result-box.result-error {
    border-color: var(--danger);
  }

  .result-title {
    font-weight: 600;
    font-size: 0.9rem;
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .result-output {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 0.8rem;
    color: var(--text);
    background: var(--bg-card);
    padding: 0.75rem;
    border-radius: var(--radius-sm);
    overflow-x: auto;
  }

  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .modal-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    width: 90%;
    max-width: 500px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .modal-lg {
    max-width: 800px;
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--border);
  }

  .modal-header h3 {
    margin: 0;
    font-size: 1.1rem;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .close-btn {
    background: transparent;
    border: none;
    font-size: 1.25rem;
    color: var(--text-muted);
    cursor: pointer;
  }

  .modal-body {
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .modal-info {
    margin: 0;
    font-size: 0.875rem;
    color: var(--text-muted);
    line-height: 1.5;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .form-group label {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--text);
  }

  .stepper-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .stepper-btn {
    width: 2.5rem;
    height: 2.5rem;
    padding: 0;
    font-size: 1.2rem;
    font-weight: 700;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .scale-input {
    width: 5rem;
    text-align: center;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--text);
    font-size: 1.1rem;
    font-weight: 700;
    padding: 0.4rem;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
    padding: 1rem 1.25rem;
    border-top: 1px solid var(--border);
    background: var(--bg-inset);
  }

  .terminal-log-viewer {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 1rem;
    font-family: var(--font-mono);
    font-size: 0.8rem;
    color: var(--text);
    max-height: 400px;
    overflow-y: auto;
    white-space: pre-wrap;
    word-break: break-all;
    line-height: 1.45;
  }

  .log-loading {
    padding: 2rem;
    text-align: center;
    color: var(--text-muted);
  }

  .spin {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }
</style>
