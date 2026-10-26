package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleListAppStore(w http.ResponseWriter, r *http.Request) {
	catalog := s.appStore.Catalog(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"apps": catalog,
	})
}

func (s *Server) handleInstallApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if appID == "" {
		http.Error(w, `{"error":"ID aplikasi wajib diisi"}`, http.StatusBadRequest)
		return
	}

	if err := s.appStore.Install(appID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "appstore.install", appID, true, "Memicu 1-Click instalasi "+appID, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Instalasi telah dimulai di background",
	})
}

func (s *Server) handleGetAppTask(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	task := s.appStore.GetTask(appID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"task": task,
	})
}

func (s *Server) handleSearchAppStore(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	pkgs, err := s.appStore.SearchGlobalPackages(r.Context(), q)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"packages": pkgs,
	})
}

func (s *Server) handleUninstallApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if appID == "" {
		http.Error(w, `{"error":"ID aplikasi wajib diisi"}`, http.StatusBadRequest)
		return
	}

	if err := s.appStore.Uninstall(appID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = s.store.RecordAudit(r.Context(), "user", "appstore.uninstall", appID, true, "Memicu pencopotan "+appID, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Pencopotan paket telah dimulai di background",
	})
}

