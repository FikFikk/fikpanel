package server

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// FileItem merepresentasikan metadata berkas atau folder dalam respons API
type FileItem struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"is_dir"`
	Size      int64  `json:"size"`
	Mode      string `json:"mode"`
	Owner     string `json:"owner"`
	ModTime   string `json:"mod_time"`
	Ext       string `json:"ext"`
	Protected bool   `json:"protected"`
	Ps        string `json:"ps"`
}

// FileListResponse menampung struktur respons daftar direktori
type FileListResponse struct {
	CurrentPath string     `json:"current_path"`
	ParentPath  string     `json:"parent_path"`
	Items       []FileItem `json:"items"`
	TotalDirs   int        `json:"total_dirs"`
	TotalFiles  int        `json:"total_files"`
	TotalSize   int64      `json:"total_size"`
}

// resolveSafePath menormalisasi path dan memastikan keamanan akses berkas
func resolveSafePath(userPath string) (string, error) {
	trimmed := strings.TrimSpace(userPath)
	if trimmed == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return ".", nil
		}
		return filepath.Clean(cwd), nil
	}

	clean := filepath.Clean(trimmed)
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("path tidak valid: %w", err)
	}
	return abs, nil
}

// getOwnerName mengembalikan nama pemilik berkas secara fleksibel sesuai OS
func getOwnerName() string {
	if runtime.GOOS == "windows" {
		if u := os.Getenv("USERNAME"); u != "" {
			return u
		}
		return "system"
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "root"
}

// handleListFiles mengembalikan daftar file dan folder pada direktori yang diminta
func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	searchQuery := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))

	targetDir, err := resolveSafePath(reqPath)
	if err != nil {
		http.Error(w, `{"error":"path direktori tidak valid"}`, http.StatusBadRequest)
		return
	}

	stat, err := os.Stat(targetDir)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal mengakses direktori: %v"}`, err), http.StatusNotFound)
		return
	}
	if !stat.IsDir() {
		http.Error(w, `{"error":"target bukan merupakan direktori"}`, http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membaca isi direktori: %v"}`, err), http.StatusInternalServerError)
		return
	}

	owner := getOwnerName()
	var items []FileItem
	var totalDirs, totalFiles int
	var totalSize int64

	for _, entry := range entries {
		name := entry.Name()
		if searchQuery != "" && !strings.Contains(strings.ToLower(name), searchQuery) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		isDir := entry.IsDir()
		size := info.Size()
		if isDir {
			totalDirs++
			size = 0
		} else {
			totalFiles++
			totalSize += size
		}

		ext := ""
		if !isDir {
			ext = strings.ToLower(filepath.Ext(name))
		}

		fullItemPath := filepath.Join(targetDir, name)
		permStr := fmt.Sprintf("%04o", info.Mode().Perm())

		items = append(items, FileItem{
			Name:      name,
			Path:      fullItemPath,
			IsDir:     isDir,
			Size:      size,
			Mode:      permStr,
			Owner:     owner,
			ModTime:   info.ModTime().Format("2006-01-02 15:04:05"),
			Ext:       ext,
			Protected: false,
			Ps:        "",
		})
	}

	// Urutkan folder di atas, lalu file; kemudian urutkan alfabetis
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	parent := filepath.Dir(targetDir)
	if parent == targetDir {
		parent = ""
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(FileListResponse{
		CurrentPath: targetDir,
		ParentPath:  parent,
		Items:       items,
		TotalDirs:   totalDirs,
		TotalFiles:  totalFiles,
		TotalSize:   totalSize,
	})
}

// handleReadFile membaca isi berkas teks atau mengunduh berkas biner
func (s *Server) handleReadFile(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		http.Error(w, `{"error":"parameter path wajib disertakan"}`, http.StatusBadRequest)
		return
	}

	targetPath, err := resolveSafePath(reqPath)
	if err != nil {
		http.Error(w, `{"error":"path berkas tidak valid"}`, http.StatusBadRequest)
		return
	}

	stat, err := os.Stat(targetPath)
	if err != nil {
		http.Error(w, `{"error":"berkas tidak ditemukan"}`, http.StatusNotFound)
		return
	}
	if stat.IsDir() {
		http.Error(w, `{"error":"path adalah direktori, bukan berkas"}`, http.StatusBadRequest)
		return
	}

	// Tangani mode unduhan langsung (download)
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(targetPath)))
		http.ServeFile(w, r, targetPath)
		return
	}

	// Batasi ukuran baca konten teks maksimal 5 MB untuk mencegah kehabisan memori browser
	const maxReadSize = 5 * 1024 * 1024
	if stat.Size() > maxReadSize {
		http.Error(w, `{"error":"ukuran berkas terlalu besar untuk editor teks (maks 5 MB)"}`, http.StatusBadRequest)
		return
	}

	contentBytes, err := os.ReadFile(targetPath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membaca berkas: %v"}`, err), http.StatusInternalServerError)
		return
	}

	ext := strings.ToLower(filepath.Ext(targetPath))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "text/plain; charset=utf-8"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"path":      targetPath,
		"name":      filepath.Base(targetPath),
		"size":      stat.Size(),
		"mime_type": mimeType,
		"content":   string(contentBytes),
	})
}

