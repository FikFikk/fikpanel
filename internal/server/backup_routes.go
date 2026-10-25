package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/store"
)

func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	configs, err := s.store.ListBackupConfigs(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal mengambil konfigurasi backup"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"configs": configs,
	})
}

func (s *Server) handleCreateBackupConfig(w http.ResponseWriter, r *http.Request) {
	var c store.BackupConfig
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if c.Name == "" || c.TargetName == "" {
		http.Error(w, `{"error":"nama backup dan target wajib diisi"}`, http.StatusBadRequest)
		return
	}
	if c.StorageType == "" {
		c.StorageType = "local"
	}

	id, err := s.store.CreateBackupConfig(r.Context(), &c)
	if err != nil {
		http.Error(w, `{"error":"gagal menyimpan konfigurasi backup"}`, http.StatusInternalServerError)
		return
	}
	c.ID = id

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "backup.config_create", c.Name, true, "Membuat konfigurasi backup", uid)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"config": c,
	})
}

func (s *Server) handleDeleteBackupConfig(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"ID backup tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteBackupConfig(r.Context(), id); err != nil {
		http.Error(w, `{"error":"gagal menghapus konfigurasi backup"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
		"id": id,
	})
}

func (s *Server) handleListBackupRecords(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"ID backup tidak valid"}`, http.StatusBadRequest)
		return
	}

	records, err := s.store.ListBackupRecords(r.Context(), id, 30)
	if err != nil {
		http.Error(w, `{"error":"gagal memuat riwayat arsip backup"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"records": records,
	})
}

func (s *Server) handleRunBackupNow(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"ID backup tidak valid"}`, http.StatusBadRequest)
		return
	}

	cfg, err := s.store.GetBackupConfig(r.Context(), id)
	if err != nil || cfg == nil {
		http.Error(w, `{"error":"konfigurasi backup tidak ditemukan"}`, http.StatusNotFound)
		return
	}

	// Eksekusi backup
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_ = s.executeBackupTask(ctx, cfg)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Tugas backup telah dimulai di background",
	})
}

func (s *Server) executeBackupTask(ctx context.Context, cfg *store.BackupConfig) error {
	backupDir := "/var/backups/fikpanel"
	_ = os.MkdirAll(backupDir, 0755)

	timestamp := time.Now().Format("20060102-150405")
	sanitizedTarget := strings.ReplaceAll(cfg.TargetName, "/", "_")
	fileName := fmt.Sprintf("%s_%s_%s.tar.gz", cfg.TargetType, sanitizedTarget, timestamp)
	targetPath := filepath.Join(backupDir, fileName)

	var runErr error
	if cfg.TargetType == "database" {
		// Dump database sql
		cmd := exec.CommandContext(ctx, "mysqldump", "--all-databases")
		if cfg.TargetName != "*" && cfg.TargetName != "" {
			cmd = exec.CommandContext(ctx, "mysqldump", cfg.TargetName)
		}
		gzipCmd := exec.CommandContext(ctx, "gzip")
		outFile, err := os.Create(targetPath)
		if err == nil {
			defer outFile.Close()
			cmdPipe, _ := cmd.StdoutPipe()
			gzipCmd.Stdin = cmdPipe
			gzipCmd.Stdout = outFile
			_ = cmd.Start()
			_ = gzipCmd.Start()
			_ = cmd.Wait()
			runErr = gzipCmd.Wait()
		} else {
			runErr = err
		}
	} else {
		// Backup direktori website
		srcDir := fmt.Sprintf("/var/www/%s", cfg.TargetName)
		if !strings.HasPrefix(cfg.TargetName, "/") {
			srcDir = fmt.Sprintf("/var/www/%s", cfg.TargetName)
		} else {
			srcDir = cfg.TargetName
		}
		cmd := exec.CommandContext(ctx, "tar", "-czf", targetPath, "-C", filepath.Dir(srcDir), filepath.Base(srcDir))
		runErr = cmd.Run()
	}

	status := "success"
	errMsg := ""
	var fileSize int64 = 0

	if runErr != nil {
		status = "failed"
		errMsg = runErr.Error()
	} else {
		if fi, err := os.Stat(targetPath); err == nil {
			fileSize = fi.Size()
		}
	}

	record := &store.BackupRecord{
		ConfigID:      cfg.ID,
		FileName:      fileName,
		FileSizeBytes: fileSize,
		StorageType:   cfg.StorageType,
		StoragePath:   targetPath,
		Status:        status,
		ErrorMessage:  errMsg,
	}

	_, _ = s.store.CreateBackupRecord(context.Background(), record)
	return runErr
}
