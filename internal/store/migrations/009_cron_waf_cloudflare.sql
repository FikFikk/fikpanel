-- Migrasi 009: Penambahan Berkas Log Cron Dinamis, Modul WAF Terpadu, dan Integrasi Cloudflare

-- 1. Tambah kolom log_file pada tabel cron_jobs
ALTER TABLE cron_jobs ADD COLUMN log_file TEXT DEFAULT '';

-- 2. Tabel konfigurasi situs terproteksi WAF
CREATE TABLE IF NOT EXISTS waf_sites (
    site_name TEXT PRIMARY KEY,
    total_interceptions INTEGER DEFAULT 0,
    fake_spider INTEGER DEFAULT 1,
    using_cdn INTEGER DEFAULT 0,
    smart_cc INTEGER DEFAULT 100,
    crawl_protection INTEGER DEFAULT 0,
    rate_limit INTEGER DEFAULT 0,
    cc_defense INTEGER DEFAULT 1,
    status INTEGER DEFAULT 1,
    updated_at INTEGER NOT NULL
);

-- 3. Tabel aturan daftar hitam / daftar putih WAF
CREATE TABLE IF NOT EXISTS waf_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_type TEXT NOT NULL, -- 'ip_whitelist', 'ip_blacklist', 'ua_whitelist', 'ua_blacklist', 'url_whitelist', 'url_blacklist'
    ip_version TEXT DEFAULT 'IPv4', -- 'IPv4', 'IPv6'
    start_ip TEXT DEFAULT '',
    end_ip TEXT DEFAULT '',
    pattern TEXT DEFAULT '',
    remarks TEXT DEFAULT '',
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_waf_rules_type ON waf_rules(rule_type);

-- 4. Tabel rekaman kejadian serangan WAF (untuk Attack Map & Laporan)
CREATE TABLE IF NOT EXISTS waf_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_name TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    country TEXT NOT NULL,
    country_code TEXT NOT NULL,
    latitude REAL DEFAULT 0,
    longitude REAL DEFAULT 0,
    attack_type TEXT NOT NULL, -- 'SQL Injection', 'XSS', 'CC Flood', 'Path Traversal', 'Fake Spider', 'Bad Bot', 'Blocked IP'
    action TEXT NOT NULL, -- 'block', 'intercept', 'challenge'
    url_path TEXT NOT NULL,
    user_agent TEXT DEFAULT '',
    status_code INTEGER DEFAULT 403,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_waf_events_created ON waf_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_waf_events_site ON waf_events(site_name);

-- 5. Tabel konfigurasi koneksi Cloudflare
CREATE TABLE IF NOT EXISTS cloudflare_configs (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    api_token TEXT NOT NULL,
    email TEXT DEFAULT '',
    connected INTEGER DEFAULT 0,
    account_name TEXT DEFAULT '',
    account_id TEXT DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
