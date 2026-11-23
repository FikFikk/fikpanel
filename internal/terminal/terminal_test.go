package terminal

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"fikpanel/internal/store"
	"golang.org/x/crypto/ssh"
)

func TestSecretBoxEncryptDecrypt(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_term_crypto_*")
	if err != nil {
		t.Fatalf("Gagal membuat direktori sementara: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := store.Open(filepath.Join(tempDir, "crypto.db"))
	if err != nil {
		t.Fatalf("Gagal membuka database: %v", err)
	}
	defer s.Close()

	box := NewSecretBox(s)
	ctx := context.Background()

	secret := "SangatRahasia#2026"
	enc, err := box.Encrypt(ctx, secret)
	if err != nil {
		t.Fatalf("Enkripsi gagal: %v", err)
	}
	if enc == secret || !IsEncrypted(enc) {
		t.Fatalf("Nilai terenkripsi harus berprefiks enc:v1:, got %q", enc)
	}

	// Idempotensi enkripsi ulang
	enc2, err := box.Encrypt(ctx, enc)
	if err != nil || enc2 != enc {
		t.Fatalf("Enkripsi ulang harus idempoten: got %q, err %v", enc2, err)
	}

	// Dekripsi normal
	dec, err := box.Decrypt(ctx, enc)
	if err != nil || dec != secret {
		t.Fatalf("Dekripsi gagal: got %q, want %q, err %v", dec, secret, err)
	}

	// Kompatibilitas plaintext lawas
	legacy, err := box.Decrypt(ctx, "plaintext-lama")
	if err != nil || legacy != "plaintext-lama" {
		t.Fatalf("Plaintext lawas harus dilewatkan utuh: got %q, err %v", legacy, err)
	}

	// Instansiasi baru dengan store sama harus bisa mendekripsi dengan master key tersimpan
	boxReloaded := NewSecretBox(s)
	decReloaded, err := boxReloaded.Decrypt(ctx, enc)
	if err != nil || decReloaded != secret {
		t.Fatalf("Dekripsi lintas instance gagal: got %q, err %v", decReloaded, err)
	}
}

func TestTOFUHostKeyVerification(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_term_tofu_*")
	if err != nil {
		t.Fatalf("Gagal membuat direktori sementara: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := store.Open(filepath.Join(tempDir, "tofu.db"))
	if err != nil {
		t.Fatalf("Gagal membuka database: %v", err)
	}
	defer s.Close()

	pub1, _, _ := ed25519.GenerateKey(rand.Reader)
	sshPub1, _ := ssh.NewPublicKey(pub1)

	pub2, _, _ := ed25519.GenerateKey(rand.Reader)
	sshPub2, _ := ssh.NewPublicKey(pub2)

	var fp string
	cb := BuildTOFUHostKeyCallback(s, "10.0.0.5:22", &fp)

	// 1. Koneksi pertama: otomatis dipercaya dan dicatat (TOFU)
	if err := cb("10.0.0.5:22", nil, sshPub1); err != nil {
		t.Fatalf("Koneksi TOFU pertama harus diizinkan: %v", err)
	}
	if fp == "" {
		t.Fatalf("Fingerprint SHA-256 harus terisi")
	}

	// 2. Koneksi kedua dengan kunci sama: lolos
	if err := cb("10.0.0.5:22", nil, sshPub1); err != nil {
		t.Fatalf("Koneksi kedua dengan host key sama harus lolos: %v", err)
	}

	// 3. Koneksi ketiga dengan kunci berbeda: WAJIB ditolak dengan HostKeyMismatchError
	err = cb("10.0.0.5:22", nil, sshPub2)
	if err == nil {
		t.Fatalf("Kunci berbeda wajib ditolak untuk mencegah MITM")
	}
	if _, ok := IsHostKeyMismatch(err); !ok {
		t.Fatalf("Error harus bertipe HostKeyMismatchError, got %T: %v", err, err)
	}

	// 4. Setelah reset host key, kunci baru dapat dipercaya kembali
	if err := s.DeleteKnownHost(context.Background(), "10.0.0.5:22"); err != nil {
		t.Fatalf("Gagal menghapus known host: %v", err)
	}
	if err := cb("10.0.0.5:22", nil, sshPub2); err != nil {
		t.Fatalf("Setelah reset, kunci baru harus dipercaya: %v", err)
	}
}
