package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/database"
	"fikpanel/internal/sites"
	"fikpanel/internal/store"
)

// GitHubRepoItem merepresentasikan repositori GitHub untuk dropdown pemilih
type GitHubRepoItem struct {
	FullName      string `json:"full_name"`
	Name          string `json:"name"`
	Private       bool   `json:"private"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	DefaultBranch string `json:"default_branch"`
	Language      string `json:"language"`
	Description   string `json:"description"`
	UpdatedAt     string `json:"updated_at"`
}

// BackupArchiveItem merepresentasikan berkas cadangan lokal yang dapat dipakai untuk membuat situs
type BackupArchiveItem struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	UpdatedAt int64  `json:"updated_at"`
}

// readOrGenerateSSHDeployKey membaca kunci SSH publik server atau membuat kunci ed25519 baru jika diminta
func readOrGenerateSSHDeployKey(generate bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "/root"
	}
	sshDir := filepath.Join(home, ".ssh")
	keyPath := filepath.Join(sshDir, "id_ed25519")
	pubPath := keyPath + ".pub"

	if data, err := os.ReadFile(pubPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		return strings.TrimSpace(string(data)), nil
	}
	// Cek fallback id_rsa.pub jika belum generate
	if !generate {
		if data, err := os.ReadFile(filepath.Join(sshDir, "id_rsa.pub")); err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return strings.TrimSpace(string(data)), nil
		}
		return "", nil
	}

	if runtime.GOOS == "windows" {
		return "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFikPanelDeployKeyMock fikpanel@server", nil
	}

	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", fmt.Errorf("gagal membuat direktori .ssh: %w", err)
	}
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "fikpanel-server"
	}
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-C", "fikpanel-deploy@"+hostname, "-f", keyPath, "-N", "")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("gagal membuat SSH deploy key: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	data, err := os.ReadFile(pubPath)
	if err != nil {
		return "", fmt.Errorf("gagal membaca SSH public key: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// handleGitStatus mengambil status koneksi akun GitHub dan SSH Deploy Key server
func (s *Server) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token, _ := s.store.GetSetting(ctx, "github_token")
	username, _ := s.store.GetSetting(ctx, "github_username")
	avatar, _ := s.store.GetSetting(ctx, "github_avatar")
	mode, _ := s.store.GetSetting(ctx, "github_mode")
	clientID, _ := s.store.GetSetting(ctx, "github_oauth_client_id")
	sshKey, _ := readOrGenerateSSHDeployKey(false)

	connected := username != "" && (mode == "token" && token != "" || mode == "public")
	avatarProxy := avatar
	if username != "" {
		avatarProxy = "/api/sites/git/avatar?u=" + url.QueryEscape(username)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":              true,
		"connected":       connected,
		"username":        username,
		"avatar_url":      avatarProxy,
		"mode":            mode, // "token" (Public + Private) atau "public" (Public repos saja)
		"has_token":       token != "",
		"oauth_client_id": clientID,
		"ssh_public_key":  sshKey,
	})
}

// handleGitAvatar mem-proxy foto profil GitHub melalui origin panel sendiri agar lolos aturan CSP ketat
func (s *Server) handleGitAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	targetURL, _ := s.store.GetSetting(ctx, "github_avatar")
	u := strings.TrimSpace(r.URL.Query().Get("u"))
	if targetURL == "" && u != "" {
		targetURL = "https://github.com/" + url.PathEscape(u) + ".png?size=96"
	}
	if targetURL == "" {
		http.NotFound(w, r)
		return
	}

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	req.Header.Set("User-Agent", "FikPanel-Server")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		http.NotFound(w, r)
		return
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/png"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 2*1024*1024))
}

// handleGitConnect menghubungkan akun GitHub via Personal Access Token (mendukung repo Private & Public) atau Username Publik
func (s *Server) handleGitConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode     string `json:"mode"`     // "token" atau "public"
		Token    string `json:"token"`    // ghp_... / github_pat_...
		Username string `json:"username"` // username/org publik jika mode == "public"
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}

	req.Mode = strings.TrimSpace(strings.ToLower(req.Mode))
	req.Token = strings.TrimSpace(req.Token)
	req.Username = strings.TrimSpace(req.Username)

	client := &http.Client{Timeout: 10 * time.Second}

	if req.Mode == "public" || (req.Token == "" && req.Username != "") {
		if req.Username == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "masukkan username atau organisasi GitHub"})
			return
		}
		apiReq, _ := http.NewRequestWithContext(r.Context(), "GET", "https://api.github.com/users/"+url.PathEscape(req.Username), nil)
		apiReq.Header.Set("Accept", "application/vnd.github+json")
		apiReq.Header.Set("User-Agent", "FikPanel-Server")
		resp, err := client.Do(apiReq)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "gagal menghubungi API GitHub: " + err.Error()})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("akun GitHub '%s' tidak ditemukan (HTTP %d)", req.Username, resp.StatusCode)})
			return
		}
		var profile struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&profile)
		if profile.Login == "" {
			profile.Login = req.Username
		}
		_ = s.store.SetSetting(r.Context(), "github_mode", "public")
		_ = s.store.SetSetting(r.Context(), "github_username", profile.Login)
		_ = s.store.SetSetting(r.Context(), "github_avatar", profile.AvatarURL)
		_ = s.store.SetSetting(r.Context(), "github_token", "")
		s.auditSite(r, "git.connect", profile.Login, true, "Menghubungkan akun GitHub mode publik")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":         true,
			"connected":  true,
			"mode":       "public",
			"username":   profile.Login,
			"avatar_url": profile.AvatarURL,
		})
		return
	}

	if req.Token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token akses GitHub (PAT) tidak boleh kosong"})
		return
	}

	apiReq, _ := http.NewRequestWithContext(r.Context(), "GET", "https://api.github.com/user", nil)
	apiReq.Header.Set("Accept", "application/vnd.github+json")
	apiReq.Header.Set("Authorization", "Bearer "+req.Token)
	apiReq.Header.Set("User-Agent", "FikPanel-Server")

	resp, err := client.Do(apiReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "gagal menghubungi API GitHub: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "token GitHub tidak valid atau kedaluwarsa"})
		return
	}

	var profile struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil || profile.Login == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membaca profil akun GitHub"})
		return
	}

	_ = s.store.SetSetting(r.Context(), "github_mode", "token")
	_ = s.store.SetSetting(r.Context(), "github_token", req.Token)
	_ = s.store.SetSetting(r.Context(), "github_username", profile.Login)
	_ = s.store.SetSetting(r.Context(), "github_avatar", profile.AvatarURL)
	s.auditSite(r, "git.connect", profile.Login, true, "Menghubungkan akun GitHub dengan akses penuh (Public & Private)")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"connected":  true,
		"mode":       "token",
		"username":   profile.Login,
		"avatar_url": profile.AvatarURL,
	})
}

// handleGitDeviceStart memulai alur GitHub OAuth Device Flow resmi (persetujuan browser 1-klik tanpa callback URL)
func (s *Server) handleGitDeviceStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID string `json:"client_id"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	clientID := strings.TrimSpace(req.ClientID)
	if clientID == "" {
		clientID, _ = s.store.GetSetting(r.Context(), "github_oauth_client_id")
	}
	if clientID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Masukkan Client ID GitHub OAuth App Anda, atau gunakan mode Token Akses 1-Klik di sebelahnya",
		})
		return
	}
	_ = s.store.SetSetting(r.Context(), "github_oauth_client_id", clientID)

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", "repo read:user")

	apiReq, _ := http.NewRequestWithContext(r.Context(), "POST", "https://github.com/login/device/code", strings.NewReader(form.Encode()))
	apiReq.Header.Set("Accept", "application/json")
	apiReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	apiReq.Header.Set("User-Agent", "FikPanel-Server")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(apiReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "gagal menghubungi GitHub OAuth: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&data)
	if errMsg, ok := data["error_description"].(string); ok && errMsg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": errMsg})
		return
	}
	if _, ok := data["device_code"].(string); !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pastikan Device Flow sudah diaktifkan pada pengaturan GitHub OAuth App Anda"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":               true,
		"device_code":      data["device_code"],
		"user_code":        data["user_code"],
		"verification_uri": data["verification_uri"],
		"expires_in":       data["expires_in"],
		"interval":         data["interval"],
	})
}

