package terminal

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/store"
	"golang.org/x/crypto/ssh"
)

// SSHConfig menyimpan parameter koneksi SSH untuk pooling dan dialing
type SSHConfig struct {
	ServerID   int64
	Host       string
	Port       int
	Username   string
	AuthType   string
	Password   string
	PrivateKey string
	Passphrase string
}

// HostKeyMismatchError muncul saat host key server berubah dari yang sebelumnya dipercaya (indikasi MITM / rebuild VPS)
type HostKeyMismatchError struct {
	HostPort            string
	KeyType             string
	ExpectedFingerprint string
	ActualFingerprint   string
}

func (e *HostKeyMismatchError) Error() string {
	return fmt.Sprintf(
		"HOST KEY MISMATCH untuk %s! Sidik jari lama (%s) berbeda dengan server saat ini (%s). Jika VPS baru di-rebuild, klik Reset Host Key di daftar server lalu hubungkan ulang.",
		e.HostPort, e.ExpectedFingerprint, e.ActualFingerprint,
	)
}

// IsHostKeyMismatch memeriksa apakah error berasal dari ketidakcocokan host key SSH
func IsHostKeyMismatch(err error) (*HostKeyMismatchError, bool) {
	var target *HostKeyMismatchError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

// BuildTOFUHostKeyCallback membangun validator host key SSH dengan kebijakan Trust On First Use (TOFU)
func BuildTOFUHostKeyCallback(s *store.Store, hostPort string, outFingerprint *string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fp := ssh.FingerprintSHA256(key)
		pubB64 := base64.StdEncoding.EncodeToString(key.Marshal())
		keyType := key.Type()
		if outFingerprint != nil {
			*outFingerprint = fp
		}

		// Jika store tidak tersedia (misal unit test terisolasi), izinkan saja
		if s == nil {
			return nil
		}

		existing, err := s.GetKnownHost(ctx, hostPort)
		if err != nil {
			return fmt.Errorf("gagal memeriksa known_hosts: %w", err)
		}
		if existing == nil {
			// Koneksi pertama kali: catat sidik jari host key (TOFU)
			if upsertErr := s.UpsertKnownHost(ctx, &store.TerminalKnownHost{
				HostPort:    hostPort,
				KeyType:     keyType,
				Fingerprint: fp,
				PublicKey:   pubB64,
			}); upsertErr != nil {
				return fmt.Errorf("gagal menyimpan host key baru: %w", upsertErr)
			}
			return nil
		}

		if existing.PublicKey != pubB64 {
			return &HostKeyMismatchError{
				HostPort:            hostPort,
				KeyType:             keyType,
				ExpectedFingerprint: existing.Fingerprint,
				ActualFingerprint:   fp,
			}
		}

		// Perbarui waktu kunjungan terakhir
		_ = s.UpsertKnownHost(ctx, existing)
		return nil
	}
}

type pooledSSHConn struct {
	serverID    int64
	hostPort    string
	client      *ssh.Client
	fingerprint string
	lastUsed    time.Time
}

// SSHPool mengelola koneksi SSH remote yang persisten dengan idle eviction & keepalive
type SSHPool struct {
	mu      sync.Mutex
	store   *store.Store
	entries map[string]*pooledSSHConn
	idleTTL time.Duration
}

// NewSSHPool membuat instance pool koneksi SSH dengan batas idle default 5 menit
func NewSSHPool(s *store.Store, idleTTL time.Duration) *SSHPool {
	if idleTTL <= 0 {
		idleTTL = 5 * time.Minute
	}
	return &SSHPool{
		store:   s,
		entries: make(map[string]*pooledSSHConn),
		idleTTL: idleTTL,
	}
}

func normalizeSSHConfig(cfg SSHConfig) SSHConfig {
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Port <= 0 {
		cfg.Port = 22
	}
	if strings.TrimSpace(cfg.Username) == "" {
		cfg.Username = "root"
	}
	if strings.TrimSpace(cfg.AuthType) == "" {
		cfg.AuthType = "password"
	}
	return cfg
}

func sshPoolKey(cfg SSHConfig) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s|%d|%s|%s|%s|%s|%s",
		cfg.ServerID, cfg.Host, cfg.Port, cfg.Username, cfg.AuthType,
		cfg.Password, cfg.PrivateKey, cfg.Passphrase,
	)
	return hex.EncodeToString(h.Sum(nil))
}

