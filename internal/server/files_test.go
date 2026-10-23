package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

// setupTestServer membuat instance server uji coba dengan database SQLite in-memory
func setupTestServer(t *testing.T) (*Server, func()) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "test_panel.db")
	st, err := store.Open(tempDB)
	if err != nil {
		t.Fatalf("Gagal inisialisasi test store: %v", err)
	}

	hub := metrics.NewHub()
	svcMgr := services.NewManager(st.DB())
	srv, err := NewServer(st, hub, svcMgr, 0)
	if err != nil {
		st.Close()
		t.Fatalf("Gagal buat test server: %v", err)
	}

	cleanup := func() {
		st.Close()
	}
	return srv, cleanup
}

func TestFileManagerAPIs(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Buat direktori sementara untuk pengujian berkas
	testDir := t.TempDir()

	// 1. Uji Pembuatan Berkas Baru
	newFilePayload, _ := json.Marshal(FileNewRequest{
		ParentPath: testDir,
		Name:       "halo.txt",
		Type:       "file",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/files/new", bytes.NewReader(newFilePayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleNewFile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("handleNewFile gagal, kode: %d, body: %s", w.Code, w.Body.String())
	}

	filePath := filepath.Join(testDir, "halo.txt")

	// 2. Uji Simpan Konten Berkas
	savePayload, _ := json.Marshal(FileSaveRequest{
		Path:    filePath,
		Content: "Halo dari unit test fikpanel!",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/files/save", bytes.NewReader(savePayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.handleSaveFile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("handleSaveFile gagal, kode: %d, body: %s", w.Code, w.Body.String())
	}

	// 3. Uji Baca Konten Berkas
	req = httptest.NewRequest(http.MethodGet, "/api/files/read?path="+filePath, nil)
	w = httptest.NewRecorder()
	srv.handleReadFile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("handleReadFile gagal, kode: %d, body: %s", w.Code, w.Body.String())
	}

	var readRes map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &readRes)
	if readRes["content"] != "Halo dari unit test fikpanel!" {
		t.Errorf("Konten yang dibaca tidak sesuai: %v", readRes["content"])
	}

	// 4. Uji List Direktori
	req = httptest.NewRequest(http.MethodGet, "/api/files/list?path="+testDir, nil)
	w = httptest.NewRecorder()
	srv.handleListFiles(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("handleListFiles gagal, kode: %d, body: %s", w.Code, w.Body.String())
	}

	var listRes FileListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &listRes)
	if listRes.TotalFiles != 1 || len(listRes.Items) != 1 {
		t.Errorf("Jumlah berkas dalam list salah: total_files=%d, len=%d", listRes.TotalFiles, len(listRes.Items))
	}

	// 5. Uji Rename Berkas
	renamePayload, _ := json.Marshal(FileRenameRequest{
		Path:    filePath,
		NewName: "renamed.txt",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/files/rename", bytes.NewReader(renamePayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.handleRenameFile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("handleRenameFile gagal, kode: %d, body: %s", w.Code, w.Body.String())
	}

	renamedPath := filepath.Join(testDir, "renamed.txt")
	if _, err := os.Stat(renamedPath); err != nil {
		t.Fatalf("Berkas setelah rename tidak ditemukan: %v", err)
	}

	// 6. Uji Hapus Berkas
	delPayload, _ := json.Marshal(FileDeleteRequest{
		Paths: []string{renamedPath},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/files/delete", bytes.NewReader(delPayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.handleDeleteFiles(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("handleDeleteFiles gagal, kode: %d, body: %s", w.Code, w.Body.String())
	}

	if _, err := os.Stat(renamedPath); !os.IsNotExist(err) {
		t.Errorf("Berkas seharusnya sudah terhapus, tetapi masih ada")
	}
}

func TestFileUploadAPI(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	testDir := t.TempDir()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("destination", testDir)

	part, err := writer.CreateFormFile("files", "test_upload.log")
	if err != nil {
		t.Fatalf("Gagal buat form file: %v", err)
	}
	_, _ = part.Write([]byte("isi file upload"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/files/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.handleUploadFile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("handleUploadFile gagal, status: %d, body: %s", w.Code, w.Body.String())
	}

	uploadedFilePath := filepath.Join(testDir, "test_upload.log")
	content, err := os.ReadFile(uploadedFilePath)
	if err != nil || string(content) != "isi file upload" {
		t.Errorf("Isi berkas yang diunggah tidak sesuai: %s (err: %v)", string(content), err)
	}
}
