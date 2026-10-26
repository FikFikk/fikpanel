-- Migration 008: Persistent System Monitoring (aaPanel style)
CREATE TABLE IF NOT EXISTS monitor_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Simpan konfigurasi awal: monitoring aktif secara default dengan interval perekaman 60 detik (retensi 30 hari)
INSERT OR IGNORE INTO monitor_settings (key, value, updated_at) VALUES ('enabled', '1', unixepoch());
INSERT OR IGNORE INTO monitor_settings (key, value, updated_at) VALUES ('save_days', '30', unixepoch());

CREATE TABLE IF NOT EXISTS monitor_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    cpu REAL NOT NULL,
    ram_used INTEGER NOT NULL,
    ram_total INTEGER NOT NULL,
    load_1m REAL NOT NULL,
    load_5m REAL NOT NULL,
    load_15m REAL NOT NULL,
    disk_used INTEGER NOT NULL,
    disk_total INTEGER NOT NULL,
    net_in INTEGER NOT NULL,
    net_out INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_monitor_snapshots_ts ON monitor_snapshots(timestamp);
