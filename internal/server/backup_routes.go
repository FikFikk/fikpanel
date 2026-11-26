package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fikpanel/internal/store"
)

// handleListBackups mengembalikan daftar seluruh profil kebijakan pencadangan
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

// handleCreateBackupConfig menyimpan profil kebijakan pencadangan baru (Lokal atau S3/MinIO/R2)
func (s *Server) handleCreateBackupConfig(w http.ResponseWriter, r *http.Request) {
	var c store.BackupConfig
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.TargetName) == "" {
		http.Error(w, `{"error":"nama backup dan target wajib diisi"}`, http.StatusBadRequest)
		return
	}
	if c.StorageType == "" {
		c.StorageType = "local"
	}
	if c.StorageType == "s3" {
		if strings.TrimSpace(c.S3Bucket) == "" || strings.TrimSpace(c.S3AccessKey) == "" || strings.TrimSpace(c.S3SecretKey) == "" {
			http.Error(w, `{"error":"bucket, access key, dan secret key wajib diisi untuk penyimpanan S3"}`, http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(c.S3Region) == "" {
			c.S3Region = "us-east-1"
		}
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

// handleDeleteBackupConfig menghapus profil kebijakan pencadangan beserta daftar riwayatnya
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

// handleListBackupRecords mengembalikan riwayat arsip snapshot milik satu kebijakan backup
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

// handleDownloadBackupRecord melayani pengunduhan berkas lokal dari satu catatan riwayat backup
func (s *Server) handleDownloadBackupRecord(w http.ResponseWriter, r *http.Request) {
	recID, err := strconv.ParseInt(r.PathValue("record_id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID riwayat backup tidak valid"})
		return
	}
	rec, err := s.store.GetBackupRecord(r.Context(), recID)
	if err != nil || rec == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "riwayat arsip backup tidak ditemukan"})
		return
	}

	// Pastikan berkas fisik tersedia di direktori backup panel
	base, safePath, err := validateBackupFileName(rec.FileName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if _, err := os.Stat(safePath); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "berkas arsip lokal tidak ditemukan (mungkin sudah dipindahkan ke S3 atau dihapus)"})
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, base))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, safePath)
}

// handleDeleteBackupRecord menghapus catatan riwayat backup beserta berkas fisiknya bila tersimpan di disk lokal
func (s *Server) handleDeleteBackupRecord(w http.ResponseWriter, r *http.Request) {
	recID, err := strconv.ParseInt(r.PathValue("record_id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ID riwayat backup tidak valid"})
		return
	}
	rec, err := s.store.GetBackupRecord(r.Context(), recID)
	if err != nil || rec == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "riwayat arsip backup tidak ditemukan"})
		return
	}

	if _, safePath, err := validateBackupFileName(rec.FileName); err == nil {
		_ = os.Remove(safePath)
	}
	if err := s.store.DeleteBackupRecord(r.Context(), recID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "gagal menghapus riwayat backup"})
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "backup.record_delete", rec.FileName, true, "Menghapus snapshot backup", nil)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Snapshot %s berhasil dihapus", rec.FileName),
	})
}

// handleRunBackupNow memicu eksekusi snapshot backup di latar belakang
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

// executeBackupTask mengeksekusi pencadangan database/direktori, unggah S3 SigV4 jika dipilih, serta menerapkan retensi
func (s *Server) executeBackupTask(ctx context.Context, cfg *store.BackupConfig) error {
	dir := backupDirPath()
	_ = os.MkdirAll(dir, 0755)

	timestamp := time.Now().Format("20060102-150405")
	sanitizedTarget := strings.ReplaceAll(strings.TrimPrefix(cfg.TargetName, "pg:"), "/", "_")
	sanitizedTarget = strings.ReplaceAll(sanitizedTarget, "*", "all")

	ext := "tar.gz"
	if cfg.TargetType == "database" {
		ext = "sql.gz"
	}
	fileName := fmt.Sprintf("%s_%s_%s.%s", cfg.TargetType, sanitizedTarget, timestamp, ext)
	targetPath := filepath.Join(dir, fileName)

	var runErr error
	if runtime.GOOS != "linux" {
		runErr = os.WriteFile(targetPath, []byte("FikPanel simulated archive for "+cfg.TargetName), 0644)
	} else if cfg.TargetType == "database" {
		engine := "mysql"
		targetDB := strings.TrimSpace(cfg.TargetName)
		if strings.HasPrefix(targetDB, "pg:") {
			engine = "postgresql"
			targetDB = strings.TrimPrefix(targetDB, "pg:")
		}
		runErr = dumpSingleDatabaseToGzip(ctx, engine, targetDB, targetPath)
	} else {
		// Backup direktori website atau direktori kustom
		srcDir := cfg.TargetName
		if !strings.HasPrefix(srcDir, "/") {
			srcDir = fmt.Sprintf("/var/www/%s", cfg.TargetName)
		}
		cmd := exec.CommandContext(ctx, "tar", "-czf", targetPath, "-C", filepath.Dir(srcDir), filepath.Base(srcDir))
		if out, err := cmd.CombinedOutput(); err != nil {
			runErr = fmt.Errorf("tar gagal: %w (%s)", err, strings.TrimSpace(string(out)))
		}
	}

	status := "success"
	errMsg := ""
	var fileSize int64
	storagePath := targetPath

	if runErr != nil {
		status = "failed"
		errMsg = runErr.Error()
		_ = os.Remove(targetPath)
	} else {
		if fi, err := os.Stat(targetPath); err == nil {
			fileSize = fi.Size()
		}
		// Unggah ke S3 / MinIO / Cloudflare R2 menggunakan AWS Signature Version 4
		if strings.EqualFold(cfg.StorageType, "s3") {
			objectKey := fmt.Sprintf("fikpanel-backups/%s", fileName)
			s3URL, upErr := uploadFileToS3(ctx, cfg, objectKey, targetPath)
			if upErr != nil {
				status = "failed"
				errMsg = fmt.Sprintf("backup lokal sukses, namun unggah S3 gagal: %v", upErr)
				runErr = upErr
			} else {
				storagePath = s3URL
			}
		}
	}

	record := &store.BackupRecord{
		ConfigID:      cfg.ID,
		FileName:      fileName,
		FileSizeBytes: fileSize,
		StorageType:   cfg.StorageType,
		StoragePath:   storagePath,
		Status:        status,
		ErrorMessage:  errMsg,
	}

	_, _ = s.store.CreateBackupRecord(context.Background(), record)

	// Terapkan batas retensi keep_copies untuk menghapus salinan lama yang melebihi kuota
	if status == "success" && cfg.KeepCopies > 0 {
		prunedPaths, pruneErr := s.store.PruneBackupRecords(context.Background(), cfg.ID, cfg.KeepCopies)
		if pruneErr == nil {
			for _, p := range prunedPaths {
				if !strings.HasPrefix(p, "http://") && !strings.HasPrefix(p, "https://") && !strings.HasPrefix(p, "s3://") {
					_ = os.Remove(p)
				}
			}
		}
	}

	return runErr
}

