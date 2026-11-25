// Definisi tipe data untuk metrik sistem, layanan, situs, dan respon API
export interface MetricPoint {
  timestamp: number;
  cpu: number;
  ram_used: number;
  ram_total: number;
  disk_used: number;
  disk_total: number;
  net_in: number;
  net_out: number;
  load_avg: number;
}

export type SiteKind = 'proxy' | 'static' | 'php' | 'node' | 'go' | 'python';
export type SSLStatus = 'none' | 'pending' | 'issued' | 'failed';

export type SiteItem = Site;
export interface Site {
  id: number;
  domain: string;
  aliases: string;
  kind: SiteKind;
  upstream: string;
  root_path: string;
  ssl_mode: string;
  force_https: number;
  hsts: number;
  php_version: string;
  description: string;
  extra_conf: string;
  enabled: number;
  updated_at: number;
  framework?: string;
  ssl_status: SSLStatus;
  ssl_expires: number;
  live_status?: 'running' | 'down' | 'degraded' | 'stopped' | string;
  live_detail?: string;
  request_count?: number;
  request_source?: 'nginx' | 'global_log' | 'waf' | 'tunnel' | 'none' | string;
  backup_count?: number;
  has_git?: boolean;
  git_branch?: string;
  git_repo?: string;
  git_commit?: string;
  has_cicd?: boolean;
  cicd_provider?: string;
}

export interface DomainCheck {
  input: string;
  domain: string;
  wildcard: boolean;
  auto: boolean;
  error?: string;
}

export interface DNSResult {
  domain: string;
  ips: string[] | null;
  points_here: boolean;
  error?: string;
}

export interface DomainReport {
  primary: string;
  aliases: string[];
  checks: DomainCheck[] | null;
  dns: DNSResult[];
  server_ips: string[];
  valid: boolean;
}

export interface SiteMeta {
  php_versions: string[];
  server_ips: string[];
}

export interface Certificate {
  site_id: number;
  status: SSLStatus;
  issuer: string;
  not_before: number;
  not_after: number;
  domains: string;
  signature: string;
  key_type: string;
  last_error: string;
  renewed_at: number;
  challenge_type?: 'http01' | 'dns01' | string;
  dns_provider?: string;
}

export interface DNSProviderField {
  key: string;
  label: string;
  placeholder: string;
  secret: boolean;
  required: boolean;
}

export interface DNSProviderMeta {
  id: string;
  name: string;
  description: string;
  configured: boolean;
  source?: string;
  fields: DNSProviderField[];
}

export interface SSLJob {
  running: boolean;
  step: string;
  ok: boolean;
  message: string;
  started_at: number;
  finished_at: number;
}

export interface SiteDetail {
  site: Site;
  domains: string[];
  certificate: Certificate | null;
  ssl_job: SSLJob | null;
}

export interface SoftwareItem {
  name: string;
  display_name: string;
  category: 'webserver' | 'database' | 'runtime' | 'tools';
  installed: boolean;
  version: string;
  running: boolean;
  service_unit: string;
}

export interface SitesData {
  sites: Site[];
  web_server: SoftwareItem;
}

export interface Service {
  unit: string;
  name?: string;
  label: string;
  description?: string;
  category?: 'web' | 'database' | 'security' | 'supervisor' | 'system' | string;
  category_label?: string;
  load_state?: string;
  active_state: string;
  sub_state: string;
  unit_file_state?: string;
  main_pid: number;
  memory_current: number;
  memory_peak?: number;
  tasks_current?: number;
  active_enter_timestamp?: string;
  uptime?: string;
  exec_start?: string;
  restart?: string;
  protected: boolean;
}

export interface AuditLog {
  id: number;
  ts: number;
  user_id: number | null;
  actor: string;
  action: string;
  target: string;
  ok: number;
  detail: string;
}

export interface OverviewData {
  system: {
    hostname: string;
    os: string;
    kernel: string;
    version?: string;
    cpu_cores: number;
    uptime_sec: number;
    ip: string;
  };
  counts: {
    sites_total: number;
    sites_running: number;
    sites_stopped: number;
    services_total: number;
    services_active: number;
    databases_count: number;
    database_engine: string;
    security_risk: number;
  };
  server_time?: {
    unix: number;
    timezone_name: string;
    offset_sec: number;
  };
  latest_metric: MetricPoint;
}

