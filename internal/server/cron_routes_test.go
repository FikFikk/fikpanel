package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"fikpanel/internal/store"
)

func TestCronRoutesValidationRunStopAndSystem(t *testing.T) {
	srv, cookie, cleanup := setupTestServerWithCookie(t)
	defer cleanup()

	// 1. Tolak ekspresi crontab tidak valid (HTTP 400)
	badSpecBody, _ := json.Marshal(map[string]any{
		"name":    "Jadwal Ngawur",
		"spec":    "bukan crontab valid",
		"command": "echo hi",
	})
	reqBadSpec := httptest.NewRequest(http.MethodPost, "/api/cron/jobs", bytes.NewReader(badSpecBody))
	reqBadSpec.AddCookie(cookie)
	reqBadSpec.Header.Set("Content-Type", "application/json")
	reqBadSpec.Header.Set("X-Panel-Request", "1")
	wBadSpec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wBadSpec, reqBadSpec)
	if wBadSpec.Code != http.StatusBadRequest {
		t.Fatalf("Spec tidak valid harus ditolak 400, didapat: %d %s", wBadSpec.Code, wBadSpec.Body.String())
	}

	// 2. Tolak perintah berbahaya shutdown/mkfs/rm -rf / (HTTP 400)
	badCmdBody, _ := json.Marshal(map[string]any{
		"name":    "Perintah Berbahaya",
		"spec":    "0 2 * * *",
		"command": "shutdown -h now",
	})
	reqBadCmd := httptest.NewRequest(http.MethodPost, "/api/cron/jobs", bytes.NewReader(badCmdBody))
	reqBadCmd.AddCookie(cookie)
	reqBadCmd.Header.Set("Content-Type", "application/json")
	reqBadCmd.Header.Set("X-Panel-Request", "1")
	wBadCmd := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wBadCmd, reqBadCmd)
	if wBadCmd.Code != http.StatusBadRequest {
		t.Fatalf("Perintah berbahaya harus ditolak 400, didapat: %d %s", wBadCmd.Code, wBadCmd.Body.String())
	}

	// 3. Buat tugas valid dengan @daily -> dinormalisasi ke "0 0 * * *" dan next_run_at terisi
	goodBody, _ := json.Marshal(map[string]any{
		"name":           "Echo Produksi",
		"schedule_type":  "daily",
		"spec":           "@daily",
		"command":        "echo fikpanel-cron-ok",
		"timeout_sec":    60,
		"save_log":       true,
		"notify_on_fail": true,
	})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/cron/jobs", bytes.NewReader(goodBody))
	reqCreate.AddCookie(cookie)
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("X-Panel-Request", "1")
	wCreate := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wCreate, reqCreate)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/cron/jobs valid gagal: %d %s", wCreate.Code, wCreate.Body.String())
	}
	var createResp struct {
		OK  bool          `json:"ok"`
		Job store.CronJob `json:"job"`
	}
	_ = json.NewDecoder(wCreate.Body).Decode(&createResp)
	if !createResp.OK || createResp.Job.Spec != "0 0 * * *" || createResp.Job.NextRunAt <= 0 {
		t.Fatalf("Respon pembuatan cron tidak sesuai: %+v", createResp.Job)
	}

	// 4. Jalankan sekarang (POST /api/cron/jobs/{id}/run) -> cepat selesai mengembalikan HTTP 200 + log
	reqRun := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/cron/jobs/%d/run", createResp.Job.ID), nil)
	reqRun.AddCookie(cookie)
	reqRun.Header.Set("X-Panel-Request", "1")
	wRun := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wRun, reqRun)
	if wRun.Code != http.StatusOK {
		t.Fatalf("POST /api/cron/jobs/{id}/run gagal: %d %s", wRun.Code, wRun.Body.String())
	}

	// 5. Endpoint read-only GET /api/cron/system harus mengembalikan HTTP 200
	reqSys := httptest.NewRequest(http.MethodGet, "/api/cron/system", nil)
	reqSys.AddCookie(cookie)
	wSys := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wSys, reqSys)
	if wSys.Code != http.StatusOK {
		t.Fatalf("GET /api/cron/system gagal: %d %s", wSys.Code, wSys.Body.String())
	}

	// 6. Hapus riwayat log DB (DELETE /api/cron/jobs/{id}/logs)
	reqClr := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/cron/jobs/%d/logs", createResp.Job.ID), nil)
	reqClr.AddCookie(cookie)
	reqClr.Header.Set("X-Panel-Request", "1")
	wClr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wClr, reqClr)
	if wClr.Code != http.StatusOK {
		t.Fatalf("DELETE /api/cron/jobs/{id}/logs gagal: %d %s", wClr.Code, wClr.Body.String())
	}
}
