package database

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// PhpMyAdminDaemon mengelola siklus hidup proses PHP built-in server untuk phpMyAdmin resmi
type PhpMyAdminDaemon struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	port      int
	docRoot   string
	folder    string
	proxy     *httputil.ReverseProxy
	isStarted bool
}

// Global daemon instance untuk reverse proxy phpMyAdmin resmi
var globalPmaDaemon = &PhpMyAdminDaemon{
	port: 8885,
}

// FindPhpMyAdminLocation mencari keberadaan berkas direktori instalasi phpMyAdmin fisik
func FindPhpMyAdminLocation() (docRoot string, folder string, fullPath string, found bool) {
	// 1. Cek kandidat lokasi instalasi standar di Linux dan Windows (termasuk Laragon/XAMPP/WAMP di berbagai drive)
	candidates := []string{
		"C:\\laragon\\etc\\apps\\phpMyAdmin",
		"D:\\laragon\\etc\\apps\\phpMyAdmin",
		"E:\\laragon\\etc\\apps\\phpMyAdmin",
		"C:\\xampp\\phpMyAdmin",
		"D:\\xampp\\phpMyAdmin",
		"E:\\xampp\\phpMyAdmin",
		"C:\\wamp64\\apps\\phpmyadmin",
		"C:\\wamp\\apps\\phpmyadmin",
		"/usr/share/phpmyadmin",
		"/www/server/phpmyadmin",
		"/var/www/html/phpmyadmin",
		"/var/lib/fikpanel/phpmyadmin",
	}

	// 2. Cek direktori storage lokal workspace bila dipasang mandiri
	cwd, err := os.Getwd()
	if err == nil {
		candidates = append(candidates, filepath.Join(cwd, "storage", "phpmyadmin"))
	}

	for _, cand := range candidates {
		idx := filepath.Join(cand, "index.php")
		if _, err := os.Stat(idx); err == nil {
			parent := filepath.Dir(cand)
			base := filepath.Base(cand)
			return parent, base, cand, true
		}
	}

	return "", "", "", false
}

// findPhpBinary mencari binary PHP yang tersedia di sistem
func findPhpBinary() string {
	if p, err := exec.LookPath("php"); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		candidates := []string{
			"C:\\laragon\\bin\\php\\php-8.3.28-nts-Win32-vs16-x64\\php.exe",
			"C:\\laragon\\bin\\php\\php-8.2.0-Win32-vs16-x64\\php.exe",
			"C:\\xampp\\php\\php.exe",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
		matches, _ := filepath.Glob("C:\\laragon\\bin\\php\\*\\php.exe")
		if len(matches) > 0 {
			return matches[0]
		}
	}
	return "php"
}

// isPortListening memeriksa apakah port lokal sedang mendengarkan
func isPortListening(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 250*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

// startServer menyalakan proses PHP built-in server di latar belakang
func (d *PhpMyAdminDaemon) startServer() error {
	phpBin := findPhpBinary()

	// Pastikan port bebas atau cari port alternatif
	if isPortListening(d.port) {
		// Port sudah aktif melayani
		d.isStarted = true
		return nil
	}

	// Siapkan argumen: php -S 127.0.0.1:port -t docRoot
	cmd := exec.Command(phpBin, "-S", fmt.Sprintf("127.0.0.1:%d", d.port), "-t", d.docRoot)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("gagal mengeksekusi biner php (%s): %w", phpBin, err)
	}

	d.cmd = cmd
	d.isStarted = true

	// Tunggu port siap mendengarkan hingga 2 detik
	ready := false
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if isPortListening(d.port) {
			ready = true
			break
		}
	}

	if !ready {
		return fmt.Errorf("server PHP phpMyAdmin pada 127.0.0.1:%d tidak merespons dalam 2 detik", d.port)
	}

	return nil
}

// GetPhpMyAdminProxy mengembalikan ReverseProxy aktif yang mengarah ke instance resmi phpMyAdmin
func GetPhpMyAdminProxy() (*httputil.ReverseProxy, error) {
	globalPmaDaemon.mu.Lock()
	defer globalPmaDaemon.mu.Unlock()

	docRoot, folder, _, found := FindPhpMyAdminLocation()
	if !found {
		return nil, fmt.Errorf("direktori fisik paket resmi phpMyAdmin tidak ditemukan di server")
	}

	globalPmaDaemon.docRoot = docRoot
	globalPmaDaemon.folder = folder

	// Pastikan server PHP built-in berjalan
	if !isPortListening(globalPmaDaemon.port) {
		if err := globalPmaDaemon.startServer(); err != nil {
			return nil, err
		}
	}

	if globalPmaDaemon.proxy == nil {
		targetURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", globalPmaDaemon.port))
		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			clientHost := req.Host
			clientProto := "http"
			if req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https" {
				clientProto = "https"
			}

			originalDirector(req)
			req.Host = targetURL.Host

			// Normalisasi path agar sesuai nama subfolder phpMyAdmin fisik
			lowerPath := strings.ToLower(req.URL.Path)
			if strings.HasPrefix(lowerPath, "/pma/") {
				req.URL.Path = "/" + globalPmaDaemon.folder + "/" + req.URL.Path[5:]
			} else if lowerPath == "/pma" {
				req.URL.Path = "/" + globalPmaDaemon.folder + "/"
			} else if strings.HasPrefix(lowerPath, "/phpmyadmin/") {
				req.URL.Path = "/" + globalPmaDaemon.folder + "/" + req.URL.Path[12:]
			} else if lowerPath == "/phpmyadmin" {
				req.URL.Path = "/" + globalPmaDaemon.folder + "/"
			}

			// Teruskan header proxy standar agar URL dan skema klien tetap konsisten
			if clientHost != "" {
				req.Header.Set("X-Forwarded-Host", clientHost)
			}
			req.Header.Set("X-Forwarded-Proto", clientProto)
		}

		proxy.ModifyResponse = func(resp *http.Response) error {
			// Normalisasi Location redirect jika phpMyAdmin mengarahkan ke folder fisik internal
			if loc := resp.Header.Get("Location"); loc != "" {
				if strings.Contains(loc, "/"+globalPmaDaemon.folder+"/") {
					resp.Header.Set("Location", strings.Replace(loc, "/"+globalPmaDaemon.folder+"/", "/phpmyadmin/", 1))
				}
			}

			// Normalisasi path Set-Cookie agar sesi tersimpan untuk rute /phpmyadmin/ dan /pma/
			cookies := resp.Header.Values("Set-Cookie")
			if len(cookies) > 0 {
				resp.Header.Del("Set-Cookie")
				for _, c := range cookies {
					cNorm := strings.Replace(c, "Path=/"+globalPmaDaemon.folder, "Path=/phpmyadmin", 1)
					cNorm = strings.Replace(cNorm, "path=/"+globalPmaDaemon.folder, "path=/phpmyadmin", 1)
					resp.Header.Add("Set-Cookie", cNorm)
				}
			}
			return nil
		}

		globalPmaDaemon.proxy = proxy
	}

	return globalPmaDaemon.proxy, nil
}
