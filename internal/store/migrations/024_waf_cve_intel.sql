-- Migrasi 024: Cache CVE Threat Intelligence Feed & Status 1-Klik Virtual Patching WAF
CREATE TABLE IF NOT EXISTS waf_cve_cache (
    cve_id TEXT PRIMARY KEY,
    ghsa_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL DEFAULT 'high',
    cvss_score REAL NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'builtin',
    product TEXT NOT NULL DEFAULT 'web',
    affected_range TEXT NOT NULL DEFAULT '',
    patched_version TEXT NOT NULL DEFAULT '',
    published_at TEXT NOT NULL DEFAULT '',
    is_kev INTEGER NOT NULL DEFAULT 0,
    reference_url TEXT NOT NULL DEFAULT '',
    virtual_patch_id TEXT NOT NULL DEFAULT '',
    fetched_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_waf_cve_published ON waf_cve_cache(is_kev DESC, published_at DESC, cvss_score DESC);
CREATE INDEX IF NOT EXISTS idx_waf_cve_product ON waf_cve_cache(product);

CREATE TABLE IF NOT EXISTS waf_virtual_patches (
    patch_id TEXT PRIMARY KEY,
    enabled INTEGER NOT NULL DEFAULT 1,
    updated_at INTEGER NOT NULL
);
