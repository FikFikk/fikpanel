-- Tambah kolom notifikasi kegagalan webhook dan direktori kerja pada tabel cron_jobs
ALTER TABLE cron_jobs ADD COLUMN notify_on_fail INTEGER NOT NULL DEFAULT 1;
ALTER TABLE cron_jobs ADD COLUMN working_dir TEXT NOT NULL DEFAULT '';
