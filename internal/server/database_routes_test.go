package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

func TestDatabaseRoutes(t *testing.T) {
	// Buat database SQLite sementara untuk pengujian
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_db_routes.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("gagal membuka store sementara: %v", err)
	}
	defer st.Close()

	hub := metrics.NewHub()
	svc := services.NewManager(st.DB())
	srv, err := NewServer(st, hub, svc, 8888)
	if err != nil {
		t.Fatalf("gagal menginisialisasi server: %v", err)
	}

	// 1. Uji GET /api/databases/engines
	reqEngines := httptest.NewRequest(http.MethodGet, "/api/databases/engines", nil)
	wEngines := httptest.NewRecorder()
	srv.handleListEngines(wEngines, reqEngines)

	if wEngines.Code != http.StatusOK {
		t.Fatalf("diharapkan status 200, didapat %d", wEngines.Code)
	}

	var enginesResp struct {
		Engines []struct {
			Name      string `json:"name"`
			Installed bool   `json:"installed"`
		} `json:"engines"`
	}
	if err := json.Unmarshal(wEngines.Body.Bytes(), &enginesResp); err != nil {
		t.Fatalf("gagal unmarshal respon engines: %v", err)
	}
	if len(enginesResp.Engines) != 4 {
		t.Errorf("diharapkan 4 engine terdaftar, didapat %d", len(enginesResp.Engines))
	}

	// 2. Uji GET /api/databases/mysql/list (memastikan database bersih tanpa data dummy awal)
	reqList := httptest.NewRequest(http.MethodGet, "/api/databases/mysql/list", nil)
	reqList.SetPathValue("engine", "mysql")
	wList := httptest.NewRecorder()
	srv.handleListDatabases(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("diharapkan status 200 saat mengambil daftar db, didapat %d", wList.Code)
	}

	var listResp struct {
		Databases []store.DatabaseRecord `json:"databases"`
	}
	if err := json.Unmarshal(wList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("gagal parse daftar database: %v", err)
	}
	if len(listResp.Databases) != 0 {
		t.Errorf("diharapkan database awal bersih (0 database), didapat: %d", len(listResp.Databases))
	}

	// 3. Uji POST /api/databases/mysql/create
	createPayload := map[string]string{
		"name":        "sql_unit_test",
		"username":    "usr_unit_test",
		"password":    "P@ssw0rdUnitTest123!",
		"access_host": "localhost",
		"note":        "Testing creation",
	}
	bodyBytes, _ := json.Marshal(createPayload)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/databases/mysql/create", bytes.NewReader(bodyBytes))
	reqCreate.SetPathValue("engine", "mysql")
	wCreate := httptest.NewRecorder()
	srv.handleCreateDatabase(wCreate, reqCreate)

	if wCreate.Code != http.StatusCreated {
		t.Fatalf("diharapkan status 201 saat membuat db, didapat %d: %s", wCreate.Code, wCreate.Body.String())
	}

	// 4. Uji DELETE /api/databases/mysql/{name}
	reqDelete := httptest.NewRequest(http.MethodDelete, "/api/databases/mysql/sql_unit_test", nil)
	reqDelete.SetPathValue("engine", "mysql")
	reqDelete.SetPathValue("name", "sql_unit_test")
	wDelete := httptest.NewRecorder()
	srv.handleDeleteDatabase(wDelete, reqDelete)

	if wDelete.Code != http.StatusOK {
		t.Fatalf("diharapkan status 200 saat menghapus db, didapat %d: %s", wDelete.Code, wDelete.Body.String())
	}

	// 5. Uji POST /api/databases/install (Quick Install trigger)
	installPayload := map[string]string{
		"engine":  "mongodb",
		"version": "7.0",
		"mode":    "native",
	}
	installBytes, _ := json.Marshal(installPayload)
	reqInstall := httptest.NewRequest(http.MethodPost, "/api/databases/install", bytes.NewReader(installBytes))
	wInstall := httptest.NewRecorder()
	srv.handleInstallDatabase(wInstall, reqInstall)

	if wInstall.Code != http.StatusAccepted {
		t.Fatalf("diharapkan status 202 saat memicu install, didapat %d: %s", wInstall.Code, wInstall.Body.String())
	}

	var installResp struct {
		OK     bool   `json:"ok"`
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(wInstall.Body.Bytes(), &installResp); err != nil || !installResp.OK || installResp.TaskID == "" {
		t.Fatalf("respon instalasi tidak valid: %s", wInstall.Body.String())
	}

	// 6. Uji GET /api/system/tasks
	reqTasks := httptest.NewRequest(http.MethodGet, "/api/system/tasks", nil)
	wTasks := httptest.NewRecorder()
	srv.handleListTasks(wTasks, reqTasks)

	if wTasks.Code != http.StatusOK {
		t.Fatalf("diharapkan status 200 saat mengambil daftar tasks, didapat %d", wTasks.Code)
	}
}
