package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/store"
	"golang.org/x/crypto/ssh"
)

const (
	pwdSentinelStart = "__FIK_PWD_START__"
	pwdSentinelEnd   = "__FIK_PWD_END__"
)

// TerminalExecRequest mendefinisikan permintaan eksekusi perintah terminal lokal atau remote
type TerminalExecRequest struct {
	Command  string `json:"command"`
	ServerID int64  `json:"server_id,omitempty"` // 0 atau kosong = server lokal
	Cwd      string `json:"cwd,omitempty"`       // Direktori kerja aktif pada tab terminal
}

// TerminalCompleteRequest mendefinisikan permintaan auto-complete Tab pada terminal
type TerminalCompleteRequest struct {
	Command  string `json:"command"`
	Cwd      string `json:"cwd,omitempty"`
	ServerID int64  `json:"server_id,omitempty"`
}

// TerminalCompletionItem merepresentasikan satu kandidat file/folder hasil Tab completion
type TerminalCompletionItem struct {
	Name        string `json:"name"`
	Display     string `json:"display"`
	Replacement string `json:"replacement"`
	IsDir       bool   `json:"is_dir"`
	Mode        string `json:"mode"`
	Size        int64  `json:"size"`
	ModTime     string `json:"mod_time"`
	Detail      string `json:"detail"`
}

// shellSingleQuote membungkus string dalam tanda kutip tunggal yang aman untuk POSIX shell
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// buildStatefulBashCommand membungkus perintah agar berpindah ke cwd terlebih dahulu dan mencetak pwd akhir
func buildStatefulBashCommand(cwd, cmdClean string) string {
	var sb strings.Builder
	trimmedCwd := strings.TrimSpace(cwd)
	if trimmedCwd != "" && trimmedCwd != "~" {
		if strings.HasPrefix(trimmedCwd, "~/") {
			sb.WriteString("cd ~ && cd " + shellSingleQuote(trimmedCwd[2:]) + " 2>/dev/null\n")
		} else {
			sb.WriteString("cd " + shellSingleQuote(trimmedCwd) + " 2>/dev/null\n")
		}
	} else if trimmedCwd == "~" {
		sb.WriteString("cd ~ 2>/dev/null\n")
	}

	// Tangani otomatis php artisan make:filament-user atau pembuatan User via tinker agar tidak gagal NonInteractive & langsung diberi role super_admin
	isFilamentUserCmd := strings.Contains(cmdClean, "make:filament-user")
	isTinkerUserCmd := strings.Contains(cmdClean, "tinker") && strings.Contains(cmdClean, "User::")
	if isFilamentUserCmd && !strings.Contains(cmdClean, "--name") {
		cmdClean = cmdClean + ` --name="Super Admin" --email="admin@fikpanel.local" --password="password123" --no-interaction`
	}

	sb.WriteString(cmdClean)
	sb.WriteString("\n__FIK_EC=$?\n")

	if isFilamentUserCmd || isTinkerUserCmd {
		// Otomatis berikan role super_admin & izin Filament Shield penuh pada user yang baru dibuat agar tidak terkena 403 Forbidden
		sb.WriteString(`if [ -f artisan ] && [ $__FIK_EC -eq 0 ]; then
  if [ -f app/Models/User.php ] && ! grep -q "FilamentUser\|canAccessPanel" app/Models/User.php 2>/dev/null; then
    sed -i 's/^APP_ENV=production/APP_ENV=local/' .env 2>/dev/null || true
  fi
  php -r '
    require "vendor/autoload.php";
    $app = require_once "bootstrap/app.php";
    $app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
    if (class_exists(App\Models\User::class)) {
      $u = App\Models\User::latest("id")->first();
      if ($u) {
        if (class_exists(App\Models\Company::class)) {
          $comp = App\Models\Company::first();
          if ($comp && empty($u->company_id)) {
            $u->company_id = $comp->id;
            if (array_key_exists("default_company_id", $u->getAttributes())) $u->default_company_id = $comp->id;
          }
        }
        if (array_key_exists("is_multi_company", $u->getAttributes()) || property_exists($u, "is_multi_company")) {
          $u->is_multi_company = true;
        }
        if (method_exists($u, "saveQuietly")) { $u->saveQuietly(); }
        if (method_exists($u, "assignRole") && class_exists(Spatie\Permission\Models\Role::class)) {
          $role = Spatie\Permission\Models\Role::firstOrCreate(["name" => "super_admin", "guard_name" => "web"]);
          if (class_exists(Spatie\Permission\Models\Permission::class)) {
            $perms = Spatie\Permission\Models\Permission::all();
            if ($perms->count() > 0) $role->syncPermissions($perms);
          }
          $u->assignRole($role);
          if (class_exists(Spatie\Permission\PermissionRegistrar::class)) {
            app()[Spatie\Permission\PermissionRegistrar::class]->forgetCachedPermissions();
          }
          echo "\n[FikPanel] Role super_admin & izin akses panel Filament berhasil diberikan ke: " . $u->email . "\n";
        }
      }
    }
  ' 2>/dev/null || true
  php artisan optimize:clear >/dev/null 2>&1 || true
  chown -R www-data:www-data .env storage bootstrap/cache database public 2>/dev/null || true
  chmod -R 775 storage bootstrap/cache database 2>/dev/null || true
fi
`)
	}

	sb.WriteString("printf \"\\n" + pwdSentinelStart + "%s" + pwdSentinelEnd + "\\n\" \"$(pwd)\"\nexit $__FIK_EC\n")
	return sb.String()
}