export interface FileItem {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
  mode: string;
  owner: string;
  mod_time: string;
  ext: string;
  protected: boolean;
  ps: string;
}

export interface FileListData {
  current_path: string;
  parent_path: string;
  items: FileItem[];
  total_dirs: number;
  total_files: number;
  total_size: number;
}

// ═══ Nginx Manager ═══
export interface ManagerTab {
  id: string;
  label: string;
  icon: string;
}
export interface NginxStatus {
  installed: boolean;
  running: boolean;
  version: string;
  master_pid: number;
  workers: number;
  memory_bytes: number;
  started_at: number;
  conf_path: string;
  error_log_path: string;
  daemon: boolean;
  alert: boolean;
  mock: boolean;
}

export interface NginxLoad {
  available: boolean;
  hint?: string;
  active: number;
  accepts: number;
  handled: number;
  requests: number;
  reading: number;
  writing: number;
  waiting: number;
  workers: number;
  memory_bytes: number;
  timestamp: number;
}

export interface NginxSwitchJob {
  running: boolean;
  target: string;
  started_at: number;
  finished_at: number;
  ok: boolean;
  message: string;
}

export interface NginxVersions {
  current: string;
  available: string[];
  job: NginxSwitchJob;
}

export interface NginxOptimization {
  worker_processes: string;
  worker_connections: number;
  keepalive_timeout: number;
  gzip: boolean;
  gzip_min_length: number;
  gzip_comp_level: number;
  client_max_body_size: number;
  server_names_hash_bucket_size: number;
  client_header_buffer_size: number;
  client_body_buffer_size: number;
}

export interface NginxErrorLog {
  path: string;
  size: number;
  lines: number;
  content: string;
}

// AI Provider, Percakapan, dan Pesan Riwayat
export type AIProviderKind = 'openai_compat' | 'anthropic' | 'gemini';

export interface AIProvider {
  id: number;
  name: string;
  kind: AIProviderKind;
  base_url: string;
  model: string;
  api_key: string;
  has_key?: boolean;
  is_default: boolean;
  created_at: number;
  updated_at: number;
}

export interface AIConversation {
  id: number;
  user_id: number;
  title: string;
  created_at: number;
  updated_at: number;
  message_count: number;
}

export interface AIMessage {
  id: number;
  conversation_id: number;
  conversationID?: number;
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
  tool_name?: string;
  ts: number;
}

export interface EngineVersion {
  version: string;
  label: string;
  is_recommended: boolean;
}

export interface DatabaseEngineInfo {
  name: 'mysql' | 'postgresql' | 'mongodb' | 'redis';
  display_name: string;
  installed: boolean;
  version: string;
  running: boolean;
  port: number;
  databases_count: number;
  service_unit: string;
  supported_versions: string[];
  versions?: EngineVersion[];
  description: string;
}

export interface DatabaseItem {
  id: number;
  engine: string;
  name: string;
  username: string;
  password: string;
  size_bytes: number;
  backup_count: number;
  location: string;
  note: string;
  created_at: number;
  timestamp?: number;
}

export interface SystemTask {
  id: string;
  type: string;
  target: string;
  status: 'pending' | 'running' | 'success' | 'failed' | 'cancelled';
  log: string;
  created_at: number;
  updated_at: number;
}

// ── Security Center & Scanner ─────────────────────────────────────────────
export interface SSHConfig {
  is_linux: boolean;
  installed: boolean;
  active_service: boolean;
  port: number;
  password_auth: boolean;
  pubkey_auth: boolean;
  permit_root_login: 'yes' | 'prohibit-password' | 'no' | string;
  max_auth_tries: number;
  cloud_init_override: boolean;
  cloud_init_file?: string;
  config_source: string;
  authorized_key_count: number;
}

export interface SSHAuthorizedKey {
  id: string;
  type: string;
  fingerprint: string;
  comment: string;
  preview: string;
}

export interface Fail2BanJail {
  name: string;
  currently_failed: number;
  total_failed: number;
  banned_count: number;
  banned_ips: string[];
}

export interface Fail2BanStatus {
  is_linux: boolean;
  installed: boolean;
  running: boolean;
  jails: Fail2BanJail[];
}

