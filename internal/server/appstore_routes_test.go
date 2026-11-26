package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"
)

func TestAppStoreRoutesAndValidation(t *testing.T) {
	tempDir := t.TempDir()
	st, err := store.Open(filepath.Join(tempDir, "test_appstore.db"))
	if err != nil {
		t.Fatalf("gagal membuka store: %v", err)
	}
	defer st.Close()

	if _, err := st.EnsureDevUser("root", ""); err != nil {
		t.Fatalf("gagal seed dev user: %v", err)
	}

	srv, err := NewServer(st, metrics.NewHub(), services.NewManager(st.DB()), 0)
	if err != nil {
		t.Fatalf("gagal membuat server: %v", err)
	}

	// Login untuk memperoleh cookie sesi admin
	loginBody, _ := json.Marshal(map[string]string{"username": "root", "password": ""})
	reqLogin := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wLogin, reqLogin)
	if wLogin.Code != http.StatusOK {
		t.Fatalf("login gagal: %d", wLogin.Code)
	}
	var sessionCookie *http.Cookie
	for _, c := range wLogin.Result().Cookies() {
		if c.Name == "panel_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("cookie panel_session tidak ditemukan")
	}

	// 1. GET /api/appstore/catalog
	reqCat := httptest.NewRequest("GET", "/api/appstore/catalog", nil)
	reqCat.AddCookie(sessionCookie)
	wCat := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wCat, reqCat)
	if wCat.Code != http.StatusOK || !strings.Contains(wCat.Body.String(), `"golang"`) || !strings.Contains(wCat.Body.String(), `"cloudflared"`) {
		t.Fatalf("katalog appstore harus memuat paket baru, dapat %d: %s", wCat.Code, wCat.Body.String())
	}

	// 2. GET /api/appstore/bundles
	reqBun := httptest.NewRequest("GET", "/api/appstore/bundles", nil)
	reqBun.AddCookie(sessionCookie)
	wBun := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wBun, reqBun)
	if wBun.Code != http.StatusOK || !strings.Contains(wBun.Body.String(), "bundle-laravel") {
		t.Fatalf("endpoint bundles harus mengembalikan bundle-laravel, dapat %d: %s", wBun.Code, wBun.Body.String())
	}

	// 3. GET /api/appstore/inspect/nginx
	reqInsp := httptest.NewRequest("GET", "/api/appstore/inspect/nginx", nil)
	reqInsp.AddCookie(sessionCookie)
	wInsp := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wInsp, reqInsp)
	if wInsp.Code != http.StatusOK || !strings.Contains(wInsp.Body.String(), `"name":"nginx"`) {
		t.Fatalf("endpoint inspect nginx gagal, dapat %d: %s", wInsp.Code, wInsp.Body.String())
	}

	// 4. Tolak injeksi flag pada POST /api/appstore/install/pkg:--force-yes
	reqBad := httptest.NewRequest("POST", "/api/appstore/install/pkg:--force-yes", nil)
	reqBad.Header.Set("X-Panel-Request", "1")
	reqBad.AddCookie(sessionCookie)
	wBad := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Fatalf("ekspektasi 400 untuk paket tidak valid, dapat %d: %s", wBad.Code, wBad.Body.String())
	}

	// 5. POST /api/appstore/apt-update & GET /api/appstore/tasks
	reqUp := httptest.NewRequest("POST", "/api/appstore/apt-update", nil)
	reqUp.Header.Set("X-Panel-Request", "1")
	reqUp.AddCookie(sessionCookie)
	wUp := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wUp, reqUp)
	if wUp.Code != http.StatusOK || !strings.Contains(wUp.Body.String(), "sys:apt-update") {
		t.Fatalf("endpoint apt-update gagal, dapat %d: %s", wUp.Code, wUp.Body.String())
	}

	reqTasks := httptest.NewRequest("GET", "/api/appstore/tasks", nil)
	reqTasks.AddCookie(sessionCookie)
	wTasks := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wTasks, reqTasks)
	if wTasks.Code != http.StatusOK || !strings.Contains(wTasks.Body.String(), "sys:apt-update") {
		t.Fatalf("endpoint tasks gagal, dapat %d: %s", wTasks.Code, wTasks.Body.String())
	}

	// 6. POST /api/appstore/service/nginx/restart
	reqSvc := httptest.NewRequest("POST", "/api/appstore/service/nginx/restart", nil)
	reqSvc.Header.Set("X-Panel-Request", "1")
	reqSvc.AddCookie(sessionCookie)
	wSvc := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wSvc, reqSvc)
	if wSvc.Code != http.StatusOK {
		t.Fatalf("endpoint service restart nginx gagal, dapat %d: %s", wSvc.Code, wSvc.Body.String())
	}
}