// dialSSHClient membuka koneksi TCP+SSH baru dengan validasi host key TOFU
func dialSSHClient(s *store.Store, cfg SSHConfig, timeout time.Duration) (*ssh.Client, string, error) {
	cfg = normalizeSSHConfig(cfg)
	if cfg.Host == "" {
		return nil, "", errors.New("alamat host SSH tidak boleh kosong")
	}

	var authMethods []ssh.AuthMethod
	if cfg.AuthType == "key" {
		var signer ssh.Signer
		var err error
		keyBytes := []byte(strings.TrimSpace(cfg.PrivateKey))
		if strings.TrimSpace(cfg.Passphrase) != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(cfg.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}
		if err != nil {
			return nil, "", fmt.Errorf("kunci privat SSH tidak valid: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	hostPort := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var fingerprint string
	clientCfg := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: BuildTOFUHostKeyCallback(s, hostPort, &fingerprint),
		Timeout:         timeout,
	}

	client, err := ssh.Dial("tcp", hostPort, clientCfg)
	if err != nil {
		return nil, fingerprint, fmt.Errorf("koneksi SSH ke %s gagal: %w", hostPort, err)
	}
	return client, fingerprint, nil
}

// isClientAlive mengecek apakah koneksi SSH masih sehat melalui paket keepalive ringan
func isClientAlive(client *ssh.Client) bool {
	if client == nil {
		return false
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		done <- err
	}()
	select {
	case err := <-done:
		return err == nil
	case <-time.After(2 * time.Second):
		return false
	}
}

// Acquire mengambil koneksi SSH aktif dari pool atau membuka koneksi baru bila belum tersedia/terputus.
// Pemanggil TIDAK boleh menutup *ssh.Client yang dikembalikan karena dikelola oleh pool.
func (p *SSHPool) Acquire(cfg SSHConfig, timeout time.Duration) (*ssh.Client, string, error) {
	cfg = normalizeSSHConfig(cfg)
	key := sshPoolKey(cfg)

	p.mu.Lock()
	existing := p.entries[key]
	if existing != nil {
		existing.lastUsed = time.Now()
	}
	p.mu.Unlock()

	if existing != nil && isClientAlive(existing.client) {
		return existing.client, existing.fingerprint, nil
	}

	// Bersihkan koneksi usang bila pengecekan keepalive gagal
	if existing != nil {
		p.mu.Lock()
		if cur, ok := p.entries[key]; ok && cur == existing {
			delete(p.entries, key)
		}
		p.mu.Unlock()
		_ = existing.client.Close()
	}

	client, fp, err := dialSSHClient(p.store, cfg, timeout)
	if err != nil {
		return nil, fp, err
	}

	hostPort := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	p.mu.Lock()
	// Jika goroutine lain sempat menyimpan koneksi baru, tutup yang lama agar tidak bocor
	if old, ok := p.entries[key]; ok && old != nil {
		_ = old.client.Close()
	}
	p.entries[key] = &pooledSSHConn{
		serverID:    cfg.ServerID,
		hostPort:    hostPort,
		client:      client,
		fingerprint: fp,
		lastUsed:    time.Now(),
	}
	p.mu.Unlock()

	return client, fp, nil
}

// DialFresh membuka koneksi SSH satu kali pakai (untuk pengujian koneksi sebelum disimpan)
func (p *SSHPool) DialFresh(cfg SSHConfig, timeout time.Duration) (*ssh.Client, string, error) {
	return dialSSHClient(p.store, cfg, timeout)
}

// InvalidateServer menutup dan menghapus seluruh koneksi pool milik serverID tertentu
func (p *SSHPool) InvalidateServer(serverID int64) {
	if serverID <= 0 {
		return
	}
	p.mu.Lock()
	var toClose []*ssh.Client
	for k, entry := range p.entries {
		if entry.serverID == serverID {
			toClose = append(toClose, entry.client)
			delete(p.entries, k)
		}
	}
	p.mu.Unlock()
	for _, c := range toClose {
		_ = c.Close()
	}
}

// InvalidateHost menutup seluruh koneksi pool untuk pasangan host:port tertentu
func (p *SSHPool) InvalidateHost(hostPort string) {
	p.mu.Lock()
	var toClose []*ssh.Client
	for k, entry := range p.entries {
		if entry.hostPort == hostPort {
			toClose = append(toClose, entry.client)
			delete(p.entries, k)
		}
	}
	p.mu.Unlock()
	for _, c := range toClose {
		_ = c.Close()
	}
}

// StartJanitor menjalankan pembersihan berkala koneksi SSH idle dan menutup semua koneksi saat ctx berakhir
func (p *SSHPool) StartJanitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				p.CloseAll()
				return
			case <-ticker.C:
				p.evictIdle()
			}
		}
	}()
}

func (p *SSHPool) evictIdle() {
	cutoff := time.Now().Add(-p.idleTTL)
	p.mu.Lock()
	var toClose []*ssh.Client
	for k, entry := range p.entries {
		if entry.lastUsed.Before(cutoff) {
			toClose = append(toClose, entry.client)
			delete(p.entries, k)
		}
	}
	p.mu.Unlock()
	for _, c := range toClose {
		_ = c.Close()
	}
}

// CloseAll menutup seluruh koneksi SSH aktif di dalam pool saat server shutdown
func (p *SSHPool) CloseAll() {
	p.mu.Lock()
	var toClose []*ssh.Client
	for k, entry := range p.entries {
		toClose = append(toClose, entry.client)
		delete(p.entries, k)
	}
	p.mu.Unlock()
	for _, c := range toClose {
		_ = c.Close()
	}
}
