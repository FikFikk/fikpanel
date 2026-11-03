-- Migrasi 014: Tambah kolom framework pada tabel sites untuk mencatat metadata framework proyek (Laravel, Nuxt, Next.js, Hono, Go Service, dll.)
ALTER TABLE sites ADD COLUMN framework TEXT NOT NULL DEFAULT '';
