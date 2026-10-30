package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"fikpanel/internal/cloudflare"
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

// handleListCloudflareZones mengambil daftar seluruh domain di akun Cloudflare beserta deteksi arah DNS & diagnostik izin token
func (s *Server) handleListCloudflareZones(w http.ResponseWriter, r *http.Request) {
	if s.cloudflare == nil {
		http.Error(w, `{"error":"Cloudflare client belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	// Deteksi seluruh IP server (dengan IPv4 Publik di urutan indeks 0)
	var serverIPs []string
	if s.sites != nil {
		serverIPs = s.sites.ServerIPs(r.Context())
	}
	// Periksa query param server_ip jika pengguna menentukan manual
	if ipParam := strings.TrimSpace(r.URL.Query().Get("server_ip")); ipParam != "" {
		serverIPs = append([]string{ipParam}, serverIPs...)
	}

	res, err := s.cloudflare.ListZonesWithDiagnostics(r.Context(), serverIPs)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":               true,
		"zones":            res.Zones,
		"server_ip":        res.ServerIP,
		"all_server_ips":   res.AllServerIPs,
		"api_zones_count":  res.APIZonesCount,
		"token_diagnostic": res.TokenDiagnostic,
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
	if serverIP == "" && s.sites != nil {
		serverIP = strings.Join(s.sites.ServerIPs(r.Context()), ",")
	}
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

// handleSaveCloudflareDNSRecord membuat atau memperbarui entri DNS di zona Cloudflare
func (s *Server) handleSaveCloudflareDNSRecord(w http.ResponseWriter, r *http.Request) {
	if s.cloudflare == nil {
		http.Error(w, `{"error":"Cloudflare client belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	zoneID := r.PathValue("id")
	if zoneID == "" {
		http.Error(w, `{"error":"ID zone tidak valid"}`, http.StatusBadRequest)
		return
	}

	var req cloudflare.DNSRecordInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	rec, err := s.cloudflare.SaveDNSRecord(r.Context(), zoneID, req)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "cloudflare.dns_save", req.Name, true, "Menyimpan DNS record "+req.Type+" "+req.Name+" -> "+req.Content, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Rekaman DNS Cloudflare berhasil disimpan",
		"record":  rec,
	})
}

// handleDeleteCloudflareDNSRecord menghapus satu entri DNS dari zona Cloudflare
func (s *Server) handleDeleteCloudflareDNSRecord(w http.ResponseWriter, r *http.Request) {
	if s.cloudflare == nil {
		http.Error(w, `{"error":"Cloudflare client belum diinisialisasi"}`, http.StatusInternalServerError)
		return
	}

	zoneID := r.PathValue("id")
	recID := r.PathValue("record_id")
	if zoneID == "" || recID == "" {
		http.Error(w, `{"error":"ID zone atau record tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.cloudflare.DeleteDNSRecord(r.Context(), zoneID, recID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "cloudflare.dns_delete", recID, true, "Menghapus DNS record Cloudflare", nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Rekaman DNS berhasil dihapus",
	})
}

// handleUpdateCloudflareZoneSetting mengubah pengaturan edge Cloudflare (ssl, security_level, always_use_https, brotli, tls_1_3, dll.)
func (s *Server) handleUpdateCloudflareZoneSetting(w http.ResponseWriter, r *http.Request) {
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
		Setting string `json:"setting"`
		Value   string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.cloudflare.UpdateZoneSetting(r.Context(), zoneID, strings.TrimSpace(req.Setting), strings.TrimSpace(req.Value)); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "cloudflare.zone_setting", zoneID, true, "Mengubah pengaturan Cloudflare "+req.Setting+" = "+req.Value, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Pengaturan Cloudflare berhasil diperbarui",
		"setting": req.Setting,
		"value":   req.Value,
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
