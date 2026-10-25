-- Migrasi 004: Manajemen Database dan Antrean Tugas Sistem (Task Queue)

CREATE TABLE IF NOT EXISTS databases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    engine TEXT NOT NULL,              -- 'mysql', 'postgresql', 'mongodb', 'redis'
    name TEXT NOT NULL,
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    size_bytes INTEGER DEFAULT 0,
    backup_count INTEGER DEFAULT 0,
    location TEXT DEFAULT 'localhost',
    note TEXT DEFAULT '',
    created_at INTEGER NOT NULL,
    UNIQUE(engine, name)
);

CREATE INDEX IF NOT EXISTS idx_databases_engine ON databases(engine);

CREATE TABLE IF NOT EXISTS system_tasks (
    id TEXT PRIMARY KEY,               -- task_uuid
    type TEXT NOT NULL,                -- 'software_install', 'db_backup', dll
    target TEXT NOT NULL,              -- misal: 'mongodb-7.0'
    status TEXT NOT NULL,              -- 'pending', 'running', 'success', 'failed', 'cancelled'
    log TEXT DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON system_tasks(status);

-- Data bawaan untuk demonstrasi engine MySQL / MariaDB (menyerupai data referensi aaPanel)
INSERT OR IGNORE INTO databases (id, engine, name, username, password, size_bytes, backup_count, location, note, created_at)
VALUES 
    (1, 'mysql', 'sql_fikpanel_app', 'usr_fikpanel', 'pAssw0rdFik_99x!', 18874368, 1, 'localhost', 'Aplikasi Web Utama FikPanel', strftime('%s', 'now')),
    (2, 'mysql', 'sql_demo_portal', 'usr_demo', 'd3mo_Secur3Key#', 4194304, 0, 'localhost', 'Portal Berita & Dokumentasi', strftime('%s', 'now'));
