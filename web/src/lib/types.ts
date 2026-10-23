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

export interface Site {
  id: number;
  domain: string;
  aliases: string;
  kind: 'proxy' | 'static' | 'php';
  upstream: string;
  root_path: string;
  ssl_mode: string;
  force_https: number;
  extra_conf: string;
  enabled: number;
  updated_at: number;
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