// handleGitDevicePoll memeriksa apakah pengguna sudah menekan tombol Authorize di GitHub
func (s *Server) handleGitDevicePoll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID   string `json:"client_id"`
		DeviceCode string `json:"device_code"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	clientID := strings.TrimSpace(req.ClientID)
	if clientID == "" {
		clientID, _ = s.store.GetSetting(r.Context(), "github_oauth_client_id")
	}
	if clientID == "" || strings.TrimSpace(req.DeviceCode) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parameter client_id dan device_code wajib diisi"})
		return
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("device_code", strings.TrimSpace(req.DeviceCode))
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	apiReq, _ := http.NewRequestWithContext(r.Context(), "POST", "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	apiReq.Header.Set("Accept", "application/json")
	apiReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	apiReq.Header.Set("User-Agent", "FikPanel-Server")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(apiReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	var tokenRes struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tokenRes)
	if tokenRes.AccessToken == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"pending": tokenRes.Error == "authorization_pending" || tokenRes.Error == "slow_down",
			"status":  tokenRes.Error,
			"message": tokenRes.ErrorDesc,
		})
		return
	}

	// Ambil profil user dengan access_token baru
	uReq, _ := http.NewRequestWithContext(r.Context(), "GET", "https://api.github.com/user", nil)
	uReq.Header.Set("Accept", "application/vnd.github+json")
	uReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
	uReq.Header.Set("User-Agent", "FikPanel-Server")
	uResp, err := client.Do(uReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer uResp.Body.Close()
	var profile struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	_ = json.NewDecoder(uResp.Body).Decode(&profile)

	_ = s.store.SetSetting(r.Context(), "github_mode", "token")
	_ = s.store.SetSetting(r.Context(), "github_token", tokenRes.AccessToken)
	_ = s.store.SetSetting(r.Context(), "github_username", profile.Login)
	_ = s.store.SetSetting(r.Context(), "github_avatar", profile.AvatarURL)
	s.auditSite(r, "git.connect.oauth", profile.Login, true, "Otorisasi GitHub OAuth Device Flow berhasil")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"pending":    false,
		"connected":  true,
		"mode":       "token",
		"username":   profile.Login,
		"avatar_url": profile.AvatarURL,
	})
}

// handleGitDisconnect menghapus kredensial akun GitHub dari server
func (s *Server) handleGitDisconnect(w http.ResponseWriter, r *http.Request) {
	_ = s.store.SetSetting(r.Context(), "github_mode", "")
	_ = s.store.SetSetting(r.Context(), "github_token", "")
	_ = s.store.SetSetting(r.Context(), "github_username", "")
	_ = s.store.SetSetting(r.Context(), "github_avatar", "")
	s.auditSite(r, "git.disconnect", "github", true, "Memutuskan koneksi akun GitHub")
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleGitListRepos mengambil daftar repositori GitHub milik akun terhubung
func (s *Server) handleGitListRepos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token, _ := s.store.GetSetting(ctx, "github_token")
	username, _ := s.store.GetSetting(ctx, "github_username")
	mode, _ := s.store.GetSetting(ctx, "github_mode")

	var apiURL string
	if mode == "token" && token != "" {
		apiURL = "https://api.github.com/user/repos?per_page=100&sort=updated&affiliation=owner,collaborator,organization_member"
	} else if username != "" {
		apiURL = fmt.Sprintf("https://api.github.com/users/%s/repos?per_page=100&sort=updated", url.PathEscape(username))
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "akun GitHub belum dihubungkan"})
		return
	}

	apiReq, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	apiReq.Header.Set("Accept", "application/vnd.github+json")
	apiReq.Header.Set("User-Agent", "FikPanel-Server")
	if mode == "token" && token != "" {
		apiReq.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(apiReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "gagal mengambil repositori GitHub: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("GitHub API mengembalikan status %d", resp.StatusCode)})
		return
	}

	var repos []GitHubRepoItem
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal membaca daftar repositori"})
		return
	}
	if repos == nil {
		repos = []GitHubRepoItem{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"repos": repos,
	})
}

// handleGitListBranches mengambil daftar branch dari repositori GitHub tertentu
func (s *Server) handleGitListBranches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	if repo == "" || !strings.Contains(repo, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "format repo harus owner/name"})
		return
	}

	token, _ := s.store.GetSetting(ctx, "github_token")
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/branches?per_page=100", repo)
	apiReq, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	apiReq.Header.Set("Accept", "application/vnd.github+json")
	apiReq.Header.Set("User-Agent", "FikPanel-Server")
	if token != "" {
		apiReq.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(apiReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	var raw []struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&raw)
	branches := make([]string, 0, len(raw))
	for _, b := range raw {
		if b.Name != "" {
			branches = append(branches, b.Name)
		}
	}
	if len(branches) == 0 {
		branches = []string{"main", "master"}
	}

	// Inspeksi struktur berkas root repositori agar deteksi runtime akurat meski GitHub melabeli (CSS/HTML)
	detectedKind := ""
	detectedPostCmd := ""
	needsDB := false
	isLaravel := false
	isFilament := false

	contentsURL := fmt.Sprintf("https://api.github.com/repos/%s/contents", repo)
	cReq, _ := http.NewRequestWithContext(ctx, "GET", contentsURL, nil)
	cReq.Header.Set("Accept", "application/vnd.github+json")
	cReq.Header.Set("User-Agent", "FikPanel-Server")
	if token != "" {
		cReq.Header.Set("Authorization", "Bearer "+token)
	}
	if cResp, err := client.Do(cReq); err == nil {
		defer cResp.Body.Close()
		var files []struct {
			Name string `json:"name"`
		}
		if json.NewDecoder(cResp.Body).Decode(&files) == nil && len(files) > 0 {
			fileSet := make(map[string]bool, len(files))
			for _, f := range files {
				fileSet[strings.ToLower(f.Name)] = true
			}
			switch {
			case fileSet["composer.json"] || fileSet["artisan"]:
				detectedKind = "php"
				needsDB = true
				if fileSet["artisan"] {
					isLaravel = true
					detectedPostCmd = "composer install --no-interaction --prefer-dist --optimize-autoloader && php artisan key:generate --force && php artisan migrate --force"
				} else {
					detectedPostCmd = "composer install --no-interaction --prefer-dist"
				}
				// Periksa isi composer.json untuk mendeteksi penggunaan Filament Panel
				if fileSet["composer.json"] {
					compURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/composer.json", repo)
					compReq, _ := http.NewRequestWithContext(ctx, "GET", compURL, nil)
					compReq.Header.Set("Accept", "application/vnd.github.raw")
					compReq.Header.Set("User-Agent", "FikPanel-Server")
					if token != "" {
						compReq.Header.Set("Authorization", "Bearer "+token)
					}
					if compResp, err := client.Do(compReq); err == nil {
						rawComp, _ := io.ReadAll(io.LimitReader(compResp.Body, 128*1024))
						_ = compResp.Body.Close()
						compStr := strings.ToLower(string(rawComp))
						if strings.Contains(compStr, "filament/filament") {
							isFilament = true
							isLaravel = true
						} else if strings.Contains(compStr, "laravel/framework") {
							isLaravel = true
						}
					}
				}
			case fileSet["go.mod"]:
				detectedKind = "go"
				detectedPostCmd = "go build -o app ."
			case fileSet["package.json"] || fileSet["ecosystem.config.cjs"] || fileSet["ecosystem.config.js"]:
				detectedKind = "node"
				if fileSet["drizzle.config.ts"] || fileSet["drizzle.config.js"] || fileSet["prisma"] {
					needsDB = true
				}
				if fileSet["ecosystem.config.cjs"] {
					detectedPostCmd = "npm install && npm run build && (pm2 restart ecosystem.config.cjs --update-env || pm2 start ecosystem.config.cjs)"
				} else {
					detectedPostCmd = "npm install && npm run build"
				}
			case fileSet["requirements.txt"] || fileSet["pyproject.toml"] || fileSet["manage.py"]:
				detectedKind = "python"
				detectedPostCmd = "pip install -r requirements.txt"
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                true,
		"branches":          branches,
		"detected_kind":     detectedKind,
		"detected_post_cmd": detectedPostCmd,
		"needs_db":          needsDB,
		"is_laravel":        isLaravel,
		"is_filament":       isFilament,
	})
}

// handleGitSSHKey menghasilkan atau mengembalikan SSH Deploy Key publik milik server
func (s *Server) handleGitSSHKey(w http.ResponseWriter, r *http.Request) {
	key, err := readOrGenerateSSHDeployKey(true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":             true,
		"ssh_public_key": key,
	})
}

// handleListSiteBackupArchives memindai berkas arsip cadangan lokal (.tar.gz, .tgz, .zip) di server
func (s *Server) handleListSiteBackupArchives(w http.ResponseWriter, r *http.Request) {
	searchDirs := []string{
		"/var/backups/fikpanel",
		"/home/fikpanel/backups",
		"/www/backup/site",
		"/var/backups",
		"/root",
		"/tmp",
	}

	seen := map[string]bool{}
	var archives []BackupArchiveItem

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)
			if !strings.HasSuffix(lower, ".tar.gz") && !strings.HasSuffix(lower, ".tgz") && !strings.HasSuffix(lower, ".zip") {
				continue
			}
			fullPath := filepath.ToSlash(filepath.Join(dir, name))
			if seen[fullPath] {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			seen[fullPath] = true
			archives = append(archives, BackupArchiveItem{
				Name:      name,
				Path:      fullPath,
				SizeBytes: info.Size(),
				UpdatedAt: info.ModTime().Unix(),
			})
		}
	}
	if archives == nil {
		archives = []BackupArchiveItem{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"archives": archives,
	})
}

// handleUploadSiteBackupArchive menerima unggahan arsip (.zip / .tar.gz) dari browser ke /var/backups/fikpanel
func (s *Server) handleUploadSiteBackupArchive(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gagal membaca berkas unggahan (maksimal 256 MB)"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "berkas arsip tidak ditemukan pada form"})
		return
	}
	defer file.Close()

	baseName := filepath.Base(header.Filename)
	lower := strings.ToLower(baseName)
	if !strings.HasSuffix(lower, ".tar.gz") && !strings.HasSuffix(lower, ".tgz") && !strings.HasSuffix(lower, ".zip") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hanya mendukung format arsip .tar.gz, .tgz, atau .zip"})
		return
	}

	targetDir := "/var/backups/fikpanel"
	if runtime.GOOS == "windows" {
		targetDir = os.TempDir()
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menyiapkan direktori cadangan: " + err.Error()})
		return
	}

	dstPath := filepath.Join(targetDir, baseName)
	out, err := os.Create(dstPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menyimpan berkas arsip: " + err.Error()})
		return
	}
	defer out.Close()

	written, err := io.Copy(out, file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menulis berkas arsip: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"archive": BackupArchiveItem{
			Name:      baseName,
			Path:      filepath.ToSlash(dstPath),
			SizeBytes: written,
			UpdatedAt: time.Now().Unix(),
		},
	})
}

// applyPostSiteProvisioning menjalankan langkah tambahan setelah situs dibuat:
// 1. Pembuatan Database MySQL Otomatis (dijalankan lebih awal agar siap saat build/migrate Git berjalan)
// 2. Deploy Template (Laravel, WordPress, Express, Gin, FastAPI, SPA)
// 3. Git Clone (termasuk repositori privat), injeksi .env otomatis, sinkronisasi port ecosystem.config.cjs, & Post-Clone Build Command
// 4. Ekstraksi Arsip Backup (.tar.gz / .zip)
// 5. Penerbitan SSL Let's Encrypt Otomatis
func (s *Server) applyPostSiteProvisioning(ctx context.Context, created *store.Site, in SiteInput, uid *int64) map[string]string {
	notes := map[string]string{}
	// Gunakan context background agar tahapan pasca-clone (seperti Auto-SSL setelah composer install yang memakan waktu > 60 detik)
	// tidak batal akibat timeout HTTP request dari browser/proxy
	if s.bgCtx != nil {
		ctx = s.bgCtx
	} else {
		ctx = context.Background()
	}

	dbName := strings.TrimSpace(in.DBName)
	dbUser := strings.TrimSpace(in.DBUser)
	dbPass := strings.TrimSpace(in.DBPass)

	// Jika situs dibuat ulang tanpa centang CreateDB namun database domain ini sudah terdaftar di panel, gunakan kembali kredensialnya
	if !in.CreateDB {
		base := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			return '_'
		}, strings.ToLower(created.Domain))
		base = strings.Trim(base, "_")
		if len(base) > 12 {
			base = base[:12]
		}
		if existingDB, err := s.store.GetDatabaseByName(ctx, "mysql", base+"_db"); err == nil && existingDB != nil {
			in.CreateDB = true
			dbName = existingDB.Name
			dbUser = existingDB.Username
			dbPass = existingDB.Password
		}
	}

	// 1. Auto-Create Database MySQL terlebih dahulu agar skema tersedia saat proses build / migrasi Git
	if in.CreateDB && dbName != "" && dbUser != "" && dbPass != "" {
		updateSiteDeployProgress(created.Domain, 18, "database", "Menyiapkan database MySQL ("+dbName+") & kredensial user...")
		if err := database.ProvisionDatabase(ctx, "mysql", dbName, dbUser, dbPass, "localhost"); err != nil {
			notes["db_warning"] = err.Error()
		} else {
			_, _ = s.store.CreateDatabase(ctx, store.DatabaseRecord{
				Engine:    "mysql",
				Name:      dbName,
				Username:  dbUser,
				Password:  dbPass,
				SizeBytes: 4096,
				Location:  "localhost",
				Note:      "Dibuat otomatis bersama situs " + created.Domain,
			})
			notes["database"] = dbName
		}
	}

	// 2. One-Click Deploy Template
	if tpl := strings.TrimSpace(strings.ToLower(in.DeployTemplate)); tpl != "" && created.RootPath != "" && runtime.GOOS != "windows" {
		updateSiteDeployProgress(created.Domain, 45, "template", "Menyiapkan struktur berkas template "+tpl+"...")
		if err := provisionDeployTemplate(created, tpl, dbName, dbUser, dbPass); err != nil {
			notes["template_warning"] = err.Error()
		} else {
			notes["template"] = tpl
		}
	}

	// 3. Git Clone Repository + Auto .env + Sinkronisasi Port Ecosystem PM2 + Build
	if repoURL := strings.TrimSpace(in.GitRepoURL); repoURL != "" && created.RootPath != "" && runtime.GOOS != "windows" {
		updateSiteDeployProgress(created.Domain, 28, "git_clone", "Mengkloning repositori Git dari GitHub...")
		token, _ := s.store.GetSetting(ctx, "github_token")
		if err := s.cloneGitRepoToSite(ctx, created, repoURL, strings.TrimSpace(in.GitBranch), strings.TrimSpace(in.GitPostCmd), token, in.CreateDB, dbName, dbUser, dbPass); err != nil {
			notes["git_warning"] = err.Error()
		} else {
			notes["git"] = "cloned"
		}
	}

	// 4. Restore dari Arsip Backup
	if archivePath := strings.TrimSpace(in.BackupArchive); archivePath != "" && created.RootPath != "" && runtime.GOOS != "windows" {
		updateSiteDeployProgress(created.Domain, 55, "backup", "Mengekstrak arsip backup & memulihkan database...")
		if err := s.extractBackupToSite(ctx, created, archivePath, dbName); err != nil {
			notes["backup_warning"] = err.Error()
		} else {
			notes["backup"] = "restored"
		}
	}

	// 4b. Auto-Create / Update Akun Super Admin Laravel / Filament beserta Role super_admin
	if created.Kind == "php" && created.RootPath != "" && runtime.GOOS != "windows" {
		targetDir := created.RootPath
		if filepath.Base(targetDir) == "public" {
			targetDir = filepath.Dir(targetDir)
		}
		_, errArtisan := os.Stat(filepath.Join(targetDir, "artisan"))
		compBytes, _ := os.ReadFile(filepath.Join(targetDir, "composer.json"))
		hasFilament := strings.Contains(strings.ToLower(string(compBytes)), "filament/filament")

		if errArtisan == nil && (in.CreateAdmin || hasFilament) {
			adminName := strings.TrimSpace(in.AdminName)
			if adminName == "" {
				adminName = "Super Admin"
			}
			adminEmail := strings.TrimSpace(in.AdminEmail)
			if adminEmail == "" {
				adminEmail = "admin@" + created.Domain
			}
			adminPass := in.AdminPass
			if strings.TrimSpace(adminPass) == "" {
				adminPass = "admin12345"
			}
			updateSiteDeployProgress(created.Domain, 90, "admin", "Membuat akun Super Admin ("+adminEmail+") & role super_admin...")
			if err := s.provisionLaravelSuperAdmin(ctx, created, adminName, adminEmail, adminPass); err != nil {
				notes["admin_warning"] = err.Error()
			} else {
				notes["admin_email"] = adminEmail
			}
		}
	}

	// 5. Auto-Issue Let's Encrypt SSL
	if in.AutoSSL {
		updateSiteDeployProgress(created.Domain, 96, "ssl", "Menerbitkan & memasang sertifikat SSL HTTPS...")
		created.SSLMode = "letsencrypt"
		created.ForceHTTPS = 1
		_ = s.store.UpdateSite(ctx, created)
		_ = s.sites.Apply(ctx, created, "")
		domain := created.Domain
		_ = s.sites.IssueSSL(ctx, created.ID, true, func(ok bool, msg string) {
			_ = s.store.RecordAudit(s.bgCtx, "user", "ssl.issue", domain, ok, msg, uid)
		})
		notes["ssl"] = "issuing"
	}

	updateSiteDeployProgress(created.Domain, 100, "done", "Deployment selesai! Situs siap digunakan.")
	return notes
}

var ecosystemPortRegex = regexp.MustCompile(`(?i)\bPORT\s*:\s*['"]?(\d{2,5})['"]?`)

// cloneGitRepoToSite mengkloning repositori Git ke direktori proyek situs, menyiapkan .env, menyelaraskan port PM2, dan menjalankan build
func (s *Server) cloneGitRepoToSite(ctx context.Context, site *store.Site, repoURL, branch, postCmd, githubToken string, createDB bool, dbName, dbUser, dbPass string) error {
	targetDir := site.RootPath
	if filepath.Base(targetDir) == "public" {
		targetDir = filepath.Dir(targetDir)
	}

	// Bersihkan file placeholder sambutan bawaan agar git clone tidak gagal karena folder tidak kosong
	_ = os.Remove(filepath.Join(site.RootPath, "index.html"))
	_ = os.Remove(filepath.Join(site.RootPath, "index.php"))
	_ = os.Remove(filepath.Join(site.RootPath, "package.json"))
	_ = os.Remove(filepath.Join(site.RootPath, "server.js"))
	_ = os.Remove(filepath.Join(site.RootPath, "main.go"))
	_ = os.Remove(filepath.Join(site.RootPath, "main.py"))

	authURL := repoURL
	if githubToken != "" && strings.HasPrefix(repoURL, "https://github.com/") {
		authURL = strings.Replace(repoURL, "https://github.com/", "https://x-access-token:"+githubToken+"@github.com/", 1)
	}

	tmpDir := targetDir + "_git_tmp"
	_ = os.RemoveAll(tmpDir)

	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, authURL, tmpDir)

	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=accept-new")
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpDir)
		cleanOut := strings.ReplaceAll(string(out), githubToken, "***")
		return fmt.Errorf("git clone gagal: %s", strings.TrimSpace(cleanOut))
	}

	// Reset remote URL agar token tidak tersimpan dalam teks polos di .git/config
	if authURL != repoURL {
		_ = exec.CommandContext(cctx, "git", "-C", tmpDir, "remote", "set-url", "origin", repoURL).Run()
	}

	// Pindahkan seluruh isi tmpDir ke targetDir tanpa melalui shell
	_ = os.MkdirAll(targetDir, 0o755)
	cpCmd := exec.CommandContext(cctx, "cp", "-a", tmpDir+"/.", targetDir+"/")
	if out, err := cpCmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("gagal menyalin hasil clone: %s", strings.TrimSpace(string(out)))
	}
	_ = os.RemoveAll(tmpDir)

	// Deteksi otomatis PORT pada ecosystem.config.cjs / ecosystem.config.js dan selaraskan upstream Nginx
	appPort := "3000"
	if u, err := url.Parse(site.Upstream); err == nil && u.Port() != "" {
		appPort = u.Port()
	}
	for _, ecoName := range []string{"ecosystem.config.cjs", "ecosystem.config.js"} {
		if raw, err := os.ReadFile(filepath.Join(targetDir, ecoName)); err == nil {
			if m := ecosystemPortRegex.FindStringSubmatch(string(raw)); len(m) == 2 && m[1] != "" {
				appPort = m[1]
				newUpstream := "http://127.0.0.1:" + appPort
				if site.Upstream != newUpstream && (site.Kind == "node" || site.Kind == "proxy") {
					site.Upstream = newUpstream
					_ = s.store.UpdateSite(ctx, site)
					_ = s.sites.Apply(ctx, site, "")
				}
				break
			}
		}
	}

	// Injeksi & sinkronisasi file .env otomatis (selalu perbarui kredensial DB agar tidak tertinggal .env lama saat re-deploy)
	envPath := filepath.Join(targetDir, ".env")
	existingAppKey := ""
	if rawOldEnv, err := os.ReadFile(envPath); err == nil {
		for _, line := range strings.Split(string(rawOldEnv), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "APP_KEY=") && len(trimmed) > len("APP_KEY=") {
				existingAppKey = strings.TrimPrefix(trimmed, "APP_KEY=")
				break
			}
		}
	}
	if existingAppKey == "" {
		keyBytes := make([]byte, 32)
		if _, err := rand.Read(keyBytes); err == nil {
			existingAppKey = "base64:" + base64.StdEncoding.EncodeToString(keyBytes)
		}
	}
	{
		// Deteksi apakah proyek memakai Filament namun model User belum mengimplementasikan FilamentUser (canAccessPanel)
		// Pada Filament, jika User tidak mengimplementasikan FilamentUser, APP_ENV selain 'local' otomatis ditolak 403 Forbidden
		appEnvValue := "production"
		if rawComp, err := os.ReadFile(filepath.Join(targetDir, "composer.json")); err == nil && strings.Contains(string(rawComp), "filament/filament") {
			if rawUserModel, err := os.ReadFile(filepath.Join(targetDir, "app", "Models", "User.php")); err == nil {
				if !strings.Contains(string(rawUserModel), "FilamentUser") && !strings.Contains(string(rawUserModel), "canAccessPanel") {
					appEnvValue = "local"
				}
			}
		}

		// Buat nama cookie sesi dengan prefiks unik tanpa karakter titik (.) karena PHP otomatis mengubah titik menjadi underscore di $_COOKIE
		// dan mengutamakan cookie pertama pada header Cookie (RFC 6265), sehingga prefiks 'fikpanel_' mencegah tabrakan dengan cookie lama di browser.
		sessionCookieName := "fikpanel_" + sanitizeCookieSlug(site.Domain) + "_session"

		var envLines []string
		envLines = append(envLines, fmt.Sprintf("PORT=%s", appPort))
		envLines = append(envLines, "NODE_ENV=production")
		envLines = append(envLines, fmt.Sprintf("APP_NAME=\"%s\"", site.Domain))
		envLines = append(envLines, fmt.Sprintf("APP_ENV=%s", appEnvValue))
		envLines = append(envLines, "APP_DEBUG=false")
		envLines = append(envLines, fmt.Sprintf("APP_URL=https://%s", site.Domain))
		envLines = append(envLines, fmt.Sprintf("APP_KEY=%s", existingAppKey))
		envLines = append(envLines, "SESSION_DRIVER=file")
		envLines = append(envLines, fmt.Sprintf("SESSION_COOKIE=%s", sessionCookieName))
		envLines = append(envLines, "CACHE_STORE=file")
		if createDB && dbName != "" && dbUser != "" {
			envLines = append(envLines, fmt.Sprintf("DATABASE_URL=\"mysql://%s:%s@127.0.0.1:3306/%s\"", dbUser, dbPass, dbName))
			envLines = append(envLines, "DB_CONNECTION=mysql")
			envLines = append(envLines, "DB_HOST=127.0.0.1")
			envLines = append(envLines, "DB_PORT=3306")
			envLines = append(envLines, fmt.Sprintf("DB_DATABASE=%s", dbName))
			envLines = append(envLines, fmt.Sprintf("DB_USERNAME=%s", dbUser))
			envLines = append(envLines, fmt.Sprintf("DB_PASSWORD=%s", dbPass))
		}
		// Pertahankan variabel tambahan dari .env.example yang belum didefinisikan di atas
		if rawExample, err := os.ReadFile(filepath.Join(targetDir, ".env.example")); err == nil {
			definedKeys := map[string]bool{
				"PORT": true, "NODE_ENV": true, "APP_NAME": true, "APP_ENV": true,
				"APP_DEBUG": true, "APP_URL": true, "APP_KEY": true, "SESSION_DRIVER": true, "SESSION_COOKIE": true,
				"CACHE_STORE": true, "DATABASE_URL": true, "DB_CONNECTION": true,
				"DB_HOST": true, "DB_PORT": true, "DB_DATABASE": true, "DB_USERNAME": true, "DB_PASSWORD": true,
			}
			for _, line := range strings.Split(string(rawExample), "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "#") {
					continue
				}
				if eqIdx := strings.IndexByte(trimmed, '='); eqIdx > 0 {
					k := strings.TrimSpace(trimmed[:eqIdx])
					if !definedKeys[k] {
						envLines = append(envLines, trimmed)
						definedKeys[k] = true
					}
				}
			}
		}
		_ = os.WriteFile(envPath, []byte(strings.Join(envLines, "\n")+"\n"), 0o644)
	}

	updateSiteDeployProgress(site.Domain, 44, "env", "Menyuntikkan variabel .env & menyelaraskan struktur direktori...")

	// Siapkan struktur folder wajib Laravel / Filament (storage & bootstrap/cache)
	hasArtisan := false
	if _, err := os.Stat(filepath.Join(targetDir, "artisan")); err == nil {
		hasArtisan = true
		for _, sub := range []string{
			"storage/framework/sessions",
			"storage/framework/views",
			"storage/framework/cache/data",
			"storage/logs",
			"storage/app/public",
			"bootstrap/cache",
		} {
			_ = os.MkdirAll(filepath.Join(targetDir, sub), 0o775)
		}
	}

	// Penyesuaian otomatis Web Root antara proyek berstruktur /public (Laravel/Filament/Symfony) dan PHP biasa (native index.php di root)
	if site.Kind == "php" || site.Kind == "static" {
		hasPubIndex := false
		if st, err := os.Stat(filepath.Join(targetDir, "public", "index.php")); err == nil && !st.IsDir() {
			hasPubIndex = true
		} else if st, err := os.Stat(filepath.Join(targetDir, "public", "index.html")); err == nil && !st.IsDir() {
			hasPubIndex = true
		}
		hasRootIndex := false
		if st, err := os.Stat(filepath.Join(targetDir, "index.php")); err == nil && !st.IsDir() {
			hasRootIndex = true
		} else if st, err := os.Stat(filepath.Join(targetDir, "index.html")); err == nil && !st.IsDir() {
			hasRootIndex = true
		}

		desiredRoot := site.RootPath
		if hasArtisan || hasPubIndex {
			desiredRoot = filepath.ToSlash(filepath.Join(targetDir, "public"))
			_ = os.MkdirAll(desiredRoot, 0o755)
		} else if hasRootIndex && strings.HasSuffix(filepath.ToSlash(site.RootPath), "/public") {
			// Proyek PHP biasa / WordPress yang meletakkan index.php langsung di root proyek (bukan di /public)
			desiredRoot = filepath.ToSlash(targetDir)
			_ = os.Remove(filepath.Join(targetDir, "public"))
		}

		if desiredRoot != "" && desiredRoot != site.RootPath {
			site.RootPath = desiredRoot
			_ = s.store.UpdateSite(ctx, site)
			_ = s.sites.Apply(ctx, site, "")
		}
	}

	hasComposer := false
	if _, err := os.Stat(filepath.Join(targetDir, "composer.json")); err == nil {
		hasComposer = true
	}

	if postCmd != "" || hasArtisan || hasComposer {
		// Sisipkan otomatis npx drizzle-kit push apabila repo memakai Drizzle ORM dan DB diaktifkan
		hasDrizzle := false
		if _, err := os.Stat(filepath.Join(targetDir, "drizzle.config.ts")); err == nil {
			hasDrizzle = true
		} else if _, err := os.Stat(filepath.Join(targetDir, "drizzle.config.js")); err == nil {
			hasDrizzle = true
		}
		if hasDrizzle && createDB && !strings.Contains(postCmd, "drizzle-kit") {
			if strings.Contains(postCmd, "npm run build") {
				postCmd = strings.Replace(postCmd, "npm run build", "npm run build && npx drizzle-kit push", 1)
			} else {
				postCmd = postCmd + " && npx drizzle-kit push"
			}
		}

		bctx, bcancel := context.WithTimeout(ctx, 240*time.Second)
		defer bcancel()
		buildEnv := append(os.Environ(),
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"CI=true",
			"COMPOSER_ALLOW_SUPERUSER=1",
			"COMPOSER_MEMORY_LIMIT=-1",
		)

		// Otomasi penuh proyek PHP Biasa, Laravel, & Laravel Filament dalam tahapan terukur agar persentase progress akurat
		if hasArtisan {
			var combinedLog []byte
			runStage := func(pct int, stage, label, script string) {
				updateSiteDeployProgress(site.Domain, pct, stage, label)
				c := exec.CommandContext(bctx, "sh", "-c", script)
				c.Dir = targetDir
				c.Env = buildEnv
				out, _ := c.CombinedOutput()
				combinedLog = append(combinedLog, out...)
				combinedLog = append(combinedLog, '\n')
			}

			runStage(56, "composer", "Menginstal dependensi Composer & membangun aset frontend...", strings.Join([]string{
				"(composer install --no-interaction --prefer-dist --optimize-autoloader || composer install --no-interaction --prefer-dist --optimize-autoloader --ignore-platform-reqs || composer install --no-interaction --prefer-dist --ignore-platform-reqs --no-scripts || true)",
				"if [ -f package.json ] && [ ! -f public/build/manifest.json ]; then (npm install && npm run build) || true; fi",
			}, "; "))

			runStage(72, "migrate", "Menjalankan migrasi skema database & seeder (php artisan migrate)...", strings.Join([]string{
				"(php artisan key:generate --force || true)",
				"if ! php artisan migrate --force; then if [ -f bootstrap/providers.php ]; then cp bootstrap/providers.php bootstrap/providers.php.fikbak && printf '<?php return [];\\n' > bootstrap/providers.php && (php artisan migrate --force || true); mv bootstrap/providers.php.fikbak bootstrap/providers.php; fi; fi",
				"(php artisan package:discover --ansi || true)",
				"(php artisan migrate --force --seed || php artisan migrate --force || true)",
			}, "; "))

			runStage(84, "filament", "Menyiapkan aset Filament, Shield permission & symlink storage...", strings.Join([]string{
				"(php artisan storage:link --force || true)",
				"(php artisan filament:upgrade || php artisan filament:assets || true)",
				"(php artisan shield:generate --all --panel=admin --no-interaction 2>/dev/null || php artisan shield:generate --all --no-interaction 2>/dev/null || true)",
				"(php artisan shield:super-admin --user=1 --panel=admin --no-interaction 2>/dev/null || php artisan shield:super-admin --user=1 --no-interaction 2>/dev/null || true)",
				"(php artisan optimize:clear || true)",
			}, "; "))

			_ = os.WriteFile(filepath.Join(targetDir, "fikpanel-deploy.log"), combinedLog, 0o644)
		} else {
			if hasComposer && (postCmd == "" || strings.Contains(postCmd, "composer install")) {
				postCmd = "(composer install --no-interaction --prefer-dist --optimize-autoloader || composer install --no-interaction --prefer-dist --ignore-platform-reqs || composer install --no-interaction --prefer-dist --ignore-platform-reqs --no-scripts || true)"
			}
			updateSiteDeployProgress(site.Domain, 65, "build", "Menjalankan perintah build & instalasi dependensi proyek...")
			buildCmd := exec.CommandContext(bctx, "sh", "-c", postCmd+"; (command -v pm2 >/dev/null 2>&1 && pm2 save || true)")
			buildCmd.Dir = targetDir
			buildCmd.Env = buildEnv
			out, _ := buildCmd.CombinedOutput()
			_ = os.WriteFile(filepath.Join(targetDir, "fikpanel-deploy.log"), out, 0o644)
		}
	}

	// Langkah akhir mutlak untuk seluruh proyek PHP / Static: pastikan kepemilikan www-data:www-data & izin tulis storage/cache
	if runtime.GOOS != "windows" && (site.Kind == "php" || site.Kind == "static" || hasArtisan) {
		pctx, pcancel := context.WithTimeout(ctx, 30*time.Second)
		defer pcancel()
		permScript := fmt.Sprintf(
			"chown -R www-data:www-data %q 2>/dev/null || true; chmod 755 /home/fikpanel /home/fikpanel/web %q %q/public 2>/dev/null || true; chmod 644 %q/.env 2>/dev/null || true; chmod -R 775 %q/storage %q/bootstrap/cache 2>/dev/null || true",
			targetDir, targetDir, targetDir, targetDir, targetDir, targetDir,
		)
		_ = exec.CommandContext(pctx, "sh", "-c", permScript).Run()
	}

	return nil
}