// extractAndStripPwdSentinel mengekstrak direktori kerja terbaru dari output dan menghapus penanda sentinel
func extractAndStripPwdSentinel(rawStdout, fallbackCwd string) (string, string) {
	resolvedCwd := strings.TrimSpace(fallbackCwd)
	startIdx := strings.LastIndex(rawStdout, pwdSentinelStart)
	if startIdx == -1 {
		return rawStdout, resolvedCwd
	}
	endIdx := strings.Index(rawStdout[startIdx:], pwdSentinelEnd)
	if endIdx == -1 {
		return rawStdout, resolvedCwd
	}
	endIdx += startIdx

	extracted := strings.TrimSpace(rawStdout[startIdx+len(pwdSentinelStart) : endIdx])
	if extracted != "" {
		resolvedCwd = extracted
	}

	// Bersihkan baris sentinel beserta newline pembuka/penutupnya
	prefix := rawStdout[:startIdx]
	prefix = strings.TrimSuffix(prefix, "\r\n")
	prefix = strings.TrimSuffix(prefix, "\n")

	suffix := rawStdout[endIdx+len(pwdSentinelEnd):]
	suffix = strings.TrimPrefix(suffix, "\r\n")
	suffix = strings.TrimPrefix(suffix, "\n")

	clean := prefix + suffix
	return clean, resolvedCwd
}

