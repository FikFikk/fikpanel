package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":              true,
		"connected":       connected,
		"username":        username,
		"avatar_url":      avatar,
		"mode":            mode, // "token" (Public + Private) atau "public" (Public repos saja)
		"has_token":       token != "",
		"oauth_client_id": clientID,
		"ssh_public_key":  sshKey,
	})
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"branches": branches,
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
// 1. Deploy Template (Laravel, WordPress, Express, Gin, FastAPI, SPA)
// 2. Git Clone (termasuk repositori privat menggunakan token GitHub tersimpan) + Post-Clone Build Command
// 3. Ekstraksi Arsip Backup (.tar.gz / .zip)
// 4. Pembuatan Database MySQL Otomatis
// 5. Penerbitan SSL Let's Encrypt Otomatis
func (s *Server) applyPostSiteProvisioning(ctx context.Context, created *store.Site, in SiteInput, uid *int64) map[string]string {
	notes := map[string]string{}

	// 1. One-Click Deploy Template
	if tpl := strings.TrimSpace(strings.ToLower(in.DeployTemplate)); tpl != "" && created.RootPath != "" && runtime.GOOS != "windows" {
		if err := provisionDeployTemplate(created, tpl, in.DBName, in.DBUser, in.DBPass); err != nil {
			notes["template_warning"] = err.Error()
		} else {
			notes["template"] = tpl
		}
	}

	// 2. Git Clone Repository
	if repoURL := strings.TrimSpace(in.GitRepoURL); repoURL != "" && created.RootPath != "" && runtime.GOOS != "windows" {
		token, _ := s.store.GetSetting(ctx, "github_token")
		if err := cloneGitRepoToSite(ctx, created, repoURL, strings.TrimSpace(in.GitBranch), strings.TrimSpace(in.GitPostCmd), token); err != nil {
			notes["git_warning"] = err.Error()
		} else {
			notes["git"] = "cloned"
		}
	}

	// 3. Restore dari Arsip Backup
	if archivePath := strings.TrimSpace(in.BackupArchive); archivePath != "" && created.RootPath != "" && runtime.GOOS != "windows" {
		if err := extractBackupToSite(ctx, created, archivePath); err != nil {
			notes["backup_warning"] = err.Error()
		} else {
			notes["backup"] = "restored"
		}
	}

	// 4. Auto-Create Database MySQL
	if in.CreateDB && strings.TrimSpace(in.DBName) != "" && strings.TrimSpace(in.DBUser) != "" && strings.TrimSpace(in.DBPass) != "" {
		dbName := strings.TrimSpace(in.DBName)
		dbUser := strings.TrimSpace(in.DBUser)
		dbPass := strings.TrimSpace(in.DBPass)
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

	// 5. Auto-Issue Let's Encrypt SSL
	if in.AutoSSL {
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

	return notes
}

// cloneGitRepoToSite mengkloning repositori Git ke direktori proyek situs
func cloneGitRepoToSite(ctx context.Context, site *store.Site, repoURL, branch, postCmd, githubToken string) error {
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

	// Pastikan folder public tersedia jika root_path berakhir dengan /public tetapi repo tidak memiliki sub-folder public
	if strings.HasSuffix(site.RootPath, "/public") {
		if _, err := os.Stat(site.RootPath); os.IsNotExist(err) {
			_ = os.MkdirAll(site.RootPath, 0o755)
		}
	}

	if postCmd != "" {
		bctx, bcancel := context.WithTimeout(ctx, 120*time.Second)
		defer bcancel()
		buildCmd := exec.CommandContext(bctx, "sh", "-c", postCmd)
		buildCmd.Dir = targetDir
		_, _ = buildCmd.CombinedOutput()
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
		envContent := fmt.Sprintf("APP_NAME=\"%s\"\nAPP_ENV=production\nAPP_DEBUG=false\nAPP_URL=https://%s\n\nDB_CONNECTION=mysql\nDB_HOST=127.0.0.1\nDB_PORT=3306\nDB_DATABASE=%s\nDB_USERNAME=%s\nDB_PASSWORD=%s\n",
			site.Domain, site.Domain, dbName, dbUser, dbPass)
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

// extractBackupToSite mengekstrak arsip .tar.gz / .tgz / .zip ke direktori root situs
func extractBackupToSite(ctx context.Context, site *store.Site, archivePath string) error {
	cleanPath := filepath.Clean(archivePath)
	if _, err := os.Stat(cleanPath); err != nil {
		return fmt.Errorf("berkas arsip tidak ditemukan: %s", cleanPath)
	}
	_ = os.MkdirAll(site.RootPath, 0o755)
	_ = os.Remove(filepath.Join(site.RootPath, "index.html"))

	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	lower := strings.ToLower(cleanPath)
	var cmd *exec.Cmd
	if strings.HasSuffix(lower, ".zip") {
		cmd = exec.CommandContext(cctx, "unzip", "-o", cleanPath, "-d", site.RootPath)
	} else {
		cmd = exec.CommandContext(cctx, "tar", "-xzf", cleanPath, "-C", site.RootPath)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gagal mengekstrak arsip: %s", strings.TrimSpace(string(out)))
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
