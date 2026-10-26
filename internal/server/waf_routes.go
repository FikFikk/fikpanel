package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"fikpanel/internal/waf"
)

// handleGetWAFOverview mengembalikan ringkasan statistik WAF, top IP serangan, dan peta
func (s *Server) handleGetWAFOverview(w http.ResponseWriter, r *http.Request) {
	if s.waf == nil {
		http.Error(w, `{"error":"WAF engine belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	ov, err := s.waf.GetOverview(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal memuat data overview WAF: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"overview": ov,
	})
}

// handleListWAFSites mengembalikan daftar seluruh situs terproteksi WAF
func (s *Server) handleListWAFSites(w http.ResponseWriter, r *http.Request) {
	if s.waf == nil {
		http.Error(w, `{"error":"WAF engine belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	sites, err := s.waf.ListSites(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal memuat daftar situs WAF"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"sites": sites,
	})
}

// handleUpdateWAFSite memperbarui parameter proteksi WAF pada satu situs
func (s *Server) handleUpdateWAFSite(w http.ResponseWriter, r *http.Request) {
	if s.waf == nil {
		http.Error(w, `{"error":"WAF engine belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	domain := r.PathValue("domain")
	if domain == "" {
		http.Error(w, `{"error":"nama domain tidak valid"}`, http.StatusBadRequest)
		return
	}

	var req waf.WAFSite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	req.SiteName = domain
	if err := s.waf.UpdateSite(r.Context(), req); err != nil {
		http.Error(w, `{"error":"gagal memperbarui konfigurasi WAF situs"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "waf.site_update", domain, true, "Memperbarui konfigurasi WAF situs "+domain, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Konfigurasi WAF situs berhasil diperbarui",
		"site":    req,
	})
}

// handleListWAFRules mengembalikan daftar aturan daftar putih/hitam WAF
func (s *Server) handleListWAFRules(w http.ResponseWriter, r *http.Request) {
	if s.waf == nil {
		http.Error(w, `{"error":"WAF engine belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	ruleType := r.URL.Query().Get("type")
	rules, err := s.waf.ListRules(r.Context(), ruleType)
	if err != nil {
		http.Error(w, `{"error":"gagal memuat aturan WAF"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"rules": rules,
	})
}

// handleAddWAFRule menambahkan aturan filter baru ke sistem WAF
func (s *Server) handleAddWAFRule(w http.ResponseWriter, r *http.Request) {
	if s.waf == nil {
		http.Error(w, `{"error":"WAF engine belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	var req waf.WAFRule
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if req.RuleType == "" {
		http.Error(w, `{"error":"rule_type wajib diisi"}`, http.StatusBadRequest)
		return
	}

	id, err := s.waf.AddRule(r.Context(), req)
	if err != nil {
		http.Error(w, `{"error":"gagal menyimpan aturan WAF"}`, http.StatusInternalServerError)
		return
	}
	req.ID = id

	_ = s.store.RecordAudit(r.Context(), "user", "waf.rule_add", req.RuleType, true, "Menambahkan aturan WAF "+req.RuleType, nil)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"rule": req,
	})
}

// handleDeleteWAFRule menghapus aturan WAF berdasarkan ID
func (s *Server) handleDeleteWAFRule(w http.ResponseWriter, r *http.Request) {
	if s.waf == nil {
		http.Error(w, `{"error":"WAF engine belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"ID aturan tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.waf.DeleteRule(r.Context(), id); err != nil {
		http.Error(w, `{"error":"gagal menghapus aturan WAF"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "waf.rule_delete", idStr, true, "Menghapus aturan WAF", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
		"id": id,
	})
}

// handleListWAFEvents mengembalikan riwayat insiden serangan WAF
func (s *Server) handleListWAFEvents(w http.ResponseWriter, r *http.Request) {
	if s.waf == nil {
		http.Error(w, `{"error":"WAF engine belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	limit := 50
	if limStr := r.URL.Query().Get("limit"); limStr != "" {
		if l, err := strconv.Atoi(limStr); err == nil && l > 0 {
			limit = l
		}
	}

	events, err := s.waf.ListEvents(r.Context(), limit)
	if err != nil {
		http.Error(w, `{"error":"gagal memuat daftar insiden WAF"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"events": events,
	})
}