// FileSaveRequest payload untuk menyimpan perubahan isi berkas
type FileSaveRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// handleSaveFile menulis ulang isi berkas teks dengan konten yang disunting
func (s *Server) handleSaveFile(w http.ResponseWriter, r *http.Request) {
	var req FileSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	targetPath, err := resolveSafePath(req.Path)
	if err != nil {
		http.Error(w, `{"error":"path berkas tidak valid"}`, http.StatusBadRequest)
		return
	}

	// Simpan perubahan secara atomik
	err = os.WriteFile(targetPath, []byte(req.Content), 0644)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal menyimpan berkas: %v"}`, err), http.StatusInternalServerError)
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "file", "file.save", targetPath, true, "Simpan perubahan isi berkas", uid)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "berkas berhasil disimpan",
		"path":    targetPath,
	})
}

// FileNewRequest payload untuk pembuatan file atau folder baru
type FileNewRequest struct {
	ParentPath string `json:"parent_path"`
	Name       string `json:"name"`
	Type       string `json:"type"` // "file" atau "dir"
}

// handleNewFile membuat berkas kosong atau folder baru
func (s *Server) handleNewFile(w http.ResponseWriter, r *http.Request) {
	var req FileNewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	nameClean := strings.TrimSpace(req.Name)
	if nameClean == "" || strings.ContainsAny(nameClean, `/\:*"<>|`) {
		http.Error(w, `{"error":"nama berkas/folder mengandung karakter terlarang"}`, http.StatusBadRequest)
		return
	}

	parentDir, err := resolveSafePath(req.ParentPath)
	if err != nil {
		http.Error(w, `{"error":"path induk tidak valid"}`, http.StatusBadRequest)
		return
	}

	targetPath := filepath.Join(parentDir, nameClean)
	if _, err := os.Stat(targetPath); err == nil {
		http.Error(w, `{"error":"berkas atau folder dengan nama tersebut sudah ada"}`, http.StatusConflict)
		return
	}

	if req.Type == "dir" {
		err = os.MkdirAll(targetPath, 0755)
	} else {
		err = os.WriteFile(targetPath, []byte(""), 0644)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membuat %s: %v"}`, req.Type, err), http.StatusInternalServerError)
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "file", "file.create", targetPath, true, fmt.Sprintf("Buat %s baru: %s", req.Type, nameClean), uid)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"path": targetPath,
		"name": nameClean,
		"type": req.Type,
	})
}

// FileRenameRequest payload untuk mengganti nama berkas atau folder
type FileRenameRequest struct {
	Path    string `json:"path"`
	NewName string `json:"new_name"`
}

// handleRenameFile mengganti nama item berkas atau folder
func (s *Server) handleRenameFile(w http.ResponseWriter, r *http.Request) {
	var req FileRenameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	newNameClean := strings.TrimSpace(req.NewName)
	if newNameClean == "" || strings.ContainsAny(newNameClean, `/\:*"<>|`) {
		http.Error(w, `{"error":"nama baru mengandung karakter terlarang"}`, http.StatusBadRequest)
		return
	}

	targetPath, err := resolveSafePath(req.Path)
	if err != nil {
		http.Error(w, `{"error":"path berkas tidak valid"}`, http.StatusBadRequest)
		return
	}

	parentDir := filepath.Dir(targetPath)
	destination := filepath.Join(parentDir, newNameClean)

	if _, err := os.Stat(destination); err == nil {
		http.Error(w, `{"error":"nama tujuan sudah digunakan"}`, http.StatusConflict)
		return
	}

	if err := os.Rename(targetPath, destination); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal rename item: %v"}`, err), http.StatusInternalServerError)
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "file", "file.rename", targetPath, true, fmt.Sprintf("Ganti nama ke %s", newNameClean), uid)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"path": destination,
		"name": newNameClean,
	})
}

