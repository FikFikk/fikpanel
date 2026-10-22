package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fikpanel/internal/store"
)

func TestServiceManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_svc_test_*")
	if err != nil {
		t.Fatalf("Gagal membuat direktori sementara: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_svc.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Gagal membuka store SQLite: %v", err)
	}
	defer s.Close()

	mgr := NewManager(s.DB())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := mgr.SeedDefaultServices(ctx); err != nil {
		t.Fatalf("Gagal inisialisasi default services: %v", err)
	}

	list, err := mgr.ListServices(ctx)
	if err != nil {
		t.Fatalf("Gagal membaca daftar service: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("Daftar service default harus berjumlah 3, ditemukan: %d", len(list))
	}
}
