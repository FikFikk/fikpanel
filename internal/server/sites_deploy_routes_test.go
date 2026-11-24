package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"fikpanel/internal/store"
)

func TestQuoteEnvValueAndDeployHelpers(t *testing.T) {
	got := quoteEnvValue(`pass#123$word"ok`)
	want := `"pass#123\$word\"ok"`
	if got != want {
		t.Fatalf("quoteEnvValue salah: got=%s want=%s", got, want)
	}
	if resolvePHPBinary("") != "php" {
		t.Fatalf("resolvePHPBinary kosong harus fallback ke php")
	}
}

func TestSiteDeployConfigWebhookAndSchedulerRoutes(t *testing.T) {
	srv, cookie, cleanup := setupTestServerWithCookie(t)
	defer cleanup()

	// Buat situs PHP dummy di SQLite
	site, err := srv.store.CreateSite(t.Context(), &store.Site{
		Domain:     "filament.example.com",
		Aliases:    "[]",
		Kind:       "php",
		RootPath:   t.TempDir(),
		PHPVersion: "8.3",
		SSLMode:    "none",
		Enabled:    1,
	})
	if err != nil {
		t.Fatalf("Gagal membuat site uji: %v", err)
	}

	// 1. GET /api/sites/{id}/deploy otomatis menginisialisasi config & secret 64-hex
	reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/sites/%d/deploy", site.ID), nil)
	reqGet.AddCookie(cookie)
	wGet := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("GET deploy info gagal: %d %s", wGet.Code, wGet.Body.String())
	}
	var infoResp struct {
		OK         bool                   `json:"ok"`
		WebhookURL string                 `json:"webhook_url"`
		Config     store.SiteDeployConfig `json:"config"`
	}
	if err := json.NewDecoder(wGet.Body).Decode(&infoResp); err != nil {
		t.Fatalf("Decode deploy info gagal: %v", err)
	}
	if !infoResp.OK || len(infoResp.Config.WebhookSecret) != 64 || infoResp.Config.WebhookEnabled {
		t.Fatalf("Inisialisasi SiteDeployConfig tidak sesuai: %+v", infoResp.Config)
	}

	// 2. Webhook ditolak 403 saat belum diaktifkan
	reqWhOff := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/webhooks/git/%d/%s", site.ID, infoResp.Config.WebhookSecret), bytes.NewReader([]byte(`{}`)))
	wWhOff := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wWhOff, reqWhOff)
	if wWhOff.Code != http.StatusForbidden {
		t.Fatalf("Diharapkan 403 saat webhook nonaktif, didapat: %d", wWhOff.Code)
	}

	// 3. Aktifkan webhook & rotasi secret via PUT /api/sites/{id}/deploy/config
	updBody, _ := json.Marshal(map[string]any{
		"webhook_enabled":  true,
		"auto_optimize":    true,
		"maintenance_mode": true,
		"rebuild_assets":   true,
		"rotate_secret":    true,
	})
	reqUpd := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/sites/%d/deploy/config", site.ID), bytes.NewReader(updBody))
	reqUpd.AddCookie(cookie)
	reqUpd.Header.Set("Content-Type", "application/json")
	reqUpd.Header.Set("X-Panel-Request", "1")
	wUpd := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wUpd, reqUpd)
	if wUpd.Code != http.StatusOK {
		t.Fatalf("PUT deploy config gagal: %d %s", wUpd.Code, wUpd.Body.String())
	}
	var updResp struct {
		Config store.SiteDeployConfig `json:"config"`
	}
	_ = json.NewDecoder(wUpd.Body).Decode(&updResp)
	if !updResp.Config.WebhookEnabled || updResp.Config.WebhookSecret == infoResp.Config.WebhookSecret {
		t.Fatalf("Rotasi secret atau toggle webhook gagal: %+v", updResp.Config)
	}
	newSecret := updResp.Config.WebhookSecret

	// 4. Uji webhook ping GitHub dengan HMAC-SHA256 valid -> HTTP 200 pong
	pingPayload := []byte(`{"zen":"Keep it logically awesome."}`)
	mac := hmac.New(sha256.New, []byte(newSecret))
	mac.Write(pingPayload)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	reqPing := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/webhooks/git/%d/%s", site.ID, newSecret), bytes.NewReader(pingPayload))
	reqPing.Header.Set("X-GitHub-Event", "ping")
	reqPing.Header.Set("X-Hub-Signature-256", validSig)
	wPing := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wPing, reqPing)
	if wPing.Code != http.StatusOK {
		t.Fatalf("Webhook ping HMAC valid gagal: %d %s", wPing.Code, wPing.Body.String())
	}

	// 5. Uji webhook dengan HMAC tidak valid -> HTTP 401
	reqBadSig := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/webhooks/git/%d/%s", site.ID, newSecret), bytes.NewReader(pingPayload))
	reqBadSig.Header.Set("X-GitHub-Event", "ping")
	reqBadSig.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	wBadSig := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wBadSig, reqBadSig)
	if wBadSig.Code != http.StatusUnauthorized {
		t.Fatalf("Diharapkan 401 untuk HMAC tidak valid, didapat: %d", wBadSig.Code)
	}

	// 6. Uji validasi commit rollback (harus menolak injeksi shell / non-hex)
	badRbBody, _ := json.Marshal(map[string]string{"commit": "main; rm -rf /"})
	reqBadRb := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/sites/%d/deploy/rollback", site.ID), bytes.NewReader(badRbBody))
	reqBadRb.AddCookie(cookie)
	reqBadRb.Header.Set("Content-Type", "application/json")
	reqBadRb.Header.Set("X-Panel-Request", "1")
	wBadRb := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wBadRb, reqBadRb)
	if wBadRb.Code != http.StatusBadRequest {
		t.Fatalf("Diharapkan 400 untuk hash commit ilegal, didapat: %d %s", wBadRb.Code, wBadRb.Body.String())
	}

	// 7. Uji toggle Laravel Scheduler ON lalu OFF di Cron Manager SQLite
	schedOnBody, _ := json.Marshal(map[string]bool{"enabled": true})
	reqSchedOn := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/sites/%d/laravel/scheduler", site.ID), bytes.NewReader(schedOnBody))
	reqSchedOn.AddCookie(cookie)
	reqSchedOn.Header.Set("Content-Type", "application/json")
	reqSchedOn.Header.Set("X-Panel-Request", "1")
	wSchedOn := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wSchedOn, reqSchedOn)
	if wSchedOn.Code != http.StatusOK {
		t.Fatalf("Toggle Laravel Scheduler ON gagal: %d %s", wSchedOn.Code, wSchedOn.Body.String())
	}
	if job := srv.findSiteLaravelSchedulerCron(t.Context(), site.Domain); job == nil || !job.Enabled || job.Spec != "* * * * *" {
		t.Fatalf("Cron Laravel Scheduler tidak tersimpan dengan benar: %+v", job)
	}

	schedOffBody, _ := json.Marshal(map[string]bool{"enabled": false})
	reqSchedOff := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/sites/%d/laravel/scheduler", site.ID), bytes.NewReader(schedOffBody))
	reqSchedOff.AddCookie(cookie)
	reqSchedOff.Header.Set("Content-Type", "application/json")
	reqSchedOff.Header.Set("X-Panel-Request", "1")
	wSchedOff := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wSchedOff, reqSchedOff)
	if wSchedOff.Code != http.StatusOK {
		t.Fatalf("Toggle Laravel Scheduler OFF gagal: %d %s", wSchedOff.Code, wSchedOff.Body.String())
	}
	if job := srv.findSiteLaravelSchedulerCron(t.Context(), site.Domain); job == nil || job.Enabled {
		t.Fatalf("Cron Laravel Scheduler seharusnya nonaktif: %+v", job)
	}
}