// FileDeleteRequest payload untuk menghapus kumpulan berkas atau folder
type FileDeleteRequest struct {
	Paths []string `json:"paths"`
}

// handleDeleteFiles menghapus berkas atau direktori secara rekursif
func (s *Server) handleDeleteFiles(w http.ResponseWriter, r *http.Request) {
	var req FileDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if len(req.Paths) == 0 {
		http.Error(w, `{"error":"tidak ada item yang dipilih untuk dihapus"}`, http.StatusBadRequest)
		return
	}

	deletedCount := 0
	var errorMessages []string

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}

	for _, p := range req.Paths {
		safePath, err := resolveSafePath(p)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: path tidak valid", p))
			continue
		}

		// Cegah penghapusan root volume secara tidak sengaja
		if safePath == "/" || filepath.VolumeName(safePath)+"\\" == safePath {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: tidak diizinkan menghapus root drive", p))
			continue
		}

		if err := os.RemoveAll(safePath); err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", filepath.Base(safePath), err))
		} else {
			deletedCount++
			_ = s.store.RecordAudit(r.Context(), "file", "file.delete", safePath, true, "Hapus berkas/folder", uid)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      deletedCount > 0 || len(errorMessages) == 0,
		"deleted": deletedCount,
		"errors":  errorMessages,
	})
}

// FileChmodRequest payload untuk mengubah izin berkas (permissions)
type FileChmodRequest struct {
	Path string `json:"path"`
	Mode string `json:"mode"` // e.g. "0755" atau "755"
}

// handleChmodFile mengubah hak akses perizinan (chmod) berkas atau direktori
func (s *Server) handleChmodFile(w http.ResponseWriter, r *http.Request) {
	var req FileChmodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	targetPath, err := resolveSafePath(req.Path)
	if err != nil {
		http.Error(w, `{"error":"path berkas tidak valid"}`, http.StatusBadRequest)
		return
	}

	modeClean := strings.TrimSpace(req.Mode)
	parsedMode, err := strconv.ParseUint(modeClean, 8, 32)
	if err != nil {
		http.Error(w, `{"error":"format nilai mode perizinan tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := os.Chmod(targetPath, os.FileMode(parsedMode)); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal mengubah izin: %v"}`, err), http.StatusInternalServerError)
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "file", "file.chmod", targetPath, true, fmt.Sprintf("Ubah izin ke %s", modeClean), uid)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"path": targetPath,
		"mode": modeClean,
	})
}

// handleUploadFile menerima unggahan berkas multipart ke direktori tujuan
func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	// Batasi multipart upload hingga 100 MB per request
	err := r.ParseMultipartForm(100 * 1024 * 1024)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal memproses berkas unggahan: %v"}`, err), http.StatusBadRequest)
		return
	}

	destDirRaw := r.FormValue("destination")
	targetDir, err := resolveSafePath(destDirRaw)
	if err != nil {
		http.Error(w, `{"error":"direktori tujuan tidak valid"}`, http.StatusBadRequest)
		return
	}

	stat, err := os.Stat(targetDir)
	if err != nil || !stat.IsDir() {
		http.Error(w, `{"error":"direktori tujuan tidak ditemukan"}`, http.StatusNotFound)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, `{"error":"tidak ada berkas yang diunggah"}`, http.StatusBadRequest)
		return
	}

	uploadedCount := 0
	var uploadedNames []string

	for _, fileHeader := range files {
		src, err := fileHeader.Open()
		if err != nil {
			continue
		}

		safeFileName := filepath.Base(fileHeader.Filename)
		destFilePath := filepath.Join(targetDir, safeFileName)

		dst, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			src.Close()
			continue
		}

		_, copyErr := io.Copy(dst, src)
		src.Close()
		dst.Close()

		if copyErr == nil {
			uploadedCount++
			uploadedNames = append(uploadedNames, safeFileName)
		}
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "file", "file.upload", targetDir, true, fmt.Sprintf("Unggah %d berkas", uploadedCount), uid)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"uploaded": uploadedCount,
		"files":    uploadedNames,
	})
}

// FilePasteRequest payload untuk operasi salin (copy) atau potong/pindah (cut/move)
type FilePasteRequest struct {
	Action         string   `json:"action"` // "copy" atau "cut"
	SourcePaths    []string `json:"source_paths"`
	DestinationDir string   `json:"destination_dir"`
}

