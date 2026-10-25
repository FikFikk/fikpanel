-- Migrasi 003: Fleksibilitas provider AI (OpenAI-compatible, Anthropic, Gemini) dan pembaruan riwayat chat
CREATE TABLE IF NOT EXISTS ai_providers_new (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,
  kind        TEXT NOT NULL CHECK (kind IN ('openai_compat','anthropic','gemini')),
  base_url    TEXT NOT NULL,
  model       TEXT NOT NULL,
  api_key     TEXT NOT NULL DEFAULT '',
  is_default  INTEGER NOT NULL DEFAULT 0,
  created_at  INTEGER NOT NULL DEFAULT 0,
  updated_at  INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO ai_providers_new (id, name, kind, base_url, model, api_key, is_default, created_at, updated_at)
  SELECT id, name, kind, base_url, model, COALESCE(CAST(api_key_enc AS TEXT), ''), is_default, strftime('%s','now'), strftime('%s','now') FROM ai_providers;

DROP TABLE ai_providers;
ALTER TABLE ai_providers_new RENAME TO ai_providers;

-- Kolom timestamp pembaruan untuk pengurutan riwayat percakapan terbaru
ALTER TABLE ai_conversations ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0;
