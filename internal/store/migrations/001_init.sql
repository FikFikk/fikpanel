-- Skema Database Awal fikpanel
CREATE TABLE IF NOT EXISTS users (
  id            INTEGER PRIMARY KEY,
  username      TEXT NOT NULL UNIQUE CHECK (length(username) BETWEEN 3 AND 32),
  password_hash TEXT NOT NULL,                 -- argon2id encoded string
  totp_secret   BLOB,                          -- terenkripsi AES-GCM, NULL = off
  role          TEXT NOT NULL DEFAULT 'admin' CHECK (role IN ('admin','viewer')),
  created_at    INTEGER NOT NULL,
  last_login_at INTEGER
);

CREATE TABLE IF NOT EXISTS sessions (
  token_hash  BLOB PRIMARY KEY,                -- sha256(token)
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER NOT NULL,
  sudo_until  INTEGER,                         -- re-auth window untuk aksi sensitif
  ip          TEXT,
  user_agent  TEXT
);

CREATE TABLE IF NOT EXISTS sites (
  id          INTEGER PRIMARY KEY,
  domain      TEXT NOT NULL UNIQUE,
  aliases     TEXT NOT NULL DEFAULT '[]',      -- JSON array
  kind        TEXT NOT NULL CHECK (kind IN ('proxy','static','php')),
  upstream    TEXT,                            -- misal: http://127.0.0.1:3000
  root_path   TEXT,                            -- direktori web root (static/php)
  ssl_mode    TEXT NOT NULL DEFAULT 'none' CHECK (ssl_mode IN ('none','letsencrypt','custom')),
  force_https INTEGER NOT NULL DEFAULT 1,
  extra_conf  TEXT NOT NULL DEFAULT '',        -- snippet custom yang divalidasi nginx -t
  enabled     INTEGER NOT NULL DEFAULT 1,
  updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS certificates (
  id          INTEGER PRIMARY KEY,
  site_id     INTEGER NOT NULL UNIQUE REFERENCES sites(id) ON DELETE CASCADE,
  issuer      TEXT NOT NULL,
  not_after   INTEGER NOT NULL,
  last_error  TEXT,
  renewed_at  INTEGER
);

CREATE TABLE IF NOT EXISTS managed_services (  -- allowlist unit systemd yang dikelola
  unit        TEXT PRIMARY KEY CHECK (unit GLOB '*.service'),
  label       TEXT,
  protected   INTEGER NOT NULL DEFAULT 0       -- 1 = stop/disable butuh sudo mode
);

CREATE TABLE IF NOT EXISTS audit_log (
  id          INTEGER PRIMARY KEY,
  ts          INTEGER NOT NULL,
  user_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
  actor       TEXT NOT NULL CHECK (actor IN ('user','system','ai')),
  action      TEXT NOT NULL,                   -- misal: 'site.create', 'service.restart'
  target      TEXT,
  ok          INTEGER NOT NULL,
  detail      TEXT                             -- maks 4096 byte
);
CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit_log(ts);

CREATE TABLE IF NOT EXISTS settings (          -- konfigurasi key-value kecil
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_providers (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,
  kind        TEXT NOT NULL CHECK (kind IN ('openai_compat','anthropic')),
  base_url    TEXT NOT NULL,
  model       TEXT NOT NULL,
  api_key_enc BLOB,                            -- AES-256-GCM, NULL untuk Ollama lokal
  is_default  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS ai_conversations (
  id         INTEGER PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title      TEXT,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_messages (
  id              INTEGER PRIMARY KEY,
  conversation_id INTEGER NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
  role            TEXT NOT NULL CHECK (role IN ('user','assistant','tool')),
  content         TEXT NOT NULL,               -- pesan sudah diredaksi dari secret
  tool_name       TEXT,
  ts              INTEGER NOT NULL
);