export interface ScanFinding {
  id: string;
  category: 'baseline' | 'risk' | 'malware' | 'vuln';
  severity: 'critical' | 'warning' | 'safe';
  title: string;
  detail: string;
  fix_hint: string;
  target_tab: string;
  action_key?: string;
  fixable: boolean;
}

export interface SecurityScanReport {
  id?: number;
  score: number;
  rating: string;
  scanned_at: number;
  risk_count: number;
  malware_count: number;
  vuln_count: number;
  baseline_pass: number;
  baseline_total: number;
  findings: ScanFinding[];
}

// ── Persistent Monitoring ──────────────────────────────────────────────────
export interface MonitorConfig {
  enabled: boolean;
  save_days: number;
}

export interface MonitorSnapshot {
  id: number;
  timestamp: number;
  cpu: number;
  ram_used: number;
  ram_total: number;
  load_1m: number;
  load_5m: number;
  load_15m: number;
  disk_used: number;
  disk_total: number;
  net_in: number;
  net_out: number;
}

// ── App Store & Toolkit Installer ─────────────────────────────────────────
export interface AppPackage {
  id: string;
  name: string;
  category: 'webserver' | 'runtime' | 'database' | 'tools';
  version: string;
  description: string;
  icon: string;
  installed: boolean;
  status: 'running' | 'stopped' | 'not_installed';
  apt_packages: string[];
}

export interface InstallTask {
  app_id: string;
  status: 'running' | 'success' | 'failed';
  logs: string;
  started_at: number;
}

// ── System Process & Cleaner ──────────────────────────────────────────────
export interface SystemProcess {
  pid: number;
  user: string;
  cpu: number;
  memory: number;
  command: string;
}

export interface PortListening {
  proto: string;
  port: string;
  address: string;
  program: string;
  pid: number;
}

export interface CleanerScanResult {
  apt_cache_bytes: number;
  system_log_bytes: number;
  tmp_bytes: number;
  docker_reclaim_bytes: number;
  total_bytes: number;
}

// ── DNS Management ────────────────────────────────────────────────────────
export interface DNSZone {
  id: number;
  name: string;
  ttl: number;
  created_at: number;
  updated_at: number;
}

export interface DNSRecord {
  id: number;
  zone_id: number;
  type: string;
  name: string;
  content: string;
  priority: number;
  ttl: number;
  created_at: number;
  timestamp?: number;
}

// ── PHP Versions ──────────────────────────────────────────────────────────
export interface PHPExtensionInfo {
  name: string;
  type: string;
  description: string;
  installed: boolean;
  enabled: boolean;
  package_name?: string;
  install_command?: string;
  uninstall_command?: string;
  is_core?: boolean;
}

export interface PHPVersionDetail {
  version: string;
  installed: boolean;
  running: boolean;
  is_cli_default?: boolean;
  fpm_socket: string;
  ini_path: string;
  pool_conf_path: string;
  log_path: string;
  slow_log_path: string;
  service_status: {
    active?: boolean;
    pid?: string | number;
    memory?: string;
    uptime?: string;
    since?: string;
  };
  settings: Record<string, string>;
  fpm_settings: Record<string, string>;
  opcache_settings: Record<string, string>;
  disabled_functions: string[];
  loaded_extensions: string[];
  all_extensions: PHPExtensionInfo[];
  sites?: AttachedSite[];
  sites_count?: number;
  fpmSocket?: string;
  iniPath?: string;
  serviceStatus?: {
    active?: boolean;
    pid?: string | number;
    memory?: string;
    uptime?: string;
    since?: string;
  };
}

export interface AttachedSite {
  id: number;
  domain: string;
  user: string;
  root_path: string;
  enabled: boolean;
  ssl_mode?: string;
}

export interface PHPVersionInfo {
  version: string;
  installed: boolean;
  running: boolean;
  is_cli_default?: boolean;
  fpm_socket: string;
  ini_path: string;
  settings: Record<string, string>;
  sites?: AttachedSite[];
  sites_count?: number;
}

// ── Webhooks & Alerts ─────────────────────────────────────────────────────
export interface SystemWebhook {
  id: number;
  name: string;
  channel: string;
  webhook_url: string;
  notify_on_high_cpu: boolean;
  notify_on_high_ram: boolean;
  notify_on_backup: boolean;
  notify_on_ssl_expiry: boolean;
  created_at: number;
}