// provisionDeployTemplate menyiapkan struktur berkas starter sesuai template One-Click Deploy
func provisionDeployTemplate(site *store.Site, tpl, dbName, dbUser, dbPass string) error {
	root := site.RootPath
	projectDir := root
	if filepath.Base(root) == "public" {
		projectDir = filepath.Dir(root)
	}
	_ = os.MkdirAll(root, 0o755)

	switch tpl {
	case "laravel":
		sessionCookieName := sanitizeCookieSlug(site.Domain) + "_session"
		envContent := fmt.Sprintf("APP_NAME=\"%s\"\nAPP_ENV=production\nAPP_DEBUG=false\nAPP_URL=https://%s\nSESSION_COOKIE=%s\n\nDB_CONNECTION=mysql\nDB_HOST=127.0.0.1\nDB_PORT=3306\nDB_DATABASE=%s\nDB_USERNAME=%s\nDB_PASSWORD=%s\n",
			site.Domain, site.Domain, sessionCookieName, dbName, dbUser, dbPass)
		_ = os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0o640)
		laravelIndex := fmt.Sprintf(`<?php
// Laravel Starter Point — %s
header('Content-Type: text/html; charset=utf-8');
?>
<!doctype html>
<html lang="id">
<head><meta charset="utf-8"><title>%s — Laravel Ready</title>
<style>body{background:#090d16;color:#f8fafc;font-family:system-ui,sans-serif;display:grid;place-items:center;min-height:100vh;margin:0}.card{background:#111827;border:1px solid #1f2937;border-radius:12px;padding:2rem 2.5rem;max-width:520px}h1{margin:0 0 .5rem;font-size:1.35rem;color:#10b981}p{color:#94a3b8;line-height:1.6;font-size:.92rem}code{background:#090d16;padding:.2rem .45rem;border-radius:4px;color:#e2e8f0}</style>
</head>
<body><div class="card"><h1>Laravel Environment Siap</h1><p>Domain <b>%s</b> dengan PHP <code>%s</code> dan struktur <code>/public</code> siap digunakan. Jalankan <code>composer create-project laravel/laravel .</code> di Terminal atau tarik repositori Git Anda.</p></div></body></html>`, site.Domain, site.Domain, site.Domain, phpVersionLabel(site.PHPVersion))
		_ = os.WriteFile(filepath.Join(root, "index.php"), []byte(laravelIndex), 0o644)
	case "wordpress":
		wpConfig := fmt.Sprintf(`<?php
// Konfigurasi Database WordPress Otomatis — %s
define('DB_NAME', '%s');
define('DB_USER', '%s');
define('DB_PASSWORD', '%s');
define('DB_HOST', 'localhost');
define('DB_CHARSET', 'utf8mb4');
define('DB_COLLATE', '');
$table_prefix = 'wp_';
define('WP_DEBUG', false);
`, site.Domain, dbName, dbUser, dbPass)
		_ = os.WriteFile(filepath.Join(root, "wp-config-sample-fikpanel.php"), []byte(wpConfig), 0o640)
	case "spa":
		spaHTML := fmt.Sprintf(`<!doctype html>
<html lang="id">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title>
<style>body{background:#090d16;color:#f8fafc;font-family:system-ui,sans-serif;display:grid;place-items:center;min-height:100vh;margin:0}.box{background:#111827;border:1px solid #1f2937;border-radius:12px;padding:2rem 2.5rem;max-width:500px}</style>
</head>
<body><div id="app" class="box"><h2>%s</h2><p>SPA Routing (React / Vue / Vite / Svelte) aktif dengan fallback <code>index.html</code>.</p></div></body></html>`, site.Domain, site.Domain)
		_ = os.WriteFile(filepath.Join(root, "index.html"), []byte(spaHTML), 0o644)
	}
	return nil
}

