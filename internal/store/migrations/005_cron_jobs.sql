-- Migrasi 005: Cron Job Scheduler & Execution History

CREATE TABLE IF NOT EXISTS cron_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    schedule_type TEXT NOT NULL,       -- 'minute_n', 'hourly', 'daily', 'weekly', 'monthly', 'custom'
    spec TEXT NOT NULL,                -- standard cron expression misal: "0 2 * * *" atau "*/15 * * *"
    command TEXT NOT NULL,             -- shell command / script path
    enabled INTEGER NOT NULL DEFAULT 1, -- 1: active, 0: paused
    save_log INTEGER NOT NULL DEFAULT 1,
    timeout_sec INTEGER NOT NULL DEFAULT 3600,
    last_run_at INTEGER DEFAULT 0,
    last_status TEXT DEFAULT '',       -- 'success', 'failed', 'running'
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cron_jobs_enabled ON cron_jobs(enabled);

CREATE TABLE IF NOT EXISTS cron_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL,
    status TEXT NOT NULL,              -- 'success', 'failed', 'timeout'
    output TEXT DEFAULT '',
    exit_code INTEGER DEFAULT 0,
    duration_ms INTEGER DEFAULT 0,
    started_at INTEGER NOT NULL,
    finished_at INTEGER NOT NULL,
    FOREIGN KEY(job_id) REFERENCES cron_jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_cron_logs_job ON cron_logs(job_id, started_at DESC);