// ── Backup & Arsip ───────────────────────────────────────────────────────
export interface BackupConfig {
  id: number;
  name: string;
  target_type: 'database' | 'site' | 'directory';
  target_name: string;
  storage_type: 'local' | 's3';
  s3_endpoint: string;
  s3_bucket: string;
  s3_access_key: string;
  s3_secret_key: string;
  s3_region: string;
  keep_copies: number;
  auto_cron: boolean;
  created_at: number;
  updated_at: number;
}

export interface BackupRecord {
  id: number;
  config_id: number;
  file_name: string;
  file_size_bytes: number;
  storage_type: string;
  storage_path: string;
  status: 'success' | 'failed' | 'running';
  error_message: string;
  created_at: number;
}

// ── Firewall & UFW ────────────────────────────────────────────────────────
export interface FirewallRule {
  number: number;
  to: string;
  action: string;
  from: string;
  proto: string;
}

export interface FirewallStatus {
  enabled: boolean;
  rules: FirewallRule[];
  raw: string;
}

// ── Enterprise Security Center & Firewall Guard ───────────────────────────
export interface FirewallRule {
  number: number;
  to: string;
  port: string;
  protocol: string;
  action: 'ALLOW' | 'DENY' | 'REJECT' | 'LIMIT' | string;
  direction: 'Inbound' | 'Outbound' | string;
  from: string;
  ip_version: string;
  service: string;
  remarks: string;
  listening: boolean;
  process: string;
  protected: boolean;
}

export interface FirewallListeningPort {
  protocol: string;
  address: string;
  port: string;
  process: string;
  pid: number;
  exposed_public: boolean;
  service: string;
}

export interface FirewallExtendedStatus {
  is_linux: boolean;
  installed: boolean;
  enabled: boolean;
  hostname: string;
  default_incoming: string;
  default_outgoing: string;
  block_icmp: boolean;
  ssh_port: number;
  panel_port: number;
  inbound_count: number;
  outbound_count: number;
  listening_count: number;
  rules: FirewallRule[];
  listening_ports: FirewallListeningPort[];
  site_logs_path: string;
  site_logs_size: string;
  site_logs_bytes: number;
  raw: string;
}

export interface BruteForceConfig {
  enabled: boolean;
  username_protection: boolean;
  username_max_retries: number;
  ip_protection: boolean;
  ip_max_retries: number;
  ip_ban_duration_minutes: number;
  retention_minutes: number;
  notify_on_login: boolean;
  notify_on_ban: boolean;
  auto_sync_ufw: boolean;
}

export interface SecurityIPRule {
  id: number;
  list_type: 'whitelist' | 'blacklist';
  ip: string;
  remarks: string;
  source: string;
  sync_ufw: boolean;
  added_at: number;
  expires_at: number;
}

export interface SecurityLoginEvent {
  id: number;
  time: number;
  ip: string;
  username: string;
  source: 'panel' | 'ssh' | string;
  status: 'failed' | 'success' | 'banned' | string;
  reason: string;
  user_agent?: string;
}

export interface SystemAccount {
  username: string;
  uid: number;
  gid: number;
  shell: string;
  home_dir: string;
  locked: boolean;
  lock_reason: string;
  is_protected: boolean;
}

export interface SSHLog {
  timestamp: number;
  user: string;
  ip: string;
  port: number;
  status: 'success' | 'failed';
  method: string;
  message: string;
}

export interface SSHDayStat {
  date: string;
  label: string;
  success: number;
  failed: number;
}

export interface SSHStats {
  config: SSHConfig;
  log_source: string;
  successful_logins: number;
  failed_logins: number;
  today_success: number;
  today_fail: number;
  yesterday_success: number;
  yesterday_fail: number;
  daily_stats: SSHDayStat[];
  recent_logs: SSHLog[];
}

export interface LastLoginInfo {
  available: boolean;
  ip: string;
  user: string;
  time: string;
  timestamp: number;
  port?: number;
  success: boolean;
}

export interface ProtectionItem {
  key: string;
  name: string;
  installed: boolean;
  enabled: boolean;
  status_text: string;
  summary: string;
  target_tab: string;
  toggleable: boolean;
}

