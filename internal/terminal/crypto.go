package terminal

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"fikpanel/internal/store"
)

const (
	credPrefix     = "enc:v1:"
	credKeySetting = "terminal_cred_master_key"
)

// SecretBox mengelola enkripsi/dekripsi kredensial SSH (AES-256-GCM) sebelum disimpan ke SQLite
type SecretBox struct {
	mu    sync.Mutex
	store *store.Store
	key   []byte
}

// NewSecretBox membuat instance enkriptor kredensial terminal yang terhubung ke store
func NewSecretBox(s *store.Store) *SecretBox {
	return &SecretBox{store: s}
}

// ensureKey memuat atau membuat kunci master 32-byte AES-256 secara otomatis pada tabel settings
func (b *SecretBox) ensureKey(ctx context.Context) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.key) == 32 {
		return b.key, nil
	}

	rawHex, err := b.store.GetSetting(ctx, credKeySetting)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca kunci enkripsi terminal: %w", err)
	}
	if strings.TrimSpace(rawHex) != "" {
		decoded, decErr := hex.DecodeString(strings.TrimSpace(rawHex))
		if decErr == nil && len(decoded) == 32 {
			b.key = decoded
			return b.key, nil
		}
	}

	// Buat kunci master acak 256-bit baru jika belum tersedia
	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return nil, fmt.Errorf("gagal membangkitkan kunci enkripsi acak: %w", err)
	}
	if err := b.store.SetSetting(ctx, credKeySetting, hex.EncodeToString(newKey)); err != nil {
		return nil, fmt.Errorf("gagal menyimpan kunci enkripsi terminal: %w", err)
	}
	b.key = newKey
	return b.key, nil
}

// IsEncrypted memeriksa apakah string sudah berada dalam format terenkripsi enc:v1:
func IsEncrypted(val string) bool {
	return strings.HasPrefix(val, credPrefix)
}

// Encrypt mengenkripsi string plaintext menggunakan AES-256-GCM (idempoten untuk nilai terenkripsi)
func (b *SecretBox) Encrypt(ctx context.Context, plain string) (string, error) {
	if plain == "" || IsEncrypted(plain) {
		return plain, nil
	}
	key, err := b.ensureKey(ctx)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return credPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt mendekripsi nilai terenkripsi; nilai plaintext lawas dilewatkan apa adanya untuk kompatibilitas mundur
func (b *SecretBox) Decrypt(ctx context.Context, stored string) (string, error) {
	if stored == "" || !IsEncrypted(stored) {
		return stored, nil
	}
	key, err := b.ensureKey(ctx)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, credPrefix))
	if err != nil {
		return "", fmt.Errorf("format ciphertext kredensial rusak: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("payload ciphertext terlalu pendek")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("dekripsi kredensial gagal (kunci atau data tidak cocok): %w", err)
	}
	return string(plain), nil
}

// EncryptServerCredentials mengenkripsi kolom sensitif (password, private key, passphrase) pada objek server
func (b *SecretBox) EncryptServerCredentials(ctx context.Context, srv *store.TerminalServer) error {
	if srv == nil {
		return nil
	}
	var err error
	if srv.Password, err = b.Encrypt(ctx, srv.Password); err != nil {
		return err
	}
	if srv.PrivateKey, err = b.Encrypt(ctx, srv.PrivateKey); err != nil {
		return err
	}
	if srv.Passphrase, err = b.Encrypt(ctx, srv.Passphrase); err != nil {
		return err
	}
	return nil
}

// DecryptServerCredentials mendekripsi kolom sensitif pada salinan objek server untuk keperluan koneksi SSH
func (b *SecretBox) DecryptServerCredentials(ctx context.Context, srv *store.TerminalServer) error {
	if srv == nil {
		return nil
	}
	var err error
	if srv.Password, err = b.Decrypt(ctx, srv.Password); err != nil {
		return err
	}
	if srv.PrivateKey, err = b.Decrypt(ctx, srv.PrivateKey); err != nil {
		return err
	}
	if srv.Passphrase, err = b.Decrypt(ctx, srv.Passphrase); err != nil {
		return err
	}
	return nil
}

// MigratePlaintextServers mengenkripsi secara transparan seluruh kredensial server lawas yang masih plaintext
func (b *SecretBox) MigratePlaintextServers(ctx context.Context) error {
	servers, err := b.store.ListTerminalServers(ctx)
	if err != nil {
		return err
	}
	for i := range servers {
		srv := servers[i]
		needsUpdate := (srv.Password != "" && !IsEncrypted(srv.Password)) ||
			(srv.PrivateKey != "" && !IsEncrypted(srv.PrivateKey)) ||
			(srv.Passphrase != "" && !IsEncrypted(srv.Passphrase))
		if !needsUpdate {
			continue
		}
		if err := b.EncryptServerCredentials(ctx, &srv); err != nil {
			return err
		}
		if err := b.store.UpdateTerminalServer(ctx, &srv); err != nil {
			return err
		}
	}
	return nil
}
