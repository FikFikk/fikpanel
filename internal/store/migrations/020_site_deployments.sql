-- Konfigurasi auto-deploy webhook, optimasi produksi, dan riwayat deployment per situs (Laravel/Filament/Git)
CREATE TABLE IF NOT EXISTS site_deploy_configs (
    site_id          INTEGER PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
    webhook_secret   TEXT    NOT NULL DEFAULT '',
    webhook_enabled  INTEGER NOT NULL DEFAULT 0,
    auto_optimize    INTEGER NOT NULL DEFAULT 1,
    maintenance_mode INTEGER NOT NULL DEFAULT 1,
    rebuild_assets   INTEGER NOT NULL DEFAULT 1,
    branch           TEXT    NOT NULL DEFAULT '',
    updated_at       INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS site_deployments (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id        INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    trigger        TEXT    NOT NULL DEFAULT 'manual',
    branch         TEXT    NOT NULL DEFAULT '',
    old_commit     TEXT    NOT NULL DEFAULT '',
    new_commit     TEXT    NOT NULL DEFAULT '',
    commit_message TEXT    NOT NULL DEFAULT '',
    status         TEXT    NOT NULL DEFAULT 'success',
    duration_ms    INTEGER NOT NULL DEFAULT 0,
    output         TEXT    NOT NULL DEFAULT '',
    created_at     INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_site_deployments_site_created
    ON site_deployments(site_id, created_at DESC, id DESC);
