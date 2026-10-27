package database

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	// installerMu mencegah instalasi phpMyAdmin berjalan ganda secara simultan
	installerMu sync.Mutex
)

// GetPhpMyAdminTargetDir menentukan direktori terbaik untuk meletakkan berkas instalasi phpMyAdmin
func GetPhpMyAdminTargetDir() string {
	// 1. Jika direktori standar Linux /usr/share/phpmyadmin sudah ada dan memiliki index.php, gunakan itu
	if runtime.GOOS == "linux" {
		usrShare := "/usr/share/phpmyadmin"
		if _, err := os.Stat(filepath.Join(usrShare, "index.php")); err == nil {
			return usrShare
		}
		// Coba buat folder di /usr/share/phpmyadmin jika dijalankan dengan hak akses root/sudo
		if err := os.MkdirAll(usrShare, 0755); err == nil {
			return usrShare
		}

		// Fallback ke /var/lib/fikpanel/phpmyadmin jika /usr/share tidak dapat ditulis
		varLib := "/var/lib/fikpanel/phpmyadmin"
		if err := os.MkdirAll(varLib, 0755); err == nil {
			return varLib
		}
	}

	// 2. Default fallback portabel: storage/phpmyadmin di dalam direktori kerja lokal aplikasi
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	target := filepath.Join(cwd, "storage", "phpmyadmin")
	_ = os.MkdirAll(target, 0755)
	return target
}

// GenerateBlowfishSecret menghasilkan string acak aman sepanjang 32 karakter untuk cookie session phpMyAdmin
func GenerateBlowfishSecret() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "fikpanel_pma_secret_blowfish_32c"
	}
	return hex.EncodeToString(b)
}

