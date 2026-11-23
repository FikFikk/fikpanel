-- Migrasi 017: Tabel Uptime & SSL Monitor beserta riwayat heartbeat pemeriksaan
CREATE TABLE IF NOT EXISTS uptime_monitors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    target TEXT NOT NULL,
    monitor_type TEXT NOT NULL DEFAULT 'http',
    interval_sec INTEGER NOT NULL DEFAULT 60,
    timeout_sec INTEGER NOT NULL DEFAULT 10,
    expected_status INTEGER NOT NULL DEFAULT 200,
    keyword TEXT NOT NULL DEFAULT '',
    notify_webhook INTEGER NOT NULL DEFAULT 1,
    public_visible INTEGER NOT NULL DEFAULT 1,
    enabled INTEGER NOT NULL DEFAULT 1,
    last_status TEXT NOT NULL DEFAULT 'pending',
    last_code INTEGER NOT NULL DEFAULT 0,
    last_latency_ms INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    ssl_days_remaining INTEGER NOT NULL DEFAULT -1,
    ssl_issuer TEXT NOT NULL DEFAULT '',
    last_checked_at INTEGER NOT NULL DEFAULT 0,
    last_notified_at INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS uptime_checks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    monitor_id INTEGER NOT NULL REFERENCES uptime_monitors(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    status_code INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    ssl_days INTEGER NOT NULL DEFAULT -1,
    error_msg TEXT NOT NULL DEFAULT '',
    checked_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_uptime_checks_monitor_ts ON uptime_checks(monitor_id, checked_at DESC);
