package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store merepresentasikan koneksi database terkelola untuk fikpanel
type Store struct {
	db *sql.DB
}

// Open menginisialisasi koneksi SQLite dengan konfigurasi WAL dan PRAGMA keamanan
func Open(dbPath string) (*Store, error) {
	// Konfigurasi DSN dengan timeout busy dan mode WAL
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=cache_size(-2000)", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka database: %w", err)
	}

	// Membatasi koneksi writer tunggal untuk mencegah contention kunci SQLite
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("gagal verifikasi koneksi database: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("gagal menjalankan migrasi database: %w", err)
	}

	return s, nil
}

// DB mengembalikan objek *sql.DB untuk operasi query
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close menutup koneksi database SQLite
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate menjalankan seluruh file migrasi ter-embed secara berurutan dan mencatat versinya
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx,
		"CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)"); err != nil {
		return fmt.Errorf("gagal membuat tabel schema_migrations: %w", err)
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("gagal membaca folder migrasi: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	// Prefiks angka (001_, 002_, ...) menjamin urutan eksekusi leksikal
	sort.Strings(files)

	for _, name := range files {
		var exists int
		err := s.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM schema_migrations WHERE version = ?", name).Scan(&exists)
		if err != nil {
			return fmt.Errorf("gagal memeriksa status migrasi %s: %w", name, err)
		}
		if exists > 0 {
			continue
		}
		if err := s.applyMigration(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration mengeksekusi satu file migrasi dalam transaksi dan mencatat versinya
func (s *Store) applyMigration(ctx context.Context, name string) error {
	content, err := migrationFS.ReadFile("migrations/" + name)
	if err != nil {
		return fmt.Errorf("gagal membaca file migrasi %s: %w", name, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi migrasi %s: %w", name, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("gagal mengeksekusi migrasi %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", name, time.Now().Unix()); err != nil {
		return fmt.Errorf("gagal mencatat migrasi %s: %w", name, err)
	}
	return tx.Commit()
}

