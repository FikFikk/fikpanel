package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"fikpanel/internal/store"
)

// DNS Routes
func (s *Server) handleListDNSZones(w http.ResponseWriter, r *http.Request) {
	zones, err := s.store.ListDNSZones(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal memuat daftar zona DNS"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "zones": zones})
}

func (s *Server) handleCreateDNSZone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		TTL  int    `json:"ttl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, `{"error":"nama zona DNS wajib diisi"}`, http.StatusBadRequest)
		return
	}
	id, err := s.store.CreateDNSZone(r.Context(), req.Name, req.TTL)
	if err != nil {
		http.Error(w, `{"error":"gagal membuat zona DNS"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": id})
}

func (s *Server) handleDeleteDNSZone(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	if err := s.store.DeleteDNSZone(r.Context(), id); err != nil {
		http.Error(w, `{"error":"gagal menghapus zona DNS"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (s *Server) handleListDNSRecords(w http.ResponseWriter, r *http.Request) {
	zoneIDStr := r.PathValue("id")
	zoneID, _ := strconv.ParseInt(zoneIDStr, 10, 64)
	records, err := s.store.ListDNSRecords(r.Context(), zoneID)
	if err != nil {
		http.Error(w, `{"error":"gagal memuat rekod DNS"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "records": records})
}

func (s *Server) handleCreateDNSRecord(w http.ResponseWriter, r *http.Request) {
	var rec store.DNSRecord
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil || rec.ZoneID == 0 || rec.Type == "" || rec.Name == "" || rec.Content == "" {
		http.Error(w, `{"error":"parameter rekod DNS tidak lengkap"}`, http.StatusBadRequest)
		return
	}
	id, err := s.store.CreateDNSRecord(r.Context(), &rec)
	if err != nil {
		http.Error(w, `{"error":"gagal menambahkan rekod DNS"}`, http.StatusInternalServerError)
		return
	}
	rec.ID = id
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "record": rec})
}

func (s *Server) handleDeleteDNSRecord(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	if err := s.store.DeleteDNSRecord(r.Context(), id); err != nil {
		http.Error(w, `{"error":"gagal menghapus rekod DNS"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// PHP Management Routes
func (s *Server) handleListPHPVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := s.phpMgr.ListVersions(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal memeriksa instalasi PHP"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "versions": versions})
}

func (s *Server) handleRestartPHPFPM(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	if err := s.phpMgr.RestartFPM(r.Context(), ver); err != nil {
		http.Error(w, `{"error":"gagal me-restart PHP-FPM"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "version": ver})
}

func (s *Server) handleUpdatePHPSetting(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" || req.Value == "" {
		http.Error(w, `{"error":"konfigurasi tidak valid"}`, http.StatusBadRequest)
		return
	}
	if err := s.phpMgr.UpdateSetting(r.Context(), ver, req.Key, req.Value); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (s *Server) handleGetPHPVersionDetail(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	detail, err := s.phpMgr.GetVersionDetail(r.Context(), ver)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"gagal memuat detail PHP: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "detail": detail})
}

func (s *Server) handleControlPHPService(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	var req struct {
		Action string `json:"action"` // start, stop, restart, reload
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Action == "" {
		http.Error(w, `{"error":"aksi layanan wajib diisi"}`, http.StatusBadRequest)
		return
	}
	if err := s.phpMgr.ControlService(r.Context(), ver, req.Action); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", fmt.Sprintf("php.service_%s", req.Action), fmt.Sprintf("php%s-fpm", ver), true, fmt.Sprintf("Mengontrol layanan PHP %s: %s", ver, req.Action), nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "version": ver, "action": req.Action, "message": fmt.Sprintf("Layanan php%s-fpm berhasil di-%s", ver, req.Action)})
}

func (s *Server) handleSavePHPConfigBatch(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	var req struct {
		Settings map[string]string `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Settings) == 0 {
		http.Error(w, `{"error":"parameter konfigurasi tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}
	if err := s.phpMgr.SaveConfigBatch(r.Context(), ver, req.Settings); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "php.config_update", fmt.Sprintf("php%s.ini", ver), true, fmt.Sprintf("Memperbarui konfigurasi visual php.ini untuk PHP %s", ver), nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": fmt.Sprintf("Konfigurasi php.ini PHP %s berhasil disimpan dan layanan di-reload", ver)})
}

func (s *Server) handleSavePHPFpmBatch(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	var req struct {
		Settings map[string]string `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Settings) == 0 {
		http.Error(w, `{"error":"parameter pool FPM tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}
	if err := s.phpMgr.SaveFpmPoolBatch(r.Context(), ver, req.Settings); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "php.fpm_update", fmt.Sprintf("php%s-fpm-pool", ver), true, fmt.Sprintf("Memperbarui konfigurasi pool FPM www.conf untuk PHP %s", ver), nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": fmt.Sprintf("Konfigurasi pool FPM PHP %s berhasil disimpan dan layanan di-reload", ver)})
}

func (s *Server) handleUpdatePHPDisabledFunctions(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	var req struct {
		Functions []string `json:"functions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"format fungsi tidak valid"}`, http.StatusBadRequest)
		return
	}
	if err := s.phpMgr.UpdateDisabledFunctions(r.Context(), ver, req.Functions); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "php.disable_functions_update", fmt.Sprintf("php%s", ver), true, fmt.Sprintf("Memperbarui daftar disable_functions untuk PHP %s", ver), nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": fmt.Sprintf("Daftar disable_functions PHP %s berhasil disimpan dan layanan di-reload", ver)})
}

func (s *Server) handleGetPHPRawIni(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	content, path, err := s.phpMgr.GetRawIni(r.Context(), ver)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "version": ver, "path": path, "content": content})
}

func (s *Server) handleSavePHPRawIni(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"konten berkas tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}
	if err := s.phpMgr.SaveRawIni(r.Context(), ver, req.Content); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "php.raw_ini_save", fmt.Sprintf("php%s.ini", ver), true, fmt.Sprintf("Menyimpan berkas mentah php.ini untuk PHP %s", ver), nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": fmt.Sprintf("Berkas php.ini untuk PHP %s berhasil disimpan dan PHP-FPM di-reload", ver)})
}

func (s *Server) handleGetPHPRawFpm(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	content, path, err := s.phpMgr.GetRawFpm(r.Context(), ver)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "version": ver, "path": path, "content": content})
}

func (s *Server) handleSavePHPRawFpm(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"konten berkas pool tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}
	if err := s.phpMgr.SaveRawFpm(r.Context(), ver, req.Content); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "php.raw_fpm_save", fmt.Sprintf("php%s-pool.conf", ver), true, fmt.Sprintf("Menyimpan berkas mentah pool www.conf untuk PHP %s", ver), nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": fmt.Sprintf("Berkas pool www.conf untuk PHP %s berhasil disimpan dan PHP-FPM di-reload", ver)})
}

func (s *Server) handleGetPHPLogs(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	logType := r.URL.Query().Get("type")
	if logType == "" {
		logType = "fpm"
	}
	logs, err := s.phpMgr.GetLogs(r.Context(), ver, logType)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "version": ver, "type": logType, "logs": logs})
}