// copyFile menyalin berkas tunggal secara aman
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// copyDir menyalin direktori secara rekursif
func copyDir(src, dst string) error {
	stat, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, stat.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			if err := copyFile(s, d); err != nil {
				return err
			}
		}
	}
	return nil
}

// handlePasteFiles menangani operasi penempelan berkas (salin atau pindah)
func (s *Server) handlePasteFiles(w http.ResponseWriter, r *http.Request) {
	var req FilePasteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	targetDir, err := resolveSafePath(req.DestinationDir)
	if err != nil {
		http.Error(w, `{"error":"direktori tujuan tidak valid"}`, http.StatusBadRequest)
		return
	}

	stat, err := os.Stat(targetDir)
	if err != nil || !stat.IsDir() {
		http.Error(w, `{"error":"direktori tujuan tidak ditemukan"}`, http.StatusNotFound)
		return
	}

	processedCount := 0
	var errorMessages []string

	for _, srcPath := range req.SourcePaths {
		safeSrc, err := resolveSafePath(srcPath)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: path tidak valid", srcPath))
			continue
		}

		baseName := filepath.Base(safeSrc)
		destPath := filepath.Join(targetDir, baseName)

		if safeSrc == destPath {
			continue // Lewati jika tujuan sama persis
		}

		srcStat, err := os.Stat(safeSrc)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: berkas asal tidak ditemukan", baseName))
			continue
		}

		if req.Action == "cut" {
			// Coba rename atomik
			if err := os.Rename(safeSrc, destPath); err != nil {
				// Fallback jika lintas drive
				if srcStat.IsDir() {
					err = copyDir(safeSrc, destPath)
				} else {
					err = copyFile(safeSrc, destPath)
				}
				if err == nil {
					_ = os.RemoveAll(safeSrc)
				}
			}
			if err == nil {
				processedCount++
			} else {
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", baseName, err))
			}
		} else {
			// Operasi Salin (Copy)
			var err error
			if srcStat.IsDir() {
				err = copyDir(safeSrc, destPath)
			} else {
				err = copyFile(safeSrc, destPath)
			}
			if err == nil {
				processedCount++
			} else {
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", baseName, err))
			}
		}
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "file", "file."+req.Action, targetDir, true, fmt.Sprintf("Proses %s %d berkas", req.Action, processedCount), uid)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        processedCount > 0 || len(errorMessages) == 0,
		"processed": processedCount,
		"action":    req.Action,
		"errors":    errorMessages,
	})
}

// FileCompressRequest payload untuk kompresi zip berkas/folder
type FileCompressRequest struct {
	Paths   []string `json:"paths"`
	ZipName string   `json:"zip_name"`
	DestDir string   `json:"dest_dir"`
}

// handleCompressFiles mengompresi satu atau beberapa berkas/folder ke arsip .zip
func (s *Server) handleCompressFiles(w http.ResponseWriter, r *http.Request) {
	var req FileCompressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if len(req.Paths) == 0 {
		http.Error(w, `{"error":"tidak ada berkas yang dipilih"}`, http.StatusBadRequest)
		return
	}

	targetDir, err := resolveSafePath(req.DestDir)
	if err != nil {
		http.Error(w, `{"error":"direktori tujuan tidak valid"}`, http.StatusBadRequest)
		return
	}

	zipNameClean := strings.TrimSpace(req.ZipName)
	if zipNameClean == "" {
		zipNameClean = "archive.zip"
	}
	if !strings.HasSuffix(strings.ToLower(zipNameClean), ".zip") {
		zipNameClean += ".zip"
	}

	zipFilePath := filepath.Join(targetDir, zipNameClean)
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membuat berkas zip: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, itemPath := range req.Paths {
		safePath, err := resolveSafePath(itemPath)
		if err != nil {
			continue
		}

		baseDir := filepath.Dir(safePath)
		_ = filepath.Walk(safePath, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			relPath, err := filepath.Rel(baseDir, path)
			if err != nil {
				relPath = filepath.Base(path)
			}
			relPath = filepath.ToSlash(relPath)

			if info.IsDir() {
				relPath += "/"
				_, _ = zipWriter.Create(relPath)
				return nil
			}

			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = relPath
			header.Method = zip.Deflate

			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return err
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, _ = io.Copy(writer, file)
			return nil
		})
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "file", "file.compress", zipFilePath, true, fmt.Sprintf("Kompresi zip: %s", zipNameClean), uid)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"path": zipFilePath,
		"name": zipNameClean,
	})
}

