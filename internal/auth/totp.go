package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	totpPeriodSeconds = 30
	totpDigits        = 6
)

// GenerateTOTPSecret menghasilkan rahasia acak 160-bit dalam format Base32 tanpa padding sesuai RFC 6238
func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gagal membangkitkan secret TOTP: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

// BuildOtpAuthURI menyusun URI standar otpauth:// untuk dipindai Google Authenticator / Authy
func BuildOtpAuthURI(issuer, account, secret string) string {
	cleanIssuer := strings.TrimSpace(issuer)
	if cleanIssuer == "" {
		cleanIssuer = "FikPanel"
	}
	cleanAccount := strings.TrimSpace(account)
	if cleanAccount == "" {
		cleanAccount = "admin"
	}
	label := url.PathEscape(cleanIssuer + ":" + cleanAccount)
	q := url.Values{}
	q.Set("secret", strings.ToUpper(strings.TrimSpace(secret)))
	q.Set("issuer", cleanIssuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// ComputeTOTPCode menghitung kode OTP 6 digit untuk secret Base32 pada waktu tertentu (RFC 6238)
func ComputeTOTPCode(secret string, at time.Time) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalized)
	if err != nil {
		// Coba decode dengan padding standar bila secret lama menyertakan '='
		key, err = base32.StdEncoding.DecodeString(normalized)
		if err != nil {
			return "", fmt.Errorf("secret TOTP tidak valid: %w", err)
		}
	}

	counter := uint64(at.Unix() / totpPeriodSeconds)
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	codeInt := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])

	mod := codeInt % 1_000_000
	return fmt.Sprintf("%06d", mod), nil
}

// VerifyTOTPCode memverifikasi kode 6 digit dengan toleransi geser jam ±1 jendela (30 detik) secara constant-time
func VerifyTOTPCode(secret, code string, now time.Time) bool {
	cleanCode := strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(cleanCode) != totpDigits {
		return false
	}
	for _, delta := range []time.Duration{-totpPeriodSeconds * time.Second, 0, totpPeriodSeconds * time.Second} {
		expected, err := ComputeTOTPCode(secret, now.Add(delta))
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(cleanCode), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}

const recoveryAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateRecoveryCodes membuat daftar kode pemulihan 2FA sekali pakai (format XXXX-XXXX) beserta hash SHA-256
func GenerateRecoveryCodes(count int) (plain []string, hashes []string, err error) {
	if count <= 0 {
		count = 8
	}
	plain = make([]string, 0, count)
	hashes = make([]string, 0, count)
	raw := make([]byte, 8)
	for i := 0; i < count; i++ {
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, fmt.Errorf("gagal membuat kode pemulihan: %w", err)
		}
		var sb strings.Builder
		for j := 0; j < 8; j++ {
			if j == 4 {
				sb.WriteByte('-')
			}
			sb.WriteByte(recoveryAlphabet[int(raw[j])%len(recoveryAlphabet)])
		}
		code := sb.String()
		plain = append(plain, code)
		hashes = append(hashes, HashRecoveryCode(code))
	}
	return plain, hashes, nil
}

// HashRecoveryCode menghitung hash SHA-256 hex untuk kode pemulihan setelah normalisasi huruf besar/tanpa tanda hubung
func HashRecoveryCode(code string) string {
	norm := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(code), "-", ""), " ", ""))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}