func (s *Server) handleClearPHPLogs(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	logType := r.URL.Query().Get("type")
	if logType == "" {
		logType = "fpm"
	}
	if err := s.phpMgr.ClearLogs(r.Context(), ver, logType); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	_ = s.store.RecordAudit(r.Context(), "user", "php.clear_logs", fmt.Sprintf("php%s-%s.log", ver, logType), true, fmt.Sprintf("Mengosongkan berkas log PHP %s (%s)", ver, logType), nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": fmt.Sprintf("Log PHP %s (%s) berhasil dikosongkan", ver, logType)})
}

func (s *Server) handleGetPHPInfo(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	info, err := s.phpMgr.GetPhpInfo(r.Context(), ver)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "version": ver, "info": info})
}

type PHPExtensionActionRequest struct {
	Extension string `json:"extension"`
}

func (s *Server) handleInstallPHPExtension(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	var req PHPExtensionActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}
	ext := strings.TrimSpace(req.Extension)
	if ext == "" {
		http.Error(w, `{"error":"nama ekstensi tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	out, err := s.phpMgr.InstallExtension(r.Context(), ver, ext)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "php.extension.install", ver, true, fmt.Sprintf("Memasang ekstensi %s untuk PHP %s", ext, ver), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Ekstensi %s berhasil dipasang untuk PHP %s", ext, ver),
		"output":  out,
	})
}

func (s *Server) handleUninstallPHPExtension(w http.ResponseWriter, r *http.Request) {
	ver := r.PathValue("version")
	var req PHPExtensionActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}
	ext := strings.TrimSpace(req.Extension)
	if ext == "" {
		http.Error(w, `{"error":"nama ekstensi tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	out, err := s.phpMgr.UninstallExtension(r.Context(), ver, ext)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "php.extension.uninstall", ver, true, fmt.Sprintf("Mencopot ekstensi %s dari PHP %s", ext, ver), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": fmt.Sprintf("Ekstensi %s berhasil dicopot dari PHP %s", ext, ver),
		"output":  out,
	})
}

// Webhooks Routes
func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	webhooks, err := s.store.ListWebhooks(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal memuat daftar webhook"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "webhooks": webhooks})
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var hook store.SystemWebhook
	if err := json.NewDecoder(r.Body).Decode(&hook); err != nil || hook.Name == "" || hook.WebhookURL == "" {
		http.Error(w, `{"error":"nama dan URL webhook wajib diisi"}`, http.StatusBadRequest)
		return
	}
	id, err := s.store.CreateWebhook(r.Context(), &hook)
	if err != nil {
		http.Error(w, `{"error":"gagal menyimpan webhook"}`, http.StatusInternalServerError)
		return
	}
	hook.ID = id
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "webhook": hook})
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	if err := s.store.DeleteWebhook(r.Context(), id); err != nil {
		http.Error(w, `{"error":"gagal menghapus webhook"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
