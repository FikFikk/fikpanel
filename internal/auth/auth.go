package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parameter Argon2id yang disesuaikan dengan batasan GOMEMLIMIT=48MiB
const (
	argonMemory      = 19 * 1024 // 19 MiB memori
	argonIterations  = 2         // 2 iterasi waktu
	argonParallelism = 1         // 1 thread paralel
	argonSaltLen     = 16        // 16 byte salt
	argonKeyLen      = 32        // 32 byte hash output
)

var (
	ErrInvalidHash         = errors.New("format hash kata sandi tidak valid")
	ErrIncompatibleVersion = errors.New("versi argon2 tidak didukung")
)

// HashPassword membuat hash password berbasis Argon2id dengan format terstandarisasi
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("gagal menghasilkan salt kriptografis: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Format: $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonIterations, argonParallelism, b64Salt, b64Hash)

	return encoded, nil
}

// VerifyPassword membandingkan teks password mentah dengan hash tersimpan secara constant-time
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, ErrInvalidHash
	}

	if parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	if version != argon2.Version {
		return false, ErrIncompatibleVersion
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}

	comparisonHash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expectedHash)))

	// Gunakan ConstantTimeCompare untuk mencegah serangan timing
	if subtle.ConstantTimeCompare(expectedHash, comparisonHash) == 1 {
		return true, nil
	}

	return false, nil
}
