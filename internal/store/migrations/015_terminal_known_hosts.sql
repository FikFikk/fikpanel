-- Migrasi 015: Tabel verifikasi host key SSH (TOFU - Trust On First Use)
CREATE TABLE IF NOT EXISTS terminal_known_hosts (
    host_port TEXT PRIMARY KEY,
    key_type TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    public_key TEXT NOT NULL,
    first_seen INTEGER NOT NULL,
    last_seen INTEGER NOT NULL
);
