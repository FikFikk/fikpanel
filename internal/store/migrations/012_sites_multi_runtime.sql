-- Migrasi 012: Perluas CHECK constraint kolom kind pada tabel sites untuk mendukung runtime node, go, dan python
CREATE TABLE IF NOT EXISTS certificates_backup AS SELECT * FROM certificates;
DROP TABLE certificates;

CREATE TABLE IF NOT EXISTS sites_new (
  id          INTEGER PRIMARY KEY,
  domain      TEXT NOT NULL UNIQUE,
  aliases     TEXT NOT NULL DEFAULT '[]',
  kind        TEXT NOT NULL CHECK (kind IN ('proxy','static','php','node','go','python')),
  upstream    TEXT,
  root_path   TEXT,
  ssl_mode    TEXT NOT NULL DEFAULT 'none' CHECK (ssl_mode IN ('none','letsencrypt','custom')),
  force_https INTEGER NOT NULL DEFAULT 1,
  extra_conf  TEXT NOT NULL DEFAULT '',
  enabled     INTEGER NOT NULL DEFAULT 1,
  updated_at  INTEGER NOT NULL,
  hsts        INTEGER NOT NULL DEFAULT 0,
  php_version TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT ''
);

INSERT INTO sites_new (
  id, domain, aliases, kind, upstream, root_path, ssl_mode, force_https,
  extra_conf, enabled, updated_at, hsts, php_version, description
)
SELECT
  id, domain, aliases, kind, upstream, root_path, ssl_mode, force_https,
  extra_conf, enabled, updated_at, hsts, php_version, description
FROM sites;

DROP TABLE sites;
ALTER TABLE sites_new RENAME TO sites;

CREATE TABLE IF NOT EXISTS certificates (
  id          INTEGER PRIMARY KEY,
  site_id     INTEGER NOT NULL UNIQUE REFERENCES sites(id) ON DELETE CASCADE,
  issuer      TEXT NOT NULL,
  not_after   INTEGER NOT NULL,
  last_error  TEXT,
  renewed_at  INTEGER,
  status      TEXT NOT NULL DEFAULT 'none',
  not_before  INTEGER NOT NULL DEFAULT 0,
  domains     TEXT NOT NULL DEFAULT '[]',
  signature   TEXT NOT NULL DEFAULT '',
  key_type    TEXT NOT NULL DEFAULT ''
);

INSERT INTO certificates (
  id, site_id, issuer, not_after, last_error, renewed_at,
  status, not_before, domains, signature, key_type
)
SELECT
  id, site_id, issuer, not_after, last_error, renewed_at,
  COALESCE(status, 'none'), COALESCE(not_before, 0), COALESCE(domains, '[]'),
  COALESCE(signature, ''), COALESCE(key_type, '')
FROM certificates_backup;

DROP TABLE certificates_backup;