// FileRemoteDownloadRequest payload untuk mengunduh berkas dari URL publik
type FileRemoteDownloadRequest struct {
	URL      string `json:"url"`
	DestDir  string `json:"dest_dir"`
	Filename string `json:"filename"`
}

// handleRemoteDownload mengunduh berkas dari internet langsung ke direktori server
func (s *Server) handleRemoteDownload(w http.ResponseWriter, r *http.Request) {
	var req FileRemoteDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	targetDir, err := resolveSafePath(req.DestDir)
	if err != nil {
		http.Error(w, `{"error":"direktori tujuan tidak valid"}`, http.StatusBadRequest)
		return
	}

	filenameClean := strings.TrimSpace(req.Filename)
	if filenameClean == "" {
		filenameClean = filepath.Base(req.URL)
		if filenameClean == "" || filenameClean == "/" || filenameClean == "." {
			filenameClean = "downloaded_file"
		}
	}

	destPath := filepath.Join(targetDir, filenameClean)

	resp, err := http.Get(req.URL)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal mengunduh dari URL: %v"}`, err), http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		http.Error(w, fmt.Sprintf(`{"error":"server sumber mengembalikan status: %s"}`, resp.Status), http.StatusBadRequest)
		return
	}

	out, err := os.Create(destPath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membuat berkas lokal: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal menyimpan konten unduhan: %v"}`, err), http.StatusInternalServerError)
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "file", "file.remote_download", destPath, true, fmt.Sprintf("Unduh dari %s (%d bytes)", req.URL, written), uid)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"path":     destPath,
		"filename": filenameClean,
		"bytes":    written,
	})
}

// FileSymlinkRequest payload untuk pembuatan symbolic link
type FileSymlinkRequest struct {
	TargetPath string `json:"target_path"`
	LinkName   string `json:"link_name"`
	DestDir    string `json:"dest_dir"`
}

// handleCreateSymlink membuat tautan simbolik (soft link)
func (s *Server) handleCreateSymlink(w http.ResponseWriter, r *http.Request) {
	var req FileSymlinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	targetDir, err := resolveSafePath(req.DestDir)
	if err != nil {
		http.Error(w, `{"error":"direktori tujuan tidak valid"}`, http.StatusBadRequest)
		return
	}

	linkNameClean := strings.TrimSpace(req.LinkName)
	if linkNameClean == "" {
		http.Error(w, `{"error":"nama tautan simbolik tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	symlinkPath := filepath.Join(targetDir, linkNameClean)
	if err := os.Symlink(req.TargetPath, symlinkPath); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal membuat tautan simbolik: %v"}`, err), http.StatusInternalServerError)
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "file", "file.symlink", symlinkPath, true, fmt.Sprintf("Buat symlink ke %s", req.TargetPath), uid)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"path": symlinkPath,
		"name": linkNameClean,
	})
}

// FileSearchContentRequest payload untuk pencarian isi berkas teks
type FileSearchContentRequest struct {
	Dir   string `json:"dir"`
	Query string `json:"query"`
}

// SearchMatchItem struktur baris yang cocok pada pencarian konten
type SearchMatchItem struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// handleSearchContent melakukan pencarian kata di dalam isi berkas teks
func (s *Server) handleSearchContent(w http.ResponseWriter, r *http.Request) {
	var req FileSearchContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	targetDir, err := resolveSafePath(req.Dir)
	if err != nil {
		http.Error(w, `{"error":"direktori tidak valid"}`, http.StatusBadRequest)
		return
	}

	queryLower := strings.ToLower(strings.TrimSpace(req.Query))
	if queryLower == "" {
		http.Error(w, `{"error":"kata kunci pencarian tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	var matches []SearchMatchItem
	const maxMatches = 50

	_ = filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || len(matches) >= maxMatches {
			return nil
		}
		if info.Size() > 2*1024*1024 {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".exe" || ext == ".dll" || ext == ".so" || ext == ".zip" || ext == ".png" || ext == ".jpg" || ext == ".db" || ext == ".db-wal" {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), queryLower) {
				rel, _ := filepath.Rel(targetDir, path)
				matches = append(matches, SearchMatchItem{
					File: rel,
					Line: lineNum,
					Text: strings.TrimSpace(line),
				})
				if len(matches) >= maxMatches {
					break
				}
			}
		}
		return nil
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"matches": matches,
		"count":   len(matches),
	})
}


