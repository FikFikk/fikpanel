// Package credentials memastikan panel tidak pernah berjalan dengan akun administrator tanpa
// kata sandi: membuat akun awal dengan kata sandi acak, mengganti kata sandi kosong pada
// instalasi lama, dan menyimpan kredensial yang dibangkitkan ke berkas khusus root (0600).
package credentials

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fikpanel/internal/auth"
	"fikpanel/internal/store"
)

const (
	// FileName adalah nama berkas kredensial di direktori data (default /opt/fikpanel/data).
	FileName = "admin-credentials.txt"
	// DefaultAdminUsername adalah akun administrator awal yang dibuat saat instalasi.
	DefaultAdminUsername = "root"
	// GeneratedPasswordLength adalah panjang kata sandi acak yang dibangkitkan.
	GeneratedPasswordLength = 20
)

// Credential adalah pasangan username dan kata sandi plaintext yang baru dibangkitkan.
type Credential struct {
	Username string
	Password string
}

// Result merangkum tindakan EnsureAdmin.
type Result struct {
	Created        []Credential
	Rotated        []Credential
	RevokedSession int64
}

// Changed melaporkan apakah ada kredensial baru yang ditulis.
func (r Result) Changed() bool { return len(r.Created) > 0 || len(r.Rotated) > 0 }

// PathFor mengembalikan lokasi berkas kredensial yang berdampingan dengan database.
func PathFor(dbPath string) string {
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		abs = dbPath
	}
	return filepath.Join(filepath.Dir(abs), FileName)
}

// EnsureAdmin membuat akun root dengan kata sandi acak bila belum ada pengguna sama sekali, dan
// mengganti kata sandi setiap akun yang masih kosong (warisan mode --dev lama) sekaligus mencabut
// seluruh sesi login akun tersebut. Kredensial baru ditulis ke credPath dengan izin 0600.
func EnsureAdmin(ctx context.Context, st *store.Store, credPath string) (Result, error) {
	var res Result
	users, err := st.ListUsers(ctx)
	if err != nil {
		return res, err
	}

	if len(users) == 0 {
		pw, err := auth.GeneratePassword(GeneratedPasswordLength)
		if err != nil {
			return res, err
		}
		hash, err := auth.HashPassword(pw)
		if err != nil {
			return res, err
		}
		if _, err := st.CreateAdminUser(ctx, DefaultAdminUsername, hash); err != nil {
			return res, err
		}
		res.Created = append(res.Created, Credential{Username: DefaultAdminUsername, Password: pw})
		return res, Write(credPath, res.Created, "akun administrator awal")
	}

	for _, u := range users {
		empty, err := auth.VerifyPassword("", u.PasswordHash)
		if err != nil || !empty {
			continue
		}
		pw, err := auth.GeneratePassword(GeneratedPasswordLength)
		if err != nil {
			return res, err
		}
		hash, err := auth.HashPassword(pw)
		if err != nil {
			return res, err
		}
		revoked, err := st.SetPasswordAndRevokeSessions(ctx, u.ID, hash, "")
		if err != nil {
			return res, err
		}
		res.RevokedSession += revoked
		res.Rotated = append(res.Rotated, Credential{Username: u.Username, Password: pw})
	}
	if len(res.Rotated) > 0 {
		return res, Write(credPath, res.Rotated, "kata sandi kosong diganti otomatis demi keamanan")
	}
	return res, nil
}

// Write menyimpan kredensial ke path secara atomik dengan izin 0600 (hanya root).
func Write(path string, creds []Credential, reason string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# FikPanel - kredensial administrator\n")
	fmt.Fprintf(&b, "# Dibuat otomatis %s (%s).\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"), reason)
	b.WriteString("# Hanya root yang dapat membaca berkas ini. Ganti kata sandi setelah login di menu Pengaturan;\n")
	b.WriteString("# berkas ini dihapus otomatis setelah kata sandi diganti. Lupa kata sandi: sudo fikpanel reset-password\n")
	for _, c := range creds {
		fmt.Fprintf(&b, "\nusername: %s\npassword: %s\n", c.Username, c.Password)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+FileName+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, writeErr := tmp.WriteString(b.String())
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// Forget menghapus berkas kredensial bila isinya hanya untuk username tersebut, karena kata
// sandinya sudah tidak berlaku lagi. Berkas berisi beberapa akun dibiarkan.
func Forget(path, username string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	var names []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if name, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "username:"); ok {
			names = append(names, strings.TrimSpace(name))
		}
	}
	f.Close()
	if len(names) == 1 && names[0] == username {
		_ = os.Remove(path)
	}
}