export interface ProtectionOverview {
  is_linux: boolean;
  hostname: string;
  enabled_count: number;
  total_count: number;
  client_ip: string;
  is_client_whitelisted: boolean;
  last_ssh_login: LastLoginInfo;
  last_panel_login: LastLoginInfo;
  latest_scan: SecurityScanReport | null;
  pillars: ProtectionItem[];
}

export interface SysctlParam {
  key: string;
  title: string;
  description: string;
  enabled: boolean;
  current_value: string;
  recommended_value: string;
  persisted: boolean;
}

export interface HardeningStatus {
  is_linux: boolean;
  compiler_blocked: boolean;
  compilers_found: string[];
  icmp_blocked: boolean;
  sysctl_params: SysctlParam[];
  fim_enabled: boolean;
  fim_baseline_count: number;
  fim_last_updated: number;
}

export interface FIMEntryStatus {
  path: string;
  status: 'ok' | 'modified' | 'missing';
  expected_hash: string;
  actual_hash: string;
  size: number;
  mode: string;
  mod_time: number;
}

export interface FIMCheckResult {
  enabled: boolean;
  total_files: number;
  ok_count: number;
  modified_count: number;
  missing_count: number;
  last_baseline_at: number;
  checked_at: number;
  items: FIMEntryStatus[];
}

// ── Node & PM2 Management ─────────────────────────────────────────────────
export interface NodeSystemStatus {
  node_installed: boolean;
  node_version: string;
  pm2_installed: boolean;
  pm2_version: string;
}

export interface PM2Process {
  id: number;
  name: string;
  namespace?: string;
  version?: string;
  mode?: string;
  pid: number;
  uptime?: number;
  restart_count?: number;
  status: 'online' | 'stopped' | 'errored' | 'launching';
  cpu: number;
  memory: number;
  user: string;
  script?: string;
  cwd?: string;
}

// ── Cron Job Management ──────────────────────────────────────────────────
export interface CronJob {
  id: number;
  name: string;
  schedule_type: 'minutes' | 'minute_n' | 'hourly' | 'daily' | 'weekly' | 'monthly' | 'custom' | string;
  spec: string;
  command: string;
  working_dir?: string;
  enabled: boolean;
  save_log: boolean;
  notify_on_fail?: boolean;
  log_file?: string;
  timeout_sec: number;
  last_run_at: number;
  last_status: string;
  created_at: number;
  updated_at: number;
  next_run_at?: number;
  is_running?: boolean;
}

export interface CronLog {
  id: number;
  job_id: number;
  status: 'success' | 'failed' | 'timeout' | 'stopped' | 'interrupted' | string;
  output: string;
  exit_code: number;
  duration_ms: number;
  started_at: number;
  finished_at: number;
}

export interface SystemCronEntry {
  source: string;
  file: string;
  schedule: string;
  user: string;
  command: string;
}

// ── Docker & Kontainerisasi (10 Modul) ──────────────────────────────────────
export interface DockerContainerPort {
  ip: string;
  private_port: number;
  public_port: number;
  type: string;
}

export interface DockerContainer {
  id: string;
  names: string[];
  image: string;
  command: string;
  created: number;
  state: string;
  status: string;
  ports: DockerContainerPort[];
  cpu_percent: number;
  mem_usage: string;
  size_rw: string;
}

export interface DockerImage {
  id: string;
  repository: string;
  tag: string;
  digest: string;
  created: number;
  size: number;
  size_human: string;
  containers: number;
}

export interface DockerVolume {
  name: string;
  driver: string;
  scope: string;
  mountpoint: string;
  created_at: string;
  size_human: string;
  ref_count: number;
}

export interface DockerNetwork {
  id: string;
  name: string;
  driver: string;
  scope: string;
  subnet: string;
  gateway: string;
  internal: boolean;
  containers_count: number;
}

export interface DockerComposeProject {
  name: string;
  status: string;
  config_path: string;
  containers_count: number;
  yaml_content: string;
}

export interface OneClickTemplate {
  id: string;
  name: string;
  category: string;
  icon: string;
  description: string;
  image: string;
  default_port: number;
  env_params: Array<{ key: string; label: string; default: string; type: string }>;
  volume_mount: string;
  readme: string;
}

