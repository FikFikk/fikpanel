-- Migrasi 019: Dukungan tantangan ACME DNS-01 (Wildcard SSL via go-acme/lego) dan penyimpanan kredensial DNS provider
ALTER TABLE certificates ADD COLUMN challenge_type TEXT NOT NULL DEFAULT 'http01';
ALTER TABLE certificates ADD COLUMN dns_provider TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS acme_dns_providers (
    provider TEXT PRIMARY KEY,
    credentials_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