// ServerTestRequest mendefinisikan permintaan pengujian koneksi SSH remote
type ServerTestRequest struct {
	ServerID   int64  `json:"server_id,omitempty"`
	Host       string `json:"host,omitempty"`
	Port       int    `json:"port,omitempty"`
	Username   string `json:"username,omitempty"`
	AuthType   string `json:"auth_type,omitempty"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

// dialSSH membuka koneksi SSH terenkripsi ke host remote
func dialSSH(host string, port int, username, authType, password, privateKey, passphrase string, timeout time.Duration) (*ssh.Client, error) {
	if port <= 0 {
		port = 22
	}
	if username == "" {
		username = "root"
	}

	var authMethods []ssh.AuthMethod
	if authType == "key" {
		var signer ssh.Signer
		var err error
		keyBytes := []byte(strings.TrimSpace(privateKey))
		if strings.TrimSpace(passphrase) != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}
		if err != nil {
			return nil, fmt.Errorf("kunci privat SSH tidak valid: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.Password(password))
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("koneksi SSH ke %s gagal: %w", addr, err)
	}
	return client, nil
}

// handleTerminalExec mengeksekusi perintah di server lokal atau server SSH remote
func (s *Server) handleTerminalExec(w http.ResponseWriter, r *http.Request) {
	var req TerminalExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload tidak valid"})
		return
	}

	cmdClean := strings.TrimSpace(req.Command)
	if cmdClean == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "perintah kosong"})
		return
	}

	// 1. Eksekusi Remote melalui SSH jika ServerID diberikan
	if req.ServerID > 0 {
		srv, err := s.store.GetTerminalServer(r.Context(), req.ServerID)
		if err != nil || srv == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "server remote tidak ditemukan"})
			return
		}

		client, err := dialSSH(srv.Host, srv.Port, srv.Username, srv.AuthType, srv.Password, srv.PrivateKey, srv.Passphrase, 15*time.Second)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"ok":          false,
				"command":     cmdClean,
				"output":      fmt.Sprintf("Gagal terhubung ke remote server [%s:%d]: %v", srv.Host, srv.Port, err),
				"server_id":   srv.ID,
				"server_name": srv.Name,
				"cwd":         req.Cwd,
			})
			return
		}
		defer client.Close()

		session, err := client.NewSession()
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"ok":          false,
				"command":     cmdClean,
				"output":      fmt.Sprintf("Gagal membuka sesi remote SSH: %v", err),
				"server_id":   srv.ID,
				"server_name": srv.Name,
				"cwd":         req.Cwd,
			})
			return
		}
		defer session.Close()

		var stdout, stderr bytes.Buffer
		session.Stdout = &stdout
		session.Stderr = &stderr

		wrappedRemoteCmd := "bash -c " + shellSingleQuote(buildStatefulBashCommand(req.Cwd, cmdClean))
		err = session.Run(wrappedRemoteCmd)
		output, resolvedCwd := extractAndStripPwdSentinel(stdout.String(), req.Cwd)
		if stderr.Len() > 0 {
			if len(output) > 0 {
				output += "\n"
			}
			output += strings.TrimRight(stderr.String(), "\r\n")
		}

		if err != nil && len(output) == 0 {
			output = fmt.Sprintf("Error eksekusi: %v", err)
		}

		if len(output) > 65536 {
			output = output[:65536] + "\n... [output dipotong 64KB]"
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":          err == nil,
			"command":     cmdClean,
			"output":      output,
			"cwd":         resolvedCwd,
			"server_id":   srv.ID,
			"server_name": srv.Name,
			"prompt_user": srv.Username,
			"prompt_host": srv.Host,
		})
		return
	}

	// 2. Eksekusi Lokal di node fikpanel saat ini
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	baseCwd := strings.TrimSpace(req.Cwd)
	if baseCwd == "" {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			baseCwd = wd
		}
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdClean)
		if baseCwd != "" && baseCwd != "~" {
			cmd.Dir = baseCwd
		}
	} else {
		wrappedLocalCmd := buildStatefulBashCommand(baseCwd, cmdClean)
		cmd = exec.CommandContext(ctx, "bash", "-c", wrappedLocalCmd)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output, resolvedCwd := extractAndStripPwdSentinel(stdout.String(), baseCwd)
	if stderr.Len() > 0 {
		if len(output) > 0 {
			output += "\n"
		}
		output += strings.TrimRight(stderr.String(), "\r\n")
	}

	if err != nil && len(output) == 0 {
		output = fmt.Sprintf("Error: %v", err)
	}

	if len(output) > 65536 {
		output = output[:65536] + "\n... [output dipotong 64KB]"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          err == nil,
		"command":     cmdClean,
		"output":      output,
		"cwd":         resolvedCwd,
		"server_id":   0,
		"server_name": "Local server",
		"prompt_user": "root",
		"prompt_host": "fikpanel",
	})
}

// parseCommandCompletionToken memecah baris input perintah menjadi awalan perintah, direktori parsial, dan prefix nama file
func parseCommandCompletionToken(rawCmd string) (cmdPrefix, rawDirPrefix, filePrefix string, dirsOnly bool) {
	fields := strings.Fields(rawCmd)
	hasTrailingSpace := strings.HasSuffix(rawCmd, " ")

	var partialToken string
	if hasTrailingSpace {
		cmdPrefix = rawCmd
		partialToken = ""
	} else if lastSpace := strings.LastIndexByte(rawCmd, ' '); lastSpace >= 0 {
		cmdPrefix = rawCmd[:lastSpace+1]
		partialToken = rawCmd[lastSpace+1:]
	} else {
		cmdPrefix = ""
		partialToken = rawCmd
	}

	if len(fields) > 0 {
		firstCmd := fields[0]
		if (firstCmd == "cd" || firstCmd == "pushd" || firstCmd == "rmdir") && (hasTrailingSpace || len(fields) >= 2) {
			dirsOnly = true
		}
	}

	normalizedToken := strings.ReplaceAll(partialToken, "\\", "/")
	if lastSlash := strings.LastIndex(normalizedToken, "/"); lastSlash >= 0 {
		rawDirPrefix = normalizedToken[:lastSlash+1]
		filePrefix = normalizedToken[lastSlash+1:]
	} else {
		rawDirPrefix = ""
		filePrefix = normalizedToken
	}

	return cmdPrefix, rawDirPrefix, filePrefix, dirsOnly
}

// longestCommonPrefix menghitung awalan karakter terpanjang yang sama dari daftar nama kandidat
func longestCommonPrefix(items []TerminalCompletionItem) string {
	if len(items) == 0 {
		return ""
	}
	prefix := items[0].Name
	for _, item := range items[1:] {
		for !strings.HasPrefix(item.Name, prefix) {
			if len(prefix) == 0 {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

// formatLsMode membentuk representasi permission gaya ls -la (misal drwxr-xr-x atau -rw-r--r--)
func formatLsMode(mode os.FileMode, isDir bool) string {
	var b [10]byte
	if isDir {
		b[0] = 'd'
	} else if mode&os.ModeSymlink != 0 {
		b[0] = 'l'
	} else {
		b[0] = '-'
	}
	const rwx = "rwxrwxrwx"
	for i := 0; i < 9; i++ {
		if mode&(1<<uint(8-i)) != 0 {
			b[i+1] = rwx[i]
		} else {
			b[i+1] = '-'
		}
	}
	return string(b[:])
}

// handleTerminalComplete menangani fitur auto-complete Tab dan Tab 2x (ls -la kandidat) pada terminal
func (s *Server) handleTerminalComplete(w http.ResponseWriter, r *http.Request) {
	var req TerminalCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload tidak valid"})
		return
	}

	cmdPrefix, rawDirPrefix, filePrefix, dirsOnly := parseCommandCompletionToken(req.Command)
	showHidden := strings.HasPrefix(filePrefix, ".")

	type rawCandidate struct {
		name    string
		isDir   bool
		mode    string
		size    int64
		modTime string
	}

	var rawList []rawCandidate
	resolvedCwd := strings.TrimSpace(req.Cwd)

	if req.ServerID > 0 {
		// Auto-complete pada server SSH remote
		srv, err := s.store.GetTerminalServer(r.Context(), req.ServerID)
		if err != nil || srv == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "server remote tidak ditemukan"})
			return
		}

		client, err := dialSSH(srv.Host, srv.Port, srv.Username, srv.AuthType, srv.Password, srv.PrivateKey, srv.Passphrase, 6*time.Second)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"ok":                false,
				"matches":           []TerminalCompletionItem{},
				"completed_command": req.Command,
				"cwd":               resolvedCwd,
			})
			return
		}
		defer client.Close()

		session, err := client.NewSession()
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"ok":                false,
				"matches":           []TerminalCompletionItem{},
				"completed_command": req.Command,
				"cwd":               resolvedCwd,
			})
			return
		}
		defer session.Close()

		var script strings.Builder
		if resolvedCwd != "" && resolvedCwd != "~" {
			if strings.HasPrefix(resolvedCwd, "~/") {
				script.WriteString("cd ~ && cd " + shellSingleQuote(resolvedCwd[2:]) + " 2>/dev/null\n")
			} else {
				script.WriteString("cd " + shellSingleQuote(resolvedCwd) + " 2>/dev/null\n")
			}
		}
		script.WriteString("pwd\n")
		if rawDirPrefix != "" {
			if rawDirPrefix == "~/" {
				script.WriteString("cd ~ 2>/dev/null || exit 0\n")
			} else if strings.HasPrefix(rawDirPrefix, "~/") {
				script.WriteString("cd ~ && cd " + shellSingleQuote(rawDirPrefix[2:]) + " 2>/dev/null || exit 0\n")
			} else {
				script.WriteString("cd " + shellSingleQuote(rawDirPrefix) + " 2>/dev/null || exit 0\n")
			}
		}
		script.WriteString("find -L . -maxdepth 1 -mindepth 1 -printf '%f\\t%y\\t%M\\t%s\\t%Tb %Td %TH:%TM\\n' 2>/dev/null\n")

		out, _ := session.Output("bash -c " + shellSingleQuote(script.String()))
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "/") {
			resolvedCwd = strings.TrimSpace(lines[0])
			lines = lines[1:]
		}
		for _, line := range lines {
			parts := strings.Split(line, "\t")
			if len(parts) < 5 {
				continue
			}
			name := parts[0]
			isDir := parts[1] == "d"
			mode := parts[2]
			size, _ := strconv.ParseInt(parts[3], 10, 64)
			modTime := parts[4]
			rawList = append(rawList, rawCandidate{
				name:    name,
				isDir:   isDir,
				mode:    mode,
				size:    size,
				modTime: modTime,
			})
		}
	} else {
		// Auto-complete pada server lokal
		homeDir, _ := os.UserHomeDir()
		if resolvedCwd == "" {
			if wd, err := os.Getwd(); err == nil {
				resolvedCwd = wd
			}
		} else if resolvedCwd == "~" {
			resolvedCwd = homeDir
		} else if strings.HasPrefix(resolvedCwd, "~/") {
			resolvedCwd = filepath.Join(homeDir, resolvedCwd[2:])
		}

		searchDir := resolvedCwd
		if rawDirPrefix != "" {
			if rawDirPrefix == "~/" {
				searchDir = homeDir
			} else if strings.HasPrefix(rawDirPrefix, "~/") {
				searchDir = filepath.Join(homeDir, rawDirPrefix[2:])
			} else if strings.HasPrefix(rawDirPrefix, "/") || filepath.IsAbs(rawDirPrefix) {
				searchDir = filepath.Clean(rawDirPrefix)
			} else {
				searchDir = filepath.Join(resolvedCwd, rawDirPrefix)
			}
		}

		entries, err := os.ReadDir(searchDir)
		if err == nil {
			for _, entry := range entries {
				name := entry.Name()
				info, infoErr := os.Stat(filepath.Join(searchDir, name))
				if infoErr != nil {
					info, infoErr = entry.Info()
				}
				isDir := entry.IsDir()
				var mode os.FileMode = 0644
				var size int64
				modTime := time.Now().Format("Jan 02 15:04")
				if infoErr == nil && info != nil {
					isDir = info.IsDir()
					mode = info.Mode()
					size = info.Size()
					modTime = info.ModTime().Format("Jan 02 15:04")
				}
				rawList = append(rawList, rawCandidate{
					name:    name,
					isDir:   isDir,
					mode:    formatLsMode(mode, isDir),
					size:    size,
					modTime: modTime,
				})
			}
		}
	}

	// Filter kandidat berdasarkan prefix & dirsOnly
	filterMatches := func(caseInsensitive bool) []TerminalCompletionItem {
		var result []TerminalCompletionItem
		for _, cand := range rawList {
			if !showHidden && strings.HasPrefix(cand.name, ".") {
				continue
			}
			if dirsOnly && !cand.isDir {
				continue
			}
			if caseInsensitive {
				if !strings.HasPrefix(strings.ToLower(cand.name), strings.ToLower(filePrefix)) {
					continue
				}
			} else {
				if !strings.HasPrefix(cand.name, filePrefix) {
					continue
				}
			}

			display := cand.name
			replacement := rawDirPrefix + cand.name
			nlink := 1
			if cand.isDir {
				display = cand.name + "/"
				replacement = rawDirPrefix + cand.name + "/"
				nlink = 2
			} else {
				replacement = rawDirPrefix + cand.name + " "
			}

			detail := fmt.Sprintf("%s %2d root root %8d %s %s", cand.mode, nlink, cand.size, cand.modTime, display)
			result = append(result, TerminalCompletionItem{
				Name:        cand.name,
				Display:     display,
				Replacement: cmdPrefix + replacement,
				IsDir:       cand.isDir,
				Mode:        cand.mode,
				Size:        cand.size,
				ModTime:     cand.modTime,
				Detail:      detail,
			})
		}
		return result
	}

	matches := filterMatches(false)
	if len(matches) == 0 && filePrefix != "" {
		matches = filterMatches(true)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Name < matches[j].Name
	})

	completedCommand := req.Command
	commonPrefix := filePrefix
	if len(matches) == 1 {
		completedCommand = matches[0].Replacement
		commonPrefix = matches[0].Name
	} else if len(matches) > 1 {
		lcp := longestCommonPrefix(matches)
		commonPrefix = lcp
		if len(lcp) > len(filePrefix) {
			completedCommand = cmdPrefix + rawDirPrefix + lcp
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                true,
		"cwd":               resolvedCwd,
		"prefix":            filePrefix,
		"common_prefix":     commonPrefix,
		"completed_command": completedCommand,
		"dirs_only":         dirsOnly,
		"matches":           matches,
	})
}

// handleListTerminalServers mengembalikan seluruh server SSH remote yang terdaftar
func (s *Server) handleListTerminalServers(w http.ResponseWriter, r *http.Request) {
	servers, err := s.store.ListTerminalServers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Mask kredensial sensitif untuk respons API
	type SanitizedServer struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Host        string `json:"host"`
		Port        int    `json:"port"`
		Username    string `json:"username"`
		AuthType    string `json:"auth_type"`
		HasPassword bool   `json:"has_password"`
		HasKey      bool   `json:"has_key"`
		Remarks     string `json:"remarks"`
		CreatedAt   int64  `json:"created_at"`
		UpdatedAt   int64  `json:"updated_at"`
	}

	sanitized := make([]SanitizedServer, 0, len(servers))
	for _, srv := range servers {
		sanitized = append(sanitized, SanitizedServer{
			ID:          srv.ID,
			Name:        srv.Name,
			Host:        srv.Host,
			Port:        srv.Port,
			Username:    srv.Username,
			AuthType:    srv.AuthType,
			HasPassword: srv.Password != "",
			HasKey:      srv.PrivateKey != "",
			Remarks:     srv.Remarks,
			CreatedAt:   srv.CreatedAt,
			UpdatedAt:   srv.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, sanitized)
}

// handleCreateTerminalServer menambahkan konfigurasi server remote baru
func (s *Server) handleCreateTerminalServer(w http.ResponseWriter, r *http.Request) {
	var srv store.TerminalServer
	if err := json.NewDecoder(r.Body).Decode(&srv); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "format JSON tidak valid"})
		return
	}

	srv.Host = strings.TrimSpace(srv.Host)
	if srv.Host == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "alamat host server wajib diisi"})
		return
	}
	if srv.Port <= 0 {
		srv.Port = 22
	}
	if srv.Username == "" {
		srv.Username = "root"
	}
	if srv.AuthType == "" {
		srv.AuthType = "password"
	}
	if srv.Name == "" {
		srv.Name = srv.Host
	}

	id, err := s.store.CreateTerminalServer(r.Context(), &srv)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"ok": true,
		"id": id,
	})
}

// handleUpdateTerminalServer memperbarui konfigurasi server remote
func (s *Server) handleUpdateTerminalServer(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID server tidak valid"})
		return
	}

	existing, err := s.store.GetTerminalServer(r.Context(), id)
	if err != nil || existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "server tidak ditemukan"})
		return
	}

	var req store.TerminalServer
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "format JSON tidak valid"})
		return
	}

	req.ID = id
	req.Host = strings.TrimSpace(req.Host)
	if req.Host == "" {
		req.Host = existing.Host
	}
	if req.Name == "" {
		req.Name = existing.Name
	}
	if req.Port <= 0 {
		req.Port = existing.Port
	}
	if req.Username == "" {
		req.Username = existing.Username
	}
	if req.AuthType == "" {
		req.AuthType = existing.AuthType
	}
	// Jika password atau kunci kosong pada payload update, pertahankan nilai sebelumnya
	if req.Password == "" && req.AuthType == existing.AuthType {
		req.Password = existing.Password
	}
	if req.PrivateKey == "" && req.AuthType == existing.AuthType {
		req.PrivateKey = existing.PrivateKey
		req.Passphrase = existing.Passphrase
	}

	if err := s.store.UpdateTerminalServer(r.Context(), &req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"id": id,
	})
}

// handleDeleteTerminalServer menghapus konfigurasi server remote
func (s *Server) handleDeleteTerminalServer(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID server tidak valid"})
		return
	}

	if err := s.store.DeleteTerminalServer(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"id": id,
	})
}

// handleTestTerminalServer menguji konektivitas SSH secara langsung dan mengukur latensi
func (s *Server) handleTestTerminalServer(w http.ResponseWriter, r *http.Request) {
	var req ServerTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "format JSON tidak valid"})
		return
	}

	host := strings.TrimSpace(req.Host)
	port := req.Port
	username := req.Username
	authType := req.AuthType
	password := req.Password
	privateKey := req.PrivateKey
	passphrase := req.Passphrase

	// Jika ServerID diberikan, muat data dari database
	if req.ServerID > 0 {
		srv, err := s.store.GetTerminalServer(r.Context(), req.ServerID)
		if err != nil || srv == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "server tidak ditemukan"})
			return
		}
		host = srv.Host
		port = srv.Port
		username = srv.Username
		authType = srv.AuthType
		password = srv.Password
		privateKey = srv.PrivateKey
		passphrase = srv.Passphrase
	}

	if host == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "host server wajib diisi"})
		return
	}
	if port <= 0 {
		port = 22
	}
	if username == "" {
		username = "root"
	}
	if authType == "" {
		authType = "password"
	}

	start := time.Now()
	client, err := dialSSH(host, port, username, authType, password, privateKey, passphrase, 6*time.Second)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":         false,
			"latency_ms": latency,
			"error":      fmt.Sprintf("Gagal terhubung: %v", err),
		})
		return
	}
	defer client.Close()

	// Coba ambil banner OS singkat dari sesi remote
	var banner string
	session, err := client.NewSession()
	if err == nil {
		defer session.Close()
		out, runErr := session.Output("uname -srm 2>/dev/null || echo 'SSH Connected'")
		if runErr == nil {
			banner = strings.TrimSpace(string(out))
		}
	}
	if banner == "" {
		banner = string(client.ServerVersion())
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"latency_ms": latency,
		"banner":     banner,
		"message":    fmt.Sprintf("Koneksi berhasil (%d ms)", latency),
	})
}

// handleListTerminalCommands mengembalikan daftar pintasan perintah tersimpan
func (s *Server) handleListTerminalCommands(w http.ResponseWriter, r *http.Request) {
	cmds, err := s.store.ListTerminalCommands(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if cmds == nil {
		cmds = []store.TerminalCommand{}
	}
	writeJSON(w, http.StatusOK, cmds)
}

// handleCreateTerminalCommand menyimpan pintasan perintah baru
func (s *Server) handleCreateTerminalCommand(w http.ResponseWriter, r *http.Request) {
	var c store.TerminalCommand
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "format JSON tidak valid"})
		return
	}

	c.Name = strings.TrimSpace(c.Name)
	c.Content = strings.TrimSpace(c.Content)
	if c.Name == "" || c.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nama dan konten perintah wajib diisi"})
		return
	}

	id, err := s.store.CreateTerminalCommand(r.Context(), &c)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"ok": true,
		"id": id,
	})
}

// handleUpdateTerminalCommand memperbarui pintasan perintah yang ada
func (s *Server) handleUpdateTerminalCommand(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID perintah tidak valid"})
		return
	}

	var c store.TerminalCommand
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "format JSON tidak valid"})
		return
	}

	c.ID = id
	c.Name = strings.TrimSpace(c.Name)
	c.Content = strings.TrimSpace(c.Content)
	if c.Name == "" || c.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nama dan konten perintah wajib diisi"})
		return
	}

	if err := s.store.UpdateTerminalCommand(r.Context(), &c); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"id": id,
	})
}

// handleDeleteTerminalCommand menghapus pintasan perintah
func (s *Server) handleDeleteTerminalCommand(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID perintah tidak valid"})
		return
	}

	if err := s.store.DeleteTerminalCommand(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"id": id,
	})
}