export interface DockerRegistry {
  id: string;
  name: string;
  url: string;
  username: string;
  is_default: boolean;
}

export interface DockerStatus {
  installed: boolean;
  compose_installed: boolean;
  running: boolean;
  version: string;
  compose_version: string;
  containers_total: number;
  containers_running: number;
  containers_paused: number;
  containers_stopped: number;
  images_total: number;
  volumes_total: number;
  networks_total: number;
  server_version: string;
  storage_driver: string;
  memory_limit: boolean;
  swap_limit: boolean;
  root_dir: string;
}

export interface DockerSystemInfo {
  hostname: string;
  os_version: string;
  architecture: string;
  kernel_version: string;
  cpu_cores: number;
  memory_size: string;
  docker_version: string;
  compose_version: string;
  unix_address: string;
  data_directory: string;
}

export interface DockerDaemonConfig {
  registry_mirrors: string[];
  compose_path: string;
  private_registry: string;
  log_rotation: boolean;
  log_max_size: string;
  log_max_file: string;
  ipv6: boolean;
  ipv6_subnet: string;
  iptables: boolean;
  live_restore: boolean;
  one_click_dir: string;
  raw_json: string;
}

// ── Kubernetes Orchestrator ───────────────────────────────────────────────
export interface K8sNode {
  name: string;
  status: string;
  roles: string;
  version: string;
  os_image: string;
}

export interface K8sNamespace {
  name: string;
  status: string;
  age: string;
}

export interface K8sPod {
  name: string;
  namespace: string;
  status: string;
  ready: string;
  restarts: number;
  age: string;
  ip: string;
  node: string;
  containers: string[];
}

export interface K8sDeployment {
  name: string;
  namespace: string;
  ready_replicas: number;
  replicas: number;
  available_replicas: number;
  age: string;
  images: string[];
}

export interface K8sService {
  name: string;
  namespace: string;
  type: string;
  cluster_ip: string;
  external_ip: string;
  ports: string;
  age: string;
}

export interface ClusterStatus {
  available: boolean;
  cluster_type: string;
  server_version: string;
  nodes_count: number;
  pods_count: number;
  nodes: K8sNode[];
}

// ── Host Discovery & Adoption ─────────────────────────────────────────────
export interface NginxDiscoveredItem {
  config_file: string;
  server_name: string;
  aliases?: string[];
  root_dir: string;
  port: number;
  ssl: boolean;
  proxy_pass: string;
  is_adopted: boolean;
  ssl_cert_path?: string;
  ssl_key_path?: string;
  ssl_issuer?: string;
  ssl_not_after?: number;
}

export interface DockerDiscoveredItem {
  project_name: string;
  compose_path: string;
  working_dir: string;
  containers: string[];
  running: boolean;
}

export interface PM2DiscoveredItem {
  id: number;
  name: string;
  script: string;
  working_dir: string;
  status: string;
  instances: number;
}

export interface DatabaseDiscoveredItem {
  engine: string;
  port: number;
  running: boolean;
  databases: string[];
  adopted?: string[];
}

export interface DiscoveryResult {
  nginx_vhosts: NginxDiscoveredItem[];
  docker_projects: DockerDiscoveredItem[];
  pm2_apps: PM2DiscoveredItem[];
  databases: DatabaseDiscoveredItem[];
  kubernetes_detected: boolean;
  kubernetes_type: string;
}

export interface ServerTimeInfo {
  unix: number;
  iso: string;
  timezone_name: string;
  offset_sec: number;
}

// ── Web Application Firewall (WAF) ──────────────────────────────────────────
export interface WAFSite {
  site_name: string;
  total_interceptions: number;
  fake_spider: boolean;
  using_cdn: boolean;
  smart_cc: number;
  crawl_protection: boolean;
  rate_limit: boolean;
  cc_defense: number;
  status: boolean;
  updated_at: number;
}

export interface WAFRule {
  id: number;
  rule_type: 'ip_whitelist' | 'ip_blacklist' | 'ua_whitelist' | 'ua_blacklist' | 'url_whitelist' | 'url_blacklist';
  ip_version: 'IPv4' | 'IPv6';
  start_ip?: string;
  end_ip?: string;
  pattern?: string;
  remarks?: string;
  created_at: number;
}

