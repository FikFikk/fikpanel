-- Kolom tambahan situs: HSTS, versi PHP, dan deskripsi
ALTER TABLE sites ADD COLUMN hsts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sites ADD COLUMN php_version TEXT NOT NULL DEFAULT '';
ALTER TABLE sites ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- Metadata sertifikat lengkap untuk tampilan detail SSL
ALTER TABLE certificates ADD COLUMN status TEXT NOT NULL DEFAULT 'none';  -- none | pending | issued | failed
ALTER TABLE certificates ADD COLUMN not_before INTEGER NOT NULL DEFAULT 0;
ALTER TABLE certificates ADD COLUMN domains TEXT NOT NULL DEFAULT '[]';   -- JSON array SAN
ALTER TABLE certificates ADD COLUMN signature TEXT NOT NULL DEFAULT '';
ALTER TABLE certificates ADD COLUMN key_type TEXT NOT NULL DEFAULT '';