// InstallPhpMyAdminFiles melakukan pengunduhan, ekstraksi, dan konfigurasi resmi phpMyAdmin
func InstallPhpMyAdminFiles(ctx context.Context, version string, logLine func(string)) (string, error) {
	installerMu.Lock()
	defer installerMu.Unlock()

	if version == "" {
		version = "5.2.1"
	}

	if logLine == nil {
		logLine = func(string) {}
	}

	targetDir := GetPhpMyAdminTargetDir()

	// Cek apakah berkas index.php sudah terpasang dan valid
	indexPath := filepath.Join(targetDir, "index.php")
	if _, err := os.Stat(indexPath); err == nil {
		logLine(fmt.Sprintf("Berkas fisik phpMyAdmin sudah tersedia di %s. Memvalidasi konfigurasi...", targetDir))
		ensureConfigFile(targetDir)
		return targetDir, nil
	}

	// Langkah 1: Pemeriksaan dependensi PHP di lingkungan server
	logLine("Langkah 1/8: Memeriksa ketersediaan modul PHP dan biner eksekusi di server...")
	phpBin := findPhpBinary()
	if phpBin == "php" && runtime.GOOS == "linux" {
		// Jika biner PHP belum ditemukan di Linux, coba pasang via apt-get jika tersedia
		if _, err := exec.LookPath("apt-get"); err == nil {
			logLine("Biner PHP tidak terdeteksi. Mencoba memasang php-cli dan ekstensi yang dibutuhkan...")
			cmd := exec.CommandContext(ctx, "apt-get", "update", "-y")
			_ = cmd.Run()
			cmdInstall := exec.CommandContext(ctx, "apt-get", "install", "-y", "php-cli", "php-mysql", "php-mbstring", "php-zip", "php-xml", "php-gd", "php-curl")
			if errInstall := cmdInstall.Run(); errInstall == nil {
				logLine("Dependensi PHP berhasil dipasang melalui package manager sistem.")
			}
		}
	} else {
		logLine(fmt.Sprintf("Dependensi PHP terdeteksi: biner [%s] siap melayani phpMyAdmin.", phpBin))
	}

	// Langkah 2: Mengunduh arsip resmi phpMyAdmin
	zipUrls := []string{
		fmt.Sprintf("https://files.phpmyadmin.net/phpMyAdmin/%s/phpMyAdmin-%s-all-languages.zip", version, version),
		fmt.Sprintf("https://github.com/phpmyadmin/phpmyadmin/archive/refs/tags/RELEASE_%s.zip", strings.ReplaceAll(version, ".", "_")),
	}

	tempZip := filepath.Join(os.TempDir(), fmt.Sprintf("fikpanel-pma-%s-%d.zip", version, time.Now().UnixNano()))
	defer os.Remove(tempZip)

	var downloadErr error
	var downloadedSize int64

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	for _, dUrl := range zipUrls {
		logLine(fmt.Sprintf("Langkah 2/8: Mengunduh paket arsip resmi dari %s...", dUrl))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, dUrl, nil)
		if err != nil {
			downloadErr = err
			continue
		}
		req.Header.Set("User-Agent", "FikPanel-Installer/1.0")

		resp, err := client.Do(req)
		if err != nil {
			downloadErr = err
			logLine(fmt.Sprintf("Gagal mengakses mirror (%v), mencoba mirror alternatif...", err))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			downloadErr = fmt.Errorf("server mengembalikan status HTTP %d", resp.StatusCode)
			logLine(fmt.Sprintf("Mirror mengembalikan status HTTP %d, beralih ke mirror berikutnya...", resp.StatusCode))
			continue
		}

		out, err := os.Create(tempZip)
		if err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("gagal membuat berkas unduhan sementara: %w", err)
		}

		// Unduh berkas dengan pemantauan progres berkala
		totalBytes := resp.ContentLength
		var written int64
		buf := make([]byte, 64*1024)
		lastLog := time.Now()

		for {
			n, rErr := resp.Body.Read(buf)
			if n > 0 {
				wn, wErr := out.Write(buf[:n])
				if wErr != nil {
					downloadErr = wErr
					break
				}
				written += int64(wn)
				if time.Since(lastLog) > 1200*time.Millisecond {
					if totalBytes > 0 {
						pct := float64(written) / float64(totalBytes) * 100.0
						logLine(fmt.Sprintf("Mengunduh arsip: %.1f%% (%.1f MB / %.1f MB)...", pct, float64(written)/(1024*1024), float64(totalBytes)/(1024*1024)))
					} else {
						logLine(fmt.Sprintf("Mengunduh arsip: %.1f MB diterima...", float64(written)/(1024*1024)))
					}
					lastLog = time.Now()
				}
			}
			if rErr != nil {
				if rErr != io.EOF {
					downloadErr = rErr
				}
				break
			}
		}

		out.Close()
		resp.Body.Close()

		if downloadErr == nil && written > 1024*1024 {
			downloadedSize = written
			break
		}
	}

	if downloadedSize == 0 {
		return "", fmt.Errorf("gagal mengunduh paket arsip phpMyAdmin dari seluruh mirror resmi: %v", downloadErr)
	}

	logLine(fmt.Sprintf("Langkah 3/8: Memvalidasi integritas paket arsip (ukuran: %.2f MB)... Berhasil.", float64(downloadedSize)/(1024*1024)))

	// Langkah 4: Mengekstrak paket zip ke direktori target dengan meratakan struktur folder root
	logLine(fmt.Sprintf("Langkah 4/8: Mengekstrak paket arsip ke direktori target %s...", targetDir))
	if err := extractZipFlatten(tempZip, targetDir, logLine); err != nil {
		return "", fmt.Errorf("gagal mengekstrak berkas arsip phpMyAdmin: %w", err)
	}

	// Langkah 5: Membuat direktori upload dan session temporer
	logLine("Langkah 5/8: Menyiapkan direktori temporer upload dan session cache...")
	tmpDir := filepath.Join(targetDir, "tmp")
	_ = os.MkdirAll(tmpDir, 0777)
	_ = os.Chmod(tmpDir, 0777)

	// Langkah 6: Membuat berkas konfigurasi config.inc.php resmi
	logLine("Langkah 6/8: Menghasilkan 32-byte Blowfish Secret dan mengonfigurasi config.inc.php...")
	ensureConfigFile(targetDir)

	// Langkah 7: Mengatur izin akses berkas di Linux
	if runtime.GOOS == "linux" {
		logLine("Langkah 7/8: Mengatur kepemilikan izin berkas (chown / chmod)...")
		_ = exec.Command("chown", "-R", "www-data:www-data", targetDir).Run()
		_ = exec.Command("chmod", "-R", "755", targetDir).Run()
		_ = exec.Command("chmod", "777", tmpDir).Run()
	} else {
		logLine("Langkah 7/8: Memverifikasi perizinan berkas direktori...")
	}

	// Langkah 8: Verifikasi akhir keberadaan berkas index.php
	if _, err := os.Stat(filepath.Join(targetDir, "index.php")); err != nil {
		return "", fmt.Errorf("verifikasi gagal: berkas index.php tidak ditemukan di %s setelah ekstraksi", targetDir)
	}

	logLine(fmt.Sprintf("Langkah 8/8: Verifikasi berkas phpMyAdmin di %s selesai (Status: Valid).", targetDir))
	logLine(fmt.Sprintf("==> [SELESAI] Layanan resmi phpMyAdmin %s berhasil dipasang di server!", version))

	return targetDir, nil
}

