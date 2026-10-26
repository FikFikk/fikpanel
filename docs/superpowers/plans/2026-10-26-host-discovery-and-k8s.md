# Host Master Discovery & Kubernetes Orchestrator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform FikPanel into a master server control plane with dynamic auto-discovery of Nginx, Docker, PM2, and Databases, configurable custom working directories, and a full Kubernetes orchestration suite.

**Architecture:** 
1. Build `internal/discovery` to parse existing Nginx vhosts, Docker compose working directories, PM2 apps, and local databases without mutating or breaking existing configs.
2. Build `internal/k8s` wrapper using `kubectl` / K3s kubeconfig to inspect cluster health, list pods/deployments, scale replicas, stream container logs, and dynamically apply YAML manifests.
3. Integrate REST endpoints in `internal/server` and build `web/src/routes/Kubernetes.svelte` with custom working directory selectors in the UI.
4. Add `fikpanel discover` CLI command and update `scripts/install.sh`.

**Tech Stack:** Go 1.26 (modernc sqlite, exec, os), Svelte 5 (runes, design tokens), TypeScript, Vite, Tailwind-free Vanilla CSS.

---

### Task 1: Auto-Discovery Engine for Host Infrastructure

**Files:**
- Create: `internal/discovery/scanner.go`
- Create: `internal/discovery/scanner_test.go`
- Modify: `internal/store/advanced.go` (if needed for recording adopted items)

**Interfaces:**
- Produces: `DiscoveryResult`, `NginxVhostItem`, `DockerDiscoveredItem`, `PM2DiscoveredItem`, `DatabaseDiscoveredItem`
- Functions: `ScanAll(ctx context.Context, customRoots []string) (*DiscoveryResult, error)`

- [ ] **Step 1: Write the failing test**
Create test checking Nginx config parsing, Docker compose discovery, and PM2 extraction.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test ./internal/discovery/...`

- [ ] **Step 3: Implement `internal/discovery/scanner.go`**
Support Linux and Windows fallback, parsing `/etc/nginx/` vhosts, scanning project roots (e.g., `/home/*`, `/var/www`, `/opt`) for `docker-compose.yml`, querying `pm2 jlist`, and checking database listening sockets.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test ./internal/discovery/...`

- [ ] **Step 5: Commit**
Git: `feat(discovery): implement host infrastructure discovery engine`

---

### Task 2: Kubernetes Orchestration Module (`internal/k8s`)

**Files:**
- Create: `internal/k8s/types.go`
- Create: `internal/k8s/client.go`
- Create: `internal/k8s/client_test.go`

**Interfaces:**
- Produces: `Client`, `ClusterStatus`, `K8sPod`, `K8sDeployment`, `K8sService`, `K8sNamespace`
- Methods: `GetStatus(ctx)`, `ListNamespaces(ctx)`, `ListPods(ctx, ns)`, `ListDeployments(ctx, ns)`, `ScaleDeployment(ctx, ns, name, replicas)`, `ApplyManifest(ctx, yamlStr)`, `GetPodLogs(ctx, ns, name, container, lines)`

- [ ] **Step 1: Write the failing unit tests for K8s types and parser**
Create `internal/k8s/client_test.go` to test JSON/YAML parsing of kubectl output.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test ./internal/k8s/...`

- [ ] **Step 3: Implement `internal/k8s/client.go` and `internal/k8s/types.go`**
Auto-detect K3s `/etc/rancher/k3s/k3s.yaml`, MicroK8s, or standard kubeconfig. Wrap `kubectl` commands with JSON outputs (`kubectl get pods -o json`) and safe fallback for non-cluster environments.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test ./internal/k8s/...`

- [ ] **Step 5: Commit**
Git: `feat(k8s): implement kubernetes cluster client and manifest manager`

---

### Task 3: Discovery & Kubernetes REST Endpoints

**Files:**
- Create: `internal/server/discovery_routes.go`
- Create: `internal/server/k8s_routes.go`
- Modify: `internal/server/server.go`

**Interfaces:**
- Endpoints:
  - `POST /api/discovery/scan`
  - `POST /api/discovery/adopt`
  - `GET /api/k8s/status`
  - `GET /api/k8s/namespaces`
  - `GET /api/k8s/pods`
  - `GET /api/k8s/deployments`
  - `POST /api/k8s/deployments/{name}/scale`
  - `POST /api/k8s/apply`
  - `GET /api/k8s/pods/{name}/logs`
  - `DELETE /api/k8s/resources`

- [ ] **Step 1: Write endpoint tests in `internal/server/`**
- [ ] **Step 2: Run test to verify it fails**
- [ ] **Step 3: Implement handlers in `discovery_routes.go` and `k8s_routes.go`**
- [ ] **Step 4: Register routes in `server.go`**
- [ ] **Step 5: Run tests to verify they pass**
Run: `go test ./internal/server/...`
- [ ] **Step 6: Commit**
Git: `feat(server): register discovery and kubernetes management endpoints`

---

### Task 4: Dynamic Working Directory & Project Roots

**Files:**
- Modify: `internal/store/advanced.go`
- Modify: `internal/server/routes.go` (Sites API to support updating directory)

- [ ] **Step 1: Add custom directory update for sites and node apps**
- [ ] **Step 2: Verify database migrations and tests pass**
Run: `go test ./internal/store/...`
- [ ] **Step 3: Commit**
Git: `feat(projects): add dynamic root directories support`

---

### Task 5: Frontend Kubernetes UI & Server Discovery Bar

**Files:**
- Create: `web/src/routes/Kubernetes.svelte`
- Modify: `web/src/lib/layout/nav.ts`
- Modify: `web/src/App.svelte`
- Modify: `web/src/lib/types.ts`

- [ ] **Step 1: Add Kubernetes types to `web/src/lib/types.ts`**
- [ ] **Step 2: Add `kubernetes` navigation item to `nav.ts` and loader in `App.svelte`**
- [ ] **Step 3: Implement `Kubernetes.svelte`** (Cluster overview, Pods list, Scale modal, YAML manifest deployer, Log stream modal)
- [ ] **Step 4: Add Discovery modal / quick action button on Dashboard**
- [ ] **Step 5: Verify UI lint and build**
Run: `npm run build` in `web`
- [ ] **Step 6: Commit**
Git: `feat(ui): add kubernetes cluster manager and host discovery interface`

---

### Task 6: CLI Discovery & Installer Update

**Files:**
- Modify: `cmd/fikpanel/main.go`
- Modify: `scripts/install.sh`

- [ ] **Step 1: Implement `fikpanel discover` command in `cmd/fikpanel/main.go`**
- [ ] **Step 2: Update `scripts/install.sh` to trigger discovery at finish**
- [ ] **Step 3: Verify Go compilation and test**
Run: `go build ./...`
- [ ] **Step 4: Commit**
Git: `feat(cli): add headless discover command and automate in install script`

---

### Task 7: Documentation & Verification

**Files:**
- Modify: `documentation-api.md`
- Modify: `documentation.md`

- [ ] **Step 1: Document all new Kubernetes and Discovery endpoints in `documentation-api.md`**
- [ ] **Step 2: Document business rules in `documentation.md`**
- [ ] **Step 3: Run full verification (`go test ./...` and `npm run build`)**
- [ ] **Step 4: Final commit**