export interface WAFEvent {
  id: number;
  site_name: string;
  client_ip: string;
  country: string;
  country_code: string;
  region?: string;
  city?: string;
  isp?: string;
  latitude: number;
  longitude: number;
  device?: string;
  os?: string;
  browser?: string;
  method?: string;
  attack_type: string;
  action: 'block' | 'intercept' | 'challenge';
  url_path: string;
  user_agent?: string;
  rule_name?: string;
  status_code: number;
  created_at: number;
}

export interface WAFAccessLog {
  id: number;
  site_name: string;
  client_ip: string;
  country: string;
  country_code: string;
  region?: string;
  city?: string;
  isp?: string;
  latitude: number;
  longitude: number;
  device?: string;
  os?: string;
  browser?: string;
  method: string;
  url_path: string;
  protocol?: string;
  status_code: number;
  bytes_sent: number;
  referer?: string;
  user_agent?: string;
  is_threat: boolean;
  attack_type: string;
  action: 'allow' | 'block' | 'intercept' | 'challenge';
  rule_name?: string;
  created_at: number;
  timestamp?: number;
}

export interface WAFVisitorItem {
  ip: string;
  count: number;
  bytes_total: number;
  country: string;
  country_code: string;
  city?: string;
  isp?: string;
  device?: string;
  os?: string;
  browser?: string;
  threat_count: number;
  last_site?: string;
  last_path?: string;
  last_seen: number;
  requests?: number;
  threats?: number;
  bytes_sent?: number;
}

export interface WAFOverview {
  today_requests: number;
  malicious_requests: number;
  yesterday_requests: number;
  yesterday_malicious: number;
  total_requests_all_time?: number;
  total_bandwidth_bytes?: number;
  status_breakdown?: {
    s2xx: number;
    s3xx: number;
    s4xx: number;
    s5xx: number;
  };
  intercept_rate: number;
  realtime_qps: number;
  origin_latency_ms: number;
  top_attack_ips: {
    ip: string;
    count: number;
    country: string;
    country_code: string;
    region?: string;
    city?: string;
    isp?: string;
  }[];
  attacked_domains: { domain: string; count: number }[];
  traffic_ranking: {
    domain: string;
    requests: number;
    bandwidth_bytes?: number;
    bytes_sent?: number;
    threats?: number;
    status_2xx?: number;
    status_3xx?: number;
    status_4xx?: number;
    status_5xx?: number;
  }[];
  visited_pages: {
    site_name?: string;
    domain?: string;
    path: string;
    last_status?: number;
    bytes_sent?: number;
    method?: string;
    count: number;
    status_code?: number;
    bytes_total?: number;
    is_threat?: boolean;
  }[];
  top_visitors?: WAFVisitorItem[];
  recent_events: WAFEvent[];
  recent_access_logs?: WAFAccessLog[];
  total_rules_count: number;
  waf_global_enabled: boolean;
  host_location?: WAFHostLocation;
  enforcement?: WAFEnforcementStatus;
}

export interface WAFHostLocation {
  ip: string;
  country: string;
  country_code: string;
  city: string;
  isp: string;
  latitude: number;
  longitude: number;
  label: string;
}

export interface WAFEnforcementStatus {
  global_enabled: boolean;
  nginx_enforced: boolean;
  config_path: string;
  active_rules_count: number;
  protected_sites_count: number;
  last_synced_at: number;
  last_sync_error?: string;
  config_preview?: string;
}

// ── Cloudflare Integration ──────────────────────────────────────────────────
export interface CloudflareConfig {
  connected: boolean;
  api_token_masked?: string;
  email?: string;
  account_name?: string;
  account_id?: string;
  auth_mode?: 'api_token' | 'global_key';
  last_sync?: number;
}

export interface CloudflareDNSRecord {
  id: string;
  type: string; // A, AAAA, CNAME, TXT, MX, etc.
  name: string;
  content: string;
  proxied: boolean;
  proxiable?: boolean;
  ttl: number;
  priority?: number;
  points_to_server: boolean;
  in_local_sites?: boolean;
}

