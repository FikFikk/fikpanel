package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	password := "RahasiaSuperKuat123!"

	// Uji hashing password
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Gagal melakukan hashing password: %v", err)
	}

	if hash == "" {
		t.Fatal("Hash password tidak boleh kosong")
	}

	// Uji verifikasi password benar
	match, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("Galat saat verifikasi password valid: %v", err)
	}
	if !match {
		t.Errorf("Password yang cocok dilaporkan tidak valid")
	}

	// Uji verifikasi password salah
	matchWrong, err := VerifyPassword("PasswordSalah123!", hash)
	if err != nil {
		t.Fatalf("Galat saat verifikasi password salah: %v", err)
	}
	if matchWrong {
		t.Errorf("Password salah dilaporkan cocok")
	}
}
