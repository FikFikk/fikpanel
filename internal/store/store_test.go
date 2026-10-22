package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreOpenAndMigrate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_store_test_*")
	if err != nil {
		t.Fatalf("Gagal membuat direktori sementara: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_panel.db")

	// Uji pembukaan store dan eksekusi migrasi otomatis
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Gagal membuka store SQLite: %v", err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Verifikasi apakah tabel users dan sites berhasil dibuat
	tables := []string{"users", "sessions", "sites", "certificates", "managed_services", "audit_log", "settings"}
	for _, table := range tables {
		var count int
		query := "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?"
		if err := s.DB().QueryRowContext(ctx, query, table).Scan(&count); err != nil {
			t.Fatalf("Gagal memeriksa keberadaan tabel %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("Tabel %s tidak ditemukan setelah migrasi", table)
		}
	}
}
