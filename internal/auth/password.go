package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// MinPasswordLength adalah panjang minimum kata sandi administrator panel.
const MinPasswordLength = 8

// passwordAlphabet menghindari karakter yang mudah tertukar (0/O, 1/l/I) dan karakter
// yang menyulitkan copy-paste di terminal atau shell.
const passwordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// GeneratePassword membuat kata sandi acak kriptografis sepanjang length karakter.
// 20 karakter dari alfabet 57 simbol setara ~116 bit entropi.
func GeneratePassword(length int) (string, error) {
	if length < MinPasswordLength {
		length = MinPasswordLength
	}
	max := big.NewInt(int64(len(passwordAlphabet)))
	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("gagal membangkitkan kata sandi acak: %w", err)
		}
		out[i] = passwordAlphabet[n.Int64()]
	}
	return string(out), nil
}

// ValidateNewPassword memeriksa aturan minimum kata sandi baru.
func ValidateNewPassword(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("kata sandi minimal %d karakter", MinPasswordLength)
	}
	return nil
}
