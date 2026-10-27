package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/waf"
)

var (
	// Cache memori lokal untuk tile peta CartoDB Dark Matter
	wafTileCache   = make(map[string][]byte)
	wafTileCacheMu sync.RWMutex
	wafTileClient  = &http.Client{Timeout: 6 * time.Second}
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

// handleWAFMapTile melayani proxy dan caching lokal tile peta dunia CartoDB Dark Matter
func (s *Server) handleWAFMapTile(w http.ResponseWriter, r *http.Request) {
	z := r.PathValue("z")
	x := r.PathValue("x")
	y := r.PathValue("y")

	if z == "" || x == "" || y == "" {
		http.Error(w, `{"error":"parameter koordinat tile tidak valid"}`, http.StatusBadRequest)
		return
	}

	y = strings.TrimSuffix(y, ".png")
	cacheKey := fmt.Sprintf("%s/%s/%s", z, x, y)

	// 1. Cek cache memori lokal terlebih dahulu
	wafTileCacheMu.RLock()
	cachedData, exists := wafTileCache[cacheKey]
	wafTileCacheMu.RUnlock()

	if exists && len(cachedData) > 0 {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=2592000, immutable")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(cachedData)
		return
	}

	// 2. Upstream Esri World Dark Gray Base (bebas API key, tanpa watermark)
	// Catatan: Format ArcGIS tile adalah /tile/{level}/{row}/{column} -> {z}/{y}/{x}
	upstreamURL := fmt.Sprintf("https://services.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Base/MapServer/tile/%s/%s/%s", z, y, x)

	req, err := http.NewRequestWithContext(r.Context(), "GET", upstreamURL, nil)
	if err != nil {
		http.Error(w, `{"error":"gagal membuat request tile upstream"}`, http.StatusInternalServerError)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FikPanel/0.2.0; +https://fikpanel.net)")

	resp, err := wafTileClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		// Fallback ke OpenStreetMap bila Esri mengalami kendala
		osmURL := fmt.Sprintf("https://tile.openstreetmap.org/%s/%s/%s.png", z, x, y)
		fallbackReq, _ := http.NewRequestWithContext(r.Context(), "GET", osmURL, nil)
		if fallbackReq != nil {
			fallbackReq.Header.Set("User-Agent", "FikPanel-WAF/0.2.0 (Admin panel map proxy)")
			resp, err = wafTileClient.Do(fallbackReq)
		}
	}

	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		http.Error(w, `{"error":"gagal mengunduh tile peta"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		http.Error(w, `{"error":"gagal membaca konten tile"}`, http.StatusInternalServerError)
		return
	}

	// 3. Simpan ke cache memori (dibatasi 1500 tile ~ 3MB RAM agar performa ringan)
	wafTileCacheMu.Lock()
	if len(wafTileCache) > 1500 {
		count := 0
		for k := range wafTileCache {
			delete(wafTileCache, k)
			count++
			if count > 750 {
				break
			}
		}
	}
	wafTileCache[cacheKey] = bodyBytes
	wafTileCacheMu.Unlock()

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=2592000, immutable")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(bodyBytes)
}

