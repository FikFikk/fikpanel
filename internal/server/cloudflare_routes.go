package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleGetCloudflareConfig mengambil status integrasi Cloudflare
func (s *Server) handleGetCloudflareConfig(w http.ResponseWriter, r *http.Request) {
	if s.cloudflare == nil {
		http.Error(w, `{"error":"Cloudflare client belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	cfg, err := s.cloudflare.GetConfig(r.Context())
	if err != nil {
		http.Error(w, `{"error":"gagal membaca konfigurasi Cloudflare: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"config": cfg,
	})
}

// handleConnectCloudflare memvalidasi dan menyimpan token Cloudflare baru
func (s *Server) handleConnectCloudflare(w http.ResponseWriter, r *http.Request) {
	if s.cloudflare == nil {
		http.Error(w, `{"error":"Cloudflare client belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	var req struct {
		APIToken string `json:"api_token"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	cfg, err := s.cloudflare.SaveConfig(r.Context(), req.APIToken, req.Email)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "cloudflare.connect", "", true, "Menghubungkan akun Cloudflare", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Berhasil terhubung ke akun Cloudflare",
		"config":  cfg,
	})
}

// handleDisconnectCloudflare memutuskan integrasi Cloudflare
func (s *Server) handleDisconnectCloudflare(w http.ResponseWriter, r *http.Request) {
	if s.cloudflare == nil {
		http.Error(w, `{"error":"Cloudflare client belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	if err := s.cloudflare.Disconnect(r.Context()); err != nil {
		http.Error(w, `{"error":"gagal memutuskan integrasi Cloudflare"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "cloudflare.disconnect", "", true, "Memutuskan integrasi Cloudflare", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Integrasi Cloudflare telah diputuskan",
	})
}

// handleListCloudflareZones mengambil daftar seluruh domain di akun Cloudflare beserta deteksi arah DNS
func (s *Server) handleListCloudflareZones(w http.ResponseWriter, r *http.Request) {
	if s.cloudflare == nil {
		http.Error(w, `{"error":"Cloudflare client belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	// Deteksi IP publik server dari manajer sites
	serverIP := ""
	if s.sites != nil {
		if ips := s.sites.ServerIPs(r.Context()); len(ips) > 0 {
			serverIP = ips[0]
		}
	}
	// Periksa query param server_ip jika pengguna menentukan manual
	if ipParam := r.URL.Query().Get("server_ip"); ipParam != "" {
		serverIP = ipParam
	}

	zones, err := s.cloudflare.ListZones(r.Context(), serverIP)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"zones":     zones,
		"server_ip": serverIP,
	})
}

// handleListCloudflareDNSRecords mengambil daftar entri DNS dari satu zone
func (s *Server) handleListCloudflareDNSRecords(w http.ResponseWriter, r *http.Request) {
	if s.cloudflare == nil {
		http.Error(w, `{"error":"Cloudflare client belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	zoneID := r.PathValue("id")
	if zoneID == "" {
		http.Error(w, `{"error":"ID zone tidak valid"}`, http.StatusBadRequest)
		return
	}

	serverIP := r.URL.Query().Get("server_ip")
	records, err := s.cloudflare.ListDNSRecords(r.Context(), zoneID, serverIP)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"records": records,
	})
}

// handlePointZoneToServer mengarahkan rekaman A domain ke server ini secara 1-klik
func (s *Server) handlePointZoneToServer(w http.ResponseWriter, r *http.Request) {
	if s.cloudflare == nil {
		http.Error(w, `{"error":"Cloudflare client belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	zoneID := r.PathValue("id")
	if zoneID == "" {
		http.Error(w, `{"error":"ID zone tidak valid"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Domain   string `json:"domain"`
		ServerIP string `json:"server_ip"`
		Proxied  bool   `json:"proxied"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.ServerIP) == "" {
		http.Error(w, `{"error":"server_ip tidak boleh kosong"}`, http.StatusBadRequest)
		return
	}

	if err := s.cloudflare.PointZoneToServer(r.Context(), zoneID, req.Domain, req.ServerIP, req.Proxied); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "cloudflare.point_dns", req.Domain, true, "Mengarahkan DNS Cloudflare domain "+req.Domain+" ke IP "+req.ServerIP, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Rekaman DNS berhasil diarahkan ke server ini",
	})
}

// handlePurgeCloudflareCache mengosongkan cache CDN Cloudflare untuk satu domain
func (s *Server) handlePurgeCloudflareCache(w http.ResponseWriter, r *http.Request) {
	if s.cloudflare == nil {
		http.Error(w, `{"error":"Cloudflare client belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	zoneID := r.PathValue("id")
	if zoneID == "" {
		http.Error(w, `{"error":"ID zone tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.cloudflare.PurgeZoneCache(r.Context(), zoneID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "cloudflare.purge_cache", zoneID, true, "Mengosongkan cache CDN Cloudflare", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Seluruh cache CDN Cloudflare berhasil dikosongkan",
	})
}

// handleToggleCloudflareDevMode mengubah status mode pengembangan (bypass cache)
func (s *Server) handleToggleCloudflareDevMode(w http.ResponseWriter, r *http.Request) {
	if s.cloudflare == nil {
		http.Error(w, `{"error":"Cloudflare client belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	zoneID := r.PathValue("id")
	if zoneID == "" {
		http.Error(w, `{"error":"ID zone tidak valid"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Enable bool `json:"enable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.cloudflare.ToggleDevelopmentMode(r.Context(), zoneID, req.Enable); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Mode pengembangan Cloudflare berhasil diperbarui",
		"enabled": req.Enable,
	})
}