func phpVersionLabel(v string) string {
	if v == "" {
		return "FPM Default"
	}
	return v
}

// extractBackupToSite mengekstrak arsip .tar.gz / .tgz / .zip ke direktori root situs dan memulihkan dump SQL jika tersedia
func (s *Server) extractBackupToSite(ctx context.Context, site *store.Site, archivePath, dbName string) error {
	cleanPath := filepath.Clean(archivePath)
	if _, err := os.Stat(cleanPath); err != nil {
		return fmt.Errorf("berkas arsip tidak ditemukan: %s", cleanPath)
	}
	targetDir := site.RootPath
	if filepath.Base(targetDir) == "public" {
		targetDir = filepath.Dir(targetDir)
	}
	_ = os.MkdirAll(targetDir, 0o755)
	_ = os.Remove(filepath.Join(site.RootPath, "index.html"))
	_ = os.Remove(filepath.Join(site.RootPath, "index.php"))

	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	lower := strings.ToLower(cleanPath)
	var cmd *exec.Cmd
	if strings.HasSuffix(lower, ".zip") {
		cmd = exec.CommandContext(cctx, "unzip", "-o", cleanPath, "-d", targetDir)
	} else {
		cmd = exec.CommandContext(cctx, "tar", "-xzf", cleanPath, "-C", targetDir)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gagal mengekstrak arsip: %s", strings.TrimSpace(string(out)))
	}

	// Jika hasil ekstraksi merupakan proyek Laravel/Symfony dengan subfolder public/index.php, arahkan root_path ke /public
	pubIndex := filepath.Join(targetDir, "public", "index.php")
	if st, err := os.Stat(pubIndex); err == nil && !st.IsDir() {
		newRoot := filepath.ToSlash(filepath.Join(targetDir, "public"))
		if site.RootPath != newRoot {
			site.RootPath = newRoot
			_ = s.store.UpdateSite(ctx, site)
			_ = s.sites.Apply(ctx, site, "")
		}
		_ = exec.CommandContext(cctx, "bash", "-c", fmt.Sprintf("chown -R www-data:www-data %q/storage %q/bootstrap/cache %q/.env 2>/dev/null; chmod -R 775 %q/storage %q/bootstrap/cache 2>/dev/null", targetDir, targetDir, targetDir, targetDir, targetDir)).Run()
	}

	// Jika ada berkas dump SQL di dalam arsip (misal database-backup-*.sql) dan database tersedia, impor secara otomatis
	if dbName != "" {
		if matches, err := filepath.Glob(filepath.Join(targetDir, "database-backup-*.sql")); err == nil && len(matches) > 0 {
			sqlFile, err := os.Open(matches[0])
			if err == nil {
				var importCmd *exec.Cmd
				if _, err := os.Stat("/etc/mysql/debian.cnf"); err == nil {
					importCmd = exec.CommandContext(cctx, "mysql", "--defaults-file=/etc/mysql/debian.cnf", dbName)
				} else {
					importCmd = exec.CommandContext(cctx, "mysql", "-u", "root", dbName)
				}
				importCmd.Stdin = sqlFile
				_ = importCmd.Run()
				_ = sqlFile.Close()
			}
		}
	}
	return nil
}

