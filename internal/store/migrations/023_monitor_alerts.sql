-- Migrasi 023: Ekstensi kolom CPU IOWait/Steal & Disk I/O pada monitor_snapshots serta tabel monitor_alerts terpadu
ALTER TABLE monitor_snapshots ADD COLUMN cpu_iowait REAL NOT NULL DEFAULT 0;
ALTER TABLE monitor_snapshots ADD COLUMN cpu_steal REAL NOT NULL DEFAULT 0;
ALTER TABLE monitor_snapshots ADD COLUMN disk_read INTEGER NOT NULL DEFAULT 0;
ALTER TABLE monitor_snapshots ADD COLUMN disk_write INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS monitor_alerts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    severity TEXT NOT NULL,
    target TEXT NOT NULL,
    metric_value REAL NOT NULL DEFAULT 0,
    threshold REAL NOT NULL DEFAULT 0,
    message TEXT NOT NULL,
    notified INTEGER NOT NULL DEFAULT 0,
    acknowledged INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_monitor_alerts_ts ON monitor_alerts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_monitor_alerts_source ON monitor_alerts(source, created_at DESC);
