-- Skema persistensi Security Center: daftar IP, log autentikasi, riwayat pemindaian, dan File Integrity Monitor (FIM)
CREATE TABLE IF NOT EXISTS security_ip_lists (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  list_type  TEXT NOT NULL CHECK (list_type IN ('whitelist', 'blacklist')),
  ip         TEXT NOT NULL,
  remarks    TEXT NOT NULL DEFAULT '',
  source     TEXT NOT NULL DEFAULT 'manual',
  sync_ufw   INTEGER NOT NULL DEFAULT 0,
  added_at   INTEGER NOT NULL,
  expires_at INTEGER NOT NULL DEFAULT 0,
  UNIQUE(list_type, ip)
);
CREATE INDEX IF NOT EXISTS idx_sec_ip_type_exp ON security_ip_lists(list_type, expires_at);

CREATE TABLE IF NOT EXISTS security_login_events (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  ts         INTEGER NOT NULL,
  ip         TEXT NOT NULL,
  username   TEXT NOT NULL DEFAULT '',
  source     TEXT NOT NULL DEFAULT 'panel' CHECK (source IN ('panel', 'ssh')),
  status     TEXT NOT NULL DEFAULT 'failed' CHECK (status IN ('failed', 'success', 'banned')),
  reason     TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_sec_login_ts ON security_login_events(ts DESC);
CREATE INDEX IF NOT EXISTS idx_sec_login_ip ON security_login_events(source, status, ip, ts);
CREATE INDEX IF NOT EXISTS idx_sec_login_user ON security_login_events(source, status, username, ts);

CREATE TABLE IF NOT EXISTS security_scan_reports (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  scanned_at     INTEGER NOT NULL,
  score          INTEGER NOT NULL,
  rating         TEXT NOT NULL,
  risk_count     INTEGER NOT NULL DEFAULT 0,
  malware_count  INTEGER NOT NULL DEFAULT 0,
  vuln_count     INTEGER NOT NULL DEFAULT 0,
  baseline_pass  INTEGER NOT NULL DEFAULT 0,
  baseline_total INTEGER NOT NULL DEFAULT 0,
  findings_json  TEXT NOT NULL DEFAULT '[]'
);
CREATE INDEX IF NOT EXISTS idx_sec_scan_ts ON security_scan_reports(scanned_at DESC);

CREATE TABLE IF NOT EXISTS security_fim_baselines (
  path       TEXT PRIMARY KEY,
  sha256     TEXT NOT NULL,
  size       INTEGER NOT NULL,
  mode       TEXT NOT NULL,
  mod_time   INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- Pastikan alamat loopback selalu masuk daftar putih bawaan agar akses internal tidak terblokir
INSERT OR IGNORE INTO security_ip_lists (list_type, ip, remarks, source, sync_ufw, added_at, expires_at)
VALUES ('whitelist', '127.0.0.1', 'Loopback internal host', 'system', 0, strftime('%s', 'now'), 0);