// hmacSHA256 menghitung HMAC-SHA256 mentah untuk derivasi kunci AWS Signature v4
func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// signS3PutRequest menandatangani permintaan HTTP PUT S3 menggunakan AWS Signature Version 4 (kompatibel AWS S3, MinIO, R2, B2)
func signS3PutRequest(req *http.Request, accessKey, secretKey, region, payloadHash string, now time.Time) {
	if region == "" {
		region = "us-east-1"
	}
	amzDate := now.UTC().Format("20060102T150405Z")
	dateStamp := now.UTC().Format("20060102")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}

	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", host, payloadHash, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	crHash := sha256.Sum256([]byte(canonicalRequest))
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hex.EncodeToString(crHash[:]),
	}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, signedHeaders, signature,
	)
	req.Header.Set("Authorization", authHeader)
}

// buildS3ObjectURL membangun endpoint path-style yang kompatibel dengan MinIO, Cloudflare R2, dan AWS S3
func buildS3ObjectURL(endpoint, bucket, region, objectKey string) (string, error) {
	ep := strings.TrimSpace(endpoint)
	if ep == "" {
		if region == "" {
			region = "us-east-1"
		}
		ep = fmt.Sprintf("https://s3.%s.amazonaws.com", region)
	}
	if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
		ep = "https://" + ep
	}
	u, err := url.Parse(strings.TrimRight(ep, "/"))
	if err != nil {
		return "", fmt.Errorf("format S3 endpoint tidak valid: %w", err)
	}
	cleanBucket := strings.Trim(bucket, "/")
	cleanKey := strings.TrimLeft(objectKey, "/")
	u.Path = "/" + cleanBucket + "/" + cleanKey
	return u.String(), nil
}

// uploadFileToS3 mengunggah berkas arsip ke penyimpanan objek S3/MinIO/R2 dengan AWS Signature v4
func uploadFileToS3(ctx context.Context, cfg *store.BackupConfig, objectKey, filePath string) (string, error) {
	targetURL, err := buildS3ObjectURL(cfg.S3Endpoint, cfg.S3Bucket, cfg.S3Region, objectKey)
	if err != nil {
		return "", err
	}

	// Hitung SHA256 payload untuk tanda tangan AWS SigV4 yang diterima ketat oleh AWS/R2/MinIO
	fHash, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("gagal membuka arsip untuk unggah S3: %w", err)
	}
	hasher := sha256.New()
	size, err := io.Copy(hasher, fHash)
	_ = fHash.Close()
	if err != nil {
		return "", fmt.Errorf("gagal menghitung checksum arsip: %w", err)
	}
	payloadHash := hex.EncodeToString(hasher.Sum(nil))

	fBody, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer fBody.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, targetURL, fBody)
	if err != nil {
		return "", err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/gzip")
	signS3PutRequest(req, strings.TrimSpace(cfg.S3AccessKey), strings.TrimSpace(cfg.S3SecretKey), strings.TrimSpace(cfg.S3Region), payloadHash, time.Now())

	client := &http.Client{Timeout: 8 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("koneksi ke S3 gagal: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("S3 menolak unggahan (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(bodySnippet)))
	}

	return fmt.Sprintf("s3://%s/%s", strings.Trim(cfg.S3Bucket, "/"), strings.TrimLeft(objectKey, "/")), nil
}
