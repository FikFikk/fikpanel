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

export type SiteKind = 'proxy' | 'static' | 'php';
export type SSLStatus = 'none' | 'pending' | 'issued' | 'failed';

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
  ssl_status: SSLStatus;
  ssl_expires: number;
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
  label: string;
  active_state: string;
  sub_state: string;
  main_pid: number;
  memory_current: number;
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
  port: number;
  password_auth: boolean;
  permit_root_login: boolean;
  pubkey_auth: boolean;
  active_service: boolean;
}

export interface Fail2BanJail {
  name: string;
  currently_failed: number;
  total_failed: number;
  banned_count: number;
  banned_ips: string[];
}

export interface Fail2BanStatus {
  installed: boolean;
  running: boolean;
  jails: Fail2BanJail[];
}

export interface ScanFinding {
  category: 'baseline' | 'risk' | 'malware' | 'vuln';
  severity: 'critical' | 'warning' | 'safe';
  title: string;
  detail: string;
  fix_hint: string;
}

export interface SecurityScanReport {
  score: number;
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
}

// ── PHP Versions ──────────────────────────────────────────────────────────
export interface PHPVersionInfo {
  version: string;
  installed: boolean;
  running: boolean;
  fpm_socket: string;
  ini_path: string;
  settings: Record<string, string>;
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
export interface PortRule {
  number: number;
  rule_id?: string;
  rule_type?: string;
  ip_version?: string;
  protocol: string;
  port: string;
  status: string;
  strategy: string;
  direction: string;
  source_ip: string;
  remarks: string;
  add_time: string;
}

export interface IPRule {
  number: number;
  ip: string;
  strategy: string;
  direction: string;
  remarks: string;
  add_time: string;
}

export interface FirewallExtendedStatus {
  enabled: boolean;
  block_icmp: boolean;
  port_rules_count: number;
  ip_rules_count: number;
  port_forward_count: number;
  area_rules_count: number;
  rules: PortRule[];
  ip_rules: IPRule[];
  site_logs_path: string;
  site_logs_size: string;
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
}

export interface WhitelistIP {
  ip: string;
  remarks: string;
  added_at: number;
}

export interface BlacklistIP {
  ip: string;
  reason: string;
  added_at: number;
  expires_at: number;
}

export interface FailedLoginEvent {
  time: number;
  ip: string;
  username: string;
  reason: string;
  source: 'panel' | 'ssh';
}

export interface SystemAccount {
  username: string;
  uid: number;
  gid: number;
  shell: string;
  home_dir: string;
  locked: boolean;
}

export interface SSHLog {
  timestamp: number;
  user: string;
  ip: string;
  port: number;
  status: 'success' | 'failed';
  message: string;
}

export interface SSHStats {
  service_active: boolean;
  port: number;
  password_auth: boolean;
  key_auth: boolean;
  permit_root_login: boolean;
  successful_logins: number;
  failed_logins: number;
  today_success: number;
  today_fail: number;
  yesterday_success: number;
  yesterday_fail: number;
  seven_days_success: number[];
  seven_days_fail: number[];
  recent_logs: SSHLog[];
}

export interface SecurityChecklistItem {
  id: string;
  title: string;
  description: string;
  status: 'safe' | 'warning' | 'danger';
  fixable: boolean;
  action_key: string;
}

export interface LastLoginInfo {
  ip: string;
  time: string;
  port: number;
  success: boolean;
}

export interface ServerSecurityOverview {
  score: number;
  rating: string;
  last_ssh_login: LastLoginInfo;
  last_panel_login: LastLoginInfo;
  checklist: SecurityChecklistItem[];
}

export interface ProtectionItem {
  installed: boolean;
  enabled: boolean;
  summary: string;
}

export interface ProtectionOverview {
  firewall: ProtectionItem;
  ssh_protection: ProtectionItem;
  nginx_waf: ProtectionItem;
  tamper_proof: ProtectionItem;
  intrusion_detection: ProtectionItem;
  fail2ban: ProtectionItem;
  enabled_count: number;
  total_count: number;
  client_ip: string;
  is_client_whitelisted: boolean;
}

export interface HardeningStatus {
  compiler_blocked: boolean;
  icmp_blocked: boolean;
  syn_flood_protected: boolean;
  ip_spoofing_protected: boolean;
  core_dump_disabled: boolean;
  tamper_proof_enabled: boolean;
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
  schedule_type: 'minute_n' | 'hourly' | 'daily' | 'weekly' | 'monthly' | 'custom';
  spec: string;
  command: string;
  enabled: boolean;
  save_log: boolean;
  timeout_sec: number;
  last_run_at: number;
  last_status: string;
  created_at: number;
  updated_at: number;
}

export interface CronLog {
  id: number;
  job_id: number;
  status: 'success' | 'failed' | 'timeout';
  output: string;
  exit_code: number;
  duration_ms: number;
  started_at: number;
  finished_at: number;
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
  root_dir: string;
  port: number;
  ssl: boolean;
  proxy_pass: string;
  is_adopted: boolean;
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
}

export interface DiscoveryResult {
  nginx_vhosts: NginxDiscoveredItem[];
  docker_projects: DockerDiscoveredItem[];
  pm2_apps: PM2DiscoveredItem[];
  databases: DatabaseDiscoveredItem[];
  kubernetes_detected: boolean;
  kubernetes_type: string;
}

// ── Cron Scheduler ────────────────────────────────────────────────────────
export interface CronJob {
  id: number;
  name: string;
  schedule_type: string;
  spec: string;
  command: string;
  enabled: boolean;
  save_log: boolean;
  timeout_sec: number;
  last_run_at: number;
  last_status: string;
  created_at: number;
  updated_at: number;
}

export interface CronLog {
  id: number;
  job_id: number;
  started_at: number;
  finished_at: number;
  duration_ms: number;
  status: string;
  exit_code: number;
  output: string;
}

export interface ServerTimeInfo {
  unix: number;
  iso: string;
  timezone_name: string;
  offset_sec: number;
}

export interface GlobalPackage {
  name: string;
  description: string;
  installed: boolean;
}
