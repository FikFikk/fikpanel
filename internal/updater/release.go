package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const (
	// DefaultRepo adalah repositori GitHub sumber rilis resmi FikPanel.
	DefaultRepo = "FikFikk/fikpanel"
	// ChecksumsAsset adalah nama berkas daftar checksum yang diunggah workflow rilis.
	ChecksumsAsset = "SHA256SUMS.txt"
	// BinaryName adalah nama biner di dalam arsip rilis.
	BinaryName = "fikpanel"

	maxChecksumsBytes int64 = 64 << 10
	maxArchiveBytes   int64 = 256 << 20
	maxBinaryBytes    int64 = 512 << 20
)

// releaseBaseURL adalah origin halaman dan aset rilis. Hanya dapat diganti saat kompilasi
// (-ldflags "-X fikpanel/internal/updater.releaseBaseURL=...") untuk mirror atau pengujian;
// tidak ada opsi runtime yang dapat mengalihkan sumber unduhan.
var releaseBaseURL = "https://github.com"

// ErrReleaseNotFound menandakan tag rilis atau asetnya belum tersedia di GitHub
// (umumnya karena workflow rilis masih membangun biner beberapa menit setelah tag di-push).
var ErrReleaseNotFound = errors.New("rilis atau aset belum tersedia di GitHub Releases")

// Client mengakses halaman dan aset GitHub Releases tanpa GitHub API (bebas rate-limit API 60 req/jam).
type Client struct {
	HTTP      *http.Client
	BaseURL   string
	Repo      string
	UserAgent string
}

// NewClient membuat klien rilis dengan konfigurasi default repositori resmi.
func NewClient(userAgent string) *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 10 * time.Minute},
		BaseURL:   releaseBaseURL,
		Repo:      DefaultRepo,
		UserAgent: userAgent,
	}
}

// AssetName mengembalikan nama arsip rilis Linux untuk arsitektur Go tertentu (amd64/arm64).
func AssetName(arch string) string {
	return "fikpanel-linux-" + arch + ".tar.gz"
}

// ResolveLatestTag menentukan tag rilis stabil terbaru (non-draft, non-prerelease) dari
// redirect https://github.com/<repo>/releases/latest -> .../releases/tag/<tag>.
func (c *Client) ResolveLatestTag(ctx context.Context) (string, error) {
	noRedirect := *c.HTTP
	noRedirect.Timeout = 30 * time.Second
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/"+c.Repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := noRedirect.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal menghubungi GitHub: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))

	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("GitHub tidak mengembalikan redirect rilis terbaru (status %s)", resp.Status)
	}
	loc := resp.Header.Get("Location")
	const marker = "/releases/tag/"
	idx := strings.LastIndex(loc, marker)
	if idx < 0 {
		return "", fmt.Errorf("belum ada rilis stabil yang dipublikasikan (redirect ke %q)", loc)
	}
	tag, err := url.PathUnescape(loc[idx+len(marker):])
	if err != nil {
		return "", fmt.Errorf("tag rilis pada redirect tidak valid: %w", err)
	}
	if !tagPattern.MatchString(tag) {
		return "", fmt.Errorf("tag rilis terbaru %q tidak sesuai format vX.Y.Z", tag)
	}
	return tag, nil
}

// DownloadURL membangun URL unduhan aset untuk tag tertentu.
func (c *Client) DownloadURL(tag, asset string) string {
	return fmt.Sprintf("%s/%s/releases/download/%s/%s", c.BaseURL, c.Repo, url.PathEscape(tag), url.PathEscape(asset))
}

// Download mengunduh satu aset rilis ke dst. Berkas ditulis ke dst+".part" lalu di-rename,
// sehingga dst tidak pernah berisi unduhan setengah jadi.
func (c *Client) Download(ctx context.Context, tag, asset, dst string, maxBytes int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.DownloadURL(tag, asset), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("unduhan %s gagal: %w", asset, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s pada %s", ErrReleaseNotFound, asset, tag)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unduhan %s gagal dengan status %s", asset, resp.Status)
	}
	if resp.ContentLength > maxBytes {
		return fmt.Errorf("ukuran %s (%d byte) melebihi batas %d byte", asset, resp.ContentLength, maxBytes)
	}

	tmp := dst + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(f, io.LimitReader(resp.Body, maxBytes+1))
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("gagal menyimpan %s: %w", asset, err)
	}
	if n > maxBytes {
		_ = os.Remove(tmp)
		return fmt.Errorf("ukuran %s melebihi batas %d byte", asset, maxBytes)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		_ = os.Remove(tmp)
		return fmt.Errorf("unduhan %s terpotong (%d dari %d byte)", asset, n, resp.ContentLength)
	}
	return os.Rename(tmp, dst)
}

// ParseChecksums mengurai format keluaran `sha256sum` ("<hex>  <nama>" atau "<hex> *<nama>").
func ParseChecksums(data []byte) map[string]string {
	sums := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		if len(sum) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(sum); err != nil {
			continue
		}
		sums[strings.TrimPrefix(fields[1], "*")] = sum
	}
	return sums
}

// FileSHA256 menghitung checksum SHA-256 (hex) sebuah berkas.
func FileSHA256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ExtractBinary mengekstrak satu berkas reguler bernama name dari arsip tar.gz ke dst (mode 0755).
// Entri lain diabaikan, sehingga path traversal di dalam arsip tidak pernah ditulis ke disk.
func ExtractBinary(archivePath, name, dst string, maxBytes int64) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("arsip rilis bukan gzip yang valid: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("berkas %q tidak ditemukan di dalam arsip rilis", name)
		}
		if err != nil {
			return fmt.Errorf("arsip rilis rusak: %w", err)
		}
		if path.Clean(strings.TrimPrefix(hdr.Name, "./")) != name {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			return fmt.Errorf("entri %q di arsip bukan berkas reguler", hdr.Name)
		}
		if hdr.Size <= 0 || hdr.Size > maxBytes {
			return fmt.Errorf("ukuran biner %q tidak wajar (%d byte)", hdr.Name, hdr.Size)
		}

		_ = os.Remove(dst)
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		n, copyErr := io.Copy(out, io.LimitReader(tr, maxBytes+1))
		syncErr := out.Sync()
		closeErr := out.Close()
		if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
			_ = os.Remove(dst)
			return fmt.Errorf("gagal mengekstrak biner: %w", err)
		}
		if n != hdr.Size {
			_ = os.Remove(dst)
			return fmt.Errorf("biner di arsip terpotong (%d dari %d byte)", n, hdr.Size)
		}
		return os.Chmod(dst, 0o755)
	}
}
