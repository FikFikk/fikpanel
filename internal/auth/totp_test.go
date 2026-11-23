package auth

import (
	"strings"
	"testing"
	"time"
)

func TestTOTPGenerateComputeVerify(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret gagal: %v", err)
	}
	if len(secret) < 16 {
		t.Fatalf("Panjang secret TOTP terlalu pendek: %d", len(secret))
	}

	uri := BuildOtpAuthURI("FikPanel", "root", secret)
	if !strings.HasPrefix(uri, "otpauth://totp/") || !strings.Contains(uri, "secret="+secret) {
		t.Fatalf("URI otpauth tidak valid: %s", uri)
	}

	now := time.Unix(1700000000, 0)
	code, err := ComputeTOTPCode(secret, now)
	if err != nil || len(code) != 6 {
		t.Fatalf("ComputeTOTPCode gagal: code=%q err=%v", code, err)
	}

	if !VerifyTOTPCode(secret, code, now) {
		t.Fatalf("Kode TOTP valid harus lolos verifikasi")
	}
	// Toleransi geser jam ±30 detik
	if !VerifyTOTPCode(secret, code, now.Add(25*time.Second)) {
		t.Fatalf("Kode TOTP dalam jendela toleransi ±1 step harus lolos")
	}
	if VerifyTOTPCode(secret, "000000", now) && code != "000000" {
		t.Fatalf("Kode TOTP salah tidak boleh lolos")
	}
}

func TestRecoveryCodes(t *testing.T) {
	plain, hashes, err := GenerateRecoveryCodes(8)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes gagal: %v", err)
	}
	if len(plain) != 8 || len(hashes) != 8 {
		t.Fatalf("Jumlah kode pemulihan salah: plain=%d hashes=%d", len(plain), len(hashes))
	}
	if HashRecoveryCode(strings.ToLower(strings.ReplaceAll(plain[0], "-", ""))) != hashes[0] {
		t.Fatalf("Normalisasi hash kode pemulihan tidak cocok")
	}
}
