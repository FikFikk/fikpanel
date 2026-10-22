package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
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

// migrate menjalankan seluruh file migrasi SQL ter-embed
func (s *Store) migrate(ctx context.Context) error {
	// Baca file migrasi awal 001_init.sql
	content, err := migrationFS.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("gagal membaca file migrasi 001: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi migrasi: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("gagal mengeksekusi skrip migrasi: %w", err)
	}

	return tx.Commit()
}

