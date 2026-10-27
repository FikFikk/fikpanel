package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTerminalStoreOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_terminal_test_*")
	if err != nil {
		t.Fatalf("Gagal membuat direktori sementara: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_terminal.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Gagal inisialisasi database store: %v", err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Uji Operasi Pintasan Perintah (Default seed & CRUD)
	cmds, err := s.ListTerminalCommands(ctx)
	if err != nil {
		t.Fatalf("Gagal membaca daftar perintah default: %v", err)
	}
	if len(cmds) == 0 {
		t.Fatalf("Perintah default bawaan migrasi tidak boleh kosong")
	}

	newCmd := &TerminalCommand{
		Name:    "Test Command",
		Content: "echo 'hello terminal'",
	}
	cmdID, err := s.CreateTerminalCommand(ctx, newCmd)
	if err != nil {
		t.Fatalf("Gagal membuat perintah baru: %v", err)
	}

	fetchedCmd, err := s.GetTerminalCommand(ctx, cmdID)
	if err != nil || fetchedCmd == nil {
		t.Fatalf("Gagal mengambil perintah tersimpan: %v", err)
	}
	if fetchedCmd.Name != "Test Command" {
		t.Errorf("Nama perintah tidak cocok: got %s, want %s", fetchedCmd.Name, "Test Command")
	}

	fetchedCmd.Content = "echo 'updated'"
	if err := s.UpdateTerminalCommand(ctx, fetchedCmd); err != nil {
		t.Fatalf("Gagal memperbarui perintah: %v", err)
	}

	if err := s.DeleteTerminalCommand(ctx, cmdID); err != nil {
		t.Fatalf("Gagal menghapus perintah: %v", err)
	}

	// 2. Uji Operasi Remote Server CRUD
	srv := &TerminalServer{
		Name:     "VPS Singapore",
		Host:     "192.168.10.50",
		Port:     22,
		Username: "ubuntu",
		AuthType: "password",
		Password: "secret-password",
		Remarks:  "Production Server",
	}
	srvID, err := s.CreateTerminalServer(ctx, srv)
	if err != nil {
		t.Fatalf("Gagal membuat remote server: %v", err)
	}

	fetchedSrv, err := s.GetTerminalServer(ctx, srvID)
	if err != nil || fetchedSrv == nil {
		t.Fatalf("Gagal mengambil remote server tersimpan: %v", err)
	}
	if fetchedSrv.Host != "192.168.10.50" || fetchedSrv.Username != "ubuntu" {
		t.Errorf("Data server tidak sesuai ekspektasi: %+v", fetchedSrv)
	}

	fetchedSrv.Remarks = "Staging Server"
	if err := s.UpdateTerminalServer(ctx, fetchedSrv); err != nil {
		t.Fatalf("Gagal memperbarui data server: %v", err)
	}

	srvList, err := s.ListTerminalServers(ctx)
	if err != nil || len(srvList) != 1 {
		t.Fatalf("Jumlah server dalam daftar salah: got %d, want 1", len(srvList))
	}

	if err := s.DeleteTerminalServer(ctx, srvID); err != nil {
		t.Fatalf("Gagal menghapus server: %v", err)
	}

	srvListAfter, err := s.ListTerminalServers(ctx)
	if err != nil || len(srvListAfter) != 0 {
		t.Errorf("Server harusnya kosong setelah dihapus: got %d", len(srvListAfter))
	}
}