export interface CloudflareZone {
  id: string;
  name: string;
  status: string;
  paused: boolean;
  development_mode: number;
  ssl_mode?: string;
  security_level?: string;
  always_use_https?: boolean;
  auto_https_rewrites?: boolean;
  brotli?: boolean;
  tls_1_3?: boolean;
  name_servers?: string[];
  dns_records?: CloudflareDNSRecord[];
  points_to_server?: boolean;
  matched_records?: string[];
  local_sites?: string[];
  source?: 'cloudflare_api' | 'local_dns_fallback';
  read_only?: boolean;
}

export interface GlobalPackage {
  name: string;
  description: string;
  installed: boolean;
}

// ── Web Terminal, Remote SSH & Command Shortcuts ───────────────────────────
export interface TerminalServer {
  id: number;
  name: string;
  host: string;
  port: number;
  username: string;
  auth_type: 'password' | 'key';
  has_password?: boolean;
  has_key?: boolean;
  host_key_fingerprint?: string;
  password?: string;
  private_key?: string;
  passphrase?: string;
  remarks: string;
  created_at?: number;
  updated_at?: number;
}

export interface TerminalCommand {
  id: number;
  name: string;
  content: string;
  created_at?: number;
  updated_at?: number;
}

export interface TerminalCompletionItem {
  name: string;
  display: string;
  replacement: string;
  is_dir: boolean;
  mode: string;
  size: number;
  mod_time: string;
  detail: string;
}

export interface TerminalHistoryItem {
  command: string;
  output: string;
  time: string;
  ok?: boolean;
  cwd?: string;
  exitCode?: number;
  durationMs?: number;
  streaming?: boolean;
  isCompletionList?: boolean;
}

export type TerminalMode = 'pty' | 'console';

export interface TerminalTab {
  id: string;
  serverId: number; // 0 = lokal
  name: string;
  host: string;
  username: string;
  cwd?: string;
  mode: TerminalMode;
  connected: boolean;
  ptyStatus?: 'connecting' | 'connected' | 'disconnected' | 'error';
  fingerprint?: string;
  history: TerminalHistoryItem[];
  commandInput: string;
  draftInput?: string;
  historyIndex: number;
  executing: boolean;
}

// ── Uptime & SSL Monitor ──────────────────────────────────────────────────
export interface UptimeCheck {
  id: number;
  monitor_id: number;
  status: 'up' | 'down' | string;
  status_code: number;
  latency_ms: number;
  ssl_days: number;
  error_msg?: string;
  checked_at: number;
}

export interface UptimeMonitor {
  id: number;
  name: string;
  target: string;
  monitor_type: 'http' | 'tcp';
  interval_sec: number;
  timeout_sec: number;
  expected_status: number;
  keyword: string;
  notify_webhook: boolean;
  public_visible: boolean;
  enabled: boolean;
  last_status: 'up' | 'down' | 'pending' | string;
  last_code: number;
  last_latency_ms: number;
  last_error: string;
  ssl_days_remaining: number;
  ssl_issuer: string;
  last_checked_at: number;
  last_notified_at: number;
  created_at: number;
  uptime_24h_pct: number;
  avg_latency_ms: number;
  recent_checks: UptimeCheck[];
}

// Konfigurasi Auto-Deploy Webhook & Riwayat Deployment per Situs
export interface SiteDeployConfig {
  site_id: number;
  webhook_secret: string;
  webhook_enabled: boolean;
  auto_optimize: boolean;
  maintenance_mode: boolean;
  rebuild_assets: boolean;
  branch: string;
  updated_at: number;
}

export interface SiteDeploymentItem {
  id: number;
  site_id: number;
  trigger: 'initial' | 'manual' | 'webhook' | 'rollback' | string;
  branch: string;
  old_commit: string;
  new_commit: string;
  commit_message: string;
  status: 'success' | 'failed' | 'warning' | 'running' | string;
  duration_ms: number;
  output: string;
  created_at: number;
}

export interface SiteDeployInfo {
  ok: boolean;
  config: SiteDeployConfig;
  webhook_url: string;
  has_git: boolean;
  branch: string;
  current_commit: string;
  is_laravel: boolean;
  is_filament: boolean;
  php_binary: string;
  scheduler_enabled: boolean;
  scheduler_cron_id: number;
  deployments: SiteDeploymentItem[];
}



