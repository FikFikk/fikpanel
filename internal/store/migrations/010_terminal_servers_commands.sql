-- Migrasi 010: Tabel Server Remote dan Pintasan Perintah Terminal

CREATE TABLE IF NOT EXISTS terminal_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 22,
    username TEXT NOT NULL DEFAULT 'root',
    auth_type TEXT NOT NULL DEFAULT 'password',
    password TEXT NOT NULL DEFAULT '',
    private_key TEXT NOT NULL DEFAULT '',
    passphrase TEXT NOT NULL DEFAULT '',
    remarks TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS terminal_commands (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Perintah bawaan sistem yang siap pakai
INSERT INTO terminal_commands (name, content, created_at, updated_at) VALUES 
('Uptime & Load Average', 'uptime', unixepoch(), unixepoch()),
('Memory Usage (RAM)', 'free -h', unixepoch(), unixepoch()),
('Disk Space Usage', 'df -h', unixepoch(), unixepoch()),
('OS & Kernel Version', 'uname -a', unixepoch(), unixepoch()),
('Top 10 CPU Processes', 'ps aux --sort=-%cpu | head -n 11', unixepoch(), unixepoch()),
('Top 10 Memory Processes', 'ps aux --sort=-%mem | head -n 11', unixepoch(), unixepoch()),
('Active Network Ports', 'ss -tulpn', unixepoch(), unixepoch()),
('Docker Containers Status', 'docker ps -a', unixepoch(), unixepoch());