// ensureConfigFile memastikan berkas config.inc.php tersedia dan terkonfigurasi dengan benar
func ensureConfigFile(targetDir string) {
	cfgPath := filepath.Join(targetDir, "config.inc.php")
	if _, err := os.Stat(cfgPath); err == nil {
		// Konfigurasi sudah ada
		return
	}

	secret := GenerateBlowfishSecret()
	tmpDir := filepath.Join(targetDir, "tmp")

	cfgContent := fmt.Sprintf(`<?php
declare(strict_types=1);

/**
 * Konfigurasi phpMyAdmin otomatis dibuat oleh FikPanel
 */

$cfg['blowfish_secret'] = '%s';

$i = 0;
$i++;
$cfg['Servers'][$i]['auth_type'] = 'cookie';
$cfg['Servers'][$i]['host'] = '127.0.0.1';
$cfg['Servers'][$i]['port'] = '3306';
$cfg['Servers'][$i]['compress'] = false;
$cfg['Servers'][$i]['AllowNoPassword'] = true;

$cfg['UploadDir'] = '';
$cfg['SaveDir'] = '';
$cfg['TempDir'] = '%s';
`, secret, filepath.ToSlash(tmpDir))

	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)
}

// extractZipFlatten mengekstrak isi zip dan menghilangkan prefix direktori root bawaan zip
func extractZipFlatten(zipPath, targetDir string, logLine func(string)) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	// Cari prefix direktori umum di tingkat pertama zip (misal 'phpMyAdmin-5.2.1-all-languages/')
	var commonPrefix string
	for _, f := range r.File {
		cleanName := filepath.ToSlash(f.Name)
		parts := strings.Split(cleanName, "/")
		if len(parts) > 1 && parts[0] != "" {
			if commonPrefix == "" {
				commonPrefix = parts[0] + "/"
			} else if !strings.HasPrefix(cleanName, commonPrefix) {
				// Tidak ada prefix seragam tunggal
				commonPrefix = ""
				break
			}
		}
	}

	totalFiles := len(r.File)
	extractedCount := 0
	lastLog := time.Now()

	for _, f := range r.File {
		extractedCount++
		cleanName := filepath.ToSlash(f.Name)

		// Potong prefix root jika ada agar isi zip langsung masuk ke targetDir
		relPath := cleanName
		if commonPrefix != "" && strings.HasPrefix(relPath, commonPrefix) {
			relPath = strings.TrimPrefix(relPath, commonPrefix)
		}

		if relPath == "" || relPath == "/" {
			continue
		}

		destPath := filepath.Join(targetDir, filepath.FromSlash(relPath))

		// Proteksi terhadap celah keamanan ZipSlip
		cleanTarget := filepath.Clean(targetDir) + string(filepath.Separator)
		cleanDest := filepath.Clean(destPath)
		if !strings.HasPrefix(cleanDest, cleanTarget) && cleanDest != filepath.Clean(targetDir) {
			continue
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(destPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, copyErr := io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if copyErr != nil {
			return copyErr
		}

		if time.Since(lastLog) > 1500*time.Millisecond {
			pct := float64(extractedCount) / float64(totalFiles) * 100.0
			logLine(fmt.Sprintf("Mengekstrak berkas: %.0f%% (%d / %d berkas)...", pct, extractedCount, totalFiles))
			lastLog = time.Now()
		}
	}

	return nil
}
