package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"fikpanel/internal/ai"
	"fikpanel/internal/store"
)

// handleListAIProviders mengembalikan seluruh daftar provider AI dengan API key tersamar
func (s *Server) handleListAIProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := s.store.ListAIProviders(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	type providerDTO struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Kind      string `json:"kind"`
		BaseURL   string `json:"base_url"`
		Model     string `json:"model"`
		APIKey    string `json:"api_key"`
		HasKey    bool   `json:"has_key"`
		IsDefault bool   `json:"is_default"`
		CreatedAt int64  `json:"created_at"`
		UpdatedAt int64  `json:"updated_at"`
	}

	var dtoList []providerDTO
	for _, p := range providers {
		dtoList = append(dtoList, providerDTO{
			ID:        p.ID,
			Name:      p.Name,
			Kind:      p.Kind,
			BaseURL:   p.BaseURL,
			Model:     p.Model,
			APIKey:    p.MaskAPIKey(),
			HasKey:    p.APIKey != "",
			IsDefault: p.IsDefault,
			CreatedAt: p.CreatedAt,
			UpdatedAt: p.UpdatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"providers": dtoList,
	})
}

// handleCreateAIProvider menambahkan provider AI kustom baru
func (s *Server) handleCreateAIProvider(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Kind      string `json:"kind"`
		BaseURL   string `json:"base_url"`
		Model     string `json:"model"`
		APIKey    string `json:"api_key"`
		IsDefault bool   `json:"is_default"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.Model = strings.TrimSpace(req.Model)
	if req.Name == "" || req.BaseURL == "" || req.Model == "" {
		http.Error(w, `{"error":"Nama, Base URL, dan Model wajib diisi"}`, http.StatusBadRequest)
		return
	}

	if req.Kind != "openai_compat" && req.Kind != "anthropic" && req.Kind != "gemini" {
		req.Kind = "openai_compat"
	}

	p := &store.AIProvider{
		Name:      req.Name,
		Kind:      req.Kind,
		BaseURL:   req.BaseURL,
		Model:     req.Model,
		APIKey:    strings.TrimSpace(req.APIKey),
		IsDefault: req.IsDefault,
	}

	if err := s.store.CreateAIProvider(r.Context(), p); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	user := GetUserFromContext(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	_ = s.store.RecordAudit(r.Context(), "user", "ai.provider_create", p.Name, true, fmt.Sprintf("Tambah provider %s (%s)", p.Name, p.Model), uid)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"provider": p,
	})
}

// handleUpdateAIProvider memperbarui konfigurasi provider yang ada
func (s *Server) handleUpdateAIProvider(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, `{"error":"ID provider tidak valid"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Name      string `json:"name"`
		Kind      string `json:"kind"`
		BaseURL   string `json:"base_url"`
		Model     string `json:"model"`
		APIKey    string `json:"api_key"`
		IsDefault bool   `json:"is_default"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.Model = strings.TrimSpace(req.Model)
	if req.Name == "" || req.BaseURL == "" || req.Model == "" {
		http.Error(w, `{"error":"Nama, Base URL, dan Model wajib diisi"}`, http.StatusBadRequest)
		return
	}

	p := &store.AIProvider{
		ID:        id,
		Name:      req.Name,
		Kind:      req.Kind,
		BaseURL:   req.BaseURL,
		Model:     req.Model,
		APIKey:    strings.TrimSpace(req.APIKey),
		IsDefault: req.IsDefault,
	}

	if err := s.store.UpdateAIProvider(r.Context(), p); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// handleDeleteAIProvider menghapus provider AI dari sistem
func (s *Server) handleDeleteAIProvider(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, `{"error":"ID provider tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteAIProvider(r.Context(), id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// handleSetDefaultAIProvider menyetel provider aktif default
func (s *Server) handleSetDefaultAIProvider(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, `{"error":"ID provider tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.store.SetDefaultAIProvider(r.Context(), id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// handleTestAIProvider menguji koneksi langsung ke provider sebelum disimpan
func (s *Server) handleTestAIProvider(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      int64  `json:"id,omitempty"`
		Kind    string `json:"kind"`
		BaseURL string `json:"base_url"`
		Model   string `json:"model"`
		APIKey  string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	// Jika ID dikirimkan dan APIKey kosong di form, gunakan API key yang tersimpan di database
	if apiKey == "" && req.ID > 0 {
		existing, _ := s.store.GetAIProvider(r.Context(), req.ID)
		if existing != nil {
			apiKey = existing.APIKey
		}
	}

	testProvider := &store.AIProvider{
		Kind:    req.Kind,
		BaseURL: strings.TrimSpace(req.BaseURL),
		Model:   strings.TrimSpace(req.Model),
		APIKey:  apiKey,
	}

	latency, reply, err := s.ai.TestProviderConnection(r.Context(), testProvider)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Uji koneksi gagal: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":         true,
		"latency_ms": latency,
		"reply":      reply,
	})
}

// handleListAIConversations mengembalikan daftar riwayat percakapan pengguna
func (s *Server) handleListAIConversations(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"tidak terautentikasi"}`, http.StatusUnauthorized)
		return
	}

	convs, err := s.store.ListAIConversations(r.Context(), user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	if convs == nil {
		convs = []store.AIConversation{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":            true,
		"conversations": convs,
	})
}

// handleCreateAIConversation membuat percakapan baru secara manual
func (s *Server) handleCreateAIConversation(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"tidak terautentikasi"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	conv, err := s.store.CreateAIConversation(r.Context(), user.ID, strings.TrimSpace(req.Title))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":           true,
		"conversation": conv,
	})
}

// handleDeleteAIConversation menghapus satu sesi percakapan
func (s *Server) handleDeleteAIConversation(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"tidak terautentikasi"}`, http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, `{"error":"ID percakapan tidak valid"}`, http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteAIConversation(r.Context(), id, user.ID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// handleClearAIConversations menghapus seluruh riwayat percakapan milik pengguna
func (s *Server) handleClearAIConversations(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"tidak terautentikasi"}`, http.StatusUnauthorized)
		return
	}

	if err := s.store.ClearAllAIConversations(r.Context(), user.ID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// handleGetAIMessages mengembalikan riwayat pesan dari percakapan tertentu
func (s *Server) handleGetAIMessages(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"tidak terautentikasi"}`, http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, `{"error":"ID percakapan tidak valid"}`, http.StatusBadRequest)
		return
	}

	// Validasi kepemilikan percakapan
	conv, err := s.store.GetAIConversation(r.Context(), id, user.ID)
	if err != nil || conv == nil {
		http.Error(w, `{"error":"percakapan tidak ditemukan"}`, http.StatusNotFound)
		return
	}

	msgs, err := s.store.ListAIMessages(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []store.AIMessage{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":           true,
		"conversation": conv,
		"messages":     msgs,
	})
}

// handleAIChat menerima pesan pengguna, memanggil provider AI, dan mengembalikan respon
func (s *Server) handleAIChat(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"tidak terautentikasi"}`, http.StatusUnauthorized)
		return
	}

	var req ai.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	resp, err := s.ai.ExecuteChat(r.Context(), user.ID, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	var uid *int64 = &user.ID
	_ = s.store.RecordAudit(r.Context(), "ai", "ai.diagnose", req.Message, true, fmt.Sprintf("AI dialog via %s", resp.ProviderName), uid)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":              true,
		"conversation_id": resp.ConversationID,
		"response":        resp.Response,
		"provider_name":   resp.ProviderName,
		"model":           resp.Model,
		"actions":         resp.Actions,
	})
}