// handleBatchCreateSites memproses pembuatan banyak situs sekaligus (Batch Create)
func (s *Server) handleBatchCreateSites(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lines   string `json:"lines"`
		AutoSSL bool   `json:"auto_ssl"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}

	rawLines := strings.Split(req.Lines, "\n")
	if len(rawLines) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "masukkan minimal satu baris definisi situs"})
		return
	}

	type BatchResultItem struct {
		Line    string `json:"line"`
		Domain  string `json:"domain"`
		Kind    string `json:"kind"`
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}

	var uid *int64
	if u := GetUserFromContext(r.Context()); u != nil {
		uid = &u.ID
	}

	phpVersions := s.sites.PHPVersions()
	defaultPHP := "8.3"
	if len(phpVersions) > 0 {
		defaultPHP = phpVersions[0]
	}

	var results []BatchResultItem
	createdCount := 0

	for _, raw := range rawLines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Format fleksibel: domain|root_path|kind|php_or_port|create_db
		// Juga mendukung format aaPanel: domain|root_path|ftp|db|php_version
		parts := strings.Split(line, "|")
		domainPart := strings.ReplaceAll(strings.TrimSpace(parts[0]), ",", "\n")
		rootPart := ""
		kindPart := "php"
		phpVer := defaultPHP
		upstream := ""
		createDB := false

		if len(parts) >= 2 && strings.TrimSpace(parts[1]) != "" && strings.TrimSpace(parts[1]) != "1" && strings.TrimSpace(parts[1]) != "0" {
			rootPart = strings.TrimSpace(parts[1])
		}
		if len(parts) >= 3 {
			p2 := strings.TrimSpace(strings.ToLower(parts[2]))
			switch p2 {
			case "php", "static", "node", "go", "python", "proxy":
				kindPart = p2
			}
		}
		if len(parts) >= 4 {
			p3 := strings.TrimSpace(parts[3])
			if p3 == "1" {
				createDB = true
			} else if p3 == "0" || p3 == "static" {
				if len(parts) == 4 {
					kindPart = "static"
				}
			} else if strings.HasPrefix(p3, "http://") || strings.HasPrefix(p3, "https://") {
				upstream = p3
			} else if len(p3) == 2 && p3[0] >= '5' && p3[0] <= '8' {
				// Format aaPanel misal "83" -> "8.3"
				phpVer = fmt.Sprintf("%c.%c", p3[0], p3[1])
			} else if strings.Contains(p3, ".") {
				phpVer = p3
			}
		}
		if len(parts) >= 5 {
			p4 := strings.TrimSpace(parts[4])
			if p4 == "1" {
				createDB = true
			} else if p4 == "0" || strings.EqualFold(p4, "static") {
				kindPart = "static"
			} else if len(p4) == 2 && p4[0] >= '5' && p4[0] <= '8' {
				phpVer = fmt.Sprintf("%c.%c", p4[0], p4[1])
			} else if strings.Contains(p4, ".") {
				phpVer = p4
			}
		}

		in := SiteInput{
			Domains:    domainPart,
			AddWWW:     false,
			Kind:       kindPart,
			RootPath:   rootPart,
			PHPVersion: phpVer,
			Upstream:   upstream,
			AutoSSL:    req.AutoSSL,
			CreateDB:   createDB,
		}
		if createDB {
			cleanDom := strings.NewReplacer(".", "_", "-", "_").Replace(strings.Split(domainPart, "\n")[0])
			if len(cleanDom) > 16 {
				cleanDom = cleanDom[:16]
			}
			in.DBName = cleanDom + "_db"
			in.DBUser = cleanDom + "_u"
			in.DBPass = fmt.Sprintf("Fik_%d_Db!", time.Now().UnixNano()%100000)
		}

		siteObj, err := s.buildSite(r, in, &store.Site{SSLMode: "none", ForceHTTPS: 1, Enabled: 1})
		if err != nil {
			results = append(results, BatchResultItem{Line: line, Domain: domainPart, Kind: kindPart, OK: false, Message: err.Error()})
			continue
		}

		created, err := s.store.CreateSite(r.Context(), siteObj)
		if err != nil {
			results = append(results, BatchResultItem{Line: line, Domain: siteObj.Domain, Kind: kindPart, OK: false, Message: err.Error()})
			continue
		}

		if err := s.sites.EnsureRoot(created); err != nil {
			_ = s.store.DeleteSite(r.Context(), created.ID)
			results = append(results, BatchResultItem{Line: line, Domain: created.Domain, Kind: kindPart, OK: false, Message: err.Error()})
			continue
		}
		if err := s.sites.Apply(r.Context(), created, ""); err != nil {
			_ = s.store.DeleteSite(r.Context(), created.ID)
			results = append(results, BatchResultItem{Line: line, Domain: created.Domain, Kind: kindPart, OK: false, Message: err.Error()})
			continue
		}

		_ = s.applyPostSiteProvisioning(r.Context(), created, in, uid)
		createdCount++
		results = append(results, BatchResultItem{
			Line:    line,
			Domain:  created.Domain,
			Kind:    created.Kind,
			OK:      true,
			Message: "Situs aktif di Nginx (" + sites.DefaultRoot(created.Domain, created.Kind, "fikpanel") + ")",
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":            createdCount > 0,
		"created_count": createdCount,
		"total_lines":   len(results),
		"results":       results,
	})
}

// LaravelAdminUserItem merepresentasikan ringkasan akun pengguna di tabel users proyek Laravel / Filament
type LaravelAdminUserItem struct {
	ID    int64    `json:"id"`
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// resolveProjectDir menentukan direktori induk proyek dari RootPath situs
func resolveProjectDir(rootPath string) string {
	dir := strings.TrimSpace(rootPath)
	if filepath.Base(dir) == "public" {
		return filepath.Dir(dir)
	}
	return dir
}

// provisionLaravelSuperAdmin membuat atau memperbarui akun di tabel users serta memberikan role super_admin & admin (Spatie Permission / Filament Shield)
func (s *Server) provisionLaravelSuperAdmin(ctx context.Context, site *store.Site, adminName, adminEmail, adminPass string) error {
	projectDir := resolveProjectDir(site.RootPath)
	if projectDir == "" {
		return fmt.Errorf("direktori proyek kosong")
	}
	if _, err := os.Stat(filepath.Join(projectDir, "artisan")); err != nil {
		return fmt.Errorf("file artisan tidak ditemukan di %s", projectDir)
	}

	phpScript := `
require "vendor/autoload.php";
$app = require_once "bootstrap/app.php";
$app->make(\Illuminate\Contracts\Console\Kernel::class)->bootstrap();
$name = getenv("FIK_ADMIN_NAME") ?: "Super Admin";
$email = getenv("FIK_ADMIN_EMAIL");
$pass = getenv("FIK_ADMIN_PASS");
$modelClass = "\\App\\Models\\User";
if (!class_exists($modelClass)) {
    fwrite(STDERR, "Model App\\Models\\User tidak ditemukan\n");
    exit(1);
}
$attrs = [
    "name" => $name,
    "password" => \Illuminate\Support\Facades\Hash::make($pass),
];
try {
    $table = (new $modelClass)->getTable();
    $cols = \Illuminate\Support\Facades\Schema::getColumnListing($table);
    if (in_array("email_verified_at", $cols)) { $attrs["email_verified_at"] = now(); }
    if (in_array("is_admin", $cols)) { $attrs["is_admin"] = 1; }
    if (in_array("is_active", $cols)) { $attrs["is_active"] = 1; }
    if (in_array("role", $cols)) { $attrs["role"] = "super_admin"; }
} catch (\Throwable $e) {}
$u = $modelClass::unguarded(fn () => $modelClass::updateOrCreate(["email" => $email], $attrs));
if (class_exists("\\Spatie\\Permission\\Models\\Role") && method_exists($u, "assignRole")) {
    $guard = config("auth.defaults.guard", "web");
    $r = \Spatie\Permission\Models\Role::findOrCreate("super_admin", $guard);
    if (class_exists("\\Spatie\\Permission\\Models\\Permission")) {
        $perms = \Spatie\Permission\Models\Permission::where("guard_name", $guard)->get();
        if ($perms->count() > 0) { $r->syncPermissions($perms); }
    }
    $u->assignRole($r);
    try {
        $rAdmin = \Spatie\Permission\Models\Role::findOrCreate("admin", $guard);
        $u->assignRole($rAdmin);
    } catch (\Throwable $e) {}
    try {
        app(\Spatie\Permission\PermissionRegistrar::class)->forgetCachedPermissions();
    } catch (\Throwable $e) {}
}
echo "OK:" . $u->id;
`
	pctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(pctx, "php", "-r", phpScript)
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(),
		"FIK_ADMIN_NAME="+adminName,
		"FIK_ADMIN_EMAIL="+adminEmail,
		"FIK_ADMIN_PASS="+adminPass,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal membuat akun Super Admin: %s", strings.TrimSpace(string(out)))
	}

	// Pastikan permission cache & log tetap dimiliki oleh www-data
	permCmd := exec.CommandContext(pctx, "sh", "-c", "chown -R www-data:www-data storage bootstrap/cache 2>/dev/null || true")
	permCmd.Dir = projectDir
	_ = permCmd.Run()

	return nil
}

// listLaravelAdminUsers mengambil daftar pengguna beserta role-nya dari proyek Laravel / Filament
func listLaravelAdminUsers(ctx context.Context, projectDir string) ([]LaravelAdminUserItem, bool, bool) {
	hasArtisan := false
	isFilament := false
	if _, err := os.Stat(filepath.Join(projectDir, "artisan")); err == nil {
		hasArtisan = true
	}
	if rawComp, err := os.ReadFile(filepath.Join(projectDir, "composer.json")); err == nil {
		compLower := strings.ToLower(string(rawComp))
		if strings.Contains(compLower, "filament/filament") {
			isFilament = true
			hasArtisan = true
		}
	}
	if !hasArtisan || runtime.GOOS == "windows" {
		return []LaravelAdminUserItem{}, hasArtisan, isFilament
	}

	phpScript := `
require "vendor/autoload.php";
$app = require_once "bootstrap/app.php";
$app->make(\Illuminate\Contracts\Console\Kernel::class)->bootstrap();
$modelClass = "\\App\\Models\\User";
if (!class_exists($modelClass)) { echo "[]"; exit(0); }
$users = $modelClass::query()->orderBy("id", "asc")->limit(25)->get();
$out = [];
foreach ($users as $u) {
    $roles = [];
    if (method_exists($u, "getRoleNames")) {
        try { $roles = $u->getRoleNames()->toArray(); } catch (\Throwable $e) {}
    }
    $out[] = [
        "id" => (int) $u->id,
        "name" => (string) ($u->name ?? ""),
        "email" => (string) ($u->email ?? ""),
        "roles" => array_values($roles),
    ];
}
echo json_encode($out);
`
	lctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(lctx, "php", "-r", phpScript)
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return []LaravelAdminUserItem{}, hasArtisan, isFilament
	}
	var list []LaravelAdminUserItem
	if idx := strings.IndexByte(string(out), '['); idx >= 0 {
		_ = json.Unmarshal(out[idx:], &list)
	}
	if list == nil {
		list = []LaravelAdminUserItem{}
	}
	return list, hasArtisan, isFilament
}

// handleGetSiteLaravelAdmin mengembalikan status proyek Laravel/Filament dan daftar akun admin yang terdaftar di DB proyek
func (s *Server) handleGetSiteLaravelAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := siteID(r)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	site, err := s.store.GetSite(r.Context(), id)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	projectDir := resolveProjectDir(site.RootPath)
	users, isLaravel, isFilament := listLaravelAdminUsers(r.Context(), projectDir)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"is_laravel":  isLaravel,
		"is_filament": isFilament,
		"users":       users,
	})
}

// handleSaveSiteLaravelAdmin membuat atau memperbarui akun Super Admin pada proyek Laravel / Filament milik situs
func (s *Server) handleSaveSiteLaravelAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := siteID(r)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	site, err := s.store.GetSite(r.Context(), id)
	if err != nil {
		writeSiteError(w, err)
		return
	}
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeNginxBody(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Super Admin"
	}
	email := strings.TrimSpace(req.Email)
	pass := req.Password
	if email == "" || len(pass) < 4 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email dan password (minimal 4 karakter) wajib diisi"})
		return
	}

	if err := s.provisionLaravelSuperAdmin(r.Context(), site, name, email, pass); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	s.auditSite(r, "site.laravel.admin", site.Domain, true, fmt.Sprintf("Super Admin dibuat/diperbarui: %s", email))
	projectDir := resolveProjectDir(site.RootPath)
	users, isLaravel, isFilament := listLaravelAdminUsers(r.Context(), projectDir)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"is_laravel":  isLaravel,
		"is_filament": isFilament,
		"users":       users,
	})
}

// SiteDeployProgress merepresentasikan status persentase dan tahapan deployment situs secara real-time
type SiteDeployProgress struct {
	Domain    string `json:"domain"`
	Percent   int    `json:"percent"`
	Stage     string `json:"stage"`
	StepLabel string `json:"step_label"`
	UpdatedAt int64  `json:"updated_at"`
}

var siteDeployProgressMap sync.Map

// updateSiteDeployProgress memperbarui persentase dan deskripsi tahapan deployment untuk domain terkait
func updateSiteDeployProgress(domain string, percent int, stage, stepLabel string) {
	cleanDom := strings.ToLower(strings.TrimSpace(domain))
	prog := SiteDeployProgress{
		Domain:    cleanDom,
		Percent:   percent,
		Stage:     stage,
		StepLabel: stepLabel,
		UpdatedAt: time.Now().UnixMilli(),
	}
	if cleanDom != "" {
		siteDeployProgressMap.Store(cleanDom, prog)
	}
	siteDeployProgressMap.Store("__latest__", prog)
}

// handleGetSiteDeployProgress mengembalikan persentase deployment situs terkini untuk ditampilkan pada progress bar UI
func (s *Server) handleGetSiteDeployProgress(w http.ResponseWriter, r *http.Request) {
	dom := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("domain")))
	if dom != "" {
		if val, ok := siteDeployProgressMap.Load(dom); ok {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"ok":       true,
				"progress": val,
			})
			return
		}
	}
	if val, ok := siteDeployProgressMap.Load("__latest__"); ok {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":       true,
			"progress": val,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"progress": SiteDeployProgress{
			Domain:    dom,
			Percent:   5,
			Stage:     "init",
			StepLabel: "Menyiapkan provisioning situs...",
			UpdatedAt: time.Now().UnixMilli(),
		},
	})
}

// sanitizeCookieSlug mengubah nama domain menjadi identifier aman tanpa titik (.) untuk nama cookie sesi PHP/Laravel
func sanitizeCookieSlug(domain string) string {
	slug := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return '_'
	}, strings.TrimSpace(domain))
	for strings.Contains(slug, "__") {
		slug = strings.ReplaceAll(slug, "__", "_")
	}
	slug = strings.Trim(slug, "_")
	if slug == "" {
		return "laravel"
	}
	return slug
}


