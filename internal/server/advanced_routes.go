package server

import (
	"encoding/json"
	"net/http"
	"strconv"

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
		http.Error(w, `{"error":"gagal mengupdate php.ini"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
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
