-- Migrasi 007: DNS Manager, PHP Manager, System Tuning & Webhook Notifications

CREATE TABLE IF NOT EXISTS dns_zones (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,          -- misal: example.com
    ttl INTEGER DEFAULT 3600,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS dns_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_id INTEGER NOT NULL,
    type TEXT NOT NULL,                 -- 'A', 'AAAA', 'CNAME', 'TXT', 'MX', 'NS'
    name TEXT NOT NULL,                 -- '@', 'www', 'mail', dll
    content TEXT NOT NULL,              -- IP atau target domain / teks
    priority INTEGER DEFAULT 0,
    ttl INTEGER DEFAULT 3600,
    created_at INTEGER NOT NULL,
    FOREIGN KEY(zone_id) REFERENCES dns_zones(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_dns_records_zone ON dns_records(zone_id);

CREATE TABLE IF NOT EXISTS system_webhooks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    channel TEXT NOT NULL,              -- 'telegram', 'discord', 'generic'
    webhook_url TEXT NOT NULL,
    notify_on_high_cpu INTEGER DEFAULT 1,
    notify_on_high_ram INTEGER DEFAULT 1,
    notify_on_backup INTEGER DEFAULT 1,
    notify_on_ssl_expiry INTEGER DEFAULT 1,
    created_at INTEGER NOT NULL
);
