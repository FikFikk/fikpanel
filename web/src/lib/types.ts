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