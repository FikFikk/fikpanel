-- Migrasi 006: Backup Configurations & S3 Storage Integrations

CREATE TABLE IF NOT EXISTS backup_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    target_type TEXT NOT NULL,          -- 'database', 'site', 'directory'
    target_name TEXT NOT NULL,          -- nama database atau domain website
    storage_type TEXT NOT NULL,         -- 'local', 's3'
    s3_endpoint TEXT DEFAULT '',
    s3_bucket TEXT DEFAULT '',
    s3_access_key TEXT DEFAULT '',
    s3_secret_key TEXT DEFAULT '',
    s3_region TEXT DEFAULT 'us-east-1',
    keep_copies INTEGER DEFAULT 5,
    auto_cron INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS backup_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    file_size_bytes INTEGER DEFAULT 0,
    storage_type TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    status TEXT NOT NULL,               -- 'success', 'failed', 'running'
    error_message TEXT DEFAULT '',
    created_at INTEGER NOT NULL,
    FOREIGN KEY(config_id) REFERENCES backup_configs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_backup_records_config ON backup_records(config_id, created_at DESC);
