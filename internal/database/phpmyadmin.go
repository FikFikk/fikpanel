package database

import (
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PmaSSOSession menyimpan parameter sesi SSO sementara untuk injeksi form phpMyAdmin
type PmaSSOSession struct {
	Token     string
	Database  string
	Username  string
	Password  string
	CreatedAt time.Time
}

var (
	ssoMu       sync.RWMutex
	ssoSessions = make(map[string]PmaSSOSession)
)

// StorePmaSSOSession menyimpan sesi SSO sementara (berlaku 5 menit)
func StorePmaSSOSession(token, db, username, password string) {
	ssoMu.Lock()
	defer ssoMu.Unlock()
	now := time.Now()
	for k, v := range ssoSessions {
		if now.Sub(v.CreatedAt) > 5*time.Minute {
			delete(ssoSessions, k)
		}
	}
	ssoSessions[token] = PmaSSOSession{
		Token:     token,
		Database:  db,
		Username:  username,
		Password:  password,
		CreatedAt: now,
	}
}

// ConsumePmaSSOSession mengambil dan langsung memusnahkan sesi SSO dari RAM (burn-after-reading / single-use)
func ConsumePmaSSOSession(token string) (PmaSSOSession, bool) {
	ssoMu.Lock()
	defer ssoMu.Unlock()
	s, found := ssoSessions[token]
	if found {
		delete(ssoSessions, token)
	}
	return s, found
}

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
			// Hapus kompresi agar proxy dapat memeriksa dan menginjeksi field login secara efisien
			req.Header.Del("Accept-Encoding")
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

			// Injeksi auto-fill username, password, dan rute database pada formulir login phpMyAdmin
			if strings.Contains(resp.Header.Get("Content-Type"), "text/html") && resp.Request != nil {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err == nil {
					htmlStr := string(bodyBytes)
					if strings.Contains(htmlStr, "login_form") || strings.Contains(htmlStr, "input_username") {
						q := resp.Request.URL.Query()
						ssoToken := q.Get("sso_token")
						targetDb := q.Get("db")
						targetUser := q.Get("pma_username")
						if targetUser == "" {
							targetUser = q.Get("user")
						}
						targetPass := ""
						targetRoute := q.Get("route")

						if ssoToken != "" {
							if sso, ok := ConsumePmaSSOSession(ssoToken); ok {
								if targetDb == "" {
									targetDb = sso.Database
								}
								if targetUser == "" {
									targetUser = sso.Username
								}
								if targetPass == "" {
									targetPass = sso.Password
								}
							}
						}

						if targetUser != "" || targetDb != "" {
							// 1. Isi field username secara langsung di HTML
							if targetUser != "" {
								escapedUser := html.EscapeString(targetUser)
								htmlStr = strings.Replace(htmlStr, `id="input_username" value=""`, fmt.Sprintf(`id="input_username" value="%s"`, escapedUser), 1)
								htmlStr = strings.Replace(htmlStr, `name="pma_username" id="input_username" value=""`, fmt.Sprintf(`name="pma_username" id="input_username" value="%s"`, escapedUser), 1)
							}

							// 2. Isi field password bila tersedia di sesi SSO
							if targetPass != "" {
								escapedPass := html.EscapeString(targetPass)
								htmlStr = strings.Replace(htmlStr, `id="input_password" value=""`, fmt.Sprintf(`id="input_password" value="%s"`, escapedPass), 1)
								htmlStr = strings.Replace(htmlStr, `name="pma_password" id="input_password" value=""`, fmt.Sprintf(`name="pma_password" id="input_password" value="%s"`, escapedPass), 1)
							}

							// 3. Kunci rute dan target database saat form login dikirim
							if targetRoute == "" && targetDb != "" {
								targetRoute = "/database/structure"
							}
							if targetRoute != "" {
								htmlStr = strings.Replace(htmlStr, `<input type="hidden" name="route" value="/">`, fmt.Sprintf(`<input type="hidden" name="route" value="%s">`, html.EscapeString(targetRoute)), 1)
							}
							if targetDb != "" {
								if !strings.Contains(htmlStr, `name="db"`) {
									htmlStr = strings.Replace(htmlStr, `<input type="hidden" name="server" value="1">`, fmt.Sprintf(`<input type="hidden" name="db" value="%s"><input type="hidden" name="server" value="1">`, html.EscapeString(targetDb)), 1)
								}
								htmlStr = strings.Replace(htmlStr, `action="index.php?route=/"`, fmt.Sprintf(`action="index.php?route=%s&db=%s"`, url.QueryEscape(targetRoute), url.QueryEscape(targetDb)), 1)

								// Badge visual database di header kartu login
								badgeHtml := fmt.Sprintf(`<div class="card-header d-flex justify-content-between align-items-center"><span>Log in <span style="font-size:11px;font-weight:600;color:#235a81;background:#e2e8f0;padding:2px 8px;border-radius:4px;margin-left:8px;">Database: %s</span></span>`, html.EscapeString(targetDb))
								htmlStr = strings.Replace(htmlStr, `<div class="card-header">`, badgeHtml, 1)
							}

							// 4. Injeksi skrip klien sinkronisasi untuk menjaga isi field tetap stabil terhadap event peramban
							syncScript := fmt.Sprintf(`<script>
(function() {
  var tUser = %q;
  var tPass = %q;
  var tDb = %q;
  var tRoute = %q;
  function syncPma() {
    var u = document.getElementById('input_username');
    var p = document.getElementById('input_password');
    var form = document.getElementById('login_form');
    if (u && tUser && (!u.value || u.value === '')) {
      u.value = tUser;
      u.dispatchEvent(new Event('input', { bubbles: true }));
      u.dispatchEvent(new Event('change', { bubbles: true }));
    }
    if (p && tPass && (!p.value || p.value === '')) {
      p.value = tPass;
      p.dispatchEvent(new Event('input', { bubbles: true }));
      p.dispatchEvent(new Event('change', { bubbles: true }));
    }
    if (form) {
      if (tRoute) {
        var r = form.querySelector('input[name="route"]');
        if (r) { r.value = tRoute; }
        else {
          var ni = document.createElement('input');
          ni.type = 'hidden'; ni.name = 'route'; ni.value = tRoute;
          form.appendChild(ni);
        }
      }
      if (tDb) {
        var d = form.querySelector('input[name="db"]');
        if (d) { d.value = tDb; }
        else {
          var ni2 = document.createElement('input');
          ni2.type = 'hidden'; ni2.name = 'db'; ni2.value = tDb;
          form.appendChild(ni2);
        }
      }
    }
    // Bersihkan parameter sso_token dari URL address bar agar tidak tersisa di history peramban
    try {
      if (window.history && window.history.replaceState) {
        var uObj = new URL(window.location.href);
        if (uObj.searchParams.has('sso_token')) {
          uObj.searchParams.delete('sso_token');
          uObj.searchParams.delete('pma_username');
          window.history.replaceState({}, document.title, uObj.pathname + uObj.search);
        }
      }
    } catch (e) {}
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', syncPma);
  } else {
    syncPma();
  }
  setTimeout(syncPma, 50);
  setTimeout(syncPma, 200);
  setTimeout(syncPma, 600);
})();
</script>`, targetUser, targetPass, targetDb, targetRoute)

							if strings.Contains(htmlStr, "</body>") {
								htmlStr = strings.Replace(htmlStr, "</body>", syncScript+"\n</body>", 1)
							} else {
								htmlStr += syncScript
							}

							resp.Body = io.NopCloser(strings.NewReader(htmlStr))
							resp.ContentLength = int64(len(htmlStr))
							resp.Header.Set("Content-Length", strconv.Itoa(len(htmlStr)))
						} else {
							resp.Body = io.NopCloser(strings.NewReader(htmlStr))
						}
					} else {
						resp.Body = io.NopCloser(strings.NewReader(htmlStr))
					}
				}
			}
			return nil
		}

		globalPmaDaemon.proxy = proxy
	}

	return globalPmaDaemon.proxy, nil
}
