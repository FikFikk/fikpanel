package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/store"
	"golang.org/x/crypto/ssh"
)

// TerminalExecRequest mendefinisikan permintaan eksekusi perintah terminal lokal atau remote
type TerminalExecRequest struct {
	Command  string `json:"command"`
	ServerID int64  `json:"server_id,omitempty"` // 0 atau kosong = server lokal
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
			})
			return
		}
		defer session.Close()

		var stdout, stderr bytes.Buffer
		session.Stdout = &stdout
		session.Stderr = &stderr

		err = session.Run(cmdClean)
		output := stdout.String()
		if stderr.Len() > 0 {
			if len(output) > 0 {
				output += "\n--- STDERR ---\n"
			}
			output += stderr.String()
		}

		if err != nil && len(output) == 0 {
			output = fmt.Sprintf("Error eksekusi: %v", err)
		}

		if len(output) > 65536 {
			output = output[:65536] + "\n... [output dipotong 64KB]"
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":          true,
			"command":     cmdClean,
			"output":      output,
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

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdClean)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", cmdClean)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		if len(output) > 0 {
			output += "\n--- STDERR ---\n"
		}
		output += stderr.String()
	}

	if err != nil && len(output) == 0 {
		output = fmt.Sprintf("Error: %v", err)
	}

	if len(output) > 65536 {
		output = output[:65536] + "\n... [output dipotong 64KB]"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"command":     cmdClean,
		"output":      output,
		"server_id":   0,
		"server_name": "Local server",
		"prompt_user": "root",
		"prompt_host": "fikpanel",
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
